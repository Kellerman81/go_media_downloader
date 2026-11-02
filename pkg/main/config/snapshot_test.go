package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadToleratesUnnamedAndDuplicateEntries(t *testing.T) {
	cfg := testMainConfig()
	cfg.Media.Movies[0].Lists = append(cfg.Media.Movies[0].Lists,
		MediaListsConfig{TemplateList: "listA", TemplateQuality: "SD"},
		MediaListsConfig{Name: "M-LIST", TemplateQuality: "HD"})
	cfg.Quality = append(cfg.Quality, QualityConfig{Name: "hd", CutoffQuality: "second"})
	cfg.Scheduler = append(cfg.Scheduler, SchedulerConfig{})

	file := useTempConfig(t, cfg)

	snap := getCurrentConfig()
	if len(snap.issues) == 0 {
		t.Error("expected warnings for unnamed/duplicate entries")
	}

	m := GetSettingsMedia("movie_M")
	if len(m.Lists) != 3 || len(m.ListsMap) != 1 || len(m.ListsMapIdx) != 1 {
		t.Errorf("lists=%d map=%d idx=%d, want 3/1/1", len(m.Lists), len(m.ListsMap), len(m.ListsMapIdx))
	}

	if _, ok := m.ListsMap[""]; ok {
		t.Error("unnamed list indexed under empty key")
	}

	if strings.Join(m.ListsQualities, ",") != "HD" {
		t.Errorf("ListsQualities = %v, want [HD]", m.ListsQualities)
	}

	if q := GetSettingsQuality("HD"); q == nil || q.CutoffQuality == "second" {
		t.Error("duplicate quality: first entry must win")
	}

	if _, ok := snap.Scheduler[""]; ok {
		t.Error("unnamed scheduler indexed")
	}

	if err := WriteCfg(); err != nil {
		t.Fatal(err)
	}

	got, err := Readconfigtoml()
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Media.Movies[0].Lists) != 3 || len(got.Quality) != 2 || len(got.Scheduler) != 3 {
		t.Errorf("unnamed/duplicate entries not round-tripped: %s", file)
	}
}

func TestUpdateIsCopyOnWrite(t *testing.T) {
	useTempConfig(t, testMainConfig())

	old := getCurrentConfig()
	oldPaths := GetSettingsPathAll()
	oldMovie := old.Media["movie_M"]
	oldAlpha := old.Path["alpha"]

	if err := UpdateCfgEntry(Conf{Name: "path_alpha", Data: PathsConfig{Name: "alpha", MinSize: 99}}); err != nil {
		t.Fatal(err)
	}

	if err := DeleteCfgEntry("path_zeta"); err != nil {
		t.Fatal(err)
	}

	if oldAlpha.MinSize != 0 || oldPaths[1].MinSize != 0 {
		t.Error("update mutated a previously obtained snapshot")
	}

	if names(oldPaths, func(c *PathsConfig) string { return c.Name }) != "zeta,alpha,mid" {
		t.Errorf("delete mutated old slice: %v", oldPaths)
	}

	if oldMovie.Data[0].CfgPath != oldAlpha {
		t.Error("old media config now points at a new path object")
	}

	if GetSettingsPath("alpha").MinSize != 99 || GetSettingsPath("zeta") != nil {
		t.Error("new snapshot does not reflect the change")
	}

	if GetSettingsMedia("movie_M").Data[0].CfgPath.MinSize != 99 {
		t.Error("new media config not linked to the updated path")
	}
}

func TestInvalidUpdateIsNotPersisted(t *testing.T) {
	file := useTempConfig(t, testMainConfig())

	before, _ := os.ReadFile(file)
	snap := getCurrentConfig()

	err := UpdateCfgEntry(Conf{Name: "indexer_idx", Data: IndexersConfig{Name: "idx"}}) // missing URL
	if err == nil {
		t.Fatal("expected validation error")
	}

	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Error("invalid config was written to disk")
	}

	if getCurrentConfig() != snap {
		t.Error("invalid config replaced the live snapshot")
	}

	entries, _ := os.ReadDir(filepath.Dir(file))
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestConfigHelpersWithoutSnapshot(t *testing.T) {
	useSnapshot(t, nil)

	if GetCfgAll() != nil || GetSettingTemplatesFor("quality") != nil || GetSettingsDownloaderAll() != nil {
		t.Error("helpers returned data without a config")
	}

	RangeSettingsList(func(string, *ListsConfig) { t.Error("ranged without a config") })
}
