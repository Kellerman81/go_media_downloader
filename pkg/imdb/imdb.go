package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	_ "net/http/pprof"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mozillazg/go-unidecode/table"
	"github.com/pelletier/go-toml/v2"

	"github.com/h2non/filetype"
)

type mainConfig struct {
	Imdbindexer imdbConfig `koanf:"imdbindexer" toml:"imdbindexer"`
}
type imdbConfig struct {
	Indexedtypes     []string `toml:"indexed_types"`
	Indexedlanguages []string `toml:"indexed_languages"`
	Indexfull        bool     `toml:"index_full"`
	ImdbIDSize       int      `toml:"imdbid_size"`
	UseMemory        bool     `toml:"use_memory"`
}

// Validate validates the configuration and sets defaults
func (c *imdbConfig) Validate() error {
	if c.ImdbIDSize <= 0 {
		c.ImdbIDSize = DefaultImdbIDSize
	}
	if c.ImdbIDSize > MaxImdbIDSize {
		return fmt.Errorf("imdbid_size %d exceeds maximum %d", c.ImdbIDSize, MaxImdbIDSize)
	}
	return nil
}

const (
	configfile = "./config/config.toml"
	tempDBPath = "./databases/imdbtemp.db"

	// Default configuration values
	DefaultImdbIDSize = 1200000

	// Maximum values for safety
	MaxImdbIDSize = 20000000

	// Optimized batch processing constants
	SQLParamBatch    = 99    // Max records per SQL batch (SQLite limit: 999 params, titles have 10 params: 99*10=990)
	ValueArgsCapInit = 50000 // Pre-allocated buffers

	// tsvMaxLineSize bounds a single dataset line; IMDb rows are far below this.
	tsvMaxLineSize = 1024 * 1024

	// Buffer sizes
	BufferPoolSize = 100
)

// Build info variables (set by build flags)
var (
	version    string
	buildstamp string
	githash    string
)

// Global objects that need to be package-level
var (
	PlBuffer  = newPool(BufferPoolSize, 0, nil, func(b *bytes.Buffer) { b.Reset() })
	nilStruct = struct{}{}
)

// Character substitution maps for slug generation
var (
	substituteRuneSpace = map[rune]string{
		'&':  "and",
		'@':  "at",
		'"':  "",
		'\'': "",
		'’':  "",
		'_':  "",
		' ':  "-",
		'‒':  "-", // figure dash
		'–':  "-", // en dash
		'—':  "-", // em dash
		'―':  "-", // horizontal bar
		'ä':  "ae",
		'Ä':  "ae",
		'ö':  "oe",
		'Ö':  "oe",
		'ü':  "ue",
		'Ü':  "ue",
		'ß':  "ss",
	}
	subRune = map[rune]bool{
		'a': true,
		'b': true,
		'c': true,
		'd': true,
		'e': true,
		'f': true,
		'g': true,
		'h': true,
		'i': true,
		'j': true,
		'k': true,
		'l': true,
		'm': true,
		'n': true,
		'o': true,
		'p': true,
		'q': true,
		'r': true,
		's': true,
		't': true,
		'u': true,
		'v': true,
		'w': true,
		'x': true,
		'y': true,
		'z': true,
		'0': true,
		'1': true,
		'2': true,
		'3': true,
		'4': true,
		'5': true,
		'6': true,
		'7': true,
		'8': true,
		'9': true,
		'-': true,
	}
)

// csvgetintarr converts the string value in record to an int.
// Returns 0 if the value is empty, "0", "\\N", or cannot be parsed as an int.
func csvgetintarr(record string) int {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return getint
}

// csvgetuint32arr converts the string value from the provided CSV
// record to a uint32. It returns 0 if the value is
// empty, "0", or "\\N".
func csvgetuint32arr(record string) uint32 {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	if err != nil {
		return 0
	}
	return uint32(getint)
}

// csvgetfloatarr converts the string value in record to a float32.
// Returns 0 if the value is empty, "0", "0.0", "\\N", or cannot be parsed as a float32.
func csvgetfloatarr(record string) float32 {
	if record == "" || record == "0" || record == "0.0" || record == "\\N" {
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// csvgetboolarr converts the string value in record to a bool.
// Returns false if the value is "\\N", otherwise returns true if the
// value is "1", "t", "T", "true", "TRUE", or "True", and false otherwise.
func csvgetboolarr(record string) bool {
	if record == "" || record == "\\N" {
		return false
	}
	switch record {
	case "1", "t", "T", "true", "TRUE", "True":
		return true
	}
	return false
}

func main() {
	if err := run(); err != nil {
		slog.Error("Application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Setup structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig)
		cancel()
	}()

	logger.Info("IMDB Importer starting",
		"version", version,
		"githash", githash,
		"buildstamp", buildstamp)

	// Load and validate configuration
	cfg, err := loadCfgDataDBImproved()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create processor
	processor, err := newimdbProcessor(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create processor: %w", err)
	}
	runErr := processor.Run(ctx)
	if closeErr := processor.Close(); closeErr != nil {
		logger.Error("Error closing processor", "error", closeErr)
		if runErr == nil {
			runErr = closeErr
		}
	}
	if runErr != nil {
		// The main app swaps in imdbtemp.db after a successful exit; never leave
		// a partial one behind.
		if err := os.Remove(tempDBPath); err != nil && !os.IsNotExist(err) {
			logger.Error("Failed to remove incomplete temp database", "file", tempDBPath, "error", err)
		}
		return fmt.Errorf("import process failed: %w", runErr)
	}

	logger.Info("IMDB import completed successfully")
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

	return nil
}

// unescapeString unescapes HTML entities in the string of the given record.
// Optimized for maximum performance with fast path checks.
func unescapeString(record string) string {
	if record == "" || record == "\\N" {
		return ""
	}
	if strings.ContainsRune(record, '&') {
		return html.UnescapeString(record)
	}
	return record
}

// stringToSlug converts a string of the record
// to a slug format. Optimized for performance.
func stringToSlug(instr string) string {
	if len(instr) == 0 {
		return ""
	}

	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}
	inbyte = bytes.TrimRight(inbyte, "- ")
	inbyte = bytes.TrimLeft(inbyte, "- ")
	return string(inbyte)
}

// unidecode2 converts a unicode string to an ASCII transliteration by
// replacing each unicode rune with its best ASCII approximation. It handles
// special cases like converting to lowercase and inserting separators between
// contiguous substitutions. This allows sanitizing unicode strings into
// a more filesystem-friendly ASCII format.
func unidecode2(s string) []byte {
	ret := PlBuffer.Get()
	var laststr string
	var lastrune rune
	// var c byte
	// Fast check for '&' using byte indexing instead of ContainsRune
	if strings.ContainsRune(s, '&') {
		s = html.UnescapeString(s)
	}
	ret.Grow(len(s) + 10)
	for _, r := range s {
		if val, ok := substituteRuneSpace[r]; ok {
			if laststr != "" && val == laststr {
				continue
			}
			if lastrune == '-' && val == "-" {
				continue
			}
			ret.WriteString(val)
			laststr = val
			if val == "-" {
				lastrune = '-'
			} else {
				lastrune = ' '
			}
			continue
		}
		if laststr != "" {
			laststr = ""
		}

		if r < unicode.MaxASCII {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			if _, ok := subRune[r]; !ok {
				if lastrune == '-' {
					continue
				}
				lastrune = '-'
				ret.WriteRune('-')
			} else {
				if lastrune == '-' && r == '-' {
					continue
				}
				lastrune = r
				ret.WriteRune(r)
			}
			continue
		}
		if r > 0xeffff {
			continue
		}

		section := r >> 8   // Chop off the last two hex digits
		position := r % 256 // Last two hex digits
		if tb, ok := table.Tables[section]; ok {
			if len(tb) > int(position) {
				if len(tb[position]) >= 1 {
					if tb[position][0] > unicode.MaxASCII && lastrune != '-' {
						lastrune = '-'
						ret.WriteRune('-')
						continue
					}
				}
				// Transliterations may hold capitals, spaces or punctuation
				// ("Ri ", "E"): apply the ASCII rules so the slug matches the
				// main module's logger.StringToSlug output.
				tr := tb[position]
				for i := range len(tr) {
					c := rune(tr[i])
					if 'A' <= c && c <= 'Z' {
						c += 'a' - 'A'
					}
					if _, ok := subRune[c]; !ok {
						if lastrune != '-' {
							lastrune = '-'
							ret.WriteRune('-')
						}
						continue
					}
					if lastrune == '-' && c == '-' {
						continue
					}
					lastrune = c
					ret.WriteRune(c)
				}
			}
		}
	}
	defer PlBuffer.Put(ret)
	return ret.Bytes()
}

// downloadClient has no overall Client.Timeout: the IMDb dumps are several
// hundred MB and a fixed whole-transfer deadline would either be too short for
// slow links or too long to catch a hang. Connection setup and response headers
// are bounded here; a stalled body is caught by downloadStallTimeout, and
// downloadMaxDuration is a last-resort backstop.
var downloadClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

const (
	downloadStallTimeout = 2 * time.Minute
	downloadMaxDuration  = 3 * time.Hour
)

var errDownloadStalled = errors.New("download stalled: no data received")

// stallReader resets the stall timer every time body data arrives.
type stallReader struct {
	r     io.Reader
	timer *time.Timer
}

func (s *stallReader) Read(b []byte) (int, error) {
	n, err := s.r.Read(b)
	if n > 0 {
		s.timer.Reset(downloadStallTimeout)
	}
	return n, err
}

// downloadFile downloads the content from the given URL
// and saves it to a file in the given directory with the given filename.
// Non-200 responses, truncated bodies and stalled transfers are errors.
func downloadFile(ctx context.Context, saveIn string, fileprefix string, filename string, url string) error {
	ctx, cancelMax := context.WithTimeout(ctx, downloadMaxDuration)
	defer cancelMax()
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	timer := time.AfterFunc(downloadStallTimeout, func() { cancel(errDownloadStalled) })
	defer timer.Stop()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return fmt.Errorf("%w: %w", cause, err)
		}
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected HTTP status %s for %s", resp.Status, url)
	}

	// Create the file
	if len(filename) == 0 {
		filename = path.Base(resp.Request.URL.String())
	}
	if len(fileprefix) >= 1 {
		filename = fileprefix + filename
	}
	out, err := os.Create(path.Join(saveIn, filename))
	if err != nil {
		return err
	}
	_, err = io.Copy(out, &stallReader{r: resp.Body, timer: timer})
	if err == nil {
		err = out.Sync()
	}
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if cause := context.Cause(ctx); cause != nil {
			return fmt.Errorf("%w: %w", cause, err)
		}
		return err
	}
	return nil
}

// gunzip decompresses a gzipped file to a target filename.
// A truncated or corrupt archive (unexpected EOF, checksum mismatch) is an
// error; the partially written target is removed in that case.
func gunzip(source string, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	reader, err := gzip.NewReader(bufio.NewReaderSize(in, 1<<20))
	if err != nil {
		return fmt.Errorf("open gzip %s: %w", source, err)
	}
	defer reader.Close()

	bodyo, err := match(reader)
	if err != nil {
		return fmt.Errorf("read gzip %s: %w", source, err)
	}

	if err := copyfile(target, 0o666, bodyo); err != nil {
		if rmErr := os.Remove(target); rmErr != nil && !os.IsNotExist(rmErr) {
			slog.Warn("Failed to remove partial file", "file", target, "error", rmErr)
		}
		return fmt.Errorf("extract %s: %w", source, err)
	}
	return nil
}

// copyfile copies the contents of the source reader to the file at the provided path.
// It creates any necessary parent directories, truncates any existing file, sets the mode,
// copies the data, syncs, and closes the file. Any error is returned.
func copyfile(path string, mode os.FileMode, src io.Reader) error {
	// We add the execution permission to be able to create files inside it
	err := os.MkdirAll(filepath.Dir(path), mode|os.ModeDir|100)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, src)
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// match reads the first 512 bytes, calls types.Match and returns a reader
// for the whole stream
func match(r *gzip.Reader) (io.Reader, error) {
	buffer := make([]byte, 512)

	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return nil, err
	}
	_, err = filetype.Match(buffer)

	return io.MultiReader(bytes.NewBuffer(buffer[:n]), r), err
}

type poolobj[T any] struct {
	// objs is a channel of type T
	objs chan *T
	// Function will be run on Get() - include here your logic to create the initial object
	constructor func(*T)
	// Function will be run on Put() - include here your logic to reset the object
	destructor func(*T)
}

// newPool creates a new Poolobj initialized with the given parameters.
//
// maxsize specifies the maximum number of objects that can be kept in the
// pool.
//
// initcreate specifies the initial number of objects to create in the pool
// on startup.
//
// constructor, if non-nil, is called whenever a new object needs to be
// created.
//
// destructor, if non-nil, is called whenever an object is removed from
// the pool.
func newPool[T any](maxsize, initcreate int, constructor func(*T), destructor func(*T)) poolobj[T] {
	var a poolobj[T]
	a.constructor = constructor
	a.objs = make(chan *T, maxsize)
	if initcreate > 0 {
		for i := 0; i < initcreate; i++ {
			var bo T
			if a.constructor != nil {
				a.constructor(&bo)
			}
			a.objs <- &bo
		}
	}
	a.destructor = destructor
	return a
}

// Get retrieves an object from the pool or creates a new one if none are
// available. If a constructor was provided, it will be called to initialize
// any newly created objects.
func (p *poolobj[T]) Get() *T {
	if len(p.objs) >= 1 {
		return <-p.objs
	}
	var bo T
	if p.constructor != nil {
		p.constructor(&bo)
	}
	return &bo
}

// Put returns an object to the pool.
// If the pool is not at capacity, it calls the destructor function if provided,
// then sends the object back on the channel.
func (p *poolobj[T]) Put(bo *T) {
	if bo == nil {
		return
	}
	if len(p.objs) < cap(p.objs) {
		if p.destructor != nil {
			p.destructor(bo)
		}
		p.objs <- bo
	}
}

// ProgressReporter interface for reporting processing progress
type ProgressReporter interface {
	Report(stage string, current, total int64, message string)
}

// DefaultProgressReporter provides basic console progress reporting
type DefaultProgressReporter struct {
	logger *slog.Logger
}

func (p *DefaultProgressReporter) Report(stage string, current, total int64, message string) {
	if total > 0 {
		percent := float64(current) / float64(total) * 100
		p.logger.Info("Progress", "stage", stage, "current", current, "total", total, "percent", fmt.Sprintf("%.1f%%", percent), "message", message)
	} else {
		p.logger.Info("Progress", "stage", stage, "current", current, "message", message)
	}
}

// imdbProcessor encapsulates all IMDB processing state and behavior
type imdbProcessor struct {
	ctx      context.Context
	cancel   context.CancelFunc
	logger   *slog.Logger
	progress ProgressReporter
	config   imdbConfig

	// Database connection and transaction
	db *sql.DB
	tx *sql.Tx

	// Processing state
	allowEmptyLang bool
	allLangs       bool

	// Caches and maps
	imdbCache map[uint32]struct{}
	titleMap  map[string]struct{}
	akaMap    map[string]struct{}

	// SQL building buffers for batch processing
	sqlBuild       strings.Builder
	valueArgs      []any
	batchRows      int
	sqlBuildGenre  strings.Builder
	valueArgsGenre []any
	batchRowsGenre int
}

// Run executes the complete IMDB import process
func (p *imdbProcessor) Run(ctx context.Context) error {
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
func (p *imdbProcessor) initDatabase() error {
	// Remove existing temp database
	if err := os.Remove(tempDBPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove existing temp database: %w", err)
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
func (p *imdbProcessor) setupProcessing() error {
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

	// Initialize title map based on indexed types
	p.titleMap = make(map[string]struct{}, len(p.config.Indexedtypes))
	for _, titleType := range p.config.Indexedtypes {
		p.titleMap[titleType] = nilStruct
	}

	p.logger.Info("Processing setup completed",
		"title_types_configured", len(p.config.Indexedtypes),
	)

	return nil
}

// executeSQL executes SQL with direct database writes for speed
func (p *imdbProcessor) executeSQL(query string, last bool, args []any) error {
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
func (p *imdbProcessor) buildSQLBatch(sqlTemplate string, paramTemplate string, isGenre bool) {
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
	if isGenre {
		p.batchRowsGenre++
	} else {
		p.batchRows++
	}
}

// finalize completes the import process with comprehensive cleanup
func (p *imdbProcessor) finalize() error {
	startTime := time.Now()
	p.logger.Info("Starting finalization process")

	// Commit any pending transactions
	if p.tx != nil {
		err := p.tx.Commit()
		p.tx = nil
		if err != nil {
			return fmt.Errorf("failed to commit final transaction: %w", err)
		}
	}

	// Final database optimizations
	if err := p.optimizeFinalDatabase(); err != nil {
		p.logger.Warn("Failed to optimize database", "error", err)
	}

	// Vacuum if using memory mode
	if p.config.UseMemory {
		p.logger.Info("Vacuuming in-memory database to disk")
		if _, err := p.db.Exec("VACUUM INTO ?", tempDBPath); err != nil {
			return fmt.Errorf("failed to vacuum in-memory database to disk: %w", err)
		}
	}

	// Verify import success with detailed statistics
	stats, err := p.gatherImportStats()
	if err != nil {
		return fmt.Errorf("failed to gather import statistics: %w", err)
	}

	if stats.TotalTitles == 0 {
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
func (p *imdbProcessor) gatherImportStats() (*ImportStats, error) {
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
	if stat, err := os.Stat(tempDBPath); err == nil {
		stats.DatabaseSizeMB = float64(stat.Size()) / 1024 / 1024
	}

	return stats, nil
}

// optimizeFinalDatabase applies final optimizations to the database
func (p *imdbProcessor) optimizeFinalDatabase() error {
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

// validateProcessorConfig does minimal validation for performance
func (p *imdbProcessor) validateProcessorConfig() error {
	// Skip validation for maximum performance
	return nil
}

// initImdbdb initializes a SQLite database connection with proper error handling
func (p *imdbProcessor) initImdbdb(dbfile string) (*sql.DB, error) {
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
func (p *imdbProcessor) commitBatch(isGenre bool, force bool) error {
	var builder *strings.Builder
	var valueArgs *[]any
	var rows *int
	var paramLimit int

	if isGenre {
		builder = &p.sqlBuildGenre
		valueArgs = &p.valueArgsGenre
		rows = &p.batchRowsGenre
		paramLimit = SQLParamBatch * 2 // 2 params per genre record (tconst, genre)
	} else {
		builder = &p.sqlBuild
		valueArgs = &p.valueArgs
		rows = &p.batchRows
		paramLimit = SQLParamBatch * 10 // 10 params per title/aka/rating record (worst case)
	}

	// Use SQLite parameter limit for batching
	if len(*valueArgs) >= paramLimit || force {
		if builder.Len() > 0 {
			err := p.executeSQL(builder.String(), force, *valueArgs)
			lost := *rows
			// Always reset: a failed batch must not be re-sent with every later flush.
			builder.Reset()
			*valueArgs = (*valueArgs)[:0]
			*rows = 0
			if err != nil {
				return fmt.Errorf("batch insert failed, %d rows lost: %w", lost, err)
			}
		}
	}

	return nil
}

// downloadIMDBFiles downloads the required IMDB dataset files
func (p *imdbProcessor) downloadIMDBFiles(ctx context.Context) error {
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

		// A .tsv left over from an earlier run must never be imported in place
		// of a failed download/extract.
		outputName := strings.TrimSuffix(file.name, ".gz")
		if err := os.Remove(outputName); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale %s: %w", outputName, err)
		}

		err := downloadFile(ctx, "./", "", file.name, file.url)
		if err == nil {
			err = gunzip("./"+file.name, outputName)
		}

		// Remove compressed file
		if rmErr := os.Remove("./" + file.name); rmErr != nil && !os.IsNotExist(rmErr) {
			p.logger.Warn("Failed to remove compressed file", "file", file.name, "error", rmErr)
		}
		if err != nil {
			return fmt.Errorf("failed to download/extract %s: %w", file.name, err)
		}
	}

	return nil
}

// createInMemorySchema creates optimized database schema in memory
func (p *imdbProcessor) createInMemorySchema() error {
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

		// Performance indexes for titles (idx_imdb_* mirror schema/imdbdb migrations)
		`CREATE INDEX "idx_titles_type" ON "imdb_titles" ("title_type");`,
		`CREATE INDEX "idx_imdb_titles_start_year" ON "imdb_titles" ("start_year");`,
		`CREATE INDEX "idx_imdb_titles_slug" ON "imdb_titles" ("slug");`,
		`CREATE INDEX "idx_titles_runtime" ON "imdb_titles" ("runtime_minutes");`,
		`CREATE INDEX "idx_imdb_titles_primary_title" ON "imdb_titles" ("primary_title");`,
		`CREATE INDEX "idx_imdb_titles_original_title" ON "imdb_titles" ("original_title");`,
		`CREATE INDEX "idx_imdb_titles_primary_original_slug" ON "imdb_titles" ("primary_title", "original_title", "slug");`,

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

		// Performance indexes for akas (idx_imdb_* mirror schema/imdbdb migrations)
		`CREATE INDEX "idx_imdb_akas_tconst" ON "imdb_akas" ("tconst");`,
		`CREATE INDEX "idx_akas_region" ON "imdb_akas" ("region");`,
		`CREATE INDEX "idx_akas_language" ON "imdb_akas" ("language");`,
		`CREATE INDEX "idx_imdb_akas_slug" ON "imdb_akas" ("slug");`,
		`CREATE INDEX "idx_imdb_akas_title" ON "imdb_akas" ("title");`,
		`CREATE INDEX "idx_imdb_akas_title_slug" ON "imdb_akas" ("title", "slug");`,

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

	p.logger.Info("Created optimized in-memory schema", "tables", 4, "indexes", 18)
	return nil
}

// runMigrations runs database migrations
func (p *imdbProcessor) runMigrations() error {
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
func (p *imdbProcessor) processTitles(ctx context.Context) error {
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
func (p *imdbProcessor) processAkas(ctx context.Context) error {
	p.logger.Info("Processing akas dataset")

	p.initAkaFilter()

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

// initAkaFilter builds the region filter from indexed_languages. An empty list
// imports all regions; an empty string or \N in the list matches akas without
// a region, which IMDb writes as \N.
func (p *imdbProcessor) initAkaFilter() {
	p.akaMap = make(map[string]struct{}, len(p.config.Indexedlanguages))
	p.allowEmptyLang = false
	p.allLangs = len(p.config.Indexedlanguages) == 0
	for _, lang := range p.config.Indexedlanguages {
		if lang == "" || lang == "\\N" {
			p.allowEmptyLang = true
		} else {
			p.akaMap[lang] = nilStruct
		}
	}
}

// processRatings processes the ratings dataset
func (p *imdbProcessor) processRatings(ctx context.Context) error {
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

// tsvReader reads IMDb dataset files. They are plain tab-separated values
// without any quoting, so encoding/csv (which treats a leading '"' as a quoted
// field and merges following lines) must not be used.
type tsvReader struct {
	sc   *bufio.Scanner
	line int
}

func newTSVReader(r io.Reader) *tsvReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), tsvMaxLineSize)
	return &tsvReader{sc: sc}
}

// Read returns the fields of the next line, io.EOF at the end of input, or the
// underlying read error (including a line longer than tsvMaxLineSize).
func (t *tsvReader) Read() ([]string, error) {
	if !t.sc.Scan() {
		if err := t.sc.Err(); err != nil {
			return nil, fmt.Errorf("read error after line %d: %w", t.line, err)
		}
		return nil, io.EOF
	}
	t.line++
	return strings.Split(strings.TrimSuffix(t.sc.Text(), "\r"), "\t"), nil
}

// readHeader consumes the header line.
func (t *tsvReader) readHeader() error {
	if _, err := t.Read(); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("failed to read header: file is empty")
		}
		return fmt.Errorf("failed to read header: %w", err)
	}
	return nil
}

// malformedLines counts and logs dataset lines with an unexpected field count.
type malformedLines struct {
	logger *slog.Logger
	file   string
	count  int
}

const malformedLogLimit = 20

func (m *malformedLines) add(line, fields, want int) {
	m.count++
	if m.count <= malformedLogLimit {
		m.logger.Warn("Skipping malformed line", "file", m.file, "line", line, "fields", fields, "expected", want)
	}
}

func (m *malformedLines) report() {
	if m.count > 0 {
		m.logger.Warn("Malformed lines skipped", "file", m.file, "count", m.count)
	}
}

// processTitlesCSV processes title records from a TSV file
func (p *imdbProcessor) processTitlesCSV(ctx context.Context, file io.Reader) error {
	reader := newTSVReader(file)
	if err := reader.readHeader(); err != nil {
		return err
	}
	malformed := malformedLines{logger: p.logger, file: "title.basics.tsv"}

	// Initialize performance tracking
	startTime := time.Now()
	rowCount := 0
	processedCount := 0
	errorCount := 0
	lastReportTime := startTime

	p.logger.Info("Starting title processing")

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
			return err
		}

		rowCount++

		if len(record) != 9 {
			errorCount++
			malformed.add(reader.line, len(record), 9)
			continue
		}

		if err := p.processTitleRecord(record); err != nil {
			return fmt.Errorf("line %d: %w", reader.line, err)
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

	malformed.report()
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
func (p *imdbProcessor) processTitleRecord(record []string) error {
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
func (p *imdbProcessor) processGenresBatch(tconst, genres string) error {
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

// processAkasCSV processes alternative title records from a TSV file
func (p *imdbProcessor) processAkasCSV(ctx context.Context, file io.Reader) error {
	reader := newTSVReader(file)
	if err := reader.readHeader(); err != nil {
		return err
	}
	malformed := malformedLines{logger: p.logger, file: "title.akas.tsv"}
	defer malformed.report()

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
			return err
		}

		if len(record) != 8 {
			malformed.add(reader.line, len(record), 8)
			continue
		}

		if err := p.processAkaRecord(record); err != nil {
			return fmt.Errorf("line %d: %w", reader.line, err)
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
func (p *imdbProcessor) processAkaRecord(record []string) error {
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

	if p.config.Indexfull && len(record) >= 8 {
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
func (p *imdbProcessor) shouldIndexLanguage(region string) bool {
	if p.allLangs {
		return true
	}
	if region == "" || region == "\\N" {
		return p.allowEmptyLang
	}
	_, ok := p.akaMap[region]
	return ok
}

// processRatingsCSV processes rating records from a TSV file
func (p *imdbProcessor) processRatingsCSV(ctx context.Context, file io.Reader) error {
	reader := newTSVReader(file)
	if err := reader.readHeader(); err != nil {
		return err
	}
	malformed := malformedLines{logger: p.logger, file: "title.ratings.tsv"}
	defer malformed.report()

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
			return err
		}

		if len(record) != 3 {
			malformed.add(reader.line, len(record), 3)
			continue
		}

		if err := p.processRatingRecord(record); err != nil {
			return fmt.Errorf("line %d: %w", reader.line, err)
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
func (p *imdbProcessor) processRatingRecord(record []string) error {
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

// Close cleans up resources used by the processor
func (p *imdbProcessor) Close() error {
	p.cancel()

	var errs []error

	// Only reached with an open transaction on a failure path (finalize commits
	// and clears it); the temp database is discarded then, so roll back.
	if p.tx != nil {
		if err := p.tx.Rollback(); err != nil {
			errs = append(errs, err)
		}
		p.tx = nil
	}

	// Close database
	if p.db != nil {
		if err := p.db.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Clear maps to help GC
	clear(p.imdbCache)
	clear(p.titleMap)
	clear(p.akaMap)
	p.imdbCache = nil
	p.titleMap = nil
	p.akaMap = nil

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errs)
	}
	return nil
}

// NewimdbProcessor creates a new IMDB processor with the given configuration
func newimdbProcessor(ctx context.Context, config imdbConfig, logger *slog.Logger) (*imdbProcessor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	p := &imdbProcessor{
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		progress: &DefaultProgressReporter{logger: logger},
		config:   config,

		imdbCache:      make(map[uint32]struct{}, config.ImdbIDSize),
		valueArgs:      make([]any, 0, ValueArgsCapInit),
		valueArgsGenre: make([]any, 0, ValueArgsCapInit),
	}

	// Pre-allocate string builders for batch SQL generation
	p.sqlBuild.Grow(1000000)     // 1MB should be enough
	p.sqlBuildGenre.Grow(500000) // 0.5MB for genres

	return p, nil
}
