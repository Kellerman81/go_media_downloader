package importfeed

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/sizedwaitgroup"
)

func JobImportMovies(dbmovie database.Dbmovie, configTemplate string, listConfig string, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := dbmovie.ImdbID
	if jobName == "" {
		jobName = listConfig
	}
	defer func() {
		importJobRunning = logger.FindAndDeleteStringArray(importJobRunning, jobName)
		wg.Done()
	}()
	if logger.CheckStringArray(importJobRunning, jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning = append(importJobRunning, jobName)
	}

	cdbmovie, _ := database.CountRowsStatic("Select count(id) from dbmovies where imdb_id = ?", dbmovie.ImdbID)
	//cdbmovie, _ := database.CountRows("dbmovies", database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
	if cdbmovie == 0 {
		logger.Log.Debug("Get Movie Metadata: ", dbmovie.ImdbID)

		if !config.ConfigCheck("general") {
			return
		}
		cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

		if len(cfg_general.MovieMetaSourcePriority) >= 1 {
			for idxmeta := range cfg_general.MovieMetaSourcePriority {
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "imdb") {
					logger.Log.Debug("Get Movie Metadata - imdb: ", dbmovie.ImdbID)
					dbmovie.GetImdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "tmdb") {
					logger.Log.Debug("Get Movie Metadata - tmdb: ", dbmovie.ImdbID)
					dbmovie.GetTmdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "omdb") {
					logger.Log.Debug("Get Movie Metadata - omdb: ", dbmovie.ImdbID)
					dbmovie.GetOmdbMetadata(false)
				}
				if strings.EqualFold(cfg_general.MovieMetaSourcePriority[idxmeta], "trakt") {
					logger.Log.Debug("Get Movie Metadata - trakt: ", dbmovie.ImdbID)
					dbmovie.GetTraktMetadata(false)
				}
			}
		} else {
			dbmovie.GetMetadata(cfg_general.MovieMetaSourceImdb, cfg_general.MovieMetaSourceTmdb, cfg_general.MovieMetaSourceOmdb, cfg_general.MovieMetaSourceTrakt)
		}

		list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
		if !config.ConfigCheck("list_" + list.Template_list) {
			return
		}
		if !AllowMovieImport(dbmovie.ImdbID, list.Template_list) {
			return
		}

		cdbmovie2, _ := database.CountRowsStatic("Select count(id) from dbmovies where imdb_id = ?", dbmovie.ImdbID)
		//cdbmovie2, _ := database.CountRows("dbmovies", database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
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
			titles, _ := database.QueryStaticColumnsOneString("select title from dbmovie_titles where dbmovie_id=?", "select count(id) from dbmovie_titles where dbmovie_id=?", dbmovie.ID)

			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			titlegroup := dbmovie.GetTitles(configEntry.Metadata_title_languages, cfg_general.MovieAlternateTitleMetaSourceImdb, cfg_general.MovieAlternateTitleMetaSourceTmdb, cfg_general.MovieAlternateTitleMetaSourceTrakt)
			for idxtitle := range titlegroup {
				titlefound := false
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
			id, err := database.QueryColumnStatic("Select id from dbmovies where imdb_id = ?", dbmovie.ImdbID)
			if err == nil {
				dbmovie = database.Dbmovie{ID: uint(id.(int64))}
			}
			id = nil
		}
	} else {
		if dbmovie.ID == 0 {
			id, err := database.QueryColumnStatic("Select id from dbmovies where imdb_id = ?", dbmovie.ImdbID)
			if err == nil {
				dbmovie = database.Dbmovie{ID: uint(id.(int64))}
			}
			id = nil
		}
	}
	movietest, _ := database.QueryStaticColumnsOneStringOneInt("select listname, id from movies where dbmovie_id=?", "select count(id) from movies where dbmovie_id=?", dbmovie.ID)
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if len(list.Ignore_map_lists) >= 1 {
		list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
		for idx := range list.Ignore_map_lists {
			counter, _ := database.CountRowsStatic("Select count(id) from movies where listname=? and dbmovie_id=?", list.Ignore_map_lists[idx], dbmovie.ID)
			if counter >= 1 {
				return
			}
		}
	}

	foundmovie := false
	counter, _ := database.CountRowsStatic("Select count(id) from movies where listname=? and dbmovie_id=?", listConfig, dbmovie.ID)
	//counter, _ := database.CountRows("movies", database.Query{Where: "listname=? and dbmovie_id=?", WhereArgs: []interface{}{list.Name, dbmovie.ID}})
	if counter >= 1 {
		foundmovie = true
	}

	if foundmovie {
		for idxreplace := range list.Replace_map_lists {
			for idxtitle := range movietest {
				if strings.EqualFold(movietest[idxtitle].Str, list.Replace_map_lists[idxreplace]) {
					database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality}, database.Query{Where: "id=?", WhereArgs: []interface{}{movietest[idxtitle].Num}})
				}
			}
		}
	} else {
		logger.Log.Debug("Add Movie: ", dbmovie.ImdbID)
		_, moviereserr := database.InsertArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality})
		if moviereserr != nil {
			logger.Log.Error(moviereserr)
			return
		}
		for idxreplace := range list.Replace_map_lists {
			for idxtitle := range movietest {
				if strings.EqualFold(movietest[idxtitle].Str, list.Replace_map_lists[idxreplace]) {
					database.UpdateArray("movies", []string{"missing", "listname", "dbmovie_id", "quality_profile"}, []interface{}{true, listConfig, dbmovie.ID, list.Template_quality}, database.Query{Where: "id=?", WhereArgs: []interface{}{movietest[idxtitle].Num}})
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
		//countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? and num_votes < ?", WhereArgs: []interface{}{imdb, list.MinVotes}})
		if countergenre >= 1 {
			logger.Log.Debug("error vote count too low for", imdb)
			return false
		}
	}
	if list.MinRating != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_ratings where tconst = ? and average_rating < ?", imdb, list.MinRating)
		//countergenre, _ := database.ImdbCountRows("imdb_ratings", database.Query{Where: "tconst = ? and average_rating < ?", WhereArgs: []interface{}{imdb, list.MinRating}})
		if countergenre >= 1 {
			logger.Log.Debug("error average vote too low for", imdb)
			return false
		}
	}
	if len(list.Excludegenre) >= 1 {
		excludebygenre := false
		for idxgenre := range list.Excludegenre {
			countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Excludegenre[idxgenre])
			//countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{imdb, list.Excludegenre[idxgenre]}})
			if countergenre >= 1 {
				excludebygenre = true
				logger.Log.Debug("error excluded genre", list.Excludegenre[idxgenre], imdb)
				break
			}
		}
		if excludebygenre {
			return false
		}
	}
	if len(list.Includegenre) >= 1 {
		includebygenre := false
		for idxgenre := range list.Includegenre {
			countergenre, _ := database.ImdbCountRowsStatic("Select count(tconst) from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE", imdb, list.Includegenre[idxgenre])
			//countergenre, _ := database.ImdbCountRows("imdb_genres", database.Query{Where: "tconst = ? and genre = ? COLLATE NOCASE", WhereArgs: []interface{}{imdb, list.Includegenre[idxgenre]}})
			if countergenre >= 1 {
				includebygenre = true
				break
			}
		}
		if !includebygenre {
			logger.Log.Debug("error included genre not found", list.Includegenre, imdb)
			return false
		}
	}
	return true
}
func JobReloadMovies(dbmovie database.Dbmovie, configTemplate string, listConfig string, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := dbmovie.ImdbID
	if jobName == "" {
		jobName = listConfig
	}
	defer func() {
		importJobRunning = logger.FindAndDeleteStringArray(importJobRunning, jobName)
		wg.Done()
	}()
	if logger.CheckStringArray(importJobRunning, jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning = append(importJobRunning, jobName)
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.SchedulerDisabled {
		return
	}

	dbmovie, _ = database.GetDbmovie(database.Query{Where: "imdb_id = ?", WhereArgs: []interface{}{dbmovie.ImdbID}})
	logger.Log.Debug("Get Movie Metadata: ", dbmovie.ImdbID)
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
		database.Query{Where: "id=?", WhereArgs: []interface{}{dbmovie.ID}})

	movies, _ := database.QueryStaticColumnsOneString("select listname from movies where dbmovie_id=?", "select count(id) from movies where dbmovie_id=?", dbmovie.ID)

	var getconfigentry config.MediaTypeConfig //:= config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if len(movies) >= 1 {
		for _, idx := range config.ConfigGetPrefix("movie_") {
			cfg_movie := config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound := false
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
	titles, _ := database.QueryStaticColumnsOneString("select title from dbmovie_titles where dbmovie_id=?", "select count(id) from dbmovie_titles where dbmovie_id=?", dbmovie.ID)
	titlegroup := dbmovie.GetTitles(getconfigentry.Metadata_title_languages, cfg_general.MovieAlternateTitleMetaSourceImdb, cfg_general.MovieAlternateTitleMetaSourceTmdb, cfg_general.MovieAlternateTitleMetaSourceTrakt)
	for idxtitle := range titlegroup {
		titlefound := false
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

func checkifmovieyearmatches(entriesfound int, yearint int, movies []database.Movie) (imdb string, movie database.Movie) {

	if entriesfound >= 1 && yearint != 0 {
		foundyear := 0
		foundyear1 := 0
		imdbyear := ""
		imdbyear1 := ""
		var movieyear database.Movie
		var movieyear1 database.Movie
		for idx := range movies {

			dbmovie, _ := database.GetDbmovie(database.Query{Select: "year, imdb_id", Where: "id=?", WhereArgs: []interface{}{movies[idx].DbmovieID}})
			if dbmovie.Year == yearint {
				imdbyear = dbmovie.ImdbID
				movieyear = movies[idx]
				foundyear += 1
			}
			if !config.ConfigCheck("quality_" + movies[idx].QualityProfile) {
				continue
			}
			cfg_quality := config.ConfigGet("quality_" + movies[idx].QualityProfile).Data.(config.QualityConfig)
			allowyear1 := cfg_quality.CheckYear1

			if allowyear1 {
				if dbmovie.Year == yearint+1 || dbmovie.Year == yearint-1 {
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
	return "", database.Movie{}
}

func movieCheckIfYear(dbmovies []database.Dbmovie, listname string, yearint int, allowyear1 bool) (bool, database.Movie, string, int) {

	for idx := range dbmovies {
		movies, _ := database.QueryMovies(database.Query{Where: "Dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbmovies[idx].ID, listname}})
		entriesfound := len(movies)
		imdb, movie := checkifmovieyearmatches(entriesfound, yearint, movies)
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
		imdb, movie := checkifmovieyearmatches(entriesfound, yearint, movies)
		if imdb != "" {
			entriesfound = 1
			logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", dbmovietitles[idx].Title)
			return true, movie, imdb, entriesfound
		}
	}
	return false, database.Movie{}, "", 0
}

func MovieFindDbByTitle(title string, year string, listname string, allowyear1 bool, searchtype string) (movie database.Movie, imdb string, entriesfound int) {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	searchfor := title
	yearint, _ := strconv.Atoi(year)
	slugged := logger.StringToSlug(title)

	dbmoviestemp, _ := database.QueryStaticColumnsOneStringOneInt("select title, id from dbmovies where title = ? COLLATE NOCASE", "select count(id) from dbmovies where title = ? COLLATE NOCASE", title)
	dbmovies := make([]database.Dbmovie, 0, len(dbmoviestemp))

	for idx := range dbmoviestemp {
		dbmovies = append(dbmovies, database.Dbmovie{ID: uint(dbmoviestemp[idx].Num), Title: dbmoviestemp[idx].Str})
	}
	found, foundmovie, foundimdb, foundentries := movieCheckIfYear(dbmovies, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmoviestitletemp, _ := database.QueryStaticColumnsOneStringOneInt("select title, dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE", "select count(id) from dbmovie_titles where title = ? COLLATE NOCASE", title)
	dbmovietitles := make([]database.DbmovieTitle, 0, len(dbmoviestitletemp))

	for idx := range dbmoviestitletemp {
		dbmovietitles = append(dbmovietitles, database.DbmovieTitle{ID: uint(dbmoviestitletemp[idx].Num), Title: dbmoviestitletemp[idx].Str})
	}
	found, foundmovie, foundimdb, foundentries = movieCheckAlternateIfYear(dbmovietitles, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmoviestemp, _ = database.QueryStaticColumnsOneStringOneInt("select title, id from dbmovies where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCAS", "select count(id) from dbmovies where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCAS", slugged, slugged)
	dbmovies = make([]database.Dbmovie, 0, len(dbmoviestemp))
	for idx := range dbmoviestemp {
		dbmovies = append(dbmovies, database.Dbmovie{ID: uint(dbmoviestemp[idx].Num), Title: dbmoviestemp[idx].Str})
	}
	found, foundmovie, foundimdb, foundentries = movieCheckIfYear(dbmovies, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	dbmoviestitletemp, _ = database.QueryStaticColumnsOneStringOneInt("select title, dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE", "select count(id) from dbmovie_titles where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE", slugged, slugged)
	dbmovietitles = make([]database.DbmovieTitle, 0, len(dbmoviestitletemp))
	for idx := range dbmoviestitletemp {
		dbmovietitles = append(dbmovietitles, database.DbmovieTitle{ID: uint(dbmoviestitletemp[idx].Num), Title: dbmoviestitletemp[idx].Str})
	}
	found, foundmovie, foundimdb, foundentries = movieCheckAlternateIfYear(dbmovietitles, listname, yearint, allowyear1)
	if found && foundimdb != "" {
		return foundmovie, foundimdb, foundentries
	}

	searchprovider := []string{"imdb", "tmdb", "omdb"}

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
			if strings.EqualFold(searchprovider[idxprovider], "imdb") {
				if cfg_general.MovieMetaSourceImdb {
					//Search in Imdb
					imdbtitles, _ := database.QueryImdbStaticColumnsOneStringOneInt("select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE)", "select count(tconst) from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE)", title, title, slugged)

					if len(imdbtitles) >= 1 {
						foundimdb := 0
						foundimdb1 := 0
						imdbloop := ""
						imdbloop1 := ""
						for idximdb := range imdbtitles {
							if yearint == 0 && len(imdbtitles) == 1 {
								imdbloop = imdbtitles[idximdb].Str
								foundimdb += 1
							}
							if imdbtitles[idximdb].Num == yearint {
								imdbloop = imdbtitles[idximdb].Str
								foundimdb += 1
							}
							if allowyear1 {
								if imdbtitles[idximdb].Num == yearint+1 {
									imdbloop1 = imdbtitles[idximdb].Str
									foundimdb1 += 1
								}
								if imdbtitles[idximdb].Num == yearint-1 {
									imdbloop1 = imdbtitles[idximdb].Str
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

					imdbaka, _ := database.QueryImdbStaticColumnsOneString("select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "select count(tconst) from imdb_akas where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, slugged)

					if len(imdbaka) >= 1 {
						imdb = imdbaka[0].Str
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
				if cfg_general.MovieMetaSourceTmdb {
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
				if cfg_general.MovieMetaSourceOmdb {
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

func MovieGetListFilter(lists []string, dbid uint, yearint int) (imdb string, list string) {
	for idx := range lists {
		movies, movieserr := database.QueryMovies(database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{dbid, lists[idx]}})
		if movieserr != nil {
			logger.Log.Error(movieserr)
			return
		}
		entriesfound := len(movies)
		if entriesfound >= 1 {
			logger.Log.Debug("Movie found with dbid: ", dbid, " and list: ", lists[idx])
			imdb_get, _ := checkifmovieyearmatches(entriesfound, yearint, movies)
			if imdb_get != "" {
				entriesfound = 1
				imdb = imdb_get
				list = lists[idx]

				break
			}
		}

	}
	return
}
func MovieFindListByTitle(title string, year string, lists []string, searchtype string) (list string, imdb string, entriesfound int, dbmovie database.Dbmovie) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	searchfor := title
	if year == "0" {
		year = ""
	}
	yearint, _ := strconv.Atoi(year)
	slugged := logger.StringToSlug(title)
	logger.Log.Debug("DB Search for ", title)
	dbmovies, _ := database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})

	if len(dbmovies) >= 1 {
		for idx := range dbmovies {
			logger.Log.Debug("DB Search for - filter dbid: ", dbmovies[idx].ID, " year: ", yearint)
			imdb_get, list_get := MovieGetListFilter(lists, dbmovies[idx].ID, yearint)
			logger.Log.Debug("DB Search for - results dbid: ", dbmovies[idx].ID, " imdb: ", imdb_get, " list: ", list_get)
			if imdb_get != "" && list_get != "" {
				entriesfound = 1
				imdb = imdb_get
				list = list_get
				dbmovie = dbmovies[idx]
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}
	logger.Log.Debug("DB Search alternate title for ", title)
	dbmovietitles, _ := database.QueryStaticColumnsOneInt("select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE", "select count(id) from dbmovie_titles where title = ? COLLATE NOCASE", title)

	if len(dbmovietitles) >= 1 {
		for idx := range dbmovietitles {
			imdb_get, list_get := MovieGetListFilter(lists, uint(dbmovietitles[idx].Num), yearint)
			if imdb_get != "" && list_get != "" {
				entriesfound = 1
				imdb = imdb_get
				list = list_get
				dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{dbmovietitles[idx].Num}})
				dbmovie = dbmovieget
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}

	logger.Log.Debug("DB Search for ", slugged)
	dbmovies, _ = database.QueryDbmovie(database.Query{Where: "title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE", WhereArgs: []interface{}{slugged, slugged}})
	if len(dbmovies) >= 1 {
		for idx := range dbmovies {
			imdb_get, list_get := MovieGetListFilter(lists, dbmovies[idx].ID, yearint)
			if imdb_get != "" && list_get != "" {
				entriesfound = 1
				imdb = imdb_get
				list = list_get
				dbmovie = dbmovies[idx]
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}
	logger.Log.Debug("DB Search alternate title for ", slugged)
	dbmovietitles, _ = database.QueryStaticColumnsOneInt("select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE", "select count(id) from dbmovie_titles where title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE", slugged, slugged)
	if len(dbmovietitles) >= 1 {
		for idx := range dbmovietitles {
			imdb_get, list_get := MovieGetListFilter(lists, uint(dbmovietitles[idx].Num), yearint)
			if imdb_get != "" && list_get != "" {
				entriesfound = 1
				imdb = imdb_get
				list = list_get
				dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{dbmovietitles[idx].Num}})
				dbmovie = dbmovieget
				logger.Log.Debug("DB Search succeded. Found Movies: ", entriesfound, " for ", title)
				return
			}
		}
	}

	searchprovider := []string{"imdb", "tmdb", "omdb"}

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
		argslist := []interface{}{}
		for idx := range lists {
			argslist = append(argslist, lists[idx])
		}
		for idxprovider := range searchprovider {
			if strings.EqualFold(searchprovider[idxprovider], "imdb") {
				if cfg_general.MovieMetaSourceImdb {
					//Search in Imdb
					logger.Log.Debug("Imdb Search for ", title, " and ", slugged)
					imdbtitles, _ := database.QueryImdbStaticColumnsOneStringOneInt("select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE)", "select count(tconst) from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE)", title, title, slugged)

					if len(imdbtitles) >= 1 {
						foundimdb := 0
						foundimdb1 := 0
						imdbloop := ""
						imdbloop1 := ""
						for idximdb := range imdbtitles {
							if yearint == 0 && len(imdbtitles) == 1 {
								imdbloop = imdbtitles[idximdb].Str
								foundimdb += 1
							}
							if imdbtitles[idximdb].Num == yearint {
								imdbloop = imdbtitles[idximdb].Str
								foundimdb += 1
							}
							if imdbtitles[idximdb].Num == yearint+1 {
								imdbloop1 = imdbtitles[idximdb].Str
								foundimdb1 += 1
							}
							if imdbtitles[idximdb].Num == yearint-1 {
								imdbloop1 = imdbtitles[idximdb].Str
								foundimdb1 += 1
							}
						}
						if foundimdb == 1 {
							imdb = imdbloop
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							if len(movies) >= 1 {
								list = movies[0].Listname
								dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
								dbmovie = dbmovieget
							}
							entriesfound = len(movies)
							argsimdb = nil
							logger.Log.Debug("Imdb Search (Year) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
						if foundimdb1 == 1 {
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdbloop1)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.quality_profile, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							argsimdb = nil
							if len(movies) >= 1 {
								if !config.ConfigCheck("quality_" + movies[0].QualityProfile) {
									continue
								}
								cfg_quality := config.ConfigGet("quality_" + movies[0].QualityProfile).Data.(config.QualityConfig)
								if cfg_quality.CheckYear1 {
									imdb = imdbloop1
									list = movies[0].Listname
									entriesfound = len(movies)
									dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
									dbmovie = dbmovieget
								}
							}

							logger.Log.Debug("Imdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}

					logger.Log.Debug("Imdb Aka Search for ", title, " and ", slugged)
					imdbaka, _ := database.QueryImdbStaticColumnsOneString("select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "select count(tconst) from imdb_akas where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, slugged)

					if len(imdbaka) == 1 {
						imdb = imdbaka[0].Str
						argsimdb := []interface{}{}
						argsimdb = append(argsimdb, imdb)
						argsimdb = append(argsimdb, argslist...)
						movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
						argsimdb = nil
						if len(movies) >= 1 {
							list = movies[0].Listname
							dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
							dbmovie = dbmovieget
						}
						entriesfound = len(movies)

						logger.Log.Debug("Imdb Aka Search succeded. Found Movies: ", entriesfound, " for ", title)
						return
					}
				}
			}
			if strings.EqualFold(searchprovider[idxprovider], "tmdb") {
				if cfg_general.MovieMetaSourceTmdb {
					logger.Log.Debug("Tmdb Search for ", searchfor, " ", year)
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
							if tmdbresult.ReleaseDate.Year() == yearint+1 {
								tmdbloopid1 = tmdbresult.ID
								foundtmdb1 += 1
							}
							if tmdbresult.ReleaseDate.Year() == yearint-1 {
								tmdbloopid1 = tmdbresult.ID
								foundtmdb1 += 1
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
								movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.quality_profile, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
								argsimdb = nil
								if len(movies) >= 1 {
									if !config.ConfigCheck("quality_" + movies[0].QualityProfile) {
										continue
									}
									cfg_quality := config.ConfigGet("quality_" + movies[0].QualityProfile).Data.(config.QualityConfig)
									if cfg_quality.CheckYear1 {
										list = movies[0].Listname
										entriesfound = len(movies)
										dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
										dbmovie = dbmovieget
									}
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
				if cfg_general.MovieMetaSourceOmdb {
					logger.Log.Debug("Omdb Search for ", title, " and ", year)
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
							if omdbyearint == yearint+1 {
								omdbloop1 = searchomdb.Search[idxresult].ImdbID
								foundomdb1 += 1
							}
							if omdbyearint == yearint-1 {
								omdbloop1 = searchomdb.Search[idxresult].ImdbID
								foundomdb1 += 1
							}
						}

						if foundomdb == 1 {
							imdb = omdbloop
							argsimdb := []interface{}{}
							argsimdb = append(argsimdb, imdb)
							argsimdb = append(argsimdb, argslist...)
							movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							argsimdb = nil
							if len(movies) >= 1 {
								list = movies[0].Listname
								dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
								dbmovie = dbmovieget
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
							movies, _ := database.QueryMovies(database.Query{Select: "movies.listname, movies.quality_profile, movies.dbmovie_id", InnerJoin: " Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and Movies.listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: argsimdb})
							argsimdb = nil
							if len(movies) >= 1 {
								if !config.ConfigCheck("quality_" + movies[0].QualityProfile) {
									continue
								}
								cfg_quality := config.ConfigGet("quality_" + movies[0].QualityProfile).Data.(config.QualityConfig)
								if cfg_quality.CheckYear1 {
									list = movies[0].Listname
									entriesfound = len(movies)
									dbmovieget, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movies[0].DbmovieID}})
									dbmovie = dbmovieget
								}
							}

							logger.Log.Debug("Omdb Search (Year+1) succeded. Found Movies: ", entriesfound, " for ", title)
							return
						}
					}
				}
			}
		}
		argslist = nil
	}

	logger.Log.Debug("All Movie Lookups failed: ", title)
	return
}

func Findseriebyname(title string, listname string) (database.Serie, int) {
	logger.Log.Debug("Find Serie by Name", title, " in ", listname)
	titleslug := logger.StringToSlug(title)
	counter, _ := database.CountRowsStatic("Select count(id) from dbseries where Seriename = ? COLLATE NOCASE or Slug = ? COLLATE NOCASE", title, titleslug)
	if counter >= 2 || counter == 0 {
		return database.Serie{}, 0
	}
	id, err := database.QueryColumnStatic("Select id from dbseries where Seriename = ? COLLATE NOCASE", title)
	if err != nil {
		id, err = database.QueryColumnStatic("Select id from dbseries where Slug = ? COLLATE NOCASE", titleslug)
	}
	if err == nil {
		logger.Log.Debug("Find Serie by Name", title, " in ", listname, " dbserie found ", id)
		findseries, _ := database.QuerySeries(database.Query{Where: "dbserie_id=? AND listname = ?", WhereArgs: []interface{}{id, listname}})

		id = nil
		if len(findseries) == 1 {
			logger.Log.Debug("Found Serie by Name", title, " in ", listname)
			return findseries[0], len(findseries)
		}
	}
	id = nil
	return database.Serie{}, 0
}
func Findseriebyalternatename(title string, listname string) (database.Serie, int) {
	logger.Log.Debug("Find Serie by Name", title, " in ", listname)
	titleslug := logger.StringToSlug(title)
	counter, _ := database.CountRowsStatic("Select count(id) from dbseries inner join Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id where Dbserie_alternates.Title = ? COLLATE NOCASE or Dbserie_alternates.Slug = ? COLLATE NOCASE", title, titleslug)
	if counter >= 2 || counter == 0 {
		return database.Serie{}, 0
	}
	id, err := database.QueryColumnStatic("Select id from dbseries inner join Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id where Dbserie_alternates.Title = ? COLLATE NOCASE", title)
	if err != nil {
		id, err = database.QueryColumnStatic("Select id from dbseries inner join Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id where Dbserie_alternates.Slug = ? COLLATE NOCASE", titleslug)
	}
	if err == nil {
		findseries, _ := database.QuerySeries(database.Query{Where: "DbSerie_id = ? AND listname = ?", WhereArgs: []interface{}{id, listname}})

		id = nil
		if len(findseries) == 1 {
			logger.Log.Debug("Found Serie by Name", title, " in ", listname)
			return findseries[0], len(findseries)
		}
	}
	id = nil
	return database.Serie{}, 0
}
func GetEpisodeArray(identifiedby string, str1 string, str2 string) []string {
	episodeArray := []string{}
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

var importJobRunning []string

func JobImportDbSeries(serieconfig config.SerieConfig, configTemplate string, listConfig string, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := serieconfig.Name
	if jobName == "" {
		jobName = listConfig
	}

	defer func() {
		importJobRunning = logger.FindAndDeleteStringArray(importJobRunning, jobName)
		wg.Done()
	}()
	if logger.CheckStringArray(importJobRunning, jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning = append(importJobRunning, jobName)
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

		finddbserie, _ := database.GetDbserie(database.Query{Where: "Seriename = ? COLLATE NOCASE", WhereArgs: []interface{}{serieconfig.Name}})
		cdbserie, _ := database.CountRowsStatic("Select count(id) from dbseries where Seriename = ? COLLATE NOCASE", serieconfig.Name)
		//cdbserie, _ := database.CountRows("dbseries", database.Query{Where: "Seriename = ? COLLATE NOCASE", WhereArgs: []interface{}{serieconfig.Name}})
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
			for idxalt := range serieconfig.AlternateName {
				countera, _ := database.CountRowsStatic("Select count(id) from dbserie_alternates where Dbserie_id = ? and title = ? COLLATE NOCASE", dbserie.ID, serieconfig.AlternateName[idxalt])
				//countera, _ := database.CountRows("dbserie_alternates", database.Query{Where: "Dbserie_id = ? and title = ? COLLATE NOCASE", WhereArgs: []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt]}})
				if countera == 0 {
					database.InsertArray("dbserie_alternates", []string{"dbserie_id", "title", "slug"}, []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt])})
				}
			}
		} else {
			dbserie = finddbserie
		}
	} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
		dbserie.ThetvdbID = serieconfig.TvdbID
		dbserie.Identifiedby = serieconfig.Identifiedby
		cdbserie, _ := database.CountRowsStatic("Select count(id) from dbseries where Thetvdb_id = ?", serieconfig.TvdbID)
		//cdbserie, _ := database.CountRows("dbseries", database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
		if cdbserie == 0 {
			logger.Log.Debug("DbSeries get metadata for: ", serieconfig.TvdbID)

			if !config.ConfigCheck("imdb") {
				return
			}
			cfg_imdb := config.ConfigGet("imdb").Data.(config.ImdbConfig)

			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases...)
			dbserieadded = true
			cdbserie2, _ := database.CountRowsStatic("Select count(id) from dbseries where Thetvdb_id = ?", serieconfig.TvdbID)
			//cdbserie2, _ := database.CountRows("dbseries", database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
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
			titles, _ := database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id=?", "select count(id) from dbserie_alternates where dbserie_id=?", dbserie.ID)
			titlegroup := dbserie.GetTitles(configEntry.Metadata_title_languages, cfg_general.SerieAlternateTitleMetaSourceImdb, cfg_general.SerieAlternateTitleMetaSourceTrakt)
			for idxalt := range titlegroup {
				titlefound := false
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
			titles, _ = database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id=?", "select count(id) from dbserie_alternates where dbserie_id=?", dbserie.ID)
			for idxalt := range serieconfig.AlternateName {
				titlefound := false
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

	var serie database.Serie

	serietest, _ := database.QueryStaticColumnsOneStringOneInt("select listname, id from series where dbserie_id=?", "select count(id) from series where dbserie_id=?", dbserie.ID)

	//serietest, _ := database.QuerySeries(database.Query{Where: "Dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
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
				database.UpdateArray("series", []string{"missing", "listname", "dbserie_id", "quality_profile"}, []interface{}{true, listConfig, dbserie.ID, list.Template_quality}, database.Query{Where: "id=?", WhereArgs: []interface{}{serietest[idxtitle].Num}})
			}
		}
	}

	cserie, _ := database.CountRowsStatic("Select count(id) from series where Dbserie_id = ? and listname = ?", dbserie.ID, listConfig)
	//cserie, _ := database.CountRows("series", database.Query{Where: "Dbserie_id = ? and listname = ?", WhereArgs: []interface{}{dbserie.ID, list.Name}})
	if cserie == 0 {
		logger.Log.Debug("Series add for: ", serieconfig.TvdbID)
		inres, inreserr := database.InsertArray("series", []string{"dbserie_id", "listname", "rootpath"}, []interface{}{dbserie.ID, listConfig, serieconfig.Target})
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
		id, err := database.QueryColumnStatic("Select id from series where Dbserie_id = ? and listname = ?", dbserie.ID, listConfig)
		if err == nil {
			serie = database.Serie{ID: uint(id.(int64))}
		}
		id = nil
	}
	if checkall || dbserieadded {
		if strings.EqualFold(serieconfig.Source, "none") {
			//Don't add episodes automatically
		} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
			logger.Log.Debug("DbSeries get episodes for: ", serieconfig.TvdbID)
			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
			adddbepisodes := make([]database.DbserieEpisode, 0, len(episode))
			dbepisode, _ := database.QueryStaticColumnsTwoString("select season, episode from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
			for idxepi := range episode {
				entryfound := false
				for idxentry := range dbepisode {
					if strings.EqualFold(dbepisode[idxentry].Str1, episode[idxepi].Season) && strings.EqualFold(dbepisode[idxentry].Str2, episode[idxepi].Episode) {
						entryfound = true
						break
					}
				}
				if !entryfound {
					dbserieepisode := episode[idxepi]
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
		dbepisode, _ := database.QueryStaticColumnsOneInt("select id from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
		episodes, _ := database.QueryStaticColumnsOneInt("select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?", "select count(id) from serie_episodes where dbserie_id = ? and serie_id = ?", dbserie.ID, serie.ID)
		for idxdbepi := range dbepisode {
			epifound := false
			for idxepi := range episodes {
				if episodes[idxepi].Num == dbepisode[idxdbepi].Num {
					epifound = true
					break
				}
			}
			if !epifound {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, serie.ID, true, list.Template_quality, dbepisode[idxdbepi].Num})
			}
		}

		logger.Log.Debug("DbSeries add episodes end for: ", serieconfig.TvdbID)
	} else {
		dbepisode, _ := database.QueryStaticColumnsOneInt("select id from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
		episodes, _ := database.QueryStaticColumnsOneInt("select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?", "select count(id) from serie_episodes where dbserie_id = ? and serie_id = ?", dbserie.ID, serie.ID)
		for idxdbepi := range dbepisode {
			epifound := false
			for idxepi := range episodes {
				if episodes[idxepi].Num == dbepisode[idxdbepi].Num {
					epifound = true
					break
				}
			}
			if !epifound {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, serie.ID, true, list.Template_quality, dbepisode[idxdbepi].Num})
			}
		}
		logger.Log.Debug("DbSeries add episodes end for: ", serieconfig.TvdbID)
	}
}

func JobReloadDbSeries(dbserie database.Dbserie, configTemplate string, listConfig string, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := dbserie.Seriename
	if jobName == "" {
		jobName = listConfig
	}
	defer func() {
		importJobRunning = logger.FindAndDeleteStringArray(importJobRunning, jobName)
		wg.Done()
	}()
	if logger.CheckStringArray(importJobRunning, jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		importJobRunning = append(importJobRunning, jobName)
	}

	logger.Log.Debug("DbSeries Add for: ", dbserie.ThetvdbID)

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !config.ConfigCheck("imdb") {
		return
	}
	cfg_imdb := config.ConfigGet("imdb").Data.(config.ImdbConfig)

	dbserie, _ = database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{dbserie.ThetvdbID}})
	logger.Log.Debug("DbSeries get metadata for: ", dbserie.ThetvdbID)

	getfirstseries, _ := database.QuerySeries(database.Query{Select: "id, listname", Where: "Dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})

	var getconfigentry config.MediaTypeConfig //:= config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)

	if len(getfirstseries) >= 1 {
		for _, idx := range config.ConfigGetPrefix("serie_") {
			cfg_serie := config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound := false
			for idxlist := range cfg_serie.Lists {
				if cfg_serie.Lists[idxlist].Name == getfirstseries[0].Listname {
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
	addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
	if dbserie.Seriename == "" {
		addaliases = dbserie.GetMetadata(getconfigentry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
	}
	alternateNames := make([]string, 0, len(addaliases)+1)
	alternateNames = append(alternateNames, addaliases...)
	alternateNames = append(alternateNames, dbserie.Seriename)

	database.UpdateArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
		[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby},
		database.Query{Where: "id=?", WhereArgs: []interface{}{dbserie.ID}})

	logger.Log.Debug("DbSeries get metadata end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add titles for: ", dbserie.ThetvdbID)
	titles, _ := database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id=?", "select count(id) from dbserie_alternates where dbserie_id=?", dbserie.ID)
	titlegroup := dbserie.GetTitles(getconfigentry.Metadata_title_languages, cfg_general.SerieAlternateTitleMetaSourceImdb, cfg_general.SerieAlternateTitleMetaSourceTrakt)
	for idxalt := range titlegroup {
		titlefound := false
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
	titles, _ = database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id=?", "select count(id) from dbserie_alternates where dbserie_id=?", dbserie.ID)
	for idxalt := range alternateNames {
		titlefound := false
		for idxtitle := range titles {
			if strings.EqualFold(titles[idxtitle].Str, alternateNames[idxalt]) {
				titlefound = true
				break
			}
		}
		if !titlefound {
			database.InsertArray("dbserie_alternates",
				[]string{"dbserie_id", "title", "slug"},
				[]interface{}{dbserie.ID, alternateNames[idxalt], logger.StringToSlug(alternateNames[idxalt])})
		}
	}

	logger.Log.Debug("DbSeries add titles end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add serie end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries get episodes for: ", dbserie.ThetvdbID)
	dbepisode, _ := database.QueryDbserieEpisodes(database.Query{Select: "id, season, episode", Where: "dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})

	episodes := dbserie.GetEpisodes(getconfigentry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
	for idxepi := range episodes {
		epifound := false
		for idxdbepi := range dbepisode {
			if strings.EqualFold(episodes[idxepi].Season, dbepisode[idxdbepi].Season) && strings.EqualFold(episodes[idxepi].Episode, dbepisode[idxdbepi].Episode) {
				epifound = true
				database.UpdateArray("dbserie_episodes",
					[]string{"title", "first_aired", "overview", "poster", "runtime"},
					[]interface{}{episodes[idxepi].Title, episodes[idxepi].FirstAired, episodes[idxepi].Overview, episodes[idxepi].Poster, episodes[idxepi].Runtime},
					database.Query{Where: "id=?", WhereArgs: []interface{}{dbepisode[idxdbepi].ID}})
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

	foundseries, _ := database.QuerySeries(database.Query{Select: "id, listname", Where: "Dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})

	var getlist config.MediaListsConfig
	for idxserie := range foundseries {

		for _, idx := range config.ConfigGetPrefix("serie_") {
			cfg_serie := config.ConfigGet(idx.Name).Data.(config.MediaTypeConfig)

			listfound := false
			for idxlist := range cfg_serie.Lists {
				if cfg_serie.Lists[idxlist].Name == foundseries[idxserie].Listname {
					listfound = true
					getlist = cfg_serie.Lists[idxlist]
					break
				}
			}
			if listfound {
				break
			}
		}
		dbepisode, _ := database.QueryStaticColumnsOneInt("select id from dbserie_episodes where dbserie_id = ?", "select count(id) from dbserie_episodes where dbserie_id = ?", dbserie.ID)
		episodes, _ := database.QueryStaticColumnsOneInt("select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?", "select count(id) from serie_episodes where dbserie_id = ? and serie_id = ?", dbserie.ID, foundseries[idxserie].ID)
		for idxdbepi := range dbepisode {
			epifound := false
			for idxepi := range episodes {
				if episodes[idxepi].Num == dbepisode[idxdbepi].Num {
					epifound = true
					break
				}
			}
			if !epifound {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, foundseries[idxserie].ID, true, getlist.Template_quality, dbepisode[idxdbepi].Num})
			}
		}
	}

	logger.Log.Debug("DbSeries add episodes end for: ", dbserie.ThetvdbID)
}
