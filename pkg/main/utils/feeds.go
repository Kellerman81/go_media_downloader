package utils

import (
	"context"
	"encoding/csv"
	"errors"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/pelletier/go-toml/v2"
)

// feedResults is a struct that contains the results from processing feeds
// Series contains the configuration for a TV series
// Movies contains a slice of movie IDs as strings.
type feedResults struct {
	Series []config.SerieConfig // Configuration for a TV series
	Movies []string             // Slice of movie IDs as strings
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
	plfeeds.Init(5, func(b *feedResults) {
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
	var movieid uint
	var allowed bool
	for idxdiscover := range cfglist.CfgList.TmdbDiscover {
		arr, err := apiexternal.DiscoverTmdbMovie(cfglist.CfgList.TmdbDiscover[idxdiscover])
		if err != nil {
			logger.LogDynamicany1StringErr(
				"debug",
				"discover could not be executed",
				err,
				"query",
				cfglist.CfgList.TmdbDiscover[idxdiscover],
			)
			continue
		}
		for idx := range arr.Results {
			movieid = importfeed.MovieFindDBIDByTmdbID(&arr.Results[idx].ID)
			if movieid != 0 {
				if getmovieid(&movieid, cfglist) {
					continue
				}

				if cfglist.IgnoreMapListsLen >= 1 {
					if config.GetSettingsGeneral().UseMediaCache {
						if database.CacheOneStringTwoIntIndexFunc(
							logger.CacheMovie,
							func(elem *database.DbstaticOneStringTwoInt) bool {
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
				logger.LogDynamicany1IntErr(
					"debug",
					"imdb id could not be retrieved",
					err,
					logger.StrImdb,
					arr.Results[idx].ID,
				)
				continue
			}
			allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
			if allowed {
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
	var imdbid string
	var moviedbexternal *apiexternal.TheMovieDBTVExternal
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
			if imdbid == "" {
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
	var imdbid string
	var moviedbexternal *apiexternal.TheMovieDBTVExternal
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
			if imdbid == "" {
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
	) == 0 {
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
	var movieid uint
	var allowed bool
	for idxlist := range cfglist.CfgList.TmdbList {
		arr, err := apiexternal.GetTmdbList(cfglist.CfgList.TmdbList[idxlist])
		if err != nil {
			continue
		}

		for idx := range arr.Items {
			movieid = importfeed.MovieFindDBIDByTmdbID(&arr.Items[idx].ID)
			if movieid != 0 {
				if getmovieid(&movieid, cfglist) {
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
							func(elem *database.DbstaticOneStringTwoInt) bool {
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
				logger.LogDynamicany1IntErr(
					"debug",
					"imdb id could not be retrieved",
					err,
					logger.StrImdb,
					arr.Items[idx].ID,
				)
				continue
			}
			allowed, _ = importfeed.AllowMovieImport(&moviedbexternal.ImdbID, cfglist.CfgList)
			if allowed {
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

	var allowed bool
	var movieid uint
	for {
		record, err := parserimdb.Read()
		if err != nil {
			break
		}
		// if logger.CheckContextEnded(ctx) != nil {
		// 	break
		// }
		if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}
		record[1] = logger.AddImdbPrefix(record[1])
		movieid = importfeed.MovieFindDBIDByImdb(&record[1])

		if movieid != 0 {
			if getmovieid(&movieid, cfglistp) {
				continue
			}

			if cfglistp.IgnoreMapListsLen >= 1 {
				if config.GetSettingsGeneral().UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(
						logger.CacheMovie,
						func(elem *database.DbstaticOneStringTwoInt) bool {
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
			logger.LogDynamicany1String("debug", "dbmovie not found in cache", logger.StrImdb, record[1])
		}
		allowed, _ = importfeed.AllowMovieImport(&record[1], cfglistp.CfgList)
		if allowed {
			d.Movies = append(d.Movies, record[1])
		}
	}
	i := len(d.Movies)
	logger.LogDynamicany2StrAny(
		"info",
		"imdb list fetched",
		logger.StrURL,
		cfglistp.CfgList.URL,
		strtoparse,
		&i,
	)
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
func feeds(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, d *feedResults) error {
	switch list.CfgList.ListType {
	case "seriesconfig":
		var err error
		d.Series, err = getseriesconfig(list.CfgList)
		return err
	case "traktpublicshowlist":
		return d.gettraktserielist(4, list)
	case "imdbcsv":
		return d.getimdbcsv(list)
	case "imdbfile":
		return d.getimdbfile(list)
	case "tmdblist":
		return d.gettmdblist(list)
	case "tmdbshowlist":
		return d.gettmdbshowlist(list)
	case "traktpublicmovielist":
		return d.gettraktmovielist(4, list)
	case "traktmoviepopular":
		return d.gettraktmovielist(1, list)
	case "traktmovieanticipated":
		return d.gettraktmovielist(3, list)
	case "traktmovietrending":
		return d.gettraktmovielist(2, list)
	case "traktseriepopular":
		return d.gettraktserielist(1, list)
	case "traktserieanticipated":
		return d.gettraktserielist(3, list)
	case "traktserietrending":
		return d.gettraktserielist(2, list)
	case "tmdbmoviediscover":
		return d.gettmdbmoviediscover(list)
	case "tmdbshowdiscover":
		return d.gettmdbshowdiscover(list)
		// TODO: tmdbp private / public lists - tmdb popular / upcoming movies
		// TODO: trakt list cleanup
	case "newznabrss":
		return searcher.Getnewznabrss(cfgp, list)
	}
	return errors.New("switch not found " + list.CfgList.ListType)
}

// getmovieid checks if the given movie ID exists in the database for the specified list.
// It first checks the media cache if enabled, otherwise does a direct database query.
// Returns true if the movie ID exists in the list, false otherwise.
func getmovieid(dbid *uint, cfglistp *config.MediaListsConfig) bool {
	if config.GetSettingsGeneral().UseMediaCache {
		dbidn := *dbid
		if database.CacheOneStringTwoIntIndexFunc(
			logger.CacheMovie,
			func(elem *database.DbstaticOneStringTwoInt) bool {
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
