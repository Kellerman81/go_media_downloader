package api

import (
	"net/http"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
)

func AddAllRoutes(rg *gin.RouterGroup) {
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

// @Summary      Search all feeds
// @Description  Search all feeds of movies and series for new entries
// @Tags         feeds
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/feeds [get]
func apiAllGetFeeds(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}

	utils.MoviesAllJobs(logger.StrFeeds, true)
	utils.SeriesAllJobs(logger.StrFeeds, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all folders
// @Description  Search all folders of movies and series for new entries
// @Tags         data
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/data [get]
func apiAllGetData(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs("data", true)
	utils.SeriesAllJobs("data", true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all rss feeds
// @Description  Search all rss feeds of movies and series for new releases
// @Tags         search
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/search/rss [get]
func apiAllGetRss(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs(logger.StrRss, true)
	utils.SeriesAllJobs(logger.StrRss, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Missing
// @Description  Search all media of movies and series for missing releases
// @Tags         search
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/search/missing/full [get]
func apiAllGetMissingFull(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs(logger.StrSearchMissingFull, true)
	utils.SeriesAllJobs(logger.StrSearchMissingFull, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Missing Incremental
// @Description  Search all media of movies and series for missing releases (incremental)
// @Tags         search
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/search/missing/inc [get]
func apiAllGetMissingInc(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs(logger.StrSearchMissingInc, true)
	utils.SeriesAllJobs(logger.StrSearchMissingInc, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Upgrades
// @Description  Search all media of movies and series for upgrades
// @Tags         search
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/search/upgrade/full [get]
func apiAllGetUpgradeFull(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs(logger.StrSearchUpgradeFull, true)
	utils.SeriesAllJobs(logger.StrSearchUpgradeFull, true)
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Search all Upgrades Incremental
// @Description  Search all media of movies and series for upgrades (incremental)
// @Tags         search
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/all/search/upgrade/inc [get]
func apiAllGetUpgradeInc(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	utils.MoviesAllJobs(logger.StrSearchUpgradeInc, true)
	utils.SeriesAllJobs(logger.StrSearchUpgradeInc, true)
	c.JSON(http.StatusOK, "ok")
}

func auth(c *gin.Context) int {
	if queryParam, ok := c.GetQuery("apikey"); ok {
		if queryParam == "" {
			c.Header("Content-Type", "application/json")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.AbortWithStatus(http.StatusUnauthorized)
			return http.StatusUnauthorized
		}
		if queryParam != config.SettingsGeneral.WebAPIKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.AbortWithStatus(http.StatusUnauthorized)
			return http.StatusUnauthorized
		}
		c.Next()
		return http.StatusOK
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	c.AbortWithStatus(http.StatusUnauthorized)
	return http.StatusUnauthorized
}
