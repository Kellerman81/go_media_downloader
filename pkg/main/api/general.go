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

	// "github.com/goccy/go-json".

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
)

func AddGeneralRoutes(routerapi *gin.RouterGroup) {
	routerapi.Use(checkauth)
	{
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
		routerapi.DELETE("/db/clearcache", apiDBClearCache)
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
}

type apiparse struct {
	Name    string
	Typ     string
	Path    string
	Config  string
	Quality string
	Year    bool
}

// @Summary      Debug information
// @Description  Shows some stats
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object} Statsresults
// @Failure      401  {object}  Jsonerror
// @Router       /api/debugstats [get].
func apiDebugStats(ctx *gin.Context) {
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
	ctx.JSON(http.StatusOK, gin.H{
		"GC Stats":     string(gcjson),
		"Mem Stats":    string(memjson),
		"GOOS":         runtime.GOOS,
		"NumCPU":       runtime.NumCPU(),
		"NumGoroutine": runtime.NumGoroutine(),
		"GOARCH":       runtime.GOARCH,
		"WorkerStats":  worker.GetStats(),
	})
}

type Statsresults struct {
	GCStats      string       `json:"GC Stats"`
	MemStats     string       `json:"Mem Stats"`
	GOOS         string       `json:"GOOS"`
	GOARCH       string       `json:"GOARCH"`
	NumCPU       int          `json:"NumCPU"`
	NumGoroutine int          `json:"NumGoroutine"`
	WorkerStats  worker.Stats `json:"WorkerStats"`
}

// @Summary      Queue
// @Description  Lists Queued and Started Jobs (but not finished)
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]worker.Job}
// @Failure      401  {object}  Jsonerror
// @Router       /api/queue [get].
func apiQueueList(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": worker.GetQueues()})
}

// @Summary      Queue History
// @Description  Lists Started Jobs and finished but not queued jobs
// @Tags         general
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondata{data=[]database.JobHistory}
// @Failure      401    {object}  Jsonerror
// @Router       /api/queue/history [get].
func apiQueueListStarted(ctx *gin.Context) {
	var query database.Querywithargs
	var limit, page int
	query.OrderBy = "ID desc"
	query.Limit = 100
	if queryParam, ok := ctx.GetQuery("limit"); ok {
		if queryParam != "" {
			limit, _ = strconv.Atoi(queryParam)
			query.Limit = uint(limit)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/trakt/authorize [get].
func apiTraktGetAuthURL(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, apiexternal.GetTraktAuthURL())
}

// @Summary      Trakt Save Token
// @Description  Saves Trakt token after Authorization
// @Tags         general
// @Param        code  path      string  true  "code"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondata{data=any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/trakt/token/{code} [get].
func apiTraktGetStoreToken(ctx *gin.Context) {
	apiexternal.SetTraktToken(apiexternal.GetTraktAuthToken(ctx.Param("code")))

	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})
	ctx.JSON(http.StatusOK, gin.H{"data": apiexternal.GetTraktToken()})
}

// @Summary      Trakt Get List (Auth Test)
// @Description  Get User List
// @Tags         general
// @Param        user  path      string  true  "Trakt Username"
// @Param        list  path      string  true  "List Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondataerror{data=[]apiexternal.TraktUserList}
// @Failure      401   {object}  Jsonerror
// @Router       /api/trakt/user/{user}/{list} [get].
func apiTraktGetUserList(ctx *gin.Context) {
	lim := "10"
	list, err := apiexternal.GetTraktUserList(
		ctx.Param("user"),
		ctx.Param("list"),
		"movie,show",
		&lim,
	)
	ctx.JSON(http.StatusOK, gin.H{"data": list, "error": err})
}

// @Summary      Refresh Slugs
// @Description  Regenerates Slugs
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/slug [get].
func apiDBRefreshSlugs(ctx *gin.Context) {
	dbmovies := database.QueryDbmovie(database.Querywithargs{})
	var slug string
	for idx := range dbmovies {
		slug = logger.StringToSlug(dbmovies[idx].Title)
		database.ExecN("update dbmovies set slug = ? where id = ?", slug, &dbmovies[idx].ID)
	}

	dbmoviestitles := database.QueryDbmovieTitle(database.Querywithargs{})
	for idx := range dbmoviestitles {
		slug = logger.StringToSlug(dbmoviestitles[idx].Title)
		database.ExecN(
			"update dbmovie_titles set slug = ? where id = ?",
			slug,
			&dbmoviestitles[idx].ID,
		)
	}

	dbserie := database.QueryDbserie(database.Querywithargs{})
	for idx := range dbserie {
		slug = logger.StringToSlug(dbserie[idx].Seriename)
		database.ExecN("update dbseries set slug = ? where id = ?", slug, &dbserie[idx].ID)
	}

	dbserietitles := database.QueryDbserieAlternates(database.Querywithargs{})
	for idx := range dbserietitles {
		slug = logger.StringToSlug(dbserietitles[idx].Title)
		database.ExecN(
			"update dbserie_alternates set slug = ? where id = ?",
			slug,
			&dbserietitles[idx].ID,
		)
	}
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Parse a string
// @Description  Parses a string for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  Jsondataerror{data=database.ParseInfo}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/parse/string [post].
func apiParseString(ctx *gin.Context) {
	var getcfg apiparse
	var err error
	if err = ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var cfgv string
	if getcfg.Typ == logger.StrMovie {
		cfgv = "movie_" + getcfg.Config
	} else {
		cfgv = "serie_" + getcfg.Config
	}
	cfgp := config.GetSettingsMedia(cfgv)
	parse := parser.ParseFile(getcfg.Name, false, false, cfgp, -1)
	// parse := parser.NewFileParser(getcfg.Name, cfgp, false, -1)
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)
	err = parser.GetDBIDs(parse, cfgp, true)
	ctx.JSON(http.StatusOK, gin.H{"data": parse, "error": err})
	parse.Close()
}

// @Summary      Parse a file
// @Description  Parses a file for testing
// @Tags         parse
// @Param        toparse  body      apiparse  true  "To Parse"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}  Jsondata{data=database.ParseInfo}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/parse/file [post].
func apiParseFile(ctx *gin.Context) {
	var getcfg apiparse
	var err error
	if err = ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var cfgv string
	if getcfg.Typ == logger.StrMovie {
		cfgv = "movie_" + getcfg.Config
	} else {
		cfgv = "serie_" + getcfg.Config
	}
	cfgp := config.GetSettingsMedia(cfgv)
	// defer parse.Close()
	parse := parser.ParseFile(getcfg.Path, true, false, cfgp, -1)
	// parse := parser.NewFileParser(filepath.Base(getcfg.Path), cfgp, false, -1)
	parse.File = getcfg.Path
	parser.ParseVideoFile(parse, config.GetSettingsQuality(getcfg.Quality))
	parser.GetPriorityMapQual(parse, cfgp, config.GetSettingsQuality(getcfg.Quality), true, true)
	parser.GetDBIDs(parse, cfgp, true)
	ctx.JSON(http.StatusOK, gin.H{"data": parse})
	parse.Close()
}

// @Summary      Generate IMDB Cache
// @Description  Downloads IMDB Dataset and creates a new database from it
// @Tags         general
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/fillimdb [get].
func apiFillImdb(ctx *gin.Context) {
	config.GetSettingsGeneral().Jobs["RefreshImdb"](0)
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Stop Scheduler
// @Description  Stops all Schedulers
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/stop [get].
func apiSchedulerStop(c *gin.Context) {
	// scheduler.QueueData.Stop()
	// scheduler.QueueFeeds.Stop()
	// scheduler.QueueSearch.Stop()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Start Scheduler
// @Description  Start all Schedulers
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/start [get].
func apiSchedulerStart(c *gin.Context) {
	// scheduler.QueueData.Start()
	// scheduler.QueueFeeds.Start()
	// scheduler.QueueSearch.Start()
	c.JSON(http.StatusOK, "ok")
}

// @Summary      Scheduler Jobs
// @Description  Lists Planned Jobs
// @Tags         scheduler
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  Jsondata{data=map[string]worker.jobSchedule}
// @Failure      401  {object}  Jsonerror
// @Router       /api/scheduler/list [get].
func apiSchedulerList(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": worker.GetSchedules()})
}

// @Summary      Close DB
// @Description  Closes all database connections
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/close [get].
func apiDBClose(ctx *gin.Context) {
	database.DBClose()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Backup DB
// @Description  Saves DB
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/backup [get].
func apiDBBackup(ctx *gin.Context) {
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		worker.StopCronWorker()
		worker.CloseWorkerPools()
	}
	backupto := "./backup/data.db." + database.GetVersion() + "." + time.Now().
		Format("20060102_150405")
	database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
	if config.GetSettingsGeneral().DatabaseBackupStopTasks {
		worker.InitWorkerPools(
			config.GetSettingsGeneral().WorkerSearch,
			config.GetSettingsGeneral().WorkerFiles,
			config.GetSettingsGeneral().WorkerMetadata,
			config.GetSettingsGeneral().WorkerRSS,
			config.GetSettingsGeneral().WorkerIndexer,
		)
		worker.StartCronWorker()
	}
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Integrity DB
// @Description  Integrity Check DB
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/integrity [get].
func apiDBIntegrity(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, database.DBIntegrityCheck())
}

// @Summary      Clear DB Table
// @Description  Clears a DB Table
// @Tags         database
// @Param        name  path      string  true  "Table Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/clear/{name} [delete].
func apiDBClear(ctx *gin.Context) {
	err := database.ExecNErr("DELETE from " + ctx.Param("name"))
	database.ExecN("VACUUM")
	if err == nil {
		ctx.JSON(http.StatusOK, "ok")
	} else {
		ctx.JSON(http.StatusForbidden, err)
	}
}

// @Summary      Clear Caches
// @Description  Clears Caches
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/clearcache [delete].
func apiDBClearCache(ctx *gin.Context) {
	database.ClearCaches()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary      Vacuum DB
// @Description  Vacuum database
// @Tags         database
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
// @Router       /api/db/vacuum [get].
func apiDBVacuum(ctx *gin.Context) {
	err := database.ExecNErr("VACUUM")
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
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns ok"
// @Failure      401   {object}  Jsonerror
// @Router       /api/db/oldjobs [delete].
func apiDBRemoveOldJobs(ctx *gin.Context) {
	if queryParam, ok := ctx.GetQuery("days"); ok {
		if queryParam != "" {
			days, _ := strconv.Atoi(queryParam)

			scantime := time.Now()
			if days != 0 {
				scantime = scantime.AddDate(0, 0, 0-days)
				_, err := database.DeleteRow("job_histories", "created_at < ?", scantime)
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
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}   Jsondata{data=[]database.Qualities}
// @Failure      401  {object}  Jsonerror
// @Router       /api/quality [get].
func apiGetQualities(ctx *gin.Context) {
	ctx.JSON(
		http.StatusOK,
		gin.H{
			"data": database.StructscanT[database.Qualities](
				false,
				database.Getdatarow[uint](false, "select count() from qualities"),
				"select * from qualities",
			),
		},
	)
}

// @Summary      Delete Quality
// @Description  Deletes a quality
// @Tags         quality
// @Param        id   path      string  true  "Id of Quality to delete"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}   Jsondata{data=[]database.Qualities}
// @Failure      401  {object}  Jsonerror
// @Router       /api/quality/{id} [delete].
func apiQualityDelete(ctx *gin.Context) {
	database.DeleteRow("qualities", logger.FilterByID, ctx.Param("id"))
	database.SetVars()
	ctx.JSON(
		http.StatusOK,
		gin.H{
			"data": database.StructscanT[database.Qualities](
				false,
				database.Getdatarow[uint](false, "select count() from qualities"),
				"select * from qualities",
			),
		},
	)
}

// @Summary      Update Quality
// @Description  Updates or adds a quality
// @Tags         quality
// @Param        quality  body      database.Qualities  true  "Quality"
// @Param        apikey query     string    true  "apikey"
// @Success      200      {object}   Jsondata{data=[]database.Qualities}
// @Failure      400      {object}  Jsonerror
// @Failure      401      {object}  Jsonerror
// @Router       /api/quality [post].
func apiQualityUpdate(ctx *gin.Context) {
	var quality database.Qualities
	if err := ctx.ShouldBindJSON(&quality); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter := database.Getdatarow[uint](
		false,
		"select count() from qualities where id != 0 and id = ?",
		&quality.ID,
	)

	if counter == 0 {
		database.InsertArray(
			"qualities",
			[]string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			quality.QualityType,
			quality.Name,
			quality.Regex,
			quality.Strings,
			quality.Priority,
			quality.UseRegex,
		)
	} else {
		database.UpdateArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			"id != 0 and id = ?", quality.QualityType, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex, quality.ID)
	}
	database.SetVars()
	ctx.JSON(
		http.StatusOK,
		gin.H{
			"data": database.StructscanT[database.Qualities](
				false,
				database.Getdatarow[uint](false, "select count() from qualities"),
				"select * from qualities",
			),
		},
	)
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        name    path      string  true  "Quality Name: ex. SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/get/{name} [get].
func apiListQualityPriorities(ctx *gin.Context) {
	if !config.CheckGroup("quality_", ctx.Param("name")) {
		ctx.JSON(http.StatusNotFound, "quality not found")
		return
	}
	returnprios := make([]parser.Prioarr, 0, 1000)
	for _, prio := range parser.Getallprios() {
		if prio.QualityGroup == ctx.Param("name") {
			returnprios = append(returnprios, prio)
		}
	}
	// ctx.JSON(http.StatusOK, gin.H{"data": parser.Getallprios()[ctx.Param("name")]})
	ctx.JSON(http.StatusOK, gin.H{"data": returnprios})
}

// @Summary      List Quality Priorities
// @Description  List allowed qualities and their priorities
// @Tags         quality
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/all [get].
func apiListAllQualityPriorities(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": parser.Getallprios()})
}

// @Summary      List Quality Priorities
// @Description  List all qualities and their priorities
// @Tags         quality
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}   Jsondata{data=[]parser.Prioarr}
// @Failure      401  {object}  Jsonerror
// @Failure      404     {object}  string
// @Router       /api/quality/complete [get].
func apiListCompleteAllQualityPriorities(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": parser.Getcompleteallprios()})
}

// @Summary      Get Complete Config
// @Description  Get All Config Parameters
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401  {object}  Jsonerror
// @Router       /api/config/all [get].
func apiConfigAll(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Clear Config
// @Description  Clears the configuration and sets some examples -> Use with caution
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/clear [delete].
func apiConfigClear(ctx *gin.Context) {
	config.ClearCfg()
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Get Config
// @Description  Gets a configuration
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  Jsondata{data=any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/get/{name} [get].
func apiConfigGet(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()[ctx.Param("name")]})
}

// @Summary      Delete Config
// @Description  Deletes a configuration -> Use with caution -> also resets your comments
// @Tags         config
// @Param        name  path      string  true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {array}  Jsondata{data=map[string]any}
// @Failure      401  {object}  Jsonerror
// @Router       /api/config/delete/{name} [delete].
func apiConfigDelete(ctx *gin.Context) {
	config.DeleteCfgEntry(ctx.Param("name"))
	config.WriteCfg()
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Reload ConfigFile
// @Description  Refreshes the config from the file
// @Tags         config
// @Param        apikey query     string    true  "apikey"
// @Success      200  {array}  Jsondata{data=map[string]any}
// @Failure      401   {object}  Jsonerror
// @Router       /api/config/refresh [get].
func apiConfigRefreshFile(ctx *gin.Context) {
	config.LoadCfgDB(true)
	ctx.JSON(http.StatusOK, gin.H{"data": config.GetCfgAll()})
}

// @Summary      Update Config
// @Description  Updates a configuration -> Use with caution -> also resets your comments
// @Tags         config
// @Param        config  body      any  true  "Config"
// @Param        name    path      string       true  "Type Name: ex. quality_SD"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {array}  Jsondata{data=map[string]any}
// @Failure      401     {object}  Jsonerror
// @Failure      400  {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/config/update/{name} [post].
func apiConfigUpdate(ctx *gin.Context) {
	name := ctx.Param("name")
	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}
	switch left {
	case "general":
		var getcfg config.GeneralConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "downloader":
		var getcfg config.DownloaderConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case logger.StrImdb:
		var getcfg config.ImdbConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "indexer":
		var getcfg config.IndexersConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "list":
		var getcfg config.ListsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "serie":
	case "movie":
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "notification":
		var getcfg config.NotificationConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "path":
		var getcfg config.PathsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "quality":
		var getcfg config.QualityConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "regex":
		var getcfg config.RegexConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		config.UpdateCfgEntry(config.Conf{Name: ctx.Param("name"), Data: getcfg})
	case "scheduler":
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
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  Jsondata{data=map[string]any}
// @Failure      401     {object}  Jsonerror
// @Router       /api/config/type/{type} [get].
func apiListConfigType(ctx *gin.Context) {
	list := make(map[string]any)
	name := ctx.Param("type")
	left, right := logger.SplitByLR(name, '_')
	if left == "" {
		left = right
	}
	switch left {
	case "general":
		list["general"] = config.GetSettingsGeneral()
	case logger.StrImdb:
		list[logger.StrImdb] = config.GetSettingsImdb()
	case "downloader":
		config.RangeSettingsDownloader(func(key string, cfgdata *config.DownloaderConfig) {
			if strings.HasPrefix(key, right) {
				list["downloader_"+key] = cfgdata
			}
		})
	case "indexer":
		config.RangeSettingsIndexer(func(key string, cfgdata *config.IndexersConfig) {
			if strings.HasPrefix(key, right) {
				list["indexer_"+key] = cfgdata
			}
		})
	case "list":
		config.RangeSettingsList(func(key string, cfgdata *config.ListsConfig) {
			if strings.HasPrefix(key, right) {
				list["list_"+key] = cfgdata
			}
		})
	case logger.StrSerie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) {
			if strings.HasPrefix(key, right) {
				list["serie_"+key] = cfgdata
			}
		})
	case logger.StrMovie:
		config.RangeSettingsMedia(func(key string, cfgdata *config.MediaTypeConfig) {
			if strings.HasPrefix(key, right) {
				list["movie_"+key] = cfgdata
			}
		})
	case "notification":
		config.RangeSettingsNotification(func(key string, cfgdata *config.NotificationConfig) {
			if strings.HasPrefix(key, right) {
				list["notification_"+key] = cfgdata
			}
		})
	case "path":
		config.RangeSettingsPath(func(key string, cfgdata *config.PathsConfig) {
			if strings.HasPrefix(key, right) {
				list["path_"+key] = cfgdata
			}
		})
	case "quality":
		config.RangeSettingsQuality(func(key string, cfgdata *config.QualityConfig) {
			if strings.HasPrefix(key, right) {
				list["quality_"+key] = cfgdata
			}
		})
	case "regex":
		config.RangeSettingsRegex(func(key string, cfgdata *config.RegexConfig) {
			if strings.HasPrefix(key, right) {
				list["regex_"+key] = cfgdata
			}
		})
	case "scheduler":
		config.RangeSettingsScheduler(func(key string, cfgdata *config.SchedulerConfig) {
			if strings.HasPrefix(key, right) {
				list["scheduler_"+key] = cfgdata
			}
		})
	}

	ctx.JSON(http.StatusOK, gin.H{"data": list})
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
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  JSONNaming
// @Failure      400     {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/naming [post].
func apiNamingGenerate(ctx *gin.Context) {
	var cfg apiNameInput
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// defer mediacfg.Close()
	if cfg.GroupType == logger.StrMovie {
		movie, _ := database.GetMovies(
			database.Querywithargs{Where: logger.FilterByID},
			cfg.MovieID,
		)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath, false, false, 0)
		// defer s.Close()
		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata
		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = movie.Rootpath
		m := parser.ParseFile(
			cfg.FilePath,
			true,
			true,
			cfgp,
			cfgp.GetMediaListsEntryListID(movie.Listname),
		)
		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)

		s.GenerateNamingTemplate(&orgadata2, m, &movie.DbmovieID)
		ctx.JSON(
			http.StatusOK,
			gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m},
		)
	} else {
		series, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, cfg.SerieID)
		// defer logger.ClearVar(&series)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)

		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath,
			false, false, 0,
		)
		// defer s.Close()
		to := filepath.Dir(cfg.FilePath)
		var orgadata2 structure.Organizerdata
		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = series.Rootpath

		m := parser.ParseFile(cfg.FilePath, true, true, cfgp, cfgp.GetMediaListsEntryListID(series.Listname))
		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)
		m.SerieID = series.ID
		m.DbserieID = series.DbserieID
		s.GetSeriesEpisodes(&orgadata2, m, true, s.Cfgp.Lists[orgadata2.Listid].CfgQuality)

		var firstepiid uint
		for _, entry := range m.Episodes {
			firstepiid = entry.Num1
			break
		}

		s.GenerateNamingTemplate(&orgadata2, m, &firstepiid)
		ctx.JSON(http.StatusOK, gin.H{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m})
	}
}

type apiStructureJSON struct {
	Folder                     string
	Grouptype                  string
	Sourcepathtemplate         string
	Targetpathtemplate         string
	Configentry                string
	Forceid                    uint
	Disableruntimecheck        bool
	Disabledisallowed          bool
	Disabledeletewronglanguage bool
}

// @Summary      Structure Single Item
// @Description  Structure a single folder
// @Tags         general
// @Param        config  body      apiStructureJSON  true  "Config"
// @Param        apikey query     string    true  "apikey"
// @Success      200     {object}  string
// @Failure      400     {object}  Jsonerror
// @Failure      401     {object}  Jsonerror
// @Router       /api/structure [post].
func apiStructure(ctx *gin.Context) {
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
	// defer media.Close()
	if config.GetSettingsMedia(getconfig).Name != cfg.Configentry {
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
	if !scanner.CheckFileExist(cfg.Folder) {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "folder not found"})
		return
	}
	cfgp := config.GetSettingsMedia(getconfig)

	var cfgimport *config.MediaDataImportConfig
	for _, imp := range cfgp.DataImport {
		if strings.EqualFold(imp.TemplatePath, cfg.Sourcepathtemplate) {
			cfgimport = &imp
			break
		}
	}

	checkruntime := config.GetSettingsPath(cfg.Sourcepathtemplate).CheckRuntime
	if cfg.Disableruntimecheck {
		checkruntime = false
	}
	deletewronglanguage := config.GetSettingsPath(cfg.Sourcepathtemplate).DeleteWrongLanguage
	if cfg.Disabledeletewronglanguage {
		deletewronglanguage = false
	}
	// structurevar := structure.NewStructure(cfgp, cfgimport, cfg.Sourcepathtemplate, cfg.Targetpathtemplate, false, false, 0)

	// structurevar.ManualId = cfg.Forceid
	structure.OrganizeSingleFolder(
		ctx,
		cfg.Folder,
		cfgp,
		cfgimport,
		cfg.Targetpathtemplate,
		checkruntime,
		deletewronglanguage,
		cfg.Forceid,
	)

	ctx.JSON(http.StatusOK, gin.H{})
}
