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
	"github.com/Kellerman81/go_media_downloader/worker"
	"github.com/rs/zerolog"

	"github.com/DeanThompson/ginpprof"

	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/sqlite" // sql driver
	"github.com/GoAdminGroup/themes/adminlte"
	"github.com/gin-gonic/gin"

	//webapp "github.com/maxence-charriere/go-app/v9/pkg/app"

	//ginlog "github.com/toorop/gin-logrus"
	ginlog "github.com/gin-contrib/logger"
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
	config.LoadCfgDB()

	if config.SettingsGeneral.WebPort == "" {
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB()
	}
	worker.InitWorkerPools(config.SettingsGeneral.WorkerIndexer, config.SettingsGeneral.WorkerParse, config.SettingsGeneral.WorkerSearch, config.SettingsGeneral.WorkerFiles, config.SettingsGeneral.WorkerMetadata)
	logger.InitLogger(logger.Config{
		LogLevel:     config.SettingsGeneral.LogLevel,
		LogFileSize:  config.SettingsGeneral.LogFileSize,
		LogFileCount: config.SettingsGeneral.LogFileCount,
		LogCompress:  config.SettingsGeneral.LogCompress,
		TimeFormat:   config.SettingsGeneral.TimeFormat,
		TimeZone:     config.SettingsGeneral.TimeZone,
	})
	logger.DisableVariableCleanup = config.SettingsGeneral.DisableVariableCleanup
	logger.DisableParserStringMatch = config.SettingsGeneral.DisableParserStringMatch
	logger.Log.Info().Msg("Starting go_media_downloader")
	logger.Log.Info().Msg("Version: " + version + " " + githash)
	logger.Log.Info().Msg("Build Date: " + buildstamp)
	logger.Log.Info().Msg("Programmer: kellerman81")
	if config.SettingsGeneral.LogLevel != "Debug" {
		logger.Log.Info().Msg("Hint: Set Loglevel to Debug to see possible API Paths")
	}
	logger.Log.Info().Msg("------------------------------")
	logger.Log.Info().Send()

	apiexternal.NewOmdbClient(config.SettingsGeneral.OmdbAPIKey, config.SettingsGeneral.Omdblimiterseconds, config.SettingsGeneral.Omdblimitercalls, config.SettingsGeneral.OmdbDisableTLSVerify, config.SettingsGeneral.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.SettingsGeneral.TheMovieDBApiKey, config.SettingsGeneral.Tmdblimiterseconds, config.SettingsGeneral.Tmdblimitercalls, config.SettingsGeneral.TheMovieDBDisableTLSVerify, config.SettingsGeneral.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.SettingsGeneral.Tvdblimiterseconds, config.SettingsGeneral.Tvdblimitercalls, config.SettingsGeneral.TvdbDisableTLSVerify, config.SettingsGeneral.TvdbTimeoutSeconds)
	apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, *config.GetTrakt("trakt_token"), config.SettingsGeneral.Traktlimiterseconds, config.SettingsGeneral.Traktlimitercalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)

	logger.Log.Info().Msg("Initialize Database")

	logger.Log.Info().Msg("Check Database for Upgrades")
	database.UpgradeDB()
	database.UpgradeIMDB()
	database.InitDB(config.SettingsGeneral.DBLogLevel)
	database.InitImdbdb()

	logger.Log.Info().Msg("Check Database for Errors")
	if database.DBQuickCheck() != "ok" {
		logger.Log.Error().Msg("integrity check failed")
		database.DBClose()
		os.Exit(100)
	}

	logger.Log.Info().Msg("Init Regex")
	database.GetVars()

	logger.Log.Info().Msg("Init Priorities")
	parser.GetAllQualityPriorities()

	parser.LoadDBPatterns()

	logger.Log.Info().Msg("Check Fill IMDB")
	counter := database.QueryImdbIntColumn("select count() from imdb_titles")
	if counter == 0 {
		utils.FillImdb()
	}

	logger.Log.Info().Msg("Check Fill DB")
	counter = database.QueryIntColumn("select count() from dbmovies")
	if counter == 0 {
		utils.InitialFillMovies()
	}
	counter = database.QueryIntColumn("select count() from dbseries")
	if counter == 0 {
		utils.InitialFillSeries()
	}

	logger.Log.Info().Msg("Starting Scheduler")
	scheduler.InitScheduler()

	logger.Log.Info().Msg("Starting API")
	router := gin.New()
	router.Use(ginlog.SetLogger(ginlog.WithLogger(func(_ *gin.Context, l zerolog.Logger) zerolog.Logger {
		return l.Output(gin.DefaultWriter).With().Logger()
	})))

	//corsconfig := cors.DefaultConfig()
	//corsconfig.AllowHeaders = []string{"*"}
	//corsconfig.AllowOrigins = []string{"*"}
	//corsconfig.AllowMethods = []string{"*"}

	if !strings.EqualFold(config.SettingsGeneral.LogLevel, logger.StrDebug) {
		gin.SetMode(gin.ReleaseMode)
	}
	//router.Use(ginlog.Logger(logger.Log), cors.New(corsconfig), gin.Recovery())

	logger.Log.Info().Msg("Starting API Endpoints")
	routerapi := router.Group("/api")
	api.AddGeneralRoutes(routerapi)

	logger.Log.Info().Msg("Starting API Endpoints-2")
	api.AddAllRoutes(routerapi.Group("/all"))

	logger.Log.Info().Msg("Starting API Endpoints-3")
	api.AddMoviesRoutes(routerapi.Group("/movies"))

	logger.Log.Info().Msg("Starting API Endpoints-4")
	api.AddSeriesRoutes(routerapi.Group("/series"))

	//Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	//router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !config.SettingsGeneral.DisableSwagger {
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

	if strings.EqualFold(config.SettingsGeneral.LogLevel, logger.StrDebug) {
		ginpprof.Wrap(router)
	}

	//webapp.Route("/web", &web.Home{})
	//router.Handle("GET", "/web", gin.WrapH(&webapp.Handler{}))

	logger.Log.Info().Str("port", config.SettingsGeneral.WebPort).Msg("Starting API Webserver on port")
	server := &http.Server{
		Addr:    ":" + config.SettingsGeneral.WebPort,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			database.DBClose()
			logger.Log.Error().Err(err).Msg("listen")
			//logger.Logerror(err, "listen")
		}
	}()
	logger.Log.Info().Str("port", config.SettingsGeneral.WebPort).Msg("Started API Webserver on port ")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info().Msg("Server shutting down")

	worker.CloseWorkerPools()

	for idx := range scheduler.Crons {
		scheduler.Crons[idx].Stop()
	}
	for idx := range scheduler.Ticker {
		scheduler.Ticker[idx].Stop()
	}
	logger.Log.Info().Msg("Queues stopped")

	config.Slepping(true, 5)
	database.DBClose()
	logger.Log.Info().Msg("Databases stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Error().Err(err).Msg("server shutdown")
		//logger.Logerror(err, "server shutdown")
	}
	ctx.Done()

	logger.Log.Info().Msg("Server exiting")
}
