package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// asAdmin gives a test request the session of the built-in admin, as
// protectAdminRoutes does for a logged-in admin.
func asAdmin(c *gin.Context) {
	c.Set("session", &Session{UserID: DefaultUsername, Role: database.RoleAdmin})
}

func asUser(c *gin.Context, role string, groups ...string) {
	c.Set("session", &Session{UserID: "anna", Role: role, Groups: groups})
}

// accessTestData: movie groups EN and DE, one movie with a file in each
// list, and a series group X.
func accessTestData(t *testing.T) {
	t.Helper()

	accessTestDataWith(t, config.MediaConfig{Movies: append(mediaGroup("EN"), mediaGroup("DE")...), Series: mediaGroup("X")})
}

// accessTestDataWith is accessTestData with other media groups.
func accessTestDataWith(t *testing.T, media config.MediaConfig) {
	t.Helper()

	useMediaConfig(t, media)

	for _, s := range []string{
		"insert into dbmovies (id, title) values (1, 'English'), (2, 'Deutsch')",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List'), (2, 2, 'DE List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location) values (11, 1, 1, '/en.mkv'), (12, 2, 2, '/de.mkv')",
		"insert into dbseries (id, seriename) values (1, 'Show')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'X List')",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (1, 1, 'S01E01', 'Pilot', '1', '1')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id, missing) values (5, 1, 1, 1, 1)",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUserAccessData(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	de := &userAccess{Name: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	movies, series := libKindByType("movie"), libKindByType("series")

	for name, tc := range map[string]struct{ got, want bool }{
		"group DE":           {de.AllowsGroup("movie_de"), true},
		"group EN":           {de.AllowsGroup("movie_EN"), false},
		"list DE":            {de.AllowsList("de list"), true},
		"entry DE":           {de.allowsEntry(movies, new(uint(2))), true},
		"entry EN":           {de.allowsEntry(movies, new(uint(1))), false},
		"metadata row DE":    {de.allowsDB(movies, new(uint(2))), true},
		"metadata row EN":    {de.allowsDB(movies, new(uint(1))), false},
		"file DE":            {de.allowsFile(movies, new(uint(12))), true},
		"file EN":            {de.allowsFile(movies, new(uint(11))), false},
		"episode of X":       {de.allowsEpisode(new(uint(5))), false},
		"series of X":        {de.allowsEntry(series, new(uint(1))), false},
		"admin sees EN":      {adminAccess.allowsFile(movies, new(uint(11))), true},
		"no groups, no list": {(&userAccess{Role: database.RoleUser}).allowsEntry(movies, new(uint(2))), false},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: %v, want %v", name, tc.got, tc.want)
		}
	}

	// Library: only the allowed group, and nothing of the other.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asUser(c, database.RoleUser, "movie_DE")
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/library?group=movie_EN", nil)
	LibraryPage(c)

	if body := w.Body.String(); !strings.Contains(body, "movie_DE") || strings.Contains(body, "group=movie_EN") ||
		!strings.Contains(body, ".lib-admin, .lib-search, .lib-download, .lib-report { display: none") {
		t.Errorf("library page: groups or hidden actions wrong")
	}

	for _, tc := range []struct {
		handler gin.HandlerFunc
		query   string
		params  gin.Params
		want    int
	}{
		{HandleLibraryItem, "type=movie&id=2", nil, http.StatusOK},
		{HandleLibraryItem, "type=movie&id=1", nil, http.StatusNotFound},
		{HandleLibraryPlaylist, "type=movie&id=1", nil, http.StatusNotFound},
		{HandleLibraryCover, "", gin.Params{{Key: "type", Value: "movie"}, {Key: "id", Value: "1"}}, http.StatusNotFound},
		{HandlePlayerInfo, "", gin.Params{{Key: "kind", Value: "movie"}, {Key: "id", Value: "11"}}, http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asUser(c, database.RoleUser, "movie_DE")
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		c.Params = tc.params
		tc.handler(c)

		if w.Code != tc.want {
			t.Errorf("%s %v: %d, want %d", tc.query, tc.params, w.Code, tc.want)
		}
	}

	// Calendar: entries of other groups are dropped, mixed events keep theirs.
	events := de.calendarEvents([]calendarEvent{
		{Title: "both", Entries: []calendarEntry{{ID: 1, List: "EN List"}, {ID: 2, List: "DE List", LibraryURL: "/de"}}},
		{Title: "english", Entries: []calendarEntry{{ID: 1, List: "EN List"}}},
	})
	if len(events) != 1 || events[0].ID != 2 || events[0].LibraryURL != "/de" || events[0].Listname != "DE List" {
		t.Errorf("calendar events: %+v", events)
	}
}

func TestUserAccessPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	user := &userAccess{Name: "u", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	searcher := &userAccess{Name: "s", Role: database.RoleSearchingUser, Groups: []string{"movie_DE"}, Features: []string{database.FeatureSearch}}
	requester := &userAccess{Name: "r", Role: database.RoleRequestingUser, Groups: []string{"movie_DE"}, Features: []string{database.FeatureRequest}}

	for _, tc := range []struct {
		acc          *userAccess
		method, path string
		want         bool
	}{
		{user, http.MethodGet, "/api/admin/library", true},
		{user, http.MethodGet, "/api/admin/library/play/movie/12/stream", true},
		{user, http.MethodGet, "/api/admin/calendar/content", true},
		{user, http.MethodPost, "/api/admin/library/delete", false},
		{user, http.MethodGet, "/api/admin/tableedit/movies/2", false},
		{user, http.MethodGet, "/api/admin/config/general", false},
		{user, http.MethodGet, "/api/admin/users", false},
		{user, http.MethodPost, "/api/admin/proxy", false},
		{user, http.MethodGet, "/api/admin/metadata-search", false},
		{user, http.MethodPost, "/api/admin/account", true},
		{searcher, http.MethodPost, "/api/admin/proxy", true},
		{searcher, http.MethodGet, "/api/admin/metadata-search", false},
		{requester, http.MethodGet, "/api/admin/metadata-search", true},
		{requester, http.MethodPost, "/api/admin/add/movie", true},
		{requester, http.MethodPost, "/api/admin/add/movie/manual", false},
		{requester, http.MethodPost, "/api/admin/add/audiobook/manual", false},
		{requester, http.MethodPost, "/api/admin/add/book/author", false},
		{requester, http.MethodPost, "/api/admin/add/artist-albums", false},
		{requester, http.MethodPost, "/api/admin/discover/series-albums", false},
		{requester, http.MethodGet, "/api/admin/requests", false},
		{requester, http.MethodPost, "/api/admin/proxy", false},
		{adminAccess, http.MethodGet, "/api/admin/config/general", true},
	} {
		if got := tc.acc.allowsPath(tc.method, tc.path); got != tc.want {
			t.Errorf("%s %s %s: %v, want %v", tc.acc.Role, tc.method, tc.path, got, tc.want)
		}
	}

	// The proxy: one entry of the own groups only.
	for path, want := range map[string]bool{
		"/api/movies/search/list/2":               true,
		"/api/movies/search/list/1":               false,
		"/api/series/episodes/search/list/5":      false,
		"/api/movies/search/all/missing/movie_DE": false,
		"/api/movies/refresh/2":                   false,
	} {
		if got := searcher.allowsProxyPath(path); got != want {
			t.Errorf("proxy %s: %v, want %v", path, got, want)
		}
	}

	if user.allowsProxyPath("/api/movies/search/list/2") {
		t.Error("a viewing account may search")
	}

	// Add Media: only lists of the own groups, and the list must be named.
	for form, want := range map[string]bool{
		"movie_list=DE+List":                    true,
		"movie_list=EN+List":                    false,
		"tmdb_id=5":                             false,
		"movie_list=DE+List&series_list=X+List": false,
		"manualMovie_list=de+list":              true,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/add/movie", strings.NewReader(form))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		if got := requester.allowsRequestTarget(c); got != want {
			t.Errorf("add %s: %v, want %v", form, got, want)
		}
	}

	// The Add Media page lists only the own groups' lists.
	_, _, body := metadataSearchBodyAs(t, requester)
	if !strings.Contains(body, "DE List") || strings.Contains(body, "EN List") {
		t.Error("Add Media lists other groups' lists")
	}

	if !strings.Contains(body, ".meta-manual") {
		t.Error("manual entry not hidden for a requesting user")
	}

	if _, _, admin := metadataSearchBodyAs(t, adminAccess); strings.Contains(admin, ".meta-manual, ") {
		t.Error("manual entry hidden for the admin")
	}
}

func metadataSearchBodyAs(t *testing.T, acc *userAccess) (int, string, string) {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("session", &Session{UserID: acc.Name, Role: acc.Role, Groups: acc.Groups})
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/metadata-search", nil)
	MetadataSearchPage(c)

	return w.Code, "", w.Body.String()
}

func TestUserAccounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	post := func(h gin.HandlerFunc, form url.Values) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h(c)

		return w.Body.String()
	}

	if body := post(HandleUserSave, url.Values{"username": {"Admin"}, "role": {"user"}, "password": {"secret123"}}); !strings.Contains(body, "built-in admin") {
		t.Error("the admin name was accepted")
	}

	if body := post(HandleUserSave, url.Values{"username": {"anna"}, "role": {"user"}, "password": {"short"}}); !strings.Contains(body, "8 characters") {
		t.Error("a short password was accepted")
	}

	post(HandleUserSave, url.Values{"username": {"anna"}, "role": {"searchinguser"}, "password": {"secret123"}, "groups": {"movie_DE", "serie_X"}})

	if body := post(HandleUserSave, url.Values{"username": {"ANNA"}, "role": {"user"}, "password": {"secret123"}}); !strings.Contains(body, "already exists") {
		t.Error("a second anna was accepted")
	}

	acc, ok := authenticateUser("Anna", "secret123")
	if !ok || acc.Name != "anna" || acc.Role != database.RoleSearchingUser || len(acc.Groups) != 2 {
		t.Fatalf("login: %+v %v", acc, ok)
	}

	if _, ok := authenticateUser("anna", "wrong-password"); ok {
		t.Error("a wrong password logged in")
	}

	if _, ok := authenticateUser(DefaultUsername, webPassword()); !ok {
		t.Error("the built-in admin can't log in")
	}

	// A new password ends the sessions and the remembered logins.
	sess := sessionStore.createSession(acc, "")
	key, _ := rememberKeyFor("anna")
	token := newRememberToken(key, "anna", sess.ExpiresAt)

	u, _ := database.GetUserByName(new("anna"))
	post(HandleUserPassword, url.Values{"id": {strconv.FormatUint(uint64(u.ID), 10)}, "password": {"another123"}})

	if _, ok := sessionStore.getSession(sess.ID); ok {
		t.Error("session kept after a password reset")
	}

	if key2, _ := rememberKeyFor("anna"); string(key2) == string(key) {
		t.Error("remember key unchanged after a password reset")
	} else if _, ok := parseRememberToken(key2, token, sess.CreatedAt); ok {
		t.Error("old remembered login still valid")
	}

	if _, ok := authenticateUser("anna", "another123"); !ok {
		t.Error("the new password doesn't work")
	}

	post(HandleUserDelete, url.Values{"id": {strconv.FormatUint(uint64(u.ID), 10)}})

	if _, ok := authenticateUser("anna", "another123"); ok {
		t.Error("a deleted account logged in")
	}
}

// TestRequestDefaultList: a requesting user adds to the group's default list
// (add_found_list, else the first list) and to no other.
func TestRequestDefaultList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group := mediaGroup("DE")
	group[0].Lists = append(group[0].Lists, config.MediaListsConfig{Name: "DE Wishes", TemplateList: "listA", TemplateQuality: "HD", Enabled: true})
	group[0].Data[0].AddFoundList = "de wishes"
	useMediaConfig(t, config.MediaConfig{Movies: append(group, mediaGroup("EN")...)})

	requester := &userAccess{Name: "r", Role: database.RoleRequestingUser, Groups: []string{"movie_DE", "movie_EN"}, Features: []string{database.FeatureRequest}}

	if got := getMediaListsByType(requester, "movie"); strings.Join(got, ",") != "DE Wishes,EN List" {
		t.Errorf("offered lists: %v", got)
	}

	if got := getMediaListsByType(adminAccess, "movie"); len(got) != 3 {
		t.Errorf("admin lists: %v", got)
	}

	for form, want := range map[string]bool{"movie_list=DE+Wishes": true, "movie_list=DE+List": false, "movie_list=EN+List": true} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/add/movie", strings.NewReader(form))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		if got := requester.allowsRequestTarget(c); got != want {
			t.Errorf("%s: %v, want %v", form, got, want)
		}
	}
}
