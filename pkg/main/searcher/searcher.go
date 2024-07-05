package searcher

import (
	"bytes"
	"cmp"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// ConfigSearcher is a struct containing configuration and search results
type ConfigSearcher struct {
	// Cfgp is a pointer to a MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// Quality is a pointer to a QualityConfig
	Quality *config.QualityConfig
	// searchActionType is a string indicating the search action type
	searchActionType string //missing,upgrade,rss
	// Sourceentry is a Nzbwithprio result
	//Sourceentry apiexternal.Nzbwithprio
	// Dl contains the search results
	Raw apiexternal.NzbSlice
	// Denied is a slice containing denied apiexternal.Nzbwithprio results
	Denied []apiexternal.Nzbwithprio
	// Accepted is a slice containing accepted apiexternal.Nzbwithprio results
	Accepted []apiexternal.Nzbwithprio
	Done     bool
}

const (
	skippedstr = "Skipped"
)

var (
	strIdentifier      = "identifier"
	strCheckedFor      = "checked for"
	strTitlesearch     = "titlesearch"
	strRejectedby      = "rejected by"
	strResolution      = "resolution"
	strQuality         = "quality"
	strAudio           = "audio"
	strCodec           = "codec"
	strMediaid         = "Media ID"
	episodeprefixarray = []string{"", logger.StrSpace, "0", " 0"}
	errOther           = errors.New("other error")
	errYearEmpty       = errors.New("year empty")
	errSearchvarEmpty  = errors.New("searchvar empty")
	errRegexEmpty      = errors.New("regex template empty")
	plsearcher         = pool.NewPool(100, 10, func(cs *ConfigSearcher) {
		cs.Raw.Arr = make([]apiexternal.Nzbwithprio, 0, 6000)
		cs.Raw.Mu = sync.Mutex{}
	}, func(cs *ConfigSearcher) {
		cs.Cfgp = nil
		cs.Quality = nil
		cs.searchActionType = ""
		//ce.Close()

		for idx := range cs.Denied {
			cs.Denied[idx].ClearArr()
		}
		for idx := range cs.Accepted {
			cs.Accepted[idx].ClearArr()
		}
		for idx := range cs.Raw.Arr {
			cs.Raw.Arr[idx].ClearArr()
		}
		clear(cs.Denied)
		clear(cs.Accepted)
		clear(cs.Raw.Arr)
		cs.Denied = cs.Denied[:0]
		cs.Accepted = cs.Accepted[:0]
		cs.Raw.Arr = cs.Raw.Arr[:0]
	})
)

// SearchRSS searches the RSS feeds of the enabled Newznab indexers for the
// given media type and quality configuration. It returns a ConfigSearcher
// instance for managing the search, or an error if no search could be started.
// Results are added to the passed in DownloadResults instance.
func (s *ConfigSearcher) SearchRSS(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, downloadresults bool, autoclose bool) error {
	if autoclose {
		defer s.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	if s == nil {
		return errSearchvarEmpty
	}
	s.Quality = quality
	s.searchindexers(2, true, false, nil, nil, 0, "", false)
	if s.Done && len(s.Raw.Arr) >= 1 {
		s.searchparse(nil, nil)
		if downloadresults {
			s.Download()
		}
	}
	return nil
}

// runrsssearch executes a search against the RSS feed of the indexer at index2.
// It queries the database for the last ID, searches the RSS feed since that ID,
// and updates the database with the new last ID.
// Returns true if the search was successful, false otherwise.
func (s *ConfigSearcher) runrsssearch(idxcfg *config.IndexersConfig) bool {
	_, firstid, err := apiexternal.QueryNewznabRSSLast(idxcfg, s.Quality, database.GetdatarowN[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", &s.Cfgp.NamePrefix, &s.Quality.Name, &idxcfg.URL), s.Quality.QualityIndexerByQualityAndTemplate(idxcfg), &s.Raw)
	if err == nil {
		if firstid != "" {
			addrsshistory(&idxcfg.URL, &firstid, s.Quality, &s.Cfgp.NamePrefix)
		}
		return true
	}
	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamicany("error", "Error searching indexer", err, &logger.StrIndexer, &idxcfg.Name)
	}
	return false
}

// searchSeriesRSSSeason searches configured indexers for the given TV series
// season using the RSS search APIs. It handles executing searches across
// enabled newznab indexers, parsing results, and adding accepted entries to
// the search results. Returns the searcher and error if any.
func (s *ConfigSearcher) searchSeriesRSSSeason(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, thetvdbid int, season string, useseason bool, downloadentries bool, autoclose bool) error {
	if autoclose {
		defer s.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	if s == nil {
		return logger.ErrCfgpNotFound
	}
	s.Quality = quality
	logger.LogDynamicany("info", "Search for season", &logger.StrTvdb, thetvdbid, &logger.StrSeason, season) //logpointerr
	s.searchindexers(3, false, false, nil, nil, thetvdbid, season, useseason)
	if s.Done && len(s.Raw.Arr) >= 1 {
		s.searchparse(nil, nil)

		if downloadentries {
			s.Download()
		}
		logger.LogDynamicany("info", "Ended Search for season", &logger.StrTvdb, thetvdbid, &logger.StrSeason, season, &logger.StrAccepted, len(s.Accepted), &logger.StrDenied, len(s.Denied)) //logpointerr
	}
	return nil
}

// runrssseasonsearch executes a season search for the given TV series on the
// indexer at the provided index. It returns true if the search was successful,
// false otherwise.
func (s *ConfigSearcher) runrssseasonsearch(idxcfg *config.IndexersConfig, thetvdbid int, season string, useseason bool) bool {
	_, _, err := apiexternal.QueryNewznabTvTvdb(idxcfg, s.Quality, thetvdbid, s.Quality.QualityIndexerByQualityAndTemplate(idxcfg), season, "", useseason, false, &s.Raw)
	if err == nil {
		return true
	}
	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamicany("error", "Error searching indexer", err, &logger.StrIndexer, &idxcfg.Name)
	}
	return false
}

// MediaSearch searches indexers for the given media entry (movie or TV episode)
// using the configured quality profile. It handles filling search variables,
// executing searches across enabled indexers, parsing results, and optionally
// downloading accepted entries. Returns the search results and error if any.
func (s *ConfigSearcher) MediaSearch(cfgp *config.MediaTypeConfig, mediaid *uint, titlesearch bool, downloadentries bool, autoclose bool) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	if autoclose {
		defer s.Close()
	}
	if s == nil || *mediaid == 0 {
		return errSearchvarEmpty
	}
	e := apiexternal.PLNzbwithprio.Get()
	defer e.Close()
	var err error
	if cfgp.Useseries {
		err = s.episodeFillSearchVar(mediaid, e)
	} else {
		err = s.movieFillSearchVar(mediaid, e)
	}
	if err != nil {
		if !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			logger.LogDynamicany("error", "Media Search Failed", err, &logger.StrID, mediaid)
		}
		apiexternal.PLNzbwithprio.Put(e)
		return err
	}

	if s.Quality == nil {
		return errSearchvarEmpty
	}

	s.searchlog("info", "Search for media id", mediaid, titlesearch)

	var sourcealttitles []database.DbstaticTwoStringOneInt
	if s.Quality.BackupSearchForAlternateTitle {
		sourcealttitles = database.Getentryalternatetitlesdirect(&e.Dbid, s.Cfgp.Useseries)
	}
	s.searchindexers(1, false, titlesearch, e, sourcealttitles, 0, "", false)
	if !s.Done {
		s.searchlog("error", "All searches failed", mediaid, titlesearch)
		return nil
	}
	database.ExecNMap(cfgp.Useseries, logger.UpdateMediaLastscan, mediaid)

	if len(s.Raw.Arr) >= 1 {
		s.searchparse(e, sourcealttitles)
		if downloadentries {
			s.Download()
		}
		if len(s.Accepted) >= 1 || len(s.Denied) >= 1 {
			s.searchlog("info", "Ended Search for media id", mediaid, titlesearch)
		}
	}
	return nil
}

func (s *ConfigSearcher) searchlog(typev string, msg string, mediaid *uint, titlesearch bool) {
	if len(s.Accepted) >= 1 {
		logger.LogDynamicany(typev, msg, &logger.StrID, mediaid, &logger.StrSeries, &s.Cfgp.Useseries, &strTitlesearch, &titlesearch, &logger.StrAccepted, len(s.Accepted), &logger.StrDenied, len(s.Denied)) //logpointer
		return
	}
	if len(s.Denied) >= 1 {
		logger.LogDynamicany(typev, msg, &logger.StrID, mediaid, &logger.StrSeries, &s.Cfgp.Useseries, &strTitlesearch, &titlesearch, &logger.StrDenied, len(s.Denied)) //logpointer
		return
	}
	logger.LogDynamicany(typev, msg, &logger.StrID, mediaid, &logger.StrSeries, &s.Cfgp.Useseries, &strTitlesearch, &titlesearch) //logpointer
}

// getaddstr returns a string to append to the search query based on the
// media year or identifier if available. For series it returns empty string.
func (s *ConfigSearcher) getaddstr(e *apiexternal.Nzbwithprio) string {
	if !s.Cfgp.Useseries && e.Info.Year != 0 {
		return logger.JoinStrings(logger.StrSpace, logger.IntToString(e.Info.Year)) //JoinStrings
	} else if e.Info.Identifier != "" {
		return logger.JoinStrings(logger.StrSpace, e.Info.Identifier) //JoinStrings
	}
	return ""
}

// runmediasearch searches for the given media using the provided search parameters.
// It iterates through the configured indexers and attempts different search queries based on the alternate titles.
// Returns true if the search completed successfully, false otherwise.
func (s *ConfigSearcher) runmediasearch(idxcfg *config.IndexersConfig, titlesearch bool, e *apiexternal.Nzbwithprio, sourcealttitles []database.DbstaticTwoStringOneInt) bool {
	cats := s.Quality.QualityIndexerByQualityAndTemplate(idxcfg)
	if cats == nil {
		logger.LogDynamicany("error", "Error getting quality config")
		return false
	}

	usequerysearch := true
	if !titlesearch {
		if !s.Cfgp.Useseries && e.Info.Imdb != "" {
			usequerysearch = false
		} else if s.Cfgp.Useseries && e.NZB.TVDBID != 0 {
			usequerysearch = false
		}
	}
	var err error
	if !usequerysearch {
		if !s.Cfgp.Useseries && e.Info.Imdb != "" {
			_, _, err = apiexternal.QueryNewznabMovieImdb(idxcfg, s.Quality, strings.Trim(e.Info.Imdb, "t"), cats, &s.Raw)
		} else if s.Cfgp.Useseries && e.NZB.TVDBID != 0 {
			_, _, err = apiexternal.QueryNewznabTvTvdb(idxcfg, s.Quality, e.NZB.TVDBID, cats, e.NZB.Season, e.NZB.Episode, true, true, &s.Raw)
		}
	}

	if err != nil && !errors.Is(err, logger.ErrToWait) {
		if s.Cfgp.Useseries {
			logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbepisodeID, &logger.StrSeries, &s.Cfgp.Useseries, err)
		} else {
			logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbmovieID, &logger.StrSeries, &s.Cfgp.Useseries, err)
		}
	} else {
		if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
			logger.LogDynamicany("debug", "Broke loop - result found")
			return true
		}
	}
	if s.Quality.SearchForTitleIfEmpty && !titlesearch && len(s.Raw.Arr) == 0 {
		titlesearch = true
		usequerysearch = true
	}
	if !titlesearch {
		return (err == nil)
	}
	addstr := s.getaddstr(e)
	if titlesearch || s.Quality.BackupSearchForTitle {
		_, _, err = apiexternal.QueryNewznabQuery(idxcfg, s.Quality, logger.JoinStrings(e.WantedTitle, addstr), cats, &s.Raw)
		if err != nil && !errors.Is(err, logger.ErrToWait) {
			if s.Cfgp.Useseries {
				logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbepisodeID, &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &e.WantedTitle, err)
			} else {
				logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbmovieID, &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &e.WantedTitle, err)
			}
			if !s.Quality.BackupSearchForAlternateTitle {
				return false
			}
		} else {
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
				logger.LogDynamicany("debug", "Broke loop - result found")
				return true
			}
		}
	}
	if titlesearch && s.Quality.BackupSearchForAlternateTitle {
		searched := sourcealttitles[:0]
		var success bool
	rootloop:
		for idx := range sourcealttitles {
			if idx != 0 && sourcealttitles[idx].Str1 == "" {
				//logger.LogDynamicany("error", "Skipped empty title")
				continue
			}

			for idxepi := range searched {
				if searched[idxepi].Str1 == sourcealttitles[idx].Str1 || strings.EqualFold(searched[idxepi].Str1, sourcealttitles[idx].Str1) {
					continue rootloop
				}
			}
			searched = append(searched, sourcealttitles[idx])
			if strings.ContainsRune(sourcealttitles[idx].Str1, '(') || strings.ContainsRune(sourcealttitles[idx].Str1, ')') || strings.ContainsRune(sourcealttitles[idx].Str1, '&') {
				sourcealttitles[idx].Str1 = logger.StringRemoveAllRunesMulti(sourcealttitles[idx].Str1, '&', '(', ')')
			}
			_, _, err = apiexternal.QueryNewznabQuery(idxcfg, s.Quality, logger.JoinStrings(sourcealttitles[idx].Str1, addstr), cats, &s.Raw)

			if err != nil && !errors.Is(err, logger.ErrToWait) {
				if s.Cfgp.Useseries {
					logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbepisodeID, &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &sourcealttitles[idx], err)
				} else {
					logger.LogDynamicany("error", "Error Searching Media by Title", &strMediaid, &e.NzbmovieID, &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &sourcealttitles[idx], err)
				}
			} else {
				if err == nil {
					success = true
				}
				if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
					logger.LogDynamicany("debug", "Broke loop - result found")
					break
				}
			}
		}
		return success
	}
	return true
}

// searchindexers searches the configured indexers for media content based on the provided search parameters.
// It submits search tasks to a worker pool and waits for them to complete. The function returns true if any of the
// indexer searches were successful, indicating that search results are available.
//
// The function checks the configured indexers and skips any that are not enabled or do not support RSS feeds
// if the search is for a user's RSS feed. It also skips indexers that have exceeded their API rate limit.
// For each enabled indexer, the function submits a search task to the worker pool, which can be one of three
// types: media search, RSS search, or RSS season search. The function returns true if any of the search tasks
// were successful.
func (s *ConfigSearcher) searchindexers(searchtype int, userss bool, titlesearch bool, e *apiexternal.Nzbwithprio, sourcealttitles []database.DbstaticTwoStringOneInt, thetvdbid int, season string, useseason bool) {
	if len(s.Quality.Indexer) == 0 {
		return
	}
	wg := pool.NewSizedGroup(config.SettingsGeneral.WorkerIndexer)
	s.Done = false
	for _, indcfg := range s.Quality.IndexerCfg {
		if userss && !indcfg.Rssenabled {
			continue
		}
		if !userss && !indcfg.Enabled {
			continue
		}
		if s.Quality == nil || indcfg == nil || !strings.EqualFold(indcfg.IndexerType, "newznab") {
			continue
		}
		if ok, _ := apiexternal.NewznabCheckLimiter(indcfg); !ok {
			continue
		}
		wg.Add()
		go s.processsearch(wg, searchtype, indcfg, titlesearch, sourcealttitles, thetvdbid, season, useseason, e)
	}
	wg.Wait()
	wg.Close()
}

func (s *ConfigSearcher) processsearch(wg *pool.SizedWaitGroup, searchtype int, indcfg *config.IndexersConfig, titlesearch bool, sourcealttitles []database.DbstaticTwoStringOneInt, thetvdbid int, season string, useseason bool, e *apiexternal.Nzbwithprio) {
	defer logger.HandlePanic()
	defer wg.Done()
	var done bool
	switch searchtype {
	case 1:
		done = s.runmediasearch(indcfg, titlesearch, e, sourcealttitles)
	case 2:
		done = s.runrsssearch(indcfg)
	case 3:
		done = s.runrssseasonsearch(indcfg, thetvdbid, season, useseason)
	}
	if !s.Done && done {
		s.Done = done
	}
}

// Download iterates through the Accepted list and starts downloading each entry,
// tracking entries already downloaded to avoid duplicates. It handles both movies
// and TV series based on config and entry details.
func (s *ConfigSearcher) Download() {
	if len(s.Accepted) == 0 {
		return
	}
	downloaded := make([]uint, 0, len(s.Accepted))

	for idx := range s.Accepted {
		if s.Accepted[idx].Info.Priority == 0 {
			logger.LogDynamicany("error", "download not wanted", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &s.Accepted[idx].NZB.Title)
			continue
		}
		if s.checkdownloaded(downloaded, idx) {
			continue
		}
		qualcfg := s.getentryquality(&s.Accepted[idx])
		if qualcfg == nil {
			logger.LogDynamicany("info", "nzb found - start downloading", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &s.Accepted[idx].NZB.Title, "minimum prio", &s.Accepted[idx].MinimumPriority, &logger.StrPriority, &s.Accepted[idx].Info.Priority)
		} else {
			logger.LogDynamicany("info", "nzb found - start downloading", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &s.Accepted[idx].NZB.Title, "quality", &qualcfg.Name, "minimum prio", &s.Accepted[idx].MinimumPriority, &logger.StrPriority, &s.Accepted[idx].Info.Priority)
		}
		if !s.Cfgp.Useseries && s.Accepted[idx].NzbmovieID != 0 {
			downloaded = append(downloaded, s.Accepted[idx].NzbmovieID)
			downloader.DownloadMovie(s.Cfgp, &s.Accepted[idx])
		} else if s.Cfgp.Useseries && s.Accepted[idx].NzbepisodeID != 0 {
			downloaded = append(downloaded, s.Accepted[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(s.Cfgp, &s.Accepted[idx])
		} else if s.Accepted[idx].NzbmovieID != 0 {
			downloaded = append(downloaded, s.Accepted[idx].NzbmovieID)
			downloader.DownloadMovie(s.Cfgp, &s.Accepted[idx])
		} else if s.Accepted[idx].NzbepisodeID != 0 {
			downloaded = append(downloaded, s.Accepted[idx].NzbepisodeID)
			downloader.DownloadSeriesEpisode(s.Cfgp, &s.Accepted[idx])
		}
	}
}

// checkdownloaded checks if the entry at index idx has already been downloaded
// by looking for its movie ID or episode ID in the downloaded slice.
// It returns true if the entry is already in downloaded.
func (s *ConfigSearcher) checkdownloaded(downloaded []uint, idx int) bool {
	for idxi := range downloaded {
		if s.Accepted[idx].NzbmovieID != 0 && downloaded[idxi] == s.Accepted[idx].NzbmovieID {
			return true
		}
		if s.Accepted[idx].NzbepisodeID != 0 && downloaded[idxi] == s.Accepted[idx].NzbepisodeID {
			return true
		}
	}
	return false
}

// filterTestQualityWanted checks if the quality attributes of the
// Nzbwithprio entry match the wanted quality configuration. It returns
// true if any unwanted quality is found to stop further processing of
// the entry.
func (s *ConfigSearcher) filterTestQualityWanted(entry *apiexternal.Nzbwithprio, quality *config.QualityConfig) bool {
	if quality == nil {
		return false
	}
	if quality.WantedResolutionLen >= 1 && entry.Info.Resolution != "" {
		if !logger.SlicesContainsI(quality.WantedResolution, entry.Info.Resolution) {
			s.logdenied("unwanted Resolution", entry, &strResolution, &entry.Info.Resolution, &logger.StrWanted, &quality.WantedResolution)
			return true
		}
	}

	if quality.WantedQualityLen >= 1 && entry.Info.Quality != "" {
		if !logger.SlicesContainsI(quality.WantedQuality, entry.Info.Quality) {
			s.logdenied("unwanted Quality", entry, &strQuality, &entry.Info.Quality, &logger.StrWanted, &quality.WantedQuality)
			return true
		}
	}

	if quality.WantedAudioLen >= 1 && entry.Info.Audio != "" {
		if !logger.SlicesContainsI(quality.WantedAudio, entry.Info.Audio) {
			s.logdenied("unwanted Audio", entry, &strAudio, &entry.Info.Audio, &logger.StrWanted, &quality.WantedAudio)
			return true
		}
	}

	if quality.WantedCodecLen >= 1 && entry.Info.Codec != "" {
		if !logger.SlicesContainsI(quality.WantedCodec, entry.Info.Codec) {
			s.logdenied("unwanted Codec", entry, &strCodec, &entry.Info.Codec, &logger.StrWanted, &quality.WantedCodec)
			return true
		}
	}
	return false
}

// Close closes the ConfigSearcher, including closing any open connections and clearing resources.
func (s *ConfigSearcher) Close() {
	if s == nil {
		return
	}
	plsearcher.Put(s)
}

// searchparse parses the raw search results, runs validation on each entry, assigns quality
// profiles and priorities, separates accepted and denied entries, and sorts accepted entries
// by priority
func (s *ConfigSearcher) searchparse(e *apiexternal.Nzbwithprio, alttitles []database.DbstaticTwoStringOneInt) {
	if len(s.Raw.Arr) == 0 {
		return
	}
	s.Denied = s.Raw.Arr
	s.Denied = s.Denied[:0]
	s.Accepted = s.Raw.Arr
	s.Accepted = s.Accepted[:0]
	var err error
	for idxraw := range s.Raw.Arr {
		if s.Raw.Arr[idxraw].NZB.DownloadURL == "" {
			s.logdenied("no url", &s.Raw.Arr[idxraw])
			continue
		}
		if s.Raw.Arr[idxraw].NZB.Title == "" {
			s.logdenied("no title", &s.Raw.Arr[idxraw])
			continue
		}
		if s.Raw.Arr[idxraw].NZB.Title != "" && (s.Raw.Arr[idxraw].NZB.Title[:1] == logger.StrSpace || s.Raw.Arr[idxraw].NZB.Title[len(s.Raw.Arr[idxraw].NZB.Title)-1:] == logger.StrSpace) {
			s.Raw.Arr[idxraw].NZB.Title = strings.Trim(s.Raw.Arr[idxraw].NZB.Title, logger.StrSpace)
		}
		if len(s.Raw.Arr[idxraw].NZB.Title) <= 3 {
			s.logdenied("short title", &s.Raw.Arr[idxraw])
			continue
		}
		if s.checkprocessed(&s.Raw.Arr[idxraw]) {
			continue
		}
		//Check Size
		if s.Raw.Arr[idxraw].NZB.Indexer != nil {
			skipemptysize := false
			indcfg := s.Quality.QualityIndexerByQualityAndTemplate(s.Raw.Arr[idxraw].NZB.Indexer)
			if indcfg != nil {
				skipemptysize = indcfg.SkipEmptySize
			}
			if !skipemptysize {
				if _, ok := config.SettingsList[s.Raw.Arr[idxraw].NZB.Indexer.Name]; ok {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				} else if s.Raw.Arr[idxraw].NZB.Indexer.Getlistbyindexer() != nil {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				}
			}
			if skipemptysize && s.Raw.Arr[idxraw].NZB.Size == 0 {
				s.logdenied("no size", &s.Raw.Arr[idxraw])
				continue
			}
		}

		//check history
		if s.filterSizeNzbs(&s.Raw.Arr[idxraw]) {
			continue
		}
		if s.checkcorrectid(e, &s.Raw.Arr[idxraw]) {
			continue
		}

		parser.ParseFileP(s.Raw.Arr[idxraw].NZB.Title, false, false, s.Cfgp, -1, &s.Raw.Arr[idxraw].Info)
		//if s.searchActionType == logger.StrRss {
		if !s.Cfgp.Useseries && !s.Raw.Arr[idxraw].NZB.Indexer.TrustWithIMDBIDs {
			s.Raw.Arr[idxraw].Info.Imdb = ""
		}
		if s.Cfgp.Useseries && !s.Raw.Arr[idxraw].NZB.Indexer.TrustWithTVDBIDs {
			s.Raw.Arr[idxraw].Info.Tvdb = ""
		}
		//}
		if s.searchActionType == logger.StrRss && e != nil {
			e.Close()
		}
		err = parser.GetDBIDs(&s.Raw.Arr[idxraw].Info, s.Cfgp, true)
		if err != nil {
			s.logdenied(err.Error(), &s.Raw.Arr[idxraw])
			continue
		}
		var qual *config.QualityConfig
		var skip bool
		if s.searchActionType == logger.StrRss {
			skip, qual = s.getmediadatarss(&s.Raw.Arr[idxraw], -1, false)
			if skip {
				continue
			}
		} else {
			if s.getmediadata(e, &s.Raw.Arr[idxraw]) {
				continue
			}
			qual = s.Quality
			s.Raw.Arr[idxraw].WantedAlternates = alttitles
		}
		//needs the identifier from getmediadata

		if qual == nil {
			qual = s.getentryquality(&s.Raw.Arr[idxraw])
		}
		if qual == nil {
			s.logdenied("unknown Quality", &s.Raw.Arr[idxraw])
			continue
		}
		if s.checkhistory(&s.Raw.Arr[idxraw], qual) {
			continue
		}
		if s.checkepisode(e, &s.Raw.Arr[idxraw]) {
			continue
		}

		if s.filterRegexNzbs(&s.Raw.Arr[idxraw], qual) {
			continue
		}

		if s.Raw.Arr[idxraw].Info.Priority == 0 {
			parser.GetPriorityMapQual(&s.Raw.Arr[idxraw].Info, s.Cfgp, qual, false, true)

			if s.Raw.Arr[idxraw].Info.Priority == 0 {
				s.logdenied("unknown Prio", &s.Raw.Arr[idxraw], &logger.StrFound, &s.Raw.Arr[idxraw].Info)
				continue
			}
		}

		s.Raw.Arr[idxraw].Info.StripTitlePrefixPostfixGetQual(qual)

		//check quality
		if s.filterTestQualityWanted(&s.Raw.Arr[idxraw], qual) {
			continue
		}
		//check priority

		if s.getminimumpriority(&s.Raw.Arr[idxraw], qual) {
			continue
		}
		if s.Raw.Arr[idxraw].MinimumPriority != 0 && s.Raw.Arr[idxraw].MinimumPriority == s.Raw.Arr[idxraw].Info.Priority {
			s.logdenied("same Prio", &s.Raw.Arr[idxraw])
			continue
		}

		if s.Raw.Arr[idxraw].MinimumPriority != 0 {
			if qual.UseForPriorityMinDifference == 0 && s.Raw.Arr[idxraw].Info.Priority <= s.Raw.Arr[idxraw].MinimumPriority {
				s.logdenied("lower Prio", &s.Raw.Arr[idxraw], &logger.StrFound, &s.Raw.Arr[idxraw].Info.Priority, &logger.StrWanted, &s.Raw.Arr[idxraw].MinimumPriority)
				continue
			}
			if qual.UseForPriorityMinDifference != 0 && s.Raw.Arr[idxraw].Info.Priority <= (s.Raw.Arr[idxraw].MinimumPriority+qual.UseForPriorityMinDifference) {
				s.logdenied("lower Prio", &s.Raw.Arr[idxraw], &logger.StrFound, &s.Raw.Arr[idxraw].Info.Priority, &logger.StrWanted, &s.Raw.Arr[idxraw].MinimumPriority)
				continue
			}
		}

		if s.checkyear(e, &s.Raw.Arr[idxraw], qual) {
			continue
		}

		if s.checktitle(&s.Raw.Arr[idxraw], qual) {
			continue
		}
		logger.LogDynamicany("debug", "Release ok", "quality", &qual.Name, &logger.StrTitle, &s.Raw.Arr[idxraw].NZB.Title, "minimum prio", &s.Raw.Arr[idxraw].MinimumPriority, &logger.StrPriority, &s.Raw.Arr[idxraw].Info.Priority)

		s.Accepted = append(s.Accepted, s.Raw.Arr[idxraw])

		if qual.CheckUntilFirstFound {
			break
		}
	}
	if database.DBLogLevel == logger.StrDebug {
		logger.LogDynamicany("debug", "Entries found", "Count", len(s.Raw.Arr))
	}
	if len(s.Accepted) > 1 {
		slices.SortFunc(s.Accepted, func(a, b apiexternal.Nzbwithprio) int {
			return cmp.Compare(a.Info.Priority, b.Info.Priority)
		})
		// sort.Slice(s.Accepted, func(i, j int) bool {
		// 	return s.Accepted[i].Info.Priority > s.Accepted[j].Info.Priority
		// })
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
			s.logdenied("disabled Upgrade", entry)
			return true
		}
	} else {
		if entry.DontSearch {
			s.logdenied("disabled Search", entry)
			return true
		}
	}
	return false
}

// checkcorrectid checks if the entry matches the expected ID based on
// whether it is a movie or series. For movies it checks the IMDB ID,
// trimming any "t0" prefix. For series it checks the TVDB ID. If the
// IDs don't match, it logs a message and returns true to skip the entry.
func (s *ConfigSearcher) checkcorrectid(sourceentry *apiexternal.Nzbwithprio, entry *apiexternal.Nzbwithprio) bool {
	if s.searchActionType == logger.StrRss {
		return false
	}
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}
	if !s.Cfgp.Useseries {
		if entry.NZB.IMDBID != "" && sourceentry.Info.Imdb != "" && sourceentry.Info.Imdb != entry.NZB.IMDBID {
			if strings.TrimLeft(sourceentry.Info.Imdb, "t0") != strings.TrimLeft(entry.NZB.IMDBID, "t0") {
				s.logdenied("not matched imdb", entry, &logger.StrWanted, &sourceentry.Info.Imdb, &logger.StrFound, &entry.NZB.IMDBID)
				return true
			}
		}
		return false
	}
	if sourceentry.NZB.TVDBID != 0 && entry.NZB.TVDBID != 0 && sourceentry.NZB.TVDBID != entry.NZB.TVDBID {
		s.logdenied("not matched tvdb", entry, &logger.StrWanted, &sourceentry.NZB.TVDBID, &logger.StrFound, &entry.NZB.TVDBID)
		return true
	}
	return false
}

// getmediadata validates the media data in the given entry against the
// source entry to determine if it is a match. It sets various priority
// and search control fields on the entry based on the source entry
// configuration. Returns true to skip/reject the entry if no match, false
// to continue processing if a match.
func (s *ConfigSearcher) getmediadata(sourceentry *apiexternal.Nzbwithprio, entry *apiexternal.Nzbwithprio) bool {
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}
	if !s.Cfgp.Useseries {
		if sourceentry.NzbmovieID != entry.Info.MovieID {
			s.logdenied("unwanted Movie", entry, &logger.StrFound, &entry.Info.MovieID, &logger.StrWanted, &sourceentry.NzbmovieID, &logger.StrImdb, &sourceentry.Info.Imdb, &logger.StrConfig, &s.Cfgp.NamePrefix)
			return true
		}
		entry.NzbmovieID = sourceentry.NzbmovieID
		entry.Dbid = sourceentry.Dbid
		entry.MinimumPriority = sourceentry.MinimumPriority
		entry.DontSearch = sourceentry.DontSearch
		entry.DontUpgrade = sourceentry.DontUpgrade
		entry.WantedTitle = sourceentry.WantedTitle
		return false
	}

	//Parse Series
	if entry.Info.SerieEpisodeID != sourceentry.NzbepisodeID {
		s.logdenied("unwanted Episode", entry, &logger.StrFound, &entry.Info.SerieEpisodeID, &logger.StrWanted, &sourceentry.NzbepisodeID, &strIdentifier, &sourceentry.Info.Identifier, &logger.StrConfig, &s.Cfgp.NamePrefix)
		return true
	}
	entry.NzbepisodeID = sourceentry.NzbepisodeID
	entry.Dbid = sourceentry.Dbid
	entry.MinimumPriority = sourceentry.MinimumPriority
	entry.DontSearch = sourceentry.DontSearch
	entry.DontUpgrade = sourceentry.DontUpgrade
	entry.WantedTitle = sourceentry.WantedTitle

	return false
}

// getmediadatarss processes an Nzbwithprio entry for adding to the RSS feed.
// It handles movie and series entries differently based on ConfigSearcher.Cfgp.Useseries.
// For movies, it tries to add the entry to the list with ID addinlistid, or adds it if addifnotfound is true.
// For series, it calls getserierss to filter the entry.
// It returns true if the entry should be skipped.
func (s *ConfigSearcher) getmediadatarss(entry *apiexternal.Nzbwithprio, addinlistid int8, addifnotfound bool) (bool, *config.QualityConfig) {
	if !s.Cfgp.Useseries {
		if addinlistid != -1 && s.Cfgp != nil {
			entry.Info.ListID = addinlistid
		}

		return s.getmovierss(entry, addinlistid, addifnotfound)
	}

	//Parse Series
	//Filter RSS Series
	//return s.getserierss(entry)
	if entry.Info.SerieID == 0 {
		s.logdenied("unwanted Serie", entry)
		return true, nil
	}
	if entry.Info.DbserieID == 0 {
		s.logdenied("unwanted DBSerie", entry)
		return true, nil
	}
	if entry.Info.DbserieEpisodeID == 0 {
		s.logdenied("unwanted DBEpisode", entry)
		return true, nil
	}
	if entry.Info.SerieEpisodeID == 0 {
		s.logdenied("unwanted Episode", entry)
		return true, nil
	}
	entry.NzbepisodeID = entry.Info.SerieEpisodeID
	entry.Dbid = entry.Info.DbserieID

	database.GetdatarowArgs("select serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.seriename from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id where serie_episodes.id = ?", &entry.Info.SerieEpisodeID, &entry.DontSearch, &entry.DontUpgrade, &entry.Quality, &entry.Listname, &entry.WantedTitle)
	entry.Info.ListID = s.Cfgp.GetMediaListsEntryListID(entry.Listname)
	if entry.Quality == "" {
		return false, config.SettingsQuality[s.Cfgp.DefaultQuality]
	}
	return false, config.SettingsQuality[entry.Quality]
}

// checkyear validates the year in the entry title against the year
// configured for the wanted entry. It returns false if a match is found,
// or true to skip the entry if no match is found. This is used during
// search result processing to filter entries by year.
func (s *ConfigSearcher) checkyear(sourceentry *apiexternal.Nzbwithprio, entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	if s.Cfgp.Useseries || s.searchActionType == logger.StrRss {
		return false
	}
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return false
	}

	if sourceentry.Info.Year == 0 {
		s.logdenied("no year", entry)
		return true
	}
	if (qual.CheckYear || qual.CheckYear1) && logger.ContainsInt(entry.NZB.Title, sourceentry.Info.Year) {
		return false
	}
	if qual.CheckYear1 && logger.ContainsInt(entry.NZB.Title, sourceentry.Info.Year+1) {
		return false
	}
	if qual.CheckYear1 && logger.ContainsInt(entry.NZB.Title, sourceentry.Info.Year-1) {
		return false
	}
	s.logdenied("unwanted Year", entry, &logger.StrWanted, &sourceentry.Info.Year)
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
		entry.Info.StripTitlePrefixPostfixGetQual(qual)
	}

	wantedslug := logger.StringToSlugBytes(entry.WantedTitle)
	//defer clear(wantedslug)
	if entry.WantedTitle != "" {
		if qual.CheckTitle && entry.WantedTitle != "" && database.ChecknzbtitleB(entry.WantedTitle, wantedslug, entry.Info.Title, qual.CheckYear1, entry.Info.Year) {
			return false
		}
	}
	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.Info.Title, ']') {
		for i := len(entry.Info.Title) - 1; i >= 0; i-- {
			if strings.EqualFold(entry.Info.Title[i:i+1], "]") {
				if i < (len(entry.Info.Title) - 1) {
					trytitle = strings.TrimLeft(entry.Info.Title[i+1:], "-. ")
					if qual.CheckTitle && entry.WantedTitle != "" && database.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle, qual.CheckYear1, entry.Info.Year) {
						return false
					}
				}
			}
		}
	}
	if entry.Dbid != 0 && len(entry.WantedAlternates) == 0 {
		entry.WantedAlternates = database.Getentryalternatetitlesdirect(&entry.Dbid, s.Cfgp.Useseries)
	}

	if entry.Info.Title == "" || len(entry.WantedAlternates) == 0 {
		s.logdenied("unwanted Title", entry)
		return true
	}
	for idx := range entry.WantedAlternates {
		if entry.WantedAlternates[idx].Str1 == "" {
			continue
		}
		if entry.WantedAlternates[idx].ChecknzbtitleC(entry.Info.Title, qual.CheckYear1, entry.Info.Year) {
			return false
		}

		if trytitle != "" {
			if entry.WantedAlternates[idx].ChecknzbtitleC(trytitle, qual.CheckYear1, entry.Info.Year) {
				return false
			}
		}
	}
	s.logdenied("unwanted Title and alternate", entry)
	return true
}

// checkepisode validates the episode identifier in the entry against the
// season and episode values. It returns false if the identifier matches the
// expected format, or true to skip the entry if the identifier is invalid.
func (s *ConfigSearcher) checkepisode(sourceentry *apiexternal.Nzbwithprio, entry *apiexternal.Nzbwithprio) bool {
	//Checkepisode
	if !s.Cfgp.Useseries {
		return false
	}
	if s.searchActionType == logger.StrRss {
		if sourceentry == nil {
			//s.logdenied("no sourceentry", entry)
			return false
		}

		if sourceentry.Info.Identifier == "" {
			return false
		}
	}
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return false
	}

	if sourceentry.Info.Identifier == "" {
		s.logdenied("no identifier", entry)
		return true
	}
	if logger.ContainsI(entry.NZB.Title, sourceentry.Info.Identifier) {
		return false
	}
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" && strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(sourceentry.Info.Identifier, '-', '.')) {
			return false
		}
		if logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(sourceentry.Info.Identifier, '-', ' ')) {
			return false
		}
	}
	// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
	altIdentifier := bytes.TrimLeft(logger.StringToByteArr(sourceentry.Info.Identifier), "sS0")
	if bytes.ContainsRune(altIdentifier, 'E') || bytes.ContainsRune(altIdentifier, 'e') {
		altIdentifier = logger.ByteReplaceWithByte(logger.ByteReplaceWithByte(altIdentifier, 'e', 'x'), 'E', 'x')
	}

	arrnzbtitle := logger.StringToByteArr(entry.NZB.Title)
	if logger.ContainsByteI(arrnzbtitle, altIdentifier) {
		return false
	}
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" && strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		if logger.ContainsByteI(arrnzbtitle, logger.ByteReplaceWithByte(altIdentifier, '-', '.')) {
			return false
		}
		if logger.ContainsByteI(arrnzbtitle, logger.ByteReplaceWithByte(altIdentifier, '-', ' ')) {
			return false
		}
	}

	if sourceentry.NZB.Season == "" || sourceentry.NZB.Episode == "" {
		s.logdenied("unwanted Identifier", entry, &strIdentifier, &sourceentry.Info.Identifier)
		return true
	}

	sprefix, eprefix := "s", "e"
	if logger.ContainsI(sourceentry.Info.Identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else if !logger.ContainsI(sourceentry.Info.Identifier, "s") && !logger.ContainsI(sourceentry.Info.Identifier, "e") {
		s.logdenied("unwanted Identifier", entry, &strIdentifier, &sourceentry.Info.Identifier)
		return true
	}

	if !logger.HasPrefixI(sourceentry.Info.Identifier, logger.JoinStrings(sprefix, sourceentry.NZB.Season)) {
		s.logdenied("unwanted Season", entry, &strIdentifier, &sourceentry.Info.Identifier)
		return true
	}
	if !logger.ContainsI(sourceentry.Info.Identifier, sourceentry.NZB.Episode) {
		s.logdenied("unwanted Identifier", entry, &strIdentifier, &sourceentry.Info.Identifier)
		return true
	}

	//suffixcheck
	for idxsub := range episodeprefixarray {
		if logger.HasSuffixI(sourceentry.Info.Identifier, logger.JoinStrings(eprefix, episodeprefixarray[idxsub], sourceentry.NZB.Episode)) {
			return false
		}
	}

	episodesuffixarray := []string{eprefix, logger.StrSpace, logger.StrDash}
	firstpart := logger.JoinStrings(eprefix, sourceentry.NZB.Episode)
	for idx := range episodesuffixarray {
		if logger.ContainsI(sourceentry.Info.Identifier, logger.JoinStrings(firstpart, episodesuffixarray[idx])) {
			return false
		}
		for idxsub := range episodeprefixarray {
			if logger.HasSuffixI(sourceentry.Info.Identifier, logger.JoinStrings(eprefix, episodeprefixarray[idxsub], logger.JoinStrings(sourceentry.NZB.Episode, episodesuffixarray[idx]))) {
				return false
			}
		}
	}

	s.logdenied("unwanted Identifier", entry, &strIdentifier, &sourceentry.Info.Identifier)
	return true
}

// getmovierss validates the movie data in the entry, sets additional fields,
// queries the database for movie data like dontSearch/dontUpgrade flags,
// and returns false to continue search result processing or true to skip the entry.
func (s *ConfigSearcher) getmovierss(entry *apiexternal.Nzbwithprio, addinlistid int8, addifnotfound bool) (bool, *config.QualityConfig) {
	//Add DbMovie if not found yet and enabled
	if entry.Info.DbmovieID == 0 && (!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
		s.logdenied("unwanted DBMovie", entry)
		return true, nil
	}
	//add movie if not found
	if addifnotfound && (entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0) && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if addinlistid == -1 {
			return true, nil
		}
		bl, err := importfeed.AllowMovieImport(entry.NZB.IMDBID, s.Cfgp.Lists[addinlistid].CfgList)
		if err != nil {
			s.logdenied(err.Error(), entry)
			return true, nil
		}
		if !bl {
			s.logdenied("unallowed DBMovie", entry)
			return true, nil
		}
	}
	if addifnotfound && entry.Info.DbmovieID == 0 && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		var err error
		entry.Info.DbmovieID, err = importfeed.JobImportMovies(entry.NZB.IMDBID, s.Cfgp, addinlistid, true)
		if err != nil {
			s.logdenied(err.Error(), entry)
			return true, nil
		}
		database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
		if entry.Info.MovieID == 0 || entry.Info.DbmovieID == 0 {
			s.logdenied("unwanted Movie", entry)
			return true, nil
		}
	}
	if entry.Info.DbmovieID == 0 {
		s.logdenied("unwanted DBMovie", entry)
		return true, nil
	}

	//continue only if dbmovie found
	//Get List of movie by dbmovieid, year and possible lists

	//if list was not found : should we add the movie to the list?
	if addifnotfound && entry.Info.MovieID == 0 && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if addinlistid == -1 {
			s.logdenied("no addinlist", entry)
			return true, nil
		}

		err := importfeed.Checkaddmovieentry(&entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid], entry.NZB.IMDBID)
		if err != nil {
			s.logdenied(err.Error(), entry)
			return true, nil
		}
		if entry.Info.DbmovieID != 0 && entry.Info.MovieID == 0 {
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
		}
		if entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0 {
			s.logdenied("unwanted Movie", entry)
			return true, nil
		}
	}

	if entry.Info.MovieID == 0 {
		s.logdenied("unwanted Movie", entry)
		return true, nil
	}
	entry.Dbid = entry.Info.DbmovieID
	entry.NzbmovieID = entry.Info.MovieID

	database.GetdatarowArgs("select movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?", &entry.Info.MovieID, &entry.DontSearch, &entry.DontUpgrade, &entry.Listname, &entry.Quality, &entry.WantedTitle)

	entry.Info.ListID = s.Cfgp.GetMediaListsEntryListID(entry.Listname)
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
	if entry.Info.MovieID != 0 {
		return database.GetMediaQualityConfig(s.Cfgp, entry.Info.MovieID)
	}
	if entry.Info.SerieEpisodeID != 0 {
		return database.GetMediaQualityConfig(s.Cfgp, entry.Info.SerieEpisodeID)
	}
	return nil
}

// GetRSSFeed queries the RSS feed for the given media list, searches for and downloads new items,
// and adds them to the search results. It handles checking if the indexer is blocked,
// configuring the custom RSS feed URL, getting the last ID to prevent duplicates,
// parsing results, and updating the RSS history.
func (s *ConfigSearcher) getRSSFeed(listentry *config.MediaListsConfig, downloadentries bool, autoclose bool) error {
	if autoclose {
		defer s.Close()
	}
	if listentry.TemplateList == "" {
		return logger.ErrListnameTemplateEmpty
	}

	s.searchActionType = logger.StrRss

	intid := -1
	for idx := range s.Quality.Indexer {
		if s.Quality.Indexer[idx].TemplateIndexer == listentry.TemplateList || strings.EqualFold(s.Quality.Indexer[idx].TemplateIndexer, listentry.TemplateList) {
			intid = idx
			break
		}
	}
	if intid != -1 && s.Quality.Indexer[intid].TemplateRegex == "" {
		return errRegexEmpty
	}

	blockinterval := -5
	if config.SettingsGeneral.FailedIndexerBlockTime != 0 {
		blockinterval = -1 * config.SettingsGeneral.FailedIndexerBlockTime
	}

	intval := logger.TimeGetNow().Add(time.Minute * time.Duration(blockinterval))
	if database.GetdatarowN[uint](false, "select count() from indexer_fails where indexer = ? and last_fail > ?", &listentry.CfgList.URL, &intval) >= 1 {
		logger.LogDynamicany("debug", "Indexer temporarily disabled due to fail in last", &logger.StrListname, &listentry.TemplateList, "Minutes", &blockinterval)
		return logger.ErrDisabled
	}

	if s.Cfgp == nil {
		return errOther
	}

	customindexer := *config.SettingsIndexer[listentry.TemplateList]
	customindexer.Name = listentry.TemplateList
	customindexer.Customrssurl = listentry.CfgList.URL
	customindexer.URL = listentry.CfgList.URL
	customindexer.MaxEntries = logger.StringToUInt16(listentry.CfgList.Limit)
	_, firstid, err := apiexternal.QueryNewznabRSSLastCustom(&customindexer, s.Quality, database.GetdatarowN[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE", &listentry.TemplateList, &s.Quality.Name), nil, &s.Raw)
	if err != nil {
		return err
	}
	if len(s.Raw.Arr) >= 1 {
		if firstid != "" {
			addrsshistory(&listentry.CfgList.URL, &firstid, s.Quality, &listentry.TemplateList)
		}
		s.searchparse(nil, nil)

		if downloadentries {
			s.Download()
		}
	}
	return nil
}

// MovieFillSearchVar fills the search variables for the given movie ID.
// It queries the database to get the movie details and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) movieFillSearchVar(movieid *uint, e *apiexternal.Nzbwithprio) error {
	database.GetdatarowArgs("select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?", movieid, &e.Dbid, &e.DontSearch, &e.DontUpgrade, &e.Listname, &e.Quality, &e.Info.Year, &e.Info.Imdb, &e.WantedTitle)
	if e.Dbid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	if e.DontSearch {
		return logger.ErrDisabled
	}

	e.NzbmovieID = *movieid
	if s.Quality == nil || s.Quality.Name != e.Quality {
		if e.Quality == "" {
			s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
		} else {
			var ok bool
			s.Quality, ok = config.SettingsQuality[e.Quality]
			if !ok {
				s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
			}
		}
	}
	e.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, movieid, false, -1, s.Quality)

	var err error
	s.searchActionType, err = getsearchtype(e.MinimumPriority, e.DontUpgrade, false)
	if err != nil {
		return err
	}

	if e.Info.Year == 0 {
		return errYearEmpty
	}
	e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(e.Listname)

	return nil
}

// EpisodeFillSearchVar fills the search variables for the given episode ID.
// It queries the database to get the necessary data and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) episodeFillSearchVar(episodeid *uint, e *apiexternal.Nzbwithprio) error {
	//dbserie_episode_id, dbserie_id, serie_id, dont_search, dont_upgrade
	database.GetdatarowArgs("select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?", episodeid, &e.Info.DbserieEpisodeID, &e.Dbid, &e.Info.SerieID, &e.DontSearch, &e.DontUpgrade, &e.Quality, &e.Listname, &e.NZB.TVDBID, &e.WantedTitle, &e.NZB.Season, &e.NZB.Episode, &e.Info.Identifier)
	if e.Info.DbserieEpisodeID == 0 || e.Dbid == 0 || e.Info.SerieID == 0 {
		return logger.ErrNotFoundDbserie
	}
	if e.DontSearch {
		return logger.ErrDisabled
	}
	e.Info.DbserieID = e.Dbid

	e.NzbepisodeID = *episodeid
	if s.Quality == nil || s.Quality.Name != e.Quality {
		if e.Quality == "" {
			s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
		} else {
			var ok bool
			s.Quality, ok = config.SettingsQuality[e.Quality]
			if !ok {
				s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
			}
		}
	}
	e.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, episodeid, false, -1, s.Quality)

	e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(e.Listname)
	var err error
	s.searchActionType, err = getsearchtype(e.MinimumPriority, e.DontUpgrade, false)
	if err != nil {
		return err
	}
	return nil
}

// // Getsearchalternatetitles returns alternate search titles to use when searching for media.
// // It takes in alternate titles from the database, a flag indicating if this is already a title search,
// // and the quality configuration. It returns a slice of string alternate titles.
// func (s *ConfigSearcher) getsearchalternatetitles(titlesearch bool, sourcealttitles []database.DbstaticTwoStringOneInt, qualcfg *config.QualityConfig) []database.DbstaticTwoStringOneInt {
// 	i := 2
// 	if qualcfg.BackupSearchForAlternateTitle {
// 		i += len(sourcealttitles)
// 	}
// 	n := make([]database.DbstaticTwoStringOneInt, 0, i)
// 	if qualcfg.BackupSearchForAlternateTitle {
// 		if qualcfg.BackupSearchForTitle {
// 			if !titlesearch {
// 				n = append(n, database.DbstaticTwoStringOneInt{})
// 			}
// 			n = append(n, database.DbstaticTwoStringOneInt{Str1: e.WantedTitle})
// 			return append(n, sourcealttitles...)
// 		}
// 		if titlesearch && !qualcfg.BackupSearchForTitle {
// 			return append(n, sourcealttitles...)
// 		}
// 	}
// 	if qualcfg.BackupSearchForTitle {
// 		if !titlesearch {
// 			n = append(n, database.DbstaticTwoStringOneInt{})
// 		}
// 		return append(n, database.DbstaticTwoStringOneInt{Str1: e.WantedTitle})
// 	}
// 	if !titlesearch {
// 		return append(n, database.DbstaticTwoStringOneInt{})
// 	}
// 	return append(n, database.DbstaticTwoStringOneInt{Str1: e.WantedTitle})
// }

// filterSizeNzbs checks if the NZB entry size is within the configured
// minimum and maximum size limits, and returns true if it should be
// rejected based on its size.
func (s *ConfigSearcher) filterSizeNzbs(entry *apiexternal.Nzbwithprio) bool {
	for idxdataimport := range s.Cfgp.DataImport {
		if s.Cfgp.DataImport[idxdataimport].CfgPath == nil {
			continue
		}
		if s.Cfgp.DataImport[idxdataimport].CfgPath.MinSize != 0 && entry.NZB.Size < s.Cfgp.DataImport[idxdataimport].CfgPath.MinSizeByte {
			s.logdenied("too small", entry, &logger.StrFound, &entry.NZB.Size)
			return true
		}

		if s.Cfgp.DataImport[idxdataimport].CfgPath.MaxSize != 0 && entry.NZB.Size > s.Cfgp.DataImport[idxdataimport].CfgPath.MaxSizeByte {
			s.logdenied("too big", entry, &logger.StrFound, &entry.NZB.Size)
			return true
		}
	}
	return false
}

// filterRegexNzbs checks if the given NZB entry matches the required regexes
// and does not match any rejected regexes from the quality configuration.
// Returns true if the entry fails the regex checks, false if it passes.
func (s *ConfigSearcher) filterRegexNzbs(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	regexcfg := entry.Getregexcfg(qual)
	if regexcfg == nil {
		s.logdenied("Denied by Regex", entry, "regex_template empty")
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
		s.logdenied("not matched required", entry, &strCheckedFor, &regexcfg.Required[0])
		return true
	}

	for idx := range regexcfg.Rejected {
		if database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.NZB.Title, 1) {
			if !database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.WantedTitle, 1) {
				bl := false
				for idxi := range entry.WantedAlternates {
					if database.RegexGetMatchesFind(regexcfg.Rejected[idx], entry.WantedAlternates[idxi].Str1, 1) {
						bl = true
						break
					}
				}
				if !bl {
					s.logdenied("Denied by Regex", entry, &strRejectedby, &regexcfg.Rejected[idx])
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
		if database.CheckcachedUrlHistory(s.Cfgp.Useseries, &entry.NZB.DownloadURL) {
			s.logdenied("already downloaded url", entry)
			return true
		}
	}
	if entry.NZB.Indexer == nil {
		return false
	}
	indcfg := qual.QualityIndexerByQualityAndTemplate(entry.NZB.Indexer)
	if indcfg != nil {
		if !indcfg.HistoryCheckTitle {
			return false
		}
	}

	if database.CheckcachedTitleHistory(s.Cfgp.Useseries, &entry.NZB.Title) {
		s.logdenied("already downloaded title", entry)
		return true
	}
	return false
}

// checkprocessed checks if the given entry is already in the denied or accepted lists to avoid duplicate processing.
// It loops through the denied and accepted entries and returns true if it finds a match on the download URL or title.
// Otherwise returns false. Part of ConfigSearcher.
func (s *ConfigSearcher) checkprocessed(entry *apiexternal.Nzbwithprio) bool {
	for idx := range s.Denied {
		if s.Denied[idx].NZB.DownloadURL == entry.NZB.DownloadURL {
			return true
		}
		if s.Denied[idx].NZB.Title == entry.NZB.Title {
			return true
		}
	}
	for idx := range s.Accepted {
		if s.Accepted[idx].NZB.DownloadURL == entry.NZB.DownloadURL {
			return true
		}
		if s.Accepted[idx].NZB.Title == entry.NZB.Title {
			return true
		}
	}
	return false
}

// logdenied logs a denied entry with the given reason and optional additional fields.
// It sets the reason and additional reason on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied(reason string, entry *apiexternal.Nzbwithprio, addfields ...any) {
	if reason != "" {
		vals := logger.PLArrAny.Get()
		defer logger.PLArrAny.Put(vals)
		if len(addfields) > 0 {
			vals.Arr = append(vals.Arr, addfields...)
		}
		vals.Arr = append(vals.Arr, &logger.StrReason, &entry.Reason, &logger.StrTitle, &entry.NZB.Title)

		entry.Reason = reason
		if len(addfields) > 0 {
			entry.AdditionalReason = addfields[1]
		}
		logger.LogDynamicany("debug", skippedstr, vals.Arr...)
	}
	s.Denied = append(s.Denied, *entry)
}

// NewSearcher creates a new ConfigSearcher instance.
// It initializes the searcher with the given media type config,
// quality config, search action type, and media ID.
// If no quality config is provided but a media ID is given,
// it will look up the quality config for that media in the database.
// It gets a searcher instance from the pool and sets the configs,
// then returns the initialized searcher.
func NewSearcher(cfgp *config.MediaTypeConfig, quality *config.QualityConfig, searchActionType string, mediaid uint) *ConfigSearcher {
	s := plsearcher.Get()
	//s.Dl = plsearchresults.Get()
	s.Cfgp = cfgp
	s.searchActionType = searchActionType
	if quality == nil {
		if mediaid != 0 {
			s.Quality = database.GetMediaQualityConfig(cfgp, mediaid)
		}
	} else {
		s.Quality = quality
	}
	return s
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

// Getnewznabrss queries Newznab indexers from the given MediaListsConfig
// using the provided MediaTypeConfig. It searches for and downloads any
// matching RSS feed items.
func Getnewznabrss(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig) error {
	if list.CfgList == nil || cfgp == nil {
		return logger.ErrNotFound
	}

	return NewSearcher(cfgp, list.CfgQuality, logger.StrRss, 0).getRSSFeed(list, true, true)
}

// SearchSerieRSSSeasonSingle searches for a single season of a series.
// It takes the series ID, season number, whether to search the full season or missing episodes,
// media type config, whether to auto close the results, and a pointer to search results.
// It returns a config searcher instance and error.
// It queries the database to map the series ID to thetvdb ID, gets the quality config,
// calls the search function, handles errors, downloads results,
// closes the results if autoclose is true, and returns the config searcher.
func SearchSerieRSSSeasonSingle(serieid *uint, season string, useseason bool, cfgp *config.MediaTypeConfig, autoclose bool, results *ConfigSearcher) (*ConfigSearcher, error) {
	if autoclose {
		defer results.Close()
	}
	tvdb := database.Getdatarow1[int](false, "select thetvdb_id from dbseries where id = ?", database.Getdatarow1[uint](false, "select dbserie_id from series where id = ?", serieid))
	if tvdb == 0 {
		return nil, logger.ErrTvdbEmpty
	}
	listid := database.GetMediaListIDGetListname(cfgp, serieid)
	if listid == -1 {
		return nil, logger.ErrListnameEmpty
	}

	err := results.searchSeriesRSSSeason(cfgp, cfgp.Lists[listid].CfgQuality, tvdb, season, useseason, true, false)
	if err != nil && !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamicany("error", "Season Search Inc Failed", err, &logger.StrID, serieid)

		return nil, err
	}
	if autoclose {
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
	searchseasons(cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20"), 20, "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )")
}

// SearchSeriesRSSSeasonsAll searches all seasons for series matching the given
// media type config. It searches series that have missing episodes and calls
// searchseasons to perform the actual search.
func SearchSeriesRSSSeasonsAll(cfgp *config.MediaTypeConfig) {
	searchseasons(cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1"), database.GetdatarowN[uint](false, "select count() from series"), "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )")
}

// searchseasons searches for missing episodes for series matching the given
// configuration and quality settings. It selects a random sample of series
// to search, gets the distinct seasons with missing episodes for each, and
// searches those seasons on the RSS feeds of the enabled indexers. Results
// are added to the passed in DownloadResults instance.
func searchseasons(cfgp *config.MediaTypeConfig, queryrange string, queryrangecount uint, queryseason string, queryseasoncount string) {
	args := logger.PLArrAny.Get()

	//args := make([]any, 0, len(cfgp.Lists))
	for idx := range cfgp.Lists {
		args.Arr = append(args.Arr, &cfgp.Lists[idx].Name)
	}
	tbl := database.GetrowsNuncached[database.DbstaticTwoUint](queryrangecount, queryrange, args.Arr)
	logger.PLArrAny.Put(args)
	if len(tbl) == 0 {
		return
	}

	var listid int8
	var arr []string
	for idx := range tbl {
		arr = database.GetrowsNsize[string](false, queryseasoncount, queryseason, &tbl[idx].Num2, &tbl[idx].Num1, &tbl[idx].Num1, &tbl[idx].Num2)
		for idx2 := range arr {
			listid = database.GetMediaListIDGetListname(cfgp, &tbl[idx].Num1)
			if listid == -1 {
				continue
			}
			NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, 0).searchSeriesRSSSeason(cfgp, cfgp.Lists[listid].CfgQuality, database.GetdatarowN[int](false, "select thetvdb_id from dbseries where id = ?", &tbl[idx].Num2), arr[idx2], true, true, true)
		}
		clear(arr)
	}
	//clear(tbl)
}

// Getpriobyfiles returns the minimum priority of existing files for the given media
// ID, and optionally returns a slice of file paths for existing files below
// the given wanted priority. If useseries is true it will look up series IDs instead of media IDs.
// If id is nil it will return 0 priority.
// If useall is true it will include files marked as deleted.
// If wantedprio is -1 it will not return any file paths.
func Getpriobyfiles(useseries bool, id *uint, useall bool, wantedprio int, qualcfg *config.QualityConfig) (int, []string) {
	if qualcfg == nil || *id == 0 {
		return 0, nil
	}

	var oldf []string
	arr := database.GetrowsNsize[database.FilePrio](false, logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID), logger.GetStringsMap(useseries, logger.DBFilePrioFilesByID), id)
	var minPrio, prio int
	for idx := range arr {
		prio = 0
		if !qualcfg.UseForPriorityResolution && !useall {
			arr[idx].ResolutionID = 0
		}
		if !qualcfg.UseForPriorityQuality && !useall {
			arr[idx].QualityID = 0
		}
		if !qualcfg.UseForPriorityAudio && !useall {
			arr[idx].AudioID = 0
		}
		if !qualcfg.UseForPriorityCodec && !useall {
			arr[idx].CodecID = 0
		}
		intid := parser.Findpriorityidxwanted(arr[idx].ResolutionID, arr[idx].QualityID, arr[idx].CodecID, arr[idx].AudioID, qualcfg)
		if intid != -1 {
			prio = parser.GetwantedArrPrio(intid)
		}
		if prio == 0 {
			intid = parser.Findpriorityidx(arr[idx].ResolutionID, arr[idx].QualityID, arr[idx].CodecID, arr[idx].AudioID, qualcfg)
			if intid != -1 {
				prio = parser.GetwantedArrPrio(intid)
			}
		}

		if intid == -1 {
			logger.LogDynamicany("debug", "prio not found", "searched for", parser.BuildPrioStr(arr[idx].ResolutionID, arr[idx].QualityID, arr[idx].CodecID, arr[idx].AudioID), "in", &qualcfg.Name)
			prio = 0
		}
		if !qualcfg.UseForPriorityOther && !useall {

		} else {
			if arr[idx].Proper {
				prio += 5
			}
			if arr[idx].Extended {
				prio += 2
			}
			if arr[idx].Repack {
				prio++
			}
		}

		//prio = parser.GetIDPrioritySimpleParse(&arr[idx], useseries, qualcfg, useall)

		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
		if wantedprio != -1 && wantedprio > prio {
			oldf = append(oldf, arr[idx].Location)
		}
	}
	//clear(arr)
	if wantedprio == -1 {
		return minPrio, nil
	}
	if len(oldf) == 0 {
		return minPrio, nil
	}
	return minPrio, oldf
}
