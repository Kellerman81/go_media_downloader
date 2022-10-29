package api

import (
	"encoding/json"
	"errors"
	"net/http"
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
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
)

func AddGeneralRoutes(routerapi *gin.RouterGroup) {
	routerapi.GET("/trakt/authorize", apiTraktGetAuthUrl)
	routerapi.GET("/trakt/token/:code", apiTraktGetStoreToken)
	routerapi.GET("/trakt/user/:user/:list", apiTraktGetUserList)
	routerapi.GET("/debugstats", apiDebugStats)
	routerapi.GET("/queue", apiQueueList)
	routerapi.GET("/queue/history", apiQueueListStarted)
	routerapi.GET("/fillimdb", apiFillImdb)
	routerapi.GET("/scheduler/stop", apiSchedulerStop)
	routerapi.GET("/scheduler/start", apiSchedulerStart)
	routerapi.GET("/scheduler/list", apiSchedulerList)
	routerapi.GET("/db/close", apiDbClose)
	routerapi.GET("/db/integrity", apiDbIntegrity)
	routerapi.GET("/db/backup", apiDbBackup)
	routerapi.DELETE("/db/clear/:name", apiDbClear)
	routerapi.DELETE("/db/oldjobs", apiDbRemoveOldJobs)
	routerapi.GET("/db/vacuum", apiDbVacuum)
	routerapi.POST("/parse/string", apiParseString)
	routerapi.POST("/parse/file", apiParseFile)
	routerapi.POST("/naming", apiNamingGenerate)
	routerapi.POST("/structure", apiStructure)
	routerapi.GET("/quality", apiGetQualities)
	routerapi.DELETE("/quality/:id", apiQualityDelete)
	routerapi.POST("/quality", apiQualityUpdate)
	routerapi.GET("/quality/:name/:config", apiListQualityPriorities)
	routerapi.GET("/quality/all", apiListAllQualityPriorities)
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
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var gc debug.GCStats
	debug.ReadGCStats(&gc)
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	gcjson, _ := json.Marshal(gc)
	memjson, _ := json.Marshal(mem)

	runtime.GC()
	ctx.JSON(http.StatusOK, gin.H{"GC Stats": string(gcjson), "Mem Stats": string(memjson)})
}

// @Summary      Queue
// @Description  Lists Queued and Started Jobs (but not finished)
// @Tags         general
// @Success      200  {object}  map[string]tasks.Job
// @Failure      401  {object}  string
// @Router       /api/queue [get]
func apiQueueList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}

	var r []tasks.Job
	for _, value := range tasks.GetQueues() {
		r = append(r, value.Queue)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": r})
}

// @Summary      Queue History
// @Description  Lists Started Jobs and finished but not queued jobs
// @Tags         general
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Success      200    {object}   database.JobHistoryJson
// @Failure      401    {object}  string
// @Router       /api/queue/history [get]
func apiQueueListStarted(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var query database.Query
	limit := 0
	query.OrderBy = "ID desc"
	query.Limit = 100
	page := 0
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint64(limit)
		}
	}
	if limit != 0 {
		if queryParam, ok := ctx.GetQuery("page"); ok {
			if queryParam != "" {
				page, _ = strconv.Atoi(queryParam)
				if page >= 2 {
					query.Offset = uint64((page - 1) * limit)
				}
			}
		}
	}
	if queryParam, ok := ctx.GetQuery("order"); ok {
		if queryParam != "" {
			query.OrderBy = queryParam
		}
	}
	jobs, _ := database.QueryJobHistory(&query)
	ctx.JSON(http.StatusOK, gin.H{"data": jobs})
}

// @Summary      Trakt Authorize
// @Description  Get trakt auth url
// @Tags         general
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/trakt/authorize [get]
func apiTraktGetAuthUrl(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, apiexternal.TraktApi.GetAuthUrl())
}

// @Summary      Trakt Save Token
// @Description  Saves Trakt token after Authorization
// @Tags         general
// @Param        code  path      string  true  "code"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/trakt/token/{code} [get]
func apiTraktGetStoreToken(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}

	token := apiexternal.TraktApi.GetAuthToken(ctx.Param("code"))
	apiexternal.TraktApi.Token = token

	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: *token})
	ctx.JSON(http.StatusOK, gin.H{"data": *token})

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
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	list, err := apiexternal.TraktApi.GetUserList(ctx.Param("user"), ctx.Param("list"), "movie,show", 10)
	ctx.JSON(http.StatusOK, gin.H{"list": list, "error": err})
}

// @Summary      Refresh Slugs
// @Description  Regenerates Slugs
// @Tags         general
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/slug [get]
func apiDBRefreshSlugs(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	dbmovies, _ := database.QueryDbmovie(&database.Query{})
	for idx := range dbmovies {
		database.UpdateColumn("dbmovies", "slug", logger.StringToSlug(dbmovies[idx].Title), &database.Query{Where: "id = ?"}, dbmovies[idx].ID)
	}

	dbmoviestitles, _ := database.QueryDbmovieTitle(&database.Query{})
	for idx := range dbmoviestitles {
		database.UpdateColumn("dbmovie_titles", "slug", logger.StringToSlug(dbmoviestitles[idx].Title), &database.Query{Where: "id = ?"}, dbmoviestitles[idx].ID)
	}

	dbserie, _ := database.QueryDbserie(&database.Query{})
	for idx := range dbserie {
		database.UpdateColumn("dbseries", "slug", logger.StringToSlug(dbserie[idx].Seriename), &database.Query{Where: "id = ?"}, dbserie[idx].ID)
	}

	dbserietitles, _ := database.QueryDbserieAlternates(&database.Query{})
	for idx := range dbserietitles {
		database.UpdateColumn("dbserie_alternates", "slug", logger.StringToSlug(dbserietitles[idx].Title), &database.Query{Where: "id = ?"}, dbserietitles[idx].ID)
	}
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Parse a string
// @Description  Parses a string for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Success      200      {object}  parser.ParseInfo
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/parse/string [post]
func apiParseString(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse, _ := parser.NewFileParser(getcfg.Name, getcfg.Year, getcfg.Typ)
	//defer parse.Close()
	if getcfg.Typ == "movie" {
		parser.GetPriorityMap(parse, "movie_"+getcfg.Config, getcfg.Quality, true)
		parser.GetDbIDs("movie", parse, "movie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		return
	}
	if getcfg.Typ == "series" {
		parser.GetPriorityMap(parse, "serie_"+getcfg.Config, getcfg.Quality, true)
		parser.GetDbIDs("series", parse, "serie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
}

// @Summary      Parse a file
// @Description  Parses a file for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Success      200      {object}  parser.ParseInfo
// @Failure      400      {object}  string
// @Failure      401      {object}  string
// @Router       /api/parse/file [post]
func apiParseFile(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse, _ := parser.NewFileParser(filepath.Base(getcfg.Path), getcfg.Year, getcfg.Typ)
	//defer parse.Close()
	if getcfg.Typ == "movie" {
		parser.ParseVideoFile(parse, getcfg.Path, "movie_"+getcfg.Config, getcfg.Quality)
		parser.GetPriorityMap(parse, "movie_"+getcfg.Config, getcfg.Quality, true)
		parser.GetDbIDs("movie", parse, "movie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		return
	}
	if getcfg.Typ == "series" {
		parser.ParseVideoFile(parse, getcfg.Path, "serie_"+getcfg.Config, getcfg.Quality)
		parser.GetPriorityMap(parse, "serie_"+getcfg.Config, getcfg.Quality, true)
		parser.GetDbIDs("series", parse, "serie_"+getcfg.Config, "", true)
		ctx.JSON(http.StatusOK, gin.H{"data": parse})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
}

// @Summary      Generate IMDB Cache
// @Description  Downloads IMDB Dataset and creates a new database from it
// @Tags         general
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/fillimdb [get]
func apiFillImdb(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
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
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Start Scheduler
// @Description  Start all Schedulers
// @Tags         scheduler
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/scheduler/start [get]
func apiSchedulerStart(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueData.Start()
	scheduler.QueueFeeds.Start()
	scheduler.QueueSearch.Start()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Scheduler Jobs
// @Description  Lists Planned Jobs
// @Tags         scheduler
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/scheduler/list [get]
func apiSchedulerList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var r []tasks.JobSchedule
	for _, value := range tasks.GetSchedules() {
		r = append(r, value)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": r})
}

// @Summary      Close DB
// @Description  Closes all database connections
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/close [get]
func apiDbClose(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
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
func apiDbBackup(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.Cfg.General.MaxDatabaseBackups)
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Integrity DB
// @Description  Integrity Check DB
// @Tags         database
// @Success      200  {object}  string
// @Failure      401  {object}  string
// @Router       /api/db/integrity [get]
func apiDbIntegrity(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, database.DbIntegrityCheck())
}

// @Summary      Clear DB Table
// @Description  Clears a DB Table
// @Tags         database
// @Param        name  path      string  true  "Table Name"
// @Success      200   {object}  string
// @Failure      401   {object}  string
// @Router       /api/db/clear/{name} [delete]
func apiDbClear(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.Dbexec("main", "DELETE from "+ctx.Param("name"), []interface{}{})
	database.Dbexec("main", "VACUUM", []interface{}{})
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
func apiDbVacuum(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	_, err := database.Dbexec("main", "VACUUM", []interface{}{})
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
func apiDbRemoveOldJobs(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	if queryParam, ok := ctx.GetQuery("days"); ok {
		if queryParam != "" {
			days, _ := strconv.Atoi(queryParam)

			scantime := time.Now()
			if days != 0 {
				scantime = scantime.AddDate(0, 0, 0-days)
				_, err := database.DeleteRow("job_histories", &database.Query{Where: "created_at < ?"}, scantime)
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
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	qualities, _ := database.QueryQualities(&database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": qualities})
}

// @Summary      Delete Quality
// @Description  Deletes a quality
// @Tags         quality
// @Param        id   path      string  true  "Id of Quality to delete"
// @Success      200  {object}   database.Qualities
// @Failure      401  {object}  string
// @Router       /api/quality/{id} [delete]
func apiQualityDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("qualities", &database.Query{Where: "id = ?"}, ctx.Param("id"))
	database.GetVars()
	qualities, _ := database.QueryQualities(&database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": qualities})
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
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var quality database.Qualities
	if err := ctx.ShouldBindJSON(&quality); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("qualities", &database.Query{Where: "id != 0 and id = ?"}, quality.ID)

	if counter == 0 {
		database.InsertArray("qualities", &logger.InStringArrayStruct{Arr: []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"}},
			[]interface{}{quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex})
	} else {
		database.UpdateArray("qualities", &logger.InStringArrayStruct{Arr: []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"}},
			[]interface{}{quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex},
			&database.Query{Where: "id != 0 and id = ?"}, quality.ID)
	}
	database.GetVars()
	qualities, _ := database.QueryQualities(&database.Query{})
	ctx.JSON(http.StatusOK, gin.H{"data": qualities})
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        name    path      string  true  "Quality Name: ex. SD"
// @Param        config  path      string  true  "Config Name: ex. movie_EN or serie_EN"
// @Success      200     {object}   parser.ParseInfo
// @Failure      401  {object}  string
// @Failure      404     {object}  string
// @Router       /api/quality/{name}/{config} [get]
func apiListQualityPriorities(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	if !config.ConfigCheck("quality_" + ctx.Param("name")) {
		ctx.JSON(http.StatusNotFound, "quality not found")
		return
	}
	if !config.ConfigCheck(ctx.Param("config")) {
		ctx.JSON(http.StatusNotFound, "config not found")
		return
	}
	//defer qual.Close()

	var parserreturn []apiexternal.ParseInfo
	for idxreso := range database.DBConnect.Getresolutions {
		wantedreso := false

		for idxwantreso := range config.Cfg.Quality[ctx.Param("name")].Wanted_resolution {
			if strings.EqualFold(config.Cfg.Quality[ctx.Param("name")].Wanted_resolution[idxwantreso], database.DBConnect.Getresolutions[idxreso].Name) {
				wantedreso = true
				break
			}
		}
		if !wantedreso {
			continue
		}
		for idxqual := range database.DBConnect.Getqualities {
			wantedqual := false
			for idxwantqual := range config.Cfg.Quality[ctx.Param("name")].Wanted_quality {
				if strings.EqualFold(config.Cfg.Quality[ctx.Param("name")].Wanted_quality[idxwantqual], database.DBConnect.Getqualities[idxqual].Name) {
					wantedqual = true
					break
				}
			}
			if !wantedqual {
				continue
			}
			for idxcodec := range database.DBConnect.Getcodecs {
				for idxaudio := range database.DBConnect.Getaudios {
					parse := apiexternal.ParseInfo{
						Resolution:   database.DBConnect.Getresolutions[idxreso].Name,
						Quality:      database.DBConnect.Getqualities[idxqual].Name,
						Codec:        database.DBConnect.Getcodecs[idxcodec].Name,
						Audio:        database.DBConnect.Getaudios[idxaudio].Name,
						ResolutionID: database.DBConnect.Getresolutions[idxreso].ID,
						QualityID:    database.DBConnect.Getqualities[idxqual].ID,
						CodecID:      database.DBConnect.Getcodecs[idxcodec].ID,
						AudioID:      database.DBConnect.Getaudios[idxaudio].ID,
					}
					parser.GetIDPriorityMap(&parse, ctx.Param("config"), ctx.Param("name"), true)
					parserreturn = append(parserreturn, parse)
				}
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parserreturn})
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        name    path      string  true  "Quality Name: ex. SD"
// @Param        config  path      string  true  "Config Name: ex. movie_EN or serie_EN"
// @Success      200     {object}   parser.ParseInfo
// @Failure      401  {object}  string
// @Failure      404     {object}  string
// @Router       /api/quality/all [get]
func apiListAllQualityPriorities(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": parser.Getallprios()})
}

// @Summary      Get Complete Config
// @Description  Get All Config Parameters
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401  {object}  string
// @Router       /api/config/all [get]
func apiConfigAll(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"data": config.Cfg})
}

// @Summary      Clear Config
// @Description  Clears the configuration and sets some examples
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401   {object}  string
// @Router       /api/config/clear [delete]
func apiConfigClear(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	config.ClearCfg()
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.Cfg})
}

// @Summary      Get Config
// @Description  Gets a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Success      200   {object}  interface{}
// @Failure      401   {object}  string
// @Router       /api/config/get/{name} [get]
func apiConfigGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var data interface{}
	name := ctx.Param("name")
	if name == "general" {
		data = config.Cfg.General
	}
	if name == "imdb" || name == "imdbindexer" {
		data = config.Cfg.Imdbindexer
	}

	if strings.HasPrefix(name, "downloader") {
		data = config.Cfg.Downloader[strings.Replace(name, "downloader_", "", 1)]
	}
	if strings.HasPrefix(name, "indexer_") {
		data = config.Cfg.Indexers[strings.Replace(name, "indexer_", "", 1)]
	}
	if strings.HasPrefix(name, "list_") {
		data = config.Cfg.Lists[strings.Replace(name, "list_", "", 1)]
	}
	if strings.HasPrefix(name, "movie_") {
		data = config.Cfg.Movies[strings.Replace(name, "movie_", "", 1)]
	}
	if strings.HasPrefix(name, "notification_") {
		data = config.Cfg.Notification[strings.Replace(name, "notification_", "", 1)]
	}
	if strings.HasPrefix(name, "path_") {
		data = config.Cfg.Paths[strings.Replace(name, "path_", "", 1)]
	}
	if strings.HasPrefix(name, "quality_") {
		data = config.Cfg.Quality[strings.Replace(name, "quality_", "", 1)]
	}
	if strings.HasPrefix(name, "regex_") {
		data = config.Cfg.Regex[strings.Replace(name, "regex_", "", 1)]
	}
	if strings.HasPrefix(name, "scheduler_") {
		data = config.Cfg.Scheduler[strings.Replace(name, "scheduler_", "", 1)]
	}
	if strings.HasPrefix(name, "serie_") {
		data = config.Cfg.Series[strings.Replace(name, "serie_", "", 1)]
	}
	ctx.JSON(http.StatusOK, gin.H{"data": data})
}

// @Summary      Delete Config
// @Description  Deletes a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Success      200   {array}  map[string]interface{}
// @Failure      401  {object}  string
// @Router       /api/config/delete/{name} [delete]
func apiConfigDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	config.DeleteCfgEntry(ctx.Param("name"))
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.Cfg})
}

// @Summary      Reload ConfigFile
// @Description  Refreshes the config from the file
// @Tags         config
// @Success      200  {array}  map[string]interface{}
// @Failure      401   {object}  string
// @Router       /api/config/refresh [get]
func apiConfigRefreshFile(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	config.LoadCfgDB(config.GetCfgFile())
	ctx.JSON(http.StatusOK, gin.H{"data": config.Cfg})
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
	if ApiAuth(ctx) == http.StatusUnauthorized {
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
	if strings.HasPrefix(name, "imdb") {
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
	if strings.HasPrefix(name, "serie") {
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	}
	if strings.HasPrefix(name, "movie") {
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
	ctx.JSON(http.StatusOK, gin.H{"data": config.Cfg})
}

// @Summary      List Config Type
// @Description  List configurations of type
// @Tags         config
// @Param        type  path      string  true  "Type Name: ex. quality"
// @Success      200     {object}  map[string]interface{}
// @Failure      401     {object}  string
// @Router       /api/config/type/{type} [get]
func apiListConfigType(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	list := make(map[string]interface{})
	name := ctx.Param("type")
	if strings.HasPrefix("general", name) {
		list["general"] = config.Cfg.General
	}
	if strings.HasPrefix("imdb", name) {
		list["imdb"] = config.Cfg.Imdbindexer
	}
	if strings.HasPrefix("downloader_", name) {
		for key := range config.Cfg.Downloader {
			if strings.HasPrefix("downloader_"+key, name) {
				list["downloader_"+key] = config.Cfg.Downloader[key]
			}
		}
	}
	if strings.HasPrefix("indexer_", name) {
		for key := range config.Cfg.Indexers {
			if strings.HasPrefix("indexer_"+key, name) {
				list["indexer_"+key] = config.Cfg.Indexers[key]
			}
		}
	}
	if strings.HasPrefix("list_", name) {
		for key := range config.Cfg.Lists {
			if strings.HasPrefix("list_"+key, name) {
				list["list_"+key] = config.Cfg.Lists[key]
			}
		}
	}
	if strings.HasPrefix("movie_", name) {
		for key := range config.Cfg.Movies {
			if strings.HasPrefix("movie_"+key, name) {
				list["movie_"+key] = config.Cfg.Movies[key]
			}
		}
	}
	if strings.HasPrefix("notification_", name) {
		for key := range config.Cfg.Notification {
			if strings.HasPrefix("notification_"+key, name) {
				list["notification_"+key] = config.Cfg.Notification[key]
			}
		}
	}
	if strings.HasPrefix("path_", name) {
		for key := range config.Cfg.Paths {
			if strings.HasPrefix("path_"+key, name) {
				list["path_"+key] = config.Cfg.Paths[key]
			}
		}
	}
	if strings.HasPrefix("quality_", name) {
		for key := range config.Cfg.Quality {
			if strings.HasPrefix("quality_"+key, name) {
				list["quality_"+key] = config.Cfg.Quality[key]
			}
		}
	}
	if strings.HasPrefix("regex_", name) {
		for key := range config.Cfg.Regex {
			if strings.HasPrefix("regex_"+key, name) {
				list["regex_"+key] = config.Cfg.Regex[key]
			}
		}
	}
	if strings.HasPrefix("scheduler_", name) {
		for key := range config.Cfg.Scheduler {
			if strings.HasPrefix("scheduler_"+key, name) {
				list["scheduler_"+key] = config.Cfg.Scheduler[key]
			}
		}
	}
	if strings.HasPrefix("serie_", name) {
		for key := range config.Cfg.Series {
			if strings.HasPrefix("serie_"+key, name) {
				list["serie_"+key] = config.Cfg.Series[key]
			}
		}
	}
	ctx.JSON(http.StatusOK, gin.H{"data": list})
}

type apiNameInput struct {
	Cfg_Media string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}
type apiNameInputJson struct {
	Cfg_Media string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}

// @Summary      Name Generation Test
// @Description  Test your Naming Convention
// @Tags         general
// @Param        config  body      apiNameInputJson  true  "Config"
// @Success      200     {object}  string
// @Failure      400     {object}  string
// @Failure      401     {object}  string
// @Router       /api/naming [post]
func apiNamingGenerate(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg apiNameInput
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//defer mediacfg.Close()
	if cfg.GroupType == "movie" {
		movie, _ := database.GetMovies(&database.Query{Where: "id = ?"}, cfg.MovieID)
		//defer logger.ClearVar(&movie)

		s, _ := structure.NewStructure(
			cfg.Cfg_Media,
			movie.Listname,
			cfg.GroupType,
			movie.Rootpath,
			config.Cfg.Media[cfg.Cfg_Media].DataImport[0].Template_path,
			config.Cfg.Media[cfg.Cfg_Media].Data[0].Template_path,
		)
		//defer s.Close()
		s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)

		s.ParseFileAdditional(cfg.FilePath, filepath.Dir(cfg.FilePath), false, 0, true)

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, movie.Rootpath, movie.DbmovieID, "", "", []database.Dbstatic_TwoInt{})
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename})
	} else {
		series, _ := database.GetSeries(&database.Query{Where: "id = ?"}, cfg.SerieID)
		//defer logger.ClearVar(&series)

		s, _ := structure.NewStructure(
			cfg.Cfg_Media,
			series.Listname,
			cfg.GroupType,
			series.Rootpath,
			config.Cfg.Media[cfg.Cfg_Media].DataImport[0].Template_path,
			config.Cfg.Media[cfg.Cfg_Media].Data[0].Template_path,
		)
		//defer s.Close()
		s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)
		s.ParseFileAdditional(cfg.FilePath, filepath.Dir(cfg.FilePath), false, 0, true)

		_, _, mapepi := s.GetSeriesEpisodes(series.ID, series.DbserieID, cfg.FilePath, filepath.Dir(cfg.FilePath))

		var firstdbepiid, firstepiid uint
		for key := range mapepi {
			firstdbepiid = uint(mapepi[key].Num2)
			firstepiid = uint(mapepi[key].Num1)
			break
		}

		serietitle, episodetitle := s.GetEpisodeTitle(firstdbepiid, cfg.FilePath)

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, series.Rootpath, firstepiid, serietitle, episodetitle, mapepi)
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename})
	}
}

type apiStructureJson struct {
	Folder                     string
	Disableruntimecheck        bool
	Disabledisallowed          bool
	Disabledeletewronglanguage bool
	Grouptype                  string
	Sourcepathtemplate         string
	Targetpathtemplate         string
	Configentry                string
	Forceid                    int
}

// @Summary      Structure Single Item
// @Description  Structure a single folder
// @Tags         general
// @Param        config  body      apiStructureJson  true  "Config"
// @Success      200     {object}  string
// @Failure      400     {object}  string
// @Failure      401     {object}  string
// @Router       /api/structure [post]
func apiStructure(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg apiStructureJson
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	getconfig := ""
	if strings.EqualFold(cfg.Grouptype, "movie") {
		getconfig = "movie_" + cfg.Configentry
	}
	if strings.EqualFold(cfg.Grouptype, "series") {
		getconfig = "serie_" + cfg.Configentry
	}
	//defer media.Close()
	if config.Cfg.Media[getconfig].Name != cfg.Configentry {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "media config not found"})
		return
	}

	if !config.ConfigCheck("path_" + cfg.Sourcepathtemplate) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "source config not found"})
		return
	}

	if !config.ConfigCheck("path_" + cfg.Targetpathtemplate) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "target config not found"})
		return
	}
	if cfg.Forceid != 0 {
		structure.StructureSingleFolderAs(cfg.Folder, cfg.Forceid, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Disabledeletewronglanguage, cfg.Grouptype, cfg.Sourcepathtemplate, cfg.Targetpathtemplate, getconfig)
	} else {
		structure.StructureSingleFolder(cfg.Folder, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Disabledeletewronglanguage, cfg.Grouptype, cfg.Sourcepathtemplate, cfg.Targetpathtemplate, getconfig)
	}
}
