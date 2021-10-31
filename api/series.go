// series
package api

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/gin-gonic/gin"
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
		database.DeleteRow("dbseries", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	routerseries.GET("/list/:name", func(ctx *gin.Context) {
		movies, _ := database.QueryResultSeries(database.Query{InnerJoin: "Dbseries on series.dbserie_id=dbseries.id", Where: "series.listname=?", WhereArgs: []interface{}{ctx.Param("name")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})
	routerseries.POST("/list/", updateSeries)
	routerseries.DELETE("/list/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episodes", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("series", database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	routerseries.GET("/all/refresh", apirefreshSeriesInc)
	routerseries.GET("/all/refreshall", apirefreshSeries)
	routerseries.GET("/refresh/:id", apirefreshSerie)

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
		database.DeleteRow("dbserie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})
	routerseries.POST("/episodes/list/", updateEpisode)
	routerseries.GET("/episodes/list/:id", func(ctx *gin.Context) {
		movies, _ := database.QueryResultSerieEpisodes(database.Query{InnerJoin: "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id", Where: "series.id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": movies, "rows": len(movies)})
	})
	routerseries.DELETE("/episodes/list/:id", func(ctx *gin.Context) {
		database.DeleteRow("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		database.DeleteRow("serie_episodes", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
		ctx.JSON(http.StatusOK, gin.H{"data": "ok"})
	})

	routerseriessearch := routerseries.Group("/search")
	{
		routerseriessearch.GET("/id/:id", apiSeriesSearch)
		routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
	}

	routerseriesepisodessearch := routerseries.Group("/episodes/search")
	{
		routerseriesepisodessearch.GET("/id/:id", apiSeriesEpisodeSearch)
	}
}

var allowedjobsseries []string = []string{"rss", "data", "datafull", "checkmissing", "checkmissingflag", "structure", "searchmissingfull",
	"searchmissinginc", "searchupgradefull", "searchupgradeinc", "searchmissingfulltitle",
	"searchmissinginctitle", "searchupgradefulltitle", "searchupgradeinctitle", "clearhistory", "feeds"}

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
		go utils.Series_all_jobs(c.Param("job"), true)
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	} else {
		returnval := "Job " + c.Param("job") + " not allowed!"
		c.JSON(http.StatusOK, gin.H{"data": returnval})
	}
}
func apiseriesJobs(c *gin.Context) {
	allowed := false
	for idxallow := range allowedjobsseries {
		if strings.EqualFold(allowedjobsseries[idxallow], c.Param("job")) {
			allowed = true
			break
		}
	}
	if allowed {
		returnval := "Job " + c.Param("job") + " started"
		go utils.Series_single_jobs(c.Param("job"), c.Param("name"), "", true)
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

func apirefreshSerie(c *gin.Context) {
	go utils.RefreshSerie(c.Param("id"))
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apirefreshSeries(c *gin.Context) {
	go utils.RefreshSeries()
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apirefreshSeriesInc(c *gin.Context) {
	go utils.RefreshSeriesInc()
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}

func apiSeriesSearch(c *gin.Context) {
	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		for idxlist := range cfg_serie.Lists {
			if strings.EqualFold(cfg_serie.Lists[idxlist].Name, serie.Listname) {
				go utils.SearchSerieSingle(serie, cfg_serie, true)
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}
func apiSeriesEpisodeSearch(c *gin.Context) {
	serieepi, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{c.Param("id")}})

	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepi.SerieID}})

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		for idxlist := range cfg_serie.Lists {
			if strings.EqualFold(cfg_serie.Lists[idxlist].Name, serie.Listname) {
				go utils.SearchSerieEpisodeSingle(serieepi, cfg_serie, true)
				c.JSON(http.StatusOK, gin.H{"data": "started"})
				return
			}
		}
	}
}
func apiSeriesClearHistoryName(c *gin.Context) {
	go utils.Series_single_jobs("clearhistory", c.Param("name"), "", true)
	c.JSON(http.StatusOK, gin.H{"data": "started"})
}
