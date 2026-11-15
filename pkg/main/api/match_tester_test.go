package api

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
)

func TestRenderMTUpgrade(t *testing.T) {
	if renderMTUpgrade(nil) != nil {
		t.Fatal("nil check must render nothing")
	}

	lib := []structure.AlbumLibraryFile{{Location: "/lib/1 - Kapitel 1.mp3", Format: "mp3"}}

	cases := []struct {
		name  string
		check structure.AlbumUpgradeCheck
		want  []string
	}{
		{"empty library", structure.AlbumUpgradeCheck{Allowed: true}, []string{"no files of this release yet"}},
		{"blocked", structure.AlbumUpgradeCheck{
			Existing: lib, OldPriority: 110, NewPriority: 110, NewFiles: 1,
		}, []string{"Processing will stop", "110 for the new files vs 110", "1 - Kapitel 1.mp3", "alert-danger"}},
		{"upgrade", structure.AlbumUpgradeCheck{
			Existing: lib, OldPriority: 100, NewPriority: 120, NewFiles: 1, Allowed: true,
		}, []string{"count as an upgrade", "alert-info"}},
		{"not compared", structure.AlbumUpgradeCheck{Existing: lib, Allowed: true}, []string{"do not block"}},
	}

	for _, tc := range cases {
		var b strings.Builder
		if err := renderMTUpgrade(&tc.check).Render(&b); err != nil {
			t.Fatal(err)
		}

		for _, w := range tc.want {
			if !strings.Contains(b.String(), w) {
				t.Errorf("%s: output lacks %q:\n%s", tc.name, w, b.String())
			}
		}
	}
}

func TestMTEntryPreselect(t *testing.T) {
	entry := func(dist float64, countOK bool) *mtEntry {
		return &mtEntry{Entry: &importfeed.PreviewEntry{Candidates: []*importfeed.PreviewCandidate{
			{Dist: dist, CountOK: countOK},
		}}}
	}

	cases := []struct {
		name string
		e    *mtEntry
		want bool
	}{
		{"strong, count ok", entry(0.05, true), true},
		{"weak", entry(0.5, true), false},
		{"strong, missing or extra tracks not allowed", entry(0.05, false), false},
		{"no candidate", &mtEntry{Entry: &importfeed.PreviewEntry{}}, false},
	}

	for _, tc := range cases {
		if got := tc.e.preselect(0.1); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMTSources(t *testing.T) {
	root := t.TempDir()
	imp := filepath.Join(root, "import")
	unprocessed := filepath.Join(root, "tocheck")

	for _, d := range []string{
		imp,
		filepath.Join(unprocessed, "no_match", "Book A"),
		filepath.Join(unprocessed, "no_match", "Book B"),
		filepath.Join(unprocessed, "no_match", ".hidden"),
		filepath.Join(unprocessed, "no_files"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(unprocessed, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Two imports sharing one move_unprocessed path, as in production.
	cfgp := &config.MediaTypeConfig{DataImport: []config.MediaDataImportConfig{
		{TemplatePath: "a", CfgPath: &config.PathsConfig{Path: imp}, MoveUnprocessed: unprocessed},
		{TemplatePath: "b", CfgPath: &config.PathsConfig{Path: imp}, MoveUnprocessed: unprocessed},
	}}

	labels := func(src []mtSource) []string {
		out := make([]string, len(src))
		for i := range src {
			out[i] = src[i].Label
		}

		return out
	}

	want := []string{
		"Import: " + imp + " (a)",
		"Unprocessed: no_files (0 folders)",
		"Unprocessed: no_match (2 folders)",
	}
	if got := labels(mtSources(cfgp, true)); !slices.Equal(got, want) {
		t.Errorf("with counts: %q, want %q", got, want)
	}

	if got := labels(mtSources(cfgp, false)); !slices.Equal(got, []string{
		"Import: " + imp + " (a)", "Unprocessed: no_files", "Unprocessed: no_match",
	}) {
		t.Errorf("without counts: %q", got)
	}

	value := "0|" + filepath.Join(unprocessed, "no_match")
	if idx, path, _, ok := mtResolveSource(cfgp, value); !ok || idx != 0 || path != filepath.Join(unprocessed, "no_match") {
		t.Errorf("resolve %q: %d %q %v", value, idx, path, ok)
	}

	if _, _, _, ok := mtResolveSource(cfgp, "0|/etc"); ok {
		t.Error("a path outside the sources resolved")
	}
}

func TestRenderMatchTesterPageGroupsConfigsByKind(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{
		Movies: mediaGroup("EN"), Music: mediaGroup("Music"), AudioBooks: mediaGroup("Audio"),
	})

	var b strings.Builder
	if err := renderMatchTesterPage("token").Render(&b); err != nil {
		t.Fatal(err)
	}

	page := b.String()

	// Music and audiobooks share the album matcher; series and books have no
	// config here and get no group.
	for _, want := range []string{
		`<optgroup label="Music &amp; Audiobooks"><option value="music_Music"`,
		`<option value="audiobook_Audio"`,
		`<optgroup label="Movies"><option value="movie_EN"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %s", want)
		}
	}

	if strings.Contains(page, `label="Series"`) || strings.Contains(page, `label="Books"`) {
		t.Error("group for a kind without config")
	}

	if mtKindOf(config.MediaTypeAudiobook).Key != "album" || mtKindOf(config.MediaTypeMovie).Key != "movie" {
		t.Error("kind from media type")
	}
}
