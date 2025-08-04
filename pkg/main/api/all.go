package api

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	gin "github.com/gin-gonic/gin"
)

// Redundant structs removed - using StandardResponse from common.go instead
type JSONNaming struct {
	M          database.ParseInfo `json:"m"`
	Foldername string             `json:"foldername"`
	Filename   string             `json:"filename"`
}
type Jsonresults struct {
	Accepted []apiexternal.Nzbwithprio `json:"accepted"`
	Denied   []apiexternal.Nzbwithprio `json:"denied"`
}

// AddAllRoutes sets up HTTP routes for the "all" API endpoints.
// It configures routes for feeds, data scanning, and search operations
// that apply to both movies and series. All routes require API key authentication.
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
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
	sendSuccess(c, StrOK)
}

// checkauth validates API key authentication for requests.
// It checks for the "apikey" query parameter and compares it against
// the configured WebAPIKey. Returns 401 Unauthorized if validation fails.
func checkauth(c *gin.Context) {
	apikey, ok := c.GetQuery(StrApikey)
	if !ok {
		sendUnauthorized(c, "unauthorized - no apikey in query")
		c.Abort()
		return
	}

	if apikey != config.GetSettingsGeneral().WebAPIKey {
		sendUnauthorized(c, "unauthorized - wrong apikey in query")
		c.Abort()
		return
	}

	c.Next()
}
