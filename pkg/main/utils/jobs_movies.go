package utils

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
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
func jobImportMovieParseV2(m *apiexternal.FileParser, pathv string, updatemissing bool, cfgp *config.MediaTypeConfig, listid int, list *config.MediaListsConfig, addfound bool) error {
	if list.CfgQuality == nil {
		return errors.New("quality template not found")
	}

	if m.M.MovieID == 0 && addfound {
		if m.M.Imdb != "" {
			if getdbmovieidbyimdb(m, cfgp, listid) {
				return logger.ErrNotFoundMovie
			}
		}
	}
	if m.M.MovieID == 0 && addfound {
		if m.M.Imdb == "" {
			importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m)
		}

		var bl bool
		if m.M.Imdb != "" {
			m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&m.M.Imdb)
			if m.M.DbmovieID != 0 {
				bl = true
			}
		}
		if bl && list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.M.MovieID, &m.M.DbmovieID, &list.Name)
			if m.M.MovieID == 0 {
				if m.M.Imdb == "" {
					if config.SettingsGeneral.UseMediaCache {
						database.CacheThreeStringIntIndexFuncGetImdb(logger.CacheDBMovie, int(m.M.DbmovieID), &m.M)
					} else {
						_ = database.ScanrowsNdyn(false, "select imdb_id from dbmovies where id = ?", &m.M.Imdb, &m.M.DbmovieID)
					}
				}
				if m.M.Imdb != "" {
					if getdbmovieidbyimdb(m, cfgp, listid) && m.M.MovieID == 0 {
						return logger.ErrNotFoundMovie
					}
				}
			}
		} else if list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
			importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m)
			var err error
			if m.M.DbmovieID == 0 {
				m.M.DbmovieID, err = importfeed.JobImportMovies(m.M.Imdb, cfgp, listid, true)
			}
			if err != nil && m.M.MovieID == 0 {
				database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.M.MovieID, &m.M.DbmovieID, &list.Name)
			}
			if err != nil && m.M.MovieID == 0 {
				err = logger.ErrNotFoundMovie
			}

			if err != nil {
				return err
			}
		}
	}

	if m.M.MovieID == 0 {
		structure.AddUnmatched(cfgp, &pathv, &list.Name, m)
		return logger.ErrNotFoundMovie
	}

	parser.GetPriorityMapQual(&m.M, cfgp, list.CfgQuality, true, false)
	err := parser.ParseVideoFile(m, pathv, list.CfgQuality)
	if err != nil {
		return err
	}
	var i int
	if m.M.Priority >= list.CfgQuality.CutoffPriority {
		i = 1
	}

	if m.M.MovieID != 0 && database.GetdatarowN[string](false, "select rootpath from movies where id = ?", &m.M.MovieID) == "" {
		structure.UpdateRootpath(pathv, "movies", &m.M.MovieID, cfgp)
	}

	basestr := filepath.Base(pathv)
	extstr := filepath.Ext(pathv)
	database.ExecN("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&pathv, &basestr, &extstr, &list.CfgQuality.Name, &m.M.ResolutionID, &m.M.QualityID, &m.M.CodecID, &m.M.AudioID, &m.M.Proper, &m.M.Repack, &m.M.Extended, &m.M.MovieID, &m.M.DbmovieID, &m.M.Height, &m.M.Width)
	if updatemissing {
		database.ExecN("update movies set missing = 0 where id = ?", &m.M.MovieID)
		database.ExecN("update movies set quality_reached = ? where id = ?", &i, &m.M.MovieID)
	}

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, pathv)
		database.AppendStringCache(logger.CacheFilesMovie, pathv)
	}

	database.ExecN("delete from movie_file_unmatcheds where filepath = ?", &pathv)
	return nil
}

// getdbmovieidbyimdb retrieves the database movie ID for the given movie struct by looking it up based on the IMDB ID.
// It first checks if the movie struct already has the ID populated, otherwise it calls the import job to add the movie to the database if needed.
// It also populates the local movie ID by querying the movies table based on the retrieved database movie ID.
func getdbmovieidbyimdb(m *apiexternal.FileParser, cfgp *config.MediaTypeConfig, listid int) bool {
	if m.M.DbmovieID == 0 {
		var err error
		m.M.DbmovieID, err = importfeed.JobImportMovies(m.M.Imdb, cfgp, listid, true)
		if err != nil {
			return true
		}
	}
	if m.M.MovieID == 0 {
		database.ScanrowsNdyn(false, "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", &m.M.MovieID, &m.M.Imdb, &cfgp.Lists[listid].Name)
	}
	return false
}

// importnewmoviessingle imports new movies from a feed for a single list.
// It takes a media config and list ID, gets the feed, checks/filters movies,
// and submits import jobs. It uses worker pools and caching for performance.
func importnewmoviessingle(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error {
	logger.LogDynamic("info", "get feeds for", logger.NewLogField("config", &cfgp.NamePrefix), logger.NewLogField(logger.StrListname, &list.Name))

	feed, err := feeds(cfgp, list)
	if err != nil {
		return err
	}
	defer feed.Close()
	if feed == nil {
		return nil
	}
	if len(feed.Movies) == 0 {
		return nil
	}

	listnamefilter := getlistnamefilterignore(list)

	workergroup := worker.WorkerPoolParse.Group()
	var getid uint
	args := make([]any, list.IgnoreMapListsLen+1)
	for i := range list.IgnoreMapLists {
		args[i] = &list.IgnoreMapLists[i]
	}
	for idx := range feed.Movies {
		if feed.Movies[idx] == "" {
			continue
		}

		movieid := importfeed.MovieFindDBIDByImdb(&feed.Movies[idx])

		if movieid != 0 {
			intmovie := int(movieid)
			if config.SettingsGeneral.UseMediaCache {
				if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
					return elem.Num1 == intmovie && (elem.Str == list.Name || strings.EqualFold(elem.Str, list.Name))
				}) {
					continue
				}
			} else if _ = database.ScanrowsNdyn(false, database.QueryCountMoviesByDBIDList, &getid, &movieid, &list.Name); getid >= 1 {
				continue
			}

			if list.IgnoreMapListsLen >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
						return elem.Num1 == intmovie && logger.SlicesContainsI(list.IgnoreMapLists, elem.Str)
					}) {
						continue
					}
				} else {
					args[list.IgnoreMapListsLen] = &movieid
					if _ = database.ScanrowsNdyn(false, logger.JoinStrings("select count() from movies where ", listnamefilter, "dbmovie_id = ?"), &getid, args...); getid >= 1 {
						continue
					}
				}
			}
		}

		allowed, _ := importfeed.AllowMovieImport(feed.Movies[idx], list.CfgList)
		if allowed {
			workergroup.Submit(func() {
				importfeed.JobImportMoviesByList(feed.Movies[idx], idx, cfgp, listid, true)
			})
		} else {
			logger.LogDynamic("debug", "not allowed movie", logger.NewLogField(logger.StrImdb, &feed.Movies[idx]))
		}
	}
	workergroup.Wait()
	//clear(args)
	args = nil
	return nil
}

// checkreachedmoviesflag checks if the quality cutoff has been reached for all movies in the given list config.
// It queries the movies table for the list, checks the priority of existing files against the config quality cutoff,
// and updates the quality_reached flag in the database accordingly.
func checkreachedmoviesflag(listcfg *config.MediaListsConfig) {
	arr := database.QueryMovies(listcfg.Name)
	for idx := range arr {
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.LogDynamic("debug", "Quality for Movie not found", logger.NewLogField(logger.StrID, arr[idx].ID))
			continue
		}

		minPrio, _ := searcher.Getpriobyfiles(false, &arr[idx].ID, false, -1, config.SettingsQuality[arr[idx].QualityProfile])
		if minPrio >= config.SettingsQuality[arr[idx].QualityProfile].CutoffPriority {
			if !arr[idx].QualityReached {
				database.ExecN("update movies set quality_reached = 1 where id = ?", &arr[idx].ID)
			}
		} else {
			if arr[idx].QualityReached {
				database.ExecN("update movies set quality_reached = 0 where id = ?", &arr[idx].ID)
				continue
			}
		}
	}
	//clear(arr)
	arr = nil
}

// refreshMovies refreshes movie data for all movies in the database. It takes a media config and calls refreshmovies to select all distinct imdb_id values from the dbmovies table joined with the movies table, and refresh each movie by calling the import job.
func refreshMovies(cfgp *config.MediaTypeConfig) {
	refreshmovies(cfgp, database.GetdatarowN[int](false, "select count() from dbmovies"), "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id", nil)
}

// RefreshMovie refreshes the data for the given movie by looking up its ID and calling refreshmovies.
// It takes the media config and the movie ID string.
// It converts the ID to an int and calls refreshmovies to refresh that single movie.
func RefreshMovie(cfgp *config.MediaTypeConfig, id string) {
	idint := logger.StringToInt(id)
	refreshmovies(cfgp, 1, "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?", &idint)
}

// refreshMoviesInc incrementally refreshes movie data for up to 100 movies.
// It calls refreshmovies to select the latest updated movies from the database,
// up to a limit of 100, and refresh each one by calling the import job.
func refreshMoviesInc(cfgp *config.MediaTypeConfig) {
	refreshmovies(cfgp, 100, "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100", nil)
}

// getrefreshmovies queries the database to get a list of movie IDs to refresh.
// It takes a count of movies to return, a query to run, and an optional query parameter.
// It executes the query with or without the parameter based on if arg is nil.
// The query should return a list of distinct imdb_id values to refresh.
// It returns a string slice containing the imdb_id values.
func getrefreshmovies(count int, query string, arg *int) []string {
	if count == 0 {
		return nil
	}
	if arg != nil {
		return database.GetrowsN[string](false, count, query, arg)
	}
	return database.GetrowsN[string](false, count, query)
}

// refreshmovies refreshes movie data for the given movies. It takes a media config, count of movies to refresh, a query to get the movie IDs, and an optional parameter for the query. It gets the list of movie IDs to refresh, logs info for each, looks up the list name, and calls the import job. Any errors are logged.
func refreshmovies(cfgp *config.MediaTypeConfig, count int, query string, arg *int) {
	if count == 0 {
		return
	}

	arr := getrefreshmovies(count, query, arg)
	for idx := range arr {
		logger.LogDynamic("info", "Refresh Movie", logger.NewLogField(logger.StrImdb, &arr[idx]))
		listname := database.GetdatarowN[string](false, "SELECT listname FROM movies JOIN dbmovies ON dbmovies.id = movies.dbmovie_id WHERE dbmovies.imdb_id = ?", &arr[idx])
		if listname == "" {
			continue
		}
		importfeed.JobImportMoviesByList(arr[idx], idx, findconfigTemplateNameOnList(false, listname), config.GetMediaListsEntryListID(cfgp, listname), false)
	}
	//clear(arr)
	arr = nil
}

// findconfigTemplateNameOnList searches through the global config.SettingsMedia
// to find the MediaTypeConfig that matches the given useseries and listname.
// It returns the matching MediaTypeConfig, or nil if not found.
func findconfigTemplateNameOnList(useseries bool, listname string) *config.MediaTypeConfig {
	for _, media := range config.SettingsMedia {
		if useseries == media.Useseries {
			if _, ok := media.ListsMap[listname]; ok {
				return media
			}
			for idx := range media.Lists {
				if media.Lists[idx].Name == listname || strings.EqualFold(media.Lists[idx].Name, listname) {
					return media
				}
			}
		}
	}
	logger.LogDynamic("debug", "config template not found", logger.NewLogField("series", useseries), logger.NewLogField("list", &listname))
	return nil
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
