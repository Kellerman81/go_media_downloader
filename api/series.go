// series
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
// @Param apikey query string true "apikey"
// @Success 200 {array} database.Dbserie
// @Failure 401 {object} string
// @Router /api/series [get]
func ApiSeriesGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	series, _ := database.QueryDbserie(database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
}

// @Summary Delete Series
// @Description Delete Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200
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
	database.DeleteRow("dbseries", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary List Series (List)
// @Description List Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultSeries
// @Failure 401 {object} string
// @Router /api/series/{name} [get]
func ApiSeriesListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QueryResultSeries(database.Query{InnerJoin: "Dbseries on series.dbserie_id=dbseries.id", Where: "series.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary Delete Series (List)
// @Description Delete Series
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/list/{id} [delete]
func ApiSeriesListDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary List Series Unmatched
// @Description List Unmatched episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {array} database.SerieFileUnmatchedJson
// @Failure 401 {object} string
// @Router /api/series/unmatched [get]
func ApiSeriesUnmatched(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QuerySerieFileUnmatched(database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary List Series Episodes
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {array} database.DbserieEpisodeJson
// @Failure 401 {object} string
// @Router /api/series/episodes [get]
func ApiSeriesEpisodesGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	series, _ := database.QueryDbserieEpisodes(database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
}

// @Summary List Series Episodes (Single)
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.DbserieEpisodeJson
// @Failure 401 {object} string
// @Router /api/series/episodes/{id} [get]
func ApiSeriesEpisodesGetSingle(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	series, _ := database.QueryDbserieEpisodes(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
}

// @Summary Delete Episode
// @Description Delete Series Episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/episodes/{id} [delete]
func ApiSeriesEpisodesDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("dbserie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary List Series Episodes (List)
// @Description List episodes
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.ResultSerieEpisodesJson
// @Failure 401 {object} string
// @Router /api/series/episodes/list/{id} [get]
func ApiSeriesEpisodesListGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	movies, _ := database.QueryResultSerieEpisodes(database.Query{InnerJoin: "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id", Where: "series.id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
}

// @Summary Delete Episode (List)
// @Description Delete Series Episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/episodes/list/{id} [delete]
func ApiSeriesEpisodesListDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Start Jobs (All Lists)
// @Description Starts a Job
// @Tags series
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name: ex. datafull"
// @Param apikey query string true "apikey"
// @Success 200
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
			scheduler.QueueData.DispatchIn(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.DispatchIn(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "feeds":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.Series_all_jobs(c.Param("job"), true)
			}, time.Second*1)
		case "refresh":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshSeries()
			}, time.Second*1)
		case "refreshinc":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshSeriesInc()
			}, time.Second*1)
		default:
			scheduler.QueueData.DispatchIn(func() {
				utils.Series_all_jobs(c.Param("job"), true)
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
// @Tags series
// @Accept  json
// @Produce  json
// @Param job path string true "Job Name: ex. datafull"
// @Param name path string false "List Name: ex. list"
// @Param apikey query string true "apikey"
// @Success 200
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
			scheduler.QueueData.DispatchIn(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "rss", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.DispatchIn(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "feeds":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		case "refresh":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshSeries()
			}, time.Second*1)
		case "refreshinc":
			scheduler.QueueFeeds.DispatchIn(func() {
				utils.RefreshSeriesInc()
			}, time.Second*1)
		default:
			scheduler.QueueData.DispatchIn(func() {
				utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
			}, time.Second*1)
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, returnval)
	}
}

// @Summary Update Series (Global)
// @Description Updates or creates a series
// @Tags series
// @Accept  json
// @Produce  json
// @Param series body database.Dbserie true "Series"
// @Param apikey query string true "apikey"
// @Success 200
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
	c.JSON(http.StatusOK, gin.H{"data": inres, "error": inerr})
}

// @Summary Update Series Episodes (Global)
// @Description Updates or creates a episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param episode body database.DbserieEpisodeJson true "Episode"
// @Param apikey query string true "apikey"
// @Success 200
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
	c.JSON(http.StatusOK, gin.H{"data": inres, "error": inerr})
}

// @Summary Update Series (List)
// @Description Updates or creates a series
// @Tags series
// @Accept  json
// @Produce  json
// @Param series body database.SerieJson true "Series"
// @Param apikey query string true "apikey"
// @Success 200
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

	if counter == 0 {
		inres, _ = database.InsertArray("series", []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch})
	} else {
		inres, _ = database.UpdateArray("series", []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serie.ID}})
	}
	c.JSON(http.StatusOK, gin.H{"data": inres})
}

// @Summary Update Series Episodes (List)
// @Description Updates or creates a episode
// @Tags series
// @Accept  json
// @Produce  json
// @Param episode body database.SerieEpisodeJson true "Episode"
// @Param apikey query string true "apikey"
// @Success 200
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

	if counter == 0 {
		inres, _ = database.InsertArray("serie_episodes",
			[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch})
	} else {
		inres, _ = database.UpdateArray("serie_episodes", []string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{serieepisode.ID}})
	}
	c.JSON(http.StatusOK, gin.H{"data": inres})
}

// @Summary Refresh Single Series
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Serie ID"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/all/refresh/{id} [get]
func apirefreshSerie(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshSerie(c.Param("id"))
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Series
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/all/refreshall [get]
func apirefreshSeries(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshSeries()
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Refresh Series Incremental
// @Description Refreshes Series Metadata
// @Tags series
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/all/refresh [get]
func apirefreshSeriesInc(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.DispatchIn(func() {
		utils.RefreshSeriesInc()
	}, time.Second*1)
	c.JSON(http.StatusOK, "started")
}

// @Summary Search a series
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param apikey query string true "apikey"
// @Success 200
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
				scheduler.QueueSearch.DispatchIn(func() {
					utils.SearchSerieSingle(serie, cfg_serie, true)
				}, time.Second*1)
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
}

// @Summary Search a series
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Series ID"
// @Param season path string true "Season"
// @Param apikey query string true "apikey"
// @Success 200
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
				scheduler.QueueSearch.DispatchIn(func() {
					utils.SearchSerieSeasonSingle(serie, c.Param("season"), cfg_serie, true)
				}, time.Second*1)
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
}

// @Summary Search a episode
// @Description Searches for upgrades and missing
// @Tags series
// @Accept  json
// @Produce  json
// @Param id path int true "Episode ID"
// @Param apikey query string true "apikey"
// @Success 200
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
				scheduler.QueueSearch.DispatchIn(func() {
					utils.SearchSerieEpisodeSingle(serieepi, cfg_serie, true)
				}, time.Second*1)
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
}

// @Summary Clear History (Full List)
// @Description Clear Episode Download History
// @Tags series
// @Accept  json
// @Produce  json
// @Param name path string true "List Name"
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/series/search/history/clear/{name} [get]
func apiSeriesClearHistoryName(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	go utils.Series_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}
