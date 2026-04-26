package searcher

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/newznab"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks" // Register audiobook handler
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/books"      // Register book handler
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"     // Register movie handler
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/music"  // Register music handler
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series" // Register series handler
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/alitto/pond/v2"
)

// ConfigSearcher is a struct containing configuration and search results.
type ConfigSearcher struct {
	// Dl contains the search results
	Raw apiexternal.NzbSlice
	// Denied is a slice containing denied apiexternal.Nzbwithprio results
	Denied []apiexternal_v2.Nzbwithprio
	// Accepted is a slice containing accepted apiexternal.Nzbwithprio results
	Accepted []apiexternal_v2.Nzbwithprio
	// searchActionType is a string indicating the search action type
	searchActionType string // missing,upgrade,rss
	Done             int32  // atomic flag: 0 = false, 1 = true
	// isArtistAuthorSearch indicates this is an artist/author-based search
	// which should always use getmediadatarss for result processing
	isArtistAuthorSearch bool
	// isSeasonSearch indicates this is a season or date-series name search
	// (searchTypeSeason or searchTypeSeasonDate) that should use getmediadatarss
	isSeasonSearch bool
	// Cfgp is a pointer to a MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// Quality is a pointer to a QualityConfig
	Quality *config.QualityConfig

	// Optimization: Pre-allocated buffers for frequent operations
	episodeBuffer [4]string // Pre-allocated buffer for episode prefix array
	// qualityChecks    [4]qualityCheck // Pre-allocated for quality validation
	indexerConfigMap map[string]int // Cache for indexer config lookups

	// Optimization: Maps for O(1) duplicate checking
	downloadedMap   map[uint]struct{}   // O(1) lookup for downloaded items
	processedURLs   map[string]struct{} // O(1) lookup for processed URLs
	processedTitles map[string]struct{} // O(1) lookup for processed titles
}

type searchParams struct {
	e               apiexternal_v2.Nzbwithprio
	sourcealttitles []syncops.DbstaticTwoStringOneInt
	season          string
	searchtype      int
	thetvdbid       int
	mediaid         uint
	useseason       bool
	titlesearch     bool
}

const (
	skippedstr           = "Skipped"
	searchTypeMissing    = 1
	searchTypeRSS        = 2
	searchTypeSeason     = 3
	searchTypeSeasonDate = 4

	// Optimized buffer sizes for better memory management.
	defaultRawCapacity      = 100 // Start small, grow dynamically (was 8000)
	defaultDeniedCapacity   = 500 // Pre-allocate reasonable size
	defaultAcceptedCapacity = 50  // Most searches return fewer results
	defaultDownloadedCap    = 100 // For downloaded ID tracking
	defaultProcessedCap     = 200 // For URL/title duplicate tracking

	// Pool sizes.
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
	errSearchvarEmpty     = errors.New("searchvar empty")
	errSearchIDEmpty      = errors.New("search id empty")
	errSearchQualityEmpty = errors.New("search quality empty")
	errRegexEmpty         = errors.New("regex template empty")
	plsearcher            pool.Poolobj[ConfigSearcher]
	plsearchparam         pool.Poolobj[searchParams]
)

// clearNzbSlice efficiently clears the contents of an Nzbwithprio slice by resetting
// AdditionalReason to nil and clearing the Episodes and Languages maps for each element.
// This helps prepare the slice for reuse without reallocating memory.
func clearNzbSlice(slice []apiexternal_v2.Nzbwithprio) {
	for i := range slice {
		slice[i].AdditionalReason = nil
		clear(slice[i].Info.Episodes)
		clear(slice[i].Info.Languages)
	}
}

// reset clears and resets the ConfigSearcher's internal state, efficiently resetting search-related slices
// and preparing the searcher for a new search operation. It zeroes out the search action type,
// marks the search as not done, and clears the Denied, Accepted, and Raw result arrays.
//
// Optimized: Uses Go 1.21+ clear() builtin for maps which is more efficient than
// manual delete loops as it can reset the map in a single operation.
func (s *ConfigSearcher) reset() {
	s.searchActionType = ""
	s.isArtistAuthorSearch = false
	atomic.StoreInt32(&s.Done, 0)

	// Clear slices efficiently - reset internal references before truncating
	clearNzbSlice(s.Denied)
	clearNzbSlice(s.Accepted)
	clearNzbSlice(s.Raw.Arr)

	s.Denied = s.Denied[:0]
	s.Accepted = s.Accepted[:0]
	s.Raw.Arr = s.Raw.Arr[:0]

	// Clear optimization maps for reuse using Go 1.21+ clear() builtin
	// This is more efficient than manual delete loops
	clear(s.downloadedMap)
	clear(s.processedURLs)
	clear(s.processedTitles)
	clear(s.indexerConfigMap)
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
	plsearchparam.Init(200, paramPoolSize, nil, func(cs *searchParams) bool {
		*cs = searchParams{}
		return false
	})
	plsearcher.Init(200, searcherPoolSize, func(cs *ConfigSearcher) {
		cs.Raw.Arr = make([]apiexternal_v2.Nzbwithprio, 0, defaultRawCapacity)
		cs.Denied = make([]apiexternal_v2.Nzbwithprio, 0, defaultDeniedCapacity)
		cs.Accepted = make([]apiexternal_v2.Nzbwithprio, 0, defaultAcceptedCapacity)
		cs.indexerConfigMap = make(map[string]int, 10)

		// Initialize optimization maps for O(1) lookups
		cs.downloadedMap = make(map[uint]struct{}, defaultDownloadedCap)
		cs.processedURLs = make(map[string]struct{}, defaultProcessedCap)
		cs.processedTitles = make(map[string]struct{}, defaultProcessedCap)

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
	if s == nil {
		// Pool exhausted, create new instance with optimized allocations
		s = &ConfigSearcher{
			Raw: apiexternal.NzbSlice{
				Arr: make([]apiexternal_v2.Nzbwithprio, 0, defaultRawCapacity),
			},
			Denied:           make([]apiexternal_v2.Nzbwithprio, 0, defaultDeniedCapacity),
			Accepted:         make([]apiexternal_v2.Nzbwithprio, 0, defaultAcceptedCapacity),
			indexerConfigMap: make(map[string]int, 10),
			downloadedMap:    make(map[uint]struct{}, defaultDownloadedCap),
			processedURLs:    make(map[string]struct{}, defaultProcessedCap),
			processedTitles:  make(map[string]struct{}, defaultProcessedCap),
		}
		copy(s.episodeBuffer[:], episodePrefixes[:])
	}

	s.Cfgp = cfgp
	s.searchActionType = searchActionType

	if quality != nil {
		s.Quality = quality
	} else if mediaid != nil {
		s.Quality = database.GetMediaQualityConfig(cfgp, mediaid)
	}

	return s
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

	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
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

	handler := mediatype.Get(cfgp.IsType)
	if handler == nil {
		return logger.ErrNotFound
	}

	err := handler.FillSearchVar(&p.e, mediaid)
	if err != nil {
		if !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			logger.Logtype("error", 0).
				Uint(logger.StrID, mediaid).
				Uint("Search Type", cfgp.IsType).
				Err(err).
				Msg("Media Search Failed")

			return err
		}

		return nil
	}

	s.setQualityConfig(&p.e.Quality)

	nzbID := handler.GetNzbIDP(&p.e)

	// Use audio priority for audio types, video priority for video types
	if cfgp.IsType == config.MediaTypeMusic ||
		cfgp.IsType == config.MediaTypeAudiobook ||
		cfgp.IsType == config.MediaTypeBook {
		p.e.MinimumPriority, _ = GetpriobyfilesAudio(
			cfgp.IsType,
			nzbID,
			false,
			-1,
			s.Quality,
			false,
		)
	} else {
		p.e.MinimumPriority, _ = Getpriobyfiles(
			cfgp.IsType,
			nzbID,
			false,
			-1,
			s.Quality,
			false,
		)
	}

	p.e.Info.ListID = s.Cfgp.GetMediaListsEntryListID(p.e.Listname)

	s.searchActionType, err = getsearchtype(p.e.MinimumPriority, p.e.DontUpgrade, false)
	if err != nil {
		if !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			logger.Logtype("error", 0).
				Uint(logger.StrID, mediaid).
				Uint("Search Type", cfgp.IsType).
				Err(err).
				Msg("Media Search Failed")

			return err
		}

		return nil
	}

	if s.Quality == nil {
		logger.Logtype("error", 0).
			Uint(logger.StrID, mediaid).
			Str("Quality", p.e.Quality).
			Uint("Search Type", cfgp.IsType).
			Err(errSearchQualityEmpty).
			Msg("Media Search Quality Failed")

		return errSearchQualityEmpty
	}

	s.searchlog("info", "Search for media id", p)
	// logger.Logtype("debug", 1).Uint("mediaid", mediaid).Bool("titlesearch", titlesearch).Msg("Pre Alternative Titles")

	if titlesearch || s.Quality.BackupSearchForTitle || s.Quality.BackupSearchForAlternateTitle {
		p.sourcealttitles = database.GetDbstaticTwoStringOneInt(
			database.Getentryalternatetitlesdirect(&p.e.Dbid, s.Cfgp.IsType),
			p.e.Dbid,
		)
	}

	// logger.Logtype("debug", 1).Uint("mediaid", mediaid).Msg("Pre searchindexers")
	s.searchindexers(ctx, false, p)
	// logger.Logtype("debug", 1).Uint("mediaid", mediaid).Msg("Post searchindexers")
	if atomic.LoadInt32(&s.Done) == 0 && len(s.Raw.Arr) == 0 {
		s.searchlog("error", "All searches failed", p)
		return nil
	}

	database.ExecN(mtstrings.GetStringsMap(cfgp.IsType, logger.UpdateMediaLastscan), &p.mediaid)

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
	case searchTypeSeasonDate:
		return s.handleSeasonDateSearch(indcfg, p)
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

	atomic.StoreInt32(&s.Done, 0)

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

		pl.Submit(func() {
			defer logger.HandlePanic()

			// Check rate limiter inside the pool to avoid blocking the main thread
			if !apiexternal.NewznabCheckLimiter(indcfg) {
				logger.Logtype("debug", 2).
					Str(logger.StrIndexer, indcfg.Name).
					Str("quality", s.Quality.Name).
					Str("search_type", s.searchActionType).
					Msg("Skipping indexer - rate limited or circuit breaker open")

				return
			}

			// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Msg("Starting executeSearch")
			err := s.executeSearch(p, indcfg)
			// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Err(err).Msg("Completed executeSearch")
			if err == nil {
				atomic.CompareAndSwapInt32(&s.Done, 0, 1)
			}
		})
	}

	// logger.Logtype("debug", 1).Int("submitted_tasks", len(s.Quality.IndexerCfg)).Msg("Pre pl.Wait() in searchindexers")
	errjobs := pl.Wait()
	// logger.Logtype("debug", 1).Msg("Post pl.Wait() in searchindexers")
	if errjobs == nil {
		return
	}

	// Check if ALL indexers failed or just SOME
	if atomic.LoadInt32(&s.Done) == 0 {
		// All indexers failed
		logger.Logtype("error", 2).
			Str("search_type", s.searchActionType).
			Str("quality", s.Quality.Name).
			Err(errjobs).
			Msg("Failed to search indexers - all indexers failed")
	} else {
		// Some indexers failed, but at least one succeeded
		// logger.Logtype("warning", 2).
		// 	Str("search_type", s.searchActionType).
		// 	Str("quality", s.Quality.Name).
		// 	Err(errjobs).
		// 	Msg("Some indexers failed during search")
	}
}

// searchnameid is a method of the ConfigSearcher struct that performs a search for a media item
// by its name or ID. It checks various conditions to determine the appropriate search method,
// such as whether to use a query search or a search by ID, and whether to search for a movie
// or a TV series. It also handles errors that may occur during the search and logs them.
// The method returns a boolean indicating whether the search was successful.
func (s *ConfigSearcher) searchnameid(p *searchParams, indcfg *config.IndexersConfig) error {
	logger.Logtype("debug", 3).
		Str(logger.StrIndexer, indcfg.Name).
		Str("quality", s.Quality.Name).
		Str("search_type", s.searchActionType).
		Msg("Starting media search")

	cats := s.Quality.QualityIndexerByQualityAndTemplate(indcfg)
	if cats == -1 {
		logger.Logtype("error", 3).
			Str(logger.StrIndexer, indcfg.Name).
			Str("quality", s.Quality.Name).
			Str("search_type", s.searchActionType).
			Msg("Quality configuration not found for indexer")

		return errors.New("quality configuration not found for indexer")
	}

	// Query-only media types (books, audiobooks, music) always use query search
	// as they don't have IMDB/TVDB IDs for ID-based search.
	// HasSearchID checks if the entry has a valid search ID for the media type.
	usequerysearch := p.titlesearch ||
		!mediatype.SupportsIDSearch(s.Cfgp.IsType) ||
		!mediatype.HasSearchID(s.Cfgp.IsType, &p.e)

	var err error

	// ID-based search (more efficient)
	if !usequerysearch {
		logger.Logtype("debug", 2).
			Str(logger.StrIndexer, indcfg.Name).
			Str("search_type", s.searchActionType).
			Msg("Using ID-based search strategy")

		err = s.performIDSearch(p, indcfg, cats)

		// Check if we should fallback to title search
		if s.Quality.SearchForTitleIfEmpty && len(s.Raw.Arr) == 0 {
			logger.Logtype("debug", 2).
				Str(logger.StrIndexer, indcfg.Name).
				Str("search_type", s.searchActionType).
				Msg("ID search returned no results, falling back to title search")

			usequerysearch = true
		}
	}

	// Title-based search
	if usequerysearch {
		logger.Logtype("debug", 2).
			Str(logger.StrIndexer, indcfg.Name).
			Str("search_type", s.searchActionType).
			Msg("Using title-based search strategy")

		errsub := s.performTitleSearch(p, indcfg, cats)
		if err == nil && errsub != nil {
			err = errsub
		}
	}

	// Log search completion
	if err == nil {
		logger.Logtype("debug", 4).
			Str(logger.StrIndexer, indcfg.Name).
			Str("quality", s.Quality.Name).
			Str("search_type", s.searchActionType).
			Int("results", len(s.Raw.Arr)).
			Msg("Media search completed successfully")
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
		// Use SearchFor if populated (includes artist/author for music/books/audiobooks),
		// otherwise fall back to WantedTitle
		searchQuery := p.e.WantedTitle
		if p.e.SearchFor != "" {
			searchQuery = p.e.SearchFor
		}

		// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Str("title", searchQuery).Msg("Pre executeQuerySearch Title")
		if err = s.executeQuerySearch(p, indcfg, cats, searchQuery, "Title"); err == nil {
			// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Msg("Post executeQuerySearch Title - success")
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) > 0 {
				return nil
			}
		}
	}

	// Absolute episode search (for shows with absolute numbering like anime or long-running series)
	// Only search by absolute episode if it's filled and different from regular episode
	if mediatype.SupportsAbsoluteEpisode(s.Cfgp.IsType) && p.e.Info.AbsoluteEpisode > 0 {
		// Build search string with absolute episode number (e.g., "Series Name E643")
		absoluteSearch := p.e.WantedTitle + " E" + strconv.Itoa(p.e.Info.AbsoluteEpisode)
		if err = s.executeQuerySearch(
			p,
			indcfg,
			cats,
			absoluteSearch,
			"Absolute Episode",
		); err == nil {
			if s.Quality.CheckUntilFirstFound && len(s.Accepted) > 0 {
				return nil
			}
		}
	}

	// Alternative title search
	if s.Quality.BackupSearchForAlternateTitle && len(p.sourcealttitles) > 0 {
		// Track word sets of already-searched titles to skip redundant searches
		// Optimized: Pre-allocate with exact capacity needed
		searchedWordSets := make([][]string, 1, len(p.sourcealttitles)+1)

		// Add the primary title's word set as first element
		searchedWordSets[0] = titleToWordSet(p.e.WantedTitle)

		// Cache wanted title for comparison
		wantedTitle := p.e.WantedTitle

		for i := range p.sourcealttitles {
			altTitle := &p.sourcealttitles[i]
			if altTitle.Str1 == "" || altTitle.Str1 == wantedTitle {
				continue
			}

			searchstr := altTitle.Str1
			logger.StringRemoveAllRunesP(&searchstr, '&', '(', ')')

			// Check if this title's words are a subset/superset of an already-searched title
			altWords := titleToWordSet(searchstr)
			if isWordSetRedundant(altWords, searchedWordSets) {
				logger.Logtype("debug", 4).
					Str(logger.StrIndexer, indcfg.Name).
					Str("skipped_title", searchstr).
					Str("primary_title", wantedTitle).
					Msg("Skipping redundant alternate title search")

				continue
			}

			searchedWordSets = append(searchedWordSets, altWords)

			if errsub := s.executeQuerySearch(
				p,
				indcfg,
				cats,
				searchstr,
				"Alternative Title",
			); errsub == nil {
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
//
// Optimized: Removed redundant 'downloaded' slice since downloadedMap already
// provides O(1) duplicate tracking. This eliminates unnecessary slice allocations
// and append operations.
func (s *ConfigSearcher) Download() {
	if len(s.Accepted) == 0 {
		return
	}

	for idx := range s.Accepted {
		if s.checkdownloaded(idx) {
			continue
		}

		entry := &s.Accepted[idx]

		qualcfg := s.getentryquality(&entry.Info)
		if qualcfg == nil {
			logger.Logtype("info", 5).
				Uint(logger.StrSeries, s.Cfgp.IsType).
				Str(logger.StrTitle, entry.NZB.Title).
				Str("search_type", s.searchActionType).
				Int(logger.StrMinPrio, entry.MinimumPriority).
				Int(logger.StrPriority, entry.Info.Priority).
				Msg("NZB found - starting download")
		} else {
			logger.Logtype("info", 6).
				Uint(logger.StrSeries, s.Cfgp.IsType).
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrQuality, qualcfg.Name).
				Str("search_type", s.searchActionType).
				Int(logger.StrMinPrio, entry.MinimumPriority).
				Int(logger.StrPriority, entry.Info.Priority).
				Msg("NZB found - starting download")
		}

		// Download based on media type using appropriate handler
		switch s.Cfgp.IsType {
		case config.MediaTypeMovie:
			if entry.NzbmovieID != 0 {
				s.downloadedMap[entry.NzbmovieID] = struct{}{} // O(1) duplicate tracking
				downloader.DownloadMovie(s.Cfgp, entry)
			}

		case config.MediaTypeSeries:
			if entry.NzbepisodeID != 0 {
				s.downloadedMap[entry.NzbepisodeID] = struct{}{} // O(1) duplicate tracking
				downloader.DownloadSeriesEpisode(s.Cfgp, entry)
			}

		case config.MediaTypeBook:
			if entry.NzbbookID != 0 {
				s.downloadedMap[entry.NzbbookID] = struct{}{}
				downloader.DownloadBook(s.Cfgp, entry)
			}

		case config.MediaTypeAudiobook:
			if entry.NzbaudiobookID != 0 {
				s.downloadedMap[entry.NzbaudiobookID] = struct{}{}
				downloader.DownloadAudiobook(s.Cfgp, entry)
			}

		case config.MediaTypeMusic:
			if entry.NzbalbumID != 0 {
				s.downloadedMap[entry.NzbalbumID] = struct{}{}
				downloader.DownloadAlbum(s.Cfgp, entry)
			}
		}
	}
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
//
// Optimized: Hoisted handler lookup outside the loop, removed redundant slice assignment,
// and cached frequently accessed values.
func (s *ConfigSearcher) searchparse(
	e *apiexternal_v2.Nzbwithprio,
	alttitles []syncops.DbstaticTwoStringOneInt,
) {
	rawLen := len(s.Raw.Arr)
	if rawLen == 0 {
		return
	}

	// Reset slices while preserving capacity
	s.Denied = s.Denied[:0]
	s.Accepted = s.Accepted[:0]

	// Hoist handler lookup outside the loop - single lookup instead of per-entry
	handler := mediatype.Get(s.Cfgp.IsType)

	// Cache search type comparison result
	// Artist/author searches should also use RSS processing path (getmediadatarss)
	isRSS := s.searchActionType == logger.StrRss || s.isArtistAuthorSearch || s.isSeasonSearch

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

		if !isRSS && s.checkcorrectid(e, entry) {
			continue
		}

		parser_v2.ParseFileP(
			entry.NZB.Title,
			false,
			false,
			s.Cfgp,
			-1,
			&entry.Info,
		)

		// For music, fall back to category-inferred format when the release title
		// has no explicit format indicator (e.g. "DENNISONN-Polyhedron EP-(XR593)-WEB-2026-PTC"
		// has category 3010=MP3 but no "MP3" or "FLAC" in the name).
		if s.Cfgp.IsType == config.MediaTypeMusic && entry.Info.AudioFormat == "" {
			entry.Info.AudioFormat = audioFormatFromCategory(entry.NZB.Category)
		}

		// For movies/series, fall back to category-inferred resolution when the release title
		// has no explicit resolution indicator.
		if (s.Cfgp.IsType == config.MediaTypeMovie || s.Cfgp.IsType == config.MediaTypeSeries) &&
			entry.Info.Resolution == "" && entry.NZB.Category != "" && entry.NZB.Indexer != nil {
			provider := apiexternal.Getnewznabclient(entry.NZB.Indexer)

			entry.Info.Resolution = resolutionFromCategory(
				entry.NZB.Category,
				provider.SupportedCategories,
			)
		}

		if handler != nil {
			handler.ClearUntrustedID(entry)
		}

		if err := parser.GetDBIDs(&entry.Info, s.Cfgp, true, false); err != nil {
			s.logdenied1Str(
				err.Error(),
				entry,
				strCheckedFor,
				entry.Info.Title,
			)

			continue
		}

		var qual *config.QualityConfig
		if isRSS {
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
		// Track in optimization maps for O(1) duplicate detection
		if entry.NZB.DownloadURL != "" {
			s.processedURLs[entry.NZB.DownloadURL] = struct{}{}
		}

		if entry.NZB.Title != "" {
			s.processedTitles[entry.NZB.Title] = struct{}{}
		}

		if qual.CheckUntilFirstFound {
			break
		}
	}

	if database.DBLogLevel == logger.StrDebug {
		logger.Logtype("debug", 1).
			Int("Count", rawLen).
			Msg("Entries found")
	}

	if len(s.Accepted) > 1 {
		slices.SortFunc(s.Accepted, func(a, b apiexternal_v2.Nzbwithprio) int {
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
	// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Str("searchterm", searchTerm).Msg("Pre QueryNewznabQuery")
	_, _, err := apiexternal.QueryNewznabQuery(
		s.Cfgp, &p.e, indcfg, s.Quality, searchTerm, cats, &s.Raw,
	)
	// logger.Logtype("debug", 1).Str("indexer", indcfg.Name).Err(err).Msg("Post QueryNewznabQuery")

	if err != nil && !errors.Is(err, logger.ErrToWait) && !errors.Is(err, newznab.ErrBroke) {
		p.e.Info.TempID = p.mediaid
		logsearcherror(
			logger.JoinStrings("Error Searching Media by ", searchType),
			p.e.Info.TempID,
			s.Cfgp.IsType,
			searchTerm,
			err,
		)

		return err
	}

	return nil
}

// getmediadata validates the media data in the given entry against the
// source entry to determine if it is a match. It sets various priority
// and search control fields on the entry based on the source entry
// configuration. Returns true to skip/reject the entry if no match, false
// to continue processing if a match.
func (s *ConfigSearcher) getmediadata(sourceentry, entry *apiexternal_v2.Nzbwithprio) bool {
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}

	handler := mediatype.Get(s.Cfgp.IsType)
	if handler == nil {
		return true
	}

	if !handler.CheckMediaMatch(sourceentry, entry) {
		reason := handler.GetUnwantedReason()
		logger.Logtype("debug", 0).
			Str(logger.StrReason, reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Uint(logger.StrFound, handler.GetFoundID(entry)).
			Uint(logger.StrWanted, handler.GetNzbID(sourceentry)).
			Str(logger.StrConfig, s.Cfgp.NamePrefix).
			Msg(skippedstr)

		entry.Reason = reason
		s.logdenied("", entry)

		return true
	}

	handler.SetNzbID(entry, handler.GetNzbID(sourceentry))

	entry.Dbid = sourceentry.Dbid
	entry.MinimumPriority = sourceentry.MinimumPriority
	entry.DontSearch = sourceentry.DontSearch
	entry.DontUpgrade = sourceentry.DontUpgrade
	entry.WantedTitle = sourceentry.WantedTitle

	return false
}

// getmediadatarss processes an Nzbwithprio entry for adding to the RSS feed.
// It handles movie and series entries using the registered media type handler.
// For movies, it tries to add the entry to the list with ID addinlistid, or adds it if addifnotfound is true.
// For series, it validates the series/episode identifiers.
// It returns true if the entry should be skipped.
func (s *ConfigSearcher) getmediadatarss(
	entry *apiexternal_v2.Nzbwithprio,
	addinlistid int,
	addifnotfound bool,
) (bool, *config.QualityConfig) {
	handler := mediatype.Get(s.Cfgp.IsType)
	if handler == nil {
		return true, nil
	}

	// Handle list ID setting (for movies: sets ListID from addinlistid)
	handler.HandleRSSListID(entry, addinlistid)

	// Movie-specific: check for addifnotfound with IMDB and handle movie import
	if s.Cfgp.IsType == config.MediaTypeMovie {
		if entry.Info.DbmovieID == 0 &&
			(!addifnotfound || !logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt)) {
			s.logdenied("unwanted DBMovie", entry)
			return true, nil
		}

		// Movie-specific: handle movie import if needed
		if addifnotfound && (entry.Info.DbmovieID == 0 || entry.Info.MovieID == 0) &&
			logger.HasPrefixI(entry.NZB.IMDBID, logger.StrTt) {
			if skip := s.handleMovieImport(entry, addinlistid); skip {
				return true, nil
			}
		}
	}

	if reason := handler.ValidateRSSIDs(entry); reason != "" {
		s.logdenied(reason, entry)
		return true, nil
	}

	handler.SetRSSIDs(entry)

	mediaID := handler.GetRSSMediaID(entry)
	getrssdata(&mediaID, s.Cfgp.IsType, entry)

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
	// Query-only media types (books, audiobooks, music) don't support ID-based search
	// as they don't have IMDB/TVDB IDs. Return nil to fall through to title search.
	if !mediatype.SupportsIDSearch(s.Cfgp.IsType) {
		return nil
	}

	var err error

	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		err = h.PerformIDSearch(indcfg, s.Quality, &p.e, cats, &s.Raw)
	}

	if err != nil && !errors.Is(err, logger.ErrToWait) {
		p.e.Info.TempID = p.mediaid
		logsearcherror("Error Searching Media by ID", p.e.Info.TempID, s.Cfgp.IsType, "", err)
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

// handleMovieImport attempts to import a movie into the system when necessary.
// It checks if movie import is allowed, performs database lookups, and handles
// movie entry creation for a given NZB entry. Returns true if import should be
// skipped, false if successful.
//
// Parameters:
//   - entry: The NZB entry containing movie information
//   - addinlistid: The list ID to add the movie to
//
// Returns a boolean indicating whether movie import processing should be halted.
func (s *ConfigSearcher) handleMovieImport(
	entry *apiexternal_v2.Nzbwithprio,
	addinlistid int,
) bool {
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
// If isType is true, the function will use a different set of query
// parameters to retrieve the data.
func getrssdata(id *uint, isType uint, entry *apiexternal_v2.Nzbwithprio) {
	database.GetdatarowArgs(
		mtstrings.GetStringsMap(isType, "GetRSSData"),
		id,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.WantedTitle,
	)
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
// of the TemplateIndexer field. Uses an O(1) cache to avoid repeated O(n) searches.
// Returns the index of the matching indexer or -1 if no match is found.
func (s *ConfigSearcher) findIndexerConfig(templateList string) int {
	// Check cache first - O(1)
	if idx, exists := s.indexerConfigMap[templateList]; exists {
		return idx
	}

	// Cache miss - do O(n) search and populate cache
	for idx := range s.Quality.Indexer {
		if s.Quality.Indexer[idx].TemplateIndexer == templateList ||
			strings.EqualFold(s.Quality.Indexer[idx].TemplateIndexer, templateList) {
			s.indexerConfigMap[templateList] = idx
			return idx
		}
	}

	// Cache negative result to avoid repeated failed lookups
	s.indexerConfigMap[templateList] = -1

	return -1
}

// setQualityConfig sets the quality configuration for the ConfigSearcher.
// If the current quality is nil or different from the provided quality name,
// it updates the quality based on the given name or falls back to the default quality.
// If the quality name is empty or not found in the settings, it uses the default quality.
func (s *ConfigSearcher) setQualityConfig(qualityName *string) {
	if s.Quality != nil && s.Quality.Name == *qualityName {
		return
	}

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
		Uint(logger.StrSeries, s.Cfgp.IsType).
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
func logsearcherror(msg string, id uint, isType uint, title string, err error) {
	// Skip logging for expected errors in debug mode
	if database.DBLogLevel == logger.StrDebug && isExpectedError(err) {
		return
	}

	// Skip logging rate limit errors
	if err != nil && strings.Contains(err.Error(), "] rate limit") {
		return
	}

	logger.Logtype("error", 1).
		Uint(strMediaid, id).
		Uint(logger.StrSeries, isType).
		Str(logger.StrTitle, title).
		Err(err).
		Msg(msg)
}

// deniedappend appends the given Nzbwithprio entry to the ConfigSearcher's Denied slice
// and adds it to the processed maps for O(1) duplicate detection.
func (s *ConfigSearcher) deniedappend(entry *apiexternal_v2.Nzbwithprio) {
	s.Denied = append(s.Denied, *entry)
	// Track in optimization maps for O(1) lookups
	if entry.NZB.DownloadURL != "" {
		s.processedURLs[entry.NZB.DownloadURL] = struct{}{}
	}

	if entry.NZB.Title != "" {
		s.processedTitles[entry.NZB.Title] = struct{}{}
	}
}

// logdenied logs a denied entry with the given reason and optional additional fields.
// It sets the reason and additional reason on the entry, and appends the entry to s.Denied.
func (s *ConfigSearcher) logdenied(reason string, entry *apiexternal_v2.Nzbwithprio) {
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
func (s *ConfigSearcher) logdenied1Int64(reason string, entry *apiexternal_v2.Nzbwithprio) {
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
	entry *apiexternal_v2.Nzbwithprio,
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
	entry *apiexternal_v2.Nzbwithprio,
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
	entry *apiexternal_v2.Nzbwithprio,
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

// SearchArtistMissing searches for missing albums by artist name.
// It finds up to 20 random artists with missing albums and searches for each.
func SearchArtistMissing(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, true, true)
}

// SearchArtistUpgrade searches for albums needing quality upgrade by artist name.
func SearchArtistUpgrade(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, false, true)
}

// SearchAuthorAudiobookMissing searches for missing audiobooks by author name.
func SearchAuthorAudiobookMissing(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, true, false)
}

// SearchAuthorAudiobookUpgrade searches for audiobooks needing quality upgrade by author name.
func SearchAuthorAudiobookUpgrade(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, false, false)
}

// SearchAuthorBookMissing searches for missing books by author name.
func SearchAuthorBookMissing(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, true, false)
}

// SearchAuthorBookUpgrade searches for books needing quality upgrade by author name.
func SearchAuthorBookUpgrade(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	return searchArtistsOrAuthors(ctx, cfgp, false, false)
}

// searchArtistsOrAuthors is the core function that searches by artist/author name.
// forMissing: true for missing items, false for upgrades
// isMusic: true for music (artists), false for audiobooks/books (authors).
func searchArtistsOrAuthors(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	forMissing bool,
	isMusic bool,
) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	// Build list arguments
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	var listnames []string
	for _, lst := range cfgp.ListsMap {
		name := lst.Name // copy to avoid pointer issues

		args.Arr = append(args.Arr, &name)
		listnames = append(listnames, lst.Name)
	}

	// Debug: log listnames being searched
	logger.Logtype("debug", 2).
		Strs("listnames", listnames).
		Msg("Artist/author search listnames")

	// Get the handler for this media type
	handler := mediatype.Get(cfgp.IsType)
	if handler == nil {
		return logger.ErrNotFound
	}

	// Build query key based on missing/upgrade
	var queryKey, queryEndKey string
	if isMusic {
		if forMissing {
			queryKey = "SearchArtistsMissing"
			queryEndKey = "SearchArtistsMissingEnd"
		} else {
			queryKey = "SearchArtistsUpgrade"
			queryEndKey = "SearchArtistsUpgradeEnd"
		}
	} else {
		if forMissing {
			queryKey = "SearchAuthorsMissing"
			queryEndKey = "SearchAuthorsMissingEnd"
		} else {
			queryKey = "SearchAuthorsUpgrade"
			queryEndKey = "SearchAuthorsUpgradeEnd"
		}
	}

	// Build the query with list placeholders
	query := logger.JoinStrings(
		mtstrings.GetStringsMap(cfgp.IsType, queryKey),
		cfgp.ListsQu,
		mtstrings.GetStringsMap(cfgp.IsType, queryEndKey),
	)

	// Debug: log query and args
	logger.Logtype("debug", 2).
		Str("query", query).
		Int("num_args", len(args.Arr)).
		Bool("is_music", isMusic).
		Bool("for_missing", forMissing).
		Msg("Artist/author search query")

	// Get artists/authors with missing/upgrade items
	rows := database.GetrowsN[database.DbstaticOneStringOneUInt](
		false,
		20,
		query,
		args.Arr...,
	)

	if len(rows) == 0 {
		logger.Logtype("debug", 2).
			Bool("missing", forMissing).
			Bool("is_music", isMusic).
			Msg("No artists/authors found for search")

		return nil
	}

	searchType := "missing"
	if !forMissing {
		searchType = "upgrade"
	}

	entityType := "author"
	if isMusic {
		entityType = "artist"
	}

	logger.Logtype("info", 2).
		Int("count", len(rows)).
		Str("type", searchType).
		Str("entity", entityType).
		Msg("Starting artist/author search")

	var err error
	for i := range rows {
		if errsub := searchSingleArtistOrAuthor(ctx, cfgp, &rows[i], forMissing); errsub != nil {
			err = errsub
		}
	}

	return err
}

// vaAlternateName returns the alternate form of a VA/Various Artists name,
// or empty string if the name is not a VA variant.
func vaAlternateName(name string) string {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, "va") || strings.EqualFold(name, "v.a.") ||
		strings.EqualFold(name, "v.a") {
		return "Various Artists"
	}

	if strings.EqualFold(name, "various artists") {
		return "VA"
	}

	return ""
}

// searchSingleArtistOrAuthor searches for a single artist/author's items.
func searchSingleArtistOrAuthor(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	row *database.DbstaticOneStringOneUInt,
	forMissing bool,
) error {
	if row.Str == "" {
		return nil
	}

	// Get first available quality config
	var quality *config.QualityConfig
	for _, lst := range cfgp.Lists {
		if lst.CfgQuality != nil {
			quality = lst.CfgQuality
			break
		}
	}

	if quality == nil {
		return nil
	}

	searchType := "missing"
	if !forMissing {
		searchType = "upgrade"
	}

	logger.Logtype("info", 3).
		Str("name", row.Str).
		Uint("id", row.Num).
		Str("type", searchType).
		Msg("Searching for artist/author")

	// Create a new searcher
	s := NewSearcher(cfgp, quality, logger.StrRss, nil)
	if s == nil {
		return errSearchvarEmpty
	}

	defer s.Close()

	// Mark this as an artist/author search to ensure getmediadatarss is used
	s.isArtistAuthorSearch = true

	// Set up search params
	p := plsearchparam.Get()
	defer plsearchparam.Put(p)

	p.searchtype = searchTypeMissing
	p.titlesearch = true

	// Set the search query to the artist/author name
	// Note: Don't set WantedTitle here - it will be populated from the database
	// by getrssdata for each matched album/book/audiobook during result processing
	p.e.SearchFor = row.Str

	// Search all indexers with the artist/author name
	s.searchindexers(ctx, false, p)

	// Also search with alternate VA/Various Artists form
	altName := vaAlternateName(row.Str)
	if altName != "" {
		p.e.SearchFor = altName
		s.searchindexers(ctx, false, p)
	}

	// For audiobooks, also search by series name if the author has any
	if cfgp.IsType == config.MediaTypeAudiobook {
		seriesNames := database.Getrowssize[string](
			false,
			"SELECT count(DISTINCT series_name) FROM dbaudiobooks JOIN dbaudiobook_authors ON dbaudiobooks.id = dbaudiobook_authors.dbaudiobook_id WHERE dbaudiobook_authors.dbauthor_id = ? AND dbaudiobooks.series_name != ''",
			"SELECT DISTINCT series_name FROM dbaudiobooks JOIN dbaudiobook_authors ON dbaudiobooks.id = dbaudiobook_authors.dbaudiobook_id WHERE dbaudiobook_authors.dbauthor_id = ? AND dbaudiobooks.series_name != ''",
			&row.Num,
		)
		for _, sn := range seriesNames {
			p.e.SearchFor = sn
			s.searchindexers(ctx, false, p)
			logger.Logtype("info", 3).
				Str("author", row.Str).
				Str("series", sn).
				Msg("Searching by series name")
		}
	}

	// For music, also search by series name if the artist has any
	if cfgp.IsType == config.MediaTypeMusic {
		seriesNames := database.Getrowssize[string](
			false,
			"SELECT count(DISTINCT series_name) FROM dbalbums JOIN dbalbum_artists ON dbalbums.id = dbalbum_artists.dbalbum_id WHERE dbalbum_artists.dbartist_id = ? AND dbalbums.series_name != ''",
			"SELECT DISTINCT series_name FROM dbalbums JOIN dbalbum_artists ON dbalbums.id = dbalbum_artists.dbalbum_id WHERE dbalbum_artists.dbartist_id = ? AND dbalbums.series_name != ''",
			&row.Num,
		)
		for _, sn := range seriesNames {
			p.e.SearchFor = sn
			s.searchindexers(ctx, false, p)
			logger.Logtype("info", 3).
				Str("artist", row.Str).
				Str("series", sn).
				Msg("Searching by series name")
		}
	}

	// Process results
	if atomic.LoadInt32(&s.Done) != 0 || len(s.Raw.Arr) >= 1 {
		s.processSearchResults(true, "", nil, quality, nil)

		logger.Logtype("info", 2).
			Str("name", row.Str).
			Int(logger.StrAccepted, len(s.Accepted)).
			Int(logger.StrDenied, len(s.Denied)).
			Msg("Ended artist/author search")
	}

	return nil
}

// Getpriobyfiles returns the minimum priority of existing files for the given media
// ID, and optionally returns a slice of file paths for existing files below
// the given wanted priority. If isType is true it will look up series IDs instead of media IDs.
// If id is nil it will return 0 priority.
// If useall is true it will include files marked as deleted.
// calcPrioFromFiles is the shared loop for Getpriobyfiles and GetpriobyfilesAudio.
// n is the number of entries; prio(i) and loc(i) return the priority and file path
// for entry i.
func calcPrioFromFiles(
	n int,
	wantedprio int,
	getold bool,
	prio func(i int) int,
	loc func(i int) string,
) (int, []string) {
	collectOldFiles := getold && wantedprio != -1

	var (
		minPrio int
		oldf    []string
	)

	if collectOldFiles {
		oldf = make([]string, 0, n)
	}

	for i := range n {
		p := prio(i)

		if p > minPrio {
			minPrio = p
		}

		if collectOldFiles && wantedprio > p {
			oldf = append(oldf, loc(i))
		}
	}

	if !collectOldFiles {
		return minPrio, nil
	}

	return minPrio, oldf
}

// If wantedprio is -1 it will not return any file paths.
//
// Optimized: Cached condition checks outside the loop to avoid repeated evaluation.
func Getpriobyfiles(
	isType uint,
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
		mtstrings.GetStringsMap(isType, logger.DBCountFilesByMediaID),
		mtstrings.GetStringsMap(isType, logger.DBFilePrioFilesByID),
		id,
	)

	if len(arr) == 0 {
		return 0, nil
	}

	return calcPrioFromFiles(len(arr), wantedprio, getold,
		func(i int) int { return calculateFilePriority(&arr[i], qualcfg, useall) },
		func(i int) string { return arr[i].Location },
	)
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

	var prio int
	// Try wanted priorities first
	intid := parser.Findpriorityidxwanted(r, q, c, a, 0, qualcfg)
	if intid == -1 {
		intid = parser.Findpriorityidx(r, q, c, a, 0, qualcfg)
		if intid == -1 {
			logger.Logtype("debug", 2).
				Str("in", qualcfg.Name).
				Str("searched for", parser.BuildPrioStr(file.ResolutionID, file.QualityID, file.CodecID, file.AudioID, 0)).
				Msg("prio not found")

			return 0
		}

		prio = parser.GetAllArrPrio(intid)
	} else {
		prio = parser.GetwantedArrPrio(intid)
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

// GetpriobyfilesAudio returns the highest priority and old file locations for audio media (music/audiobooks).
// It calculates priority based on audio attributes: format, bitrate, sample rate, and bit depth.
// If useall is true it will include all files. If wantedprio is -1 it will not return any file paths.
func GetpriobyfilesAudio(
	isType uint,
	id *uint,
	useall bool,
	wantedprio int,
	qualcfg *config.QualityConfig,
	getold bool,
) (int, []string) {
	if qualcfg == nil || id == nil || *id == 0 {
		return 0, nil
	}

	arr := database.Getrowssize[database.AudioFilePrio](
		false,
		mtstrings.GetStringsMap(isType, logger.DBCountFilesByMediaID),
		mtstrings.GetStringsMap(isType, logger.DBAudioFilePrioFilesByID),
		id,
	)

	if len(arr) == 0 {
		return 0, nil
	}

	return calcPrioFromFiles(len(arr), wantedprio, getold,
		func(i int) int { return calculateAudioFilePriority(&arr[i], qualcfg, useall) },
		func(i int) string { return arr[i].Location },
	)
}

// calculateAudioFilePriority determines the priority of an audio file based on format, bitrate,
// sample rate, and bit depth using the qualities table (type 5) for format priority.
func calculateAudioFilePriority(
	file *database.AudioFilePrio,
	qualcfg *config.QualityConfig,
	useall bool,
) int {
	if file == nil || qualcfg == nil {
		return 0
	}

	// Look up audio format ID from qualities table
	var audioformat uint
	if qualcfg.UseForPriorityAudioFormat || useall {
		audioformat = database.GetAudioFormatID(file.Format)
	}

	// Use the same priority lookup as movies (resolution/quality/codec/audio all 0 for audio media)
	intid := parser.Findpriorityidxwanted(0, 0, 0, 0, audioformat, qualcfg)

	var prio int
	if intid == -1 {
		intid = parser.Findpriorityidx(0, 0, 0, 0, audioformat, qualcfg)
		if intid != -1 {
			prio = parser.GetAllArrPrio(intid)
		}
	} else {
		prio = parser.GetwantedArrPrio(intid)
	}

	// Bitrate bonus modifier
	if qualcfg.UseForPriorityAudioBitrate || useall {
		prio += calculateAudioBitratePriorityFromFile(file.Bitrate, file.Format)
	}

	// Bit depth bonus for hi-res audio (24-bit, 32-bit)
	if file.BitDepth >= 24 {
		prio += (file.BitDepth - 16)
	}

	// Sample rate bonus for hi-res audio (above 44.1kHz)
	if file.SampleRate > 44100 {
		if file.SampleRate >= 96000 {
			prio += 10
		} else if file.SampleRate >= 48000 {
			prio += 5
		}
	}

	return prio
}

// calculateAudioBitratePriorityFromFile returns priority bonus based on audio bitrate.
func calculateAudioBitratePriorityFromFile(bitrate int, format string) int {
	format = strings.ToLower(format)

	// For lossless formats, bitrate varies with content, so give a small bonus
	switch format {
	case "flac", "alac", "wav", "aiff", "ape", "wv", "wavpack", "dsd", "dsf":
		return 5 // Small bonus for having bitrate info
	}

	// For lossy formats, higher bitrate = better quality
	switch {
	case bitrate >= 320:
		return 30 // 320kbps (highest common MP3)
	case bitrate >= 256:
		return 25 // 256kbps
	case bitrate >= 192:
		return 20 // 192kbps
	case bitrate >= 160:
		return 15 // 160kbps
	case bitrate >= 128:
		return 10 // 128kbps
	case bitrate > 0:
		return 5 // Low bitrate
	}

	return 0
}

// validateBasicEntry performs basic validation on an entry.
func (s *ConfigSearcher) validateBasicEntry(entry *apiexternal_v2.Nzbwithprio) bool {
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
	case entry.AlbumID != 0:
		return database.GetMediaQualityConfig(s.Cfgp, &entry.AlbumID)
	case entry.AudiobookID != 0:
		return database.GetMediaQualityConfig(s.Cfgp, &entry.AudiobookID)
	case entry.BookID != 0:
		return database.GetMediaQualityConfig(s.Cfgp, &entry.BookID)
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
	entry *apiexternal_v2.Nzbwithprio,
	cfgqual *config.QualityConfig,
) bool {
	if entry.MinimumPriority != 0 {
		return false
	}

	// Set temp ID for priority lookup
	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		h.SetEntryTempID(entry)
	}

	// Use audio priority function for audio media types
	if s.Cfgp.IsType == config.MediaTypeMusic ||
		s.Cfgp.IsType == config.MediaTypeAudiobook ||
		s.Cfgp.IsType == config.MediaTypeBook {
		entry.MinimumPriority, _ = GetpriobyfilesAudio(
			s.Cfgp.IsType,
			&entry.Info.TempID,
			false,
			-1,
			cfgqual,
			false,
		)
	} else {
		entry.MinimumPriority, _ = Getpriobyfiles(
			s.Cfgp.IsType,
			&entry.Info.TempID,
			false,
			-1,
			cfgqual,
			false,
		)
	}

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
