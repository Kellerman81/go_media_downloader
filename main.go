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

	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/ginview"
	"github.com/gin-gonic/gin"
	ginlog "github.com/toorop/gin-logrus"
)

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
	//parse, _ := utils.NewFileParser(cfg, "Bull.2016.S03E12.HDTV.x264-KILLERS (tvdb311945) (tvdb1234)", true, "series")
	//parse, _ := utils.NewFileParser(cfg, "Rampage.Capital.Punishment.2014.BRRIP.H264.AAC-MAJESTiC (tt3448226)", false, "movie")
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

	scheduler.InitScheduler()

	router := gin.New()
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
	routerapi := router.Group("/api/" + cfg_general.WebApiKey)
	{
		routerapi.GET("/fillimdb", func(ctx *gin.Context) {
			go utils.InitFillImdb()
		})
		routerapi.GET("/scheduler/stop", func(c *gin.Context) {
			scheduler.QueueData.Stop()
			scheduler.QueueFeeds.Stop()
			scheduler.QueueSearch.Stop()
		})
		routerapi.GET("/scheduler/start", func(c *gin.Context) {
			scheduler.QueueData.Start()
			scheduler.QueueFeeds.Start()
			scheduler.QueueSearch.Start()
		})
		routerapi.GET("/db/close", func(ctx *gin.Context) {
			database.DB.Close()
			database.DBImdb.Close()
		})
		routerapi.GET("/db/clear/:name", func(ctx *gin.Context) {
			database.ReadWriteMu.Lock()
			database.DB.Exec("DELETE FROM " + ctx.Param("name") + "; VACUUM;")
			database.ReadWriteMu.Unlock()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})
		routerapi.GET("/db/vacuum", func(ctx *gin.Context) {
			database.ReadWriteMu.Lock()
			database.DB.Exec("VACUUM;")
			database.ReadWriteMu.Unlock()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})
		routerapi.GET("/quality/:name/:config", func(c *gin.Context) {
			if !config.ConfigCheck(c.Param("name")) {
				c.JSON(http.StatusNotFound, gin.H{"data": "quality not found"})
				return
			}
			if !config.ConfigCheck(c.Param("config")) {
				c.JSON(http.StatusNotFound, gin.H{"data": "config not found"})
				return
			}
			var qual config.QualityConfig
			config.ConfigGet(c.Param("name"), &qual)
			var media config.MediaTypeConfig
			config.ConfigGet(c.Param("config"), &media)
			var parser []utils.ParseInfo
			for idxreso := range database.Getresolutions {
				wantedreso := false
				for idxwantreso := range qual.Wanted_resolution {
					if strings.EqualFold(qual.Wanted_resolution[idxwantreso], database.Getresolutions[idxreso].Name) {
						wantedreso = true
						break
					}
				}
				if !wantedreso {
					continue
				}
				for idxqual := range database.Getqualities {
					wantedqual := false
					for idxwantqual := range qual.Wanted_quality {
						if strings.EqualFold(qual.Wanted_quality[idxwantqual], database.Getqualities[idxqual].Name) {
							wantedqual = true
							break
						}
					}
					if !wantedqual {
						continue
					}
					for idxcodec := range database.Getcodecs {
						for idxaudio := range database.Getaudios {
							parse := utils.ParseInfo{
								Resolution:   database.Getresolutions[idxreso].Name,
								Quality:      database.Getqualities[idxqual].Name,
								Codec:        database.Getcodecs[idxcodec].Name,
								Audio:        database.Getaudios[idxaudio].Name,
								ResolutionID: database.Getresolutions[idxreso].ID,
								QualityID:    database.Getqualities[idxqual].ID,
								CodecID:      database.Getcodecs[idxcodec].ID,
								AudioID:      database.Getaudios[idxaudio].ID,
							}
							parse.GetIDPriority(media, qual)
							parser = append(parser, parse)
						}
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{"data": parser})
		})

		routerapi.GET("/config/all", func(ctx *gin.Context) {
			ctx.JSON(http.StatusOK, gin.H{"data": config.ConfigGetAll()})
		})
		routerapi.GET("/config/clear", func(ctx *gin.Context) {
			keys, _ := config.ConfigDB.Keys([]byte("*"), 0, 0, true)
			for _, idx := range keys {
				config.ConfigDB.Delete(string(idx))
			}
			config.ClearCfg()
			config.WriteCfg()
			ctx.JSON(http.StatusOK, gin.H{"data": config.ConfigGetAll()})
		})
		routerapi.GET("/config/get/:name", func(ctx *gin.Context) {
			configs := config.ConfigGetAll()
			ctx.JSON(http.StatusOK, gin.H{"data": configs[ctx.Param("name")]})
		})

		routerapi.DELETE("/config/delete/:name", func(ctx *gin.Context) {
			configs := config.ConfigGetAll()
			config.ConfigDB.Delete(ctx.Param("name"))
			delete(configs, ctx.Param("name"))
			config.UpdateCfg(configs)
			config.WriteCfg()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})

		routerapi.POST("/config/update/:name", func(ctx *gin.Context) {
			name := ctx.Param("name")
			configs := config.ConfigGetAll()
			if strings.HasPrefix(name, "general") {
				var getcfg config.GeneralConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "downloader_") {
				var getcfg config.DownloaderConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "imdb") {
				var getcfg config.ImdbConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "indexer") {
				var getcfg config.IndexersConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "list") {
				var getcfg config.ListsConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "serie") {
				var getcfg config.MediaTypeConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "movie") {
				var getcfg config.MediaTypeConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "notification") {
				var getcfg config.NotificationConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "path") {
				var getcfg config.PathsConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "quality") {
				var getcfg config.QualityConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "regex") {
				var getcfg config.RegexConfigIn
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			if strings.HasPrefix(name, "scheduler") {
				var getcfg config.SchedulerConfig
				if err := ctx.ShouldBindJSON(&getcfg); err != nil {
					ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				configs[ctx.Param("name")] = getcfg
			}
			config.UpdateCfg(configs)
			config.WriteCfg()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})

		routerapi.GET("/config/type/:type", func(ctx *gin.Context) {
			configs := config.ConfigGetAll()
			list := make(map[string]interface{})
			for name, value := range configs {
				if strings.HasPrefix(name, ctx.Param("type")) {
					list[name] = value
				}
			}
			ctx.JSON(http.StatusOK, gin.H{"data": list})
		})
		routerall := routerapi.Group("/all")
		api.AddAllRoutes(routerall)

		routermovies := routerapi.Group("/movies")
		api.AddMoviesRoutes(routermovies)

		routerseries := routerapi.Group("/series")
		api.AddSeriesRoutes(routerseries)
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
