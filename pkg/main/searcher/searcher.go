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

func SearchMovieMissing(cfg string, jobcount int, titlesearch bool) {

	var scaninterval int
	var scandatepre int

	if len(config.Cfg.Media[cfg].Data) >= 1 {
		if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[0].Template_path) {
			return
		}
		scaninterval = config.Cfg.Paths[config.Cfg.Media[cfg].Data[0].Template_path].MissingScanInterval
		scandatepre = config.Cfg.Paths[config.Cfg.Media[cfg].Data[0].Template_path].MissingScanReleaseDatePre
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	qu.Select = "movies.id as num"
	qu.InnerJoin = "dbmovies on dbmovies.id=movies.dbmovie_id"
	qu.OrderBy = "movies.Lastscan asc"
	argcount := len(config.Cfg.Media[cfg].Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range config.Cfg.Media[cfg].Lists {
		whereArgs[i] = config.Cfg.Media[cfg].Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and (movies.lastscan is null or movies.Lastscan < ?)"
		whereArgs[len(config.Cfg.Media[cfg].Lists)] = scantime
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			whereArgs[len(config.Cfg.Media[cfg].Lists)+1] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		qu.Where = "movies.missing = 1 and movies.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ")"
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			whereArgs[len(config.Cfg.Media[cfg].Lists)] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfg, "movies", titlesearch, &qu, &whereArgs)
}

func downloadMovieSearchResults(first bool, movieid uint, cfg string, searchtype string, searchresults *SearchResults) {
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("title", searchresults.Nzbs[0].NZB.Title))
		downloadnow := downloader.NewDownloader(cfg)
		downloadnow.SetMovie(movieid)
		downloadnow.Nzb = searchresults.Nzbs[0]
		downloadnow.DownloadNzb()
		downloadnow.Close()
	}
}
func SearchMovieSingle(movieid uint, cfg string, titlesearch bool) {
	searchtype := "missing"
	missing, _ := database.QueryColumnBool("select missing from movies where id = ?", movieid)
	if !missing {
		searchtype = "upgrade"
	}
	quality, _ := database.QueryColumnString("select quality_profile from movies where id = ?", movieid)

	searchnow := NewSearcher(cfg, quality)
	defer searchnow.Close()

	results, err := searchnow.MovieSearch(movieid, false, titlesearch)
	if err == nil {
		defer results.Close()
		downloadMovieSearchResults(true, movieid, cfg, searchtype, results)
	}
}

func SearchMovieUpgrade(cfg string, jobcount int, titlesearch bool) {
	var scaninterval int

	if len(config.Cfg.Media[cfg].Data) >= 1 {
		if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[0].Template_path) {
			return
		}
		scaninterval = config.Cfg.Paths[config.Cfg.Media[cfg].Data[0].Template_path].UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	argcount := len(config.Cfg.Media[cfg].Lists)
	if scaninterval != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range config.Cfg.Media[cfg].Lists {
		whereArgs[i] = config.Cfg.Media[cfg].Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and (lastscan is null or Lastscan < ?)"
		whereArgs[len(config.Cfg.Media[cfg].Lists)] = scantime
	} else {
		qu.Where = "quality_reached = 0 and missing = 0 and listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ")"
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.Select = "id as num"
	qu.OrderBy = "Lastscan asc"

	searchlist(cfg, "movies", titlesearch, &qu, &whereArgs)
}

func SearchSerieSingle(serieid uint, cfg string, titlesearch bool) {
	episodes := database.QueryStaticIntArray("select id as num from serie_episodes where serie_id = ?", 0, serieid)
	if len(episodes) >= 1 {
		for idx := range episodes {
			missing := episodes[idx]
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(uint(missing), cfg, titlesearch)
			//})
		}
	}
	episodes = nil
}

func SearchSerieSeasonSingle(serieid uint, season string, cfg string, titlesearch bool) {
	episodes := database.QueryStaticIntArray("select id as num from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)", 0, serieid, season)
	if len(episodes) >= 1 {
		for idx := range episodes {
			missing := episodes[idx]
			//workergroup.Submit(func() {
			SearchSerieEpisodeSingle(uint(missing), cfg, titlesearch)
			//})
		}
	}
	episodes = nil
}

func SearchSerieRSSSeasonSingle(serieid uint, season int, useseason bool, cfg string) {
	qualstr, _ := database.QueryColumnString("select quality_profile from serie_episodes where serie_id = ?", serieid)
	if qualstr == "" {
		return
	}
	dbserieid, _ := database.QueryColumnUint("select dbserie_id from series where id = ?", serieid)
	tvdb, err := database.QueryColumnUint("select thetvdb_id from dbseries where id = ?", dbserieid)
	if err != nil {
		return
	}
	SearchSerieRSSSeason(cfg, qualstr, int(tvdb), season, useseason)
}
func SearchSeriesRSSSeasons(cfg string) {
	argcount := len(config.Cfg.Media[cfg].Lists)
	whereArgs := make([]interface{}, argcount)
	for i := range config.Cfg.Media[cfg].Lists {
		whereArgs[i] = config.Cfg.Media[cfg].Lists[i].Name
	}
	series, _ := database.QueryStaticColumnsTwoInt("select id as num1, dbserie_id as num2 from series where listname in (?"+strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1)+") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != 0 and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1", whereArgs...)
	if len(series) >= 1 {
		var seasons []string = make([]string, 0, 5)
		var seasonint int
		var err error
		for idx := range series {
			seasons = database.QueryStaticStringArray("select distinct season from dbserie_episodes where dbserie_id = ?", false, 10, series[idx].Num2)
			for idxseason := range seasons {
				if seasons[idxseason] == "" {
					continue
				}
				seasonint, err = strconv.Atoi(seasons[idxseason])
				if err == nil {
					//workergroup.Submit(func() {
					SearchSerieRSSSeasonSingle(uint(series[idx].Num1), seasonint, true, cfg)
					//})
				}
			}
		}
		seasons = nil
	}
	series = nil
}
func SearchSerieEpisodeSingle(episodeid uint, cfg string, titlesearch bool) {
	quality, _ := database.QueryColumnString("select quality_profile from serie_episodes where id = ?", episodeid)

	searchnow := NewSearcher(cfg, quality)
	defer searchnow.Close()
	searchresults, err := searchnow.SeriesSearch(episodeid, false, titlesearch)
	if err != nil {
		return
	}
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.GlobalLogger.Debug("nzb found - start downloading", zap.String("title", searchresults.Nzbs[0].NZB.Title))
		downloadnow := downloader.NewDownloader(cfg)
		downloadnow.SetSeriesEpisode(episodeid)
		downloadnow.Nzb = searchresults.Nzbs[0]
		downloadnow.DownloadNzb()
		downloadnow.Close()
	}
	defer searchresults.Close()
}
func SearchSerieMissing(cfg string, jobcount int, titlesearch bool) {
	rowdata := config.Cfg.Media[cfg].Data[0]

	var scaninterval int
	var scandatepre int

	if !config.ConfigCheck("path_" + rowdata.Template_path) {
		return
	}
	scaninterval = config.Cfg.Paths[rowdata.Template_path].MissingScanInterval
	scandatepre = config.Cfg.Paths[rowdata.Template_path].MissingScanReleaseDatePre

	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
		logger.Log.GlobalLogger.Debug("Search before scan", zap.Time("Time", scantime))
	}

	var qu database.Query
	qu.Select = "serie_episodes.id as num"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

	argcount := len(config.Cfg.Media[cfg].Lists)
	if scaninterval != 0 {
		argcount++
	}
	if scandatepre != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range config.Cfg.Media[cfg].Lists {
		whereArgs[i] = config.Cfg.Media[cfg].Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"
		whereArgs[len(config.Cfg.Media[cfg].Lists)] = scantime
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			whereArgs[len(config.Cfg.Media[cfg].Lists)+1] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	} else {
		qu.Where = "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			whereArgs[len(config.Cfg.Media[cfg].Lists)] = time.Now().AddDate(0, 0, 0+scandatepre)
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfg, "serie_episodes", titlesearch, &qu, &whereArgs)
}

func SearchSerieUpgrade(cfg string, jobcount int, titlesearch bool) {
	var scaninterval int

	if len(config.Cfg.Media[cfg].Data) >= 1 {
		if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[0].Template_path) {
			return
		}
		scaninterval = config.Cfg.Paths[config.Cfg.Media[cfg].Data[0].Template_path].UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	var qu database.Query
	qu.Select = "serie_episodes.ID as num"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.ID=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"

	argcount := len(config.Cfg.Media[cfg].Lists)
	if scaninterval != 0 {
		argcount++
	}
	whereArgs := make([]interface{}, argcount)
	for i := range config.Cfg.Media[cfg].Lists {
		whereArgs[i] = config.Cfg.Media[cfg].Lists[i].Name
	}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
		whereArgs[len(config.Cfg.Media[cfg].Lists)] = scantime
	} else {
		qu.Where = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(config.Cfg.Media[cfg].Lists)-1) + ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having COUNT(*) = 1)"
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}

	searchlist(cfg, "serie_episodes", titlesearch, &qu, &whereArgs)
	whereArgs = nil
}
func searchlist(cfg string, table string, titlesearch bool, qu *database.Query, args *[]interface{}) {
	missingepisode, _ := database.QueryStaticColumnsOneIntQueryObject(table, qu, *args...)
	var typemovies bool
	if len(missingepisode) >= 1 {
		typemovies = strings.HasPrefix(cfg, "movie_")
		for idx := range missingepisode {
			if typemovies {
				//workergroup.Submit(func() {
				SearchMovieSingle(uint(missingepisode[idx].Num), cfg, titlesearch)
				//})
			} else {
				//workergroup.Submit(func() {
				SearchSerieEpisodeSingle(uint(missingepisode[idx].Num), cfg, titlesearch)
				//})
			}
		}
	}
	missingepisode = nil
}

func SearchSerieRSS(cfg string, quality string) {
	logger.Log.GlobalLogger.Debug("Get Rss Series List")

	searchrssnow(cfg, quality, "series")
}

func searchrssnow(cfg string, quality string, mediatype string) {
	searchnow := NewSearcher(cfg, quality)
	defer searchnow.Close()
	results, err := searchnow.SearchRSS(mediatype, false)
	if err == nil {
		downloadNzb(results, cfg, mediatype)

		defer results.Close()
	}
}

func SearchSerieRSSSeason(cfg string, quality string, thetvdb_id int, season int, useseason bool) {
	logger.Log.GlobalLogger.Debug("Get Rss Series List")

	searchnow := NewSearcher(cfg, quality)
	defer searchnow.Close()
	results, err := searchnow.SearchSeriesRSSSeason("series", thetvdb_id, season, useseason)
	if err == nil {
		downloadNzb(results, cfg, "series")

		defer results.Close()
	}
}

func downloadNzb(searchresults *SearchResults, cfg string, mediatype string) {
	var downloaded []uint = make([]uint, 0, len(searchresults.Nzbs))
	var breakfor bool
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

		downloadnow := downloader.NewDownloader(cfg)

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
func SearchMovieRSS(cfg string, quality string) {
	logger.Log.GlobalLogger.Debug("Get Rss Movie List")

	searchrssnow(cfg, quality, "movie")
}

type SearchResults struct {
	Nzbs     []apiexternal.Nzbwithprio
	Rejected []apiexternal.Nzbwithprio
	Searched []string
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
	Cfg              string
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

func NewSearcher(cfg string, quality string) *Searcher {
	return &Searcher{
		Cfg:     cfg,
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

// searchGroupType == movie || series
func (s *Searcher) SearchRSS(searchGroupType string, fetchall bool) (*SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
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
		return nil, errors.New("no results")
	}
	if dl != nil {
		if len(dl.Nzbs) == 0 {
			logger.Log.GlobalLogger.Info("No new entries found")
		}
	}
	return dl, nil
}

func (s *Searcher) rsssearchindexerloop(processedindexer *int, fetchall bool) (dl *SearchResults) {
	return s.queryindexers(s.Quality, "rss", fetchall, 0, 0, false, processedindexer, false)
}

func (t *Searcher) rsssearchindexer(quality string, indexer string, fetchall bool, dl *SearchResults) bool {
	if addsearched(dl, indexer+quality) {
		return true
	}
	var nzbindexer apiexternal.NzbIndexer
	cats, erri := t.initIndexer(quality, indexer, "rss", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled {
			return true
		}
		logger.Log.GlobalLogger.Debug("Skipped Indexer", zap.String("indexer", indexer), zap.Error(erri))
		return false
	}

	if fetchall {
		nzbindexer.LastRssId = ""
	}
	maxentries := config.Cfg.Indexers[indexer].MaxRssEntries
	maxloop := config.Cfg.Indexers[indexer].RssEntriesloop
	if maxentries == 0 {
		maxentries = 10
	}
	if maxloop == 0 {
		maxloop = 2
	}

	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&nzbindexer, maxentries, cats, maxloop)

	if nzberr != nil {

		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.String("indexer", indexer), zap.Error(nzberr))
		failedindexer(failed)
		return false
	} else {
		defer nzbs.Close()
		if !fetchall {
			if lastids != "" && len((nzbs.Arr)) >= 1 {
				addrsshistory(nzbindexer.URL, lastids, t.Quality, t.Cfg)
			}
		}
		//logger.Log.GlobalLogger.Debug("Search RSS ended - found entries", zap.Int("entries", len((nzbs.Arr))))
		t.parseentries(nzbs, dl, quality, indexer, "", false)
	}
	return true
}

// searchGroupType == movie || series
func (s *Searcher) SearchSeriesRSSSeason(searchGroupType string, thetvdb_id int, season int, useseason bool) (*SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	processedindexer := 0
	dl := s.rssqueryseriesindexerloop(&processedindexer, thetvdb_id, season, useseason)
	if processedindexer == 0 {
		blockinterval := 5
		if config.Cfg.General.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.Cfg.General.FailedIndexerBlockTime
		}
		config.Slepping(false, blockinterval*60)
	}
	if dl == nil {
		return nil, errors.New("no results")
	}
	return dl, nil
}

func (s *Searcher) rssqueryseriesindexerloop(processedindexer *int, thetvdb_id int, season int, useseason bool) (dl *SearchResults) {
	return s.queryindexers(s.Quality, "rssserie", false, thetvdb_id, season, useseason, processedindexer, false)
}

func (t *Searcher) rssqueryseriesindexer(quality string, indexer string, thetvdb_id int, season int, useseason bool, dl *SearchResults) bool {

	if addsearched(dl, indexer+quality+strconv.Itoa(thetvdb_id)+strconv.Itoa(season)) {
		return true
	}
	var nzbindexer apiexternal.NzbIndexer
	cats, erri := t.initIndexer(quality, indexer, "api", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled {
			return true
		}
		logger.Log.GlobalLogger.Debug("Skipped Indexer", zap.String("indexer", indexer), zap.Error(erri))
		return false
	}

	nzbs, _, nzberr := apiexternal.QueryNewznabTvTvdb(&nzbindexer, thetvdb_id, cats, season, 0, useseason, false)

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed ", zap.String("indexer", indexer), zap.Error(nzberr))
		failedindexer(nzbindexer.URL)
		return false
	} else {
		defer nzbs.Close()
		t.parseentries(nzbs, dl, quality, indexer, "", false)
	}
	return true
}

func (s *Searcher) checkhistory(quality string, indexer string, entry apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Check History
	historytable := "serie_episode_histories"
	if strings.EqualFold(s.SearchGroupType, "movie") {
		historytable = "movie_histories"
	}
	found := logger.GlobalCache.CheckNoType(historytable + "_url")
	if !found {
		logger.GlobalCache.Set(historytable+"_url", database.QueryStaticStringArray("select url from "+historytable, false, 0), 8*time.Hour)
	}
	historycache := logger.GlobalCache.GetData(historytable + "_url")

	if len(entry.NZB.DownloadURL) > 1 {
		for idx := range historycache.Value.([]string) {
			if strings.EqualFold(historycache.Value.([]string)[idx], entry.NZB.DownloadURL) {
				denynzb("Already downloaded (Url)", entry, dl)
				return true
			}
		}
	}
	if config.QualityIndexerByQualityAndTemplateGetFieldBool(quality, indexer, "History_check_title") && len(entry.NZB.Title) > 1 {
		found = logger.GlobalCache.CheckNoType(historytable + "_title")
		if !found {
			logger.GlobalCache.Set(historytable+"_title", database.QueryStaticStringArray("select title from "+historytable, false, 0), 8*time.Hour)
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
			if len(entry.NZB.IMDBID) >= 1 {
				//Check Correct Imdb
				tempimdb := strings.TrimPrefix(entry.NZB.IMDBID, "tt")
				tempimdb = strings.TrimLeft(tempimdb, "0")

				wantedimdb := strings.TrimPrefix(s.Dbmovie.ImdbID, "tt")
				wantedimdb = strings.TrimLeft(wantedimdb, "0")
				if wantedimdb != tempimdb && len(wantedimdb) >= 1 && len(tempimdb) >= 1 {
					denynzb("Imdb not match", entry, dl)
					return true
				}
			}
		} else {
			//Check TVDB Id
			if s.Dbserie.ThetvdbID >= 1 && len(entry.NZB.TVDBID) >= 1 {
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
			denied := s.getmovierss(entry, addinlist, addifnotfound, s.Cfg, s.Quality, dl)
			if denied {
				return true
			}
			dbmovieid = s.Movie.DbmovieID
			entry.WantedTitle = s.Dbmovie.Title
			entry.QualityTemplate = s.Movie.QualityProfile

			//Check Minimal Priority
			entry.MinimumPriority = GetHighestMoviePriorityByFiles(entry.NzbmovieID, s.Cfg, s.Movie.QualityProfile)

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

		entry.WantedAlternates = database.QueryStaticStringArray("select title from dbmovie_titles where dbmovie_id = ?", false, 0, dbmovieid)
	} else {
		if s.SearchActionType == "rss" {
			//Filter RSS Series
			denied := s.getserierss(entry, s.Cfg, s.Quality, dl)
			if denied {
				return true
			}
			entry.QualityTemplate = s.SerieEpisode.QualityProfile
			entry.WantedTitle = s.Dbserie.Seriename

			//Check Minimum Priority
			entry.MinimumPriority = GetHighestEpisodePriorityByFiles(entry.NzbepisodeID, s.Cfg, entry.QualityTemplate)

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

		entry.WantedAlternates = database.QueryStaticStringArray("select title from dbserie_alternates where dbserie_id = ?", false, 0, s.Dbserie.ID)
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
		if checkyear && !checkyear1 && !strings.Contains(entry.NZB.Title, yearstr) && len(yearstr) >= 1 && yearstr != "0" {
			denynzb("Unwanted Year", entry, dl)
			return true
		} else {
			if checkyear1 && len(yearstr) >= 1 && yearstr != "0" {
				yearint, _ := strconv.Atoi(yearstr)
				if !strings.Contains(entry.NZB.Title, strconv.Itoa(yearint+1)) && !strings.Contains(entry.NZB.Title, strconv.Itoa(yearint-1)) && !strings.Contains(entry.NZB.Title, strconv.Itoa(yearint)) {
					denynzb("Unwanted Year1", entry, dl)
					return true
				}
			}
		}
	}
	return false
}
func Checktitle(entry apiexternal.Nzbwithprio, dl *SearchResults) bool {
	checktitle := config.Cfg.Quality[entry.QualityTemplate].CheckTitle
	//Checktitle
	if checktitle {
		titlefound := false
		if entry.WantedTitle != "" {
			if checktitle && apiexternal.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title) && len(entry.WantedTitle) >= 1 {
				titlefound = true
			}
			if !titlefound && entry.ParseInfo.Year != 0 {
				if checktitle && apiexternal.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title+" "+strconv.Itoa(entry.ParseInfo.Year)) && len(entry.WantedTitle) >= 1 {
					titlefound = true
				}
			}
		}
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
					if checktitle && apiexternal.Checknzbtitle(entry.WantedAlternates[idxtitle], entry.ParseInfo.Title+" "+strconv.Itoa(entry.ParseInfo.Year)) {
						alttitlefound = true
						break
					}
				}
			}
			if len(entry.WantedAlternates) >= 1 && !alttitlefound {
				denynzb("Unwanted Title and Alternate", entry, dl)
				return true
			}
		}
		if len(entry.WantedAlternates) == 0 && !titlefound {
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

		lower_identifier := strings.ToLower(s.Dbserieepisode.Identifier)
		lower_parse_identifier := strings.ToLower(entry.ParseInfo.Identifier)
		lower_title := strings.ToLower(entry.NZB.Title)
		alt_identifier := strings.TrimLeft(lower_identifier, "s0")
		alt_identifier = strings.Replace(alt_identifier, "e", "x", -1)
		if strings.Contains(lower_title, lower_identifier) ||
			strings.Contains(lower_title, strings.Replace(lower_identifier, "-", ".", -1)) ||
			strings.Contains(lower_title, strings.Replace(lower_identifier, "-", " ", -1)) ||
			strings.Contains(lower_title, alt_identifier) ||
			strings.Contains(lower_title, strings.Replace(alt_identifier, "-", ".", -1)) ||
			strings.Contains(lower_title, strings.Replace(alt_identifier, "-", " ", -1)) {

			matchfound = true
		} else {
			seasonarray := []string{"s" + s.Dbserieepisode.Season + "e", "s0" + s.Dbserieepisode.Season + "e", "s" + s.Dbserieepisode.Season + " e", "s0" + s.Dbserieepisode.Season + " e", s.Dbserieepisode.Season + "x", s.Dbserieepisode.Season + " x"}
			episodearray := []string{"e" + s.Dbserieepisode.Episode, "e0" + s.Dbserieepisode.Episode, "x" + s.Dbserieepisode.Episode, "x0" + s.Dbserieepisode.Episode}
			for idxseason := range seasonarray {
				if strings.HasPrefix(lower_parse_identifier, seasonarray[idxseason]) {
					for idxepisode := range episodearray {
						if strings.HasSuffix(lower_parse_identifier, episodearray[idxepisode]) {
							matchfound = true
							break
						}
						if strings.Contains(lower_parse_identifier, episodearray[idxepisode]+" ") {
							matchfound = true
							break
						} else if strings.Contains(lower_parse_identifier, episodearray[idxepisode]+"-") {
							matchfound = true
							break
						} else if strings.Contains(lower_parse_identifier, episodearray[idxepisode]+"e") {
							matchfound = true
							break
						} else if strings.Contains(lower_parse_identifier, episodearray[idxepisode]+"x") {
							matchfound = true
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
func (s *Searcher) convertnzbs(quality string, indexer string, entries *apiexternal.NZBArr, addinlist string, addifnotfound bool, dl *SearchResults) {
	for entryidx := range entries.Arr {
		entries.Arr[entryidx].Indexer = indexer

		//Check Title Length
		if len(entries.Arr[entryidx].NZB.DownloadURL) == 0 {
			denynzb("No Url", entries.Arr[entryidx], dl)
			return
		}
		if len(entries.Arr[entryidx].NZB.Title) == 0 {
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
		index := config.QualityIndexerByQualityAndTemplate(quality, entries.Arr[entryidx].Indexer)
		defer index.Close()
		if index.Template_regex == "" {
			denynzb("No Indexer Regex Template", entries.Arr[entryidx], dl)
			return
		}
		if index.Filter_size_nzbs(s.Cfg, entries.Arr[entryidx].NZB.Title, entries.Arr[entryidx].NZB.Size) {
			denynzb("Wrong size", entries.Arr[entryidx], dl)
			return
		}

		if s.checkhistory(quality, entries.Arr[entryidx].Indexer, entries.Arr[entryidx], dl) {
			return
		}

		if s.checkcorrectid(entries.Arr[entryidx], dl) {
			return
		}

		//Regex
		regexdeny, regexrule := entries.Arr[entryidx].Filter_regex_nzbs(index.Template_regex, entries.Arr[entryidx].NZB.Title)
		if regexdeny {
			denynzb("Denied by Regex: "+regexrule, entries.Arr[entryidx], dl)
			return
		}

		//Parse
		parsefile := entries.Arr[entryidx].ParseInfo.File == ""
		if entries.Arr[entryidx].ParseInfo.File != "" {
			if entries.Arr[entryidx].ParseInfo.Title == "" || entries.Arr[entryidx].ParseInfo.Resolution == "" || entries.Arr[entryidx].ParseInfo.Quality == "" {
				parsefile = true
			}
		}
		if parsefile {
			includeyear := false
			if s.SearchGroupType == "series" {
				includeyear = true
			}
			var err error
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
			parser.GetPriorityMap(&entries.Arr[entryidx].ParseInfo, s.Cfg, entries.Arr[entryidx].QualityTemplate, false)
			entries.Arr[entryidx].Prio = entries.Arr[entryidx].ParseInfo.Priority
		}

		importfeed.StripTitlePrefixPostfix(&entries.Arr[entryidx].ParseInfo.Title, entries.Arr[entryidx].QualityTemplate)

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
		if !Filter_test_quality_wanted(&entries.Arr[entryidx].ParseInfo, entries.Arr[entryidx].QualityTemplate, entries.Arr[entryidx].NZB.Title) {
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
}

func Filter_test_quality_wanted(m *apiexternal.ParseInfo, qualityTemplate string, title string) bool {
	wanted_release_resolution := false
	quals := &logger.InStringArrayStruct{Arr: config.Cfg.Quality[qualityTemplate].Wanted_resolution}
	defer quals.Close()
	if len(quals.Arr) >= 1 && m.Resolution != "" {
		if logger.InStringArray(m.Resolution, quals) {
			wanted_release_resolution = true
		}
	}

	if len(quals.Arr) >= 1 && !wanted_release_resolution {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted resolution", zap.String("title", title), zap.String("Quality", qualityTemplate), zap.String("Resolution", m.Resolution))
		return false
	}
	wanted_release_quality := false

	quals.Arr = config.Cfg.Quality[qualityTemplate].Wanted_quality
	if len(quals.Arr) >= 1 && m.Quality != "" {
		if logger.InStringArray(m.Quality, quals) {
			wanted_release_quality = true
		}
	}
	if len(quals.Arr) >= 1 && !wanted_release_quality {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted quality", zap.String("title", title), zap.String("Wanted Quality", qualityTemplate), zap.String("Quality", m.Quality))
		return false
	}

	wanted_release_audio := false

	quals.Arr = config.Cfg.Quality[qualityTemplate].Wanted_audio
	if len(quals.Arr) >= 1 && m.Audio != "" {
		if logger.InStringArray(m.Audio, quals) {
			wanted_release_audio = true
		}
	}
	if len(quals.Arr) >= 1 && !wanted_release_audio {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted audio", zap.String("title", title), zap.String("Quality", qualityTemplate))
		return false
	}
	wanted_release_codec := false

	quals.Arr = config.Cfg.Quality[qualityTemplate].Wanted_codec
	if len(quals.Arr) >= 1 && m.Codec != "" {
		if logger.InStringArray(m.Codec, quals) {
			wanted_release_codec = true
		}
	}
	if len(quals.Arr) >= 1 && !wanted_release_codec {
		logger.Log.GlobalLogger.Debug("Skipped - unwanted codec", zap.String("title", title), zap.String("Quality", qualityTemplate))
		return false
	}
	return true
}

func insertmovie(imdb string, cfg string, addinlist string) (uint, error) {
	importfeed.JobImportMovies(imdb, cfg, addinlist, true)
	founddbmovie, founddbmovieerr := database.QueryColumnUint("select id from dbmovies where imdb_id = ?", imdb)
	return founddbmovie, founddbmovieerr
}

func (s *Searcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlist string, addifnotfound bool, cfg string, qualityTemplate string, dl *SearchResults) bool {
	//Parse
	var err error
	parser.GetDbIDs("movie", &entry.ParseInfo, cfg, addinlist, true)
	loopdbmovie := entry.ParseInfo.DbmovieID
	loopmovie := entry.ParseInfo.MovieID
	//Get DbMovie by imdbid

	//Add DbMovie if not found yet and enabled
	if loopdbmovie == 0 {
		if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") {
			if !allowMovieImport(entry.NZB.IMDBID, cfg, addinlist) {
				denynzb("Not Allowed Movie", *entry, dl)
				return true
			}
			var err2 error
			loopdbmovie, err2 = insertmovie(entry.NZB.IMDBID, cfg, addinlist)
			loopmovie, err = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", loopdbmovie, addinlist)
			if err != nil || err2 != nil {
				denynzb("Not Wanted Movie", *entry, dl)
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
			if !allowMovieImport(entry.NZB.IMDBID, cfg, addinlist) {
				denynzb("Not Allowed Movie", *entry, dl)
				return true
			}
			loopdbmovie, _ = insertmovie(entry.NZB.IMDBID, cfg, addinlist)
			loopmovie, _ = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", loopdbmovie, addinlist)
			if loopdbmovie == 0 || loopmovie == 0 {
				denynzb("Not Wanted Movie", *entry, dl)
				return true
			}
		} else {
			denynzb("Not Wanted Movie", *entry, dl)
			return true
		}
	}
	if loopmovie == 0 {
		denynzb("Not Wanted Movie", *entry, dl)
		return true
	}

	s.Movie, _ = database.GetMovies(&database.Query{Where: "id = ?"}, loopmovie)
	s.Dbmovie, _ = database.GetDbmovie(&database.Query{Where: "id = ?"}, loopdbmovie)

	entry.QualityTemplate = s.Movie.QualityProfile
	importfeed.StripTitlePrefixPostfix(&entry.ParseInfo.Title, entry.QualityTemplate)
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
func (s *Searcher) getserierss(entry *apiexternal.Nzbwithprio, cfg string, qualityTemplate string, dl *SearchResults) bool {
	parser.GetDbIDs("series", &entry.ParseInfo, cfg, "", true)
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

	s.SerieEpisode, _ = database.GetSerieEpisodes(&database.Query{Where: "id = ?"}, entry.NzbepisodeID)
	s.Serie, _ = database.GetSeries(&database.Query{Where: "id = ?"}, s.SerieEpisode.SerieID)
	s.Dbserie, _ = database.GetDbserie(&database.Query{Where: "id = ?"}, s.SerieEpisode.DbserieID)
	s.Dbserieepisode, _ = database.GetDbserieEpisodes(&database.Query{Where: "id = ?"}, s.SerieEpisode.DbserieEpisodeID)
	entry.WantedAlternates = database.QueryStaticStringArray("select title from dbserie_alternates where dbserie_id = ?", false, 0, loopdbseries)
	return false
}

func (s *Searcher) GetRSSFeed(searchGroupType string, cfg string, listname string) (*SearchResults, error) {
	list_Template_list := config.Cfg.Media[cfg].ListsMap[listname].Template_list
	if list_Template_list == "" {
		return nil, errNoList
	}
	if !config.ConfigCheck("list_" + list_Template_list) {
		return nil, errNoList
	}
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.GlobalLogger.Error("Quality for RSS not found")
		return nil, errNoQuality
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	var cfg_indexer config.QualityIndexerConfig
	for idx := range config.Cfg.Quality[s.Quality].Indexer {
		if config.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer == list_Template_list {
			cfg_indexer = config.Cfg.Quality[s.Quality].Indexer[idx]
			break
		}
	}
	if cfg_indexer.Template_regex == "" {
		return nil, errNoRegex
	}

	lastindexerid, _ := database.QueryColumnString("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", list_Template_list, s.Quality, "")

	url := config.Cfg.Lists[list_Template_list].Url
	indexer := apiexternal.NzbIndexer{Name: list_Template_list, Customrssurl: url, LastRssId: lastindexerid}
	blockinterval := -5
	if config.Cfg.General.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.Cfg.General.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("select count() from indexer_fails where indexer = ? and last_fail > ?", url, time.Now().Add(time.Minute*time.Duration(blockinterval)))
	if counter >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fail in the last ", zap.Int("Minutes", blockinterval), zap.String("Listname", list_Template_list))
		return nil, errIndexerDisabled
	}

	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(&indexer, config.Cfg.Lists[list_Template_list].Limit, "", 1)

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab RSS Search failed", zap.Error(nzberr))
		failedindexer(failed)
	} else {
		defer nzbs.Close()
		if lastids != "" && len((nzbs.Arr)) >= 1 {
			addrsshistory(indexer.URL, lastids, s.Quality, list_Template_list)
		}
		dl := SearchResults{Rejected: make([]apiexternal.Nzbwithprio, 0, len(nzbs.Arr))}

		s.parseentries(nzbs, &dl, s.Quality, list_Template_list, listname, config.Cfg.Media[cfg].ListsMap[listname].Addfound)

		if len(dl.Nzbs) == 0 {
			logger.Log.GlobalLogger.Info("No new entries found")
		}
		return &dl, nil
	}
	return nil, errOther
}

func addrsshistory(url string, lastid string, quality string, config string) {
	database.UpsertNamed("r_sshistories",
		&logger.InStringArrayStruct{Arr: []string{"indexer", "last_id", "list", "config"}},
		database.RSSHistory{Indexer: url, LastID: lastid, List: quality, Config: config},
		"config = :config COLLATE NOCASE and list = :list COLLATE NOCASE and indexer = :indexer COLLATE NOCASE",
		&database.Query{Where: "config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE"},
		config, quality, url)
}

func (s *Searcher) MovieSearch(movieid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	s.SearchGroupType = "movie"
	var err error
	s.Movie, err = database.GetMovies(&database.Query{Where: "id = ?"}, movieid)
	if err != nil {
		return nil, err
	}
	s.Dbmovie, _ = database.GetDbmovie(&database.Query{Where: "id = ?"}, s.Movie.DbmovieID)

	if s.Movie.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Skipped - Search disabled")
		return nil, errSearchDisabled
	}

	if s.Dbmovie.Year == 0 {
		//logger.Log.GlobalLogger.Debug("Skipped - No Year")
		return nil, errors.New("year not found")
	}

	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		logger.Log.GlobalLogger.Error("Quality for Movie: " + strconv.Itoa(int(movieid)) + " not found")
		return nil, errNoQuality
	}
	s.Quality = s.Movie.QualityProfile
	s.MinimumPriority = GetHighestMoviePriorityByFiles(movieid, s.Cfg, s.Movie.QualityProfile)

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
		database.UpdateColumnStatic("Update movies set lastscan = ? where id = ?", sql.NullTime{Time: time.Now(), Valid: true}, movieid)
	}

	if dl == nil {
		return nil, errors.New("no results")
	}
	return dl, nil
}

func (s *Searcher) mediasearchindexerloop(processedindexer *int, titlesearch bool) (dl *SearchResults) {
	return s.queryindexers(s.Movie.QualityProfile, "movie", false, 0, 0, false, processedindexer, titlesearch)
}
func (t *Searcher) mediasearchindexer(mediatype string, quality string, indexer string, titlesearch bool, dl *SearchResults) bool {
	_, cats, erri := t.initIndexerUrlCat(quality, indexer, "api")
	if erri != nil {
		if erri == errIndexerDisabled {
			return true
		}
		logger.Log.GlobalLogger.Debug("Skipped Indexer", zap.String("indexer", indexer), zap.Error(erri))

		return false
	}
	var seasonint int
	var episodeint int
	var err error
	searchfor := ""

	var dbid, mediaid uint
	var qualityprofile, name, querytitle string

	if mediatype == "movie" {
		dbid = t.Dbmovie.ID
		mediaid = t.Movie.ID
		qualityprofile = t.Movie.QualityProfile
		name = t.Dbmovie.Title
		querytitle = "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ? COLLATE NOCASE"
	}
	if mediatype == "serie" {

		if t.Dbserieepisode.Season != "" {
			seasonint, err = strconv.Atoi(t.Dbserieepisode.Season)
			if err != nil {
				logger.Log.GlobalLogger.Error("Season not convertable")
				return false
			}
		}
		if t.Dbserieepisode.Episode != "" {
			episodeint, err = strconv.Atoi(t.Dbserieepisode.Episode)
			if err != nil {
				logger.Log.GlobalLogger.Error("Episode not convertable")
				return false
			}
		}
		dbid = t.Dbserieepisode.ID
		qualityprofile = t.SerieEpisode.QualityProfile
		mediaid = t.SerieEpisode.ID
		name = t.Dbserie.Seriename
		querytitle = "select distinct title from dbserie_alternates where dbserie_id = ? and title != ? COLLATE NOCASE"
	}

	releasefound := false
	if !config.ConfigCheck("quality_" + qualityprofile) {
		logger.Log.GlobalLogger.Error("Quality for: " + strconv.Itoa(int(mediaid)) + " not found")
		return false
	}
	checkfirst := config.Cfg.Quality[qualityprofile].CheckUntilFirstFound
	checktitle := config.Cfg.Quality[qualityprofile].BackupSearchForTitle
	checkalttitle := config.Cfg.Quality[qualityprofile].BackupSearchForAlternateTitle

	if mediatype == "movie" && t.Dbmovie.ImdbID != "" {
		t.searchMedia(mediatype, t.Dbmovie.ImdbID, false, mediaid, quality, indexer, "", 0, 0, cats, dl)
	} else if mediatype == "serie" && t.Dbserie.ThetvdbID != 0 {
		t.searchMedia(mediatype, strconv.Itoa(t.Dbserie.ThetvdbID), false, mediaid, quality, indexer, "", seasonint, episodeint, cats, dl)
	}
	if len(dl.Nzbs) >= 1 {
		logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
		releasefound = true

		if checkfirst { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound {
			logger.Log.GlobalLogger.Debug("Break Indexer loop - entry found", zap.String("title", name))
			return true
		}
	}
	if !releasefound && checktitle && titlesearch { //config.Cfg.Quality[qualityprofile].BackupSearchForTitle
		searchfor = strings.Replace(name, "(", "", -1)
		searchfor = strings.Replace(searchfor, ")", "", -1)
		if mediatype == "movie" {
			if t.Dbmovie.Year != 0 {
				searchfor += " " + strconv.Itoa(t.Dbmovie.Year)
			}
			t.searchMedia(mediatype, t.Dbmovie.ImdbID, true, mediaid, quality, indexer, searchfor, 0, 0, cats, dl)
		} else if mediatype == "serie" {
			t.searchMedia(mediatype, strconv.Itoa(t.Dbserie.ThetvdbID), true, mediaid, quality, indexer, searchfor+" "+t.Dbserie.Identifiedby, seasonint, episodeint, cats, dl)
		}
		if len(dl.Nzbs) >= 1 {
			logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
			releasefound = true

			if checkfirst { //config.Cfg.Quality[qualityprofile].CheckUntilFirstFound
				logger.Log.GlobalLogger.Debug("Break Indexer loop - entry found", zap.String("title", name))
				return true
			}
		}
	}
	if !releasefound && checkalttitle && titlesearch { //config.Cfg.Quality[qualityprofile].BackupSearchForAlternateTitle
		var yearstr, tvdbstr string
		if mediatype == "movie" {
			yearstr = strconv.Itoa(t.Dbmovie.Year)
		} else {
			tvdbstr = strconv.Itoa(t.Dbserie.ThetvdbID)
		}
		alttitle := database.QueryStaticStringArray(querytitle, false, 0, dbid, name)
		for idxalt := range alttitle {
			if alttitle[idxalt] == "" {
				continue
			}
			if mediatype == "movie" {

				searchfor = alttitle[idxalt]
				if yearstr != "0" {
					searchfor += " (" + yearstr + ")"
				}
				t.searchMedia(mediatype, t.Dbmovie.ImdbID, true, mediaid, quality, indexer, searchfor, 0, 0, cats, dl)
			} else if mediatype == "serie" {
				searchfor = alttitle[idxalt] + " " + t.Dbserie.Identifiedby
				t.searchMedia(mediatype, tvdbstr, true, mediaid, quality, indexer, searchfor, seasonint, episodeint, cats, dl)
			}
			if len(dl.Nzbs) >= 1 {
				logger.Log.GlobalLogger.Debug("Indexer loop - entries found", zap.Int("entries", len(dl.Nzbs)))
				releasefound = true

				if checkfirst {
					logger.Log.GlobalLogger.Debug("Break Indexer loop - entry found", zap.String("Title", name))
					break
				}
			}
		}
		if len(dl.Nzbs) >= 1 && checkfirst {
			logger.Log.GlobalLogger.Debug("Break Indexer loop - entry found", zap.String("Title", name))
			return true
		}
	}
	return true
}

func (s *Searcher) SeriesSearch(episodeid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	var err error
	s.SerieEpisode, err = database.GetSerieEpisodes(&database.Query{Where: "id = ?"}, episodeid)
	if err != nil {
		return nil, err
	}

	s.Serie, _ = database.GetSeries(&database.Query{Where: "id = ?"}, s.SerieEpisode.SerieID)
	s.Dbserie, _ = database.GetDbserie(&database.Query{Where: "id = ?"}, s.SerieEpisode.DbserieID)
	s.Dbserieepisode, _ = database.GetDbserieEpisodes(&database.Query{Where: "id = ?"}, s.SerieEpisode.DbserieEpisodeID)

	s.SearchGroupType = "series"
	if s.SerieEpisode.DontSearch && !forceDownload {
		logger.Log.GlobalLogger.Debug("Search not wanted: ")
		return nil, errSearchDisabled
	}

	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		logger.Log.GlobalLogger.Error("Quality for Episode: " + strconv.Itoa(int(episodeid)) + " not found")
		return nil, errNoQuality
	}
	s.Quality = s.SerieEpisode.QualityProfile
	s.MinimumPriority = GetHighestEpisodePriorityByFiles(episodeid, s.Cfg, s.SerieEpisode.QualityProfile)

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
		database.UpdateColumnStatic("Update serie_episodes set lastscan = ? where id = ?", sql.NullTime{Time: time.Now(), Valid: true}, episodeid)
	}

	if dl == nil {
		return nil, errors.New("no results")
	}
	return dl, nil
}

func (s *Searcher) mediasearchindexerloopseries(processedindexer *int, titlesearch bool) (dl *SearchResults) {
	return s.queryindexers(s.SerieEpisode.QualityProfile, "serie", false, 0, 0, false, processedindexer, titlesearch)
}

func (s *Searcher) queryindexers(quality string, searchaction string, fetchall bool, thetvdb_id int, season int, useseason bool, processedindexer *int, titlesearch bool) *SearchResults {
	dl := new(SearchResults)
	workergroup := logger.WorkerPools["Indexer"].Group()
	for idx := range config.Cfg.Quality[quality].Indexer {
		index := config.Cfg.Quality[quality].Indexer[idx].Template_indexer
		workergroup.Submit(func() {
			var ok bool
			switch searchaction {
			case "movie", "serie":
				ok = s.mediasearchindexer(searchaction, quality, index, titlesearch, dl)
			case "rss":
				ok = s.rsssearchindexer(quality, index, fetchall, dl)
			case "rssserie":
				ok = s.rssqueryseriesindexer(quality, index, thetvdb_id, season, useseason, dl)
			}

			if ok {
				*processedindexer += 1
			}
		})
	}
	workergroup.Wait()
	return dl
}

func (s *Searcher) initIndexer(quality string, indexer string, rssapi string, nzbIndexer *apiexternal.NzbIndexer) (string, error) {
	if !config.ConfigCheck("indexer_" + indexer) {
		return "", errNoIndexer
	}

	if !(strings.EqualFold(config.Cfg.Indexers[indexer].IndexerType, "newznab")) {
		return "", errors.New("indexer Type Wrong")
	}
	if !config.Cfg.Indexers[indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", errIndexerDisabled
	} else if !config.Cfg.Indexers[indexer].Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", errIndexerDisabled
	}

	userid, _ := strconv.Atoi(config.Cfg.Indexers[indexer].Userid)

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[indexer].Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fails - waiting till ", zap.Duration("waitfor", waitfor), zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", errIndexerDisabled
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", s.Cfg, s.Quality, config.Cfg.Indexers[indexer].Url)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                     config.Cfg.Indexers[indexer].Url,
		Apikey:                  config.Cfg.Indexers[indexer].Apikey,
		UserID:                  userid,
		SkipSslCheck:            config.Cfg.Indexers[indexer].DisableTLSVerify,
		Addquotesfortitlequery:  config.Cfg.Indexers[indexer].Addquotesfortitlequery,
		Additional_query_params: config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "Additional_query_params"),
		LastRssId:               lastindexerid,
		RssDownloadAll:          config.Cfg.Indexers[indexer].RssDownloadAll,
		Customapi:               config.Cfg.Indexers[indexer].Customapi,
		Customurl:               config.Cfg.Indexers[indexer].Customurl,
		Customrssurl:            config.Cfg.Indexers[indexer].Customrssurl,
		Customrsscategory:       config.Cfg.Indexers[indexer].Customrsscategory,
		Limitercalls:            config.Cfg.Indexers[indexer].Limitercalls,
		Limiterseconds:          config.Cfg.Indexers[indexer].Limiterseconds,
		LimitercallsDaily:       config.Cfg.Indexers[indexer].LimitercallsDaily,
		TimeoutSeconds:          config.Cfg.Indexers[indexer].TimeoutSeconds,
		MaxAge:                  config.Cfg.Indexers[indexer].MaxAge,
		OutputAsJson:            config.Cfg.Indexers[indexer].OutputAsJson}

	return config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "Categories_indexer"), nil
}

func (s *Searcher) initIndexerUrlCat(quality string, indexer string, rssapi string) (string, string, error) {
	if !config.ConfigCheck("indexer_" + indexer) {
		return "", "", errNoIndexer
	}

	if !(strings.EqualFold(config.Cfg.Indexers[indexer].IndexerType, "newznab")) {
		return "", "", errors.New("indexer Type Wrong")
	}
	if !config.Cfg.Indexers[indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", "", errIndexerDisabled
	} else if !config.Cfg.Indexers[indexer].Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", "", errIndexerDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[indexer].Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled", zap.Duration("waitfor", waitfor), zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return "", "", errIndexerDisabled
	}

	return config.Cfg.Indexers[indexer].Url, config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "Categories_indexer"), nil
}

func (s *Searcher) initNzbIndexer(quality string, indexer string, rssapi string, nzbIndexer *apiexternal.NzbIndexer) error {
	if !config.ConfigCheck("indexer_" + indexer) {
		return errNoIndexer
	}

	if !(strings.EqualFold(config.Cfg.Indexers[indexer].IndexerType, "newznab")) {
		return errors.New("indexer type wrong")
	}
	if !config.Cfg.Indexers[indexer].Rssenabled && strings.EqualFold(rssapi, "rss") {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return errIndexerDisabled
	} else if !config.Cfg.Indexers[indexer].Enabled {
		//logger.Log.GlobalLogger.Debug("Indexer disabled", zap.String("Indexer", config.Cfg.Indexers[indexer].Name))
		return errIndexerDisabled
	}

	userid, _ := strconv.Atoi(config.Cfg.Indexers[indexer].Userid)

	if ok, _ := apiexternal.NewznabCheckLimiter(config.Cfg.Indexers[indexer].Url); !ok {
		//logger.Log.GlobalLogger.Debug("Indexer temporarily disabled due to fails - waiting till ", zap.Duration("waitfor", waitfor), zap.String("indexer", config.Cfg.Indexers[indexer].Name))
		return errIndexerDisabled
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", s.Cfg, s.Quality, config.Cfg.Indexers[indexer].Url)
	}

	*nzbIndexer = apiexternal.NzbIndexer{
		URL:                     config.Cfg.Indexers[indexer].Url,
		Apikey:                  config.Cfg.Indexers[indexer].Apikey,
		UserID:                  userid,
		SkipSslCheck:            config.Cfg.Indexers[indexer].DisableTLSVerify,
		Addquotesfortitlequery:  config.Cfg.Indexers[indexer].Addquotesfortitlequery,
		Additional_query_params: config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "Additional_query_params"),
		LastRssId:               lastindexerid,
		RssDownloadAll:          config.Cfg.Indexers[indexer].RssDownloadAll,
		Customapi:               config.Cfg.Indexers[indexer].Customapi,
		Customurl:               config.Cfg.Indexers[indexer].Customurl,
		Customrssurl:            config.Cfg.Indexers[indexer].Customrssurl,
		Customrsscategory:       config.Cfg.Indexers[indexer].Customrsscategory,
		Limitercalls:            config.Cfg.Indexers[indexer].Limitercalls,
		Limiterseconds:          config.Cfg.Indexers[indexer].Limiterseconds,
		LimitercallsDaily:       config.Cfg.Indexers[indexer].LimitercallsDaily,
		TimeoutSeconds:          config.Cfg.Indexers[indexer].TimeoutSeconds,
		MaxAge:                  config.Cfg.Indexers[indexer].MaxAge,
		OutputAsJson:            config.Cfg.Indexers[indexer].OutputAsJson}

	return nil
}

func failedindexer(failed string) {
	// database.UpsertNamed("indexer_fails",
	// 	[]string{"indexer", "last_fail"},
	// 	database.IndexerFail{Indexer: failed, LastFail: sql.NullTime{Time: time.Now(), Valid: true}},
	// 	&database.Query{Where: "indexer = ?"}, failed,
	// 	"indexer = :indexer")
}

func addsearched(dl *SearchResults, searched string) bool {
	for idx := range dl.Searched {
		if dl.Searched[idx] == searched {
			return true
		}
	}
	dl.Searched = append(dl.Searched, searched)
	return false
}

func (s *Searcher) searchMedia(mediatype string, searchid string, searchtitle bool, id uint, quality string, indexer string, title string, season int, episode int, cats string, dl *SearchResults) bool {
	searched := indexer + quality + strconv.Itoa(int(id)) + title + strconv.FormatBool(searchtitle)
	if mediatype == "serie" {
		searched += strconv.Itoa(season) + strconv.Itoa(episode)
	}
	if addsearched(dl, searched) {
		return true
	}
	if !config.ConfigCheck("quality_" + quality) {
		logger.Log.GlobalLogger.Error("Quality for: " + strconv.Itoa(int(id)) + " not found")
		return false
	}
	var nzbindexer apiexternal.NzbIndexer
	erri := s.initNzbIndexer(quality, indexer, "api", &nzbindexer)
	if erri != nil {
		if erri == errIndexerDisabled {
			return true
		}
		logger.Log.GlobalLogger.Debug("Skipped Indexer", zap.String("indexer", indexer), zap.Error(erri))
		return false
	}

	var nzbs *apiexternal.NZBArr //= make([]parser.NZB, 0, 20)
	var nzberr error

	backuptitlesearch := config.Cfg.Quality[quality].BackupSearchForTitle
	usetitlesearch := false
	if searchtitle && title != "" && backuptitlesearch {
		usetitlesearch = true
	}
	if !searchtitle {
		if mediatype == "movie" && s.Dbmovie.ImdbID != "" {
			nzbs, _, nzberr = apiexternal.QueryNewznabMovieImdb(&nzbindexer, strings.Trim(s.Dbmovie.ImdbID, "t"), cats)
		} else if mediatype == "movie" && s.Dbmovie.ImdbID == "" && backuptitlesearch {
			usetitlesearch = true
		} else if mediatype == "serie" && s.Dbserie.ThetvdbID != 0 {
			nzbs, _, nzberr = apiexternal.QueryNewznabTvTvdb(&nzbindexer, s.Dbserie.ThetvdbID, cats, season, episode, true, true)
		} else if mediatype == "serie" && s.Dbserie.ThetvdbID == 0 && backuptitlesearch {
			usetitlesearch = true
		}
	}
	if usetitlesearch {
		nzbs, _, nzberr = apiexternal.QueryNewznabQuery(&nzbindexer, title, cats, "search")
	}

	if nzberr != nil {
		logger.Log.GlobalLogger.Error("Newznab Search failed", zap.String("title", title), zap.String("indexer", nzbindexer.URL), zap.Error(nzberr))
		failedindexer(nzbindexer.URL)
		return false
	} else {
		defer nzbs.Close()
		s.parseentries(nzbs, dl, quality, indexer, "", false)
	}
	return true
}

func (s *Searcher) parseentries(nzbs *apiexternal.NZBArr, dl *SearchResults, quality string, indexer string, listname string, addfound bool) {
	if len((nzbs.Arr)) >= 1 {
		if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "Template_regex")) {
			dl.Rejected = make([]apiexternal.Nzbwithprio, len(nzbs.Arr))
			for idx := range nzbs.Arr {
				nzbs.Arr[idx].Denied = true
				nzbs.Arr[idx].Reason = "Denied by Regex"
				dl.Rejected[idx] = nzbs.Arr[idx]
			}
		} else {
			if len(dl.Rejected) == 0 {
				dl.Rejected = make([]apiexternal.Nzbwithprio, 0, len(nzbs.Arr))
			}
			s.convertnzbs(quality, indexer, nzbs, listname, addfound, dl)

			if len(dl.Nzbs) > 1 {
				sort.Slice(dl.Nzbs, func(i, j int) bool {
					return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
				})
			}
		}
	}
}

func allowMovieImport(imdb string, cfg string, listname string) bool {
	if !config.ConfigCheck("list_" + config.Cfg.Media[cfg].ListsMap[listname].Template_list) {
		return false
	}
	var countergenre int

	if config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].MinVotes != 0 {
		countergenre, _ = database.ImdbCountRowsStatic("select count() from imdb_ratings where tconst = ? and num_votes < ?", imdb, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].MinVotes)
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error vote count too low for ", zap.String("imdb", imdb))
			return false
		}
	}
	if config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].MinRating != 0 {
		countergenre, _ = database.ImdbCountRowsStatic("select count() from imdb_ratings where tconst = ? and average_rating < ?", imdb, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].MinRating)
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Debug("error average vote too low for ", zap.String("imdb", imdb))
			return false
		}
	}
	excludebygenre := false
	for idxgenre := range config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Excludegenre {
		countergenre, _ = database.ImdbCountRowsStatic("select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Excludegenre[idxgenre])
		if countergenre >= 1 {
			excludebygenre = true
			logger.Log.GlobalLogger.Debug("error excluded genre ", zap.String("excluded", config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Excludegenre[idxgenre]), zap.String("imdb", imdb))
			break
		}
	}
	if excludebygenre {
		return false
	}
	includebygenre := false
	for idxgenre := range config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Includegenre {
		countergenre, _ = database.ImdbCountRowsStatic("select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Includegenre[idxgenre])
		if countergenre >= 1 {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Includegenre) >= 1 {
		logger.Log.GlobalLogger.Debug("error included genre not found ", zap.String("imdb", imdb))
		return false
	}
	return true
}

func GetHighestMoviePriorityByFiles(movieid uint, cfg string, qualityTemplate string) (minPrio int) {
	foundfiles := database.QueryStaticIntArray("select id as num from movie_files where movie_id = ?", 0, movieid)

	var prio int
	for idx := range foundfiles {
		prio = GetMovieDBPriorityById(uint(foundfiles[idx]), cfg, qualityTemplate)
		if minPrio == 0 {
			minPrio = prio
		} else {
			if minPrio < prio {
				minPrio = prio
			}
		}
	}
	return minPrio
}

func GetHighestEpisodePriorityByFiles(episodeid uint, cfg string, qualityTemplate string) int {
	foundfiles := database.QueryStaticIntArray("select id as num from serie_episode_files where serie_episode_id = ?", 0, episodeid)

	minPrio := 0
	var prio int
	for idx := range foundfiles {
		prio = GetSerieDBPriorityById(uint(foundfiles[idx]), cfg, qualityTemplate)
		if minPrio == 0 {
			minPrio = prio
		} else {
			if minPrio < prio {
				minPrio = prio
			}
		}
	}
	return minPrio
}

func GetSerieDBPriorityById(episodefileid uint, cfg string, qualityTemplate string) int {
	serieepisodefile, err := database.GetSerieEpisodeFiles(&database.Query{Where: "id = ?"}, episodefileid)
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

	parser.GetIDPriorityMap(&m, cfg, qualityTemplate, true)
	prio := m.Priority
	return prio
}

func GetMovieDBPriorityById(moviefileid uint, cfg string, qualityTemplate string) int {
	moviefile, err := database.GetMovieFiles(&database.Query{Where: "id = ?"}, moviefileid)
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
	parser.GetIDPriorityMap(&m, cfg, qualityTemplate, true)
	prio := m.Priority
	return prio
}
