package database

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

// exclusionExtIDs are the external ids that identify a metadata row of
// each kind, as an expression over that row.
var exclusionExtIDs = map[string]struct{ table, expr string }{
	"movie":     {"dbmovies", "COALESCE(imdb_id, '')"},
	"series":    {"dbseries", "CASE WHEN COALESCE(thetvdb_id, 0) > 0 THEN CAST(thetvdb_id AS TEXT) ELSE COALESCE(imdb_id, '') END"},
	"album":     {"dbalbums", "COALESCE(NULLIF(musicbrainz_release_group_id, ''), COALESCE(musicbrainz_release_id, ''))"},
	"audiobook": {"dbaudiobooks", "COALESCE(asin, '')"},
	"book":      {"dbbooks", "COALESCE(NULLIF(isbn_13, ''), COALESCE(openlibrary_id, ''))"},
}

// AddImportExclusion keeps the list imports of mediaGroup from adding the
// metadata row dbID of kind again.
func AddImportExclusion(kind, mediaGroup string, dbID *uint, title string) error {
	ext, ok := exclusionExtIDs[kind]
	if !ok {
		return ErrUserInvalid
	}

	return ExecNErr("INSERT OR IGNORE INTO import_exclusions (kind, media_group, db_id, ext_id, title) "+
		"SELECT ?, ?, id, "+ext.expr+", ? FROM "+ext.table+" WHERE id = ?", kind, mediaGroup, title, dbID)
}

// ImportExcluded reports whether the list imports of mediaGroup must not
// add the metadata row dbID of kind (by the row or its external id).
func ImportExcluded(kind, mediaGroup string, dbID *uint) bool {
	ext, ok := exclusionExtIDs[kind]
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
