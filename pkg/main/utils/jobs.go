package utils

import (
	"bytes"
	"database/sql"
	"errors"
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
	return database.InsertStatic(&database.Querywithargs{QueryString: "Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, ?)", Args: []interface{}{job.JobType, job.JobGroup, job.JobCategory, job.Started}})
}
func endjobhistory(id int64) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update job_histories set ended = ? where id = ?", Args: []interface{}{time.Now().In(logger.TimeZone), id}})
}

func InitialFillSeries() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for series")

	var dbresult sql.Result
	var dbid int64
	for _, cfgp := range config.Cfg.Series {
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewseriessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
		cfgp.Close()
	}
	for _, cfgp := range config.Cfg.Series {
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
		cfgp.Close()
	}
}

func InitialFillMovies() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for movies")

	FillImdb()

	var dbresult sql.Result
	var dbid int64

	for _, cfgp := range config.Cfg.Movies {
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewmoviessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
		cfgp.Close()
	}

	for _, cfgp := range config.Cfg.Movies {
		dbresult, _ = insertjobhistory(&database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(dbid)
		cfgp.Close()
	}
}

func FillImdb() {
	group := logger.WorkerPools["Files"].Group()
	group.Submit(func() {
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
	})
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
	listTemplateList, listenabled := cfgp.GetTemplateList(listname)
	//listmao.Close()
	if !config.Check("list_" + listTemplateList) {
		logger.Log.GlobalLogger.Debug("Error - list not found")
		return nil, errNoList
	}

	if !listenabled {
		logger.Log.GlobalLogger.Debug("Error - list not enabled")
		return nil, errNoListEnabled
	}

	cfglist := config.Cfg.Lists[listTemplateList]
	defer cfglist.Close()
	switch cfglist.ListType {
	case "seriesconfig":
		content, err := os.ReadFile(cfglist.SeriesConfigFile)
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
		return getTraktUserPublicShowList(listTemplateList)
	case "imdbcsv":
		return getMissingIMDBMoviesV2(listTemplateList)
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(listTemplateList)
	case "traktmoviepopular":
		return gettractmoviefeeds("popular", cfglist.Limit, listTemplateList)
	case "traktmovieanticipated":
		return gettractmoviefeeds("anticipated", cfglist.Limit, listTemplateList)
	case "traktmovietrending":
		return gettractmoviefeeds("trending", cfglist.Limit, listTemplateList)
	case "traktseriepopular":
		return gettractseriefeeds("popular", cfglist.Limit)
	case "traktserieanticipated":
		return gettractseriefeeds("anticipated", cfglist.Limit)
	case "traktserietrending":
		return gettractseriefeeds("trending", cfglist.Limit)
	case "newznabrss":
		searchresults, err := (&searcher.Searcher{
			Cfgp:    cfgp,
			Quality: cfgp.ListsMap[listname].TemplateQuality,
		}).GetRSSFeed("movie", cfgp, listname)
		if err != nil {
			return nil, err
		}
		for idxres := range searchresults.Nzbs {
			logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.Stringp("url", &searchresults.Nzbs[idxres].NZB.Title))
			if searchresults.Nzbs[idxres].NzbmovieID != 0 {
				downloader.DownloadMovie(cfgp, searchresults.Nzbs[idxres].NzbmovieID, &searchresults.Nzbs[idxres])
			} else if searchresults.Nzbs[idxres].NzbepisodeID != 0 {
				downloader.DownloadSeriesEpisode(cfgp, searchresults.Nzbs[idxres].NzbepisodeID, &searchresults.Nzbs[idxres])
			}
		}
		searchresults.Close()
		return nil, errNoList
	}

	logger.Log.GlobalLogger.Error("Feed Config not found", zap.String("template", listTemplateList), zap.String("type", listname))
	return nil, errNoConfig
}

func getmovietraktdata(traktlist string, limit int) (*apiexternal.TraktMovieGroup, error) {
	switch traktlist {
	case "popular":
		return apiexternal.TraktAPI.GetMoviePopular(limit)
	case "trending":
		return apiexternal.TraktAPI.GetMovieTrending(limit)
	case "anticipated":
		return apiexternal.TraktAPI.GetMovieAnticipated(limit)
	default:
		return nil, errwrongtype
	}
}
func gettractmoviefeeds(traktlist string, limit int, templateList string) (*feedResults, error) {
	traktpopular, _ := getmovietraktdata(traktlist, limit)

	if traktpopular == nil {
		return nil, errwrongtype
	}
	results := feedResults{Movies: make([]string, 0, len(traktpopular.Movies))}

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

func getserietraktdata(traktlist string, limit int) (*apiexternal.TraktSerieGroup, error) {
	switch traktlist {
	case "popular":
		return apiexternal.TraktAPI.GetSeriePopular(limit)
	case "trending":
		return apiexternal.TraktAPI.GetSerieTrending(limit)
	case "anticipated":
		return apiexternal.TraktAPI.GetSerieAnticipated(limit)
	default:
		return nil, errwrongtype
	}
}
func gettractseriefeeds(traktlist string, limit int) (*feedResults, error) {
	traktpopular, _ := getserietraktdata(traktlist, limit)

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
	var cnt int
	var ok bool
	var pathpercent string

	var unmatcheddb, unwantedpaths *logger.InStringArrayStruct
	queryunmatched := "select filepath from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)"
	queryunmatchedcount := "select count() from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)"
	querywronglistfiles := "select location from " + tablefiles + " where location like ?"
	querywronglistfilescount := "select count() from " + tablefiles + " where location like ?"
	for idx := range cfgp.Data {
		if !config.Check("path_" + cfgp.Data[idx].TemplatePath) {
			logger.Log.Warn("Config not found ", cfgp.Data[idx].TemplatePath)
			continue
		}
		if !scanner.CheckFileExist(config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path) {
			continue
		}
		cnt, ok = logger.GlobalCounter[config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path]
		pathpercent = config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path + "%"
		if ok {
			allfiles.Arr = make([]string, 0, cnt)
		} else {
			allfiles.Arr = []string{}
		}

		//var pathdir string
		if filepath.WalkDir(config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path, scanner.Walk(cfgp.Data[idx].TemplatePath, &allfiles, false)) != nil {
			continue
		}
		if len(allfiles.Arr) == 0 {
			logger.Log.GlobalLogger.Warn("No Files Found in", zap.String("Path", config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path))
		}
		logger.GlobalMu.Lock()
		logger.GlobalCounter[config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path] = len(allfiles.Arr)
		logger.GlobalMu.Unlock()
		if len(allfiles.Arr) == 0 {
			continue
		}

		unmatcheddb = &logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: queryunmatchedcount, Args: []interface{}{pathpercent, time.Now().Add(time.Hour * -12)}}),
			&database.Querywithargs{QueryString: queryunmatched, Args: []interface{}{pathpercent, time.Now().Add(time.Hour * -12)}}, &unmatcheddb.Arr)

		unwantedpaths = &logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: querywronglistfilescount, Args: []interface{}{pathpercent}}),
			&database.Querywithargs{QueryString: querywronglistfiles, Args: []interface{}{pathpercent}}, &unwantedpaths.Arr)

		//reduce vars with function
		loopgetnewfiles(cfgp, idx, typestring, &allfiles, unmatcheddb, unwantedpaths)
		unmatcheddb.Close()
		unwantedpaths.Close()
	}
	allfiles.Close()
}

func loopgetnewfiles(cfgp *config.MediaTypeConfig, idx int, typestring string, allfiles *logger.InStringArrayStruct, unmatcheddb *logger.InStringArrayStruct, unwantedpaths *logger.InStringArrayStruct) {
	addfoundlist := cfgp.Data[idx].AddFoundList
	addfound := cfgp.Data[idx].AddFound
	workergroup := logger.WorkerPools["Parse"].Group()
	for idxall := range allfiles.Arr {
		path := allfiles.Arr[idxall]
		if logger.InStringArrayCaseSensitive(path, unmatcheddb) {
			continue
		}
		if logger.InStringArrayCaseSensitive(path, unwantedpaths) {
			continue
		}

		if typestring == "series" {
			workergroup.Submit(func() {
				jobImportSeriesParseV2(&importstruct{path: path, updatemissing: true, cfgp: cfgp, listname: addfoundlist})
			})
		} else {
			workergroup.Submit(func() {
				jobImportMovieParseV2(&importstruct{path: path, updatemissing: true, cfgp: cfgp, listname: addfoundlist, addfound: addfound})
			})
		}
	}
	workergroup.Wait()
}

func SingleJobs(typ string, job string, cfgpstr string, listname string, force bool) {
	cfgp := config.Cfg.Media[cfgpstr]
	defer cfgp.Close()
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

	logger.Log.GlobalLogger.Info(jobstarted, zap.Stringp("Job", &jobName))

	category := "Movie"
	if typ != "movie" {
		category = "Serie"
	}
	dbresult, _ := insertjobhistory(&database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: category, Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})

	switch job {
	case "datafull":
		getNewFilesMap(&cfgp, "")
	case "structure":
		structureSingle(typ, &cfgp)

	}

	if job == "data" {
		for _, list := range getjoblists(&cfgp, listname) {
			switch job {
			case "data":
				getNewFilesMap(&cfgp, list.Name)
			}
		}
	}
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	logger.Log.GlobalLogger.Info(jobended, zap.Stringp("Job", &job), zap.Stringp("config", &cfgp.NamePrefix))
}

func structureSingle(typ string, cfgp *config.MediaTypeConfig) {
	if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
		return
	}

	for idxdata := range cfgp.DataImport {
		if !config.Check("path_" + cfgp.DataImport[idxdata].TemplatePath) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.DataImport[idxdata].TemplatePath))

			continue
		}

		if lastSeriesStructure == config.Cfg.Paths[cfgp.DataImport[idxdata].TemplatePath].Path {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastSeriesStructure = config.Cfg.Paths[cfgp.DataImport[idxdata].TemplatePath].Path

		structure.OrganizeFolders(typ, cfgp.DataImport[idxdata].TemplatePath, cfgp.Data[0].TemplatePath, cfgp)
	}
}
