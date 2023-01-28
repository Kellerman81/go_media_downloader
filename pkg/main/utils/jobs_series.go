package utils

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
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

const serieepiunmatched = "SerieEpisode not matched episode - serieepisode not found"
const seriedbunmatched = "SerieEpisode not matched episode - dbserieepisode not found"
const queryrootpathseries = "select rootpath from series where id = ?"
const querycountfilesseries = "select count() from serie_episode_files where location = ? and serie_episode_id = ?"
const queryidseriesbyseriesdbepisode = "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?"
const queryidentifiedbyseries = "select lower(identifiedby) from dbseries where id = ?"
const querycountfilesserieslocation = "select count() from serie_episode_files where location = ?"

var lastSeriesStructure string

func updateRootpath(file string, objtype string, objid uint, cfgp *config.MediaTypeConfig) {
	var rootpath, tempfoldername, firstfolder string
	for idxdata := range cfgp.Data {
		if !config.Check("path_" + cfgp.Data[idxdata].TemplatePath) {
			continue
		}
		if strings.Contains(file, config.Cfg.Paths[cfgp.Data[idxdata].TemplatePath].Path) {
			rootpath = config.Cfg.Paths[cfgp.Data[idxdata].TemplatePath].Path
			tempfoldername = strings.Replace(file, config.Cfg.Paths[cfgp.Data[idxdata].TemplatePath].Path, "", -1)
			tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
			tempfoldername = filepath.Dir(tempfoldername)
			_, firstfolder = logger.Getrootpath(tempfoldername)
			rootpath = filepath.Join(rootpath, firstfolder)
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update " + objtype + " set rootpath = ? where id = ?", Args: []interface{}{rootpath, objid}})
			return
		}
	}

}
func JobImportSeriesParseV2(file string, updatemissing bool, cfgp *config.MediaTypeConfig, listname string) {
	jobImportSeriesParseV2(&importstruct{path: file, updatemissing: updatemissing, cfgp: cfgp, listname: listname})
}

type importstruct struct {
	path          string
	updatemissing bool
	cfgp          *config.MediaTypeConfig
	listname      string
	addfound      bool
}

func (c *importstruct) close() {
	if c == nil {
		return
	}
	c = nil
}

func jobImportSeriesParseV2(imp *importstruct) {
	defer imp.close()

	var counter int
	database.QueryColumn(&database.Querywithargs{QueryString: querycountfilesserieslocation, Args: []interface{}{imp.path}}, &counter)
	if counter >= 1 {
		return
	}
	m := parser.NewFileParser(filepath.Base(imp.path), true, "series")
	defer m.Close()
	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("series", m, imp.cfgp, "", true)
	if m.SerieID != 0 && m.Listname != "" {
		imp.listname = m.Listname
	}

	if imp.listname == "" {
		return
	}

	if m.DbserieID == 0 || m.SerieID == 0 {
		seriesSetUnmatched(m, imp.path, imp.listname)
		return
	}
	var identifiedby string
	if database.QueryColumn(&database.Querywithargs{QueryString: queryidentifiedbyseries, Args: []interface{}{m.DbserieID}}, &identifiedby) != nil {
		return
	}

	episodeArray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
	if episodeArray == nil {
		return
	}
	defer episodeArray.Close()
	lenarray := len(episodeArray.Arr)
	if lenarray == 0 {
		return
	}

	checkfiles := make([]database.DbstaticTwoUint, 0, lenarray)
	if lenarray == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
		checkfiles = append(checkfiles, database.DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID})
		//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", m.DbserieEpisodeID), zap.Uint("serieepisodeid", m.SerieEpisodeID))
	} else {
		var dbserieepisodeid, serieepisodeid uint
		cntseries := database.Querywithargs{QueryString: queryidseriesbyseriesdbepisode}
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
				cntseries.Args = []interface{}{m.SerieID, dbserieepisodeid}
				database.QueryColumn(&cntseries, &serieepisodeid)
				if serieepisodeid != 0 {
					checkfiles = append(checkfiles, database.DbstaticTwoUint{Num1: serieepisodeid, Num2: dbserieepisodeid})
					//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", dbserieepisodeid), zap.Uint("serieepisodeid", serieepisodeid))
				} else {
					logger.Log.GlobalLogger.Debug(serieepiunmatched, zap.Stringp("file", &imp.path))
					seriesSetUnmatched(m, imp.path, imp.listname)
					checkfiles = nil
					return
				}
			} else {
				logger.Log.GlobalLogger.Debug(seriedbunmatched, zap.Stringp("file", &imp.path))
				seriesSetUnmatched(m, imp.path, imp.listname)
				checkfiles = nil
				return
			}
		}
	}
	if len(checkfiles) == 0 {
		seriesSetUnmatched(m, imp.path, imp.listname)
		return
	}

	var reached bool
	parser.GetPriorityMap(m, imp.cfgp, imp.cfgp.ListsMap[imp.listname].TemplateQuality, true, false)
	err := parser.ParseVideoFile(m, imp.path, imp.cfgp, imp.cfgp.ListsMap[imp.listname].TemplateQuality)
	if err != nil {
		logger.Log.GlobalLogger.Error("Parse failed", zap.String("file", imp.path), zap.Error(err))
		return
	}
	if m.Priority >= parser.NewCutoffPrio(imp.cfgp, imp.cfgp.ListsMap[imp.listname].TemplateQuality) {
		reached = true
	}
	basefile := filepath.Base(imp.path)
	extfile := filepath.Ext(imp.path)
	for idx := range checkfiles {
		err = database.QueryColumn(&database.Querywithargs{QueryString: querycountfilesseries, Args: []interface{}{imp.path, checkfiles[idx].Num1}}, &counter)
		if counter != 0 || err != nil {
			continue
		}
		database.InsertNamed("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :height, :width)",
			database.SerieEpisodeFile{
				Location:         imp.path,
				Filename:         basefile,
				Extension:        extfile,
				QualityProfile:   imp.cfgp.ListsMap[imp.listname].TemplateQuality,
				ResolutionID:     m.ResolutionID,
				QualityID:        m.QualityID,
				CodecID:          m.CodecID,
				AudioID:          m.AudioID,
				Proper:           m.Proper,
				Repack:           m.Repack,
				Extended:         m.Extended,
				SerieID:          m.SerieID,
				SerieEpisodeID:   checkfiles[idx].Num1,
				DbserieEpisodeID: checkfiles[idx].Num2,
				DbserieID:        m.DbserieID,
				Height:           m.Height,
				Width:            m.Width})

		if imp.updatemissing {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set missing = ? where id = ?", Args: []interface{}{0, checkfiles[idx].Num1}})
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{reached, checkfiles[idx].Num1}})
			if imp.cfgp.ListsMap[imp.listname].TemplateQuality != "" {
				database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set quality_profile = ? where id = ?", Args: []interface{}{imp.cfgp.ListsMap[imp.listname].TemplateQuality, checkfiles[idx].Num1}})
			}
		}

		database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from serie_file_unmatcheds where filepath = ?", Args: []interface{}{imp.path}})
	}
	var rootpath string
	err = database.QueryColumn(&database.Querywithargs{QueryString: queryrootpathseries, Args: []interface{}{m.SerieID}}, &rootpath)
	if rootpath == "" && m.SerieID != 0 && err == nil {
		updateRootpath(imp.path, "series", m.SerieID, imp.cfgp)
	}
}

func seriesSetUnmatched(m *apiexternal.ParseInfo, file string, listname string) {
	var id uint
	database.QueryColumn(&database.Querywithargs{QueryString: "select id from serie_file_unmatcheds where filepath = ? and listname = ?", Args: []interface{}{file, listname}}, &id)
	if id == 0 {
		database.InsertNamed("Insert into serie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", database.SerieFileUnmatched{Listname: listname, Filepath: file, LastChecked: sql.NullTime{Time: time.Now(), Valid: true}, ParsedData: buildparsedstring(m)})
	} else {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, id}})
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{buildparsedstring(m), id}})
	}
}

func RefreshSerie(id string) {
	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?", id)
}

func RefreshSeries() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0")
}

func RefreshSeriesInc() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}

	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20")
}

func refreshseriesquery(query string, args ...interface{}) {
	var dbseries []database.DbstaticTwoStringOneInt
	database.QueryStaticColumnsTwoStringOneInt(false, 0, &database.Querywithargs{QueryString: query, Args: args}, &dbseries)
	var oldlistname string
	var cfgp config.MediaTypeConfig
	for idxserie := range dbseries {
		logger.Log.GlobalLogger.Info("Refresh Serie ", zap.Int("row", idxserie), zap.Int("row count", len(dbseries)), zap.Int("tvdb", dbseries[idxserie].Num))
		if oldlistname != dbseries[idxserie].Str2 {
			cfgp = config.Cfg.Media[config.FindconfigTemplateOnList("serie_", dbseries[idxserie].Str2)]
			oldlistname = dbseries[idxserie].Str2
		}
		importfeed.JobImportDbSeries(&config.SerieConfig{TvdbID: dbseries[idxserie].Num, Name: dbseries[idxserie].Str1}, &cfgp, dbseries[idxserie].Str2, true, false)
	}
	cfgp.Close()
	dbseries = nil
}

func SeriesAllJobs(job string, force bool) {

	logger.Log.GlobalLogger.Info("Started Jobfor all", zap.Stringp("Job", &job))
	for idx := range config.Cfg.Series {
		SeriesSingleJobs(job, config.Cfg.Series[idx].NamePrefix, "", force)
	}
}

func SeriesSingleJobs(job string, cfgpstr string, listname string, force bool) {

	jobName := job
	cfgp := config.Cfg.Media[cfgpstr]
	defer cfgp.Close()
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

	logger.Log.GlobalLogger.Info(jobstarted, zap.Stringp("Job", &jobName))

	//dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	dbresult, _ := insertjobhistory(&database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	searchmissingIncremental := cfgp.SearchmissingIncremental
	searchupgradeIncremental := cfgp.SearchupgradeIncremental
	if searchmissingIncremental == 0 {
		searchmissingIncremental = 20
	}
	if searchupgradeIncremental == 0 {
		searchupgradeIncremental = 20
	}

	var searchserie, searchtitle, searchmissing bool
	var searchinterval int
	switch job {
	case "datafull":
		getNewFilesMap(&cfgp, "")
	case "rssseasons":
		searcher.SearchSeriesRSSSeasons(cfgpstr)
	case "searchmissingfull":
		searchserie = true
		searchmissing = true
	case "searchmissinginc":
		searchserie = true
		searchmissing = true
		searchinterval = searchmissingIncremental
	case "searchupgradefull":
		searchserie = true
	case "searchupgradeinc":
		searchserie = true
		searchinterval = searchupgradeIncremental
	case "searchmissingfulltitle":
		searchserie = true
		searchmissing = true
		searchtitle = true
	case "searchmissinginctitle":
		searchserie = true
		searchmissing = true
		searchtitle = true
		searchinterval = searchmissingIncremental
	case "searchupgradefulltitle":
		searchserie = true
		searchtitle = true
	case "searchupgradeinctitle":
		searchserie = true
		searchtitle = true
		searchinterval = searchupgradeIncremental
	case "structure":
		seriesStructureSingle(&cfgp)
	}
	if searchserie {
		searcher.SearchSerie(&cfgp, searchmissing, searchinterval, searchtitle)
	}

	if job == "data" || job == "checkmissing" || job == "checkmissingflag" || job == "checkreachedflag" || job == "clearhistory" || job == "feeds" || job == "rss" {
		var lists []config.MediaListsConfig
		if listname != "" {
			lists = []config.MediaListsConfig{cfgp.ListsMap[listname]}
		} else {
			lists = cfgp.Lists
		}
		qualis := logger.InStringArrayStruct{Arr: make([]string, len(lists))}

		for idxlist := range lists {
			qualis.Arr[idxlist] = lists[idxlist].TemplateQuality
			switch job {
			case "data":
				getNewFilesMap(&cfgp, lists[idxlist].Name)
			case "checkmissing":
				checkmissingepisodessingle(lists[idxlist].Name)
			case "checkmissingflag":
				checkmissingepisodesflag(lists[idxlist].Name)
			case "checkreachedflag":
				checkreachedepisodesflag(&cfgp, lists[idxlist].Name)
			case "clearhistory":
				database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{lists[idxlist].Name}})
			case "feeds":
				importnewseriessingle(&cfgp, lists[idxlist].Name)
			default:
				// other stuff
			}
		}
		lists = nil
		if job == "rss" {
			for _, qual := range unique(&qualis).Arr {
				switch job {
				case "rss":
					searcher.SearchSerieRSS(&cfgp, qual)
				}
			}
		}
		qualis.Close()
	}
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	logger.Log.GlobalLogger.Info(jobended, zap.Stringp("Job", &job), zap.Stringp("config", &cfgp.NamePrefix))
}

func importnewseriessingle(cfgp *config.MediaTypeConfig, listname string) {
	logger.Log.GlobalLogger.Info("Get Serie Config ", zap.Stringp("Listname", &listname))
	feed, err := feeds(cfgp, listname)
	if err != nil {
		return
	}
	defer feed.Close()
	if len(feed.Series.Serie) >= 1 {
		workergroup := logger.WorkerPools["Metadata"].Group()
		for idxserie := range feed.Series.Serie {
			serie := feed.Series.Serie[idxserie]
			logger.Log.GlobalLogger.Info("Import Serie ", zap.Int("row", idxserie), zap.Stringp("serie", &serie.Name))
			workergroup.Submit(func() {
				importfeed.JobImportDbSeries(&serie, cfgp, listname, false, true)
			})
		}
		workergroup.Wait()
	}
}

func checkmissingepisodesflag(listname string) {
	var episodes []database.DbstaticOneIntOneBool
	database.QueryStaticColumnsOneIntOneBool(&database.Querywithargs{QueryString: "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)", Args: []interface{}{listname}}, &episodes)
	var counter int
	querycount := "select count() from serie_episode_files where serie_episode_id = ?"
	for idxepi := range episodes {
		database.QueryColumn(&database.Querywithargs{QueryString: querycount, Args: []interface{}{episodes[idxepi].Num}}, &counter)
		if counter >= 1 && episodes[idxepi].Bl {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set missing = ? where id = ?", Args: []interface{}{0, episodes[idxepi].Num}})
			continue
		}
		if counter == 0 && !episodes[idxepi].Bl {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set missing = ? where id = ?", Args: []interface{}{1, episodes[idxepi].Num}})
		}
	}
	episodes = nil
}

func checkreachedepisodesflag(cfgp *config.MediaTypeConfig, listname string) {
	var episodes []database.SerieEpisode
	database.QuerySerieEpisodes(&database.Querywithargs{Query: database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{listname}}, &episodes)
	var reached bool
	for idxepi := range episodes {
		if !config.Check("quality_" + episodes[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for Episode: %d not found", episodes[idxepi].ID))
			continue
		}
		reached = false
		if searcher.GetHighestEpisodePriorityByFiles(false, true, episodes[idxepi].ID, cfgp, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfgp, episodes[idxepi].QualityProfile) {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{0, episodes[idxepi].ID}})
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{1, episodes[idxepi].ID}})
		}
	}
	episodes = nil
}

func checkmissingepisodessingle(listname string) {
	var filesfound []string
	database.QueryStaticStringArray(false, database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", Args: []interface{}{listname}}), &database.Querywithargs{QueryString: "select location from serie_episode_files where serie_id in (Select id from series where listname = ?)", Args: []interface{}{listname}}, &filesfound)
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "serie")
			//})
		}
	}
	filesfound = nil
}

func getTraktUserPublicShowList(cfgplist *config.MediaListsConfig) (*feedResults, error) {
	defer cfgplist.Close()
	if !cfgplist.Enabled {
		return nil, errNoListEnabled
	}
	if !config.Check("list_" + cfgplist.TemplateList) {
		return nil, errNoList
	}
	if config.Cfg.Lists[cfgplist.TemplateList].TraktListType == "" {
		return nil, errors.New("not show")
	}
	if config.Cfg.Lists[cfgplist.TemplateList].TraktUsername == "" || config.Cfg.Lists[cfgplist.TemplateList].TraktListName == "" {
		return nil, errors.New("no user")
	}
	data, err := apiexternal.TraktAPI.GetUserList(config.Cfg.Lists[cfgplist.TemplateList].TraktUsername, config.Cfg.Lists[cfgplist.TemplateList].TraktListName, config.Cfg.Lists[cfgplist.TemplateList].TraktListType, config.Cfg.Lists[cfgplist.TemplateList].Limit)
	if err != nil {
		logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", config.Cfg.Lists[cfgplist.TemplateList].TraktListName))
		return nil, errNoListRead
	}
	defer data.Close()
	var results feedResults
	results.Series.Serie = []config.SerieConfig{}
	for idx := range data.Entries {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: data.Entries[idx].Serie.Title, TvdbID: data.Entries[idx].Serie.Ids.Tvdb,
		})
	}
	return &results, nil
}

func seriesStructureSingle(cfgp *config.MediaTypeConfig) {
	if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
		return
	}

	var mappathimport string
	for idxdata := range cfgp.DataImport {
		mappathimport = cfgp.DataImport[idxdata].TemplatePath
		if !config.Check("path_" + mappathimport) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", mappathimport))

			continue
		}

		if lastSeriesStructure == config.Cfg.Paths[mappathimport].Path {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = config.Cfg.Paths[mappathimport].Path

		structure.OrganizeFolders("series", mappathimport, cfgp.Data[0].TemplatePath, cfgp)
	}
}
