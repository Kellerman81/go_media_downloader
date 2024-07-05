package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/api"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/goadmin"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"

	"github.com/DeanThompson/ginpprof"
	"github.com/gin-gonic/gin"
	//webapp "github.com/maxence-charriere/go-app/v9/pkg/app"
	//ginlog "github.com/toorop/gin-logrus"
)

// @title                       Go Media Downloader API
// @version                     1.0
// @Schemes                     http https
// @host                        localhost:9090
// @securitydefinitions.apikey  ApiKeyAuth
// @in                          query
// @name                        apikey
// @Accept                      json
// @Produce                     json
var (
	version    string
	buildstamp string
	githash    string
)

func main() {
	//debug.SetGCPercent(30)
	os.Mkdir("./temp", 0777)
	cfgfile := config.Configfile
	if !scanner.CheckFileExist(cfgfile) {
		config.ClearCfg()
		config.WriteCfg()
	}
	err := config.LoadCfgDB()
	if err != nil {
		os.Exit(1)
	}
	database.InitCache()
	worker.InitWorkerPools(config.SettingsGeneral.WorkerSearch, config.SettingsGeneral.WorkerFiles, config.SettingsGeneral.WorkerMetadata)
	logger.InitLogger(logger.Config{
		LogLevel:      config.SettingsGeneral.LogLevel,
		LogFileSize:   config.SettingsGeneral.LogFileSize,
		LogFileCount:  config.SettingsGeneral.LogFileCount,
		LogCompress:   config.SettingsGeneral.LogCompress,
		LogToFileOnly: config.SettingsGeneral.LogToFileOnly,
		LogColorize:   config.SettingsGeneral.LogColorize,
		TimeFormat:    config.SettingsGeneral.TimeFormat,
		TimeZone:      config.SettingsGeneral.TimeZone,
		LogZeroValues: config.SettingsGeneral.LogZeroValues,
	})
	logger.LogDynamicany("info", "Starting go_media_downloader")
	logger.LogDynamicany("info", "Version: "+version+" "+githash)
	logger.LogDynamicany("info", "Build Date: "+buildstamp)
	logger.LogDynamicany("info", "Programmer: kellerman81")
	if config.SettingsGeneral.LogLevel != "Debug" {
		logger.LogDynamicany("info", "Hint: Set Loglevel to Debug to see possible API Paths")
	}
	logger.LogDynamicany("info", "------------------------------")

	apiexternal.NewOmdbClient(config.SettingsGeneral.OmdbAPIKey, config.SettingsGeneral.OmdbLimiterSeconds, config.SettingsGeneral.OmdbLimiterCalls, config.SettingsGeneral.OmdbDisableTLSVerify, config.SettingsGeneral.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.SettingsGeneral.TheMovieDBApiKey, config.SettingsGeneral.TmdbLimiterSeconds, config.SettingsGeneral.TmdbLimiterCalls, config.SettingsGeneral.TheMovieDBDisableTLSVerify, config.SettingsGeneral.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.SettingsGeneral.TvdbLimiterSeconds, config.SettingsGeneral.TvdbLimiterCalls, config.SettingsGeneral.TvdbDisableTLSVerify, config.SettingsGeneral.TvdbTimeoutSeconds)
	apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, config.SettingsGeneral.TraktLimiterSeconds, config.SettingsGeneral.TraktLimiterCalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)

	logger.LogDynamicany("info", "Initialize Database")
	err = database.UpgradeDB()
	if err != nil {
		logger.LogDynamicany("fatal", "Database Upgrade Failed", err)
	}
	database.UpgradeIMDB()
	err = database.InitDB(config.SettingsGeneral.DBLogLevel)
	if err != nil {
		logger.LogDynamicany("fatal", "Database Initialization Failed", err)
	}
	err = database.InitImdbdb()
	if err != nil {
		logger.LogDynamicany("fatal", "IMDB Database Initialization Failed", err)
	}

	if database.DBQuickCheck() != "ok" {
		logger.LogDynamicany("fatal", "integrity check failed")
		database.DBClose()
		os.Exit(100)
	}

	database.SetVars()

	parser.GenerateAllQualityPriorities()

	parser.LoadDBPatterns()
	parser.GenerateCutoffPriorities()
	if config.SettingsGeneral.SearcherSize == 0 {
		config.SettingsGeneral.SearcherSize = 5000
	}
	//searcher.DefineSearchPool(config.SettingsGeneral.SearcherSize)

	logger.LogDynamicany("info", "Check Fill IMDB")
	if database.GetdatarowN[uint](true, "select count() from imdb_titles") == 0 {
		utils.FillImdb()
	}

	if database.GetdatarowN[uint](false, "select count() from dbmovies") == 0 {
		logger.LogDynamicany("info", "Initial Fill Movies")
		utils.InitialFillMovies()
	}
	if database.GetdatarowN[uint](false, "select count() from dbseries") == 0 {
		logger.LogDynamicany("info", "Initial Fill Series")
		utils.InitialFillSeries()
	}

	utils.Refreshcache(true)
	utils.Refreshcache(false)

	worker.CreateCronWorker()
	logger.LogDynamicany("info", "Starting Scheduler")
	scheduler.InitScheduler()
	worker.StartCronWorker()

	logger.LogDynamicany("info", "Starting API")
	router := gin.New()
	// router.Use(ginlog.SetLogger(ginlog.WithLogger(func(_ *gin.Context, l zerolog.Logger) zerolog.Logger {
	// 	return l.Output(gin.DefaultWriter).With().Logger()
	// })))

	//corsconfig := cors.DefaultConfig()
	//corsconfig.AllowHeaders = []string{"*"}
	//corsconfig.AllowOrigins = []string{"*"}
	//corsconfig.AllowMethods = []string{"*"}

	if !strings.EqualFold(config.SettingsGeneral.LogLevel, logger.StrDebug) {
		gin.SetMode(gin.ReleaseMode)
	}
	//router.Use(ginlog.Logger(logger.Log), cors.New(corsconfig), gin.Recovery())

	routerapi := router.Group("/api")
	api.AddGeneralRoutes(routerapi)
	api.AddAllRoutes(routerapi.Group("/all"))
	api.AddMoviesRoutes(routerapi.Group("/movies"))
	api.AddSeriesRoutes(routerapi.Group("/series"))

	//Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	//router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !config.SettingsGeneral.DisableSwagger {
	// 	docs.SwaggerInfo.BasePath = "/"
	// 	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// }

	if config.SettingsGeneral.WebPortalEnabled {
		goadmin.Init(router)
	}

	if strings.EqualFold(config.SettingsGeneral.LogLevel, logger.StrDebug) {
		ginpprof.Wrap(router)
	}

	//webapp.Route("/web", &web.Home{})
	//router.Handle("GET", "/web", gin.WrapH(&webapp.Handler{}))

	logger.LogDynamicany("info", "Starting API Webserver on port", "port", &config.SettingsGeneral.WebPort)
	server := http.Server{
		Addr:              ":" + config.SettingsGeneral.WebPort,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			database.DBClose()
			logger.LogDynamicany("error", "listen", err)
			//logger.LogDynamicError("error", err, "listen")
		}
	}()
	logger.LogDynamicany("info", "Started API Webserver on port ", "port", &config.SettingsGeneral.WebPort)

	// Wait for interrupt signal to gracefully shutdown the server with
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.LogDynamicany("info", "Server shutting down")

	worker.StopCronWorker()
	worker.CloseWorkerPools()

	logger.LogDynamicany("info", "Queues stopped")

	config.Slepping(true, 5)
	database.DBClose()
	logger.LogDynamicany("info", "Databases stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.LogDynamicany("error", "server shutdown", err)
		//logger.LogDynamicError("error", err, "server shutdown")
	}
	ctx.Done()

	logger.LogDynamicany("info", "Server exiting")
}
