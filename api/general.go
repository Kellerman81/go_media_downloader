package api

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/scheduler"
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

// @Summary Parse a string
// @Description Parses a string for testing
// @Tags parse
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200 {object} apiparse
// @Failure 401 {object} string
// @Router /api/parse/string [get]
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
// @Param apikey query string true "apikey"
// @Success 200 {object} apiparse
// @Failure 401 {object} string
// @Router /api/parse/file [get]
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
// @Success 200
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
// @Success 200
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
// @Success 200
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

// @Summary Close DB
// @Description Closes all database connections
// @Tags database
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
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
// @Success 200
// @Failure 401 {object} string
// @Router /api/db/clear/{name} [get]
func ApiDbClear(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.ReadWriteMu.Lock()
	database.DB.Exec("DELETE FROM " + ctx.Param("name") + "; VACUUM;")
	database.ReadWriteMu.Unlock()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Vacuum DB
// @Description Vacuum database
// @Tags database
// @Accept  json
// @Produce  json
// @Param apikey query string true "apikey"
// @Success 200
// @Failure 401 {object} string
// @Router /api/db/vacuum [get]
func ApiDbVacuum(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.ReadWriteMu.Lock()
	database.DB.Exec("VACUUM;")
	database.ReadWriteMu.Unlock()
	ctx.JSON(http.StatusOK, "ok")
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
// @Success 200
// @Failure 401 {object} string
// @Router /api/quality/{id} [delete]
func ApiQualityDelete(ctx *gin.Context) {
	if ApiAuth(ctx) == http.StatusUnauthorized {
		return
	}
	database.DeleteRow("qualities", database.Query{Where: "id=?", WhereArgs: []interface{}{ctx.Param("id")}})
	database.GetVars()
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Update Quality
// @Description Updates or adds a quality
// @Tags quality
// @Accept  json
// @Produce  json
// @Param quality body database.Qualities true "Quality"
// @Param apikey query string true "apikey"
// @Success 200
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
	ctx.JSON(http.StatusOK, "ok")
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
// @Router /api/config/clear [get]
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
// @Success 200
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
	ctx.JSON(http.StatusOK, "ok")
}

// @Summary Update Config
// @Description Updates a configuration
// @Tags config
// @Accept  json
// @Produce  json
// @Param config body interface{} true "Config"
// @Param name path string true "Type Name: ex. quality_SD"
// @Param apikey query string true "apikey"
// @Success 200
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
	ctx.JSON(http.StatusOK, "ok")
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
