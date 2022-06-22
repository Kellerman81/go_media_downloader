package utils

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/shomali11/parallelizer"
)

func updateRootpath(file string, objtype string, objid uint, configTemplate string) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	rootpath := ""
	for idx := range configEntry.Data {
		if !config.ConfigCheck("path_" + configEntry.Data[idx].Template_path) {
			continue
		}
		cfg_path := config.ConfigGet("path_" + configEntry.Data[idx].Template_path).Data.(config.PathsConfig)

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
	database.UpdateColumn(objtype, "rootpath", rootpath, database.Query{Where: "id = ?", WhereArgs: []interface{}{objid}})

}
func JobImportSeriesParseV2(file string, updatemissing bool, configTemplate string, listConfig string) {
	jobImportSeriesParseV2(file, updatemissing, configTemplate, listConfig)
}
func jobImportSeriesParseV2(file string, updatemissing bool, configTemplate string, listConfig string) {
	logger.Log.Debug("Series Parse: ", file)

	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if list.Name == "" {

		return
	}

	if ok := checkignorelistsonpath(configTemplate, file, listConfig); !ok {

		return
	}
	if ok := checkunmatched(configTemplate, file, listConfig); !ok {
		return
	}
	counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_id in (Select id from series where listname = ?) and serie_episode_id <> 0", file, listConfig)
	if counter >= 1 {
		return
	}

	m, err := parser.NewFileParser(filepath.Base(file), true, "series")
	if err != nil {
		return
	}
	defer m.Close()

	m.Resolution = strings.ToLower(m.Resolution)
	m.Audio = strings.ToLower(m.Audio)
	m.Codec = strings.ToLower(m.Codec)
	var titlebuilder bytes.Buffer
	titlebuilder.WriteString(m.Title)
	if m.Year != 0 {
		titlebuilder.WriteString(" (")
		titlebuilder.WriteString(strconv.Itoa(m.Year))
		titlebuilder.WriteString(")")
	}
	seriestitle := ""
	matched := config.RegexGet("RegexSeriesTitle").FindStringSubmatch(filepath.Base(file))
	defer logger.ClearVar(&matched)
	if len(matched) >= 2 {
		seriestitle = matched[1]
	}
	logger.Log.Debug("Parsed SerieEpisode: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " Matched: ", matched, " Identifier: ", m.Identifier, " Date: ", m.Date, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

	series, entriesfound, err := m.FindSerieByParser(titlebuilder.String(), seriestitle, listConfig)
	titlebuilder.Reset()
	addunmatched := false
	if err == nil {
		defer logger.ClearVar(&series)
		if entriesfound >= 1 {
			m.GetPriority(configTemplate, list.Template_quality)
			errparsev := m.ParseVideoFile(file, configTemplate, list.Template_quality)
			if errparsev != nil {

				logger.Log.Error("Parse failed: ", errparsev)
				return
			}

			dbseries, err := database.QueryColumnUint("Select dbserie_id from series where id = ?", series)
			identifiedby, err := database.QueryColumnString("Select identifiedby from dbseries where id = ?", dbseries)
			if err != nil {

				return
			}

			epiarray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
			defer logger.ClearVar(&epiarray)
			var reached bool
			for _, epi := range epiarray {
				epi = strings.Trim(epi, "-EX")
				if strings.ToLower(identifiedby) != "date" {
					epi = strings.TrimLeft(epi, "0")
				}
				if epi == "" {
					continue
				}
				logger.Log.Debug("Episode Identifier: ", epi)

				dbserieepisodeid, _ := importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbseries, m.Identifier, m.SeasonStr, epi)
				matched := false
				if dbserieepisodeid != 0 {
					serieepisodeid, _ := database.QueryColumnUint("Select id from serie_episodes where serie_id = ? and dbserie_episode = ?", series, dbserieepisodeid)
					if serieepisodeid != 0 {
						matched = true
						rootpath, err := database.QueryColumnString("Select rootpath from series where id = ?", series)
						if rootpath == "" && series != 0 {
							updateRootpath(file, "series", series, configTemplate)
						}
						counter, err = database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_episode_id = ?", file, serieepisodeid)
						if counter == 0 && err == nil {
							reached = false
							if m.Priority >= parser.NewCutoffPrio(configTemplate, list.Template_quality).Priority {
								reached = true
							}

							logger.Log.Debug("Parsed and add: ", file, " as ", m.Title)

							database.InsertArray("serie_episode_files",
								[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "height", "width"},
								[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, series, serieepisodeid, dbserieepisodeid, dbseries, m.Height, m.Width})

							if updatemissing {
								database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id = ?", WhereArgs: []interface{}{serieepisodeid}})
								database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id = ?", WhereArgs: []interface{}{serieepisodeid}})
								database.UpdateColumn("serie_episodes", "quality_profile", list.Template_quality, database.Query{Where: "id = ?", WhereArgs: []interface{}{serieepisodeid}})
							}

							database.DeleteRow("serie_file_unmatcheds", database.Query{Where: "filepath = ?", WhereArgs: []interface{}{file}})

						} else {
							logger.Log.Debug("Already Parsed: ", file)
						}
					}
				}
				if !matched {
					addunmatched = true
					logger.Log.Debug("SerieEpisode not matched loop: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio, " Season ", m.Season, " Epi ", epi)
					logger.Log.Infoln("SerieEpisode not matched: ", file)
				}
			}
		} else {
			addunmatched = true
			logger.Log.Debug("SerieEpisode not matched: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
			logger.Log.Infoln("SerieEpisode not matched: ", file)
		}
	}

	if addunmatched {
		mjson, mjsonerr := json.Marshal(m)
		defer logger.ClearVar(&mjson)
		if mjsonerr == nil {
			database.UpsertArray("serie_file_unmatcheds",
				[]string{"listname", "filepath", "last_checked", "parsed_data"},
				[]interface{}{listConfig, file, time.Now(), string(mjson)},
				database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, listConfig}})
		}
	}

}

func RefreshSerie(id string) {
	dbseries, _ := database.QueryStaticColumnsOneInt("Select id from dbseries where id = ?", "", id)
	defer logger.ClearVar(&dbseries)
	for idxserie, serie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " num: ", serie.Num)
		importfeed.JobReloadDbSeries(uint(serie.Num), "", "", true)
	}
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

	dbseries, _ := database.QueryStaticColumnsOneInt("Select id from dbseries", "Select count(id) from dbseries")
	defer logger.ClearVar(&dbseries)
	for idxserie, serie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " num: ", serie.Num)
		importfeed.JobReloadDbSeries(uint(serie.Num), "", "", true)
	}
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

	dbseries, _ := database.QueryStaticColumnsOneInt("Select id from dbseries where status = 'Continuing' order by updated_at asc limit 20", "Select count(id) from dbseries where status = 'Continuing' order by updated_at asc limit 20")
	defer logger.ClearVar(&dbseries)
	for idxserie, serie := range dbseries {
		logger.Log.Info("Refresh Serie ", idxserie, " of ", len(dbseries), " num: ", serie.Num)
		importfeed.JobReloadDbSeries(uint(serie.Num), "", "", true)
	}
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
			func() { GetNewFilesMap(configTemplate, "series", "") }()
		case "rssseasons":
			func() { searcher.SearchSeriesRSSSeasons(configTemplate) }()
		case "searchmissingfull":
			func() { searcher.SearchSerieMissing(configTemplate, 0, false) }()
		case "searchmissinginc":
			func() { searcher.SearchSerieMissing(configTemplate, cfg_serie.Searchmissing_incremental, false) }()
		case "searchupgradefull":
			func() { searcher.SearchSerieUpgrade(configTemplate, 0, false) }()
		case "searchupgradeinc":
			func() { searcher.SearchSerieUpgrade(configTemplate, cfg_serie.Searchupgrade_incremental, false) }()
		case "searchmissingfulltitle":
			func() { searcher.SearchSerieMissing(configTemplate, 0, true) }()
		case "searchmissinginctitle":
			func() { searcher.SearchSerieMissing(configTemplate, cfg_serie.Searchmissing_incremental, true) }()
		case "searchupgradefulltitle":
			func() { searcher.SearchSerieUpgrade(configTemplate, 0, true) }()
		case "searchupgradeinctitle":
			func() { searcher.SearchSerieUpgrade(configTemplate, cfg_serie.Searchupgrade_incremental, true) }()
		case "structure":
			func() { seriesStructureSingle(configTemplate) }()

		}
		if listname != "" {
			cfg_serie.Lists = config.MedialistConfigFilterByListName(cfg_serie.Lists, listname)
		}
		qualis := make([]string, 0, len(cfg_serie.Lists))
		defer logger.ClearVar(&qualis)

		for idxlist := range cfg_serie.Lists {
			foundentry := false
			for idx2 := range qualis {
				if qualis[idx2] == cfg_serie.Lists[idxlist].Template_quality {
					foundentry = true
					break
				}
			}
			if !foundentry {
				qualis = append(qualis, cfg_serie.Lists[idxlist].Template_quality)
			}
			switch job {
			case "data":
				func() { GetNewFilesMap(configTemplate, "series", cfg_serie.Lists[idxlist].Name) }()
			case "checkmissing":
				func() { checkmissingepisodessingle(configTemplate, cfg_serie.Lists[idxlist].Name) }()
			case "checkmissingflag":
				func() { checkmissingepisodesflag(configTemplate, cfg_serie.Lists[idxlist].Name) }()
			case "checkreachedflag":
				func() { checkreachedepisodesflag(configTemplate, cfg_serie.Lists[idxlist].Name) }()
			case "clearhistory":
				func() {
					database.DeleteRow("serie_episode_histories", database.Query{Where: "serie_id in (Select id from series where listname = ?)", WhereArgs: []interface{}{cfg_serie.Lists[idxlist].Name}})
				}()
			case "feeds":
				func() { Importnewseriessingle(configTemplate, cfg_serie.Lists[idxlist].Name) }()
			default:
				// other stuff
			}
		}
		for idx := range qualis {
			switch job {
			case "rss":
				func() { searcher.SearchSerieRSS(configTemplate, qualis[idx]) }()
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", configTemplate)
	}
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})
	}
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
	feed, err := feeds(configTemplate, listConfig)
	if err != nil {
		return
	}
	defer logger.ClearVar(&feed)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerMetadata))
	for idxserie := range feed.Series.Serie {
		serie := feed.Series.Serie[idxserie]
		logger.Log.Info("Import Serie ", idxserie, " name: ", serie.Name)
		swg.Add(func() {
			importfeed.JobImportDbSeries(serie, configTemplate, listConfig, false)
		})
	}
	swg.Wait()
	swg.Close()
}

func checkmissingepisodesflag(configTemplate string, listConfig string) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "id, missing", Where: "serie_id in (Select id from series where listname = ?)", WhereArgs: []interface{}{listConfig}})
	defer logger.ClearVar(&episodes)
	for idxepi := range episodes {
		counter, err := database.CountRowsStatic("Select count(id) from serie_episode_files where serie_episode_id = ?", episodes[idxepi].ID)
		if counter >= 1 {
			if episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 0, database.Query{Where: "id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		} else {
			if !episodes[idxepi].Missing && err == nil {
				database.UpdateColumn("Serie_episodes", "missing", 1, database.Query{Where: "id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		}
	}
}

func checkreachedepisodesflag(configTemplate string, listConfig string) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ?)", WhereArgs: []interface{}{listConfig}})
	defer logger.ClearVar(&episodes)
	for idxepi := range episodes {
		if !config.ConfigCheck("quality_" + episodes[idxepi].QualityProfile) {
			logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(episodes[idxepi].ID)) + " not found")
			continue
		}
		reached := false
		if parser.GetHighestEpisodePriorityByFiles(episodes[idxepi].ID, configTemplate, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrio(configTemplate, episodes[idxepi].QualityProfile).Priority {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumn("Serie_episodes", "quality_reached", 0, database.Query{Where: "id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumn("Serie_episodes", "quality_reached", 1, database.Query{Where: "id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
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
	filesfound := database.QueryStaticColumnsOneStringNoError("Select location from serie_episode_files where serie_id in (Select id from series where listname = ?)", "Select count(id) from serie_episode_files where serie_id in (Select id from series where listname = ?)", listConfig)
	defer logger.ClearVar(&filesfound)
	swf := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerFiles))

	for idx := range filesfound {
		str := filesfound[idx].Str
		swf.Add(func() {
			jobImportFileCheck(str, "serie")
		})
	}
	swf.Wait()
	swf.Close()
}

func getTraktUserPublicShowList(configTemplate string, listConfig string) (config.MainSerieConfig, error) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if !list.Enabled {
		return config.MainSerieConfig{}, errors.New("list not enabled")
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return config.MainSerieConfig{}, errors.New("list not found")
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)
	if len(cfg_list.TraktUsername) >= 1 && len(cfg_list.TraktListName) >= 1 {
		if len(cfg_list.TraktListType) == 0 {
			cfg_list.TraktListType = "show"
		}
		data, err := apiexternal.TraktApi.GetUserList(cfg_list.TraktUsername, cfg_list.TraktListName, cfg_list.TraktListType, cfg_list.Limit)
		if err != nil {
			logger.Log.Error("Failed to read trakt list: ", cfg_list.TraktListName)
			return config.MainSerieConfig{}, errors.New("list not readable")
		}
		defer logger.ClearVar(&data)
		d := config.MainSerieConfig{}
		defer logger.ClearVar(&d)
		for idx := range data {
			d.Serie = append(d.Serie, config.SerieConfig{
				Name:   data[idx].Serie.Title,
				TvdbID: data[idx].Serie.Ids.Tvdb,
			})
		}
		return d, nil
	}
	return config.MainSerieConfig{}, errors.New("list other error")
}

var lastSeriesStructure string

func seriesStructureSingle(configTemplate string) {

	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for idxpath := range row.DataImport {
		mappathimport := row.DataImport[idxpath].Template_path
		if !config.ConfigCheck("path_" + mappathimport) {
			logger.Log.Errorln("Path not found: ", "path_"+mappathimport)

			continue
		}
		cfg_path_import := config.ConfigGet("path_" + mappathimport).Data.(config.PathsConfig)

		if !config.ConfigCheck("path_" + row.Data[0].Template_path) {
			logger.Log.Errorln("Path not found: ", "path_"+row.Data[0].Template_path)
			continue
		}
		cfg_path := ""
		if len(row.Data) >= 1 {
			cfg_path = "path_" + row.Data[0].Template_path

		}
		if strings.EqualFold(lastSeriesStructure, cfg_path_import.Path) && lastSeriesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = cfg_path_import.Path

		structure.StructureFolders("series", "path_"+mappathimport, cfg_path, configTemplate)

	}
}
