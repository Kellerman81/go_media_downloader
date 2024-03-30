package utils

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
)

// feedResults is a struct that contains the results from processing feeds
// Series contains the configuration for a TV series
// Movies contains a slice of movie IDs as strings
type feedResults struct {
	Series config.MainSerieConfig // Configuration for a TV series
	Movies []string               // Slice of movie IDs as strings
}

var (
	globalCounter = sync.Map{}
	plfeeds       = pool.NewPool(100, 0, func(b *feedResults) {
		b.Movies = make([]string, 0, 10000)
	}, func(b *feedResults) {
		clear(b.Movies)
		b.Movies = b.Movies[:0]
		clear(b.Series.Serie)
		b.Series.Serie = b.Series.Serie[:0]
		b.Series = config.MainSerieConfig{}
	})
)

// gettraktmovielist queries the Trakt API for popular, trending, or anticipated movies
// based on the usetraktmovie parameter. It returns a feedResults struct containing the
// list of movie IDs with just the imdb ID populated. This allows checking if the movie
// already exists in the database before doing more API calls to get full details.
func gettraktmovielist(usetraktmovie int, cfglist *config.MediaListsConfig) (*feedResults, error) {
	switch usetraktmovie {
	case 1:
		return processmoviesgroup(cfglist, apiexternal.GetTraktMoviePopular(cfglist.CfgList.Limit))
	case 2:
		return processmoviesgroup(cfglist, apiexternal.GetTraktMovieTrending(cfglist.CfgList.Limit))
	case 3:
		return processmoviesgroup(cfglist, apiexternal.GetTraktMovieAnticipated(cfglist.CfgList.Limit))
	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return nil, errors.New("username empty")
		}
		if cfglist.CfgList.TraktListType == "" {
			return nil, errors.New("list type empty")
		}
		data, err := apiexternal.GetTraktUserList(cfglist.CfgList.TraktUsername, cfglist.CfgList.TraktListName, cfglist.CfgList.TraktListType, cfglist.CfgList.Limit)
		if err != nil {
			return nil, err
		}
		defer clear(data)
		return processmoviesgroup(cfglist, data)
	}
	return nil, logger.Errwrongtype
}

// processmoviesgroup processes a slice of Trakt API responses for movies
// (anticipated, trending, popular, or user list) and returns a feedResults struct
// containing the movie IDs to check if the movie already exists before making
// more API calls to get full details.
func processmoviesgroup[T any](cfglist *config.MediaListsConfig, traktpopular []T) (*feedResults, error) {
	if len(traktpopular) == 0 {
		return nil, logger.Errwrongtype
	}
	d := plfeeds.Get()
	//d.Movies = make([]string, 0, len(traktpopular))
	for idx := range traktpopular {
		switch tt := any(traktpopular[idx]).(type) {
		case apiexternal.TraktUserList:
			processmovies(d, &tt.Movie, cfglist)
		case apiexternal.TraktMovieAnticipated:
			processmovies(d, &tt.Movie, cfglist)
		case apiexternal.TraktMovieTrending:
			processmovies(d, &tt.Movie, cfglist)
		case apiexternal.TraktMovie:
			processmovies(d, &tt, cfglist)
		}
	}
	return d, nil
}

// processmovies checks if a movie from the Trakt API response already exists
// in the database under the given list name. If not, it appends the IMDb ID
// to the feedResults movie list to retrieve full details later.
func processmovies(d *feedResults, row *apiexternal.TraktMovie, cfglist *config.MediaListsConfig) {
	if row.Ids.Imdb == "" {
		return
	}
	if database.GetdatarowN[int](false, "select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &row.Ids.Imdb, &cfglist.Name) == 0 {
		d.Movies = append(d.Movies, row.Ids.Imdb)
	}
}

// gettraktserielist queries the Trakt API for popular, trending, or
// anticipated TV series based on the usetraktserie parameter. It returns
// a feedResults struct containing the list of TV series with just the
// title and tvdb ID populated. This allows checking if the series already
// exists in the database before doing more API calls to get full details.
func gettraktserielist(usetraktserie int, cfglist *config.MediaListsConfig) (*feedResults, error) {
	switch usetraktserie {
	case 1:
		return processeeriesgroup(apiexternal.GetTraktSeriePopular(cfglist.CfgList.Limit))
	case 2:
		return processeeriesgroup(apiexternal.GetTraktSerieTrending(cfglist.CfgList.Limit))
	case 3:
		return processeeriesgroup(apiexternal.GetTraktSerieAnticipated(cfglist.CfgList.Limit))
	case 4:
		if cfglist.CfgList.TraktUsername == "" || cfglist.CfgList.TraktListName == "" {
			return nil, errors.New("username empty")
		}
		if cfglist.CfgList.TraktListType == "" {
			return nil, errors.New("list type empty")
		}
		traktpopular, err := apiexternal.GetTraktUserList(cfglist.CfgList.TraktUsername, cfglist.CfgList.TraktListName, cfglist.CfgList.TraktListType, cfglist.CfgList.Limit)
		if err != nil {
			return nil, err
		}
		defer clear(traktpopular)
		return processeeriesgroup(traktpopular)
	}
	return nil, logger.Errwrongtype
}

// processeeriesgroup processes a slice of Trakt API responses for TV series
// (anticipated, trending, popular, or user list) and returns a feedResults struct
// containing the series titles and tvdb IDs to check if the series already exists
// before making more API calls to get full details.
func processeeriesgroup[T any](traktpopular []T) (*feedResults, error) {
	if len(traktpopular) == 0 {
		return nil, logger.Errwrongtype
	}
	d := plfeeds.Get()
	d.Series.Serie = make([]config.SerieConfig, 0, len(traktpopular))
	for idx := range traktpopular {
		switch tt := any(traktpopular[idx]).(type) {
		case apiexternal.TraktUserList:
			processseries(d, &tt.Serie, idx)
		case apiexternal.TraktSerieAnticipated:
			processseries(d, &tt.Serie, idx)
		case apiexternal.TraktSerieTrending:
			processseries(d, &tt.Serie, idx)
		case apiexternal.TraktSerie:
			processseries(d, &tt, idx)
		}
	}
	return d, nil
}

// processseries populates the idx'th element in the feedResults series list
// with the title and tvdb ID from the TraktSerie row. It copies the data from
// the Trakt API response into the config data structure.
func processseries(d *feedResults, row *apiexternal.TraktSerie, idx int) {
	d.Series.Serie[idx] = config.SerieConfig{
		Name: row.Title, TvdbID: row.Ids.Tvdb,
	}
}

// Close cleans up resources used by feedResults if cleanup is enabled.
func (s *feedResults) Close() {
	plfeeds.Put(s)
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

	switch list.CfgList.ListType {
	case "seriesconfig":
		return getseriesconfig(list.CfgList)
	case "traktpublicshowlist":
		return gettraktserielist(4, list)
	case "imdbcsv":
		return getimdbcsv(list)
	case "traktpublicmovielist":
		return gettraktmovielist(4, list)
	case "traktmoviepopular":
		return gettraktmovielist(1, list)
	case "traktmovieanticipated":
		return gettraktmovielist(3, list)
	case "traktmovietrending":
		return gettraktmovielist(2, list)
	case "traktseriepopular":
		return gettraktserielist(1, list)
	case "traktserieanticipated":
		return gettraktserielist(3, list)
	case "traktserietrending":
		return gettraktserielist(2, list)
	case "newznabrss":
		return nil, searcher.Getnewznabrss(cfgp, list)
	}
	return nil, errors.New("switch not found")
}

// getseriesconfig loads the series config from the given file path.
// It returns a feedResults struct containing the parsed series list on success.
// Returns an error if the file could not be opened or parsed.
func getseriesconfig(cfglist *config.ListsConfig) (*feedResults, error) {
	content, err := os.Open(cfglist.SeriesConfigFile)
	if err != nil {
		return nil, errors.New("loading config")
	}
	d := plfeeds.Get()
	err = logger.ParseToml(content, &d.Series)
	content.Close()
	return d, err
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
func getimdbcsv(cfglistp *config.MediaListsConfig) (*feedResults, error) {
	resp, err := apiexternal.WebGet(cfglistp.CfgList.URL)
	if err != nil {
		return nil, errors.New("csv read")
	}
	defer resp.Body.Close()
	parserimdb := csv.NewReader(resp.Body)
	parserimdb.ReuseRecord = true

	var c int
	cf, ok := globalCounter.Load(cfglistp.CfgList.URL)
	if !ok {
		c = 100
	} else {
		c, ok = cf.(int)
		if !ok {
			c = 100
		}
	}

	listnamefilter := getlistnamefilterignore(cfglistp)

	d := plfeeds.Get()
	//d.Movies = make([]string, 0, c+10)

	var movieid uint
	var intmovie int
	var allowed bool

	args := make([]any, cfglistp.IgnoreMapListsLen+1)
	for i := range cfglistp.IgnoreMapLists {
		args[i] = &cfglistp.IgnoreMapLists[i]
	}

	var getid uint
	for {
		record, err := parserimdb.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			continue
		}
		movieid = importfeed.MovieFindDBIDByImdb(&record[1])

		if movieid != 0 {
			if getmovieid(&movieid, cfglistp) {
				continue
			}

			if cfglistp.IgnoreMapListsLen >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					intmovie = int(movieid)
					if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
						return elem.Num1 == intmovie && logger.SlicesContainsI(cfglistp.IgnoreMapLists, elem.Str)
					}) {
						continue
					}
				} else if listnamefilter != "" {
					args[cfglistp.IgnoreMapListsLen] = &movieid
					_ = database.ScanrowsNdyn(false, logger.JoinStrings("select count() from movies where ", listnamefilter, " and dbmovie_id = ?"), &getid, args...)
					if getid >= 1 {
						continue
					}
				}
			}
		} else {
			logger.LogDynamic("debug", "dbmovie not found in cache", logger.NewLogField(logger.StrImdb, &record[1]))
		}
		allowed, _ = importfeed.AllowMovieImport(record[1], cfglistp.CfgList)
		if allowed {
			d.Movies = append(d.Movies, record[1])
		}
	}
	//processimdbcsvrow(parserimdb, cfglistp, &d, listnamefilter)

	if !ok || c != len(d.Movies) {
		globalCounter.Store(cfglistp.CfgList.URL, len(d.Movies))
	}
	clear(args)

	logger.LogDynamic("info", "imdb list fetched", logger.NewLogField("url", cfglistp.CfgList.URL), logger.NewLogField("entries to parse", len(d.Movies)))
	return d, nil
}

// getmovieid checks if the given movie ID exists in the database for the specified list.
// It first checks the media cache if enabled, otherwise does a direct database query.
// Returns true if the movie ID exists in the list, false otherwise.
func getmovieid(dbid *uint, cfglistp *config.MediaListsConfig) bool {
	if dbid == nil {
		return false
	}
	if config.SettingsGeneral.UseMediaCache {
		id := int(*dbid)
		if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
			return elem.Num1 == id && strings.EqualFold(elem.Str, cfglistp.Name)
		}) {
			return true
		}
	} else if database.GetdatarowN[uint](false, database.QueryCountMoviesByDBIDList, dbid, &cfglistp.Name) >= 1 {
		return true
	}
	return false
}

// getlistnamefilterignore returns a SQL WHERE clause to filter movies
// by list name ignore lists. If the list has ignore lists configured,
// it will generate a clause to exclude movies in those lists.
// Otherwise returns empty string.
func getlistnamefilterignore(list *config.MediaListsConfig) string {
	if list.IgnoreMapListsLen >= 1 {
		return logger.JoinStrings("listname in (?", list.IgnoreMapListsQu, ") and ")
	}
	return ""
}
