package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"os"

	"github.com/Kellerman81/go_media_downloader/api"
	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/newznab"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/recoilme/pudge"
	"golang.org/x/oauth2"

	"github.com/DeanThompson/ginpprof"

	docs "github.com/Kellerman81/go_media_downloader/docs"
	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/ginview"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	ginlog "github.com/toorop/gin-logrus"
)

// @title go_media_downloader API

func main() {
	debug.SetGCPercent(20)

	pudb, _ := config.OpenConfig("config.db")
	config.ConfigDB = pudb
	//config.CacheConfig()
	//scanner.CleanUpFolder("./backup", 10)
	pudge.BackupAll("")
	os.Mkdir("./temp", 0777)
	f, errcfg := config.LoadCfgDB(config.Configfile)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WebPort == "" {
		fmt.Println("Checked for general - config is missing", cfg_general)
		os.Exit(0)
	}
	if errcfg == nil && cfg_general.EnableFileWatcher {
		f.Watch(func(event interface{}, err error) {
			if err != nil {
				log.Printf("watch error: %v", err)
				return
			}

			log.Println("cfg reloaded")
			time.Sleep(time.Duration(2) * time.Second)
			config.LoadCfgDataDB(f, config.Configfile)
		})
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

	apiexternal.NewOmdbClient(cfg_general.OmdbApiKey, cfg_general.Omdblimiterseconds, cfg_general.Omdblimitercalls)
	apiexternal.NewTmdbClient(cfg_general.TheMovieDBApiKey, cfg_general.Tmdblimiterseconds, cfg_general.Tmdblimitercalls)
	apiexternal.NewTvdbClient(cfg_general.Tvdblimiterseconds, cfg_general.Tvdblimitercalls)
	if config.ConfigCheck("trakt_token") {
		cfg_trakt := config.ConfigGet("trakt_token").Data.(oauth2.Token)
		apiexternal.NewTraktClient(cfg_general.TraktClientId, cfg_general.TraktClientSecret, cfg_trakt, cfg_general.Traktlimiterseconds, cfg_general.Traktlimitercalls)
	} else {
		apiexternal.NewTraktClient(cfg_general.TraktClientId, cfg_general.TraktClientSecret, oauth2.Token{}, cfg_general.Traktlimiterseconds, cfg_general.Traktlimitercalls)
	}

	apiexternal.NewznabClients = make(map[string]*newznab.Client, 10)

	structure.StructureJobRunning = make(map[string]bool, 10)
	importfeed.MovieImportJobRunning = make(map[string]bool, 10)
	importfeed.SeriesImportJobRunning = make(map[string]bool, 10)

	logger.Log.Infoln("Initialize Database")
	database.InitDb(cfg_general.DBLogLevel)

	dbimdb := database.InitImdbdb(cfg_general.DBLogLevel, "imdb")
	database.DBImdb = dbimdb

	logger.Log.Infoln("Check Database for Upgrades")
	database.UpgradeDB()
	database.GetVars()
	parser.LoadDBPatterns()

	logger.Log.Infoln("Check Database for Errors")
	str := database.DbQuickCheck()
	if str != "ok" {
		logger.Log.Errorln("integrity check failed", str)
		config.ConfigDB.Close()
		database.DB.Close()
		os.Exit(100)
	}

	logger.Log.Infoln("Remove Old DB Backups")
	database.RemoveOldDbBackups(cfg_general.MaxDatabaseBackups)

	counter, _ := database.CountRows("dbmovies", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for movies")
		file := "./init_imdb"
		if runtime.GOOS == "windows" {
			file = "init_imdb.exe"
		}
		exec.Command(file).Run()
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			database.DBImdb.Close()
			os.Remove("./imdb.db")
			os.Rename("./imdbtemp.db", "./imdb.db")
			dbnew := database.InitImdbdb("info", "imdb")
			dbnew.SetMaxOpenConns(5)
			database.DBImdb = dbnew
		}

		for _, idxmovie := range config.ConfigGetPrefix("movie_") {
			if !config.ConfigCheck(idxmovie.Name) {
				continue
			}
			cfg_movie := config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

			job := strings.ToLower("feeds")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})
			for idxlist := range cfg_movie.Lists {
				utils.Importnewmoviessingle(idxmovie.Name, cfg_movie.Lists[idxlist].Name)
			}
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}

		for _, idxmovie := range config.ConfigGetPrefix("movie_") {
			if !config.ConfigCheck(idxmovie.Name) {
				continue
			}
			cfg_movie := config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

			job := strings.ToLower("datafull")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})

			utils.Getnewmovies(idxmovie.Name)
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
	}
	counter, _ = database.CountRows("dbseries", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for series")

		for _, idxserie := range config.ConfigGetPrefix("serie_") {
			if !config.ConfigCheck(idxserie.Name) {
				continue
			}
			cfg_serie := config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

			job := strings.ToLower("feeds")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
			for idxlist := range cfg_serie.Lists {
				utils.Importnewseriessingle(idxserie.Name, cfg_serie.Lists[idxlist].Name)
			}
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
		for _, idxserie := range config.ConfigGetPrefix("serie_") {
			if !config.ConfigCheck(idxserie.Name) {
				continue
			}
			cfg_serie := config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

			job := strings.ToLower("datafull")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
			utils.Getnewepisodes(idxserie.Name)
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
	}

	logger.Log.Infoln("Starting Scheduler")
	scheduler.InitScheduler()

	config.RegexSeriesIdentifier, _ = regexp.Compile(`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`)
	config.RegexSeriesTitle, _ = regexp.Compile(`^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`)

	logger.Log.Infoln("Starting API")
	router := gin.New()
	docs.SwaggerInfo.BasePath = "/"
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
			Funcs: template.FuncMap{"copy": func() string {
				return time.Now().Format("2006")
			}},
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
	routerall := routerapi.Group("/all")
	api.AddAllRoutes(routerall)

	logger.Log.Infoln("Starting API Endpoints-3")
	routermovies := routerapi.Group("/movies")
	api.AddMoviesRoutes(routermovies)

	logger.Log.Infoln("Starting API Endpoints-4")
	routerseries := routerapi.Group("/series")
	api.AddSeriesRoutes(routerseries)
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

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
			config.ConfigDB.Close()
			database.DB.Close()
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("receive interrupt signal")

	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()

	config.Slepping(true, 5)

	database.DBImdb.Close()
	database.DB.Close()
	config.ConfigDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}

	// Close db
	if err := pudge.CloseAll(); err != nil {
		log.Fatal("Database Shutdown:", err)
	}

	log.Println("Server exiting")
}
