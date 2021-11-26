// movies
package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
)

func AddMoviesRoutes(routermovies *gin.RouterGroup) {

	routermovies.GET("/all/refresh", apirefreshMoviesInc)
	routermovies.GET("/all/refreshall", apirefreshMovies)
	routermovies.GET("/refresh/:id", apirefreshMovie)

	routermovies.GET("/unmatched", ApiMovieListUnmatched)
	routermovies.GET("/", ApiMovieList)
	routermovies.POST("/", updateDBMovie)
	routermovies.DELETE("/:id", ApiMovieDelete)

	routermovies.POST("/list/", updateMovie)

	routermovies.GET("/list/:name", ApiMovieListGet)
	routermovies.DELETE("/list/:id", ApiMovieDeleteList)

	routermovies.GET("/job/:job", apimoviesAllJobs)
	routermovies.GET("/job/:job/:name", apimoviesJobs)

	routermoviessearch := routermovies.Group("/search")
	{
		routermoviessearch.GET("/id/:id", apimoviesSearch)
		routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
		routermoviessearch.GET("/history/clearid/:id", apiMoviesClearHistoryID)
	}
}

// @Summary List Movies
// @Description List Movies
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Success 200 {array} database.DbmovieJson
// @Failure 401 {object} string
// @Router /api/movies [get]
func ApiMovieList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}

	rows, _ := database.CountRows("dbmovies", query)
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
	movies, _ := database.QueryDbmovie(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary List Unmatched Movies
// @Description List Unmatched Movies
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Success 200 {array} database.MovieFileUnmatchedJson
// @Failure 401 {object} string
// @Router /api/movies/unmatched [get]
func ApiMovieListUnmatched(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	rows, _ := database.CountRows("movie_file_unmatcheds", query)
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
	movies, _ := database.QueryMovieFileUnmatched(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary Delete Movies
// @Description Deletes Movies from all lists
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID: ex. 1"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/{id} [delete]
func ApiMovieDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("movies", database.Query{Where: "dbmovie_id=" + ctx.Param("id")})
	_, err := database.DeleteRow("dbmovies", database.Query{Where: "id=" + ctx.Param("id")})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary List Movies (List)
// @Description Lists Movies from a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param name path string true "List Name: ex. EN"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultMoviesJson
// @Failure 401 {object} string
// @Router /api/movies/list/{name} [get]
func ApiMovieListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	query.InnerJoin = "dbmovies on movies.dbmovie_id=dbmovies.id"
	query.Where = "movies.listname=?"
	query.WhereArgs = []interface{}{ctx.Param("name")}

	rows, _ := database.CountRows("movies", query)
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
	movies, _ := database.QueryResultMovies(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary Delete a Movie (List)
// @Description Deletes a Movie from a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID: ex. 1"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/list/{id} [delete]
func ApiMovieDeleteList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.DeleteRow("movies", database.Query{Where: "id=" + ctx.Param("id")})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

var allowedjobsmovies []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds", "refresh", "refreshinc"}

// @Summary Start Jobs (All Lists)
// @Description Starts a Job
// @Tags movie
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 204 {object} string
// @Failure 401 {object} string
// @Router /api/movies/job/{job} [get]
func apimoviesAllJobs(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		switch c.Param("job") {
		case "data", "datafull", "checkmissing", "checkmissingflag", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			})
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			})
		case "feeds":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			})
		case "refresh":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshMovies()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshMoviesInc()
			})
		default:
			scheduler.QueueData.Dispatch(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary Start Jobs
// @Description Starts a Job
// @Tags movie
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param name path string false "List Name: ex. list"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 204 {object} string
// @Failure 401 {object} string
// @Router /api/movies/job/{job}/{name} [get]
func apimoviesJobs(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		switch c.Param("job") {
		case "data", "datafull", "checkmissing", "checkmissingflag", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "feeds":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "refresh":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshMovies()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshMoviesInc()
			})
		default:
			scheduler.QueueData.Dispatch(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary Update Movie (Global)
// @Description Updates or creates a movie
// @Tags movie
// @Accept  json
// @Produce  json
// @Param movie body database.DbmovieJson true "Movie"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/movies [post]
func updateDBMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var dbmovie database.DbmovieJson
	if err := c.ShouldBindJSON(&dbmovie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbmovie.ID}})
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug})
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbmovie.ID}})
	}
	if err == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, err)
	}
}

// @Summary Update Movie (List)
// @Description Updates or creates a movie in a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param movie body database.MovieJson true "Movie"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/movies/list [post]
func updateMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var movie database.MovieJson
	if err := c.ShouldBindJSON(&movie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{movie.ID}})
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath})
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{movie.ID}})
	}
	if err == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, err)
	}
}

// @Summary Search a movie
// @Description Searches for upgrades and missing
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/search/id/{id} [get]
func apimoviesSearch(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})
	movie_keys, _ := config.ConfigDB.Keys([]byte("movie_*"), 0, 0, true)

	for _, idxmovie := range movie_keys {
		var cfg_movie config.MediaTypeConfig
		config.ConfigGet(string(idxmovie), &cfg_movie)

		for idxlist := range cfg_movie.Lists {
			if strings.EqualFold(cfg_movie.Lists[idxlist].Name, movie.Listname) {
				scheduler.QueueSearch.Dispatch(func() {
					utils.SearchMovieSingle(movie, cfg_movie, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary Refresh Movies
// @Description Refreshes Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/all/refreshall [get]
func apirefreshMovies(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshMovies()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh a Movie
// @Description Refreshes specific Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/all/refresh/{id} [get]
func apirefreshMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshMovie(c.Param("id"))
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Movies (Incremental)
// @Description Refreshes Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/all/refresh [get]
func apirefreshMoviesInc(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshMoviesInc()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Clear History (Full List)
// @Description Clear Movies Download History
// @Tags movie
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/search/history/clear/{name} [get]
func apimoviesClearHistoryName(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	go utils.Movies_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}

// @Summary Clear History (Single Item)
// @Description Clear Episode Download History
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path string true "Movie ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/movies/search/history/clearid/{id} [get]
func apiMoviesClearHistoryID(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	inres, inerr := database.DeleteRow("movie_histories", database.Query{Where: "movie_id = ?)", WhereArgs: []interface{}{c.Param("id")}})

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
