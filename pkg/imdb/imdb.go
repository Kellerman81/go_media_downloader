package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	_ "net/http/pprof"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mozillazg/go-unidecode/table"

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
	LoopSize         int      `toml:"loop_size"`
	UseMemory        bool     `toml:"use_memory"`
	UseCache         bool     `toml:"use_cache"`
}

// Validate validates the configuration and sets defaults
func (c *imdbConfig) Validate() error {
	if c.ImdbIDSize <= 0 {
		c.ImdbIDSize = DefaultImdbIDSize
	}
	if c.LoopSize <= 0 {
		c.LoopSize = SQLBatchSize // Set to actual batch size used
	}
	if c.ImdbIDSize > MaxImdbIDSize {
		return fmt.Errorf("imdbid_size %d exceeds maximum %d", c.ImdbIDSize, MaxImdbIDSize)
	}
	if c.LoopSize > MaxLoopSize {
		return fmt.Errorf("loop_size %d exceeds maximum %d", c.LoopSize, MaxLoopSize)
	}
	return nil
}

const (
	configfile = "./config/config.toml"

	// Default configuration values
	DefaultImdbIDSize = 1200000
	DefaultLoopSize   = SQLBatchSize // Match the actual batch size

	// Maximum values for safety
	MaxImdbIDSize = 20000000
	MaxLoopSize   = 1000000

	// Optimized batch processing constants
	SQLBatchSize     = 400000 // Large but reasonable batches for commit frequency
	SQLParamBatch    = 99     // Max records per SQL batch (SQLite limit: 999 params, titles have 10 params: 99*10=990)
	ValueArgsCapInit = 50000  // Pre-allocated buffers
	SQLCacheSize     = 1000

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
	PlBuffer  = NewPool(BufferPoolSize, 0, nil, func(b *bytes.Buffer) { b.Reset() })
	nilStruct = struct{}{}
)

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

// IMDBProcessor encapsulates all IMDB processing state and behavior
type IMDBProcessor struct {
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

	// Caches and maps
	imdbCache map[uint32]struct{}
	titleMap  map[string]struct{}
	akaMap    map[string]struct{}
	sqlCache  map[string]*sql.Stmt

	// SQL building buffers for batch processing
	sqlBuild       strings.Builder
	valueArgs      []any
	sqlBuildGenre  strings.Builder
	valueArgsGenre []any

	// Prepared statements
	stmtShortTitles  *sql.Stmt
	stmtLongTitles   *sql.Stmt
	stmtGenre        *sql.Stmt
	stmtShortAkas    *sql.Stmt
	stmtLongAkas     *sql.Stmt
	stmtShortRatings *sql.Stmt
}

// NewIMDBProcessor creates a new IMDB processor with the given configuration
func NewIMDBProcessor(ctx context.Context, config imdbConfig, logger *slog.Logger) (*IMDBProcessor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)

	p := &IMDBProcessor{
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger,
		progress: &DefaultProgressReporter{logger: logger},
		config:   config,

		imdbCache:      make(map[uint32]struct{}, config.ImdbIDSize),
		sqlCache:       make(map[string]*sql.Stmt, SQLCacheSize),
		valueArgs:      make([]any, 0, ValueArgsCapInit),
		valueArgsGenre: make([]any, 0, ValueArgsCapInit),
	}

	// Pre-allocate string builders for batch SQL generation
	p.sqlBuild.Grow(1000000)     // 1MB should be enough
	p.sqlBuildGenre.Grow(500000) // 0.5MB for genres

	return p, nil
}

// Close cleans up resources used by the processor
func (p *IMDBProcessor) Close() error {
	p.cancel()

	var errs []error

	// Close prepared statements
	if p.stmtShortTitles != nil {
		if err := p.stmtShortTitles.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.stmtLongTitles != nil {
		if err := p.stmtLongTitles.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.stmtGenre != nil {
		if err := p.stmtGenre.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.stmtShortAkas != nil {
		if err := p.stmtShortAkas.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.stmtLongAkas != nil {
		if err := p.stmtLongAkas.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if p.stmtShortRatings != nil {
		if err := p.stmtShortRatings.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	// Close cached statements
	for _, stmt := range p.sqlCache {
		if stmt != nil {
			if err := stmt.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Commit or rollback any active transaction
	if p.tx != nil {
		if err := p.tx.Commit(); err != nil {
			p.logger.Warn("Failed to commit final transaction", "error", err)
			if rollbackErr := p.tx.Rollback(); rollbackErr != nil {
				errs = append(errs, rollbackErr)
			}
		}
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
	clear(p.sqlCache)
	p.imdbCache = nil
	p.titleMap = nil
	p.akaMap = nil
	p.sqlCache = nil

	if len(errs) > 0 {
		return fmt.Errorf("multiple errors during close: %v", errs)
	}
	return nil
}

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
		'Ä':  "Ae",
		'ö':  "oe",
		'Ö':  "Oe",
		'ü':  "ue",
		'Ü':  "Ue",
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
	processor, err := NewIMDBProcessor(ctx, cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create processor: %w", err)
	}
	defer func() {
		if closeErr := processor.Close(); closeErr != nil {
			logger.Error("Error closing processor", "error", closeErr)
		}
	}()

	// Run the import process
	if err := processor.Run(ctx); err != nil {
		return fmt.Errorf("import process failed: %w", err)
	}

	logger.Info("IMDB import completed successfully")
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
				if lastrune == '-' && tb[position] == "-" {
					continue
				}
				ret.WriteString(tb[position])
			}
		}
	}
	defer PlBuffer.Put(ret)
	return ret.Bytes()
}

// downloadFile downloads the content from the given URL
// and saves it to a file in the given directory with the given filename.
// It returns any error encountered.
func downloadFile(saveIn string, fileprefix string, filename string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	if len(filename) == 0 {
		filename = path.Base(resp.Request.URL.String())
	}
	var filepath string
	if len(fileprefix) >= 1 {
		filename = fileprefix + filename
	}
	filepath = path.Join(saveIn, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	out.Sync()
	return err
}

// RemoveFile removes the file at the given path.
// It returns an error if the file could not be removed.
func RemoveFile(file string) error {
	var err error
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		err := os.Remove(file)
		if err != nil {
			fmt.Println("File could not be removed: ", file, " Error: ", err)
		} else {
			fmt.Println("File removed: ", file)
		}
	} else {
		fmt.Println("File not found: ", file)
	}
	return err
}

// gunzip decompresses a gzipped file to a target filename.
// It takes a gzipped source filename and a target filename to decompress to.
// It handles reading the gzipped data, decompressing it, and writing the
// decompressed data to the target file.
func gunzip(source string, target string) {
	data, _ := os.ReadFile(source)
	body := bytes.NewBuffer(data)
	reader, err := gzip.NewReader(body)
	if err != nil {
		fmt.Println("err1. ", err)
		return
	}
	defer reader.Close()

	bodyo, err := match(reader)
	if err != nil {
		fmt.Println("err2. ", err)
		return
	}

	err = copyfile(target, 0o666, bodyo)
	if err != nil {
		fmt.Println("err3. ", err)
	}
}

// copyfile copies the contents of the source reader to the file at the provided path.
// It creates any necessary parent directories, truncates any existing file, sets the mode,
// copies the data, syncs, and closes the file. Any errors are printed and returned.
func copyfile(path string, mode os.FileMode, src io.Reader) error {
	// We add the execution permission to be able to create files inside it
	err := os.MkdirAll(filepath.Dir(path), mode|os.ModeDir|100)
	if err != nil {
		fmt.Println("err4. ", err)
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		fmt.Println("err5. ", err)
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, src)
	file.Sync()
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

type Poolobj[T any] struct {
	// objs is a channel of type T
	objs chan *T
	// Function will be run on Get() - include here your logic to create the initial object
	constructor func(*T)
	// Function will be run on Put() - include here your logic to reset the object
	destructor func(*T)
}

// NewPool creates a new Poolobj initialized with the given parameters.
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
func NewPool[T any](maxsize, initcreate int, constructor func(*T), destructor func(*T)) Poolobj[T] {
	var a Poolobj[T]
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
func (p *Poolobj[T]) Get() *T {
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
func (p *Poolobj[T]) Put(bo *T) {
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
