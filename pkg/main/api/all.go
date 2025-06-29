package api

import (
	"net/http"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	gin "github.com/gin-gonic/gin"
)

type Jsonerror struct {
	Error string `json:"error"`
}

type Jsondata struct {
	Data any `json:"data"`
}
type Jsondataerror struct {
	Error string `json:"error"`
	Data  any    `json:"data"`
}
type Jsondatarows struct {
	Total int `json:"total"`
	Data  any `json:"data"`
}
type JSONNaming struct {
	M          database.ParseInfo `json:"m"`
	Foldername string             `json:"foldername"`
	Filename   string             `json:"filename"`
}
type Jsonresults struct {
	Accepted []apiexternal.Nzbwithprio `json:"accepted"`
	Denied   []apiexternal.Nzbwithprio `json:"denied"`
}

func AddAllRoutes(rg *gin.RouterGroup) {
	rg.Use(checkauth)
	{
		rg.GET("/feeds", apiAllGetFeeds)
		rg.GET("/data", apiAllGetData)

		routerallsearch := rg.Group("/search")
		{
			routerallsearch.GET("/rss", apiAllGetRss)
			routerallsearch.GET("/missing/full", apiAllGetMissingFull)
			routerallsearch.GET("/missing/inc", apiAllGetMissingInc)
			routerallsearch.GET("/upgrade/full", apiAllGetUpgradeFull)
			routerallsearch.GET("/upgrade/inc", apiAllGetUpgradeInc)
		}
	}
}

// @Summary      Search all feeds
// @Description  Search all feeds of movies and series for new entries
// @Tags         feeds
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/feeds [get].
func apiAllGetFeeds(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrFeeds, true)
	utils.SeriesAllJobs(logger.StrFeeds, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all folders
// @Description  Search all folders of movies and series for new entries
// @Tags         data
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/data [get].
func apiAllGetData(c *gin.Context) {
	utils.MoviesAllJobs("data", true)
	utils.SeriesAllJobs("data", true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all rss feeds
// @Description  Search all rss feeds of movies and series for new releases
// @Tags         search
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/search/rss [get].
func apiAllGetRss(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrRss, true)
	utils.SeriesAllJobs(logger.StrRss, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Missing
// @Description  Search all media of movies and series for missing releases
// @Tags         search
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/search/missing/full [get].
func apiAllGetMissingFull(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrSearchMissingFull, true)
	utils.SeriesAllJobs(logger.StrSearchMissingFull, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Missing Incremental
// @Description  Search all media of movies and series for missing releases (incremental)
// @Tags         search
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/search/missing/inc [get].
func apiAllGetMissingInc(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrSearchMissingInc, true)
	utils.SeriesAllJobs(logger.StrSearchMissingInc, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Upgrades
// @Description  Search all media of movies and series for upgrades
// @Tags         search
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/search/upgrade/full [get].
func apiAllGetUpgradeFull(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrSearchUpgradeFull, true)
	utils.SeriesAllJobs(logger.StrSearchUpgradeFull, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Upgrades Incremental
// @Description  Search all media of movies and series for upgrades (incremental)
// @Tags         search
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/all/search/upgrade/inc [get].
func apiAllGetUpgradeInc(c *gin.Context) {
	utils.MoviesAllJobs(logger.StrSearchUpgradeInc, true)
	utils.SeriesAllJobs(logger.StrSearchUpgradeInc, true)
	c.JSON(http.StatusOK, "ok")
}

func checkauth(c *gin.Context) {
	var msg string
	if queryParam, ok := c.GetQuery("apikey"); ok {
		if queryParam == config.SettingsGeneral.WebAPIKey {
			c.Next()
			return
		}
		msg = "wrong apikey in query"
	} else {
		msg = "no apikey in query"
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - " + msg})
	c.AbortWithStatus(http.StatusUnauthorized)
}
