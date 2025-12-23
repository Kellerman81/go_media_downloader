package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/api"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/apprise"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/gotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushbullet"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushover"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rtorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
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

	"github.com/DeanThompson/ginpprof"
	"github.com/gin-gonic/gin"
	// webapp "github.com/maxence-charriere/go-app/v9/pkg/app"
	// ginlog "github.com/toorop/gin-logrus".
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

// parseServerURL extracts host, port, and SSL from a server URL string
// Example: "https://gotify.example.com:8080" -> ("gotify.example.com", 8080, true).
func parseServerURL(serverURL string) (host string, port int, useSSL bool) {
	// Default values
	host = serverURL
	port = 80
	useSSL = false

	// Check for SSL/HTTPS
	if strings.HasPrefix(serverURL, "https://") {
		useSSL = true
		port = 443
		serverURL = strings.TrimPrefix(serverURL, "https://")
	} else if strings.HasPrefix(serverURL, "http://") {
		serverURL = strings.TrimPrefix(serverURL, "http://")
	}

	// Extract host and port
	if strings.Contains(serverURL, ":") {
		parts := strings.Split(serverURL, ":")

		host = parts[0]
		if portNum, err := strconv.Atoi(parts[1]); err == nil {
			port = portNum
		}
	} else {
		host = serverURL
	}

	return host, port, useSSL
}

// buildBaseURL constructs a base URL from hostname, port, and SSL settings
// Example: ("localhost", 8080, false) -> "http://localhost:8080"
func buildBaseURL(hostname string, port int, useSSL bool) string {
	protocol := "http"
	if useSSL {
		protocol = "https"
	}

	return protocol + "://" + hostname + ":" + strconv.Itoa(port)
}

func initproviders() {
	clientManager := apiexternal_v2.NewClientManager()

	// Set global client manager for v2 API access
	apiexternal_v2.SetGlobalClientManager(clientManager)

	cm, exists := apiexternal_v2.GetGlobalClientManager()
	if !exists {
		logger.Logtype(logger.StatusWarning, 0).
			Msg("Global ClientManager not available for provider registration")
		return
	}

	// Register notification providers with full configuration
	config.RangeSettingsNotification(func(name string, notifCfg *config.NotificationConfig) {
		switch notifCfg.NotificationType {
		case "pushover":
			// Pushover supports NewProviderWithConfig with static rate limiting
			pushoverConfig := base.ClientConfig{
				Name:                      "pushover_" + notifCfg.Name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Pushover handles auth via form parameters
				RateLimitCalls:            300,           // Pushover allows ~10,000/month = ~300/hour conservative
				RateLimitSeconds:          3600,          // 1 hour
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				EnableStats:               true,
				UserAgent:                 "go-media-downloader/2.0",
			}
			if provider := pushover.NewProviderWithConfig(pushoverConfig, notifCfg.Apikey, notifCfg.Recipient); provider != nil {
				cm.RegisterNotificationProvider("pushover_"+notifCfg.Name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered pushover notification provider with rate limiting")
			}

		case "gotify":
			// Gotify supports NewProviderWithConfig with static rate limiting
			if notifCfg.ServerURL == "" || notifCfg.Apikey == "" {
				break
			}
			// Parse server URL to extract host, port, and SSL settings
			host, port, useSSL := parseServerURL(notifCfg.ServerURL)

			gotifyConfig := base.ClientConfig{
				Name:                      "gotify_" + name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Gotify provider handles auth internally
				RateLimitCalls:            1000,          // Gotify is self-hosted, generous limit
				RateLimitSeconds:          3600,          // 1 hour
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				EnableStats:               true,
				UserAgent:                 "go-media-downloader/2.0",
			}
			if provider := gotify.NewProviderWithConfig(gotifyConfig, host, port, notifCfg.Apikey, useSSL); provider != nil {
				cm.RegisterNotificationProvider(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered gotify notification provider with rate limiting")
			}

		case "pushbullet":
			// Pushbullet only has simple NewProvider
			if notifCfg.Apikey == "" {
				break
			}

			if provider := pushbullet.NewProvider(notifCfg.Apikey); provider != nil {
				cm.RegisterNotificationProvider(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered pushbullet notification provider")
			}

		case "apprise":
			// Apprise only has simple NewProvider
			if notifCfg.ServerURL == "" || notifCfg.AppriseURLs == "" {
				break
			}

			host, port, useSSL := parseServerURL(notifCfg.ServerURL)

			urls := strings.Split(notifCfg.AppriseURLs, ",")
			if provider := apprise.NewProvider(host, port, notifCfg.Apikey, urls, useSSL); provider != nil {
				cm.RegisterNotificationProvider(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered apprise notification provider")
			}

			// case "sendmail":
			// 	// Sendmail only has simple NewProvider
			// 	if notifCfg.SMTPServer != "" && notifCfg.SMTPFromEmail != "" &&
			// 		notifCfg.SMTPToEmail != "" {
			// 		port, _ := strconv.Atoi(notifCfg.SMTPPort)
			// 		if port == 0 {
			// 			port = 587 // Default SMTP port
			// 		}

			// 		toEmails := []string{notifCfg.SMTPToEmail}
			// 		if provider := sendmail.NewProvider(notifCfg.SMTPServer, port, notifCfg.SMTPFromEmail, toEmails, notifCfg.SMTPUsername, notifCfg.SMTPPassword); provider != nil {
			// 			cm.RegisterNotificationProvider(name, provider)
			// 			logger.Logtype(logger.StatusDebug, 0).
			// 				Str("notification", name).
			// 				Msg("Registered sendmail notification provider")
			// 		}
			// 	}
		}
	})

	// Register download client providers with full configuration
	config.RangeSettingsDownloader(func(name string, dlCfg *config.DownloaderConfig) {
		if !dlCfg.Enabled {
			return
		}

		switch dlCfg.DlType {
		case "qbittorrent":
			if dlCfg.Hostname == "" {
				break
			}

			if provider, err := qbittorrent.NewProvider(dlCfg.Hostname, dlCfg.Port, dlCfg.Username, dlCfg.Password, strings.HasPrefix(dlCfg.Hostname, "https")); err == nil &&
				provider != nil {
				providers.SetQBittorrent(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered qBittorrent download provider")
			}

		case "deluge":
			if dlCfg.Hostname == "" {
				break
			}

			baseURL := buildBaseURL(
				dlCfg.Hostname,
				dlCfg.Port,
				strings.HasPrefix(dlCfg.Hostname, "https"),
			)
			if provider := deluge.NewProvider(baseURL, dlCfg.Username, dlCfg.Password); provider != nil {
				providers.SetDeluge(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered Deluge download provider")
			}

		case "transmission":
			if dlCfg.Hostname == "" {
				break
			}

			baseURL := buildBaseURL(
				dlCfg.Hostname,
				dlCfg.Port,
				strings.HasPrefix(dlCfg.Hostname, "https"),
			)
			if provider := transmission.NewProvider(baseURL, dlCfg.Username, dlCfg.Password); provider != nil {
				providers.SetTransmission(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered Transmission download provider")
			}

		case "rtorrent":
			if dlCfg.Hostname == "" {
				break
			}

			urlBase := ""
			if provider, err := rtorrent.NewProvider(dlCfg.Hostname, dlCfg.Port, dlCfg.Username, dlCfg.Password, strings.HasPrefix(dlCfg.Hostname, "https"), urlBase); err == nil &&
				provider != nil {
				providers.SetRTorrent(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered rTorrent download provider")
			}

		case "sabnzbd":
			if dlCfg.Hostname == "" || dlCfg.Password == "" { // Password field used for API key
				break
			}

			if provider, err := sabnzbd.NewProvider(dlCfg.Hostname, dlCfg.Port, dlCfg.Password, strings.HasPrefix(dlCfg.Hostname, "https")); err == nil &&
				provider != nil {
				providers.SetSABnzbd(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered SABnzbd download provider")
			}

		case "nzbget":
			if dlCfg.Hostname == "" {
				break
			}

			if provider, err := nzbget.NewProvider(dlCfg.Hostname, dlCfg.Port, dlCfg.Username, dlCfg.Password, strings.HasPrefix(dlCfg.Hostname, "https")); err == nil &&
				provider != nil {
				providers.SetNZBGet(name, provider)
				logger.Logtype(logger.StatusDebug, 0).
					Str("downloader", name).
					Msg("Registered NZBGet download provider")
			}
		}
	})
}

// main initializes and starts the Go Media Downloader application server.
// It sets up configuration, database connections, worker pools, schedulers,
// external API clients, and the web server with graceful shutdown handling.
func main() {
	// debug.SetGCPercent(30)
	os.Mkdir("./temp", 0o777)

	err := config.LoadCfgDB(false)
	if err != nil {
		os.Exit(1)
	}

	if config.GetSettingsGeneral().EnableFileWatcher {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			fmt.Printf("creating a new watcher: %s", err)
			return
		}
		defer watcher.Close()

		// Add all files from the commandline.
		st, err := os.Lstat(config.Configfile)
		if err != nil {
			fmt.Printf("%s", err)
			return
		}

		if st.IsDir() {
			fmt.Printf("%q is a directory, not a file", config.Configfile)
			return
		}

		// Watch the directory, not the file itself.
		err = watcher.Add(filepath.Dir(config.Configfile))
		if err != nil {
			fmt.Printf("%q: %s", config.Configfile, err)
			return
		}

		// Start listening for events.
		go func() {
			for {
				select {
				// Read from Errors.
				case err, ok := <-watcher.Errors:
					if !ok { // Channel was closed (i.e. Watcher.Close() was called).
						return
					}

					fmt.Printf("ERROR: %s", err)
				// Read from Events.
				case e, ok := <-watcher.Events:
					if !ok { // Channel was closed (i.e. Watcher.Close() was called).
						return
					}

					if strings.Contains(e.Name, "config.toml") {
						if e.Has(fsnotify.Write) {
							config.Loadallsettings(true)
							utils.LoadGlobalSchedulerConfig()
							utils.LoadSchedulerConfig()
						}
					} else {
						continue
					}
				}
			}
		}()
	}

	database.InitCache()

	general := config.GetSettingsGeneral()
	worker.InitWorkerPools(
		general.WorkerSearch,
		general.WorkerFiles,
		general.WorkerMetadata,
		general.WorkerRSS,
		general.WorkerIndexer,
	)

	// Initialize syncops single-writer system
	syncops.InitSyncOps()

	// Register additional SyncMaps with syncops for architectural consistency
	syncops.RegisterSyncMap(syncops.MapTypeStructEmpty, importfeed.GetImportJobRunning())
	syncops.RegisterSyncMap(syncops.MapTypeAny, syncops.NewSyncMap[syncops.SyncAny](20))

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
		LogZeroValues: general.LogZeroValues,
	})
	logger.Logtype("info", 0).Msg("Starting go_media_downloader")
	logger.Logtype("info", 0).Msg("Version: " + version + " " + githash)
	logger.Logtype("info", 0).Msg("Build Date: " + buildstamp)
	logger.Logtype("info", 0).Msg("Programmer: kellerman81")

	if general.LogLevel != "Debug" {
		logger.Logtype("info", 0).Msg("Hint: Set Loglevel to Debug to see possible API Paths")
	}

	logger.Logtype("info", 0).Msg("------------------------------")

	apiexternal.NewOmdbClient(
		general.OmdbAPIKey,
		general.OmdbLimiterSeconds,
		general.OmdbLimiterCalls,
		general.OmdbDisableTLSVerify,
		general.OmdbTimeoutSeconds,
	)
	apiexternal.NewTmdbClient(
		general.TheMovieDBApiKey,
		general.TmdbLimiterSeconds,
		general.TmdbLimiterCalls,
		general.TheMovieDBDisableTLSVerify,
		general.TmdbTimeoutSeconds,
	)
	apiexternal.NewTvdbClient(
		general.TvdbLimiterSeconds,
		general.TvdbLimiterCalls,
		general.TvdbDisableTLSVerify,
		general.TvdbTimeoutSeconds,
	)
	apiexternal.NewTVmazeClient(
		general.TvmazeLimiterSeconds,
		general.TvmazeLimiterCalls,
		general.TvmazeDisableTLSVerify,
		general.TvmazeTimeoutSeconds,
	)
	apiexternal.NewTraktClient(
		general.TraktClientID,
		general.TraktClientSecret,
		general.TraktLimiterSeconds,
		general.TraktLimiterCalls,
		general.TraktDisableTLSVerify,
		general.TraktTimeoutSeconds,
		general.TraktRedirectUrl,
	)
	apiexternal.NewPlexClient(
		general.PlexLimiterSeconds,
		general.PlexLimiterCalls,
		general.PlexDisableTLSVerify,
		general.PlexTimeoutSeconds,
	)
	apiexternal.NewJellyfinClient(
		general.JellyfinLimiterSeconds,
		general.JellyfinLimiterCalls,
		general.JellyfinDisableTLSVerify,
		general.JellyfinTimeoutSeconds,
	)

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

	err = database.InitImdbdb()
	if err != nil {
		logger.Logtype("fatal", 0).Err(err).Msg("IMDB Database Initialization Failed")
	}

	if database.DBQuickCheck() != "ok" {
		logger.Logtype("fatal", 0).Msg("integrity check failed")
		database.DBClose()
		os.Exit(100)
	}

	logger.Logtype("info", 0).Msg("Set Vars")
	// _ = html.UnescapeString("test")
	database.SetVars()

	logger.Logtype("info", 0).Msg("Generate All Quality Priorities")
	parser.GenerateAllQualityPriorities()

	logger.Logtype("info", 0).Msg("Load DB Patterns")
	parser.LoadDBPatterns()

	logger.Logtype("info", 0).Msg("Load DB Cutoff")
	parser.GenerateCutoffPriorities()

	if general.SearcherSize == 0 {
		general.SearcherSize = 5000
	}

	initproviders()
	// searcher.DefineSearchPool(general.SearcherSize)

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
	config.RangeSettingsIndexer(func(_ string, idx *config.IndexersConfig) {
		apiexternal.Getnewznabclient(idx)
	})

	// logger.Logtype("info", 0).Msg("Range Notification")
	// config.RangeSettingsNotification(func(_ string, idx *config.NotificationConfig) {
	// 	switch idx.NotificationType {
	// 	case "pushover":
	// 		apiexternal.GetPushoverclient(idx.Apikey)
	// 	case "gotify":
	// 		if idx.ServerURL != "" {
	// 			apiexternal.GetGotifyClient(idx.ServerURL, idx.Apikey)
	// 		}
	// 	case "pushbullet":
	// 		if idx.Apikey != "" {
	// 			apiexternal.GetPushbulletClient(idx.Apikey)
	// 		}
	// 	case "apprise":
	// 		if idx.ServerURL != "" {
	// 			apiexternal.GetAppriseClient(idx.ServerURL)
	// 		}
	// 	}
	// })

	worker.RegisterWorkerSyncMaps()
	logger.Logtype("info", 0).Msg("Create Cron Worker")
	worker.CreateCronWorker()

	logger.Logtype("info", 0).Msg("Inits")
	utils.Init()
	searcher.Init()

	logger.Logtype("info", 0).Msg("Refresh Cache")
	utils.Refreshcache(true)
	utils.Refreshcache(false)
	logger.Logtype("info", 0).Msg("Starting Scheduler")
	scheduler.InitScheduler()
	worker.StartCronWorker()

	logger.Logtype("info", 0).Msg("Starting API")

	router := gin.New()
	router.Use(logger.GinLogger(), logger.ErrorLogger())
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

	// Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	// router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !general.DisableSwagger {
	// 	docs.SwaggerInfo.BasePath = "/"
	// 	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// }

	if general.WebPortalEnabled {
		// goadmin.Init(router)
	}

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

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			database.DBClose()
			logger.Logtype("error", 0).Err(err).Msg("listen")
			// logger.LogDynamicError("error", err, "listen")
		}
	}()

	logger.Logtype("info", 1).Str("port", general.WebPort).Msg("Started API Webserver on port ")

	// Wait for interrupt signal to gracefully shutdown the server with
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Logtype("info", 0).Msg("Server shutting down")

	worker.StopCronWorker()
	worker.CloseWorkerPools()
	syncops.Shutdown()

	logger.Logtype("info", 0).Msg("Queues stopped")

	config.Slepping(true, 5)
	database.StopCache()
	database.DBClose()
	logger.Logtype("info", 0).Msg("Databases and cache stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Logtype("error", 0).Err(err).Msg("server shutdown")
		// logger.LogDynamicError("error", err, "server shutdown")
	}

	ctx.Done()

	logger.Logtype("info", 0).Msg("Server exiting")
}
