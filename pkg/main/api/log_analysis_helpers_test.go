package api

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeLogFile(t *testing.T, path string, modTime time.Time, lines ...string) {
	t.Helper()

	content := strings.Join(lines, "\n") + "\n"

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	if strings.HasSuffix(path, ".gz") {
		gz := gzip.NewWriter(f)
		if _, err := gz.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}

		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	} else if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}
}

func logLine(at time.Time, level, message, errText string) string {
	line := fmt.Sprintf(`{"level":%q,"time":%q,"message":%q`, level, at.Format(time.RFC3339), message)
	if errText != "" {
		line += fmt.Sprintf(`,"error":%q`, errText)
	}

	return line + "}"
}

func webLine(at time.Time, method, path string, elapsed float64, status int) string {
	return fmt.Sprintf(`{"level":"info","method":%q,"path":%q,"time":%q,"elapsed_ms":%g,"status_code":%d,"message":"Request"}`,
		method, path, at.Format(time.RFC3339), elapsed, status)
}

func TestPerformLogAnalysis(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }

	// Rotated two days ago: outside a 24h range, so the file is not read.
	writeLogFile(t, filepath.Join(dir, "downloader-2026-11-10T12-00-00.000.log"), ago(48*time.Hour),
		logLine(ago(49*time.Hour), "error", "old", ""))
	// Rotated (compressed) five hours ago, holding entries from 6h and 5h ago.
	writeLogFile(t, filepath.Join(dir, "downloader-2026-11-12T07-00-00.000.log.gz"), ago(5*time.Hour),
		logLine(ago(6*time.Hour), "error", "exec", "disk I/O error (8714)"),
		logLine(ago(5*time.Hour), "debug", "Skipped", ""))
	writeLogFile(t, filepath.Join(dir, "downloader.log"), now,
		logLine(ago(30*time.Hour), "error", "too old", ""),
		logLine(ago(3*time.Hour), "error", "exec", "disk I/O error (522)"),
		logLine(ago(2*time.Hour), "warn", "skipped - disallowed", ""),
		logLine(ago(2*time.Hour), "info", "a failed search mentioned in an info line", ""),
		"not json",
		logLine(ago(time.Hour), "error", "Media Search Failed", "dbalbum 42 not found\nsecond line"))
	writeLogFile(t, filepath.Join(dir, "weblogs.log"), now,
		webLine(ago(2*time.Hour), "GET", "/api/series/12/episodes?apikey=x", 40, 200),
		webLine(ago(time.Hour), "GET", "/api/series/7/episodes", 60, 200),
		webLine(ago(time.Hour), "POST", "/api/admin/log-analysis", 1500, 500),
		webLine(ago(time.Hour), "GET", "/api/missing", 10, 404))

	r, err := performLogAnalysis(dir, "downloader", "24h", "DEBUG", 1000, true, true, true, now)
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Files) != 2 || r.TotalEntries != 6 || r.Unparsed != 1 {
		t.Fatalf("files %v, entries %d, unparsed %d; want 2 files, 6 entries, 1 unparsed", r.Files, r.TotalEntries, r.Unparsed)
	}

	if r.ErrorCount != 3 || r.WarningCount != 1 || r.InfoCount != 1 || r.DebugCount != 1 {
		t.Errorf("levels e/w/i/d = %d/%d/%d/%d, want 3/1/1/1 (by level field, not by words in the line)",
			r.ErrorCount, r.WarningCount, r.InfoCount, r.DebugCount)
	}

	if !r.First.Equal(ago(6*time.Hour)) || !r.Last.Equal(ago(time.Hour)) {
		t.Errorf("span %v .. %v", r.First, r.Last)
	}

	if len(r.TopErrors) != 3 || r.TopErrors[0] != (CountedItem{Name: "exec: disk I/O error (#)", Count: 2}) ||
		r.TopErrors[1].Name != "Media Search Failed: dbalbum # not found" {
		t.Errorf("top errors = %+v", r.TopErrors)
	}

	if r.Requests != 4 || r.ServerErrors != 1 || r.ClientErrors != 1 || r.SlowRequests != 1 ||
		r.MaxResponseMs != 1500 || r.SlowestRequest != "POST /api/admin/log-analysis" {
		t.Errorf("web = %+v", r)
	}

	if len(r.TopEndpoints) == 0 || r.TopEndpoints[0] != (CountedItem{Name: "GET /api/series/:id/episodes", Count: 2, AvgMs: 50, MaxMs: 60}) {
		t.Errorf("top endpoints = %+v", r.TopEndpoints)
	}

	// Minimum level WARN, newest 2 entries only.
	r, err = performLogAnalysis(dir, "downloader", "24h", "WARN", 2, false, false, false, now)
	if err != nil {
		t.Fatal(err)
	}

	if r.TotalEntries != 2 || !r.Truncated || r.ErrorCount != 1 || r.WarningCount != 1 || !r.First.Equal(ago(2*time.Hour)) {
		t.Errorf("WARN, max 2: entries %d, truncated %v, errors %d, warnings %d, first %v; want the newest warning and error",
			r.TotalEntries, r.Truncated, r.ErrorCount, r.WarningCount, r.First)
	}

	if r.Requests != 0 || r.WebFiles != nil {
		t.Errorf("web log read although neither performance nor access was selected")
	}

	// "all" reads every file, including the old rotated one.
	r, err = performLogAnalysis(dir, "downloader", "all", "ERROR", 1000, false, false, false, now)
	if err != nil {
		t.Fatal(err)
	}

	if len(r.Files) != 3 || r.ErrorCount != 5 {
		t.Errorf("all: files %d, errors %d; want 3, 5", len(r.Files), r.ErrorCount)
	}

	if _, err := performLogAnalysis(dir, "errors", "24h", "ERROR", 10, false, false, false, now); err == nil {
		t.Error("missing errors.log: no error")
	}
}
