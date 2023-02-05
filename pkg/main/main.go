package main

import (
	"context"
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
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/utils"
	"go.uber.org/zap"

	"github.com/DeanThompson/ginpprof"

	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/sqlite" // sql driver
	"github.com/GoAdminGroup/themes/adminlte"
	"github.com/gin-gonic/gin"

	//webapp "github.com/maxence-charriere/go-app/v9/pkg/app"

	//ginlog "github.com/toorop/gin-logrus"
	ginzap "github.com/gin-contrib/zap"
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
	config.LoadCfgDB(config.GetCfgFile())

	if config.Cfg.General.WebPort == "" {
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB(config.GetCfgFile())
	}
	logger.InitWorkerPools(config.Cfg.General.WorkerIndexer, config.Cfg.General.WorkerParse, config.Cfg.General.WorkerSearch, config.Cfg.General.WorkerFiles, config.Cfg.General.WorkerMetadata)
	logger.InitLogger(logger.Config{
		LogLevel:     config.Cfg.General.LogLevel,
		LogFileSize:  config.Cfg.General.LogFileSize,
		LogFileCount: config.Cfg.General.LogFileCount,
		LogCompress:  config.Cfg.General.LogCompress,
		TimeFormat:   config.Cfg.General.TimeFormat,
		TimeZone:     config.Cfg.General.TimeZone,
	})
	logger.DisableVariableCleanup = config.Cfg.General.DisableVariableCleanup
	logger.DisableParserStringMatch = config.Cfg.General.DisableParserStringMatch
	logger.Log.GlobalLogger.Info("Starting go_media_downloader")
	logger.Log.GlobalLogger.Info("Version: " + version + " " + githash)
	logger.Log.GlobalLogger.Info("Build Date: " + buildstamp)
	logger.Log.GlobalLogger.Info("Programmer: kellerman81")
	if config.Cfg.General.LogLevel != "Debug" {
		logger.Log.GlobalLogger.Info("Hint: Set Loglevel to Debug to see possible API Paths")
	}
	logger.Log.GlobalLogger.Info("------------------------------")
	logger.Log.GlobalLogger.Info("")

	apiexternal.NewOmdbClient(config.Cfg.General.OmdbAPIKey, config.Cfg.General.Omdblimiterseconds, config.Cfg.General.Omdblimitercalls, config.Cfg.General.OmdbDisableTLSVerify, config.Cfg.General.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.Cfg.General.TheMovieDBApiKey, config.Cfg.General.Tmdblimiterseconds, config.Cfg.General.Tmdblimitercalls, config.Cfg.General.TheMovieDBDisableTLSVerify, config.Cfg.General.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.Cfg.General.Tvdblimiterseconds, config.Cfg.General.Tvdblimitercalls, config.Cfg.General.TvdbDisableTLSVerify, config.Cfg.General.TvdbTimeoutSeconds)
	apiexternal.NewTraktClient(config.Cfg.General.TraktClientID, config.Cfg.General.TraktClientSecret, *config.GetTrakt("trakt_token"), config.Cfg.General.Traktlimiterseconds, config.Cfg.General.Traktlimitercalls, config.Cfg.General.TraktDisableTLSVerify, config.Cfg.General.TraktTimeoutSeconds)

	logger.Log.GlobalLogger.Info("Initialize Database")

	logger.Log.GlobalLogger.Info("Check Database for Upgrades")
	database.UpgradeDB()
	database.UpgradeIMDB()
	database.InitDb(config.Cfg.General.DBLogLevel)
	database.InitImdbdb(config.Cfg.General.DBLogLevel)

	logger.Log.GlobalLogger.Info("Check Database for Errors")
	if database.DbQuickCheck() != "ok" {
		logger.Log.GlobalLogger.Error("integrity check failed")
		database.DBClose()
		os.Exit(100)
	}

	logger.Log.GlobalLogger.Info("Init Regex")
	database.GetVars()

	logger.Log.GlobalLogger.Info("Init Priorities")
	parser.GetAllQualityPriorities()

	logger.Log.GlobalLogger.Info("Check Fill IMDB")
	counter, err := database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: "select count() from imdb_titles"})
	if counter == 0 || err != nil {
		utils.FillImdb()
	}

	logger.Log.GlobalLogger.Info("Check Fill DB")
	counter, _ = database.CountRows("dbmovies", &database.Querywithargs{})
	if counter == 0 {
		utils.InitialFillMovies()
	}
	counter, _ = database.CountRows("dbseries", &database.Querywithargs{})
	if counter == 0 {
		utils.InitialFillSeries()
	}

	logger.Log.GlobalLogger.Info("Starting Scheduler")
	go scheduler.InitScheduler()

	logger.Log.GlobalLogger.Info("Starting API")
	router := gin.New()

	//corsconfig := cors.DefaultConfig()
	//corsconfig.AllowHeaders = []string{"*"}
	//corsconfig.AllowOrigins = []string{"*"}
	//corsconfig.AllowMethods = []string{"*"}

	if !strings.EqualFold(config.Cfg.General.LogLevel, "debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	logger.Log.GlobalLogger.Info("Starting API Logger")
	router.Use(ginzap.Ginzap(logger.Log.GlobalLogger, time.RFC3339, true))

	// Logs all panic to error log
	//   - stack means whether output the stack info.
	router.Use(ginzap.RecoveryWithZap(logger.Log.GlobalLogger, true))
	//router.Use(ginlog.Logger(logger.Log), cors.New(corsconfig), gin.Recovery())

	logger.Log.GlobalLogger.Info("Starting API Endpoints")
	routerapi := router.Group("/api")
	api.AddGeneralRoutes(routerapi)

	logger.Log.GlobalLogger.Info("Starting API Endpoints-2")
	api.AddAllRoutes(routerapi.Group("/all"))

	logger.Log.GlobalLogger.Info("Starting API Endpoints-3")
	api.AddMoviesRoutes(routerapi.Group("/movies"))

	logger.Log.GlobalLogger.Info("Starting API Endpoints-4")
	api.AddSeriesRoutes(routerapi.Group("/series"))

	//Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	//router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !config.Cfg.General.DisableSwagger {
	// 	docs.SwaggerInfo.BasePath = "/"
	// 	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// }

	if true {
		template.AddComp(chartjs.NewChart())
		eng := engine.Default()
		acfg := &goadmincfg.Config{
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
		if err := eng.AddConfig(acfg).AddPlugins(eng.AdminPlugin()).AddGenerators(tables.Generators).
			Use(router); err != nil {
			panic(err)
		}
		eng.HTML("GET", "/admin", pages.GetDashBoardO)
		eng.HTML("GET", "/", pages.GetDashBoardO)
		eng.HTML("GET", "/actions", pages.GetActionsPage)
		eng.Services["sqlite"] = database.GetSqliteDB().InitDB(map[string]goadmincfg.Database{
			"default": {Driver: "sqlite", File: "./databases/admin.db"},
			"media":   {Driver: "sqlite"}})
		eng.Adapter.SetConnection(db.GetConnection(eng.Services))
		router.Static("/admin/uploads", "./temp")
	}

	if strings.EqualFold(config.Cfg.General.LogLevel, "debug") {
		ginpprof.Wrap(router)
	}

	//webapp.Route("/web", &web.Home{})
	//router.Handle("GET", "/web", gin.WrapH(&webapp.Handler{}))

	logger.Log.GlobalLogger.Info("Starting API Webserver on port ", zap.String("port", config.Cfg.General.WebPort))
	server := &http.Server{
		Addr:    ":" + config.Cfg.General.WebPort,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			database.DBClose()
			logger.Log.GlobalLogger.Fatal("listen ", zap.Error(err))
		}
	}()
	logger.Log.GlobalLogger.Info("Started API Webserver on port ", zap.String("port", config.Cfg.General.WebPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.GlobalLogger.Info("Server shutting down")

	logger.CloseWorkerPools()

	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()
	logger.Log.GlobalLogger.Info("Queues stopped")

	config.Slepping(true, 5)

	database.DBClose()
	logger.Log.GlobalLogger.Info("Databases stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Log.GlobalLogger.Fatal("Server Shutdown:", zap.Error(err))
	}
	ctx.Done()

	logger.Log.GlobalLogger.Info("Server exiting")
}
