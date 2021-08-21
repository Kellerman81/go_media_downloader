// series
package api

import (
	"database/sql"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/gin-gonic/gin"
	"github.com/remeh/sizedwaitgroup"
)

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.GET("/", func(ctx *gin.Context) {
		series, _ := database.QueryDbserie(database.Query{})
		ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
	})
	routerseries.POST("/", updateDBSeries)
	routerseries.DELETE("/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("dbserie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		result, _ := database.DeleteRow("dbseries", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})

	routerseries.GET("/list/:name", func(ctx *gin.Context) {
		movies, _ := database.QueryResultSeries(database.Query{InnerJoin: "Dbseries on series.dbserie_id=dbseries.id", Where: "series.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
		// var counter int64
		// database.DB.Table("series").Joins("inner join Dbseries on series.dbserie_id=dbseries.id").Where("series.listname=?", ctx.Param("name")).Count(&counter)
		// series := make([]database.ResultSeries, 0, counter)
		// database.DB.Table("series").Select("dbseries.*, series.listname, series.rootpath, series.id AS SerieID").Joins("inner join Dbseries on series.dbserie_id=dbseries.id").Where("series.listname=?", ctx.Param("name")).Find(&series)
		// ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": counter})
	})
	routerseries.POST("/list/", updateSeries)
	routerseries.DELETE("/list/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		result, _ := database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})

	routerseries.GET("/all/refresh", apirefreshSeriesInc)
	routerseries.GET("/all/refreshall", apirefreshSeries)

	routerseries.GET("/job/:job", apiseriesAllJobs)
	routerseries.GET("/job/:job/:name", apiseriesJobs)

	routerseries.GET("/unmatched", func(ctx *gin.Context) {
		movies, _ := database.QuerySerieFileUnmatched(database.Query{})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})

	routerseries.GET("/episodes", func(ctx *gin.Context) {
		series, _ := database.QueryDbserieEpisodes(database.Query{})
		ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
	})
	routerseries.POST("/episodes", updateDBEpisode)

	routerseries.GET("/episodes/:id", func(ctx *gin.Context) {
		series, _ := database.QueryDbserieEpisodes(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": series, "rows": len(series)})
	})
	routerseries.DELETE("/episodes/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		result, _ := database.DeleteRow("dbserie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})
	routerseries.POST("/episodes/list/", updateEpisode)
	routerseries.GET("/episodes/list/:id", func(ctx *gin.Context) {
		movies, _ := database.QueryResultSerieEpisodes(database.Query{InnerJoin: "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id", Where: "series.id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})

		// var counter int64
		// database.DB.Table("serie_episodes").Joins("inner join dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id").Joins("inner join series on series.id=serie_episodes.serie_id").Where("series.id=?", ctx.Param("id")).Count(&counter)
		// seriesepi := make([]database.ResultSerieEpisodes, 0, counter)
		// database.DB.Table("serie_episodes").Select("dbserie_episodes.*, serie_episodes.lastscan, serie_episodes.blacklisted, serie_episodes.quality_reached, serie_episodes.quality_profile, serie_episodes.missing, series.listname, serie_episodes.id AS SerieEpisodeID").Joins("inner join dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id").Joins("inner join series on series.id=serie_episodes.serie_id").Where("series.id=?", ctx.Param("id")).Find(&seriesepi)
		// ctx.JSON(http.StatusOK, gin.H{"data": seriesepi, "rows": counter})
	})
	routerseries.DELETE("/episodes/list/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		result, _ := database.DeleteRow("serie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": result.RowsAffected})
	})

	routerseriessearch := routerseries.Group("/search")
	{
		routerseriessearch.GET("/id/:id", apiSeriesSearch)
		routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
	}

	routerseriesepisodessearch := routerseries.Group("/episodes/search")
	{
		routerseriesepisodessearch.GET("/series/episodes/search/id/:id", apiSeriesEpisodeSearch)
	}
}

var allowedjobsseries []string = []string{"rss", "data", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "clearhistory", "feeds"}

func apiseriesAllJobs(c *gin.Context) {
	allowed := false
	for idxallow := range allowedjobsseries {
		if strings.EqualFold(allowedjobsseries[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		go Series_all_jobs(c.Param("job"), true)
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}
func apiseriesJobs(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Imdb", "List"}, config.Configfile)
	allowed := false
	for idxallow := range allowedjobsseries {
		if strings.EqualFold(allowedjobsseries[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		go Series_single_jobs(cfg, c.Param("job"), c.Param("name"), "", true)
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}

func updateDBSeries(c *gin.Context) {
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
func updateDBEpisode(c *gin.Context) {
	var dbserieepisode database.DbserieEpisode
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
func updateSeries(c *gin.Context) {
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
func updateEpisode(c *gin.Context) {
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

func apirefreshSeries(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Downloader", "List"}, config.Configfile)
	go RefreshSeries(cfg)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apirefreshSeriesInc(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Downloader", "List"}, config.Configfile)
	go RefreshSeriesInc(cfg)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apiSeriesSearch(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Downloader", "List"}, config.Configfile)

	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	for idxserie := range cfg.Serie {
		for idxlist := range cfg.Serie[idxserie].Lists {
			if strings.EqualFold(cfg.Serie[idxserie].Lists[idxlist].Name, serie.Listname) {
				go utils.SearchSerieSingle(cfg, serie, cfg.Serie[idxserie], true)
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}
func apiSeriesEpisodeSearch(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Downloader", "List"}, config.Configfile)

	serieepi, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepi.SerieID}})

	for idxserie := range cfg.Serie {
		for idxlist := range cfg.Serie[idxserie].Lists {
			if strings.EqualFold(cfg.Serie[idxserie].Lists[idxlist].Name, serie.Listname) {
				go utils.SearchSerieEpisodeSingle(cfg, serieepi, cfg.Serie[idxserie], true)
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}
func apiSeriesClearHistoryName(c *gin.Context) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Imdb", "List"}, config.Configfile)

	go Series_single_jobs(cfg, "clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func RefreshSeries(cfg config.Cfg) {
	if cfg.General.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg.General.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{})
	for idxserie := range dbseries {
		sw.Add()
		utils.JobReloadDbSeries(cfg, dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func RefreshSeriesInc(cfg config.Cfg) {
	if cfg.General.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg.General.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 100})

	for idxserie := range dbseries {
		sw.Add()
		utils.JobReloadDbSeries(cfg, dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func Series_all_jobs(job string, force bool) {
	cfg, _, _ := config.LoadCfg([]string{"Serie", "General", "Regex", "Quality", "Path", "Indexer", "Imdb", "Downloader", "Notification", "List"}, config.Configfile)

	logger.Log.Info("Started Job: ", job, " for all")
	for idxserie := range cfg.Serie {
		Series_single_jobs(cfg, job, cfg.Serie[idxserie].Name, "", force)
	}
}
func Series_all_jobs_cfg(cfg config.Cfg, job string, force bool) {
	logger.Log.Info("Started Job: ", job, " for all")
	for idxserie := range cfg.Serie {
		Series_single_jobs(cfg, job, cfg.Serie[idxserie].Name, "", force)
	}
}

var SerieJobRunning map[string]bool

func Series_single_jobs(cfg config.Cfg, job string, typename string, listname string, force bool) {
	if cfg.General.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", typename)
		return
	}
	jobName := job + typename + listname
	defer func() {
		database.ReadWriteMu.Lock()
		delete(SerieJobRunning, jobName)
		database.ReadWriteMu.Unlock()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := SerieJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		SerieJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()
	logger.Log.Info("Started Job: ", job, " for ", typename)

	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, typename, "Serie", time.Now()})
	_, ok := cfg.Serie[typename]
	if ok {
		switch job {
		case "searchmissingfull":
			utils.SearchSerieMissing(cfg, cfg.Serie[typename], 0, false)
		case "searchmissinginc":
			utils.SearchSerieMissing(cfg, cfg.Serie[typename], cfg.Serie[typename].Searchmissing_incremental, false)
		case "searchupgradefull":
			utils.SearchSerieUpgrade(cfg, cfg.Serie[typename], 0, false)
		case "searchupgradeinc":
			utils.SearchSerieUpgrade(cfg, cfg.Serie[typename], cfg.Serie[typename].Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			utils.SearchSerieMissing(cfg, cfg.Serie[typename], 0, true)
		case "searchmissinginctitle":
			utils.SearchSerieMissing(cfg, cfg.Serie[typename], cfg.Serie[typename].Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			utils.SearchSerieUpgrade(cfg, cfg.Serie[typename], 0, true)
		case "searchupgradeinctitle":
			utils.SearchSerieUpgrade(cfg, cfg.Serie[typename], cfg.Serie[typename].Searchupgrade_incremental, true)

		}
		qualis := make(map[string]bool, 10)
		for idxlist := range cfg.Serie[typename].Lists {
			if cfg.Serie[typename].Lists[idxlist].Name != listname && listname != "" {
				continue
			}
			if _, ok := qualis[cfg.Serie[typename].Lists[idxlist].Template_quality]; !ok {
				qualis[cfg.Serie[typename].Lists[idxlist].Template_quality] = true
			}
			switch job {
			case "data":
				config.Slepping(true, 6)
				getnewepisodessingle(cfg, cfg.Serie[typename], cfg.Serie[typename].Lists[idxlist])
			case "checkmissing":
				checkmissingepisodessingle(cfg, cfg.Serie[typename], cfg.Serie[typename].Lists[idxlist])
			case "checkmissingflag":
				checkmissingepisodesflag(cfg.Serie[typename], cfg.Serie[typename].Lists[idxlist])
			case "structure":
				seriesStructureSingle(cfg, cfg.Serie[typename], cfg.Serie[typename].Lists[idxlist])
			case "clearhistory":
				database.DeleteRow("serie_episode_histories", database.Query{InnerJoin: "serie_episodes ON serie_episodes.id = serie_episode_histories.serie_episode_id inner join series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{typename}})
			case "feeds":
				config.Slepping(true, 6)
				Importnewseriessingle(cfg, cfg.Serie[typename], cfg.Serie[typename].Lists[idxlist])
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				utils.SearchSerieRSS(cfg, cfg.Serie[typename], qual)
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", typename)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", job, " for ", typename)
	debug.FreeOSMemory()
}

var Lastseries string

func Importnewseriessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	results := utils.Feeds(cfg, row, list)

	logger.Log.Info("Get Serie Config", list.Name)
	logger.Log.Info("Workers: ", cfg.General.WorkerMetadata)
	swg := sizedwaitgroup.New(cfg.General.WorkerMetadata)
	for idxserie := range results.Series.Serie {
		if strings.EqualFold(Lastseries, results.Series.Serie[idxserie].Name) && Lastseries != "" {
			config.Slepping(false, 5)
		}
		Lastseries = results.Series.Serie[idxserie].Name
		logger.Log.Info("Import Serie ", idxserie, " of ", len(results.Series.Serie), " name: ", results.Series.Serie[idxserie].Name)
		swg.Add()
		utils.JobImportDbSeries(cfg, results.Series.Serie[idxserie], row, list, false, &swg)
	}
	swg.Wait()
}

var LastSeriesPath string
var LastSeriesFilePath string

func getnewepisodessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	defaultPrio := &utils.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
	defaultPrio.GetPriority(row, cfg.Quality[list.Template_quality])

	logger.Log.Info("Scan SerieEpisodeFile")
	filesfound := make([]string, 0, 5000)
	for idxpath := range row.Data {
		mappath := row.Data[idxpath].Template_path
		_, okmap := cfg.Path[mappath]
		if !okmap {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
		if strings.EqualFold(LastSeriesPath, cfg.Path[mappath].Path) && LastSeriesPath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastSeriesPath = cfg.Path[mappath].Path
		filesfound_add := scanner.GetFilesGoDir(cfg.Path[mappath].Path, cfg.Path[mappath].AllowedVideoExtensions, cfg.Path[mappath].AllowedVideoExtensionsNoRename, cfg.Path[mappath].Blocked)
		filesfound = append(filesfound, filesfound_add...)
	}
	filesadded := scanner.GetFilesSeriesAdded(filesfound, list.Name)
	logger.Log.Info("Find SerieEpisodeFile")
	logger.Log.Info("Workers: ", cfg.General.WorkerParse)
	swf := sizedwaitgroup.New(cfg.General.WorkerParse)
	for idxfile := range filesadded {
		if strings.EqualFold(LastSeriesFilePath, filesadded[idxfile]) && LastSeriesFilePath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastSeriesFilePath = filesadded[idxfile]
		logger.Log.Info("Parse Serie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
		swf.Add()
		utils.JobImportSeriesParseV2(cfg, filesadded[idxfile], true, row, list, *defaultPrio, &swf)
	}
	swf.Wait()
}

func checkmissingepisodesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "serie_episodes.*", InnerJoin: " series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{list.Name}})
	for idxepi := range episodes {
		counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		if counter >= 1 {
			if episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		} else {
			if !episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		}
	}
}

func checkmissingepisodessingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	series, _ := database.QuerySeries(database.Query{Select: "id", Where: "listname=?", WhereArgs: []interface{}{list.Name}})

	swfile := sizedwaitgroup.New(cfg.General.WorkerFiles)
	for idx := range series {
		seriefile, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "location", Where: "Serie_id=?", WhereArgs: []interface{}{series[idx].ID}})

		for idxfile := range seriefile {
			swfile.Add()
			utils.JobImportFileCheck(seriefile[idxfile].Location, "serie", &swfile)
		}
	}
	swfile.Wait()
}

var LastSeriesStructure string

func seriesStructureSingle(cfg config.Cfg, row config.MediaTypeConfig, list config.MediaListsConfig) {
	swfile := sizedwaitgroup.New(cfg.General.WorkerFiles)

	for idxpath := range row.DataImport {
		mappathimport := row.DataImport[idxpath].Template_path
		mappath := ""
		_, okmap := cfg.Path[mappathimport]
		if !okmap {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
		if len(row.Data) >= 1 {
			mappath = row.Data[0].Template_path
			_, okmap := cfg.Path[mappath]
			if !okmap {
				logger.Log.Error("Name in PathsMap not found")
				return
			}
		}
		if strings.EqualFold(LastSeriesStructure, cfg.Path[mappathimport].Path) && LastSeriesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		LastSeriesStructure = cfg.Path[mappathimport].Path
		swfile.Add()

		utils.StructureFolders(cfg, "series", cfg.Path[mappathimport], cfg.Path[mappath], row, list)
		//utils.JobStructureSeries(cfg.Path[mappathimport], cfg.Path[mappath], row, list, &swfile)
		swfile.Done()
	}
	swfile.Wait()
}
