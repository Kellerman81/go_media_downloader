package database

import (
	"database/sql"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

type Serie struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Listname       string
	Rootpath       string
	DbserieID      uint `db:"dbserie_id"`
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
	SearchSpecials bool `db:"search_specials"`
	IgnoreRuntime  bool `db:"ignore_runtime"`
}
type SerieEpisode struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Lastscan         sql.NullTime
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DontUpgrade      bool `db:"dont_upgrade"`
	DontSearch       bool `db:"dont_search"`
	IgnoreRuntime    bool `db:"ignore_runtime"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	SerieID          uint `db:"serie_id"`
	DbserieID        uint `db:"dbserie_id"`
}
type SerieFileUnmatched struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Listname    string
	Filepath    string
	LastChecked sql.NullTime `db:"last_checked"`
	ParsedData  string       `db:"parsed_data"`
}
type SerieEpisodeFile struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Location         string
	Filename         string
	Extension        string
	QualityProfile   string `db:"quality_profile"`
	Proper           bool
	Extended         bool
	Repack           bool
	Height           int
	Width            int
	ResolutionID     uint `db:"resolution_id"`
	QualityID        uint `db:"quality_id"`
	CodecID          uint `db:"codec_id"`
	AudioID          uint `db:"audio_id"`
	SerieID          uint `db:"serie_id"`
	SerieEpisodeID   uint `db:"serie_episode_id"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	DbserieID        uint `db:"dbserie_id"`
}
type SerieEpisodeHistory struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Title            string
	URL              string
	Indexer          string
	SerieType        string `db:"type"`
	Target           string
	DownloadedAt     time.Time `db:"downloaded_at"`
	Blacklisted      bool
	QualityProfile   string `db:"quality_profile"`
	ResolutionID     uint   `db:"resolution_id"`
	QualityID        uint   `db:"quality_id"`
	CodecID          uint   `db:"codec_id"`
	AudioID          uint   `db:"audio_id"`
	SerieID          uint   `db:"serie_id"`
	SerieEpisodeID   uint   `db:"serie_episode_id"`
	DbserieEpisodeID uint   `db:"dbserie_episode_id"`
	DbserieID        uint   `db:"dbserie_id"`
}

type ResultSeries struct {
	Dbserie
	Listname  string
	Rootpath  string
	DbserieID uint `db:"dbserie_id"`
}
type Dbserie struct {
	ID              uint
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	Seriename       string
	Aliases         string
	Season          string
	Status          string
	Firstaired      string
	Network         string
	Runtime         string
	Language        string
	Genre           string
	Overview        string
	Rating          string
	Siterating      string
	SiteratingCount string `db:"siterating_count"`
	Slug            string
	TraktID         int    `db:"trakt_id"`
	ImdbID          string `db:"imdb_id"`
	ThetvdbID       int    `db:"thetvdb_id"`
	FreebaseMID     string `db:"freebase_m_id"`
	FreebaseID      string `db:"freebase_id"`
	TvrageID        int    `db:"tvrage_id"`
	Facebook        string
	Instagram       string
	Twitter         string
	Banner          string
	Poster          string
	Fanart          string
	Identifiedby    string
}

type DbserieAlternate struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Title     string
	Slug      string
	Region    string
	DbserieID uint `db:"dbserie_id"`
}

type ResultSerieEpisodes struct {
	DbserieEpisode
	Listname         string
	Rootpath         string
	Lastscan         sql.NullTime
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
}

type DbserieEpisode struct {
	ID         uint
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
	Episode    string
	Season     string
	Identifier string
	Title      string
	FirstAired sql.NullTime `db:"first_aired" json:"first_aired" time_format:"2006-01-02" time_utc:"1"`
	Overview   string
	Poster     string
	Runtime    int
	DbserieID  uint `db:"dbserie_id"`
}

// Close cleans up the Dbserie value after use.
// It nulls out fields to avoid lingering references and aid garbage collection.
// This is useful to do after finishing using a Dbserie value
// that was retrieved from the database.
func (s *Dbserie) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || s == nil {
		return
	}
	*s = Dbserie{}
	//logger.Clear(s)
}
