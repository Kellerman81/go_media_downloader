// series
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
	"github.com/gin-gonic/gin"
)

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.GET("/", ApiSeriesGet)
	routerseries.POST("/", updateDBSeries)
	routerseries.DELETE("/:id", ApiSeriesDelete)

	routerseries.GET("/list/:name", ApiSeriesListGet)
	routerseries.POST("/list/", updateSeries)
	routerseries.DELETE("/list/:id", ApiSeriesListDelete)

	routerseries.GET("/all/refresh", apirefreshSeriesInc)
	routerseries.GET("/all/refreshall", apirefreshSeries)
	routerseries.GET("/refresh/:id", apirefreshSerie)

	routerseries.GET("/job/:job", apiseriesAllJobs)
	routerseries.GET("/job/:job/:name", apiseriesJobs)

	routerseries.GET("/unmatched", ApiSeriesUnmatched)

	routerseries.GET("/episodes", ApiSeriesEpisodesGet)
	routerseries.POST("/episodes", updateDBEpisode)

	routerseries.GET("/episodes/:id", ApiSeriesEpisodesGetSingle)
	routerseries.DELETE("/episodes/:id", ApiSeriesEpisodesDelete)
	routerseries.POST("/episodes/list/", updateEpisode)
	routerseries.GET("/episodes/list/:id", ApiSeriesEpisodesListGet)
	routerseries.DELETE("/episodes/list/:id", ApiSeriesEpisodesListDelete)

	routerseriessearch := routerseries.Group("/search")
	{
		routerseriessearch.GET("/id/:id", apiSeriesSearch)
		routerseriessearch.GET("/id/:id/:season", apiSeriesSearchSeason)
		routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
	}

	routerseriesepisodessearch := routerseries.Group("/episodes/search")
	{
		routerseriesepisodessearch.GET("/id/:id", apiSeriesEpisodeSearch)
	}
}

var allowedjobsseries []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds", "refresh", "refreshinc"}

// @Summary List Series
// @Description List Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.Dbserie
// @Failure 401 {object} string
// @Router /api/series [get]
func ApiSeriesGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	rows, _ := database.CountRows("dbseries", query)
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
	series, _ := database.QueryDbserie(query)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
}

// @Summary Delete Series
// @Description Delete Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/{id} [delete]
func ApiSeriesDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("dbserie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("dbseries", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary List Series (List)
// @Description List Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultSeries
// @Failure 401 {object} string
// @Router /api/series/{name} [get]
func ApiSeriesListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	query.InnerJoin = "Dbseries on series.dbserie_id=dbseries.id"
	query.Where = "series.listname=?"
	query.WhereArgs = []interface{}{ctx.Param("name")}
	rows, _ := database.CountRows("series", query)
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
	movies, _ := database.QueryResultSeries(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary Delete Series (List)
// @Description Delete Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/list/{id} [delete]
func ApiSeriesListDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary List Series Unmatched
// @Description List Unmatched episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.SerieFileUnmatchedJson
// @Failure 401 {object} string
// @Router /api/series/unmatched [get]
func ApiSeriesUnmatched(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	rows, _ := database.CountRows("serie_file_unmatcheds", query)
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
	movies, _ := database.QuerySerieFileUnmatched(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary List Series Episodes
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.DbserieEpisodeJson
// @Failure 401 {object} string
// @Router /api/series/episodes [get]
func ApiSeriesEpisodesGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	rows, _ := database.CountRows("dbserie_episodes", query)
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
	series, _ := database.QueryDbserieEpisodes(query)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
}

// @Summary List Series Episodes (Single)
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.DbserieEpisodeJson
// @Failure 401 {object} string
// @Router /api/series/episodes/{id} [get]
func ApiSeriesEpisodesGetSingle(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	query.Where = "dbserie_id=?"
	query.WhereArgs = []interface{}{ctx.Param("id")}
	rows, _ := database.CountRows("dbserie_episodes", query)
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
	series, _ := database.QueryDbserieEpisodes(query)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
}

// @Summary Delete Episode
// @Description Delete Series Episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/episodes/{id} [delete]
func ApiSeriesEpisodesDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("dbserie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary List Series Episodes (List)
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultSerieEpisodesJson
// @Failure 401 {object} string
// @Router /api/series/episodes/list/{id} [get]
func ApiSeriesEpisodesListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	query := database.Query{}
	query.InnerJoin = "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id"
	query.Where = "series.id=?"
	query.WhereArgs = []interface{}{ctx.Param("id")}
	rows, _ := database.CountRows("serie_episodes", query)
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
	movies, _ := database.QueryResultSerieEpisodes(query)
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "total": rows})
}

// @Summary Delete Episode (List)
// @Description Delete Series Episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/episodes/list/{id} [delete]
func ApiSeriesEpisodesListDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("serie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary Start Jobs (All Lists)
// @Description Starts a Job
// @Tags series
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 204 {object} string
// @Failure 401 {object} string
// @Router /api/series/job/{job} [get]
func apiseriesAllJobs(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for idxallow := range allowedjobsseries {
		if strings.EqualFold(allowedjobsseries[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		switch c.Param("job") {
		case "data", "datafull", "checkmissing", "checkmissingflag", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			})
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			})
		case "feeds":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			})
		case "refresh":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshSeries()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshSeriesInc()
			})
		default:
			scheduler.QueueData.Dispatch(func() {
				utils.Series_all_jobs(c.Param("job"), true)
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
// @Tags series
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param name path string false "List Name: ex. list"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 204 {object} string
// @Failure 401 {object} string
// @Router /api/series/job/{job}/{name} [get]
func apiseriesJobs(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for idxallow := range allowedjobsseries {
		if strings.EqualFold(allowedjobsseries[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		switch c.Param("job") {
		case "data", "datafull", "checkmissing", "checkmissingflag", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "feeds":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		case "refresh":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshSeries()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(func() {
				utils.RefreshSeriesInc()
			})
		default:
			scheduler.QueueData.Dispatch(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary Update Series (Global)
// @Description Updates or creates a series
// @Tags series
// @Accept  json
// @Produce  json
// @Param series body database.Dbserie true "Series"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/series [post]
func updateDBSeries(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var dbserie database.Dbserie
	if err := c.BindJSON(&dbserie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbseries", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbserie.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
			[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
	} else {
		inres, inerr = database.UpdateArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
			[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbserie.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary Update Series Episodes (Global)
// @Description Updates or creates a episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param episode body database.DbserieEpisodeJson true "Episode"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/series/episodes [post]
func updateDBEpisode(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var dbserieepisode database.DbserieEpisodeJson
	if err := c.BindJSON(&dbserieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbserie_episodes", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbserieepisode.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
			[]interface{}{dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID})
	} else {
		inres, inerr = database.UpdateArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
			[]interface{}{dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{dbserieepisode.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary Update Series (List)
// @Description Updates or creates a series
// @Tags series
// @Accept  json
// @Produce  json
// @Param series body database.SerieJson true "Series"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/series/list [post]
func updateSeries(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	if err := c.ShouldBindJSON(&serie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("series", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serie.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("series", []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch})
	} else {
		inres, inerr = database.UpdateArray("series", []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serie.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary Update Series Episodes (List)
// @Description Updates or creates a episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param episode body database.SerieEpisodeJson true "Episode"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/series/episodes/list [post]
func updateEpisode(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	var serieepisode database.SerieEpisode
	if err := c.ShouldBindJSON(&serieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("serie_episodes", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serieepisode.ID}})
	var inres sql.Result
	var inerr error
	if counter == 0 {
		inres, inerr = database.InsertArray("serie_episodes",
			[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch})
	} else {
		inres, inerr = database.UpdateArray("serie_episodes", []string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serieepisode.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary Refresh Single Series
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Serie ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/all/refresh/{id} [get]
func apirefreshSerie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshSerie(c.Param("id"))
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Series
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/all/refreshall [get]
func apirefreshSeries(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshSeries()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Series Incremental
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/all/refresh [get]
func apirefreshSeriesInc(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(func() {
		utils.RefreshSeriesInc()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary Search a series
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/search/id/{id} [get]
func apiSeriesSearch(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		for idxlist := range cfg_serie.Lists {
			if strings.EqualFold(cfg_serie.Lists[idxlist].Name, serie.Listname) {
				scheduler.QueueSearch.Dispatch(func() {
					utils.SearchSerieSingle(serie, cfg_serie, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary Search a series
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param season path string true "Season"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/search/id/{id}/{season} [get]
func apiSeriesSearchSeason(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		for idxlist := range cfg_serie.Lists {
			if strings.EqualFold(cfg_serie.Lists[idxlist].Name, serie.Listname) {
				scheduler.QueueSearch.Dispatch(func() {
					utils.SearchSerieSeasonSingle(serie, c.Param("season"), cfg_serie, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary Search a episode
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/episodes/search/id/{id} [get]
func apiSeriesEpisodeSearch(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	serieepi, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepi.SerieID}})

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		for idxlist := range cfg_serie.Lists {
			if strings.EqualFold(cfg_serie.Lists[idxlist].Name, serie.Listname) {
				scheduler.QueueSearch.Dispatch(func() {
					utils.SearchSerieEpisodeSingle(serieepi, cfg_serie, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary Clear History (Full List)
// @Description Clear Episode Download History
// @Tags series
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/series/search/history/clear/{name} [get]
func apiSeriesClearHistoryName(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	go utils.Series_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}
