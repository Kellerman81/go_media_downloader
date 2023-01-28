package utils

import (
	"bytes"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

type feedResults struct {
	Series config.MainSerieConfig
	Movies []string
}

const countmovies = "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"

var errNoList = errors.New("list not found")
var errNoListEnabled = errors.New("list not enabled")
var errNoListRead = errors.New("list not readable")
var errNoListOther = errors.New("list other error")
var errNoConfig = errors.New("config not found")
var errwrongtype = errors.New("wrong type")

func jobImportFileCheck(file string, dbtype string) {
	if !scanner.CheckFileExist(file) {
		query := "select id, serie_episode_id from serie_episode_files where location = ?"
		subquerycount := "select count() from serie_episode_files where serie_episode_id = ?"
		table := "serie_episode_files"
		updatetable := "serie_episodes"
		if dbtype == "movie" {
			query = "select id, movie_id from movie_files where location = ?"
			subquerycount = "select count() from movie_files where movie_id = ?"
			table = "movie_files"
			updatetable = "movies"
		}
		var files []database.DbstaticTwoInt
		database.QueryStaticColumnsTwoInt(&database.Querywithargs{QueryString: query, Args: []interface{}{file}}, &files)
		var err error
		var counter int
		for idx := range files {
			logger.Log.GlobalLogger.Debug("File was removed", zap.Stringp("file", &file))
			err = database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from " + table + " where id = ?", Args: []interface{}{files[idx].Num1}})
			if err == nil {
				err = database.QueryColumn(&database.Querywithargs{QueryString: subquerycount, Args: []interface{}{files[idx].Num2}}, &counter)
				if counter == 0 && err == nil {
					database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update " + updatetable + " set missing = ? where id = ?", Args: []interface{}{1, files[idx].Num2}})
				}
			}
		}
		files = nil
	}
}

func insertjobhistory(job *database.JobHistory) (sql.Result, error) {
	defer logger.ClearVar(job)
	return database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", job)
}
func endjobhistory(id int64) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update job_histories set ended = ? where id = ?", Args: []interface{}{time.Now().In(logger.TimeZone), id}})
}

func InitialFillSeries() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for series")

	var dbresult sql.Result
	var dbid int64
	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Series {
		cfgp = config.Cfg.Series[idx]
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewseriessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)

	}
	for idx := range config.Cfg.Series {
		cfgp = config.Cfg.Series[idx]
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	cfgp.Close()
}

func InitialFillMovies() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for movies")

	FillImdb()

	var dbresult sql.Result
	var dbid int64

	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Movies {
		cfgp = config.Cfg.Movies[idx]
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewmoviessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)

	}

	for idx := range config.Cfg.Movies {
		cfgp = config.Cfg.Movies[idx]
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	cfgp.Close()
}

func FillImdb() {
	group := logger.WorkerPools["Files"].Group()
	group.Submit(func() { fillimdb() })
	group.Wait()
}
func buildparsedstring(m *apiexternal.ParseInfo) string {
	var bld strings.Builder
	bld.Grow(200)
	defer bld.Reset()
	if m.AudioID != 0 {
		bld.WriteString(" Audioid: ")
		bld.WriteString(logger.UintToString(m.AudioID))
	}
	if m.CodecID != 0 {
		bld.WriteString(" Codecid: ")
		bld.WriteString(logger.UintToString(m.CodecID))
	}
	if m.QualityID != 0 {
		bld.WriteString(" Qualityid: ")
		bld.WriteString(logger.UintToString(m.QualityID))
	}
	if m.ResolutionID != 0 {
		bld.WriteString(" Resolutionid: ")
		bld.WriteString(logger.UintToString(m.ResolutionID))
	}
	if m.EpisodeStr != "" {
		bld.WriteString(" Episode: ")
		bld.WriteString(m.EpisodeStr)
	}
	if m.Identifier != "" {
		bld.WriteString(" Identifier: ")
		bld.WriteString(m.Identifier)
	}
	if m.Listname != "" {
		bld.WriteString(" Listname: ")
		bld.WriteString(m.Listname)
	}
	if m.SeasonStr != "" {
		bld.WriteString(" Season: ")
		bld.WriteString(m.SeasonStr)
	}
	if m.Title != "" {
		bld.WriteString(" Title: ")
		bld.WriteString(m.Title)
	}
	if m.Tvdb != "" {
		bld.WriteString(" Tvdb: ")
		bld.WriteString(m.Tvdb)
	}
	if m.Imdb != "" {
		bld.WriteString(" Imdb: ")
		bld.WriteString(m.Imdb)
	}
	if m.Year != 0 {
		bld.WriteString(" Year: ")
		bld.WriteString(logger.IntToString(m.Year))
	}
	return bld.String()
}
func fillimdb() {
	file := "./init_imdb"
	if runtime.GOOS == "windows" {
		file = "init_imdb.exe"
	}
	cmd := exec.Command(file)
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf

	if scanner.CheckFileExist(file) && cmd.Run() == nil {
		logger.Log.GlobalLogger.Info(stdoutBuf.String())
		database.CloseImdb()
		os.Remove("./databases/imdb.db")
		os.Rename("./databases/imdbtemp.db", "./databases/imdb.db")
		database.InitImdbdb("info")
	}
	stdoutBuf.Reset()
	cmd = nil
}

func (s *feedResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Series.Close()
	if len(s.Movies) >= 1 {
		s.Movies = nil
	}
	s = nil
}

func feeds(cfgp *config.MediaTypeConfig, listname string) (*feedResults, error) {
	if !cfgp.ListsMap[listname].Enabled {
		logger.Log.GlobalLogger.Debug("Error - Group list not enabled")
		return nil, errNoListEnabled
	}
	listmao := cfgp.GetList(listname)
	listTemplateList := listmao.TemplateList
	listmao.Close()
	if !config.Check("list_" + listTemplateList) {
		logger.Log.GlobalLogger.Debug("Error - list not found")
		return nil, errNoList
	}

	if !config.Cfg.Lists[listTemplateList].Enabled {
		logger.Log.GlobalLogger.Debug("Error - list not enabled")
		return nil, errNoListEnabled
	}

	switch config.Cfg.Lists[listTemplateList].ListType {
	case "seriesconfig":
		content, err := os.ReadFile(config.Cfg.Lists[listTemplateList].SeriesConfigFile)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
		}
		feeddata := feedResults{}
		err = toml.Unmarshal(content, &feeddata.Series)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
			return nil, errNoList
		}
		return &feeddata, nil
	case "traktpublicshowlist":
		return getTraktUserPublicShowList(cfgp.GetList(listname))
	case "imdbcsv":
		return getMissingIMDBMoviesV2(cfgp.GetList(listname))
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(cfgp.GetList(listname))
	case "traktmoviepopular":
		return gettractmoviefeeds("popular", config.Cfg.Lists[listTemplateList].Limit, listTemplateList)
	case "traktmovieanticipated":
		return gettractmoviefeeds("anticipated", config.Cfg.Lists[listTemplateList].Limit, listTemplateList)
	case "traktmovietrending":
		return gettractmoviefeeds("trending", config.Cfg.Lists[listTemplateList].Limit, listTemplateList)
	case "traktseriepopular":
		return gettractseriefeeds("popular", config.Cfg.Lists[listTemplateList].Limit)
	case "traktserieanticipated":
		return gettractseriefeeds("anticipated", config.Cfg.Lists[listTemplateList].Limit)
	case "traktserietrending":
		return gettractseriefeeds("trending", config.Cfg.Lists[listTemplateList].Limit)
	case "newznabrss":

		searchresults, err := (&searcher.Searcher{
			Cfgp:    cfgp,
			Quality: cfgp.ListsMap[listname].TemplateQuality,
		}).GetRSSFeed("movie", cfgp, listname)
		if err != nil {
			return nil, err
		}
		var downloadnow *downloader.Downloadertype
		for idxres := range searchresults.Nzbs {
			logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.Stringp("url", &searchresults.Nzbs[idxres].NZB.Title))
			downloadnow = downloader.NewDownloader(cfgp)
			if searchresults.Nzbs[idxres].NzbmovieID != 0 {
				downloadnow.SetMovie(searchresults.Nzbs[idxres].NzbmovieID)
			} else if searchresults.Nzbs[idxres].NzbepisodeID != 0 {
				downloadnow.SetSeriesEpisode(searchresults.Nzbs[idxres].NzbepisodeID)
			}
			downloadnow.Nzb = searchresults.Nzbs[idxres]
			downloadnow.DownloadNzb()
			downloadnow.Close()
		}
		searchresults.Close()
		return nil, errNoList
	}

	logger.Log.GlobalLogger.Error("Feed Config not found", zap.String("template", listTemplateList), zap.String("type", listname))
	return nil, errNoConfig
}

func gettractmoviefeeds(traktlist string, limit int, templateList string) (*feedResults, error) {
	var traktpopular *apiexternal.TraktMovieGroup
	switch traktlist {
	case "popular":
		traktpopular, _ = apiexternal.TraktAPI.GetMoviePopular(limit)
	case "trending":
		traktpopular, _ = apiexternal.TraktAPI.GetMovieTrending(limit)
	case "anticipated":
		traktpopular, _ = apiexternal.TraktAPI.GetMovieAnticipated(limit)
	default:
		return nil, errwrongtype

	}
	if traktpopular == nil {
		return nil, errwrongtype
	}
	var results feedResults
	results.Movies = make([]string, 0, len(traktpopular.Movies))

	var countermovie int
	for idx := range traktpopular.Movies {
		if traktpopular.Movies[idx].Ids.Imdb != "" {
			database.QueryColumn(&database.Querywithargs{QueryString: countmovies, Args: []interface{}{traktpopular.Movies[idx].Ids.Imdb, templateList}}, &countermovie)
			if countermovie == 0 {
				results.Movies = append(results.Movies, traktpopular.Movies[idx].Ids.Imdb)
			}
		}
	}
	traktpopular.Close()
	return &results, nil
}

func gettractseriefeeds(traktlist string, limit int) (*feedResults, error) {
	var traktpopular *apiexternal.TraktSerieGroup
	switch traktlist {
	case "popular":
		traktpopular, _ = apiexternal.TraktAPI.GetSeriePopular(limit)
	case "trending":
		traktpopular, _ = apiexternal.TraktAPI.GetSerieTrending(limit)
	case "anticipated":
		traktpopular, _ = apiexternal.TraktAPI.GetSerieAnticipated(limit)
	default:
		return nil, errwrongtype

	}

	if traktpopular == nil {
		return nil, errwrongtype
	}
	var results feedResults
	for idx := range traktpopular.Series {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: traktpopular.Series[idx].Title, TvdbID: traktpopular.Series[idx].Ids.Tvdb,
		})
	}
	traktpopular.Close()
	return &results, nil
}

func getNewFilesMap(cfgp *config.MediaTypeConfig, checklist string) {
	table := "movie_file_unmatcheds"
	tablefiles := "movie_files"
	typestring := "movies"
	if strings.HasPrefix(cfgp.NamePrefix, "serie") {
		table = "serie_file_unmatcheds"
		tablefiles = "serie_episode_files"
		typestring = "series"
	}
	var allfiles logger.InStringArrayStruct
	var cnt, lennorename, lenfiles, lenblock int
	var ok bool
	var extlower, pathpercent string
	var pathcfg config.PathsConfig
	var walkfunc = func(path string, info fs.DirEntry, errwalk error) error {
		if errwalk != nil {
			return errwalk
		}
		if info.IsDir() {
			return nil
		}
		extlower = filepath.Ext(path)
		ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsIn)
		if lennorename >= 1 && !ok {
			ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsNoRenameIn)
		}

		if lennorename == 0 && lenfiles == 0 && !ok {
			ok = true
		}

		//Check IgnoredPaths
		if lenblock >= 1 && ok && logger.InStringArrayContainsCaseInSensitive(path, &pathcfg.BlockedLowerIn) {
			return nil
		}

		if ok {
			allfiles.Arr = append(allfiles.Arr, path)
		}
		return nil
	}
	var unmatcheddb, unwantedpaths *logger.InStringArrayStruct
	workergroup := logger.WorkerPools["Parse"].Group()
	queryunmatched := "select filepath from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)"
	queryunmatchedcount := "select count() from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)"
	querywronglistfiles := "select location from " + tablefiles + " where location like ?"
	querywronglistfilescount := "select count() from " + tablefiles + " where location like ?"
	for idx := range cfgp.Data {
		if !config.Check("path_" + cfgp.Data[idx].TemplatePath) {
			logger.Log.Warn("Config not found ", cfgp.Data[idx].TemplatePath)
			continue
		}
		pathcfg = config.Cfg.Paths[cfgp.Data[idx].TemplatePath]
		if !scanner.CheckFileExist(pathcfg.Path) {
			continue
		}
		cnt, ok = logger.GlobalCounter[pathcfg.Path]
		pathpercent = pathcfg.Path + "%"
		if ok {
			allfiles.Arr = make([]string, 0, cnt)
		} else {
			allfiles.Arr = []string{}
		}

		lennorename = len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr)
		lenfiles = len(pathcfg.AllowedVideoExtensionsIn.Arr)
		lenblock = len(pathcfg.BlockedLowerIn.Arr)
		//var pathdir string

		errwalk := filepath.WalkDir(pathcfg.Path, walkfunc)
		if len(allfiles.Arr) == 0 {
			logger.Log.GlobalLogger.Warn("No Files Found in", zap.String("Path", pathcfg.Path))
		}
		logger.GlobalMu.Lock()
		logger.GlobalCounter[pathcfg.Path] = len(allfiles.Arr)
		logger.GlobalMu.Unlock()
		if errwalk != nil {
			continue
		}
		if len(allfiles.Arr) == 0 {
			continue
		}
		cnt = database.CountRowsStaticNoError(&database.Querywithargs{QueryString: queryunmatchedcount, Args: []interface{}{pathpercent, time.Now().Add(time.Hour * -12)}})

		unmatcheddb = &logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false, cnt, &database.Querywithargs{QueryString: queryunmatched, Args: []interface{}{pathpercent, time.Now().Add(time.Hour * -12)}}, &unmatcheddb.Arr)

		unwantedpaths = &logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: querywronglistfilescount, Args: []interface{}{pathpercent}}),
			&database.Querywithargs{QueryString: querywronglistfiles, Args: []interface{}{pathpercent}}, &unwantedpaths.Arr)

		for idxall := range allfiles.Arr {
			path := allfiles.Arr[idxall]
			if logger.InStringArrayCaseSensitive(path, unmatcheddb) {
				continue
			}
			if logger.InStringArrayCaseSensitive(path, unwantedpaths) {
				continue
			}

			if typestring == "series" {
				addfoundlist := cfgp.Data[idx].AddFoundList
				workergroup.Submit(func() {
					jobImportSeriesParseV2(&importstruct{path: path, updatemissing: true, cfgp: cfgp, listname: addfoundlist})
				})
			} else {
				addfound := cfgp.Data[idx].AddFound
				addfoundlist := cfgp.Data[idx].AddFoundList
				workergroup.Submit(func() {
					jobImportMovieParseV2(&importstruct{path: path, updatemissing: true, cfgp: cfgp, listname: addfoundlist, addfound: addfound})
				})
			}
		}
	}
	pathcfg.Close()
	workergroup.Wait()
	unmatcheddb.Close()
	unwantedpaths.Close()
	allfiles.Close()
}

func SingleJobs(typ string, job string, cfgpstr string, listname string, force bool) {
	jobName := job
	cfgp := config.Cfg.Media[cfgpstr]
	if cfgp.Name != "" {
		jobName += "_" + cfgp.NamePrefix
	}
	if listname != "" {
		jobName += "_" + listname
	}

	if config.Cfg.General.SchedulerDisabled && !force {
		logger.Log.GlobalLogger.Info("Skipped Job", zap.String("Job", job), zap.String("config", cfgp.NamePrefix))
		cfgp.Close()
		return
	}

	logger.Log.GlobalLogger.Info(jobstarted, zap.Stringp("Job", &jobName))

	category := "Movie"
	if typ != "movie" {
		category = "Serie"
	}
	//dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	dbresult, _ := insertjobhistory(&database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: category, Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})

	switch job {
	case "datafull":
		getNewFilesMap(&cfgp, "")
	case "structure":
		structureSingle(typ, &cfgp)

	}

	if job == "data" {
		var lists []config.MediaListsConfig
		if listname != "" {
			lists = []config.MediaListsConfig{cfgp.ListsMap[listname]}
		} else {
			lists = cfgp.Lists
		}

		for idxlist := range lists {
			switch job {
			case "data":
				getNewFilesMap(&cfgp, lists[idxlist].Name)
			}
		}
	}
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	logger.Log.GlobalLogger.Info(jobended, zap.Stringp("Job", &job), zap.Stringp("config", &cfgp.NamePrefix))
	cfgp.Close()
}

func structureSingle(typ string, cfgp *config.MediaTypeConfig) {
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

		structure.OrganizeFolders(typ, mappathimport, cfgp.Data[0].TemplatePath, cfgp)
	}
}
