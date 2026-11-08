package api

import (
	"strings"
	"testing"

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
