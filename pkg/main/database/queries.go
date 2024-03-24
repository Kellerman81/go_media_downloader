package database

import (
	"database/sql"
	"errors"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/jmoiron/sqlx"
)

// ParseInfo is a struct containing parsed information about media files
type ParseInfo struct {
	// File is the path to the media file
	File string
	// Title is the title of the media
	Title string
	// Season is the season number, if applicable
	Season int `json:"season,omitempty"`
	// Episode is the episode number, if applicable
	Episode int `json:"episode,omitempty"`
	// SeasonStr is the season number as a string, if applicable
	SeasonStr string `json:"seasonstr,omitempty"`
	// EpisodeStr is the episode number as a string, if applicable
	EpisodeStr string `json:"episodestr,omitempty"`
	// Year is the year of release
	Year int `json:"year,omitempty"`
	// Resolution is the video resolution
	Resolution string `json:"resolution,omitempty"`
	// ResolutionID is the database ID of the resolution
	ResolutionID uint `json:"resolutionid,omitempty"`
	// Quality is the video quality description
	Quality string `json:"quality,omitempty"`
	// QualityID is the database ID of the quality
	QualityID uint `json:"qualityid,omitempty"`
	// Codec is the video codec
	Codec string `json:"codec,omitempty"`
	// CodecID is the database ID of the codec
	CodecID uint `json:"codecid,omitempty"`
	// Audio is the audio description
	Audio string `json:"audio,omitempty"`
	// AudioID is the database ID of the audio
	AudioID uint `json:"audioid,omitempty"`
	// Priority is the priority for downloading
	Priority int `json:"priority,omitempty"`
	// Identifier is an identifier string
	Identifier string `json:"identifier,omitempty"`
	// Date is the release date
	Date string `json:"date,omitempty"`
	// Extended is a flag indicating if it is an extended version
	Extended bool `json:"extended,omitempty"`
	// Proper is a flag indicating if it is a proper release
	Proper bool `json:"proper,omitempty"`
	// Repack is a flag indicating if it is a repack release
	Repack bool `json:"repack,omitempty"`
	// Imdb is the IMDB ID
	Imdb string `json:"imdb,omitempty"`
	// Tvdb is the TVDB ID
	Tvdb string `json:"tvdb,omitempty"`
	// Qualityset is the quality configuration
	Qualityset *config.QualityConfig `json:"qualityset,omitempty"`
	// Languages is a list of language codes
	Languages []string `json:"languages,omitempty"`
	// Runtime is the runtime in minutes
	Runtime int `json:"runtime,omitempty"`
	// Height is the video height in pixels
	Height int `json:"height,omitempty"`
	// Width is the video width in pixels
	Width int `json:"width,omitempty"`
	// DbmovieID is the database ID of the movie
	DbmovieID uint `json:"dbmovieid,omitempty"`
	// MovieID is the application ID of the movie
	MovieID uint `json:"movieid,omitempty"`
	// DbserieID is the database ID of the TV series
	DbserieID uint `json:"dbserieid,omitempty"`
	// DbserieEpisodeID is the database ID of the episode
	DbserieEpisodeID uint `json:"dbserieepisodeid,omitempty"`
	// SerieID is the application ID of the TV series
	SerieID uint `json:"serieid,omitempty"`
	// SerieEpisodeID is the application ID of the episode
	SerieEpisodeID uint `json:"serieepisodeid,omitempty"`
	// ListID is the ID of the list this came from
	ListID int

	//SluggedTitle     string
	//Listname         string   `json:"listname,omitempty"`
	//ListCfg *config.ListsConfig
	//Group           string   `json:"group,omitempty"`
	//Region          string   `json:"region,omitempty"`
	//Hardcoded       bool     `json:"hardcoded,omitempty"`
	//Container       string   `json:"container,omitempty"`
	//Widescreen      bool     `json:"widescreen,omitempty"`
	//Website         string   `json:"website,omitempty"`
	//Sbs             string   `json:"sbs,omitempty"`
	//Unrated         bool     `json:"unrated,omitempty"`
	//Subs            string   `json:"subs,omitempty"`
	//ThreeD          bool     `json:"3d,omitempty"`
}

// Querywithargs is a struct to hold query arguments
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
	Limit int
	// Offset is the OFFSET clause value
	Offset int
	// defaultcolumns is used for default columns
	defaultcolumns string
	// size is the size of the result
	size int
}

type DbstaticOneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

type dbstaticOneInt struct {
	Num int `db:"num"`
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
	Num1 int    `db:"num1"`
	Num2 int    `db:"num2"`
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
type dbstaticThreeStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
	Num1 int    `db:"num1"`
}
type DbstaticThreeStringTwoInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
	Num1 int    `db:"num1"`
	Num2 int    `db:"num2"`
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
	QueryDbseriesGetIdentifiedByID                 = "select lower(identifiedby) from dbseries where id = ?"
	QueryDbserieEpisodesGetSeasonEpisodeByDBID     = "select season, episode from dbserie_episodes where dbserie_id = ?"
	QueryDbserieEpisodesCountByDBID                = "select count() from dbserie_episodes where dbserie_id = ?"
	QuerySeriesCountByDBID                         = "select count() from series where dbserie_id = ?"
	QueryUpdateHistory                             = "update job_histories set ended = datetime('now','localtime') where id = ?"
	QueryCountMoviesByDBIDList                     = "select count() from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
	QuerySeriesGetIDByDBIDListname                 = "select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE"
	QueryDbseriesGetIDByTvdb                       = "select id from dbseries where thetvdb_id = ?"
	QueryMoviesGetIDByDBIDListname                 = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
	QueryDbseriesGetIDByName                       = "select id from dbseries where seriename = ? COLLATE NOCASE"
	QueryDbmovieTitlesGetTitleByIDLmit1            = "select title from dbmovie_titles where dbmovie_id = ? limit 1"
	QuerySerieEpisodesGetDBSerieEpisodeIDByID      = "select dbserie_episode_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetDBSerieIDByID             = "select dbserie_id from serie_episodes where id = ?"
	QuerySerieEpisodesGetSerieIDByID               = "select serie_id from serie_episodes where id = ?"
	QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier  = "select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE"
	QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier2 = "select id from dbserie_episodes where dbserie_id = ? and identifier=REPLACE(?,?,?) COLLATE NOCASE"
	sel                                            = ("select ")
	coun                                           = ("count()")
	all                                            = (".*")
	from                                           = (" from ")
	join                                           = (" inner join ")
	where                                          = (" where ")
	order                                          = (" order by ")
	limit                                          = (" limit ")
	bqaudio                                        = (" Audioid: ")
	bqcodec                                        = (" Codecid: ")
	bqquality                                      = (" Qualityid: ")
	bqresolution                                   = (" Resolutionid: ")
	bqepisode                                      = (" Episode: ")
	bqIdentifier                                   = (" Identifier: ")
	bqListname                                     = (" Listname: ")
	bqSeason                                       = (" Season: ")
	bqTitle                                        = (" Title: ")
	bqTvdb                                         = (" Tvdb: ")
	bqImdb                                         = (" Imdb: ")
	bqYear                                         = (" Year: ")
)

var (
	readWriteMu = &sync.RWMutex{}
	DBConnect   dbGlobal
	dbData      *sqlx.DB
	dbImdb      *sqlx.DB
	DBVersion   = "1"
	DBLogLevel  = "Info"
)

// cachedelete deletes an item from the cache by key. It first checks the type of
// the cached value and handles closing any open resources before deleting the
// key/value pair from the cache. This helps prevent memory leaks by cleaning up
// prepared statements, connections, etc. that are no longer needed.
func cachedelete(key, value any) {
	if value != nil {
		switch item := value.(type) {
		case *itemstmt:
			if item.value != nil {
				item.value.Close()
				if item.value != nil {
					*item.value = sqlx.Stmt{}
				}
			}
			*item = itemstmt{}
		case *itemregex:
			*item.value = regexp.Regexp{}
			*item = itemregex{}
		case *cacheOneStringIntExpire:
			clear(item.Arr)
			*item = cacheOneStringIntExpire{}
		case *CacheOneStringTwoIntExpire:
			clear(item.Arr)
			*item = CacheOneStringTwoIntExpire{}
		case *CacheThreeStringTwoIntExpire:
			clear(item.Arr)
			*item = CacheThreeStringTwoIntExpire{}
		case *cacheStringExpire:
			clear(item.Arr)
			*item = cacheStringExpire{}
		case *cacheTwoIntExpire:
			clear(item.Arr)
			*item = cacheTwoIntExpire{}
		case *CacheTwoStringIntExpire:
			clear(item.Arr)
			*item = CacheTwoStringIntExpire{}
		}
	}
	cache.items.Delete(key)
}

// Buildparsedstring concatenates the ParseInfo fields into a string for logging.
func (m *ParseInfo) Buildparsedstring() string {
	bld := logger.PlBuffer.Get()
	bld.Grow(350)
	//defer bld.Reset()
	if m.AudioID != 0 {
		bld.WriteString(bqaudio)
		logger.BuilderAddUint(bld, m.AudioID)
	}
	if m.CodecID != 0 {
		bld.WriteString(bqcodec)
		logger.BuilderAddUint(bld, m.CodecID)
	}
	if m.QualityID != 0 {
		bld.WriteString(bqquality)
		logger.BuilderAddUint(bld, m.QualityID)
	}
	if m.ResolutionID != 0 {
		bld.WriteString(bqresolution)
		logger.BuilderAddUint(bld, m.ResolutionID)
	}
	if m.EpisodeStr != "" {
		bld.WriteString(bqepisode)
		bld.WriteString(m.EpisodeStr)
	}
	if m.Identifier != "" {
		bld.WriteString(bqIdentifier)
		bld.WriteString(m.Identifier)
	}
	if m.ListID != -1 {
		bld.WriteString(bqListname)
		logger.BuilderAddInt(bld, m.ListID)
	}
	if m.SeasonStr != "" {
		bld.WriteString(bqSeason)
		bld.WriteString(m.SeasonStr)
	}
	if m.Title != "" {
		bld.WriteString(bqTitle)
		bld.WriteString(m.Title)
	}
	if m.Tvdb != "" {
		bld.WriteString(bqTvdb)
		bld.WriteString(m.Tvdb)
	}
	if m.Imdb != "" {
		bld.WriteString(bqImdb)
		bld.WriteString(m.Imdb)
	}
	if m.Year != 0 {
		bld.WriteString(bqYear)
		logger.BuilderAddInt(bld, m.Year)
	}
	defer logger.PlBuffer.Put(bld)
	return bld.String()
}

// getdb returns the database connection to use based on
// the imdb parameter. If imdb is true, it returns the
// dbImdb connection, otherwise it returns the dbData
// connection.
func getdb(imdb bool) *sqlx.DB {
	if imdb {
		return dbImdb
	}
	return dbData
}

// queryGenericsT scans multiple rows from sqlx.Rows into a slice of any type T.
// It handles scanning into simple types as well as structs.
// For structs, it uses the getfunc mapping function to get the fields to scan into.
// Size is a hint for the initial slice capacity.
func queryGenericsT[T any](size int, rows *sqlx.Rows) []T {
	var result []T
	if size != 0 {
		result = make([]T, 0, size)
	}
	var u T
	d := getfunc2(&u)
	for rows.Next() {
		if d.simplescan {
			if rows.Scan(&u) == nil {
				result = append(result, u)
			}
		} else if d.structscan {
			if rows.StructScan(&u) == nil {
				result = append(result, u)
			}
		} else if rows.Scan(d.arr...) == nil {
			result = append(result, u)
		}
	}
	return result
}

type fieldconfig struct {
	simplescan bool
	structscan bool
	arr        []any
}

// getfunc2 determines how to scan a value of type any into a sqlx.Rows
// result. It returns a fieldconfig struct indicating whether the value is a
// simple type to scan directly, a struct to scan with StructScan, or a custom
// slice of fields to scan individually.
func getfunc2[T any](u *T) fieldconfig {
	var f fieldconfig
	switch elem := any(u).(type) {
	case *DbstaticOneIntOneBool:
		f.arr = []any{&elem.Num, &elem.Bl}
	case *dbstaticOneInt:
		f.arr = []any{&elem.Num}
	case *DbstaticOneStringOneInt:
		f.arr = []any{&elem.Str, &elem.Num}
	case *DbstaticOneStringOneUInt:
		f.arr = []any{&elem.Str, &elem.Num}
	case *DbstaticOneStringTwoInt:
		f.arr = []any{&elem.Str, &elem.Num1, &elem.Num2}
	case *DbstaticTwoStringOneInt:
		f.arr = []any{&elem.Str1, &elem.Str2, &elem.Num}
	case *DbstaticTwoInt:
		f.arr = []any{&elem.Num1, &elem.Num2}
	case *DbstaticTwoUint:
		f.arr = []any{&elem.Num1, &elem.Num2}
	case *DbstaticThreeString:
		f.arr = []any{&elem.Str1, &elem.Str2, &elem.Str3}
	case *dbstaticThreeStringOneInt:
		f.arr = []any{&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1}
	case *DbstaticThreeStringTwoInt:
		f.arr = []any{&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1, &elem.Num2}
	case *FilePrio:
		f.arr = []any{&elem.Location, &elem.DBID, &elem.ID, &elem.ResolutionID, &elem.QualityID, &elem.CodecID, &elem.AudioID, &elem.Proper, &elem.Repack, &elem.Extended}
	case *DbstaticTwoString:
		f.arr = []any{&elem.Str1, &elem.Str2}
	case *int, *string, *uint:
		f.simplescan = true
	default:
		f.structscan = true
	}
	return f
}

// structscan queries the database using the given query string and scans the
// result into the given struct pointer. It handles locking/unlocking the read
// write mutex, logging any errors, and returning sql.ErrNoRows if no rows were
// returned.
func structscan[T any](querystring string, imdb bool, u *T, id ...any) error {
	readWriteMu.RLock()
	err := GlobalCache.GetStmt(querystring, imdb, getdb(imdb)).QueryRowx(id...).StructScan(u)
	readWriteMu.RUnlock()
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
		if err.Error() == "sql: database is closed" {
			cache.items.Delete(querystring)
		}
	}
	return err
}

// structscanG queries the database using the given query string and scans the
// result into the given generic type T. It handles locking/unlocking the read
// write mutex, logging any errors, and returning sql.ErrNoRows if no rows were
// returned.
func structscanG[T any, R any](querystring string, imdb bool, id *R) (T, error) {
	var u T
	err := structscan(querystring, imdb, &u, id)
	return u, err
}

// GetDbmovieByIDP retrieves a Dbmovie by ID. It takes a uint ID and a
// pointer to a Dbmovie struct to scan the result into. It executes a SQL query
// using the structscan function to select the dbmovie data and scan it into
// the Dbmovie struct. Returns an error if there was a problem retrieving the data.
func GetDbmovieByIDP(id uint, u *Dbmovie) error {
	return structscan("select id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?", false, u, &id)
}

// GetDbmovieByID retrieves a Dbmovie by ID. It takes a uint ID
// and returns a Dbmovie struct and error.
// It executes a SQL query using the structscanG function to select the
// dbmovie data and scan it into the Dbmovie struct.
// Returns an error if there was a problem retrieving the data.
func GetDbmovieByID(id uint) (Dbmovie, error) {
	return structscanG[Dbmovie]("select id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?", false, &id)
}

// QueryDbmovie queries the dbmovies table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of Dbmovie structs.
func QueryDbmovie(qu Querywithargs, args ...any) []Dbmovie {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovies"
	qu.defaultcolumns = "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[Dbmovie](false, qu.Limit, qu.QueryString, args...)
}

// QueryDbmovieTitle queries the dbmovie_titles table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of DbmovieTitle structs.
func QueryDbmovieTitle(qu Querywithargs, args ...any) []DbmovieTitle {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "dbmovie_titles"
	qu.defaultcolumns = "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[DbmovieTitle](false, qu.Limit, qu.QueryString, args...)
}

// GetDbserieByIDP retrieves a Dbserie by ID. It takes a uint ID
// and a pointer to a Dbserie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// dbserie data and scan it into the Dbserie struct.
// Returns an error if there was a problem retrieving the data.
func GetDbserieByIDP(id uint, u *Dbserie) error {
	return structscan("select id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?", false, u, &id)
}

// GetDbserieByID retrieves a Dbserie by ID. It takes a uint ID
// and returns a Dbserie struct and error.
// It executes a SQL query using the structscanG function to select the
// dbserie data and scan it into the Dbserie struct.
// Returns an error if there was a problem retrieving the data.
func GetDbserieByID(id uint) (Dbserie, error) {
	return structscanG[Dbserie]("select id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?", false, &id)
}

// QueryDbserie queries the dbseries table using the provided Querywithargs struct and arguments.
// It sets the query size and limit, table name, default columns to select, builds the query if needed,
// and executes the query using QueryStaticArrayN, returning a slice of Dbserie structs.
func QueryDbserie(qu Querywithargs, args ...any) []Dbserie {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbseries"
	qu.defaultcolumns = "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[Dbserie](false, qu.Limit, qu.QueryString, args...)
}

// GetDbserieEpisodesByIDP retrieves a DbserieEpisode by ID. It takes a uint ID
// and a pointer to a DbserieEpisode struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// dbserie episode data and scan it into the DbserieEpisode struct.
// Returns an error if there was a problem retrieving the data.
func GetDbserieEpisodesByIDP(id uint, u *DbserieEpisode) error {
	return structscan("select id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id from dbserie_episodes where id = ?", false, u, &id)
}

// GetSerieByIDP retrieves a Serie by ID. It takes a uint ID
// and a pointer to a Serie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// serie data and scan it into the Serie struct.
// Returns an error if there was a problem retrieving the data.
func GetSerieByIDP(id uint, u *Serie) error {
	return structscan("select id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search from series where id = ?", false, u, &id)
}

// GetSerieEpisodesByIDP retrieves a SerieEpisode by ID. It takes a uint ID
// and a pointer to a SerieEpisode struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// serie episode data and scan it into the SerieEpisode struct.
// Returns an error if there was a problem retrieving the data.
func GetSerieEpisodesByIDP(id uint, u *SerieEpisode) error {
	return structscan("select id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id from serie_episodes where id = ?", false, u, &id)
}

// GetMoviesByIDP retrieves a Movie by ID. It takes a uint ID
// and a pointer to a Movie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// movie data and scan it into the Movie struct.
// Returns an error if there was a problem retrieving the data.
func GetMoviesByIDP(id uint, u *Movie) error {
	return structscan("select id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id from movies where id = ?", false, u, &id)
}

// QueryDbserieEpisodes queries the dbserie_episodes table based on the provided Querywithargs struct and arguments.
// It sets the query size limit from the Limit field if greater than 0.
// It sets the default columns to query.
// It builds the query string if not already set.
// It executes the query using QueryStaticArrayN to return a slice of DbserieEpisode structs.
func QueryDbserieEpisodes(qu Querywithargs, args ...any) []DbserieEpisode {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[DbserieEpisode](false, qu.Limit, qu.QueryString, args...)
}

// QueryDbserieAlternates queries the dbserie_alternates table based on the provided Querywithargs struct and arguments.
// It sets the query size limit from the Limit field if greater than 0.
// It sets the default columns to query.
// It builds the query string if not already set.
// It executes the query using QueryStaticArrayN to return a slice of DbserieAlternate structs.
func QueryDbserieAlternates(qu Querywithargs, args ...any) []DbserieAlternate {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "dbserie_alternates"
	qu.defaultcolumns = "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[DbserieAlternate](false, qu.Limit, qu.QueryString, args...)
}

// GetSeries retrieves a Serie struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a Serie struct.
// Returns the Serie struct and any error.
func GetSeries(qu Querywithargs, args ...any) (Serie, error) {
	qu.Table = logger.StrSeries
	qu.defaultcolumns = "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	var u Serie
	err := structscan(qu.QueryString, false, &u, args...)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, qu.QueryString))
	}
	return u, err
}

// GetSerieEpisodes retrieves a SerieEpisode struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a SerieEpisode struct.
// Returns a SerieEpisode struct and any error.
func GetSerieEpisodes(qu Querywithargs, args ...any) (SerieEpisode, error) {
	qu.Table = "serie_episodes"
	qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	var u SerieEpisode
	err := structscan(qu.QueryString, false, &u, args...)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, qu.QueryString))
	}
	return u, err
}

// QuerySerieEpisodes retrieves all SerieEpisode records for the given series listname.
// It takes a pointer to a string containing the listname to search for.
// It returns a slice of SerieEpisode structs matching the listname.
func QuerySerieEpisodes(arg *string) []SerieEpisode {
	if arg == nil {
		return nil
	}

	return Getrows1size[SerieEpisode](false, "select count() from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", "select id, quality_reached, quality_profile from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", arg)
}

// GetMovies retrieves a Movie struct based on the provided Querywithargs.
// It sets the query table and columns.
// It builds the query if not already set.
// It executes the query and scans the result into a Movie struct.
// Returns the Movie struct and any error.
func GetMovies(qu Querywithargs, args ...any) (Movie, error) {
	qu.Table = "movies"
	qu.defaultcolumns = "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	var u Movie
	err := structscan(qu.QueryString, false, &u, args...)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, qu.QueryString))
	}
	return u, err
}

// QueryMovies retrieves all Movie records matching the given listname.
// It takes a string containing the listname to search for.
// It returns a slice of Movie structs matching the listname.
func QueryMovies(arg string) []Movie {
	if arg == "" {
		return nil
	}
	return Getrows1size[Movie](false, "select count() from movies where listname = ? COLLATE NOCASE", "select id, quality_reached, quality_profile from movies where listname = ? COLLATE NOCASE", &arg)
}

// QueryJobHistory retrieves JobHistory records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of JobHistory structs matching the query.
func QueryJobHistory(qu Querywithargs, args ...any) []JobHistory {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "job_histories"
	qu.defaultcolumns = "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[JobHistory](false, qu.Limit, qu.QueryString, args...)
}

// QuerySerieFileUnmatched retrieves SerieFileUnmatched records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of SerieFileUnmatched structs matching the query.
func QuerySerieFileUnmatched(qu Querywithargs, args ...any) []SerieFileUnmatched {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[SerieFileUnmatched](false, qu.Limit, qu.QueryString, args...)
}

// QueryMovieFileUnmatched retrieves MovieFileUnmatched records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of MovieFileUnmatched structs matching the query.
func QueryMovieFileUnmatched(qu Querywithargs, args ...any) []MovieFileUnmatched {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "movie_file_unmatcheds"
	qu.defaultcolumns = "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[MovieFileUnmatched](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultMovies retrieves ResultMovies records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultMovies structs matching the query.
func QueryResultMovies(qu Querywithargs, args ...any) []ResultMovies {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = "movies"
	qu.defaultcolumns = `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[ResultMovies](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultSeries retrieves ResultSeries records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultSeries structs matching the query.
func QueryResultSeries(qu Querywithargs, args ...any) []ResultSeries {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}
	qu.Table = logger.StrSeries
	qu.defaultcolumns = `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[ResultSeries](false, qu.Limit, qu.QueryString, args...)
}

// QueryResultSerieEpisodes retrieves ResultSerieEpisodes records matching the query arguments.
// It takes a Querywithargs struct to define the query parameters.
// It returns a slice of ResultSerieEpisodes structs matching the query.
func QueryResultSerieEpisodes(qu Querywithargs, args ...any) []ResultSerieEpisodes {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "serie_episodes"
	qu.defaultcolumns = `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[ResultSerieEpisodes](false, qu.Limit, qu.QueryString, args...)
}

// GetImdbRating queries the imdb_ratings table to get the average rating and number of votes for the given IMDb ID.
// It populates the rating fields on the Dbmovie struct if they are empty or overwrite is true.
func GetImdbRating(arg *string, movie *Dbmovie, overwrite bool) {
	if arg == nil {
		return
	}
	imdbratedata, err := structscanG[ImdbRatings]("select * from imdb_ratings where tconst = ?", true, arg)
	if err == nil {
		if (movie.VoteAverage == 0 || overwrite) && imdbratedata.AverageRating != 0 {
			movie.VoteAverage = imdbratedata.AverageRating
		}
		if (movie.VoteCount == 0 || overwrite) && imdbratedata.NumVotes != 0 {
			movie.VoteCount = imdbratedata.NumVotes
		}
		imdbratedata.Close()
	}
}

// QueryImdbAka queries the imdb_akas table to get alternate titles and regional releases for the given IMDb ID.
// It takes a Querywithargs for pagination and filtering, and a pointer to the IMDb ID to query on.
// It returns a slice of ImdbAka structs containing the alternate title data.
func QueryImdbAka[R any](qu Querywithargs, arg *R) []imdbAka {
	qu.size = -1
	if qu.Limit >= 1 {
		qu.size = qu.Limit
	}

	qu.Table = "imdb_akas"
	qu.defaultcolumns = "id,created_at,updated_at,tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.QueryString == "" {
		qu.Buildquery(false)
	}
	return GetrowsN[imdbAka](true, qu.Limit, qu.QueryString, arg)
}

// GetImdbTitle queries the imdb_titles table to populate movie details from IMDb.
// It takes the IMDb id pointer, movie struct pointer, and a boolean overwrite flag.
// It will populate the movie struct with data from IMDb if fields are empty or overwrite is true.
// This handles setting the title, year, adult flag, genres, original title, runtime, slug, url,
// vote average, and vote count.
func GetImdbTitle(arg *string, movie *Dbmovie, overwrite bool) {
	if arg == nil {
		return
	}

	imdbdata, err := structscanG[ImdbTitle]("select * from imdb_titles where tconst = ?", true, arg)
	if err != nil {
		return
	}

	if (movie.Title == "" || overwrite) && imdbdata.PrimaryTitle != "" {
		movie.Title = imdbdata.PrimaryTitle
	}
	if (movie.Year == 0 || overwrite) && imdbdata.StartYear != 0 {
		movie.Year = imdbdata.StartYear
	}
	if (!movie.Adult && imdbdata.IsAdult) || overwrite {
		movie.Adult = imdbdata.IsAdult
	}
	if (movie.Genres == "" || overwrite) && imdbdata.Genres != "" {
		movie.Genres = imdbdata.Genres
	}
	if (movie.OriginalTitle == "" || overwrite) && imdbdata.OriginalTitle != "" {
		movie.OriginalTitle = imdbdata.OriginalTitle
	}
	if (movie.Runtime == 0 || movie.Runtime == 1 || movie.Runtime == 2 || movie.Runtime == 3 || movie.Runtime == 60 || movie.Runtime == 90 || movie.Runtime == 120 || overwrite) && imdbdata.RuntimeMinutes != 0 {
		if movie.Runtime != 0 && (imdbdata.RuntimeMinutes == 1 || imdbdata.RuntimeMinutes == 2 || imdbdata.RuntimeMinutes == 3 || imdbdata.RuntimeMinutes == 60 || imdbdata.RuntimeMinutes == 90 || imdbdata.RuntimeMinutes == 120) {
			logger.LogDynamic("debug", "skipped imdb movie runtime for", logger.NewLogField(logger.StrImdb, movie.ImdbID))
		} else {
			logger.LogDynamic("debug", "set imdb movie runtime for", logger.NewLogField(logger.StrImdb, movie.ImdbID))
			movie.Runtime = imdbdata.RuntimeMinutes
		}
	}
	if (movie.Slug == "" || overwrite) && imdbdata.Slug != "" {
		movie.Slug = imdbdata.Slug
	}
	if movie.URL == "" || overwrite {
		movie.URL = logger.URLJoinPath("https://www.imdb.com/title/", movie.ImdbID)
	}

	GetImdbRating(&movie.ImdbID, movie, overwrite)
	imdbdata.Close()
}

// Buildquery constructs the SQL query string from the Querywithargs fields.
// It handles adding the SELECT columns, FROM table, JOINs, WHERE, ORDER BY
// and LIMIT clauses based on the configured fields.
func (qu *Querywithargs) Buildquery(count bool) {
	i := len(qu.Table) + len(qu.Where) + len(qu.InnerJoin) + len(qu.OrderBy) + 50
	if !count {
		if len(qu.Select) >= 1 {
			i += len(qu.Select)
		} else {
			i += len(qu.defaultcolumns)
		}
	}
	bld := logger.PlBuffer.Get()
	bld.Grow(i)
	//defer bld.Reset()
	bld.WriteString(sel)
	if len(qu.Select) >= 1 {
		bld.WriteString(qu.Select)
	} else {
		if logger.ContainsI(qu.defaultcolumns, qu.Table+".") {
			bld.WriteString(qu.defaultcolumns)
		} else {
			if count {
				bld.WriteString(coun)
			} else {
				if qu.InnerJoin != "" {
					bld.WriteString(qu.Table)
					bld.WriteString(all)
				} else {
					bld.WriteString(qu.defaultcolumns)
				}
			}
		}
	}
	bld.WriteString(from)
	bld.WriteString(qu.Table)
	if qu.InnerJoin != "" {
		bld.WriteString(join)
		bld.WriteString(qu.InnerJoin)
	}
	if qu.Where != "" {
		bld.WriteString(where)
		bld.WriteString(qu.Where)
	}
	if qu.OrderBy != "" {
		bld.WriteString(order)
		bld.WriteString(qu.OrderBy)
	}
	if qu.Limit != 0 {
		bld.WriteString(limit)
		if qu.Offset != 0 {
			logger.BuilderAddInt(bld, qu.Offset)
			bld.WriteRune(',')
		}
		logger.BuilderAddInt(bld, qu.Limit)
		//bld.WriteString(strconv.Itoa(qu.Limit))
	}
	qu.QueryString = bld.String()
	logger.PlBuffer.Put(bld)
}

// ScanrowsNdyn scans a single row into a pointer to a struct,
// setting fields of the struct to zero values if sql.ErrNoRows is returned.
// It takes a bool indicating if the query is for the imdb database,
// the query string, a pointer to the struct to scan into,
// and optional variadic arguments.
// It returns any error from the query.
func ScanrowsNdyn[R any](imdb bool, querystring string, obj *R, args ...any) error {
	readWriteMu.RLock()
	err := GlobalCache.GetStmt(querystring, imdb, getdb(imdb)).QueryRow(args...).Scan(obj)
	readWriteMu.RUnlock()
	if err != nil {
		switch val := any(obj).(type) {
		case *int:
			if *val != 0 {
				*val = 0
			}
		case *uint:
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
		if !errors.Is(err, sql.ErrNoRows) {
			logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
		}
		if err.Error() == "sql: database is closed" {
			cache.items.Delete(querystring)
		}
	}
	return err
}

// DBLock locks the database for write access by calling Lock on readWriteMu.
func DBLock() {
	readWriteMu.Lock()
}

// DBUnlock unlocks the database by calling Unlock on readWriteMu, which releases the write lock.
func DBUnlock() {
	readWriteMu.Unlock()
}

// getdatarowN executes the given querystring with multiple arguments and scans the result into obj,
// handling locking, logging errors, and returning the scanned object.
func GetdatarowN[O any](imdb bool, querystring string, args ...any) O {
	var obj O
	_ = ScanrowsNdyn(imdb, querystring, &obj, args...)
	return obj
}

// GetdatarowArgs executes the given querystring with the provided argument
// and scans the result into the given slice of objects, handling locking,
// logging errors, and returning the scanned objects.
func GetdatarowArgs[T any](querystring string, arg *T, objs ...any) {
	readWriteMu.RLock()
	err := GlobalCache.GetStmt(querystring, false, dbData).QueryRow(arg).Scan(objs...)
	readWriteMu.RUnlock()
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
	}
	if err != nil && err.Error() == "sql: database is closed" {
		cache.items.Delete(querystring)
	}

}

// getrows1 executes the given querystring with one argument, scans the result rows into
// a slice of the generic type T, handles locking, logging errors, and returns the slice.
// The size parameter limits the number of rows scanned.
func Getrows1size[T any, R any](imdb bool, sizeq string, querystring string, arg *R) []T {
	return GetrowsN[T](imdb, GetdatarowN[int](imdb, sizeq, arg), querystring, arg)
}

// getrowsN executes the given querystring with multiple arguments, scans the result
// rows into a slice of the generic type T, handles locking, logging errors,
// and returns the slice. The size parameter limits the number of rows scanned.
func GetrowsN[T any](imdb bool, size int, querystring string, args ...any) []T {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := GlobalCache.GetStmt(querystring, imdb, getdb(imdb)).Queryx(args...)
	if err == nil {
		defer rows.Close()
		return queryGenericsT[T](size, rows)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
	}
	if err.Error() == "sql: database is closed" {
		cache.items.Delete(querystring)
	}
	return nil
}

// getrowsNuncached executes the given querystring with multiple arguments against the uncached database connection, scans the result
// rows into a slice of the generic type T, handles locking, logging errors,
// and returns the slice. The size parameter limits the number of rows scanned.
func GetrowsNuncached[T any](size int, querystring string, args []any) []T {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := dbData.Queryx(querystring, args...)
	if err == nil {
		defer rows.Close()
		return queryGenericsT[T](size, rows)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
	}
	return nil
}

// QueryDBEpisodeID retrieves the database episode ID for the given series ID, season number, and episode number.
// It handles locking, error logging, and zeroing the output ID on error.
func QueryDBEpisodeID(dbserie *uint, season *int, episode any, outid *uint) {
	ScanrowsNdyn(false, "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?", outid, dbserie, season, episode)
}

// SetDBEpisodeIDByIdentifier sets the database episode ID for the given series ID and identifier.
// It handles nil checks, retries with normalized identifiers, error handling, and zeroing the ID on error.
func SetDBEpisodeIDByIdentifier(dbepiid *uint, dbserieid *uint, identifier *string) {
	if identifier == nil || *identifier == "" {
		return
	}
	if dbserieid == nil || *dbserieid == 0 {
		return
	}
	err := ScanrowsNdyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier, dbepiid, dbserieid, identifier)
	if err != nil && strings.ContainsRune(*identifier, '.') {
		err = ScanrowsNdyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier2, dbepiid, dbserieid, identifier, ".", "-")
	}
	if err == nil {
		return
	}
	if strings.ContainsRune(*identifier, ' ') {
		ScanrowsNdyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier2, dbepiid, dbserieid, identifier, " ", "-")
	}
}

// QueryImdbAkaCountByTitleSlug executes a query against the imdb database to get the number of aka title records matching the given title or slug parameters. Returns 0 if either parameter is nil. The title and slug values are matched case insensitively.
func QueryImdbAkaCountByTitleSlug(arg *string, arg2 *string) int {
	if arg == nil || arg2 == nil {
		return 0
	}
	return GetdatarowN[int](true, "select count() from (select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?)", arg, arg2)
}

// execN executes a SQL statement with multiple arguments and returns the sql.Result and error.
// It locks access to the database during the query, logs any errors, and handles error cases.
func ExecN(querystring string, args ...any) (sql.Result, error) {
	readWriteMu.Lock()
	result, err := GlobalCache.GetStmt(querystring, false, dbData).Exec(args...)
	readWriteMu.Unlock()
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.LogDynamic("error", "exec", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, querystring))
		}
		if err.Error() == "sql: database is closed" {
			cache.items.Delete(querystring)
		}

		return nil, err
	}
	return result, nil
}

// ExecNid executes the given querystring with multiple arguments, returns the generated ID from the insert statement, handles errors.
func ExecNid(querystring string, args ...any) (int64, error) {
	dbresult, err := ExecN(querystring, args...)
	if err != nil {
		return 0, err
	}
	newid, err := dbresult.LastInsertId()
	if err != nil {
		logger.LogDynamic("error", "query insert", logger.NewLogFieldValue(err), logger.NewLogField("query", querystring))
		return 0, err
	}
	return newid, nil
}

// InsertArray inserts a row into the given database table, with the provided columns and values.
// The number of columns must match the number of value parameters.
// It handles building the SQL insert statement from the parameters, executing the insert,
// and returning the result or any error.
func InsertArray(table string, columns []string, values ...any) (sql.Result, error) {
	if len(columns) != len(values) {
		return nil, errors.New("wrong number of columns")
	}
	return ExecN("insert into "+table+" ("+strings.Join(columns, ",")+") values (?"+strings.Repeat(",?", len(columns)-1)+")", values...)
}

// UpdateArray updates rows in the given database table by setting the provided
// columns to the corresponding value parameters. It builds the SQL UPDATE
// statement dynamically based on the parameters. The optional where parameter
// allows specifying a WHERE clause to filter the rows to update. It handles
// executing the statement and returning the result or any error.
func UpdateArray(table string, columns []string, where string, args ...any) (sql.Result, error) {
	bld := logger.PlBuffer.Get()
	i := 12 + len(table)
	i += logger.Getstringarrlength(columns)
	i += len(columns)
	if where != "" {
		i += len(where) + 7
	}
	bld.Grow(i)
	bld.WriteString("update ")
	bld.WriteString(table)
	bld.WriteString(" set ")
	for idx := range columns {
		if idx != 0 {
			bld.WriteRune(',')
		}
		bld.WriteString(columns[idx])
		bld.WriteString(" = ?")
	}
	if where != "" {
		bld.WriteString(" where ")
		bld.WriteString(where)
	}
	defer logger.PlBuffer.Put(bld)
	return ExecN(bld.String(), args...)
}

// DeleteRow deletes rows from the given database table that match the provided
// WHERE clause and arguments. It returns the sql.Result and error from the
// query execution. The table parameter specifies the table name to delete from.
// The where parameter allows specifying a WHERE condition to filter the rows
// to delete. The args parameters allow providing arguments to replace any ?
// placeholders in the where condition.
func DeleteRow(table string, where string, args ...any) (sql.Result, error) {
	var querystring = "delete from " + table
	if where != "" {
		querystring = querystring + " where " + where
	}
	if DBLogLevel == logger.StrDebug {
		logger.LogDynamic("debug", "query count", logger.NewLogField(logger.StrQuery, querystring), logger.NewLogField("args", args))
	}
	return ExecN(querystring, args...)
}

// queryrowfulllockconnect executes the given SQL query while holding a write lock
// on the database. It scans the result into str and returns any error.
func queryrowfulllockconnect(query string) string {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	tempdb, err := sql.Open("sqlite3", "file:./databases/data.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		return ""
	}
	defer tempdb.Close()
	var str string
	err = tempdb.QueryRow(query).Scan(&str)
	if err == nil {
		return str
	}
	if !errors.Is(err, sql.ErrNoRows) {
		logger.LogDynamic("error", "select", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrQuery, query))
	}
	return ""
}

// DBQuickCheck checks the database for errors using the
// PRAGMA quick_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBQuickCheck() string {
	logger.LogDynamic("info", "Check Database for Errors")
	str := queryrowfulllockconnect("PRAGMA quick_check;")
	logger.LogDynamic("info", "Check Database for Errors finished")
	return str
}

// DBIntegrityCheck checks the database integrity using the
// PRAGMA integrity_check statement. It logs informational
// messages before and after running the statement.
// The string result from running the statement is
// returned.
func DBIntegrityCheck() string {
	logger.LogDynamic("info", "Check Database for Integrity")
	str := queryrowfulllockconnect("PRAGMA integrity_check;")
	logger.LogDynamic("info", "Check Database for Integrity finished")
	return str
}

// getentryalternatetitlesdirect queries the database to get alternate titles for the given media entry ID.
// It returns a slice of DbstaticTwoString structs containing the alternate titles and slugs.
// If useseries is true, it will query the series alternates table, otherwise it queries the movies titles table.
// It first checks the cache if enabled, otherwise queries the DB directly.
func Getentryalternatetitlesdirect(dbid uint, useseries bool) []DbstaticTwoStringOneInt {
	if config.SettingsGeneral.UseMediaCache {
		a := GetCachedObj[CacheTwoStringIntExpire](logger.GetStringsMap(useseries, logger.CacheMediaTitles))
		b := a.Arr[:0]
		intid := int(dbid)
		for idx := range a.Arr {
			if a.Arr[idx].Num != intid {
				continue
			}
			b = append(b, a.Arr[idx])
		}
		return b
	}
	return Getrows1size[DbstaticTwoStringOneInt](false, logger.GetStringsMap(useseries, logger.DBCountDBTitlesDBID), logger.GetStringsMap(useseries, logger.DBDistinctDBTitlesDBID), &dbid)
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
	dbImdb.Close()

	_ = os.Chmod(dbfile, 0777)
	os.Remove(dbfile)
	err := os.Rename(dbfiletemp, dbfile)
	if err == nil {
		logger.LogDynamic("debug", "File renamed", logger.NewLogFieldValue(dbfiletemp))
	}
	InitImdbdb()
	readWriteMu.Unlock()
}
