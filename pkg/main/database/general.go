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
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type DBGlobal struct {
	Getqualities     []QualitiesRegex
	Getresolutions   []QualitiesRegex
	Getcodecs        []QualitiesRegex
	Getaudios        []QualitiesRegex
	AudioStr         []string
	CodecStr         []string
	QualityStr       []string
	ResolutionStr    []string
	GetqualitiesIn   InQualitiesArray
	GetresolutionsIn InQualitiesArray
	GetcodecsIn      InQualitiesArray
	GetaudiosIn      InQualitiesArray
}

type dblocal struct {
	DB *sqlx.DB
	TX *sql.Tx
}

var readWriteMu *sync.RWMutex = &sync.RWMutex{}

var DBVersion string = "1"
var DBLogLevel string = "Info"

func DBClose() {
	if DB != nil {
		DB.Close()
	}
	if DBImdb != nil {
		DBImdb.Close()
	}
}

var DBConnect DBGlobal
var DB *sqlx.DB
var DBImdb *sqlx.DB

type QualitiesRegex struct {
	Regex string
	Qualities
}

type InQualitiesArray struct {
	Arr []QualitiesRegex
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

type Dbfiles struct {
	Location string
	ID       uint
}

func InitDb(dbloglevel string) {
	if _, err := os.Stat("./databases/data.db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/data.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	var err error
	DB, err = sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	DB.SetMaxIdleConns(15)
	DB.SetMaxOpenConns(5)
	DBLogLevel = strings.ToLower(dbloglevel)
}

func LockDb(db *dblocal) {
	readWriteMu.Lock()
}
func UnlockDb(db *dblocal) {
	readWriteMu.Unlock()
}
func RLockDb(db *dblocal) {
	readWriteMu.RLock()
}
func RUnlockDb(db *dblocal) {
	readWriteMu.RUnlock()
}
func CloseImdb() {
	if DBImdb != nil {
		DBImdb.Close()
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
	DBImdb, err = sqlx.Connect("sqlite3", "file:./databases/imdb.db?_fk=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	DBImdb.SetMaxIdleConns(15)
	DBImdb.SetMaxOpenConns(5)
	//logger.GlobalCache.Set("DBImdb", dblocal{DB: db, readWriteMu: &sync.RWMutex{}}, 0)
}

func GetVars() {
	priodesc := "priority desc"
	quali, _ := QueryQualities(Querywithargs{Query: Query{Where: "Type=1", OrderBy: priodesc}})
	logger.Log.GlobalLogger.Info("Database Get Variables 1")
	DBConnect.Getresolutions = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.Getresolutions[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali, _ = QueryQualities(Querywithargs{Query: Query{Where: "Type=2", OrderBy: priodesc}})
	logger.Log.GlobalLogger.Info("Database Get Variables 2")
	DBConnect.Getqualities = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.Getqualities[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali, _ = QueryQualities(Querywithargs{Query: Query{Where: "Type=3", OrderBy: priodesc}})
	logger.Log.GlobalLogger.Info("Database Get Variables 3")
	DBConnect.Getcodecs = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.Getcodecs[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}

	quali, _ = QueryQualities(Querywithargs{Query: Query{Where: "Type=4", OrderBy: priodesc}})
	logger.Log.GlobalLogger.Info("Database Get Variables 4")
	DBConnect.Getaudios = make([]QualitiesRegex, len(quali))
	for idx := range quali {
		logger.GlobalRegexCache.SetRegexp(quali[idx].Regex, quali[idx].Regex, 0)
		quali[idx].StringsLower = strings.ToLower(quali[idx].Strings)
		DBConnect.Getaudios[idx] = QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]}
	}
	quali = nil

	DBConnect.AudioStr = make([]string, 0, len(DBConnect.Getaudios)*2)
	for idx := range DBConnect.Getaudios {
		DBConnect.AudioStr = append(DBConnect.AudioStr, strings.Split(DBConnect.Getaudios[idx].StringsLower, ",")...)
	}
	DBConnect.CodecStr = make([]string, 0, len(DBConnect.Getcodecs)*2)
	for idx := range DBConnect.Getcodecs {
		DBConnect.CodecStr = append(DBConnect.CodecStr, strings.Split(DBConnect.Getcodecs[idx].StringsLower, ",")...)
	}
	DBConnect.QualityStr = make([]string, 0, len(DBConnect.Getqualities)*7)
	for idx := range DBConnect.Getqualities {
		DBConnect.QualityStr = append(DBConnect.QualityStr, strings.Split(DBConnect.Getqualities[idx].StringsLower, ",")...)
	}
	DBConnect.ResolutionStr = make([]string, 0, len(DBConnect.Getresolutions)*4)
	for idx := range DBConnect.Getresolutions {
		DBConnect.ResolutionStr = append(DBConnect.ResolutionStr, strings.Split(DBConnect.Getresolutions[idx].StringsLower, ",")...)
	}

	DBConnect.GetaudiosIn = InQualitiesArray{Arr: DBConnect.Getaudios}
	DBConnect.GetcodecsIn = InQualitiesArray{Arr: DBConnect.Getcodecs}
	DBConnect.GetqualitiesIn = InQualitiesArray{Arr: DBConnect.Getqualities}
	DBConnect.GetresolutionsIn = InQualitiesArray{Arr: DBConnect.Getresolutions}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(backupPath string, maxbackups int) error {
	_, err := dbexec("main", &Querywithargs{QueryString: "VACUUM INTO ?", Args: []interface{}{backupPath}})
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
type JobHistoryJson struct {
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

func RemoveOldDbBackups(max int) error {
	if max == 0 {
		return nil
	}

	prefix := "data.db."
	files, err := oldDatabaseFiles(prefix)
	if err != nil {
		return err
	}

	var remove []backupInfo = make([]backupInfo, 0, max)

	if max > 0 && max < len(files) {
		var preserved []string = make([]string, 0, max)

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
	var backupFiles []backupInfo = make([]backupInfo, 0, len(files))

	for idx := range files {
		if files[idx].IsDir() {
			continue
		}
		if strings.HasPrefix(files[idx].Name(), prefix) {
			if t, err := timeFromName(files[idx].Name(), prefix, ""); err == nil {
				backupFiles = append(backupFiles, backupInfo{t, files[idx]})
				continue
			}
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

type backupInfo struct {
	timestamp time.Time
	fs.DirEntry
}

// byFormatTime sorts by newest time formatted in the name.
type byFormatTime []backupInfo

func (b byFormatTime) Less(i, j int) bool {
	return b[i].timestamp.After(b[j].timestamp)
}

func (b byFormatTime) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byFormatTime) Len() int {
	return len(b)
}
