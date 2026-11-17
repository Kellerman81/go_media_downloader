package api

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestContinueAndWatched: a started movie shows in its group's Continue
// row; marking it watched (only for files the account may see) takes it
// out and the detail marks it.
func TestContinueAndWatched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	h := database.PlayHistory{Username: "anna", Kind: "movie", FileID: 12, Title: "Deutsch", Position: 600, Duration: 6000}
	if err := database.PlayProgress(&h, true); err != nil {
		t.Fatal(err)
	}

	groups := libraryGroups()
	de := &groups[slices.IndexFunc(groups, func(g libGroup) bool { return g.Name == "movie_DE" })]

	row := renderComponentToString(libContinueRow("anna", de))
	if !strings.Contains(row, "Continue") || !strings.Contains(row, "Deutsch") || !strings.Contains(row, "1 h 30 min left") {
		t.Fatalf("continue row: %s", row)
	}

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	if w := callAs(anna, HandleLibraryWatched, url.Values{"kind": {"movie"}, "id": {"11"}, "watched": {"1"}}); w.Code != http.StatusNotFound {
		t.Errorf("file of another group: %d", w.Code)
	}

	callAs(anna, HandleLibraryWatched, url.Values{"kind": {"movie"}, "id": {"12"}, "watched": {"1"}})

	if row := libContinueRow("anna", de); row != nil {
		t.Errorf("watched movie still in Continue: %s", renderComponentToString(row))
	}

	if mark := renderComponentToString(libWatchedMark("movie", 12,
		database.PlayCompletions("anna", "movie", "movie_files", "movie_id", new(uint64(2))))); !strings.Contains(mark, "btn-success") {
		t.Errorf("watched mark: %s", mark)
	}
}

// TestWatchedEntry: the selection mode marks all files of an entry (a
// series: every episode file) for the account, only in its groups, and
// not albums.
func TestWatchedEntry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	for _, q := range []string{
		"insert into serie_episode_files (id, location, serie_id, serie_episode_id, dbserie_id) values (31, '/e1', 1, 5, 1), (32, '/e2', 1, 5, 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"serie_X", "movie_DE"}}
	mark := func(typ, id, on string) int {
		return callAs(anna, HandleLibraryWatchedEntry, url.Values{"type": {typ}, "id": {id}, "watched": {on}}).Code
	}

	count := func() int {
		return database.Getdatarow[int](false, "select count() from play_history where username = 'anna' and kind = 'episode' and completion = 1")
	}

	if code := mark("series", "1", "1"); code != http.StatusOK && code != http.StatusNoContent || count() != 2 {
		t.Fatalf("series watched: %d, %d files", code, count())
	}

	if code := mark("series", "1", "0"); count() != 0 {
		t.Errorf("series unwatched: %d, %d files left", code, count())
	}

	if code := mark("movie", "1", "1"); code != http.StatusNotFound {
		t.Errorf("movie of another group: %d", code)
	}

	if code := mark("album", "1", "1"); code != http.StatusNotFound && code != http.StatusBadRequest {
		t.Errorf("album: %d", code)
	}
}
