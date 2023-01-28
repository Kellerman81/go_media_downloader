package searcher

import (
	"database/sql"
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
	searchid      string
	id            uint
	quality       string
	indexer       string
	title         string
	episode       int
	cats          string
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
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	MinimumPriority  int
	Movie            database.Movie
	SerieEpisode     database.SerieEpisode
	imdb             string
	year             int
	title            string
	identifier       string
	season           string
	episode          string
	thetvdbid        int
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
func SearchMovie(cfgp *config.MediaTypeConfig, missing bool, jobcount int, titlesearch bool) {

	var scaninterval int
	var scandatepre int

	if len(cfgp.Data) >= 1 {
		if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
			return
		}
		if missing {
			scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanInterval
			scandatepre = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanReleaseDatePre
		} else {
			scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].UpgradeScanInterval
		}
	}

	var q database.Querywithargs
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
		q.Query.Where = "movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	} else {
		q.Query.Where = "quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	}
	var i int
	if scaninterval != 0 {
		i++
		q.Query.Where += " and (movies.lastscan is null or movies.Lastscan < ?)"
		q.Args[len(cfgp.Lists)] = time.Now().AddDate(0, 0, 0-scaninterval)
	}
	if scandatepre != 0 {
		q.Query.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
		q.Args[len(cfgp.Lists)+i] = time.Now().AddDate(0, 0, 0+scandatepre)
	}
	if jobcount >= 1 {
		q.Query.Limit = jobcount
	}

	searchlist(cfgp, "movies", titlesearch, &q)
	q.Close()
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
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(episodes[idx], cfgp, titlesearch)
			//})
		}
	}
	episodes = nil
}

func SearchSerieSeasonSingle(serieid uint, season string, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var episodes []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: "select id from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", Args: []interface{}{serieid, season}}, &episodes)
	if len(episodes) >= 1 {
		for idx := range episodes {
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(episodes[idx], cfgp, titlesearch)
			//})
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
		queryseason := "select distinct season from dbserie_episodes where dbserie_id = ?"
		for idx := range series {
			seasons = []string{}
			database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: queryseason, Args: []interface{}{series[idx].Num2}}, &seasons)
			for idxseason := range seasons {
				if seasons[idxseason] == "" {
					continue
				}
				//workergroup.Submit(func() {
				SearchSerieRSSSeasonSingle(uint(series[idx].Num1), logger.StringToInt(seasons[idxseason]), true, &cfgp)
				//})
			}
		}
		seasons = nil
	}
	series = nil
	cfgp.Close()
}
func SearchSerieEpisodeSingle(episodeid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	var quality string
	database.QueryColumn(&database.Querywithargs{QueryString: queryqualprofseries, Args: []interface{}{episodeid}}, &quality)
	searchstuff(cfgp, quality, "series", &searchstruct{mediatype: "series", episodeid: episodeid, forceDownload: false, titlesearch: titlesearch})
}
func SearchSerie(cfgp *config.MediaTypeConfig, missing bool, jobcount int, titlesearch bool) {
	var scaninterval int
	var scandatepre int

	if len(cfgp.Data) >= 1 {
		if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
			return
		}
		if missing {
			scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanInterval
			scandatepre = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanReleaseDatePre
		} else {
			scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].UpgradeScanInterval
		}
	}
	var q database.Querywithargs
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
		q.Query.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
	} else {
		q.Query.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	}
	if scaninterval != 0 {
		q.Query.Where += " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
		q.Args[len(cfgp.Lists)] = time.Now().AddDate(0, 0, 0-scaninterval)
		if scandatepre != 0 {
			q.Query.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			q.Args[len(cfgp.Lists)+1] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		q.Query.Where += " and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
		if scandatepre != 0 {
			q.Query.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			q.Args[len(cfgp.Lists)] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		q.Query.Limit = jobcount
	}

	searchlist(cfgp, "serie_episodes", titlesearch, &q)
	q.Close()
}
func searchlist(cfgp *config.MediaTypeConfig, table string, titlesearch bool, qu *database.Querywithargs) {
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
	missingepisode = nil
}

func SearchSerieRSS(cfgp *config.MediaTypeConfig, quality string) {
	searchstuff(cfgp, quality, "rss", &searchstruct{mediatype: "series"})
}

func searchstuff(cfgp *config.MediaTypeConfig, quality string, searchtype string, data *searchstruct) {
	defer data.close()
	var results *SearchResults
	var err error
	switch searchtype {
	case "rss":
		results, err = SearchRSS(cfgp, quality, data.mediatype, false)
	case "rssseason":
		results, err = SearchSeriesRSSSeason(cfgp, quality, data.mediatype, data.thetvdbid, data.season, data.useseason)
	case "movie":
		results, err = MovieSearch(cfgp, data.movieid, false, data.titlesearch)
	case "series":
		results, err = SeriesSearch(cfgp, data.episodeid, false, data.titlesearch)
	}

	if err != nil {
		data = nil
		return
	}
	var downloaded = make([]uint, 0, len(results.Nzbs))
	var breakfor bool
	var downloadnow *downloader.Downloadertype
	for idx := range results.Nzbs {
		breakfor = false
		for idxs := range downloaded {
			if downloaded[idxs] == results.Nzbs[idx].NzbmovieID && results.Nzbs[idx].NzbmovieID != 0 {
				breakfor = true
				break
			}
			if downloaded[idxs] == results.Nzbs[idx].NzbepisodeID && results.Nzbs[idx].NzbepisodeID != 0 {
				breakfor = true
				break
			}
		}
		if breakfor {
			break
		}
		logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.Stringp("title", &results.Nzbs[idx].NZB.Title), zap.Intp("minimum prio", &results.Nzbs[idx].MinimumPriority), zap.Intp("prio", &results.Nzbs[idx].ParseInfo.Priority), zap.Stringp("quality", &results.Nzbs[idx].QualityTemplate))

		downloadnow = downloader.NewDownloader(cfgp)

		if data.mediatype == "movie" {
			downloaded = append(downloaded, results.Nzbs[idx].NzbmovieID)
			downloadnow.SetMovie(results.Nzbs[idx].NzbmovieID)
		} else {
			downloaded = append(downloaded, results.Nzbs[idx].NzbepisodeID)
			downloadnow.SetSeriesEpisode(results.Nzbs[idx].NzbepisodeID)
		}
		downloadnow.Nzb = results.Nzbs[idx]
		downloadnow.DownloadNzb()
		downloadnow.Close()
	}
	results.Close()
	data = nil
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
			s.Nzbs[idx].Close()
			s.Nzbs[idx].Close()
		}
	}
	s.Nzbs = nil
	if len(s.Rejected) >= 1 {
		for idx := range s.Rejected {
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
	//s.Cfgp.Close()
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
				SearchGroupType:  searchGroupType,
				SearchActionType: "rss",
			}).rsssearchindexer(&searchstruct{quality: quality, mediatype: "rss", titlesearch: false, indexer: indexertemplate}, fetchall, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	//dl := s.queryindexers(s.Quality, "rss", fetchall, &processedindexer, false, queryparams{})

	if processedindexer == 0 && len(config.Cfg.Quality[quality].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) rsssearchindexer(search *searchstruct, fetchall bool, dl *SearchResults) bool {
	defer search.close()
	// if addsearched(dl, search.indexer+search.quality) {
	// 	return true
	// }
	var nzbindexer apiexternal.NzbIndexer
	cats, erri := s.initIndexer(search, "rss", &nzbindexer)
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
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &search.indexer), zap.Error(erri))
		// nzbindexer.Close()
		return false
	}

	if fetchall {
		nzbindexer.LastRssID = ""
	}
	maxentries := config.Cfg.Indexers[search.indexer].MaxRssEntries
	maxloop := config.Cfg.Indexers[search.indexer].RssEntriesloop
	if maxentries == 0 {
		maxentries = 10
	}
	if maxloop == 0 {
		maxloop = 2
	}

	nzbs, _, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&nzbindexer, maxentries, cats, maxloop)
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.Stringp("indexer", &search.indexer), zap.Error(nzberr))
		// nzbindexer.Close()
		return false
	}
	defer nzbs.Close()
	if !fetchall && lastids != "" && len((nzbs.Arr)) >= 1 {
		addrsshistory(nzbindexer.URL, lastids, s.Quality, s.Cfgp.NamePrefix)
	}
	if nzbs != nil && len((nzbs.Arr)) >= 1 {
		logger.Log.GlobalLogger.Debug("Search RSS ended - found entries", zap.Int("entries", len((nzbs.Arr))), zap.Stringp("indexer", &nzbindexer.Name))
		s.parseentries(nzbs, dl, search, "", false)
	}
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
				SearchGroupType:  searchGroupType,
				SearchActionType: "rss",
			}).rssqueryseriesindexer(&searchstruct{quality: quality, mediatype: "rss", titlesearch: false, indexer: indexertemplate}, thetvdbid, season, useseason, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()

	if processedindexer == 0 && len(config.Cfg.Quality[quality].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) rssqueryseriesindexer(search *searchstruct, thetvdbid int, season int, useseason bool, dl *SearchResults) bool {
	defer search.close()
	var nzbindexer apiexternal.NzbIndexer
	cats, erri := s.initIndexer(search, "api", &nzbindexer)
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
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &search.indexer), zap.Error(erri))
		// nzbindexer.Close()
		return false
	}

	nzbs, _, nzberr := apiexternal.QueryNewznabTvTvdb(&nzbindexer, thetvdbid, cats, season, 0, useseason, false)
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.Stringp("indexer", &search.indexer), zap.Error(nzberr))
		// nzbindexer.Close()
		return false
	}
	defer nzbs.Close()

	if nzbs != nil && len((nzbs.Arr)) >= 1 {
		s.parseentries(nzbs, dl, search, "", false)
		return true
	}
	return false
	// nzbindexer.Close()
}

func (s *Searcher) checkhistory(search *searchstruct, historyurlcache *cache.Return, historytitlecache *cache.Return, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Check History
	if len(entry.NZB.DownloadURL) > 1 {
		for idx := range historyurlcache.Value.([]string) {
			if strings.EqualFold(historyurlcache.Value.([]string)[idx], entry.NZB.DownloadURL) {
				denynzb("Already downloaded (Url)", entry, dl)
				return true
			}
		}
	}
	if config.QualityIndexerByQualityAndTemplateGetFieldBool(search.quality, search.indexer, "HistoryCheckTitle") && len(entry.NZB.Title) > 1 {
		for idx := range historytitlecache.Value.([]string) {
			if strings.EqualFold(historytitlecache.Value.([]string)[idx], entry.NZB.Title) {
				denynzb("Already downloaded (Title)", entry, dl)
				// historycache.Close()
				return true
			}
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
			if s.getmovierss(entry, addinlist, addifnotfound, s.Cfgp, dl) {
				return true
			}
			entry.WantedTitle = s.title
			entry.QualityTemplate = s.Movie.QualityProfile

			//Check Minimal Priority
			entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, s.Cfgp, s.Movie.QualityProfile)

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
			entry.MinimumPriority = s.MinimumPriority
			if s.MinimumPriority == 0 {
				entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, s.Cfgp, s.Movie.QualityProfile)
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
		entry.WantedTitle = s.title

		//Check Minimum Priority
		entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, s.Cfgp, entry.QualityTemplate)

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
		entry.MinimumPriority = s.MinimumPriority
		if s.MinimumPriority == 0 {
			entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, s.Cfgp, entry.QualityTemplate)
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
	if (config.Cfg.Quality[entry.QualityTemplate].CheckYear || config.Cfg.Quality[entry.QualityTemplate].CheckYear1) && strings.Contains(entry.NZB.Title, stryear) {
		return false
	}
	if config.Cfg.Quality[entry.QualityTemplate].CheckYear1 && strings.Contains(entry.NZB.Title, logger.IntToString(s.year+1)) {
		return false
	}
	if config.Cfg.Quality[entry.QualityTemplate].CheckYear1 && strings.Contains(entry.NZB.Title, logger.IntToString(s.year-1)) {
		return false
	}
	denynzb("Unwanted Year", entry, dl)
	return true
}
func Checktitle(entry *apiexternal.Nzbwithprio, searchGroupType string, dl *SearchResults) bool {
	//Checktitle
	if !config.Cfg.Quality[entry.QualityTemplate].CheckTitle {
		return false
	}
	yearstr := logger.IntToString(entry.ParseInfo.Year)
	lentitle := len(entry.WantedTitle)
	title := importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityTemplate)
	//title := entry.ParseInfo.Title
	if entry.WantedTitle != "" {
		if config.Cfg.Quality[entry.QualityTemplate].CheckTitle && lentitle >= 1 && apiexternal.Checknzbtitle(entry.WantedTitle, title) {
			return false
		}
		if entry.ParseInfo.Year != 0 && config.Cfg.Quality[entry.QualityTemplate].CheckTitle && lentitle >= 1 && apiexternal.Checknzbtitle(entry.WantedTitle, title+" "+yearstr) {
			return false
		}
	}
	if strings.EqualFold(searchGroupType, "movie") && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}, &entry.WantedAlternates)
	}
	if strings.EqualFold(searchGroupType, "series") && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: "select distinct title from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{entry.Dbid}}, &entry.WantedAlternates)
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

		if entry.ParseInfo.Year != 0 && config.Cfg.Quality[entry.QualityTemplate].CheckTitle && apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], title+" "+yearstr) {
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
	var matchfound bool

	lowerIdentifier := strings.ToLower(s.identifier)
	lowerParseIdentifier := strings.ToLower(entry.ParseInfo.Identifier)
	lowerTitle := strings.ToLower(entry.NZB.Title)
	altIdentifier := strings.ReplaceAll(strings.TrimLeft(lowerIdentifier, "s0"), "e", "x")
	if strings.Contains(lowerTitle, lowerIdentifier) ||
		strings.Contains(lowerTitle, altIdentifier) {
		return false
	}
	if strings.Contains(lowerIdentifier, "-") {
		if strings.Contains(lowerTitle, strings.ReplaceAll(lowerIdentifier, "-", ".")) ||
			strings.Contains(lowerTitle, strings.ReplaceAll(lowerIdentifier, "-", " ")) ||
			strings.Contains(lowerTitle, strings.ReplaceAll(altIdentifier, "-", ".")) ||
			strings.Contains(lowerTitle, strings.ReplaceAll(altIdentifier, "-", " ")) {
			return false
		}
	}

	seasonarray := []string{"s" + s.season + "e", "s0" + s.season + "e", "s" + s.season + " e", "s0" + s.season + " e", s.season + "x", s.season + " x"}
	episodearray := []string{"e" + s.episode, "e0" + s.episode, "x" + s.episode, "x0" + s.episode}
	for idxseason := range seasonarray {
		if !strings.HasPrefix(lowerParseIdentifier, seasonarray[idxseason]) {
			continue
		}
		for idxepisode := range episodearray {
			if strings.HasSuffix(lowerParseIdentifier, episodearray[idxepisode]) {
				matchfound = true
			} else if strings.Contains(lowerParseIdentifier, episodearray[idxepisode]+" ") {
				matchfound = true
			} else if strings.Contains(lowerParseIdentifier, episodearray[idxepisode]+"-") {
				matchfound = true
			} else if strings.Contains(lowerParseIdentifier, episodearray[idxepisode]+"e") {
				matchfound = true
			} else if strings.Contains(lowerParseIdentifier, episodearray[idxepisode]+"x") {
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

func filterTestQualityWanted(quality *config.QualityConfig, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	var wanted bool
	lenqual := len(quality.WantedResolutionIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Resolution != "" && logger.InStringArray(entry.ParseInfo.Resolution, &quality.WantedResolutionIn) {
		wanted = true
	}

	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted resolution", entry, dl, entry.ParseInfo.Resolution)
		return false
	}
	wanted = false

	lenqual = len(quality.WantedQualityIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Quality != "" && logger.InStringArray(entry.ParseInfo.Quality, &quality.WantedQualityIn) {
		wanted = true
	}
	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted quality", entry, dl, entry.ParseInfo.Quality)

		return false
	}

	wanted = false

	lenqual = len(quality.WantedAudioIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Audio != "" && logger.InStringArray(entry.ParseInfo.Audio, &quality.WantedAudioIn) {
		wanted = true
	}
	if lenqual >= 1 && !wanted {
		denynzb("Skipped - unwanted audio", entry, dl, entry.ParseInfo.Audio)

		return false
	}
	wanted = false

	lenqual = len(quality.WantedCodecIn.Arr)
	if lenqual >= 1 && entry.ParseInfo.Codec != "" && logger.InStringArray(entry.ParseInfo.Codec, &quality.WantedCodecIn) {
		wanted = true
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
		if !allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
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
		if !allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
			denynzb("Not Allowed Movie", entry, dl)
			return true
		}
		loopdbmovie = importfeed.JobImportMovies(entry.NZB.IMDBID, cfgp, addinlist, true)
		database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylist, Args: []interface{}{loopdbmovie, addinlist}}, &loopmovie)
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
	entry.ParseInfo.Title = importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityTemplate)
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

		//if entry.ParseInfo.DbserieID != 0 {
		//	database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{entry.ParseInfo.DbserieID}}, &entry.WantedAlternates)
		//}
	}
	return false
}

func (s *Searcher) GetRSSFeed(searchGroupType string, cfgp *config.MediaTypeConfig, listname string) (*SearchResults, error) {
	if cfgp.ListsMap[listname].TemplateList == "" {
		return nil, errNoList
	}
	if !config.Check("list_" + cfgp.ListsMap[listname].TemplateList) {
		return nil, errNoList
	}
	if !config.Check("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	for idx := range config.Cfg.Quality[s.Quality].Indexer {
		if config.Cfg.Quality[s.Quality].Indexer[idx].TemplateIndexer == cfgp.ListsMap[listname].TemplateList {
			if config.Cfg.Quality[s.Quality].Indexer[idx].TemplateRegex == "" {
				return nil, errNoRegex
			}
			break
		}
	}

	var lastindexerid string
	database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{cfgp.ListsMap[listname].TemplateList, s.Quality, ""}}, &lastindexerid)

	blockinterval := -5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.Cfg.General.FailedIndexerBlockTime
	}
	var counter int
	database.QueryColumn(&database.Querywithargs{QueryString: "select count() from indexer_fails where indexer = ? and last_fail > ?", Args: []interface{}{config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].URL, time.Now().Add(time.Minute * time.Duration(blockinterval))}}, &counter)
	if counter >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fail in the last ", zap.Int("Minutes", blockinterval), zap.String("Listname", cfgp.ListsMap[listname].TemplateList))
		return nil, errIndexerDisabled
	}
	nzbs, _, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&apiexternal.NzbIndexer{Name: cfgp.ListsMap[listname].TemplateList, Customrssurl: config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].URL, LastRssID: lastindexerid}, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Limit, "", 1)
	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed", zap.Error(nzberr))
		// indexer.Close()
		return nil, nzberr
	} else {
		defer nzbs.Close()
		if lastids != "" && len((nzbs.Arr)) >= 1 {
			addrsshistory(config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].URL, lastids, s.Quality, cfgp.ListsMap[listname].TemplateList)
		}
		if nzbs != nil && len((nzbs.Arr)) >= 1 {
			dl := SearchResults{mu: &sync.Mutex{}}

			s.parseentries(nzbs, &dl, &searchstruct{quality: s.Quality, indexer: cfgp.ListsMap[listname].TemplateList}, listname, cfgp.ListsMap[listname].Addfound)
			if len(dl.Nzbs) > 1 {
				sort.Slice(dl.Nzbs, func(i, j int) bool {
					return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
				})
			}
			// indexer.Close()
			return &dl, nil
		}
		return nil, errOther
	}
}

func addrsshistory(url string, lastid string, quality string, config string) {
	database.UpsertNamed("r_sshistories",
		&logger.InStringArrayStruct{Arr: []string{"indexer", "last_id", "list", "config"}},
		database.RSSHistory{Indexer: url, LastID: lastid, List: quality, Config: config},
		"config = :config COLLATE NOCASE and list = :list COLLATE NOCASE and indexer = :indexer COLLATE NOCASE",
		&database.Querywithargs{Query: database.Query{Where: "config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE"}, Args: []interface{}{config, quality, url}})
}

func MovieSearch(cfgp *config.MediaTypeConfig, movieid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	var movie database.Movie
	err := database.GetMovies(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{movieid}}, &movie)
	if err != nil {
		logger.Log.GlobalLogger.Debug("Skipped - movie not found")
		return nil, err
	}
	if movie.DbmovieID == 0 {
		return nil, errors.New("missing data")
	}

	if movie.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Skipped - Search disabled")
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

	minimumPriority := GetHighestMoviePriorityByFiles(false, true, movie.ID, cfgp, movie.QualityProfile)
	var alternateTitles []string
	database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ''", Args: []interface{}{movie.DbmovieID}}, &alternateTitles)

	searchActionType := "upgrade"
	if minimumPriority == 0 {
		searchActionType = "missing"
	} else {
		if movie.DontUpgrade && !forceDownload {
			logger.Log.GlobalLogger.Debug("Skipped - Upgrade disabled", zap.Stringp("title", &title))
			return nil, errUpgradeDisabled
		}
	}

	var processedindexer int

	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	for idx := range config.Cfg.Quality[movie.QualityProfile].Indexer {
		indexertemplate := config.Cfg.Quality[movie.QualityProfile].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Enabled {
			processedindexer++
			continue
		}
		workergroup.Submit(func() {
			logger.Log.GlobalLogger.Debug("Search for movie id", zap.Uint("id", movie.ID))
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          movie.QualityProfile,
				SearchGroupType:  "movie",
				SearchActionType: searchActionType,
				MinimumPriority:  minimumPriority,
				Movie:            movie,
				imdb:             imdb,
				year:             year,
				title:            title,
				AlternateTitles:  alternateTitles,
			}).mediasearchindexer(&searchstruct{id: movie.ID, title: title, searchid: imdb, quality: movie.QualityProfile, mediatype: "movie", titlesearch: titlesearch, indexer: indexertemplate}, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()

	if processedindexer == 0 && len(config.Cfg.Quality[movie.QualityProfile].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer >= 1 {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set lastscan = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, movieid}})
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) mediasearchindexer(search *searchstruct, dl *SearchResults) bool {
	defer search.close()
	var err error
	_, search.cats, err = s.initIndexerURLCat(search, "api")
	if err != nil {
		if err == errIndexerDisabled || err == errNoIndexer {
			return true
		}
		if err == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &search.indexer), zap.Error(err))

		return false
	}
	var qualityprofile string

	if search.mediatype == "movie" {
		qualityprofile = s.Movie.QualityProfile
	} else if search.mediatype == "series" {
		qualityprofile = s.SerieEpisode.QualityProfile
	}
	//logger.Log.Debug("Qualty: ", qualityprofile)

	if !config.Check("quality_" + qualityprofile) {
		logger.Log.GlobalLogger.Error("Quality for: " + search.searchid + spacenotfound)
		return false
	}

	//inittitle := search.titlesearch
	//func (s *Searcher) searchMedia(mediatype string, searchid string, searchtitle bool, id uint, quality string, indexer string, title string, season int, episode int, cats string, dl *SearchResults) bool {
	if !search.titlesearch && search.searchid != "" && search.searchid != "0" {
		s.searchMedia(search, "", dl)
		if len(dl.Nzbs) >= 1 {
			if database.DBLogLevel == "debug" {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Title", &search.title), zap.Stringp("Indexer", &search.indexer), zap.Int("entries", len(dl.Nzbs)))
			}

			if config.Cfg.Quality[qualityprofile].CheckUntilFirstFound { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound {
				return true
			}
		}
	}
	if config.Cfg.Quality[qualityprofile].SearchForTitleIfEmpty && len(dl.Nzbs) == 0 {
		search.titlesearch = true
	}
	if !search.titlesearch {
		return true
	}
	var addstr, searchfor string
	if search.mediatype == "movie" && s.year != 0 {
		addstr = " (" + logger.IntToString(s.year) + ")"
	} else if search.mediatype == "series" && s.identifier != "" {
		addstr = " " + s.identifier
	}
	var searched logger.InStringArrayStruct
	defer searched.Close()
	if config.Cfg.Quality[qualityprofile].BackupSearchForTitle { //config.Cfg.Quality[qualityprofile].BackupSearchForTitle
		searchfor = strings.ReplaceAll(search.title, "(", "")
		searchfor = strings.ReplaceAll(searchfor, ")", "")
		searchfor += addstr
		searched.Arr = append(searched.Arr, searchfor)
		s.searchMedia(search, searchfor, dl)

		if len(dl.Nzbs) >= 1 {
			if database.DBLogLevel == "debug" {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Searchfor", &search.title), zap.Stringp("Indexer", &search.indexer), zap.Int("entries", len(dl.Nzbs)))
			}

			if config.Cfg.Quality[qualityprofile].CheckUntilFirstFound { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound
				return true
			}
		}
	}
	if config.Cfg.Quality[qualityprofile].SearchForAlternateTitleIfEmpty && len(dl.Nzbs) == 0 {
		search.titlesearch = true
	}
	if !config.Cfg.Quality[qualityprofile].BackupSearchForAlternateTitle {
		return true
	}
	for idxalt := range s.AlternateTitles {
		if s.AlternateTitles[idxalt] == "" {
			continue
		}
		searchfor = strings.ReplaceAll(s.AlternateTitles[idxalt], "(", "")
		searchfor = strings.ReplaceAll(searchfor, ")", "")
		searchfor += addstr
		if logger.InStringArray(searchfor, &searched) {
			continue
		}
		searched.Arr = append(searched.Arr, searchfor)
		s.searchMedia(search, searchfor, dl)

		if len(dl.Nzbs) == 0 {
			continue
		}
		if database.DBLogLevel == "debug" {
			logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Stringp("Searchfor", &searchfor), zap.Stringp("Indexer", &search.indexer), zap.Int("entries", len(dl.Nzbs)))
		}

		if config.Cfg.Quality[qualityprofile].CheckUntilFirstFound {
			break
		}
	}
	return true
}

func SeriesSearch(cfgp *config.MediaTypeConfig, episodeid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	var serieEpisode database.SerieEpisode
	err := database.GetSerieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodeid}}, &serieEpisode)
	if err != nil {
		return nil, err
	}
	if serieEpisode.DbserieEpisodeID == 0 || serieEpisode.DbserieID == 0 || serieEpisode.SerieID == 0 {
		return nil, errors.New("missing data")
	}
	if serieEpisode.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Search not wanted: ")
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

	minimumPriority := GetHighestEpisodePriorityByFiles(false, true, serieEpisode.ID, cfgp, serieEpisode.QualityProfile)
	var alternateTitles []string
	database.QueryStaticStringArray(false, 10, &database.Querywithargs{QueryString: "select distinct title from dbserie_alternates where dbserie_id = ? and title != ''", Args: []interface{}{serieEpisode.DbserieID}}, &alternateTitles)

	searchActionType := "upgrade"
	if minimumPriority == 0 {
		searchActionType = "missing"
	} else {
		if serieEpisode.DontUpgrade && !forceDownload {
			logger.Log.GlobalLogger.Debug("Upgrade not wanted", zap.Stringp("title", &title))
			return nil, errUpgradeDisabled
		}
	}

	var processedindexer int
	dl := SearchResults{mu: &sync.Mutex{}}
	workergroup := logger.WorkerPools["Indexer"].Group()

	for idx := range config.Cfg.Quality[serieEpisode.QualityProfile].Indexer {
		indexertemplate := config.Cfg.Quality[serieEpisode.QualityProfile].Indexer[idx].TemplateIndexer
		if !config.Cfg.Indexers[indexertemplate].Enabled {
			processedindexer++
			continue
		}
		workergroup.Submit(func() {
			var searchid string
			if thetvdbid != 0 {
				searchid = logger.IntToString(thetvdbid)
			}
			var seasonset, episodeset int
			if season != "" {
				seasonset = logger.StringToInt(season)
			}
			if episode != "" {
				episodeset = logger.StringToInt(episode)
			}
			logger.Log.GlobalLogger.Debug("Search for serie id", zap.Uint("id", serieEpisode.ID))
			if (&Searcher{
				Cfgp:             cfgp,
				Quality:          serieEpisode.QualityProfile,
				SearchGroupType:  "series",
				SearchActionType: searchActionType,
				MinimumPriority:  minimumPriority,
				SerieEpisode:     serieEpisode,
				title:            title,
				thetvdbid:        thetvdbid,
				season:           season,
				episode:          episode,
				identifier:       identifier,
				AlternateTitles:  alternateTitles,
			}).mediasearchindexer(&searchstruct{id: serieEpisode.ID, title: title, searchid: searchid, season: seasonset, episode: episodeset, episodeid: serieEpisode.ID, quality: serieEpisode.QualityProfile, mediatype: "series", titlesearch: titlesearch, indexer: indexertemplate}, &dl) {
				processedindexer++
			}
		})
	}
	workergroup.Wait()
	//dl := s.queryindexers(s.SerieEpisode.QualityProfile, "series", false, &processedindexer, titlesearch, queryparams{})

	if processedindexer == 0 && len(config.Cfg.Quality[serieEpisode.QualityProfile].Indexer) >= 1 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}

	if processedindexer >= 1 {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set lastscan = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, episodeid}})
	}

	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	return &dl, nil
}

func (s *Searcher) initIndexer(search *searchstruct, rssapi string, nzbIndexer *apiexternal.NzbIndexer) (string, error) {
	if !config.Check("indexer_" + search.indexer) {
		return "", errNoIndexer
	}
	if !strings.EqualFold(config.Cfg.Indexers[search.indexer].IndexerType, "newznab") {
		// idxcfg.Close()
		return "", errors.New("indexer Type Wrong")
	}
	if !config.Cfg.Indexers[search.indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return "", errIndexerDisabled
	} else if !config.Cfg.Indexers[search.indexer].Enabled {
		// idxcfg.Close()
		return "", errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[search.indexer].URL); !ok {
		// idxcfg.Close()
		return "", errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, config.Cfg.Indexers[search.indexer].URL}}, &lastindexerid)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    config.Cfg.Indexers[search.indexer].URL,
		Apikey:                 config.Cfg.Indexers[search.indexer].Apikey,
		UserID:                 config.Cfg.Indexers[search.indexer].Userid,
		SkipSslCheck:           config.Cfg.Indexers[search.indexer].DisableTLSVerify,
		DisableCompression:     config.Cfg.Indexers[search.indexer].DisableCompression,
		Addquotesfortitlequery: config.Cfg.Indexers[search.indexer].Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "AdditionalQueryParams"),
		LastRssID:              lastindexerid,
		Customapi:              config.Cfg.Indexers[search.indexer].Customapi,
		Customurl:              config.Cfg.Indexers[search.indexer].Customurl,
		Customrssurl:           config.Cfg.Indexers[search.indexer].Customrssurl,
		Customrsscategory:      config.Cfg.Indexers[search.indexer].Customrsscategory,
		Limitercalls:           config.Cfg.Indexers[search.indexer].Limitercalls,
		Limiterseconds:         config.Cfg.Indexers[search.indexer].Limiterseconds,
		LimitercallsDaily:      config.Cfg.Indexers[search.indexer].LimitercallsDaily,
		TimeoutSeconds:         config.Cfg.Indexers[search.indexer].TimeoutSeconds,
		MaxAge:                 config.Cfg.Indexers[search.indexer].MaxAge,
		OutputAsJSON:           config.Cfg.Indexers[search.indexer].OutputAsJSON}

	// idxcfg.Close()
	return config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "CategoriesIndexer"), nil
}

func (s *Searcher) initIndexerURLCat(search *searchstruct, rssapi string) (string, string, error) {
	if !config.Check("indexer_" + search.indexer) {
		return "", "", errNoIndexer
	}

	if !strings.EqualFold(config.Cfg.Indexers[search.indexer].IndexerType, "newznab") {
		// idxcfg.Close()
		return "", "", errors.New("indexer Type Wrong")
	}
	if !config.Cfg.Indexers[search.indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return "", "", errIndexerDisabled
	} else if !config.Cfg.Indexers[search.indexer].Enabled {
		// idxcfg.Close()
		return "", "", errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[search.indexer].URL); !ok {
		// idxcfg.Close()
		return "", "", errToWait
	}
	// defer idxcfg.Close()
	return config.Cfg.Indexers[search.indexer].URL, config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "CategoriesIndexer"), nil
}

func (s *Searcher) initNzbIndexer(search *searchstruct, rssapi string, nzbIndexer *apiexternal.NzbIndexer) error {
	if !config.Check("indexer_" + search.indexer) {
		return errNoIndexer
	}
	if !strings.EqualFold(config.Cfg.Indexers[search.indexer].IndexerType, "newznab") {
		// idxcfg.Close()
		return errors.New("indexer type wrong")
	}
	if !config.Cfg.Indexers[search.indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		// idxcfg.Close()
		return errIndexerDisabled
	} else if !config.Cfg.Indexers[search.indexer].Enabled {
		// idxcfg.Close()
		return errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[search.indexer].URL); !ok {
		// idxcfg.Close()
		return errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, config.Cfg.Indexers[search.indexer].URL}}, &lastindexerid)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    config.Cfg.Indexers[search.indexer].URL,
		Apikey:                 config.Cfg.Indexers[search.indexer].Apikey,
		UserID:                 config.Cfg.Indexers[search.indexer].Userid,
		SkipSslCheck:           config.Cfg.Indexers[search.indexer].DisableTLSVerify,
		DisableCompression:     config.Cfg.Indexers[search.indexer].DisableCompression,
		Addquotesfortitlequery: config.Cfg.Indexers[search.indexer].Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "AdditionalQueryParams"),
		LastRssID:              lastindexerid,
		Customapi:              config.Cfg.Indexers[search.indexer].Customapi,
		Customurl:              config.Cfg.Indexers[search.indexer].Customurl,
		Customrssurl:           config.Cfg.Indexers[search.indexer].Customrssurl,
		Customrsscategory:      config.Cfg.Indexers[search.indexer].Customrsscategory,
		Limitercalls:           config.Cfg.Indexers[search.indexer].Limitercalls,
		Limiterseconds:         config.Cfg.Indexers[search.indexer].Limiterseconds,
		LimitercallsDaily:      config.Cfg.Indexers[search.indexer].LimitercallsDaily,
		TimeoutSeconds:         config.Cfg.Indexers[search.indexer].TimeoutSeconds,
		MaxAge:                 config.Cfg.Indexers[search.indexer].MaxAge,
		OutputAsJSON:           config.Cfg.Indexers[search.indexer].OutputAsJSON}

	// idxcfg.Close()
	return nil
}

func (s *Searcher) searchMedia(search *searchstruct, title string, dl *SearchResults) {
	if !config.Check("quality_" + search.quality) {
		logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for: %d%s", search.id, spacenotfound))
		return
	}
	var nzbindexer apiexternal.NzbIndexer
	erri := s.initNzbIndexer(search, "api", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			logger.Log.GlobalLogger.Debug("No Indexer", zap.Stringp("indexer", &search.indexer), zap.Error(erri))
			// nzbindexer.Close()
			return
		}
		if erri == errToWait {
			logger.Log.GlobalLogger.Debug("Indexer needs waiting", zap.Stringp("indexer", &search.indexer), zap.Error(erri))
			time.Sleep(10 * time.Second)
			// nzbindexer.Close()
			return
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.Stringp("indexer", &search.indexer), zap.Error(erri))
		// nzbindexer.Close()
		return
	}

	var nzbs *apiexternal.NZBArr
	var nzberr error

	if !search.titlesearch {
		if search.mediatype == "movie" && s.imdb != "" {
			nzbs, _, nzberr = apiexternal.QueryNewznabMovieImdb(&nzbindexer, strings.Trim(s.imdb, "t"), search.cats)
		} else if search.mediatype == "series" && s.thetvdbid != 0 {
			nzbs, _, nzberr = apiexternal.QueryNewznabTvTvdb(&nzbindexer, s.thetvdbid, search.cats, search.season, search.episode, true, true)
		}
	} else {
		nzbs, _, nzberr = apiexternal.QueryNewznabQuery(&nzbindexer, title, search.cats, "search")
	}
	if nzberr != nil && nzberr != apiexternal.Errnoresults {
		logger.Log.GlobalLogger.Error("Newznab Search failed", zap.Stringp("title", &search.title), zap.Stringp("indexer", &nzbindexer.URL), zap.Error(nzberr))
	} else if nzbs != nil && len((nzbs.Arr)) >= 1 {
		s.parseentries(nzbs, dl, search, "", false)

		if database.DBLogLevel == "debug" {
			logger.Log.GlobalLogger.Debug("Entries found", zap.Stringp("indexer", &search.indexer), zap.Stringp("title", &search.title), zap.Int("Count", len(nzbs.Arr)))
		}
	}
	// nzbindexer.Close()
	nzbs.Close()
}

func filterSizeNzbs(cfgp *config.MediaTypeConfig, indexer *config.QualityIndexerConfig, entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	if indexer.SkipEmptySize && entry.NZB.Size == 0 {
		denynzb("Missing size", entry, dl)
		return true
	}
	for idx := range cfgp.DataImport {
		if !config.Check("path_" + cfgp.DataImport[idx].TemplatePath) {
			return false
		}

		if config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].MinSize != 0 && entry.NZB.Size < config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].MinSizeByte {
			denynzb("Too Small", entry, dl)
			return true
		}

		if config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].MaxSize != 0 && entry.NZB.Size > config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].MaxSizeByte {
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

	for idx := range config.Cfg.Regex[templateregex].Required {
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Required[idx], s.NZB.Title, 1) {
			requiredmatched = true
			break
		}
	}
	if len(config.Cfg.Regex[templateregex].Required) >= 1 && !requiredmatched {
		denynzb(deniedbyregex, s, dl, "required not matched")
		return true
	}
	for idx := range config.Cfg.Regex[templateregex].Rejected {
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], s.WantedTitle, 1) {
			//Regex is in title - skip test
			continue
		}
		breakfor = false
		for idxwanted := range s.WantedAlternates {
			if s.WantedAlternates[idxwanted] == s.WantedTitle {
				continue
			}
			if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], s.WantedAlternates[idxwanted], 1) {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], s.NZB.Title, 1) {
			//logger.Log.GlobalLogger.Debug(regexrejected, zap.String("title", title), zap.String("regex", config.Cfg.Regex[templateregex].Rejected[idx]))
			denynzb(deniedbyregex, s, dl, config.Cfg.Regex[templateregex].Rejected[idx])
			return true
		}
	}
	return false
}
func (s *Searcher) parseentries(nzbs *apiexternal.NZBArr, dl *SearchResults, search *searchstruct, listname string, addfound bool) {
	if len((nzbs.Arr)) == 0 {
		return
	}
	if !config.Check("regex_" + config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "TemplateRegex")) {
		dl.mu.Lock()
		if len(dl.Rejected) == 0 {
			dl.Rejected = make([]apiexternal.Nzbwithprio, 0, len(nzbs.Arr))
		} else {
			dl.Rejected = logger.GrowSliceBy(dl.Rejected, len(nzbs.Arr))
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
	} else {
		dl.Rejected = logger.GrowSliceBy(dl.Rejected, len(nzbs.Arr))
	}
	dl.mu.Unlock()
	var index *config.QualityIndexerConfig
	var parsefile, includeyear, denied bool
	var conf config.QualityConfig

	historytable := "serie_episode_histories"
	if strings.EqualFold(s.SearchGroupType, "movie") {
		historytable = "movie_histories"
	}
	var get []string
	historyurlcache := new(cache.Return)
	if !logger.GlobalCache.CheckNoType(historytable + "_url") {
		database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select url from " + historytable}, &get)
		historyurlcache.Value = get
		logger.GlobalCache.Set(historytable+"_url", historyurlcache.Value, 8*time.Hour)
	} else {
		historyurlcache = logger.GlobalCache.GetData(historytable + "_url")
	}

	historytitlecache := new(cache.Return)
	if !logger.GlobalCache.CheckNoType(historytable + "_title") {
		get = []string{}
		database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select title from " + historytable}, &get)
		historytitlecache.Value = get
		logger.GlobalCache.Set(historytable+"_title", get, 8*time.Hour)
	} else {
		historytitlecache = logger.GlobalCache.GetData(historytable + "_title")
	}
	get = nil

	for entryidx := range nzbs.Arr {
		nzbs.Arr[entryidx].Indexer = search.indexer

		//Check Title Length
		if nzbs.Arr[entryidx].NZB.DownloadURL == "" {
			denynzb("No Url", &nzbs.Arr[entryidx], dl)
			continue
		}
		if nzbs.Arr[entryidx].NZB.Title == "" {
			denynzb("No Title", &nzbs.Arr[entryidx], dl)
			continue
		}
		if len(strings.Trim(nzbs.Arr[entryidx].NZB.Title, " ")) <= 3 {
			denynzb("Title too short", &nzbs.Arr[entryidx], dl)
			continue
		}
		denied = false
		for idx := range dl.Rejected {
			if dl.Rejected[idx].NZB.DownloadURL == nzbs.Arr[entryidx].NZB.DownloadURL {
				//denynzb("Already rejected", &nzbs.Arr[entryidx], dl)
				denied = true
				break
			}
		}
		if denied {
			continue
		}
		dl.mu.Lock()
		for idx := range dl.Nzbs {
			if dl.Nzbs[idx].NZB.DownloadURL == nzbs.Arr[entryidx].NZB.DownloadURL {
				denied = true
				break
			}
		}
		dl.mu.Unlock()
		if denied {
			denynzb("Already added", &nzbs.Arr[entryidx], dl)
			continue
		}

		//Check Size
		index = config.QualityIndexerByQualityAndTemplate(search.quality, nzbs.Arr[entryidx].Indexer)
		if index.TemplateRegex == "" {
			index.Close()
			denynzb("No Indexer Regex Template", &nzbs.Arr[entryidx], dl)
			continue
		}
		if filterSizeNzbs(s.Cfgp, index, &nzbs.Arr[entryidx], dl) {
			index.Close()
			continue
		}

		if s.checkhistory(search, historyurlcache, historytitlecache, &nzbs.Arr[entryidx], dl) {
			index.Close()
			continue
		}

		if s.checkcorrectid(&nzbs.Arr[entryidx], dl) {
			index.Close()
			continue
		}

		//Regex
		if filterRegexNzbs(&nzbs.Arr[entryidx], index.TemplateRegex, dl) {
			index.Close()
			continue
		}
		index.Close()

		//Parse
		parsefile = false
		if nzbs.Arr[entryidx].ParseInfo.File == "" {
			parsefile = true
		} else if nzbs.Arr[entryidx].ParseInfo.File != "" && (nzbs.Arr[entryidx].ParseInfo.Title == "" || nzbs.Arr[entryidx].ParseInfo.Resolution == "" || nzbs.Arr[entryidx].ParseInfo.Quality == "") {
			parsefile = true
		}
		if parsefile {
			includeyear = false
			if s.SearchGroupType == "series" {
				includeyear = true
			}
			nzbs.Arr[entryidx].ParseInfo = *parser.NewFileParser(nzbs.Arr[entryidx].NZB.Title, includeyear, s.SearchGroupType)
			//entries.Arr[entryidx].ParseInfo, err = parser.NewFileParserNoPt(entries.Arr[entryidx].NZB.Title, includeyear, s.SearchGroupType)
		}

		if s.getmediadata(&nzbs.Arr[entryidx], dl, listname, addfound) {
			continue
		}

		if nzbs.Arr[entryidx].ParseInfo.Priority == 0 {
			parser.GetPriorityMap(&nzbs.Arr[entryidx].ParseInfo, s.Cfgp, nzbs.Arr[entryidx].QualityTemplate, false, true)
			nzbs.Arr[entryidx].Prio = nzbs.Arr[entryidx].ParseInfo.Priority
		}

		nzbs.Arr[entryidx].ParseInfo.Title = importfeed.StripTitlePrefixPostfix(nzbs.Arr[entryidx].ParseInfo.Title, nzbs.Arr[entryidx].QualityTemplate)

		//check quality
		conf = config.Cfg.Quality[nzbs.Arr[entryidx].QualityTemplate]
		if !filterTestQualityWanted(&conf, &nzbs.Arr[entryidx], dl) {
			conf.Close()
			continue
		}
		conf.Close()
		//check priority
		if nzbs.Arr[entryidx].ParseInfo.Priority == 0 {
			denynzb("Prio unknown", &nzbs.Arr[entryidx], dl)
			continue
		}

		if nzbs.Arr[entryidx].MinimumPriority == nzbs.Arr[entryidx].ParseInfo.Priority {
			denynzb("Prio same", &nzbs.Arr[entryidx], dl, nzbs.Arr[entryidx].MinimumPriority)
			continue
		}
		if nzbs.Arr[entryidx].MinimumPriority != 0 && config.Cfg.Quality[nzbs.Arr[entryidx].QualityTemplate].UseForPriorityMinDifference == 0 && nzbs.Arr[entryidx].ParseInfo.Priority <= nzbs.Arr[entryidx].MinimumPriority {
			denynzb("Prio lower", &nzbs.Arr[entryidx], dl, nzbs.Arr[entryidx].MinimumPriority)
			continue
		}
		if nzbs.Arr[entryidx].MinimumPriority != 0 && config.Cfg.Quality[nzbs.Arr[entryidx].QualityTemplate].UseForPriorityMinDifference != 0 && (config.Cfg.Quality[nzbs.Arr[entryidx].QualityTemplate].UseForPriorityMinDifference+nzbs.Arr[entryidx].ParseInfo.Priority) <= nzbs.Arr[entryidx].MinimumPriority {
			denynzb("Prio lower", &nzbs.Arr[entryidx], dl, nzbs.Arr[entryidx].MinimumPriority)
			continue
		}

		if s.checkyear(&nzbs.Arr[entryidx], dl) {
			continue
		}

		if Checktitle(&nzbs.Arr[entryidx], s.SearchGroupType, dl) {
			continue
		}
		if s.checkepisode(&nzbs.Arr[entryidx], dl) {
			continue
		}
		logger.Log.GlobalLogger.Debug("Release ok", zap.Intp("minimum prio", &nzbs.Arr[entryidx].MinimumPriority), zap.Intp("prio", &nzbs.Arr[entryidx].ParseInfo.Priority), zap.Stringp("quality", &nzbs.Arr[entryidx].QualityTemplate), zap.Stringp("title", &nzbs.Arr[entryidx].NZB.Title))

		dl.mu.Lock()
		dl.Nzbs = append(dl.Nzbs, nzbs.Arr[entryidx])
		historytitlecache.Value = append(historytitlecache.Value.([]string), nzbs.Arr[entryidx].NZB.Title)
		historyurlcache.Value = append(historyurlcache.Value.([]string), nzbs.Arr[entryidx].NZB.DownloadURL)
		dl.mu.Unlock()
		// index.Close()
	}
	historytitlecache = nil
	historyurlcache = nil
}

func allowMovieImport(imdb string, cfgp *config.MediaTypeConfig, listname string) bool {
	if !config.Check("list_" + cfgp.ListsMap[listname].TemplateList) {
		return false
	}
	var countergenre int

	if config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinVotes != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and num_votes < ?", Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinVotes}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error vote count too low for ", zap.Stringp("imdb", &imdb))
			return false
		}
	}
	if config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinRating != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and average_rating < ?", Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinRating}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error average vote too low for ", zap.Stringp("imdb", &imdb))
			return false
		}
	}
	var excludebygenre bool
	countimdb := "select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE"
	for idxgenre := range config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre[idxgenre]}})
		if countergenre >= 1 {
			excludebygenre = true
			logger.Log.GlobalLogger.Debug("error excluded genre ", zap.Stringp("excluded", &config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre[idxgenre]), zap.Stringp("imdb", &imdb))
			break
		}
	}
	if excludebygenre {
		return false
	}
	var includebygenre bool
	for idxgenre := range config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre {
		countergenre, _ = database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre[idxgenre]}})
		if countergenre >= 1 {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre) >= 1 {
		logger.Log.GlobalLogger.Debug("error included genre not found ", zap.Stringp("imdb", &imdb))
		return false
	}
	return true
}

func GetHighestMoviePriorityByFiles(useall bool, checkwanted bool, movieid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) (minPrio int) {
	var foundfiles []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: querymoviefiles, Args: []interface{}{movieid}}, &foundfiles)

	var prio int
	for idx := range foundfiles {
		prio = GetMovieDBPriorityByID(useall, checkwanted, foundfiles[idx], cfgp, qualityTemplate)
		if prio == 0 && checkwanted {
			prio = GetMovieDBPriorityByID(useall, false, foundfiles[idx], cfgp, qualityTemplate)
		}
		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
	}
	foundfiles = nil
	return minPrio
}

func GetHighestEpisodePriorityByFiles(useall bool, checkwanted bool, episodeid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	var foundfiles []uint
	database.QueryStaticUintArray(0, &database.Querywithargs{QueryString: queryseriefiles, Args: []interface{}{episodeid}}, &foundfiles)

	var prio, minPrio int
	for idx := range foundfiles {
		prio = GetSerieDBPriorityByID(useall, checkwanted, foundfiles[idx], cfgp, qualityTemplate)
		if prio == 0 && checkwanted {
			prio = GetSerieDBPriorityByID(useall, false, foundfiles[idx], cfgp, qualityTemplate)
		}
		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
	}
	foundfiles = nil
	return minPrio
}

func GetSerieDBPriorityByID(useall bool, checkwanted bool, episodefileid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	var serieepisodefile database.SerieEpisodeFile
	if database.GetSerieEpisodeFiles(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodefileid}}, &serieepisodefile) != nil {
		return 0
	}
	m := apiexternal.ParseInfo{
		ResolutionID: serieepisodefile.ResolutionID,
		QualityID:    serieepisodefile.QualityID,
		CodecID:      serieepisodefile.CodecID,
		AudioID:      serieepisodefile.AudioID,
		Proper:       serieepisodefile.Proper,
		Extended:     serieepisodefile.Extended,
		Repack:       serieepisodefile.Repack,
		Title:        serieepisodefile.Filename,
		File:         serieepisodefile.Location,
	}

	parser.GetIDPriorityMap(&m, cfgp, qualityTemplate, useall, checkwanted)
	prio := m.Priority
	m.Close()
	return prio
}

func GetMovieDBPriorityByID(useall bool, checkwanted bool, moviefileid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	var moviefile database.MovieFile
	if database.GetMovieFiles(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{moviefileid}}, &moviefile) != nil {
		return 0
	}
	m := apiexternal.ParseInfo{
		ResolutionID: moviefile.ResolutionID,
		QualityID:    moviefile.QualityID,
		CodecID:      moviefile.CodecID,
		AudioID:      moviefile.AudioID,
		Proper:       moviefile.Proper,
		Extended:     moviefile.Extended,
		Repack:       moviefile.Repack,
		Title:        moviefile.Filename,
		File:         moviefile.Location,
	}

	parser.GetIDPriorityMap(&m, cfgp, qualityTemplate, useall, checkwanted)
	prio := m.Priority
	m.Close()
	return prio
}
