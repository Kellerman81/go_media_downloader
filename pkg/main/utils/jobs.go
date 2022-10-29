package utils

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"regexp"
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
		files, _ := database.QueryStaticColumnsTwoInt(query, file)
		for idx := range files {
			logger.Log.GlobalLogger.Debug("File was removed", zap.String("file", file))
			err := database.DeleteRowStatic("Delete from "+table+" where id = ?", files[idx].Num1)
			if err == nil {
				counter, err := database.CountRowsStatic(subquerycount, files[idx].Num2)
				if counter == 0 && err == nil {
					database.UpdateColumnStatic("Update "+updatetable+" set missing = ? where id = ?", 1, files[idx].Num2)
				}
			}
		}
	}
}

var errNoList error = errors.New("list not found")
var errNoListEnabled error = errors.New("list not enabled")
var errNoListRead error = errors.New("list not readable")
var errNoListOther error = errors.New("list other error")
var errNoFiles error = errors.New("files not found")
var errNoConfig error = errors.New("config not found")
var errNoGeneral error = errors.New("general not found")

func InitRegex() {
	logger.GlobalRegexCache.SetRegexp("RegexSeriesIdentifier", regexp.MustCompile(`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`), 0)
	logger.GlobalRegexCache.SetRegexp("RegexSeriesTitle", regexp.MustCompile(`^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`), 0)
}

func InitialFillSeries() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for series")

	var dbresult sql.Result
	var dbid int64
	for idx := range config.Cfg.Series {
		dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range config.Cfg.Series[idx].Lists {
			importnewseriessingle(config.Cfg.Series[idx].NamePrefix, config.Cfg.Series[idx].Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)

	}
	for idx := range config.Cfg.Series {
		dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Series[idx].Name, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(config.Cfg.Series[idx].NamePrefix, "series", "")
		dbid, _ = dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)
	}
}

func getFilesDirParse4(cfg string, rootpath string, pathcfgstr string, checklist string, typestring string, addfound bool, addfoundlist string) error {

	if pathcfgstr == "" {
		return errNoGeneral
	}

	if scanner.CheckFileExist(rootpath) {
		allfiles, err := scanner.GetFilesDirAll(rootpath)
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
		queryunmatched := "select filepath from movie_file_unmatcheds where filepath like ? and (last_checked > ? or last_checked is null)"
		if typestring == "series" {
			queryunmatched = "select filepath from serie_file_unmatcheds where filepath like ? and (last_checked > ? or last_checked is null)"
		}
		counter := database.CountRowsStaticNoError(strings.Replace(queryunmatched, " filepath from", " count() from", 1), rootpath+"%", time.Now().Add(time.Hour*-12))
		unmatcheddb := &logger.InStringArrayStruct{Arr: database.QueryStaticStringArray(queryunmatched, false, counter, rootpath+"%", time.Now().Add(time.Hour*-12))}
		defer unmatcheddb.Close()

		querywronglistfiles := "select location from movie_files where location like ?"
		if typestring == "series" {
			querywronglistfiles = "select location from serie_episode_files where location like ?"
		}

		unwantedpaths := &logger.InStringArrayStruct{Arr: database.QueryStaticStringArray(querywronglistfiles, false,
			database.CountRowsStaticNoError(strings.Replace(querywronglistfiles, "location from", "count() from", 1), rootpath+"%"),
			rootpath+"%")}
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
					jobImportSeriesParseV2(path, true, cfg, addfoundlist)
				})
			} else {
				workergroup.Submit(func() {
					jobImportMovieParseV2(path, cfg, addfoundlist, true, addfound)
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

	for idx := range config.Cfg.Movies {
		dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "feeds", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		for idxlist := range config.Cfg.Movies[idx].Lists {
			importnewmoviessingle(config.Cfg.Movies[idx].NamePrefix, config.Cfg.Movies[idx].Lists[idxlist].Name)
		}
		dbid, _ = dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)

	}

	for idx := range config.Cfg.Movies {
		dbresult, _ = database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: "datafull", JobGroup: config.Cfg.Movies[idx].Name, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
		getNewFilesMap(config.Cfg.Movies[idx].NamePrefix, "movie", "")
		dbid, _ = dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)
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

func feeds(cfg string, listname string) (*feedResults, error) {
	list_Template_list := config.Cfg.Media[cfg].ListsMap[listname].Template_list
	if !config.Cfg.Media[cfg].ListsMap[listname].Enabled {
		logger.Log.GlobalLogger.Debug("Error - Group list not enabled")
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + list_Template_list) {
		logger.Log.GlobalLogger.Debug("Error - list not found")
		return nil, errNoList
	}

	if !config.Cfg.Lists[list_Template_list].Enabled {
		logger.Log.GlobalLogger.Debug("Error - list not enabled")
		return nil, errNoListEnabled
	}

	switch config.Cfg.Lists[list_Template_list].ListType {
	case "seriesconfig":
		content, err := os.ReadFile(config.Cfg.Lists[list_Template_list].Series_config_file)
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
		series, err := getTraktUserPublicShowList(cfg, listname)
		if err == nil {
			defer series.Close()
			return &feedResults{Series: *series}, err
		}
		return nil, err
	case "imdbcsv":
		return getMissingIMDBMoviesV2(cfg, listname)
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(cfg, listname)
	case "traktmoviepopular":
		return gettractmoviefeeds("popular", config.Cfg.Lists[list_Template_list].Limit, list_Template_list)
	case "traktmovieanticipated":
		return gettractmoviefeeds("anticipated", config.Cfg.Lists[list_Template_list].Limit, list_Template_list)
	case "traktmovietrending":
		return gettractmoviefeeds("trending", config.Cfg.Lists[list_Template_list].Limit, list_Template_list)
	case "traktseriepopular":
		return gettractseriefeeds("popular", config.Cfg.Lists[list_Template_list].Limit)
	case "traktserieanticipated":
		return gettractseriefeeds("anticipated", config.Cfg.Lists[list_Template_list].Limit)
	case "traktserietrending":
		return gettractseriefeeds("trending", config.Cfg.Lists[list_Template_list].Limit)
	case "newznabrss":
		srcher := searcher.NewSearcher(cfg, config.Cfg.Media[cfg].ListsMap[listname].Template_quality)
		defer srcher.Close()
		searchresults, err := srcher.GetRSSFeed("movie", cfg, listname)
		if err != nil {
			return nil, err
		}
		defer searchresults.Close()
		for idxres := range searchresults.Nzbs {
			logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("url", searchresults.Nzbs[idxres].NZB.Title))
			downloadnow := downloader.NewDownloader(cfg)
			if searchresults.Nzbs[idxres].NzbmovieID != 0 {
				downloadnow.SetMovie(searchresults.Nzbs[idxres].NzbmovieID)
			} else if searchresults.Nzbs[idxres].NzbepisodeID != 0 {
				downloadnow.SetSeriesEpisode(searchresults.Nzbs[idxres].NzbepisodeID)
			}
			downloadnow.Nzb = searchresults.Nzbs[idxres]
			downloadnow.DownloadNzb()
			downloadnow.Close()
		}
		return nil, errNoList
	}

	logger.Log.GlobalLogger.Error("Feed Config not found", zap.String("template", list_Template_list), zap.String("type", listname))
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
		return nil, errors.New("wrong type")

	}
	if traktpopular == nil {
		return nil, errors.New("wrong type")
	}
	var results feedResults
	results.Movies = make([]string, 0, len(traktpopular.Movies))

	for idx := range traktpopular.Movies {
		if len(traktpopular.Movies[idx].Ids.Imdb) >= 1 {
			countermovie, _ := database.CountRowsStatic("select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", traktpopular.Movies[idx].Ids.Imdb, templateList)
			if countermovie == 0 {
				results.Movies = append(results.Movies, traktpopular.Movies[idx].Ids.Imdb)
			}
		}
	}
	traktpopular = nil
	return &results, nil
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
		return nil, errors.New("wrong type")

	}

	if traktpopular == nil {
		return nil, errors.New("wrong type")
	}
	var results feedResults
	results.Series = config.MainSerieConfig{Serie: logger.CopyFunc(traktpopular.Series, func(elem apiexternal.TraktSerie) config.SerieConfig {
		return config.SerieConfig{
			Name: elem.Title, TvdbID: elem.Ids.Tvdb,
		}
	})}
	traktpopular = nil
	return &results, nil
}

func findFilesMap(cfg string, checklist string) error {
	typestring := "movies"
	if strings.HasPrefix(cfg, "serie") {
		typestring = "series"
	}
	for idx := range config.Cfg.Media[cfg].Data {
		if config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[idx].Template_path) {
			getFilesDirParse4(cfg, config.Cfg.Paths[config.Cfg.Media[cfg].Data[idx].Template_path].Path, config.Cfg.Media[cfg].Data[idx].Template_path, checklist, typestring, config.Cfg.Media[cfg].Data[idx].AddFound, config.Cfg.Media[cfg].Data[idx].AddFoundList)
		}
	}
	return nil
}

func getNewFilesMap(cfg string, mediatype string, checklist string) {
	findFilesMap(cfg, checklist)
}
