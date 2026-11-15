package api

import (
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// userAccess is what the logged-in account may do: admins everything,
// the other roles view and play the media groups they were given, plus
// searching (searchinguser) or requesting new media (requestinguser).
type userAccess struct {
	Name   string
	Role   string
	Groups []string // media configs (movie_DE); not used for admins
}

var adminAccess = &userAccess{Name: DefaultUsername, Role: database.RoleAdmin}

// accessOf returns the access of the request's session; none (no groups,
// no rights) without one.
func accessOf(c *gin.Context) *userAccess {
	if v, ok := c.Get("session"); ok {
		if s, ok := v.(*Session); ok {
			return s.access()
		}
	}

	return &userAccess{}
}

func (a *userAccess) Admin() bool { return a.Role == database.RoleAdmin }

// CanSearch: may start searches and downloads.
func (a *userAccess) CanSearch() bool {
	return a.Role == database.RoleAdmin || a.Role == database.RoleSearchingUser
}

// CanRequest: may add new media to the wanted lists.
func (a *userAccess) CanRequest() bool {
	return a.Role == database.RoleAdmin || a.Role == database.RoleRequestingUser
}

// AllowsGroup reports whether the account may see the media config.
func (a *userAccess) AllowsGroup(name string) bool {
	return a.Admin() || slices.ContainsFunc(a.Groups, func(g string) bool { return strings.EqualFold(g, name) })
}

// Lists are the list names of the allowed media groups (nil for admins:
// all).
func (a *userAccess) Lists() []string {
	if a.Admin() {
		return nil
	}

	var out []string

	for _, g := range a.Groups {
		if cfgp := config.GetSettingsMedia(g); cfgp != nil {
			for i := range cfgp.Lists {
				out = append(out, cfgp.Lists[i].Name)
			}
		}
	}

	return out
}

// AllowsList reports whether the list belongs to an allowed media group.
func (a *userAccess) AllowsList(listname string) bool {
	return a.Admin() || slices.ContainsFunc(a.Lists(), func(l string) bool { return strings.EqualFold(l, listname) })
}

// listsIn is " IN (?, ...)" over the allowed lists and its arguments; a
// condition that matches nothing without lists.
func (a *userAccess) listsIn() (string, []any) {
	lists := a.Lists()
	if len(lists) == 0 {
		return " IN (NULL)", nil
	}

	args := make([]any, len(lists))
	for i := range lists {
		args[i] = &lists[i]
	}

	return " COLLATE NOCASE IN (?" + strings.Repeat(", ?", len(lists)-1) + ")", args
}

// entryTable is the list entry table of a media kind (movies, series, ...).
func (k *libKind) entryTable() string {
	t, _, _ := strings.Cut(k.From, " ")
	return t
}

// dbColumn is the list entry column referencing the metadata row
// (dbmovie_id, ...).
func (k *libKind) dbColumn() string {
	return strings.TrimSuffix(k.DbTable, "s") + "_id"
}

// allowsEntry reports whether the list entry (movies.id, series.id, ...)
// belongs to an allowed media group.
func (a *userAccess) allowsEntry(k *libKind, id any) bool {
	if a.Admin() {
		return true
	}

	in, args := a.listsIn()

	return database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM "+k.entryTable()+" WHERE id = ? AND listname"+in+")",
		append([]any{id}, args...)...)
}

// allowsDB reports whether a metadata row (dbmovies.id, ...) has a list
// entry in an allowed media group; it may have entries in several groups.
func (a *userAccess) allowsDB(k *libKind, dbID any) bool {
	if a.Admin() {
		return true
	}

	in, args := a.listsIn()

	return database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM "+k.entryTable()+" WHERE "+k.dbColumn()+" = ? AND listname"+in+")",
		append([]any{dbID}, args...)...)
}

// allowsFile reports whether a file row (movie_files.id, ...) belongs to a
// list entry of an allowed media group.
func (a *userAccess) allowsFile(k *libKind, fileID any) bool {
	if a.Admin() {
		return true
	}

	in, args := a.listsIn()

	return database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM "+k.Files+" f INNER JOIN "+k.entryTable()+" e ON e.id = f."+k.FilesFK+
			" WHERE f.id = ? AND e.listname"+in+")",
		append([]any{fileID}, args...)...)
}

// allowsEpisode reports whether a serie_episodes row belongs to an allowed
// media group.
func (a *userAccess) allowsEpisode(episodeID any) bool {
	if a.Admin() {
		return true
	}

	in, args := a.listsIn()

	return database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM serie_episodes se INNER JOIN series e ON e.id = se.serie_id WHERE se.id = ? AND e.listname"+in+")",
		append([]any{episodeID}, args...)...)
}

// userPaths are the pages and calls every account may use (GET, by path or
// path prefix ending in "/"); their handlers limit the data to the
// account's media groups.
var userPaths = []string{
	"/api/admin/library", "/api/admin/library/grid", "/api/admin/library/item", "/api/admin/library/random",
	"/api/admin/library/filters", "/api/admin/library/links", "/api/admin/library/playlist",
	"/api/admin/library/playall", "/api/admin/library/cover/", "/api/admin/library/thumb/", "/api/admin/library/play/",
	"/api/admin/calendar", "/api/admin/calendar/content", "/api/admin/calendar/events",
	"/api/admin/calendar/day-events", "/api/admin/calendar/ical",
}

// requestPaths are the "Add media" page and its search and add calls.
// Only additions of one search result: they become requests an admin
// approves. The bulk additions (an author's books, an artist's albums, a
// music series) and manual entries stay with admins.
var requestPaths = []string{
	"/api/admin/metadata-search", "/api/admin/search/",
	"/api/admin/add/movie", "/api/admin/add/series", "/api/admin/add/book", "/api/admin/add/audiobook",
	"/api/admin/add/album",
}

// requestListFields are the form fields the Add Media calls name their
// target list in, with its media kind (mediaEntriesForKind).
var requestListFields = map[string]string{
	"movie_list": "movie", "manualMovie_list": "movie",
	"series_list": "serie", "manualSeries_list": "serie",
	"book_list": "book", "manualBook_list": "book",
	"audiobook_list": "audiobook", "manualAudiobook_list": "audiobook",
	"music_list": "music",
}

// allowsRequestTarget reports whether an add call names only lists of the
// account's media groups. A list resolves to its media group the way the
// add handlers resolve it (findCfgpAndListID); a call naming none is
// refused.
func (a *userAccess) allowsRequestTarget(c *gin.Context) bool {
	if a.Admin() {
		return true
	}

	found := false

	for field, kind := range requestListFields {
		name := c.PostForm(field)
		if name == "" {
			continue
		}

		// Only the group's default list, the one Add Media offers them.
		cfgp, listid := findCfgpAndListID(name, kind+"_", mediaEntriesForKind(kind))
		if cfgp == nil || listid < 0 || !a.AllowsGroup(cfgp.NamePrefix) ||
			!strings.EqualFold(cfgp.Lists[listid].Name, defaultRequestList(cfgp)) {
			return false
		}

		found = true
	}

	return found
}

func pathIn(path string, paths []string) bool {
	return slices.ContainsFunc(paths, func(p string) bool {
		return path == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(path, p))
	})
}

// allowsPath reports whether the account may call the admin path at all.
func (a *userAccess) allowsPath(method, path string) bool {
	switch {
	case a.Admin():
		return true
	case path == "/api/admin/account":
		return true // own password
	case method == http.MethodGet && pathIn(path, userPaths):
		return true
	case path == "/api/admin/proxy":
		return a.CanSearch() // the proxied path is checked by the proxy
	case pathIn(path, requestPaths):
		return a.CanRequest()
	}

	return false
}

// addsMedia reports whether the call adds media (needs allowsRequestTarget).
func addsMedia(path string) bool {
	return strings.HasPrefix(path, "/api/admin/add/")
}

// authorizeAccount keeps non-admin accounts on the pages of their role: a
// page load elsewhere goes to the Library, anything else gets 403.
func authorizeAccount(c *gin.Context) {
	a := accessOf(c)
	if a.allowsPath(c.Request.Method, c.Request.URL.Path) &&
		(!addsMedia(c.Request.URL.Path) || a.allowsRequestTarget(c)) {
		return
	}

	if c.Request.Method == http.MethodGet && c.GetHeader("HX-Request") == "" && c.GetHeader("X-Requested-With") == "" {
		c.Redirect(http.StatusFound, homePath(a))
	} else {
		c.String(http.StatusForbidden, "not allowed for this account")
	}

	c.Abort()
}

// calendarEvents keeps the calendar entries of the account's media groups;
// an event left without entries is dropped.
func (a *userAccess) calendarEvents(events []calendarEvent) []calendarEvent {
	if a.Admin() {
		return events
	}

	lists := a.Lists()
	allowed := func(en calendarEntry) bool {
		return slices.ContainsFunc(lists, func(l string) bool { return strings.EqualFold(l, en.List) })
	}

	today := calendarToday()
	out := events[:0]

	for i := range events {
		e := events[i]
		if !slices.ContainsFunc(e.Entries, func(en calendarEntry) bool { return !allowed(en) }) {
			out = append(out, e)
			continue
		}

		e.Entries = slices.DeleteFunc(slices.Clone(e.Entries), func(en calendarEntry) bool { return !allowed(en) })
		if len(e.Entries) == 0 {
			continue
		}

		e.settle(today)
		out = append(out, e)
	}

	return out
}

// playLibKinds are the media kinds of the player's file kinds.
var playLibKinds = map[string]string{"movie": "movie", "episode": "series", "track": "album", "audiobook": "audiobook"}

// playFileFor is playFile for the request's :kind/:id, when the file
// belongs to one of the account's media groups.
func playFileFor(c *gin.Context) (string, playKind, bool) {
	kind, id := c.Param("kind"), c.Param("id")

	n, err := strconv.ParseUint(id, 10, 64)
	if k := libKindByType(playLibKinds[kind]); err != nil || k == nil || !accessOf(c).allowsFile(k, &n) {
		return "", playKind{}, false
	}

	return playFile(kind, id)
}

// userProxySearches are the proxy paths a searching account may use: the
// search of one list entry (or episode), with the kind it belongs to.
var userProxySearches = []struct {
	re   *regexp.Regexp
	kind string // libKind type; "episode": a serie_episodes id
}{
	{regexp.MustCompile(`^/api/movies/search/list/(\d+)$`), "movie"},
	{regexp.MustCompile(`^/api/series/episodes/search/list/(\d+)$`), "episode"},
	{regexp.MustCompile(`^/api/series/search/id/(\d+)$`), "series"},
	{regexp.MustCompile(`^/api/music/search/list/(\d+)$`), "album"},
	{regexp.MustCompile(`^/api/audiobooks/search/list/(\d+)$`), "audiobook"},
	{regexp.MustCompile(`^/api/books/search/list/(\d+)$`), "book"},
}

// allowsProxyPath reports whether a non-admin may call the proxied API
// path: only searching accounts, only the search of one entry of their
// media groups (no bulk searches, refreshes, queue or scheduler).
func (a *userAccess) allowsProxyPath(path string) bool {
	if a.Admin() {
		return true
	}

	if !a.CanSearch() {
		return false
	}

	for _, s := range userProxySearches {
		m := s.re.FindStringSubmatch(path)
		if m == nil {
			continue
		}

		id, err := strconv.ParseUint(m[1], 10, 64)
		if err != nil {
			return false
		}

		if s.kind == "episode" {
			return a.allowsEpisode(&id)
		}

		return a.allowsEntry(libKindByType(s.kind), &id)
	}

	return false
}

// accName is the account's name for the browser (the built-in admin:
// "admin").
func accName(acc *userAccess) string {
	if acc == nil || acc.Name == "" {
		return DefaultUsername
	}

	return acc.Name
}
