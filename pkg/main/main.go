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
	"golang.org/x/oauth2"

	"github.com/DeanThompson/ginpprof"

	_ "github.com/GoAdminGroup/go-admin/adapter/gin"
	_ "github.com/GoAdminGroup/go-admin/modules/db/drivers/sqlite" // sql driver
	"github.com/GoAdminGroup/themes/adminlte"
	"github.com/gin-gonic/gin"
	ginlog "github.com/toorop/gin-logrus"
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
	config.LoadCfgDB(config.Configfile)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WebPort == "" {
		//fmt.Println("Checked for general - config is missing", cfg_general)
		//os.Exit(0)
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB(config.Configfile)
	}

	logger.InitLogger(logger.LoggerConfig{
		LogLevel:     cfg_general.LogLevel,
		LogFileSize:  cfg_general.LogFileSize,
		LogFileCount: cfg_general.LogFileCount,
		LogCompress:  cfg_general.LogCompress,
	})
	logger.Log.Infoln("Starting go_media_downloader")
	logger.Log.Infoln("Version: " + version + " " + githash)
	logger.Log.Infoln("Build Date: " + buildstamp)
	logger.Log.Infoln("Programmer: kellerman81")
	logger.Log.Infoln("Hint: Set Loglevel to Debug to see possible API Paths")
	logger.Log.Infoln("------------------------------")
	logger.Log.Infoln("")

	apiexternal.NewOmdbClient(cfg_general.OmdbApiKey, cfg_general.Omdblimiterseconds, cfg_general.Omdblimitercalls, cfg_general.OmdbDisableTLSVerify)
	apiexternal.NewTmdbClient(cfg_general.TheMovieDBApiKey, cfg_general.Tmdblimiterseconds, cfg_general.Tmdblimitercalls, cfg_general.TheMovieDBDisableTLSVerify)
	apiexternal.NewTvdbClient(cfg_general.Tvdblimiterseconds, cfg_general.Tvdblimitercalls, cfg_general.TvdbDisableTLSVerify)
	if config.ConfigCheck("trakt_token") {
		apiexternal.NewTraktClient(cfg_general.TraktClientId, cfg_general.TraktClientSecret, config.ConfigGet("trakt_token").Data.(oauth2.Token), cfg_general.Traktlimiterseconds, cfg_general.Traktlimitercalls, cfg_general.TraktDisableTLSVerify)
	} else {
		apiexternal.NewTraktClient(cfg_general.TraktClientId, cfg_general.TraktClientSecret, oauth2.Token{}, cfg_general.Traktlimiterseconds, cfg_general.Traktlimitercalls, cfg_general.TraktDisableTLSVerify)
	}

	logger.Log.Infoln("Initialize Database")
	database.InitDb(cfg_general.DBLogLevel)

	database.DBImdb = database.InitImdbdb(cfg_general.DBLogLevel, "imdb")

	logger.Log.Infoln("Check Database for Upgrades")
	database.UpgradeDB()
	database.GetVars()
	parser.LoadDBPatterns()

	logger.Log.Infoln("Check Database for Errors")
	if database.DbQuickCheck() != "ok" {
		logger.Log.Errorln("integrity check failed")
		database.DB.Close()
		os.Exit(100)
	}

	utils.InitRegex()

	counter, _ := database.CountRows("dbmovies", database.Query{})
	if counter == 0 {
		utils.InitialFillMovies()
	}
	counter, _ = database.CountRows("dbseries", database.Query{})
	if counter == 0 {
		utils.InitialFillSeries()
	}

	logger.Log.Infoln("Starting Scheduler")
	scheduler.InitScheduler()

	logger.Log.Infoln("Starting API")
	router := gin.New()
	if !strings.EqualFold(cfg_general.LogLevel, "debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	logger.Log.Infoln("Starting API Logger")
	router.Use(ginlog.Logger(logger.Log), gin.Recovery())

	logger.Log.Infoln("Starting API Endpoints")
	routerapi := router.Group("/api")
	api.AddGeneralRoutes(routerapi)

	logger.Log.Infoln("Starting API Endpoints-2")
	api.AddAllRoutes(routerapi.Group("/all"))

	logger.Log.Infoln("Starting API Endpoints-3")
	api.AddMoviesRoutes(routerapi.Group("/movies"))

	logger.Log.Infoln("Starting API Endpoints-4")
	api.AddSeriesRoutes(routerapi.Group("/series"))

	//Less RAM Usage for static file - don't forget to recreate html file
	router.Static("/swagger", "./docs")
	//router.StaticFile("/swagger/index.html", "./docs/api.html")
	// if !cfg_general.DisableSwagger {
	// 	docs.SwaggerInfo.BasePath = "/"
	// 	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	// }

	var goadmin bool = true

	if goadmin {
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
		// if err := eng.AddConfigFromYAML("./admin.yml").
		// 	AddGenerators(tables.Generators).
		// 	Use(router); err != nil {
		// 	panic(err)
		// }
		eng.HTML("GET", "/admin", pages.GetDashBoardO)
		eng.HTML("GET", "/", pages.GetDashBoardO)
		eng.HTML("GET", "/actions", pages.GetActionsPage)
		eng.Services["sqlite"] = database.GetSqliteDB().InitDB(map[string]goadmincfg.Database{
			"default": {Driver: "sqlite", File: "./databases/admin.db"},
			"media":   {Driver: "sqlite"}})
		defaultConnection := db.GetConnection(eng.Services)
		eng.Adapter.SetConnection(defaultConnection)
		router.Static("/admin/uploads", "./temp")
		//router.GET("/admin", pages.GetDashBoard2)

		//eng.HTML("GET", "/admin", pages.GetDashBoard2)
		//eng.HTML("GET", "/admin/", pages.GetDashBoard2)
		//eng.HTML("GET", "/", pages.GetDashBoard2)
		//eng.HTMLFile("GET", "/admin/hello", "./html/hello.tmpl", map[string]interface{}{
		//	"msg": "Hello world",
		//})
	}

	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
		ginpprof.Wrap(router)
	}

	logger.Log.Infoln("Starting API Webserver on port", cfg_general.WebPort)
	server := &http.Server{
		Addr:    ":" + cfg_general.WebPort,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			database.DB.Close()
			logger.Log.Fatalf("listen: %s\n", err)
		}
	}()
	logger.Log.Infoln("Started API Webserver on port", cfg_general.WebPort)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Infoln("Server shutting down")

	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()
	logger.Log.Infoln("Queues stopped")

	config.Slepping(true, 5)

	database.DBImdb.Close()
	database.DB.Close()
	logger.Log.Infoln("Databases stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Fatal("Server Shutdown:", err)
	}

	logger.Log.Println("Server exiting")
}
