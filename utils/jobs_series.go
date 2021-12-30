package utils

import (
	"encoding/json"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/remeh/sizedwaitgroup"
)

func JobImportSeriesParseV2(file string, updatemissing bool, configEntry config.MediaTypeConfig, list config.MediaListsConfig, minPrio parser.ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	logger.Log.Debug("Series Parse: ", file)

	filecounter, _ := database.CountRows("serie_episode_files", database.Query{InnerJoin: "Series ON Series.ID = Serie_episode_files.serie_id", Where: "Serie_episode_files.location = ? and series.listname = ? and Serie_episode_files.serie_episode_id <> 0", WhereArgs: []interface{}{file, list.Name}})
	if filecounter >= 1 {
		return
	}

	parsecounter, _ := database.CountRows("serie_file_unmatcheds", database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
	if parsecounter >= 1 {
		return
	}

	m, err := parser.NewFileParser(filepath.Base(file), true, "series")
	if err != nil {
		return
	}
	m.Resolution = strings.ToLower(m.Resolution)
	m.Audio = strings.ToLower(m.Audio)
	m.Codec = strings.ToLower(m.Codec)
	var titlebuilder strings.Builder
	titlebuilder.Grow(50)
	titlebuilder.WriteString(m.Title)
	if m.Year != 0 {
		titlebuilder.WriteString(" (")
		titlebuilder.WriteString(strconv.Itoa(m.Year))
		titlebuilder.WriteString(")")
	}
	seriestitle := ""
	matched := config.RegexSeriesTitle.FindStringSubmatch(filepath.Base(file))
	if len(matched) >= 2 {
		seriestitle = matched[1]
	}
	logger.Log.Debug("Parsed SerieEpisode: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " Matched: ", matched, " Identifier: ", m.Identifier, " Date: ", m.Date, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
	//logger.Log.Debug("Parse Data: ", m)

	//find dbseries
	series, entriesfound := importfeed.FindSerieByParser(*m, titlebuilder.String(), seriestitle, list.Name)
	addunmatched := false
	if entriesfound >= 1 {

		if !config.ConfigCheck("quality_" + list.Template_quality) {
			return
		}
		var cfg_quality config.QualityConfig
		config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

		cutoffPrio := parser.NewCutoffPrio(configEntry, cfg_quality)

		m.GetPriority(configEntry, cfg_quality)
		errparsev := m.ParseVideoFile(file, configEntry, cfg_quality)
		if errparsev != nil {
			return
		}
		teststr := config.RegexSeriesIdentifier.FindStringSubmatch(m.Identifier)
		if len(teststr) == 0 {
			logger.Log.Warn("Failed parse identifier: ", file, " as ", m.Title, m.Identifier)
			return
		}

		testDbSeries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})

		for _, epi := range importfeed.GetEpisodeArray(testDbSeries.Identifiedby, teststr[1], teststr[2]) {
			epi = strings.Trim(epi, "-EX")
			if strings.ToLower(testDbSeries.Identifiedby) != "date" {
				epi = strings.TrimLeft(epi, "0")
			}
			if epi == "" {
				continue
			}
			logger.Log.Debug("Episode Identifier: ", epi)

			var SeriesEpisode database.SerieEpisode
			var SeriesEpisodeErr error
			if strings.EqualFold(testDbSeries.Identifiedby, "date") {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series.ID, strings.Replace(epi, ".", "-", -1)}})
			} else {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series.ID, m.Season, epi}})
				if SeriesEpisodeErr != nil {
					SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series.ID, m.Identifier}})
				}
			}
			if SeriesEpisodeErr == nil {
				_, SeriesEpisodeFileerr := database.GetSerieEpisodeFiles(database.Query{Select: "id", Where: "location = ? AND serie_episode_id = ?", WhereArgs: []interface{}{file, SeriesEpisode.ID}})
				if SeriesEpisodeFileerr != nil {
					if SeriesEpisode.DbserieID == 0 {
						logger.Log.Warn("Failed parse match sub1: ", file, " as ", m.Title)
						continue
					}
					reached := false
					if m.Priority >= cutoffPrio.Priority {
						reached = true
					}
					if series.Rootpath == "" && series.ID != 0 {
						rootpath := ""
						for idxpath := range configEntry.Data {
							if !config.ConfigCheck("path_" + configEntry.Data[idxpath].Template_path) {
								continue
							}
							var cfg_path config.PathsConfig
							config.ConfigGet("path_"+configEntry.Data[idxpath].Template_path, &cfg_path)

							pppath := cfg_path.Path
							if strings.Contains(file, pppath) {
								rootpath = pppath
								tempfoldername := strings.Replace(file, pppath, "", -1)
								tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
								tempfoldername = filepath.Dir(tempfoldername)
								_, firstfolder := logger.Getrootpath(tempfoldername)
								rootpath = filepath.Join(rootpath, firstfolder)
								break
							}
						}
						database.UpdateColumn("series", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{series.ID}})
					}

					logger.Log.Debug("Parsed and add: ", file, " as ", m.Title)

					counterif, _ := database.CountRows("serie_episode_files", database.Query{Where: "location = ? AND serie_episode_id = ?", WhereArgs: []interface{}{file, SeriesEpisode.ID}})
					if counterif == 0 {
						database.InsertArray("serie_episode_files",
							[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id"},
							[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, SeriesEpisode.SerieID, SeriesEpisode.ID, SeriesEpisode.DbserieEpisodeID, SeriesEpisode.DbserieID})
					}
					if updatemissing {
						database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
						database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
						database.UpdateColumn("serie_episodes", "quality_profile", list.Template_quality, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
					}

					database.DeleteRow("serie_file_unmatcheds", database.Query{Where: "filepath = ?", WhereArgs: []interface{}{file}})

				} else {
					logger.Log.Debug("Already Parsed: ", file)
				}
			} else {
				addunmatched = true
				logger.Log.Debug("SerieEpisode not matched loop: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio, " Season ", m.Season, " Epi ", epi)
			}
		}
	} else {
		addunmatched = true
		logger.Log.Debug("SerieEpisode not matched: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
	}
	if addunmatched {
		mjson, _ := json.Marshal(m)
		valuesupsert := make(map[string]interface{})
		valuesupsert["listname"] = list.Name
		valuesupsert["filepath"] = file
		valuesupsert["last_checked"] = time.Now()
		valuesupsert["parsed_data"] = string(mjson)
		database.Upsert("serie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})
	}
}

func RefreshSerie(id string) {
	sw := sizedwaitgroup.New(1)
	dbseries, _ := database.QueryDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
	for idxserie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " tvdb: ", dbseries[idxserie].ThetvdbID)
		sw.Add()
		importfeed.JobReloadDbSeries(dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func RefreshSeries() {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{})
	for idxserie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " tvdb: ", dbseries[idxserie].ThetvdbID)
		sw.Add()
		importfeed.JobReloadDbSeries(dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func RefreshSeriesInc() {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

	for idxserie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " tvdb: ", dbseries[idxserie].ThetvdbID)
		sw.Add()
		importfeed.JobReloadDbSeries(dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func Series_all_jobs(job string, force bool) {
	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	logger.Log.Info("Started Job: ", job, " for all")
	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		Series_single_jobs(job, cfg_serie.Name, "", force)
	}
}

func Series_single_jobs(job string, typename string, listname string, force bool) {

	jobName := job + "_series"
	if typename != "" {
		jobName += "_" + typename
	}
	if listname != "" {
		jobName += "_" + listname
	}
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if cfg_general.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", typename)
		return
	}

	logger.Log.Info("Started Job: ", jobName)

	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, typename, "Serie", time.Now()})
	ok, _ := config.ConfigDB.Has("serie_" + typename)
	if ok {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet("serie_"+typename, &cfg_serie)
		if cfg_serie.Searchmissing_incremental == 0 {
			cfg_serie.Searchmissing_incremental = 20
		}
		if cfg_serie.Searchupgrade_incremental == 0 {
			cfg_serie.Searchupgrade_incremental = 20
		}

		switch job {
		case "datafull":
			Getnewepisodes(cfg_serie)
		case "searchmissingfull":
			searcher.SearchSerieMissing(cfg_serie, 0, false)
		case "searchmissinginc":
			searcher.SearchSerieMissing(cfg_serie, cfg_serie.Searchmissing_incremental, false)
		case "searchupgradefull":
			searcher.SearchSerieUpgrade(cfg_serie, 0, false)
		case "searchupgradeinc":
			searcher.SearchSerieUpgrade(cfg_serie, cfg_serie.Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			searcher.SearchSerieMissing(cfg_serie, 0, true)
		case "searchmissinginctitle":
			searcher.SearchSerieMissing(cfg_serie, cfg_serie.Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			searcher.SearchSerieUpgrade(cfg_serie, 0, true)
		case "searchupgradeinctitle":
			searcher.SearchSerieUpgrade(cfg_serie, cfg_serie.Searchupgrade_incremental, true)

		}
		if listname != "" {
			logger.Log.Debug("Listname: ", listname)
			var templists []config.MediaListsConfig
			for idxlist := range cfg_serie.Lists {
				if cfg_serie.Lists[idxlist].Name == listname {
					templists = append(templists, cfg_serie.Lists[idxlist])
				}
			}
			logger.Log.Debug("Listname: found: ", templists)
			cfg_serie.Lists = templists
		}
		qualis := make(map[string]bool, 10)
		for idxlist := range cfg_serie.Lists {
			if _, ok := qualis[cfg_serie.Lists[idxlist].Template_quality]; !ok {
				qualis[cfg_serie.Lists[idxlist].Template_quality] = true
			}
			switch job {
			case "data":
				getnewepisodessingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "checkmissing":
				checkmissingepisodessingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "checkmissingflag":
				checkmissingepisodesflag(cfg_serie, cfg_serie.Lists[idxlist])
			case "checkreachedflag":
				checkreachedepisodesflag(cfg_serie, cfg_serie.Lists[idxlist])
			case "structure":
				seriesStructureSingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "clearhistory":
				database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_id in (Select id from series where listname=?)", WhereArgs: []interface{}{typename}})
			case "feeds":
				Importnewseriessingle(cfg_serie, cfg_serie.Lists[idxlist])
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				searcher.SearchSerieRSS(cfg_serie, qual)
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", typename)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", job, " for ", typename)
	debug.FreeOSMemory()
}

func Importnewseriessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if cfg_general.WorkerMetadata == 0 {
		cfg_general.WorkerMetadata = 1
	}

	results := Feeds(row, list)

	logger.Log.Info("Get Serie Config", list.Name)
	logger.Log.Info("Workers: ", cfg_general.WorkerMetadata)
	swg := sizedwaitgroup.New(cfg_general.WorkerMetadata)
	for idxserie := range results.Series.Serie {
		logger.Log.Info("Import Serie ", idxserie, " of ", len(results.Series.Serie), " name: ", results.Series.Serie[idxserie].Name)
		swg.Add()
		importfeed.JobImportDbSeries(results.Series.Serie[idxserie], row, list, false, &swg)
	}
	swg.Wait()
}

func Getnewepisodes(row config.MediaTypeConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	logger.Log.Info("Scan SerieEpisodeFile")
	var filesfound []string
	if len(row.Data) == 1 {
		if config.ConfigCheck("path_" + row.Data[0].Template_path) {
			var cfg_path config.PathsConfig
			config.ConfigGet("path_"+row.Data[0].Template_path, &cfg_path)

			filesfound = scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		}
	} else {
		for idxpath := range row.Data {
			if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
				continue
			}
			var cfg_path config.PathsConfig
			config.ConfigGet("path_"+row.Data[idxpath].Template_path, &cfg_path)

			filesfound = append(filesfound, scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)...)
		}
	}

	logger.Log.Info("Workers: ", cfg_general.WorkerParse)
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for _, list := range row.Lists {
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			continue
		}
		var cfg_quality config.QualityConfig
		config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

		defaultPrio := &parser.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
		defaultPrio.GetPriority(row, cfg_quality)

		filesadded := scanner.GetFilesSeriesAdded(filesfound, list.Name)
		for idxfile := range filesadded {
			logger.Log.Info("Parse Serie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
			swf.Add()
			JobImportSeriesParseV2(filesadded[idxfile], true, row, list, *defaultPrio, &swf)
		}
	}
	swf.Wait()
}
func getnewepisodessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

	defaultPrio := &parser.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
	defaultPrio.GetPriority(row, cfg_quality)

	logger.Log.Info("Scan SerieEpisodeFile")
	var filesfound []string
	for idxpath := range row.Data {
		if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
			continue
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+row.Data[idxpath].Template_path, &cfg_path)

		filesfound_add := scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		filesfound = append(filesfound, filesfound_add...)
	}
	filesadded := scanner.GetFilesSeriesAdded(filesfound, list.Name)
	logger.Log.Info("Find SerieEpisodeFile")
	logger.Log.Info("Workers: ", cfg_general.WorkerParse)
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for idxfile := range filesadded {
		logger.Log.Info("Parse Serie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
		swf.Add()
		JobImportSeriesParseV2(filesadded[idxfile], true, row, list, *defaultPrio, &swf)
	}
	swf.Wait()
}

func checkmissingepisodesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "serie_episodes.id, serie_episodes.missing", InnerJoin: " series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{list.Name}})
	for idxepi := range episodes {
		counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		if counter >= 1 {
			if episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		} else {
			if !episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		}
	}
}

func checkreachedepisodesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "serie_episodes.id, serie_episodes.quality_reached, serie_episodes.quality_profile", InnerJoin: " series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{list.Name}})
	for idxepi := range episodes {
		if !config.ConfigCheck("quality_" + episodes[idxepi].QualityProfile) {
			continue
		}
		var cfg_quality config.QualityConfig
		config.ConfigGet("quality_"+episodes[idxepi].QualityProfile, &cfg_quality)

		MinimumPriority := parser.GetHighestEpisodePriorityByFiles(episodes[idxepi], row, cfg_quality)
		cutoffPrio := parser.NewCutoffPrio(row, cfg_quality)
		reached := false
		if MinimumPriority >= cutoffPrio.Priority {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumn("Serie_episodes", "quality_reached", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumn("Serie_episodes", "quality_reached", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		}
	}
}

func checkmissingepisodessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	series, _ := database.QuerySeries(database.Query{Select: "id", Where: "listname=?", WhereArgs: []interface{}{list.Name}})

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)
	for idx := range series {
		seriefile, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "location", Where: "Serie_id=?", WhereArgs: []interface{}{series[idx].ID}})

		for idxfile := range seriefile {
			swfile.Add()
			JobImportFileCheck(seriefile[idxfile].Location, "serie", &swfile)
		}
	}
	swfile.Wait()
}

func GetTraktUserPublicShowList(configEntry config.MediaTypeConfig, list config.MediaListsConfig) config.MainSerieConfig {
	if !list.Enabled {
		return config.MainSerieConfig{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return config.MainSerieConfig{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)

	if len(cfg_list.TraktUsername) >= 1 && len(cfg_list.TraktListName) >= 1 {
		if len(cfg_list.TraktListType) == 0 {
			cfg_list.TraktListType = "show"
		}
		data, err := apiexternal.TraktApi.GetUserList(cfg_list.TraktUsername, cfg_list.TraktListName, cfg_list.TraktListType, cfg_list.Limit)
		if err != nil {
			logger.Log.Error("Failed to read trakt list: ", cfg_list.TraktListName)
			return config.MainSerieConfig{}
		}
		d := config.MainSerieConfig{}

		for idx := range data {
			d.Serie = append(d.Serie, config.SerieConfig{
				Name:   data[idx].Serie.Title,
				TvdbID: data[idx].Serie.Ids.Tvdb,
			})
		}
		return d
	}
	return config.MainSerieConfig{}
}

var lastSeriesStructure string

func seriesStructureSingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)

	for idxpath := range row.DataImport {
		mappathimport := row.DataImport[idxpath].Template_path
		if !config.ConfigCheck("path_" + mappathimport) {
			continue
		}
		var cfg_path_import config.PathsConfig
		config.ConfigGet("path_"+mappathimport, &cfg_path_import)

		if !config.ConfigCheck("path_" + row.Data[0].Template_path) {
			continue
		}
		var cfg_path config.PathsConfig
		if len(row.Data) >= 1 {
			config.ConfigGet("path_"+row.Data[0].Template_path, &cfg_path)
		}
		if strings.EqualFold(lastSeriesStructure, cfg_path_import.Path) && lastSeriesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = cfg_path_import.Path
		swfile.Add()

		structure.StructureFolders("series", cfg_path_import, cfg_path, row, list)
		swfile.Done()
	}
	swfile.Wait()
}
