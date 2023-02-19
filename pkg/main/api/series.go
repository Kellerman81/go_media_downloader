// series
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
	"github.com/gin-gonic/gin"
)

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.GET("/", apiSeriesGet)
	routerseries.POST("/", updateDBSeries)
	routerseries.DELETE("/:id", apiSeriesDelete)

	routerseries.GET("/list/:name", apiSeriesListGet)
	routerseries.POST("/list/", updateSeries)
	routerseries.DELETE("/list/:id", apiSeriesListDelete)

	routerseries.GET("/all/refresh", apirefreshSeriesInc)
	routerseries.GET("/all/refreshall", apirefreshSeries)
	routerseries.GET("/refresh/:id", apirefreshSerie)

	routerseries.GET("/job/:job", apiseriesAllJobs)
	routerseries.GET("/job/:job/:name", apiseriesJobs)

	routerseries.GET("/unmatched", apiSeriesUnmatched)

	routerseries.GET("/episodes", apiSeriesEpisodesGet)
	routerseries.POST("/episodes", updateDBEpisode)

	routerseries.GET("/episodes/:id", apiSeriesEpisodesGetSingle)
	routerseries.DELETE("/episodes/:id", apiSeriesEpisodesDelete)
	routerseries.POST("/episodes/list/", updateEpisode)
	routerseries.GET("/episodes/list/:id", apiSeriesEpisodesListGet)
	routerseries.DELETE("/episodes/list/:id", apiSeriesEpisodesListDelete)
	routerseries.GET("/rss/search/list/:group", apiSeriesRssSearchList)

	routerseriessearch := routerseries.Group("/search")
	{
		routerseriessearch.GET("/id/:id", apiSeriesSearch)
		routerseriessearch.GET("/id/:id/:season", apiSeriesSearchSeason)
		routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
		routerseriessearch.GET("/history/clearid/:id", apiSeriesClearHistoryID)
	}

	routerseriessearchrss := routerseries.Group("/searchrss")
	{
		routerseriessearchrss.GET("/id/:id", apiSeriesSearchRSS)
		routerseriessearchrss.GET("/id/:id/:season", apiSeriesSearchRSSSeason)
	}

	routerseriesepisodessearch := routerseries.Group("/episodes/search")
	{
		routerseriesepisodessearch.GET("/id/:id", apiSeriesEpisodeSearch)
		routerseriesepisodessearch.GET("/list/:id", apiSeriesEpisodeSearchList)
		routerseriesepisodessearch.POST("/download/:id", apiSeriesEpisodeSearchDownload)
	}
}

const querybydbserieid = "dbserie_id = ?"

// @Summary      List Series
// @Description  List Series
// @Tags         series
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.Dbserie
// @Failure      401    {object}  string
// @Router       /api/series [get]
func apiSeriesGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	rows, _ := database.CountRows("dbseries", &database.Querywithargs{Query: query})
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
	var series []database.Dbserie
	database.QueryDbserie(&database.Querywithargs{Query: query}, &series)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      Delete Series
// @Description  Delete Series
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/{id} [delete]
func apiSeriesDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("dbserie_episodes", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("series", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("dbseries", &database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      List Series (List)
// @Description  List Series
// @Tags         series
// @Param        name   path      string  true   "List Name"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.ResultSeries
// @Failure      401    {object}  string
// @Router       /api/series/list/{name} [get]
func apiSeriesListGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	query.InnerJoin = "Dbseries on series.dbserie_id=dbseries.id"
	query.Where = "series.listname = ? COLLATE NOCASE"
	rows, _ := database.CountRows("series", &database.Querywithargs{Query: query})
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
	series, _ := database.QueryResultSeries(&database.Querywithargs{Query: query, Args: []interface{}{ctx.Param("name")}})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      Delete Series (List)
// @Description  Delete Series
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/list/{id} [delete]
func apiSeriesListDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("series", &database.Querywithargs{Query: database.Query{Where: querybydbserieid}, Args: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      List Series Unmatched
// @Description  List Unmatched episodes
// @Tags         series
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.SerieFileUnmatchedJSON
// @Failure      401    {object}  string
// @Router       /api/series/unmatched [get]
func apiSeriesUnmatched(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	rows, _ := database.CountRows("serie_file_unmatcheds", &database.Querywithargs{Query: query})
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
	series, _ := database.QuerySerieFileUnmatched(&database.Querywithargs{Query: query})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      List Series Episodes
// @Description  List episodes
// @Tags         series
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.DbserieEpisodeJSON
// @Failure      401    {object}  string
// @Router       /api/series/episodes [get]
func apiSeriesEpisodesGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	rows, _ := database.CountRows("dbserie_episodes", &database.Querywithargs{Query: query})
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
	var series []database.DbserieEpisode
	database.QueryDbserieEpisodes(&database.Querywithargs{Query: query}, &series)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      List Series Episodes (Single)
// @Description  List episodes
// @Tags         series
// @Param        id     path      int     true   "Series ID"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.DbserieEpisodeJSON
// @Failure      401    {object}  string
// @Router       /api/series/episodes/{id} [get]
func apiSeriesEpisodesGetSingle(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	query.Where = querybydbserieid
	rows, _ := database.CountRows("dbserie_episodes", &database.Querywithargs{Query: query})
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
	var series []database.DbserieEpisode
	database.QueryDbserieEpisodes(&database.Querywithargs{Query: query, Args: []interface{}{ctx.Param("id")}}, &series)
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      Delete Episode
// @Description  Delete Series Episode
// @Tags         series
// @Param        id   path      int  true  "Episode ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/episodes/{id} [delete]
func apiSeriesEpisodesDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", &database.Querywithargs{Query: database.Query{Where: "dbserie_episode_id = ?"}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: "dbserie_episode_id = ?"}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episodes", &database.Querywithargs{Query: database.Query{Where: "dbserie_episode_id = ?"}, Args: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("dbserie_episodes", &database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      List Series Episodes (List)
// @Description  List episodes
// @Tags         series
// @Param        id     path      int     true   "Series ID"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.ResultSerieEpisodesJSON
// @Failure      401    {object}  string
// @Router       /api/series/episodes/list/{id} [get]
func apiSeriesEpisodesListGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	query.InnerJoin = "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id"
	query.Where = "series.id = ?"
	rows, _ := database.CountRows("serie_episodes", &database.Querywithargs{Query: query})
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
	series, _ := database.QueryResultSerieEpisodes(&database.Querywithargs{Query: query, Args: []interface{}{ctx.Param("id")}})
	ctx.JSON(http.StatusOK, gin.H{"data": series, "total": rows})
	series = nil
}

// @Summary      Delete Episode (List)
// @Description  Delete Series Episode
// @Tags         series
// @Param        id   path      int  true  "Episode ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/episodes/list/{id} [delete]
func apiSeriesEpisodesListDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("serie_episode_files", &database.Querywithargs{Query: database.Query{Where: "serie_episode_id = ?"}, Args: []interface{}{ctx.Param("id")}})
	database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: "serie_episode_id = ?"}, Args: []interface{}{ctx.Param("id")}})
	_, err := database.DeleteRow("serie_episodes", &database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{ctx.Param("id")}})

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

const allowedjobsseriesstr = "rss,rssseasons,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// @Summary      Start Jobs (All Lists)
// @Description  Starts a Job
// @Tags         series
// @Param        job  path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Success      200  {object}  string
// @Failure      204  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/job/{job} [get]
func apiseriesAllJobs(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsseriesstr, ",") {
		if strings.EqualFold(allow, c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"

		//defer cfgSerie.Close()
		//defer cfg_list.Close()
		for idx := range config.Cfg.Series {
			cfgpstr := "serie_" + config.Cfg.Series[idx].Name

			switch c.Param("job") {
			case "data", "datafull", "structure", "clearhistory":
				scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+config.Cfg.Series[idx].Name, func() {
					utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
				})
			case "rss", "rssseasons", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
				scheduler.QueueSearch.Dispatch(c.Param("job")+"_series_"+config.Cfg.Series[idx].Name, func() {
					utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
				})
			case "feeds", "checkmissing", "checkmissingflag", "checkreachedflag":
				for idxlist := range config.Cfg.Series[idx].Lists {
					if !config.Cfg.Series[idx].Lists[idxlist].Enabled {
						continue
					}
					if !config.Check("list_" + config.Cfg.Series[idx].Lists[idxlist].TemplateList) {
						continue
					}

					if !config.Cfg.Lists[config.Cfg.Series[idx].Lists[idxlist].TemplateList].Enabled {
						continue
					}
					listname := config.Cfg.Series[idx].Lists[idxlist].Name
					if c.Param("job") == "feeds" {
						scheduler.QueueFeeds.Dispatch(c.Param("job")+"_series_"+config.Cfg.Series[idx].Name, func() {
							utils.SingleJobs("series", c.Param("job"), cfgpstr, listname, true)
						})
					} else if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
						scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+config.Cfg.Series[idx].Name, func() {
							utils.SingleJobs("series", c.Param("job"), cfgpstr, listname, true)
						})
					}
					//cfg_list.Close()
				}
			case "refresh":
			case "refreshinc":

			default:
				scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+config.Cfg.Series[idx].Name, func() {
					utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
				})
			}
			//cfgSerie.Close()
		}
		switch c.Param("job") {
		case "refresh":
			scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeries, func() {
				utils.RefreshSeries()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeriesInc, func() {
				utils.RefreshSeriesInc()
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Start Jobs
// @Description  Starts a Job
// @Tags         series
// @Param        job   path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "List Name: ex. list"
// @Success      200   {object}  string
// @Failure      204   {object}  string
// @Failure      401   {object}  string
// @Router       /api/series/job/{job}/{name} [get]
func apiseriesJobs(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	allowed := false
	for _, allow := range strings.Split(allowedjobsseriesstr, ",") {
		if strings.EqualFold(allow, c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		cfgpstr := "serie_" + c.Param("name")
		switch c.Param("job") {
		case "data", "datafull", "structure", "clearhistory":
			scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+c.Param("name"), func() {
				utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
			})
		case "rss", "rssseasons", "searchmissingfull", "searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle", "searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle":
			scheduler.QueueSearch.Dispatch(c.Param("job")+"_series_"+c.Param("name"), func() {
				utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
			})
		case "feeds", "checkmissing", "checkmissingflag", "checkreachedflag":
			for idxlist := range config.Cfg.Series[c.Param("name")].Lists {
				if !config.Cfg.Series[c.Param("name")].Lists[idxlist].Enabled {
					continue
				}
				if !config.Check("list_" + config.Cfg.Series[c.Param("name")].Lists[idxlist].TemplateList) {
					continue
				}

				if !config.Cfg.Lists[config.Cfg.Series[c.Param("name")].Lists[idxlist].TemplateList].Enabled {
					continue
				}
				listname := config.Cfg.Series[c.Param("name")].Lists[idxlist].Name
				if c.Param("job") == "feeds" {
					scheduler.QueueFeeds.Dispatch(c.Param("job")+"_series_"+c.Param("name"), func() {
						utils.SingleJobs("series", c.Param("job"), cfgpstr, listname, true)
					})
				}
				if c.Param("job") == "checkmissing" || c.Param("job") == "checkmissingflag" || c.Param("job") == "checkreachedflag" {
					scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+c.Param("name"), func() {
						utils.SingleJobs("series", c.Param("job"), cfgpstr, listname, true)
					})
				}
				//cfg_list.Close()
			}
		case "refresh":
			scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeries, func() {
				utils.RefreshSeries()
			})
		case "refreshinc":
			scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeriesInc, func() {
				utils.RefreshSeriesInc()
			})
		default:
			scheduler.QueueData.Dispatch(c.Param("job")+"_series_"+c.Param("name"), func() {
				utils.SingleJobs("series", c.Param("job"), cfgpstr, "", true)
			})
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Update Series (Global)
// @Description  Updates or creates a series
// @Tags         series
// @Param        series  body      database.Dbserie  true  "Series"
// @Success      200  {object}  string
// @Failure      400     {object}  string
// @Failure      401  {object}  string
// @Router       /api/series [post]
func updateDBSeries(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var dbserie database.Dbserie
	if err := c.BindJSON(&dbserie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbseries", &database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{dbserie.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbseries", &logger.InStringArrayStruct{Arr: []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"}},
			[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
	} else {
		inres, inerr = database.UpdateArray("dbseries", &logger.InStringArrayStruct{Arr: []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"}},
			[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby},
			&database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{dbserie.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series Episodes (Global)
// @Description  Updates or creates a episode
// @Tags         series
// @Param        episode  body      database.DbserieEpisodeJSON  true  "Episode"
// @Success      200      {object}  string
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/series/episodes [post]
func updateDBEpisode(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var dbserieepisode database.DbserieEpisodeJSON
	if err := c.BindJSON(&dbserieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("dbserie_episodes", &database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{dbserieepisode.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbserie_episodes", &logger.InStringArrayStruct{Arr: []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"}},
			[]interface{}{dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID})
	} else {
		inres, inerr = database.UpdateArray("dbserie_episodes", &logger.InStringArrayStruct{Arr: []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"}},
			[]interface{}{dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID},
			&database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{dbserieepisode.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series (List)
// @Description  Updates or creates a series
// @Tags         series
// @Param        series  body      database.SerieJSON  true  "Series"
// @Success      200  {object}  string
// @Failure      400     {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/list [post]
func updateSeries(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	if err := c.ShouldBindJSON(&serie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("series", &database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{serie.ID}})
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("series", &logger.InStringArrayStruct{Arr: []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"}},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch})
	} else {
		inres, inerr = database.UpdateArray("series", &logger.InStringArrayStruct{Arr: []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"}},
			[]interface{}{serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch},
			&database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{serie.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series Episodes (List)
// @Description  Updates or creates a episode
// @Tags         series
// @Param        episode  body      database.SerieEpisodeJSON  true  "Episode"
// @Success      200      {object}  string
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/series/episodes/list [post]
func updateEpisode(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serieepisode database.SerieEpisode
	if err := c.ShouldBindJSON(&serieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("serie_episodes", &database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{serieepisode.ID}})
	var inres sql.Result
	var inerr error
	if counter == 0 {
		inres, inerr = database.InsertArray("serie_episodes",
			&logger.InStringArrayStruct{Arr: []string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"}},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch})
	} else {
		inres, inerr = database.UpdateArray("serie_episodes", &logger.InStringArrayStruct{Arr: []string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"}},
			[]interface{}{serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch},
			&database.Querywithargs{Query: database.Query{Where: "id != 0 and id = ?"}, Args: []interface{}{serieepisode.ID}})
	}
	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Refresh Single Series
// @Description  Refreshes Series Metadata
// @Tags         series
// @Param        id   path      int  true  "Serie ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/refresh/{id} [get]
func apirefreshSerie(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch("Refresh Single Serie", func() {
		utils.RefreshSerie(c.Param("id"))
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Series
// @Description  Refreshes Series Metadata
// @Tags         series
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/all/refreshall [get]
func apirefreshSeries(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeries, func() {
		utils.RefreshSeries()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Series Incremental
// @Description  Refreshes Series Metadata
// @Tags         series
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/all/refresh [get]
func apirefreshSeriesInc(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueFeeds.Dispatch(logger.StrRefreshSeriesInc, func() {
		utils.RefreshSeriesInc()
	})
	c.JSON(http.StatusOK, "started")
}

// @Summary      Search a series (all seasons)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/search/id/{id} [get]
func apiSeriesSearch(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{c.Param("id")}}, &serie)
	//defer logger.ClearVar(&serie)

	for idx := range config.Cfg.Series {
		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				scheduler.QueueSearch.Dispatch("searchseries_series_"+config.Cfg.Series[idx].Name+"_"+strconv.Itoa(int(serie.ID)), func() {
					searcher.SearchSerieSingle(serie.ID, &cfgp, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a series (one season)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Param        season  path      string  true  "Season"
// @Success      200    {object}  string
// @Failure      401    {object}  string
// @Router       /api/series/search/id/{id}/{season} [get]
func apiSeriesSearchSeason(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{c.Param("id")}}, &serie)
	//defer logger.ClearVar(&serie)

	for idx := range config.Cfg.Series {
		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				scheduler.QueueSearch.Dispatch("searchseriesseason_series_"+config.Cfg.Series[idx].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					searcher.SearchSerieSeasonSingle(serie.ID, c.Param("season"), &cfgp, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a series (any season - one search call)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id      path      int     true  "Series ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/searchrss/id/{id} [get]
func apiSeriesSearchRSS(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.Query{Select: "id, listname", Where: logger.FilterByID}, Args: []interface{}{c.Param("id")}}, &serie)
	//defer logger.ClearVar(&serie)

	for idx := range config.Cfg.Series {
		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				scheduler.QueueSearch.Dispatch("searchseriesseason_series_"+config.Cfg.Series[idx].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					searcher.SearchSerieRSSSeasonSingle(serie.ID, 0, false, &cfgp)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a series (one season - one search call)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id      path      int     true  "Series ID"
// @Param        season  path      string  true  "Season"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/series/searchrss/id/{id}/{season} [get]
func apiSeriesSearchRSSSeason(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.Query{Select: "id, listname", Where: logger.FilterByID}, Args: []interface{}{c.Param("id")}}, &serie)
	//defer logger.ClearVar(&serie)

	seasonint, _ := strconv.Atoi(c.Param("season"))

	for idx := range config.Cfg.Series {
		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				scheduler.QueueSearch.Dispatch("searchseriesseason_series_"+config.Cfg.Series[idx].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					searcher.SearchSerieRSSSeasonSingle(serie.ID, seasonint, true, &cfgp)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a episode
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id   path      int  true  "Episode ID"
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/series/episodes/search/id/{id} [get]
func apiSeriesEpisodeSearch(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	id, _ := strconv.Atoi(c.Param("id"))
	uid := uint(id)
	var serieid uint
	database.QueryColumn(&database.Querywithargs{QueryString: "select serie_id from serie_episodes where id = ?", Args: []interface{}{c.Param("id")}}, &serieid)
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{serieid}}, &serie)
	//defer logger.ClearVar(&serie)

	for idx := range config.Cfg.Series {

		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				scheduler.QueueSearch.Dispatch("searchseriesepisode_series_"+config.Cfg.Series[idx].Name+"_"+strconv.Itoa(id), func() {
					searcher.SearchSerieEpisodeSingle(uid, &cfgp, true)
				})
				c.JSON(http.StatusOK, "started")
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Search a episode (list ok, nok)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id             path      int     true   "Episode ID"
// @Param        searchByTitle  query     string  false  "searchByTitle"
// @Success      200            {object}  string
// @Failure      401            {object}  string
// @Router       /api/series/episodes/search/list/{id} [get]
func apiSeriesEpisodeSearchList(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serieepi database.SerieEpisode
	database.GetSerieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{c.Param("id")}}, &serieepi)
	//defer logger.ClearVar(&serieepi)

	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{serieepi.SerieID}}, &serie)
	//defer logger.ClearVar(&serie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}

	for idx := range config.Cfg.Series {

		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				searchresults, err := searcher.SeriesSearch(&cfgp, serieepi.ID, false, titlesearch)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					cfgp.Close()
					//searchnow.Close()
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
				searchresults.Close()
				cfgp.Close()
				//searchnow.Close()
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Series RSS (list ok, nok)
// @Description  Series RSS
// @Tags         series
// @Param        group  path      string  true  "Group Name"
// @Success      200     {object}  string
// @Failure      401     {object}  string
// @Router       /api/series/rss/search/list/{group} [get]
func apiSeriesRssSearchList(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}

	for idx := range config.Cfg.Series {

		if strings.EqualFold(config.Cfg.Series[idx].Name, c.Param("group")) {
			cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
			searchresults, err := searcher.SearchRSS(&cfgp, config.Cfg.Series[idx].TemplateQuality, "series", true)
			if err != nil {
				str := "failed with " + err.Error()
				c.JSON(http.StatusNotFound, str)
				cfgp.Close()
				return
			}
			c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Nzbs, "rejected": searchresults.Rejected})
			cfgp.Close()
			searchresults.Close()
			return
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Download a episode (manual)
// @Description  Downloads a release after select
// @Tags         series
// @Param        nzb  body      apiexternal.Nzbwithprio  true  "Nzb: Req. Title, Indexer, tvdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "Episode ID"
// @Success      200     {object}  string
// @Failure      401     {object}  string
// @Router       /api/series/episodes/search/download/{id} [post]
func apiSeriesEpisodeSearchDownload(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	var serieepi database.SerieEpisode
	database.GetSerieEpisodes(&database.Querywithargs{Query: database.Query{Select: "id, serie_id, missing", Where: logger.FilterByID}, Args: []interface{}{c.Param("id")}}, &serieepi)
	//defer logger.ClearVar(&serieepi)
	var serie database.Serie
	database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{serieepi.SerieID}}, &serie)
	//defer logger.ClearVar(&serie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//defer logger.ClearVar(&nzb)

	for idx := range config.Cfg.Series {
		for idxlist := range config.Cfg.Series[idx].Lists {
			if strings.EqualFold(config.Cfg.Series[idx].Lists[idxlist].Name, serie.Listname) {
				cfgp := config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
				downloader.DownloadSeriesEpisode(&cfgp, serieepi.ID, &nzb)
				c.JSON(http.StatusOK, "started")
				cfgp.Close()
				return
			}
		}
	}
	c.JSON(http.StatusNoContent, "Nothing Done")
}

// @Summary      Clear History (Full List)
// @Description  Clear Episode Download History
// @Tags         series
// @Param        name  path      string  true  "List Name"
// @Success      200     {object}  string
// @Failure      401     {object}  string
// @Router       /api/series/search/history/clear/{name} [get]
func apiSeriesClearHistoryName(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.SingleJobs("series", "clearhistory", "serie_"+c.Param("name"), "", true)
	c.JSON(http.StatusOK, "started")
}

// @Summary      Clear History (Single Item)
// @Description  Clear Episode Download History
// @Tags         series
// @Param        id   path      string  true  "Episode ID"
// @Success      200     {object}  string
// @Failure      401     {object}  string
// @Router       /api/series/search/history/clearid/{id} [get]
func apiSeriesClearHistoryID(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	inres, inerr := database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: "serie_episode_id = ?"}, Args: []interface{}{c.Param("id")}})

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
