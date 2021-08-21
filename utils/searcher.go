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
	"github.com/remeh/sizedwaitgroup"
)

func SearchMovieMissing(cfg config.Cfg, configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(configEntry.Data) >= 1 {
		_, okmap := cfg.Path[configEntry.Data[0].Template_path]
		if okmap {
			scaninterval = cfg.Path[configEntry.Data[0].Template_path].MissingScanInterval
		} else {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
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
		qu.Where = "missing = 1 AND listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (lastscan is null or Lastscan < ?)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "missing = 1 AND listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = argslist
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.OrderBy = "Lastscan asc"
	missingmovies, _ := database.QueryMovies(qu)

	// searchnow := NewSearcher(configEntry, list)
	swg := sizedwaitgroup.New(cfg.General.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()
		searchnow := NewSearcher(cfg, configEntry, missingmovies[idx].QualityProfile)
		searchresults := searchnow.MovieSearch(missingmovies[idx], false, titlesearch)
		if len(searchresults.Nzbs) >= 1 {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
			downloadnow := NewDownloader(cfg, configEntry, "missing")
			downloadnow.SetMovie(missingmovies[idx])
			downloadnow.DownloadNzb(searchresults.Nzbs[0])
		}
		swg.Done()
		// swg.Add()
		// jobFindDownloadMissingNzbMovieImdb(movie, configEntry, list, &swg)
	}
	swg.Wait()
}

func SearchMovieSingle(cfg config.Cfg, movie database.Movie, configEntry config.MediaTypeConfig, titlesearch bool) {
	searchtype := "missing"
	if !movie.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(cfg, configEntry, movie.QualityProfile)
	searchresults := searchnow.MovieSearch(movie, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := NewDownloader(cfg, configEntry, searchtype)
		downloadnow.SetMovie(movie)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}

func SearchMovieUpgrade(cfg config.Cfg, configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(configEntry.Data) >= 1 {
		_, okmap := cfg.Path[configEntry.Data[0].Template_path]
		if okmap {
			scaninterval = cfg.Path[configEntry.Data[0].Template_path].UpgradeScanInterval
		} else {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
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

	swg := sizedwaitgroup.New(cfg.General.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()
		searchnow := NewSearcher(cfg, configEntry, missingmovies[idx].QualityProfile)
		searchresults := searchnow.MovieSearch(missingmovies[idx], false, titlesearch)
		if len(searchresults.Nzbs) >= 1 {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
			downloadnow := NewDownloader(cfg, configEntry, "upgrade")
			downloadnow.SetMovie(missingmovies[idx])
			downloadnow.DownloadNzb(searchresults.Nzbs[0])
		}
		swg.Done()

		// swg.Add()
		// jobFindDownloadUpgradeNzbMovieImdb(movie, configEntry, list, &swg)
	}
	swg.Wait()
}

func SearchSerieSingle(cfg config.Cfg, serie database.Serie, configEntry config.MediaTypeConfig, titlesearch bool) {
	swg := sizedwaitgroup.New(cfg.General.WorkerSearch)
	episodes, _ := database.QuerySerieEpisodes(database.Query{Where: "serie_id = ?", WhereArgs: []interface{}{serie.ID}})
	for idx := range episodes {
		swg.Add()
		searchtype := "missing"
		if !episodes[idx].Missing {
			searchtype = "upgrade"
		}
		searchnow := NewSearcher(cfg, configEntry, episodes[idx].QualityProfile)
		searchresults := searchnow.SeriesSearch(episodes[idx], false, titlesearch)
		if len(searchresults.Nzbs) >= 1 {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
			downloadnow := NewDownloader(cfg, configEntry, searchtype)
			downloadnow.SetSeriesEpisode(episodes[idx])
			downloadnow.DownloadNzb(searchresults.Nzbs[0])
		}
		swg.Done()
	}
	swg.Wait()
}

func SearchSerieEpisodeSingle(cfg config.Cfg, row database.SerieEpisode, configEntry config.MediaTypeConfig, titlesearch bool) {
	searchtype := "missing"
	if !row.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(cfg, configEntry, row.QualityProfile)
	searchresults := searchnow.SeriesSearch(row, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := NewDownloader(cfg, configEntry, searchtype)
		downloadnow.SetSeriesEpisode(row)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}
func SearchSerieMissing(cfg config.Cfg, configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(configEntry.Data) >= 1 {
		_, okmap := cfg.Path[configEntry.Data[0].Template_path]
		if okmap {
			scaninterval = cfg.Path[configEntry.Data[0].Template_path].MissingScanInterval
		} else {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
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

	qu := database.Query{}
	qu.Select = "Serie_episodes.*"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "serie_episodes.missing = 1 AND dbserie_episodes.Season != 0 and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")"
		qu.WhereArgs = argslist
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.OrderBy = "Lastscan asc"
	missingepisode, _ := database.QuerySerieEpisodes(qu)

	swg := sizedwaitgroup.New(cfg.General.WorkerSearch)
	for idx := range missingepisode {
		dbepi, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		epicount, _ := database.CountRows("dbserie_episodes", database.Query{Where: "identifier=? and dbserie_id=?", WhereArgs: []interface{}{dbepi.Identifier, dbepi.DbserieID}})
		if epicount >= 2 {
			continue
		}
		swg.Add()
		searchnow := NewSearcher(cfg, configEntry, missingepisode[idx].QualityProfile)
		searchresults := searchnow.SeriesSearch(missingepisode[idx], false, titlesearch)
		if len(searchresults.Nzbs) >= 1 {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
			downloadnow := NewDownloader(cfg, configEntry, "missing")
			downloadnow.SetSeriesEpisode(missingepisode[idx])
			downloadnow.DownloadNzb(searchresults.Nzbs[0])
		}
		swg.Done()
		// swg.Add()
		// jobFindDownloadMissingNzbSeriesTvdb(serieepisode, configEntry, list, &swg)
	}
	swg.Wait()
}
func SearchSerieUpgrade(cfg config.Cfg, configEntry config.MediaTypeConfig, jobcount int, titlesearch bool) {
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(configEntry.Data) >= 1 {
		_, okmap := cfg.Path[configEntry.Data[0].Template_path]
		if okmap {
			scaninterval = cfg.Path[configEntry.Data[0].Template_path].UpgradeScanInterval
		} else {
			logger.Log.Error("Name in PathsMap not found")
			return
		}
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

	qu := database.Query{}
	qu.Select = "Serie_episodes.*"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"
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
	qu.OrderBy = "Lastscan asc"
	missingepisode, _ := database.QuerySerieEpisodes(qu)

	swg := sizedwaitgroup.New(cfg.General.WorkerSearch)
	for idx := range missingepisode {
		dbepi, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		epicount, _ := database.CountRows("dbserie_episodes", database.Query{Where: "identifier=? and dbserie_id=?", WhereArgs: []interface{}{dbepi.Identifier, dbepi.DbserieID}})
		if epicount >= 2 {
			continue
		}
		swg.Add()
		searchnow := NewSearcher(cfg, configEntry, missingepisode[idx].QualityProfile)
		searchresults := searchnow.SeriesSearch(missingepisode[idx], false, titlesearch)
		if len(searchresults.Nzbs) >= 1 {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
			downloadnow := NewDownloader(cfg, configEntry, "upgrade")
			downloadnow.SetSeriesEpisode(missingepisode[idx])
			downloadnow.DownloadNzb(searchresults.Nzbs[0])
		}
		swg.Done()
		// swg.Add()
		// jobFindDownloadUpgradeNzbSeriesTvdb(serieepisode, configEntry, list, &swg)
	}
	swg.Wait()
}

func SearchSerieRSS(cfg config.Cfg, configEntry config.MediaTypeConfig, quality string) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(cfg, configEntry, quality)
	searchresults := searchnow.SearchRSS("series")
	downloaded := make(map[int]bool, 10)
	for idx := range searchresults.Nzbs {
		if _, nok := downloaded[int(searchresults.Nzbs[idx].Nzbepisode.ID)]; nok {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded[int(searchresults.Nzbs[idx].Nzbepisode.ID)] = true
		downloadnow := NewDownloader(cfg, configEntry, "rss")
		downloadnow.SetSeriesEpisode(searchresults.Nzbs[idx].Nzbepisode)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

func SearchMovieRSS(cfg config.Cfg, configEntry config.MediaTypeConfig, quality string) {
	logger.Log.Debug("Get Rss Movie List")

	searchnow := NewSearcher(cfg, configEntry, quality)
	searchresults := searchnow.SearchRSS("movie")
	downloaded := make(map[int]bool, 10)
	for idx := range searchresults.Nzbs {
		if _, nok := downloaded[int(searchresults.Nzbs[idx].Nzbmovie.ID)]; nok {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded[int(searchresults.Nzbs[idx].Nzbmovie.ID)] = true
		downloadnow := NewDownloader(cfg, configEntry, "rss")
		downloadnow.SetMovie(searchresults.Nzbs[idx].Nzbmovie)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

type searchResults struct {
	Nzbs []nzbwithprio
}

type Searcher struct {
	Cfg               config.Cfg
	ConfigEntry       config.MediaTypeConfig
	Quality           string
	SearchGroupType   string //series, movies
	SearchActionType  string //missing,upgrade,rss
	Indexer           config.QualityIndexerConfig
	Indexercategories []int
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
}

func NewSearcher(cfg config.Cfg, configEntry config.MediaTypeConfig, quality string) Searcher {
	return Searcher{
		Cfg:         cfg,
		ConfigEntry: configEntry,
		Quality:     quality,
	}
}

//searchGroupType == movie || series
func (s *Searcher) SearchRSS(searchGroupType string) searchResults {
	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	maxitems := s.Cfg.Indexer[s.Cfg.Quality[s.Quality].Indexer[0].Template_indexer].MaxRssEntries
	if s.Cfg.Indexer[s.Cfg.Quality[s.Quality].Indexer[0].Template_indexer].RssEntriesloop >= 1 {
		maxitems = maxitems * s.Cfg.Indexer[s.Cfg.Quality[s.Quality].Indexer[0].Template_indexer].RssEntriesloop
	}
	retnzb := make([]nzbwithprio, 0, maxitems)
	lists := make([]string, 0, len(s.ConfigEntry.Lists))
	for idxlisttest := range s.ConfigEntry.Lists {
		lists = append(lists, s.ConfigEntry.Lists[idxlisttest].Name)
	}
	if len(lists) == 0 {
		logger.Log.Error("lists empty for config ", searchGroupType, " ", s.ConfigEntry.Name)
		return searchResults{}
	}
	for idx := range s.Cfg.Quality[s.Quality].Indexer {
		erri := s.InitIndexer(s.Cfg.Quality[s.Quality].Indexer[idx], "rss")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		s.Indexer = s.Cfg.Quality[s.Quality].Indexer[idx]

		nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast([]apiexternal.NzbIndexer{s.Nzbindexer}, s.Cfg.Indexer[s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer].MaxRssEntries, s.Indexercategories, s.Cfg.Indexer[s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer].RssEntriesloop)
		if nzberr != nil {
			logger.Log.Error("Newznab RSS Search failed ", s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer)
			for _, failedidx := range failed {
				failmap := make(map[string]interface{})
				failmap["indexer"] = failedidx
				failmap["last_fail"] = time.Now()
				database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
			}
		} else {
			for keyval, idval := range lastids {
				rssmap := make(map[string]interface{})
				rssmap["indexer"] = keyval
				rssmap["last_id"] = idval
				rssmap["list"] = s.Quality
				rssmap["config"] = s.ConfigEntry.Name
				database.Upsert("r_sshistories", rssmap, database.Query{Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{s.ConfigEntry.Name, s.Quality, keyval}})
			}
			logger.Log.Debug("Search RSS ended - found entries: ", len(nzbs))
			if len(nzbs) >= 1 {
				if strings.ToLower(s.SearchGroupType) == "movie" {
					retnzb = append(retnzb, filter_movies_rss_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], lists, s.Cfg.Quality[s.Quality].Indexer[idx], nzbs)...)
					logger.Log.Debug("Search RSS ended - found entries after filter: ", len(retnzb))
				} else {
					retnzb = append(retnzb, filter_series_rss_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], lists, s.Cfg.Quality[s.Quality].Indexer[idx], nzbs)...)
					logger.Log.Debug("Search RSS ended - found entries after filter: ", len(retnzb))
				}
			}
		}
	}
	if len(retnzb) > 1 {
		sort.Slice(retnzb, func(i, j int) bool {
			return retnzb[i].Prio > retnzb[j].Prio
		})
	}
	if len(retnzb) == 0 {
		logger.Log.Info("No new entries found")
	}
	return searchResults{Nzbs: retnzb}
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

	dbmoviealt, _ := database.QueryDbmovieTitle(database.Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.DbmovieID}})

	s.MinimumPriority = getHighestMoviePriorityByFiles(s.Cfg, movie, s.ConfigEntry, s.Cfg.Quality[s.Quality])

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
	dl.Nzbs = make([]nzbwithprio, 0, 10)
	titleschecked := make(map[string]bool, 10)

	processedindexer := 0
	for idx := range s.Cfg.Quality[s.Quality].Indexer {
		erri := s.InitIndexer(s.Cfg.Quality[s.Quality].Indexer[idx], "api")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		processedindexer += 1
		s.Indexer = s.Cfg.Quality[s.Quality].Indexer[idx]

		releasefound := false
		if s.Dbmovie.ImdbID != "" {
			dl_add := s.MoviesSearchImdb(movie)
			if len(dl_add.Nzbs) >= 1 {
				logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if len(dl_add.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
				break
			}
		}
		if !releasefound && s.Cfg.Quality[s.Quality].BackupSearchForTitle && titlesearch {
			dl_add := s.MoviesSearchTitle(movie, s.Dbmovie.Slug)
			titleschecked[s.Dbmovie.Title] = true
			if len(dl_add.Nzbs) >= 1 {
				logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if len(dl_add.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbmovie.Title)
				break
			}
		}
		if !releasefound && s.Cfg.Quality[s.Quality].BackupSearchForAlternateTitle && titlesearch {
			for idx := range dbmoviealt {
				if _, ok := titleschecked[dbmoviealt[idx].Slug]; !ok {
					dl_add := s.MoviesSearchTitle(movie, dbmoviealt[idx].Title)
					titleschecked[dbmoviealt[idx].Title] = true
					if len(dl_add.Nzbs) >= 1 {
						logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
						releasefound = true
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
					}
				}
			}
			if len(dl.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
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

	dbseriealt, _ := database.QueryDbserieAlternates(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{serieEpisode.DbserieID}})

	s.MinimumPriority = getHighestEpisodePriorityByFiles(s.Cfg, serieEpisode, s.ConfigEntry, s.Cfg.Quality[s.Quality])

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
	dl.Nzbs = make([]nzbwithprio, 0, 10)

	processedindexer := 0
	for idx := range s.Cfg.Quality[s.Quality].Indexer {
		titleschecked := make(map[string]bool, 10)
		erri := s.InitIndexer(s.Cfg.Quality[s.Quality].Indexer[idx], "api")
		if erri != nil {
			logger.Log.Debug("Skipped Indexer: ", s.Cfg.Quality[s.Quality].Indexer[idx].Template_indexer, " ", erri)
			continue
		}
		processedindexer += 1
		s.Indexer = s.Cfg.Quality[s.Quality].Indexer[idx]
		releasefound := false
		if s.Dbserie.ThetvdbID != 0 {
			dl_add := s.SeriesSearchTvdb()
			if len(dl_add.Nzbs) >= 1 {
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if len(dl_add.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
				break
			}
		}
		if !releasefound && s.Cfg.Quality[s.Quality].BackupSearchForTitle && titlesearch {
			dl_add := s.SeriesSearchTitle(logger.StringToSlug(s.Dbserie.Seriename))
			titleschecked[s.Dbserie.Seriename] = true
			if len(dl_add.Nzbs) >= 1 {
				releasefound = true
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if len(dl_add.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
				break
			}
		}
		if !releasefound && s.Cfg.Quality[s.Quality].BackupSearchForAlternateTitle && titlesearch {
			for idx := range dbseriealt {
				if _, ok := titleschecked[dbseriealt[idx].Title]; !ok {
					dl_add := s.SeriesSearchTitle(logger.StringToSlug(dbseriealt[idx].Title))
					titleschecked[dbseriealt[idx].Title] = true
					if len(dl_add.Nzbs) >= 1 {
						releasefound = true
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
					}
					if len(dl_add.Nzbs) >= 1 && s.Cfg.Quality[s.Quality].CheckUntilFirstFound {
						logger.Log.Debug("Break Indexer loop - entry found: ", dbserie.Seriename, " ", dbserieepisode.Identifier)
						break
					}
				}
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
	if !(strings.ToLower(s.Cfg.Indexer[indexer.Template_indexer].Type) == "newznab") {
		return errors.New("indexer Type Wrong")
	}
	if !s.Cfg.Indexer[indexer.Template_indexer].Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", s.Cfg.Indexer[indexer.Template_indexer].Name)
		return errors.New("indexer disabled")
	} else if !s.Cfg.Indexer[indexer.Template_indexer].Enabled {
		logger.Log.Debug("Indexer disabled: ", s.Cfg.Indexer[indexer.Template_indexer].Name)
		return errors.New("indexer disabled")
	}

	userid, _ := strconv.Atoi(s.Cfg.Indexer[indexer.Template_indexer].Userid)

	lastfailed := sql.NullTime{Time: time.Now().Add(time.Minute * -1), Valid: true}
	counter, _ := database.CountRows("indexer_fails", database.Query{Where: "indexer=? and last_fail > ?", WhereArgs: []interface{}{s.Cfg.Indexer[indexer.Template_indexer].Url, lastfailed}})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last Minute: ", s.Cfg.Indexer[indexer.Template_indexer].Name)
		return errors.New("indexer disabled")
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		indexrssid, _ := database.GetRssHistory(database.Query{Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{s.ConfigEntry.Name, s.Quality, s.Cfg.Indexer[indexer.Template_indexer].Url}})
		lastindexerid = indexrssid.LastID
	}

	nzbindexer := apiexternal.NzbIndexer{
		URL:                     s.Cfg.Indexer[indexer.Template_indexer].Url,
		Apikey:                  s.Cfg.Indexer[indexer.Template_indexer].Apikey,
		UserID:                  userid,
		SkipSslCheck:            true,
		Addquotesfortitlequery:  s.Cfg.Indexer[indexer.Template_indexer].Addquotesfortitlequery,
		Additional_query_params: indexer.Additional_query_params,
		LastRssId:               lastindexerid,
		Customapi:               s.Cfg.Indexer[indexer.Template_indexer].Customapi,
		Customurl:               s.Cfg.Indexer[indexer.Template_indexer].Customurl,
		Customrssurl:            s.Cfg.Indexer[indexer.Template_indexer].Customrssurl,
		Limitercalls:            s.Cfg.Indexer[indexer.Template_indexer].Limitercalls,
		Limiterseconds:          s.Cfg.Indexer[indexer.Template_indexer].Limiterseconds}
	s.Nzbindexer = nzbindexer
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

func (s Searcher) MoviesSearchImdb(movie database.Movie) searchResults {
	retnzb := []nzbwithprio{}

	if strings.HasPrefix(s.Dbmovie.ImdbID, "tt") {
		s.Dbmovie.ImdbID = strings.Trim(s.Dbmovie.ImdbID, "t")
	}
	nzbs, failed, nzberr := apiexternal.QueryNewznabMovieImdb([]apiexternal.NzbIndexer{s.Nzbindexer}, s.Dbmovie.ImdbID, s.Indexercategories)
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Dbmovie.ImdbID, " with ", s.Nzbindexer.URL)
		for _, failedidx := range failed {
			failmap := make(map[string]interface{})
			failmap["indexer"] = failedidx
			failmap["last_fail"] = time.Now()
			database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
		}
	}
	if len(nzbs) >= 1 {
		retnzb = append(retnzb, filter_movies_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], s.Indexer, nzbs, s.Movie.ID, 0, s.MinimumPriority, s.Dbmovie, database.Dbserie{}, s.Dbmovie.Title, []string{}, strconv.Itoa(s.Dbmovie.Year))...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(retnzb))
	}
	return searchResults{retnzb}
}

func (s Searcher) MoviesSearchTitle(movie database.Movie, title string) searchResults {
	retnzb := []nzbwithprio{}
	if len(title) == 0 {
		return searchResults{retnzb}
	}
	searchfor := title + " (" + strconv.Itoa(s.Dbmovie.Year) + ")"
	if s.Cfg.Quality[s.Quality].ExcludeYearFromTitleSearch {
		searchfor = title
	}
	logger.Log.Info("Search Movie by name: ", title)
	nzbs, failed, nzberr := apiexternal.QueryNewznabQuery([]apiexternal.NzbIndexer{s.Nzbindexer}, searchfor, s.Indexercategories, "movie")
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", title, " with ", s.Nzbindexer.URL)
		for _, failedidx := range failed {
			failmap := make(map[string]interface{})
			failmap["indexer"] = failedidx
			failmap["last_fail"] = time.Now()
			database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
		}
	}
	if len(nzbs) >= 1 {
		retnzb = append(retnzb, filter_movies_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], s.Indexer, nzbs, movie.ID, 0, s.MinimumPriority, s.Dbmovie, database.Dbserie{}, s.Dbmovie.Title, []string{}, strconv.Itoa(s.Dbmovie.Year))...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(retnzb))
	}
	return searchResults{retnzb}
}

func (s Searcher) SeriesSearchTvdb() searchResults {
	retnzb := []nzbwithprio{}
	logger.Log.Info("Search Series by tvdbid: ", s.Dbserie.ThetvdbID, " S", s.Dbserieepisode.Season, "E", s.Dbserieepisode.Episode)
	seasonint, _ := strconv.Atoi(s.Dbserieepisode.Season)
	episodeint, _ := strconv.Atoi(s.Dbserieepisode.Episode)
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb([]apiexternal.NzbIndexer{s.Nzbindexer}, s.Dbserie.ThetvdbID, s.Indexercategories, seasonint, episodeint)
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Dbserie.ThetvdbID, " with ", s.Nzbindexer.URL)
		for _, failedidx := range failed {
			failmap := make(map[string]interface{})
			failmap["indexer"] = failedidx
			failmap["last_fail"] = time.Now()
			database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
		}
	}
	if len(nzbs) >= 1 {
		retnzb = append(retnzb, filter_series_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], s.Indexer, nzbs, 0, s.SerieEpisode.ID, s.MinimumPriority, database.Dbmovie{}, s.Dbserie)...)
		logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(retnzb))
	}
	return searchResults{retnzb}
}

func (s Searcher) SeriesSearchTitle(title string) searchResults {
	retnzb := []nzbwithprio{}
	if title != "" && s.Dbserieepisode.Identifier != "" && s.Cfg.Quality[s.Quality].BackupSearchForTitle {
		logger.Log.Info("Search Series by title: ", title, " ", s.Dbserieepisode.Identifier)
		searchfor := title + " " + s.Dbserieepisode.Identifier
		nzbs, failed, nzberr := apiexternal.QueryNewznabQuery([]apiexternal.NzbIndexer{s.Nzbindexer}, searchfor, s.Indexercategories, "search")
		if nzberr != nil {
			logger.Log.Error("Newznab Search failed: ", title, " with ", s.Nzbindexer.URL)
			for _, failedidx := range failed {
				failmap := make(map[string]interface{})
				failmap["indexer"] = failedidx
				failmap["last_fail"] = time.Now()
				database.Upsert("indexer_fails", failmap, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
			}
		}
		if len(nzbs) >= 1 {
			retnzb = append(retnzb, filter_series_nzbs(s.Cfg, s.ConfigEntry, s.Cfg.Quality[s.Quality], s.Indexer, nzbs, 0, s.SerieEpisode.ID, s.MinimumPriority, database.Dbmovie{}, s.Dbserie)...)
			logger.Log.Debug("Search Series by tvdbid ended - found entries after filter: ", len(retnzb))
		}
	}
	return searchResults{retnzb}
}
