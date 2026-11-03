package importfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestPickReleaseIDForGroup(t *testing.T) {
	rels := []apiexternal_v2.ReleaseSearchResult{
		{MusicBrainzID: "other-group", ReleaseGroupID: "rg2", Status: "Official"},
		{MusicBrainzID: "bootleg", ReleaseGroupID: "rg1", Status: "Bootleg"},
		{MusicBrainzID: "official", ReleaseGroupID: "rg1", Status: "Official"},
	}

	if got := pickReleaseIDForGroup(rels, "rg1"); got != "official" {
		t.Errorf("got %q, want official", got)
	}

	if got := pickReleaseIDForGroup(rels[:2], "rg1"); got != "bootleg" {
		t.Errorf("fallback got %q, want bootleg", got)
	}

	if got := pickReleaseIDForGroup(rels[:1], "rg1"); got != "" {
		t.Errorf("foreign group got %q, want empty", got)
	}
}

// fakeMusicBrainz points getMusicBrainzProvider at a server that answers with
// respond(path) (an empty JSON object when it returns "") and returns the
// provider plus the request URIs seen so far.
func fakeMusicBrainz(t *testing.T, respond func(path string) string) (*musicbrainz.Provider, func() []string) {
	t.Helper()

	var (
		mu   sync.Mutex
		seen []string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.RequestURI())
		mu.Unlock()

		body := "{}"
		if respond != nil {
			if b := respond(r.URL.Path); b != "" {
				body = b
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	p := musicbrainz.NewProviderWithConfig(base.ClientConfig{
		BaseURL: srv.URL, UserAgent: "test", RateLimitCalls: 100, RateLimitSeconds: 1,
	})

	orig := getMusicBrainzProvider
	getMusicBrainzProvider = func() *musicbrainz.Provider { return p }

	t.Cleanup(func() { getMusicBrainzProvider = orig })

	return p, func() []string {
		mu.Lock()
		defer mu.Unlock()

		return slices.Clone(seen)
	}
}

// countingMusicBrainz is fakeMusicBrainz counting requests only.
func countingMusicBrainz(t *testing.T) *atomic.Int32 {
	t.Helper()

	var calls atomic.Int32

	fakeMusicBrainz(t, func(string) string {
		calls.Add(1)
		return ""
	})

	return &calls
}

func TestImportAlbumsBySeriesUsesSeriesID(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr(
		"insert into dbalbums (id, title, musicbrainz_release_group_id) values (1, 'Bravo Hits 1', 'rgKnown')",
	); err != nil {
		t.Fatal(err)
	}

	p, seen := fakeMusicBrainz(t, func(path string) string {
		if path == "/series/sid1" {
			// Shape of a real series?inc=release-group-rels response.
			return `{"type":"Release group series","relations":[
				{"type":"part of","target-type":"release_group","release_group":{"id":"rgKnown","title":"Bravo Hits 1"}},
				{"type":"part of","target-type":"release_group","release_group":{"id":"rgNew","title":"Bravo Hits 2"}}]}`
		}

		return ""
	})

	album := config.ManualConfig{AlbumSeriesName: "Bravo Hits", AlbumSeriesID: "sid1"}
	if err := importAlbumsBySeries(context.Background(), &album, &config.MediaTypeConfig{}, -1, p); err != nil {
		t.Fatal(err)
	}

	var groupLookups []string

	for _, uri := range seen() {
		if strings.HasPrefix(uri, "/release?") {
			t.Errorf("series with an ID must not run a release search: %s", uri)
		}

		if strings.HasPrefix(uri, "/release-group/") {
			groupLookups = append(groupLookups, uri)
		}
	}

	// Only the unknown volume is looked up; the known one costs no request.
	if len(groupLookups) != 1 || !strings.HasPrefix(groupLookups[0], "/release-group/rgNew") {
		t.Errorf("release-group lookups = %v, want only rgNew", groupLookups)
	}
}

func TestImportAlbumsBySeriesByName(t *testing.T) {
	useTestDB(t)

	// Two exact-name series, as on MusicBrainz; the larger one is the main
	// series. The near match is never looked up.
	p, seen := fakeMusicBrainz(t, func(path string) string {
		rel := func(ids ...string) string {
			parts := make([]string, len(ids))
			for i, id := range ids {
				parts[i] = `{"type":"part of","target-type":"release_group","release_group":{"id":"` + id + `"}}`
			}

			return `{"relations":[` + strings.Join(parts, ",") + `]}`
		}

		switch path {
		case "/series":
			return `{"series":[
				{"id":"sMisc","name":"Future Trance","type":"Release group series","disambiguation":"misc."},
				{"id":"sMain","name":"Future Trance","type":"Release group series"},
				{"id":"sBest","name":"Future Trance: Best Of","type":"Release group series"}]}`
		case "/series/sMisc":
			return rel("rgM1")
		case "/series/sMain":
			return rel("rg1", "rg2", "rg3")
		}

		return ""
	})

	album := config.ManualConfig{AlbumSeriesName: "Future Trance"}
	if err := importAlbumsBySeries(context.Background(), &album, &config.MediaTypeConfig{}, -1, p); err != nil {
		t.Fatal(err)
	}

	uris := seen()
	if len(uris) < 2 {
		t.Fatalf("requests = %v", uris)
	}

	u, _ := url.Parse(uris[0])
	if u.Path != "/series" || u.Query().Get("query") != `series:"Future Trance"` {
		t.Errorf("first request = %s, want a series search for the exact name", uris[0])
	}

	var groupLookups []string

	for _, uri := range uris {
		if strings.HasPrefix(uri, "/release?") {
			t.Errorf("resolved series must not fall back to a release search: %s", uri)
		}

		if strings.HasPrefix(uri, "/series/sBest") {
			t.Errorf("near-match series must not be looked up: %s", uri)
		}

		if strings.HasPrefix(uri, "/release-group/") {
			groupLookups = append(groupLookups, strings.SplitN(strings.TrimPrefix(uri, "/release-group/"), "?", 2)[0])
		}
	}

	// The larger series wins: its release groups are added, not sMisc's.
	if !slices.Equal(groupLookups, []string{"rg1", "rg2", "rg3"}) {
		t.Errorf("release groups added = %v, want those of the larger series", groupLookups)
	}
}

func TestImportAlbumsBySeriesNameFallbackIsPhrase(t *testing.T) {
	useTestDB(t)

	// No series found: the release title is searched as a phrase.
	p, seen := fakeMusicBrainz(t, nil)

	album := config.ManualConfig{AlbumSeriesName: "Future Trance"}
	if err := importAlbumsBySeries(context.Background(), &album, &config.MediaTypeConfig{}, -1, p); err != nil {
		t.Fatal(err)
	}

	uris := seen()
	if len(uris) != 2 {
		t.Fatalf("requests = %v, want series search then release search", uris)
	}

	u, _ := url.Parse(uris[1])
	if q := u.Query().Get("query"); u.Path != "/release" || q != `release:"Future Trance"` {
		t.Errorf("fallback = %s (query %q), want the series name as a release-title phrase", uris[1], q)
	}
}

func TestAddAlbumToDatabaseKnownAlbumNoLookups(t *testing.T) {
	useTestDB(t)

	calls := countingMusicBrainz(t)

	if err := database.ExecNErr(`insert into dbalbums (id, title, musicbrainz_release_id, musicbrainz_release_group_id, total_runtime_ms)
		values (1, 'Known', 'rel1', 'rg1', 1000)`); err != nil {
		t.Fatal(err)
	}

	if err := database.ExecNErr("insert into dbtracks (dbalbum_id, title, disc_number, track_number) values (1, 'T', 1, 1)"); err != nil {
		t.Fatal(err)
	}

	release := apiexternal_v2.ReleaseSearchResult{Title: "Known", MusicBrainzID: "rel1", ReleaseGroupID: "rg1"}
	_ = addAlbumToDatabase(context.Background(), &release, &config.MediaTypeConfig{}, -1, nil)

	if n := calls.Load(); n != 0 {
		t.Errorf("known complete album made %d MusicBrainz calls, want 0", n)
	}

	// Without tracks the details are still fetched once; the series lookup
	// stays reserved for new albums.
	if err := database.ExecNErr("delete from dbtracks"); err != nil {
		t.Fatal(err)
	}

	_ = addAlbumToDatabase(context.Background(), &release, &config.MediaTypeConfig{}, -1, nil)

	if n := calls.Load(); n != 1 {
		t.Errorf("known album without tracks made %d MusicBrainz calls, want 1 (release details)", n)
	}
}

func TestAddAlbumToDatabaseNewAlbumLooksUp(t *testing.T) {
	useTestDB(t)

	calls := countingMusicBrainz(t)

	release := apiexternal_v2.ReleaseSearchResult{Title: "New", MusicBrainzID: "rel2", ReleaseGroupID: "rg2"}
	_ = addAlbumToDatabase(context.Background(), &release, &config.MediaTypeConfig{}, -1, nil)

	// Release details plus the release-group series lookup.
	if n := calls.Load(); n != 2 {
		t.Errorf("new album made %d MusicBrainz calls, want 2", n)
	}

	if got := database.Getdatarow[int](false, "select count() from dbalbums where musicbrainz_release_id = 'rel2'"); got != 1 {
		t.Errorf("new album rows = %d, want 1", got)
	}
}

func TestReleaseImportFilters(t *testing.T) {
	cfgp := &config.MediaTypeConfig{Data: []config.MediaDataConfig{
		{},
		{AllowedReleaseTypes: []string{"Album"}, MBMediaFormats: []string{"CD"}},
	}}

	types, formats := releaseImportFilters(&config.ManualConfig{}, cfgp)
	if !slices.Equal(types, []string{"Album"}) || !slices.Equal(formats, []string{"CD"}) {
		t.Errorf("from media config: types %v, formats %v", types, formats)
	}

	_, formats = releaseImportFilters(&config.ManualConfig{MBMediaFormats: []string{"Vinyl"}}, cfgp)
	if !slices.Equal(formats, []string{"Vinyl"}) {
		t.Errorf("entry formats must win, got %v", formats)
	}

	types, formats = releaseImportFilters(&config.ManualConfig{}, &config.MediaTypeConfig{})
	if types != nil || formats != nil {
		t.Errorf("no config: types %v, formats %v, want none", types, formats)
	}
}

func TestImportAlbumsByArtistOneEditionPerAlbum(t *testing.T) {
	useTestDB(t)

	// Four editions of two albums: G1 as vinyl and twice as CD, G2 once as CD.
	p, seen := fakeMusicBrainz(t, func(path string) string {
		if path != "/release" {
			return ""
		}

		edition := func(id, group, format string) string {
			return `{"id":"` + id + `","title":"Album ` + group + `","release-group":{"id":"` + group +
				`","primary-type":"Album"},"media":[{"format":"` + format + `"}]}`
		}

		return `{"releases":[` + strings.Join([]string{
			edition("vinyl1", "G1", "Vinyl"),
			edition("cd1", "G1", "CD"),
			edition("cd2", "G1", "CD"),
			edition("cd3", "G2", "CD"),
		}, ",") + `]}`
	})

	cfgp := &config.MediaTypeConfig{Data: []config.MediaDataConfig{{MBMediaFormats: []string{"CD"}}}}
	album := config.ManualConfig{ArtistName: "Artist", ArtistID: "a1"}

	if err := importAlbumsByArtist(context.Background(), &album, cfgp, -1, p); err != nil {
		t.Fatal(err)
	}

	var details []string

	for _, uri := range seen() {
		if rest, ok := strings.CutPrefix(uri, "/release/"); ok {
			details = append(details, strings.SplitN(rest, "?", 2)[0])
		}
	}

	// The vinyl edition is filtered, cd1 stands for G1 (the vinyl edition
	// must not have claimed the group), cd2 is skipped, cd3 stands for G2.
	if !slices.Equal(details, []string{"cd1", "cd3"}) {
		t.Errorf("editions imported = %v, want [cd1 cd3]", details)
	}
}
