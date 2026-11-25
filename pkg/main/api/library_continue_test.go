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

	row := plain(renderComponentToString(libContinueRow("anna", de)))
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

// TestNextEpisodes: after the first episode is finished the second is next,
// in the apps and in the Library's Next up row.
func TestNextEpisodes(t *testing.T) {
	clientTestData(t)

	groups := groupsOf(adminAccess, "series")
	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 0 {
		t.Fatalf("nothing played yet: %+v", rows)
	}

	fid := uint(21)
	if err := database.SetWatched("admin", "episode", &fid, true); err != nil {
		t.Fatal(err)
	}

	rows := nextEpisodes(groups, "admin", nil, 10, true)
	if len(rows) != 1 || rows[0].ID != 6 || rows[0].FileID != 22 {
		t.Fatalf("next: %+v", rows)
	}

	g := libGroupByName("serie_X")
	if out := renderComponentToString(libNextUpRow("admin", "en", g)); !strings.Contains(out, "S01E02") || !strings.Contains(out, "libContinue(&#39;episode&#39;, 22, 1") {
		t.Fatalf("row: %s", out)
	}

	// Stopped in the middle: in Continue, not in Next up.
	_ = database.PlayProgress(&database.PlayHistory{Username: "admin", Kind: "episode", FileID: 22, Position: 300, Duration: 1200}, true)

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 0 {
		t.Fatalf("started episode in Next up: %+v", rows)
	}

	fid = 22
	_ = database.SetWatched("admin", "episode", &fid, true)

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 0 {
		t.Fatalf("all watched: %+v", rows)
	}
}

// TestContinueDrop: the Continue card's button clears where the account
// stopped (only in its groups), and what it played 60 days ago isn't
// offered any more.
func TestContinueDrop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	play := func() {
		if err := database.PlayProgress(&database.PlayHistory{Username: "anna", Kind: "movie", FileID: 12, Position: 600, Duration: 6000}, true); err != nil {
			t.Fatal(err)
		}
	}

	play()

	de := libGroupByName("movie_DE")
	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}

	if w := callAs(anna, HandleLibraryContinueDrop, url.Values{"row": {"continue"}, "type": {"movie"}, "id": {"1"}}); w.Code != http.StatusNotFound {
		t.Errorf("movie of another group: %d", w.Code)
	}

	if w := callAs(anna, HandleLibraryContinueDrop, url.Values{"row": {"nextup"}, "type": {"movie"}, "id": {"2"}}); w.Code != http.StatusBadRequest {
		t.Errorf("movie in Next up: %d", w.Code)
	}

	if w := callAs(anna, HandleLibraryContinueDrop, url.Values{"row": {"continue"}, "type": {"movie"}, "id": {"2"}}); w.Code >= http.StatusMultipleChoices {
		t.Fatalf("drop: %d %s", w.Code, w.Body)
	}

	if row := libContinueRow("anna", de); row != nil || database.PlayResume("anna", "movie", new(uint(12))) != 0 {
		t.Errorf("dropped movie still in Continue: %s", renderComponentToString(row))
	}

	play()

	if libContinueRow("anna", de) == nil {
		t.Fatal("played again, not in Continue")
	}

	_ = database.ExecNErr("update play_history set updated_at = datetime('now', 'localtime', '-61 days') where username = 'anna'")

	if row := libContinueRow("anna", de); row != nil {
		t.Errorf("stale movie in Continue: %s", renderComponentToString(row))
	}
}

// TestNextUpDrop: a series removed from Next up stays out until an episode
// of it is played again, and a series not played for 60 days drops out.
func TestNextUpDrop(t *testing.T) {
	clientTestData(t)

	groups := groupsOf(adminAccess, "series")
	if err := database.SetWatched("admin", "episode", new(uint(21)), true); err != nil {
		t.Fatal(err)
	}

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 1 {
		t.Fatalf("next: %+v", rows)
	}

	admin := &Session{UserID: "admin", Role: database.RoleAdmin}
	if w := callAs(admin, HandleLibraryContinueDrop, url.Values{"row": {"nextup"}, "type": {"series"}, "id": {"1"}}); w.Code >= http.StatusMultipleChoices {
		t.Fatalf("drop: %d %s", w.Code, w.Body)
	}

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 0 {
		t.Fatalf("dropped series in Next up: %+v", rows)
	}

	// The apps ask for one series: it is answered.
	if rows := nextEpisodes(groups, "admin", []uint64{1}, 10, false); len(rows) != 1 {
		t.Errorf("asked for the series: %+v", rows)
	}

	_ = database.ExecNErr("update play_history set updated_at = datetime('now', 'localtime', '+1 minute') where username = 'admin'")

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 1 {
		t.Fatalf("played again, not in Next up: %+v", rows)
	}

	_ = database.ExecNErr("update play_history set updated_at = datetime('now', 'localtime', '-61 days') where username = 'admin'")

	if rows := nextEpisodes(groups, "admin", nil, 10, true); len(rows) != 0 {
		t.Errorf("stale series in Next up: %+v", rows)
	}
}

// TestWatchedMany: "Mark all shown" sends the files of all shown rows at
// once; one file the account may not see stops the whole request.
func TestWatchedMany(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	if err := database.ExecNErr("insert into serie_episode_files (id, location, serie_id, serie_episode_id, dbserie_id) values (31, '/e1', 1, 5, 1), (32, '/e2', 1, 5, 1)"); err != nil {
		t.Fatal(err)
	}

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"serie_X"}}
	watched := func() int {
		return database.Getdatarow[int](false, "select count() from play_history where username = 'anna' and kind = 'episode' and completion = 1")
	}

	if w := callAs(anna, HandleLibraryWatched, url.Values{"kind": {"episode"}, "id": {"31", "99999"}, "watched": {"1"}}); w.Code != http.StatusNotFound || watched() != 0 {
		t.Fatalf("with an unknown file: %d, %d watched", w.Code, watched())
	}

	if w := callAs(anna, HandleLibraryWatched, url.Values{"kind": {"episode"}, "id": {"31", "32"}, "watched": {"1"}}); w.Code >= http.StatusMultipleChoices || watched() != 2 {
		t.Fatalf("all shown: %d, %d watched", w.Code, watched())
	}

	if callAs(anna, HandleLibraryWatched, url.Values{"kind": {"episode"}, "id": {"31", "32"}, "watched": {"0"}}); watched() != 0 {
		t.Errorf("all shown not watched: %d watched", watched())
	}
}
