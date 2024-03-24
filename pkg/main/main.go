package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"os"

	"github.com/GoAdminGroup/go-admin/engine"
	goadmincfg "github.com/GoAdminGroup/go-admin/modules/config"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/modules/language"
	"github.com/GoAdminGroup/go-admin/template"
	"github.com/GoAdminGroup/go-admin/template/chartjs"
	"github.com/Kellerman81/go_media_downloader/api"
	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/goadmin/pages"
	"github.com/Kellerman81/go_media_downloader/goadmin/tables"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/Kellerman81/go_media_downloader/worker"

	"github.com/DeanThompson/ginpprof"

	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/sqlite" // sql driver
	"github.com/GoAdminGroup/themes/adminlte"
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
var version string
var buildstamp string
var githash string

func main() {
	//debug.SetGCPercent(30)
	os.Mkdir("./temp", 0777)
	if !scanner.CheckFileExist(config.Configfile) {
		config.ClearCfg()
		config.WriteCfg()
	}
	err := config.LoadCfgDB()
	if err != nil {
		os.Exit(1)
	}
	database.InitCache()
	worker.InitWorkerPools(config.SettingsGeneral.WorkerIndexer, config.SettingsGeneral.WorkerParse, config.SettingsGeneral.WorkerSearch, config.SettingsGeneral.WorkerFiles, config.SettingsGeneral.WorkerMetadata)
	logger.InitLogger(logger.Config{
		LogLevel:      config.SettingsGeneral.LogLevel,
		LogFileSize:   config.SettingsGeneral.LogFileSize,
		LogFileCount:  config.SettingsGeneral.LogFileCount,
		LogCompress:   config.SettingsGeneral.LogCompress,
		LogToFileOnly: config.SettingsGeneral.LogToFileOnly,
		LogColorize:   config.SettingsGeneral.LogColorize,
		TimeFormat:    config.SettingsGeneral.TimeFormat,
		TimeZone:      config.SettingsGeneral.TimeZone,
	})
	logger.LogDynamic("info", "Starting go_media_downloader")
	logger.LogDynamic("info", "Version: "+version+" "+githash)
	logger.LogDynamic("info", "Build Date: "+buildstamp)
	logger.LogDynamic("info", "Programmer: kellerman81")
	if config.SettingsGeneral.LogLevel != "Debug" {
		logger.LogDynamic("info", "Hint: Set Loglevel to Debug to see possible API Paths")
	}
	logger.LogDynamic("info", "------------------------------")

	apiexternal.NewOmdbClient(config.SettingsGeneral.OmdbAPIKey, config.SettingsGeneral.OmdbLimiterSeconds, config.SettingsGeneral.OmdbLimiterCalls, config.SettingsGeneral.OmdbDisableTLSVerify, config.SettingsGeneral.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.SettingsGeneral.TheMovieDBApiKey, config.SettingsGeneral.TmdbLimiterSeconds, config.SettingsGeneral.TmdbLimiterCalls, config.SettingsGeneral.TheMovieDBDisableTLSVerify, config.SettingsGeneral.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.SettingsGeneral.TvdbLimiterSeconds, config.SettingsGeneral.TvdbLimiterCalls, config.SettingsGeneral.TvdbDisableTLSVerify, config.SettingsGeneral.TvdbTimeoutSeconds)
	apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, config.SettingsGeneral.TraktLimiterSeconds, config.SettingsGeneral.TraktLimiterCalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)

	logger.LogDynamic("info", "Initialize Database")
	err = database.UpgradeDB()
	if err != nil {
		logger.LogDynamic("fatal", "Database Upgrade Failed", logger.NewLogFieldValue(err))
	}
	database.UpgradeIMDB()
	err = database.InitDB(config.SettingsGeneral.DBLogLevel)
	if err != nil {
		logger.LogDynamic("fatal", "Database Initialization Failed", logger.NewLogFieldValue(err))
	}
	err = database.InitImdbdb()
	if err != nil {
		logger.LogDynamic("fatal", "IMDB Database Initialization Failed", logger.NewLogFieldValue(err))
	}

	if database.DBQuickCheck() != "ok" {
		logger.LogDynamic("fatal", "integrity check failed")
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
	searcher.DefineSearchPool(config.SettingsGeneral.SearcherSize)

	for range config.SettingsGeneral.WorkerParse * config.SettingsGeneral.WorkerFiles {
		parse := apiexternal.ParserPool.Get()
		apiexternal.ParserPool.Put(parse)
	}

	logger.LogDynamic("info", "Check Fill IMDB")
	if database.GetdatarowN[int](true, "select count() from imdb_titles") == 0 {
		utils.FillImdb()
	}

	if database.GetdatarowN[int](false, "select count() from dbmovies") == 0 {
		logger.LogDynamic("info", "Initial Fill Movies")
		utils.InitialFillMovies()
	}
	if database.GetdatarowN[int](false, "select count() from dbseries") == 0 {
		logger.LogDynamic("info", "Initial Fill Series")
		utils.InitialFillSeries()
	}

	utils.Refreshcache(true)
	utils.Refreshcache(false)

	worker.CreateCronWorker()
	logger.LogDynamic("info", "Starting Scheduler")
	scheduler.InitScheduler()
	worker.StartCronWorker()

	logger.LogDynamic("info", "Starting API")
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
		template.AddComp(chartjs.NewChart())
		eng := engine.Default()
		acfg := goadmincfg.Config{
			Databases: goadmincfg.DatabaseList{
				"default": {Driver: "sqlite", File: "./databases/admin.db"},
			},
			AppID:                    "PPbLwfSG2Cwa",
			Theme:                    "adminlte",
			UrlPrefix:                "/admin",
			Env:                      goadmincfg.EnvLocal,
			Debug:                    true,
			Language:                 language.EN,
			Title:                    "Go Media Downloader",
			LoginTitle:               "Go Media Downloader",
			Logo:                     "<b>Go</b> Media Downloader",
			FooterInfo:               "Go Media Downloader by Kellerman81",
			IndexUrl:                 "/",
			GoModFilePath:            "",
			BootstrapFilePath:        "",
			LoginUrl:                 "/login",
			AssetRootPath:            "",
			AssetUrl:                 "",
			ColorScheme:              adminlte.ColorschemeSkinBlack,
			AccessLogPath:            "./logs/access.log",
			ErrorLogPath:             "./logs/error.log",
			InfoLogPath:              "./logs/info.log",
			HideConfigCenterEntrance: true,
			HideAppInfoEntrance:      true,
			HideToolEntrance:         true,
			HidePluginEntrance:       true,
			NoLimitLoginIP:           true,
		}
		if err := eng.AddConfig(&acfg).AddPlugins(eng.AdminPlugin()).AddGenerators(tables.Generators).
			Use(router); err != nil {
			panic(err)
		}
		eng.HTML("GET", "/admin", pages.GetDashBoard)
		eng.HTML("GET", "/", pages.GetDashBoard)
		eng.HTML("GET", "/actions", pages.GetActionsPage)
		eng.Services["sqlite"] = database.GetSqliteDB().InitDB(map[string]goadmincfg.Database{
			"default": {Driver: "sqlite", File: "./databases/admin.db"},
			"media":   {Driver: "sqlite"}})
		eng.Adapter.SetConnection(db.GetConnection(eng.Services))
		router.Static("/admin/uploads", "./temp")
	}

	if strings.EqualFold(config.SettingsGeneral.LogLevel, logger.StrDebug) {
		ginpprof.Wrap(router)
	}

	//webapp.Route("/web", &web.Home{})
	//router.Handle("GET", "/web", gin.WrapH(&webapp.Handler{}))

	logger.LogDynamic("info", "Starting API Webserver on port", logger.NewLogField("port", config.SettingsGeneral.WebPort))
	server := http.Server{
		Addr:    ":" + config.SettingsGeneral.WebPort,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			database.DBClose()
			logger.LogDynamic("error", "listen", logger.NewLogField("", err))
			//logger.LogDynamicError("error", err, "listen")
		}
	}()
	logger.LogDynamic("info", "Started API Webserver on port ", logger.NewLogField("port", config.SettingsGeneral.WebPort))

	// Wait for interrupt signal to gracefully shutdown the server with
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.LogDynamic("info", "Server shutting down")

	worker.StopCronWorker()
	worker.CloseWorkerPools()

	logger.LogDynamic("info", "Queues stopped")

	config.Slepping(true, 5)
	database.DBClose()
	logger.LogDynamic("info", "Databases stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.LogDynamic("error", "server shutdown", logger.NewLogField("", err))
		//logger.LogDynamicError("error", err, "server shutdown")
	}
	ctx.Done()

	logger.LogDynamic("info", "Server exiting")
}
