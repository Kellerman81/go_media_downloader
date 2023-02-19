package searcher

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

type searchstruct struct {
	mediatype     string
	thetvdbid     int
	season        int
	useseason     bool
	movieid       uint
	forceDownload bool
	titlesearch   bool
	episodeid     uint
}
type SearchResults struct {
	Nzbs     []apiexternal.Nzbwithprio
	Rejected []apiexternal.Nzbwithprio
	mu       *sync.Mutex
	//Searched []string
}
type Searcher struct {
	Cfgp             *config.MediaTypeConfig
	Quality          string
	QualityCfg       *config.QualityConfig
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	MinimumPriority  int
	Movie            database.Movie
	SerieEpisode     database.SerieEpisode
	imdb             string
	year             int
	title            string
	identifier       string
	loweridentifier  string
	season           string
	episode          string
	thetvdbid        int
	Listcfg          config.ListsConfig
	AlternateTitles  []string
}

const spacenotfound = " not found"
const notwantedmovie = "Not Wanted Movie"
const skippedindexer = "Skipped Indexer"
const queryqualprofmovies = "select quality_profile from movies where id = ?"
const queryqualprofseries = "select quality_profile from serie_episodes where serie_id = ?"
const querydbserieidseries = "select dbserie_id from series where id = ?"
const querytvdbidseries = "select thetvdb_id from dbseries where id = ?"
const deniedbyregex = "Denied by Regex"
const queryidmoviesbylist = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
const skippedstr = "Skipped"
const querymoviefiles = "select id from movie_files where movie_id = ?"
const queryseriefiles = "select id from serie_episode_files where serie_episode_id = ?"

var errNoQuality = errors.New("quality not found")
var errNoList = errors.New("list not found")
var errNoRegex = errors.New("regex not found")
var errNoIndexer = errors.New("indexer not found")
var errOther = errors.New("other error")
var errIndexerDisabled = errors.New("indexer disabled")
var errSearchDisabled = errors.New("search disabled")
var errUpgradeDisabled = errors.New("upgrade disabled")
var errToWait = errors.New("please wait")

func (s *searchstruct) close() {
	if s == nil {
		return
	}
	s = nil
}

func getintervals(cfgp *config.MediaTypeConfig, missing bool) (int, int) {

	if len(cfgp.Data) >= 1 {
		if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
			return 0, 0
		}
		if missing {
			return config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanInterval, config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanReleaseDatePre
		} else {
			return config.Cfg.Paths[cfgp.Data[0].TemplatePath].UpgradeScanInterval, 0
		}
	}
	return 0, 0
}
func SearchMovie(cfgp *config.MediaTypeConfig, missing bool, jobcount int, titlesearch bool) *database.Querywithargs {

	scaninterval, scandatepre := getintervals(cfgp, missing)

	q := new(database.Querywithargs)
	q.Query.Select = "movies.id"
	q.Query.InnerJoin = "dbmovies on dbmovies.id=movies.dbmovie_id"
	q.Query.OrderBy = "movies.Lastscan asc"
	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	q.Args = make([]interface{}, argcount)
	for i := range cfgp.Lists {
		q.Args[i] = cfgp.Lists[i].Name
	}
	q.DontCache = true
	if missing {
		q.Query.Where = "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	} else {
		q.Query.Where = "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	}
	var i int
	if scaninterval != 0 {
		i++
		q.Query.Where += " and (movies.lastscan is null or movies.Lastscan < ?)"
		q.Args[len(cfgp.Lists)] = logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)
	}
	if scandatepre != 0 {
		q.Query.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
		q.Args[len(cfgp.Lists)+i] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
	}
	if jobcount >= 1 {
		q.Query.Limit = jobcount
	}
	return q
}

func SearchMovieSingle(movieid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var quality string
	database.QueryColumn(&database.Querywithargs{QueryString: queryqualprofmovies, Args: []interface{}{movieid}}, &quality)
	searchstuff(cfgp, quality, "movie", &searchstruct{mediatype: "movie", movieid: movieid, forceDownload: false, titlesearch: titlesearch})
}

func SearchSerieSingle(serieid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var episodes []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: "select id from serie_episodes where serie_id = ?", Args: []interface{}{serieid}}, &episodes)
	if len(episodes) >= 1 {
		for idx := range episodes {
			SearchSerieEpisodeSingle(episodes[idx], cfgp, titlesearch)
		}
	}
	episodes = nil
}

func SearchSerieSeasonSingle(serieid uint, season string, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var episodes []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: "select id from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", Args: []interface{}{serieid, season}}, &episodes)
	if len(episodes) >= 1 {
		for idx := range episodes {
			SearchSerieEpisodeSingle(episodes[idx], cfgp, titlesearch)
		}
	}
	episodes = nil
}

func SearchSerieRSSSeasonSingle(serieid uint, season int, useseason bool, cfgp *config.MediaTypeConfig) {
	var qualstr string
	database.QueryColumn(&database.Querywithargs{QueryString: queryqualprofseries, Args: []interface{}{serieid}}, &qualstr)
	if qualstr == "" {
		return
	}
	var dbserieid, tvdb uint
	database.QueryColumn(&database.Querywithargs{QueryString: querydbserieidseries, Args: []interface{}{serieid}}, &dbserieid)
	if database.QueryColumn(&database.Querywithargs{QueryString: querytvdbidseries, Args: []interface{}{dbserieid}}, &tvdb) != nil {
		return
	}
	searchstuff(cfgp, qualstr, "rssseason", &searchstruct{mediatype: "series", thetvdbid: int(tvdb), season: season, useseason: useseason})
}
func SearchSeriesRSSSeasons(cfgpstr string) {
	cfgp := config.Cfg.Media[cfgpstr]
	defer cfgp.Close()
	argcount := len(cfgp.Lists)
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	var series []database.DbstaticTwoInt
	database.QueryStaticColumnsTwoInt(&database.Querywithargs{DontCache: true, QueryString: "select id, dbserie_id from series where listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != 0 and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1", Args: whereArgs}, &series)

	whereArgs = nil
	if len(series) >= 1 {
		var seasons []string
		queryseason := "select distinct season from dbserie_episodes where dbserie_id = ? and season != ''"
		for idx := range series {
			seasons = []string{}
			database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: queryseason, Args: []interface{}{series[idx].Num2}}, &seasons)
			for idxseas := range seasons {
				SearchSerieRSSSeasonSingle(uint(series[idx].Num1), logger.StringToInt(seasons[idxseas]), true, &cfgp)
			}
		}
		seasons = nil
	}
	series = nil
}
func SearchSerieEpisodeSingle(episodeid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var quality string
	database.QueryColumn(&database.Querywithargs{QueryString: queryqualprofseries, Args: []interface{}{episodeid}}, &quality)
	searchstuff(cfgp, quality, "series", &searchstruct{mediatype: "series", episodeid: episodeid, forceDownload: false, titlesearch: titlesearch})
}
func SearchSerie(cfgp *config.MediaTypeConfig, missing bool, jobcount int, titlesearch bool) *database.Querywithargs {
	scaninterval, scandatepre := getintervals(cfgp, missing)

	q := new(database.Querywithargs)
	q.Query.Select = "serie_episodes.id"
	q.Query.OrderBy = "Lastscan asc"
	q.Query.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	q.Args = make([]interface{}, argcount)
	for i := range cfgp.Lists {
		q.Args[i] = cfgp.Lists[i].Name
	}
	q.DontCache = true
	if missing {
		q.Query.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
	} else {
		q.Query.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	}
	if scaninterval != 0 {
		q.Query.Where += " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)"
		q.Args[len(cfgp.Lists)] = logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)
		if scandatepre != 0 {
			q.Query.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			q.Args[len(cfgp.Lists)+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		if scandatepre != 0 {
			q.Query.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			q.Args[len(cfgp.Lists)] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		q.Query.Limit = jobcount
	}
	return q
}
func Searchlist(cfgp *config.MediaTypeConfig, table string, titlesearch bool, qu *database.Querywithargs) {
	var missingepisode []uint
	database.QueryStaticColumnsOneUintQueryObject(table, qu, &missingepisode)
	if len(missingepisode) >= 1 {
		typemovies := strings.HasPrefix(cfgp.NamePrefix, "movie_")
		for idx := range missingepisode {
			if typemovies {
				//workergroup.Submit(func() {
				SearchMovieSingle(missingepisode[idx], cfgp, titlesearch)
				//})
			} else {
				//workergroup.Submit(func() {
				SearchSerieEpisodeSingle(missingepisode[idx], cfgp, titlesearch)
				//})
			}
		}
	}
	qu.Close()
	missingepisode = nil
}

func SearchSerieRSS(cfgp *config.MediaTypeConfig, quality string) {
	searchstuff(cfgp, quality, "rss", &searchstruct{mediatype: "series"})
}

func getnzbresults(cfgp *config.MediaTypeConfig, quality string, searchtype string, data *searchstruct) (*SearchResults, error) {
	switch searchtype {
	case "rss":
		return SearchRSS(cfgp, quality, data.mediatype, false)
	case "rssseason":
		return SearchSeriesRSSSeason(cfgp, quality, data.mediatype, data.thetvdbid, data.season, data.useseason)
	case "movie":
		return MovieSearch(cfgp, data.movieid, false, data.titlesearch)
	case "series":
		return SeriesSearch(cfgp, data.episodeid, false, data.titlesearch)
	}
	return nil, errors.New("no match " + searchtype)
}
func searchstuff(cfgp *config.MediaTypeConfig, quality string, searchtype string, data *searchstruct) {
	results, err := getnzbresults(cfgp, quality, searchtype, data)

	if err != nil {
		logger.Log.GlobalLogger.Error("Search failed", zap.Error(err), zap.Any("struct", data))
		data.close()
		return
	}
	var downloaded []uint
	for idx := range results.Nzbs {
		if results.Nzbs[idx].NzbmovieID != 0 {
			if slices.Contains(downloaded, results.Nzbs[idx].NzbmovieID) {
				break
			}
		}
		if results.Nzbs[idx].NzbepisodeID != 0 {
			if slices.Contains(downloaded, results.Nzbs[idx].NzbepisodeID) {
				break
			}
		}
		logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.Stringp("title", &results.Nzbs[idx].NZB.Title), zap.Intp("minimum prio", &results.Nzbs[idx].MinimumPriority), zap.Intp("prio", &results.Nzbs[idx].ParseInfo.Priority), zap.Stringp("quality", &results.Nzbs[idx].QualityTemplate))

		if data.mediatype == "movie" {
			downloaded = append(downloaded, results.Nzbs[idx].NzbmovieID)
			downloader.DownloadMovie(cfgp, results.Nzbs[idx].NzbmovieID, &results.Nzbs[idx])
		} else {
			downloaded = append(downloaded, results.Nzbs[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(cfgp, results.Nzbs[idx].NzbepisodeID, &results.Nzbs[idx])
		}
	}
	results.Close()
	data.close()
}

func SearchMovieRSS(cfgp *config.MediaTypeConfig, quality string) {
	searchstuff(cfgp, quality, "rss", &searchstruct{mediatype: "movie"})
}

func (s *SearchResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	if len(s.Nzbs) >= 1 {
		for idx := range s.Nzbs {
			s.Nzbs[idx].QualityCfg.Close()
			s.Nzbs[idx].Close()
		}
	}
	s.Nzbs = nil
	if len(s.Rejected) >= 1 {
		for idx := range s.Rejected {
			s.Rejected[idx].QualityCfg.Close()
			s.Rejected[idx].Close()
		}
	}
	s.Rejected = nil
	s = nil
}

func (s *Searcher) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Listcfg.Close()
	s.Movie.Close()
	s.SerieEpisode.Close()
	s.AlternateTitles = nil
	s = nil
}

// searchGroupType == movie || series
func SearchRSS(cfgp *config.MediaTypeConfig, quality string, searchGroupType string, fetchall bool) (*SearchResults, error) {
	if !config.Check("quality_" + quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found", zap.Stringp("Searched: ", &quality))
		return nil, errNoQuality
	}

	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	var processedindexer int
	cfgqual := config.Cfg.Quality[quality]
	for idx := range config.Cfg.Quality[quality].Indexer {
		indexertemplate := config.Cfg.Quality[quality].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Rssenabled {
			//processedindexer++
			continue
		}
		workergroup.Submit(func() {
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          quality,
				QualityCfg:       &cfgqual,
				SearchGroupType:  searchGroupType,
				SearchActionType: "rss",
			}).rsssearchindexer(quality, indexertemplate, fetchall, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	cfgqual.Close()
	//dl := s.queryindexers(s.Quality, "rss", fetchall, &processedindexer, false, queryparams{})

	if processedindexer == 0 && len(config.Cfg.Quality[quality].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer == 0 {
		return nil, errors.New("no indexer searched")
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) rsssearchindexer(quality string, indexer string, fetchall bool, dl *SearchResults) bool {
	defer s.Close()
	// if addsearched(dl, search.indexer+search.quality) {
	// 	return true
	// }
	nzbindexer := new(apiexternal.NzbIndexer)
	cats, maxloop, maxentries, erri := s.initIndexer(quality, indexer, "rss", nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			// nzbindexer.Close()
			return true
		}
		if erri == errToWait {
			time.Sleep(10 * time.Second)
			// nzbindexer.Close()
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &indexer), zap.Error(erri))
		// nzbindexer.Close()
		return false
	}

	if fetchall {
		nzbindexer.LastRssID = ""
	}
	if maxentries == 0 {
		maxentries = 10
	}
	if maxloop == 0 {
		maxloop = 2
	}

	nzbs, _, lastids, nzberr := apiexternal.QueryNewznabRSSLast(nzbindexer, maxentries, cats, maxloop)
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.Stringp("indexer", &indexer), zap.Error(nzberr))
		// nzbindexer.Close()
		return false
	}
	if !fetchall && lastids != "" && len((nzbs.Arr)) >= 1 {
		addrsshistory(nzbindexer.URL, lastids, s.Quality, s.Cfgp.NamePrefix)
	}
	if nzbs != nil && len((nzbs.Arr)) >= 1 {
		logger.Log.GlobalLogger.Debug("Search RSS ended - found entries", zap.Int("entries", len((nzbs.Arr))), zap.Stringp("indexer", &nzbindexer.Name))
		s.parseentries(nzbs, dl, quality, indexer, "", false)
	}
	nzbs.Close()
	// nzbindexer.Close()
	return true
}

// searchGroupType == movie || series
func SearchSeriesRSSSeason(cfgp *config.MediaTypeConfig, quality string, searchGroupType string, thetvdbid int, season int, useseason bool) (*SearchResults, error) {
	if !config.Check("quality_" + quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	var processedindexer int
	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	cfgqual := config.Cfg.Quality[quality]
	for idx := range config.Cfg.Quality[quality].Indexer {
		indexertemplate := config.Cfg.Quality[quality].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Rssenabled {
			processedindexer++
			continue
		}
		workergroup.Submit(func() {
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          quality,
				QualityCfg:       &cfgqual,
				SearchGroupType:  searchGroupType,
				SearchActionType: "rss",
			}).rssqueryseriesindexer(quality, indexertemplate, thetvdbid, season, useseason, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	cfgqual.Close()

	if processedindexer == 0 && len(config.Cfg.Quality[quality].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer == 0 {
		return nil, errors.New("no indexer searched")
	}
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) rssqueryseriesindexer(quality string, indexer string, thetvdbid int, season int, useseason bool, dl *SearchResults) bool {
	defer s.Close()
	nzbindexer := new(apiexternal.NzbIndexer)
	cats, _, _, erri := s.initIndexer(quality, indexer, "api", nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			// nzbindexer.Close()
			return true
		}
		if erri == errToWait {
			time.Sleep(10 * time.Second)
			// nzbindexer.Close()
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &indexer), zap.Error(erri))
		// nzbindexer.Close()
		return false
	}

	nzbs, _, nzberr := apiexternal.QueryNewznabTvTvdb(nzbindexer, thetvdbid, cats, season, 0, useseason, false)
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.Stringp("indexer", &indexer), zap.Error(nzberr))
		// nzbindexer.Close()
		return false
	}

	if nzbs != nil && len((nzbs.Arr)) >= 1 {
		s.parseentries(nzbs, dl, quality, indexer, "", false)
		nzbs.Close()
		return true
	}
	return false
	// nzbindexer.Close()
}

func (s *Searcher) checkhistory(quality string, indexer string, historyurlcache *cache.Return, historytitlecache *cache.Return, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Check History
	if len(entry.NZB.DownloadURL) > 1 {
		if slices.Contains(historyurlcache.Value.(logger.InStringArrayStruct).Arr, entry.NZB.DownloadURL) {
			denynzb("Already downloaded (Url)", entry, dl)
			// historycache.Close()
			return true
		}
	}
	if config.QualityIndexerByQualityAndTemplateGetFieldBool(quality, indexer, "HistoryCheckTitle") && len(entry.NZB.Title) > 1 {
		if slices.Contains(historytitlecache.Value.(logger.InStringArrayStruct).Arr, entry.NZB.Title) {
			denynzb("Already downloaded (Title)", entry, dl)
			// historycache.Close()
			return true
		}
	}
	return false
}
func (s *Searcher) checkcorrectid(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {

	if s.SearchActionType == "rss" {
		return false
	}
	if strings.EqualFold(s.SearchGroupType, "movie") && entry.NZB.IMDBID != "" && s.imdb != "" {
		//Check Correct Imdb
		if strings.TrimLeft(strings.TrimPrefix(s.imdb, "tt"), "0") != strings.TrimLeft(strings.TrimPrefix(entry.NZB.IMDBID, "tt"), "0") {
			denynzb("Imdb not match", entry, dl)
			return true
		}
	}
	if !strings.EqualFold(s.SearchGroupType, "series") {
		return false
	}
	if entry.NZB.TVDBID != 0 && s.thetvdbid >= 1 && s.thetvdbid != entry.NZB.TVDBID {
		denynzb("Tvdbid not match", entry, dl)
		return true
	}
	return false
}
func (s *Searcher) getmediadata(entry *apiexternal.Nzbwithprio, dl *SearchResults, addinlist string, addifnotfound bool) bool {

	if strings.EqualFold(s.SearchGroupType, "movie") {
		if s.SearchActionType == "rss" {
			//Filter RSS Movies
			if addinlist != "" && s.Listcfg.Name != s.Cfgp.ListsMap[addinlist].TemplateList {
				s.Listcfg = config.Cfg.Lists[s.Cfgp.ListsMap[addinlist].TemplateList]
			}
			if s.getmovierss(entry, addinlist, addifnotfound, s.Cfgp, dl) {
				return true
			}
			entry.WantedTitle = s.title
			entry.QualityTemplate = s.Movie.QualityProfile
			//Check Minimal Priority
			entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, s.Cfgp, entry.QualityCfg)

			if entry.MinimumPriority != 0 {
				if s.Movie.DontUpgrade {
					denynzb("Upgrade disabled", entry, dl)
					return true
				}
			} else {
				if s.Movie.DontSearch {
					denynzb("Search disabled", entry, dl)
					return true
				}
			}
		} else {
			entry.NzbmovieID = s.Movie.ID
			entry.Dbid = s.Movie.DbmovieID
			entry.QualityTemplate = s.Movie.QualityProfile
			entry.QualityCfg = s.QualityCfg
			entry.MinimumPriority = s.MinimumPriority
			if s.MinimumPriority == 0 {
				entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, s.Cfgp, entry.QualityCfg)
			}
			entry.ParseInfo.MovieID = s.Movie.ID
			entry.ParseInfo.DbmovieID = s.Movie.DbmovieID
			entry.WantedTitle = s.title
			entry.WantedAlternates = s.AlternateTitles
		}

		//Check QualityProfile
		if !config.Check("quality_" + entry.QualityTemplate) {
			denynzb("Unknown quality", entry, dl)
			return true
		}

		return false
	}

	//Parse Series
	if s.SearchActionType == "rss" {
		//Filter RSS Series
		if s.getserierss(entry, s.Cfgp, dl) {
			return true
		}
		entry.QualityTemplate = s.SerieEpisode.QualityProfile
		if s.Quality != entry.QualityTemplate {
			s.Quality = entry.QualityTemplate
			*entry.QualityCfg = config.Cfg.Quality[entry.QualityTemplate]
		}
		if entry.QualityCfg == nil {
			*entry.QualityCfg = config.Cfg.Quality[entry.QualityTemplate]
		}
		entry.WantedTitle = s.title

		//Check Minimum Priority
		entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, s.Cfgp, entry.QualityCfg)

		if entry.MinimumPriority != 0 {
			if s.SerieEpisode.DontUpgrade {
				denynzb("Upgrade disabled", entry, dl)
				return true
			}
		} else {
			if s.SerieEpisode.DontSearch {
				denynzb("Search disabled", entry, dl)
				return true
			}
		}
	} else {
		entry.NzbepisodeID = s.SerieEpisode.ID
		entry.Dbid = s.SerieEpisode.DbserieID
		entry.QualityTemplate = s.SerieEpisode.QualityProfile
		entry.QualityCfg = s.QualityCfg
		entry.MinimumPriority = s.MinimumPriority
		if s.MinimumPriority == 0 {
			entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, s.Cfgp, entry.QualityCfg)
		}
		entry.ParseInfo.DbserieID = s.SerieEpisode.DbserieID
		entry.ParseInfo.DbserieEpisodeID = s.SerieEpisode.DbserieEpisodeID
		entry.ParseInfo.SerieEpisodeID = s.SerieEpisode.ID
		entry.ParseInfo.SerieID = s.SerieEpisode.SerieID
		entry.WantedTitle = s.title
		entry.WantedAlternates = s.AlternateTitles
	}

	//Check Quality Profile
	if !config.Check("quality_" + entry.QualityTemplate) {
		denynzb("Unknown Quality Profile", entry, dl)
		return true
	}

	return false
}
func (s *Searcher) checkyear(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	if !strings.EqualFold(s.SearchGroupType, "movie") || s.SearchActionType == "rss" {
		return false
	}
	if s.year == 0 {
		denynzb("No Year", entry, dl)
		return true
	}
	stryear := logger.IntToString(s.year)
	checkyear := entry.QualityCfg.CheckYear
	checkyear1 := entry.QualityCfg.CheckYear1

	if (checkyear || checkyear1) && strings.Contains(entry.NZB.Title, stryear) {
		return false
	}
	if checkyear1 && strings.Contains(entry.NZB.Title, logger.IntToString(s.year+1)) {
		return false
	}
	if checkyear1 && strings.Contains(entry.NZB.Title, logger.IntToString(s.year-1)) {
		return false
	}
	denynzb("Unwanted Year", entry, dl)
	return true
}
func Checktitle(entry *apiexternal.Nzbwithprio, searchGroupType string, dl *SearchResults) bool {
	//Checktitle
	if !entry.QualityCfg.CheckTitle {
		return false
	}
	yearstr := logger.IntToString(entry.ParseInfo.Year)
	lentitle := len(entry.WantedTitle)
	title := importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityCfg)
	//title := entry.ParseInfo.Title
	if entry.WantedTitle != "" {
		if entry.QualityCfg.CheckTitle && lentitle >= 1 && apiexternal.Checknzbtitle(entry.WantedTitle, title) {
			return false
		}
		if entry.ParseInfo.Year != 0 && entry.QualityCfg.CheckTitle && lentitle >= 1 && apiexternal.Checknzbtitle(entry.WantedTitle, title+" "+yearstr) {
			return false
		}
	}
	if strings.EqualFold(searchGroupType, "movie") && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}),
			&database.Querywithargs{QueryString: "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}, &entry.WantedAlternates)
	}
	if strings.EqualFold(searchGroupType, "series") && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}),
			&database.Querywithargs{QueryString: "select distinct title from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}, &entry.WantedAlternates)
	}
	lenalt := len(entry.WantedAlternates)
	if lenalt == 0 || entry.ParseInfo.Title == "" {
		denynzb("Unwanted Title", entry, dl)
		return true
	}
	for idxtitle := range entry.WantedAlternates {
		if entry.WantedAlternates[idxtitle] == "" {
			continue
		}
		if apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], title) {
			return false
		}

		if entry.ParseInfo.Year != 0 && entry.QualityCfg.CheckTitle && apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], title+" "+yearstr) {
			return false
		}
	}
	denynzb("Unwanted Title and Alternate", entry, dl)
	return true
}
func (s *Searcher) checkepisode(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {

	//Checkepisode
	if strings.EqualFold(s.SearchGroupType, "movie") {
		return false
	}

	// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03

	lowerTitle := strings.ToLower(entry.NZB.Title)
	altIdentifier := logger.StringReplaceRune(strings.TrimLeft(s.loweridentifier, "s0"), 'e', 'x')
	if strings.Contains(lowerTitle, s.loweridentifier) ||
		strings.Contains(lowerTitle, altIdentifier) {
		return false
	}
	if logger.StringContainsRune(s.loweridentifier, '-') {
		if strings.Contains(lowerTitle, logger.StringReplaceRune(s.loweridentifier, '-', '.')) ||
			strings.Contains(lowerTitle, logger.StringReplaceRune(s.loweridentifier, '-', ' ')) ||
			strings.Contains(lowerTitle, logger.StringReplaceRune(altIdentifier, '-', '.')) ||
			strings.Contains(lowerTitle, logger.StringReplaceRune(altIdentifier, '-', ' ')) {
			return false
		}
	}

	var matchfound bool
	//lowerParseIdentifier := s.loweridentifier
	//if entry.ParseInfo.Identifier != s.identifier {
	//	lowerParseIdentifier = strings.ToLower(entry.ParseInfo.Identifier)
	//}
	seasonarray := []string{"s" + s.season + "e", "s0" + s.season + "e", "s" + s.season + " e", "s0" + s.season + " e", s.season + "x", s.season + " x"}
	episodearray := []string{"e" + s.episode, "e0" + s.episode, "x" + s.episode, "x0" + s.episode}
	for idxseason := range seasonarray {
		if !strings.HasPrefix(s.loweridentifier, seasonarray[idxseason]) {
			continue
		}
		for idxepisode := range episodearray {
			if strings.HasSuffix(s.loweridentifier, episodearray[idxepisode]) {
				matchfound = true
			} else if strings.Contains(s.loweridentifier, episodearray[idxepisode]+" ") {
				matchfound = true
			} else if strings.Contains(s.loweridentifier, episodearray[idxepisode]+"-") {
				matchfound = true
			} else if strings.Contains(s.loweridentifier, episodearray[idxepisode]+"e") {
				matchfound = true
			} else if strings.Contains(s.loweridentifier, episodearray[idxepisode]+"x") {
				matchfound = true
			}
			if matchfound {
				break
			}
		}
		break
	}
	seasonarray = nil
	episodearray = nil
	if !matchfound {
		denynzb("identifier not match ", entry, dl, s.identifier)
		return true
	}
	return false
}

func filterTestQualityWanted(qualitystr string, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	var wanted bool
	lenqual := len(entry.QualityCfg.WantedResolutionIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Resolution != "" {
		if slices.ContainsFunc(entry.QualityCfg.WantedResolutionIn.Arr, func(c string) bool {
			return strings.EqualFold(c, entry.ParseInfo.Resolution)
		}) {
			wanted = true
		}
	}

	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted resolution", entry, dl, entry.ParseInfo.Resolution)
		return false
	}
	wanted = false

	lenqual = len(entry.QualityCfg.WantedQualityIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Quality != "" {
		if slices.ContainsFunc(entry.QualityCfg.WantedQualityIn.Arr, func(c string) bool {
			return strings.EqualFold(c, entry.ParseInfo.Quality)
		}) {
			wanted = true
		}
	}
	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted quality", entry, dl, entry.ParseInfo.Quality)

		return false
	}

	wanted = false

	lenqual = len(entry.QualityCfg.WantedAudioIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Audio != "" {
		if slices.ContainsFunc(entry.QualityCfg.WantedAudioIn.Arr, func(c string) bool {
			return strings.EqualFold(c, entry.ParseInfo.Audio)
		}) {
			wanted = true
		}
	}
	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted audio", entry, dl, entry.ParseInfo.Audio)

		return false
	}
	wanted = false

	lenqual = len(entry.QualityCfg.WantedCodecIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Codec != "" {
		if slices.ContainsFunc(entry.QualityCfg.WantedCodecIn.Arr, func(c string) bool {
			return strings.EqualFold(c, entry.ParseInfo.Codec)
		}) {
			wanted = true
		}
	}
	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted codec", entry, dl, entry.ParseInfo.Codec)

		return false
	}
	return true
}

func (s *Searcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlist string, addifnotfound bool, cfgp *config.MediaTypeConfig, dl *SearchResults) bool {
	//Parse
	parser.GetDbIDs("movie", &entry.ParseInfo, cfgp, addinlist, true)
	loopdbmovie := entry.ParseInfo.DbmovieID
	loopmovie := entry.ParseInfo.MovieID
	//Get DbMovie by imdbid

	//Add DbMovie if not found yet and enabled
	if loopdbmovie == 0 && (!addifnotfound || !strings.HasPrefix(entry.NZB.IMDBID, "tt")) {
		denynzb("Not Wanted DBMovie", entry, dl)
		return true
	}
	if loopdbmovie == 0 && addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") {
		if !s.allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
			denynzb("Not Allowed Movie", entry, dl)
			return true
		}
		loopdbmovie = importfeed.JobImportMovies(entry.NZB.IMDBID, cfgp, addinlist, true)
		if database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylist, Args: []interface{}{loopdbmovie, addinlist}}, &loopmovie) != nil || loopdbmovie == 0 {
			denynzb(notwantedmovie, entry, dl)
			return true
		}
	}
	if loopdbmovie == 0 {
		denynzb("Not Wanted DBMovie", entry, dl)
		return true
	}

	//continue only if dbmovie found
	//Get List of movie by dbmovieid, year and possible lists

	//if list was not found : should we add the movie?
	if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") && loopmovie == 0 {
		if !s.allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
			denynzb("Not Allowed Movie", entry, dl)
			return true
		}
		loopdbmovie = importfeed.JobImportMovies(entry.NZB.IMDBID, cfgp, addinlist, true)
		if loopdbmovie != 0 {
			database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylist, Args: []interface{}{loopdbmovie, addinlist}}, &loopmovie)
		}
		if loopdbmovie == 0 || loopmovie == 0 {
			denynzb(notwantedmovie, entry, dl)
			return true
		}
	} else {
		denynzb(notwantedmovie, entry, dl)
		return true
	}

	if loopmovie == 0 {
		denynzb(notwantedmovie, entry, dl)
		return true
	}

	database.GetMovies(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{loopmovie}}, &s.Movie)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select year from dbmovies where id = ?", Args: []interface{}{loopmovie}}, &s.year)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select imdb_id from dbmovies where id = ?", Args: []interface{}{loopmovie}}, &s.imdb)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select title from dbmovies where id = ?", Args: []interface{}{loopmovie}}, &s.title)

	entry.Dbid = s.Movie.DbmovieID
	entry.NzbmovieID = s.Movie.ID
	entry.QualityTemplate = s.Movie.QualityProfile
	if s.Quality != entry.QualityTemplate {
		s.Quality = entry.QualityTemplate
		*entry.QualityCfg = config.Cfg.Quality[entry.QualityTemplate]
	}
	if entry.QualityCfg == nil {
		*entry.QualityCfg = config.Cfg.Quality[entry.QualityTemplate]
	}
	entry.ParseInfo.Title = importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityCfg)
	return false
}

func denynzb(reason string, entry *apiexternal.Nzbwithprio, dl *SearchResults, optional ...interface{}) {
	if dl == nil {
		return
	}
	if len(optional) >= 1 {
		entry.AdditionalReason = optional[0]
		logger.Log.GlobalLogger.Debug(skippedstr, zap.Stringp("reason", &reason), zap.Stringp("title", &entry.NZB.Title), zap.Any("additional", optional))
	} else {
		logger.Log.GlobalLogger.Debug(skippedstr, zap.Stringp("reason", &reason), zap.Stringp("title", &entry.NZB.Title))
	}
	entry.Denied = true
	entry.Reason = reason
	dl.mu.Lock()
	dl.Rejected = append(dl.Rejected, *entry)
	dl.mu.Unlock()
}

func (s *Searcher) getserierss(entry *apiexternal.Nzbwithprio, cfgp *config.MediaTypeConfig, dl *SearchResults) bool {
	parser.GetDbIDs("series", &entry.ParseInfo, cfgp, "", true)

	if entry.ParseInfo.SerieID == 0 {
		denynzb("Unwanted Serie", entry, dl)
		return true
	}
	if entry.ParseInfo.DbserieID == 0 {
		denynzb("Unwanted DBSerie", entry, dl)
		return true
	}
	if entry.ParseInfo.DbserieEpisodeID == 0 {
		denynzb("Unwanted DB Episode", entry, dl)
		return true
	}
	if entry.ParseInfo.SerieEpisodeID == 0 {
		denynzb("Unwanted Episode", entry, dl)
		return true
	}
	entry.NzbepisodeID = entry.ParseInfo.SerieEpisodeID
	entry.Dbid = entry.ParseInfo.DbserieID
	if entry.NzbepisodeID != 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "Select thetvdb_id from dbseries where id = ?", Args: []interface{}{entry.ParseInfo.DbserieID}}, &s.thetvdbid)
		database.QueryColumn(&database.Querywithargs{QueryString: "Select season from dbserie_episodes where id = ?", Args: []interface{}{entry.ParseInfo.DbserieEpisodeID}}, &s.season)
		database.QueryColumn(&database.Querywithargs{QueryString: "Select episode from dbserie_episodes where id = ?", Args: []interface{}{entry.ParseInfo.DbserieEpisodeID}}, &s.episode)
		database.QueryColumn(&database.Querywithargs{QueryString: "Select identifier from dbserie_episodes where id = ?", Args: []interface{}{entry.ParseInfo.DbserieEpisodeID}}, &s.identifier)
		database.QueryColumn(&database.Querywithargs{QueryString: "Select seriename from dbseries where id = ?", Args: []interface{}{entry.ParseInfo.DbserieID}}, &s.title)
		s.loweridentifier = strings.ToLower(s.identifier)
		//if entry.ParseInfo.DbserieID != 0 {
		//	database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{entry.ParseInfo.DbserieID}}, &entry.WantedAlternates)
		//}
	}
	return false
}

func (s *Searcher) GetRSSFeed(searchGroupType string, cfgp *config.MediaTypeConfig, listname string) (*SearchResults, error) {
	defer s.Close()
	if listname == "" {
		return nil, errNoList
	}
	templatelist := cfgp.ListsMap[listname].TemplateList
	if templatelist == "" {
		return nil, errNoList
	}
	if !config.Check("list_" + templatelist) {
		return nil, errNoList
	}
	if !config.Check("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	intid := slices.IndexFunc(config.Cfg.Quality[s.Quality].Indexer, func(c config.QualityIndexerConfig) bool {
		return c.TemplateIndexer == templatelist
	})
	if intid != -1 && config.Cfg.Quality[s.Quality].Indexer[intid].TemplateRegex == "" {
		return nil, errNoRegex
	}

	var lastindexerid string
	database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{templatelist, s.Quality, ""}}, &lastindexerid)

	blockinterval := -5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.Cfg.General.FailedIndexerBlockTime
	}

	s.Listcfg = config.Cfg.Lists[templatelist]

	var counter int
	database.QueryColumn(&database.Querywithargs{QueryString: "select count() from indexer_fails where indexer = ? and last_fail > ?", Args: []interface{}{s.Listcfg.URL, logger.TimeGetNow().Add(time.Minute * time.Duration(blockinterval))}}, &counter)
	if counter >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fail in the last ", zap.Int("Minutes", blockinterval), zap.String("Listname", templatelist))
		return nil, errIndexerDisabled
	}
	index := &apiexternal.NzbIndexer{Name: templatelist, Customrssurl: s.Listcfg.URL, LastRssID: lastindexerid}
	nzbs, _, lastids, nzberr := apiexternal.QueryNewznabRSSLast(index, s.Listcfg.Limit, "", 1)
	index = nil
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed", zap.Error(nzberr))
		// indexer.Close()
		return nil, nzberr
	} else {
		if lastids != "" && len((nzbs.Arr)) >= 1 {
			addrsshistory(s.Listcfg.URL, lastids, s.Quality, templatelist)
		}
		if nzbs != nil && len((nzbs.Arr)) >= 1 {
			dl := SearchResults{mu: &sync.Mutex{}}
			s.parseentries(nzbs, &dl, s.Quality, templatelist, listname, cfgp.ListsMap[listname].Addfound)
			if len(dl.Nzbs) > 1 {
				sort.Slice(dl.Nzbs, func(i, j int) bool {
					return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
				})
			}
			nzbs.Close()
			// indexer.Close()
			return &dl, nil
		}
		return nil, errOther
	}
}

func addrsshistory(url string, lastid string, quality string, config string) {
	var id int
	database.QueryColumn(&database.Querywithargs{QueryString: "select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{config, quality, url}}, &id)
	if id >= 1 {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "update r_sshistories set last_id = ? where id = ?", Args: []interface{}{lastid, id}})
	} else {
		database.InsertStatic(&database.Querywithargs{QueryString: "insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?)", Args: []interface{}{config, quality, url, lastid}})
	}
}

func MovieSearch(cfgp *config.MediaTypeConfig, movieid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	movie := new(database.Movie)
	if database.GetMovies(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{movieid}}, movie) != nil {
		return nil, errors.New("skipped - movie not found")
	}
	defer movie.Close()
	if movie.DbmovieID == 0 {
		return nil, errors.New("skipped - dbmovie not found")
	}

	if movie.DontSearch && !forceDownload {
		return nil, errSearchDisabled
	}

	if !config.Check("quality_" + movie.QualityProfile) {
		logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for Movie: %d%s", movieid, spacenotfound))
		return nil, errNoQuality
	}
	var year int
	database.QueryColumn(&database.Querywithargs{QueryString: "Select year from dbmovies where id = ?", Args: []interface{}{movie.DbmovieID}}, &year)

	if year == 0 {
		//logger.Log.GlobalLogger.Debug("Skipped - No Year")
		return nil, errors.New("year not found")
	}

	var imdb, title string
	database.QueryColumn(&database.Querywithargs{QueryString: "Select imdb_id from dbmovies where id = ?", Args: []interface{}{movie.DbmovieID}}, &imdb)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select title from dbmovies where id = ?", Args: []interface{}{movie.DbmovieID}}, &title)
	cfgqual := config.Cfg.Quality[movie.QualityProfile]
	minimumPriority := GetHighestMoviePriorityByFiles(false, true, movie.ID, cfgp, &cfgqual)
	var alternateTitles []string
	database.QueryStaticStringArray(false,
		database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{movie.DbmovieID}}),
		&database.Querywithargs{QueryString: "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{movie.DbmovieID}}, &alternateTitles)

	searchActionType := "upgrade"
	if minimumPriority == 0 {
		searchActionType = "missing"
	} else {
		if movie.DontUpgrade && !forceDownload {
			cfgqual.Close()
			return nil, errUpgradeDisabled
		}
	}

	var processedindexer int

	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	logger.Log.GlobalLogger.Debug("Search for movie id", zap.Uint("id", movie.ID))
	for idx := range config.Cfg.Quality[movie.QualityProfile].Indexer {
		indexertemplate := config.Cfg.Quality[movie.QualityProfile].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Enabled {
			//processedindexer++
			continue
		}
		workergroup.Submit(func() {
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          movie.QualityProfile,
				QualityCfg:       &cfgqual,
				SearchGroupType:  "movie",
				SearchActionType: searchActionType,
				MinimumPriority:  minimumPriority,
				Movie:            *movie,
				imdb:             imdb,
				year:             year,
				title:            title,
				AlternateTitles:  alternateTitles,
			}).mediasearchindexer(indexertemplate, titlesearch, 0, 0, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	cfgqual.Close()

	if processedindexer == 0 && len(config.Cfg.Quality[movie.QualityProfile].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer == 0 {
		return nil, errors.New("no indexer searched")
	}
	logger.Log.GlobalLogger.Debug("Ended Search for movie id", zap.Uint("id", movie.ID), zap.Int("Accepted", len(dl.Nzbs)), zap.Int("Denied", len(dl.Rejected)))
	if processedindexer >= 1 {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set lastscan = ? where id = ?", Args: []interface{}{logger.SqlTimeGetNow(), movieid}})
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) mediasearchindexer(indexer string, titlesearch bool, season int, episode int, dl *SearchResults) bool {
	defer s.Close()
	_, cats, err := s.initIndexerURLCat(s.Quality, indexer, "api")
	if err != nil {
		if err == errIndexerDisabled || err == errNoIndexer {
			return true
		}
		if err == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &indexer), zap.Error(err))

		return false
	}
	qualityprofile := s.SerieEpisode.QualityProfile
	if s.SearchGroupType == "movie" {
		qualityprofile = s.Movie.QualityProfile
	}
	//logger.Log.Debug("Qualty: ", qualityprofile)

	if !config.Check("quality_" + qualityprofile) {
		//logger.Log.GlobalLogger.Error("Quality for: " + search.searchid + spacenotfound)
		return false
	}

	//inittitle := search.titlesearch
	//func (s *Searcher) searchMedia(mediatype string, searchid string, searchtitle bool, id uint, quality string, indexer string, title string, season int, episode int, cats string, dl *SearchResults) bool {
	if !titlesearch && ((s.SearchGroupType == "movie" && s.imdb != "") || (s.SearchGroupType == "series" && s.thetvdbid != 0)) {
		s.searchMedia(s.Quality, indexer, s.SearchGroupType, cats, titlesearch, season, episode, "", dl)
		if len(dl.Nzbs) >= 1 {
			if database.DBLogLevel == "debug" {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Title", &s.title), zap.Stringp("Indexer", &indexer), zap.Int("entries", len(dl.Nzbs)))
			}

			if s.QualityCfg.CheckUntilFirstFound {
				return true
			}
		}
	}
	if s.QualityCfg.SearchForTitleIfEmpty && len(dl.Nzbs) == 0 {
		titlesearch = true
	}
	if !titlesearch {
		return true
	}
	var addstr, searchfor string
	if s.SearchGroupType == "movie" && s.year != 0 {
		addstr = " (" + logger.IntToString(s.year) + ")"
	} else if s.SearchGroupType == "series" && s.identifier != "" {
		addstr = " " + s.identifier
	}
	searched := new(logger.InStringArrayStruct)
	defer searched.Close()
	if s.QualityCfg.BackupSearchForTitle {
		searchfor = logger.StringDeleteRuneAll(s.title, '(')
		searchfor = logger.StringDeleteRuneAll(searchfor, '&')
		searchfor = logger.StringDeleteRuneAll(searchfor, ')') + addstr
		searched.Arr = append(searched.Arr, searchfor)
		s.searchMedia(s.Quality, indexer, s.SearchGroupType, cats, titlesearch, season, episode, searchfor, dl)

		if len(dl.Nzbs) >= 1 {
			if database.DBLogLevel == "debug" {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Searchfor", &s.title), zap.Stringp("Indexer", &indexer), zap.Int("entries", len(dl.Nzbs)))
			}

			if s.QualityCfg.CheckUntilFirstFound {
				return true
			}
		}
	}
	if s.QualityCfg.SearchForAlternateTitleIfEmpty && len(dl.Nzbs) == 0 {
		titlesearch = true
	}
	if !s.QualityCfg.BackupSearchForAlternateTitle {
		return true
	}
	for idxalt := range s.AlternateTitles {
		if s.AlternateTitles[idxalt] == "" {
			continue
		}
		searchfor = logger.StringDeleteRuneAll(s.AlternateTitles[idxalt], '(')
		searchfor = logger.StringDeleteRuneAll(searchfor, '&')
		searchfor = logger.StringDeleteRuneAll(searchfor, ')') + addstr
		if slices.ContainsFunc(searched.Arr, func(c string) bool { return strings.EqualFold(c, searchfor) }) {
			//if logger.InStringArray(searchfor, searched) {
			continue
		}
		searched.Arr = append(searched.Arr, searchfor)
		s.searchMedia(s.Quality, indexer, s.SearchGroupType, cats, titlesearch, season, episode, searchfor, dl)

		if len(dl.Nzbs) == 0 {
			continue
		}
		if database.DBLogLevel == "debug" {
			logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Searchfor", &searchfor), zap.Stringp("Indexer", &indexer), zap.Int("entries", len(dl.Nzbs)))
		}

		if s.QualityCfg.CheckUntilFirstFound {
			break
		}
	}
	return true
}

func SeriesSearch(cfgp *config.MediaTypeConfig, episodeid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	serieEpisode := new(database.SerieEpisode)
	if database.GetSerieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodeid}}, serieEpisode) != nil {
		return nil, errors.New("skipped - episode not found")
	}
	defer serieEpisode.Close()
	if serieEpisode.DbserieEpisodeID == 0 || serieEpisode.DbserieID == 0 || serieEpisode.SerieID == 0 {
		return nil, errors.New("skipped - dbepisode not found")
	}
	if serieEpisode.DontSearch && !forceDownload {
		return nil, errSearchDisabled
	}

	if !config.Check("quality_" + serieEpisode.QualityProfile) {
		logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for Episode: %d%s", episodeid, spacenotfound))
		return nil, errNoQuality
	}
	var thetvdbid int
	database.QueryColumn(&database.Querywithargs{QueryString: "Select thetvdb_id from dbseries where id = ?", Args: []interface{}{serieEpisode.DbserieID}}, &thetvdbid)
	var season, episode, identifier, title string
	database.QueryColumn(&database.Querywithargs{QueryString: "Select season from dbserie_episodes where id = ?", Args: []interface{}{serieEpisode.DbserieEpisodeID}}, &season)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select episode from dbserie_episodes where id = ?", Args: []interface{}{serieEpisode.DbserieEpisodeID}}, &episode)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select identifier from dbserie_episodes where id = ?", Args: []interface{}{serieEpisode.DbserieEpisodeID}}, &identifier)
	database.QueryColumn(&database.Querywithargs{QueryString: "Select seriename from dbseries where id = ?", Args: []interface{}{serieEpisode.DbserieID}}, &title)
	cfgqual := config.Cfg.Quality[serieEpisode.QualityProfile]
	minimumPriority := GetHighestEpisodePriorityByFiles(false, true, serieEpisode.ID, cfgp, &cfgqual)
	var alternateTitles []string
	database.QueryStaticStringArray(false,
		database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{serieEpisode.DbserieID}}),
		&database.Querywithargs{QueryString: "select distinct title from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{serieEpisode.DbserieID}},
		&alternateTitles)

	searchActionType := "upgrade"
	if minimumPriority == 0 {
		searchActionType = "missing"
	} else {
		if serieEpisode.DontUpgrade && !forceDownload {
			cfgqual.Close()
			return nil, errUpgradeDisabled
		}
	}

	var processedindexer int
	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	var seasonset, episodeset int
	if season != "" {
		seasonset = logger.StringToInt(season)
	}
	if episode != "" {
		episodeset = logger.StringToInt(episode)
	}
	logger.Log.GlobalLogger.Debug("Search for serie id", zap.Uint("id", serieEpisode.ID))
	for idx := range config.Cfg.Quality[serieEpisode.QualityProfile].Indexer {
		indexertemplate := config.Cfg.Quality[serieEpisode.QualityProfile].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Enabled {
			//processedindexer++
			continue
		}
		workergroup.Submit(func() {
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          serieEpisode.QualityProfile,
				QualityCfg:       &cfgqual,
				SearchGroupType:  "series",
				SearchActionType: searchActionType,
				MinimumPriority:  minimumPriority,
				SerieEpisode:     *serieEpisode,
				title:            title,
				thetvdbid:        thetvdbid,
				season:           season,
				episode:          episode,
				identifier:       identifier,
				loweridentifier:  strings.ToLower(identifier),
				AlternateTitles:  alternateTitles,
			}).mediasearchindexer(indexertemplate, titlesearch, seasonset, episodeset, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	cfgqual.Close()

	//dl := s.queryindexers(s.SerieEpisode.QualityProfile, "series", false, &processedindexer, titlesearch, queryparams{})

	if processedindexer == 0 && len(config.Cfg.Quality[serieEpisode.QualityProfile].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer == 0 {
		return nil, errors.New("no indexer searched")
	}
	logger.Log.GlobalLogger.Debug("Ended Search for serie id", zap.Uint("id", serieEpisode.ID), zap.Int("Accepted", len(dl.Nzbs)), zap.Int("Denied", len(dl.Rejected)))

	if processedindexer >= 1 {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set lastscan = ? where id = ?", Args: []interface{}{logger.SqlTimeGetNow(), episodeid}})
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) initIndexer(quality string, indexer string, rssapi string, nzbIndexer *apiexternal.NzbIndexer) (string, int, int, error) {
	if !config.Check("indexer_" + indexer) {
		return "", 0, 0, errNoIndexer
	}
	cfgind := config.Cfg.Indexers[indexer]
	defer cfgind.Close()
	if !strings.EqualFold(cfgind.IndexerType, "newznab") {
		// idxcfg.Close()
		return "", 0, 0, errors.New("indexer Type Wrong")
	}
	if !cfgind.Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return "", 0, 0, errIndexerDisabled
	} else if !cfgind.Enabled {
		// idxcfg.Close()
		return "", 0, 0, errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(cfgind.URL); !ok {
		// idxcfg.Close()
		return "", 0, 0, errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, cfgind.URL}}, &lastindexerid)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    cfgind.URL,
		Apikey:                 cfgind.Apikey,
		UserID:                 cfgind.Userid,
		SkipSslCheck:           cfgind.DisableTLSVerify,
		DisableCompression:     cfgind.DisableCompression,
		Addquotesfortitlequery: cfgind.Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "AdditionalQueryParams"),
		LastRssID:              lastindexerid,
		Customapi:              cfgind.Customapi,
		Customurl:              cfgind.Customurl,
		Customrssurl:           cfgind.Customrssurl,
		Customrsscategory:      cfgind.Customrsscategory,
		Limitercalls:           cfgind.Limitercalls,
		Limiterseconds:         cfgind.Limiterseconds,
		LimitercallsDaily:      cfgind.LimitercallsDaily,
		TimeoutSeconds:         cfgind.TimeoutSeconds,
		MaxAge:                 cfgind.MaxAge,
		OutputAsJSON:           cfgind.OutputAsJSON}

	//cfgind.MaxRssEntries
	//cfgind.RssEntriesloop
	// idxcfg.Close()
	return config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "CategoriesIndexer"), cfgind.RssEntriesloop, cfgind.MaxRssEntries, nil
}

func (s *Searcher) initIndexerURLCat(quality string, indexer string, rssapi string) (string, string, error) {
	if !config.Check("indexer_" + indexer) {
		return "", "", errNoIndexer
	}

	cfgind := config.Cfg.Indexers[indexer]
	defer cfgind.Close()
	if !strings.EqualFold(cfgind.IndexerType, "newznab") {
		// idxcfg.Close()
		return "", "", errors.New("indexer Type Wrong")
	}
	if !cfgind.Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return "", "", errIndexerDisabled
	} else if !cfgind.Enabled {
		// idxcfg.Close()
		return "", "", errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(cfgind.URL); !ok {
		// idxcfg.Close()
		return "", "", errToWait
	}
	// defer idxcfg.Close()
	return cfgind.URL, config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "CategoriesIndexer"), nil
}

func (s *Searcher) initNzbIndexer(quality string, indexer string, rssapi string, nzbIndexer *apiexternal.NzbIndexer) error {
	if !config.Check("indexer_" + indexer) {
		return errNoIndexer
	}
	cfgind := config.Cfg.Indexers[indexer]
	defer cfgind.Close()
	if !strings.EqualFold(cfgind.IndexerType, "newznab") {
		// idxcfg.Close()
		return errors.New("indexer type wrong")
	}
	if !cfgind.Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return errIndexerDisabled
	} else if !cfgind.Enabled {
		// idxcfg.Close()
		return errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(cfgind.URL); !ok {
		// idxcfg.Close()
		return errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, cfgind.URL}}, &lastindexerid)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    cfgind.URL,
		Apikey:                 cfgind.Apikey,
		UserID:                 cfgind.Userid,
		SkipSslCheck:           cfgind.DisableTLSVerify,
		DisableCompression:     cfgind.DisableCompression,
		Addquotesfortitlequery: cfgind.Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "AdditionalQueryParams"),
		LastRssID:              lastindexerid,
		Customapi:              cfgind.Customapi,
		Customurl:              cfgind.Customurl,
		Customrssurl:           cfgind.Customrssurl,
		Customrsscategory:      cfgind.Customrsscategory,
		Limitercalls:           cfgind.Limitercalls,
		Limiterseconds:         cfgind.Limiterseconds,
		LimitercallsDaily:      cfgind.LimitercallsDaily,
		TimeoutSeconds:         cfgind.TimeoutSeconds,
		MaxAge:                 cfgind.MaxAge,
		OutputAsJSON:           cfgind.OutputAsJSON}

	// idxcfg.Close()
	return nil
}

func (s *Searcher) getnzbs(mediatype string, cats string, titlesearch bool, season int, episode int, nzbindexer *apiexternal.NzbIndexer, title string) (*apiexternal.NZBArr, bool, error) {
	if !titlesearch {
		if mediatype == "movie" && s.imdb != "" {
			return apiexternal.QueryNewznabMovieImdb(nzbindexer, strings.Trim(s.imdb, "t"), cats)
		} else if mediatype == "series" && s.thetvdbid != 0 {
			return apiexternal.QueryNewznabTvTvdb(nzbindexer, s.thetvdbid, cats, season, episode, true, true)
		}
	} else {
		return apiexternal.QueryNewznabQuery(nzbindexer, title, cats, "search")
	}
	return nil, false, errors.New("not matched")
}
func (s *Searcher) searchMedia(quality string, indexer string, mediatype string, cats string, titlesearch bool, season int, episode int, title string, dl *SearchResults) {
	if !config.Check("quality_" + quality) {
		return
	}
	nzbindexer := new(apiexternal.NzbIndexer)
	erri := s.initNzbIndexer(quality, indexer, "api", nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			logger.Log.GlobalLogger.Debug("No Indexer", zap.Stringp("indexer", &indexer), zap.Error(erri))
			// nzbindexer.Close()
			return
		}
		if erri == errToWait {
			logger.Log.GlobalLogger.Debug("Indexer needs waiting", zap.Stringp("indexer", &indexer), zap.Error(erri))
			time.Sleep(10 * time.Second)
			// nzbindexer.Close()
			return
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &indexer), zap.Error(erri))
		// nzbindexer.Close()
		return
	}

	nzbs, _, nzberr := s.getnzbs(mediatype, cats, titlesearch, season, episode, nzbindexer, title)

	if nzberr != nil && nzberr != apiexternal.Errnoresults {
		logger.Log.GlobalLogger.Error("Newznab Search failed", zap.Stringp("indexer", &nzbindexer.URL), zap.Error(nzberr))
	}

	if nzberr == nil && nzbs != nil && len((nzbs.Arr)) >= 1 {
		s.parseentries(nzbs, dl, quality, indexer, "", false)

		if database.DBLogLevel == "debug" {
			logger.Log.GlobalLogger.Debug("Entries found", zap.Stringp("indexer", &indexer), zap.Int("Count", len(nzbs.Arr)))
		}
	}
	// nzbindexer.Close()
	nzbs.Close()
}

func filterSizeNzbs(cfgp *config.MediaTypeConfig, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	var templatepath string
	for idx := range cfgp.DataImport {
		templatepath = cfgp.DataImport[idx].TemplatePath
		if !config.Check("path_" + templatepath) {
			return false
		}
		//cfgpath := config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath]
		if config.Cfg.GetPath(templatepath).MinSize != 0 && entry.NZB.Size < config.Cfg.GetPath(templatepath).MinSizeByte {
			denynzb("Too Small", entry, dl)
			return true
		}

		if config.Cfg.GetPath(templatepath).MaxSize != 0 && entry.NZB.Size > config.Cfg.GetPath(templatepath).MaxSizeByte {
			//logger.Log.GlobalLogger.Debug("Skipped - MaxSize not matched", zap.Stringp("title", &entry.NZB.Title))
			denynzb("Too Big", entry, dl)
			return true
		}
	}
	return false
}

func filterRegexNzbs(s *apiexternal.Nzbwithprio, templateregex string, dl *SearchResults) bool {
	if templateregex == "" {
		denynzb(deniedbyregex, s, dl, "regex_template empty")
		return true
	}
	var breakfor, requiredmatched bool
	for _, reg := range config.Cfg.Regex[templateregex].Required {
		if config.RegexGetMatchesFind(reg, s.NZB.Title, 1) {
			requiredmatched = true
			break
		}
	}
	if len(config.Cfg.Regex[templateregex].Required) >= 1 && !requiredmatched {
		denynzb("required not matched", s, dl)
		return true
	}
	for _, reg := range config.Cfg.Regex[templateregex].Rejected {
		if config.RegexGetMatchesFind(reg, s.WantedTitle, 1) {
			//Regex is in title - skip test
			continue
		}
		breakfor = false
		for idxwanted := range s.WantedAlternates {
			if s.WantedAlternates[idxwanted] == s.WantedTitle {
				continue
			}
			if config.RegexGetMatchesFind(reg, s.WantedAlternates[idxwanted], 1) {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		if config.RegexGetMatchesFind(reg, s.NZB.Title, 1) {
			//logger.Log.GlobalLogger.Debug(regexrejected, zap.String("title", title), zap.String("regex", cfgregex.Rejected[idx]))
			denynzb(deniedbyregex, s, dl, reg)
			return true
		}
	}
	return false
}
func (s *Searcher) parseentries(nzbs *apiexternal.NZBArr, dl *SearchResults, quality string, indexer string, listname string, addfound bool) {
	if len((nzbs.Arr)) == 0 {
		return
	}
	if !config.Check("regex_" + config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "TemplateRegex")) {
		dl.mu.Lock()
		if len(dl.Rejected) == 0 && len(nzbs.Arr) >= 1 {
			dl.Rejected = make([]apiexternal.Nzbwithprio, 0, len(nzbs.Arr))
		} else if len(nzbs.Arr) > len(dl.Rejected) {
			//dl.Rejected = logger.GrowSliceBy(dl.Rejected, len(nzbs.Arr))
			dl.Rejected = slices.Grow(dl.Rejected, len(nzbs.Arr))
		}
		for idx := range nzbs.Arr {
			nzbs.Arr[idx].Denied = true
			nzbs.Arr[idx].Reason = "Denied by Regex"
			dl.Rejected = append(dl.Rejected, nzbs.Arr[idx])
		}
		dl.mu.Unlock()
		return
	}
	dl.mu.Lock()
	if len(dl.Rejected) == 0 && len(nzbs.Arr) >= 1 {
		dl.Rejected = make([]apiexternal.Nzbwithprio, 0, len(nzbs.Arr))
	} else if len(nzbs.Arr) > len(dl.Rejected) {
		dl.Rejected = slices.Grow(dl.Rejected, len(nzbs.Arr))
		//dl.Rejected = logger.GrowSliceBy(dl.Rejected, len(nzbs.Arr))
	}
	dl.mu.Unlock()
	var templateregex string
	var parsefile, includeyear, denied, skipemptysize bool

	historytable := "serie_episode_histories"
	if strings.EqualFold(s.SearchGroupType, "movie") {
		historytable = "movie_histories"
	}
	var historyurlcache *cache.Return
	if !logger.GlobalCache.CheckNoType(historytable + "_url") {
		get := logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from " + historytable}),
			&database.Querywithargs{QueryString: "select distinct url from " + historytable}, &get.Arr)
		logger.GlobalCache.Set(historytable+"_url", get, 8*time.Hour, false)
		historyurlcache = new(cache.Return)
		historyurlcache.Value = get
		get.Close()
	} else {
		historyurlcache = logger.GlobalCache.GetData(historytable + "_url")
	}

	var historytitlecache *cache.Return
	if !logger.GlobalCache.CheckNoType(historytable + "_title") {
		get := logger.InStringArrayStruct{}
		database.QueryStaticStringArray(false,
			database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from " + historytable}),
			&database.Querywithargs{QueryString: "select distinct title from " + historytable}, &get.Arr)
		logger.GlobalCache.Set(historytable+"_title", get, 8*time.Hour, false)
		historytitlecache = new(cache.Return)
		historytitlecache.Value = get
		get.Close()
	} else {
		historytitlecache = logger.GlobalCache.GetData(historytable + "_title")
	}

	for _, entry := range nzbs.Arr {
		entry.Indexer = indexer

		//Check Title Length
		if entry.NZB.DownloadURL == "" {
			denynzb("No Url", &entry, dl)
			continue
		}
		if entry.NZB.Title == "" {
			denynzb("No Title", &entry, dl)
			continue
		}
		if len(strings.Trim(entry.NZB.Title, " ")) <= 3 {
			denynzb("Title too short", &entry, dl)
			continue
		}
		denied = false
		dl.mu.Lock()
		if slices.ContainsFunc(dl.Rejected, func(c apiexternal.Nzbwithprio) bool { return c.NZB.DownloadURL == entry.NZB.DownloadURL }) {
			denied = true
		}
		dl.mu.Unlock()
		if denied {
			continue
		}
		dl.mu.Lock()
		if slices.ContainsFunc(dl.Nzbs, func(c apiexternal.Nzbwithprio) bool { return c.NZB.DownloadURL == entry.NZB.DownloadURL }) {
			denied = true
		}
		dl.mu.Unlock()
		if denied {
			denynzb("Already added", &entry, dl)
			continue
		}

		//Check Size
		templateregex, skipemptysize = config.QualityIndexerByQualityAndTemplateFirTemplateAndSize(quality, entry.Indexer)
		if templateregex == "" {
			denynzb("No Indexer Regex Template", &entry, dl)
			continue
		}
		if skipemptysize && entry.NZB.Size == 0 {
			denynzb("Missing size", &entry, dl)
			continue
		}

		if filterSizeNzbs(s.Cfgp, &entry, dl) {
			continue
		}

		if s.checkhistory(quality, indexer, historyurlcache, historytitlecache, &entry, dl) {
			continue
		}

		if s.checkcorrectid(&entry, dl) {
			continue
		}

		//Regex
		if filterRegexNzbs(&entry, templateregex, dl) {
			continue
		}

		//Parse
		parsefile = false
		if entry.ParseInfo.File == "" {
			parsefile = true
		} else if entry.ParseInfo.File != "" && (entry.ParseInfo.Title == "" || entry.ParseInfo.Resolution == "" || entry.ParseInfo.Quality == "") {
			parsefile = true
		}
		if parsefile {
			includeyear = false
			if s.SearchGroupType == "series" {
				includeyear = true
			}
			entry.ParseInfo = *parser.NewFileParser(entry.NZB.Title, includeyear, s.SearchGroupType)
			//entries.Arr[entryidx].ParseInfo, err = parser.NewFileParserNoPt(entries.Arr[entryidx].NZB.Title, includeyear, s.SearchGroupType)
		}

		if s.getmediadata(&entry, dl, listname, addfound) {
			continue
		}

		if entry.ParseInfo.Priority == 0 {
			parser.GetPriorityMapQual(&entry.ParseInfo, s.Cfgp, entry.QualityCfg, false, true)
			entry.Prio = entry.ParseInfo.Priority
		}

		entry.ParseInfo.Title = importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityCfg)

		//check quality
		if !filterTestQualityWanted(entry.QualityTemplate, &entry, dl) {
			continue
		}
		//check priority
		if entry.ParseInfo.Priority == 0 {
			denynzb("Prio unknown", &entry, dl)
			continue
		}

		if entry.MinimumPriority == entry.ParseInfo.Priority {
			denynzb("Prio same", &entry, dl, entry.MinimumPriority)
			continue
		}
		if entry.MinimumPriority != 0 && entry.QualityCfg.UseForPriorityMinDifference == 0 && entry.ParseInfo.Priority <= entry.MinimumPriority {
			denynzb("Prio lower", &entry, dl, entry.MinimumPriority)
			continue
		}
		if entry.MinimumPriority != 0 && entry.QualityCfg.UseForPriorityMinDifference != 0 && (entry.QualityCfg.UseForPriorityMinDifference+entry.ParseInfo.Priority) <= entry.MinimumPriority {
			denynzb("Prio lower", &entry, dl, entry.MinimumPriority)
			continue
		}

		if s.checkyear(&entry, dl) {
			continue
		}

		if Checktitle(&entry, s.SearchGroupType, dl) {
			continue
		}
		if s.checkepisode(&entry, dl) {
			continue
		}
		logger.Log.GlobalLogger.Debug("Release ok", zap.Intp("minimum prio", &entry.MinimumPriority), zap.Intp("prio", &entry.ParseInfo.Priority), zap.Stringp("quality", &entry.QualityTemplate), zap.Stringp("title", &entry.NZB.Title))

		dl.mu.Lock()
		dl.Nzbs = append(dl.Nzbs, entry)
		dl.mu.Unlock()
		// index.Close()
	}
	historytitlecache = nil
	historyurlcache = nil
}

func (s *Searcher) allowMovieImport(imdb string, cfgp *config.MediaTypeConfig, listname string) bool {
	if listname == "" {
		return false
	}
	templatelist := cfgp.ListsMap[listname].TemplateList
	if !config.Check("list_" + templatelist) {
		return false
	}
	var countergenre int

	if s.Listcfg.Name != templatelist {
		s.Listcfg = config.Cfg.Lists[templatelist]
	}

	if s.Listcfg.MinVotes != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and num_votes < ?", Args: []interface{}{imdb, s.Listcfg.MinVotes}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error vote count too low for ", zap.Stringp("imdb", &imdb))
			return false
		}
	}
	if s.Listcfg.MinRating != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and average_rating < ?", Args: []interface{}{imdb, s.Listcfg.MinRating}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error average vote too low for ", zap.Stringp("imdb", &imdb))
			return false
		}
	}
	var excludebygenre bool
	countimdb := "select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE"
	for idxgenre := range s.Listcfg.Excludegenre {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, s.Listcfg.Excludegenre[idxgenre]}})
		if countergenre >= 1 {
			excludebygenre = true
			logger.Log.GlobalLogger.Debug("error excluded genre ", zap.Stringp("excluded", &s.Listcfg.Excludegenre[idxgenre]), zap.Stringp("imdb", &imdb))
			break
		}
	}
	if excludebygenre {
		return false
	}
	var includebygenre bool
	for idxgenre := range s.Listcfg.Includegenre {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, s.Listcfg.Includegenre[idxgenre]}})
		if countergenre >= 1 {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(s.Listcfg.Includegenre) >= 1 {
		logger.Log.GlobalLogger.Debug("error included genre not found ", zap.Stringp("imdb", &imdb))
		return false
	}
	return true
}
func GetHighestMoviePriorityByFilesGetQual(useall bool, checkwanted bool, movieid uint, cfgp *config.MediaTypeConfig, qualitytemplate string) (minPrio int) {
	cfgqual := config.Cfg.Quality[qualitytemplate]
	defer cfgqual.Close()
	return GetHighestMoviePriorityByFiles(useall, checkwanted, movieid, cfgp, &cfgqual)
}
func GetHighestMoviePriorityByFiles(useall bool, checkwanted bool, movieid uint, cfgp *config.MediaTypeConfig, cfgqual *config.QualityConfig) (minPrio int) {
	return getPriorityByFiles(querymoviefiles, "movie_files", useall, checkwanted, movieid, cfgp, cfgqual)
}

func getPriorityByFiles(querystring string, table string, useall bool, checkwanted bool, id uint, cfgp *config.MediaTypeConfig, cfgqual *config.QualityConfig) (minPrio int) {
	var foundfiles []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: querystring, Args: []interface{}{id}}, &foundfiles)

	var prio int
	for idx := range foundfiles {
		prio = parser.Getdbidsfromfiles(useall, checkwanted, foundfiles[idx], table, cfgp, cfgqual)
		if prio == 0 && checkwanted {
			prio = parser.Getdbidsfromfiles(useall, false, foundfiles[idx], table, cfgp, cfgqual)
		}
		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
	}
	foundfiles = nil
	return minPrio
}

func GetHighestEpisodePriorityByFilesGetQual(useall bool, checkwanted bool, movieid uint, cfgp *config.MediaTypeConfig, qualitytemplate string) (minPrio int) {
	cfgqual := config.Cfg.Quality[qualitytemplate]
	defer cfgqual.Close()
	return GetHighestEpisodePriorityByFiles(useall, checkwanted, movieid, cfgp, &cfgqual)
}
func GetHighestEpisodePriorityByFiles(useall bool, checkwanted bool, episodeid uint, cfgp *config.MediaTypeConfig, cfgqual *config.QualityConfig) int {
	return getPriorityByFiles(queryseriefiles, "serie_episode_files", useall, checkwanted, episodeid, cfgp, cfgqual)
}

func GetSerieDBPriorityByID(useall bool, checkwanted bool, episodefileid uint, cfgp *config.MediaTypeConfig, cfgqual *config.QualityConfig) int {
	serieepisodefile := new(database.SerieEpisodeFile)
	if database.GetSerieEpisodeFiles(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodefileid}}, serieepisodefile) != nil {
		return 0
	}
	defer serieepisodefile.Close()
	return parser.GetIDPriorityMap(&apiexternal.ParseInfo{
		ResolutionID: serieepisodefile.ResolutionID,
		QualityID:    serieepisodefile.QualityID,
		CodecID:      serieepisodefile.CodecID,
		AudioID:      serieepisodefile.AudioID,
		Proper:       serieepisodefile.Proper,
		Extended:     serieepisodefile.Extended,
		Repack:       serieepisodefile.Repack,
		Title:        serieepisodefile.Filename,
		File:         serieepisodefile.Location,
	}, cfgp, cfgqual, useall, checkwanted)
}

func GetMovieDBPriorityByID(useall bool, checkwanted bool, moviefileid uint, cfgp *config.MediaTypeConfig, cfgqual *config.QualityConfig) int {
	moviefile := new(database.MovieFile)
	if database.GetMovieFiles(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{moviefileid}}, moviefile) != nil {
		return 0
	}
	defer moviefile.Close()
	return parser.GetIDPriorityMap(&apiexternal.ParseInfo{
		ResolutionID: moviefile.ResolutionID,
		QualityID:    moviefile.QualityID,
		CodecID:      moviefile.CodecID,
		AudioID:      moviefile.AudioID,
		Proper:       moviefile.Proper,
		Extended:     moviefile.Extended,
		Repack:       moviefile.Repack,
		Title:        moviefile.Filename,
		File:         moviefile.Location,
	}, cfgp, cfgqual, useall, checkwanted)
}
