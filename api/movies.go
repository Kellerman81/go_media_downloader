// movies
package api

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

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
	}
}

// @Summary List Movies
// @Description List Movies
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {array} database.DbmovieJson
// @Failure 401 {object} string
// @Router /api/movies [get]
func ApiMovieList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QueryDbmovieJson(database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary List Unmatched Movies
// @Description List Unmatched Movies
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {array} database.MovieFileUnmatchedJson
// @Failure 401 {object} string
// @Router /api/movies/unmatched [get]
func ApiMovieListUnmatched(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QueryMovieFileUnmatched(database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary Delete Movies
// @Description Deletes Movies from all lists
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID: ex. 1"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/{id} [delete]
func ApiMovieDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("movies", database.Query{Where: "dbmovie_id=" + ctx.Param("id")})
	database.DeleteRow("dbmovies", database.Query{Where: "id=" + ctx.Param("id")})
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary List Movies (List)
// @Description Lists Movies from a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param name path string true "List Name: ex. EN"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultMoviesJson
// @Failure 401 {object} string
// @Router /api/movies/list/{name} [get]
func ApiMovieListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QueryResultMovies(database.Query{InnerJoin: "dbmovies on movies.dbmovie_id=dbmovies.id", Where: "movies.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary Delete a Movie (List)
// @Description Deletes a Movie from a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID: ex. 1"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/list/{id} [delete]
func ApiMovieDeleteList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("movies", database.Query{Where: "id=" + ctx.Param("id")})
	ctx.JSON(http.StatusOK, "ok")
}

var allowedjobsmovies []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds", "refresh", "refreshinc"}

// @Summary Start Jobs (All Lists)
// @Description Starts a Job
// @Tags movie
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name: ex. datafull"
// @Param apikey query string true "apikey"
// @Success 200
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
			scheduler.QueueData.DispatchIn(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.DispatchIn(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "feeds":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "refresh":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshMovies()
			}, time.Second*1)
		case "refreshinc":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshMoviesInc()
			}, time.Second*1)
		default:
			scheduler.QueueData.DispatchIn(func() {
				utils.Movies_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, returnval)
	}
}

// @Summary Start Jobs
// @Description Starts a Job
// @Tags movie
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name: ex. datafull"
// @Param name path string false "List Name: ex. list"
// @Param apikey query string true "apikey"
// @Success 200
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
			scheduler.QueueData.DispatchIn(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.DispatchIn(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "feeds":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "refresh":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshMovies()
			}, time.Second*1)
		case "refreshinc":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshMoviesInc()
			}, time.Second*1)
		default:
			scheduler.QueueData.DispatchIn(func() {
				utils.Movies_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, returnval)
	}
}

// @Summary Update Movie (Global)
// @Description Updates or creates a movie
// @Tags movie
// @Accept  json
// @Produce  json
// @Param movie body database.DbmovieJson true "Movie"
// @Param apikey query string true "apikey"
// @Success 200
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

// @Summary Update Movie (List)
// @Description Updates or creates a movie in a list
// @Tags movie
// @Accept  json
// @Produce  json
// @Param movie body database.MovieJson true "Movie"
// @Param apikey query string true "apikey"
// @Success 200
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

// @Summary Search a movie
// @Description Searches for upgrades and missing
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID"
// @Param apikey query string true "apikey"
// @Success 200
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
				scheduler.QueueSearch.DispatchIn(func() {
					utils.SearchMovieSingle(movie, cfg_movie, true)
				}, time.Second*1)
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
}

// @Summary Refresh Movies
// @Description Refreshes Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/all/refreshall [get]
func apirefreshMovies(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMovies()
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh a Movie
// @Description Refreshes specific Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param id path int true "Movie ID"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/all/refresh/{id} [get]
func apirefreshMovie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMovie(c.Param("id"))
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Movies (Incremental)
// @Description Refreshes Movie Metadata
// @Tags movie
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/all/refresh [get]
func apirefreshMoviesInc(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMoviesInc()
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Clear History (Full List)
// @Description Clear Movies Download History
// @Tags movie
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/movies/search/history/clear/{name} [get]
func apimoviesClearHistoryName(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	go utils.Movies_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}
