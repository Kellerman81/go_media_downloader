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

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sqlx.DB
var DBImdb *sqlx.DB
var WriteMu *sync.RWMutex
var ReadWriteMu *sync.Mutex
var Getqualities []QualitiesRegex
var Getresolutions []QualitiesRegex
var Getcodecs []QualitiesRegex
var Getaudios []QualitiesRegex

type QualitiesRegex struct {
	Regexp *regexp.Regexp
	Qualities
}

func InitDb(dbloglevel string) {
	if _, err := os.Stat("./data.db"); os.IsNotExist(err) {
		_, err := os.Create("./data.db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sqlx.Connect("sqlite3", "file:data.db?_fk=1&_mutex=no&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(1)
	ReadWriteMu = &sync.Mutex{}
	WriteMu = &sync.RWMutex{}
	DB = db
}

func InitImdbdb(dbloglevel string, dbfile string) *sqlx.DB {
	if _, err := os.Stat("./" + dbfile + ".db"); os.IsNotExist(err) {
		_, err := os.Create("./" + dbfile + ".db") // Create SQLite file
		if err != nil {
			log.Fatal(err.Error())
		}
	}
	db, err := sqlx.Connect("sqlite3", "file:"+dbfile+".db?_fk=1&_mutex=no&_cslike=0")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(1)
	return db
}

func GetVars() {

	quali, _ := QueryQualities(Query{Where: "Type=1", OrderBy: "priority desc"})
	Getresolutions = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		Getresolutions = append(Getresolutions, QualitiesRegex{Regexp: regexp.MustCompile(qu.Regex), Qualities: qu})
	}
	quali, _ = QueryQualities(Query{Where: "Type=2", OrderBy: "priority desc"})
	Getqualities = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		Getqualities = append(Getqualities, QualitiesRegex{Regexp: regexp.MustCompile(qu.Regex), Qualities: qu})
	}
	quali, _ = QueryQualities(Query{Where: "Type=3", OrderBy: "priority desc"})
	Getcodecs = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		Getcodecs = append(Getcodecs, QualitiesRegex{Regexp: regexp.MustCompile(qu.Regex), Qualities: qu})
	}
	quali, _ = QueryQualities(Query{Where: "Type=4", OrderBy: "priority desc"})
	Getaudios = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		Getaudios = append(Getaudios, QualitiesRegex{Regexp: regexp.MustCompile(qu.Regex), Qualities: qu})
	}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(db *sqlx.DB, backupPath string) error {
	if db == nil {
		var err error
		db, err = sqlx.Connect("sqlite3", "file:data.db?_fk=true")
		if err != nil {
			return fmt.Errorf("open database data.db failed:%s", err)
		}
		defer db.Close()
	}

	_, err := db.Exec(`VACUUM INTO "` + backupPath + `"`)
	if err != nil {
		return fmt.Errorf("vacuum failed: %s", err)
	}

	return nil
}

var DBVersion string

func UpgradeDB() {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://data.db?cache=shared&_fk=1&_mutex=no&_cslike=0",
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

	prefix := "data.db." + DBVersion + "."
	oldDbVersion, _ := strconv.Atoi(DBVersion)
	if oldDbVersion != 0 {
		oldDbVersion = oldDbVersion - 1
	}
	oldprefix := "data.db." + strconv.Itoa(oldDbVersion) + "."
	files, err := oldDatabaseFiles(prefix, oldprefix)
	if err != nil {
		return err
	}

	var remove []backupInfo

	if max > 0 && max < len(files) {
		preserved := make(map[string]bool)
		for _, f := range files {
			// Only count the uncompressed log file or the
			// compressed log file, not both.
			fn := f.Name()
			if !strings.HasPrefix(fn, prefix) && !strings.HasPrefix(fn, oldprefix) {
				continue
			}

			preserved[fn] = true

			if len(preserved) > max {
				remove = append(remove, f)
			}
		}
	}

	for _, f := range remove {
		errRemove := os.Remove(filepath.Join("./backup", f.Name()))
		if err == nil && errRemove != nil {
			err = errRemove
		}
	}

	return err
}

func oldDatabaseFiles(prefix string, oldprefix string) ([]backupInfo, error) {
	files, err := ioutil.ReadDir("./backup")
	if err != nil {
		return nil, fmt.Errorf("can't read log file directory: %s", err)
	}
	backupFiles := []backupInfo{}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if strings.HasPrefix(f.Name(), prefix) {
			if t, err := timeFromName(f.Name(), prefix, ""); err == nil {
				backupFiles = append(backupFiles, backupInfo{t, f})
				continue
			}
		}

		if strings.HasPrefix(f.Name(), oldprefix) {
			if t, err := timeFromName(f.Name(), oldprefix, ""); err == nil {
				backupFiles = append(backupFiles, backupInfo{t, f})
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
