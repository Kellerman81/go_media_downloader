package utils

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/remeh/sizedwaitgroup"
)

var MovieImportJobRunning map[string]bool

func JobImportMovies(cfg config.Cfg, dbmovie database.Dbmovie, configEntry config.MediaTypeConfig, list config.MediaListsConfig, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := dbmovie.ImdbID
	if jobName == "" {
		jobName = list.Name
	}
	defer func() {
		database.ReadWriteMu.Lock()
		delete(MovieImportJobRunning, jobName)
		database.ReadWriteMu.Unlock()
		wg.Done()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := MovieImportJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		MovieImportJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()

	finddbmovie, _ := database.GetDbmovie(database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
	cdbmovie, _ := database.CountRows("dbmovies", database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
	if cdbmovie == 0 {
		logger.Log.Debug("Get Movie Metadata: ", dbmovie.ImdbID)
		if len(cfg.General.MovieMetaSourcePriority) >= 1 {
			for idxmeta := range cfg.General.MovieMetaSourcePriority {
				if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "imdb") {
					logger.Log.Debug("Get Movie Metadata - imdb: ", dbmovie.ImdbID)
					dbmovie.GetImdbMetadata(false)
				}
				if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "tmdb") {
					logger.Log.Debug("Get Movie Metadata - tmdb: ", dbmovie.ImdbID)
					dbmovie.GetTmdbMetadata(false)
				}
				if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "omdb") {
					logger.Log.Debug("Get Movie Metadata - omdb: ", dbmovie.ImdbID)
					dbmovie.GetOmdbMetadata(false)
				}
				if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "trakt") {
					logger.Log.Debug("Get Movie Metadata - trakt: ", dbmovie.ImdbID)
					dbmovie.GetTraktMetadata(false)
				}
			}
		} else {
			dbmovie.GetMetadata(cfg.General.MovieMetaSourceImdb, cfg.General.MovieMetaSourceTmdb, cfg.General.MovieMetaSourceOmdb, cfg.General.MovieMetaSourceTrakt)
		}

		cdbmovie2, _ := database.CountRows("dbmovies", database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
		if cdbmovie2 == 0 {
			dbresult, dbresulterr := database.InsertArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
				[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug})
			if dbresulterr != nil {
				logger.Log.Error(dbresulterr)
				return
			}
			newid, newiderr := dbresult.LastInsertId()
			if newiderr != nil {
				logger.Log.Error(newiderr)
				return
			}
			dbmovie.ID = uint(newid)
			if dbmovie.ID == 0 {
				logger.Log.Error("gettig dbmovie id error")
				return
			}
			logger.Log.Debug("Get Movie Titles: ", dbmovie.Title)
			titlegroup := dbmovie.GetTitles(configEntry.Metadata_title_languages, cfg.General.MovieMetaSourceImdb, cfg.General.MovieMetaSourceTmdb, cfg.General.MovieMetaSourceTrakt)
			for idxtitle := range titlegroup {
				countert, _ := database.CountRows("dbmovie_titles", database.Query{Where: "dbmovie_id = ? and title = ?", WhereArgs: []interface{}{dbmovie.ID, titlegroup[idxtitle].Title}})
				if countert == 0 {
					database.InsertArray("dbmovie_titles", []string{"dbmovie_id", "title", "slug"}, []interface{}{dbmovie.ID, titlegroup[idxtitle].Title, titlegroup[idxtitle].Slug})
				}
			}
		} else {
			dbmovie, _ = database.GetDbmovie(database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
		}
	} else {
		dbmovie = finddbmovie
	}
	for idxignore := range list.Ignore_map_lists {
		counteri, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovie.ID, list.Ignore_map_lists[idxignore]}})
		if counteri >= 1 {
			return
		}
	}
	counterm, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovie.ID, list.Name}})
	if counterm >= 1 {
		for idxreplace := range list.Replace_map_lists {
			movietest, _ := database.QueryMovies(database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovie.ID, list.Replace_map_lists[idxreplace]}})
			for _, replacemovie := range movietest {
				database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, list.Name, dbmovie.ID, list.Template_quality}, database.Query{Where: "id=?", WhereArgs: []interface{}{replacemovie.ID}})
			}
		}
	} else {
		logger.Log.Debug("Add Movie: ", dbmovie.Title)
		_, moviereserr := database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, list.Name, dbmovie.ID, list.Template_quality})
		if moviereserr != nil {
			logger.Log.Error(moviereserr)
			return
		}
		for idxreplace := range list.Replace_map_lists {
			movietest, _ := database.QueryMovies(database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovie.ID, list.Replace_map_lists[idxreplace]}})
			for _, replacemovie := range movietest {
				database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, list.Name, dbmovie.ID, list.Template_quality}, database.Query{Where: "id=?", WhereArgs: []interface{}{replacemovie.ID}})
			}
		}
	}
}

func JobReloadMovies(cfg config.Cfg, dbmovie database.Dbmovie, configEntry config.MediaTypeConfig, list config.MediaListsConfig, wg *sizedwaitgroup.SizedWaitGroup) {
	if cfg.General.SchedulerDisabled {
		return
	}
	jobName := dbmovie.ImdbID
	if jobName == "" {
		jobName = list.Name
	}
	defer func() {
		database.ReadWriteMu.Lock()
		delete(MovieImportJobRunning, jobName)
		database.ReadWriteMu.Unlock()
		wg.Done()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := MovieImportJobRunning[jobName]; nok {
		if MovieImportJobRunning[jobName] {
			logger.Log.Debug("Job already running: ", jobName)
			database.ReadWriteMu.Unlock()
			return
		}
	} else {
		MovieImportJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()

	dbmovie, _ = database.GetDbmovie(database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
	logger.Log.Debug("Get Movie Metadata: ", dbmovie.ImdbID)
	if len(cfg.General.MovieMetaSourcePriority) >= 1 {
		for idxmeta := range cfg.General.MovieMetaSourcePriority {
			if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "imdb") {
				logger.Log.Debug("Get Movie Titles - imdb: ", dbmovie.Title)
				dbmovie.GetImdbMetadata(false)
			}
			if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "tmdb") {
				logger.Log.Debug("Get Movie Titles - tmdb: ", dbmovie.Title)
				dbmovie.GetTmdbMetadata(false)
			}
			if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "omdb") {
				logger.Log.Debug("Get Movie Titles - omdb: ", dbmovie.Title)
				dbmovie.GetOmdbMetadata(false)
			}
			if strings.EqualFold(cfg.General.MovieMetaSourcePriority[idxmeta], "trakt") {
				logger.Log.Debug("Get Movie Titles - trakt: ", dbmovie.Title)
				dbmovie.GetTraktMetadata(false)
			}
		}
	} else {
		dbmovie.GetMetadata(cfg.General.MovieMetaSourceImdb, cfg.General.MovieMetaSourceTmdb, cfg.General.MovieMetaSourceOmdb, cfg.General.MovieMetaSourceTrakt)
	}
	database.UpdateArray("dbmovies",
		[]string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
		[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
		database.Query{Where: "id=?", WhereArgs: []interface{}{dbmovie.ID}})

	logger.Log.Debug("Get Movie Titles: ", dbmovie.Title)
	titlegroup := dbmovie.GetTitles(configEntry.Metadata_title_languages, cfg.General.MovieMetaSourceImdb, cfg.General.MovieMetaSourceTmdb, cfg.General.MovieMetaSourceTrakt)
	for idxtitle := range titlegroup {
		_, dbmovietitleerr := database.GetDbmovieTitle(database.Query{Where: "dbmovie_id = ? and title = ?", WhereArgs: []interface{}{dbmovie.ID, titlegroup[idxtitle].Title}})
		if dbmovietitleerr != nil {
			database.InsertArray("dbmovie_titles", []string{"dbmovie_id", "title", "slug"}, []interface{}{dbmovie.ID, titlegroup[idxtitle].Title, titlegroup[idxtitle].Slug})
		}
	}
}

func checkifmovieyearmatches(entriesfound int, yearint int, movies []database.Movie, allowyear1 bool) (imdb string, movie database.Movie) {
	if entriesfound >= 1 && yearint != 0 {
		if entriesfound == 1 {
			dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
			if dbmovie.Year == yearint {
				imdb = dbmovie.ImdbID
				movie = movies[0]
				return
			}
			if allowyear1 {
				if dbmovie.Year == yearint+1 {
					imdb = dbmovie.ImdbID
					movie = movies[0]
					return
				}
				if dbmovie.Year == yearint-1 {
					imdb = dbmovie.ImdbID
					movie = movies[0]
					return
				}
			}
		} else {
			foundyear := 0
			foundyear1 := 0
			imdbyear := ""
			imdbyear1 := ""
			var movieyear database.Movie
			var movieyear1 database.Movie
			for idx := range movies {
				dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idx].DbmovieID}})
				if dbmovie.Year == yearint {
					imdbyear = dbmovie.ImdbID
					movieyear = movies[idx]
					foundyear += 1
				}
				if allowyear1 {
					if dbmovie.Year == yearint+1 {
						imdbyear1 = dbmovie.ImdbID
						movieyear1 = movies[idx]
						foundyear1 += 1
					}
					if dbmovie.Year == yearint-1 {
						imdbyear1 = dbmovie.ImdbID
						movieyear1 = movies[idx]
						foundyear1 += 1
					}
				}
			}
			if foundyear == 1 {
				return imdbyear, movieyear
			}
			if foundyear1 == 1 {
				return imdbyear1, movieyear1
			}
		}
	}
	return "", database.Movie{}
}

func movieCheckIfYear(dbmovies []database.Dbmovie, listname string, yearint int, allowyear1 bool) (bool, database.Movie, string, int) {
	for idx := range dbmovies {
		movies, _ := database.QueryMovies(database.Query{Where: "Dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovies[idx].ID, listname}})
		entriesfound := len(movies)
		imdb, movie := checkifmovieyearmatches(entriesfound, yearint, movies, allowyear1)
		if imdb != "" {
			entriesfound = 1
			logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", dbmovies[idx].Title)
			return true, movie, imdb, entriesfound
		}
	}
	return false, database.Movie{}, "", 0
}
func movieCheckAlternateIfYear(dbmovietitles []database.DbmovieTitle, listname string, yearint int, allowyear1 bool) (bool, database.Movie, string, int) {
	for idx := range dbmovietitles {
		movies, _ := database.QueryMovies(database.Query{Where: "Dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovietitles[idx].DbmovieID, listname}})
		entriesfound := len(movies)
		imdb, movie := checkifmovieyearmatches(entriesfound, yearint, movies, allowyear1)
		if imdb != "" {
			entriesfound = 1
			logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", dbmovietitles[idx].Title)
			return true, movie, imdb, entriesfound
		}
	}
	return false, database.Movie{}, "", 0
}
func movieFindDbByTitle(cfg config.Cfg, title string, year string, listname string, allowyear1 bool, searchtype string) (movie database.Movie, imdb string, entriesfound int) {
	searchfor := title
	yearint, _ := strconv.Atoi(year)
	slugged := logger.StringToSlug(title)

	dbmovies, _ := database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})
	found, foundmovie, foundimdb, foundentries := movieCheckIfYear(dbmovies, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmovietitles, _ := database.QueryDbmovieTitle(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})
	found, foundmovie, foundimdb, foundentries = movieCheckAlternateIfYear(dbmovietitles, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmovies, _ = database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{slugged}})
	found, foundmovie, foundimdb, foundentries = movieCheckIfYear(dbmovies, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmovietitles, _ = database.QueryDbmovieTitle(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{slugged}})
	found, foundmovie, foundimdb, foundentries = movieCheckAlternateIfYear(dbmovietitles, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	searchprovider := []string{"imdb", "tmdb", "omdb"}
	if strings.EqualFold(searchtype, "rss") {
		if len(cfg.General.MovieRSSMetaSourcePriority) >= 1 {
			searchprovider = cfg.General.MovieRSSMetaSourcePriority
		}
	} else {
		if len(cfg.General.MovieParseMetaSourcePriority) >= 1 {
			searchprovider = cfg.General.MovieParseMetaSourcePriority
		}
	}
	if len(searchprovider) >= 1 {
		for idxprovider := range searchprovider {
			if strings.EqualFold(searchprovider[idxprovider], "imdb") {
				if cfg.General.MovieMetaSourceImdb {
					//Search in Imdb
					imdbtitles, _ := database.QueryImdbTitle(database.Query{Select: "tconst,start_year", Where: "(primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", WhereArgs: []interface{}{title, title, slugged}})
					if len(imdbtitles) >= 1 {
						foundimdb := 0
						foundimdb1 := 0
						imdbloop := ""
						imdbloop1 := ""
						for idximdb := range imdbtitles {
							if yearint == 0 && len(imdbtitles) == 1 {
								imdbloop = imdbtitles[idximdb].Tconst
								foundimdb += 1
							}
							if imdbtitles[idximdb].StartYear == yearint {
								imdbloop = imdbtitles[idximdb].Tconst
								foundimdb += 1
							}
							if allowyear1 {
								if imdbtitles[idximdb].StartYear == yearint+1 {
									imdbloop1 = imdbtitles[idximdb].Tconst
									foundimdb1 += 1
								}
								if imdbtitles[idximdb].StartYear == yearint-1 {
									imdbloop1 = imdbtitles[idximdb].Tconst
									foundimdb1 += 1
								}
							}
						}
						if foundimdb == 1 {
							imdb = imdbloop
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
							entriesfound = len(movies)
							if entriesfound >= 1 {
								movie = movies[0]
							}
							logger.Log.Debug("Imdb Search (Year) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
						if foundimdb1 == 1 {
							imdb = imdbloop1
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
							entriesfound = len(movies)
							if entriesfound >= 1 {
								movie = movies[0]
							}
							logger.Log.Debug("Imdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}

					imdbaka, _ := database.QueryImdbAka(database.Query{Select: "distinct tconst", Where: "title = ? COLLATE NOCASE or slug = ?", WhereArgs: []interface{}{title, slugged}})
					if len(imdbaka) >= 1 {
						imdb = imdbaka[0].Tconst
						movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
						entriesfound = len(movies)
						if entriesfound >= 1 {
							movie = movies[0]
						}
						logger.Log.Debug("Imdb Aka Search succeded. Found Movies: ", entriesfound, " for ", title)
						return
					}
				}
			}
			if strings.EqualFold(searchprovider[idxprovider], "tmdb") {
				if cfg.General.MovieMetaSourceTmdb {
					getmovie, _ := apiexternal.TmdbApi.SearchMovie(searchfor)
					if len(getmovie.Results) >= 1 {
						foundtmdb := 0
						foundtmdb1 := 0
						tmdbloopid := 0
						tmdbloopid1 := 0
						for _, tmdbresult := range getmovie.Results {
							if yearint == 0 && len(getmovie.Results) == 1 {
								tmdbloopid = tmdbresult.ID
								foundtmdb += 1
							}
							if tmdbresult.ReleaseDate.Year() == yearint {
								tmdbloopid = tmdbresult.ID
								foundtmdb += 1
							}
							if allowyear1 {
								if tmdbresult.ReleaseDate.Year() == yearint+1 {
									tmdbloopid1 = tmdbresult.ID
									foundtmdb1 += 1
								}
								if tmdbresult.ReleaseDate.Year() == yearint-1 {
									tmdbloopid1 = tmdbresult.ID
									foundtmdb1 += 1
								}
							}
						}
						tmdbid := 0
						if foundtmdb == 1 {
							tmdbid = tmdbloopid
						} else if foundtmdb1 == 1 {
							tmdbid = tmdbloopid1
						}
						if tmdbid != 0 {
							moviedbexternal, foundexternal := apiexternal.TmdbApi.GetMovieExternal(tmdbid)
							//logger.Log.Debug("results moviedbexternalsearch. ", moviedbexternal)
							if foundexternal == nil {
								imdb = moviedbexternal.ImdbID
								movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
								entriesfound = len(movies)
								if entriesfound >= 1 {
									movie = movies[0]
								}
								logger.Log.Debug("Tmdb Search succeded. Found Movies: ", entriesfound, " for ", title)
								return
							} else {
								logger.Log.Error("MovieDB Externals Search failed: ", title)
							}
						}
					}
				}
			}
			if strings.EqualFold(searchprovider[idxprovider], "omdb") {
				if cfg.General.MovieMetaSourceOmdb {
					searchomdb, _ := apiexternal.OmdbApi.SearchMovie(title, "")
					if len(searchomdb.Search) >= 1 {
						foundomdb := 0
						foundomdb1 := 0
						omdbloop := ""
						omdbloop1 := ""
						for idxresult := range searchomdb.Search {
							if yearint == 0 && len(searchomdb.Search) == 1 {
								omdbloop = searchomdb.Search[idxresult].ImdbID
								foundomdb += 1
							}
							omdbyearint, _ := strconv.Atoi(searchomdb.Search[idxresult].Year)
							if omdbyearint == yearint {
								omdbloop = searchomdb.Search[idxresult].ImdbID
								foundomdb += 1
							}
							if allowyear1 {
								if omdbyearint == yearint+1 {
									omdbloop1 = searchomdb.Search[idxresult].ImdbID
									foundomdb1 += 1
								}
								if omdbyearint == yearint-1 {
									omdbloop1 = searchomdb.Search[idxresult].ImdbID
									foundomdb1 += 1
								}
							}
						}

						if foundomdb == 1 {
							imdb = omdbloop
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
							entriesfound = len(movies)
							if entriesfound >= 1 {
								movie = movies[0]
							}
							logger.Log.Debug("Omdb Search (Year) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
						if foundomdb1 == 1 {
							imdb = omdbloop1
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{imdb, listname}})
							entriesfound = len(movies)
							if entriesfound >= 1 {
								movie = movies[0]
							}
							logger.Log.Debug("Omdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}
				}
			}
		}
	}

	logger.Log.Debug("All Movie Lookups failed: ", title)
	return
}

func movieGetListFilter(lists []string, dbid uint, yearint int, allowyear1 bool) (imdb string, list string) {
	for idx := range lists {
		movies, _ := database.QueryMovies(database.Query{Where: "Dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbid, lists[idx]}})
		entriesfound := len(movies)
		imdb, _ = checkifmovieyearmatches(entriesfound, yearint, movies, allowyear1)
		if imdb != "" {
			entriesfound = 1
			list = lists[idx]
			break
		}
	}
	return
}
func movieFindListByTitle(cfg config.Cfg, title string, year string, lists []string, allowyear1 bool, searchtype string) (list string, imdb string, entriesfound int) {
	argslist := []interface{}{}
	for idx := range lists {
		argslist = append(argslist, lists[idx])
	}
	searchfor := title
	if year == "0" {
		year = ""
	}
	yearint, _ := strconv.Atoi(year)
	slugged := logger.StringToSlug(title)
	dbmovies, _ := database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})
	if len(dbmovies) >= 1 {
		for idx := range dbmovies {
			imdb, list = movieGetListFilter(lists, dbmovies[idx].ID, yearint, allowyear1)
			if imdb != "" && list != "" {
				entriesfound = 1
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}
	dbmovietitles, _ := database.QueryDbmovieTitle(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})

	if len(dbmovietitles) >= 1 {
		for idx := range dbmovietitles {
			imdb, list = movieGetListFilter(lists, dbmovietitles[idx].DbmovieID, yearint, allowyear1)
			if imdb != "" && list != "" {
				entriesfound = 1
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}

	dbmovies, _ = database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{slugged}})
	if len(dbmovies) >= 1 {
		for idx := range dbmovies {
			imdb, list = movieGetListFilter(lists, dbmovies[idx].ID, yearint, allowyear1)
			if imdb != "" && list != "" {
				entriesfound = 1
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}
	dbmovietitles, _ = database.QueryDbmovieTitle(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{slugged}})
	if len(dbmovietitles) >= 1 {
		for idx := range dbmovietitles {
			imdb, list = movieGetListFilter(lists, dbmovietitles[idx].DbmovieID, yearint, allowyear1)
			if imdb != "" && list != "" {
				entriesfound = 1
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}

	searchprovider := []string{"imdb", "tmdb", "omdb"}
	if strings.EqualFold(searchtype, "rss") {
		if len(cfg.General.MovieRSSMetaSourcePriority) >= 1 {
			searchprovider = cfg.General.MovieRSSMetaSourcePriority
		}
	} else {
		if len(cfg.General.MovieParseMetaSourcePriority) >= 1 {
			searchprovider = cfg.General.MovieParseMetaSourcePriority
		}
	}
	if len(searchprovider) >= 1 {
		for idxprovider := range searchprovider {
			if strings.EqualFold(searchprovider[idxprovider], "imdb") {
				if cfg.General.MovieMetaSourceImdb {
					//Search in Imdb
					imdbtitles, _ := database.QueryImdbTitle(database.Query{Select: "tconst,start_year", Where: "(primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", WhereArgs: []interface{}{title, title, slugged}})
					if len(imdbtitles) >= 1 {
						foundimdb := 0
						foundimdb1 := 0
						imdbloop := ""
						imdbloop1 := ""
						for idximdb := range imdbtitles {
							if yearint == 0 && len(imdbtitles) == 1 {
								imdbloop = imdbtitles[idximdb].Tconst
								foundimdb += 1
							}
							if imdbtitles[idximdb].StartYear == yearint {
								imdbloop = imdbtitles[idximdb].Tconst
								foundimdb += 1
							}
							if allowyear1 {
								if imdbtitles[idximdb].StartYear == yearint+1 {
									imdbloop1 = imdbtitles[idximdb].Tconst
									foundimdb1 += 1
								}
								if imdbtitles[idximdb].StartYear == yearint-1 {
									imdbloop1 = imdbtitles[idximdb].Tconst
									foundimdb1 += 1
								}
							}
						}
						if foundimdb == 1 {
							imdb = imdbloop
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							if len(movies) >= 1 {
								list = movies[0].Listname
							}
							entriesfound = len(movies)
							logger.Log.Debug("Imdb Search (Year) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
						if foundimdb1 == 1 {
							imdb = imdbloop1
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							if len(movies) >= 1 {
								list = movies[0].Listname
							}
							entriesfound = len(movies)
							logger.Log.Debug("Imdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}

					imdbaka, _ := database.QueryImdbAka(database.Query{Select: "distinct tconst", Where: "title = ? COLLATE NOCASE or slug = ?", WhereArgs: []interface{}{title, slugged}})
					if len(imdbaka) == 1 {
						imdb = imdbaka[0].Tconst
						argsimdb := []interface{}{}
						argsimdb = append(argsimdb, imdb)
						argsimdb = append(argsimdb, argslist...)
						movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
						if len(movies) >= 1 {
							list = movies[0].Listname
						}
						entriesfound = len(movies)

						logger.Log.Debug("Imdb Aka Search succeded. Found Movies: ", entriesfound, " for ", title)
						return
					}
				}
			}
			if strings.EqualFold(searchprovider[idxprovider], "tmdb") {
				if cfg.General.MovieMetaSourceTmdb {
					getmovie, _ := apiexternal.TmdbApi.SearchMovie(searchfor + " " + year)
					if len(getmovie.Results) >= 1 {
						foundtmdb := 0
						foundtmdb1 := 0
						tmdbloopid := 0
						tmdbloopid1 := 0
						for _, tmdbresult := range getmovie.Results {
							if yearint == 0 && len(getmovie.Results) == 1 {
								tmdbloopid = tmdbresult.ID
								foundtmdb += 1
							}
							if tmdbresult.ReleaseDate.Year() == yearint {
								tmdbloopid = tmdbresult.ID
								foundtmdb += 1
							}
							if allowyear1 {
								if tmdbresult.ReleaseDate.Year() == yearint+1 {
									tmdbloopid1 = tmdbresult.ID
									foundtmdb1 += 1
								}
								if tmdbresult.ReleaseDate.Year() == yearint-1 {
									tmdbloopid1 = tmdbresult.ID
									foundtmdb1 += 1
								}
							}
						}
						tmdbid := 0
						if foundtmdb == 1 {
							tmdbid = tmdbloopid
						} else if foundtmdb1 == 1 {
							tmdbid = tmdbloopid1
						}
						if tmdbid != 0 {
							moviedbexternal, foundexternal := apiexternal.TmdbApi.GetMovieExternal(tmdbid)
							//logger.Log.Debug("results moviedbexternalsearch. ", moviedbexternal)
							if foundexternal == nil {
								imdb = moviedbexternal.ImdbID
								argsimdb := []interface{}{}
								argsimdb = append(argsimdb, imdb)
								argsimdb = append(argsimdb, argslist...)
								movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
								if len(movies) >= 1 {
									list = movies[0].Listname
								}
								entriesfound = len(movies)
								logger.Log.Debug("Tmdb Search succeded. Found Movies: ", entriesfound, " for ", title)
								return
							} else {
								logger.Log.Error("MovieDB Externals Search failed: ", title)
							}
						}
					}
				}
			}
			if strings.EqualFold(searchprovider[idxprovider], "omdb") {
				if cfg.General.MovieMetaSourceOmdb {
					searchomdb, _ := apiexternal.OmdbApi.SearchMovie(title, year)
					if len(searchomdb.Search) >= 1 {
						foundomdb := 0
						foundomdb1 := 0
						omdbloop := ""
						omdbloop1 := ""
						for idxresult := range searchomdb.Search {
							if yearint == 0 && len(searchomdb.Search) == 1 {
								omdbloop = searchomdb.Search[idxresult].ImdbID
								foundomdb += 1
							}
							omdbyearint, _ := strconv.Atoi(searchomdb.Search[idxresult].Year)
							if omdbyearint == yearint {
								omdbloop = searchomdb.Search[idxresult].ImdbID
								foundomdb += 1
							}
							if allowyear1 {
								if omdbyearint == yearint+1 {
									omdbloop1 = searchomdb.Search[idxresult].ImdbID
									foundomdb1 += 1
								}
								if omdbyearint == yearint-1 {
									omdbloop1 = searchomdb.Search[idxresult].ImdbID
									foundomdb1 += 1
								}
							}
						}

						if foundomdb == 1 {
							imdb = omdbloop
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							if len(movies) >= 1 {
								list = movies[0].Listname
							}
							entriesfound = len(movies)
							logger.Log.Debug("Omdb Search (Year) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
						if foundomdb1 == 1 {
							imdb = omdbloop1
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							if len(movies) >= 1 {
								list = movies[0].Listname
							}
							entriesfound = len(movies)
							logger.Log.Debug("Omdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}
				}
			}
		}
	}

	logger.Log.Debug("All Movie Lookups failed: ", title)
	return
}

func JobImportMovieParseV2(cfg config.Cfg, file string, configEntry config.MediaTypeConfig, list config.MediaListsConfig, updatemissing bool, minPrio ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	movie, movieerr := database.GetMovies(database.Query{Select: "movies.*", InnerJoin: "movie_files ON Movies.ID = movie_files.movie_id", Where: "movie_files.location = ? and movies.listname = ?", WhereArgs: []interface{}{file, list.Name}})
	if movieerr == nil {
		for idxignore := range list.Ignore_map_lists {
			countermi, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{movie.DbmovieID, list.Ignore_map_lists[idxignore]}})
			if countermi >= 1 {
				return
			}
		}
	}

	parsetest, _ := database.QueryMovieFileUnmatched(database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
	if len(parsetest) >= 1 {
		return
	}
	logger.Log.Debug("Parse Movie: ", file)

	m, err := NewFileParser(cfg, filepath.Base(file), false, "movie")

	addunmatched := false
	if err == nil {
		m.Title = strings.Trim(m.Title, " ")
		for idxstrip := range cfg.Quality[list.Template_quality].TitleStripSuffixForSearch {
			if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(cfg.Quality[list.Template_quality].TitleStripSuffixForSearch[idxstrip])) {
				m.Title = trimStringInclAfterStringInsensitive(m.Title, cfg.Quality[list.Template_quality].TitleStripSuffixForSearch[idxstrip])
				m.Title = strings.Trim(m.Title, " ")
			}
		}
		m.Resolution = strings.ToLower(m.Resolution)
		m.Audio = strings.ToLower(m.Audio)
		m.Codec = strings.ToLower(m.Codec)
		m.Quality = strings.ToLower(m.Quality)
		logger.Log.Debug("Parsed Movie: ", file, " as ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

		entriesfound := 0
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
			entriesfound = len(movies)
			if len(movies) == 1 {
				movie = movies[0]
			}
		}
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ?", WhereArgs: []interface{}{m.Imdb}})
			if len(movies) >= 1 {
				return
			}
		}
		if entriesfound == 0 {
			getmovie, imdb, entriesfound := movieFindDbByTitle(cfg, m.Title, strconv.Itoa(m.Year), list.Name, cfg.Quality[list.Template_quality].CheckYear1, "parse")
			if entriesfound >= 1 {
				m.Imdb = imdb
				movie = getmovie
			}
		}

		if movie.ID == 0 {
			if list.Addfound {
				if len(m.Imdb) >= 1 {
					sww := sizedwaitgroup.New(1)
					var dbmovie database.Dbmovie
					dbmovie.ImdbID = m.Imdb
					sww.Add()
					JobImportMovies(cfg, dbmovie, configEntry, list, &sww)
					sww.Wait()
					movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
					if len(movies) == 1 {
						movie = movies[0]
					}
				}
			}
		}
		if movie.ID >= 1 {
			cutoffPrio := NewCutoffPrio(cfg, configEntry, cfg.Quality[list.Template_quality])

			m.GetPriority(configEntry, cfg.Quality[list.Template_quality])
			errparsev := m.ParseVideoFile(file, configEntry, cfg.Quality[list.Template_quality])
			if errparsev != nil {
				return
			}
			counterf, _ := database.CountRows("movie_files", database.Query{Where: "location = ? AND movie_id = ?", WhereArgs: []interface{}{file, movie.ID}})
			if counterf == 0 {
				reached := false
				if m.Priority >= cutoffPrio.Priority {
					reached = true
				}

				if movie.Rootpath == "" && movie.ID != 0 {
					rootpath := ""
					for idxpath := range configEntry.Data {
						pppath := cfg.Path[configEntry.Data[idxpath].Template_path].Path
						if strings.Contains(file, pppath) {
							rootpath = pppath
							tempfoldername := strings.Replace(file, pppath, "", -1)
							tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
							tempfoldername = filepath.Dir(tempfoldername)
							_, firstfolder := getrootpath(tempfoldername)
							rootpath = filepath.Join(rootpath, firstfolder)
							break
						}
					}
					database.UpdateColumn("movies", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
				}

				database.InsertArray("movie_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id"},
					[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID})
				if updatemissing {
					database.UpdateColumn("movies", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
					database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
				}

				database.DeleteRow("movie_file_unmatcheds", database.Query{Where: "filepath = ?", WhereArgs: []interface{}{file}})
			}
		} else {
			addunmatched = true
			logger.Log.Error("Movie Parse failed - not matched: ", file)
		}
	} else {
		addunmatched = true
		logger.Log.Error("Movie Parse failed: ", file)
	}

	if addunmatched {
		mjson, _ := json.Marshal(m)
		valuesupsert := make(map[string]interface{})
		valuesupsert["listname"] = list.Name
		valuesupsert["filepath"] = file
		valuesupsert["last_checked"] = time.Now()
		valuesupsert["parsed_data"] = string(mjson)
		database.Upsert("movie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})

	}
}

func readCSVFromURL(url string) ([][]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		logger.Log.Error("Failed to get CSV from: ", url)
		return nil, err
	}

	defer resp.Body.Close()
	reader := csv.NewReader(resp.Body)
	//reader.Comma = ';'
	data, err := reader.ReadAll()
	if err != nil {
		logger.Log.Error("Failed to read CSV from: ", url)
		return nil, err
	}

	return data, nil
}

func getMissingIMDBMoviesV2(cfg config.Cfg, configEntry config.MediaTypeConfig, list config.MediaListsConfig) []database.Dbmovie {
	if !list.Enabled {
		return []database.Dbmovie{}
	}
	if len(cfg.List[list.Template_list].Url) >= 1 {
		data, err := readCSVFromURL(cfg.List[list.Template_list].Url)
		if err != nil {
			logger.Log.Error("Failed to read CSV from: ", cfg.List[list.Template_list].Url)
			return []database.Dbmovie{}
		}
		d := make([]database.Dbmovie, 0, len(data))

		for idx := range data {
			// skip header
			if idx == 0 {
				continue
			}

			if len(cfg.List[list.Template_list].Excludegenre) >= 1 {
				excludebygenre := false
				for idxgenre := range cfg.List[list.Template_list].Excludegenre {
					countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? and genre = ?", WhereArgs: []interface{}{data[idx][1], cfg.List[list.Template_list].Excludegenre[idxgenre]}})
					if countergenre >= 1 {
						excludebygenre = true
						break
					}
				}
				if excludebygenre {
					continue
				}
			}
			if len(cfg.List[list.Template_list].Includegenre) >= 1 {
				includebygenre := false
				for idxgenre := range cfg.List[list.Template_list].Includegenre {
					countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? and genre = ?", WhereArgs: []interface{}{data[idx][1], cfg.List[list.Template_list].Includegenre[idxgenre]}})
					if countergenre >= 1 {
						includebygenre = true
						break
					}
				}
				if !includebygenre {
					continue
				}
			}
			year, _ := strconv.ParseInt(data[idx][10], 0, 64)
			votes, _ := strconv.ParseInt(data[idx][12], 0, 64)
			voteavg, _ := strconv.ParseFloat(data[idx][8], 64)
			d = append(d, database.Dbmovie{ImdbID: data[idx][1], Title: data[idx][5], URL: data[idx][6], VoteAverage: float32(voteavg), Year: int(year), VoteCount: int(votes)})
		}
		return d
	}
	return []database.Dbmovie{}
}
