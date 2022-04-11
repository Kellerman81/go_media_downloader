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
	"github.com/Kellerman81/go_media_downloader/newznab"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/sizedwaitgroup"
)

func SearchMovieMissing(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	var scandatepre int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		cfg_path := config.ConfigGet("path_" + configEntry.Data[0].Template_path).Data.(config.PathsConfig)

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
	argslist = nil
	argsscan = nil
	// searchnow := NewSearcher(configEntry, list)
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()

		go func(missing database.Movie) {
			SearchMovieSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(missingmovies[idx])
	}
	swg.Wait()
}

func SearchMovieSingle(movie database.Movie, configTemplate string, titlesearch bool) {
	searchtype := "missing"
	if !movie.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(configTemplate, movie.QualityProfile)
	searchresults := searchnow.MovieSearch(movie, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := downloader.NewDownloader(configTemplate, searchtype)
		downloadnow.SetMovie(movie)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}

func SearchMovieUpgrade(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		cfg_path := config.ConfigGet("path_" + configEntry.Data[0].Template_path).Data.(config.PathsConfig)

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

	argslist = nil
	argsscan = nil
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingmovies {
		swg.Add()
		go func(missing database.Movie) {
			SearchMovieSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(missingmovies[idx])
	}
	swg.Wait()
}

func SearchSerieSingle(serie database.Serie, configTemplate string, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	episodes, _ := database.QuerySerieEpisodes(database.Query{Where: "serie_id = ?", WhereArgs: []interface{}{serie.ID}})
	for idx := range episodes {
		swg.Add()
		go func(missing database.SerieEpisode) {
			SearchSerieEpisodeSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(episodes[idx])
	}
	swg.Wait()
}

func SearchSerieSeasonSingle(serie database.Serie, season string, configTemplate string, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	episodes, _ := database.QuerySerieEpisodes(database.Query{Where: "serie_id = ? and dbserie_episode_id IN (Select id from dbserie_episodes where Season=?)", WhereArgs: []interface{}{serie.ID, season}})
	for idx := range episodes {
		swg.Add()
		go func(missing database.SerieEpisode) {
			SearchSerieEpisodeSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(episodes[idx])
	}
	swg.Wait()
}

func SearchSerieRSSSeasonSingle(serie database.Serie, season int, useseason bool, configTemplate string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	quals, _ := database.QueryStaticColumnsOneString("Select distinct quality_profile from serie_episodes where serie_id=?", "Select count(distinct quality_profile) from serie_episodes where serie_id=?", serie.ID)
	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{serie.DbserieID}})
	SearchSerieRSSSeason(configTemplate, quals[0].Str, dbserie.ThetvdbID, season, useseason)
}
func SearchSeriesRSSSeasons(configTemplate string) {
	var series []database.Serie
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	argslist := []interface{}{}
	for idxlisttest := range configEntry.Lists {
		argslist = append(argslist, configEntry.Lists[idxlisttest].Name)
	}
	series, _ = database.QuerySeries(database.Query{
		Where:     "(Select Count(id) from serie_episodes where (missing=1 or quality_reached=0) AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and serie_id=series.ID) >= 1 AND series.listname in (?" + strings.Repeat(",?", len(configEntry.Lists)-1) + ")",
		WhereArgs: argslist,
		InnerJoin: "series on series.id=serie_episodes.serie_id inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id"})
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range series {
		seasons, _ := database.QueryStaticColumnsOneString("Select distinct season from dbserie_episodes where dbserie_id=?", "Select count(distinct season) from dbserie_episodes where dbserie_id=?", series[idx].DbserieID)
		for idxs := range seasons {
			swg.Add()
			go func(season string) {
				seasonint, _ := strconv.Atoi(season)
				SearchSerieRSSSeasonSingle(series[idx], seasonint, true, configTemplate)
				swg.Done()
			}(seasons[idxs].Str)
		}
	}
	swg.Wait()
}
func SearchSerieEpisodeSingle(row database.SerieEpisode, configTemplate string, titlesearch bool) {
	searchtype := "missing"
	if !row.Missing {
		searchtype = "upgrade"
	}
	searchnow := NewSearcher(configTemplate, row.QualityProfile)
	searchresults := searchnow.SeriesSearch(row, false, titlesearch)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := downloader.NewDownloader(configTemplate, searchtype)
		downloadnow.SetSeriesEpisode(row)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}
func SearchSerieMissing(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	var scandatepre int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		cfg_path := config.ConfigGet("path_" + configEntry.Data[0].Template_path).Data.(config.PathsConfig)

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

	qu := database.Query{
		Select:    "Serie_episodes.*",
		OrderBy:   "Lastscan asc",
		InnerJoin: "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"}
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = argsscan
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			qu.WhereArgs = append(argsscan, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	} else {
		qu.Where = "serie_episodes.missing = 1 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
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

	argslist = nil
	argsscan = nil
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingepisode {
		// dbepi, dbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, dbserie_id", Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		// if dbepierr != nil {
		// 	continue
		// }
		// epicount, _ := database.CountRowsStatic("Select count(id) from dbserie_episodes where identifier=? COLLATE NOCASE and dbserie_id=?", dbepi.Identifier, dbepi.DbserieID)
		// if epicount >= 2 {
		// 	continue
		// }
		swg.Add()
		go func(missing database.SerieEpisode) {
			SearchSerieEpisodeSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(missingepisode[idx])
	}
	swg.Wait()
}
func SearchSerieUpgrade(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	if len(configEntry.Data) >= 1 {
		if !config.ConfigCheck("path_" + configEntry.Data[0].Template_path) {
			return
		}
		cfg_path := config.ConfigGet("path_" + configEntry.Data[0].Template_path).Data.(config.PathsConfig)

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
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(lists)-1) + ") and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = args
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingepisode, _ := database.QuerySerieEpisodes(qu)

	args = nil
	argsscan = nil
	swg := sizedwaitgroup.New(cfg_general.WorkerSearch)
	for idx := range missingepisode {
		// dbepi, dbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, dbserie_id", Where: "id=?", WhereArgs: []interface{}{missingepisode[idx].DbserieEpisodeID}})
		// if dbepierr != nil {
		// 	continue
		// }
		// epicount, _ := database.CountRowsStatic("Select count(id) from dbserie_episodes where identifier=? COLLATE NOCASE and dbserie_id=?", dbepi.Identifier, dbepi.DbserieID)
		// if epicount >= 2 {
		// 	continue
		// }
		swg.Add()
		go func(missing database.SerieEpisode) {
			SearchSerieEpisodeSingle(missing, configTemplate, titlesearch)
			swg.Done()
		}(missingepisode[idx])
	}
	swg.Wait()
}

func SearchSerieRSS(configTemplate string, quality string) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(configTemplate, quality)
	searchresults := searchnow.SearchRSS("series", false)
	downloaded := []uint{}
	for idx := range searchresults.Nzbs {
		breakfor := false
		for idxs := range downloaded {
			if downloaded[idxs] == searchresults.Nzbs[idx].NzbepisodeID {
				breakfor = true
				break
			}
		}
		if breakfor {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded = append(downloaded, searchresults.Nzbs[idx].NzbepisodeID)

		nzbepisode, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{searchresults.Nzbs[idx].NzbepisodeID}})

		downloadnow := downloader.NewDownloader(configTemplate, "rss")
		downloadnow.SetSeriesEpisode(nzbepisode)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

func SearchSerieRSSSeason(configTemplate string, quality string, thetvdb_id int, season int, useseason bool) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(configTemplate, quality)
	searchresults := searchnow.SearchSeriesRSSSeason("series", thetvdb_id, season, useseason)
	downloaded := []uint{}
	for idx := range searchresults.Nzbs {
		breakfor := false
		for idxs := range downloaded {
			if downloaded[idxs] == searchresults.Nzbs[idx].NzbepisodeID {
				breakfor = true
				break
			}
		}
		if breakfor {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded = append(downloaded, searchresults.Nzbs[idx].NzbepisodeID)

		nzbepisode, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{searchresults.Nzbs[idx].NzbepisodeID}})

		downloadnow := downloader.NewDownloader(configTemplate, "rss")
		downloadnow.SetSeriesEpisode(nzbepisode)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

func SearchMovieRSS(configTemplate string, quality string) {
	logger.Log.Debug("Get Rss Movie List")

	searchnow := NewSearcher(configTemplate, quality)
	searchresults := searchnow.SearchRSS("movie", false)
	downloaded := []uint{}
	for idx := range searchresults.Nzbs {
		breakfor := false
		for idxs := range downloaded {
			if downloaded[idxs] == searchresults.Nzbs[idx].NzbmovieID {
				breakfor = true
				break
			}
		}
		if breakfor {
			continue
		}
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idx].NZB.Title)
		downloaded = append(downloaded, searchresults.Nzbs[idx].NzbmovieID)
		nzbmovie, _ := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{searchresults.Nzbs[idx].NzbmovieID}})

		downloadnow := downloader.NewDownloader(configTemplate, "rss")
		downloadnow.SetMovie(nzbmovie)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
	}
}

type searchResults struct {
	Nzbs     []parser.Nzbwithprio
	Rejected []parser.Nzbwithprio
}

type searcher struct {
	//ConfigEntry      config.MediaTypeConfig
	ConfigTemplate   string
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	Indexer          config.QualityIndexerConfig
	MinimumPriority  int

	//nzb
	//Nzbindexer apiexternal.NzbIndexer

	//Movies
	Movie database.Movie
	//Dbmovie database.Dbmovie

	//Series
	SerieEpisode database.SerieEpisode
	//Dbserie        database.Dbserie
	//Dbserieepisode database.DbserieEpisode

	Imdb string
	Year int

	Tvdb       int
	Season     string
	Episode    string
	Identifier string
	Name       string

	Listname string
}

//func NewSearcher(configTemplate string, quality string) searcher {
func NewSearcher(template string, quality string) searcher {
	return searcher{
		//ConfigEntry: configEntry,
		ConfigTemplate: template,
		Quality:        quality,
	}
}

//searchGroupType == movie || series
func (s *searcher) SearchRSS(searchGroupType string, fetchall bool) searchResults {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return searchResults{}
	}
	configEntry := config.ConfigGet(s.ConfigTemplate).Data.(config.MediaTypeConfig)

	cfg_quality := config.ConfigGet("quality_" + s.Quality).Data.(config.QualityConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(lists) == 0 {
		logger.Log.Error("lists empty for config ", searchGroupType, " ", configEntry.Name)
		return searchResults{}
	}
	var dl searchResults
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := sizedwaitgroup.New(cfg_general.WorkerIndexer)
	for idx := range cfg_quality.Indexer {
		swi.Add()
		go func(index config.QualityIndexerConfig) {
			dladd, ok := s.rsssearchindexer(index, fetchall, lists, &swi)
			if ok {
				if len(dl.Rejected) == 0 {
					dl.Rejected = dladd.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dladd.Rejected...)
				}
				if len(dl.Nzbs) == 0 {
					dl.Nzbs = dladd.Nzbs
				} else {
					dl.Nzbs = append(dl.Nzbs, dladd.Nzbs...)
				}
			}
		}(cfg_quality.Indexer[idx])
	}
	swi.Wait()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	if len(dl.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return searchResults{Nzbs: dl.Nzbs, Rejected: dl.Rejected}
}

func (t searcher) rsssearchindexer(index config.QualityIndexerConfig, fetchall bool, lists []string, swi *sizedwaitgroup.SizedWaitGroup) (searchResults, bool) {
	defer swi.Done()
	cfgindex, nzbindexer, cats, erri := t.initIndexer(index, "rss")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", index.Template_indexer, " ", erri)
		return searchResults{}, false
	}
	if fetchall {
		nzbindexer.LastRssId = ""
	}
	t.Indexer = index
	if cfgindex.MaxRssEntries == 0 {
		cfgindex.MaxRssEntries = 10
	}
	if cfgindex.RssEntriesloop == 0 {
		cfgindex.RssEntriesloop = 2
	}
	var dl searchResults
	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(nzbindexer, cfgindex.MaxRssEntries, cats, cfgindex.RssEntriesloop)
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {

		logger.Log.Error("Newznab RSS Search failed ", index.Template_indexer)
		failedindexer(failed)
		return searchResults{}, false
	} else {
		if !fetchall {
			if lastids != "" && len((*nzbs)) >= 1 {
				addrsshistory(nzbindexer.URL, lastids, t.Quality, t.ConfigTemplate)
			}
		}
		logger.Log.Debug("Search RSS ended - found entries: ", len((*nzbs)))
		if len((*nzbs)) >= 1 {
			tofilter := t.convertnzbs(nzbs)
			tofilter.nzbsFilterStart()
			tofilter.setDataField(lists, "", false)
			tofilter.nzbsFilterBlock2()
			if len(tofilter.NzbsDenied) >= 1 {
				dl.Rejected = tofilter.NzbsDenied
			}
			if len(tofilter.Nzbs) >= 1 {
				dl.Nzbs = tofilter.Nzbs
			}
			defer func() {
				tofilter.Nzbs = nil
				tofilter.NzbsDenied = nil
				tofilter = nil
			}()
		}
	}
	return dl, true
}

//searchGroupType == movie || series
func (s *searcher) SearchSeriesRSSSeason(searchGroupType string, thetvdb_id int, season int, useseason bool) searchResults {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return searchResults{}
	}
	configEntry := config.ConfigGet(s.ConfigTemplate).Data.(config.MediaTypeConfig)

	cfg_quality := config.ConfigGet("quality_" + s.Quality).Data.(config.QualityConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}
	if len(lists) == 0 {
		logger.Log.Error("lists empty for config ", searchGroupType, " ", configEntry.Name)
		return searchResults{}
	}
	var dl searchResults
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := sizedwaitgroup.New(cfg_general.WorkerIndexer)
	for idx := range cfg_quality.Indexer {
		swi.Add()
		go func(index config.QualityIndexerConfig) {
			dladd, ok := s.rssqueryseriesindexer(index, thetvdb_id, season, useseason, lists, &swi)
			if ok {
				if len(dl.Rejected) == 0 {
					dl.Rejected = dladd.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dladd.Rejected...)
				}
				if len(dl.Nzbs) == 0 {
					dl.Nzbs = dladd.Nzbs
				} else {
					dl.Nzbs = append(dl.Nzbs, dladd.Nzbs...)
				}
			}
		}(cfg_quality.Indexer[idx])
	}
	swi.Wait()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	if len(dl.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return searchResults{Nzbs: dl.Nzbs, Rejected: dl.Rejected}
}

func (t searcher) rssqueryseriesindexer(index config.QualityIndexerConfig, thetvdb_id int, season int, useseason bool, lists []string, swi *sizedwaitgroup.SizedWaitGroup) (searchResults, bool) {
	defer swi.Done()
	_, nzbindexer, cats, erri := t.initIndexer(index, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", index.Template_indexer, " ", erri)
		return searchResults{}, false
	}
	t.Indexer = index
	var dl searchResults
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb(nzbindexer, thetvdb_id, cats, season, 0, useseason, false)
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {

		logger.Log.Error("Newznab RSS Search failed ", index.Template_indexer)
		failedindexer(failed)
		return searchResults{}, false
	} else {
		logger.Log.Debug("Search RSS ended - found entries: ", len((*nzbs)))
		if len((*nzbs)) >= 1 {
			tofilter := t.convertnzbs(nzbs)
			tofilter.nzbsFilterStart()
			tofilter.setDataField(lists, "", false)
			tofilter.nzbsFilterBlock2()
			if len(tofilter.NzbsDenied) >= 1 {
				dl.Rejected = tofilter.NzbsDenied
			}
			if len(tofilter.Nzbs) >= 1 {
				dl.Nzbs = tofilter.Nzbs
			}
			defer func() {
				tofilter.Nzbs = nil
				tofilter.NzbsDenied = nil
				tofilter = nil
			}()
		}
	}
	return dl, true
}

type nzbFilter struct {
	T          *searcher
	ToFilter   []parser.Nzbwithprio
	Nzbs       []parser.Nzbwithprio
	NzbsDenied []parser.Nzbwithprio
}

func (s *searcher) convertnzbs(nzbs *[]newznab.NZB) *nzbFilter {
	nzbs_out := make([]parser.Nzbwithprio, 0, len((*nzbs)))
	for _, nzb := range *nzbs {
		nzbs_out = append(nzbs_out, parser.Nzbwithprio{
			Indexer: s.Indexer.Template_indexer,
			NZB:     nzb,
		})
	}

	n := &nzbFilter{T: s, NzbsDenied: make([]parser.Nzbwithprio, 0, len(nzbs_out)), ToFilter: nzbs_out}
	return n
}
func (n *nzbFilter) nzbsFilterStart() {
	nextup := make([]parser.Nzbwithprio, 0, int(len(n.ToFilter)/50))
	for idx := range n.ToFilter {
		if len(strings.Trim(n.ToFilter[idx].NZB.Title, " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", n.ToFilter[idx].NZB.Title)
			n.ToFilter[idx].Denied = true
			n.ToFilter[idx].Reason = "Title too short"
			n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
			continue
		}
		if filter_size_nzbs(n.T.ConfigTemplate, &n.T.Indexer, n.ToFilter[idx].NZB.Title, n.ToFilter[idx].NZB.Size) {
			n.ToFilter[idx].Denied = true
			n.ToFilter[idx].Reason = "Wrong size"
			n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
			continue
		}
		historytable := "serie_episode_histories"
		if strings.ToLower(n.T.SearchGroupType) == "movie" {
			historytable = "movie_histories"
		}
		countertitle, _ := database.CountRowsStatic("Select count(id) from "+historytable+" where url = ?", n.ToFilter[idx].NZB.DownloadURL)
		if countertitle >= 1 {
			logger.Log.Debug("Skipped - Already Downloaded: ", n.ToFilter[idx].NZB.Title)
			n.ToFilter[idx].Denied = true
			n.ToFilter[idx].Reason = "Already downloaded"
			n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
			continue
		}
		if n.T.Indexer.History_check_title {
			countertitle, _ = database.CountRowsStatic("Select count(id) from "+historytable+" where title = ? COLLATE NOCASE", n.ToFilter[idx].NZB.Title)
			if countertitle >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded (Title): ", n.ToFilter[idx].NZB.Title)
				n.ToFilter[idx].Denied = true
				n.ToFilter[idx].Reason = "Already downloaded"
				n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
				continue
			}
		}
		if n.T.SearchActionType != "rss" {
			if strings.ToLower(n.T.SearchGroupType) == "movie" {
				tempimdb := strings.TrimPrefix(n.ToFilter[idx].NZB.IMDBID, "tt")
				tempimdb = strings.TrimLeft(tempimdb, "0")

				wantedimdb := strings.TrimPrefix(n.T.Imdb, "tt")
				wantedimdb = strings.TrimLeft(wantedimdb, "0")
				if wantedimdb != tempimdb && len(wantedimdb) >= 1 && len(tempimdb) >= 1 {
					logger.Log.Debug("Skipped - Imdb not match: ", n.ToFilter[idx].NZB.Title, " - imdb in nzb: ", tempimdb, " imdb wanted: ", wantedimdb)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Imdbid not correct"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			} else {
				if strconv.Itoa(n.T.Tvdb) != n.ToFilter[idx].NZB.TVDBID && n.T.Tvdb >= 1 && len(n.ToFilter[idx].NZB.TVDBID) >= 1 {
					logger.Log.Debug("Skipped - Tvdb not match: ", n.ToFilter[idx].NZB.Title, " - Tvdb in nzb: ", n.ToFilter[idx].NZB.TVDBID, " Tvdb wanted: ", n.T.Tvdb)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Tvdbid not correct"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			}
		}
		nextup = append(nextup, n.ToFilter[idx])
	}
	n.ToFilter = nextup
}

//regex
func (n *nzbFilter) nzbsFilterBlock2() {
	n.Nzbs = []parser.Nzbwithprio{}
	if !config.ConfigCheck("regex_" + n.T.Indexer.Template_regex) {
		for idx := range n.ToFilter {
			n.ToFilter[idx].Denied = true
			n.ToFilter[idx].Reason = "Denied by Regex"
			n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
		}
	} else {
		cfg_regex := config.ConfigGet("regex_" + n.T.Indexer.Template_regex).Data.(config.RegexConfig)

		for idx := range n.ToFilter {
			//Regex
			regexdeny, regexrule := filter_regex_nzbs(cfg_regex, n.ToFilter[idx].NZB.Title, n.ToFilter[idx].WantedTitle, n.ToFilter[idx].WantedAlternates)
			if regexdeny {
				n.ToFilter[idx].Denied = true
				n.ToFilter[idx].Reason = "Denied by Regex: " + regexrule
				n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
				continue
			}

			//Parse
			var m parser.ParseInfo
			var err error
			if n.ToFilter[idx].ParseInfo.File == "" {
				if n.T.SearchGroupType == "series" {
					m, err = parser.NewFileParser(n.ToFilter[idx].NZB.Title, true, "series")
				} else {
					m, err = parser.NewFileParser(n.ToFilter[idx].NZB.Title, false, "movie")
				}
				if err != nil {
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Error Parsing"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					logger.Log.Error("Error parsing: ", n.ToFilter[idx].NZB.Title, " error: ", err)
					continue
				}
			} else {
				m = n.ToFilter[idx].ParseInfo
			}
			if n.ToFilter[idx].ParseInfo.Priority == 0 {
				m.GetPriority(n.T.ConfigTemplate, n.ToFilter[idx].QualityTemplate)
			}

			m.StripTitlePrefixPostfix(n.ToFilter[idx].QualityTemplate)
			n.ToFilter[idx].ParseInfo = m
			n.ToFilter[idx].Prio = m.Priority
			//Parse end

			qualityconfig := config.ConfigGet("quality_" + n.ToFilter[idx].QualityTemplate).Data.(config.QualityConfig)
			//Year
			if strings.ToLower(n.T.SearchGroupType) == "movie" && n.T.SearchActionType != "rss" {
				yearstr := strconv.Itoa(n.T.Year)
				if yearstr == "0" {
					logger.Log.Debug("Skipped - no year: ", n.ToFilter[idx].NZB.Title)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "No Year"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
				if qualityconfig.CheckYear && !qualityconfig.CheckYear1 && !strings.Contains(n.ToFilter[idx].NZB.Title, yearstr) && len(yearstr) >= 1 && yearstr != "0" {
					logger.Log.Debug("Skipped - unwanted year: ", n.ToFilter[idx].NZB.Title, " wanted ", yearstr)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Wrong Year"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				} else {
					if qualityconfig.CheckYear1 && len(yearstr) >= 1 && yearstr != "0" {
						yearint, _ := strconv.Atoi(yearstr)
						if !strings.Contains(n.ToFilter[idx].NZB.Title, strconv.Itoa(yearint+1)) && !strings.Contains(n.ToFilter[idx].NZB.Title, strconv.Itoa(yearint-1)) && !strings.Contains(n.ToFilter[idx].NZB.Title, strconv.Itoa(yearint)) {
							logger.Log.Debug("Skipped - unwanted year: ", n.ToFilter[idx].NZB.Title, " wanted (+-1) ", yearint)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Wrong Year"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					}
				}
			}

			//Checktitle
			if strings.ToLower(n.T.SearchGroupType) == "movie" {
				if qualityconfig.CheckTitle {
					titlefound := false
					if qualityconfig.CheckTitle && parser.Checknzbtitle(n.ToFilter[idx].WantedTitle, n.ToFilter[idx].ParseInfo.Title) && len(n.ToFilter[idx].WantedTitle) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound := false
						for idxtitle := range n.ToFilter[idx].WantedAlternates {
							if parser.Checknzbtitle(n.ToFilter[idx].WantedAlternates[idxtitle], n.ToFilter[idx].ParseInfo.Title) {
								alttitlefound = true
								break
							}
						}
						if len(n.ToFilter[idx].WantedAlternates) >= 1 && !alttitlefound {
							logger.Log.Debug("Skipped - unwanted title and alternate: ", n.ToFilter[idx].NZB.Title, " wanted ", n.ToFilter[idx].WantedTitle, " ", n.ToFilter[idx].WantedAlternates)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Wrong Title"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					}
					if len(n.ToFilter[idx].WantedAlternates) == 0 && !titlefound {
						logger.Log.Debug("Skipped - unwanted title: ", n.ToFilter[idx].NZB.Title, " wanted ", n.ToFilter[idx].WantedTitle)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Wrong Title"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
				}
			} else {
				if qualityconfig.CheckTitle {
					toskip := true
					if n.ToFilter[idx].WantedTitle != "" {
						if qualityconfig.CheckTitle && parser.Checknzbtitle(n.ToFilter[idx].WantedTitle, n.ToFilter[idx].ParseInfo.Title) && len(n.ToFilter[idx].WantedTitle) >= 1 {
							toskip = false
						}
						if toskip {
							for idxtitle := range n.ToFilter[idx].WantedAlternates {
								if parser.Checknzbtitle(n.ToFilter[idx].WantedAlternates[idxtitle], n.ToFilter[idx].ParseInfo.Title) {
									toskip = false
									break
								}
							}
						}
						if toskip {
							logger.Log.Debug("Skipped - seriename provided but not found ", n.ToFilter[idx].WantedTitle)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Serie name not found"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					} else {
						logger.Log.Debug("Skipped - seriename not provided or searchfortitle disabled")
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Serie name not provided"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
				}
			}

			//Checkepisode
			if strings.ToLower(n.T.SearchGroupType) != "movie" {
				foundepi, foundepierr := database.GetSerieEpisodes(database.Query{Select: "dbserie_episode_id", Where: "id = ?", WhereArgs: []interface{}{n.ToFilter[idx].NzbepisodeID}})
				if foundepierr == nil {
					founddbepi, founddbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier, season, episode", Where: "id = ?", WhereArgs: []interface{}{foundepi.DbserieEpisodeID}})
					if founddbepierr == nil {
						// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
						matchfound := false

						alt_identifier := strings.TrimLeft(founddbepi.Identifier, "S0")
						alt_identifier = strings.Replace(alt_identifier, "E", "x", -1)
						tolower := strings.ToLower(n.ToFilter[idx].NZB.Title)
						if strings.Contains(tolower, strings.ToLower(founddbepi.Identifier)) ||
							strings.Contains(tolower, strings.ToLower(strings.Replace(founddbepi.Identifier, "-", ".", -1))) ||
							strings.Contains(tolower, strings.ToLower(strings.Replace(founddbepi.Identifier, "-", " ", -1))) ||
							strings.Contains(tolower, strings.ToLower(alt_identifier)) ||
							strings.Contains(tolower, strings.ToLower(strings.Replace(alt_identifier, "-", ".", -1))) ||
							strings.Contains(tolower, strings.ToLower(strings.Replace(alt_identifier, "-", " ", -1))) {

							matchfound = true
						} else {
							seasonvars := []string{"s" + founddbepi.Season + "e", "s0" + founddbepi.Season + "e", "s" + founddbepi.Season + " e", "s0" + founddbepi.Season + " e", founddbepi.Season + "x", founddbepi.Season + " x"}
							episodevars := []string{"e" + founddbepi.Episode, "e0" + founddbepi.Episode, "x" + founddbepi.Episode, "x0" + founddbepi.Episode}
							identifierlower := strings.ToLower(n.ToFilter[idx].ParseInfo.Identifier)
							for idxseason := range seasonvars {
								if strings.HasPrefix(identifierlower, seasonvars[idxseason]) {
									for idxepisode := range episodevars {
										if strings.HasSuffix(identifierlower, episodevars[idxepisode]) {
											matchfound = true
											break
										}
										if strings.Contains(identifierlower, episodevars[idxepisode]+" ") {
											matchfound = true
											break
										}
										if strings.Contains(identifierlower, episodevars[idxepisode]+"-") {
											matchfound = true
											break
										}
										if strings.Contains(identifierlower, episodevars[idxepisode]+"e") {
											matchfound = true
											break
										}
										if strings.Contains(identifierlower, episodevars[idxepisode]+"x") {
											matchfound = true
											break
										}
									}
									break
								}
							}
						}
						if !matchfound {
							logger.Log.Debug("Skipped - seriename provided dbepi found but identifier not match ", founddbepi.Identifier, " in: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Wrong episode identifier"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					} else {
						logger.Log.Debug("Skipped - seriename provided dbepi not found", n.ToFilter[idx].WantedTitle)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "DB Episode not found"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
				} else {
					logger.Log.Debug("Skipped - seriename provided epi not found", n.ToFilter[idx].WantedTitle)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Episode not found"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			}

			//check quality
			if !filter_test_quality_wanted(n.ToFilter[idx].QualityTemplate, n.ToFilter[idx].ParseInfo, n.ToFilter[idx].NZB.Title) {
				logger.Log.Debug("Skipped - unwanted quality: ", n.ToFilter[idx].NZB.Title)
				n.ToFilter[idx].Denied = true
				n.ToFilter[idx].Reason = "Wrong Quality"
				n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
				continue
			}

			//check priority
			if n.ToFilter[idx].ParseInfo.Priority != 0 {
				if n.ToFilter[idx].MinimumPriority != 0 {
					if n.ToFilter[idx].ParseInfo.Priority <= n.ToFilter[idx].MinimumPriority {
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Prio lower. have: " + strconv.Itoa(n.ToFilter[idx].MinimumPriority)
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						logger.Log.Debug("Skipped - Prio lower: ", n.ToFilter[idx].NZB.Title, " old prio ", n.ToFilter[idx].MinimumPriority, " found prio ", n.ToFilter[idx].ParseInfo.Priority)
						continue
					}
					logger.Log.Debug("ok - prio higher: ", n.ToFilter[idx].NZB.Title, " old prio ", n.ToFilter[idx].MinimumPriority, " found prio ", n.ToFilter[idx].ParseInfo.Priority)
				}
			} else {
				n.ToFilter[idx].Denied = true
				n.ToFilter[idx].Reason = "Prio not matched"
				n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
				logger.Log.Debug("Skipped - Prio not matched: ", n.ToFilter[idx].NZB.Title)
				continue
			}
			n.Nzbs = append(n.Nzbs, n.ToFilter[idx])
		}
	}
	n.ToFilter = nil
}

//Needs s.Movie or s.SerieEpisode (for non RSS)
func (n *nzbFilter) setDataField(lists []string, addinlist string, addifnotfound bool) {
	nextup := make([]parser.Nzbwithprio, 0, int(len(n.ToFilter)/50))
	for idx := range n.ToFilter {
		if strings.ToLower(n.T.SearchGroupType) == "movie" {
			loopmovie := database.Movie{}
			loopdbmovie := database.Dbmovie{}
			if n.T.SearchActionType == "rss" {
				if n.ToFilter[idx].NZB.IMDBID != "" {
					var founddbmovie database.Dbmovie
					var founddbmovieerr error
					searchimdb := n.ToFilter[idx].NZB.IMDBID
					if !strings.HasPrefix(searchimdb, "tt") {
						searchimdb = "tt" + n.ToFilter[idx].NZB.IMDBID
					}
					founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})

					if !strings.HasPrefix(n.ToFilter[idx].NZB.IMDBID, "tt") && founddbmovieerr != nil {
						searchimdb = "tt0" + n.ToFilter[idx].NZB.IMDBID
						founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})
						if founddbmovieerr != nil {
							searchimdb = "tt00" + n.ToFilter[idx].NZB.IMDBID
							founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})
							if founddbmovieerr != nil {
								searchimdb = "tt000" + n.ToFilter[idx].NZB.IMDBID
								founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})
								if founddbmovieerr != nil {
									searchimdb = "tt0000" + n.ToFilter[idx].NZB.IMDBID
									founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})
								}
							}
						}
					}
					m, nerr := parser.NewFileParser(n.ToFilter[idx].NZB.Title, false, "movie")
					if nerr != nil {
						logger.Log.Debug("Skipped - Error Parsing: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Error Parsing Movie"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					m.StripTitlePrefixPostfix(n.T.Quality)
					n.ToFilter[idx].ParseInfo = m
					list, imdb := importfeed.MovieGetListFilter(lists, founddbmovie.ID, m.Year)
					if list != "" {
						n.ToFilter[idx].NZB.IMDBID = imdb
						getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{founddbmovie.ID, list}})
						loopmovie = getmovie
					} else {
						if addifnotfound && strings.HasPrefix(n.ToFilter[idx].NZB.IMDBID, "tt") {
							if !allowMovieImport(n.ToFilter[idx].NZB.IMDBID, addinlist) {
								continue
							}

							sww := sizedwaitgroup.New(1)
							var dbmovie database.Dbmovie
							dbmovie.ImdbID = n.ToFilter[idx].NZB.IMDBID
							sww.Add()
							importfeed.JobImportMovies(dbmovie, n.T.ConfigTemplate, addinlist, &sww)
							sww.Wait()
							founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
							getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{founddbmovie.ID, addinlist}})
							loopmovie = getmovie
						} else {
							logger.Log.Debug("Skipped - Not Wanted Movie: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Not Wanted Movie"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					}
					if founddbmovieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Movie: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Not Wanted DB Movie"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					loopdbmovie = founddbmovie
				} else {
					m, errparse := parser.NewFileParser(n.ToFilter[idx].NZB.Title, false, "movie")
					if errparse != nil {
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Error Parsing"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						logger.Log.Error("Error parsing: ", n.ToFilter[idx].NZB.Title, " error: ", errparse)
						continue
					}
					m.StripTitlePrefixPostfix(n.T.Quality)
					n.ToFilter[idx].ParseInfo = m
					list, imdb, entriesfound, dbmovie := importfeed.MovieFindListByTitle(n.ToFilter[idx].ParseInfo.Title, strconv.Itoa(n.ToFilter[idx].ParseInfo.Year), lists, "rss")
					if entriesfound >= 1 {
						n.ToFilter[idx].NZB.IMDBID = imdb
						loopdbmovie = dbmovie
						getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list}})
						loopmovie = getmovie
					} else {
						//s.Listname =
						if addifnotfound {
							//Search imdb!
							_, imdbget, imdbfound := importfeed.MovieFindDbByTitle(n.ToFilter[idx].ParseInfo.Title, strconv.Itoa(n.ToFilter[idx].ParseInfo.Year), addinlist, true, "rss")
							if imdbfound == 0 {
								if !allowMovieImport(n.ToFilter[idx].NZB.IMDBID, addinlist) {
									continue
								}

								sww := sizedwaitgroup.New(1)
								var dbmovie database.Dbmovie
								dbmovie.ImdbID = imdbget
								sww.Add()
								importfeed.JobImportMovies(dbmovie, n.T.ConfigTemplate, addinlist, &sww)
								sww.Wait()
								founddbmovie, _ := database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
								loopdbmovie = founddbmovie
								getmovie, _ := database.GetMovies(database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{loopdbmovie.ID, addinlist}})
								loopmovie = getmovie
							}
						} else {
							logger.Log.Debug("Skipped - Not Wanted DB Movie: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Not Wanted DB Movie"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					}
				}
			} else {
				loopmovie = n.T.Movie
				founddbmovie, _ := database.GetDbmovie(database.Query{Select: "id, title", Where: "id = ?", WhereArgs: []interface{}{n.T.Movie.DbmovieID}})
				loopdbmovie = founddbmovie
			}
			n.ToFilter[idx].NzbmovieID = loopmovie.ID
			if !config.ConfigCheck("quality_" + loopmovie.QualityProfile) {
				logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(loopmovie.ID)) + " not found")
				continue
			}
			n.ToFilter[idx].QualityTemplate = loopmovie.QualityProfile

			n.ToFilter[idx].MinimumPriority = parser.GetHighestMoviePriorityByFiles(loopmovie, n.T.ConfigTemplate, loopmovie.QualityProfile)
			if n.ToFilter[idx].MinimumPriority == 0 {
				//s.SearchActionType = "missing"
				if loopmovie.DontSearch {
					logger.Log.Debug("Skipped - Search disabled: ", loopdbmovie.Title)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Search Disabled"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			} else {
				//s.SearchActionType = "upgrade"
				if loopmovie.DontUpgrade {
					logger.Log.Debug("Skipped - Upgrade disabled: ", loopdbmovie.Title)
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Upgrade Disabled"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			}
			foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbmovie_titles where dbmovie_id=?", "Select count(id) from dbmovie_titles where dbmovie_id=?", loopdbmovie.ID)
			//dbmoviealt, _ := database.QueryDbmovieTitle(database.Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{loopdbmovie.ID}})
			n.ToFilter[idx].WantedAlternates = make([]string, 0, len(foundalternate))
			for idxalt := range foundalternate {
				n.ToFilter[idx].WantedAlternates = append(n.ToFilter[idx].WantedAlternates, foundalternate[idxalt].Str)
			}

			n.ToFilter[idx].WantedTitle = loopdbmovie.Title
		} else {
			loopepisode := database.SerieEpisode{}
			loopdbseries := database.Dbserie{}
			if n.T.SearchActionType == "rss" {
				var foundepisode database.SerieEpisode
				if len(n.ToFilter[idx].NZB.TVDBID) >= 1 {
					founddbserie, founddbserieerr := database.GetDbserie(database.Query{Select: "id, seriename, Identifiedby", Where: "thetvdb_id = ?", WhereArgs: []interface{}{n.ToFilter[idx].NZB.TVDBID}})

					if founddbserieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Serie: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Unwanted Dbserie"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					loopdbseries = founddbserie

					foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbserie_alternates where dbserie_id=?", "Select count(id) from dbserie_alternates where dbserie_id=?", founddbserie.ID)
					n.ToFilter[idx].WantedAlternates = make([]string, 0, len(foundalternate))
					for idxalt := range foundalternate {
						n.ToFilter[idx].WantedAlternates = append(n.ToFilter[idx].WantedAlternates, foundalternate[idxalt].Str)
					}
					args := []interface{}{}
					args = append(args, founddbserie.ID)
					for idxlist := range lists {
						args = append(args, lists[idxlist])
					}
					foundserie, foundserieerr := database.GetSeries(database.Query{Select: "id", Where: "dbserie_id = ? and listname IN (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: args})
					args = nil
					if foundserieerr != nil {
						logger.Log.Debug("Skipped - Not Wanted Serie: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Unwanted Serie"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}

					var founddbepisode database.DbserieEpisode
					var founddbepisodeerr error
					if strings.EqualFold(founddbserie.Identifiedby, "date") {
						tempparse, _ := parser.NewFileParser(n.ToFilter[idx].NZB.Title, true, "series")
						if tempparse.Date == "" {
							logger.Log.Debug("Skipped - Date wanted but not found: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Date not found"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
						tempparse.StripTitlePrefixPostfix(n.T.Quality)
						n.ToFilter[idx].ParseInfo = tempparse
						tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
						tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
						founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ?", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})
						if founddbepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted DB Episode: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Unwanted DB Episode"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					} else {
						founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, strings.TrimPrefix(strings.TrimPrefix(n.ToFilter[idx].NZB.Season, "S"), "0"), strings.TrimPrefix(strings.TrimPrefix(n.ToFilter[idx].NZB.Episode, "E"), "0")}})
						if founddbepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted DB Episode: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Unwanted DB Episode"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					}
					var foundepisodeerr error
					foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
					if foundepisodeerr != nil {
						logger.Log.Debug("Skipped - Not Wanted Episode: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Unwanted Episode"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					if foundepisode.DontSearch || foundepisode.DontUpgrade || (!foundepisode.Missing && foundepisode.QualityReached) {
						logger.Log.Debug("Skipped - Notwanted or Already reached: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Unwanted or reached"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					if !config.ConfigCheck("quality_" + foundepisode.QualityProfile) {
						logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(loopepisode.ID)) + " not found")
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Quality profile not found"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
					if foundepisode.QualityProfile == "" {
						foundepisode.QualityProfile = n.T.Quality
					}
					loopepisode = foundepisode
				} else {
					var foundserie database.Serie
					tempparse, _ := parser.NewFileParser(n.ToFilter[idx].NZB.Title, true, "series")
					tempparse.StripTitlePrefixPostfix(n.T.Quality)
					n.ToFilter[idx].ParseInfo = tempparse
					titleyear := tempparse.Title
					if tempparse.Year != 0 {
						titleyear += " (" + strconv.Itoa(tempparse.Year) + ")"
					}
					seriestitle := ""
					matched := config.RegexGet("RegexSeriesTitle").FindStringSubmatch(n.ToFilter[idx].NZB.Title)
					if len(matched) >= 2 {
						seriestitle = matched[1]
					}
					for idxlist := range lists {
						series, entriesfound := n.ToFilter[idx].ParseInfo.FindSerieByParser(titleyear, seriestitle, lists[idxlist])
						if entriesfound >= 1 {
							foundserie = series
							break
						}
					}
					if foundserie.ID != 0 {
						founddbserie, _ := database.GetDbserie(database.Query{Select: "id, seriename, Identifiedby", Where: "id = ?", WhereArgs: []interface{}{foundserie.DbserieID}})

						loopdbseries = founddbserie
						var founddbepisode database.DbserieEpisode
						var founddbepisodeerr error
						if strings.EqualFold(founddbserie.Identifiedby, "date") {
							if tempparse.Date == "" {
								logger.Log.Debug("Skipped - Date wanted but not found: ", n.ToFilter[idx].NZB.Title)
								n.ToFilter[idx].Denied = true
								n.ToFilter[idx].Reason = "Unwanted Date"
								n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
								continue
							}
							tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
							tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
							founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})

							if founddbepisodeerr != nil {
								logger.Log.Debug("Skipped - Not Wanted DB Episode: ", n.ToFilter[idx].NZB.Title)
								n.ToFilter[idx].Denied = true
								n.ToFilter[idx].Reason = "Unwanted DB Episode"
								n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
								continue
							}
						} else {
							founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, strings.TrimPrefix(strings.TrimPrefix(n.ToFilter[idx].NZB.Season, "S"), "0"), strings.TrimPrefix(strings.TrimPrefix(n.ToFilter[idx].NZB.Episode, "E"), "0")}})
							if founddbepisodeerr != nil {
								logger.Log.Debug("Skipped - Not Wanted DB Episode: ", n.ToFilter[idx].NZB.Title)
								n.ToFilter[idx].Denied = true
								n.ToFilter[idx].Reason = "Unwanted DB Episode"
								n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
								continue
							}
						}
						var foundepisodeerr error
						foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
						if foundepisodeerr != nil {
							logger.Log.Debug("Skipped - Not Wanted Episode: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Unwanted Episode"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
						loopepisode = foundepisode
						if !config.ConfigCheck("quality_" + foundepisode.QualityProfile) {
							logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(loopepisode.ID)) + " not found")
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "Quality Profile unknown"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
						cfg_quality := config.ConfigGet("quality_" + foundepisode.QualityProfile).Data.(config.QualityConfig)
						if cfg_quality.BackupSearchForTitle {
							n.ToFilter[idx].NzbepisodeID = foundepisode.ID
							foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbserie_alternates where dbserie_id=?", "Select count(id) from dbserie_alternates where dbserie_id=?", founddbserie.ID)
							n.ToFilter[idx].WantedAlternates = make([]string, 0, len(foundalternate))
							for idxalt := range foundalternate {
								n.ToFilter[idx].WantedAlternates = append(n.ToFilter[idx].WantedAlternates, foundalternate[idxalt].Str)
							}

						} else {
							logger.Log.Debug("Skipped - no tvbdid: ", n.ToFilter[idx].NZB.Title)
							n.ToFilter[idx].Denied = true
							n.ToFilter[idx].Reason = "No Tvdb id"
							n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
							continue
						}
					} else {
						logger.Log.Debug("Skipped - Not Wanted Serie: ", n.ToFilter[idx].NZB.Title)
						n.ToFilter[idx].Denied = true
						n.ToFilter[idx].Reason = "Unwanted Serie"
						n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
						continue
					}
				}
			} else {
				loopepisode = n.T.SerieEpisode
				founddbserie, _ := database.GetDbserie(database.Query{Select: "seriename", Where: "id = ?", WhereArgs: []interface{}{n.T.SerieEpisode.DbserieID}})

				loopdbseries = founddbserie
			}
			n.ToFilter[idx].NzbepisodeID = loopepisode.ID
			if !config.ConfigCheck("quality_" + loopepisode.QualityProfile) {
				logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(loopepisode.ID)) + " not found")
				n.ToFilter[idx].Denied = true
				n.ToFilter[idx].Reason = "Quality Profile unknown"
				n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
				continue
			}

			n.ToFilter[idx].MinimumPriority = parser.GetHighestEpisodePriorityByFiles(loopepisode, n.T.ConfigTemplate, loopepisode.QualityProfile)

			if n.ToFilter[idx].MinimumPriority == 0 {
				//s.SearchActionType = "missing"
				if loopepisode.DontSearch {
					logger.Log.Debug("Skipped - Search disabled")
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Search Disabled"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			} else {
				//s.SearchActionType = "upgrade"
				if loopepisode.DontUpgrade {
					logger.Log.Debug("Skipped - Upgrade disabled")
					n.ToFilter[idx].Denied = true
					n.ToFilter[idx].Reason = "Upgrade Disabled"
					n.NzbsDenied = append(n.NzbsDenied, n.ToFilter[idx])
					continue
				}
			}
			n.ToFilter[idx].QualityTemplate = loopepisode.QualityProfile
			n.ToFilter[idx].WantedTitle = loopdbseries.Seriename
			foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbserie_alternates where dbserie_id=?", "Select count(id) from dbserie_alternates where dbserie_id=?", loopepisode.DbserieID)
			n.ToFilter[idx].WantedAlternates = make([]string, 0, len(foundalternate))
			for idxalt := range foundalternate {
				n.ToFilter[idx].WantedAlternates = append(n.ToFilter[idx].WantedAlternates, foundalternate[idxalt].Str)
			}
		}
		nextup = append(nextup, n.ToFilter[idx])
	}
	n.ToFilter = nextup
}

func (s searcher) GetRSSFeed(searchGroupType string, listConfig string) searchResults {
	list := config.ConfigGetMediaListConfig(s.ConfigTemplate, listConfig)
	if !config.ConfigCheck("list_" + list.Template_list) {
		return searchResults{}
	}
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return searchResults{}
	}
	cfg_quality := config.ConfigGet("quality_" + s.Quality).Data.(config.QualityConfig)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"

	var cfg_indexer config.QualityIndexerConfig
	for idx := range cfg_quality.Indexer {
		if cfg_quality.Indexer[idx].Template_indexer == list.Template_list {
			cfg_indexer = cfg_quality.Indexer[idx]
		}
	}
	if cfg_indexer.Template_regex == "" {
		return searchResults{}
	}

	configEntry := config.ConfigGet(s.ConfigTemplate).Data.(config.MediaTypeConfig)
	lists := make([]string, 0, len(configEntry.Lists))
	for idxlisttest := range configEntry.Lists {
		lists = append(lists, configEntry.Lists[idxlisttest].Name)
	}

	var lastindexerid string
	indexrssid, indexrssiderr := database.GetRssHistory(database.Query{Select: "last_id", Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{"list_" + list.Template_list, s.Quality, ""}})
	if indexrssiderr == nil {
		lastindexerid = indexrssid.LastID
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)
	indexer := apiexternal.NzbIndexer{Name: "list_" + list.Template_list, Customrssurl: cfg_list.Url, LastRssId: lastindexerid}
	s.Indexer = cfg_indexer
	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(indexer, cfg_list.Limit, []int{}, 1)
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {
		logger.Log.Error("Newznab RSS Search failed")
		failedindexer(failed)

	} else {
		if lastids != "" && len((*nzbs)) >= 1 {
			addrsshistory(indexer.URL, lastids, s.Quality, "list_"+list.Template_list)
		}
		tofilter := s.convertnzbs(nzbs)
		tofilter.nzbsFilterStart()
		tofilter.setDataField(lists, listConfig, list.Addfound)
		tofilter.nzbsFilterBlock2()

		defer func() {
			tofilter.Nzbs = nil
			tofilter.NzbsDenied = nil
			tofilter = nil
		}()
		if len(tofilter.Nzbs) > 1 {
			sort.Slice(tofilter.Nzbs, func(i, j int) bool {
				return tofilter.Nzbs[i].Prio > tofilter.Nzbs[j].Prio
			})
		}
		if len(tofilter.Nzbs) == 0 {
			logger.Log.Info("No new entries found")
		}
		return searchResults{Nzbs: tofilter.Nzbs}
	}
	return searchResults{}
}

func addrsshistory(url string, lastid string, quality string, config string) {
	database.UpsertArray("r_sshistories", []string{"indexer", "last_id", "list", "config"}, []interface{}{url, lastid, quality, config}, database.Query{Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{config, quality, url}})
}

func (s *searcher) MovieSearch(movie database.Movie, forceDownload bool, titlesearch bool) searchResults {
	s.SearchGroupType = "movie"
	if movie.DontSearch && !forceDownload {
		logger.Log.Debug("Skipped - Search disabled")
		return searchResults{}
	}
	s.Movie = movie

	imdb, _ := database.QueryColumnStatic("Select imdb_id from dbmovies where id=?", movie.DbmovieID)
	s.Imdb = imdb.(string)
	imdb = nil

	year, _ := database.QueryColumnStatic("Select year from dbmovies where id=?", movie.DbmovieID)
	s.Year = int(year.(int64))
	if s.Year == 0 {
		logger.Log.Debug("Skipped - No Year")
		return searchResults{}
	}
	year = nil

	title, _ := database.QueryColumnStatic("Select title from dbmovies where id=?", movie.DbmovieID)
	s.Name = title.(string)
	title = nil

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(s.Movie.ID)) + " not found")
		return searchResults{}
	}
	cfg_quality := config.ConfigGet("quality_" + s.Movie.QualityProfile).Data.(config.QualityConfig)
	s.Quality = s.Movie.QualityProfile

	s.MinimumPriority = parser.GetHighestMoviePriorityByFiles(movie, s.ConfigTemplate, s.Movie.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if movie.DontUpgrade && !forceDownload {
			logger.Log.Debug("Skipped - Upgrade disabled: ", s.Name)
			return searchResults{}
		}
	}

	var dl searchResults

	processedindexer := 0
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := sizedwaitgroup.New(cfg_general.WorkerIndexer)
	for idx := range cfg_quality.Indexer {
		swi.Add()
		go func(index config.QualityIndexerConfig) {
			dladd, ok := s.moviesearchindexer(index, movie, titlesearch, &swi)
			if ok {
				processedindexer += 1
				if len(dl.Rejected) == 0 {
					dl.Rejected = dladd.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dladd.Rejected...)
				}
				if len(dl.Nzbs) == 0 {
					dl.Nzbs = dladd.Nzbs
				} else {
					dl.Nzbs = append(dl.Nzbs, dladd.Nzbs...)
				}
			}
		}(cfg_quality.Indexer[idx])
	}
	swi.Wait()
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

func (t searcher) moviesearchindexer(index config.QualityIndexerConfig, movie database.Movie, titlesearch bool, swi *sizedwaitgroup.SizedWaitGroup) (searchResults, bool) {
	defer swi.Done()

	_, nzbindexer, cats, erri := t.initIndexer(index, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", index.Template_indexer, " ", erri)

		return searchResults{}, false
	}
	t.Indexer = index

	var dl searchResults
	releasefound := false
	if !config.ConfigCheck("quality_" + t.Movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(t.Movie.ID)) + " not found")
		return searchResults{}, false
	}
	cfg_quality := config.ConfigGet("quality_" + t.Movie.QualityProfile).Data.(config.QualityConfig)
	if t.Imdb != "" {
		logger.Log.Info("Search movie by imdb ", t.Imdb, " with indexer ", index.Template_indexer)
		dl_add := t.moviesSearchImdb(movie, []string{t.Movie.Listname}, cats)
		dl.Rejected = dl_add.Rejected
		if len(dl_add.Nzbs) >= 1 {
			logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
			releasefound = true
			dl.Nzbs = dl_add.Nzbs
			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
				return dl, true
			}
		}
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	titleschecked := []string{}
	if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
		if checkindexerblock(cfg_general.FailedIndexerBlockTime, nzbindexer.URL) {
			return dl, true
		}
		logger.Log.Info("Search movie by title ", t.Name, " with indexer ", index.Template_indexer)
		dl_add := t.moviesSearchTitle(movie, t.Name, []string{t.Movie.Listname}, cats)
		if len(dl.Rejected) == 0 {
			dl.Rejected = dl_add.Rejected
		} else {
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
		}
		titleschecked = append(titleschecked, t.Name)
		if len(dl_add.Nzbs) >= 1 {
			logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
			releasefound = true
			if len(dl.Nzbs) == 0 {
				dl.Nzbs = dl_add.Nzbs
			} else {
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
				return dl, true
			}
		}
	}
	if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
		foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbmovie_titles where dbmovie_id=?", "Select count(id) from dbmovie_titles where dbmovie_id=?", t.Movie.DbmovieID)
		for idxalt := range foundalternate {
			if checkindexerblock(cfg_general.FailedIndexerBlockTime, nzbindexer.URL) {
				continue
			}
			if !logger.CheckStringArray(titleschecked, foundalternate[idxalt].Str) {
				logger.Log.Info("Search movie by title ", foundalternate[idxalt].Str, " with indexer ", index.Template_indexer)
				dl_add := t.moviesSearchTitle(movie, foundalternate[idxalt].Str, []string{t.Movie.Listname}, cats)
				if len(dl.Rejected) == 0 {
					dl.Rejected = dl_add.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
				}
				titleschecked = append(titleschecked, foundalternate[idxalt].Str)
				if len(dl_add.Nzbs) >= 1 {
					logger.Log.Debug("Indexer loop - entries found: ", len(dl_add.Nzbs))
					releasefound = true
					if len(dl.Nzbs) == 0 {
						dl.Nzbs = dl_add.Nzbs
					} else {
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
					}
					if cfg_quality.CheckUntilFirstFound {
						logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
						break
					}
				}
			}
		}
		if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
			logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
			return dl, true
		}
	}
	return dl, true
}

func (s *searcher) SeriesSearch(serieEpisode database.SerieEpisode, forceDownload bool, titlesearch bool) searchResults {
	s.SearchGroupType = "series"
	if serieEpisode.DontSearch && !forceDownload {
		logger.Log.Debug("Search not wanted: ")
		return searchResults{}
	}
	s.SerieEpisode = serieEpisode

	tvdb, _ := database.QueryColumnStatic("Select thetvdb_id from dbseries where id=?", serieEpisode.DbserieID)
	s.Tvdb = int(tvdb.(int64))
	tvdb = nil

	title, _ := database.QueryColumnStatic("Select seriename from dbseries where id=?", serieEpisode.DbserieID)
	s.Name = title.(string)
	title = nil

	season, _ := database.QueryColumnStatic("Select season from dbserie_episodes where id=?", serieEpisode.DbserieEpisodeID)
	s.Season = season.(string)
	season = nil
	episode, _ := database.QueryColumnStatic("Select episode from dbserie_episodes where id=?", serieEpisode.DbserieEpisodeID)
	s.Episode = episode.(string)
	episode = nil
	identifier, _ := database.QueryColumnStatic("Select identifier from dbserie_episodes where id=?", serieEpisode.DbserieEpisodeID)
	s.Identifier = identifier.(string)
	identifier = nil

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(s.SerieEpisode.ID)) + " not found")
		return searchResults{}
	}
	cfg_quality := config.ConfigGet("quality_" + s.SerieEpisode.QualityProfile).Data.(config.QualityConfig)
	s.Quality = s.SerieEpisode.QualityProfile
	s.MinimumPriority = parser.GetHighestEpisodePriorityByFiles(serieEpisode, s.ConfigTemplate, s.SerieEpisode.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if serieEpisode.DontUpgrade && !forceDownload {
			logger.Log.Debug("Upgrade not wanted: ", s.Name)
			return searchResults{}
		}
	}

	var dl searchResults

	series, _ := database.GetSeries(database.Query{Select: "listname", Where: "id=?", WhereArgs: []interface{}{serieEpisode.SerieID}})

	processedindexer := 0
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := sizedwaitgroup.New(cfg_general.WorkerIndexer)
	for idx := range cfg_quality.Indexer {
		swi.Add()
		go func(index config.QualityIndexerConfig) {
			dladd, ok := s.seriessearchindexer(index, series, titlesearch, &swi)
			if ok {
				processedindexer += 1
				if len(dl.Rejected) == 0 {
					dl.Rejected = dladd.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dladd.Rejected...)
				}
				if len(dl.Nzbs) == 0 {
					dl.Nzbs = dladd.Nzbs
				} else {
					dl.Nzbs = append(dl.Nzbs, dladd.Nzbs...)
				}
			}
		}(cfg_quality.Indexer[idx])
	}
	swi.Wait()
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

func (t searcher) seriessearchindexer(index config.QualityIndexerConfig, series database.Serie, titlesearch bool, swi *sizedwaitgroup.SizedWaitGroup) (searchResults, bool) {
	defer swi.Done()
	_, nzbindexer, cats, erri := t.initIndexer(index, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", index.Template_indexer, " ", erri)
		return searchResults{}, false
	}
	t.Indexer = index

	if !config.ConfigCheck("quality_" + t.SerieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(t.SerieEpisode.ID)) + " not found")
		return searchResults{}, false
	}
	cfg_quality := config.ConfigGet("quality_" + t.SerieEpisode.QualityProfile).Data.(config.QualityConfig)

	releasefound := false
	var dl searchResults
	if t.Tvdb != 0 {
		logger.Log.Info("Search serie by tvdbid ", t.Tvdb, " with indexer ", index.Template_indexer)
		dl_add := t.seriesSearchTvdb(t.SerieEpisode, []string{series.Listname}, cats)
		dl.Rejected = dl_add.Rejected
		if len(dl_add.Nzbs) >= 1 {
			releasefound = true
			dl.Nzbs = dl_add.Nzbs
			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
				return dl, true
			}
		}
	}
	titleschecked := []string{}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
		if checkindexerblock(cfg_general.FailedIndexerBlockTime, nzbindexer.URL) {
			return dl, true
		}
		logger.Log.Info("Search serie by title ", t.Name, " with indexer ", index.Template_indexer)
		dl_add := t.seriesSearchTitle(t.SerieEpisode, t.Name, []string{series.Listname}, cats)
		if len(dl.Rejected) == 0 {
			dl.Rejected = dl_add.Rejected
		} else {
			dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
		}
		titleschecked = append(titleschecked, t.Name)
		if len(dl_add.Nzbs) >= 1 {
			releasefound = true
			if len(dl.Nzbs) == 0 {
				dl.Nzbs = dl_add.Nzbs
			} else {
				dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
			}
			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
				return dl, true
			}
		}
	}
	if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
		foundalternate, _ := database.QueryStaticColumnsOneString("Select title from dbserie_alternates where dbserie_id=?", "Select count(id) from dbserie_alternates where dbserie_id=?", t.SerieEpisode.DbserieID)
		for idxalt := range foundalternate {
			if checkindexerblock(cfg_general.FailedIndexerBlockTime, nzbindexer.URL) {
				continue
			}
			if !logger.CheckStringArray(titleschecked, foundalternate[idxalt].Str) {
				logger.Log.Info("Search serie by title ", foundalternate[idxalt].Str, " with indexer ", index.Template_indexer)
				dl_add := t.seriesSearchTitle(t.SerieEpisode, foundalternate[idxalt].Str, []string{series.Listname}, cats)
				if len(dl.Rejected) == 0 {
					dl.Rejected = dl_add.Rejected
				} else {
					dl.Rejected = append(dl.Rejected, dl_add.Rejected...)
				}
				titleschecked = append(titleschecked, foundalternate[idxalt].Str)
				if len(dl_add.Nzbs) >= 1 {
					releasefound = true
					if len(dl.Nzbs) == 0 {
						dl.Nzbs = dl_add.Nzbs
					} else {
						dl.Nzbs = append(dl.Nzbs, dl_add.Nzbs...)
					}
					if cfg_quality.CheckUntilFirstFound {
						logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
						break
					}
				}
			}
		}
		if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
			logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
			return dl, true
		}
	}
	return dl, true
}

func checkindexerblock(FailedIndexerBlockTime int, url string) bool {
	blockinterval := -5
	if FailedIndexerBlockTime != 0 {
		blockinterval = -1 * FailedIndexerBlockTime
	}
	lastfailed := sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer=? and last_fail > ?", url, lastfailed)
	if counter >= 1 {
		return true
	} else {
		return false
	}
}

func (s *searcher) initIndexer(indexer config.QualityIndexerConfig, rssapi string) (config.IndexersConfig, apiexternal.NzbIndexer, []int, error) {
	logger.Log.Debug("Indexer search: ", indexer.Template_indexer)

	if !config.ConfigCheck("indexer_" + indexer.Template_indexer) {
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, []int{}, errors.New("indexer config missing")
	}
	cfg_indexer := config.ConfigGet("indexer_" + indexer.Template_indexer).Data.(config.IndexersConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !(strings.ToLower(cfg_indexer.Type) == "newznab") {
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, []int{}, errors.New("indexer Type Wrong")
	}
	if !cfg_indexer.Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, []int{}, errors.New("indexer disabled")
	} else if !cfg_indexer.Enabled {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, []int{}, errors.New("indexer disabled")
	}

	userid, _ := strconv.Atoi(cfg_indexer.Userid)

	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	lastfailed := sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer=? and last_fail > ?", cfg_indexer.Url, lastfailed)
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, []int{}, errors.New("indexer disabled")
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		indexrssid, indexrssiderr := database.GetRssHistory(database.Query{Select: "last_id", Where: "config=? and list=? and indexer=?", WhereArgs: []interface{}{s.ConfigTemplate, s.Quality, cfg_indexer.Url}})
		if indexrssiderr == nil {
			lastindexerid = indexrssid.LastID
		}
	}

	nzbindexer := apiexternal.NzbIndexer{
		URL:                     cfg_indexer.Url,
		Apikey:                  cfg_indexer.Apikey,
		UserID:                  userid,
		SkipSslCheck:            cfg_indexer.DisableTLSVerify,
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
		return cfg_indexer, nzbindexer, cats, nil
	} else {
		intcat, _ := strconv.Atoi(indexer.Categories_indexer)
		return cfg_indexer, nzbindexer, []int{intcat}, nil
	}
}

func (s *searcher) moviesSearchImdb(movie database.Movie, lists []string, cats []int) searchResults {
	logger.Log.Info("Search Movie by imdb: ", s.Imdb)
	_, nzbindexer, _, _ := s.initIndexer(s.Indexer, "api")
	nzbs, failed, nzberr := apiexternal.QueryNewznabMovieImdb(nzbindexer, strings.Trim(s.Imdb, "t"), cats)
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Imdb, " with ", nzbindexer.URL, " error ", nzberr)
		failedindexer(failed)

		return searchResults{}
	}

	if len((*nzbs)) >= 1 {
		tofilter := s.convertnzbs(nzbs)
		tofilter.nzbsFilterStart()
		tofilter.setDataField(lists, "", false)
		tofilter.nzbsFilterBlock2()

		defer func() {
			tofilter.Nzbs = nil
			tofilter.NzbsDenied = nil
			tofilter = nil
		}()

		return searchResults{Nzbs: tofilter.Nzbs, Rejected: tofilter.NzbsDenied}
	}
	return searchResults{}
}

func (s *searcher) moviesSearchTitle(movie database.Movie, title string, lists []string, cats []int) searchResults {
	if len(title) == 0 {
		return searchResults{Nzbs: []parser.Nzbwithprio{}}
	}
	if !config.ConfigCheck("quality_" + movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(movie.ID)) + " not found")
		return searchResults{}
	}
	cfg_quality := config.ConfigGet("quality_" + movie.QualityProfile).Data.(config.QualityConfig)

	searchfor := title + " (" + strconv.Itoa(s.Year) + ")"
	if cfg_quality.ExcludeYearFromTitleSearch {
		searchfor = title
	}
	logger.Log.Info("Search Movie by name: ", title)
	_, nzbindexer, _, _ := s.initIndexer(s.Indexer, "api")
	nzbs, failed, nzberr := apiexternal.QueryNewznabQuery(nzbindexer, searchfor, cats, "movie")
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", title, " with ", nzbindexer.URL, " error ", nzberr)
		failedindexer(failed)

		return searchResults{}
	}

	if len((*nzbs)) >= 1 {
		tofilter := s.convertnzbs(nzbs)
		tofilter.nzbsFilterStart()
		tofilter.setDataField(lists, "", false)
		tofilter.nzbsFilterBlock2()

		defer func() {
			tofilter.Nzbs = nil
			tofilter.NzbsDenied = nil
			tofilter = nil
		}()
		return searchResults{Nzbs: tofilter.Nzbs, Rejected: tofilter.NzbsDenied}
	}
	return searchResults{}
}

func failedindexer(failed []string) {
	for _, failedidx := range failed {
		database.UpsertArray("indexer_fails", []string{"indexer", "last_fail"}, []interface{}{failedidx, time.Now()}, database.Query{Where: "indexer=?", WhereArgs: []interface{}{failedidx}})
	}
}

func (s *searcher) seriesSearchTvdb(serieEpisode database.SerieEpisode, lists []string, cats []int) searchResults {
	_, nzbindexer, _, _ := s.initIndexer(s.Indexer, "api")
	logger.Log.Info("Search Series by tvdbid: ", s.Tvdb, " S", s.Season, "E", s.Episode)
	seasonint, _ := strconv.Atoi(s.Season)
	episodeint, _ := strconv.Atoi(s.Episode)
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb(nzbindexer, s.Tvdb, cats, seasonint, episodeint, true, true)
	defer func() {
		nzbs = nil
	}()
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Tvdb, " with ", nzbindexer.URL, " error ", nzberr)
		failedindexer(failed)

		return searchResults{}
	}

	if len((*nzbs)) >= 1 {
		tofilter := s.convertnzbs(nzbs)
		tofilter.nzbsFilterStart()
		tofilter.setDataField(lists, "", false)
		tofilter.nzbsFilterBlock2()

		defer func() {
			tofilter.Nzbs = nil
			tofilter.NzbsDenied = nil
			tofilter = nil
		}()
		return searchResults{Nzbs: tofilter.Nzbs, Rejected: tofilter.NzbsDenied}
	}
	return searchResults{}
}

func (s *searcher) seriesSearchTitle(serieEpisode database.SerieEpisode, title string, lists []string, cats []int) searchResults {
	if !config.ConfigCheck("quality_" + serieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(serieEpisode.ID)) + " not found")
		return searchResults{}
	}
	cfg_quality := config.ConfigGet("quality_" + serieEpisode.QualityProfile).Data.(config.QualityConfig)

	_, nzbindexer, _, _ := s.initIndexer(s.Indexer, "api")
	if title != "" && s.Identifier != "" && cfg_quality.BackupSearchForTitle {
		logger.Log.Info("Search Series by title: ", title, " ", s.Identifier)
		searchfor := title + " " + s.Identifier
		nzbs, failed, nzberr := apiexternal.QueryNewznabQuery(nzbindexer, searchfor, cats, "search")
		defer func() {
			nzbs = nil
		}()
		if nzberr != nil {
			logger.Log.Error("Newznab Search failed: ", title, " with ", nzbindexer.URL, " error ", nzberr)
			failedindexer(failed)

			return searchResults{}
		}

		if len((*nzbs)) >= 1 {
			tofilter := s.convertnzbs(nzbs)
			tofilter.nzbsFilterStart()
			tofilter.setDataField(lists, "", false)
			tofilter.nzbsFilterBlock2()

			defer func() {
				tofilter.Nzbs = nil
				tofilter.NzbsDenied = nil
				tofilter = nil
			}()

			return searchResults{Nzbs: tofilter.Nzbs, Rejected: tofilter.NzbsDenied}
		}

	}
	return searchResults{}
}

func allowMovieImport(imdb string, listTemplate string) bool {
	if !config.ConfigCheck("list_" + listTemplate) {
		return false
	}
	list := config.ConfigGet("list_" + listTemplate).Data.(config.ListsConfig)
	if list.MinVotes != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_ratings where tconst = ? and num_votes < ?", imdb, list.MinVotes)
		if countergenre >= 1 {
			logger.Log.Debug("error vote count too low for", imdb)
			return false
		}
	}
	if list.MinRating != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_ratings where tconst = ? and average_rating < ?", imdb, list.MinRating)
		if countergenre >= 1 {
			logger.Log.Debug("error average vote too low for", imdb)
			return false
		}
	}
	if len(list.Excludegenre) >= 1 {
		excludebygenre := false
		for idxgenre := range list.Excludegenre {
			countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Excludegenre[idxgenre])
			if countergenre >= 1 {
				excludebygenre = true
				logger.Log.Debug("error excluded genre", list.Excludegenre[idxgenre], imdb)
				break
			}
		}
		if excludebygenre {
			return false
		}
	}
	if len(list.Includegenre) >= 1 {
		includebygenre := false
		for idxgenre := range list.Includegenre {
			countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Includegenre[idxgenre])
			if countergenre >= 1 {
				includebygenre = true
				break
			}
		}
		if !includebygenre {
			logger.Log.Debug("error included genre not found", list.Includegenre, imdb)
			return false
		}
	}
	return true
}
