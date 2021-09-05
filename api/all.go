package api

import (
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

func apiAllGetFeeds(c *gin.Context) {
	go utils.Movies_all_jobs("feeds", true)
	go utils.Series_all_jobs("feeds", true)
}
func apiAllGetData(c *gin.Context) {
	go utils.Movies_all_jobs("data", true)
	go utils.Series_all_jobs("data", true)
}

func apiAllGetRss(c *gin.Context) {
	go utils.Movies_all_jobs("rss", true)
	go utils.Series_all_jobs("rss", true)
}

func apiAllGetMissingFull(c *gin.Context) {
	go utils.Movies_all_jobs("searchmissingfull", true)
	go utils.Series_all_jobs("searchmissingfull", true)
}
func apiAllGetMissingInc(c *gin.Context) {
	go utils.Movies_all_jobs("searchmissinginc", true)
	go utils.Series_all_jobs("searchmissinginc", true)
}

func apiAllGetUpgradeFull(c *gin.Context) {
	go utils.Movies_all_jobs("searchupgradefull", true)
	go utils.Series_all_jobs("searchupgradefull", true)
}
func apiAllGetUpgradeInc(c *gin.Context) {
	go utils.Movies_all_jobs("searchupgradeinc", true)
	go utils.Series_all_jobs("searchupgradeinc", true)
}
