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
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
	"go.uber.org/zap"
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
	routermovies.DELETE("/list/:id", ApiMovieDeleteList)

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
// @Success      200    {object}   database.DbmovieJson
// @Failure      401    {object}  string
// @Router       /api/movies [get]
func apiMovieList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query

	rows, _ := database.CountRows("dbmovies", &query)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint64(limit)
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = uint64((page - 1) * limit)
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	movies, _ := database.QueryDbmovie(&query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary      List Unmatched Movies
// @Description  List Unmatched Movies
// @Tags         movie
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.MovieFileUnmatchedJson
// @Failure      401    {object}  string
// @Router       /api/movies/unmatched [get]
func apiMovieListUnmatched(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	rows, _ := database.CountRows("movie_file_unmatcheds", &query)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint64(limit)
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = uint64((page - 1) * limit)
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	movies, _ := database.QueryMovieFileUnmatched(&query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary      Delete Movies
// @Description  Deletes Movies from all lists
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/{id} [delete]
func apiMovieDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("movies", &database.Query{Where: "dbmovie_id=" + ctx.Param("id")})
	_, err := database.DeleteRow("dbmovies", &database.Query{Where: "id=" + ctx.Param("id")})

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
// @Success      200    {object}   database.ResultMoviesJson
// @Failure      401    {object}  string
// @Router       /api/movies/list/{name} [get]
func apiMovieListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	query.InnerJoin = "dbmovies on movies.dbmovie_id=dbmovies.id"
	query.Where = "movies.listname = ? COLLATE NOCASE"

	rows, _ := database.CountRows("movies", &query)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint64(limit)
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = uint64((page - 1) * limit)
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	movies, _ := database.QueryResultMovies(&query, ctx.Param("name"))
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary      Delete a Movie (List)
// @Description  Deletes a Movie from a list
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/list/{id} [delete]
func ApiMovieDeleteList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.DeleteRow("movies", &database.Query{Where: "id=" + ctx.Param("id")})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

const allowedjobsmoviesstr string = "rss,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// @Summary      Start Jobs (All Lists)
// @Description  Starts a Job
// @Tags         movie
// @Param        job  path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Success      200  {object}  string
// @Failure      204  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/job/{job} [get]
func apimoviesAllJobs(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsmoviesstr, ",") {
		if strings.EqualFold(allow, c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"

		//defer cfg_movie.Close()
		//defer cfg_list.Close()
		for idx := range config.Cfg.Movies {
			cfg_movie := "movie_" + config.Cfg.Movies[idx].Name

			switch c.Param("job") {
			case "data", "datafull", "structure", "clearhistory":
				scheduler.QueueData.Dispatch(c.Param("job")+"_"+cfg_movie, func() {
					utils.Movies_single_jobs(c.Param("job"), cfg_movie, "", true)
				})
			case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
				scheduler.QueueSearch.Dispatch(c.Param("job")+"_"+cfg_movie, func() {
					utils.Movies_single_jobs(c.Param("job"), cfg_movie, "", true)
				})
			case "feeds", "checkmissing", "checkmissingflag", "checkreachedflag":
				for idxlist := range config.Cfg.Movies[idx].Lists {
					if !config.Cfg.Movies[idx].Lists[idxlist].Enabled {
						continue
					}
					if !config.ConfigCheck("list_" + config.Cfg.Movies[idx].Lists[idxlist].Template_list) {
						continue
					}

					if !config.Cfg.Lists[config.Cfg.Movies[idx].Lists[idxlist].Template_list].Enabled {
						continue
					}
					listname := config.Cfg.Movies[idx].Lists[idxlist].Name
					if c.Param("job") == "feeds" {
						scheduler.QueueFeeds.Dispatch(c.Param("job")+"_"+cfg_movie+"_"+listname, func() {
							utils.Movies_single_jobs(c.Param("job"), cfg_movie, listname, true)
						})
					}
					if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
						scheduler.QueueData.Dispatch(c.Param("job")+"_"+cfg_movie+"_"+listname, func() {
							utils.Movies_single_jobs(c.Param("job"), cfg_movie, listname, true)
						})
					}
					//cfg_list.Close()
				}
			case "refresh":
				scheduler.QueueFeeds.Dispatch("Refresh Movies", func() {
					utils.RefreshMovies()
				})
			case "refreshinc":
				scheduler.QueueFeeds.Dispatch("Refresh Movies Incremental", func() {
					utils.RefreshMoviesInc()
				})
			default:
				scheduler.QueueData.Dispatch(c.Param("job")+"_"+cfg_movie, func() {
					utils.Movies_single_jobs(c.Param("job"), cfg_movie, "", true)
				})
			}
			//cfg_movie.Close()
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsmoviesstr, ",") {
		if strings.EqualFold(allow, c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		switch c.Param("job") {
		case "data", "datafull", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+c.Param("name"), func() {
				utils.Movies_single_jobs(c.Param("job"), "movie_"+c.Param("name"), "", true)
			})
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(c.Param("job")+"_movies_"+c.Param("name"), func() {
				utils.Movies_single_jobs(c.Param("job"), "movie_"+c.Param("name"), "", true)
			})
		case "feeds", "checkmissing", "checkmissingflag", "checkreachedflag":

			//defer cfg_movie.Close()
			//defer cfg_list.Close()
			for idx := range config.Cfg.Movies {
				if strings.EqualFold(config.Cfg.Movies[idx].Name, c.Param("name")) {
					for idxlist := range config.Cfg.Movies[idx].Lists {
						if !config.Cfg.Movies[idx].Lists[idxlist].Enabled {
							continue
						}
						if !config.ConfigCheck("list_" + config.Cfg.Movies[idx].Lists[idxlist].Template_list) {
							continue
						}

						if !config.Cfg.Lists[config.Cfg.Movies[idx].Lists[idxlist].Template_list].Enabled {
							continue
						}
						listname := config.Cfg.Movies[idx].Lists[idxlist].Name
						if c.Param("job") == "feeds" {
							logger.Log.GlobalLogger.Debug("add job ", zap.String("Title", config.Cfg.Movies[idx].Name), zap.String("List", config.Cfg.Movies[idx].Lists[idxlist].Name))
							scheduler.QueueFeeds.Dispatch(c.Param("job")+"_movies_"+config.Cfg.Movies[idx].Name+"_"+config.Cfg.Movies[idx].Lists[idxlist].Name, func() {
								utils.Movies_single_jobs(c.Param("job"), "movie_"+c.Param("name"), listname, true)
							})
						}
						if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
							scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+config.Cfg.Movies[idx].Name+"_"+config.Cfg.Movies[idx].Lists[idxlist].Name, func() {
								utils.Movies_single_jobs(c.Param("job"), "movie_"+c.Param("name"), listname, true)
							})
						}
						//cfg_list.Close()
					}
				}
				//cfg_movie.Close()
			}
		case "refresh":
			scheduler.QueueFeeds.Dispatch("Refresh Movies", func() {
				utils.RefreshMovies()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch("Refresh Movies Incremental", func() {
				utils.RefreshMoviesInc()
			})
		default:
			scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+c.Param("name"), func() {
				utils.Movies_single_jobs(c.Param("job"), "movie_"+c.Param("name"), "", true)
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Update Movie (Global)
// @Description  Updates or creates a movie
// @Tags         movie
// @Param        movie  body      database.DbmovieJson  true  "Movie"
// @Success      200    {object}  string
// @Failure      400    {object}  string
// @Failure      401    {object}  string
// @Router       /api/movies [post]
func updateDBMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var dbmovie database.DbmovieJson
	if err := c.ShouldBindJSON(&dbmovie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", &database.Query{Where: "id != 0 and id = ?"}, dbmovie.ID)
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("dbmovies", &logger.InStringArrayStruct{Arr: []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"}},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug})
	} else {
		inres, err = database.UpdateArray("dbmovies", &logger.InStringArrayStruct{Arr: []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"}},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
			&database.Query{Where: "id != 0 and id = ?"}, dbmovie.ID)
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
// @Param        movie  body      database.MovieJson  true  "Movie"
// @Success      200    {object}  string
// @Failure      400    {object}  string
// @Failure      401    {object}  string
// @Router       /api/movies/list [post]
func updateMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var movie database.MovieJson
	if err := c.ShouldBindJSON(&movie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", &database.Query{Where: "id != 0 and id = ?"}, movie.ID)
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("movies", &logger.InStringArrayStruct{Arr: []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"}},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath})
	} else {
		inres, err = database.UpdateArray("dbmovies", &logger.InStringArrayStruct{Arr: []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"}},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath},
			&database.Query{Where: "id != 0 and id = ?"}, movie.ID)
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(&database.Query{Where: "id = ?"}, c.Param("id"))
	//defer logger.ClearVar(&movie)

	for idx := range config.Cfg.Movies {
		configTemplate := "movie_" + config.Cfg.Movies[idx].Name
		for idxlist := range config.Cfg.Movies[idx].Lists {
			if strings.EqualFold(config.Cfg.Movies[idx].Lists[idxlist].Name, movie.Listname) {
				scheduler.QueueSearch.Dispatch("searchmovie_movies_"+config.Cfg.Movies[idx].Name+"_"+strconv.Itoa(int(movie.ID)), func() {
					searcher.SearchMovieSingle(movie.ID, configTemplate, true)
				})
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(&database.Query{Where: "id = ?"}, c.Param("id"))
	//defer logger.ClearVar(&movie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}

	for idx := range config.Cfg.Movies {
		configTemplate := "movie_" + config.Cfg.Movies[idx].Name
		for idxlist := range config.Cfg.Movies[idx].Lists {
			if strings.EqualFold(config.Cfg.Movies[idx].Lists[idxlist].Name, movie.Listname) {
				searchnow := searcher.NewSearcher(configTemplate, movie.QualityProfile)
				defer searchnow.Close()
				searchresults, err := searchnow.MovieSearch(movie.ID, false, titlesearch)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					//searchnow.Close()
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
				//searchnow.Close()
				defer searchresults.Close()
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}

	for idx := range config.Cfg.Movies {
		configTemplate := "movie_" + config.Cfg.Movies[idx].Name
		if strings.EqualFold(config.Cfg.Movies[idx].Name, c.Param("group")) {
			searchnow := searcher.NewSearcher(configTemplate, config.Cfg.Movies[idx].Template_quality)
			defer searchnow.Close()
			searchresults, err := searchnow.SearchRSS("movie", true)
			if err != nil {
				str := "failed with " + err.Error()
				c.JSON(http.StatusNotFound, str)
				return
			}
			defer searchresults.Close()
			c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
			return
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Download a movie (manual)
// @Description  Downloads a release after select
// @Tags         movie
// @Param        nzb  body      nzb.NzbwithprioJson  true  "Nzb: Req. Title, Indexer, imdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/search/download/{id} [post]
func apimoviesSearchDownload(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(&database.Query{Where: "id = ?"}, c.Param("id"))
	//defer logger.ClearVar(&movie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//defer logger.ClearVar(&nzb)

	for idx := range config.Cfg.Movies {
		configTemplate := "movie_" + config.Cfg.Movies[idx].Name
		for idxlist := range config.Cfg.Movies[idx].Lists {
			if strings.EqualFold(config.Cfg.Movies[idx].Lists[idxlist].Name, movie.Listname) {
				downloadnow := downloader.NewDownloader(configTemplate)
				downloadnow.SetMovie(movie.ID)
				downloadnow.Nzb = nzb
				downloadnow.DownloadNzb()
				downloadnow.Close()
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch("Refresh Movies", func() {
		utils.RefreshMovies()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh a Movie
// @Description  Refreshes specific Movie Metadata
// @Tags         movie
// @Param        id   path      int  true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/all/refresh/{id} [get]
func apirefreshMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch("Refresh Single Movie_"+c.Param("id"), func() {
		utils.RefreshMovie(c.Param("id"))
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Movies (Incremental)
// @Description  Refreshes Movie Metadata
// @Tags         movie
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/all/refresh [get]
func apirefreshMoviesInc(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch("Refresh Movies Incremental", func() {
		utils.RefreshMoviesInc()
	})
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	name := "movie_" + c.Param("name")
	utils.Movies_single_jobs("clearhistory", name, "", true)
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	inres, inerr := database.DeleteRow("movie_histories", &database.Query{Where: "movie_id = ?"}, c.Param("id"))

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
