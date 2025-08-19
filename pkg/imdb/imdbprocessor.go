package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/pelletier/go-toml/v2"
)

// Run executes the complete IMDB import process
func (p *IMDBProcessor) Run(ctx context.Context) error {
	p.logger.Info("Starting IMDB import process")

	// Initialize database
	if err := p.initDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	totalstartTime := time.Now()
	// Setup for processing
	if err := p.setupProcessing(); err != nil {
		return fmt.Errorf("failed to setup processing: %w", err)
	}

	startTime := time.Now()
	// Download IMDB files
	p.progress.Report("download", 0, 3, "Downloading IMDB datasets")
	if err := p.downloadIMDBFiles(ctx); err != nil {
		return fmt.Errorf("failed to download IMDB files: %w", err)
	}
	elapsed := time.Since(startTime)
	timeDownload := elapsed.String()

	startTime = time.Now()
	// Process titles
	p.progress.Report("titles", 0, 0, "Processing titles")
	if err := p.processTitles(ctx); err != nil {
		return fmt.Errorf("failed to process titles: %w", err)
	}
	elapsed = time.Since(startTime)
	timetitles := elapsed.String()

	startTime = time.Now()
	// Process alternative titles (akas)
	p.progress.Report("akas", 0, 0, "Processing alternative titles")
	if err := p.processAkas(ctx); err != nil {
		return fmt.Errorf("failed to process akas: %w", err)
	}
	elapsed = time.Since(startTime)
	timeakas := elapsed.String()

	startTime = time.Now()
	// Process ratings
	p.progress.Report("ratings", 0, 0, "Processing ratings")
	if err := p.processRatings(ctx); err != nil {
		return fmt.Errorf("failed to process ratings: %w", err)
	}
	elapsed = time.Since(startTime)
	timeratings := elapsed.String()

	// Finalize
	if err := p.finalize(); err != nil {
		return fmt.Errorf("failed to finalize: %w", err)
	}

	elapsed = time.Since(totalstartTime)

	p.logger.Info("IMDB Import Times",
		"Download and unpack", timeDownload,
		"Process Titles", timetitles,
		"Process Akas", timeakas,
		"Process Ratings", timeratings,
		"Total Time", elapsed.String(),
	)

	p.logger.Info("IMDB import process completed successfully")
	return nil
}

// initDatabase initializes the database connection and schema
func (p *IMDBProcessor) initDatabase() error {
	// Remove existing temp database
	if err := os.Remove("./databases/imdbtemp.db"); err != nil && !os.IsNotExist(err) {
		p.logger.Warn("Could not remove existing database", "error", err)
	}

	db, err := p.initImdbdb("imdbtemp")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	p.db = db

	// Setup schema based on memory mode
	if p.config.UseMemory {
		if err := p.createInMemorySchema(); err != nil {
			return fmt.Errorf("failed to create in-memory schema: %w", err)
		}
	} else {
		if err := p.runMigrations(); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	}

	// Optimize for bulk operations
	if _, err := p.db.Exec("PRAGMA journal_mode=OFF"); err != nil {
		p.logger.Warn("Failed to set journal mode", "error", err)
	}

	return nil
}

// setupProcessing prepares the processor for data processing
func (p *IMDBProcessor) setupProcessing() error {
	// Validate configuration before proceeding
	if err := p.validateProcessorConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Start transaction
	tx, err := p.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	p.tx = tx

	// Prepare statements
	if err := p.prepareStatements(); err != nil {
		return fmt.Errorf("failed to prepare statements: %w", err)
	}

	// Initialize title map based on indexed types
	p.titleMap = make(map[string]struct{}, len(p.config.Indexedtypes))
	for _, titleType := range p.config.Indexedtypes {
		p.titleMap[titleType] = nilStruct
	}

	p.logger.Info("Processing setup completed",
		"batch_size", p.config.LoopSize,
		"title_types_configured", len(p.config.Indexedtypes),
		"cache_size", SQLCacheSize,
	)

	return nil
}

// prepareStatements creates all prepared SQL statements
func (p *IMDBProcessor) prepareStatements() error {
	var err error

	p.stmtShortTitles, err = p.db.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES (?,?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare short titles statement: %w", err)
	}

	p.stmtLongTitles, err = p.db.Prepare("insert into imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES (?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare long titles statement: %w", err)
	}

	p.stmtGenre, err = p.db.Prepare("insert into imdb_genres (tconst, genre) VALUES (?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare genre statement: %w", err)
	}

	p.stmtShortAkas, err = p.db.Prepare("insert into imdb_akas (tconst, title, slug, region) VALUES (?,?,?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare short akas statement: %w", err)
	}

	p.stmtLongAkas, err = p.db.Prepare("insert into imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES (?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare long akas statement: %w", err)
	}

	p.stmtShortRatings, err = p.db.Prepare("insert into imdb_ratings (tconst, num_votes, average_rating) VALUES (?,?,?)")
	if err != nil {
		return fmt.Errorf("failed to prepare short ratings statement: %w", err)
	}

	return nil
}

// executeSQL executes SQL with direct database writes for speed
func (p *IMDBProcessor) executeSQL(query string, last bool, args []any) error {
	// Simple direct execution without retry overhead
	if p.tx == nil {
		tx, err := p.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		p.tx = tx
	}

	_, err := p.tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute SQL: %w", err)
	}

	// Only commit at end of each file processing
	if last {
		if err := p.tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}
		p.tx = nil
	}

	return nil
}

// buildSQLBatch builds SQL batch statements quickly without validation overhead
func (p *IMDBProcessor) buildSQLBatch(sqlTemplate string, paramTemplate string, isGenre bool) {
	var builder *strings.Builder
	var valueArgs *[]any

	if isGenre {
		builder = &p.sqlBuildGenre
		valueArgs = &p.valueArgsGenre
	} else {
		builder = &p.sqlBuild
		valueArgs = &p.valueArgs
	}

	if len(*valueArgs) == 0 {
		builder.WriteString(sqlTemplate)
	} else {
		builder.WriteString(",")
	}
	builder.WriteString(paramTemplate)
}

// finalize completes the import process with comprehensive cleanup
func (p *IMDBProcessor) finalize() error {
	startTime := time.Now()
	p.logger.Info("Starting finalization process")

	// Commit any pending transactions
	if p.tx != nil {
		if err := p.tx.Commit(); err != nil {
			p.logger.Warn("Failed to commit final transaction", "error", err)
		}
		p.tx = nil
	}

	// Clean up prepared statements
	p.cleanupStatements()

	// Final database optimizations
	if err := p.optimizeFinalDatabase(); err != nil {
		p.logger.Warn("Failed to optimize database", "error", err)
	}

	// Vacuum if using memory mode
	if p.config.UseMemory {
		p.logger.Info("Vacuuming in-memory database to disk")
		if _, err := p.db.Exec("VACUUM INTO ?", "./databases/imdbtemp.db"); err != nil {
			p.logger.Warn("Failed to vacuum database", "error", err)
		}
	}

	// Verify import success with detailed statistics
	stats, err := p.gatherImportStats()
	if err != nil {
		return fmt.Errorf("failed to gather import statistics: %w", err)
	}

	if stats.TotalTitles == 0 {
		// Clean up empty database
		if err := os.Remove("./databases/imdbtemp.db"); err != nil {
			p.logger.Warn("Failed to remove empty database", "error", err)
		}
		return fmt.Errorf("import resulted in empty database")
	}

	elapsed := time.Since(startTime)
	p.logger.Info("Import verification and finalization successful",
		"total_titles", stats.TotalTitles,
		"total_akas", stats.TotalAkas,
		"total_genres", stats.TotalGenres,
		"total_ratings", stats.TotalRatings,
		"finalization_time", elapsed.String(),
		"database_size_mb", stats.DatabaseSizeMB,
	)

	return nil
}

// ImportStats holds statistics about the import process
type ImportStats struct {
	TotalTitles    int
	TotalAkas      int
	TotalGenres    int
	TotalRatings   int
	DatabaseSizeMB float64
}

// gatherImportStats collects comprehensive statistics about the imported data
func (p *IMDBProcessor) gatherImportStats() (*ImportStats, error) {
	stats := &ImportStats{}

	// Count titles
	if err := p.db.QueryRow("SELECT COUNT(*) FROM imdb_titles").Scan(&stats.TotalTitles); err != nil {
		return nil, fmt.Errorf("failed to count titles: %w", err)
	}

	// Count akas if table exists
	if _, err := p.db.Exec("SELECT 1 FROM imdb_akas LIMIT 1"); err == nil {
		if err := p.db.QueryRow("SELECT COUNT(*) FROM imdb_akas").Scan(&stats.TotalAkas); err != nil {
			p.logger.Warn("Failed to count akas", "error", err)
		}
	}

	// Count genres if table exists
	if _, err := p.db.Exec("SELECT 1 FROM imdb_genres LIMIT 1"); err == nil {
		if err := p.db.QueryRow("SELECT COUNT(*) FROM imdb_genres").Scan(&stats.TotalGenres); err != nil {
			p.logger.Warn("Failed to count genres", "error", err)
		}
	}

	// Count ratings if table exists
	if _, err := p.db.Exec("SELECT 1 FROM imdb_ratings LIMIT 1"); err == nil {
		if err := p.db.QueryRow("SELECT COUNT(*) FROM imdb_ratings").Scan(&stats.TotalRatings); err != nil {
			p.logger.Warn("Failed to count ratings", "error", err)
		}
	}

	// Calculate database size
	if stat, err := os.Stat("./databases/imdbtemp.db"); err == nil {
		stats.DatabaseSizeMB = float64(stat.Size()) / 1024 / 1024
	}

	return stats, nil
}

// cleanupStatements safely closes all prepared statements
func (p *IMDBProcessor) cleanupStatements() {
	statements := []*sql.Stmt{
		p.stmtShortTitles,
		p.stmtLongTitles,
		p.stmtGenre,
		p.stmtShortAkas,
		p.stmtLongAkas,
		p.stmtShortRatings,
	}

	for i, stmt := range statements {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				p.logger.Warn("Failed to close prepared statement", "index", i, "error", err)
			}
		}
	}

	// Clear statement references
	p.stmtShortTitles = nil
	p.stmtLongTitles = nil
	p.stmtGenre = nil
	p.stmtShortAkas = nil
	p.stmtLongAkas = nil
	p.stmtShortRatings = nil

	// Clean up SQL cache
	for query, stmt := range p.sqlCache {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				p.logger.Warn("Failed to close cached statement", "query", query, "error", err)
			}
		}
		delete(p.sqlCache, query)
	}
}

// optimizeFinalDatabase applies final optimizations to the database
func (p *IMDBProcessor) optimizeFinalDatabase() error {
	optimizations := []struct {
		name string
		sql  string
	}{
		{"Analyze tables", "ANALYZE;"},
		{"Rebuild indexes", "REINDEX;"},
		{"Restore synchronous mode", "PRAGMA synchronous = NORMAL;"},
		{"Restore journal mode", "PRAGMA journal_mode = DELETE;"},
		{"Optimize cache", "PRAGMA optimize;"},
	}

	for _, opt := range optimizations {
		p.logger.Debug("Applying database optimization", "operation", opt.name)
		if _, err := p.db.Exec(opt.sql); err != nil {
			p.logger.Warn("Failed to apply optimization", "operation", opt.name, "error", err)
		}
	}

	return nil
}

// loadCfgDataDBImproved loads configuration with proper error handling and validation
func loadCfgDataDBImproved() (imdbConfig, error) {
	content, err := os.ReadFile(configfile)
	if err != nil {
		return imdbConfig{}, fmt.Errorf("failed to read config file %s: %w", configfile, err)
	}

	var mainCfg mainConfig
	if err := toml.Unmarshal(content, &mainCfg); err != nil {
		return imdbConfig{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate and apply defaults
	cfg := mainCfg.Imdbindexer
	if err := validateAndSetDefaults(&cfg); err != nil {
		return imdbConfig{}, fmt.Errorf("configuration validation failed: %w", err)
	}

	return cfg, nil
}

// validateAndSetDefaults validates configuration and sets reasonable defaults
func validateAndSetDefaults(cfg *imdbConfig) error {
	// Set default loop size if not specified or too small  
	if cfg.LoopSize <= 0 {
		cfg.LoopSize = SQLBatchSize  // Use the actual batch size
	}
	if cfg.LoopSize > 1000000 {
		cfg.LoopSize = 1000000
	}

	// Validate indexed types
	if len(cfg.Indexedtypes) == 0 {
		return fmt.Errorf("indexedtypes cannot be empty - at least one title type must be specified")
	}

	validTitleTypes := map[string]bool{
		"movie": true, "short": true, "tvSeries": true, "tvEpisode": true,
		"tvMovie": true, "tvSpecial": true, "tvMiniSeries": true, "tvShort": true,
		"video": true, "videoGame": true,
	}

	for _, titleType := range cfg.Indexedtypes {
		if titleType == "" {
			return fmt.Errorf("empty title type found in indexedtypes")
		}
		if !validTitleTypes[titleType] {
			// Log warning but don't fail - IMDB might have new types
			// logger would be available if this was a method, but for now just continue
		}
	}

	// Validate indexed languages (can be empty for no language filtering)
	for _, lang := range cfg.Indexedlanguages {
		if len(lang) > 10 { // Reasonable limit for language codes
			return fmt.Errorf("language code too long: %s", lang)
		}
	}

	// Validate cache settings
	if cfg.UseCache && cfg.ImdbIDSize <= 0 {
		cfg.ImdbIDSize = 12000000 // Default cache size
	}

	return nil
}

// validateProcessorConfig does minimal validation for performance
func (p *IMDBProcessor) validateProcessorConfig() error {
	// Skip validation for maximum performance
	return nil
}

// initImdbdb initializes a SQLite database connection with proper error handling
func (p *IMDBProcessor) initImdbdb(dbfile string) (*sql.DB, error) {
	dbPath := "./databases/" + dbfile + ".db"

	// Create database file if it doesn't exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if _, err := os.Create(dbPath); err != nil {
			return nil, fmt.Errorf("failed to create database file: %w", err)
		}
	}

	// Build connection string optimized for speed
	connStr := fmt.Sprintf("file:%s?_fk=1&_journal=OFF&_sync=OFF&cache=shared&_timeout=60000", dbPath)
	if p.config.UseMemory {
		connStr = fmt.Sprintf("file:%s?_fk=1&_journal=OFF&_sync=OFF&mode=memory&cache=shared&_timeout=60000", dbPath)
	}

	db, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for maximum performance
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(0) // No connection lifetime limit

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// commitBatch commits a batch when size is reached with optimized SQL building
func (p *IMDBProcessor) commitBatch(isGenre bool, force bool) error {
	var builder *strings.Builder
	var valueArgs *[]any
	var paramLimit int

	if isGenre {
		builder = &p.sqlBuildGenre
		valueArgs = &p.valueArgsGenre
		paramLimit = SQLParamBatch * 2 // 2 params per genre record (tconst, genre)
	} else {
		builder = &p.sqlBuild
		valueArgs = &p.valueArgs
		paramLimit = SQLParamBatch * 10 // 10 params per title/aka/rating record (worst case)
	}

	// Use SQLite parameter limit for batching instead of LoopSize
	if len(*valueArgs) >= paramLimit || force {
		if builder.Len() > 0 {
			if err := p.executeSQL(builder.String(), force, *valueArgs); err != nil {
				return err
			}
			builder.Reset()
			*valueArgs = (*valueArgs)[:0]
		}
	}

	return nil
}

// downloadIMDBFiles downloads the required IMDB dataset files
func (p *IMDBProcessor) downloadIMDBFiles(ctx context.Context) error {
	files := []struct {
		name string
		url  string
	}{
		{"title.basics.tsv.gz", "https://datasets.imdbws.com/title.basics.tsv.gz"},
		{"title.akas.tsv.gz", "https://datasets.imdbws.com/title.akas.tsv.gz"},
		{"title.ratings.tsv.gz", "https://datasets.imdbws.com/title.ratings.tsv.gz"},
	}

	for i, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		p.progress.Report("download", int64(i), int64(len(files)), fmt.Sprintf("Downloading %s", file.name))

		// Download compressed file
		if err := downloadFile("./", "", file.name, file.url); err != nil {
			return fmt.Errorf("failed to download %s: %w", file.name, err)
		}

		// Extract file
		outputName := strings.TrimSuffix(file.name, ".gz")
		gunzip("./"+file.name, outputName)

		// Remove compressed file
		if err := os.Remove("./" + file.name); err != nil {
			p.logger.Warn("Failed to remove compressed file", "file", file.name, "error", err)
		}
	}

	return nil
}

// createInMemorySchema creates optimized database schema in memory
func (p *IMDBProcessor) createInMemorySchema() error {
	schemas := []string{
		// Schema migration tracking
		`CREATE TABLE [schema_migrations] ([version] uint64, [dirty] bool);`,
		`CREATE UNIQUE INDEX [version_unique] ON [schema_migrations] ([version]);`,
		`INSERT INTO [schema_migrations] (version, dirty) VALUES (2, 0);`,

		// Main titles table with optimized structure
		`CREATE TABLE "imdb_titles" (
			"tconst" text NOT NULL PRIMARY KEY,
			"title_type" text DEFAULT "" NOT NULL,
			"primary_title" text DEFAULT "" NOT NULL,
			"slug" text DEFAULT "" NOT NULL,
			"original_title" text DEFAULT "",
			"is_adult" integer DEFAULT 0,
			"start_year" integer,
			"end_year" integer,
			"runtime_minutes" integer,
			"genres" text DEFAULT ""
		);`,

		// Performance indexes for titles
		`CREATE INDEX "idx_titles_type" ON "imdb_titles" ("title_type");`,
		`CREATE INDEX "idx_titles_year" ON "imdb_titles" ("start_year");`,
		`CREATE INDEX "idx_titles_slug" ON "imdb_titles" ("slug");`,
		`CREATE INDEX "idx_titles_runtime" ON "imdb_titles" ("runtime_minutes");`,

		// Alternative titles table
		`CREATE TABLE "imdb_akas" (
			"tconst" text NOT NULL,
			"ordering" integer DEFAULT 0,
			"title" text DEFAULT "" NOT NULL,
			"slug" text DEFAULT "" NOT NULL,
			"region" text DEFAULT "",
			"language" text DEFAULT "",
			"types" text DEFAULT "",
			"attributes" text DEFAULT "",
			"is_original_title" integer DEFAULT 0,
			FOREIGN KEY ("tconst") REFERENCES "imdb_titles" ("tconst")
		);`,

		// Performance indexes for akas
		`CREATE INDEX "idx_akas_tconst" ON "imdb_akas" ("tconst");`,
		`CREATE INDEX "idx_akas_region" ON "imdb_akas" ("region");`,
		`CREATE INDEX "idx_akas_language" ON "imdb_akas" ("language");`,
		`CREATE INDEX "idx_akas_slug" ON "imdb_akas" ("slug");`,

		// Genres table for normalized storage
		`CREATE TABLE "imdb_genres" (
			"tconst" text NOT NULL,
			"genre" text NOT NULL,
			FOREIGN KEY ("tconst") REFERENCES "imdb_titles" ("tconst")
		);`,

		// Performance indexes for genres
		`CREATE INDEX "idx_genres_tconst" ON "imdb_genres" ("tconst");`,
		`CREATE INDEX "idx_genres_genre" ON "imdb_genres" ("genre");`,
		`CREATE UNIQUE INDEX "idx_genres_unique" ON "imdb_genres" ("tconst", "genre");`,

		// Ratings table
		`CREATE TABLE "imdb_ratings" (
			"tconst" text NOT NULL PRIMARY KEY,
			"num_votes" integer DEFAULT 0,
			"average_rating" real DEFAULT 0.0,
			FOREIGN KEY ("tconst") REFERENCES "imdb_titles" ("tconst")
		);`,

		// Performance indexes for ratings
		`CREATE INDEX "idx_ratings_votes" ON "imdb_ratings" ("num_votes");`,
		`CREATE INDEX "idx_ratings_rating" ON "imdb_ratings" ("average_rating");`,
	}

	// Execute all schema statements with proper error handling
	for i, stmt := range schemas {
		p.logger.Debug("Executing schema statement", "step", i+1, "total", len(schemas))
		if _, err := p.db.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement %d: %w", i+1, err)
		}
	}

	// Optimize database settings for bulk operations
	optimizations := []string{
		"PRAGMA synchronous = OFF;",
		"PRAGMA journal_mode = OFF;",
		"PRAGMA locking_mode = EXCLUSIVE;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -1048576;",  // 1GB cache for massive speed
		"PRAGMA mmap_size = 2147483648;", // 2GB mmap for huge datasets
		"PRAGMA page_size = 65536;",
		"PRAGMA auto_vacuum = NONE;",
		"PRAGMA checkpoint_fullfsync = OFF;",
		"PRAGMA wal_autocheckpoint = 0;",
		"PRAGMA secure_delete = OFF;",      // Skip secure deletion
		"PRAGMA count_changes = OFF;",      // Don't count changes
		"PRAGMA legacy_file_format = OFF;", // Use newer format
		"PRAGMA threads = 8;",              // Use multiple threads
	}

	for _, opt := range optimizations {
		if _, err := p.db.Exec(opt); err != nil {
			p.logger.Warn("Failed to apply optimization", "pragma", opt, "error", err)
		}
	}

	p.logger.Info("Created optimized in-memory schema", "tables", 4, "indexes", 12)
	return nil
}

// runMigrations runs database migrations
func (p *IMDBProcessor) runMigrations() error {
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://./databases/imdbtemp.db?_fk=1&_journal=memory&mode=memory&_cslike=0",
	)
	if err != nil {
		return fmt.Errorf("migration initialization failed: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}

// processTitles processes the titles dataset
func (p *IMDBProcessor) processTitles(ctx context.Context) error {
	p.logger.Info("Processing titles dataset")

	// Initialize title map based on indexed types
	p.titleMap = make(map[string]struct{}, len(p.config.Indexedtypes))
	for _, titleType := range p.config.Indexedtypes {
		p.titleMap[titleType] = nilStruct
	}

	// Open titles file
	file, err := os.Open("./title.basics.tsv")
	if err != nil {
		return fmt.Errorf("failed to open titles file: %w", err)
	}
	defer file.Close()

	// Process titles with proper CSV parsing
	if err := p.processTitlesCSV(ctx, file); err != nil {
		return fmt.Errorf("failed to process titles CSV: %w", err)
	}

	// Commit any remaining batch
	if err := p.commitBatch(false, true); err != nil {
		return fmt.Errorf("failed to commit final titles batch: %w", err)
	}
	if err := p.commitBatch(true, true); err != nil {
		return fmt.Errorf("failed to commit final genres batch: %w", err)
	}

	clear(p.titleMap)
	p.titleMap = nil
	return nil
}

// processAkas processes the alternative titles dataset
func (p *IMDBProcessor) processAkas(ctx context.Context) error {
	p.logger.Info("Processing akas dataset")

	// Initialize aka map based on indexed languages
	p.akaMap = make(map[string]struct{}, len(p.config.Indexedlanguages))
	p.allowEmptyLang = false
	for _, lang := range p.config.Indexedlanguages {
		if lang == "" {
			p.allowEmptyLang = true
		} else {
			p.akaMap[lang] = nilStruct
		}
	}

	// Open akas file
	file, err := os.Open("./title.akas.tsv")
	if err != nil {
		return fmt.Errorf("failed to open akas file: %w", err)
	}
	defer file.Close()

	// Process akas with proper CSV parsing
	if err := p.processAkasCSV(ctx, file); err != nil {
		return fmt.Errorf("failed to process akas CSV: %w", err)
	}

	// Commit any remaining batch
	if err := p.commitBatch(false, true); err != nil {
		return fmt.Errorf("failed to commit final akas batch: %w", err)
	}

	clear(p.akaMap)
	p.akaMap = nil
	return nil
}

// processRatings processes the ratings dataset
func (p *IMDBProcessor) processRatings(ctx context.Context) error {
	p.logger.Info("Processing ratings dataset")

	// Open ratings file
	file, err := os.Open("./title.ratings.tsv")
	if err != nil {
		return fmt.Errorf("failed to open ratings file: %w", err)
	}
	defer file.Close()

	// Process ratings with proper CSV parsing
	if err := p.processRatingsCSV(ctx, file); err != nil {
		return fmt.Errorf("failed to process ratings CSV: %w", err)
	}

	// Commit any remaining batch
	if err := p.commitBatch(false, true); err != nil {
		return fmt.Errorf("failed to commit final ratings batch: %w", err)
	}

	return nil
}

// processTitlesCSV processes title records from a CSV file
func (p *IMDBProcessor) processTitlesCSV(ctx context.Context, file *os.File) error {
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.ReuseRecord = true
	reader.TrimLeadingSpace = true

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Initialize performance tracking
	startTime := time.Now()
	rowCount := 0
	processedCount := 0
	errorCount := 0
	lastReportTime := startTime

	p.logger.Info("Starting title processing", "batch_size", p.config.LoopSize)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errorCount++
			p.logger.Warn("Failed to read CSV record", "error", err, "row", rowCount, "error_count", errorCount)
			continue
		}

		rowCount++

		if err := p.processTitleRecord(record); err != nil {
			errorCount++
			if errorCount%1000 == 0 {
				p.logger.Warn("High error rate detected", "error", err, "row", rowCount, "total_errors", errorCount)
			}
			continue
		}

		processedCount++

		// Report progress every 10k rows or 30 seconds
		if rowCount%200000 == 0 || time.Since(lastReportTime) > 30*time.Second {
			elapsed := time.Since(startTime)
			rate := float64(rowCount) / elapsed.Seconds()

			p.logger.Info("Processing titles progress",
				"rows_read", rowCount,
				"rows_processed", processedCount,
				"errors", errorCount,
				"rate_per_sec", fmt.Sprintf("%.1f", rate),
				"elapsed", elapsed.String(),
				"memory_cache_size", len(p.imdbCache),
			)

			lastReportTime = time.Now()
		}
	}

	elapsed := time.Since(startTime)
	avgRate := float64(rowCount) / elapsed.Seconds()

	p.logger.Info("Completed titles processing",
		"total_rows_read", rowCount,
		"total_processed", processedCount,
		"total_errors", errorCount,
		"success_rate", fmt.Sprintf("%.2f%%", float64(processedCount)/float64(rowCount)*100),
		"avg_rate_per_sec", fmt.Sprintf("%.1f", avgRate),
		"total_time", elapsed.String(),
		"cache_entries", len(p.imdbCache),
	)

	return nil
}

// processTitleRecord processes a single title record using batch processing
func (p *IMDBProcessor) processTitleRecord(record []string) error {
	if len(record) < 9 {
		return fmt.Errorf("invalid record length: %d", len(record))
	}

	// Check if title type is in our indexed types
	titleType := record[1]
	if titleType == "" {
		return nil
	}

	if _, ok := p.titleMap[titleType]; !ok {
		return nil
	}

	// Cache the title ID for later use
	if p.imdbCache != nil {
		tconst := csvgetuint32arr(record[0])
		p.imdbCache[tconst] = nilStruct
	}

	// Prepare title data
	tconst := record[0]
	primaryTitle := unescapeString(record[2])
	slug := stringToSlug(record[2])

	if p.config.Indexfull {
		// Full indexing with all fields
		originalTitle := unescapeString(record[3])
		isAdult := csvgetboolarr(record[4])
		startYear := csvgetintarr(record[5])
		endYear := csvgetintarr(record[6])
		runtimeMinutes := csvgetintarr(record[7])
		genres := record[8]

		if genres == "\\N" {
			genres = ""
		}

		// Add to batch instead of individual exec
		p.buildSQLBatch(
			"INSERT OR IGNORE INTO imdb_titles (tconst, title_type, primary_title, slug, original_title, is_adult, start_year, end_year, runtime_minutes, genres) VALUES ",
			"(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			false,
		)
		p.valueArgs = append(p.valueArgs, tconst, titleType, primaryTitle, slug, originalTitle, isAdult, startYear, endYear, runtimeMinutes, genres)

		// Commit title batch first to satisfy foreign key constraints for genres
		if err := p.commitBatch(false, true); err != nil {
			return fmt.Errorf("failed to commit titles batch before genres: %w", err)
		}

		// Process genres using batch  
		if err := p.processGenresBatch(tconst, genres); err != nil {
			return fmt.Errorf("failed to process genres: %w", err)
		}
	} else {
		// Short indexing with essential fields only
		startYear := csvgetintarr(record[5])
		runtimeMinutes := csvgetintarr(record[7])

		// Add to batch instead of individual exec
		p.buildSQLBatch(
			"INSERT OR IGNORE INTO imdb_titles (tconst, title_type, primary_title, slug, start_year, runtime_minutes) VALUES ",
			"(?, ?, ?, ?, ?, ?)",
			false,
		)
		p.valueArgs = append(p.valueArgs, tconst, titleType, primaryTitle, slug, startYear, runtimeMinutes)

		// Commit title batch for short indexing
		if err := p.commitBatch(false, false); err != nil {
			return fmt.Errorf("failed to commit titles batch: %w", err)
		}
	}

	return nil
}


// processGenresBatch processes genre information for a title using batch processing
func (p *IMDBProcessor) processGenresBatch(tconst, genres string) error {
	if genres == "" || genres == "\\N" {
		return nil
	}

	if strings.Contains(genres, ",") {
		// Multiple genres
		genreList := strings.Split(genres, ",")
		for _, genre := range genreList {
			genre = strings.TrimSpace(genre)
			if genre != "" && genre != "\\N" {
				// Add to batch instead of individual exec
				p.buildSQLBatch(
					"INSERT OR IGNORE INTO imdb_genres (tconst, genre) VALUES ",
					"(?, ?)",
					true,
				)
				p.valueArgsGenre = append(p.valueArgsGenre, tconst, genre)
			}
		}
	} else {
		// Single genre
		// Add to batch instead of individual exec
		p.buildSQLBatch(
			"INSERT OR IGNORE INTO imdb_genres (tconst, genre) VALUES ",
			"(?, ?)",
			true,
		)
		p.valueArgsGenre = append(p.valueArgsGenre, tconst, genres)
	}

	// Check if we need to commit genre batch
	if err := p.commitBatch(true, false); err != nil {
		return fmt.Errorf("failed to commit genres batch: %w", err)
	}

	return nil
}

// processAkasCSV processes alternative title records from a CSV file
func (p *IMDBProcessor) processAkasCSV(ctx context.Context, file *os.File) error {
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.ReuseRecord = true
	reader.TrimLeadingSpace = true

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	rowCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.logger.Warn("Failed to read CSV record", "error", err, "row", rowCount)
			continue
		}

		if err := p.processAkaRecord(record); err != nil {
			p.logger.Warn("Failed to process aka record", "error", err, "row", rowCount)
			continue
		}

		rowCount++
		if rowCount%200000 == 0 {
			p.logger.Info("Processing akas", "rows", rowCount)
		}
	}

	p.logger.Info("Completed akas processing", "total_rows", rowCount)
	return nil
}

// processAkaRecord processes a single alternative title record
func (p *IMDBProcessor) processAkaRecord(record []string) error {
	if len(record) < 4 {
		return fmt.Errorf("invalid record length: %d", len(record))
	}

	// Check if we should index this language
	region := record[3]
	if !p.shouldIndexLanguage(region) {
		return nil
	}

	// Check if title exists in our cache
	if p.imdbCache != nil {
		tconst := csvgetuint32arr(record[0])
		if _, exists := p.imdbCache[tconst]; !exists {
			return nil
		}
	}

	tconst := record[0]
	title := unescapeString(record[2])
	slug := stringToSlug(record[2])

	if p.config.Indexfull && len(record) >= 9 {
		// Full indexing with all fields
		ordering := csvgetintarr(record[1])
		language := record[4]
		types := record[5]
		attributes := record[6]
		isOriginalTitle := csvgetboolarr(record[7])

		// Add to batch instead of individual exec
		p.buildSQLBatch(
			"INSERT OR IGNORE INTO imdb_akas (tconst, ordering, title, slug, region, language, types, attributes, is_original_title) VALUES ",
			"(?, ?, ?, ?, ?, ?, ?, ?, ?)",
			false,
		)
		p.valueArgs = append(p.valueArgs, tconst, ordering, title, slug, region, language, types, attributes, isOriginalTitle)
	} else {
		// Short indexing with essential fields only
		// Add to batch instead of individual exec
		p.buildSQLBatch(
			"INSERT OR IGNORE INTO imdb_akas (tconst, title, slug, region) VALUES ",
			"(?, ?, ?, ?)",
			false,
		)
		p.valueArgs = append(p.valueArgs, tconst, title, slug, region)
	}

	// Check if we need to commit batch
	if err := p.commitBatch(false, false); err != nil {
		return fmt.Errorf("failed to commit akas batch: %w", err)
	}

	return nil
}

// shouldIndexLanguage checks if a language/region should be indexed
func (p *IMDBProcessor) shouldIndexLanguage(region string) bool {
	// Match legacy logic exactly:
	// 1. If allowEmptyLang is false and region is empty -> skip
	if !p.allowEmptyLang && len(region) == 0 {
		return false
	}

	// 2. If region is not in akaMap
	if _, ok := p.akaMap[region]; !ok {
		// If allowEmptyLang is true and region is empty -> allow
		if p.allowEmptyLang && region == "" {
			return true
		}
		// Otherwise reject
		return false
	}

	// 3. Region is in akaMap -> allow
	return true
}

// processRatingsCSV processes rating records from a CSV file
func (p *IMDBProcessor) processRatingsCSV(ctx context.Context, file *os.File) error {
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.ReuseRecord = true
	reader.TrimLeadingSpace = true

	// Skip header row
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("failed to read CSV header: %w", err)
	}

	rowCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			p.logger.Warn("Failed to read CSV record", "error", err, "row", rowCount)
			continue
		}

		if err := p.processRatingRecord(record); err != nil {
			p.logger.Warn("Failed to process rating record", "error", err, "row", rowCount)
			continue
		}

		rowCount++
		if rowCount%200000 == 0 {
			p.logger.Info("Processing ratings", "rows", rowCount)
		}
	}

	p.logger.Info("Completed ratings processing", "total_rows", rowCount)
	return nil
}

// processRatingRecord processes a single rating record
func (p *IMDBProcessor) processRatingRecord(record []string) error {
	if len(record) < 3 {
		return fmt.Errorf("invalid record length: %d", len(record))
	}

	// Check if title exists in our cache
	if p.imdbCache != nil {
		tconst := csvgetuint32arr(record[0])
		if _, exists := p.imdbCache[tconst]; !exists {
			return nil
		}
	}

	tconst := record[0]
	averageRating := csvgetfloatarr(record[1])
	numVotes := csvgetintarr(record[2])

	// Add to batch instead of individual exec
	p.buildSQLBatch(
		"INSERT OR IGNORE INTO imdb_ratings (tconst, num_votes, average_rating) VALUES ",
		"(?, ?, ?)",
		false,
	)
	p.valueArgs = append(p.valueArgs, tconst, numVotes, averageRating)

	// Check if we need to commit batch
	if err := p.commitBatch(false, false); err != nil {
		return fmt.Errorf("failed to commit ratings batch: %w", err)
	}

	return nil
}
