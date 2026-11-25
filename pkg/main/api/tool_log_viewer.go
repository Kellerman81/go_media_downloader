package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderLogViewerPage renders a page for viewing log files.
func renderLogViewerPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-file-text header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Log Viewer")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Real-time application logs and system events monitoring. Filter by log level and automatically refresh to stay up-to-date with system activity.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("logForm"),
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Log Configuration")),

					renderFormGroup("log", map[string]string{
						"LineCount": "Number of lines to display from the end of the log file",
					}, map[string]string{
						"LineCount": "Number of Lines",
					}, "LineCount", "number", "100", nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Filter Options")),

					renderFormGroup("log", map[string]string{
						"LogLevel": "Filter logs by level (leave empty for all)",
					}, map[string]string{
						"LogLevel": "Log Level Filter",
					}, "LogLevel", "select", "", map[string][]string{
						"options": {"", "error", "warn", "info", "debug"},
					}),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Load Log"),
					html.Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					html.I(html.Class("fas fa-sync-alt me-1")),
				),
				html.Button(
					html.Class("btn btn-info ml-2"),
					gomponents.Text("Auto-Refresh"),
					html.Type("button"),
					hx.Target("#logResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/logviewer"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#logForm"),
					hx.Trigger("every 5s"),
					html.ID("autoRefreshBtn"),
					html.I(html.Class("fas fa-play me-1")),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('logForm').reset(); document.getElementById('logResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("logResults"),
			html.Class("mt-4"),
			html.Style(
				"min-height: 400px; background: #1e1e1e; color: #f8f8f2; font-family: 'Courier New', monospace; padding: 1rem; overflow-y: auto; max-height: 600px; border-radius: 8px;",
			),
			html.P(
				html.Class("text-center text-muted"),
				gomponents.Text("Click 'Load Log' to view application logs..."),
			),
		),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Usage"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Log Viewer Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Real-time log monitoring with filtering capabilities"),
				),
				html.Ul(
					html.Class("mb-3 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("📂 Logs are read from the 'logs/downloader.log' file"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"🔄 Lines are displayed in reverse chronological order (newest first)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"🎯 Use the log level filter to show only specific types of messages",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("⚡ Auto-refresh updates the display every 5 seconds"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("⏱️ Large log files may take a moment to load"),
					),
				),
			),
		),
	)
}

// maxLogViewerLines caps how many log lines one log viewer request renders.
const maxLogViewerLines = 10000

// HandleLogViewer handles log viewing requests.
func HandleLogViewer(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	lineCountStr := c.PostForm("log_LineCount")
	logLevel := c.PostForm("log_LogLevel")

	lineCount := 100
	if lineCountStr != "" {
		if parsed, err := strconv.Atoi(lineCountStr); err == nil && parsed > 0 {
			lineCount = min(parsed, maxLogViewerLines)
		}
	}

	// Read log file
	logPath := filepath.Join("logs", "downloader.log")

	lines, err := readLastLines(logPath, lineCount, logLevel)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to read log file: "+err.Error(), "danger"))
		return
	}

	// Create log display
	var logNodes []gomponents.Node
	for _, line := range lines {
		// Determine log level for styling
		class := "text-muted"
		if strings.Contains(line, "error") || strings.Contains(line, "ERROR") {
			class = "text-danger"
		} else if strings.Contains(line, "warn") || strings.Contains(line, "WARN") {
			class = "text-warning"
		} else if strings.Contains(line, "info") || strings.Contains(line, "INFO") {
			class = "text-info"
		}

		logNodes = append(logNodes,
			html.Div(
				html.Class(class+" font-monospace small"),
				gomponents.Text(line),
			),
		)
	}

	result := html.Div(
		html.Class("card border-0 shadow-sm border-info mb-4"),
		html.Div(
			html.Class("card-header border-0"),
			html.Style(
				"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
			),
			html.Div(
				html.Class("d-flex align-items-center justify-content-between"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-file-text me-1")),
						gomponents.Text("Logs"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text(fmt.Sprintf("Log Entries (Last %d lines)", len(lines))),
					),
				),
				html.Span(
					html.Class("badge bg-info"),
					gomponents.Text(time.Now().Format("15:04:05")),
				),
			),
		),
		html.Div(
			html.Class("card-body"),
			html.P(
				html.Class("card-text text-muted mb-3"),
				gomponents.Text(
					fmt.Sprintf("Loaded at: %s", time.Now().Format("2006-01-02 15:04:05")),
				),
				func() gomponents.Node {
					if logLevel != "" {
						return gomponents.Text(" - Filtered by level: " + logLevel)
					}

					return gomponents.Text("")
				}(),
			),
			html.Div(
				html.Class("log-container"),
				html.Style(
					"max-height: 400px; overflow-y: auto; background: #f8f9fa; padding: 10px; border-radius: 4px;",
				),
				gomponents.Group(logNodes),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// readLastLines reads the last n lines from a file, optionally filtering by log level.
// tailChunkSize is the initial trailing-chunk size read by readLastLines.
// Doubled and re-read from further back whenever the current chunk doesn't
// yield enough matching lines, instead of ever reading the whole file.
const tailChunkSize = 64 * 1024

// tailMaxWindow caps how far back readLastLines reads when a level filter
// matches rarely, so a large log is never read whole into memory.
const tailMaxWindow = 64 << 20

func readLastLines(filename string, lineCount int, logLevel string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileSize := stat.Size()

	var filteredLines []string

	// Read a bounded trailing chunk from EOF instead of the whole file -
	// grow the window and re-read further back only if the current chunk
	// doesn't contain enough matching lines yet, up to the whole file.
	for chunkSize := min(int64(tailChunkSize), fileSize); ; chunkSize *= 2 {
		if chunkSize > fileSize {
			chunkSize = fileSize
		}

		offset := fileSize - chunkSize

		buffer := make([]byte, chunkSize)
		if _, err := file.ReadAt(buffer, offset); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}

		lines := strings.Split(string(buffer), "\n")
		if offset > 0 && len(lines) > 0 {
			// The chunk starts mid-file, so its first "line" is really the
			// tail of a line whose beginning is outside this chunk - drop it
			// rather than treat a partial line as complete.
			lines = lines[1:]
		}

		filteredLines = filteredLines[:0]
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Apply log level filter if specified
			if logLevel == "" ||
				strings.Contains(strings.ToLower(line), strings.ToLower(logLevel)) {
				filteredLines = append(filteredLines, line)
			}
		}

		if len(filteredLines) >= lineCount || offset == 0 || chunkSize >= tailMaxWindow {
			break
		}
	}

	// Get last n lines and reverse order (newest first)
	start := 0
	if len(filteredLines) > lineCount {
		start = len(filteredLines) - lineCount
	}

	result := filteredLines[start:]

	// Reverse the slice to show newest first
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}
