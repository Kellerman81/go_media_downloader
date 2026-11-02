package config

import (
	"context"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestValidation(t *testing.T) {
	music := &MainConfig{
		Paths: []PathsConfig{{Name: "p"}},
		Media: MediaConfig{Music: []MediaTypeConfig{{
			Name: "Mu",
			Data: []MediaDataConfig{{TemplatePath: "p"}},
		}}},
	}
	if snap, err := buildConfigSnapshot(music); err != nil || len(snap.issues) != 0 {
		t.Errorf("music-only config without indexers/quality rejected: %v %v", err, snap)
	}

	// Checks introduced with issues are non-fatal when building from disk.
	issueCases := map[string]func(*MainConfig){
		"duplicate quality": func(c *MainConfig) { c.Quality = append(c.Quality, QualityConfig{Name: "hd"}) },
		"duplicate media":   func(c *MainConfig) { c.Media.Music = append(c.Media.Music, testMedia("MU", "HD")) },
		"duplicate media list": func(c *MainConfig) {
			c.Media.Movies[0].Lists = append(c.Media.Movies[0].Lists, MediaListsConfig{Name: "m-LIST"})
		},
		"empty scheduler name": func(c *MainConfig) { c.Scheduler = append(c.Scheduler, SchedulerConfig{}) },
		"book without data":    func(c *MainConfig) { c.Media.Books[0].Data = nil },
		"worker search 101":    func(c *MainConfig) { c.General.WorkerSearch = 101 },
		"dangling reference":   func(c *MainConfig) { c.Media.Series[0].TemplateQuality = "missing" },
	}
	for name, mutate := range issueCases {
		cfg := testMainConfig()
		mutate(cfg)

		snap, err := buildConfigSnapshot(cfg)
		if err != nil || len(snap.issues) == 0 {
			t.Errorf("%s: want non-fatal issue, got err=%v issues=%v", name, err, snap)
		}
	}

	// Checks that were fatal before stay fatal.
	fatalCases := map[string]func(*MainConfig){
		"no media":           func(c *MainConfig) { c.Media = MediaConfig{} },
		"movie without data": func(c *MainConfig) { c.Media.Movies[0].Data = nil },
		"unnamed quality":    func(c *MainConfig) { c.Quality = append(c.Quality, QualityConfig{}) },
		"indexer without url": func(c *MainConfig) {
			c.Indexers = append(c.Indexers, IndexersConfig{Name: "x"})
		},
	}
	for name, mutate := range fatalCases {
		cfg := testMainConfig()
		mutate(cfg)

		if _, err := buildConfigSnapshot(cfg); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}

	cfg := testMainConfig()
	cfg.General.WorkerParse = 0

	snap, err := buildConfigSnapshot(cfg)
	if err != nil || snap.General.WorkerParse != 1 {
		t.Errorf("worker_parse 0 should auto-correct to 1: err=%v", err)
	}
}

func TestTemplateResolutionCaseInsensitive(t *testing.T) {
	cfg := testMainConfig()
	cfg.Media.Movies[0].TemplateQuality = "hd"
	cfg.Media.Movies[0].Data[0].TemplatePath = "ALPHA"
	cfg.Media.Movies[0].Lists[0].TemplateScheduler = "other"
	cfg.Media.Series[0].TemplateQuality = "missing"

	snap, err := buildConfigSnapshot(cfg)
	if err != nil {
		t.Fatal(err)
	}

	m := snap.Media["movie_M"]
	if m.CfgQuality == nil || m.CfgQuality.Name != "HD" {
		t.Errorf("quality 'hd' did not resolve to 'HD': %+v", m.CfgQuality)
	}

	if m.Data[0].CfgPath == nil || m.Data[0].CfgPath.Name != "alpha" {
		t.Errorf("path 'ALPHA' did not resolve")
	}

	if m.Lists[0].CfgScheduler == nil || m.Lists[0].CfgScheduler.Name != "Other" {
		t.Errorf("list scheduler 'other' did not resolve")
	}

	if snap.Media["serie_S"].CfgQuality != nil {
		t.Errorf("dangling reference resolved to something")
	}
}

func TestJobsSurviveRebuild(t *testing.T) {
	useTempConfig(t, testMainConfig())

	GetSettingsGeneral().Jobs = map[string]func(uint32, context.Context) error{"x": nil}
	GetSettingsMedia("music_Mu").Jobs = map[string]func(uint32, context.Context) error{"y": nil}

	if err := UpdateCfgEntry(Conf{Name: "path_mid", Data: PathsConfig{Name: "mid", MinSize: 1}}); err != nil {
		t.Fatal(err)
	}

	if _, ok := GetSettingsGeneral().Jobs["x"]; !ok {
		t.Error("general jobs dropped by rebuild")
	}

	if _, ok := GetSettingsMedia("music_Mu").Jobs["y"]; !ok {
		t.Error("media jobs dropped by rebuild")
	}
}

func TestLegacyImdbKeysStillLoad(t *testing.T) {
	var cfg MainConfig
	if err := toml.Unmarshal([]byte("[imdbindexer]\nloop_size = 400000\nuse_cache = true\nimdbid_size = 5\n"), &cfg); err != nil {
		t.Fatalf("old config with removed keys failed to decode: %v", err)
	}

	if cfg.Imdbindexer.ImdbIDSize != 5 {
		t.Errorf("known key lost: %+v", cfg.Imdbindexer)
	}
}

func TestQualityIndexerLookups(t *testing.T) {
	rgx := &RegexConfig{Name: "strict"}
	q := &QualityConfig{Indexer: []QualityIndexerConfig{
		{TemplateIndexer: "other"},
		{TemplateIndexer: "NZBgeek", CfgRegex: rgx, HistoryCheckTitle: true, SkipEmptySize: true},
	}}

	ind := &IndexersConfig{Name: "nzbgeek"}
	missing := &IndexersConfig{Name: "drunken"}

	if got := q.QualityIndexerByQualityAndTemplate(ind); got != 1 {
		t.Errorf("index = %d, want 1 (case-insensitive)", got)
	}

	if q.QualityIndexerByQualityAndTemplateCheckRegex(ind) != rgx || !q.QualityIndexerByQualityAndTemplateCheckTitle(ind) ||
		!q.QualityIndexerByQualityAndTemplateSkipEmpty(ind) {
		t.Error("matched indexer settings not returned")
	}

	for _, i := range []*IndexersConfig{missing, nil} {
		if q.QualityIndexerByQualityAndTemplate(i) != -1 || q.QualityIndexerByQualityAndTemplateCheckRegex(i) != nil ||
			q.QualityIndexerByQualityAndTemplateCheckTitle(i) || q.QualityIndexerByQualityAndTemplateSkipEmpty(i) {
			t.Errorf("no-match lookups for %+v returned a value", i)
		}
	}
}
