package database

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
type DbstaticOneStringTwoInt struct {
	Str  string `db:"str"`
	Num1 uint   `db:"num1"`
	Num2 uint   `db:"num2"`
}

type DbstaticTwoStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Num  uint   `db:"num"`
}
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
type DbstaticThreeStringTwoInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
	Num1 int    `db:"num1"`
	Num2 uint   `db:"num2"`
}

type DbstaticTwoString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
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
func queryGenericsT[t any](size uint, rows *sql.Rows, querystring string) []t {
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
func getfuncarr(u any, s *sql.Rows) error {
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
	case *DbstaticOneStringTwoInt:
		return s.Scan(&elem.Str, &elem.Num1, &elem.Num2)
	case *DbstaticTwoStringOneInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Num)
	case *DbstaticTwoStringOneRInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Num)
	case *DbstaticThreeString:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Str3)
	case *DbstaticThreeStringTwoInt:
		return s.Scan(&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1, &elem.Num2)
	case *ImdbRatings:
		return s.Scan(&elem.AverageRating, &elem.NumVotes)
	case *FilePrio:
		return s.Scan(&elem.Location, &elem.DBID, &elem.ID, &elem.ResolutionID, &elem.QualityID, &elem.CodecID, &elem.AudioID, &elem.Proper, &elem.Repack, &elem.Extended)
	case *string, *int, *int8, *int16, *int32, *int64, *uint, *uint8, *uint16, *uint32, *uint64, *float32, *float64, *bool:
		return s.Scan(u)
	default:
		return logger.ErrNotFound
	}
}

// structscan queries the database using the given query string and scans the
// result into the given struct pointer. It handles locking/unlocking the read
// write mutex, logging any errors, and returning sql.ErrNoRows if no rows were
// returned.
func structscan[t any](querystring string, imdb bool, id ...any) (*t, error) {
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
	return structscan[Dbmovie](
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
	return structscan[Dbserie](
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
	return structscan[Serie](qu.QueryString, false, args...)
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
	return structscan[SerieEpisode](qu.QueryString, false, args...)
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
	return structscan[Movie](qu.QueryString, false, args...)
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

// Buildquery constructs the SQL query string from the Querywithargs fields.
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
	stmtp := globalCache.getXStmtP(querystring)
	if stmtp == nil {
		// no pointer
		stmt := globalCache.getXStmt(querystring, imdb)
		return stmt.QueryRowContext(sqlCTX, args...)
	}
	return stmtp.QueryRowContext(sqlCTX, args...)
}

// queryContext is a helper function that executes a SQL query with the provided arguments.
// It uses the GlobalCache to cache the prepared statement for the given query string and database connection.
// The function handles both cached statement pointers and non-pointer statements.
//
// The function takes the following arguments:
// - querystring: the SQL query to execute
// - imdb: a boolean indicating whether to use the "imdb" database connection or the default one
// - args: variadic arguments to pass to the SQL query
//
// Returns a *sql.Rows and an error from executing the query
func queryContext(querystring string, imdb bool, args []any) (*sql.Rows, error) {
	stmtp := globalCache.getXStmtP(querystring)
	if stmtp == nil {
		// no pointer
		stmt := globalCache.getXStmt(querystring, imdb)
		return stmt.QueryContext(sqlCTX, args...)
	}
	return stmtp.QueryContext(sqlCTX, args...)
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
	stmtp := globalCache.getXStmtP(querystring)
	if stmtp == nil {
		// no pointer
		stmt := globalCache.getXStmt(querystring, imdb)
		return stmt.QueryRowxContext(sqlCTX, args...)
	}
	return stmtp.QueryRowxContext(sqlCTX, args...)
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
	stmtp := globalCache.getXStmtP(querystring)
	if stmtp == nil {
		// no pointer
		stmt := globalCache.getXStmt(querystring, imdb)
		return stmt.QueryxContext(sqlCTX, args...)
	}
	return stmtp.QueryxContext(sqlCTX, args...)
}

// Getdatarow0 is a generic function that executes a SQL query with the provided arguments and
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
		logger.LogDynamicany1StringErr("error", "exec", err, strQuery, querystring)
	}
	if err.Error() == "sql: database is closed" {
		cache.itemsxstmt.DeleteFuncImdbVal(func(x bool) bool {
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
func GetdatarowArgsImdb(querystring string, arg any, objs ...any) {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	err := queryRowContext(querystring, true, []any{arg}).Scan(objs...)
	logSQLError(
		err,
		querystring,
	)
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
	rows, err := queryContext(querystring, imdb, args)
	if err != nil || rows == nil {
		logSQLError(err, querystring)
		return nil
	}
	defer rows.Close()
	return queryGenericsT[t](size, rows, querystring)
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
	rows, err := dbData.QueryContext(sqlCTX, querystring, args...)
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
	stmtp := globalCache.getXStmtP(querystring)
	var r sql.Result
	var err error

	if stmtp == nil {
		// no pointer
		stmt := globalCache.getXStmt(querystring, false)
		r, err = stmt.ExecContext(sqlCTX, args...)
	} else {
		r, err = stmtp.ExecContext(sqlCTX, args...)
	}
	if err != nil {
		logger.LogDynamicany1StringErr("error", "query exec", err, strQuery, querystring)
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
	if DBLogLevel == logger.StrDebug {
		logger.LogDynamicany2StrAny("debug", "query delete", strQuery, querystring, "args", args)
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
		logger.LogDynamicany1StringErr("error", "select", err, strQuery, query)
	}
	return ""
}

// DBQuickCheck checks the database for errors using the
// PRAGMA quick_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBQuickCheck() string {
	logger.LogDynamicany0("info", "Check Database for Errors")
	return queryrowfulllockconnect("PRAGMA quick_check;")
}

// DBIntegrityCheck checks the database integrity using the
// PRAGMA integrity_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBIntegrityCheck() string {
	logger.LogDynamicany0("info", "Check Database for Integrity")
	return queryrowfulllockconnect("PRAGMA integrity_check;")
}

// Getentryalternatetitlesdirect retrieves a slice of DbstaticTwoStringOneInt objects that represent alternate titles for the movie with the given database ID. If the UseMediaCache setting is enabled, it will retrieve the titles from the cache. Otherwise, it will retrieve the titles directly from the database.
func Getentryalternatetitlesdirect(dbid *uint, useseries bool) []DbstaticTwoStringOneInt {
	if config.SettingsGeneral.UseMediaCache {
		return GetCachedArr(cache.itemstwostring,
			logger.GetStringsMap(useseries, logger.CacheMediaTitles),
			false,
			true,
		)
	}
	return Getrowssize[DbstaticTwoStringOneInt](
		false,
		logger.GetStringsMap(useseries, logger.DBCountDBTitlesDBID),
		logger.GetStringsMap(useseries, logger.DBDistinctDBTitlesDBID),
		dbid,
	)
}

// GetDbstaticTwoStringOneInt returns a slice of DbstaticTwoStringOneInt objects that match the given id. It first finds the index of the first match, then sorts the slice and returns the subslice containing all matches.
func GetDbstaticTwoStringOneInt(s []DbstaticTwoStringOneInt, id uint) []DbstaticTwoStringOneInt {
	if len(s) == 0 {
		return nil
	}
	a := -1
	for idx := range s {
		if s[idx].Num == id {
			if a == -1 {
				a = idx
				break
			}
		}
	}
	if a == -1 {
		return nil
	}
	slices.SortFunc(s, func(a, b DbstaticTwoStringOneInt) int {
		return cmp.Compare(a.Num, b.Num)
	})
	i, j := -1, -1
	for idx := range s {
		if s[idx].Num == id {
			if i == -1 {
				i = idx
			}
			j = idx
		}
	}
	if i == -1 || j == -1 {
		return nil
	}
	if j == 0 {
		return s[i:]
	}
	return s[i : j+1]
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
		logger.LogDynamicany1String("debug", "File renamed", logger.StrFile, dbfiletemp)
	}
	InitImdbdb()
}

// ChecknzbtitleC checks if the given nzbtitle matches the title or alternate title of the movie. It also allows checking for the movie title with the year before and after the given year.
func (movie *DbstaticTwoStringOneInt) ChecknzbtitleC(
	nzbtitle string,
	allowpm1 bool,
	yearu uint16,
) bool {
	if strings.EqualFold(movie.Str1, nzbtitle) {
		return true
	}
	return ChecknzbtitleB(movie.Str1, movie.Str2, nzbtitle, allowpm1, yearu)
}
