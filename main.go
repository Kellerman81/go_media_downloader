package main

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"os"

	"github.com/Kellerman81/go_media_downloader/api"
	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/utils"
	"golang.org/x/oauth2"

	"github.com/DeanThompson/ginpprof"

	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/ginview"
	"github.com/gin-gonic/gin"
	ginlog "github.com/toorop/gin-logrus"
)

// @title go_media_downloader API

func main() {
	//debug.SetGCPercent(30)

	os.Mkdir("./temp", 0777)
	config.LoadCfgDB(config.Configfile)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WebPort == "" {
		fmt.Println("Checked for general - config is missing", cfg_general)
		os.Exit(0)
	}

	logger.InitLogger(logger.LoggerConfig{
		LogLevel:     cfg_general.LogLevel,
		LogFileSize:  cfg_general.LogFileSize,
		LogFileCount: cfg_general.LogFileCount,
		LogCompress:  cfg_general.LogCompress,
	})
	logger.Log.Infoln("Starting go_media_downloader")
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

	if _, err := os.Stat("./views"); !os.IsNotExist(err) {
		logger.Log.Infoln("Starting API Websites")
		router.HTMLRender = ginview.New(goview.Config{
			Root:      "views",
			Extension: ".html",
			Master:    "layouts/master",
			//Partials:  []string{"partials/ad"},
			//Funcs: template.FuncMap{"copy": func() string {
			//	return time.Now().Format("2006")
			//}},
			DisableCache: false,
			Delims:       goview.Delims{},
		})
		//router.HTMLRender = ginview.Default()
		router.Static("/dist", "./views/dist")
		router.Static("/pages", "./views/pages")
		router.Static("/plugins", "./views/plugins")
		router.Static("/build", "./views/build")
		router.GET("/", func(ctx *gin.Context) {
			//render with master-
			ctx.HTML(http.StatusOK, "index", gin.H{
				"title": "Index title!",
				"add": func(a int, b int) int {
					return a + b
				},
			})
		})
		router.GET("/dbmovies", func(ctx *gin.Context) {
			//render with master-
			ctx.HTML(http.StatusOK, "dbmovies", gin.H{
				"title": "DB Movies",
			})
		})

		router.GET("/page", func(ctx *gin.Context) {
			//render only file, must full name with extension
			ctx.HTML(http.StatusOK, "page.html", gin.H{"title": "Page file title!!"})
		})
	}
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
	logger.Log.Println("receive interrupt signal")

	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()

	config.Slepping(true, 5)

	database.DBImdb.Close()
	database.DB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Fatal("Server Shutdown:", err)
	}

	logger.Log.Println("Server exiting")
}
