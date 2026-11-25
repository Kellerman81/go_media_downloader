package api

import (
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// HandleMediaCleanup handles media cleanup requests.
func HandleMediaCleanup(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	findOrphans := c.PostForm("cleanup_FindOrphans") == "on" ||
		c.PostForm("cleanup_FindOrphans") == "true"
	findDuplicates := c.PostForm("cleanup_FindDuplicates") == "on" ||
		c.PostForm("cleanup_FindDuplicates") == "true"
	findBroken := c.PostForm("cleanup_FindBroken") == "on" ||
		c.PostForm("cleanup_FindBroken") == "true"
	findEmpty := c.PostForm("cleanup_FindEmpty") == "on" ||
		c.PostForm("cleanup_FindEmpty") == "true"
	mediaTypes := c.PostForm("cleanup_MediaTypes")
	if mediaTypes == "" {
		mediaTypes = "all"
	}
	minFileSizeStr := c.PostForm("cleanup_MinFileSize")

	if !findOrphans && !findDuplicates && !findBroken && !findEmpty {
		c.String(http.StatusOK, renderComponentToString(
			html.Div(
				html.Class("card border-0 shadow-sm border-warning mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-warning me-3"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Warning"),
						),
						html.H5(
							html.Class("card-title mb-0 text-warning fw-bold"),
							gomponents.Text("No Options Selected"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted"),
						gomponents.Text(
							"Please select at least one cleanup option to perform the scan.",
						),
					),
				),
			),
		))

		return
	}

	// Parse min file size
	minFileSize, err := strconv.ParseInt(minFileSizeStr, 10, 64)
	if err != nil {
		minFileSize = 0
	}

	minFileSize *= 1024 * 1024 // Convert MB to bytes

	// Perform actual cleanup scan
	startTime := time.Now()

	orphanedCount := 0
	duplicateCount := 0
	brokenCount := 0
	emptyCount := 0

	scanPaths, rejectedPaths := cleanupScanPaths(c.PostForm("cleanup_Paths"))

	// Perform requested scans
	if findBroken {
		brokenCount = performBrokenLinksCheck(mediaTypes)
	}

	if findEmpty && len(scanPaths) > 0 {
		emptyCount = performEmptyDirectoriesCheck(scanPaths, minFileSize)
	}

	if findOrphans && len(scanPaths) > 0 {
		orphanedCount = performOrphanedFilesCheck(scanPaths, mediaTypes, minFileSize)
	}

	if findDuplicates && len(scanPaths) > 0 {
		duplicateCount = performDuplicateFilesCheck(scanPaths, minFileSize)
	}

	totalIssues := orphanedCount + duplicateCount + brokenCount + emptyCount
	scanDuration := time.Since(startTime)

	// Build results table
	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Cleanup Configuration:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Media Types:")), html.Td(gomponents.Text(mediaTypes))),
		html.Tr(
			html.Td(gomponents.Text("Min File Size:")),
			html.Td(gomponents.Text(minFileSizeStr+" MB")),
		),
		html.Tr(
			html.Td(gomponents.Text("Mode:")),
			html.Td(gomponents.Text("Scan only - nothing is changed or deleted")),
		),
		html.Tr(
			html.Td(gomponents.Text("Scan Duration:")),
			html.Td(gomponents.Text(scanDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Scan Results:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add detailed results
	if findOrphans {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Orphaned Files:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d files found", orphanedCount))),
			),
		)
	}

	if findDuplicates {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Possible Duplicates:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d files with the same size as another file (size match only, contents not compared)", duplicateCount))),
			),
		)
	}

	if findBroken {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Broken Links:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d broken database entries", brokenCount))),
			),
		)
	}

	if findEmpty {
		results = append(
			results,
			html.Tr(
				html.Td(gomponents.Text("Empty Directories:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d empty folders", emptyCount))),
			),
		)
	}

	results = append(
		results,
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(html.Td(html.Strong(gomponents.Text("Summary:"))), html.Td(gomponents.Text(""))),
		html.Tr(
			html.Td(gomponents.Text("Total Issues Found:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalIssues))),
		),
		html.Tr(
			html.Td(gomponents.Text("Paths Scanned:")),
			html.Td(gomponents.Text(func() string {
				if len(scanPaths) == 0 {
					return "none"
				}

				return strings.Join(scanPaths, ", ")
			}())),
		),
	)

	if len(rejectedPaths) > 0 {
		results = append(results, html.Tr(
			html.Td(gomponents.Text("Paths Skipped:")),
			html.Td(gomponents.Text(strings.Join(rejectedPaths, ", ")+" (not inside a configured media path)")),
		))
	}

	// Determine alert styling based on results
	alertClass := "card border-0 shadow-sm border-success mb-4"

	message := fmt.Sprintf("Cleanup Scan Complete - %d Issues Found", totalIssues)
	if totalIssues > 10 {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = fmt.Sprintf(
			"Cleanup Scan Complete - %d Issues Found (Action Required)",
			totalIssues,
		)
	} else if totalIssues > 5 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf(
			"Cleanup Scan Complete - %d Issues Found (Review Required)",
			totalIssues,
		)
	}

	result := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				if totalIssues > 10 {
					return "#f8d7da 0%, #f5c6cb 100%"
				} else if totalIssues > 5 {
					return "#fff3cd 0%, #ffeaa7 100%"
				}

				return "#d4edda 0%, #c3e6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					if totalIssues > 10 {
						return "bg-danger"
					} else if totalIssues > 5 {
						return "bg-warning"
					}

					return "bg-success"
				}()+" me-3"), html.I(html.Class("fas fa-broom me-1")), gomponents.Text("Cleanup")),
				html.H5(html.Class("card-title mb-0 "+func() string {
					if totalIssues > 10 {
						return "text-danger"
					} else if totalIssues > 5 {
						return "text-warning"
					}

					return "text-success"
				}()+" fw-bold"), gomponents.Text(message)),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(
				html.Class("table table-hover mb-0"),
				html.Style("background: transparent;"),
				html.TBody(gomponents.Group(results)),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// configuredMediaRoots returns the deduplicated library paths of every
// configured media group.
func configuredMediaRoots() []string {
	media := config.GetSettingsMediaAll()

	var roots []string

	for _, group := range [][]config.MediaTypeConfig{media.Movies, media.Series, media.Books, media.AudioBooks, media.Music} {
		for i := range group {
			for _, pathCfg := range group[i].Data {
				if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
					roots = append(roots, filepath.Clean(pathCfg.CfgPath.Path))
				}
			}
		}
	}

	return collapseNestedPaths(roots)
}

// collapseNestedPaths sorts and deduplicates paths and drops every path that
// lies inside another one in the list, so a directory tree is walked once.
func collapseNestedPaths(paths []string) []string {
	sorted := slices.Clone(paths)
	for i := range sorted {
		sorted[i] = filepath.Clean(sorted[i])
	}

	slices.Sort(sorted)
	sorted = slices.Compact(sorted)

	out := make([]string, 0, len(sorted))
	for _, p := range sorted {
		if !pathWithinAny(p, out) {
			out = append(out, p)
		}
	}

	return out
}

// cleanupScanPaths turns the cleanup tool's newline-separated path input into
// the list of paths to scan. Only paths inside a configured media root are
// accepted (the rest are returned as rejected); empty input means all roots.
func cleanupScanPaths(input string) (scan, rejected []string) {
	roots := configuredMediaRoots()

	if strings.TrimSpace(input) == "" {
		return roots, nil
	}

	seen := make(map[string]bool)

	for _, line := range strings.Split(input, "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}

		p = filepath.Clean(p)
		if seen[p] {
			continue
		}

		seen[p] = true

		if pathWithinAny(p, roots) {
			scan = append(scan, p)
		} else {
			rejected = append(rejected, p)
		}
	}

	return collapseNestedPaths(scan), rejected
}

// pathWithinAny reports whether p equals or lies below one of roots.
func pathWithinAny(p string, roots []string) bool {
	for _, root := range roots {
		rel, err := filepath.Rel(root, p)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

// renderMediaCleanupWizardPage renders a page for media cleanup operations.
func renderMediaCleanupWizardPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-broom header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Media Cleanup Wizard")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Find orphaned files, duplicates and broken links in your media library, and check and repair the database rows behind it.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("cleanupForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Cleanup Options")),

					renderFormGroup("cleanup",
						map[string]string{
							"FindOrphans": "Files on disk that aren't tracked in the database",
						},
						map[string]string{
							"FindOrphans": "Find Orphaned Files",
						},
						"FindOrphans", "checkbox", true, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindDuplicates": "Files with the same size (contents are not compared)",
						},
						map[string]string{
							"FindDuplicates": "Find Duplicate Files",
						},
						"FindDuplicates", "checkbox", false, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindBroken": "Database entries pointing to missing files",
						},
						map[string]string{
							"FindBroken": "Find Broken Links",
						},
						"FindBroken", "checkbox", false, nil),

					renderFormGroup("cleanup",
						map[string]string{
							"FindEmpty": "Folders that contain no media files",
						},
						map[string]string{
							"FindEmpty": "Find Empty Directories",
						},
						"FindEmpty", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Scan Configuration"),
					),

					renderFormGroup("cleanup", map[string]string{
						"MediaTypes": "Select which media types to scan",
					}, map[string]string{
						"MediaTypes": "Media Types",
					}, "MediaTypes", "select", "all", map[string][]string{
						"options": {"all", "movies", "series", "books", "audiobooks", "music"},
					}),

					renderFormGroup("cleanup", map[string]string{
						"MinFileSize": "Minimum file size in MB to consider (0 = no limit)",
					}, map[string]string{
						"MinFileSize": "Min File Size (MB)",
					}, "MinFileSize", "number", "10", nil),

					renderFormGroup("cleanup", map[string]string{
						"Paths": "Specific paths to scan, one per line, inside the configured media paths (leave empty for all configured paths)",
					}, map[string]string{
						"Paths": "Scan Paths",
					}, "Paths", "textarea", "", nil),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Start Cleanup Scan"),
					html.Type("button"),
					hx.Target("#cleanupResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/cleanup"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#cleanupForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('cleanupForm').reset(); document.getElementById('cleanupResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("cleanupResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		renderDatabaseChecksSection(csrfToken),
	)
}
