package importfeed

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
)

func getmoviemetalists(dbmovieid uint, imdb string, cfg string, listname string) {
	movietest, _ := database.QueryStaticColumnsOneStringOneInt("select listname as str, id as num from movies where dbmovie_id = ?", false, 0, dbmovieid)

	template_quality := config.Cfg.Media[cfg].ListsMap[listname].Template_quality

	toadd := true
	for idx2 := range movietest {
		for idxlist := range config.Cfg.Media[cfg].ListsMap[listname].Ignore_map_lists {
			if strings.EqualFold(movietest[idx2].Str, config.Cfg.Media[cfg].ListsMap[listname].Ignore_map_lists[idxlist]) {
				return
			}
		}
		if strings.EqualFold(movietest[idx2].Str, config.Cfg.Media[cfg].ListsMap[listname].Name) {
			toadd = false
			break
		}
		for idxlist := range config.Cfg.Media[cfg].ListsMap[listname].Replace_map_lists {
			if strings.EqualFold(movietest[idx2].Str, config.Cfg.Media[cfg].ListsMap[listname].Replace_map_lists[idxlist]) {
				if template_quality == "" {
					database.UpdateNamed("Update movies SET missing = :missing, listname = :listname, dbmovie_id = :dbmovie_id where id = :id", database.Movie{Listname: config.Cfg.Media[cfg].ListsMap[listname].Name, Missing: true, DbmovieID: dbmovieid, ID: uint(movietest[idx2].Num)})
				} else {
					database.UpdateNamed("Update movies SET missing = :missing, listname = :listname, dbmovie_id = :dbmovie_id, quality_profile = :quality_profile where id = :id", database.Movie{Listname: config.Cfg.Media[cfg].ListsMap[listname].Name, Missing: true, DbmovieID: dbmovieid, QualityProfile: template_quality, ID: uint(movietest[idx2].Num)})
				}
			}
		}
	}

	if toadd {
		logger.Log.GlobalLogger.Info("Insert Movie for", zap.String("imdb", imdb))
		_, err := database.InsertStatic("Insert into movies (missing, listname, dbmovie_id, quality_profile) values (?, ?, ?, ?)", true, config.Cfg.Media[cfg].ListsMap[listname].Name, dbmovieid, template_quality)
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
			return
		}
	}
}
func JobImportMovies(imdbid string, cfg string, listname string, addnew bool) {
	jobName := imdbid
	if jobName == "" {
		jobName = config.Cfg.Media[cfg].ListsMap[listname].Name
	}
	if jobName == "" {
		return
	}
	lastJob, ok := logger.GlobalCache.Get("ImportJobRunning")
	if ok {
		if lastJob.Value.(string) == jobName {
			logger.Log.GlobalLogger.Debug("Job already running", zap.String("job", jobName))
			return
		}
	}
	logger.GlobalCache.Set("ImportJobRunning", jobName, 5*time.Minute)

	counter, err := database.CountRowsStatic("select count() from dbmovies where imdb_id = ?", imdbid)
	if err != nil {
		return
	}
	if counter == 0 && !addnew {
		return
	}
	template_list := config.Cfg.Media[cfg].ListsMap[listname].Template_list
	if !config.ConfigCheck("list_" + template_list) {
		return
	}
	dbmovieadded := false
	var dbmovieID uint
	if counter == 0 && addnew {
		counter, err = database.CountRowsStatic("select count() from dbmovies where imdb_id = ?", imdbid)
		if err != nil {
			return
		}

		if !AllowMovieImport(imdbid, template_list) {
			return
		}

		if counter == 0 {
			logger.Log.GlobalLogger.Debug("Insert dbmovie for", zap.String("imdb", imdbid))
			dbresult, err := database.InsertStatic("insert into dbmovies (Imdb_ID) VALUES (?)", imdbid)
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			newid, err := dbresult.LastInsertId()

			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			dbmovieID = uint(newid)
			dbmovieadded = true
		} else {
			dbmovieID, _ = database.QueryColumnUint("select id from dbmovies where imdb_id = ?", imdbid)
		}
	}

	if dbmovieadded || !addnew {
		//logger.Log.GlobalLogger.Debug("Get metadata for", zap.String("imdb", imdbid))
		updatemoviemetadata(imdbid, cfg, true)
	}

	if addnew {
		if dbmovieID == 0 {
			dbmovieID, _ = database.QueryColumnUint("select id from dbmovies where imdb_id = ?", imdbid)
			if dbmovieID == 0 {
				return
			}
		}

		getmoviemetalists(dbmovieID, imdbid, cfg, listname)
	}
}

func AllowMovieImport(imdb string, listTemplate string) bool {
	if !config.ConfigCheck("list_" + listTemplate) {
		logger.Log.GlobalLogger.Warn("error list not found", zap.String("list", listTemplate))
		return false
	}

	if config.Cfg.Lists[listTemplate].MinVotes != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("select count() from imdb_ratings where tconst = ? and num_votes < ?", imdb, config.Cfg.Lists[listTemplate].MinVotes)
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Warn("error vote count too low for", zap.String("imdb", imdb))
			return false
		}
	}
	if config.Cfg.Lists[listTemplate].MinRating != 0 {
		countergenre, _ := database.ImdbCountRowsStatic("select count() from imdb_ratings where tconst = ? and average_rating < ?", imdb, config.Cfg.Lists[listTemplate].MinRating)
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Warn("error average vote too low for", zap.String("imdb", imdb))
			return false
		}
	}

	excludedgenres := config.Cfg.Lists[listTemplate].Excludegenre
	includedgenres := config.Cfg.Lists[listTemplate].Includegenre

	if len(excludedgenres) >= 1 || len(includedgenres) >= 1 {
		genrearr := &logger.InStringArrayStruct{Arr: database.QueryStaticStringArray("select genre from imdb_genres where tconst = ?", true, 0, imdb)}
		defer genrearr.Close()

		excludebygenre := false

		for idx := range excludedgenres {
			if logger.InStringArray(excludedgenres[idx], genrearr) {
				excludebygenre = true
				logger.Log.GlobalLogger.Warn("error excluded genre", zap.String("excluded", excludedgenres[idx]), zap.String("imdb", imdb))
				break
			}
		}
		if excludebygenre && len(excludedgenres) >= 1 {
			return false
		}

		includebygenre := false
		for idx := range includedgenres {
			if logger.InStringArray(includedgenres[idx], genrearr) {
				includebygenre = true
				break
			}
		}
		if !includebygenre && len(includedgenres) >= 1 {
			logger.Log.GlobalLogger.Warn("error included genre not found", zap.String("imdb", imdb))
			return false
		}
	}
	return true
}

func updatemoviemetadata(imdb string, cfg string, overwrite bool) {
	dbmovie, err := database.GetDbmovie(&database.Query{Where: "imdb_id = ?"}, imdb)
	if err != nil {
		return
	}
	dbmovie.Getmoviemetadata(overwrite)

	database.UpdateNamed("Update dbmovies SET Title = :title , Release_Date = :release_date , Year = :year , Adult = :adult , Budget = :budget , Genres = :genres , Original_Language = :original_language , Original_Title = :original_title , Overview = :overview , Popularity = :popularity , Revenue = :revenue , Runtime = :runtime , Spoken_Languages = :spoken_languages , Status = :status , Tagline = :tagline , Vote_Average = :vote_average , Vote_Count = :vote_count , Trakt_ID = :trakt_id , Moviedb_ID = :moviedb_id , Imdb_ID = :imdb_id , Freebase_M_ID = :freebase_m_id , Freebase_ID = :freebase_id , Facebook_ID = :facebook_id , Instagram_ID = :instagram_id , Twitter_ID = :twitter_id , URL = :url , Backdrop = :backdrop , Poster = :poster , Slug = :slug where id = :id", dbmovie)

	dbmovie.Getmoviemetatitles(cfg)

	if dbmovie.Title == "" {
		settitle, _ := database.QueryColumnString("select title from dbmovie_titles where dbmovie_id = ?", dbmovie.ID)
		database.UpdateColumnStatic("Update dbmovies SET Title = ? where id = ?", settitle, dbmovie.ID)
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

func findqualityyear1(imdb_id string) bool {
	qualityTemplate, _ := database.QueryColumnString("select movies.quality_profile from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id = ?", imdb_id)
	if qualityTemplate != "" {
		if config.Cfg.Quality[qualityTemplate].CheckYear1 {
			return true
		}
	}
	return false
}
func findimdbbytitle(title string, slugged string, yearint int) (string, bool, bool) {
	imdbtitles, _ := database.QueryStaticColumnsOneStringOneInt("select tconst as str,start_year as num from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", true, 0, title, title, slugged)

	if len(imdbtitles) >= 1 {
		for idximdb := range imdbtitles {
			found, found1 := checkifdbmovieyearmatches(imdbtitles[idximdb].Num, yearint)
			if found {
				return imdbtitles[idximdb].Str, found, found1
			}
			if found1 {
				if findqualityyear1(imdbtitles[idximdb].Str) {
					return imdbtitles[idximdb].Str, found, found1
				}
			}
		}
	}

	var dbyear uint
	imdbakas := database.QueryStaticStringArray("select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?", true, 0, title, slugged)
	for idxaka := range imdbakas {
		dbyear, _ = database.QueryImdbColumnUint("select start_year from imdb_titles where tconst = ?", imdbakas[idxaka])
		found, found1 := checkifdbmovieyearmatches(int(dbyear), yearint)
		if found {
			return imdbakas[idxaka], found, found1
		}
		if found1 {
			if findqualityyear1(imdbakas[idxaka]) {
				return imdbakas[idxaka], found, found1
			}
		}
	}

	return "", false, false
}

func findtmdbbytitle(title string, slugged string, yearint int) (string, bool, bool) {
	getmovie, _ := apiexternal.TmdbApi.SearchMovie(title)

	if len(getmovie.Results) >= 1 {
		var imdb_id string
		var moviedbexternal apiexternal.TheMovieDBTVExternal
		var err error
		var dbyear uint
		var found, found1 bool
		for idx2 := range getmovie.Results {
			imdb_id, _ = database.QueryColumnString("select imdb_id from dbmovies where moviedb_id = ?", getmovie.Results[idx2].ID)
			if imdb_id == "" {
				moviedbexternal, err = apiexternal.TmdbApi.GetMovieExternal(strconv.Itoa(getmovie.Results[idx2].ID))
				if err == nil {
					imdb_id = moviedbexternal.ImdbID
				} else {
					return "", false, false
				}
			}
			if imdb_id != "" {
				dbyear, _ = database.QueryImdbColumnUint("select start_year from imdb_titles where tconst = ?", imdb_id)
				found, found1 = checkifdbmovieyearmatches(int(dbyear), yearint)
				if found {
					return imdb_id, found, found1
				}
				if found1 {
					if findqualityyear1(imdb_id) {
						return imdb_id, found, found1
					}
				}
			}
		}
	}

	return "", false, false
}

func findomdbbytitle(title string, slugged string, yearint int) (string, bool, bool) {
	var searchomdb apiexternal.OmDBMovieSearchGlobal
	apiexternal.OmdbApi.SearchMovie(title, "", &searchomdb)

	if len(searchomdb.Search) >= 1 {
		var dbyear int
		var found, found1 bool
		for idximdb := range searchomdb.Search {
			dbyear, _ = strconv.Atoi(searchomdb.Search[idximdb].Year)
			found, found1 = checkifdbmovieyearmatches(dbyear, yearint)
			if found {
				return searchomdb.Search[idximdb].ImdbID, found, found1
			}
			if found1 {
				if findqualityyear1(searchomdb.Search[idximdb].ImdbID) {
					return searchomdb.Search[idximdb].ImdbID, found, found1
				}
			}
		}
	}
	return "", false, false
}

func StripTitlePrefixPostfix(title *string, qualityTemplate string) {
	if qualityTemplate == "" {
		logger.Log.GlobalLogger.Error("missing quality information")
		return
	}
	list := config.Cfg.Quality[qualityTemplate].TitleStripSuffixForSearch
	for idx := range list {
		*title = strings.Trim(logger.TrimStringInclAfterStringInsensitive(*title, list[idx]), " ")
	}
	list = config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch
	for idx := range list {
		*title = strings.Trim(logger.TrimStringPrefixInsensitive(*title, list[idx]), " ")
	}
}
func MovieFindDbIdByTitle(imdb string, title string, year string, searchtype string, addifnotfound bool) (uint, bool, bool) {
	var found1, found2 bool
	if imdb == "" {
		imdb, found1, found2 = MovieFindImdbIDByTitle(title, year, searchtype, addifnotfound)
		if !found1 && !found2 {
			return 0, false, false
		}
	} else {
		found1 = true
	}
	dbid, dbiderr := database.QueryColumnUint("select id from dbmovies where imdb_id = ?", imdb)
	if dbiderr != nil {
		return 0, false, false
	}
	return dbid, found1, found2
}

func MovieFindImdbIDByTitle(title string, year string, searchtype string, addifnotfound bool) (string, bool, bool) {
	yearint, _ := strconv.Atoi(year)

	slugged := logger.StringToSlug(title)
	dbmoviestemp, _ := database.QueryStaticColumnsOneStringOneInt("select imdb_id as str, year as num from dbmovies where title = ? COLLATE NOCASE OR slug = ?", false, 0, title, slugged)

	var found, found1 bool
	for idx := range dbmoviestemp {
		logger.Log.GlobalLogger.Debug("Find movie by title - check imdb", zap.String("imdb", dbmoviestemp[idx].Str))
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, yearint)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}
	dbmoviestemp, _ = database.QueryStaticColumnsOneStringOneInt("select dbmovies.imdb_id as str, dbmovies.year as num from dbmovie_titles inner join dbmovies on dbmovies.id=dbmovie_titles.dbmovie_id where dbmovie_titles.title = ? COLLATE NOCASE OR dbmovie_titles.slug = ?", false, 0, title, slugged)

	for idx := range dbmoviestemp {
		logger.Log.GlobalLogger.Debug("Find movie by alttitle - check imdb", zap.String("imdb", dbmoviestemp[idx].Str))
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, yearint)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}
	if addifnotfound {
		searchprovider := []string{"imdb", "tmdb", "omdb"}
		if strings.EqualFold(searchtype, "rss") {
			rssmeta := config.Cfg.General.MovieRSSMetaSourcePriority
			if len(rssmeta) >= 1 {
				searchprovider = rssmeta
			}
		} else {
			parsemeta := config.Cfg.General.MovieParseMetaSourcePriority
			if len(parsemeta) >= 1 {
				searchprovider = parsemeta
			}
		}
		if len(searchprovider) >= 1 {
			var imdb string
			useimdb := config.Cfg.General.MovieMetaSourceImdb
			usetmdb := config.Cfg.General.MovieMetaSourceTmdb
			useomdb := config.Cfg.General.MovieMetaSourceOmdb
			for idxprovider := range searchprovider {
				found = false
				found1 = false
				switch searchprovider[idxprovider] {
				case "imdb":
					if useimdb {
						imdb, found, found1 = findimdbbytitle(title, slugged, yearint)
					}
				case "tmdb":
					if usetmdb {
						imdb, found, found1 = findtmdbbytitle(title, slugged, yearint)
					}
				case "omdb":
					if useomdb {
						imdb, found, found1 = findomdbbytitle(title, slugged, yearint)
					}
				}
				if found || found1 {
					return imdb, found, found1
				}
			}
		}
	}

	return "", false, false
}

func FindDbserieByName(title string) (uint, error) {
	id, err := database.QueryColumnUint("select id from dbseries where seriename = ? COLLATE NOCASE", title)
	if id == 0 {
		slugged := logger.StringToSlug(title)
		id, err = database.QueryColumnUint("select id from dbseries where slug = ?", slugged)
		if id == 0 {
			id, err = database.QueryColumnUint("select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE or Slug = ?", title, slugged)
		}
	}
	return id, err
}

var errNotFound error = errors.New("not found")

func FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid uint, identifier string, season string, episode string) (uint, error) {
	if len(season) >= 1 && len(episode) >= 1 {
		id, err := database.QueryColumnUint("select id from dbserie_episodes where dbserie_id = ? and season = ? COLLATE NOCASE and episode = ? COLLATE NOCASE", dbserieid, strings.TrimLeft(season, "0"), strings.TrimLeft(episode, "0"))
		if err == nil {
			return id, err
		}
	}
	if len(identifier) >= 1 {
		id, err := database.QueryColumnUint("select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", dbserieid, identifier)
		if err == nil {
			return id, err
		}
		if strings.Contains(identifier, ".") {
			id, err = database.QueryColumnUint("select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", dbserieid, strings.Replace(identifier, ".", "-", -1))
			if err == nil {
				return id, err
			}
		}
		if strings.Contains(identifier, " ") {
			id, err = database.QueryColumnUint("select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", dbserieid, strings.Replace(identifier, " ", "-", -1))
			if err == nil {
				return id, err
			}
		}
	}
	return 0, errNotFound
}
func GetEpisodeArray(identifiedby string, identifier string) *logger.InStringArrayStruct {
	str1, str2 := config.RegexGetMatchesStr1Str2("RegexSeriesIdentifier", identifier)
	if str1 == "" && str2 == "" {
		return nil
	}

	if identifiedby == "date" {
		str1 = str2
		str1 = strings.Replace(str1, " ", "-", -1)
		str1 = strings.Replace(str1, ".", "-", -1)
	}
	episodeArray := &logger.InStringArrayStruct{Arr: make([]string, 0, 5)}
	if strings.ContainsRune(str1, []rune("E")[0]) {
		episodeArray.Arr = strings.Split(str1, "E")
	} else if strings.ContainsRune(str1, []rune("X")[0]) {
		episodeArray.Arr = strings.Split(str1, "X")
	} else if strings.ContainsRune(str1, []rune("e")[0]) {
		episodeArray.Arr = strings.Split(str1, "e")
	} else if strings.ContainsRune(str1, []rune("x")[0]) {
		episodeArray.Arr = strings.Split(str1, "x")
	} else if strings.ContainsRune(str1, []rune("-")[0]) && identifiedby != "date" {
		episodeArray.Arr = strings.Split(str1, "-")
	}
	if len(episodeArray.Arr) == 0 && identifiedby == "date" {
		episodeArray.Arr = append(episodeArray.Arr, str1)
	}
	return episodeArray
}

func JobImportDbSeries(serieconfig *config.SerieConfig, cfg string, listname string, checkall bool, addnew bool) {
	jobName := serieconfig.Name
	if jobName == "" {
		jobName = config.Cfg.Media[cfg].ListsMap[listname].Name
	}
	if jobName == "" {
		logger.Log.GlobalLogger.Debug("Job skipped - no name")
		return
	}

	lastJob, ok := logger.GlobalCache.Get("ImportJobRunning")
	if ok {
		if lastJob.Value.(string) == jobName {
			logger.Log.GlobalLogger.Debug("Job already running", zap.String("job", jobName))
			return
		}
	}
	logger.GlobalCache.Set("ImportJobRunning", jobName, 5*time.Minute)

	if !addnew && (cfg == "" || listname == "") {
		listname, _ = database.QueryColumnString("select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", serieconfig.TvdbID)

		if len(listname) >= 1 {
			cfg = config.FindconfigTemplateOnList("serie_", listname)
		}
	}
	if cfg == "" || listname == "" {
		logger.Log.GlobalLogger.Info("Series not fetched because list or template is empty", zap.String("config", config.Cfg.Media[cfg].Name), zap.String("Listname", listname))
		return
	}

	var dbserie database.Dbserie
	if serieconfig.TvdbID != 0 {
		var err error
		dbserie, err = database.GetDbserie(&database.Query{Where: "thetvdb_id = ?"}, serieconfig.TvdbID)
		if err != nil && !addnew {
			logger.Log.GlobalLogger.Debug("Job skipped - getdata failed", zap.String("job", jobName))
			return
		}
	}
	dbserieadded := false
	if dbserie.Seriename == "" {
		dbserie.Seriename = serieconfig.Name
	}
	if dbserie.Identifiedby == "" {
		dbserie.Identifiedby = serieconfig.Identifiedby
	}

	var counter int
	var err error
	if strings.EqualFold(serieconfig.Source, "none") && addnew {
		counter, _ = database.CountRowsStatic("select count() from dbseries where seriename = ? COLLATE NOCASE", serieconfig.Name)

		if counter == 0 {
			dbserieadded = true
			inres, err := database.InsertNamed("insert into dbseries (seriename, aliases, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, trakt_id, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, poster, fanart, identifiedby) values (:seriename, :aliases, :season, :status, :firstaired, :network, :runtime, :language, :genre, :overview, :rating, :siterating, :siterating_count, :slug, :trakt_id, :imdb_id, :thetvdb_id, :freebase_m_id, :freebase_id, :tvrage_id, :facebook, :instagram, :twitter, :banner, :poster, :fanart, :identifiedby)", dbserie)
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			newid, err := inres.LastInsertId()
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			dbserie.ID = uint(newid)
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
			for idxalt := range serieconfig.AlternateName {
				if serieconfig.AlternateName[idxalt] == "" {
					continue
				}
				counter, err = database.CountRowsStatic("select count() from dbserie_alternates where Dbserie_id = ? and title = ? COLLATE NOCASE", dbserie.ID, serieconfig.AlternateName[idxalt])
				if err != nil {
					continue
				}
				if counter == 0 {
					database.InsertStatic("Insert into dbserie_alternates (title, slug, dbserie_id) values (?, ?, ?)", serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt]), dbserie.ID)
				}
			}
		} else {
			dbserie.ID, err = database.QueryColumnUint("select id from dbseries where seriename = ? COLLATE NOCASE", serieconfig.Name)

			if err != nil {
				logger.Log.GlobalLogger.Debug("Job skipped - id not fetched", zap.String("job", jobName))
				return
			}
		}
	}
	if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
		dbserie.ThetvdbID = serieconfig.TvdbID
		counter, _ = database.CountRowsStatic("select count() from dbseries where thetvdb_id = ?", serieconfig.TvdbID)

		if counter == 0 && addnew {
			if !config.ConfigCheck("imdb") {
				return
			}
			logger.Log.GlobalLogger.Debug("Insert dbseries for", zap.Int("tvdb", serieconfig.TvdbID))
			inres, err := database.InsertStatic("insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)", dbserie.Seriename, dbserie.ThetvdbID, dbserie.Identifiedby)
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			newid, err := inres.LastInsertId()
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
			dbserieadded = true
			dbserie.ID = uint(newid)
		}
		if dbserie.ID != 0 && (dbserieadded || !addnew) {
			//Update Metadata
			overwrite := !addnew
			logger.Log.GlobalLogger.Debug("Get metadata for", zap.Int("tvdb", serieconfig.TvdbID))
			addaliases := dbserie.GetMetadata("", config.Cfg.General.SerieMetaSourceTmdb, config.Cfg.General.SerieMetaSourceTrakt, overwrite, true)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(config.Cfg.Media[cfg].Metadata_language, config.Cfg.General.SerieMetaSourceTmdb, config.Cfg.General.SerieMetaSourceTrakt, overwrite, true)
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases...)
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
			addaliases = nil

			database.UpdateNamed("Update dbseries SET Seriename = :seriename, Aliases = :aliases, Season = :season, Status = :status, Firstaired = :firstaired, Network = :network, Runtime = :runtime, Language = :language, Genre = :genre, Overview = :overview, Rating = :rating, Siterating = :siterating, Siterating_Count = :siterating_count, Slug = :slug, Trakt_ID = :trakt_id, Imdb_ID = :imdb_id, Thetvdb_ID = :thetvdb_id, Freebase_M_ID = :freebase_m_id, Freebase_ID = :freebase_id, Tvrage_ID = :tvrage_id, Facebook = :facebook, Instagram = :instagram, Twitter = :twitter, Banner = :banner, Poster = :poster, Fanart = :fanart, Identifiedby = :identifiedby where id = :id", dbserie)

			titles, _ := database.QueryStaticColumnsOneStringOneInt("select title as str, id as num from dbserie_alternates where dbserie_id = ?", false, 0, dbserie.ID)

			titlegroup := dbserie.GetTitles(cfg, config.Cfg.General.SerieAlternateTitleMetaSourceImdb, config.Cfg.General.SerieAlternateTitleMetaSourceTrakt)
			var titlefound bool
			for idxadd := range serieconfig.AlternateName {
				titlefound = false
				for idxcheck := range titlegroup {
					if titlegroup[idxcheck].Title == serieconfig.AlternateName[idxadd] {
						titlefound = true
						break
					}
				}
				if !titlefound {
					titlegroup = append(titlegroup, database.DbserieAlternate{Title: serieconfig.AlternateName[idxadd]})
				}
			}

			var toadd bool
			for idxalt := range titlegroup {
				if titlegroup[idxalt].Title == "" {
					continue
				}
				toadd = true
				for idx2 := range titles {
					if strings.EqualFold(titles[idx2].Str, titlegroup[idxalt].Title) {
						toadd = false
						break
					}
				}
				if toadd {
					database.InsertStatic("Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", titlegroup[idxalt].Title, titlegroup[idxalt].Slug, dbserie.ID, titlegroup[idxalt].Region)
				}
			}
			titlegroup = nil
			titles = nil

			if checkall || dbserieadded || !addnew {
				if strings.EqualFold(serieconfig.Source, "none") {
					//Don't add episodes automatically
				} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
					logger.Log.GlobalLogger.Debug("Get episodes for", zap.Int("tvdb", serieconfig.TvdbID))

					dbserie.InsertEpisodes(config.Cfg.Media[cfg].Metadata_language, config.Cfg.General.SerieMetaSourceTrakt)
				}
			}
		}
		if counter >= 1 && addnew {
			dbserie.ID, err = database.QueryColumnUint("select id from dbseries where thetvdb_id = ?", serieconfig.TvdbID)
			if err != nil {
				logger.Log.GlobalLogger.Debug("Job skipped - id not fetched", zap.String("job", jobName))
				return
			}
		}
	}
	if dbserie.ID != 0 && addnew {
		//Add Entry in SeriesTable

		if listname == "" {
			logger.Log.GlobalLogger.Debug("Series skip for", zap.String("serie", serieconfig.Name))
			return
		}
		serietest, _ := database.QueryStaticColumnsOneStringOneInt("select lower(listname) as str, id as num from series where dbserie_id = ?", false, 0, dbserie.ID)

		for idx2 := range serietest {
			for idx := range config.Cfg.Media[cfg].ListsMap[listname].Ignore_map_lists {
				if strings.EqualFold(serietest[idx2].Str, config.Cfg.Media[cfg].ListsMap[listname].Ignore_map_lists[idx]) {
					logger.Log.GlobalLogger.Debug("Series skip2 for", zap.String("serie", serieconfig.Name))
					serietest = nil
					return
				}
			}
			for idxreplace := range config.Cfg.Media[cfg].ListsMap[listname].Replace_map_lists {
				if strings.EqualFold(serietest[idx2].Str, config.Cfg.Media[cfg].ListsMap[listname].Replace_map_lists[idxreplace]) {
					database.UpdateNamed("Update series SET listname = :listname, dbserie_id = :dbserie_id where id = :id", database.Serie{Listname: config.Cfg.Media[cfg].ListsMap[listname].Name, DbserieID: dbserie.ID, ID: uint(serietest[idx2].Num)})
				}
			}
		}
		serietest = nil

		//var serie database.Serie

		if database.CountRowsStaticNoError("Select count() from series where dbserie_id = ? and listname = ?", dbserie.ID, config.Cfg.Media[cfg].ListsMap[listname].Name) == 0 {
			logger.Log.GlobalLogger.Debug("Add series for", zap.Int("tvdb", serieconfig.TvdbID), zap.String("Listname", config.Cfg.Media[cfg].ListsMap[listname].Name))
			_, err := database.InsertStatic("Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)", dbserie.ID, config.Cfg.Media[cfg].ListsMap[listname].Name, serieconfig.Target, serieconfig.SearchSpecials, serieconfig.DontSearch, serieconfig.DontUpgrade)
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
		}
	}

	if dbserie.ID != 0 {
		//logger.Log.GlobalLogger.Info("Refresh Episodes of list ", zap.String("job", jobName))
		series := database.QueryStaticIntArray("select id as num from series where dbserie_id = ?", 0, dbserie.ID)

		dbepisode := database.QueryStaticIntArray("select id as num from dbserie_episodes where dbserie_id = ?", 0, dbserie.ID)

		episodes := new(logger.InIntArrayStruct)
		for idxserie := range series {
			episodes.Arr = database.QueryStaticIntArray(
				"select dbserie_episode_id as num from serie_episodes where dbserie_id = ? and serie_id = ?",
				0,
				dbserie.ID, series[idxserie])

			for idxdbepi := range dbepisode {
				if !logger.InIntArray(dbepisode[idxdbepi], episodes) {
					database.InsertStatic("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, ?, ?, ?)", dbserie.ID, series[idxserie], true, config.Cfg.Media[cfg].ListsMap[listname].Template_quality, dbepisode[idxdbepi])
				}
			}
		}
		episodes.Close()
		series = nil
		dbepisode = nil
	}
}
