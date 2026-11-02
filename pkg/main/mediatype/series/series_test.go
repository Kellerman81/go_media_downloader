package series

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
)

func TestTrimStringInclAfterString(t *testing.T) {
	tests := []struct{ in, trim, want string }{
		{"Episode Title 720p WEB", "720p", "Episode Title "},
		{"Episode Title WEBDL", "webdl", "Episode Title "},
		{"Episode Title", "1080p", "Episode Title"},
		{"Episode Title", "", "Episode Title"},
	}

	for _, tt := range tests {
		if got := trimStringInclAfterString(tt.in, tt.trim); got != tt.want {
			t.Errorf("trimStringInclAfterString(%q, %q) = %q, want %q", tt.in, tt.trim, got, tt.want)
		}
	}
}

func TestListArgs(t *testing.T) {
	cfgp := &config.MediaTypeConfig{Lists: []config.MediaListsConfig{{Name: "a"}, {Name: "b"}}}

	args := listArgs(cfgp)
	if len(args) != 2 || *args[0].(*string) != "a" || *args[1].(*string) != "b" {
		t.Errorf("listArgs = %v", args)
	}
}

func TestSearchConfigByName(t *testing.T) {
	file := filepath.Join(t.TempDir(), "series.toml")

	err := os.WriteFile(file, []byte(`
[[config]]
name = "Breaking Bad"
tvdb_id = 81189
alternatename = ["BB", " Во все тяжкие "]

[[config]]
name = " Dark "
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}

	list := &config.MediaListsConfig{CfgList: &config.ListsConfig{ManualConfigFile: file}}

	for name, want := range map[string]int{"breaking bad": 81189, "bb": 81189, "во все тяжкие": 81189, "dark": 0} {
		cfg, ok := Handler.SearchConfigByName(name, list)
		if !ok || cfg.TvdbID != want {
			t.Errorf("SearchConfigByName(%q) = %+v, %t", name, cfg, ok)
		}
	}

	if _, ok := Handler.SearchConfigByName("Lost", list); ok {
		t.Error("unknown series found")
	}

	missing := &config.MediaListsConfig{CfgList: &config.ListsConfig{ManualConfigFile: filepath.Join(t.TempDir(), "none.toml")}}
	if _, ok := Handler.SearchConfigByName("Dark", missing); ok {
		t.Error("missing file reported a match")
	}

	if _, ok := Handler.SearchConfigByName("Dark", &config.MediaListsConfig{CfgList: &config.ListsConfig{}}); ok {
		t.Error("list without a manual config file reported a match")
	}
}

func TestMoveOtherFilesAfterOrganize(t *testing.T) {
	var calls []string

	err := Handler.MoveOtherFilesAfterOrganize(&mediatype.MoveOtherFilesParams{
		TargetPath:      "/tv/Show/Season 1",
		Filename:        "Show - S01E01",
		WalkCleanupFn:   func(_, target, name string) { calls = append(calls, "companions:"+target+"/"+name) },
		NotifyFn:        func() { calls = append(calls, "notify") },
		CleanupFolderFn: func() { calls = append(calls, "cleanup") },
	})
	if err != nil {
		t.Fatal(err)
	}

	// Companions are moved before the notification and the folder cleanup.
	want := []string{"companions:/tv/Show/Season 1/Show - S01E01", "notify", "cleanup"}
	if !slices.Equal(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
}

func TestCleanupAfterRemoveAndScheduler(t *testing.T) {
	removed := false
	if err := Handler.CleanupAfterRemove("", "", "", nil, func() { removed = true }); err != nil || !removed {
		t.Errorf("CleanupAfterRemove = %v, removed %t", err, removed)
	}

	sched := &config.SchedulerConfig{
		IntervalIndexerRssSeasons: "6h", CronIndexerRssSeasons: "0 */6 * * *",
		IntervalIndexerRssSeasonsAll: "1d", CronIndexerRssSeasonsAll: "0 3 * * *",
	}

	if i, c := Handler.GetSchedulerRssSeasons(sched, logger.StrRssSeasons); i != "6h" || c != "0 */6 * * *" {
		t.Errorf("rss seasons = %q %q", i, c)
	}

	if i, c := Handler.GetSchedulerRssSeasons(sched, logger.StrRssSeasonsAll); i != "1d" || c != "0 3 * * *" {
		t.Errorf("rss seasons all = %q %q", i, c)
	}

	if i, c := Handler.GetSchedulerRssSeasons(sched, "other"); i != "" || c != "" {
		t.Errorf("other job = %q %q", i, c)
	}

	if i, c := Handler.GetSchedulerRssArtistsAuthors(sched, logger.StrRssArtists); i != "" || c != "" {
		t.Errorf("series artist job = %q %q", i, c)
	}
}
