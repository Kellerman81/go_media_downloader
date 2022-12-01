package searcher

import (
	"database/sql"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"go.uber.org/zap"
)

const breakindexerloop string = "Break Indexer loop - entry found"
const spacenotfound string = " not found"
const notwantedmovie string = "Not Wanted Movie"
const skippedindexer string = "Skipped Indexer"
const noresults string = "no results"

func SearchMovieMissing(cfgp *config.MediaTypeConfig, jobcount int, titlesearch bool) {

	var scaninterval int
	var scandatepre int

	if len(cfgp.Data) >= 1 {
		if !config.ConfigCheck("path_" + cfgp.Data[0].TemplatePath) {
			return
		}
		scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanInterval
		scandatepre = config.Cfg.Paths[cfgp.Data[0].TemplatePath].MissingScanReleaseDatePre
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	qu.Select = "movies.id as num"
	qu.InnerJoin = "dbmovies on dbmovies.id=movies.dbmovie_id"
	qu.OrderBy = "movies.Lastscan asc"
	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (movies.lastscan is null or movies.Lastscan < ?)"
		whereArgs[len(cfgp.Lists)] = scantime
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			whereArgs[len(cfgp.Lists)+1] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		qu.Where = "movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			whereArgs[len(cfgp.Lists)] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfgp, "movies", titlesearch, database.Querywithargs{Query: qu, Args: whereArgs})
}

//	func downloadMovieSearchResults(first bool, movieid uint, cfgp *config.MediaTypeConfig, searchtype string, searchresults *SearchResults) {
//		if len(searchresults.Nzbs) >= 1 {
//			logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("title", searchresults.Nzbs[0].NZB.Title))
//			downloadnow := downloader.NewDownloader(cfgp)
//			downloadnow.SetMovie(movieid)
//			downloadnow.Nzb = searchresults.Nzbs[0]
//			downloadnow.DownloadNzb()
//			downloadnow.Close()
//		}
//	}
func SearchMovieSingle(movieid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	quality, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select quality_profile from movies where id = ?", Args: []interface{}{movieid}})
	searchstuff(cfgp, quality, "movie", searchstruct{mediatype: "movie", movieid: movieid, forceDownload: false, titlesearch: titlesearch})
}

func SearchMovieUpgrade(cfgp *config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int

	if len(cfgp.Data) >= 1 {
		if !config.ConfigCheck("path_" + cfgp.Data[0].TemplatePath) {
			return
		}
		scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (lastscan is null or Lastscan < ?)"
		whereArgs[len(cfgp.Lists)] = scantime
	} else {
		qu.Where = "quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ")"
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.Select = "id as num"
	qu.OrderBy = "Lastscan asc"

	searchlist(cfgp, "movies", titlesearch, database.Querywithargs{Query: qu, Args: whereArgs})
}

func SearchSerieSingle(serieid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	episodes := database.QueryStaticIntArray(0, database.Querywithargs{QueryString: "select id as num from serie_episodes where serie_id = ?", Args: []interface{}{serieid}})
	if len(episodes) >= 1 {
		for idx := range episodes {
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(uint(episodes[idx]), cfgp, titlesearch)
			//})
		}
	}
	episodes = nil
}

func SearchSerieSeasonSingle(serieid uint, season string, cfgp *config.MediaTypeConfig, titlesearch bool) {
	episodes := database.QueryStaticIntArray(0, database.Querywithargs{QueryString: "select id as num from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", Args: []interface{}{serieid, season}})
	if len(episodes) >= 1 {
		for idx := range episodes {
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(uint(episodes[idx]), cfgp, titlesearch)
			//})
		}
	}
	episodes = nil
}

func SearchSerieRSSSeasonSingle(serieid uint, season int, useseason bool, cfgp *config.MediaTypeConfig) {
	qualstr, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select quality_profile from serie_episodes where serie_id = ?", Args: []interface{}{serieid}})
	if qualstr == "" {
		return
	}
	dbserieid, _ := database.QueryColumnUint(database.Querywithargs{QueryString: "select dbserie_id from series where id = ?", Args: []interface{}{serieid}})
	tvdb, err := database.QueryColumnUint(database.Querywithargs{QueryString: "select thetvdb_id from dbseries where id = ?", Args: []interface{}{dbserieid}})
	if err != nil {
		return
	}
	SearchSerieRSSSeason(cfgp, qualstr, int(tvdb), season, useseason)
}
func SearchSeriesRSSSeasons(cfgp *config.MediaTypeConfig) {
	argcount := len(cfgp.Lists)
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	series, _ := database.QueryStaticColumnsTwoInt(database.Querywithargs{QueryString: "select id as num1, dbserie_id as num2 from series where listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != 0 and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1", Args: whereArgs})
	if len(series) >= 1 {
		var seasons []string
		var seasonint int
		var err error
		queryseason := "select distinct season from dbserie_episodes where dbserie_id = ?"
		for idx := range series {
			seasons = database.QueryStaticStringArray(false, 10, database.Querywithargs{QueryString: queryseason, Args: []interface{}{series[idx].Num2}})
			for idxseason := range seasons {
				if seasons[idxseason] == "" {
					continue
				}
				seasonint, err = strconv.Atoi(seasons[idxseason])
				if err == nil {
					//workergroup.Submit(func() {
					SearchSerieRSSSeasonSingle(uint(series[idx].Num1), seasonint, true, cfgp)
					//})
				}
			}
		}
		seasons = nil
	}
	series = nil
	whereArgs = nil
}
func SearchSerieEpisodeSingle(episodeid uint, cfgp *config.MediaTypeConfig, titlesearch bool) {
	quality, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select quality_profile from serie_episodes where id = ?", Args: []interface{}{episodeid}})
	searchstuff(cfgp, quality, "series", searchstruct{mediatype: "series", episodeid: episodeid, forceDownload: false, titlesearch: titlesearch})
}
func SearchSerieMissing(cfgp *config.MediaTypeConfig, jobcount int, titlesearch bool) {
	rowdata := cfgp.Data[0]

	var scaninterval int
	var scandatepre int

	if !config.ConfigCheck("path_" + rowdata.TemplatePath) {
		return
	}
	scaninterval = config.Cfg.Paths[rowdata.TemplatePath].MissingScanInterval
	scandatepre = config.Cfg.Paths[rowdata.TemplatePath].MissingScanReleaseDatePre

	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
		logger.Log.GlobalLogger.Debug("Search before scan", zap.Time("Time", scantime))
	}

	var qu database.Query
	qu.Select = "serie_episodes.id as num"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
		whereArgs[len(cfgp.Lists)] = scantime
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			whereArgs[len(cfgp.Lists)+1] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		qu.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			whereArgs[len(cfgp.Lists)] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfgp, "serie_episodes", titlesearch, database.Querywithargs{Query: qu, Args: whereArgs})
}

func SearchSerieUpgrade(cfgp *config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int

	if len(cfgp.Data) >= 1 {
		if !config.ConfigCheck("path_" + cfgp.Data[0].TemplatePath) {
			return
		}
		scaninterval = config.Cfg.Paths[cfgp.Data[0].TemplatePath].UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	qu.Select = "serie_episodes.ID as num"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.ID=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

	argcount := len(cfgp.Lists)
	if scaninterval != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range cfgp.Lists {
		whereArgs[i] = cfgp.Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
		whereArgs[len(cfgp.Lists)] = scantime
	} else {
		qu.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(cfgp.Lists)-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfgp, "serie_episodes", titlesearch, database.Querywithargs{Query: qu, Args: whereArgs})
	whereArgs = nil
}
func searchlist(cfgp *config.MediaTypeConfig, table string, titlesearch bool, qu database.Querywithargs) {
	missingepisode, _ := database.QueryStaticColumnsOneIntQueryObject(table, qu)
	var typemovies bool
	if len(missingepisode) >= 1 {
		typemovies = strings.HasPrefix(cfgp.NamePrefix, "movie_")
		for idx := range missingepisode {
			if typemovies {
				//workergroup.Submit(func() {
				SearchMovieSingle(uint(missingepisode[idx].Num), cfgp, titlesearch)
				//})
			} else {
				//workergroup.Submit(func() {
				SearchSerieEpisodeSingle(uint(missingepisode[idx].Num), cfgp, titlesearch)
				//})
			}
		}
	}
	missingepisode = nil
}

func SearchSerieRSS(cfgp *config.MediaTypeConfig, quality string) {
	logger.Log.GlobalLogger.Debug("Get Rss Series List")

	searchrssnow(cfgp, quality, "series")
}

func searchrssnow(cfgp *config.MediaTypeConfig, quality string, mediatype string) {
	searchstuff(cfgp, quality, "rss", searchstruct{mediatype: mediatype})
}

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

func searchstuff(cfgp *config.MediaTypeConfig, quality string, searchtype string, data searchstruct) {
	searchnow := NewSearcher(cfgp, quality)
	defer searchnow.Close()
	var results *SearchResults
	var err error
	switch searchtype {
	case "rss":
		results, err = searchnow.SearchRSS(data.mediatype, false)
	case "rssseason":
		results, err = searchnow.SearchSeriesRSSSeason(data.mediatype, data.thetvdbid, data.season, data.useseason)
	case "movie":
		results, err = searchnow.MovieSearch(data.movieid, false, data.titlesearch)
	case "series":
		results, err = searchnow.SeriesSearch(data.episodeid, false, data.titlesearch)
	}

	if err == nil {
		downloadNzb(results, cfgp, data.mediatype)

		defer results.Close()
	}
}

func SearchSerieRSSSeason(cfgp *config.MediaTypeConfig, quality string, thetvdbid int, season int, useseason bool) {
	searchstuff(cfgp, quality, "rssseason", searchstruct{mediatype: "series", thetvdbid: thetvdbid, season: season, useseason: useseason})
}

func downloadNzb(searchresults *SearchResults, cfgp *config.MediaTypeConfig, mediatype string) {
	var downloaded []uint = make([]uint, 0, len(searchresults.Nzbs))
	var breakfor bool
	var downloadnow *downloader.Downloadertype
	for idx := range searchresults.Nzbs {
		breakfor = false
		for idxs := range downloaded {
			if mediatype == "movie" {
				if downloaded[idxs] == searchresults.Nzbs[idx].NzbmovieID {
					breakfor = true
					break
				}
			} else {
				if downloaded[idxs] == searchresults.Nzbs[idx].NzbepisodeID {
					breakfor = true
					break
				}
			}
		}
		if breakfor {
			continue
		}
		logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("title", searchresults.Nzbs[idx].NZB.Title))

		downloadnow = downloader.NewDownloader(cfgp)

		if mediatype == "movie" {
			downloaded = append(downloaded, searchresults.Nzbs[idx].NzbmovieID)
			downloadnow.SetMovie(searchresults.Nzbs[idx].NzbmovieID)
		} else {
			downloaded = append(downloaded, searchresults.Nzbs[idx].NzbepisodeID)
			downloadnow.SetSeriesEpisode(searchresults.Nzbs[idx].NzbepisodeID)
		}
		downloadnow.Nzb = searchresults.Nzbs[idx]
		downloadnow.DownloadNzb()
		downloadnow.Close()
	}
}
func SearchMovieRSS(cfgp *config.MediaTypeConfig, quality string) {
	logger.Log.GlobalLogger.Debug("Get Rss Movie List")

	searchrssnow(cfgp, quality, "movie")
}

type SearchResults struct {
	Nzbs     []apiexternal.Nzbwithprio
	Rejected []apiexternal.Nzbwithprio
	//Searched []string
}

func (s *SearchResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		if len(s.Nzbs) >= 1 {
			for idx := range s.Nzbs {
				s.Nzbs[idx].Close()
			}
			s.Nzbs = nil
		}
		if len(s.Rejected) >= 1 {
			for idx := range s.Rejected {
				s.Rejected[idx].Close()
			}
			s.Rejected = nil
		}
		s = nil
	}
}

type Searcher struct {
	Cfgp             *config.MediaTypeConfig
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	MinimumPriority  int
	Movie            database.Movie
	SerieEpisode     database.SerieEpisode
	Dbmovie          database.Dbmovie
	Dbserieepisode   database.DbserieEpisode
	Serie            database.Serie
	Dbserie          database.Dbserie
}

func NewSearcher(cfgp *config.MediaTypeConfig, quality string) *Searcher {
	return &Searcher{
		Cfgp:    cfgp,
		Quality: quality,
	}
}

func (s *Searcher) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		s = nil
	}
}

var errNoQuality error = errors.New("quality not found")
var errNoList error = errors.New("list not found")
var errNoRegex error = errors.New("regex not found")
var errNoIndexer error = errors.New("indexer not found")
var errOther error = errors.New("other error")
var errIndexerDisabled error = errors.New("indexer disabled")
var errSearchDisabled error = errors.New("search disabled")
var errUpgradeDisabled error = errors.New("upgrade disabled")
var errToWait error = errors.New("please wait")

// searchGroupType == movie || series
func (s *Searcher) SearchRSS(searchGroupType string, fetchall bool) (*SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found", zap.String("Searched: ", s.Quality))
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	processedindexer := 0
	dl := s.rsssearchindexerloop(&processedindexer, fetchall)

	if processedindexer == 0 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}

	if dl == nil {
		return nil, errors.New(noresults)
	}
	if dl != nil {
		if len(dl.Nzbs) == 0 {
			logger.Log.GlobalLogger.Info("No new entries found")
		}
	}
	return dl, nil
}

func (s *Searcher) rsssearchindexerloop(processedindexer *int, fetchall bool) *SearchResults {
	return s.queryindexers(s.Quality, "rss", fetchall, processedindexer, false, queryparams{})
}

func (t *Searcher) rsssearchindexer(search *searchstruct, fetchall bool, dl *SearchResults) bool {
	// if addsearched(dl, search.indexer+search.quality) {
	// 	return true
	// }
	var nzbindexer apiexternal.NzbIndexer
	defer nzbindexer.Close()
	cats, erri := t.initIndexer(search, "rss", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			return true
		}
		if erri == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.String("indexer", search.indexer), zap.Error(erri))
		return false
	}

	if fetchall {
		nzbindexer.LastRssId = ""
	}
	idxcfg := config.Cfg.Indexers[search.indexer]
	maxentries := idxcfg.MaxRssEntries
	maxloop := idxcfg.RssEntriesloop
	if maxentries == 0 {
		maxentries = 10
	}
	if maxloop == 0 {
		maxloop = 2
	}

	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&nzbindexer, maxentries, cats, maxloop)

	if nzberr != nil {

		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.String("indexer", search.indexer), zap.Error(nzberr))
		failedindexer(failed)
		return false
	} else {
		defer nzbs.Close()
		if !fetchall {
			if lastids != "" && len((nzbs.Arr)) >= 1 {
				addrsshistory(nzbindexer.URL, lastids, t.Quality, t.Cfgp.NamePrefix)
			}
		}
		//logger.Log.GlobalLogger.Debug("Search RSS ended - found entries", zap.Int("entries", len((nzbs.Arr))))
		t.parseentries(nzbs, dl, search, "", false)
	}
	return true
}

// searchGroupType == movie || series
func (s *Searcher) SearchSeriesRSSSeason(searchGroupType string, thetvdbid int, season int, useseason bool) (*SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	processedindexer := 0
	dl := s.rssqueryseriesindexerloop(&processedindexer, thetvdbid, season, useseason)
	if processedindexer == 0 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if dl == nil {
		return nil, errors.New(noresults)
	}
	return dl, nil
}

func (s *Searcher) rssqueryseriesindexerloop(processedindexer *int, thetvdbid int, season int, useseason bool) *SearchResults {
	return s.queryindexers(s.Quality, "rssserie", false, processedindexer, false, queryparams{thetvdbid, season, useseason})
}

func (t *Searcher) rssqueryseriesindexer(search *searchstruct, thetvdbid int, season int, useseason bool, dl *SearchResults) bool {

	// if addsearched(dl, search.indexer+search.quality+strconv.Itoa(thetvdbid)+strconv.Itoa(season)) {
	// 	return true
	// }
	var nzbindexer apiexternal.NzbIndexer
	defer nzbindexer.Close()
	cats, erri := t.initIndexer(search, "api", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			return true
		}
		if erri == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.String("indexer", search.indexer), zap.Error(erri))
		return false
	}

	nzbs, _, nzberr := apiexternal.QueryNewznabTvTvdb(&nzbindexer, thetvdbid, cats, season, 0, useseason, false)

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.String("indexer", search.indexer), zap.Error(nzberr))
		failedindexer(nzbindexer.URL)
		return false
	} else {
		defer nzbs.Close()
		t.parseentries(nzbs, dl, search, "", false)
	}
	return true
}

func (s *Searcher) checkhistory(search *searchstruct, entry apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Check History
	historytable := "serie_episode_histories"
	if strings.EqualFold(s.SearchGroupType, "movie") {
		historytable = "movie_histories"
	}
	found := logger.GlobalCache.CheckNoType(historytable + "_url")
	if !found {
		logger.GlobalCache.Set(historytable+"_url", database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: "select url from " + historytable}), 8*time.Hour)
	}
	historycache := logger.GlobalCache.GetData(historytable + "_url")
	defer historycache.Close()
	if len(entry.NZB.DownloadURL) > 1 {
		for idx := range historycache.Value.([]string) {
			if strings.EqualFold(historycache.Value.([]string)[idx], entry.NZB.DownloadURL) {
				denynzb("Already downloaded (Url)", entry, dl)
				return true
			}
		}
	}
	if config.QualityIndexerByQualityAndTemplateGetFieldBool(search.quality, search.indexer, "HistoryCheckTitle") && len(entry.NZB.Title) > 1 {
		found = logger.GlobalCache.CheckNoType(historytable + "_title")
		if !found {
			logger.GlobalCache.Set(historytable+"_title", database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: "select title from " + historytable}), 8*time.Hour)
		}
		historycache = logger.GlobalCache.GetData(historytable + "_title")
		for idx := range historycache.Value.([]string) {
			if strings.EqualFold(historycache.Value.([]string)[idx], entry.NZB.Title) {
				denynzb("Already downloaded (Title)", entry, dl)
				return true
			}
		}
	}
	return false
}
func (s *Searcher) checkcorrectid(entry apiexternal.Nzbwithprio, dl *SearchResults) bool {

	if s.SearchActionType != "rss" {
		if strings.EqualFold(s.SearchGroupType, "movie") {
			if entry.NZB.IMDBID != "" {
				//Check Correct Imdb
				tempimdb := strings.TrimPrefix(entry.NZB.IMDBID, "tt")
				tempimdb = strings.TrimLeft(tempimdb, "0")

				wantedimdb := strings.TrimPrefix(s.Dbmovie.ImdbID, "tt")
				wantedimdb = strings.TrimLeft(wantedimdb, "0")
				if wantedimdb != tempimdb && wantedimdb != "" && tempimdb != "" {
					denynzb("Imdb not match", entry, dl)
					return true
				}
			}
		} else {
			//Check TVDB Id
			if s.Dbserie.ThetvdbID >= 1 && entry.NZB.TVDBID != "" {
				if strconv.Itoa(s.Dbserie.ThetvdbID) != entry.NZB.TVDBID {
					denynzb("Tvdbid not match", entry, dl)
					return true
				}
			}
		}
	}
	return false
}
func (s *Searcher) getmediadata(entry *apiexternal.Nzbwithprio, dl *SearchResults, addinlist string, addifnotfound bool) bool {

	if strings.EqualFold(s.SearchGroupType, "movie") {
		var dbmovieid uint
		if s.SearchActionType == "rss" {
			//Filter RSS Movies
			denied := s.getmovierss(entry, addinlist, addifnotfound, s.Cfgp, s.Quality, dl)
			if denied {
				return true
			}
			dbmovieid = s.Movie.DbmovieID
			entry.WantedTitle = s.Dbmovie.Title
			entry.QualityTemplate = s.Movie.QualityProfile

			//Check Minimal Priority
			entry.MinimumPriority = GetHighestMoviePriorityByFiles(entry.NzbmovieID, s.Cfgp, s.Movie.QualityProfile)

			if entry.MinimumPriority != 0 {
				if s.Movie.DontUpgrade {
					denynzb("Upgrade disabled", *entry, dl)
					return true
				}
			} else {
				if s.Movie.DontSearch {
					denynzb("Search disabled", *entry, dl)
					return true
				}
			}
		} else {
			dbmovieid = s.Dbmovie.ID
			entry.NzbmovieID = s.Movie.ID
			entry.QualityTemplate = s.Movie.QualityProfile
			entry.MinimumPriority = s.MinimumPriority
			entry.ParseInfo.MovieID = s.Movie.ID
			entry.ParseInfo.DbmovieID = s.Movie.DbmovieID
			entry.WantedTitle = s.Dbmovie.Title
		}

		//Check QualityProfile
		if !config.ConfigCheck("quality_" + entry.QualityTemplate) {
			denynzb("Unknown quality", *entry, dl)
			return true
		}

		entry.WantedAlternates = database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: "select title from dbmovie_titles where dbmovie_id = ?", Args: []interface{}{dbmovieid}})
	} else {
		if s.SearchActionType == "rss" {
			//Filter RSS Series
			denied := s.getserierss(entry, s.Cfgp, s.Quality, dl)
			if denied {
				return true
			}
			entry.QualityTemplate = s.SerieEpisode.QualityProfile
			entry.WantedTitle = s.Dbserie.Seriename

			//Check Minimum Priority
			entry.MinimumPriority = GetHighestEpisodePriorityByFiles(entry.NzbepisodeID, s.Cfgp, entry.QualityTemplate)

			if entry.MinimumPriority != 0 {
				if s.SerieEpisode.DontUpgrade {
					denynzb("Upgrade disabled", *entry, dl)
					return true
				}
			} else {
				if s.SerieEpisode.DontSearch {
					denynzb("Search disabled", *entry, dl)
					return true
				}
			}
		} else {
			entry.NzbepisodeID = s.SerieEpisode.ID
			entry.QualityTemplate = s.SerieEpisode.QualityProfile
			entry.MinimumPriority = s.MinimumPriority
			entry.ParseInfo.DbserieID = s.Dbserie.ID
			entry.ParseInfo.DbserieEpisodeID = s.Dbserieepisode.ID
			entry.ParseInfo.SerieEpisodeID = s.SerieEpisode.ID
			entry.ParseInfo.SerieID = s.Serie.ID
			entry.WantedTitle = s.Dbserie.Seriename
		}

		//Check Quality Profile
		if !config.ConfigCheck("quality_" + entry.QualityTemplate) {
			denynzb("Unknown Quality Profile", *entry, dl)
			return true
		}

		entry.WantedAlternates = database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{s.Dbserie.ID}})
	}
	return false
}
func (s *Searcher) checkyear(entry apiexternal.Nzbwithprio, dl *SearchResults) bool {
	checkyear := config.Cfg.Quality[entry.QualityTemplate].CheckYear

	checkyear1 := config.Cfg.Quality[entry.QualityTemplate].CheckYear1

	if strings.EqualFold(s.SearchGroupType, "movie") && s.SearchActionType != "rss" {
		yearstr := strconv.Itoa(s.Dbmovie.Year)
		if yearstr == "0" {
			denynzb("No Year", entry, dl)
			return true
		}
		if checkyear && !checkyear1 && !strings.Contains(entry.NZB.Title, yearstr) {
			denynzb("Unwanted Year", entry, dl)
			return true
		} else {
			if checkyear1 {
				if !strings.Contains(entry.NZB.Title, strconv.Itoa(s.Dbmovie.Year+1)) && !strings.Contains(entry.NZB.Title, strconv.Itoa(s.Dbmovie.Year-1)) && !strings.Contains(entry.NZB.Title, strconv.Itoa(s.Dbmovie.Year)) {
					denynzb("Unwanted Year1", entry, dl)
					return true
				}
			}
		}
	}
	return false
}
func Checktitle(entry apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Checktitle
	if config.Cfg.Quality[entry.QualityTemplate].CheckTitle {
		yearstr := strconv.Itoa(entry.ParseInfo.Year)
		lentitle := len(entry.WantedTitle)
		titlefound := false
		if entry.WantedTitle != "" {
			if config.Cfg.Quality[entry.QualityTemplate].CheckTitle && apiexternal.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title) && lentitle >= 1 {
				titlefound = true
			}
			if !titlefound && entry.ParseInfo.Year != 0 {
				if config.Cfg.Quality[entry.QualityTemplate].CheckTitle && apiexternal.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title+" "+yearstr) && lentitle >= 1 {
					titlefound = true
				}
			}
		}
		lenalt := len(entry.WantedAlternates)
		if !titlefound && entry.ParseInfo.Title != "" {
			alttitlefound := false
			for idxtitle := range entry.WantedAlternates {
				if entry.WantedAlternates[idxtitle] == "" {
					continue
				}
				if apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], entry.ParseInfo.Title) {
					alttitlefound = true
					break
				}

				if entry.ParseInfo.Year != 0 {
					if config.Cfg.Quality[entry.QualityTemplate].CheckTitle && apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], entry.ParseInfo.Title+" "+yearstr) {
						alttitlefound = true
						break
					}
				}
			}
			if lenalt >= 1 && !alttitlefound {
				denynzb("Unwanted Title and Alternate", entry, dl)
				return true
			}
		}
		if lenalt == 0 && !titlefound {
			denynzb("Unwanted Title", entry, dl)
			return true
		}
	}
	return false
}
func (s *Searcher) checkepisode(entry apiexternal.Nzbwithprio, dl *SearchResults) bool {

	//Checkepisode
	if !strings.EqualFold(s.SearchGroupType, "movie") {

		// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
		matchfound := false

		lowerIdentifier := strings.ToLower(s.Dbserieepisode.Identifier)
		lowerParseIdentifier := strings.ToLower(entry.ParseInfo.Identifier)
		lowerTitle := strings.ToLower(entry.NZB.Title)
		altIdentifier := strings.TrimLeft(lowerIdentifier, "s0")
		altIdentifier = strings.Replace(altIdentifier, "e", "x", -1)
		if strings.Contains(lowerTitle, lowerIdentifier) ||
			strings.Contains(lowerTitle, strings.Replace(lowerIdentifier, "-", ".", -1)) ||
			strings.Contains(lowerTitle, strings.Replace(lowerIdentifier, "-", " ", -1)) ||
			strings.Contains(lowerTitle, altIdentifier) ||
			strings.Contains(lowerTitle, strings.Replace(altIdentifier, "-", ".", -1)) ||
			strings.Contains(lowerTitle, strings.Replace(altIdentifier, "-", " ", -1)) {

			matchfound = true
		} else {
			seasonarray := []string{"s" + s.Dbserieepisode.Season + "e", "s0" + s.Dbserieepisode.Season + "e", "s" + s.Dbserieepisode.Season + " e", "s0" + s.Dbserieepisode.Season + " e", s.Dbserieepisode.Season + "x", s.Dbserieepisode.Season + " x"}
			episodearray := []string{"e" + s.Dbserieepisode.Episode, "e0" + s.Dbserieepisode.Episode, "x" + s.Dbserieepisode.Episode, "x0" + s.Dbserieepisode.Episode}
			for idxseason := range seasonarray {
				if strings.HasPrefix(lowerParseIdentifier, seasonarray[idxseason]) {
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
			}
		}
		if !matchfound {
			denynzb("identifier not match "+s.Dbserieepisode.Identifier, entry, dl)
			return true
		}
	}
	return false
}

const deniedbyregex string = "Denied by Regex: "

func (s *Searcher) convertnzbs(search *searchstruct, entries *apiexternal.NZBArr, addinlist string, addifnotfound bool, dl *SearchResults) {
	var index *config.QualityIndexerConfig
	var regexdeny, parsefile, includeyear bool
	var regexrule string
	var err error
	var conf config.QualityConfig

	for entryidx := range entries.Arr {
		entries.Arr[entryidx].Indexer = search.indexer
		conf = config.Cfg.Quality[entries.Arr[entryidx].QualityTemplate]

		//Check Title Length
		if entries.Arr[entryidx].NZB.DownloadURL == "" {
			denynzb("No Url", entries.Arr[entryidx], dl)
			return
		}
		if entries.Arr[entryidx].NZB.Title == "" {
			denynzb("No Title", entries.Arr[entryidx], dl)
			return
		}
		if len(strings.Trim(entries.Arr[entryidx].NZB.Title, " ")) <= 3 {
			denynzb("Title too short", entries.Arr[entryidx], dl)
			return
		}
		for idx := range dl.Rejected {
			if dl.Rejected[idx].NZB.DownloadURL == entries.Arr[entryidx].NZB.DownloadURL {
				return
			}
		}
		for idx := range dl.Nzbs {
			if dl.Nzbs[idx].NZB.DownloadURL == entries.Arr[entryidx].NZB.DownloadURL {
				return
			}
		}

		//Check Size
		index = config.QualityIndexerByQualityAndTemplate(search.quality, entries.Arr[entryidx].Indexer)
		if index.TemplateRegex == "" {
			denynzb("No Indexer Regex Template", entries.Arr[entryidx], dl)
			return
		}
		if index.FilterSizeNzbs(s.Cfgp, entries.Arr[entryidx].NZB.Title, entries.Arr[entryidx].NZB.Size) {
			denynzb("Wrong size", entries.Arr[entryidx], dl)
			return
		}

		if s.checkhistory(search, entries.Arr[entryidx], dl) {
			return
		}

		if s.checkcorrectid(entries.Arr[entryidx], dl) {
			return
		}

		//Regex
		regexdeny, regexrule = entries.Arr[entryidx].FilterRegexNzbs(index.TemplateRegex, entries.Arr[entryidx].NZB.Title)
		if regexdeny {
			denynzb(deniedbyregex+regexrule, entries.Arr[entryidx], dl)
			return
		}

		//Parse
		parsefile = entries.Arr[entryidx].ParseInfo.File == ""
		if entries.Arr[entryidx].ParseInfo.File != "" {
			if entries.Arr[entryidx].ParseInfo.Title == "" || entries.Arr[entryidx].ParseInfo.Resolution == "" || entries.Arr[entryidx].ParseInfo.Quality == "" {
				parsefile = true
			}
		}
		if parsefile {
			includeyear = false
			if s.SearchGroupType == "series" {
				includeyear = true
			}
			entries.Arr[entryidx].ParseInfo, err = parser.NewFileParserNoPt(entries.Arr[entryidx].NZB.Title, includeyear, s.SearchGroupType)
			if err != nil {
				denynzb("Error parsing", entries.Arr[entryidx], dl)
				return
			}
		}

		if s.getmediadata(&entries.Arr[entryidx], dl, addinlist, addifnotfound) {
			return
		}

		if entries.Arr[entryidx].ParseInfo.Priority == 0 {
			parser.GetPriorityMap(&entries.Arr[entryidx].ParseInfo, s.Cfgp, entries.Arr[entryidx].QualityTemplate, false, true)
			entries.Arr[entryidx].Prio = entries.Arr[entryidx].ParseInfo.Priority
		}

		entries.Arr[entryidx].ParseInfo.Title = importfeed.StripTitlePrefixPostfix(entries.Arr[entryidx].ParseInfo.Title, entries.Arr[entryidx].QualityTemplate)

		if s.checkyear(entries.Arr[entryidx], dl) {
			return
		}
		if Checktitle(entries.Arr[entryidx], dl) {
			return
		}

		if s.checkepisode(entries.Arr[entryidx], dl) {
			return
		}

		//check quality
		if !filterTestQualityWanted(&entries.Arr[entryidx].ParseInfo, entries.Arr[entryidx].QualityTemplate, &conf, entries.Arr[entryidx].NZB.Title) {
			denynzb("Unwanted Quality", entries.Arr[entryidx], dl)
			return
		}

		//check priority
		if entries.Arr[entryidx].ParseInfo.Priority != 0 {
			if entries.Arr[entryidx].MinimumPriority != 0 {
				if config.Cfg.Quality[entries.Arr[entryidx].QualityTemplate].UseForPriorityMinDifference != 0 {
					if (config.Cfg.Quality[entries.Arr[entryidx].QualityTemplate].UseForPriorityMinDifference + entries.Arr[entryidx].ParseInfo.Priority) <= entries.Arr[entryidx].MinimumPriority {
						denynzb("Prio lower. have: "+strconv.Itoa(entries.Arr[entryidx].MinimumPriority), entries.Arr[entryidx], dl)
						return
					}
				} else if entries.Arr[entryidx].ParseInfo.Priority <= entries.Arr[entryidx].MinimumPriority {
					denynzb("Prio lower. have: "+strconv.Itoa(entries.Arr[entryidx].MinimumPriority), entries.Arr[entryidx], dl)
					return
				}
			}
		} else {
			denynzb("Prio unknown", entries.Arr[entryidx], dl)
			return
		}
		dl.Nzbs = append(dl.Nzbs, entries.Arr[entryidx])
	}
	index.Close()
}

func filterTestQualityWanted(m *apiexternal.ParseInfo, qualityTemplate string, quality *config.QualityConfig, title string) bool {
	wantedReleaseResolution := false
	lenqual := len(quality.WantedResolutionIn.Arr)
	if lenqual >= 1 && m.Resolution != "" {
		if logger.InStringArray(m.Resolution, &quality.WantedResolutionIn) {
			wantedReleaseResolution = true
		}
	}

	if lenqual >= 1 && !wantedReleaseResolution {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted resolution", zap.String("title", title), zap.String("Quality", qualityTemplate), zap.String("Resolution", m.Resolution))
		return false
	}
	wantedReleaseQuality := false

	lenqual = len(quality.WantedQualityIn.Arr)
	if lenqual >= 1 && m.Quality != "" {
		if logger.InStringArray(m.Quality, &quality.WantedQualityIn) {
			wantedReleaseQuality = true
		}
	}
	if lenqual >= 1 && !wantedReleaseQuality {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted quality", zap.String("title", title), zap.String("Wanted Quality", qualityTemplate), zap.String("Quality", m.Quality))
		return false
	}

	wantedReleaseAudio := false

	lenqual = len(quality.WantedAudioIn.Arr)
	if lenqual >= 1 && m.Audio != "" {
		if logger.InStringArray(m.Audio, &quality.WantedAudioIn) {
			wantedReleaseAudio = true
		}
	}
	if lenqual >= 1 && !wantedReleaseAudio {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted audio", zap.String("title", title), zap.String("Quality", qualityTemplate))
		return false
	}
	wantedReleaseCodec := false

	lenqual = len(quality.WantedCodecIn.Arr)
	if lenqual >= 1 && m.Codec != "" {
		if logger.InStringArray(m.Codec, &quality.WantedCodecIn) {
			wantedReleaseCodec = true
		}
	}
	if lenqual >= 1 && !wantedReleaseCodec {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted codec", zap.String("title", title), zap.String("Quality", qualityTemplate))
		return false
	}
	return true
}

func insertmovie(imdb string, cfgp *config.MediaTypeConfig, addinlist string) (uint, error) {
	importfeed.JobImportMovies(imdb, cfgp, addinlist, true)
	founddbmovie, founddbmovieerr := database.QueryColumnUint(database.Querywithargs{QueryString: "select id from dbmovies where imdb_id = ?", Args: []interface{}{imdb}})
	return founddbmovie, founddbmovieerr
}

func (s *Searcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlist string, addifnotfound bool, cfgp *config.MediaTypeConfig, qualityTemplate string, dl *SearchResults) bool {
	//Parse
	var err error
	parser.GetDbIDs("movie", &entry.ParseInfo, cfgp, addinlist, true)
	loopdbmovie := entry.ParseInfo.DbmovieID
	loopmovie := entry.ParseInfo.MovieID
	//Get DbMovie by imdbid

	//Add DbMovie if not found yet and enabled
	if loopdbmovie == 0 {
		if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") {
			if !allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
				denynzb("Not Allowed Movie", *entry, dl)
				return true
			}
			var err2 error
			loopdbmovie, err2 = insertmovie(entry.NZB.IMDBID, cfgp, addinlist)
			loopmovie, err = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", Args: []interface{}{loopdbmovie, addinlist}})
			if err != nil || err2 != nil {
				denynzb(notwantedmovie, *entry, dl)
				return true
			}
		} else {
			denynzb("Not Wanted DBMovie", *entry, dl)
			return true
		}
	}

	//continue only if dbmovie found
	if loopdbmovie != 0 {
		//Get List of movie by dbmovieid, year and possible lists

		//if list was not found : should we add the movie?
		if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") && loopmovie == 0 {
			if !allowMovieImport(entry.NZB.IMDBID, cfgp, addinlist) {
				denynzb("Not Allowed Movie", *entry, dl)
				return true
			}
			loopdbmovie, _ = insertmovie(entry.NZB.IMDBID, cfgp, addinlist)
			loopmovie, _ = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", Args: []interface{}{loopdbmovie, addinlist}})
			if loopdbmovie == 0 || loopmovie == 0 {
				denynzb(notwantedmovie, *entry, dl)
				return true
			}
		} else {
			denynzb(notwantedmovie, *entry, dl)
			return true
		}
	}
	if loopmovie == 0 {
		denynzb(notwantedmovie, *entry, dl)
		return true
	}

	s.Movie, _ = database.GetMovies(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{loopmovie}})
	s.Dbmovie, _ = database.GetDbmovie(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{loopdbmovie}})

	entry.QualityTemplate = s.Movie.QualityProfile
	entry.ParseInfo.Title = importfeed.StripTitlePrefixPostfix(entry.ParseInfo.Title, entry.QualityTemplate)
	return false
}

func denynzb(reason string, entry apiexternal.Nzbwithprio, dl *SearchResults) {
	logger.Log.GlobalLogger.Debug("Skipped", zap.String("reason", reason), zap.String("title", entry.NZB.Title))
	entry.Denied = true
	entry.Reason = reason
	if dl != nil {
		dl.Rejected = append(dl.Rejected, entry)
	}
}
func (s *Searcher) getserierss(entry *apiexternal.Nzbwithprio, cfgp *config.MediaTypeConfig, qualityTemplate string, dl *SearchResults) bool {
	parser.GetDbIDs("series", &entry.ParseInfo, cfgp, "", true)
	loopdbseries := entry.ParseInfo.DbserieID

	if loopdbseries != 0 {
		if entry.ParseInfo.SerieID == 0 {
			denynzb("Unwanted Serie", *entry, dl)
			return true
		}
	} else {
		denynzb("Unwanted DBSerie", *entry, dl)
		return true
	}
	if entry.ParseInfo.DbserieEpisodeID == 0 {
		denynzb("Unwanted DB Episode", *entry, dl)
		return true
	}

	entry.NzbepisodeID = entry.ParseInfo.SerieEpisodeID
	if entry.NzbepisodeID == 0 {
		denynzb("Unwanted Episode", *entry, dl)
		return true
	}

	s.SerieEpisode, _ = database.GetSerieEpisodes(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{entry.NzbepisodeID}})
	s.Serie, _ = database.GetSeries(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.SerieID}})
	s.Dbserie, _ = database.GetDbserie(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.DbserieID}})
	s.Dbserieepisode, _ = database.GetDbserieEpisodes(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.DbserieEpisodeID}})
	entry.WantedAlternates = database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{loopdbseries}})
	return false
}

func (s *Searcher) GetRSSFeed(searchGroupType string, cfgp *config.MediaTypeConfig, listname string) (*SearchResults, error) {
	listTemplateList := cfgp.ListsMap[listname].TemplateList
	if listTemplateList == "" {
		return nil, errNoList
	}
	if !config.ConfigCheck("list_" + listTemplateList) {
		return nil, errNoList
	}
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	var cfgIndexer config.QualityIndexerConfig
	for idx := range config.Cfg.Quality[s.Quality].Indexer {
		if config.Cfg.Quality[s.Quality].Indexer[idx].TemplateIndexer == listTemplateList {
			cfgIndexer = config.Cfg.Quality[s.Quality].Indexer[idx]
			break
		}
	}
	if cfgIndexer.TemplateRegex == "" {
		return nil, errNoRegex
	}

	lastindexerid, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{listTemplateList, s.Quality, ""}})

	url := config.Cfg.Lists[listTemplateList].Url
	indexer := apiexternal.NzbIndexer{Name: listTemplateList, Customrssurl: url, LastRssId: lastindexerid}
	defer indexer.Close()
	blockinterval := -5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.Cfg.General.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic(database.Querywithargs{QueryString: "select count() from indexer_fails where indexer = ? and last_fail > ?", Args: []interface{}{url, time.Now().Add(time.Minute * time.Duration(blockinterval))}})
	if counter >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fail in the last ", zap.Int("Minutes", blockinterval), zap.String("Listname", listTemplateList))
		return nil, errIndexerDisabled
	}

	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&indexer, config.Cfg.Lists[listTemplateList].Limit, "", 1)

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed", zap.Error(nzberr))
		failedindexer(failed)
	} else {
		defer nzbs.Close()
		if lastids != "" && len((nzbs.Arr)) >= 1 {
			addrsshistory(indexer.URL, lastids, s.Quality, listTemplateList)
		}
		dl := new(SearchResults)

		s.parseentries(nzbs, dl, &searchstruct{quality: s.Quality, indexer: listTemplateList}, listname, cfgp.ListsMap[listname].Addfound)

		if len(dl.Nzbs) == 0 {
			logger.Log.GlobalLogger.Info("No new entries found")
		}
		return dl, nil
	}
	return nil, errOther
}

func addrsshistory(url string, lastid string, quality string, config string) {
	database.UpsertNamed("r_sshistories",
		&logger.InStringArrayStruct{Arr: []string{"indexer", "last_id", "list", "config"}},
		database.RSSHistory{Indexer: url, LastID: lastid, List: quality, Config: config},
		"config = :config COLLATE NOCASE and list = :list COLLATE NOCASE and indexer = :indexer COLLATE NOCASE",
		database.Querywithargs{Query: database.Query{Where: "config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE"}, Args: []interface{}{config, quality, url}})
}

func (s *Searcher) MovieSearch(movieid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	s.SearchGroupType = "movie"
	var err error
	s.Movie, err = database.GetMovies(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{movieid}})
	if err != nil {
		logger.Log.GlobalLogger.Debug("Skipped - movie not found")
		return nil, err
	}
	s.Dbmovie, _ = database.GetDbmovie(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.Movie.DbmovieID}})

	if s.Movie.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Skipped - Search disabled")
		return nil, errSearchDisabled
	}

	if s.Dbmovie.Year == 0 {
		//logger.Log.GlobalLogger.Debug("Skipped - No Year")
		return nil, errors.New("year not found")
	}

	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		logger.Log.GlobalLogger.Error("Quality for Movie: " + strconv.Itoa(int(movieid)) + spacenotfound)
		return nil, errNoQuality
	}
	s.Quality = s.Movie.QualityProfile
	s.MinimumPriority = GetHighestMoviePriorityByFiles(movieid, s.Cfgp, s.Movie.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if s.Movie.DontUpgrade && !forceDownload {
			logger.Log.GlobalLogger.Debug("Skipped - Upgrade disabled", zap.String("title", s.Dbmovie.Title))
			return nil, errUpgradeDisabled
		}
	}

	processedindexer := 0
	dl := s.mediasearchindexerloop(&processedindexer, titlesearch)

	if processedindexer == 0 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if processedindexer >= 1 {
		database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update movies set lastscan = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, movieid}})
	}

	if dl == nil {
		return nil, errors.New(noresults)
	}
	return dl, nil
}

func (s *Searcher) mediasearchindexerloop(processedindexer *int, titlesearch bool) *SearchResults {
	return s.queryindexers(s.Movie.QualityProfile, "movie", false, processedindexer, titlesearch, queryparams{})
}
func (t *Searcher) mediasearchindexer(search *searchstruct, dl *SearchResults) bool {
	var err error
	_, search.cats, err = t.initIndexerUrlCat(search, "api")
	if err != nil {
		if err == errIndexerDisabled || err == errNoIndexer {
			return true
		}
		if err == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.String("indexer", search.indexer), zap.Error(err))

		return false
	}
	searchfor := ""

	var qualityprofile, name, querytitle string

	var dbid uint
	if search.mediatype == "movie" {
		dbid = t.Dbmovie.ID
		qualityprofile = t.Movie.QualityProfile
		name = t.Dbmovie.Title
		querytitle = "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ? COLLATE NOCASE"
	}
	if search.mediatype == "serie" {

		dbid = t.Dbserieepisode.ID
		qualityprofile = t.SerieEpisode.QualityProfile
		name = t.Dbserie.Seriename
		querytitle = "select distinct title from dbserie_alternates where dbserie_id = ? and title != ? COLLATE NOCASE"
	}
	//logger.Log.Debug("Qualty: ", qualityprofile)

	releasefound := false
	if !config.ConfigCheck("quality_" + qualityprofile) {
		logger.Log.GlobalLogger.Error("Quality for: " + search.searchid + spacenotfound)
		return false
	}
	checkfirst := config.Cfg.Quality[qualityprofile].CheckUntilFirstFound
	checktitle := config.Cfg.Quality[qualityprofile].BackupSearchForTitle
	checkalttitle := config.Cfg.Quality[qualityprofile].BackupSearchForAlternateTitle

	//func (s *Searcher) searchMedia(mediatype string, searchid string, searchtitle bool, id uint, quality string, indexer string, title string, season int, episode int, cats string, dl *SearchResults) bool {
	if search.mediatype == "movie" && t.Dbmovie.ImdbID != "" {
		search.titlesearch = false
		search.title = ""
		t.searchMedia(search, dl)
	} else if search.mediatype == "serie" && t.Dbserie.ThetvdbID != 0 {
		search.titlesearch = false
		search.title = ""
		t.searchMedia(search, dl)
	}
	if len(dl.Nzbs) >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
		releasefound = true

		if checkfirst { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound {
			logger.Log.GlobalLogger.Debug(breakindexerloop, zap.String("title", name))
			return true
		}
	}
	if (!releasefound && checktitle) || (!releasefound && checktitle && search.titlesearch) { //config.Cfg.Quality[qualityprofile].BackupSearchForTitle
		searchfor = strings.Replace(name, "(", "", -1)
		searchfor = strings.Replace(searchfor, ")", "", -1)
		if search.mediatype == "movie" {
			yearstr := strconv.Itoa(t.Dbmovie.Year)
			if t.Dbmovie.Year != 0 {
				searchfor += " " + yearstr
			}
			search.titlesearch = true
			search.title = searchfor
			t.searchMedia(search, dl)
		} else if search.mediatype == "serie" {
			search.titlesearch = true
			search.title = searchfor + " " + t.Dbserieepisode.Identifier
			t.searchMedia(search, dl)
		}
		if len(dl.Nzbs) >= 1 {
			logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
			releasefound = true

			if checkfirst { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound
				logger.Log.GlobalLogger.Debug(breakindexerloop, zap.String("title", name))
				return true
			}
		}
	}
	if (!releasefound && checkalttitle) || (!releasefound && checkalttitle && search.titlesearch) { //config.Cfg.Quality[qualityprofile].BackupSearchForAlternateTitle
		alttitle := database.QueryStaticStringArray(false, 0, database.Querywithargs{QueryString: querytitle, Args: []interface{}{dbid, name}})
		search.titlesearch = true
		yearstr := ""
		if search.mediatype == "movie" {
			yearstr = strconv.Itoa(t.Dbmovie.Year)
		}
		for idxalt := range alttitle {
			if alttitle[idxalt] == "" {
				continue
			}
			if search.mediatype == "movie" {

				search.title = alttitle[idxalt]
				if yearstr != "0" {
					search.title += " (" + yearstr + ")"
				}
				t.searchMedia(search, dl)
			} else if search.mediatype == "serie" {
				search.title = alttitle[idxalt] + " " + t.Dbserieepisode.Identifier
				t.searchMedia(search, dl)
			}
			if len(dl.Nzbs) >= 1 {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
				releasefound = true

				if checkfirst {
					logger.Log.GlobalLogger.Debug(breakindexerloop, zap.String("Title", name))
					break
				}
			}
		}
		alttitle = nil
		if len(dl.Nzbs) >= 1 && checkfirst {
			logger.Log.GlobalLogger.Debug(breakindexerloop, zap.String("Title", name))
			return true
		}
	}
	return true
}

func (s *Searcher) SeriesSearch(episodeid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	var err error
	s.SerieEpisode, err = database.GetSerieEpisodes(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodeid}})
	if err != nil {
		return nil, err
	}

	s.Serie, _ = database.GetSeries(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.SerieID}})
	s.Dbserie, _ = database.GetDbserie(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.DbserieID}})
	s.Dbserieepisode, _ = database.GetDbserieEpisodes(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{s.SerieEpisode.DbserieEpisodeID}})

	s.SearchGroupType = "series"
	if s.SerieEpisode.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Search not wanted: ")
		return nil, errSearchDisabled
	}

	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		logger.Log.GlobalLogger.Error("Quality for Episode: " + strconv.Itoa(int(episodeid)) + spacenotfound)
		return nil, errNoQuality
	}
	s.Quality = s.SerieEpisode.QualityProfile
	s.MinimumPriority = GetHighestEpisodePriorityByFiles(episodeid, s.Cfgp, s.SerieEpisode.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if s.SerieEpisode.DontUpgrade && !forceDownload {
			logger.Log.GlobalLogger.Debug("Upgrade not wanted", zap.String("title", s.Dbserie.Seriename))
			return nil, errUpgradeDisabled
		}
	}

	processedindexer := 0
	dl := s.mediasearchindexerloopseries(&processedindexer, titlesearch)

	if processedindexer == 0 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}

	if processedindexer >= 1 {
		database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update serie_episodes set lastscan = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, episodeid}})
	}

	if dl == nil {
		return nil, errors.New(noresults)
	}
	return dl, nil
}

func (s *Searcher) mediasearchindexerloopseries(processedindexer *int, titlesearch bool) *SearchResults {
	return s.queryindexers(s.SerieEpisode.QualityProfile, "serie", false, processedindexer, titlesearch, queryparams{})
}

type queryparams struct {
	thetvdbid int
	season    int
	useseason bool
}

func (s *Searcher) queryindexers(quality string, searchaction string, fetchall bool, processedindexer *int, titlesearch bool, params queryparams) *SearchResults {
	dl := new(SearchResults)
	workergroup := logger.WorkerPools["Indexer"].Group()

	var seasonint int
	var episodeint int
	var err error
	for idx := range config.Cfg.Quality[quality].Indexer {
		search := searchstruct{quality: quality, mediatype: searchaction, titlesearch: titlesearch, indexer: config.Cfg.Quality[quality].Indexer[idx].TemplateIndexer}
		switch searchaction {
		case "movie":
			search.id = s.Movie.ID
			search.searchid = s.Dbmovie.ImdbID
		case "serie":
			search.id = s.SerieEpisode.ID
			if s.Dbserie.ThetvdbID != 0 {
				search.searchid = strconv.Itoa(s.Dbserie.ThetvdbID)
			}
			if s.Dbserieepisode.Season != "" {
				seasonint, err = strconv.Atoi(s.Dbserieepisode.Season)
				if err != nil {
					continue
				}
				search.season = seasonint
			}
			if s.Dbserieepisode.Episode != "" {
				episodeint, err = strconv.Atoi(s.Dbserieepisode.Episode)
				if err != nil {
					continue
				}
				search.episode = episodeint
			}
		default:
		}
		workergroup.Submit(func() {
			switch searchaction {
			case "movie", "serie":
				ok := s.mediasearchindexer(&search, dl)
				if ok {
					*processedindexer += 1
				}
			case "rss":
				ok := s.rsssearchindexer(&search, fetchall, dl)
				if ok {
					*processedindexer += 1
				}
			case "rssserie":
				ok := s.rssqueryseriesindexer(&search, params.thetvdbid, params.season, params.useseason, dl)
				if ok {
					*processedindexer += 1
				}
			}

		})
	}
	workergroup.Wait()
	return dl
}

func (s *Searcher) initIndexer(search *searchstruct, rssapi string, nzbIndexer *apiexternal.NzbIndexer) (string, error) {
	if !config.ConfigCheck("indexer_" + search.indexer) {
		return "", errNoIndexer
	}
	idxcfg := config.Cfg.Indexers[search.indexer]
	if !(strings.EqualFold(idxcfg.IndexerType, "newznab")) {
		return "", errors.New("indexer Type Wrong")
	}
	if !idxcfg.Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return "", errIndexerDisabled
	} else if !idxcfg.Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return "", errIndexerDisabled
	}

	userid, _ := strconv.Atoi(idxcfg.Userid)

	if ok, _ := apiexternal.NewznabCheckLimiter(idxcfg.Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fails - waiting till ", zap.Duration("waitfor", waitfor), zap.String("Indexer", idxcfg.Name))
		return "", errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString(database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, idxcfg.Url}})
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    idxcfg.Url,
		Apikey:                 idxcfg.Apikey,
		UserID:                 userid,
		SkipSslCheck:           idxcfg.DisableTLSVerify,
		Addquotesfortitlequery: idxcfg.Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "AdditionalQueryParams"),
		LastRssId:              lastindexerid,
		RssDownloadAll:         idxcfg.RssDownloadAll,
		Customapi:              idxcfg.Customapi,
		Customurl:              idxcfg.Customurl,
		Customrssurl:           idxcfg.Customrssurl,
		Customrsscategory:      idxcfg.Customrsscategory,
		Limitercalls:           idxcfg.Limitercalls,
		Limiterseconds:         idxcfg.Limiterseconds,
		LimitercallsDaily:      idxcfg.LimitercallsDaily,
		TimeoutSeconds:         idxcfg.TimeoutSeconds,
		MaxAge:                 idxcfg.MaxAge,
		OutputAsJson:           idxcfg.OutputAsJson}

	return config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "CategoriesIndexer"), nil
}

func (s *Searcher) initIndexerUrlCat(search *searchstruct, rssapi string) (string, string, error) {
	if !config.ConfigCheck("indexer_" + search.indexer) {
		return "", "", errNoIndexer
	}

	idxcfg := config.Cfg.Indexers[search.indexer]
	if !(strings.EqualFold(idxcfg.IndexerType, "newznab")) {
		return "", "", errors.New("indexer Type Wrong")
	}
	if !idxcfg.Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return "", "", errIndexerDisabled
	} else if !idxcfg.Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return "", "", errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(idxcfg.Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled", zap.Duration("waitfor", waitfor), zap.String("Indexer", idxcfg.Name))
		return "", "", errToWait
	}

	return idxcfg.Url, config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "CategoriesIndexer"), nil
}

func (s *Searcher) initNzbIndexer(search *searchstruct, rssapi string, nzbIndexer *apiexternal.NzbIndexer) error {
	if !config.ConfigCheck("indexer_" + search.indexer) {
		return errNoIndexer
	}
	idxcfg := config.Cfg.Indexers[search.indexer]
	if !(strings.EqualFold(idxcfg.IndexerType, "newznab")) {
		return errors.New("indexer type wrong")
	}
	if !idxcfg.Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return errIndexerDisabled
	} else if !idxcfg.Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", idxcfg.Name))
		return errIndexerDisabled
	}

	userid, _ := strconv.Atoi(idxcfg.Userid)

	if ok, _ := apiexternal.NewznabCheckLimiter(idxcfg.Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fails - waiting till ", zap.Duration("waitfor", waitfor), zap.String("indexer", idxcfg.Name))
		return errToWait
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString(database.Querywithargs{QueryString: "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", Args: []interface{}{s.Cfgp.NamePrefix, s.Quality, idxcfg.Url}})
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                    idxcfg.Url,
		Apikey:                 idxcfg.Apikey,
		UserID:                 userid,
		SkipSslCheck:           idxcfg.DisableTLSVerify,
		Addquotesfortitlequery: idxcfg.Addquotesfortitlequery,
		AdditionalQueryParams:  config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "AdditionalQueryParams"),
		LastRssId:              lastindexerid,
		RssDownloadAll:         idxcfg.RssDownloadAll,
		Customapi:              idxcfg.Customapi,
		Customurl:              idxcfg.Customurl,
		Customrssurl:           idxcfg.Customrssurl,
		Customrsscategory:      idxcfg.Customrsscategory,
		Limitercalls:           idxcfg.Limitercalls,
		Limiterseconds:         idxcfg.Limiterseconds,
		LimitercallsDaily:      idxcfg.LimitercallsDaily,
		TimeoutSeconds:         idxcfg.TimeoutSeconds,
		MaxAge:                 idxcfg.MaxAge,
		OutputAsJson:           idxcfg.OutputAsJson}

	return nil
}

func failedindexer(failed string) {
	// database.UpsertNamed("indexer_fails",
	// 	[]string{"indexer", "last_fail"},
	// 	database.IndexerFail{Indexer: failed, LastFail: sql.NullTime{Time: time.Now(), Valid: true}},
	// 	&database.Query{Where: "indexer = ?"}, failed,
	// 	"indexer = :indexer")
}

// func addsearched(dl *SearchResults, searched string) bool {
// 	for idx := range dl.Searched {
// 		if dl.Searched[idx] == searched {
// 			return true
// 		}
// 	}
// 	dl.Searched = append(dl.Searched, searched)
// 	return false
// }

func (s *Searcher) searchMedia(search *searchstruct, dl *SearchResults) bool {
	strid := strconv.Itoa(int(search.id))
	// searched := search.indexer + search.quality + strid + search.title + strconv.FormatBool(search.titlesearch)
	// if search.mediatype == "serie" {
	// 	searched += strconv.Itoa(search.season) + strconv.Itoa(search.episode)
	// }
	// if addsearched(dl, searched) {
	// 	return true
	// }
	if !config.ConfigCheck("quality_" + search.quality) {
		logger.Log.GlobalLogger.Error("Quality for: " + strid + spacenotfound)
		return false
	}
	nzbindexer := new(apiexternal.NzbIndexer)
	defer nzbindexer.Close()
	erri := s.initNzbIndexer(search, "api", nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled || erri == errNoIndexer {
			return true
		}
		if erri == errToWait {
			time.Sleep(10 * time.Second)
			return true
		}
		logger.Log.GlobalLogger.Debug(skippedindexer, zap.String("indexer", search.indexer), zap.Error(erri))
		return false
	}

	var nzbs *apiexternal.NZBArr
	var nzberr error

	backuptitlesearch := config.Cfg.Quality[search.quality].BackupSearchForTitle
	usetitlesearch := false
	if search.titlesearch && search.title != "" && backuptitlesearch {
		usetitlesearch = true
	}
	if !search.titlesearch {
		if search.mediatype == "movie" && s.Dbmovie.ImdbID != "" {
			nzbs, _, nzberr = apiexternal.QueryNewznabMovieImdb(nzbindexer, strings.Trim(s.Dbmovie.ImdbID, "t"), search.cats)
		} else if search.mediatype == "movie" && s.Dbmovie.ImdbID == "" && backuptitlesearch {
			usetitlesearch = true
		} else if search.mediatype == "serie" && s.Dbserie.ThetvdbID != 0 {
			nzbs, _, nzberr = apiexternal.QueryNewznabTvTvdb(nzbindexer, s.Dbserie.ThetvdbID, search.cats, search.season, search.episode, true, true)
		} else if search.mediatype == "serie" && (s.Dbserie.ThetvdbID == 0 || backuptitlesearch) {
			usetitlesearch = true
		}
	}
	//logger.Log.GlobalLogger.Debug("usetitlesearch", zap.Bool("backuptitlesearch", backuptitlesearch), zap.Bool("usetitlesearch", usetitlesearch), zap.String("indexer", search.indexer), zap.String("title", search.title))
	if usetitlesearch {
		//logger.Log.GlobalLogger.Debug("search for", zap.String("indexer", nzbindexer.URL), zap.String("title", search.title))
		nzbs, _, nzberr = apiexternal.QueryNewznabQuery(nzbindexer, search.title, search.cats, "search")
	}

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab Search failed", zap.String("title", search.title), zap.String("indexer", nzbindexer.URL), zap.Error(nzberr))
		failedindexer(nzbindexer.URL)
		return false
	} else {
		defer nzbs.Close()
		s.parseentries(nzbs, dl, search, "", false)
	}
	return true
}

func (s *Searcher) parseentries(nzbs *apiexternal.NZBArr, dl *SearchResults, search *searchstruct, listname string, addfound bool) {
	if len((nzbs.Arr)) >= 1 {
		if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplateGetFieldString(search.quality, search.indexer, "TemplateRegex")) {
			dl.Rejected = make([]apiexternal.Nzbwithprio, len(nzbs.Arr))
			for idx := range nzbs.Arr {
				nzbs.Arr[idx].Denied = true
				nzbs.Arr[idx].Reason = "Denied by Regex"
				dl.Rejected[idx] = nzbs.Arr[idx]
			}
		} else {
			dl.Rejected = logger.GrowSliceBy(dl.Rejected, len(nzbs.Arr))
			s.convertnzbs(search, nzbs, listname, addfound, dl)

			if len(dl.Nzbs) > 1 {
				sort.Slice(dl.Nzbs, func(i, j int) bool {
					return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
				})
			}
		}
	}
}

func allowMovieImport(imdb string, cfgp *config.MediaTypeConfig, listname string) bool {
	if !config.ConfigCheck("list_" + cfgp.ListsMap[listname].TemplateList) {
		return false
	}
	var countergenre int

	if config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinVotes != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and num_votes < ?", Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinVotes}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error vote count too low for ", zap.String("imdb", imdb))
			return false
		}
	}
	if config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinRating != 0 {
		countergenre, _ = database.ImdbCountRowsStatic(database.Querywithargs{QueryString: "select count() from imdb_ratings where tconst = ? and average_rating < ?", Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].MinRating}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error average vote too low for ", zap.String("imdb", imdb))
			return false
		}
	}
	excludebygenre := false
	countimdb := "select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE"
	for idxgenre := range config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre {
		countergenre, _ = database.ImdbCountRowsStatic(database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre[idxgenre]}})
		if countergenre >= 1 {
			excludebygenre = true
			logger.Log.GlobalLogger.Debug("error excluded genre ", zap.String("excluded", config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Excludegenre[idxgenre]), zap.String("imdb", imdb))
			break
		}
	}
	if excludebygenre {
		return false
	}
	includebygenre := false
	for idxgenre := range config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre {
		countergenre, _ = database.ImdbCountRowsStatic(database.Querywithargs{QueryString: countimdb, Args: []interface{}{imdb, config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre[idxgenre]}})
		if countergenre >= 1 {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList].Includegenre) >= 1 {
		logger.Log.GlobalLogger.Debug("error included genre not found ", zap.String("imdb", imdb))
		return false
	}
	return true
}

func GetHighestMoviePriorityByFiles(movieid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) (minPrio int) {
	foundfiles := database.QueryStaticIntArray(0, database.Querywithargs{QueryString: "select id as num from movie_files where movie_id = ?", Args: []interface{}{movieid}})

	var prio int
	for idx := range foundfiles {
		prio = GetMovieDBPriorityById(uint(foundfiles[idx]), cfgp, qualityTemplate)
		if minPrio < prio {
			minPrio = prio
		}
	}
	foundfiles = nil
	return minPrio
}

func GetHighestEpisodePriorityByFiles(episodeid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	foundfiles := database.QueryStaticIntArray(0, database.Querywithargs{QueryString: "select id as num from serie_episode_files where serie_episode_id = ?", Args: []interface{}{episodeid}})

	minPrio := 0
	var prio int
	for idx := range foundfiles {
		prio = GetSerieDBPriorityById(uint(foundfiles[idx]), cfgp, qualityTemplate)
		if minPrio < prio {
			minPrio = prio
		}
	}
	foundfiles = nil
	return minPrio
}

func GetSerieDBPriorityById(episodefileid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	serieepisodefile, err := database.GetSerieEpisodeFiles(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodefileid}})
	if err != nil {
		return 0
	}
	var m apiexternal.ParseInfo

	m.ResolutionID = serieepisodefile.ResolutionID
	m.QualityID = serieepisodefile.QualityID
	m.CodecID = serieepisodefile.CodecID
	m.AudioID = serieepisodefile.AudioID
	m.Proper = serieepisodefile.Proper
	m.Extended = serieepisodefile.Extended
	m.Repack = serieepisodefile.Repack
	m.Title = serieepisodefile.Filename
	m.File = serieepisodefile.Location

	parser.GetIDPriorityMap(&m, cfgp, qualityTemplate, true, false)
	prio := m.Priority
	return prio
}

func GetMovieDBPriorityById(moviefileid uint, cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	moviefile, err := database.GetMovieFiles(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{moviefileid}})
	if err != nil {
		return 0
	}
	var m apiexternal.ParseInfo

	m.ResolutionID = moviefile.ResolutionID
	m.QualityID = moviefile.QualityID
	m.CodecID = moviefile.CodecID
	m.AudioID = moviefile.AudioID
	m.Proper = moviefile.Proper
	m.Extended = moviefile.Extended
	m.Repack = moviefile.Repack
	parser.GetIDPriorityMap(&m, cfgp, qualityTemplate, true, false)
	prio := m.Priority
	return prio
}
