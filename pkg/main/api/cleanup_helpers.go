package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// cleanupFileTables maps the cleanup tool's media type choices to the
// tables holding that type's tracked file locations.
var cleanupFileTables = []struct{ mediaType, table string }{
	{"movies", "movie_files"},
	{"series", "serie_episode_files"},
	{"books", "book_files"},
	{"audiobooks", "audiobook_files"},
	{"music", "album_files"},
}

// trackedFileLocations returns the file locations stored for the selected
// media type ("all" or empty selects every type).
func trackedFileLocations(mediaTypes string) []string {
	var locations []string

	for _, ft := range cleanupFileTables {
		if mediaTypes != "" && mediaTypes != "all" && mediaTypes != ft.mediaType {
			continue
		}

		locations = append(locations, database.GetrowsN[string](false, 0, "SELECT location FROM "+ft.table)...)
	}

	return locations
}

// performBrokenLinksCheck checks for database entries pointing to missing files.
func performBrokenLinksCheck(mediaTypes string) int {
	brokenCount := 0

	for _, location := range trackedFileLocations(mediaTypes) {
		if !checkFileExists(location) {
			brokenCount++
		}
	}

	return brokenCount
}

// performEmptyDirectoriesCheck counts directories below each scan path that
// contain no media file (at least minFileSize bytes) anywhere beneath them.
// One walk per path: every qualifying file marks all its ancestor
// directories as non-empty.
func performEmptyDirectoriesCheck(scanPaths []string, minFileSize int64) int {
	emptyCount := 0

	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		var dirs []string

		hasMedia := make(map[string]bool)

		_ = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Continue on errors
			}

			if d.IsDir() {
				if path != basePath {
					dirs = append(dirs, path)
				}

				return nil
			}

			if !isMediaFile(path) {
				return nil
			}

			if minFileSize > 0 {
				if info, statErr := d.Info(); statErr != nil || info.Size() < minFileSize {
					return nil
				}
			}

			for dir := filepath.Dir(path); dir != basePath && !hasMedia[dir]; dir = filepath.Dir(dir) {
				hasMedia[dir] = true

				if parent := filepath.Dir(dir); parent == dir {
					break
				}
			}

			return nil
		})

		for _, dir := range dirs {
			if !hasMedia[dir] {
				emptyCount++
			}
		}
	}

	return emptyCount
}

// performOrphanedFilesCheck finds files on disk not tracked in database.
func performOrphanedFilesCheck(scanPaths []string, mediaTypes string, minFileSize int64) int {
	orphanedCount := 0

	locations := trackedFileLocations(mediaTypes)

	dbFiles := make(map[string]bool, len(locations))
	for _, location := range locations {
		dbFiles[location] = true
	}

	// Scan filesystem and check against database
	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			// Check if it's a media file and meets size requirements
			if !isMediaFile(path) {
				return nil
			}

			if minFileSize > 0 {
				if info, statErr := d.Info(); statErr != nil || info.Size() < minFileSize {
					return nil
				}
			}

			// Check if file is tracked in database
			if !dbFiles[path] {
				orphanedCount++
			}

			return nil
		})
		if err != nil {
			continue // Continue with next path on error
		}
	}

	return orphanedCount
}

// performDuplicateFilesCheck finds files with identical sizes (simple duplicate detection).
func performDuplicateFilesCheck(scanPaths []string, minFileSize int64) int {
	sizeMap := make(map[int64][]string)
	duplicateCount := 0

	// Scan all files and group by size
	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if !isMediaFile(path) {
				return nil
			}

			info, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			size := info.Size()
			if minFileSize > 0 && size < minFileSize {
				return nil
			}

			sizeMap[size] = append(sizeMap[size], path)

			return nil
		})
		if err != nil {
			continue // Continue with next path on error
		}
	}

	// Count files that have duplicates (same size)
	for _, files := range sizeMap {
		if len(files) > 1 {
			duplicateCount += len(files) - 1 // Count all but one as duplicates
		}
	}

	return duplicateCount
}

// checkFileExists checks if a file exists at the given path.
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// isMediaFile checks if the file extension indicates a media file.
func isMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mediaExtensions := []string{
		".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v",
		".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m2ts",
		// audio (music, audiobooks)
		".mp3", ".m4a", ".m4b", ".flac", ".ogg", ".opus", ".aac", ".wav", ".wma", ".alac",
		// books
		".epub", ".mobi", ".azw", ".azw3", ".pdf", ".cbz", ".cbr",
	}

	return slices.Contains(mediaExtensions, ext)
}

// dbCheckSamples is how many example rows the database checks show.
const dbCheckSamples = 10

// renderDatabaseChecksSection is the database part of the media cleanup page:
// one checkbox per consistency check, a preview and a repair button.
func renderDatabaseChecksSection(csrfToken string) gomponents.Node {
	checks := database.ConsistencyChecks()

	items := make([]gomponents.Node, 0, len(checks))
	for _, c := range checks {
		id := "dbcheck_" + c.Key
		items = append(items, html.Div(
			html.Class("form-check mb-2"),
			html.Input(html.Class("form-check-input"), html.Type("checkbox"), html.ID(id),
				html.Name("check"), html.Value(c.Key), html.Checked()),
			html.Label(html.Class("form-check-label"), html.For(id),
				html.Strong(gomponents.Text(c.Title)),
				html.Div(html.Class("small text-muted"), gomponents.Text(c.Description)),
			),
		))
	}

	button := func(label, icon, class, action string, extra ...gomponents.Node) gomponents.Node {
		return html.Button(
			html.Type("button"),
			html.Class("btn "+class+" me-2"),
			html.Name("action"), html.Value(action),
			hx.Post("/api/admin/cleanup/database"),
			hx.Target("#dbChecksResults"),
			hx.Swap("innerHTML"),
			hx.Include("#dbChecksForm"),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			gomponents.Group(extra),
			html.I(html.Class("fas "+icon+" me-1")),
			gomponents.Text(label),
		)
	}

	return html.Div(
		html.Class("mt-5"),
		html.H5(html.Class("form-section-title"), gomponents.Text("Database Checks")),
		html.P(html.Class("text-muted"), gomponents.Text(
			"Rows in the database that contradict each other. Check shows what is affected without changing anything; "+
				"Repair fixes the selected checks in the listed order. Back up the database first "+
				"(Database Maintenance → Backup).")),
		html.Form(
			html.ID("dbChecksForm"),
			gomponents.Group(items),
			html.Div(
				html.Class("form-group submit-group mt-3"),
				button("Check Database", "fa-magnifying-glass", "btn-primary", "preview"),
				button("Repair Selected", "fa-wrench", "btn-danger", "repair",
					hx.Confirm("Repair the selected database checks? Rows are deleted or changed; this cannot be undone without a backup.")),
			),
		),
		html.Div(html.ID("dbChecksResults"), html.Class("mt-4")),
	)
}

// HandleDatabaseChecks runs the selected database consistency checks of the
// media cleanup page, as a preview or as a repair.
func HandleDatabaseChecks(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	selected := c.PostFormArray("check")
	if len(selected) == 0 {
		c.String(http.StatusOK, renderAlert("Select at least one check.", "warning"))
		return
	}

	repair := c.PostForm("action") == "repair"

	rows := make([]gomponents.Node, 0, len(selected))

	var total int64

	// Run in the checks' own order, not the order the browser sent them.
	for _, check := range database.ConsistencyChecks() {
		if !slices.Contains(selected, check.Key) {
			continue
		}

		res, err := database.RunConsistencyCheck(check.Key, repair, dbCheckSamples, seriesEpisodeQuality)
		total += res.Total()

		rows = append(rows, renderDatabaseCheckRow(&check, &res, err, repair))
	}

	title := fmt.Sprintf("%d rows affected", total)
	class := "alert-success"

	switch {
	case repair:
		title = fmt.Sprintf("Repair finished: %d rows changed", total)
	case total > 0:
		class = "alert-warning"
		title += " - nothing was changed"
	}

	c.String(http.StatusOK, renderComponentToString(html.Div(
		html.Div(html.Class("alert "+class), html.Strong(gomponents.Text(title))),
		html.Table(
			html.Class("table table-sm align-top"),
			html.THead(html.Tr(
				html.Th(gomponents.Text("Check")),
				html.Th(gomponents.Text("Rows")),
				html.Th(gomponents.Text("Details")),
			)),
			html.TBody(gomponents.Group(rows)),
		),
	)))
}

func renderDatabaseCheckRow(check *database.ConsistencyCheck, res *database.ConsistencyResult, err error, repair bool) gomponents.Node {
	tables := make([]string, 0, len(res.Counts))
	for t, n := range res.Counts {
		tables = append(tables, fmt.Sprintf("%s: %d", t, n))
	}

	slices.Sort(tables)

	details := []gomponents.Node{}
	if len(tables) > 0 {
		details = append(details, html.Div(gomponents.Text(strings.Join(tables, ", "))))
	}

	if len(res.Samples) > 0 {
		label := fmt.Sprintf("Examples (%d)", len(res.Samples))

		items := make([]gomponents.Node, 0, len(res.Samples))
		for _, s := range res.Samples {
			items = append(items, html.Li(html.Code(gomponents.Text(s))))
		}

		details = append(details, html.Details(
			html.Summary(gomponents.Text(label)),
			html.Ul(html.Class("small mb-0"), gomponents.Group(items)),
		))
	}

	if res.Note != "" {
		details = append(details, html.Div(html.Class("text-warning"), gomponents.Text(res.Note)))
	}

	if err != nil {
		details = append(details, html.Div(html.Class("text-danger"), gomponents.Text("Error: "+err.Error())))
	}

	if len(details) == 0 {
		text := "OK"
		if repair {
			text = "Nothing to repair"
		}

		details = append(details, html.Span(html.Class("text-success"), gomponents.Text(text)))
	}

	return html.Tr(
		html.Td(gomponents.Text(check.Title)),
		html.Td(gomponents.Text(strconv.FormatInt(res.Total(), 10))),
		html.Td(gomponents.Group(details)),
	)
}

// seriesEpisodeQuality returns the quality for new episode rows of a series
// without one: its list's template quality, else its media config's.
func seriesEpisodeQuality(_ uint, listname string) string {
	cfgp, listid := findSeriesCfgpAndListID(listname)
	if cfgp == nil {
		return ""
	}

	if listid >= 0 && listid < len(cfgp.Lists) && cfgp.Lists[listid].TemplateQuality != "" {
		return cfgp.Lists[listid].TemplateQuality
	}

	return cfgp.TemplateQuality
}
