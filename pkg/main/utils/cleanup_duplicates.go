package utils

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// dupTableSpec describes one per-config item table and the child rows that hang
// off it. Foreign-key cascade is disabled in this database (optional reference
// columns store 0 instead of NULL), so child rows must be deleted explicitly
// before the parent row — otherwise they linger as orphans until CleanupOrphans.
type dupTableSpec struct {
	label      string // log label: "movie" / "series" / ...
	table      string // parent table:  movies / series / books / audiobooks / albums
	idCol      string // db-item id column: dbmovie_id / dbserie_id / ...
	fileTable  string // child files table (used to detect a "real" row and to delete)
	fileFK     string // FK column in fileTable -> parent id
	histTable  string // child histories table
	histFK     string // FK column in histTable -> parent id
	childTable string // optional extra child rows (serie_episodes); "" if none
	childFK    string // FK column in childTable -> parent id
	// refs are tables that reference this row's id (e.g. audiobooks.author_id ->
	// authors.id). Before a duplicate row is deleted, these references are
	// repointed to the kept row so nothing is orphaned. Each entry is {table, column}.
	refs [][2]string
}

// specForType returns the table layout for a media group type. The second
// return is false for unknown types (nothing to clean).
func specForType(isType uint) (dupTableSpec, bool) {
	switch isType {
	case config.MediaTypeMovie:
		return dupTableSpec{
			label: "movie", table: "movies", idCol: "dbmovie_id",
			fileTable: "movie_files", fileFK: "movie_id",
			histTable: "movie_histories", histFK: "movie_id",
		}, true
	case config.MediaTypeSeries:
		return dupTableSpec{
			label: "series", table: "series", idCol: "dbserie_id",
			fileTable: "serie_episode_files", fileFK: "serie_id",
			histTable: "serie_episode_histories", histFK: "serie_id",
			childTable: "serie_episodes", childFK: "serie_id",
		}, true
	case config.MediaTypeBook:
		return dupTableSpec{
			label: "book", table: "books", idCol: "dbbook_id",
			fileTable: "book_files", fileFK: "book_id",
			histTable: "book_histories", histFK: "book_id",
		}, true
	case config.MediaTypeAudiobook:
		return dupTableSpec{
			label: "audiobook", table: "audiobooks", idCol: "dbaudiobook_id",
			fileTable: "audiobook_files", fileFK: "audiobook_id",
			histTable: "audiobook_histories", histFK: "audiobook_id",
		}, true
	case config.MediaTypeMusic:
		return dupTableSpec{
			label: "album", table: "albums", idCol: "dbalbum_id",
			fileTable: "album_files", fileFK: "album_id",
			histTable: "album_histories", histFK: "album_id",
		}, true
	}

	return dupTableSpec{}, false
}

// trackerSpecForType returns the author/artist tracker table that also lives
// per-config for a media group type, or false when the type has none. These
// rows carry no files but are referenced by items (audiobooks/books/albums), so
// their refs are repointed to the keeper before a duplicate is removed.
func trackerSpecForType(isType uint) (dupTableSpec, bool) {
	switch isType {
	case config.MediaTypeBook, config.MediaTypeAudiobook:
		return dupTableSpec{
			label: "author", table: "authors", idCol: "dbauthor_id",
			refs: [][2]string{
				{"books", "author_id"},
				{"audiobooks", "author_id"},
				{"book_series", "author_id"},
			},
		}, true
	case config.MediaTypeMusic:
		return dupTableSpec{
			label: "artist", table: "artists", idCol: "dbartist_id",
			refs: [][2]string{
				{"albums", "artist_id"},
			},
		}, true
	}

	return dupTableSpec{}, false
}

// CleanupListDuplicates removes rows that exist more than once for the same
// media item within a single media group config — the same db-item (dbmovie /
// dbserie / dbbook / dbaudiobook / dbalbum) appearing under two sibling lists of
// the same group. Exactly one row is kept per (config, item): a row that has
// downloaded files is preferred, otherwise the oldest (lowest id); all other
// rows (and their files/histories) are deleted. It also collapses exact
// duplicate serie_episodes (same serie_id + dbserie_episode_id).
//
// When apply is false nothing is written — every group that would be changed is
// logged so the result can be reviewed first. Cross-group duplicates (the same
// item in config "DE" and config "EN") are intentionally left untouched.
func CleanupListDuplicates(apply bool) {
	var totalGroups, totalDeleted int

	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		// Need at least two sibling lists for an intra-config duplicate.
		if cfgp.ListsLen < 2 {
			return nil
		}

		if spec, ok := specForType(cfgp.IsType); ok {
			g, d := dedupTableForConfig(spec, cfgp, apply)
			totalGroups += g
			totalDeleted += d
		}

		// Author/artist tracker rows also live per-config for book/audiobook/music.
		if spec, ok := trackerSpecForType(cfgp.IsType); ok {
			g, d := dedupTableForConfig(spec, cfgp, apply)
			totalGroups += g
			totalDeleted += d
		}

		return nil
	})

	eg, ed := cleanupDuplicateEpisodes(apply)
	totalGroups += eg
	totalDeleted += ed

	mode := "DRY-RUN (no changes written)"
	if apply {
		mode = "APPLIED"
	}

	logger.Logtype("info", 0).
		Int("duplicate_groups", totalGroups).
		Int("rows_deleted", totalDeleted).
		Str("mode", mode).
		Msg("List duplicate cleanup finished")
}

// dedupTableForConfig finds and collapses duplicate rows of one table across a
// single media group's sibling lists. Returns the number of duplicate groups
// found and the number of rows deleted.
func dedupTableForConfig(spec dupTableSpec, cfgp *config.MediaTypeConfig, apply bool) (int, int) {
	listargs := make([]any, 0, cfgp.ListsLen)
	for i := range cfgp.ListsNames {
		listargs = append(listargs, &cfgp.ListsNames[i])
	}

	inClause := "listname in (?" + cfgp.ListsQu + ")"

	// Distinct db-item ids that have more than one row across this group's lists.
	dupIDs := database.Getrowssize[uint](false,
		"select count() from (select "+spec.idCol+" from "+spec.table+
			" where "+inClause+" group by "+spec.idCol+" having count(*) > 1)",
		"select "+spec.idCol+" from "+spec.table+
			" where "+inClause+" group by "+spec.idCol+" having count(*) > 1",
		listargs...)

	var groups, deleted int

	for i := range dupIDs {
		if g, d := dedupeGroup(spec, cfgp.Name, inClause, dupIDs[i], listargs, apply); g {
			groups++
			deleted += d
		}
	}

	return groups, deleted
}

// dedupeGroup keeps one row for a single (config, db-item) and removes the rest.
// Returns whether a duplicate group was found and how many rows were deleted.
func dedupeGroup(
	spec dupTableSpec,
	cfgName, inClause string,
	dbid uint,
	listargs []any,
	apply bool,
) (bool, int) {
	args := make([]any, 0, len(listargs)+1)
	args = append(args, &dbid)
	args = append(args, listargs...)

	ids := database.Getrowssize[uint](false,
		"select count() from "+spec.table+" where "+spec.idCol+" = ? and "+inClause,
		"select id from "+spec.table+" where "+spec.idCol+" = ? and "+inClause+" order by id",
		args...)
	if len(ids) < 2 {
		return false, 0
	}

	keeper := pickKeeper(spec, ids)

	var deleted int

	for _, id := range ids {
		if id == keeper {
			continue
		}

		if apply {
			// Repoint anything referencing the doomed row to the keeper first,
			// then remove the row and its children.
			for _, ref := range spec.refs {
				database.ExecN(
					"update "+ref[0]+" set "+ref[1]+" = ? where "+ref[1]+" = ?",
					&keeper, &id,
				)
			}

			deleteParentRow(spec, id)
		}

		deleted++
	}

	logger.Logtype("info", 0).
		Str("type", spec.label).
		Str("config", cfgName).
		Uint(spec.idCol, dbid).
		Uint("kept_id", keeper).
		Int("deleted", deleted).
		Bool("applied", apply).
		Msg("Duplicate list entry group")

	return true, deleted
}

// pickKeeper chooses the row to retain: the first (lowest id) row that has
// downloaded files, otherwise the lowest id. ids must be sorted ascending.
func pickKeeper(spec dupTableSpec, ids []uint) uint {
	if spec.fileTable != "" {
		for _, id := range ids {
			if database.Getdatarow[uint](false,
				"select count() from "+spec.fileTable+" where "+spec.fileFK+" = ?", &id) > 0 {
				return id
			}
		}
	}

	return ids[0]
}

// deleteParentRow removes a parent row and all of its child rows (files,
// histories and, for series, serie_episodes). FK cascade is disabled, so each
// is deleted explicitly.
func deleteParentRow(spec dupTableSpec, id uint) {
	if spec.fileTable != "" {
		database.ExecN("delete from "+spec.fileTable+" where "+spec.fileFK+" = ?", &id)
	}

	if spec.histTable != "" {
		database.ExecN("delete from "+spec.histTable+" where "+spec.histFK+" = ?", &id)
	}

	if spec.childTable != "" {
		database.ExecN("delete from "+spec.childTable+" where "+spec.childFK+" = ?", &id)
	}

	database.ExecN("delete from "+spec.table+" where id = ?", &id)
}

// cleanupDuplicateEpisodes collapses exact duplicate episode rows: the same
// dbserie_episode_id linked to the same serie_id more than once. The row with
// files is kept (else the lowest id); the rest and their files/histories go.
func cleanupDuplicateEpisodes(apply bool) (int, int) {
	pairs := database.Getrowssize[database.DbstaticTwoUint](false,
		"select count() from (select serie_id, dbserie_episode_id from serie_episodes"+
			" group by serie_id, dbserie_episode_id having count(*) > 1)",
		"select serie_id, dbserie_episode_id from serie_episodes"+
			" group by serie_id, dbserie_episode_id having count(*) > 1")

	var groups, deleted int

	for i := range pairs {
		ids := database.Getrowssize[uint](false,
			"select count() from serie_episodes where serie_id = ? and dbserie_episode_id = ?",
			"select id from serie_episodes where serie_id = ? and dbserie_episode_id = ? order by id",
			&pairs[i].Num1, &pairs[i].Num2)
		if len(ids) < 2 {
			continue
		}

		keeper := ids[0]

		for _, id := range ids {
			if database.Getdatarow[uint](false,
				"select count() from serie_episode_files where serie_episode_id = ?", &id) > 0 {
				keeper = id
				break
			}
		}

		for _, id := range ids {
			if id == keeper {
				continue
			}

			if apply {
				database.ExecN("delete from serie_episode_files where serie_episode_id = ?", &id)
				database.ExecN("delete from serie_episode_histories where serie_episode_id = ?", &id)
				database.ExecN("delete from serie_episodes where id = ?", &id)
			}

			deleted++
		}

		groups++
	}

	if groups > 0 {
		logger.Logtype("info", 0).
			Int("episode_groups", groups).
			Int("episodes_deleted", deleted).
			Bool("applied", apply).
			Msg("Duplicate serie_episodes cleanup")
	}

	return groups, deleted
}
