package searcher

import (
	"cmp"
	"context"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/alitto/pond/v2"
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

	// Optimization: Pre-allocated buffers for frequent operations
	episodeBuffer    [4]string       // Pre-allocated buffer for episode prefix array
	qualityChecks    [4]qualityCheck // Pre-allocated for quality validation
	indexerConfigMap map[string]int  // Cache for indexer config lookups
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

// qualityCheck represents a single quality validation check
type qualityCheck struct {
	enabled    bool
	entryValue string
	wantedList []string
	fieldName  string
	logField   string
}

const (
	skippedstr        = "Skipped"
	searchTypeMissing = 1
	searchTypeRSS     = 2
	searchTypeSeason  = 3

	// Buffer sizes for better memory management
	defaultRawCapacity      = 8000
	defaultDeniedCapacity   = 1000
	defaultAcceptedCapacity = 100

	// Pool sizes
	searcherPoolSize = 10
	paramPoolSize    = 5
)

var (
	strRegexEmpty         = "regex_template empty"
	strMinutes            = "Minutes"
	strIdentifier         = "identifier"
	strCheckedFor         = "checked for"
	strTitlesearch        = "titlesearch"
	strRejectedby         = "rejected by"
	strMediaid            = "Media ID"
	episodePrefixes       = [4]string{"", logger.StrSpace, "0", " 0"}
	errOther              = errors.New("other error")
	errYearEmpty          = errors.New("year empty")
	errSearchvarEmpty     = errors.New("searchvar empty")
	errSearchIDEmpty      = errors.New("search id empty")
	errSearchQualityEmpty = errors.New("search quality empty")
	errRegexEmpty         = errors.New("regex template empty")
	plsearcher            pool.Poolobj[ConfigSearcher]
	plsearchparam         pool.Poolobj[searchParams]
	// Optimization: String interner for frequently used strings
	stringInterner = sync.Map{} // Cache for frequently used strings
)

// clearNzbSlice efficiently clears the contents of an Nzbwithprio slice by resetting
// AdditionalReason to nil and clearing the Episodes and Languages maps for each element.
// This helps prepare the slice for reuse without reallocating memory.
func clearNzbSlice(slice []apiexternal.Nzbwithprio) {
	for i := range slice {
		slice[i].AdditionalReason = nil
		clear(slice[i].Info.Episodes)
		clear(slice[i].Info.Languages)
	}
}

// reset clears and resets the ConfigSearcher's internal state, efficiently resetting search-related slices
// and preparing the searcher for a new search operation. It zeroes out the search action type,
// marks the search as not done, and clears the Denied, Accepted, and Raw result arrays.
func (s *ConfigSearcher) reset() {
	s.searchActionType = ""
	s.Done = false

	// Clear slices efficiently
	clearNzbSlice(s.Denied)
	clearNzbSlice(s.Accepted)
	clearNzbSlice(s.Raw.Arr)

	s.Denied = s.Denied[:0]
	s.Accepted = s.Accepted[:0]
	s.Raw.Arr = s.Raw.Arr[:0]
}

// Init initializes the searcher subsystem by setting up object pools for efficient
// memory management during search operations. It creates two pools:
//   - plsearchparam: Pool for searchParams structs used in search operations
//   - plsearcher: Pool for ConfigSearcher instances with pre-allocated buffers
//
// The ConfigSearcher pool is pre-configured with optimized buffer sizes for:
//   - Raw search results (8000 capacity)
//   - Indexer configuration lookup map (10 capacity)
//   - Episode prefix buffer for efficient string operations
//
// This initialization reduces memory allocations during frequent search operations.
func Init() {
	plsearchparam.Init(paramPoolSize, nil, func(cs *searchParams) bool {
		*cs = searchParams{}
		return false
	})
	plsearcher.Init(searcherPoolSize, func(cs *ConfigSearcher) {
		cs.Raw.Arr = make([]apiexternal.Nzbwithprio, 0, defaultRawCapacity)
		// cs.Denied = make([]apiexternal.Nzbwithprio, 0, 1000)
		// cs.Accepted = make([]apiexternal.Nzbwithprio, 0, 100)
		cs.indexerConfigMap = make(map[string]int, 10) // Pre-allocate map

		// Pre-populate episode buffer
		copy(cs.episodeBuffer[:], episodePrefixes[:])
	}, func(cs *ConfigSearcher) bool {
		cs.reset()
		return false
	})
}

// NewSearcher creates a new ConfigSearcher instance.
// It initializes the searcher with the given media type config,
// quality config, search action type, and media ID.
// If no quality config is provided but a media ID is given,
// it will look up the quality config for that media in the database.
// It gets a searcher instance from the pool and sets the configs,
// then returns the initialized searcher.
func NewSearcher(
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	searchActionType string,
	mediaid *uint,
) *ConfigSearcher {
	s := plsearcher.Get()
	s.Cfgp = cfgp
	s.searchActionType = searchActionType

	if quality != nil {
		s.Quality = quality
	} else if mediaid != nil {
		s.Quality = database.GetMediaQualityConfig(cfgp, mediaid)
	}
	return s
}

// SearchRSS searches the RSS feeds of the enabled Newznab indexers for the
// given media type and quality configuration. It returns a ConfigSearcher
// instance for managing the search, or an error if no search could be started.
// Results are added to the passed in DownloadResults instance.
func (s *ConfigSearcher) SearchRSS(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	downloadresults, autoclose bool,
) error {
	if s == nil {
		return errSearchvarEmpty
	}
	if autoclose {
		defer s.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	s.Quality = quality
	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.searchtype = searchTypeRSS
	s.searchindexers(ctx, true, p)
	if s.Done {
		s.processSearchResults(downloadresults, "", nil, s.Quality, nil)
	}
	return nil
}

// MediaSearch searches indexers for the given media entry (movie or TV episode)
// using the configured quality profile. It handles filling search variables,
// executing searches across enabled indexers, parsing results, and optionally
// downloading accepted entries. Returns the search results and error if any.
func (s *ConfigSearcher) MediaSearch(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	mediaid uint,
	titlesearch, downloadentries, autoclose bool,
) error {
	if s == nil {
		logger.Logtype("error", 0).
			Uint(logger.StrID, mediaid).
			Err(errSearchvarEmpty).
			Msg("Media Search Failed")
		return errSearchvarEmpty
	}
	if autoclose {
		defer s.Close()
	}
	if cfgp == nil {
		logger.Logtype("error", 0).
			Uint(logger.StrID, mediaid).
			Err(logger.ErrCfgpNotFound).
			Msg("Media Search Failed")
		return logger.ErrCfgpNotFound
	}

	if mediaid == 0 {
		logger.Logtype("error", 0).
			Uint(logger.StrID, mediaid).
			Err(errSearchIDEmpty).
			Msg("Media Search Failed")
		return errSearchIDEmpty
	}
	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.mediaid = mediaid
	p.titlesearch = titlesearch
	p.searchtype = searchTypeMissing
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
			logger.Logtype("error", 0).
				Uint(logger.StrID, mediaid).
				Bool("Search Series", cfgp.Useseries).
				Err(err).
				Msg("Media Search Failed")
			return err
		}
		return nil
	}

	if s.Quality == nil {
		logger.Logtype("error", 0).
			Uint(logger.StrID, mediaid).
			Bool("Search Series", cfgp.Useseries).
			Err(errSearchQualityEmpty).
			Msg("Media Search Quality Failed")
		return errSearchQualityEmpty
	}

	s.searchlog("info", "Search for media id", p)

	if titlesearch || s.Quality.BackupSearchForTitle || s.Quality.BackupSearchForAlternateTitle {
		p.sourcealttitles = database.GetDbstaticTwoStringOneInt(
			database.Getentryalternatetitlesdirect(&p.e.Dbid, s.Cfgp.Useseries),
			p.e.Dbid,
		)
	}
	s.searchindexers(ctx, false, p)
	if !s.Done {
		s.searchlog("error", "All searches failed", p)
		return nil
	}
	database.ExecN(logger.GetStringsMap(cfgp.Useseries, logger.UpdateMediaLastscan), &p.mediaid)

	if len(s.Raw.Arr) > 0 {
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

// executeSearch performs a search based on the search type specified in the search parameters.
// It supports different search types: missing media, RSS feed, and season search.
// Returns a boolean indicating whether the search was successful.
func (s *ConfigSearcher) executeSearch(p *searchParams, indcfg *config.IndexersConfig) error {
	switch p.searchtype {
	case searchTypeMissing:
		return s.searchnameid(p, indcfg)
	case searchTypeRSS:
		return s.handleRSSSearch(indcfg, p)
	case searchTypeSeason:
		return s.handleSeasonSearch(indcfg, p)
	default:
		return nil
	}
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
	var pl pond.TaskGroup

	if p.searchtype == searchTypeRSS {
		pl = worker.WorkerPoolIndexerRSS.NewGroupContext(ctx)
	} else {
		pl = worker.WorkerPoolIndexer.NewGroupContext(ctx)
	}

	s.Done = false
	for _, indcfg := range s.Quality.IndexerCfg {
		indcfg := indcfg
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
		pl.Submit(func() {
			defer logger.HandlePanic()
			err := s.executeSearch(p, indcfg)
			if err == nil && !s.Done {
				s.Done = true
			}
		})
	}
	errjobs := pl.Wait()
	if errjobs != nil {
		logger.LogDynamicanyErr(
			"error",
			"Error searching indexers",
			errjobs,
		)
	}
}

// handleRSSSearch performs an RSS search for a specific indexer configuration.
// It queries the last RSS entry, updates the RSS history if a new entry is found,
// and handles potential errors during the search process.
// Returns true if the RSS search is successful, false otherwise.
func (s *ConfigSearcher) handleRSSSearch(indcfg *config.IndexersConfig, _ *searchParams) error {
	firstid, err := apiexternal.QueryNewznabRSSLast(
		indcfg,
		s.Quality,
		database.Getdatarow[string](
			false,
			"select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE",
			&s.Cfgp.NamePrefix,
			&s.Quality.Name,
			&indcfg.URL,
		),
		s.Quality.QualityIndexerByQualityAndTemplate(indcfg),
		&s.Raw,
	)

	if err == nil {
		if firstid != "" {
			addrsshistory(&indcfg.URL, &firstid, s.Quality, &s.Cfgp.NamePrefix)
		}
		return nil
	}

	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamicany1StringErr(
			"error",
			"Error searching indexer",
			err,
			logger.StrIndexer,
			indcfg.Name,
		)
		return err
	}
	return nil
}

// handleSeasonSearch performs a TV season search for a specific indexer configuration.
// It queries the TV series using TVDB ID and season information, and handles potential
// errors during the search process. Returns true if the season search is successful,
// false otherwise.
func (s *ConfigSearcher) handleSeasonSearch(indcfg *config.IndexersConfig, p *searchParams) error {
	_, _, err := apiexternal.QueryNewznabTvTvdb(
		indcfg,
		s.Quality,
		p.thetvdbid,
		s.Quality.QualityIndexerByQualityAndTemplate(indcfg),
		p.season,
		"",
		p.useseason,
		false,
		&s.Raw,
	)
	if err == nil {
		return nil
	}

	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) {
		logger.LogDynamicany1StringErr(
			"error",
			"Error searching indexer",
			err,
			logger.StrIndexer,
			indcfg.Name,
		)
		return err
	}
	return nil
}

// searchnameid is a method of the ConfigSearcher struct that performs a search for a media item
// by its name or ID. It checks various conditions to determine the appropriate search method,
// such as whether to use a query search or a search by ID, and whether to search for a movie
// or a TV series. It also handles errors that may occur during the search and logs them.
// The method returns a boolean indicating whether the search was successful.
func (s *ConfigSearcher) searchnameid(p *searchParams, indcfg *config.IndexersConfig) error {
	cats := s.Quality.QualityIndexerByQualityAndTemplate(indcfg)
	if cats == -1 {
		logger.LogDynamicany0("error", "Error getting quality config")
		return errors.New("Error getting quality config")
	}

	usequerysearch := p.titlesearch || (s.Cfgp.Useseries && p.e.NZB.TVDBID == 0) ||
		(!s.Cfgp.Useseries && p.e.Info.Imdb == "")

	var err error

	// ID-based search (more efficient)
	if !usequerysearch {
		err = s.performIDSearch(p, indcfg, cats)

		// Check if we should fallback to title search
		if s.Quality.SearchForTitleIfEmpty && len(s.Raw.Arr) == 0 {
			usequerysearch = true
		}
	}

	// Title-based search
	if usequerysearch {
		errsub := s.performTitleSearch(p, indcfg, cats)
		if err == nil && errsub != nil {
			err = errsub
		}
	}

	return err
}

// performTitleSearch extracted and optimized.
func (s *ConfigSearcher) performTitleSearch(
	p *searchParams,
	indcfg *config.IndexersConfig,
	cats int,
) error {
	var err error

	// Primary title search
	if p.titlesearch || s.Quality.BackupSearchForTitle {
		if err = s.executeQuerySearch(p, indcfg, cats, p.e.WantedTitle, "Title"); err == nil {
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) > 0 {
				return nil
			}
		}
	}

	// Alternative title search
	if s.Quality.BackupSearchForAlternateTitle {
		for _, altTitle := range p.sourcealttitles {
			if altTitle.Str1 == "" || altTitle.Str1 == p.e.WantedTitle {
				continue
			}

			searchstr := altTitle.Str1
			logger.StringRemoveAllRunesP(&searchstr, '&', '(', ')')

			if errsub := s.executeQuerySearch(p, indcfg, cats, searchstr, "Alternative Title"); errsub == nil {
				if s.Quality.CheckUntilFirstFound && len(s.Accepted) > 0 {
					break
				}
			} else {
				err = errsub
			}
		}
	}

	return err
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
		entry := &s.Accepted[idx]
		qualcfg := s.getentryquality(&entry.Info)
		if qualcfg == nil {
			logger.LogDynamicany(
				"info",
				"nzb found - start downloading",
				&logger.StrSeries,
				&s.Cfgp.Useseries,
				&logger.StrTitle,
				&entry.NZB.Title,
				&logger.StrMinPrio,
				&entry.MinimumPriority,
				&logger.StrPriority,
				&entry.Info.Priority,
			)
		} else {
			logger.LogDynamicany("info", "nzb found - start downloading", &logger.StrSeries, &s.Cfgp.Useseries, &logger.StrTitle, &entry.NZB.Title, &logger.StrQuality, &qualcfg.Name, &logger.StrMinPrio, &entry.MinimumPriority, &logger.StrPriority, &entry.Info.Priority)
		}
		if entry.NzbmovieID != 0 {
			downloaded = append(downloaded, entry.NzbmovieID)
			downloader.DownloadMovie(s.Cfgp, entry)
		} else if entry.NzbepisodeID != 0 {
			downloaded = append(downloaded, entry.NzbepisodeID)
			downloader.DownloadSeriesEpisode(s.Cfgp, entry)
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
func (s *ConfigSearcher) filterTestQualityWanted(
	entry *apiexternal.Nzbwithprio,
	quality *config.QualityConfig,
) bool {
	if quality == nil {
		return false
	}
	qualityChecks := []struct {
		enabled    bool
		entryValue string
		wantedList []string
		fieldName  string
		logField   string
	}{
		{
			enabled:    quality.WantedResolutionLen >= 1,
			entryValue: entry.Info.Resolution,
			wantedList: quality.WantedResolution,
			fieldName:  "Resolution",
			logField:   logger.StrFound,
		},
		{
			enabled:    quality.WantedQualityLen >= 1,
			entryValue: entry.Info.Quality,
			wantedList: quality.WantedQuality,
			fieldName:  "Quality",
			logField:   logger.StrFound,
		},
		{
			enabled:    quality.WantedAudioLen >= 1,
			entryValue: entry.Info.Audio,
			wantedList: quality.WantedAudio,
			fieldName:  "Audio",
			logField:   logger.StrFound,
		},
		{
			enabled:    quality.WantedCodecLen >= 1,
			entryValue: entry.Info.Codec,
			wantedList: quality.WantedCodec,
			fieldName:  "Codec",
			logField:   logger.StrFound,
		},
	}

	for i := range qualityChecks {
		check := &qualityChecks[i]
		if check.enabled && check.entryValue != "" {
			if !logger.SlicesContainsI(check.wantedList, check.entryValue) {
				reason := "unwanted " + check.fieldName
				logger.Logtype("debug", 0).
					Str(logger.StrReason, reason).
					Str(logger.StrTitle, entry.NZB.Title).
					Str(check.logField, check.entryValue).
					Strs(logger.StrWanted, check.wantedList).
					Msg(skippedstr)
				entry.Reason = reason
				s.logdenied("", entry)
				return true
			}
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
func (s *ConfigSearcher) searchparse(
	e *apiexternal.Nzbwithprio,
	alttitles []database.DbstaticTwoStringOneInt,
) {
	if len(s.Raw.Arr) == 0 {
		return
	}
	s.Denied = s.Raw.Arr
	s.Denied = s.Denied[:0]
	if s.Accepted == nil {
		s.Accepted = make([]apiexternal.Nzbwithprio, 0, 100)
	} else {
		s.Accepted = s.Accepted[:0]
	}
	for rawidx := range s.Raw.Arr {
		entry := &s.Raw.Arr[rawidx]
		if !s.validateBasicEntry(entry) {
			continue
		}

		if s.checkprocessed(&entry.NZB) {
			continue
		}

		if s.validateSize(entry) {
			continue
		}

		if s.searchActionType != logger.StrRss && s.checkcorrectid(e, entry) {
			continue
		}

		parser.ParseFileP(
			entry.NZB.Title,
			false,
			false,
			s.Cfgp,
			-1,
			&entry.Info,
		)
		if !s.Cfgp.Useseries && !entry.NZB.Indexer.TrustWithIMDBIDs {
			entry.Info.Imdb = ""
		}
		if s.Cfgp.Useseries && !entry.NZB.Indexer.TrustWithTVDBIDs {
			entry.Info.Tvdb = ""
		}
		if err := parser.GetDBIDs(&entry.Info, s.Cfgp, true); err != nil {
			s.logdenied1Str(
				err.Error(),
				entry,
				strCheckedFor,
				entry.Info.Title,
			)
			continue
		}
		var qual *config.QualityConfig
		if s.searchActionType == logger.StrRss {
			skip, q := s.getmediadatarss(entry, -1, false)
			if skip {
				continue
			}
			qual = q
		} else {
			if s.getmediadata(e, entry) {
				continue
			}
			qual = s.Quality
			entry.WantedAlternates = alttitles
		}
		// needs the identifier from getmediadata

		if qual == nil {
			qual = s.getentryquality(&entry.Info)
			if qual == nil {
				s.logdenied("unknown Quality", entry)
				continue
			}
		}

		if s.validateEntry(e, entry, qual) {
			continue
		}

		s.Accepted = append(s.Accepted, *entry)

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

// executeQuerySearch performs a Newznab query search for media content
// using the provided search parameters, indexer configuration, and quality settings.
// It returns a boolean indicating whether the search was successful.
//
// Parameters:
//   - p: Search parameters containing media entry details
//   - indcfg: Indexers configuration
//   - cats: Category identifier for the search
//   - searchTerm: Term used for searching media content
//   - searchType: Type of search being performed
//
// Returns:
//   - true if the search completes without errors, false otherwise
func (s *ConfigSearcher) executeQuerySearch(
	p *searchParams,
	indcfg *config.IndexersConfig,
	cats int,
	searchTerm, searchType string,
) error {
	_, _, err := apiexternal.QueryNewznabQuery(
		s.Cfgp, &p.e, indcfg, s.Quality, searchTerm, cats, &s.Raw,
	)

	if err != nil && !errors.Is(err, logger.ErrToWait) {
		p.e.Info.TempID = p.mediaid
		logsearcherror(
			"Error Searching Media by "+searchType,
			p.e.Info.TempID,
			s.Cfgp.Useseries,
			searchTerm,
			err,
		)
		return err
	}

	return nil
}

// validateSize checks if an NZB entry meets size-related validation criteria.
// It verifies whether the entry should be skipped based on empty size configuration
// and performs additional size filtering.
//
// Parameters:
//   - entry: The NZB entry to validate
//
// Returns:
//   - true if the entry should be filtered out due to size constraints, false otherwise
func (s *ConfigSearcher) validateSize(entry *apiexternal.Nzbwithprio) bool {
	if entry.NZB.Indexer == nil {
		return false
	}

	skipemptysize := s.Quality.QualityIndexerByQualityAndTemplateSkipEmpty(entry.NZB.Indexer)
	if !skipemptysize {
		if ok := config.TestSettingsList(entry.NZB.Indexer.Name); ok {
			skipemptysize = s.Quality.Indexer[0].SkipEmptySize
		} else if entry.NZB.Indexer.Getlistbyindexer() != nil {
			skipemptysize = s.Quality.Indexer[0].SkipEmptySize
		}
	}

	if skipemptysize && entry.NZB.Size == 0 {
		s.logdenied("no size", entry)
		return true
	}

	return s.filterSizeNzbs(entry)
}

// validateEntry combines multiple validation steps.
func (s *ConfigSearcher) validateEntry(
	e, entry *apiexternal.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	// History check
	if s.checkhistory(entry, qual) {
		return true
	}

	// Episode check for series
	if s.searchActionType != logger.StrRss && s.checkepisode(e, entry) {
		return true
	}

	// Regex filtering
	if s.filterRegexNzbs(entry, qual) {
		return true
	}

	// Priority calculation
	if entry.Info.Priority == 0 {
		parser.GetPriorityMapQual(&entry.Info, s.Cfgp, qual, false, true)
		if entry.Info.Priority == 0 {
			s.logdenied1Str("unknown Prio", entry, logger.StrFound, entry.Info.Title)
			return true
		}
	}

	entry.Info.StripTitlePrefixPostfixGetQual(qual)

	// Quality validation
	if s.filterTestQualityWanted(entry, qual) {
		return true
	}

	// Priority validation
	if s.getminimumpriority(entry, qual) {
		return true
	}

	if entry.MinimumPriority != 0 && entry.MinimumPriority == entry.Info.Priority {
		s.logdenied("same Prio", entry)
		return true
	}

	if entry.MinimumPriority != 0 {
		minDiff := qual.UseForPriorityMinDifference
		threshold := entry.MinimumPriority
		if minDiff != 0 {
			threshold += minDiff
		}

		if entry.Info.Priority <= threshold {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "lower Prio").
				Str(logger.StrTitle, entry.NZB.Title).
				Int(logger.StrFound, entry.Info.Priority).
				Int(logger.StrWanted, entry.MinimumPriority).
				Msg(skippedstr)
			entry.Reason = "lower Prio"
			s.logdenied("", entry)
			return true
		}
	}

	// Year check for movies
	if s.searchActionType != logger.StrRss && s.checkyear(e, entry, qual) {
		return true
	}

	// Title check
	if s.checktitle(entry, qual) {
		return true
	}

	logger.LogDynamicany(
		"debug", "Release ok",
		&logger.StrQuality, &qual.Name,
		&logger.StrTitle, &entry.NZB.Title,
		&logger.StrMinPrio, &entry.MinimumPriority,
		&logger.StrPriority, &entry.Info.Priority,
	)

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
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted Movie").
				Str(logger.StrTitle, entry.NZB.Title).
				Uint(logger.StrFound, entry.Info.MovieID).
				Uint(logger.StrWanted, sourceentry.NzbmovieID).
				Str(logger.StrImdb, sourceentry.Info.Imdb).
				Str(logger.StrConfig, s.Cfgp.NamePrefix).
				Msg(skippedstr)
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
func (s *ConfigSearcher) getmediadatarss(
	entry *apiexternal.Nzbwithprio,
	addinlistid int,
	addifnotfound bool,
) (bool, *config.QualityConfig) {
	if s.Cfgp.Useseries {
		return s.processSeriesRSS(entry)
	}
	return s.processMovieRSS(entry, addinlistid, addifnotfound)
}

// processSeriesRSS processes an NZB entry for a TV series in the RSS feed.
// It validates the series, episode, and database identifiers before further processing.
// Returns a boolean indicating whether the entry should be skipped, and the quality configuration.
// If no specific quality is set, it uses the default quality from the configuration.
func (s *ConfigSearcher) processSeriesRSS(
	entry *apiexternal.Nzbwithprio,
) (bool, *config.QualityConfig) {
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
		return false, config.GetSettingsQuality(s.Cfgp.DefaultQuality)
	}
	return false, config.GetSettingsQuality(entry.Quality)
}

// performIDSearch searches for media content using either IMDB (for movies) or TVDB (for TV series) identifiers
// across configured indexers. It performs a search based on the current configuration (series or movies)
// and handles potential search errors. Returns true if a search was successful or if the first matching
// result is found, depending on the quality configuration.
func (s *ConfigSearcher) performIDSearch(
	p *searchParams,
	indcfg *config.IndexersConfig,
	cats int,
) error {
	var err error

	if !s.Cfgp.Useseries && p.e.Info.Imdb != "" {
		_, _, err = apiexternal.QueryNewznabMovieImdb(
			indcfg, s.Quality, logger.Trim(p.e.Info.Imdb, 't'), cats, &s.Raw,
		)
	} else if s.Cfgp.Useseries && p.e.NZB.TVDBID != 0 {
		_, _, err = apiexternal.QueryNewznabTvTvdb(
			indcfg, s.Quality, p.e.NZB.TVDBID, cats,
			p.e.NZB.Season, p.e.NZB.Episode, true, true, &s.Raw,
		)
	}

	if err != nil && !errors.Is(err, logger.ErrToWait) {
		p.e.Info.TempID = p.mediaid
		logsearcherror("Error Searching Media by ID", p.e.Info.TempID, s.Cfgp.Useseries, "", err)
		return err
	}

	if err == nil {
		if s.Quality.CheckUntilFirstFound && len(s.Accepted) > 0 {
			return nil
		}
		return nil
	}

	return nil
}

// processMovieRSS handles processing of a movie RSS entry, including import checks, list assignment,
// and quality configuration. It determines whether a movie entry should be processed based on
// database movie ID, list configuration, and IMDB identifier. Returns a boolean indicating
// whether processing should be skipped and the associated quality configuration.
func (s *ConfigSearcher) processMovieRSS(
	entry *apiexternal.Nzbwithprio,
	addinlistid int,
	addifnotfound bool,
) (bool, *config.QualityConfig) {
	if addinlistid != -1 && s.Cfgp != nil {
		entry.Info.ListID = addinlistid
	}

	if entry.Info.DbmovieID == 0 &&
		(!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
		s.logdenied("unwanted DBMovie", entry)
		return true, nil
	}

	// Handle movie import if needed
	if addifnotfound && (entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0) &&
		logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
		if skip := s.handleMovieImport(entry, addinlistid); skip {
			return true, nil
		}
	}

	if entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0 {
		s.logdenied("unwanted Movie", entry)
		return true, nil
	}

	entry.Dbid = entry.Info.DbmovieID
	entry.NzbmovieID = entry.Info.MovieID

	getrssdata(&entry.Info.MovieID, false, entry)
	entry.Info.ListID = s.Cfgp.GetMediaListsEntryListID(entry.Listname)

	if entry.Quality == "" {
		return false, config.GetSettingsQuality(s.Cfgp.DefaultQuality)
	}
	return false, config.GetSettingsQuality(entry.Quality)
}

// handleMovieImport attempts to import a movie into the system when necessary.
// It checks if movie import is allowed, performs database lookups, and handles
// movie entry creation for a given NZB entry. Returns true if import should be
// skipped, false if successful.
//
// Parameters:
//   - entry: The NZB entry containing movie information
//   - addinlistid: The list ID to add the movie to
//
// Returns a boolean indicating whether movie import processing should be halted
func (s *ConfigSearcher) handleMovieImport(entry *apiexternal.Nzbwithprio, addinlistid int) bool {
	if addinlistid == -1 {
		return true
	}

	if entry.Info.DbmovieID == 0 {
		bl, err := importfeed.AllowMovieImport(&entry.NZB.IMDBID, s.Cfgp.Lists[addinlistid].CfgList)
		if err != nil {
			s.logdenied(err.Error(), entry)
			return true
		}
		if !bl {
			s.logdenied("unallowed DBMovie", entry)
			return true
		}

		entry.Info.DbmovieID, err = importfeed.JobImportMovies(
			entry.NZB.IMDBID,
			s.Cfgp,
			addinlistid,
			true,
		)
		if err != nil {
			s.logdenied(err.Error(), entry)
			return true
		}
	}

	if entry.Info.MovieID == 0 {
		if entry.Info.DbmovieID != 0 {
			database.Scanrowsdyn(
				false, database.QueryMoviesGetIDByDBIDListname,
				&entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name,
			)
		}

		if entry.Info.MovieID == 0 {
			err := importfeed.Checkaddmovieentry(
				&entry.Info.DbmovieID,
				&s.Cfgp.Lists[addinlistid],
				entry.NZB.IMDBID,
			)
			if err != nil {
				s.logdenied(err.Error(), entry)
				return true
			}

			if entry.Info.DbmovieID != 0 {
				database.Scanrowsdyn(
					false, database.QueryMoviesGetIDByDBIDListname,
					&entry.Info.MovieID, &entry.Info.DbmovieID, &s.Cfgp.Lists[addinlistid].Name,
				)
			}
		}
	}

	return false
}

// getrssdata retrieves RSS data for the given movie ID, and populates the
// provided Nzbwithprio entry with the retrieved data, including the
// DontSearch, DontUpgrade, Listname, Quality, and WantedTitle fields.
// If useseries is true, the function will use a different set of query
// parameters to retrieve the data.
func getrssdata(id *uint, useseries bool, entry *apiexternal.Nzbwithprio) {
	database.GetdatarowArgs(
		logger.GetStringsMap(useseries, "GetRSSData"),
		id,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.WantedTitle,
	)
}

// checktitle validates the title and alternate titles of the entry against
// the wanted title and quality configuration. It returns false if a match is
// found, or true to skip the entry if no match is found. This is an internal
// function used during search result processing.
func (s *ConfigSearcher) checktitle(
	entry *apiexternal.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	// Checktitle
	if !qual.CheckTitle {
		return false
	}
	if !qual.CheckTitleOnIDSearch && entry.IDSearched {
		return false
	}

	if !entry.NZB.Indexer.CheckTitleOnIDSearch && entry.IDSearched {
		return false
	}
	entry.Info.StripTitlePrefixPostfixGetQual(qual)

	wantedslug := logger.StringToSlug(entry.WantedTitle)
	if entry.WantedTitle != "" && qual.CheckTitle &&
		database.ChecknzbtitleB(
			entry.WantedTitle,
			wantedslug,
			entry.NZB.Title,
			qual.CheckYear1,
			entry.Info.Year,
		) {
		return false
	}
	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.Info.Title, ']') {
		if idx := strings.LastIndexByte(entry.Info.Title, ']'); idx != -1 &&
			idx < len(entry.Info.Title)-1 {
			trytitle = logger.TrimLeft(entry.Info.Title[idx+1:], '-', '.', ' ')
			if qual.CheckTitle && entry.WantedTitle != "" &&
				database.ChecknzbtitleB(
					entry.WantedTitle,
					wantedslug,
					trytitle,
					qual.CheckYear1,
					entry.Info.Year,
				) {
				return false
			}
		}
	}
	if entry.Dbid != 0 && len(entry.WantedAlternates) == 0 {
		entry.WantedAlternates = database.GetDbstaticTwoStringOneInt(
			database.Getentryalternatetitlesdirect(&entry.Dbid, s.Cfgp.Useseries),
			entry.Dbid,
		)
	}

	if entry.Info.Title == "" || len(entry.WantedAlternates) == 0 {
		s.logdenied("unwanted Title", entry)
		return true
	}
	for idx := range entry.WantedAlternates {
		if entry.WantedAlternates[idx].Str1 == "" {
			continue
		}
		if database.ChecknzbtitleB(
			entry.WantedAlternates[idx].Str1,
			entry.WantedAlternates[idx].Str2,
			entry.NZB.Title,
			qual.CheckYear1,
			entry.Info.Year,
		) {
			return false
		}

		if trytitle != "" && trytitle != entry.WantedAlternates[idx].Str1 &&
			trytitle != entry.WantedTitle {
			if database.ChecknzbtitleB(
				entry.WantedAlternates[idx].Str1,
				entry.WantedAlternates[idx].Str2,
				trytitle,
				qual.CheckYear1,
				entry.Info.Year,
			) {
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
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return false
	}
	if s.searchActionType == logger.StrRss && sourceentry.Info.Identifier == "" {
		return false
	}

	if sourceentry.Info.Identifier == "" {
		s.logdenied("no identifier", entry)
		return true
	}
	if logger.ContainsI(entry.NZB.Title, sourceentry.Info.Identifier) {
		return false
	}
	if s.checkAlternativeFormats(sourceentry, entry) {
		return false
	}
	if s.checkAlternativeIdentifier(sourceentry, entry) {
		return false
	}

	// Final validation for season/episode format
	if sourceentry.NZB.Season == "" || sourceentry.NZB.Episode == "" {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}
	return s.checkEpisodeFormat(sourceentry, entry, sourceentry.Info.Identifier)
}

// checkEpisodeFormat validates the episode identifier format for a given entry.
// It checks if the identifier matches the expected season and episode format.
// Returns false if the identifier is valid, true if the entry should be skipped.
func (s *ConfigSearcher) checkEpisodeFormat(
	sourceentry, entry *apiexternal.Nzbwithprio,
	identifier string,
) bool {
	sprefix, eprefix := "s", "e"
	if logger.ContainsI(identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else if !logger.ContainsI(identifier, "s") && !logger.ContainsI(identifier, "e") {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	if !logger.HasPrefixI(identifier, sprefix+sourceentry.NZB.Season) {
		s.logdenied1StrNo("unwanted Season", entry, &sourceentry.Info)
		return true
	}

	if !logger.ContainsI(identifier, sourceentry.NZB.Episode) {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	// Check episode suffixes
	for _, prefix := range episodePrefixes {
		if logger.HasSuffixI(identifier, eprefix+prefix+sourceentry.NZB.Episode) {
			return false
		}
	}

	// Check various suffix patterns
	suffixPatterns := []string{eprefix, logger.StrSpace, logger.StrDash}
	for _, pattern := range suffixPatterns {
		if logger.ContainsI(identifier, eprefix+sourceentry.NZB.Episode+pattern) {
			return false
		}
		for _, prefix := range episodePrefixes {
			if logger.HasSuffixI(identifier, eprefix+prefix+sourceentry.NZB.Episode+pattern) {
				return false
			}
		}
	}

	s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
	return true
}

// checkAlternativeFormats checks for alternative identifier formats when season and episode are not explicitly specified.
// It handles cases where the original identifier uses a hyphen separator, and checks if the entry title
// contains the same identifier with dot or space separators instead.
// Returns true if an alternative format match is found, otherwise false.
func (s *ConfigSearcher) checkAlternativeFormats(sourceentry, entry *apiexternal.Nzbwithprio) bool {
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" &&
		strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		// Check dot separator
		if strings.ContainsRune(entry.NZB.Title, '.') &&
			logger.ContainsI(entry.NZB.Title,
				logger.StringReplaceWith(sourceentry.Info.Identifier, '-', '.')) {
			return true
		}

		// Check space separator
		if strings.ContainsRune(entry.NZB.Title, ' ') &&
			logger.ContainsI(entry.NZB.Title,
				logger.StringReplaceWith(sourceentry.Info.Identifier, '-', ' ')) {
			return true
		}
	}
	return false
}

// checkAlternativeIdentifier checks for alternative identifier formats by converting and transforming the source identifier.
// It handles cases like removing leading 's/S', converting 'E/e' to 'x' format, and checking for alternative separators.
// Returns true if an alternative identifier match is found in the entry title, otherwise false.
func (s *ConfigSearcher) checkAlternativeIdentifier(
	sourceentry, entry *apiexternal.Nzbwithprio,
) bool {
	altIdentifier := logger.TrimLeft(sourceentry.Info.Identifier, 's', 'S', '0')

	// Convert E/e to x format
	if strings.ContainsRune(altIdentifier, 'E') {
		logger.StringReplaceWithP(&altIdentifier, 'E', 'x')
	} else if strings.ContainsRune(altIdentifier, 'e') {
		logger.StringReplaceWithP(&altIdentifier, 'e', 'x')
	}

	if logger.ContainsI(entry.NZB.Title, altIdentifier) {
		return true
	}

	// Check alternative separators for converted identifier
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" &&
		strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		if strings.ContainsRune(entry.NZB.Title, '.') &&
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', '.')) {
			return true
		}

		if strings.ContainsRune(entry.NZB.Title, ' ') &&
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', ' ')) {
			return true
		}
	}

	return false
}

// GetRSSFeed queries the RSS feed for the given media list, searches for and downloads new items,
// and adds them to the search results. It handles checking if the indexer is blocked,
// configuring the custom RSS feed URL, getting the last ID to prevent duplicates,
// parsing results, and updating the RSS history.
func (s *ConfigSearcher) getRSSFeed(
	listentry *config.MediaListsConfig,
	downloadentries bool,
) error {
	if s == nil {
		return errSearchvarEmpty
	}
	defer s.Close()
	if listentry.TemplateList == "" {
		return logger.ErrListnameTemplateEmpty
	}

	s.searchActionType = logger.StrRss

	intid := s.findIndexerConfig(listentry.TemplateList)
	if intid == -1 || s.Quality.Indexer[intid].TemplateRegex == "" {
		return errRegexEmpty
	}

	if s.isIndexerBlocked(listentry.CfgList.URL) {
		logger.LogDynamicany2StrAny(
			"debug", "Indexer temporarily disabled due to fail in last",
			logger.StrListname, listentry.TemplateList,
			strMinutes, -1*config.GetSettingsGeneral().FailedIndexerBlockTime,
		)
		return logger.ErrDisabled
	}

	if s.Cfgp == nil {
		return errOther
	}

	customindexer := setupIndexerConfig(listentry)
	firstid, err := apiexternal.QueryNewznabRSSLastCustom(
		customindexer,
		s.Quality,
		database.Getdatarow[string](
			false,
			"select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE",
			&listentry.TemplateList,
			&s.Quality.Name,
		),
		-1,
		&s.Raw,
	)
	if err != nil {
		return err
	}

	s.processSearchResults(
		downloadentries,
		firstid,
		&listentry.CfgList.URL,
		s.Quality,
		&listentry.TemplateList,
	)

	return nil
}

// isIndexerBlocked checks if an indexer is temporarily blocked due to previous failures.
// It returns true if the indexer has failed within the configured block interval,
// preventing repeated attempts to use a problematic indexer.
// Returns false if blocking is disabled or no recent failures are found.
func (s *ConfigSearcher) isIndexerBlocked(url string) bool {
	if config.GetSettingsGeneral().FailedIndexerBlockTime == 0 {
		return false
	}

	blockinterval := -1 * config.GetSettingsGeneral().FailedIndexerBlockTime
	intval := logger.TimeGetNow().Add(time.Minute * time.Duration(blockinterval))

	return database.Getdatarow[uint](
		false,
		"select count() from indexer_fails where last_fail > ? and indexer = ?",
		&intval, &url,
	) >= 1
}

// findIndexerConfig searches for an indexer configuration in the Quality.Indexer slice
// based on the provided templateList. It performs a case-insensitive comparison
// of the TemplateIndexer field. Returns the index of the matching indexer or -1 if no match is found.
func (s *ConfigSearcher) findIndexerConfig(templateList string) int {
	for idx := range s.Quality.Indexer {
		if s.Quality.Indexer[idx].TemplateIndexer == templateList ||
			strings.EqualFold(s.Quality.Indexer[idx].TemplateIndexer, templateList) {
			return idx
		}
	}
	return -1
}

// MovieFillSearchVar fills the search variables for the given movie ID.
// It queries the database to get the movie details and configures the
// search based on priorities, upgrade settings etc.
func (s *ConfigSearcher) movieFillSearchVar(p *searchParams) error {
	if p.e.NzbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}
	database.GetdatarowArgs(
		"select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
		&p.e.NzbmovieID,
		&p.e.Dbid,
		&p.e.DontSearch,
		&p.e.DontUpgrade,
		&p.e.Listname,
		&p.e.Quality,
		&p.e.Info.Year,
		&p.e.Info.Imdb,
		&p.e.WantedTitle,
	)
	if p.e.Dbid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	if p.e.DontSearch {
		return logger.ErrDisabled
	}
	if p.e.Info.Year == 0 {
		return errYearEmpty
	}
	s.setQualityConfig(&p.e.Quality)

	p.e.MinimumPriority, _ = Getpriobyfiles(
		s.Cfgp.Useseries,
		&p.e.NzbmovieID,
		false,
		-1,
		s.Quality,
		false,
	)

	var err error
	s.searchActionType, err = getsearchtype(p.e.MinimumPriority, p.e.DontUpgrade, false)
	if err != nil {
		return err
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
	database.GetdatarowArgs(
		"select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?",
		&p.e.NzbepisodeID,
		&p.e.Info.DbserieEpisodeID,
		&p.e.Dbid,
		&p.e.Info.SerieID,
		&p.e.DontSearch,
		&p.e.DontUpgrade,
		&p.e.Quality,
		&p.e.Listname,
		&p.e.NZB.TVDBID,
		&p.e.WantedTitle,
		&p.e.NZB.Season,
		&p.e.NZB.Episode,
		&p.e.Info.Identifier,
	)
	if p.e.Info.DbserieEpisodeID == 0 || p.e.Dbid == 0 || p.e.Info.SerieID == 0 {
		return logger.ErrNotFound
	}
	if p.e.DontSearch {
		return logger.ErrDisabled
	}
	p.e.Info.DbserieID = p.e.Dbid

	s.setQualityConfig(&p.e.Quality)

	p.e.MinimumPriority, _ = Getpriobyfiles(
		s.Cfgp.Useseries,
		&p.e.NzbepisodeID,
		false,
		-1,
		s.Quality,
		false,
	)

	p.e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(p.e.Listname)
	var err error
	s.searchActionType, err = getsearchtype(p.e.MinimumPriority, p.e.DontUpgrade, false)
	return err
}

// setQualityConfig sets the quality configuration for the ConfigSearcher.
// If the current quality is nil or different from the provided quality name,
// it updates the quality based on the given name or falls back to the default quality.
// If the quality name is empty or not found in the settings, it uses the default quality.
func (s *ConfigSearcher) setQualityConfig(qualityName *string) {
	if s.Quality == nil || s.Quality.Name != *qualityName {
		if *qualityName == "" {
			s.Quality = config.GetSettingsQuality(s.Cfgp.DefaultQuality)
		} else {
			var ok bool
			s.Quality, ok = config.GetSettingsQualityOk(*qualityName)
			if !ok {
				s.Quality = config.GetSettingsQuality(s.Cfgp.DefaultQuality)
			}
		}
	}
}

// filterSizeNzbs checks if the NZB entry size is within the configured
// minimum and maximum size limits, and returns true if it should be
// rejected based on its size.
func (s *ConfigSearcher) filterSizeNzbs(entry *apiexternal.Nzbwithprio) bool {
	if entry.NZB.Size == 0 {
		return false // Skip size check if no size info
	}
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
func (s *ConfigSearcher) filterRegexNzbs(
	entry *apiexternal.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	regexcfg := entry.Getregexcfg(qual)
	if regexcfg == nil {
		s.logdenied1Str("Denied by Regex", entry, strRegexEmpty, "")
		return true
	}

	if regexcfg.RequiredLen >= 1 {
		var bl bool
		for idx := range regexcfg.Required {
			if database.RegexGetMatchesFind(regexcfg.Required[idx], entry.NZB.Title, 1) {
				bl = true
				break
			}
		}
		if !bl {
			s.logdenied1Str("not matched required", entry, strCheckedFor, regexcfg.Required[0])
			return true
		}
	}

	for idxr := range regexcfg.Rejected {
		if !database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.NZB.Title, 1) {
			continue
		}

		// Check if wanted title matches (allowed)
		if database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.WantedTitle, 1) {
			continue
		}
		bl := false
		for idx := range entry.WantedAlternates {
			if entry.WantedTitle != entry.WantedAlternates[idx].Str1 &&
				database.RegexGetMatchesFind(
					regexcfg.Rejected[idxr],
					entry.WantedAlternates[idx].Str1,
					1,
				) {
				bl = true
				break
			}
		}
		if !bl {
			s.logdenied1Str(
				"Denied by Regex",
				entry,
				strRejectedby,
				regexcfg.Rejected[idxr],
			)
			return true
		}
	}
	return false
}

// checkhistory checks if the given entry is already in the history cache
// to avoid duplicate downloads. It checks based on the download URL and title.
// Returns true if a duplicate is found, false otherwise.
func (s *ConfigSearcher) checkhistory(
	entry *apiexternal.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	if entry.NZB.DownloadURL != "" &&
		database.CheckcachedURLHistory(s.Cfgp.Useseries, &entry.NZB.DownloadURL) {
		s.logdenied("already downloaded url", entry)
		return true
	}
	if entry.NZB.Indexer == nil ||
		!qual.QualityIndexerByQualityAndTemplateCheckTitle(entry.NZB.Indexer) {
		return false
	}

	if entry.NZB.Title != "" &&
		database.CheckcachedTitleHistory(s.Cfgp.Useseries, &entry.NZB.Title) {
		s.logdenied("already downloaded title", entry)
		return true
	}
	return false
}

// checkprocessed checks if the given entry is already in the denied or accepted lists to avoid duplicate processing.
// It loops through the denied and accepted entries and returns true if it finds a match on the download URL or title.
// Otherwise returns false. Part of ConfigSearcher.
func (s *ConfigSearcher) checkprocessed(entry *apiexternal.Nzb) bool {
	for idx := range s.Denied {
		if s.Denied[idx].NZB.DownloadURL == entry.DownloadURL ||
			s.Denied[idx].NZB.Title == entry.Title {
			return true
		}
	}
	for idx := range s.Accepted {
		if s.Accepted[idx].NZB.DownloadURL == entry.DownloadURL ||
			s.Accepted[idx].NZB.Title == entry.Title {
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
	if s.searchActionType == logger.StrRss || sourceentry == nil {
		return false
	}
	if !s.Cfgp.Useseries {
		if entry.NZB.IMDBID != "" && entry.NZB.IMDBID != "tt0000000" &&
			sourceentry.Info.Imdb != "" &&
			sourceentry.Info.Imdb != entry.NZB.IMDBID {
			if logger.TrimLeft(
				sourceentry.Info.Imdb,
				't',
				'0',
			) != logger.TrimLeft(
				entry.NZB.IMDBID,
				't',
				'0',
			) {
				logger.Logtype("debug", 0).
					Str(logger.StrReason, "not matched imdb").
					Str(logger.StrTitle, entry.NZB.Title).
					Str(logger.StrFound, entry.NZB.IMDBID).
					Str(logger.StrWanted, sourceentry.Info.Imdb).
					Msg(skippedstr)
				entry.Reason = "not matched imdb"
				s.logdenied("", entry)
				return true
			}
		}
		return false
	}
	if sourceentry.NZB.TVDBID != 0 && entry.NZB.TVDBID != 0 &&
		sourceentry.NZB.TVDBID != entry.NZB.TVDBID {
		logger.Logtype("debug", 0).
			Str(logger.StrReason, "not matched tvdb").
			Str(logger.StrTitle, entry.NZB.Title).
			Int(logger.StrFound, entry.NZB.TVDBID).
			Int(logger.StrWanted, sourceentry.NZB.TVDBID).
			Msg(skippedstr)
		entry.Reason = "not matched tvdb"
		s.logdenied("", entry)
		return true
	}
	return false
}

// checkyear validates the year in the entry title against the year
// configured for the wanted entry. It returns false if a match is found,
// or true to skip the entry if no match is found. This is used during
// search result processing to filter entries by year.
func (s *ConfigSearcher) checkyear(
	sourceentry, entry *apiexternal.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	if s.Cfgp.Useseries || s.searchActionType == logger.StrRss || sourceentry == nil {
		return false
	}

	if sourceentry.Info.Year == 0 {
		s.logdenied("no year", entry)
		return true
	}
	if qual.CheckYear || qual.CheckYear1 {
		targetYear := sourceentry.Info.Year

		// Check exact year
		if logger.ContainsInt(entry.NZB.Title, targetYear) {
			return false
		}

		// Check year +/- 1 if enabled
		if qual.CheckYear1 {
			if logger.ContainsInt(entry.NZB.Title, targetYear+1) ||
				logger.ContainsInt(entry.NZB.Title, targetYear-1) {
				return false
			}
		}
	}

	s.logdenied1UInt16("unwanted Year", entry, sourceentry.Info.Year)
	return true
}

// searchSeriesRSSSeason searches configured indexers for the given TV series
// season using the RSS search APIs. It handles executing searches across
// enabled newznab indexers, parsing results, and adding accepted entries to
// the search results. Returns the searcher and error if any.
func (s *ConfigSearcher) searchSeriesRSSSeason(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	thetvdbid int,
	season string,
	useseason, downloadentries, autoclose bool,
) error {
	if s == nil {
		return errSearchvarEmpty
	}
	if autoclose {
		defer s.Close()
	}
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	s.Quality = quality

	p := plsearchparam.Get()
	defer plsearchparam.Put(p)
	p.searchtype = searchTypeSeason
	p.thetvdbid = thetvdbid
	p.season = season
	p.useseason = useseason

	logger.LogDynamicany2StrAny(
		"info",
		"Search for season",
		logger.StrSeason,
		p.season,
		logger.StrTvdb,
		&p.thetvdbid,
	) // logpointerr
	s.searchindexers(ctx, false, p)

	if s.Done {
		s.processSearchResults(downloadentries, "", nil, s.Quality, nil)

		logger.Logtype("info", 0).
			Int(logger.StrTvdb, p.thetvdbid).
			Str(logger.StrSeason, p.season).
			Int(logger.StrAccepted, len(s.Accepted)).
			Int(logger.StrDenied, len(s.Denied)).
			Msg("Ended Search for season")
	}
	return nil
}

// SearchSeriesRSSSeasons searches the RSS feeds for missing episodes for
// random series. It selects up to 20 random series that have missing
// episodes, gets the distinct seasons with missing episodes for each,
// and searches the RSS feeds for those seasons.
func SearchSeriesRSSSeasons(cfgp *config.MediaTypeConfig) error {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	return searchseasons(
		context.Background(),
		cfgp,
		logger.JoinStrings(
			"select id, dbserie_id from series where listname in (?",
			cfgp.ListsQu,
			") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20",
		),
		20,
		"select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )",
		"select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )",
		args,
	)
}

// SearchSeriesRSSSeasonsAll searches all seasons for series matching the given
// media type config. It searches series that have missing episodes and calls
// searchseasons to perform the actual search.
func SearchSeriesRSSSeasonsAll(cfgp *config.MediaTypeConfig) error {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	return searchseasons(
		context.Background(),
		cfgp,
		logger.JoinStrings(
			"select id, dbserie_id from series where listname in (?",
			cfgp.ListsQu,
			") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1",
		),
		database.Getdatarow[uint](false, "select count() from series"),
		"select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )",
		"select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )",
		args,
	)
}

// searchseason searches for missing episodes for a specific series and season.
// It retrieves the distinct seasons with missing episodes for the given series,
// and then searches the RSS feeds of the enabled indexers for those seasons.
// The results are added to the DownloadResults instance.
func searchseason(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	row *database.DbstaticTwoUint,
	queryseason, queryseasoncount string,
) error {
	seasonCount := database.Getdatarow[uint](
		false,
		queryseasoncount,
		&row.Num2,
		&row.Num1,
		&row.Num1,
		&row.Num2,
	)
	if seasonCount == 0 {
		return nil // errors.New("No seasons found")
	}

	// Get list ID once
	listid := database.GetMediaListIDGetListname(cfgp, &row.Num1)
	if listid == -1 {
		return errors.New("List not found")
	}
	tvdbid := database.Getdatarow[int](
		false,
		"select thetvdb_id from dbseries where id = ?",
		&row.Num2,
	)
	if tvdbid == 0 {
		return nil // errors.New("TVDB ID not found")
	}
	seasons := database.GetrowsN[string](
		false,
		seasonCount,
		queryseason,
		&row.Num2,
		&row.Num1,
		&row.Num1,
		&row.Num2,
	)

	var err error
	for _, season := range seasons {
		if errsub := NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, nil).searchSeriesRSSSeason(
			ctx,
			cfgp,
			cfgp.Lists[listid].CfgQuality,
			tvdbid,
			season,
			true,
			true,
			true,
		); errsub != nil {
			err = errsub
		}
	}
	return err
}

// SearchSerieRSSSeasonSingle searches for a single season of a series.
// It takes the series ID, season number, whether to search the full season or missing episodes,
// media type config, whether to auto close the results, and a pointer to search results.
// It returns a config searcher instance and error.
// It queries the database to map the series ID to thetvdb ID, gets the quality config,
// calls the search function, handles errors, downloads results,
// closes the results if autoclose is true, and returns the config searcher.
func SearchSerieRSSSeasonSingle(
	serieid *uint,
	season string,
	useseason bool,
	cfgp *config.MediaTypeConfig,
) (*ConfigSearcher, error) {
	if serieid == nil || *serieid == 0 {
		return nil, logger.ErrNotFound
	}

	var dbserieid uint
	var tvdb int
	database.GetdatarowArgs(
		"select s.dbserie_id, d.thetvdb_id from series s inner join dbseries d on d.id = s.dbserie_id where s.id = ?",
		serieid,
		&dbserieid,
		&tvdb,
	)
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
	err := results.searchSeriesRSSSeason(
		context.Background(),
		cfgp,
		cfgp.Lists[listid].CfgQuality,
		tvdb,
		season,
		useseason,
		true,
		false,
	)
	if err != nil && !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
		logger.Logtype("error", 0).
			Err(err).
			Uint(logger.StrID, *serieid).
			Msg("Season Search Inc Failed")
		results.Close()
		return nil, err
	}
	return results, nil
}

// addrsshistory updates the rss history table with the last processed item id
// for the given rss feed url, quality profile name, and config name. It will
// insert a new row if one does not exist yet for that combination.
func addrsshistory(urlv, lastid *string, quality *config.QualityConfig, configv *string) {
	id := database.Getdatarow[uint](
		false,
		"select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE",
		configv,
		&quality.Name,
		urlv,
	)
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
func getsearchtype(minimumPriority int, dont, force bool) (string, error) {
	switch {
	case minimumPriority == 0:
		return "missing", nil
	case dont && !force:
		return "", logger.ErrDisabled
	default:
		return "upgrade", nil
	}
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

// searchlog logs information about the results of a media search, including the number of accepted and denied entries.
// The function takes the following parameters:
// - typev: a string indicating the type of log message (e.g. "info", "error")
// - msg: a string describing the search event
// - p: a pointer to a searchParams struct containing information about the search.
func (s *ConfigSearcher) searchlog(typev, msg string, p *searchParams) {
	// Only create log entry if we have results to report
	if len(s.Accepted) == 0 && len(s.Denied) == 0 && typev != "error" {
		return
	}

	logv := logger.Logtype(typev, 1).
		Uint(logger.StrID, p.mediaid).
		Bool(logger.StrSeries, s.Cfgp.Useseries).
		Bool(strTitlesearch, p.titlesearch)

	if len(s.Accepted) >= 1 {
		logv.Int(logger.StrAccepted, len(s.Accepted))
	}
	if len(s.Denied) >= 1 {
		logv.Int(logger.StrDenied, len(s.Denied))
	}

	logv.Msg(msg)
}

// logsearcherror logs an error during a search operation with details about the media search context.
// It skips logging for expected errors when in debug mode to reduce noise.
// The function logs the error with media ID, series flag, title, and the specific error message.
func logsearcherror(msg string, id uint, useseries bool, title string, err error) {
	// Skip logging for expected errors in debug mode
	if database.DBLogLevel == logger.StrDebug && isExpectedError(err) {
		return
	}

	logger.Logtype("error", 1).
		Uint(strMediaid, id).
		Bool(logger.StrSeries, useseries).
		Str(logger.StrTitle, title).
		Err(err).
		Msg(msg)
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
		logger.Logtype("debug", 1).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Msg(skippedstr)
	}
	s.deniedappend(entry)
}

// logdenied1Int64 logs a denied entry with the given reason and the NZB size as an additional int64 field.
// It sets the reason and additional int64 field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1Int64(reason string, entry *apiexternal.Nzbwithprio) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Int64(logger.StrFound, entry.NZB.Size).
			Msg(skippedstr)
		entry.AdditionalReasonInt = entry.NZB.Size
	}
	s.deniedappend(entry)
}

// logdenied1UInt16 logs a denied entry with the given reason and an additional uint16 field.
// It sets the reason and additional uint16 field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1UInt16(
	reason string,
	entry *apiexternal.Nzbwithprio,
	value1 uint16,
) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Uint16(logger.StrWanted, value1).
			Msg(skippedstr)
		entry.AdditionalReasonInt = int64(value1)
	}
	s.deniedappend(entry)
}

// logdenied1Str logs a denied entry with the given reason and an additional string field.
// It sets the reason and additional string field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1Str(
	reason string,
	entry *apiexternal.Nzbwithprio,
	field1 string,
	value1 string,
) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Str(field1, value1).
			Msg(skippedstr)
		entry.AdditionalReasonStr = value1
	}
	s.deniedappend(entry)
}

// logdenied1StrNo logs a denied entry with the given reason and an additional string field.
// It sets the reason and additional string field on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied1StrNo(
	reason string,
	entry *apiexternal.Nzbwithprio,
	value1 *database.ParseInfo,
) {
	if reason != "" {
		entry.Reason = reason
		logger.Logtype("debug", 1).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Str(strIdentifier, value1.Identifier).
			Msg(skippedstr)
	}
	s.deniedappend(entry)
}

// searchseasons searches for missing episodes for series matching the given
// configuration and quality settings. It selects a random sample of series
// to search, gets the distinct seasons with missing episodes for each, and
// searches those seasons on the RSS feeds of the enabled indexers. Results
// are added to the passed in DownloadResults instance.
func searchseasons(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	queryrange string,
	queryrangecount uint,
	queryseason, queryseasoncount string,
	args *logger.Arrany,
) error {
	tbl := database.GetrowsN[database.DbstaticTwoUint](
		false,
		queryrangecount,
		queryrange,
		args.Arr...)
	var err error
	for idx := range tbl {
		if errsub := searchseason(ctx, cfgp, &tbl[idx], queryseason, queryseasoncount); errsub != nil {
			err = errsub
		}
	}
	return err
}

// Getpriobyfiles returns the minimum priority of existing files for the given media
// ID, and optionally returns a slice of file paths for existing files below
// the given wanted priority. If useseries is true it will look up series IDs instead of media IDs.
// If id is nil it will return 0 priority.
// If useall is true it will include files marked as deleted.
// If wantedprio is -1 it will not return any file paths.
func Getpriobyfiles(
	useseries bool,
	id *uint,
	useall bool,
	wantedprio int,
	qualcfg *config.QualityConfig,
	getold bool,
) (int, []string) {
	if qualcfg == nil || id == nil || *id == 0 {
		return 0, nil
	}
	arr := database.Getrowssize[database.FilePrio](
		false,
		logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID),
		logger.GetStringsMap(useseries, logger.DBFilePrioFilesByID),
		id,
	)

	if len(arr) == 0 {
		return 0, nil
	}

	var minPrio int
	var oldf []string
	if getold && wantedprio != -1 {
		oldf = make([]string, 0, len(arr))
	}
	var prio int

	for i := range arr {
		prio = calculateFilePriority(&arr[i], qualcfg, useall)

		if minPrio == 0 || prio > minPrio {
			minPrio = prio
		}

		if getold && wantedprio != -1 && wantedprio > prio {
			oldf = append(oldf, arr[i].Location)
		}
	}

	if wantedprio == -1 || !getold {
		return minPrio, nil
	}
	return minPrio, oldf
}

// calculateFilePriority determines the priority of a file based on its resolution, quality, codec, audio attributes,
// and optional special attributes like 'proper', 'extended', or 'repack' flags.
// The priority is calculated using quality configuration settings and can consider all attributes or
// selectively use specific attributes based on configuration.
// Returns an integer representing the file's priority, with optional bonuses for special attributes.
func calculateFilePriority(
	file *database.FilePrio,
	qualcfg *config.QualityConfig,
	useall bool,
) int {
	if file == nil || qualcfg == nil {
		return 0
	}
	var r, q, a, c uint

	if useall {
		r, q, c, a = file.ResolutionID, file.QualityID, file.CodecID, file.AudioID
	} else {
		if qualcfg.UseForPriorityResolution {
			r = file.ResolutionID
		}
		if qualcfg.UseForPriorityQuality {
			q = file.QualityID
		}
		if qualcfg.UseForPriorityAudio {
			a = file.AudioID
		}
		if qualcfg.UseForPriorityCodec {
			c = file.CodecID
		}
	}

	// Try wanted priorities first
	intid := parser.Findpriorityidxwanted(r, q, c, a, qualcfg)
	if intid == -1 {
		intid = parser.Findpriorityidx(r, q, c, a, qualcfg)
	}

	var prio int
	if intid != -1 {
		prio = parser.GetwantedArrPrio(intid)
	} else {
		logger.LogDynamicany2Str("debug", "prio not found", "in", qualcfg.Name, "searched for",
			parser.BuildPrioStr(file.ResolutionID, file.QualityID, file.CodecID, file.AudioID))
		return 0
	}

	// Add bonuses for special attributes
	if qualcfg.UseForPriorityOther || useall {
		if file.Proper {
			prio += 5
		}
		if file.Extended {
			prio += 2
		}
		if file.Repack {
			prio++
		}
	}

	return prio
}

// validateBasicEntry performs basic validation on an entry.
func (s *ConfigSearcher) validateBasicEntry(entry *apiexternal.Nzbwithprio) bool {
	if entry.NZB.DownloadURL == "" {
		s.logdenied("no url", entry)
		return false
	}
	if entry.NZB.Title == "" {
		s.logdenied("no title", entry)
		return false
	}

	entry.NZB.Title = logger.Trim(entry.NZB.Title, ' ')
	if len(entry.NZB.Title) <= 3 {
		s.logdenied("short title", entry)
		return false
	}

	return true
}

// setupIndexerConfig creates a custom indexer configuration.
func setupIndexerConfig(listentry *config.MediaListsConfig) *config.IndexersConfig {
	customindexer := *config.GetSettingsIndexer(listentry.TemplateList)
	customindexer.Name = listentry.TemplateList
	customindexer.Customrssurl = listentry.CfgList.URL
	customindexer.URL = listentry.CfgList.URL
	customindexer.MaxEntries = logger.StringToUInt16(listentry.CfgList.Limit)
	return &customindexer
}

// getentryquality returns the quality config for the given entry.
// If the entry is for a movie, it gets the config from the movies database using the movie ID.
// If the entry is for a TV episode, it gets the config from the series database using the episode ID.
// If no ID is set, it returns nil.
func (s *ConfigSearcher) getentryquality(entry *database.ParseInfo) *config.QualityConfig {
	switch {
	case entry.MovieID != 0:
		return database.GetMediaQualityConfig(s.Cfgp, &entry.MovieID)
	case entry.SerieEpisodeID != 0:
		return database.GetMediaQualityConfig(s.Cfgp, &entry.SerieEpisodeID)
	default:
		return nil
	}
}

// processSearchResults handles the common pattern of processing search results.
func (s *ConfigSearcher) processSearchResults(
	downloadentries bool,
	firstid string,
	url *string,
	quality *config.QualityConfig,
	configPrefix *string,
) {
	if len(s.Raw.Arr) == 0 {
		return
	}
	if firstid != "" {
		addrsshistory(url, &firstid, quality, configPrefix)
	}
	s.searchparse(nil, nil)
	if downloadentries && len(s.Accepted) > 0 {
		s.Download()
	}
}

// getminimumpriority checks the minimum priority configured for the entry's movie or series.
// It sets the MinimumPriority field on the entry based on priorities configured in the quality
// profiles. Returns true to skip the entry if upgrade/search is disabled or priority does not meet
// configured minimum.
func (s *ConfigSearcher) getminimumpriority(
	entry *apiexternal.Nzbwithprio,
	cfgqual *config.QualityConfig,
) bool {
	if entry.MinimumPriority != 0 {
		return false
	}

	// Set temp ID for priority lookup
	if !s.Cfgp.Useseries {
		entry.Info.TempID = entry.NzbmovieID
	} else {
		entry.Info.TempID = entry.NzbepisodeID
	}

	entry.MinimumPriority, _ = Getpriobyfiles(
		s.Cfgp.Useseries,
		&entry.Info.TempID,
		false,
		-1,
		cfgqual,
		false,
	)

	// Check upgrade/search permissions
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

// isExpectedError checks if the given error is a known expected error type.
// It returns true if the error is either "no results" or "wait" error.
func isExpectedError(err error) bool {
	return errors.Is(err, logger.Errnoresults) || errors.Is(err, logger.ErrToWait)
}
