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
		database.DeleteRow("dbmovies", database.Query{Where: "id=" + ctx.Param("id")})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	routermovies.POST("/list/", updateMovie)

	routermovies.GET("/list/:name", func(ctx *gin.Context) {
		movies, _ := database.QueryResultMovies(database.Query{InnerJoin: "dbmovies on movies.dbmovie_id=dbmovies.id", Where: "movies.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})
	routermovies.DELETE("/list/:id", func(ctx *gin.Context) {
		database.DeleteRow("movies", database.Query{Where: "id=" + ctx.Param("id")})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	routermovies.GET("/job/:job", apimoviesAllJobs)
	routermovies.GET("/job/:job/:name", apimoviesJobs)

	routermoviessearch := routermovies.Group("/search")
	{
		routermoviessearch.GET("/id/:id", apimoviesSearch)
		routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
	}
}

var allowedjobsmovies []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds", "refresh", "refreshinc"}

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
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}
func apimoviesJobs(c *gin.Context) {
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
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}

func apirefreshMovies(c *gin.Context) {
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMovies()
	}, time.Second*1)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}
func apirefreshMovie(c *gin.Context) {
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMovie(c.Param("id"))
	}, time.Second*1)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apirefreshMoviesInc(c *gin.Context) {
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshMoviesInc()
	}, time.Second*1)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apimoviesClearHistoryName(c *gin.Context) {
	go utils.Movies_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}
