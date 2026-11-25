package api

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// The log files the analysis reads, as written by the logger package:
// zerolog JSON, one entry per line, rotated by lumberjack into
// <name>-<timestamp>.log (or .log.gz with log_compress).
const (
	appLogName   = "downloader"
	errorLogName = "errors"
	webLogName   = "weblogs"
)

// maxAnalyzedEntries bounds the Max Lines input; the newest entries are kept
// in memory until the scan ends.
const maxAnalyzedEntries = 200000

// slowRequestMs is the response time from which a web request counts as slow.
const slowRequestMs = 1000

// logTimeRanges maps the page's Time Range options to durations.
var logTimeRanges = map[string]time.Duration{
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"12h": 12 * time.Hour,
	"24h": 24 * time.Hour,
	"3d":  3 * 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

// logLevelRank orders zerolog's level names; the page's Log Level is a minimum.
var logLevelRank = map[string]int{
	"trace": 0, "debug": 1, "info": 2, "warn": 3, "error": 4, "fatal": 5, "panic": 6,
}

// CountedItem is a grouped log entry (error message, endpoint) and how often it occurred.
type CountedItem struct {
	Name  string
	Count int
	AvgMs float64 // endpoints only
	MaxMs float64 // endpoints only
}

// LogAnalysisResults holds the results of log file analysis.
type LogAnalysisResults struct {
	Files          []string
	First, Last    time.Time // time span of the analyzed entries
	TotalEntries   int64
	Unparsed       int64 // lines that aren't JSON log entries
	Truncated      bool  // more entries matched than Max Lines; the newest were kept
	ErrorCount     int64 // error, fatal and panic
	WarningCount   int64
	InfoCount      int64
	DebugCount     int64
	TopErrors      []CountedItem // warnings and errors grouped by message and error text
	DistinctErrors int

	// Web requests from weblogs.log (same time range and Max Lines).
	WebFiles         []string
	Requests         int64
	AvgResponseMs    float64
	MaxResponseMs    float64
	SlowestRequest   string
	SlowRequests     int64 // at least slowRequestMs
	ClientErrors     int64 // 4xx
	ServerErrors     int64 // 5xx
	TopEndpoints     []CountedItem
	SlowestEndpoints []CountedItem
}

// logEntry holds the fields of a log line that the analysis uses.
type logEntry struct {
	Level   string          `json:"level"`
	Time    string          `json:"time"`
	Message string          `json:"message"`
	Error   json.RawMessage `json:"error"`
	Method  string          `json:"method"`
	Path    string          `json:"path"`
	Elapsed float64         `json:"elapsed_ms"`
	Status  int             `json:"status_code"`
	at      time.Time
}

// getLogDir returns the directory holding the application's log files.
func getLogDir() (string, error) {
	for _, dir := range []string{"logs", "../logs", "../../logs"} {
		if _, err := os.Stat(filepath.Join(dir, appLogName+".log")); err == nil {
			return filepath.Abs(dir)
		}
	}

	return "", errors.New("no logs/downloader.log found")
}

// logFiles returns the current and rotated files of one log, oldest first.
// Rotated files last written before since can't hold entries in range and
// are skipped.
func logFiles(dir, name string, since time.Time) []string {
	rotated, _ := filepath.Glob(filepath.Join(dir, name+"-*.log"))
	gz, _ := filepath.Glob(filepath.Join(dir, name+"-*.log.gz"))
	rotated = append(rotated, gz...)

	// lumberjack's backup names end in the rotation time, so they sort by age.
	slices.Sort(rotated)

	files := make([]string, 0, len(rotated)+1)
	for _, f := range rotated {
		if info, err := os.Stat(f); err == nil && !info.ModTime().Before(since) {
			files = append(files, f)
		}
	}

	if current := filepath.Join(dir, name+".log"); fileExists(current) {
		files = append(files, current)
	}

	return files
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseLogTime parses a log entry's time with the logger's configured format.
func parseLogTime(s string) (time.Time, bool) {
	for _, layout := range []string{logger.GetTimeFormat(), time.RFC3339Nano} {
		if layout == "" {
			continue
		}

		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

// readLogEntries reads the entries of files that are at or after since (a zero
// since keeps all) and at or above minLevel. Only the newest maxEntries are
// returned; truncated reports whether older ones were dropped. Entries without
// a parsable time are kept, as they can't be placed outside the range.
func readLogEntries(files []string, since time.Time, minLevel int, maxEntries int) (entries []logEntry, unparsed int64, truncated bool, err error) {
	ring := make([]logEntry, 0, min(maxEntries, 4096))
	next := 0 // ring write position once it is full

	for _, file := range files {
		if err := scanLogFile(file, func(line []byte) {
			var e logEntry
			if jerr := json.Unmarshal(line, &e); jerr != nil {
				var syntaxErr *json.SyntaxError
				if errors.As(jerr, &syntaxErr) || e.Level == "" {
					unparsed++
					return
				}
				// A field of an unexpected type; the other fields are filled.
			}

			if rank, ok := logLevelRank[e.Level]; ok && rank < minLevel {
				return
			}

			if t, ok := parseLogTime(e.Time); ok {
				if t.Before(since) {
					return
				}

				e.at = t
			}

			if len(ring) < maxEntries {
				ring = append(ring, e)
				return
			}

			truncated = true
			ring[next] = e
			next = (next + 1) % maxEntries
		}); err != nil {
			return nil, unparsed, truncated, err
		}
	}

	// Restore chronological order: the oldest kept entry is at next.
	return append(ring[next:], ring[:next]...), unparsed, truncated, nil
}

func scanLogFile(path string, fn func([]byte)) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("open %s: %w", filepath.Base(path), err)
		}
		defer gz.Close()

		r = gz
	}

	scanner := bufio.NewScanner(r)
	// Entries with long error texts (HTML error pages) exceed the 64 KB default.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		fn(scanner.Bytes())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}

	return nil
}

var (
	digitsRe     = regexp.MustCompile(`\d+`)
	pathNumberRe = regexp.MustCompile(`/\d+(/|$)`)
)

// errorGroup is the key warnings and errors are grouped by: the message plus
// the first line of the error text, with numbers masked so ids and counts
// don't split one problem into many groups.
func errorGroup(e *logEntry) string {
	key := e.Message
	if key == "" {
		key = "(no message)"
	}

	if len(e.Error) == 0 || string(e.Error) == "null" {
		return key
	}

	var text string
	if json.Unmarshal(e.Error, &text) != nil {
		text = string(e.Error)
	}

	text, _, _ = strings.Cut(text, "\n")
	text = digitsRe.ReplaceAllString(strings.TrimSpace(text), "#")

	if text == "" {
		return key
	}

	if len(text) > 120 {
		text = text[:120] + "…"
	}

	return key + ": " + text
}

// endpointGroup is a request's method and path with the query removed and
// numeric path segments shown as :id.
func endpointGroup(e *logEntry) string {
	path, _, _ := strings.Cut(e.Path, "?")
	for pathNumberRe.MatchString(path) {
		path = pathNumberRe.ReplaceAllString(path, "/:id$1")
	}

	return strings.TrimSpace(e.Method + " " + path)
}

// topCounted returns up to n items with the highest count.
func topCounted(counts map[string]*CountedItem, n int, less func(a, b *CountedItem) int) []CountedItem {
	items := make([]*CountedItem, 0, len(counts))
	for _, it := range counts {
		items = append(items, it)
	}

	slices.SortFunc(items, less)

	out := make([]CountedItem, 0, min(n, len(items)))
	for _, it := range items[:min(n, len(items))] {
		out = append(out, *it)
	}

	return out
}

func byCountDesc(a, b *CountedItem) int {
	if a.Count != b.Count {
		return b.Count - a.Count
	}

	return strings.Compare(a.Name, b.Name)
}

// performLogAnalysis analyzes the application log (logName "downloader" or
// "errors") and, for performance metrics and access patterns, weblogs.log.
func performLogAnalysis(
	logDir, logName, timeRange, logLevel string,
	maxLines int64,
	errorPattern, performanceMetrics, accessPattern bool,
	now time.Time,
) (*LogAnalysisResults, error) {
	var since time.Time
	if d, ok := logTimeRanges[timeRange]; ok {
		since = now.Add(-d)
	}

	minLevel := logLevelRank[strings.ToLower(logLevel)]
	if strings.EqualFold(logLevel, "warning") {
		minLevel = logLevelRank["warn"]
	}

	maxEntries := int(min(max(maxLines, 1), maxAnalyzedEntries))

	results := &LogAnalysisResults{Files: logFiles(logDir, logName, since)}
	if len(results.Files) == 0 {
		return nil, fmt.Errorf("no %s log files in %s", logName, logDir)
	}

	entries, unparsed, truncated, err := readLogEntries(results.Files, since, minLevel, maxEntries)
	if err != nil {
		return nil, err
	}

	results.Unparsed = unparsed
	results.Truncated = truncated
	results.TotalEntries = int64(len(entries))

	errorsByGroup := make(map[string]*CountedItem)

	for i := range entries {
		e := &entries[i]
		if !e.at.IsZero() {
			if results.First.IsZero() || e.at.Before(results.First) {
				results.First = e.at
			}

			if e.at.After(results.Last) {
				results.Last = e.at
			}
		}

		switch e.Level {
		case "error", "fatal", "panic":
			results.ErrorCount++
		case "warn":
			results.WarningCount++
		case "info":
			results.InfoCount++
		case "debug", "trace":
			results.DebugCount++
		}

		if errorPattern && logLevelRank[e.Level] >= logLevelRank["warn"] {
			key := errorGroup(e)
			if it := errorsByGroup[key]; it != nil {
				it.Count++
			} else {
				errorsByGroup[key] = &CountedItem{Name: key, Count: 1}
			}
		}
	}

	results.DistinctErrors = len(errorsByGroup)
	results.TopErrors = topCounted(errorsByGroup, 10, byCountDesc)

	if performanceMetrics || accessPattern {
		if err := analyzeWebLog(results, logDir, since, maxEntries); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// analyzeWebLog fills the request statistics from weblogs.log.
func analyzeWebLog(results *LogAnalysisResults, logDir string, since time.Time, maxEntries int) error {
	results.WebFiles = logFiles(logDir, webLogName, since)
	if len(results.WebFiles) == 0 {
		return nil
	}

	requests, _, _, err := readLogEntries(results.WebFiles, since, 0, maxEntries)
	if err != nil {
		return err
	}

	byEndpoint := make(map[string]*CountedItem)

	var totalMs float64

	for i := range requests {
		e := &requests[i]
		if e.Path == "" {
			continue
		}

		results.Requests++
		totalMs += e.Elapsed

		if e.Elapsed > results.MaxResponseMs {
			results.MaxResponseMs = e.Elapsed
			results.SlowestRequest = e.Method + " " + e.Path
		}

		if e.Elapsed >= slowRequestMs {
			results.SlowRequests++
		}

		switch {
		case e.Status >= 500:
			results.ServerErrors++
		case e.Status >= 400:
			results.ClientErrors++
		}

		key := endpointGroup(e)

		it := byEndpoint[key]
		if it == nil {
			it = &CountedItem{Name: key}
			byEndpoint[key] = it
		}

		it.AvgMs = (it.AvgMs*float64(it.Count) + e.Elapsed) / float64(it.Count+1)
		it.Count++
		it.MaxMs = max(it.MaxMs, e.Elapsed)
	}

	if results.Requests > 0 {
		results.AvgResponseMs = totalMs / float64(results.Requests)
	}

	results.TopEndpoints = topCounted(byEndpoint, 10, byCountDesc)
	results.SlowestEndpoints = topCounted(byEndpoint, 5, func(a, b *CountedItem) int {
		switch {
		case a.AvgMs > b.AvgMs:
			return -1
		case a.AvgMs < b.AvgMs:
			return 1
		}

		return strings.Compare(a.Name, b.Name)
	})

	return nil
}
