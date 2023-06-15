package utils

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/worker"
	"github.com/pelletier/go-toml/v2"
)

type feedResults struct {
	Series config.MainSerieConfig
	Movies []string
}

const (
	serieepiunmatched = "SerieEpisode not matched episode - serieepisode not found"
	seriedbunmatched  = "SerieEpisode not matched episode - dbserieepisode not found"
	jobstarted        = "Started Job"
	jobended          = "Ended Job"
)

var (
	lastStructure string
)

func insertjobhistory(jobtype string, jobgroup string, jobcategory string) int64 {
	result, err := database.InsertStatic("Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, ?)", jobtype, jobgroup, jobcategory, database.SQLTimeGetNow())
	if err == nil && result != nil {
		return database.InsertRetID(result)
	}
	return 0
}
func endjobhistory(id int64) {
	database.UpdateColumnStatic(database.QueryUpdateHistory, database.SQLTimeGetNow(), id)
}

func InitialFillSeries() {
	logger.Log.Info().Msg("Starting initial DB fill for series")

	database.Refreshunmatchedcached("serie_")
	database.Refreshfilescached("serie_")
	var dbid int64
	var err error
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		dbid = insertjobhistory(logger.StrFeeds, config.SettingsMedia[idxp].Name, logger.StrSeries)
		for idx2 := range config.SettingsMedia[idxp].Lists {
			err = importnewseriessingle(config.SettingsMedia[idxp].NamePrefix, config.SettingsMedia[idxp].Lists[idx2].Name)

			if err != nil {
				logger.Logerror(err, "Import new series failed")
			}
		}
		endjobhistory(dbid)
	}
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		dbid = insertjobhistory(logger.StrDataFull, config.SettingsMedia[idxp].Name, logger.StrSeries)
		getNewFilesMap(config.SettingsMedia[idxp].NamePrefix)
		endjobhistory(dbid)
	}
}

func InitialFillMovies() {
	logger.Log.Info().Msg("Starting initial DB fill for movies")

	database.Refreshunmatchedcached("movie_")
	database.Refreshfilescached("movie_")
	var dbid int64
	var err error
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		dbid = insertjobhistory(logger.StrFeeds, config.SettingsMedia[idxp].Name, logger.StrMovie)
		for idx2 := range config.SettingsMedia[idxp].Lists {
			err = importnewmoviessingle(config.SettingsMedia[idxp].NamePrefix, config.SettingsMedia[idxp].Lists[idx2].Name)
			if err != nil {
				logger.Logerror(err, "Import new movies failed")
			}
		}
		endjobhistory(dbid)
	}

	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		dbid = insertjobhistory(logger.StrDataFull, config.SettingsMedia[idxp].Name, logger.StrMovie)
		getNewFilesMap(config.SettingsMedia[idxp].NamePrefix)
		endjobhistory(dbid)
	}
}

func FillImdb() {
	dbinsert := insertjobhistory(logger.StrData, "RefreshImdb", logger.StrMovie)
	workergroup := worker.WorkerPoolMetadata.Group()
	workergroup.Submit(func() {
		var outputBuf, stdErr bytes.Buffer
		err := parser.ExecCmd(parser.GetImdbFilename(), new(string), "imdb", &outputBuf, &stdErr)
		if err == nil {
			logger.Log.Info().Msg(outputBuf.String())
			//logger.LogAnyInfo("imdb import", logger.LoggerValue{Name: "output", Value: stdoutBuf.String()})
			database.CloseImdb()
			scanner.RemoveFile("./databases/imdb.db")
			scanner.RenameFileSimple("./databases/imdbtemp.db", "./databases/imdb.db")
			database.InitImdbdb()
		}
		outputBuf.Reset()
		stdErr.Reset()
	})
	workergroup.Wait()
	if dbinsert != 0 {
		endjobhistory(dbinsert)
	}
}

func (s *feedResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Series.Close()
	logger.Clear(&s.Movies)
	logger.ClearVar(s)
}

func feeds(cfgpstr string, listname string, templatelist string) (*feedResults, error) {

	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var templateenabled bool
	if i != -1 {
		templateenabled = config.SettingsMedia[cfgpstr].Lists[i].Enabled
	}
	if !templateenabled {
		return nil, logger.ErrDisabled
	}
	listTemplateList, listenabled := config.GetTemplateList(cfgpstr, listname)
	//listmao.Close()
	if !config.CheckGroup("list_", listTemplateList) {
		return nil, errors.New("list template not found")
	}

	if !listenabled || !config.SettingsList["list_"+templatelist].Enabled {
		return nil, errors.New("list template disabled")
	}

	var usetraktserie, usetraktmovie string
	switch config.SettingsList["list_"+templatelist].ListType {
	case "seriesconfig":
		return getseriesconfig(templatelist)
	case "traktpublicshowlist":
		return getTraktUserPublicShowList(listTemplateList)
	case "imdbcsv":
		return getimdbcsv(cfgpstr, listname, listTemplateList)
	case "traktpublicmovielist":
		return getTraktUserPublicMovieList(listTemplateList)
	case "traktmoviepopular":
		usetraktmovie = "popular"
	case "traktmovieanticipated":
		usetraktmovie = "anticipated"
	case "traktmovietrending":
		usetraktmovie = "trending"
	case "traktseriepopular":
		usetraktserie = "popular"
	case "traktserieanticipated":
		usetraktserie = "anticipated"
	case "traktserietrending":
		usetraktserie = "trending"
	case "newznabrss":
		return getnewznabrss(cfgpstr, listname)
	default:
		return nil, errors.New("switch not found")
	}

	if usetraktmovie != "" {
		return gettraktmovielist(usetraktmovie, templatelist, listTemplateList)
	}

	if usetraktserie != "" {
		return gettraktserielist(usetraktserie, templatelist)
	}

	return nil, errors.New("feed config not found")
}

func getseriesconfig(templatelist string) (*feedResults, error) {
	content, err := os.ReadFile(config.SettingsList["list_"+templatelist].SeriesConfigFile)
	if err != nil {
		return nil, errors.New("loading config")
	}
	var feeddata feedResults
	if toml.Unmarshal(content, &feeddata.Series) != nil {
		return nil, errors.New("unmarshal config")
	}
	logger.Clear(&content)
	return &feeddata, nil
}

func getimdbcsv(cfgpstr string, listname string, listTemplateList string) (*feedResults, error) {
	if config.SettingsList["list_"+listTemplateList].URL == "" {
		return nil, errors.New("no url")
	}
	urlv := config.SettingsList["list_"+listTemplateList].URL
	resp, err := apiexternal.WebClient.Do(logger.HTTPGetRequest(&urlv))
	if err != nil || resp == nil {
		return nil, errors.New("csv read")
	}

	defer resp.Body.Close()

	parserimdb := csv.NewReader(resp.Body)
	parserimdb.ReuseRecord = true

	d := feedResults{Movies: make([]string, 0, logger.GlobalCounter[config.SettingsList["list_"+listTemplateList].URL])}
	cfglist := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var listnamefilter string
	var args []interface{}
	if len(config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapLists) >= 1 {
		args = config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapListsInt
		listnamefilter = "listname in (" + logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapLists)-1) + ")"
	}
	var record []string
	//var movieid, ti int
	//var foundmovie, allowed bool

	var movieid int
	var foundmovie bool
	var allowed bool
	for {
		record, err = parserimdb.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("failed to get row")
		}
		if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}
		movieid = 0
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool {
				return strings.EqualFold(elem.Str3, record[1])
			})
			if ti != -1 {
				movieid = database.CacheDBMovie[ti].Num1
			}
		} else {
			movieid = database.QueryIntColumn(database.QueryDbmoviesGetIDByImdb, &record[1])
		}
		//movieid := cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, record[1]) }).Num1

		if movieid != 0 {
			foundmovie = false
			if config.SettingsGeneral.UseMediaCache {
				foundmovie = logger.IndexFunc(&database.CacheMovie, func(elem database.DbstaticOneStringOneInt) bool {
					return elem.Num == movieid && strings.EqualFold(elem.Str, listname)
				}) != -1
			} else {
				foundmovie = database.QueryIntColumn("select count() from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", &movieid, &listname) >= 1
			}
			if !foundmovie && len(config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapLists) >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					foundmovie = logger.IndexFunc(&database.CacheMovie, func(elem database.DbstaticOneStringOneInt) bool {
						return elem.Num == movieid && logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapLists, elem.Str)
					}) != -1
				} else if listnamefilter != "" {
					foundmovie = database.QueryIntColumn("select count() from movies where dbmovie_id = ? and "+listnamefilter, append([]interface{}{&movieid}, args...)...) >= 1
				}
			}
			if foundmovie {
				continue
			}
		} else {
			logger.Log.Debug().Str("imdb", record[1]).Msg("dbmovie not found in cache")
		}
		allowed, _ = importfeed.AllowMovieImport(&record[1], config.SettingsMedia[cfgpstr].Lists[cfglist].TemplateList)
		if allowed {
			d.Movies = append(d.Movies, record[1])
		}
	}
	logger.GlobalCounter[config.SettingsList["list_"+listTemplateList].URL] = len(d.Movies)
	logger.ClearVar(parserimdb)
	logger.Clear(&args)
	logger.Clear(&record)
	logger.Log.Info().Int("entries to parse", len(d.Movies)).Str("url", config.SettingsList["list_"+listTemplateList].URL).Msg("imdb list fetched")
	//logger.LogAnyInfo("imdb list fetched", logger.LoggerValue{Name: "entries to parse", Value: len(d.Movies)}, logger.LoggerValue{Name: "url", Value: config.SettingsList["list_"+listTemplateList].URL})
	return &d, nil
}

func getnewznabrss(cfgpstr string, listname string) (*feedResults, error) {
	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var templatequality string
	if i != -1 {
		templatequality = config.SettingsMedia[cfgpstr].Lists[i].TemplateQuality
	}
	search := searcher.NewSearcher(cfgpstr, templatequality, "", "", 0)
	defer search.Close()
	searchresults, err := search.GetRSSFeed(logger.StrMovie, listname)
	if err != nil {
		return nil, err
	}
	for idxres := range searchresults.Accepted {
		logger.Log.Debug().Str(logger.StrURL, searchresults.Accepted[idxres].NZB.Title).Msg("nzb found - start downloading")
		if searchresults.Accepted[idxres].NzbmovieID != 0 {
			downloader.DownloadMovie(cfgpstr, searchresults.Accepted[idxres].NzbmovieID, &searchresults.Accepted[idxres])
		} else if searchresults.Accepted[idxres].NzbepisodeID != 0 {
			downloader.DownloadSeriesEpisode(cfgpstr, searchresults.Accepted[idxres].NzbepisodeID, &searchresults.Accepted[idxres])
		}
	}
	searchresults.Close()
	return &feedResults{}, nil
}

func getraktapimovielist(usetraktmovie string, cfglistlimit int) (*[]apiexternal.TraktMovie, error) {
	switch usetraktmovie {
	case "popular":
		return apiexternal.TraktAPI.GetMoviePopular(cfglistlimit)
	case "trending":
		return apiexternal.TraktAPI.GetMovieTrending(cfglistlimit)
	case "anticipated":
		return apiexternal.TraktAPI.GetMovieAnticipated(cfglistlimit)
	}
	return nil, nil
}

func gettraktmovielist(usetraktmovie string, templatelist string, listTemplateList string) (*feedResults, error) {
	traktpopular, _ := getraktapimovielist(usetraktmovie, config.SettingsList["list_"+templatelist].Limit)

	if traktpopular == nil || len(*traktpopular) == 0 {
		return nil, logger.Errwrongtype
	}
	results := feedResults{Movies: make([]string, 0, len(*traktpopular))}
	var countermovie int
	for idx := range *traktpopular {
		if (*traktpopular)[idx].Ids.Imdb != "" {
			countermovie = database.QueryIntColumn("select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &(*traktpopular)[idx].Ids.Imdb, &listTemplateList)
			if countermovie == 0 {
				results.Movies = append(results.Movies, (*traktpopular)[idx].Ids.Imdb)
			}
		}
	}
	logger.Clear(traktpopular)
	return &results, nil
}

func gettraktapiserielist(usetraktserie string, cfglistlimit int) (*[]apiexternal.TraktSerie, error) {
	switch usetraktserie {
	case "popular":
		return apiexternal.TraktAPI.GetSeriePopular(cfglistlimit)
	case "trending":
		return apiexternal.TraktAPI.GetSerieTrending(cfglistlimit)
	case "anticipated":
		return apiexternal.TraktAPI.GetSerieAnticipated(cfglistlimit)
	}
	return nil, nil
}
func gettraktserielist(usetraktserie string, templatelist string) (*feedResults, error) {
	traktpopular, _ := gettraktapiserielist(usetraktserie, config.SettingsList["list_"+templatelist].Limit)

	if traktpopular == nil || len(*traktpopular) == 0 {
		return nil, logger.Errwrongtype
	}
	results := feedResults{Series: config.MainSerieConfig{Serie: make([]config.SerieConfig, 0, len(*traktpopular))}}
	//var results feedResults
	for idx := range *traktpopular {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: (*traktpopular)[idx].Title, TvdbID: (*traktpopular)[idx].Ids.Tvdb,
		})
	}
	logger.Clear(traktpopular)
	return &results, nil
}

func getNewFilesMap(cfgpstr string) {

	workergroup := worker.WorkerPoolParse.Group()
	var added int
	var path string
	for idx := range config.SettingsMedia[cfgpstr].Data {
		if !config.CheckGroup("path_", config.SettingsMedia[cfgpstr].Data[idx].TemplatePath) {
			//logger.LogerrorStr(nil, "template", config.SettingsMedia[cfgpstr].Data[idx].TemplatePath, "config not found")
			logger.Log.Error().Err(nil).Str("template", config.SettingsMedia[cfgpstr].Data[idx].TemplatePath).Msg("config not found")
			continue
		}
		path = config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].Data[idx].TemplatePath].Path
		if !scanner.CheckFileExist(&path) {
			logger.Log.Debug().Str(logger.StrPath, path).Str("template", config.SettingsMedia[cfgpstr].Data[idx].TemplatePath).Msg("path not found")
			continue
		}
		addfoundlist := config.SettingsMedia[cfgpstr].Data[idx].AddFoundList
		addfound := config.SettingsMedia[cfgpstr].Data[idx].AddFound
		scanner.WalkdirProcess(path, true, func(file *string, _ *fs.DirEntry) error {
			if !scanner.Filterfile(file, false, config.SettingsMedia[cfgpstr].Data[idx].TemplatePath) {
				return nil
			}
			if structure.CheckUnmatched(cfgpstr, file) {
				return nil
			}
			if structure.CheckFiles(cfgpstr, file) {
				return nil
			}
			pathv := *file
			if cfgpstr[:5] == logger.StrSerie {
				added++
				workergroup.Submit(func() {
					err := jobImportSeriesParseV2(pathv, true, cfgpstr, addfoundlist)
					if err != nil {
						//logger.LogAnyError(err, "Error Importing Serie", logger.LoggerValue{Name: path, Value: pathv})
						logger.Log.Error().Err(err).Str("Path", pathv).Msg("Error Importing Serie")
					}
				})
			} else {
				added++
				workergroup.Submit(func() {
					err := jobImportMovieParseV2(pathv, true, cfgpstr, addfoundlist, addfound)
					if err != nil && err.Error() != "movie ignored" {
						//logger.LogAnyError(err, "Error Importing Movie", logger.LoggerValue{Name: path, Value: pathv})
						logger.Log.Error().Err(err).Str("Path", pathv).Msg("Error Importing Movie")
					}
				})
			}
			return nil
		})
	}
	if added >= 1 {
		workergroup.Wait()
	}
}

func SingleJobs(typ string, job string, cfgpstr string, listname string, force bool) {

	if config.SettingsGeneral.SchedulerDisabled && !force {
		logger.Log.Info().Str(logger.StrJob, job).Msg("skipped job")
		return
	}

	logjob(cfgpstr, job, listname, jobstarted)

	category := logger.StrSeries
	if typ == logger.StrMovie {
		category = logger.StrMovie
	}

	var jobprefix string
	if cfgpstr != "" {
		jobprefix = config.SettingsMedia[cfgpstr].NamePrefix
	}
	dbinsert := insertjobhistory(job, jobprefix, category)

	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag || job == logger.StrClearHistory || job == logger.StrFeeds || job == logger.StrCheckMissing || job == logger.StrCheckMissingFlag {
		qualis := make([]string, 0, len(config.SettingsMedia[cfgpstr].Lists)/2)

		if listname != "" {
			runjoblistfunc(job, cfgpstr, listname, typ)
		} else {
			var breakv bool
			for idx := range config.SettingsMedia[cfgpstr].Lists {
				if job == logger.StrRss {
					breakv = false
					for idxi := range qualis {
						if qualis[idxi] == config.SettingsMedia[cfgpstr].Lists[idx].TemplateQuality {
							breakv = true
							break
						}
					}
					if !breakv {
						qualis = append(qualis, config.SettingsMedia[cfgpstr].Lists[idx].TemplateQuality)
					}
				}
				runjoblistfunc(job, cfgpstr, config.SettingsMedia[cfgpstr].Lists[idx].Name, typ)
			}
		}

		if job == logger.StrRss {
			database.Refreshhistorycache(typ)
			var results *searcher.SearchResults
			var err error
			for idx := range qualis {
				results, err = searcher.SearchRSS(cfgpstr, qualis[idx], category, false)
				if err != nil && err != logger.ErrDisabled {
					//logger.LogAnyError(err, "Search RSS Failed", logger.LoggerValue{Name: "quality", Value: qualis[idx]}, logger.LoggerValue{Name: "typ", Value: typ})
					logger.Log.Error().Err(err).Str("quality", qualis[idx]).Str("typ", typ).Msg("Search RSS Failed")
				} else {
					if results != nil && len(results.Accepted) >= 1 {
						results.Download(category, cfgpstr)
					}
				}
				results.Close()
			}
		}
		logger.Clear(&qualis)
	} else {
		runjoblistfunc(job, cfgpstr, "", typ)
	}
	if dbinsert != 0 {
		endjobhistory(dbinsert)
	}
	logjob(cfgpstr, job, listname, jobended)
}

func logjob(cfgpstr string, job string, listname string, msg string) {
	//logger.LogAnyInfo(msg, logger.LoggerValue{Name: "cfg", Value: cfgpstr}, logger.LoggerValue{Name: "list", Value: listname}, logger.LoggerValue{Name: "Routines", Value: runtime.NumGoroutine()})
	logger.Log.Info().Str("cfg", cfgpstr).Str(logger.StrJob, job).Str("list", listname).Int("Num Goroutines", runtime.NumGoroutine()).Msg(msg)
}
func getintervals(cfgpstr string, missing bool) (int, int) {

	if len(config.SettingsMedia[cfgpstr].Data) >= 1 {
		if !config.CheckGroup("path_", config.SettingsMedia[cfgpstr].Data[0].TemplatePath) {
			return 0, 0
		}
		if missing {
			return config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].Data[0].TemplatePath].MissingScanInterval, config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].Data[0].TemplatePath].MissingScanReleaseDatePre
		}
		return config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].Data[0].TemplatePath].UpgradeScanInterval, 0
	}
	return 0, 0
}
func runjoblistfunc(job string, cfgpstr string, listname string, typ string) {
	if typ == logger.StrMovie {
		database.RefreshMoviesCache()
		database.Refreshmoviestitlecache()
	} else {
		database.Refreshseriestitlecache()
	}
	var searchinterval int
	switch job {
	case logger.StrData, logger.StrDataFull:
		if cfgpstr == "" {
			return
		}
		database.Refreshunmatchedcached(cfgpstr)
		database.Refreshfilescached(cfgpstr)
		getNewFilesMap(cfgpstr)
	case logger.StrCheckMissing:
		checkmissing(typ, listname)
	case "cleanqueue":
		worker.Cleanqueue()
	case logger.StrCheckMissingFlag:
		checkmissingflag(typ, listname)
	case logger.StrReachedFlag:
		if cfgpstr == "" {
			return
		}
		if typ == logger.StrMovie {
			checkreachedmoviesflag(cfgpstr, listname)
		} else {
			checkreachedepisodesflag(cfgpstr, listname)
		}
	case logger.StrStructure:
		if cfgpstr == "" {
			return
		}
		structurefolders(cfgpstr, typ)
	case logger.StrRssSeasons:
		if cfgpstr == "" {
			return
		}
		database.Refreshhistorycache(typ)
		searcher.SearchSeriesRSSSeasons(cfgpstr)
	case logger.StrRssSeasonsAll:
		if cfgpstr == "" {
			return
		}
		database.Refreshhistorycache(typ)
		searcher.SearchSeriesRSSSeasonsAll(cfgpstr)
	case "refreshinc":
		if typ == logger.StrMovie {
			RefreshMoviesInc()
		} else {
			RefreshSeriesInc()
		}
	case "refresh":
		if typ == logger.StrMovie {
			RefreshMovies()
		} else {
			RefreshSeries()
		}
	case logger.StrClearHistory:
		if typ == logger.StrMovie {
			database.DeleteRow(false, "movie_histories", "movie_id in (Select id from movies where listname = ? COLLATE NOCASE)", &listname)
		} else {
			database.DeleteRow(false, "serie_episode_histories", "serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &listname)
		}
	case logger.StrFeeds:
		if cfgpstr == "" {
			return
		}
		var err error
		if typ == logger.StrMovie {
			err = importnewmoviessingle(cfgpstr, listname)
		} else {
			err = importnewseriessingle(cfgpstr, listname)
		}
		if err != nil {
			logger.Log.Error().Err(err).Str("Listname", listname).Msg("import feeds failed")
		}
	case logger.StrRss:
		database.Refreshhistorycache(typ)
	case logger.StrSearchMissingFull:
		if cfgpstr == "" {
			return
		}
		runjobsearch(typ, cfgpstr, true, 0, false)
	case logger.StrSearchMissingInc:
		if cfgpstr == "" {
			return
		}
		searchinterval = config.SettingsMedia[cfgpstr].SearchmissingIncremental
		if searchinterval == 0 {
			searchinterval = 20
		}
		runjobsearch(typ, cfgpstr, true, searchinterval, false)
	case logger.StrSearchUpgradeFull:
		if cfgpstr == "" {
			return
		}
		runjobsearch(typ, cfgpstr, false, 0, false)
	case logger.StrSearchUpgradeInc:
		if cfgpstr == "" {
			return
		}
		searchinterval = config.SettingsMedia[cfgpstr].SearchupgradeIncremental
		if searchinterval == 0 {
			searchinterval = 20
		}
		runjobsearch(typ, cfgpstr, false, searchinterval, false)
	case logger.StrSearchMissingFullTitle:
		if cfgpstr == "" {
			return
		}
		runjobsearch(typ, cfgpstr, true, 0, true)
	case logger.StrSearchMissingIncTitle:
		if cfgpstr == "" {
			return
		}
		searchinterval = config.SettingsMedia[cfgpstr].SearchmissingIncremental
		if searchinterval == 0 {
			searchinterval = 20
		}
		runjobsearch(typ, cfgpstr, true, searchinterval, true)
	case logger.StrSearchUpgradeFullTitle:
		if cfgpstr == "" {
			return
		}
		runjobsearch(typ, cfgpstr, false, 0, true)
	case logger.StrSearchUpgradeIncTitle:
		if cfgpstr == "" {
			return
		}
		searchinterval = config.SettingsMedia[cfgpstr].SearchupgradeIncremental
		if searchinterval == 0 {
			searchinterval = 20
		}
		runjobsearch(typ, cfgpstr, false, searchinterval, true)
	default:
		logger.Log.Error().Err(nil).Str(logger.StrJob, job).Msg("Switch not found")
	}
}

func runjobsearch(typ string, cfgpstr string, searchmissing bool, searchinterval int, searchtitle bool) {
	database.Refreshhistorycache(typ)
	scaninterval, scandatepre := getintervals(cfgpstr, searchmissing)
	var q database.Querywithargs
	q.DontCache = true

	args := config.SettingsMedia[cfgpstr].ListsInt
	listcount := len(args)
	if searchinterval >= 1 {
		q.Limit = searchinterval
	}

	if typ == logger.StrMovie {
		q.Table = "movies"
		q.Select = "movies.id"
		q.InnerJoin = "dbmovies on dbmovies.id=movies.dbmovie_id"
		q.OrderBy = "movies.Lastscan asc"

		if searchmissing {
			q.Where = "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (" + logger.StringsRepeat("?", ",?", listcount-1) + ")"
		} else {
			q.Where = "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (" + logger.StringsRepeat("?", ",?", listcount-1) + ")"
		}
		if scaninterval != 0 {
			q.Where += " and (movies.lastscan is null or movies.Lastscan < ?)"
			args = append(args, logger.TimeGetNow().AddDate(0, 0, 0-scaninterval))
		}
		if scandatepre != 0 {
			q.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			args = append(args, logger.TimeGetNow().AddDate(0, 0, 0+scandatepre))
		}
		//searcher.SearchMovie(cfgpstr, searchmissing, searchinterval, searchtitle)
	} else {
		q.Table = "serie_episodes"
		q.Select = "serie_episodes.id"
		q.OrderBy = "Lastscan asc"
		q.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

		if searchmissing {
			q.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (" + logger.StringsRepeat("?", ",?", listcount-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
		} else {
			q.Where = database.QuerySearchSeriesUpgrade + " in (" + logger.StringsRepeat("?", ",?", listcount-1) + ")"
		}
		if scaninterval != 0 {
			q.Where += " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)"
			args = append(args, logger.TimeGetNow().AddDate(0, 0, 0-scaninterval))
			if scandatepre != 0 {
				q.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
				args = append(args, logger.TimeGetNow().AddDate(0, 0, 0+scandatepre))
			}
		} else {
			if scandatepre != 0 {
				q.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
				args = append(args, logger.TimeGetNow().AddDate(0, 0, 0+scandatepre))
			}
		}

		//searcher.SearchSerie(cfgpstr, searchmissing, searchinterval, searchtitle)
	}
	q.Buildquery(true)
	count := database.QueryIntColumn(q.QueryString, args...)
	q.Buildquery(false)

	tbl := database.QueryStaticUintArrayNoError(false, count, q.QueryString, args...)
	logger.Clear(&args)
	var err error
	var results *searcher.SearchResults
	for idx := range *tbl {
		switch typ {
		case logger.StrMovie:
			results, err = searcher.MovieSearch(cfgpstr, (*tbl)[idx], false, searchtitle)
		default:
			results, err = searcher.SeriesSearch(cfgpstr, (*tbl)[idx], false, searchtitle)
		}
		if err != nil {
			if err != nil && err != logger.ErrDisabled {
				//logger.LogAnyError(err, "Search Failed", logger.LoggerValue{Name: "id", Value: (*tbl)[idx]}, logger.LoggerValue{Name: "typ", Value: typ})
				logger.Log.Error().Str("typ", typ).Err(err).Uint("id", (*tbl)[idx]).Msg("Search Failed")
			}
		} else {
			if results != nil && len(results.Accepted) >= 1 {
				results.Download(typ, cfgpstr)
			}
		}
		results.Close()

		//_, err := searcher.SearchMyMedia(cfgpstr, database.QueryStringColumn(database.QueryMoviesGetQualityByID, tbl[idx]), typ, typ, 0, 0, false, tbl[idx], searchtitle)

	}
	logger.Clear(tbl)
}

func checkmissing(typvar string, listname string) {
	querycount := "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"
	query := "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"
	if typvar == logger.StrMovie {
		query = "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"
		querycount = "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"
	}

	files := database.QueryStaticStringArray(false,
		database.QueryIntColumn(querycount, &listname),
		query, &listname)
	var counter int
	var tbl *[]database.DbstaticTwoInt
	var subquerycount, table, updatetable string
	for fileidx := range *files {
		if scanner.CheckFileExist(&(*files)[fileidx]) {
			continue
		}
		query = database.QuerySerieEpisodeFilesGetIDEpisodeIDByLocation
		subquerycount = database.QuerySerieEpisodeFilesCountByEpisodeID
		table = "serie_episode_files"
		updatetable = "serie_episodes"
		if typvar == logger.StrMovie {
			query = database.QueryMovieFilesGetIDMovieIDByLocation
			subquerycount = database.QueryMovieFilesCountByMovieID
			table = "movie_files"
			updatetable = "movies"
		}
		tbl = database.QueryStaticColumnsTwoInt(true, 1, query, &(*files)[fileidx])
		for tblidx := range *tbl {
			logger.Log.Debug().Str(logger.StrFile, (*files)[fileidx]).Msg("File was removed")
			if database.DeleteRowStatic(false, "Delete from "+table+" where id = ?", (*tbl)[tblidx].Num1) == nil {
				counter = database.QueryIntColumn(subquerycount, (*tbl)[tblidx].Num2)
				if counter == 0 {
					database.UpdateColumnStatic("Update "+updatetable+" set missing = ? where id = ?", 1, (*tbl)[tblidx].Num2)
				}
			}
		}
		logger.Clear(tbl)
	}
	logger.Clear(files)
}

func checkmissingflag(typvar string, listname string) {
	query := database.QuerySerieEpisodeFilesGetIDMissingByListname
	querysize := database.QuerySerieEpisodeFilesCountByListname
	querycount := database.QuerySerieEpisodeFilesCountByEpisodeID
	queryupdate := "update serie_episodes set missing = ? where id = ?"
	if typvar == logger.StrMovie {
		query = database.QueryMoviesGetIDMissingByListname
		querysize = database.QueryMoviesCountByListname
		querycount = database.QueryMovieFilesCountByMovieID
		queryupdate = "update movies set missing = ? where id = ?"
	}

	tbl := database.QueryStaticColumnsOneIntOneBool(database.QueryIntColumn(querysize, &listname), query, &listname)
	var counter int
	var set bool
	var setmissing int
	for idx := range *tbl {
		counter = database.QueryIntColumn(querycount, (*tbl)[idx].Num)
		set = false
		setmissing = 0
		if counter >= 1 && (*tbl)[idx].Bl {
			set = true
		}
		if counter == 0 && !(*tbl)[idx].Bl {
			set = true
			setmissing = 1
		}
		if set {
			database.UpdateColumnStatic(queryupdate, setmissing, (*tbl)[idx].Num)
		}
	}
	logger.Clear(tbl)
}
