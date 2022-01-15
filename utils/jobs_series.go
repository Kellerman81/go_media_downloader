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
	"github.com/Kellerman81/go_media_downloader/sizedwaitgroup"
	"github.com/Kellerman81/go_media_downloader/structure"
)

func jobImportSeriesParseV2(file string, updatemissing bool, configTemplate string, listConfig string, minPrio parser.ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	logger.Log.Debug("Series Parse: ", file)

	filecounter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_id in (Select id from series where listname = ?) and serie_episode_id <> 0", file, listConfig)
	//filecounter, _ := database.CountRows("serie_episode_files", database.Query{Where: "location = ? and serie_id in (Select id from series where listname = ?) and serie_episode_id <> 0", WhereArgs: []interface{}{file, list.Name}})
	if filecounter >= 1 {
		return
	}

	parsecounter, _ := database.CountRowsStatic("Select count(id) from serie_file_unmatcheds where filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", file, listConfig, time.Now().Add(time.Hour*-12))
	//parsecounter, _ := database.CountRows("serie_file_unmatcheds", database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
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

	//find dbseries
	series, entriesfound := m.FindSerieByParser(titlebuilder.String(), seriestitle, listConfig)
	addunmatched := false
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if entriesfound >= 1 {

		list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			return
		}

		m.GetPriority(configTemplate, list.Template_quality)
		errparsev := m.ParseVideoFile(file, configTemplate, list.Template_quality)
		if errparsev != nil {
			return
		}
		teststr := config.RegexSeriesIdentifier.FindStringSubmatch(m.Identifier)
		if len(teststr) == 0 {
			logger.Log.Warn("Failed parse identifier: ", file, " as ", m.Title, m.Identifier)
			return
		}

		testDbSeries, _ := database.GetDbserie(database.Query{Select: "identifiedby", Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})

		identifiedby := strings.ToLower(testDbSeries.Identifiedby)
		for _, epi := range importfeed.GetEpisodeArray(testDbSeries.Identifiedby, teststr[1], teststr[2]) {
			epi = strings.Trim(epi, "-EX")
			if identifiedby != "date" {
				epi = strings.TrimLeft(epi, "0")
			}
			if epi == "" {
				continue
			}
			logger.Log.Debug("Episode Identifier: ", epi)

			var SeriesEpisode database.SerieEpisode
			var SeriesEpisodeErr error
			if identifiedby == "date" {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series.ID, strings.Replace(epi, ".", "-", -1)}})
			} else {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series.ID, m.Season, epi}})
				if SeriesEpisodeErr != nil {
					SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series.ID, m.Identifier}})
				}
			}
			if SeriesEpisodeErr == nil {
				filecounter2, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_episode_id = ?", file, SeriesEpisode.ID)
				if filecounter2 == 0 {
					if SeriesEpisode.DbserieID == 0 {
						logger.Log.Warn("Failed parse match sub1: ", file, " as ", m.Title)
						continue
					}
					reached := false
					if m.Priority >= parser.NewCutoffPrio(configTemplate, list.Template_quality).Priority {
						reached = true
					}
					if series.Rootpath == "" && series.ID != 0 {
						rootpath := ""
						for idxpath := range configEntry.Data {
							if !config.ConfigCheck("path_" + configEntry.Data[idxpath].Template_path) {
								continue
							}
							cfg_path := config.ConfigGet("path_" + configEntry.Data[idxpath].Template_path).Data.(config.PathsConfig)

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

					counterif, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_episode_id = ?", file, SeriesEpisode.ID)
					//counterif, _ := database.CountRows("serie_episode_files", database.Query{Where: "location = ? AND serie_episode_id = ?", WhereArgs: []interface{}{file, SeriesEpisode.ID}})
					if counterif == 0 {
						database.InsertArray("serie_episode_files",
							[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "height", "width"},
							[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, SeriesEpisode.SerieID, SeriesEpisode.ID, SeriesEpisode.DbserieEpisodeID, SeriesEpisode.DbserieID, m.Height, m.Width})
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
		database.UpsertArray("serie_file_unmatcheds",
			[]string{"listname", "filepath", "last_checked", "parsed_data"},
			[]interface{}{listConfig, file, time.Now(), string(mjson)},
			database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, listConfig}})
		mjson = nil
	}
}

func RefreshSerie(id string) {
	sw := sizedwaitgroup.New(1)
	dbseries, _ := database.QueryDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
	for idxserie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " tvdb: ", dbseries[idxserie].ThetvdbID)
		sw.Add()
		go func(serie database.Dbserie) {
			importfeed.JobReloadDbSeries(serie, "", "", true, &sw)
		}(dbseries[idxserie])
	}
	sw.Wait()

}

func RefreshSeries() {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

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
		go func(serie database.Dbserie) {
			importfeed.JobReloadDbSeries(serie, "", "", true, &sw)
		}(dbseries[idxserie])
	}
	sw.Wait()

}

func RefreshSeriesInc() {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

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
		go func(serie database.Dbserie) {
			importfeed.JobReloadDbSeries(serie, "", "", true, &sw)
		}(dbseries[idxserie])
	}
	sw.Wait()

}

func Series_all_jobs(job string, force bool) {

	logger.Log.Info("Started Job: ", job, " for all")
	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		if !config.ConfigCheck(idxserie.Name) {
			continue
		}

		Series_single_jobs(job, idxserie.Name, "", force)
	}
}

func Series_single_jobs(job string, configTemplate string, listname string, force bool) {

	jobName := job + "_series"
	if configTemplate != "" {
		jobName += "_" + configTemplate
	}
	if listname != "" {
		jobName += "_" + listname
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", configTemplate)
		return
	}

	logger.Log.Info("Started Job: ", jobName)

	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, configTemplate, "Serie", time.Now()})
	if config.ConfigCheck(configTemplate) {
		cfg_serie := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
		if cfg_serie.Searchmissing_incremental == 0 {
			cfg_serie.Searchmissing_incremental = 20
		}
		if cfg_serie.Searchupgrade_incremental == 0 {
			cfg_serie.Searchupgrade_incremental = 20
		}

		switch job {
		case "datafull":
			Getnewepisodes(configTemplate)
		case "searchmissingfull":
			searcher.SearchSerieMissing(configTemplate, 0, false)
		case "searchmissinginc":
			searcher.SearchSerieMissing(configTemplate, cfg_serie.Searchmissing_incremental, false)
		case "searchupgradefull":
			searcher.SearchSerieUpgrade(configTemplate, 0, false)
		case "searchupgradeinc":
			searcher.SearchSerieUpgrade(configTemplate, cfg_serie.Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			searcher.SearchSerieMissing(configTemplate, 0, true)
		case "searchmissinginctitle":
			searcher.SearchSerieMissing(configTemplate, cfg_serie.Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			searcher.SearchSerieUpgrade(configTemplate, 0, true)
		case "searchupgradeinctitle":
			searcher.SearchSerieUpgrade(configTemplate, cfg_serie.Searchupgrade_incremental, true)
		case "structure":
			seriesStructureSingle(configTemplate)

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
		qualis := []string{}

		for idxlist := range cfg_serie.Lists {
			if !logger.CheckStringArray(qualis, cfg_serie.Lists[idxlist].Template_quality) {
				qualis = append(qualis, cfg_serie.Lists[idxlist].Template_quality)
			}
			switch job {
			case "data":
				getnewepisodessingle(configTemplate, cfg_serie.Lists[idxlist].Name)
			case "checkmissing":
				checkmissingepisodessingle(configTemplate, cfg_serie.Lists[idxlist].Name)
			case "checkmissingflag":
				checkmissingepisodesflag(configTemplate, cfg_serie.Lists[idxlist].Name)
			case "checkreachedflag":
				checkreachedepisodesflag(configTemplate, cfg_serie.Lists[idxlist].Name)
			case "clearhistory":
				database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_id in (Select id from series where listname=?)", WhereArgs: []interface{}{cfg_serie.Lists[idxlist].Name}})
			case "feeds":
				Importnewseriessingle(configTemplate, cfg_serie.Lists[idxlist].Name)
			default:
				// other stuff
			}
		}
		for idx := range qualis {
			switch job {
			case "rss":
				searcher.SearchSerieRSS(configTemplate, qualis[idx])
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", configTemplate)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", job, " for ", configTemplate)
	debug.FreeOSMemory()
}

func Importnewseriessingle(configTemplate string, listConfig string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerMetadata == 0 {
		cfg_general.WorkerMetadata = 1
	}

	logger.Log.Info("Get Serie Config", listConfig)
	logger.Log.Info("Workers: ", cfg_general.WorkerMetadata)
	swg := sizedwaitgroup.New(cfg_general.WorkerMetadata)
	for idxserie, serie := range feeds(configTemplate, listConfig).Series.Serie {
		logger.Log.Info("Import Serie ", idxserie, " name: ", serie.Name)
		swg.Add()
		go func(serie config.SerieConfig) {
			importfeed.JobImportDbSeries(serie, configTemplate, listConfig, false, &swg)
		}(serie)
	}
	swg.Wait()
}

func Getnewepisodes(configTemplate string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	logger.Log.Info("Scan SerieEpisodeFile")
	filesfound := findFiles(configTemplate)

	logger.Log.Info("Workers: ", cfg_general.WorkerParse)
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for _, list := range config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig).Lists {
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			continue
		}
		for idxfile, file := range scanner.GetFilesSeriesAdded(filesfound, configTemplate, list.Name, list.Name) {
			logger.Log.Info("Parse Serie ", idxfile, " path: ", file)
			swf.Add()
			go func(file string) {
				jobImportSeriesParseV2(file, true, configTemplate, list.Name, parser.NewDefaultPrio(configTemplate, list.Template_quality), &swf)
			}(file)
		}
	}
	swf.Wait()
	filesfound = nil
}
func getnewepisodessingle(configTemplate string, listConfig string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}
	logger.Log.Info("Scan SerieEpisodeFile")
	logger.Log.Info("Workers: ", cfg_general.WorkerParse)
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for idxfile, file := range scanner.GetFilesSeriesAdded(findFiles(configTemplate), configTemplate, listConfig, listConfig) {
		logger.Log.Info("Parse Serie ", idxfile, " path: ", file)
		swf.Add()
		go func(file string) {
			jobImportSeriesParseV2(file, true, configTemplate, listConfig, parser.NewDefaultPrio(configTemplate, list.Template_quality), &swf)
		}(file)
	}
	swf.Wait()
}

func checkmissingepisodesflag(configTemplate string, listConfig string) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "id, missing", Where: "serie_id in (Select id from series where listname = ?)", WhereArgs: []interface{}{listConfig}})
	for idxepi := range episodes {
		counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where serie_episode_id=?", episodes[idxepi].ID)
		//counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
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

func checkreachedepisodesflag(configTemplate string, listConfig string) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ?)", WhereArgs: []interface{}{listConfig}})
	for idxepi := range episodes {
		if !config.ConfigCheck("quality_" + episodes[idxepi].QualityProfile) {
			continue
		}
		reached := false
		if parser.GetHighestEpisodePriorityByFiles(episodes[idxepi], configTemplate, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrio(configTemplate, episodes[idxepi].QualityProfile).Priority {
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

func checkmissingepisodessingle(configTemplate string, listConfig string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)

	for _, filerow := range database.QueryStaticColumnsOneStringNoError("Select location from serie_episode_files where serie_id in (Select id from series where listname = ?)", "Select count(id) from serie_episode_files where serie_id in (Select id from series where listname = ?)", listConfig) {
		swfile.Add()
		go func(file string) {
			jobImportFileCheck(file, "serie", &swfile)
		}(filerow.Str)
	}
	swfile.Wait()
}

func getTraktUserPublicShowList(configTemplate string, listConfig string) config.MainSerieConfig {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if !list.Enabled {
		return config.MainSerieConfig{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return config.MainSerieConfig{}
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)
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

func seriesStructureSingle(configTemplate string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	//swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)

	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for idxpath := range row.DataImport {
		mappathimport := row.DataImport[idxpath].Template_path
		if !config.ConfigCheck("path_" + mappathimport) {
			logger.Log.Debug("Path not found: ", "path_"+mappathimport)

			continue
		}
		cfg_path_import := config.ConfigGet("path_" + mappathimport).Data.(config.PathsConfig)

		if !config.ConfigCheck("path_" + row.Data[0].Template_path) {
			logger.Log.Debug("Path not found: ", "path_"+row.Data[0].Template_path)
			continue
		}
		var cfg_path config.PathsConfig
		if len(row.Data) >= 1 {
			cfg_path = config.ConfigGet("path_" + row.Data[0].Template_path).Data.(config.PathsConfig)

		}
		if strings.EqualFold(lastSeriesStructure, cfg_path_import.Path) && lastSeriesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = cfg_path_import.Path
		//swfile.Add()

		structure.StructureFolders("series", cfg_path_import, cfg_path, configTemplate)
		//swfile.Done()

	}
	//swfile.Wait()
}
