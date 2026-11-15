package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// maxOrphanPasses bounds the repeated cleanup sweeps - deleting an orphaned
// parent (e.g. a movies row) can orphan its own children (movie_files), so
// the sweep repeats until a pass deletes nothing.
const maxOrphanPasses = 5

// CleanupOrphans deletes child rows whose parent rows no longer exist,
// replicating the ON DELETE CASCADE behavior the schema declares. SQLite
// foreign-key enforcement is intentionally disabled for this database
// (optional reference columns store 0 instead of NULL, which enforcement
// would reject), so cascades never fire and orphans accumulate.
//
// The relationships are read from the schema itself via
// pragma_foreign_key_list, so new tables and constraints are picked up
// automatically. A value of 0 or NULL in the referencing column is treated
// as "no parent" and left alone. Returns the number of deleted rows per
// child table (only tables where rows were deleted are included) and the
// joined errors of every statement that failed.
//
// With dryRun the orphans are only counted, in a single pass: rows that
// would become orphaned by deleting their parent are not included.
func CleanupOrphans(dryRun bool) (map[string]int64, error) {
	deleted := make(map[string]int64)

	tables := GetAllTableNames()
	if len(tables) == 0 {
		return deleted, nil
	}

	var errs []error

	passes := maxOrphanPasses
	if dryRun {
		passes = 1
	}

	for range passes {
		var passDeleted int64

		for _, table := range tables {
			if table == "schema_migrations" {
				continue
			}

			// Str1 = referenced (parent) table, Str2 = referencing column in
			// the child table, Str3 = referenced column (usually "id").
			// Composite foreign keys would need per-id grouping; the schema
			// only uses single-column keys (seq = 0).
			fks := GetrowsN[DbstaticThreeString](
				false,
				0,
				`SELECT "table", "from", coalesce("to",'id') FROM pragma_foreign_key_list(?) WHERE seq = 0`,
				&table,
			)

			for idx := range fks {
				n, err := orphanRows(table, &fks[idx], dryRun)
				if err != nil {
					errs = append(errs, err)
					continue
				}

				if n > 0 {
					deleted[table] += n
					passDeleted += n

					if !dryRun {
						logger.Logtype("info", 3).
							Str("table", table).
							Str("column", fks[idx].Str2).
							Str("parent", fks[idx].Str1).
							Int64("deleted", n).
							Msg("Removed orphaned rows")
					}
				}
			}
		}

		// A failing statement would fail identically on the next pass.
		if passDeleted == 0 || len(errs) > 0 {
			break
		}
	}

	return deleted, errors.Join(errs...)
}

// orphanRows removes (or with dryRun counts) rows from child whose
// referencing column (fk.Str2) points to a missing row in the parent table
// (fk.Str1, column fk.Str3). 0 and NULL are treated as "no parent" and left
// alone.
func orphanRows(child string, fk *DbstaticThreeString, dryRun bool) (int64, error) {
	// Identifiers come from sqlite_master/pragma_foreign_key_list (the schema
	// itself), not user input; they are quoted for safety regardless.
	where := ` WHERE "` + fk.Str2 + `" IS NOT NULL AND "` + fk.Str2 + `" != 0 AND "` + fk.Str2 + `" NOT IN (SELECT "` + fk.Str3 + `" FROM "` + fk.Str1 + `")`

	if dryRun {
		var n int64
		if err := dbData.QueryRowContext(sqlCTX, `SELECT COUNT(*) FROM "`+child+`"`+where).Scan(&n); err != nil {
			return 0, fmt.Errorf("count orphans in %s (parent %s): %w", child, fk.Str1, err)
		}

		return n, nil
	}

	readWriteMu.Lock()
	defer readWriteMu.Unlock()

	result, err := dbData.ExecContext(sqlCTX, `DELETE FROM "`+child+`"`+where)
	if err != nil {
		logger.Logtype("error", 2).
			Str("table", child).
			Str("parent", fk.Str1).
			Err(err).
			Msg("orphan cleanup failed")

		return 0, fmt.Errorf("delete orphans in %s (parent %s): %w", child, fk.Str1, err)
	}

	n, _ := result.RowsAffected()

	return n, nil
}

// ConsistencyCheck is one database consistency check of the media cleanup
// page: rows that break an invariant the rest of the app relies on.
type ConsistencyCheck struct {
	Key         string
	Title       string
	Description string // what the rows are and what the repair does
	parts       []checkPart
}

// checkPart is the share of a check in one table. ids selects the ids of the
// affected rows of table; sample returns one text column describing an
// affected row (LIMIT is appended); repair fixes the rows given as id IN (...).
type checkPart struct {
	table, ids, sample, repair string
}

// ConsistencyResult is the outcome of counting or repairing one check.
type ConsistencyResult struct {
	Counts  map[string]int64 // affected (or repaired) rows per table
	Samples []string
	Note    string // repair caveats, e.g. rows that were skipped
}

// Total is the sum of Counts.
func (r *ConsistencyResult) Total() int64 {
	var n int64
	for _, c := range r.Counts {
		n += c
	}

	return n
}

// mediaFileTables pairs each list-entry table with its file table and the
// file table's reference to it.
var mediaFileTables = []struct{ entries, files, fk string }{
	{"movies", "movie_files", "movie_id"},
	{"serie_episodes", "serie_episode_files", "serie_episode_id"},
	{"albums", "album_files", "album_id"},
	{"audiobooks", "audiobook_files", "audiobook_id"},
	{"books", "book_files", "book_id"},
}

// requiredRefs are reference columns that are never 0 in a valid row. The
// orphan cleanup skips 0 (it means "no parent" for optional references), so
// such rows survive it.
var requiredRefs = []struct{ table, column string }{
	{"movies", "dbmovie_id"},
	{"series", "dbserie_id"},
	{"serie_episodes", "serie_id"},
	{"serie_episodes", "dbserie_episode_id"},
	{"albums", "dbalbum_id"},
	{"audiobooks", "dbaudiobook_id"},
	{"books", "dbbook_id"},
}

// missingEpisodeLinks lists (serie_id, dbserie_id, dbserie_episode_id) for
// known episodes of a tracked series that have no serie_episodes row.
// serie_episodes has no index, so this is a set difference rather than a
// correlated NOT EXISTS (0.05 s instead of minutes on 116k rows).
const missingEpisodeLinks = `SELECT m.serie_id, s.dbserie_id, m.ep FROM (
	SELECT s.id serie_id, d.id ep FROM series s JOIN dbserie_episodes d ON d.dbserie_id = s.dbserie_id
	EXCEPT SELECT serie_id, dbserie_episode_id FROM serie_episodes) m
	JOIN series s ON s.id = m.serie_id`

const episodeLinksKey = "episodelinks"

// ConsistencyChecks returns the checks in the order they should be repaired:
// deleting rows first, so later checks don't count rows that go away.
func ConsistencyChecks() []ConsistencyCheck {
	empty := make([]checkPart, 0, len(mediaFileTables)+len(requiredRefs))
	for _, ft := range mediaFileTables {
		empty = append(empty, checkPart{
			table:  ft.files,
			ids:    "SELECT id FROM " + ft.files + " WHERE coalesce(" + ft.fk + ", 0) = 0",
			sample: "SELECT location FROM " + ft.files + " WHERE coalesce(" + ft.fk + ", 0) = 0",
			repair: "DELETE FROM " + ft.files,
		})
	}

	for _, ref := range requiredRefs {
		empty = append(empty, checkPart{
			table: ref.table,
			ids:   "SELECT id FROM " + ref.table + " WHERE coalesce(" + ref.column + ", 0) = 0",
			sample: "SELECT '" + ref.table + " id ' || id || ': " + ref.column + " is 0' FROM " + ref.table +
				" WHERE coalesce(" + ref.column + ", 0) = 0",
			repair: "DELETE FROM " + ref.table,
		})
	}

	noExt := make([]checkPart, 0, len(mediaFileTables))
	for _, ft := range mediaFileTables {
		noExt = append(noExt, checkPart{
			table:  ft.files,
			ids:    "SELECT id FROM " + ft.files + " WHERE coalesce(extension, '') = ''",
			sample: "SELECT location FROM " + ft.files + " WHERE coalesce(extension, '') = ''",
			repair: "DELETE FROM " + ft.files,
		})
	}

	dups := make([]checkPart, 0, len(mediaFileTables))
	missing := make([]checkPart, 0, len(mediaFileTables))
	reached := make([]checkPart, 0, len(mediaFileTables))

	for _, ft := range mediaFileTables {
		dupWhere := " FROM " + ft.files + " x WHERE EXISTS (SELECT 1 FROM " + ft.files +
			" y WHERE y.location = x.location AND y." + ft.fk + " = x." + ft.fk + " AND y.id < x.id)"
		dups = append(dups, checkPart{
			table:  ft.files,
			ids:    "SELECT x.id" + dupWhere,
			sample: "SELECT x.location" + dupWhere,
			repair: "DELETE FROM " + ft.files,
		})

		// serie_episodes has no listname; its samples name the series list.
		list := "p.listname"
		if ft.entries == "serie_episodes" {
			list = "(SELECT listname FROM series WHERE id = p.serie_id)"
		}

		noFiles := "NOT EXISTS (SELECT 1 FROM " + ft.files + " f WHERE f." + ft.fk + " = p.id)"
		missingWhere := " FROM " + ft.entries + " p WHERE coalesce(p.missing, 0) != (" + noFiles + ")"
		missing = append(missing, checkPart{
			table: ft.entries,
			ids:   "SELECT p.id" + missingWhere,
			sample: "SELECT '" + ft.entries + " id ' || p.id || ' (' || " + list + " || '): missing=' || coalesce(p.missing, 0) || ', ' || " +
				"(SELECT count(*) FROM " + ft.files + " f WHERE f." + ft.fk + " = p.id) || ' files'" + missingWhere,
			repair: "UPDATE " + ft.entries + " SET missing = NOT EXISTS (SELECT 1 FROM " + ft.files +
				" f WHERE f." + ft.fk + " = " + ft.entries + ".id)",
		})

		reachedWhere := " FROM " + ft.entries + " p WHERE p.quality_reached = 1 AND " + noFiles
		reached = append(reached, checkPart{
			table:  ft.entries,
			ids:    "SELECT p.id" + reachedWhere,
			sample: "SELECT '" + ft.entries + " id ' || p.id || ' (' || " + list + " || ')'" + reachedWhere,
			repair: "UPDATE " + ft.entries + " SET quality_reached = 0",
		})
	}

	return []ConsistencyCheck{
		{
			Key:   "emptyrefs",
			Title: "Rows without a parent reference",
			Description: "File rows that belong to no list entry, and list entries that point to no movie, series, " +
				"episode, album or book (reference 0). The orphan cleanup skips 0, so they stay forever. Repair deletes them; " +
				"the files are picked up again by the next library scan.",
			parts: empty,
		},
		{
			Key:   "orphans",
			Title: "Orphaned rows",
			Description: "Rows whose parent row was deleted (the schema's ON DELETE CASCADE, which SQLite doesn't enforce here). " +
				"Repair deletes them, repeating until no new orphans appear. Same as Database Maintenance → Orphan cleanup.",
		},
		{
			Key:   "noextension",
			Title: "File rows without a file extension",
			Description: "File rows whose location has no extension, usually a folder recorded as a file. " +
				"They match nothing and keep their entry from being searched. Repair deletes them.",
			parts: noExt,
		},
		{
			Key:   "dupfiles",
			Title: "Duplicate file rows",
			Description: "The same file recorded more than once for the same entry (usually two scans a few minutes apart). " +
				"Repair keeps the oldest row. A multi-episode file linked to several episodes is not a duplicate.",
			parts: dups,
		},
		{
			Key:   episodeLinksKey,
			Title: "Episodes not linked to their series",
			Description: "Known episodes of a tracked series without an episode row, usually because the series import stopped " +
				"early. Such episodes are never searched or matched. Repair adds them as missing, with the series' quality profile.",
		},
		{
			Key:   "missingflag",
			Title: "Missing flag out of sync",
			Description: "Entries marked missing although they have files, or not missing although they have none. " +
				"Wrong flags make the searcher look for what you have and skip what you don't. " +
				"Repair recomputes the flag from the file rows (like the checkmissingflag job), " +
				"so entries without files will be searched again.",
			parts: missing,
		},
		{
			Key:         "reachedflag",
			Title:       "Quality reached without files",
			Description: "Entries marked quality reached that have no file, so upgrade searches skip them. Repair clears the flag.",
			parts:       reached,
		},
	}
}

// RunConsistencyCheck counts (repair false) or repairs the rows of the check
// with the given key. episodeQuality resolves the quality profile for new
// episode rows of a series that has none (list template, then config default);
// only the episode link repair uses it.
func RunConsistencyCheck(key string, repair bool, sampleLimit int,
	episodeQuality func(serieID uint, listname string) string,
) (ConsistencyResult, error) {
	res := ConsistencyResult{Counts: map[string]int64{}}

	switch key {
	case "orphans":
		counts, err := CleanupOrphans(!repair)
		res.Counts = counts

		return res, err
	case episodeLinksKey:
		return episodeLinks(repair, sampleLimit, episodeQuality)
	}

	var check *ConsistencyCheck

	checks := ConsistencyChecks()
	for i := range checks {
		if checks[i].Key == key {
			check = &checks[i]
			break
		}
	}

	if check == nil {
		return res, fmt.Errorf("unknown consistency check %q", key)
	}

	var errs []error

	for _, part := range check.parts {
		if repair {
			n, err := execCounted(part.repair + " WHERE id IN (" + part.ids + ")")
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", part.table, err))
			}

			if n > 0 {
				res.Counts[part.table] += n
			}

			continue
		}

		var n int64
		if err := dbData.QueryRowContext(sqlCTX, "SELECT count(*) FROM ("+part.ids+")").Scan(&n); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", part.table, err))
			continue
		}

		if n == 0 {
			continue
		}

		res.Counts[part.table] += n
		if left := sampleLimit - len(res.Samples); left > 0 {
			res.Samples = append(res.Samples,
				GetrowsN[string](false, uint(left), part.sample+" LIMIT "+strconv.Itoa(left))...)
		}
	}

	if repair && res.Total() > 0 {
		logger.Logtype("info", 1).
			Str("check", key).
			Int64("rows", res.Total()).
			Msg("Repaired database consistency check")
	}

	return res, errors.Join(errs...)
}

func execCounted(query string) (int64, error) {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()

	result, err := dbData.ExecContext(sqlCTX, query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

type episodeLink struct{ serieID, dbserieID, episodeID uint }

func queryEpisodeLinks() ([]episodeLink, error) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := dbData.QueryContext(sqlCTX, missingEpisodeLinks)
	if err != nil {
		return nil, fmt.Errorf("missing episode links: %w", err)
	}
	defer rows.Close()

	var links []episodeLink

	for rows.Next() {
		var l episodeLink
		if err := rows.Scan(&l.serieID, &l.dbserieID, &l.episodeID); err != nil {
			return nil, fmt.Errorf("missing episode links: %w", err)
		}

		links = append(links, l)
	}

	return links, rows.Err()
}

// episodeLinks counts or creates the missing serie_episodes rows, the same
// rows jobImportDBSeries would insert: missing = 1 with the series' quality.
func episodeLinks(repair bool, sampleLimit int,
	episodeQuality func(serieID uint, listname string) string,
) (ConsistencyResult, error) {
	res := ConsistencyResult{Counts: map[string]int64{}}

	links, err := queryEpisodeLinks()
	if err != nil || len(links) == 0 {
		return res, err
	}

	bySerie := make(map[uint][]episodeLink)

	var order []uint

	for _, l := range links {
		if _, ok := bySerie[l.serieID]; !ok {
			order = append(order, l.serieID)
		}

		bySerie[l.serieID] = append(bySerie[l.serieID], l)
	}

	if !repair {
		res.Counts["serie_episodes"] = int64(len(links))

		for _, id := range order {
			if len(res.Samples) >= sampleLimit {
				break
			}

			name := Getdatarow[string](false,
				"SELECT d.seriename || ' (' || s.listname || ')' FROM series s JOIN dbseries d ON d.id = s.dbserie_id WHERE s.id = ?",
				&id)
			res.Samples = append(res.Samples, fmt.Sprintf("%s: %d episodes", name, len(bySerie[id])))
		}

		return res, nil
	}

	var (
		skipped []string
		errs    []error
	)

	for _, id := range order {
		// Same fallback order as jobImportDBSeries. All reads happen before the
		// transaction, which holds the write lock.
		quality := Getdatarow[string](false, "SELECT quality_profile FROM series WHERE id = ?", &id)
		if quality == "" {
			quality = Getdatarow[string](false,
				"SELECT quality_profile FROM serie_episodes WHERE serie_id = ? AND coalesce(quality_profile, '') != '' LIMIT 1",
				&id)
		}

		listname := Getdatarow[string](false, "SELECT listname FROM series WHERE id = ?", &id)
		if quality == "" && episodeQuality != nil {
			quality = episodeQuality(id, listname)
		}

		if quality == "" {
			skipped = append(skipped, fmt.Sprintf("series id %d (%s)", id, listname))
			continue
		}

		eps := bySerie[id]

		err := ExecNTx(func(exec func(querystring string, args ...any) error) error {
			for i := range eps {
				if err := exec(
					"INSERT INTO serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) VALUES (?, ?, 1, ?, ?)",
					&eps[i].dbserieID, &eps[i].serieID, &quality, &eps[i].episodeID,
				); err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("series id %d: %w", id, err))
			continue
		}

		res.Counts["serie_episodes"] += int64(len(eps))
	}

	if len(skipped) > 0 {
		res.Note = "Skipped, no quality profile found (set one on the series or its list): " + strings.Join(skipped, ", ")
	}

	if res.Total() > 0 {
		logger.Logtype("info", 1).
			Int64("rows", res.Total()).
			Msg("Added missing episode rows")
	}

	return res, errors.Join(errs...)
}
