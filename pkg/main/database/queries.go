package database

import (
	"database/sql"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/jmoiron/sqlx"
)

type Query struct {
	Select    string
	Where     string
	OrderBy   string
	Limit     int
	Offset    int
	InnerJoin string
}

type Querywithargs struct {
	QueryString    string
	Select         string
	Table          string
	InnerJoin      string
	Where          string
	OrderBy        string
	Limit          int
	Offset         int
	DontCache      bool
	defaultcolumns string
	size           int
}

type DbstaticOneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

type DbstaticOneInt struct {
	Num int `db:"num"`
}

type DbstaticOneStringOneInt struct {
	Str string `db:"str"`
	Num int    `db:"num"`
}

type DbstaticTwoStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Num  int    `db:"num"`
}

type DbstaticTwoInt struct {
	Num1 int `db:"num1"`
	Num2 int `db:"num2"`
}
type DbstaticTwoUint struct {
	Num1 uint `db:"num1"`
	Num2 uint `db:"num2"`
}

type DbstaticThreeString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
}
type DbstaticThreeStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
	Num1 int    `db:"num1"`
}

type DbstaticTwoString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
}

const (
	FilterByImdb                  = "imdb_id = ?"
	FilterByTvdb                  = "thetvdb_id = ?"
	FilterByTconst                = "tconst = ?"
	strWhere                      = " where "
	QueryDbmoviesGetImdbByMoviedb = "select imdb_id from dbmovies where moviedb_id = ?"
	//QueryDbmoviesGetImdbYearByTitleSlug = "select imdb_id, year from dbmovies where title = ? COLLATE NOCASE OR slug = ?"
	QueryDbmoviesGetImdbByID    = "select imdb_id from dbmovies where id = ?"
	QueryDbmoviesGetIDByImdb    = "select id from dbmovies where imdb_id = ?"
	QueryDbmoviesGetYearByID    = "Select year from dbmovies where id = ?"
	QueryDbmoviesGetTitleByID   = "Select title from dbmovies where id = ?"
	QueryDbmoviesGetRuntimeByID = "Select runtime from dbmovies where id = ?"

	QueryDbmovieTitlesGetTitleByIDLmit1   = "select title from dbmovie_titles where dbmovie_id = ? limit 1"
	QueryDbmovieTitlesGetTitleByID        = "select title from dbmovie_titles where dbmovie_id = ?"
	QueryDbmovieTitlesGetTitleByIDNoEmpty = "select distinct title from dbmovie_titles where dbmovie_id = ? and title != ''"
	//QueryDbmovieTitlesGetImdbYearByTitleSlug = "select dbmovies.imdb_id, dbmovies.year from dbmovie_titles inner join dbmovies on dbmovies.id=dbmovie_titles.dbmovie_id where dbmovie_titles.title = ? COLLATE NOCASE OR dbmovie_titles.slug = ?"

	QueryMoviesGetQualityByImdb = "select movies.quality_profile from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id = ?"
	QueryMoviesGetQualityByID   = "select quality_profile from movies where id = ?"
	QueryMoviesGetRootpathByID  = "select rootpath from movies where id = ?"
	QueryMoviesGetListnameByID  = "select listname from movies where id = ?"
	QueryMoviesGetDBIDByID      = "select dbmovie_id from movies where id = ?"
	//QueryMoviesGetDontSearchByID      = "Select dont_search from movies where id = ?"
	//QueryMoviesGetDontUpgradeByID     = "Select dont_upgrade from movies where id = ?"
	QueryMoviesGetIDByImdbListname    = "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"
	QueryMoviesCountByListname        = "Select count() from movies where listname = ? COLLATE NOCASE"
	QueryMoviesGetIDByDBIDListname    = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
	QueryMoviesGetIDMissingByListname = "Select id, missing from movies where listname = ? COLLATE NOCASE"

	QueryMovieFilesCountByMovieID         = "select count() from movie_files where movie_id = ?"
	QueryMovieFilesGetIDByMovieID         = "select id from movie_files where movie_id = ?"
	QueryMovieFilesGetIDMovieIDByLocation = "select id, movie_id from movie_files where location = ?"

	QueryImdbRatingsCountByImdbVotes      = "select count() from imdb_ratings where tconst = ? and num_votes < ?"
	QueryImdbRatingsCountByImdbRating     = "select count() from imdb_ratings where tconst = ? and average_rating < ?"
	QueryImdbGenresGetGenreByImdb         = "select genre from imdb_genres where tconst = ?"
	QueryImdbGenresCountByImdbGenre       = "select count() from imdb_genres where tconst = ? and genre = ? COLLATE NOCASE"
	QueryImdbTitlesGetImdbYearByTitleSlug = "select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)"
	QueryImdbTitlesCountByTitleSlug       = "select count() from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)"
	QueryImdbTitlesGetYearByImdb          = "select start_year from imdb_titles where tconst = ?"
	QueryImdbAkasGetImdbByTitleSlug       = "select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?"
	QueryImdbAkasCountByTitleSlug         = "select count() from imdb_akas where title = ? COLLATE NOCASE or slug = ?"

	QueryDbseriesGetIDByName       = "select id from dbseries where seriename = ? COLLATE NOCASE"
	QueryDbseriesGetIDByTvdb       = "select id from dbseries where thetvdb_id = ?"
	QueryDbseriesGetIdentifiedByID = "select lower(identifiedby) from dbseries where id = ?"
	QueryDbseriesGetTvdbByID       = "select thetvdb_id from dbseries where id = ?"
	QueryDbseriesGetSerienameByID  = "Select seriename from dbseries where id = ?"
	QueryDbseriesGetRuntimeByID    = "select runtime from dbseries where id = ?"
	QueryDbseriesGetIDBySlug       = "select id from dbseries where slug = ?"

	QueryDbserieAlternatesGetDBIDByTitleSlug    = "select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE or Slug = ?"
	QueryDbserieAlternatesGetTitleByDBID        = "select title from dbserie_alternates where dbserie_id = ?"
	QueryDbserieAlternatesGetTitleByDBIDNoEmpty = "select distinct title from dbserie_alternates where dbserie_id = ? and title != ''"

	QueryDbserieEpisodesGetIDByDBIDSeasonEpisode = "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?"
	QueryDbserieEpisodesGetIDByDBIDIdentifier    = "select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE"
	QueryDbserieEpisodesGetIDByDBID              = "select id from dbserie_episodes where dbserie_id = ?"
	QueryDbserieEpisodesGetSeasonByID            = "Select season from dbserie_episodes where id = ?"
	QueryDbserieEpisodesGetEpisodeByID           = "Select episode from dbserie_episodes where id = ?"
	QueryDbserieEpisodesGetTitleByID             = "select title from dbserie_episodes where id = ?"
	QueryDbserieEpisodesGetSeasonEpisodeByDBID   = "select season, episode from dbserie_episodes where dbserie_id = ?"

	QuerySeriesGetRootpathByID     = "select rootpath from series where id = ?"
	QuerySeriesGetDBIDByID         = "select dbserie_id from series where id = ?"
	QuerySeriesGetIDByDBIDListname = "select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE"
	QuerySeriesGetListnameByID     = "select listname from series where id = ?"
	QuerySeriesGetIDByDBID         = "select id from series where dbserie_id = ?"

	QuerySerieEpisodesGetIDBySerieDBEpisode       = "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?"
	QuerySerieEpisodesGetIDBySerie                = "select id from serie_episodes where serie_id = ?"
	QuerySerieEpisodesGetQualityBySerieID         = "select quality_profile from serie_episodes where serie_id = ?"
	QuerySerieEpisodesGetSerieIDByID              = "select serie_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetDBSerieIDByID            = "select dbserie_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetDBSerieEpisodeIDByID     = "Select dbserie_episode_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetQualityByID              = "Select quality_profile from serie_episodes where id = ?"
	QuerySerieEpisodesGetDBEpisodeIDByDBIDSerieID = "select dbserie_episode_id from serie_episodes where dbserie_id = ? and serie_id = ?"
	QuerySerieEpisodesGetDBEpisodeIDSerieIDByDBID = "select dbserie_episode_id, serie_id from serie_episodes where dbserie_id = ?"

	QuerySerieEpisodeFilesCountByEpisodeID         = "select count() from serie_episode_files where serie_episode_id = ?"
	QuerySerieEpisodeFilesCountByListname          = "select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)"
	QuerySerieEpisodeFilesGetIDByEpisodeID         = "select id from serie_episode_files where serie_episode_id = ?"
	QuerySerieEpisodeFilesGetIDMissingByListname   = "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)"
	QuerySerieEpisodeFilesGetIDEpisodeIDByLocation = "select id, serie_episode_id from serie_episode_files where location = ?"

	QuerySearchSeriesUpgrade = "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname"
	QueryUpdateHistory       = "Update job_histories set ended = ? where id = ?"
	QueryUpdateSerieLastscan = "Update serie_episodes set lastscan = ? where id = ?"
	Queryupdateseries        = "Update dbseries SET Seriename = :seriename, Aliases = :aliases, Season = :season, Status = :status, Firstaired = :firstaired, Network = :network, Runtime = :runtime, Language = :language, Genre = :genre, Overview = :overview, Rating = :rating, Siterating = :siterating, Siterating_Count = :siterating_count, Slug = :slug, Trakt_ID = :trakt_id, Imdb_ID = :imdb_id, Thetvdb_ID = :thetvdb_id, Freebase_M_ID = :freebase_m_id, Freebase_ID = :freebase_id, Tvrage_ID = :tvrage_id, Facebook = :facebook, Instagram = :instagram, Twitter = :twitter, Banner = :banner, Poster = :poster, Fanart = :fanart, Identifiedby = :identifiedby where id = :id"
	QueryupdateseriesStatic  = "Update dbseries SET Seriename = ?, Aliases = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?"
	Queryupdatemovie         = "Update dbmovies SET Title = :title , Release_Date = :release_date , Year = :year , Adult = :adult , Budget = :budget , Genres = :genres , Original_Language = :original_language , Original_Title = :original_title , Overview = :overview , Popularity = :popularity , Revenue = :revenue , Runtime = :runtime , Spoken_Languages = :spoken_languages , Status = :status , Tagline = :tagline , Vote_Average = :vote_average , Vote_Count = :vote_count , Trakt_ID = :trakt_id , Moviedb_ID = :moviedb_id , Imdb_ID = :imdb_id , Freebase_M_ID = :freebase_m_id , Freebase_ID = :freebase_id , Facebook_ID = :facebook_id , Instagram_ID = :instagram_id , Twitter_ID = :twitter_id , URL = :url , Backdrop = :backdrop , Poster = :poster , Slug = :slug where id = :id"
	QueryupdatemovieStatic   = "Update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?"
	Queryidunmatched         = "select id from movie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE"
)

var (
	QueryDBMovieColumns                                = "select id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where "
	QuerySerieeDBIDListnameRootByID                    = "select dbserie_id, listname, rootpath from series where id = ?"
	QueryMovieDBIDListnameRootByID                     = "select dbmovie_id, listname, rootpath from movies where id = ?"
	QuerySerieEpisodesGetIDsDontQualityByID            = "select dbserie_episode_id, dbserie_id, serie_id, dont_search, dont_upgrade, quality_profile from serie_episodes where id = ?"
	QuerySerieEpisodesGetDontQualityByID               = "select dont_search, dont_upgrade, quality_profile from serie_episodes where id = ?"
	QueryMoviesGetIDsDontQualityByID                   = "select dbmovie_id, dont_search, dont_upgrade, quality_profile from movies where id = ?"
	QueryMoviesGetDontQualityByID                      = "select dont_search, dont_upgrade, quality_profile from movies where id = ?"
	QueryDbmoviesGetYearImdbTitleByID                  = "select year, imdb_id, title from dbmovies where id = ?"
	QueryDbseriesGetTvdbSerienameByID                  = "select thetvdb_id, seriename from dbseries where id = ?"
	QueryDbserieEpisodesGetSeasonEpisodeIdentifierByID = "select season, episode, identifier from dbserie_episodes where id = ?"
)

func RemoveFromTwoUIntArrayStructV1V2(in *[]DbstaticTwoUint, v1 uint, v2 uint) {
	intid := -1
	for idxi := range *in {
		if (*in)[idxi].Num1 == v1 && (*in)[idxi].Num2 == v2 {
			intid = idxi
			break
		}
	}
	//intid := logger.IndexFunc(in, func(c DbstaticTwoUint) bool { return c.Num1 == v1 && c.Num2 == v2 })
	if intid != -1 {
		//logger.Delete(in, intid, intid+1)
		logger.Delete(in, intid)
	}
}

func getdb(imdb bool) *sqlx.DB {
	if imdb {
		return dbImdb
	}
	return dbData
}
func getstatement(cached bool, imdb bool, querystring *string) *sqlx.Stmt {
	if cached && logger.GlobalCacheStmt.CheckNoType(querystring) {
		stmt := logger.GlobalCacheStmt.GetData(querystring)
		if stmt != nil {
			return stmt
		}
	}
	sq, err := getdb(imdb).Preparex(*querystring)
	if err != nil {
		logerror(err, querystring, "preparing sql")
		return nil
	}
	if cached {
		logger.GlobalCacheStmt.Set(querystring, sq, time.Minute*20, false)
	}
	return sq
}

func getnamedstatement(cached bool, imdb bool, querystring *string) *sqlx.NamedStmt {
	if cached && logger.GlobalCacheNamed.CheckNoType(querystring) {
		stmt := logger.GlobalCacheNamed.GetData(querystring)
		if stmt != nil {
			return stmt
		}
	}
	sq, err := getdb(imdb).PrepareNamed(*querystring)
	if err != nil {
		logerror(err, querystring, "preparing named sql")
		return nil
	}
	if cached {
		logger.GlobalCacheNamed.Set(querystring, sq, time.Minute*20, false)
	}
	return sq
}

//	func checkargs(args *[]interface{}) []interface{} {
//		if args == nil {
//			return []interface{}{}
//		}
//		return *args
//	}
func execsql(cached bool, imdb bool, querystring *string, args *[]interface{}) (sql.Result, error) {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	defer logger.Clear(args)
	if DBLogLevel == logger.StrDebug {
		logger.Log.Debug().Str(logger.StrQuery, *querystring).Any("args", args).Msg("query exec")
	}

	return getstatement(cached, imdb, querystring).Exec(*args...)
}

func execnamedsql(imdb bool, querystring *string, args *[]interface{}) (sql.Result, error) {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	if DBLogLevel == logger.StrDebug {
		logger.Log.Debug().Str(logger.StrQuery, *querystring).Any("args", args).Msg("query exec")
	}

	defer logger.Clear(args)
	if args == nil || len(*args) == 0 {
		return getnamedstatement(true, imdb, querystring).Exec(nil)
	}
	return getnamedstatement(true, imdb, querystring).Exec((*args)[0])
}

// scans single row into any Structure
func queryComplexObject[T any](imdb bool, querystring *string, args *[]interface{}) (*T, error) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var u T
	return &u, logerror(getstatement(true, imdb, querystring).QueryRowx(*args...).StructScan(&u), querystring, "select complex")
}

func logerror(err error, querystring *string, msg string) error {
	if err != nil && err != sql.ErrNoRows {
		logger.Log.Error().Err(err).Str(logger.StrQuery, *querystring).Msg(msg)
	}
	return err
}

// type queryobj struct {
// 	imdb       bool
// 	scaninto   []interface{}
// 	query      string
// 	args       []interface{}
// 	cached     bool
// 	simplescan bool
// 	size       int
// }

// scans any single row into variables
func queryGenericsSingle(imdb bool, fulllock bool, scanInto *[]interface{}, querystring *string, args *[]interface{}) error {
	if fulllock {
		readWriteMu.Lock()
		defer readWriteMu.Unlock()
	} else {
		readWriteMu.RLock()
		defer readWriteMu.RUnlock()
	}
	return logerror(getstatement(true, imdb, querystring).QueryRow(*args...).Scan(*scanInto...), querystring, "select single")
}

// scans any single row into a single variable
func queryGenericsSingleObjT[T any](imdb bool, querystring *string, args *[]interface{}) *T {
	var obj T
	//queryGenericsSingleOne(imdb, &obj, querystring, args)
	queryGenericsSingle(imdb, false, &[]interface{}{&obj}, querystring, args)
	return &obj
}

// scans multiple rows into any Structure - must provide scanFunc Mapping for Columns
func queryGenericsT[T any](cached bool, imdb bool, simplescan bool, size int, scanFunc func(elem *T) []interface{}, querystring *string, args *[]interface{}) *[]T {
	var result []T
	if size >= 1 {
		result = make([]T, 0, size)
	}
	queryGenericsTAssigned(cached, imdb, simplescan, &result, scanFunc, querystring, args)
	return &result
}

// scans multiple rows into any Structure - must provide scanFunc Mapping for Columns
func queryGenericsTAssigned[T any](cached bool, imdb bool, simplescan bool, result *[]T, scanFunc func(elem *T) []interface{}, querystring *string, args *[]interface{}) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getstatement(cached, imdb, querystring).Queryx(*args...)
	logger.Clear(args)

	if err != nil {
		logerror(err, querystring, "select")
		return
	}
	defer rows.Close()
	var u T
	var arr []interface{}
	if scanFunc != nil {
		arr = scanFunc(&u)
	}
	for rows.Next() {
		if scanFunc == nil {
			if simplescan {
				err = rows.Scan(&u)
			} else {
				err = rows.StructScan(&u)
			}
		} else {
			err = rows.Scan(arr...)
		}

		if err != nil {
			logerror(err, querystring, "select array")
			return
		}
		*result = append(*result, u)
	}
	logger.Clear(&arr)
}

// need to return all prio ids and strings
func Queryfilesprio(querystring string, resid *uint, qualid *uint, codecid *uint, audid *uint, proper *bool, extended *bool, repack *bool, args ...interface{}) {
	queryGenericsSingle(false, false, &[]interface{}{resid, qualid, codecid, audid, proper, extended, repack}, &querystring, &args)
}

func QueryEpisodeData(id uint, dbepisodeid *uint, dbserieid *uint, serieid *uint, dontsearch *bool, dontupgrade *bool, quality *string) {
	queryGenericsSingle(false, false, &[]interface{}{dbepisodeid, dbserieid, serieid, dontsearch, dontupgrade, quality}, &QuerySerieEpisodesGetIDsDontQualityByID, &[]interface{}{id})
}

func QueryEpisodeDataDont(id uint, dontsearch *bool, dontupgrade *bool, quality *string) {
	queryGenericsSingle(false, false, &[]interface{}{dontsearch, dontupgrade, quality}, &QuerySerieEpisodesGetDontQualityByID, &[]interface{}{id})
}
func QueryMovieData(id uint, dbmovie *uint, dontsearch *bool, dontupgrade *bool, quality *string) {
	queryGenericsSingle(false, false, &[]interface{}{dbmovie, dontsearch, dontupgrade, quality}, &QueryMoviesGetIDsDontQualityByID, &[]interface{}{id})
}
func QueryMovieDataSearch(id uint, dbmovie *uint, listname *string, rootpath *string) {
	queryGenericsSingle(false, false, &[]interface{}{dbmovie, listname, rootpath}, &QueryMovieDBIDListnameRootByID, &[]interface{}{id})
}
func QuerySerieDataSearch(id uint, dbserie *uint, listname *string, rootpath *string) {
	queryGenericsSingle(false, false, &[]interface{}{dbserie, listname, rootpath}, &QuerySerieeDBIDListnameRootByID, &[]interface{}{id})
}
func QueryMovieDataDont(id uint, dontsearch *bool, dontupgrade *bool, quality *string) {
	queryGenericsSingle(false, false, &[]interface{}{dontsearch, dontupgrade, quality}, &QueryMoviesGetDontQualityByID, &[]interface{}{id})
}
func QueryDbmovieData(id uint, year *int, imdb *string, title *string) {
	if id == 0 {
		return
	}
	queryGenericsSingle(false, false, &[]interface{}{year, imdb, title}, &QueryDbmoviesGetYearImdbTitleByID, &[]interface{}{id})
}

func QueryDbserieData(id uint, tvdbid *int, seriename *string) {
	queryGenericsSingle(false, false, &[]interface{}{tvdbid, seriename}, &QueryDbseriesGetTvdbSerienameByID, &[]interface{}{id})
}

func QueryDbserieEpisodeData(id uint, season *string, episode *string, identifier *string) {
	queryGenericsSingle(false, false, &[]interface{}{season, episode, identifier}, &QueryDbserieEpisodesGetSeasonEpisodeIdentifierByID, &[]interface{}{id})
}

func GetDbmovie(where string, val interface{}) (*Dbmovie, error) {
	query := QueryDBMovieColumns + where
	return queryComplexObject[Dbmovie](false, &query, &[]interface{}{val})
}

func QueryDbmovie(qu Querywithargs, args ...interface{}) *[]Dbmovie {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovies"
	qu.defaultcolumns = "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[Dbmovie](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[Dbmovie](false, qu.size, qu.QueryString, &args)
}

func QueryDbmovieTitle(qu Querywithargs, args ...interface{}) *[]DbmovieTitle {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovie_titles"
	qu.defaultcolumns = "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[DbmovieTitle](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[DbmovieTitle](false, qu.size, qu.QueryString, &args)
}

func GetDbserieByID(id uint) (*Dbserie, error) {
	query := "select id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?"
	return queryComplexObject[Dbserie](false, &query, &[]interface{}{id})
}

func QueryDbserie(qu Querywithargs, args ...interface{}) *[]Dbserie {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbseries"
	qu.defaultcolumns = "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[Dbserie](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[Dbserie](false, qu.size, qu.QueryString, &args)
}

func GetDbserieEpisodesByID(id uint) (*DbserieEpisode, error) {
	query := "select id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id from dbserie_episodes where id = ?"
	return queryComplexObject[DbserieEpisode](false, &query, &[]interface{}{id})
}

func QueryDbserieEpisodes(qu Querywithargs, args ...interface{}) *[]DbserieEpisode {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[DbserieEpisode](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[DbserieEpisode](false, qu.size, qu.QueryString, &args)
}
func QueryDbserieAlternates(qu Querywithargs, args ...interface{}) *[]DbserieAlternate {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_alternates"
	qu.defaultcolumns = "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[DbserieAlternate](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[DbserieAlternate](false, qu.size, qu.QueryString, &args)
}

func GetSeries(qu Querywithargs, args ...interface{}) (*Serie, error) {
	//var result Serie
	qu.Table = logger.StrSeries
	qu.defaultcolumns = "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryComplexObject[Serie](false, &qu.QueryString, &args)
}

func GetSerieEpisodes(qu Querywithargs, args ...interface{}) (*SerieEpisode, error) {
	//var result
	qu.Table = "serie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryComplexObject[SerieEpisode](false, &qu.QueryString, &args)
}

func QuerySerieEpisodes(querystring string, args ...interface{}) *[]SerieEpisode {

	// qu.Table = "serie_episodes"
	// qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	// if qu.QueryString == "" {
	// 	qu.Buildquery(false)
	// }
	return queryGenericsT[SerieEpisode](true, false, false, 0, nil, &querystring, &args)
	//return queryComplexScan[SerieEpisode](false, 0, querystring, &args)
}

func GetSerieEpisodeFiles(querystring string, args ...interface{}) (*SerieEpisodeFile, error) {
	//var result SerieEpisodeFile
	// qu.Table = "serie_episode_files"
	// qu.defaultcolumns = "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	return queryComplexObject[SerieEpisodeFile](false, &querystring, &args)
}

func GetMovies(qu Querywithargs, args ...interface{}) (*Movie, error) {
	//var result Movie
	qu.Table = "movies"
	qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryComplexObject[Movie](false, &qu.QueryString, &args)
}

func QueryMovies(querystring string, args ...interface{}) *[]Movie {

	// qu.Table = "movies"
	// qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	//var result MovieGroup
	// if qu.QueryString == "" {
	// 	qu.Buildquery(false)
	// }
	return queryGenericsT[Movie](true, false, false, 0, nil, &querystring, &args)
	//return queryComplexScan[Movie](false, 0, querystring, &args)
}

func GetMovieFiles(querystring string, args ...interface{}) (*MovieFile, error) {
	//var result MovieFile
	// qu.Table = "movie_files"
	// qu.defaultcolumns = "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	return queryComplexObject[MovieFile](false, &querystring, &args)
}

func QueryQualities(querystring string, args ...interface{}) *[]Qualities {
	// qu.Table = "qualities"
	// qu.defaultcolumns = "id,created_at,updated_at,type,name,regex,strings,priority,use_regex,Regexgroup"
	return queryGenericsT[Qualities](true, false, false, 0, nil, &querystring, &args)
	//return queryComplexScan[Qualities](false, 0, querystring, &args)
}
func QueryJobHistory(qu Querywithargs, args ...interface{}) *[]JobHistory {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "job_histories"
	qu.defaultcolumns = "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[JobHistory](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[JobHistory](false, qu.size, qu.QueryString, &args)
}

func QuerySerieFileUnmatched(qu Querywithargs, args ...interface{}) *[]SerieFileUnmatched {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[SerieFileUnmatched](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[SerieFileUnmatched](false, qu.size, qu.QueryString, &args)
}

func QueryMovieFileUnmatched(qu Querywithargs, args ...interface{}) *[]MovieFileUnmatched {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "movie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[MovieFileUnmatched](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[MovieFileUnmatched](false, qu.size, qu.QueryString, &args)
}
func QueryResultMovies(qu Querywithargs, args ...interface{}) *[]ResultMovies {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "movies"
	qu.defaultcolumns = `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[ResultMovies](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[ResultMovies](false, qu.size, qu.QueryString, &args)
}
func QueryResultSeries(qu Querywithargs, args ...interface{}) *[]ResultSeries {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = logger.StrSeries
	qu.defaultcolumns = `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[ResultSeries](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[ResultSeries](false, qu.size, qu.QueryString, &args)
}

func QueryResultSerieEpisodes(qu Querywithargs, args ...interface{}) *[]ResultSerieEpisodes {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_episodes"
	qu.defaultcolumns = `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[ResultSerieEpisodes](true, false, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[ResultSerieEpisodes](false, qu.size, qu.QueryString, &args)
}

func GetImdbRating(querystring string, args ...interface{}) (*ImdbRatings, error) {
	// qu.Table = "imdb_ratings"
	// qu.defaultcolumns = "id,created_at,updated_at,tconst,num_votes,average_rating"
	// qu.imdb = true
	return queryComplexObject[ImdbRatings](true, &querystring, &args)
}

func QueryImdbAka(qu Querywithargs, args ...interface{}) *[]ImdbAka {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "imdb_akas"
	qu.defaultcolumns = "id,created_at,updated_at,tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return queryGenericsT[ImdbAka](true, true, false, qu.size, nil, &qu.QueryString, &args)
	//return queryComplexScan[ImdbAka](true, qu.size, qu.QueryString, &args)
}

func GetImdbTitle(querystring string, args ...interface{}) (*ImdbTitle, error) {
	// qu.Table = "imdb_titles"
	// qu.defaultcolumns = "tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres"
	// qu.imdb = true
	return queryComplexObject[ImdbTitle](true, &querystring, &args)
}

func (qu *Querywithargs) Buildquery(count bool) {
	var bld strings.Builder
	if count {
		bld.Grow(len(qu.Table) + len(qu.Where) + len(qu.InnerJoin) + len(qu.OrderBy) + 50)
	} else {
		if len(qu.Select) >= 1 {
			bld.Grow(len(qu.Table) + len(qu.Select) + len(qu.Where) + len(qu.InnerJoin) + len(qu.OrderBy) + 50)
		} else {
			bld.Grow(len(qu.Table) + len(qu.defaultcolumns) + len(qu.Where) + len(qu.InnerJoin) + len(qu.OrderBy) + 50)
		}
	}
	bld.WriteString("select ")
	if len(qu.Select) >= 1 {
		bld.WriteString(qu.Select)
	} else {
		if logger.ContainsI(qu.defaultcolumns, qu.Table+".") {
			bld.WriteString(qu.defaultcolumns)
		} else {
			if count {
				bld.WriteString("count()")
			} else {
				if qu.InnerJoin != "" {
					bld.WriteString(qu.Table)
					bld.WriteString(".*")
				} else {
					bld.WriteString(qu.defaultcolumns)
				}
			}
		}
	}
	bld.WriteString(" from ")
	bld.WriteString(qu.Table)
	if qu.InnerJoin != "" {
		bld.WriteString(" inner join ")
		bld.WriteString(qu.InnerJoin)
	}
	if qu.Where != "" {
		bld.WriteString(" where ")
		bld.WriteString(qu.Where)
	}
	if qu.OrderBy != "" {
		bld.WriteString(" order by ")
		bld.WriteString(qu.OrderBy)
	}
	if qu.Limit != 0 {
		if qu.Offset != 0 {
			bld.WriteString(" limit ")
			bld.WriteString(logger.IntToString(qu.Offset) + "," + logger.IntToString(qu.Limit))
		} else {
			bld.WriteString(" limit ")
			bld.WriteString(logger.IntToString(qu.Limit))
		}
	}
	qu.QueryString = bld.String()
	bld.Reset()
}

// requires 1 column - int
func QueryStaticColumnsOneIntOneBool(size int, querystring string, args ...interface{}) *[]DbstaticOneIntOneBool {
	return queryGenericsT(true, false, false, size, func(elem *DbstaticOneIntOneBool) []interface{} {
		return []interface{}{&elem.Num, &elem.Bl}
	}, &querystring, &args)
}

// requires 1 column - string
func QueryStaticStringArray(imdb bool, size int, querystring string, args ...interface{}) *[]string {
	return queryGenericsT[string](true, imdb, true, size, nil, &querystring, &args)
}

func QueryStaticStringArrayObj(obj *[]string, imdb bool, querystring string, args ...interface{}) {
	queryGenericsTAssigned(true, imdb, true, obj, nil, &querystring, &args)
}

// requires 1 column - string
func QueryStaticIntArray(size int, querystring string, args ...interface{}) *[]int {
	return queryGenericsT[int](true, false, true, size, nil, &querystring, &args)
}

func QueryStaticUintArrayNoError(cached bool, size int, querystring string, args ...interface{}) *[]uint {
	return queryGenericsT[uint](cached, false, true, size, nil, &querystring, &args)
}

// requires 2 columns- string and int
func QueryStaticColumnsOneStringOneInt(imdb bool, size int, querystring string, args ...interface{}) *[]DbstaticOneStringOneInt {
	return queryGenericsT(true, imdb, false, size, func(elem *DbstaticOneStringOneInt) []interface{} {
		return []interface{}{&elem.Str, &elem.Num}
	}, &querystring, &args)
}

// requires 2 columns- string and int
func QueryStaticColumnsTwoStringOneInt(imdb bool, size int, querystring string, args ...interface{}) *[]DbstaticTwoStringOneInt {
	return queryGenericsT(true, imdb, false, size, func(elem *DbstaticTwoStringOneInt) []interface{} {
		return []interface{}{&elem.Str1, &elem.Str2, &elem.Num}
	}, &querystring, &args)
}

// requires 2 columns- int and int
func QueryStaticColumnsTwoInt(cached bool, size int, querystring string, args ...interface{}) *[]DbstaticTwoInt {
	return queryGenericsT(cached, false, false, size, func(elem *DbstaticTwoInt) []interface{} {
		return []interface{}{&elem.Num1, &elem.Num2}
	}, &querystring, &args)
}

// requires 3 columns- string - imdb-db
func QueryStaticColumnsThreeString(querystring string, args ...interface{}) *[]DbstaticThreeString {
	return queryGenericsT(true, true, false, 0, func(elem *DbstaticThreeString) []interface{} {
		return []interface{}{&elem.Str1, &elem.Str2, &elem.Str3}
	}, &querystring, &args)
}

// requires 3 columns- string - imdb-db
func QueryStaticColumnsThreeStringOneInt(imdb bool, size int, querystring string, args ...interface{}) *[]DbstaticThreeStringOneInt {
	return queryGenericsT(true, imdb, false, size, func(elem *DbstaticThreeStringOneInt) []interface{} {
		return []interface{}{&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1}
	}, &querystring, &args)
}

// requires 2 columns- int and int
func QueryStaticColumnsTwoString(imdb bool, size int, querystring string, args ...interface{}) *[]DbstaticTwoString {
	return queryGenericsT(true, imdb, false, size, func(elem *DbstaticTwoString) []interface{} {
		return []interface{}{&elem.Str1, &elem.Str2}
	}, &querystring, &args)
}

// Uses column id
func CountRows(table string, qu Querywithargs, args ...interface{}) int {
	qu.Offset = 0
	qu.Limit = 0
	qu.Select = "count()"
	qu.Table = table
	qu.Buildquery(true)
	return queryIntColumn(false, &qu.QueryString, &args)
}

func QueryColumn(querystring string, obj interface{}, args ...interface{}) error {
	//return queryGenericsSingleOne(false, obj, querystring, args)
	return queryGenericsSingle(false, false, &[]interface{}{obj}, &querystring, &args)
}

func QueryBoolColumn(querystring string, args ...interface{}) bool {
	return *queryGenericsSingleObjT[bool](false, &querystring, &args)
}
func QueryStringColumn(querystring string, args ...interface{}) string {
	return *queryGenericsSingleObjT[string](false, &querystring, &args)
}

func QueryUintColumn(querystring string, args ...interface{}) uint {
	return *queryGenericsSingleObjT[uint](false, &querystring, &args)
}

func queryIntColumn(imdb bool, querystring *string, args *[]interface{}) int {
	return *queryGenericsSingleObjT[int](imdb, querystring, args)
}
func QueryIntColumn(querystring string, args ...interface{}) int {
	return *queryGenericsSingleObjT[int](false, &querystring, &args)
}

func QueryCountColumn(table string, argsstr string, args ...interface{}) int {
	str := "select count() from " + table
	if len(argsstr) >= 1 {
		str += " where " + argsstr
	}
	return queryIntColumn(false, &str, &args)
	// var bld strings.Builder
	// bld.Grow(80)
	// bld.WriteString("select count() from ")
	// bld.WriteString(table)
	// //str := "select count() from " + table
	// if len(argsstr) >= 1 {
	// 	bld.WriteString(" where ")
	// 	bld.WriteString(argsstr)
	// 	//str += " where " + argsstr
	// }
	// str := bld.String()
	// //bld.Reset()
	// return queryIntColumn(false, &str, &args)
}

func QueryImdbIntColumn(querystring string, args ...interface{}) int {
	return queryIntColumn(true, &querystring, &args)
}

func InsertStatic(querystring string, args ...interface{}) (sql.Result, error) {
	result, err := execsql(true, false, &querystring, &args)
	if err != nil {
		logger.Log.Error().Str(logger.StrQuery, querystring).Any("Values", args).Err(err).Msg("Insert")
	}
	return result, err
}
func InsertNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := execnamedsql(false, &query, &[]interface{}{obj})
	if err != nil {
		logger.Log.Error().Str(logger.StrQuery, query).Any("values", &obj).Err(err).Msg("Insert")
	}

	return result, err
}
func InsertArray(table string, columns []string, values ...interface{}) (sql.Result, error) {
	query := "insert into " + table + " (" + strings.Join(columns, ",") + ") values (" + logger.StringsRepeat("?", ",?", len(columns)-1) + ")"
	result, err := execsql(false, false, &query, &values)
	if err != nil {
		logger.Log.Error().Str("Table", table).Any("Colums", columns).Any("Values", values).Err(err).Msg("Insert")
	}
	return result, err
}

func Dbexec(querystring string, args []interface{}) (sql.Result, error) {
	if DBLogLevel == logger.StrDebug {
		logger.Log.Debug().Str(logger.StrQuery, querystring).Any("args", args).Msg("query exec")
	}
	return execsql(true, false, &querystring, &args)
}

func UpdateArray(table string, columns []string, where string, args ...interface{}) (sql.Result, error) {
	var cols, query string
	for idx := range columns {
		if idx != 0 {
			cols += ","
		}
		cols += columns[idx] + " = ?"
	}
	if where != "" {
		query = "update " + table + " set " + cols + strWhere + where
	} else {
		query = "update " + table + " set " + cols
	}
	result, err := execsql(false, false, &query, &args)
	if err != nil {
		logger.Log.Error().Str("Table", table).Any("Columns", &columns).Any("Values", args).Str("where", where).Err(err).Msg("Update")
	}
	return result, err
}
func UpdateNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := execnamedsql(false, &query, &[]interface{}{obj})
	if err != nil {
		logger.Log.Error().Str(logger.StrQuery, query).Any("Values", &obj).Err(err).Msg("Update")
	}

	return result, err
}

func UpdateColumn(table string, column string, value interface{}, where string, args ...interface{}) (sql.Result, error) {
	setvalues := append([]interface{}{value}, args...)

	querystring := "update " + table + " set " + column + " = ?"
	if where != "" {
		querystring += strWhere + where
	}
	result, err := execsql(true, false, &querystring, &setvalues)
	logger.Clear(&setvalues)
	if err != nil {
		logger.Log.Error().Str("Table", table).Str("Column", column).Any("Value", value).Str("where", where).Any("Values", args).Err(err).Msg("Update")
	}
	return result, err
}
func UpdateColumnStatic(querystring string, args ...interface{}) error {
	_, err := execsql(true, false, &querystring, &args)
	if err != nil {
		logger.Log.Error().Str(logger.StrQuery, querystring).Any("Values", args).Err(err).Msg("Update")
	}
	return err
}

func DeleteRow(imdb bool, table string, where string, args ...interface{}) (sql.Result, error) {
	var querystring string
	if where != "" {
		querystring = "delete from " + table + strWhere + where
	} else {
		querystring = "delete from " + table
	}
	if DBLogLevel == logger.StrDebug {
		logger.Log.Debug().Str(logger.StrQuery, querystring).Any("args", args).Msg("query count")
	}

	result, err := execsql(true, imdb, &querystring, &args)

	if err != nil {
		logger.Log.Error().Str("Table", table).Str("Where", where).Any("Values", args).Err(err).Msg("Delete")
	}
	return result, err
}
func DeleteRowStatic(imdb bool, querystring string, args ...interface{}) error {
	_, err := execsql(true, imdb, &querystring, &args)
	if err != nil {
		logger.Log.Error().Str(logger.StrQuery, querystring).Any("Values", args).Err(err).Msg("Insert")
	}
	return err
}

func DBQuickCheck() string {
	logger.Log.Info().Msg("Check Database for Errors")
	var str string
	query := "PRAGMA quick_check;"
	queryGenericsSingle(false, true, &[]interface{}{&str}, &query, &[]interface{}{})
	return str
}

func DBIntegrityCheck() string {
	var str string
	query := "PRAGMA integrity_check;"
	queryGenericsSingle(false, true, &[]interface{}{&str}, &query, &[]interface{}{})
	return str
}

func InsertRetID(dbresult sql.Result) int64 {
	newid, err := dbresult.LastInsertId()
	if err != nil {
		logger.Logerror(err, "query insert")
		return 0
	}
	return newid
}
