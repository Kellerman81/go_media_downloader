package utils

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"os/exec"
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
	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
)

func jobImportFileCheck(file string, dbtype string) {
	if !scanner.CheckFileExist(file) {
		query := "select id as num1, serie_episode_id as num2 from serie_episode_files where location = ?"
		subquerycount := "select count() from serie_episode_files where serie_episode_id = ?"
		table := "serie_episode_files"
		updatetable := "serie_episodes"
		if dbtype == "movie" {
			query = "select id as num1, movie_id as num2 from movie_files where location = ?"
			subquerycount = "select count() from movie_files where movie_id = ?"
			table = "movie_files"
			updatetable = "movies"
		}
		files, _ := database.QueryStaticColumnsTwoInt(database.Querywithargs{QueryString: query, Args: []interface{}{file}})
		var err error
		var counter int
		for idx := range files {
			logger.Log.GlobalLogger.Debug("File was removed", zap.String("file", file))
			err = database.DeleteRowStatic(database.Querywithargs{QueryString: "Delete from " + table + " where id = ?", Args: []interface{}{files[idx].Num1}})
			if err == nil {
				counter, err = database.CountRowsStatic(database.Querywithargs{QueryString: subquerycount, Args: []interface{}{files[idx].Num2}})
				if counter == 0 && err == nil {
					database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update " + updatetable + " set missing = ? where id = ?", Args: []interface{}{1, files[idx].Num2}})
				}
			}
		}
		files = nil
	}
}

var errNoList error = errors.New("list not found")
var errNoListEnabled error = errors.New("list not enabled")
var errNoListRead error = errors.New("list not readable")
var errNoListOther error = errors.New("list other error")
var errNoFiles error = errors.New("files not found")
var errNoConfig error = errors.New("config not found")
var errNoGeneral error = errors.New("general not found")

const wrongtype string = "wrong type"

func InitRegex() {
	logger.GlobalRegexCache.SetRegexp("RegexSeriesIdentifier", `(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`, 0)
	logger.GlobalRegexCache.SetRegexp("RegexSeriesTitle", `^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`, 0)
}

func insertjobhistory(job database.JobHistory) (sql.Result, error) {
	return database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", job)
}
func endjobhistory(id uint) {
	database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update job_histories set ended = ? where id = ?", Args: []interface{}{time.Now().In(logger.TimeZone), id}})
}

func InitialFillSeries() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for series")

	var dbresult sql.Result
	var dbid int64
	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Series {
		cfgp = config.Cfg.Series[idx]
		dbresult, _ = insertjobhistory(database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewseriessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(uint(dbid))

	}
	for idx := range config.Cfg.Series {
		cfgp = config.Cfg.Series[idx]
		dbresult, _ = insertjobhistory(database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "series", "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(uint(dbid))
	}
}

func getFilesDirParse4(cfgp *config.MediaTypeConfig, rootpath string, pathcfgstr string, checklist string, typestring string, addfound bool, addfoundlist string) error {

	if pathcfgstr == "" {
		return errNoGeneral
	}

	if scanner.CheckFileExist(rootpath) {
		allfiles, err := scanner.GetFilesDirAll(rootpath, true)
		if err != nil {
			return errNoFiles
		}
		defer allfiles.Close()
		if len(allfiles.Arr) == 0 {
			return errNoFiles
		}
		videofiles, err := scanner.FilterFilesDir(allfiles, config.Cfg.Paths[pathcfgstr].Name, false, false)
		if err != nil {
			return errNoFiles
		}
		defer videofiles.Close()
		if len(videofiles.Arr) == 0 {
			return errNoFiles
		}
		table := "movie_file_unmatcheds"
		tablefiles := "movie_files"
		if typestring == "series" {
			table = "serie_file_unmatcheds"
			tablefiles = "serie_episode_files"
		}
		queryunmatched := "select filepath from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)"
		counter := database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from " + table + " where filepath like ? and (last_checked > ? or last_checked is null)", Args: []interface{}{rootpath + "%", time.Now().Add(time.Hour * -12)}})
		unmatcheddb := &logger.InStringArrayStruct{Arr: database.QueryStaticStringArray(false, counter, database.Querywithargs{QueryString: queryunmatched, Args: []interface{}{rootpath + "%", time.Now().Add(time.Hour * -12)}})}
		defer unmatcheddb.Close()

		querywronglistfiles := "select location from " + tablefiles + " where location like ?"

		unwantedpaths := &logger.InStringArrayStruct{Arr: database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from " + tablefiles + " where location like ?", Args: []interface{}{rootpath + "%"}}),
			database.Querywithargs{QueryString: querywronglistfiles, Args: []interface{}{rootpath + "%"}})}
		defer unwantedpaths.Close()

		workergroup := logger.WorkerPools["Parse"].Group()
		for idx := range videofiles.Arr {
			if logger.InStringArrayCaseSensitive(videofiles.Arr[idx], unwantedpaths) {
				continue
			}
			if logger.InStringArrayCaseSensitive(videofiles.Arr[idx], unmatcheddb) {
				continue
			}
			path := videofiles.Arr[idx]

			if typestring == "series" {
				workergroup.Submit(func() {
					jobImportSeriesParseV2(path, true, cfgp, addfoundlist)
				})
			} else {
				workergroup.Submit(func() {
					jobImportMovieParseV2(path, cfgp, addfoundlist, true, addfound)
				})
			}
		}
		workergroup.Wait()
	}
	return nil
}

func InitialFillMovies() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for movies")

	FillImdb()

	var dbresult sql.Result
	var dbid int64

	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Movies {
		cfgp = config.Cfg.Movies[idx]
		dbresult, _ = insertjobhistory(database.JobHistory{JobType: "feeds", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range cfgp.Lists {
			importnewmoviessingle(&cfgp, cfgp.Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(uint(dbid))

	}

	for idx := range config.Cfg.Movies {
		cfgp = config.Cfg.Movies[idx]
		dbresult, _ = insertjobhistory(database.JobHistory{JobType: "datafull", JobGroup: cfgp.Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		//dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(&cfgp, "movie", "")
		dbid, _ = dbresult.LastInsertId()
		endjobhistory(uint(dbid))
	}
}

func FillImdb() {
	group := logger.WorkerPools["Files"].Group()
	group.Submit(func() { fillimdb() })
	group.Wait()
}
func fillimdb() {
	file := "./init_imdb"
	if runtime.GOOS == "windows" {
		file = "init_imdb.exe"
	}
	cmd := exec.Command(file)
	var stdoutBuf bytes.Buffer
	defer stdoutBuf.Reset()
	cmd.Stdout = &stdoutBuf

	errexec := cmd.Run()
	if scanner.CheckFileExist(file) && errexec == nil {
		logger.Log.GlobalLogger.Info(stdoutBuf.String())
		database.CloseImdb()
		os.Remove("./databases/imdb.db")
		os.Rename("./databases/imdbtemp.db", "./databases/imdb.db")
		database.InitImdbdb("info")
	}
	logger.ClearVar(&cmd)
}

type feedResults struct {
	Series config.MainSerieConfig
	Movies []string
}

func (s *feedResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		s.Series.Close()
		if len(s.Movies) >= 1 {
			s.Movies = nil
		}
		logger.ClearVar(&s)
	}
}

func feeds(cfgp *config.MediaTypeConfig, listname string) (*feedResults, error) {
	listTemplateList := cfgp.ListsMap[listname].TemplateList
	if !cfgp.ListsMap[listname].Enabled {
		logger.Log.GlobalLogger.Debug("Error - Group list not enabled")
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + listTemplateList) {
		logger.Log.GlobalLogger.Debug("Error - list not found")
		return nil, errNoList
	}

	if !config.Cfg.Lists[listTemplateList].Enabled {
		logger.Log.GlobalLogger.Debug("Error - list not enabled")
		return nil, errNoListEnabled
	}

	cfgplist := cfgp.ListsMap[listname]
	switch config.Cfg.Lists[listTemplateList].ListType {
	case "seriesconfig":
		content, err := os.ReadFile(config.Cfg.Lists[listTemplateList].SeriesConfigFile)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
		}
		defer logger.ClearVar(&content)
		var results config.MainSerieConfig
		err = toml.Unmarshal(content, &results)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
			return nil, errNoList
		}
		defer results.Close()
		return &feedResults{Series: results}, nil
	case "traktpublicshowlist":
		series, err := getTraktUserPublicShowList(&cfgplist)
		if err == nil {
			defer series.Close()
			return &feedResults{Series: *series}, err
		}
		return nil, err
	case "imdbcsv":
		return getMissingIMDBMoviesV2(&cfgplist)
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(&cfgplist)
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
		srcher := searcher.NewSearcher(cfgp, cfgp.ListsMap[listname].TemplateQuality)
		defer srcher.Close()
		searchresults, err := srcher.GetRSSFeed("movie", cfgp, listname)
		if err != nil {
			return nil, err
		}
		defer searchresults.Close()
		var downloadnow *downloader.Downloadertype
		for idxres := range searchresults.Nzbs {
			logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("url", searchresults.Nzbs[idxres].NZB.Title))
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
		downloadnow.Close()
		return nil, errNoList
	}

	logger.Log.GlobalLogger.Error("Feed Config not found", zap.String("template", listTemplateList), zap.String("type", listname))
	return nil, errNoConfig
}

func gettractmoviefeeds(traktlist string, limit int, templateList string) (*feedResults, error) {
	var traktpopular *apiexternal.TraktMovieGroup
	switch traktlist {
	case "popular":
		traktpopular, _ = apiexternal.TraktApi.GetMoviePopular(limit)
	case "trending":
		traktpopular, _ = apiexternal.TraktApi.GetMovieTrending(limit)
	case "anticipated":
		traktpopular, _ = apiexternal.TraktApi.GetMovieAnticipated(limit)
	default:
		return nil, errors.New(wrongtype)

	}
	if traktpopular == nil {
		return nil, errors.New(wrongtype)
	}
	results := new(feedResults)
	results.Movies = logger.GrowSliceBy(results.Movies, len(traktpopular.Movies))

	countseries := "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"
	var countermovie int
	for idx := range traktpopular.Movies {
		if traktpopular.Movies[idx].Ids.Imdb != "" {
			countermovie, _ = database.CountRowsStatic(database.Querywithargs{QueryString: countseries, Args: []interface{}{traktpopular.Movies[idx].Ids.Imdb, templateList}})
			if countermovie == 0 {
				results.Movies = append(results.Movies, traktpopular.Movies[idx].Ids.Imdb)
			}
		}
	}
	traktpopular = nil
	return results, nil
}

func gettractseriefeeds(traktlist string, limit int) (*feedResults, error) {
	var traktpopular *apiexternal.TraktSerieGroup
	switch traktlist {
	case "popular":
		traktpopular, _ = apiexternal.TraktApi.GetSeriePopular(limit)
	case "trending":
		traktpopular, _ = apiexternal.TraktApi.GetSerieTrending(limit)
	case "anticipated":
		traktpopular, _ = apiexternal.TraktApi.GetSerieAnticipated(limit)
	default:
		return nil, errors.New(wrongtype)

	}

	if traktpopular == nil {
		return nil, errors.New(wrongtype)
	}
	results := new(feedResults)
	results.Series = config.MainSerieConfig{Serie: logger.CopyFunc(traktpopular.Series, func(elem apiexternal.TraktSerie) config.SerieConfig {
		return config.SerieConfig{
			Name: elem.Title, TvdbID: elem.Ids.Tvdb,
		}
	})}
	traktpopular = nil
	return results, nil
}

func findFilesMap(cfgp *config.MediaTypeConfig, checklist string) error {
	typestring := "movies"
	if strings.HasPrefix(cfgp.NamePrefix, "serie") {
		typestring = "series"
	}
	for idx := range cfgp.Data {
		if config.ConfigCheck("path_" + cfgp.Data[idx].TemplatePath) {
			getFilesDirParse4(cfgp, config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path, cfgp.Data[idx].TemplatePath, checklist, typestring, cfgp.Data[idx].AddFound, cfgp.Data[idx].AddFoundList)
		}
	}
	return nil
}

func getNewFilesMap(cfgp *config.MediaTypeConfig, mediatype string, checklist string) {
	findFilesMap(cfgp, checklist)
}
