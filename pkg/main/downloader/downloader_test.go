package downloader

import (
	"context"
	"errors"
	"maps"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

func TestDownloadNzbGuardsMissingConfig(t *testing.T) {
	tests := []struct {
		name    string
		quality *config.QualityConfig
		want    error
	}{
		{"nil quality", nil, errNoQualityIndexer},
		{"no indexers", &config.QualityConfig{Name: "q"}, errNoQualityIndexer},
		{
			"nil nzb indexer falls back to first entry",
			&config.QualityConfig{
				Name:    "q",
				Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "x"}},
			},
			logger.ErrPathTemplateNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &downloadertype{
				Cfgp:    &config.MediaTypeConfig{},
				Nzb:     &apiexternal_v2.Nzbwithprio{},
				Quality: tt.quality,
			}

			if err := d.downloadNzb(); !errors.Is(err, tt.want) {
				t.Fatalf("downloadNzb() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSavePath(t *testing.T) {
	tests := []struct {
		dlType, dlTo, want string
	}{
		{"sabnzbd", "/torrents", "/target"},
		{"qbittorrent", "/torrents", "/torrents"},
		{"deluge", "", "/target"},
		{"transmission", "/t", "/t"},
		{"aria2", "/a", "/a"},
		{"nzbvortex", "/a", "/target"},
	}

	for _, tt := range tests {
		d := &downloadertype{
			TargetCfg:     &config.PathsConfig{Path: "/target"},
			DownloaderCfg: &config.DownloaderConfig{DlType: tt.dlType, DelugeDlTo: tt.dlTo},
		}
		if got := d.savePath(); got != tt.want {
			t.Errorf("%s/%q: savePath() = %q, want %q", tt.dlType, tt.dlTo, got, tt.want)
		}
	}
}

func TestGetdownloadtargetfolder(t *testing.T) {
	nzb := func(title, infoTitle string) *apiexternal_v2.Nzbwithprio {
		n := &apiexternal_v2.Nzbwithprio{}
		n.NZB.Title = title
		n.Info.Title = infoTitle
		n.Info.Resolution = "1080p"
		n.Info.Quality = "WEB"

		return n
	}

	movie := &downloadertype{Cfgp: &config.MediaTypeConfig{IsType: config.MediaTypeMovie}, Nzb: nzb("Heat.1995.1080p", "")}
	movie.Dbmovie.ImdbID = "tt0113277"

	if got := movie.getdownloadtargetfolder(); got != "Heat.1995.1080p (tt0113277)" {
		t.Errorf("movie with db imdb = %q", got)
	}

	movie.Dbmovie.ImdbID = ""
	movie.Nzb.NZB.IMDBID = "0113277"

	if got := movie.getdownloadtargetfolder(); got != "Heat.1995.1080p (tt0113277)" {
		t.Errorf("movie with nzb imdb = %q", got)
	}

	serie := &downloadertype{Cfgp: &config.MediaTypeConfig{IsType: config.MediaTypeSeries}, Nzb: nzb("Dark.S01E01", "")}
	serie.Dbserie.ThetvdbID = 334824

	if got := serie.getdownloadtargetfolder(); got != "Dark.S01E01 (tvdb334824)" {
		t.Errorf("series = %q", got)
	}

	book := &downloadertype{Cfgp: &config.MediaTypeConfig{IsType: config.MediaTypeAudiobook}, Nzb: nzb("", "Mythos")}

	if got := book.getdownloadtargetfolder(); got != "Mythos" {
		t.Errorf("audiobook = %q", got)
	}

	// No IDs and no handler-specific result: title, or info title with quality.
	plain := &downloadertype{Cfgp: &config.MediaTypeConfig{IsType: config.MediaTypeMovie}, Nzb: nzb("", "Heat")}

	if got := plain.getdownloadtargetfolder(); got != "Heat[1080p WEB]" {
		t.Errorf("fallback = %q", got)
	}

	unknown := &downloadertype{Cfgp: &config.MediaTypeConfig{IsType: 9999}, Nzb: nzb("Some.Release", "")}

	if got := unknown.getdownloadtargetfolder(); got != "Some.Release" {
		t.Errorf("unregistered type = %q", got)
	}
}

type fakeTorrentClient struct {
	got apiexternal_v2.TorrentAddRequest
}

func (f *fakeTorrentClient) AddTorrent(
	_ context.Context,
	req apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	f.got = req
	return &apiexternal_v2.TorrentAddResponse{Success: true}, nil
}

type fakeNZBClient struct{ url, category, name string }

func (f *fakeNZBClient) AddNZBURL(_ context.Context, nzbURL, category, name string, _ int, _ bool) (string, error) {
	f.url, f.category, f.name = nzbURL, category, name
	return "uuid-1", nil
}

func TestDownloadByClient(t *testing.T) {
	torrent, nzb := &fakeTorrentClient{}, &fakeNZBClient{}

	providers.ReplaceDownloadProviders(map[string]any{"fl": torrent, "vortex": nzb})
	t.Cleanup(func() { providers.ReplaceDownloadProviders(nil) })

	newD := func(name, dlType string) *downloadertype {
		return &downloadertype{
			Nzb:           &apiexternal_v2.Nzbwithprio{NZB: apiexternal_v2.Nzb{DownloadURL: "http://i/get?a=1&amp;b=2"}},
			DownloaderCfg: &config.DownloaderConfig{Name: name, DlType: dlType, DelugeDlTo: "/dl", AddPaused: true},
			Category:      "movies",
			Targetfile:    "Heat",
		}
	}

	if id, err := newD("fl", "flood").downloadByClient(); err != nil || id != "" {
		t.Fatalf("flood add: id %q, err %v", id, err)
	}

	if g := torrent.got; g.URL != "http://i/get?a=1&b=2" || g.SavePath != "/dl" || g.Category != "movies" || !g.Paused {
		t.Errorf("torrent request %+v", g)
	}

	if id, err := newD("vortex", "nzbvortex").downloadByClient(); err != nil || id != "uuid-1" {
		t.Fatalf("nzbvortex add: id %q, err %v", id, err)
	}

	if nzb.url != "http://i/get?a=1&b=2" || nzb.category != "movies" || nzb.name != "Heat" {
		t.Errorf("nzb add %+v", nzb)
	}

	if _, err := newD("missing", "flood").downloadByClient(); !errors.Is(err, errNoDownloadClient) {
		t.Errorf("unregistered client: %v", err)
	}

	if _, err := newD("x", "bogus").downloadByClient(); !errors.Is(err, errUnknownDownloader) {
		t.Errorf("unknown type: %v", err)
	}
}

// listingClient adds a torrent without returning its hash, like qBittorrent
// for a torrent URL; it shows up in the next listing.
type listingClient struct {
	states map[string]apiexternal_v2.DownloadState
}

func (l *listingClient) DownloadStates(context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	out := make(map[string]apiexternal_v2.DownloadState, len(l.states))
	maps.Copy(out, l.states)

	return out, nil
}

func (l *listingClient) AddTorrent(
	_ context.Context,
	_ apiexternal_v2.TorrentAddRequest,
) (*apiexternal_v2.TorrentAddResponse, error) {
	l.states["NEWHASH"] = apiexternal_v2.DownloadState{Name: "Heat.1995.1080p"}
	return &apiexternal_v2.TorrentAddResponse{Success: true}, nil
}

func TestFindNewDownload(t *testing.T) {
	c := &listingClient{states: map[string]apiexternal_v2.DownloadState{"OLD": {Name: "Other"}}}

	providers.ReplaceDownloadProviders(map[string]any{"fl": c})
	t.Cleanup(func() { providers.ReplaceDownloadProviders(nil) })

	d := &downloadertype{
		Nzb:           &apiexternal_v2.Nzbwithprio{NZB: apiexternal_v2.Nzb{Title: "Heat.1995.1080p", DownloadURL: "http://i/get/1.torrent"}},
		DownloaderCfg: &config.DownloaderConfig{Name: "fl", DlType: "flood"},
	}

	before := d.snapshotDownloads()
	if len(before) != 1 {
		t.Fatalf("snapshot %v", before)
	}

	id, err := d.downloadByClient()
	if err != nil || id != "" {
		t.Fatalf("add: %q %v", id, err)
	}

	if got := d.findNewDownload(before); got != "NEWHASH" {
		t.Errorf("found %q, want NEWHASH", got)
	}

	d.Nzb.NZB.DownloadURL = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567"
	if d.snapshotDownloads() != nil {
		t.Error("a magnet carries its hash and needs no snapshot")
	}
}
