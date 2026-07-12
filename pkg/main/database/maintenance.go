package database

import (
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
// child table (only tables where rows were deleted are included).
func CleanupOrphans() map[string]int64 {
	deleted := make(map[string]int64)

	tables := GetAllTableNames()
	if len(tables) == 0 {
		return deleted
	}

	for range maxOrphanPasses {
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
				n := deleteOrphanRows(table, &fks[idx])
				if n > 0 {
					deleted[table] += n
					passDeleted += n

					logger.Logtype("info", 3).
						Str("table", table).
						Str("column", fks[idx].Str2).
						Str("parent", fks[idx].Str1).
						Int64("deleted", n).
						Msg("Removed orphaned rows")
				}
			}
		}

		if passDeleted == 0 {
			break
		}
	}

	return deleted
}

// deleteOrphanRows removes rows from child whose referencing column (fk.Str2)
// points to a missing row in the parent table (fk.Str1, column fk.Str3).
// 0 and NULL are treated as "no parent" and left alone.
func deleteOrphanRows(child string, fk *DbstaticThreeString) int64 {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()

	// Identifiers come from sqlite_master/pragma_foreign_key_list (the schema
	// itself), not user input; they are quoted for safety regardless.
	result, err := dbData.ExecContext(
		sqlCTX,
		`DELETE FROM "`+child+`" WHERE "`+fk.Str2+`" IS NOT NULL AND "`+fk.Str2+`" != 0 AND "`+fk.Str2+`" NOT IN (SELECT "`+fk.Str3+`" FROM "`+fk.Str1+`")`,
	)
	if err != nil {
		logger.Logtype("error", 2).
			Str("table", child).
			Str("parent", fk.Str1).
			Err(err).
			Msg("orphan cleanup failed")

		return 0
	}

	n, _ := result.RowsAffected()

	return n
}
