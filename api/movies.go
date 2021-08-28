// movies
package api

import (
	"database/sql"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
	"github.com/remeh/sizedwaitgroup"
)

func AddMoviesRoutes(routermovies *gin.RouterGroup) {

	routermovies.GET("/all/refresh", apirefreshMoviesInc)
	routermovies.GET("/all/refreshall", apirefreshMovies)

	routermovies.GET("/unmatched", func(ctx *gin.Context) {
		movies, _ := database.QueryMovieFileUnmatched(database.Query{})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})
	routermovies.GET("/", func(ctx *gin.Context) {
		movies, _ := database.QueryDbmovie(database.Query{})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})
	routermovies.POST("/", updateDBMovie)
	routermovies.DELETE("/:id", func(ctx *gin.Context) {
		database.DeleteRow("movies", database.Query{Where: "dbmovie_id=" + ctx.Param("id")})
		result, _ := database.DeleteRow("dbmovies", database.Query{Where: "id=" + ctx.Param("id")})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})

	routermovies.POST("/list/", updateMovie)

	routermovies.GET("/list/:name", func(ctx *gin.Context) {
		movies, _ := database.QueryResultMovies(database.Query{InnerJoin: "dbmovies on movies.dbmovie_id=dbmovies.id", Where: "movies.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
		// var counter int64
		// database.DB.Table("movies").Joins("inner join dbmovies on movies.dbmovie_id=dbmovies.id").Where("movies.listname=?", ctx.Param("name")).Count(&counter)
		// movies := make([]database.ResultMovies, 0, counter)
		// database.DB.Table("movies").Select("dbmovies.*, movies.listname, movies.lastscan, movies.blacklisted, movies.missing, movies.quality_reached, movies.quality_profile, movies.rootpath, movies.id AS MovieID").Joins("inner join dbmovies on movies.dbmovie_id=dbmovies.id").Where("movies.listname=?", ctx.Param("name")).Find(&movies)
		// ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": counter})
	})
	routermovies.DELETE("/list/:id", func(ctx *gin.Context) {
		result, _ := database.DeleteRow("movies", database.Query{Where: "id=" + ctx.Param("id")})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})

	routermovies.GET("/job/:job", apimoviesAllJobs)
	routermovies.GET("/job/:job/:name", apimoviesJobs)

	routermoviessearch := routermovies.Group("/search")
	{
		routermoviessearch.GET("/id/:id", apimoviesSearch)
		routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
	}
}

var allowedjobsmovies []string = []string{"rss", "data", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds"}

func apimoviesAllJobs(c *gin.Context) {
	allowed := false
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		go Movies_all_jobs(c.Param("job"), true)
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}
func apimoviesJobs(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)

	allowed := false
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		go Movies_single_jobs(cfg, c.Param("job"), c.Param("name"), "", true)
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}

func updateDBMovie(c *gin.Context) {
	var dbmovie database.Dbmovie
	if err := c.ShouldBindJSON(&dbmovie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbmovie.ID}})
	var inres sql.Result

	if counter == 0 {
		inres, _ = database.InsertArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug})
	} else {
		inres, _ = database.UpdateArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbmovie.ID}})
	}
	c.JSON(http.StatusOK, gin.H{"data": inres})
}

func updateMovie(c *gin.Context) {
	var movie database.Movie
	if err := c.ShouldBindJSON(&movie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{movie.ID}})
	var inres sql.Result

	if counter == 0 {
		inres, _ = database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath})
	} else {
		inres, _ = database.UpdateArray("dbmovies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{movie.ID}})
	}
	c.JSON(http.StatusOK, gin.H{"data": inres})
}

func apimoviesSearch(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)

	movie, _ := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})
	for idxmovie := range cfg.Movie {
		for idxlist := range cfg.Movie[idxmovie].Lists {
			if strings.EqualFold(cfg.Movie[idxmovie].Lists[idxlist].Name, movie.Listname) {
				go utils.SearchMovieSingle(cfg, movie, cfg.Movie[idxmovie], true)
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}

func apirefreshMovies(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)
	go RefreshMovies(cfg)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apirefreshMoviesInc(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)
	go RefreshMoviesInc(cfg)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apimoviesClearHistoryName(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)

	go Movies_single_jobs(cfg, "clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

var Lastmovie string

func importnewmoviessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	results := utils.Feeds(cfg, row, list)

	swg := sizedwaitgroup.New(cfg.General.WorkerMetadata)
	for idxmovie := range results.Movies {
		if strings.EqualFold(Lastmovie, results.Movies[idxmovie].ImdbID) && Lastmovie != "" {
			config.Slepping(false, 5)
		}
		Lastmovie = results.Movies[idxmovie].ImdbID
		logger.Log.Info("Import Movie ", idxmovie, " of ", len(results.Movies), " imdb: ", results.Movies[idxmovie].ImdbID)
		swg.Add()
		utils.JobImportMovies(cfg, results.Movies[idxmovie], row, list, &swg)
	}
	swg.Wait()
}

var LastMoviePath string
var LastMoviesFilePath string

func getnewmoviessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	defaultPrio := &utils.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
	defaultPrio.GetPriority(row, cfg.Quality[list.Template_quality])

	logger.Log.Info("Scan Movie File")
	filesfound := make([]string, 0, 5000)
	for idxpath := range row.Data {
		mappath := row.Data[idxpath].Template_path
		_, okmap := cfg.Path[mappath]
		if !okmap {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
		if strings.EqualFold(LastMoviePath, cfg.Path[mappath].Path) && LastMoviePath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastMoviePath = cfg.Path[mappath].Path
		filesfound_add := scanner.GetFilesGoDir(cfg.Path[mappath].Path, cfg.Path[mappath].AllowedVideoExtensions, cfg.Path[mappath].AllowedVideoExtensionsNoRename, cfg.Path[mappath].Blocked)
		filesfound = append(filesfound, filesfound_add...)
	}
	filesadded := scanner.GetFilesAdded(filesfound, list.Name)
	logger.Log.Info("Find Movie File")
	swf := sizedwaitgroup.New(cfg.General.WorkerParse)
	for idxfile := range filesadded {
		if strings.EqualFold(LastMoviesFilePath, filesadded[idxfile]) && LastMoviesFilePath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastMoviesFilePath = filesadded[idxfile]
		logger.Log.Info("Parse Movie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
		swf.Add()
		utils.JobImportMovieParseV2(cfg, filesadded[idxfile], row, list, true, *defaultPrio, &swf)
	}
	swf.Wait()
}

func checkmissingmoviessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	movies, _ := database.QueryMovies(database.Query{Where: "listname = ?", WhereArgs: []interface{}{list.Name}})

	swfile := sizedwaitgroup.New(cfg.General.WorkerFiles)
	for idx := range movies {
		moviefile, _ := database.QueryMovieFiles(database.Query{Select: "location", Where: "movie_id = ?", WhereArgs: []interface{}{movies[idx].ID}})
		for idxfile := range moviefile {
			swfile.Add()
			utils.JobImportFileCheck(moviefile[idxfile].Location, "movie", &swfile)
		}
	}
	swfile.Wait()
}

func checkmissingmoviesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	movies, _ := database.QueryMovies(database.Query{Where: "listname = ?", WhereArgs: []interface{}{list.Name}})

	for idxmovie := range movies {
		counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies[idxmovie].ID}})
		if counter >= 1 {
			if movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		} else {
			if !movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		}
	}
}

var LastMoviesStructure string

func moviesStructureSingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	swfile := sizedwaitgroup.New(cfg.General.WorkerFiles)

	for idxpath := range row.DataImport {
		mappath := ""
		_, okmap := cfg.Path[row.DataImport[idxpath].Template_path]
		if !okmap {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
		if len(row.Data) >= 1 {
			mappath = row.Data[0].Template_path
			_, okmap := cfg.Path[mappath]
			if !okmap {
				logger.Log.Error("Name in PathsMap not found")
				return
			}
		}
		if strings.EqualFold(LastMoviesStructure, cfg.Path[row.DataImport[idxpath].Template_path].Path) && LastMoviesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		LastMoviesStructure = cfg.Path[row.DataImport[idxpath].Template_path].Path
		swfile.Add()
		utils.StructureFolders(cfg, "movie", cfg.Path[row.DataImport[idxpath].Template_path], cfg.Path[mappath], row, list)
		//utils.JobStructureMovies(cfg.Path[mappathimport], cfg.Path[mappath], row, list, &swfile)
		swfile.Done()
	}
	swfile.Wait()
}

func RefreshMovies(cfg config.Cfg) {
	if cfg.General.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg.General.WorkerFiles)
	dbmovies, _ := database.QueryDbmovie(database.Query{})

	for idxmovie := range dbmovies {
		sw.Add()
		utils.JobReloadMovies(cfg, dbmovies[idxmovie], config.MediaTypeConfig{}, config.MediaListsConfig{}, &sw)
	}
	sw.Wait()
}

func RefreshMoviesInc(cfg config.Cfg) {
	if cfg.General.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg.General.WorkerFiles)
	dbmovies, _ := database.QueryDbmovie(database.Query{Limit: 100, OrderBy: "updated_at desc"})

	for idxmovie := range dbmovies {
		sw.Add()
		utils.JobReloadMovies(cfg, dbmovies[idxmovie], config.MediaTypeConfig{}, config.MediaListsConfig{}, &sw)
	}
	sw.Wait()
}

func Movies_all_jobs(job string, force bool) {
	cfg, _, _ := config.LoadCfg([]string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)

	for idxmovie := range cfg.Movie {
		Movies_single_jobs(cfg, job, cfg.Movie[idxmovie].Name, "", force)
	}
}
func Movies_all_jobs_cfg(cfg config.Cfg, job string, force bool) {
	for idxmovie := range cfg.Movie {
		Movies_single_jobs(cfg, job, cfg.Movie[idxmovie].Name, "", force)
	}
}

var MovieJobRunning map[string]bool

func Movies_single_jobs(cfg config.Cfg, job string, typename string, listname string, force bool) {
	if cfg.General.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", typename)
		return
	}
	jobName := job + typename + listname
	defer func() {
		database.ReadWriteMu.Lock()
		delete(MovieJobRunning, jobName)
		database.ReadWriteMu.Unlock()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := MovieJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		MovieJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()
	job = strings.ToLower(job)
	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, typename, "Movie", time.Now()})
	logger.Log.Info("Started Job: ", job, " for ", typename)
	_, ok := cfg.Movie[typename]
	if ok {
		switch job {
		case "searchmissingfull":
			utils.SearchMovieMissing(cfg, cfg.Movie[typename], 0, false)
		case "searchmissinginc":
			utils.SearchMovieMissing(cfg, cfg.Movie[typename], cfg.Movie[typename].Searchmissing_incremental, false)
		case "searchupgradefull":
			utils.SearchMovieUpgrade(cfg, cfg.Movie[typename], 0, false)
		case "searchupgradeinc":
			utils.SearchMovieUpgrade(cfg, cfg.Movie[typename], cfg.Movie[typename].Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			utils.SearchMovieMissing(cfg, cfg.Movie[typename], 0, true)
		case "searchmissinginctitle":
			utils.SearchMovieMissing(cfg, cfg.Movie[typename], cfg.Movie[typename].Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			utils.SearchMovieUpgrade(cfg, cfg.Movie[typename], 0, true)
		case "searchupgradeinctitle":
			utils.SearchMovieUpgrade(cfg, cfg.Movie[typename], cfg.Movie[typename].Searchupgrade_incremental, true)
		}
		qualis := make(map[string]bool, 10)
		for idxlist := range cfg.Movie[typename].Lists {
			if cfg.Movie[typename].Lists[idxlist].Name != listname && listname != "" {
				continue
			}
			if _, ok := qualis[cfg.Movie[typename].Lists[idxlist].Template_quality]; !ok {
				qualis[cfg.Movie[typename].Lists[idxlist].Template_quality] = true
			}
			switch job {
			case "data":
				config.Slepping(true, 6)
				getnewmoviessingle(cfg, cfg.Movie[typename], cfg.Movie[typename].Lists[idxlist])
			case "checkmissing":
				checkmissingmoviessingle(cfg, cfg.Movie[typename], cfg.Movie[typename].Lists[idxlist])
			case "checkmissingflag":
				checkmissingmoviesflag(cfg.Movie[typename], cfg.Movie[typename].Lists[idxlist])
			case "structure":
				moviesStructureSingle(cfg, cfg.Movie[typename], cfg.Movie[typename].Lists[idxlist])
			case "clearhistory":
				database.DeleteRow("movie_histories", database.Query{InnerJoin: "movies ON movies.id = movie_histories.movie_id", Where: "movies.listname=?", WhereArgs: []interface{}{typename}})
			case "feeds":
				config.Slepping(true, 6)
				importnewmoviessingle(cfg, cfg.Movie[typename], cfg.Movie[typename].Lists[idxlist])
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				utils.SearchMovieRSS(cfg, cfg.Movie[typename], qual)
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", typename)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", job, " for ", typename)
	debug.FreeOSMemory()
}
