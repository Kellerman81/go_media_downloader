package utils

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"go.uber.org/zap"
)

func updateRootpath(file string, objtype string, objid uint, cfg string) {
	rootpath := ""
	var pppath, tempfoldername, firstfolder string
	for idxdata := range config.Cfg.Media[cfg].Data {
		if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[idxdata].Template_path) {
			continue
		}
		pppath = config.Cfg.Paths[config.Cfg.Media[cfg].Data[idxdata].Template_path].Path
		if strings.Contains(file, pppath) {
			rootpath = pppath
			tempfoldername = strings.Replace(file, pppath, "", -1)
			tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
			tempfoldername = filepath.Dir(tempfoldername)
			_, firstfolder = logger.Getrootpath(tempfoldername)
			rootpath = filepath.Join(rootpath, firstfolder)
			break
		}
	}
	database.UpdateColumnStatic("Update "+objtype+" set rootpath = ? where id = ?", rootpath, objid)

}
func JobImportSeriesParseV2(file string, updatemissing bool, cfg string, listname string) {
	jobImportSeriesParseV2(file, updatemissing, cfg, listname)
}
func jobImportSeriesParseV2(file string, updatemissing bool, cfg string, listname string) bool {
	m, err := parser.NewFileParser(filepath.Base(file), true, "series")
	if err != nil {
		return false
	}
	defer m.Close()

	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("series", m, cfg, "", true)
	if m.SerieID != 0 && m.Listname != "" {
		listname = m.Listname
	}

	if listname == "" {
		return false
	}

	counter, _ := database.CountRowsStatic("select count() from serie_episode_files where location = ?", file)
	if counter >= 1 {
		return false
	}

	addunmatched := false
	if m.DbserieID != 0 && m.SerieID != 0 {
		err = parser.ParseVideoFile(m, file, cfg, config.Cfg.Media[cfg].ListsMap[listname].Template_quality)
		if err != nil {
			logger.Log.GlobalLogger.Error("Parse failed", zap.Error(err))
			return false
		}
		identifiedby, err := database.QueryColumnString("select lower(identifiedby) from dbseries where id = ?", m.DbserieID)
		if err != nil {
			return false
		}

		episodeArray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
		if episodeArray == nil {
			return false
		}
		defer episodeArray.Close()
		if len(episodeArray.Arr) == 0 {
			return false
		}

		checkfiles := make([]database.Dbstatic_TwoInt, 0, len(episodeArray.Arr))
		defer logger.ClearVar(&checkfiles)
		if len(episodeArray.Arr) == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
			checkfiles = append(checkfiles, database.Dbstatic_TwoInt{Num1: int(m.SerieEpisodeID), Num2: int(m.DbserieEpisodeID)})
			logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", m.DbserieEpisodeID), zap.Uint("serieepisodeid", m.SerieEpisodeID))
		} else {
			var dbserieepisodeid, serieepisodeid uint
			for idx := range episodeArray.Arr {
				if episodeArray.Arr[idx] == "" {
					continue
				}
				episodeArray.Arr[idx] = strings.Trim(episodeArray.Arr[idx], "-EX")
				if identifiedby != "date" {
					episodeArray.Arr[idx] = strings.TrimLeft(episodeArray.Arr[idx], "0")
				}

				dbserieepisodeid, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, m.Identifier, m.SeasonStr, episodeArray.Arr[idx])
				if dbserieepisodeid != 0 {
					serieepisodeid, _ = database.QueryColumnUint("select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?", m.SerieID, dbserieepisodeid)
					if serieepisodeid != 0 {
						checkfiles = append(checkfiles, database.Dbstatic_TwoInt{Num1: int(serieepisodeid), Num2: int(dbserieepisodeid)})
						logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", dbserieepisodeid), zap.Uint("serieepisodeid", serieepisodeid))
					} else {
						addunmatched = true
						logger.Log.GlobalLogger.Debug("SerieEpisode not matched episode - serieepisode not found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier))
					}
				} else {
					addunmatched = true
					logger.Log.GlobalLogger.Debug("SerieEpisode not matched episode - dbserieepisode not found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier))
				}
			}
		}
		if len(checkfiles) >= 1 {
			reached := false
			parser.GetPriorityMap(m, cfg, config.Cfg.Media[cfg].ListsMap[listname].Template_quality, true)
			if m.Priority >= parser.NewCutoffPrio(cfg, config.Cfg.Media[cfg].ListsMap[listname].Template_quality) {
				reached = true
			}
			basefile := filepath.Base(file)
			extfile := filepath.Ext(file)
			added := false
			for idx := range checkfiles {
				counter, err = database.CountRowsStatic("select count() from serie_episode_files where location = ? and serie_episode_id = ?", file, checkfiles[idx].Num1)
				if counter == 0 && err == nil {
					database.InsertNamed("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :height, :width)",
						database.SerieEpisodeFile{
							Location:         file,
							Filename:         basefile,
							Extension:        extfile,
							QualityProfile:   config.Cfg.Media[cfg].ListsMap[listname].Template_quality,
							ResolutionID:     m.ResolutionID,
							QualityID:        m.QualityID,
							CodecID:          m.CodecID,
							AudioID:          m.AudioID,
							Proper:           m.Proper,
							Repack:           m.Repack,
							Extended:         m.Extended,
							SerieID:          m.SerieID,
							SerieEpisodeID:   uint(checkfiles[idx].Num1),
							DbserieEpisodeID: uint(checkfiles[idx].Num2),
							DbserieID:        m.DbserieID,
							Height:           m.Height,
							Width:            m.Width})

					if updatemissing {
						database.UpdateColumnStatic("Update serie_episodes set missing = ? where id = ?", 0, checkfiles[idx].Num1)
						database.UpdateColumnStatic("Update serie_episodes set quality_reached = ? where id = ?", reached, checkfiles[idx].Num1)
						if config.Cfg.Media[cfg].ListsMap[listname].Template_quality != "" {
							database.UpdateColumnStatic("Update serie_episodes set quality_profile = ? where id = ?", config.Cfg.Media[cfg].ListsMap[listname].Template_quality, checkfiles[idx].Num1)
						}
					}

					database.DeleteRowStatic("Delete from serie_file_unmatcheds where filepath = ?", file)
					added = true
				} else {
					added = true
				}
			}
			rootpath, err := database.QueryColumnString("select rootpath from series where id = ?", m.SerieID)
			if rootpath == "" && m.SerieID != 0 && err == nil {
				updateRootpath(file, "series", m.SerieID, cfg)
			}
			if added {
				return true
			}
		} else if !addunmatched {
			addunmatched = true
		}
	} else {
		addunmatched = true
	}

	if addunmatched {
		logger.Log.GlobalLogger.Debug("SerieEpisode not matched", zap.String("file", file), zap.String("title", m.Title))
		mjson := ""
		if m.AudioID != 0 {
			mjson += " Audioid: " + strconv.FormatUint(uint64(m.AudioID), 10)
		}
		if m.CodecID != 0 {
			mjson += " Codecid: " + strconv.FormatUint(uint64(m.CodecID), 10)
		}
		if m.QualityID != 0 {
			mjson += " Qualityid: " + strconv.FormatUint(uint64(m.QualityID), 10)
		}
		if m.ResolutionID != 0 {
			mjson += " Resolutionid: " + strconv.FormatUint(uint64(m.ResolutionID), 10)
		}
		if m.EpisodeStr != "" {
			mjson += " Episode: " + m.EpisodeStr
		}
		if m.Identifier != "" {
			mjson += " Identifier: " + m.Identifier
		}
		if m.Listname != "" {
			mjson += " Listname: " + m.Listname
		}
		if m.SeasonStr != "" {
			mjson += " Season: " + m.SeasonStr
		}
		if m.Title != "" {
			mjson += " Title: " + m.Title
		}
		if m.Tvdb != "" {
			mjson += " Tvdb: " + m.Tvdb
		}

		id, _ := database.QueryColumnUint("select id from serie_file_unmatcheds where filepath = ? and listname = ?", file, listname)
		if id == 0 {
			database.InsertNamed("Insert into serie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", database.SerieFileUnmatched{Listname: listname, Filepath: file, LastChecked: sql.NullTime{Time: time.Now(), Valid: true}, ParsedData: mjson})
		} else {
			database.UpdateColumnStatic("Update serie_file_unmatcheds SET last_checked = ? where id = ?", sql.NullTime{Time: time.Now(), Valid: true}, id)
			database.UpdateColumnStatic("Update serie_file_unmatcheds SET parsed_data = ? where id = ?", mjson, id)
		}
	}
	return false
}

func RefreshSerie(id string) {
	refreshseriesquery("select seriename as str, thetvdb_id as num from dbseries where id = ?", id)
}

func RefreshSeries() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshseriesquery("select seriename as str, thetvdb_id as num from dbseries where thetvdb_id != 0")
}

func RefreshSeriesInc() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}

	refreshseriesquery("select seriename as str, thetvdb_id as num from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20")
}

func refreshseriesquery(query string, args ...interface{}) {
	dbseries, _ := database.QueryStaticColumnsOneStringOneInt(query, false, 0, args...)

	for idxserie := range dbseries {
		logger.Log.GlobalLogger.Info("Refresh Serie ", zap.Int("row", idxserie), zap.Int("row count", len(dbseries)), zap.Int("tvdb", dbseries[idxserie].Num))
		listname, _ := database.QueryColumnString("select listname from series where dbserie_id in (select id from dbseries where thetvdb_id = ?)", dbseries[idxserie].Num)

		importfeed.JobImportDbSeries(&config.SerieConfig{TvdbID: dbseries[idxserie].Num, Name: dbseries[idxserie].Str}, config.FindconfigTemplateOnList("serie_", listname), listname, true, false)
	}
}

func Series_all_jobs(job string, force bool) {

	logger.Log.GlobalLogger.Info("Started Jobfor all", zap.String("Job", job))
	for idx := range config.Cfg.Series {
		Series_single_jobs(job, "serie_"+config.Cfg.Series[idx].Name, "", force)
	}
}

func Series_single_jobs(job string, cfg string, listname string, force bool) {

	jobName := job
	if cfg != "" {
		jobName += "_" + cfg
	}
	if listname != "" {
		jobName += "_" + listname
	}

	if config.Cfg.General.SchedulerDisabled && !force {
		logger.Log.GlobalLogger.Info("Skipped Job", zap.String("Job", job), zap.String("config", cfg))
		return
	}

	logger.Log.GlobalLogger.Info("Started Job", zap.String("Job", jobName))

	dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	lists := config.Cfg.Media[cfg].Lists
	searchmissing_incremental := config.Cfg.Media[cfg].Searchmissing_incremental
	searchupgrade_incremental := config.Cfg.Media[cfg].Searchupgrade_incremental
	if searchmissing_incremental == 0 {
		searchmissing_incremental = 20
	}
	if searchupgrade_incremental == 0 {
		searchupgrade_incremental = 20
	}

	switch job {
	case "datafull":
		getNewFilesMap(cfg, "series", "")
	case "rssseasons":
		searcher.SearchSeriesRSSSeasons(cfg)
	case "searchmissingfull":
		searcher.SearchSerieMissing(cfg, 0, false)
	case "searchmissinginc":
		searcher.SearchSerieMissing(cfg, searchmissing_incremental, false)
	case "searchupgradefull":
		searcher.SearchSerieUpgrade(cfg, 0, false)
	case "searchupgradeinc":
		searcher.SearchSerieUpgrade(cfg, searchupgrade_incremental, false)
	case "searchmissingfulltitle":
		searcher.SearchSerieMissing(cfg, 0, true)
	case "searchmissinginctitle":
		searcher.SearchSerieMissing(cfg, searchmissing_incremental, true)
	case "searchupgradefulltitle":
		searcher.SearchSerieUpgrade(cfg, 0, true)
	case "searchupgradeinctitle":
		searcher.SearchSerieUpgrade(cfg, searchupgrade_incremental, true)
	case "structure":
		seriesStructureSingle(cfg)

	}
	if listname != "" {
		lists = []config.MediaListsConfig{config.Cfg.Media[cfg].ListsMap[listname]}
	}
	var qualis []string = make([]string, len(lists))

	for idxlist := range lists {
		qualis[idxlist] = lists[idxlist].Template_quality
		switch job {
		case "data":
			getNewFilesMap(cfg, "series", lists[idxlist].Name)
		case "checkmissing":
			checkmissingepisodessingle(cfg, lists[idxlist].Name)
		case "checkmissingflag":
			checkmissingepisodesflag(cfg, lists[idxlist].Name)
		case "checkreachedflag":
			checkreachedepisodesflag(cfg, lists[idxlist].Name)
		case "clearhistory":
			database.DeleteRow("serie_episode_histories", &database.Query{Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, lists[idxlist].Name)
		case "feeds":
			importnewseriessingle(cfg, lists[idxlist].Name)
		default:
			// other stuff
		}
	}
	unique := unique(&logger.InStringArrayStruct{Arr: qualis})
	for idxuni := range unique {
		switch job {
		case "rss":
			searcher.SearchSerieRSS(cfg, unique[idxuni])
		}
	}
	unique = nil
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)
	}
	logger.Log.GlobalLogger.Info("Ended Job", zap.String("Job", job), zap.String("config", cfg))

}

func importnewseriessingle(cfg string, listname string) {
	logger.Log.GlobalLogger.Info("Get Serie Config ", zap.String("Listname", listname))
	feed, err := feeds(cfg, listname)
	if err != nil {
		return
	}
	defer feed.Close()
	if len(feed.Series.Serie) >= 1 {
		workergroup := logger.WorkerPools["Metadata"].Group()
		for idxserie := range feed.Series.Serie {
			serie := feed.Series.Serie[idxserie]
			logger.Log.GlobalLogger.Info("Import Serie ", zap.Int("row", idxserie), zap.String("serie", serie.Name))
			workergroup.Submit(func() {
				importfeed.JobImportDbSeries(&serie, cfg, listname, false, true)
			})
		}
		workergroup.Wait()
	}
}

func checkmissingepisodesflag(cfg string, listname string) {
	episodes, _ := database.QueryStaticColumnsOneIntOneBool("select id as num, missing as bl from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)", listname)
	var counter int
	var err error
	for idxepi := range episodes {
		counter, err = database.CountRowsStatic("select count() from serie_episode_files where serie_episode_id = ?", episodes[idxepi].Num)
		if counter >= 1 {
			if episodes[idxepi].Bl {
				database.UpdateColumnStatic("Update Serie_episodes set missing = ? where id = ?", 0, episodes[idxepi].Num)
			}
		} else {
			if !episodes[idxepi].Bl && err == nil {
				database.UpdateColumnStatic("Update Serie_episodes set missing = ? where id = ?", 1, episodes[idxepi].Num)
			}
		}
	}
	episodes = nil
}

func checkreachedepisodesflag(cfg string, listname string) {
	episodes, _ := database.QuerySerieEpisodes(&database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, listname)
	var reached bool
	for idxepi := range episodes {
		if !config.ConfigCheck("quality_" + episodes[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error("Quality for Episode: " + strconv.Itoa(int(episodes[idxepi].ID)) + " not found")
			continue
		}
		reached = false
		if searcher.GetHighestEpisodePriorityByFiles(episodes[idxepi].ID, cfg, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfg, episodes[idxepi].QualityProfile) {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumnStatic("Update Serie_episodes set quality_reached = ? where id = ?", 0, episodes[idxepi].ID)
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumnStatic("Update Serie_episodes set quality_reached = ? where id = ?", 1, episodes[idxepi].ID)
		}
	}
	episodes = nil
}

func checkmissingepisodessingle(cfg string, listname string) {
	filesfound := database.QueryStaticStringArray("select location as str from serie_episode_files where serie_id in (Select id from series where listname = ?)", false, database.CountRowsStaticNoError("select location as str from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", listname), listname)
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "serie")
			//})
		}
	}
	filesfound = nil
}

func getTraktUserPublicShowList(cfg string, listname string) (*config.MainSerieConfig, error) {
	if !config.Cfg.Media[cfg].ListsMap[listname].Enabled {
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + config.Cfg.Media[cfg].ListsMap[listname].Template_list) {
		return nil, errNoList
	}
	if len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktUsername) >= 1 && len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName) >= 1 {
		if len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListType) == 0 {
			return nil, errors.New("not show")
		}
		data, err := apiexternal.TraktApi.GetUserList(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktUsername, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListType, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Limit)
		if err != nil {
			logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName))
			return nil, errNoListRead
		}
		var results config.MainSerieConfig

		results.Serie = logger.CopyFunc(data.Entries, func(elem apiexternal.TraktUserList) config.SerieConfig {
			return config.SerieConfig{
				Name: elem.Serie.Title, TvdbID: elem.Serie.Ids.Tvdb,
			}
		})
		data = nil
		return &results, nil
	}
	return nil, errNoListOther
}

func seriesStructureSingle(cfg string) {
	if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[0].Template_path) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", config.Cfg.Media[cfg].Data[0].Template_path))
		return
	}
	var lastSeriesStructure *cache.CacheReturn
	var ok bool

	var mappathimport string
	for idxdata := range config.Cfg.Media[cfg].DataImport {
		mappathimport = config.Cfg.Media[cfg].DataImport[idxdata].Template_path
		if !config.ConfigCheck("path_" + mappathimport) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", mappathimport))

			continue
		}

		lastSeriesStructure, ok = logger.GlobalCache.Get("lastSeriesStructure")
		if ok {
			if lastSeriesStructure.Value.(string) == config.Cfg.Paths[mappathimport].Path {
				time.Sleep(time.Duration(15) * time.Second)
			}
		}

		logger.GlobalCache.Set("lastSeriesStructure", config.Cfg.Paths[mappathimport].Path, 5*time.Minute)
		structure.StructureFolders("series", mappathimport, config.Cfg.Media[cfg].Data[0].Template_path, cfg)

	}
	lastSeriesStructure = nil
}
