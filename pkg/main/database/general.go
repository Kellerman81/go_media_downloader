package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jmoiron/sqlx"

	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" //Needed for Migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"      //Needed for Migrate
	_ "github.com/mattn/go-sqlite3"                           //Needed for DB
)

type DBGlobal struct {
	AudioStrIn       []string
	CodecStrIn       []string
	QualityStrIn     []string
	ResolutionStrIn  []string
	GetqualitiesIn   []QualitiesRegex
	GetresolutionsIn []QualitiesRegex
	GetcodecsIn      []QualitiesRegex
	GetaudiosIn      []QualitiesRegex
}
type JobHistory struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	JobType     string    `db:"job_type"`
	JobCategory string    `db:"job_category"`
	JobGroup    string    `db:"job_group"`
	Started     sql.NullTime
	Ended       sql.NullTime
}

type JobHistoryJSON struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	JobType     string    `db:"job_type"`
	JobCategory string    `db:"job_category"`
	JobGroup    string    `db:"job_group"`
	Started     time.Time
	Ended       time.Time
}
type RSSHistory struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Config    string
	List      string
	Indexer   string
	LastID    string `db:"last_id"`
}
type IndexerFail struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Indexer   string
	LastFail  sql.NullTime `db:"last_fail"`
}

type QualitiesRegex struct {
	Regex string
	Qualities
}

type backupInfo struct {
	timestamp time.Time
	path      string
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []backupInfo

var (
	CacheFilesMovie               []string
	CacheFilesMovieExpire         time.Time
	CacheFilesSeries              []string
	CacheFilesSeriesExpire        time.Time
	CacheUnmatchedMovie           []string
	CacheUnmatchedMovieExpire     time.Time
	CacheUnmatchedSeries          []string
	CacheUnmatchedSeriesExpire    time.Time
	CacheHistoryTitleMovie        []string
	CacheHistoryTitleMovieExpire  time.Time
	CacheHistoryUrlMovie          []string
	CacheHistoryUrlMovieExpire    time.Time
	CacheHistoryTitleSeries       []string
	CacheHistoryTitleSeriesExpire time.Time
	CacheHistoryUrlSeries         []string
	CacheHistoryUrlSeriesExpire   time.Time
	CacheTitlesSeries             []DbstaticTwoStringOneInt
	CacheTitlesSeriesExpire       time.Time
	CacheTitlesMovie              []DbstaticTwoStringOneInt
	CacheTitlesMovieExpire        time.Time
	CacheMovie                    []DbstaticOneStringOneInt
	CacheMovieExpire              time.Time
	CacheDBMovie                  []DbstaticThreeStringOneInt
	CacheDBMovieExpire            time.Time

	readWriteMu = &sync.RWMutex{}
	DBConnect   DBGlobal
	dbData      *sqlx.DB
	dbImdb      *sqlx.DB
	DBVersion   = "1"
	DBLogLevel  = "Info"
)

func Checkcacheexpire(tim time.Time) bool {
	now := time.Now()
	return tim.After(now)
}

func DBClose() {
	if dbData != nil {
		dbData.Close()
	}
	if dbImdb != nil {
		dbImdb.Close()
	}
}

func InitDB(dbloglevel string) {
	if _, err := os.Stat("./databases/data.db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/data.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	var err error
	dbData, err = sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	dbData.SetMaxIdleConns(15)
	dbData.SetMaxOpenConns(5)
	DBLogLevel = strings.ToLower(dbloglevel)
}

func CloseImdb() {
	if dbImdb != nil {
		dbImdb.Close()
	}
}
func GetVersion() string {
	return DBVersion
}
func SetVersion(str string) {
	DBVersion = str
}

func InitImdbdb() {
	var err error
	if _, err = os.Stat("./databases/imdb.db"); os.IsNotExist(err) {
		_, err = os.Create("./databases/imdb.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	dbImdb, err = sqlx.Connect("sqlite3", "file:./databases/imdb.db?_fk=1&mode=rwc&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	dbImdb.SetMaxIdleConns(15)
	dbImdb.SetMaxOpenConns(5)
}

func GetVars() {
	quali := QueryQualities("select * from qualities where type=1 order by priority desc")
	logger.Log.Info().Msg("Database Get Variables 1")
	DBConnect.GetresolutionsIn = make([]QualitiesRegex, len(*quali))
	for idx := range *quali {
		(*quali)[idx].StringsLower = strings.ToLower((*quali)[idx].Strings)
		logger.GlobalCacheRegex.SetRegexp(&(*quali)[idx].Regex, 0)
		DBConnect.GetresolutionsIn[idx] = QualitiesRegex{Regex: (*quali)[idx].Regex, Qualities: (*quali)[idx]}
	}

	quali = QueryQualities("select * from qualities where type=2 order by priority desc")
	logger.Log.Info().Msg("Database Get Variables 2")
	DBConnect.GetqualitiesIn = make([]QualitiesRegex, len(*quali))
	for idx := range *quali {
		(*quali)[idx].StringsLower = strings.ToLower((*quali)[idx].Strings)
		logger.GlobalCacheRegex.SetRegexp(&(*quali)[idx].Regex, 0)
		DBConnect.GetqualitiesIn[idx] = QualitiesRegex{Regex: (*quali)[idx].Regex, Qualities: (*quali)[idx]}
	}

	quali = QueryQualities("select * from qualities where type=3 order by priority desc")
	logger.Log.Info().Msg("Database Get Variables 3")
	DBConnect.GetcodecsIn = make([]QualitiesRegex, len(*quali))
	for idx := range *quali {
		(*quali)[idx].StringsLower = strings.ToLower((*quali)[idx].Strings)
		logger.GlobalCacheRegex.SetRegexp(&(*quali)[idx].Regex, 0)
		DBConnect.GetcodecsIn[idx] = QualitiesRegex{Regex: (*quali)[idx].Regex, Qualities: (*quali)[idx]}
	}

	quali = QueryQualities("select * from qualities where type=4 order by priority desc")
	logger.Log.Info().Msg("Database Get Variables 4")
	DBConnect.GetaudiosIn = make([]QualitiesRegex, len(*quali))
	for idx := range *quali {
		(*quali)[idx].StringsLower = strings.ToLower((*quali)[idx].Strings)
		logger.GlobalCacheRegex.SetRegexp(&(*quali)[idx].Regex, 0)
		DBConnect.GetaudiosIn[idx] = QualitiesRegex{Regex: (*quali)[idx].Regex, Qualities: (*quali)[idx]}
	}
	logger.Clear(quali)

	for idx := range config.SettingsRegex {
		for idxreg := range config.SettingsRegex[idx].Rejected {
			if !logger.GlobalCacheRegex.CheckNoType(&config.SettingsRegex[idx].Rejected[idxreg]) {
				logger.GlobalCacheRegex.SetRegexp(&config.SettingsRegex[idx].Rejected[idxreg], 0)
			}
		}
		for idxreg := range config.SettingsRegex[idx].Required {
			if !logger.GlobalCacheRegex.CheckNoType(&config.SettingsRegex[idx].Required[idxreg]) {
				logger.GlobalCacheRegex.SetRegexp(&config.SettingsRegex[idx].Required[idxreg], 0)
			}
		}
	}

	//logger.Grow(&DBConnect.AudioStrIn, len(DBConnect.GetaudiosIn)*2)
	for idx := range DBConnect.GetaudiosIn {
		DBConnect.AudioStrIn = append(DBConnect.AudioStrIn, strings.Split(DBConnect.GetaudiosIn[idx].StringsLower, ",")...)
	}
	//logger.Grow(&DBConnect.CodecStrIn, len(DBConnect.GetcodecsIn)*2)
	for idx := range DBConnect.GetcodecsIn {
		DBConnect.CodecStrIn = append(DBConnect.CodecStrIn, strings.Split(DBConnect.GetcodecsIn[idx].StringsLower, ",")...)
	}
	//logger.Grow(&DBConnect.QualityStrIn, len(DBConnect.GetqualitiesIn)*7)
	for idx := range DBConnect.GetqualitiesIn {
		DBConnect.QualityStrIn = append(DBConnect.QualityStrIn, strings.Split(DBConnect.GetqualitiesIn[idx].StringsLower, ",")...)
	}
	//logger.Grow(&DBConnect.ResolutionStrIn, len(DBConnect.GetresolutionsIn)*4)
	for idx := range DBConnect.GetresolutionsIn {
		DBConnect.ResolutionStrIn = append(DBConnect.ResolutionStrIn, strings.Split(DBConnect.GetresolutionsIn[idx].StringsLower, ",")...)
	}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(backupPath string, maxbackups int) error {
	query := "VACUUM INTO ?"
	_, err := execsql(true, false, &query, &[]interface{}{backupPath})
	if err != nil {
		return errors.New("vacuum failed: " + err.Error())
	}
	return RemoveOldDBBackups(maxbackups)
}

func UpgradeDB() {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://./databases/data.db?_fk=1&_cslike=0",
	)

	vers, _, _ := m.Version()
	DBVersion = logger.IntToString(int(vers))
	if err != nil {
		log.Fatalf("migration failed... %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database.. %v", err)
	}
}
func UpgradeIMDB() {
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://./databases/imdb.db?_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		fmt.Println(fmt.Errorf("migration failed... %v", err))
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		fmt.Println(fmt.Errorf("an error occurred while syncing the database.. %v", err))
	}
}

func RemoveOldDBBackups(max int) error {
	if max == 0 {
		return nil
	}

	prefix := "data.db."
	files, err := oldDatabaseFiles(prefix)
	if err != nil {
		return err
	}

	if max == 0 || max >= len(*files) {
		logger.Clear(files)
		return err
	}

	i := 0
	for idx := range *files {
		// Only count the uncompressed log file or the
		// compressed log file, not both.
		if !strings.HasPrefix((*files)[idx].path, prefix) {
			//if len(files[idx].path) >= len(prefix) && files[idx].path[:len(prefix)] != prefix {
			continue
		}

		i++

		if i > max {
			errRemove := os.Remove(logger.PathJoin("./backup", (*files)[idx].path))
			if err == nil && errRemove != nil {
				err = errRemove
			}
		}
	}
	logger.Clear(files)

	return err
}

func oldDatabaseFiles(prefix string) (*[]backupInfo, error) {
	files, err := os.ReadDir("./backup")
	if err != nil {
		return nil, errors.New("can't read log file directory: " + err.Error())
	}
	if len(files) == 0 {
		return &[]backupInfo{}, nil
	}
	var backupFiles = make([]backupInfo, 0, len(files))

	var fn string
	for idx := range files {
		if files[idx].IsDir() {
			continue
		}
		fn = files[idx].Name()
		if !logger.HasPrefixI(fn, prefix) {
			//if len(files[idx].Name()) >= len(prefix) && files[idx].Name()[:len(prefix)] != prefix {
			continue
		}
		if t, err := timeFromName(fn, prefix); err == nil {
			backupFiles = append(backupFiles, backupInfo{t, files[idx].Name()})
			continue
		}
	}
	logger.Clear(&files)

	sort.Sort(byFormatTime(backupFiles))

	return &backupFiles, nil
}

func timeFromName(filename, prefix string) (time.Time, error) {
	if !logger.HasPrefixI(filename, prefix) {
		//if len(filename) >= len(prefix) && filename[:len(prefix)] != prefix {
		return time.Time{}, logger.ErrWrongPrefix
	}
	ts := filename[len(prefix):]
	if idx := strings.Index(ts, "."); idx != -1 {
		ts = ts[idx+1:]
	}
	return time.Parse("20060102_150405", ts)
}

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}

func SQLTimeGetNow() *sql.NullTime {
	return TimeToSQLTime(time.Now().In(&logger.TimeZone), true)
}
func TimeToSQLTime(time time.Time, valid bool) *sql.NullTime {
	return &sql.NullTime{Time: time, Valid: valid}
}

func ParseDate(date string) *sql.NullTime {
	if date == "" {
		return &sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02", date)
	return TimeToSQLTime(t, err == nil)
}

// func SetStringsCache(cachekey string, countsql string, querysql string, dur time.Duration, queryargs ...interface{}) {
// 	if !logger.GlobalCache.CheckNoType(cachekey) {
// 		tbl := QueryStaticStringArray(false,
// 			QueryIntColumn(countsql, queryargs...),
// 			querysql, queryargs...)
// 		cache.SetTable(logger.GlobalCache, cachekey, tbl, dur)
// 	}
// }

// func SetOneStringOneIntCache(cachekey string, countsql string, querysql string, dur time.Duration) {
// 	if !logger.GlobalCache.CheckNoType(cachekey) {
// 		tbl := queryGenericsT(true, false, QueryIntColumn(countsql), func(elem *DbstaticOneStringOneInt) []interface{} {
// 			return []interface{}{&elem.Str, &elem.Num}
// 		}, querysql, nil)
// 		cache.SetTable(logger.GlobalCache, cachekey, tbl, dur)
// 	}
// }

// func SetTwoStringOneIntCache(cachekey string, countsql string, querysql string, dur time.Duration) {
// 	if !logger.GlobalCache.CheckNoType(cachekey) {
// 		tbl := queryGenericsT(true, false, QueryIntColumn(countsql), func(elem *DbstaticTwoStringOneInt) []interface{} {
// 			return []interface{}{&elem.Str1, &elem.Str2, &elem.Num}
// 		}, querysql, nil)
// 		cache.SetTable(logger.GlobalCache, cachekey, tbl, dur)
// 	}
// }

// func SetThreeStringOneIntCache(cachekey string, countsql string, querysql string, dur time.Duration) {
// 	if !logger.GlobalCache.CheckNoType(cachekey) {
// 		tbl := queryGenericsT(true, false, QueryIntColumn(countsql), func(elem *DbstaticThreeStringOneInt) []interface{} {
// 			return []interface{}{&elem.Str1, &elem.Str2, &elem.Str3, &elem.Num1}
// 		}, querysql, nil)
// 		cache.SetTable(logger.GlobalCache, cachekey, tbl, dur)
// 	}
// }

func RefreshMoviesCache() {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	// if logger.GlobalCache.CheckNoType("dbmovies_cached") && logger.GlobalCache.CheckNoType("movies_cached") {
	// 	return
	// }
	// SetThreeStringOneIntCache("dbmovies_cached", "select count(*) from dbmovies", "select title, slug, imdb_id, id from dbmovies", 8*time.Hour)
	// SetOneStringOneIntCache("movies_cached", QueryMoviesCount, "select lower(listname), dbmovie_id from movies", 8*time.Hour)

	if Checkcacheexpire(CacheDBMovieExpire) && Checkcacheexpire(CacheMovieExpire) {
		return
	}
	CacheDBMovie = *QueryStaticColumnsThreeStringOneInt(false, QueryCountColumn("dbmovies", ""), "select title, slug, imdb_id, id from dbmovies")
	CacheDBMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)

	CacheMovie = *QueryStaticColumnsOneStringOneInt(false, QueryCountColumn("movies", ""), "select lower(listname), dbmovie_id from movies")
	CacheMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
}

func Refreshmoviestitlecache() {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	// if logger.GlobalCache.CheckNoType("dbmovietitles_title_slug_cache") {
	// 	return
	// }
	// SetTwoStringOneIntCache("dbmovietitles_title_slug_cache", "select count(*) from dbmovie_titles", "select title, slug, dbmovie_id from dbmovie_titles", 8*time.Hour)

	if Checkcacheexpire(CacheTitlesMovieExpire) {
		return
	}
	CacheTitlesMovie = *QueryStaticColumnsTwoStringOneInt(false,
		QueryIntColumn("select count(*) from dbmovie_titles"),
		"select title, slug, dbmovie_id from dbmovie_titles")
	CacheTitlesMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
}
func Refreshhistorycache(str string) {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	if logger.HasPrefixI(str, logger.StrMovie) && Checkcacheexpire(CacheHistoryUrlMovieExpire) && Checkcacheexpire(CacheHistoryTitleMovieExpire) {
		return
	}
	if !logger.HasPrefixI(str, logger.StrMovie) && Checkcacheexpire(CacheHistoryUrlSeriesExpire) && Checkcacheexpire(CacheHistoryTitleSeriesExpire) {
		return
	}

	if logger.HasPrefixI(str, logger.StrSerie) {
		CacheHistoryUrlSeries = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from serie_episode_histories"),
			"select distinct url from serie_episode_histories")
		CacheHistoryTitleSeries = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from serie_episode_histories"),
			"select distinct title from serie_episode_histories")
		CacheHistoryUrlSeriesExpire = logger.TimeGetNow().Add(time.Hour * -12)
		CacheHistoryTitleSeriesExpire = logger.TimeGetNow().Add(time.Hour * -12)
	} else {
		CacheHistoryUrlMovie = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from movie_histories"),
			"select distinct url from movie_histories")
		CacheHistoryTitleMovie = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from movie_histories"),
			"select distinct title from movie_histories")
		CacheHistoryUrlMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
		CacheHistoryTitleMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
	}
	// historytableurl := "serie_episode_histories_url"
	// historytabletitle := "serie_episode_histories_title"
	// historytablecount := "select count() from serie_episode_histories"
	// historytablequeryurl := "select distinct url from serie_episode_histories"
	// historytablequerytitle := "select distinct title from serie_episode_histories"
	// if logger.HasPrefixI(str, logger.StrMovie) {
	// 	historytableurl = "movie_histories_url"
	// 	historytabletitle = "movie_histories_title"
	// 	historytablecount = "select count() from movie_histories"
	// 	historytablequeryurl = "select distinct url from movie_histories"
	// 	historytablequerytitle = "select distinct title from movie_histories"
	// }

	// SetStringsCache(historytableurl, historytablecount, historytablequeryurl, 8*time.Hour)

	// SetStringsCache(historytabletitle, historytablecount, historytablequerytitle, 8*time.Hour)

}

func Refreshseriestitlecache() {
	if !config.SettingsGeneral.UseMediaCache {
		return
	}
	// if logger.GlobalCache.CheckNoType("dbseries_title_slug_cache") {
	// 	return
	// }
	// SetTwoStringOneIntCache("dbseries_title_slug_cache", "select count(*) from dbseries", "select seriename, slug, id from dbseries", 8*time.Hour)

	if Checkcacheexpire(CacheTitlesSeriesExpire) {
		return
	}
	CacheTitlesSeries = *QueryStaticColumnsTwoStringOneInt(false,
		QueryIntColumn("select count(*) from dbseries"),
		"select seriename, slug, id from dbseries")
	CacheTitlesSeriesExpire = logger.TimeGetNow().Add(time.Hour * -12)
}
func Refreshfilescached(cfgpstr string) string {
	if !config.SettingsGeneral.UseMediaCache {
		return ""
	}
	if cfgpstr[:5] == logger.StrSerie && Checkcacheexpire(CacheFilesSeriesExpire) {
		return "serie_episode_files_cached"
	}
	if cfgpstr[:5] != logger.StrSerie && Checkcacheexpire(CacheFilesMovieExpire) {
		return "movie_files_cached"
	}

	tablefilescached := "serie_episode_files_cached"
	if cfgpstr[:5] == logger.StrSerie {
		CacheFilesSeries = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from serie_episode_files"),
			"select location from serie_episode_files")
		CacheFilesSeriesExpire = logger.TimeGetNow().Add(time.Hour * -12)
	} else {
		tablefilescached = "movie_files_cached"
		CacheFilesMovie = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from movie_files"),
			"select location from movie_files")
		CacheFilesMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
	}

	//SetStringsCache(tablefilescached, querycountunwanted, queryunwanted, 3*time.Hour)

	return tablefilescached
}

func Refreshunmatchedcached(cfgpstr string) string {
	if !config.SettingsGeneral.UseMediaCache {
		return ""
	}
	if cfgpstr[:5] == logger.StrSerie && Checkcacheexpire(CacheUnmatchedSeriesExpire) {
		return logger.StrSerieFileUnmatched
	}
	if cfgpstr[:5] != logger.StrSerie && Checkcacheexpire(CacheUnmatchedMovieExpire) {
		return logger.StrMovieFileUnmatched
	}

	tablecached := logger.StrSerieFileUnmatched
	//querycountunmatched := "select count() from serie_file_unmatcheds where (last_checked > ? or last_checked is null)"
	//queryunmatched := "select filepath from serie_file_unmatcheds where (last_checked > ? or last_checked is null)"
	if cfgpstr[:5] != logger.StrSerie {
		CacheUnmatchedSeries = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from movie_file_unmatcheds where (last_checked > ? or last_checked is null)", TimeToSQLTime(logger.TimeGetNow().Add(time.Hour*-12), true)),
			"select filepath from movie_file_unmatcheds where (last_checked > ? or last_checked is null)", TimeToSQLTime(logger.TimeGetNow().Add(time.Hour*-12), true))
		CacheUnmatchedSeriesExpire = logger.TimeGetNow().Add(time.Hour * -12)
	} else {
		CacheUnmatchedMovie = *QueryStaticStringArray(false,
			QueryIntColumn("select count() from movie_file_unmatcheds where (last_checked > ? or last_checked is null)", TimeToSQLTime(logger.TimeGetNow().Add(time.Hour*-12), true)),
			"select filepath from movie_file_unmatcheds where (last_checked > ? or last_checked is null)", TimeToSQLTime(logger.TimeGetNow().Add(time.Hour*-12), true))
		CacheUnmatchedMovieExpire = logger.TimeGetNow().Add(time.Hour * -12)
	}

	//SetStringsCache(tablecached, querycountunmatched, queryunmatched, 3*time.Hour, TimeToSQLTime(logger.TimeGetNow().Add(time.Hour*-12), true))

	return tablecached
}
