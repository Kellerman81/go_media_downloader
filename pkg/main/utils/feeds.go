package utils

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/csrfapi"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/htmlxpath"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
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
}

// Sentinel errors for feed processing.
var (
	errwrongtype             = errors.New("wrong type")
	errcsvread               = errors.New("csv read")
	errusernameempty         = errors.New("username empty")
	errlistnameempty         = errors.New("list type empty")
	errPlexConfigMissing     = errors.New("plex server URL, token, and username are required")
	errJellyfinConfigMissing = errors.New("jellyfin server URL, API key, and username are required")
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

		return false
	})
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
	traktLimiter := providers.GetTrakt().GetRateLimiter()
	if traktLimiter != nil {
		if allowed, waitTime := traktLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	switch usetraktmovie {
	case 1:
		for arr := range apiexternal.GetTraktMoviePopular(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			checkaddimdbfeed(&arr.IDs.Imdb, cfglist, d)
		}

	case 2:
		for arr := range apiexternal.GetTraktMovieTrending(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			checkaddimdbfeed(&arr.IDs.Imdb, cfglist, d)
		}

	case 3:
		for arr := range apiexternal.GetTraktMovieAnticipated(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			checkaddimdbfeed(&arr.IDs.Imdb, cfglist, d)
		}

	case 4:
		return d.getTraktUserMovieList(cfglist)
	default:
		return errwrongtype
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

	traktLimiter := providers.GetTrakt().GetRateLimiter()
	if traktLimiter != nil {
		if allowed, waitTime := traktLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTraktUserList(
		cfglist.CfgList.TraktUsername,
		cfglist.CfgList.TraktListName,
		cfglist.CfgList.TraktListType,
		&cfglist.CfgList.Limit,
	)
	if err != nil {
		return err
	}

	removeFromList := cfglist.CfgList.RemoveFromList && cfglist.CfgList.TraktListName != "watchlist"
	for idx := range arr {
		if checkaddimdbfeed(&arr[idx].Movie.IDs.Imdb, cfglist, d) && removeFromList {
			apiexternal.RemoveMovieFromTraktUserList(
				cfglist.CfgList.TraktUsername,
				cfglist.CfgList.TraktListName,
				arr[idx].Movie.IDs.Imdb,
			)
		}
	}

	return nil
}

// gettmdbmoviepopular retrieves a list of popular movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmoviepopular(cfglist *config.MediaListsConfig) error {
	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	if tmdbLimiter != nil {
		if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTmdbPopularMovies(&cfglist.CfgList.Limit)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbmovietrending retrieves a list of trending movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmovietrending(cfglist *config.MediaListsConfig) error {
	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	if tmdbLimiter != nil {
		if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTmdbTrendingMovies(&cfglist.CfgList.Limit)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbmovieupcoming retrieves a list of upcoming movies from The Movie Database (TMDb).
// It gets the IMDB ID for each movie and adds it to the d.Movies slice.
func (d *feedResults) gettmdbmovieupcoming(cfglist *config.MediaListsConfig) error {
	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	if tmdbLimiter != nil {
		if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTmdbUpcomingMovies(&cfglist.CfgList.Limit)
	if err != nil {
		return err
	}

	return d.processTmdbMovieResults(arr, cfglist)
}

// gettmdbseriepopular retrieves a list of popular TV series from The Movie Database (TMDb).
// It gets the TVDB ID for each series and adds it to the d.Series slice.
func (d *feedResults) gettmdbseriepopular(cfglist *config.MediaListsConfig) error {
	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	if tmdbLimiter != nil {
		if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTmdbPopularSeries(&cfglist.CfgList.Limit)
	if err != nil {
		return err
	}

	return d.processTmdbSeriesResults(arr, cfglist)
}

// gettmdbserietrending retrieves a list of trending TV series from The Movie Database (TMDb).
// It gets the TVDB ID for each series and adds it to the d.Series slice.
func (d *feedResults) gettmdbserietrending(cfglist *config.MediaListsConfig) error {
	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	if tmdbLimiter != nil {
		if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTmdbTrendingSeries(&cfglist.CfgList.Limit)
	if err != nil {
		return err
	}

	return d.processTmdbSeriesResults(arr, cfglist)
}

// processTmdbMovieResults processes TMDB movie search results and adds allowed movies to d.Movies.
func (d *feedResults) processTmdbMovieResults(
	arr *apiexternal.TheMovieDBSearch,
	cfglist *config.MediaListsConfig,
) error {
	if arr == nil {
		return nil
	}

	var allowed bool
	for idx := range arr.Results {
		moviedbexternal, err := apiexternal.GetTmdbMovieExternal(arr.Results[idx].ID)
		if err != nil {
			logger.Logtype("debug", 1).
				Int(logger.StrImdb, arr.Results[idx].ID).
				Err(err).
				Msg("imdb id could not be retrieved")

			continue
		}

		allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
		if allowed || d.AddAll {
			d.Movies = append(d.Movies, moviedbexternal.ImdbID)
		}
	}

	return nil
}

// processTmdbSeriesResults processes TMDB series search results and adds series to d.Series.
func (d *feedResults) processTmdbSeriesResults(
	arr *apiexternal.TheMovieDBSearchTV,
	_ *config.MediaListsConfig,
) error {
	if arr == nil {
		return nil
	}

	var imdbid string
	for idx := range arr.Results {
		database.Scanrowsdyn(
			false,
			"select id from dbseries where moviedb_id = ?",
			&imdbid,
			&arr.Results[idx].ID,
		)

		if imdbid != "" && !d.AddAll {
			continue
		}

		moviedbexternal, err := apiexternal.GetTVExternal(arr.Results[idx].ID)
		if err != nil || moviedbexternal == nil || moviedbexternal.TvdbID == 0 {
			continue
		}

		d.Series = append(d.Series, config.ManualConfig{
			Name: arr.Results[idx].Name, TvdbID: moviedbexternal.TvdbID,
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

	tmdbLimiter := providers.GetTMDB().GetRateLimiter()

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		query := cfglist.CfgList.TmdbDiscover[idxdiscover]

		if tmdbLimiter != nil {
			if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
				time.Sleep(waitTime)
			}
		}

		arr, err := apiexternal.DiscoverTmdbMovie(query)
		if err != nil {
			logger.Logtype("debug", 1).
				Str("query", query).
				Err(err).
				Msg("discover could not be executed")

			continue
		}

		for idx := range arr.Results {
			if tmdbLimiter != nil {
				if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
					time.Sleep(waitTime)
				}
			}

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

			moviedbexternal, err := apiexternal.GetTmdbMovieExternal(arr.Results[idx].ID)
			if err != nil {
				logger.Logtype("debug", 1).
					Int(logger.StrImdb, arr.Results[idx].ID).
					Err(err).
					Msg("imdb id could not be retrieved")

				continue
			}

			allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
			if allowed || d.AddAll {
				d.Movies = append(d.Movies, moviedbexternal.ImdbID)
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

	var (
		imdbid          string
		moviedbexternal *apiexternal.TheMovieDBTVExternal
	)

	tmdbLimiter := providers.GetTMDB().GetRateLimiter()

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		if tmdbLimiter != nil {
			if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
				time.Sleep(waitTime)
			}
		}

		arr, err := apiexternal.DiscoverTmdbSerie(cfglist.CfgList.TmdbDiscover[idxdiscover])
		if err != nil {
			continue
		}

		for idx := range arr.Results {
			database.Scanrowsdyn(
				false,
				"select id from dbseries where moviedb_id = ?",
				&imdbid,
				&arr.Results[idx].ID,
			)

			if imdbid == "" || d.AddAll {
				if tmdbLimiter != nil {
					if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
						time.Sleep(waitTime)
					}
				}

				moviedbexternal, err = apiexternal.GetTVExternal(arr.Results[idx].ID)
				if err != nil || moviedbexternal == nil || moviedbexternal.TvdbID == 0 {
					continue
				}

				d.Series = append(d.Series, config.ManualConfig{
					Name: arr.Results[idx].Name, TvdbID: moviedbexternal.TvdbID,
				})
			}
		}
	}

	return nil
}

// gettmdbshowlist retrieves a list of TV shows from The Movie Database (TMDb) based on the configuration settings in the provided MediaListsConfig.
// It iterates through the TmdbList configuration, fetching the TV show details from TMDb and adding them to the d.Series slice if they are not already in the database.
func (d *feedResults) gettmdbshowlist(cfglist *config.MediaListsConfig) error {
	if len(cfglist.CfgList.TmdbDiscover) == 0 {
		return nil
	}

	var (
		imdbid          string
		moviedbexternal *apiexternal.TheMovieDBTVExternal
	)

	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	for idxdiscover := range cfglist.CfgList.TmdbList {
		if tmdbLimiter != nil {
			if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
				time.Sleep(waitTime)
			}
		}

		arr, err := apiexternal.GetTmdbList(cfglist.CfgList.TmdbList[idxdiscover])
		if err != nil {
			continue
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

			if tmdbLimiter != nil {
				if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
					time.Sleep(waitTime)
				}
			}

			moviedbexternal, err = apiexternal.GetTVExternal(arr.Items[idx].ID)
			if err != nil || moviedbexternal == nil || moviedbexternal.TvdbID == 0 {
				continue
			}

			d.Series = append(d.Series, config.ManualConfig{
				Name: arr.Items[idx].Name, TvdbID: moviedbexternal.TvdbID,
			})
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

	if database.Getdatarow[uint](
		false,
		"select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE",
		imdb,
		&cfglist.Name,
	) == 0 || d.AddAll {
		d.Movies = append(d.Movies, *imdb)
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

	tmdbLimiter := providers.GetTMDB().GetRateLimiter()
	for idxlist := range cfglist.CfgList.TmdbList {
		if tmdbLimiter != nil {
			if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
				time.Sleep(waitTime)
			}
		}

		arr, err := apiexternal.GetTmdbList(cfglist.CfgList.TmdbList[idxlist])
		if err != nil {
			continue
		}

		listID := cfglist.CfgList.TmdbList[idxlist]
		for idx := range arr.Items {
			itemID := arr.Items[idx].ID

			if tmdbLimiter != nil {
				if allowed, waitTime := tmdbLimiter.Check(); !allowed && waitTime > 0 {
					time.Sleep(waitTime)
				}
			}

			movieid = importfeed.MovieFindDBIDByTmdbID(&itemID)

			if movieid != 0 {
				if getmovieid(&movieid, cfglist) && !d.AddAll {
					if removeFromList {
						apiexternal.RemoveFromTmdbList(listID, itemID)
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
								apiexternal.RemoveFromTmdbList(listID, itemID)
							}

							continue
						}
					} else if listnamefilter != "" && slices.Contains(existing, movieid) {
						if removeFromList {
							apiexternal.RemoveFromTmdbList(listID, itemID)
						}

						continue
					}
				}
			}

			moviedbexternal, err := apiexternal.GetTmdbMovieExternal(itemID)
			if err != nil {
				logger.Logtype("debug", 1).
					Int(logger.StrImdb, itemID).
					Err(err).
					Msg("imdb id could not be retrieved")

				continue
			}

			allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
			if !allowed && !d.AddAll {
				continue
			}

			d.Movies = append(d.Movies, moviedbexternal.ImdbID)

			if removeFromList {
				apiexternal.RemoveFromTmdbList(listID, itemID)
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
	traktLimiter := providers.GetTrakt().GetRateLimiter()
	if traktLimiter != nil {
		if allowed, waitTime := traktLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	switch usetraktserie {
	case 1:
		for arr := range apiexternal.GetTraktSeriePopular(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			d.Series = append(d.Series, config.ManualConfig{
				Name: arr.Title, TvdbID: arr.IDs.Tvdb,
			})
		}

	case 2:
		for arr := range apiexternal.GetTraktSerieTrending(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			d.Series = append(d.Series, config.ManualConfig{
				Name: arr.Title, TvdbID: arr.IDs.Tvdb,
			})
		}

	case 3:
		for arr := range apiexternal.GetTraktSerieAnticipated(&cfglist.CfgList.Limit, cfglist.CfgList.URL) {
			d.Series = append(d.Series, config.ManualConfig{
				Name: arr.Title, TvdbID: arr.IDs.Tvdb,
			})
		}

	case 4:
		return d.getTraktUserSeriesList(cfglist)
	default:
		return errwrongtype
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

	traktLimiter := providers.GetTrakt().GetRateLimiter()
	if traktLimiter != nil {
		if allowed, waitTime := traktLimiter.Check(); !allowed && waitTime > 0 {
			time.Sleep(waitTime)
		}
	}

	arr, err := apiexternal.GetTraktUserList(
		cfglist.CfgList.TraktUsername,
		cfglist.CfgList.TraktListName,
		cfglist.CfgList.TraktListType,
		&cfglist.CfgList.Limit,
	)
	if err != nil {
		return err
	}

	removeFromList := cfglist.CfgList.RemoveFromList
	for idx := range arr {
		d.Series = append(d.Series, config.ManualConfig{
			Name: arr[idx].Serie.Title, TvdbID: arr[idx].Serie.IDs.Tvdb,
		})
		if removeFromList {
			apiexternal.RemoveSerieFromTraktUserList(
				cfglist.CfgList.TraktUsername,
				cfglist.CfgList.TraktListName,
				arr[idx].Movie.IDs.Tvdb,
			)
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
	return apiexternal.ProcessHTTP(
		nil,
		cfglistp.CfgList.URL,
		false,
		func(ctx context.Context, r *http.Response) error {
			d.parseimdbcsv(ctx, r.Body, cfglistp)
			return nil
		},
		nil,
	)
}

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
		return d, d.getmoviescraper(cfgp, list)

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
		return d, searcher.Getnewznabrss(cfgp, list)
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
		return d, d.getmusiccharts(list)

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
		return d, d.getbookbestsellers(list)
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

	watchlistItems, err := apiexternal.GetPlexWatchlist(
		cfglist.CfgList.PlexServerURL,
		cfglist.CfgList.PlexToken,
		cfglist.CfgList.PlexUsername,
	)
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str("server", cfglist.CfgList.PlexServerURL).
		Msg("Processing Plex watchlist items")

	for i := range watchlistItems {
		if apiexternal.IsPlexItemMovie(watchlistItems[i]) {
			// Process movie
			imdbID := apiexternal.ExtractIMDBFromPlexItem(watchlistItems[i])
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", watchlistItems[i].Title).
					Msg("No IMDB ID found for Plex movie")
			}
		} else if apiexternal.IsPlexItemShow(watchlistItems[i]) {
			// Process TV show
			tvdbID := apiexternal.ExtractTVDBFromPlexItem(watchlistItems[i])
			if tvdbID != 0 {
				d.Series = append(d.Series, config.ManualConfig{
					Name:   watchlistItems[i].Title,
					TvdbID: tvdbID,
				})
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

	watchlistItems, err := apiexternal.GetJellyfinWatchlist(
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
		if apiexternal.IsJellyfinItemMovie(watchlistItems[i]) {
			// Process movie
			imdbID := apiexternal.ExtractIMDBFromJellyfinItem(watchlistItems[i])
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).
					Str("title", apiexternal.GetJellyfinItemTitle(watchlistItems[i])).
					Msg("No IMDB ID found for Jellyfin movie")
			}
		} else if apiexternal.IsJellyfinItemSeries(watchlistItems[i]) {
			// Process TV series
			tvdbID := apiexternal.ExtractTVDBFromJellyfinItem(watchlistItems[i])
			if tvdbID != 0 {
				d.Series = append(d.Series, config.ManualConfig{
					Name:   apiexternal.GetJellyfinItemTitle(watchlistItems[i]),
					TvdbID: tvdbID,
				})
			} else {
				logger.Logtype("debug", 1).
					Str("title", apiexternal.GetJellyfinItemTitle(watchlistItems[i])).
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
	imdbIDs, err := runMovieScraper(cfgp, cfglist)
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
	cfgp *config.MediaTypeConfig,
	cfglist *config.MediaListsConfig,
) ([]string, error) {
	var imdbIDs []string

	switch cfglist.CfgList.MovieScraperType {
	case "htmlxpath":
		ids, err := runMovieHTMLXPathScraper(cfgp, cfglist)
		if err != nil {
			return nil, err
		}

		imdbIDs = ids

	case "csrfapi":
		ids, err := runMovieCSRFAPIScraper(cfgp, cfglist)
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
	ctx := context.Background()

	imdbIDs, err := scraper.Scrape(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape movies: %w", err)
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
func fetchAndParseChartPage(url string, fn func(*html.Node)) error {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
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
func (d *feedResults) getmusiccharts(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.URL == "" {
		return errMusicchartsURLRequired
	}

	if cfglist.CfgList.ChartEntryNodeXPath == "" {
		return errMusicchartsEntryNodeRequired
	}

	if cfglist.CfgList.ChartTitleXPath == "" {
		return errMusicchartsTitleRequired
	}

	// Pre-build query strings once; per-entry args are built inside the loop.
	checkIgnoreAlbum := cfglist.IgnoreMapListsLen >= 1
	ignoreAlbumQuery := "SELECT count() FROM albums JOIN dbalbums ON albums.dbalbum_id = dbalbums.id WHERE LOWER(dbalbums.title) = LOWER(?) AND albums.listname IN (?" + cfglist.IgnoreMapListsQu + ")"

	checkReplaceAlbum := cfglist.ReplaceMapListsLen >= 1

	var replaceAlbumQuery string
	if checkReplaceAlbum {
		replaceAlbumQu := strings.Repeat(",?", cfglist.ReplaceMapListsLen-1)

		replaceAlbumQuery = "SELECT albums.id FROM albums JOIN dbalbums ON albums.dbalbum_id = dbalbums.id WHERE LOWER(dbalbums.title) = LOWER(?) AND albums.listname IN (?" + replaceAlbumQu + ")"
	}

	return fetchAndParseChartPage(cfglist.CfgList.URL, func(doc *html.Node) {
		nodes := htmlquery.Find(doc, cfglist.CfgList.ChartEntryNodeXPath)

		if limit, err := strconv.Atoi(
			cfglist.CfgList.Limit,
		); err == nil && limit > 0 &&
			len(nodes) > limit {
			nodes = nodes[:limit]
		}

		for _, node := range nodes {
			title := extractChartText(
				node,
				cfglist.CfgList.ChartTitleXPath,
				cfglist.CfgList.ChartTitleAttribute,
			)
			if title == "" {
				continue
			}

			// Skip albums that are already tracked in any ignore-template list.
			if checkIgnoreAlbum {
				qargs := make([]any, 0, cfglist.IgnoreMapListsLen+1)

				qargs = append(qargs, title)
				for i := range cfglist.IgnoreMapLists {
					qargs = append(qargs, cfglist.IgnoreMapLists[i])
				}

				if database.Getdatarow[uint](false, ignoreAlbumQuery, qargs...) > 0 {
					continue
				}
			}

			// Move albums that exist in a replace-template list to the current list.
			if checkReplaceAlbum {
				rqargs := make([]any, 0, cfglist.ReplaceMapListsLen+1)

				rqargs = append(rqargs, title)
				for i := range cfglist.ReplaceMapLists {
					rqargs = append(rqargs, cfglist.ReplaceMapLists[i])
				}

				if albumID := database.Getdatarow[uint](
					false,
					replaceAlbumQuery,
					rqargs...); albumID > 0 {
					if cfglist.TemplateQuality == "" {
						database.ExecN(
							"UPDATE albums SET listname = ? WHERE id = ?",
							&cfglist.Name,
							&albumID,
						)
					} else {
						database.ExecN(
							"UPDATE albums SET listname = ?, quality_profile = ? WHERE id = ?",
							&cfglist.Name, &cfglist.TemplateQuality, &albumID,
						)
					}

					continue // already tracked; moved to this list in-place
				}
			}

			artist := extractChartText(
				node,
				cfglist.CfgList.ChartArtistXPath,
				cfglist.CfgList.ChartArtistAttribute,
			)
			if artist == "" {
				artist = cfglist.CfgList.ChartDefaultArtist
			}

			d.Albums = append(d.Albums, config.ManualConfig{
				Name:       title,
				ArtistName: artist,
			})
		}

		logger.Logtype("info", 2).
			Str(logger.StrURL, cfglist.CfgList.URL).
			Int(strtoparse, len(d.Albums)).
			Msg("music chart fetched")
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
func (d *feedResults) getbookbestsellers(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.URL == "" {
		return errBookbestsellersURLRequired
	}

	if cfglist.CfgList.ChartEntryNodeXPath == "" {
		return errBookbestsellersEntryRequired
	}

	if cfglist.CfgList.ChartTitleXPath == "" {
		return errBookbestsellersTitleRequired
	}

	// Pre-build query strings once; per-entry args are built inside the loop.
	checkIgnoreBook := cfglist.IgnoreMapListsLen >= 1
	ignoreBookQuery := "SELECT count() FROM books JOIN dbbooks ON books.dbbook_id = dbbooks.id WHERE LOWER(dbbooks.title) = LOWER(?) AND books.listname IN (?" + cfglist.IgnoreMapListsQu + ")"

	checkReplaceBook := cfglist.ReplaceMapListsLen >= 1

	var replaceBookQuery string
	if checkReplaceBook {
		replaceBookQu := strings.Repeat(",?", cfglist.ReplaceMapListsLen-1)

		replaceBookQuery = "SELECT books.id FROM books JOIN dbbooks ON books.dbbook_id = dbbooks.id WHERE LOWER(dbbooks.title) = LOWER(?) AND books.listname IN (?" + replaceBookQu + ")"
	}

	return fetchAndParseChartPage(cfglist.CfgList.URL, func(doc *html.Node) {
		nodes := htmlquery.Find(doc, cfglist.CfgList.ChartEntryNodeXPath)

		if limit, err := strconv.Atoi(
			cfglist.CfgList.Limit,
		); err == nil && limit > 0 &&
			len(nodes) > limit {
			nodes = nodes[:limit]
		}

		for _, node := range nodes {
			title := extractChartText(
				node,
				cfglist.CfgList.ChartTitleXPath,
				cfglist.CfgList.ChartTitleAttribute,
			)
			if title == "" {
				continue
			}

			// Skip books that are already tracked in any ignore-template list.
			if checkIgnoreBook {
				qargs := make([]any, 0, cfglist.IgnoreMapListsLen+1)

				qargs = append(qargs, title)
				for i := range cfglist.IgnoreMapLists {
					qargs = append(qargs, cfglist.IgnoreMapLists[i])
				}

				if database.Getdatarow[uint](false, ignoreBookQuery, qargs...) > 0 {
					continue
				}
			}

			// Move books that exist in a replace-template list to the current list.
			if checkReplaceBook {
				rqargs := make([]any, 0, cfglist.ReplaceMapListsLen+1)

				rqargs = append(rqargs, title)
				for i := range cfglist.ReplaceMapLists {
					rqargs = append(rqargs, cfglist.ReplaceMapLists[i])
				}

				if bookID := database.Getdatarow[uint](
					false,
					replaceBookQuery,
					rqargs...); bookID > 0 {
					if cfglist.TemplateQuality == "" {
						database.ExecN(
							"UPDATE books SET listname = ? WHERE id = ?",
							&cfglist.Name,
							&bookID,
						)
					} else {
						database.ExecN(
							"UPDATE books SET listname = ?, quality_profile = ? WHERE id = ?",
							&cfglist.Name, &cfglist.TemplateQuality, &bookID,
						)
					}

					continue // already tracked; moved to this list in-place
				}
			}

			author := extractChartText(
				node,
				cfglist.CfgList.ChartArtistXPath,
				cfglist.CfgList.ChartArtistAttribute,
			)
			if author == "" {
				author = cfglist.CfgList.ChartDefaultArtist
			}

			d.Books = append(d.Books, config.ManualConfig{
				Name:       title,
				AuthorName: author,
			})
		}

		logger.Logtype("info", 2).
			Str(logger.StrURL, cfglist.CfgList.URL).
			Int(strtoparse, len(d.Books)).
			Msg("book bestseller list fetched")
	})
}

// runMovieCSRFAPIScraper runs the CSRF API movie scraper.
// It scrapes movies from CSRF-protected JSON APIs and returns IMDB IDs.
func runMovieCSRFAPIScraper(
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
	ctx := context.Background()

	imdbIDs, err := scraper.Scrape(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape movies: %w", err)
	}

	return imdbIDs, nil
}
