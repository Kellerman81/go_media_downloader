package utils

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"os"
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
// Movies contains a slice of movie IDs as strings
type feedResults struct {
	Series []config.SerieConfig // Configuration for a TV series
	Movies []string             // Slice of movie IDs as strings
}

var (
	errwrongtype = errors.New("wrong type")
	plfeeds      = pool.NewPool(100, 5, func(b *feedResults) {
		b.Movies = make([]string, 0, 10000)
		b.Series = make([]config.SerieConfig, 0, 1000)
	}, func(b *feedResults) {
		clear(b.Movies)
		b.Movies = b.Movies[:0]
		clear(b.Series)
		b.Series = b.Series[:0]
	})
)

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
	var checkid int
	switch usetraktmovie {
	case 1:
		arr, _ := apiexternal.GetTraktMoviePopular(&cfglist.CfgList.Limit)
		for idx := range arr {
			if arr[idx].Ids.Imdb == "" {
				continue
			}
			database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &checkid, &arr[idx].Ids.Imdb, &cfglist.Name)
			if checkid == 0 {
				d.Movies = append(d.Movies, arr[idx].Ids.Imdb)
			}
		}
		//clear(arr)
		return nil
	case 2:
		arr, _ := apiexternal.GetTraktMovieTrending(&cfglist.CfgList.Limit)
		for idx := range arr {
			if arr[idx].Movie.Ids.Imdb == "" {
				continue
			}
			database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &checkid, &arr[idx].Movie.Ids.Imdb, &cfglist.Name)
			if checkid == 0 {
				//if database.GetdatarowN[int](false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &arr[idx].Movie.Ids.Imdb, &cfglist.Name) == 0 {
				d.Movies = append(d.Movies, arr[idx].Movie.Ids.Imdb)
			}
		}
		//clear(arr)
		return nil
	case 3:
		arr, _ := apiexternal.GetTraktMovieAnticipated(&cfglist.CfgList.Limit)
		for idx := range arr {
			if arr[idx].Movie.Ids.Imdb == "" {
				continue
			}
			database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &checkid, &arr[idx].Movie.Ids.Imdb, &cfglist.Name)
			if checkid == 0 {
				//if database.GetdatarowN[int](false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &arr[idx].Movie.Ids.Imdb, &cfglist.Name) == 0 {
				d.Movies = append(d.Movies, arr[idx].Movie.Ids.Imdb)
			}
		}
		//clear(arr)
		return nil
	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return errors.New("username empty")
		}
		if cfglist.CfgList.TraktListType == "" {
			return errors.New("list type empty")
		}
		arr, err := apiexternal.GetTraktUserList(cfglist.CfgList.TraktUsername, cfglist.CfgList.TraktListName, cfglist.CfgList.TraktListType, &cfglist.CfgList.Limit)
		if err != nil {
			return err
		}
		for idx := range arr {
			if arr[idx].Movie.Ids.Imdb == "" {
				continue
			}
			database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &checkid, &arr[idx].Movie.Ids.Imdb, &cfglist.Name)
			if checkid == 0 {
				//if database.GetdatarowN[int](false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &arr[idx].Movie.Ids.Imdb, &cfglist.Name) == 0 {
				d.Movies = append(d.Movies, arr[idx].Movie.Ids.Imdb)
			}
		}
		//clear(arr)
		return nil
	}
	return errwrongtype
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
		arr, _ := apiexternal.GetTraktSeriePopular(&cfglist.CfgList.Limit)
		for idx := range arr {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr[idx].Title, TvdbID: arr[idx].Ids.Tvdb,
			})
		}
		//clear(arr)
		return nil
	case 2:
		arr, _ := apiexternal.GetTraktSerieTrending(&cfglist.CfgList.Limit)
		for idx := range arr {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr[idx].Serie.Title, TvdbID: arr[idx].Serie.Ids.Tvdb,
			})
		}
		//clear(arr)
		return nil
	case 3:
		arr, _ := apiexternal.GetTraktSerieAnticipated(&cfglist.CfgList.Limit)
		for idx := range arr {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr[idx].Serie.Title, TvdbID: arr[idx].Serie.Ids.Tvdb,
			})
		}
		//clear(arr)
		return nil
	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return errors.New("username empty")
		}
		if cfglist.CfgList.TraktListType == "" {
			return errors.New("list type empty")
		}
		arr, err := apiexternal.GetTraktUserList(cfglist.CfgList.TraktUsername, cfglist.CfgList.TraktListName, cfglist.CfgList.TraktListType, &cfglist.CfgList.Limit)
		if err != nil {
			return err
		}
		for idx := range arr {
			d.Series = append(d.Series, config.SerieConfig{
				Name: arr[idx].Serie.Title, TvdbID: arr[idx].Serie.Ids.Tvdb,
			})
		}
		//clear(arr)
		return nil
	}
	return errwrongtype
}

// Close clears the Movies and Series slices and returns the feedResults instance to the plfeeds pool.
func (d *feedResults) Close() {
	if d == nil {
		return
	}
	plfeeds.Put(d)
}

// getseriesconfig loads the series config from the given file path.
// It returns a feedResults struct containing the parsed series list on success.
// Returns an error if the file could not be opened or parsed.
func (d *feedResults) getseriesconfig(cfglist *config.ListsConfig) ([]config.SerieConfig, error) {
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
	cl := apiexternal.GetCl()
	ctx, ctxcancel := context.WithTimeout(cl.Ctx, cl.Timeout*5)
	defer ctxcancel()

	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}
	resp, err := cl.Getdo(ctx, cfglistp.CfgList.URL, false, nil)
	//resp, err := apiexternal.WebGet(cfglistp.CfgList.URL)
	if err != nil {
		return errors.New("csv read")
	}
	defer resp.Close()
	parserimdb := csv.NewReader(resp)
	parserimdb.ReuseRecord = true

	listnamefilter := cfglistp.Getlistnamefilterignore()

	var allowed bool

	//args := make([]any, 0, len(cfglistp.IgnoreMapLists)+1)
	args := logger.PLArrAny.Get()
	for idx := range cfglistp.IgnoreMapLists {
		args.Arr = append(args.Arr, &cfglistp.IgnoreMapLists[idx])
	}
	args.Arr = append(args.Arr, &logger.V0)

	var movieid uint
	var record []string
	if err := logger.CheckContextEnded(ctx); err != nil {
		parserimdb = nil
		return err
	}
	var checkid int
	for {
		record, err = parserimdb.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}
		movieid = importfeed.MovieFindDBIDByImdb(record[1])

		if movieid != 0 {
			if getmovieid(&movieid, cfglistp) {
				continue
			}

			if cfglistp.IgnoreMapListsLen >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
						return elem.Num1 == movieid && logger.SlicesContainsI(cfglistp.IgnoreMapLists, elem.Str)
					}) {
						continue
					}
				} else if listnamefilter != "" {
					args.Arr[cfglistp.IgnoreMapListsLen] = &movieid
					database.ScanrowsNArr(false, logger.JoinStrings("select count() from movies where ", listnamefilter, " and dbmovie_id = ?"), &checkid, args.Arr)
					if checkid >= 1 {
						continue
					}
				}
			}
		} else {
			logger.LogDynamicany("debug", "dbmovie not found in cache", &logger.StrImdb, &record[1])
		}
		allowed, _ = importfeed.AllowMovieImport(record[1], cfglistp.CfgList)
		if allowed {
			d.Movies = append(d.Movies, record[1])
		}
	}
	logger.PLArrAny.Put(args)
	//args = nil
	parserimdb = nil
	logger.LogDynamicany("info", "imdb list fetched", &logger.StrURL, &cfglistp.CfgList.URL, "entries to parse", len(d.Movies))
	return nil
}

// feeds fetches the configured media list for the given config and list ID.
// It handles looking up the correct list type and calling the appropriate
// handler function. Returns a feedResults struct containing the parsed list
// items on success, or an error if the list could not be fetched or parsed.
func feeds(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig) (*feedResults, error) {
	if !list.Enabled {
		return nil, logger.ErrDisabled
	}
	if list.CfgList == nil {
		return nil, errors.New("list template not found")
	}

	if !list.CfgList.Enabled {
		return nil, logger.ErrDisabled
	}

	d := plfeeds.Get()
	var err error
	switch list.CfgList.ListType {
	case "seriesconfig":
		d.Series, err = d.getseriesconfig(list.CfgList)
	case "traktpublicshowlist":
		err = d.gettraktserielist(4, list)
	case "imdbcsv":
		err = d.getimdbcsv(list)
	case "traktpublicmovielist":
		err = d.gettraktmovielist(4, list)
	case "traktmoviepopular":
		err = d.gettraktmovielist(1, list)
	case "traktmovieanticipated":
		err = d.gettraktmovielist(3, list)
	case "traktmovietrending":
		err = d.gettraktmovielist(2, list)
	case "traktseriepopular":
		err = d.gettraktserielist(1, list)
	case "traktserieanticipated":
		err = d.gettraktserielist(3, list)
	case "traktserietrending":
		err = d.gettraktserielist(2, list)
	case "newznabrss":
		err = searcher.Getnewznabrss(cfgp, list)
	default:
		err = errors.New("switch not found")
	}
	if err != nil {
		d.Close()
	}
	return d, err
}

// getmovieid checks if the given movie ID exists in the database for the specified list.
// It first checks the media cache if enabled, otherwise does a direct database query.
// Returns true if the movie ID exists in the list, false otherwise.
func getmovieid(dbid *uint, cfglistp *config.MediaListsConfig) bool {
	if dbid == nil || *dbid == 0 {
		return false
	}
	if config.SettingsGeneral.UseMediaCache {
		if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
			return elem.Num1 == *dbid && (elem.Str == cfglistp.Name || strings.EqualFold(elem.Str, cfglistp.Name))
		}) {
			return true
		}
	} else if database.GetdatarowN[uint](false, database.QueryCountMoviesByDBIDList, dbid, &cfglistp.Name) >= 1 {
		return true
	}
	return false
}
