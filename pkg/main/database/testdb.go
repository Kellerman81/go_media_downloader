package database

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"
)

// UseMigratedTestDB points the package at a fresh in-memory main database
// built from the *.up.sql migrations in schemaDir, and returns a function that
// restores the previous database. It lets tests in other packages run code
// that queries through this package. Not for production use.
func UseMigratedTestDB(schemaDir string) (func(), error) {
	return useMigratedTestDB(schemaDir, &dbData)
}

// UseMigratedTestIMDB is UseMigratedTestDB for the IMDb database
// (schema/imdbdb). Not for production use.
func UseMigratedTestIMDB(schemaDir string) (func(), error) {
	return useMigratedTestDB(schemaDir, &dbImdb)
}

func useMigratedTestDB(schemaDir string, target **sqlx.DB) (func(), error) {
	files, err := filepath.Glob(filepath.Join(schemaDir, "*.up.sql"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, os.ErrNotExist
	}

	sort.Strings(files)

	db, err := sqlx.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			db.Close()
			return nil, err
		}

		if _, err := db.Exec(string(b)); err != nil {
			db.Close()
			return nil, err
		}
	}

	NewCache(time.Hour, time.Hour)
	// Statements are cached by SQL text: drop any bound to the previous db.
	cache.ristrettoStmt.Clear()

	prev := *target
	*target = db

	return func() {
		// Statements are cached by SQL text; drop the ones bound to this db.
		cache.ristrettoStmt.Clear()
		*target = prev
		db.Close()
	}, nil
}
