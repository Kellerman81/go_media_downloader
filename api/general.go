package api

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/scheduler"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
	gin "github.com/gin-gonic/gin"
)

type apiparse struct {
	Name    string
	Year    bool
	Typ     string
	Path    string
	Config  string
	Quality string
}

// @Summary Queue
// @Description Lists Queued and Started Jobs (but not finished)
// @Tags general
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]tasks.Job
// @Failure 401 {object} string
// @Router /api/queue [get]
func ApiQueueList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}

	var r []tasks.Job
	for _, value := range tasks.GlobalQueue.Queue {
		r = append(r, value)
	}
	ctx.JSON(http.StatusOK, r)
}

// @Summary Queue History
// @Description Lists Started Jobs and finished but not queued jobs
// @Tags general
// @Accept  json
// @Produce  json
// @Param limit query int false "Limit"
// @Param page query int false "Page"
// @Param order query string false "Order By"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.JobHistoryJson
// @Failure 401 {object} string
// @Router /api/queue/history [get]
func ApiQueueListStarted(ctx *gin.Context) {
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

// @Summary Trakt Auhtorize
// @Description List Qualities with regex filters
// @Tags general
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/trakt/authorize [get]
func ApiTraktGetAuthUrl(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, apiexternal.TraktApi.GetAuthUrl())
}

// @Summary Trakt Save Token
// @Description Saves Trakt token after Authorization
// @Tags general
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/trakt/token/{code} [get]
func ApiTraktGetStoreToken(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	token := apiexternal.TraktApi.GetAuthToken(ctx.Param("code"))
	apiexternal.TraktApi.Token = token
	configs := config.ConfigGetAll()
	configs["trakt_token"] = *token
	config.UpdateCfg(configs)
	config.ConfigDB.Set("trakt_token", *token)
	ctx.JSON(http.StatusOK, *token)
}

// @Summary Trakt Get List (Auth Test)
// @Description Get User List
// @Tags general
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/trakt/user/{user}/{list} [get]
func ApiTraktGetUserList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	list, err := apiexternal.TraktApi.GetUserList(ctx.Param("user"), ctx.Param("list"), "movie,show", 10)
	ctx.JSON(http.StatusOK, gin.H{"list": list, "error": err})
}

// @Summary Parse a string
// @Description Parses a string for testing
// @Tags parse
// @Accept  json
// @Produce  json
// @Param toparse body apiparse true "To Parse"
// @Param apikey query string true "apikey"
// @Success 200 {object} utils.ParseInfo
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/parse/string [post]
func ApiParseString(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse, _ := utils.NewFileParser(getcfg.Name, getcfg.Year, getcfg.Typ)
	if getcfg.Typ == "movie" {
		var typcfg config.MediaTypeConfig
		config.ConfigGet("movie_"+getcfg.Config, &typcfg)
		var qualcfg config.QualityConfig
		config.ConfigGet("quality_"+getcfg.Quality, &qualcfg)
		parse.GetPriority(typcfg, qualcfg)
	}
	if getcfg.Typ == "series" {
		var typcfg config.MediaTypeConfig
		config.ConfigGet("serie_"+getcfg.Config, &typcfg)
		var qualcfg config.QualityConfig
		config.ConfigGet("quality_"+getcfg.Quality, &qualcfg)
		parse.GetPriority(typcfg, qualcfg)
	}
	ctx.JSON(http.StatusOK, parse)
}

// @Summary Parse a file
// @Description Parses a file for testing
// @Tags parse
// @Accept  json
// @Produce  json
// @Param toparse body apiparse true "To Parse"
// @Param apikey query string true "apikey"
// @Success 200 {object} utils.ParseInfo
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/parse/file [post]
func ApiParseFile(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var getcfg apiparse
	if err := ctx.ShouldBindJSON(&getcfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	parse, _ := utils.NewFileParser(filepath.Base(getcfg.Path), getcfg.Year, getcfg.Typ)
	if getcfg.Typ == "movie" {
		var typcfg config.MediaTypeConfig
		config.ConfigGet("movie_"+getcfg.Config, &typcfg)
		var qualcfg config.QualityConfig
		config.ConfigGet("quality_"+getcfg.Quality, &qualcfg)

		parse.ParseVideoFile(getcfg.Path, typcfg, qualcfg)
		parse.GetPriority(typcfg, qualcfg)
	}
	if getcfg.Typ == "series" {
		var typcfg config.MediaTypeConfig
		config.ConfigGet("serie_"+getcfg.Config, &typcfg)
		var qualcfg config.QualityConfig
		config.ConfigGet("quality_"+getcfg.Quality, &qualcfg)

		parse.ParseVideoFile(getcfg.Path, typcfg, qualcfg)
		parse.GetPriority(typcfg, qualcfg)
	}
	ctx.JSON(http.StatusOK, parse)
}

// @Summary Generate IMDB Cache
// @Description Downloads IMDB Dataset and creates a new database from it
// @Tags general
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/fillimdb [get]
func ApiFillImdb(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	go utils.InitFillImdb()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Stop Scheduler
// @Description Stops all Schedulers
// @Tags scheduler
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/scheduler/stop [get]
func ApiSchedulerStop(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueData.Stop()
	scheduler.QueueFeeds.Stop()
	scheduler.QueueSearch.Stop()
	c.JSON(http.StatusOK, "ok")
}

// @Summary Start Scheduler
// @Description Start all Schedulers
// @Tags scheduler
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/scheduler/start [get]
func ApiSchedulerStart(c *gin.Context) {
	if ApiAuth(c) == http.StatusUnauthorized {
		return
	}
	scheduler.QueueData.Start()
	scheduler.QueueFeeds.Start()
	scheduler.QueueSearch.Start()
	c.JSON(http.StatusOK, "ok")
}

// @Summary Scheduler Jobs
// @Description Lists Planned Jobs
// @Tags scheduler
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {object} string
// @Failure 401 {object} string
// @Router /api/scheduler/list [get]
func ApiSchedulerList(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var r []tasks.JobSchedule
	for _, value := range tasks.GlobalSchedules.Schedule {
		r = append(r, value)
	}
	ctx.JSON(http.StatusOK, r)
}

// @Summary Close DB
// @Description Closes all database connections
// @Tags database
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/db/close [get]
func ApiDbClose(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DB.Close()
	database.DBImdb.Close()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Clear DB Table
// @Description Clears a DB Table
// @Tags database
// @Accept  json
// @Produce  json
// @Param name path string true "Table Name"
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/db/clear/{name} [delete]
func ApiDbClear(ctx *gin.Context) {
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

// @Summary Vacuum DB
// @Description Vacuum database
// @Tags database
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/db/vacuum [get]
func ApiDbVacuum(ctx *gin.Context) {
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

// @Summary Clean Old Jobs
// @Description Removes Jobs started over x days ago from db
// @Tags database
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Param days query int true "Days ago"
// @Success 200 {string} string
// @Failure 401 {object} string
// @Router /api/db/oldjobs [delete]
func ApiDbRemoveOldJobs(ctx *gin.Context) {
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

// @Summary List Qualities
// @Description List Qualities with regex filters
// @Tags quality
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {array} database.Qualities
// @Failure 401 {object} string
// @Router /api/quality [get]
func ApiGetQualities(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
}

// @Summary Delete Quality
// @Description Deletes a quality
// @Tags quality
// @Accept  json
// @Produce  json
// @Param id path string true "Id of Quality to delete"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.Qualities
// @Failure 401 {object} string
// @Router /api/quality/{id} [delete]
func ApiQualityDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("qualities", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.GetVars()
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
}

// @Summary Update Quality
// @Description Updates or adds a quality
// @Tags quality
// @Accept  json
// @Produce  json
// @Param quality body database.Qualities true "Quality"
// @Param apikey query string true "apikey"
// @Success 200 {array} database.Qualities
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/quality [post]
func ApiQualityUpdate(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var quality database.Qualities
	if err := ctx.ShouldBindJSON(&quality); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	counter, _ := database.CountRows("qualities", database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{quality.ID}})

	if counter == 0 {
		database.InsertArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			[]interface{}{quality.Type, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex})
	} else {
		database.UpdateArray("qualities", []string{"Type", "Name", "Regex", "Strings", "Priority", "Use_Regex"},
			[]interface{}{quality.Type, quality.Name, quality.Regex, quality.Strings, quality.Priority, quality.UseRegex},
			database.Query{Where: "id != 0 and id=?", WhereArgs: []interface{}{quality.ID}})
	}
	database.GetVars()
	qualities, _ := database.QueryQualities(database.Query{})
	ctx.JSON(http.StatusOK, qualities)
}

// @Summary List Quality Priorities
// @Description List allowed qualities and their priorities
// @Tags quality
// @Accept  json
// @Produce  json
// @Param name path string true "Quality Name: ex. quality_SD"
// @Param config path string true "Config Name: ex. movie_EN or serie_EN"
// @Param apikey query string true "apikey"
// @Success 200 {array} utils.ParseInfo
// @Failure 401 {object} string
// @Failure 404 {object} string
// @Router /api/quality/{name}/{config} [get]
func ApiListQualityPriorities(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	if !config.ConfigCheck(ctx.Param("name")) {
		ctx.JSON(http.StatusNotFound, "quality not found")
		return
	}
	if !config.ConfigCheck(ctx.Param("config")) {
		ctx.JSON(http.StatusNotFound, "config not found")
		return
	}
	var qual config.QualityConfig
	config.ConfigGet(ctx.Param("name"), &qual)
	var media config.MediaTypeConfig
	config.ConfigGet(ctx.Param("config"), &media)
	var parser []utils.ParseInfo
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
					parse := utils.ParseInfo{
						Resolution:   database.Getresolutions[idxreso].Name,
						Quality:      database.Getqualities[idxqual].Name,
						Codec:        database.Getcodecs[idxcodec].Name,
						Audio:        database.Getaudios[idxaudio].Name,
						ResolutionID: database.Getresolutions[idxreso].ID,
						QualityID:    database.Getqualities[idxqual].ID,
						CodecID:      database.Getcodecs[idxcodec].ID,
						AudioID:      database.Getaudios[idxaudio].ID,
					}
					parse.GetIDPriority(media, qual)
					parser = append(parser, parse)
				}
			}
		}
	}
	ctx.JSON(http.StatusOK, parser)
}

// @Summary Get Complete Config
// @Description Get All Config Parameters
// @Tags config
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} string
// @Router /api/config/all [get]
func ApiConfigAll(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
}

// @Summary Clear Config
// @Description Clears the configuration and sets some examples
// @Tags config
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} string
// @Router /api/config/clear [delete]
func ApiConfigClear(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	keys, _ := config.ConfigDB.Keys([]byte("*"), 0, 0, true)
	for _, idx := range keys {
		config.ConfigDB.Delete(string(idx))
	}
	config.ClearCfg()
	config.WriteCfg()
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
}

// @Summary Get Config
// @Description Gets a configuration
// @Tags config
// @Accept  json
// @Produce  json
// @Param name path string true "Type Name: ex. quality_SD"
// @Param apikey query string true "apikey"
// @Success 200 {object} interface{}
// @Failure 401 {object} string
// @Router /api/config/get/{name} [get]
func ApiConfigGet(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	configs := config.ConfigGetAll()
	ctx.JSON(http.StatusOK, configs[ctx.Param("name")])
}

// @Summary Delete Config
// @Description Deletes a configuration
// @Tags config
// @Accept  json
// @Produce  json
// @Param name path string true "Type Name: ex. quality_SD"
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} string
// @Router /api/config/delete/{name} [delete]
func ApiConfigDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	configs := config.ConfigGetAll()
	config.ConfigDB.Delete(ctx.Param("name"))
	delete(configs, ctx.Param("name"))
	config.UpdateCfg(configs)
	config.WriteCfg()
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
}

// @Summary Update Config
// @Description Updates a configuration
// @Tags config
// @Accept  json
// @Produce  json
// @Param config body interface{} true "Config"
// @Param name path string true "Type Name: ex. quality_SD"
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/config/update/{name} [post]
func ApiConfigUpdate(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	name := ctx.Param("name")
	configs := config.ConfigGetAll()
	if strings.HasPrefix(name, "general") {
		var getcfg config.GeneralConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "downloader_") {
		var getcfg config.DownloaderConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "imdb") {
		var getcfg config.ImdbConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "indexer") {
		var getcfg config.IndexersConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "list") {
		var getcfg config.ListsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "serie") {
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "movie") {
		var getcfg config.MediaTypeConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "notification") {
		var getcfg config.NotificationConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "path") {
		var getcfg config.PathsConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "quality") {
		var getcfg config.QualityConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "regex") {
		var getcfg config.RegexConfigIn
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	if strings.HasPrefix(name, "scheduler") {
		var getcfg config.SchedulerConfig
		if err := ctx.ShouldBindJSON(&getcfg); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		configs[ctx.Param("name")] = getcfg
	}
	config.UpdateCfg(configs)
	config.WriteCfg()
	ctx.JSON(http.StatusOK, config.ConfigGetAll())
}

// @Summary List Config Type
// @Description List configurations of type
// @Tags config
// @Accept  json
// @Produce  json
// @Param type path string true "Type Name: ex. quality"
// @Param apikey query string true "apikey"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} string
// @Router /api/config/type/{type} [get]
func ApiListConfigType(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	configs := config.ConfigGetAll()
	list := make(map[string]interface{})
	for name, value := range configs {
		if strings.HasPrefix(name, ctx.Param("type")) {
			list[name] = value
		}
	}
	ctx.JSON(http.StatusOK, list)
}

type ApiNameInput struct {
	Cfg_Media string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}
type ApiNameInputJson struct {
	Cfg_Media string `json:"cfg_media"`
	GroupType string `json:"grouptype"`
	FilePath  string `json:"filepath"`
	MovieID   int    `json:"movieid"`
	SerieID   int    `json:"serieid"`
}

// @Summary Name Generation Test
// @Description Test your Naming Convention
// @Tags general
// @Accept  json
// @Produce  json
// @Param config body ApiNameInputJson true "Config"
// @Param apikey query string true "apikey"
// @Success 200 {object} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/naming [post]
func ApiNamingGenerate(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg ApiNameInput
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var cfg_media config.MediaTypeConfig
	config.ConfigGet(cfg.Cfg_Media, &cfg_media)

	if cfg.GroupType == "movie" {
		movie, _ := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{cfg.MovieID}})

		var cfg_list config.MediaListsConfig
		for idxlist := range cfg_media.Lists {
			if cfg_media.Lists[idxlist].Name == movie.Listname {
				cfg_list = cfg_media.Lists[idxlist]
				break
			}
		}

		s, _ := utils.NewStructure(
			cfg_media,
			cfg_list,
			cfg.GroupType,
			movie.Rootpath,
			config.PathsConfig{},
			config.PathsConfig{},
		)
		m, _ := s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)
		s.ParseFileAdditional(cfg.FilePath, m, filepath.Dir(cfg.FilePath), false, 0)

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, *m, movie, database.Serie{}, "", database.SerieEpisode{}, "", []int{})
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename})
	} else {
		series, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{cfg.SerieID}})

		var cfg_list config.MediaListsConfig
		for idxlist := range cfg_media.Lists {
			if cfg_media.Lists[idxlist].Name == series.Listname {
				cfg_list = cfg_media.Lists[idxlist]
				break
			}
		}

		s, _ := utils.NewStructure(
			cfg_media,
			cfg_list,
			cfg.GroupType,
			series.Rootpath,
			config.PathsConfig{},
			config.PathsConfig{},
		)
		m, _ := s.ParseFile(cfg.FilePath, true, filepath.Dir(cfg.FilePath), false)
		s.ParseFileAdditional(cfg.FilePath, m, filepath.Dir(cfg.FilePath), false, 0)

		_, episodes, _, serietitle, episodetitle, seriesEpisode, _ := s.GetSeriesEpisodes(series, cfg.FilePath, *m, filepath.Dir(cfg.FilePath))

		foldername, filename := s.GenerateNamingTemplate(cfg.FilePath, *m, database.Movie{}, series, serietitle, seriesEpisode, episodetitle, episodes)
		ctx.JSON(http.StatusOK, gin.H{"foldername": foldername, "filename": filename})
	}
}

type ApiStructureJson struct {
	Folder              string
	Disableruntimecheck bool
	Disabledisallowed   bool
	Grouptype           string
	Sourcepathtemplate  string
	Targetpathtemplate  string
	Configentry         string
	Forceid             int
}

// @Summary Structure Single Item
// @Description Structure a single folder
// @Tags general
// @Accept  json
// @Produce  json
// @Param config body ApiStructureJson true "Config"
// @Param apikey query string true "apikey"
// @Success 200 {object} string
// @Failure 400 {object} string
// @Failure 401 {object} string
// @Router /api/structure [post]
func ApiStructure(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	var cfg ApiStructureJson
	if err := ctx.BindJSON(&cfg); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cfg_media config.MediaTypeConfig
	if strings.EqualFold(cfg.Grouptype, "movie") {
		cfg.Configentry = "movie_" + cfg.Configentry
	}
	if strings.EqualFold(cfg.Grouptype, "series") {
		cfg.Configentry = "serie_" + cfg.Configentry
	}
	if config.ConfigGet(cfg.Configentry, &cfg_media) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "media config not found"})
		return
	}

	var cfg_source config.PathsConfig
	if config.ConfigGet("path_"+cfg.Sourcepathtemplate, &cfg_source) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "source config not found"})
		return
	}

	var cfg_target config.PathsConfig
	if config.ConfigGet("path_"+cfg.Targetpathtemplate, &cfg_target) != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "target config not found"})
		return
	}

	for idxlist := range cfg_media.Lists {
		if cfg.Forceid != 0 {
			utils.StructureSingleFolderAs(cfg.Folder, cfg.Forceid, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Grouptype, cfg_source, cfg_target, cfg_media, cfg_media.Lists[idxlist])
		} else {
			utils.StructureSingleFolder(cfg.Folder, cfg.Disableruntimecheck, cfg.Disabledisallowed, cfg.Grouptype, cfg_source, cfg_target, cfg_media, cfg_media.Lists[idxlist])
		}
	}
}
