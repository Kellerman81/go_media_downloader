package utils

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
)

// jobImportMovieParseV2 parses a movie file at the given path using the
// provided parser and list config. It handles adding new movies to the DB,
// updating existing movie records, and caching parsed data.
func jobImportMovieParseV2(m *database.ParseInfo, pathv string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addfound bool) error {
	if list.CfgQuality == nil {
		return errors.New("quality template not found")
	}

	if m.MovieID == 0 && addfound {
		if m.Imdb != "" {
			if getdbmovieidbyimdb(m, cfgp, list) {
				return logger.ErrNotFoundMovie
			}
		}
	}
	if m.MovieID == 0 && addfound {
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
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.MovieID, &m.DbmovieID, &list.Name)
			if m.MovieID == 0 {
				if m.Imdb == "" {
					if config.SettingsGeneral.UseMediaCache {
						m.CacheThreeStringIntIndexFuncGetImdb(logger.CacheDBMovie, m.DbmovieID)
					} else {
						_ = database.Scanrows1dyn(false, "select imdb_id from dbmovies where id = ?", &m.Imdb, &m.DbmovieID)
					}
				}
				if m.Imdb != "" {
					if getdbmovieidbyimdb(m, cfgp, list) {
						if m.MovieID == 0 {
							return logger.ErrNotFoundMovie
						}
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
				database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.MovieID, &m.DbmovieID, &list.Name)
			}
			if err != nil && m.MovieID == 0 {
				err = logger.ErrNotFoundMovie
			}

			if err != nil {
				return err
			}
		}
	}

	if m.MovieID == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name)
		return logger.ErrNotFoundMovie
	}

	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)
	err := parser.ParseVideoFile(m, pathv, list.CfgQuality)
	if err != nil {
		return err
	}
	var i int
	if m.Priority >= list.CfgQuality.CutoffPriority {
		i = 1
	}

	if m.MovieID != 0 {
		database.Scanrows1dyn(false, "select rootpath from movies where id = ?", &m.TempTitle, &m.MovieID)
		if m.TempTitle == "" {
			structure.UpdateRootpath(pathv, "movies", &m.MovieID, cfgp)
		}
	}
	m.TempTitle = pathv
	database.ExecN("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&m.TempTitle, filepath.Base(pathv), filepath.Ext(pathv), &list.CfgQuality.Name, &m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID, &m.Proper, &m.Repack, &m.Extended, &m.MovieID, &m.DbmovieID, &m.Height, &m.Width)
	//if updatemissing {
	database.Exec1("update movies set missing = 0 where id = ?", &m.MovieID)
	database.ExecN("update movies set quality_reached = ? where id = ?", &i, &m.MovieID)
	//}

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, pathv)
		database.AppendCache(logger.CacheFilesMovie, pathv)
	}

	database.Exec1("delete from movie_file_unmatcheds where filepath = ?", &m.TempTitle)
	return nil
}

// getdbmovieidbyimdb retrieves the database movie ID for the given movie struct by looking it up based on the IMDB ID.
// It first checks if the movie struct already has the ID populated, otherwise it calls the import job to add the movie to the database if needed.
// It also populates the local movie ID by querying the movies table based on the retrieved database movie ID.
func getdbmovieidbyimdb(m *database.ParseInfo, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig) bool {
	if m.DbmovieID == 0 {
		var err error
		m.DbmovieID, err = importfeed.JobImportMovies(m.Imdb, cfgp, cfgp.GetMediaListsEntryListID(list.Name), true)
		if err != nil {
			return true
		}
	}
	if m.MovieID == 0 {
		database.ScanrowsNdyn(false, "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &m.MovieID, &m.Imdb, &list.Name)
	}
	return false
}

// importnewmoviessingle imports new movies from a feed for a single list.
// It takes a media config and list ID, gets the feed, checks/filters movies,
// and submits import jobs. It uses worker pools and caching for performance.
func importnewmoviessingle(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int8) error {
	logger.LogDynamicany("info", "get feeds for", &logger.StrConfig, &cfgp.NamePrefix, &logger.StrListname, &list.Name)

	feed, err := feeds(cfgp, list)
	if err != nil {
		return err
	}
	if feed == nil || len(feed.Movies) == 0 {
		feed.Close()
		return nil
	}

	listnamefilter := list.Getlistnamefilterignore()

	//workergroup := worker.GetPoolParserGroup()
	wg := pool.NewSizedGroup(int(config.SettingsGeneral.WorkerParse))

	var getid uint
	//args := make([]any, 0, len(list.IgnoreMapLists)+1)
	args := logger.PLArrAny.Get()
	for idx := range list.IgnoreMapLists {
		args.Arr = append(args.Arr, &list.IgnoreMapLists[idx])
	}
	args.Arr = append(args.Arr, &logger.V0)
	var movieid uint
	var allowed bool
	for idx := range feed.Movies {
		if feed.Movies[idx] == "" {
			continue
		}

		movieid = importfeed.MovieFindDBIDByImdb(feed.Movies[idx])

		if movieid != 0 {
			if config.SettingsGeneral.UseMediaCache {
				if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
					return elem.Num1 == movieid && (elem.Str == list.Name || strings.EqualFold(elem.Str, list.Name))
				}) {
					continue
				}
			} else if _ = database.ScanrowsNdyn(false, database.QueryCountMoviesByDBIDList, &getid, &movieid, &list.Name); getid >= 1 {
				continue
			}

			if list.IgnoreMapListsLen >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
						return elem.Num1 == movieid && logger.SlicesContainsI(list.IgnoreMapLists, elem.Str)
					}) {
						continue
					}
				} else {
					args.Arr[list.IgnoreMapListsLen] = &movieid
					if _ = database.ScanrowsNArr(false, logger.JoinStrings("select count() from movies where ", listnamefilter, "dbmovie_id = ?"), &getid, args.Arr); getid >= 1 {
						continue
					}
				}
			}
		}

		allowed, _ = importfeed.AllowMovieImport(feed.Movies[idx], list.CfgList)
		if allowed {
			//workergroup.Submit(func() {
			wg.Add()
			go importfeed.JobImportMoviesByList(wg, feed.Movies[idx], idx, cfgp, listid, true)
		} else {
			logger.LogDynamicany("debug", "not allowed movie", &logger.StrImdb, &feed.Movies[idx])
		}
	}
	wg.Wait()
	wg.Close()
	feed.Close()
	logger.PLArrAny.Put(args)
	//logger.PLArrAny.Put(args)
	//args = nil
	return nil
}

// checkreachedmoviesflag checks if the quality cutoff has been reached for all movies in the given list config.
// It queries the movies table for the list, checks the priority of existing files against the config quality cutoff,
// and updates the quality_reached flag in the database accordingly.
func checkreachedmoviesflag(listcfg *config.MediaListsConfig) {
	arr := database.QueryMovies(&listcfg.Name)
	for idx := range arr {
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.LogDynamicany("debug", "Quality for Movie not found", &logger.StrID, &arr[idx].ID)
			continue
		}

		minPrio, _ := searcher.Getpriobyfiles(false, &arr[idx].ID, false, -1, config.SettingsQuality[arr[idx].QualityProfile])
		if minPrio >= config.SettingsQuality[arr[idx].QualityProfile].CutoffPriority {
			if !arr[idx].QualityReached {
				database.Exec1("update movies set quality_reached = 1 where id = ?", &arr[idx].ID)
			}
		} else {
			if arr[idx].QualityReached {
				database.Exec1("update movies set quality_reached = 0 where id = ?", &arr[idx].ID)
				continue
			}
		}
	}
	//clear(arr)
}

// refreshMovies refreshes movie data for all movies in the database. It takes a media config and calls refreshmovies to select all distinct imdb_id values from the dbmovies table joined with the movies table, and refresh each movie by calling the import job.
func refreshMovies(cfgp *config.MediaTypeConfig) {
	refreshmovies(cfgp, database.GetrowsN[string](false, database.GetdatarowN[uint](false, "select count() from dbmovies"), "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id"))
}

// RefreshMovie refreshes the data for the given movie by looking up its ID and calling refreshmovies.
// It takes the media config and the movie ID string.
// It converts the ID to an int and calls refreshmovies to refresh that single movie.
func RefreshMovie(cfgp *config.MediaTypeConfig, id string) {
	idint := logger.StringToInt(id)
	refreshmovies(cfgp, database.GetrowsN[string](false, 1, "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?", &idint))
}

// refreshMoviesInc incrementally refreshes movie data for up to 100 movies.
// It calls refreshmovies to select the latest updated movies from the database,
// up to a limit of 100, and refresh each one by calling the import job.
func refreshMoviesInc(cfgp *config.MediaTypeConfig) {
	refreshmovies(cfgp, database.GetrowsN[string](false, 100, "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100"))
}

// refreshmovies refreshes movie data for the given movies. It takes a media config, count of movies to refresh, a query to get the movie IDs, and an optional parameter for the query. It gets the list of movie IDs to refresh, logs info for each, looks up the list name, and calls the import job. Any errors are logged.
func refreshmovies(cfgp *config.MediaTypeConfig, arr []string) {
	if len(arr) == 0 {
		return
	}
	var listid int8
	var listname string
	for idx := range arr {
		database.Scanrows1dyn(false, "SELECT listname FROM movies JOIN dbmovies ON dbmovies.id = movies.dbmovie_id WHERE dbmovies.imdb_id = ?", &listname, &arr[idx])
		if listname == "" {
			continue
		}
		listid = cfgp.GetMediaListsEntryListID(listname)
		if listid == -1 {
			for _, media := range config.SettingsMedia {
				if media.Useseries || media.Name == cfgp.Name {
					continue
				}

				for k := range media.Lists {
					if media.Lists[k].Name == listname || strings.EqualFold(media.Lists[k].Name, listname) {
						listid = int8(k)
						break
					}
				}
				if listid != -1 {
					break
				}
			}
			if listid == -1 {
				logger.LogDynamicany("error", "List not found", &logger.StrListname, &listname, &logger.StrImdb, &arr[idx])
			}
			continue
		}
		logger.LogDynamicany("info", "Refresh Movie", &logger.StrImdb, &arr[idx])
		importfeed.JobImportMoviesByList(nil, arr[idx], idx, cfgp, listid, false)
	}
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
		SingleJobs(job, media.NamePrefix, "", force)
	}
}
