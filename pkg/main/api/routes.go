package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
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
		routerapi.GET("/cache/cleanupfiles", apiCacheCleanupFiles)
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
	loadLanguages(i18nDir)

	// Apply authentication middleware to all routes
	routerapi.Use(protectAdminRoutes)
	routerapi.Use(i18nMiddleware)

	// Public routes (no authentication required)
	routerapi.Static("/static", "./static")
	routerapi.GET("/", handleRootRedirect)
	routerapi.GET("/login", loginPage)
	routerapi.POST("/login", handleLogin)
	routerapi.GET("/logout", handleLogout)

	// All admin and manage routes are automatically protected by the middleware
	routerapi.GET("/admin", apiAdminInterface)
	routerapi.POST("/admin/paths/check", HandlePathCheck)
	routerapi.GET("/admin/paths/browse", HandlePathBrowse)
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
	routerapi.POST("/admin/wizard/test/tools", HandleWizardCheckTools)
	routerapi.POST("/admin/wizard/test/services", HandleWizardTestServices)
	routerapi.POST("/admin/wizard/test/dlfolders", HandleWizardCheckDownloadFolders)
	routerapi.POST("/admin/wizard/review", HandleWizardReview)
	routerapi.POST("/admin/wizard/run", HandleWizardRunJob)
	routerapi.GET("/admin/config/:configtype", adminPageConfig)
	routerapi.POST("/admin/config/:configtype/update", HandleConfigUpdate)
	routerapi.GET("/admin/seriesconfig", renderSeriesConfigPage)
	routerapi.GET("/admin/seriesconfig/add", handleSeriesConfigAdd)
	routerapi.GET("/admin/seriesconfig/edit", handleSeriesConfigEdit)
	routerapi.POST("/admin/seriesconfig/delete", handleSeriesConfigDelete)
	routerapi.POST("/admin/seriesconfig/update", handleSeriesConfigUpdate)
	routerapi.POST("/admin/seriesconfig/global", handleSeriesConfigGlobal)
	routerapi.POST("/admin/seriesconfig/create", handleSeriesConfigCreate)
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
	routerapi.GET("/admin/changelog", adminPageChangelog)
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
	routerapi.POST("/admin/cleanup/database", HandleDatabaseChecks)
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
	routerapi.GET("/admin/quality-priorities", adminPageQualityPriorities)
	routerapi.GET("/admin/quality-priorities/editor", HandleQualityPrioritiesEditor)
	routerapi.POST("/admin/quality-priorities/generate", HandleQualityPrioritiesGenerate)
	routerapi.POST("/admin/quality-priorities/save", HandleQualityPrioritiesSave)
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
	routerapi.GET("/admin/statistics/plays", webStatisticsPlays)
	routerapi.GET("/admin/statistics/jobs", webStatisticsJobs)
	routerapi.GET("/admin/statistics/quality", webStatisticsQuality)

	routerapi.GET("/admin/regex-tester", adminPageRegexTester)
	routerapi.POST("/admin/regex-tester/test", HandleRegexTesting)
	routerapi.POST("/admin/regex-tester/pattern", HandleRegexPattern)
	routerapi.GET("/admin/naming-generator", adminPageNamingGenerator)
	routerapi.GET("/admin/naming-generator/fields", HandleNamingFieldsForType)
	routerapi.POST("/admin/naming-generator/check", HandleNamingCheck)

	routerapi.GET("/admin/database/:tablename", adminPageDatabase)
	routerapi.GET("/admin/grid/:grid", adminPageGrid)
	routerapi.GET("/admin/queue/partial", renderQueuePartial)
	routerapi.GET("/admin/dashboard/cards", dashboardCardsPartial)
	routerapi.GET("/admin/wanted", renderWantedPage)
	routerapi.GET("/admin/wanted/partial", renderWantedPartial)
	routerapi.GET("/admin/wanted/tab", renderWantedTab)

	// Calendar routes
	routerapi.GET("/admin/calendar", CalendarPageHandler)
	routerapi.GET("/admin/library", LibraryPage)
	routerapi.GET("/admin/help", HelpPage)
	routerapi.GET("/admin/users", UsersPage)
	routerapi.GET("/admin/requests", RequestsPage)
	routerapi.GET("/admin/nowplaying", NowPlayingPage)
	routerapi.GET("/admin/reports", ReportsPage)
	routerapi.GET("/admin/exclusions", ExclusionsPage)
	routerapi.POST("/admin/exclusions/delete", HandleExclusionDelete)
	routerapi.GET("/admin/exclusions/search", HandleExclusionSearch)
	routerapi.POST("/admin/exclusions/add", HandleExclusionAdd)
	routerapi.POST("/admin/reports/resolve", HandleReportResolve)
	routerapi.GET("/admin/library/download/:type/:id", HandleLibraryDownload)
	routerapi.GET("/admin/library/trailer", HandleLibraryTrailer)
	routerapi.GET("/admin/library/extras", HandleLibraryExtras)
	routerapi.GET("/admin/library/collection", HandleLibraryCollection)
	routerapi.POST("/admin/library/watched", HandleLibraryWatched)
	routerapi.POST("/admin/library/watched-entry", HandleLibraryWatchedEntry)
	routerapi.POST("/admin/library/continue-drop", HandleLibraryContinueDrop)
	routerapi.POST("/admin/library/collection/add", HandleLibraryCollectionAdd)
	routerapi.GET("/admin/library/download-entry/:type/:id", HandleLibraryDownloadEntry)
	routerapi.POST("/admin/library/report", HandleLibraryReport)
	routerapi.GET("/admin/nowplaying/list", HandleNowPlayingList)
	routerapi.POST("/admin/nowplaying/stop", HandleNowPlayingStop)
	routerapi.POST("/admin/account/settings", HandleAccountSettings)
	routerapi.POST("/admin/account/setting", HandleAccountSetting)
	routerapi.GET("/admin/account/notifications", HandleAccountNotifications)
	routerapi.POST("/admin/account/notifications/dismiss", HandleAccountNotificationDismiss)
	routerapi.POST("/admin/account/email", HandleAccountEmail)
	routerapi.POST("/admin/account/messages", HandleAccountMessages)
	routerapi.POST("/admin/account/push-test", HandleAccountPushTest)
	routerapi.POST("/admin/library/play/progress", HandlePlayerProgress)
	routerapi.POST("/admin/requests/approve", HandleRequestApprove)
	routerapi.POST("/admin/requests/reject", HandleRequestReject)
	routerapi.POST("/admin/users/save", HandleUserSave)
	routerapi.POST("/admin/users/delete", HandleUserDelete)
	routerapi.POST("/admin/users/password", HandleUserPassword)
	routerapi.GET("/admin/users/activity", HandleUserActivity)
	routerapi.GET("/admin/account", AccountPage)
	routerapi.POST("/admin/account", HandleAccountPassword)
	routerapi.GET("/admin/library/grid", HandleLibraryGrid)
	routerapi.GET("/admin/library/item", HandleLibraryItem)
	routerapi.GET("/admin/library/random", HandleLibraryRandom)
	routerapi.GET("/admin/library/filters", HandleLibraryFilters)
	routerapi.GET("/admin/library/links", HandleLibraryLinks)
	routerapi.GET("/admin/library/playlist", HandleLibraryPlaylist)
	routerapi.GET("/admin/library/playall", HandleLibraryPlayAll)
	routerapi.GET("/admin/library/playlists", HandlePlaylists)
	routerapi.GET("/admin/library/playlists/view", HandlePlaylistView)
	routerapi.GET("/admin/library/playlists/play", HandlePlaylistPlay)
	routerapi.POST("/admin/library/playlists/save", HandlePlaylistSave)
	routerapi.POST("/admin/library/playlists/delete", HandlePlaylistDelete)
	routerapi.POST("/admin/library/playlists/add", HandlePlaylistAdd)
	routerapi.POST("/admin/library/playlists/favorite", HandleLibraryFavorite)
	routerapi.POST("/admin/library/playlists/remove", HandlePlaylistRemove)
	routerapi.POST("/admin/library/playlists/move", HandlePlaylistMove)
	routerapi.GET("/admin/library/cover/:type/:id", HandleLibraryCover)
	routerapi.GET("/admin/library/artist-image/:id", HandleLibraryArtistImage)
	routerapi.GET("/admin/library/read/:id/*name", HandleLibraryRead)
	routerapi.POST("/admin/library/read/progress", HandleLibraryReadProgress)
	routerapi.GET("/admin/library/thumb/:kind/:id", HandleLibraryThumb)
	routerapi.POST("/admin/library/delete", HandleLibraryDelete)
	routerapi.POST("/admin/library/delete-part", HandleLibraryDeletePart)
	routerapi.GET("/admin/library/play/speedtest", HandlePlayerSpeedTest)
	routerapi.GET("/admin/library/play/book", HandlePlayerBook)
	routerapi.GET("/admin/library/play/:kind/:id/info", HandlePlayerInfo)
	routerapi.GET("/admin/library/play/:kind/:id/stream", HandlePlayerStream)
	routerapi.GET("/admin/library/play/:kind/:id/hls", HandlePlayerHLSStart)
	routerapi.GET("/admin/library/play/:kind/:id/sub/:sid", HandlePlayerSub)
	routerapi.GET("/admin/library/play/hls/:sid/:file", HandlePlayerHLSFile)
	routerapi.GET("/admin/calendar/content", CalendarContentHandler)
	routerapi.GET("/admin/calendar/events", CalendarEventsHandler)
	routerapi.GET("/admin/calendar/day-events", CalendarDayEventsHandler)
	routerapi.GET("/admin/calendar/ical", CalendarICalHandler)

	// Metadata search routes
	routerapi.GET("/admin/metadata-search", MetadataSearchPage)
	routerapi.POST("/admin/search/movies", SearchMovieMetadata)
	routerapi.POST("/admin/search/series", SearchSeriesMetadata)
	routerapi.POST("/admin/search/books", SearchBookMetadata)
	routerapi.POST("/admin/search/audiobooks", SearchAudiobookMetadata)
	routerapi.POST("/admin/search/music", SearchMusicMetadata)
	routerapi.POST("/admin/search/music/series", SearchMusicSeriesMetadata)
	routerapi.POST("/admin/search/music/artist", SearchMusicArtists)
	routerapi.POST("/admin/search/music/artist-releases", ListArtistReleasesForSelection)
	routerapi.POST("/admin/discover/series-albums", DiscoverSeriesAlbumsByName)
	routerapi.POST("/admin/add/movie", requestable("movie", AddMovieToDatabase))
	routerapi.POST("/admin/add/series", requestable("series", AddSeriesToDatabase))
	routerapi.POST("/admin/add/book", requestable("book", AddBookToDatabase))
	routerapi.POST("/admin/add/audiobook", requestable("audiobook", AddAudiobookToDatabase))
	routerapi.POST("/admin/add/movie/manual", requestable("movie", AddMovieManual))
	routerapi.POST("/admin/add/series/manual", requestable("series", AddSeriesManual))
	routerapi.POST("/admin/add/book/manual", requestable("book", AddBookManual))
	routerapi.POST("/admin/add/audiobook/manual", requestable("audiobook", AddAudiobookManual))
	routerapi.POST("/admin/add/book/author", AddBooksByAuthor)
	routerapi.POST("/admin/add/audiobook/author", AddAudiobooksByAuthor)
	routerapi.POST("/admin/add/album", requestable("album", AddAlbumToDatabase))
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
	routerapi.GET("/admin/translations", HandleTranslationsEditor)
	routerapi.POST("/admin/translations", HandleTranslationSave)
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

// AddAllRoutes sets up HTTP routes for the "all" API endpoints.
// It configures routes for feeds, data scanning, and search operations
// that apply to every configured media type (movies, series, books,
// audiobooks, and music - not just movies/series, since utils.AllJobs with
// MediaTypeAll iterates config.RangeSettingsMedia with no type filter).
// All routes require API key authentication.
func AddAllRoutes(rg *gin.RouterGroup) {
	rg.Use(checkauth)
	{
		rg.GET("/feeds", apiAllGetFeeds)
		rg.GET("/data", apiAllGetData)

		routerallsearch := rg.Group("/search")
		{
			routerallsearch.GET("/rss", apiAllGetRss)
			routerallsearch.GET("/missing/full", apiAllGetMissingFull)
			routerallsearch.GET("/missing/inc", apiAllGetMissingInc)
			routerallsearch.GET("/upgrade/full", apiAllGetUpgradeFull)
			routerallsearch.GET("/upgrade/inc", apiAllGetUpgradeInc)
		}
	}
}

func AddMoviesRoutes(routermovies *gin.RouterGroup) {
	routermovies.Use(checkauth)
	{
		routermovies.GET("/all/refresh", apirefreshMoviesInc)
		routermovies.GET("/all/refreshall", apirefreshMovies)
		routermovies.GET("/refresh/:id", apirefreshMovie)

		routermovies.GET("/unmatched", apiMovieListUnmatched)
		routermovies.GET("/", apiMovieList)
		routermovies.POST("/", updateDBMovie)
		routermovies.DELETE("/:id", apiMovieDelete)

		routermovies.POST("/list/", updateMovie)

		routermovies.GET("/list/:name", apiMovieListGet)
		routermovies.GET("/metadata/:imdb", apiMovieMetadataGet)
		routermovies.DELETE("/list/:id", movieDeleteList)

		routermovies.GET("/job/:job", apimoviesAllJobs)
		routermovies.GET("/job/:job/:name", apimoviesJobs)

		routermovies.GET("/rss/search/list/:group", apiMoviesRssSearchList)

		routermoviessearch := routermovies.Group("/search")
		{
			routermoviessearch.GET("/id/:id", apimoviesSearch)
			routermoviessearch.GET("/list/:id", apimoviesSearchList)
			routermoviessearch.POST("/download/:id", apimoviesSearchDownload)
			routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
			routermoviessearch.GET("/history/clearid/:id", apiMoviesClearHistoryID)
		}
	}
}

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.Use(checkauth)
	{
		routerseries.GET("/", apiSeriesGet)
		routerseries.POST("/", updateDBSeries)
		routerseries.DELETE("/:id", apiSeriesDelete)

		routerseries.GET("/list/:name", apiSeriesListGet)
		routerseries.POST("/list/", updateSeries)
		routerseries.DELETE("/list/:id", apiSeriesListDelete)

		routerseries.GET("/metadata/:tvdb", apiSeriesMetadataGet)

		routerseries.GET("/all/refresh", apirefreshSeriesInc)
		routerseries.GET("/all/refreshall", apirefreshSeries)
		routerseries.GET("/refresh/:id", apirefreshSerie)

		routerseries.GET("/job/:job", apiseriesAllJobs)
		routerseries.GET("/job/:job/:name", apiseriesJobs)

		routerseries.GET("/unmatched", apiSeriesUnmatched)

		routerseries.GET("/episodes", apiSeriesEpisodesGet)
		routerseries.POST("/episodes", updateDBEpisode)

		routerseries.GET("/episodes/:id", apiSeriesEpisodesGetSingle)
		routerseries.DELETE("/episodes/:id", apiSeriesEpisodesDelete)
		routerseries.POST("/episodes/list/", updateEpisode)
		routerseries.GET("/episodes/list/:id", apiSeriesEpisodesListGet)
		routerseries.DELETE("/episodes/list/:id", apiSeriesEpisodesListDelete)
		routerseries.GET("/rss/search/list/:group", apiSeriesRssSearchList)

		routerseriessearch := routerseries.Group("/search")
		{
			routerseriessearch.GET("/id/:id", apiSeriesSearch)
			routerseriessearch.GET("/id/:id/:season", apiSeriesSearchSeason)
			routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
			routerseriessearch.GET("/history/clearid/:id", apiSeriesClearHistoryID)
		}

		routerseriessearchrss := routerseries.Group("/searchrss")
		{
			routerseriessearchrss.GET("/id/:id", apiSeriesSearchRSS)
			routerseriessearchrss.GET("/id/:id/:season", apiSeriesSearchRSSSeason)
			routerseriessearchrss.GET("/list/id/:id", apiSeriesSearchRSSList)
			routerseriessearchrss.GET("/seasons/id/:id", apiSeriesSearchRSSSeasons)
		}

		routerseriesepisodessearch := routerseries.Group("/episodes/search")
		{
			routerseriesepisodessearch.GET("/id/:id", apiSeriesEpisodeSearch)
			routerseriesepisodessearch.GET("/list/:id", apiSeriesEpisodeSearchList)
			routerseriesepisodessearch.POST("/download/:id", apiSeriesEpisodeSearchDownload)
		}
	}
}

// AddBooksRoutes adds routes for book management.
func AddBooksRoutes(routerbooks *gin.RouterGroup) {
	routerbooks.Use(checkauth)
	{
		routerbooks.GET("/", apiBooksDBList)
		routerbooks.GET("/list/:name", apiBooksListGet)
		routerbooks.DELETE("/:id", apiBookDelete)

		routerbooks.GET("/refresh/:id", apirefreshBook)

		routerbooks.GET("/job/:job", apiBooksAllJobs)
		routerbooks.GET("/job/:job/:name", apiBooksJobs)

		routerbooks.GET("/feeds/date/:name/:listname", apiBooksFeedsDate)

		routerbooks.GET("/rss/search/list/:group", apiBooksRssSearchList)

		routerbookssearch := routerbooks.Group("/search")
		{
			routerbookssearch.GET("/list/:id", apiBooksSearchList)
			routerbookssearch.GET("/missing/:name", apiBooksSearchMissing)
			routerbookssearch.GET("/history/clear/:name", apiBooksClearHistoryName)
			routerbookssearch.GET("/history/clearid/:id", apiBooksClearHistoryID)
			routerbookssearch.GET("/authors/missing/:name", apiBooksSearchAuthorsMissing)
			routerbookssearch.GET("/authors/upgrade/:name", apiBooksSearchAuthorsUpgrade)
		}
	}
}

// AddAudiobooksRoutes adds routes for audiobook management.
func AddAudiobooksRoutes(routeraudiobooks *gin.RouterGroup) {
	routeraudiobooks.Use(checkauth)
	{
		routeraudiobooks.GET("/all/refresh", apirefreshAudiobooksInc)
		routeraudiobooks.GET("/all/refreshall", apirefreshAudiobooks)
		routeraudiobooks.GET("/refresh/:id", apirefreshAudiobook)

		routeraudiobooks.GET("/tag/all", apiRetagAllAudiobooks)
		routeraudiobooks.GET("/tag/author/:id", apiRetagAuthorAudiobooks)
		routeraudiobooks.GET("/tag/:id", apiRetagAudiobook)

		routeraudiobooks.GET("/", apiAudiobooksDBList)
		routeraudiobooks.GET("/list/:name", apiAudiobooksListGet)
		routeraudiobooks.DELETE("/:id", apiAudiobookDelete)

		routeraudiobooks.GET("/job/:job", apiAudiobooksAllJobs)
		routeraudiobooks.GET("/job/:job/:name", apiAudiobooksJobs)

		routeraudiobooks.GET("/author/add/:name/:listname", apiAudiobooksAddAuthor)
		routeraudiobooks.GET("/rss/search/list/:group", apiAudiobooksRssSearchList)

		routeraudiobookssearch := routeraudiobooks.Group("/search")
		{
			routeraudiobookssearch.GET("/list/:id", apiAudiobooksSearchList)
			routeraudiobookssearch.GET("/missing/:name", apiAudiobooksSearchMissing)
			routeraudiobookssearch.GET("/history/clear/:name", apiAudiobooksClearHistoryName)
			routeraudiobookssearch.GET("/history/clearid/:id", apiAudiobooksClearHistoryID)
			routeraudiobookssearch.GET("/authors/missing/:name", apiAudiobooksSearchAuthorsMissing)
			routeraudiobookssearch.GET("/authors/upgrade/:name", apiAudiobooksSearchAuthorsUpgrade)
		}
	}
}

// AddMusicRoutes adds routes for music/album management.
func AddMusicRoutes(routermusic *gin.RouterGroup) {
	routermusic.Use(checkauth)
	{
		routermusic.GET("/all/refresh", apirefreshMusicInc)
		routermusic.GET("/all/refreshall", apirefreshMusic)
		routermusic.GET("/refresh/:id", apirefreshAlbum)

		routermusic.GET("/tag/all", apiRetagAllAlbums)
		routermusic.GET("/tag/artist/:id", apiRetagArtistAlbums)
		routermusic.GET("/tag/:id", apiRetagAlbum)

		routermusic.GET("/", apiMusicDBList)
		routermusic.GET("/list/:name", apiMusicListGet)
		routermusic.DELETE("/:id", apiMusicDelete)

		// Artist routes
		routermusic.GET("/artists", apiArtistsList)
		routermusic.GET("/artist/add/:name/:listname", apiMusicAddArtist)

		routermusic.GET("/job/:job", apiMusicAllJobs)
		routermusic.GET("/job/:job/:name", apiMusicJobs)

		routermusic.GET("/feeds/date/:name/:listname", apiMusicFeedsDate)

		routermusic.GET("/rss/search/list/:group", apiMusicRssSearchList)

		routermusic.GET("/discover/series/artist/:id", apiMusicDiscoverSeriesForArtist)

		routermusicsearch := routermusic.Group("/search")
		{
			routermusicsearch.GET("/list/:id", apiMusicSearchList)
			routermusicsearch.GET("/missing/:name", apiMusicSearchMissing)
			routermusicsearch.GET("/history/clear/:name", apiMusicClearHistoryName)
			routermusicsearch.GET("/history/clearid/:id", apiMusicClearHistoryID)
			routermusicsearch.GET("/artists/missing/:name", apiMusicSearchArtistsMissing)
			routermusicsearch.GET("/artists/upgrade/:name", apiMusicSearchArtistsUpgrade)
		}
	}
}

// AddClientRoutes adds what outside apps and devices use, each while its
// serve_* setting is on: WebDAV (/dav), OPDS (/opds), the Subsonic API
// (/subsonic), the Jellyfin API (/jellyfin) and the DLNA server (/dlna;
// StartDLNAAnnouncements announces it). They log in themselves; DLNA
// answers local devices only.
func AddClientRoutes(router *gin.Engine) {
	davOn := serveEnabled(func(g *config.GeneralConfig) bool { return g.ServeWebDAV })

	// OPTIONS needs no login: Windows asks the server root and the share
	// before it sends one, and gives up when they don't speak WebDAV.
	router.OPTIONS("/", davOn, HandleWebDAVOptions)
	router.OPTIONS(davPrefix, davOn, HandleWebDAVOptions)
	router.OPTIONS(davPrefix+"/*path", davOn, HandleWebDAVOptions)

	dav := router.Group(davPrefix, davOn, basicAuth("WebDAV"))
	for _, m := range []string{
		http.MethodGet, http.MethodHead, http.MethodPut, http.MethodPost, http.MethodDelete,
		"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK",
	} {
		dav.Handle(m, "", HandleWebDAV)
		dav.Handle(m, "/*path", HandleWebDAV)
	}

	opds := router.Group("/opds", serveEnabled(func(g *config.GeneralConfig) bool { return g.ServeOPDS }), basicAuth("OPDS"))
	opds.GET("", HandleOPDSRoot)
	opds.GET("/new", HandleOPDSNew)
	opds.GET("/authors", HandleOPDSAuthors)
	opds.GET("/author/:id", HandleOPDSAuthor)
	opds.GET("/group/:name", HandleOPDSGroup)
	opds.GET("/search", HandleOPDSSearch)
	opds.GET("/search.xml", HandleOPDSSearchDescription)
	opds.GET("/cover/:id", HandleOPDSCover)
	opds.GET("/file/:id/*name", HandleOPDSFile)

	subsonic := router.Group("/subsonic/rest", serveEnabled(func(g *config.GeneralConfig) bool { return g.ServeSubsonic }), i18nMiddleware)
	subsonic.GET("/:method", HandleSubsonic)
	subsonic.POST("/:method", HandleSubsonic)

	router.Any("/jellyfin/*path", serveEnabled(func(g *config.GeneralConfig) bool { return g.ServeJellyfin }), HandleJellyfin)

	dlna := router.Group("/dlna", serveEnabled(func(g *config.GeneralConfig) bool { return g.ServeDLNA }), dlnaGuard)
	dlna.GET("/device.xml", HandleDLNADevice)
	dlna.GET("/cd.xml", HandleDLNACDSCPD)
	dlna.GET("/cm.xml", HandleDLNACMSCPD)
	dlna.StaticFile("/icon.png", "./static/img/icons/icon-48x48.png")
	dlna.POST("/cd/control", HandleDLNACDControl)
	dlna.POST("/cm/control", HandleDLNACMControl)

	for _, m := range []string{"SUBSCRIBE", "UNSUBSCRIBE"} {
		dlna.Handle(m, "/cd/event", HandleDLNAEvent)
		dlna.Handle(m, "/cm/event", HandleDLNAEvent)
	}

	dlna.GET("/res/:kind/:id/*name", HandleDLNAResource)
	dlna.HEAD("/res/:kind/:id/*name", HandleDLNAResource)
	dlna.GET("/cover/:type/:id", HandleDLNACover)
}
