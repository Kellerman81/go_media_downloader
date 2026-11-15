package api

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The Library page shows the entries of one media group as a cover grid or
// a list, with search, filter, sort and paging, and a detail dialog per entry.

// libPageSizes are the "per page" choices of the grid; 0 is all.
var libPageSizes = []int{100, 200, 500, 1000, 0}

// libDefaultPageSize is the page size without a choice.
const libDefaultPageSize = 100

// libKind describes the tables of one media type.
type libKind struct {
	Type      string // movie, series, album, audiobook, book (as in coverSources)
	Prefix    string // media group name prefix
	Label     string
	Icon      string
	Select    string // id, db_id, title, sub, year, overview, listname, missing, have
	From      string
	FilesFK   string // files/history column referencing the list entry
	Files     string
	History   string
	DbTable   string // metadata table, edited next to the list entry
	Square    bool   // album and audiobook covers
	SearchURL string // admin proxy search path, + list entry id
	Genre     string // genre column expression ("" = no genres)
	// Incomplete is the filter "metadata incomplete", over the grid columns.
	Incomplete string
	Resolution bool // files have a resolution (filter by it)
}

var libKinds = []libKind{
	{
		Type: "movie", DbTable: "dbmovies", Prefix: "movie_", Label: "Movies", Icon: "fa-film",
		Select: `e.id, d.id AS db_id, d.title, '' AS sub, COALESCE(d.year, 0) AS year, COALESCE(d.overview, '') AS overview,
			e.listname, e.missing, EXISTS (SELECT 1 FROM movie_files f WHERE f.movie_id = e.id) AS have, e.dont_search`,
		From:    "movies e INNER JOIN dbmovies d ON d.id = e.dbmovie_id",
		FilesFK: "movie_id", Files: "movie_files", History: "movie_histories",
		SearchURL: "/api/movies/search/list/", Genre: "d.genres", Resolution: true,
		Incomplete: "(title = '' OR year = 0 OR overview = '')",
	},
	{
		Type: "series", DbTable: "dbseries", Prefix: "serie_", Label: "Series", Icon: "fa-tv",
		Select: `e.id, d.id AS db_id, d.seriename AS title, COALESCE(d.network, '') AS sub,
			CAST(COALESCE(substr(d.firstaired, 1, 4), '0') AS INTEGER) AS year, COALESCE(d.overview, '') AS overview,
			e.listname, ms.serie_id IS NOT NULL AS missing, hv.serie_id IS NOT NULL AS have, e.dont_search`,
		// serie_episodes and serie_episode_files have no index on serie_id:
		// each is grouped once instead of scanned per series (1.4 s -> 25 ms).
		From: `series e INNER JOIN dbseries d ON d.id = e.dbserie_id
			LEFT JOIN (SELECT DISTINCT serie_id FROM serie_episodes WHERE missing = 1) ms ON ms.serie_id = e.id
			LEFT JOIN (SELECT DISTINCT serie_id FROM serie_episode_files) hv ON hv.serie_id = e.id`,
		FilesFK: "serie_id", Files: "serie_episode_files", History: "serie_episode_histories",
		SearchURL: "/api/series/search/id/", Genre: "d.genre", Resolution: true,
		Incomplete: "(title = '' OR overview = '')",
	},
	{
		Type: "album", DbTable: "dbalbums", Prefix: "music_", Label: "Music", Icon: "fa-compact-disc", Square: true,
		Select: `e.id, d.id AS db_id, d.title, COALESCE((SELECT ar.name FROM dbalbum_artists daa INNER JOIN dbartists ar ON ar.id = daa.dbartist_id
			WHERE daa.dbalbum_id = d.id ORDER BY daa.position LIMIT 1), '') AS sub, COALESCE(d.year, 0) AS year, '' AS overview,
			e.listname, e.missing, EXISTS (SELECT 1 FROM album_files f WHERE f.album_id = e.id) AS have, e.dont_search`,
		From:    "albums e INNER JOIN dbalbums d ON d.id = e.dbalbum_id",
		FilesFK: "album_id", Files: "album_files", History: "album_histories",
		SearchURL: "/api/music/search/list/", Genre: "d.genres",
		// A missing year is too common with albums to be worth listing.
		Incomplete: "(title = '' OR sub = '')",
	},
	{
		Type: "audiobook", DbTable: "dbaudiobooks", Prefix: "audiobook_", Label: "Audiobooks", Icon: "fa-headphones", Square: true,
		Select: `e.id, d.id AS db_id, d.title, COALESCE((SELECT au.name FROM dbaudiobook_authors daa INNER JOIN dbauthors au ON au.id = daa.dbauthor_id
			WHERE daa.dbaudiobook_id = d.id ORDER BY daa.position LIMIT 1), '') AS sub, COALESCE(d.year, 0) AS year,
			COALESCE(d.description, '') AS overview, e.listname, e.missing,
			EXISTS (SELECT 1 FROM audiobook_files f WHERE f.audiobook_id = e.id) AS have, e.dont_search`,
		From:    "audiobooks e INNER JOIN dbaudiobooks d ON d.id = e.dbaudiobook_id",
		FilesFK: "audiobook_id", Files: "audiobook_files", History: "audiobook_histories",
		SearchURL:  "/api/audiobooks/search/list/",
		Incomplete: "(title = '' OR sub = '' OR overview = '')",
	},
	{
		Type: "book", DbTable: "dbbooks", Prefix: "book_", Label: "Books", Icon: "fa-book",
		Select: `e.id, d.id AS db_id, d.title, COALESCE((SELECT name FROM dbauthors WHERE id = d.dbauthor_id), '') AS sub,
			COALESCE(d.year, 0) AS year, COALESCE(d.description, '') AS overview, e.listname, e.missing,
			EXISTS (SELECT 1 FROM book_files f WHERE f.book_id = e.id) AS have, e.dont_search`,
		From:    "books e INNER JOIN dbbooks d ON d.id = e.dbbook_id",
		FilesFK: "book_id", Files: "book_files", History: "book_histories",
		SearchURL: "/api/books/search/list/", Genre: "d.genres",
		Incomplete: "(title = '' OR sub = '' OR year = 0)",
	},
}

func libKindByType(t string) *libKind {
	for i := range libKinds {
		if libKinds[i].Type == t {
			return &libKinds[i]
		}
	}

	return nil
}

// libGroup is a media group of the config.
type libGroup struct {
	Name  string // e.g. movie_EN
	Kind  *libKind
	Lists []string
}

// libraryGroups returns the media groups in config order of type, then name.
func libraryGroups() []libGroup {
	var groups []libGroup

	config.RangeSettingsMedia(func(name string, media *config.MediaTypeConfig) error {
		for i := range libKinds {
			if strings.HasPrefix(name, libKinds[i].Prefix) {
				g := libGroup{Name: name, Kind: &libKinds[i]}
				for j := range media.Lists {
					g.Lists = append(g.Lists, media.Lists[j].Name)
				}

				groups = append(groups, g)

				break
			}
		}

		return nil
	})

	order := func(g libGroup) int {
		return slices.IndexFunc(libKinds, func(k libKind) bool { return k.Type == g.Kind.Type })
	}
	slices.SortFunc(groups, func(a, b libGroup) int {
		if d := order(a) - order(b); d != 0 {
			return d
		}

		return strings.Compare(a.Name, b.Name)
	})

	return groups
}

// libQuery is the state of the grid: group, search, filter, sort, view, page.
type libQuery struct {
	Group, Search, Filter, Sort, View string
	List, Genre                       string // extra filters
	Res, YearFrom, YearTo             int    // resolution id, year range
	Page                              int
	Seed                              int64 // order of sort=random, fixed per page visit so paging is stable
	Rev                               bool  // the sort's other direction (rev=1)
	Size                              int   // entries per page, 0 = all (size=all)
}

var (
	libFilters = [][2]string{{"all", "All"}, {"missing", "Missing"}, {"have", "Downloaded"}, {"incomplete", "Metadata incomplete"}}
	libSorts   = [][2]string{
		{"title", "Title"}, {"added", "Recently added"}, {"downloaded", "Last downloaded"}, {"filed", "Last file added"},
		{"year", "Year"}, {"random", "Random"},
	}
)

func libQueryFrom(c *gin.Context) libQuery {
	q := libQuery{Group: c.Query("group"), Search: strings.TrimSpace(c.Query("q")), Filter: c.Query("filter"),
		Sort: c.Query("sort"), View: c.Query("view")}
	if !accessOf(c).AllowsGroup(q.Group) {
		q.Group = "" // not a media group: every handler refuses it
	}
	q.Page, _ = strconv.Atoi(c.Query("page"))
	q.Seed, _ = strconv.ParseInt(c.Query("seed"), 10, 64)
	q.Rev = c.Query("rev") == "1"

	// Any size up to 1000 works; the page offers libPageSizes.
	q.Size = libDefaultPageSize
	if c.Query("size") == "all" {
		q.Size = 0
	} else if n, err := strconv.Atoi(c.Query("size")); err == nil && n > 0 && n <= 1000 {
		q.Size = n
	}
	q.List, q.Genre = c.Query("list"), strings.TrimSpace(c.Query("genre"))
	q.Res, _ = strconv.Atoi(c.Query("res"))
	q.YearFrom, _ = strconv.Atoi(c.Query("year_from"))
	q.YearTo, _ = strconv.Atoi(c.Query("year_to"))

	if q.Page < 1 {
		q.Page = 1
	}

	// Opening the page shows what is there to watch or listen to.
	if !slices.ContainsFunc(libFilters, func(f [2]string) bool { return f[0] == q.Filter }) {
		q.Filter = "have"
	}

	if !slices.ContainsFunc(libSorts, func(s [2]string) bool { return s[0] == q.Sort }) {
		q.Sort = "title"
	}

	if q.View != "list" && q.View != "cards" && q.View != "episodes" {
		q.View = "grid"
	}

	return q
}

func (q libQuery) values() url.Values {
	return url.Values{"group": {q.Group}, "q": {q.Search}, "filter": {q.Filter}, "sort": {q.Sort}, "view": {q.View},
		"page": {strconv.Itoa(q.Page)}, "seed": {strconv.FormatInt(q.Seed, 10)},
		"list": {q.List}, "genre": {q.Genre}, "res": {strconv.Itoa(q.Res)},
		"year_from": {strconv.Itoa(q.YearFrom)}, "year_to": {strconv.Itoa(q.YearTo)}, "rev": {map[bool]string{true: "1"}[q.Rev]},
		"size": {q.sizeParam()}}
}

func (q libQuery) sizeParam() string {
	if q.Size == 0 {
		return "all"
	}

	return strconv.Itoa(q.Size)
}

// limit is the LIMIT/OFFSET of the query's page; none for all.
func (q libQuery) limit() string {
	if q.Size == 0 {
		return ""
	}

	return " LIMIT " + strconv.Itoa(q.Size) + " OFFSET " + strconv.Itoa((q.Page-1)*q.Size)
}

// capacity is the row slice to allocate for a page.
func (q libQuery) capacity() uint {
	if q.Size == 0 {
		return 1000
	}

	return uint(q.Size)
}

// libSortControl is the sort select with the direction button next to it.
func libSortControl(q libQuery, sorts [][2]string) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(sorts))
	for _, o := range sorts {
		opts = append(opts, html.Option(html.Value(o[0]), gomponents.If(o[0] == q.Sort, html.Selected()), gomponents.Text(o[1])))
	}

	return html.Div(html.Class("col-6 col-lg-3"), html.Div(html.Class("input-group flex-nowrap"),
		html.Select(html.Class("form-select"), html.Name("sort"), gomponents.Attr("aria-label", "Sort"),
			gomponents.Attr("onchange", "libShowSortDir()"), gomponents.Group(opts)),
		html.Input(html.Type("hidden"), html.Name("rev"), html.ID("libRev"), html.Value(map[bool]string{true: "1"}[q.Rev])),
		html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("libSortDir"),
			gomponents.Attr("onclick", "libToggleSortDir()"), gomponents.If(q.Sort == "random", html.Disabled()),
			html.Title("Reverse the order"), gomponents.Attr("aria-label", "Reverse the order"),
			html.I(html.Class("fa-solid "+map[bool]string{false: "fa-arrow-down-wide-short", true: "fa-arrow-up-short-wide"}[q.Rev])))))
}

// libOrder is the ORDER BY of a sort: its natural direction (title A-Z,
// dates and years newest first) or, with rev, the other one. Entries without
// a title or date stay last either way; random has no direction.
func libOrder(q libQuery, episodes bool) string {
	type pair struct{ normal, rev string }

	orders := map[string]pair{
		"title": {"title = '', title COLLATE NOCASE", "title = '', title COLLATE NOCASE DESC"},
		"added": {"added DESC, id DESC", "added, id"},
		"year":  {"year = 0, year DESC, title = '', title COLLATE NOCASE", "year = 0, year, title = '', title COLLATE NOCASE"},
		"dates": {"sortkey IS NULL, sortkey DESC, id DESC", "sortkey IS NULL, sortkey, id"},
	}

	if episodes {
		// By series, then season and episode; year is the air date.
		orders["title"] = pair{"sub COLLATE NOCASE, serie_id, season_n, episode_n, ident",
			"sub COLLATE NOCASE DESC, serie_id DESC, season_n DESC, episode_n DESC, ident DESC"}
		orders["year"] = pair{"aired = '', aired DESC, sub COLLATE NOCASE, season_n DESC, episode_n DESC",
			"aired = '', aired, sub COLLATE NOCASE, season_n, episode_n"}
	}

	sort := q.Sort
	switch sort {
	case "random":
		// A shuffle that stays the same for one seed, so pages don't repeat entries.
		return libShuffleOrder(q.Seed)
	case "downloaded", "filed":
		sort = "dates"
	}

	o, ok := orders[sort]
	if !ok {
		o = orders["title"]
	}

	if q.Rev {
		return o.rev
	}

	return o.normal
}

// libRow is one entry of the grid.
type libRow struct {
	ID       uint   `db:"id"`
	DbID     uint   `db:"db_id"`
	Title    string `db:"title"`
	Sub      string `db:"sub"`
	Year     int    `db:"year"`
	Overview string `db:"overview"`
	Listname string `db:"listname"`
	Missing  bool   `db:"missing"`
	Have     bool   `db:"have"`
	Genre    string `db:"genre"`
	// DontSearch: held back from searching (dont_search, e.g. a media
	// request not approved yet): no search buttons.
	DontSearch bool `db:"dont_search"`
	NoTitle    bool `db:"-"` // the metadata has no title; Title is the folder name
}

// libWhere builds the WHERE clause and its arguments of a query.
func libWhere(g *libGroup, q libQuery) (string, []any) {
	if len(g.Lists) == 0 {
		return "1 = 0", nil
	}

	args := make([]any, 0, len(g.Lists)+2)
	for _, l := range g.Lists {
		args = append(args, l)
	}

	where := "e.listname COLLATE NOCASE IN (" + strings.TrimSuffix(strings.Repeat("?,", len(g.Lists)), ",") + ")"

	if sw, sargs := libSearchWhere(q.Search); sw != "" {
		where += " AND " + sw
		args = append(args, sargs...)
	}

	k := g.Kind

	switch q.Filter {
	case "missing":
		where += " AND missing"
	case "have":
		where += " AND have"
	case "incomplete":
		where += " AND " + k.Incomplete
	}

	if q.List != "" && slices.ContainsFunc(g.Lists, func(l string) bool { return strings.EqualFold(l, q.List) }) {
		where += " AND e.listname = ? COLLATE NOCASE"
		args = append(args, q.List)
	}

	if q.Res > 0 && k.Resolution {
		// One pass over the files table (serie_episode_files is large).
		where += " AND id IN (SELECT " + k.FilesFK + " FROM " + k.Files + " WHERE resolution_id = ?)"
		args = append(args, q.Res)
	}

	if q.Genre != "" && k.Genre != "" {
		// Genres are stored as "A, B" or "A,B".
		where += " AND (',' || replace(genre, ', ', ',') || ',') LIKE ?"
		args = append(args, "%,"+q.Genre+",%")
	}

	if q.YearFrom > 0 {
		where += " AND year >= ?"
		args = append(args, q.YearFrom)
	}

	if q.YearTo > 0 {
		where += " AND year <= ?"
		args = append(args, q.YearTo)
	}

	return where, args
}

// libTerm is a word or "quoted phrase" of the search; not: -excluded.
type libTerm struct {
	text string
	not  bool
}

// libSearchGroups splits a search into alternatives (separated by | or the
// word OR), each a list of terms that must all match (separated by spaces,
// & or the word AND).
func libSearchGroups(s string) [][]libTerm {
	var (
		groups [][]libTerm
		cur    []libTerm
	)

	flush := func() {
		if len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
	}

	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }

	for i := 0; i < len(s); {
		c := s[i]

		switch {
		case space(c):
			i++

			continue
		case c == '|':
			flush()
			i++

			continue
		case c == '&':
			i++

			continue
		}

		not := false
		if c == '-' && i+1 < len(s) && !space(s[i+1]) {
			not = true
			i++
		}

		var word string

		if i < len(s) && s[i] == '"' {
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				word, i = s[i+1:], len(s)
			} else {
				word, i = s[i+1:i+1+end], i+2+end
			}
		} else {
			j := i
			for j < len(s) && !space(s[j]) && s[j] != '|' && s[j] != '&' {
				j++
			}

			word, i = s[i:j], j

			if word == "OR" && !not {
				flush()

				continue
			}

			if word == "AND" && !not {
				continue
			}
		}

		if word = strings.TrimSpace(word); word != "" {
			cur = append(cur, libTerm{text: word, not: not})
		}
	}

	flush()

	return groups
}

// libSearchWhere is the condition of a search over titles, artists/authors
// and the plot or description: `bravo | kuschelrock | "top 100"` matches
// any alternative, words of one alternative must all match (any order),
// -word excludes; & and AND join words explicitly. "" when there is nothing
// to search.
func libSearchWhere(search string) (string, []any) {
	groups := libSearchGroups(search)
	if len(groups) == 0 {
		return "", nil
	}

	var args []any

	alts := make([]string, 0, len(groups))

	for _, g := range groups {
		terms := make([]string, 0, len(g))
		for _, t := range g {
			cond := "(COALESCE(title, '') LIKE ? OR COALESCE(sub, '') LIKE ? OR COALESCE(overview, '') LIKE ?)"
			if t.not {
				cond = "NOT " + cond
			}

			like := "%" + t.text + "%"
			terms = append(terms, cond)
			args = append(args, like, like, like)
		}

		alts = append(alts, "("+strings.Join(terms, " AND ")+")")
	}

	return "(" + strings.Join(alts, " OR ") + ")", args
}

// libPlayAllMax caps the files "Play all" queues.
const libPlayAllMax = 2000

// HandleLibraryPlayAll returns the files of everything the grid's search and
// filters match, all pages, as a playlist (/admin/library/playall?<grid
// parameters>[&shuffle=1]): in the grid's order (an album's tracks in order),
// or shuffled across all files.
func HandleLibraryPlayAll(c *gin.Context) {
	q := libQueryFrom(c)

	g := libGroupByName(q.Group)
	if g == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown media group"})
		return
	}

	eg := *g
	episodes := q.View == "episodes" && g.Kind.Type == "series"

	if episodes {
		eg.Kind = &libEpisodeKind
	}

	k := eg.Kind

	kind := libPlayKinds[k.Type]
	if episodes {
		kind = "episode"
	}

	if kind == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "these can't be played"})
		return
	}

	where, args := libWhere(&eg, q)
	sortKey, join := libSortJoin(k, q.Sort)

	fileOrder := "m.rn, " + cmp.Or(libFileOrder[k.Type], "location")
	if c.Query("shuffle") == "1" {
		fileOrder = "random()"
	}

	rows := database.StructscanT[struct {
		ID       uint   `db:"id"`
		Location string `db:"location"`
		Title    string `db:"title"`
	}](false, 200,
		"WITH m AS (SELECT id, title, ROW_NUMBER() OVER (ORDER BY "+libOrder(q, episodes)+") AS rn FROM ("+
			libInner(k, sortKey, join)+") e WHERE "+where+") "+
			"SELECT f.id, f.location, COALESCE(m.title, '') AS title FROM m INNER JOIN "+k.Files+" f ON f."+k.FilesFK+" = m.id "+
			"WHERE f.location != '' ORDER BY "+fileOrder+" LIMIT "+strconv.Itoa(libPlayAllMax), args...)

	list := make([]libPlaylistItem, 0, len(rows))
	for _, r := range rows {
		label := path.Base(r.Location)
		if k.Type == "movie" && r.Title != "" {
			label = r.Title
		}

		list = append(list, libPlaylistItem{Kind: kind, ID: r.ID, Label: label})
	}

	entry := g.Name
	if q.Search != "" {
		entry += ": " + q.Search
	}

	c.JSON(http.StatusOK, gin.H{"list": list, "entry": entry, "capped": len(list) == libPlayAllMax})
}

// libSortJoin is the sortkey of the date sorts and its join: the history or
// files table grouped once (max per entry).
func libSortJoin(k *libKind, sort string) (sortKey, join string) {
	switch sort {
	case "downloaded":
		return "sk.d", " LEFT JOIN (SELECT " + k.FilesFK + " AS fk, max(downloaded_at) AS d FROM " + k.History +
			" GROUP BY " + k.FilesFK + ") sk ON sk.fk = e.id"
	case "filed":
		return "sk.d", " LEFT JOIN (SELECT " + k.FilesFK + " AS fk, max(created_at) AS d FROM " + k.Files +
			" GROUP BY " + k.FilesFK + ") sk ON sk.fk = e.id"
	}

	return "NULL", ""
}

// libInner is the derived table the grid queries filter and sort: the
// kind's columns plus genre, added date and sortkey (from extra joins).
func libInner(k *libKind, sortKey, extraFrom string) string {
	genre := "''"
	if k.Genre != "" {
		genre = "COALESCE(" + k.Genre + ", '')"
	}

	return "SELECT " + k.Select + ", " + genre + " AS genre, e.created_at AS added, " + sortKey + " AS sortkey FROM " + k.From + extraFrom
}

// libFillTitles shows the folder of an entry's first file for entries
// whose metadata has no title (seen with albums).
func libFillTitles(k *libKind, rows []libRow) {
	for i := range rows {
		if rows[i].Title != "" {
			continue
		}

		rows[i].NoTitle = true

		loc := database.Getdatarow[string](false, "SELECT location FROM "+k.Files+" WHERE "+k.FilesFK+" = ? AND location != '' LIMIT 1", &rows[i].ID)
		if dir := path.Base(path.Dir(strings.ReplaceAll(loc, "\\", "/"))); loc != "" && dir != "." && dir != "/" {
			rows[i].Title = dir
		} else {
			rows[i].Title = "(no title)"
		}
	}
}

// libraryRows returns one page of a group and the total count.
func libraryRows(g *libGroup, q libQuery) ([]libRow, int) {
	where, args := libWhere(g, q)
	k := g.Kind
	sortKey, join := libSortJoin(k, q.Sort)
	inner := libInner(k, sortKey, join)

	order := libOrder(q, false)

	total := database.Getdatarow[int](false, "SELECT count() FROM ("+inner+") e WHERE "+where, args...)
	rows := database.StructscanT[libRow](false, q.capacity(),
		"SELECT id, db_id, title, sub, year, overview, listname, missing, have, genre, dont_search FROM ("+inner+") e WHERE "+where+
			" ORDER BY "+order+q.limit(),
		args...)

	libFillTitles(k, rows)

	return rows, total
}

// libEpisodeKind lists the episodes of series groups one by one (view
// "episodes"): title is the episode's, sub the series name.
var libEpisodeKind = libKind{
	Type: "episode", Label: "Episodes", Icon: "fa-tv",
	Select: `e.id, s.id AS serie_id, d.id AS db_id, COALESCE(de.title, '') AS title, d.seriename AS sub,
		COALESCE(de.identifier, '') AS ident, CAST(COALESCE(substr(de.first_aired, 1, 4), '0') AS INTEGER) AS year,
		COALESCE(substr(de.first_aired, 1, 10), '') AS aired, COALESCE(de.overview, '') AS overview, s.listname, e.missing,
		EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = e.id) AS have,
		CAST(de.season AS INTEGER) AS season_n, CAST(de.episode AS INTEGER) AS episode_n`,
	From: `serie_episodes e INNER JOIN series s ON s.id = e.serie_id INNER JOIN dbseries d ON d.id = s.dbserie_id
		INNER JOIN dbserie_episodes de ON de.id = e.dbserie_episode_id`,
	FilesFK: "serie_episode_id", Files: "serie_episode_files", History: "serie_episode_histories",
	Genre: "d.genre", Resolution: true, Incomplete: "(title = '' OR overview = '')",
}

// libEpRow is one episode of the episode view.
type libEpRow struct {
	ID       uint   `db:"id"`
	SerieID  uint   `db:"serie_id"`
	DbID     uint   `db:"db_id"`
	Title    string `db:"title"`
	Sub      string `db:"sub"`
	Ident    string `db:"ident"`
	Aired    string `db:"aired"`
	Overview string `db:"overview"`
	Missing  bool   `db:"missing"`
	Have     bool   `db:"have"`
	FileID   uint   `db:"file_id"`
}

// libraryEpisodeRows returns one page of the episodes of a series group,
// with the grid's search, filters and sorts.
func libraryEpisodeRows(g *libGroup, q libQuery) ([]libEpRow, int) {
	eg := *g
	eg.Kind = &libEpisodeKind
	k := eg.Kind

	where, args := libWhere(&eg, q)
	sortKey, join := libSortJoin(k, q.Sort)
	inner := libInner(k, sortKey, join)

	order := libOrder(q, true)

	total := database.Getdatarow[int](false, "SELECT count() FROM ("+inner+") e WHERE "+where, args...)
	rows := database.StructscanT[libEpRow](false, q.capacity(),
		"SELECT id, serie_id, db_id, title, sub, ident, aired, overview, missing, have, "+
			"COALESCE((SELECT f.id FROM serie_episode_files f WHERE f.serie_episode_id = e.id ORDER BY f.id LIMIT 1), 0) AS file_id "+
			"FROM ("+inner+") e WHERE "+where+
			" ORDER BY "+order+q.limit(),
		args...)

	return rows, total
}

// libEpisodeItem is an episode in the episode view: it opens its series,
// and plays the episode from its button.
func libEpisodeItem(r *libEpRow, order int) gomponents.Node {
	head := r.Sub
	if r.Ident != "" {
		head += " · " + r.Ident
	}

	label := head
	if r.Title != "" {
		label += " · " + r.Title
	}

	var badge gomponents.Node

	switch {
	case r.Missing:
		badge = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("Missing"))
	case r.Have:
		badge = html.Span(html.Class("badge bg-success"), gomponents.Text("Downloaded"))
	}

	overview := libShortOverview(&libRow{Overview: r.Overview})

	return html.Div(html.Class("list-group-item list-group-item-action d-flex gap-3 align-items-start"),
		html.Role("button"), html.TabIndex("0"),
		gomponents.Attr("onclick", fmt.Sprintf("openLibraryItem('series', %d)", r.SerieID)),
		gomponents.Attr("onkeydown", fmt.Sprintf("if(event.key==='Enter'){openLibraryItem('series', %d)}", r.SerieID)),
		html.Img(html.Class("rounded flex-shrink-0 lib-thumb"), html.Src("/api/admin/library/cover/series/"+strconv.FormatUint(uint64(r.DbID), 10)),
			html.Alt(""), gomponents.Attr("loading", "lazy")),
		html.Div(html.Class("flex-grow-1"), html.Style("min-width:0"),
			html.Div(html.Class("d-flex gap-2 align-items-center"),
				gomponents.If(r.FileID > 0, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success py-0"), html.Title("Play"),
					html.Data("play-kind", "episode"), html.Data("play-id", strconv.FormatUint(uint64(r.FileID), 10)),
					html.Data("play-list", "grid"), html.Data("play-order", strconv.Itoa(order)), html.Data("play-label", label),
					gomponents.Attr("onclick", "event.stopPropagation(); libPlayGrid(this)"),
					html.I(html.Class("fa-solid fa-play")))),
				html.Strong(html.Class("text-truncate"), html.Title(head), gomponents.Text(head)), badge),
			html.Div(html.Class("text-truncate"), html.Title(r.Title), gomponents.Text(r.Title)),
			gomponents.If(r.Aired != "", html.Div(html.Class("small text-muted"), gomponents.Text("Aired "+r.Aired))),
			gomponents.If(overview != "", html.P(html.Class("small mb-0 mt-1"), gomponents.Text(overview))),
		),
	)
}

func renderLibraryEpisodes(g *libGroup, q libQuery) gomponents.Node {
	rows, total := libraryEpisodeRows(g, q)
	if total == 0 {
		return html.P(html.Class("text-muted text-center py-5"), gomponents.Text("Nothing found."))
	}

	items := make([]gomponents.Node, 0, len(rows))
	for i := range rows {
		items = append(items, libEpisodeItem(&rows[i], i))
	}

	return gomponents.Group([]gomponents.Node{
		libCountLine(q, total, "episodes", true),
		html.Div(html.Class("list-group shadow-sm"), gomponents.Group(items)),
		libPager(q, total),
	})
}

// HandleLibraryRandom picks a random entry matching the grid's search and
// filter (/admin/library/random).
func HandleLibraryRandom(c *gin.Context) {
	q := libQueryFrom(c)

	groups := libraryGroups()
	idx := slices.IndexFunc(groups, func(g libGroup) bool { return g.Name == q.Group })

	if idx < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown media group"})
		return
	}

	g := &groups[idx]
	where, args := libWhere(g, q)

	id := database.Getdatarow[uint](false,
		"SELECT id FROM ("+libInner(g.Kind, "NULL", "")+") e WHERE "+where+" ORDER BY random() LIMIT 1", args...)
	if id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "nothing matches the search and filter"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"type": g.Kind.Type, "id": id})
}

// libShuffleOrder is an ORDER BY that shuffles the ids the same way for one
// seed, so the pages of sort=random don't repeat entries. SQLite has no
// seeded random(); the ids are hashed with two squarings modulo a prime
// (linear formulas leave a visible stride). All steps stay below 2^62.
func libShuffleOrder(seed int64) string {
	const p = "2147483647"

	k, sd := strconv.FormatInt(1_000_000_007+seed%1_000_000_000, 10), strconv.FormatInt(seed%1_000_000_000, 10)
	x := "((id * " + k + " + " + sd + ") % " + p + ")"
	x = "((" + x + " * " + x + ") % " + p + ")"
	x = "((" + x + " * 48271 + " + sd + ") % " + p + ")"

	return "(" + x + " * " + x + ") % " + p + ", id"
}

// LibraryPage renders the Library page (/admin/library).
func LibraryPage(c *gin.Context) {
	acc := accessOf(c)
	groups := slices.DeleteFunc(libraryGroups(), func(g libGroup) bool { return !acc.AllowsGroup(g.Name) })
	q := libQueryFrom(c)

	if q.Group == "" && len(groups) > 0 {
		q.Group = groups[0].Name
	}

	if q.Seed == 0 {
		q.Seed = rand.Int64N(1 << 30)
	}

	var content gomponents.Node = html.P(html.Class("text-muted"), gomponents.Text("No media groups are configured."))

	if len(groups) > 0 {
		content = renderLibrary(groups, q, getCSRFToken(c))
	}

	// Buttons of actions the account's role doesn't have (the server refuses
	// those actions anyway).
	var hide []string
	if !acc.Admin() {
		hide = append(hide, ".lib-admin")
	}

	if !acc.CanSearch() {
		hide = append(hide, ".lib-search")
	}

	renderPageIn(c, "Library", false, html.Div(
		html.Class("config-section-enhanced"),
		gomponents.If(len(hide) > 0, html.StyleEl(gomponents.Raw(strings.Join(hide, ", ")+" { display: none !important; }"))),
		renderEnhancedPageHeader("fa-solid fa-photo-film", "Library",
			"Browse the movies, series, albums, audiobooks and books of your media groups."),
		content,
	))
}

func renderLibrary(groups []libGroup, q libQuery, csrfToken string) gomponents.Node {
	tabs := make([]gomponents.Node, 0, len(groups))
	for _, g := range groups {
		class := "nav-link"
		if g.Name == q.Group {
			class += " active"
		}

		// Switching groups only reloads the grid, so the player keeps playing.
		tabs = append(tabs, html.Li(html.Class("nav-item"), html.A(html.Class(class),
			html.Href("/api/admin/library?group="+url.QueryEscape(g.Name)), html.Data("group", g.Name), html.Data("kind", g.Kind.Type),
			gomponents.Attr("onclick", "return libSwitchGroup(event, this)"),
			html.I(html.Class("fa-solid "+g.Kind.Icon+" me-1")), gomponents.Text(g.Name))))
	}

	selectOf := func(name, label string, opts [][2]string, current string) gomponents.Node {
		nodes := make([]gomponents.Node, 0, len(opts))
		for _, o := range opts {
			nodes = append(nodes, html.Option(html.Value(o[0]), gomponents.If(o[0] == current, html.Selected()), gomponents.Text(o[1])))
		}

		return html.Div(html.Class("col-6 col-lg-2"),
			html.Select(html.Class("form-select"), html.Name(name), gomponents.Attr("aria-label", label), gomponents.Group(nodes)))
	}

	cur := libGroupByName(q.Group)
	series := cur != nil && cur.Kind.Type == "series"

	viewBtn := func(view, icon, label string) gomponents.Node {
		class := "btn btn-outline-secondary"
		// The episode view is only offered for series (libSwitchGroup).
		if view == "episodes" && !series {
			class += " d-none"
		}

		return gomponents.Group([]gomponents.Node{
			html.Input(html.Type("radio"), html.Class("btn-check"), html.Name("view"), html.ID("libView"+view), html.Value(view),
				gomponents.If(q.View == view && (view != "episodes" || series), html.Checked())),
			html.Label(html.Class(class), html.For("libView"+view), html.Title(label),
				html.I(html.Class("fa-solid "+icon))),
		})
	}

	return gomponents.Group([]gomponents.Node{
		html.Ul(html.Class("nav nav-pills mb-3 gap-1"), gomponents.Group(tabs)),
		html.Form(
			html.ID("libForm"), html.Class("card border-0 shadow-sm mb-3"),
			hx.Get("/api/admin/library/grid"), hx.Target("#libGrid"), hx.PushURL("false"),
			hx.Trigger("input delay:350ms from:#libSearch, change"),
			html.Input(html.Type("hidden"), html.Name("group"), html.Value(q.Group)),
			html.Input(html.Type("hidden"), html.Name("seed"), html.Value(strconv.FormatInt(q.Seed, 10))),
			html.Input(html.Type("hidden"), html.Name("size"), html.ID("libSize"), html.Value(q.sizeParam())),
			html.Div(html.Class("card-body row g-2 align-items-center"),
				html.Div(html.Class("col-12 col-lg-3"), html.Div(html.Class("input-group flex-nowrap"),
					html.Input(html.Type("search"), html.Class("form-control"), html.ID("libSearch"), html.Name("q"),
						html.Value(q.Search), html.Placeholder("Search… a & b | c, \"phrase\", -not"),
						html.Title("Words must all match (& or AND joins them explicitly). | or OR: any of the alternatives. \"…\": exact phrase. -word: exclude.\n"+
							"Example: bravo & hits | kuschelrock | \"top 100\" | chart -xmas")),
					// Saved views: search and filters under a name (libSaveView).
					html.Button(html.Type("button"), html.Class("btn btn-outline-secondary dropdown-toggle"), html.ID("libSavedBtn"),
						gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
						html.Title("Saved views"), gomponents.Attr("aria-label", "Saved views"),
						html.I(html.Class("fa-regular fa-bookmark"))),
					html.Ul(html.Class("dropdown-menu dropdown-menu-end"), html.ID("libSavedMenu")))),
				selectOf("filter", "Filter", libFilters, q.Filter),
				libSortControl(q, libSorts),
				html.Div(html.Class("col-12 col-lg-4 d-flex flex-wrap align-items-center justify-content-lg-end gap-2"),
					html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("libSelectBtn"),
						html.Title("Select several entries for search, refresh or delete"), gomponents.Attr("onclick", "libSelectMode()"),
						html.I(html.Class("fa-regular fa-square-check me-1")), gomponents.Text("Select")),
					html.Button(html.Type("button"), html.Class("btn btn-outline-primary"), html.ID("libRandomBtn"),
						html.Title("Open a random entry that matches the search and filter"), gomponents.Attr("onclick", "libRandom()"),
						html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text("Random")),
					html.Div(html.Class("btn-group"), html.Role("group"), gomponents.Attr("aria-label", "View"),
						viewBtn("grid", "fa-grip", "Covers"), viewBtn("cards", "fa-table-cells-large", "Covers with descriptions"),
						viewBtn("list", "fa-list", "List with descriptions"),
						viewBtn("episodes", "fa-list-ol", "Episodes, without grouping by series")),
					// Cover size: more or fewer covers per row (libSetCoverSize).
					html.Span(html.Class("d-inline-flex align-items-center gap-1 text-nowrap"), html.Title("Cover size"),
						html.I(html.Class("fa-regular fa-image small text-muted")),
						html.Input(html.Type("range"), html.Class("lib-size"), html.ID("libCoverSize"), html.Style("width:6rem"),
							html.Min("90"), html.Max("320"), html.Step("10"), html.Value("150"), gomponents.Attr("aria-label", "Cover size"),
							gomponents.Attr("oninput", "libSetCoverSize(+this.value)")),
						html.I(html.Class("fa-regular fa-image text-muted")))),
			),
			renderLibraryExtraFilters(libGroupByName(q.Group), q),
		),
		html.Div(html.ID("libSelBar"), html.Class("lib-selbar d-none"),
			html.Span(html.Class("fw-semibold me-2"), html.ID("libSelCount"), gomponents.Text("0 selected")),
			libSelBtn("libSelectPage()", "btn-outline-secondary", "fa-check-double", "Select all on this page"),
			libSelBtn("libSelectClear()", "btn-outline-secondary", "fa-eraser", "Clear"),
			html.Span(html.Class("flex-grow-1")),
			libSelBtn("libBulk('search')", "btn-primary lib-search", "fa-magnifying-glass", "Search"),
			libSelBtn("libBulk('refresh')", "btn-outline-primary lib-admin", "fa-rotate", "Refresh metadata"),
			libSelBtn("libBulk('delete')", "btn-outline-danger lib-admin", "fa-trash", "Delete entries"),
			libSelBtn("libBulk('deletefiles')", "btn-danger lib-admin", "fa-trash-can", "Delete with files"),
			libSelBtn("libSelectMode(false)", "btn-outline-secondary", "fa-xmark", "Done"),
		),
		html.Div(html.ID("libGrid"), renderLibraryGrid(q)),
		renderLibraryModal(),
		// The player docks at the bottom of the page, outside the dialog, so
		// it keeps playing while the library is browsed.
		playerDock(),
		html.StyleEl(gomponents.Raw(libraryCSS + playerCSS)),
		html.Script(gomponents.Raw(playerJS)),
		html.Script(gomponents.Raw(strings.ReplaceAll(libraryJS, "{{csrf}}", jsString(csrfToken)))),
	})
}

// libGroupByName returns a configured media group.
func libGroupByName(name string) *libGroup {
	groups := libraryGroups()
	if idx := slices.IndexFunc(groups, func(g libGroup) bool { return g.Name == name }); idx >= 0 {
		return &groups[idx]
	}

	return nil
}

// libMaxGenres caps the genre list: music has thousands of free-form genres.
const libMaxGenres = 100

// libGenres lists the most common genres of a group's entries, by name.
func libGenres(g *libGroup) []string {
	if g.Kind.Genre == "" || len(g.Lists) == 0 {
		return nil
	}

	args := make([]any, 0, len(g.Lists))
	for _, l := range g.Lists {
		args = append(args, l)
	}

	return commonGenres(database.GetrowsN[string](false, 500, "SELECT COALESCE("+g.Kind.Genre+", '') FROM "+g.Kind.From+
		" WHERE e.listname COLLATE NOCASE IN ("+strings.TrimSuffix(strings.Repeat("?,", len(args)), ",")+")", args...), libMaxGenres)
}

// commonGenres returns the limit most common genres of comma-separated genre
// lists, by name. Spellings that differ only in case ("Romance", "romance")
// are one genre (the filter matches either), shown capitalised when any
// spelling is, else in the most used one.
func commonGenres(lists []string, limit int) []string {
	type genre struct {
		total     int
		spellings map[string]int
	}

	byKey := map[string]*genre{}

	for _, list := range lists {
		for _, g := range strings.Split(list, ",") {
			if g = strings.TrimSpace(g); g == "" {
				continue
			}

			key := strings.ToLower(g)
			if byKey[key] == nil {
				byKey[key] = &genre{spellings: map[string]int{}}
			}

			byKey[key].total++
			byKey[key].spellings[g]++
		}
	}

	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b string) int {
		if c := byKey[b].total - byKey[a].total; c != 0 {
			return c
		}

		return strings.Compare(a, b)
	})
	keys = keys[:min(len(keys), limit)]
	slices.Sort(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		best, n, upper := "", 0, false
		for s, c := range byKey[k].spellings {
			r, _ := utf8.DecodeRuneInString(s)
			u := unicode.IsUpper(r)
			if best == "" || (u && !upper) || (u == upper && (c > n || (c == n && s < best))) {
				best, n, upper = s, c, u
			}
		}

		out = append(out, best)
	}

	return out
}

// renderLibraryExtraFilters is the second filter row: list, resolution,
// genre and year range, with the options of the group.
func renderLibraryExtraFilters(g *libGroup, q libQuery) gomponents.Node {
	if g == nil {
		return nil
	}

	sel := func(name, label, current string, opts [][2]string) gomponents.Node {
		nodes := []gomponents.Node{html.Option(html.Value(""), gomponents.Text(label))}
		for _, o := range opts {
			nodes = append(nodes, html.Option(html.Value(o[0]), gomponents.If(strings.EqualFold(o[0], current), html.Selected()), gomponents.Text(o[1])))
		}

		return html.Div(html.Class("col-6 col-md-2"),
			html.Select(html.Class("form-select form-select-sm"), html.Name(name), gomponents.Attr("aria-label", label), gomponents.Group(nodes)))
	}

	var cols []gomponents.Node

	if len(g.Lists) > 1 {
		opts := make([][2]string, 0, len(g.Lists))
		for _, l := range g.Lists {
			opts = append(opts, [2]string{l, l})
		}

		cols = append(cols, sel("list", "All lists", q.List, opts))
	}

	if g.Kind.Resolution {
		var opts [][2]string
		for _, r := range database.StructscanT[struct {
			ID   int    `db:"id"`
			Name string `db:"name"`
		}](false, 20, "SELECT id, name FROM qualities q WHERE type = 1 AND EXISTS (SELECT 1 FROM "+g.Kind.Files+
			" f WHERE f.resolution_id = q.id) ORDER BY priority DESC") {
			opts = append(opts, [2]string{strconv.Itoa(r.ID), r.Name})
		}

		if len(opts) > 0 {
			cols = append(cols, sel("res", "Any resolution", strconv.Itoa(q.Res), opts))
		}
	}

	if genres := libGenres(g); len(genres) > 0 {
		opts := make([][2]string, 0, len(genres))
		for _, genre := range genres {
			opts = append(opts, [2]string{genre, genre})
		}

		cols = append(cols, sel("genre", "Any genre", q.Genre, opts))
	}

	year := func(name, label string, v int) gomponents.Node {
		value := ""
		if v > 0 {
			value = strconv.Itoa(v)
		}

		return html.Div(html.Class("col-6 col-md-2"), html.Input(html.Type("number"), html.Class("form-control form-control-sm"),
			html.Name(name), html.Placeholder(label), html.Value(value), html.Min("1800"), html.Max("2200"), gomponents.Attr("aria-label", label)))
	}

	cols = append(cols, year("year_from", "From year", q.YearFrom), year("year_to", "To year", q.YearTo))

	return html.Div(html.ID("libFilters2"), html.Class("card-body pt-0 row g-2 align-items-center"), gomponents.Group(cols))
}

// HandleLibraryFilters renders the second filter row of a group
// (/admin/library/filters?group=), for switching groups in place.
func HandleLibraryFilters(c *gin.Context) {
	q := libQueryFrom(c)
	c.String(http.StatusOK, renderComponentToString(renderLibraryExtraFilters(libGroupByName(q.Group), libQuery{})))
}

// HandleLibraryLinks returns the pages of a movie or series in the user's
// Plex/Jellyfin (/admin/library/links?type=&id=, a db* ID). It's asked for
// after the dialog shows, as the first lookup lists the whole library.
func HandleLibraryLinks(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Query("id"), 10, 64)

	var links []metadata.MediaServerLink

	if k := libKindByType(c.Query("type")); k == nil || !accessOf(c).allowsDB(k, &id) {
		c.JSON(http.StatusOK, []metadata.MediaServerLink{})
		return
	}

	switch c.Query("type") {
	case "movie":
		imdb := database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbmovies WHERE id = ?", &id)
		tmdb := database.Getdatarow[int](false, "SELECT COALESCE(moviedb_id, 0) FROM dbmovies WHERE id = ?", &id)
		links = metadata.MediaServerLinks("movie", imdb, tmdb, 0)
	case "series":
		imdb := database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbseries WHERE id = ?", &id)
		tvdb := database.Getdatarow[int](false, "SELECT COALESCE(thetvdb_id, 0) FROM dbseries WHERE id = ?", &id)
		links = metadata.MediaServerLinks("show", imdb, 0, tvdb)
	}

	if links == nil {
		links = []metadata.MediaServerLink{}
	}

	c.JSON(http.StatusOK, links)
}

func libSelBtn(onclick, class, icon, label string) gomponents.Node {
	return html.Button(html.Type("button"), html.Class("btn btn-sm "+class), gomponents.Attr("onclick", onclick),
		html.I(html.Class("fa-solid "+icon+" me-1")), gomponents.Text(label))
}

// libPlaylistItem is a file of a playlist, as lpPlayList takes it.
type libPlaylistItem struct {
	Kind  string `json:"kind"`
	ID    uint   `json:"id"`
	Label string `json:"label"`
}

// HandleLibraryPlaylist returns the files of a list entry to play
// (/admin/library/playlist?type=&id=): movie, album and audiobook take the
// list entry ID, series a serie_episodes ID (that episode's files).
func HandleLibraryPlaylist(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	typ := c.Query("type")

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown entry"})
		return
	}

	if acc := accessOf(c); typ == "series" && !acc.allowsEpisode(&id) ||
		typ != "series" && (libKindByType(typ) == nil || !acc.allowsEntry(libKindByType(typ), &id)) {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown entry"})
		return
	}

	entryID, label := id, ""

	if typ == "series" {
		ep := database.StructscanT[struct {
			SerieID uint   `db:"serie_id"`
			Ident   string `db:"identifier"`
			Title   string `db:"title"`
		}](false, 1, `SELECT se.serie_id, COALESCE(dse.identifier, '') AS identifier, COALESCE(dse.title, '') AS title
			FROM serie_episodes se INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id WHERE se.id = ?`, &id)
		if len(ep) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "unknown episode"})
			return
		}

		entryID, label = uint64(ep[0].SerieID), strings.TrimSpace(ep[0].Ident+" "+ep[0].Title)
	}

	k := libKindByType(typ)
	kind := libPlayKinds[typ]

	var it *libItem
	if k != nil && kind != "" {
		it = loadLibraryItem(k, &entryID)
	}

	if it == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "nothing to play"})
		return
	}

	query := "SELECT id, location, '' AS quality_profile FROM " + k.Files + " WHERE " + k.FilesFK + " = ? ORDER BY " + libFileOrder[k.Type] + " LIMIT 500"
	if typ == "series" {
		query = "SELECT id, location, '' AS quality_profile FROM serie_episode_files WHERE serie_episode_id = ? ORDER BY id"
	}

	list := []libPlaylistItem{}

	for _, f := range database.StructscanT[libFile](false, 50, query, &id) {
		l := path.Base(f.Location)
		if label != "" {
			l = label
		}

		list = append(list, libPlaylistItem{Kind: kind, ID: f.ID, Label: l})
	}

	c.JSON(http.StatusOK, gin.H{"list": list, "entry": it.Title, "cover": libCoverURL(k, &it.libRow)})
}

// playerDock is the element the bottom-bar player (playerJS) lives in.
func playerDock() gomponents.Node {
	return html.Div(html.ID("libPlayer"), html.Class("lp-dock d-none"), html.Role("region"), gomponents.Attr("aria-label", "Player"))
}

// HandleLibraryGrid renders the grid fragment (/admin/library/grid).
func HandleLibraryGrid(c *gin.Context) {
	c.String(http.StatusOK, renderComponentToString(renderLibraryGrid(libQueryFrom(c))))
}

func renderLibraryGrid(q libQuery) gomponents.Node {
	groups := libraryGroups()

	idx := slices.IndexFunc(groups, func(g libGroup) bool { return g.Name == q.Group })
	if idx < 0 {
		return html.P(html.Class("text-muted"), gomponents.Text("Unknown media group."))
	}

	g := &groups[idx]

	// Episodes are a view of series groups only.
	if q.View == "episodes" {
		if g.Kind.Type == "series" {
			return renderLibraryEpisodes(g, q)
		}

		q.View = "grid"
	}

	rows, total := libraryRows(g, q)

	if total == 0 {
		return html.P(html.Class("text-muted text-center py-5"), gomponents.Text("Nothing found."))
	}

	items := make([]gomponents.Node, 0, len(rows))
	for i := range rows {
		switch q.View {
		case "list":
			items = append(items, libListItem(g.Kind, &rows[i]))
		case "cards":
			items = append(items, libDescCard(g.Kind, &rows[i]))
		default:
			items = append(items, libCard(g.Kind, &rows[i]))
		}
	}

	square := map[bool]string{true: " lib-square"}[g.Kind.Square]

	var body gomponents.Node

	switch q.View {
	case "list":
		body = html.Div(html.Class("list-group shadow-sm"), gomponents.Group(items))
	case "cards":
		body = html.Div(html.Class("lib-grid lib-desc"+square), gomponents.Group(items))
	default:
		body = html.Div(html.Class("lib-grid"+square), gomponents.Group(items))
	}

	return gomponents.Group([]gomponents.Node{
		libCountLine(q, total, "entries", libPlayKinds[g.Kind.Type] != ""),
		body,
		libPager(q, total),
	})
}

func libCoverURL(k *libKind, r *libRow) string {
	return "/api/admin/library/cover/" + k.Type + "/" + strconv.FormatUint(uint64(r.DbID), 10)
}

func libStatusBadge(r *libRow) gomponents.Node {
	switch {
	case r.Missing:
		return html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("Missing"))
	case r.Have:
		return html.Span(html.Class("badge bg-success"), gomponents.Text("Downloaded"))
	}

	return nil
}

// libRefreshPaths are the API paths of the metadata refresh per type.
var libRefreshPaths = map[string]string{"movie": "movies", "series": "series", "album": "music", "audiobook": "audiobooks", "book": "books"}

// libOpenAttrs open the entry on click, or select it in selection mode; the
// data attributes carry what the bulk actions call.
func libOpenAttrs(k *libKind, r *libRow) gomponents.Node {
	return gomponents.Group([]gomponents.Node{
		html.Role("button"), html.TabIndex("0"),
		html.Data("lib-type", k.Type), html.Data("lib-id", strconv.FormatUint(uint64(r.ID), 10)),
		html.Data("lib-title", r.Title),
		gomponents.If(!r.DontSearch, html.Data("lib-search", k.SearchURL+strconv.FormatUint(uint64(r.ID), 10))),
		html.Data("lib-refresh", "/api/"+libRefreshPaths[k.Type]+"/refresh/"+strconv.FormatUint(uint64(r.DbID), 10)),
		gomponents.Attr("onclick", fmt.Sprintf("libCardClick(this, %s, %d)", jsString(k.Type), r.ID)),
		gomponents.Attr("onkeydown", fmt.Sprintf("if(event.key==='Enter'){libCardClick(this, %s, %d)}", jsString(k.Type), r.ID)),
	})
}

// libTitleAttrs mark a folder name shown for a missing title.
func libTitleAttrs(r *libRow) gomponents.Node {
	if r.NoTitle {
		return gomponents.Group([]gomponents.Node{html.Class("fst-italic"), html.Title("The metadata has no title; this is the folder name")})
	}

	return html.Title(r.Title)
}

func libCard(k *libKind, r *libRow) gomponents.Node {
	sub := r.Sub
	if sub == "" && r.Year > 0 {
		sub = strconv.Itoa(r.Year)
	}

	return html.Div(html.Class("lib-card"), libOpenAttrs(k, r),
		html.Div(html.Class("lib-cover"),
			html.Img(html.Src(libCoverURL(k, r)), html.Alt(""), gomponents.Attr("loading", "lazy")),
			html.Div(html.Class("lib-badge"), libStatusBadge(r)),
		),
		html.Div(html.Class("lib-title text-truncate"), libTitleAttrs(r), gomponents.Text(r.Title)),
		html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(sub)),
	)
}

// libShortOverview cuts a description for the grid views.
func libShortOverview(r *libRow) string {
	if runes := []rune(r.Overview); len(runes) > 320 {
		return string(runes[:320]) + "…"
	}

	return r.Overview
}

// libMeta is the "artist · year" line of a row.
func libMeta(r *libRow) string {
	if r.Year > 0 {
		return strings.TrimPrefix(r.Sub+" · "+strconv.Itoa(r.Year), " · ")
	}

	return r.Sub
}

// libDescCard is a cover with the list view's text below it.
func libDescCard(k *libKind, r *libRow) gomponents.Node {
	overview := libShortOverview(r)

	return html.Div(html.Class("lib-card lib-desc-card"), libOpenAttrs(k, r),
		html.Div(html.Class("lib-cover"),
			html.Img(html.Src(libCoverURL(k, r)), html.Alt(""), gomponents.Attr("loading", "lazy")),
			html.Div(html.Class("lib-badge"), libStatusBadge(r)),
		),
		html.Div(html.Class("lib-title"), libTitleAttrs(r), gomponents.Text(r.Title)),
		gomponents.If(libMeta(r) != "", html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(libMeta(r)))),
		gomponents.If(overview != "", html.P(html.Class("lib-overview"), gomponents.Text(overview))),
	)
}

func libListItem(k *libKind, r *libRow) gomponents.Node {
	overview := libShortOverview(r)
	meta := libMeta(r)

	return html.Div(html.Class("list-group-item list-group-item-action d-flex gap-3 align-items-start"), libOpenAttrs(k, r),
		html.Img(html.Class("rounded flex-shrink-0 lib-thumb"+map[bool]string{true: " lib-thumb-square"}[k.Square]),
			html.Src(libCoverURL(k, r)), html.Alt(""), gomponents.Attr("loading", "lazy")),
		html.Div(html.Class("flex-grow-1"), html.Style("min-width:0"),
			html.Div(html.Class("d-flex gap-2 align-items-center"),
				html.Strong(html.Class("text-truncate"), libTitleAttrs(r), gomponents.Text(r.Title)), libStatusBadge(r)),
			html.Div(html.Class("small text-muted"), gomponents.Text(meta)),
			gomponents.If(overview != "", html.P(html.Class("small mb-0 mt-1"), gomponents.Text(overview))),
		),
	)
}

// libCountLine is "123 entries" with the per-page choice next to it; the
// choice goes into the form's size field (libSetPageSize).
func libCountLine(q libQuery, total int, noun string, playable bool) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(libPageSizes)+1)
	if !slices.Contains(libPageSizes, q.Size) {
		opts = append(opts, html.Option(html.Value(q.sizeParam()), html.Selected(), gomponents.Text(q.sizeParam())))
	}

	for _, n := range libPageSizes {
		v, label := strconv.Itoa(n), strconv.Itoa(n)
		if n == 0 {
			v, label = "all", "All"
		}

		opts = append(opts, html.Option(html.Value(v), gomponents.If(n == q.Size, html.Selected()), gomponents.Text(label)))
	}

	return html.Div(html.Class("d-flex align-items-center gap-2 text-muted small mb-2"),
		gomponents.Textf("%d %s", total, noun),
		html.Span(gomponents.Text("·")),
		html.Label(html.For("libPageSize"), gomponents.Text("Per page")),
		html.Select(html.Class("form-select form-select-sm w-auto"), html.ID("libPageSize"),
			gomponents.Attr("onchange", "libSetPageSize(this.value)"), gomponents.Group(opts)),
		// Everything found (all pages) as the player's playlist.
		gomponents.If(playable && total > 0, gomponents.Group([]gomponents.Node{
			html.Span(html.Class("flex-grow-1")),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"),
				html.Title(fmt.Sprintf("Play everything found (all pages, up to %d files)", libPlayAllMax)),
				gomponents.Attr("onclick", "libPlayAll(false)"), html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text("Play all")),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"),
				html.Title("Play everything found in random order"),
				gomponents.Attr("onclick", "libPlayAll(true)"), html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text("Shuffle")),
		})))
}

func libPager(q libQuery, total int) gomponents.Node {
	if q.Size == 0 {
		return nil
	}

	pages := (total + q.Size - 1) / q.Size
	if pages <= 1 {
		return nil
	}

	link := func(page int, label string, disabled, active bool) gomponents.Node {
		v := q
		v.Page = page

		class := "page-item"
		if disabled {
			class += " disabled"
		}

		if active {
			class += " active"
		}

		return html.Li(html.Class(class), html.A(html.Class("page-link lib-pager"), html.Href("#"),
			hx.Get("/api/admin/library/grid?"+v.values().Encode()), hx.Target("#libGrid"),
			gomponents.Text(label)))
	}

	nodes := []gomponents.Node{link(q.Page-1, "‹", q.Page <= 1, false)}

	for p := max(1, q.Page-3); p <= min(pages, q.Page+3); p++ {
		nodes = append(nodes, link(p, strconv.Itoa(p), false, p == q.Page))
	}

	nodes = append(nodes, link(q.Page+1, "›", q.Page >= pages, false))

	return html.Nav(html.Class("mt-3"), gomponents.Attr("aria-label", "Pages"),
		html.Ul(html.Class("pagination justify-content-center flex-wrap"), gomponents.Group(nodes)),
		html.P(html.Class("text-center text-muted small"), gomponents.Textf("Page %d of %d", q.Page, pages)))
}

func renderLibraryModal() gomponents.Node {
	// No focus trap: the player bar below the dialog must keep the focus
	// (its quality list closes otherwise).
	return html.Div(html.Class("modal fade"), html.ID("libModal"), html.TabIndex("-1"), gomponents.Attr("aria-labelledby", "libModalTitle"),
		html.Data("bs-focus", "false"),
		html.Div(html.Class("modal-dialog modal-xl modal-dialog-centered modal-dialog-scrollable"),
			html.Div(html.Class("modal-content"),
				html.Div(html.Class("modal-header"),
					html.H5(html.Class("modal-title"), html.ID("libModalTitle")),
					html.Button(html.Type("button"), html.Class("btn-close"), gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"))),
				html.Div(html.Class("modal-body"), html.ID("libModalBody")),
			)))
}

// libPlayKinds maps library types to player kinds (books have no player).
var libPlayKinds = map[string]string{"movie": "movie", "series": "episode", "album": "track", "audiobook": "audiobook"}

// libFileOrder sorts the files of an entry: tracks and parts by disc and number.
var libFileOrder = map[string]string{
	"movie": "location", "series": "location", "book": "location",
	"album":     "COALESCE(disc_number, 0), COALESCE(track_number, 0), location",
	"audiobook": "COALESCE(disc_number, 0), COALESCE(track_number, 0), location",
}

// libPlayButton starts the player on a file; buttons with the same list form
// the playlist, in order.
func libPlayButton(kind string, fileID uint, list string, order int, label string) gomponents.Node {
	return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success py-0 me-1"), html.Title("Play"),
		html.Data("play-kind", kind), html.Data("play-id", strconv.FormatUint(uint64(fileID), 10)),
		html.Data("play-list", list), html.Data("play-order", strconv.Itoa(order)), html.Data("play-label", label),
		gomponents.Attr("onclick", "libPlay(this)"),
		html.I(html.Class("fa-solid fa-play")))
}

// libItem is the detail of one list entry.
type libItem struct {
	libRow
	Kind   *libKind
	Filter string // the grid's filter, applied to the episodes of a series
}

func loadLibraryItem(k *libKind, id *uint64) *libItem {
	rows := database.StructscanT[libRow](false, 1,
		"SELECT "+k.Select+" FROM "+k.From+" WHERE e.id = ?", id)
	if len(rows) == 0 {
		return nil
	}

	libFillTitles(k, rows)

	return &libItem{libRow: rows[0], Kind: k}
}

// HandleLibraryItem renders the detail dialog body (/admin/library/item).
func HandleLibraryItem(c *gin.Context) {
	k := libKindByType(c.Query("type"))
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)

	if k == nil || err != nil || !accessOf(c).allowsEntry(k, &id) {
		c.String(http.StatusNotFound, renderAlert("Unknown entry", "warning"))
		return
	}

	it := loadLibraryItem(k, &id)
	if it == nil {
		c.String(http.StatusNotFound, renderAlert("The entry no longer exists", "warning"))
		return
	}

	it.Filter = c.Query("filter")
	if it.Filter != "have" && it.Filter != "missing" {
		it.Filter = "all"
	}

	c.String(http.StatusOK, renderComponentToString(renderLibraryItem(it)))
}

type libFile struct {
	ID       uint   `db:"id"`
	Location string `db:"location"`
	Quality  string `db:"quality_profile"`
}

type libHistory struct {
	Title   string `db:"title"`
	Indexer string `db:"indexer"`
	When    string `db:"downloaded_at"`
}

type libEpisode struct {
	FileID   uint   `db:"file_id"`
	ID       uint   `db:"id"`
	DbID     uint   `db:"db_id"`
	Ident    string `db:"identifier"`
	Title    string `db:"title"`
	Aired    string `db:"aired"`
	Missing  bool   `db:"missing"`
	HasFiles bool   `db:"have"`
	Source   string `db:"scraper_url"`
	// DontSearch: the episode or its series is held back from searching.
	DontSearch bool `db:"dont_search"`
}

func renderLibraryItem(it *libItem) gomponents.Node {
	k := it.Kind

	files := database.StructscanT[libFile](false, 50,
		"SELECT id, location, COALESCE(quality_profile, '') AS quality_profile FROM "+k.Files+" WHERE "+k.FilesFK+" = ? ORDER BY "+
			libFileOrder[k.Type]+" LIMIT 500", &it.ID)
	history := database.StructscanT[libHistory](false, 25,
		"SELECT title, COALESCE(indexer, '') AS indexer, COALESCE(substr(downloaded_at, 1, 16), '') AS downloaded_at FROM "+
			k.History+" WHERE "+k.FilesFK+" = ? ORDER BY id DESC LIMIT 25", &it.ID)

	// Counts of all rows; the tables show the first ones.
	fileCount := database.Getdatarow[int](false, "SELECT count() FROM "+k.Files+" WHERE "+k.FilesFK+" = ?", &it.ID)

	var (
		episodes          []libEpisode
		episodeCount, all int
	)

	if k.Type == "series" {
		epWhere := map[string]string{
			"have":    " AND EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = se.id)",
			"missing": " AND se.missing = 1",
		}[it.Filter]

		all = database.Getdatarow[int](false, "SELECT count() FROM serie_episodes WHERE serie_id = ?", &it.ID)
		episodeCount = database.Getdatarow[int](false, "SELECT count() FROM serie_episodes se WHERE se.serie_id = ?"+epWhere, &it.ID)
		episodes = database.StructscanT[libEpisode](false, 100,
			`SELECT se.id, dse.id AS db_id, COALESCE(dse.identifier, '') AS identifier, COALESCE(dse.title, '') AS title,
				COALESCE(substr(dse.first_aired, 1, 10), '') AS aired, se.missing, COALESCE(dse.scraper_url, '') AS scraper_url,
				COALESCE((SELECT f.id FROM serie_episode_files f WHERE f.serie_episode_id = se.id ORDER BY f.id LIMIT 1), 0) AS file_id,
				EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = se.id) AS have,
				(se.dont_search OR s.dont_search) AS dont_search
			FROM serie_episodes se INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
				INNER JOIN series s ON s.id = se.serie_id
			WHERE se.serie_id = ?`+epWhere+` ORDER BY CAST(dse.season AS INTEGER) DESC, CAST(dse.episode AS INTEGER) DESC, dse.identifier DESC LIMIT 1000`, &it.ID)
	}

	meta := []string{}
	if it.Sub != "" {
		meta = append(meta, it.Sub)
	}

	if it.Year > 0 {
		meta = append(meta, strconv.Itoa(it.Year))
	}

	meta = append(meta, it.Listname)

	tab := func(id, label string, count int, active bool) gomponents.Node {
		class := "nav-link"
		if active {
			class += " active"
		}

		if id == "libTabEdit" {
			class += " lib-admin"
		}

		return html.Li(html.Class("nav-item"), html.Button(html.Class(class), html.Type("button"),
			gomponents.Attr("data-bs-toggle", "tab"), gomponents.Attr("data-bs-target", "#"+id),
			gomponents.Text(label), gomponents.If(count >= 0, html.Span(html.Class("badge bg-secondary ms-1"), gomponents.Text(strconv.Itoa(count))))))
	}

	pane := func(id string, active bool, children ...gomponents.Node) gomponents.Node {
		class := "tab-pane fade pt-3"
		if active {
			class += " show active"
		}

		return html.Div(html.Class(class), html.ID(id), gomponents.Group(children))
	}

	playKind := libPlayKinds[k.Type]

	fileRows := make([]gomponents.Node, 0, len(files))
	for i, f := range files {
		fileRows = append(fileRows, html.Tr(
			html.Td(html.Class("text-nowrap"), gomponents.If(playKind != "", libPlayButton(playKind, f.ID, "files", i, path.Base(f.Location)))),
			html.Td(html.Class("font-monospace small text-break"), gomponents.Text(f.Location)),
			html.Td(html.Class("small"), gomponents.Text(f.Quality)),
			html.Td(html.Class("text-end text-nowrap"), libPartDeleteButtons("file", k.Type, f.ID, true))))
	}

	histRows := make([]gomponents.Node, 0, len(history))
	for _, h := range history {
		histRows = append(histRows, html.Tr(html.Td(html.Class("small text-nowrap"), gomponents.Text(h.When)),
			html.Td(html.Class("small text-break"), gomponents.Text(h.Title)), html.Td(html.Class("small"), gomponents.Text(h.Indexer))))
	}

	epRows := make([]gomponents.Node, 0, len(episodes))
	for i, e := range episodes {
		status := html.Span(html.Class("badge bg-light text-dark border"), gomponents.Text("—"))

		switch {
		case e.HasFiles:
			status = html.Span(html.Class("badge bg-success"), gomponents.Text("Downloaded"))
		case e.Missing:
			status = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("Missing"))
		}

		var thumb gomponents.Node = html.Td()
		if e.FileID > 0 {
			thumb = html.Td(html.Img(html.Class("lib-ep-thumb rounded"), gomponents.Attr("loading", "lazy"), html.Alt(""),
				html.Src("/api/admin/library/thumb/episode/"+strconv.FormatUint(uint64(e.FileID), 10))))
		}

		epRows = append(epRows, html.Tr(html.Data("ep-i", strconv.Itoa(i)), html.Data("ep-title", e.Title),
			thumb,
			html.Td(html.Class("small text-nowrap font-monospace"), gomponents.Text(e.Ident)),
			html.Td(html.Class("small"), gomponents.Text(e.Title)),
			html.Td(html.Class("small text-nowrap"), gomponents.Text(e.Aired)),
			html.Td(status),
			html.Td(html.Class("text-end text-nowrap"),
				// The list is newest first; the playlist runs oldest first.
				gomponents.If(e.FileID > 0, libPlayButton("episode", e.FileID, "episodes", len(episodes)-i, e.Ident+" "+e.Title)),
				gomponents.If(!e.HasFiles && !e.DontSearch, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary py-0 me-1 lib-search"),
					html.Title("Search this episode"),
					gomponents.Attr("onclick", fmt.Sprintf("libAction('search', %s)", jsString("/api/series/episodes/search/list/"+strconv.FormatUint(uint64(e.ID), 10)))),
					html.I(html.Class("fa-solid fa-magnifying-glass")))),
				gomponents.If(libHTTPURL(e.Source), html.A(html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Href(e.Source),
					html.Target("_blank"), html.Rel("noopener"), html.Title("The page this episode was scraped from"),
					html.I(html.Class("fa-solid fa-globe")))),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1 lib-admin"), html.Title("Edit this episode"),
					gomponents.Attr("onclick", fmt.Sprintf("libEdit(%s, %d, %s, %d, %s)", jsString("serie_episodes"), e.ID,
						jsString("dbserie_episodes"), e.DbID, jsString(e.Ident))),
					html.I(html.Class("fa-solid fa-pen"))),
				libPartDeleteButtons("episode", k.Type, e.ID, e.HasFiles)),
		))
	}

	table := func(head []string, rows []gomponents.Node, total int, empty string) gomponents.Node {
		if len(rows) == 0 {
			return html.P(html.Class("text-muted small"), gomponents.Text(empty))
		}

		var note gomponents.Node
		if total > len(rows) {
			note = html.P(html.Class("text-muted small mt-2 mb-0"), gomponents.Textf("Showing %d of %d.", len(rows), total))
		}

		ths := make([]gomponents.Node, 0, len(head))
		for _, h := range head {
			ths = append(ths, html.Th(html.Class("small"), gomponents.Text(h)))
		}

		return gomponents.Group([]gomponents.Node{html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(gomponents.Group(ths))), html.TBody(gomponents.Group(rows)))), note})
	}

	idStr := strconv.FormatUint(uint64(it.ID), 10)
	refreshURL := map[string]string{"movie": "movies", "series": "series", "album": "music", "audiobook": "audiobooks", "book": "books"}[k.Type]
	searchURL := k.SearchURL + idStr

	if k.Type == "series" {
		searchURL = "/api/series/search/id/" + idStr
	}

	var playFirst gomponents.Node
	if playKind != "" && len(files) > 0 {
		list := "files"
		if k.Type == "series" {
			list = "episodes"
		}

		playFirst = html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"),
			gomponents.Attr("onclick", "libPlayFirst("+jsString(list)+")"),
			html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text("Play"))

		// Series and albums also play shuffled.
		shuffle := map[string]string{"series": "Play the downloaded episodes in random order", "album": "Play the tracks in random order"}[k.Type]
		if k.Type == "album" && len(files) < 2 ||
			k.Type == "series" && !slices.ContainsFunc(episodes, func(e libEpisode) bool { return e.FileID > 0 }) {
			shuffle = ""
		}

		if shuffle != "" {
			playFirst = gomponents.Group([]gomponents.Node{playFirst,
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"),
					html.Title(shuffle),
					gomponents.Attr("onclick", "libPlayRandom("+jsString(list)+")"),
					html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text("Random"))})
		}
	}

	actions := html.Div(html.Class("d-flex flex-wrap gap-2 mt-3"),
		playFirst,
		// Filled with "Open in Plex/Jellyfin" after the dialog shows.
		gomponents.If(k.Type == "movie" || k.Type == "series", html.Span(html.ID("libLinks"), html.Class("d-contents"),
			html.Data("type", k.Type), html.Data("id", strconv.FormatUint(uint64(it.DbID), 10)))),
		gomponents.If(!it.DontSearch, html.Button(html.Type("button"), html.Class("btn btn-sm btn-primary lib-search"),
			gomponents.Attr("onclick", fmt.Sprintf("libAction('search', %s)", jsString(searchURL))),
			html.I(html.Class("fa-solid fa-magnifying-glass me-1")), gomponents.Text(map[bool]string{true: "Search missing episodes", false: "Search"}[k.Type == "series"]))),
		gomponents.If(it.DontSearch, html.Span(html.Class("badge bg-secondary align-self-center"),
			html.Title("Not searched (dont_search): a media request not approved yet, or switched off"),
			html.I(html.Class("fa-solid fa-ban me-1")), gomponents.Text("Not searched"))),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary lib-admin"),
			gomponents.Attr("onclick", fmt.Sprintf("libAction('refresh', %s)", jsString("/api/"+refreshURL+"/refresh/"+strconv.FormatUint(uint64(it.DbID), 10)))),
			html.I(html.Class("fa-solid fa-rotate me-1")), gomponents.Text("Refresh metadata")),
		html.Span(html.Class("flex-grow-1")),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger lib-admin"),
			gomponents.Attr("onclick", fmt.Sprintf("libDelete(%s, %d, false)", jsString(k.Type), it.ID)),
			html.I(html.Class("fa-solid fa-trash me-1")), gomponents.Text("Delete entry")),
		gomponents.If(fileCount > 0, html.Button(html.Type("button"), html.Class("btn btn-sm btn-danger lib-admin"),
			gomponents.Attr("onclick", fmt.Sprintf("libDelete(%s, %d, true)", jsString(k.Type), it.ID)),
			html.I(html.Class("fa-solid fa-trash-can me-1")), gomponents.Text(map[bool]string{true: "Delete entry and file", false: fmt.Sprintf("Delete entry and %d files", fileCount)}[fileCount == 1]))),
	)

	coverClass := "rounded shadow-sm lib-detail-cover"
	if k.Square {
		coverClass += " lib-detail-square"
	}

	overview := it.Overview
	if overview == "" {
		overview = "No description available."
	}

	return html.Div(
		html.Data("title", it.Title),
		html.Div(html.Class("d-flex flex-column flex-md-row gap-4"),
			html.Img(html.Class(coverClass), html.Src(libCoverURL(k, &it.libRow)), html.Alt("")),
			html.Div(html.Class("flex-grow-1"), html.Style("min-width:0"),
				html.Div(html.Class("mb-2 d-flex gap-2 align-items-center flex-wrap"),
					html.Span(html.Class("badge bg-primary"), html.I(html.Class("fa-solid "+k.Icon+" me-1")), gomponents.Text(k.Label)),
					libStatusBadge(&it.libRow)),
				html.Div(html.Class("text-muted small mb-2"), gomponents.Text(strings.Join(meta, " · "))),
				html.P(html.Class("mb-0"), html.Style("white-space:pre-line"), gomponents.Text(overview)),
				actions,
				libExtLinksNode(libExtLinks(it)),
			),
		),
		html.Ul(html.Class("nav nav-tabs mt-4"), html.Role("tablist"),
			gomponents.If(k.Type == "series", tab("libTabEpisodes", "Episodes", episodeCount, true)),
			tab("libTabFiles", "Files", fileCount, k.Type != "series"),
			tab("libTabHistory", "History", len(history), false),
			tab("libTabEdit", "Edit", -1, false),
		),
		html.Div(html.Class("tab-content"),
			gomponents.If(k.Type == "series", pane("libTabEpisodes", true,
				libEpisodeFilterNote(it.Filter, episodeCount, all),
				gomponents.If(len(episodes) > 1, html.Div(html.Class("d-flex align-items-center gap-2 mb-2"),
					html.Label(html.Class("small text-muted"), html.For("libEpSort"), gomponents.Text("Order")),
					html.Div(html.Class("input-group input-group-sm w-auto"),
						html.Select(html.Class("form-select form-select-sm"), html.ID("libEpSort"), gomponents.Attr("onchange", "libSortEpisodes()"),
							html.Option(html.Value("episode"), gomponents.Text("Episode")),
							html.Option(html.Value("title"), gomponents.Text("Title")),
							html.Option(html.Value("random"), gomponents.Text("Random"))),
						html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("libEpDir"),
							html.Title("Reverse the order"), gomponents.Attr("aria-label", "Reverse the order"), gomponents.Attr("onclick", "libToggleEpisodeDir()"),
							html.I(html.Class("fa-solid fa-arrow-down-wide-short")))))),
				table([]string{"", "Episode", "Title", "Aired", "Status", ""}, epRows, episodeCount,
					map[bool]string{true: "No episodes.", false: "No episodes match the filter."}[it.Filter == "all"]))),
			pane("libTabFiles", k.Type != "series", table([]string{"", "File", "Quality", ""}, fileRows, fileCount, "No files.")),
			pane("libTabHistory", false, table([]string{"Downloaded", "Release", "Indexer"}, histRows, len(histRows), "No downloads yet.")),
			pane("libTabEdit", false,
				html.Div(html.ID("libEditForms"),
					html.Data("entry-table", strings.Fields(k.From)[0]), html.Data("entry-id", idStr),
					html.Data("db-table", k.DbTable), html.Data("db-id", strconv.FormatUint(uint64(it.DbID), 10)),
					gomponents.If(k.Type == "series", html.Data("episodes", "1")),
					html.P(html.Class("text-muted small"), gomponents.Text("Loading…")))),
		),
	)
}

// libEpisodeFilterNote says that the episode list follows the grid filter,
// with a button to show all episodes.
func libEpisodeFilterNote(filter string, shown, all int) gomponents.Node {
	if filter == "all" {
		return nil
	}

	label := map[string]string{"have": "downloaded", "missing": "missing"}[filter]

	return html.Div(html.Class("d-flex flex-wrap align-items-center gap-2 small text-muted mb-2"),
		html.I(html.Class("fa-solid fa-filter")),
		gomponents.Textf("Showing the %d %s of %d episodes, like the filter of the grid.", shown, label, all),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-link p-0"), gomponents.Attr("onclick", "libItemFilter('all')"),
			gomponents.Text("Show all episodes")))
}

// libPartDeleteButtons are the delete buttons of an episode or a file row:
// remove the row only, and (when it has files) also delete them on disk.
func libPartDeleteButtons(part, kind string, id uint, hasFiles bool) gomponents.Node {
	call := func(withFiles bool) gomponents.Node {
		return gomponents.Attr("onclick", fmt.Sprintf("libDeletePart(%s, %s, %d, %t, this)", jsString(part), jsString(kind), id, withFiles))
	}

	title := map[string]string{"episode": "Remove this episode from the list (files on disk are kept)", "file": "Remove this file from the list (the file on disk is kept)"}[part]
	titleFiles := map[string]string{"episode": "Delete this episode and its files on disk", "file": "Delete this file on disk"}[part]

	return gomponents.Group([]gomponents.Node{
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger py-0 me-1 lib-admin"), html.Title(title),
			gomponents.Attr("aria-label", title), call(false), html.I(html.Class("fa-solid fa-trash"))),
		gomponents.If(hasFiles, html.Button(html.Type("button"), html.Class("btn btn-sm btn-danger py-0 lib-admin"), html.Title(titleFiles),
			gomponents.Attr("aria-label", titleFiles), call(true), html.I(html.Class("fa-solid fa-trash-can")))),
	})
}

func libHTTPURL(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

// libExtLinks are the pages of an entry on the metadata sites, like the
// calendar shows them (IMDb, TMDB, TheTVDB, Trakt, the scraped site,
// MusicBrainz, Discogs, Audible, Goodreads, ...).
func libExtLinks(it *libItem) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	switch it.Kind.Type {
	case "movie":
		m := database.StructscanT[struct {
			IMDb  string `db:"imdb_id"`
			TMDb  int    `db:"moviedb_id"`
			Trakt int    `db:"trakt_id"`
			URL   string `db:"url"`
		}](false, 1, "SELECT COALESCE(imdb_id, '') AS imdb_id, COALESCE(moviedb_id, 0) AS moviedb_id, COALESCE(trakt_id, 0) AS trakt_id, "+
			"COALESCE(url, '') AS url FROM dbmovies WHERE id = ?", &it.DbID)
		if len(m) == 0 {
			return nil
		}

		links = movieLinks(m[0].IMDb, m[0].TMDb, m[0].Trakt, m[0].URL)
	case "series":
		sr := database.StructscanT[struct {
			IMDb  string `db:"imdb_id"`
			TVDb  int    `db:"thetvdb_id"`
			Trakt int    `db:"trakt_id"`
		}](false, 1, "SELECT COALESCE(imdb_id, '') AS imdb_id, COALESCE(thetvdb_id, 0) AS thetvdb_id, COALESCE(trakt_id, 0) AS trakt_id "+
			"FROM dbseries WHERE id = ?", &it.DbID)
		if len(sr) == 0 {
			return nil
		}

		links = seriesLinks(sr[0].IMDb, sr[0].TVDb, sr[0].Trakt)

		// Scraped series (sites): the site of their newest scraped episode.
		src := database.Getdatarow[string](false, "SELECT COALESCE(scraper_url, '') FROM dbserie_episodes WHERE dbserie_id = ? AND scraper_url != '' ORDER BY id DESC LIMIT 1", &it.DbID)
		if u, err := url.Parse(src); err == nil && libHTTPURL(src) && u.Host != "" {
			add("Source", "fa-solid fa-globe", u.Scheme+"://"+u.Host+"/")
		}
	case "album":
		a := database.StructscanT[AlbumCalendarQuery](false, 1, `SELECT COALESCE(musicbrainz_release_group_id, '') AS mb_group,
			COALESCE(musicbrainz_release_id, '') AS mb_release, COALESCE(discogs_master_id, '') AS discogs_master,
			COALESCE(discogs_release_id, '') AS discogs_release, COALESCE(deezer_id, '') AS deezer, COALESCE(itunes_id, '') AS itunes
			FROM dbalbums WHERE id = ?`, &it.DbID)
		if len(a) == 0 {
			return nil
		}

		a[0].Artist, a[0].Title = it.Sub, it.Title
		if it.NoTitle {
			a[0].Artist = "" // no Last.fm search for a folder name
		}

		return albumLinks(&a[0])
	case "audiobook":
		ab := AudiobookCalendarQuery{Title: it.Title, Author: it.Sub,
			ASIN: database.Getdatarow[string](false, "SELECT COALESCE(asin, '') FROM dbaudiobooks WHERE id = ?", &it.DbID)}

		return audiobookLinks(&ab, audibleRegionOf(it.Listname))
	case "book":
		b := database.StructscanT[BookCalendarQuery](false, 1, `SELECT COALESCE(isbn_13, '') AS isbn, COALESCE(openlibrary_id, '') AS openlibrary,
			COALESCE(goodreads_id, '') AS goodreads FROM dbbooks WHERE id = ?`, &it.DbID)
		if len(b) == 0 {
			return nil
		}

		b[0].Title = it.Title

		return bookLinks(&b[0], it.Sub)
	}

	return links
}

func libExtLinksNode(links []calendarExtLink) gomponents.Node {
	if len(links) == 0 {
		return nil
	}

	nodes := make([]gomponents.Node, 0, len(links))
	for _, l := range links {
		nodes = append(nodes, html.A(html.Class("btn btn-sm btn-outline-secondary"), html.Href(l.URL), html.Target("_blank"), html.Rel("noopener"),
			html.I(html.Class(l.Icon+" me-1")), gomponents.Text(l.Label)))
	}

	return html.Div(html.Class("d-flex flex-wrap gap-2 mt-2"), gomponents.Group(nodes))
}

// HandleLibraryDeletePart deletes one episode (part=episode, a serie_episodes
// ID) or one file row (part=file) of a list entry, with the files on disk
// when files=1 (/admin/library/delete-part). Only IDs are taken, never paths.
func HandleLibraryDeletePart(c *gin.Context) {
	k := libKindByType(c.PostForm("type"))
	part := c.PostForm("part")
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if k == nil || err != nil || (part == "episode" && k.Type != "series") || (part != "episode" && part != "file") {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown episode or file"})
		return
	}

	rowTable, filesQuery := k.Files, "SELECT location FROM "+k.Files+" WHERE id = ?"
	if part == "episode" {
		rowTable, filesQuery = "serie_episodes", "SELECT location FROM serie_episode_files WHERE serie_episode_id = ?"
	}

	if database.Getdatarow[int](false, "SELECT count() FROM "+rowTable+" WHERE id = ?", &id) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "the " + part + " no longer exists"})
		return
	}

	removed := 0

	if c.PostForm("files") == "1" {
		for _, loc := range database.GetrowsN[string](false, 10, filesQuery, &id) {
			if loc == "" {
				continue
			}

			if _, err := scanner.RemoveFile(loc); err != nil {
				logger.Logtype("error", 0).Err(err).Str("file", loc).Msg("Library: delete file")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("a file could not be deleted (%d deleted before); the %s was kept", removed, part),
				})

				return
			}

			removed++
		}
	}

	if part == "episode" {
		// Same rows as DELETE /api/series/episodes/list/:id.
		database.DeleteRow("serie_episode_files", "serie_episode_id = ?", &id)
		database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", &id)
	}

	if _, err := database.DeleteRow(rowTable, "id = ?", &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Logtype("info", 0).Str("type", k.Type).Str("part", part).Uint64(logger.StrID, id).Int("files_deleted", removed).
		Msg("Library: deleted")

	c.JSON(http.StatusOK, gin.H{"deleted": true, "files": removed})
}

// HandleLibraryDelete deletes a list entry, with its files on disk when
// files=1 (/admin/library/delete). Only IDs are taken, never paths.
func HandleLibraryDelete(c *gin.Context) {
	k := libKindByType(c.PostForm("type"))
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if k == nil || err != nil || loadLibraryItem(k, &id) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	removed, failed := 0, 0

	if c.PostForm("files") == "1" {
		for _, loc := range database.GetrowsN[string](false, 100,
			"SELECT location FROM "+k.Files+" WHERE "+k.FilesFK+" = ?", &id) {
			if _, err := scanner.RemoveFile(loc); err != nil {
				failed++

				logger.Logtype("error", 0).Err(err).Str("file", loc).Msg("Library: delete file")

				continue
			}

			removed++
		}

		if failed > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("%d files could not be deleted (%d deleted); the entry was kept", failed, removed),
			})

			return
		}
	}

	if err := deleteLibraryEntry(k, &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Logtype("info", 0).Str("type", k.Type).Uint64(logger.StrID, id).Int("files_deleted", removed).Msg("Library: entry deleted")

	c.JSON(http.StatusOK, gin.H{"deleted": true, "files": removed})
}

// deleteLibraryEntry removes a list entry with its file, history (and for a
// series episode) rows; files on disk stay.
func deleteLibraryEntry(k *libKind, id *uint64) error {
	// Same rows as the per-type list delete endpoints.
	if k.Type == "series" {
		database.DeleteRow("serie_episode_files", "serie_id = ?", id)
		database.DeleteRow("serie_episode_histories", "serie_id = ?", id)
		database.DeleteRow("serie_episodes", "serie_id = ?", id)
	} else {
		database.DeleteRow(k.Files, k.FilesFK+" = ?", id)
		database.DeleteRow(k.History, k.FilesFK+" = ?", id)
	}

	_, err := database.DeleteRow(k.entryTable(), "id = ?", id)

	return err
}

const libraryCSS = `
.lib-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(var(--lib-size,150px),1fr));gap:1rem}
.lib-card{cursor:pointer;min-width:0}
.lib-card:focus-visible{outline:2px solid var(--bs-primary,#3b7ddd);outline-offset:3px;border-radius:.5rem}
.lib-cover{position:relative;aspect-ratio:2/3;border-radius:.5rem;overflow:hidden;background:#e9ecef;box-shadow:0 1px 3px rgba(0,0,0,.15);transition:transform .15s,box-shadow .15s}
.lib-square .lib-cover{aspect-ratio:1/1}
.lib-cover img{width:100%;height:100%;object-fit:cover;display:block}
.lib-card:hover .lib-cover{transform:translateY(-2px);box-shadow:0 6px 16px rgba(0,0,0,.2)}
.lib-badge{position:absolute;top:.4rem;right:.4rem}
.lib-title{font-weight:600;font-size:.9rem;margin-top:.4rem}
.lib-sub{font-size:.8rem;color:#6c757d}
.lib-thumb{width:calc(var(--lib-size,150px) * .375);height:calc(var(--lib-size,150px) * .5625);object-fit:cover;background:#e9ecef;flex-shrink:0}
.lib-thumb-square{height:calc(var(--lib-size,150px) * .375)}
.lib-detail-cover{width:200px;aspect-ratio:2/3;object-fit:cover;align-self:flex-start;background:#e9ecef}
.lib-detail-square{aspect-ratio:1/1}
#libModal .edit-form-header{display:none}
#libModal .lib-edit-section{border:1px solid #dee2e6;border-radius:.5rem;padding:0 1rem 1rem;margin-bottom:1.5rem;scroll-margin-top:1rem}
#libModal .lib-edit-head{font-weight:600;font-size:1.05rem;padding:.75rem 0;margin-bottom:.5rem;border-bottom:1px solid #dee2e6}
.lib-ep-thumb{width:96px;height:54px;object-fit:cover;background:#e9ecef;display:block}
.d-contents{display:contents}
.lib-selbar{position:sticky;top:0;z-index:20;display:flex;flex-wrap:wrap;align-items:center;gap:.4rem;padding:.6rem .75rem;margin-bottom:.75rem;background:var(--bs-body-bg,#fff);border:1px solid var(--bs-primary,#3b7ddd);border-radius:.5rem;box-shadow:0 2px 8px rgba(0,0,0,.1)}
.lib-selecting [data-lib-id]{position:relative}
.lib-selecting [data-lib-id]::before{content:"";position:absolute;top:.45rem;left:.45rem;width:1.4rem;height:1.4rem;border-radius:.3rem;border:2px solid #fff;background:rgba(0,0,0,.35);z-index:3;box-shadow:0 0 0 1px rgba(0,0,0,.3)}
.lib-selecting .list-group-item[data-lib-id]::before{top:50%;transform:translateY(-50%);left:.25rem}
.lib-selecting [data-lib-id].lib-selected::before{content:"\2713";background:var(--bs-primary,#3b7ddd);color:#fff;font-weight:700;text-align:center;line-height:1.1rem}
.lib-selecting [data-lib-id].lib-selected .lib-cover,.lib-selecting .list-group-item.lib-selected{outline:3px solid var(--bs-primary,#3b7ddd);outline-offset:2px}
.lib-desc{grid-template-columns:repeat(auto-fill,minmax(calc(var(--lib-size,150px) + 50px),1fr));gap:1.25rem}
.lib-desc-card .lib-title{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.lib-overview{font-size:.8rem;margin:.3rem 0 0;display:-webkit-box;-webkit-line-clamp:5;-webkit-box-orient:vertical;overflow:hidden}
@media (max-width:575px){.lib-grid{grid-template-columns:repeat(auto-fill,minmax(min(var(--lib-size,110px),42vw),1fr))}.lib-desc{grid-template-columns:repeat(auto-fill,minmax(min(calc(var(--lib-size,110px) + 50px),42vw),1fr))}.lib-detail-cover{width:140px}}
`

const libraryJS = `
var libCsrf = {{csrf}};
var libCurrent = null;

function libReloadGrid() { htmx.trigger('#libForm', 'change'); }

// The episode list of a series follows the grid's filter unless filter is given.
// The open entry is kept in the address (#movie/12), so reloading, the back
// button and bookmarks bring the dialog back.
function openLibraryItem(type, id, filter, fromHistory) {
	if (filter === undefined) { var f = document.querySelector('#libForm [name="filter"]'); filter = f ? f.value : 'all'; }
	libCurrent = { type: type, id: id, filter: filter };
	var hash = '#' + type + '/' + id;
	if (!fromHistory && location.hash !== hash) { history.pushState(null, '', hash); }
	var body = document.getElementById('libModalBody');
	document.getElementById('libModalTitle').textContent = '';
	body.innerHTML = '<div class="text-center p-5"><div class="spinner-border text-primary" role="status"></div></div>';
	bootstrap.Modal.getOrCreateInstance(document.getElementById('libModal')).show();
	fetch('/api/admin/library/item?type=' + encodeURIComponent(type) + '&id=' + id + '&filter=' + encodeURIComponent(filter), { credentials: 'same-origin' })
		.then(function (r) { return r.text(); })
		.then(function (html) {
			body.innerHTML = html;
			var root = body.firstElementChild;
			document.getElementById('libModalTitle').textContent = root && root.dataset.title ? root.dataset.title : '';
			libLoadLinks();
		})
		.catch(function (e) { body.innerHTML = '<div class="alert alert-danger">' + e.message + '</div>'; });
}

// libLoadLinks adds "Open in Plex/Jellyfin" for a movie or series.
function libLoadLinks() {
	var box = document.getElementById('libLinks');
	if (!box) { return; }
	fetch('/api/admin/library/links?type=' + encodeURIComponent(box.dataset.type) + '&id=' + encodeURIComponent(box.dataset.id), { credentials: 'same-origin' })
		.then(function (r) { return r.ok ? r.json() : []; })
		.then(function (links) {
			(links || []).forEach(function (l) {
				var a = document.createElement('a');
				a.className = 'btn btn-sm btn-outline-secondary';
				a.href = l.url; a.target = '_blank'; a.rel = 'noopener';
				a.innerHTML = '<i class="fa-solid fa-arrow-up-right-from-square me-1"></i>';
				a.appendChild(document.createTextNode('Open in ' + l.server));
				box.appendChild(a);
			});
		})
		.catch(function () {});
}

var libHashRe = /^#(movie|series|album|audiobook|book)\/(\d+)$/;

document.getElementById('libModal').addEventListener('hidden.bs.modal', function () {
	if (libHashRe.test(location.hash)) { history.replaceState(null, '', location.pathname + location.search); }
});

window.addEventListener('popstate', function () {
	var m = libHashRe.exec(location.hash);
	if (m) { openLibraryItem(m[1], +m[2], undefined, true); return; }
	var modal = bootstrap.Modal.getInstance(document.getElementById('libModal'));
	if (modal) { modal.hide(); }
});

(function () {
	var m = libHashRe.exec(location.hash);
	if (m) { openLibraryItem(m[1], +m[2], undefined, true); }
})();

// ---- Selecting several entries ----
var libSel = { on: false, items: new Map() };

function libCardClick(el, type, id) {
	if (libSel.on) { libToggleSel(el); return; }
	openLibraryItem(type, id);
}

function libSelectMode(on) {
	libSel.on = on === undefined ? !libSel.on : !!on;
	if (!libSel.on) { libSel.items.clear(); }
	document.getElementById('libGrid').classList.toggle('lib-selecting', libSel.on);
	document.getElementById('libSelBar').classList.toggle('d-none', !libSel.on);
	document.getElementById('libSelectBtn').classList.toggle('active', libSel.on);
	libSelSync();
}

function libSelKey(el) { return el.dataset.libType + ':' + el.dataset.libId; }

function libToggleSel(el) {
	var k = libSelKey(el);
	if (libSel.items.has(k)) { libSel.items.delete(k); } else {
		libSel.items.set(k, { type: el.dataset.libType, id: el.dataset.libId, title: el.dataset.libTitle, search: el.dataset.libSearch, refresh: el.dataset.libRefresh });
	}
	libSelSync();
}

function libSelSync(text) {
	document.querySelectorAll('#libGrid [data-lib-id]').forEach(function (el) { el.classList.toggle('lib-selected', libSel.items.has(libSelKey(el))); });
	document.getElementById('libSelCount').textContent = text || (libSel.items.size + ' selected');
}

function libSelectPage() {
	document.querySelectorAll('#libGrid [data-lib-id]').forEach(function (el) {
		if (!libSel.items.has(libSelKey(el))) { libToggleSel(el); }
	});
}

function libSelectClear() { libSel.items.clear(); libSelSync(); }

// The selection survives paging and filtering.
document.body.addEventListener('htmx:afterSwap', function (e) { if (e.target && e.target.id === 'libGrid') { libSelSync(); } });
// A listener, not hx-on: htmx compiles hx-on with eval, which a CSP may block.
document.body.addEventListener('htmx:afterRequest', function (e) {
	if (e.detail && e.detail.elt && e.detail.elt.classList.contains('lib-pager')) { window.scrollTo({ top: 0, behavior: 'smooth' }); }
});

// libBulk runs an action on every selected entry, one after the other.
function libBulk(action) {
	var items = Array.from(libSel.items.values());
	if (!items.length) { libNotify('error', 'Nothing selected'); return; }
	var what = { search: 'Search', refresh: 'Refresh the metadata of', delete: 'Delete', deletefiles: 'Delete WITH their files on disk' }[action];
	var msg = what + ' ' + items.length + (items.length === 1 ? ' entry' : ' entries') + '?' +
		(action === 'deletefiles' ? ' The files cannot be restored.' : action === 'delete' ? ' Files on disk are kept.' : '');
	confirmAction('Please confirm', msg, function () {
		var ok = 0, failed = 0, i = 0;
		function one(it) {
			if (action === 'search' || action === 'refresh') {
				return gmdApiProxyFetch(action === 'search' ? it.search : it.refresh).then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
			}
			var form = new URLSearchParams({ type: it.type, id: it.id, files: action === 'deletefiles' ? '1' : '0', csrf_token: libCsrf });
			return fetch('/api/admin/library/delete', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
				.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
		}
		(function next() {
			if (i >= items.length) {
				libNotify(failed ? 'error' : 'success', what + ': ' + ok + ' done' + (failed ? ', ' + failed + ' failed' : ''));
				if (action === 'delete' || action === 'deletefiles') { libSel.items.clear(); libReloadGrid(); }
				libSelSync();
				return;
			}
			var it = items[i++];
			libSelSync('Working ' + i + ' of ' + items.length + '…');
			one(it).then(function () { ok++; }, function () { failed++; }).then(next);
		})();
	});
}

// libEdit loads the edit forms of a list entry row and its metadata row
// (the database browser's forms) into the Edit tab.
function libEdit(entryTable, entryID, dbTable, dbID, label) {
	var box = document.getElementById('libEditForms');
	if (!box) { return; }
	// Mark the tab as loaded first: showing it below must not replace these
	// forms with the entry's own (an episode's pencil opens the tab too).
	box.dataset.loaded = '1';
	box.innerHTML = '';
	var sections = [[dbTable, dbID, 'Metadata', 'fa-database'], [entryTable, entryID, 'List entry', 'fa-list']];
	var nav = document.createElement('div');
	nav.className = 'd-flex flex-wrap gap-2 align-items-center mb-3 small';
	nav.innerHTML = '<span class="text-muted">' + (label ? 'Editing ' : 'Jump to') + '</span>';
	if (label) { var b = document.createElement('strong'); b.textContent = label; nav.appendChild(b); nav.appendChild(document.createTextNode(' ·')); }
	box.appendChild(nav);
	if (!label && box.dataset.episodes === '1') {
		var hint = document.createElement('p');
		hint.className = 'small text-muted';
		hint.innerHTML = 'Episodes: use the <i class="fa-solid fa-pen"></i> button of an episode in the Episodes tab.';
		box.appendChild(hint);
	}
	sections.forEach(function (p, n) {
		var id = 'libEditSec' + n;
		var a = document.createElement('a');
		a.href = '#' + id; a.className = 'link-primary'; a.textContent = p[2] + ' (' + p[0] + ')';
		a.addEventListener('click', function (e) { e.preventDefault(); document.getElementById(id).scrollIntoView({ behavior: 'smooth', block: 'start' }); });
		nav.appendChild(a);
		var sec = document.createElement('div');
		sec.className = 'lib-edit-section';
		sec.id = id;
		sec.innerHTML = '<div class="lib-edit-head"><i class="fa-solid ' + p[3] + ' me-2 text-primary"></i><span></span></div><div class="text-muted small">Loading…</div>';
		sec.querySelector('.lib-edit-head span').textContent = p[2] + ' · ' + p[0];
		box.appendChild(sec);
		fetch('/api/admin/tableedit/' + encodeURIComponent(p[0]) + '/' + p[1], { credentials: 'same-origin' })
			.then(function (r) { return r.text(); })
			.then(function (html) {
				sec.querySelector('div.text-muted').outerHTML = html;
				// The lookup selects (e.g. the metadata row an entry points to)
				// load their options like on the database page.
				if (typeof window.initChoicesGlobal === 'function') { window.initChoicesGlobal(); }
			});
	});
	var tab = document.querySelector('[data-bs-target="#libTabEdit"]');
	if (tab) { bootstrap.Tab.getOrCreateInstance(tab).show(); }
}

// Opening the Edit tab loads the entry's forms once.
document.addEventListener('shown.bs.tab', function (e) {
	if (e.target.getAttribute('data-bs-target') !== '#libTabEdit') { return; }
	var box = document.getElementById('libEditForms');
	if (box && !box.dataset.loaded) {
		box.dataset.loaded = '1';
		libEdit(box.dataset.entryTable, box.dataset.entryId, box.dataset.dbTable, box.dataset.dbId, '');
	}
});

// Edit forms inside the dialog save without leaving the page.
document.addEventListener('submit', function (e) {
	var form = e.target;
	if (!form.closest || !form.closest('#libModal')) { return; }
	e.preventDefault();
	var data = new FormData(form);
	// A lookup select still waiting for (or failing) its option list must not
	// blank the reference it holds.
	form.querySelectorAll('select.choices-ajax[data-selected-value]').forEach(function (sel) {
		if (sel.name && !sel.value && sel.dataset.selectedValue) { data.set(sel.name, sel.dataset.selectedValue); }
	});
	// URL-encoded: the update endpoint doesn't parse multipart bodies.
	fetch(form.action, { method: 'POST', credentials: 'same-origin', body: new URLSearchParams(data), headers: { 'X-CSRF-Token': libCsrf } })
		.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok || d.success === false) { throw new Error(d.error || ('HTTP ' + r.status)); } }); })
		.then(function () { libNotify('success', 'Saved'); libReloadGrid(); })
		.catch(function (err) { libNotify('error', 'Saving failed: ' + err.message); });
});

// libRandom opens a random entry of the current group, search and filter.
function libRandom() {
	var params = new URLSearchParams(new FormData(document.getElementById('libForm')));
	fetch('/api/admin/library/random?' + params.toString(), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) { openLibraryItem(d.type, d.id); })
		.catch(function (e) { libNotify('error', e.message); });
}

// libSetCoverSize sets the width of the covers (px) in the cover views: more
// or fewer per row. Remembered in the browser.
function libSetCoverSize(px, keep) {
	var grid = document.getElementById('libGrid'), r = document.getElementById('libCoverSize');
	if (!grid || !r) { return; }
	grid.style.setProperty('--lib-size', px + 'px');
	r.value = px;
	r.style.setProperty('--v', Math.round((px - r.min) / (r.max - r.min) * 100) + '%');
	if (!keep) { lpStore('lib-cover-size', String(px)); }
}
(function () {
	var px = parseInt(lpStore('lib-cover-size'), 10);
	if (px >= 90 && px <= 320) { libSetCoverSize(px, true); } else if (document.getElementById('libCoverSize')) {
		var r = document.getElementById('libCoverSize');
		r.style.setProperty('--v', Math.round((150 - r.min) / (r.max - r.min) * 100) + '%');
	}
})();

// libPlayAll plays everything the search and filters find, all pages
// (shuffled: in random order across all files).
function libPlayAll(shuffle) {
	var params = new URLSearchParams(new FormData(document.getElementById('libForm')));
	if (shuffle) { params.set('shuffle', '1'); }
	fetch('/api/admin/library/playall?' + params.toString(), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (!d.list || !d.list.length) { throw new Error('nothing to play'); }
			if (d.capped) { libNotify('info', 'Playing the first ' + d.list.length + ' files.'); }
			lpPlayList(d.list.map(function (x) { return { kind: x.kind, id: String(x.id), label: x.label }; }), 0, d.entry, '');
		})
		.catch(function (e) { libNotify('error', 'Can\'t play: ' + e.message); });
}

// ---- Saved views ----
// A saved view is the form (group, search, filters, sort, view, page size)
// under a name, kept in this browser. Opening one loads the page with it.
function libSavedViews() {
	try { return JSON.parse(lpStore('lib-saved-views') || '[]') || []; } catch (e) { return []; }
}

function libSavedRender() {
	var menu = document.getElementById('libSavedMenu');
	if (!menu) { return; }
	var views = libSavedViews(), esc = function (t) { var d = document.createElement('div'); d.textContent = t; return d.innerHTML; };
	var html = views.map(function (v, i) {
		return '<li class="d-flex align-items-center"><a class="dropdown-item text-truncate" href="/api/admin/library?' + esc(v.params) + '" title="' + esc(v.name) + '">' + esc(v.name) + '</a>' +
			'<button type="button" class="btn btn-sm btn-link text-danger px-2" title="Delete this saved view" onclick="libDeleteSaved(' + i + ', event)"><i class="fa-solid fa-xmark"></i></button></li>';
	}).join('');
	if (!views.length) { html = '<li><span class="dropdown-item-text small text-muted">No saved views yet.</span></li>'; }
	menu.innerHTML = html + '<li><hr class="dropdown-divider"></li>' +
		'<li><a class="dropdown-item" href="#" onclick="libSaveView(); return false;"><i class="fa-solid fa-plus me-2"></i>Save current view…</a></li>';
}

function libSaveView() {
	var form = document.getElementById('libForm');
	var params = new URLSearchParams(new FormData(form));
	params.delete('seed');
	var name = prompt('Name for this view (search and filters):', params.get('q') || params.get('group') || '');
	if (!name || !(name = name.trim())) { return; }
	var views = libSavedViews().filter(function (v) { return v.name !== name; });
	views.push({ name: name, params: params.toString() });
	views.sort(function (a, b) { return a.name.localeCompare(b.name); });
	lpStore('lib-saved-views', JSON.stringify(views));
	libSavedRender();
	libNotify('success', 'Saved "' + name + '".');
}

function libDeleteSaved(i, e) {
	if (e) { e.preventDefault(); e.stopPropagation(); }
	var views = libSavedViews();
	views.splice(i, 1);
	lpStore('lib-saved-views', JSON.stringify(views));
	libSavedRender();
}
libSavedRender();

// libSetPageSize shows size entries per page ("all" for all), from page 1.
// Remembered in the browser; a page opened without a size uses it.
function libSetPageSize(size, keep) {
	var f = document.getElementById('libSize');
	if (!f) { return; }
	f.value = size;
	if (!keep) { lpStore('lib-page-size', size); }
	htmx.trigger(document.getElementById('libForm'), 'change');
}
(function () {
	var size = lpStore('lib-page-size'), f = document.getElementById('libSize');
	if (!f || !size || size === f.value || /[?&]size=/.test(location.search)) { return; }
	// htmx sets the form up once the page has loaded.
	var apply = function () { libSetPageSize(size, true); };
	if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', apply); } else { apply(); }
})();

// libToggleSortDir reverses the grid's sort; random has no direction.
function libToggleSortDir() {
	var rev = document.getElementById('libRev');
	rev.value = rev.value === '1' ? '' : '1';
	libShowSortDir();
	htmx.trigger(document.getElementById('libForm'), 'change');
}

function libShowSortDir() {
	var form = document.getElementById('libForm'), btn = document.getElementById('libSortDir');
	var rev = document.getElementById('libRev').value === '1';
	btn.disabled = form.querySelector('[name="sort"]').value === 'random';
	btn.innerHTML = '<i class="fa-solid ' + (rev ? 'fa-arrow-up-short-wide' : 'fa-arrow-down-wide-short') + '"></i>';
}

// ---- Episodes of a series in the dialog ----
// The list comes newest first. It sorts by episode, title or at random in
// the browser; the playlist (Play) follows the shown order, except that
// episode order always plays oldest first.
function libShuffle(a) {
	for (var i = a.length - 1; i > 0; i--) { var j = Math.floor(Math.random() * (i + 1)), t = a[i]; a[i] = a[j]; a[j] = t; }
	return a;
}

function libSortEpisodes() {
	var tb = document.querySelector('#libTabEpisodes tbody'), by = document.getElementById('libEpSort');
	if (!tb || !by) { return; }
	var dirBtn = document.getElementById('libEpDir'), rev = dirBtn.dataset.rev === '1';
	var rows = [].slice.call(tb.rows), n = rows.length;
	dirBtn.disabled = by.value === 'random';
	dirBtn.innerHTML = '<i class="fa-solid ' + (rev ? 'fa-arrow-up-short-wide' : 'fa-arrow-down-wide-short') + '"></i>';
	if (by.value === 'random') {
		libShuffle(rows);
	} else {
		rows.sort(function (a, b) {
			var c = by.value === 'title'
				? (a.dataset.epTitle || '').localeCompare(b.dataset.epTitle || '', undefined, { numeric: true, sensitivity: 'base' }) || a.dataset.epI - b.dataset.epI
				: a.dataset.epI - b.dataset.epI;
			return rev ? -c : c;
		});
	}
	rows.forEach(function (r, i) {
		tb.appendChild(r);
		var b = r.querySelector('[data-play-list="episodes"]');
		if (b) { b.dataset.playOrder = by.value === 'episode' ? n - r.dataset.epI : i; }
	});
}

function libToggleEpisodeDir() {
	var b = document.getElementById('libEpDir');
	b.dataset.rev = b.dataset.rev === '1' ? '' : '1';
	libSortEpisodes();
}

// libPlayRandom plays a list (the downloaded episodes of a series, the tracks
// of an album) in shuffled order.
function libPlayRandom(list) {
	var btns = libShuffle(libPlayButtons(list));
	if (!btns.length) { return; }
	var cover = document.querySelector('#libModalBody .lib-detail-cover');
	lpPlayList(btns.map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; }), 0,
		document.getElementById('libModalTitle').textContent, cover ? cover.getAttribute('src') : '');
}

// libSwitchGroup shows another media group without leaving the page.
function libSwitchGroup(e, a) {
	if (e.ctrlKey || e.metaKey || e.shiftKey || e.button === 1) { return true; }
	var form = document.getElementById('libForm');
	form.querySelector('[name="group"]').value = a.dataset.group;
	form.querySelector('[name="q"]').value = '';
	document.querySelectorAll('[data-group]').forEach(function (x) { x.classList.toggle('active', x === a); });
	// The episode view belongs to series groups.
	var series = a.dataset.kind === 'series', epView = document.getElementById('libViewepisodes');
	document.querySelector('label[for="libViewepisodes"]').classList.toggle('d-none', !series);
	if (!series && epView.checked) { document.getElementById('libViewgrid').checked = true; }
	history.replaceState(null, '', a.href);
	libSelectMode(false);
	// The list, genre and resolution filters belong to the group.
	fetch('/api/admin/library/filters?group=' + encodeURIComponent(a.dataset.group), { credentials: 'same-origin' })
		.then(function (r) { return r.text(); })
		.catch(function () { return ''; })
		.then(function (html) {
			var row = document.getElementById('libFilters2');
			if (row && html) { row.outerHTML = html; }
			htmx.trigger(form, 'change');
		});
	return false;
}


function libAction(kind, url) {
	var label = kind === 'refresh' ? 'Refresh the metadata?' : 'Start the search?';
	confirmAction('Please confirm', label, function () {
		gmdApiProxyFetch(url)
			.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok) { throw new Error((d && d.error) || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				if (d && d.accepted) { libNotify('success', 'Search done: ' + d.accepted.length + ' accepted, ' + (d.denied ? d.denied.length : 0) + ' denied'); }
				else { libNotify('success', kind === 'refresh' ? 'Metadata refresh started' : 'Search started'); }
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}

function libItemFilter(filter) { if (libCurrent) { openLibraryItem(libCurrent.type, libCurrent.id, filter); } }

// libDeletePart deletes one episode or file row; the row leaves the table so
// a running player keeps playing.
function libDeletePart(part, type, id, withFiles, btn) {
	var msg = withFiles
		? (part === 'episode' ? 'Delete this episode AND its files on disk?' : 'Delete this file on disk?') + ' The files cannot be restored.'
		: (part === 'episode' ? 'Remove this episode from the list?' : 'Remove this file from the list?') + ' Files on disk are kept.';
	confirmAction('Please confirm', msg, function () {
		var form = new URLSearchParams({ part: part, type: type, id: id, files: withFiles ? '1' : '0', csrf_token: libCsrf });
		fetch('/api/admin/library/delete-part', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				var row = btn && btn.closest('tr');
				if (row) { row.remove(); }
				libNotify('success', withFiles ? d.files + (d.files === 1 ? ' file' : ' files') + ' deleted' : 'Removed from the list');
				libReloadGrid();
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}

function libDelete(type, id, withFiles) {
	var msg = withFiles ? 'Delete this entry AND its files on disk? The files cannot be restored.' : 'Delete this entry from the list? Files on disk are kept.';
	confirmAction('Please confirm', msg, function () {
		var form = new URLSearchParams({ type: type, id: id, files: withFiles ? '1' : '0', csrf_token: libCsrf });
		fetch('/api/admin/library/delete', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				bootstrap.Modal.getOrCreateInstance(document.getElementById('libModal')).hide();
				libNotify('success', withFiles ? 'Entry and ' + d.files + ' files deleted' : 'Entry deleted');
				libReloadGrid();
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}
`

// playerCSS and playerJS are the bottom-bar player, shared by the Library
// and the calendar. The page provides an element #libPlayer (playerDock).
const playerCSS = `
#libPlayer .lp-shuffle.active,#libPlayer .lp-repeat.active{color:#fff;background:var(--bs-primary,#3b7ddd);border-color:var(--bs-primary,#3b7ddd)}
.lp-dock{position:fixed;left:0;right:0;bottom:0;z-index:1065;background:var(--bs-body-bg,#fff);border-top:1px solid #dee2e6;box-shadow:0 -4px 16px rgba(0,0,0,.15);padding:.5rem 1rem}
.lp-bar{display:flex;align-items:center;gap:.75rem}
.lp-tl{position:relative;height:24px;cursor:pointer;touch-action:none;outline:none}
.lp-tl-track{position:absolute;left:0;right:0;top:50%;height:4px;margin-top:-2px;border-radius:2px;background:rgba(128,128,128,.35);overflow:hidden;transition:height .1s,margin .1s}
.lp-tl:hover .lp-tl-track,.lp-tl-drag .lp-tl-track,.lp-tl:focus-visible .lp-tl-track{height:6px;margin-top:-3px}
.lp-tl-buf{position:absolute;top:0;bottom:0;background:rgba(59,125,221,.35)}
.lp-tl-fill{position:absolute;left:0;top:0;bottom:0;background:var(--bs-primary,#3b7ddd)}
.lp-tl-knob{position:absolute;top:50%;width:14px;height:14px;margin:-7px 0 0 -7px;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
.lp-tl:focus-visible .lp-tl-knob{box-shadow:0 0 0 2px var(--bs-body-bg,#fff),0 0 0 4px rgba(59,125,221,.5)}
.lp-tl-hover{position:absolute;top:4px;bottom:4px;width:2px;margin-left:-1px;background:rgba(128,128,128,.8);pointer-events:none}
.lp-tl-tip{position:absolute;bottom:100%;transform:translateX(-50%);background:#212529;color:#fff;font-size:.75rem;line-height:1.2;padding:.15rem .4rem;border-radius:.25rem;white-space:nowrap;pointer-events:none}
.lp-pos,.lp-dur{min-width:3.2em;font-variant-numeric:tabular-nums}
.lp-main{flex:1 1 auto;min-width:0;display:flex;flex-direction:column;gap:.2rem}
.lp-head{min-width:0}
.lp-media{flex:0 0 auto;cursor:pointer}
.lp-media video{width:160px;height:90px;object-fit:contain;display:block}
.lp-cover{width:64px;height:64px;object-fit:cover;border-radius:.375rem;display:block}
.lp-panel{display:none}
.lp-expanded .lp-panel{display:block}
.lp-expanded .lp-bar{flex-wrap:wrap}
.lp-expanded .lp-media{flex-basis:100%}
.lp-expanded .lp-media video{width:100%;height:auto;max-height:55vh}
.lp-expanded .lp-cover{width:160px;height:160px;margin:0 auto}
body.lp-docked{padding-bottom:var(--lp-h,0)}
body.lp-docked .modal-dialog{margin-bottom:calc(var(--lp-h,0px) + var(--bs-modal-margin,1.75rem))}
body.lp-docked .modal-dialog-scrollable{height:calc(100% - var(--bs-modal-margin,1.75rem)*2 - var(--lp-h,0px))}
body.lp-docked .modal-dialog-centered{min-height:calc(100% - var(--bs-modal-margin,1.75rem)*2 - var(--lp-h,0px))}
@media (max-width:575px){.lp-media video{width:96px;height:54px}.lp-cover{width:48px;height:48px}.lp-vol{display:none}}
#libPlayer:fullscreen,#libPlayer.lp-fullwin{background:#000;color:#fff;padding:1rem;display:flex;flex-direction:column;justify-content:center;overflow:auto}
#libPlayer.lp-fullwin{top:0;height:100vh;height:100dvh;z-index:1080;border-top:0;box-shadow:none;padding-bottom:max(1rem,env(safe-area-inset-bottom))}
body.lp-fullwin-on{overflow:hidden}
#libPlayer:fullscreen .lp-bar,#libPlayer.lp-fullwin .lp-bar{flex-wrap:wrap}
#libPlayer:fullscreen .lp-media,#libPlayer.lp-fullwin .lp-media{flex-basis:100%}
#libPlayer:fullscreen video{width:100%;height:auto;max-height:calc(100vh - 8rem)}
#libPlayer.lp-fullwin video{width:100%;height:auto;max-height:calc(100vh - 9rem);max-height:calc(100dvh - 9rem)}
#libPlayer:fullscreen .lp-panel,#libPlayer.lp-fullwin .lp-panel{display:none!important}
#libPlayer:fullscreen .btn-outline-secondary,#libPlayer.lp-fullwin .btn-outline-secondary{color:#fff;border-color:#adb5bd}
#libPlayer.lp-fullwin .text-muted{color:#ced4da!important}
.lp-vol,.lib-size{-webkit-appearance:none;appearance:none;height:4px;border-radius:2px;cursor:pointer;background:linear-gradient(to right,var(--bs-primary,#3b7ddd) var(--v,100%),rgba(128,128,128,.35) var(--v,100%))}
.lp-vol::-webkit-slider-runnable-track,.lib-size::-webkit-slider-runnable-track{height:4px;background:transparent}
.lp-vol::-webkit-slider-thumb,.lib-size::-webkit-slider-thumb{-webkit-appearance:none;width:14px;height:14px;margin-top:-5px;border:0;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
.lp-vol::-moz-range-track,.lib-size::-moz-range-track{height:4px;background:transparent}
.lp-vol::-moz-range-thumb,.lib-size::-moz-range-thumb{width:14px;height:14px;border:0;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
`

const playerJS = `
// ---- Player ----
// The player is a bar docked at the bottom of the page with its own
// controls. Direct files can be seeked by the browser; repackaged and
// converted streams restart at the wanted position (start=seconds) unless it
// is already buffered, so the player keeps its own position (lp.offset).
// Multi-part audiobooks get one timeline across their parts (lp.book), and
// the next entry is loaded ahead so it starts without a gap (lp.pre).
var lp = null;

// lpStore keeps the Library's and the player's settings (saved views, sizes,
// volume, resume positions) per account: browsers are shared. The built-in
// admin still finds what was saved before accounts existed (no prefix).
function lpStore(key, value) {
	var user = window.gmdUser || 'admin', k = 'u:' + user + ':' + key;
	try {
		if (value === undefined) {
			var v = localStorage.getItem(k);
			return v === null && user === 'admin' ? localStorage.getItem(key) : v;
		}
		localStorage.setItem(k, value);
	} catch (e) { return null; }
}

function lpFmt(t) {
	if (!isFinite(t) || t < 0) { t = 0; }
	var h = Math.floor(t / 3600), m = Math.floor(t % 3600 / 60), sec = Math.floor(t % 60);
	return (h ? h + ':' + String(m).padStart(2, '0') : m) + ':' + String(sec).padStart(2, '0');
}

function libPlayFirst(list) {
	var btns = libPlayButtons(list);
	if (btns.length) { libPlay(btns[0]); }
}

// libPlayGrid plays an episode of the episode view, then the ones after it
// on the page.
function libPlayGrid(btn) {
	var btns = [].slice.call(document.querySelectorAll('#libGrid [data-play-list="grid"]'))
		.sort(function (a, b) { return a.dataset.playOrder - b.dataset.playOrder; });
	var list = btns.map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; });
	lpPlayList(list, btns.indexOf(btn), '', '');
}

function libPlayButtons(list) {
	return [].slice.call(document.querySelectorAll('#libModalBody [data-play-list="' + list + '"]'))
		.sort(function (a, b) { return a.dataset.playOrder - b.dataset.playOrder; });
}

function lpBtn(cls, icon, title) {
	return '<button type="button" class="btn btn-sm btn-outline-secondary ' + cls + '" title="' + title + '" aria-label="' + title + '"><i class="fa-solid ' + icon + '"></i></button>';
}

function lpQ(sel) { return document.querySelector('#libPlayer ' + sel); }

function libPlay(btn) {
	var list = libPlayButtons(btn.dataset.playList).map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; });
	var idx = list.findIndex(function (x) { return x.id === btn.dataset.playId; });
	var cover = document.querySelector('#libModalBody .lib-detail-cover');
	lpPlayList(list, idx, document.getElementById('libModalTitle').textContent, cover ? cover.getAttribute('src') : '');
}

// lpPlayList plays a list of files ({kind, id, label}) from index idx in the
// bottom bar; entry and cover describe what the list belongs to.
function lpPlayList(list, idx, entry, cover) {
	lpStop();
	// Shuffle and repeat are for music; speed and sleep timer for audiobooks.
	var music = list.length > 0 && list[0].kind === 'track';
	var book = list.length > 0 && list[0].kind === 'audiobook';
	lp = { list: list, idx: idx < 0 ? 0 : idx, offset: 0, music: music, isBook: book, hist: [], played: {},
		shuffle: music && lpStore('lp-shuffle') === '1', repeat: music ? (lpStore('lp-repeat') || 'off') : 'off' };
	// The entry the playlist belongs to, for the bar and the media keys.
	lp.entry = entry || '';
	lp.cover = cover || '';
	var box = document.getElementById('libPlayer');
	box.classList.remove('d-none', 'lp-expanded');
	var rates = [0.75, 1, 1.25, 1.5, 1.75, 2];
	box.innerHTML =
		'<div class="lp-panel">' +
		'<div class="lp-chapters d-flex flex-wrap gap-1 mb-2 d-none"></div>' +
		'<div class="lp-list list-group mb-2 d-none" style="max-height:30vh;overflow-y:auto;position:relative"></div></div>' +
		'<div class="lp-bar">' +
		'<div class="lp-media"></div>' +
		'<div class="lp-main">' +
		'<div class="d-flex align-items-center gap-2 lp-head">' +
		'<div class="text-truncate"><span class="lp-title fw-semibold"></span> <span class="small text-muted lp-entry"></span></div>' +
		'<span class="small text-muted text-truncate lp-info"></span><span class="small text-muted text-nowrap lp-buf"></span></div>' +
		'<div class="lp-seek d-flex align-items-center gap-2">' +
		'<button type="button" class="btn btn-link btn-sm p-0 text-reset text-decoration-none small lp-pos" title="Jump to a time: click and type it, e.g. 1:23:45">0:00</button>' +
		'<input type="text" class="form-control form-control-sm d-none lp-goto" inputmode="numeric" placeholder="h:mm:ss" aria-label="Jump to time" style="width:6rem">' +
		'<div class="flex-grow-1 lp-tl" role="slider" tabindex="0" aria-label="Position" aria-valuemin="0" title="Click or drag to jump">' +
		'<div class="lp-tl-track"><div class="lp-tl-buf"></div><div class="lp-tl-fill"></div></div>' +
		'<div class="lp-tl-hover d-none"></div><div class="lp-tl-knob"></div><div class="lp-tl-tip d-none"></div></div>' +
		'<span class="small lp-dur">0:00</span></div>' +
		'<div class="d-flex flex-wrap align-items-center gap-1 lp-controls">' +
		lpBtn('lp-prev', 'fa-backward-step', 'Previous (P)') + lpBtn('lp-back', 'fa-rotate-left', 'Back 10 seconds (←)') +
		'<button type="button" class="btn btn-sm btn-primary lp-play" title="Play or pause (Space)" aria-label="Play or pause"><i class="fa-solid fa-play"></i></button>' +
		lpBtn('lp-fwd', 'fa-rotate-right', 'Forward 10 seconds (→)') + lpBtn('lp-next', 'fa-forward-step', 'Next (N)') +
		(music ? lpBtn('lp-shuffle', 'fa-shuffle', 'Shuffle') + lpBtn('lp-repeat', 'fa-repeat', 'Repeat') : '') +
		(book ? '<select class="form-select form-select-sm w-auto lp-rate" title="Speed" aria-label="Speed">' +
			rates.map(function (r) { return '<option value="' + r + '">' + r + '×</option>'; }).join('') + '</select>' +
			'<select class="form-select form-select-sm w-auto lp-sleep" title="Sleep timer" aria-label="Sleep timer">' +
			'<option value="">Sleep: off</option><option value="15">15 min</option><option value="30">30 min</option>' +
			'<option value="45">45 min</option><option value="60">60 min</option>' +
			'<option value="chapter">End of chapter</option><option value="file">End of part</option></select>' : '') +
		'<span class="flex-grow-1"></span>' +
		lpBtn('lp-mute', 'fa-volume-high', 'Mute (M)') +
		'<input type="range" class="lp-vol" style="width:5rem" min="0" max="1" step="0.05" aria-label="Volume">' +
		'<select class="form-select form-select-sm w-auto lp-audio d-none" style="max-width:14rem" title="Audio track" aria-label="Audio track"></select>' +
		'<select class="form-select form-select-sm w-auto lp-subs d-none" style="max-width:14rem" title="Subtitles" aria-label="Subtitles"></select>' +
		'<select class="form-select form-select-sm w-auto lp-quality" style="max-width:22rem" aria-label="Quality"></select>' +
		lpBtn('lp-show-chapters d-none', 'fa-bookmark', 'Show the chapters') +
		lpBtn('lp-show-list d-none', 'fa-list', 'Show the playlist') +
		lpBtn('lp-expand', 'fa-up-right-and-down-left-from-center', 'Bigger or smaller') +
		lpBtn('lp-full', 'fa-expand', 'Full screen (F)') +
		'<button type="button" class="btn btn-sm btn-outline-danger lp-close" title="Close the player" aria-label="Close the player"><i class="fa-solid fa-xmark"></i></button>' +
		'</div></div></div>';
	lpQ('.lp-entry').textContent = lp.entry && lp.entry !== list[lp.idx].label ? '· ' + lp.entry : '';
	lpQ('.lp-expand').addEventListener('click', function () {
		lp.sized = true;
		var cur = lpSize();
		lpSetSize(cur === 'mini' ? 'expanded' : cur === 'expanded' && !lp.audio ? 'fullwin' : 'mini');
	});
	// Chapters and playlist are hidden until asked for; showing one makes
	// the player big.
	[['.lp-show-chapters', '.lp-chapters'], ['.lp-show-list', '.lp-list']].forEach(function (p) {
		lpQ(p[0]).addEventListener('click', function () {
			var part = lpQ(p[1]), show = part.classList.contains('d-none');
			part.classList.toggle('d-none', !show);
			this.classList.toggle('active', show);
			if (show && lpSize() !== 'expanded') { lp.sized = true; lpSetSize('expanded'); }
			if (show && p[1] === '.lp-list') {
				var cur = part.querySelector('.active');
				if (cur) { part.scrollTop = cur.offsetTop - part.clientHeight / 2; }
			}
			lpDockSpace();
		});
	});
	lpQ('.lp-quality').addEventListener('change', function () {
		lpStore(lp.audio ? 'lp-q-audio' : 'lp-q-video', this.value);
		lpDiscardPre();
		lpStart(lpPosition());
	});
	lpQ('.lp-audio').addEventListener('change', function () {
		var t = (lp.info.audioTracks || [])[+this.value];
		lpStore('lp-audio-lang', t && t.lang ? t.lang : '');
		lp.sel.audio = +this.value;
		lpDiscardPre();
		lpStart(lpPosition());
	});
	lpQ('.lp-subs').addEventListener('change', function () { lpSetSub(this.value, true); });
	lpTimeline();
	lpQ('.lp-play').addEventListener('click', lpToggle);
	lpQ('.lp-mute').addEventListener('click', function () { if (lp.el) { lp.el.muted = !lp.el.muted; } });
	lpQ('.lp-vol').addEventListener('input', function () { lpVolShow(+this.value); if (lp.el) { lp.el.muted = false; lp.el.volume = +this.value; } });
	lpQ('.lp-full').addEventListener('click', lpFullscreen);
	lpQ('.lp-prev').addEventListener('click', function () { lpStep(-1, false); });
	lpQ('.lp-back').addEventListener('click', function () { lpJump(-10); });
	lpQ('.lp-fwd').addEventListener('click', function () { lpJump(10); });
	lpQ('.lp-next').addEventListener('click', function () { lpStep(1, false); });
	lpQ('.lp-close').addEventListener('click', lpClose);
	if (music) {
		lpQ('.lp-shuffle').addEventListener('click', function () {
			lp.shuffle = !lp.shuffle; lp.played = {}; lp.played[lp.idx] = true;
			lpStore('lp-shuffle', lp.shuffle ? '1' : '0'); lpDiscardPre(); lpShowModes();
		});
		lpQ('.lp-repeat').addEventListener('click', function () {
			lp.repeat = { off: 'all', all: 'one', one: 'off' }[lp.repeat] || 'off';
			lpStore('lp-repeat', lp.repeat); lpDiscardPre(); lpShowModes();
		});
		lpShowModes();
	}
	if (book) {
		var rate = lpQ('.lp-rate');
		rate.value = String(parseFloat(lpStore('lp-rate')) || 1);
		if (!rate.value) { rate.value = '1'; }
		rate.addEventListener('change', function () { lpStore('lp-rate', this.value); lpApplyRate(lp.el); });
		lpQ('.lp-sleep').addEventListener('change', function () { lpSetSleep(this.value); });
	}
	if (list.length > 1) {
		lpQ('.lp-list').innerHTML = list.map(function (x, i) {
			return '<button type="button" class="list-group-item list-group-item-action py-1 small" data-i="' + i + '"></button>';
		}).join('');
		box.querySelectorAll('.lp-list [data-i]').forEach(function (b) {
			b.textContent = list[+b.dataset.i].label;
			b.addEventListener('click', function () { lpLoad(+b.dataset.i); });
		});
		lpQ('.lp-show-list').classList.remove('d-none');
	}
	lpDockSpace();
	lpSessionHandlers();
	// iOS starts media only from a tap, or on elements a tap started once:
	// both element types are made and started here, still in the tap. Only
	// there: other browsers mark an element started without a source as
	// playing.
	lp.ready = lpUseHLS() ? { audio: lpUnlock(lpMakeEl(true)), video: lpUnlock(lpMakeEl(false)) } : {};
	// The connection is measured before the stream starts, so the two don't
	// share the line (a result is kept for 30 minutes). A multi-part
	// audiobook's timeline loads meanwhile.
	var mine = lp;
	var bookInfo = book && list.length > 1
		? fetch('/api/admin/library/play/book?ids=' + list.map(function (x) { return x.id; }).join(','), { credentials: 'same-origin' })
			.then(function (r) { return r.ok ? r.json() : null; }).catch(function () { return null; })
		: Promise.resolve(null);
	Promise.all([lpMeasure(false), bookInfo]).then(function (res) {
		if (lp !== mine) { return; }
		if (res[1] && res[1].duration > 0) { lp.book = res[1]; }
		lpLoad(lp.idx);
	});
}

// ---- Connection speed and quality marks ----
function lpMeasure(force) {
	var cached = null;
	try { cached = JSON.parse(lpStore('lp-net') || 'null'); } catch (e) { cached = null; }
	if (!force && cached && Date.now() - cached.at < 30 * 60000) { lp.net = cached.kbps; return Promise.resolve(); }
	var info = lpQ('.lp-info');
	if (info) { info.textContent = 'Measuring the connection…'; }
	var ctrl = new AbortController(), bytes = 0, t0 = 0, t1 = 0;
	var stop = setTimeout(function () { ctrl.abort(); }, 2500);
	return fetch('/api/admin/library/play/speedtest?kb=16384', { credentials: 'same-origin', cache: 'no-store', signal: ctrl.signal })
		.then(function (r) {
			var reader = r.body.getReader();
			function pump() {
				return reader.read().then(function (res) {
					if (res.done) { return; }
					// Timing starts with the first data, so the request's round trip doesn't count.
					if (!t0) { t0 = performance.now(); } else { bytes += res.value.length; }
					t1 = performance.now();
					return pump();
				});
			}
			return pump();
		})
		.catch(function () {})
		.then(function () {
			clearTimeout(stop);
			var secs = (t1 - t0) / 1000;
			if (bytes > 65536 && secs > 0.005) {
				var kbps = Math.round(bytes * 8 / 1000 / secs);
				lpStore('lp-net', JSON.stringify({ kbps: kbps, at: Date.now() }));
				if (lp) { lp.net = kbps; }
			}
			if (lp) { lpMarkQualities(); lpShowInfo(); }
		});
}

function lpFmtKbps(k) { return k >= 1000 ? (k / 1000).toFixed(1) + ' Mbit/s' : k + ' kbit/s'; }

// lpMarkQualities marks each quality: ★ the best one that should play
// without stalls, ✓ the others that fit, ⚠ the ones that will stall because
// the connection is too slow or the server converts slower than real time.
// A stream needs some headroom over its average bitrate.
function lpMarkQualities() {
	if (!lp || !lp.info) { return; }
	var sel = lpQ('.lp-quality');
	if (!sel) { return; }
	var best = -1, bestKbps = -1, notes = [];
	lp.info.qualities.forEach(function (q, i) {
		var note = '';
		var netOK = lp.net ? (q.kbps || 0) <= lp.net * 0.75 : null;
		var convOK = q.converted && q.speed ? q.speed >= 1.1 : null;
		if (netOK === false) { note = ' ⚠ needs a faster connection'; }
		else if (convOK === false) { note = ' ⚠ server converts at ' + q.speed.toFixed(1) + '×'; }
		else if (netOK) { note = ' ✓'; if ((q.kbps || 0) > bestKbps) { bestKbps = q.kbps || 0; best = i; } }
		notes.push(note);
	});
	if (best >= 0) { notes[best] = ' ★ best without stalls'; }
	[].forEach.call(sel.options, function (o, i) { o.textContent = lp.info.qualities[i].label + notes[i]; });
}

function lpShowModes() {
	var sh = lpQ('.lp-shuffle'), rp = lpQ('.lp-repeat');
	if (!sh || !rp) { return; }
	sh.classList.toggle('active', lp.shuffle);
	sh.setAttribute('aria-pressed', lp.shuffle ? 'true' : 'false');
	var label = { off: 'Repeat: off', all: 'Repeat: all', one: 'Repeat: this track' }[lp.repeat];
	rp.classList.toggle('active', lp.repeat !== 'off');
	rp.title = label; rp.setAttribute('aria-label', label);
	rp.innerHTML = '<i class="fa-solid fa-repeat"></i>' + (lp.repeat === 'one' ? '<sup class="ms-1">1</sup>' : '');
}

function lpItem() { return lp.list[lp.idx]; }
function lpBaseOf(it) { return '/api/admin/library/play/' + it.kind + '/' + it.id; }
function lpBase() { return lpBaseOf(lpItem()); }
function lpPosKey() { var it = lpItem(); return 'lp-pos:' + it.kind + ':' + it.id; }
// lpPosition is the position in the current file.
function lpPosition() { return lp.el ? lp.offset + (lp.el.currentTime || 0) : 0; }
// On a book's timeline, the current part starts at lpPartStart.
function lpPartStart() { return lp.book && lp.book.parts[lp.idx] ? lp.book.parts[lp.idx].start : 0; }
function lpAbs() { return lpPartStart() + lpPosition(); }
function lpTotal() { return lp.book ? lp.book.duration : (lp.info && lp.info.duration) || 0; }
function lpChapters() { return lp.book ? lp.book.chapters : (lp.info && lp.info.chapters) || []; }

// lpUnlock starts an element without a source during a tap (it fails, but
// iOS allows the element to play from then on).
function lpUnlock(el) {
	try { var p = el.play(); if (p && p.catch) { p.catch(function () {}); } } catch (e) { /* nothing to play yet */ }
	return el;
}

// lpTake returns the element prepared in the tap for audio or video, or a new one.
function lpTake(audio) {
	var key = audio ? 'audio' : 'video';
	if (lp.ready && lp.ready[key]) { var el = lp.ready[key]; lp.ready[key] = null; return el; }
	return lpMakeEl(audio);
}

// lpMakeEl creates a media element; its events only count while it is the
// playing one (a preloaded next element stays quiet).
function lpMakeEl(audio) {
	var el = document.createElement(audio ? 'audio' : 'video');
	if (!audio) { el.className = 'rounded bg-black'; el.setAttribute('playsinline', ''); }
	el.preload = 'auto';
	var vol = parseFloat(lpStore('lp-vol'));
	if (vol >= 0 && vol <= 1) { el.volume = vol; }
	function mine(fn) { return function (e) { if (lp && lp.el === el) { fn(e); } }; }
	el.addEventListener('timeupdate', mine(lpTick));
	el.addEventListener('ended', mine(lpEnded));
	el.addEventListener('play', mine(function () { lp.userPaused = false; lp.buffering = null; lpShowInfo(); lpShowState(); }));
	el.addEventListener('pause', mine(function () { if (!lp.buffering) { lp.userPaused = true; lp.pausedAt = Date.now(); } lpShowState(); }));
	el.addEventListener('waiting', mine(function () { if (!lp.userPaused && !lp.buffering && !el.ended) { lpBuffer(true); } }));
	el.addEventListener('progress', mine(lpShowBuffered));
	el.addEventListener('volumechange', mine(lpShowState));
	el.addEventListener('loadedmetadata', mine(function () { lpApplyRate(el); lpShowInfo(); }));
	el.addEventListener('click', mine(lpToggle));
	// A broken stream (e.g. an HLS session removed during a long pause)
	// restarts at the position, twice at most per file.
	el.addEventListener('error', mine(function () {
		if (!el.getAttribute('src') || lpDirect() || (lp.retries || 0) >= 2) { return; }
		lp.retries = (lp.retries || 0) + 1;
		lpStart(lpPosition());
	}));
	return el;
}

function lpApplyRate(el) {
	if (!el || !lp || !lp.isBook) { return; }
	var r = parseFloat(lpStore('lp-rate')) || 1;
	el.defaultPlaybackRate = r;
	el.playbackRate = r;
}

// lpPlaceEl puts el into the bar (audio next to the cover).
function lpPlaceEl(el) {
	var media = lpQ('.lp-media');
	var old = lp.el;
	if (old && old !== el) { lpUnfeed(old); old.pause(); old.removeAttribute('src'); old.load(); old.remove(); }
	if (!media.querySelector('img') && el.tagName === 'AUDIO' && lp.cover) {
		var img = document.createElement('img'); img.className = 'lp-cover'; img.alt = ''; img.src = lp.cover;
		media.appendChild(img);
	}
	if (el.tagName === 'VIDEO') { var i = media.querySelector('img'); if (i) { i.remove(); } }
	media.appendChild(el);
	lp.el = el;
}

// lpLoad plays entry i, at startAt seconds into it when given (else where
// it was left). A preloaded entry starts right away.
function lpLoad(i, startAt) {
	lp.idx = i;
	lp.retries = 0;
	lp.played[i] = true;
	if (lp.hist[lp.hist.length - 1] !== i) { lp.hist.push(i); }
	var box = document.getElementById('libPlayer');
	lpQ('.lp-title').textContent = lpItem().label;
	var listBox = box.querySelector('.lp-list');
	box.querySelectorAll('.lp-list [data-i]').forEach(function (b) {
		b.classList.toggle('active', +b.dataset.i === i);
		// Scroll the playlist only, not the page.
		if (+b.dataset.i === i && (b.offsetTop < listBox.scrollTop || b.offsetTop + b.offsetHeight > listBox.scrollTop + listBox.clientHeight)) {
			listBox.scrollTop = b.offsetTop - listBox.clientHeight / 2;
		}
	});
	var pre = lp.pre;
	lp.pre = null;
	if (pre && pre.idx === i && pre.info && pre.el && startAt === undefined) {
		lpApply(pre.info, pre.el, 0);
		return;
	}
	if (pre) { lpDiscard(pre); }
	var mine = lp;
	fetch(lpBase() + '/info', { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (info) { if (lp === mine && lp.idx === i) { lpApply(info, null, startAt); } })
		.catch(function (e) { libNotify('error', 'Can\'t play: ' + e.message); });
}

// lpShowMeta shows what is playing: the track, part, movie or episode, and
// next to it artist and album, author and book, year and genres or series
// and episode (info.meta); before that, the playlist's label.
function lpShowMeta() {
	var m = (lp.info && lp.info.meta) || {}, label = lpItem().label;
	lpQ('.lp-title').textContent = m.title || label;
	lpQ('.lp-title').title = label;
	var sub = m.sub || (lp.entry && lp.entry !== label ? lp.entry : '');
	lpQ('.lp-entry').textContent = sub ? '· ' + sub : '';
}

function lpApply(info, preEl, startAt) {
	lp.info = info;
	lpShowMeta();
	lp.audio = info.audio;
	lp.sel = lpPickTracks(info);
	lp.cues = null;
	lp.subNote = '';
	// The next file was loaded ahead as it is; another track restarts it.
	if (preEl && lpSelParams()) { lpDiscard({ el: preEl }); preEl = null; startAt = 0; }
	if (preEl) {
		lpPlaceEl(preEl);
	} else if (!lp.el || (lp.el.tagName === 'AUDIO') !== info.audio) {
		lpPlaceEl(lpTake(info.audio));
	}
	var sel = lpQ('.lp-quality');
	var wanted = lpWantedQuality(info);
	sel.innerHTML = '';
	info.qualities.forEach(function (q) {
		var o = document.createElement('option'); o.value = q.value; o.textContent = q.label;
		if (q.value === wanted) { o.selected = true; }
		sel.appendChild(o);
	});
	lpMarkQualities();
	lpShowTracks();
	if (lpSubById(lp.sel.sub)) { lpSetSub(lp.sel.sub, false); } else { lpApplySubs(); }
	lpShowChapters();
	lpQ('.lp-dur').textContent = lpFmt(lpTotal());
	lpTlShow(lpAbs());
	lpQ('.lp-full').classList.toggle('d-none', info.audio);
	// Video opens in the bigger player, unless the size was chosen already.
	if (!info.audio && !lp.sized) {
		lp.sized = true;
		lpSetSize('expanded');
	}
	lpSizeButton();
	lpSessionMeta();
	if (preEl) {
		// Preloaded from the start of the file with the wanted quality.
		lp.offset = 0;
		lpApplyRate(lp.el);
		lpShowInfo();
		lp.userPaused = false;
		lpBuffer();
		return;
	}
	var t = startAt;
	if (t === undefined) {
		// Movies, episodes and audiobooks resume where they stopped; tracks start over.
		var resume = lp.music ? 0 : +(lpStore(lpPosKey()) || 0);
		t = resume > 10 && resume < (info.duration || 0) - 30 ? resume : 0;
	}
	lpStart(t);
}

function lpWantedQuality(info) {
	var wanted = lpStore(info.audio ? 'lp-q-audio' : 'lp-q-video');
	return info.qualities.some(function (q) { return q.value === wanted; }) ? wanted : 'original';
}

function lpShowChapters() {
	var ch = lpQ('.lp-chapters');
	ch.innerHTML = '';
	lpChapters().forEach(function (c) {
		var b = document.createElement('button');
		b.type = 'button'; b.className = 'btn btn-sm btn-outline-secondary py-0';
		b.textContent = c.title || lpFmt(c.start);
		b.title = lpFmt(c.start);
		b.dataset.start = c.start;
		b.addEventListener('click', function () { lpSeekAbs(c.start); });
		ch.appendChild(b);
	});
	lpQ('.lp-show-chapters').classList.toggle('d-none', !ch.childElementCount);
}

function lpDirect() { return lp.info.mode === 'direct' && lpQ('.lp-quality').value === 'original' && !lpSelParams(); }

// ---- Audio tracks and subtitles ----
// lp.sel is the audio track (index) and subtitle (id) of the current file.
// Another audio track or a burned-in picture subtitle makes the server
// repackage or convert the file (lpSelParams). Text subtitles come as cues
// (seconds in the file) and go into the video's text track, shifted by
// lp.offset. The languages last chosen are preferred for the next file.
function lpFind(arr, fn) { for (var i = 0; i < arr.length; i++) { if (fn(arr[i])) { return arr[i]; } } return null; }

function lpPickTracks(info) {
	var sel = { audio: 0, sub: '' };
	var at = info.audioTracks || [], subs = info.subtitles || [];
	var al = lpStore('lp-audio-lang') || '';
	var a = (al && lpFind(at, function (t) { return t.lang === al; })) || lpFind(at, function (t) { return t.default; });
	if (a) { sel.audio = a.index; }
	var audioLang = at[sel.audio] ? at[sel.audio].lang : '';
	// Only text subtitles are chosen by themselves; burning in converts.
	var sl = lpStore('lp-sub-lang') || '';
	var s = sl ? lpFind(subs, function (x) { return x.lang === sl && !x.image && !x.forced; }) || lpFind(subs, function (x) { return x.lang === sl && !x.image; }) : null;
	if (!s && audioLang) { s = lpFind(subs, function (x) { return x.forced && !x.image && x.lang === audioLang; }); }
	if (s) { sel.sub = s.id; }
	return sel;
}

function lpSubById(id) { return id && lp.info ? lpFind(lp.info.subtitles || [], function (x) { return x.id === id; }) : null; }

function lpSelParams() {
	if (!lp || !lp.sel || lp.audio) { return ''; }
	var p = lp.sel.audio ? '&a=' + lp.sel.audio : '';
	var s = lpSubById(lp.sel.sub);
	if (s && s.image) { p += '&burn=' + s.id.slice(1); }
	return p;
}

function lpShowTracks() {
	var at = (!lp.audio && lp.info.audioTracks) || [], subs = (!lp.audio && lp.info.subtitles) || [];
	var a = lpQ('.lp-audio'), su = lpQ('.lp-subs');
	a.innerHTML = '';
	at.forEach(function (t) {
		var o = document.createElement('option'); o.value = t.index; o.textContent = t.label;
		if (t.index === lp.sel.audio) { o.selected = true; }
		a.appendChild(o);
	});
	a.classList.toggle('d-none', at.length < 2);
	su.innerHTML = '<option value="">Subtitles: off</option>';
	subs.forEach(function (x) {
		var o = document.createElement('option'); o.value = x.id; o.textContent = x.label;
		if (x.id === lp.sel.sub) { o.selected = true; }
		su.appendChild(o);
	});
	su.classList.toggle('d-none', !subs.length);
}

// lpSetSub switches the subtitle; user: chosen in the menu (remembered, and
// a burned-in one restarts the stream).
function lpSetSub(id, user) {
	var old = lpSubById(lp.sel.sub), s = lpSubById(id);
	lp.sel.sub = id;
	lp.cues = null;
	if (user) { lpStore('lp-sub-lang', s ? (s.lang || '') : ''); }
	if (user && ((old && old.image) || (s && s.image))) { lpDiscardPre(); lpStart(lpPosition()); }
	if (s && !s.image) { lpLoadCues(s); } else { lpApplySubs(); }
}

function lpLoadCues(s) {
	var mine = lp, idx = lp.idx;
	lp.subNote = 'Loading subtitles…';
	lpShowInfo();
	fetch(lpBase() + '/sub/' + encodeURIComponent(s.id), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (lp !== mine || lp.idx !== idx || lp.sel.sub !== s.id) { return; }
			lp.subNote = '';
			lp.cues = d.cues || [];
			lpApplySubs();
			lpShowInfo();
		})
		.catch(function (e) {
			if (lp !== mine) { return; }
			lp.subNote = '';
			lpShowInfo();
			libNotify('error', 'Can\'t load the subtitles: ' + e.message);
		});
}

// lpApplySubs puts the cues into the element's text track, in the times of
// the stream (which starts at lp.offset of the file).
function lpApplySubs() {
	var el = lp && lp.el;
	if (!el || el.tagName !== 'VIDEO') { return; }
	var cues = lp.cues || [];
	var tr = el.lpTrack;
	if (!tr) {
		if (!cues.length) { return; }
		tr = el.lpTrack = el.addTextTrack('subtitles', 'Subtitles');
	}
	tr.mode = 'hidden';
	while (tr.cues && tr.cues.length) { tr.removeCue(tr.cues[0]); }
	if (!cues.length) { tr.mode = 'disabled'; return; }
	var off = lp.offset || 0, Cue = window.VTTCue || window.TextTrackCue;
	cues.forEach(function (c) {
		if (c.e - off <= 0) { return; }
		try { tr.addCue(new Cue(Math.max(0, c.s - off), c.e - off, c.t)); } catch (e) { /* a cue the browser refuses */ }
	});
	tr.mode = 'showing';
}

// A browser without Media Source Extensions (the iPhone: every browser there
// is Apple's WebKit) only plays video it can fetch in byte ranges; converted
// video goes there as HLS, which it plays natively. Every other browser takes
// the converted stream through MSE (lpFeed). Decided by what the browser can
// do, not what it calls itself: recent Chrome also plays HLS natively but
// treats the growing playlist as live, fetching one segment in four, and a
// Chrome with a Safari user agent (DevTools override) would land there.
var lpHLSSupport = null;
function lpUseHLS() {
	if (lpHLSSupport === null) {
		var v = document.createElement('video');
		var mse = !!(window.MediaSource && MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028,mp4a.40.2"'));
		lpHLSSupport = !mse && !!(v.canPlayType && v.canPlayType('application/vnd.apple.mpegurl'));
	}
	return lpHLSSupport;
}

// ---- Converted video through Media Source Extensions ----
// A plain <video src> reads an endless converted stream only a few seconds
// ahead, so a jump of 10 s already restarts the conversion. lpFeed fetches
// the stream itself and appends it to a SourceBuffer: everything received is
// buffered (up to lpFeedAhead seconds; then the server and its conversion
// wait), and jumps within it are instant. 60 s behind the position are kept
// for jumps back. On any MSE error the player falls back to the plain stream.
var lpFeedAhead = 120;

// lpPid names this page's player to the server: a new stream (a jump past
// what is buffered) replaces the player's previous one there at once, so
// quick jumps don't fill player_max_transcodes with streams already left.
var lpPid = Math.random().toString(36).slice(2, 10);

function lpMSEType() {
	var audio = !lp.info.audioTracks || lp.info.audioTracks.length > 0;
	var type = 'video/mp4; codecs="avc1.640028' + (audio ? ',mp4a.40.2' : '') + '"';
	return window.MediaSource && MediaSource.isTypeSupported(type) ? type : '';
}

function lpUnfeed(el) { if (el && el.lpFeed) { el.lpFeed.abort(); el.lpFeed = null; } }

// t is the wanted position: the stream may begin a little earlier (at a
// keyframe, X-Stream-Start), the player starts at t anyway. A stream that
// breaks off (the connection dropped, e.g. a proxy closing it during a long
// pause) marks the feed broken; the player restarts it where it is
// (lpRestartBroken) instead of treating that as the end.
function lpFeed(el, url, t) {
	var type = lpMSEType();
	if (!type || lp.noMSE) { return false; }
	var ms = new MediaSource(), ctrl = new AbortController(), blobURL = URL.createObjectURL(ms);
	var queue = [], done = false, reading = false, reader = null, sb = null, alive = true, first = true;
	var feed = el.lpFeed = { broken: false, abort: function () { alive = false; ctrl.abort(); URL.revokeObjectURL(blobURL); } };
	function broke(e) {
		if (!alive) { return; }
		feed.broken = true;
		// What is buffered still plays; lpTick restarts when it runs out.
		if (window.console) { console.warn('player: stream broke off', e); }
	}
	el.src = blobURL;
	function failed(e) {
		if (!alive) { return; }
		lpUnfeed(el);
		if (window.console) { console.warn('player: MSE failed, plain stream instead', e); }
		if (lp && lp.el === el) { lp.noMSE = true; lpStart(lpPosition()); }
	}
	function ahead() { var b = sb.buffered; return b.length ? b.end(b.length - 1) - el.currentTime : 0; }
	function read() {
		if (!alive || reading || done) { return; }
		if (ahead() > lpFeedAhead || queue.length > 16) { setTimeout(read, 1000); return; }
		reading = true;
		reader.read().then(function (res) {
			reading = false;
			if (!alive) { return; }
			if (res.done) {
				// Ended before the end of the file: the connection was cut.
				var b = sb.buffered, end = lp.offset + (b.length ? b.end(b.length - 1) : 0);
				if (lp.info && lp.info.duration && end < lp.info.duration - 3) { broke(new Error('stream ended at ' + Math.floor(end) + ' s')); return; }
				done = true;
			} else { queue.push(res.value); }
			pump();
			read();
		}).catch(function (e) { reading = false; broke(e); });
	}
	// trim removes what lies far behind the position; force: the browser's
	// buffer is full, keep only 10 s.
	function trim(force) {
		var keep = force ? 10 : 60, b = sb.buffered;
		if (b.length && el.currentTime - b.start(0) > keep + 5) {
			try { sb.remove(0, el.currentTime - keep); } catch (e) { /* busy: next time */ }
		} else if (force) { setTimeout(pump, 1000); }
	}
	function pump() {
		if (!alive || !sb || sb.updating) { return; }
		if (first && sb.buffered.length) {
			first = false;
			// Start at the wanted position, not at the keyframe before it.
			var want = Math.max(sb.buffered.start(0), (t || 0) - lp.offset);
			if (lp.el === el && want > el.currentTime + 0.05) { el.currentTime = want; }
		}
		if (queue.length) {
			var n = 0, i;
			for (i = 0; i < queue.length; i++) { n += queue[i].length; }
			var chunk = new Uint8Array(n);
			for (n = 0, i = 0; i < queue.length; i++) { chunk.set(queue[i], n); n += queue[i].length; }
			queue = [];
			try { sb.appendBuffer(chunk); } catch (e) {
				if (e.name === 'QuotaExceededError') { queue.unshift(chunk); trim(true); return; }
				failed(e);
			}
			return;
		}
		if (done) {
			if (ms.readyState === 'open') { try { ms.endOfStream(); } catch (e) { /* closed meanwhile */ } }
			return;
		}
		trim(false);
	}
	ms.addEventListener('sourceopen', function () {
		if (!alive) { return; }
		try { sb = ms.addSourceBuffer(type); } catch (e) { failed(e); return; }
		sb.addEventListener('updateend', pump);
		sb.addEventListener('error', function (e) { failed(e); });
		fetch(url, { credentials: 'same-origin', signal: ctrl.signal })
			.then(function (r) {
				if (!r.ok || !r.body) {
					return r.text().then(function (msg) { throw new Error(msg || ('HTTP ' + r.status)); });
				}
				var st = parseFloat(r.headers.get('X-Stream-Start'));
				if (alive && lp && lp.el === el && !isNaN(st)) { lp.offset = st; lpApplySubs(); }
				reader = r.body.getReader();
				read();
			})
			.catch(function (e) {
				if (!alive) { return; }
				feed.broken = true;
				if (lp && lp.el === el) { libNotify('error', 'Can\'t play: ' + e.message); lp.buffering = null; lp.userPaused = true; lpShowInfo(); lpShowState(); }
			});
	}, { once: true });
	return true;
}

// lpRestartBroken restarts a stream that broke off, where it is.
function lpRestartBroken() {
	if (!lp || !lp.el) { return; }
	lp.buffering = null;
	lpStart(lpPosition());
}

// lpStale: the stream can't go on after a pause: its MSE feed broke off, a
// plain converted stream was paused so long that the connection is likely
// gone, or the HLS session was (nearly) removed for being idle.
function lpStale() {
	var el = lp.el;
	if (!el || lpDirect()) { return false; }
	if (el.lpFeed) { return el.lpFeed.broken; }
	var paused = lp.pausedAt ? Date.now() - lp.pausedAt : 0;
	return paused > (lp.hlsSid ? 150000 : 30000);
}

// lpReachable: t (seconds of the stream) can be played without a new
// request: buffered by lpFeed, or seekable for the browser.
function lpReachable(el, t) { return (el.lpFeed && lpInRanges(el.buffered, t)) || lpInRanges(el.seekable, t); }

function lpStopHLS() {
	if (lp && lp.hlsSid) {
		fetch('/api/admin/library/play/hls/' + lp.hlsSid + '/stop', { credentials: 'same-origin', keepalive: true }).catch(function () {});
		lp.hlsSid = '';
	}
}

function lpStart(t) {
	var q = lpQ('.lp-quality').value;
	lpUnfeed(lp.el);
	lp.el.controls = false;
	if (lpDirect()) {
		lpStopHLS();
		lp.offset = 0;
		lp.el.src = lpBase() + '/stream?q=original';
		if (t > 0) { lp.el.addEventListener('loadedmetadata', function f() { lp.el.removeEventListener('loadedmetadata', f); lp.el.currentTime = t; }); }
	} else if (!lp.audio && lpUseHLS()) {
		lp.offset = t;
		var el = lp.el, mine = lp, prev = lp.hlsSid || '';
		lp.hlsSid = '';
		el.removeAttribute('src');
		fetch(lpBase() + '/hls?q=' + encodeURIComponent(q) + '&start=' + Math.floor(t) + lpSelParams() + '&prev=' + encodeURIComponent(prev), { credentials: 'same-origin' })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				if (lp !== mine || lp.el !== el) {
					fetch('/api/admin/library/play/hls/' + d.session + '/stop', { credentials: 'same-origin' }).catch(function () {});
					return;
				}
				lp.hlsSid = d.session;
				// The stream may begin at the keyframe before t.
				if (d.start >= 0 && d.start <= t) {
					lp.offset = d.start;
					lpApplySubs();
					if (t - d.start > 0.05) { el.addEventListener('loadedmetadata', function f() { el.removeEventListener('loadedmetadata', f); el.currentTime = t - d.start; }); }
				}
				el.src = d.url;
				lpShowInfo();
				lpBuffer();
			})
			.catch(function (e) { libNotify('error', 'Can\'t play: ' + e.message); });
	} else {
		lpStopHLS();
		lp.offset = t;
		var url = lpBase() + '/stream?q=' + encodeURIComponent(q) + '&start=' + Math.floor(t) + lpSelParams() + '&pid=' + lpPid;
		if (lp.audio || !lpFeed(lp.el, url, t)) { lp.el.src = url; }
	}
	lpApplySubs();
	lpApplyRate(lp.el);
	lpShowInfo();
	lpShowState();
	lp.userPaused = false;
	lpBuffer();
}

// ---- Loading the next entry ahead ----
// lpNextIndex is the entry after the current one, or -1 at the end of the
// list (auto: the step at the end of a file, which only wraps when repeating).
function lpNextIndex(auto) {
	var n = lp.list.length;
	if (lp.shuffle && n > 1) {
		var left = [];
		for (var j = 0; j < n; j++) { if (!lp.played[j]) { left.push(j); } }
		if (!left.length) {
			if (auto && lp.repeat !== 'all') { return -1; }
			lp.played = {};
			lp.played[lp.idx] = true;
			for (var k = 0; k < n; k++) { if (k !== lp.idx) { left.push(k); } }
		}
		return left[Math.floor(Math.random() * left.length)];
	}
	var i = lp.idx + 1;
	if (i >= n) { return lp.repeat === 'all' ? 0 : -1; }
	return i;
}

// lpPreload starts loading the next entry some seconds before the current
// one ends, so it plays without a gap.
function lpPreload() {
	if (lp.pre || lp.repeat === 'one' || !lp.info || lp.list.length < 2 || (lp.sleep && lp.sleep.mode === 'file')) { return; }
	// Video over HLS starts its own session when it plays.
	if (!lp.audio && lpUseHLS()) { return; }
	var left = (lp.info.duration || 0) - lpPosition();
	if (!(lp.info.duration > 0) || left > 15) { return; }
	var i = lpNextIndex(true);
	if (i < 0) { lp.pre = { idx: -1 }; return; }
	var pre = { idx: i };
	lp.pre = pre;
	var it = lp.list[i];
	fetch(lpBaseOf(it) + '/info', { credentials: 'same-origin' })
		.then(function (r) { return r.ok ? r.json() : null; })
		.then(function (info) {
			if (!info || !lp || lp.pre !== pre) { return; }
			var el = lpMakeEl(info.audio);
			var q = lpWantedQuality(info);
			el.src = lpBaseOf(it) + '/stream?q=' + encodeURIComponent(q) + (info.mode === 'direct' && q === 'original' ? '' : '&start=0');
			el.load();
			pre.info = info;
			pre.el = el;
		})
		.catch(function () {});
}

function lpDiscard(pre) { if (pre && pre.el) { lpUnfeed(pre.el); pre.el.removeAttribute('src'); pre.el.load(); } }
function lpDiscardPre() { if (lp && lp.pre) { lpDiscard(lp.pre); lp.pre = null; } }

// ---- Buffering ----
// When the stream runs dry, the player waits until lpPrebuffer seconds lie
// ahead, so a varying speed doesn't stutter.
function lpPrebuffer() { return lp.audio ? 5 : 15; }

// lpAhead is how many seconds are buffered after the current position.
function lpAhead() {
	var el = lp.el, t = el.currentTime, b = el.buffered;
	for (var i = 0; i < b.length; i++) { if (b.start(i) <= t + 0.5 && b.end(i) >= t) { return b.end(i) - t; } }
	return 0;
}

function lpInRanges(ranges, t) {
	for (var i = 0; i < ranges.length; i++) { if (ranges.start(i) <= t && ranges.end(i) >= t) { return true; } }
	return false;
}

// lpBuffer pauses until playback can go on: a start (open, resume, seek,
// quality) only until the browser can play, which keeps buffering ahead
// while playing; a stall in playback until lpPrebuffer() seconds are there,
// so it doesn't stutter.
function lpBuffer(stall) {
	if (!lp || !lp.el) { return; }
	var el = lp.el, token = {}, since = Date.now();
	lp.buffering = token;
	if (!el.paused) { el.pause(); }
	lpShowInfo();
	lpShowState();
	(function check() {
		if (!lp || lp.buffering !== token || lp.el !== el) { return; }
		var need = stall ? lpPrebuffer() : 0;
		if (lp.info && lp.info.duration) { need = Math.min(need, Math.max(lp.info.duration - lpPosition() - 0.5, 0)); }
		// Nothing more is coming.
		if (el.lpFeed && el.lpFeed.broken && lpAhead() < need + 1) { lpRestartBroken(); return; }
		// A browser that stops loading while paused never reaches the
		// target: after 8 s, whatever it has must do.
		if (el.readyState >= 3 && (lpAhead() >= need || Date.now() - since > 8000)) {
			lp.buffering = null;
			lpShowInfo();
			el.play().catch(lpPlayRefused);
			return;
		}
		lpShowBuffered();
		setTimeout(check, 250);
	})();
}

// lpPlayRefused: the browser (iOS) only starts playback from a tap.
function lpPlayRefused(err) {
	if (!lp || !err || err.name !== 'NotAllowedError') { return; }
	lp.buffering = null;
	lp.userPaused = true;
	lpShowState();
	lpQ('.lp-info').textContent = 'Tap ▶ to start';
}

function lpShowBuffered() {
	if (!lp || !lp.el) { return; }
	var ahead = lpAhead();
	lpQ('.lp-buf').textContent = ahead >= 1 ? '· ' + Math.floor(ahead) + ' s buffered' : '';
}

// lpShowInfo says how the file is played, with the picture size actually
// received, so a chosen quality can be checked.
function lpShowInfo() {
	if (!lp || !lp.info) { return; }
	var sel = lpQ('.lp-quality');
	// Another audio track repackages a direct file; burning in converts.
	var extra = lpSelParams(), burn = extra.indexOf('burn=') >= 0;
	var mode = burn ? 'transcode' : lp.info.mode === 'direct' && extra ? 'remux' : lp.info.mode;
	var text = lpDirect() ? 'Original file'
		: sel.value === 'original' ? (mode === 'remux' ? 'Original, repackaged' : 'Original, converted')
		: 'Converted to ' + lp.info.qualities[sel.selectedIndex].label;
	if (burn) { text += ' · subtitles burned in'; }
	if (!lp.audio && lp.el && lp.el.videoHeight) { text += ' · ' + lp.el.videoWidth + '×' + lp.el.videoHeight; }
	if (lp.hlsSid) { text += ' · HLS'; }
	if (lp.subNote) { text += ' · ' + lp.subNote; }
	if (lp.buffering) { text += ' · Buffering…'; }
	if (lp.net) { text += ' · connection ' + lpFmtKbps(lp.net); }
	if (lp.sleep) { text += ' · ' + lpSleepText(); }
	lpQ('.lp-info').textContent = text;
}

// lpVolShow fills the volume line up to v (0..1).
function lpVolShow(v) { var r = lpQ('.lp-vol'); if (r) { r.style.setProperty('--v', Math.round(v * 100) + '%'); } }

function lpShowState() {
	if (!lp || !lp.el) { return; }
	// While buffering, playback is on its way: the button pauses.
	lpQ('.lp-play').innerHTML = '<i class="fa-solid ' + (lp.el.paused && !lp.buffering ? 'fa-play' : 'fa-pause') + '"></i>';
	lpQ('.lp-mute').innerHTML = '<i class="fa-solid ' + (lp.el.muted || lp.el.volume === 0 ? 'fa-volume-xmark' : 'fa-volume-high') + '"></i>';
	lpQ('.lp-vol').value = lp.el.muted ? 0 : lp.el.volume;
	lpVolShow(lp.el.muted ? 0 : lp.el.volume);
	lpStore('lp-vol', String(lp.el.volume));
	if ('mediaSession' in navigator) { navigator.mediaSession.playbackState = lp.el.paused && !lp.buffering ? 'paused' : 'playing'; }
}

function lpToggle() {
	if (!lp || !lp.el) { return; }
	if (lp.buffering) { lp.buffering = null; lp.userPaused = true; lpShowInfo(); lpShowState(); return; }
	if (lp.el.paused) {
		if (lpStale()) { lp.pausedAt = 0; lpRestartBroken(); return; }
		// play() inside the tap, so iOS allows it; buffering may pause again.
		lp.el.play().catch(lpPlayRefused);
		lp.userPaused = false;
		lpBuffer();
	} else { lp.el.pause(); }
}

// lpFullscreen shows the player (video with its controls) full screen.
// Where page elements can't go full screen (the iPhone), the player fills
// the browser window instead (lpFullWindow), keeping its own controls; older
// Safari has the prefixed API.
function lpFullscreen() {
	if (!lp || lp.audio || !lp.el) { return; }
	var box = document.getElementById('libPlayer');
	var fsEl = document.fullscreenElement || document.webkitFullscreenElement;
	if (box.classList.contains('lp-fullwin')) {
		lpFullWindow(false);
	} else if (fsEl) {
		(document.exitFullscreen || document.webkitExitFullscreen).call(document);
	} else if (box.requestFullscreen && document.fullscreenEnabled !== false) {
		box.requestFullscreen().catch(function () { lpFullWindow(true); });
	} else if (box.webkitRequestFullscreen && document.webkitFullscreenEnabled) {
		box.webkitRequestFullscreen();
	} else {
		lpFullWindow(true);
	}
}

// The player has three sizes: 'mini' (a bar), 'expanded' (bigger at the
// bottom, with chapters and playlist) and 'fullwin' (covering the window,
// video only). The size button cycles through them.
function lpSize() {
	var box = document.getElementById('libPlayer');
	return box.classList.contains('lp-fullwin') ? 'fullwin' : box.classList.contains('lp-expanded') ? 'expanded' : 'mini';
}

function lpSetSize(size) {
	var box = document.getElementById('libPlayer');
	// Full window keeps 'expanded' underneath: leaving it returns there.
	box.classList.toggle('lp-expanded', size !== 'mini');
	lpFullWindow(size === 'fullwin');
}

// lpSizeButton shows what the size button does next.
function lpSizeButton() {
	var b = document.querySelector('#libPlayer .lp-expand');
	if (!b || !lp) { return; }
	var size = lpSize(), next = size === 'mini' ? 'expanded' : size === 'expanded' && !lp.audio ? 'fullwin' : 'mini';
	var look = { expanded: ['fa-up-right-and-down-left-from-center', 'Bigger player'], fullwin: ['fa-maximize', 'Full window'], mini: ['fa-down-left-and-up-right-to-center', 'Small player'] }[next];
	b.innerHTML = '<i class="fa-solid ' + look[0] + '"></i>';
	b.title = look[1];
	b.setAttribute('aria-label', look[1]);
	b.classList.toggle('active', size !== 'mini');
}

// lpFullWindow lets the player cover the browser window (on) or go back.
function lpFullWindow(on) {
	var box = document.getElementById('libPlayer');
	if (!box) { return; }
	box.classList.toggle('lp-fullwin', on);
	document.body.classList.toggle('lp-fullwin-on', on);
	var b = box.querySelector('.lp-full');
	if (b) {
		b.classList.toggle('active', on);
		b.title = on ? 'Leave full window (F, Esc)' : 'Full screen (F)';
		b.innerHTML = '<i class="fa-solid ' + (on ? 'fa-compress' : 'fa-expand') + '"></i>';
	}
	lpSizeButton();
	lpDockSpace();
}

// lpSeek jumps to t seconds of the current file. A converted stream
// restarts there, unless t lies in the part the browser already has.
function lpSeek(t) {
	if (!lp || !lp.el) { return; }
	t = Math.max(0, lp.info && lp.info.duration ? Math.min(t, lp.info.duration - 1) : t);
	if (lpDirect()) {
		lp.el.currentTime = t;
	} else if (t >= lp.offset && lpReachable(lp.el, t - lp.offset)) {
		lp.el.currentTime = t - lp.offset;
	} else {
		if (!lpSeekAhead(t)) { lpStart(t); }
		return;
	}
	lp.userPaused = false;
	lpBuffer();
}

// lpSeekAhead jumps forward in a repackaged or converted stream to a point
// it hasn't received yet but soon will (at most 30 s past what it has): it
// waits up to 8 s for it instead of restarting the conversion there, which
// would start with an empty buffer. False: restart instead.
function lpSeekAhead(t) {
	var el = lp.el, rel = t - lp.offset, b = el.buffered;
	if (rel < 0 || !b.length || rel > b.end(b.length - 1) + 30) { return false; }
	var token = { target: t }, since = Date.now();
	lp.buffering = token;
	lp.userPaused = false;
	if (!el.paused) { el.pause(); }
	lpShowInfo();
	lpShowState();
	(function check() {
		if (!lp || lp.buffering !== token || lp.el !== el) { return; }
		if (lpInRanges(el.buffered, rel) && lpReachable(el, rel)) {
			lp.buffering = null;
			el.currentTime = rel;
			lpBuffer();
			return;
		}
		if (Date.now() - since > 8000) { lp.buffering = null; lpStart(t); return; }
		lpShowBuffered();
		setTimeout(check, 250);
	})();
	return true;
}

// lpSeekAbs jumps on the timeline: across the parts of a book, else within
// the file.
function lpSeekAbs(t) {
	if (!lp) { return; }
	if (!lp.book) { lpSeek(t); return; }
	t = Math.max(0, Math.min(t, lp.book.duration - 1));
	var parts = lp.book.parts, i = 0;
	while (i < parts.length - 1 && t >= parts[i + 1].start) { i++; }
	if (i === lp.idx) { lpSeek(t - parts[i].start); } else { lpDiscardPre(); lpLoad(i, t - parts[i].start); }
}

// Several presses while a jump waits for the stream add up.
function lpJump(d) {
	if (!lp || !lp.el) { return; }
	var waiting = lp.buffering && lp.buffering.target !== undefined;
	lpSeekAbs((waiting ? lpPartStart() + lp.buffering.target : lpAbs()) + d);
}

// ---- Timeline ----
// The position bar: a line with the played part, what is loaded and a
// marker. Hovering shows the time under the pointer, a click or drag jumps
// there; the position label takes a typed time (1:23:45, 83:45, 90).

// lpTlShow draws the marker at abs seconds (or the dragged time).
function lpTlShow(abs) {
	var tl = lpQ('.lp-tl'), total = lpTotal();
	if (!tl) { return; }
	var pct = function (t) { return (total > 0 ? Math.max(0, Math.min(1, t / total)) * 100 : 0) + '%'; };
	lpQ('.lp-tl-fill').style.width = pct(abs);
	lpQ('.lp-tl-knob').style.left = pct(abs);
	// The loaded part around the position.
	var buf = lpQ('.lp-tl-buf'), el = lp.el, from = 0, to = 0;
	if (el && el.buffered) {
		for (var i = 0; i < el.buffered.length; i++) {
			if (el.buffered.start(i) <= el.currentTime + 0.5 && el.buffered.end(i) >= el.currentTime) {
				var base = lpPartStart() + (lp.offset || 0);
				from = base + el.buffered.start(i);
				to = base + el.buffered.end(i);
			}
		}
	}
	buf.style.left = pct(from);
	buf.style.width = (total > 0 ? Math.max(0, Math.min(to, total) - from) / total * 100 : 0) + '%';
	tl.setAttribute('aria-valuemax', String(Math.floor(total)));
	tl.setAttribute('aria-valuenow', String(Math.floor(abs)));
	tl.setAttribute('aria-valuetext', lpFmt(abs) + ' of ' + lpFmt(total));
}

// lpParseTime reads "1:23:45", "83:45" or "90" (seconds); NaN if invalid.
function lpParseTime(s) {
	var parts = String(s).trim().split(':');
	if (parts.length > 3 || parts.some(function (p) { return !/^\d+$/.test(p); })) { return NaN; }
	return parts.reduce(function (acc, p) { return acc * 60 + +p; }, 0);
}

function lpTimeline() {
	var tl = lpQ('.lp-tl'), tip = lpQ('.lp-tl-tip'), hover = lpQ('.lp-tl-hover');
	function at(e) {
		var r = tl.getBoundingClientRect(), f = r.width ? Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)) : 0;
		return { t: f * lpTotal(), x: f * 100 + '%' };
	}
	function showTip(p) {
		tip.textContent = lpFmt(p.t);
		tip.style.left = p.x;
		hover.style.left = p.x;
		tip.classList.remove('d-none');
		hover.classList.remove('d-none');
	}
	function hideTip() { tip.classList.add('d-none'); hover.classList.add('d-none'); }
	tl.addEventListener('pointermove', function (e) {
		if (!lp || !lpTotal()) { return; }
		var p = at(e);
		showTip(p);
		if (lp.tlDrag) { lp.tlDrag = p.t; lpTlShow(p.t); }
	});
	tl.addEventListener('pointerleave', function () { if (!lp || !lp.tlDrag) { hideTip(); } });
	tl.addEventListener('pointerdown', function (e) {
		if (!lp || !lpTotal() || e.button > 0) { return; }
		e.preventDefault();
		tl.focus();
		tl.setPointerCapture(e.pointerId);
		tl.classList.add('lp-tl-drag');
		var p = at(e);
		lp.tlDrag = p.t;
		showTip(p);
		lpTlShow(p.t);
	});
	function end(e, jump) {
		if (!lp || !lp.tlDrag) { return; }
		var t = lp.tlDrag;
		lp.tlDrag = null;
		tl.classList.remove('lp-tl-drag');
		if (e.pointerType !== 'mouse') { hideTip(); }
		if (jump) { lpSeekAbs(t); } else { lpTlShow(lpAbs()); }
	}
	tl.addEventListener('pointerup', function (e) { end(e, true); });
	tl.addEventListener('pointercancel', function (e) { end(e, false); });
	// Arrows jump 10 s (the player's keys); Page Up/Down a minute.
	tl.addEventListener('keydown', function (e) {
		var d = { PageUp: 60, PageDown: -60 }[e.key];
		if (d) { e.preventDefault(); e.stopPropagation(); lpJump(d); }
		if (e.key === 'Home') { e.preventDefault(); e.stopPropagation(); lpSeekAbs(0); }
	});
	// The position label turns into a field for a typed time.
	var pos = lpQ('.lp-pos'), go = lpQ('.lp-goto');
	function closeGo() { go.classList.add('d-none'); go.classList.remove('is-invalid'); pos.classList.remove('d-none'); }
	pos.addEventListener('click', function () {
		if (!lp || !lpTotal()) { return; }
		go.value = lpFmt(lpAbs());
		pos.classList.add('d-none');
		go.classList.remove('d-none');
		go.focus();
		go.select();
	});
	go.addEventListener('keydown', function (e) {
		if (e.key === 'Escape') { e.preventDefault(); closeGo(); pos.focus(); return; }
		if (e.key !== 'Enter') { return; }
		e.preventDefault();
		var t = lpParseTime(go.value);
		if (isNaN(t) || t > lpTotal()) { go.classList.add('is-invalid'); return; }
		closeGo();
		lpSeekAbs(t);
	});
	go.addEventListener('blur', closeGo);
}

function lpTick() {
	var abs = lpAbs();
	if (!lp.tlDrag) { lpTlShow(abs); }
	lpQ('.lp-pos').textContent = lpFmt(abs);
	lpShowBuffered();
	var now = Date.now();
	if (!lp.music && (!lp.saved || now - lp.saved > 5000)) { lp.saved = now; lpStore(lpPosKey(), String(Math.floor(lpPosition()))); }
	// The chapter being played is highlighted.
	var cur = null;
	lpQ('.lp-chapters').querySelectorAll('[data-start]').forEach(function (b) { if (+b.dataset.start <= abs + 0.5) { cur = b; } });
	lpQ('.lp-chapters').querySelectorAll('[data-start]').forEach(function (b) { b.classList.toggle('active', b === cur); });
	if (lp.el.lpFeed && lp.el.lpFeed.broken && !lp.el.paused && lpAhead() < 5) { lpRestartBroken(); return; }
	lpSleepTick(abs);
	lpPreload();
	lpSessionPosition();
}

function lpEnded() {
	// A converted stream that ends well before the file does was cut off.
	if (!lpDirect() && lp.info && lp.info.duration && lpPosition() < lp.info.duration - 10) { lpRestartBroken(); return; }
	if (lp.repeat === 'one') { lpSeek(0); return; }
	lpStore(lpPosKey(), '0');
	if (lp.sleep && lp.sleep.mode === 'file') { lpSleepNow(); return; }
	lpStep(1, true);
}

// lpStep goes to the next (d=1) or previous (d=-1) entry; auto is the step
// at the end of a file, which stops at the end of the list unless repeating.
function lpStep(d, auto) {
	if (!lp) { return; }
	if (d > 0) {
		// The preloaded entry is the one lpNextIndex picked.
		if (lp.pre && lp.pre.idx >= 0) { lpLoad(lp.pre.idx); return; }
		var next = lpNextIndex(auto);
		if (next < 0 && !auto && lp.shuffle) { next = lpNextIndex(false); }
		if (next >= 0) { lpLoad(next); }
		return;
	}
	lpDiscardPre();
	var n = lp.list.length, i;
	if (lp.shuffle && n > 1) {
		if (lp.hist.length < 2) { return; }
		lp.hist.pop();
		lpLoad(lp.hist.pop());
		return;
	}
	i = lp.idx - 1;
	if (i < 0) { if (lp.repeat !== 'all') { return; } i = n - 1; }
	lpLoad(i);
}

// ---- Sleep timer (audiobooks) ----
function lpSetSleep(v) {
	if (!v) { lp.sleep = null; lpShowInfo(); return; }
	if (v === 'chapter') {
		var abs = lpAbs(), next = null;
		lpChapters().forEach(function (c) { if (next === null && c.start > abs + 1) { next = c.start; } });
		lp.sleep = { mode: 'chapter', at: next === null ? lpTotal() : next };
	} else if (v === 'file') {
		lp.sleep = { mode: 'file' };
		lpDiscardPre();
	} else {
		lp.sleep = { mode: 'time', at: Date.now() + (+v) * 60000 };
	}
	lpShowInfo();
}

function lpSleepText() {
	if (lp.sleep.mode === 'time') { return 'sleep in ' + Math.max(1, Math.ceil((lp.sleep.at - Date.now()) / 60000)) + ' min'; }
	return lp.sleep.mode === 'chapter' ? 'sleep at the end of the chapter' : 'sleep at the end of the part';
}

function lpSleepTick(abs) {
	if (!lp.sleep) { return; }
	if ((lp.sleep.mode === 'time' && Date.now() >= lp.sleep.at) || (lp.sleep.mode === 'chapter' && abs >= lp.sleep.at)) { lpSleepNow(); }
}

function lpSleepNow() {
	lp.sleep = null;
	lp.buffering = null;
	if (lp.el) { lp.el.pause(); }
	var s = lpQ('.lp-sleep');
	if (s) { s.value = ''; }
	lpShowInfo();
	libNotify('info', 'Sleep timer: paused');
}

// ---- Media keys and lock screen (Media Session API) ----
function lpSessionHandlers() {
	if (!('mediaSession' in navigator)) { return; }
	var ms = navigator.mediaSession;
	var set = function (action, fn) { try { ms.setActionHandler(action, fn); } catch (e) { /* not supported */ } };
	set('play', function () { if (lp && lp.el && lp.el.paused) { lpToggle(); } });
	set('pause', function () { if (lp && lp.el && !lp.el.paused) { lpToggle(); } });
	set('previoustrack', function () { lpStep(-1, false); });
	set('nexttrack', function () { lpStep(1, false); });
	set('seekbackward', function (d) { lpJump(-((d && d.seekOffset) || 10)); });
	set('seekforward', function (d) { lpJump((d && d.seekOffset) || 10); });
	set('seekto', function (d) { if (d && d.seekTime !== undefined) { lpSeekAbs(d.seekTime); } });
}

function lpSessionMeta() {
	if (!('mediaSession' in navigator) || !window.MediaMetadata) { return; }
	var art = lp.cover ? [{ src: new URL(lp.cover, location.href).href, sizes: '512x512' }] : [];
	var m = (lp.info && lp.info.meta) || {};
	navigator.mediaSession.metadata = new MediaMetadata({ title: m.title || lpItem().label, artist: m.artist || lp.entry || '',
		album: m.album || lp.entry || '', artwork: art });
}

function lpSessionPosition() {
	if (!('mediaSession' in navigator) || !navigator.mediaSession.setPositionState) { return; }
	var total = lpTotal();
	if (!(total > 0)) { return; }
	try { navigator.mediaSession.setPositionState({ duration: total, position: Math.min(lpAbs(), total), playbackRate: (lp.el && lp.el.playbackRate) || 1 }); } catch (e) { /* ignore */ }
}

function lpSessionClear() {
	if (!('mediaSession' in navigator)) { return; }
	navigator.mediaSession.metadata = null;
	navigator.mediaSession.playbackState = 'none';
	['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto'].forEach(function (a) {
		try { navigator.mediaSession.setActionHandler(a, null); } catch (e) { /* ignore */ }
	});
}

// ---- Keyboard ----
// Space play/pause, ←/→ 10 seconds, N/P next/previous, M mute, F full
// screen; not while typing in a field.
document.addEventListener('keydown', function (e) {
	if (!lp || !lp.el || e.ctrlKey || e.metaKey || e.altKey) { return; }
	var t = e.target, tag = t && t.tagName;
	if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (t && t.isContentEditable)) { return; }
	switch (e.key) {
		case ' ': case 'k': case 'K':
			if (tag === 'BUTTON' && e.key === ' ') { return; }
			e.preventDefault(); lpToggle(); break;
		case 'ArrowLeft': e.preventDefault(); lpJump(-10); break;
		case 'ArrowRight': e.preventDefault(); lpJump(10); break;
		case 'n': case 'N': lpStep(1, false); break;
		case 'p': case 'P': lpStep(-1, false); break;
		case 'm': case 'M': lp.el.muted = !lp.el.muted; break;
		case 'f': case 'F': lpFullscreen(); break;
		case 'Escape': if (document.getElementById('libPlayer').classList.contains('lp-fullwin')) { lpFullWindow(false); } break;
	}
});

// Stopping the media element ends the stream, which stops ffmpeg.
function lpStop() {
	if (lp) { lpDiscardPre(); lpStopHLS(); lp.ready = null; }
	if (lp && lp.el) { lpUnfeed(lp.el); lp.el.pause(); lp.el.removeAttribute('src'); lp.el.load(); }
	if (document.fullscreenElement) { document.exitFullscreen().catch(function () {}); }
	lpFullWindow(false);
	lp = null;
}

function lpClose() {
	lpStop();
	lpSessionClear();
	var box = document.getElementById('libPlayer');
	if (box) { box.classList.add('d-none'); box.innerHTML = ''; }
	lpDockSpace();
}

// The page and the dialog leave room for the docked player.
function lpDockSpace() {
	var box = document.getElementById('libPlayer');
	var on = box && !box.classList.contains('d-none');
	// Covering the window, the player needs no room below the page.
	if (on && box.classList.contains('lp-fullwin')) { return; }
	document.body.classList.toggle('lp-docked', !!on);
	document.documentElement.style.setProperty('--lp-h', on ? box.offsetHeight + 'px' : '0px');
}
if (window.ResizeObserver) { new ResizeObserver(lpDockSpace).observe(document.getElementById('libPlayer')); }

function libNotify(kind, msg) { if (typeof showToaster === 'function') { showToaster(kind, msg); } else { alert(msg); } }
`
