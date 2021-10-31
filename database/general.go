package database

import (
	"database/sql"
	"log"
	"os"
	"regexp"
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
		r := regexp.MustCompile(qu.Regex)
		p := QualitiesRegex{Regexp: r, Qualities: qu}
		Getresolutions = append(Getresolutions, p)
	}
	quali, _ = QueryQualities(Query{Where: "Type=2", OrderBy: "priority desc"})
	Getqualities = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		r := regexp.MustCompile(qu.Regex)
		p := QualitiesRegex{Regexp: r, Qualities: qu}
		Getqualities = append(Getqualities, p)
	}
	quali, _ = QueryQualities(Query{Where: "Type=3", OrderBy: "priority desc"})
	Getcodecs = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		r := regexp.MustCompile(qu.Regex)
		p := QualitiesRegex{Regexp: r, Qualities: qu}
		Getcodecs = append(Getcodecs, p)
	}
	quali, _ = QueryQualities(Query{Where: "Type=4", OrderBy: "priority desc"})
	Getaudios = make([]QualitiesRegex, 0, len(quali))
	for _, qu := range quali {
		r := regexp.MustCompile(qu.Regex)
		p := QualitiesRegex{Regexp: r, Qualities: qu}
		Getaudios = append(Getaudios, p)
	}
}
func Upgrade(c *gin.Context) {
	UpgradeDB()
}

func UpgradeDB() {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite3://data.db?cache=shared&_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		log.Fatalf("migration failed... %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("An error occurred while syncing the database.. %v", err)
	}
}

type Version struct {
	ID            uint
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
	Version       string
	VersionNumber int `db:"version_number"`
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
