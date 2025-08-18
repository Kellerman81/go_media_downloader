package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	// "github.com/goccy/go-json".

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/PuerkitoBio/goquery"
	gin "github.com/gin-gonic/gin"
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
		routerapi.GET("/db/close", apiDBClose)
		routerapi.GET("/db/integrity", apiDBIntegrity)
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
	routerapi.GET("/admin/:configtype", adminPageConfig)
	routerapi.POST("/admin/:configtype/update", HandleConfigUpdate)
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

	// New Management Tools Routes
	routerapi.GET("/admin/media-cleanup", adminPageMediaCleanup)
	routerapi.POST("/admin/cleanup", HandleMediaCleanup)
	routerapi.GET("/admin/missing-episodes", adminPageMissingEpisodes)
	routerapi.POST("/admin/missing-episodes", HandleMissingEpisodes)
	routerapi.GET("/admin/log-analysis", adminPageLogAnalysis)
	routerapi.POST("/admin/log-analysis", HandleLogAnalysis)
	routerapi.POST("/admin/log-analysis/export", HandleLogAnalysis)
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

	routerapi.GET("/admin/regex-tester", adminPageRegexTester)
	routerapi.POST("/admin/regex-tester/test", HandleRegexTesting)
	routerapi.GET("/admin/naming-generator", adminPageNamingGenerator)
	routerapi.GET("/admin/naming-generator/fields/:type", HandleNamingFieldsForType)
	routerapi.POST("/admin/naming-generator/preview", HandleNamingPreview)
	routerapi.POST("/admin/naming-generator/verify", HandleNamingVerify)

	routerapi.GET("/admin/database/:tablename", adminPageDatabase)
	routerapi.GET("/admin/grid/:grid", adminPageGrid)

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

	routerapi.Any("/manage/media/form/:typev", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "media_main_"+ctx.Param("typev")+"_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[3]] = true
		}
		form := renderMediaForm(ctx.Param("typev"), &config.MediaTypeConfig{Name: "new" + strconv.Itoa(len(formKeys))}, getCSRFToken(ctx))
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/downloader/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "downloader_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderDownloaderForm(&config.DownloaderConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/lists/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "lists_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderListsForm(&config.ListsConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/indexers/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "indexers_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderIndexersForm(&config.IndexersConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/paths/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "paths_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderPathsForm(&config.PathsConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/notification/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "notifications_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderNotificationForm(&config.NotificationConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/regex/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "regex_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderRegexForm(&config.RegexConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/quality/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "quality_main_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[2]] = true
		}
		form := renderQualityForm(&config.QualityConfig{Name: "new" + strconv.Itoa(len(formKeys))}, getCSRFToken(ctx))
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/qualityreorder/form/:typev", func(ctx *gin.Context) {
		var count int
		a, err := goquery.NewDocumentFromReader(ctx.Request.Body)
		if err == nil {
			a.Find("#qualityContainer").Children().Each(
				func(i int, s *goquery.Selection) {
					s.Find(".qualityreorder").Each(func(i int, s *goquery.Selection) {
						s.Find("array-item card").Each(
							func(i int, s *goquery.Selection) {
								count++
							},
						)
					})
				},
			)
		}
		form := renderQualityReorderForm(count, ctx.Param("typev"), &config.QualityReorderConfig{})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/qualityindexer/form/:typev", func(ctx *gin.Context) {
		var count int
		a, err := goquery.NewDocumentFromReader(ctx.Request.Body)
		if err == nil {
			a.Find("#qualityContainer").Children().Each(
				func(i int, s *goquery.Selection) {
					s.Find(".qualityindexer").Each(func(i int, s *goquery.Selection) {
						s.Find("array-item card").Each(
							func(i int, s *goquery.Selection) {
								count++
							},
						)
					})
				},
			)
		}
		form := renderQualityIndexerForm(count, ctx.Param("typev"), &config.QualityIndexerConfig{})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/scheduler/form", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, "scheduler_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[1]] = true
		}
		form := renderSchedulerForm(&config.SchedulerConfig{Name: "new" + strconv.Itoa(len(formKeys))})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/mediadata/form/:prefix/:configv", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_data"
		logger.Logtype("info", 1).Str("prefix", prefix).Msg("prefix")
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			logger.Logtype("info", 1).Str("key", key).Msg("key")
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			logger.Logtype("info", 1).Str("splitted", fmt.Sprintf("%v", strings.Split(key, "_"))).Msg("key")
			formKeys[strings.Split(key, "_")[4]] = true
		}
		jv, _ := json.Marshal(formKeys)
		logger.Logtype("info", 1).Str("keys", string(jv)).Msg("keys")
		form := renderMediaDataForm(prefix, len(formKeys), &config.MediaDataConfig{})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/mediaimport/form/:prefix/:configv", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_dataimport"
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_TemplatePath")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[4]] = true
		}
		form := renderMediaDataImportForm(prefix, len(formKeys), &config.MediaDataImportConfig{})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/medianotification/form/:prefix/:configv", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_notification"
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_MapNotification")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[4]] = true
		}
		form := renderMediaNotificationForm(prefix, len(formKeys), &config.MediaNotificationConfig{})
		var buf strings.Builder
		form.Render(&buf)
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, buf.String())
	})
	routerapi.Any("/manage/medialists/form/:prefix/:configv", func(ctx *gin.Context) {
		if err := ctx.Request.ParseForm(); err != nil {
			ctx.String(http.StatusOK, "")
			return
		}
		prefix := "media_" + ctx.Param("prefix") + "_" + ctx.Param("configv") + "_lists"
		formKeys := make(map[any]bool)
		for key := range ctx.Request.PostForm {
			if (strings.Contains(key, "_Name")) == false || (strings.Contains(key, prefix+"_")) == false {
				continue
			}
			formKeys[strings.Split(key, "_")[4]] = true
		}
		form := renderMediaListsForm(prefix, len(formKeys), &config.MediaListsConfig{})
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

	f, err := os.Create("./temp/heapdump")
	if err != nil {
		panic(err)
	}

	debug.WriteHeapDump(f.Fd())

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
// @Success      200  {object}  Jsondata{data=map[string]worker.Job}
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
	sendJSONResponse(ctx, http.StatusOK, apiexternal.GetTraktAuthURL())
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
	apiexternal.SetTraktToken(apiexternal.GetTraktAuthToken(code))

	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})
	sendJSONResponse(ctx, http.StatusOK, apiexternal.GetTraktToken())
}

// @Summary      Trakt Get List (Auth Test)
// @Description  Get User List
// @Tags         general
// @Param        user  path      string  true  "Trakt Username"
// @Param        list  path      string  true  "List Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondataerror{data=[]apiexternal.TraktUserList}
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

	lim := "10"
	list, err := apiexternal.GetTraktUserList(user, listParam, "movie,show", &lim)
	ctx.JSON(http.StatusOK, gin.H{"data": list, "error": err})
}

// @Summary      Refresh Slugs
// @Description  Regenerates Slugs
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/slug [get].
func apiDBRefreshSlugs(ctx *gin.Context) {
	dbmovies := database.QueryDbmovie(database.Querywithargs{})
	var slug string
	for idx := range dbmovies {
		slug = logger.StringToSlug(dbmovies[idx].Title)
		database.ExecN("update dbmovies set slug = ? where id = ?", slug, &dbmovies[idx].ID)
	}

	dbmoviestitles := database.QueryDbmovieTitle(database.Querywithargs{})
	for idx := range dbmoviestitles {
		slug = logger.StringToSlug(dbmoviestitles[idx].Title)
		database.ExecN(
			"update dbmovie_titles set slug = ? where id = ?",
			slug,
			&dbmoviestitles[idx].ID,
		)
	}

	dbserie := database.QueryDbserie(database.Querywithargs{})
	for idx := range dbserie {
		slug = logger.StringToSlug(dbserie[idx].Seriename)
		database.ExecN("update dbseries set slug = ? where id = ?", slug, &dbserie[idx].ID)
	}

	dbserietitles := database.QueryDbserieAlternates(database.Querywithargs{})
	for idx := range dbserietitles {
		slug = logger.StringToSlug(dbserietitles[idx].Title)
		database.ExecN(
			"update dbserie_alternates set slug = ? where id = ?",
			slug,
			&dbserietitles[idx].ID,
		)
	}
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
	parse := parser.ParseFile(getcfg.Name, false, false, cfgp, -1)
	// parse := parser.NewFileParser(getcfg.Name, cfgp, false, -1)
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)
	err := parser.GetDBIDs(parse, cfgp, true)
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
	parse := parser.ParseFile(getcfg.Path, true, false, cfgp, -1)
	// parse := parser.NewFileParser(filepath.Base(getcfg.Path), cfgp, false, -1)
	parse.File = getcfg.Path
	parser.ParseVideoFile(parse, config.GetSettingsQuality(getcfg.Quality))
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)
	parser.GetDBIDs(parse, cfgp, true)
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
	// scheduler.QueueData.Stop()
	// scheduler.QueueFeeds.Stop()
	// scheduler.QueueSearch.Stop()
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
	// scheduler.QueueData.Start()
	// scheduler.QueueFeeds.Start()
	// scheduler.QueueSearch.Start()
	sendSuccess(c, StrOK)
}

// @Summary      Scheduler Jobs
// @Description  Lists Planned Jobs
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]worker.jobSchedule}
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/list [get].
func apiSchedulerList(ctx *gin.Context) {
	sendJSONResponse(ctx, http.StatusOK, worker.GetSchedules())
}

// @Summary      Close DB
// @Description  Closes all database connections
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/close [get].
func apiDBClose(ctx *gin.Context) {
	database.DBClose()
	sendSuccess(ctx, StrOK)
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
		worker.StopCronWorker()
		worker.CloseWorkerPools()
	}
	backupto := "./backup/data.db." + database.GetVersion() + "." + time.Now().
		Format("20060102_150405")
	database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		worker.InitWorkerPools(
			config.GetSettingsGeneral().WorkerSearch,
			config.GetSettingsGeneral().WorkerFiles,
			config.GetSettingsGeneral().WorkerMetadata,
			config.GetSettingsGeneral().WorkerRSS,
			config.GetSettingsGeneral().WorkerIndexer,
		)
		worker.StartCronWorker()
	}
	sendSuccess(ctx, StrOK)
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

	err := database.ExecNErr("DELETE from " + tableName)
	database.ExecN("VACUUM")
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
	database.DeleteRow("qualities", logger.FilterByID, id)
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

	if counter == 0 {
		database.InsertArray(
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
		database.UpdateArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			"id != 0 and id = ?", quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex, quality.ID)
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
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll())
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
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll())
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
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll()[name])
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
	config.DeleteCfgEntry(name)
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll())
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
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll())
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
	switch left {
	case "general":
		var getcfg config.GeneralConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "downloader":
		var getcfg config.DownloaderConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case logger.StrImdb:
		var getcfg config.ImdbConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "indexer":
		var getcfg config.IndexersConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "list":
		var getcfg config.ListsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "serie":
	case "movie":
		var getcfg config.MediaTypeConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "notification":
		var getcfg config.NotificationConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "path":
		var getcfg config.PathsConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "quality":
		var getcfg config.QualityConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "regex":
		var getcfg config.RegexConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	case "scheduler":
		var getcfg config.SchedulerConfig
		if !bindJSONWithValidation(ctx, &getcfg) {
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: name, Data: getcfg})
	}
	config.WriteCfg()
	sendJSONResponse(ctx, http.StatusOK, config.GetCfgAll())
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

	sendJSONResponse(ctx, http.StatusOK, list)
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
		movie, _ := database.GetMovies(
			database.Querywithargs{Where: logger.FilterByID},
			cfg.MovieID,
		)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath, false, false, 0)
		// defer s.Close()
		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata
		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = movie.Rootpath
		m := parser.ParseFile(
			cfg.FilePath,
			true,
			true,
			cfgp,
			cfgp.GetMediaListsEntryListID(movie.Listname),
		)
		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)

		s.GenerateNamingTemplate(&orgadata2, m, &movie.DbmovieID)
		ctx.JSON(
			http.StatusOK,
			gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m},
		)
	} else {
		series, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, cfg.SerieID)
		// defer logger.ClearVar(&series)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)

		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath,
			false, false, 0,
		)
		// defer s.Close()
		to := filepath.Dir(cfg.FilePath)
		var orgadata2 structure.Organizerdata
		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = series.Rootpath

		m := parser.ParseFile(cfg.FilePath, true, true, cfgp, cfgp.GetMediaListsEntryListID(series.Listname))
		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)
		m.SerieID = series.ID
		m.DbserieID = series.DbserieID
		s.GetSeriesEpisodes(&orgadata2, m, true, s.Cfgp.Lists[orgadata2.Listid].CfgQuality)

		var firstepiid uint
		for _, entry := range m.Episodes {
			firstepiid = entry.Num1
			break
		}

		s.GenerateNamingTemplate(&orgadata2, m, &firstepiid)
		ctx.JSON(http.StatusOK, gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m})
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
	CacheType string `json:"cache_type" binding:"required"`
	Key       string `json:"key" binding:"required"`
	Value     string `json:"value" binding:"required"`
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

// @Summary      Cancel Queue Job
// @Description  Cancel a running or pending job in the queue
// @Tags         queue
// @Param        id     path     string    true  "Queue job ID"
// @Param        apikey query    string    true  "apikey"
// @Success      200    {object} gin.H{"success": bool}
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Failure      404    {object} Jsonerror
// @Router       /api/queue/cancel/{id} [delete]
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

	sendJSONResponse(ctx, http.StatusOK, gin.H{
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

	valid := false
	for _, validType := range validTypes {
		if req.CacheType == validType {
			valid = true
			break
		}
	}

	if !valid {
		sendBadRequest(ctx, "Invalid cache type. Use /api/cache/list to see available types")
		return
	}

	// Add to appropriate cache based on type
	// For simplicity, most cache types use string append
	database.AppendCache(req.Key, req.Value)

	logger.Logtype("info", 0).Str("type", req.CacheType).Str("key", req.Key).Str("value", req.Value).Msg("Manual cache entry added")
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

	valid := false
	for _, validType := range validTypes {
		if cacheType == validType {
			valid = true
			break
		}
	}

	if !valid {
		sendBadRequest(ctx, "Invalid cache type. Use /api/cache/list to see available types")
		return
	}

	// Clear the specific cache type
	database.ClearCacheType(cacheType)

	logger.Logtype("info", 0).Str("type", cacheType).Msg("Manual cache type cleared")
	sendSuccess(ctx, fmt.Sprintf("Cache type %s cleared successfully", cacheType))
}
