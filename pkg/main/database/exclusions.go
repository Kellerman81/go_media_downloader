package database

import "strings"

// ImportExclusion is a title the list imports of a media group don't add
// again (import_exclusions).
type ImportExclusion struct {
	ID         uint   `db:"id"`
	Kind       string `db:"kind"` // movie, series, album, audiobook, book
	MediaGroup string `db:"media_group"`
	DBID       uint   `db:"db_id"`
	ExtID      string `db:"ext_id"`
	Title      string `db:"title"`
	CreatedAt  string `db:"created_at"`
}

// exclusionTables describe the metadata table of each kind: the external
// id that identifies a row (an expression over it), its title and year, and
// the list entry table pointing at it.
var exclusionTables = map[string]struct{ table, expr, title, year, entries, entryFK string }{
	"movie": {"dbmovies", "COALESCE(imdb_id, '')", "title", "COALESCE(year, 0)", "movies", "dbmovie_id"},
	"series": {
		"dbseries", "CASE WHEN COALESCE(thetvdb_id, 0) > 0 THEN CAST(thetvdb_id AS TEXT) ELSE COALESCE(imdb_id, '') END",
		"seriename", "CAST(COALESCE(substr(firstaired, 1, 4), '0') AS INTEGER)", "series", "dbserie_id",
	},
	"album": {
		"dbalbums", "COALESCE(NULLIF(musicbrainz_release_group_id, ''), COALESCE(musicbrainz_release_id, ''))",
		"title", "COALESCE(year, 0)", "albums", "dbalbum_id",
	},
	"audiobook": {"dbaudiobooks", "COALESCE(asin, '')", "title", "COALESCE(year, 0)", "audiobooks", "dbaudiobook_id"},
	"book":      {"dbbooks", "COALESCE(NULLIF(isbn_13, ''), COALESCE(openlibrary_id, ''))", "title", "COALESCE(year, 0)", "books", "dbbook_id"},
}

// AddImportExclusion keeps the list imports of mediaGroup from adding the
// metadata row dbID of kind again.
func AddImportExclusion(kind, mediaGroup string, dbID *uint, title string) error {
	ext, ok := exclusionTables[kind]
	if !ok {
		return ErrUserInvalid
	}

	return ExecNErr("INSERT OR IGNORE INTO import_exclusions (kind, media_group, db_id, ext_id, title) "+
		"SELECT ?, ?, id, "+ext.expr+", ? FROM "+ext.table+" WHERE id = ?", kind, mediaGroup, title, dbID)
}

// ImportExcluded reports whether the list imports of mediaGroup must not
// add the metadata row dbID of kind (by the row or its external id).
func ImportExcluded(kind, mediaGroup string, dbID *uint) bool {
	ext, ok := exclusionTables[kind]
	if !ok || dbID == nil || *dbID == 0 {
		return false
	}

	return Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM import_exclusions WHERE kind = ? AND media_group = ? COLLATE NOCASE "+
		"AND (db_id = ? OR (ext_id <> '' AND ext_id = (SELECT "+ext.expr+" FROM "+ext.table+" WHERE id = ?))))",
		kind, mediaGroup, dbID, dbID)
}

// GetImportExclusions returns all exclusions, newest first.
func GetImportExclusions() []ImportExclusion {
	return StructscanT[ImportExclusion](false, 0,
		"SELECT id, kind, media_group, db_id, ext_id, title, created_at FROM import_exclusions ORDER BY id DESC")
}

// DeleteImportExclusion lets the list imports add the title again.
func DeleteImportExclusion(id *uint) error {
	return ExecNErr("DELETE FROM import_exclusions WHERE id = ?", id)
}

// ExclusionCandidate is a metadata row found to exclude.
type ExclusionCandidate struct {
	DBID     uint   `db:"id"`
	Title    string `db:"title"`
	Year     int    `db:"year"`
	ExtID    string `db:"ext_id"`
	Excluded bool   `db:"excluded"` // already excluded in the media group
	Listed   bool   `db:"listed"`   // in one of lists (the media group's)
}

// SearchExclusionCandidates finds metadata rows of kind by title (part) or
// external id (whole), for excluding them from mediaGroup, whose lists are
// lists. Exact titles come first.
func SearchExclusionCandidates(kind, mediaGroup string, lists []string, search string, limit int) []ExclusionCandidate {
	ext, ok := exclusionTables[kind]
	search = strings.TrimSpace(search)

	if !ok || search == "" {
		return nil
	}

	listed := "0"
	if len(lists) > 0 {
		listed = "EXISTS(SELECT 1 FROM " + ext.entries + " e WHERE e." + ext.entryFK + " = d.id AND e.listname COLLATE NOCASE IN (?" +
			strings.Repeat(",?", len(lists)-1) + "))"
	}

	// The rows are found and cut to limit first; only those are checked
	// against the exclusions and lists (a short search matches thousands).
	args := []any{kind, mediaGroup}
	for _, l := range lists {
		args = append(args, l)
	}

	args = append(args, search, "%"+search+"%", search, limit)

	return StructscanT[ExclusionCandidate](false, uint(max(limit, 0)), //nolint:gosec // limit is not negative
		"SELECT d.id, d.title, d.year, d.ext_id, "+
			"EXISTS(SELECT 1 FROM import_exclusions x WHERE x.kind = ? AND x.media_group = ? COLLATE NOCASE "+
			"AND (x.db_id = d.id OR (d.ext_id <> '' AND x.ext_id = d.ext_id))) AS excluded, "+listed+" AS listed "+
			"FROM (SELECT id, COALESCE("+ext.title+", '') AS title, "+ext.year+" AS year, "+ext.expr+" AS ext_id, "+
			ext.title+" = ? COLLATE NOCASE AS exact FROM "+ext.table+" WHERE "+ext.title+" LIKE ? OR "+ext.expr+" = ? "+
			"ORDER BY exact DESC, title COLLATE NOCASE, id LIMIT ?) d ORDER BY d.exact DESC, d.title COLLATE NOCASE, d.id", args...)
}

// ExclusionTitle is the title of the metadata row dbID of kind; ok is
// false when there is no such row.
func ExclusionTitle(kind string, dbID *uint) (title string, ok bool) {
	ext, known := exclusionTables[kind]
	if !known {
		return "", false
	}

	rows := GetrowsN[string](false, 1, "SELECT COALESCE("+ext.title+", '') FROM "+ext.table+" WHERE id = ?", dbID)
	if len(rows) == 0 {
		return "", false
	}

	return rows[0], true
}
