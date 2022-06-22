package importfeed

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func JobImportMovies(imdbid string, configTemplate string, listConfig string) {
	jobName := imdbid
	if jobName == "" {
		jobName = listConfig
	}
	if importJobRunning.Contains(jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning.Add(jobName)
		defer importJobRunning.Remove(jobName)
	}

	cdbmovie, errdbmovie := database.CountRowsStatic("Select count(id) from dbmovies where imdb_id = ?", imdbid)
	if errdbmovie != nil {
		return
	}
	var dbmovie database.Dbmovie
	dbmovie.ImdbID = imdbid
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if cdbmovie == 0 {
		logger.Log.Debug("Get Movie Metadata: ", imdbid)

		if !config.ConfigCheck("general") {
			return
		}
		if len(cfg_general.MovieMetaSourcePriority) >= 1 {
			for idxmeta := range cfg_general.MovieMetaSourcePriority {
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "imdb") {
					logger.Log.Debug("Get Movie Metadata - imdb: ", imdbid)
					dbmovie.GetImdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "tmdb") {
					logger.Log.Debug("Get Movie Metadata - tmdb: ", imdbid)
					dbmovie.GetTmdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "omdb") {
					logger.Log.Debug("Get Movie Metadata - omdb: ", imdbid)
					dbmovie.GetOmdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "trakt") {
					logger.Log.Debug("Get Movie Metadata - trakt: ", imdbid)
					dbmovie.GetTraktMetadata(false)
				}
			}
		} else {
			dbmovie.GetMetadata(cfg_general.MovieMetaSourceImdb, cfg_general.MovieMetaSourceTmdb, cfg_general.MovieMetaSourceOmdb, cfg_general.MovieMetaSourceTrakt)
		}

		if list.Name == "" {
			return
		}
		if !config.ConfigCheck("list_" + list.Template_list) {
			return
		}
		if !AllowMovieImport(dbmovie.ImdbID, list.Template_list) {
			return
		}

		cdbmovie2, errdbmovie := database.CountRowsStatic("Select count(id) from dbmovies where imdb_id = ?", dbmovie.ImdbID)
		if errdbmovie != nil {
			return
		}
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
			titles, _ := database.QueryStaticColumnsOneString("select title from dbmovie_titles where dbmovie_id = ?", "select count(id) from dbmovie_titles where dbmovie_id = ?", dbmovie.ID)
			defer logger.ClearVar(&titles)

			titlegroup := dbmovie.GetTitles(configTemplate, cfg_general.MovieAlternateTitleMetaSourceImdb, cfg_general.MovieAlternateTitleMetaSourceTmdb, cfg_general.MovieAlternateTitleMetaSourceTrakt)
			defer logger.ClearVar(&titlegroup)
			var titlefound bool
			for idxtitle := range titlegroup {
				if titlegroup[idxtitle].Title == "" {
					continue
				}
				titlefound = false
				for idxtitleall := range titles {
					if strings.EqualFold(titles[idxtitleall].Str, titlegroup[idxtitle].Title) {
						titlefound = true
						break
					}
				}
				if !titlefound {
					database.InsertArray("dbmovie_titles", []string{"dbmovie_id", "title", "slug", "region"}, []interface{}{dbmovie.ID, titlegroup[idxtitle].Title, titlegroup[idxtitle].Slug, titlegroup[idxtitle].Region})
				}
			}
		} else {
			id, err := database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", dbmovie.ImdbID)
			if err == nil {
				dbmovie = database.Dbmovie{ID: id}
			}
		}
	} else {
		if dbmovie.ID == 0 {
			id, err := database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", dbmovie.ImdbID)
			if err == nil {
				dbmovie = database.Dbmovie{ID: id}
			}
		}
	}
	movietest, _ := database.QueryStaticColumnsOneStringOneInt("select listname, id from movies where dbmovie_id = ?", "select count(id) from movies where dbmovie_id = ?", dbmovie.ID)
	defer logger.ClearVar(&movietest)
	if len(list.Ignore_map_lists) >= 1 {
		var counter int
		for idx := range list.Ignore_map_lists {
			counter, _ = database.CountRowsStatic("Select count(id) from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", list.Ignore_map_lists[idx], dbmovie.ID)
			if counter >= 1 {
				return
			}
		}
	}

	foundmovie := false
	counter, errsearch := database.CountRowsStatic("Select count(id) from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", listConfig, dbmovie.ID)
	if errsearch != nil {
		return
	}
	if counter >= 1 {
		foundmovie = true
	}

	if foundmovie {
		for idxreplace := range list.Replace_map_lists {
			for idxtitle := range movietest {
				if strings.EqualFold(movietest[idxtitle].Str, list.Replace_map_lists[idxreplace]) {
					database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality}, database.Query{Where: "id = ?", WhereArgs: []interface{}{movietest[idxtitle].Num}})
				}
			}
		}
	} else {
		logger.Log.Infoln("Insert Movie: ", dbmovie.ImdbID)
		_, moviereserr := database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality})
		if moviereserr != nil {
			logger.Log.Error(moviereserr)
			return
		}
		for idxreplace := range list.Replace_map_lists {
			for idxtitle := range movietest {
				if strings.EqualFold(movietest[idxtitle].Str, list.Replace_map_lists[idxreplace]) {
					database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality}, database.Query{Where: "id = ?", WhereArgs: []interface{}{movietest[idxtitle].Num}})
				}
			}
		}
	}
}

func AllowMovieImport(imdb string, listTemplate string) bool {
	if !config.ConfigCheck("list_" + listTemplate) {
		return false
	}
	list := config.ConfigGet("list_" + listTemplate).Data.(config.ListsConfig)

	if list.MinVotes != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_ratings where tconst = ? and num_votes < ?", imdb, list.MinVotes)
		if countergenre >= 1 {
			logger.Log.Warningln("error vote count too low for", imdb)
			return false
		}
	}
	if list.MinRating != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_ratings where tconst = ? and average_rating < ?", imdb, list.MinRating)
		if countergenre >= 1 {
			logger.Log.Warningln("error average vote too low for", imdb)
			return false
		}
	}
	if len(list.Excludegenre) >= 1 || len(list.Includegenre) >= 1 {
		genres, _ := database.QueryImdbStaticColumnsOneString("Select genre from imdb_genres where tconst = ?", "Select count(id) from imdb_genres where tconst = ?", imdb)
		defer logger.ClearVar(&genres)
		genrearr := make([]string, 0, len(genres))
		defer logger.ClearVar(&genrearr)
		for idxgenre := range genres {
			genrearr = append(genrearr, strings.ToLower(genres[idxgenre].Str))
		}
		if len(list.Excludegenre) >= 1 {
			excludebygenre := false
			var foundentry bool
			for idxgenre := range list.Excludegenre {
				foundentry = false
				for idxgenre2 := range genrearr {
					if genrearr[idxgenre2] == strings.ToLower(list.Excludegenre[idxgenre]) {
						foundentry = true
						break
					}
				}
				if foundentry {
					excludebygenre = true
					logger.Log.Warningln("error excluded genre", list.Excludegenre[idxgenre], imdb)
					break
				}
			}
			if excludebygenre {
				return false
			}
		}
		if len(list.Includegenre) >= 1 {
			includebygenre := false
			var foundentry bool
			for idxgenre := range list.Includegenre {
				foundentry = false
				for idxgenre2 := range genrearr {
					if genrearr[idxgenre2] == strings.ToLower(list.Includegenre[idxgenre]) {
						foundentry = true
						break
					}
				}
				if foundentry {
					includebygenre = true
					logger.Log.Warningln("error excluded genre", list.Excludegenre[idxgenre], imdb)
					break
				}
			}
			if !includebygenre {
				logger.Log.Warningln("error included genre not found", list.Includegenre, imdb)
				return false
			}
		}
	}
	return true
}
func JobReloadMovies(imdb string, configTemplate string, listConfig string) {
	jobName := imdb
	if jobName == "" {
		jobName = listConfig
	}
	if importJobRunning.Contains(jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning.Add(jobName)
		defer importJobRunning.Remove(jobName)
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.SchedulerDisabled {
		return
	}

	logger.Log.Debug("Get Movie Metadata: ", imdb)
	dbmovie, _ := database.GetDbmovie(database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{imdb}})
	defer logger.ClearVar(&dbmovie)

	if len(cfg_general.MovieMetaSourcePriority) >= 1 {
		for idxmeta := range cfg_general.MovieMetaSourcePriority {
			if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "imdb") {
				logger.Log.Debug("Get Movie Titles - imdb: ", dbmovie.Title)
				dbmovie.GetImdbMetadata(false)
			}
			if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "tmdb") {
				logger.Log.Debug("Get Movie Titles - tmdb: ", dbmovie.Title)
				dbmovie.GetTmdbMetadata(false)
			}
			if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "omdb") {
				logger.Log.Debug("Get Movie Titles - omdb: ", dbmovie.Title)
				dbmovie.GetOmdbMetadata(false)
			}
			if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "trakt") {
				logger.Log.Debug("Get Movie Titles - trakt: ", dbmovie.Title)
				dbmovie.GetTraktMetadata(false)
			}
		}
	} else {
		dbmovie.GetMetadata(cfg_general.MovieMetaSourceImdb, cfg_general.MovieMetaSourceTmdb, cfg_general.MovieMetaSourceOmdb, cfg_general.MovieMetaSourceTrakt)
	}
	database.UpdateArray("dbmovies",
		[]string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
		[]interface{}{dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug},
		database.Query{Where: "id = ?", WhereArgs: []interface{}{dbmovie.ID}})

	movies, _ := database.QueryStaticColumnsOneString("select listname from movies where dbmovie_id = ?", "select count(id) from movies where dbmovie_id = ?", dbmovie.ID)
	defer logger.ClearVar(&movies)

	var getconfigentry config.MediaTypeConfig //:= config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	defer logger.ClearVar(&getconfigentry)

	if len(movies) >= 1 {
		var listfound bool
		var cfg_movie config.MediaTypeConfig
		defer logger.ClearVar(&cfg_movie)
		for _, idx := range config.ConfigGetPrefix("movie_") {
			cfg_movie = config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound = false
			for idxlist := range cfg_movie.Lists {
				if cfg_movie.Lists[idxlist].Name == movies[0].Str {
					listfound = true
					getconfigentry = cfg_movie
					break
				}
			}
			if listfound {
				break
			}
		}
	}
	if getconfigentry.Name == "" {
		return
	}
	logger.Log.Debug("Get Movie Titles: ", dbmovie.Title)
	titles, _ := database.QueryStaticColumnsOneString("select title from dbmovie_titles where dbmovie_id = ?", "select count(id) from dbmovie_titles where dbmovie_id = ?", dbmovie.ID)
	defer logger.ClearVar(&titles)
	titlegroup := dbmovie.GetTitles("movie_"+getconfigentry.Name, cfg_general.MovieAlternateTitleMetaSourceImdb, cfg_general.MovieAlternateTitleMetaSourceTmdb, cfg_general.MovieAlternateTitleMetaSourceTrakt)
	defer logger.ClearVar(&titlegroup)
	var titlefound bool
	for idxtitle := range titlegroup {
		if titlegroup[idxtitle].Title == "" {
			continue
		}
		titlefound = false
		for idxtitleall := range titles {
			if strings.EqualFold(titles[idxtitleall].Str, titlegroup[idxtitle].Title) {
				titlefound = true
				break
			}
		}
		if !titlefound {
			database.InsertArray("dbmovie_titles", []string{"dbmovie_id", "title", "slug", "region"}, []interface{}{dbmovie.ID, titlegroup[idxtitle].Title, titlegroup[idxtitle].Slug, titlegroup[idxtitle].Region})
		}
	}
}

func checkifdbmovieyearmatches(dbmovieyear int, haveyear int) (bool, bool) {
	if dbmovieyear != 0 && haveyear != 0 {
		if dbmovieyear == haveyear {
			return true, false
		}

		if dbmovieyear == haveyear+1 || dbmovieyear == haveyear-1 {

			return false, true
		}
	}
	return false, false
}

func findimdbbytitle(title string, slugged string, yearint int) (imdb string, found bool, found1 bool) {
	var qualityTemplate string

	imdbtitles, _ := database.QueryImdbStaticColumnsOneStringOneInt("select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", "select count(tconst) from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", title, title, slugged)
	defer logger.ClearVar(&imdbtitles)

	if len(imdbtitles) >= 1 {
		for idximdb := range imdbtitles {
			found, found1 = checkifdbmovieyearmatches(imdbtitles[idximdb].Num, yearint)
			if found || found1 {
				if found {
					return imdbtitles[idximdb].Str, found, found1
				}
				if found1 {
					qualityTemplate, _ = database.QueryColumnString("Select movies.quality_profile from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id = ? limit 1", imdbtitles[idximdb].Str)
					if qualityTemplate != "" {
						qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
						if qualityconfig.CheckYear1 {
							return imdbtitles[idximdb].Str, found, found1
						}
					}
				}
			}
		}
	}

	imdbaka, _ := database.QueryImdbStaticColumnsOneString("select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?", "select count(tconst) from imdb_akas where title = ? COLLATE NOCASE or slug = ?", title, slugged)
	defer logger.ClearVar(&imdbaka)

	var dbyear uint
	for idximdb := range imdbaka {
		dbyear, _ = database.QueryImdbColumnUint("Select start_year from imdb_titles where tconst = ?", imdbaka[idximdb].Str)
		found, found1 = checkifdbmovieyearmatches(int(dbyear), yearint)
		if found || found1 {
			imdb = imdbaka[idximdb].Str
			if found || found1 {
				if found {
					return imdb, found, found1
				}
				if found1 {
					qualityTemplate, _ = database.QueryColumnString("Select movies.quality_profile from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id = ? limit 1", imdb)
					if qualityTemplate != "" {
						qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
						if qualityconfig.CheckYear1 {
							return imdb, found, found1
						}
					}
				}
			}
		}
	}

	return
}

func findtmdbbytitle(title string, slugged string, yearint int) (imdb string, found bool, found1 bool) {
	getmovie, _ := apiexternal.TmdbApi.SearchMovie(title)

	defer logger.ClearVar(&getmovie)

	if len(getmovie.Results) >= 1 {
		var dbyear uint
		for idx2 := range getmovie.Results {
			moviedbexternal, err := apiexternal.TmdbApi.GetMovieExternal(getmovie.Results[idx2].ID)
			if err == nil {
				dbyear, _ = database.QueryImdbColumnUint("Select start_year from imdb_titles where tconst = ?", moviedbexternal.ImdbID)
				found, found1 = checkifdbmovieyearmatches(int(dbyear), yearint)
				if found || found1 {
					imdb = moviedbexternal.ImdbID
					return
				}
			}
		}
	}

	return
}

func findomdbbytitle(title string, slugged string, yearint int) (imdb string, found bool, found1 bool) {
	searchomdb, _ := apiexternal.OmdbApi.SearchMovie(title, "")
	defer logger.ClearVar(&searchomdb)

	if len(searchomdb.Search) >= 1 {
		var dbyear int
		for idximdb := range searchomdb.Search {
			dbyear, _ = strconv.Atoi(searchomdb.Search[idximdb].Year)
			found, found1 = checkifdbmovieyearmatches(dbyear, yearint)
			if found || found1 {
				imdb = searchomdb.Search[idximdb].ImdbID
				return
			}
		}
	}
	return
}

func MovieFindDbIdByTitle(title string, year string, searchtype string, addifnotfound bool) (uint, bool, bool) {

	if !config.ConfigCheck("general") {
		return 0, false, false
	}
	imdb, imdbfound, imdbfound1 := MovieFindImdbIDByTitle(title, year, searchtype, addifnotfound)
	if !imdbfound && !imdbfound1 {
		logger.Log.Warningln("All Movie Lookups failed: ", title)
		return 0, false, false
	}
	dbid, dbiderr := database.QueryColumnUint("select id from dbmovies where imdb_id = ?", imdb)
	if dbiderr != nil {
		logger.Log.Warningln("All Movie Lookups failed: ", title)
		return 0, false, false
	}
	return dbid, imdbfound, imdbfound1
}

func MovieFindImdbIDByTitle(title string, year string, searchtype string, addifnotfound bool) (string, bool, bool) {

	if !config.ConfigCheck("general") {
		return "", false, false
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	yearint, _ := strconv.Atoi(year)

	slugged := logger.StringToSlug(title)
	dbmoviestemp, _ := database.QueryStaticColumnsOneStringOneInt("select imdb_id, year from dbmovies where title = ? COLLATE NOCASE OR slug = ?", "select count(id) from dbmovies where title = ? COLLATE NOCASE OR slug = ?", title, slugged)
	defer logger.ClearVar(&dbmoviestemp)
	var found, found1 bool

	for idx := range dbmoviestemp {
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, yearint)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}

	dbmoviestemp, _ = database.QueryStaticColumnsOneStringOneInt("select dbmovies.imdb_id, dbmovies.year from dbmovie_titles inner join dbmovies on dbmovies.id=dbmovie_titles.dbmovie_id where dbmovie_titles.title = ? COLLATE NOCASE OR dbmovie_titles.slug = ?", "select count(dbmovie_titles.id) from dbmovie_titles inner join dbmovies on dbmovies.id=dbmovie_titles.dbmovie_id where dbmovie_titles.title = ? COLLATE NOCASE OR dbmovie_titles.slug = ?", title, slugged)

	for idx := range dbmoviestemp {
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, yearint)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}
	if addifnotfound {
		searchprovider := []string{"imdb", "tmdb", "omdb"}
		defer logger.ClearVar(&searchprovider)
		if strings.EqualFold(searchtype, "rss") {
			if len(cfg_general.MovieRSSMetaSourcePriority) >= 1 {
				searchprovider = cfg_general.MovieRSSMetaSourcePriority
			}
		} else {
			if len(cfg_general.MovieParseMetaSourcePriority) >= 1 {
				searchprovider = cfg_general.MovieParseMetaSourcePriority
			}
		}
		if len(searchprovider) >= 1 {
			var imdb string
			for idxprovider := range searchprovider {
				if strings.EqualFold(searchprovider[idxprovider], "imdb") {
					if cfg_general.MovieMetaSourceImdb {
						imdb, found, found1 = findimdbbytitle(title, slugged, yearint)
						if found || found1 {
							return imdb, found, found1
						}
					}
				}
				if strings.EqualFold(searchprovider[idxprovider], "tmdb") {
					if cfg_general.MovieMetaSourceTmdb {
						imdb, found, found1 = findtmdbbytitle(title, slugged, yearint)
						if found || found1 {
							return imdb, found, found1
						}
					}
				}
				if strings.EqualFold(searchprovider[idxprovider], "omdb") {
					if cfg_general.MovieMetaSourceOmdb {
						imdb, found, found1 = findomdbbytitle(title, slugged, yearint)
						if found || found1 {
							return imdb, found, found1
						}
					}
				}
			}
		}
	}

	logger.Log.Warningln("All Movie Lookups failed: ", title)
	return "", false, false
}

func MovieGetListFilter(configTemplate string, dbid int, yearint int) (imdb string, list string) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	var movies []database.Dbstatic_OneInt
	defer logger.ClearVar(&movies)

	var found, found1 bool

	foundmovies, _ := database.QueryStaticColumnsOneStringOneInt("Select listname, id from movies where dbmovie_id = ?", "Select count(id) from movies where dbmovie_id = ?", dbid)
	defer logger.ClearVar(&foundmovies)
	for listtestidx := range configEntry.Lists {
		for listfoundidx := range foundmovies {
			if configEntry.Lists[listtestidx].Name == foundmovies[listfoundidx].Str {
				found, found1 = checkifdbmovieyearmatches(foundmovies[listfoundidx].Num, yearint)
				if found || found1 {
					imdb, _ = database.QueryColumnString("Select imdb_id from dbmovies where id = ?", dbid)
					list = foundmovies[listfoundidx].Str
					return
				}
			}
		}
	}
	return
}

func FindDbserieByName(title string) (uint, error) {
	logger.Log.Debug("Find Serie by Name", title)
	id, err := database.QueryColumnUint("Select id from dbseries where Seriename = ? COLLATE NOCASE", title)
	if err != nil {
		slugged := logger.StringToSlug(title)
		id, err = database.QueryColumnUint("Select id from dbseries where Slug = ?", slugged)
		if err != nil {
			id, err = database.QueryColumnUint("Select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE", title)
			if err != nil {
				id, err = database.QueryColumnUint("Select dbserie_id from Dbserie_alternates where Slug = ?", slugged)
			}
		}
	}
	return id, err
}
func FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid uint, identifier string, season string, episode string) (uint, error) {
	var id uint
	var err error
	if len(season) >= 1 && len(episode) >= 1 {
		id, err = database.QueryColumnUint("Select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?", dbserieid, season, episode)
		if err == nil {
			return id, err
		}
	}
	if len(identifier) >= 1 {
		id, err = database.QueryColumnUint("Select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", dbserieid, identifier)
		if err == nil {
			return id, err
		}
		if strings.Contains(identifier, ".") {
			id, err = database.QueryColumnUint("Select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", dbserieid, strings.Replace(identifier, ".", "-", -1))
			if err == nil {
				return id, err
			}
		}
	}
	return 0, errors.New("nothing found")
}
func GetEpisodeArray(identifiedby string, identifier string) []string {
	teststr := config.RegexGet("RegexSeriesIdentifier").FindStringSubmatch(identifier)
	defer logger.ClearVar(&teststr)
	if len(teststr) == 0 {
		return []string{}
	}
	var str1, str2 string
	str1 = teststr[1]
	str2 = teststr[2]
	var episodeArray []string
	defer logger.ClearVar(&episodeArray)
	if strings.EqualFold(identifiedby, "date") {
		str1 = str2
		str1 = strings.Replace(str1, " ", "-", -1)
		str1 = strings.Replace(str1, ".", "-", -1)
	}
	str1 = strings.ToUpper(str1)
	if strings.Contains(str1, "E") {
		episodeArray = strings.Split(str1, "E")
	} else if strings.Contains(str1, "X") {
		episodeArray = strings.Split(str1, "X")
	} else if strings.Contains(str1, "-") && !strings.EqualFold(identifiedby, "date") {
		episodeArray = strings.Split(str1, "-")
	}
	if len(episodeArray) == 0 && strings.EqualFold(identifiedby, "date") {
		episodeArray = append(episodeArray, str1)
	}
	return episodeArray
}

var importJobRunning logger.StringSet

func JobImportDbSeries(serieconfig config.SerieConfig, configTemplate string, listConfig string, checkall bool) {
	jobName := serieconfig.Name
	if jobName == "" {
		jobName = listConfig
	}

	if importJobRunning.Contains(jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning.Add(jobName)
		defer importJobRunning.Remove(jobName)
	}

	var dbserie database.Dbserie
	dbserieadded := false

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.EqualFold(serieconfig.Source, "none") {
		dbserie.Seriename = serieconfig.Name
		dbserie.Identifiedby = serieconfig.Identifiedby

		cdbserie, dbseriesearcherr := database.CountRowsStatic("Select count(id) from dbseries where Seriename = ? COLLATE NOCASE", serieconfig.Name)
		if dbseriesearcherr != nil {
			return
		}
		if cdbserie == 0 {
			dbserieadded = true
			inres, inreserr := database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
				[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
			if inreserr != nil {
				logger.Log.Error(inreserr)
				return
			}
			newid, newiderr := inres.LastInsertId()
			if newiderr != nil {
				logger.Log.Error(newiderr)
				return
			}
			dbserie.ID = uint(newid)
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
			var countera int
			var alternateerr error
			for idxalt := range serieconfig.AlternateName {
				if serieconfig.AlternateName[idxalt] == "" {
					continue
				}
				countera, alternateerr = database.CountRowsStatic("Select count(id) from dbserie_alternates where Dbserie_id = ? and title = ? COLLATE NOCASE", dbserie.ID, serieconfig.AlternateName[idxalt])
				if alternateerr != nil {
					continue
				}
				if countera == 0 {
					database.InsertArray("dbserie_alternates", []string{"dbserie_id", "title", "slug"}, []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt])})
				}
			}
		} else {
			finddbserie, _ := database.GetDbserie(database.Query{Where: "Seriename = ? COLLATE NOCASE", WhereArgs: []interface{}{serieconfig.Name}})
			dbserie = finddbserie
		}
	} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
		dbserie.ThetvdbID = serieconfig.TvdbID
		dbserie.Identifiedby = serieconfig.Identifiedby
		cdbserie, errdbserie := database.CountRowsStatic("Select count(id) from dbseries where Thetvdb_id = ?", serieconfig.TvdbID)
		if errdbserie != nil {
			return
		}
		if cdbserie == 0 {
			logger.Log.Debug("DbSeries get metadata for: ", serieconfig.TvdbID)

			if !config.ConfigCheck("imdb") {
				return
			}

			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_general.SerieMetaSourceTrakt, false)
			defer logger.ClearVar(&addaliases)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_general.SerieMetaSourceTrakt, false)
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases...)
			dbserieadded = true
			cdbserie2, errdbserie2 := database.CountRowsStatic("Select count(id) from dbseries where Thetvdb_id = ?", serieconfig.TvdbID)
			if errdbserie2 != nil {
				return
			}
			if cdbserie2 == 0 {
				inres, inreserr := database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
					[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
				if inreserr != nil {
					logger.Log.Error(inreserr)
					return
				}
				newid, newiderr := inres.LastInsertId()
				if newiderr != nil {
					logger.Log.Error(newiderr)
					return
				}
				dbserie.ID = uint(newid)
			} else {
				dbserie, _ = database.GetDbserie(database.Query{Select: "id, thetvdb_id, imdb_id, trakt_id", Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
			}
			titles, _ := database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", dbserie.ID)
			defer logger.ClearVar(&titles)
			titlegroup := dbserie.GetTitles(configTemplate, cfg_general.SerieAlternateTitleMetaSourceImdb, cfg_general.SerieAlternateTitleMetaSourceTrakt)
			defer logger.ClearVar(&titlegroup)
			var titlefound bool
			for idxalt := range titlegroup {
				if titlegroup[idxalt].Title == "" {
					continue
				}
				titlefound = false
				for idxtitle := range titles {
					if strings.EqualFold(titles[idxtitle].Str, titlegroup[idxalt].Title) {
						titlefound = true
						break
					}
				}
				if !titlefound {
					database.InsertArray("dbserie_alternates", []string{"dbserie_id", "title", "slug", "region"}, []interface{}{dbserie.ID, titlegroup[idxalt].Title, titlegroup[idxalt].Slug, titlegroup[idxalt].Region})
				}
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
			titles, _ = database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", dbserie.ID)
			for idxalt := range serieconfig.AlternateName {
				if serieconfig.AlternateName[idxalt] == "" {
					continue
				}
				titlefound = false
				for idxtitle := range titles {
					if strings.EqualFold(titles[idxtitle].Str, serieconfig.AlternateName[idxalt]) {
						titlefound = true
						break
					}
				}
				if !titlefound {
					database.InsertArray("dbserie_alternates", []string{"dbserie_id", "title", "slug"}, []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt])})
				}
			}
			logger.Log.Debug("DbSeries get metadata end for: ", serieconfig.TvdbID)
		} else {
			if dbserie.ID == 0 || dbserie.ThetvdbID == 0 || dbserie.ImdbID == "" || dbserie.TraktID == 0 {
				finddbserie, _ := database.GetDbserie(database.Query{Select: "id, thetvdb_id, imdb_id, trakt_id", Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
				dbserie = finddbserie
			}
		}
	}

	serietest, _ := database.QueryStaticColumnsOneStringOneInt("select listname, id from series where dbserie_id = ?", "select count(id) from series where dbserie_id = ?", dbserie.ID)
	defer logger.ClearVar(&serietest)

	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if list.Name == "" {
		return
	}
	if len(list.Ignore_map_lists) >= 1 {
		for idx := range list.Ignore_map_lists {
			for idxtest := range serietest {
				if strings.EqualFold(list.Ignore_map_lists[idx], serietest[idxtest].Str) {
					return
				}
			}
		}
	}
	for idxreplace := range list.Replace_map_lists {
		for idxtitle := range serietest {
			if strings.EqualFold(serietest[idxtitle].Str, list.Replace_map_lists[idxreplace]) {
				database.UpdateArray("series", []string{"missing", "listname", "dbserie_id", "quality_profile"}, []interface{}{true, listConfig, dbserie.ID, list.Template_quality}, database.Query{Where: "id = ?", WhereArgs: []interface{}{serietest[idxtitle].Num}})
			}
		}
	}

	var serie database.Serie
	cserie, _ := database.CountRowsStatic("Select count(id) from series where Dbserie_id = ? and listname = ? COLLATE NOCASE", dbserie.ID, listConfig)

	if cserie == 0 {
		logger.Log.Debug("Series add for: ", serieconfig.TvdbID)
		inres, inreserr := database.InsertArray("series", []string{"dbserie_id", "listname", "rootpath", "search_specials", "dont_search", "dont_upgrade"}, []interface{}{dbserie.ID, listConfig, serieconfig.Target, serieconfig.SearchSpecials, serieconfig.DontSearch, serieconfig.DontUpgrade})
		if inreserr != nil {
			logger.Log.Error(inreserr)
			return
		}
		newid, newiderr := inres.LastInsertId()
		if newiderr != nil {
			logger.Log.Error(newiderr)
			return
		}
		serie.ID = uint(newid)
	} else {
		id, err := database.QueryColumnUint("Select id from series where Dbserie_id = ? and listname = ? COLLATE NOCASE", dbserie.ID, listConfig)
		if err == nil {
			serie = database.Serie{ID: id, SearchSpecials: serieconfig.SearchSpecials, DontSearch: serieconfig.DontSearch, DontUpgrade: serieconfig.DontUpgrade}
		}
	}
	if checkall || dbserieadded {
		if strings.EqualFold(serieconfig.Source, "none") {
			//Don't add episodes automatically
		} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
			logger.Log.Debug("DbSeries get episodes for: ", serieconfig.TvdbID)
			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
			defer logger.ClearVar(&episode)
			adddbepisodes := make([]database.DbserieEpisode, 0, len(episode))
			defer logger.ClearVar(&adddbepisodes)
			dbepisode, _ := database.QueryStaticColumnsTwoString("select season, episode from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
			defer logger.ClearVar(&dbepisode)
			var entryfound bool
			var dbserieepisode database.DbserieEpisode
			for idxepi := range episode {
				entryfound = false
				for idxentry := range dbepisode {
					if strings.EqualFold(dbepisode[idxentry].Str1, episode[idxepi].Season) && strings.EqualFold(dbepisode[idxentry].Str2, episode[idxepi].Episode) {
						entryfound = true
						break
					}
				}
				if !entryfound {
					dbserieepisode = episode[idxepi]
					dbserieepisode.DbserieID = dbserie.ID
					adddbepisodes = append(adddbepisodes, dbserieepisode)
				}
			}
			if len(adddbepisodes) >= 1 {
				database.ReadWriteMu.Lock()
				database.DB.NamedExec("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (:episode, :season, :identifier, :title, :first_aired, :overview, :poster, :dbserie_id)", adddbepisodes)
				database.ReadWriteMu.Unlock()
			}

		}
	}

	dbepisode, _ := database.QueryStaticColumnsOneInt("select id from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
	episodes, _ := database.QueryStaticColumnsOneInt("select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?", "select count(id) from serie_episodes where dbserie_id = ? and serie_id = ?", dbserie.ID, serie.ID)
	defer logger.ClearVar(&dbepisode)
	defer logger.ClearVar(&episodes)
	var epifound bool
	var cnt int
	var errepisearch error

	for idxdbepi := range dbepisode {
		epifound = false
		for idxepi := range episodes {
			if episodes[idxepi].Num == dbepisode[idxdbepi].Num {
				epifound = true
				break
			}
		}
		if !epifound {
			cnt, errepisearch = database.CountRowsStatic("Select Count(*) FROM serie_episodes where serie_id = ? and dbserie_episode_id = ?", serie.ID, dbepisode[idxdbepi].Num)
			if cnt == 0 && errepisearch == nil {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, serie.ID, true, list.Template_quality, dbepisode[idxdbepi].Num})
			}
		}
	}
	logger.Log.Debug("DbSeries add episodes end for: ", serieconfig.TvdbID)

}

func JobReloadDbSeries(id uint, configTemplate string, listConfig string, checkall bool) {
	dbserie, _ := database.GetDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
	jobName := dbserie.Seriename
	if jobName == "" {
		jobName = listConfig
	}
	if importJobRunning.Contains(jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning.Add(jobName)
		defer importJobRunning.Remove(jobName)
	}

	logger.Log.Debug("DbSeries Add for: ", dbserie.ThetvdbID)

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("imdb") {
		return
	}

	dbserie, _ = database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{dbserie.ThetvdbID}})
	logger.Log.Debug("DbSeries get metadata for: ", dbserie.ThetvdbID)

	getfirstseries, _ := database.QueryStaticColumnsOneStringOneInt("Select listname, id from series where dbserie_id = ?", "Select count(id) from series where dbserie_id = ?", dbserie.ID)
	defer logger.ClearVar(&getfirstseries)

	var getconfigentry config.MediaTypeConfig //:= config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)

	if len(getfirstseries) >= 1 {
		var listfound bool
		for _, idx := range config.ConfigGetPrefix("serie_") {
			cfg_serie := config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound = false
			for idxlist := range cfg_serie.Lists {
				if cfg_serie.Lists[idxlist].Name == getfirstseries[0].Str {
					listfound = true
					getconfigentry = cfg_serie
					break
				}
			}
			if listfound {
				break
			}
		}
	}
	if getconfigentry.Name == "" {
		return
	}
	addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_general.SerieMetaSourceTrakt, false)
	defer logger.ClearVar(&addaliases)
	if dbserie.Seriename == "" {
		addaliases = dbserie.GetMetadata(getconfigentry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_general.SerieMetaSourceTrakt, false)
	}
	length := 1
	if addaliases != nil {
		length += len(addaliases)
	}
	alternateNames := make([]string, 0, length)
	defer logger.ClearVar(&alternateNames)
	if addaliases != nil {
		alternateNames = append(alternateNames, addaliases...)
	}
	alternateNames = append(alternateNames, dbserie.Seriename)

	database.UpdateArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
		[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby},
		database.Query{Where: "id = ?", WhereArgs: []interface{}{dbserie.ID}})

	logger.Log.Debug("DbSeries get metadata end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add titles for: ", dbserie.ThetvdbID)
	titles, _ := database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", dbserie.ID)
	defer logger.ClearVar(&titles)
	titlegroup := dbserie.GetTitles("serie_"+getconfigentry.Name, cfg_general.SerieAlternateTitleMetaSourceImdb, cfg_general.SerieAlternateTitleMetaSourceTrakt)
	defer logger.ClearVar(&titlegroup)
	var titlefound bool
	for idxalt := range titlegroup {
		if titlegroup[idxalt].Title == "" {
			continue
		}
		titlefound = false
		for idxtitle := range titles {
			if strings.EqualFold(titles[idxtitle].Str, titlegroup[idxalt].Title) {
				titlefound = true
				break
			}
		}
		if !titlefound {
			database.InsertArray("dbserie_alternates",
				[]string{"dbserie_id", "title", "slug", "region"},
				[]interface{}{dbserie.ID, titlegroup[idxalt].Title, titlegroup[idxalt].Slug, titlegroup[idxalt].Region})
		}
	}
	for _, title := range database.QueryStaticStringArray("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", dbserie.ID) {
		if title == "" {
			continue
		}
		titlefound = false
		for idxtitle := range titles {
			if strings.EqualFold(titles[idxtitle].Str, title) {
				titlefound = true
				break
			}
		}
		if !titlefound {
			database.InsertArray("dbserie_alternates",
				[]string{"dbserie_id", "title", "slug"},
				[]interface{}{dbserie.ID, title, logger.StringToSlug(title)})
		}
	}

	logger.Log.Debug("DbSeries add titles end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add serie end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries get episodes for: ", dbserie.ThetvdbID)
	dbepisode, _ := database.QueryDbserieEpisodes(database.Query{Select: "id, season, episode", Where: "dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})
	defer logger.ClearVar(&dbepisode)

	episodes := dbserie.GetEpisodes(getconfigentry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
	defer logger.ClearVar(&episodes)
	var epifound bool
	for idxepi := range episodes {
		epifound = false
		for idxdbepi := range dbepisode {
			if strings.EqualFold(episodes[idxepi].Season, dbepisode[idxdbepi].Season) && strings.EqualFold(episodes[idxepi].Episode, dbepisode[idxdbepi].Episode) {
				epifound = true
				database.UpdateArray("dbserie_episodes",
					[]string{"title", "first_aired", "overview", "poster", "runtime"},
					[]interface{}{episodes[idxepi].Title, episodes[idxepi].FirstAired, episodes[idxepi].Overview, episodes[idxepi].Poster, episodes[idxepi].Runtime},
					database.Query{Where: "id = ?", WhereArgs: []interface{}{dbepisode[idxdbepi].ID}})
				break
			}
		}
		if !epifound {
			database.InsertArray("dbserie_episodes",
				[]string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "runtime", "dbserie_id"},
				[]interface{}{episodes[idxepi].Episode, episodes[idxepi].Season, episodes[idxepi].Identifier, episodes[idxepi].Title, episodes[idxepi].FirstAired, episodes[idxepi].Overview, episodes[idxepi].Poster, episodes[idxepi].Runtime, episodes[idxepi].DbserieID})
		}
	}

	logger.Log.Debug("DbSeries get episodes end for: ", dbserie.ThetvdbID)

	foundseries, _ := database.QueryStaticColumnsOneStringOneInt("Select listname, id from series where dbserie_id = ?", "Select count(id) from series where dbserie_id = ?", dbserie.ID)
	defer logger.ClearVar(&foundseries)
	var listfound bool
	var dbepisodeint []database.Dbstatic_OneInt
	defer logger.ClearVar(&dbepisodeint)
	var episodesint []database.Dbstatic_OneInt
	defer logger.ClearVar(&episodesint)
	var cnt int
	var errepisearch error

	for idxserie := range foundseries {

		var getlist config.MediaListsConfig
		for _, idx := range config.ConfigGetPrefix("serie_") {
			cfg_serie := config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound = false
			for idxlist := range cfg_serie.Lists {
				if cfg_serie.Lists[idxlist].Name == foundseries[idxserie].Str {
					listfound = true
					getlist = cfg_serie.Lists[idxlist]
					break
				}
			}
			if listfound {
				break
			}
		}
		dbepisodeint, _ = database.QueryStaticColumnsOneInt("select id from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
		episodesint, _ = database.QueryStaticColumnsOneInt("select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?", "select count(id) from serie_episodes where dbserie_id = ? and serie_id = ?", dbserie.ID, foundseries[idxserie].Num)
		for idxdbepi := range dbepisodeint {
			epifound = false
			for idxepi := range episodesint {
				if episodesint[idxepi].Num == dbepisodeint[idxdbepi].Num {
					epifound = true
					break
				}
			}
			if !epifound {
				cnt, errepisearch = database.CountRowsStatic("Select Count(*) FROM serie_episodes where serie_id = ? and dbserie_episode_id = ?", foundseries[idxserie].Num, dbepisodeint[idxdbepi].Num)
				if cnt == 0 && errepisearch == nil {
					database.InsertArray("serie_episodes",
						[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
						[]interface{}{dbserie.ID, foundseries[idxserie].Num, true, getlist.Template_quality, dbepisodeint[idxdbepi].Num})
				}
			}
		}
	}

	logger.Log.Debug("DbSeries add episodes end for: ", dbserie.ThetvdbID)
}

func SerieFindTvdbIDByTitle(title string, searchtype string, addifnotfound bool) (int, bool) {

	if !config.ConfigCheck("general") {
		return 0, false
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	slugged := logger.StringToSlug(title)
	dbseriestemp, _ := database.QueryStaticColumnsOneInt("select thetvdb_id from dbseries where seriename = ? COLLATE NOCASE OR slug = ?", "select count(id) from dbseries where seriename = ? COLLATE NOCASE OR slug = ?", title, slugged)
	defer logger.ClearVar(&dbseriestemp)

	if len(dbseriestemp) == 1 {
		return dbseriestemp[0].Num, true
	}

	dbseriestemp, _ = database.QueryStaticColumnsOneInt("select dbseries.thetvdb_id from dbserie_alternates inner join dbseries on dbseries.id=dbserie_alternates.dbserie_id where dbserie_alternates.title = ? COLLATE NOCASE OR dbserie_alternates.slug = ?", "select count(dbserie_alternates.id) from dbserie_alternates inner join dbseries on dbseries.id=dbserie_alternates.dbserie_id where dbserie_alternates.title = ? COLLATE NOCASE OR dbserie_alternates.slug = ?", title, slugged)

	if len(dbseriestemp) == 1 {
		return dbseriestemp[0].Num, true
	}

	if addifnotfound {
		searchprovider := []string{"tvdb", "tmdb", "omdb"}
		defer logger.ClearVar(&searchprovider)
		if strings.EqualFold(searchtype, "rss") {
			if len(cfg_general.MovieRSSMetaSourcePriority) >= 1 {
				searchprovider = cfg_general.MovieRSSMetaSourcePriority
			}
		} else {
			if len(cfg_general.MovieParseMetaSourcePriority) >= 1 {
				searchprovider = cfg_general.MovieParseMetaSourcePriority
			}
		}
		if len(searchprovider) >= 1 {
			for idxprovider := range searchprovider {
				if strings.EqualFold(searchprovider[idxprovider], "tmdb") {
					if cfg_general.SerieMetaSourceTmdb {
					}
				}
				if strings.EqualFold(searchprovider[idxprovider], "trakt") {
					if cfg_general.SerieMetaSourceTrakt {
					}
				}
			}
		}
	}

	logger.Log.Warningln("All Movie Lookups failed: ", title)
	return 0, false
}
