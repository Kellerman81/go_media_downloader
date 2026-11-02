package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DeanThompson/ginpprof"
	"github.com/Kellerman81/go_media_downloader/pkg/main/api"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/acoustid"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deezer"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discogs"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/itunes"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lastfm"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/theaudiodb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
)

// @title                       Go Media Downloader API
// @version                     1.0
// @Schemes                     http https
// @host                        localhost:9090
// @securitydefinitions.apikey  ApiKeyAuth
// @in                          query
// @name                        apikey
// @Accept                      json
// @Produce                     json.
var (
	version    string
	buildstamp string
	githash    string
)

// orDefault returns v, or def when v is 0 (setting left unset).
func orDefault(v, def int) int {
	if v == 0 {
		return def
	}

	return v
}

func initproviders() {
	clientManager := apiexternal_v2.NewClientManager()

	// Set global client manager for v2 API access
	apiexternal_v2.SetGlobalClientManager(clientManager)

	if _, exists := apiexternal_v2.GetGlobalClientManager(); !exists {
		logger.Logtype(logger.StatusWarning, 0).
			Msg("Global ClientManager not available for provider registration")
		return
	}

	// Notification providers and download clients are re-registered on
	// every config reload as well.
	providers.RegisterNotificationProviders()
	providers.RegisterDownloadClients()

	// Register book/audiobook/music metadata providers
	general := config.GetSettingsGeneral()

	// Initialize OpenLibrary provider (free, no API key required)
	if provider := openlibrary.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   orDefault(general.OpenLibraryLimiterCalls, 100),
		RateLimitSeconds: orDefault(int(general.OpenLibraryLimiterSeconds), 60),
		UserAgent:        general.UserAgent,
	}); provider != nil {
		providers.SetOpenLibrary(provider)
		logger.Logtype(logger.StatusDebug, 0).Msg("Registered OpenLibrary provider")
	}

	// Initialize Goodreads provider if API key is configured
	if apiKey := general.GoodreadsAPIKey; apiKey != "" {
		if provider := goodreads.NewProvider(apiKey); provider != nil {
			providers.SetGoodreads(provider)
			logger.Logtype(logger.StatusDebug, 0).Msg("Registered Goodreads provider")
		}
	}

	// Initialize music metadata providers according to the priority list.
	// GetMusicMetaSourcePriority returns all providers when the list is empty (backwards-compatible).
	for _, source := range config.GetMusicMetaSourcePriority() {
		switch source {
		case "musicbrainz":
			if provider := musicbrainz.NewProviderWithConfig(base.ClientConfig{
				Timeout:          30 * time.Second,
				RateLimitCalls:   orDefault(general.MusicBrainzLimiterCalls, 1),
				RateLimitSeconds: orDefault(int(general.MusicBrainzLimiterSeconds), 1),
				UserAgent:        general.UserAgent,
			}); provider != nil {
				providers.SetMusicBrainz(provider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered MusicBrainz provider")
			}

		case "acoustid":
			// API key already validated by GetMusicMetaSourcePriority.
			if provider := acoustid.NewProviderWithConfig(base.ClientConfig{
				Timeout:          15 * time.Second,
				RateLimitCalls:   orDefault(general.AcoustIDLimiterCalls, 3),
				RateLimitSeconds: orDefault(int(general.AcoustIDLimiterSeconds), 1),
				UserAgent:        general.UserAgent,
			}, general.AcoustIDAPIKey); provider != nil {
				providers.SetAcoustID(provider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered AcoustID provider")
			}

		case "lastfm":
			// API key already validated by GetMusicMetaSourcePriority.
			if provider := lastfm.NewProvider(); provider != nil {
				providers.SetLastFM(provider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered Last.fm provider")
			}

		case "discogs":
			var dProvider *discogs.Provider
			if general.DiscogsToken != "" {
				dProvider = discogs.NewProviderWithToken(general.DiscogsToken)
			} else {
				dProvider = discogs.NewProvider()
			}

			if dProvider != nil {
				providers.SetDiscogs(dProvider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered Discogs provider")
			}

		case "deezer":
			if dzProvider := deezer.NewProvider(); dzProvider != nil {
				providers.SetDeezer(dzProvider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered Deezer provider")
			}

		case "theaudiodb":
			if tadbProvider := theaudiodb.NewProvider(); tadbProvider != nil {
				providers.SetTheAudioDB(tadbProvider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered TheAudioDB provider")
			}

		case "itunes":
			if itProvider := itunes.NewProvider(); itProvider != nil {
				providers.SetITunes(itProvider)
				logger.Logtype(logger.StatusDebug, 0).Msg("Registered iTunes provider")
			}
		}
	}

	// Initialize Audnex provider (region-agnostic, provides chapter data)
	if provider := audnex.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   orDefault(general.AudnexLimiterCalls, 10),
		RateLimitSeconds: orDefault(int(general.AudnexLimiterSeconds), 1),
		UserAgent:        general.UserAgent,
	}); provider != nil {
		providers.SetAudnex(provider)
		logger.Logtype(logger.StatusDebug, 0).Msg("Registered Audnex provider")
	}

	// Initialize Audible provider for US region (default)
	// Region-specific providers will be created on-demand based on media type configs
	if provider := audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   orDefault(general.AudibleLimiterCalls, 1),
		RateLimitSeconds: orDefault(int(general.AudibleLimiterSeconds), 2),
		UserAgent:        general.UserAgent,
	}, audible.RegionUS); provider != nil {
		providers.SetAudible("us", provider)
		logger.Logtype(logger.StatusDebug, 0).
			Str("region", "us").
			Msg("Registered Audible provider (default US region)")
	}
}

// main initializes and starts the Go Media Downloader application server.
// It sets up configuration, database connections, worker pools, schedulers,
// external API clients, and the web server with graceful shutdown handling.
// cleanupDuplicatesArgs scans the command line for the one-off duplicate-cleanup
// flags. runCleanup is true when --cleanup-duplicates is present; apply is true
// when --apply is also present (otherwise it is a dry run).
func cleanupDuplicatesArgs() (runCleanup, apply bool) {
	for _, a := range os.Args[1:] {
		switch a {
		case "--cleanup-duplicates", "-cleanup-duplicates":
			runCleanup = true
		case "--apply", "-apply":
			apply = true
		}
	}

	return runCleanup, apply
}

// startConfigWatcher watches the config file's directory and reloads the
// settings when config.toml is written.
func startConfigWatcher() (*fsnotify.Watcher, error) {
	st, err := os.Lstat(config.Configfile)
	if err != nil {
		return nil, err
	}

	if st.IsDir() {
		return nil, fmt.Errorf("%q is a directory, not a file", config.Configfile)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating a new watcher: %w", err)
	}

	// Watch the directory, not the file itself.
	if err := watcher.Add(filepath.Dir(config.Configfile)); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("%q: %w", config.Configfile, err)
	}

	go func() {
		for {
			select {
			case err, ok := <-watcher.Errors:
				if !ok { // Channel was closed (i.e. Watcher.Close() was called).
					return
				}

				fmt.Printf("ERROR: %s", err) //nolint:forbidigo // logger not initialized yet

			case e, ok := <-watcher.Events:
				if !ok { // Channel was closed (i.e. Watcher.Close() was called).
					return
				}

				if strings.Contains(e.Name, "config.toml") &&
					(e.Has(fsnotify.Write) || e.Has(fsnotify.Create)) {
					// Create: config.WriteCfg/UpdateCfgEntry replace the file via rename.
					config.Loadallsettings(true)
					parser.GenerateAllQualityPriorities()
					parser.GenerateCutoffPriorities()
					scheduler.Reload()
					providers.ReloadClientProviders()
					providers.RegisterNotificationProviders()
				}
			}
		}
	}()

	return watcher, nil
}

func main() {
	// debug.SetGCPercent(30)
	os.Mkdir("./temp", 0o777)

	err := config.LoadCfgDB(false)
	if err != nil {
		fmt.Println(
			"Fatal: failed to load config:",
			err,
		)
		os.Exit(1)
	}

	if config.GetSettingsGeneral().EnableFileWatcher {
		// A watcher failure only loses config hot-reload; keep running.
		if watcher, err := startConfigWatcher(); err != nil {
			fmt.Fprintf( //nolint:forbidigo // logger not initialized yet
				os.Stderr,
				"ERROR: config file watcher disabled: %s\n",
				err,
			)
		} else {
			defer watcher.Close()
		}
	}

	database.InitCache()

	general := config.GetSettingsGeneral()
	worker.InitWorkerPools(
		general.WorkerSearch,
		general.WorkerFiles,
		general.WorkerMetadata,
		general.WorkerRSS,
		general.WorkerIndexer,
		general.WorkerParse,
	)

	// Initialize syncops single-writer system
	syncops.InitSyncOps()

	// Register additional SyncMaps with syncops for architectural consistency
	syncops.RegisterSyncMap(syncops.MapTypeStructEmpty, importfeed.GetImportJobRunning())

	// Register API client SyncMaps
	logger.InitLogger(logger.Config{
		LogLevel:      general.LogLevel,
		LogFileSize:   general.LogFileSize,
		LogFileCount:  general.LogFileCount,
		LogCompress:   general.LogCompress,
		LogToFileOnly: general.LogToFileOnly,
		LogColorize:   general.LogColorize,
		TimeFormat:    general.TimeFormat,
		TimeZone:      general.TimeZone,
		// LogZeroValues: general.LogZeroValues,
	})
	logger.Logtype("info", 0).Msg("Starting go_media_downloader")
	logger.Logtype("info", 0).Msg("Version: " + version + " " + githash)
	logger.Logtype("info", 0).Msg("Build Date: " + buildstamp)
	logger.Logtype("info", 0).Msg("Programmer: kellerman81")

	if !strings.EqualFold(general.LogLevel, logger.StrDebug) {
		logger.Logtype("info", 0).Msg("Hint: Set Loglevel to Debug to see possible API Paths")
	}

	logger.Logtype("info", 0).Msg("------------------------------")

	providers.RegisterMetadataProviders()
	providers.RegisterMediaServerProviders()

	logger.Logtype("info", 0).Msg("Initialize Database")

	err = database.UpgradeDB()
	if err != nil {
		logger.Logtype("fatal", 0).Err(err).Msg("Database Upgrade Failed")
	}

	database.UpgradeIMDB()

	err = database.InitDB(general.DBLogLevel)
	if err != nil {
		logger.Logtype("fatal", 0).Err(err).Msg("Database Initialization Failed")
	}

	// Repair legacy rows that stored empty/non-numeric text in numeric columns
	// so the strict sqlite driver can scan them (idempotent; cheap on clean DBs).
	database.NormalizeNumericColumns()

	// Remove child rows whose parents are gone. The schema's ON DELETE CASCADE
	// rules never fire because FK enforcement is disabled (optional reference
	// columns store 0 instead of NULL), so orphans accumulate over time.
	if _, err := database.CleanupOrphans(false); err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Orphan cleanup incomplete")
	}

	if closed, err := database.CloseInterruptedJobHistories(); err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Closing interrupted job histories failed")
	} else if closed > 0 {
		logger.Logtype("info", 0).Int64("rows", closed).Msg("Closed job histories left open by a previous run")
	}

	err = database.InitImdbdb()
	if err != nil {
		logger.Logtype("fatal", 0).Err(err).Msg("IMDB Database Initialization Failed")
	}

	if database.DBQuickCheck() != "ok" {
		// Use error level so we can call os.Exit with the intended code (100).
		// log.Fatal would call os.Exit(1) internally and skip the explicit exit below.
		// The OS releases any pending defers (e.g. watcher.Close) when the process exits.
		logger.Logtype("error", 0).Msg("integrity check failed")
		database.DBClose()
		os.Exit(100) //nolint:gocritic // intentional early exit; OS frees resources
	}

	logger.Logtype("info", 0).Msg("Set Vars")
	// _ = html.UnescapeString("test")
	database.SetVars()

	logger.Logtype("info", 0).Msg("Generate All Quality Priorities")
	parser.GenerateAllQualityPriorities()

	logger.Logtype("info", 0).Msg("Load DB Patterns")
	parser.LoadDBPatterns()

	// One-off maintenance: remove duplicate list entries (same item under two
	// sibling lists of one media group). Run with --cleanup-duplicates for a
	// dry-run report, add --apply to actually delete. The process exits after.
	if runCleanup, apply := cleanupDuplicatesArgs(); runCleanup {
		logger.Logtype("info", 0).Bool("apply", apply).Msg("Running list duplicate cleanup")
		utils.CleanupListDuplicates(apply)
		database.DBClose()
		os.Exit(0) //nolint:gocritic // intentional early exit; OS frees resources
	}

	logger.Logtype("info", 0).Msg("Load DB Cutoff")
	parser.GenerateCutoffPriorities()

	// Surface a missing/misconfigured ffprobe or mediainfo once at startup
	// instead of as per-file errors during scans.
	parser.CheckAnalyzerPaths()

	initproviders()

	logger.Logtype("info", 0).Msg("Check Fill IMDB")

	if database.Getdatarow[uint](true, "select count() from imdb_titles") == 0 {
		utils.FillImdb()
	}

	if database.Getdatarow[uint](false, "select count() from dbmovies") == 0 {
		logger.Logtype("info", 0).Msg("Initial Fill Movies")
		utils.InitialFillMovies()
	}

	if database.Getdatarow[uint](false, "select count() from dbseries") == 0 {
		logger.Logtype("info", 0).Msg("Initial Fill Series")
		utils.InitialFillSeries()
	}

	logger.Logtype("info", 0).Msg("Range Indexers")
	providers.SyncIndexerProviders()

	worker.RegisterWorkerSyncMaps()
	logger.Logtype("info", 0).Msg("Create Cron Worker")
	worker.CreateCronWorker()

	logger.Logtype("info", 0).Msg("Inits")
	utils.Init()
	searcher.Init()

	logger.Logtype("info", 0).Msg("Refresh Cache")
	utils.Refreshcache(config.MediaTypeSeries)
	utils.Refreshcache(config.MediaTypeMovie)
	utils.Refreshcache(config.MediaTypeBook)
	utils.Refreshcache(config.MediaTypeAudiobook)
	utils.Refreshcache(config.MediaTypeMusic)
	logger.Logtype("info", 0).Msg("Starting Scheduler")
	scheduler.InitScheduler()
	worker.StartCronWorker()

	logger.Logtype("info", 0).Msg("Starting API")

	router := gin.New()
	// Recovery must come first so a panic in any handler is turned into a 500
	// response instead of crashing the connection (which surfaces as a 502 behind
	// a reverse proxy).
	router.Use(gin.Recovery(), logger.GinLogger(), logger.ErrorLogger())
	// router.Use(ginlog.SetLogger(ginlog.WithLogger(func(_ *gin.Context, l zerolog.Logger) zerolog.Logger {
	// 	return l.Output(gin.DefaultWriter).With().Logger()
	// })))

	// corsconfig := cors.DefaultConfig()
	// corsconfig.AllowHeaders = []string{"*"}
	// corsconfig.AllowOrigins = []string{"*"}
	// corsconfig.AllowMethods = []string{"*"}

	if !strings.EqualFold(general.LogLevel, logger.StrDebug) {
		gin.SetMode(gin.ReleaseMode)
	}

	// router.Use(ginlog.Logger(logger.Log), cors.New(corsconfig), gin.Recovery())
	router.Static("/static", "./static")
	// Serve the browser's default /favicon.ico request with the small app icon.
	router.StaticFile("/favicon.ico", "./static/img/icons/icon-32x32.png")

	// Root path redirect
	router.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api/")
	})

	routerapi := router.Group("/api")
	api.AddWebRoutes(routerapi)
	api.AddGeneralRoutes(routerapi)
	api.AddAllRoutes(routerapi.Group("/all"))
	api.AddMoviesRoutes(routerapi.Group("/movies"))
	api.AddSeriesRoutes(routerapi.Group("/series"))
	api.AddBooksRoutes(routerapi.Group("/books"))
	api.AddAudiobooksRoutes(routerapi.Group("/audiobooks"))
	api.AddMusicRoutes(routerapi.Group("/music"))

	// 404 handler for unmatched routes
	router.NoRoute(api.NotFoundPage)

	// Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	// router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !general.DisableSwagger {
	// 	docs.SwaggerInfo.BasePath = "/"
	// 	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// }

	if strings.EqualFold(general.LogLevel, logger.StrDebug) {
		ginpprof.Wrap(router)
	}

	// webapp.Route("/web", &web.Home{})
	// router.Handle("GET", "/web", gin.WrapH(&webapp.Handler{}))

	logger.Logtype("info", 1).Str("port", general.WebPort).Msg("Starting API Webserver on port")

	server := http.Server{
		Addr:              ":" + general.WebPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		MaxHeaderBytes:    20 << 20,
	}

	// Signal handling is set up before the listener starts so a listen
	// failure can trigger the normal shutdown path instead of leaving the
	// app running against a closed database.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	listenFailed := make(chan struct{})

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Logtype("error", 0).Err(err).Str("port", general.WebPort).Msg("listen failed - shutting down")
			close(listenFailed)
		}
	}()

	logger.Logtype("info", 1).Str("port", general.WebPort).Msg("Started API Webserver on port ")

	webPassword := general.WebAPIKey
	if webPassword == "" {
		webPassword = api.DefaultPassword
	}

	//nolint:forbidigo // user-facing startup hint, easy to spot next to the JSON logs
	fmt.Printf(
		"Web UI: http://localhost:%s/api/admin (user '%s', password '%s')\n",
		general.WebPort, api.DefaultUsername, webPassword,
	)

	exitCode := 0

	select {
	case <-quit:
	case <-listenFailed:
		exitCode = 1
	}

	logger.Logtype("info", 0).Msg("Server shutting down")

	// Stop accepting requests first so no handler touches the workers or the
	// DB while they are being torn down.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := server.Shutdown(ctx); err != nil {
		logger.Logtype("error", 0).Err(err).Msg("server shutdown")
	}
	cancel()

	worker.StopCronWorker()
	worker.CloseWorkerPools()
	utils.StopAllIRCSessions()
	syncops.Shutdown()

	logger.Logtype("info", 0).Msg("Queues stopped")

	config.Slepping(true, 5)
	database.StopCache()
	database.DBClose()
	logger.Logtype("info", 0).Msg("Databases and cache stopped")

	logger.Logtype("info", 0).Msg("Server exiting")

	if exitCode != 0 {
		os.Exit(exitCode) //nolint:gocritic // after the full shutdown path above
	}
}
