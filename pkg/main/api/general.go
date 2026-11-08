package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"maragu.dev/gomponents"
)

// AddGeneralRoutes configures general API routes for the application.
// It sets up endpoints for Trakt integration, debug stats, queue management,
// database operations, parsing utilities, quality management, and configuration.
// All routes require API key authentication.
func AddGeneralRoutes(routerapi *gin.RouterGroup) {
	routerapi.Use(checkauth)
	{
		routerapi.GET("/trakt/authorize", apiTraktGetAuthURL)
		routerapi.GET("/trakt/token/:code", apiTraktGetStoreToken)
		routerapi.GET("/trakt/user/:user/:list", apiTraktGetUserList)
		routerapi.GET("/debugstats", apiDebugStats)
		routerapi.GET("/queue", apiQueueList)
		routerapi.GET("/queue/history", apiQueueListStarted)
		routerapi.DELETE("/queue/cancel/:id", apiQueueCancel)
		routerapi.GET("/fillimdb", apiFillImdb)
		routerapi.GET("/scheduler/stop", apiSchedulerStop)
		routerapi.GET("/scheduler/start", apiSchedulerStart)
		routerapi.GET("/scheduler/list", apiSchedulerList)
		routerapi.POST("/scheduler/stop/:id", apiScheduleStop)
		routerapi.POST("/scheduler/start/:id", apiScheduleStart)
		routerapi.POST("/scheduler/queue/:id", apiScheduleQueue)
		routerapi.GET("/db/integrity", apiDBIntegrity)
		routerapi.GET("/db/cleanuporphans", apiDBCleanupOrphans)
		routerapi.GET("/db/backup", apiDBBackup)
		routerapi.DELETE("/db/delete/:name/:id", apiDBDelete)
		routerapi.DELETE("/db/clear/:name", apiDBClear)
		routerapi.DELETE("/db/clearcache", apiDBClearCache)
		routerapi.DELETE("/db/oldjobs", apiDBRemoveOldJobs)
		routerapi.GET("/db/vacuum", apiDBVacuum)
		routerapi.POST("/parse/string", apiParseString)
		routerapi.POST("/parse/file", apiParseFile)
		routerapi.POST("/naming", apiNamingGenerate)
		routerapi.POST("/structure", apiStructure)
		routerapi.POST("/album/force-match", apiAlbumForceMatch)
		routerapi.GET("/quality", apiGetQualities)
		routerapi.DELETE("/quality/:id", apiQualityDelete)
		routerapi.POST("/quality", apiQualityUpdate)
		routerapi.GET("/quality/all", apiListAllQualityPriorities)
		routerapi.GET("/quality/complete", apiListCompleteAllQualityPriorities)
		routerapi.GET("/quality/get/:name", apiListQualityPriorities)
		routerapi.GET("/slug", apiDBRefreshSlugs)

		routerapi.GET("/config/all", apiConfigAll)
		routerapi.DELETE("/config/clear", apiConfigClear)
		routerapi.GET("/config/refresh", apiConfigRefreshFile)
		routerapi.GET("/config/get/:name", apiConfigGet)

		routerapi.DELETE("/config/delete/:name", apiConfigDelete)

		routerapi.POST("/config/update/:name", apiConfigUpdate)

		routerapi.GET("/config/type/:type", apiListConfigType)

		routerapi.GET("/cache/refresh", apiCacheRefresh)
		routerapi.GET("/cache/list", apiCacheList)
		routerapi.POST("/cache/add", apiCacheAdd)
		routerapi.DELETE("/cache/remove/:key", apiCacheRemove)
		routerapi.DELETE("/cache/clear/:type", apiCacheClear)

		routerapi.GET("/statistics", apiStatistics)
		routerapi.GET("/statistics/movies", apiMovieStatistics)
		routerapi.GET("/statistics/series", apiSeriesStatistics)
		routerapi.GET("/statistics/workers", apiWorkerStatistics)
	}
}

// AddWebRoutes sets up web interface routes for the admin panel.
// It configures static file serving and admin pages for configuration
// management including general settings, quality profiles, downloaders,
// indexers, lists, media settings, paths, notifications, regex patterns,
// and schedulers. Includes both GET routes for displaying forms and
// POST routes for handling updates.
func AddWebRoutes(routerapi *gin.RouterGroup) {
	// Start session cleanup goroutine
	startSessionCleanup()

	// Apply authentication middleware to all routes
	routerapi.Use(protectAdminRoutes)

	// Public routes (no authentication required)
	routerapi.Static("/static", "./static")
	routerapi.GET("/", handleRootRedirect)
	routerapi.GET("/login", loginPage)
	routerapi.POST("/login", handleLogin)
	routerapi.GET("/logout", handleLogout)

	// All admin and manage routes are automatically protected by the middleware
	routerapi.GET("/admin", apiAdminInterface)
	routerapi.POST("/admin/paths/check", HandlePathCheck)
	routerapi.POST("/admin/proxy", HandleAdminAPIProxy)
	routerapi.POST("/admin/generate-apikey", HandleGenerateAPIKey)
	routerapi.POST("/admin/indexers/test", HandleTestIndexerConnection)
	routerapi.POST("/admin/downloader/test", HandleTestDownloaderConnection)
	routerapi.POST("/admin/notifications/test", HandleTestNotificationConnection)
	routerapi.POST("/admin/lists/test", HandleTestListConnection)
	routerapi.GET("/admin/wizard", adminPageSetupWizard)
	routerapi.GET("/admin/wizard/dismiss", HandleWizardDismiss)
	routerapi.GET("/admin/wizard/step/:n", HandleWizardStepGet)
	routerapi.POST("/admin/wizard/step/:n", HandleWizardStepSave)
	routerapi.POST("/admin/wizard/test/indexer", HandleWizardTestIndexer)
	routerapi.POST("/admin/wizard/test/downloader", HandleWizardTestDownloader)
	routerapi.GET("/admin/config/:configtype", adminPageConfig)
	routerapi.POST("/admin/config/:configtype/update", HandleConfigUpdate)
	routerapi.GET("/admin/seriesconfig", renderSeriesConfigPage)
	routerapi.GET("/admin/seriesconfig/add", handleSeriesConfigAdd)
	routerapi.GET("/admin/seriesconfig/edit", handleSeriesConfigEdit)
	routerapi.POST("/admin/seriesconfig/delete", handleSeriesConfigDelete)
	routerapi.POST("/admin/seriesconfig/update", handleSeriesConfigUpdate)
	routerapi.GET("/admin/seriesconfig/helper", handleScraperHelper)
	routerapi.GET("/admin/testparse", adminPageTestParse)
	routerapi.POST("/admin/testparse", HandleTestParse)
	routerapi.GET("/admin/moviemetadata", adminPageMovieMetadata)
	routerapi.POST("/admin/moviemetadata", HandleMovieMetadata)
	routerapi.GET("/admin/traktauth", adminPageTraktAuth)
	routerapi.POST("/admin/traktauth", HandleTraktAuth)
	routerapi.GET("/admin/namingtest", adminPageNamingTest)
	routerapi.POST("/admin/namingtest", HandleNamingTest)
	routerapi.GET("/admin/jobmanagement", adminPageJobManagement)
	routerapi.POST("/admin/jobmanagement", HandleJobManagement)
	routerapi.GET("/admin/crongen", adminPageCronGenerator)
	routerapi.POST("/admin/crongen/validate", HandleCronValidation)
	routerapi.GET("/admin/debugstats", adminPageDebugStats)
	routerapi.POST("/admin/debugstats", HandleDebugStats)
	routerapi.GET("/admin/dbmaintenance", adminPageDatabaseMaintenance)
	routerapi.POST("/admin/dbmaintenance", HandleDatabaseMaintenance)
	routerapi.GET("/admin/searchdownload", adminPageSearchDownload)
	routerapi.POST("/admin/searchdownload", HandleSearchDownload)
	routerapi.GET("/admin/pushovertest", adminPagePushoverTest)
	routerapi.POST("/admin/pushovertest", HandlePushoverTest)
	routerapi.GET("/admin/logviewer", adminPageLogViewer)
	routerapi.POST("/admin/logviewer", HandleLogViewer)
	routerapi.GET("/admin/feedparse", adminPageFeedParsing)
	routerapi.POST("/admin/feedparse", HandleFeedParsing)
	routerapi.POST("/admin/feed-lists", HandleFeedLists)
	routerapi.GET("/admin/folderstructure", adminPageFolderStructure)
	routerapi.POST("/admin/folderstructure", HandleFolderStructure)
	routerapi.GET("/admin/matchtester", adminPageMatchTester)
	routerapi.GET("/admin/matchtester/sources", HandleMatchTesterSources)
	routerapi.POST("/admin/matchtester/start", HandleMatchTesterStart)
	routerapi.GET("/admin/matchtester/page", HandleMatchTesterPage)
	routerapi.GET("/admin/matchtester/folder", HandleMatchTesterFolder)
	routerapi.GET("/admin/matchtester/entry", HandleMatchTesterEntry)
	routerapi.GET("/admin/matchtester/status", HandleMatchTesterStatus)
	routerapi.POST("/admin/matchtester/select", HandleMatchTesterSelect)
	routerapi.POST("/admin/matchtester/process", HandleMatchTesterProcess)

	// New Management Tools Routes
	routerapi.GET("/admin/media-cleanup", adminPageMediaCleanup)
	routerapi.POST("/admin/cleanup", HandleMediaCleanup)
	routerapi.GET("/admin/missing-episodes", adminPageMissingEpisodes)
	routerapi.POST("/admin/missing-episodes", HandleMissingEpisodes)
	routerapi.GET("/admin/log-analysis", adminPageLogAnalysis)
	routerapi.POST("/admin/log-analysis", HandleLogAnalysis)
	routerapi.GET("/admin/storage-health", adminPageStorageHealth)
	routerapi.POST("/admin/storage-health", HandleStorageHealth)
	routerapi.POST("/admin/storage-health/monitor", HandleStorageHealth)
	routerapi.GET("/admin/service-health", adminPageServiceHealth)
	routerapi.POST("/admin/service-health", HandleServiceHealth)
	routerapi.POST("/admin/service-health/quick", HandleServiceHealth)

	routerapi.GET("/admin/api-testing", adminPageAPITesting)
	routerapi.POST("/admin/api-testing/execute", HandleAPITesting)

	routerapi.GET("/admin/quality-reorder", adminPageQualityReorder)
	routerapi.POST("/admin/quality-reorder", HandleQualityReorder)
	routerapi.GET("/admin/quality-reorder/rules", HandleQualityReorderRules)
	routerapi.POST("/admin/quality-reorder/add-rule", HandleQualityReorderAddRule)
	routerapi.POST("/admin/quality-reorder/reset-rules", HandleQualityReorderResetRules)
	routerapi.GET("/admin/quality-profile-rules", HandleQualityProfileRules)

	routerapi.GET("/admin/statistics", webStatisticsPage)
	routerapi.GET("/admin/statistics/data", webStatisticsData)
	routerapi.GET("/admin/statistics/movies", webStatisticsMovies)
	routerapi.GET("/admin/statistics/series", webStatisticsSeries)
	routerapi.GET("/admin/statistics/music", webStatisticsMusic)
	routerapi.GET("/admin/statistics/books", webStatisticsBooks)
	routerapi.GET("/admin/statistics/audiobooks", webStatisticsAudiobooks)
	routerapi.GET("/admin/statistics/storage", webStatisticsStorage)
	routerapi.GET("/admin/statistics/workers", webStatisticsWorkers)
	routerapi.GET("/admin/statistics/http", webStatisticsHTTP)
	routerapi.GET("/admin/statistics/metadata", webStatisticsMetadata)
	routerapi.GET("/admin/statistics/notification", webStatisticsNotification)
	routerapi.GET("/admin/statistics/downloader", webStatisticsDownloader)
	routerapi.GET("/admin/statistics/system", webStatisticsSystem)
	routerapi.GET("/admin/statistics/activity", webStatisticsActivity)
	routerapi.GET("/admin/statistics/backlog", webStatisticsBacklog)
	routerapi.GET("/admin/statistics/jobs", webStatisticsJobs)
	routerapi.GET("/admin/statistics/quality", webStatisticsQuality)

	routerapi.GET("/admin/regex-tester", adminPageRegexTester)
	routerapi.POST("/admin/regex-tester/test", HandleRegexTesting)
	routerapi.GET("/admin/naming-generator", adminPageNamingGenerator)
	routerapi.GET("/admin/naming-generator/fields/:type", HandleNamingFieldsForType)
	routerapi.POST("/admin/naming-generator/preview", HandleNamingPreview)
	routerapi.POST("/admin/naming-generator/verify", HandleNamingVerify)

	routerapi.GET("/admin/database/:tablename", adminPageDatabase)
	routerapi.GET("/admin/grid/:grid", adminPageGrid)
	routerapi.GET("/admin/queue/partial", renderQueuePartial)
	routerapi.GET("/admin/dashboard/cards", dashboardCardsPartial)
	routerapi.GET("/admin/wanted", renderWantedPage)
	routerapi.GET("/admin/wanted/partial", renderWantedPartial)
	routerapi.GET("/admin/wanted/tab", renderWantedTab)

	// Calendar routes
	routerapi.GET("/admin/calendar", CalendarPageHandler)
	routerapi.GET("/admin/calendar/controls", CalendarControlsHandler)
	routerapi.GET("/admin/calendar/content", CalendarContentHandler)
	routerapi.GET("/admin/calendar/events", CalendarEventsHandler)
	routerapi.GET("/admin/calendar/day-events", CalendarDayEventsHandler)
	routerapi.GET("/admin/calendar/ical", CalendarICalHandler)

	// Metadata search routes
	routerapi.GET("/admin/metadata-search/movies", MovieMetadataSearchPage)
	routerapi.GET("/admin/metadata-search/series", SeriesMetadataSearchPage)
	routerapi.GET("/admin/metadata-search/books", BookMetadataSearchPage)
	routerapi.GET("/admin/metadata-search/audiobooks", AudiobookMetadataSearchPage)
	routerapi.GET("/admin/metadata-search/music", MusicMetadataSearchPage)
	routerapi.POST("/admin/search/movies", SearchMovieMetadata)
	routerapi.POST("/admin/search/series", SearchSeriesMetadata)
	routerapi.POST("/admin/search/books", SearchBookMetadata)
	routerapi.POST("/admin/search/audiobooks", SearchAudiobookMetadata)
	routerapi.POST("/admin/search/music", SearchMusicMetadata)
	routerapi.POST("/admin/search/music/series", SearchMusicSeriesMetadata)
	routerapi.POST("/admin/search/music/artist", SearchMusicArtists)
	routerapi.POST("/admin/search/music/artist-releases", ListArtistReleasesForSelection)
	routerapi.POST("/admin/discover/series-albums", DiscoverSeriesAlbumsByName)
	routerapi.POST("/admin/add/movie", AddMovieToDatabase)
	routerapi.POST("/admin/add/series", AddSeriesToDatabase)
	routerapi.POST("/admin/add/book", AddBookToDatabase)
	routerapi.POST("/admin/add/audiobook", AddAudiobookToDatabase)
	routerapi.POST("/admin/add/movie/manual", AddMovieManual)
	routerapi.POST("/admin/add/series/manual", AddSeriesManual)
	routerapi.POST("/admin/add/book/manual", AddBookManual)
	routerapi.POST("/admin/add/audiobook/manual", AddAudiobookManual)
	routerapi.POST("/admin/add/book/author", AddBooksByAuthor)
	routerapi.POST("/admin/add/audiobook/author", AddAudiobooksByAuthor)
	routerapi.POST("/admin/add/album", AddAlbumToDatabase)
	routerapi.POST("/admin/add/artist-albums", AddArtistAlbumsFiltered)
	routerapi.POST("/admin/add/albums-selected", AddSelectedAlbums)

	// Handle POST requests to database routes - these should not be used for form submissions
	// but provide helpful error message to prevent 404s
	routerapi.POST("/admin/database/:name", func(ctx *gin.Context) {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Form submission should use /api/admin/table/{name}/update/{id} or /api/admin/table/{name}/insert endpoints",
		})
	})

	// routerapi.GET("/admin/table", apiAdminTableDataForm)
	routerapi.Any("/admin/tablejson/:table", apiAdminTableDataJson)
	// routerapi.POST("/admin/table", apiAdminTableDataForm)
	routerapi.GET("/admin/tableedit/:name/:id", apiAdminTableDataEditForm)
	// routerapi.GET("/admin/table/:name", apiAdminTableData)
	// routerapi.GET("/admin/table/:name/schema", apiAdminTableSchema)
	routerapi.POST("/admin/table/:name/insert", apiAdminTableInsert)
	routerapi.POST("/admin/table/:name/update/:index", apiAdminTableUpdate)
	routerapi.POST("/admin/table/:name/delete/:index", apiAdminTableDelete)
	routerapi.Any("/admin/dropdown/:table/:field", apiAdminDropdownData)

	// Add-row endpoints. The add buttons sit inside the config form, so htmx
	// posts the current rows; the new row gets max existing index + 1 so it
	// never collides with a posted row (also after rows were removed).
	routerapi.Any("/manage/media/form/:typev", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "media_main_"+ctx.Param("typev")+"_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderMediaForm(
			ctx.Param("typev"),
			idx,
			&config.MediaTypeConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/downloader/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "downloader_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderDownloaderForm(
			idx,
			&config.DownloaderConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/lists/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "lists_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderListsForm(
			idx,
			&config.ListsConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/indexers/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "indexers_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderIndexersForm(
			idx,
			&config.IndexersConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/paths/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "paths_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderPathsForm(
			idx,
			&config.PathsConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/notification/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "notifications_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderNotificationForm(
			idx,
			&config.NotificationConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/regex/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "regex_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderRegexForm(idx, &config.RegexConfig{Name: "new" + strconv.Itoa(idx)}),
		)
	})
	routerapi.Any("/manage/quality/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "quality_main_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderQualityForm(
			idx,
			&config.QualityConfig{Name: "new" + strconv.Itoa(idx)},
			getCSRFToken(ctx),
		))
	})
	routerapi.Any("/manage/qualityreorder/form/:typev", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "quality_"+ctx.Param("typev")+"_reorder_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderQualityReorderForm(idx, ctx.Param("typev"), &config.QualityReorderConfig{}),
		)
	})
	routerapi.Any("/manage/qualityindexer/form/:typev", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(
			ctx,
			"quality_"+ctx.Param("typev")+"_indexer_",
			"_TemplateIndexer",
		)
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderQualityIndexerForm(idx, ctx.Param("typev"), &config.QualityIndexerConfig{}),
		)
	})
	routerapi.Any("/manage/scheduler/form", func(ctx *gin.Context) {
		idx, ok := nextRowIndexFromForm(ctx, "scheduler_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderSchedulerForm(idx, &config.SchedulerConfig{Name: "new" + strconv.Itoa(idx)}),
		)
	})
	routerapi.Any("/manage/mediadata/form/:prefix/:configv", func(ctx *gin.Context) {
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_data"

		idx, ok := nextRowIndexFromForm(ctx, prefix+"_", "_TemplatePath")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderMediaDataForm(prefix, idx, &config.MediaDataConfig{}))
	})
	routerapi.Any("/manage/mediaimport/form/:prefix/:configv", func(ctx *gin.Context) {
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_dataimport"

		idx, ok := nextRowIndexFromForm(ctx, prefix+"_", "_TemplatePath")
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderMediaDataImportForm(prefix, idx, &config.MediaDataImportConfig{}),
		)
	})
	routerapi.Any("/manage/medianotification/form/:prefix/:configv", func(ctx *gin.Context) {
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_notification"

		idx, ok := nextRowIndexFromForm(ctx, prefix+"_", "_MapNotification")
		if !ok {
			return
		}

		renderAddedRow(
			ctx,
			renderMediaNotificationForm(prefix, idx, &config.MediaNotificationConfig{}),
		)
	})
	routerapi.Any("/manage/medialists/form/:prefix/:configv", func(ctx *gin.Context) {
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_lists"

		idx, ok := nextRowIndexFromForm(ctx, prefix+"_", "_Name")
		if !ok {
			return
		}

		renderAddedRow(ctx, renderMediaListsForm(prefix, idx, &config.MediaListsConfig{}))
	})

	// Config preview endpoint - returns readonly HTML view of a config item
	routerapi.GET("/manage/config/preview/:type/:name", func(ctx *gin.Context) {
		configType := ctx.Param("type")
		configName := ctx.Param("name")

		if configName == "" {
			ctx.String(
				http.StatusOK,
				"<div class='alert alert-warning'>No configuration selected</div>",
			)

			return
		}

		var form gomponents.Node
		switch configType {
		case "path":
			if cfg := config.GetSettingsPath(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"Path: "+configName,
					renderPathsForm(0, cfg, getCSRFToken(ctx)),
				)
			}

		case "list":
			if cfg := config.GetSettingsList(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"List: "+configName,
					renderListsForm(0, cfg, getCSRFToken(ctx)),
				)
			}

		case "quality":
			if cfg := config.GetSettingsQuality(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"Quality: "+configName,
					renderQualityForm(0, cfg, ""),
				)
			}

		case "scheduler":
			if cfg := config.GetSettingsScheduler(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"Scheduler: "+configName,
					renderSchedulerForm(0, cfg),
				)
			}

		case "indexer":
			if cfg := config.GetSettingsIndexer(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"Indexer: "+configName,
					renderIndexersForm(0, cfg, getCSRFToken(ctx)),
				)
			}

		case "downloader":
			for _, cfg := range config.GetSettingsDownloaderAll() {
				if cfg.Name == configName {
					form = renderConfigPreviewReadonly(
						"Downloader: "+configName,
						renderDownloaderForm(0, &cfg, getCSRFToken(ctx)),
					)

					break
				}
			}

		case "regex":
			for _, cfg := range config.GetSettingsRegexAll() {
				if cfg.Name == configName {
					form = renderConfigPreviewReadonly("Regex: "+configName, renderRegexForm(0, &cfg))
					break
				}
			}

		case "notification":
			if cfg := config.GetSettingsNotification(configName); cfg != nil {
				form = renderConfigPreviewReadonly(
					"Notification: "+configName,
					renderNotificationForm(0, cfg, getCSRFToken(ctx)),
				)
			}
		}

		if form == nil {
			ctx.String(
				http.StatusOK,
				"<div class='alert alert-warning'>Configuration not found: "+configType+"/"+configName+"</div>",
			)

			return
		}

		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
}

type apiparse struct {
	Name    string
	Typ     string
	Path    string
	Config  string
	Quality string
	Year    bool
}

// @Summary      Debug information
// @Description  Shows some stats
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object} Statsresults
// @Failure      401  {object}  Jsonerror
// @Router       /api/debugstats [get].
func apiDebugStats(ctx *gin.Context) {
	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	gcjson, _ := json.Marshal(gc)
	memjson, _ := json.Marshal(mem)

	debug.FreeOSMemory()
	ctx.JSON(http.StatusOK, gin.H{
		"GC Stats":     string(gcjson),
		"Mem Stats":    string(memjson),
		"GOOS":         runtime.GOOS,
		"NumCPU":       runtime.NumCPU(),
		"NumGoroutine": runtime.NumGoroutine(),
		"GOARCH":       runtime.GOARCH,
		"WorkerStats":  worker.GetStats(),
	})
}

type Statsresults struct {
	GCStats      string       `json:"GC Stats"`
	MemStats     string       `json:"Mem Stats"`
	GOOS         string       `json:"GOOS"`
	GOARCH       string       `json:"GOARCH"`
	NumCPU       int          `json:"NumCPU"`
	NumGoroutine int          `json:"NumGoroutine"`
	WorkerStats  worker.Stats `json:"WorkerStats"`
}

// @Summary      Queue
// @Description  Lists Queued and Started Jobs (but not finished)
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]syncops.Job}
// @Failure      401  {object}  Jsonerror
// @Router       /api/queue [get].
func apiQueueList(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, worker.GetQueues())
}

// @Summary      Queue History
// @Description  Lists Started Jobs and finished but not queued jobs
// @Tags         general
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondata{data=[]database.JobHistory}
// @Failure      401    {object}  Jsonerror
// @Router       /api/queue/history [get].
func apiQueueListStarted(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	// Set defaults if not provided
	if query.OrderBy == "" {
		query.OrderBy = "ID desc"
	}

	if query.Limit == 0 {
		query.Limit = 100
	}

	jobs := database.QueryJobHistory(query)
	sendJSONResponse(ctx, http.StatusOK, jobs)
}

// @Summary      Trakt Authorize
// @Description  Get trakt auth url
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/trakt/authorize [get].
func apiTraktGetAuthURL(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, traktAuthURL())
}

// @Summary      Trakt Save Token
// @Description  Saves Trakt token after Authorization
// @Tags         general
// @Param        code  path      string  true  "code"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondata{data=any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/trakt/token/{code} [get].
func apiTraktGetStoreToken(ctx *gin.Context) {
	code, ok := getParamID(ctx, "code")
	if !ok {
		return
	}

	_, _ = traktExchangeCode(ctx.Request.Context(), code)

	sendJSONResponse(ctx, http.StatusOK, traktToken())
}

// @Summary      Trakt Get List (Auth Test)
// @Description  Get User List
// @Tags         general
// @Param        user  path      string  true  "Trakt Username"
// @Param        list  path      string  true  "List Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondataerror{data=[]trakt.TraktUserListItem}
// @Failure      401   {object}  Jsonerror
// @Router       /api/trakt/user/{user}/{list} [get].
func apiTraktGetUserList(ctx *gin.Context) {
	user, ok := getParamID(ctx, "user")
	if !ok {
		return
	}

	listParam, ok := getParamID(ctx, "list")
	if !ok {
		return
	}

	list, err := traktUserList(ctx.Request.Context(), user, listParam, "movie,show")
	if len(list) > 10 {
		list = list[:10]
	}

	ctx.JSON(http.StatusOK, gin.H{"data": list, "error": err})
}

// @Summary      Refresh Slugs
// @Description  Regenerates slugs for all tables. By default only populates empty slugs. Use force=true to regenerate all slugs.
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Param        force  query     string    false "Set to 'true' to regenerate all slugs, not just empty ones"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/slug [get].
func apiDBRefreshSlugs(ctx *gin.Context) {
	// Check for force parameter - if true, update all slugs; otherwise only empty ones
	force := ctx.Query("force") == "true"

	// Populate slugs for all tables using the unified function
	database.PopulateAllSlugs(force)

	sendSuccess(ctx, StrOK)
}

// @Summary      Parse a string
// @Description  Parses a string for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  Jsondataerror{data=database.ParseInfo}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/parse/string [post].
func apiParseString(ctx *gin.Context) {
	var getcfg apiparse
	if !bindJSONWithValidation(ctx, &getcfg) {
		return
	}

	var cfgv string
	if getcfg.Typ == logger.StrMovie {
		cfgv = "movie_" + getcfg.Config
	} else {
		cfgv = "serie_" + getcfg.Config
	}

	cfgp := config.GetSettingsMedia(cfgv)
	parse := parser_v2.ParseFile(getcfg.Name, false, false, cfgp, -1)
	// parse := parser.NewFileParser(getcfg.Name, cfgp, false, -1)
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)

	err := parser.GetDBIDs(parse, cfgp, true, false)
	ctx.JSON(http.StatusOK, gin.H{"data": parse, "error": err})
	parse.Close()
}

// @Summary      Parse a file
// @Description  Parses a file for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  Jsondata{data=database.ParseInfo}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/parse/file [post].
func apiParseFile(ctx *gin.Context) {
	var getcfg apiparse
	if !bindJSONWithValidation(ctx, &getcfg) {
		return
	}

	var cfgv string
	if getcfg.Typ == logger.StrMovie {
		cfgv = "movie_" + getcfg.Config
	} else {
		cfgv = "serie_" + getcfg.Config
	}

	cfgp := config.GetSettingsMedia(cfgv)
	// defer parse.Close()
	parse := parser_v2.ParseFile(getcfg.Path, true, false, cfgp, -1)
	// parse := parser.NewFileParser(filepath.Base(getcfg.Path), cfgp, false, -1)
	parse.File = getcfg.Path
	parser.ParseVideoFile(ctx, parse, config.GetSettingsQuality(getcfg.Quality))
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)
	parser.GetDBIDs(parse, cfgp, true, false)
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
	parse.Close()
}

// @Summary      Generate IMDB Cache
// @Description  Downloads IMDB Dataset and creates a new database from it
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/fillimdb [get].
func apiFillImdb(ctx *gin.Context) {
	config.GetSettingsGeneral().Jobs["RefreshImdb"](0, ctx)
	sendSuccess(ctx, StrOK)
}

// @Summary      Stop Scheduler
// @Description  Stops all Schedulers
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/stop [get].
func apiSchedulerStop(c *gin.Context) {
	// The old scheduler.QueueData/Feeds/Search stop calls this replaced were
	// commented out, left over from a prior scheduler implementation - this
	// endpoint did nothing at all while still reporting success. worker's
	// StopCronWorker is the current implementation's equivalent (already used
	// the same way in apiDBBackup above).
	worker.StopCronWorker()
	sendSuccess(c, StrOK)
}

// @Summary      Start Scheduler
// @Description  Start all Schedulers
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/start [get].
func apiSchedulerStart(c *gin.Context) {
	worker.StartCronWorker()
	sendSuccess(c, StrOK)
}

// @Summary      Stop Schedule
// @Description  Stops one schedule: it queues no jobs until started again (in memory only - a restart or scheduler config reload starts it again)
// @Tags         scheduler
// @Param        id     path     string    true  "Schedule ID"
// @Param        apikey query    string    true  "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Failure      404    {object} Jsonerror
// @Router       /api/scheduler/stop/{id} [post].
func apiScheduleStop(ctx *gin.Context) {
	setSchedulePaused(ctx, true)
}

// @Summary      Start Schedule
// @Description  Starts a schedule stopped via /api/scheduler/stop/{id} again
// @Tags         scheduler
// @Param        id     path     string    true  "Schedule ID"
// @Param        apikey query    string    true  "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Failure      404    {object} Jsonerror
// @Router       /api/scheduler/start/{id} [post].
func apiScheduleStart(ctx *gin.Context) {
	setSchedulePaused(ctx, false)
}

// @Summary      Queue Schedule
// @Description  Queues one run of a schedule's job now - also for a stopped schedule
// @Tags         scheduler
// @Param        id     path     string    true  "Schedule ID"
// @Param        apikey query    string    true  "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Failure      404    {object} Jsonerror
// @Failure      409    {object} Jsonerror
// @Router       /api/scheduler/queue/{id} [post].
func apiScheduleQueue(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		sendBadRequest(ctx, "Invalid schedule ID format")
		return
	}

	switch err := worker.QueueSchedule(uint32(id)); {
	case err == nil:
		sendSuccess(ctx, StrOK)
	case errors.Is(err, logger.ErrNotFound):
		sendNotFound(ctx, "Schedule not found")
	case errors.Is(err, worker.ErrNotQueued):
		sendJSONError(ctx, http.StatusConflict, "Job is already queued or running")
	default:
		sendJSONError(ctx, http.StatusConflict, err.Error())
	}
}

func setSchedulePaused(ctx *gin.Context, paused bool) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 32)
	if err != nil {
		sendBadRequest(ctx, "Invalid schedule ID format")
		return
	}

	if !worker.SetSchedulePaused(uint32(id), paused) {
		sendNotFound(ctx, "Schedule not found")
		return
	}

	sendSuccess(ctx, StrOK)
}

// @Summary      Scheduler Jobs
// @Description  Lists Planned Jobs
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]syncops.JobSchedule}
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/list [get].
func apiSchedulerList(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, worker.GetSchedules())
}

// @Summary      Backup DB
// @Description  Saves DB
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/backup [get].
func apiDBBackup(ctx *gin.Context) {
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		// Pause, not Close: Close also stops the interval schedules for good.
		worker.StopCronWorker()
		worker.PauseWorkerPools(context.Background())
	}

	backupto := "./backup/data.db." + database.GetVersion() + "." + time.Now().
		Format("20060102_150405")
	err := database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)

	// Workers must always be restarted, whether or not the backup itself
	// succeeded - otherwise a failed backup would also permanently leave
	// the app's background workers stopped.
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		worker.InitWorkerPools(
			config.GetSettingsGeneral().WorkerSearch,
			config.GetSettingsGeneral().WorkerFiles,
			config.GetSettingsGeneral().WorkerMetadata,
			config.GetSettingsGeneral().WorkerRSS,
			config.GetSettingsGeneral().WorkerIndexer,
			config.GetSettingsGeneral().WorkerParse,
		)
		worker.StartCronWorker()
	}

	handleDBError(ctx, err, StrOK)
}

// @Summary      Integrity DB
// @Description  Integrity Check DB
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/integrity [get].
func apiDBIntegrity(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, database.DBIntegrityCheck())
}

// @Summary      Cleanup Orphaned Rows
// @Description  Deletes child rows whose parent rows no longer exist, per the schema's foreign-key relationships. Returns deleted row counts per table.
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  map[string]int64
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/cleanuporphans [get].
func apiDBCleanupOrphans(ctx *gin.Context) {
	deleted, err := database.CleanupOrphans(false)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "orphan cleanup incomplete: "+err.Error())
		return
	}

	sendJSONResponse(ctx, http.StatusOK, deleted)
}

// @Summary      Clear DB Table
// @Description  Clears a DB Table
// @Tags         database
// @Param        name  path      string  true  "Table Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/clear/{name} [delete].
func apiDBClear(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	// tableName is concatenated directly into the DELETE below (a table
	// name can't be bound as a `?` SQL parameter), so it must be checked
	// against the same table allowlist the DB browser already uses
	// (database.GetTableDefaults returns a zero-value QueryParams, with an
	// empty Table field, for anything not in its known-table switch) -
	// otherwise any apikey holder could DELETE/VACUUM arbitrary tables.
	if database.GetTableDefaults(tableName).Table == "" {
		sendBadRequest(ctx, "Unknown table: "+tableName)
		return
	}

	err := database.ExecNErr("DELETE from " + tableName)
	if err == nil {
		err = database.ExecNErr("VACUUM")
	}

	handleDBError(ctx, err, StrOK)
}

// @Summary      Delete Row from DB Table
// @Description  Deletes a Row from a DB Table
// @Tags         database
// @Param        name  path      string  true  "Table Name"
// @Param        id    path      string  true  "Row ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/delete/{name}/{id} [delete].
func apiDBDelete(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// See apiDBClear for why tableName must be checked against the
	// allowlist before being concatenated into the query.
	if database.GetTableDefaults(tableName).Table == "" {
		sendBadRequest(ctx, "Unknown table: "+tableName)
		return
	}

	err := database.ExecNErr("DELETE FROM "+tableName+" WHERE id = ?", id)
	handleDBError(ctx, err, StrOK)
}

// @Summary      Clear Caches
// @Description  Clears Caches
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/clearcache [delete].
func apiDBClearCache(ctx *gin.Context) {
	database.ClearCaches()
	sendSuccess(ctx, StrOK)
}

// @Summary      Vacuum DB
// @Description  Vacuum database
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/vacuum [get].
func apiDBVacuum(ctx *gin.Context) {
	err := database.ExecNErr("VACUUM")
	handleDBError(ctx, err, StrOK)
}

// @Summary      Clean Old Jobs
// @Description  Removes Jobs started over x days ago from db
// @Tags         database
// @Param        days  query     int  true  "Days ago"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/oldjobs [delete].
func apiDBRemoveOldJobs(ctx *gin.Context) {
	queryParam, ok := ctx.GetQuery("days")
	if !ok {
		sendForbidden(ctx, "days missing")
		return
	}

	if queryParam == "" {
		sendForbidden(ctx, "days empty")
		return
	}

	days, _ := strconv.Atoi(queryParam)
	if days == 0 {
		sendSuccess(ctx, StrOK)
		return
	}

	scantime := time.Now().AddDate(0, 0, 0-days)
	_, err := database.DeleteRow("job_histories", "created_at < ?", scantime)
	handleDBError(ctx, err, StrOK)
}

// @Summary      List Qualities
// @Description  List Qualities with regex filters
// @Tags         quality
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}   Jsondata{data=[]database.Qualities}
// @Failure      401  {object}  Jsonerror
// @Router       /api/quality [get].
func apiGetQualities(ctx *gin.Context) {
	data := database.StructscanT[database.Qualities](
		false,
		database.Getdatarow[uint](false, "select count() from qualities"),
		"select * from qualities",
	)
	sendJSONResponse(ctx, http.StatusOK, data)
}

// @Summary      Delete Quality
// @Description  Deletes a quality
// @Tags         quality
// @Param        id   path      string  true  "Id of Quality to delete"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}   Jsondata{data=[]database.Qualities}
// @Failure      401  {object}  Jsonerror
// @Router       /api/quality/{id} [delete].
func apiQualityDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	if _, err := database.DeleteRow("qualities", logger.FilterByID, id); err != nil {
		sendBadRequest(ctx, "Failed to delete quality: "+err.Error())
		return
	}

	database.SetVars()

	data := database.StructscanT[database.Qualities](
		false,
		database.Getdatarow[uint](false, "select count() from qualities"),
		"select * from qualities",
	)
	sendJSONResponse(ctx, http.StatusOK, data)
}

// @Summary      Update Quality
// @Description  Updates or adds a quality
// @Tags         quality
// @Param        quality  body      database.Qualities  true  "Quality"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}   Jsondata{data=[]database.Qualities}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/quality [post].
func apiQualityUpdate(ctx *gin.Context) {
	var quality database.Qualities
	if !bindJSONWithValidation(ctx, &quality) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from qualities where id != 0 and id = ?",
		&quality.ID,
	)

	var writeErr error

	if counter == 0 {
		_, writeErr = database.InsertArray(
			"qualities",
			[]string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			quality.QualityType,
			quality.Name,
			quality.Regex,
			quality.Strings,
			quality.Priority,
			quality.UseRegex,
		)
	} else {
		_, writeErr = database.UpdateArray(
			"qualities",
			[]string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			"id != 0 and id = ?",
			quality.QualityType,
			quality.Name,
			quality.Regex,
			quality.Strings,
			quality.Priority,
			quality.UseRegex,
			quality.ID,
		)
	}

	if writeErr != nil {
		sendBadRequest(ctx, "Failed to save quality: "+writeErr.Error())
		return
	}

	database.SetVars()

	data := database.StructscanT[database.Qualities](
		false,
		database.Getdatarow[uint](false, "select count() from qualities"),
		"select * from qualities",
	)
	sendJSONResponse(ctx, http.StatusOK, data)
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        name    path      string  true  "Quality Name: ex. SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/get/{name} [get].
func apiListQualityPriorities(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	if !config.CheckGroup("quality_", name) {
		sendNotFound(ctx, "quality not found")
		return
	}

	returnprios := make([]parser.Prioarr, 0, 1000)
	for _, prio := range parser.Getallprios() {
		if prio.QualityGroup == name {
			returnprios = append(returnprios, prio)
		}
	}

	sendJSONResponse(ctx, http.StatusOK, returnprios)
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/all [get].
func apiListAllQualityPriorities(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, parser.Getallprios())
}

// @Summary      List Quality Priorities
// @Description  List all qualities and their priorities
// @Tags         quality
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/complete [get].
func apiListCompleteAllQualityPriorities(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, parser.Getcompleteallprios())
}

// @Summary      Get Complete Config
// @Description  Get All Config Parameters
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401  {object}  Jsonerror
// @Router       /api/config/all [get].
func apiConfigAll(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// configRedacted replaces secret config values in API responses.
const configRedacted = "***REDACTED***"

// isSecretConfigField reports whether a config struct field name (as
// encoded to JSON, e.g. "WebAPIKey", "Password", "PlexToken") holds a
// credential.
func isSecretConfigField(name string) bool {
	n := strings.ToLower(name)

	return strings.Contains(n, "password") || strings.Contains(n, "apikey") ||
		strings.Contains(n, "api_key") || strings.Contains(n, "token") ||
		strings.Contains(n, "secret") || strings.HasSuffix(n, "key")
}

// redactConfigSecrets returns a JSON-shaped copy of v with every non-empty
// secret field replaced, so config read endpoints don't hand out the web
// API key, download-client passwords or indexer keys.
func redactConfigSecrets(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}

	redactSecretsWalk(out)

	return out
}

func redactSecretsWalk(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if s != "" && isSecretConfigField(k) {
					t[k] = configRedacted
				}

				continue
			}

			redactSecretsWalk(val)
		}

	case []any:
		for _, val := range t {
			redactSecretsWalk(val)
		}
	}
}

// restoreRedactedSecrets puts the stored value back into every field of dst
// that still holds configRedacted, so a client that read a config through the
// redacting endpoints and posts it back does not overwrite real secrets.
func restoreRedactedSecrets(dst any, name string) {
	stored, ok := config.GetCfgAll()[name]
	if !ok {
		return
	}

	var cur, old any

	b, err := json.Marshal(dst)
	if err != nil || json.Unmarshal(b, &cur) != nil {
		return
	}

	b, err = json.Marshal(stored)
	if err != nil || json.Unmarshal(b, &old) != nil {
		return
	}

	if !mergeRedacted(cur, old) {
		return
	}

	if b, err = json.Marshal(cur); err == nil {
		_ = json.Unmarshal(b, dst)
	}
}

// mergeRedacted replaces configRedacted values in cur with the value at the
// same position in old and reports whether anything was replaced.
func mergeRedacted(cur, old any) bool {
	changed := false

	switch c := cur.(type) {
	case map[string]any:
		o, _ := old.(map[string]any)
		for k, v := range c {
			if s, ok := v.(string); ok && s == configRedacted {
				if ov, ok := o[k]; ok {
					c[k] = ov
				} else {
					c[k] = ""
				}

				changed = true

				continue
			}

			if mergeRedacted(v, o[k]) {
				changed = true
			}
		}

	case []any:
		o, _ := old.([]any)
		for i, v := range c {
			var ov any
			if i < len(o) {
				ov = o[i]
			}

			if mergeRedacted(v, ov) {
				changed = true
			}
		}
	}

	return changed
}

// @Summary      Clear Config
// @Description  Clears the configuration and sets some examples -> Use with caution
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/clear [delete].
func apiConfigClear(ctx *gin.Context) {
	config.ClearCfg()
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      Get Config
// @Description  Gets a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondata{data=any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/get/{name} [get].
func apiConfigGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()[name]))
}

// @Summary      Delete Config
// @Description  Deletes a configuration -> Use with caution -> also resets your comments
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {array}  Jsondata{data=map[string]any}
// @Failure      401  {object}  Jsonerror
// @Router       /api/config/delete/{name} [delete].
func apiConfigDelete(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	if err := config.DeleteCfgEntry(name); err == nil {
		reloadRuntimeFor(name)
	}

	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      Reload ConfigFile
// @Description  Refreshes the config from the file
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/refresh [get].
func apiConfigRefreshFile(ctx *gin.Context) {
	config.LoadCfgDB(true)
	parser.GenerateAllQualityPriorities()
	parser.GenerateCutoffPriorities()
	scheduler.Reload()
	providers.ReloadClientProviders()
	providers.RegisterNotificationProviders()
	sendSuccess(ctx, "config reloaded")
}

// reloadRuntimeFor re-applies a saved or deleted config entry to the parts of
// the running app that only read it at registration time: the download-client,
// indexer and notification providers, and the scheduler. Without the config file watcher
// nothing else would pick the change up before a restart.
func reloadRuntimeFor(name string) {
	if strings.HasPrefix(name, "downloader_") || strings.HasPrefix(name, "indexer_") {
		providers.ReloadClientProviders()
	}

	// Notification providers carry the general user agent.
	if name == "general" || strings.HasPrefix(name, "notification_") {
		providers.RegisterNotificationProviders()
	}

	if affectsScheduler(name) {
		scheduler.Reload()
	}
}

// affectsScheduler reports whether a config entry feeds the schedules: the
// scheduler templates, the media configs that reference them, and general
// (UseCronInsteadOfInterval).
func affectsScheduler(name string) bool {
	if name == "general" {
		return true
	}

	for _, p := range []string{"scheduler_", "serie_", "movie_", "book_", "audiobook_", "music_"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}

	return false
}

// @Summary      Update Config
// @Description  Updates a configuration -> Use with caution -> also resets your comments
// @Tags         config
// @Param        config  body      any  true  "Config"
// @Param        name    path      string       true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {array}  Jsondata{data=map[string]any}
// @Failure      401     {object}  Jsonerror
// @Failure      400  {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/config/update/{name} [post].
func apiConfigUpdate(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}

	var err error

	switch left {
	case "general":
		var getcfg config.GeneralConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "downloader":
		var getcfg config.DownloaderConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case logger.StrImdb:
		var getcfg config.ImdbConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "indexer":
		var getcfg config.IndexersConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "list":
		var getcfg config.ListsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "serie", "movie":
		var getcfg config.MediaTypeConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "notification":
		var getcfg config.NotificationConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "path":
		var getcfg config.PathsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "quality":
		var getcfg config.QualityConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "regex":
		var getcfg config.RegexConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	case "scheduler":
		var getcfg config.SchedulerConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}

		restoreRedactedSecrets(&getcfg, name)

		err = config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})

	default:
		sendBadRequest(ctx, "Unknown config type: "+left)
		return
	}

	if err != nil {
		sendBadRequest(ctx, "Failed to update config: "+err.Error())
		return
	}

	reloadRuntimeFor(name)
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(config.GetCfgAll()))
}

// @Summary      List Config Type
// @Description  List configurations of type
// @Tags         config
// @Param        type  path      string  true  "Type Name: ex. quality"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  Jsondata{data=map[string]any}
// @Failure      401     {object}  Jsonerror
// @Router       /api/config/type/{type} [get].
func apiListConfigType(ctx *gin.Context) {
	list := make(map[string]any)

	name, ok := getParamID(ctx, "type")
	if !ok {
		return
	}

	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}

	switch left {
	case "general":
		list["general"] = config.GetSettingsGeneral()
	case logger.StrImdb:
		list[logger.StrImdb] = config.GetSettingsImdb()
	case "downloader":
		config.RangeSettingsDownloader(func(key string, cfgdata *config.DownloaderConfig) {
			if strings.HasPrefix(key, right) {
				list["downloader_"+key] = cfgdata
			}
		})

	case "indexer":
		config.RangeSettingsIndexer(func(key string, cfgdata *config.IndexersConfig) {
			if strings.HasPrefix(key, right) {
				list["indexer_"+key] = cfgdata
			}
		})

	case "list":
		config.RangeSettingsList(func(key string, cfgdata *config.ListsConfig) {
			if strings.HasPrefix(key, right) {
				list["list_"+key] = cfgdata
			}
		})

	case logger.StrSerie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) error {
			if strings.HasPrefix(key, right) {
				list["serie_"+key] = cfgdata
			}

			return nil
		})

	case logger.StrMovie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) error {
			if strings.HasPrefix(key, right) {
				list["movie_"+key] = cfgdata
			}

			return nil
		})

	case "notification":
		config.RangeSettingsNotification(func(key string, cfgdata *config.NotificationConfig) {
			if strings.HasPrefix(key, right) {
				list["notification_"+key] = cfgdata
			}
		})

	case "path":
		config.RangeSettingsPath(func(key string, cfgdata *config.PathsConfig) {
			if strings.HasPrefix(key, right) {
				list["path_"+key] = cfgdata
			}
		})

	case "quality":
		config.RangeSettingsQuality(func(key string, cfgdata *config.QualityConfig) {
			if strings.HasPrefix(key, right) {
				list["quality_"+key] = cfgdata
			}
		})

	case "regex":
		config.RangeSettingsRegex(func(key string, cfgdata *config.RegexConfig) {
			if strings.HasPrefix(key, right) {
				list["regex_"+key] = cfgdata
			}
		})

	case "scheduler":
		config.RangeSettingsScheduler(func(key string, cfgdata *config.SchedulerConfig) {
			if strings.HasPrefix(key, right) {
				list["scheduler_"+key] = cfgdata
			}
		})
	}

	sendJSONResponse(ctx, http.StatusOK, redactConfigSecrets(list))
}

type apiNameInput struct {
	CfgMedia  string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}

// @Summary      Name Generation Test
// @Description  Test your Naming Convention
// @Tags         general
// @Param        config  body      apiNameInput  true  "Config"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  JSONNaming
// @Failure      400     {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/naming [post].
func apiNamingGenerate(ctx *gin.Context) {
	var cfg apiNameInput
	if !bindJSONWithValidation(ctx, &cfg) {
		return
	}

	// defer mediacfg.Close()
	if cfg.GroupType == logger.StrMovie {
		movie, err := database.GetMovies(
			database.Querywithargs{Where: logger.FilterByID},
			cfg.MovieID,
		)
		if err != nil || movie == nil {
			sendBadRequest(ctx, "movie not found")
			return
		}

		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		if cfgp == nil || len(cfgp.DataImport) == 0 || len(cfgp.Data) == 0 {
			sendBadRequest(ctx, "media config not found or has no data/data_import templates")
			return
		}

		s := structure.NewStructure(
			cfgp,
			cfgp.DataImport[0].TemplatePath,
			cfgp.Data[0].TemplatePath, false, false, 0)
		if s == nil {
			sendBadRequest(ctx, "failed to create organizer - check that Structure is enabled for this config")
			return
		}

		defer s.Close()

		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata

		orgadata2.MediaFile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = movie.Rootpath

		m := parser_v2.ParseFile(
			cfg.FilePath,
			true,
			true,
			cfgp,
			cfgp.GetMediaListsEntryListID(movie.Listname),
		)

		if m == nil || m.ListID < 0 || m.ListID >= len(cfgp.Lists) {
			sendBadRequest(ctx, "file could not be parsed or list not found")
			return
		}

		orgadata2.Listid = m.ListID
		namingPreviewProbe(ctx, cfgp, m, cfg.FilePath, cfgp.Lists[m.ListID].CfgQuality)

		s.GenerateNamingTemplate(&orgadata2, m, &movie.DbmovieID)
		ctx.JSON(
			http.StatusOK,
			gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m},
		)
	} else {
		series, err := database.GetSeries(
			database.Querywithargs{Where: logger.FilterByID},
			cfg.SerieID,
		)
		if err != nil || series == nil {
			sendBadRequest(ctx, "series not found")
			return
		}
		// defer logger.ClearVar(&series)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		if cfgp == nil || len(cfgp.DataImport) == 0 || len(cfgp.Data) == 0 {
			sendBadRequest(ctx, "media config not found or has no data/data_import templates")
			return
		}

		s := structure.NewStructure(
			cfgp,
			cfgp.DataImport[0].TemplatePath,
			cfgp.Data[0].TemplatePath,
			false, false, 0,
		)
		if s == nil {
			sendBadRequest(ctx, "failed to create organizer - check that Structure is enabled for this config")
			return
		}

		defer s.Close()

		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata

		orgadata2.MediaFile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = series.Rootpath

		m := parser_v2.ParseFile(
			cfg.FilePath,
			true,
			true,
			cfgp,
			cfgp.GetMediaListsEntryListID(series.Listname),
		)

		if m == nil || m.ListID < 0 || m.ListID >= len(cfgp.Lists) {
			sendBadRequest(ctx, "file could not be parsed or list not found")
			return
		}

		orgadata2.Listid = m.ListID
		namingPreviewProbe(ctx, cfgp, m, cfg.FilePath, cfgp.Lists[m.ListID].CfgQuality)

		m.SerieID = series.ID
		m.DbserieID = series.DbserieID
		if err := m.Getepisodestoimport(); err != nil || len(m.Episodes) == 0 {
			sendBadRequest(ctx, "no matching episode found for this file in the selected series")
			return
		}

		s.GenerateNamingTemplate(&orgadata2, m, &m.Episodes[0].Num1)
		ctx.JSON(
			http.StatusOK,
			gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m},
		)
	}
}

type apiStructureJSON struct {
	Folder                     string
	Grouptype                  string
	Sourcepathtemplate         string
	Targetpathtemplate         string
	Configentry                string
	Forceid                    uint
	Disableruntimecheck        bool
	Disabledisallowed          bool
	Disabledeletewronglanguage bool
}

type apiCacheAddJSON struct {
	CacheType string `binding:"required" json:"cache_type"`
	Key       string `binding:"required" json:"key"`
	Value     string `binding:"required" json:"value"`
}

// @Summary      Structure Single Item
// @Description  Structure a single folder
// @Tags         general
// @Param        config  body      apiStructureJSON  true  "Config"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  string
// @Failure      400     {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/structure [post].
func apiStructure(ctx *gin.Context) {
	var cfg apiStructureJSON
	if !bindJSONWithValidation(ctx, &cfg) {
		return
	}

	var getconfig string
	if strings.EqualFold(cfg.Grouptype, logger.StrMovie) {
		getconfig = "movie_" + cfg.Configentry
	}

	if strings.EqualFold(cfg.Grouptype, logger.StrSeries) {
		getconfig = "serie_" + cfg.Configentry
	}

	// defer media.Close()
	if config.GetSettingsMedia(getconfig).Name != cfg.Configentry {
		sendBadRequest(ctx, "media config not found")
		return
	}

	if !config.CheckGroup("path_", cfg.Sourcepathtemplate) {
		sendBadRequest(ctx, "source config not found")
		return
	}

	if !config.CheckGroup("path_", cfg.Targetpathtemplate) {
		sendBadRequest(ctx, "target config not found")
		return
	}

	if !scanner.CheckFileExist(cfg.Folder) {
		sendBadRequest(ctx, "folder not found")
		return
	}

	cfgp := config.GetSettingsMedia(getconfig)

	var cfgimport *config.MediaDataImportConfig
	for _, imp := range cfgp.DataImport {
		if strings.EqualFold(imp.TemplatePath, cfg.Sourcepathtemplate) {
			cfgimport = &imp
			break
		}
	}

	checkruntime := config.GetSettingsPath(cfg.Sourcepathtemplate).CheckRuntime
	if cfg.Disableruntimecheck {
		checkruntime = false
	}

	deletewronglanguage := config.GetSettingsPath(cfg.Sourcepathtemplate).DeleteWrongLanguage
	if cfg.Disabledeletewronglanguage {
		deletewronglanguage = false
	}

	// structurevar := structure.NewStructure(cfgp, cfgimport, cfg.Sourcepathtemplate, cfg.Targetpathtemplate, false, false, 0)

	// structurevar.ManualId = cfg.Forceid
	structure.OrganizeSingleFolder(
		ctx,
		cfg.Folder,
		cfgp,
		cfgimport,
		cfg.Targetpathtemplate,
		checkruntime,
		deletewronglanguage,
		cfg.Forceid,
	)

	ctx.JSON(http.StatusOK, gin.H{})
}

type apiAlbumForceMatchJSON struct {
	Folder             string `binding:"required" json:"folder"`
	Configentry        string `binding:"required" json:"configentry"`
	Grouptype          string `binding:"required" json:"grouptype"`
	Sourcepathtemplate string `binding:"required" json:"sourcepathtemplate"`
	Targetpathtemplate string `binding:"required" json:"targetpathtemplate"`
	ForcedID           string `binding:"required" json:"forced_id"`
	Preview            bool   `                   json:"preview"`
}

// @Summary      Force-match album folder
// @Description  Match a folder against a specific MBID or ASIN and either preview the rename plan or execute the full organize workflow
// @Tags         general
// @Param        config  body      apiAlbumForceMatchJSON  true  "Config"
// @Param        apikey  query     string                  true  "apikey"
// @Success      200     {object}  structure.AlbumForceMatchPreview
// @Failure      400     {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/album/force-match [post].
func apiAlbumForceMatch(ctx *gin.Context) {
	var cfg apiAlbumForceMatchJSON
	if !bindJSONWithValidation(ctx, &cfg) {
		return
	}

	var getconfig string
	if strings.EqualFold(cfg.Grouptype, "music") {
		getconfig = "music_" + cfg.Configentry
	} else if strings.EqualFold(cfg.Grouptype, "audiobook") {
		getconfig = "audiobook_" + cfg.Configentry
	} else {
		sendBadRequest(ctx, "grouptype must be music or audiobook")
		return
	}

	if config.GetSettingsMedia(getconfig).Name != cfg.Configentry {
		sendBadRequest(ctx, "media config not found")
		return
	}

	if !config.CheckGroup("path_", cfg.Sourcepathtemplate) {
		sendBadRequest(ctx, "source config not found")
		return
	}

	if !config.CheckGroup("path_", cfg.Targetpathtemplate) {
		sendBadRequest(ctx, "target config not found")
		return
	}

	if !scanner.CheckFileExist(cfg.Folder) {
		sendBadRequest(ctx, "folder not found")
		return
	}

	cfgp := config.GetSettingsMedia(getconfig)

	var cfgimport *config.MediaDataImportConfig
	for _, imp := range cfgp.DataImport {
		if strings.EqualFold(imp.TemplatePath, cfg.Sourcepathtemplate) {
			cfgimport = &imp
			break
		}
	}

	s := structure.NewStructure(
		cfgp,
		cfg.Sourcepathtemplate,
		cfg.Targetpathtemplate,
		false,
		false,
		0,
	)
	if s == nil {
		sendBadRequest(ctx, "failed to create organizer")
		return
	}

	preview, err := s.ForceMatchAlbumFolder(
		ctx,
		cfg.Folder,
		cfgp,
		cfgimport,
		structure.AlbumForce{ID: cfg.ForcedID},
		cfg.Preview,
	)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if cfg.Preview {
		ctx.JSON(http.StatusOK, preview)
	} else {
		ctx.JSON(http.StatusOK, gin.H{})
	}
}

// @Summary      Cancel Queue Job
// @Description  Cancel a running or pending job in the queue
// @Tags         queue
// @Param        id     path     string    true  "Queue job ID"
// @Param        apikey query    string    true  "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Failure      404    {object} Jsonerror
// @Router       /api/queue/cancel/{id} [delete].
func apiQueueCancel(ctx *gin.Context) {
	idStr := ctx.Param("id")
	if idStr == "" {
		sendBadRequest(ctx, "Missing queue ID")
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		sendBadRequest(ctx, "Invalid queue ID format")
		return
	}

	queueID := uint32(id)

	// Check if the job exists in the queue
	queues := worker.GetQueues()
	if _, exists := queues[queueID]; !exists {
		ctx.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Queue job not found",
		})

		return
	}

	// Cancel the job and remove it from the queue
	worker.CancelQueueEntry(queueID)

	// Flat like the not-found reply above: the queue page reads data.success,
	// and sendJSONResponse would nest it under "data", so a successful cancel
	// was reported as failed.
	ctx.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Queue job canceled successfully",
	})
}

// @Summary      Refresh Cache
// @Description  Manually trigger a full cache refresh for database tables
// @Tags         cache
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=string} "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/cache/refresh [get].
func apiCacheRefresh(ctx *gin.Context) {
	config.GetSettingsGeneral().Jobs["RefreshCache"](0, ctx)
	sendSuccess(ctx, StrOK)
}

// @Summary      List Cache Types
// @Description  List all available cache types and their current status
// @Tags         cache
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]any} "cache types and status"
// @Failure      401  {object}  Jsonerror
// @Router       /api/cache/list [get].
func apiCacheList(ctx *gin.Context) {
	cacheInfo := map[string]any{
		"available_types": []string{
			logger.CacheMovie,
			logger.CacheSeries,
			logger.CacheDBMovie,
			logger.CacheDBSeries,
			logger.CacheDBSeriesAlt,
			logger.CacheTitlesMovie,
			logger.CacheUnmatchedMovie,
			logger.CacheUnmatchedSeries,
			logger.CacheFilesMovie,
			logger.CacheFilesSeries,
			logger.CacheHistoryURLMovie,
			logger.CacheHistoryTitleMovie,
			logger.CacheHistoryURLSeries,
			logger.CacheHistoryTitleSeries,
		},
		"description": map[string]string{
			logger.CacheMovie:              "Movie list cache",
			logger.CacheSeries:             "Series list cache",
			logger.CacheDBMovie:            "Movie database cache",
			logger.CacheDBSeries:           "Series database cache",
			logger.CacheDBSeriesAlt:        "Alternative series database cache",
			logger.CacheTitlesMovie:        "Movie titles cache",
			logger.CacheUnmatchedMovie:     "Unmatched movie files cache",
			logger.CacheUnmatchedSeries:    "Unmatched series files cache",
			logger.CacheFilesMovie:         "Movie files cache",
			logger.CacheFilesSeries:        "Series files cache",
			logger.CacheHistoryURLMovie:    "Movie URL history cache",
			logger.CacheHistoryTitleMovie:  "Movie title history cache",
			logger.CacheHistoryURLSeries:   "Series URL history cache",
			logger.CacheHistoryTitleSeries: "Series title history cache",
		},
	}
	sendJSONResponse(ctx, http.StatusOK, cacheInfo)
}

// @Summary      Add Cache Entry
// @Description  Manually add an entry to a specific cache
// @Tags         cache
// @Param        cache  body      apiCacheAddJSON  true  "Cache entry data"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=string} "returns ok"
// @Failure      400  {object}  Jsonerror
// @Failure      401  {object}  Jsonerror
// @Router       /api/cache/add [post].
func apiCacheAdd(ctx *gin.Context) {
	var req apiCacheAddJSON
	if !bindJSONWithValidation(ctx, &req) {
		return
	}

	// Validate cache type
	validTypes := []string{
		logger.CacheMovie, logger.CacheSeries, logger.CacheDBMovie, logger.CacheDBSeries,
		logger.CacheDBSeriesAlt, logger.CacheTitlesMovie, logger.CacheUnmatchedMovie,
		logger.CacheUnmatchedSeries, logger.CacheFilesMovie, logger.CacheFilesSeries,
		logger.CacheHistoryURLMovie, logger.CacheHistoryTitleMovie,
		logger.CacheHistoryURLSeries, logger.CacheHistoryTitleSeries,
	}

	valid := slices.Contains(validTypes, req.CacheType)

	if !valid {
		sendBadRequest(ctx, "Invalid cache type. Use /api/cache/list to see available types")
		return
	}

	// Add to appropriate cache based on type
	// For simplicity, most cache types use string append
	database.AppendCache(req.Key, req.Value)

	logger.Logtype("info", 0).
		Str("type", req.CacheType).
		Str("key", req.Key).
		Str("value", req.Value).
		Msg("Manual cache entry added")
	sendSuccess(ctx, "Cache entry added successfully")
}

// @Summary      Remove Cache Entry
// @Description  Remove a specific cache entry by key
// @Tags         cache
// @Param        key    path     string    true  "Cache key to remove"
// @Param        apikey query    string    true  "apikey"
// @Success      200  {object}  Jsondata{data=string} "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/cache/remove/{key} [delete].
func apiCacheRemove(ctx *gin.Context) {
	key := ctx.Param("key")
	if key == "" {
		sendBadRequest(ctx, "Missing cache key")
		return
	}

	// Remove from all cache types that might contain this key
	// This is a bit brute force, but ensures removal from all relevant caches
	database.DeleteCacheEntry(key)

	logger.Logtype("info", 0).Str("key", key).Msg("Manual cache entry removed")
	sendSuccess(ctx, "Cache entry removed successfully")
}

// @Summary      Clear Cache Type
// @Description  Clear all entries from a specific cache type
// @Tags         cache
// @Param        type   path     string    true  "Cache type to clear"
// @Param        apikey query    string    true  "apikey"
// @Success      200  {object}  Jsondata{data=string} "returns ok"
// @Failure      400  {object}  Jsonerror
// @Failure      401  {object}  Jsonerror
// @Router       /api/cache/clear/{type} [delete].
func apiCacheClear(ctx *gin.Context) {
	cacheType := ctx.Param("type")
	if cacheType == "" {
		sendBadRequest(ctx, "Missing cache type")
		return
	}

	// Validate cache type
	validTypes := []string{
		logger.CacheMovie, logger.CacheSeries, logger.CacheDBMovie, logger.CacheDBSeries,
		logger.CacheDBSeriesAlt, logger.CacheTitlesMovie, logger.CacheUnmatchedMovie,
		logger.CacheUnmatchedSeries, logger.CacheFilesMovie, logger.CacheFilesSeries,
		logger.CacheHistoryURLMovie, logger.CacheHistoryTitleMovie,
		logger.CacheHistoryURLSeries, logger.CacheHistoryTitleSeries,
	}

	valid := slices.Contains(validTypes, cacheType)

	if !valid {
		sendBadRequest(ctx, "Invalid cache type. Use /api/cache/list to see available types")
		return
	}

	// Clear the specific cache type
	database.ClearCacheType(cacheType)

	logger.Logtype("info", 0).Str("type", cacheType).Msg("Manual cache type cleared")
	sendSuccess(ctx, fmt.Sprintf("Cache type %s cleared successfully", cacheType))
}
