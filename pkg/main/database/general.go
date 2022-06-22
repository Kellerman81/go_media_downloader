package database

import (
	"database/sql"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB
var SQLDB *sql.DB
var DBImdb *sqlx.DB
var ReadWriteMu *sync.RWMutex
var Getqualities []QualitiesRegex
var Getresolutions []QualitiesRegex
var Getcodecs []QualitiesRegex
var Getaudios []QualitiesRegex
var AudioStr, CodecStr, QualityStr, ResolutionStr []string

type QualitiesRegex struct {
	Regex string
	Qualities
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
	db, err := sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=1&_mutex=full&rt=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	ReadWriteMu = &sync.RWMutex{}
	DB = db
	SQLDB = db.DB
}

func InitImdbdb(dbloglevel string, dbfile string) *sqlx.DB {
	if _, err := os.Stat("./databases/" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./databases/" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sqlx.Connect("sqlite3", "file:./databases/"+dbfile+".db?_fk=1&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(15)
	db.SetMaxOpenConns(5)
	return db
}

func GetVars() {

	logger.Log.Infoln("Database Get Variables 1")
	quali, _ := QueryQualities(Query{Where: "Type=1", OrderBy: "priority desc"})
	defer logger.ClearVar(&quali)
	logger.Log.Infoln("Database Get Variables 1-1")
	Getresolutions = make([]QualitiesRegex, 0, len(quali))
	for idx := range quali {
		logger.Log.Infoln("Database Get Variables 1-", quali[idx])
		config.RegexDelete(quali[idx].Regex)
		config.RegexAdd(quali[idx].Regex, *regexp.MustCompile(quali[idx].Regex))
		Getresolutions = append(Getresolutions, QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]})
	}

	logger.Log.Infoln("Database Get Variables 2")
	quali, _ = QueryQualities(Query{Where: "Type=2", OrderBy: "priority desc"})
	Getqualities = make([]QualitiesRegex, 0, len(quali))
	for idx := range quali {
		config.RegexDelete(quali[idx].Regex)
		config.RegexAdd(quali[idx].Regex, *regexp.MustCompile(quali[idx].Regex))
		Getqualities = append(Getqualities, QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]})
	}

	logger.Log.Infoln("Database Get Variables 3")
	quali, _ = QueryQualities(Query{Where: "Type=3", OrderBy: "priority desc"})
	Getcodecs = make([]QualitiesRegex, 0, len(quali))
	for idx := range quali {
		config.RegexDelete(quali[idx].Regex)
		config.RegexAdd(quali[idx].Regex, *regexp.MustCompile(quali[idx].Regex))
		Getcodecs = append(Getcodecs, QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]})
	}

	logger.Log.Infoln("Database Get Variables 4")
	quali, _ = QueryQualities(Query{Where: "Type=4", OrderBy: "priority desc"})
	Getaudios = make([]QualitiesRegex, 0, len(quali))
	for idx := range quali {
		config.RegexDelete(quali[idx].Regex)
		config.RegexAdd(quali[idx].Regex, *regexp.MustCompile(quali[idx].Regex))
		Getaudios = append(Getaudios, QualitiesRegex{Regex: quali[idx].Regex, Qualities: quali[idx]})
	}

	logger.Log.Infoln("Database Get Variables 5")
	AudioStr = make([]string, 0, len(Getaudios)*2)
	var splitted []string
	defer logger.ClearVar(&splitted)
	for idx := range Getaudios {
		splitted = strings.Split(Getaudios[idx].Strings, ",")
		for idxsplit := range splitted {
			AudioStr = append(AudioStr, splitted[idxsplit])
		}
	}
	logger.Log.Infoln("Database Get Variables 6")
	CodecStr = make([]string, 0, len(Getcodecs)*2)
	for idx := range Getcodecs {
		splitted = strings.Split(Getcodecs[idx].Strings, ",")
		for idxsplit := range splitted {
			CodecStr = append(CodecStr, splitted[idxsplit])
		}
	}
	logger.Log.Infoln("Database Get Variables 7")
	QualityStr = make([]string, 0, len(Getqualities)*7)
	for idx := range Getqualities {
		splitted = strings.Split(Getqualities[idx].Strings, ",")
		for idxsplit := range splitted {
			QualityStr = append(QualityStr, splitted[idxsplit])
		}
	}
	logger.Log.Infoln("Database Get Variables 8")
	ResolutionStr = make([]string, 0, len(Getresolutions)*4)
	for idx := range Getresolutions {
		splitted = strings.Split(Getresolutions[idx].Strings, ",")
		for idxsplit := range splitted {
			ResolutionStr = append(ResolutionStr, splitted[idxsplit])
		}
	}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(db *sqlx.DB, backupPath string, maxbackups int) error {
	if db == nil {
		var err error
		db, err = sqlx.Connect("sqlite3", "file:./databases/data.db?_fk=true")
		if err != nil {
			return fmt.Errorf("open database data.db failed:%s", err)
		}
		defer db.Close()
	}

	_, err := db.Exec(`VACUUM INTO "` + backupPath + `"`)
	if err != nil {
		return fmt.Errorf("vacuum failed: %s", err)
	}
	RemoveOldDbBackups(maxbackups)

	return nil
}

var DBVersion string

func UpgradeDB() {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://./databases/data.db?cache=shared&_fk=1&_cslike=0",
	)
	defer logger.ClearVar(m)
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
	defer logger.ClearVar(&files)
	if err != nil {
		return err
	}

	var remove []backupInfo
	defer logger.ClearVar(&remove)

	if max > 0 && max < len(files) {
		preserved := make([]string, 0, len(files))
		defer logger.ClearVar(&preserved)

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
	files, err := ioutil.ReadDir("./backup")
	defer logger.ClearVar(&files)
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}
	backupFiles := make([]backupInfo, 0, len(files))

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
	ts := strings.Repeat(filename[len(prefix):len(filename)-len(ext)], 1)
	if idx := strings.Index(ts, "."); idx != -1 {
		idn := idx + 1
		ts = strings.Repeat(ts[idn:], 1)
	}
	return time.Parse("20060102_150405", ts)
}

type backupInfo struct {
	timestamp time.Time
	os.FileInfo
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
