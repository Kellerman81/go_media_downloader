package api

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestLibraryCollections: the Collections view lists the collections with
// at least two movies of the group, a click filters the grid to them.
func TestLibraryCollections(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, s := range []string{
		"update dbmovies set collection_id = 9485, collection_name = 'The Fast and the Furious Collection' where id in (3, 5, 7)",
		"update dbmovies set collection_id = 1, collection_name = 'Single Collection' where id = 9",
		"update dbmovies set collection_id = 0 where id = 11",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&view=collections")
	if !strings.Contains(out, "The Fast and the Furious Collection") || strings.Contains(out, "Single Collection") ||
		!strings.Contains(out, "libShowCollection(9485)") || !strings.Contains(out, "/api/admin/library/cover/movie/3") ||
		!strings.Contains(out, "2003–2007") {
		t.Errorf("collections view:\n%.3000s", out)
	}

	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&view=collections&q=nothing"); strings.Contains(out, "Furious") {
		t.Errorf("search on collection names:\n%.2000s", out)
	}

	_, out = libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&coll=9485")
	if strings.Count(out, `class="lib-card"`) != 3 || !strings.Contains(out, "libShowCollection(0)") ||
		!strings.Contains(out, "Movie 05") || strings.Contains(out, "Movie 09") {
		t.Errorf("collection filter:\n%.3000s", out)
	}
}
