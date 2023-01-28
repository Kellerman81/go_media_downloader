package importfeed

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
)

const querycountdbmoviesbyimdb = "select count() from dbmovies where imdb_id = ?"
const querycountratingbyvotes = "select count() from imdb_ratings where tconst = ? and num_votes < ?"
const querycountratingbyrating = "select count() from imdb_ratings where tconst = ? and average_rating < ?"
const querycountdbseriesbyseriename = "select count() from dbseries where seriename = ? COLLATE NOCASE"
const querycountdbseriesbytvdbid = "select count() from dbseries where thetvdb_id = ?"

const queryiddbmoviesbyimdb = "select id from dbmovies where imdb_id = ?"
const queryiddbseriesbyname = "select id from dbseries where seriename = ? COLLATE NOCASE"
const queryiddbseriesbyslug = "select id from dbseries where slug = ?"
const queryiddbseriesepisodesbyseason = "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?"
const queryiddbseriesepisodesbyidentifier = "select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE"

const querygenregenresbytconst = "select genre from imdb_genres where tconst = ?"
const queryfirsttitlemovie = "select title from dbmovie_titles where dbmovie_id = ? limit 1"
const queryqualmoviesbyimdb = "select movies.quality_profile from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id = ?"
const querytconstyeartitlesbytitle = "select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)"
const querytconstakasbytitle = "select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?"
const queryyeartitlesbytconst = "select start_year from imdb_titles where tconst = ?"
const queryimdbdbmoviesbymoviedbid = "select imdb_id from dbmovies where moviedb_id = ?"
const queryimdbyeardbmoviesbytitle = "select imdb_id, year from dbmovies where title = ? COLLATE NOCASE OR slug = ?"
const queryimdbyeardbmoviestitlesbytitle = "select dbmovies.imdb_id, dbmovies.year from dbmovie_titles inner join dbmovies on dbmovies.id=dbmovie_titles.dbmovie_id where dbmovie_titles.title = ? COLLATE NOCASE OR dbmovie_titles.slug = ?"
const querydbserieidseriealternatebytitle = "select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE or Slug = ?"
const queryupdateseries = "Update dbseries SET Seriename = :seriename, Aliases = :aliases, Season = :season, Status = :status, Firstaired = :firstaired, Network = :network, Runtime = :runtime, Language = :language, Genre = :genre, Overview = :overview, Rating = :rating, Siterating = :siterating, Siterating_Count = :siterating_count, Slug = :slug, Trakt_ID = :trakt_id, Imdb_ID = :imdb_id, Thetvdb_ID = :thetvdb_id, Freebase_M_ID = :freebase_m_id, Freebase_ID = :freebase_id, Tvrage_ID = :tvrage_id, Facebook = :facebook, Instagram = :instagram, Twitter = :twitter, Banner = :banner, Poster = :poster, Fanart = :fanart, Identifiedby = :identifiedby where id = :id"
const queryupdatemovie = "Update dbmovies SET Title = :title , Release_Date = :release_date , Year = :year , Adult = :adult , Budget = :budget , Genres = :genres , Original_Language = :original_language , Original_Title = :original_title , Overview = :overview , Popularity = :popularity , Revenue = :revenue , Runtime = :runtime , Spoken_Languages = :spoken_languages , Status = :status , Tagline = :tagline , Vote_Average = :vote_average , Vote_Count = :vote_count , Trakt_ID = :trakt_id , Moviedb_ID = :moviedb_id , Imdb_ID = :imdb_id , Freebase_M_ID = :freebase_m_id , Freebase_ID = :freebase_id , Facebook_ID = :facebook_id , Instagram_ID = :instagram_id , Twitter_ID = :twitter_id , URL = :url , Backdrop = :backdrop , Poster = :poster , Slug = :slug where id = :id"

var importJobRunning string
var errNotFound = errors.New("not found")

func JobImportMovies(imdbid string, cfgp *config.MediaTypeConfig, listname string, addnew bool) uint {
	jobName := imdbid
	if cfgp.Name == "" {
		logger.Log.GlobalLogger.Debug("Job cfpg missing", zap.String("job", jobName))
		return 0
	}
	if !addnew && (cfgp.Name == "" || listname == "") {
		database.QueryColumn(&database.Querywithargs{QueryString: "select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)", Args: []interface{}{imdbid}}, &listname)
	}
	if jobName == "" {
		jobName = cfgp.ListsMap[listname].Name
	}
	if jobName == "" {
		return 0
	}
	if importJobRunning == jobName {
		logger.Log.GlobalLogger.Debug("Job already running", zap.String("job", jobName))
		return 0
	}
	importJobRunning = jobName

	var counter int
	database.QueryColumn(&database.Querywithargs{QueryString: querycountdbmoviesbyimdb, Args: []interface{}{imdbid}}, &counter)
	if counter == 0 && !addnew {
		return 0
	}
	if !config.Check("list_" + cfgp.ListsMap[listname].TemplateList) {
		return 0
	}
	var dbmovieadded bool
	var dbmovieID uint
	if counter == 0 && addnew {
		if !AllowMovieImport(imdbid, cfgp.ListsMap[listname].TemplateList) {
			return 0
		}

		logger.Log.GlobalLogger.Debug("Insert dbmovie for", zap.Stringp("imdb", &imdbid))
		dbresult, err := database.InsertStatic(&database.Querywithargs{QueryString: "insert into dbmovies (Imdb_ID) VALUES (?)", Args: []interface{}{imdbid}})
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
			return 0
		}
		newid, err := dbresult.LastInsertId()

		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
			return 0
		}
		dbmovieID = uint(newid)
		dbmovieadded = true
	}
	if dbmovieID == 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: queryiddbmoviesbyimdb, Args: []interface{}{imdbid}}, &dbmovieID)
	}
	if dbmovieID == 0 {
		return 0
	}

	if dbmovieadded || !addnew {
		logger.Log.GlobalLogger.Debug("Get metadata for", zap.Stringp("imdb", &imdbid))
		var dbmovie database.Dbmovie
		if database.GetDbmovie(&database.Querywithargs{Query: database.QueryFilterByImdb, Args: []interface{}{imdbid}}, &dbmovie) == nil {

			dbmovie.Getmoviemetadata(true)

			database.UpdateNamed(queryupdatemovie, dbmovie)

			dbmovie.Getmoviemetatitles(cfgp)

			if dbmovie.Title == "" {
				database.QueryColumn(&database.Querywithargs{QueryString: queryfirsttitlemovie, Args: []interface{}{dbmovie.ID}}, &dbmovie.Title)
				if dbmovie.Title != "" {
					database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update dbmovies SET Title = ? where id = ?", Args: []interface{}{dbmovie.Title, dbmovie.ID}})
				}
			}
		}
	}

	if !addnew {
		return dbmovieID
	}
	if dbmovieID == 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: queryiddbmoviesbyimdb, Args: []interface{}{imdbid}}, &dbmovieID)
		if dbmovieID == 0 {
			return 0
		}
	}
	listmap := cfgp.GetList(listname)
	var movietest []database.DbstaticOneStringOneInt
	database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: "select listname, id from movies where dbmovie_id = ?", Args: []interface{}{dbmovieID}}, &movietest)

	var broke, toadd bool
	for idx2 := range movietest {
		for idxlist := range listmap.IgnoreMapLists {
			if strings.EqualFold(movietest[idx2].Str, listmap.IgnoreMapLists[idxlist]) {
				broke = true
				break
			}
		}
		if broke {
			break
		}
		if strings.EqualFold(movietest[idx2].Str, listmap.Name) {
			toadd = true
			break
		}
		for idxlist := range listmap.ReplaceMapLists {
			if strings.EqualFold(movietest[idx2].Str, listmap.ReplaceMapLists[idxlist]) {
				if listmap.TemplateQuality == "" {
					database.UpdateNamed("Update movies SET missing = :missing, listname = :listname, dbmovie_id = :dbmovie_id where id = :id", database.Movie{Listname: listmap.Name, Missing: true, DbmovieID: dbmovieID, ID: uint(movietest[idx2].Num)})
				} else {
					database.UpdateNamed("Update movies SET missing = :missing, listname = :listname, dbmovie_id = :dbmovie_id, quality_profile = :quality_profile where id = :id", database.Movie{Listname: listmap.Name, Missing: true, DbmovieID: dbmovieID, QualityProfile: listmap.TemplateQuality, ID: uint(movietest[idx2].Num)})
				}
			}
		}
	}
	movietest = nil

	if !toadd {
		logger.Log.GlobalLogger.Info("Insert Movie for", zap.Stringp("imdb", &imdbid))
		_, err := database.InsertStatic(&database.Querywithargs{QueryString: "Insert into movies (missing, listname, dbmovie_id, quality_profile) values (?, ?, ?, ?)", Args: []interface{}{true, listmap.Name, dbmovieID, listmap.TemplateQuality}})
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
		}
	}
	listmap.Close()
	return dbmovieID
}

func AllowMovieImport(imdb string, listTemplate string) bool {
	if !config.Check("list_" + listTemplate) {
		logger.Log.GlobalLogger.Warn("error list not found", zap.String("list", listTemplate))
		return false
	}

	if config.Cfg.Lists[listTemplate].MinVotes != 0 {
		countergenre, _ := database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: querycountratingbyvotes, Args: []interface{}{imdb, config.Cfg.Lists[listTemplate].MinVotes}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Warn("error vote count too low for", zap.Stringp("imdb", &imdb))
			return false
		}
	}
	if config.Cfg.Lists[listTemplate].MinRating != 0 {
		countergenre, _ := database.ImdbCountRowsStatic(&database.Querywithargs{QueryString: querycountratingbyrating, Args: []interface{}{imdb, config.Cfg.Lists[listTemplate].MinRating}})
		if countergenre >= 1 {
			logger.Log.GlobalLogger.Warn("error average vote too low for", zap.Stringp("imdb", &imdb))
			return false
		}
	}

	if len(config.Cfg.Lists[listTemplate].Excludegenre) == 0 && len(config.Cfg.Lists[listTemplate].Includegenre) == 0 {
		return true
	}
	var genrearr logger.InStringArrayStruct
	database.QueryStaticStringArray(true, 0, &database.Querywithargs{QueryString: querygenregenresbytconst, Args: []interface{}{imdb}}, &genrearr.Arr)
	defer genrearr.Close()
	var excludebygenre bool

	for idx := range config.Cfg.Lists[listTemplate].Excludegenre {
		if logger.InStringArray(config.Cfg.Lists[listTemplate].Excludegenre[idx], &genrearr) {
			excludebygenre = true
			logger.Log.GlobalLogger.Warn("error excluded genre", zap.Stringp("excluded", &config.Cfg.Lists[listTemplate].Excludegenre[idx]), zap.Stringp("imdb", &imdb))
			break
		}
	}
	if excludebygenre && len(config.Cfg.Lists[listTemplate].Excludegenre) >= 1 {
		return false
	}

	var includebygenre bool
	for idx := range config.Cfg.Lists[listTemplate].Includegenre {
		if logger.InStringArray(config.Cfg.Lists[listTemplate].Includegenre[idx], &genrearr) {
			includebygenre = true
			break
		}
	}
	if !includebygenre && len(config.Cfg.Lists[listTemplate].Includegenre) >= 1 {
		logger.Log.GlobalLogger.Warn("error included genre not found", zap.Stringp("imdb", &imdb))
		return false
	}
	return true
}

func checkifdbmovieyearmatches(dbmovieyear int, haveyear int) (bool, bool) {
	if dbmovieyear == 0 || haveyear == 0 {
		return false, false
	}
	if dbmovieyear == haveyear {
		return true, false
	}

	if dbmovieyear == haveyear+1 || dbmovieyear == haveyear-1 {

		return false, true
	}
	return false, false
}

func findqualityyear1(imdbID string) bool {
	var qualityTemplate string
	database.QueryColumn(&database.Querywithargs{QueryString: queryqualmoviesbyimdb, Args: []interface{}{imdbID}}, &qualityTemplate)
	if qualityTemplate == "" {
		return false
	}
	return config.Cfg.Quality[qualityTemplate].CheckYear1
}

func findimdbbytitle(title string, slugged string, yearint int) (string, bool, bool) {
	var imdbtitles []database.DbstaticOneStringOneInt
	database.QueryStaticColumnsOneStringOneInt(true, 0, &database.Querywithargs{QueryString: querytconstyeartitlesbytitle, Args: []interface{}{title, title, slugged}}, &imdbtitles)
	defer logger.ClearVar(&imdbtitles)
	var found, found1 bool
	if len(imdbtitles) >= 1 {
		for idximdb := range imdbtitles {
			found, found1 = checkifdbmovieyearmatches(imdbtitles[idximdb].Num, yearint)
			if found {
				return imdbtitles[idximdb].Str, found, found1
			}
			if found1 && findqualityyear1(imdbtitles[idximdb].Str) {
				return imdbtitles[idximdb].Str, found, found1
			}
		}
	}

	var dbyear int
	var imdbakas logger.InStringArrayStruct
	database.QueryStaticStringArray(true, 0, &database.Querywithargs{QueryString: querytconstakasbytitle, Args: []interface{}{title, slugged}}, &imdbakas.Arr)
	defer imdbakas.Close()

	for idxaka := range imdbakas.Arr {
		database.QueryImdbColumn(&database.Querywithargs{QueryString: queryyeartitlesbytconst, Args: []interface{}{imdbakas.Arr[idxaka]}}, &dbyear)
		found, found1 = checkifdbmovieyearmatches(dbyear, yearint)
		if found {
			return imdbakas.Arr[idxaka], found, found1
		}
		if found1 && findqualityyear1(imdbakas.Arr[idxaka]) {
			return imdbakas.Arr[idxaka], found, found1
		}
	}

	return "", false, false
}

func findtmdbbytitle(title string, yearint int) (string, bool, bool) {
	getmovie, _ := apiexternal.TmdbAPI.SearchMovie(title)
	defer getmovie.Close()
	if len(getmovie.Results) >= 1 {
		var imdbID string
		var moviedbexternal *apiexternal.TheMovieDBTVExternal
		var err error
		var dbyear int
		var found, found1 bool
		for idx2 := range getmovie.Results {
			database.QueryColumn(&database.Querywithargs{QueryString: queryimdbdbmoviesbymoviedbid, Args: []interface{}{getmovie.Results[idx2].ID}}, &imdbID)
			if imdbID == "" {
				moviedbexternal, err = apiexternal.TmdbAPI.GetMovieExternal(getmovie.Results[idx2].ID)
				if err == nil {
					imdbID = moviedbexternal.ImdbID
					moviedbexternal = nil
				} else {
					return "", false, false
				}
			}
			if imdbID != "" {
				database.QueryImdbColumn(&database.Querywithargs{QueryString: queryyeartitlesbytconst, Args: []interface{}{imdbID}}, &dbyear)
				found, found1 = checkifdbmovieyearmatches(dbyear, yearint)
				if found {
					return imdbID, found, found1
				}
				if found1 && findqualityyear1(imdbID) {
					return imdbID, found, found1
				}
			}
		}
	}
	return "", false, false
}

func findomdbbytitle(title string, yearint int) (string, bool, bool) {
	var searchomdb apiexternal.OmDBMovieSearchGlobal
	apiexternal.OmdbAPI.SearchMovie(title, "", &searchomdb)
	defer searchomdb.Close()
	if len(searchomdb.Search) >= 1 {
		var found, found1 bool
		for idximdb := range searchomdb.Search {
			found, found1 = checkifdbmovieyearmatches(logger.StringToInt(searchomdb.Search[idximdb].Year), yearint)
			if found {
				return searchomdb.Search[idximdb].ImdbID, found, found1
			}
			if found1 && findqualityyear1(searchomdb.Search[idximdb].ImdbID) {
				return searchomdb.Search[idximdb].ImdbID, found, found1
			}
		}
	}
	return "", false, false
}

func StripTitlePrefixPostfix(title string, qualityTemplate string) string {
	if qualityTemplate == "" {
		logger.Log.GlobalLogger.Error("missing quality information")
		return title
	}
	lowertitle := strings.ToLower(title)
	var trimidx int
	for idx := range config.Cfg.Quality[qualityTemplate].TitleStripSuffixForSearch {
		if strings.Contains(title, config.Cfg.Quality[qualityTemplate].TitleStripSuffixForSearch[idx]) {
			trimidx = strings.Index(title, config.Cfg.Quality[qualityTemplate].TitleStripSuffixForSearch[idx])
			if trimidx != -1 {
				title = strings.TrimRight(title[:trimidx], "-. ")
				lowertitle = strings.TrimRight(lowertitle[:trimidx], "-. ")
			}
		} else {
			trimidx = strings.Index(lowertitle, strings.ToLower(config.Cfg.Quality[qualityTemplate].TitleStripSuffixForSearch[idx]))
			if trimidx != -1 {
				title = strings.TrimRight(title[:trimidx], "-. ")
				lowertitle = strings.TrimRight(lowertitle[:trimidx], "-. ")
			}
		}
		//title = strings.Trim(logger.TrimStringInclAfterStringInsensitive(title, list[idx]), " ")
	}
	for idx := range config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch {
		if strings.HasPrefix(title, config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx]) {
			title = strings.TrimLeft(title[(strings.Index(title, config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx])+len(config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx])):], "-. ")
		} else if len(config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx]) <= len(title) && strings.EqualFold(title[:len(config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx])], config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx]) {
			title = strings.TrimLeft(title[len(config.Cfg.Quality[qualityTemplate].TitleStripPrefixForSearch[idx]):], "-. ")
		}
		//title = strings.Trim(logger.TrimStringPrefixInsensitive(title, list[idx]), " ")
	}
	return title
}

func MovieFindDbIDByTitle(imdb string, title string, year int, searchtype string, addifnotfound bool) (uint, bool, bool) {
	var found1, found2 bool
	if imdb == "" {
		imdb, found1, found2 = MovieFindImdbIDByTitle(title, year, searchtype, addifnotfound)
		if !found1 && !found2 {
			return 0, false, false
		}
	} else {
		found1 = true
	}
	var dbid uint
	if database.QueryColumn(&database.Querywithargs{QueryString: queryiddbmoviesbyimdb, Args: []interface{}{imdb}}, &dbid) != nil {
		return 0, false, false
	}
	return dbid, found1, found2
}

func MovieFindImdbIDByTitle(title string, year int, searchtype string, addifnotfound bool) (string, bool, bool) {
	slugged := logger.StringToSlug(title)
	var dbmoviestemp []database.DbstaticOneStringOneInt
	database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: queryimdbyeardbmoviesbytitle, Args: []interface{}{title, slugged}}, &dbmoviestemp)
	defer logger.ClearVar(&dbmoviestemp)
	var found, found1 bool
	for idx := range dbmoviestemp {
		//logger.Log.GlobalLogger.Debug("Find movie by title - check imdb", zap.String("imdb", dbmoviestemp[idx].Str))
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, year)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}
	dbmoviestemp = []database.DbstaticOneStringOneInt{}
	database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: queryimdbyeardbmoviestitlesbytitle, Args: []interface{}{title, slugged}}, &dbmoviestemp)

	for idx := range dbmoviestemp {
		//logger.Log.GlobalLogger.Debug("Find movie by alttitle - check imdb", zap.String("imdb", dbmoviestemp[idx].Str))
		found, found1 = checkifdbmovieyearmatches(dbmoviestemp[idx].Num, year)
		if found || found1 {
			return dbmoviestemp[idx].Str, found, found1
		}
	}
	if !addifnotfound {
		return "", false, false
	}
	searchprovider := []string{"imdb", "tmdb", "omdb"}
	if strings.EqualFold(searchtype, "rss") {
		if len(config.Cfg.General.MovieRSSMetaSourcePriority) >= 1 {
			searchprovider = config.Cfg.General.MovieRSSMetaSourcePriority
		}
	} else {
		if len(config.Cfg.General.MovieParseMetaSourcePriority) >= 1 {
			searchprovider = config.Cfg.General.MovieParseMetaSourcePriority
		}
	}
	if len(searchprovider) == 0 {
		return "", false, false
	}
	defer logger.ClearVar(&searchprovider)
	var imdb string
	for idxprovider := range searchprovider {
		found = false
		found1 = false
		switch searchprovider[idxprovider] {
		case "imdb":
			if config.Cfg.General.MovieMetaSourceImdb {
				logger.Log.GlobalLogger.Debug("Find movie by title - check imdb", zap.Stringp("title", &title), zap.Intp("year", &year))
				imdb, found, found1 = findimdbbytitle(title, slugged, year)
			}
		case "tmdb":
			if config.Cfg.General.MovieMetaSourceTmdb {
				logger.Log.GlobalLogger.Debug("Find movie by title - check tmdb", zap.Stringp("title", &title), zap.Intp("year", &year))
				imdb, found, found1 = findtmdbbytitle(title, year)
			}
		case "omdb":
			if config.Cfg.General.MovieMetaSourceOmdb {
				logger.Log.GlobalLogger.Debug("Find movie by title - check omdb", zap.Stringp("title", &title), zap.Intp("year", &year))
				imdb, found, found1 = findomdbbytitle(title, year)
			}
		}
		if found || found1 {
			logger.Log.GlobalLogger.Debug("Find movie by title - found", zap.Stringp("title", &title), zap.Intp("year", &year), zap.Stringp("imdb", &imdb))

			return imdb, found, found1
		}
	}

	return "", false, false
}

func FindDbserieByName(title string) (uint, error) {
	var id uint
	err := database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesbyname, Args: []interface{}{title}}, &id)
	if id == 0 {
		slugged := logger.StringToSlug(title)
		err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesbyslug, Args: []interface{}{slugged}}, &id)
		if id == 0 {
			err = database.QueryColumn(&database.Querywithargs{QueryString: querydbserieidseriealternatebytitle, Args: []interface{}{title, slugged}}, &id)
		}
	}
	return id, err
}

func FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid uint, identifier string, season string, episode string) (uint, error) {
	var id uint
	var err error
	if season != "" && episode != "" {
		err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesepisodesbyseason, Args: []interface{}{dbserieid, strings.TrimLeft(season, "0"), strings.TrimLeft(episode, "0")}}, &id)
		if err == nil {
			return id, err
		}
	}
	if identifier != "" {
		err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesepisodesbyidentifier, Args: []interface{}{dbserieid, identifier}}, &id)
		if err == nil {
			return id, err
		}
		if strings.Contains(identifier, ".") {
			err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesepisodesbyidentifier, Args: []interface{}{dbserieid, strings.ReplaceAll(identifier, ".", "-")}}, &id)
			if err == nil {
				return id, err
			}
		}
		if strings.Contains(identifier, " ") {
			err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesepisodesbyidentifier, Args: []interface{}{dbserieid, strings.ReplaceAll(identifier, " ", "-")}}, &id)
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
		str1 = strings.ReplaceAll(str2, " ", "-")
		str1 = strings.ReplaceAll(str1, ".", "-")
	}
	if identifiedby == "date" {
		return &logger.InStringArrayStruct{Arr: []string{str1}}
	}
	if strings.ContainsRune(str1, []rune("E")[0]) {
		return &logger.InStringArrayStruct{Arr: strings.Split(str1, "E")}
	} else if strings.ContainsRune(str1, []rune("e")[0]) {
		return &logger.InStringArrayStruct{Arr: strings.Split(str1, "e")}
	} else if strings.ContainsRune(str1, []rune("X")[0]) {
		return &logger.InStringArrayStruct{Arr: strings.Split(str1, "X")}
	} else if strings.ContainsRune(str1, []rune("x")[0]) {
		return &logger.InStringArrayStruct{Arr: strings.Split(str1, "x")}
	} else if identifiedby != "date" && strings.ContainsRune(str1, []rune("-")[0]) {
		return &logger.InStringArrayStruct{Arr: strings.Split(str1, "-")}
	}
	return &logger.InStringArrayStruct{}
}

func padNumberWithZero(value int) string {
	return fmt.Sprintf("%02d", value)
}
func JobImportDbSeries(serieconfig *config.SerieConfig, cfgp *config.MediaTypeConfig, listname string, checkall bool, addnew bool) {
	defer serieconfig.Close()
	jobName := serieconfig.Name
	if jobName == "" {
		jobName = cfgp.ListsMap[listname].Name
	}
	if jobName == "" {
		logger.Log.GlobalLogger.Debug("Job skipped - no name")
		return
	}

	if importJobRunning == jobName {
		logger.Log.GlobalLogger.Debug("Job already running", zap.String("job", jobName))
		return
	}
	importJobRunning = jobName
	if cfgp.Name == "" {
		logger.Log.GlobalLogger.Debug("Job cfpg missing", zap.String("job", jobName))
		return
	}
	if !addnew && (cfgp.Name == "" || listname == "") {
		database.QueryColumn(&database.Querywithargs{QueryString: "select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", Args: []interface{}{serieconfig.TvdbID}}, &listname)
	}
	if cfgp.Name == "" || listname == "" {
		logger.Log.GlobalLogger.Info("Series not fetched because list or template is empty", zap.String("config", cfgp.Name), zap.String("Listname", listname))
		return
	}

	var dbserie database.Dbserie
	// defer dbserie.Close()
	if serieconfig.TvdbID != 0 {
		if database.GetDbserie(&database.Querywithargs{Query: database.QueryFilterByTvdb, Args: []interface{}{serieconfig.TvdbID}}, &dbserie) != nil && !addnew {
			logger.Log.GlobalLogger.Debug("Job skipped - getdata failed", zap.Stringp("job", &jobName))
			return
		}
	}
	var dbserieadded bool
	if dbserie.Seriename == "" {
		dbserie.Seriename = serieconfig.Name
	}
	if dbserie.Identifiedby == "" {
		dbserie.Identifiedby = serieconfig.Identifiedby
	}

	var counter int
	var err error
	if strings.EqualFold(serieconfig.Source, "none") && addnew {
		database.QueryColumn(&database.Querywithargs{QueryString: querycountdbseriesbyseriename, Args: []interface{}{serieconfig.Name}}, &counter)

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
			queryalternate := "select count() from dbserie_alternates where Dbserie_id = ? and title = ? COLLATE NOCASE"
			for idxalt := range serieconfig.AlternateName {
				if serieconfig.AlternateName[idxalt] == "" {
					continue
				}
				err = database.QueryColumn(&database.Querywithargs{QueryString: queryalternate, Args: []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt]}}, &counter)
				if err != nil {
					continue
				}
				if counter == 0 {
					database.InsertStatic(&database.Querywithargs{QueryString: "Insert into dbserie_alternates (title, slug, dbserie_id) values (?, ?, ?)", Args: []interface{}{serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt]), dbserie.ID}})
				}
			}
		} else {
			err = database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesbyname, Args: []interface{}{serieconfig.Name}}, &dbserie.ID)

			if err != nil {
				logger.Log.GlobalLogger.Debug("Job skipped - id not fetched", zap.Stringp("job", &jobName))
				return
			}
		}
	}
	if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
		if dbserie.ID == 0 {
			dbserie.ThetvdbID = serieconfig.TvdbID
			database.QueryColumn(&database.Querywithargs{QueryString: querycountdbseriesbytvdbid, Args: []interface{}{serieconfig.TvdbID}}, &counter)
			if counter == 0 && addnew {
				if !config.Check("imdb") {
					return
				}
				logger.Log.GlobalLogger.Debug("Insert dbseries for", zap.Int("tvdb", serieconfig.TvdbID))
				inres, err := database.InsertStatic(&database.Querywithargs{QueryString: "insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)", Args: []interface{}{dbserie.Seriename, dbserie.ThetvdbID, dbserie.Identifiedby}})
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
		}
		if dbserie.ID != 0 && (dbserieadded || !addnew) {
			//Update Metadata
			logger.Log.GlobalLogger.Debug("Get metadata for", zap.Int("tvdb", serieconfig.TvdbID))
			addaliases := dbserie.GetMetadata("", config.Cfg.General.SerieMetaSourceTmdb, config.Cfg.General.SerieMetaSourceTrakt, !addnew, true)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(cfgp.MetadataLanguage, config.Cfg.General.SerieMetaSourceTmdb, config.Cfg.General.SerieMetaSourceTrakt, !addnew, true)
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases...)
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
			addaliases = nil

			database.UpdateNamed(queryupdateseries, dbserie)

			var titles []database.DbstaticOneStringOneInt
			database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: "select title, id from dbserie_alternates where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &titles)

			var processed logger.InStringArrayStruct

			arrmetalang := logger.InStringArrayStruct{Arr: cfgp.MetadataTitleLanguages}
			var titlegroup []database.DbserieAlternate
			//var regionok bool
			if config.Cfg.General.SerieAlternateTitleMetaSourceImdb && dbserie.ImdbID != "" {
				queryimdbid := dbserie.ImdbID
				if !strings.HasPrefix(dbserie.ImdbID, "tt") {
					queryimdbid = "tt" + dbserie.ImdbID
				}

				var imdbakadata []database.ImdbAka
				database.QueryImdbAka(&database.Querywithargs{Query: database.QueryFilterByTconst, Args: []interface{}{queryimdbid}}, &imdbakadata)

				titlegroup = make([]database.DbserieAlternate, 0, len(imdbakadata))
				lenarr := len(arrmetalang.Arr)
				for idximdb := range imdbakadata {
					if logger.InStringArray(imdbakadata[idximdb].Region, &arrmetalang) || lenarr == 0 {
						titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: imdbakadata[idximdb].Title, Slug: imdbakadata[idximdb].Slug, Region: imdbakadata[idximdb].Region})
						processed.Arr = append(processed.Arr, imdbakadata[idximdb].Title)
					}
				}
				imdbakadata = nil
			}
			if config.Cfg.General.SerieAlternateTitleMetaSourceTrakt && (dbserie.TraktID != 0 || dbserie.ImdbID != "") {
				queryid := dbserie.ImdbID
				if dbserie.TraktID != 0 {
					queryid = logger.IntToString(dbserie.TraktID)
				}
				traktaliases, err := apiexternal.TraktAPI.GetSerieAliases(queryid)
				if err == nil {
					titlegroup = logger.GrowSliceBy(titlegroup, len(traktaliases.Aliases))
					lenarr := len(arrmetalang.Arr)
					for idxalias := range traktaliases.Aliases {
						if logger.InStringArray(traktaliases.Aliases[idxalias].Country, &arrmetalang) || lenarr == 0 {
							titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: traktaliases.Aliases[idxalias].Title, Slug: logger.StringToSlug(traktaliases.Aliases[idxalias].Title), Region: traktaliases.Aliases[idxalias].Country})
							processed.Arr = append(processed.Arr, traktaliases.Aliases[idxalias].Title)
						}
					}
					traktaliases.Close()
				} else {
					logger.Log.GlobalLogger.Warn("Serie trakt aliases not found for", zap.Int("tvdb", dbserie.ThetvdbID))
				}
			}
			processed.Close()

			arrmetalang.Close()
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
					database.InsertStatic(&database.Querywithargs{QueryString: "Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", Args: []interface{}{titlegroup[idxalt].Title, titlegroup[idxalt].Slug, dbserie.ID, titlegroup[idxalt].Region}})
				}
			}
			titlegroup = nil
			titles = nil

			if (checkall || dbserieadded || !addnew) && (serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb")) {
				logger.Log.GlobalLogger.Debug("Get episodes for", zap.Int("tvdb", serieconfig.TvdbID))

				if dbserie.ThetvdbID != 0 {
					tvdbdetails, err := apiexternal.TvdbAPI.GetSeriesEpisodes(dbserie.ThetvdbID, cfgp.MetadataLanguage)

					if err == nil {
						var tbl []database.DbstaticTwoString
						database.QueryStaticColumnsTwoString(false, 0, &database.Querywithargs{QueryString: "select season, episode from dbserie_episodes where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &tbl)
						var ok bool
						var strseason, strepisode string
						for idx := range tvdbdetails.Data {
							ok = true
							strepisode = logger.IntToString(tvdbdetails.Data[idx].AiredEpisodeNumber)
							strseason = logger.IntToString(tvdbdetails.Data[idx].AiredSeason)
							for idxtbl := range tbl {
								if tbl[idxtbl].Str1 == strseason && tbl[idxtbl].Str2 == strepisode {
									ok = false
									break
								}
							}
							if !ok {
								continue
							}
							database.InsertNamed("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (:episode, :season, :identifier, :title, :first_aired, :overview, :poster, :dbserie_id)", database.DbserieEpisode{
								Episode:    strepisode,
								Season:     strseason,
								Identifier: "S" + padNumberWithZero(tvdbdetails.Data[idx].AiredSeason) + "E" + padNumberWithZero(tvdbdetails.Data[idx].AiredEpisodeNumber),
								Title:      tvdbdetails.Data[idx].EpisodeName,
								Overview:   tvdbdetails.Data[idx].Overview,
								Poster:     tvdbdetails.Data[idx].Poster,
								DbserieID:  dbserie.ID,
								FirstAired: logger.ParseDate(tvdbdetails.Data[idx].FirstAired, "2006-01-02")})

						}
						tbl = nil
						tvdbdetails.Close()
					} else {
						logger.Log.GlobalLogger.Warn("Serie tvdb episodes not found for", zap.Int("tvdb", dbserie.ThetvdbID))
					}
				}
				if config.Cfg.General.SerieMetaSourceTrakt && dbserie.ImdbID != "" {
					seasons, err := apiexternal.TraktAPI.GetSerieSeasons(dbserie.ImdbID)
					if err == nil {
						var episodes apiexternal.TraktSerieSeasonEpisodeGroup
						//var identifier string
						var tbl []database.DbstaticTwoString
						database.QueryStaticColumnsTwoString(false, 0, &database.Querywithargs{QueryString: "select season, episode from dbserie_episodes where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &tbl)
						var ok bool

						var strseason, strepisode string
						for idxseason := range seasons.Seasons {
							episodes.Episodes = []apiexternal.TraktSerieSeasonEpisodes{}
							err = apiexternal.TraktAPI.GetSerieSeasonEpisodes(dbserie.ImdbID, seasons.Seasons[idxseason].Number, &episodes)
							if err == nil {
								for idxepi := range episodes.Episodes {
									ok = true
									strepisode = logger.IntToString(episodes.Episodes[idxepi].Episode)
									strseason = logger.IntToString(episodes.Episodes[idxepi].Season)
									for idxtbl := range tbl {
										if tbl[idxtbl].Str1 == strseason && tbl[idxtbl].Str2 == strepisode {
											ok = false
											break
										}
									}
									if !ok {
										continue
									}
									database.InsertNamed("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (:episode, :season, :identifier, :title, :first_aired, :overview, :poster, :dbserie_id)", database.DbserieEpisode{
										Episode:    strepisode,
										Season:     strseason,
										Identifier: "S" + padNumberWithZero(episodes.Episodes[idxepi].Season) + "E" + padNumberWithZero(episodes.Episodes[idxepi].Episode),
										Title:      episodes.Episodes[idxepi].Title,
										FirstAired: sql.NullTime{Time: episodes.Episodes[idxepi].FirstAired, Valid: true},
										Overview:   episodes.Episodes[idxepi].Overview,
										DbserieID:  dbserie.ID,
										Runtime:    episodes.Episodes[idxepi].Runtime})
									//else {
									// 	if episodes.Episodes[idxepi].Title != "" {
									// 		UpdateColumnStatic("update dbserie_episodes set title = ? where id = ? and title = ''", episodes.Episodes[idxepi].Title, counter)
									// 	}
									// 	if !episodes.Episodes[idxepi].FirstAired.IsZero() {
									// 		UpdateColumnStatic("update dbserie_episodes set first_aired = ? where id = ? and first_aired is null", sql.NullTime{Time: episodes.Episodes[idxepi].FirstAired, Valid: true}, counter)
									// 	}
									// 	if episodes.Episodes[idxepi].Overview != "" {
									// 		UpdateColumnStatic("update dbserie_episodes set overview = ? where id = ? and overview = ''", episodes.Episodes[idxepi].Overview, counter)
									// 	}
									// 	if episodes.Episodes[idxepi].Runtime != 0 {
									// 		UpdateColumnStatic("update dbserie_episodes set runtime = ? where id = ? and Runtime = 0", episodes.Episodes[idxepi].Runtime, counter)
									// 	}
									// }
								}
							} else {
								logger.Log.GlobalLogger.Warn("Serie trakt episodes not found for", zap.Stringp("imdb", &dbserie.ImdbID), zap.Int("season", seasons.Seasons[idxseason].Number))
							}
						}
						tbl = nil
						seasons.Close()
						episodes.Close()
					} else {
						logger.Log.GlobalLogger.Warn("Serie trakt seasons not found for", zap.Stringp("imdb", &dbserie.ImdbID))
					}
				}
			}
		}
	}

	if dbserie.ID == 0 {
		return
	}

	if addnew {
		//Add Entry in SeriesTable

		if listname == "" {
			logger.Log.GlobalLogger.Debug("Series skip for", zap.String("serie", serieconfig.Name))
			return
		}
		var serietest []database.DbstaticOneStringOneInt
		database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: "select lower(listname), id from series where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &serietest)

		for idx2 := range serietest {
			for idx := range cfgp.ListsMap[listname].IgnoreMapLists {
				if strings.EqualFold(serietest[idx2].Str, cfgp.ListsMap[listname].IgnoreMapLists[idx]) {
					logger.Log.GlobalLogger.Debug("Series skip2 for", zap.String("serie", serieconfig.Name))
					serietest = nil
					return
				}
			}
			for idxreplace := range cfgp.ListsMap[listname].ReplaceMapLists {
				if strings.EqualFold(serietest[idx2].Str, cfgp.ListsMap[listname].ReplaceMapLists[idxreplace]) {
					database.UpdateNamed("Update series SET listname = :listname, dbserie_id = :dbserie_id where id = :id", database.Serie{Listname: cfgp.ListsMap[listname].Name, DbserieID: dbserie.ID, ID: uint(serietest[idx2].Num)})
				}
			}
		}
		serietest = nil

		//var serie database.Serie

		if database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "Select count() from series where dbserie_id = ? and listname = ?", Args: []interface{}{dbserie.ID, cfgp.ListsMap[listname].Name}}) == 0 {
			logger.Log.GlobalLogger.Debug("Add series for", zap.Int("tvdb", serieconfig.TvdbID), zap.String("Listname", cfgp.ListsMap[listname].Name))
			_, err := database.InsertStatic(&database.Querywithargs{QueryString: "Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)", Args: []interface{}{dbserie.ID, cfgp.ListsMap[listname].Name, serieconfig.Target, serieconfig.SearchSpecials, serieconfig.DontSearch, serieconfig.DontUpgrade}})
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
				return
			}
		}
	}

	//logger.Log.GlobalLogger.Info("Refresh Episodes of list ", zap.String("job", jobName))
	var series []int
	database.QueryStaticIntArray(0, &database.Querywithargs{QueryString: "select id from series where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &series)

	var dbepisode []int
	database.QueryStaticIntArray(0, &database.Querywithargs{QueryString: "select id from dbserie_episodes where dbserie_id = ?", Args: []interface{}{dbserie.ID}}, &dbepisode)

	episodesint := new(logger.InIntArrayStruct)
	queryepisodes := "select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?"
	for idxserie := range series {
		episodesint.Arr = []int{}
		database.QueryStaticIntArray(
			0,
			&database.Querywithargs{QueryString: queryepisodes, Args: []interface{}{dbserie.ID, series[idxserie]}}, &episodesint.Arr)

		for idxdbepi := range dbepisode {
			if !logger.InIntArray(dbepisode[idxdbepi], episodesint) {
				database.InsertStatic(&database.Querywithargs{QueryString: "Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, ?, ?, ?)", Args: []interface{}{dbserie.ID, series[idxserie], true, cfgp.ListsMap[listname].TemplateQuality, dbepisode[idxdbepi]}})
			}
		}
	}
	episodesint.Close()
	series = nil
	dbepisode = nil
}
