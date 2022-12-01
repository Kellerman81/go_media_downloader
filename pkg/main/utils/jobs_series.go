package utils

import (
	"database/sql"
	"errors"
	"path/filepath"
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
	"go.uber.org/zap"
)

func updateRootpath(file string, objtype string, objid uint, cfgp *config.MediaTypeConfig) {
	rootpath := ""
	var pppath, tempfoldername, firstfolder string
	for idxdata := range cfgp.Data {
		if !config.ConfigCheck("path_" + cfgp.Data[idxdata].TemplatePath) {
			continue
		}
		pppath = config.Cfg.Paths[cfgp.Data[idxdata].TemplatePath].Path
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
	database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update " + objtype + " set rootpath = ? where id = ?", Args: []interface{}{rootpath, objid}})

}
func JobImportSeriesParseV2(file string, updatemissing bool, cfgp *config.MediaTypeConfig, listname string) {
	jobImportSeriesParseV2(file, updatemissing, cfgp, listname)
}

const serieepiunmatched string = "SerieEpisode not matched episode - serieepisode not found"
const seriedbunmatched string = "SerieEpisode not matched episode - dbserieepisode not found"
const serieunmatched string = "SerieEpisode not matched"

func jobImportSeriesParseV2(file string, updatemissing bool, cfgp *config.MediaTypeConfig, listname string) bool {
	m, err := parser.NewFileParser(filepath.Base(file), true, "series")
	if err != nil {
		return false
	}
	defer m.Close()

	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("series", m, cfgp, "", true)
	if m.SerieID != 0 && m.Listname != "" {
		listname = m.Listname
	}

	if listname == "" {
		return false
	}

	counter, _ := database.CountRowsStatic(database.Querywithargs{QueryString: "select count() from serie_episode_files where location = ?", Args: []interface{}{file}})
	if counter >= 1 {
		return false
	}

	addunmatched := false
	if m.DbserieID != 0 && m.SerieID != 0 {
		err = parser.ParseVideoFile(m, file, cfgp, cfgp.ListsMap[listname].TemplateQuality)
		if err != nil {
			logger.Log.GlobalLogger.Error("Parse failed", zap.Error(err))
			return false
		}
		identifiedby, err := database.QueryColumnString(database.Querywithargs{QueryString: "select lower(identifiedby) from dbseries where id = ?", Args: []interface{}{m.DbserieID}})
		if err != nil {
			return false
		}

		episodeArray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
		if episodeArray == nil {
			return false
		}
		defer episodeArray.Close()
		lenarray := len(episodeArray.Arr)
		if lenarray == 0 {
			return false
		}

		checkfiles := make([]database.Dbstatic_TwoInt, 0, lenarray)
		defer logger.ClearVar(&checkfiles)
		if lenarray == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
			checkfiles = append(checkfiles, database.Dbstatic_TwoInt{Num1: int(m.SerieEpisodeID), Num2: int(m.DbserieEpisodeID)})
			//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", m.DbserieEpisodeID), zap.Uint("serieepisodeid", m.SerieEpisodeID))
		} else {
			var dbserieepisodeid, serieepisodeid uint
			countseries := "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?"
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
					serieepisodeid, _ = database.QueryColumnUint(database.Querywithargs{QueryString: countseries, Args: []interface{}{m.SerieID, dbserieepisodeid}})
					if serieepisodeid != 0 {
						checkfiles = append(checkfiles, database.Dbstatic_TwoInt{Num1: int(serieepisodeid), Num2: int(dbserieepisodeid)})
						//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", dbserieepisodeid), zap.Uint("serieepisodeid", serieepisodeid))
					} else {
						addunmatched = true
						logger.Log.GlobalLogger.Debug(serieepiunmatched, zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier))
					}
				} else {
					addunmatched = true
					logger.Log.GlobalLogger.Debug(seriedbunmatched, zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier))
				}
			}
		}
		if len(checkfiles) >= 1 {
			reached := false
			parser.GetPriorityMap(m, cfgp, cfgp.ListsMap[listname].TemplateQuality, true, false)
			if m.Priority >= parser.NewCutoffPrio(cfgp, cfgp.ListsMap[listname].TemplateQuality) {
				reached = true
			}
			basefile := filepath.Base(file)
			extfile := filepath.Ext(file)
			added := false
			countquery := "select count() from serie_episode_files where location = ? and serie_episode_id = ?"
			for idx := range checkfiles {
				counter, err = database.CountRowsStatic(database.Querywithargs{QueryString: countquery, Args: []interface{}{file, checkfiles[idx].Num1}})
				if counter == 0 && err == nil {
					database.InsertNamed("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :height, :width)",
						database.SerieEpisodeFile{
							Location:         file,
							Filename:         basefile,
							Extension:        extfile,
							QualityProfile:   cfgp.ListsMap[listname].TemplateQuality,
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
						database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_episodes set missing = ? where id = ?", Args: []interface{}{0, checkfiles[idx].Num1}})
						database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{reached, checkfiles[idx].Num1}})
						if cfgp.ListsMap[listname].TemplateQuality != "" {
							database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_episodes set quality_profile = ? where id = ?", Args: []interface{}{cfgp.ListsMap[listname].TemplateQuality, checkfiles[idx].Num1}})
						}
					}

					database.DeleteRowStatic(database.Querywithargs{QueryString: "Delete from serie_file_unmatcheds where filepath = ?", Args: []interface{}{file}})
					added = true
				} else {
					added = true
				}
			}
			rootpath, err := database.QueryColumnString(database.Querywithargs{QueryString: "select rootpath from series where id = ?", Args: []interface{}{m.SerieID}})
			if rootpath == "" && m.SerieID != 0 && err == nil {
				updateRootpath(file, "series", m.SerieID, cfgp)
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
		//logger.Log.GlobalLogger.Debug(serieunmatched, zap.String("file", file), zap.String("title", m.Title))
		var bld strings.Builder
		defer bld.Reset()
		if m.AudioID != 0 {
			bld.WriteString(" Audioid: " + strconv.FormatUint(uint64(m.AudioID), 10))
		}
		if m.CodecID != 0 {
			bld.WriteString(" Codecid: " + strconv.FormatUint(uint64(m.CodecID), 10))
		}
		if m.QualityID != 0 {
			bld.WriteString(" Qualityid: " + strconv.FormatUint(uint64(m.QualityID), 10))
		}
		if m.ResolutionID != 0 {
			bld.WriteString(" Resolutionid: " + strconv.FormatUint(uint64(m.ResolutionID), 10))
		}
		if m.EpisodeStr != "" {
			bld.WriteString(" Episode: " + m.EpisodeStr)
		}
		if m.Identifier != "" {
			bld.WriteString(" Identifier: " + m.Identifier)
		}
		if m.Listname != "" {
			bld.WriteString(" Listname: " + m.Listname)
		}
		if m.SeasonStr != "" {
			bld.WriteString(" Season: " + m.SeasonStr)
		}
		if m.Title != "" {
			bld.WriteString(" Title: " + m.Title)
		}
		if m.Tvdb != "" {
			bld.WriteString(" Tvdb: " + m.Tvdb)
		}

		id, _ := database.QueryColumnUint(database.Querywithargs{QueryString: "select id from serie_file_unmatcheds where filepath = ? and listname = ?", Args: []interface{}{file, listname}})
		if id == 0 {
			database.InsertNamed("Insert into serie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", database.SerieFileUnmatched{Listname: listname, Filepath: file, LastChecked: sql.NullTime{Time: time.Now(), Valid: true}, ParsedData: bld.String()})
		} else {
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, id}})
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{bld.String(), id}})
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
	dbseries, _ := database.QueryStaticColumnsOneStringOneInt(false, 0, database.Querywithargs{QueryString: query, Args: args})

	querylist := "select listname from series where dbserie_id in (select id from dbseries where thetvdb_id = ?)"
	var listname string
	var cfgp config.MediaTypeConfig
	for idxserie := range dbseries {
		logger.Log.GlobalLogger.Info("Refresh Serie ", zap.Int("row", idxserie), zap.Int("row count", len(dbseries)), zap.Int("tvdb", dbseries[idxserie].Num))
		listname, _ = database.QueryColumnString(database.Querywithargs{QueryString: querylist, Args: []interface{}{dbseries[idxserie].Num}})
		cfgp = config.Cfg.Media[config.FindconfigTemplateOnList("serie_", listname)]
		importfeed.JobImportDbSeries(&config.SerieConfig{TvdbID: dbseries[idxserie].Num, Name: dbseries[idxserie].Str}, &cfgp, listname, true, false)
	}
	args = nil
	dbseries = nil
}

func SeriesAllJobs(job string, force bool) {

	logger.Log.GlobalLogger.Info("Started Jobfor all", zap.String("Job", job))
	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Series {
		cfgp = config.Cfg.Media["serie_"+config.Cfg.Series[idx].Name]
		SeriesSingleJobs(job, &cfgp, "", force)
	}
}

func SeriesSingleJobs(job string, cfgp *config.MediaTypeConfig, listname string, force bool) {

	jobName := job
	if cfgp.Name != "" {
		jobName += "_" + cfgp.NamePrefix
	}
	if listname != "" {
		jobName += "_" + listname
	}

	if config.Cfg.General.SchedulerDisabled && !force {
		logger.Log.GlobalLogger.Info("Skipped Job", zap.String("Job", job), zap.String("config", cfgp.NamePrefix))
		return
	}

	logger.Log.GlobalLogger.Info(jobstarted, zap.String("Job", jobName))

	//dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	dbresult, _ := insertjobhistory(database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	lists := cfgp.Lists
	searchmissingIncremental := cfgp.SearchmissingIncremental
	searchupgradeIncremental := cfgp.SearchupgradeIncremental
	if searchmissingIncremental == 0 {
		searchmissingIncremental = 20
	}
	if searchupgradeIncremental == 0 {
		searchupgradeIncremental = 20
	}

	switch job {
	case "datafull":
		getNewFilesMap(cfgp, "series", "")
	case "rssseasons":
		searcher.SearchSeriesRSSSeasons(cfgp)
	case "searchmissingfull":
		searcher.SearchSerieMissing(cfgp, 0, false)
	case "searchmissinginc":
		searcher.SearchSerieMissing(cfgp, searchmissingIncremental, false)
	case "searchupgradefull":
		searcher.SearchSerieUpgrade(cfgp, 0, false)
	case "searchupgradeinc":
		searcher.SearchSerieUpgrade(cfgp, searchupgradeIncremental, false)
	case "searchmissingfulltitle":
		searcher.SearchSerieMissing(cfgp, 0, true)
	case "searchmissinginctitle":
		searcher.SearchSerieMissing(cfgp, searchmissingIncremental, true)
	case "searchupgradefulltitle":
		searcher.SearchSerieUpgrade(cfgp, 0, true)
	case "searchupgradeinctitle":
		searcher.SearchSerieUpgrade(cfgp, searchupgradeIncremental, true)
	case "structure":
		seriesStructureSingle(cfgp)

	}
	if listname != "" {
		lists = []config.MediaListsConfig{cfgp.ListsMap[listname]}
	}
	var qualis []string = make([]string, len(lists))

	for idxlist := range lists {
		qualis[idxlist] = lists[idxlist].TemplateQuality
		switch job {
		case "data":
			getNewFilesMap(cfgp, "series", lists[idxlist].Name)
		case "checkmissing":
			checkmissingepisodessingle(cfgp, lists[idxlist].Name)
		case "checkmissingflag":
			checkmissingepisodesflag(cfgp, lists[idxlist].Name)
		case "checkreachedflag":
			checkreachedepisodesflag(cfgp, lists[idxlist].Name)
		case "clearhistory":
			database.DeleteRow("serie_episode_histories", database.Querywithargs{Query: database.Query{Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{lists[idxlist].Name}})
		case "feeds":
			importnewseriessingle(cfgp, lists[idxlist].Name)
		default:
			// other stuff
		}
	}
	lists = nil
	unique := unique(&logger.InStringArrayStruct{Arr: qualis})
	for idxuni := range unique {
		switch job {
		case "rss":
			searcher.SearchSerieRSS(cfgp, unique[idxuni])
		}
	}
	unique = nil
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(uint(dbid))
	}
	logger.Log.GlobalLogger.Info(jobended, zap.String("Job", job), zap.String("config", cfgp.NamePrefix))

}

func importnewseriessingle(cfgp *config.MediaTypeConfig, listname string) {
	logger.Log.GlobalLogger.Info("Get Serie Config ", zap.String("Listname", listname))
	feed, err := feeds(cfgp, listname)
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
				importfeed.JobImportDbSeries(&serie, cfgp, listname, false, true)
			})
		}
		workergroup.Wait()
	}
}

func checkmissingepisodesflag(cfgp *config.MediaTypeConfig, listname string) {
	episodes, _ := database.QueryStaticColumnsOneIntOneBool(database.Querywithargs{QueryString: "select id as num, missing as bl from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)", Args: []interface{}{listname}})
	var counter int
	var err error
	querycount := "select count() from serie_episode_files where serie_episode_id = ?"
	for idxepi := range episodes {
		counter, err = database.CountRowsStatic(database.Querywithargs{QueryString: querycount, Args: []interface{}{episodes[idxepi].Num}})
		if counter >= 1 {
			if episodes[idxepi].Bl {
				database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update Serie_episodes set missing = ? where id = ?", Args: []interface{}{0, episodes[idxepi].Num}})
			}
		} else {
			if !episodes[idxepi].Bl && err == nil {
				database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update Serie_episodes set missing = ? where id = ?", Args: []interface{}{1, episodes[idxepi].Num}})
			}
		}
	}
	episodes = nil
}

func checkreachedepisodesflag(cfgp *config.MediaTypeConfig, listname string) {
	episodes, _ := database.QuerySerieEpisodes(database.Querywithargs{Query: database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{listname}})
	var reached bool
	for idxepi := range episodes {
		if !config.ConfigCheck("quality_" + episodes[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error("Quality for Episode: " + strconv.Itoa(int(episodes[idxepi].ID)) + " not found")
			continue
		}
		reached = false
		if searcher.GetHighestEpisodePriorityByFiles(episodes[idxepi].ID, cfgp, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfgp, episodes[idxepi].QualityProfile) {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{0, episodes[idxepi].ID}})
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{1, episodes[idxepi].ID}})
		}
	}
	episodes = nil
}

func checkmissingepisodessingle(cfgp *config.MediaTypeConfig, listname string) {
	filesfound := database.QueryStaticStringArray(false, database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select location as str from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", Args: []interface{}{listname}}), database.Querywithargs{QueryString: "select location as str from serie_episode_files where serie_id in (Select id from series where listname = ?)", Args: []interface{}{listname}})
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "serie")
			//})
		}
	}
	filesfound = nil
}

func getTraktUserPublicShowList(cfgplist *config.MediaListsConfig) (*config.MainSerieConfig, error) {
	if !cfgplist.Enabled {
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + cfgplist.TemplateList) {
		return nil, errNoList
	}
	if config.Cfg.Lists[cfgplist.TemplateList].TraktUsername != "" && config.Cfg.Lists[cfgplist.TemplateList].TraktListName != "" {
		if config.Cfg.Lists[cfgplist.TemplateList].TraktListType == "" {
			return nil, errors.New("not show")
		}
		data, err := apiexternal.TraktApi.GetUserList(config.Cfg.Lists[cfgplist.TemplateList].TraktUsername, config.Cfg.Lists[cfgplist.TemplateList].TraktListName, config.Cfg.Lists[cfgplist.TemplateList].TraktListType, config.Cfg.Lists[cfgplist.TemplateList].Limit)
		if err != nil {
			logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", config.Cfg.Lists[cfgplist.TemplateList].TraktListName))
			return nil, errNoListRead
		}
		results := new(config.MainSerieConfig)

		results.Serie = logger.CopyFunc(data.Entries, func(elem apiexternal.TraktUserList) config.SerieConfig {
			return config.SerieConfig{
				Name: elem.Serie.Title, TvdbID: elem.Serie.Ids.Tvdb,
			}
		})
		data = nil
		return results, nil
	}
	return nil, errNoListOther
}

var lastSeriesStructure string

func seriesStructureSingle(cfgp *config.MediaTypeConfig) {
	if !config.ConfigCheck("path_" + cfgp.Data[0].TemplatePath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
		return
	}

	var mappathimport string
	for idxdata := range cfgp.DataImport {
		mappathimport = cfgp.DataImport[idxdata].TemplatePath
		if !config.ConfigCheck("path_" + mappathimport) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", mappathimport))

			continue
		}

		if lastSeriesStructure == config.Cfg.Paths[mappathimport].Path {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = config.Cfg.Paths[mappathimport].Path

		structure.StructureFolders("series", mappathimport, cfgp.Data[0].TemplatePath, cfgp)
	}
}
