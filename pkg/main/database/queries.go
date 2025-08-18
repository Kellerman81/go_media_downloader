package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/jmoiron/sqlx"
)

// Querywithargs is a struct to hold query arguments.
type Querywithargs struct {
	// QueryString is the base SQL query
	QueryString string
	// Select is the columns to select
	Select string
	// Table is the main table in the query
	Table string
	// InnerJoin is any inner join statements
	InnerJoin string
	// Where is the WHERE clause
	Where string
	// OrderBy is the ORDER BY clause
	OrderBy string
	// Limit is the LIMIT clause value
	Limit uint
	// Offset is the OFFSET clause value
	Offset int
	// defaultcolumns is used for default columns
	defaultcolumns string
	// size is the size of the result
	size uint
}

type FilePrio struct {
	Location     string
	DBID         uint
	ID           uint
	ResolutionID uint
	QualityID    uint
	CodecID      uint
	AudioID      uint
	Proper       bool
	Repack       bool
	Extended     bool
}

type DbstaticOneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

type DbstaticOneStringOneInt struct {
	Str string `db:"str"`
	Num int    `db:"num"`
}
type DbstaticOneStringOneUInt struct {
	Str string `db:"str"`
	Num uint   `db:"num"`
}

// type DbstaticOneStringTwoInt struct {
// 	Str  string `db:"str"`
// 	Num1 uint   `db:"num1"`
// 	Num2 uint   `db:"num2"`
// }

//	type DbstaticTwoStringOneInt struct {
//		Str1 string `db:"str1"`
//		Str2 string `db:"str2"`
//		Num  uint   `db:"num"`
//	}
type DbstaticTwoStringOneRInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Num  int    `db:"num"`
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

// type DbstaticThreeStringTwoInt struct {
// 	Str1 string `db:"str1"`
// 	Str2 string `db:"str2"`
// 	Str3 string `db:"str3"`
// 	Num1 int    `db:"num1"`
// 	Num2 uint   `db:"num2"`
// }

type DbstaticTwoString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
}

const (
	QueryDbseriesGetIdentifiedByID                    = "select lower(identifiedby) from dbseries where id = ?"
	QueryDbserieEpisodesGetSeasonEpisodeByDBID        = "select season, episode from dbserie_episodes where dbserie_id = ?"
	QueryDbserieEpisodesCountByDBID                   = "select count() from dbserie_episodes where dbserie_id = ?"
	QuerySeriesCountByDBID                            = "select count() from series where dbserie_id = ?"
	QueryUpdateHistory                                = "update job_histories set ended = datetime('now','localtime') where id = ?"
	QueryCountMoviesByDBIDList                        = "select count() from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
	QuerySeriesGetIDByDBIDListname                    = "select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE"
	QueryDbseriesGetIDByTvdb                          = "select id from dbseries where thetvdb_id = ?"
	QueryMoviesGetIDByDBIDListname                    = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
	QueryDbmovieTitlesGetTitleByIDLmit1               = "select title from dbmovie_titles where dbmovie_id = ? limit 1"
	QuerySerieEpisodesGetDBSerieEpisodeIDByID         = "select dbserie_episode_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetDBSerieIDByID                = "select dbserie_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetSerieIDByID                  = "select serie_id from serie_episodes where id = ?"
	QueryDBSerieEpisodeGetIDByDBSerieIDIdentifierDot  = "select id from dbserie_episodes where dbserie_id = ? and identifier=REPLACE(?,'.','-') COLLATE NOCASE"
	QueryDBSerieEpisodeGetIDByDBSerieIDIdentifierDash = "select id from dbserie_episodes where dbserie_id = ? and identifier=REPLACE(?,' ','-') COLLATE NOCASE"
)

var (
	strQuery    = "Query"
	readWriteMu = sync.RWMutex{}
	globalVarMu = sync.RWMutex{} // Protects DBConnect, DBVersion, DBLogLevel
	sqlCTX      = context.Background()
	DBConnect   DBGlobal
	dbData      *sqlx.DB
	dbImdb      *sqlx.DB
	DBVersion   = "1"
	DBLogLevel  = "Info"
)

// GetMutex returns the shared read-write mutex used for database operations.
func GetMutex() *sync.RWMutex {
	return &readWriteMu
}

// getGlobalVarMutex returns the mutex used to protect global variables.
func getGlobalVarMutex() *sync.RWMutex {
	return &globalVarMu
}

// GetDBConnect returns a copy of the DBConnect global variable in a thread-safe manner.
func GetDBConnect() DBGlobal {
	globalVarMu.RLock()
	defer globalVarMu.RUnlock()
	return DBConnect
}

// SetDBConnect updates the DBConnect global variable in a thread-safe manner.
func SetDBConnect(dbConnect DBGlobal) {
	globalVarMu.Lock()
	defer globalVarMu.Unlock()
	DBConnect = dbConnect
}

// GetDBVersion returns the current database version in a thread-safe manner.
func GetDBVersion() string {
	globalVarMu.RLock()
	defer globalVarMu.RUnlock()
	return DBVersion
}

// SetDBVersion updates the database version in a thread-safe manner.
func SetDBVersion(version string) {
	globalVarMu.Lock()
	defer globalVarMu.Unlock()
	DBVersion = version
}

// GetDBLogLevel returns the current database log level in a thread-safe manner.
func GetDBLogLevel() string {
	globalVarMu.RLock()
	defer globalVarMu.RUnlock()
	return DBLogLevel
}

// SetDBLogLevel updates the database log level in a thread-safe manner.
func SetDBLogLevel(level string) {
	globalVarMu.Lock()
	defer globalVarMu.Unlock()
	DBLogLevel = level
}

// getdb returns the database connection to use based on
// the imdb parameter. If imdb is true, it returns the
// dbImdb connection, otherwise it returns the dbData
// connection.
func Getdb(imdb bool) *sqlx.DB {
	if imdb {
		return dbImdb
	}
	return dbData
}

// queryGenericsT scans multiple rows from sqlx.Rows into a slice of any type T.
// It handles scanning into simple types as well as structs.
// For structs, it uses the getfunc mapping function to get the fields to scan into.
// Size is a hint for the initial slice capacity.
func queryGenericsT[t any](size uint, rows *sqlx.Rows, querystring string) []t {
	var zero t
	isSimpleType := isSimpleType(zero)

	capacity := size
	if capacity == 0 {
		capacity = 16 // reasonable default
	}
	result := make([]t, 0, capacity)
	for rows.Next() {
		var u t
		var err error

		if isSimpleType {
			err = rows.Scan(&u)
		} else {
			err = getfuncarr(&u, rows)
		}

		if err != nil {
			continue
		}
		result = append(result, u)
	}
	logSQLError(rows.Err(), querystring)
	return result
}

// isSimpleType checks if the given value is a simple type (string, numeric, or boolean).
// It returns true for primitive types that can be directly scanned from a database row,
// and false for complex types like structs or pointers.
func isSimpleType[T any](v T) bool {
	switch any(v).(type) {
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool:
		return true
	default:
		return false
	}
}

// getfuncarr is a helper function that scans the results of a SQL query into a
// generic struct type. It uses a type switch to determine the appropriate
// fields to scan into the struct based on its type. This allows the function
// to be used with a variety of different struct types that have the
// appropriate field types.
func getfuncarr(u any, s *sqlx.Rows) error {
	switch elem := u.(type) {
	case *DbstaticTwoString:
		return s.Scan(&elem.Str1, &elem.Str2)
	case *DbstaticOneStringOneInt:
		return s.Scan(&elem.Str, &elem.Num)
	case *DbstaticOneStringOneUInt:
		return s.Scan(&elem.Str, &elem.Num)
	case *DbstaticOneIntOneBool:
		return s.Scan(&elem.Num, &elem.Bl)
	case *DbstaticTwoUint:
		return s.Scan(&elem.Num1, &elem.Num2)
	case *syncops.DbstaticOneStringTwoInt:
		return s.Scan(&elem.Str, &elem.Num1, &elem.Num2)
	case *syncops.DbstaticTwoStringOneInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Num)
	case *DbstaticTwoStringOneRInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Num)
	case *DbstaticThreeString:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Str3)
	case *syncops.DbstaticThreeStringTwoInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1, &elem.Num2)
	case *ImdbRatings:
		return s.Scan(&elem.AverageRating, &elem.NumVotes)
	case *FilePrio:
		return s.Scan(&elem.Location, &elem.DBID, &elem.ID, &elem.ResolutionID, &elem.QualityID, &elem.CodecID, &elem.AudioID, &elem.Proper, &elem.Repack, &elem.Extended)
	case *string, *int, *int8, *int16, *int32, *int64, *uint, *uint8, *uint16, *uint32, *uint64, *float32, *float64, *bool:
		return s.Scan(u)
	default:
		return s.StructScan(&u)
	}
}

// structscan queries the database using the given query string and scans the
// result into the given struct pointer. It handles locking/unlocking the read
// write mutex, logging any errors, and returning sql.ErrNoRows if no rows were
// returned.
func Structscan[t any](querystring string, imdb bool, id ...any) (*t, error) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var u t
	err := queryRowxContext(querystring, imdb, id).StructScan(&u)
	if err != nil {
		logSQLError(err, querystring)
		return nil, err
	}
	return &u, err
}

// structscan1 executes a SQL query and scans the result into the provided struct.
// The function takes a query string, a boolean indicating whether the query is for an IMDB database,
// a pointer to a struct to scan the result into, and a pointer to a uint to store the ID of the
// scanned row. It returns an error if the query fails.
func structscan1(querystring string, u any, id *uint) error {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	return queryRowxContext(querystring, false, []any{id}).StructScan(u)
}

// StructscanT executes a SQL query and scans the result into a slice of the provided struct type.
// It takes a boolean indicating whether the query is for an IMDB database, the expected size of the
// result set, the SQL query string, and any arguments for the query. It returns a slice of the
// provided struct type containing the scanned rows.
// The function acquires a read lock on the readWriteMu mutex, executes the query, and scans the
// results into the provided struct type. If an error occurs during the query or scanning, it is
// logged and the function returns nil.
func StructscanT[t any](imdb bool, size uint, querystring string, args ...any) []t {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := queryxContext(querystring, imdb, args)
	if err != nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()
	var result []t
	if size != 0 {
		result = make([]t, 0, size)
	}
	var u t
	for rows.Next() {
		err = rows.StructScan(&u)
		if err != nil {
			logSQLError(err, querystring)
			continue
		}
		result = append(result, u)
	}
	logSQLError(rows.Err(), querystring)
	return result
}

// GetDbmovieByID retrieves a Dbmovie by ID. It takes a uint ID
// and returns a Dbmovie struct and error.
// It executes a SQL query using the structscanG function to select the
// dbmovie data and scan it into the Dbmovie struct.
// Returns an error if there was a problem retrieving the data.
func GetDbmovieByID(id *uint) (*Dbmovie, error) {
	return Structscan[Dbmovie](
		"select id,created_at,updated_at,title,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?",
		false,
		id,
	)
}

// QueryDbmovie queries the dbmovies table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of Dbmovie structs.
func QueryDbmovie(qu Querywithargs, args ...any) []Dbmovie {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovies"
	qu.defaultcolumns = "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[Dbmovie](false, qu.Limit, qu.QueryString, args...)
}

type QueryParams struct {
	Table                  string
	DefaultColumns         string
	DefaultQuery           string
	DefaultQueryParamCount int
	DefaultOrderBy         string
	Object                 any
}

func GetTableDefaults(table string) QueryParams {
	var q QueryParams
	switch table {
	case "dbmovies":
		q.Table = "dbmovies"
		q.DefaultColumns = "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
		q.DefaultQuery = " where id like ? or title like ? or year like ? or moviedb_id like ? or imdb_id like ? or slug like ? or trakt_id like ?"
		q.DefaultQueryParamCount = 7
		q.DefaultOrderBy = " order by id desc"
		q.Object = Dbmovie{}
	case "dbmovie_titles":
		q.Table = "dbmovie_titles LEFT JOIN dbmovies ON dbmovie_titles.dbmovie_id = dbmovies.id"
		q.DefaultColumns = "dbmovie_titles.id as id,dbmovie_titles.created_at as created_at,dbmovie_titles.updated_at as updated_at,dbmovie_titles.dbmovie_id as dbmovie_id,dbmovie_titles.title as title,dbmovie_titles.slug as slug,dbmovie_titles.region as region,dbmovies.title as movie_title"
		q.DefaultQuery = " where dbmovie_titles.id like ? or dbmovie_titles.dbmovie_id like ? or dbmovie_titles.title like ? or dbmovie_titles.slug like ? or dbmovie_titles.region like ?"
		q.DefaultQueryParamCount = 5
		q.DefaultOrderBy = " order by dbmovie_titles.id desc"
		q.Object = DbmovieTitle{}
	case "dbseries":
		q.Table = "dbseries"
		q.DefaultColumns = "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
		q.DefaultQuery = " where id like ? or seriename like ? or season like ? or slug like ? or imdb_id like ? or thetvdb_id like ? or trakt_id like ?"
		q.DefaultQueryParamCount = 7
		q.DefaultOrderBy = " order by id desc"
		q.Object = Dbserie{}
	case "dbserie_alternates":
		q.Table = "dbserie_alternates LEFT JOIN dbseries ON dbserie_alternates.dbserie_id = dbseries.id"
		q.DefaultColumns = "dbserie_alternates.id as id,dbserie_alternates.created_at as created_at,dbserie_alternates.updated_at as updated_at,dbserie_alternates.dbserie_id as dbserie_id,dbserie_alternates.title as title,dbserie_alternates.slug as slug,dbserie_alternates.region as region,dbseries.seriename as series_name"
		q.DefaultQuery = " where dbserie_alternates.id like ? or dbserie_alternates.dbserie_id like ? or dbserie_alternates.title like ? or dbserie_alternates.slug like ? or dbserie_alternates.region like ?"
		q.DefaultQueryParamCount = 5
		q.DefaultOrderBy = " order by dbserie_alternates.id desc"
		q.Object = DbserieAlternate{}
	case "dbserie_episodes":
		q.Table = "dbserie_episodes LEFT JOIN dbseries ON dbserie_episodes.dbserie_id = dbseries.id"
		q.DefaultColumns = "dbserie_episodes.id as id,dbserie_episodes.created_at as created_at,dbserie_episodes.updated_at as updated_at,dbserie_episodes.episode as episode,dbserie_episodes.season as season,dbserie_episodes.identifier as identifier,dbserie_episodes.title as title,dbserie_episodes.first_aired as first_aired,dbserie_episodes.overview as overview,dbserie_episodes.poster as poster,dbserie_episodes.runtime as runtime,dbserie_episodes.dbserie_id as dbserie_id,dbseries.seriename as series_name"
		q.DefaultQuery = " where dbserie_episodes.id like ? or dbserie_episodes.episode like ? or dbserie_episodes.season like ? or dbserie_episodes.dbserie_id like ? or dbserie_episodes.title like ? or dbserie_episodes.identifier like ?"
		q.DefaultQueryParamCount = 6
		q.DefaultOrderBy = " order by dbserie_episodes.id desc"
		q.Object = DbserieEpisode{}
	case "movies":
		q.Table = "movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id"
		q.DefaultColumns = "movies.id as id,movies.created_at as created_at,movies.updated_at as updated_at,movies.blacklisted as blacklisted,movies.quality_reached as quality_reached,movies.quality_profile as quality_profile,movies.missing as missing,movies.dont_upgrade as dont_upgrade,movies.dont_search as dont_search,movies.listname as listname,movies.rootpath as rootpath,movies.dbmovie_id as dbmovie_id,dbmovies.title as movie_title"
		q.DefaultQuery = " where movies.id like ? or movies.quality_profile like ? or movies.listname like ? or movies.rootpath like ? or movies.dbmovie_id like ?"
		q.DefaultQueryParamCount = 5
		q.DefaultOrderBy = " order by movies.id desc"
		q.Object = Movie{}
	case "series":
		q.Table = "series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id"
		q.DefaultColumns = "series.id as id,series.created_at as created_at,series.updated_at as updated_at,series.listname as listname,series.rootpath as rootpath,series.dbserie_id as dbserie_id,series.dont_upgrade as dont_upgrade,series.dont_search as dont_search,dbseries.seriename as series_name"
		q.DefaultQuery = " where series.id like ? or series.listname like ? or series.rootpath like ? or series.dbserie_id like ?"
		q.DefaultQueryParamCount = 4
		q.DefaultOrderBy = " order by series.id desc"
		q.Object = Serie{}
	case "serie_episodes":
		q.Table = "serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id"
		q.DefaultColumns = "serie_episodes.id as id,serie_episodes.created_at as created_at,serie_episodes.updated_at as updated_at,serie_episodes.blacklisted as blacklisted,serie_episodes.quality_reached as quality_reached,serie_episodes.quality_profile as quality_profile,serie_episodes.missing as missing,serie_episodes.dont_upgrade as dont_upgrade,serie_episodes.dont_search as dont_search,serie_episodes.dbserie_episode_id as dbserie_episode_id,serie_episodes.serie_id as serie_id,serie_episodes.dbserie_id as dbserie_id,dbserie_episodes.title as episode_title"
		q.DefaultQuery = " where serie_episodes.id like ? or serie_episodes.quality_profile like ? or serie_episodes.dbserie_episode_id like ? or serie_episodes.serie_id like ? or serie_episodes.dbserie_id like ?"
		q.DefaultQueryParamCount = 5
		q.DefaultOrderBy = " order by serie_episodes.id desc"
		q.Object = SerieEpisode{}
	case "job_histories":
		q.Table = "job_histories"
		q.DefaultColumns = "id,created_at,updated_at,job_type,job_category,job_group,started,ended,CASE WHEN started IS NOT NULL AND ended IS NOT NULL THEN ROUND((julianday(ended) - julianday(started)) * 86400) ELSE NULL END as duration"
		q.DefaultQuery = " where id like ? or job_type like ? or job_category like ? or job_group like ?"
		q.DefaultQueryParamCount = 4
		q.DefaultOrderBy = " order by started desc"
		q.Object = JobHistory{}
	case "serie_file_unmatcheds":
		q.Table = "serie_file_unmatcheds LEFT JOIN series ON serie_file_unmatcheds.listname = series.listname"
		q.DefaultColumns = "serie_file_unmatcheds.id as id,serie_file_unmatcheds.created_at as created_at,serie_file_unmatcheds.updated_at as updated_at,serie_file_unmatcheds.listname as listname,serie_file_unmatcheds.filepath as filepath,serie_file_unmatcheds.last_checked as last_checked,serie_file_unmatcheds.parsed_data as parsed_data,series.rootpath as series_rootpath"
		q.DefaultQuery = " where serie_file_unmatcheds.id like ? or serie_file_unmatcheds.listname like ? or serie_file_unmatcheds.filepath like ?"
		q.DefaultQueryParamCount = 3
		q.DefaultOrderBy = " order by serie_file_unmatcheds.id desc"
		q.Object = SerieFileUnmatched{}
	case "movie_file_unmatcheds":
		q.Table = "movie_file_unmatcheds LEFT JOIN movies ON movie_file_unmatcheds.listname = movies.listname"
		q.DefaultColumns = "movie_file_unmatcheds.id as id,movie_file_unmatcheds.created_at as created_at,movie_file_unmatcheds.updated_at as updated_at,movie_file_unmatcheds.listname as listname,movie_file_unmatcheds.filepath as filepath,movie_file_unmatcheds.last_checked as last_checked,movie_file_unmatcheds.parsed_data as parsed_data,movies.quality_profile as movie_quality_profile"
		q.DefaultQuery = " where movie_file_unmatcheds.id like ? or movie_file_unmatcheds.listname like ? or movie_file_unmatcheds.filepath like ?"
		q.DefaultQueryParamCount = 3
		q.DefaultOrderBy = " order by movie_file_unmatcheds.id desc"
		q.Object = MovieFileUnmatched{}
	case "qualities":
		q.Table = "qualities"
		q.DefaultColumns = "id,created_at,updated_at,name,regex,strings,type,priority,regexgroup,use_regex"
		q.DefaultQuery = " where id like ? or name like ? or regex like ? or strings like ? or type like ? or regexgroup like ?"
		q.DefaultQueryParamCount = 6
		q.DefaultOrderBy = " order by id desc"
		q.Object = Qualities{}
	case "movie_files":
		q.Table = "movie_files LEFT JOIN dbmovies ON  movie_files.dbmovie_id = dbmovies.id"
		q.DefaultColumns = "movie_files.id as id,movie_files.location as location,movie_files.filename as filename,movie_files.extension as extension,movie_files.quality_profile as quality_profile,movie_files.created_at as created_at,movie_files.updated_at as updated_at,movie_files.resolution_id as resolution_id,movie_files.quality_id as quality_id,movie_files.codec_id as codec_id,movie_files.audio_id as audio_id,movie_files.movie_id as movie_id,movie_files.dbmovie_id as dbmovie_id,movie_files.height as height,movie_files.width as width,movie_files.proper as proper,movie_files.extended as extended,movie_files.repack as repack,dbmovies.title as movie_title"
		q.DefaultQuery = " where movie_files.id like ? or movie_files.location like ? or movie_files.filename like ? or movie_files.extension like ? or movie_files.quality_profile like ? or movie_files.movie_id like ? or movie_files.dbmovie_id like ?"
		q.DefaultQueryParamCount = 7
		q.DefaultOrderBy = " order by movie_files.id desc"
		q.Object = MovieFile{}
	case "serie_episode_files":
		q.Table = "serie_episode_files LEFT JOIN dbserie_episodes ON serie_episode_files.dbserie_episode_id = dbserie_episodes.id"
		q.DefaultColumns = "serie_episode_files.id as id,serie_episode_files.location as location,serie_episode_files.filename as filename,serie_episode_files.extension as extension,serie_episode_files.quality_profile as quality_profile,serie_episode_files.created_at as created_at,serie_episode_files.updated_at as updated_at,serie_episode_files.resolution_id as resolution_id,serie_episode_files.quality_id as quality_id,serie_episode_files.codec_id as codec_id,serie_episode_files.audio_id as audio_id,serie_episode_files.serie_id as serie_id,serie_episode_files.serie_episode_id as serie_episode_id,serie_episode_files.dbserie_episode_id as dbserie_episode_id,serie_episode_files.dbserie_id as dbserie_id,serie_episode_files.height as height,serie_episode_files.width as width,serie_episode_files.proper as proper,serie_episode_files.extended as extended,serie_episode_files.repack as repack,dbserie_episodes.title as episode_title"
		q.DefaultQuery = " where serie_episode_files.id like ? or serie_episode_files.location like ? or serie_episode_files.filename like ? or serie_episode_files.extension like ? or serie_episode_files.quality_profile like ? or serie_episode_files.serie_id like ? or serie_episode_files.serie_episode_id like ? or serie_episode_files.dbserie_episode_id like ? or serie_episode_files.dbserie_id like ?"
		q.DefaultQueryParamCount = 9
		q.DefaultOrderBy = " order by serie_episode_files.id desc"
		q.Object = SerieEpisodeFile{}
	case "movie_histories":
		q.Table = "movie_histories LEFT JOIN dbmovies ON movie_histories.dbmovie_id = dbmovies.id"
		q.DefaultColumns = "movie_histories.id as id,movie_histories.title as title,movie_histories.url as url,movie_histories.indexer as indexer,movie_histories.type as type,movie_histories.target as target,movie_histories.quality_profile as quality_profile,movie_histories.created_at as created_at,movie_histories.updated_at as updated_at,movie_histories.downloaded_at as downloaded_at,movie_histories.resolution_id as resolution_id,movie_histories.quality_id as quality_id,movie_histories.codec_id as codec_id,movie_histories.audio_id as audio_id,movie_histories.movie_id as movie_id,movie_histories.dbmovie_id as dbmovie_id,movie_histories.blacklisted as blacklisted,dbmovies.title as movie_title"
		q.DefaultQuery = " where movie_histories.id like ? or movie_histories.title like ? or movie_histories.url like ? or movie_histories.indexer like ? or movie_histories.type like ? or movie_histories.target like ? or movie_histories.quality_profile like ? or movie_histories.movie_id like ? or movie_histories.dbmovie_id like ?"
		q.DefaultQueryParamCount = 9
		q.DefaultOrderBy = " order by movie_histories.id desc"
		q.Object = MovieHistory{}
	case "serie_episode_histories":
		q.Table = "serie_episode_histories LEFT JOIN dbserie_episodes ON serie_episode_histories.dbserie_episode_id = dbserie_episodes.id"
		q.DefaultColumns = "serie_episode_histories.id as id,serie_episode_histories.title as title,serie_episode_histories.url as url,serie_episode_histories.indexer as indexer,serie_episode_histories.type as type,serie_episode_histories.target as target,serie_episode_histories.quality_profile as quality_profile,serie_episode_histories.created_at as created_at,serie_episode_histories.updated_at as updated_at,serie_episode_histories.downloaded_at as downloaded_at,serie_episode_histories.resolution_id as resolution_id,serie_episode_histories.quality_id as quality_id,serie_episode_histories.codec_id as codec_id,serie_episode_histories.audio_id as audio_id,serie_episode_histories.serie_id as serie_id,serie_episode_histories.serie_episode_id as serie_episode_id,serie_episode_histories.dbserie_episode_id as dbserie_episode_id,serie_episode_histories.dbserie_id as dbserie_id,serie_episode_histories.blacklisted as blacklisted,dbserie_episodes.title as episode_title"
		q.DefaultQuery = " where serie_episode_histories.id like ? or serie_episode_histories.title like ? or serie_episode_histories.url like ? or serie_episode_histories.indexer like ? or serie_episode_histories.type like ? or serie_episode_histories.target like ? or serie_episode_histories.quality_profile like ? or serie_episode_histories.serie_id like ? or serie_episode_histories.serie_episode_id like ? or serie_episode_histories.dbserie_episode_id like ? or serie_episode_histories.dbserie_id like ?"
		q.DefaultQueryParamCount = 11
		q.DefaultOrderBy = " order by serie_episode_histories.id desc"
		q.Object = SerieEpisodeHistory{}
	}
	return q
}

// QueryDbmovieTitle queries the dbmovie_titles table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of DbmovieTitle structs.
func QueryDbmovieTitle(qu Querywithargs, args ...any) []DbmovieTitle {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovie_titles"
	qu.defaultcolumns = "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[DbmovieTitle](false, qu.Limit, qu.QueryString, args...)
}

// GetDbserieByID retrieves a Dbserie by ID. It takes a uint ID
// and returns a Dbserie struct and error.
// It executes a SQL query using the structscanG function to select the
// dbserie data and scan it into the Dbserie struct.
// Returns an error if there was a problem retrieving the data.
func GetDbserieByID(id *uint) (*Dbserie, error) {
	return Structscan[Dbserie](
		"select id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?",
		false,
		id,
	)
}

// QueryDbserie queries the dbseries table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of Dbserie structs.
func QueryDbserie(qu Querywithargs, args ...any) []Dbserie {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbseries"
	qu.defaultcolumns = "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[Dbserie](false, qu.Limit, qu.QueryString, args...)
}

// QueryDbserieEpisodes queries the dbserie_episodes table based on the provided Querywithargs struct and arguments.
// It sets the query size limit from the Limit field if greater than 0.
// It sets the default columns to query.
// It builds the query string if not already set.
// It executes the query using QueryStaticArrayN to return a slice of DbserieEpisode structs.
func QueryDbserieEpisodes(qu Querywithargs, args ...any) []DbserieEpisode {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[DbserieEpisode](false, qu.Limit, qu.QueryString, args...)
}

// QueryDbserieAlternates queries the dbserie_alternates table based on the provided Querywithargs struct and arguments.
// It sets the query size limit from the Limit field if greater than 0.
// It sets the default columns to query.
// It builds the query string if not already set.
// It executes the query using QueryStaticArrayN to return a slice of DbserieAlternate structs.
func QueryDbserieAlternates(qu Querywithargs, args ...any) []DbserieAlternate {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_alternates"
	qu.defaultcolumns = "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[DbserieAlternate](false, qu.Limit, qu.QueryString, args...)
}

// GetSeries retrieves a Serie struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a Serie struct.
// Returns the Serie struct and any error.
func GetSeries(qu Querywithargs, args ...any) (*Serie, error) {
	qu.Table = logger.StrSeries
	qu.defaultcolumns = "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return Structscan[Serie](qu.QueryString, false, args...)
}

// GetSerieEpisodes retrieves a SerieEpisode struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a SerieEpisode struct.
// Returns a SerieEpisode struct and any error.
func GetSerieEpisodes(qu Querywithargs, args ...any) (*SerieEpisode, error) {
	qu.Table = "serie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return Structscan[SerieEpisode](qu.QueryString, false, args...)
}

// QuerySerieEpisodes retrieves all SerieEpisode records for the given series listname.
// It takes a pointer to a string containing the listname to search for.
// It returns a slice of SerieEpisode structs matching the listname.
func QuerySerieEpisodes(args *string) []SerieEpisode {
	return StructscanT[SerieEpisode](
		false,
		Getdatarow[uint](
			false,
			"select count() from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
			args,
		),
		"select id, quality_reached, quality_profile from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		args,
	)
}

// GetMovies retrieves a Movie struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a Movie struct.
// Returns the Movie struct and any error.
func GetMovies(qu Querywithargs, args ...any) (*Movie, error) {
	qu.Table = "movies"
	qu.defaultcolumns = "id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return Structscan[Movie](qu.QueryString, false, args...)
}

// QueryMovies retrieves all Movie records matching the given listname.
// It takes a string containing the listname to search for.
// It returns a slice of Movie structs matching the listname.
func QueryMovies(args *string) []Movie {
	return StructscanT[Movie](
		false,
		Getdatarow[uint](
			false,
			"select count() from movies where listname = ? COLLATE NOCASE",
			args,
		),
		"select id, quality_reached, quality_profile from movies where listname = ? COLLATE NOCASE",
		args,
	)
}

// QueryJobHistory retrieves JobHistory records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of JobHistory structs matching the query.
func QueryJobHistory(qu Querywithargs, args ...any) []JobHistory {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "job_histories"
	qu.defaultcolumns = "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[JobHistory](false, qu.Limit, qu.QueryString, args...)
}

// QuerySerieFileUnmatched retrieves SerieFileUnmatched records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of SerieFileUnmatched structs matching the query.
func QuerySerieFileUnmatched(qu Querywithargs, args ...any) []SerieFileUnmatched {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[SerieFileUnmatched](false, qu.Limit, qu.QueryString, args...)
}

// QueryMovieFileUnmatched retrieves MovieFileUnmatched records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of MovieFileUnmatched structs matching the query.
func QueryMovieFileUnmatched(qu Querywithargs, args ...any) []MovieFileUnmatched {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "movie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[MovieFileUnmatched](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultMovies retrieves ResultMovies records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultMovies structs matching the query.
func QueryResultMovies(qu Querywithargs, args ...any) []ResultMovies {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "movies"
	qu.defaultcolumns = `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[ResultMovies](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultSeries retrieves ResultSeries records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultSeries structs matching the query.
func QueryResultSeries(qu Querywithargs, args ...any) []ResultSeries {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = logger.StrSeries
	qu.defaultcolumns = `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[ResultSeries](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultSerieEpisodes retrieves ResultSerieEpisodes records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultSerieEpisodes structs matching the query.
func QueryResultSerieEpisodes(qu Querywithargs, args ...any) []ResultSerieEpisodes {
	qu.size = 0
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_episodes"
	qu.defaultcolumns = `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.QueryString == "" {
		qu.buildquery()
	}
	return StructscanT[ResultSerieEpisodes](false, qu.Limit, qu.QueryString, args...)
}

// Buildquery constructs the SQL query string from the Querywithargs fieldbseries.
// It handles adding the SELECT columns, FROM table, JOINs, WHERE, ORDER BY
// and LIMIT clauses based on the configured fields.
func (qu *Querywithargs) buildquery() {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	bld.WriteString("select ")
	switch {
	case qu.Select != "":
		bld.WriteString(qu.Select)
	case logger.ContainsI(qu.defaultcolumns, qu.Table+logger.StrDot):
		bld.WriteString(qu.defaultcolumns)
	default:
		if qu.InnerJoin != "" {
			bld.WriteString(qu.Table)
			bld.WriteString(".*")
		} else {
			bld.WriteString(qu.defaultcolumns)
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
		bld.WriteString(" limit ")
		if qu.Offset != 0 {
			bld.WriteInt(qu.Offset)
			bld.WriteByte(',')
		}
		bld.WriteUInt(qu.Limit)
	}
	qu.QueryString = bld.String()
}

// Scanrows1dyn executes a SQL query and scans the result into the provided object.
// The query string and arguments are passed as parameters.
// If the query fails, the error is logged and returned.
// The function acquires a read lock on the readWriteMu mutex before executing the query,
// and releases it when the function returns.
func Scanrowsdyn(imdb bool, querystring string, obj any, args ...any) {
	scandatarow(imdb, querystring, obj, args)
}

// ScanrowsNArr executes a SQL query and scans the result into the provided object.
// It uses the GlobalCache to retrieve a prepared statement for the given query string,
// and then executes the query with the provided arguments, scanning the result into the
// provided object.
// If an error occurs during the query execution or scanning, it is logged and returned.
func ScanrowsNArr(imdb bool, querystring string, obj any, args []any) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	err := queryRowContext(querystring, imdb, args).Scan(obj)
	logSQLErrorReset(
		err,
		obj,
		querystring,
	)
}

// checkerrorvalue is a helper function that sets the value of the provided object to a zero value
// if the object is a pointer to an int, uint, string, or bool. For other types, it sets the
// element of the object to the zero value using reflection.
//
// This function is used to ensure that any error values returned from a database query are
// properly reset to their zero values before being returned to the caller.
func checkerrorvalue(obj any) {
	if obj == nil {
		return
	}
	switch val := obj.(type) {
	case *int:
		if *val != 0 {
			*val = 0
		}
	case *int8:
		if *val != 0 {
			*val = 0
		}
	case *int16:
		if *val != 0 {
			*val = 0
		}
	case *int32:
		if *val != 0 {
			*val = 0
		}
	case *int64:
		if *val != 0 {
			*val = 0
		}
	case *uint:
		if *val != 0 {
			*val = 0
		}
	case *uint8:
		if *val != 0 {
			*val = 0
		}
	case *uint16:
		if *val != 0 {
			*val = 0
		}
	case *uint32:
		if *val != 0 {
			*val = 0
		}
	case *string:
		if *val != "" {
			*val = ""
		}
	case *bool:
		if *val {
			*val = false
		}
	default:
		reflect.ValueOf(obj).Elem().SetZero()
	}
}

// scandatarow is a helper function that executes a SQL query with the provided arguments and
// scans the result into the given object. It uses the GlobalCache to cache the prepared
// statement for the given query string and database connection. The function also handles
// locking, logging errors, and returning the scanned object.
//
// The function takes the following arguments:
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one.
// - querystring: the SQL query to execute.
// - s: the object to scan the result into.
// - arg, arg2, arg3: the arguments to pass to the SQL query.
func scandatarow(imdb bool, querystring string, s any, args []any) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	err := queryRowContext(querystring, imdb, args).Scan(s)
	logSQLErrorReset(err, s, querystring)
}

// queryRowContext is a helper function that executes a SQL query with the provided arguments.
// It uses the GlobalCache to cache the prepared statement for the given query string and database connection.
// The function handles both cached statement pointers and non-pointer statements.
//
// The function takes the following arguments:
// - querystring: the SQL query to execute
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one
// - args: variadic arguments to pass to the SQL query
//
// Returns a *sql.Row from executing the query
func queryRowContext(querystring string, imdb bool, args []any) *sql.Row {
	stmtp := globalCache.getXStmt(querystring, imdb)
	return stmtp.QueryRowContext(sqlCTX, args...)
}

// queryRowxContext is a helper function that executes a SQL query using sqlx with the provided arguments.
// It uses the GlobalCache to cache the prepared statement for the given query string and database connection.
// The function handles both cached statement pointers and non-pointer statements.
//
// The function takes the following arguments:
// - querystring: the SQL query to execute
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one
// - args: variadic arguments to pass to the SQL query
//
// Returns a *sqlx.Row from executing the query
func queryRowxContext(querystring string, imdb bool, args []any) *sqlx.Row {
	stmt := globalCache.getXStmt(querystring, imdb)
	return stmt.QueryRowxContext(sqlCTX, args...)
}

// queryxContext is a helper function that executes a SQL query using sqlx with the provided arguments.
// It uses the GlobalCache to cache the prepared statement for the given query string and database connection.
// The function handles both cached statement pointers and non-pointer statements.
//
// The function takes the following arguments:
// - querystring: the SQL query to execute
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one
// - args: variadic arguments to pass to the SQL query
//
// Returns a *sqlx.Rows and an error from executing the query
func queryxContext(querystring string, imdb bool, args []any) (*sqlx.Rows, error) {
	stmt := globalCache.getXStmt(querystring, imdb)
	return stmt.QueryxContext(sqlCTX, args...)
}

// Getdatarow is a generic function that executes a SQL query with the provided arguments and
// returns the result as a value of type uint. It uses the GlobalCache to cache the prepared
// statement for the given query string and database connection.
//
// The function takes the following arguments:
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one.
// - querystring: the SQL query to execute.
//
// The function returns a value of type uint, which is the result of the SQL query.
func Getdatarow[o any](imdb bool, querystring string, args ...any) o {
	var s o
	scandatarow(imdb, querystring, &s, args)
	return s
}

// logSQLError logs an SQL error, handling cases where the error is due to the database being closed.
// If the error is not a "sql: database is closed" error, it logs the error using the logger.LogDynamicany function.
// If the error is a "sql: database is closed" error, it deletes the corresponding cache item.
func logSQLError(err error, querystring string) {
	if err == nil {
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.Logtype("error", 1).
			Str(strQuery, querystring).
			Err(err).
			Msg("exec")
	}
	if err.Error() == "sql: database is closed" {
		syncops.QueueSyncMapDeleteFuncImdbVal(syncops.MapTypeXStmt, func(x bool) bool {
			return x
		}, func(s sqlx.Stmt) {
			s.Close()
		})
	}
}

// logSQLErrorReset logs an SQL error, checks the error value, and then calls logSQLError.
// If the error is not nil, it calls checkerrorvalue with the provided 's' argument.
// It then calls logSQLError with the error and the provided querystring argument.
func logSQLErrorReset(err error, s any, querystring string) {
	if err != nil {
		checkerrorvalue(s)
	}
	logSQLError(err, querystring)
}

// GetdatarowArgs executes the given querystring with the provided argument
// and scans the result into the given slice of objects, handling locking,
// logging errors, and returning the scanned objects.
func GetdatarowArgs(querystring string, arg any, objs ...any) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	err := queryRowContext(querystring, false, []any{arg}).Scan(objs...)
	logSQLError(
		err,
		querystring,
	)
}

// GetdatarowArgsImdb executes the given querystring with the provided argument
// and scans the result into the given slice of objects, handling locking,
// logging errors, and returning the scanned objects. This version of the function
// uses the "imdb" database connection instead of the default one.
func GetdatarowArgsImdb(querystring string, arg any, objs ...any) error {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	err := queryRowContext(querystring, true, []any{arg}).Scan(objs...)
	logSQLError(
		err,
		querystring,
	)
	return err
}

// Getrowssize executes the given querystring with the provided argument against the database,
// scans the result rows into a slice of the generic type T, handles locking, logging errors,
// and returns the slice. The sizeq parameter limits the number of rows scanned.
// If imdb is true, the query will be executed against the imdb database, otherwise the main database.
func Getrowssize[t any](imdb bool, sizeq, querystring string, args ...any) []t {
	return GetrowsN[t](imdb, Getdatarow[uint](imdb, sizeq, args...), querystring, args...)
}

// GetrowsN executes the given querystring with multiple arguments against the database, scans the result
// rows into a slice of the generic type T, handles locking, logging errors,
// and returns the slice. The size parameter limits the number of rows scanned.
// If imdb is true, the query will be executed against the imdb database, otherwise the main database.
func GetrowsN[t any](imdb bool, size uint, querystring string, args ...any) []t {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := queryxContext(querystring, imdb, args)
	if err != nil || rows == nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()
	return queryGenericsT[t](size, rows, querystring)
}

func GetrowsType(o any, imdb bool, size uint, querystring string, args ...any) []map[string]any {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := queryxContext(querystring, imdb, args)
	if err != nil || rows == nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()

	capacity := size
	if capacity == 0 {
		capacity = 16 // reasonable default
	}
	result := make([]map[string]any, 0, capacity)

	for rows.Next() {
		o := make(map[string]any)
		err := rows.MapScan(o)
		if err == nil {
			result = append(result, o)
		}
	}

	logSQLError(rows.Err(), querystring)
	return result
}

func GetrowsTypeOLD(o any, imdb bool, size uint, querystring string, args ...any) []map[string]any {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := queryxContext(querystring, imdb, args)
	if err != nil || rows == nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()

	capacity := size
	if capacity == 0 {
		capacity = 16 // reasonable default
	}
	result := make([]map[string]any, 0, capacity)

	columns, _ := rows.Columns()
	count := len(columns)
	values := make([]any, count)
	valuePtrs := make([]any, count)

	for rows.Next() {
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		rows.Scan(valuePtrs...)

		obj := map[string]any{}
		for i, column := range columns {
			obj[column] = values[i]
		}

		result = append(result, obj)
	}

	logSQLError(rows.Err(), querystring)
	return result
}

// GetrowsNuncached executes a SQL query and returns the results as a slice of the specified type T.
// It acquires a read lock on the readWriteMu mutex before executing the query, and releases the lock when the function returns.
// If the query executes successfully, it calls queryGenericsT to convert the rows to the specified type and returns the slice.
// If the query fails with an error other than sql.ErrNoRows, it logs the error using logger.LogDynamicany.
// If the query fails with sql.ErrNoRows, it returns a nil slice.
func GetrowsNuncached[t DbstaticTwoUint | DbstaticOneStringOneUInt | uint](
	size uint,
	querystring string,
	args []any,
) []t {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := dbData.QueryxContext(sqlCTX, querystring, args...)
	if err != nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()
	return queryGenericsT[t](size, rows, querystring)
}

// ExecN executes the given query string with the provided arguments, and returns the result or any error.
// It acquires a read/write lock before executing the query to ensure thread-safety, and releases the lock when the query is complete.
// If the query fails due to the database being closed, it removes the query from the cache to prevent future failed attempts.
// If the query fails for any other reason, it logs the error.
func ExecN(querystring string, args ...any) {
	exec(querystring, args)
}

// ExecNErr executes the given SQL query string with the provided arguments and returns any error that occurred.
// The function acquires a read/write lock before executing the query to ensure thread-safety.
// If an error occurs during the execution, it is returned.
func ExecNErr(querystring string, args ...any) error {
	_, err := exec(querystring, args)
	return err
}

// ExecNMap executes a database query using the provided query string and arguments.
// If useseries is true, it uses the query string from the logger.Mapstringsseries map,
// otherwise it uses the query string from the logger.Mapstringsmovies map.
// The function acquires a read/write lock before executing the query and releases it after the query is executed.
// If an error occurs during the query execution, it is logged using the logExecError function.
func ExecNMap(useseries bool, query string, args ...any) {
	ExecN(logger.GetStringsMap(useseries, query), args...)
}

// ExecNid executes the given querystring with multiple arguments, returns the generated ID from the insert statement, handles errors.
func ExecNid(querystring string, args ...any) (int64, error) {
	dbresult, err := exec(querystring, args)
	if err != nil {
		return 0, err
	}
	newid, err := dbresult.LastInsertId()
	if err != nil {
		return 0, err
	}
	return newid, nil
}

// exec executes the given SQL query with the provided arguments and logs any errors that occur.
// It acquires a read/write lock before executing the query and releases it when the query is complete.
// The querystring parameter specifies the SQL query to execute.
// The arg, arg2, and arg3 parameters are the arguments to pass to the query.
// If the query fails, it logs the error and returns it.
func exec(querystring string, args []any) (sql.Result, error) {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	stmt := globalCache.getXStmt(querystring, false)

	r, err := stmt.ExecContext(sqlCTX, args...)
	if err != nil {
		logger.Logtype("error", 1).
			Str(strQuery, querystring).
			Err(err).
			Msg("query exec")
		return nil, err
	}
	return r, nil
}

// InsertArray inserts a row into the given database table, with the provided columns and values.
// The number of columns must match the number of value parameters.
// It handles building the SQL insert statement from the parameters, executing the insert,
// and returning the result or any error.
func InsertArray(table string, columns []string, values ...any) (sql.Result, error) {
	if len(columns) != len(values) {
		return nil, errors.New("wrong number of columns")
	}
	return exec(
		"insert into "+table+" ("+strings.Join(
			columns,
			",",
		)+") values (?"+strings.Repeat(
			",?",
			len(columns)-1,
		)+")",
		values,
	)
}

// UpdateArray updates rows in the given database table by setting the provided
// columns to the corresponding value parameters. It builds the SQL UPDATE
// statement dynamically based on the parameters. The optional where parameter
// allows specifying a WHERE clause to filter the rows to update. It handles
// executing the statement and returning the result or any error.
func UpdateArray(table string, columns []string, where string, args ...any) (sql.Result, error) {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	bld.WriteString("update ")
	bld.WriteString(table)
	bld.WriteString(" set ")
	for idx := range columns {
		if idx != 0 {
			bld.WriteByte(',')
		}
		bld.WriteString(columns[idx])
		bld.WriteString(" = ?")
	}
	if where != "" {
		bld.WriteString(" where ")
		bld.WriteString(where)
	}
	return exec(bld.String(), args)
}

// DeleteRow deletes rows from the given database table that match the provided
// WHERE clause and arguments. It returns the sql.Result and error from the
// query execution. The table parameter specifies the table name to delete from.
// The where parameter allows specifying a WHERE condition to filter the rows
// to delete. The args parameters allow providing arguments to replace any ?
// placeholders in the where condition.
func DeleteRow(table, where string, args ...any) (sql.Result, error) {
	querystring := "delete from " + table
	if where != "" {
		querystring = querystring + " where " + where
	}
	if GetDBLogLevel() == logger.StrDebug {
		logger.Logtype("debug", 2).
			Str(strQuery, querystring).
			Interface("args", args).
			Msg("query delete")
	}
	return exec(querystring, args)
}

// queryrowfulllockconnect executes the given SQL query while holding a write lock
// on the database. It scans the result into str and returns any error.
func queryrowfulllockconnect(query string) string {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	var str string
	err := dbData.QueryRowContext(sqlCTX, query).Scan(&str)
	if err == nil {
		return str
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.Logtype("error", 1).
			Str(strQuery, query).
			Err(err).
			Msg("select")
	}
	return ""
}

// DBQuickCheck checks the database for errors using the
// PRAGMA quick_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBQuickCheck() string {
	logger.Logtype("info", 0).
		Msg("Check Database for Errors")
	return queryrowfulllockconnect("PRAGMA quick_check;")
}

// DBIntegrityCheck checks the database integrity using the
// PRAGMA integrity_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBIntegrityCheck() string {
	logger.Logtype("info", 0).
		Msg("Check Database for Integrity")
	return queryrowfulllockconnect("PRAGMA integrity_check;")
}

// Getentryalternatetitlesdirect retrieves a slice of DbstaticTwoStringOneInt objects that represent alternate titles for the movie with the given database ID. If the UseMediaCache setting is enabled, it will retrieve the titles from the cache. Otherwise, it will retrieve the titles directly from the database.
func Getentryalternatetitlesdirect(dbid *uint, useseries bool) []syncops.DbstaticTwoStringOneInt {
	if dbid == nil {
		return nil
	}
	if config.GetSettingsGeneral().UseMediaCache {
		return GetCachedTwoStringArr(
			logger.GetStringsMap(useseries, logger.CacheMediaTitles),
			false,
			true,
		)
	}
	return Getrowssize[syncops.DbstaticTwoStringOneInt](
		false,
		logger.GetStringsMap(useseries, logger.DBCountDBTitlesDBID),
		logger.GetStringsMap(useseries, logger.DBDistinctDBTitlesDBID),
		dbid,
	)
}

// GetDbstaticTwoStringOneInt returns a slice of DbstaticTwoStringOneInt objects that match the given id.
// It filters the input slice to include only elements where Num equals the specified id.
func GetDbstaticTwoStringOneInt(s []syncops.DbstaticTwoStringOneInt, id uint) []syncops.DbstaticTwoStringOneInt {
	if len(s) == 0 {
		return nil
	}

	// Filter elements that match the ID
	var result []syncops.DbstaticTwoStringOneInt
	for idx := range s {
		if s[idx].Num == id {
			result = append(result, s[idx])
		}
	}

	// Return nil if no matches found, otherwise return the filtered slice
	if len(result) == 0 {
		return nil
	}
	return result
}

// ExchangeImdbDB exchanges the imdb.db file with a temp copy.
// It first checks if the main imdb.db file exists, locks the db,
// makes the main file writable, deletes it, renames the temp
// copy to the main name, unlocks the db, and logs the result.
func ExchangeImdbDB() {
	dbfile := "./databases/imdb.db"
	dbfiletemp := "./databases/imdbtemp.db"
	if !checkFile(dbfile) {
		return
	}
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	dbImdb.Close()

	os.Chmod(dbfile, 0o777)
	os.Remove(dbfile)
	err := os.Rename(dbfiletemp, dbfile)
	if err == nil {
		logger.Logtype("debug", 1).
			Str(logger.StrFile, dbfiletemp).
			Msg("File renamed")
	} else {
		logger.Logtype("error", 1).
			Str(logger.StrFile, dbfiletemp).
			Str("target", dbfile).
			Err(err).
			Msg("Failed to rename database file")
	}
	InitImdbdb()
}

// ChecknzbtitleC checks if the given nzbtitle matches the title or alternate title of the movie. It also allows checking for the movie title with the year before and after the given year.
func ChecknzbtitleC(movie *syncops.DbstaticTwoStringOneInt,
	nzbtitle string,
	allowpm1 bool,
	yearu uint16,
) bool {
	if strings.EqualFold(movie.Str1, nzbtitle) {
		return true
	}
	return ChecknzbtitleB(movie.Str1, movie.Str2, nzbtitle, allowpm1, yearu)
}
