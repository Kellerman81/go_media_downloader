package api

import (
	gin "github.com/gin-gonic/gin"
)

func AddAllRoutes(rg *gin.RouterGroup) {
	//	movies := rg.Group("/movies")
	// movies.GET("/:ID", GetMovie)
	// movies.GET("/", GetMovies)
	// movies.POST("/", PostMovie)
	// movies.DELETE("/:ID", DeleteMovie)
	// movies.PUT("/:ID", UpdateMovie)
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
	go Movies_all_jobs("feeds", true)
	go Series_all_jobs("feeds", true)
}
func apiAllGetData(c *gin.Context) {
	go Movies_all_jobs("data", true)
	go Series_all_jobs("data", true)
}

func apiAllGetRss(c *gin.Context) {
	go Movies_all_jobs("rss", true)
	go Series_all_jobs("rss", true)
}

func apiAllGetMissingFull(c *gin.Context) {
	go Movies_all_jobs("searchmissingfull", true)
	go Series_all_jobs("searchmissingfull", true)
}
func apiAllGetMissingInc(c *gin.Context) {
	go Movies_all_jobs("searchmissinginc", true)
	go Series_all_jobs("searchmissinginc", true)
}

func apiAllGetUpgradeFull(c *gin.Context) {
	go Movies_all_jobs("searchupgradefull", true)
	go Series_all_jobs("searchupgradefull", true)
}
func apiAllGetUpgradeInc(c *gin.Context) {
	go Movies_all_jobs("searchupgradeinc", true)
	go Series_all_jobs("searchupgradeinc", true)
}
