package utils

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/csrfapi"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/htmlxpath"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/antchfx/htmlquery"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/net/html"
)

// feedResults is a struct that contains the results from processing feeds
// Series contains the configuration for a TV series
// Movies contains a slice of movie IDs as strings.
// Books/Audiobooks/Albums use ManualConfig to support three import modes:
//   - Full author/artist (AuthorName/ArtistName set)
//   - Book/Album series (BookSeriesName/AlbumSeriesName set)
//   - Single item (only Name set)
type feedResults struct {
	Series     []config.ManualConfig // Configuration for a TV series
	Movies     []string              // Slice of movie IDs as strings
	Books      []config.ManualConfig // Configuration for books (author, book series, or single book)
	Audiobooks []config.ManualConfig // Configuration for audiobooks (author, book series, or single audiobook)
	Albums     []config.ManualConfig // Configuration for music (artist, album series, or single album)
	AddAll     bool

	// ircKey/ircLines are the IRC session and announce lines this feed
	// drained, until the caller commits or requeues them.
	ircKey   string
	ircLines []string
}

// Sentinel errors for feed processing.
var (
	errwrongtype             = errors.New("wrong type")
	errcsvread               = errors.New("csv read")
	errusernameempty         = errors.New("username empty")
	errlistnameempty         = errors.New("list type empty")
	errPlexConfigMissing     = errors.New("plex server URL, token, and username are required")
	errJellyfinConfigMissing = errors.New("jellyfin server URL, API key, and username are required")
	errTraktNotInitialized   = errors.New("trakt provider not initialized")
	errScraperTypeMissing    = errors.New(
		"movie_scraper_type is required for moviescraper list type",
	)
	errScraperURLMissing = errors.New(
		"movie_scraper_start_url is required for moviescraper list type",
	)
	errMusicchartsURLRequired       = errors.New("url is required for musiccharts list type")
	errMusicchartsEntryNodeRequired = errors.New(
		"chart_entry_node_xpath is required for musiccharts list type",
	)
	errMusicchartsTitleRequired = errors.New(
		"chart_title_xpath is required for musiccharts list type",
	)
	errBookbestsellersURLRequired   = errors.New("url is required for bookbestsellers list type")
	errBookbestsellersEntryRequired = errors.New(
		"chart_entry_node_xpath is required for bookbestsellers list type",
	)
	errBookbestsellersTitleRequired = errors.New(
		"chart_title_xpath is required for bookbestsellers list type",
	)
	plfeeds pool.Poolobj[feedResults]
)

// String constants to avoid repeated allocations.
const strtoparse = "entries to parse"

// Init initializes the plfeeds pool object, which is used to manage the feedResults
// struct. It sets the initial capacity for the Movies and Series slices, and
// provides a cleanup function to clear the slices when they are reused.
func Init() {
	plfeeds.Init(200, 5, func(b *feedResults) {
		b.Movies = make([]string, 0, 10000)
		b.Series = make([]config.ManualConfig, 0, 1000)
		b.Books = make([]config.ManualConfig, 0, 1000)
		b.Audiobooks = make([]config.ManualConfig, 0, 1000)
		b.Albums = make([]config.ManualConfig, 0, 1000)
	}, func(b *feedResults) bool {
		if len(b.Movies) > 0 {
			clear(b.Movies)

			b.Movies = b.Movies[:0]
		}

		if len(b.Series) > 0 {
			clear(b.Series)

			b.Series = b.Series[:0]
		}

		if len(b.Books) > 0 {
			clear(b.Books)

			b.Books = b.Books[:0]
		}

		if len(b.Audiobooks) > 0 {
			clear(b.Audiobooks)

			b.Audiobooks = b.Audiobooks[:0]
		}

		if len(b.Albums) > 0 {
			clear(b.Albums)

			b.Albums = b.Albums[:0]
		}

		b.ircKey, b.ircLines = "", nil

		return false
	})
}

// waitRateLimiter blocks until the limiter permits the next request, sleeping for
// the limiter's suggested wait time. A nil limiter is a no-op.
func waitRateLimiter(limiter *slidingwindow.Limiter) {
	if limiter == nil {
		return
	}

	if allowed, waitTime := limiter.Check(); !allowed && waitTime > 0 {
		time.Sleep(waitTime)
	}
}

// gettraktmovielist retrieves a list of movies from Trakt.tv based on the specified configuration.
// The list type is determined by the `usetraktmovie` parameter, which can be one of the following:
// 1 - Popular movies
// 2 - Trending movies
// 3 - Anticipated movies
// 4 - User's custom list
//
// The retrieved movie information is added to the `d.Movies` slice.
// An error is returned if there is a problem retrieving the movie list.
func (d *feedResults) gettraktmovielist(usetraktmovie int, cfglist *config.MediaListsConfig) error {
	waitRateLimiter(providers.GetTrakt().GetRateLimiter())

	provider := providers.GetTrakt()

	var seq iter.Seq[apiexternal_v2.MovieSearchResult]

	switch usetraktmovie {
	case 1:
		seq = traktSeq("popular movies", cfglist.CfgList.Limit, traktMovies(provider.GetPopularMovies, cfglist.CfgList.URL))
	case 2:
		seq = traktSeq("trending movies", cfglist.CfgList.Limit, traktMovies(provider.GetTrendingMovies, cfglist.CfgList.URL))
	case 3:
		seq = traktSeq("anticipated movies", cfglist.CfgList.Limit, traktMovies(provider.GetUpcomingMovies, cfglist.CfgList.URL))
	case 4:
		return d.getTraktUserMovieList(cfglist)
	default:
		return errwrongtype
	}

	for arr := range seq {
		checkaddimdbfeed(&arr.IMDbID, cfglist, d)
	}

	return nil
}

// getTraktUserMovieList handles user-specific Trakt movie list retrieval.
func (d *feedResults) getTraktUserMovieList(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
		return errusernameempty
	}

	if cfglist.CfgList.TraktListType == "" {
		return errlistnameempty
	}

	waitRateLimiter(providers.GetTrakt().GetRateLimiter())

	arr, err := traktUserList(cfglist)
	if err != nil {
		return err
	}

	removeFromList := cfglist.CfgList.RemoveFromList && cfglist.CfgList.TraktListName != "watchlist"
	for idx := range arr {
		if arr[idx].Movie == nil {
			continue
		}

		imdb := arr[idx].Movie.IDs.IMDB
		checkaddimdbfeed(&imdb, cfglist, d)

		// Remove both newly added and already tracked entries, like gettmdblist.
		if removeFromList && imdb != "" {
			if err := providers.GetTrakt().RemoveMovieFromTraktUserList(
				context.Background(),
				cfglist.CfgList.TraktUsername,
				cfglist.CfgList.TraktListName,
				imdb,
			); err != nil {
				logger.Logtype("warn", 1).
					Str("imdb", imdb).
					Str("list", cfglist.CfgList.TraktListName).
					Err(err).
					Msg("Failed to remove movie from Trakt user list")
			}
		}
	}

	return nil
}

// traktUserList returns the items of the list's Trakt user list or
// watchlist, capped at a configured limit > 0.
func traktUserList(cfglist *config.MediaListsConfig) ([]trakt.TraktUserListItem, error) {
	provider := providers.GetTrakt()
	if provider == nil {
		return nil, errTraktNotInitialized
	}

	items, err := provider.GetTraktUserList(
		context.Background(),
		cfglist.CfgList.TraktUsername,
		cfglist.CfgList.TraktListName,
		cfglist.CfgList.TraktListType,
	)
	if err != nil {
		return nil, err
	}

	if n, convErr := strconv.Atoi(strings.TrimSpace(cfglist.CfgList.Limit)); convErr == nil && n > 0 && n < len(items) {
		items = items[:n]
	}

	return items, nil
}

// gettmdbmoviepopular retrieves a list of popular movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmoviepopular(cfglist *config.MediaListsConfig) error {
	provider := providers.GetTMDB()
	waitRateLimiter(provider.GetRateLimiter())

	arr, err := tmdbMovieList(cfglist.CfgList.Limit, provider.GetPopularMovies)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbmovietrending retrieves a list of trending movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmovietrending(cfglist *config.MediaListsConfig) error {
	provider := providers.GetTMDB()
	waitRateLimiter(provider.GetRateLimiter())

	arr, err := tmdbMovieList(cfglist.CfgList.Limit, provider.GetTrendingMovies)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbmovieupcoming retrieves a list of upcoming movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmovieupcoming(cfglist *config.MediaListsConfig) error {
	provider := providers.GetTMDB()
	waitRateLimiter(provider.GetRateLimiter())

	arr, err := tmdbMovieList(cfglist.CfgList.Limit, provider.GetUpcomingMovies)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbseriepopular retrieves a list of popular TV series from The Movie Database (TMDb).
// It gets the TVDB ID for each series and adds it to the d.Series slice.
func (d *feedResults) gettmdbseriepopular(cfglist *config.MediaListsConfig) error {
	provider := providers.GetTMDB()
	waitRateLimiter(provider.GetRateLimiter())

	arr, err := tmdbSerieList(cfglist.CfgList.Limit, provider.GetPopularSeries)
	if err != nil {
		return err
	}

	return d.processTmdbSeriesResults(arr, cfglist)
}

// gettmdbserietrending retrieves a list of trending TV series from The Movie Database (TMDb).
// It gets the TVDB ID for each series and adds it to the d.Series slice.
func (d *feedResults) gettmdbserietrending(cfglist *config.MediaListsConfig) error {
	provider := providers.GetTMDB()
	waitRateLimiter(provider.GetRateLimiter())

	arr, err := tmdbSerieList(cfglist.CfgList.Limit, provider.GetTrendingSeries)
	if err != nil {
		return err
	}

	return d.processTmdbSeriesResults(arr, cfglist)
}

// processTmdbMovieResults processes TMDB movie search results and adds allowed movies to d.Movies.
func (d *feedResults) processTmdbMovieResults(
	arr []apiexternal_v2.MovieSearchResult,
	cfglist *config.MediaListsConfig,
) error {
	var allowed bool
	for idx := range arr {
		imdbID, err := tmdbMovieIMDbID(arr[idx].ID)
		if err != nil {
			logger.Logtype("debug", 1).
				Int(logger.StrImdb, arr[idx].ID).
				Err(err).
				Msg("imdb id could not be retrieved")

			continue
		}

		allowed, _ = importfeed.AllowMovieImport(&imdbID, cfglist.CfgList)
		if allowed || d.AddAll {
			d.Movies = append(d.Movies, imdbID)
		}
	}

	return nil
}

// tmdbMovieIMDbID returns the IMDB ID of a TMDB movie, logger.ErrNoID when
// TMDB has none.
func tmdbMovieIMDbID(tmdbID int) (string, error) {
	provider := providers.GetTMDB()
	if provider == nil {
		return "", logger.ErrNotFound
	}

	ids, err := provider.GetMovieExternalIDs(context.Background(), tmdbID)
	if err != nil {
		return "", err
	}

	if ids == nil || ids.IMDbID == "" {
		return "", logger.ErrNoID
	}

	return ids.IMDbID, nil
}

// tmdbSerieTVDbID returns the TVDB ID of a TMDB series, 0 when unknown.
func tmdbSerieTVDbID(tmdbID int) int {
	provider := providers.GetTMDB()
	if provider == nil {
		return 0
	}

	ids, err := provider.GetSeriesExternalIDs(context.Background(), tmdbID)
	if err != nil || ids == nil {
		return 0
	}

	return ids.TVDbID
}

// processTmdbSeriesResults processes TMDB series search results and adds series to d.Series.
func (d *feedResults) processTmdbSeriesResults(
	arr []apiexternal_v2.SeriesSearchResult,
	_ *config.MediaListsConfig,
) error {
	var imdbid string
	for idx := range arr {
		database.Scanrowsdyn(
			false,
			"select id from dbseries where moviedb_id = ?",
			&imdbid,
			&arr[idx].ID,
		)

		if imdbid != "" && !d.AddAll {
			continue
		}

		tvdbID := tmdbSerieTVDbID(arr[idx].ID)
		if tvdbID == 0 {
			continue
		}

		d.Series = append(d.Series, config.ManualConfig{
			Name: arr[idx].Name, TvdbID: tvdbID,
		})
	}

	return nil
}

// gettmdbmoviediscover retrieves a list of movies from The Movie Database (TMDb) based on the specified configuration.
// It checks the database for existing movies and adds new movies to the `d.Movies` slice if they are allowed to be imported.
// The function returns an error if there is a problem retrieving the movie list from TMDb.
func (d *feedResults) gettmdbmoviediscover(cfglist *config.MediaListsConfig) error {
	if len(cfglist.CfgList.TmdbDiscover) == 0 {
		return nil
	}

	listnamefilter := cfglist.Getlistnamefilterignore()

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	// Pre-allocate args slice capacity
	if cap(args.Arr) < len(cfglist.IgnoreMapLists) {
		args.Arr = make([]any, 0, len(cfglist.IgnoreMapLists))
	}

	for idx := range cfglist.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglist.IgnoreMapLists[idx])
	}

	// Cache config lookups to avoid repeated map access
	useMediaCache := config.GetSettingsGeneral().UseMediaCache
	hasIgnoreMapLists := cfglist.IgnoreMapListsLen >= 1

	var existing []uint
	if !useMediaCache && listnamefilter != "" {
		countQuery := "select count() from movies where " + listnamefilter
		selectQuery := "select dbmovie_id from movies where " + listnamefilter

		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](false, countQuery, args.Arr...),
			selectQuery,
			args.Arr,
		)
	}

	var (
		movieid uint
		allowed bool
	)

	tmdbp := providers.GetTMDB()
	tmdbLimiter := tmdbp.GetRateLimiter()

	pages := cfglist.CfgList.GetTmdbPages()

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		query := cfglist.CfgList.TmdbDiscover[idxdiscover]

		for page := 1; page <= pages; page++ {
			waitRateLimiter(tmdbLimiter)

			arr, err := tmdbp.DiscoverMovies(context.Background(), query, page)
			if err != nil {
				logger.Logtype("debug", 1).
					Str("query", query).
					Int("page", page).
					Err(err).
					Msg("discover could not be executed")

				break
			}

			if len(arr.Results) == 0 {
				break
			}

			for idx := range arr.Results {
				waitRateLimiter(tmdbLimiter)

				movieid = importfeed.MovieFindDBIDByTmdbID(&arr.Results[idx].ID)
				if movieid != 0 {
					if getmovieid(&movieid, cfglist) && !d.AddAll {
						continue
					}

					if hasIgnoreMapLists {
						if useMediaCache {
							if database.CacheOneStringTwoIntIndexFunc(
								logger.CacheMovie,
								func(elem *syncops.DbstaticOneStringTwoInt) bool {
									return elem.Num1 == movieid &&
										logger.SlicesContainsI(cfglist.IgnoreMapLists, elem.Str)
								},
							) {
								continue
							}
						} else if listnamefilter != "" && slices.Contains(existing, movieid) {
							continue
						}
					}
				}

				imdbID, err := tmdbMovieIMDbID(arr.Results[idx].ID)
				if err != nil {
					logger.Logtype("debug", 1).
						Int(logger.StrImdb, arr.Results[idx].ID).
						Err(err).
						Msg("imdb id could not be retrieved")

					continue
				}

				allowed, _ = importfeed.AllowMovieImport(&imdbID, cfglist.CfgList)
				if allowed || d.AddAll {
					d.Movies = append(d.Movies, imdbID)
				}
			}

			// TMDB reports how many pages the query actually has; stop there so a
			// generous tmdb_pages costs no extra requests.
			if arr.TotalPages > 0 && page >= arr.TotalPages {
				break
			}
		}
	}

	return nil
}

// gettmdbshowdiscover retrieves a list of TV shows from The Movie Database (TMDb) based on the configuration settings in the provided MediaListsConfig.
// It iterates through the TmdbDiscover configuration, fetching the TV show details from TMDb and adding them to the d.Series slice if they are not already in the database.
func (d *feedResults) gettmdbshowdiscover(cfglist *config.MediaListsConfig) error {
	if len(cfglist.CfgList.TmdbDiscover) == 0 {
		return nil
	}

	var imdbid string

	tmdbp := providers.GetTMDB()
	tmdbLimiter := tmdbp.GetRateLimiter()

	pages := cfglist.CfgList.GetTmdbPages()

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		for page := 1; page <= pages; page++ {
			waitRateLimiter(tmdbLimiter)

			arr, err := tmdbp.DiscoverTV(
				context.Background(),
				cfglist.CfgList.TmdbDiscover[idxdiscover],
				page,
			)
			if err != nil {
				break
			}

			if len(arr.Results) == 0 {
				break
			}

			for idx := range arr.Results {
				database.Scanrowsdyn(
					false,
					"select id from dbseries where moviedb_id = ?",
					&imdbid,
					&arr.Results[idx].ID,
				)

				if imdbid == "" || d.AddAll {
					waitRateLimiter(tmdbLimiter)

					tvdbID := tmdbSerieTVDbID(arr.Results[idx].ID)
					if tvdbID == 0 {
						continue
					}

					d.Series = append(d.Series, config.ManualConfig{
						Name: arr.Results[idx].Name, TvdbID: tvdbID,
					})
				}
			}

			// TMDB reports how many pages the query actually has; stop there so a
			// generous tmdb_pages costs no extra requests.
			if arr.TotalPages > 0 && page >= arr.TotalPages {
				break
			}
		}
	}

	return nil
}

// gettmdbshowlist retrieves a list of TV shows from The Movie Database (TMDb) based on the configuration settings in the provided MediaListsConfig.
// It iterates through the TmdbList configuration, fetching the TV show details from TMDb and adding them to the d.Series slice if they are not already in the database.
func (d *feedResults) gettmdbshowlist(cfglist *config.MediaListsConfig) error {
	// Guards the slice this function actually iterates. It checked TmdbDiscover,
	// so a series list with tmdb_list set but no tmdb_discover imported nothing.
	if len(cfglist.CfgList.TmdbList) == 0 {
		return nil
	}

	var imdbid string

	tmdbp := providers.GetTMDB()
	tmdbLimiter := tmdbp.GetRateLimiter()
	pages := cfglist.CfgList.GetTmdbPages()

	for idxdiscover := range cfglist.CfgList.TmdbList {
		for page := 1; page <= pages; page++ {
			waitRateLimiter(tmdbLimiter)

			arr, err := tmdbp.GetTMDBList(context.Background(), cfglist.CfgList.TmdbList[idxdiscover], page)
			if err != nil {
				break
			}

			if len(arr.Items) == 0 {
				break
			}

			for idx := range arr.Items {
				database.Scanrowsdyn(
					false,
					"select id from dbseries where moviedb_id = ?",
					&imdbid,
					&arr.Items[idx].ID,
				)

				if imdbid != "" && !d.AddAll {
					continue
				}

				waitRateLimiter(tmdbLimiter)

				tvdbID := tmdbSerieTVDbID(arr.Items[idx].ID)
				if tvdbID == 0 {
					continue
				}

				// TV entries carry name, movie entries title.
				name := arr.Items[idx].Title
				if name == "" {
					name = arr.Items[idx].Name
				}

				d.Series = append(d.Series, config.ManualConfig{Name: name, TvdbID: tvdbID})
			}

			// TMDB reports how many pages the query actually has; stop there so a
			// generous tmdb_pages costs no extra requests.
			if arr.TotalPages > 0 && page >= arr.TotalPages {
				break
			}
		}
	}

	return nil
}

// checkaddimdbfeed checks if the given IMDB ID is not already in the movies list, and if so, adds it to the d.Movies slice.
// The function takes an IMDB ID pointer, a MediaListsConfig pointer, and a feedResults pointer as arguments.
// It first checks if the IMDB ID is not nil and not an empty string. If it is, the function returns without doing anything.
// It then checks if the movie with the given IMDB ID is not already in the movies list with the given list name. If the movie is not found, it adds the IMDB ID to the d.Movies slice.
func checkaddimdbfeed(imdb *string, cfglist *config.MediaListsConfig, d *feedResults) bool {
	if imdb == nil || *imdb == "" {
		return false
	}

	movieid := importfeed.MovieFindDBIDByImdb(imdb)

	if movieid == 0 || !getmovieid(&movieid, cfglist) || d.AddAll {
		d.Movies = append(d.Movies, *imdb)
		return true
	}

	return false
}

// checkaddtvdbfeed checks if a series with the given TVDB ID is not already tracked
// in the given list, and if so, adds it to the d.Series slice. Mirrors checkaddimdbfeed's
// dedup gating for the movie path so entries from a Trakt/Plex/Jellyfin series list are
// not reprocessed on every scheduled run regardless of whether they are already imported.
func checkaddtvdbfeed(tvdbid int, name string, cfglist *config.MediaListsConfig, d *feedResults) bool {
	if tvdbid == 0 {
		d.Series = append(d.Series, config.ManualConfig{Name: name, TvdbID: tvdbid})
		return true
	}

	dbserieid := database.Getdatarow[uint](false, database.QueryDbseriesGetIDByTvdb, &tvdbid)

	if dbserieid == 0 ||
		database.Getdatarow[uint](
			false,
			database.QuerySeriesGetIDByDBIDListname,
			&dbserieid,
			&cfglist.Name,
		) == 0 ||
		d.AddAll {
		d.Series = append(d.Series, config.ManualConfig{Name: name, TvdbID: tvdbid})
		return true
	}

	return false
}

// gettmdblist retrieves a list of movies from The Movie Database (TMDb) based on the configuration settings in the provided MediaListsConfig.
// It iterates through the TmdbList configuration, fetching the movie details from TMDb and adding them to the d.Movies slice if they are not already in the database.
// The function checks if the movie is already in the database, and if it is in the ignore list, it skips adding the movie to the list.
// If the movie is not in the database and is allowed to be imported, it adds the IMDB ID to the d.Movies slice.
// The function also removes the movie from the TMDb list if the RemoveFromList option is enabled in the configuration.
func (d *feedResults) gettmdblist(cfglist *config.MediaListsConfig) error {
	listnamefilter := cfglist.Getlistnamefilterignore()

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	// Pre-allocate args slice capacity
	if cap(args.Arr) < len(cfglist.IgnoreMapLists) {
		args.Arr = make([]any, 0, len(cfglist.IgnoreMapLists))
	}

	for idx := range cfglist.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglist.IgnoreMapLists[idx])
	}

	// Cache config lookups to avoid repeated map access
	useMediaCache := config.GetSettingsGeneral().UseMediaCache
	hasIgnoreMapLists := cfglist.IgnoreMapListsLen >= 1
	removeFromList := cfglist.CfgList.RemoveFromList

	var existing []uint
	if !useMediaCache && listnamefilter != "" {
		countQuery := "select count() from movies where " + listnamefilter
		selectQuery := "select dbmovie_id from movies where " + listnamefilter

		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](false, countQuery, args.Arr...),
			selectQuery,
			args.Arr,
		)
	}

	var (
		movieid uint
		allowed bool
	)

	tmdbp := providers.GetTMDB()
	tmdbLimiter := tmdbp.GetRateLimiter()
	pages := cfglist.CfgList.GetTmdbPages()

	for idxlist := range cfglist.CfgList.TmdbList {
		listID := cfglist.CfgList.TmdbList[idxlist]

		// Collect every requested page before processing any item. This function
		// can remove items from the source list, and removing while paging
		// forward shifts the remaining pages and silently skips entries.
		var items []int

		for page := 1; page <= pages; page++ {
			waitRateLimiter(tmdbLimiter)

			arr, err := tmdbp.GetTMDBList(context.Background(), listID, page)
			if err != nil {
				break
			}

			if len(arr.Items) == 0 {
				break
			}

			for idx := range arr.Items {
				items = append(items, arr.Items[idx].ID)
			}

			// TMDB reports how many pages the list actually has; stop there so a
			// generous tmdb_pages costs no extra requests.
			if arr.TotalPages > 0 && page >= arr.TotalPages {
				break
			}
		}

		for _, itemID := range items {
			waitRateLimiter(tmdbLimiter)

			movieid = importfeed.MovieFindDBIDByTmdbID(&itemID)

			if movieid != 0 {
				if getmovieid(&movieid, cfglist) && !d.AddAll {
					if removeFromList {
						tmdbp.RemoveFromTMDBList(context.Background(), listID, itemID)
					}

					continue
				}

				if hasIgnoreMapLists {
					if useMediaCache {
						if database.CacheOneStringTwoIntIndexFunc(
							logger.CacheMovie,
							func(elem *syncops.DbstaticOneStringTwoInt) bool {
								return elem.Num1 == movieid &&
									logger.SlicesContainsI(cfglist.IgnoreMapLists, elem.Str)
							},
						) {
							if removeFromList {
								tmdbp.RemoveFromTMDBList(context.Background(), listID, itemID)
							}

							continue
						}
					} else if listnamefilter != "" && slices.Contains(existing, movieid) {
						if removeFromList {
							tmdbp.RemoveFromTMDBList(context.Background(), listID, itemID)
						}

						continue
					}
				}
			}

			// Without an IMDB id nothing is imported, so the item must also
			// stay on the TMDB list.
			imdbID, err := tmdbMovieIMDbID(itemID)
			if err != nil {
				logger.Logtype("debug", 1).
					Int(logger.StrImdb, itemID).
					Err(err).
					Msg("imdb id could not be retrieved")

				continue
			}

			allowed, _ = importfeed.AllowMovieImport(&imdbID, cfglist.CfgList)
			if !allowed && !d.AddAll {
				continue
			}

			d.Movies = append(d.Movies, imdbID)

			if removeFromList {
				tmdbp.RemoveFromTMDBList(context.Background(), listID, itemID)
			}
		}
	}

	return nil
}

// gettraktserielist retrieves a list of TV series from Trakt.tv based on the specified configuration.
// The list type is determined by the `usetraktserie` parameter, which can be one of the following:
// 1 - Popular series
// 2 - Trending series
// 3 - Anticipated series
// 4 - User's custom list
//
// The retrieved series information is added to the `d.Series.Serie` slice.
// An error is returned if there is a problem retrieving the series list.
func (d *feedResults) gettraktserielist(usetraktserie int, cfglist *config.MediaListsConfig) error {
	waitRateLimiter(providers.GetTrakt().GetRateLimiter())

	provider := providers.GetTrakt()

	var seq iter.Seq[apiexternal_v2.SeriesSearchResult]

	switch usetraktserie {
	case 1:
		seq = traktSeq("popular series", cfglist.CfgList.Limit, traktSeries(provider.GetPopularSeries, cfglist.CfgList.URL))
	case 2:
		seq = traktSeq("trending series", cfglist.CfgList.Limit, traktSeries(provider.GetTrendingSeries, cfglist.CfgList.URL))
	case 3:
		seq = traktSeq(
			"anticipated series",
			cfglist.CfgList.Limit,
			func(ctx context.Context, page, size int) ([]apiexternal_v2.SeriesSearchResult, int, error) {
				items, err := provider.GetTraktSerieAnticipated(ctx, page, size, cfglist.CfgList.URL)
				if err != nil {
					return nil, 0, err
				}

				out := make([]apiexternal_v2.SeriesSearchResult, 0, len(items))
				for i := range items {
					if show := items[i].Show; show != nil {
						out = append(out, apiexternal_v2.SeriesSearchResult{
							ID: show.IDs.Trakt, Name: show.Title, TVDbID: show.IDs.TVDB, IMDbID: show.IDs.IMDB,
						})
					}
				}

				return out, len(items), nil
			},
		)
	case 4:
		return d.getTraktUserSeriesList(cfglist)
	default:
		return errwrongtype
	}

	for arr := range seq {
		checkaddtvdbfeed(arr.TVDbID, arr.Name, cfglist, d)
	}

	return nil
}

// getTraktUserSeriesList handles user-specific Trakt series list retrieval.
func (d *feedResults) getTraktUserSeriesList(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
		return errusernameempty
	}

	if cfglist.CfgList.TraktListType == "" {
		return errlistnameempty
	}

	waitRateLimiter(providers.GetTrakt().GetRateLimiter())

	arr, err := traktUserList(cfglist)
	if err != nil {
		return err
	}

	// TraktListName != "watchlist" guard matches getTraktUserMovieList above -
	// protects a user's Trakt watchlist from being auto-emptied by this feed.
	removeFromList := cfglist.CfgList.RemoveFromList && cfglist.CfgList.TraktListName != "watchlist"
	for idx := range arr {
		show := arr[idx].Show
		if show == nil {
			continue
		}

		checkaddtvdbfeed(show.IDs.TVDB, show.Title, cfglist, d)

		// Remove both newly added and already tracked entries, like gettmdblist.
		if removeFromList && show.IDs.TVDB != 0 {
			if err := providers.GetTrakt().RemoveSerieFromTraktUserList(
				context.Background(),
				cfglist.CfgList.TraktUsername,
				cfglist.CfgList.TraktListName,
				show.IDs.TVDB,
			); err != nil {
				logger.Logtype("warn", 1).
					Int("tvdb", show.IDs.TVDB).
					Str("list", cfglist.CfgList.TraktListName).
					Err(err).
					Msg("Failed to remove series from Trakt user list")
			}
		}
	}

	return nil
}

// loadManualConfig is a unified function for loading TOML config files.
// It handles series, audiobook, book, and music configurations.
// Returns the parsed ManualConfig slice or an error if loading fails.
func loadManualConfig(
	cfglist *config.ListsConfig,
	configType string,
) ([]config.ManualConfig, error) {
	content, err := os.Open(cfglist.ManualConfigFile)
	if err != nil {
		return nil, fmt.Errorf("loading %s config: %w", configType, err)
	}
	defer content.Close()

	var s config.MainManualConfig
	if err = toml.NewDecoder(content).Decode(&s); err != nil {
		return nil, fmt.Errorf("decoding %s config: %w", configType, err)
	}

	return s.Config, nil
}

// getseriesconfig loads the series config from the given file path.
func getseriesconfig(cfglist *config.ListsConfig) ([]config.ManualConfig, error) {
	return loadManualConfig(cfglist, "series")
}

// getaudiobookconfig loads the audiobook config from the given file path.
func getaudiobookconfig(cfglist *config.ListsConfig) ([]config.ManualConfig, error) {
	return loadManualConfig(cfglist, "audiobook")
}

// getbookconfig loads the book config from the given file path.
func getbookconfig(cfglist *config.ListsConfig) ([]config.ManualConfig, error) {
	return loadManualConfig(cfglist, "book")
}

// getmusicconfig loads the music/album config from the given file path.
func getmusicconfig(cfglist *config.ListsConfig) ([]config.ManualConfig, error) {
	return loadManualConfig(cfglist, "music")
}

// getimdbcsv loads an IMDB CSV list from a URL, parses it, filters movies,
// and returns a feedResults struct containing the allowed movie IDs.
//
// It uses a csv.Reader to parse the CSV data from the URL.
// It iterates through each row, getting the movie ID, checking if it already
// exists for this list, applying any configured filters, and adding allowed
// movies to the results list.
func (d *feedResults) getimdbcsv(cfglistp *config.MediaListsConfig) error {
	ctx := context.Background()

	return csvListClient().MakeRequestWithGracePeriod(
		ctx,
		http.MethodGet,
		cfglistp.CfgList.URL,
		nil,
		nil,
		func(r *http.Response) error {
			d.parseimdbcsv(ctx, r.Body, cfglistp)
			return nil
		},
		30*time.Second,
	)
}

// csvListClient is the HTTP client for CSV list downloads (10 calls per
// second, 30s timeout).
var csvListClient = sync.OnceValue(func() *base.BaseClient {
	return base.NewBaseClient(base.ClientConfig{
		Name:             "imdbcsv",
		Timeout:          30 * time.Second,
		AuthType:         base.AuthNone,
		RateLimitCalls:   10,
		RateLimitSeconds: 1,
		UserAgent:        config.GetSettingsGeneral().UserAgent,
	})
})

// parseimdbcsv parses an IMDB CSV file from the provided io.ReadCloser, filters the movies based on the given configuration,
// and appends the allowed movie IDs to the d.Movies slice.
//
// It uses a csv.Reader to parse the CSV data from the io.ReadCloser.
// It iterates through each row, getting the movie ID, checking if it already
// exists for this list, applying any configured filters, and adding allowed
// movies to the d.Movies list.
func (d *feedResults) parseimdbcsv(
	_ context.Context,
	resp io.ReadCloser,
	cfglistp *config.MediaListsConfig,
) {
	parserimdb := csv.NewReader(resp)

	parserimdb.ReuseRecord = true

	listnamefilter := cfglistp.Getlistnamefilterignore()

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	// Pre-allocate args slice capacity
	if cap(args.Arr) < len(cfglistp.IgnoreMapLists) {
		args.Arr = make([]any, 0, len(cfglistp.IgnoreMapLists))
	}

	for idx := range cfglistp.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglistp.IgnoreMapLists[idx])
	}

	// Cache the config lookup to avoid repeated map access in the loop
	useMediaCache := config.GetSettingsGeneral().UseMediaCache
	hasIgnoreMapLists := cfglistp.IgnoreMapListsLen >= 1

	var existing []uint
	if !useMediaCache && listnamefilter != "" {
		countQuery := "select count() from movies where " + listnamefilter
		selectQuery := "select dbmovie_id from movies where " + listnamefilter

		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](false, countQuery, args.Arr...),
			selectQuery,
			args.Arr,
		)
	}

	var (
		allowed bool
		movieid uint
	)

	for {
		record, err := parserimdb.Read()
		if err != nil {
			break
		}

		if len(record) < 2 || record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}

		record[1] = logger.AddImdbPrefix(record[1])
		movieid = importfeed.MovieFindDBIDByImdb(&record[1])

		if movieid != 0 {
			if getmovieid(&movieid, cfglistp) && !d.AddAll {
				continue
			}

			if hasIgnoreMapLists {
				if useMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(
						logger.CacheMovie,
						func(elem *syncops.DbstaticOneStringTwoInt) bool {
							return elem.Num1 == movieid &&
								logger.SlicesContainsI(cfglistp.IgnoreMapLists, elem.Str)
						},
					) {
						continue
					}
				} else if listnamefilter != "" && slices.Contains(existing, movieid) {
					continue
				}
			}
		} else {
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, record[1]).
				Msg("dbmovie not found in cache")
		}

		allowed, _ = importfeed.AllowMovieImport(&record[1], cfglistp.CfgList)
		if allowed || d.AddAll {
			d.Movies = append(d.Movies, record[1])
		}
	}

	movieCount := len(d.Movies)
	logger.Logtype("info", 2).
		Str(logger.StrURL, cfglistp.CfgList.URL).
		Int(strtoparse, movieCount).
		Msg("imdb list fetched")
}

// getimdbfile fetches the IMDB CSV file specified in the config and parses it using the parseimdbcsv method.
// It returns an error if the file cannot be opened.
func (d *feedResults) getimdbfile(cfglistp *config.MediaListsConfig) error {
	resp, err := os.Open(cfglistp.CfgList.IMDBCSVFile)
	if err != nil {
		return errcsvread
	}
	defer resp.Close()

	d.parseimdbcsv(context.Background(), resp, cfglistp)

	return nil
}

// Feeds fetches the configured media list for the given config and list ID.
// It handles looking up the correct list type and calling the appropriate
// handler function. Returns a feedResults struct containing the parsed list
// items on success, or an error if the list could not be fetched or parsed.
func Feeds(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	addall bool,
) (*feedResults, error) {
	d := plfeeds.Get()

	d.AddAll = addall
	switch list.CfgList.ListType {
	case "seriesconfig":
		var err error

		d.Series, err = getseriesconfig(list.CfgList)
		return d, err

	case "moviescraper":
		return d, d.getmoviescraper(ctx, cfgp, list)

	case "traktpublicshowlist":
		return d, d.gettraktserielist(4, list)
	case "imdbcsv":
		return d, d.getimdbcsv(list)
	case "imdbfile":
		return d, d.getimdbfile(list)
	case "tmdblist":
		return d, d.gettmdblist(list)
	case "tmdbshowlist":
		return d, d.gettmdbshowlist(list)
	case "traktpublicmovielist":
		return d, d.gettraktmovielist(4, list)
	case "traktmoviepopular":
		return d, d.gettraktmovielist(1, list)
	case "traktmovieanticipated":
		return d, d.gettraktmovielist(3, list)
	case "traktmovietrending":
		return d, d.gettraktmovielist(2, list)
	case "traktseriepopular":
		return d, d.gettraktserielist(1, list)
	case "traktserieanticipated":
		return d, d.gettraktserielist(3, list)
	case "traktserietrending":
		return d, d.gettraktserielist(2, list)
	case "tmdbmoviediscover":
		return d, d.gettmdbmoviediscover(list)
	case "tmdbshowdiscover":
		return d, d.gettmdbshowdiscover(list)
	case "tmdbmoviepopular":
		return d, d.gettmdbmoviepopular(list)
	case "tmdbseriepopular":
		return d, d.gettmdbseriepopular(list)
	case "tmdbmovietrending":
		return d, d.gettmdbmovietrending(list)
	case "tmdbserietrending":
		return d, d.gettmdbserietrending(list)
	case "tmdbmovieupcoming":
		return d, d.gettmdbmovieupcoming(list)
	case "newznabrss":
		return d, searcher.Getnewznabrss(ctx, cfgp, list)
	case "plexwatchlist":
		return d, d.getplexwatchlist(list)
	case "jellyfinwatchlist":
		return d, d.getjellyfinwatchlist(list)
	case "audiobookconfig":
		var err error

		d.Audiobooks, err = getaudiobookconfig(list.CfgList)
		logger.Logtype("debug", 1).
			Int("loaded_count", len(d.Audiobooks)).
			Str("config_file", list.CfgList.ManualConfigFile).
			Err(err).
			Msg("Loaded audiobook config")

		return d, err

	case "bookconfig":
		var err error

		d.Books, err = getbookconfig(list.CfgList)
		return d, err

	case "musicconfig":
		var err error

		d.Albums, err = getmusicconfig(list.CfgList)
		return d, err

	// Music chart scrapers
	// Set URL to the full chart URL, optionally including a date path segment:
	//   offiziellecharts.de example: https://www.offiziellecharts.de/charts/album/for-date-1772209337000
	//   officialcharts.com  example: https://www.officialcharts.com/charts/albums-chart/20260220/7502/
	// Any officialcharts.com chart is supported — just change the chart slug and/or chart-ID in the URL.
	// Configure ChartEntryNodeXPath, ChartTitleXPath, ChartArtistXPath in the list config.
	case "musiccharts":
		return d, d.getmusiccharts(ctx, list)

	case "lastfmtopartists":
		return d, d.getlastfmtopartists(ctx, list, cfgp)

	// Book bestseller scrapers
	// Supported sites (set URL accordingly):
	//   https://www.thalia.de/buch/aktuelles/buch-bestseller
	//   https://www.bestsellerliste.de/spiegel-bestseller-hardcover-belletristik/
	//   https://www.bestsellerliste.de/spiegel-bestseller-paperback-belletristik/
	//   https://www.bestsellerliste.de/spiegel-bestseller-taschenbuecher-belletristik/
	// Configure ChartEntryNodeXPath, ChartTitleXPath, ChartArtistXPath in the list config.
	case "bookbestsellers":
		return d, d.getbookbestsellers(ctx, list)

	// IRC announce channel monitoring.
	// Connects to irc_server, joins irc_channels and parses announce lines with
	// irc_announce_regex. The parent media type decides what is imported:
	//   movies -> imdb id, series -> name+tvdb, music -> album+artist,
	//   audiobooks/books -> title+author.
	case "irc":
		return d, d.getirc(ctx, cfgp, list)
	}

	return d, errors.New("switch not found " + list.CfgList.ListType)
}

// getmovieid checks if the given movie ID exists in the database for the specified list.
// It first checks the media cache if enabled, otherwise does a direct database query.
// Returns true if the movie ID exists in the list, false otherwise.
func getmovieid(dbid *uint, cfglistp *config.MediaListsConfig) bool {
	if config.GetSettingsGeneral().UseMediaCache {
		dbidn := *dbid
		if database.CacheOneStringTwoIntIndexFunc(
			logger.CacheMovie,
			func(elem *syncops.DbstaticOneStringTwoInt) bool {
				return elem.Num1 == dbidn &&
					(elem.Str == cfglistp.Name || strings.EqualFold(elem.Str, cfglistp.Name))
			},
		) {
			return true
		}
	} else if database.Getdatarow[uint](false, database.QueryCountMoviesByDBIDList, dbid, &cfglistp.Name) >= 1 {
		return true
	}

	return false
}

// getplexwatchlist retrieves watchlist items from a Plex Media Server.
func (d *feedResults) getplexwatchlist(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.PlexServerURL == "" || cfglist.CfgList.PlexToken == "" ||
		cfglist.CfgList.PlexUsername == "" {
		return errPlexConfigMissing
	}

	logger.Logtype("info", 1).
		Str("server", cfglist.CfgList.PlexServerURL).
		Msg("Fetching Plex watchlist")

	plexp := providers.GetPlex()
	if plexp == nil {
		return errPlexConfigMissing
	}

	watchlistItems, err := plexp.GetWatchlist(
		context.Background(),
		cfglist.CfgList.PlexServerURL,
		cfglist.CfgList.PlexToken,
	)
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str("server", cfglist.CfgList.PlexServerURL).
		Msg("Processing Plex watchlist items")

	for i := range watchlistItems {
		if watchlistItems[i].IsMovie() {
			// Process movie
			imdbID := watchlistItems[i].IMDbID()
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", watchlistItems[i].Title).
					Msg("No IMDB ID found for Plex movie")
			}
		} else if watchlistItems[i].IsShow() {
			// Process TV show
			tvdbID := watchlistItems[i].TVDbID()
			if tvdbID != 0 {
				checkaddtvdbfeed(tvdbID, watchlistItems[i].Title, cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", watchlistItems[i].Title).
					Msg("No TVDB ID found for Plex show")
			}
		}
	}

	return nil
}

// getjellyfinwatchlist retrieves watchlist items from a Jellyfin Media Server.
func (d *feedResults) getjellyfinwatchlist(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.JellyfinServerURL == "" || cfglist.CfgList.JellyfinToken == "" ||
		cfglist.CfgList.JellyfinUsername == "" {
		return errJellyfinConfigMissing
	}

	logger.Logtype("info", 1).
		Str("server", cfglist.CfgList.JellyfinServerURL).
		Msg("Fetching Jellyfin watchlist")

	jellyfinp := providers.GetJellyfin()
	if jellyfinp == nil {
		return errJellyfinConfigMissing
	}

	watchlistItems, err := jellyfinp.GetWatchlist(
		context.Background(),
		cfglist.CfgList.JellyfinServerURL,
		cfglist.CfgList.JellyfinToken,
		cfglist.CfgList.JellyfinUsername,
	)
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str("server", cfglist.CfgList.JellyfinServerURL).
		Msg("Processing Jellyfin watchlist items")

	for i := range watchlistItems {
		if watchlistItems[i].IsMovie() {
			// Process movie
			imdbID := watchlistItems[i].IMDbID()
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", watchlistItems[i].Title()).
					Msg("No IMDB ID found for Jellyfin movie")
			}
		} else if watchlistItems[i].IsSeries() {
			// Process TV series
			tvdbID := watchlistItems[i].TVDbID()
			if tvdbID != 0 {
				checkaddtvdbfeed(tvdbID, watchlistItems[i].Title(), cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", watchlistItems[i].Title()).
					Msg("No TVDB ID found for Jellyfin series")
			}
		}
	}

	return nil
}

// getmoviescraper runs a movie scraper to extract and import movies from external sources.
// It validates the scraper configuration, runs the appropriate scraper type (HTML/XPath or CSRF API),
// and populates d.Movies with IMDB IDs from the scraped movies.
func (d *feedResults) getmoviescraper(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) error {
	if cfglist.CfgList.MovieScraperType == "" {
		return errScraperTypeMissing
	}

	if cfglist.CfgList.MovieScraperStartURL == "" {
		return errScraperURLMissing
	}

	logger.Logtype("info", 1).
		Str("scraper_type", cfglist.CfgList.MovieScraperType).
		Str("start_url", cfglist.CfgList.MovieScraperStartURL).
		Msg("Starting movie scraper")

	// Run the movie scraper based on type
	imdbIDs, err := runMovieScraper(ctx, cfgp, cfglist)
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Int("count", len(imdbIDs)).
		Msg("Movie scraper completed")

	// Add IMDB IDs to Movies slice for import
	for idx := range imdbIDs {
		checkaddimdbfeed(&imdbIDs[idx], cfglist, d)
	}

	return nil
}

// runMovieScraper executes the movie scraper and returns a list of IMDB IDs.
// It creates the appropriate scraper based on the type and runs it to extract movie data.
func runMovieScraper(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) ([]string, error) {
	var imdbIDs []string

	switch cfglist.CfgList.MovieScraperType {
	case "htmlxpath":
		ids, err := runMovieHTMLXPathScraper(ctx, cfgp, cfglist)
		if err != nil {
			return nil, err
		}

		imdbIDs = ids

	case "csrfapi":
		ids, err := runMovieCSRFAPIScraper(ctx, cfgp, cfglist)
		if err != nil {
			return nil, err
		}

		imdbIDs = ids

	default:
		return nil, fmt.Errorf(
			"unsupported movie scraper type: %s",
			cfglist.CfgList.MovieScraperType,
		)
	}

	return imdbIDs, nil
}

// runMovieHTMLXPathScraper runs the HTML/XPath movie scraper.
// It scrapes movies from HTML pages using XPath selectors and returns IMDB IDs.
func runMovieHTMLXPathScraper(
	ctx context.Context,
	_ *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) ([]string, error) {
	// Import the htmlxpath package (will be added to imports at top)
	cfg := &htmlxpath.MovieConfig{
		SiteName:         cfglist.CfgList.Name,
		StartURL:         cfglist.CfgList.MovieScraperStartURL,
		BaseURL:          cfglist.CfgList.MovieScraperSiteURL,
		SceneNodeXPath:   cfglist.CfgList.MovieSceneNodeXPath,
		TitleXPath:       cfglist.CfgList.MovieTitleXPath,
		YearXPath:        cfglist.CfgList.MovieYearXPath,
		ImdbIDXPath:      cfglist.CfgList.MovieImdbIDXPath,
		URLXPath:         cfglist.CfgList.MovieURLXPath,
		RatingXPath:      cfglist.CfgList.MovieRatingXPath,
		GenreXPath:       cfglist.CfgList.MovieGenreXPath,
		ReleaseDateXPath: cfglist.CfgList.MovieReleaseDateXPath,
		TitleAttribute:   cfglist.CfgList.MovieTitleAttribute,
		URLAttribute:     cfglist.CfgList.MovieURLAttribute,
		PaginationType:   cfglist.CfgList.MoviePaginationType,
		PageIncrement:    cfglist.CfgList.MoviePageIncrement,
		PageURLPattern:   cfglist.CfgList.MoviePageURLPattern,
		DateFormat:       cfglist.CfgList.MovieDateFormat,
		WaitSeconds:      cfglist.CfgList.MovieWaitSeconds,
	}

	scraper, err := htmlxpath.NewMovieScraper(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create movie scraper: %w", err)
	}

	// Scrape movies (limit to 10 pages for now)
	imdbIDs, err := scraper.Scrape(ctx, 10)
	if err != nil {
		if len(imdbIDs) == 0 {
			return nil, fmt.Errorf("failed to scrape movies: %w", err)
		}

		logger.Logtype("warn", 0).
			Err(err).
			Str("list", cfglist.CfgList.Name).
			Int("imdb_ids", len(imdbIDs)).
			Msg("Movie scraper stopped early; using ids from earlier pages")
	}

	return imdbIDs, nil
}

// extractChartText extracts text from an HTML node via XPath, optionally from an attribute.
func extractChartText(node *html.Node, xpath, attr string) string {
	if xpath == "" {
		return ""
	}

	found := htmlquery.FindOne(node, xpath)
	if found == nil {
		return ""
	}

	if attr != "" {
		return strings.TrimSpace(htmlquery.SelectAttr(found, attr))
	}

	return strings.TrimSpace(htmlquery.InnerText(found))
}

// fetchAndParseChartPage fetches url, parses the HTML and calls fn with the document root.
// The request is bounded by a timeout so a slow or hung chart server cannot block
// the feed job indefinitely.
func fetchAndParseChartPage(ctx context.Context, url string, fn func(*html.Node)) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("chart page request: %w", err)
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	)
	req.Header.Set(
		"Accept",
		"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
	)
	req.Header.Set("Accept-Language", "de-DE,de;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("chart page fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chart page returned status %d for %s", resp.StatusCode, url)
	}

	doc, err := htmlquery.Parse(resp.Body)
	if err != nil {
		return fmt.Errorf("chart page parse: %w", err)
	}

	fn(doc)

	return nil
}

// getmusiccharts fetches a music chart page (offiziellecharts.de, officialcharts.com, or any
// HTML chart page) and appends the scraped entries to d.Albums as ManualConfig values.
//
// Configuration (set in the list config):
//   - URL: full chart URL, optionally including a date path segment
//     offiziellecharts.de:  https://www.offiziellecharts.de/charts/album/for-date-1772209337000
//     officialcharts.com:   https://www.officialcharts.com/charts/albums-chart/20260220/7502/
//   - ChartEntryNodeXPath: XPath that selects each chart entry node
//   - ChartTitleXPath:     XPath (relative to entry) for the album title
//   - ChartArtistXPath:    XPath (relative to entry) for the artist name (optional)
//   - ChartTitleAttribute / ChartArtistAttribute: use an HTML attribute instead of inner text
//
// chartScrapeConfig parameterizes scrapeChart for a specific media type
// (music albums vs books): the table/column names, the validation errors to
// return, the log message, and a callback that appends a scraped entry.
type chartScrapeConfig struct {
	table, dbTable, fkColumn   string
	errURL, errEntry, errTitle error
	logMsg                     string
	// artistMatch is an SQL condition (one ? placeholder) restricting a
	// title match to entries by the scraped artist/author.
	artistMatch  string
	appendResult func(title, artist string)
}

// scrapeChart fetches an HTML chart/bestseller page and appends matching entries
// via sc.appendResult. It validates required config, skips entries already tracked
// in an ignore-template list, and moves entries found in a replace-template list to
// the current list. Shared by getmusiccharts and getbookbestsellers.
func (d *feedResults) scrapeChart(
	ctx context.Context,
	cfglist *config.MediaListsConfig,
	sc *chartScrapeConfig,
) error {
	if cfglist.CfgList.URL == "" {
		return sc.errURL
	}

	if cfglist.CfgList.ChartEntryNodeXPath == "" {
		return sc.errEntry
	}

	if cfglist.CfgList.ChartTitleXPath == "" {
		return sc.errTitle
	}

	// Pre-build query strings once; per-entry args are built inside the loop.
	join := sc.table + " JOIN " + sc.dbTable + " ON " + sc.table + "." + sc.fkColumn + " = " + sc.dbTable + ".id"
	titleMatch := "LOWER(" + sc.dbTable + ".title) = LOWER(?) AND " + sc.table + ".listname IN (?"
	// Same as titleMatch plus the artist/author condition; used when the chart
	// provides an artist so same-titled entries by other artists don't match.
	titleArtistMatch := "LOWER(" + sc.dbTable + ".title) = LOWER(?) AND " + sc.artistMatch +
		" AND " + sc.table + ".listname IN (?"
	useArtist := sc.artistMatch != "" && cfglist.CfgList.ChartArtistXPath != ""

	checkIgnore := cfglist.IgnoreMapListsLen >= 1
	ignoreQuery := "SELECT count() FROM " + join + " WHERE " + titleMatch + cfglist.IgnoreMapListsQu + ")"
	ignoreArtistQuery := "SELECT count() FROM " + join + " WHERE " + titleArtistMatch + cfglist.IgnoreMapListsQu + ")"

	checkReplace := cfglist.ReplaceMapListsLen >= 1

	var replaceQuery, replaceArtistQuery, updateListnameQuery, updateListnameQualityQuery string
	if checkReplace {
		replaceQu := strings.Repeat(",?", cfglist.ReplaceMapListsLen-1)

		replaceQuery = "SELECT " + sc.table + ".id FROM " + join + " WHERE " + titleMatch + replaceQu + ")"
		replaceArtistQuery = "SELECT " + sc.table + ".id FROM " + join + " WHERE " + titleArtistMatch + replaceQu + ")"
		updateListnameQuery = "UPDATE " + sc.table + " SET listname = ? WHERE id = ?"
		updateListnameQualityQuery = "UPDATE " + sc.table + " SET listname = ?, quality_profile = ? WHERE id = ?"
	}

	return fetchAndParseChartPage(ctx, cfglist.CfgList.URL, func(doc *html.Node) {
		nodes := htmlquery.Find(doc, cfglist.CfgList.ChartEntryNodeXPath)

		if limit, err := strconv.Atoi(
			cfglist.CfgList.Limit,
		); err == nil && limit > 0 &&
			len(nodes) > limit {
			nodes = nodes[:limit]
		}

		var added int

		qargs := make([]any, 0, cfglist.IgnoreMapListsLen+2)
		rqargs := make([]any, 0, cfglist.ReplaceMapListsLen+2)

		for _, node := range nodes {
			title := extractChartText(
				node,
				cfglist.CfgList.ChartTitleXPath,
				cfglist.CfgList.ChartTitleAttribute,
			)
			if title == "" {
				continue
			}

			scrapedArtist := extractChartText(
				node,
				cfglist.CfgList.ChartArtistXPath,
				cfglist.CfgList.ChartArtistAttribute,
			)
			matchArtist := useArtist && scrapedArtist != ""

			// Skip entries that are already tracked in any ignore-template list.
			if checkIgnore {
				qargs = qargs[:0]

				query := ignoreQuery
				qargs = append(qargs, title)
				if matchArtist {
					query = ignoreArtistQuery
					qargs = append(qargs, scrapedArtist)
				}

				for i := range cfglist.IgnoreMapLists {
					qargs = append(qargs, cfglist.IgnoreMapLists[i])
				}

				if database.Getdatarow[uint](false, query, qargs...) > 0 {
					continue
				}
			}

			// Move entries that exist in a replace-template list to the current list.
			if checkReplace {
				rqargs = rqargs[:0]

				query := replaceQuery
				rqargs = append(rqargs, title)
				if matchArtist {
					query = replaceArtistQuery
					rqargs = append(rqargs, scrapedArtist)
				}

				for i := range cfglist.ReplaceMapLists {
					rqargs = append(rqargs, cfglist.ReplaceMapLists[i])
				}

				if id := database.Getdatarow[uint](false, query, rqargs...); id > 0 {
					if cfglist.TemplateQuality == "" {
						database.ExecN(updateListnameQuery, &cfglist.Name, &id)
					} else {
						database.ExecN(
							updateListnameQualityQuery,
							&cfglist.Name, &cfglist.TemplateQuality, &id,
						)
					}

					continue // already tracked; moved to this list in-place
				}
			}

			artist := scrapedArtist
			if artist == "" {
				artist = cfglist.CfgList.ChartDefaultArtist
			}

			sc.appendResult(title, artist)

			added++
		}

		logger.Logtype("info", 2).
			Str(logger.StrURL, cfglist.CfgList.URL).
			Int(strtoparse, added).
			Msg(sc.logMsg)
	})
}

func (d *feedResults) getmusiccharts(ctx context.Context, cfglist *config.MediaListsConfig) error {
	return d.scrapeChart(ctx, cfglist, &chartScrapeConfig{
		table:    "albums",
		dbTable:  "dbalbums",
		fkColumn: "dbalbum_id",
		errURL:   errMusicchartsURLRequired,
		errEntry: errMusicchartsEntryNodeRequired,
		errTitle: errMusicchartsTitleRequired,
		logMsg:   "music chart fetched",
		artistMatch: "EXISTS (SELECT 1 FROM dbalbum_artists x JOIN dbartists y ON y.id = x.dbartist_id " +
			"WHERE x.dbalbum_id = dbalbums.id AND LOWER(y.name) = LOWER(?))",
		appendResult: func(title, artist string) {
			d.Albums = append(d.Albums, config.ManualConfig{Name: title, ArtistName: artist})
		},
	})
}

// getbookbestsellers fetches a book bestseller page (thalia.de, bestsellerliste.de/Spiegel, or
// any HTML bestseller page) and appends the scraped entries to d.Books as ManualConfig values.
//
// Configuration (set in the list config):
//   - URL: full bestseller page URL, e.g.:
//     https://www.thalia.de/buch/aktuelles/buch-bestseller
//     https://www.bestsellerliste.de/spiegel-bestseller-hardcover-belletristik/
//     https://www.bestsellerliste.de/spiegel-bestseller-paperback-belletristik/
//     https://www.bestsellerliste.de/spiegel-bestseller-taschenbuecher-belletristik/
//   - ChartEntryNodeXPath: XPath that selects each bestseller entry node
//   - ChartTitleXPath:     XPath (relative to entry) for the book title
//   - ChartArtistXPath:    XPath (relative to entry) for the author name (optional)
//   - ChartTitleAttribute / ChartArtistAttribute: use an HTML attribute instead of inner text
func (d *feedResults) getbookbestsellers(ctx context.Context, cfglist *config.MediaListsConfig) error {
	return d.scrapeChart(ctx, cfglist, &chartScrapeConfig{
		table:    "books",
		dbTable:  "dbbooks",
		fkColumn: "dbbook_id",
		errURL:   errBookbestsellersURLRequired,
		errEntry: errBookbestsellersEntryRequired,
		errTitle: errBookbestsellersTitleRequired,
		logMsg:   "book bestseller list fetched",
		artistMatch: "EXISTS (SELECT 1 FROM dbauthors y WHERE LOWER(y.name) = LOWER(?) AND " +
			"(y.id = dbbooks.dbauthor_id OR y.id IN (SELECT dbauthor_id FROM dbbook_authors WHERE dbbook_id = dbbooks.id)))",
		appendResult: func(title, author string) {
			d.Books = append(d.Books, config.ManualConfig{Name: title, AuthorName: author})
		},
	})
}

// runMovieCSRFAPIScraper runs the CSRF API movie scraper.
// It scrapes movies from CSRF-protected JSON APIs and returns IMDB IDs.
func runMovieCSRFAPIScraper(
	ctx context.Context,
	_ *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) ([]string, error) {
	// Import the csrfapi package (will be added to imports at top)
	cfg := &csrfapi.MovieConfig{
		SiteName:         cfglist.CfgList.Name,
		StartURL:         cfglist.CfgList.MovieScraperStartURL,
		BaseURL:          cfglist.CfgList.MovieScraperSiteURL,
		CSRFCookieName:   cfglist.CfgList.MovieCSRFCookieName,
		CSRFHeaderName:   cfglist.CfgList.MovieCSRFHeaderName,
		APIURLPattern:    cfglist.CfgList.MovieAPIURLPattern,
		PageStartIndex:   cfglist.CfgList.MoviePageStartIndex,
		ResultsArrayPath: cfglist.CfgList.MovieResultsArrayPath,
		TitleField:       cfglist.CfgList.MovieTitleField,
		YearField:        cfglist.CfgList.MovieYearField,
		ImdbIDField:      cfglist.CfgList.MovieImdbIDField,
		URLField:         cfglist.CfgList.MovieURLField,
		RatingField:      cfglist.CfgList.MovieRatingField,
		GenreField:       cfglist.CfgList.MovieGenreField,
		ReleaseDateField: cfglist.CfgList.MovieReleaseDateField,
		DateFormat:       cfglist.CfgList.MovieDateFormat,
		WaitSeconds:      cfglist.CfgList.MovieWaitSeconds,
	}

	scraper, err := csrfapi.NewMovieScraper(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSRF API movie scraper: %w", err)
	}

	// Scrape movies (limit to 10 pages for now)
	imdbIDs, err := scraper.Scrape(ctx, 10)
	if err != nil {
		if len(imdbIDs) == 0 {
			return nil, fmt.Errorf("failed to scrape movies: %w", err)
		}

		logger.Logtype("warn", 0).
			Err(err).
			Str("list", cfglist.CfgList.Name).
			Int("imdb_ids", len(imdbIDs)).
			Msg("Movie scraper stopped early; using ids from earlier pages")
	}

	return imdbIDs, nil
}
