package api

import (
	"encoding/json"
	"errors"
	"fmt"
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
	ctx.JSON(http.StatusOK, r)
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
	query := database.Query{}
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
	jobs, _ := database.QueryJobHistory(query)
	ctx.JSON(http.StatusOK, jobs)
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
	ctx.JSON(http.StatusOK, *token)

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
	dbmovies, _ := database.QueryDbmovie(database.Query{})
	for _, movie := range dbmovies {
		slug := logger.StringToSlug(movie.Title)
		database.UpdateColumn("dbmovies", "slug", slug, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie.ID}})
	}

	dbmoviestitles, _ := database.QueryDbmovieTitle(database.Query{})
	for _, movie := range dbmoviestitles {
		slug := logger.StringToSlug(movie.Title)
		database.UpdateColumn("dbmovie_titles", "slug", slug, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie.ID}})
	}

	dbserie, _ := database.QueryDbserie(database.Query{})
	for _, movie := range dbserie {
		slug := logger.StringToSlug(movie.Seriename)
		database.UpdateColumn("dbseries", "slug", slug, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie.ID}})
	}

	dbserietitles, _ := database.QueryDbserieAlternates(database.Query{})
	for _, movie := range dbserietitles {
		slug := logger.StringToSlug(movie.Title)
		database.UpdateColumn("dbserie_alternates", "slug", slug, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie.ID}})
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
	defer parse.Close()
	if getcfg.Typ == "movie" {
		parse.GetPriority("movie_"+getcfg.Config, getcfg.Quality)
		ctx.JSON(http.StatusOK, parse)
		return
	}
	if getcfg.Typ == "series" {
		parse.GetPriority("serie_"+getcfg.Config, getcfg.Quality)
		ctx.JSON(http.StatusOK, parse)
		return
	}
	ctx.JSON(http.StatusOK, parse)
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
	defer parse.Close()
	if getcfg.Typ == "movie" {
		parse.ParseVideoFile(getcfg.Path, "movie_"+getcfg.Config, getcfg.Quality)
		parse.GetPriority("movie_"+getcfg.Config, getcfg.Quality)
		ctx.JSON(http.StatusOK, parse)
		return
	}
	if getcfg.Typ == "series" {
		parse.ParseVideoFile(getcfg.Path, "serie_"+getcfg.Config, getcfg.Quality)
		parse.GetPriority("serie_"+getcfg.Config, getcfg.Quality)
		ctx.JSON(http.StatusOK, parse)
		return
	}
	ctx.JSON(http.StatusOK, parse)
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
	ctx.JSON(http.StatusOK, r)
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
	database.DB.Close()
	database.DBImdb.Close()
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
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")), cfg_general.MaxDatabaseBackups)
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
	database.ReadWriteMu.Lock()
	_, err := database.DB.Exec("DELETE FROM " + ctx.Param("name") + "; VACUUM;")
	database.ReadWriteMu.Unlock()
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
	database.ReadWriteMu.Lock()
	_, err := database.DB.Exec("VACUUM;")
	database.ReadWriteMu.Unlock()
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
				_, err := database.DeleteRow("job_histories", database.Query{Where: "created_at < ?", WhereArgs: []interface{}{scantime}})
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
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
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
	database.DeleteRow("qualities", database.Query{Where: "id = ?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.GetVars()
	parser.LoadDBPatterns()
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
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
	counter, _ := database.CountRows("qualities", database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{quality.ID}})

	if counter == 0 {
		database.InsertArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			[]interface{}{quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex})
	} else {
		database.UpdateArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			[]interface{}{quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex},
			database.Query{Where: "id != 0 and id = ?", WhereArgs: []interface{}{quality.ID}})
	}
	database.GetVars()
	parser.LoadDBPatterns()
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
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
	qual := config.ConfigGet("quality_" + ctx.Param("name")).Data.(config.QualityConfig)

	var parserreturn []parser.ParseInfo
	for idxreso := range database.Getresolutions {
		wantedreso := false

		for idxwantreso := range qual.Wanted_resolution {
			if strings.EqualFold(qual.Wanted_resolution[idxwantreso], database.Getresolutions[idxreso].Name) {
				wantedreso = true
				break
			}
		}
		if !wantedreso {
			continue
		}
		for idxqual := range database.Getqualities {
			wantedqual := false
			for idxwantqual := range qual.Wanted_quality {
				if strings.EqualFold(qual.Wanted_quality[idxwantqual], database.Getqualities[idxqual].Name) {
					wantedqual = true
					break
				}
			}
			if !wantedqual {
				continue
			}
			for idxcodec := range database.Getcodecs {
				for idxaudio := range database.Getaudios {
					parse := parser.ParseInfo{
						Resolution:   database.Getresolutions[idxreso].Name,
						Quality:      database.Getqualities[idxqual].Name,
						Codec:        database.Getcodecs[idxcodec].Name,
						Audio:        database.Getaudios[idxaudio].Name,
						ResolutionID: database.Getresolutions[idxreso].ID,
						QualityID:    database.Getqualities[idxqual].ID,
						CodecID:      database.Getcodecs[idxcodec].ID,
						AudioID:      database.Getaudios[idxaudio].ID,
					}
					parse.GetIDPriority(ctx.Param("config"), ctx.Param("name"))
					parserreturn = append(parserreturn, parse)
				}
			}
		}
	}
	ctx.JSON(http.StatusOK, parserreturn)
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
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
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
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
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
	ctx.JSON(http.StatusOK, config.ConfigGet(ctx.Param("name")).Data)
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
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
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
	config.LoadCfgDB(config.Configfile)
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
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
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
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
	configs := config.ConfigGetAll()
	list := make(map[string]interface{})
	for _, value := range configs {
		if strings.HasPrefix(value.Name, ctx.Param("type")) {
			list[value.Name] = value.Data
		}
	}
	ctx.JSON(http.StatusOK, list)
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

	if cfg.GroupType == "movie" {
		movie, _ := database.GetMovies(database.Query{Where: "id = ?", WhereArgs: []interface{}{cfg.MovieID}})

		s, _ := structure.NewStructure(
			cfg.Cfg_Media,
			movie.Listname,
			cfg.GroupType,
			movie.Rootpath,
			"",
			"",
		)
		defer s.Close()
		s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)

		s.ParseFileAdditional(cfg.FilePath, filepath.Dir(cfg.FilePath), false, 0)

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, movie.Rootpath, movie.DbmovieID, "", "", []int{})
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename})
	} else {
		series, _ := database.GetSeries(database.Query{Where: "id = ?", WhereArgs: []interface{}{cfg.SerieID}})

		s, _ := structure.NewStructure(
			cfg.Cfg_Media,
			series.Listname,
			cfg.GroupType,
			series.Rootpath,
			"",
			"",
		)
		defer s.Close()
		s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)
		s.ParseFileAdditional(cfg.FilePath, filepath.Dir(cfg.FilePath), false, 0)

		_, episodes, _, serietitle, episodetitle, seriesEpisode, _, _ := s.GetSeriesEpisodes(series.ID, series.DbserieID, cfg.FilePath, filepath.Dir(cfg.FilePath))

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, series.Rootpath, seriesEpisode.ID, serietitle, episodetitle, episodes)
		defer logger.ClearVar(&episodes)
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
	media := config.ConfigGet(getconfig).Data.(config.MediaTypeConfig)
	if media.Name != cfg.Configentry {
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
		structure.StructureSingleFolderAs(cfg.Folder, cfg.Forceid, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Disabledeletewronglanguage, cfg.Grouptype, "path_"+cfg.Sourcepathtemplate, "path_"+cfg.Targetpathtemplate, getconfig)
	} else {
		structure.StructureSingleFolder(cfg.Folder, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Disabledeletewronglanguage, cfg.Grouptype, "path_"+cfg.Sourcepathtemplate, "path_"+cfg.Targetpathtemplate, getconfig)
	}
}
