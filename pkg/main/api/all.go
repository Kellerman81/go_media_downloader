package api

import (
	"crypto/subtle"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/gin-gonic/gin"
)

// JSONNaming holds media parse info with folder and filename context.
type JSONNaming struct {
	M          database.ParseInfo `json:"m"`
	Foldername string             `json:"foldername"`
	Filename   string             `json:"filename"`
}

// Jsonresults is referenced only from Swagger @Success doc comments (never
// constructed in code), so it exists purely to document the shape of the
// accepted/denied lists for swag codegen.
type Jsonresults struct {
	Accepted []apiexternal_v2.Nzbwithprio `json:"accepted"`
	Denied   []apiexternal_v2.Nzbwithprio `json:"denied"`
}

// AddAllRoutes sets up HTTP routes for the "all" API endpoints.
// It configures routes for feeds, data scanning, and search operations
// that apply to every configured media type (movies, series, books,
// audiobooks, and music - not just movies/series, since utils.AllJobs with
// MediaTypeAll iterates config.RangeSettingsMedia with no type filter).
// All routes require API key authentication.
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

// @Summary      Import all lists of every media group
// @Description  Runs the feeds job for every list of every configured media group: new entries from IMDB, Trakt, TMDB, RSS and the other list types are added to the database. The request waits until all imports are finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/feeds [get].
func apiAllGetFeeds(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrFeeds, true)
	sendSuccess(c, StrOK)
}

// @Summary      Scan all library folders
// @Description  Runs the data scan for every media group: scans the library paths for new, changed and removed files and updates the file tables. The request waits until all scans are finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/data [get].
func apiAllGetData(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, "data", true)
	sendSuccess(c, StrOK)
}

// @Summary      RSS search for every media group
// @Description  Reads the RSS feeds of the indexers of every media group and downloads accepted releases for missing items and upgrades. The request waits until all searches are finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/search/rss [get].
func apiAllGetRss(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrRss, true)
	sendSuccess(c, StrOK)
}

// @Summary      Search all missing items (full)
// @Description  Searches the indexers for every missing item of every media group and downloads accepted releases. Can take a long time and use many indexer calls. The request waits until finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/search/missing/full [get].
func apiAllGetMissingFull(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrSearchMissingFull, true)
	sendSuccess(c, StrOK)
}

// @Summary      Search missing items (incremental)
// @Description  Searches the indexers for the missing items of every media group that were not searched recently (incremental batch) and downloads accepted releases. The request waits until finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/search/missing/inc [get].
func apiAllGetMissingInc(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrSearchMissingInc, true)
	sendSuccess(c, StrOK)
}

// @Summary      Search all upgrades (full)
// @Description  Searches the indexers for better releases of every item below its quality cutoff, in every media group, and downloads accepted upgrades. Can take a long time. The request waits until finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/search/upgrade/full [get].
func apiAllGetUpgradeFull(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrSearchUpgradeFull, true)
	sendSuccess(c, StrOK)
}

// @Summary      Search upgrades (incremental)
// @Description  Searches the indexers for better releases of items below their quality cutoff that were not searched recently (incremental batch), in every media group. The request waits until finished and then returns ok.
// @Tags         all
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Jobs queued for every media group"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/all/search/upgrade/inc [get].
func apiAllGetUpgradeInc(c *gin.Context) {
	utils.AllJobs(utils.MediaTypeAll, logger.StrSearchUpgradeInc, true)
	sendSuccess(c, StrOK)
}

var emptyAPIKeyLogOnce sync.Once

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

	expected := config.GetSettingsGeneral().WebAPIKey
	if expected == "" {
		// ConstantTimeCompare of two empty slices returns 1, so an unset key
		// would otherwise open the whole REST API to "?apikey=".
		emptyAPIKeyLogOnce.Do(func() {
			logger.Logtype("error", 0).
				Msg("REST API disabled: general.webapikey is empty - set an API key in the config to enable API access")
		})
		sendUnauthorized(c, "unauthorized - api key not configured")
		c.Abort()

		return
	}

	if apikey == "" {
		sendUnauthorized(c, "unauthorized - wrong apikey in query")
		c.Abort()
		return
	}

	if subtle.ConstantTimeCompare([]byte(apikey), []byte(expected)) != 1 {
		sendUnauthorized(c, "unauthorized - wrong apikey in query")
		c.Abort()
		return
	}

	c.Next()
}
