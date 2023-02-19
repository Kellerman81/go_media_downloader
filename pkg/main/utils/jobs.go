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
	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/alitto/pond"
	"github.com/pelletier/go-toml/v2"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
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
	if scanner.CheckFileExist(file) {
		return
	}
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
	var counter int
	for idx := range files {
		logger.Log.GlobalLogger.Debug("File was removed", zap.Stringp("file", &file))
		if database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from " + table + " where id = ?", Args: []interface{}{files[idx].Num1}}) == nil {
			database.QueryColumn(&database.Querywithargs{QueryString: subquerycount, Args: []interface{}{files[idx].Num2}}, &counter)
			if counter == 0 {
				database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update " + updatetable + " set missing = ? where id = ?", Args: []interface{}{1, files[idx].Num2}})
			}
		}
	}
	files = nil
}

func insertjobhistory(jobtype string, jobgroup string, jobcategory string) int64 {
	result, err := database.InsertStatic(&database.Querywithargs{QueryString: "Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, ?)", Args: []interface{}{jobtype, jobgroup, jobcategory, logger.SqlTimeGetNow()}})
	if err == nil && result != nil {
		dbid, err := result.LastInsertId()
		if err == nil {
			return dbid
		} else {
			logger.Log.GlobalLogger.Debug("error task", zap.Int64("id", dbid), zap.Error(err))
		}
	}
	return 0
}
func endjobhistory(id int64) {
	//logger.Log.GlobalLogger.Debug("Ended task", zap.Int64("id", id))
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update job_histories set ended = ? where id = ?", Args: []interface{}{logger.SqlTimeGetNow(), id}})
}

func InitialFillSeries() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for series")

	var dbid int64
	for _, cfgp := range config.Cfg.Series {
		dbid = insertjobhistory("feeds", cfgp.Name, "Serie")
		for idx := range cfgp.Lists {
			importnewseriessingle(&cfgp, cfgp.Lists[idx].Name)
		}
		endjobhistory(dbid)
		cfgp.Close()
	}
	for _, cfgp := range config.Cfg.Series {
		dbid = insertjobhistory("datafull", cfgp.Name, "Serie")
		getNewFilesMap(&cfgp, "")
		endjobhistory(dbid)
		cfgp.Close()
	}
}

func InitialFillMovies() {
	logger.Log.GlobalLogger.Info("Starting initial DB fill for movies")
	var dbid int64

	for _, cfgp := range config.Cfg.Movies {
		dbid = insertjobhistory("feeds", cfgp.Name, "Movie")
		for idx := range cfgp.Lists {
			importnewmoviessingle(&cfgp, cfgp.Lists[idx].Name)
		}
		endjobhistory(dbid)
		cfgp.Close()
	}

	for _, cfgp := range config.Cfg.Movies {
		dbid = insertjobhistory("datafull", cfgp.Name, "Movie")
		getNewFilesMap(&cfgp, "")
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

func feeds(cfgp *config.MediaTypeConfig, listname string, cfglist *config.ListsConfig) (*feedResults, error) {
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

	if !listenabled || !cfglist.Enabled {
		logger.Log.GlobalLogger.Debug("Error - list not enabled")
		return nil, errNoListEnabled
	}

	switch cfglist.ListType {
	case "seriesconfig":
		content, err := os.ReadFile(cfglist.SeriesConfigFile)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
		}
		feeddata := new(feedResults)
		err = toml.Unmarshal(content, &feeddata.Series)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error loading config. ", zap.Error(err))
			return nil, errNoList
		}
		return feeddata, nil
	case "traktpublicshowlist":
		return getTraktUserPublicShowList(listTemplateList, cfglist)
	case "imdbcsv":
		return getMissingIMDBMoviesV2(listTemplateList, cfglist)
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(listTemplateList, cfglist)
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
	results := &feedResults{Movies: make([]string, 0, len(traktpopular.Movies))}

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
	return results, nil
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
	results := new(feedResults)
	for idx := range traktpopular.Series {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: traktpopular.Series[idx].Title, TvdbID: traktpopular.Series[idx].Ids.Tvdb,
		})
	}
	traktpopular.Close()
	return results, nil
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

	var unmatchedcache *cache.Return
	if !logger.GlobalCache.CheckNoType(table + "_cached") {
		unmatchedpaths := logger.InStringArrayStruct{}
		queryunmatched := "select filepath from " + table + " where (last_checked > ? or last_checked is null)"
		queryunmatchedcount := "select count() from " + table + " where (last_checked > ? or last_checked is null)"
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: queryunmatchedcount, Args: []interface{}{sql.NullTime{Time: logger.TimeGetNow().Add(time.Hour * -12), Valid: true}}}),
			&database.Querywithargs{QueryString: queryunmatched, Args: []interface{}{sql.NullTime{Time: logger.TimeGetNow().Add(time.Hour * -12), Valid: true}}},
			&unmatchedpaths.Arr)
		logger.GlobalCache.Set(table+"_cached", unmatchedpaths, 3*time.Hour, false)
		unmatchedcache = new(cache.Return)
		unmatchedcache.Value = unmatchedpaths
		unmatchedpaths.Close()
	} else {
		unmatchedcache = logger.GlobalCache.GetData(table + "_cached")
	}

	var unwantedcache *cache.Return
	if !logger.GlobalCache.CheckNoType(tablefiles + "_cached") {
		unwantedpaths := logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from " + tablefiles}),
			&database.Querywithargs{QueryString: "select location from " + tablefiles},
			&unwantedpaths.Arr)
		logger.GlobalCache.Set(tablefiles+"_cached", unwantedpaths, 3*time.Hour, false)
		unwantedcache = new(cache.Return)
		unwantedcache.Value = unwantedpaths
		unwantedpaths.Close()
	} else {
		unwantedcache = logger.GlobalCache.GetData(tablefiles + "_cached")
	}

	workergroup := logger.WorkerPools["Parse"].Group()
	for idx := range cfgp.Data {
		if !config.Check("path_" + cfgp.Data[idx].TemplatePath) {
			logger.Log.Warn("Config not found ", cfgp.Data[idx].TemplatePath)
			continue
		}
		if !scanner.CheckFileExist(config.Cfg.Paths[cfgp.Data[idx].TemplatePath].Path) {
			continue
		}
		cfgpath := config.Cfg.Paths[cfgp.Data[idx].TemplatePath]
		//var pathdir string
		filepath.WalkDir(cfgpath.Path, WalkDataDir(workergroup, cfgp.Data[idx].TemplatePath, cfgp, &cfgpath, cfgp.Data[idx].AddFoundList, cfgp.Data[idx].AddFound, typestring, unmatchedcache, unwantedcache))
		cfgpath.Close()
	}
	workergroup.Wait()
	unmatchedcache = nil
	unwantedcache = nil
}

func WalkDataDir(workergroup *pond.TaskGroup, pathcfgstr string, cfgp *config.MediaTypeConfig, cfgpath *config.PathsConfig, addfoundlist string, addfound bool, typestring string, unmatcheddb *cache.Return, unwantedpaths *cache.Return) func(path string, info fs.DirEntry, errwalk error) error {
	return func(path string, info fs.DirEntry, errwalk error) error {
		if errwalk != nil {
			return errwalk
		}
		if info.IsDir() {
			return nil
		}
		extlower := filepath.Ext(path)
		var ok bool

		if slices.ContainsFunc(cfgpath.AllowedVideoExtensionsIn.Arr, func(c string) bool { return strings.EqualFold(c, extlower) }) {
			ok = true
		}

		if len(cfgpath.AllowedVideoExtensionsNoRenameIn.Arr) >= 1 && !ok {
			if slices.ContainsFunc(cfgpath.AllowedVideoExtensionsNoRenameIn.Arr, func(c string) bool { return strings.EqualFold(c, extlower) }) {
				ok = true
			}
		}

		if len(cfgpath.AllowedVideoExtensionsNoRenameIn.Arr) == 0 && len(cfgpath.AllowedVideoExtensionsIn.Arr) == 0 && !ok {
			ok = true
		}

		//Check IgnoredPaths

		if len(cfgpath.BlockedLowerIn.Arr) >= 1 && ok {
			if logger.InStringArrayContainsCaseInSensitive(path, &cfgpath.BlockedLowerIn) {
				ok = false
			}
		}
		//if lenblock >= 1 && ok && logger.InStringArrayContainsCaseInSensitive(path, &cfgpath.BlockedLowerIn) {
		//	return nil
		//}
		if !ok {
			return nil
		}
		if slices.Contains(unmatcheddb.Value.(logger.InStringArrayStruct).Arr, path) {
			return nil
		}
		if slices.Contains(unwantedpaths.Value.(logger.InStringArrayStruct).Arr, path) {
			return nil
		}
		if typestring == "series" {
			workergroup.Submit(func() {
				jobImportSeriesParseV2(path, true, cfgp, addfoundlist, false)
			})
		} else {
			workergroup.Submit(func() {
				jobImportMovieParseV2(path, true, cfgp, addfoundlist, addfound)
			})
		}
		return nil
	}
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
	dbinsert := insertjobhistory(job, cfgp.NamePrefix, category)

	searchmissingIncremental := cfgp.SearchmissingIncremental
	searchupgradeIncremental := cfgp.SearchupgradeIncremental
	if searchmissingIncremental == 0 {
		searchmissingIncremental = 20
	}
	if searchupgradeIncremental == 0 {
		searchupgradeIncremental = 20
	}
	var search, searchtitle, searchmissing bool
	var searchinterval int
	switch job {
	case "datafull":
		getNewFilesMap(&cfgp, "")
	case "searchmissingfull":
		search = true
		searchmissing = true
	case "searchmissinginc":
		search = true
		searchmissing = true
		searchinterval = searchmissingIncremental
	case "searchupgradefull":
		search = true
	case "searchupgradeinc":
		search = true
		searchinterval = searchupgradeIncremental
	case "searchmissingfulltitle":
		search = true
		searchmissing = true
		searchtitle = true
	case "searchmissinginctitle":
		search = true
		searchmissing = true
		searchtitle = true
		searchinterval = searchmissingIncremental
	case "searchupgradefulltitle":
		search = true
		searchtitle = true
	case "searchupgradeinctitle":
		search = true
		searchtitle = true
		searchinterval = searchupgradeIncremental
	case "structure":
		structurefolders(&cfgp, typ)

	}

	if search {
		if typ == "movie" {
			searcher.Searchlist(&cfgp, "movies", searchtitle, searcher.SearchMovie(&cfgp, searchmissing, searchinterval, searchtitle))
		} else {
			searcher.Searchlist(&cfgp, "serie_episodes", searchtitle, searcher.SearchSerie(&cfgp, searchmissing, searchinterval, searchtitle))
		}
	}

	if job == "data" {
		for _, list := range getjoblists(&cfgp, listname) {
			getNewFilesMap(&cfgp, list.Name)
		}
	}

	if job == "rss" || job == "checkreachedflag" || job == "clearhistory" || job == "feeds" || job == "checkmissing" || job == "checkmissingflag" {
		qualis := new(logger.InStringArrayStruct)

		for _, list := range getjoblists(&cfgp, listname) {
			if job == "rss" && !slices.Contains(qualis.Arr, list.TemplateQuality) {
				qualis.Arr = append(qualis.Arr, list.TemplateQuality)
			}
			switch job {
			case "checkmissing":
				checkmissing(typ, list.Name)
			case "checkmissingflag":
				checkmissingflag(typ, list.Name)
			case "checkreachedflag":
				if typ == "movie" {
					checkreachedmoviesflag(&cfgp, list.Name)
				} else {
					checkreachedepisodesflag(&cfgp, list.Name)
				}
			case "clearhistory":
				if typ == "movie" {
					database.DeleteRow("movie_histories", &database.Querywithargs{Query: database.Query{Where: "movie_id in (Select id from movies where listname = ? COLLATE NOCASE)"}, Args: []interface{}{list.Name}})
				} else {
					database.DeleteRow("serie_episode_histories", &database.Querywithargs{Query: database.Query{Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{list.Name}})
				}
			case "feeds":
				if typ == "movie" {
					importnewmoviessingle(&cfgp, list.Name)
				} else {
					importnewseriessingle(&cfgp, list.Name)
				}
			}
		}
		if job == "rss" {
			if typ == "movie" {
				for idx := range qualis.Arr {
					searcher.SearchMovieRSS(&cfgp, qualis.Arr[idx])
				}
			} else {
				for idx := range qualis.Arr {
					searcher.SearchSerieRSS(&cfgp, qualis.Arr[idx])
				}
			}
		}
		qualis.Close()
	}
	if dbinsert != 0 {
		endjobhistory(dbinsert)
	}
	logger.Log.GlobalLogger.Info(jobended, zap.Stringp("Job", &job), zap.Stringp("config", &cfgp.NamePrefix))
}

func checkmissing(typvar string, listname string) {
	var filesfound []string
	var query, querycount string
	if typvar == "movie" {
		query = "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"
		querycount = "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"
	} else {
		querycount = "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"
		query = "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"
	}
	database.QueryStaticStringArray(false,
		database.CountRowsStaticNoError(&database.Querywithargs{QueryString: querycount, Args: []interface{}{listname}}),
		&database.Querywithargs{QueryString: query, Args: []interface{}{listname}},
		&filesfound)
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			jobImportFileCheck(filesfound[idx], typvar)
		}
	}
	filesfound = nil
}

func checkmissingflag(typvar string, listname string) {
	var query, querycount, queryupdate string
	if typvar == "movie" {
		query = "Select id, missing from movies where listname = ? COLLATE NOCASE"
		querycount = "select count() from movie_files where movie_id = ?"
		queryupdate = "update movies set missing = ? where id = ?"
	} else {
		query = "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)"
		querycount = "select count() from serie_episode_files where serie_episode_id = ?"
		queryupdate = "update serie_episodes set missing = ? where id = ?"
	}
	var missing []database.DbstaticOneIntOneBool
	database.QueryStaticColumnsOneIntOneBool(&database.Querywithargs{QueryString: query, Args: []interface{}{listname}}, &missing)

	var counter, setmissing int
	var set bool
	for idxmovie := range missing {
		counter = database.CountRowsStaticNoError(&database.Querywithargs{QueryString: querycount, Args: []interface{}{missing[idxmovie].Num}})
		set = false
		if counter >= 1 && missing[idxmovie].Bl {
			set = true
			setmissing = 0
		}
		if counter == 0 && !missing[idxmovie].Bl {
			set = true
			setmissing = 1
		}
		if set {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: queryupdate, Args: []interface{}{setmissing, missing[idxmovie].Num}})
		}
	}
	missing = nil
}
