package downloader

import (
	"errors"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series"
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
