package database

import (
	"database/sql"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jmoiron/sqlx"

	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" //Needed for Migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"      //Needed for Migrate
	_ "github.com/mattn/go-sqlite3"                           //Needed for DB
)

type DBGlobal struct {
	AudioStrIn       logger.InStringArrayStruct
	CodecStrIn       logger.InStringArrayStruct
	QualityStrIn     logger.InStringArrayStruct
	ResolutionStrIn  logger.InStringArrayStruct
	GetqualitiesIn   InQualitiesArray
	GetresolutionsIn InQualitiesArray
	GetcodecsIn      InQualitiesArray
	GetaudiosIn      InQualitiesArray
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

type InQualitiesArray struct {
	Arr []QualitiesRegex
}

type backupInfo struct {
	timestamp time.Time
	fs.DirEntry
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []backupInfo

var readWriteMu = &sync.RWMutex{}

var DBConnect DBGlobal
var dbData *sqlx.DB
var dbImdb *sqlx.DB
var DBVersion = "1"
var DBLogLevel = "Info"

func DBClose() {
	if dbData != nil {
		dbData.Close()
	}
	if dbImdb != nil {
		dbImdb.Close()
	}
}

func InQualitiesRegexArray(target string, strArray *InQualitiesArray) uint {
	for idx := range strArray.Arr {
		if strings.EqualFold(strArray.Arr[idx].Name, target) {
			return strArray.Arr[idx].ID
		}
	}
	strArray = nil
	return 0
}

func InitDb(dbloglevel string) {
	if _, err := os.Stat("./databases/data.db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/data.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	var err error
	dbData, err = sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&_mutex=full&rt=1&_cslike=0")
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

func InitImdbdb(dbloglevel string) {
	if _, err := os.Stat("./databases/imdb.db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/imdb.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	var err error
	dbImdb, err = sqlx.Connect("sqlite3", "file:./databases/imdb.db?_fk=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	dbImdb.SetMaxIdleConns(15)
	dbImdb.SetMaxOpenConns(5)
}

func GetVars() {
	priodesc := "priority desc"
	var quali []Qualities
	QueryQualities(&Querywithargs{Query: Query{Where: "Type=1", OrderBy: priodesc}}, &quali)
	logger.Log.GlobalLogger.Info("Database Get Variables 1")
	DBConnect.GetresolutionsIn = InQualitiesArray{}
	DBConnect.GetresolutionsIn.Arr = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.GetresolutionsIn.Arr[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali = []Qualities{}
	QueryQualities(&Querywithargs{Query: Query{Where: "Type=2", OrderBy: priodesc}}, &quali)
	logger.Log.GlobalLogger.Info("Database Get Variables 2")
	DBConnect.GetqualitiesIn = InQualitiesArray{}
	DBConnect.GetqualitiesIn.Arr = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.GetqualitiesIn.Arr[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali = []Qualities{}
	QueryQualities(&Querywithargs{Query: Query{Where: "Type=3", OrderBy: priodesc}}, &quali)
	logger.Log.GlobalLogger.Info("Database Get Variables 3")
	DBConnect.GetcodecsIn = InQualitiesArray{}
	DBConnect.GetcodecsIn.Arr = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.GetcodecsIn.Arr[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali = []Qualities{}
	QueryQualities(&Querywithargs{Query: Query{Where: "Type=4", OrderBy: priodesc}}, &quali)
	logger.Log.GlobalLogger.Info("Database Get Variables 4")
	DBConnect.GetaudiosIn = InQualitiesArray{}
	DBConnect.GetaudiosIn.Arr = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.GetaudiosIn.Arr[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}
	quali = nil

	DBConnect.AudioStrIn.Arr = make([]string, 0, len(DBConnect.GetaudiosIn.Arr)*2)
	for idx := range DBConnect.GetaudiosIn.Arr {
		DBConnect.AudioStrIn.Arr = append(DBConnect.AudioStrIn.Arr, strings.Split(DBConnect.GetaudiosIn.Arr[idx].StringsLower, ",")...)
	}
	DBConnect.CodecStrIn.Arr = make([]string, 0, len(DBConnect.GetcodecsIn.Arr)*2)
	for idx := range DBConnect.GetcodecsIn.Arr {
		DBConnect.CodecStrIn.Arr = append(DBConnect.CodecStrIn.Arr, strings.Split(DBConnect.GetcodecsIn.Arr[idx].StringsLower, ",")...)
	}
	DBConnect.QualityStrIn.Arr = make([]string, 0, len(DBConnect.GetqualitiesIn.Arr)*7)
	for idx := range DBConnect.GetqualitiesIn.Arr {
		DBConnect.QualityStrIn.Arr = append(DBConnect.QualityStrIn.Arr, strings.Split(DBConnect.GetqualitiesIn.Arr[idx].StringsLower, ",")...)
	}
	DBConnect.ResolutionStrIn.Arr = make([]string, 0, len(DBConnect.GetresolutionsIn.Arr)*4)
	for idx := range DBConnect.GetresolutionsIn.Arr {
		DBConnect.ResolutionStrIn.Arr = append(DBConnect.ResolutionStrIn.Arr, strings.Split(DBConnect.GetresolutionsIn.Arr[idx].StringsLower, ",")...)
	}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(backupPath string, maxbackups int) error {
	_, err := dbexec(&Querywithargs{QueryString: "VACUUM INTO ?", Args: []interface{}{backupPath}})
	if err != nil {
		return errors.New("vacuum failed: " + err.Error())
	}
	RemoveOldDbBackups(maxbackups)

	return nil
}

func UpgradeDB() {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://./databases/data.db?cache=shared&_fk=1&_cslike=0",
	)

	vers, _, _ := m.Version()
	DBVersion = strconv.Itoa(int(vers))
	if err != nil {
		log.Fatalf("migration failed... %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database.. %v", err)
	}
}

func RemoveOldDbBackups(max int) error {
	if max == 0 {
		return nil
	}

	prefix := "data.db."
	files, err := oldDatabaseFiles(prefix)
	if err != nil {
		return err
	}

	var remove = make([]backupInfo, 0, max)

	if max == 0 || max >= len(files) {
		return err
	}

	var preserved = make([]string, 0, max)

	for idx := range files {
		// Only count the uncompressed log file or the
		// compressed log file, not both.
		fn := files[idx].Name()
		if !strings.HasPrefix(fn, prefix) {
			continue
		}

		preserved = append(preserved, fn)

		if len(preserved) > max {
			remove = append(remove, files[idx])
		}
	}

	for idx := range remove {
		errRemove := os.Remove(filepath.Join("./backup", remove[idx].Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}

	return err
}

func oldDatabaseFiles(prefix string) ([]backupInfo, error) {
	files, err := os.ReadDir("./backup")
	if err != nil {
		return nil, errors.New("can't read log file directory: " + err.Error())
	}
	var backupFiles = make([]backupInfo, 0, len(files))

	for idx := range files {
		if files[idx].IsDir() {
			continue
		}
		if !strings.HasPrefix(files[idx].Name(), prefix) {
			continue
		}
		if t, err := timeFromName(files[idx].Name(), prefix, ""); err == nil {
			backupFiles = append(backupFiles, backupInfo{t, files[idx]})
			continue
		}
	}

	sort.Sort(byFormatTime(backupFiles))

	return backupFiles, nil
}

func timeFromName(filename, prefix, ext string) (time.Time, error) {
	if !strings.HasPrefix(filename, prefix) {
		return time.Time{}, errors.New("mismatched prefix")
	}
	if !strings.HasSuffix(filename, ext) {
		return time.Time{}, errors.New("mismatched extension")
	}
	ts := filename[len(prefix) : len(filename)-len(ext)]
	if idx := strings.Index(ts, "."); idx != -1 {
		idn := idx + 1
		ts = ts[idn:]
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
