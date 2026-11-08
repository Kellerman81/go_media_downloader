package providers

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/acoustid"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deezer"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discogs"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/itunes"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lastfm"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/omdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/theaudiodb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvmaze"
)

func TestDownloadProviderRegistry(t *testing.T) {
	ReplaceDownloadProviders(nil)
	t.Cleanup(func() { ReplaceDownloadProviders(nil) })

	var nilDeluge *deluge.Provider

	kept := transmission.NewProvider("http://h2:9091", "", "")
	ReplaceDownloadProviders(map[string]any{
		"main":  deluge.NewProvider("http://h:8112", "", ""),
		"other": kept,
		"nil":   nilDeluge,
	})

	all := GetAllDownloadProviders()
	if _, ok := all["deluge_main"]; !ok || len(all) != 2 {
		t.Errorf("listed %v, want deluge_main and transmission_other", all)
	}

	if GetDownloadClient("other") != kept || GetDownloadClient("nil") != nil {
		t.Error("lookup by downloader name wrong")
	}

	ReplaceDownloadProviders(map[string]any{"other": kept})

	if all = GetAllDownloadProviders(); len(all) != 1 || all["transmission_other"] != kept {
		t.Errorf("ReplaceDownloadProviders left %v, want only transmission_other", all)
	}
}

// clearMetadataProviders resets every global metadata provider for the test
// and restores the previous ones afterwards.
func clearMetadataProviders(t *testing.T) {
	t.Helper()

	registryMutex.Lock()
	saved := []any{
		omdbProvider, tmdbProvider, traktProvider, tvdbProvider, tvmazeProvider, audnexProvider,
		openlibraryProvider, goodreadsProvider, musicbrainzProvider, acoustidProvider, lastfmProvider,
		discogsProvider, deezerProvider, theaudiodbProvider, itunesProvider,
	}
	savedAudible := audibleProviders
	audibleProviders = map[string]*audible.Provider{}
	omdbProvider, tmdbProvider, traktProvider, tvdbProvider, tvmazeProvider = nil, nil, nil, nil, nil
	audnexProvider, openlibraryProvider, goodreadsProvider, musicbrainzProvider = nil, nil, nil, nil
	acoustidProvider, lastfmProvider, discogsProvider, deezerProvider = nil, nil, nil, nil
	theaudiodbProvider, itunesProvider = nil, nil
	registryMutex.Unlock()

	t.Cleanup(func() {
		registryMutex.Lock()
		defer registryMutex.Unlock()

		omdbProvider, _ = saved[0].(*omdb.Provider)
		tmdbProvider, _ = saved[1].(*tmdb.Provider)
		traktProvider, _ = saved[2].(*trakt.Provider)
		tvdbProvider, _ = saved[3].(*tvdb.Provider)
		tvmazeProvider, _ = saved[4].(*tvmaze.Provider)
		audnexProvider, _ = saved[5].(*audnex.Provider)
		openlibraryProvider, _ = saved[6].(*openlibrary.Provider)
		goodreadsProvider, _ = saved[7].(*goodreads.Provider)
		musicbrainzProvider, _ = saved[8].(*musicbrainz.Provider)
		acoustidProvider, _ = saved[9].(*acoustid.Provider)
		lastfmProvider, _ = saved[10].(*lastfm.Provider)
		discogsProvider, _ = saved[11].(*discogs.Provider)
		deezerProvider, _ = saved[12].(*deezer.Provider)
		theaudiodbProvider, _ = saved[13].(*theaudiodb.Provider)
		itunesProvider, _ = saved[14].(*itunes.Provider)
		audibleProviders = savedAudible
	})
}

func TestMetadataProviderRegistry(t *testing.T) {
	clearMetadataProviders(t)

	if len(GetAllMetadataProviders()) != 0 {
		t.Fatalf("registry not empty: %v", GetAllMetadataProviders())
	}

	om, tm, tr, tv, mz := &omdb.Provider{}, &tmdb.Provider{}, &trakt.Provider{}, &tvdb.Provider{}, &tvmaze.Provider{}
	an, ol, gr, mb := &audnex.Provider{}, &openlibrary.Provider{}, &goodreads.Provider{}, &musicbrainz.Provider{}
	ac, lf, dc, dz, ta, it := &acoustid.Provider{}, &lastfm.Provider{}, &discogs.Provider{}, &deezer.Provider{}, &theaudiodb.Provider{}, &itunes.Provider{}
	au := &audible.Provider{}

	SetOMDB(om)
	SetTMDB(tm)
	SetTrakt(tr)
	SetTVDB(tv)
	SetTVMaze(mz)
	SetAudnex(an)
	SetOpenLibrary(ol)
	SetGoodreads(gr)
	SetMusicBrainz(mb)
	SetAcoustID(ac)
	SetLastFM(lf)
	SetDiscogs(dc)
	SetDeezer(dz)
	SetTheAudioDB(ta)
	SetITunes(it)
	SetAudible("de", au)
	SetAudible("uk", nil)

	if GetOMDB() != om || GetTMDB() != tm || GetTrakt() != tr || GetTVDB() != tv || GetTVMaze() != mz ||
		GetAudnex() != an || GetOpenLibrary() != ol || GetGoodreads() != gr || GetMusicBrainz() != mb ||
		GetAcoustID() != ac || GetLastFM() != lf || GetDiscogs() != dc || GetDeezer() != dz ||
		GetTheAudioDB() != ta || GetITunes() != it || GetAudible("de") != au {
		t.Error("a getter returned a different provider than was set")
	}

	all := GetAllMetadataProviders()
	for _, key := range []string{
		"omdb", "tmdb", "trakt", "tvdb", "tvmaze", "audnex", "openlibrary", "goodreads", "musicbrainz",
		"acoustid", "lastfm", "discogs", "deezer", "theaudiodb", "itunes", "audible_de",
	} {
		if all[key] == nil {
			t.Errorf("GetAllMetadataProviders is missing %q", key)
		}
	}

	if _, ok := all["audible_uk"]; ok || len(all) != 16 {
		t.Errorf("nil audible listed or wrong count: %d entries", len(all))
	}

	if aud := GetAllAudible(); len(aud) != 2 || aud["de"] != au {
		t.Errorf("GetAllAudible = %v", aud)
	}

	SetTMDB(nil)

	if _, ok := GetAllMetadataProviders()["tmdb"]; ok {
		t.Error("cleared provider still listed")
	}
}

func TestIndexerRegistry(t *testing.T) {
	registryMutex.Lock()
	saved := indexerProviders
	indexerProviders = map[string]*newznab.Provider{}
	registryMutex.Unlock()

	t.Cleanup(func() {
		registryMutex.Lock()
		indexerProviders = saved
		registryMutex.Unlock()
	})

	p := &newznab.Provider{}
	SetIndexer("geek", p)
	SetIndexer("slug", &newznab.Provider{})

	if GetIndexer("geek") != p || len(GetAllIndexers()) != 2 {
		t.Errorf("indexers = %v", GetAllIndexers())
	}

	cp := GetAllIndexers()
	delete(cp, "geek")

	if GetIndexer("geek") == nil {
		t.Error("GetAllIndexers returned the live map")
	}

	SetIndexer("slug", nil)
	RemoveIndexer("geek")

	if len(GetAllIndexers()) != 0 {
		t.Errorf("indexers after removal = %v", GetAllIndexers())
	}
}

func resetSharedProviders(t *testing.T) {
	t.Helper()

	registryMutex.Lock()
	prevMB, prevFB, prevOL := musicbrainzProvider, musicbrainzFallback, openlibraryProvider
	prevAud := audibleProviders
	musicbrainzProvider, musicbrainzFallback, openlibraryProvider = nil, nil, nil
	audibleProviders = make(map[string]*audible.Provider)
	registryMutex.Unlock()

	t.Cleanup(func() {
		registryMutex.Lock()
		musicbrainzProvider, musicbrainzFallback, openlibraryProvider = prevMB, prevFB, prevOL
		audibleProviders = prevAud
		registryMutex.Unlock()
	})
}

func TestSharedMusicBrainz(t *testing.T) {
	resetSharedProviders(t)

	fb := SharedMusicBrainz()
	if fb == nil || SharedMusicBrainz() != fb {
		t.Fatal("fallback is not a single shared instance")
	}

	if GetMusicBrainz() != nil {
		t.Error("fallback must not make MusicBrainz look enabled via GetMusicBrainz")
	}

	reg := musicbrainz.NewProvider()
	SetMusicBrainz(reg)

	if SharedMusicBrainz() != reg {
		t.Error("registered provider must win over the fallback")
	}
}

func TestSharedOpenLibrary(t *testing.T) {
	resetSharedProviders(t)

	p := SharedOpenLibrary()
	if p == nil || SharedOpenLibrary() != p || GetOpenLibrary() != p {
		t.Fatal("fallback not stored in the registry")
	}

	reg := openlibrary.NewProvider()
	SetOpenLibrary(reg)

	if SharedOpenLibrary() != reg {
		t.Error("registered provider not returned")
	}
}

func TestSharedAudible(t *testing.T) {
	resetSharedProviders(t)

	us := SharedAudible("")
	if us == nil || SharedAudible("us") != us || GetAudible("us") != us {
		t.Fatal("empty region must map to the shared us instance")
	}

	de := SharedAudible("de")
	if de == us || SharedAudible("de") != de {
		t.Error("regions must have separate, shared instances")
	}
}

func TestSharedProvidersConcurrent(t *testing.T) {
	resetSharedProviders(t)

	const n = 16

	got := make(chan *musicbrainz.Provider, n)
	for range n {
		go func() { got <- SharedMusicBrainz() }()
	}

	first := <-got
	for range n - 1 {
		if <-got != first {
			t.Fatal("concurrent first use created more than one instance")
		}
	}
}
