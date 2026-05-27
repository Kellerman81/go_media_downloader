package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite" // Needed for Migrate
	_ "github.com/golang-migrate/migrate/v4/source/file"     // Needed for Migrate
	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

// DBGlobal stores globally accessible slices and arrays.
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
	// GetaudioformatsIn is a globally accessible slice of Qualities structs for audio format (type 5)
	GetaudioformatsIn []Qualities
}

type JobHistory struct {
	CreatedAt   time.Time    `comment:"Record creation timestamp"   db:"created_at"   displayname:"Date Created"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" db:"updated_at"   displayname:"Last Updated"`
	JobType     string       `comment:"Type of job"                 db:"job_type"     displayname:"Job Type"`
	JobCategory string       `comment:"Job category classification" db:"job_category" displayname:"Job Category"`
	JobGroup    string       `comment:"Job group identifier"        db:"job_group"    displayname:"Job Group"`
	Started     sql.NullTime `comment:"Job start timestamp"                           displayname:"Start Time"`
	Ended       sql.NullTime `comment:"Job completion timestamp"                      displayname:"End Time"`
	ID          uint         `comment:"Unique job identifier"                         displayname:"Job ID"`
}

type RSSHistory struct {
	Config    string    `comment:"RSS configuration name"      displayname:"Configuration Name"`
	List      string    `comment:"RSS list identifier"         displayname:"List Name"`
	Indexer   string    `comment:"RSS indexer source"          displayname:"Indexer Name"`
	LastID    string    `comment:"Last processed item"         displayname:"Last Item ID"       db:"last_id"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID        uint      `comment:"Unique RSS identifier"       displayname:"RSS ID"`
}
type IndexerFail struct {
	Indexer   string       `comment:"Failed indexer name"         displayname:"Indexer Name"`
	LastFail  sql.NullTime `comment:"Last failure timestamp"      displayname:"Last Failure" db:"last_fail"`
	CreatedAt time.Time    `comment:"Record creation timestamp"   displayname:"Date Created" db:"created_at"`
	UpdatedAt time.Time    `comment:"Last modification timestamp" displayname:"Last Updated" db:"updated_at"`
	ID        uint         `comment:"Unique failure identifier"   displayname:"Failure ID"`
}

type backupInfo struct {
	path      string
	timestamp time.Time
}

const (
	strRegexSeriesIdentifier = "RegexSeriesIdentifier"
	strRegexSeriesTitle      = "RegexSeriesTitle"
	strRegexSeriesTitleDate  = "RegexSeriesTitleDate"
)

// InitCfg initializes the application configuration, cache, logger, and database systems.
// It loads configuration from the database, initializes the cache, sets up the logger
// with configured parameters, upgrades databases, and initializes both main and IMDB databases.
// This is a comprehensive initialization function used in testing and setup scenarios.
func InitCfg() {
	config.LoadCfgDB(false)

	InitCache()

	logger.InitLogger(logger.Config{
		LogLevel:      config.GetSettingsGeneral().LogLevel,
		LogFileSize:   config.GetSettingsGeneral().LogFileSize,
		LogFileCount:  config.GetSettingsGeneral().LogFileCount,
		LogCompress:   config.GetSettingsGeneral().LogCompress,
		LogToFileOnly: config.GetSettingsGeneral().LogToFileOnly,
		LogColorize:   config.GetSettingsGeneral().LogColorize,
		TimeFormat:    config.GetSettingsGeneral().TimeFormat,
		TimeZone:      config.GetSettingsGeneral().TimeZone,
		// LogZeroValues: config.GetSettingsGeneral().LogZeroValues,
	})

	err := UpgradeDB()
	if err != nil {
		logger.Logtype("fatal", 0).
			Err(err).
			Msg("Database Upgrade Failed")
	}

	UpgradeIMDB()

	err = InitDB(config.GetSettingsGeneral().DBLogLevel)
	if err != nil {
		logger.Logtype("fatal", 0).
			Err(err).
			Msg("Database Initialization Failed")
	}

	// Populate slugs for existing records if DB version >= 23
	if vers, _ := strconv.Atoi(DBVersion); vers >= 23 {
		PopulateSlugs(false)
	}

	err = InitImdbdb()
	if err != nil {
		logger.Logtype("fatal", 0).
			Err(err).
			Msg("IMDB Database Initialization Failed")
	}

	SetVars()
}

// DBClose closes any open database connections to the data.db and imdb.db
// SQLite databases. It is intended to be called when the application is
// shutting down to cleanly close the connections.
func DBClose() {
	sqlCTX.Done()

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
	return !errors.Is(err, os.ErrNotExist)
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

	dbData, err = sqlx.Connect(
		"sqlite",
		"file:./databases/data.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0",
	)
	if err != nil {
		return err
	}

	dbData.SetMaxIdleConns(5)
	dbData.SetMaxOpenConns(25)
	dbData.SetConnMaxLifetime(5 * time.Minute) // Rotate connections
	dbData.SetConnMaxIdleTime(1 * time.Minute) // Close idle
	SetDBLogLevel(strings.ToLower(dbloglevel)) // Use thread-safe accessor

	return nil
}

// CloseImdb closes the dbImdb database connection if it is open.
// It first invalidates any prepared statements to prevent resource leaks,
// then closes the database connection and sets the global dbImdb variable to nil.
// This function should be called during application shutdown or database reinitialization.
func CloseImdb() {
	if dbImdb != nil {
		InvalidateImdbStmt()
		dbImdb.Close()
	}
}

// GetVersion returns the current database version string stored in the DBVersion global variable.
// This version string is used for database migration tracking and compatibility checks.
// The version follows semantic versioning patterns and is updated during database upgrades.
func GetVersion() string {
	return GetDBVersion() // Use thread-safe accessor
}

// SetVersion sets the global DBVersion variable to the given version string.
// This function is used during database initialization and migration processes
// to update the current database schema version. Should be called after
// successful database upgrades to maintain version consistency.
func SetVersion(str string) {
	SetDBVersion(str) // Use thread-safe accessor
}

// OpenImdbdb opens a connection to the imdb.db SQLite database.
// It creates the file if it does not exist.
func OpenImdbdb() {
	dbImdb, _ = sqlx.Open(
		"sqlite",
		"file:./databases/imdb.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0",
	) // sqlite == modernc, sqlite3 = mattn
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

	dbImdb, err = sqlx.Connect(
		"sqlite",
		"file:./databases/imdb.db?_fk=1&mode=rwc&_mutex=full&rt=1&_cslike=0",
	)
	if err != nil {
		return err
	}

	dbImdb.SetMaxIdleConns(2)
	dbImdb.SetMaxOpenConns(10)
	dbImdb.SetConnMaxLifetime(5 * time.Minute) // Rotate connections
	dbImdb.SetConnMaxIdleTime(2 * time.Minute) // Close idle

	return nil
}

// getqualityregexes queries the database for quality regexes of the given type,
// converts them to lowercase, compiles the regexes, and returns them along with
// the corresponding quality data from the database.
func getqualityregexes(querystr, querycount string) []Qualities {
	count := Getdatarow[uint](false, querycount)
	if count == 0 {
		return nil
	}

	q := StructscanT[Qualities](false, count, querystr)
	if len(q) == 0 {
		return nil
	}

	for idx := range q {
		q[idx].StringsLower = strings.ToLower(q[idx].Strings)
		q[idx].StringsLowerSplitted = strings.Split(q[idx].StringsLower, ",")
		globalCache.setStaticRegexp(q[idx].Regex)
	}

	return q
}

// SetVars populates the global regex variables from the database.
// It retrieves the quality regexes from the database and processes them to populate:
// - DBConnect.GetresolutionsIn
// - DBConnect.GetqualitiesIn
// - DBConnect.GetcodecsIn
// - DBConnect.GetaudiosIn
// It also processes the config regex settings, and splits the regex strings to populate:
// - DBConnect.AudioStrIn
// - DBConnect.CodecStrIn
// - DBConnect.QualityStrIn
// - DBConnect.ResolutionStrIn.
func SetVars() {
	// Acquire write lock to safely modify DBConnect global variable
	mu := getGlobalVarMutex()

	mu.Lock()
	defer mu.Unlock()

	var totalAudioCap, totalCodecCap, totalQualityCap, totalResolutionCap int
	// prepare regexes - if you don't do this - you might get a memory leak
	DBConnect.GetresolutionsIn = getqualityregexes(
		"select * from qualities where type=1 order by priority desc",
		"select count() from qualities where type=1",
	)

	DBConnect.GetqualitiesIn = getqualityregexes(
		"select * from qualities where type=2 order by priority desc",
		"select count() from qualities where type=2",
	)

	DBConnect.GetcodecsIn = getqualityregexes(
		"select * from qualities where type=3 order by priority desc",
		"select count() from qualities where type=3",
	)

	DBConnect.GetaudiosIn = getqualityregexes(
		"select * from qualities where type=4 order by priority desc",
		"select count() from qualities where type=4",
	)

	DBConnect.GetaudioformatsIn = getqualityregexes(
		"select * from qualities where type=5 order by priority desc",
		"select count() from qualities where type=5",
	)

	globalCache.setStaticRegexp(strRegexSeriesIdentifier)
	globalCache.setStaticRegexp(strRegexSeriesTitle)
	globalCache.setStaticRegexp(strRegexSeriesTitleDate)
	config.RangeSettingsRegex(func(_ string, cfgregex *config.RegexConfig) {
		for i := range cfgregex.Rejected {
			globalCache.setStaticRegexp(cfgregex.Rejected[i])
		}

		for i := range cfgregex.Required {
			globalCache.setStaticRegexp(cfgregex.Required[i])
		}
	})

	// Calculate required capacity
	for idx := range DBConnect.GetaudiosIn {
		totalAudioCap += len(DBConnect.GetaudiosIn[idx].StringsLowerSplitted)
	}

	for idx := range DBConnect.GetcodecsIn {
		totalCodecCap += len(DBConnect.GetcodecsIn[idx].StringsLowerSplitted)
	}

	for idx := range DBConnect.GetqualitiesIn {
		totalQualityCap += len(DBConnect.GetqualitiesIn[idx].StringsLowerSplitted)
	}

	for idx := range DBConnect.GetresolutionsIn {
		totalResolutionCap += len(DBConnect.GetresolutionsIn[idx].StringsLowerSplitted)
	}

	DBConnect.AudioStrIn = make([]string, 0, totalAudioCap)
	DBConnect.CodecStrIn = make([]string, 0, totalCodecCap)
	DBConnect.QualityStrIn = make([]string, 0, totalQualityCap)
	DBConnect.ResolutionStrIn = make([]string, 0, totalResolutionCap)

	// Populate slices efficiently
	for idx := range DBConnect.GetaudiosIn {
		DBConnect.AudioStrIn = append(DBConnect.AudioStrIn,
			DBConnect.GetaudiosIn[idx].StringsLowerSplitted...)
	}

	for idx := range DBConnect.GetcodecsIn {
		DBConnect.CodecStrIn = append(DBConnect.CodecStrIn,
			DBConnect.GetcodecsIn[idx].StringsLowerSplitted...)
	}

	for idx := range DBConnect.GetqualitiesIn {
		DBConnect.QualityStrIn = append(DBConnect.QualityStrIn,
			DBConnect.GetqualitiesIn[idx].StringsLowerSplitted...)
	}

	for idx := range DBConnect.GetresolutionsIn {
		DBConnect.ResolutionStrIn = append(DBConnect.ResolutionStrIn,
			DBConnect.GetresolutionsIn[idx].StringsLowerSplitted...)
	}

	// prepare SQL statements for the cache (not expiring)
	globalCache.addStaticXStmt(
		"select primary_title, start_year, is_adult, genres, original_title, runtime_minutes, slug from imdb_titles where tconst = ?",
		true,
	)
	globalCache.addStaticXStmt(
		"select num_votes, average_rating from imdb_ratings where tconst = ?",
		true,
	)
	globalCache.addStaticXStmt(
		"select count() from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)",
		true,
	)
	globalCache.addStaticXStmt(
		"select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)",
		true,
	)
	globalCache.addStaticXStmt(
		"select count() from (select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?)",
		true,
	)
	globalCache.addStaticXStmt(
		"select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?",
		true,
	)
	globalCache.addStaticXStmt("select start_year from imdb_titles where tconst = ?", true)
	globalCache.addStaticXStmt(
		"select count() from imdb_ratings where tconst = ? and num_votes < ?",
		true,
	)
	globalCache.addStaticXStmt(
		"select count() from imdb_ratings where tconst = ? and average_rating < ?",
		true,
	)
	globalCache.addStaticXStmt("select count() from imdb_genres where tconst = ?", true)
	globalCache.addStaticXStmt("select genre from imdb_genres where tconst = ?", true)
	globalCache.addStaticXStmt("select count() from imdb_akas where tconst = ?", true)
	globalCache.addStaticXStmt("select title, region, slug from imdb_akas where tconst = ?", true)
	globalCache.addStaticXStmt("select count() from imdb_akas where tconst = ?", true)
	globalCache.addStaticXStmt("select region, title, slug from imdb_akas where tconst = ?", true)

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		SetVarsType(media.IsType, media)
		return nil
	})

	globalCache.addStaticXStmt(
		"select count() from indexer_fails where  last_fail > ? and indexer = ?",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from movie_file_unmatcheds where filepath = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select filepath from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from movie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into movie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		false,
	)
	globalCache.addStaticXStmt(
		"update movie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		false,
	)
	globalCache.addStaticXStmt("delete from movie_file_unmatcheds where filepath = ?", false)
	globalCache.addStaticXStmt(
		"delete from movie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from serie_file_unmatcheds where filepath = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select filepath from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from serie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into serie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		false,
	)
	globalCache.addStaticXStmt(
		"update serie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		false,
	)
	globalCache.addStaticXStmt("delete from serie_file_unmatcheds where filepath = ?", false)
	globalCache.addStaticXStmt(
		"delete from serie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		false,
	)

	globalCache.addStaticXStmt("select count() from dbserie_episodes where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )",
		false,
	)
	globalCache.addStaticXStmt(
		"select count(distinct season) from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from dbserie_episodes where dbserie_id = ? and identifier=REPLACE(?,'.','-') COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from dbserie_episodes where dbserie_id = ? and identifier=REPLACE(?,' ','-') COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select id from dbserie_episodes where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id from dbserie_episodes where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select season, episode from dbserie_episodes where dbserie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where missing=1 and dbserie_id = ? )",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct season from dbserie_episodes where dbserie_id = ? and season != '' and ((Select search_specials from series where id =?)=1 OR ((Select search_specials from series where id =?)=0 and season != '0')) and dbserie_episodes.id in ( Select distinct dbserie_episode_id from serie_episodes where dbserie_id = ? )",
		false,
	)
	globalCache.addStaticXStmt("select title from dbserie_episodes where id = ?", false)
	globalCache.addStaticXStmt(
		"select episode from dbserie_episodes where id = ? and episode != ''",
		false,
	)
	globalCache.addStaticXStmt("select runtime, season from dbserie_episodes where id = ?", false)
	globalCache.addStaticXStmt(
		"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt(
		"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from (select distinct url from movie_histories)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from (select distinct title from movie_histories)",
		false,
	)
	globalCache.addStaticXStmt("select count() from movie_histories where title = ?", false)
	globalCache.addStaticXStmt("select count() from movie_histories where url = ?", false)
	globalCache.addStaticXStmt("select distinct url from movie_histories", false)
	globalCache.addStaticXStmt("select distinct title from movie_histories", false)
	globalCache.addStaticXStmt(
		"Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?, ?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt(
		"delete from movie_histories where movie_id in (Select id from movies where listname = ? COLLATE NOCASE)",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from (select distinct url from serie_episode_histories)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from (select distinct title from serie_episode_histories)",
		false,
	)
	globalCache.addStaticXStmt("select count() from serie_episode_histories where title = ?", false)
	globalCache.addStaticXStmt("select count() from serie_episode_histories where url = ?", false)
	globalCache.addStaticXStmt("select distinct url from serie_episode_histories", false)
	globalCache.addStaticXStmt("select distinct title from serie_episode_histories", false)
	globalCache.addStaticXStmt(
		"Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt(
		"delete from serie_episode_histories where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		false,
	)

	globalCache.addStaticXStmt("select count() from dbmovies", false)
	globalCache.addStaticXStmt("select id from dbmovies where slug = ?", false)
	globalCache.addStaticXStmt("select id from dbmovies where title = ? COLLATE NOCASE", false)
	globalCache.addStaticXStmt("select id from dbmovies where imdb_id = ?", false)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,title,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?",
		false,
	)
	globalCache.addStaticXStmt("select imdb_id from dbmovies where id = ?", false)
	globalCache.addStaticXStmt("select imdb_id from dbmovies where moviedb_id = ?", false)
	globalCache.addStaticXStmt("select title, slug, imdb_id, year, id from dbmovies", false)
	globalCache.addStaticXStmt("select year, title, slug from dbmovies where id = ?", false)
	globalCache.addStaticXStmt("select year from dbmovies where id = ?", false)
	globalCache.addStaticXStmt("select runtime from dbmovies where id = ?", false)
	globalCache.addStaticXStmt(
		"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where dbmovies.id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id",
		false,
	)
	globalCache.addStaticXStmt(
		"update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
		false,
	)
	globalCache.addStaticXStmt("update dbmovies SET Title = ? where id = ?", false)
	globalCache.addStaticXStmt("insert into dbmovies (Imdb_ID) VALUES (?)", false)

	globalCache.addStaticXStmt(
		"select count() from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select count() from movies", false)
	globalCache.addStaticXStmt(
		"select count() from movies where listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from movies where listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from movies where dbmovie_id = ? and listname = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id,missing from movies where listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id from movies where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select id, quality_reached, quality_profile from movies where listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select dbmovie_id, rootpath, listname from movies where id = ?",
		false,
	)
	globalCache.addStaticXStmt("select dbmovie_id from movies where id = ?", false)
	globalCache.addStaticXStmt("select lower(listname), dbmovie_id, id from movies", false)
	globalCache.addStaticXStmt("select listname from movies where id = ?", false)
	globalCache.addStaticXStmt(
		"select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)",
		false,
	)
	globalCache.addStaticXStmt(
		"select listname from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("SELECT listname FROM movies where dbmovie_id = ?", false)
	globalCache.addStaticXStmt("select rootpath from movies where id = ?", false)
	globalCache.addStaticXStmt("select quality_profile from movies where id = ?", false)
	globalCache.addStaticXStmt(
		"select movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
		false,
	)
	globalCache.addStaticXStmt("update movies set missing = ? where id = ?", false)
	globalCache.addStaticXStmt(
		"update movies set lastscan = datetime('now','localtime') where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"update movies SET missing = 0, quality_reached = ? where id = ?",
		false,
	)
	globalCache.addStaticXStmt("update movies set rootpath = ? where id = ?", false)
	globalCache.addStaticXStmt("update movies set missing = 0 where id = ?", false)
	globalCache.addStaticXStmt("update movies set quality_reached = ? where id = ?", false)
	globalCache.addStaticXStmt("update movies set quality_reached = 1 where id = ?", false)
	globalCache.addStaticXStmt("update movies set quality_reached = 0 where id = ?", false)
	globalCache.addStaticXStmt("update movies set missing = 1 where id = ?", false)
	globalCache.addStaticXStmt(
		"Insert into movies (missing, listname, dbmovie_id, quality_profile) values (1, ?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt("select count() from series where dbserie_id = ?", false)
	globalCache.addStaticXStmt("select count() from series", false)
	globalCache.addStaticXStmt(
		"select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select count() from series", false)
	globalCache.addStaticXStmt(
		"select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select id from series where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search from series where id = ?",
		false,
	)
	globalCache.addStaticXStmt("select dbserie_id from series where id = ?", false)
	globalCache.addStaticXStmt(
		"select dbserie_id, rootpath, listname from series where id = ?",
		false,
	)
	globalCache.addStaticXStmt("select lower(listname), dbserie_id, id from series", false)
	globalCache.addStaticXStmt("select lower(listname), id from series where dbserie_id = ?", false)
	globalCache.addStaticXStmt("select listname from series where id = ?", false)
	globalCache.addStaticXStmt(
		"select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)",
		false,
	)
	globalCache.addStaticXStmt("select rootpath from series where id = ?", false)
	globalCache.addStaticXStmt("select rootpath from series where id = ?", false)
	globalCache.addStaticXStmt("update series SET listname = ?, dbserie_id = ? where id = ?", false)
	globalCache.addStaticXStmt(
		"update series SET aliases=?, search_specials=?, dont_search=?, dont_upgrade=? where dbserie_id = ? and listname = ?",
		false,
	)
	globalCache.addStaticXStmt("update series set rootpath = ? where id = ?", false)
	globalCache.addStaticXStmt(
		"Insert into series (dbserie_id, listname, rootpath, aliases, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt("select count() from serie_episodes where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select id, quality_reached, quality_profile from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt(
		"select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id from serie_episodes where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select serie_episodes.dont_search, serie_episodes.dont_upgrade, series.listname, serie_episodes.quality_profile, dbseries.seriename from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id where serie_episodes.id = ?",
		false,
	)
	globalCache.addStaticXStmt("select dbserie_episode_id from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt("select dbserie_id from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt("select serie_id from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt("select quality_profile from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt("select quality_profile from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt(
		"select quality_profile from serie_episodes where serie_id = ? and quality_profile != '' and quality_profile is not NULL limit 1",
		false,
	)
	globalCache.addStaticXStmt(
		"select dbserie_episode_id, serie_id from serie_episodes where dbserie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?",
		false,
	)
	globalCache.addStaticXStmt("select ignore_runtime from serie_episodes where id = ?", false)
	globalCache.addStaticXStmt("update serie_episodes set missing = ? where id = ?", false)
	globalCache.addStaticXStmt(
		"update serie_episodes set lastscan = datetime('now','localtime') where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"update serie_episodes SET missing = 0, quality_reached = ? where id = ?",
		false,
	)
	globalCache.addStaticXStmt("update serie_episodes set missing = 0 where id = ?", false)
	globalCache.addStaticXStmt("update serie_episodes set quality_reached = ? where id = ?", false)
	globalCache.addStaticXStmt("update serie_episodes set quality_profile = ? where id = ?", false)
	globalCache.addStaticXStmt("update Serie_episodes set quality_reached = 0 where id = ?", false)
	globalCache.addStaticXStmt("update Serie_episodes set quality_reached = 1 where id = ?", false)
	globalCache.addStaticXStmt("update serie_episodes set missing = 1 where id = ?", false)
	globalCache.addStaticXStmt(
		"Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, 1, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt("select count() from dbseries", false)
	globalCache.addStaticXStmt("select count() from dbseries where thetvdb_id = ?", false)
	globalCache.addStaticXStmt("select count() from dbseries", false)
	globalCache.addStaticXStmt("select id from dbseries where thetvdb_id = ?", false)
	globalCache.addStaticXStmt("select id from dbseries where slug = ?", false)
	globalCache.addStaticXStmt("select id from dbseries where seriename = ? COLLATE NOCASE", false)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,seriename,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select id,created_at,updated_at,seriename,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?",
		false,
	)
	globalCache.addStaticXStmt("select lower(identifiedby) from dbseries where id = ?", false)
	globalCache.addStaticXStmt("select 0, seriename, slug from dbseries where id = ?", false)
	globalCache.addStaticXStmt("select thetvdb_id from dbseries where id = ?", false)
	globalCache.addStaticXStmt("select runtime from dbseries where id = ?", false)
	globalCache.addStaticXStmt("select seriename, slug, '', 0, id from dbseries", false)
	globalCache.addStaticXStmt("select seriename from dbseries where id = ?", false)
	globalCache.addStaticXStmt(
		"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20",
		false,
	)
	globalCache.addStaticXStmt(
		"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0",
		false,
	)
	globalCache.addStaticXStmt(
		"update dbseries SET Seriename = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt(
		"select count() from (select distinct title, slug from dbserie_alternates where dbserie_id = ? and title != '')",
		false,
	)
	globalCache.addStaticXStmt("select count() from dbserie_alternates", false)
	globalCache.addStaticXStmt("select count() from dbserie_alternates  where title != ''", false)
	globalCache.addStaticXStmt("select count() from dbserie_alternates where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select count() from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select dbserie_id from dbserie_alternates where slug = ?", false)
	globalCache.addStaticXStmt(
		"select dbserie_id from dbserie_alternates where title = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct title, slug, dbserie_id from dbserie_alternates where dbserie_id = ? and title != ''",
		false,
	)
	globalCache.addStaticXStmt("select title from dbserie_alternates where dbserie_id = ?", false)
	globalCache.addStaticXStmt(
		"select title, id from dbserie_alternates where dbserie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select title, slug, dbserie_id from dbserie_alternates where title != ''",
		false,
	)
	globalCache.addStaticXStmt(
		"delete from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into dbserie_alternates (title, slug, dbserie_id) values (?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt("select count() from dbmovie_titles", false)
	globalCache.addStaticXStmt("select count() from dbmovie_titles where title != ''", false)
	globalCache.addStaticXStmt(
		"select count() from (select distinct title, slug from dbmovie_titles where dbmovie_id = ? and title != '')",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from dbmovie_titles where dbmovie_id = ? and title = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("select count() from dbmovie_titles where dbmovie_id = ?", false)
	globalCache.addStaticXStmt(
		"select title from dbmovie_titles where dbmovie_id = ? limit 1",
		false,
	)
	globalCache.addStaticXStmt(
		"select title, slug, dbmovie_id from dbmovie_titles where title != ''",
		false,
	)
	globalCache.addStaticXStmt("select title, slug from dbmovie_titles where dbmovie_id = ?", false)
	globalCache.addStaticXStmt("select dbmovie_id from dbmovie_titles where slug = ?", false)
	globalCache.addStaticXStmt(
		"select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select distinct title, slug, dbmovie_id from dbmovie_titles where dbmovie_id = ? and title != ''",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt(
		"select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select last_id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt(
		"select id from r_sshistories where config = ? COLLATE NOCASE and list = ? COLLATE NOCASE and indexer = ? COLLATE NOCASE",
		false,
	)
	globalCache.addStaticXStmt("update r_sshistories set last_id = ? where id = ?", false)
	globalCache.addStaticXStmt(
		"insert into r_sshistories (config, list, indexer, last_id) values (?, ?, ?, ?)",
		false,
	)

	globalCache.addStaticXStmt("select count() from serie_episode_files where location = ?", false)
	globalCache.addStaticXStmt("select count() from serie_episode_files", false)
	globalCache.addStaticXStmt(
		"select count() from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from serie_episode_files where serie_episode_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from serie_episode_files where serie_episode_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select count() from serie_episode_files where location = ? and serie_episode_id = ?",
		false,
	)
	globalCache.addStaticXStmt("select location from serie_episode_files", false)
	globalCache.addStaticXStmt(
		"select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt(
		"select location, id, serie_episode_id from serie_episode_files",
		false,
	)
	globalCache.addStaticXStmt(
		"select location, id from serie_episode_files where serie_episode_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select location, serie_episode_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from serie_episode_files where serie_episode_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt("delete from serie_episode_files where id = ?", false)
	globalCache.addStaticXStmt(
		"delete from serie_episode_files where serie_id = ? and location = ?",
		false,
	)

	globalCache.addStaticXStmt("select count() from movie_files", false)
	globalCache.addStaticXStmt("select count() from movie_files where location = ?", false)
	globalCache.addStaticXStmt(
		"select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt("select count() from movie_files where movie_id = ?", false)
	globalCache.addStaticXStmt("select location from movie_files", false)
	globalCache.addStaticXStmt(
		"select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		false,
	)
	globalCache.addStaticXStmt("select location, id, movie_id from movie_files", false)
	globalCache.addStaticXStmt("select location, id from movie_files where movie_id = ?", false)
	globalCache.addStaticXStmt(
		"select location, movie_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from movie_files where movie_id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		false,
	)
	globalCache.addStaticXStmt("delete from movie_files where movie_id = ? and location = ?", false)
	globalCache.addStaticXStmt("delete from movie_files where id = ?", false)

	globalCache.addStaticXStmt(
		"update job_histories set ended = datetime('now','localtime') where id = ?",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))",
		false,
	)
	globalCache.addStaticXStmt(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))",
		false,
	)
}

func SetVarsType(isType uint, media *config.MediaTypeConfig) {
	switch isType {
	case config.MediaTypeMovie:
		{
			for _, cfgplist := range media.ListsMap {
				globalCache.addStaticXStmt(
					logger.JoinStrings(
						"select count() from movies where listname in (?",
						cfgplist.IgnoreMapListsQu,
						") and dbmovie_id = ?",
					),
					false,
				)
				globalCache.addStaticXStmt(
					logger.JoinStrings(
						"select count() from movies where listname in (?",
						cfgplist.IgnoreMapListsQu,
						") and dbmovie_id = ?",
					),
					false,
				)
			}
		}

	case config.MediaTypeSeries:
		{
			globalCache.addStaticXStmt(
				logger.JoinStrings(
					"select id, dbserie_id from series where listname in (?",
					media.ListsQu,
					") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where ((serie_episodes.missing=1 and series.search_specials=1) or (serie_episodes.missing=1 and dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1 ORDER BY RANDOM() limit 20",
				),
				false,
			)
			globalCache.addStaticXStmt(
				logger.JoinStrings(
					"select id, dbserie_id from series where listname in (?",
					media.ListsQu,
					") and (select count() from serie_episodes inner join dbserie_episodes on dbserie_episodes.id = serie_episodes.dbserie_episode_id and serie_episodes.dbserie_id=series.dbserie_id where (series.search_specials=1 or (dbserie_episodes.season != '0' and series.search_specials=0)) and serie_episodes.serie_id = series.id) >= 1",
				),
				false,
			)
		}

	case config.MediaTypeBook:
		{
			for _, cfgplist := range media.ListsMap {
				globalCache.addStaticXStmt(
					logger.JoinStrings(
						"select count() from books where listname in (?",
						cfgplist.IgnoreMapListsQu,
						") and dbbook_id = ?",
					),
					false,
				)
			}
		}

	case config.MediaTypeAudiobook:
		{
			for _, cfgplist := range media.ListsMap {
				globalCache.addStaticXStmt(
					logger.JoinStrings(
						"select count() from audiobooks where listname in (?",
						cfgplist.IgnoreMapListsQu,
						") and dbaudiobook_id = ?",
					),
					false,
				)
			}
		}

	case config.MediaTypeMusic:
		{
			for _, cfgplist := range media.ListsMap {
				globalCache.addStaticXStmt(
					logger.JoinStrings(
						"select count() from albums where listname in (?",
						cfgplist.IgnoreMapListsQu,
						") and dbalbum_id = ?",
					),
					false,
				)
			}
		}
	}
}

// Upgrade handles upgrading the database by calling UpgradeDB.
func Upgrade(_ *gin.Context) {
	UpgradeDB()
}

// ApiPopulateSlugs handles populating slugs for existing records via API call.
// Supports force query parameter to update all slugs, not just empty ones.
func ApiPopulateSlugs(ctx *gin.Context) {
	force := ctx.Query("force") == "true"
	PopulateAllSlugs(force)
}

// PopulateAllSlugs updates slug fields for all tables that have slugs.
// If force is true, updates all records; otherwise only those with empty slugs.
func PopulateAllSlugs(force bool) {
	// Movies and series tables
	populateTableSlugs("dbmovies", "title", force)
	populateTableSlugs("dbmovie_titles", "title", force)
	populateTableSlugs("dbseries", "seriename", force)
	populateTableSlugs("dbserie_alternates", "title", force)

	// Books tables
	populateTableSlugs("dbbooks", "title", force)
	populateTableSlugs("dbbook_titles", "title", force)

	// Audiobooks tables
	populateTableSlugs("dbaudiobooks", "title", force)
	populateTableSlugs("dbaudiobook_titles", "title", force)

	// Music tables
	populateTableSlugs("dbalbums", "title", force)
	populateTableSlugs("dbalbum_titles", "title", force)

	// Authors/artists/series tables (from migration 23)
	PopulateSlugs(force)
}

// backupdb backs up the database to the specified backupPath. It acquires a
// read/write lock before performing the backup to ensure consistency. If an
// error occurs during the backup, it is returned.
func backupdb(backupPath *string) error {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()

	_, err := dbData.ExecContext(sqlCTX, "VACUUM INTO ?", backupPath)

	return err
}

// Backup the database. If db is nil, then uses the existing database
// connection.
func Backup(backupPath *string, maxbackups int) error {
	err := backupdb(backupPath)
	if err != nil {
		logger.Logtype("error", 1).
			Str("query", "VACUUM INTO ?").
			Err(err).
			Msg("exec")

		return err
	}

	logger.Logtype("info", 0).
		Msg("End db backup")

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

	var (
		tu time.Time
		t  time.Time
	)

	for idx := range files {
		if files[idx].IsDir() {
			continue
		}

		if !logger.HasPrefixI(files[idx].Name(), "data.db.") {
			continue
		}

		t = timeFromName(files[idx].Name(), "data.db.")
		if !t.Equal(tu) {
			backupFiles = append(backupFiles, backupInfo{timestamp: t, path: files[idx].Name()})
			continue
		}
	}

	if maxbackups == 0 || maxbackups >= len(backupFiles) {
		return nil
	}

	slices.SortFunc(backupFiles, func(a, b backupInfo) int {
		if a.timestamp.Equal(b.timestamp) {
			return 0
		}

		if logger.TimeAfter(a.timestamp, b.timestamp) {
			return -1
		}

		return 1
	})

	a := backupFiles[maxbackups:]
	for idx := range a {
		os.Remove(filepath.Join("./backup", a[idx].path))
	}

	return nil
}

// UpgradeDB initializes the database schema and upgrades to the latest version.
// It returns an error if migration fails.
func UpgradeDB() error {
	m, err := migrate.New(
		"file://./schema/db",
		"sqlite://./databases/data.db?_fk=1&_cslike=0",
	)
	if err != nil {
		return err
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	versionAfter, _, _ := m.Version()

	DBVersion = strconv.FormatInt(
		int64(versionAfter),
		10,
	)

	return nil
}

// PopulateSlugs updates empty slug fields for existing records in
// dbauthors, dbbook_series, dbartists, and dbartist_aliases tables.
// If force is true, updates all slugs regardless of whether they're empty.
// Note: Caller must hold readWriteMu lock when calling via API.
func PopulateSlugs(force bool) {
	// Populate slugs for dbauthors
	populateTableSlugs("dbauthors", "name", force)

	// Populate slugs for dbbook_series
	populateTableSlugs("dbbook_series", "name", force)

	// Populate slugs for dbartists
	populateTableSlugs("dbartists", "name", force)

	// Populate slugs for dbartist_aliases
	populateTableSlugs("dbartist_aliases", "alias", force)
}

// populateTableSlugs updates slug fields for a specific table.
// If force is true, updates all records; otherwise only those with empty slugs.
// Uses GetrowsNuncached and ExecN for consistency with other database operations.
func populateTableSlugs(tableName, nameColumn string, force bool) {
	var query string
	if force {
		query = fmt.Sprintf("SELECT %s as str, id as num FROM %s", nameColumn, tableName)
	} else {
		query = fmt.Sprintf(
			"SELECT %s as str, id as num FROM %s WHERE slug = '' OR slug IS NULL",
			nameColumn,
			tableName,
		)
	}

	records := GetrowsNuncached[DbstaticOneStringOneUInt](0, query, nil)
	if len(records) == 0 {
		return
	}

	updateQuery := fmt.Sprintf("UPDATE %s SET slug = ? WHERE id = ?", tableName)

	var count int

	for idx := range records {
		slug := logger.StringToSlugCached(records[idx].Str)
		if slug == "" {
			continue
		}

		ExecN(updateQuery, slug, &records[idx].Num)

		count++
	}

	if count > 0 {
		logger.Logtype("info", 0).
			Str("table", tableName).
			Int("count", count).
			Msg("Populated slugs for existing records")
	}
}

// UpgradeIMDB migrates the imdb database to the latest version. It initializes
// a database migration manager, pointing it to the migration scripts. It then
// runs the Up() method to apply any necessary changes. Any errors are printed.
func UpgradeIMDB() {
	m, err := migrate.New(
		"file://./schema/imdbdb",
		"sqlite://./databases/imdb.db?_fk=1&_mutex=no&_cslike=0",
	)
	if err != nil {
		logger.Logtype("error", 1).Err(err).Msg("migration failed")
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		logger.Logtype("error", 1).Err(err).Msg("error syncing database")
	}
}

// timeFromName parses a filename to extract a timestamp, given a known prefix.
// It returns a zero Time if parsing fails.
func timeFromName(filename, prefix string) time.Time {
	if !logger.HasPrefixI(filename, prefix) {
		return time.Time{}
	}

	idx := strings.Index(filename[len(prefix):], logger.StrDot)
	if idx != -1 {
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

// GetMediaQualityConfig returns the QualityConfig from cfgp for the
// media with the given ID. It first checks if there is a quality profile
// set for that media in the database. If not, it returns the default
// QualityConfig from cfgp.
func GetMediaQualityConfig(cfgp *config.MediaTypeConfig, mediaid *uint) *config.QualityConfig {
	return cfgp.GetMediaQualityConfigStr(
		Getdatarow[string](
			false,
			mtstrings.GetStringsMap(cfgp.IsType, logger.DBQualityMediaByID),
			mediaid,
		),
	)
}

// GetMediaListIDGetListname returns the index of the media list with the given name
// in cfgp for the movie with the given ID. It returns -1 if cfgp is nil,
// listname is empty, or no list with that name exists.
func GetMediaListIDGetListname(cfgp *config.MediaTypeConfig, mediaid *uint) int {
	if cfgp == nil {
		logger.Logtype("error", 0).
			Msg("the config couldnt be found")
		return -1
	}

	if *mediaid == 0 {
		return -1
	}

	if config.GetSettingsGeneral().UseMediaCache {
		id := cfgp.GetMediaListsEntryListID(
			CacheOneStringTwoIntIndexFuncStr(cfgp.IsType, logger.CacheMedia, *mediaid),
		)
		if id >= 0 {
			return id
		}

		return -1
	}

	return cfgp.GetMediaListsEntryListID(
		Getdatarow[string](
			false,
			mtstrings.GetStringsMap(cfgp.IsType, logger.DBListnameByMediaID),
			mediaid,
		),
	)
}

// GetDBStaticTwoStringIdx1 returns the index of the DbstaticTwoString element
// with Str1 equal to v, or -1 if not found.
func GetDBStaticTwoStringIdx1(tbl []DbstaticTwoString, v string) int {
	for idx := range tbl {
		if tbl[idx].Str1 == v || strings.EqualFold(tbl[idx].Str1, v) {
			return idx
		}
	}

	return -1
}

// GetDBStaticOneStringOneIntIdx searches tbl for an element where Str equals v, and returns
// the index of that element, or -1 if not found.
func GetDBStaticOneStringOneIntIdx(tbl []DbstaticOneStringOneUInt, v string) int {
	for idx := range tbl {
		if tbl[idx].Str == v || strings.EqualFold(tbl[idx].Str, v) {
			return idx
		}
	}

	return -1
}

func GetSettingTemplatesFor(key string) map[string][]string {
	// Get a thread-safe copy of DBConnect
	dbConnect := GetDBConnect()

	out := make(map[string][]string)

	switch key {
	case "quality":
		out["options"] = make([]string, 0, len(dbConnect.QualityStrIn))
		out["options"] = append(out["options"], "")
		out["options"] = append(out["options"], dbConnect.QualityStrIn...)

	case "resolution":
		out["options"] = make([]string, 0, len(dbConnect.ResolutionStrIn))
		out["options"] = append(out["options"], "")
		out["options"] = append(out["options"], dbConnect.ResolutionStrIn...)

	case "audio":
		out["options"] = make([]string, 0, len(dbConnect.AudioStrIn))
		out["options"] = append(out["options"], "")
		out["options"] = append(out["options"], dbConnect.AudioStrIn...)

	case "codec":
		out["options"] = make([]string, 0, len(dbConnect.CodecStrIn))
		out["options"] = append(out["options"], "")
		out["options"] = append(out["options"], dbConnect.CodecStrIn...)

	default:
		return nil
	}

	return out
}

// types
// have
// movie: dbmovies + titles, movies
// series: dbseries + episodes + alternates, series + episodes - match single episodes to a series

// maybe
// books: dbauthors + dbseries + dbbooks [key: isbn or if not there some other identifier], authors + series + books - difference to series: the possibility to not track all books of an author (track book only + track series only) - possible source for implementation: https://gitlab.com/LazyLibrarian/LazyLibrarian
// audiobooks: dbauthors + dbseries + dbaudiobooks + (+tracks but only for organize and as live query) [asin from audible], authors + series + audiobooks - difference to series: the possibility to not track all audiobooks of an author (track audiobook only + track series only + track all albums of author) - audiobooks contains multiple files (tracks) and the files need to be matched to tracks and maybe resorted to match the runtimes - a max different calculation is needed with an option to define strength for author, album, title, track, length - software https://github.com/beetbox/beets can be used as inspiration and https://github.com/Neurrone/beets-audible for matching and metadata - if possible with the option to enable a tag-writer
// music: dbauthors + dbseries + dbalbums (on album can have different releases like in musicbrainz) + (+tracks but only for organize and as live query) (+format column), authors+series+albums - difference to series: the possibility to not track all albums of an author (track album only + track series only + track all albums of author) - albums contains multiple files (tracks) and the files need to be matched to tracks and maybe resorted to match the runtimes - a max different calculation is needed with an option to define strength for author, album, title, track, length - software https://github.com/beetbox/beets can be used as inspiration - if possible with the option to enable a tag-writer
// needed for music and audiobooks: (references from beets) fetchart, Chromaprint/Acoustid, Discogs, musicbrainz, audible, goodreads, embedart, autotagger
// for movies: maybe a trailer downloader and a subtitle downloader
// the idea would be to use the authors and series for all - but the series/dbseries can be empty - its just needed for naming and for searching series only - but we need then for series and authors an option to define if we want to search books, audiobooks and/or music
