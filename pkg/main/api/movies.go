// movies
package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/Kellerman81/go_media_downloader/worker"
	gin "github.com/gin-gonic/gin"
)

func AddMoviesRoutes(routermovies *gin.RouterGroup) {

	routermovies.GET("/all/refresh", apirefreshMoviesInc)
	routermovies.GET("/all/refreshall", apirefreshMovies)
	routermovies.GET("/refresh/:id", apirefreshMovie)

	routermovies.GET("/unmatched", apiMovieListUnmatched)
	routermovies.GET("/", apiMovieList)
	routermovies.POST("/", updateDBMovie)
	routermovies.DELETE("/:id", apiMovieDelete)

	routermovies.POST("/list/", updateMovie)

	routermovies.GET("/list/:name", apiMovieListGet)
	routermovies.DELETE("/list/:id", movieDeleteList)

	routermovies.GET("/job/:job", apimoviesAllJobs)
	routermovies.GET("/job/:job/:name", apimoviesJobs)

	routermovies.GET("/rss/search/list/:group", apiMoviesRssSearchList)

	routermoviessearch := routermovies.Group("/search")
	{
		routermoviessearch.GET("/id/:id", apimoviesSearch)
		routermoviessearch.GET("/list/:id", apimoviesSearchList)
		routermoviessearch.POST("/download/:id", apimoviesSearchDownload)
		routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
		routermoviessearch.GET("/history/clearid/:id", apiMoviesClearHistoryID)
	}
}

// @Summary      List Movies
// @Description  List Movies
// @Tags         movie
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.DbmovieJSON
// @Failure      401    {object}  string
// @Router       /api/movies [get]
func apiMovieList(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Querywithargs

	rows := database.QueryIntColumn("select count() from dbmovies")
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = limit
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = (page - 1) * limit
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryDbmovie(query), "total": rows})
}

// @Summary      List Unmatched Movies
// @Description  List Unmatched Movies
// @Tags         movie
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.MovieFileUnmatchedJSON
// @Failure      401    {object}  string
// @Router       /api/movies/unmatched [get]
func apiMovieListUnmatched(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Querywithargs
	rows := database.QueryIntColumn("select count() from movie_file_unmatcheds")
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = limit
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = (page - 1) * limit
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryMovieFileUnmatched(query), "total": rows})
}

// @Summary      Delete Movies
// @Description  Deletes Movies from all lists
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/{id} [delete]
func apiMovieDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow(false, "movies", "dbmovie_id=?", ctx.Param("id"))
	_, err := database.DeleteRow(false, "dbmovies", "id=?", ctx.Param("id"))

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      List Movies (List)
// @Description  Lists Movies from a list
// @Tags         movie
// @Param        name   path      string  true   "List Name: ex. EN"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.ResultMoviesJSON
// @Failure      401    {object}  string
// @Router       /api/movies/list/{name} [get]
func apiMovieListGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Querywithargs
	query.InnerJoin = "dbmovies on movies.dbmovie_id=dbmovies.id"
	query.Where = "movies.listname = ? COLLATE NOCASE"

	rows := database.QueryIntColumn("select count() from movies where listname = ? COLLATE NOCASE", ctx.Param("name"))
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = limit
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = (page - 1) * limit
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryResultMovies(query, ctx.Param("name")), "total": rows})
}

// @Summary      Delete a Movie (List)
// @Description  Deletes a Movie from a list
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/list/{id} [delete]
func movieDeleteList(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.DeleteRow(false, "movies", "id=?", ctx.Param("id"))

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

const allowedjobsmoviesstr = "rss,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// @Summary      Start Jobs (All Lists)
// @Description  Starts a Job
// @Tags         movie
// @Param        job  path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Success      200  {object}  string
// @Failure      204  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/job/{job} [get]
func apimoviesAllJobs(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsmoviesstr, ",") {
		if strings.EqualFold(allow, c.Param(logger.StrJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(logger.StrJobLower) + " started"

		//defer cfgMovie.Close()
		//defer cfg_list.Close()
		for idxp := range config.SettingsMedia {
			if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
				continue
			}

			cfgpstr := config.SettingsMedia[idxp].NamePrefix

			switch c.Param(logger.StrJobLower) {
			case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
				worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_"+cfgpstr, func() {
					utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Data"))
			case logger.StrRss, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_"+cfgpstr, func() {
					utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Search"))
			case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:
				for idxi := range config.SettingsMedia[idxp].Lists {
					if !config.SettingsMedia[idxp].Lists[idxi].Enabled {
						continue
					}
					if !config.CheckGroup("list_", config.SettingsMedia[idxp].Lists[idxi].TemplateList) {
						continue
					}

					if !config.SettingsList["list_"+config.SettingsMedia[idxp].Lists[idxi].TemplateList].Enabled {
						continue
					}
					listname := config.SettingsMedia[idxp].Lists[idxi].Name
					logger.Log.Debug().Str("listname", listname).Msg("dispatch movie")
					if c.Param(logger.StrJobLower) == logger.StrFeeds {
						worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_"+cfgpstr+"_"+listname, func() {
							utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, listname, true)
						}, "Feeds"))
					}
					if c.Param(logger.StrJobLower) == logger.StrCheckMissing || c.Param(logger.StrJobLower) == logger.StrCheckMissingFlag || c.Param(logger.StrJobLower) == logger.StrReachedFlag {
						worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_"+cfgpstr+"_"+listname, func() {
							utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, listname, true)
						}, "Data"))
					}
					//cfg_list.Close()
				}
			case "refresh":
				worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMovies, func() {
					utils.SingleJobs(logger.StrMovie, "refresh", logger.StrMovie, "", false)
				}, "Feeds"))
			case "refreshinc":
				worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMoviesInc, func() {
					utils.SingleJobs(logger.StrMovie, "refreshinc", logger.StrMovie, "", false)
				}, "Feeds"))
			default:
				worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_"+cfgpstr, func() {
					utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Data"))
			}
			//cfgMovie.Close()
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(logger.StrJobLower) + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Start Jobs
// @Description  Starts a Job
// @Tags         movie
// @Param        job   path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "List Name: ex. list"
// @Success      200   {object}  string
// @Failure      204   {object}  string
// @Failure      401   {object}  string
// @Router       /api/movies/job/{job}/{name} [get]
func apimoviesJobs(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsmoviesstr, ",") {
		if strings.EqualFold(allow, c.Param(logger.StrJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(logger.StrJobLower) + " started"
		cfgpstr := "movie_" + c.Param("name")

		switch c.Param(logger.StrJobLower) {
		case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
			worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_movies_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Data"))
		case logger.StrRss, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
			worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_movies_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Search"))
		case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:

			//defer cfgMovie.Close()
			//defer cfg_list.Close()
			for idxp := range config.SettingsMedia {
				if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
					continue
				}
				if strings.EqualFold(config.SettingsMedia[idxp].Name, c.Param("name")) {
					for idxlist := range config.SettingsMedia[idxp].Lists {
						if !config.SettingsMedia[idxp].Lists[idxlist].Enabled {
							continue
						}
						if !config.CheckGroup("list_", config.SettingsMedia[idxp].Lists[idxlist].TemplateList) {
							continue
						}

						if !config.SettingsList["list_"+config.SettingsMedia[idxp].Lists[idxlist].TemplateList].Enabled {
							continue
						}
						listname := config.SettingsMedia[idxp].Lists[idxlist].Name
						if c.Param(logger.StrJobLower) == logger.StrFeeds {
							logger.Log.Debug().Str(logger.StrTitle, config.SettingsMedia[idxp].Name).Str("List", config.SettingsMedia[idxp].Lists[idxlist].Name).Msg("add job")
							worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_movies_"+config.SettingsMedia[idxp].Name+"_"+config.SettingsMedia[idxp].Lists[idxlist].Name, func() {
								utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, listname, true)
							}, "Feeds"))
						}
						if c.Param(logger.StrJobLower) == logger.StrCheckMissing || c.Param(logger.StrJobLower) == logger.StrCheckMissingFlag || c.Param(logger.StrJobLower) == logger.StrReachedFlag {
							worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_movies_"+config.SettingsMedia[idxp].Name+"_"+config.SettingsMedia[idxp].Lists[idxlist].Name, func() {
								utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, listname, true)
							}, "Data"))
						}
						//cfg_list.Close()
					}
				}
				//cfgMovie.Close()
			}
		case "refresh":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMovies, func() {
				utils.SingleJobs(logger.StrMovie, "refresh", logger.StrMovie, "", false)
			}, "Feeds"))
		case "refreshinc":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMoviesInc, func() {
				utils.SingleJobs(logger.StrMovie, "refreshinc", logger.StrMovie, "", false)
			}, "Feeds"))
		default:
			worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_movies_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrMovie, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Data"))
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(logger.StrJobLower) + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Update Movie (Global)
// @Description  Updates or creates a movie
// @Tags         movie
// @Param        movie  body      database.DbmovieJSON  true  "Movie"
// @Success      200    {object}  string
// @Failure      400    {object}  string
// @Failure      401    {object}  string
// @Router       /api/movies [post]
func updateDBMovie(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var dbmovie database.DbmovieJSON
	if err := c.ShouldBindJSON(&dbmovie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.QueryIntColumn("select count() from dbmovies where id != 0 and id = ?", dbmovie.ID)
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug)
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			"id != 0 and id = ?", dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug, dbmovie.ID)
	}
	if err == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, err)
	}
}

// @Summary      Update Movie (List)
// @Description  Updates or creates a movie in a list
// @Tags         movie
// @Param        movie  body      database.MovieJSON  true  "Movie"
// @Success      200    {object}  string
// @Failure      400    {object}  string
// @Failure      401    {object}  string
// @Router       /api/movies/list [post]
func updateMovie(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var movie database.MovieJSON
	if err := c.ShouldBindJSON(&movie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.QueryIntColumn("select count() from dbmovies where id != 0 and id = ?", movie.ID)
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath)
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			"id != 0 and id = ?", movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath, movie.ID)
	}
	if err == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, err)
	}
}

// @Summary      Search a movie
// @Description  Searches for upgrades and missing
// @Tags         movie
// @Param        id   path      int  true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/search/id/{id} [get]
func apimoviesSearch(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&movie)
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}

		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, movie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchmovie_movies_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(int(movie.ID)), func() {
					//searcher.SearchMyMedia(cfgdata.(config.MediaTypeConfig).NamePrefix, database.QueryStringColumn(database.QueryMoviesGetQualityByID, movie.ID), logger.StrMovie, logger.StrMovie, 0, 0, false, movie.ID, true)
					results, err := searcher.MovieSearch(config.SettingsMedia[idxp].NamePrefix, movie.ID, false, true)

					if err != nil {
						if err != nil && err != logger.ErrDisabled {
							logger.Log.Error().Err(err).Uint("id", movie.ID).Str("typ", logger.StrMovie).Msg("Search Failed")
						}
					} else {
						if results == nil || len(results.Accepted) == 0 {
							results.Close()
						} else {
							results.Download(logger.StrMovie, "movie_"+config.SettingsMedia[idxp].Name)
						}
					}
					results.Close()
				}, "Search"))
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a movie (List ok, nok)
// @Description  Searches for upgrades and missing
// @Tags         movie
// @Param        id             path      int     true   "Movie ID"
// @Success      200            {object}  string
// @Failure      401            {object}  string
// @Router       /api/movies/search/list/{id} [get]
func apimoviesSearchList(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&movie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, movie.Listname) {
				searchresults, err := searcher.MovieSearch(config.SettingsMedia[idxp].NamePrefix, movie.ID, false, titlesearch)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					//searchnow.Close()
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
				//searchnow.Close()
				searchresults.Close()
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Movie RSS (list ok, nok)
// @Description  Movie RSS
// @Tags         movie
// @Param        group  path      string  true  "Group Name"
// @Success      200    {object}  string
// @Failure      401    {object}  string
// @Router       /api/movies/rss/search/list/{group} [get]
func apiMoviesRssSearchList(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		if strings.EqualFold(config.SettingsMedia[idxp].Name, c.Param("group")) {
			templatequality := config.SettingsMedia[idxp].TemplateQuality
			searchresults, err := searcher.SearchRSS(config.SettingsMedia[idxp].NamePrefix, templatequality, logger.StrMovie, true)
			if err != nil {
				str := "failed with " + err.Error()
				c.JSON(http.StatusNotFound, str)
				return
			}
			c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
			searchresults.Close()
			return
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Download a movie (manual)
// @Description  Downloads a release after select
// @Tags         movie
// @Param        nzb  body      apiexternal.Nzbwithprio  true  "Nzb: Req. Title, Indexer, imdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/search/download/{id} [post]
func apimoviesSearchDownload(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&movie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//defer logger.ClearVar(&nzb)
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, movie.Listname) {
				downloader.DownloadMovie(config.SettingsMedia[idxp].NamePrefix, movie.ID, &nzb)
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Refresh Movies
// @Description  Refreshes Movie Metadata
// @Tags         movie
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/all/refreshall [get]
func apirefreshMovies(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMovies, func() {
		utils.SingleJobs(logger.StrMovie, "refresh", logger.StrMovie, "", false)
	}, "Feeds"))
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh a Movie
// @Description  Refreshes specific Movie Metadata
// @Tags         movie
// @Param        id   path      int  true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/refresh/{id} [get]
func apirefreshMovie(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc("Refresh Single Movie_"+c.Param(logger.StrID), func() {
		utils.RefreshMovie(c.Param(logger.StrID))
	}, "Feeds"))
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Movies (Incremental)
// @Description  Refreshes Movie Metadata
// @Tags         movie
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/all/refresh [get]
func apirefreshMoviesInc(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshMoviesInc, func() {
		utils.SingleJobs(logger.StrMovie, "refreshinc", logger.StrMovie, "", false)
	}, "Feeds"))
	c.JSON(http.StatusOK, "started")
}

// @Summary      Clear History (Full List)
// @Description  Clear Movies Download History
// @Tags         movie
// @Param        name  path      string  true  "List Name"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/movies/search/history/clear/{name} [get]
func apimoviesClearHistoryName(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.SingleJobs(logger.StrMovie, logger.StrClearHistory, "movie_"+c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}

// @Summary      Clear History (Single Item)
// @Description  Clear Episode Download History
// @Tags         movie
// @Param        id   path      string  true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/search/history/clearid/{id} [get]
func apiMoviesClearHistoryID(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	inres, inerr := database.DeleteRow(false, "movie_histories", "movie_id = ?", c.Param(logger.StrID))

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
