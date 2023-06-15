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
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/Kellerman81/go_media_downloader/worker"
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
		routerseriessearchrss.GET("/list/id/:id", apiSeriesSearchRSSList)
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
	var query database.Querywithargs
	rows := database.QueryIntColumn("select count() from dbseries")
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QueryDbserie(query), "total": rows})
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
	database.DeleteRow(false, "serie_episode_files", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, "serie_episode_histories", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, "serie_episodes", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, "dbserie_episodes", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, logger.StrSeries, querybydbserieid, ctx.Param("id"))
	_, err := database.DeleteRow(false, "dbseries", logger.FilterByID, ctx.Param("id"))

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
	var query database.Querywithargs
	query.InnerJoin = "dbseries on series.dbserie_id=dbseries.id"
	query.Where = "series.listname = ? COLLATE NOCASE"
	rows := database.QueryIntColumn("select count() from series where listname = ?", ctx.Param("name"))
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QueryResultSeries(query, ctx.Param("name")), "total": rows})
	//series = nil
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
	database.DeleteRow(false, "serie_episode_files", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, "serie_episode_histories", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(false, "serie_episodes", querybydbserieid, ctx.Param("id"))
	_, err := database.DeleteRow(false, logger.StrSeries, querybydbserieid, ctx.Param("id"))

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
	var query database.Querywithargs
	rows := database.QueryIntColumn("select count() from serie_file_unmatcheds")
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QuerySerieFileUnmatched(query), "total": rows})
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
	var query database.Querywithargs
	rows := database.QueryIntColumn("select count() from dbserie_episodes")
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QueryDbserieEpisodes(query), "total": rows})
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
	var query database.Querywithargs
	query.Where = querybydbserieid
	rows := database.QueryCountColumn("series", "dbserie_id = ?", ctx.Param("id"))
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QueryDbserieEpisodes(query, ctx.Param("id")), "total": rows})
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
	database.DeleteRow(false, "serie_episode_files", "dbserie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow(false, "serie_episode_histories", "dbserie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow(false, "serie_episodes", "dbserie_episode_id = ?", ctx.Param("id"))
	_, err := database.DeleteRow(false, "dbserie_episodes", logger.FilterByID, ctx.Param("id"))

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
	var query database.Querywithargs
	query.InnerJoin = "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id"
	query.Where = "series.id = ?"
	rows := database.QueryCountColumn("serie_episodes", "serie_id = ?", ctx.Param("id"))
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
	ctx.JSON(http.StatusOK, gin.H{logger.StrData: database.QueryResultSerieEpisodes(query, ctx.Param("id")), "total": rows})
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
	database.DeleteRow(false, "serie_episode_files", "serie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow(false, "serie_episode_histories", "serie_episode_id = ?", ctx.Param("id"))
	_, err := database.DeleteRow(false, "serie_episodes", logger.FilterByID, ctx.Param("id"))

	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

const allowedjobsseriesstr = "rss,rssseasons,rssseasonsall,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

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
		if strings.EqualFold(allow, c.Param(logger.StrJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(logger.StrJobLower) + " started"

		//defer cfgSerie.Close()
		//defer cfg_list.Close()
		for idxp := range config.SettingsMedia {
			if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
				continue
			}

			cfgpstr := "serie_" + config.SettingsMedia[idxp].Name

			switch c.Param(logger.StrJobLower) {
			case logger.StrData, logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
				worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+config.SettingsMedia[idxp].Name, func() {
					utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Data"))
			case logger.StrRss, logger.StrRssSeasons, logger.StrRssSeasonsAll, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+config.SettingsMedia[idxp].Name, func() {
					utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Search"))
			case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:
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
						worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+config.SettingsMedia[idxp].Name, func() {
							utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, listname, true)
						}, "Feeds"))
					} else if c.Param(logger.StrJobLower) == logger.StrCheckMissing || c.Param(logger.StrJobLower) == logger.StrCheckMissingFlag || c.Param(logger.StrJobLower) == logger.StrReachedFlag {
						worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+config.SettingsMedia[idxp].Name, func() {
							utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, listname, true)
						}, "Data"))
					}
					//cfg_list.Close()
				}
			case "refresh":
			case "refreshinc":

			default:
				worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+config.SettingsMedia[idxp].Name, func() {
					utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
				}, "Data"))
			}
			//cfgSerie.Close()
		}
		switch c.Param(logger.StrJobLower) {
		case "refresh":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeries, func() {
				utils.SingleJobs(logger.StrSeries, "refresh", logger.StrSeries, "", false)
			}, "Feeds"))
		case "refreshinc":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeriesInc, func() {
				utils.SingleJobs(logger.StrSeries, "refreshinc", logger.StrSeries, "", false)
			}, "Feeds"))
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(logger.StrJobLower) + " not allowed!"
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
		if strings.EqualFold(allow, c.Param(logger.StrJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(logger.StrJobLower) + " started"
		cfgpstr := "serie_" + c.Param("name")
		switch c.Param(logger.StrJobLower) {
		case logger.StrData, logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
			worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Data"))
		case logger.StrRss, logger.StrRssSeasons, logger.StrRssSeasonsAll, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
			worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Search"))
		case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:
			for _, cfglists := range config.SettingsMedia[c.Param("name")].Lists {
				if !cfglists.Enabled {
					continue
				}
				if !config.CheckGroup("list_", cfglists.TemplateList) {
					continue
				}

				if !config.SettingsList["list_"+cfglists.TemplateList].Enabled {
					continue
				}
				listname := cfglists.Name
				if c.Param(logger.StrJobLower) == logger.StrFeeds {
					worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+c.Param("name"), func() {
						utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, listname, true)
					}, "Feeds"))
				}
				if c.Param(logger.StrJobLower) == logger.StrCheckMissing || c.Param(logger.StrJobLower) == logger.StrCheckMissingFlag || c.Param(logger.StrJobLower) == logger.StrReachedFlag {
					worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+c.Param("name"), func() {
						utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, listname, true)
					}, "Data"))
				}
				//cfg_list.Close()
			}
		case "refresh":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeries, func() {
				utils.SingleJobs(logger.StrSeries, "refresh", logger.StrSeries, "", false)
			}, "Feeds"))
		case "refreshinc":
			worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeriesInc, func() {
				utils.SingleJobs(logger.StrSeries, "refreshinc", logger.StrSeries, "", false)
			}, "Feeds"))
		default:
			worker.Dispatch(worker.WorkerPoolFiles, worker.NewJobFunc(c.Param(logger.StrJobLower)+"_series_"+c.Param("name"), func() {
				utils.SingleJobs(logger.StrSeries, c.Param(logger.StrJobLower), cfgpstr, "", true)
			}, "Data"))
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(logger.StrJobLower) + " not allowed!"
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
	counter := database.QueryIntColumn("select count() from dbseries where id != 0 and id = ?", dbserie.ID)
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
			dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby)
	} else {
		inres, inerr = database.UpdateArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
			"id != 0 and id = ?", dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby, dbserie.ID)
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
	counter := database.QueryIntColumn("select count() from dbserie_episodes where id != 0 and id = ?", dbserieepisode.ID)
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
			dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID)
	} else {
		inres, inerr = database.UpdateArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
			"id != 0 and id = ?", dbserieepisode.Episode, dbserieepisode.Season, dbserieepisode.Identifier, dbserieepisode.Title, dbserieepisode.FirstAired, dbserieepisode.Overview, dbserieepisode.Poster, dbserieepisode.DbserieID, dbserieepisode.ID)
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
	counter := database.QueryIntColumn("select count() from series where id != 0 and id = ?", serie.ID)
	var inres sql.Result
	var inerr error

	if counter == 0 {
		inres, inerr = database.InsertArray(logger.StrSeries, []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch)
	} else {
		inres, inerr = database.UpdateArray(logger.StrSeries, []string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			"id != 0 and id = ?", serie.DbserieID, serie.Listname, serie.Rootpath, serie.DontUpgrade, serie.DontSearch, serie.ID)
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
	counter := database.QueryIntColumn("select count() from serie_episodes where id != 0 and id = ?", serieepisode.ID)
	var inres sql.Result
	var inerr error
	if counter == 0 {
		inres, inerr = database.InsertArray("serie_episodes",
			[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch)
	} else {
		inres, inerr = database.UpdateArray("serie_episodes", []string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id", "blacklisted", "quality_reached", "dont_upgrade", "dont_search"},
			"id != 0 and id = ?", serieepisode.DbserieID, serieepisode.SerieID, serieepisode.Missing, serieepisode.QualityProfile, serieepisode.DbserieEpisodeID, serieepisode.Blacklisted, serieepisode.QualityReached, serieepisode.DontUpgrade, serieepisode.DontSearch, serieepisode.ID)
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
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc("Refresh Single Serie", func() {
		utils.RefreshSerie(c.Param(logger.StrID))
	}, "Feeds"))
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
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeries, func() {
		utils.SingleJobs(logger.StrSeries, "refresh", logger.StrSeries, "", false)
	}, "Feeds"))
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
	worker.Dispatch(worker.WorkerPoolMetadata, worker.NewJobFunc(logger.StrRefreshSeriesInc, func() {
		utils.SingleJobs(logger.StrSeries, "refreshinc", logger.StrSeries, "", false)
	}, "Feeds"))
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
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serie)

	var episodes *[]uint
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchseries_series_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(int(serie.ID)), func() {
					episodes = database.QueryStaticUintArrayNoError(true, database.QueryCountColumn("serie_episodes", "serie_id = ?", serie.ID), database.QuerySerieEpisodesGetIDBySerie, serie.ID)
					for idxepisode := range *episodes {
						results, err := searcher.SeriesSearch("serie_"+config.SettingsMedia[idxp].Name, (*episodes)[idxepisode], false, true)

						if err != nil {
							if err != nil && err != logger.ErrDisabled {
								logger.Log.Error().Err(err).Uint("id", (*episodes)[idxepisode]).Str("typ", logger.StrSeries).Msg("Search Failed")
							}
						} else {
							if results == nil || len(results.Accepted) == 0 {
								results.Close()
							} else {
								results.Download(logger.StrSeries, "serie_"+config.SettingsMedia[idxp].Name)

							}
						}
						results.Close()
						//searcher.SearchMyMedia("serie_"+cfgdata.(config.MediaTypeConfig).Name, database.QueryStringColumn(database.QuerySerieEpisodesGetQualityBySerieID, episodes[idxepisode]), logger.StrSeries, logger.StrSeries, 0, 0, false, episodes[idxepisode], true)
					}
					logger.Clear(episodes)
				}, "Search"))
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
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serie)

	var episodes *[]uint
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchseriesseason_series_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					episodes = database.QueryStaticUintArrayNoError(true, database.QueryIntColumn("select count() from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", serie.ID, c.Param("season")), "select id from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", serie.ID, c.Param("season"))
					for idxepisode := range *episodes {
						results, err := searcher.SeriesSearch("serie_"+config.SettingsMedia[idxp].Name, (*episodes)[idxepisode], false, true)

						if err != nil {
							if err != nil && err != logger.ErrDisabled {
								logger.Log.Error().Err(err).Uint("id", (*episodes)[idxepisode]).Str("typ", logger.StrSeries).Msg("Search Failed")
							}
						} else {
							if results == nil || len(results.Accepted) == 0 {
								results.Close()
							} else {
								results.Download(logger.StrSeries, "serie_"+config.SettingsMedia[idxp].Name)
							}
						}
						results.Close()
						//searcher.SearchMyMedia("serie_"+cfgdata.(config.MediaTypeConfig).Name, database.QueryStringColumn(database.QuerySerieEpisodesGetQualityBySerieID, episodes[idxepisode]), logger.StrSeries, logger.StrSeries, 0, 0, false, episodes[idxepisode], true)
					}
					logger.Clear(episodes)
				}, "Search"))
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
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serie)

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchseriesseason_series_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					searcher.SearchSerieRSSSeasonSingle(serie.ID, 0, false, "serie_"+config.SettingsMedia[idxp].Name)
				}, "Search"))
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
// @Router       /api/series/searchrss/list/id/{id} [get]
func apiSeriesSearchRSSList(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serie)

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				searchresults, _ := searcher.SearchSerieRSSSeasonSingle(serie.ID, 0, false, "serie_"+config.SettingsMedia[idxp].Name)
				if len(searchresults.Raw) == 0 {
					c.JSON(http.StatusNoContent, "Failed")
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
				searchresults.Close()
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
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serie)

	seasonint, _ := strconv.Atoi(c.Param("season"))

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchseriesseason_series_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func() {
					searcher.SearchSerieRSSSeasonSingle(serie.ID, seasonint, true, "serie_"+config.SettingsMedia[idxp].Name)
				}, "Search"))
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
	id, _ := strconv.Atoi(c.Param(logger.StrID))
	uid := uint(id)
	serieid := database.QueryUintColumn(database.QuerySerieEpisodesGetSerieIDByID, c.Param(logger.StrID))
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieid)
	//defer logger.ClearVar(&serie)

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}

		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch(worker.WorkerPoolSearch, worker.NewJobFunc("searchseriesepisode_series_"+config.SettingsMedia[idxp].Name+"_"+strconv.Itoa(id), func() {
					//searcher.SearchMyMedia("serie_"+cfgdata.(config.MediaTypeConfig).Name, database.QueryStringColumn(database.QuerySerieEpisodesGetQualityBySerieID, uid), logger.StrSeries, logger.StrSeries, 0, 0, false, uid, true)
					results, err := searcher.SeriesSearch("serie_"+config.SettingsMedia[idxp].Name, uid, false, true)

					if err != nil {
						if err != nil && err != logger.ErrDisabled {
							logger.Log.Error().Err(err).Uint("id", uid).Str("typ", logger.StrSeries).Msg("Search Failed")
						}
					} else {
						if results == nil || len(results.Accepted) == 0 {
							results.Close()
						} else {
							results.Download(logger.StrSeries, "serie_"+config.SettingsMedia[idxp].Name)
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
	serieepi, _ := database.GetSerieEpisodes(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serieepi)

	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieepi.SerieID)
	//defer logger.ClearVar(&serie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}

		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				searchresults, err := searcher.SeriesSearch("serie_"+config.SettingsMedia[idxp].Name, serieepi.ID, false, titlesearch)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					//searchnow.Close()
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
				searchresults.Close()
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

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}

		if strings.EqualFold(config.SettingsMedia[idxp].Name, c.Param("group")) {
			templatequality := config.SettingsMedia[idxp].TemplateQuality
			searchresults, err := searcher.SearchRSS("serie_"+config.SettingsMedia[idxp].Name, templatequality, logger.StrSeries, true)
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
	serieepi, _ := database.GetSerieEpisodes(database.Querywithargs{Select: "id, serie_id, missing", Where: logger.FilterByID}, c.Param(logger.StrID))
	//defer logger.ClearVar(&serieepi)
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieepi.SerieID)
	//defer logger.ClearVar(&serie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	//defer logger.ClearVar(&nzb)

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range config.SettingsMedia[idxp].Lists {
			if strings.EqualFold(config.SettingsMedia[idxp].Lists[idxlist].Name, serie.Listname) {
				downloader.DownloadSeriesEpisode("serie_"+config.SettingsMedia[idxp].Name, serieepi.ID, &nzb)
				c.JSON(http.StatusOK, "started")
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
	utils.SingleJobs(logger.StrSeries, logger.StrClearHistory, "serie_"+c.Param("name"), "", true)
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
	inres, inerr := database.DeleteRow(false, "serie_episode_histories", "serie_episode_id = ?", c.Param(logger.StrID))

	if inerr == nil {
		c.JSON(http.StatusOK, inres)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
