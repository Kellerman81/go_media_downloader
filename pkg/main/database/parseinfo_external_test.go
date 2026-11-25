package database_test

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/books" // registers the book tables
)

// TestRemoveUnmatchedUnder: a folder moved to move_unprocessed takes its
// files' unmatched rows along (book rows are per file); other folders',
// also one whose name starts the same, stay.
func TestRemoveUnmatchedUnder(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	for _, p := range []string{
		"/downloads/Books/Alexandra Ivy - Verlockung der Düsternis/a.epub",
		"/downloads/Books/Alexandra Ivy - Verlockung der Düsternis/sub/a.pdf",
		"/downloads/Books/Alexandra Ivy - Verlockung der Düsternis",
		"/downloads/Books/Alexandra Ivy - Verlockung der Düsternis 2/a.epub",
		"/downloads/Books/Other/a.epub",
	} {
		if err := database.ExecNErr("insert into book_file_unmatcheds (listname, filepath, parsed_data) values ('structure', ?, '')", p); err != nil {
			t.Fatal(err)
		}
	}

	database.RemoveUnmatchedUnder(config.MediaTypeBook, "/downloads/Books/Alexandra Ivy - Verlockung der Düsternis/")

	left := database.GetrowsN[string](false, 10, "select filepath from book_file_unmatcheds order by filepath")
	if len(left) != 2 || left[0] != "/downloads/Books/Alexandra Ivy - Verlockung der Düsternis 2/a.epub" || left[1] != "/downloads/Books/Other/a.epub" {
		t.Errorf("left: %q", left)
	}
}
