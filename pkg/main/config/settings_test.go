package config

import (
	"os"
	"slices"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestReloadAppliesSchedulerChanges(t *testing.T) {
	cfg := testMainConfig()
	file := useTempConfig(t, cfg)

	cfg.Scheduler = []SchedulerConfig{{Name: "Default", IntervalImdb: "5d"}, {Name: "New"}}

	cnt, _ := toml.Marshal(cfg)
	if err := os.WriteFile(file, cnt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := Loadallsettings(true); err != nil {
		t.Fatal(err)
	}

	if s := GetSettingsScheduler("Default"); s == nil || s.IntervalImdb != "5d" {
		t.Errorf("scheduler edit not applied on reload: %+v", s)
	}

	if GetSettingsScheduler("Other") != nil || GetSettingsScheduler("New") == nil {
		t.Error("scheduler add/delete not applied on reload")
	}

	if GetSettingsMedia("movie_M").CfgScheduler == nil {
		t.Error("media scheduler pointer nil after reload")
	}
}

func TestGetMusicMetaSourcePriority(t *testing.T) {
	useSnapshot(t, &ConfigSnapshot{General: &GeneralConfig{}})

	// Without API keys the key-bound providers are dropped from the defaults.
	want := []string{"musicbrainz", "discogs", "deezer", "theaudiodb", "itunes"}
	if got := GetMusicMetaSourcePriority(); !slices.Equal(got, want) {
		t.Errorf("defaults = %v, want %v", got, want)
	}

	useSnapshot(t, &ConfigSnapshot{General: &GeneralConfig{
		MusicMetaSourcePriority: []string{"lastfm", "discogs", "acoustid"},
		LastFMAPIKey:            "key",
	}})

	if got := GetMusicMetaSourcePriority(); !slices.Equal(got, []string{"lastfm", "discogs"}) {
		t.Errorf("configured = %v", got)
	}
}

func TestGetMusicSourcePenalty(t *testing.T) {
	useSnapshot(t, &ConfigSnapshot{General: &GeneralConfig{
		MusicMetaSourcePenalties: map[string]float64{"deezer": 0.05},
	}})

	if got := GetMusicSourcePenalty("deezer"); got != 0.05 {
		t.Errorf("deezer = %v", got)
	}

	if got := GetMusicSourcePenalty("discogs"); got != 0 {
		t.Errorf("unconfigured = %v", got)
	}

	if got := GetMusicSourcePenalty(""); got != 0 {
		t.Errorf("empty source = %v", got)
	}
}

func TestSettingsRangesAndLists(t *testing.T) {
	useSnapshot(t, &ConfigSnapshot{
		General: &GeneralConfig{},
		Media: map[string]*MediaTypeConfig{
			"movie_EN": {Lists: []MediaListsConfig{{Name: "imdb"}, {Name: "trakt"}}},
			"movie_DE": {Lists: []MediaListsConfig{{Name: "imdb"}}},
			"nil":      nil,
		},
		List:    map[string]*ListsConfig{"Rock": {Name: "Rock"}},
		Quality: map[string]*QualityConfig{"SD": {}, "HD": {}},
	})

	all := GetSettingsMediaListAll()
	slices.Sort(all)

	if !slices.Equal(all, []string{"imdb", "trakt"}) {
		t.Errorf("GetSettingsMediaListAll = %v", all)
	}

	var lists []string

	RangeSettingsMediaLists("movie_EN", func(l *MediaListsConfig) { lists = append(lists, l.Name) })
	RangeSettingsMediaLists("does-not-exist", func(*MediaListsConfig) { t.Error("called for a missing media key") })

	if !slices.Equal(lists, []string{"imdb", "trakt"}) {
		t.Errorf("RangeSettingsMediaLists = %v", lists)
	}

	var media int

	RangeSettingsMedia(func(string, *MediaTypeConfig) error { media++; return nil })

	if media != 2 {
		t.Errorf("RangeSettingsMedia visited %d, want 2 (nil entries skipped)", media)
	}

	var stops int

	RangeSettingsMediaBreak(func(string, *MediaTypeConfig) bool { stops++; return true })

	if stops != 1 {
		t.Errorf("RangeSettingsMediaBreak visited %d after break", stops)
	}

	var quals int

	RangeSettingsQuality(func(string, *QualityConfig) { quals++ })

	if quals != 2 {
		t.Errorf("RangeSettingsQuality visited %d", quals)
	}

	if !TestSettingsList("Rock") || TestSettingsList("Pop") {
		t.Error("TestSettingsList gave a wrong answer")
	}

	if got := (&IndexersConfig{Name: "rock"}).Getlistbyindexer(); got == nil || got.Name != "Rock" {
		t.Errorf("Getlistbyindexer = %+v", got)
	}

	if got := (&IndexersConfig{Name: "jazz"}).Getlistbyindexer(); got != nil {
		t.Errorf("Getlistbyindexer(missing) = %+v", got)
	}
}
