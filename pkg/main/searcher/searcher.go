package searcher

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/pool"
	"github.com/Kellerman81/go_media_downloader/worker"
)

// searchResults is a struct that contains slices of Nzbwithprio for Raw, Denied, and Accepted results
// As well as a boolean field bl
type searchResults struct {
	// Raw is a slice containing apiexternal.Nzbwithprio results
	Raw []apiexternal.Nzbwithprio
	// Denied is a slice containing denied apiexternal.Nzbwithprio results
	Denied []apiexternal.Nzbwithprio
	// Accepted is a slice containing accepted apiexternal.Nzbwithprio results
	Accepted []apiexternal.Nzbwithprio
	// bl is a boolean field
	bl bool
}

// ConfigSearcher is a struct containing configuration and search results
type ConfigSearcher struct {
	// Cfgp is a pointer to a MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// Quality is a pointer to a QualityConfig
	Quality *config.QualityConfig
	// searchActionType is a string indicating the search action type
	searchActionType string //missing,upgrade,rss
	// Sourceentry is a Nzbwithprio result
	Sourceentry apiexternal.Nzbwithprio
	// Dl contains the search results
	Dl searchResults
	// Mu is a mutex for synchronization
	Mu *sync.Mutex
}

const (
	skippedstr = "Skipped"
)

var (
	episodeprefixarray = []string{"", " ", "0", " 0"}
	//plsearcher         sync.Pool
	plparams = pool.NewPool(100, 0, func(b *searchparams) {}, func(b *searchparams) {
		b.sourcealttitles = nil
		*b = searchparams{}
	})
	plsearcher                pool.Poolobj[ConfigSearcher]
	strqualityunknown         = "unknown Quality"
	strpriounknown            = "unknown Prio"
	strpriosame               = "same Prio"
	strpriolower              = "lower Prio"
	strsearchdisabled         = "disabled Search"
	strupgradedisabled        = "disabled Upgrade"
	strmovieunwanted          = "unwanted Movie"
	strepisodeunwanted        = "unwanted Episode"
	stryearunwanted           = "unwanted Year"
	strtitleunwanted          = "unwanted Title"
	strtitlealternateunwanted = "unwanted Title and alternate"
	strdbmovieunwanted        = "unwanted DBMovie"
	strserieunwanted          = "unwanted Serie"
	strdbserieunwanted        = "unwanted DBSerie"
	strdbepisodeunwanted      = "unwanted DBEpisode"
	stridentifierunwanted     = "unwanted Identifier"
	strcodecunwanted          = "unwanted Codec"
	straudiounwanted          = "unwanted Audio"
	strqualityunwanted        = "unwanted Quality"
	strresolutionunwanted     = "unwanted Resolution"
	strseasonunwanted         = "unwanted Season"
	strtoosmall               = "too small"
	strtoobig                 = "too big"
	strnotmatchrequired       = "not matched required"
	strnotmatchimdb           = "not matched imdb"
	strnotmatchtvdb           = "not matched tvdb"
	strallreadydltitle        = "already downloaded title"
	strallreadydlurl          = "already downloaded url"
	strnoyear                 = "no year"
	strnotitle                = "no title"
	strnourl                  = "no url"
	strnosize                 = "no size"
	strnoidentifier           = "no identifier"
	strnoaddinlist            = "no addinlist"
	strshorttitle             = "short title"
	strunalloweddbmovie       = "unallowed DBMovie"
	deniedbyregex             = "Denied by Regex"
)

func DefineSearchPool(maxcount int) {
	plsearcher = pool.NewPool(config.SettingsGeneral.WorkerSearch+2, config.SettingsGeneral.WorkerSearch, func(cs *ConfigSearcher) {
		cs.Dl.Raw = make([]apiexternal.Nzbwithprio, 0, maxcount)
		cs.Mu = &sync.Mutex{}
	}, nil)
}

// SearchSerieRSSSeasonSingle searches for a single season of a series.
// It takes the series ID, season number, whether to search the full season or missing episodes,
// media type config, whether to auto close the results, and a pointer to search results.
// It returns a config searcher instance and error.
// It queries the database to map the series ID to thetvdb ID, gets the quality config,
// calls the search function, handles errors, downloads results,
// closes the results if autoclose is true, and returns the config searcher.
func SearchSerieRSSSeasonSingle(serieid uint, season string, useseason bool, cfgp *config.MediaTypeConfig, autoclose bool, results *ConfigSearcher) (*ConfigSearcher, error) {
	getid := database.GetdatarowN[uint](false, "select dbserie_id from series where id = ?", &serieid)
	tvdb := database.GetdatarowN[uint](false, "select thetvdb_id from dbseries where id = ?", &getid)
	if tvdb == 0 {
		return nil, logger.ErrTvdbEmpty
	}
	listid := database.GetMediaListIDGetListname(cfgp, serieid)
	if listid == -1 {
		return nil, logger.ErrListnameEmpty
	}

	err := results.searchSeriesRSSSeason(cfgp, cfgp.Lists[listid].CfgQuality, int(tvdb), season, useseason, false, false)
	if err != nil && !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamic("error", "Season Search Inc Failed", logger.NewLogFieldValue(err), logger.NewLogField("ID", serieid))

		if autoclose {
			results.Close()
		}
		return nil, err
	}
	results.Download()
	if autoclose {
		results.Close()
		return nil, nil
	}
	return results, nil
	//return SearchMyMedia(cfgpstr, qualstr, logger.StrRssSeasons, logger.StrSeries, int(tvdb), season, useseason, 0, false)
}

// SearchSeriesRSSSeasons searches the RSS feeds for missing episodes for
// random series. It selects up to 20 random series that have missing
// episodes, gets the distinct seasons with missing episodes for each,
// and searches the RSS feeds for those seasons.
func SearchSeriesRSSSeasons(cfgp *config.MediaTypeConfig) {
	if cfgp == nil {
		return
	}

	searchseasons(cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20"), 20, "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )")
}

// SearchSeriesRSSSeasonsAll searches all seasons for series matching the given
// media type config. It searches series that have missing episodes and calls
// searchseasons to perform the actual search.
func SearchSeriesRSSSeasonsAll(cfgp *config.MediaTypeConfig) {
	if cfgp == nil {
		return
	}

	searchseasons(cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1"), database.GetdatarowN[int](false, "select count() from series"), "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )")
}

// searchseasons searches for missing episodes for series matching the given
// configuration and quality settings. It selects a random sample of series
// to search, gets the distinct seasons with missing episodes for each, and
// searches those seasons on the RSS feeds of the enabled indexers. Results
// are added to the passed in DownloadResults instance.
func searchseasons(cfgp *config.MediaTypeConfig, queryrange string, queryrangecount int, queryseason string, queryseasoncount string) {
	if cfgp == nil {
		return
	}

	args := make([]any, len(cfgp.Lists))
	for i := range cfgp.Lists {
		args[i] = &cfgp.Lists[i].Name
	}
	tbl := database.GetrowsNuncached[database.DbstaticTwoInt](queryrangecount, queryrange, args)
	if len(tbl) == 0 {
		return
	}

	var getid, queryseasoncounter int

	var tbluint uint
	var listid int
	var arr []string
	for idx := range tbl {
		database.ScanrowsNdyn(false, queryseasoncount, &queryseasoncounter, &tbl[idx].Num2, &tbl[idx].Num1, &tbl[idx].Num1, &tbl[idx].Num2)
		if queryseasoncounter == 0 {
			continue
		}
		arr = database.GetrowsN[string](false, queryseasoncounter, queryseason, &tbl[idx].Num2, &tbl[idx].Num1, &tbl[idx].Num1, &tbl[idx].Num2)
		tbluint = uint(tbl[idx].Num1)
		for idx2 := range arr {
			listid = database.GetMediaListIDGetListname(cfgp, tbluint)
			if listid == -1 {
				continue
			}
			_ = database.ScanrowsNdyn(false, "select thetvdb_id from dbseries where id = ?", &getid, &tbl[idx].Num2)
			NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, &logger.V0).searchSeriesRSSSeason(cfgp, cfgp.Lists[listid].CfgQuality, getid, arr[idx2], true, true, true)
		}
		clear(arr)
	}
	clear(tbl)
}

// qualityIndexerByQualityAndTemplate returns the CategoriesIndexer string for the indexer
// in the given QualityConfig that matches the given IndexersConfig by name.
// Returns empty string if no match is found.
func qualityIndexerByQualityAndTemplate(quality *config.QualityConfig, ind *config.IndexersConfig) *config.QualityIndexerConfig {
	if ind == nil {
		return nil
	}
	for index := range quality.Indexer {
		if strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return &quality.Indexer[index]
		}
	}
	return nil
}

// getlistbyindexer returns the ListsConfig for the list matching the
// given IndexersConfig name. Returns nil if no match is found.
func getlistbyindexer(ind *config.IndexersConfig) *config.ListsConfig {
	for _, listcfg := range config.SettingsList {
		if strings.EqualFold(listcfg.Name, ind.Name) {
			return listcfg
		}
	}
	return nil
}

// SearchRSS searches the RSS feeds of the enabled Newznab indexers for the
// given media type and quality configuration. It returns a ConfigSearcher
// instance for managing the search, or an error if no search could be started.
// Results are added to the passed in DownloadResults instance.
func (searchvar *ConfigSearcher) SearchRSS(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, fetchall bool, downloadresults bool, autoclose bool) error {
	if autoclose {
		defer searchvar.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	if searchvar == nil {
		//searchvar = NewSearcher(cfgp, quality, logger.StrRss, nil)
	} else {
		searchvar.Dl.Clear()
		searchvar.Quality = quality
	}
	if searchvar == nil {
		return logger.ErrSearchvarEmpty
	}

	params := plparams.Get()
	if searchvar.searchindexers(true, params, runrsssearch) && len(searchvar.Dl.Raw) >= 1 {
		searchvar.searchparse(nil)
		if downloadresults {
			searchvar.Download()
		}
	}
	plparams.Put(params)
	return nil
}

// runrsssearch executes a search against the RSS feed of the indexer at index2.
// It queries the database for the last ID, searches the RSS feed since that ID,
// and updates the database with the new last ID.
// Returns true if the search was successful, false otherwise.
func runrsssearch(index2 int, searchvar *ConfigSearcher, _ *searchparams) bool {
	xmlret := apiexternal.QueryNewznabRSSLast(searchvar.Quality.Indexer[index2].CfgIndexer, searchvar.Quality, database.GetdatarowN[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", &searchvar.Cfgp.NamePrefix, &searchvar.Quality.Name, &searchvar.Quality.Indexer[index2].CfgIndexer.URL), qualityIndexerByQualityAndTemplate(searchvar.Quality, searchvar.Quality.Indexer[index2].CfgIndexer), searchvar.Mu, &searchvar.Dl.Raw)
	if xmlret.Err == nil {
		if xmlret.FirstID != "" {
			addrsshistory(&searchvar.Quality.Indexer[index2].CfgIndexer.URL, &xmlret.FirstID, searchvar.Quality, &searchvar.Cfgp.NamePrefix)
		}

		return true
	}
	if xmlret.Err != nil {
		if !errors.Is(xmlret.Err, logger.Errnoresults) && !errors.Is(xmlret.Err, logger.ErrToWait) {
			logger.LogDynamic("error", "Error searching indexer", logger.NewLogFieldValue(xmlret.Err), logger.NewLogField("indexer", searchvar.Quality.Indexer[index2].CfgIndexer.Name))
		}
	}
	return false
}

// NewSearcher creates a new ConfigSearcher instance.
// It initializes the searcher with the given media type config,
// quality config, search action type, and media ID.
// If no quality config is provided but a media ID is given,
// it will look up the quality config for that media in the database.
// It gets a searcher instance from the pool and sets the configs,
// then returns the initialized searcher.
func NewSearcher(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, searchActionType string, mediaid *uint) *ConfigSearcher {
	s := plsearcher.Get()
	//s := logger.GetPool(plsearcher) //plsearcher.Get().(*ConfigSearcher)
	s.searchActionType = searchActionType
	s.Cfgp = cfgp
	if quality == nil && mediaid != nil && *mediaid != 0 {
		s.Quality = database.GetMediaQualityConfig(cfgp, mediaid)
	} else {
		s.Quality = quality
	}
	return s
}

// Getnewznabrss queries Newznab indexers from the given MediaListsConfig
// using the provided MediaTypeConfig. It searches for and downloads any
// matching RSS feed items.
func Getnewznabrss(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig) error {
	if list.CfgList == nil || cfgp == nil {
		return logger.ErrNotFound
	}

	NewSearcher(cfgp, list.CfgQuality, logger.StrRss, &logger.V0).GetRSSFeed(list, true, true)
	return nil
}

// searchSeriesRSSSeason searches configured indexers for the given TV series
// season using the RSS search APIs. It handles executing searches across
// enabled newznab indexers, parsing results, and adding accepted entries to
// the search results. Returns the searcher and error if any.
func (searchvar *ConfigSearcher) searchSeriesRSSSeason(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, thetvdbid int, season string, useseason bool, downloadentries bool, autoclose bool) error {
	if autoclose {
		defer searchvar.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	if searchvar == nil {
		return logger.ErrCfgpNotFound
	}
	searchvar.Dl.Clear()
	searchvar.Quality = quality
	logger.LogDynamic("info", "Search for season", logger.NewLogField(logger.StrTvdb, thetvdbid), logger.NewLogField("season", season))

	params := plparams.Get()
	params.thetvdbid = thetvdbid
	params.season = season
	params.useseason = useseason
	if searchvar.searchindexers(false, params, runrssseasonsearch) && len(searchvar.Dl.Raw) >= 1 {
		if len(searchvar.Dl.Raw) >= 1 {
			searchvar.searchparse(nil)

			if downloadentries {
				searchvar.Download()
			}
		}
		logger.LogDynamic("info", "Ended Search for season", logger.NewLogField(logger.StrTvdb, thetvdbid), logger.NewLogField("season", season), logger.NewLogField("accepted", len(searchvar.Dl.Accepted)), logger.NewLogField("denied", len(searchvar.Dl.Denied)))
	}
	plparams.Put(params)
	return nil
}

// runrssseasonsearch executes a season search for the given TV series on the
// indexer at the provided index. It returns true if the search was successful,
// false otherwise.
func runrssseasonsearch(index2 int, searchvar *ConfigSearcher, params *searchparams) bool {
	xmlret := apiexternal.QueryNewznabTvTvdb(searchvar.Quality.Indexer[index2].CfgIndexer, searchvar.Quality, params.thetvdbid, qualityIndexerByQualityAndTemplate(searchvar.Quality, searchvar.Quality.Indexer[index2].CfgIndexer), params.season, "", params.useseason, false, searchvar.Mu, &searchvar.Dl.Raw)
	if xmlret.Err == nil {
		return true
	}
	if xmlret.Err != nil {
		if !errors.Is(xmlret.Err, logger.Errnoresults) && !errors.Is(xmlret.Err, logger.ErrToWait) {
			logger.LogDynamic("error", "Error searching indexer", logger.NewLogFieldValue(xmlret.Err), logger.NewLogField("indexer", searchvar.Quality.Indexer[index2].CfgIndexer.Name))
		}
	}
	return false
}

// addrsshistory updates the rss history table with the last processed item id
// for the given rss feed url, quality profile name, and config name. It will
// insert a new row if one does not exist yet for that combination.
func addrsshistory(urlv *string, lastid *string, quality *config.QualityConfig, configv *string) {
	id := database.GetdatarowN[uint](false, "select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", configv, &quality.Name, urlv)
	if id >= 1 {
		database.ExecN("update r_sshistories set last_id = ? where id = ?", lastid, &id)
	} else {
		database.ExecN("insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?)", configv, &quality.Name, urlv, lastid)
	}
}

// getsearchtype returns the search type string based on the minimumPriority,
// dont, and force parameters. If minimumPriority is 0, returns "missing".
// If dont is true and force is false, returns a disabled error.
// Otherwise returns "upgrade".
func getsearchtype(minimumPriority int, dont bool, force bool) (string, error) {
	if minimumPriority == 0 {
		return "missing", nil
	} else if dont && !force {
		return "", logger.ErrDisabled
	}
	return "upgrade", nil
}

// MediaSearch searches indexers for the given media entry (movie or TV episode)
// using the configured quality profile. It handles filling search variables,
// executing searches across enabled indexers, parsing results, and optionally
// downloading accepted entries. Returns the search results and error if any.
func (searchvar *ConfigSearcher) MediaSearch(cfgp *config.MediaTypeConfig, mediaid *uint, titlesearch bool, downloadentries bool, autoclose bool) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	if searchvar == nil || mediaid == nil || *mediaid == 0 {
		return logger.ErrSearchvarEmpty
	}
	if autoclose {
		defer searchvar.Close()
	}
	searchvar.Dl.Clear()
	if searchvar.Quality == nil {
		searchvar.Quality = database.GetMediaQualityConfig(cfgp, mediaid)
	}
	var err error
	if cfgp.Useseries {
		err = searchvar.EpisodeFillSearchVar(*mediaid)
	} else {
		err = searchvar.MovieFillSearchVar(*mediaid)
	}
	if err != nil {
		if !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			logger.LogDynamic("error", "Media Search Failed", logger.NewLogFieldValue(err), logger.NewLogField("id", mediaid))
		}
		return err
	}

	if searchvar.Quality == nil {
		return logger.ErrSearchvarEmpty
	}

	logger.LogDynamic("info", "Search for media id", logger.NewLogField(logger.StrID, mediaid), logger.NewLogField("series", searchvar.Cfgp.Useseries))

	params := plparams.Get()
	params.titlesearch = titlesearch
	params.sourcealttitles = getsourcetitles(searchvar)
	if !searchvar.searchindexers(false, params, runmediasearch) {
		logger.LogDynamic("error", "All searches failed")
		plparams.Put(params)
		return nil
	}
	database.ExecN(logger.GetStringsMap(cfgp.Useseries, logger.UpdateMediaLastscan), mediaid)

	if len(searchvar.Dl.Raw) >= 1 {
		searchvar.searchparse(params.sourcealttitles)
		if len(searchvar.Dl.Accepted) >= 1 || len(searchvar.Dl.Denied) >= 1 {
			logger.LogDynamic("info", "Ended Search for media id", logger.NewLogField(logger.StrID, mediaid), logger.NewLogField("series", searchvar.Cfgp.Useseries), logger.NewLogField("Accepted", len(searchvar.Dl.Accepted)), logger.NewLogField("Denied", len(searchvar.Dl.Denied)))
		}
		if downloadentries {
			searchvar.Download()
		}
	}
	plparams.Put(params)
	return nil
}

// getsourcetitles returns alternate titles from the database for the
// given ConfigSearcher if backup searching for alternate titles is enabled.
func getsourcetitles(searchvar *ConfigSearcher) []database.DbstaticTwoStringOneInt {
	if searchvar.Quality.BackupSearchForAlternateTitle {
		return database.Getentryalternatetitlesdirect(searchvar.Sourceentry.Dbid, searchvar.Cfgp.Useseries)
	}
	return nil
}

// searchparams is a struct containing fields for search parameters
type searchparams struct {
	// titlesearch is a boolean indicating if title search is enabled
	titlesearch bool
	// sourcealttitles is a slice of DbstaticTwoStringOneInt containing alternate titles
	sourcealttitles []database.DbstaticTwoStringOneInt
	// thetvdbid is an integer containing thetvdb ID
	thetvdbid int
	// season is a string containing the season number
	season string
	// useseason is a boolean indicating if season search is enabled
	useseason bool
}

// getaddstr returns a string to append to the search query based on the
// media year or identifier if available. For series it returns empty string.
func getaddstr(searchvar *ConfigSearcher) string {
	if !searchvar.Cfgp.Useseries && searchvar.Sourceentry.Info.M.Year != 0 {
		//addstr = logger.JoinStrings(" ", strconv.Itoa(searchvar.Sourceentry.Info.M.Year))
		return " " + strconv.Itoa(searchvar.Sourceentry.Info.M.Year)
	} else if searchvar.Sourceentry.Info.M.Identifier != "" {
		//addstr = logger.JoinStrings(" ", searchvar.Sourceentry.Info.M.Identifier)
		return " " + searchvar.Sourceentry.Info.M.Identifier
	}
	return ""
}

// runmediasearch searches for the given media using the provided search parameters.
// It iterates through the configured indexers and attempts different search queries based on the alternate titles.
// Returns true if the search completed successfully, false otherwise.
func runmediasearch(index2 int, searchvar *ConfigSearcher, param *searchparams) bool {
	cats := qualityIndexerByQualityAndTemplate(searchvar.Quality, searchvar.Quality.Indexer[index2].CfgIndexer)
	if cats == nil {
		logger.LogDynamic("error", "Error getting quality config")
		return false
	}
	addstr := getaddstr(searchvar)

	usequerysearch := true
	if !param.titlesearch {
		if !searchvar.Cfgp.Useseries && searchvar.Sourceentry.Info.M.Imdb != "" {
			usequerysearch = false
		} else if searchvar.Cfgp.Useseries && searchvar.Sourceentry.NZB.TVDBID != 0 {
			usequerysearch = false
		}
	}

	alttitles := searchvar.Getsearchalternatetitles(param.sourcealttitles, param.titlesearch, searchvar.Quality)
	if alttitles == nil {
		logger.LogDynamic("error", "Error getting search alternate titles")
		return false
	}
	searched := alttitles[:0]
	//if len(alttitles) > 2 {
	//searchvar.Dl.Raw = slices.Grow(searchvar.Dl.Raw, searchvar.Quality.Indexer[index2].CfgIndexer.MaxEntries*len(alttitles))
	//}
	//labTitles:
	var xmlret apiexternal.XMLResponse
	var searchtitle string
	for loopidx := range alttitles {
		if alttitles[loopidx].Str1 == "" && loopidx != 0 {
			logger.LogDynamic("error", "Skipped empty title")
			continue
		}

		if !usequerysearch {
			if !searchvar.Cfgp.Useseries && searchvar.Sourceentry.Info.M.Imdb != "" {
				xmlret = apiexternal.QueryNewznabMovieImdb(searchvar.Quality.Indexer[index2].CfgIndexer, searchvar.Quality, strings.Trim(searchvar.Sourceentry.Info.M.Imdb, "t"), cats, searchvar.Mu, &searchvar.Dl.Raw)
			} else if searchvar.Cfgp.Useseries && searchvar.Sourceentry.NZB.TVDBID != 0 {
				xmlret = apiexternal.QueryNewznabTvTvdb(searchvar.Quality.Indexer[index2].CfgIndexer, searchvar.Quality, searchvar.Sourceentry.NZB.TVDBID, cats, searchvar.Sourceentry.NZB.Season, searchvar.Sourceentry.NZB.Episode, true, true, searchvar.Mu, &searchvar.Dl.Raw)
			}
		} else if alttitles[loopidx].Str1 != "" {
			if checkdbtwostrings(searched, alttitles[loopidx].Str1) {
				continue
			}
			searched = append(searched, alttitles[loopidx])

			searchtitle = logger.StringRemoveAllRunesMulti(alttitles[loopidx].Str1, '&', '(', ')')
			//searchtitle = logger.StringRemoveAllRunes(alttitles[loopidx].Str1, '&')
			//searchtitle = logger.StringRemoveAllRunes(searchtitle, '(')
			//searchtitle = logger.StringRemoveAllRunes(searchtitle, ')')
			xmlret = apiexternal.QueryNewznabQuery(searchvar.Quality.Indexer[index2].CfgIndexer, searchvar.Quality, logger.JoinStrings(searchtitle, addstr), cats, searchvar.Mu, &searchvar.Dl.Raw)
		} else {
			continue
		}

		if xmlret.Err != nil && !errors.Is(xmlret.Err, logger.ErrToWait) {
			if searchvar.Cfgp.Useseries {
				logger.LogDynamic("error", "Error Searching Media by Title", logger.NewLogField("Media ID", searchvar.Sourceentry.NzbepisodeID), logger.NewLogField("series", searchvar.Cfgp.Useseries), logger.NewLogField("Title", alttitles[loopidx]), logger.NewLogFieldValue(xmlret.Err))
			} else {
				logger.LogDynamic("error", "Error Searching Media by Title", logger.NewLogField("Media ID", searchvar.Sourceentry.NzbmovieID), logger.NewLogField("series", searchvar.Cfgp.Useseries), logger.NewLogField("Title", alttitles[loopidx]), logger.NewLogFieldValue(xmlret.Err))
			}
		} else {
			if searchvar.Quality.CheckUntilFirstFound && len(searchvar.Dl.Accepted) >= 1 {
				logger.LogDynamic("debug", "Broke loop - result found")
				break
			}
		}
		if searchvar.Quality.SearchForTitleIfEmpty && !param.titlesearch && len(searchvar.Dl.Raw) == 0 {
			param.titlesearch = true
			usequerysearch = true
		}
		if !param.titlesearch {
			break
		}
	}
	clear(searched)
	clear(alttitles)
	return true
}

// checkdbtwostrings checks if a given string exists in a slice of DbstaticTwoStringOneInt.
// Returns true if the string is found, false otherwise.
func checkdbtwostrings(tbl []database.DbstaticTwoStringOneInt, str1 string) bool {
	for idxepi := range tbl {
		if strings.EqualFold(tbl[idxepi].Str1, str1) {
			return true
		}
	}
	return false
}

// searchindexers searches the enabled indexers and runs the given
// callback function fn for each one, passing the indexer index, searcher,
// and search params. It returns true if any callback sets searchdone to true.
// It uses a worker group to run the searches concurrently.
func (s *ConfigSearcher) searchindexers(userss bool, param *searchparams, fn func(int, *ConfigSearcher, *searchparams) bool) bool {
	workergroup := worker.WorkerPoolIndexer.Group()
	var searchdone bool
	for index := range s.Quality.Indexer {
		if userss && !s.Quality.Indexer[index].CfgIndexer.Rssenabled {
			continue
		}
		if !userss && !s.Quality.Indexer[index].CfgIndexer.Enabled {
			continue
		}
		if s.Quality == nil || s.Quality.Indexer[index].CfgIndexer == nil || !strings.EqualFold(s.Quality.Indexer[index].CfgIndexer.IndexerType, "newznab") {
			continue
		}
		if ok, _ := apiexternal.NewznabCheckLimiter(s.Quality.Indexer[index].CfgIndexer); !ok {
			continue
		}
		//index2 := index
		workergroup.Submit(
			func() {
				done := fn(index, s, param)
				if !searchdone && done {
					searchdone = done
				}
			})
	}
	workergroup.Wait()
	return searchdone
}

// Download iterates through the Accepted list and starts downloading each entry,
// tracking entries already downloaded to avoid duplicates. It handles both movies
// and TV series based on config and entry details.
func (s *ConfigSearcher) Download() {
	if len(s.Dl.Accepted) == 0 {
		return
	}
	downloaded := make([]uint, 0, len(s.Dl.Accepted))

	for idx := range s.Dl.Accepted {
		if s.Dl.Accepted[idx].Info.M.Priority == 0 {
			logger.LogDynamic("error", "download not wanted", logger.NewLogField("series", s.Cfgp.Useseries), logger.NewLogField(logger.StrTitle, s.Dl.Accepted[idx].NZB.Title))
			continue
		}
		if checkdownloaded(downloaded, idx, s) {
			continue
		}
		qualcfg := s.getentryquality(&s.Dl.Accepted[idx])
		if qualcfg == nil {
			logger.LogDynamic("info", "nzb found - start downloading", logger.NewLogField("series", s.Cfgp.Useseries), logger.NewLogField(logger.StrTitle, s.Dl.Accepted[idx].NZB.Title), logger.NewLogField("minimum prio", s.Dl.Accepted[idx].MinimumPriority), logger.NewLogField(logger.StrPriority, s.Dl.Accepted[idx].Info.M.Priority))
		} else {
			logger.LogDynamic("info", "nzb found - start downloading", logger.NewLogField("series", s.Cfgp.Useseries), logger.NewLogField(logger.StrTitle, s.Dl.Accepted[idx].NZB.Title), logger.NewLogField("quality", qualcfg.Name), logger.NewLogField("minimum prio", s.Dl.Accepted[idx].MinimumPriority), logger.NewLogField(logger.StrPriority, s.Dl.Accepted[idx].Info.M.Priority))
		}
		if !s.Cfgp.Useseries && s.Dl.Accepted[idx].NzbmovieID != 0 {
			downloaded = append(downloaded, s.Dl.Accepted[idx].NzbmovieID)
			downloader.DownloadMovie(s.Cfgp, &s.Dl.Accepted[idx].NzbmovieID, &s.Dl.Accepted[idx])
		} else if s.Cfgp.Useseries && s.Dl.Accepted[idx].NzbepisodeID != 0 {
			downloaded = append(downloaded, s.Dl.Accepted[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(s.Cfgp, &s.Dl.Accepted[idx].NzbepisodeID, &s.Dl.Accepted[idx])
		} else if s.Dl.Accepted[idx].NzbmovieID != 0 {
			downloaded = append(downloaded, s.Dl.Accepted[idx].NzbmovieID)
			downloader.DownloadMovie(s.Cfgp, &s.Dl.Accepted[idx].NzbmovieID, &s.Dl.Accepted[idx])
		} else if s.Dl.Accepted[idx].NzbepisodeID != 0 {
			downloaded = append(downloaded, s.Dl.Accepted[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(s.Cfgp, &s.Dl.Accepted[idx].NzbepisodeID, &s.Dl.Accepted[idx])
		}
	}
}

// checkdownloaded checks if the entry at index idx has already been downloaded
// by looking for its movie ID or episode ID in the downloaded slice.
// It returns true if the entry is already in downloaded.
func checkdownloaded(downloaded []uint, idx int, s *ConfigSearcher) bool {
	for idxi := range downloaded {
		if s.Dl.Accepted[idx].NzbmovieID != 0 && downloaded[idxi] == s.Dl.Accepted[idx].NzbmovieID {
			return true
		}
		if s.Dl.Accepted[idx].NzbepisodeID != 0 && downloaded[idxi] == s.Dl.Accepted[idx].NzbepisodeID {
			return true
		}
	}
	return false
}

// AppendDenied appends the given Nzbwithprio entry to the denied list.
// If entry is nil, it returns immediately without appending.
func (s *ConfigSearcher) AppendDenied(entry *apiexternal.Nzbwithprio) {
	if entry == nil {
		return
	}
	s.Dl.Denied = append(s.Dl.Denied, *entry)
}

// Close closes all Nzbwithprio entries in the SearchResults by calling
// the Close method on each entry. It then sets the slices to nil if they have
// capacity, to allow garbage collection. It also calls the Clear method on
// the logger to clear any log entries related to this SearchResults.
func (s *searchResults) Close() {
	if s == nil {
		return
	}
	s.Denied = nil
	s.Accepted = nil
	s.Raw = s.Raw[:0]
}

// Clear closes all Nzbwithprio entries in the SearchResults by calling
// the Close method on each entry. It then resets the slices to empty
// while retaining capacity, to allow garbage collection.
func (s *searchResults) Clear() {
	if s == nil || cap(s.Raw) == 0 || len(s.Raw) == 0 {
		return
	}
	s.Denied = s.Denied[:0]
	s.Accepted = s.Accepted[:0]
	s.Raw = s.Raw[:0]
}

// filterTestQualityWanted checks if the quality attributes of the
// Nzbwithprio entry match the wanted quality configuration. It returns
// true if any unwanted quality is found to stop further processing of
// the entry.
func (s *ConfigSearcher) filterTestQualityWanted(entry *apiexternal.Nzbwithprio, quality *config.QualityConfig) bool {
	s.Dl.bl = false
	if quality == nil {
		return false
	}
	lenqual := quality.WantedResolutionLen
	if lenqual >= 1 && entry.Info.M.Resolution != "" {
		s.Dl.bl = logger.SlicesContainsI(quality.WantedResolution, entry.Info.M.Resolution)
	}

	if lenqual >= 1 && !s.Dl.bl {
		s.logdenied(strresolutionunwanted, entry, logger.NewLogField("resolution", entry.Info.M.Resolution))
		return true
	}
	s.Dl.bl = false

	lenqual = quality.WantedQualityLen
	if lenqual >= 1 && entry.Info.M.Quality != "" {
		s.Dl.bl = logger.SlicesContainsI(quality.WantedQuality, entry.Info.M.Quality)
	}
	if lenqual >= 1 && !s.Dl.bl {
		s.logdenied(strqualityunwanted, entry, logger.NewLogField("quality", entry.Info.M.Quality))
		return true
	}

	s.Dl.bl = false

	lenqual = quality.WantedAudioLen
	if lenqual >= 1 && entry.Info.M.Audio != "" {
		s.Dl.bl = logger.SlicesContainsI(quality.WantedAudio, entry.Info.M.Audio)
	}
	if lenqual >= 1 && !s.Dl.bl {
		s.logdenied(straudiounwanted, entry, logger.NewLogField("audio", entry.Info.M.Audio))
		return true
	}
	s.Dl.bl = false

	lenqual = quality.WantedCodecLen
	if lenqual >= 1 && entry.Info.M.Codec != "" {
		s.Dl.bl = logger.SlicesContainsI(quality.WantedCodec, entry.Info.M.Codec)
	}
	if lenqual >= 1 && !s.Dl.bl {
		s.logdenied(strcodecunwanted, entry, logger.NewLogField("codec", entry.Info.M.Codec))
		return true
	}
	return false
}

// Close closes the ConfigSearcher, including closing any open connections and clearing resources.
func (s *ConfigSearcher) Close() {
	if s == nil {
		return
	}
	*s = ConfigSearcher{Dl: searchResults{Raw: s.Dl.Raw[:0]}, Mu: s.Mu}
	plsearcher.Put(s)
}

// searchparse parses the raw search results, runs validation on each entry, assigns quality
// profiles and priorities, separates accepted and denied entries, and sorts accepted entries
// by priority
func (s *ConfigSearcher) searchparse(alttitles []database.DbstaticTwoStringOneInt) {
	if len(s.Dl.Raw) == 0 {
		return
	}
	s.Dl.Denied = s.Dl.Raw[:0]
	s.Dl.Accepted = s.Dl.Raw[:0]
	var skipemptysize bool
	for idxraw := range s.Dl.Raw {
		if s.Dl.Raw[idxraw].NZB.DownloadURL == "" {
			s.logdenied(strnourl, &s.Dl.Raw[idxraw])
			continue
		}
		if s.Dl.Raw[idxraw].NZB.Title == "" {
			s.logdenied(strnotitle, &s.Dl.Raw[idxraw])
			continue
		}
		if s.Dl.Raw[idxraw].NZB.Title != "" && (s.Dl.Raw[idxraw].NZB.Title[:1] == " " || s.Dl.Raw[idxraw].NZB.Title[len(s.Dl.Raw[idxraw].NZB.Title)-1:] == " ") {
			s.Dl.Raw[idxraw].NZB.Title = strings.Trim(s.Dl.Raw[idxraw].NZB.Title, " ")
		}
		if len(s.Dl.Raw[idxraw].NZB.Title) <= 3 {
			s.logdenied(strshorttitle, &s.Dl.Raw[idxraw])
			continue
		}
		if s.checkprocessed(&s.Dl.Raw[idxraw]) {
			continue
		}
		//Check Size
		skipemptysize = false
		if s.Dl.Raw[idxraw].NZB.Indexer != nil {
			indcfg := qualityIndexerByQualityAndTemplate(s.Quality, s.Dl.Raw[idxraw].NZB.Indexer)
			if indcfg != nil {
				skipemptysize = indcfg.SkipEmptySize
			}
			if !skipemptysize {
				if _, ok := config.SettingsList[s.Dl.Raw[idxraw].NZB.Indexer.Name]; ok {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				} else if getlistbyindexer(s.Dl.Raw[idxraw].NZB.Indexer) != nil {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				}
			}
		}

		if skipemptysize && s.Dl.Raw[idxraw].NZB.Size == 0 {
			s.logdenied(strnosize, &s.Dl.Raw[idxraw])
			continue
		}

		//check history
		if s.filterSizeNzbs(&s.Dl.Raw[idxraw]) {
			continue
		}
		if s.checkcorrectid(&s.Dl.Raw[idxraw]) {
			continue
		}

		parser.ParseFileP(s.Dl.Raw[idxraw].NZB.Title, false, false, s.Cfgp, -1, &s.Dl.Raw[idxraw].Info)
		//if s.searchActionType == logger.StrRss {
		if !s.Cfgp.Useseries && !s.Dl.Raw[idxraw].NZB.Indexer.TrustWithIMDBIDs {
			s.Dl.Raw[idxraw].Info.M.Imdb = ""
		}
		if s.Cfgp.Useseries && !s.Dl.Raw[idxraw].NZB.Indexer.TrustWithTVDBIDs {
			s.Dl.Raw[idxraw].Info.M.Tvdb = ""
		}
		//}
		if s.searchActionType == logger.StrRss {
			s.Sourceentry.Close()
		}
		err := parser.GetDBIDs(&s.Dl.Raw[idxraw].Info)
		if err != nil {
			s.logdeniederr(err, &s.Dl.Raw[idxraw])
			continue
		}
		var qual *config.QualityConfig
		var skip bool
		if s.searchActionType == logger.StrRss {
			skip, qual = s.getmediadatarss(&s.Dl.Raw[idxraw], -1, false)
			if skip {
				continue
			}
		} else {
			if s.getmediadata(&s.Dl.Raw[idxraw]) {
				continue
			}
			qual = s.Quality
			s.Dl.Raw[idxraw].WantedAlternates = alttitles
		}
		//needs the identifier from getmediadata

		if qual == nil {
			qual = s.getentryquality(&s.Dl.Raw[idxraw])
		}
		if qual == nil {
			s.logdenied(strqualityunknown, &s.Dl.Raw[idxraw])
			continue
		}
		if s.checkhistory(&s.Dl.Raw[idxraw], qual) {
			continue
		}
		if s.checkepisode(&s.Dl.Raw[idxraw]) {
			continue
		}

		if s.filterRegexNzbs(&s.Dl.Raw[idxraw], qual) {
			continue
		}

		if s.Dl.Raw[idxraw].Info.M.Priority == 0 {
			parser.GetPriorityMapQual(&s.Dl.Raw[idxraw].Info.M, s.Cfgp, qual, false, true)
		}

		importfeed.StripTitlePrefixPostfixGetQual(&s.Dl.Raw[idxraw].Info.M, qual)

		//check quality
		if s.filterTestQualityWanted(&s.Dl.Raw[idxraw], qual) {
			continue
		}
		//check priority

		if s.getminimumpriority(&s.Dl.Raw[idxraw], qual) {
			continue
		}
		if s.Dl.Raw[idxraw].Info.M.Priority == 0 {
			s.logdenied(strpriounknown, &s.Dl.Raw[idxraw])
			continue
		}
		if s.Dl.Raw[idxraw].MinimumPriority != 0 && s.Dl.Raw[idxraw].MinimumPriority == s.Dl.Raw[idxraw].Info.M.Priority {
			s.logdenied(strpriosame, &s.Dl.Raw[idxraw])
			continue
		}

		if s.Dl.Raw[idxraw].MinimumPriority != 0 {
			if qual.UseForPriorityMinDifference == 0 && s.Dl.Raw[idxraw].Info.M.Priority <= s.Dl.Raw[idxraw].MinimumPriority {
				s.logdenied(strpriolower, &s.Dl.Raw[idxraw], logger.NewLogField("found prio", s.Dl.Raw[idxraw].Info.M.Priority))
				continue
			}
			if qual.UseForPriorityMinDifference != 0 && s.Dl.Raw[idxraw].Info.M.Priority <= (s.Dl.Raw[idxraw].MinimumPriority+qual.UseForPriorityMinDifference) {
				s.logdenied(strpriolower, &s.Dl.Raw[idxraw], logger.NewLogField("found prio", s.Dl.Raw[idxraw].Info.M.Priority))
				continue
			}
		}

		if s.checkyear(&s.Dl.Raw[idxraw], qual) {
			continue
		}

		if s.checktitle(&s.Dl.Raw[idxraw], qual) {
			continue
		}

		logger.LogDynamic("debug", "Release ok", logger.NewLogField("quality", qual.Name), logger.NewLogField(logger.StrTitle, s.Dl.Raw[idxraw].NZB.Title), logger.NewLogField("minimum prio", s.Dl.Raw[idxraw].MinimumPriority), logger.NewLogField(logger.StrPriority, s.Dl.Raw[idxraw].Info.M.Priority))

		s.Dl.Accepted = append(s.Dl.Accepted, s.Dl.Raw[idxraw])
		if qual.CheckUntilFirstFound {
			break
		}
	}
	if database.DBLogLevel == logger.StrDebug {
		logger.LogDynamic("debug", "Entries found", logger.NewLogField("Count", len(s.Dl.Raw)))
	}
	if len(s.Dl.Accepted) > 1 {
		sort.Slice(s.Dl.Accepted, func(i, j int) bool {
			return s.Dl.Accepted[i].Info.M.Priority > s.Dl.Accepted[j].Info.M.Priority
		})
	}
}

// getminimumpriority checks the minimum priority configured for the entry's movie or series.
// It sets the MinimumPriority field on the entry based on priorities configured in the quality
// profiles. Returns true to skip the entry if upgrade/search is disabled or priority does not meet
// configured minimum.
func (s *ConfigSearcher) getminimumpriority(entry *apiexternal.Nzbwithprio, cfgqual *config.QualityConfig) bool {
	if entry.MinimumPriority != 0 {
		return false
	}
	if !s.Cfgp.Useseries {
		entry.MinimumPriority, _ = Getpriobyfiles(false, &entry.NzbmovieID, false, -1, cfgqual)
	} else {
		//Check Minimum Priority
		entry.MinimumPriority, _ = Getpriobyfiles(true, &entry.NzbepisodeID, false, -1, cfgqual)
	}
	if entry.MinimumPriority != 0 {
		if entry.DontUpgrade {
			s.logdenied(strupgradedisabled, entry)
			return true
		}
	} else {
		if entry.DontSearch {
			s.logdenied(strsearchdisabled, entry)
			return true
		}
	}
	return false
}

// getregexcfg returns the regex configuration for the given quality config
// that matches the indexer name in the given Nzbwithprio entry. It first checks
// the Indexer list in the quality config, and falls back to the SettingsList
// global config if no match is found. Returns nil if no match is found.
func getregexcfg(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) *config.RegexConfig {
	if entry.NZB.Indexer != nil {
		indcfg := qualityIndexerByQualityAndTemplate(qual, entry.NZB.Indexer)
		if indcfg != nil {
			return indcfg.CfgRegex
		}
		if _, ok := config.SettingsList[entry.NZB.Indexer.Name]; ok {
			return qual.Indexer[0].CfgRegex
		}
		if getlistbyindexer(entry.NZB.Indexer) != nil {
			return qual.Indexer[0].CfgRegex
		}
	}
	return nil
}

// checkcorrectid checks if the entry matches the expected ID based on
// whether it is a movie or series. For movies it checks the IMDB ID,
// trimming any "t0" prefix. For series it checks the TVDB ID. If the
// IDs don't match, it logs a message and returns true to skip the entry.
func (s *ConfigSearcher) checkcorrectid(entry *apiexternal.Nzbwithprio) bool {
	if s.searchActionType == logger.StrRss {
		return false
	}
	if !s.Cfgp.Useseries {
		if entry.NZB.IMDBID != "" && s.Sourceentry.Info.M.Imdb != "" && s.Sourceentry.Info.M.Imdb != entry.NZB.IMDBID {
			if strings.TrimLeft(s.Sourceentry.Info.M.Imdb, "t0") != strings.TrimLeft(entry.NZB.IMDBID, "t0") {
				s.logdenied(strnotmatchimdb, entry)
				return true
			}
		}
		return false
	}
	if s.Sourceentry.NZB.TVDBID != 0 && entry.NZB.TVDBID != 0 && s.Sourceentry.NZB.TVDBID != entry.NZB.TVDBID {
		s.logdenied(strnotmatchtvdb, entry)
		return true
	}
	return false
}

// getmediadata validates the media data in the given entry against the
// source entry to determine if it is a match. It sets various priority
// and search control fields on the entry based on the source entry
// configuration. Returns true to skip/reject the entry if no match, false
// to continue processing if a match.
func (s *ConfigSearcher) getmediadata(entry *apiexternal.Nzbwithprio) bool {
	if !s.Cfgp.Useseries {
		if s.Sourceentry.NzbmovieID != entry.Info.M.MovieID {
			s.logdenied(strmovieunwanted, entry)
			return true
		}
		entry.NzbmovieID = s.Sourceentry.NzbmovieID
		entry.Dbid = s.Sourceentry.Dbid
		entry.MinimumPriority = s.Sourceentry.MinimumPriority
		entry.DontSearch = s.Sourceentry.DontSearch
		entry.DontUpgrade = s.Sourceentry.DontUpgrade
		entry.WantedTitle = s.Sourceentry.WantedTitle
		return false
	}

	//Parse Series
	if entry.Info.M.SerieEpisodeID != s.Sourceentry.NzbepisodeID {
		s.logdenied(strepisodeunwanted, entry)
		return true
	}
	entry.NzbepisodeID = s.Sourceentry.NzbepisodeID
	entry.Dbid = s.Sourceentry.Dbid
	entry.MinimumPriority = s.Sourceentry.MinimumPriority
	entry.DontSearch = s.Sourceentry.DontSearch
	entry.DontUpgrade = s.Sourceentry.DontUpgrade
	entry.WantedTitle = s.Sourceentry.WantedTitle

	return false
}

// getmediadatarss processes an Nzbwithprio entry for adding to the RSS feed.
// It handles movie and series entries differently based on ConfigSearcher.Cfgp.Useseries.
// For movies, it tries to add the entry to the list with ID addinlistid, or adds it if addifnotfound is true.
// For series, it calls getserierss to filter the entry.
// It returns true if the entry should be skipped.
func (s *ConfigSearcher) getmediadatarss(entry *apiexternal.Nzbwithprio, addinlistid int, addifnotfound bool) (bool, *config.QualityConfig) {
	if !s.Cfgp.Useseries {
		if addinlistid != -1 && s.Cfgp != nil {
			entry.Info.M.ListID = addinlistid
		}

		return s.getmovierss(entry, addinlistid, addifnotfound)
	}

	//Parse Series
	//Filter RSS Series
	return s.getserierss(entry)
}

// checkyear validates the year in the entry title against the year
// configured for the wanted entry. It returns false if a match is found,
// or true to skip the entry if no match is found. This is used during
// search result processing to filter entries by year.
func (s *ConfigSearcher) checkyear(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	if s.Cfgp.Useseries || s.searchActionType == logger.StrRss {
		return false
	}
	if s.Sourceentry.Info.M.Year == 0 {
		s.logdenied(strnoyear, entry)
		return true
	}
	if (qual.CheckYear || qual.CheckYear1) && logger.ContainsInt(entry.NZB.Title, s.Sourceentry.Info.M.Year) {
		return false
	}
	if qual.CheckYear1 && logger.ContainsInt(entry.NZB.Title, s.Sourceentry.Info.M.Year+1) {
		return false
	}
	if qual.CheckYear1 && logger.ContainsInt(entry.NZB.Title, s.Sourceentry.Info.M.Year-1) {
		return false
	}
	s.logdenied(stryearunwanted, entry)
	return true
}

// checktitle validates the title and alternate titles of the entry against
// the wanted title and quality configuration. It returns false if a match is
// found, or true to skip the entry if no match is found. This is an internal
// function used during search result processing.
func (s *ConfigSearcher) checktitle(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	//Checktitle
	if !qual.CheckTitle {
		return false
	}
	if qual != nil {
		importfeed.StripTitlePrefixPostfixGetQual(&entry.Info.M, qual)
	}

	wantedslug := logger.StringToSlug(entry.WantedTitle)
	if entry.WantedTitle != "" {
		if qual.CheckTitle && entry.WantedTitle != "" && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, entry.Info.M.Title, qual.CheckYear1, entry.Info.M.Year) {
			return false
		}
	}
	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.Info.M.Title, ']') {
		for i := len(entry.Info.M.Title) - 1; i >= 0; i-- {
			if strings.EqualFold(entry.Info.M.Title[i:i+1], "]") {
				if i < (len(entry.Info.M.Title) - 1) {
					trytitle = strings.TrimLeft(entry.Info.M.Title[i+1:], "-. ")
					if qual.CheckTitle && entry.WantedTitle != "" && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle, qual.CheckYear1, entry.Info.M.Year) {
						return false
					}
				}
			}
		}
		// return -1

		// 	u := logger.IndexILast(entry.Info.M.Title, "]")
		// 	if u != -1 && u < (len(entry.Info.M.Title)-1) {
		// 		trytitle = strings.TrimLeft(entry.Info.M.Title[u+1:], "-. ")
		// 		if qual.CheckTitle && entry.WantedTitle != "" && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle, qual.CheckYear1, entry.Info.M.Year) {
		// 			return false
		// 		}
		// 	}
	}
	if entry.Dbid != 0 && len(entry.WantedAlternates) == 0 {
		entry.WantedAlternates = database.Getentryalternatetitlesdirect(entry.Dbid, s.Cfgp.Useseries)
	}

	if entry.Info.M.Title == "" || len(entry.WantedAlternates) == 0 {
		s.logdenied(strtitleunwanted, entry)
		return true
	}
	for idx := range entry.WantedAlternates {
		if entry.WantedAlternates[idx].Str1 == "" {
			continue
		}
		if entry.WantedAlternates[idx].Str2 == "" {
			entry.WantedAlternates[idx].Str2 = logger.StringToSlug(entry.WantedAlternates[idx].Str1)
		}

		if apiexternal.ChecknzbtitleB(entry.WantedAlternates[idx].Str1, entry.WantedAlternates[idx].Str2, entry.Info.M.Title, qual.CheckYear1, entry.Info.M.Year) {
			return false
		}

		if trytitle != "" {
			if apiexternal.ChecknzbtitleB(entry.WantedAlternates[idx].Str1, entry.WantedAlternates[idx].Str2, trytitle, qual.CheckYear1, entry.Info.M.Year) {
				return false
			}
		}
	}
	s.logdenied(strtitlealternateunwanted, entry)
	return true
}

// checkepisode validates the episode identifier in the entry against the
// season and episode values. It returns false if the identifier matches the
// expected format, or true to skip the entry if the identifier is invalid.
func (s *ConfigSearcher) checkepisode(entry *apiexternal.Nzbwithprio) bool {
	//Checkepisode
	if !s.Cfgp.Useseries {
		return false
	}
	if s.Sourceentry.Info.M.Identifier == "" && s.searchActionType == logger.StrRss {
		return false
	}
	if s.Sourceentry.Info.M.Identifier == "" {
		s.logdenied(strnoidentifier, entry)
		return true
	}

	// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
	altIdentifier := string(logger.ByteReplaceWithByte(logger.StringReplaceWithByte(strings.TrimLeft(s.Sourceentry.Info.M.Identifier, "sS0"), 'e', 'x'), 'E', 'x'))
	if logger.ContainsI(entry.NZB.Title, s.Sourceentry.Info.M.Identifier) ||
		logger.ContainsI(entry.NZB.Title, altIdentifier) {
		return false
	}
	if strings.ContainsRune(s.Sourceentry.Info.M.Identifier, '-') {
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(s.Sourceentry.Info.M.Identifier, '-', '.')) ||
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', '.')) {
			return false
		}
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(s.Sourceentry.Info.M.Identifier, '-', ' ')) ||
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', ' ')) {
			return false
		}
	}

	if s.Sourceentry.NZB.Season == "" || s.Sourceentry.NZB.Episode == "" {
		s.logdenied(stridentifierunwanted, entry, logger.NewLogField("identifier", s.Sourceentry.Info.M.Identifier))
		return true
	}

	sprefix, eprefix := "s", "e"
	if logger.ContainsI(s.Sourceentry.Info.M.Identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else if !logger.ContainsI(s.Sourceentry.Info.M.Identifier, "s") && !logger.ContainsI(s.Sourceentry.Info.M.Identifier, "e") {
		s.logdenied(stridentifierunwanted, entry, logger.NewLogField("identifier", s.Sourceentry.Info.M.Identifier))
		return true
	}

	if !logger.HasPrefixI(s.Sourceentry.Info.M.Identifier, sprefix+s.Sourceentry.NZB.Season) {
		s.logdenied(strseasonunwanted, entry, logger.NewLogField("identifier", s.Sourceentry.Info.M.Identifier))
		return true
	}
	if !logger.ContainsI(s.Sourceentry.Info.M.Identifier, s.Sourceentry.NZB.Episode) {
		return true
	}

	//suffixcheck

	if checkprefixarray(s.Sourceentry.Info.M.Identifier, eprefix, s.Sourceentry.NZB.Episode) {
		return false
	}

	episodesuffixarray := []string{eprefix, " ", "-"}
	firstpart := eprefix + s.Sourceentry.NZB.Episode
	for idx := range episodesuffixarray {
		if logger.ContainsI(s.Sourceentry.Info.M.Identifier, firstpart+episodesuffixarray[idx]) {
			return false
		}
		if checkprefixarray(s.Sourceentry.Info.M.Identifier, eprefix, s.Sourceentry.NZB.Episode+episodesuffixarray[idx]) {
			return false
		}
	}

	s.logdenied(stridentifierunwanted, entry, logger.NewLogField("identifier", s.Sourceentry.Info.M.Identifier))
	return true
}

// checkprefixarray checks if string s contains v1+any value in episodeprefixarray+v2 as a suffix.
// Returns true if a match is found, false otherwise.
func checkprefixarray(s string, v1, v2 string) bool {
	for idxsub := range episodeprefixarray {
		if logger.HasSuffixI(s, v1+episodeprefixarray[idxsub]+v2) {
			return true
		}
	}
	return false
}

// getmovierss validates the movie data in the entry, sets additional fields,
// queries the database for movie data like dontSearch/dontUpgrade flags,
// and returns false to continue search result processing or true to skip the entry.
func (s *ConfigSearcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlistid int, addifnotfound bool) (bool, *config.QualityConfig) {
	//Add DbMovie if not found yet and enabled
	if entry.Info.M.DbmovieID == 0 && (!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
		s.logdenied(strdbmovieunwanted, entry)
		return true, nil
	}
	imdb := importfeed.ImdbID{Imdb: entry.NZB.IMDBID}
	//add movie if not found
	if addifnotfound && (entry.Info.M.DbmovieID == 0 || entry.Info.M.MovieID == 0) && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if addinlistid == -1 {
			return true, nil
		}
		bl, err := importfeed.AllowMovieImport(&imdb, s.Cfgp.Lists[addinlistid].CfgList)
		if err != nil {
			s.logdeniederr(err, entry)
			return true, nil
		}
		if !bl {
			s.logdenied(strunalloweddbmovie, entry)
			return true, nil
		}
	}
	if addifnotfound && entry.Info.M.DbmovieID == 0 && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		var err error
		entry.Info.M.DbmovieID, err = importfeed.JobImportMovies(&imdb, s.Cfgp, addinlistid, true)
		if err != nil {
			s.logdeniederr(err, entry)
			return true, nil
		}
		database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.M.MovieID, &entry.Info.M.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
		if entry.Info.M.MovieID == 0 || entry.Info.M.DbmovieID == 0 {
			s.logdenied(strmovieunwanted, entry)
			return true, nil
		}
	}
	if entry.Info.M.DbmovieID == 0 {
		s.logdenied(strdbmovieunwanted, entry)
		return true, nil
	}

	//continue only if dbmovie found
	//Get List of movie by dbmovieid, year and possible lists

	//if list was not found : should we add the movie to the list?
	if addifnotfound && entry.Info.M.MovieID == 0 && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if addinlistid == -1 {
			s.logdenied(strnoaddinlist, entry)
			return true, nil
		}

		err := importfeed.Checkaddmovieentry(entry.Info.M.DbmovieID, &s.Cfgp.Lists[addinlistid], &imdb)
		if err != nil {
			s.logdeniederr(err, entry)
			return true, nil
		}
		if entry.Info.M.DbmovieID != 0 && entry.Info.M.MovieID == 0 {
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.M.MovieID, &entry.Info.M.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
		}
		if entry.Info.M.DbmovieID == 0 || entry.Info.M.MovieID == 0 {
			s.logdenied(strmovieunwanted, entry)
			return true, nil
		}
	}

	if entry.Info.M.MovieID == 0 {
		s.logdenied(strmovieunwanted, entry)
		return true, nil
	}
	entry.Dbid = entry.Info.M.DbmovieID
	entry.NzbmovieID = entry.Info.M.MovieID

	database.GetdatarowArgs("select movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?", &entry.Info.M.MovieID, &entry.DontSearch, &entry.DontUpgrade, &entry.Listname, &entry.Quality, &entry.WantedTitle)

	entry.Info.M.ListID = database.GetMediaListID(s.Cfgp, entry.Listname)
	if entry.Quality == "" {
		return false, config.SettingsQuality[s.Cfgp.DefaultQuality]
	}
	return false, config.SettingsQuality[entry.Quality]
}

// getentryquality returns the quality config for the given entry.
// If the entry is for a movie, it gets the config from the movies database using the movie ID.
// If the entry is for a TV episode, it gets the config from the series database using the episode ID.
// If no ID is set, it returns nil.
func (s *ConfigSearcher) getentryquality(entry *apiexternal.Nzbwithprio) *config.QualityConfig {
	if entry.Info.M.MovieID != 0 {
		return database.GetMediaQualityConfig(s.Cfgp, &entry.Info.M.MovieID)
	}
	if entry.Info.M.SerieEpisodeID != 0 {
		return database.GetMediaQualityConfig(s.Cfgp, &entry.Info.M.SerieEpisodeID)
	}
	return nil
}

// getserierss validates the series data in the entry, sets additional fields,
// queries the database for series/episode data like dontSearch/dontUpgrade flags,
// and returns false to continue search result processing or true to skip the entry.
func (s *ConfigSearcher) getserierss(entry *apiexternal.Nzbwithprio) (bool, *config.QualityConfig) {
	if entry.Info.M.SerieID == 0 {
		s.logdenied(strserieunwanted, entry)
		return true, nil
	}
	if entry.Info.M.DbserieID == 0 {
		s.logdenied(strdbserieunwanted, entry)
		return true, nil
	}
	if entry.Info.M.DbserieEpisodeID == 0 {
		s.logdenied(strdbepisodeunwanted, entry)
		return true, nil
	}
	if entry.Info.M.SerieEpisodeID == 0 {
		s.logdenied(strepisodeunwanted, entry)
		return true, nil
	}
	entry.NzbepisodeID = entry.Info.M.SerieEpisodeID
	entry.Dbid = entry.Info.M.DbserieID

	database.GetdatarowArgs("select serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.seriename from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id where serie_episodes.id = ?", &entry.Info.M.SerieEpisodeID, &entry.DontSearch, &entry.DontUpgrade, &entry.Quality, &entry.Listname, &entry.WantedTitle)
	entry.Info.M.ListID = database.GetMediaListID(s.Cfgp, entry.Listname)
	if entry.Quality == "" {
		return false, config.SettingsQuality[s.Cfgp.DefaultQuality]
	}
	return false, config.SettingsQuality[entry.Quality]
}

// GetRSSFeed queries the RSS feed for the given media list, searches for and downloads new items,
// and adds them to the search results. It handles checking if the indexer is blocked,
// configuring the custom RSS feed URL, getting the last ID to prevent duplicates,
// parsing results, and updating the RSS history.
func (s *ConfigSearcher) GetRSSFeed(listentry *config.MediaListsConfig, downloadentries bool, autoclose bool) error {
	if autoclose {
		defer s.Close()
	}
	if listentry.TemplateList == "" {
		return errors.New("listname template empty")
	}

	s.searchActionType = logger.StrRss

	intid := -1
	for index := range s.Quality.Indexer {
		if strings.EqualFold(s.Quality.Indexer[index].TemplateIndexer, listentry.TemplateList) {
			intid = index
			break
		}
	}
	if intid != -1 && s.Quality.Indexer[intid].TemplateRegex == "" {
		return errors.New("regex template empty")
	}

	blockinterval := -5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.SettingsGeneral.FailedIndexerBlockTime
	}

	intval := logger.TimeGetNow().Add(time.Minute * time.Duration(blockinterval))
	if database.GetdatarowN[uint](false, "select count() from indexer_fails where indexer = ? and last_fail > ?", &listentry.CfgList.URL, &intval) >= 1 {
		logger.LogDynamic("debug", "Indexer temporarily disabled due to fail in last", logger.NewLogField(logger.StrListname, listentry.TemplateList), logger.NewLogField("Minutes", blockinterval))
		return logger.ErrDisabled
	}

	if s.Cfgp == nil {
		return logger.ErrOther
	}

	customindexer := *config.SettingsIndexer[listentry.TemplateList]
	customindexer.Name = listentry.TemplateList
	customindexer.Customrssurl = listentry.CfgList.URL
	customindexer.URL = listentry.CfgList.URL
	customindexer.MaxEntries = logger.StringToInt(listentry.CfgList.Limit)
	xmlret := apiexternal.QueryNewznabRSSLastCustom(&customindexer, s.Quality, database.GetdatarowN[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE", &listentry.TemplateList, &s.Quality.Name), nil, s.Mu, &s.Dl.Raw)
	if xmlret.FirstID != "" && len(s.Dl.Raw) >= 1 {
		addrsshistory(&listentry.CfgList.URL, &xmlret.FirstID, s.Quality, &s.Cfgp.NamePrefix)
	}
	if xmlret.Err != nil {
		return xmlret.Err
	}
	if len(s.Dl.Raw) >= 1 {
		if xmlret.FirstID != "" {
			addrsshistory(&listentry.CfgList.URL, &xmlret.FirstID, s.Quality, &listentry.TemplateList)
		}
		s.searchparse(nil)

		if downloadentries {
			s.Download()
		}
	}

	return nil
}

// MovieFillSearchVar fills the search variables for the given movie ID.
// It queries the database to get the movie details and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) MovieFillSearchVar(movieid uint) error {
	s.Sourceentry.Close()
	s.Sourceentry.NzbmovieID = movieid
	database.GetdatarowArgs("select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?", &movieid, &s.Sourceentry.Dbid, &s.Sourceentry.DontSearch, &s.Sourceentry.DontUpgrade, &s.Sourceentry.Listname, &s.Sourceentry.Quality, &s.Sourceentry.Info.M.Year, &s.Sourceentry.Info.M.Imdb, &s.Sourceentry.WantedTitle)
	if s.Sourceentry.Dbid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	if s.Sourceentry.DontSearch {
		return logger.ErrDisabled
	}

	s.Sourceentry.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, &movieid, false, -1, s.Quality)
	var err error
	s.searchActionType, err = getsearchtype(s.Sourceentry.MinimumPriority, s.Sourceentry.DontUpgrade, false)
	if err != nil {
		return err
	}

	if s.Sourceentry.Info.M.Year == 0 {
		return errors.New("year for movie not found")
	}
	s.Sourceentry.Info.M.ListID = database.GetMediaListID(s.Cfgp, s.Sourceentry.Listname)
	if s.Sourceentry.Quality == "" {
		s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
	}
	s.Quality = config.SettingsQuality[s.Sourceentry.Quality]
	return nil
}

// EpisodeFillSearchVar fills the search variables for the given episode ID.
// It queries the database to get the necessary data and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) EpisodeFillSearchVar(episodeid uint) error {
	s.Sourceentry.Close()
	s.Sourceentry.NzbepisodeID = episodeid

	//dbserie_episode_id, dbserie_id, serie_id, dont_search, dont_upgrade
	database.GetdatarowArgs("select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?", &episodeid, &s.Sourceentry.Info.M.DbserieEpisodeID, &s.Sourceentry.Dbid, &s.Sourceentry.Info.M.SerieID, &s.Sourceentry.DontSearch, &s.Sourceentry.DontUpgrade, &s.Sourceentry.Quality, &s.Sourceentry.Listname, &s.Sourceentry.NZB.TVDBID, &s.Sourceentry.WantedTitle, &s.Sourceentry.NZB.Season, &s.Sourceentry.NZB.Episode, &s.Sourceentry.Info.M.Identifier)
	if s.Sourceentry.Info.M.DbserieEpisodeID == 0 || s.Sourceentry.Dbid == 0 || s.Sourceentry.Info.M.SerieID == 0 {
		return logger.ErrNotFoundDbserie
	}
	if s.Sourceentry.DontSearch {
		return logger.ErrDisabled
	}

	s.Sourceentry.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, &episodeid, false, -1, s.Quality)

	s.Sourceentry.Info.M.ListID = database.GetMediaListID(s.Cfgp, s.Sourceentry.Listname)
	var err error
	s.searchActionType, err = getsearchtype(s.Sourceentry.MinimumPriority, s.Sourceentry.DontUpgrade, false)
	if err != nil {
		return err
	}
	str := s.Sourceentry.Quality
	if s.Sourceentry.Quality == "" {
		str = s.Cfgp.DefaultQuality
	}
	qual, ok := config.SettingsQuality[str]
	if !ok {
		return nil
	}
	s.Quality = qual
	return nil
}

// Getsearchalternatetitles returns alternate search titles to use when searching for media.
// It takes in alternate titles from the database, a flag indicating if this is already a title search,
// and the quality configuration. It returns a slice of string alternate titles.
func (s *ConfigSearcher) Getsearchalternatetitles(sourcealttitles []database.DbstaticTwoStringOneInt, titlesearch bool, qualcfg *config.QualityConfig) []database.DbstaticTwoStringOneInt {
	var addentry database.DbstaticTwoStringOneInt
	if qualcfg.BackupSearchForAlternateTitle {
		if qualcfg.BackupSearchForTitle {
			a := len(sourcealttitles) + 1
			if !titlesearch {
				a++
			}
			alttitles := make([]database.DbstaticTwoStringOneInt, 0, a)
			if !titlesearch {
				addentry.Str1 = s.Sourceentry.WantedTitle
				alttitles = append(alttitles, addentry)
			}
			addentry.Str1 = ""
			alttitles = append(alttitles, addentry)
			alttitles = append(alttitles, sourcealttitles...)
			return alttitles
		}
		if titlesearch && !qualcfg.BackupSearchForTitle {
			return sourcealttitles
		}
	}
	if qualcfg.BackupSearchForTitle {
		addentry.Str1 = s.Sourceentry.WantedTitle
		if !titlesearch {
			return []database.DbstaticTwoStringOneInt{{Str1: ""}, addentry}
		}
		return []database.DbstaticTwoStringOneInt{addentry}
	}
	if !titlesearch {
		return []database.DbstaticTwoStringOneInt{addentry}
	}
	addentry.Str1 = s.Sourceentry.WantedTitle
	return []database.DbstaticTwoStringOneInt{addentry}
}

// filterSizeNzbs checks if the NZB entry size is within the configured
// minimum and maximum size limits, and returns true if it should be
// rejected based on its size.
func (s *ConfigSearcher) filterSizeNzbs(entry *apiexternal.Nzbwithprio) bool {
	for idxdataimport := range s.Cfgp.DataImport {
		if s.Cfgp.DataImport[idxdataimport].CfgPath == nil {
			continue
		}
		if s.Cfgp.DataImport[idxdataimport].CfgPath.MinSize != 0 && entry.NZB.Size < s.Cfgp.DataImport[idxdataimport].CfgPath.MinSizeByte {
			s.logdenied(strtoosmall, entry)
			return true
		}

		if s.Cfgp.DataImport[idxdataimport].CfgPath.MaxSize != 0 && entry.NZB.Size > s.Cfgp.DataImport[idxdataimport].CfgPath.MaxSizeByte {
			s.logdenied(strtoobig, entry)
			return true
		}
	}
	return false
}

// filterRegexNzbs checks if the given NZB entry matches the required regexes
// and does not match any rejected regexes from the quality configuration.
// Returns true if the entry fails the regex checks, false if it passes.
func (s *ConfigSearcher) filterRegexNzbs(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	regexcfg := getregexcfg(entry, qual)
	if regexcfg == nil {
		s.logdenied(deniedbyregex, entry, logger.NewLogField("", "regex_template empty"))
		return true
	}

	var bl bool
	for idx := range regexcfg.Required {
		if database.RegexGetMatchesFind(regexcfg.Required[idx], entry.NZB.Title, 1) {
			bl = true
			break
		}
	}
	if !bl && regexcfg.RequiredLen >= 1 {
		s.logdenied(strnotmatchrequired, entry)
		return true
	}

	for idx := range regexcfg.Rejected {
		if database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.NZB.Title, 1) {
			if !database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.WantedTitle, 1) {
				bl = false
				for idxi := range entry.WantedAlternates {
					if database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.WantedAlternates[idxi].Str1, 1) {
						bl = true
						break
					}
				}
				if !bl {
					s.logdenied(deniedbyregex, entry, logger.NewLogField("rejected by", regexcfg.Rejected[idx]))
					return true
				}
			}
		}
	}
	return false
}

// checkhistory checks if the given entry is already in the history cache
// to avoid duplicate downloads. It checks based on the download URL and title.
// Returns true if a duplicate is found, false otherwise.
func (s *ConfigSearcher) checkhistory(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	if entry.NZB.DownloadURL != "" {
		if database.CheckcachedUrlHistory(s.Cfgp.Useseries, entry.NZB.DownloadURL) {
			s.logdenied(strallreadydlurl, entry)
			return true
		}
	}
	if entry.NZB.Indexer == nil {
		return false
	}
	indcfg := qualityIndexerByQualityAndTemplate(qual, entry.NZB.Indexer)
	if indcfg != nil {
		if !indcfg.HistoryCheckTitle {
			return false
		}
	}

	if database.CheckcachedTitleHistory(s.Cfgp.Useseries, entry.NZB.Title) {
		s.logdenied(strallreadydltitle, entry)
		return true
	}
	return false
}

// checkprocessed checks if the given entry is already in the denied or accepted lists to avoid duplicate processing.
// It loops through the denied and accepted entries and returns true if it finds a match on the download URL or title.
// Otherwise returns false. Part of ConfigSearcher.
func (s *ConfigSearcher) checkprocessed(entry *apiexternal.Nzbwithprio) bool {
	for idx := range s.Dl.Denied {
		if s.Dl.Denied[idx].NZB.DownloadURL == entry.NZB.DownloadURL {
			return true
		}
		if s.Dl.Denied[idx].NZB.Title == entry.NZB.Title {
			return true
		}
	}
	for idx := range s.Dl.Accepted {
		if s.Dl.Accepted[idx].NZB.DownloadURL == entry.NZB.DownloadURL {
			return true
		}
		if s.Dl.Accepted[idx].NZB.Title == entry.NZB.Title {
			return true
		}
	}
	return false
}

// logdenied logs a debug message indicating the entry was denied for the given reason.
// It appends the entry to the denied list in s.Dl.Denied.
// If addfields are provided, it will log them along with the reason and title.
// It will also set the AdditionalReason field on the entry based on the first addfield.
func (s *ConfigSearcher) logdenied(reason string, entry *apiexternal.Nzbwithprio, addfields ...logger.LogField) {
	if reason != "" {
		if len(addfields) > 0 {
			logger.LogDynamicSlice("debug", skippedstr, addfields, logger.NewLogField("reason", &reason), logger.NewLogField("title", &entry.NZB.Title))
			switch v := addfields[0].Value.(type) {
			case int:
				entry.AdditionalReason = strconv.Itoa(v)
			case *int:
				entry.AdditionalReason = strconv.Itoa(*v)
			case string:
				entry.AdditionalReason = v
			case *string:
				entry.AdditionalReason = *v
			default:
				entry.AdditionalReason = fmt.Sprintf("%v", v)
			}
		} else {
			logger.LogDynamic("debug", skippedstr, logger.NewLogField("reason", &reason), logger.NewLogField("title", &entry.NZB.Title))
		}
		entry.Reason = reason
	}
	s.AppendDenied(entry)
}

// logdeniederr logs a denied entry with the given error and optional title.
// It sets the reason on the entry to the error message, and appends the entry to s.Dl.Denied.
func (s *ConfigSearcher) logdeniederr(err error, entry *apiexternal.Nzbwithprio) {
	if err != nil {
		logger.LogDynamic("debug", skippedstr, logger.NewLogFieldValue(err), logger.NewLogField(logger.StrTitle, &entry.NZB.Title))
		entry.Reason = err.Error()
	}
	s.AppendDenied(entry)
}

// Getpriobyfiles returns the minimum priority of existing files for the given media
// ID, and optionally returns a slice of file paths for existing files below
// the given wanted priority. If useseries is true it will look up series IDs instead of media IDs.
// If id is nil it will return 0 priority.
// If useall is true it will include files marked as deleted.
// If wantedprio is -1 it will not return any file paths.
func Getpriobyfiles(useseries bool, id *uint, useall bool, wantedprio int, qualcfg *config.QualityConfig) (int, []string) {
	if qualcfg == nil || id == nil {
		return 0, nil
	}
	var minPrio int
	var oldfiles []string
	countv := database.GetdatarowN[int](false, logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID), id)
	if wantedprio != -1 && countv != 0 {
		oldfiles = make([]string, 0, countv)
	}
	arr := database.GetrowsN[database.FilePrio](false, countv, logger.GetStringsMap(useseries, logger.DBFilePrioFilesByID), id)
	var prio int
	for idx := range arr {
		prio = parser.GetIDPrioritySimpleParse(&arr[idx], useseries, qualcfg, useall)

		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
		if wantedprio != -1 && wantedprio > prio {
			oldfiles = append(oldfiles, arr[idx].Location)
		}
	}
	clear(arr)
	if wantedprio == -1 {
		return minPrio, nil
	}
	return minPrio, oldfiles
}
