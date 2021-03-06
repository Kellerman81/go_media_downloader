// movies
package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/utils"
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
	database.DeleteRow("movies", database.Query{Where: "dbmovie_id=" + ctx.Param("id")})
	_, err := database.DeleteRow("dbmovies", database.Query{Where: "id=" + ctx.Param("id")})

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
	query := database.Query{}
	query.InnerJoin = "dbmovies on movies.dbmovie_id=dbmovies.id"
	query.Where = "movies.listname = ?"
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
	_, err := database.DeleteRow("movies", database.Query{Where: "id=" + ctx.Param("id")})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

var allowedjobsmovies []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "checkreachedflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds", "refresh", "refreshinc"}

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
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"

		for _, idxmovie := range config.ConfigGetPrefix("movie_") {
			configTemplate := idxmovie
			cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)

			switch c.Param("job") {
			case "data", "datafull", "structure", "clearhistory":
				scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, "", true)
				})
			case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
				scheduler.QueueSearch.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, "", true)
				})
			case "feeds", "checkmissing", "checkmissingflag", "checkreachedflag":
				for idxlist := range cfg_movie.Lists {
					if !cfg_movie.Lists[idxlist].Enabled {
						continue
					}
					if !config.ConfigCheck("list_" + cfg_movie.Lists[idxlist].Template_list) {
						continue
					}
					cfg_list := config.ConfigGet("list_" + cfg_movie.Lists[idxlist].Template_list).Data.(config.ListsConfig)
					if !cfg_list.Enabled {
						continue
					}
					listname := cfg_movie.Lists[idxlist].Name
					if c.Param("job") == "feeds" {
						scheduler.QueueFeeds.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name+"_"+listname, func() {
							utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, listname, true)
						})
					}
					if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
						scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name+"_"+listname, func() {
							utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, listname, true)
						})
					}
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
				scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, "", true)
				})
			}
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
	for idxallow := range allowedjobsmovies {
		if strings.EqualFold(allowedjobsmovies[idxallow], c.Param("job")) {
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

			for _, idxmovie := range config.ConfigGetPrefix("movie_") {
				configTemplate := idxmovie
				if !config.ConfigCheck(configTemplate.Name) {
					continue
				}
				cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)
				if strings.EqualFold(cfg_movie.Name, c.Param("name")) {
					for idxlist := range cfg_movie.Lists {
						if !cfg_movie.Lists[idxlist].Enabled {
							continue
						}
						if !config.ConfigCheck("list_" + cfg_movie.Lists[idxlist].Template_list) {
							continue
						}
						cfg_list := config.ConfigGet("list_" + cfg_movie.Lists[idxlist].Template_list).Data.(config.ListsConfig)
						if !cfg_list.Enabled {
							continue
						}
						listname := cfg_movie.Lists[idxlist].Name
						if c.Param("job") == "feeds" {
							logger.Log.Debug("add job ", cfg_movie.Name, " ", cfg_movie.Lists[idxlist].Name)
							scheduler.QueueFeeds.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
								utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, listname, true)
							})
						}
						if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
							scheduler.QueueData.Dispatch(c.Param("job")+"_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
								utils.Movies_single_jobs(c.Param("job"), configTemplate.Name, listname, true)
							})
						}
					}
				}
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
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{dbmovie.ID}})
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug})
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
			database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{dbmovie.ID}})
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
	counter, _ := database.CountRows("dbmovies", database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{movie.ID}})
	var inres sql.Result
	var err error
	if counter == 0 {
		inres, err = database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath})
	} else {
		inres, err = database.UpdateArray("dbmovies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			[]interface{}{movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath},
			database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{movie.ID}})
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
	movie, _ := database.GetMovies(database.Query{Where: "id = ?", WhereArgs: []interface{}{c.Param("id")}})

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		configTemplate := idxmovie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)
		for idxlist := range cfg_movie.Lists {
			if strings.EqualFold(cfg_movie.Lists[idxlist].Name, movie.Listname) {
				scheduler.QueueSearch.Dispatch("searchmovie_movies_"+cfg_movie.Name+"_"+strconv.Itoa(int(movie.ID)), func() {
					searcher.SearchMovieSingle(movie.ID, configTemplate.Name, true)
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
	movie, _ := database.GetMovies(database.Query{Where: "id = ?", WhereArgs: []interface{}{c.Param("id")}})

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		configTemplate := idxmovie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)
		for idxlist := range cfg_movie.Lists {
			if strings.EqualFold(cfg_movie.Lists[idxlist].Name, movie.Listname) {
				searchnow := searcher.NewSearcher(configTemplate.Name, movie.QualityProfile)
				searchresults, err := searchnow.MovieSearch(movie.ID, false, titlesearch)
				if err != nil {
					c.JSON(http.StatusNotFound, "failed")
					searchnow.Close()
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
				searchnow.Close()
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
	var configTemplate config.Conf

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		configTemplate = idxmovie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)
		if strings.EqualFold(cfg_movie.Name, c.Param("group")) {
			searchnow := searcher.NewSearcher(configTemplate.Name, cfg_movie.Template_quality)
			searchresults, err := searchnow.SearchRSS("movie", true)
			if err != nil {
				c.JSON(http.StatusNotFound, "failed")
				searchnow.Close()
				return
			}
			searchnow.Close()
			c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
			return
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Download a movie (manual)
// @Description  Downloads a release after select
// @Tags         movie
// @Param        nzb  body      parser.NzbwithprioJson  true  "Nzb: Req. Title, Indexer, imdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "Movie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/movies/search/download/{id} [post]
func apimoviesSearchDownload(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	movie, _ := database.GetMovies(database.Query{Where: "id = ?", WhereArgs: []interface{}{c.Param("id")}})

	var nzb parser.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		configTemplate := idxmovie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)
		for idxlist := range cfg_movie.Lists {
			if strings.EqualFold(cfg_movie.Lists[idxlist].Name, movie.Listname) {
				downloadnow := downloader.NewDownloader(configTemplate.Name)
				downloadnow.SetMovie(movie.ID)
				downloadnow.DownloadNzb(nzb)
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
	inres, inerr := database.DeleteRow("movie_histories", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{c.Param("id")}})

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
