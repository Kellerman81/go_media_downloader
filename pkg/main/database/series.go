package database

import (
	"database/sql"
	"time"
)

type Serie struct {
	Listname       string
	Rootpath       string
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	ID             uint
	DbserieID      uint `db:"dbserie_id"`
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
	SearchSpecials bool `db:"search_specials"`
	IgnoreRuntime  bool `db:"ignore_runtime"`
}
type SerieEpisode struct {
	QualityProfile   string `db:"quality_profile"`
	Lastscan         sql.NullTime
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	DbserieEpisodeID uint      `db:"dbserie_episode_id"`
	SerieID          uint      `db:"serie_id"`
	DbserieID        uint      `db:"dbserie_id"`
	ID               uint
	Blacklisted      bool
	QualityReached   bool `db:"quality_reached"`
	Missing          bool
	DontUpgrade      bool `db:"dont_upgrade"`
	DontSearch       bool `db:"dont_search"`
	IgnoreRuntime    bool `db:"ignore_runtime"`
}
type SerieFileUnmatched struct {
	Listname    string
	Filepath    string
	ParsedData  string       `db:"parsed_data"`
	LastChecked sql.NullTime `db:"last_checked"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
	ID          uint
}
type SerieEpisodeFile struct {
	Location         string
	Filename         string
	Extension        string
	QualityProfile   string    `db:"quality_profile"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	ID               uint
	ResolutionID     uint `db:"resolution_id"`
	QualityID        uint `db:"quality_id"`
	CodecID          uint `db:"codec_id"`
	AudioID          uint `db:"audio_id"`
	SerieID          uint `db:"serie_id"`
	SerieEpisodeID   uint `db:"serie_episode_id"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	DbserieID        uint `db:"dbserie_id"`
	Height           uint16
	Width            uint16
	Proper           bool
	Extended         bool
	Repack           bool
}
type SerieEpisodeHistory struct {
	Title            string
	URL              string
	Indexer          string
	SerieType        string `db:"type"`
	Target           string
	QualityProfile   string    `db:"quality_profile"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	DownloadedAt     time.Time `db:"downloaded_at"`
	ID               uint
	ResolutionID     uint `db:"resolution_id"`
	QualityID        uint `db:"quality_id"`
	CodecID          uint `db:"codec_id"`
	AudioID          uint `db:"audio_id"`
	SerieID          uint `db:"serie_id"`
	SerieEpisodeID   uint `db:"serie_episode_id"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	DbserieID        uint `db:"dbserie_id"`
	Blacklisted      bool
}

type ResultSeries struct {
	Dbserie
	Listname  string
	Rootpath  string
	DbserieID uint `db:"dbserie_id"`
}
type Dbserie struct {
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
	ImdbID          string `db:"imdb_id"`
	FreebaseMID     string `db:"freebase_m_id"`
	FreebaseID      string `db:"freebase_id"`
	Facebook        string
	Instagram       string
	Twitter         string
	Banner          string
	Poster          string
	Fanart          string
	Identifiedby    string
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	TraktID         int       `db:"trakt_id"`
	ThetvdbID       int       `db:"thetvdb_id"`
	TvrageID        int       `db:"tvrage_id"`
	ID              uint
}

type DbserieAlternate struct {
	Title     string
	Slug      string
	Region    string
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	ID        uint
	DbserieID uint `db:"dbserie_id"`
}

type ResultSerieEpisodes struct {
	DbserieEpisode
	Listname         string
	Rootpath         string
	QualityProfile   string `db:"quality_profile"`
	Lastscan         sql.NullTime
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	Blacklisted      bool
	QualityReached   bool `db:"quality_reached"`
	Missing          bool
}

type DbserieEpisode struct {
	Episode    string
	Season     string
	Identifier string
	Title      string
	Overview   string
	Poster     string
	FirstAired sql.NullTime `db:"first_aired" json:"first_aired" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt  time.Time    `db:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"`
	Runtime    int
	ID         uint
	DbserieID  uint `db:"dbserie_id"`
}

// GetDbserieByIDP retrieves a Dbserie by ID. It takes a uint ID
// and a pointer to a Dbserie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// dbserie data and scan it into the Dbserie struct.
// Returns an error if there was a problem retrieving the data.
func (s *Dbserie) GetDbserieByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?",
		s,
		id,
	)
}

// GetDbserieEpisodesByIDP retrieves a DbserieEpisode by ID. It takes a uint ID
// and a pointer to a DbserieEpisode struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// dbserie episode data and scan it into the DbserieEpisode struct.
// Returns an error if there was a problem retrieving the data.
func (u *DbserieEpisode) GetDbserieEpisodesByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id from dbserie_episodes where id = ?",
		u,
		id,
	)
}

// GetSerieByIDP retrieves a Serie by ID. It takes a uint ID
// and a pointer to a Serie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// serie data and scan it into the Serie struct.
// Returns an error if there was a problem retrieving the data.
func (u *Serie) GetSerieByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search from series where id = ?",
		u,
		id,
	)
}

// GetSerieEpisodesByIDP retrieves a SerieEpisode by ID. It takes a uint ID
// and a pointer to a SerieEpisode struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// serie episode data and scan it into the SerieEpisode struct.
// Returns an error if there was a problem retrieving the data.
func (u *SerieEpisode) GetSerieEpisodesByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id from serie_episodes where id = ?",
		u,
		id,
	)
}
