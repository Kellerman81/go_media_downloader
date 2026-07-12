package searcher

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
)

var errListNotFound = errors.New("list not found")

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

	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	s.Quality = quality

	p := plsearchparam.Get()
	defer plsearchparam.Put(p)

	p.searchtype = searchTypeRSS
	s.searchindexers(ctx, true, p)

	if atomic.LoadInt32(&s.Done) != 0 || len(s.Raw.Arr) >= 1 {
		s.processSearchResults(downloadresults, "", nil, s.Quality, nil)
	}

	return nil
}

// handleRSSSearch performs an RSS search for a specific indexer configuration.
// It queries the last RSS entry, updates the RSS history if a new entry is found,
// and handles potential errors during the search process.
// Returns true if the RSS search is successful, false otherwise.
func (s *ConfigSearcher) handleRSSSearch(indcfg *config.IndexersConfig, _ *searchParams) error {
	logger.Logtype("debug", 2).
		Str(logger.StrIndexer, indcfg.Name).
		Str("quality", s.Quality.Name).
		Msg("Starting RSS search")

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
			logger.Logtype("info", 3).
				Str(logger.StrIndexer, indcfg.Name).
				Str("quality", s.Quality.Name).
				Str("last_id", firstid).
				Msg("RSS search completed with new entries")
			addrsshistory(&indcfg.URL, &firstid, s.Quality, &s.Cfgp.NamePrefix)
		} // else {

		// logger.Logtype("debug", 2).
		// 	Str(logger.StrIndexer, indcfg.Name).
		// 	Str("quality", s.Quality.Name).
		// 	Msg("RSS search completed - no new entries")
		// }
		return nil
	}

	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) &&
		!isRateLimitError(err) {
		logger.Logtype("error", 2).
			Str(logger.StrIndexer, indcfg.Name).
			Str("quality", s.Quality.Name).
			Err(err).
			Msg("RSS search failed for indexer")

		return err
	}

	return nil
}

// handleSeasonSearch performs a TV season search for a specific indexer configuration.
// It queries the TV series using TVDB ID and season information, and handles potential
// errors during the search process. Returns true if the season search is successful,
// false otherwise.
func (s *ConfigSearcher) handleSeasonSearch(indcfg *config.IndexersConfig, p *searchParams) error {
	// Season search only applies to media types with season/episode structure
	if !mediatype.SupportsSeasonSearch(s.Cfgp.IsType) {
		return nil
	}

	logger.Logtype("debug", 4).
		Str(logger.StrIndexer, indcfg.Name).
		Str("quality", s.Quality.Name).
		Int(logger.StrTvdb, p.thetvdbid).
		Str(logger.StrSeason, p.season).
		Msg("Starting season search")

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
		logger.Logtype("info", 4).
			Str(logger.StrIndexer, indcfg.Name).
			Str("quality", s.Quality.Name).
			Int(logger.StrTvdb, p.thetvdbid).
			Str(logger.StrSeason, p.season).
			Msg("Season search completed successfully")

		return nil
	}

	if !errors.Is(err, logger.Errnoresults) && !errors.Is(err, logger.ErrToWait) &&
		!isRateLimitError(err) {
		logger.Logtype("error", 5).
			Str(logger.StrIndexer, indcfg.Name).
			Str("quality", s.Quality.Name).
			Int(logger.StrTvdb, p.thetvdbid).
			Str(logger.StrSeason, p.season).
			Str("error", err.Error()).
			Msg("Season search failed for indexer")

		return err
	}

	return nil
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

	if s.Quality == nil {
		return errSearchQualityEmpty
	}

	intid := s.findIndexerConfig(listentry.TemplateList)
	if intid == -1 || s.Quality.Indexer[intid].TemplateRegex == "" {
		return errRegexEmpty
	}

	if s.isIndexerBlocked(listentry.CfgList.URL) {
		logger.Logtype("debug", 2).
			Str(logger.StrListname, listentry.TemplateList).
			Int(strMinutes, -1*config.GetSettingsGeneral().FailedIndexerBlockTime).
			Msg("Indexer temporarily disabled due to fail in last")

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

	// Season search only applies to media types with season/episode structure
	if !mediatype.SupportsSeasonSearch(cfgp.IsType) {
		return nil
	}

	s.Quality = quality

	p := plsearchparam.Get()
	defer plsearchparam.Put(p)

	p.searchtype = searchTypeSeason
	p.thetvdbid = thetvdbid
	p.season = season
	p.useseason = useseason
	s.isSeasonSearch = true

	logger.Logtype("info", 2).
		Str(logger.StrSeason, p.season).
		Int(logger.StrTvdb, p.thetvdbid).
		Msg("Search for season") // logpointerr
	s.searchindexers(ctx, false, p)

	if atomic.LoadInt32(&s.Done) != 0 || len(s.Raw.Arr) >= 1 {
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
func SearchSeriesRSSSeasons(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	return searchseasons(
		ctx,
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
func SearchSeriesRSSSeasonsAll(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	return searchseasons(
		ctx,
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

// handleSeasonDateSearch performs a name-only RSS search for date-identified series.
// Instead of querying by TVDB ID + season, it searches by series name so that all
// recent releases are returned and matched against episodes we don't have yet.
func (s *ConfigSearcher) handleSeasonDateSearch(
	indcfg *config.IndexersConfig,
	p *searchParams,
) error {
	if p.e.SearchFor == "" {
		return nil
	}

	cats := s.Quality.QualityIndexerByQualityAndTemplate(indcfg)
	if cats == -1 {
		return nil
	}

	logger.Logtype("debug", 4).
		Str(logger.StrIndexer, indcfg.Name).
		Str("quality", s.Quality.Name).
		Str("series", p.e.SearchFor).
		Msg("Starting date-series name search")

	// Pass an empty entry so getaddstr never appends a stale identifier to the query.
	emptyEntry := searchParams{}

	_, _, err := apiexternal.QueryNewznabQuery(
		s.Cfgp,
		&emptyEntry.e,
		indcfg,
		s.Quality,
		p.e.SearchFor,
		cats,
		&s.Raw,
	)
	if err == nil || errors.Is(err, logger.Errnoresults) {
		return nil
	}

	return err
}

// searchSeriesRSSByName searches a date-identified series by name only (no season filter).
// This finds all recent releases from the indexer and lets the normal match logic
// determine which episodes are still missing.
func (s *ConfigSearcher) searchSeriesRSSByName(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	seriesname string,
	downloadentries bool,
) error {
	if s == nil {
		return errSearchvarEmpty
	}

	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}

	s.Quality = quality

	p := plsearchparam.Get()
	defer plsearchparam.Put(p)

	p.searchtype = searchTypeSeasonDate
	p.e.SearchFor = seriesname
	s.isSeasonSearch = true

	logger.Logtype("info", 2).
		Str("series", seriesname).
		Msg("Search for date-series by name")

	s.searchindexers(ctx, false, p)

	if atomic.LoadInt32(&s.Done) != 0 || len(s.Raw.Arr) >= 1 {
		s.processSearchResults(downloadentries, "", nil, s.Quality, nil)

		logger.Logtype("info", 0).
			Str("series", seriesname).
			Int(logger.StrAccepted, len(s.Accepted)).
			Int(logger.StrDenied, len(s.Denied)).
			Msg("Ended date-series name search")
	}

	return nil
}

// searchseason searches for missing episodes for a specific series and season.
// It retrieves the distinct seasons with missing episodes for the given series,
// and then searches the RSS feeds of the enabled indexers for those seasons.
// For date-identified series it performs a single name-only search instead.
func searchseason(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	row *database.DbstaticTwoUint,
	queryseason, queryseasoncount string,
) error {
	// Get list ID once
	listid := database.GetMediaListIDGetListname(cfgp, &row.Num1)
	if listid == -1 {
		return errListNotFound
	}

	// Date-identified series: search by name only, no season iteration
	if database.Getdatarow[string](
		false,
		"select lower(identifiedby) from dbseries where id = ?",
		&row.Num2,
	) == logger.StrDate {
		seriename := database.Getdatarow[string](
			false,
			"select seriename from dbseries where id = ?",
			&row.Num2,
		)
		if seriename == "" {
			return nil
		}

		searcher := NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, nil)
		defer searcher.Close()

		return searcher.searchSeriesRSSByName(
			ctx, cfgp, cfgp.Lists[listid].CfgQuality, seriename, true,
		)
	}

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
	for i := range seasons {
		if errctx := logger.CheckContextEnded(ctx); errctx != nil {
			return errctx
		}

		if errsub := NewSearcher(
			cfgp,
			cfgp.Lists[listid].CfgQuality,
			logger.StrRss,
			nil,
		).searchSeriesRSSSeason(
			ctx,
			cfgp,
			cfgp.Lists[listid].CfgQuality,
			tvdbid,
			seasons[i],
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

	var (
		dbserieid uint
		tvdb      int
	)

	database.GetdatarowArgs(
		"select s.dbserie_id, d.thetvdb_id from series s inner join dbseries d on d.id = s.dbserie_id where s.id = ?",
		serieid,
		&dbserieid,
		&tvdb,
	)

	listid := database.GetMediaListIDGetListname(cfgp, serieid)
	if listid == -1 {
		return nil, logger.ErrListnameEmpty
	}

	results := NewSearcher(cfgp, cfgp.Lists[listid].CfgQuality, logger.StrRss, nil)
	if results == nil {
		return nil, errSearchvarEmpty
	}

	// Date-identified series have no TVDB ID — search by name instead
	if tvdb == 0 {
		seriename := database.Getdatarow[string](
			false,
			"select seriename from dbseries where id = ?",
			&dbserieid,
		)
		if seriename == "" {
			return nil, logger.ErrTvdbEmpty
		}

		err := results.searchSeriesRSSByName(
			context.Background(),
			cfgp,
			cfgp.Lists[listid].CfgQuality,
			seriename,
			true,
		)
		if err != nil && !errors.Is(err, logger.ErrDisabled) && !errors.Is(err, logger.ErrToWait) {
			results.Close()
			return nil, err
		}

		return results, nil
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
// for the given rss feed url, quality profile name, and config name.
// Uses a single atomic upsert so concurrent indexer searches cannot insert
// duplicate rows (the previous select-then-insert was racy).
func addrsshistory(urlv, lastid *string, quality *config.QualityConfig, configv *string) {
	database.ExecN(
		"insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?) on conflict (config collate nocase, list collate nocase, indexer collate nocase) do update set last_id = excluded.last_id",
		configv,
		&quality.Name,
		urlv,
		lastid,
	)
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
		if errctx := logger.CheckContextEnded(ctx); errctx != nil {
			return errctx
		}

		if errsub := searchseason(
			ctx,
			cfgp,
			&tbl[idx],
			queryseason,
			queryseasoncount,
		); errsub != nil {
			err = errsub
		}
	}

	return err
}
