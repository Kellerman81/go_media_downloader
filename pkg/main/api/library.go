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
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The Library page shows the entries of one media group as a cover grid or
// a list, with search, filter, sort and paging; the detail dialog of an entry
// is in library_item.go, the page script in library_script.go.

// libPageSizes are the "per page" choices of the grid; 0 is all.
var libPageSizes = []int{50, 100, 200, 500, 1000, 0}

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
	// Rating is the metadata row's rating column ("" = none): sort by
	// rating, shown in the details (movies and series, 0-10).
	Rating string
}

var libKinds = []libKind{
	{
		Type: "movie", DbTable: "dbmovies", Prefix: "movie_", Label: t("Movies"), Icon: "fa-film", Rating: "d.vote_average",
		Select: `e.id, d.id AS db_id, d.title, '' AS sub, COALESCE(d.year, 0) AS year, COALESCE(d.overview, '') AS overview,
			e.listname, e.missing, EXISTS (SELECT 1 FROM movie_files f WHERE f.movie_id = e.id) AS have, e.dont_search`,
		From:    "movies e INNER JOIN dbmovies d ON d.id = e.dbmovie_id",
		FilesFK: "movie_id", Files: "movie_files", History: "movie_histories",
		SearchURL: "/api/movies/search/list/", Genre: "d.genres", Resolution: true,
		Incomplete: "(title = '' OR year = 0 OR overview = '')",
	},
	{
		Type: "series", DbTable: "dbseries", Prefix: "serie_", Label: t("Series"), Icon: "fa-tv", Rating: "d.rating",
		Select: `e.id, d.id AS db_id, d.seriename AS title, COALESCE(d.network, '') AS sub,
			CAST(COALESCE(substr(d.firstaired, 1, 4), '0') AS INTEGER) AS year, COALESCE(d.overview, '') AS overview,
			e.listname, ms.serie_id IS NOT NULL AS missing, hv.serie_id IS NOT NULL AS have, e.dont_search`,
		// serie_episodes and serie_episode_files have no index on serie_id:
		// each is grouped once instead of scanned per series (1.4 s -> 25 ms).
		From: `series e INNER JOIN dbseries d ON d.id = e.dbserie_id
			LEFT JOIN (SELECT DISTINCT se.serie_id FROM serie_episodes se
				INNER JOIN dbserie_episodes de ON de.id = se.dbserie_episode_id INNER JOIN dbseries ds ON ds.id = de.dbserie_id
				WHERE se.missing = 1 AND NOT (CAST(de.season AS INTEGER) = 0 AND ds.identifiedby IN ('ep', ''))) ms ON ms.serie_id = e.id
			LEFT JOIN (SELECT DISTINCT serie_id FROM serie_episode_files) hv ON hv.serie_id = e.id`,
		FilesFK: "serie_id", Files: "serie_episode_files", History: "serie_episode_histories",
		SearchURL: "/api/series/search/id/", Genre: "d.genre", Resolution: true,
		Incomplete: "(title = '' OR overview = '')",
	},
	{
		Type: "album", DbTable: "dbalbums", Prefix: "music_", Label: t("Music"), Icon: "fa-compact-disc", Square: true,
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
		Type: "audiobook", DbTable: "dbaudiobooks", Prefix: "audiobook_", Label: t("Audiobooks"), Icon: "fa-headphones", Square: true,
		Rating: "d.average_rating",
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
		Type: "book", DbTable: "dbbooks", Prefix: "book_", Label: t("Books"), Icon: "fa-book", Rating: "d.average_rating",
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
	User                              string // the account (Continue row)
	Lang                              string // the account's language (metadata translations)
	Group, Search, Filter, Sort, View string
	List, Genre                       string // extra filters
	Artist                            string // albums of an artist (dbartists ID, "0": without one)
	Coll                              int    // movies of a TMDB collection (0: all)
	Cast                              string // titles with this cast member (media_cast name)
	Res, YearFrom, YearTo             int    // resolution id, year range
	Page                              int
	Seed                              int64 // order of sort=random, fixed per page visit so paging is stable
	Rev                               bool  // the sort's other direction (rev=1)
	Size                              int   // entries per page, 0 = all (size=all)
}

var (
	libFilters = [][2]string{
		{"all", t("All")}, {"missing", t("Missing")}, {"have", t("Downloaded")},
		{"watched", t("Watched / heard")}, {"unwatched", t("Not watched / heard")}, {"incomplete", t("Metadata incomplete")},
	}
	libSorts = [][2]string{
		{"title", t("Title")}, {"added", t("Recently added")}, {"downloaded", t("Last downloaded")}, {"filed", t("Last file added")},
		{"year", t("Year")}, {"rating", t("Rating")}, {"random", t("Random")},
	}
)

func libQueryFrom(c *gin.Context) libQuery {
	return libQueryOf(c.Request.URL.Query(), accessOf(c), requestLanguage(c))
}

// libQueryOf reads the grid's state from parameters (a request's, a saved
// view's) for an account.
func libQueryOf(v url.Values, acc *userAccess, lang string) libQuery {
	q := libQuery{Group: v.Get("group"), Search: strings.TrimSpace(v.Get("q")), Filter: v.Get("filter"),
		Sort: v.Get("sort"), View: v.Get("view"), User: accName(acc), Lang: lang}
	if !acc.AllowsGroup(q.Group) {
		q.Group = "" // not a media group: every handler refuses it
	}
	q.Page, _ = strconv.Atoi(v.Get("page"))
	q.Seed, _ = strconv.ParseInt(v.Get("seed"), 10, 64)
	q.Rev = v.Get("rev") == "1"

	// Any size up to 1000 works; the page offers libPageSizes.
	q.Size = libDefaultPageSize
	if v.Get("size") == "all" {
		q.Size = 0
	} else if n, err := strconv.Atoi(v.Get("size")); err == nil && n > 0 && n <= 1000 {
		q.Size = n
	}
	q.List, q.Genre = v.Get("list"), strings.TrimSpace(v.Get("genre"))
	q.Res, _ = strconv.Atoi(v.Get("res"))
	q.YearFrom, _ = strconv.Atoi(v.Get("year_from"))
	q.YearTo, _ = strconv.Atoi(v.Get("year_to"))

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

	q.Artist = v.Get("artist")
	if _, err := strconv.ParseUint(q.Artist, 10, 64); err != nil {
		q.Artist = ""
	}

	q.Coll, _ = strconv.Atoi(v.Get("coll"))
	q.Cast = strings.TrimSpace(v.Get("cast"))

	if q.View != "list" && q.View != "cards" && q.View != "episodes" && q.View != "artists" && q.View != "collections" {
		q.View = "grid"
	}

	return q
}

func (q libQuery) values() url.Values {
	return url.Values{"group": {q.Group}, "q": {q.Search}, "filter": {q.Filter}, "sort": {q.Sort}, "view": {q.View},
		"page": {strconv.Itoa(q.Page)}, "seed": {strconv.FormatInt(q.Seed, 10)},
		"list": {q.List}, "genre": {q.Genre}, "res": {strconv.Itoa(q.Res)}, "artist": {q.Artist}, "coll": {strconv.Itoa(q.Coll)}, "cast": {q.Cast},
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
		html.Select(html.Class("form-select"), html.Name("sort"), gomponents.Attr("aria-label", t("Sort")),
			gomponents.Attr("onchange", "libShowSortDir()"), gomponents.Group(opts)),
		html.Input(html.Type("hidden"), html.Name("rev"), html.ID("libRev"), html.Value(map[bool]string{true: "1"}[q.Rev])),
		html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("libSortDir"),
			gomponents.Attr("onclick", "libToggleSortDir()"), gomponents.If(q.Sort == "random", html.Disabled()),
			html.Title(t("Reverse the order")), gomponents.Attr("aria-label", t("Reverse the order")),
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
	case "downloaded", "filed", "rating":
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
	Watched    bool `db:"watched"` // the account watched (heard) all its files
	Heard      bool `db:"-"`       // say "heard" (audiobooks)
	Read       bool `db:"-"`       // say "read" (books)
	// Done of Count: the episodes (under the grid filter) or audiobook
	// parts the account watched or heard (libProgress; Count 0: not shown).
	Done, Count int  `db:"-"`
	NoTitle     bool `db:"-"` // the metadata has no title; Title is the folder name
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

	if sw, sargs := libSearchWhere(q.Search, libTrSearches(g.Kind), libCastKind(g.Kind)); sw != "" {
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
	case "watched", "unwatched":
		if expr, wargs := libWatchedExpr(k, q.User); expr != "" {
			if q.Filter == "watched" {
				where += " AND " + expr
			} else {
				where += " AND have AND NOT " + expr
			}

			args = append(args, wargs...)
		}
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

	if q.Artist != "" && k.Type == "album" {
		artist, _ := strconv.ParseUint(q.Artist, 10, 64)
		where += " AND COALESCE((SELECT dbartist_id FROM dbalbum_artists WHERE dbalbum_id = db_id ORDER BY position LIMIT 1), 0) = ?"
		args = append(args, artist)
	}

	if ck := libCastKind(k); q.Cast != "" && ck != "" {
		where += " AND EXISTS (SELECT 1 FROM media_cast mc WHERE mc.kind = ? AND mc.db_id = e.db_id AND mc.name = ? COLLATE NOCASE)"
		args = append(args, ck, q.Cast)
	}

	if q.Coll > 0 && k.Type == "movie" {
		where += " AND db_id IN (SELECT id FROM dbmovies WHERE collection_id = ?)"
		args = append(args, q.Coll)
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

// libTrSearch is a metadata_translations source a search also looks in:
// the media type and the inner query's column with its id.
type libTrSearch struct{ mediaType, column string }

// libTrSearches are the translations a kind's search looks in: the entry's
// own, and in the episode view the episode's and its series'.
func libTrSearches(k *libKind) []libTrSearch {
	if k.Type == "episode" {
		return []libTrSearch{{"dbserie_episode", "ep_db_id"}, {"dbserie", "db_id"}}
	}

	if mt := database.TranslationMediaType(k.DbTable); mt != "" {
		return []libTrSearch{{mt, "db_id"}}
	}

	return nil
}

// libCastKind is the media_cast kind of a kind's rows, whose id is db_id
// (episodes: their series); "" without a cast.
func libCastKind(k *libKind) string {
	switch k.Type {
	case "movie", "series":
		return k.Type
	case "episode":
		return "series"
	}

	return ""
}

// libSearchWhere is the condition of a search over titles, artists/authors,
// the cast (castKind, "" none) and the plot or description, and their
// translations (any language, trs):
// `bravo | kuschelrock | "top 100"` matches any alternative, words of one
// alternative must all match (any order), -word excludes; & and AND join
// words explicitly. "" when there is nothing to search.
func libSearchWhere(search string, trs []libTrSearch, castKind string) (string, []any) {
	groups := libSearchGroups(search)
	if len(groups) == 0 {
		return "", nil
	}

	var args []any

	alts := make([]string, 0, len(groups))

	for _, g := range groups {
		terms := make([]string, 0, len(g))
		for _, t := range g {
			like := "%" + t.text + "%"
			cond := "COALESCE(title, '') LIKE ? OR COALESCE(sub, '') LIKE ? OR COALESCE(overview, '') LIKE ?"
			args = append(args, like, like, like)

			for _, tr := range trs {
				cond += " OR EXISTS (SELECT 1 FROM metadata_translations mt WHERE mt.media_type = ? AND mt.media_id = " +
					tr.column + " AND mt.translation LIKE ?)"
				args = append(args, tr.mediaType, like)
			}

			if castKind != "" {
				cond += " OR EXISTS (SELECT 1 FROM media_cast mc WHERE mc.kind = ? AND mc.db_id = e.db_id AND mc.name LIKE ?)"
				args = append(args, castKind, like)
			}

			cond = "(" + cond + ")"
			if t.not {
				cond = "NOT " + cond
			}

			terms = append(terms, cond)
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
		c.JSON(http.StatusNotFound, gin.H{"error": t("unknown media group")})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": t("these can't be played")})
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
	case "rating":
		if k.Rating != "" {
			// Unrated (empty or 0) last.
			return "NULLIF(CAST(NULLIF(" + k.Rating + ", '') AS REAL), 0)", ""
		}
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

	watched, wargs := libWatchedExpr(k, q.User)
	if watched == "" {
		watched = "0"
	}

	rows := database.StructscanT[libRow](false, q.capacity(),
		"SELECT id, db_id, title, sub, year, overview, listname, missing, have, genre, dont_search, "+watched+" AS watched FROM ("+
			inner+") e WHERE "+where+" ORDER BY "+order+q.limit(),
		append(wargs, args...)...)

	libFillTitles(k, rows)
	libTranslate(k, rows, q.Lang)

	for i := range rows {
		rows[i].Heard = k.Type == "audiobook" || k.Type == "album"
		rows[i].Read = k.Type == "book"
	}

	libProgress(k, rows, q.User, q.Filter)

	return rows, total
}

// libProgress fills Done and Count of a page of series (episodes; the grid
// filter decides which: downloaded, missing or all) or audiobooks (parts):
// one query for the page.
func libProgress(k *libKind, rows []libRow, user, filter string) {
	if user == "" || len(rows) == 0 || (k.Type != "series" && k.Type != "audiobook") {
		return
	}

	ids := make([]any, 0, len(rows)+2)
	ids = append(ids, user, watchKinds[k.Type])

	for i := range rows {
		ids = append(ids, rows[i].ID)
	}

	in := strings.TrimSuffix(strings.Repeat("?,", len(rows)), ",")
	watched := strconv.FormatFloat(database.Watched, 'f', 2, 64)

	var query string

	if k.Type == "series" {
		cond := ""

		switch filter {
		case "have":
			cond = " AND EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = se.id)"
		case "missing":
			cond = " AND se.missing = 1"
		}

		query = "SELECT se.serie_id AS id, count() AS count, sum(EXISTS (SELECT 1 FROM serie_episode_files f JOIN play_history h ON h.file_id = f.id " +
			"WHERE f.serie_episode_id = se.id AND h.username = ? AND h.kind = ? AND h.completion >= " + watched + ")) AS done " +
			"FROM serie_episodes se WHERE se.serie_id IN (" + in + ")" + cond + " GROUP BY se.serie_id"
	} else {
		query = "SELECT f.audiobook_id AS id, count() AS count, sum(EXISTS (SELECT 1 FROM play_history h WHERE h.file_id = f.id " +
			"AND h.username = ? AND h.kind = ? AND h.completion >= " + watched + ")) AS done " +
			"FROM audiobook_files f WHERE f.audiobook_id IN (" + in + ") GROUP BY f.audiobook_id"
	}

	counts := database.StructscanT[struct {
		ID    uint `db:"id"`
		Count int  `db:"count"`
		Done  int  `db:"done"`
	}](false, uint(len(rows)), query, ids...)

	byID := make(map[uint]int, len(counts))
	for i := range counts {
		byID[counts[i].ID] = i
	}

	for i := range rows {
		if j, ok := byID[rows[i].ID]; ok {
			rows[i].Done, rows[i].Count = counts[j].Done, counts[j].Count
		}
	}
}

// libProgressBadge is "5 / 8": of the episodes or parts the account
// watched or heard (nil without a count).
func libProgressBadge(r *libRow) gomponents.Node {
	if r.Count == 0 {
		return nil
	}

	icon, title := "fa-eye", tf("%d of %d episodes watched", r.Done, r.Count)
	if r.Heard {
		icon, title = "fa-headphones", tf("%d of %d parts heard", r.Done, r.Count)
	}

	return html.Span(html.Class("badge text-bg-dark lib-progress"), html.Title(title), gomponents.Attr("aria-label", title),
		html.I(html.Class("fa-solid "+icon+" me-1")), gomponents.Text(strconv.Itoa(r.Done)+" / "+strconv.Itoa(r.Count)))
}

// libEpisodeKind lists the episodes of series groups one by one (view
// "episodes"): title is the episode's, sub the series name.
var libEpisodeKind = libKind{
	Type: "episode", Label: t("Episodes"), Icon: "fa-tv",
	Select: `e.id, s.id AS serie_id, d.id AS db_id, de.id AS ep_db_id, COALESCE(de.title, '') AS title, d.seriename AS sub,
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
	EpDbID   uint   `db:"ep_db_id"`
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
		"SELECT id, serie_id, db_id, ep_db_id, title, sub, ident, aired, overview, missing, have, "+
			"COALESCE((SELECT f.id FROM serie_episode_files f WHERE f.serie_episode_id = e.id ORDER BY f.id LIMIT 1), 0) AS file_id "+
			"FROM ("+inner+") e WHERE "+where+
			" ORDER BY "+order+q.limit(),
		args...)

	libTranslateEpisodes(rows, q.Lang)

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
		badge = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text(t("Missing")))
	case r.Have:
		badge = html.Span(html.Class("badge bg-success"), gomponents.Text(t("Downloaded")))
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
				gomponents.If(r.FileID > 0, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success py-0"), html.Title(t("Play")),
					html.Data("play-kind", "episode"), html.Data("play-id", strconv.FormatUint(uint64(r.FileID), 10)),
					html.Data("play-list", "grid"), html.Data("play-order", strconv.Itoa(order)), html.Data("play-label", label),
					gomponents.Attr("onclick", "event.stopPropagation(); libPlayGrid(this)"),
					html.I(html.Class("fa-solid fa-play")))),
				html.Strong(html.Class("text-truncate"), html.Title(head), gomponents.Text(head)), badge),
			html.Div(html.Class("text-truncate"), html.Title(r.Title), gomponents.Text(r.Title)),
			gomponents.If(r.Aired != "", html.Div(html.Class("small text-muted"), gomponents.Text(tf("Aired %s", r.Aired)))),
			gomponents.If(overview != "", html.P(html.Class("small mb-0 mt-1"), gomponents.Text(overview))),
		),
	)
}

func renderLibraryEpisodes(g *libGroup, q libQuery) gomponents.Node {
	rows, total := libraryEpisodeRows(g, q)
	if total == 0 {
		return html.P(html.Class("text-muted text-center py-5"), gomponents.Text(t("Nothing found.")))
	}

	items := make([]gomponents.Node, 0, len(rows))
	for i := range rows {
		items = append(items, libEpisodeItem(&rows[i], i))
	}

	return gomponents.Group([]gomponents.Node{
		libCountLine(q, total, "%d episodes", true),
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
		c.JSON(http.StatusNotFound, gin.H{"error": t("unknown media group")})
		return
	}

	g := &groups[idx]
	where, args := libWhere(g, q)

	id := database.Getdatarow[uint](false,
		"SELECT id FROM ("+libInner(g.Kind, "NULL", "")+") e WHERE "+where+" ORDER BY random() LIMIT 1", args...)
	if id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": t("nothing matches the search and filter")})
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

	var content gomponents.Node = html.P(html.Class("text-muted"), gomponents.Text(t("No media groups are configured.")))

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

	if !acc.CanDownload() {
		hide = append(hide, ".lib-download")
	}

	if !acc.CanReport() {
		hide = append(hide, ".lib-report")
	}

	renderPageIn(c, t("Library"), false, html.Div(
		html.Class("config-section-enhanced"),
		gomponents.If(len(hide) > 0, html.StyleEl(gomponents.Raw(strings.Join(hide, ", ")+" { display: none !important; }"))),
		renderEnhancedPageHeader("fa-solid fa-photo-film", t("Library"), t("Browse the movies, series, albums, audiobooks and books of your media groups.")),
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
	music := cur != nil && cur.Kind.Type == "album"
	movies := cur != nil && cur.Kind.Type == "movie"

	viewBtn := func(view, icon, label string) gomponents.Node {
		class := "btn btn-outline-secondary"
		// The episode view is only offered for series, the artists for
		// music, the collections for movies (libSwitchGroup).
		offered := (view != "episodes" || series) && (view != "artists" || music) && (view != "collections" || movies)
		if !offered {
			class += " d-none"
		}

		return gomponents.Group([]gomponents.Node{
			html.Input(html.Type("radio"), html.Class("btn-check"), html.Name("view"), html.ID("libView"+view), html.Value(view),
				gomponents.If(q.View == view && offered, html.Checked())),
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
			html.Input(html.Type("hidden"), html.Name("artist"), html.ID("libArtist"), html.Value(q.Artist)),
			html.Input(html.Type("hidden"), html.Name("coll"), html.ID("libColl"), html.Value(strconv.Itoa(q.Coll))),
			html.Input(html.Type("hidden"), html.Name("cast"), html.ID("libCast"), html.Value(q.Cast)),
			html.Div(html.Class("card-body row g-2 align-items-center"),
				html.Div(html.Class("col-12 col-lg-3"), html.Div(html.Class("input-group flex-nowrap"),
					html.Input(html.Type("search"), html.Class("form-control"), html.ID("libSearch"), html.Name("q"),
						html.Value(q.Search), html.Placeholder(t("Search… a & b | c, \"phrase\", -not")),
						html.Title(t("Words must all match (& or AND joins them explicitly). | or OR: any of the alternatives. \"…\": exact phrase. -word: exclude.")+"\n"+
							t("Example: bravo & hits | kuschelrock | \"top 100\" | chart -xmas"))),
					// Saved views: search and filters under a name (libSaveView).
					html.Button(html.Type("button"), html.Class("btn btn-outline-secondary dropdown-toggle"), html.ID("libSavedBtn"),
						gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
						html.Title(t("Saved views")), gomponents.Attr("aria-label", t("Saved views")),
						html.I(html.Class("fa-regular fa-bookmark"))),
					html.Ul(html.Class("dropdown-menu dropdown-menu-end"), html.ID("libSavedMenu")))),
				selectOf("filter", t("Filter"), libFilters, q.Filter),
				libSortControl(q, libSorts),
				html.Div(html.Class("col-12 col-lg-4 d-flex flex-wrap align-items-center justify-content-lg-end gap-2"),
					libPlaylistsButton(),
					html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("libSelectBtn"),
						html.Title(t("Select several entries for search, refresh or delete")), gomponents.Attr("onclick", "libSelectMode()"),
						html.I(html.Class("fa-regular fa-square-check me-1")), gomponents.Text(t("Select"))),
					html.Button(html.Type("button"), html.Class("btn btn-outline-primary"), html.ID("libRandomBtn"),
						html.Title(t("Open a random entry that matches the search and filter")), gomponents.Attr("onclick", "libRandom()"),
						html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text(t("Random"))),
					html.Div(html.Class("btn-group"), html.Role("group"), gomponents.Attr("aria-label", t("View")),
						viewBtn("grid", "fa-grip", t("Covers")), viewBtn("cards", "fa-table-cells-large", t("Covers with descriptions")),
						viewBtn("list", "fa-list", t("List with descriptions")),
						viewBtn("episodes", "fa-list-ol", t("Episodes, without grouping by series")),
						viewBtn("artists", "fa-users", t("Artists")),
						viewBtn("collections", "fa-layer-group", t("Collections"))),
					// Cover size: more or fewer covers per row (libSetCoverSize).
					html.Span(html.Class("d-inline-flex align-items-center gap-1 text-nowrap"), html.Title(t("Cover size")),
						html.I(html.Class("fa-regular fa-image small text-muted")),
						html.Input(html.Type("range"), html.Class("lib-size"), html.ID("libCoverSize"), html.Style("width:6rem"),
							html.Min("90"), html.Max("320"), html.Step("10"), html.Value("150"), gomponents.Attr("aria-label", t("Cover size")),
							gomponents.Attr("oninput", "libSetCoverSize(+this.value)")),
						html.I(html.Class("fa-regular fa-image text-muted")))),
			),
			renderLibraryExtraFilters(libGroupByName(q.Group), q),
		),
		html.Div(html.ID("libSelBar"), html.Class("lib-selbar d-none"),
			html.Span(html.Class("fw-semibold me-2"), html.ID("libSelCount"), gomponents.Text(t("0 selected"))),
			libSelBtn("libSelectPage()", "btn-outline-secondary", "fa-check-double", t("Select all on this page")),
			libSelBtn("libSelectClear()", "btn-outline-secondary", "fa-eraser", t("Clear")),
			html.Span(html.Class("flex-grow-1")),
			libSearchMenu(func(title bool) string {
				return "libBulk('search" + map[bool]string{true: "title"}[title] + "')"
			}, t("Search")),
			libPlaylistAddMenu("selection", t("Playlist"), "btn-outline-success"),
			libSelBtn("libBulk('watched')", "btn-outline-success", "fa-check", t("Watched")),
			libSelBtn("libBulk('unwatched')", "btn-outline-secondary", "fa-eye-slash", t("Not watched")),
			libSelBtn("libBulk('refresh')", "btn-outline-primary lib-admin", "fa-rotate", t("Refresh metadata")),
			libDeleteMenu(func(files, exclude bool) string {
				return "libBulk('delete" + map[bool]string{true: "files"}[files] + map[bool]string{true: "excl"}[exclude] + "')"
			}, true, t("and their files")),
			libSelBtn("libSelectMode(false)", "btn-outline-secondary", "fa-xmark", t("Done")),
		),
		html.Div(html.ID("libGrid"), renderLibraryGrid(q)),
		renderLibraryModal(),
		renderPlaylistModal(),
		renderReaderModal(),
		// The player docks at the bottom of the page, outside the dialog, so
		// it keeps playing while the library is browsed.
		playerDock(),
		html.StyleEl(gomponents.Raw(libraryCSS + playerCSS + playlistCSS + readerCSS)),
		html.Script(gomponents.Raw(playerJS)),
		html.Script(gomponents.Raw(strings.ReplaceAll(libraryJS, "{{csrf}}", jsString(csrfToken)))),
		html.Script(gomponents.Raw(playlistJS)),
		html.Script(gomponents.Raw(favoriteJS)),
		html.Script(gomponents.Raw(readerJS)),
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

		cols = append(cols, sel("list", t("All lists"), q.List, opts))
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
			cols = append(cols, sel("res", t("Any resolution"), strconv.Itoa(q.Res), opts))
		}
	}

	if genres := libGenres(g); len(genres) > 0 {
		opts := make([][2]string, 0, len(genres))
		for _, genre := range genres {
			opts = append(opts, [2]string{genre, genre})
		}

		cols = append(cols, sel("genre", t("Any genre"), q.Genre, opts))
	}

	year := func(name, label string, v int) gomponents.Node {
		value := ""
		if v > 0 {
			value = strconv.Itoa(v)
		}

		return html.Div(html.Class("col-6 col-md-2"), html.Input(html.Type("number"), html.Class("form-control form-control-sm"),
			html.Name(name), html.Placeholder(label), html.Value(value), html.Min("1800"), html.Max("2200"), gomponents.Attr("aria-label", label)))
	}

	cols = append(cols, year("year_from", t("From year"), q.YearFrom), year("year_to", t("To year"), q.YearTo))

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
		c.JSON(http.StatusNotFound, gin.H{"error": t("nothing to play")})
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

// HandleLibraryGrid renders the grid fragment (/admin/library/grid).
func HandleLibraryGrid(c *gin.Context) {
	c.String(http.StatusOK, renderComponentToString(renderLibraryGrid(libQueryFrom(c))))
}

func renderLibraryGrid(q libQuery) gomponents.Node {
	groups := libraryGroups()

	idx := slices.IndexFunc(groups, func(g libGroup) bool { return g.Name == q.Group })
	if idx < 0 {
		return html.P(html.Class("text-muted"), gomponents.Text(t("Unknown media group.")))
	}

	g := &groups[idx]

	// What the account didn't finish, above the first page.
	var cont gomponents.Node
	// Not while an artist, collection or cast filter narrows the grid: the
	// rows would push its chip out of sight.
	if q.Page <= 1 && q.Search == "" && q.Artist == "" && q.Coll == 0 && q.Cast == "" {
		cont = gomponents.Group([]gomponents.Node{libContinueRow(q.User, g), libNextUpRow(q.User, q.Lang, g)})
	}

	// Episodes are a view of series groups only, artists of music.
	if q.View == "episodes" {
		if g.Kind.Type == "series" {
			return gomponents.Group([]gomponents.Node{cont, renderLibraryEpisodes(g, q)})
		}

		q.View = "grid"
	}

	if q.View == "artists" {
		if g.Kind.Type == "album" {
			return renderLibraryArtists(g, q)
		}

		q.View = "grid"
	}

	if q.View == "collections" {
		if g.Kind.Type == "movie" {
			return renderLibraryCollections(g, q)
		}

		q.View = "grid"
	}

	rows, total := libraryRows(g, q)

	if total == 0 {
		empty := t("Nothing found.")
		if q.Cast != "" && libCastKind(g.Kind) != "" {
			// The cast is saved when a title is refreshed or its More tab is
			// opened; until then the filter can't find it.
			empty = t("Nothing found. The cast of a title is saved when its metadata is refreshed or its More tab is opened, so titles not refreshed yet are missing here.")
		}

		return gomponents.Group([]gomponents.Node{libArtistChip(g, q), libCollectionChip(g, q), libCastChip(g, q), cont,
			html.P(html.Class("text-muted text-center py-5"), gomponents.Text(empty))})
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
		libArtistChip(g, q),
		libCollectionChip(g, q),
		libCastChip(g, q),
		cont,
		libCountLine(q, total, "%d entries", libPlayKinds[g.Kind.Type] != ""),
		body,
		libPager(q, total),
	})
}

func libCoverURL(k *libKind, r *libRow) string {
	return "/api/admin/library/cover/" + k.Type + "/" + strconv.FormatUint(uint64(r.DbID), 10)
}

func libStatusBadge(r *libRow) gomponents.Node {
	switch {
	case r.Watched:
		label := t("Watched")
		if r.Heard {
			label = t("Heard")
		} else if r.Read {
			label = t("Read")
		}

		return html.Span(html.Class("badge bg-info"), html.I(html.Class("fa-solid fa-check me-1")), gomponents.Text(label))
	case r.Missing:
		return html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text(t("Missing")))
	case r.Have:
		return html.Span(html.Class("badge bg-success"), gomponents.Text(t("Downloaded")))
	}

	return nil
}

// libWatchedExpr is the SQL condition "the account watched (heard, read)
// every file of the entry e" with its arguments ("" for a type without
// watched state). Series: every downloaded episode; albums: every track.
func libWatchedExpr(k *libKind, user string) (string, []any) {
	kind := watchKinds[k.Type]
	if kind == "" || user == "" {
		return "", nil
	}

	return "(EXISTS (SELECT 1 FROM " + k.Files + " wf WHERE wf." + k.FilesFK + " = e.id) AND NOT EXISTS (SELECT 1 FROM " + k.Files +
		" wf WHERE wf." + k.FilesFK + " = e.id AND NOT EXISTS (SELECT 1 FROM play_history wh WHERE wh.username = ? AND wh.kind = ? " +
		"AND wh.file_id = wf.id AND wh.completion >= " + strconv.FormatFloat(database.Watched, 'f', 2, 64) + ")))", []any{user, kind}
}

// libDeleteMenu is the Delete dropdown (admins): the entry alone or with
// its files, each also as "don't add again from lists" (import
// exclusion). onclick builds the call; files offers the variants with
// files, filesLabel names them.
func libDeleteMenu(onclick func(files, exclude bool) string, files bool, filesLabel string) gomponents.Node {
	item := func(icon, label string, files, exclude bool) gomponents.Node {
		return html.Li(html.Button(html.Type("button"), html.Class("dropdown-item"+map[bool]string{true: " text-danger"}[files]),
			gomponents.Attr("onclick", onclick(files, exclude)),
			html.I(html.Class("fa-solid "+icon+" me-2")), gomponents.Text(label)))
	}

	return html.Div(html.Class("btn-group lib-admin"),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger dropdown-toggle"),
			gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
			html.I(html.Class("fa-solid fa-trash me-1")), gomponents.Text(t("Delete"))),
		html.Ul(html.Class("dropdown-menu dropdown-menu-end"),
			item("fa-trash", t("Delete entry (files kept)"), false, false),
			item("fa-ban", t("Delete entry, don't add again from lists"), false, true),
			gomponents.If(files, html.Li(html.Hr(html.Class("dropdown-divider")))),
			gomponents.If(files, item("fa-trash-can", tf("Delete entry %s", filesLabel), true, false)),
			gomponents.If(files, item("fa-ban", tf("Delete entry %s, don't add again", filesLabel), true, true))))
}

// libSearchMenu is the Search button as a dropdown: by the entry's IDs
// (IMDb, TVDB, ...) only, or also by its title. Every search URL gets
// searchByTitle explicitly, as the endpoints default differently.
func libSearchMenu(onclick func(title bool) string, label string) gomponents.Node {
	item := func(icon, text string, title bool) gomponents.Node {
		return html.Li(html.Button(html.Type("button"), html.Class("dropdown-item"),
			gomponents.Attr("onclick", onclick(title)),
			html.I(html.Class("fa-solid "+icon+" me-2")), gomponents.Text(text)))
	}

	return html.Div(html.Class("btn-group lib-search"),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-primary dropdown-toggle"),
			gomponents.Attr("data-bs-toggle", "dropdown"), gomponents.Attr("aria-expanded", "false"),
			html.I(html.Class("fa-solid fa-magnifying-glass me-1")), gomponents.Text(label)),
		html.Ul(html.Class("dropdown-menu"),
			item("fa-fingerprint", t("Search by ID"), false),
			item("fa-font", t("Search by ID and title"), true)))
}

// libRatingBadge shows the rating of a movie or series (0-10, TMDB or
// TheTVDB) in the details; nothing without one.
func libRatingBadge(k *libKind, dbID uint) gomponents.Node {
	if k.Type != "movie" && k.Type != "series" {
		return nil
	}

	r := database.StructscanT[struct {
		Rating float64 `db:"rating"`
		Votes  int     `db:"votes"`
	}](false, 1, "SELECT COALESCE(CAST(NULLIF("+k.Rating+", '') AS REAL), 0) AS rating, "+
		map[bool]string{true: "COALESCE(d.vote_count, 0)", false: "0"}[k.Type == "movie"]+" AS votes FROM "+k.DbTable+" d WHERE d.id = ?", &dbID)
	if len(r) == 0 || r[0].Rating <= 0 {
		return nil
	}

	title := t("Rating")
	if r[0].Votes > 0 {
		title += " (" + strconv.Itoa(r[0].Votes) + " votes)"
	}

	return html.Span(html.Class("badge text-bg-warning"), html.Title(title),
		html.I(html.Class("fa-solid fa-star me-1")), gomponents.Text(strconv.FormatFloat(r[0].Rating, 'f', 1, 64)+" / 10"))
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
		return gomponents.Group([]gomponents.Node{html.Class("fst-italic"), html.Title(t("The metadata has no title; this is the folder name"))})
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
			libProgressBadge(r),
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
			libProgressBadge(r),
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
				html.Strong(html.Class("text-truncate"), libTitleAttrs(r), gomponents.Text(r.Title)), libStatusBadge(r), libProgressBadge(r)),
			html.Div(html.Class("small text-muted"), gomponents.Text(meta)),
			gomponents.If(overview != "", html.P(html.Class("small mb-0 mt-1"), gomponents.Text(overview))),
		),
	)
}

// libCountLine is "123 entries" with the per-page choice next to it; the
// choice goes into the form's size field (libSetPageSize).
// countFormat is the "%d entries" text of the count, a marked format.
func libCountLine(q libQuery, total int, countFormat string, playable bool) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(libPageSizes)+1)
	if !slices.Contains(libPageSizes, q.Size) {
		opts = append(opts, html.Option(html.Value(q.sizeParam()), html.Selected(), gomponents.Text(q.sizeParam())))
	}

	for _, n := range libPageSizes {
		v, label := strconv.Itoa(n), strconv.Itoa(n)
		if n == 0 {
			v, label = "all", t("All")
		}

		opts = append(opts, html.Option(html.Value(v), gomponents.If(n == q.Size, html.Selected()), gomponents.Text(label)))
	}

	return html.Div(html.Class("d-flex align-items-center gap-2 text-muted small mb-2"),
		gomponents.Text(tf(countFormat, total)),
		html.Span(gomponents.Text("·")),
		html.Label(html.For("libPageSize"), gomponents.Text(t("Per page"))),
		html.Select(html.Class("form-select form-select-sm w-auto"), html.ID("libPageSize"),
			gomponents.Attr("onchange", "libSetPageSize(this.value)"), gomponents.Group(opts)),
		// Everything found (all pages) as the player's playlist.
		gomponents.If(playable && total > 0, gomponents.Group([]gomponents.Node{
			html.Span(html.Class("flex-grow-1")),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"),
				html.Title(tf("Play everything found (all pages, up to %d files)", libPlayAllMax)),
				gomponents.Attr("onclick", "libPlayAll(false)"), html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text(t("Play all"))),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"),
				html.Title(t("Play everything found in random order")),
				gomponents.Attr("onclick", "libPlayAll(true)"), html.I(html.Class("fa-solid fa-shuffle me-1")), gomponents.Text(t("Shuffle"))),
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

	return html.Nav(html.Class("mt-3"), gomponents.Attr("aria-label", t("Pages")),
		html.Ul(html.Class("pagination justify-content-center flex-wrap"), gomponents.Group(nodes)),
		html.P(html.Class("text-center text-muted small"), gomponents.Text(tf("Page %d of %d", q.Page, pages))))
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
						gomponents.Attr("aria-label", t("Close")))),
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
func libPlayButton(kind string, fileID uint, list string, order int, label string, watched, last bool) gomponents.Node {
	return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success py-0 me-1"), html.Title(t("Play")),
		html.Data("play-kind", kind), html.Data("play-id", strconv.FormatUint(uint64(fileID), 10)),
		html.Data("play-list", list), html.Data("play-order", strconv.Itoa(order)), html.Data("play-label", label),
		gomponents.If(watched, html.Data("watched", "1")),
		gomponents.If(last, html.Data("last", "1")),
		gomponents.Attr("onclick", "libPlay(this)"),
		html.I(html.Class("fa-solid fa-play")))
}
