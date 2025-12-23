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
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/csrfapi"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/htmlxpath"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/pelletier/go-toml/v2"
)

// feedResults is a struct that contains the results from processing feeds
// Series contains the configuration for a TV series
// Movies contains a slice of movie IDs as strings.
type feedResults struct {
	Series []config.SerieConfig // Configuration for a TV series
	Movies []string             // Slice of movie IDs as strings
	AddAll bool
}

var (
	errwrongtype     = errors.New("wrong type")
	errcsvread       = errors.New("csv read")
	errusernameempty = errors.New("username empty")
	errlistnameempty = errors.New("list type empty")
	strtoparse       = "entries to parse"
	plfeeds          pool.Poolobj[feedResults]
)

// Init initializes the plfeeds pool object, which is used to manage the feedResults
// struct. It sets the initial capacity for the Movies and Series slices, and
// provides a cleanup function to clear the slices when they are reused.
func Init() {
	plfeeds.Init(200, 5, func(b *feedResults) {
		b.Movies = make([]string, 0, 10000)
		b.Series = make([]config.SerieConfig, 0, 1000)
	}, func(b *feedResults) bool {
		if len(b.Movies) > 0 {
			clear(b.Movies)

			b.Movies = b.Movies[:0]
		}

		if len(b.Series) > 0 {
			clear(b.Series)

			b.Series = b.Series[:0]
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
	switch usetraktmovie {
	case 1:
		for _, arr := range apiexternal.GetTraktMoviePopular(&cfglist.CfgList.Limit) {
			checkaddimdbfeed(&arr.IDs.Imdb, cfglist, d)
		}
		return nil

	case 2:
		for _, arr := range apiexternal.GetTraktMovieTrending(&cfglist.CfgList.Limit) {
			checkaddimdbfeed(&arr.Movie.IDs.Imdb, cfglist, d)
		}
		return nil

	case 3:
		for _, arr := range apiexternal.GetTraktMovieAnticipated(&cfglist.CfgList.Limit) {
			checkaddimdbfeed(&arr.Movie.IDs.Imdb, cfglist, d)
		}
		return nil

	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return errusernameempty
		}

		if cfglist.CfgList.TraktListType == "" {
			return errlistnameempty
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

		for idx := range arr {
			if checkaddimdbfeed(&arr[idx].Movie.IDs.Imdb, cfglist, d) {
				if cfglist.CfgList.RemoveFromList {
					apiexternal.RemoveMovieFromTraktUserList(
						cfglist.CfgList.TraktUsername,
						cfglist.CfgList.TraktListName,
						arr[idx].Movie.IDs.Imdb,
					)
				}
			}
		}

		return nil
	}

	return errwrongtype
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

	for idx := range cfglist.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglist.IgnoreMapLists[idx])
	}

	var existing []uint
	if !config.GetSettingsGeneral().UseMediaCache && listnamefilter != "" {
		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](
				false,
				logger.JoinStrings("select count() from movies where ", listnamefilter),
				args.Arr...,
			),
			logger.JoinStrings("select dbmovie_id from movies where ", listnamefilter),
			args.Arr,
		)
	}

	var (
		movieid uint
		allowed bool
	)

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		arr, err := apiexternal.DiscoverTmdbMovie(cfglist.CfgList.TmdbDiscover[idxdiscover])
		if err != nil {
			logger.Logtype("debug", 1).
				Str("query", cfglist.CfgList.TmdbDiscover[idxdiscover]).
				Err(err).
				Msg("discover could not be executed")

			continue
		}

		for idx := range arr.Results {
			movieid = importfeed.MovieFindDBIDByTmdbID(&arr.Results[idx].ID)
			if movieid != 0 {
				if getmovieid(&movieid, cfglist) && !d.AddAll {
					continue
				}

				if cfglist.IgnoreMapListsLen >= 1 {
					if config.GetSettingsGeneral().UseMediaCache {
						if database.CacheOneStringTwoIntIndexFunc(
							logger.CacheMovie,
							func(elem *syncops.DbstaticOneStringTwoInt) bool {
								return elem.Num1 == movieid &&
									logger.SlicesContainsI(cfglist.IgnoreMapLists, elem.Str)
							},
						) {
							continue
						}
					} else if listnamefilter != "" {
						if slices.Contains(existing, movieid) {
							continue
						}
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

	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
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
				moviedbexternal, err = apiexternal.GetTVExternal(arr.Results[idx].ID)
				if err != nil || moviedbexternal == nil || moviedbexternal.TvdbID == 0 {
					continue
				}

				d.Series = append(d.Series, config.SerieConfig{
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

	for idxdiscover := range cfglist.CfgList.TmdbList {
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

			if imdbid == "" || d.AddAll {
				moviedbexternal, err = apiexternal.GetTVExternal(arr.Items[idx].ID)
				if err != nil || moviedbexternal == nil || moviedbexternal.TvdbID == 0 {
					continue
				}

				d.Series = append(d.Series, config.SerieConfig{
					Name: arr.Items[idx].Name, TvdbID: moviedbexternal.TvdbID,
				})
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

	for idx := range cfglist.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglist.IgnoreMapLists[idx])
	}

	var existing []uint
	if !config.GetSettingsGeneral().UseMediaCache && listnamefilter != "" {
		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](
				false,
				logger.JoinStrings("select count() from movies where ", listnamefilter),
				args.Arr...,
			),
			logger.JoinStrings("select dbmovie_id from movies where ", listnamefilter),
			args.Arr,
		)
	}

	var (
		movieid uint
		allowed bool
	)

	for idxlist := range cfglist.CfgList.TmdbList {
		arr, err := apiexternal.GetTmdbList(cfglist.CfgList.TmdbList[idxlist])
		if err != nil {
			continue
		}

		for idx := range arr.Items {
			movieid = importfeed.MovieFindDBIDByTmdbID(&arr.Items[idx].ID)
			if movieid != 0 {
				if getmovieid(&movieid, cfglist) && !d.AddAll {
					if cfglist.CfgList.RemoveFromList {
						apiexternal.RemoveFromTmdbList(
							cfglist.CfgList.TmdbList[idxlist],
							arr.Items[idx].ID,
						)
					}

					continue
				}

				if cfglist.IgnoreMapListsLen >= 1 {
					if config.GetSettingsGeneral().UseMediaCache {
						if database.CacheOneStringTwoIntIndexFunc(
							logger.CacheMovie,
							func(elem *syncops.DbstaticOneStringTwoInt) bool {
								return elem.Num1 == movieid &&
									logger.SlicesContainsI(cfglist.IgnoreMapLists, elem.Str)
							},
						) {
							if cfglist.CfgList.RemoveFromList {
								apiexternal.RemoveFromTmdbList(
									cfglist.CfgList.TmdbList[idxlist],
									arr.Items[idx].ID,
								)
							}

							continue
						}
					} else if listnamefilter != "" {
						if slices.Contains(existing, movieid) {
							if cfglist.CfgList.RemoveFromList {
								apiexternal.RemoveFromTmdbList(cfglist.CfgList.TmdbList[idxlist], arr.Items[idx].ID)
							}
							continue
						}
					}
				}
			}

			moviedbexternal, err := apiexternal.GetTmdbMovieExternal(arr.Items[idx].ID)
			if err != nil {
				logger.Logtype("debug", 1).
					Int(logger.StrImdb, arr.Items[idx].ID).
					Err(err).
					Msg("imdb id could not be retrieved")

				continue
			}

			allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
			if allowed || d.AddAll {
				d.Movies = append(d.Movies, moviedbexternal.ImdbID)

				if cfglist.CfgList.RemoveFromList {
					apiexternal.RemoveFromTmdbList(
						cfglist.CfgList.TmdbList[idxlist],
						arr.Items[idx].ID,
					)
				}
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
	switch usetraktserie {
	case 1:
		for _, arr := range apiexternal.GetTraktSeriePopular(&cfglist.CfgList.Limit) {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr.Title, TvdbID: arr.IDs.Tvdb,
			})
		}

		return nil

	case 2:
		for _, arr := range apiexternal.GetTraktSerieTrending(&cfglist.CfgList.Limit) {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr.Serie.Title, TvdbID: arr.Serie.IDs.Tvdb,
			})
		}

		return nil

	case 3:
		for _, arr := range apiexternal.GetTraktSerieAnticipated(&cfglist.CfgList.Limit) {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr.Serie.Title, TvdbID: arr.Serie.IDs.Tvdb,
			})
		}

		return nil

	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return errusernameempty
		}

		if cfglist.CfgList.TraktListType == "" {
			return errlistnameempty
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

		for idx := range arr {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr[idx].Serie.Title, TvdbID: arr[idx].Serie.IDs.Tvdb,
			})
			if cfglist.CfgList.RemoveFromList {
				apiexternal.RemoveSerieFromTraktUserList(
					cfglist.CfgList.TraktUsername,
					cfglist.CfgList.TraktListName,
					arr[idx].Movie.IDs.Tvdb,
				)
			}
		}

		return nil
	}

	return errwrongtype
}

// getseriesconfig loads the series config from the given file path.
// It returns a feedResults struct containing the parsed series list on success.
// Returns an error if the file could not be opened or parsed.
func getseriesconfig(cfglist *config.ListsConfig) ([]config.SerieConfig, error) {
	content, err := os.Open(cfglist.SeriesConfigFile)
	if err != nil {
		return nil, errors.New("loading config")
	}
	defer content.Close()

	var s config.MainSerieConfig

	err = toml.NewDecoder(content).Decode(&s)

	return s.Serie, err
}

// getimdbcsv loads an IMDB CSV list from a URL, parses it, filters movies,
// and returns a feedResults struct containing the allowed movie IDs.
//
// It uses a csv.Reader to parse the CSV data from the URL.
// It iterates through each row, getting the movie ID, checking if it already
// exists for this list, applying any configured filters, and adding allowed
// movies to the results list.
//
// It handles locking/unlocking a global counter for tracking list item counts.
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
//
// It handles locking/unlocking a global counter for tracking list item counts.
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

	for idx := range cfglistp.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglistp.IgnoreMapLists[idx])
	}

	var existing []uint
	if !config.GetSettingsGeneral().UseMediaCache && listnamefilter != "" {
		existing = database.GetrowsNuncached[uint](
			database.Getdatarow[uint](
				false,
				logger.JoinStrings("select count() from movies where ", listnamefilter),
				args.Arr...,
			),
			logger.JoinStrings("select dbmovie_id from movies where ", listnamefilter),
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
		// if logger.CheckContextEnded(ctx) != nil {
		// 	break
		// }
		if len(record) < 2 || record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}

		record[1] = logger.AddImdbPrefix(record[1])
		movieid = importfeed.MovieFindDBIDByImdb(&record[1])

		if movieid != 0 {
			if getmovieid(&movieid, cfglistp) && !d.AddAll {
				continue
			}

			if cfglistp.IgnoreMapListsLen >= 1 {
				if config.GetSettingsGeneral().UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(
						logger.CacheMovie,
						func(elem *syncops.DbstaticOneStringTwoInt) bool {
							return elem.Num1 == movieid &&
								logger.SlicesContainsI(cfglistp.IgnoreMapLists, elem.Str)
						},
					) {
						continue
					}
				} else if listnamefilter != "" {
					if slices.Contains(existing, movieid) {
						continue
					}
				}
			}
		} else {
			logger.Logtype("debug", 1).Str(logger.StrImdb, record[1]).Msg("dbmovie not found in cache")
		}

		allowed, _ = importfeed.AllowMovieImport(&record[1], cfglistp.CfgList)
		if allowed || d.AddAll {
			d.Movies = append(d.Movies, record[1])
		}
	}

	i := len(d.Movies)
	logger.Logtype("info", 2).
		Str(logger.StrURL, cfglistp.CfgList.URL).
		Any(strtoparse, &i).
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

// feeds fetches the configured media list for the given config and list ID.
// It handles looking up the correct list type and calling the appropriate
// handler function. Returns a feedResults struct containing the parsed list
// items on success, or an error if the list could not be fetched or parsed.
func Feeds(
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
		// TODO: tmdbp private / public lists - tmdb popular / upcoming movies
		// TODO: trakt list cleanup
	case "newznabrss":
		return d, searcher.Getnewznabrss(cfgp, list)
	case "plexwatchlist":
		return d, d.getplexwatchlist(list)
	case "jellyfinwatchlist":
		return d, d.getjellyfinwatchlist(list)
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
		return errors.New("plex server URL, token, and username are required")
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

	for _, item := range watchlistItems {
		if apiexternal.IsPlexItemMovie(item) {
			// Process movie
			imdbID := apiexternal.ExtractIMDBFromPlexItem(item)
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).Str("title", item.Title).Msg("No IMDB ID found for Plex movie")
			}
		} else if apiexternal.IsPlexItemShow(item) {
			// Process TV show
			tvdbID := apiexternal.ExtractTVDBFromPlexItem(item)
			if tvdbID != 0 {
				d.Series = append(d.Series, config.SerieConfig{
					Name:   item.Title,
					TvdbID: tvdbID,
				})
			} else {
				logger.Logtype("debug", 1).Str("title", item.Title).Msg("No TVDB ID found for Plex show")
			}
		}
	}

	return nil
}

// getjellyfinwatchlist retrieves watchlist items from a Jellyfin Media Server.
func (d *feedResults) getjellyfinwatchlist(cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.JellyfinServerURL == "" || cfglist.CfgList.JellyfinToken == "" ||
		cfglist.CfgList.JellyfinUsername == "" {
		return errors.New("jellyfin server URL, API key, and username are required")
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

	for _, item := range watchlistItems {
		if apiexternal.IsJellyfinItemMovie(item) {
			// Process movie
			imdbID := apiexternal.ExtractIMDBFromJellyfinItem(item)
			if imdbID != "" {
				checkaddimdbfeed(&imdbID, cfglist, d)
			} else {
				logger.Logtype("debug", 1).Str("title", apiexternal.GetJellyfinItemTitle(item)).Msg("No IMDB ID found for Jellyfin movie")
			}
		} else if apiexternal.IsJellyfinItemSeries(item) {
			// Process TV series
			tvdbID := apiexternal.ExtractTVDBFromJellyfinItem(item)
			if tvdbID != 0 {
				d.Series = append(d.Series, config.SerieConfig{
					Name:   apiexternal.GetJellyfinItemTitle(item),
					TvdbID: tvdbID,
				})
			} else {
				logger.Logtype("debug", 1).Str("title", apiexternal.GetJellyfinItemTitle(item)).Msg("No TVDB ID found for Jellyfin series")
			}
		}
	}

	return nil
}

// getmoviescraper runs a movie scraper to extract and import movies from external sources.
// It validates the scraper configuration, runs the appropriate scraper type (HTML/XPath or CSRF API),
// and populates d.Movies with IMDB IDs from the scraped movies.
func (d *feedResults) getmoviescraper(cfgp *config.MediaTypeConfig, cfglist *config.MediaListsConfig) error {
	if cfglist.CfgList.MovieScraperType == "" {
		return errors.New("movie_scraper_type is required for moviescraper list type")
	}

	if cfglist.CfgList.MovieScraperStartURL == "" {
		return errors.New("movie_scraper_start_url is required for moviescraper list type")
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
func runMovieScraper(cfgp *config.MediaTypeConfig, cfglist *config.MediaListsConfig) ([]string, error) {
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
		return nil, fmt.Errorf("unsupported movie scraper type: %s", cfglist.CfgList.MovieScraperType)
	}

	return imdbIDs, nil
}

// runMovieHTMLXPathScraper runs the HTML/XPath movie scraper.
// It scrapes movies from HTML pages using XPath selectors and returns IMDB IDs.
func runMovieHTMLXPathScraper(cfgp *config.MediaTypeConfig, cfglist *config.MediaListsConfig) ([]string, error) {
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

// runMovieCSRFAPIScraper runs the CSRF API movie scraper.
// It scrapes movies from CSRF-protected JSON APIs and returns IMDB IDs.
func runMovieCSRFAPIScraper(cfgp *config.MediaTypeConfig, cfglist *config.MediaListsConfig) ([]string, error) {
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
