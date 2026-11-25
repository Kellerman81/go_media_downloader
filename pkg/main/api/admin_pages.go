package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
)

func adminPage() string {
	pageNode := page("Go Media Downloader", false, false, false, renderModernAdminIntro())

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	return buf.String()
}

// adminPage generates the HTML page using gomponents
// adminPageConfig - consolidated handler for all config pages.
func adminPageConfig(ctx *gin.Context) {
	configType, ok := getParamID(ctx, "configtype")
	if !ok {
		return
	}

	var pageNode gomponents.Node

	csrfToken := getCSRFToken(ctx)

	switch configType {
	case "general":
		configv := config.GetSettingsGeneral()

		pageNode = page(
			"Config General",
			true,
			false,
			false,
			renderGeneralConfig(configv, csrfToken),
		)

	case "imdb":
		configv := config.GetSettingsImdb()

		pageNode = page("Config Imdb", true, false, false, renderImdbConfig(configv, csrfToken))

	case "media":
		configv := config.GetSettingsMediaAll()

		pageNode = page("Config Media", true, false, false, renderMediaConfig(configv, csrfToken))

	case "downloader":
		configv := config.GetSettingsDownloaderAll()

		pageNode = page(
			"Config Downloader",
			true,
			false,
			false,
			renderDownloaderConfig(configv, csrfToken),
		)

	case "indexers":
		configv := config.GetSettingsIndexerAll()

		pageNode = page(
			"Config Indexer",
			true,
			false,
			false,
			renderIndexersConfig(configv, csrfToken),
		)

	case "lists":
		configv := config.GetSettingsListAll()

		pageNode = page("Config Lists", true, false, false, renderListsConfig(configv, csrfToken))

	case "paths":
		configv := config.GetSettingsPathAll()

		pageNode = page("Config Paths", true, false, false, renderPathsConfig(configv, csrfToken))

	case "notifications":
		configv := config.GetSettingsNotificationAll()

		pageNode = page(
			"Config Notifications",
			true,
			false,
			false,
			renderNotificationConfig(configv, csrfToken),
		)

	case "quality":
		configv := config.GetSettingsQualityAll()

		pageNode = page(
			"Config Quality",
			true,
			false,
			false,
			renderQualityConfig(configv, csrfToken),
		)

	case "regex":
		configv := config.GetSettingsRegexAll()

		pageNode = page("Config Regex", true, false, false, renderRegexConfig(configv, csrfToken))

	case "scheduler":
		configv := config.GetSettingsSchedulerAll()

		pageNode = page(
			"Config Scheduler",
			true,
			false,
			false,
			renderSchedulerConfig(configv, csrfToken),
		)

	default:
		sendNotFound(ctx, "unknown config type: "+configType)
		return
	}

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTestParse(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("String Parse Test", false, false, true, renderTestParsePage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageMovieMetadata(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Movie Metadata Lookup",
		false,
		false,
		true,
		renderMovieMetadataPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTraktAuth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Trakt Authentication", false, false, true, renderTraktAuthPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageNamingTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Naming Convention Test", false, false, true, renderNamingTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageJobManagement(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Job Management", false, false, true, renderJobManagementPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageCronGenerator(ctx *gin.Context) {
	renderToolPage(ctx, "Cron Expression Generator", renderCronGeneratorPage(getCSRFToken(ctx), ctx.Query("value")))
}

func adminPageDebugStats(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Debug Statistics", false, false, true, renderDebugStatsPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageDatabaseMaintenance(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Database Maintenance",
		false,
		false,
		true,
		renderDatabaseMaintenancePage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageSearchDownload(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Search & Download", false, false, true, renderSearchDownloadPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPagePushoverTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Notification Test", false, false, true, renderPushoverTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageLogViewer(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Log Viewer", false, false, true, renderLogViewerPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFeedParsing(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Feed Parser & Results", false, false, true, renderFeedParsingPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFolderStructure(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Folder Structure Organizer",
		false,
		false,
		true,
		renderFolderStructurePage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// New Management Tools Admin Page Handlers.
func adminPageMediaCleanup(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Media Cleanup Wizard",
		false,
		false,
		true,
		renderMediaCleanupWizardPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageMissingEpisodes(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Missing Episodes Finder",
		false,
		false,
		true,
		renderMissingEpisodesFinderPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageLogAnalysis(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Log Analysis Dashboard",
		false,
		false,
		true,
		renderLogAnalysisDashboardPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageStorageHealth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Storage Health Monitor",
		false,
		false,
		true,
		renderStorageHealthMonitorPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageServiceHealth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"External Service Health Check",
		false,
		false,
		true,
		renderExternalServiceHealthCheckPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageAPITesting(ctx *gin.Context) {
	// csrfToken := getCSRFToken(ctx)
	pageNode := page("API Testing Suite", false, false, true, renderAPITestingPage())

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageGrid(ctx *gin.Context) {
	grid, ok := getParamID(ctx, "grid")
	if !ok {
		return
	}

	switch grid {
	case "queue":
		renderQueuePage(ctx)
	case "scheduler":
		renderSchedulerPage(ctx)
	case "stats":
		renderStatsPage(ctx)
	default:
		sendNotFound(ctx, "Unknown grid")
	}
}

// adminPageDatabase - consolidated handler for all database table pages.
func adminPageDatabase(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "tablename")
	if !ok {
		return
	}

	// Reject invalid table names (like source maps, static files, etc.)
	if strings.Contains(tableName, ".") || strings.Contains(tableName, "/") {
		sendNotFound(ctx, "Invalid table name: "+tableName)
		return
	}

	// Verify the table exists by checking if GetTableDefaults returns valid data
	tableDefault := database.GetTableDefaults(tableName)
	if tableDefault.Table == "" {
		sendNotFound(ctx, "Table not found: "+tableName)
		return
	}

	// Create page title from table name
	pageTitle := "Database " + strings.ToTitle(strings.ReplaceAll(tableName, "_", " "))

	pageNode := page(
		pageTitle,
		false,
		true,
		false,
		adminDatabaseContent(tableName, getCSRFToken(ctx)),
	)
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func AdminPageAny(ctx *gin.Context, pageTitle string, content gomponents.Node) {
	pageNode := page(pageTitle, false, false, true, content)
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageQualityReorder(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page(
		"Quality Reorder Testing",
		false,
		false,
		true,
		renderQualityReorderPage(csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageRegexTester(ctx *gin.Context) {
	renderToolPage(ctx, "Regex Pattern Tester",
		renderRegexTesterPage(getCSRFToken(ctx), ctx.Query("value"), ctx.Query("tab") == "pattern"))
}

func adminPageNamingGenerator(ctx *gin.Context) {
	renderToolPage(ctx, "Naming Template Generator",
		renderNamingGeneratorPage(getCSRFToken(ctx), ctx.Query("type"), ctx.Query("value")))
}
