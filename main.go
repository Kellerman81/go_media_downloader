package main

import (
	"context"
	"html/template"
	"log"
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
	"github.com/Kellerman81/go_media_downloader/newznab"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/recoilme/pudge"

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

	pudb, _ := config.OpenConfig("config.db")
	config.ConfigDB = pudb
	scanner.CleanUpFolder("./backup", 10)
	pudge.BackupAll("")
	os.Mkdir("./temp", 0777)
	f, errcfg := config.LoadCfgDB(config.Configfile)
	if errcfg == nil {
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

	defer func() {
		config.ConfigDB.Close()
	}()

	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

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
	// keys, _ := config.ConfigDB.Keys([]byte(""), 0, 0, true)

	// fmt.Println(cfg_general)
	// for _, v := range keys {
	// 	fmt.Print(string(v) + ", ")
	// }
	// database.InitDb(cfg.General.DBLogLevel)
	// database.GetMovies(database.Query{Where: })
	// utils.GetHighestMoviePriorityByFiles(cfg, )
	// counter2, _ := database.CountRows("movie_files", database.Query{Where: "movie_id =?", WhereArgs: []interface{}{999}})
	// fmt.Println(counter2)
	// return
	// dbimdb2 := database.InitImdbdb(cfg.General.DBLogLevel, "imdb")
	// database.DBImdb = dbimdb2
	// table, tableerr := database.QueryVersions(database.Query{})
	// // var inInterface map[string]interface{}
	// // inrec, _ := json.Marshal(table)
	// // json.Unmarshal(inrec, &inInterface)
	// // fmt.Println(inInterface)
	// fmt.Println(len(table))
	// fmt.Println(tableerr)
	// return

	apiexternal.NewOmdbClient(cfg_general.OmdbApiKey, cfg_general.Omdblimiterseconds, cfg_general.Omdblimitercalls)
	apiexternal.NewTmdbClient(cfg_general.TheMovieDBApiKey, cfg_general.Tmdblimiterseconds, cfg_general.Tmdblimitercalls)
	apiexternal.NewTvdbClient(cfg_general.Tvdblimiterseconds, cfg_general.Tvdblimitercalls)
	apiexternal.NewTraktClient(cfg_general.TraktClientId, cfg_general.Traktlimiterseconds, cfg_general.Traktlimitercalls)
	apiexternal.NewznabClients = make(map[string]newznab.Client, 10)
	//watch_file, parser := config.LoadConfigV2(configfile)
	//config.WatchConfig(watch_file, parser)q

	utils.SeriesStructureJobRunning = make(map[string]bool, 10)
	utils.MovieImportJobRunning = make(map[string]bool, 10)
	utils.SeriesImportJobRunning = make(map[string]bool, 10)
	utils.SerieJobRunning = make(map[string]bool, 10)
	utils.MovieJobRunning = make(map[string]bool, 10)

	database.InitDb(cfg_general.DBLogLevel)

	dbimdb := database.InitImdbdb(cfg_general.DBLogLevel, "imdb")
	database.DBImdb = dbimdb

	database.UpgradeDB()
	database.GetVars()

	//parse, _ := utils.NewFileParser(cfg, "Bull.2016.S03E12.HDTV.x264-KILLERS (tvdb311945) (tvdb1234)", true, "series")
	// parse, _ := utils.NewFileParser("Rampage.Capital.Punishment.2014.BRRIP.H264.AAC-MAJESTiC (tt3448226)", false, "movie")
	// fmt.Println("aud: ", parse.Audio)
	// fmt.Println("cod: ", parse.Codec)
	// fmt.Println("im: ", parse.Imdb)
	// fmt.Println("ql: ", parse.Quality)
	// fmt.Println("res: ", parse.Resolution)
	// fmt.Println("tit: ", parse.Title)
	// fmt.Println("year: ", parse.Year)
	// fmt.Println("season: ", parse.Season)
	// fmt.Println("episode: ", parse.Episode)
	// fmt.Println("iden: ", parse.Identifier)
	// return

	counter, _ := database.CountRows("dbmovies", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for movies")
		utils.InitFillImdb()

		movie_keys, _ := config.ConfigDB.Keys([]byte("movie_*"), 0, 0, true)

		for _, idxmovie := range movie_keys {
			var cfg_movie config.MediaTypeConfig
			config.ConfigGet(string(idxmovie), &cfg_movie)

			job := strings.ToLower("feeds")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})
			for idxlist := range cfg_movie.Lists {
				utils.Importnewmoviessingle(cfg_movie, cfg_movie.Lists[idxlist])
			}
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}

		for _, idxmovie := range movie_keys {
			var cfg_movie config.MediaTypeConfig
			config.ConfigGet(string(idxmovie), &cfg_movie)

			job := strings.ToLower("datafull")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})

			utils.Getnewmovies(cfg_movie)
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
	}
	counter, _ = database.CountRows("dbseries", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for series")
		serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

		for _, idxserie := range serie_keys {
			var cfg_serie config.MediaTypeConfig
			config.ConfigGet(string(idxserie), &cfg_serie)

			job := strings.ToLower("feeds")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
			for idxlist := range cfg_serie.Lists {
				utils.Importnewseriessingle(cfg_serie, cfg_serie.Lists[idxlist])
			}
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
		for _, idxserie := range serie_keys {
			var cfg_serie config.MediaTypeConfig
			config.ConfigGet(string(idxserie), &cfg_serie)

			job := strings.ToLower("datafull")
			dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
				[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
			utils.Getnewepisodes(cfg_serie)
			dbid, _ := dbresult.LastInsertId()
			database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

		}
	}

	database.DBImdb.SetMaxOpenConns(5)
	scheduler.InitScheduler()

	router := gin.New()
	docs.SwaggerInfo.BasePath = "/"
	if !strings.EqualFold(cfg_general.LogLevel, "debug") {
		gin.SetMode(gin.ReleaseMode)
	}
	router.Use(ginlog.Logger(logger.Log), gin.Recovery())

	if _, err := os.Stat("./views"); !os.IsNotExist(err) {
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
	routerapi := router.Group("/api")
	{
		routerapi.GET("/fillimdb", api.ApiFillImdb)
		routerapi.GET("/scheduler/stop", api.ApiSchedulerStop)
		routerapi.GET("/scheduler/start", api.ApiSchedulerStart)
		routerapi.GET("/db/close", api.ApiDbClose)
		routerapi.DELETE("/db/clear/:name", api.ApiDbClear)
		routerapi.DELETE("/db/oldjobs", api.ApiDbRemoveOldJobs)
		routerapi.GET("/db/vacuum", api.ApiDbVacuum)
		routerapi.POST("/parse/string", api.ApiParseString)
		routerapi.POST("/parse/file", api.ApiParseFile)
		routerapi.POST("/naming", api.ApiNamingGenerate)
		routerapi.POST("/structure", api.ApiStructure)
		routerapi.GET("/quality", api.ApiGetQualities)
		routerapi.DELETE("/quality/:id", api.ApiQualityDelete)
		routerapi.POST("/quality", api.ApiQualityUpdate)
		routerapi.GET("/quality/:name/:config", api.ApiListQualityPriorities)

		routerapi.GET("/config/all", api.ApiConfigAll)
		routerapi.DELETE("/config/clear", api.ApiConfigClear)
		routerapi.GET("/config/get/:name", api.ApiConfigGet)

		routerapi.DELETE("/config/delete/:name", api.ApiConfigDelete)

		routerapi.POST("/config/update/:name", api.ApiConfigUpdate)

		routerapi.GET("/config/type/:type", api.ApiListConfigType)
		routerall := routerapi.Group("/all")
		api.AddAllRoutes(routerall)

		routermovies := routerapi.Group("/movies")
		api.AddMoviesRoutes(routermovies)

		routerseries := routerapi.Group("/series")
		api.AddSeriesRoutes(routerseries)
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
		ginpprof.Wrap(router)
	}

	server := &http.Server{
		Addr:    ":" + cfg_general.WebPort,
		Handler: router,
	}

	go func() {
		// service connections
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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

	database.DBImdb.Close()
	database.DB.Close()

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
