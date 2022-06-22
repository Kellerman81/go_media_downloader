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
	"github.com/shomali11/parallelizer"
)

func SearchMovieMissing(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	var scandatepre int

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

	argslist := config.MedialistConfigToInterfaceArray(configEntry.Lists)
	defer logger.ClearVar(&argslist)

	argsscan := append(argslist, scantime)
	defer logger.ClearVar(&argsscan)
	var qu database.Query
	qu.Select = "movies.id"
	qu.InnerJoin = "dbmovies on dbmovies.id=movies.dbmovie_id"
	qu.OrderBy = "movies.Lastscan asc"
	if scaninterval != 0 {
		qu.Where = "movies.missing = 1 AND movies.listname in (?" + strings.Repeat(",?", len(argslist)-1) + ") AND (movies.lastscan is null or movies.Lastscan < ?)"
		qu.WhereArgs = argsscan
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			qu.WhereArgs = append(argsscan, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	} else {
		qu.Where = "movies.missing = 1 AND movies.listname in (?" + strings.Repeat(",?", len(argslist)-1) + ")"
		qu.WhereArgs = argslist
		if scandatepre != 0 {
			qu.Where += " and (dbmovies.release_date < ? or dbmovies.release_date is null)"
			qu.WhereArgs = append(argslist, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingmovies, _ := database.QueryStaticColumnsOneIntQueryObject("movies", qu)
	defer logger.ClearVar(&missingmovies)

	// searchnow := NewSearcher(configEntry, list)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range missingmovies {
		missing := missingmovies[idx]
		swg.Add(func() {
			SearchMovieSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func downloadMovieSearchResults(first bool, movieid uint, configTemplate string, searchtype string, searchresults SearchResults) {
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := downloader.NewDownloader(configTemplate)
		defer downloadnow.Close()
		downloadnow.SetMovie(movieid)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}
func SearchMovieSingle(movieid uint, configTemplate string, titlesearch bool) {
	searchtype := "missing"
	missing, _ := database.QueryColumnBool("Select missing from movies where id = ?", movieid)
	if !missing {
		searchtype = "upgrade"
	}
	quality, _ := database.QueryColumnString("Select quality_profile from movies where id = ?", movieid)

	searchnow := NewSearcher(configTemplate, quality)
	defer searchnow.Close()
	results, err := searchnow.MovieSearch(movieid, false, titlesearch)
	defer logger.ClearVar(&results)
	if err == nil {
		downloadMovieSearchResults(true, movieid, configTemplate, searchtype, results)
	}
}

func SearchMovieUpgrade(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
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
	argslist := config.MedialistConfigToInterfaceArray(configEntry.Lists)
	defer logger.ClearVar(&argslist)
	argsscan := append(argslist, scantime)
	defer logger.ClearVar(&argsscan)

	qu := database.Query{}
	if scaninterval != 0 {
		qu.Where = "quality_reached = 0 and missing = 0 AND listname in (?" + strings.Repeat(",?", len(argslist)-1) + ") AND (lastscan is null or Lastscan < ?)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "quality_reached = 0 and missing = 0 AND listname in (?" + strings.Repeat(",?", len(argslist)-1) + ")"
		qu.WhereArgs = argslist
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	qu.Select = "id"
	qu.OrderBy = "Lastscan asc"
	missingmovies, _ := database.QueryStaticColumnsOneIntQueryObject("movies", qu)
	defer logger.ClearVar(&missingmovies)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range missingmovies {
		missing := missingmovies[idx]
		swg.Add(func() {
			SearchMovieSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func SearchSerieSingle(serieid uint, configTemplate string, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	episodes, _ := database.QueryStaticColumnsOneInt("Select id from serie_episodes where serie_id = ?", "Select count(id) from serie_episodes where serie_id = ?", serieid)
	defer logger.ClearVar(&episodes)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range episodes {
		missing := episodes[idx]
		swg.Add(func() {
			SearchSerieEpisodeSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func SearchSerieSeasonSingle(serieid uint, season string, configTemplate string, titlesearch bool) {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	episodes, _ := database.QueryStaticColumnsOneInt("Select id from serie_episodes where serie_id = ? and dbserie_episode_id IN (Select id from dbserie_episodes where Season = ?)", "Select count(id) from serie_episodes where serie_id = ? and dbserie_episode_id IN (Select id from dbserie_episodes where Season = ?)", serieid, season)
	defer logger.ClearVar(&episodes)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range episodes {
		missing := episodes[idx]
		swg.Add(func() {
			SearchSerieEpisodeSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func SearchSerieRSSSeasonSingle(serieid uint, season int, useseason bool, configTemplate string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}

	qualstr, _ := database.QueryColumnString("Select quality_profile from serie_episodes where serie_id = ? limit 1", serieid)
	if qualstr == "" {
		return
	}
	dbserieid, err := database.QueryColumnUint("Select dbserie_id from series where id = ?", serieid)
	tvdb, err := database.QueryColumnUint("Select thetvdb_id from dbseries where id = ?", dbserieid)
	if err != nil {
		return
	}
	SearchSerieRSSSeason(configTemplate, qualstr, int(tvdb), season, useseason)
}
func SearchSeriesRSSSeasons(configTemplate string) {
	var series []database.Serie
	defer logger.ClearVar(&series)
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)

	series, _ = database.QuerySeries(database.Query{
		Select:    "id",
		Where:     "listname in (?" + strings.Repeat(",?", len(configEntry.Lists)-1) + ") AND (Select Count(*) FROM serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != 0 and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1",
		WhereArgs: config.MedialistConfigToInterfaceArray(configEntry.Lists)})
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerSearch == 0 {
		cfg_general.WorkerSearch = 1
	}
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	var seasons []string
	defer logger.ClearVar(&seasons)
	for idx := range series {
		missing := series[idx]
		seasons = database.QueryStaticStringArray("Select distinct season from dbserie_episodes where dbserie_id = ?", "Select count(distinct season) from dbserie_episodes where dbserie_id = ?", missing.DbserieID)
		for idx := range seasons {
			season := seasons[idx]
			if season == "" {
				continue
			}
			swg.Add(func() {
				seasonint, err := strconv.Atoi(season)
				if err == nil {
					SearchSerieRSSSeasonSingle(missing.ID, seasonint, true, configTemplate)
				}
			})
		}
	}
	swg.Wait()
	swg.Close()
}
func SearchSerieEpisodeSingle(episodeid uint, configTemplate string, titlesearch bool) {
	quality, _ := database.QueryColumnString("Select quality_profile from serie_episodes where id = ?", episodeid)

	searchnow := NewSearcher(configTemplate, quality)
	defer searchnow.Close()
	searchresults, err := searchnow.SeriesSearch(episodeid, false, titlesearch)
	if err != nil {
		return
	}
	defer logger.ClearVar(&searchresults)
	if len(searchresults.Nzbs) >= 1 {
		logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[0].NZB.Title)
		downloadnow := downloader.NewDownloader(configTemplate)
		defer downloadnow.Close()
		downloadnow.SetSeriesEpisode(episodeid)
		downloadnow.DownloadNzb(searchresults.Nzbs[0])
	}
}
func SearchSerieMissing(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
	var scandatepre int
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
	argslist := config.MedialistConfigToInterfaceArray(configEntry.Lists)
	defer logger.ClearVar(&argslist)
	argsscan := append(argslist, scantime)
	defer logger.ClearVar(&argsscan)

	var qu database.Query
	qu.Select = "serie_episodes.id"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.id=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"
	if scaninterval != 0 {
		qu.Where = "serie_episodes.missing = 1 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(argslist)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = argsscan
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			qu.WhereArgs = append(argsscan, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	} else {
		qu.Where = "serie_episodes.missing = 1 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(argslist)-1) + ") and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = argslist
		if scandatepre != 0 {
			qu.Where += " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"
			qu.WhereArgs = append(argslist, time.Now().AddDate(0, 0, 0+scandatepre))
		}
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	missingepisode, _ := database.QueryStaticColumnsOneIntQueryObject("serie_episodes", qu)
	defer logger.ClearVar(&missingepisode)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range missingepisode {
		missing := missingepisode[idx]
		swg.Add(func() {
			SearchSerieEpisodeSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func SearchSerieUpgrade(configTemplate string, jobcount int, titlesearch bool) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var scaninterval int
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
	args := config.MedialistConfigToInterfaceArray(configEntry.Lists)
	defer logger.ClearVar(&args)

	var qu database.Query
	qu.Select = "serie_episodes.ID"
	qu.OrderBy = "Lastscan asc"
	qu.InnerJoin = "dbserie_episodes on dbserie_episodes.ID=serie_episodes.Dbserie_episode_id inner join series on series.id=serie_episodes.serie_id"
	if scaninterval != 0 {
		argsscan := append(args, scantime)
		defer logger.ClearVar(&argsscan)
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(args)-1) + ") AND (serie_episodes.lastscan is null or serie_episodes.Lastscan < ?) and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = argsscan
	} else {
		qu.Where = "serie_episodes.missing = 0 AND serie_episodes.quality_reached = 0 AND ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?" + strings.Repeat(",?", len(args)-1) + ") and serie_episodes.dbserie_episode_id in (SELECT id FROM dbserie_episodes GROUP BY dbserie_id, identifier HAVING COUNT(*) = 1)"
		qu.WhereArgs = args
	}
	if jobcount >= 1 {
		qu.Limit = uint64(jobcount)
	}
	//missingepisode, _ := database.QuerySerieEpisodes(qu)
	missingepisode, _ := database.QueryStaticColumnsOneIntQueryObject("serie_episodes", qu)
	defer logger.ClearVar(&missingepisode)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerSearch))
	for idx := range missingepisode {
		missing := missingepisode[idx]
		swg.Add(func() {
			SearchSerieEpisodeSingle(uint(missing.Num), configTemplate, titlesearch)
		})
	}
	swg.Wait()
	swg.Close()
}

func downloadSerieNzb(searchresults SearchResults, configTemplate string) {
	var downloaded []uint
	defer logger.ClearVar(&downloaded)
	var breakfor bool
	for idx := range searchresults.Nzbs {
		breakfor = false
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

		downloadnow := downloader.NewDownloader(configTemplate)
		downloadnow.SetSeriesEpisode(searchresults.Nzbs[idx].NzbepisodeID)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
		downloadnow.Close()
	}
}
func SearchSerieRSS(configTemplate string, quality string) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(configTemplate, quality)
	defer searchnow.Close()
	results, err := searchnow.SearchRSS("series", false)
	defer logger.ClearVar(&results)
	if err == nil {
		downloadSerieNzb(results, configTemplate)
	}
}

func SearchSerieRSSSeason(configTemplate string, quality string, thetvdb_id int, season int, useseason bool) {
	logger.Log.Debug("Get Rss Series List")

	searchnow := NewSearcher(configTemplate, quality)
	defer searchnow.Close()
	results, err := searchnow.SearchSeriesRSSSeason("series", thetvdb_id, season, useseason)
	defer logger.ClearVar(&results)
	if err == nil {
		downloadSerieNzb(results, configTemplate)
	}
}

func downloadMovieNzb(searchresults SearchResults, configTemplate string) {
	var downloaded []uint
	defer logger.ClearVar(&downloaded)
	var breakfor bool
	for idx := range searchresults.Nzbs {
		breakfor = false
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
		downloadnow := downloader.NewDownloader(configTemplate)
		downloadnow.SetMovie(searchresults.Nzbs[idx].NzbmovieID)
		downloadnow.DownloadNzb(searchresults.Nzbs[idx])
		downloadnow.Close()
	}
}
func SearchMovieRSS(configTemplate string, quality string) {
	logger.Log.Debug("Get Rss Movie List")

	searchnow := NewSearcher(configTemplate, quality)
	defer searchnow.Close()
	results, err := searchnow.SearchRSS("movie", false)
	defer logger.ClearVar(&results)
	if err == nil {
		downloadMovieNzb(results, configTemplate)
	}
}

type SearchResults struct {
	Nzbs     []parser.Nzbwithprio
	Rejected []parser.Nzbwithprio
}

type Searcher struct {
	ConfigTemplate   string
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	MinimumPriority  int
	Movie            database.Movie
	SerieEpisode     database.SerieEpisode
	Imdb             string
	Year             int
	Tvdb             int
	Season           string
	Episode          string
	Identifier       string
	Name             string

	Listname string
}

//func NewSearcher(configTemplate string, quality string) Searcher {
func NewSearcher(template string, quality string) Searcher {
	return Searcher{
		ConfigTemplate: template,
		Quality:        quality,
	}
}

func (s *Searcher) Close() {
	s = nil
}

//searchGroupType == movie || series
func (s *Searcher) SearchRSS(searchGroupType string, fetchall bool) (SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return SearchResults{}, errors.New("quality not found")
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	var dl SearchResults
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	cfg_quality := config.ConfigGet("quality_" + s.Quality).Data.(config.QualityConfig)
	swi := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerIndexer))
	for idx := range cfg_quality.Indexer {
		index := cfg_quality.Indexer[idx]
		swi.Add(func() {
			s.rsssearchindexer(cfg_quality.Name, index.Template_indexer, fetchall, &dl)
		})
	}
	swi.Wait()
	swi.Close()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	if len(dl.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return dl, nil
}

func (t *Searcher) rsssearchindexer(quality string, indexer string, fetchall bool, dl *SearchResults) bool {
	cfgindex, nzbindexer, cats, erri := t.initIndexer(quality, indexer, "rss")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", indexer, " ", erri)
		return false
	}
	defer logger.ClearVar(&cfgindex)
	defer logger.ClearVar(&nzbindexer)
	if fetchall {
		nzbindexer.LastRssId = ""
	}
	if cfgindex.MaxRssEntries == 0 {
		cfgindex.MaxRssEntries = 10
	}
	if cfgindex.RssEntriesloop == 0 {
		cfgindex.RssEntriesloop = 2
	}
	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(nzbindexer, cfgindex.MaxRssEntries, cats, cfgindex.RssEntriesloop)
	defer logger.ClearVar(&nzbs)

	if nzberr != nil {

		logger.Log.Error("Newznab RSS Search failed ", indexer)
		failedindexer(failed)
		return false
	} else {
		if !fetchall {
			if lastids != "" && len((nzbs)) >= 1 {
				addrsshistory(nzbindexer.URL, lastids, t.Quality, t.ConfigTemplate)
			}
		}
		logger.Log.Debug("Search RSS ended - found entries: ", len((nzbs)))
		if len((nzbs)) >= 1 {

			if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
				for idx := range nzbs {
					dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
						NZB:     nzbs[idx],
						Indexer: indexer,
						Denied:  true,
						Reason:  "Denied by Regex"})
				}
			} else {
				for idx := range nzbs {
					t.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
				}
			}
		}
	}
	return true
}

//searchGroupType == movie || series
func (s *Searcher) SearchSeriesRSSSeason(searchGroupType string, thetvdb_id int, season int, useseason bool) (SearchResults, error) {
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return SearchResults{}, errors.New("quality not found")
	}
	// configEntry := config.ConfigGet(s.ConfigTemplate).Data.(config.MediaTypeConfig)

	cfg_quality := config.ConfigGet("quality_" + s.Quality).Data.(config.QualityConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	s.SearchGroupType = searchGroupType
	s.SearchActionType = "rss"
	var dl SearchResults
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerIndexer))
	for idx := range cfg_quality.Indexer {
		index := cfg_quality.Indexer[idx]
		swi.Add(func() {
			s.rssqueryseriesindexer(cfg_quality.Name, index.Template_indexer, thetvdb_id, season, useseason, &dl)
		})
	}
	swi.Wait()
	swi.Close()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}
	if len(dl.Nzbs) == 0 {
		logger.Log.Info("No new entries found")
	}
	return dl, nil
}

func (t *Searcher) rssqueryseriesindexer(quality string, indexer string, thetvdb_id int, season int, useseason bool, dl *SearchResults) bool {
	_, nzbindexer, cats, erri := t.initIndexer(quality, indexer, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", indexer, " ", erri)
		return false
	}
	defer logger.ClearVar(&nzbindexer)
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb(nzbindexer, thetvdb_id, cats, season, 0, useseason, false)
	defer logger.ClearVar(&nzbs)

	if nzberr != nil {

		logger.Log.Error("Newznab RSS Search failed ", indexer)
		failedindexer(failed)
		return false
	} else {
		logger.Log.Debug("Search RSS ended - found entries: ", len((nzbs)))
		if len((nzbs)) >= 1 {
			if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
				for idx := range nzbs {
					dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
						NZB:     nzbs[idx],
						Indexer: indexer,
						Denied:  true,
						Reason:  "Denied by Regex"})
				}
			} else {
				for idx := range nzbs {
					t.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
				}
			}
		}
	}
	return true
}

type nzbFilter struct {
	T          Searcher
	ToFilter   []parser.Nzbwithprio
	Nzbs       []parser.Nzbwithprio
	NzbsDenied []parser.Nzbwithprio
}

func (n *nzbFilter) Close() {
	if n == nil {
		return
	}
	if len(n.Nzbs) >= 1 {
		n.Nzbs = nil
	}
	if len(n.NzbsDenied) >= 1 {
		n.NzbsDenied = nil
	}
	if len(n.ToFilter) >= 1 {
		n.ToFilter = nil
	}
	n = nil
}
func (s *Searcher) convertnzbs(quality string, indexer string, nzb apiexternal.NZB, addinlist string, addifnotfound bool, dl *SearchResults) {
	var err error
	var entry parser.Nzbwithprio
	var counter int
	entry.NZB = nzb
	entry.Indexer = indexer
	defer logger.ClearVar(&entry)
	//Check Title Length
	if len(nzb.DownloadURL) == 0 {
		denynzb("No Url", entry, dl)
		return
	}
	if len(nzb.Title) == 0 {
		denynzb("No Title", entry, dl)
		return
	}
	if len(strings.Trim(nzb.Title, " ")) <= 3 {
		denynzb("Title too short", entry, dl)
		return
	}

	//Check Size
	index := config.QualityIndexerByQualityAndTemplate(quality, indexer)
	if index == nil {
		denynzb("No Indexer", entry, dl)
		return
	}
	if index.Filter_size_nzbs(s.ConfigTemplate, nzb.Title, nzb.Size) {
		denynzb("Wrong size", entry, dl)
		return
	}

	//Check History
	historytable := "serie_episode_histories"
	if strings.ToLower(s.SearchGroupType) == "movie" {
		historytable = "movie_histories"
	}
	if len(nzb.DownloadURL) > 1 {
		counter, _ = database.CountRowsStatic("Select count(id) from "+historytable+" where url = ?", nzb.DownloadURL)
		if counter >= 1 {
			denynzb("Already downloaded", entry, dl)
			return
		}
	}
	if index.History_check_title && len(nzb.Title) > 1 {
		counter, _ = database.CountRowsStatic("Select count(id) from "+historytable+" where title = ? COLLATE NOCASE", nzb.Title)
		if counter >= 1 {
			denynzb("Already downloaded (Title)", entry, dl)
			return
		}
	}
	if s.SearchActionType != "rss" {
		if strings.ToLower(s.SearchGroupType) == "movie" {
			if len(nzb.IMDBID) >= 1 {
				//Check Correct Imdb
				tempimdb := strings.TrimPrefix(nzb.IMDBID, "tt")
				tempimdb = strings.TrimLeft(tempimdb, "0")

				wantedimdb := strings.TrimPrefix(s.Imdb, "tt")
				wantedimdb = strings.TrimLeft(wantedimdb, "0")
				if wantedimdb != tempimdb && len(wantedimdb) >= 1 && len(tempimdb) >= 1 {
					denynzb("Imdb not match", entry, dl)
					return
				}
			}
		} else {
			//Check TVDB Id
			if strconv.Itoa(s.Tvdb) != nzb.TVDBID && s.Tvdb >= 1 && len(nzb.TVDBID) >= 1 {
				denynzb("Tvdbid not match", entry, dl)
				return
			}
		}
	}
	if strings.ToLower(s.SearchGroupType) == "movie" {
		//var loopdbmovie database.Dbmovie
		var dbmovieid, movieid uint
		if s.SearchActionType == "rss" {
			//Filter RSS Movies
			var denied bool
			denied, dbmovieid, movieid = getmovierss(&entry, addinlist, addifnotfound, s.ConfigTemplate, s.Quality, dl)
			if denied {
				return
			}
			entry.NzbmovieID = movieid
			entry.QualityTemplate, _ = database.QueryColumnString("Select quality_profile from movies where id = ?", movieid)
			entry.WantedTitle, _ = database.QueryColumnString("Select title from dbmovies where id = ?", dbmovieid)
		} else {
			dbmovieid = s.Movie.DbmovieID
			entry.NzbmovieID = s.Movie.ID
			entry.QualityTemplate = s.Movie.QualityProfile
			entry.WantedTitle, err = database.QueryColumnString("Select title from dbmovies where id = ?", s.Movie.DbmovieID)
			if err != nil {
				denynzb("Unwanted DB Movie", entry, dl)
				return
			}
		}

		//Check QualityProfile
		if !config.ConfigCheck("quality_" + entry.QualityTemplate) {
			denynzb("Unknown quality", entry, dl)
			return
		}

		//Check Minimal Priority
		entry.MinimumPriority = parser.GetHighestMoviePriorityByFiles(entry.NzbmovieID, s.ConfigTemplate, entry.QualityTemplate)

		var dont bool
		if entry.MinimumPriority == 0 {
			dont, _ = database.QueryColumnBool("Select dont_search from movies where id = ?", entry.NzbmovieID)
			if dont {
				denynzb("Search disabled", entry, dl)
				return
			}
		} else {
			//Check Upgradable
			dont, _ = database.QueryColumnBool("Select dont_upgrade from movies where id = ?", entry.NzbmovieID)
			if dont {
				denynzb("Upgrade disabled", entry, dl)
				return
			}
		}
		entry.WantedAlternates = database.QueryStaticStringArray("Select title from dbmovie_titles where dbmovie_id = ?", "Select count(id) from dbmovie_titles where dbmovie_id = ?", dbmovieid)
	} else {
		if s.SearchActionType == "rss" {
			//Filter RSS Series
			var dbserieid, episodeid uint
			var denied bool
			denied, dbserieid, episodeid = getserierss(&entry, s.ConfigTemplate, s.Quality, dl)
			if denied {
				return
			}
			entry.NzbepisodeID = episodeid
			entry.QualityTemplate, _ = database.QueryColumnString("Select quality_profile from serie_episodes where id = ?", episodeid)
			entry.WantedTitle, err = database.QueryColumnString("Select seriename from dbseries where id = ?", dbserieid)
			//s.SerieEpisode, _ = database.GetSerieEpisodes(database.Query{Where: "id = ?", WhereArgs: []interface{}{episodeid}})
		} else {
			entry.NzbepisodeID = s.SerieEpisode.ID
			entry.QualityTemplate = s.SerieEpisode.QualityProfile
			entry.WantedTitle, err = database.QueryColumnString("Select seriename from dbseries where id = ?", s.SerieEpisode.DbserieID)
		}

		//Check Quality Profile
		if !config.ConfigCheck("quality_" + entry.QualityTemplate) {
			denynzb("Unknown Quality Profile", entry, dl)
			return
		}

		//Check Minimum Priority
		entry.MinimumPriority = parser.GetHighestEpisodePriorityByFiles(entry.NzbepisodeID, s.ConfigTemplate, entry.QualityTemplate)

		var dont bool
		if entry.MinimumPriority == 0 {
			//Check Searchable
			dont, _ = database.QueryColumnBool("Select dont_search from serie_episodes where id = ?", entry.NzbepisodeID)
			if dont {
				denynzb("Search disabled", entry, dl)
				return
			}
		} else {
			//Check Upgradable
			dont, _ = database.QueryColumnBool("Select dont_upgrade from serie_episodes where id = ?", entry.NzbepisodeID)
			if dont {
				denynzb("Upgrade disabled", entry, dl)
				return
			}
		}
		entry.WantedAlternates = database.QueryStaticStringArray("Select title from dbserie_alternates where dbserie_id = ?", "Select count(id) from dbserie_alternates where dbserie_id = ?", s.SerieEpisode.DbserieID)
	}
	//Regex
	regexdeny, regexrule := entry.Filter_regex_nzbs(index.Template_regex, nzb.Title)
	if regexdeny {
		denynzb("Denied by Regex: "+regexrule, entry, dl)
		return
	}

	//Parse
	if entry.ParseInfo.Title == "" || entry.ParseInfo.Resolution == "" || entry.ParseInfo.Quality == "" {
		if s.SearchGroupType == "series" {
			entry.ParseInfo, err = parser.NewFileParser(nzb.Title, true, "series")
		} else {
			entry.ParseInfo, err = parser.NewFileParser(nzb.Title, false, "movie")
		}
		if err != nil {
			denynzb("Error parsing", entry, dl)
			return
		}
	}
	if entry.ParseInfo.Priority == 0 {
		entry.ParseInfo.GetPriority(s.ConfigTemplate, entry.QualityTemplate)
		entry.Prio = entry.ParseInfo.Priority
	}

	parser.StripTitlePrefixPostfix(&entry.ParseInfo, entry.QualityTemplate)
	//Parse end

	qualityconfig := config.ConfigGet("quality_" + entry.QualityTemplate).Data.(config.QualityConfig)
	//Year
	if strings.ToLower(s.SearchGroupType) == "movie" && s.SearchActionType != "rss" {
		yearstr := strconv.Itoa(s.Year)
		if yearstr == "0" {
			denynzb("No Year", entry, dl)
			return
		}
		if qualityconfig.CheckYear && !qualityconfig.CheckYear1 && !strings.Contains(nzb.Title, yearstr) && len(yearstr) >= 1 && yearstr != "0" {
			denynzb("Unwanted Year", entry, dl)
			return
		} else {
			if qualityconfig.CheckYear1 && len(yearstr) >= 1 && yearstr != "0" {
				yearint, _ := strconv.Atoi(yearstr)
				if !strings.Contains(nzb.Title, strconv.Itoa(yearint+1)) && !strings.Contains(nzb.Title, strconv.Itoa(yearint-1)) && !strings.Contains(nzb.Title, strconv.Itoa(yearint)) {
					denynzb("Unwanted Year1", entry, dl)
					return
				}
			}
		}
	}

	//Checktitle
	if strings.ToLower(s.SearchGroupType) == "movie" {
		if qualityconfig.CheckTitle {
			titlefound := false
			if qualityconfig.CheckTitle && parser.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title) && len(entry.WantedTitle) >= 1 {
				titlefound = true
			}
			if !titlefound {
				alttitlefound := false
				for idxtitle := range entry.WantedAlternates {
					if entry.WantedAlternates[idxtitle] == "" {
						continue
					}
					if parser.Checknzbtitle(entry.WantedAlternates[idxtitle], entry.ParseInfo.Title) {
						alttitlefound = true
						break
					}
				}
				if len(entry.WantedAlternates) >= 1 && !alttitlefound {
					denynzb("Unwanted Title and Alternate", entry, dl)
					return
				}
			}
			if len(entry.WantedAlternates) == 0 && !titlefound {
				denynzb("Unwanted Title", entry, dl)
				return
			}
		}
	} else {
		if qualityconfig.CheckTitle {
			toskip := true
			if entry.WantedTitle != "" {
				if qualityconfig.CheckTitle && parser.Checknzbtitle(entry.WantedTitle, entry.ParseInfo.Title) && len(entry.WantedTitle) >= 1 {
					toskip = false
				}
				if toskip {
					for idxtitle := range entry.WantedAlternates {
						if entry.WantedAlternates[idxtitle] == "" {
							continue
						}
						if parser.Checknzbtitle(entry.WantedAlternates[idxtitle], entry.ParseInfo.Title) {
							toskip = false
							break
						}
					}
				}
				if toskip {
					denynzb("Serie name not found", entry, dl)
					return
				}
			} else {
				denynzb("Serie name not provided", entry, dl)
				return
			}
		}
	}

	//Checkepisode
	if strings.ToLower(s.SearchGroupType) != "movie" {
		foundepi, err := database.QueryColumnUint("Select dbserie_episode_id from serie_episodes where id = ?", entry.NzbepisodeID)
		if err == nil {
			identifier, err := database.QueryColumnString("Select identifier from dbserie_episodes where id = ?", foundepi)
			if err == nil {
				// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
				matchfound := false

				alt_identifier := strings.TrimLeft(identifier, "S0")
				alt_identifier = strings.Replace(alt_identifier, "E", "x", -1)
				tolower := strings.ToLower(nzb.Title)
				if strings.Contains(tolower, strings.ToLower(identifier)) ||
					strings.Contains(tolower, strings.ToLower(strings.Replace(identifier, "-", ".", -1))) ||
					strings.Contains(tolower, strings.ToLower(strings.Replace(identifier, "-", " ", -1))) ||
					strings.Contains(tolower, strings.ToLower(alt_identifier)) ||
					strings.Contains(tolower, strings.ToLower(strings.Replace(alt_identifier, "-", ".", -1))) ||
					strings.Contains(tolower, strings.ToLower(strings.Replace(alt_identifier, "-", " ", -1))) {

					matchfound = true
				} else {
					season, _ := database.QueryColumnString("Select season from dbserie_episodes where id = ?", foundepi)
					episode, _ := database.QueryColumnString("Select episode from dbserie_episodes where id = ?", foundepi)

					identifierlower := strings.ToLower(entry.ParseInfo.Identifier)
					for _, seasonvar := range []string{"s" + season + "e", "s0" + season + "e", "s" + season + " e", "s0" + season + " e", season + "x", season + " x"} {
						if strings.HasPrefix(identifierlower, seasonvar) {
							for _, episodevar := range []string{"e" + episode, "e0" + episode, "x" + episode, "x0" + episode} {
								if strings.HasSuffix(identifierlower, episodevar) {
									matchfound = true
									break
								}
								if strings.Contains(identifierlower, episodevar+" ") {
									matchfound = true
									break
								}
								if strings.Contains(identifierlower, episodevar+"-") {
									matchfound = true
									break
								}
								if strings.Contains(identifierlower, episodevar+"e") {
									matchfound = true
									break
								}
								if strings.Contains(identifierlower, episodevar+"x") {
									matchfound = true
									break
								}
							}
							break
						}
					}
				}
				if !matchfound {
					denynzb("seriename provided dbepi found but identifier not match "+identifier, entry, dl)
					logger.ClearVar(&foundepi)
					return
				}
			} else {
				denynzb("seriename provided dbepi not found "+entry.WantedTitle, entry, dl)
				logger.ClearVar(&foundepi)
				return
			}
		} else {
			denynzb("seriename provided epi not found "+entry.WantedTitle, entry, dl)
			return
		}
	}

	//check quality
	if !Filter_test_quality_wanted(entry.ParseInfo, entry.QualityTemplate, nzb.Title) {
		denynzb("Unwanted Quality", entry, dl)
		return
	}

	//check priority
	if entry.ParseInfo.Priority != 0 {
		if entry.MinimumPriority != 0 {
			if entry.ParseInfo.Priority <= entry.MinimumPriority {
				denynzb("Prio lower. have: "+strconv.Itoa(entry.MinimumPriority), entry, dl)
				return
			}
			logger.Log.Debug("ok - prio higher: ", nzb.Title, " old prio ", entry.MinimumPriority, " found prio ", entry.ParseInfo.Priority)
		}
	} else {
		denynzb("Prio unknown", entry, dl)
		return
	}
	dl.Nzbs = append(dl.Nzbs, entry)
	//nzbs_out = nil
	return
}

func Filter_test_quality_wanted(m parser.ParseInfo, qualityTemplate string, title string) bool {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	wanted_release_resolution := false
	for idxqual := range qualityconfig.Wanted_resolution {
		if strings.EqualFold(qualityconfig.Wanted_resolution[idxqual], m.Resolution) {
			wanted_release_resolution = true
			break
		}
	}

	if len(qualityconfig.Wanted_resolution) >= 1 && !wanted_release_resolution {
		logger.Log.Debug("Skipped - unwanted resolution: ", title, " ", qualityTemplate, " ", m.Resolution)
		return false
	}
	wanted_release_quality := false
	for idxqual := range qualityconfig.Wanted_quality {
		if strings.EqualFold(qualityconfig.Wanted_quality[idxqual], m.Quality) {
			wanted_release_quality = true
			break
		}
	}
	if len(qualityconfig.Wanted_quality) >= 1 && !wanted_release_quality {
		logger.Log.Debug("Skipped - unwanted quality: ", title, " ", qualityTemplate, " ", m.Quality)
		return false
	}
	wanted_release_audio := false
	for idxqual := range qualityconfig.Wanted_audio {
		if strings.EqualFold(qualityconfig.Wanted_audio[idxqual], m.Audio) {
			wanted_release_audio = true
			break
		}
	}
	if len(qualityconfig.Wanted_audio) >= 1 && !wanted_release_audio {
		logger.Log.Debug("Skipped - unwanted audio: ", title, " ", qualityTemplate)
		return false
	}
	wanted_release_codec := false
	for idxqual := range qualityconfig.Wanted_codec {
		if strings.EqualFold(qualityconfig.Wanted_codec[idxqual], m.Codec) {
			wanted_release_codec = true
			break
		}
	}
	if len(qualityconfig.Wanted_codec) >= 1 && !wanted_release_codec {
		logger.Log.Debug("Skipped - unwanted codec: ", title, " ", qualityTemplate)
		return false
	}
	return true
}
func getSubImdb(imdb string) (uint, error) {
	searchimdb := "tt" + imdb
	founddbmovie, founddbmovieerr := database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
	if founddbmovieerr != nil {
		searchimdb := "tt0" + imdb
		founddbmovie, founddbmovieerr = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
		if founddbmovieerr != nil {
			searchimdb = "tt00" + imdb
			founddbmovie, founddbmovieerr = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
			if founddbmovieerr != nil {
				searchimdb = "tt000" + imdb
				founddbmovie, founddbmovieerr = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
				if founddbmovieerr != nil {
					searchimdb = "tt0000" + imdb
					founddbmovie, founddbmovieerr = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
				}
			}
		}
	}
	return founddbmovie, founddbmovieerr
}

func insertmovie(imdb string, configTemplate string, addinlist string) (uint, error) {
	importfeed.JobImportMovies(imdb, configTemplate, addinlist)
	founddbmovie, founddbmovieerr := database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", imdb)
	return founddbmovie, founddbmovieerr
}

func getmovierss(entry *parser.Nzbwithprio, addinlist string, addifnotfound bool, configTemplate string, qualityTemplate string, dl *SearchResults) (bool, uint, uint) {
	var loopdbmovie uint
	var loopmovie uint

	//Parse
	var err error
	entry.ParseInfo, err = parser.NewFileParser(entry.NZB.Title, false, "movie")
	if err != nil {
		denynzb("Error parsing", *entry, dl)
		entry = nil
		return true, 0, 0
	}

	//Get DbMovie by imdbid
	if entry.NZB.IMDBID != "" {
		searchimdb := entry.NZB.IMDBID
		if !strings.HasPrefix(searchimdb, "tt") {
			searchimdb = "tt" + entry.NZB.IMDBID
		}
		loopdbmovie, err = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", searchimdb)
		if loopdbmovie == 0 {
			if !strings.HasPrefix(entry.NZB.IMDBID, "tt") && err != nil {
				loopdbmovie, err = getSubImdb(entry.NZB.IMDBID)
			}
		}
	}
	//Get DbMovie by Title if not found yet
	if loopdbmovie == 0 {
		loopdbmovie, _, _ = importfeed.MovieFindDbIdByTitle(entry.ParseInfo.Title, strconv.Itoa(entry.ParseInfo.Year), "rss", addifnotfound)
	}
	//Add DbMovie if not found yet and enabled
	if loopdbmovie == 0 {
		if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") {
			if !allowMovieImport(entry.NZB.IMDBID, addinlist) {
				denynzb("Not Allowed Movie", *entry, dl)
				entry = nil
				return true, 0, 0
			}
			var err2 error
			loopdbmovie, err2 = insertmovie(entry.NZB.IMDBID, configTemplate, addinlist)
			loopmovie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", loopdbmovie, addinlist)
			if err != nil || err2 != nil {
				denynzb("Not Wanted Movie", *entry, dl)
				entry = nil
				return true, 0, 0
			}
		} else {
			denynzb("Not Wanted DBMovie", *entry, dl)
			entry = nil
			return true, 0, 0
		}
	}

	//continue only if dbmovie found
	if loopdbmovie != 0 {

		//Get List of movie by dbmovieid, year and possible lists
		parser.StripTitlePrefixPostfix(&entry.ParseInfo, qualityTemplate)
		list, imdb := importfeed.MovieGetListFilter(configTemplate, int(loopdbmovie), entry.ParseInfo.Year)
		if list != "" {
			entry.NZB.IMDBID = imdb
			loopmovie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", loopdbmovie, list)
			if err != nil {
				denynzb("Not Wanted Movie", *entry, dl)
				entry = nil
				return true, 0, 0
			}
		} else {
			//if list was not found : should we add the movie?
			if addifnotfound && strings.HasPrefix(entry.NZB.IMDBID, "tt") {
				if !allowMovieImport(entry.NZB.IMDBID, addinlist) {
					denynzb("Not Allowed Movie", *entry, dl)
					entry = nil
					return true, 0, 0
				}
				var err2 error
				loopdbmovie, err2 = insertmovie(entry.NZB.IMDBID, configTemplate, addinlist)
				loopmovie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", loopdbmovie, addinlist)
				if err != nil || err2 != nil {
					denynzb("Not Wanted Movie", *entry, dl)
					entry = nil
					return true, 0, 0
				}
			} else {
				denynzb("Not Wanted Movie", *entry, dl)
				entry = nil
				return true, 0, 0
			}
		}
	}
	return false, loopdbmovie, loopmovie
}

func denynzb(reason string, entry parser.Nzbwithprio, dl *SearchResults) {
	logger.Log.Debug("Skipped - ", reason, ": ", entry.NZB.Title)
	entry.Denied = true
	entry.Reason = reason
	dl.Rejected = append(dl.Rejected, entry)
}
func getserierss(entry *parser.Nzbwithprio, configTemplate string, qualityTemplate string, dl *SearchResults) (bool, uint, uint) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var loopdbseries uint
	var foundserie uint
	var foundserieid uint
	var err error
	var parsed bool
	if len(entry.NZB.TVDBID) >= 1 {
		loopdbseries, err = database.QueryColumnUint("Select id from dbseries where thetvdb_id = ?", entry.NZB.TVDBID)

		if err != nil {
			denynzb("Unwanted DB Series", *entry, dl)
			entry = nil
			return true, 0, 0
		}
	}
	if loopdbseries == 0 {
		entry.ParseInfo, err = parser.NewFileParser(entry.NZB.Title, true, "series")
		if err != nil {
			denynzb("Error parsing", *entry, dl)
			entry = nil
			return true, 0, 0
		}
		parsed = true
		titleyear := entry.ParseInfo.Title
		if entry.ParseInfo.Year != 0 {
			titleyear += " (" + strconv.Itoa(entry.ParseInfo.Year) + ")"
		}
		loopdbseries, err = importfeed.FindDbserieByName(titleyear)
		if loopdbseries == 0 {
			seriestitle := ""
			matched := config.RegexGet("RegexSeriesTitle").FindStringSubmatch(entry.NZB.Title)
			defer logger.ClearVar(&matched)
			if len(matched) >= 2 {
				seriestitle = matched[1]
			}
			loopdbseries, err = importfeed.FindDbserieByName(seriestitle)
			if loopdbseries == 0 {
				loopdbseries, err = importfeed.FindDbserieByName(entry.ParseInfo.Title)
			}
		}
	}

	if loopdbseries != 0 {
		for idxlist := range configEntry.Lists {
			foundserieid, err = database.QueryColumnUint("Select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE", loopdbseries, configEntry.Lists[idxlist].Name)
			if err == nil && foundserieid != 0 {
				break
			}
		}
		if foundserieid == 0 {
			denynzb("Unwanted Serie", *entry, dl)
			entry = nil
			return true, 0, 0
		}
		foundserie = foundserieid
		identifiedby, err := database.QueryColumnString("Select identifiedby from dbseries where id = ?", loopdbseries)
		if strings.EqualFold(identifiedby, "date") && !parsed {
			entry.ParseInfo, err = parser.NewFileParser(entry.NZB.Title, true, "series")
			if err != nil {
				denynzb("Error parsing", *entry, dl)
				entry = nil
				return true, 0, 0
			}
		}
	}
	var founddbepisode uint
	identifiedby, err := database.QueryColumnString("Select identifiedby from dbseries where id = ?", loopdbseries)
	if strings.EqualFold(identifiedby, "date") {
		if entry.ParseInfo.Date == "" {
			denynzb("Unwanted Date not found", *entry, dl)
			return true, 0, 0
		}
		entry.ParseInfo.Date = strings.Replace(entry.ParseInfo.Date, ".", "-", -1)
		entry.ParseInfo.Date = strings.Replace(entry.ParseInfo.Date, " ", "-", -1)

		founddbepisode, err = database.QueryColumnUint("Select id from dbserie_episodes where dbserie_id = ? and Identifier = ? COLLATE NOCASE", loopdbseries, entry.ParseInfo.Date)

		if err != nil {
			denynzb("Unwanted DB Episode", *entry, dl)
			entry = nil
			return true, 0, 0
		}
	} else {
		founddbepisode, err = database.QueryColumnUint("Select id from dbserie_episodes where dbserie_id = ? and Season = ? and Episode = ?", loopdbseries, strings.TrimPrefix(strings.TrimPrefix(entry.NZB.Season, "S"), "0"), strings.TrimPrefix(strings.TrimPrefix(entry.NZB.Episode, "E"), "0"))
		if err != nil {
			denynzb("Unwanted DB Episode", *entry, dl)
			entry = nil
			return true, 0, 0
		}
	}
	foundepisodeid, err := database.QueryColumnUint("Select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", founddbepisode, foundserie)
	if err != nil {
		denynzb("Unwanted Episode", *entry, dl)
		entry = nil
		return true, 0, 0
	}
	//cfg_quality := config.ConfigGet("quality_" + foundepisode.QualityProfile).Data.(config.QualityConfig)
	//if cfg_quality.BackupSearchForTitle {
	entry.NzbepisodeID = foundepisodeid
	entry.WantedAlternates = database.QueryStaticStringArray("Select title from dbserie_alternates where dbserie_id = ?", "Select count(id) from dbserie_alternates where dbserie_id = ?", loopdbseries)
	//}
	return false, loopdbseries, foundepisodeid
}

func (s *Searcher) GetRSSFeed(searchGroupType string, listConfig string) (SearchResults, error) {
	list := config.ConfigGetMediaListConfig(s.ConfigTemplate, listConfig)
	if list.Name == "" {
		return SearchResults{}, errors.New("list not found")
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return SearchResults{}, errors.New("list not found")
	}
	if !config.ConfigCheck("quality_" + s.Quality) {
		logger.Log.Error("Quality for RSS not found")
		return SearchResults{}, errors.New("quality not found")
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
		return SearchResults{}, errors.New("regex not found")
	}

	lastindexerid, _ := database.QueryColumnString("Select last_id from r_sshistories where config = ? and list = ? and indexer = ?", "list_"+list.Template_list, s.Quality, "")

	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)
	indexer := apiexternal.NzbIndexer{Name: "list_" + list.Template_list, Customrssurl: cfg_list.Url, LastRssId: lastindexerid}

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer = ? and last_fail > ?", cfg_list.Url, sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", "list_"+list.Template_list)
		return SearchResults{}, errors.New("indexer disabled")
	}
	nzbs, failed, lastids, nzberr := apiexternal.QueryNewznabRSSLast(indexer, cfg_list.Limit, "", 1)
	defer logger.ClearVar(&nzbs)

	if nzberr != nil {
		logger.Log.Error("Newznab RSS Search failed")
		failedindexer(failed)
	} else {
		if lastids != "" && len((nzbs)) >= 1 {
			addrsshistory(indexer.URL, lastids, s.Quality, "list_"+list.Template_list)
		}
		var dl SearchResults
		if !config.ConfigCheck("regex_" + cfg_indexer.Template_regex) {
			for idx := range nzbs {
				dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
					NZB:     nzbs[idx],
					Indexer: cfg_indexer.Template_indexer,
					Denied:  true,
					Reason:  "Denied by Regex"})
			}
		} else {
			for idx := range nzbs {
				s.convertnzbs(s.Quality, list.Template_list, nzbs[idx], listConfig, list.Addfound, &dl)
			}
		}

		if len(dl.Nzbs) > 1 {
			sort.Slice(dl.Nzbs, func(i, j int) bool {
				return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
			})
		}
		if len(dl.Nzbs) == 0 {
			logger.Log.Info("No new entries found")
		}
		return dl, nil
	}
	return SearchResults{}, errors.New("other error")
}

func addrsshistory(url string, lastid string, quality string, config string) {
	database.UpsertArray("r_sshistories", []string{"indexer", "last_id", "list", "config"}, []interface{}{url, lastid, quality, config}, database.Query{Where: "config = ? and list = ? and indexer = ?", WhereArgs: []interface{}{config, quality, url}})
}

func (s *Searcher) MovieSearch(movieid uint, forceDownload bool, titlesearch bool) (SearchResults, error) {
	s.SearchGroupType = "movie"
	s.Movie, _ = database.GetMovies(database.Query{Where: "id = ?", WhereArgs: []interface{}{movieid}})
	if s.Movie.DontSearch && !forceDownload {
		logger.Log.Debug("Skipped - Search disabled")
		return SearchResults{}, errors.New("search disabled")
	}

	imdb, err := database.QueryColumnString("Select imdb_id from dbmovies where id = ?", s.Movie.DbmovieID)
	if err != nil {
		return SearchResults{}, errors.New("imdb not found")
	}

	s.Imdb = imdb

	year, err := database.QueryColumnUint("Select year from dbmovies where id = ?", s.Movie.DbmovieID)
	if err != nil {
		return SearchResults{}, errors.New("year not found")
	}
	s.Year = int(year)
	if s.Year == 0 {
		logger.Log.Debug("Skipped - No Year")
		return SearchResults{}, errors.New("year not found")
	}

	title, err := database.QueryColumnString("Select title from dbmovies where id = ?", s.Movie.DbmovieID)
	if err != nil {
		return SearchResults{}, errors.New("title not found")
	}
	s.Name = title

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(s.Movie.ID)) + " not found")
		return SearchResults{}, errors.New("quality not found")
	}
	cfg_quality := config.ConfigGet("quality_" + s.Movie.QualityProfile).Data.(config.QualityConfig)
	s.Quality = s.Movie.QualityProfile
	s.MinimumPriority = parser.GetHighestMoviePriorityByFiles(s.Movie.ID, s.ConfigTemplate, s.Movie.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if s.Movie.DontUpgrade && !forceDownload {
			logger.Log.Debug("Skipped - Upgrade disabled: ", s.Name)
			return SearchResults{}, errors.New("upgrade disabled")
		}
	}

	var dl SearchResults
	// dl.Nzbs = make([]parser.Nzbwithprio, 0, 10)
	// dl.Rejected = make([]parser.Nzbwithprio, 0, 100)

	processedindexer := 0
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerIndexer))
	for idx := range cfg_quality.Indexer {
		index := cfg_quality.Indexer[idx]
		swi.Add(func() {
			ok := s.moviesearchindexer(cfg_quality.Name, index.Template_indexer, titlesearch, &dl)
			if ok {
				processedindexer += 1
			}
		})
	}
	swi.Wait()
	swi.Close()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}

	if processedindexer >= 1 {
		database.UpdateColumn("movies", "lastscan", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{s.Movie.ID}})
	}

	return dl, nil
}

func (t *Searcher) moviesearchindexer(quality string, indexer string, titlesearch bool, dl *SearchResults) bool {
	logger.Log.Info("Indexer search for Movie: "+strconv.Itoa(int(t.Movie.ID))+" indexer: ", indexer)
	indexerurl, cats, erri := t.initIndexerUrlCat(quality, indexer, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", indexer, " ", erri)

		return false
	}
	defer logger.ClearVar(&cats)

	// dl.Nzbs = make([]parser.Nzbwithprio, 0, 10)
	// dl.Rejected = make([]parser.Nzbwithprio, 0, 100)
	releasefound := false
	if !config.ConfigCheck("quality_" + t.Movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(t.Movie.ID)) + " not found")
		return false
	}
	cfg_quality := config.ConfigGet("quality_" + t.Movie.QualityProfile).Data.(config.QualityConfig)
	if t.Imdb != "" {
		logger.Log.Info("Search movie by imdb ", t.Imdb, " with indexer ", indexer)
		t.moviesSearchImdb(quality, indexer, cats, dl)

		if len(dl.Nzbs) >= 1 {
			logger.Log.Debug("Indexer loop - entries found: ", len(dl.Nzbs))
			releasefound = true

			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
				return true
			}
		}
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
		if checkindexerblock(cfg_general.FailedIndexerBlockTime, indexerurl) {
			return false
		}
		logger.Log.Info("Search movie by title ", t.Name, " with indexer ", indexer)
		t.moviesSearchTitle(quality, indexer, t.Name, cats, dl)
		if len(dl.Nzbs) >= 1 {
			logger.Log.Debug("Indexer loop - entries found: ", len(dl.Nzbs))
			releasefound = true

			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
				return true
			}
		}
	}
	if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
		for _, altitle := range database.QueryStaticStringArray("Select distinct title from dbmovie_titles where dbmovie_id = ? and title != ?", "Select count(distinct title) from dbmovie_titles where dbmovie_id = ? and title != ?", t.Movie.DbmovieID, t.Name) {
			if altitle == "" {
				continue
			}
			if checkindexerblock(cfg_general.FailedIndexerBlockTime, indexerurl) {
				continue
			}
			logger.Log.Info("Search movie by title ", altitle, " with indexer ", indexer)
			t.moviesSearchTitle(quality, indexer, altitle, cats, dl)

			if len(dl.Nzbs) >= 1 {
				logger.Log.Debug("Indexer loop - entries found: ", len(dl.Nzbs))
				releasefound = true

				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
					break
				}
			}
		}
		if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
			logger.Log.Debug("Break Indexer loop - entry found: ", t.Name)
			return true
		}
	}
	return true
}

func (s *Searcher) SeriesSearch(episodeid uint, forceDownload bool, titlesearch bool) (SearchResults, error) {
	var err error
	s.SerieEpisode, err = database.GetSerieEpisodes(database.Query{Where: "id = ?", WhereArgs: []interface{}{episodeid}})
	s.SearchGroupType = "series"
	if s.SerieEpisode.DontSearch && !forceDownload {
		logger.Log.Debug("Search not wanted: ")
		return SearchResults{}, errors.New("search disabled")
	}

	tvdb, err := database.QueryColumnUint("Select thetvdb_id from dbseries where id = ?", s.SerieEpisode.DbserieID)
	if err != nil {
		return SearchResults{}, errors.New("tvdb not found")
	}
	s.Tvdb = int(tvdb)

	title, err := database.QueryColumnString("Select seriename from dbseries where id = ?", s.SerieEpisode.DbserieID)
	if err != nil {
		return SearchResults{}, errors.New("seriename not found")
	}
	s.Name = title

	season, err := database.QueryColumnString("Select season from dbserie_episodes where id = ?", s.SerieEpisode.DbserieEpisodeID)
	if err != nil {
		return SearchResults{}, errors.New("season not found")
	}
	s.Season = season

	episode, err := database.QueryColumnString("Select episode from dbserie_episodes where id = ?", s.SerieEpisode.DbserieEpisodeID)
	if err != nil {
		return SearchResults{}, errors.New("episode not found")
	}
	s.Episode = episode

	identifier, err := database.QueryColumnString("Select identifier from dbserie_episodes where id = ?", s.SerieEpisode.DbserieEpisodeID)
	if err != nil {
		return SearchResults{}, errors.New("identifier not found")
	}
	s.Identifier = identifier

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(s.SerieEpisode.ID)) + " not found")
		return SearchResults{}, errors.New("quality not found")
	}
	cfg_quality := config.ConfigGet("quality_" + s.SerieEpisode.QualityProfile).Data.(config.QualityConfig)
	s.Quality = s.SerieEpisode.QualityProfile
	s.MinimumPriority = parser.GetHighestEpisodePriorityByFiles(s.SerieEpisode.ID, s.ConfigTemplate, s.SerieEpisode.QualityProfile)

	if s.MinimumPriority == 0 {
		s.SearchActionType = "missing"
	} else {
		s.SearchActionType = "upgrade"
		if s.SerieEpisode.DontUpgrade && !forceDownload {
			logger.Log.Debug("Upgrade not wanted: ", s.Name)
			return SearchResults{}, errors.New("upgrade not wanted")
		}
	}

	var dl SearchResults
	processedindexer := 0
	if cfg_general.WorkerIndexer == 0 {
		cfg_general.WorkerIndexer = 1
	}
	swi := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerIndexer))
	for idx := range cfg_quality.Indexer {
		index := cfg_quality.Indexer[idx]
		swi.Add(func() {
			ok := s.seriessearchindexer(s.Quality, index.Template_indexer, titlesearch, &dl)
			if ok {
				processedindexer += 1
			}
		})
	}
	swi.Wait()
	swi.Close()
	if len(dl.Nzbs) > 1 {
		sort.Slice(dl.Nzbs, func(i, j int) bool {
			return dl.Nzbs[i].Prio > dl.Nzbs[j].Prio
		})
	}

	if processedindexer >= 1 {
		database.UpdateColumn("serie_episodes", "lastscan", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{s.SerieEpisode.ID}})
	}

	return dl, nil
}

func (t *Searcher) seriessearchindexer(quality string, indexer string, titlesearch bool, dl *SearchResults) bool {
	indexerurl, cats, erri := t.initIndexerUrlCat(quality, indexer, "api")
	if erri != nil {
		logger.Log.Debug("Skipped Indexer: ", indexer, " ", erri)
		return false
	}
	defer logger.ClearVar(&cats)

	if !config.ConfigCheck("quality_" + t.SerieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(t.SerieEpisode.ID)) + " not found")
		return false
	}
	cfg_quality := config.ConfigGet("quality_" + t.SerieEpisode.QualityProfile).Data.(config.QualityConfig)

	releasefound := false
	// dl.Nzbs = make([]parser.Nzbwithprio, 0, 10)
	// dl.Rejected = make([]parser.Nzbwithprio, 0, 100)
	if t.Tvdb != 0 {
		logger.Log.Info("Search serie by tvdbid ", t.Tvdb, " with indexer ", indexer)
		t.seriesSearchTvdb(quality, indexer, cats, dl)
		if len(dl.Nzbs) >= 1 {
			releasefound = true

			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
				return true
			}
		}
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if !releasefound && cfg_quality.BackupSearchForTitle && titlesearch {
		if checkindexerblock(cfg_general.FailedIndexerBlockTime, indexerurl) {
			return false
		}
		logger.Log.Info("Search serie by title ", t.Name, " with indexer ", indexer)
		t.seriesSearchTitle(quality, indexer, t.Name, cats, dl)

		if len(dl.Nzbs) >= 1 {
			releasefound = true

			if cfg_quality.CheckUntilFirstFound {
				logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
				return true
			}
		}
	}
	if !releasefound && cfg_quality.BackupSearchForAlternateTitle && titlesearch {
		for _, alttitle := range database.QueryStaticStringArray("Select distinct title from dbserie_alternates where dbserie_id = ? and title != ?", "Select count(distict name) from dbserie_alternates where dbserie_id = ? and title != ?", t.SerieEpisode.DbserieID, t.Name) {
			if alttitle == "" {
				continue
			}
			if checkindexerblock(cfg_general.FailedIndexerBlockTime, indexerurl) {
				continue
			}
			logger.Log.Info("Search serie by title ", alttitle, " with indexer ", indexer)
			t.seriesSearchTitle(quality, indexer, alttitle, cats, dl)

			if len(dl.Nzbs) >= 1 {
				releasefound = true

				if cfg_quality.CheckUntilFirstFound {
					logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
					break
				}
			}
		}
		if len(dl.Nzbs) >= 1 && cfg_quality.CheckUntilFirstFound {
			logger.Log.Debug("Break Indexer loop - entry found: ", t.Name, " ", t.Identifier)
			return true
		}
	}
	return true
}

func checkindexerblock(FailedIndexerBlockTime int, url string) bool {
	blockinterval := -5
	if FailedIndexerBlockTime != 0 {
		blockinterval = -1 * FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer = ? and last_fail > ?", url, sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true})
	if counter >= 1 {
		return true
	} else {
		return false
	}
}

func (s *Searcher) initIndexer(quality string, indexer string, rssapi string) (config.IndexersConfig, apiexternal.NzbIndexer, string, error) {
	logger.Log.Debug("Indexer search: ", indexer)

	if !config.ConfigCheck("indexer_" + indexer) {
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, "", errors.New("indexer config missing")
	}
	cfg_indexer := config.ConfigGet("indexer_" + indexer).Data.(config.IndexersConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !(strings.ToLower(cfg_indexer.IndexerType) == "newznab") {
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, "", errors.New("indexer Type Wrong")
	}
	if !cfg_indexer.Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, "", errors.New("indexer disabled")
	} else if !cfg_indexer.Enabled {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, "", errors.New("indexer disabled")
	}

	userid, _ := strconv.Atoi(cfg_indexer.Userid)

	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer = ? and last_fail > ?", cfg_indexer.Url, sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", cfg_indexer.Name)
		return config.IndexersConfig{}, apiexternal.NzbIndexer{}, "", errors.New("indexer disabled")
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString("Select last_id from r_sshistories where config = ? and list = ? and indexer = ?", s.ConfigTemplate, s.Quality, cfg_indexer.Url)
	}

	confindexer := config.QualityIndexerByQualityAndTemplate(quality, indexer)
	nzbindexer := apiexternal.NzbIndexer{
		URL:                     cfg_indexer.Url,
		Apikey:                  cfg_indexer.Apikey,
		UserID:                  userid,
		SkipSslCheck:            cfg_indexer.DisableTLSVerify,
		Addquotesfortitlequery:  cfg_indexer.Addquotesfortitlequery,
		Additional_query_params: confindexer.Additional_query_params,
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
	return cfg_indexer, nzbindexer, confindexer.Categories_indexer, nil
}

func (s *Searcher) initIndexerUrlCat(quality string, indexer string, rssapi string) (string, string, error) {
	logger.Log.Debug("Indexer search: ", indexer)

	if !config.ConfigCheck("indexer_" + indexer) {
		return "", "", errors.New("indexer config missing")
	}
	cfg_indexer := config.ConfigGet("indexer_" + indexer).Data.(config.IndexersConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !(strings.ToLower(cfg_indexer.IndexerType) == "newznab") {
		return "", "", errors.New("indexer Type Wrong")
	}
	if !cfg_indexer.Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return "", "", errors.New("indexer disabled")
	} else if !cfg_indexer.Enabled {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return "", "", errors.New("indexer disabled")
	}

	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer = ? and last_fail > ?", cfg_indexer.Url, sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", cfg_indexer.Name)
		return "", "", errors.New("indexer disabled")
	}

	return cfg_indexer.Url, config.QualityIndexerByQualityAndTemplate(quality, indexer).Categories_indexer, nil
}

func (s *Searcher) initNzbIndexer(quality string, indexer string, rssapi string) (apiexternal.NzbIndexer, error) {
	logger.Log.Debug("Indexer search: ", indexer)

	if !config.ConfigCheck("indexer_" + indexer) {
		return apiexternal.NzbIndexer{}, errors.New("template not found")
	}

	qualindexer := config.QualityIndexerByQualityAndTemplate(quality, indexer)
	cfg_indexer := config.ConfigGet("indexer_" + indexer).Data.(config.IndexersConfig)

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !(strings.ToLower(cfg_indexer.IndexerType) == "newznab") {
		return apiexternal.NzbIndexer{}, errors.New("indexer type wrong")
	}
	if !cfg_indexer.Rssenabled && strings.ToLower(rssapi) == "rss" {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return apiexternal.NzbIndexer{}, errors.New("indexer disabled")
	} else if !cfg_indexer.Enabled {
		logger.Log.Debug("Indexer disabled: ", cfg_indexer.Name)
		return apiexternal.NzbIndexer{}, errors.New("indexer disabled")
	}

	userid, _ := strconv.Atoi(cfg_indexer.Userid)

	blockinterval := -5
	if cfg_general.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * cfg_general.FailedIndexerBlockTime
	}
	counter, _ := database.CountRowsStatic("Select count(id) from indexer_fails where indexer = ? and last_fail > ?", cfg_indexer.Url, sql.NullTime{Time: time.Now().Add(time.Minute * time.Duration(blockinterval)), Valid: true})
	if counter >= 1 {
		logger.Log.Debug("Indexer temporarily disabled due to fail in the last ", blockinterval, " Minute(s): ", cfg_indexer.Name)
		return apiexternal.NzbIndexer{}, errors.New("indexer disabled")
	}

	var lastindexerid string
	if s.SearchActionType == "rss" {
		lastindexerid, _ = database.QueryColumnString("Select last_id from r_sshistories where config = ? and list = ? and indexer = ?", s.ConfigTemplate, s.Quality, cfg_indexer.Url)
	}

	nzbindexer := apiexternal.NzbIndexer{
		URL:                     cfg_indexer.Url,
		Apikey:                  cfg_indexer.Apikey,
		UserID:                  userid,
		SkipSslCheck:            cfg_indexer.DisableTLSVerify,
		Addquotesfortitlequery:  cfg_indexer.Addquotesfortitlequery,
		Additional_query_params: qualindexer.Additional_query_params,
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
	return nzbindexer, nil
}

func (s *Searcher) moviesSearchImdb(quality string, indexer string, cats string, dl *SearchResults) {
	logger.Log.Info("Search Movie by imdb: ", s.Imdb)
	nzbindexer, erri := s.initNzbIndexer(quality, indexer, "api")
	if erri != nil {
		return
	}
	defer logger.ClearVar(&nzbindexer)
	nzbs, failed, nzberr := apiexternal.QueryNewznabMovieImdb(nzbindexer, strings.Trim(s.Imdb, "t"), cats)
	defer logger.ClearVar(&nzbs)

	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Imdb, " with ", nzbindexer.URL, " error ", nzberr)
		failedindexer(failed)

	} else {
		if len((nzbs)) >= 1 {
			if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
				for idx := range nzbs {
					dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
						NZB:     nzbs[idx],
						Indexer: indexer,
						Denied:  true,
						Reason:  "Denied by Regex"})
				}
			} else {
				for idx := range nzbs {
					s.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
				}
			}
		}
	}

	return
}

func (s *Searcher) moviesSearchTitle(quality string, indexer string, title string, cats string, dl *SearchResults) {
	if len(title) == 0 {
		return
	}
	if !config.ConfigCheck("quality_" + s.Movie.QualityProfile) {
		logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(s.Movie.ID)) + " not found")
		return
	}
	cfg_quality := config.ConfigGet("quality_" + s.Movie.QualityProfile).Data.(config.QualityConfig)

	searchfor := title + " (" + strconv.Itoa(s.Year) + ")"
	if cfg_quality.ExcludeYearFromTitleSearch {
		searchfor = title
	}
	if searchfor != "" && cfg_quality.BackupSearchForTitle {

		logger.Log.Info("Search Movie by name: ", title)
		nzbindexer, erri := s.initNzbIndexer(quality, indexer, "api")
		if erri != nil {
			return
		}
		defer logger.ClearVar(&nzbindexer)
		nzbs, failed, nzberr := apiexternal.QueryNewznabQuery(nzbindexer, searchfor, cats, "movie")
		defer logger.ClearVar(&nzbs)

		if nzberr != nil {
			logger.Log.Error("Newznab Search failed: ", title, " with ", nzbindexer.URL, " error ", nzberr)
			failedindexer(failed)
		} else {
			if len((nzbs)) >= 1 {
				if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
					for idx := range nzbs {
						dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
							NZB:     nzbs[idx],
							Indexer: indexer,
							Denied:  true,
							Reason:  "Denied by Regex"})
					}
				} else {
					for idx := range nzbs {
						s.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
					}
				}
			}
		}
	}
	return
}

func failedindexer(failed string) {
	database.UpsertArray(
		"indexer_fails",
		[]string{"indexer", "last_fail"},
		[]interface{}{failed, time.Now()},
		database.Query{Where: "indexer = ?", WhereArgs: []interface{}{failed}})
}

func (s *Searcher) seriesSearchTvdb(quality string, indexer string, cats string, dl *SearchResults) {
	nzbindexer, erri := s.initNzbIndexer(quality, indexer, "api")
	if erri != nil {
		return
	}
	defer logger.ClearVar(&nzbindexer)
	logger.Log.Info("Search Series by tvdbid: ", s.Tvdb, " S", s.Season, "E", s.Episode)
	seasonint, err := strconv.Atoi(s.Season)
	if err != nil {
		return
	}
	episodeint, err := strconv.Atoi(s.Episode)
	if err != nil {
		return
	}
	nzbs, failed, nzberr := apiexternal.QueryNewznabTvTvdb(nzbindexer, s.Tvdb, cats, seasonint, episodeint, true, true)
	defer logger.ClearVar(&nzbs)
	if nzberr != nil {
		logger.Log.Error("Newznab Search failed: ", s.Tvdb, " with ", nzbindexer.URL, " error ", nzberr)
		failedindexer(failed)
	} else {
		if len((nzbs)) >= 1 {
			if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
				for idx := range nzbs {
					dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
						NZB:     nzbs[idx],
						Indexer: indexer,
						Denied:  true,
						Reason:  "Denied by Regex"})
				}
			} else {
				for idx := range nzbs {
					s.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
				}
			}
		}
	}

	return
}

func (s *Searcher) seriesSearchTitle(quality string, indexer string, title string, cats string, dl *SearchResults) {
	if !config.ConfigCheck("quality_" + s.SerieEpisode.QualityProfile) {
		logger.Log.Error("Quality for Episode: " + strconv.Itoa(int(s.SerieEpisode.ID)) + " not found")
		return
	}
	cfg_quality := config.ConfigGet("quality_" + s.SerieEpisode.QualityProfile).Data.(config.QualityConfig)

	if title != "" && s.Identifier != "" && cfg_quality.BackupSearchForTitle {
		logger.Log.Info("Search Series by title: ", title, " ", s.Identifier)
		searchfor := title + " " + s.Identifier
		nzbindexer, erri := s.initNzbIndexer(quality, indexer, "api")
		if erri != nil {
			return
		}
		defer logger.ClearVar(&nzbindexer)
		nzbs, failed, nzberr := apiexternal.QueryNewznabQuery(nzbindexer, searchfor, cats, "search")
		defer logger.ClearVar(&nzbs)

		if nzberr != nil {
			logger.Log.Error("Newznab Search failed: ", title, " with ", nzbindexer.URL, " error ", nzberr)
			failedindexer(failed)
		} else {
			if len((nzbs)) >= 1 {
				if !config.ConfigCheck("regex_" + config.QualityIndexerByQualityAndTemplate(quality, indexer).Template_regex) {
					for idx := range nzbs {
						dl.Rejected = append(dl.Rejected, parser.Nzbwithprio{
							NZB:     nzbs[idx],
							Indexer: indexer,
							Denied:  true,
							Reason:  "Denied by Regex"})
					}
				} else {
					for idx := range nzbs {
						s.convertnzbs(quality, indexer, nzbs[idx], "", false, dl)
					}
				}
			}
		}
	}
	return
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
		var countergenre int
		for idxgenre := range list.Excludegenre {
			countergenre, _ = database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Excludegenre[idxgenre])
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
		var countergenre int
		for idxgenre := range list.Includegenre {
			countergenre, _ = database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Includegenre[idxgenre])
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
