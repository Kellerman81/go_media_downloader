package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/Kellerman81/go_media_downloader/worker"
	gin "github.com/gin-gonic/gin"
)

func AddGeneralRoutes(routerapi *gin.RouterGroup) {
	routerapi.GET("/trakt/authorize", apiTraktGetAuthURL)
	routerapi.GET("/trakt/token/:code", apiTraktGetStoreToken)
	routerapi.GET("/trakt/user/:user/:list", apiTraktGetUserList)
	routerapi.GET("/debugstats", apiDebugStats)
	routerapi.GET("/queue", apiQueueList)
	routerapi.GET("/queue/history", apiQueueListStarted)
	routerapi.GET("/fillimdb", apiFillImdb)
	routerapi.GET("/scheduler/stop", apiSchedulerStop)
	routerapi.GET("/scheduler/start", apiSchedulerStart)
	routerapi.GET("/scheduler/list", apiSchedulerList)
	routerapi.GET("/db/close", apiDBClose)
	routerapi.GET("/db/integrity", apiDBIntegrity)
	routerapi.GET("/db/backup", apiDBBackup)
	routerapi.DELETE("/db/clear/:name", apiDBClear)
	routerapi.DELETE("/db/oldjobs", apiDBRemoveOldJobs)
	routerapi.GET("/db/vacuum", apiDBVacuum)
	routerapi.POST("/parse/string", apiParseString)
	routerapi.POST("/parse/file", apiParseFile)
	routerapi.POST("/naming", apiNamingGenerate)
	routerapi.POST("/structure", apiStructure)
	routerapi.GET("/quality", apiGetQualities)
	routerapi.DELETE("/quality/:id", apiQualityDelete)
	routerapi.POST("/quality", apiQualityUpdate)
	routerapi.GET("/quality/all", apiListAllQualityPriorities)
	routerapi.GET("/quality/complete", apiListCompleteAllQualityPriorities)
	routerapi.GET("/quality/get/:name", apiListQualityPriorities)
	routerapi.GET("/slug", apiDBRefreshSlugs)

	routerapi.GET("/config/all", apiConfigAll)
	routerapi.DELETE("/config/clear", apiConfigClear)
	routerapi.GET("/config/refresh", apiConfigRefreshFile)
	routerapi.GET("/config/get/:name", apiConfigGet)

	routerapi.DELETE("/config/delete/:name", apiConfigDelete)

	routerapi.POST("/config/update/:name", apiConfigUpdate)

	routerapi.GET("/config/type/:type", apiListConfigType)
}

type apiparse struct {
	Name    string
	Year    bool
	Typ     string
	Path    string
	Config  string
	Quality string
}

// @Summary      Debug information
// @Description  Shows some stats
// @Tags         general
// @Success      200
// @Failure      401  {object}  string
// @Router       /api/debugstats [get]
func apiDebugStats(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var gc debug.GCStats
	debug.ReadGCStats(&gc)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	gcjson, _ := json.Marshal(gc)
	memjson, _ := json.Marshal(mem)

	f, err := os.Create("./temp/heapdump")
	if err != nil {
		panic(err)
	}

	debug.WriteHeapDump(f.Fd())

	debug.FreeOSMemory()
	ctx.JSON(http.StatusOK, gin.H{"GC Stats": string(gcjson),
		"Mem Stats":    string(memjson),
		"GOOS":         runtime.GOOS,
		"NumCPU":       runtime.NumCPU(),
		"NumGoroutine": runtime.NumGoroutine(),
		"GOARCH":       runtime.GOARCH})
}

// @Summary      Queue
// @Description  Lists Queued and Started Jobs (but not finished)
// @Tags         general
// @Success      200  {object}  map[string]worker.Job
// @Failure      401  {object}  string
// @Router       /api/queue [get]
func apiQueueList(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}

	var r []worker.Job
	for _, value := range worker.GetQueues() {
		r = append(r, *value.Queue)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": r})
}

// @Summary      Queue History
// @Description  Lists Started Jobs and finished but not queued jobs
// @Tags         general
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.JobHistoryJSON
// @Failure      401    {object}  string
// @Router       /api/queue/history [get]
func apiQueueListStarted(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Querywithargs
	limit := 0
	query.OrderBy = "ID desc"
	query.Limit = 100
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
	jobs := database.QueryJobHistory(query)
	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}

// @Summary      Trakt Authorize
// @Description  Get trakt auth url
// @Tags         general
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/trakt/authorize [get]
func apiTraktGetAuthURL(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, apiexternal.TraktAPI.GetAuthURL())
}

// @Summary      Trakt Save Token
// @Description  Saves Trakt token after Authorization
// @Tags         general
// @Param        code  path      string  true  "code"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/trakt/token/{code} [get]
func apiTraktGetStoreToken(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}

	apiexternal.TraktAPI.Token = apiexternal.TraktAPI.GetAuthToken(ctx.Param("code"))

	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.TraktAPI.Token})
	ctx.JSON(http.StatusOK, gin.H{"data": apiexternal.TraktAPI.Token})

}

// @Summary      Trakt Get List (Auth Test)
// @Description  Get User List
// @Tags         general
// @Param        user  path      string  true  "Trakt Username"
// @Param        list  path      string  true  "List Name"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/trakt/user/{user}/{list} [get]
func apiTraktGetUserList(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	list, err := apiexternal.TraktAPI.GetUserList(ctx.Param("user"), ctx.Param("list"), "movie,show", 10)
	ctx.JSON(http.StatusOK, gin.H{"list": list, "error": err})
	//list = nil
}

// @Summary      Refresh Slugs
// @Description  Regenerates Slugs
// @Tags         general
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/slug [get]
func apiDBRefreshSlugs(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	dbmovies := database.QueryDbmovie(database.Querywithargs{})
	for idx := range *dbmovies {
		database.UpdateColumn("dbmovies", "slug", logger.StringToSlug((*dbmovies)[idx].Title), logger.FilterByID, (*dbmovies)[idx].ID)
	}

	dbmoviestitles := database.QueryDbmovieTitle(database.Querywithargs{})
	for idx := range *dbmoviestitles {
		database.UpdateColumn("dbmovie_titles", "slug", logger.StringToSlug((*dbmoviestitles)[idx].Title), logger.FilterByID, (*dbmoviestitles)[idx].ID)
	}

	dbserie := database.QueryDbserie(database.Querywithargs{})
	for idx := range *dbserie {

		database.UpdateColumn("dbseries", "slug", logger.StringToSlug((*dbserie)[idx].Seriename), logger.FilterByID, (*dbserie)[idx].ID)
	}

	dbserietitles := database.QueryDbserieAlternates(database.Querywithargs{})
	for idx := range *dbserietitles {
		database.UpdateColumn("dbserie_alternates", "slug", logger.StringToSlug((*dbserietitles)[idx].Title), logger.FilterByID, (*dbserietitles)[idx].ID)
	}
	ctx.JSON(http.StatusOK, "ok")

	logger.Clear(dbmovies)
	logger.Clear(dbmoviestitles)
	logger.Clear(dbserie)
	logger.Clear(dbserietitles)
}

// @Summary      Parse a string
// @Description  Parses a string for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Success      200      {object}  apiexternal.ParseInfo
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/parse/string [post]
func apiParseString(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse := parser.NewFileParser(getcfg.Name, getcfg.Year, getcfg.Typ)
	//defer parse.Close()
	if getcfg.Typ == logger.StrMovie {
		parser.GetPriorityMapQual(&parse.M, "movie_"+getcfg.Config, getcfg.Quality, true, true)
		parser.GetDBIDs(&parse.M, "movie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		parse.Close()
		return
	}
	if getcfg.Typ == logger.StrSeries {
		parser.GetPriorityMapQual(&parse.M, "serie_"+getcfg.Config, getcfg.Quality, true, true)
		parser.GetDBIDs(&parse.M, "serie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		parse.Close()
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
	parse.Close()
}

// @Summary      Parse a file
// @Description  Parses a file for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Success      200      {object}  apiexternal.ParseInfo
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/parse/file [post]
func apiParseFile(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse := parser.NewFileParser(filepath.Base(getcfg.Path), getcfg.Year, getcfg.Typ)
	//defer parse.Close()
	if getcfg.Typ == logger.StrMovie {
		parser.ParseVideoFile(&parse.M, &getcfg.Path, getcfg.Quality)
		parser.GetPriorityMapQual(&parse.M, "movie_"+getcfg.Config, getcfg.Quality, true, true)
		parser.GetDBIDs(&parse.M, "movie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		parse.Close()
		return
	}
	if getcfg.Typ == logger.StrSeries {
		parser.ParseVideoFile(&parse.M, &getcfg.Path, getcfg.Quality)
		parser.GetPriorityMapQual(&parse.M, "serie_"+getcfg.Config, getcfg.Quality, true, true)
		parser.GetDBIDs(&parse.M, "serie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		parse.Close()
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
	parse.Close()
}

// @Summary      Generate IMDB Cache
// @Description  Downloads IMDB Dataset and creates a new database from it
// @Tags         general
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/fillimdb [get]
func apiFillImdb(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	utils.FillImdb()

	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Stop Scheduler
// @Description  Stops all Schedulers
// @Tags         scheduler
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/scheduler/stop [get]
func apiSchedulerStop(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	// scheduler.QueueData.Stop()
	// scheduler.QueueFeeds.Stop()
	// scheduler.QueueSearch.Stop()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Start Scheduler
// @Description  Start all Schedulers
// @Tags         scheduler
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/scheduler/start [get]
func apiSchedulerStart(c *gin.Context) {
	if auth(c) == http.StatusUnauthorized {
		return
	}
	// scheduler.QueueData.Start()
	// scheduler.QueueFeeds.Start()
	// scheduler.QueueSearch.Start()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Scheduler Jobs
// @Description  Lists Planned Jobs
// @Tags         scheduler
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/scheduler/list [get]
func apiSchedulerList(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var r []worker.JobSchedule
	for _, value := range worker.GetSchedules() {
		r = append(r, value)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": r})
	//r = nil
}

// @Summary      Close DB
// @Description  Closes all database connections
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/close [get]
func apiDBClose(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DBClose()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Backup DB
// @Description  Saves DB
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/backup [get]
func apiDBBackup(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.SettingsGeneral.MaxDatabaseBackups)
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Integrity DB
// @Description  Integrity Check DB
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/integrity [get]
func apiDBIntegrity(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, database.DBIntegrityCheck())
}

// @Summary      Clear DB Table
// @Description  Clears a DB Table
// @Tags         database
// @Param        name  path      string  true  "Table Name"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/db/clear/{name} [delete]
func apiDBClear(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.Dbexec("DELETE from "+ctx.Param("name"), []interface{}{})
	database.Dbexec("VACUUM", []interface{}{})
	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      Vacuum DB
// @Description  Vacuum database
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/vacuum [get]
func apiDBVacuum(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.Dbexec("VACUUM", []interface{}{})
	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      Clean Old Jobs
// @Description  Removes Jobs started over x days ago from db
// @Tags         database
// @Param        days  query     int  true  "Days ago"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/db/oldjobs [delete]
func apiDBRemoveOldJobs(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	if queryParam, ok := ctx.GetQuery("days"); ok {
		if queryParam != "" {
			days, _ := strconv.Atoi(queryParam)

			scantime := time.Now()
			if days != 0 {
				scantime = scantime.AddDate(0, 0, 0-days)
				_, err := database.DeleteRow(false, "job_histories", "created_at < ?", scantime)
				if err == nil {
					ctx.JSON(http.StatusOK, "ok")
				} else {
					ctx.JSON(http.StatusForbidden, err)
				}
			}
		} else {
			ctx.JSON(http.StatusForbidden, errors.New("days empty"))
		}
	} else {
		ctx.JSON(http.StatusForbidden, errors.New("days missing"))
	}
}

// @Summary      List Qualities
// @Description  List Qualities with regex filters
// @Tags         quality
// @Success      200  {object}   database.Qualities
// @Failure      401  {object}  string
// @Router       /api/quality [get]
func apiGetQualities(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryQualities("select * from qualities")})
}

// @Summary      Delete Quality
// @Description  Deletes a quality
// @Tags         quality
// @Param        id   path      string  true  "Id of Quality to delete"
// @Success      200  {object}   database.Qualities
// @Failure      401  {object}  string
// @Router       /api/quality/{id} [delete]
func apiQualityDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow(false, "qualities", logger.FilterByID, ctx.Param("id"))
	database.GetVars()
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryQualities("select * from qualities")})
}

// @Summary      Update Quality
// @Description  Updates or adds a quality
// @Tags         quality
// @Param        quality  body      database.Qualities  true  "Quality"
// @Success      200      {object}   database.Qualities
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/quality [post]
func apiQualityUpdate(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var quality database.Qualities
	if err := ctx.ShouldBindJSON(&quality); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.QueryIntColumn("select count() from qualities where id != 0 and id = ?", quality.ID)

	if counter == 0 {
		database.InsertArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex)
	} else {
		database.UpdateArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			"id != 0 and id = ?", quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex, quality.ID)
	}
	database.GetVars()
	ctx.JSON(http.StatusOK, gin.H{"data": database.QueryQualities("select * from qualities")})
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        name    path      string  true  "Quality Name: ex. SD"
// @Success      200     {object}   apiexternal.ParseInfo
// @Failure      401  {object}  string
// @Failure      404     {object}  string
// @Router       /api/quality/get/{name} [get]
func apiListQualityPriorities(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	if !config.CheckGroup("quality_", ctx.Param("name")) {
		ctx.JSON(http.StatusNotFound, "quality not found")
		return
	}
	returnprios := make([]parser.Prioarr, 1000)
	for _, prio := range parser.Getallprios() {
		if prio.QualityGroup == ctx.Param("name") {
			returnprios = append(returnprios, prio)
		}
	}
	//ctx.JSON(http.StatusOK, gin.H{"data": parser.Getallprios()[ctx.Param("name")]})
	ctx.JSON(http.StatusOK, gin.H{"data": returnprios})
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Success      200     {object}   apiexternal.ParseInfo
// @Failure      401  {object}  string
// @Failure      404     {object}  string
// @Router       /api/quality/all [get]
func apiListAllQualityPriorities(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parser.Getallprios()})
}

// @Summary      List Quality Priorities
// @Description  List all qualities and their priorities
// @Tags         quality
// @Success      200     {object}   apiexternal.ParseInfo
// @Failure      401  {object}  string
// @Failure      404     {object}  string
// @Router       /api/quality/complete [get]
func apiListCompleteAllQualityPriorities(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parser.Getcompleteallprios()})
}

// @Summary      Get Complete Config
// @Description  Get All Config Parameters
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401  {object}  string
// @Router       /api/config/all [get]
func apiConfigAll(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Clear Config
// @Description  Clears the configuration and sets some examples
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401   {object}  string
// @Router       /api/config/clear [delete]
func apiConfigClear(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	config.ClearCfg()
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Get Config
// @Description  Gets a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Success      200   {object}  interface{}
// @Failure      401   {object}  string
// @Router       /api/config/get/{name} [get]
func apiConfigGet(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	name := ctx.Param("name")
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()[name]})
	//data = nil
}

// @Summary      Delete Config
// @Description  Deletes a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Success      200   {array}  map[string]interface{}
// @Failure      401  {object}  string
// @Router       /api/config/delete/{name} [delete]
func apiConfigDelete(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	str := ctx.Param("name")
	config.DeleteCfgEntry(str)
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Reload ConfigFile
// @Description  Refreshes the config from the file
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401   {object}  string
// @Router       /api/config/refresh [get]
func apiConfigRefreshFile(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	config.LoadCfgDB()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Update Config
// @Description  Updates a configuration
// @Tags         config
// @Param        config  body      interface{}  true  "Config"
// @Param        name    path      string       true  "Type Name: ex. quality_SD"
// @Success      200   {array}  map[string]interface{}
// @Failure      400     {object}  string
// @Failure      401     {object}  string
// @Router       /api/config/update/{name} [post]
func apiConfigUpdate(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	name := ctx.Param("name")
	if strings.HasPrefix(name, "general") {
		var getcfg config.GeneralConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "downloader_") {
		var getcfg config.DownloaderConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, logger.StrImdb) {
		var getcfg config.ImdbConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "indexer") {
		var getcfg config.IndexersConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "list") {
		var getcfg config.ListsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, logger.StrSeries) {
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, logger.StrMovie) {
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "notification") {
		var getcfg config.NotificationConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "path") {
		var getcfg config.PathsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "quality") {
		var getcfg config.QualityConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "regex") {
		var getcfg config.RegexConfigIn
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "scheduler") {
		var getcfg config.SchedulerConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      List Config Type
// @Description  List configurations of type
// @Tags         config
// @Param        type  path      string  true  "Type Name: ex. quality"
// @Success      200     {object}  map[string]interface{}
// @Failure      401     {object}  string
// @Router       /api/config/type/{type} [get]
func apiListConfigType(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	list := make(map[string]interface{})
	name := ctx.Param("type")
	if strings.HasPrefix("general", name) {
		list["general"] = config.SettingsGeneral
	}
	if strings.HasPrefix(logger.StrImdb, name) {
		list[logger.StrImdb] = config.SettingsImdb
	}
	if strings.HasPrefix("downloader_", name) {
		for key, cfgdata := range config.SettingsDownloader {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("indexer_", name) {
		for key, cfgdata := range config.SettingsIndexer {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("list_", name) {
		for key, cfgdata := range config.SettingsList {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("movie_", name) {
		for idxp := range config.SettingsMedia {
			if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
				continue
			}
			if strings.HasPrefix(config.SettingsMedia[idxp].Name, name) {
				list[config.SettingsMedia[idxp].Name] = config.SettingsMedia[idxp]
			}
		}
	}
	if strings.HasPrefix("notification_", name) {
		for key, cfgdata := range config.SettingsNotification {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("path_", name) {
		for key, cfgdata := range config.SettingsPath {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("quality_", name) {
		for key, cfgdata := range config.SettingsQuality {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("regex_", name) {
		for key, cfgdata := range config.SettingsRegex {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("scheduler_", name) {
		for key, cfgdata := range config.SettingsScheduler {
			if strings.HasPrefix(key, name) {
				list[key] = cfgdata
			}
		}
	}
	if strings.HasPrefix("serie_", name) {
		for idxp := range config.SettingsMedia {
			if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
				continue
			}
			if strings.HasPrefix(config.SettingsMedia[idxp].Name, name) {
				list[config.SettingsMedia[idxp].Name] = config.SettingsMedia[idxp]
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
	//list = nil
}

type apiNameInput struct {
	CfgMedia  string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}

// @Summary      Name Generation Test
// @Description  Test your Naming Convention
// @Tags         general
// @Param        config  body      apiNameInput  true  "Config"
// @Success      200     {object}  string
// @Failure      400     {object}  string
// @Failure      401     {object}  string
// @Router       /api/naming [post]
func apiNamingGenerate(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg apiNameInput
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//defer mediacfg.Close()
	if cfg.GroupType == logger.StrMovie {
		movie, _ := database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, cfg.MovieID)
		//defer logger.ClearVar(&movie)

		s, _ := structure.NewStructure(
			cfg.CfgMedia,
			movie.Listname,
			cfg.GroupType,
			movie.Rootpath,
			config.SettingsMedia[cfg.CfgMedia].DataImport[0].TemplatePath,
			config.SettingsMedia[cfg.CfgMedia].Data[0].TemplatePath,
		)
		//defer s.Close()
		to := filepath.Dir(cfg.FilePath)
		m := parser.ParseFile(&cfg.FilePath, true, cfg.GroupType == logger.StrSeries, cfg.GroupType, true)
		logger.Log.Debug().Any("m", &m.M).Send()
		s.ParseFileAdditional(&cfg.FilePath, to, false, 0, false, &m.M)

		foldername, filename := s.GenerateNamingTemplate(&cfg.FilePath, movie.Rootpath, movie.DbmovieID, "", "", nil, &m.M)
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename, "m": &m.M})
		m.Close()
		//s.Close()
	} else {
		series, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, cfg.SerieID)
		//defer logger.ClearVar(&series)

		s, _ := structure.NewStructure(
			cfg.CfgMedia,
			series.Listname,
			cfg.GroupType,
			series.Rootpath,
			config.SettingsMedia[cfg.CfgMedia].DataImport[0].TemplatePath,
			config.SettingsMedia[cfg.CfgMedia].Data[0].TemplatePath,
		)
		//defer s.Close()
		to := filepath.Dir(cfg.FilePath)
		m := parser.ParseFile(&cfg.FilePath, true, cfg.GroupType == logger.StrSeries, cfg.GroupType, true)
		logger.Log.Debug().Any("m", &m.M).Send()
		s.ParseFileAdditional(&cfg.FilePath, to, false, 0, false, &m.M)

		_, _, mapepi, _ := s.GetSeriesEpisodes(series.ID, series.DbserieID, &cfg.FilePath, to, &m.M, true)

		var firstdbepiid, firstepiid uint
		for key := range *mapepi {
			firstdbepiid = (*mapepi)[key].Num2
			firstepiid = (*mapepi)[key].Num1
			break
		}

		serietitle, episodetitle := s.GetEpisodeTitle(firstdbepiid, m.M.DbserieID, &cfg.FilePath, &m.M)

		foldername, filename := s.GenerateNamingTemplate(&cfg.FilePath, series.Rootpath, firstepiid, serietitle, episodetitle, mapepi, &m.M)
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename, "map": mapepi, "m": &m.M})
		m.Close()
		//s.Close()
		logger.Clear(mapepi)
	}
}

type apiStructureJSON struct {
	Folder                     string
	Disableruntimecheck        bool
	Disabledisallowed          bool
	Disabledeletewronglanguage bool
	Grouptype                  string
	Sourcepathtemplate         string
	Targetpathtemplate         string
	Configentry                string
	Forceid                    uint
}

// @Summary      Structure Single Item
// @Description  Structure a single folder
// @Tags         general
// @Param        config  body      apiStructureJSON  true  "Config"
// @Success      200     {object}  string
// @Failure      400     {object}  string
// @Failure      401     {object}  string
// @Router       /api/structure [post]
func apiStructure(ctx *gin.Context) {
	if auth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg apiStructureJSON
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var getconfig string
	if strings.EqualFold(cfg.Grouptype, logger.StrMovie) {
		getconfig = "movie_" + cfg.Configentry
	}
	if strings.EqualFold(cfg.Grouptype, logger.StrSeries) {
		getconfig = "serie_" + cfg.Configentry
	}
	//defer media.Close()
	if config.SettingsMedia[getconfig].Name != cfg.Configentry {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "media config not found"})
		return
	}

	if !config.CheckGroup("path_", cfg.Sourcepathtemplate) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "source config not found"})
		return
	}

	if !config.CheckGroup("path_", cfg.Targetpathtemplate) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "target config not found"})
		return
	}
	if !scanner.CheckFileExist(&cfg.Folder) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "folder not found"})
		return
	}
	cacheunmatched := logger.StrSerieFileUnmatched
	if cfg.Grouptype != logger.StrSeries {
		cacheunmatched = logger.StrMovieFileUnmatched
	}
	structurevar, _ := structure.NewStructure(getconfig, "", cfg.Grouptype, "", cfg.Sourcepathtemplate, cfg.Targetpathtemplate)

	if cfg.Forceid != 0 {
		structure.OrganizeSingleFolder(cfg.Folder, cfg.Disableruntimecheck, cfg.Disabledeletewronglanguage, cacheunmatched, structurevar, cfg.Forceid)
	} else {
		structure.OrganizeSingleFolder(cfg.Folder, cfg.Disableruntimecheck, cfg.Disabledeletewronglanguage, cacheunmatched, structurevar)
	}

	structurevar.Close()
}
