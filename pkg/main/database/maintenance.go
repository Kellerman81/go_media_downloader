package database

import (
	"errors"
	"fmt"

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
