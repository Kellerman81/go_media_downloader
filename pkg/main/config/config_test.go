package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/pelletier/go-toml/v2"
)

func testMedia(name, quality string) MediaTypeConfig {
	return MediaTypeConfig{
		Name:              name,
		TemplateQuality:   quality,
		TemplateScheduler: "Default",
		Data:              []MediaDataConfig{{TemplatePath: "alpha"}},
		Lists: []MediaListsConfig{{
			Name:            name + "-list",
			TemplateList:    "listA",
			TemplateQuality: quality,
		}},
	}
}

func testMainConfig() *MainConfig {
	return &MainConfig{
		General:    GeneralConfig{WorkerFiles: 1, WorkerParse: 1, WorkerSearch: 1},
		Downloader: []DownloaderConfig{{Name: "dl2"}, {Name: "dl1"}},
		Indexers:   []IndexersConfig{{Name: "idx", URL: "http://x"}},
		Lists:      []ListsConfig{{Name: "listA"}},
		Paths:      []PathsConfig{{Name: "zeta"}, {Name: "alpha"}, {Name: "mid"}},
		Quality: []QualityConfig{{
			Name:    "HD",
			Indexer: []QualityIndexerConfig{{TemplateIndexer: "idx", TemplateDownloader: "dl1"}},
		}},
		Regex:        []RegexConfig{{Name: "r1"}},
		Notification: []NotificationConfig{{Name: "n1"}},
		Scheduler:    []SchedulerConfig{{Name: "Default", IntervalImdb: "3d"}, {Name: "Other"}},
		Media: MediaConfig{
			Movies:     []MediaTypeConfig{testMedia("M", "HD")},
			Series:     []MediaTypeConfig{testMedia("S", "HD")},
			Books:      []MediaTypeConfig{testMedia("B", "HD")},
			AudioBooks: []MediaTypeConfig{testMedia("A", "HD")},
			Music:      []MediaTypeConfig{testMedia("Mu", "HD")},
		},
	}
}

// useTempConfig writes cfg to a temp config file, loads it, and restores the
// previous global state afterwards.
func useTempConfig(t *testing.T, cfg *MainConfig) string {
	t.Helper()

	oldFile := Configfile
	oldSnap := getCurrentConfig()

	Configfile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() {
		Configfile = oldFile
		if oldSnap != nil {
			configSnapshot.Store(oldSnap)
		}
	})

	cnt, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(Configfile, cnt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Loadallsettings(false); err != nil {
		t.Fatalf("load: %v", err)
	}

	return Configfile
}

func names[T any](items []T, name func(*T) string) string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = name(&items[i])
	}

	return strings.Join(out, ",")
}

func TestWriteCfgRoundTripPreservesAllSectionsAndOrder(t *testing.T) {
	useTempConfig(t, testMainConfig())

	// Simulate a UI save: update one entry, then WriteCfg.
	if err := UpdateCfgEntry(Conf{Name: "path_mid", Data: PathsConfig{Name: "mid", MinSize: 5}}); err != nil {
		t.Fatal(err)
	}

	if err := WriteCfg(); err != nil {
		t.Fatal(err)
	}

	got, err := Readconfigtoml()
	if err != nil {
		t.Fatal(err)
	}

	mn := func(c *MediaTypeConfig) string { return c.Name }
	checks := map[string][2]string{
		"movies":     {names(got.Media.Movies, mn), "M"},
		"series":     {names(got.Media.Series, mn), "S"},
		"books":      {names(got.Media.Books, mn), "B"},
		"audiobooks": {names(got.Media.AudioBooks, mn), "A"},
		"music":      {names(got.Media.Music, mn), "Mu"},
		"paths":      {names(got.Paths, func(c *PathsConfig) string { return c.Name }), "zeta,alpha,mid"},
		"downloader": {names(got.Downloader, func(c *DownloaderConfig) string { return c.Name }), "dl2,dl1"},
		"scheduler":  {names(got.Scheduler, func(c *SchedulerConfig) string { return c.Name }), "Default,Other"},
	}
	for section, c := range checks {
		if c[0] != c[1] {
			t.Errorf("%s: got %q, want %q", section, c[0], c[1])
		}
	}

	if got.Paths[2].MinSize != 5 {
		t.Errorf("updated path not persisted: %+v", got.Paths[2])
	}

	if opts := GetSettingTemplatesFor("path")["options"]; strings.Join(opts, ",") != ",zeta,alpha,mid" {
		t.Errorf("template options not in file order: %v", opts)
	}
}

func TestUpdateDeleteBookAudiobookMusic(t *testing.T) {
	useTempConfig(t, testMainConfig())

	for _, prefix := range []string{"book_", "audiobook_", "music_"} {
		key := prefix + "New"
		if err := UpdateCfgEntry(Conf{Name: key, Data: testMedia("New", "HD")}); err != nil {
			t.Fatalf("%s update: %v", key, err)
		}

		cfg := GetSettingsMedia(key)
		if cfg == nil || cfg.CfgQuality == nil {
			t.Fatalf("%s missing after update: %+v", key, cfg)
		}

		if err := DeleteCfgEntry(key); err != nil {
			t.Fatalf("%s delete: %v", key, err)
		}

		if GetSettingsMedia(key) != nil {
			t.Fatalf("%s still present after delete", key)
		}
	}

	file, _ := Readconfigtoml()
	if len(file.Media.Books) != 1 || len(file.Media.AudioBooks) != 1 || len(file.Media.Music) != 1 {
		t.Errorf("file media counts wrong: %d %d %d",
			len(file.Media.Books), len(file.Media.AudioBooks), len(file.Media.Music))
	}

	if err := UpdateCfgEntry(Conf{Name: "bogus_x", Data: PathsConfig{Name: "x"}}); err == nil {
		t.Error("unknown prefix: expected error")
	}

	if err := UpdateCfgEntry(Conf{Name: "path_x", Data: QualityConfig{Name: "x"}}); err == nil {
		t.Error("wrong data type: expected error")
	}

	if err := DeleteCfgEntry("music_nope"); !errors.Is(err, logger.ErrNotFound) {
		t.Errorf("delete missing: got %v, want ErrNotFound", err)
	}

	if err := UpdateCfgEntryAny(42); err == nil {
		t.Error("UpdateCfgEntryAny unsupported type: expected error")
	}
}

func TestUpdateRejectsOnlyIntroducedIssues(t *testing.T) {
	cfg := testMainConfig()
	cfg.Media.Movies[0].Lists = append(cfg.Media.Movies[0].Lists, MediaListsConfig{TemplateList: "listA"})
	useTempConfig(t, cfg)

	// Unrelated entry saves fine despite the pre-existing unnamed list.
	if err := UpdateCfgEntry(Conf{Name: "path_mid", Data: PathsConfig{Name: "mid", MinSize: 3}}); err != nil {
		t.Fatalf("unrelated update blocked by pre-existing problem: %v", err)
	}

	// Re-saving the media config that already has the unnamed list is fine too.
	if err := UpdateCfgEntry(Conf{Name: "movie_M", Data: GetToml().Media.Movies[0]}); err != nil {
		t.Fatalf("re-save of entry with pre-existing problem blocked: %v", err)
	}

	snap := getCurrentConfig()

	dup := testMedia("B", "HD")
	dup.Lists = append(dup.Lists, MediaListsConfig{Name: "b-LIST"})

	empty := testMedia("B", "HD")
	empty.Lists = append(empty.Lists, MediaListsConfig{})

	rejects := map[string]Conf{
		"duplicate list name": {Name: "book_B", Data: dup},
		"empty list name":     {Name: "book_B", Data: empty},
		"empty scheduler":     {Name: "scheduler_", Data: SchedulerConfig{}},
		"dangling quality":    {Name: "music_Mu", Data: testMedia("Mu", "nope")},
	}
	for name, c := range rejects {
		if err := UpdateCfgEntry(c); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}

	if err := UpdateCfgEntryAny(append(GetToml().Regex, RegexConfig{Name: "R1"})); err == nil {
		t.Error("section save introducing a duplicate: expected rejection")
	}

	if getCurrentConfig() != snap {
		t.Error("rejected update replaced the snapshot")
	}
}

func TestConcurrentUpdatesAndReads(t *testing.T) {
	useTempConfig(t, testMainConfig())

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for range 4 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-stop:
					return
				default:
				}

				for _, p := range GetSettingsPathAll() {
					_ = p.MinSize
				}

				if m := GetSettingsMedia("movie_M"); m != nil && m.Data[0].CfgPath != nil {
					_ = m.Data[0].CfgPath.MinSize
				}

				_ = GetToml()
			}
		}()
	}

	for i := range 20 {
		if err := UpdateCfgEntry(Conf{Name: "path_alpha", Data: PathsConfig{Name: "alpha", MinSize: i}}); err != nil {
			t.Error(err)
		}

		if i%5 == 0 {
			if err := Loadallsettings(true); err != nil {
				t.Error(err)
			}
		}
	}

	close(stop)
	wg.Wait()
}

// useSnapshot installs snap as the current config for the test.
func useSnapshot(t *testing.T, snap *ConfigSnapshot) {
	t.Helper()

	old := getCurrentConfig()
	configSnapshot.Store(snap)

	t.Cleanup(func() {
		if old != nil {
			configSnapshot.Store(old)
		} else {
			configSnapshot.Store((*ConfigSnapshot)(nil))
		}
	})
}

func TestCheckGroup(t *testing.T) {
	useSnapshot(t, &ConfigSnapshot{
		General:      &GeneralConfig{},
		Quality:      map[string]*QualityConfig{"SD": {}},
		Path:         map[string]*PathsConfig{"movies": {}},
		Media:        map[string]*MediaTypeConfig{"movie_EN": {}},
		List:         map[string]*ListsConfig{"imdb": {}},
		Indexer:      map[string]*IndexersConfig{"nzbgeek": {}},
		Regex:        map[string]*RegexConfig{"default": {}},
		Notification: map[string]*NotificationConfig{"pushover": {}},
		Downloader:   map[string]*DownloaderConfig{"sab": {}},
		Scheduler:    map[string]*SchedulerConfig{"default": {}},
	})

	tests := []struct {
		prefix, name string
		want         bool
	}{
		{"quality_", "SD", true},
		{"path_", "movies", true},
		{"media_", "EN", false},
		{"list_", "imdb", true},
		{"indexer_", "nzbgeek", true},
		{"regex_", "default", true},
		{"notification_", "pushover", true},
		{"downloader_", "sab", true},
		{"scheduler_", "default", true},
		{"quality_", "HD", false},
		{"unknown_", "SD", false},
		{"quality_", "", false},
	}

	for _, tt := range tests {
		if got := CheckGroup(tt.prefix, tt.name); got != tt.want {
			t.Errorf("CheckGroup(%q, %q) = %t, want %t", tt.prefix, tt.name, got, tt.want)
		}
	}

	// Media keys include the prefix itself.
	useSnapshot(t, &ConfigSnapshot{Media: map[string]*MediaTypeConfig{"media_movie_EN": {}}})

	if !CheckGroup("media_", "movie_EN") {
		t.Error("media key with prefix not found")
	}
}

func TestCheckGroupWithoutConfig(t *testing.T) {
	useSnapshot(t, nil)

	if CheckGroup("quality_", "SD") {
		t.Error("CheckGroup without a config reported true")
	}
}

func TestGetCfgAllAndTemplates(t *testing.T) {
	snap := &ConfigSnapshot{
		General:      &GeneralConfig{},
		Imdb:         &ImdbConfig{},
		Media:        map[string]*MediaTypeConfig{"movie_EN": {NamePrefix: "movie_EN"}},
		Downloader:   map[string]*DownloaderConfig{"sab": {Name: "sab"}},
		Indexer:      map[string]*IndexersConfig{"geek": {}},
		List:         map[string]*ListsConfig{"imdb": {}},
		Notification: map[string]*NotificationConfig{"push": {}},
		Path:         map[string]*PathsConfig{"movies": {}},
		Quality:      map[string]*QualityConfig{"SD": {Name: "SD"}},
		Regex:        map[string]*RegexConfig{"def": {}},
		Scheduler:    map[string]*SchedulerConfig{"daily": {}},
	}
	snap.cachetoml.Quality = []QualityConfig{{Name: "SD"}, {Name: "HD"}}
	snap.cachetoml.Downloader = []DownloaderConfig{{Name: "sab"}}
	useSnapshot(t, snap)

	all := GetCfgAll()
	for _, key := range []string{
		"general", "imdb", "movie_EN", "downloader_sab", "indexer_geek", "list_imdb", "notification_push",
		"path_movies", "quality_SD", "regex_def", "scheduler_daily",
	} {
		if _, ok := all[key]; !ok {
			t.Errorf("GetCfgAll is missing %q", key)
		}
	}

	if len(GetCfgAllJson()) != len(all) {
		t.Error("GetCfgAllJson differs from GetCfgAll")
	}

	if got := GetSettingTemplatesFor("quality")["options"]; !slices.Equal(got, []string{"", "SD", "HD"}) {
		t.Errorf("quality templates = %v", got)
	}

	if GetSettingTemplatesFor("bogus") != nil {
		t.Error("unknown template type returned options")
	}

	if q, ok := GetSettingsQualityOk("SD"); !ok || q.Name != "SD" {
		t.Errorf("GetSettingsQualityOk(SD) = %v, %t", q, ok)
	}

	if _, ok := GetSettingsQualityOk("UHD"); ok {
		t.Error("missing quality reported ok")
	}

	if got := GetSettingsDownloaderAll(); len(got) != 1 || got[0].Name != "sab" {
		t.Errorf("GetSettingsDownloaderAll = %v", got)
	}

	counts := map[string]int{}
	RangeSettingsList(func(string, *ListsConfig) { counts["list"]++ })
	RangeSettingsIndexer(func(string, *IndexersConfig) { counts["indexer"]++ })
	RangeSettingsScheduler(func(string, *SchedulerConfig) { counts["scheduler"]++ })
	RangeSettingsNotification(func(string, *NotificationConfig) { counts["notification"]++ })
	RangeSettingsPath(func(string, *PathsConfig) { counts["path"]++ })
	RangeSettingsRegex(func(string, *RegexConfig) { counts["regex"]++ })
	RangeSettingsDownloader(func(string, *DownloaderConfig) { counts["downloader"]++ })

	for _, k := range []string{"list", "indexer", "scheduler", "notification", "path", "regex", "downloader"} {
		if counts[k] != 1 {
			t.Errorf("RangeSettings for %s visited %d entries", k, counts[k])
		}
	}
}

func TestSetEntry(t *testing.T) {
	var dst QualityConfig

	if err := setEntry(&dst, Conf{Name: "q", Data: QualityConfig{Name: "HD"}}); err != nil || dst.Name != "HD" {
		t.Errorf("value payload: %v %q", err, dst.Name)
	}

	if err := setEntry(&dst, Conf{Name: "q", Data: &QualityConfig{Name: "SD"}}); err != nil || dst.Name != "SD" {
		t.Errorf("pointer payload: %v %q", err, dst.Name)
	}

	if err := setEntry(&dst, Conf{Name: "q", Data: (*QualityConfig)(nil)}); err == nil {
		t.Error("nil pointer payload accepted")
	}

	if err := setEntry(&dst, Conf{Name: "q", Data: PathsConfig{}}); err == nil || dst.Name != "SD" {
		t.Errorf("wrong payload type: %v, dst changed to %q", err, dst.Name)
	}
}
