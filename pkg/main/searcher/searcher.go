package searcher

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// ConfigSearcher is a struct containing configuration and search results.
type ConfigSearcher struct {
	// Dl contains the search results
	Raw apiexternal.NzbSlice
	// Denied is a slice containing denied apiexternal.Nzbwithprio results
	Denied []apiexternal.Nzbwithprio
	// Accepted is a slice containing accepted apiexternal.Nzbwithprio results
	Accepted []apiexternal.Nzbwithprio
	// searchActionType is a string indicating the search action type
	searchActionType string // missing,upgrade,rss
	Done             bool
	// Cfgp is a pointer to a MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// Quality is a pointer to a QualityConfig
	Quality *config.QualityConfig
}

type searchParams struct {
	e               apiexternal.Nzbwithprio
	sourcealttitles []database.DbstaticTwoStringOneInt
	season          string
	searchtype      int
	thetvdbid       int
	mediaid         uint
	useseason       bool
	titlesearch     bool
}

const (
	skippedstr = "Skipped"
)

var (
	strRegexEmpty      = "regex_template empty"
	strMinutes         = "Minutes"
	strIdentifier      = "identifier"
	strCheckedFor      = "checked for"
	strTitlesearch     = "titlesearch"
	strRejectedby      = "rejected by"
	strMediaid         = "Media ID"
	episodeprefixarray = []string{"", logger.StrSpace, "0", " 0"}
	errOther           = errors.New("other error")
	errYearEmpty       = errors.New("year empty")
	errSearchvarEmpty  = errors.New("searchvar empty")
	errRegexEmpty      = errors.New("regex template empty")
	plsearcher         pool.Poolobj[ConfigSearcher]
	plsearchparam      pool.Poolobj[searchParams]
)

func Init() {
	plsearchparam.Init(5, nil, func(cs *searchParams) bool {
		*cs = searchParams{}
		return false
	})
	plsearcher.Init(10, func(cs *ConfigSearcher) {
		cs.Raw.Arr = make([]apiexternal.Nzbwithprio, 0, 6000)
	}, func(cs *ConfigSearcher) bool {
		cs.searchActionType = ""
		if len(cs.Denied) >= 1 {
			for i := range cs.Denied {
				cs.Denied[i].AdditionalReason = nil
				clear(cs.Denied[i].Info.Episodes)
				clear(cs.Denied[i].Info.Languages)
			}
			clear(cs.Denied)
			cs.Denied = cs.Denied[:0]
		}
		if len(cs.Accepted) >= 1 {
			for i := range cs.Accepted {
				cs.Accepted[i].AdditionalReason = nil
				clear(cs.Accepted[i].Info.Episodes)
				clear(cs.Accepted[i].Info.Languages)
			}
			clear(cs.Accepted)
			cs.Accepted = nil
		}
		if len(cs.Raw.Arr) >= 1 {
			for i := range cs.Raw.Arr {
				cs.Raw.Arr[i].AdditionalReason = nil
				clear(cs.Raw.Arr[i].Info.Episodes)
				clear(cs.Raw.Arr[i].Info.Languages)
			}
			clear(cs.Raw.Arr)
			cs.Raw.Arr = cs.Raw.Arr[:0]
		}
		cs.Done = false
		return false
	})
}

// SearchRSS searches the RSS feeds of the enabled Newznab indexers for the
// given media type and quality configuration. It returns a ConfigSearcher
// instance for managing the search, or an error if no search could be started.
// Results are added to the passed in DownloadResults instance.
func (s *ConfigSearcher) SearchRSS(ctx context.Context, cfgp *config.MediaTypeConfig, quality *config.QualityConfig, downloadresults, autoclose bool) error {
	if s == nil || autoclose {
		defer s.Close()
		if s == nil {
			return errSearchvarEmpty
		}
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	s.Quality = quality
	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.searchtype = 2
	s.searchindexers(ctx, true, p)
	if s.Done && len(s.Raw.Arr) >= 1 {
		s.searchparse(nil, nil)
		if downloadresults {
			s.Download()
		}
	}
	return nil
}

// searchSeriesRSSSeason searches configured indexers for the given TV series
// season using the RSS search APIs. It handles executing searches across
// enabled newznab indexers, parsing results, and adding accepted entries to
// the search results. Returns the searcher and error if any.
func (s *ConfigSearcher) searchSeriesRSSSeason(ctx context.Context, cfgp *config.MediaTypeConfig, quality *config.QualityConfig, thetvdbid int, season string, useseason, downloadentries, autoclose bool) error {
	if s == nil || autoclose {
		defer s.Close()
		if s == nil {
			return errSearchvarEmpty
		}
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	s.Quality = quality

	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.searchtype = 3
	p.thetvdbid = thetvdbid
	p.season = season
	p.useseason = useseason

	logger.LogDynamicany2StrAny("info", "Search for season", logger.StrSeason, p.season, logger.StrTvdb, &p.thetvdbid) // logpointerr
	s.searchindexers(ctx, false, p)
	if s.Done && len(s.Raw.Arr) >= 1 {
		s.searchparse(nil, nil)

		if downloadentries {
			s.Download()
		}
		la := len(s.Accepted)
		ld := len(s.Denied)
		logger.Logtype("info", 0).Int(logger.StrTvdb, p.thetvdbid).Str(logger.StrSeason, p.season).Int(logger.StrAccepted, la).Int(logger.StrDenied, ld).Msg("Ended Search for season")
	}
	return nil
}

// MediaSearch searches indexers for the given media entry (movie or TV episode)
// using the configured quality profile. It handles filling search variables,
// executing searches across enabled indexers, parsing results, and optionally
// downloading accepted entries. Returns the search results and error if any.
func (s *ConfigSearcher) MediaSearch(ctx context.Context, cfgp *config.MediaTypeConfig, mediaid uint, titlesearch, downloadentries, autoclose bool) error {
	if s == nil || autoclose {
		defer s.Close()
		if s == nil {
			return errSearchvarEmpty
		}
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	if mediaid == 0 {
		return errSearchvarEmpty
	}
	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.mediaid = mediaid
	p.titlesearch = titlesearch
	p.searchtype = 1
	var err error
	if cfgp.Useseries {
		p.e.NzbepisodeID = mediaid
		err = s.episodeFillSearchVar(p)
	} else {
		p.e.NzbmovieID = mediaid
		err = s.movieFillSearchVar(p)
	}
	if err != nil {
		if !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			logger.Logtype("error", 0).Uint(logger.StrID, mediaid).Err(err).Msg("Media Search Failed")
		}
		return err
	}

	if s.Quality == nil {
		return errSearchvarEmpty
	}

	s.searchlog("info", "Search for media id", p)
	if titlesearch || s.Quality.BackupSearchForTitle || s.Quality.BackupSearchForAlternateTitle {
		p.sourcealttitles = database.GetDbstaticTwoStringOneInt(database.Getentryalternatetitlesdirect(&p.e.Dbid, s.Cfgp.Useseries), p.e.Dbid)
	}
	s.searchindexers(ctx, false, p)
	if !s.Done {
		s.searchlog("error", "All searches failed", p)
		return nil
	}
	database.Exec1(logger.GetStringsMap(cfgp.Useseries, logger.UpdateMediaLastscan), &p.mediaid)

	if len(s.Raw.Arr) >= 1 {
		s.searchparse(&p.e, p.sourcealttitles)
		if downloadentries {
			s.Download()
		}
		if len(s.Accepted) >= 1 || len(s.Denied) >= 1 {
			s.searchlog("info", "Ended Search for media id", p)
		}
	}
	return nil
}

// searchlog logs information about the results of a media search, including the number of accepted and denied entries.
// The function takes the following parameters:
// - typev: a string indicating the type of log message (e.g. "info", "error")
// - msg: a string describing the search event
// - p: a pointer to a searchParams struct containing information about the search
func (s *ConfigSearcher) searchlog(typev, msg string, p *searchParams) {
	logv := logger.Logtype(typev, 1).Uint(logger.StrID, p.mediaid).Bool(logger.StrSeries, s.Cfgp.Useseries).Bool(strTitlesearch, p.titlesearch)
	if len(s.Accepted) >= 1 {
		logv.Int(logger.StrAccepted, len(s.Accepted))
	}
	if len(s.Denied) >= 1 {
		logv.Int(logger.StrDenied, len(s.Denied))
	}
	logv.Msg(msg)
}

// logsearcherror logs an error message with additional context about the media search that failed.
// The function takes the following parameters:
// - msg: a string describing the error that occurred
// - id: the ID of the media item that was being searched for
// - useseries: a boolean indicating whether the search was for a series
// - title: the title of the media item that was being searched for
// - err: the error that occurred during the search
func logsearcherror(msg string, id uint, useseries bool, title string, err error) {
	logger.Logtype("error", 1).Uint(strMediaid, id).Bool(logger.StrSeries, useseries).Str(logger.StrTitle, title).Err(err).Msg(msg)
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
func (s *ConfigSearcher) searchindexers(ctx context.Context, userss bool, p *searchParams) {
	if len(s.Quality.IndexerCfg) == 0 {
		return
	}
	pl := worker.WorkerPoolIndexer.NewGroupContext(ctx)
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
		if !apiexternal.NewznabCheckLimiter(indcfg) {
			continue
		}
		pl.SubmitErr(func() error {
			defer logger.HandlePanic()
			var done bool
			switch p.searchtype {
			case 1:
				done = s.searchnameid(p, indcfg)
			case 2:
				firstid, err := apiexternal.QueryNewznabRSSLast(indcfg, s.Quality, database.Getdatarow3[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", &s.Cfgp.NamePrefix, &s.Quality.Name, &indcfg.URL), s.Quality.QualityIndexerByQualityAndTemplate(indcfg), &s.Raw)
				if err == nil {
					if firstid != "" {
						addrsshistory(&indcfg.URL, &firstid, s.Quality, &s.Cfgp.NamePrefix)
					}
					done = true
				} else if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
					logger.LogDynamicany1StringErr("error", "Error searching indexer", err, logger.StrIndexer, indcfg.Name)
				}
			case 3:
				_, _, err := apiexternal.QueryNewznabTvTvdb(indcfg, s.Quality, p.thetvdbid, s.Quality.QualityIndexerByQualityAndTemplate(indcfg), p.season, "", p.useseason, false, &s.Raw)
				if err == nil {
					done = true
				} else if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
					logger.LogDynamicany1StringErr("error", "Error searching indexer", err, logger.StrIndexer, indcfg.Name)
				}
			}
			if done && !s.Done {
				s.Done = true
			}
			return nil
		})
	}
	pl.Wait()
}

// searchnameid is a method of the ConfigSearcher struct that performs a search for a media item
// by its name or ID. It checks various conditions to determine the appropriate search method,
// such as whether to use a query search or a search by ID, and whether to search for a movie
// or a TV series. It also handles errors that may occur during the search and logs them.
// The method returns a boolean indicating whether the search was successful.
func (s *ConfigSearcher) searchnameid(p *searchParams, indcfg *config.IndexersConfig) (done bool) {
	cats := s.Quality.QualityIndexerByQualityAndTemplate(indcfg)
	if cats == -1 {
		logger.LogDynamicany0("error", "Error getting quality config")
		return false
	}

	usequerysearch := true
	if !p.titlesearch {
		if !s.Cfgp.Useseries && p.e.Info.Imdb != "" {
			usequerysearch = false
		} else if s.Cfgp.Useseries && p.e.NZB.TVDBID != 0 {
			usequerysearch = false
		}
	}
	var err error
	if usequerysearch && !p.titlesearch {
		return false
	}
	titlesearch := p.titlesearch
	if !usequerysearch {
		if !s.Cfgp.Useseries && p.e.Info.Imdb != "" {
			_, _, err = apiexternal.QueryNewznabMovieImdb(indcfg, s.Quality, logger.Trim(p.e.Info.Imdb, 't'), cats, &s.Raw)
		} else if s.Cfgp.Useseries && p.e.NZB.TVDBID != 0 {
			_, _, err = apiexternal.QueryNewznabTvTvdb(indcfg, s.Quality, p.e.NZB.TVDBID, cats, p.e.NZB.Season, p.e.NZB.Episode, true, true, &s.Raw)
		}

		if err != nil && !errors.Is(err, logger.ErrToWait) {
			if s.Cfgp.Useseries {
				p.e.Info.TempID = p.e.NzbepisodeID
			} else {
				p.e.Info.TempID = p.e.NzbmovieID
			}
			logsearcherror("Error Searching Media by ID", p.e.Info.TempID, s.Cfgp.Useseries, "", err)
		} else {
			if err == nil {
				done = true
			}
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
				logger.LogDynamicany0("debug", "Broke loop - result found")
				return false
			}
		}
		if s.Quality.SearchForTitleIfEmpty && !titlesearch && len(s.Raw.Arr) == 0 {
			titlesearch = true
		}
	}

	if !titlesearch {
		return (err == nil)
	}
	if titlesearch || s.Quality.BackupSearchForTitle {
		_, _, err = apiexternal.QueryNewznabQuery(s.Cfgp, &p.e, indcfg, s.Quality, p.e.WantedTitle, cats, &s.Raw)
		if err != nil && !errors.Is(err, logger.ErrToWait) {
			if s.Cfgp.Useseries {
				p.e.Info.TempID = p.e.NzbepisodeID
			} else {
				p.e.Info.TempID = p.e.NzbmovieID
			}
			logsearcherror("Error Searching Media by Title", p.e.Info.TempID, s.Cfgp.Useseries, p.e.WantedTitle, err)
			if !s.Quality.BackupSearchForAlternateTitle {
				if len(s.Raw.Arr) >= 1 {
					return true
				}
				return (err == nil)
			}
		} else {
			if err == nil && !done {
				done = true
			}
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
				logger.LogDynamicany0("debug", "Broke loop - result found")
				return done
			}
		}
	}
	if !titlesearch || !s.Quality.BackupSearchForAlternateTitle {
		return done
	}
	for idx := range p.sourcealttitles {
		if idx != 0 && p.sourcealttitles[idx].Str1 == "" {
			continue
		}
		if p.sourcealttitles[idx].Str1 == p.e.WantedTitle {
			continue
		}

		searchstr := p.sourcealttitles[idx].Str1
		logger.StringRemoveAllRunesP(&searchstr, '&', '(', ')')
		_, _, err = apiexternal.QueryNewznabQuery(s.Cfgp, &p.e, indcfg, s.Quality, searchstr, cats, &s.Raw)

		if err != nil && !errors.Is(err, logger.ErrToWait) {
			if s.Cfgp.Useseries {
				p.e.Info.TempID = p.e.NzbepisodeID
			} else {
				p.e.Info.TempID = p.e.NzbmovieID
			}
			logsearcherror("Error Searching Media by Title", p.e.Info.TempID, s.Cfgp.Useseries, p.sourcealttitles[idx].Str1, err)
		} else {
			if err == nil && !done {
				done = true
			}
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) >= 1 {
				logger.LogDynamicany0("debug", "Broke loop - result found")
				break
			}
		}
	}
	if len(s.Raw.Arr) >= 1 {
		done = true
	}
	return done
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
		if s.checkdownloaded(downloaded, idx) {
			continue
		}
		qualcfg := s.getentryquality(&s.Accepted[idx].Info)
		if qualcfg == nil {
			logger.LogDynamicany("info", "nzb found - start downloading", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &s.Accepted[idx].NZB.Title, &logger.StrMinPrio, &s.Accepted[idx].MinimumPriority, &logger.StrPriority, &s.Accepted[idx].Info.Priority)
		} else {
			logger.LogDynamicany("info", "nzb found - start downloading", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &s.Accepted[idx].NZB.Title, &logger.StrQuality, &qualcfg.Name, &logger.StrMinPrio, &s.Accepted[idx].MinimumPriority, &logger.StrPriority, &s.Accepted[idx].Info.Priority)
		}
		if s.Accepted[idx].NzbmovieID != 0 {
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
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Resolution").Str(logger.StrTitle, entry.NZB.Title).Str(logger.StrFound, entry.Info.Resolution).Strs(logger.StrWanted, quality.WantedResolution).Msg(skippedstr)
			entry.Reason = "unwanted Resolution"
			s.logdenied("", entry)
			return true
		}
	}

	if quality.WantedQualityLen >= 1 && entry.Info.Quality != "" {
		if !logger.SlicesContainsI(quality.WantedQuality, entry.Info.Quality) {
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Quality").Str(logger.StrTitle, entry.NZB.Title).Str(logger.StrFound, entry.Info.Quality).Strs(logger.StrWanted, quality.WantedQuality).Msg(skippedstr)
			entry.Reason = "unwanted Quality"
			s.logdenied("", entry)
			return true
		}
	}

	if quality.WantedAudioLen >= 1 && entry.Info.Audio != "" {
		if !logger.SlicesContainsI(quality.WantedAudio, entry.Info.Audio) {
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Audio").Str(logger.StrTitle, entry.NZB.Title).Str(logger.StrFound, entry.Info.Audio).Strs(logger.StrWanted, quality.WantedAudio).Msg(skippedstr)
			entry.Reason = "unwanted Audio"
			s.logdenied("", entry)
			return true
		}
	}

	if quality.WantedCodecLen >= 1 && entry.Info.Codec != "" {
		if !logger.SlicesContainsI(quality.WantedCodec, entry.Info.Codec) {
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Codec").Str(logger.StrTitle, entry.NZB.Title).Str(logger.StrFound, entry.Info.Codec).Strs(logger.StrWanted, quality.WantedCodec).Msg(skippedstr)
			entry.Reason = "unwanted Codec"
			s.logdenied("", entry)
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
// by priority.
func (s *ConfigSearcher) searchparse(e *apiexternal.Nzbwithprio, alttitles []database.DbstaticTwoStringOneInt) {
	if len(s.Raw.Arr) == 0 {
		return
	}
	s.Denied = s.Raw.Arr
	s.Denied = s.Denied[:0]
	s.Accepted = []apiexternal.Nzbwithprio{}
	var err error
	var ok bool
	for rawidx := range s.Raw.Arr {
		if s.Raw.Arr[rawidx].NZB.DownloadURL == "" {
			s.logdenied("no url", &s.Raw.Arr[rawidx])
			continue
		}
		if s.Raw.Arr[rawidx].NZB.Title == "" {
			s.logdenied("no title", &s.Raw.Arr[rawidx])
			continue
		}
		s.Raw.Arr[rawidx].NZB.Title = logger.Trim(s.Raw.Arr[rawidx].NZB.Title, ' ')
		if len(s.Raw.Arr[rawidx].NZB.Title) <= 3 {
			s.logdenied("short title", &s.Raw.Arr[rawidx])
			continue
		}
		if s.checkprocessed(&s.Raw.Arr[rawidx].NZB) {
			continue
		}
		// Check Size
		if s.Raw.Arr[rawidx].NZB.Indexer != nil {
			skipemptysize := s.Quality.QualityIndexerByQualityAndTemplateSkipEmpty(s.Raw.Arr[rawidx].NZB.Indexer)

			if !skipemptysize {
				_, ok = config.SettingsList[s.Raw.Arr[rawidx].NZB.Indexer.Name]
				if ok {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				} else if s.Raw.Arr[rawidx].NZB.Indexer.Getlistbyindexer() != nil {
					skipemptysize = s.Quality.Indexer[0].SkipEmptySize
				}
			}
			if skipemptysize && s.Raw.Arr[rawidx].NZB.Size == 0 {
				s.logdenied("no size", &s.Raw.Arr[rawidx])
				continue
			}
		}

		// check history
		if s.filterSizeNzbs(&s.Raw.Arr[rawidx]) {
			continue
		}
		if s.searchActionType != logger.StrRss && s.checkcorrectid(e, &s.Raw.Arr[rawidx]) {
			continue
		}

		parser.ParseFileP(s.Raw.Arr[rawidx].NZB.Title, false, false, s.Cfgp, -1, &s.Raw.Arr[rawidx].Info)
		if !s.Cfgp.Useseries && !s.Raw.Arr[rawidx].NZB.Indexer.TrustWithIMDBIDs {
			s.Raw.Arr[rawidx].Info.Imdb = ""
		}
		if s.Cfgp.Useseries && !s.Raw.Arr[rawidx].NZB.Indexer.TrustWithTVDBIDs {
			s.Raw.Arr[rawidx].Info.Tvdb = ""
		}
		err = parser.GetDBIDs(&s.Raw.Arr[rawidx].Info, s.Cfgp, true)
		if err != nil {
			s.logdenied1Str(err.Error(), &s.Raw.Arr[rawidx], strCheckedFor, s.Raw.Arr[rawidx].Info.Title)
			continue
		}
		var qual *config.QualityConfig
		if s.searchActionType == logger.StrRss {
			var skip bool
			skip, qual = s.getmediadatarss(&s.Raw.Arr[rawidx], -1, false)
			if skip {
				continue
			}
		} else {
			if s.getmediadata(e, &s.Raw.Arr[rawidx]) {
				continue
			}
			qual = s.Quality
			s.Raw.Arr[rawidx].WantedAlternates = alttitles
		}
		// needs the identifier from getmediadata

		if qual == nil {
			qual = s.getentryquality(&s.Raw.Arr[rawidx].Info)
		}
		if qual == nil {
			s.logdenied("unknown Quality", &s.Raw.Arr[rawidx])
			continue
		}
		if s.checkhistory(&s.Raw.Arr[rawidx], qual) {
			continue
		}
		if s.searchActionType != logger.StrRss && s.checkepisode(e, &s.Raw.Arr[rawidx]) {
			continue
		}

		if s.filterRegexNzbs(&s.Raw.Arr[rawidx], qual) {
			continue
		}

		if s.Raw.Arr[rawidx].Info.Priority == 0 {
			parser.GetPriorityMapQual(&s.Raw.Arr[rawidx].Info, s.Cfgp, qual, false, true)

			if s.Raw.Arr[rawidx].Info.Priority == 0 {
				s.logdenied1Str("unknown Prio", &s.Raw.Arr[rawidx], logger.StrFound, s.Raw.Arr[rawidx].Info.Title)
				continue
			}
		}

		s.Raw.Arr[rawidx].Info.StripTitlePrefixPostfixGetQual(qual)

		// check quality
		if s.filterTestQualityWanted(&s.Raw.Arr[rawidx], qual) {
			continue
		}
		// check priority

		if s.getminimumpriority(&s.Raw.Arr[rawidx], qual) {
			continue
		}
		if s.Raw.Arr[rawidx].MinimumPriority != 0 && s.Raw.Arr[rawidx].MinimumPriority == s.Raw.Arr[rawidx].Info.Priority {
			s.logdenied("same Prio", &s.Raw.Arr[rawidx])
			continue
		}

		if s.Raw.Arr[rawidx].MinimumPriority != 0 {
			if (qual.UseForPriorityMinDifference == 0 && s.Raw.Arr[rawidx].Info.Priority <= s.Raw.Arr[rawidx].MinimumPriority) || (qual.UseForPriorityMinDifference != 0 && s.Raw.Arr[rawidx].Info.Priority <= (s.Raw.Arr[rawidx].MinimumPriority+qual.UseForPriorityMinDifference)) {
				logger.Logtype("debug", 0).Str(logger.StrReason, "lower Prio").Str(logger.StrTitle, s.Raw.Arr[rawidx].NZB.Title).Int(logger.StrFound, s.Raw.Arr[rawidx].Info.Priority).Int(logger.StrWanted, s.Raw.Arr[rawidx].MinimumPriority).Msg(skippedstr)
				s.Raw.Arr[rawidx].Reason = "lower Prio"
				s.logdenied("", &s.Raw.Arr[rawidx])
				continue
			}
		}

		if s.searchActionType != logger.StrRss && s.checkyear(e, &s.Raw.Arr[rawidx], qual) {
			continue
		}

		if s.checktitle(&s.Raw.Arr[rawidx], qual) {
			continue
		}
		logger.LogDynamicany("debug", "Release ok", &logger.StrQuality, &qual.Name, &logger.StrTitle, &s.Raw.Arr[rawidx].NZB.Title, &logger.StrMinPrio, &s.Raw.Arr[rawidx].MinimumPriority, &logger.StrPriority, &s.Raw.Arr[rawidx].Info.Priority)

		s.Accepted = append(s.Accepted, s.Raw.Arr[rawidx])

		if qual.CheckUntilFirstFound {
			break
		}
	}
	if database.DBLogLevel == logger.StrDebug {
		logger.LogDynamicany1Int("debug", "Entries found", "Count", len(s.Raw.Arr))
	}
	if len(s.Accepted) > 1 {
		slices.SortFunc(s.Accepted, func(a, b apiexternal.Nzbwithprio) int {
			return cmp.Compare(a.Info.Priority, b.Info.Priority)
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
		entry.Info.TempID = entry.NzbmovieID
	} else {
		entry.Info.TempID = entry.NzbepisodeID
	}
	entry.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, &entry.Info.TempID, false, -1, cfgqual, false)
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
func (s *ConfigSearcher) checkcorrectid(sourceentry, entry *apiexternal.Nzbwithprio) bool {
	if s.searchActionType == logger.StrRss {
		return false
	}
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}
	if !s.Cfgp.Useseries {
		if entry.NZB.IMDBID != "" && entry.NZB.IMDBID != "tt0000000" && sourceentry.Info.Imdb != "" && sourceentry.Info.Imdb != entry.NZB.IMDBID {
			if logger.TrimLeft(sourceentry.Info.Imdb, 't', '0') != logger.TrimLeft(entry.NZB.IMDBID, 't', '0') {
				logger.Logtype("debug", 0).Str(logger.StrReason, "not matched imdb").Str(logger.StrTitle, entry.NZB.Title).Str(logger.StrFound, entry.NZB.IMDBID).Str(logger.StrWanted, sourceentry.Info.Imdb).Msg(skippedstr)
				entry.Reason = "not matched imdb"
				s.logdenied("", entry)
				return true
			}
		}
		return false
	}
	if sourceentry.NZB.TVDBID != 0 && entry.NZB.TVDBID != 0 && sourceentry.NZB.TVDBID != entry.NZB.TVDBID {
		logger.Logtype("debug", 0).Str(logger.StrReason, "not matched tvdb").Str(logger.StrTitle, entry.NZB.Title).Int(logger.StrFound, entry.NZB.TVDBID).Int(logger.StrWanted, sourceentry.NZB.TVDBID).Msg(skippedstr)
		entry.Reason = "not matched tvdb"
		s.logdenied("", entry)
		return true
	}
	return false
}

// getmediadata validates the media data in the given entry against the
// source entry to determine if it is a match. It sets various priority
// and search control fields on the entry based on the source entry
// configuration. Returns true to skip/reject the entry if no match, false
// to continue processing if a match.
func (s *ConfigSearcher) getmediadata(sourceentry, entry *apiexternal.Nzbwithprio) bool {
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}
	if !s.Cfgp.Useseries {
		if sourceentry.NzbmovieID != entry.Info.MovieID {
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Movie").Str(logger.StrTitle, entry.NZB.Title).Uint(logger.StrFound, entry.Info.MovieID).Uint(logger.StrWanted, sourceentry.NzbmovieID).Str(logger.StrImdb, sourceentry.Info.Imdb).Str(logger.StrConfig, s.Cfgp.NamePrefix).Msg(skippedstr)
			entry.Reason = "unwanted Movie"
			s.logdenied("", entry)
			return true
		}
		entry.NzbmovieID = sourceentry.NzbmovieID
	} else {
		if entry.Info.SerieEpisodeID != sourceentry.NzbepisodeID {
			logger.Logtype("debug", 0).Str(logger.StrReason, "unwanted Episode").Str(logger.StrTitle, entry.NZB.Title).Uint(logger.StrFound, entry.Info.SerieEpisodeID).Uint(logger.StrWanted, sourceentry.NzbepisodeID).Str(strIdentifier, sourceentry.Info.Identifier).Str(logger.StrConfig, s.Cfgp.NamePrefix).Msg(skippedstr)
			entry.Reason = "unwanted Episode"
			s.logdenied("", entry)
			return true
		}
		entry.NzbepisodeID = sourceentry.NzbepisodeID
	}
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
func (s *ConfigSearcher) getmediadatarss(entry *apiexternal.Nzbwithprio, addinlistid int, addifnotfound bool) (bool, *config.QualityConfig) {
	if s.Cfgp.Useseries {
		// Parse Series
		// Filter RSS Series
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

		getrssdata(&entry.Info.SerieEpisodeID, true, entry)
		entry.Info.ListID = s.Cfgp.GetMediaListsEntryListID(entry.Listname)
		if entry.Quality == "" {
			return false, config.SettingsQuality[s.Cfgp.DefaultQuality]
		}
		return false, config.SettingsQuality[entry.Quality]
	}
	if addinlistid != -1 && s.Cfgp != nil {
		entry.Info.ListID = addinlistid
	}
	if entry.Info.DbmovieID == 0 && (!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
		s.logdenied("unwanted DBMovie", entry)
		return true, nil
	}
	// add movie if not found
	if addifnotfound && (entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0) && logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if addinlistid == -1 {
			return true, nil
		}
		bl, err := importfeed.AllowMovieImport(&entry.NZB.IMDBID, s.Cfgp.Lists[addinlistid].CfgList)
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
		database.Scanrows2dyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
		if entry.Info.MovieID == 0 || entry.Info.DbmovieID == 0 {
			s.logdenied("unwanted Movie", entry)
			return true, nil
		}
	}
	if entry.Info.DbmovieID == 0 {
		s.logdenied("unwanted DBMovie", entry)
		return true, nil
	}

	// continue only if dbmovie found
	// Get List of movie by dbmovieid, year and possible lists

	// if list was not found : should we add the movie to the list?
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
			database.Scanrows2dyn(false, database.QueryMoviesGetIDByDBIDListname, &entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name)
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

	getrssdata(&entry.Info.MovieID, false, entry)

	entry.Info.ListID = s.Cfgp.GetMediaListsEntryListID(entry.Listname)
	if entry.Quality == "" {
		return false, config.SettingsQuality[s.Cfgp.DefaultQuality]
	}
	return false, config.SettingsQuality[entry.Quality]
}

// getrssdata retrieves RSS data for the given movie ID, and populates the
// provided Nzbwithprio entry with the retrieved data, including the
// DontSearch, DontUpgrade, Listname, Quality, and WantedTitle fields.
// If useseries is true, the function will use a different set of query
// parameters to retrieve the data.
func getrssdata(id *uint, useseries bool, entry *apiexternal.Nzbwithprio) {
	database.GetdatarowArgs(logger.GetStringsMap(useseries, "GetRSSData"), id, &entry.DontSearch, &entry.DontUpgrade, &entry.Listname, &entry.Quality, &entry.WantedTitle)
}

// checkyear validates the year in the entry title against the year
// configured for the wanted entry. It returns false if a match is found,
// or true to skip the entry if no match is found. This is used during
// search result processing to filter entries by year.
func (s *ConfigSearcher) checkyear(sourceentry, entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
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
	s.logdenied1UInt16("unwanted Year", entry, sourceentry.Info.Year)
	return true
}

// checktitle validates the title and alternate titles of the entry against
// the wanted title and quality configuration. It returns false if a match is
// found, or true to skip the entry if no match is found. This is an internal
// function used during search result processing.
func (s *ConfigSearcher) checktitle(entry *apiexternal.Nzbwithprio, qual *config.QualityConfig) bool {
	// Checktitle
	if !qual.CheckTitle {
		return false
	}
	if qual != nil {
		entry.Info.StripTitlePrefixPostfixGetQual(qual)
	}

	wantedslug := logger.StringToSlug(entry.WantedTitle)
	if entry.WantedTitle != "" {
		if qual.CheckTitle && entry.WantedTitle != "" && database.ChecknzbtitleB(entry.WantedTitle, wantedslug, entry.NZB.Title, qual.CheckYear1, entry.Info.Year) {
			return false
		}
	}
	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.Info.Title, ']') {
		for i := len(entry.Info.Title) - 1; i >= 0; i-- {
			if entry.Info.Title[i] == ']' {
				if i < (len(entry.Info.Title) - 1) {
					trytitle = logger.TrimLeft(entry.Info.Title[i+1:], '-', '.', ' ')
					if qual.CheckTitle && entry.WantedTitle != "" && database.ChecknzbtitleB(entry.WantedTitle, wantedslug, trytitle, qual.CheckYear1, entry.Info.Year) {
						return false
					}
				}
			}
		}
	}
	if entry.Dbid != 0 && len(entry.WantedAlternates) == 0 {
		entry.WantedAlternates = database.GetDbstaticTwoStringOneInt(database.Getentryalternatetitlesdirect(&entry.Dbid, s.Cfgp.Useseries), entry.Dbid)
	}

	if entry.Info.Title == "" || len(entry.WantedAlternates) == 0 {
		s.logdenied("unwanted Title", entry)
		return true
	}
	for idx := range entry.WantedAlternates {
		if entry.WantedAlternates[idx].Str1 == "" {
			continue
		}
		if database.ChecknzbtitleB(entry.WantedAlternates[idx].Str1, entry.WantedAlternates[idx].Str2, entry.NZB.Title, qual.CheckYear1, entry.Info.Year) {
			return false
		}

		if trytitle != "" && trytitle != entry.WantedAlternates[idx].Str1 && trytitle != entry.WantedTitle {
			if database.ChecknzbtitleB(entry.WantedAlternates[idx].Str1, entry.WantedAlternates[idx].Str2, trytitle, qual.CheckYear1, entry.Info.Year) {
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
func (s *ConfigSearcher) checkepisode(sourceentry, entry *apiexternal.Nzbwithprio) bool {
	// Checkepisode
	if !s.Cfgp.Useseries {
		return false
	}
	if s.searchActionType == logger.StrRss {
		if sourceentry == nil {
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
		if strings.ContainsRune(entry.NZB.Title, '.') && logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(sourceentry.Info.Identifier, '-', '.')) {
			return false
		}
		if strings.ContainsRune(entry.NZB.Title, ' ') && logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(sourceentry.Info.Identifier, '-', ' ')) {
			return false
		}
	}
	// Check For S01E01 S01 E01 1x01 1 x 01 S01E02E03
	altIdentifier := logger.TrimLeft(sourceentry.Info.Identifier, 's', 'S', '0')
	if strings.ContainsRune(altIdentifier, 'E') {
		logger.StringReplaceWithP(&altIdentifier, 'E', 'x')
	}
	if strings.ContainsRune(altIdentifier, 'e') {
		logger.StringReplaceWithP(&altIdentifier, 'e', 'x')
	}

	if logger.ContainsI(entry.NZB.Title, altIdentifier) {
		return false
	}
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" && strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		if strings.ContainsRune(entry.NZB.Title, '.') && logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', '.')) {
			return false
		}
		if strings.ContainsRune(entry.NZB.Title, ' ') && logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', ' ')) {
			return false
		}
	}

	if sourceentry.NZB.Season == "" || sourceentry.NZB.Episode == "" {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	sprefix, eprefix := "s", "e"
	if logger.ContainsI(sourceentry.Info.Identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else if !logger.ContainsI(sourceentry.Info.Identifier, "s") && !logger.ContainsI(sourceentry.Info.Identifier, "e") {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	if !logger.HasPrefixI(sourceentry.Info.Identifier, (sprefix + sourceentry.NZB.Season)) {
		s.logdenied1StrNo("unwanted Season", entry, &sourceentry.Info)
		return true
	}
	if !logger.ContainsI(sourceentry.Info.Identifier, sourceentry.NZB.Episode) {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	// suffixcheck
	for idx := range episodeprefixarray {
		if logger.HasSuffixI(sourceentry.Info.Identifier, (eprefix + episodeprefixarray[idx] + sourceentry.NZB.Episode)) {
			return false
		}
	}

	if !checksuffix(eprefix, sourceentry, 0) {
		return false
	}
	if !checksuffix(eprefix, sourceentry, 1) {
		return false
	}
	if !checksuffix(eprefix, sourceentry, 2) {
		return false
	}

	s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
	return true
}

// checksuffix checks if the given episode identifier has a valid suffix. It takes the episode prefix, the source entry, and a value index to determine the suffix to check.
// The function returns true if the identifier does not have the given suffix, and false otherwise.
func checksuffix(eprefix string, sourceentry *apiexternal.Nzbwithprio, valin uint8) bool {
	var val string
	switch valin {
	case 0:
		val = eprefix
	case 1:
		val = logger.StrSpace
	case 2:
		val = logger.StrDash
	}
	if logger.ContainsI(sourceentry.Info.Identifier, (eprefix + sourceentry.NZB.Episode + val)) {
		return false
	}
	for idx := range episodeprefixarray {
		if logger.HasSuffixI(sourceentry.Info.Identifier, (eprefix + episodeprefixarray[idx] + sourceentry.NZB.Episode + val)) {
			return false
		}
	}
	return true
}

// getentryquality returns the quality config for the given entry.
// If the entry is for a movie, it gets the config from the movies database using the movie ID.
// If the entry is for a TV episode, it gets the config from the series database using the episode ID.
// If no ID is set, it returns nil.
func (s *ConfigSearcher) getentryquality(entry *database.ParseInfo) *config.QualityConfig {
	if entry.MovieID != 0 {
		return database.GetMediaQualityConfigP(s.Cfgp, &entry.MovieID)
	}
	if entry.SerieEpisodeID != 0 {
		return database.GetMediaQualityConfigP(s.Cfgp, &entry.SerieEpisodeID)
	}
	return nil
}

// GetRSSFeed queries the RSS feed for the given media list, searches for and downloads new items,
// and adds them to the search results. It handles checking if the indexer is blocked,
// configuring the custom RSS feed URL, getting the last ID to prevent duplicates,
// parsing results, and updating the RSS history.
func (s *ConfigSearcher) getRSSFeed(listentry *config.MediaListsConfig, downloadentries bool) error {
	if s == nil {
		return errSearchvarEmpty
	}
	defer s.Close()
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
	if database.Getdatarow2[uint](false, "select count() from indexer_fails where  last_fail > ? and indexer = ?", &intval, &listentry.CfgList.URL) >= 1 {
		logger.LogDynamicany2StrAny("debug", "Indexer temporarily disabled due to fail in last", logger.StrListname, listentry.TemplateList, strMinutes, blockinterval)
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
	firstid, err := apiexternal.QueryNewznabRSSLastCustom(&customindexer, s.Quality, database.Getdatarow2[string](false, "select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE", &listentry.TemplateList, &s.Quality.Name), -1, &s.Raw)
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
func (s *ConfigSearcher) movieFillSearchVar(p *searchParams) error {
	if p.e.NzbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}
	database.GetdatarowArgs("select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?", &p.e.NzbmovieID, &p.e.Dbid, &p.e.DontSearch, &p.e.DontUpgrade, &p.e.Listname, &p.e.Quality, &p.e.Info.Year, &p.e.Info.Imdb, &p.e.WantedTitle)
	if p.e.Dbid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	if p.e.DontSearch {
		return logger.ErrDisabled
	}

	if s.Quality == nil || s.Quality.Name != p.e.Quality {
		if p.e.Quality == "" {
			s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
		} else {
			var ok bool
			s.Quality, ok = config.SettingsQuality[p.e.Quality]
			if !ok {
				s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
			}
		}
	}
	p.e.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, &p.e.NzbmovieID, false, -1, s.Quality, false)

	var err error
	s.searchActionType, err = getsearchtype(p.e.MinimumPriority, p.e.DontUpgrade, false)
	if err != nil {
		return err
	}

	if p.e.Info.Year == 0 {
		return errYearEmpty
	}
	p.e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(p.e.Listname)

	return nil
}

// EpisodeFillSearchVar fills the search variables for the given episode ID.
// It queries the database to get the necessary data and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) episodeFillSearchVar(p *searchParams) error {
	if p.e.NzbepisodeID == 0 {
		return logger.ErrNotFoundEpisode
	}

	// dbserie_episode_id, dbserie_id, serie_id, dont_search, dont_upgrade
	database.GetdatarowArgs("select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?", &p.e.NzbepisodeID, &p.e.Info.DbserieEpisodeID, &p.e.Dbid, &p.e.Info.SerieID, &p.e.DontSearch, &p.e.DontUpgrade, &p.e.Quality, &p.e.Listname, &p.e.NZB.TVDBID, &p.e.WantedTitle, &p.e.NZB.Season, &p.e.NZB.Episode, &p.e.Info.Identifier)
	if p.e.Info.DbserieEpisodeID == 0 || p.e.Dbid == 0 || p.e.Info.SerieID == 0 {
		return logger.ErrNotFound
	}
	if p.e.DontSearch {
		return logger.ErrDisabled
	}
	p.e.Info.DbserieID = p.e.Dbid

	if s.Quality == nil || s.Quality.Name != p.e.Quality {
		if p.e.Quality == "" {
			s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
		} else {
			var ok bool
			s.Quality, ok = config.SettingsQuality[p.e.Quality]
			if !ok {
				s.Quality = config.SettingsQuality[s.Cfgp.DefaultQuality]
			}
		}
	}
	p.e.MinimumPriority, _ = Getpriobyfiles(s.Cfgp.Useseries, &p.e.NzbepisodeID, false, -1, s.Quality, false)

	p.e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(p.e.Listname)
	var err error
	s.searchActionType, err = getsearchtype(p.e.MinimumPriority, p.e.DontUpgrade, false)
	return err
}

// filterSizeNzbs checks if the NZB entry size is within the configured
// minimum and maximum size limits, and returns true if it should be
// rejected based on its size.
func (s *ConfigSearcher) filterSizeNzbs(entry *apiexternal.Nzbwithprio) bool {
	for _, dataimport := range s.Cfgp.DataImportMap {
		if dataimport.CfgPath == nil {
			continue
		}
		if dataimport.CfgPath.MinSize != 0 && entry.NZB.Size < dataimport.CfgPath.MinSizeByte {
			s.logdenied1Int64("too small", entry)
			return true
		}

		if dataimport.CfgPath.MaxSize != 0 && entry.NZB.Size > dataimport.CfgPath.MaxSizeByte {
			s.logdenied1Int64("too big", entry)
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
		s.logdenied1Str("Denied by Regex", entry, strRegexEmpty, "")
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
		s.logdenied1Str("not matched required", entry, strCheckedFor, regexcfg.Required[0])
		return true
	}

	for idxr := range regexcfg.Rejected {
		if database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.NZB.Title, 1) {
			if !database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.WantedTitle, 1) {
				bl = false
				for idx := range entry.WantedAlternates {
					if entry.WantedTitle != entry.WantedAlternates[idx].Str1 && database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.WantedAlternates[idx].Str1, 1) {
						bl = true
						break
					}
				}
				if !bl {
					s.logdenied1Str("Denied by Regex", entry, strRejectedby, regexcfg.Rejected[idxr])
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
		if database.CheckcachedURLHistory(s.Cfgp.Useseries, &entry.NZB.DownloadURL) {
			s.logdenied("already downloaded url", entry)
			return true
		}
	}
	if entry.NZB.Indexer == nil {
		return false
	}
	if !qual.QualityIndexerByQualityAndTemplateCheckTitle(entry.NZB.Indexer) {
		return false
	}

	if entry.NZB.Title != "" {
		if database.CheckcachedTitleHistory(s.Cfgp.Useseries, &entry.NZB.Title) {
			s.logdenied("already downloaded title", entry)
			return true
		}
	}
	return false
}

// checkprocessed checks if the given entry is already in the denied or accepted lists to avoid duplicate processing.
// It loops through the denied and accepted entries and returns true if it finds a match on the download URL or title.
// Otherwise returns false. Part of ConfigSearcher.
func (s *ConfigSearcher) checkprocessed(entry *apiexternal.Nzb) bool {
	for idx := range s.Denied {
		if s.Denied[idx].NZB.DownloadURL == entry.DownloadURL {
			return true
		}
		if s.Denied[idx].NZB.Title == entry.Title {
			return true
		}
	}
	for idx := range s.Accepted {
		if s.Accepted[idx].NZB.DownloadURL == entry.DownloadURL {
			return true
		}
		if s.Accepted[idx].NZB.Title == entry.Title {
			return true
		}
	}
	return false
}

// deniedappend appends the given Nzbwithprio entry to the ConfigSearcher's Denied slice.
func (s *ConfigSearcher) deniedappend(entry *apiexternal.Nzbwithprio) {
	s.Denied = append(s.Denied, *entry)
}

// logdenied logs a denied entry with the given reason and optional additional fields.
// It sets the reason and additional reason on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied(reason string, entry *apiexternal.Nzbwithprio) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).Str(logger.StrReason, entry.Reason).Str(logger.StrTitle, entry.NZB.Title).Msg(skippedstr)
	}
	s.deniedappend(entry)
}

// logdenied1Int64 logs a denied entry with the given reason and the NZB size as an additional int64 field.
// It sets the reason and additional int64 field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1Int64(reason string, entry *apiexternal.Nzbwithprio) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).Str(logger.StrReason, entry.Reason).Str(logger.StrTitle, entry.NZB.Title).Int64(logger.StrFound, entry.NZB.Size).Msg(skippedstr)
		entry.AdditionalReasonInt = entry.NZB.Size
	}
	s.deniedappend(entry)
}

// logdenied1UInt16 logs a denied entry with the given reason and an additional uint16 field.
// It sets the reason and additional uint16 field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1UInt16(reason string, entry *apiexternal.Nzbwithprio, value1 uint16) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).Str(logger.StrReason, entry.Reason).Str(logger.StrTitle, entry.NZB.Title).Uint16(logger.StrWanted, value1).Msg(skippedstr)
		entry.AdditionalReasonInt = int64(value1)
	}
	s.deniedappend(entry)
}

// logdenied1Str logs a denied entry with the given reason and an additional string field.
// It sets the reason and additional string field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1Str(reason string, entry *apiexternal.Nzbwithprio, field1 string, value1 string) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).Str(logger.StrReason, entry.Reason).Str(logger.StrTitle, entry.NZB.Title).Str(field1, value1).Msg(skippedstr)
		entry.AdditionalReasonStr = value1
	}
	s.deniedappend(entry)
}

// logdenied1StrNo logs a denied entry with the given reason and an additional string field.
// It sets the reason and additional string field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1StrNo(reason string, entry *apiexternal.Nzbwithprio, value1 *database.ParseInfo) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).Str(logger.StrReason, entry.Reason).Str(logger.StrTitle, entry.NZB.Title).Str(strIdentifier, value1.Identifier).Msg(skippedstr)
	}
	s.deniedappend(entry)
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
	s.Cfgp = cfgp
	s.searchActionType = searchActionType
	if quality == nil {
		if mediaid != nil {
			s.Quality = database.GetMediaQualityConfigP(cfgp, mediaid)
		}
	} else {
		s.Quality = quality
	}
	return s
}

// addrsshistory updates the rss history table with the last processed item id
// for the given rss feed url, quality profile name, and config name. It will
// insert a new row if one does not exist yet for that combination.
func addrsshistory(urlv, lastid *string, quality *config.QualityConfig, configv *string) {
	id := database.Getdatarow3[uint](false, "select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE", configv, &quality.Name, urlv)
	if id >= 1 {
		database.Exec2("update r_sshistories set last_id = ? where id = ?", lastid, &id)
	} else {
		database.ExecN("insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?)", configv, &quality.Name, urlv, lastid)
	}
}

// getsearchtype returns the search type string based on the minimumPriority,
// dont, and force parameters. If minimumPriority is 0, returns "missing".
// If dont is true and force is false, returns a disabled error.
// Otherwise returns "upgrade".
func getsearchtype(minimumPriority int, dont, force bool) (string, error) {
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

	return NewSearcher(cfgp, list.CfgQuality, logger.StrRss, nil).getRSSFeed(list, true)
}

// SearchSerieRSSSeasonSingle searches for a single season of a series.
// It takes the series ID, season number, whether to search the full season or missing episodes,
// media type config, whether to auto close the results, and a pointer to search results.
// It returns a config searcher instance and error.
// It queries the database to map the series ID to thetvdb ID, gets the quality config,
// calls the search function, handles errors, downloads results,
// closes the results if autoclose is true, and returns the config searcher.
func SearchSerieRSSSeasonSingle(serieid *uint, season string, useseason bool, cfgp *config.MediaTypeConfig) (*ConfigSearcher, error) {
	dbserieid := database.Getdatarow1[uint](false, "select dbserie_id from series where id = ?", serieid)
	tvdb := database.Getdatarow1[int](false, "select thetvdb_id from dbseries where id = ?", &dbserieid)
	if tvdb == 0 {
		return nil, logger.ErrTvdbEmpty
	}
	listid := database.GetMediaListIDGetListname(cfgp, serieid)
	if listid == -1 {
		return nil, logger.ErrListnameEmpty
	}

	results := NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, nil)
	if results == nil {
		return nil, errSearchvarEmpty
	}
	err := results.searchSeriesRSSSeason(context.Background(), cfgp, cfgp.Lists[listid].CfgQuality, tvdb, season, useseason, true, false)
	if err != nil && !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
		logger.Logtype("error", 0).Err(err).Uint(logger.StrID, *serieid).Msg("Season Search Inc Failed")
		return nil, err
	}
	return results, nil
}

// SearchSeriesRSSSeasons searches the RSS feeds for missing episodes for
// random series. It selects up to 20 random series that have missing
// episodes, gets the distinct seasons with missing episodes for each,
// and searches the RSS feeds for those seasons.
func SearchSeriesRSSSeasons(cfgp *config.MediaTypeConfig) {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	searchseasons(context.Background(), cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20"), 20, "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )", args)
}

// SearchSeriesRSSSeasonsAll searches all seasons for series matching the given
// media type config. It searches series that have missing episodes and calls
// searchseasons to perform the actual search.
func SearchSeriesRSSSeasonsAll(cfgp *config.MediaTypeConfig) {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	searchseasons(context.Background(), cfgp, logger.JoinStrings("select id, dbserie_id from series where listname in (?", cfgp.ListsQu, ") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1"), database.Getdatarow0(false, "select count() from series"), "select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )", "select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )", args)
}

// searchseasons searches for missing episodes for series matching the given
// configuration and quality settings. It selects a random sample of series
// to search, gets the distinct seasons with missing episodes for each, and
// searches those seasons on the RSS feeds of the enabled indexers. Results
// are added to the passed in DownloadResults instance.
func searchseasons(ctx context.Context, cfgp *config.MediaTypeConfig, queryrange string, queryrangecount uint, queryseason, queryseasoncount string, args *logger.Arrany) {
	tbl := database.GetrowsN[database.DbstaticTwoUint](false, queryrangecount, queryrange, args.Arr...)
	for idx := range tbl {
		searchseason(ctx, cfgp, &tbl[idx], queryseason, queryseasoncount)
	}
}

// searchseason searches for missing episodes for a specific series and season.
// It retrieves the distinct seasons with missing episodes for the given series,
// and then searches the RSS feeds of the enabled indexers for those seasons.
// The results are added to the DownloadResults instance.
func searchseason(ctx context.Context, cfgp *config.MediaTypeConfig, row *database.DbstaticTwoUint, queryseason, queryseasoncount string) {
	for _, arr := range database.GetrowsN[string](false, database.GetdatarowN(false, queryseasoncount, &row.Num2, &row.Num1, &row.Num1, &row.Num2), queryseason, &row.Num2, &row.Num1, &row.Num1, &row.Num2) {
		listid := database.GetMediaListIDGetListname(cfgp, &row.Num1)
		if listid == -1 {
			continue
		}
		NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, nil).searchSeriesRSSSeason(ctx, cfgp, cfgp.Lists[listid].CfgQuality, database.Getdatarow1[int](false, "select thetvdb_id from dbseries where id = ?", &row.Num2), arr, true, true, true)
	}
}

// Getpriobyfiles returns the minimum priority of existing files for the given media
// ID, and optionally returns a slice of file paths for existing files below
// the given wanted priority. If useseries is true it will look up series IDs instead of media IDs.
// If id is nil it will return 0 priority.
// If useall is true it will include files marked as deleted.
// If wantedprio is -1 it will not return any file paths.
func Getpriobyfiles(useseries bool, id *uint, useall bool, wantedprio int, qualcfg *config.QualityConfig, getold bool) (int, []string) {
	if qualcfg == nil || *id == 0 {
		return 0, nil
	}

	arr := database.Getrows1size[database.FilePrio](false, logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID), logger.GetStringsMap(useseries, logger.DBFilePrioFilesByID), id)

	if len(arr) == 0 {
		return 0, nil
	}
	var minPrio int
	var oldf []string
	if len(arr) > 1 && getold {
		oldf = make([]string, 0, len(arr))
	}
	var prio, intid int
	var r, q, a, c uint
	for idx := range arr {
		prio = 0
		if !useall {
			r = arr[idx].ResolutionID
			q = arr[idx].QualityID
			a = arr[idx].AudioID
			c = arr[idx].CodecID
			if !qualcfg.UseForPriorityResolution {
				r = 0
			}
			if !qualcfg.UseForPriorityQuality {
				q = 0
			}
			if !qualcfg.UseForPriorityAudio {
				a = 0
			}
			if !qualcfg.UseForPriorityCodec {
				c = 0
			}
			intid = parser.Findpriorityidxwanted(r, q, c, a, qualcfg)
		} else {
			intid = parser.Findpriorityidxwanted(arr[idx].ResolutionID, arr[idx].QualityID, arr[idx].CodecID, arr[idx].AudioID, qualcfg)
		}
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
			logger.LogDynamicany2Str("debug", "prio not found", "in", qualcfg.Name, "searched for", parser.BuildPrioStr(arr[idx].ResolutionID, arr[idx].QualityID, arr[idx].CodecID, arr[idx].AudioID))
			prio = 0
		}
		if qualcfg.UseForPriorityOther || useall {
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

		if minPrio < prio || minPrio == 0 {
			minPrio = prio
		}
		if wantedprio != -1 && wantedprio > prio {
			if len(arr) == 0 {
				if getold {
					return minPrio, []string{arr[idx].Location}
				}
				return minPrio, nil
			}
			if getold {
				oldf = append(oldf, arr[idx].Location)
			}
		}
	}
	if wantedprio == -1 {
		return minPrio, nil
	}
	if len(oldf) == 0 {
		return minPrio, nil
	}
	return minPrio, oldf
}
