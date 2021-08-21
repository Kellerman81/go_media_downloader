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
	// db := DB
	// var upgraded bool
	// var getdbversion Version
	// result := db.First(&getdbversion)
	// if result.RowsAffected == 0 {
	// 	mylog.Log.Info("Add/Update Versions Table")
	// 	db.Exec("DROP Table versions")
	// 	db.AutoMigrate(&Version{})
	// 	db.Create(&Version{Version: "0.00", VersionNumber: 0})
	// 	db.First(&getdbversion)
	// }
	// if getdbversion.VersionNumber == 0 {
	// 	mylog.Log.Info("Create Tables: v1")
	// 	db.Debug().AutoMigrate(&Dbmovie{})
	// 	db.Debug().AutoMigrate(&DbmovieTitle{})

	// 	db.Debug().AutoMigrate(&Movie{})
	// 	db.Debug().AutoMigrate(&MovieFile{})
	// 	db.Debug().AutoMigrate(&MovieHistory{})

	// 	db.Debug().AutoMigrate(&Dbserie{})
	// 	db.Debug().AutoMigrate(&DbserieAlternate{})
	// 	db.Debug().AutoMigrate(&DbserieEpisode{})

	// 	db.Debug().AutoMigrate(&Serie{})
	// 	db.Debug().AutoMigrate(&SerieEpisode{})
	// 	db.Debug().AutoMigrate(&SerieEpisodeFile{})
	// 	db.Debug().AutoMigrate(&SerieEpisodeHistory{})
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 2 {
	// 	mylog.Log.Info("Create Tables: v2")
	// 	db.Debug().AutoMigrate(&JobHistory{})
	// 	var counter int64
	// 	db.Model(SerieEpisode{}).Count(&counter)
	// 	dbepisodes := make([]SerieEpisode, 0, counter)
	// 	db.Find(&dbepisodes)
	// 	for _, row := range dbepisodes {
	// 		db.Model(&SerieEpisodeFile{}).Where("serie_episode_id=?", row.ID).Update("serie_id", row.SerieID)

	// 		db.Model(&SerieEpisodeHistory{}).Where("serie_episode_id=?", row.ID).Update("serie_id", row.SerieID)
	// 	}
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 3 {
	// 	db.Debug().AutoMigrate(&Movie{})
	// 	db.Debug().AutoMigrate(&Serie{})
	// 	db.Debug().AutoMigrate(&SerieEpisode{})
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 4 {
	// 	db.Debug().AutoMigrate(&MovieFileUnmatched{})
	// 	db.Debug().AutoMigrate(&SerieFileUnmatched{})
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 5 {
	// 	db.Debug().AutoMigrate(&Dbmovie{})
	// 	db.Debug().AutoMigrate(&DbmovieTitle{})
	// 	db.Debug().AutoMigrate(&DbserieAlternate{})
	// 	var counter int64
	// 	db.Model(Dbmovie{}).Count(&counter)
	// 	dbmovies := make([]Dbmovie, 0, counter)
	// 	db.Find(&dbmovies)
	// 	for _, row := range dbmovies {
	// 		db.Model(&Dbmovie{}).Where("id=?", row.ID).Update("slug", mylog.StringToSlug(row.Title))
	// 	}
	// 	db.Model(DbmovieTitle{}).Count(&counter)
	// 	dbmovietitles := make([]DbmovieTitle, 0, counter)
	// 	db.Find(&dbmovietitles)
	// 	for _, row := range dbmovietitles {
	// 		db.Model(&DbmovieTitle{}).Where("id=?", row.ID).Update("slug", mylog.StringToSlug(row.Title))
	// 	}
	// 	db.Model(DbserieAlternate{}).Count(&counter)
	// 	dbseriealt := make([]DbserieAlternate, 0, counter)
	// 	db.Find(&dbseriealt)
	// 	for _, row := range dbseriealt {
	// 		db.Model(&DbserieAlternate{}).Where("id=?", row.ID).Update("slug", mylog.StringToSlug(row.Title))
	// 	}
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 6 {
	// 	db.Debug().AutoMigrate(&RSSHistory{})
	// 	db.Debug().AutoMigrate(&IndexerFail{})
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 7 {
	// 	db.Debug().AutoMigrate(&Dbmovie{})
	// 	db.Debug().AutoMigrate(&Dbserie{})
	// 	upgraded = true
	// }
	// if getdbversion.VersionNumber < 8 {
	// 	db.Debug().AutoMigrate(&Qualities{})
	// 	upgraded = true
	// }
	// if upgraded {
	// 	mylog.Log.Info("Upgrade DB Completed - current version: ", dbversion, " old version: ", getdbversion.VersionNumber)
	// 	db.Model(&Version{}).Where("id=?", getdbversion.ID).Update("version_number", dbversion)
	// 	db.Model(&Version{}).Where("id=?", getdbversion.ID).Update("version", dbversionname)
	// }
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
