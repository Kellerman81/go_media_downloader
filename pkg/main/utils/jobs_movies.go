package utils

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// jobImportMovieParseV2 parses a movie file at the given path using the
// provided parser and list config. It handles adding new movies to the DB,
// updating existing movie records, and caching parsed data.
func jobImportMovieParseV2(
	m *database.ParseInfo,
	pathv string,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	addfound bool,
) error {
	if m == nil {
		return logger.ErrNotFound
	}
	if list.CfgQuality == nil {
		return errors.New("quality template not found")
	}

	if m.MovieID == 0 && addfound {
		if m.Imdb != "" {
			if getdbmovieidbyimdb(m, cfgp, list) {
				return logger.ErrNotFoundMovie
			}
		}
		if m.Imdb == "" {
			importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m, cfgp)
		}

		var bl bool
		if m.Imdb != "" {
			m.MovieFindDBIDByImdbParser()
			if m.DbmovieID != 0 {
				bl = true
			}
		}
		if bl && list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
			database.Scanrows2dyn(
				false,
				database.QueryMoviesGetIDByDBIDListname,
				&m.MovieID,
				&m.DbmovieID,
				&list.Name,
			)
			if m.MovieID == 0 {
				if m.Imdb == "" {
					if config.SettingsGeneral.UseMediaCache {
						m.CacheThreeStringIntIndexFuncGetImdb()
					} else {
						database.Scanrows1dyn(false, "select imdb_id from dbmovies where id = ?", &m.Imdb, &m.DbmovieID)
					}
				}
				if m.Imdb != "" {
					if getdbmovieidbyimdb(m, cfgp, list) {
						return logger.ErrNotFoundMovie
					}
				}
			}
		} else if list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
			importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m, cfgp)
			var err error
			if m.DbmovieID == 0 {
				m.DbmovieID, err = importfeed.JobImportMovies(m.Imdb, cfgp, cfgp.GetMediaListsEntryListID(list.Name), true)
			}
			if err != nil && m.MovieID == 0 {
				database.Scanrows2dyn(false, database.QueryMoviesGetIDByDBIDListname, &m.MovieID, &m.DbmovieID, &list.Name)

				if m.MovieID == 0 {
					err = logger.ErrNotFoundMovie
				}
			}

			if err != nil {
				return err
			}
		}
	}

	if m.MovieID == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name, logger.ErrNotFoundMovie)
		return logger.ErrNotFoundMovie
	}

	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)
	m.File = pathv
	err := parser.ParseVideoFile(m, list.CfgQuality)
	if err != nil {
		return err
	}
	var i int
	if m.Priority >= list.CfgQuality.CutoffPriority {
		i = 1
	}
	m.TempTitle = pathv
	base := filepath.Base(pathv)
	ext := filepath.Ext(pathv)

	if m.MovieID != 0 &&
		database.Getdatarow1[string](
			false,
			"select rootpath from movies where id = ?",
			&m.MovieID,
		) == "" {
		structure.UpdateRootpath(pathv, "movies", &m.MovieID, cfgp)
	}
	database.ExecN(
		"insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&m.TempTitle,
		&base,
		&ext,
		&list.CfgQuality.Name,
		&m.ResolutionID,
		&m.QualityID,
		&m.CodecID,
		&m.AudioID,
		&m.Proper,
		&m.Repack,
		&m.Extended,
		&m.MovieID,
		&m.DbmovieID,
		&m.Height,
		&m.Width,
	)
	database.Exec1("update movies set missing = 0 where id = ?", &m.MovieID)
	database.Exec2("update movies set quality_reached = ? where id = ?", &i, &m.MovieID)

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, pathv)
		database.AppendCache(logger.CacheFilesMovie, pathv)
	}

	database.Exec1("delete from movie_file_unmatcheds where filepath = ?", pathv)
	return nil
}

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
		database.Scanrows2dyn(
			false,
			"select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE",
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
func importnewmoviessingle(
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	listid int,
) error {
	logger.LogDynamicany2Str(
		"info",
		"get feeds for",
		logger.StrConfig,
		cfgp.NamePrefix,
		logger.StrListname,
		list.Name,
	)
	if !list.Enabled || !list.CfgList.Enabled {
		return logger.ErrDisabled
	}
	if list.CfgList == nil {
		return errors.New("list template not found")
	}

	feed := plfeeds.Get()
	err := feeds(cfgp, list, feed)
	if err != nil {
		return err
	}
	defer plfeeds.Put(feed)
	if feed == nil || len(feed.Movies) == 0 {
		return nil
	}

	listnamefilter := list.Getlistnamefilterignore()

	ctx := context.Background()
	defer ctx.Done()
	pl := worker.WorkerPoolParse.NewGroupContext(ctx)

	var getid uint
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)
	for idx := range list.IgnoreMapLists {
		args.Arr = append(args.Arr, &list.IgnoreMapLists[idx])
	}
	var existing []uint
	if !config.SettingsGeneral.UseMediaCache && listnamefilter != "" {
		existing = database.GetrowsNuncached[uint](
			database.GetdatarowNArg(
				false,
				logger.JoinStrings("select count() from movies where "+listnamefilter),
				args.Arr,
			),
			logger.JoinStrings("select dbmovie_id from movies where "+listnamefilter),
			args.Arr,
		)
	}
	var allowed bool
	var movieid uint
	for idx := range feed.Movies {
		if feed.Movies[idx] == "" {
			continue
		}
		if !logger.HasPrefixI(feed.Movies[idx], "tt") {
			feed.Movies[idx] = logger.AddImdbPrefixP(feed.Movies[idx])
		}
		movieid = importfeed.MovieFindDBIDByImdb(&feed.Movies[idx])

		if movieid != 0 {
			if config.SettingsGeneral.UseMediaCache {
				if database.CacheOneStringTwoIntIndexFunc(
					logger.CacheMovie,
					func(elem *database.DbstaticOneStringTwoInt) bool {
						return elem.Num1 == movieid &&
							(elem.Str == list.Name || strings.EqualFold(elem.Str, list.Name))
					},
				) {
					continue
				}
			} else if database.Scanrows2dyn(false, database.QueryCountMoviesByDBIDList, &getid, movieid, &list.Name); getid >= 1 {
				continue
			}

			if list.IgnoreMapListsLen >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(
						logger.CacheMovie,
						func(elem *database.DbstaticOneStringTwoInt) bool {
							return elem.Num1 == movieid &&
								logger.SlicesContainsI(list.IgnoreMapLists, elem.Str)
						},
					) {
						continue
					}
				} else {
					if slices.Contains(existing, movieid) {
						continue
					}
				}
			}
		}

		allowed, _ = importfeed.AllowMovieImport(&feed.Movies[idx], list.CfgList)
		if allowed {
			pl.Submit(func() {
				defer logger.HandlePanic()
				importfeed.JobImportMoviesByList(feed.Movies[idx], idx, cfgp, listid, true)
			})
		} else {
			logger.LogDynamicany1String("debug", "not allowed movie", logger.StrImdb, feed.Movies[idx])
		}
	}
	pl.Wait()
	ctx.Done()
	return nil
}

// checkreachedmoviesflag checks if the quality cutoff has been reached for all movies in the given list config.
// It queries the movies table for the list, checks the priority of existing files against the config quality cutoff,
// and updates the quality_reached flag in the database accordingly.
func checkreachedmoviesflag(listcfg *config.MediaListsConfig) {
	var minPrio int
	arr := database.QueryMovies(&listcfg.Name)
	for idx := range arr {
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.LogDynamicany1UInt(
				"debug",
				"Quality for Movie not found",
				logger.StrID,
				arr[idx].ID,
			)
			continue
		}

		minPrio, _ = searcher.Getpriobyfiles(
			false,
			&arr[idx].ID,
			false,
			-1,
			config.SettingsQuality[arr[idx].QualityProfile],
			false,
		)
		if minPrio >= config.SettingsQuality[arr[idx].QualityProfile].CutoffPriority {
			if !arr[idx].QualityReached {
				database.Exec1("update movies set quality_reached = 1 where id = ?", &arr[idx].ID)
			}
		} else {
			if arr[idx].QualityReached {
				database.Exec1("update movies set quality_reached = 0 where id = ?", &arr[idx].ID)
			}
		}
	}
}

// RefreshMovie refreshes the data for the given movie by looking up its ID and calling refreshmovies.
// It takes the media config and the movie ID string.
// It converts the ID to an int and calls refreshmovies to refresh that single movie.
func RefreshMovie(cfgp *config.MediaTypeConfig, id *string) {
	refreshmovies(
		cfgp,
		database.Getrows1[string](
			false,
			1,
			"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?",
			id,
		),
	)
}

// refreshmovies refreshes movie data for the given movies. It takes a media config, count of movies to refresh, a query to get the movie IDs, and an optional parameter for the query. It gets the list of movie IDs to refresh, logs info for each, looks up the list name, and calls the import job. Any errors are logged.
func refreshmovies(cfgp *config.MediaTypeConfig, arr []string) {
	if len(arr) == 0 {
		return
	}
	for idx := range arr {
		logger.LogDynamicany1String("info", "Refresh Movie", logger.StrImdb, arr[idx])
		importfeed.JobImportMoviesByList(
			arr[idx],
			idx,
			cfgp,
			getrefreshlistid(&arr[idx], cfgp),
			false,
		)
	}
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
	listname := database.Getdatarow1[string](
		false,
		"SELECT listname FROM movies where dbmovie_id = ?",
		&movieid,
	)
	if listname == "" {
		return -1
	}
	listid := cfgp.GetMediaListsEntryListID(listname)
	if listid == -1 {
		logger.LogDynamicany2Str(
			"error",
			"List not found",
			logger.StrListname,
			listname,
			logger.StrImdb,
			*imdb,
		)
	}
	return listid
}

// MoviesAllJobs runs the specified job for all movie media types.
// It iterates through the global config.SettingsMedia, checks for movie
// media types by prefix, and calls SingleJobs on each matching one.
// The job and force params are passed through to SingleJobs.
func MoviesAllJobs(job string, force bool) {
	if job == "" {
		return
	}
	for _, media := range config.SettingsMedia {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			continue
		}
		SingleJobs(job, media.NamePrefix, "", force, 0)
	}
}
