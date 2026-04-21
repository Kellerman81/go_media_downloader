package utils

import (
	"context"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
)

// SQL query constant for movie lookup.
const querySelectMovieByImdb = "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"

func init() {
	// Register movie-specific functions with the mediatype handler
	// movies.RegisterImportParse(jobImportMovieParseV2)
	movies.RegisterRefresh(refreshMoviesWrapper)
	// movies.RegisterInitialFill(InitialFillMovies)
}

// Wrapper functions to match the mediatype function signatures

func refreshMoviesWrapper(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if arr, ok := data.([]string); ok {
		return refreshmovies(ctx, cfgp, arr)
	}

	return nil
}

// jobImportMovieParseV2 parses a movie file at the given path using the
// provided parser and list config. It handles adding new movies to the DB,
// updating existing movie records, and caching parsed data.
// func jobImportMovieParseV2(
// 	m *database.ParseInfo,
// 	pathv string,
// 	cfgp *config.MediaTypeConfig,
// 	list *config.MediaListsConfig,
// 	addfound bool,
// ) error {
// 	// Use unified function for the common file import logic
// 	return jobImportParseCommon(m, pathv, cfgp, list, addfound)
// }

// getdbmovieidbyimdb retrieves the database movie ID for the given movie struct by looking it up based on the IMDB ID.
// It first checks if the movie struct already has the ID populated, otherwise it calls the import job to add the movie to the database if needed.
// It also populates the local movie ID by querying the movies table based on the retrieved database movie ID.
func getdbmovieidbyimdb(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
) bool {
	if m.DbmovieID == 0 {
		var err error

		m.DbmovieID, err = importfeed.JobImportMovies(
			m.Imdb,
			cfgp,
			cfgp.GetMediaListsEntryListID(list.Name),
			true,
		)
		if err != nil || m.DbmovieID == 0 {
			m.MovieID = 0
			return true
		}
	}

	if m.MovieID == 0 {
		database.Scanrowsdyn(
			false,
			querySelectMovieByImdb,
			&m.MovieID,
			&m.Imdb,
			&list.Name,
		)

		if m.MovieID == 0 {
			return true
		}
	}

	return false
}

// importnewmoviessingle imports new movies from a feed for a single list.
// It takes a media config and list ID, gets the feed, checks/filters movies,
// and submits import jobs. It uses worker pools and caching for performance.
// func importnewmoviessingle(
// 	ctx context.Context, cfgp *config.MediaTypeConfig,
// 	list *config.MediaListsConfig,
// 	listid int,
// ) error {
// 	if err := logger.CheckContextEnded(ctx); err != nil {
// 		return err
// 	}

// 	logger.Logtype("info", 2).
// 		Str(logger.StrConfig, cfgp.NamePrefix).
// 		Str(logger.StrListname, list.Name).
// 		Msg("get feeds for")

// 	if !list.Enabled || !list.CfgList.Enabled {
// 		return logger.ErrDisabled
// 	}

// 	if list.CfgList == nil {
// 		return errors.New("list template not found")
// 	}

// 	feed, err := Feeds(cfgp, list, false)
// 	if err != nil {
// 		plfeeds.Put(feed)
// 		return err
// 	}
// 	defer plfeeds.Put(feed)

// 	if len(feed.Movies) == 0 {
// 		return nil
// 	}

// 	listnamefilter := list.Getlistnamefilterignore()

// 	pl := worker.WorkerPoolParse.NewGroupContext(ctx)

// 	var getid uint

// 	args := logger.PLArrAny.Get()
// 	defer logger.PLArrAny.Put(args)

// 	for idx := range list.IgnoreMapLists {
// 		args.Arr = append(args.Arr, &list.IgnoreMapLists[idx])
// 	}

// 	var existing []uint
// 	if !config.GetSettingsGeneral().UseMediaCache && listnamefilter != "" {
// 		existing = database.GetrowsNuncached[uint](
// 			database.Getdatarow[uint](
// 				false,
// 				logger.JoinStrings("select count() from movies where "+listnamefilter),
// 				args.Arr...,
// 			),
// 			logger.JoinStrings("select dbmovie_id from movies where "+listnamefilter),
// 			args.Arr,
// 		)
// 	}

// 	var (
// 		allowed bool
// 		movieid uint
// 	)

// 	for idx := range feed.Movies {
// 		if feed.Movies[idx] == "" {
// 			continue
// 		}

// 		if err := logger.CheckContextEnded(ctx); err != nil {
// 			return err
// 		}

// 		if !logger.HasPrefixI(feed.Movies[idx], "tt") {
// 			feed.Movies[idx] = logger.AddImdbPrefix(feed.Movies[idx])
// 		}

// 		movieid = importfeed.MovieFindDBIDByImdb(&feed.Movies[idx])

// 		if movieid != 0 {
// 			if config.GetSettingsGeneral().UseMediaCache {
// 				if database.CacheOneStringTwoIntIndexFunc(
// 					logger.CacheMovie,
// 					func(elem *syncops.DbstaticOneStringTwoInt) bool {
// 						return elem.Num1 == movieid &&
// 							(elem.Str == list.Name || strings.EqualFold(elem.Str, list.Name))
// 					},
// 				) {
// 					continue
// 				}
// 			} else if database.Scanrowsdyn(false, database.QueryCountMoviesByDBIDList, &getid, &movieid, &list.Name); getid >= 1 {
// 				continue
// 			}

// 			if list.IgnoreMapListsLen >= 1 {
// 				if config.GetSettingsGeneral().UseMediaCache {
// 					if database.CacheOneStringTwoIntIndexFunc(
// 						logger.CacheMovie,
// 						func(elem *syncops.DbstaticOneStringTwoInt) bool {
// 							return elem.Num1 == movieid &&
// 								logger.SlicesContainsI(list.IgnoreMapLists, elem.Str)
// 						},
// 					) {
// 						continue
// 					}
// 				} else {
// 					if slices.Contains(existing, movieid) {
// 						continue
// 					}
// 				}
// 			}
// 		}

// 		allowed, _ = importfeed.AllowMovieImport(&feed.Movies[idx], list.CfgList)
// 		if allowed {
// 			pl.Submit(func() {
// 				defer logger.HandlePanic()

// 				importfeed.JobImportMoviesByList(ctx, feed.Movies[idx], idx, cfgp, listid, true)
// 			})
// 		} else {
// 			logger.Logtype("debug", 1).
// 				Str(logger.StrImdb, feed.Movies[idx]).
// 				Msg("not allowed movie")
// 		}
// 	}

// 	errjobs := pl.Wait()
// 	if errjobs != nil {
// 		logger.Logtype("error", 0).
// 			Err(errjobs).
// 			Msg("Error importing movies")
// 	}

// 	return nil
// }

// RefreshMovie refreshes the data for the given movie by looking up its ID and calling refreshmovies.
// It takes the media config and the movie ID string.
// It converts the ID to an int and calls refreshmovies to refresh that single movie.
func RefreshMovie(cfgp *config.MediaTypeConfig, id *string) error {
	return refreshmovies(
		context.Background(),
		cfgp,
		database.GetrowsN[string](
			false,
			1,
			"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?",
			id,
		),
	)
}

// refreshmovies refreshes movie data for the given movies. It takes a media config, count of movies to refresh, a query to get the movie IDs, and an optional parameter for the query. It gets the list of movie IDs to refresh, logs info for each, looks up the list name, and calls the import job. Any errors are logged.
func refreshmovies(ctx context.Context, cfgp *config.MediaTypeConfig, arr []string) error {
	if len(arr) == 0 {
		return nil
	}

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		logger.Logtype("info", 1).
			Str(logger.StrImdb, arr[idx]).
			Msg("Refresh Movie")

		errsub := importfeed.JobImportMoviesByList(
			ctx, arr[idx],
			idx,
			cfgp,
			getrefreshlistid(&arr[idx], cfgp),
			false,
		)
		if errsub != nil {
			err = errsub
		}
	}

	return err
}

// getrefreshlistid looks up the list ID for the given IMDB ID and media config.
// It first finds the database ID for the movie using the IMDB ID, then looks up
// the list name associated with that movie ID, and finally retrieves the list ID
// for that list name from the media config. If any of these steps fail, it returns -1.
func getrefreshlistid(imdb *string, cfgp *config.MediaTypeConfig) int {
	movieid := importfeed.MovieFindDBIDByImdb(imdb)
	if movieid == 0 {
		return -1
	}

	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM movies where dbmovie_id = ?",
		&movieid,
	)
	if listname == "" {
		return -1
	}

	return cfgp.GetMediaListsEntryListID(listname)
}
