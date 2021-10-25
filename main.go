package main

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"os/signal"
	"strings"
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
	"github.com/sirupsen/logrus"

	"github.com/DeanThompson/ginpprof"

	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/ginview"
	"github.com/gin-gonic/gin"
	ginlog "github.com/toorop/gin-logrus"
)

// func importfiles(c *gin.Context) {
// 	db := database.InitDb()
// 	var list []database.Lists
// 	db.Table("Lists").Find(&list)
// 	for _, listrow := range list {
// 		fmt.Printf("Import List: %d", listrow.ID)
// 		importfileslist(listrow.Rootfolder, listrow.ID)
// 	}
// }
// GenericJobStruct is a generic structure we'll use for submitting a job,
// this can be any structure.
type GenericJobStruct struct {
	Name string
}

// An example function, ProcessWork.
func (gjs *GenericJobStruct) ProcessWork() error {
	// Generic process, let's print out something, then wait for a time:
	logger.Log.WithFields(logrus.Fields{"Name": gjs.Name}).Info("Processing some work.")
	time.Sleep(5 * time.Second)
	logger.Log.Info("Work finished.")
	return nil
}

func main() {
	// what := []string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Serie", "Imdb", "Downloader", "Notification", "List"}
	// cfg, f, errcfg := config.LoadCfg(what, config.Configfile)
	// if errcfg != nil {
	// 	fmt.Println(errcfg)
	// 	return
	// }

	pudb, _ := config.OpenConfig("config.db")
	config.ConfigDB = pudb
	scanner.CleanUpFolder("./backup", 10)
	pudge.BackupAll("")

	f, errcfg := config.LoadCfgDB(config.Configfile)
	if errcfg == nil {
		config.WatchDB(f, config.Configfile)
	}

	defer func() {
		config.ConfigDB.Close()
	}()

	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	logger.InitLogger(logger.LoggerConfig{
		LogLevel:     cfg_general.LogLevel,
		LogFileSize:  cfg_general.LogFileSize,
		LogFileCount: cfg_general.LogFileCount,
	})
	logger.Log.Infoln("Starting go_media_downloader")
	logger.Log.Infoln("Programmer: kellerman81")
	logger.Log.Infoln("Hint: Set Loglevel to Debug to see possible API Paths")
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
	database.InitQualities()
	database.GetVars()

	scheduler.InitScheduler()

	counter, _ := database.CountRows("dbmovies", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for movies")
		utils.InitFillImdb()
		utils.Movies_all_jobs("feeds", true)
		utils.Movies_all_jobs("data", true)
	}
	counter, _ = database.CountRows("dbseries", database.Query{})
	if counter == 0 {
		logger.Log.Infoln("Starting initial DB fill for series")
		utils.Series_all_jobs("feeds", true)
		utils.Series_all_jobs("data", true)
	}

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
		routerapi.GET("/clear/:name", func(ctx *gin.Context) {
			database.ReadWriteMu.Lock()
			database.DB.Exec("DELETE FROM " + ctx.Param("name") + "; VACUUM;")
			database.ReadWriteMu.Unlock()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})
		routerapi.GET("/vacuum", func(ctx *gin.Context) {
			database.ReadWriteMu.Lock()
			database.DB.Exec("VACUUM;")
			database.ReadWriteMu.Unlock()
			ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
		})

		routerall := routerapi.Group("/all")
		api.AddAllRoutes(routerall)

		routermovies := routerapi.Group("/movies")
		api.AddMoviesRoutes(routermovies)

		routerseries := routerapi.Group("/series")
		api.AddSeriesRoutes(routerseries)

		routerscheduler := routerapi.Group("/scheduler")
		scheduler.AddSchedulerRoutes(routerscheduler)
	}
	// router.GET("/stopmemprofile", func(ctx *gin.Context) {
	// 	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 		logger.Memprofiler.Stop()
	// 	}
	// })
	// router.GET("/startmemprofile", func(ctx *gin.Context) {
	// 	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 		logger.Memprofiler = profile.Start(profile.ProfilePath("."), profile.MemProfile, profile.MemProfileHeap)
	// 	}
	// })
	// router.GET("/stopcpuprofile", func(ctx *gin.Context) {
	// 	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 		logger.Cpuprofiler.Stop()
	// 	}
	// })
	// router.GET("/startcpuprofile", func(ctx *gin.Context) {
	// 	if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 		logger.Cpuprofiler = profile.Start(profile.ProfilePath("."), profile.CPUProfile)
	// 	}
	// })
	router.GET("/dbclose", func(ctx *gin.Context) {
		database.DB = nil
	})
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

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("receive interrupt signal")

	// if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 	logger.Cpuprofiler.Stop()
	// }
	// if strings.EqualFold(cfg_general.LogLevel, "Debug") {
	// 	logger.Memprofiler.Stop()
	// }

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
