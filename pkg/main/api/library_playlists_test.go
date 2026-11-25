package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// queryAs runs a handler for a GET query as the session's account.
func queryAs(s *Session, h gin.HandlerFunc, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("session", s)
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	h(c)

	return w
}

// TestPlaylistHandlers: an account adds only what it may see, the titles
// come from the database, a series plays its episodes in order, others
// see a playlist only when it is shared, and then only their own groups'
// media.
func TestPlaylistHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	for _, s := range []string{
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (2, 1, 'S01E02', 'Second', '1', '2')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id) values (6, 1, 1, 2)",
		"insert into serie_episode_files (id, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, location) values " +
			"(22, 1, 6, 2, 1, '/s1e2.mkv'), (21, 1, 5, 1, 1, '/s1e1.mkv')",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}

	create := func(s *Session, name string) string {
		t.Helper()

		w := callAs(s, HandlePlaylistSave, url.Values{"name": {name}})

		var p struct{ ID uint }
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.ID == 0 {
			t.Fatalf("create %s: %d %s", name, w.Code, w.Body)
		}

		return strconv.FormatUint(uint64(p.ID), 10)
	}

	annas := create(anna, "Mine")

	w := callAs(anna, HandlePlaylistAdd, url.Values{"id": {annas}, "items": {
		`[{"kind":"movie","entry":2},{"kind":"movie","entry":1},{"kind":"episode","entry":1,"item":5}]`,
	}})
	if !strings.Contains(w.Body.String(), `"added":1`) || !strings.Contains(w.Body.String(), `"skipped":2`) {
		t.Errorf("anna adds: %s", w.Body)
	}

	if w := queryAs(anna, HandlePlaylistPlay, "id="+annas); !strings.Contains(w.Body.String(), `"id":12`) ||
		!strings.Contains(w.Body.String(), `"label":"Deutsch"`) {
		t.Errorf("anna plays: %s", w.Body)
	}

	if w := queryAs(anna, HandlePlaylistView, "id="+annas); !strings.Contains(w.Body.String(), "Deutsch") ||
		!strings.Contains(w.Body.String(), "libPlaylistItem(&#39;remove&#39;") {
		t.Errorf("anna's view: %s", w.Body)
	}

	shows := create(admin, "Shows")
	callAs(admin, HandlePlaylistAdd, url.Values{"id": {shows}, "items": {`[{"kind":"series","entry":1},{"kind":"episode","entry":1,"item":6}]`}})

	w = queryAs(admin, HandlePlaylistPlay, "id="+shows)

	var played struct {
		List []libPlaylistItem
	}
	if err := json.Unmarshal(w.Body.Bytes(), &played); err != nil || len(played.List) != 3 ||
		played.List[0].ID != 21 || played.List[1].ID != 22 || played.List[2].ID != 22 || played.List[2].Label != "Show S01E02 Second" {
		t.Errorf("series in order, then the episode: %s", w.Body)
	}

	if w := queryAs(anna, HandlePlaylistPlay, "id="+shows); w.Code != http.StatusNotFound {
		t.Errorf("anna plays the admin's private list: %d", w.Code)
	}

	callAs(admin, HandlePlaylistSave, url.Values{"id": {shows}, "name": {"Shows"}, "shared": {"1"}})

	if w := queryAs(anna, HandlePlaylistPlay, "id="+shows); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"list":[]`) {
		t.Errorf("anna plays the shared list of a group she can't see: %d %s", w.Code, w.Body)
	}

	if w := queryAs(anna, HandlePlaylistView, "id="+shows); !strings.Contains(w.Body.String(), "not available") ||
		strings.Contains(w.Body.String(), "libPlaylistItem(") {
		t.Errorf("anna's view of the shared list: %s", w.Body)
	}

	if w := callAs(anna, HandlePlaylistDelete, url.Values{"id": {shows}}); w.Code != http.StatusNotFound {
		t.Errorf("anna deletes the admin's list: %d", w.Code)
	}
}
