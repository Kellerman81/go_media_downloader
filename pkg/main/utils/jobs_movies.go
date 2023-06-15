package utils

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/worker"
)

// might change listname
func jobImportMovieParseV2(path string, updatemissing bool, cfgpstr string, listname string, addfound bool) error {
	if structure.CheckUnmatched(cfgpstr, &path) {
		return nil
	}
	if structure.CheckFiles(cfgpstr, &path) {
		return nil
	}
	// if logger.GlobalCache.CheckStringArrValue(logger.StrMovieFileUnmatched, path) {
	// 	return nil
	// }

	m := parser.ParseFile(&path, true, false, logger.StrMovie, true)
	defer m.Close()
	m.M.Title = strings.TrimSpace(m.M.Title)

	//keep list empty for auto detect list since the default list is in the listconfig!
	err := parser.GetDBIDs(&m.M, cfgpstr, "", true)
	if err != nil {
		return err
	}
	if m.M.MovieID != 0 && m.M.Listname != "" {
		listname = m.M.Listname
	}

	if listname == "" {
		return errors.New("listname empty")
	}
	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var templatequality string
	if i != -1 {
		templatequality = config.SettingsMedia[cfgpstr].Lists[i].TemplateQuality
	}
	if !config.CheckGroup("quality_", templatequality) {
		return errors.New("quality template not found")
	}

	err = moviefindids(&m.M, cfgpstr, listname, addfound)
	if err != nil {
		return err
	}

	if m.M.MovieID == 0 {
		id := database.QueryUintColumn(database.Queryidunmatched, &path, &listname)
		if id == 0 {
			if config.SettingsGeneral.UseMediaCache {
				database.CacheUnmatchedMovie = append(database.CacheUnmatchedMovie, path)
			}
			//cache.Append(logger.GlobalCache, logger.StrMovieFileUnmatched, path)
			database.InsertStatic("Insert into movie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", listname, path, database.SQLTimeGetNow(), apiexternal.Buildparsedstring(&m.M))
		} else {
			database.UpdateColumnStatic("Update movie_file_unmatcheds SET last_checked = ? where id = ?", database.SQLTimeGetNow(), id)
			database.UpdateColumnStatic("Update movie_file_unmatcheds SET parsed_data = ? where id = ?", apiexternal.Buildparsedstring(&m.M), id)
		}
		return logger.ErrNotFoundMovie
	}

	parser.GetPriorityMapQual(&m.M, cfgpstr, templatequality, true, false)
	err = parser.ParseVideoFile(&m.M, &path, templatequality)
	if err != nil {
		return err
	}
	var okint int
	if m.M.Priority >= parser.NewCutoffPrio(cfgpstr, templatequality) {
		okint = 1
	}

	if database.QueryStringColumn(database.QueryMoviesGetRootpathByID, m.M.MovieID) == "" && m.M.MovieID != 0 {
		updateRootpath(&path, "movies", m.M.MovieID, cfgpstr)
	}

	if config.SettingsGeneral.UseMediaCache {
		database.CacheFilesMovie = append(database.CacheFilesMovie, path)
	}
	//cache.Append(logger.GlobalCache, "movie_files_cached", path)
	database.InsertStatic("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		path, filepath.Base(path), filepath.Ext(path), templatequality, m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, m.M.Proper, m.M.Repack, m.M.Extended, m.M.MovieID, m.M.DbmovieID, m.M.Height, m.M.Width)
	if updatemissing {
		database.UpdateColumnStatic("Update movies set missing = ? where id = ?", 0, m.M.MovieID)

		updatemoviesreached(okint, m.M.MovieID)
	}

	if database.QueryUintColumn("select id from movie_file_unmatcheds where filepath = ?", &path) != 0 {
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheUnmatchedMovie, func(elem string) bool { return elem == path })
			if ti != -1 {
				logger.Delete(&database.CacheUnmatchedMovie, ti)
			}
		}
		//logger.DeleteFromStringsCache(logger.StrMovieFileUnmatched, path)

		database.DeleteRowStatic(false, "Delete from movie_file_unmatcheds where filepath = ?", &path)
	}
	return nil
}

func getdbmovieidbyimdb(m *apiexternal.ParseInfo, imdbid *string, cfgpstr string, listname string) error {
	if m.DbmovieID == 0 {
		var err error
		m.DbmovieID, err = importfeed.JobImportMovies(m.Imdb, cfgpstr, listname, true)
		if err != nil {
			return err
		}
	}
	if m.MovieID == 0 {
		m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByImdbListname, &m.Imdb, &listname)
	}
	return nil
}

func moviefindids(m *apiexternal.ParseInfo, cfgpstr string, listname string, addfound bool) error {
	var err error
	if m.MovieID == 0 && listname != "" && addfound {
		if m.Imdb != "" {
			err = getdbmovieidbyimdb(m, &m.Imdb, cfgpstr, listname)
			if err != nil {
				return err
			}
			// if m.DbmovieID == 0 {
			// 	m.DbmovieID, err = importfeed.JobImportMovies(m.Imdb, cfgpstr, listname, true)
			// 	if err != nil {
			// 		return err
			// 	}
			// }
			// if m.MovieID == 0 {
			// 	m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByImdbListname, &m.Imdb, &listname)
			// }
		}
	}
	if m.MovieID == 0 && listname != "" && addfound {
		//addFoundList := cfgp.Data[0].AddFoundList
		dbmovie, found, found1 := importfeed.MovieFindDBIDByTitle(m.Imdb, &m.Title, m.Year, logger.StrRss, config.SettingsMedia[cfgpstr].Data[0].AddFound)
		if (found || found1) && listname == config.SettingsMedia[cfgpstr].Data[0].AddFoundList && config.SettingsMedia[cfgpstr].Data[0].AddFound {
			m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, dbmovie, &listname)
			if m.MovieID == 0 {
				if m.Imdb == "" {
					m.Imdb = database.QueryStringColumn(database.QueryDbmoviesGetImdbByID, dbmovie)
				}
				if m.Imdb != "" {
					err = getdbmovieidbyimdb(m, &m.Imdb, cfgpstr, listname)
					if err != nil {
						return err
					}
					// _, err = importfeed.JobImportMovies(m.Imdb, cfgpstr, listname, true)
					// if err != nil {
					// 	return err
					// }
					// m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, dbmovie, &listname)
				}
				if m.MovieID == 0 {
					return logger.ErrNotFoundMovie
				}
			}
		} else if listname == config.SettingsMedia[cfgpstr].Data[0].AddFoundList && config.SettingsMedia[cfgpstr].Data[0].AddFound {
			imdbID, _, _ := importfeed.MovieFindImdbIDByTitle(&m.Title, m.Year, logger.StrRss, config.SettingsMedia[cfgpstr].Data[0].AddFound)
			if m.DbmovieID == 0 {
				m.DbmovieID, err = importfeed.JobImportMovies(imdbID, cfgpstr, listname, true)
				if err != nil {
					return err
				}
			}
			if m.MovieID == 0 {
				m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, dbmovie, &listname)
			}
			if m.MovieID == 0 {
				return logger.ErrNotFoundMovie
			}
		}
	}
	return nil
}

func updatemoviesreached(reached int, dbmovieid uint) {
	database.UpdateColumnStatic("Update movies set quality_reached = ? where id = ?", reached, dbmovieid)
}

func getTraktUserPublicMovieList(templatelist string) (*feedResults, error) {
	if !config.CheckGroup("list_", templatelist) {
		return nil, errors.New("list template not found")
	}

	if config.SettingsList["list_"+templatelist].TraktUsername == "" || config.SettingsList["list_"+templatelist].TraktListName == "" {
		return nil, errors.New("username empty")
	}
	if config.SettingsList["list_"+templatelist].TraktListType == "" {
		return nil, errors.New("list type empty")
	}
	data, err := apiexternal.TraktAPI.GetUserList(config.SettingsList["list_"+templatelist].TraktUsername, config.SettingsList["list_"+templatelist].TraktListName, config.SettingsList["list_"+templatelist].TraktListType, config.SettingsList["list_"+templatelist].Limit)
	if err != nil || data == nil {
		return nil, err
	}
	if len(*data) == 0 {
		return nil, logger.ErrNotFound
	}
	d := feedResults{Movies: make([]string, 0, len(*data))}
	for idx := range *data {
		d.Movies = append(d.Movies, (*data)[idx].Movie.Ids.Imdb)
	}
	logger.Clear(data)

	return &d, nil
}

func importnewmoviessingle(cfgpstr string, listname string) error {
	if listname == "" {
		return errors.New("no listname")
	}
	logger.Log.Debug().Str("config", cfgpstr).Str(logger.StrListname, listname).Msg("get feeds for")
	//logger.LogAnyDebug("get feeds for", logger.LoggerValue{Name: "config", Value: cfgpstr}, logger.LoggerValue{Name: logger.StrListname, Value: listname})
	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var strtemplate string
	if i != -1 {
		strtemplate = config.SettingsMedia[cfgpstr].Lists[i].TemplateList
	}
	feed, err := feeds(cfgpstr, listname, strtemplate)
	if err != nil {
		return err
	}
	if len(feed.Movies) == 0 {
		return nil
	}

	//if lenfeed > 900 {
	//workergroup := worker.WorkerPoolMetadata.Group()
	lenignore := len(config.SettingsMedia[cfgpstr].Lists[i].IgnoreMapLists)
	//var dbcached database.DbstaticThreeStringOneInt
	var added int

	var listnamefilter string
	if lenignore >= 1 {
		listnamefilter = " and listname in (" + logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists[i].IgnoreMapLists)-1) + ")"
	}

	var movieid, ti int
	var foundmovie, allowed bool

	for idx := range feed.Movies {
		if feed.Movies[idx] == "" {
			continue
		}

		movieid = 0
		if config.SettingsGeneral.UseMediaCache {
			ti = logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool {
				return strings.EqualFold(elem.Str3, feed.Movies[idx])
			})
			if ti != -1 {
				movieid = database.CacheDBMovie[ti].Num1
			}
		} else {
			movieid = database.QueryIntColumn(database.QueryDbmoviesGetIDByImdb, &feed.Movies[idx])
		}
		// movieid := cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool {
		// 	return strings.EqualFold(elem.Str3, feed.Movies[idx])
		// }).Num1

		foundmovie = false
		if movieid != 0 {
			if config.SettingsGeneral.UseMediaCache {
				foundmovie = logger.IndexFunc(&database.CacheMovie, func(elem database.DbstaticOneStringOneInt) bool {
					return elem.Num == movieid && strings.EqualFold(elem.Str, listname)
				}) != -1
			} else {
				foundmovie = database.QueryIntColumn("select count() from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", &movieid, &listname) >= 1
			}
			// foundmovie = cache.CheckFunc(logger.GlobalCache, "movies_cached", func(elem database.DbstaticOneStringOneInt) bool {
			// 	return elem.Num == movieid && strings.EqualFold(elem.Str, listname)
			// })
			if !foundmovie && lenignore >= 1 {
				if config.SettingsGeneral.UseMediaCache {
					foundmovie = logger.IndexFunc(&database.CacheMovie, func(elem database.DbstaticOneStringOneInt) bool {
						return elem.Num == movieid && logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[i].IgnoreMapLists, elem.Str)
					}) != -1
				} else {
					foundmovie = database.QueryIntColumn("select count() from movies where dbmovie_id = ?"+listnamefilter, append([]interface{}{&movieid}, config.SettingsMedia[cfgpstr].Lists[i].IgnoreMapListsInt...)...) >= 1
				}
				// foundmovie = cache.CheckFunc(logger.GlobalCache, "movies_cached", func(elem database.DbstaticOneStringOneInt) bool {
				// 	return elem.Num == movieid && logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[i].IgnoreMapLists, elem.Str)
				// })
			}
		}

		if !foundmovie {
			allowed, _ = importfeed.AllowMovieImport(&feed.Movies[idx], config.SettingsMedia[cfgpstr].Lists[i].TemplateList)
			if allowed {
				movie := feed.Movies[idx]
				added++
				worker.WorkerPoolMetadata.Submit(func() {
					_, err := importfeed.JobImportMovies(movie, cfgpstr, listname, true)
					if err != nil { // && err.Error() != "movie ignored"
						logger.Log.Error().Err(err).Str("Imdb", movie).Msg("Import Failed")
					}
				})
			} else {
				logger.Log.Debug().Str(logger.StrImdb, feed.Movies[idx]).Msg("not allowed movie")
			}
		}

	}
	feed.Close()
	return nil
}

func checkreachedmoviesflag(cfgpstr string, listname string) {
	tbl := database.QueryMovies("select id, quality_reached, quality_profile from movies where listname = ? COLLATE NOCASE", &listname)
	var reached bool
	for idx := range *tbl {
		if !config.CheckGroup("quality_", (*tbl)[idx].QualityProfile) {
			logger.Log.Debug().Int(logger.StrID, int((*tbl)[idx].ID)).Msg("Quality for Movie not found")
			continue
		}

		reached = false

		if searcher.GetHighestMoviePriorityByFiles(false, true, (*tbl)[idx].ID, (*tbl)[idx].QualityProfile) >= parser.NewCutoffPrio(cfgpstr, (*tbl)[idx].QualityProfile) {
			reached = true
		}
		if (*tbl)[idx].QualityReached && !reached {
			updatemoviesreached(0, (*tbl)[idx].ID)
			continue
		}

		if !(*tbl)[idx].QualityReached && reached {
			updatemoviesreached(1, (*tbl)[idx].ID)
		}
	}
	logger.Clear(tbl)
}

func RefreshMovies() {
	refreshmovies(database.QueryCountColumn("dbmovies", ""), "select distinct dbmovies.imdb_id, movies.listname from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id", 0)
}

func RefreshMovie(id string) {
	refreshmovies(1, "select distinct dbmovies.imdb_id, movies.listname from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?", logger.StringToInt(id))
}

func RefreshMoviesInc() {
	refreshmovies(100, "select distinct dbmovies.imdb_id, movies.listname from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100", 0)
}

func getrefreshmovies(count int, query string, arg int) *[]database.DbstaticTwoString {
	if arg != 0 {
		return database.QueryStaticColumnsTwoString(false, count, query, arg)
	} else {
		return database.QueryStaticColumnsTwoString(false, count, query)
	}
}
func refreshmovies(count int, query string, arg int) {
	tbl := getrefreshmovies(count, query, arg)
	var err error
	for idx := range *tbl {
		logger.Log.Debug().Str(logger.StrImdb, (*tbl)[idx].Str1).Int("row", idx).Msg("Refresh Movie")
		//logger.LogAnyDebug("refresh movie", logger.LoggerValue{Name: logger.StrImdb, Value: (*tbl)[idx].Str1}, logger.LoggerValue{Name: "row", Value: idx})
		_, err = importfeed.JobImportMovies((*tbl)[idx].Str1, config.FindconfigTemplateNameOnList("movie_", (*tbl)[idx].Str2), (*tbl)[idx].Str2, false)
		if err != nil && err.Error() != "movie ignored" {
			logger.Log.Error().Err(err).Str(logger.StrImdb, (*tbl)[idx].Str1).Msg("Movie Import Failed")
		}
	}
	logger.Clear(tbl)
}

func MoviesAllJobs(job string, force bool) {
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrMovie) {
			continue
		}
		SingleJobs(logger.StrMovie, job, config.SettingsMedia[idxp].NamePrefix, "", force)
	}
}
