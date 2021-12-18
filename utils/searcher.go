package utils

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
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/newznab"
	"github.com/remeh/sizedwaitgroup"
)

func SearchMovieMissing(configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	var scandatepre int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.Data[0].Template_path, &cfg_path)

		scaninterval = cfg_path.MissingScanInterval
		scandatepre = cfg_path.MissingScanReleaseDatePre
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}

	argslist := []interface{}{}
	for idx := range lists {
		argslist = append(argslist, lists[idx])
	}

	argsscan := append(argslist, scantime)
	qu := database.Query{Select: "movies.*", InnerJoin: "dbmovies on dbmovies.id=movies.dbmovie_id", OrderBy: "movies.Lastscan asc"}
	if scaninterval != 0 {
		qu.Where = "movies.missing = 1 AND movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (movies.lastscan is null or movies.Lastscan < ?)"
		qu.WhereArgs = argsscan
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			qu.WhereArgs = append(argsscan, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	} else {
		qu.Where = "movies.missing = 1 AND movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = argslist
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			qu.WhereArgs = append(argslist, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingmovies, _ := database.QueryMovies(qu)

	// searchnow := NewSearcher(configEntry, list)
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()
		SearchMovieSingle(missingmovies[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}

func SearchMovieSingle(movie database.Movie, configEntry config.MediaTypeConfig, titlesearch bool) {
	searchtype := "missing"
	if !movie.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(configEntry, movie.QualityProfile)
	searchresults := searchnow.MovieSearch(movie, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := NewDownloader(configEntry, searchtype)
		downloadnow.SetMovie(movie)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}

func SearchMovieUpgrade(configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.Data[0].Template_path, &cfg_path)

		scaninterval = cfg_path.UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}
	argslist := []interface{}{}
	for idx := range lists {
		argslist = append(argslist, lists[idx])
	}
	argsscan := append(argslist, scantime)

	qu := database.Query{}
	if scaninterval != 0 {
		qu.Where = "quality_reached = 0 and missing = 0 AND listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (lastscan is null or Lastscan < ?)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "quality_reached = 0 and missing = 0 AND listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = argslist
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.OrderBy = "Lastscan asc"
	missingmovies, _ := database.QueryMovies(qu)

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()
		SearchMovieSingle(missingmovies[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}

func SearchSerieSingle(serie database.Serie, configEntry config.MediaTypeConfig, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	episodes, _ := database.QuerySerieEpisodes(database.Query{Where: "serie_id = ?", WhereArgs: []interface{}{serie.ID}})
	for idx := range episodes {
		swg.Add()
		SearchSerieEpisodeSingle(episodes[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}

func SearchSerieSeasonSingle(serie database.Serie, season string, configEntry config.MediaTypeConfig, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	episodes, _ := database.QuerySerieEpisodes(database.Query{Where: "serie_id = ? and dbserie_episode_id IN (Select id from dbserie_episodes where Season=?)", WhereArgs: []interface{}{serie.ID, season}})
	for idx := range episodes {
		swg.Add()
		SearchSerieEpisodeSingle(episodes[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}

func SearchSerieEpisodeSingle(row database.SerieEpisode, configEntry config.MediaTypeConfig, titlesearch bool) {
	searchtype := "missing"
	if !row.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(configEntry, row.QualityProfile)
	searchresults := searchnow.SeriesSearch(row, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := NewDownloader(configEntry, searchtype)
		downloadnow.SetSeriesEpisode(row)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}
func SearchSerieMissing(configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	var scandatepre int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.Data[0].Template_path, &cfg_path)

		scaninterval = cfg_path.MissingScanInterval
		scandatepre = cfg_path.MissingScanReleaseDatePre
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
		logger.Log.Debug("Search before scan: ", scantime)
	}
	argslist := []interface{}{}
	for idx := range lists {
		argslist = append(argslist, lists[idx])
	}
	argsscan := append(argslist, scantime)

	qu := database.Query{Select: "Serie_episodes.*", OrderBy: "Lastscan asc", InnerJoin: "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?)"
		qu.WhereArgs = argsscan
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			qu.WhereArgs = append(argsscan, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	} else {
		qu.Where = "serie_episodes.missing = 1 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = argslist
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			qu.WhereArgs = append(argslist, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingepisode, _ := database.QuerySerieEpisodes(qu)

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingepisode {
		dbepi, dbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, dbserie_id", Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		if dbepierr != nil {
			continue
		}
		epicount, _ := database.CountRows("dbserie_episodes", database.Query{Where: "identifier=? COLLATE NOCASE and dbserie_id=?", WhereArgs: []interface{}{dbepi.Identifier, dbepi.DbserieID}})
		if epicount >= 2 {
			continue
		}
		swg.Add()
		SearchSerieEpisodeSingle(missingepisode[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}
func SearchSerieUpgrade(configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.Data[0].Template_path, &cfg_path)

		scaninterval = cfg_path.UpgradeScanInterval
	}
	scantime := time.Now()
	if scaninterval != 0 {
		scantime = scantime.AddDate(0, 0, 0-scaninterval)
	}
	args := []interface{}{}
	for idx := range lists {
		args = append(args, lists[idx])
	}
	argsscan := append(args, scantime)

	qu := database.Query{Select: "Serie_episodes.*", OrderBy: "Lastscan asc", InnerJoin: "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = args
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingepisode, _ := database.QuerySerieEpisodes(qu)

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingepisode {
		dbepi, dbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, dbserie_id", Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		if dbepierr != nil {
			continue
		}
		epicount, _ := database.CountRows("dbserie_episodes", database.Query{Where: "identifier=? COLLATE NOCASE and dbserie_id=?", WhereArgs: []interface{}{dbepi.Identifier, dbepi.DbserieID}})
		if epicount >= 2 {
			continue
		}
		swg.Add()
		SearchSerieEpisodeSingle(missingepisode[idx], configEntry, titlesearch)
		swg.Done()
	}
	swg.Wait()
}

func SearchSerieRSS(configEntry config.MediaTypeConfig, quality string) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(configEntry, quality)
	searchresults := searchnow.SearchRSS("series", false)
	downloaded := make(map[uint]bool, len(searchresults.Nzbs))
	for idx := range searchresults.Nzbs {
		if _, nok := downloaded[searchresults.Nzbs[idx].Nzbepisode.ID]; nok {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded[searchresults.Nzbs[idx].Nzbepisode.ID] = true
		downloadnow := NewDownloader(configEntry, "rss")
		downloadnow.SetSeriesEpisode(searchresults.Nzbs[idx].Nzbepisode)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

func SearchMovieRSS(configEntry config.MediaTypeConfig, quality string) {
	logger.Log.Debug("Get Rss Movie List")

	searchnow := NewSearcher(configEntry, quality)
	searchresults := searchnow.SearchRSS("movie", false)
	downloaded := make(map[uint]bool, len(searchresults.Nzbs))
	for idx := range searchresults.Nzbs {
		if _, nok := downloaded[searchresults.Nzbs[idx].Nzbmovie.ID]; nok {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded[searchresults.Nzbs[idx].Nzbmovie.ID] = true
		downloadnow := NewDownloader(configEntry, "rss")
		downloadnow.SetMovie(searchresults.Nzbs[idx].Nzbmovie)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

type searchResults struct {
	Nzbs     []Nzbwithprio
	Rejected []Nzbwithprio
}

type Searcher struct {
	ConfigEntry       config.MediaTypeConfig
	Quality           string
	SearchGroupType   string //series, movies
	SearchActionType  string //missing,upgrade,rss
	Indexer           config.QualityIndexerConfig
	Indexercategories []int
	AlternateNames    []string
	MinimumPriority   int

	//nzb
	Nzbindexer apiexternal.NzbIndexer

	//Movies
	Movie   database.Movie
	Dbmovie database.Dbmovie

	//Series
	SerieEpisode   database.SerieEpisode
	Dbserie        database.Dbserie
	Dbserieepisode database.DbserieEpisode

	List       config.ListsConfig
	NzbsDenied []Nzbwithprio
	Nzbs       []Nzbwithprio
}

func NewSearcher(configEntry config.MediaTypeConfig, quality string) Searcher {
	return Searcher{
		ConfigEntry: configEntry,
		Quality:     quality,
	}
}

//searchGroupType == movie || series
func (s *Searcher) SearchRSS(searchGroupType string, fetchall bool) searchResults {

	if !config.ConfigCheck("quality_" + s.Quality) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.Quality, &cfg_quality)

	if !config.ConfigCheck("indexer_" + cfg_quality.Indexer[0].Template_indexer) {
		return searchResults{}
	}
	var cfg_indexer config.IndexersConfig
	config.ConfigGet("indexer_"+cfg_quality.Indexer[0].Template_indexer, &cfg_indexer)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	lists := make([]string, 0, len(s.ConfigEntry.Lists))
	for idxlisttest := range s.ConfigEntry.Lists {
		lists = append(lists, s.ConfigEntry.Lists[idxlisttest].Name)
	}
	if len(lists) == 0 {
		logger.Log.Error("lists empty for config ", searchGroupType, " ", s.ConfigEntry.Name)
		return searchResults{}
	}
	for idx := range cfg_quality.Indexer {
		erri := s.InitIndexer(cfg_quality.Indexer[idx], "rss")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", cfg_quality.Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		if fetchall {
			s.Nzbindexer.LastRssId = ""
		}
		s.Indexer = cfg_quality.Indexer[idx]
		if cfg_indexer.MaxRssEntries == 0 {
			cfg_indexer.MaxRssEntries = 10
		}
		if cfg_indexer.RssEntriesloop == 0 {
			cfg_indexer.RssEntriesloop = 2
		}
		nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast([]apiexternal.NzbIndexer{s.Nzbindexer}, cfg_indexer.MaxRssEntries, s.Indexercategories, cfg_indexer.RssEntriesloop)
		if nzberr != nil {
			logger.Log.Error("Newznab RSS Search failed ", cfg_quality.Indexer[idx].Template_indexer)
			failedindexer(failed)
		} else {
			if !fetchall {
				addrsshistory(lastids, s.Quality, s.ConfigEntry.Name)
			}
			logger.Log.Debug("Search RSS ended - found entries: ", len(nzbs))
			if len(nzbs) >= 1 {
				if strings.ToLower(s.SearchGroupType) == "movie" {
					s.NzbsToNzbsPrio(nzbs)
					s.FilterNzbTitleLength()
					s.FilterNzbSize()
					s.FilterNzbHistory()
					s.SetDataField(lists, false)
					s.FilterNzbRegex()
					s.NzbParse()
					s.NzbCheckTitle()
					s.NzbCheckQualityWanted()
					s.NzbCheckMinPrio()
				} else {
					s.NzbsToNzbsPrio(nzbs)
					s.FilterNzbTitleLength()
					s.FilterNzbSize()
					s.FilterNzbHistory()
					s.SetDataField(lists, false)
					s.FilterNzbRegex()
					s.NzbParse()
					//s.NzbCheckYear(cfg_quality, strconv.Itoa(s.Dbmovie.Year))
					s.NzbCheckTitle()
					s.NzbCheckEpisodeWanted()
					s.NzbCheckQualityWanted()
					s.NzbCheckMinPrio()

				}
			}
		}
	}
	if len(s.Nzbs) > 1 {
		sort.Slice(s.Nzbs, func(i, j int) bool {
			return s.Nzbs[i].Prio > s.Nzbs[j].Prio
		})
	}
	if len(s.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return searchResults{Nzbs: s.Nzbs, Rejected: s.NzbsDenied}
}

func (s *Searcher) NzbsToNzbsPrio(nzbs []newznab.NZB) {
	s.Nzbs = make([]Nzbwithprio, 0, len(nzbs))
	s.NzbsDenied = make([]Nzbwithprio, 0, len(nzbs))
	for idx := range nzbs {
		s.Nzbs = append(s.Nzbs, Nzbwithprio{
			Indexer:   s.Indexer.Template_indexer,
			NZB:       nzbs[idx],
			ParseInfo: ParseInfo{},
		})
	}
}
func (s *Searcher) FilterNzbTitleLength() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if len(strings.Trim(Path(s.Nzbs[idx].NZB.Title, false), " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", s.Nzbs[idx].NZB.Title)
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Title too short"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
			continue
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}
func (s *Searcher) FilterNzbSize() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if filter_size_nzbs(s.ConfigEntry, s.Indexer, s.Nzbs[idx].NZB) {
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Wrong size"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
			continue
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

func (s *Searcher) FilterNzbHistory() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			countertitle, _ := database.CountRows("movie_histories", database.Query{Where: "url = ? COLLATE NOCASE", WhereArgs: []interface{}{s.Nzbs[idx].NZB.DownloadURL}})
			if countertitle >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", s.Nzbs[idx].NZB.Title)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Already downloaded"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
			if s.Indexer.History_check_title {
				countertitle, _ = database.CountRows("movie_histories", database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{s.Nzbs[idx].NZB.Title}})
				if countertitle >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", s.Nzbs[idx].NZB.Title)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Already downloaded"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
		} else {
			countertitle, _ := database.CountRows("serie_episode_histories", database.Query{Where: "url = ? COLLATE NOCASE", WhereArgs: []interface{}{s.Nzbs[idx].NZB.DownloadURL}})
			if countertitle >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", s.Nzbs[idx].NZB.Title)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Already downloaded"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
			if s.Indexer.History_check_title {
				countertitle, _ = database.CountRows("serie_episode_histories", database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{s.Nzbs[idx].NZB.Title}})
				if countertitle >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", s.Nzbs[idx].NZB.Title)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Already downloaded"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//Needs DbMovie imdbid or dbserie thetvdbid
func (s *Searcher) CheckMatchingID() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			tempimdb := s.Nzbs[idx].NZB.IMDBID
			tempimdb = strings.TrimPrefix(tempimdb, "tt")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")

			wantedimdb := s.Dbmovie.ImdbID
			wantedimdb = strings.TrimPrefix(wantedimdb, "tt")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			if wantedimdb != tempimdb && len(wantedimdb) >= 1 && len(tempimdb) >= 1 {
				logger.Log.Debug("Skipped - Imdb not match: ", s.Nzbs[idx].NZB.Title, " - imdb in nzb: ", tempimdb, " imdb wanted: ", wantedimdb)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Imdbid not correct"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
		} else {
			if strconv.Itoa(s.Dbserie.ThetvdbID) != s.Nzbs[idx].NZB.TVDBID && s.Dbserie.ThetvdbID >= 1 && len(s.Nzbs[idx].NZB.TVDBID) >= 1 {
				logger.Log.Debug("Skipped - Tvdb not match: ", s.Nzbs[idx].NZB.Title, " - Tvdb in nzb: ", s.Nzbs[idx].NZB.TVDBID, " Tvdb wanted: ", s.Dbserie.ThetvdbID)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Tvdbid not correct"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//Needs s.Movie or s.SerieEpisode (for non RSS)
func (s *Searcher) SetDataField(lists []string, addifnotfound bool) {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			loopmovie := database.Movie{}
			loopdbmovie := database.Dbmovie{}
			if s.SearchActionType == "rss" {
				if s.Nzbs[idx].NZB.IMDBID != "" {
					var founddbmovie database.Dbmovie
					var founddbmovieerr error
					searchimdb := s.Nzbs[idx].NZB.IMDBID
					if !strings.HasPrefix(searchimdb, "tt") {
						searchimdb = "tt" + s.Nzbs[idx].NZB.IMDBID
					}
					founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{searchimdb}})

					if !strings.HasPrefix(s.Nzbs[idx].NZB.IMDBID, "tt") && founddbmovieerr != nil {
						searchimdb = "tt0" + s.Nzbs[idx].NZB.IMDBID
						founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{searchimdb}})
						if founddbmovieerr != nil {
							searchimdb = "tt00" + s.Nzbs[idx].NZB.IMDBID
							founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{searchimdb}})
							if founddbmovieerr != nil {
								searchimdb = "tt000" + s.Nzbs[idx].NZB.IMDBID
								founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{searchimdb}})
								if founddbmovieerr != nil {
									searchimdb = "tt0000" + s.Nzbs[idx].NZB.IMDBID
									founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{searchimdb}})
								}
							}
						}
					}
					if addifnotfound && strings.HasPrefix(s.Nzbs[idx].NZB.IMDBID, "tt") {
						var cfg_list config.MediaListsConfig
						for idxlist := range s.ConfigEntry.Lists {
							if s.ConfigEntry.Lists[idxlist].Name == lists[0] {
								cfg_list = s.ConfigEntry.Lists[idxlist]
								break
							}
						}
						if !AllowMovieImport(s.Nzbs[idx].NZB.IMDBID, s.List) {
							continue
						}

						sww := sizedwaitgroup.New(1)
						var dbmovie database.Dbmovie
						dbmovie.ImdbID = s.Nzbs[idx].NZB.IMDBID
						sww.Add()
						JobImportMovies(dbmovie, s.ConfigEntry, cfg_list, &sww)
						sww.Wait()
						founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{dbmovie.ImdbID}})
					}
					if founddbmovieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Movie: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Not Wanted DB Movie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					loopdbmovie = founddbmovie
					n, nerr := NewFileParser(s.Nzbs[idx].NZB.Title, false, "movie")
					if nerr == nil {
						logger.Log.Debug("Skipped - Error Parsing: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Error Parsing Movie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					n.StripTitlePrefixPostfix(s.Nzbs[idx].Quality)
					s.Nzbs[idx].ParseInfo = *n
					list, imdb := movieGetListFilter(lists, founddbmovie.ID, n.Year)
					if list != "" {
						s.Nzbs[idx].NZB.IMDBID = imdb
						getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{founddbmovie.ID, list}})
						loopmovie = getmovie
					} else {
						logger.Log.Debug("Skipped - Not Wanted Movie: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Not Wanted Movie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
				} else {
					n, errparse := NewFileParser(s.Nzbs[idx].NZB.Title, false, "movie")
					if errparse != nil {
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Error Parsing"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						logger.Log.Error("Error parsing: ", s.Nzbs[idx].NZB.Title, " error: ", errparse)
						continue
					}
					n.StripTitlePrefixPostfix(s.Nzbs[idx].Quality)
					s.Nzbs[idx].ParseInfo = *n
					list, imdb, entriesfound, dbmovie := movieFindListByTitle(n.Title, strconv.Itoa(n.Year), lists, "rss")
					if entriesfound >= 1 {
						s.Nzbs[idx].NZB.IMDBID = imdb
						loopdbmovie = dbmovie
						getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list}})
						loopmovie = getmovie
					} else {
						if addifnotfound {
							//Search imdb!
							_, imdbget, imdbfound := movieFindDbByTitle(n.Title, strconv.Itoa(n.Year), lists[0], true, "rss")
							if imdbfound == 0 {
								var cfg_list config.MediaListsConfig
								for idxlist := range s.ConfigEntry.Lists {
									if s.ConfigEntry.Lists[idxlist].Name == lists[0] {
										cfg_list = s.ConfigEntry.Lists[idxlist]
										break
									}
								}
								if !AllowMovieImport(s.Nzbs[idx].NZB.IMDBID, s.List) {
									continue
								}

								sww := sizedwaitgroup.New(1)
								var dbmovie database.Dbmovie
								dbmovie.ImdbID = imdbget
								sww.Add()
								JobImportMovies(dbmovie, s.ConfigEntry, cfg_list, &sww)
								sww.Wait()
								founddbmovie, _ := database.GetDbmovie(database.Query{Where: "imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{dbmovie.ImdbID}})
								loopdbmovie = founddbmovie
								getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{loopdbmovie.ID, lists[0]}})
								loopmovie = getmovie
							}
						} else {
							logger.Log.Debug("Skipped - Not Wanted DB Movie: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Not Wanted DB Movie"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
					}
				}
			} else {
				loopmovie = s.Movie
				loopdbmovie = s.Dbmovie
			}
			s.Nzbs[idx].Nzbmovie = loopmovie
			if !config.ConfigCheck("quality_" + loopmovie.QualityProfile) {
				continue
			}
			var cfg_quality config.QualityConfig
			config.ConfigGet("quality_"+loopmovie.QualityProfile, &cfg_quality)
			s.Nzbs[idx].Quality = cfg_quality

			s.MinimumPriority = getHighestMoviePriorityByFiles(loopmovie, s.ConfigEntry, cfg_quality)
			s.Nzbs[idx].MinimumPriority = s.MinimumPriority
			if s.MinimumPriority == 0 {
				//s.SearchActionType = "missing"
				if loopmovie.DontSearch {
					logger.Log.Debug("Skipped - Search disabled: ", loopdbmovie.Title)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Search Disabled"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			} else {
				//s.SearchActionType = "upgrade"
				if loopmovie.DontUpgrade {
					logger.Log.Debug("Skipped - Upgrade disabled: ", loopdbmovie.Title)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Upgrade Disabled"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
			dbmoviealt, _ := database.QueryDbmovieTitle(database.Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{loopdbmovie.ID}})
			s.Nzbs[idx].WantedAlternates = make([]string, 0, len(dbmoviealt))
			for idxalt := range dbmoviealt {
				s.Nzbs[idx].WantedAlternates = append(s.Nzbs[idx].WantedAlternates, dbmoviealt[idxalt].Title)
			}

			s.Nzbs[idx].WantedTitle = loopdbmovie.Title
		} else {
			loopepisode := database.SerieEpisode{}
			loopdbseries := database.Dbserie{}
			if s.SearchActionType == "rss" {
				var foundepisode database.SerieEpisode
				if len(s.Nzbs[idx].NZB.TVDBID) >= 1 {
					founddbserie, founddbserieerr := database.GetDbserie(database.Query{Where: "thetvdb_id = ?", WhereArgs: []interface{}{s.Nzbs[idx].NZB.TVDBID}})

					if founddbserieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Serie: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Unwanted Dbserie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					loopdbseries = founddbserie

					foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{founddbserie.ID}})
					s.AlternateNames = make([]string, 0, len(foundalternate))
					for idxalt := range foundalternate {
						s.AlternateNames = append(s.AlternateNames, foundalternate[idxalt].Title)
					}
					args := []interface{}{}
					args = append(args, founddbserie.ID)
					for idxlist := range lists {
						args = append(args, lists[idxlist])
					}
					foundserie, foundserieerr := database.GetSeries(database.Query{Select: "id", Where: "dbserie_id = ? and listname IN (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: args})
					if foundserieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted Serie: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Unwanted Serie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}

					var founddbepisode database.DbserieEpisode
					var founddbepisodeerr error
					if strings.EqualFold(founddbserie.Identifiedby, "date") {
						tempparse, _ := NewFileParser(s.Nzbs[idx].NZB.Title, true, "series")
						if tempparse.Date == "" {
							logger.Log.Debug("Skipped - Date wanted but not found: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Date not found"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
						tempparse.StripTitlePrefixPostfix(s.Nzbs[idx].Quality)
						s.Nzbs[idx].ParseInfo = *tempparse
						tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
						tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
						founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ?", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})
						if founddbepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted DB Episode: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Unwanted DB Episode"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
					} else {
						founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, s.Nzbs[idx].NZB.Season, s.Nzbs[idx].NZB.Episode}})
						if founddbepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted DB Episode: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Unwanted DB Episode"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
					}
					var foundepisodeerr error
					foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
					if foundepisodeerr != nil {
						logger.Log.Debug("Skipped - Not Wanted Episode: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Unwanted Episode"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					if foundepisode.DontSearch || foundepisode.DontUpgrade || (!foundepisode.Missing && foundepisode.QualityReached) {
						logger.Log.Debug("Skipped - Notwanted or Already reached: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Unwanted or reached"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					if !config.ConfigCheck("quality_" + foundepisode.QualityProfile) {
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Quality profile not found"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
					if foundepisode.QualityProfile == "" {
						foundepisode.QualityProfile = s.Quality
					}
					loopepisode = foundepisode
				} else {
					var foundserie database.Serie
					tempparse, _ := NewFileParser(s.Nzbs[idx].NZB.Title, true, "series")
					tempparse.StripTitlePrefixPostfix(s.Nzbs[idx].Quality)
					s.Nzbs[idx].ParseInfo = *tempparse
					titleyear := tempparse.Title + " (" + strconv.Itoa(tempparse.Year) + ")"
					seriestitle := ""
					matched := config.RegexSeriesTitle.FindStringSubmatch(s.Nzbs[idx].NZB.Title)
					if len(matched) >= 2 {
						seriestitle = matched[1]
					}
					for idxlist := range lists {
						series, entriesfound := FindSerieByParser(*tempparse, titleyear, seriestitle, lists[idxlist])
						if entriesfound >= 1 {
							foundserie = series
							break
						}
					}
					if foundserie.ID != 0 {
						founddbserie, _ := database.GetDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{foundserie.DbserieID}})

						loopdbseries = founddbserie
						var founddbepisode database.DbserieEpisode
						var founddbepisodeerr error
						if strings.EqualFold(founddbserie.Identifiedby, "date") {
							if tempparse.Date == "" {
								logger.Log.Debug("Skipped - Date wanted but not found: ", s.Nzbs[idx].NZB.Title)
								s.Nzbs[idx].Denied = true
								s.Nzbs[idx].Reason = "Unwanted Date"
								s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
								continue
							}
							tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
							tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
							founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})

							if founddbepisodeerr != nil {
								logger.Log.Debug("Skipped - Not Wanted DB Episode: ", s.Nzbs[idx].NZB.Title)
								s.Nzbs[idx].Denied = true
								s.Nzbs[idx].Reason = "Unwanted DB Episode"
								s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
								continue
							}
						} else {
							founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, s.Nzbs[idx].NZB.Season, s.Nzbs[idx].NZB.Episode}})
							if founddbepisodeerr != nil {
								logger.Log.Debug("Skipped - Not Wanted DB Episode: ", s.Nzbs[idx].NZB.Title)
								s.Nzbs[idx].Denied = true
								s.Nzbs[idx].Reason = "Unwanted DB Episode"
								s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
								continue
							}
						}
						var foundepisodeerr error
						foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
						if foundepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted Episode: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Unwanted Episode"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
						loopepisode = foundepisode
						if !config.ConfigCheck("quality_" + foundepisode.QualityProfile) {
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "Quality Profile unknown"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
						var cfg_quality config.QualityConfig
						config.ConfigGet("quality_"+foundepisode.QualityProfile, &cfg_quality)
						if cfg_quality.BackupSearchForTitle {
							s.Nzbs[idx].Nzbepisode = foundepisode
							foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{founddbserie.ID}})
							s.AlternateNames = []string{}
							for idxalt := range foundalternate {
								s.AlternateNames = append(s.AlternateNames, foundalternate[idxalt].Title)
							}
						} else {
							logger.Log.Debug("Skipped - no tvbdid: ", s.Nzbs[idx].NZB.Title)
							s.Nzbs[idx].Denied = true
							s.Nzbs[idx].Reason = "No Tvdb id"
							s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
							continue
						}
					} else {
						logger.Log.Debug("Skipped - Not Wanted Serie: ", s.Nzbs[idx].NZB.Title)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Unwanted Serie"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
				}
			} else {
				loopepisode = s.SerieEpisode
				loopdbseries = s.Dbserie
			}
			s.Nzbs[idx].Nzbepisode = loopepisode
			if !config.ConfigCheck("quality_" + loopepisode.QualityProfile) {
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Quality Profile unknown"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
			var cfg_quality config.QualityConfig
			config.ConfigGet("quality_"+loopepisode.QualityProfile, &cfg_quality)

			s.MinimumPriority = getHighestEpisodePriorityByFiles(loopepisode, s.ConfigEntry, cfg_quality)
			s.Nzbs[idx].MinimumPriority = s.MinimumPriority

			if s.MinimumPriority == 0 {
				//s.SearchActionType = "missing"
				if loopepisode.DontSearch {
					logger.Log.Debug("Skipped - Search disabled")
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Search Disabled"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			} else {
				//s.SearchActionType = "upgrade"
				if loopepisode.DontUpgrade {
					logger.Log.Debug("Skipped - Upgrade disabled")
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Upgrade Disabled"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
			s.Nzbs[idx].Quality = cfg_quality
			s.Nzbs[idx].WantedTitle = loopdbseries.Seriename
			dbseriealt, _ := database.QueryDbserieAlternates(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{loopepisode.DbserieID}})
			s.Nzbs[idx].WantedAlternates = make([]string, 0, len(dbseriealt))
			for idxalt := range dbseriealt {
				s.Nzbs[idx].WantedAlternates = append(s.Nzbs[idx].WantedAlternates, dbseriealt[idxalt].Title)
			}
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//SetDataField needs to run first
func (s *Searcher) FilterNzbRegex() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	if !config.ConfigCheck("regex_" + s.Indexer.Template_regex) {
		for idx := range s.Nzbs {
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Denied by Regex"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
		}
	} else {
		var cfg_regex config.RegexConfig
		config.ConfigGet("regex_"+s.Indexer.Template_regex, &cfg_regex)

		for idx := range s.Nzbs {
			regexdeny, regexrule := filter_regex_nzbs(cfg_regex, s.Nzbs[idx].NZB.Title, s.Nzbs[idx].WantedTitle, s.Nzbs[idx].WantedAlternates)
			if regexdeny {
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Denied by Regex: " + regexrule
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
			retnzb = append(retnzb, s.Nzbs[idx])
		}
	}
	s.Nzbs = retnzb
}

//SetDataField needs to run first
func (s *Searcher) NzbParse() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		var m *ParseInfo
		var err error
		if s.Nzbs[idx].ParseInfo.File == "" {
			if s.SearchGroupType == "series" {
				m, err = NewFileParser(s.Nzbs[idx].NZB.Title, true, "series")
			} else {
				m, err = NewFileParser(s.Nzbs[idx].NZB.Title, false, "movie")
			}
			if err != nil {
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Error Parsing"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				logger.Log.Error("Error parsing: ", s.Nzbs[idx].NZB.Title, " error: ", err)
				continue
			}
		} else {
			m = &s.Nzbs[idx].ParseInfo
		}
		if s.Nzbs[idx].ParseInfo.Priority == 0 {
			m.GetPriority(s.ConfigEntry, s.Nzbs[idx].Quality)
		}

		m.StripTitlePrefixPostfix(s.Nzbs[idx].Quality)
		s.Nzbs[idx].ParseInfo = *m
		s.Nzbs[idx].Prio = m.Priority
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//SetDataField needs to run first
func (s *Searcher) NzbCheckYear(yearstr string) {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			if s.Nzbs[idx].Quality.CheckYear && !s.Nzbs[idx].Quality.CheckYear1 && !strings.Contains(s.Nzbs[idx].NZB.Title, yearstr) && len(yearstr) >= 1 && yearstr != "0" {
				logger.Log.Debug("Skipped - unwanted year: ", s.Nzbs[idx].NZB.Title, " wanted ", yearstr)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "Wrong Year"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			} else {
				if s.Nzbs[idx].Quality.CheckYear1 && len(yearstr) >= 1 && yearstr != "0" {
					yearint, _ := strconv.Atoi(yearstr)
					if !strings.Contains(s.Nzbs[idx].NZB.Title, strconv.Itoa(yearint+1)) && !strings.Contains(s.Nzbs[idx].NZB.Title, strconv.Itoa(yearint-1)) && !strings.Contains(s.Nzbs[idx].NZB.Title, strconv.Itoa(yearint)) {
						logger.Log.Debug("Skipped - unwanted year: ", s.Nzbs[idx].NZB.Title, " wanted (+-1) ", yearint)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Wrong Year"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
				}
			}
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//Needs ParseInfo + SetDataField needs to run first
func (s *Searcher) NzbCheckTitle() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			if s.Nzbs[idx].Quality.CheckTitle {
				titlefound := false
				if s.Nzbs[idx].Quality.CheckTitle && checknzbtitle(s.Nzbs[idx].WantedTitle, s.Nzbs[idx].ParseInfo.Title) && len(s.Nzbs[idx].WantedTitle) >= 1 {
					titlefound = true
				}
				if !titlefound {
					alttitlefound := false
					for idxtitle := range s.Nzbs[idx].WantedAlternates {
						if checknzbtitle(s.Nzbs[idx].WantedAlternates[idxtitle], s.Nzbs[idx].ParseInfo.Title) {
							alttitlefound = true
							break
						}
					}
					if len(s.Nzbs[idx].WantedAlternates) >= 1 && !alttitlefound {
						logger.Log.Debug("Skipped - unwanted title and alternate: ", s.Nzbs[idx].NZB.Title, " wanted ", s.Nzbs[idx].WantedTitle, " ", s.Nzbs[idx].WantedAlternates)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Wrong Title"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
				}
				if len(s.Nzbs[idx].WantedAlternates) == 0 && !titlefound {
					logger.Log.Debug("Skipped - unwanted title: ", s.Nzbs[idx].NZB.Title, " wanted ", s.Nzbs[idx].WantedTitle)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Wrong Title"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
			retnzb = append(retnzb, s.Nzbs[idx])
		} else {
			if s.Nzbs[idx].Quality.CheckTitle {
				toskip := true
				if s.Nzbs[idx].WantedTitle != "" {
					if s.Nzbs[idx].Quality.CheckTitle && checknzbtitle(s.Nzbs[idx].WantedTitle, s.Nzbs[idx].ParseInfo.Title) && len(s.Nzbs[idx].WantedTitle) >= 1 {
						toskip = false
					}
					if toskip {
						for idxtitle := range s.Nzbs[idx].WantedAlternates {
							if checknzbtitle(s.Nzbs[idx].WantedAlternates[idxtitle], s.Nzbs[idx].ParseInfo.Title) {
								toskip = false
								break
							}
						}
					}
					if toskip {
						logger.Log.Debug("Skipped - seriename provided but not found ", s.Nzbs[idx].WantedTitle)
						s.Nzbs[idx].Denied = true
						s.Nzbs[idx].Reason = "Serie name not found"
						s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
						continue
					}
				} else {
					logger.Log.Debug("Skipped - seriename not provided or searchfortitle disabled")
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Serie name not provided"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			}
			retnzb = append(retnzb, s.Nzbs[idx])
		}
	}
	s.Nzbs = retnzb
}

//Needs the episode id (table serie_episodes) + SetDataField needs to run first
func (s *Searcher) NzbCheckEpisodeWanted() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {

		foundepi, foundepierr := database.GetSerieEpisodes(database.Query{Select: "dbserie_episode_id", Where: "id = ?", WhereArgs: []interface{}{s.Nzbs[idx].Nzbepisode.ID}})
		if foundepierr == nil {
			founddbepi, founddbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, season, episode", Where: "id = ?", WhereArgs: []interface{}{foundepi.DbserieEpisodeID}})
			if founddbepierr == nil {
				// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
				alt_identifier := strings.TrimPrefix(founddbepi.Identifier, "S")
				alt_identifier = strings.TrimPrefix(alt_identifier, "0")
				alt_identifier = strings.Replace(alt_identifier, "E", "x", -1)
				if strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(founddbepi.Identifier)) ||
					strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(strings.Replace(founddbepi.Identifier, "-", ".", -1))) ||
					strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(strings.Replace(founddbepi.Identifier, "-", " ", -1))) ||
					strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(alt_identifier)) ||
					strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(strings.Replace(alt_identifier, "-", ".", -1))) ||
					strings.Contains(strings.ToLower(s.Nzbs[idx].NZB.Title), strings.ToLower(strings.Replace(alt_identifier, "-", " ", -1))) {

					retnzb = append(retnzb, s.Nzbs[idx])
				} else {
					seasonvars := []string{"s" + founddbepi.Season + "e", "s0" + founddbepi.Season + "e", "s" + founddbepi.Season + " e", "s0" + founddbepi.Season + " e", founddbepi.Season + "x", founddbepi.Season + " x"}
					episodevars := []string{"e" + founddbepi.Episode, "e0" + founddbepi.Episode, "x" + founddbepi.Episode, "x0" + founddbepi.Episode}
					matchfound := false
					for idxseason := range seasonvars {
						if strings.HasPrefix(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), seasonvars[idxseason]) {
							for idxepisode := range episodevars {
								if strings.HasSuffix(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), episodevars[idxepisode]) {
									matchfound = true
									break
								}
								if strings.Contains(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), episodevars[idxepisode]+" ") {
									matchfound = true
									break
								}
								if strings.Contains(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), episodevars[idxepisode]+"-") {
									matchfound = true
									break
								}
								if strings.Contains(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), episodevars[idxepisode]+"e") {
									matchfound = true
									break
								}
								if strings.Contains(strings.ToLower(s.Nzbs[idx].ParseInfo.Identifier), episodevars[idxepisode]+"x") {
									matchfound = true
									break
								}
							}
							break
						}
					}
					if matchfound {
						retnzb = append(retnzb, s.Nzbs[idx])
						continue
					}
					logger.Log.Debug("Skipped - seriename provided dbepi found but identifier not match ", founddbepi.Identifier, " in: ", s.Nzbs[idx].NZB.Title)
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Wrong episode identifier"
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					continue
				}
			} else {
				logger.Log.Debug("Skipped - seriename provided dbepi not found", s.Nzbs[idx].WantedTitle)
				s.Nzbs[idx].Denied = true
				s.Nzbs[idx].Reason = "DB Episode not found"
				s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
				continue
			}
		} else {
			logger.Log.Debug("Skipped - seriename provided epi not found", s.Nzbs[idx].WantedTitle)
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Episode not found"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
			continue
		}
	}
	s.Nzbs = retnzb
}

//Needs ParseInfo + SetDatafield
func (s *Searcher) NzbCheckQualityWanted() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if !filter_test_quality_wanted(s.Nzbs[idx].Quality, &s.Nzbs[idx].ParseInfo, s.Nzbs[idx].NZB) {
			logger.Log.Debug("Skipped - unwanted quality: ", s.Nzbs[idx].NZB.Title)
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Wrong Quality"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
			continue
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

//Needs ParseInfo
func (s *Searcher) NzbCheckMinPrio() {
	retnzb := make([]Nzbwithprio, 0, len(s.Nzbs))
	for idx := range s.Nzbs {
		if s.Nzbs[idx].ParseInfo.Priority != 0 {
			if s.Nzbs[idx].MinimumPriority != 0 {
				if s.Nzbs[idx].ParseInfo.Priority <= s.Nzbs[idx].MinimumPriority {
					s.Nzbs[idx].Denied = true
					s.Nzbs[idx].Reason = "Prio lower. have: " + strconv.Itoa(s.Nzbs[idx].MinimumPriority)
					s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
					logger.Log.Debug("Skipped - Prio lower: ", s.Nzbs[idx].NZB.Title, " old prio ", s.Nzbs[idx].MinimumPriority, " found prio ", s.Nzbs[idx].ParseInfo.Priority)
					continue
				}
				logger.Log.Debug("ok - prio higher: ", s.Nzbs[idx].NZB.Title, " old prio ", s.Nzbs[idx].MinimumPriority, " found prio ", s.Nzbs[idx].ParseInfo.Priority)
			}
		} else {
			s.Nzbs[idx].Denied = true
			s.Nzbs[idx].Reason = "Prio not matched"
			s.NzbsDenied = append(s.NzbsDenied, s.Nzbs[idx])
			logger.Log.Debug("Skipped - Prio not matched: ", s.Nzbs[idx].NZB.Title)
			continue
		}
		retnzb = append(retnzb, s.Nzbs[idx])
	}
	s.Nzbs = retnzb
}

func (s *Searcher) GetRSSFeed(searchGroupType string, list config.MediaListsConfig) searchResults {

	if !config.ConfigCheck("quality_" + s.Quality) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.Quality, &cfg_quality)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	if !config.ConfigCheck("list_" + list.Template_list) {
		return searchResults{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)
	s.List = cfg_list

	var cfg_indexer config.QualityIndexerConfig
	for idx := range cfg_quality.Indexer {
		if cfg_quality.Indexer[idx].Template_indexer == cfg_list.Name {
			cfg_indexer = cfg_quality.Indexer[idx]
		}
	}
	if cfg_indexer.Template_regex == "" {
		return searchResults{}
	}

	var lastindexerid string
	indexrssid, _ := database.GetRssHistory(database.Query{Select: "last_id", Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{cfg_list.Name, s.Quality, ""}})
	lastindexerid = indexrssid.LastID

	indexer := apiexternal.NzbIndexer{Name: cfg_list.Name, Customrssurl: cfg_list.Url, LastRssId: lastindexerid}
	s.Indexer = cfg_indexer
	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast([]apiexternal.NzbIndexer{indexer}, cfg_list.Limit, []int{}, 1)
	if nzberr != nil {
		logger.Log.Error("Newznab RSS Search failed")
		failedindexer(failed)
	} else {
		addrsshistory(lastids, s.Quality, cfg_list.Name)
		if strings.ToLower(s.SearchGroupType) == "movie" {
			s.NzbsToNzbsPrio(nzbs)
			s.FilterNzbTitleLength()
			s.FilterNzbSize()
			s.FilterNzbHistory()
			s.SetDataField([]string{list.Name}, list.Addfound)
			s.FilterNzbRegex()
			s.NzbParse()
			s.NzbCheckTitle()
			s.NzbCheckQualityWanted()
			s.NzbCheckMinPrio()
		} else {
			s.NzbsToNzbsPrio(nzbs)
			s.FilterNzbTitleLength()
			s.FilterNzbSize()
			s.FilterNzbHistory()
			s.SetDataField([]string{list.Name}, list.Addfound)
			s.FilterNzbRegex()
			s.NzbParse()
			//s.NzbCheckYear(cfg_quality, strconv.Itoa(s.Dbmovie.Year))
			s.NzbCheckTitle()
			s.NzbCheckEpisodeWanted()
			s.NzbCheckQualityWanted()
			s.NzbCheckMinPrio()

		}
		logger.Log.Debug("Search RSS ended - found entries: ", len(s.Nzbs))
	}
	if len(s.Nzbs) > 1 {
		sort.Slice(s.Nzbs, func(i, j int) bool {
			return s.Nzbs[i].Prio > s.Nzbs[j].Prio
		})
	}
	if len(s.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return searchResults{Nzbs: s.Nzbs}
}

func addrsshistory(lastids map[string]string, quality string, config string) {
	for keyval, idval := range lastids {
		rssmap := make(map[string]interface{})
		rssmap["indexer"] = keyval
		rssmap["last_id"] = idval
		rssmap["list"] = quality
		rssmap["config"] = config
		database.Upsert("r_sshistories", rssmap, database.Query{Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{config, quality, keyval}})
	}
}

func (s *Searcher) MovieSearch(movie database.Movie, forceDownload bool, titlesearch bool) searchResults {
	s.SearchGroupType = "movie"
	if movie.DontSearch && !forceDownload {
		logger.Log.Debug("Skipped - Search disabled")
		return searchResults{}
	}
	s.Movie = movie

	dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
	s.Dbmovie = dbmovie

	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.Movie.QualityProfile, &cfg_quality)
	s.Quality = s.Movie.QualityProfile

	var dbmoviealt []database.DbmovieTitle
	if (cfg_quality.BackupSearchForTitle || cfg_quality.BackupSearchForAlternateTitle) && titlesearch {
		dbmoviealt, _ = database.QueryDbmovieTitle(database.Query{Select: "title", Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.DbmovieID}})
	}

	s.MinimumPriority = getHighestMoviePriorityByFiles(movie, s.ConfigEntry, cfg_quality)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if movie.DontUpgrade && !forceDownload {
			logger.Log.Debug("Skipped - Upgrade disabled: ", dbmovie.Title)
			return searchResults{}
		}
	}

	var dl searchResults
	dl.Nzbs = make([]Nzbwithprio, 0, 10)
	dl.Rejected = make([]Nzbwithprio, 0, 10)

	processedindexer := 0
	for idx := range cfg_quality.Indexer {
		titleschecked := make([]string, 0, 1+len(dbmoviealt))
		erri := s.InitIndexer(cfg_quality.Indexer[idx], "api")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", cfg_quality.Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		processedindexer += 1
		s.Indexer = cfg_quality.Indexer[idx]

		var dl_add searchResults
		releasefound := false
		if s.Dbmovie.ImdbID != "" {
			dl_add = s.MoviesSearchImdb(movie, []string{s.Movie.Listname})
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
			if len(dl_add.Nzbs) >= 1 {
				logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
					break
				}
			}
		}
		if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
			dl_add = s.MoviesSearchTitle(movie, s.Dbmovie.Title, []string{s.Movie.Listname})
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
			titleschecked = append(titleschecked, s.Dbmovie.Title)
			if len(dl_add.Nzbs) >= 1 {
				logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
					break
				}
			}
		}
		if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
			for idxalt := range dbmoviealt {
				if !CheckStringArray(titleschecked, dbmoviealt[idxalt].Title) {
					dl_add = s.MoviesSearchTitle(movie, dbmoviealt[idxalt].Title, []string{s.Movie.Listname})
					dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
					titleschecked = append(titleschecked, dbmoviealt[idxalt].Title)
					if len(dl_add.Nzbs) >= 1 {
						logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
						releasefound = true
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
						if cfg_quality.CheckUntilFirstFound {
							logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
							break
						}
					}
				}
			}
			if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
				break
			}
		}
	}
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}

	if processedindexer >= 1 {
		database.UpdateColumn("movies", "lastscan", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
	}

	return dl
}

func (s *Searcher) SeriesSearch(serieEpisode database.SerieEpisode, forceDownload bool, titlesearch bool) searchResults {
	s.SearchGroupType = "series"
	if serieEpisode.DontSearch && !forceDownload {
		logger.Log.Debug("Search not wanted: ")
		return searchResults{}
	}
	s.SerieEpisode = serieEpisode

	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{serieEpisode.DbserieID}})
	s.Dbserie = dbserie
	dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{serieEpisode.DbserieEpisodeID}})
	s.Dbserieepisode = dbserieepisode

	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.SerieEpisode.QualityProfile, &cfg_quality)
	s.Quality = s.SerieEpisode.QualityProfile
	s.MinimumPriority = getHighestEpisodePriorityByFiles(serieEpisode, s.ConfigEntry, cfg_quality)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if serieEpisode.DontUpgrade && !forceDownload {
			logger.Log.Debug("Upgrade not wanted: ", dbserie.Seriename)
			return searchResults{}
		}
	}

	var dl searchResults
	dl.Nzbs = make([]Nzbwithprio, 0, 10)
	dl.Rejected = make([]Nzbwithprio, 0, 10)
	series, _ := database.GetSeries(database.Query{Select: "listname", Where: "id=?", WhereArgs: []interface{}{serieEpisode.SerieID}})

	var dbseriealt []database.DbserieAlternate
	if (cfg_quality.BackupSearchForTitle || cfg_quality.BackupSearchForAlternateTitle) && titlesearch {
		dbseriealt, _ = database.QueryDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{serieEpisode.DbserieID}})
	}

	processedindexer := 0
	for idx := range cfg_quality.Indexer {
		titleschecked := make([]string, 0, 1+len(dbseriealt))
		erri := s.InitIndexer(cfg_quality.Indexer[idx], "api")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", cfg_quality.Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		processedindexer += 1
		s.Indexer = cfg_quality.Indexer[idx]
		releasefound := false
		var dl_add searchResults

		if s.Dbserie.ThetvdbID != 0 {
			dl_add = s.SeriesSearchTvdb([]string{series.Listname})
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
			if len(dl_add.Nzbs) >= 1 {
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
					break
				}
			}
		}
		if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
			dl_add = s.SeriesSearchTitle(logger.StringToSlug(s.Dbserie.Seriename), []string{series.Listname})
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
			titleschecked = append(titleschecked, s.Dbserie.Seriename)
			if len(dl_add.Nzbs) >= 1 {
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
					break
				}
			}
		}
		if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
			for idxalt := range dbseriealt {
				if !CheckStringArray(titleschecked, dbseriealt[idxalt].Title) {
					dl_add = s.SeriesSearchTitle(logger.StringToSlug(dbseriealt[idxalt].Title), []string{series.Listname})
					dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
					titleschecked = append(titleschecked, dbseriealt[idxalt].Title)
					if len(dl_add.Nzbs) >= 1 {
						releasefound = true
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
						if cfg_quality.CheckUntilFirstFound {
							logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
							break
						}
					}
				}
			}
			if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
				break
			}
		}
	}
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}

	if processedindexer >= 1 {
		database.UpdateColumn("serie_episodes", "lastscan", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{serieEpisode.ID}})
	}

	return dl
}

func (s *Searcher) InitIndexer(indexer config.QualityIndexerConfig, rssapi string) error {
	logger.Log.Debug("Indexer search: ", indexer.Template_indexer)

	if !config.ConfigCheck("indexer_" + indexer.Template_indexer) {
		return errors.New("indexer config missing")
	}
	var cfg_indexer config.IndexersConfig
	config.ConfigGet("indexer_"+indexer.Template_indexer, &cfg_indexer)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if !(strings.ToLower(cfg_indexer.Type) == "newznab") {
		return errors.New("indexer Type Wrong")
	}
	if !cfg_indexer.Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return errors.New("indexer disabled")
	} else if !cfg_indexer.Enabled {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return errors.New("indexer disabled")
	}

	userid, _ := strconv.Atoi(cfg_indexer.Userid)

	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	lastfailed := sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true}
	counter, _ := database.CountRows("indexer_fails", database.Query{Where: "indexer=? and last_fail > ?", WhereArgs: []interface{}{cfg_indexer.Url, lastfailed}})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", cfg_indexer.Name)
		return errors.New("indexer disabled")
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		indexrssid, _ := database.GetRssHistory(database.Query{Select: "last_id", Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{s.ConfigEntry.Name, s.Quality, cfg_indexer.Url}})
		lastindexerid = indexrssid.LastID
	}

	s.Nzbindexer = apiexternal.NzbIndexer{
		URL:                     cfg_indexer.Url,
		Apikey:                  cfg_indexer.Apikey,
		UserID:                  userid,
		SkipSslCheck:            true,
		Addquotesfortitlequery:  cfg_indexer.Addquotesfortitlequery,
		Additional_query_params: indexer.Additional_query_params,
		LastRssId:               lastindexerid,
		RssDownloadAll:          cfg_indexer.RssDownloadAll,
		Customapi:               cfg_indexer.Customapi,
		Customurl:               cfg_indexer.Customurl,
		Customrssurl:            cfg_indexer.Customrssurl,
		Customrsscategory:       cfg_indexer.Customrsscategory,
		Limitercalls:            cfg_indexer.Limitercalls,
		Limiterseconds:          cfg_indexer.Limiterseconds,
		MaxAge:                  cfg_indexer.MaxAge,
		OutputAsJson:            cfg_indexer.OutputAsJson}
	if strings.Contains(indexer.Categories_indexer, ",") {
		catarray := strings.Split(indexer.Categories_indexer, ",")
		cats := make([]int, 0, len(catarray))
		for idx := range catarray {
			intcat, _ := strconv.Atoi(catarray[idx])
			cats = append(cats, intcat)
		}
		s.Indexercategories = cats
	} else {
		intcat, _ := strconv.Atoi(indexer.Categories_indexer)
		s.Indexercategories = []int{intcat}
	}
	return nil
}

func (s Searcher) MoviesSearchImdb(movie database.Movie, lists []string) searchResults {
	if strings.HasPrefix(s.Dbmovie.ImdbID, "tt") {
		s.Dbmovie.ImdbID = strings.Trim(s.Dbmovie.ImdbID, "t")
	}
	nzbs, failed, nzberr := apiexternal.QueryNewznabMovieImdb([]apiexternal.NzbIndexer{s.Nzbindexer}, s.Dbmovie.ImdbID, s.Indexercategories)
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Dbmovie.ImdbID, " with ", s.Nzbindexer.URL)
		failedindexer(failed)
	}

	if len(nzbs) >= 1 {
		s.NzbsToNzbsPrio(nzbs)
		s.FilterNzbTitleLength()
		s.FilterNzbSize()
		s.FilterNzbHistory()
		s.CheckMatchingID()
		s.SetDataField(lists, false)
		s.NzbCheckYear(strconv.Itoa(s.Dbmovie.Year))
		s.FilterNzbRegex()
		s.NzbParse()
		s.NzbCheckTitle()
		s.NzbCheckQualityWanted()
		s.NzbCheckMinPrio()
		//accepted, denied := filter_movies_nzbs(s.ConfigEntry, cfg_quality, s.Indexer, s.Nzbindexer, nzbs, s.Movie.ID, 0, s.MinimumPriority, s.Dbmovie, database.Dbserie{}, s.Dbmovie.Title, s.AlternateNames, strconv.Itoa(s.Dbmovie.Year))
		//retnzb = append(retnzb, accepted...)
		//retdenied = append(retdenied, denied...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(s.Nzbs))
	}
	return searchResults{Nzbs: s.Nzbs, Rejected: s.NzbsDenied}
}

func (s Searcher) MoviesSearchTitle(movie database.Movie, title string, lists []string) searchResults {
	if len(title) == 0 {
		return searchResults{Nzbs: []Nzbwithprio{}}
	}
	if !config.ConfigCheck("quality_" + s.Quality) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.Quality, &cfg_quality)

	searchfor := title + " (" + strconv.Itoa(s.Dbmovie.Year) + ")"
	if cfg_quality.ExcludeYearFromTitleSearch {
		searchfor = title
	}
	logger.Log.Info("Search Movie by name: ", title)
	nzbs, failed, nzberr := apiexternal.QueryNewznabQuery([]apiexternal.NzbIndexer{s.Nzbindexer}, searchfor, s.Indexercategories, "movie")
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", title, " with ", s.Nzbindexer.URL)
		failedindexer(failed)
	}
	if len(nzbs) >= 1 {
		s.NzbsToNzbsPrio(nzbs)
		s.FilterNzbTitleLength()
		s.FilterNzbSize()
		s.FilterNzbHistory()
		s.CheckMatchingID()
		s.SetDataField(lists, false)
		s.NzbCheckYear(strconv.Itoa(s.Dbmovie.Year))
		s.FilterNzbRegex()
		s.NzbParse()
		s.NzbCheckTitle()
		s.NzbCheckQualityWanted()
		s.NzbCheckMinPrio()

		// accepted, denied := filter_movies_nzbs(s.ConfigEntry, cfg_quality, s.Indexer, s.Nzbindexer, nzbs, movie.ID, 0, s.MinimumPriority, s.Dbmovie, database.Dbserie{}, s.Dbmovie.Title, s.AlternateNames, strconv.Itoa(s.Dbmovie.Year))
		// retdenied = append(retdenied, denied...)
		// retnzb = append(retnzb, accepted...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(s.Nzbs))
	}
	return searchResults{Nzbs: s.Nzbs, Rejected: s.NzbsDenied}
}

func failedindexer(failed []string) {
	for _, failedidx := range failed {
		failmap := make(map[string]interface{})
		failmap["indexer"] = failedidx
		failmap["last_fail"] = time.Now()
		database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
	}
}

func (s Searcher) SeriesSearchTvdb(lists []string) searchResults {
	logger.Log.Info("Search Series by tvdbid: ", s.Dbserie.ThetvdbID, " S", s.Dbserieepisode.Season, "E", s.Dbserieepisode.Episode)
	seasonint, _ := strconv.Atoi(s.Dbserieepisode.Season)
	episodeint, _ := strconv.Atoi(s.Dbserieepisode.Episode)
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb([]apiexternal.NzbIndexer{s.Nzbindexer}, s.Dbserie.ThetvdbID, s.Indexercategories, seasonint, episodeint)
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Dbserie.ThetvdbID, " with ", s.Nzbindexer.URL)
		failedindexer(failed)
	}

	if len(nzbs) >= 1 {
		s.NzbsToNzbsPrio(nzbs)
		s.FilterNzbTitleLength()
		s.FilterNzbSize()
		s.FilterNzbHistory()
		s.CheckMatchingID()
		s.SetDataField(lists, false)
		s.FilterNzbRegex()
		s.NzbParse()
		//s.NzbCheckYear(cfg_quality, strconv.Itoa(s.Dbmovie.Year))
		s.NzbCheckTitle()
		s.NzbCheckEpisodeWanted()
		s.NzbCheckQualityWanted()
		s.NzbCheckMinPrio()
		// accepted, denied := filter_series_nzbs(s.ConfigEntry, cfg_quality, s.Indexer, s.Nzbindexer, nzbs, 0, s.SerieEpisode.ID, s.MinimumPriority, database.Dbmovie{}, s.Dbserie, s.Dbserie.Seriename, s.AlternateNames)
		// retnzb = append(retnzb, accepted...)
		// retdenied = append(retdenied, denied...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(s.Nzbs))
	}
	return searchResults{Nzbs: s.Nzbs, Rejected: s.NzbsDenied}
}

func (s Searcher) SeriesSearchTitle(title string, lists []string) searchResults {
	if !config.ConfigCheck("quality_" + s.Quality) {
		return searchResults{}
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.Quality, &cfg_quality)

	if title != "" && s.Dbserieepisode.Identifier != "" && cfg_quality.BackupSearchForTitle {
		logger.Log.Info("Search Series by title: ", title, " ", s.Dbserieepisode.Identifier)
		searchfor := title + " " + s.Dbserieepisode.Identifier
		nzbs, failed, nzberr := apiexternal.QueryNewznabQuery([]apiexternal.NzbIndexer{s.Nzbindexer}, searchfor, s.Indexercategories, "search")
		if nzberr != nil {
			logger.Log.Error("Newznab Search failed: ", title, " with ", s.Nzbindexer.URL)
			failedindexer(failed)
		}
		if len(nzbs) >= 1 {
			s.NzbsToNzbsPrio(nzbs)
			s.FilterNzbTitleLength()
			s.FilterNzbSize()
			s.FilterNzbHistory()
			s.CheckMatchingID()
			s.SetDataField(lists, false)
			s.FilterNzbRegex()
			s.NzbParse()
			//s.NzbCheckYear(cfg_quality, strconv.Itoa(s.Dbmovie.Year))
			s.NzbCheckTitle()
			s.NzbCheckEpisodeWanted()
			s.NzbCheckQualityWanted()
			s.NzbCheckMinPrio()

			// accepted, denied := filter_series_nzbs(s.ConfigEntry, cfg_quality, s.Indexer, s.Nzbindexer, nzbs, 0, s.SerieEpisode.ID, s.MinimumPriority, database.Dbmovie{}, s.Dbserie, s.Dbserie.Seriename, s.AlternateNames)
			// retnzb = append(retnzb, accepted...)
			// retdenied = append(retdenied, denied...)
			logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(s.Nzbs))
		}
	}
	return searchResults{Nzbs: s.Nzbs, Rejected: s.NzbsDenied}
}
