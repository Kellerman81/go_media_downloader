package api

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// ---------------------------------------------------------------------------
// Single source of truth for the generic SQLite admin table browser's
// per-table filters.
//
// This used to be split across three independently hand-maintained lists
// that had already drifted out of sync at least once (see FINDINGS_LOG.md,
// "api/ — Generic SQLite table browser"):
//   - getFilterableFieldsForTable (web.go)                 — UI inputs
//   - renderCustomFilters's fallback switch (web_grids.go) — more UI inputs
//   - buildCustomFilters's filterMappings + switch (web_database.go) — SQL
//
// filterFieldDefs is now the one place a table's filter fields are defined.
// renderCustomFilters (UI) and buildCustomFilters (SQL) both read from it
// instead of maintaining their own list.
//
// Where the old code applied more than one SQL fragment for the very same
// field on the very same table (the generic map and the legacy switch often
// both fired — sometimes with identical columns, sometimes with a
// differently-qualified column, sometimes with a different operator), this
// is preserved verbatim as multiple entries in that field's Clauses slice
// rather than "simplified" to a single net-effect clause — that keeps this a
// pure, provably-equivalent refactor instead of a behavior change disguised
// as one.
// ---------------------------------------------------------------------------

// filterOp identifies how a filterClause turns a submitted value into a SQL
// WHERE fragment.
type filterOp int

const (
	opLike      filterOp = iota // Column LIKE '%value%'
	opEq                        // Column = value
	opGte                       // Column >= value
	opDateEq                    // DATE(Column) = value
	opNullPair                  // value "1" -> Column IS NOT NULL, "0" -> Column IS NULL (no bind arg)
	opLiteral01                 // value "1" -> Column = 1, "0" -> Column = 0 (literal, no bind arg)
	opRaw                       // RawSQL template; RawArgs(value) supplies the bind args
)

// filterClause is one SQL WHERE fragment applied when its field's query
// parameter is non-empty.
type filterClause struct {
	Column  string
	Op      filterOp
	RawSQL  string
	RawArgs func(value string) []any

	// Legacy marks a clause that originated in the old generic filterMappings
	// map (as opposed to the old "legacy switch"). buildCustomFilters's
	// dynamic generic "_id" catch-all only skipped fields that were already
	// present in that old map — this flag reproduces that exact, narrower
	// exclusion set so the catch-all's behavior is unchanged too.
	Legacy bool
}

func like(column string) filterClause      { return filterClause{Column: column, Op: opLike} }
func eqCol(column string) filterClause     { return filterClause{Column: column, Op: opEq} }
func gte(column string) filterClause       { return filterClause{Column: column, Op: opGte} }
func dateEq(column string) filterClause    { return filterClause{Column: column, Op: opDateEq} }
func nullPair(column string) filterClause  { return filterClause{Column: column, Op: opNullPair} }
func literal01(column string) filterClause { return filterClause{Column: column, Op: opLiteral01} }

func rawClause(sql string, args func(string) []any) filterClause {
	return filterClause{Op: opRaw, RawSQL: sql, RawArgs: args}
}

// legacyClause marks c as having originated in the old filterMappings map.
func legacyClause(c filterClause) filterClause {
	c.Legacy = true
	return c
}

// apply evaluates the clause for a submitted value, returning the SQL
// fragment and its bind args. ok is false when the clause does not fire for
// this value (opNullPair/opLiteral01 with a value other than "1"/"0").
func (c filterClause) apply(value string) (cond string, args []any, ok bool) {
	switch c.Op {
	case opLike:
		return c.Column + " LIKE ?", []any{"%" + value + "%"}, true
	case opEq:
		return c.Column + " = ?", []any{value}, true
	case opGte:
		return c.Column + " >= ?", []any{value}, true
	case opDateEq:
		return "DATE(" + c.Column + ") = ?", []any{value}, true
	case opNullPair:
		switch value {
		case "1":
			return c.Column + " IS NOT NULL", nil, true
		case "0":
			return c.Column + " IS NULL", nil, true
		default:
			return "", nil, false
		}
	case opLiteral01:
		switch value {
		case "1":
			return c.Column + " = 1", nil, true
		case "0":
			return c.Column + " = 0", nil, true
		default:
			return "", nil, false
		}
	case opRaw:
		return c.RawSQL, c.RawArgs(value), true
	default:
		return "", nil, false
	}
}

// tableFilterField is one filterable field for one admin DB browser table:
// how to render its filter UI (if any — some fields are SQL-only, reachable
// only via a crafted query string, exactly as before this refactor) and how
// to translate a submitted value into one or more SQL WHERE fragments.
type tableFilterField struct {
	Field string // matches the "filter-<Field>" query param / input id

	// UI. Label == "" means: render no input for this field (it only ever
	// affected SQL before this refactor too — several such gaps existed
	// pre-existing between the old UI list and the old SQL list).
	Label        string
	Widget       string // "text","number","datetime-local","date","select","select-quality"
	Placeholder  string
	Options      []string // static <select> values, parallel to OptionLabels
	OptionLabels []string

	Clauses []filterClause
}

// yesNoOptions/yesNoLabels is the "All/No/Yes" select shape repeated across
// most boolean columns.
var (
	yesNoOptions = []string{"", "0", "1"}
	yesNoLabels  = []string{"All", "No", "Yes"}
)

// filterFieldDefs is the single source of truth described above.
var filterFieldDefs = map[string][]tableFilterField{
	"dbmovies": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("title"))}},
		{Field: "year", Label: "Year", Widget: "number", Placeholder: "Year...", Clauses: []filterClause{legacyClause(eqCol("year"))}},
		{Field: "imdb_id", Label: "IMDB ID", Widget: "text", Placeholder: "tt1234567...", Clauses: []filterClause{legacyClause(like("imdb_id"))}},
		{Field: "vote_average", Label: "Vote Average", Widget: "number", Placeholder: "Rating...", Clauses: []filterClause{legacyClause(gte("vote_average"))}},
		{Field: "runtime", Label: "Runtime", Widget: "number", Placeholder: "Minutes...", Clauses: []filterClause{legacyClause(gte("runtime"))}},
		{Field: "original_language", Label: "Language", Widget: "text", Placeholder: "en, de, fr...", Clauses: []filterClause{legacyClause(eqCol("original_language"))}},
		{Field: "adult", Label: "Adult Content", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("adult"))}},
		{Field: "status", Label: "Status", Widget: "text", Placeholder: "Status...", Clauses: []filterClause{legacyClause(like("status"))}},
	},
	"movies": {
		{Field: "title", Label: "Movie Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("dbmovies.title")}},
		{Field: "imdb_id", Label: "IMDB ID", Widget: "text", Placeholder: "Filter by IMDB ID...", Clauses: []filterClause{like("dbmovies.imdb_id")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality profile...", Clauses: []filterClause{legacyClause(like("quality_profile")), eqCol("movies.quality_profile")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("movies.listname")}},
		{Field: "rootpath", Label: "Root Path", Widget: "text", Placeholder: "Path...", Clauses: []filterClause{legacyClause(like("rootpath")), like("movies.rootpath")}},
		{Field: "quality_reached", Label: "Quality Reached", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("quality_reached")), eqCol("movies.quality_reached")}},
		{Field: "missing", Label: "Missing", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("missing")), eqCol("movies.missing")}},
		// SQL-only (no live UI input — the dead renderCustomFilters fallback
		// for "movies" never renders because getFilterableFieldsForTable
		// already had entries for this table).
		{Field: "year", Clauses: []filterClause{eqCol("dbmovies.year")}},
	},
	"dbseries": {
		{Field: "seriename", Label: "Series Name", Widget: "text", Placeholder: "Filter by series name...", Clauses: []filterClause{legacyClause(like("seriename")), like("seriename")}},
		{Field: "status", Label: "Status", Widget: "text", Placeholder: "Status...", Clauses: []filterClause{legacyClause(like("status"))}},
		{Field: "genre", Label: "Genre", Widget: "text", Placeholder: "Genre...", Clauses: []filterClause{legacyClause(like("genre"))}},
		{Field: "imdb_id", Label: "IMDB ID", Widget: "text", Placeholder: "tt1234567...", Clauses: []filterClause{legacyClause(like("imdb_id"))}},
		{Field: "thetvdb_id", Label: "TVDB ID", Widget: "number", Placeholder: "TVDB ID...", Clauses: []filterClause{legacyClause(eqCol("thetvdb_id")), eqCol("thetvdb_id")}},
	},
	"qualities": {
		{Field: "type", Label: "Type", Widget: "select", Options: []string{"", "1", "2", "3", "4"}, OptionLabels: []string{"All", "Resolution", "Quality", "Codec", "Audio"}, Clauses: []filterClause{legacyClause(eqCol("type")), eqCol("type")}},
		{Field: "name", Label: "Name", Widget: "text", Placeholder: "Quality name...", Clauses: []filterClause{legacyClause(like("name")), like("name")}},
		{Field: "regex", Label: "Regex", Widget: "text", Placeholder: "Regular expression...", Clauses: []filterClause{legacyClause(like("regex"))}},
		{Field: "strings", Label: "Strings", Widget: "text", Placeholder: "String patterns...", Clauses: []filterClause{legacyClause(like("strings"))}},
		{Field: "priority", Label: "Priority", Widget: "number", Placeholder: "Priority...", Clauses: []filterClause{legacyClause(eqCol("priority")), eqCol("priority")}},
		{Field: "use_regex", Label: "Use Regex", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("use_regex")), eqCol("use_regex")}},
	},
	"series": {
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("series.listname")}},
		{Field: "rootpath", Label: "Root Path", Widget: "text", Placeholder: "Path...", Clauses: []filterClause{legacyClause(like("rootpath")), like("series.rootpath")}},
		// SQL-only (dead UI — "series" already has entries above, so
		// renderCustomFilters's dynamic fallback for it never renders).
		{Field: "seriename", Clauses: []filterClause{like("dbseries.seriename")}},
		{Field: "dont_upgrade", Clauses: []filterClause{eqCol("series.dont_upgrade")}},
		{Field: "dont_search", Clauses: []filterClause{eqCol("series.dont_search")}},
		{Field: "search_specials", Clauses: []filterClause{eqCol("series.search_specials")}},
		{Field: "ignore_runtime", Clauses: []filterClause{eqCol("series.ignore_runtime")}},
	},
	"movie_files": {
		{Field: "movie_id", Label: "Movie ID", Widget: "text", Placeholder: "Movie ID...", Clauses: []filterClause{eqCol("movie_files.movie_id")}},
		{Field: "location", Label: "Location", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("location"))}},
		{Field: "filename", Label: "Filename", Widget: "text", Placeholder: "Filename...", Clauses: []filterClause{legacyClause(like("filename")), like("movie_files.filename")}},
		{Field: "extension", Label: "Extension", Widget: "text", Placeholder: "Extension...", Clauses: []filterClause{legacyClause(eqCol("extension"))}},
		{Field: "quality_profile", Label: "Quality", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile")), like("movie_files.quality_profile")}},
		// SQL-only (dead UI text field from the unreachable renderCustomFilters fallback).
		{Field: "resolution", Clauses: []filterClause{like("movie_files.resolution")}},
	},
	"serie_episode_files": {
		{Field: "serie_episode_id", Label: "Episode ID", Widget: "text", Placeholder: "Episode ID...", Clauses: []filterClause{eqCol("serie_episode_files.serie_episode_id")}},
		{Field: "location", Label: "Location", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("location"))}},
		{Field: "filename", Label: "Filename", Widget: "text", Placeholder: "Filename...", Clauses: []filterClause{legacyClause(like("filename")), like("serie_episode_files.filename")}},
		{Field: "extension", Label: "Extension", Widget: "text", Placeholder: "Extension...", Clauses: []filterClause{legacyClause(eqCol("extension"))}},
		// SQL-only.
		{Field: "quality_profile", Clauses: []filterClause{like("serie_episode_files.quality_profile")}},
		{Field: "resolution", Clauses: []filterClause{like("serie_episode_files.resolution")}},
	},
	"dbmovie_titles": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("dbmovie_titles.title")), like("dbmovie_titles.title")}},
		{Field: "movie_title", Label: "Movie Name", Widget: "text", Placeholder: "Filter by movie name...", Clauses: []filterClause{legacyClause(like("dbmovies.title")), like("dbmovies.title")}},
		{Field: "region", Label: "Region", Widget: "text", Placeholder: "Region...", Clauses: []filterClause{legacyClause(like("dbmovie_titles.region")), like("dbmovie_titles.region")}},
	},
	"job_histories": {
		{Field: "job_type", Label: "Job Type", Widget: "text", Placeholder: "Job type...", Clauses: []filterClause{legacyClause(like("job_type")), like("job_type")}},
		{Field: "job_group", Label: "Job Group", Widget: "text", Placeholder: "Job group...", Clauses: []filterClause{legacyClause(like("job_group")), like("job_group")}},
		{Field: "job_category", Label: "Job Category", Widget: "text", Placeholder: "Job category...", Clauses: []filterClause{legacyClause(like("job_category")), like("job_category")}},
		{Field: "started", Label: "Started After", Widget: "datetime-local", Placeholder: "Start date...", Clauses: []filterClause{legacyClause(gte("started"))}},
		{Field: "ended", Label: "Ended After", Widget: "datetime-local", Placeholder: "End date...", Clauses: []filterClause{legacyClause(gte("ended")), nullPair("ended")}},
		// SQL-only (different query param than "started"; no live UI).
		{Field: "started_date", Clauses: []filterClause{dateEq("started")}},
	},
	// Book tables
	"dbbooks": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "isbn", Label: "ISBN", Widget: "text", Placeholder: "ISBN-10 or ISBN-13...", Clauses: []filterClause{rawClause("(isbn_13 LIKE ? OR isbn_10 LIKE ?)", func(v string) []any { return []any{"%" + v + "%", "%" + v + "%"} })}},
		{Field: "author", Label: "Author", Widget: "text", Placeholder: "Author name...", Clauses: []filterClause{like("dbauthors.name")}},
		{Field: "publisher", Label: "Publisher", Widget: "text", Placeholder: "Publisher...", Clauses: []filterClause{legacyClause(like("publisher")), like("publisher")}},
		{Field: "language", Label: "Language", Widget: "text", Placeholder: "Language...", Clauses: []filterClause{legacyClause(eqCol("language"))}},
		{Field: "year", Label: "Year", Widget: "number", Placeholder: "Year...", Clauses: []filterClause{legacyClause(eqCol("year"))}},
		// SQL-only.
		{Field: "genres", Clauses: []filterClause{legacyClause(like("genres"))}},
		{Field: "goodreads_id", Clauses: []filterClause{legacyClause(like("goodreads_id"))}},
		{Field: "openlibrary_id", Clauses: []filterClause{legacyClause(like("openlibrary_id"))}},
		{Field: "page_count", Clauses: []filterClause{legacyClause(gte("page_count"))}},
		{Field: "series_position", Clauses: []filterClause{legacyClause(like("series_position"))}},
	},
	"dbauthors": {
		{Field: "name", Label: "Name", Widget: "text", Placeholder: "Author name...", Clauses: []filterClause{legacyClause(like("name")), like("name")}},
		{Field: "goodreads_id", Label: "Goodreads ID", Widget: "text", Placeholder: "Goodreads ID...", Clauses: []filterClause{legacyClause(like("goodreads_id"))}},
		{Field: "openlibrary_id", Clauses: []filterClause{legacyClause(like("openlibrary_id"))}},
	},
	"dbbook_titles": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("dbbook_titles.title")), like("dbbook_titles.title")}},
		{Field: "book_title", Label: "Book Title", Widget: "text", Placeholder: "Book title...", Clauses: []filterClause{legacyClause(like("dbbooks.title")), like("dbbooks.title")}},
		{Field: "region", Label: "Region", Widget: "text", Placeholder: "Region...", Clauses: []filterClause{legacyClause(like("dbbook_titles.region")), like("dbbook_titles.region")}},
	},
	"dbbook_series": {
		{Field: "name", Label: "Series Name", Widget: "text", Placeholder: "Series name...", Clauses: []filterClause{legacyClause(like("name")), like("name")}},
		{Field: "goodreads_id", Label: "Goodreads ID", Widget: "text", Placeholder: "Goodreads ID...", Clauses: []filterClause{legacyClause(like("goodreads_id"))}},
		{Field: "openlibrary_id", Clauses: []filterClause{legacyClause(like("openlibrary_id"))}},
	},
	"books": {
		{Field: "title", Label: "Book Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("dbbooks.title")}},
		{Field: "author", Label: "Author Name", Widget: "text", Placeholder: "Author name...", Clauses: []filterClause{like("dbauthors.name")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("books.listname")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		{Field: "missing", Label: "Missing", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("missing")), eqCol("books.missing")}},
		{Field: "quality_reached", Label: "Quality Reached", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("quality_reached")), eqCol("books.quality_reached")}},
		// SQL-only.
		{Field: "rootpath", Clauses: []filterClause{legacyClause(like("rootpath"))}},
		{Field: "dont_upgrade", Clauses: []filterClause{legacyClause(eqCol("dont_upgrade"))}},
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
		{Field: "blacklisted", Clauses: []filterClause{legacyClause(eqCol("blacklisted"))}},
		{Field: "author_id", Clauses: []filterClause{legacyClause(eqCol("books.author_id")), eqCol("books.author_id")}},
		{Field: "dbauthor_id", Clauses: []filterClause{rawClause("books.author_id IN (SELECT id FROM authors WHERE dbauthor_id = ?)", func(v string) []any { return []any{v} })}},
	},
	"book_files": {
		{Field: "book_id", Label: "Book ID", Widget: "text", Placeholder: "Book ID...", Clauses: []filterClause{eqCol("book_files.book_id")}},
		{Field: "title", Label: "Book Title", Widget: "text", Placeholder: "Book title...", Clauses: []filterClause{like("dbbooks.title")}},
		{Field: "filename", Label: "Filename", Widget: "text", Placeholder: "Filename...", Clauses: []filterClause{legacyClause(like("filename")), like("book_files.filename")}},
		{Field: "location", Label: "Location", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("location"))}},
		// SQL-only.
		{Field: "extension", Clauses: []filterClause{legacyClause(eqCol("extension"))}},
		{Field: "format", Clauses: []filterClause{legacyClause(eqCol("format"))}},
		{Field: "quality_profile", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
	},
	"authors": {
		{Field: "name", Label: "Author Name", Widget: "text", Placeholder: "Author name...", Clauses: []filterClause{like("dbauthors.name")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("authors.listname")}},
		// SQL-only.
		{Field: "track_mode", Clauses: []filterClause{legacyClause(eqCol("track_mode"))}},
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
	},
	"book_series": {
		{Field: "name", Label: "Series Name", Widget: "text", Placeholder: "Series name...", Clauses: []filterClause{like("dbbook_series.name")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("book_series.listname")}},
		// SQL-only.
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
	},
	"book_file_unmatcheds": {
		{Field: "filepath", Label: "File Path", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("filepath")), like("filepath")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), like("listname")}},
	},
	"book_histories": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Release title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "indexer", Label: "Indexer", Widget: "text", Placeholder: "Indexer...", Clauses: []filterClause{legacyClause(like("indexer")), like("indexer")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		// SQL-only.
		{Field: "downloaded_date", Clauses: []filterClause{dateEq("downloaded_at")}},
	},
	// Audiobook tables
	"dbaudiobooks": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "asin", Label: "ASIN", Widget: "text", Placeholder: "Amazon ASIN...", Clauses: []filterClause{legacyClause(like("asin")), like("asin")}},
		{Field: "narrator", Label: "Narrator", Widget: "text", Placeholder: "Narrator name...", Clauses: []filterClause{like("dbnarrators.name")}},
		{Field: "publisher", Label: "Publisher", Widget: "text", Placeholder: "Publisher...", Clauses: []filterClause{legacyClause(like("publisher")), like("publisher")}},
		{Field: "language", Label: "Language", Widget: "text", Placeholder: "Language...", Clauses: []filterClause{legacyClause(eqCol("language"))}},
		{Field: "year", Label: "Year", Widget: "number", Placeholder: "Year...", Clauses: []filterClause{legacyClause(eqCol("year"))}},
		// SQL-only.
		{Field: "audible_id", Clauses: []filterClause{legacyClause(like("audible_id"))}},
		{Field: "runtime_minutes", Clauses: []filterClause{legacyClause(gte("runtime_minutes"))}},
		{Field: "abridged", Clauses: []filterClause{legacyClause(eqCol("abridged"))}},
	},
	"dbnarrators": {
		{Field: "name", Label: "Name", Widget: "text", Placeholder: "Narrator name...", Clauses: []filterClause{legacyClause(like("name")), like("name")}},
		{Field: "audible_id", Label: "Audible ID", Widget: "text", Placeholder: "Audible ID...", Clauses: []filterClause{legacyClause(like("audible_id"))}},
	},
	"dbaudiobook_titles": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("dbaudiobook_titles.title")), like("dbaudiobook_titles.title")}},
		{Field: "audiobook_title", Label: "Audiobook Title", Widget: "text", Placeholder: "Audiobook title...", Clauses: []filterClause{legacyClause(like("dbaudiobooks.title")), like("dbaudiobooks.title")}},
		{Field: "region", Label: "Region", Widget: "text", Placeholder: "Region...", Clauses: []filterClause{legacyClause(like("dbaudiobook_titles.region")), like("dbaudiobook_titles.region")}},
	},
	"audiobooks": {
		{Field: "title", Label: "Audiobook Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("dbaudiobooks.title")}},
		{Field: "narrator", Label: "Narrator", Widget: "text", Placeholder: "Narrator name...", Clauses: []filterClause{like("dbnarrators.name")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("audiobooks.listname")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		{Field: "missing", Label: "Missing", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("missing")), eqCol("audiobooks.missing")}},
		{Field: "quality_reached", Label: "Quality Reached", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("quality_reached")), eqCol("audiobooks.quality_reached")}},
		// SQL-only.
		{Field: "rootpath", Clauses: []filterClause{legacyClause(like("rootpath"))}},
		{Field: "dont_upgrade", Clauses: []filterClause{legacyClause(eqCol("dont_upgrade"))}},
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
		{Field: "blacklisted", Clauses: []filterClause{legacyClause(eqCol("blacklisted"))}},
		{Field: "author_id", Clauses: []filterClause{legacyClause(eqCol("audiobooks.author_id")), eqCol("audiobooks.author_id")}},
	},
	"audiobook_files": {
		{Field: "audiobook_id", Label: "Audiobook ID", Widget: "text", Placeholder: "Audiobook ID...", Clauses: []filterClause{eqCol("audiobook_files.audiobook_id")}},
		{Field: "title", Label: "Audiobook Title", Widget: "text", Placeholder: "Audiobook title...", Clauses: []filterClause{like("dbaudiobooks.title")}},
		{Field: "filename", Label: "Filename", Widget: "text", Placeholder: "Filename...", Clauses: []filterClause{legacyClause(like("filename")), like("audiobook_files.filename")}},
		{Field: "location", Label: "Location", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("location"))}},
		// SQL-only.
		{Field: "extension", Clauses: []filterClause{legacyClause(eqCol("extension"))}},
		{Field: "format", Clauses: []filterClause{legacyClause(eqCol("format"))}},
		{Field: "quality_profile", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		{Field: "bitrate", Clauses: []filterClause{legacyClause(gte("bitrate"))}},
	},
	"audiobook_file_unmatcheds": {
		{Field: "filepath", Label: "File Path", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("filepath")), like("filepath")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), like("listname")}},
	},
	"audiobook_histories": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Release title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "indexer", Label: "Indexer", Widget: "text", Placeholder: "Indexer...", Clauses: []filterClause{legacyClause(like("indexer")), like("indexer")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		// SQL-only.
		{Field: "downloaded_date", Clauses: []filterClause{dateEq("downloaded_at")}},
	},
	// Music tables
	"dbalbums": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{
			Field: "artist", Label: "Artist", Widget: "text", Placeholder: "Artist name...",
			Clauses: []filterClause{rawClause(
				"id IN (SELECT dbalbum_artists.dbalbum_id FROM dbalbum_artists JOIN dbartists ON dbalbum_artists.dbartist_id = dbartists.id WHERE dbartists.name LIKE ?)",
				func(v string) []any { return []any{"%" + v + "%"} },
			)},
		},
		{Field: "label", Label: "Label", Widget: "text", Placeholder: "Record label...", Clauses: []filterClause{legacyClause(like("label")), like("label")}},
		{Field: "year", Label: "Year", Widget: "number", Placeholder: "Year...", Clauses: []filterClause{legacyClause(eqCol("year"))}},
		{Field: "release_type", Label: "Release Type", Widget: "text", Placeholder: "album, ep, single...", Clauses: []filterClause{legacyClause(eqCol("release_type"))}},
		{Field: "format", Label: "Format", Widget: "text", Placeholder: "cd, vinyl, digital...", Clauses: []filterClause{legacyClause(eqCol("format"))}},
		{Field: "country", Label: "Country", Widget: "text", Placeholder: "Country...", Clauses: []filterClause{legacyClause(eqCol("country"))}},
		{Field: "musicbrainz_id", Label: "MusicBrainz ID", Widget: "text", Placeholder: "MusicBrainz release ID...", Clauses: []filterClause{legacyClause(like("musicbrainz_release_id"))}},
		{Field: "discogs_id", Label: "Discogs ID", Widget: "text", Placeholder: "Discogs release ID...", Clauses: []filterClause{legacyClause(like("discogs_release_id"))}},
	},
	"dbartists": {
		{Field: "name", Label: "Name", Widget: "text", Placeholder: "Artist name...", Clauses: []filterClause{legacyClause(like("name")), like("name")}},
		{Field: "country", Label: "Country", Widget: "text", Placeholder: "Country...", Clauses: []filterClause{legacyClause(eqCol("country"))}},
		{Field: "artist_type", Label: "Artist Type", Widget: "text", Placeholder: "Person, Group...", Clauses: []filterClause{legacyClause(eqCol("artist_type"))}},
		{Field: "musicbrainz_id", Label: "MusicBrainz ID", Widget: "text", Placeholder: "MusicBrainz artist ID...", Clauses: []filterClause{legacyClause(like("musicbrainz_id"))}},
		{Field: "discogs_id", Label: "Discogs ID", Widget: "text", Placeholder: "Discogs artist ID...", Clauses: []filterClause{legacyClause(like("discogs_id"))}},
	},
	"dbalbum_titles": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{legacyClause(like("dbalbum_titles.title")), like("dbalbum_titles.title")}},
		{Field: "album_title", Label: "Album Title", Widget: "text", Placeholder: "Album title...", Clauses: []filterClause{legacyClause(like("dbalbums.title")), like("dbalbums.title")}},
		{Field: "region", Label: "Region", Widget: "text", Placeholder: "Region...", Clauses: []filterClause{legacyClause(like("dbalbum_titles.region")), like("dbalbum_titles.region")}},
	},
	"dbtracks": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Track title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "album_title", Label: "Album Title", Widget: "text", Placeholder: "Album title...", Clauses: []filterClause{like("dbalbums.title")}},
		{Field: "track_number", Label: "Track Number", Widget: "number", Placeholder: "Track #...", Clauses: []filterClause{legacyClause(eqCol("track_number")), eqCol("track_number")}},
		// SQL-only.
		{Field: "disc_number", Clauses: []filterClause{legacyClause(eqCol("disc_number"))}},
		{Field: "explicit", Clauses: []filterClause{legacyClause(eqCol("explicit"))}},
	},
	"artists": {
		{Field: "name", Label: "Artist Name", Widget: "text", Placeholder: "Artist name...", Clauses: []filterClause{like("dbartists.name")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("artists.listname")}},
		// SQL-only.
		{Field: "track_mode", Clauses: []filterClause{legacyClause(eqCol("track_mode"))}},
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
	},
	"albums": {
		{Field: "title", Label: "Album Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("dbalbums.title")}},
		{
			Field: "artist", Label: "Artist", Widget: "text", Placeholder: "Artist name...",
			Clauses: []filterClause{rawClause(
				"albums.artist_id IN (SELECT artists.id FROM artists JOIN dbartists ON artists.dbartist_id = dbartists.id WHERE dbartists.name LIKE ?)",
				func(v string) []any { return []any{"%" + v + "%"} },
			)},
		},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), eqCol("albums.listname")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		{Field: "missing", Label: "Missing", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("missing")), eqCol("albums.missing")}},
		{Field: "quality_reached", Label: "Quality Reached", Widget: "select", Options: yesNoOptions, OptionLabels: yesNoLabels, Clauses: []filterClause{legacyClause(eqCol("quality_reached")), eqCol("albums.quality_reached")}},
		// SQL-only.
		{Field: "rootpath", Clauses: []filterClause{legacyClause(like("rootpath"))}},
		{Field: "dont_upgrade", Clauses: []filterClause{legacyClause(eqCol("dont_upgrade"))}},
		{Field: "dont_search", Clauses: []filterClause{legacyClause(eqCol("dont_search"))}},
		{Field: "blacklisted", Clauses: []filterClause{legacyClause(eqCol("blacklisted"))}},
		{Field: "artist_id", Clauses: []filterClause{legacyClause(eqCol("albums.artist_id")), eqCol("albums.artist_id")}},
		{Field: "dbartist_id", Clauses: []filterClause{rawClause("albums.artist_id IN (SELECT id FROM artists WHERE dbartist_id = ?)", func(v string) []any { return []any{v} })}},
	},
	"album_files": {
		{Field: "album_id", Label: "Album ID", Widget: "text", Placeholder: "Album ID...", Clauses: []filterClause{eqCol("album_files.album_id")}},
		{Field: "title", Label: "Album Title", Widget: "text", Placeholder: "Album title...", Clauses: []filterClause{like("dbalbums.title")}},
		{Field: "filename", Label: "Filename", Widget: "text", Placeholder: "Filename...", Clauses: []filterClause{legacyClause(like("filename")), like("album_files.filename")}},
		{Field: "location", Label: "Location", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("location"))}},
		// SQL-only.
		{Field: "extension", Clauses: []filterClause{legacyClause(eqCol("extension"))}},
		{Field: "format", Clauses: []filterClause{legacyClause(eqCol("format"))}},
		{Field: "quality_profile", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		{Field: "bitrate", Clauses: []filterClause{legacyClause(gte("bitrate"))}},
		{Field: "sample_rate", Clauses: []filterClause{legacyClause(gte("sample_rate"))}},
	},
	"album_file_unmatcheds": {
		{Field: "filepath", Label: "File Path", Widget: "text", Placeholder: "File path...", Clauses: []filterClause{legacyClause(like("filepath")), like("filepath")}},
		{Field: "listname", Label: "List Name", Widget: "text", Placeholder: "List name...", Clauses: []filterClause{legacyClause(like("listname")), like("listname")}},
	},
	"album_histories": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Release title...", Clauses: []filterClause{legacyClause(like("title")), like("title")}},
		{Field: "indexer", Label: "Indexer", Widget: "text", Placeholder: "Indexer...", Clauses: []filterClause{legacyClause(like("indexer")), like("indexer")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{legacyClause(like("quality_profile"))}},
		// SQL-only.
		{Field: "downloaded_date", Clauses: []filterClause{dateEq("downloaded_at")}},
	},

	// The following tables had no entry in the old getFilterableFieldsForTable
	// map at all — their UI came only from renderCustomFilters's fallback
	// switch (which was reachable for them, unlike for the tables above), and
	// their SQL came only from buildCustomFilters's legacy switch. There is
	// no old generic-map/legacy-switch duplication to preserve here, so each
	// field has exactly one clause.
	"dbserie_alternates": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("dbserie_alternates.title")}},
		{Field: "series_name", Label: "Series Name", Widget: "text", Placeholder: "Filter by series name...", Clauses: []filterClause{like("dbseries.seriename")}},
		{Field: "region", Label: "Region", Widget: "text", Placeholder: "Region...", Clauses: []filterClause{like("dbserie_alternates.region")}},
	},
	"dbserie_episodes": {
		{Field: "title", Label: "Episode Title", Widget: "text", Placeholder: "Filter by episode title...", Clauses: []filterClause{like("dbserie_episodes.title")}},
		{Field: "series_name", Label: "Series Name", Widget: "text", Placeholder: "Filter by series name...", Clauses: []filterClause{like("dbseries.seriename")}},
		{Field: "season", Label: "Season", Widget: "number", Placeholder: "Season...", Clauses: []filterClause{eqCol("dbserie_episodes.season")}},
		{Field: "episode", Label: "Episode", Widget: "number", Placeholder: "Episode...", Clauses: []filterClause{eqCol("dbserie_episodes.episode")}},
		{Field: "identifier", Label: "Identifier", Widget: "text", Placeholder: "Identifier...", Clauses: []filterClause{eqCol("dbserie_episodes.identifier")}},
	},
	"serie_episodes": {
		{Field: "tvdb_id", Label: "TVDB ID", Widget: "text", Placeholder: "Filter by TVDB ID...", Clauses: []filterClause{rawClause(
			"serie_episodes.dbserie_id IN (SELECT id FROM dbseries WHERE thetvdb_id LIKE ?)",
			func(v string) []any { return []any{"%" + v + "%"} },
		)}},
		{Field: "episode_title", Label: "Episode Title", Widget: "text", Placeholder: "Filter by episode title...", Clauses: []filterClause{like("dbserie_episodes.title")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "select-quality", Clauses: []filterClause{like("serie_episodes.quality_profile")}},
		{Field: "missing", Label: "Missing", Widget: "select", Options: []string{"", "1", "0"}, OptionLabels: []string{"All", "Missing", "Available"}, Clauses: []filterClause{literal01("serie_episodes.missing")}},
	},
	"movie_histories": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("movie_histories.title")}},
		{Field: "indexer", Label: "Indexer", Widget: "text", Placeholder: "Filter by indexer...", Clauses: []filterClause{like("movie_histories.indexer")}},
		{Field: "quality_profile", Label: "Quality Profile", Widget: "select-quality", Clauses: []filterClause{like("movie_histories.quality_profile")}},
		{Field: "downloaded_date", Label: "Downloaded Date", Widget: "date", Placeholder: "Downloaded date...", Clauses: []filterClause{dateEq("movie_histories.downloaded_at")}},
	},
	"movie_file_unmatcheds": {
		{Field: "filepath", Label: "Filepath", Widget: "text", Placeholder: "Filter by filepath...", Clauses: []filterClause{like("movie_file_unmatcheds.filepath")}},
		{Field: "listname", Label: "Listname", Widget: "text", Placeholder: "Filter by listname...", Clauses: []filterClause{like("movie_file_unmatcheds.listname")}},
		{Field: "movie_quality_profile", Label: "Quality Profile", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{like("movies.quality_profile")}},
	},
	"serie_file_unmatcheds": {
		{Field: "filepath", Label: "Filepath", Widget: "text", Placeholder: "Filter by filepath...", Clauses: []filterClause{like("serie_file_unmatcheds.filepath")}},
		{Field: "listname", Label: "Listname", Widget: "text", Placeholder: "Filter by listname...", Clauses: []filterClause{like("serie_file_unmatcheds.listname")}},
		{Field: "series_rootpath", Label: "Root Path", Widget: "text", Placeholder: "Root path...", Clauses: []filterClause{like("series.rootpath")}},
	},
	"serie_episode_histories": {
		{Field: "title", Label: "Title", Widget: "text", Placeholder: "Filter by title...", Clauses: []filterClause{like("serie_episode_histories.title")}},
		{Field: "episode_title", Label: "Episode Title", Widget: "text", Placeholder: "Filter by episode title...", Clauses: []filterClause{like("dbserie_episodes.title")}},
		{Field: "indexer", Label: "Indexer", Widget: "text", Placeholder: "Indexer...", Clauses: []filterClause{like("serie_episode_histories.indexer")}},
		{Field: "quality_profile", Label: "Quality", Widget: "text", Placeholder: "Quality...", Clauses: []filterClause{like("serie_episode_histories.quality_profile")}},
	},
}

// renderStaticSelect renders a <select> with a fixed set of options.
func renderStaticSelect(label, field string, options, optionLabels []string) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(options))

	for i, val := range options {
		lbl := val
		if i < len(optionLabels) {
			lbl = optionLabels[i]
		}

		opts = append(opts, createOption(val, lbl, false))
	}

	return html.Div(
		html.Label(html.Class("form-label"), gomponents.Text(label)),
		html.Select(
			append([]gomponents.Node{
				html.Class("form-control custom-filter"),
				html.ID("filter-" + field),
			}, opts...)...),
	)
}

// renderQualitySelect renders a <select> populated from the currently
// configured quality profiles.
func renderQualitySelect(label, field string) gomponents.Node {
	qualityConfigs := config.GetSettingsQualityAll()
	opts := make([]gomponents.Node, 0, 3+len(qualityConfigs))

	opts = append(opts, html.Class("form-control custom-filter"))
	opts = append(opts, html.ID("filter-"+field))
	opts = append(opts, createOption("", "All Profiles", false))

	for _, qc := range qualityConfigs {
		opts = append(opts, createOption(qc.Name, qc.Name, false))
	}

	return html.Div(
		html.Label(html.Class("form-label"), gomponents.Text(label)),
		html.Select(opts...),
	)
}

// renderInputField renders a plain <input> filter field (text/number/date/datetime-local).
func renderInputField(label, field, widget, placeholder string) gomponents.Node {
	typ := widget
	if typ == "" {
		typ = "text"
	}

	return html.Div(
		html.Label(html.Class("form-label"), gomponents.Text(label)),
		html.Input(html.Class("form-control custom-filter"), html.Type(typ),
			html.ID("filter-"+field), html.Placeholder(placeholder)),
	)
}

// renderFilterField renders one field's filter UI from its unified definition.
func renderFilterField(def tableFilterField) gomponents.Node {
	switch def.Widget {
	case "select":
		return renderStaticSelect(def.Label, def.Field, def.Options, def.OptionLabels)
	case "select-quality":
		return renderQualitySelect(def.Label, def.Field)
	default:
		return renderInputField(def.Label, def.Field, def.Widget, def.Placeholder)
	}
}

// renderFilterInputs builds the filter <input>/<select> nodes for a table
// from filterFieldDefs, skipping SQL-only entries (Label == "").
func renderFilterInputs(tableName string) []gomponents.Node {
	var nodes []gomponents.Node

	for _, def := range filterFieldDefs[tableName] {
		if def.Label == "" {
			continue
		}

		nodes = append(nodes, renderFilterField(def))
	}

	return nodes
}

// applyDefinedFilterClauses evaluates every field in filterFieldDefs[tableName]
// against ctx and returns the SQL fragments (and their bind args) for every
// clause that fired. It also returns legacyHandled: the set of fields that
// had at least one clause originating in the old filterMappings map, which
// buildCustomFilters's generic "_id" catch-all uses to reproduce the old
// code's (narrower) exclusion set exactly.
//
// Split out from buildCustomFilters so it can be exercised directly in tests
// without the DB dependency the "_id" catch-all has via getColumnTypes.
func applyDefinedFilterClauses(
	tableName string,
	ctx *gin.Context,
) (conditions []string, args []any, legacyHandled map[string]struct{}) {
	defs := filterFieldDefs[tableName]

	legacyHandled = make(map[string]struct{}, len(defs))

	for _, def := range defs {
		for _, cl := range def.Clauses {
			if cl.Legacy {
				legacyHandled[def.Field] = struct{}{}
				break
			}
		}
	}

	for _, def := range defs {
		value := getParamValue(ctx, "filter-"+def.Field)
		if value == "" {
			continue
		}

		for _, cl := range def.Clauses {
			cond, clauseArgs, ok := cl.apply(value)
			if !ok {
				continue
			}

			conditions = append(conditions, cond)
			args = append(args, clauseArgs...)
		}
	}

	return conditions, args, legacyHandled
}

// buildCustomFilters builds the SQL WHERE fragment + bind args for a table's
// submitted custom filters from filterFieldDefs. It also preserves the
// universal exact-match "id" filter and the dynamic generic "_id" catch-all
// that this function already applied on top of the old per-table lists —
// neither of those is part of the hand-maintained-list drift problem this
// refactor addresses, so both are kept exactly as they were.
func buildCustomFilters(tableName string, ctx *gin.Context) (string, []any) {
	var (
		conditions []string
		args       []any
	)

	// Universal id filter (used by ID links, e.g. ?id=71718). Every
	// GetTableDefaults(...).Table string's leading/base table is literally
	// tableName itself, so qualifying with tableName is always correct and
	// unambiguous, including for joined tables.
	if idValue := getParamValue(ctx, "filter-id"); idValue != "" {
		conditions = append(conditions, tableName+".id = ?")
		args = append(args, idValue)
	}

	definedConditions, definedArgs, legacyHandled := applyDefinedFilterClauses(tableName, ctx)
	conditions = append(conditions, definedConditions...)
	args = append(args, definedArgs...)

	// Generic _id filter: any filter-*_id query param whose column exists in
	// the base table and is not already handled above is applied as an
	// exact-match condition. Column names are validated against
	// pragma_table_info so no user-supplied string is ever interpolated
	// directly into SQL.
	{
		validCols := getColumnTypes(tableName)

		for col := range validCols {
			if !strings.HasSuffix(col, "_id") {
				continue
			}

			if _, handled := legacyHandled[col]; handled {
				continue
			}

			// Skip external IDs that are not FK row references.
			if getReferenceTable(col) == "" {
				continue
			}

			val := getParamValue(ctx, "filter-"+col)
			if val == "" {
				continue
			}

			conditions = append(conditions, col+" = ?")
			args = append(args, val)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), args
}
