package searcher

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/worker"
)

type SearchResults struct {
	Raw      []apiexternal.NZB
	Denied   []apiexternal.Nzbwithprio
	Accepted []apiexternal.Nzbwithprio
	//Rejected []*apiexternal.Nzbwithprio
	//mu *sync.Mutex
	//Searched []string
}
type Searcher struct {
	Cfgpstr          string
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss
	MinimumPriority  int
	//Movie                database.Movie
	//SerieEpisode         database.SerieEpisode
	imdb         string
	year         int
	title        string
	identifier   string
	season       string
	episode      string
	thetvdbid    int
	ListTemplate string

	MediaQualityTemplate string
	MediaDBID            uint
	MediaID              uint
	MediaMainDBID        uint
	MediaMainID          uint
	DontUpgrade          bool
	DontSearch           bool
	//Listcfg          config.ListsConfig
	AlternateTitles []string
}

const (
	notwantedmovie = "Not Wanted Movie"
	skippedindexer = "Skipped Indexer"
	deniedbyregex  = "Denied by Regex"
	skippedstr     = "Skipped"
)

func SearchSerieRSSSeasonSingle(serieid uint, season int, useseason bool, cfgpstr string) (*SearchResults, error) {
	qualstr := database.QueryStringColumn(database.QuerySerieEpisodesGetQualityBySerieID, serieid)
	if qualstr == "" {
		return nil, errors.New("quality missing")
	}
	tvdb := database.QueryUintColumn(database.QueryDbseriesGetTvdbByID, database.QueryUintColumn(database.QuerySeriesGetDBIDByID, serieid))
	if tvdb == 0 {
		return nil, errors.New("tvdb missing")
	}
	results, err := SearchSeriesRSSSeason(cfgpstr, qualstr, logger.StrSeries, int(tvdb), season, useseason)
	if err != nil {
		return nil, err
	}
	if results == nil || len(results.Accepted) == 0 {
		results.Close()
		return nil, nil
	}
	results.Download(logger.StrSeries, cfgpstr)
	return results, nil
	//return SearchMyMedia(cfgpstr, qualstr, logger.StrRssSeasons, logger.StrSeries, int(tvdb), season, useseason, 0, false)
}
func SearchSeriesRSSSeasons(cfgpstr string) {
	if len(cfgpstr) == 0 {
		return
	}

	queryseason := "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )"
	tbl := database.QueryStaticColumnsTwoInt(false, 20, "select id, dbserie_id from series where listname in ("+logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists)-1)+") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20", config.SettingsMedia[cfgpstr].ListsInt...)
	var tblseasons *[]string
	var results *SearchResults
	var err error
	for idx := range *tbl {
		tblseasons = database.QueryStaticStringArray(false, 5, queryseason, (*tbl)[idx].Num2, (*tbl)[idx].Num1, (*tbl)[idx].Num1, (*tbl)[idx].Num2)
		for idxseason := range *tblseasons {
			results, err = SearchSerieRSSSeasonSingle(uint((*tbl)[idx].Num1), logger.StringToInt((*tblseasons)[idxseason]), true, cfgpstr)
			if err != nil && err != logger.ErrDisabled {
				//logger.LogAnyError(err, "Season Search Failed", logger.LoggerValue{Name: "id", Value: (*tbl)[idx].Num1})
				logger.Log.Error().Err(err).Int("ID", (*tbl)[idx].Num1).Msg("Season Search Inc Failed")
			}
			results.Close()
		}
		logger.Clear(tblseasons)
	}
	logger.Clear(tbl)
}
func SearchSeriesRSSSeasonsAll(cfgpstr string) {
	if len(cfgpstr) == 0 {
		return
	}

	queryseason := "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )"

	tbl := database.QueryStaticColumnsTwoInt(false, database.QueryCountColumn("series", ""), "select id, dbserie_id from series where listname in ("+logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists)-1)+") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1", config.SettingsMedia[cfgpstr].ListsInt...)
	var tblseasons *[]string
	var results *SearchResults
	var err error
	for idx := range *tbl {
		tblseasons = database.QueryStaticStringArray(false, 5, queryseason, (*tbl)[idx].Num2, (*tbl)[idx].Num1, (*tbl)[idx].Num1, (*tbl)[idx].Num2)
		for idxseason := range *tblseasons {
			results, err = SearchSerieRSSSeasonSingle(uint((*tbl)[idx].Num1), logger.StringToInt((*tblseasons)[idxseason]), true, cfgpstr)
			if err != nil && err != logger.ErrDisabled {
				//logger.LogAnyError(err, "Season Search Failed", logger.LoggerValue{Name: "id", Value: (*tbl)[idx].Num1})
				logger.Log.Error().Err(err).Int("ID", (*tbl)[idx].Num1).Msg("Season Search Failed")
			}
			results.Close()
		}
		logger.Clear(tblseasons)
	}
	logger.Clear(tbl)
}

func (s *SearchResults) Download(mediatype string, cfgpstr string) {

	var downloaded []uint
	for idx := range s.Accepted {
		var breakv bool
		for idxi := range downloaded {
			if s.Accepted[idx].NzbmovieID != 0 && s.Accepted[idx].NzbmovieID == downloaded[idxi] {
				breakv = true
				break
			}
			if s.Accepted[idx].NzbepisodeID != 0 && s.Accepted[idx].NzbepisodeID == downloaded[idxi] {
				breakv = true
				break
			}
		}
		if s.Accepted[idx].NzbmovieID != 0 && breakv {
			//logger.LogerrorStr(nil, "skip already started", logger.IntToString(int(s.Accepted[idx].NzbmovieID)), "")
			logger.Log.Error().Err(nil).Str("skip already started", logger.IntToString(int(s.Accepted[idx].NzbmovieID))).Msg("already started")
			break
		}
		if s.Accepted[idx].NzbepisodeID != 0 && breakv {
			//logger.LogerrorStr(nil, "skip already started", logger.IntToString(int(s.Accepted[idx].NzbepisodeID)), "")
			logger.Log.Error().Err(nil).Str("skip already started", logger.IntToString(int(s.Accepted[idx].NzbepisodeID))).Msg("already started")
			break
		}
		logger.Log.Debug().Str("typ", mediatype).Str(logger.StrTitle, s.Accepted[idx].NZB.Title).Str("quality", s.Accepted[idx].QualityTemplate).Int("minimum prio", s.Accepted[idx].MinimumPriority).Int(logger.StrPriority, s.Accepted[idx].ParseInfo.M.Priority).Msg("nzb found - start downloading")

		if mediatype == logger.StrMovie {
			downloaded = append(downloaded, s.Accepted[idx].NzbmovieID)
			downloader.DownloadMovie(cfgpstr, s.Accepted[idx].NzbmovieID, &s.Accepted[idx])
		} else {
			downloaded = append(downloaded, s.Accepted[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(cfgpstr, s.Accepted[idx].NzbepisodeID, &s.Accepted[idx])
		}
	}
	logger.Clear(&downloaded)
}

func (s *SearchResults) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}

	for idx := range s.Denied {
		s.Denied[idx].Close()
	}
	for idx := range s.Accepted {
		s.Accepted[idx].Close()
	}
	for idx := range s.Raw {
		s.Raw[idx].Close()
	}
	logger.Clear(&s.Denied)
	logger.Clear(&s.Accepted)
	logger.Clear(&s.Raw)
	logger.ClearVar(s)
}

func (s *Searcher) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.AlternateTitles)
	logger.ClearVar(s)
}

func searchnow(search *Searcher, dl *SearchResults, quality string, updatelastsql string, updatelastid uint, checkrss bool, searchfun func(indexer string) (bool, error)) (*SearchResults, error) {

	//dl := new(SearchResults)
	workergroup := worker.WorkerPoolIndexer.Group()

	var processedindexer int
	var added int
	for index := range config.SettingsQuality["quality_"+quality].Indexer {
		if checkrss && !config.SettingsIndexer["indexer_"+config.SettingsQuality["quality_"+quality].Indexer[index].TemplateIndexer].Rssenabled {
			continue
		}
		if !checkrss && !config.SettingsIndexer["indexer_"+config.SettingsQuality["quality_"+quality].Indexer[index].TemplateIndexer].Enabled {
			continue
		}
		ind := config.SettingsQuality["quality_"+quality].Indexer[index].TemplateIndexer
		added++
		workergroup.Submit(func() {
			ok, err := searchfun(ind)
			if ok {
				processedindexer++
			}
			if err != nil && err != logger.ErrDisabled {
				//logger.LogAnyError(err, "Error searching rss indexer", logger.LoggerValue{Name: "indexer", Value: ind})
				logger.Log.Error().Err(err).Str("indexer", ind).Msg("Error searching indexer")
			}
		})
	}
	if added >= 1 {
		workergroup.Wait()
	}
	//dl := s.queryindexers(s.Quality, logger.StrRss, fetchall, &processedindexer, false, queryparams{})
	defer search.Close()
	if processedindexer == 0 && len(config.SettingsQuality["quality_"+quality].Indexer) >= 1 {
		blockinterval := 5
		if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
			blockinterval = 1 * config.SettingsGeneral.FailedIndexerBlockTime
		}
		config.Slepping(false, 60*blockinterval)
	}
	if processedindexer == 0 {
		return nil, nil
	}
	if processedindexer >= 1 && len(updatelastsql) >= 1 {
		database.UpdateColumnStatic(updatelastsql, database.SQLTimeGetNow(), &updatelastid)
	}

	if len(dl.Raw) > 1 {
		search.parseentries(dl, "", false)
		if updatelastid != 0 {
			logger.Log.Debug().Uint(logger.StrID, updatelastid).Int("Accepted", len(dl.Accepted)).Int("Denied", len(dl.Denied)).Msg("Ended Search for media id")
		}
		if database.DBLogLevel == logger.StrDebug {
			logger.Log.Debug().Int("Count", len(dl.Raw)).Msg("Entries found")
		}
		if len(dl.Accepted) > 1 {
			sort.Slice(dl.Accepted, func(i, j int) bool {
				return dl.Accepted[i].Prio > dl.Accepted[j].Prio
			})
		}
	}
	return dl, nil
}

// searchGroupType == movie || series
func SearchRSS(cfgpstr string, quality string, searchGroupType string, fetchall bool) (*SearchResults, error) {
	if cfgpstr == "" {
		return nil, logger.ErrCfgpNotFound
	}
	if !config.CheckGroup("quality_", quality) {
		return nil, errors.New("quality template not found")
	}
	if len(config.SettingsQuality["quality_"+quality].Indexer) == 0 {
		return nil, errors.New("indexer for RSS not found")
	}

	searchvar := NewSearcher(cfgpstr, quality, searchGroupType, logger.StrRss, 0)

	var dl SearchResults
	defer searchvar.Close()

	return searchnow(searchvar, &dl, quality, "", 0, true, func(ind string) (bool, error) { return searchvar.rsssearchindexer(quality, ind, fetchall, &dl) })
}

func (s *Searcher) rsssearchindexer(quality string, indexer string, fetchall bool, dl *SearchResults) (bool, error) {
	// if addsearched(dl, search.indexer+search.quality) {
	// 	return true
	// }
	nzbindexer, cats, maxloop, maxentries, erri := s.initIndexer(quality, indexer, logger.StrRss)
	if erri != nil {
		if erri == logger.ErrDisabled || erri == logger.ErrNotFound {
			// nzbindexer.Close()
			return true, erri
		}
		if erri == logger.ErrToWait {
			//time.Sleep(10 * time.Second)
			// nzbindexer.Close()
			return true, erri
		}

		// nzbindexer.Close()
		return false, erri
	}
	//defer logger.Close(&nzbindexer)

	if fetchall {
		nzbindexer.LastRssID = ""
	}
	if maxentries == 0 {
		maxentries = 10
	}
	if maxloop == 0 {
		maxloop = 2
	}

	addnzbs(dl, querynzbs(s.Cfgpstr, "rsslast", indexer, quality, nzbindexer, cats, "", 0, 0, 0, false, false, maxentries, maxloop))

	logger.ClearVar(nzbindexer)
	return true, nil
}

func querynzbs(cfgpstr string, searchtype string, indexer string, quality string, nzbindexer *apiexternal.NzbIndexer, cats string, title string, thetvdbid int, season int, episode int, useseason bool, useepisode bool, maxentries int, maxloop int) *[]apiexternal.NZB {
	switch searchtype {
	case "tvdb":
		return querynzbsret(apiexternal.QueryNewznabTvTvdb(indexer, quality, nzbindexer, thetvdbid, cats, season, episode, useseason, useepisode))
	case "query":
		return querynzbsret(apiexternal.QueryNewznabQuery(indexer, quality, nzbindexer, title, cats, "search"))
	case "imdb":
		return querynzbsret(apiexternal.QueryNewznabMovieImdb(indexer, quality, nzbindexer, title, cats))
	case "rsslast":
		n := querynzbsret(apiexternal.QueryNewznabRSSLast(indexer, quality, nzbindexer, maxentries, cats, maxloop))
		if n != nil && len(*n) >= 1 && (*n)[0].ID != "" {
			addrsshistory(nzbindexer.URL, (*n)[0].ID, quality, cfgpstr)
		}
		return n
	}
	return nil
}
func querynzbsret(n *[]apiexternal.NZB, err error) *[]apiexternal.NZB {
	if err != nil && err != logger.Errnoresults {
		logger.Clear(n)
	}
	return n
}

func addnzbs(dl *SearchResults, n *[]apiexternal.NZB) {
	if n != nil && len(*n) >= 1 {
		if len(dl.Raw) == 0 {
			dl.Raw = *n
		} else {
			logger.Grow(&dl.Raw, len(*n))
			dl.Raw = append(dl.Raw, *n...)
		}
	}
	logger.Clear(n)
}

func NewSearcher(cfgpstr string, quality string, searchGroupType string, searchActionType string, mediaid uint) *Searcher {
	return &Searcher{
		Cfgpstr:          cfgpstr,
		Quality:          quality,
		SearchGroupType:  searchGroupType,
		SearchActionType: searchActionType,
		MediaID:          mediaid,
	}
}

// searchGroupType == movie || series
func SearchSeriesRSSSeason(cfgpstr string, quality string, searchGroupType string, thetvdbid int, season int, useseason bool) (*SearchResults, error) {
	if cfgpstr == "" {
		return nil, logger.ErrCfgpNotFound
	}
	if !config.CheckGroup("quality_", quality) {
		return nil, errors.New("quality for RSS not found")
	}
	if len(config.SettingsQuality["quality_"+quality].Indexer) == 0 {
		return nil, errors.New("indexer for RSS not found")
	}

	//var processedindexer int
	var dl SearchResults
	searchvar := NewSearcher(cfgpstr, quality, searchGroupType, logger.StrRss, 0)
	defer searchvar.Close()
	return searchnow(searchvar, &dl, quality, "", 0, false, func(ind string) (bool, error) {
		return searchvar.rssqueryseriesindexer(quality, ind, thetvdbid, season, useseason, &dl)
	})

}

func (s *Searcher) rssqueryseriesindexer(quality string, indexer string, thetvdbid int, season int, useseason bool, dl *SearchResults) (bool, error) {
	nzbindexer, cats, _, _, erri := s.initIndexer(quality, indexer, "api")
	if erri != nil {
		if erri == logger.ErrDisabled || erri == logger.ErrNotFound {
			return true, erri
		}
		if erri == logger.ErrToWait {
			//time.Sleep(10 * time.Second)
			return true, erri
		}
		return false, erri
	}
	//defer logger.Close(&nzbindexer)

	addnzbs(dl, querynzbs(s.Cfgpstr, "tvdb", indexer, quality, nzbindexer, cats, "", thetvdbid, season, 0, useseason, false, 0, 0))
	logger.ClearVar(nzbindexer)
	return true, nil
}

func (s *Searcher) checkcorrectid(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {

	if s.SearchActionType == logger.StrRss {
		return false
	}
	if s.SearchGroupType == logger.StrMovie && entry.NZB.IMDBID != "" && s.imdb != "" {
		//Check Correct Imdb
		if s.imdb != entry.NZB.IMDBID {
			if logger.HasPrefixI(s.imdb, logger.StrTt) == logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
				logdenied("Imdb not match", "", entry, dl)
				return true
			}
			if strings.TrimLeft(s.imdb, "t0") != strings.TrimLeft(entry.NZB.IMDBID, "t0") {
				logdenied("Imdb not match", "", entry, dl)
				return true
			}
		}
		return false
	}
	if s.SearchGroupType != logger.StrSeries {
		return false
	}
	if entry.NZB.TVDBID != 0 && s.thetvdbid >= 1 && s.thetvdbid != entry.NZB.TVDBID {
		logdenied("Tvdbid not match", "", entry, dl)
		return true
	}
	return false
}
func (s *Searcher) getmediadata(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {

	if s.SearchGroupType == logger.StrMovie {
		entry.NzbmovieID = s.MediaID
		entry.Dbid = s.MediaDBID
		entry.QualityTemplate = s.MediaQualityTemplate
		entry.MinimumPriority = s.MinimumPriority
		if s.MinimumPriority == 0 && s.Cfgpstr != "" {
			entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, entry.QualityTemplate)
		}
		entry.ParseInfo.M.MovieID = s.MediaID
		entry.ParseInfo.M.DbmovieID = s.MediaDBID
		entry.WantedTitle = s.title
		entry.WantedAlternates = s.AlternateTitles

		//Check QualityProfile
		if !config.CheckGroup("quality_", entry.QualityTemplate) {
			logdenied("Unknown quality", entry.QualityTemplate+"_"+logger.UintToString(entry.NzbmovieID), entry, dl)
			return true
		}

		return false
	}

	//Parse Series
	entry.NzbepisodeID = s.MediaID
	entry.Dbid = s.MediaMainDBID
	entry.QualityTemplate = s.MediaQualityTemplate
	entry.MinimumPriority = s.MinimumPriority
	entry.ParseInfo.M.SerieEpisodeID = s.MediaID
	if entry.ParseInfo.M.SerieEpisodeID == 0 {
		logdenied("Unwanted Episode", "", entry, dl)
		return true
	}
	if s.MinimumPriority == 0 && s.Cfgpstr != "" {
		entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, entry.QualityTemplate)
	}
	entry.ParseInfo.M.DbserieID = s.MediaMainDBID
	entry.ParseInfo.M.DbserieEpisodeID = s.MediaDBID
	entry.ParseInfo.M.SerieID = s.MediaMainID
	entry.WantedTitle = s.title
	entry.WantedAlternates = s.AlternateTitles

	//Check Quality Profile
	if !config.CheckGroup("quality_", entry.QualityTemplate) {
		logdenied("Unknown Quality Profile", entry.QualityTemplate+"_"+logger.UintToString(entry.ParseInfo.M.SerieEpisodeID), entry, dl)
		return true
	}
	return false
}
func (s *Searcher) getmediadatarss(entry *apiexternal.Nzbwithprio, dl *SearchResults, addinlist string, addifnotfound bool) bool {
	if s.SearchGroupType == logger.StrMovie {

		i := config.GetMediaListsEntryIndex(s.Cfgpstr, addinlist)
		var templatelist string
		if i != -1 {
			templatelist = config.SettingsMedia[s.Cfgpstr].Lists[i].TemplateList
		}
		if addinlist != "" && !strings.EqualFold(s.ListTemplate, templatelist) && templatelist != "" {
			s.ListTemplate = templatelist
		}
		if s.getmovierss(entry, addinlist, addifnotfound, dl) {
			return true
		}
		//Check Minimal Priority
		entry.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, entry.NzbmovieID, entry.QualityTemplate)

		if entry.MinimumPriority != 0 {
			if s.DontUpgrade {
				logdenied("Upgrade disabled", "", entry, dl)
				return true
			}
		} else {
			if s.DontSearch {
				logdenied("Searxh disabled", "", entry, dl)
				return true
			}
		}

		//Check QualityProfile
		if !config.CheckGroup("quality_", entry.QualityTemplate) {
			logdenied("Unknown quality", entry.QualityTemplate+"_"+logger.UintToString(entry.NzbmovieID), entry, dl)
			return true
		}

		return false
	}

	//Parse Series
	//Filter RSS Series
	if s.getserierss(entry, dl) {
		return true
	}
	if !strings.EqualFold(s.Quality, entry.QualityTemplate) && entry.QualityTemplate != "" {
		s.Quality = entry.QualityTemplate
	}
	entry.WantedTitle = s.title

	//Check Minimum Priority
	entry.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, entry.NzbepisodeID, entry.QualityTemplate)

	if entry.MinimumPriority != 0 {
		if s.DontUpgrade {
			logdenied("Upgrade disabled", "", entry, dl)
			return true
		}
	} else {
		if s.DontSearch {
			logdenied("Search disabled", "", entry, dl)
			return true
		}
	}

	//Check Quality Profile
	if !config.CheckGroup("quality_", entry.QualityTemplate) {
		logdenied("Unknown Quality Profile", entry.QualityTemplate+"_"+logger.UintToString(entry.ParseInfo.M.SerieEpisodeID), entry, dl)
		return true
	}
	return false
}
func (s *Searcher) checkyear(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	if s.SearchGroupType != logger.StrMovie || s.SearchActionType == logger.StrRss {
		return false
	}
	if s.year == 0 {
		logdenied("No Year", "", entry, dl)
		return true
	}

	if (config.SettingsQuality["quality_"+entry.QualityTemplate].CheckYear || config.SettingsQuality["quality_"+entry.QualityTemplate].CheckYear1) && logger.ContainsI(entry.NZB.Title, logger.IntToString(s.year)) {
		return false
	}
	if config.SettingsQuality["quality_"+entry.QualityTemplate].CheckYear1 && logger.ContainsI(entry.NZB.Title, logger.IntToString(s.year+1)) {
		return false
	}
	if config.SettingsQuality["quality_"+entry.QualityTemplate].CheckYear1 && logger.ContainsI(entry.NZB.Title, logger.IntToString(s.year-1)) {
		return false
	}
	logdenied("Unwanted Year", "", entry, dl)
	return true
}
func (s *Searcher) checktitle(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	//Checktitle
	if !config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle {
		return false
	}
	err := importfeed.StripTitlePrefixPostfixGetQual(entry.ParseInfo.M.Title, entry.QualityTemplate)
	if err != nil {
		logger.Logerror(err, "Strip Failed")
	}
	addstr := fmt.Sprintf(" %d", entry.ParseInfo.M.Year) //" " + logger.IntToString(entry.ParseInfo.M.Year)
	titlechk := entry.ParseInfo.M.Title + addstr
	//title := entry.ParseInfo.M.Title
	wantedslug := logger.StringToSlug(entry.WantedTitle)
	if entry.WantedTitle != "" {
		if config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && len(entry.WantedTitle) >= 1 && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, entry.ParseInfo.M.Title) {
			return false
		}
		if entry.ParseInfo.M.Year != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && len(entry.WantedTitle) >= 1 && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, titlechk) {
			return false
		}
	}
	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.ParseInfo.M.Title, ']') {
		u := logger.IndexILast(entry.ParseInfo.M.Title, "]")
		if u != -1 && u < (len(entry.ParseInfo.M.Title)-1) {
			trytitle = strings.TrimLeft(entry.ParseInfo.M.Title[u+1:], "-. ")
			if config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && len(entry.WantedTitle) >= 1 && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle) {
				return false
			}
			if entry.ParseInfo.M.Year != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && len(entry.WantedTitle) >= 1 && apiexternal.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle+addstr) {
				return false
			}
		}
	}
	if s.SearchGroupType == logger.StrSeries && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArrayObj(&entry.WantedAlternates, false,
			database.QueryDbserieAlternatesGetTitleByDBIDNoEmpty, entry.Dbid)
	} else if s.SearchGroupType == logger.StrMovie && len(entry.WantedAlternates) == 0 && entry.Dbid != 0 {
		database.QueryStaticStringArrayObj(&entry.WantedAlternates, false,
			database.QueryDbmovieTitlesGetTitleByIDNoEmpty, entry.Dbid)
	}
	if len(entry.WantedAlternates) == 0 || entry.ParseInfo.M.Title == "" {
		logdenied("Unwanted Title", "", entry, dl)
		return true
	}
	for idxtitle := range entry.WantedAlternates {
		if entry.WantedAlternates[idxtitle] == "" {
			continue
		}

		wantedslug = logger.StringToSlug(entry.WantedAlternates[idxtitle])
		if apiexternal.ChecknzbtitleB(entry.WantedAlternates[idxtitle], wantedslug, entry.ParseInfo.M.Title) {
			return false
		}

		if entry.ParseInfo.M.Year != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && apiexternal.ChecknzbtitleB(entry.WantedAlternates[idxtitle], wantedslug, titlechk) {
			return false
		}

		if trytitle != "" {
			if apiexternal.ChecknzbtitleB(entry.WantedAlternates[idxtitle], wantedslug, trytitle) {
				return false
			}
			if entry.ParseInfo.M.Year != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].CheckTitle && apiexternal.ChecknzbtitleB(entry.WantedAlternates[idxtitle], wantedslug, trytitle+addstr) {
				return false
			}
		}
	}
	logdenied("Unwanted Title and Alternate", "", entry, dl)
	return true
}

func (s *Searcher) getseasonepisodearray() (*[]string, *[]string) {
	if logger.ContainsI(s.identifier, "s") && logger.ContainsI(s.identifier, "e") {
		return &[]string{"s" + s.season + "e", "s0" + s.season + "e", "s" + s.season + " e", "s0" + s.season + " e"}, &[]string{"e" + s.episode, "e0" + s.episode}
	} else if logger.ContainsI(s.identifier, "x") {
		return &[]string{s.season + "x", s.season + " x"}, &[]string{"x" + s.episode, "x0" + s.episode}
	}
	return nil, nil
}

func (s *Searcher) checkepisode(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {

	//Checkepisode
	if s.SearchGroupType == logger.StrMovie {
		return false
	}
	if s.identifier == "" {
		logdenied("No Identifier", "", entry, dl)
		return true
	}

	// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
	altIdentifier := strings.TrimLeft(s.identifier, "sS0")
	logger.StringReplaceRuneP(&altIdentifier, 'e', "x")
	logger.StringReplaceRuneP(&altIdentifier, 'E', "x")
	if logger.ContainsI(entry.NZB.Title, s.identifier) ||
		logger.ContainsI(entry.NZB.Title, altIdentifier) {
		return false
	}
	if strings.ContainsRune(s.identifier, '-') {
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceRuneS(s.identifier, '-', ".")) ||
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceRuneS(altIdentifier, '-', ".")) {
			return false
		}
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceRuneS(s.identifier, '-', " ")) ||
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceRuneS(altIdentifier, '-', " ")) {
			return false
		}
	}

	//lowerParseIdentifier := s.loweridentifier
	//if entry.ParseInfo.M.Identifier != s.identifier {
	//	lowerParseIdentifier = strings.ToLower(entry.ParseInfo.M.Identifier)
	//}

	if s.season == "" || s.episode == "" {
		logdenied("Unwanted Identifier", s.identifier, entry, dl)
		return true
	}
	//var matchfound bool
	//seasonarray, episodearray := s.getseasonepisodearray()

	var sprefix, eprefix string
	if logger.ContainsI(s.identifier, "s") && logger.ContainsI(s.identifier, "e") {
		sprefix = "s"
		eprefix = "e"
	} else if logger.ContainsI(s.identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else {
		logdenied("Unwanted Identifier", s.identifier, entry, dl)
		return true
	}

	if !logger.HasPrefixI(s.identifier, sprefix+s.season) {
		logdenied("Unwanted Season", s.identifier, entry, dl)
		return true
	}
	if logger.HasSuffixI(s.identifier, eprefix+s.episode) {
		return false
	}
	if logger.HasSuffixI(s.identifier, eprefix+"0"+s.episode) {
		return false
	}
	if logger.HasSuffixI(s.identifier, eprefix+" "+s.episode) {
		return false
	}
	if logger.HasSuffixI(s.identifier, eprefix+" 0"+s.episode) {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+s.episode+" ") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+s.episode+"-") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+s.episode+eprefix) {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" "+s.episode+" ") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" "+s.episode+"-") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" "+s.episode+eprefix) {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+"0"+s.episode+" ") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+"0"+s.episode+"-") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+"0"+s.episode+eprefix) {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" 0"+s.episode+" ") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" 0"+s.episode+"-") {
		return false
	}
	if logger.ContainsI(s.identifier, eprefix+" 0"+s.episode+eprefix) {
		return false
	}

	logdenied("Unwanted Identifier", s.identifier, entry, dl)
	return true
}

func filterTestQualityWanted(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	var wanted bool
	lenqual := len(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedResolution)
	if lenqual >= 1 && entry.ParseInfo.M.Resolution != "" {
		for idx := range config.SettingsQuality["quality_"+entry.QualityTemplate].WantedResolution {
			if strings.EqualFold(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedResolution[idx], entry.ParseInfo.M.Resolution) {
				wanted = true
				break
			}
		}
	}

	if lenqual >= 1 && !wanted {
		logdenied("Unwanted Resolution", entry.ParseInfo.M.Resolution, entry, dl)
		return true
	}
	wanted = false

	lenqual = len(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedQuality)
	if lenqual >= 1 && entry.ParseInfo.M.Quality != "" {
		for idx := range config.SettingsQuality["quality_"+entry.QualityTemplate].WantedQuality {
			if strings.EqualFold(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedQuality[idx], entry.ParseInfo.M.Quality) {
				wanted = true
				break
			}
		}
	}
	if lenqual >= 1 && !wanted {
		logdenied("Unwanted Quality", entry.ParseInfo.M.Quality, entry, dl)
		return true
	}

	wanted = false

	lenqual = len(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedAudio)
	if lenqual >= 1 && entry.ParseInfo.M.Audio != "" {
		for idx := range config.SettingsQuality["quality_"+entry.QualityTemplate].WantedAudio {
			if strings.EqualFold(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedAudio[idx], entry.ParseInfo.M.Audio) {
				wanted = true
				break
			}
		}
	}
	if lenqual >= 1 && !wanted {
		logdenied("Unwanted Audio", entry.ParseInfo.M.Audio, entry, dl)
		return true
	}
	wanted = false

	lenqual = len(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedCodec)
	if lenqual >= 1 && entry.ParseInfo.M.Codec != "" {
		for idx := range config.SettingsQuality["quality_"+entry.QualityTemplate].WantedCodec {
			if strings.EqualFold(config.SettingsQuality["quality_"+entry.QualityTemplate].WantedCodec[idx], entry.ParseInfo.M.Codec) {
				wanted = true
				break
			}
		}
	}
	if lenqual >= 1 && !wanted {
		logdenied("Unwanted Codec", entry.ParseInfo.M.Codec, entry, dl)
		return true
	}
	return false
}

func (s *Searcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlist string, addifnotfound bool, dl *SearchResults) bool {
	//Parse
	err := parser.GetDBIDs(&entry.ParseInfo.M, s.Cfgpstr, addinlist, true)

	s.MediaDBID = 0
	s.MediaID = 0
	s.MediaMainDBID = 0
	s.MediaMainID = 0
	s.DontSearch = false
	s.DontUpgrade = false
	s.MediaQualityTemplate = ""
	s.year = 0
	s.imdb = ""
	s.title = ""
	if err != nil {
		logdenied(err.Error(), "", entry, dl)
		return true
	}
	//Get DbMovie by imdbid

	//Add DbMovie if not found yet and enabled
	if entry.ParseInfo.M.DbmovieID == 0 && (!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
		logdenied("Unwanted DBMovie", "", entry, dl)
		return true
	}
	if entry.ParseInfo.M.DbmovieID == 0 && addifnotfound && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		ok, err := s.allowMovieImport(entry.NZB.IMDBID, addinlist)
		if err != nil {
			logdenied(err.Error(), "", entry, dl)
			return true
		}
		if !ok {
			logdenied("Unallowed DBMovie", "", entry, dl)
			return true
		}
		entry.ParseInfo.M.DbmovieID, err = importfeed.JobImportMovies(entry.NZB.IMDBID, s.Cfgpstr, addinlist, true)
		if err != nil {
			logdenied(err.Error(), "", entry, dl)
			return true
		}
		entry.ParseInfo.M.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, entry.ParseInfo.M.DbmovieID, &addinlist)
		if entry.ParseInfo.M.MovieID == 0 || entry.ParseInfo.M.DbmovieID == 0 {
			logdenied("Unwanted Movie", "", entry, dl)
			return true
		}
	}
	if entry.ParseInfo.M.DbmovieID == 0 {
		logdenied("Unwanted DBMovie", "", entry, dl)
		return true
	}

	//continue only if dbmovie found
	//Get List of movie by dbmovieid, year and possible lists

	//if list was not found : should we add the movie?
	if addifnotfound && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) && entry.ParseInfo.M.MovieID == 0 {
		ok, err := s.allowMovieImport(entry.NZB.IMDBID, addinlist)
		if err != nil {
			logdenied(err.Error(), "", entry, dl)
			return true
		}
		if !ok {
			logdenied("Unwanted Movie", "", entry, dl)
			return true
		}
		entry.ParseInfo.M.DbmovieID, err = importfeed.JobImportMovies(entry.NZB.IMDBID, s.Cfgpstr, addinlist, true)
		if err != nil {
			logdenied(err.Error(), "", entry, dl)
			return true
		}
		if entry.ParseInfo.M.DbmovieID != 0 {
			entry.ParseInfo.M.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, entry.ParseInfo.M.DbmovieID, &addinlist)
		}
		if entry.ParseInfo.M.DbmovieID == 0 || entry.ParseInfo.M.MovieID == 0 {
			logdenied("Unwanted Movie", "", entry, dl)
			return true
		}
	} else {
		logdenied("Unwanted Movie", "", entry, dl)
		return true
	}

	if entry.ParseInfo.M.MovieID == 0 {
		logdenied("Unwanted Movie", "", entry, dl)
		return true
	}
	s.MediaID = entry.ParseInfo.M.MovieID
	s.MediaDBID = entry.ParseInfo.M.DbmovieID
	entry.Dbid = entry.ParseInfo.M.DbmovieID
	entry.NzbmovieID = entry.ParseInfo.M.MovieID

	database.QueryMovieDataDont(entry.ParseInfo.M.MovieID, &s.DontSearch, &s.DontUpgrade, &entry.QualityTemplate)

	s.MediaQualityTemplate = entry.QualityTemplate
	database.QueryDbmovieData(entry.ParseInfo.M.DbmovieID, &s.year, &s.imdb, &entry.WantedTitle)

	s.title = entry.WantedTitle
	if !strings.EqualFold(s.Quality, entry.QualityTemplate) && entry.QualityTemplate != "" {
		s.Quality = entry.QualityTemplate
	}
	importfeed.StripTitlePrefixPostfixGetQual(entry.ParseInfo.M.Title, entry.QualityTemplate)
	return false
}

func (s *Searcher) getserierss(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	err := parser.GetDBIDs(&entry.ParseInfo.M, s.Cfgpstr, "", true)
	s.MediaDBID = 0
	s.MediaID = 0
	s.MediaMainDBID = 0
	s.MediaMainID = 0
	s.DontSearch = false
	s.DontUpgrade = false
	s.MediaQualityTemplate = ""
	s.thetvdbid = 0
	s.title = ""
	s.season = ""
	s.episode = ""
	s.identifier = ""
	if err != nil {
		logdenied(err.Error(), "", entry, dl)
		return true
	}

	if entry.ParseInfo.M.SerieID == 0 {
		logdenied("Unwanted Serie", "", entry, dl)
		return true
	}
	if entry.ParseInfo.M.DbserieID == 0 {
		logdenied("Unwanted DBSerie", "", entry, dl)
		return true
	}
	if entry.ParseInfo.M.DbserieEpisodeID == 0 {
		logdenied("Unwanted DBSerieEpisode", "", entry, dl)
		return true
	}
	if entry.ParseInfo.M.SerieEpisodeID == 0 {
		logdenied("Unwanted SerieEpisode", "", entry, dl)
		return true
	}
	entry.NzbepisodeID = entry.ParseInfo.M.SerieEpisodeID
	entry.Dbid = entry.ParseInfo.M.DbserieID
	s.MediaDBID = entry.ParseInfo.M.DbserieEpisodeID
	s.MediaID = entry.ParseInfo.M.SerieEpisodeID
	s.MediaMainDBID = entry.ParseInfo.M.DbserieID
	s.MediaMainID = entry.ParseInfo.M.SerieID

	database.QueryEpisodeDataDont(entry.ParseInfo.M.SerieEpisodeID, &s.DontSearch, &s.DontUpgrade, &entry.QualityTemplate)
	s.MediaQualityTemplate = entry.QualityTemplate
	database.QueryDbserieData(entry.ParseInfo.M.DbserieID, &s.thetvdbid, &s.title)
	database.QueryDbserieEpisodeData(entry.ParseInfo.M.DbserieEpisodeID, &s.season, &s.episode, &s.identifier)
	//if entry.ParseInfo.M.DbserieID != 0 {
	//	database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: database.QueryDbserieAlternatesGetTitleByDBID, Args: []interface{}{entry.ParseInfo.M.DbserieID}}, entry.WantedAlternates)
	//}
	return false
}

func (s *Searcher) GetRSSFeed(searchGroupType string, listname string) (*SearchResults, error) {
	if listname == "" {
		return nil, errors.New("listname empty")
	}
	i := config.GetMediaListsEntryIndex(s.Cfgpstr, listname)
	var templatelist string
	if i != -1 {
		templatelist = config.SettingsMedia[s.Cfgpstr].Lists[i].TemplateList
	}

	if templatelist == "" {
		return nil, errors.New("listname template empty")
	}
	if !config.CheckGroup("list_", templatelist) {
		return nil, errors.New("list not found")
	}
	if !config.CheckGroup("quality_", s.Quality) {
		return nil, errors.New("quality for RSS not found")
	}

	s.SearchGroupType = searchGroupType
	s.SearchActionType = logger.StrRss

	intid := -1
	for idxi := range config.SettingsQuality["quality_"+s.Quality].Indexer {
		if strings.EqualFold(config.SettingsQuality["quality_"+s.Quality].Indexer[idxi].TemplateIndexer, templatelist) {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFuncS(config.SettingsQuality["quality_"+s.Quality].Indexer, func(c config.QualityIndexerConfig) bool {
	//	return strings.EqualFold(c.TemplateIndexer, templatelist)
	//})
	if intid != -1 && config.SettingsQuality["quality_"+s.Quality].Indexer[intid].TemplateRegex == "" {
		return nil, errors.New("regex template empty")
	}

	blockinterval := -5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.SettingsGeneral.FailedIndexerBlockTime
	}

	s.ListTemplate = templatelist

	if database.QueryCountColumn("indexer_fails", "indexer = ? and last_fail > ?", config.SettingsList["list_"+s.ListTemplate].URL, logger.TimeGetNow().Add(time.Minute*time.Duration(blockinterval))) >= 1 {
		logger.Log.Debug().Str(logger.StrListname, templatelist).Int("Minutes", blockinterval).Msg("Indexer temporarily disabled due to fail in the last")
		return nil, logger.ErrDisabled
	}

	if s.Cfgpstr == "" {
		return nil, logger.ErrOther
	}

	var dl SearchResults
	dl.Raw = *querynzbs(s.Cfgpstr, "rsslast", templatelist, s.Quality, &apiexternal.NzbIndexer{Name: templatelist, Customrssurl: config.SettingsList["list_"+s.ListTemplate].URL, LastRssID: database.QueryStringColumn("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", templatelist, s.Quality, ""), InitRows: 100}, "", "", s.thetvdbid, 0, 0, false, false, config.SettingsList["list_"+s.ListTemplate].Limit, 1)
	if dl.Raw != nil && len(dl.Raw) >= 1 {
		if (dl.Raw)[0].ID != "" {
			addrsshistory(config.SettingsList["list_"+s.ListTemplate].URL, (dl.Raw)[0].ID, s.Quality, templatelist)
		}
		var addfound bool
		if i != -1 {
			addfound = config.SettingsMedia[s.Cfgpstr].Lists[i].Addfound
		}
		s.parseentries(&dl, listname, addfound)
		if len(dl.Accepted) > 1 {
			sort.Slice(dl.Accepted, func(i, j int) bool {
				return dl.Accepted[i].Prio > dl.Accepted[j].Prio
			})
		}
	}

	// indexer.Close()
	return &dl, nil
}

func addrsshistory(urlv string, lastid string, quality string, config string) {
	id := database.QueryUintColumn("select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", config, quality, urlv)
	if id >= 1 {
		database.UpdateColumnStatic("update r_sshistories set last_id = ? where id = ?", lastid, id)
	} else {
		database.InsertStatic("insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?)", config, quality, urlv, lastid)
	}
}

func getsearchtype(minimumPriority int, dont bool, force bool) (string, error) {
	if minimumPriority == 0 {
		return logger.StrMissing, nil
	} else if dont && !force {
		return "", logger.ErrDisabled
	}
	return logger.StrUpgrade, nil
}

func MovieSearch(cfgpstr string, movieid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	if cfgpstr == "" {
		return nil, logger.ErrCfgpNotFound
	}
	if database.QueryIntColumn("select count() from movies where id = ?", movieid) == 0 {
		return nil, logger.ErrNotFoundMovie
	}

	//var processedindexer int

	searchvar := &Searcher{
		Cfgpstr:         cfgpstr,
		SearchGroupType: logger.StrMovie,
		MediaID:         movieid,
	}
	//searchvar := NewSearcher(cfgpstr, "", logger.StrMovie, "", movieid)
	defer searchvar.Close()
	database.QueryMovieData(movieid, &searchvar.MediaDBID, &searchvar.DontSearch, &searchvar.DontUpgrade, &searchvar.MediaQualityTemplate)
	if searchvar.MediaDBID == 0 {
		return nil, logger.ErrNotFoundDbmovie
	}

	if searchvar.DontSearch && !forceDownload {
		return nil, logger.ErrDisabled
	}

	if !config.CheckGroup("quality_", searchvar.MediaQualityTemplate) {
		return nil, errors.New("quality for movie not found")
	}
	if len(config.SettingsQuality["quality_"+searchvar.MediaQualityTemplate].Indexer) == 0 {
		return nil, errors.New("indexer for movie not found")
	}
	searchvar.Quality = searchvar.MediaQualityTemplate
	searchvar.MinimumPriority = GetHighestMoviePriorityByFiles(false, true, movieid, searchvar.MediaQualityTemplate)
	var err error
	searchvar.SearchActionType, err = getsearchtype(searchvar.MinimumPriority, searchvar.DontUpgrade, forceDownload)
	if err != nil {
		return nil, err
	}
	database.QueryDbmovieData(searchvar.MediaDBID, &searchvar.year, &searchvar.imdb, &searchvar.title)

	if searchvar.year == 0 {
		return nil, errors.New("year for movie not found")
	}
	database.QueryStaticStringArrayObj(&searchvar.AlternateTitles, false,
		database.QueryDbmovieTitlesGetTitleByIDNoEmpty, searchvar.MediaDBID)
	//logger.LogAnyDebug("Search for movie id", logger.LoggerValue{Name: "id", Value: movieid})
	logger.Log.Debug().Uint(logger.StrID, movieid).Msg("Search for movie id")

	var dl SearchResults
	return searchnow(searchvar, &dl, searchvar.MediaQualityTemplate, "Update movies set lastscan = ? where id = ?", movieid, false, func(ind string) (bool, error) {
		return searchvar.mediasearchindexer(ind, titlesearch, 0, 0, &dl)
	})
}

func getsearchforquerystring(title *string, add string) string {
	logger.StringDeleteRuneP(title, '(')
	logger.StringDeleteRuneP(title, '&')
	logger.StringDeleteRuneP(title, ')')
	return *title + add
}

func (s *Searcher) mediasearchindexer(indexer string, titlesearch bool, season int, episode int, dl *SearchResults) (bool, error) {

	if !config.CheckGroup("indexer_", indexer) {
		return true, errors.New("indexer template not found")
	}

	if !config.CheckGroup("quality_", s.MediaQualityTemplate) {
		return false, errors.New("quality template not found")
	}

	if !strings.EqualFold(config.SettingsIndexer["indexer_"+indexer].IndexerType, "newznab") {
		// idxcfg.Close()
		return true, errors.New("not newznab")
	}
	if !config.SettingsIndexer["indexer_"+indexer].Enabled {
		// idxcfg.Close()
		return true, logger.ErrNotEnabled
	}

	if ok, err := apiexternal.NewznabCheckLimiter(config.SettingsIndexer["indexer_"+indexer].URL); !ok {
		return false, err
	}

	cats := config.QualityIndexerByQualityAndTemplateGetFieldString(s.Quality, indexer, "CategoriesIndexer")
	if !titlesearch && ((s.SearchGroupType == logger.StrMovie && s.imdb != "") || (s.SearchGroupType == logger.StrSeries && s.thetvdbid != 0)) {
		err := s.searchMedia(s.Quality, indexer, s.SearchGroupType, cats, titlesearch, season, episode, "", dl)
		if err != nil {
			//logger.LogAnyError(err, "Error searching media by id", logger.LoggerValue{Name: "searchtype", Value: s.SearchGroupType}, logger.LoggerValue{Name: "id", Value: s.MediaID})
			logger.Log.Error().Err(err).Uint("Media ID", s.MediaID).Str("searchtype", s.SearchGroupType).Msg("Error Searching Media by ID")
		} else {
			if dl != nil && len(dl.Accepted) >= 1 && config.SettingsQuality["quality_"+s.Quality].CheckUntilFirstFound {
				return true, nil
			}
		}
	}
	if config.SettingsQuality["quality_"+s.Quality].SearchForTitleIfEmpty && len(dl.Accepted) == 0 {
		titlesearch = true
	}
	if !titlesearch {
		return true, nil
	}
	var addstr string
	if s.SearchGroupType == logger.StrSeries && s.identifier != "" {
		addstr = " " + s.identifier
	} else if s.SearchGroupType == logger.StrMovie && s.year != 0 {
		addstr = fmt.Sprintf(" %d", s.year) //" " + logger.IntToString(s.year)
	}

	var titles []string
	if config.SettingsQuality["quality_"+s.Quality].BackupSearchForTitle && config.SettingsQuality["quality_"+s.Quality].BackupSearchForAlternateTitle {
		titles = append([]string{s.title}, s.AlternateTitles...)
	} else if config.SettingsQuality["quality_"+s.Quality].BackupSearchForAlternateTitle {
		titles = s.AlternateTitles
	} else if !config.SettingsQuality["quality_"+s.Quality].BackupSearchForTitle {
		titles = nil
	} else {
		titles = []string{s.title}
	}
	defer logger.Clear(&titles)
	firstfound := config.SettingsQuality["quality_"+s.Quality].CheckUntilFirstFound
	if config.SettingsQuality["quality_"+s.Quality].SearchForAlternateTitleIfEmpty && len(titles) == 1 && strings.EqualFold(titles[0], s.title) {
		firstfound = true
	}
	if len(titles) == 0 {
		return true, nil
	}
	searched := titles[:0]

	var searchfor string
	var err error
	for idxtitle := range titles {
		if titles[idxtitle] == "" {
			continue
		}
		searchfor = getsearchforquerystring(&titles[idxtitle], addstr)
		if logger.ContainsStringsI(&searched, searchfor) {
			continue
		}
		searched = append(searched, searchfor)
		err = s.searchMedia(s.Quality, indexer, s.SearchGroupType, cats, titlesearch, season, episode, searchfor, dl)
		if err != nil {
			//logger.LogAnyError(err, "Error searching media by title", logger.LoggerValue{Name: "searchtype", Value: s.SearchGroupType}, logger.LoggerValue{Name: "title", Value: searchfor}, logger.LoggerValue{Name: "id", Value: s.MediaID})
			logger.Log.Error().Err(err).Uint("Media ID", s.MediaID).Str("searchtype", s.SearchGroupType).Str("Title", searchfor).Msg("Error Searching Media by Title")
		} else {
			if dl != nil && len(dl.Accepted) >= 1 {
				if firstfound {
					logger.Clear(&searched)
					return true, nil
				}
			}
		}
	}
	logger.Clear(&searched)
	return true, nil
}

func SeriesSearch(cfgpstr string, episodeid uint, forceDownload bool, titlesearch bool) (*SearchResults, error) {
	if cfgpstr == "" {
		return nil, logger.ErrCfgpNotFound
	}
	if episodeid == 0 {
		return nil, errors.New("episode is zero")
	}
	if database.QueryIntColumn("select count() from serie_episodes where id = ?", episodeid) == 0 {
		return nil, errors.New("episode not found")
	}

	searchvar := &Searcher{
		Cfgpstr:         cfgpstr,
		SearchGroupType: logger.StrSeries,
		MediaID:         episodeid,
	}
	//searchvar := NewSearcher(cfgpstr, "", logger.StrSeries, "", episodeid)
	defer searchvar.Close()
	database.QueryEpisodeData(episodeid, &searchvar.MediaDBID, &searchvar.MediaMainDBID, &searchvar.MediaMainID, &searchvar.DontSearch, &searchvar.DontUpgrade, &searchvar.MediaQualityTemplate)
	searchvar.Quality = searchvar.MediaQualityTemplate
	if searchvar.MediaDBID == 0 || searchvar.MediaMainDBID == 0 || searchvar.MediaMainID == 0 {
		return nil, logger.ErrNotFoundDbserie
	}

	if searchvar.DontSearch && !forceDownload {
		return nil, logger.ErrDisabled
	}

	if !config.CheckGroup("quality_", searchvar.MediaQualityTemplate) {
		return nil, errors.New("quality for episode not found")
	}
	if len(config.SettingsQuality["quality_"+searchvar.MediaQualityTemplate].Indexer) == 0 {
		return nil, errors.New("indexer for episode not found")
	}

	database.QueryDbserieData(searchvar.MediaMainDBID, &searchvar.thetvdbid, &searchvar.title)
	database.QueryDbserieEpisodeData(searchvar.MediaDBID, &searchvar.season, &searchvar.episode, &searchvar.identifier)

	searchvar.MinimumPriority = GetHighestEpisodePriorityByFiles(false, true, episodeid, searchvar.MediaQualityTemplate)
	database.QueryStaticStringArrayObj(&searchvar.AlternateTitles, false,
		database.QueryDbserieAlternatesGetTitleByDBIDNoEmpty, searchvar.MediaMainDBID)

	var err error
	searchvar.SearchActionType, err = getsearchtype(searchvar.MinimumPriority, searchvar.DontUpgrade, forceDownload)
	if err != nil {
		return nil, err
	}
	//logger.LogAnyDebug("Search for serie id", logger.LoggerValue{Name: "id", Value: episodeid})
	logger.Log.Debug().Uint(logger.StrID, episodeid).Msg("Search for serie id")
	//var processedindexer int
	var dl SearchResults
	return searchnow(searchvar, &dl, searchvar.MediaQualityTemplate, database.QueryUpdateSerieLastscan, episodeid, false, func(ind string) (bool, error) {
		return searchvar.mediasearchindexer(ind, titlesearch, 0, 0, &dl)
	})
}

func (s *Searcher) initIndexer(quality string, indexer string, rssapi string) (*apiexternal.NzbIndexer, string, int, int, error) {
	if !config.CheckGroup("indexer_", indexer) {
		return nil, "", 0, 0, errors.New("indexer not found")
	}
	if !strings.EqualFold(config.SettingsIndexer["indexer_"+indexer].IndexerType, "newznab") {
		// idxcfg.Close()
		return nil, "", 0, 0, logger.Errwrongtype
	}
	if !config.SettingsIndexer["indexer_"+indexer].Rssenabled && rssapi == logger.StrRss {
		// idxcfg.Close()
		return nil, "", 0, 0, logger.ErrDisabled
	} else if !config.SettingsIndexer["indexer_"+indexer].Enabled {
		// idxcfg.Close()
		return nil, "", 0, 0, logger.ErrDisabled
	}

	if ok, _ := apiexternal.NewznabCheckLimiter(config.SettingsIndexer["indexer_"+indexer].URL); !ok {
		// idxcfg.Close()
		return nil, "", 0, 0, logger.ErrToWait
	}

	var lastindexerid string
	if s.SearchActionType == logger.StrRss {
		lastindexerid = database.QueryStringColumn("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", s.Cfgpstr, s.Quality, config.SettingsIndexer["indexer_"+indexer].URL)
	}
	var u apiexternal.NzbIndexer
	returnnzbindexer(&u, indexer, quality, lastindexerid)
	return &u, config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "CategoriesIndexer"), config.SettingsIndexer["indexer_"+indexer].RssEntriesloop, config.SettingsIndexer["indexer_"+indexer].MaxRssEntries, nil
}

func returnnzbindexer(u *apiexternal.NzbIndexer, indexer string, quality string, lastindexerid string) {

	u.URL = config.SettingsIndexer["indexer_"+indexer].URL
	u.Apikey = config.SettingsIndexer["indexer_"+indexer].Apikey
	u.UserID = config.SettingsIndexer["indexer_"+indexer].Userid
	u.SkipSslCheck = config.SettingsIndexer["indexer_"+indexer].DisableTLSVerify
	u.DisableCompression = config.SettingsIndexer["indexer_"+indexer].DisableCompression
	u.Addquotesfortitlequery = config.SettingsIndexer["indexer_"+indexer].Addquotesfortitlequery
	u.AdditionalQueryParams = config.QualityIndexerByQualityAndTemplateGetFieldString(quality, indexer, "AdditionalQueryParams")
	u.LastRssID = lastindexerid
	u.Customapi = config.SettingsIndexer["indexer_"+indexer].Customapi
	u.Customurl = config.SettingsIndexer["indexer_"+indexer].Customurl
	u.Customrssurl = config.SettingsIndexer["indexer_"+indexer].Customrssurl
	u.Customrsscategory = config.SettingsIndexer["indexer_"+indexer].Customrsscategory
	u.Limitercalls = config.SettingsIndexer["indexer_"+indexer].Limitercalls
	u.Limiterseconds = config.SettingsIndexer["indexer_"+indexer].Limiterseconds
	u.LimitercallsDaily = config.SettingsIndexer["indexer_"+indexer].LimitercallsDaily
	u.TimeoutSeconds = config.SettingsIndexer["indexer_"+indexer].TimeoutSeconds
	u.MaxAge = config.SettingsIndexer["indexer_"+indexer].MaxAge
	u.InitRows = config.SettingsIndexer["indexer_"+indexer].MaxRssEntries
	u.OutputAsJSON = config.SettingsIndexer["indexer_"+indexer].OutputAsJSON
}

func (s *Searcher) searchMedia(quality string, indexer string, mediatype string, cats string, titlesearch bool, season int, episode int, title string, dl *SearchResults) error {
	if !config.CheckGroup("quality_", quality) {
		return errors.New("quality template not found")
	}

	if !config.CheckGroup("indexer_", indexer) {
		return errors.New("indexer template not found")
	}

	if !strings.EqualFold(config.SettingsIndexer["indexer_"+indexer].IndexerType, "newznab") {
		return errors.New("not newznab")
	}
	if !config.SettingsIndexer["indexer_"+indexer].Enabled {
		return logger.ErrNotEnabled
	}

	if ok, err := apiexternal.NewznabCheckLimiter(config.SettingsIndexer["indexer_"+indexer].URL); !ok {
		return err
	}

	var lastindexerid string
	if s.SearchActionType == logger.StrRss {
		lastindexerid = database.QueryStringColumn("select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", s.Cfgpstr, s.Quality, config.SettingsIndexer["indexer_"+indexer].URL)
	}
	var nzbindexer apiexternal.NzbIndexer
	returnnzbindexer(&nzbindexer, indexer, quality, lastindexerid)

	var searchtype = "query"
	var searchfor = title
	//var n *[]apiexternal.NZB
	if !titlesearch {
		if mediatype == logger.StrMovie && s.imdb != "" {
			searchtype = "imdb"
			searchfor = strings.Trim(s.imdb, "t")
			//n, _, erri = apiexternal.QueryNewznabMovieImdb(indexer, quality, nzbindexer, strings.Trim(s.imdb, "t"), cats)
		} else if mediatype == logger.StrSeries && s.thetvdbid != 0 {
			searchtype = "tvdb"
			//n, _, erri = apiexternal.QueryNewznabTvTvdb(indexer, quality, nzbindexer, s.thetvdbid, cats, season, episode, true, true)
		}
	} //else {
	//n, _, erri = apiexternal.QueryNewznabQuery(indexer, quality, nzbindexer, title, cats, "search")
	//}

	addnzbs(dl, querynzbs(s.Cfgpstr, searchtype, indexer, quality, &nzbindexer, cats, searchfor, s.thetvdbid, season, episode, true, true, 0, 0))
	logger.ClearVar(&nzbindexer)
	return nil
}

func (s *Searcher) filterSizeNzbs(entry *apiexternal.Nzbwithprio, dl *SearchResults) bool {
	for idx := range config.SettingsMedia[s.Cfgpstr].DataImport {
		if !config.CheckGroup("path_", config.SettingsMedia[s.Cfgpstr].DataImport[idx].TemplatePath) {
			continue
		}
		if config.SettingsPath["path_"+config.SettingsMedia[s.Cfgpstr].DataImport[idx].TemplatePath].MinSize != 0 && entry.NZB.Size < config.SettingsPath["path_"+config.SettingsMedia[s.Cfgpstr].DataImport[idx].TemplatePath].MinSizeByte {
			logdenied("Too Small", "", entry, dl)
			return true
		}

		if config.SettingsPath["path_"+config.SettingsMedia[s.Cfgpstr].DataImport[idx].TemplatePath].MaxSize != 0 && entry.NZB.Size > config.SettingsPath["path_"+config.SettingsMedia[s.Cfgpstr].DataImport[idx].TemplatePath].MaxSizeByte {
			logdenied("Too Big", "", entry, dl)
			return true
		}
	}
	return false
}

func filterRegexNzbs(entry *apiexternal.Nzbwithprio, templateregex string, dl *SearchResults) bool {
	if templateregex == "" {
		logdenied(deniedbyregex, "regex_template empty", entry, dl)
		return true
	}
	var requiredmatched bool
	for idxrow := range config.SettingsRegex["regex_"+templateregex].Required {
		if config.RegexGetMatchesFind(&config.SettingsRegex["regex_"+templateregex].Required[idxrow], entry.NZB.Title, 1) {
			requiredmatched = true
			break
		}
	}
	if len(config.SettingsRegex["regex_"+templateregex].Required) >= 1 && !requiredmatched {
		logdenied("required not matched", "", entry, dl)
		return true
	}
	var breakfor bool
	for idxrow := range config.SettingsRegex["regex_"+templateregex].Rejected {
		if config.RegexGetMatchesFind(&config.SettingsRegex["regex_"+templateregex].Rejected[idxrow], entry.WantedTitle, 1) {
			//Regex is in title - skip test
			continue
		}
		breakfor = false
		for idxwanted := range entry.WantedAlternates {
			if strings.EqualFold(entry.WantedAlternates[idxwanted], entry.WantedTitle) {
				continue
			}
			if config.RegexGetMatchesFind(&config.SettingsRegex["regex_"+templateregex].Rejected[idxrow], entry.WantedAlternates[idxwanted], 1) {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		if config.RegexGetMatchesFind(&config.SettingsRegex["regex_"+templateregex].Rejected[idxrow], entry.NZB.Title, 1) {
			logdenied(deniedbyregex, config.SettingsRegex["regex_"+templateregex].Rejected[idxrow], entry, dl)
			return true
		}
	}
	return false
}

func (s *Searcher) parseentries(dl *SearchResults, listname string, addfound bool) {
	if dl == nil || len((dl.Raw)) == 0 {
		return
	}
	//historytableurl := "serie_episode_histories_url"
	//historytabletitle := "serie_episode_histories_title"
	//if logger.HasPrefixI(s.SearchGroupType, logger.StrMovie) {
	//	historytableurl = "movie_histories_url"
	//	historytabletitle = "movie_histories_title"
	//}
	if cap(dl.Denied) == 0 {
		dl.Denied = make([]apiexternal.Nzbwithprio, 0, len(dl.Raw))
	}
	if cap(dl.Accepted) == 0 {
		dl.Accepted = make([]apiexternal.Nzbwithprio, 0, 5)
	}
	var entry apiexternal.Nzbwithprio
	var cont bool
	var templateregex string
	var skipemptysize bool
	var err error
	for idx := range dl.Raw {
		//Check Title Length
		entry = apiexternal.Nzbwithprio{NZB: &dl.Raw[idx]}
		if dl.Raw[idx].DownloadURL == "" {
			logdenied("No Url", "", &entry, dl)
			//entry.Close()
			continue
		}
		if dl.Raw[idx].Title == "" {
			logdenied("No Title", "", &entry, dl)
			//entry.Close()
			continue
		}
		if len(strings.Trim(dl.Raw[idx].Title, " ")) <= 3 {
			logdenied("Title too short", "", &entry, dl)
			//entry.Close()
			continue
		}
		cont = false
		for idxi := range dl.Denied {
			if dl.Denied[idxi].NZB.DownloadURL == dl.Raw[idx].DownloadURL {
				cont = true
				break
			}
		}
		if cont {
			continue
		}
		//if logger.ContainsFunc(&dl.Denied, func(c apiexternal.Nzbwithprio) bool {
		//	return c.NZB.DownloadURL == dl.Raw[idx].DownloadURL
		//}) {
		//entry.Close()
		//logdeniedsimple("Already added", &dl.Raw[idx], dl)
		//	continue
		//}

		cont = false
		for idxi := range dl.Accepted {
			if dl.Accepted[idxi].NZB.DownloadURL == dl.Raw[idx].DownloadURL {
				cont = true
				break
			}
		}
		if cont {
			continue
		}
		//if logger.ContainsFunc(&dl.Accepted, func(c apiexternal.Nzbwithprio) bool { return c.NZB.DownloadURL == dl.Raw[idx].DownloadURL }) {
		//logdeniedsimple("Already added", &dl.Raw[idx], dl)
		//entry.Close()
		//	continue
		//}

		//Check Size
		templateregex, skipemptysize = config.QualityIndexerByQualityAndTemplateFirTemplateAndSize(dl.Raw[idx].Quality, dl.Raw[idx].Indexer)

		if templateregex == "" {
			logdenied("No Indexer Regex Template", "", &entry, dl)
			//entry.Close()
			continue
		}
		if skipemptysize && dl.Raw[idx].Size == 0 {
			logdenied("Missing size", "", &entry, dl)
			//entry.Close()
			continue
		}

		//check history
		if len(dl.Raw[idx].DownloadURL) > 1 {

			if logger.HasPrefixI(s.SearchGroupType, logger.StrMovie) {
				if config.SettingsGeneral.UseMediaCache {
					if logger.IndexFunc(&database.CacheHistoryUrlMovie, func(elem string) bool { return elem == dl.Raw[idx].DownloadURL }) != -1 {
						logdenied("Already downloaded (Url)", "", &entry, dl)
						continue
					}
				} else {
					if database.QueryIntColumn("select count() from movie_histories where url = ?", &dl.Raw[idx].DownloadURL) >= 1 {
						logdenied("Already downloaded (Url)", "", &entry, dl)
						continue
					}
				}
			} else {
				if config.SettingsGeneral.UseMediaCache {
					if logger.IndexFunc(&database.CacheHistoryUrlSeries, func(elem string) bool { return elem == dl.Raw[idx].DownloadURL }) != -1 {
						logdenied("Already downloaded (Url)", "", &entry, dl)
						continue
					}
				} else {
					if database.QueryIntColumn("select count() from serie_episode_histories where url = ?", &dl.Raw[idx].DownloadURL) >= 1 {
						logdenied("Already downloaded (Url)", "", &entry, dl)
						continue
					}
				}
			}
			// if logger.GlobalCache.CheckStringArrValue(historytableurl, dl.Raw[idx].DownloadURL) {
			// 	logdenied("Already downloaded (Url)", "", &entry, dl)
			// 	//entry.Close()
			// 	continue
			// }
		}
		if config.QualityIndexerByQualityAndTemplateGetFieldHistoryCheckTitle(dl.Raw[idx].Quality, dl.Raw[idx].Indexer) && len(dl.Raw[idx].Title) > 1 {

			if logger.HasPrefixI(s.SearchGroupType, logger.StrMovie) {
				if config.SettingsGeneral.UseMediaCache {
					if logger.IndexFunc(&database.CacheHistoryTitleMovie, func(elem string) bool { return elem == dl.Raw[idx].Title }) != -1 {
						logdenied("Already downloaded (Title)", "", &entry, dl)
						continue
					}
				} else {
					if database.QueryIntColumn("select count() from movie_histories where title = ?", &dl.Raw[idx].Title) >= 1 {
						logdenied("Already downloaded (Title)", "", &entry, dl)
						continue
					}
				}
			} else {
				if config.SettingsGeneral.UseMediaCache {
					if logger.IndexFunc(&database.CacheHistoryTitleSeries, func(elem string) bool { return elem == dl.Raw[idx].Title }) != -1 {
						logdenied("Already downloaded (Title)", "", &entry, dl)
						continue
					}
				} else {
					if database.QueryIntColumn("select count() from serie_episode_histories where title = ?", &dl.Raw[idx].Title) >= 1 {
						logdenied("Already downloaded (Title)", "", &entry, dl)
						continue
					}
				}
			}
			// if logger.GlobalCache.CheckStringArrValue(historytabletitle, dl.Raw[idx].Title) {
			// 	logdenied("Already downloaded (Title)", "", &entry, dl)
			// 	//entry.Close()
			// 	continue
			// }
		}
		// if s.checkhistory(s.Quality, entry.Indexer, idx, dl) {
		// 	continue
		// }

		if s.filterSizeNzbs(&entry, dl) {
			//entry.Close()
			continue
		}
		if s.checkcorrectid(&entry, dl) {
			//entry.Close()
			continue
		}

		entry.ParseInfo = parser.ParseFile(&dl.Raw[idx].Title, false, s.SearchGroupType == logger.StrSeries, s.SearchGroupType, false)
		if s.SearchActionType == logger.StrRss {
			if s.getmediadatarss(&entry, dl, listname, addfound) {
				//entry.Close()
				continue
			}
		} else {
			if s.getmediadata(&entry, dl) {
				//entry.Close()
				continue
			}
		}
		//needs the identifier from getmediadata
		if s.checkepisode(&entry, dl) {
			//entry.Close()
			continue
		}

		if filterRegexNzbs(&entry, templateregex, dl) {
			//entry.Close()
			continue
		}

		if entry.ParseInfo.M.Priority == 0 {
			parser.GetPriorityMapQual(&entry.ParseInfo.M, s.Cfgpstr, entry.QualityTemplate, false, true)
			entry.Prio = entry.ParseInfo.M.Priority
		}

		err = importfeed.StripTitlePrefixPostfixGetQual(entry.ParseInfo.M.Title, entry.QualityTemplate)
		if err != nil {
			logger.Logerror(err, "Strip Failed")
		}
		//check quality
		if filterTestQualityWanted(&entry, dl) {
			//entry.Close()
			continue
		}
		//check priority
		if entry.ParseInfo.M.Priority == 0 {
			logdeniedint("Prio unknown", 0, &entry, dl)
			//entry.Close()
			continue
		}

		if entry.MinimumPriority == entry.ParseInfo.M.Priority {
			logdeniedint("Prio same", 0, &entry, dl)
			//entry.Close()
			continue
		}

		if entry.MinimumPriority != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].UseForPriorityMinDifference == 0 && entry.ParseInfo.M.Priority <= entry.MinimumPriority {
			logdeniedint("Prio lower", entry.MinimumPriority, &entry, dl)
			//entry.Close()
			continue
		}

		if entry.MinimumPriority != 0 && config.SettingsQuality["quality_"+entry.QualityTemplate].UseForPriorityMinDifference != 0 && (config.SettingsQuality["quality_"+entry.QualityTemplate].UseForPriorityMinDifference+entry.ParseInfo.M.Priority) <= entry.MinimumPriority {
			logdeniedint("Prio lower", entry.MinimumPriority, &entry, dl)
			//entry.Close()
			continue
		}

		if s.checkyear(&entry, dl) {
			//entry.Close()
			continue
		}

		if s.checktitle(&entry, dl) {
			//entry.Close()
			continue
		}

		//logger.LogAnyDebug("Release ok", logger.LoggerValue{Name: "quality", Value: entry.QualityTemplate}, logger.LoggerValue{Name: "title", Value: dl.Raw[idx].Title}, logger.LoggerValue{Name: "minimum prio", Value: entry.MinimumPriority}, logger.LoggerValue{Name: logger.StrPriority, Value: entry.ParseInfo.M.Priority})
		logger.Log.Debug().Str("quality", entry.QualityTemplate).Str(logger.StrTitle, dl.Raw[idx].Title).Int("minimum prio", entry.MinimumPriority).Int(logger.StrPriority, entry.ParseInfo.M.Priority).Msg("Release ok")

		dl.Accepted = append(dl.Accepted, entry)
		//entry.Close()
		if config.SettingsQuality["quality_"+entry.QualityTemplate].CheckUntilFirstFound {
			break
		}
	}
}

func logdenied(reason string, optional string, entry *apiexternal.Nzbwithprio, dl *SearchResults) {
	if len(reason) >= 1 {
		//logger.LogAnyDebug(skippedstr, logger.LoggerValue{Name: "reason", Value: reason}, logger.LoggerValue{Name: logger.StrTitle, Value: entry.NZB.Title}, logger.LoggerValue{Name: "optional", Value: optional})
		evt := logger.Log.Debug().Str("reason", reason).Str(logger.StrTitle, entry.NZB.Title)
		if len(optional) >= 1 {
			entry.AdditionalReason = optional
			evt.Str("optional", optional)
		}
		evt.Msg(skippedstr)
		logger.ClearVar(evt)
		entry.Reason = reason
	}
	dl.Denied = append(dl.Denied, *entry)
}

func logdeniedint(reason string, optional int, entry *apiexternal.Nzbwithprio, dl *SearchResults) {
	if len(reason) >= 1 {
		//logger.LogAnyDebug(skippedstr, logger.LoggerValue{Name: "reason", Value: reason}, logger.LoggerValue{Name: logger.StrTitle, Value: entry.NZB.Title}, logger.LoggerValue{Name: "optional", Value: optional})
		evt := logger.Log.Debug().Str("reason", reason).Str(logger.StrTitle, entry.NZB.Title)
		if optional != 0 {
			entry.AdditionalReason = logger.IntToString(optional)
			evt.Int("optional", optional)
		}
		evt.Msg(skippedstr)
		logger.ClearVar(evt)
		entry.Reason = reason
	}
	dl.Denied = append(dl.Denied, *entry)
}

func (s *Searcher) allowMovieImport(imdb string, listname string) (bool, error) {
	if listname == "" {
		return false, errors.New("listname empty")
	}
	i := config.GetMediaListsEntryIndex(s.Cfgpstr, listname)
	var templatelist string
	if i != -1 {
		templatelist = config.SettingsMedia[s.Cfgpstr].Lists[i].TemplateList
	}
	if !config.CheckGroup("list_", templatelist) {
		return false, errors.New("list template not found")
	}

	if !strings.EqualFold(s.ListTemplate, templatelist) {
		s.ListTemplate = templatelist
	}

	if config.SettingsList["list_"+s.ListTemplate].MinVotes != 0 {
		if database.QueryImdbIntColumn(database.QueryImdbRatingsCountByImdbVotes, &imdb, config.SettingsList["list_"+s.ListTemplate].MinVotes) >= 1 {
			return false, errors.New("vote count too low")
		}
	}
	if config.SettingsList["list_"+s.ListTemplate].MinRating != 0 {
		if database.QueryImdbIntColumn(database.QueryImdbRatingsCountByImdbRating, &imdb, config.SettingsList["list_"+s.ListTemplate].MinRating) >= 1 {
			return false, errors.New("average vote too low")
		}
	}
	var excludeby string
	for idxgenre := range config.SettingsList["list_"+s.ListTemplate].Excludegenre {
		if database.QueryImdbIntColumn(database.QueryImdbGenresCountByImdbGenre, &imdb, config.SettingsList["list_"+s.ListTemplate].Excludegenre[idxgenre]) >= 1 {
			excludeby = config.SettingsList["list_"+s.ListTemplate].Excludegenre[idxgenre]
			break
		}
	}
	if excludeby != "" {
		return false, errors.New("excluded " + excludeby)
	}
	var includebygenre bool
	for idxgenre := range config.SettingsList["list_"+s.ListTemplate].Includegenre {
		if database.QueryImdbIntColumn(database.QueryImdbGenresCountByImdbGenre, imdb, config.SettingsList["list_"+s.ListTemplate].Includegenre[idxgenre]) >= 1 {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(config.SettingsList["list_"+s.ListTemplate].Includegenre) >= 1 {
		return false, errors.New("included genre not found")
	}
	return true, nil
}
func GetHighestMoviePriorityByFiles(useall bool, checkwanted bool, movieid uint, templatequality string) (minPrio int) {
	return getPriorityByFiles(database.QueryMovieFilesGetIDByMovieID, "Select count() from movie_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?", useall, checkwanted, movieid, templatequality)
}

func getPriorityByFiles(querystring string, querycount string, querysql string, useall bool, checkwanted bool, id uint, templatequality string) (minPrio int) {
	tbl := database.QueryStaticUintArrayNoError(true, 5, querystring, id)
	var prio int
	for idx := range *tbl {
		prio = parser.Getdbidsfromfiles(useall, checkwanted, (*tbl)[idx], querycount, querysql, templatequality)
		if prio == 0 && checkwanted {
			prio = parser.Getdbidsfromfiles(useall, false, (*tbl)[idx], querycount, querysql, templatequality)
		}
		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
	}
	logger.Clear(tbl)
	return minPrio
}

func GetHighestEpisodePriorityByFiles(useall bool, checkwanted bool, episodeid uint, templatequality string) int {
	return getPriorityByFiles(database.QuerySerieEpisodeFilesGetIDByEpisodeID, "Select count() from serie_episode_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?", useall, checkwanted, episodeid, templatequality)
}
