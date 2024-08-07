package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jmoiron/sqlx"

	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" //Needed for Migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"      //Needed for Migrate
	_ "github.com/mattn/go-sqlite3"                           //Needed for DB
)

// dbGlobal stores globally accessible slices and arrays
type DBGlobal struct {
	// AudioStrIn is a globally accessible slice of audio strings
	AudioStrIn []string
	// CodecStrIn is a globally accessible slice of codec strings
	CodecStrIn []string
	// QualityStrIn is a globally accessible slice of quality strings
	QualityStrIn []string
	// ResolutionStrIn is a globally accessible slice of resolution strings
	ResolutionStrIn []string
	// GetqualitiesIn is a globally accessible slice of Qualities structs
	GetqualitiesIn []Qualities
	// GetresolutionsIn is a globally accessible slice of Qualities structs
	GetresolutionsIn []Qualities
	// GetcodecsIn is a globally accessible slice of Qualities structs
	GetcodecsIn []Qualities
	// GetaudiosIn is a globally accessible slice of Qualities structs
	GetaudiosIn []Qualities
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

type backupInfo struct {
	timestamp time.Time
	path      string
}

const (
	strRegexSeriesIdentifier = "RegexSeriesIdentifier"
	strRegexSeriesTitle      = "RegexSeriesTitle"
	strRegexSeriesTitleDate  = "RegexSeriesTitleDate"
)

// DBClose closes any open database connections to the data.db and imdb.db
// SQLite databases. It is intended to be called when the application is
// shutting down to cleanly close the connections.
func DBClose() {
	if dbData != nil {
		dbData.Close()
	}
	if dbImdb != nil {
		dbImdb.Close()
	}
}

// checkFile checks if the file exists at the given path.
// It returns true if the file exists, false if it does not exist.
func checkFile(fpath string) bool {
	_, err := os.Stat(fpath)
	return !errors.Is(err, fs.ErrNotExist)
}

// InitDB initializes a connection to the data.db SQLite database.
// It creates the file if it does not exist and sets database
// connection parameters.
func InitDB(dbloglevel string) error {
	if !checkFile("./databases/data.db") {
		_, err := os.Create("./databases/data.db") // Create SQLite file
		if err != nil {
			return err
		}
	}
	var err error
	dbData, err = sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		return err
	}
	dbData.SetMaxIdleConns(15)
	dbData.SetMaxOpenConns(5)
	DBLogLevel = strings.ToLower(dbloglevel)
	return nil
}

// CloseImdb closes the dbImdb database connection if it is open.
func CloseImdb() {
	if dbImdb != nil {
		InvalidateImdbStmt()
		dbImdb.DB.Close()
	}
}

// GetVersion returns the current database version string stored in the DBVersion global variable.
func GetVersion() string {
	return DBVersion
}

// SetVersion sets the global DBVersion variable to the given version string.
func SetVersion(str string) {
	DBVersion = str
}

// OpenImdbdb opens a connection to the imdb.db SQLite database.
// It creates the file if it does not exist.
func OpenImdbdb() {
	dbImdb, _ = sqlx.Open("sqlite3", "file:./databases/imdb.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
}

// InitImdbdb initializes a connection to the imdb.db SQLite database.
// It creates the file if it does not exist.
func InitImdbdb() error {
	if !checkFile("./databases/imdb.db") {
		_, err := os.Create("./databases/imdb.db") // Create SQLite file
		if err != nil {
			return err
		}
	}
	var err error
	dbImdb, err = sqlx.Connect("sqlite3", "file:./databases/imdb.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		return err
	}
	dbImdb.SetMaxIdleConns(15)
	dbImdb.SetMaxOpenConns(5)
	return nil
}

// getqualityregexes queries the database for quality regexes of the given type,
// converts them to lowercase, compiles the regexes, and returns them along with
// the corresponding quality data from the database.
func getqualityregexes(querystr string, querycount string) []Qualities {
	q := GetrowsNsize[Qualities](false, querycount, querystr)
	if len(q) == 0 {
		return nil
	}
	ret := make([]Qualities, len(q))
	for idx := range q {
		q[idx].StringsLower = strings.ToLower(q[idx].Strings)
		globalCache.setRegexp(q[idx].Regex, 0)
		ret[idx] = q[idx]
	}
	return ret
}

// GetVars populates the global regex variables from the database.
// It retrieves the quality regexes from the database and processes them to populate:
// - DBConnect.GetresolutionsIn
// - DBConnect.GetqualitiesIn
// - DBConnect.GetcodecsIn
// - DBConnect.GetaudiosIn
// It also processes the config regex settings, and splits the regex strings to populate:
// - DBConnect.AudioStrIn
// - DBConnect.CodecStrIn
// - DBConnect.QualityStrIn
// - DBConnect.ResolutionStrIn
func SetVars() {
	DBConnect.GetresolutionsIn = getqualityregexes("select * from qualities where type=1 order by priority desc", "select count() from qualities where type=1")

	DBConnect.GetqualitiesIn = getqualityregexes("select * from qualities where type=2 order by priority desc", "select count() from qualities where type=2")

	DBConnect.GetcodecsIn = getqualityregexes("select * from qualities where type=3 order by priority desc", "select count() from qualities where type=3")

	DBConnect.GetaudiosIn = getqualityregexes("select * from qualities where type=4 order by priority desc", "select count() from qualities where type=4")

	globalCache.setRegexp(strRegexSeriesIdentifier, 0)
	globalCache.setRegexp(strRegexSeriesTitle, 0)
	globalCache.setRegexp(strRegexSeriesTitleDate, 0)
	for _, cfgregex := range config.SettingsRegex {
		for idxreg := range cfgregex.Rejected {
			globalCache.setRegexp(cfgregex.Rejected[idxreg], 0)
		}
		for idxreg := range cfgregex.Required {
			globalCache.setRegexp(cfgregex.Required[idxreg], 0)
		}
	}

	DBConnect.AudioStrIn = make([]string, 0, len(DBConnect.GetaudiosIn)*7)
	for idx := range DBConnect.GetaudiosIn {
		DBConnect.AudioStrIn = append(DBConnect.AudioStrIn, strings.Split(DBConnect.GetaudiosIn[idx].StringsLower, ",")...)
	}
	DBConnect.CodecStrIn = make([]string, 0, len(DBConnect.GetcodecsIn)*7)
	for idx := range DBConnect.GetcodecsIn {
		DBConnect.CodecStrIn = append(DBConnect.CodecStrIn, strings.Split(DBConnect.GetcodecsIn[idx].StringsLower, ",")...)
	}
	DBConnect.QualityStrIn = make([]string, 0, len(DBConnect.GetqualitiesIn)*7)
	for idx := range DBConnect.GetqualitiesIn {
		DBConnect.QualityStrIn = append(DBConnect.QualityStrIn, strings.Split(DBConnect.GetqualitiesIn[idx].StringsLower, ",")...)
	}
	DBConnect.ResolutionStrIn = make([]string, 0, len(DBConnect.GetresolutionsIn)*7)
	for idx := range DBConnect.GetresolutionsIn {
		DBConnect.ResolutionStrIn = append(DBConnect.ResolutionStrIn, strings.Split(DBConnect.GetresolutionsIn[idx].StringsLower, ",")...)
	}
}

// Upgrade handles upgrading the database by calling UpgradeDB.
func Upgrade(_ *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(backupPath string, maxbackups uint8) error {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	// logger.LogDynamicany("info", "Start db backup")
	// tempdb, err := sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0")
	// if err != nil {
	// 	return err
	// }
	_, err := dbData.Exec("VACUUM INTO ?", backupPath)
	//tempdb.Close()
	if err != nil {
		logger.LogDynamicany("error", "exec", err, "VACUUM INTO ?")
		return err
	}
	logger.LogDynamicany("info", "End db backup")
	if maxbackups == 0 {
		return nil
	}

	files, err := os.ReadDir("./backup")
	if err != nil {
		return errors.New("can't read log file directory: " + err.Error())
	}
	if len(files) == 0 {
		return nil
	}
	backupFiles := make([]backupInfo, 0, len(files))

	var tu time.Time
	for idx := range files {
		if files[idx].IsDir() {
			continue
		}
		if !logger.HasPrefixI(files[idx].Name(), "data.db.") {
			continue
		}
		addfile := backupInfo{timestamp: timeFromName(files[idx].Name(), "data.db."), path: files[idx].Name()}
		if !addfile.timestamp.Equal(tu) {
			backupFiles = append(backupFiles, addfile)
			continue
		}
	}

	if maxbackups == 0 || maxbackups >= uint8(len(backupFiles)) {
		//clear(backupFiles)
		//clear(files)
		return nil
	}
	slices.SortFunc(backupFiles, func(a, b backupInfo) int {
		if a.timestamp == b.timestamp {
			return 0
		}
		if a.timestamp.After(b.timestamp) {
			return -1
		}
		return 1
	})
	// sort.Slice(backupFiles, func(i, j int) bool {
	// 	return backupFiles[i].timestamp.After(backupFiles[j].timestamp)
	// })

	for _, e := range backupFiles[maxbackups:] {
		_ = os.Remove(filepath.Join("./backup", e.path))
	}
	//clear(backupFiles)
	//clear(files)

	return nil
}

// UpgradeDB initializes the database schema and upgrades to the latest version.
// It returns an error if migration fails.
func UpgradeDB() error {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://./databases/data.db?_fk=1&_cslike=0",
	)
	if err != nil {
		return err
	}

	vers, _, _ := m.Version()
	DBVersion = logger.IntToString(vers)

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

// UpgradeIMDB migrates the imdb database to the latest version. It initializes
// a database migration manager, pointing it to the migration scripts. It then
// runs the Up() method to apply any necessary changes. Any errors are printed.
func UpgradeIMDB() {
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite3://./databases/imdb.db?_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		fmt.Println(fmt.Errorf("migration failed... %w", err))
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Println(fmt.Errorf("an error occurred while syncing the database.. %w", err))
	}
}

// timeFromName parses a filename to extract a timestamp, given a known prefix.
// It returns a zero Time if parsing fails.
func timeFromName(filename, prefix string) time.Time {
	if !logger.HasPrefixI(filename, prefix) {
		//if len(filename) >= len(prefix) && filename[:len(prefix)] != prefix {
		return time.Time{}
	}
	if idx := strings.Index(filename[len(prefix):], logger.StrDot); idx != -1 {
		t, err := time.Parse("20060102_150405", filename[len(prefix):][idx+1:])
		if err != nil {
			return time.Time{}
		}
		return t
	}
	t, err := time.Parse("20060102_150405", filename[len(prefix):])
	if err != nil {
		return time.Time{}
	}
	return t
}

// ParseDate parses a date string in "2006-01-02" format and returns a sql.NullTime.
// Returns a null sql.NullTime if the date string is empty or fails to parse.
func ParseDate(date string) sql.NullTime {
	var d sql.NullTime
	if date == "" {
		return d
	}
	var err error
	d.Time, err = time.Parse("2006-01-02", date)
	if err != nil {
		return d
	}
	d.Valid = true
	return d
}

// GetMediaQualityConfig returns the QualityConfig from cfgp for the
// media with the given ID. It first checks if there is a quality profile
// set for that media in the database. If not, it returns the default
// QualityConfig from cfgp.
func GetMediaQualityConfig(cfgp *config.MediaTypeConfig, mediaid uint) *config.QualityConfig {
	return cfgp.GetMediaQualityConfigStr(Getdatarow1Map[string](false, cfgp.Useseries, logger.DBQualityMediaByID, mediaid))
}

// GetMediaListIDMovies returns the index of the media list with the given name
// in cfgp for the movie with the given ID. It returns -1 if cfgp is nil,
// listname is empty, or no list with that name exists.
func GetMediaListIDGetListname(cfgp *config.MediaTypeConfig, mediaid *uint) int8 {
	if cfgp == nil {
		logger.LogDynamicany("error", "the config couldnt be found")
		return -1
	}

	if config.SettingsGeneral.UseMediaCache {
		return cfgp.GetMediaListsEntryListID(CacheOneStringTwoIntIndexFuncStr(cfgp.Useseries, logger.CacheMedia, *mediaid))
	}
	return cfgp.GetMediaListsEntryListID(Getdatarow1Map[string](false, cfgp.Useseries, logger.DBListnameByMediaID, mediaid))
}

// GetDbStaticTwoStringIdx1 returns the index of the DbstaticTwoString element
// with Str1 equal to v, or -1 if not found.
func GetDbStaticTwoStringIdx1(tbl []DbstaticTwoString, v string) int {
	for idx := range tbl {
		if tbl[idx].Str1 == v || strings.EqualFold(tbl[idx].Str1, v) {
			return idx
		}
	}
	return -1
}

// GetDbStaticOneStringOneIntIdx searches tbl for an element where Str equals v, and returns
// the index of that element, or -1 if not found.
func GetDbStaticOneStringOneIntIdx(tbl []DbstaticOneStringOneUInt, v string) int {
	for idx := range tbl {
		if tbl[idx].Str == v || strings.EqualFold(tbl[idx].Str, v) {
			return idx
		}
	}
	return -1
}
