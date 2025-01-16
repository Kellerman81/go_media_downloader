// series
package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.Use(checkauth)
	{
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
}

const querybydbserieid = "dbserie_id = ?"

// @Summary      List Series
// @Description  List Series
// @Tags         series
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.Dbserie}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series [get].
func apiSeriesGet(ctx *gin.Context) {
	var query database.Querywithargs
	rows := database.GetdatarowN(false, "select count() from dbseries")
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/{id} [delete].
func apiSeriesDelete(ctx *gin.Context) {
	database.DeleteRow("serie_episode_files", querybydbserieid, ctx.Param("id"))
	database.DeleteRow("serie_episode_histories", querybydbserieid, ctx.Param("id"))
	database.DeleteRow("serie_episodes", querybydbserieid, ctx.Param("id"))
	database.DeleteRow("dbserie_episodes", querybydbserieid, ctx.Param("id"))
	database.DeleteRow(logger.StrSeries, querybydbserieid, ctx.Param("id"))
	_, err := database.DeleteRow("dbseries", logger.FilterByID, ctx.Param("id"))

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
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.ResultSeries}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/list/{name} [get].
func apiSeriesListGet(ctx *gin.Context) {
	var query database.Querywithargs
	list := ctx.Param("name")
	query.InnerJoin = "dbseries on series.dbserie_id=dbseries.id"
	query.Where = "series.listname = ? COLLATE NOCASE"
	rows := database.GetdatarowN(false, "select count() from series where listname = ?", &list)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
}

// @Summary      Delete Series (List)
// @Description  Delete Series
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/list/{id} [delete].
func apiSeriesListDelete(ctx *gin.Context) {
	database.DeleteRow("serie_episode_files", querybydbserieid, ctx.Param("id"))
	database.DeleteRow("serie_episode_histories", querybydbserieid, ctx.Param("id"))
	database.DeleteRow("serie_episodes", querybydbserieid, ctx.Param("id"))
	_, err := database.DeleteRow(logger.StrSeries, querybydbserieid, ctx.Param("id"))

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
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.SerieFileUnmatched}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/unmatched [get].
func apiSeriesUnmatched(ctx *gin.Context) {
	var query database.Querywithargs
	rows := database.GetdatarowN(false, "select count() from serie_file_unmatcheds")
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.DbserieEpisode}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/episodes [get].
func apiSeriesEpisodesGet(ctx *gin.Context) {
	var query database.Querywithargs
	rows := database.GetdatarowN(false, "select count() from dbserie_episodes")
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.DbserieEpisode}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/episodes/{id} [get].
func apiSeriesEpisodesGetSingle(ctx *gin.Context) {
	var query database.Querywithargs
	query.Where = querybydbserieid
	dbid := ctx.Param("id")
	rows := database.GetdatarowN(false, "select count() from series where dbserie_id = ?", &dbid)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/episodes/{id} [delete].
func apiSeriesEpisodesDelete(ctx *gin.Context) {
	database.DeleteRow("serie_episode_files", "dbserie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow("serie_episode_histories", "dbserie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow("serie_episodes", "dbserie_episode_id = ?", ctx.Param("id"))
	_, err := database.DeleteRow("dbserie_episodes", logger.FilterByID, ctx.Param("id"))

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
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.ResultSerieEpisodes}
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/episodes/list/{id} [get].
func apiSeriesEpisodesListGet(ctx *gin.Context) {
	var query database.Querywithargs
	dbid := ctx.Param("id")
	query.InnerJoin = "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id"
	query.Where = "series.id = ?"
	rows := database.GetdatarowN(false, "select count() from serie_episodes where serie_id = ?", &dbid)
	limit := 0
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/episodes/list/{id} [delete].
func apiSeriesEpisodesListDelete(ctx *gin.Context) {
	database.DeleteRow("serie_episode_files", "serie_episode_id = ?", ctx.Param("id"))
	database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", ctx.Param("id"))
	_, err := database.DeleteRow("serie_episodes", logger.FilterByID, ctx.Param("id"))

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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns job name started"
// @Failure      204  {object}  string "error message"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/job/{job} [get].
func apiseriesAllJobs(c *gin.Context) {
	allowed := false
	for _, allow := range strings.Split(allowedjobsseriesstr, ",") {
		if strings.EqualFold(allow, c.Param(strJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(strJobLower) + " started"

		var cfgp *config.MediaTypeConfig
		// defer cfgSerie.Close()
		// defer cfg_list.Close()
	contloop:
		for _, media := range config.SettingsMedia {
			if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
				continue
			}
			cfgp = media
			cfgpstr := "serie_" + media.Name

			switch c.Param(strJobLower) {
			case logger.StrData, logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
				worker.Dispatch(c.Param(strJobLower)+"_series_"+media.Name, func(key uint32) {
					utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
				}, "Data")
			case logger.StrRss, logger.StrRssSeasons, logger.StrRssSeasonsAll, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
				worker.Dispatch(c.Param(strJobLower)+"_series_"+media.Name, func(key uint32) {
					utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
				}, "Search")
			case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:
				for idxlist := range media.Lists {
					if !media.Lists[idxlist].Enabled {
						continue
					}
					if media.Lists[idxlist].CfgList == nil {
						continue
					}

					if !config.SettingsList[media.Lists[idxlist].TemplateList].Enabled {
						continue
					}
					listname := media.Lists[idxlist].Name
					if c.Param(strJobLower) == logger.StrFeeds {
						worker.Dispatch(c.Param(strJobLower)+"_series_"+media.Name, func(key uint32) {
							utils.SingleJobs(c.Param(strJobLower), cfgpstr, listname, true, key)
						}, "Feeds")
					} else if c.Param(strJobLower) == logger.StrCheckMissing || c.Param(strJobLower) == logger.StrCheckMissingFlag || c.Param(strJobLower) == logger.StrReachedFlag {
						worker.Dispatch(c.Param(strJobLower)+"_series_"+media.Name, func(key uint32) {
							utils.SingleJobs(c.Param(strJobLower), cfgpstr, listname, true, key)
						}, "Data")
					}
					// cfg_list.Close()
				}
			case "refresh":
			case "refreshinc":
			case "":
				continue contloop

			default:
				worker.Dispatch(c.Param(strJobLower)+"_series_"+media.Name, func(key uint32) {
					utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
				}, "Data")
			}
			// cfgSerie.Close()
		}
		switch c.Param(strJobLower) {
		case "refresh":
			worker.Dispatch(logger.StrRefreshSeries, func(key uint32) {
				utils.SingleJobs("refresh", cfgp.NamePrefix, "", false, key)
			}, "Feeds")
		case "refreshinc":
			worker.Dispatch(logger.StrRefreshSeriesInc, func(key uint32) {
				utils.SingleJobs("refreshinc", cfgp.NamePrefix, "", false, key)
			}, "Feeds")
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(strJobLower) + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Start Jobs
// @Description  Starts a Job
// @Tags         series
// @Param        job   path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "List Name: ex. list"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns job name started"
// @Failure      204  {object}  string "error message"
// @Failure      401   {object}  Jsonerror
// @Router       /api/series/job/{job}/{name} [get].
func apiseriesJobs(c *gin.Context) {
	allowed := false
	for _, allow := range strings.Split(allowedjobsseriesstr, ",") {
		if strings.EqualFold(allow, c.Param(strJobLower)) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param(strJobLower) + " started"
		cfgpstr := "serie_" + c.Param("name")
		switch c.Param(strJobLower) {
		case logger.StrData, logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
			worker.Dispatch(c.Param(strJobLower)+"_series_"+c.Param("name"), func(key uint32) {
				utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
			}, "Data")
		case logger.StrRss, logger.StrRssSeasons, logger.StrRssSeasonsAll, logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
			worker.Dispatch(c.Param(strJobLower)+"_series_"+c.Param("name"), func(key uint32) {
				utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
			}, "Search")
		case logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrReachedFlag:
			var cfglists config.MediaListsConfig
			for _, cfglists = range config.SettingsMedia[c.Param("name")].Lists {
				if !cfglists.Enabled {
					continue
				}
				if cfglists.CfgList == nil {
					continue
				}

				if cfglists.CfgList.Enabled {
					continue
				}
				listname := cfglists.Name
				if c.Param(strJobLower) == logger.StrFeeds {
					worker.Dispatch(c.Param(strJobLower)+"_series_"+c.Param("name"), func(key uint32) {
						utils.SingleJobs(c.Param(strJobLower), cfgpstr, listname, true, key)
					}, "Feeds")
				}
				if c.Param(strJobLower) == logger.StrCheckMissing || c.Param(strJobLower) == logger.StrCheckMissingFlag || c.Param(strJobLower) == logger.StrReachedFlag {
					worker.Dispatch(c.Param(strJobLower)+"_series_"+c.Param("name"), func(key uint32) {
						utils.SingleJobs(c.Param(strJobLower), cfgpstr, listname, true, key)
					}, "Data")
				}
				// cfg_list.Close()
			}
		case "refresh":
			worker.Dispatch(logger.StrRefreshSeries, func(key uint32) {
				utils.SingleJobs("refresh", cfgpstr, "", false, key)
			}, "Feeds")
		case "refreshinc":
			worker.Dispatch(logger.StrRefreshSeriesInc, func(key uint32) {
				utils.SingleJobs("refreshinc", cfgpstr, "", false, key)
			}, "Feeds")
		case "":
			break
		default:
			worker.Dispatch(c.Param(strJobLower)+"_series_"+c.Param("name"), func(key uint32) {
				utils.SingleJobs(c.Param(strJobLower), cfgpstr, "", true, key)
			}, "Data")
		}
		c.JSON(http.StatusOK, returnval)
	} else {
		returnval := "Job " + c.Param(strJobLower) + " not allowed!"
		c.JSON(http.StatusNoContent, returnval)
	}
}

// @Summary      Update Series (Global)
// @Description  Updates or creates a series
// @Tags         series
// @Param        series  body      database.Dbserie  true  "Series"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  int64
// @Failure      403  {object}  error "error message"
// @Failure      400     {object}  Jsonerror
// @Failure      401  {object}  Jsonerror
// @Router       /api/series [post].
func updateDBSeries(c *gin.Context) {
	var dbserie database.Dbserie
	if err := c.BindJSON(&dbserie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.GetdatarowN(false, "select count() from dbseries where id != 0 and id = ?", &dbserie.ID)
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
		var rows int64
		if counter == 0 {
			rows, _ = inres.LastInsertId()
		} else {
			rows, _ = inres.RowsAffected()
		}
		c.JSON(http.StatusOK, rows)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series Episodes (Global)
// @Description  Updates or creates a episode
// @Tags         series
// @Param        episode  body      database.DbserieEpisode  true  "Episode"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  int64
// @Failure      403      {object}  error
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/series/episodes [post].
func updateDBEpisode(c *gin.Context) {
	var dbserieepisode database.DbserieEpisode
	if err := c.BindJSON(&dbserieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.GetdatarowN(false, "select count() from dbserie_episodes where id != 0 and id = ?", &dbserieepisode.ID)
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
		var rows int64
		if counter == 0 {
			rows, _ = inres.LastInsertId()
		} else {
			rows, _ = inres.RowsAffected()
		}
		c.JSON(http.StatusOK, rows)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series (List)
// @Description  Updates or creates a series
// @Tags         series
// @Param        series  body      database.Serie  true  "Series"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  int64
// @Failure      403     {object}  error "error message"
// @Failure      400     {object}  Jsonerror
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/list [post].
func updateSeries(c *gin.Context) {
	var serie database.Serie
	if err := c.ShouldBindJSON(&serie); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.GetdatarowN(false, "select count() from series where id != 0 and id = ?", &serie.ID)
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
		var rows int64
		if counter == 0 {
			rows, _ = inres.LastInsertId()
		} else {
			rows, _ = inres.RowsAffected()
		}
		c.JSON(http.StatusOK, rows)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Update Series Episodes (List)
// @Description  Updates or creates a episode
// @Tags         series
// @Param        episode  body      database.SerieEpisode  true  "Episode"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  int64
// @Failure      403      {object}  error
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/series/episodes/list [post].
func updateEpisode(c *gin.Context) {
	var serieepisode database.SerieEpisode
	if err := c.ShouldBindJSON(&serieepisode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.GetdatarowN(false, "select count() from serie_episodes where id != 0 and id = ?", &serieepisode.ID)
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
		var rows int64
		if counter == 0 {
			rows, _ = inres.LastInsertId()
		} else {
			rows, _ = inres.RowsAffected()
		}
		c.JSON(http.StatusOK, rows)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}

// @Summary      Refresh Single Series
// @Description  Refreshes Series Metadata
// @Tags         series
// @Param        id   path      int  true  "Serie ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/refresh/{id} [get].
func apirefreshSerie(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	for _, media := range config.SettingsMedia {
		if media.NamePrefix[:5] != logger.StrMovie {
			cfgp = media
			break
		}
	}
	id := c.Param(logger.StrID)
	worker.Dispatch("Refresh Single Serie", func(uint32) {
		utils.RefreshSerie(cfgp, &id)
	}, "Feeds")
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Series
// @Description  Refreshes Series Metadata
// @Tags         series
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/all/refreshall [get].
func apirefreshSeries(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	for _, media := range config.SettingsMedia {
		if media.NamePrefix[:5] != logger.StrMovie {
			cfgp = media
			break
		}
	}
	worker.Dispatch(logger.StrRefreshSeries, func(key uint32) {
		utils.SingleJobs("refresh", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	c.JSON(http.StatusOK, "started")
}

// @Summary      Refresh Series Incremental
// @Description  Refreshes Series Metadata
// @Tags         series
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/all/refresh [get].
func apirefreshSeriesInc(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	for _, media := range config.SettingsMedia {
		if media.NamePrefix[:5] != logger.StrMovie {
			cfgp = media
			break
		}
	}
	worker.Dispatch(logger.StrRefreshSeriesInc, func(key uint32) {
		utils.SingleJobs("refreshinc", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	c.JSON(http.StatusOK, "started")
}

// @Summary      Search a series (all seasons)
// @Description  Searches for upgrades and missing
// @Tags         series
// @Param        id   path      int  true  "Series ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Success      204  {object}  string "returns nothing done"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/search/id/{id} [get].
func apiSeriesSearch(c *gin.Context) {
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serie)

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch("searchseries_series_"+media.Name+"_"+strconv.Itoa(int(serie.ID)), func(uint32) {
					episodes := database.GetrowsN[uint](false, database.GetdatarowN(false, "select count() from serie_episodes where serie_id = ?", &serie.ID), "select id from serie_episodes where serie_id = ?", &serie.ID)
					for idxepisode := range episodes {
						ctx := context.Background()
						results := searcher.NewSearcher(media, nil, "", nil)
						err := results.MediaSearch(ctx, media, episodes[idxepisode], true, false, false)

						if err != nil {
							if !errors.Is(err, logger.ErrDisabled) {
								logger.LogDynamicany("error", "Search Failed", &logger.StrID, &episodes[idxepisode], "typ", &logger.StrSeries, err)
							}
						} else {
							if results == nil || len(results.Accepted) == 0 {
							} else {
								results.Download()
							}
						}
						results.Close()
					}
				}, "Search")
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Success      204  {object}  string "returns nothing done"
// @Failure      401    {object}  Jsonerror
// @Router       /api/series/search/id/{id}/{season} [get].
func apiSeriesSearchSeason(c *gin.Context) {
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serie)

	var episodes []uint
	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch("searchseriesseason_series_"+media.Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func(uint32) {
					a := c.Param("season")
					episodes = database.GetrowsN[uint](false, database.GetdatarowN(false, "select count() from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", &serie.ID, &a), "select id from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", serie.ID, c.Param("season"))
					for idxepisode := range episodes {
						ctx := context.Background()
						results := searcher.NewSearcher(media, nil, "", nil)
						err := results.MediaSearch(ctx, media, episodes[idxepisode], true, false, false)

						if err != nil {
							if !errors.Is(err, logger.ErrDisabled) {
								logger.LogDynamicany("error", "Search Failed", &logger.StrID, &episodes[idxepisode], "typ", &logger.StrSeries, err)
							}
						} else {
							if results == nil || len(results.Accepted) == 0 {
							} else {
								results.Download()
							}
						}
						results.Close()
					}
					episodes = nil
				}, "Search")
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Success      204  {object}  string "returns nothing done"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/searchrss/id/{id} [get].
func apiSeriesSearchRSS(c *gin.Context) {
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serie)

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch("searchseriesseason_series_"+media.Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func(uint32) {
					s, _ := searcher.SearchSerieRSSSeasonSingle(&serie.ID, "", false, media)
					s.Close()
				}, "Search")
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsonresults
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/searchrss/list/id/{id} [get].
func apiSeriesSearchRSSList(c *gin.Context) {
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serie)

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				searchresults, _ := searcher.SearchSerieRSSSeasonSingle(&serie.ID, "", false, media)
				defer searchresults.Close()
				if len(searchresults.Raw.Arr) == 0 {
					c.JSON(http.StatusNoContent, "Failed")
					return
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
				// searchresults.Close()
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
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns started"
// @Success      204   {object}  string "returns nothing done"
// @Failure      401   {object}  Jsonerror
// @Router       /api/series/searchrss/id/{id}/{season} [get].
func apiSeriesSearchRSSSeason(c *gin.Context) {
	serie, _ := database.GetSeries(database.Querywithargs{Select: "id, listname", Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serie)
	season := c.Param("season")
	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch("searchseriesseason_series_"+media.Name+"_"+strconv.Itoa(int(serie.ID))+"_"+c.Param("season"), func(uint32) {
					s, _ := searcher.SearchSerieRSSSeasonSingle(&serie.ID, season, true, media)
					s.Close()
				}, "Search")
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Success      204  {object}  string "returns nothing done"
// @Failure      401  {object}  Jsonerror
// @Router       /api/series/episodes/search/id/{id} [get].
func apiSeriesEpisodeSearch(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param(logger.StrID))
	uid := uint(id)
	dbid := c.Param(logger.StrID)
	serieid := database.GetdatarowN(false, database.QuerySerieEpisodesGetSerieIDByID, &dbid)
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieid)
	// defer logger.ClearVar(&serie)

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				worker.Dispatch("searchseriesepisode_series_"+media.Name+"_"+strconv.Itoa(id), func(uint32) {
					ctx := context.Background()
					results := searcher.NewSearcher(media, nil, "", nil)
					err := results.MediaSearch(ctx, media, uid, true, false, false)

					if err != nil {
						if !errors.Is(err, logger.ErrDisabled) {
							logger.LogDynamicany("error", "Search Failed", &logger.StrID, &uid, "typ", &logger.StrSeries, err)
						}
					} else {
						if results == nil || len(results.Accepted) == 0 {
						} else {
							results.Download()
						}
					}
					results.Close()
				}, "Search")
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
// @Param        download       query     bool    false  "download"
// @Param        apikey query     string    true  "apikey"
// @Success      200            {object}  Jsonresults
// @Failure      401            {object}  string
// @Router       /api/series/episodes/search/list/{id} [get].
func apiSeriesEpisodeSearchList(c *gin.Context) {
	serieepi, _ := database.GetSerieEpisodes(database.Querywithargs{Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serieepi)

	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieepi.SerieID)
	// defer logger.ClearVar(&serie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				ctx := context.Background()
				searchresults := searcher.NewSearcher(media, nil, "", nil)
				err := searchresults.MediaSearch(ctx, media, serieepi.ID, titlesearch, false, false)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					searchresults.Close()
					return
				}
				if _, ok := c.GetQuery("download"); ok {
					searchresults.Download()
				}
				c.JSON(http.StatusOK, gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied})
				searchresults.Close()
				// searchnow.Close()
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
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  Jsonresults
// @Failure      401     {object}  Jsonerror
// @Router       /api/series/rss/search/list/{group} [get].
func apiSeriesRssSearchList(c *gin.Context) {
	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}

		if strings.EqualFold(media.Name, c.Param("group")) {
			templatequality := media.TemplateQuality
			ctx := context.Background()
			searchresults := searcher.NewSearcher(media, media.CfgQuality, logger.StrRss, nil)
			err := searchresults.SearchRSS(ctx, media, config.SettingsQuality[templatequality], false, false)
			if err != nil {
				str := "failed with " + err.Error()
				c.JSON(http.StatusNotFound, str)
				searchresults.Close()
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
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  string "returns started"
// @Success      204     {object}  string "returns nothing done"
// @Failure      400     {object}  Jsonerror
// @Router       /api/series/episodes/search/download/{id} [post].
func apiSeriesEpisodeSearchDownload(c *gin.Context) {
	serieepi, _ := database.GetSerieEpisodes(database.Querywithargs{Select: "id, serie_id, missing", Where: logger.FilterByID}, c.Param(logger.StrID))
	// defer logger.ClearVar(&serieepi)
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieepi.SerieID)
	// defer logger.ClearVar(&serie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// defer logger.ClearVar(&nzb)

	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			continue
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				downloader.DownloadSeriesEpisode(media, &nzb)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  string "returns started"
// @Failure      401     {object}  Jsonerror
// @Router       /api/series/search/history/clear/{name} [get].
func apiSeriesClearHistoryName(c *gin.Context) {
	utils.SingleJobs(logger.StrClearHistory, "serie_"+c.Param("name"), "", true, 0)
	c.JSON(http.StatusOK, "started")
}

// @Summary      Clear History (Single Item)
// @Description  Clear Episode Download History
// @Tags         series
// @Param        id   path      string  true  "Episode ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  int64
// @Failure      403     {object}  error
// @Failure      401     {object}  Jsonerror
// @Router       /api/series/search/history/clearid/{id} [get].
func apiSeriesClearHistoryID(c *gin.Context) {
	inres, inerr := database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", c.Param(logger.StrID))

	if inerr == nil {
		rows, _ := inres.RowsAffected()
		c.JSON(http.StatusOK, rows)
	} else {
		c.JSON(http.StatusForbidden, inerr)
	}
}
