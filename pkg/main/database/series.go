package database

import (
	"database/sql"
	"time"
)

type Serie struct {
	Listname       string    `displayname:"Configuration List" comment:"Configuration list name"`
	Rootpath       string    `displayname:"Storage Path" comment:"Series storage directory"`
	CreatedAt      time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt      time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID             uint      `displayname:"Series ID" comment:"Unique series identifier"`
	DbserieID      uint      `db:"dbserie_id" displayname:"Database Reference" comment:"Database series reference"`
	DontUpgrade    bool      `db:"dont_upgrade" displayname:"Upgrades Disabled" comment:"Disable quality upgrades"`
	DontSearch     bool      `db:"dont_search" displayname:"Search Disabled" comment:"Disable new searches"`
	SearchSpecials bool      `db:"search_specials" displayname:"Include Specials" comment:"Include season zero"`
	IgnoreRuntime  bool      `db:"ignore_runtime" displayname:"Skip Runtime Check" comment:"Skip runtime validation"`
}
type SerieEpisode struct {
	QualityProfile   string       `db:"quality_profile" displayname:"Quality Settings" comment:"Episode quality settings"`
	Lastscan         sql.NullTime `displayname:"Last Scanned" comment:"Last scan timestamp"`
	CreatedAt        time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt        time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	DbserieEpisodeID uint         `db:"dbserie_episode_id" displayname:"Episode Reference" comment:"Database episode reference"`
	SerieID          uint         `db:"serie_id" displayname:"Parent Series" comment:"Parent series reference"`
	DbserieID        uint         `db:"dbserie_id" displayname:"Series Reference" comment:"Database series reference"`
	ID               uint         `displayname:"Episode ID" comment:"Unique episode identifier"`
	Blacklisted      bool         `displayname:"Is Blacklisted" comment:"Episode is blacklisted"`
	QualityReached   bool         `db:"quality_reached" displayname:"Quality Target Met" comment:"Target quality achieved"`
	Missing          bool         `displayname:"Is Missing" comment:"Episode is missing"`
	DontUpgrade      bool         `db:"dont_upgrade" displayname:"Upgrades Disabled" comment:"Disable quality upgrades"`
	DontSearch       bool         `db:"dont_search" displayname:"Search Disabled" comment:"Disable new searches"`
	IgnoreRuntime    bool         `db:"ignore_runtime" displayname:"Skip Runtime Check" comment:"Skip runtime validation"`
}
type SerieFileUnmatched struct {
	Listname    string       `displayname:"Configuration List" comment:"Configuration list name"`
	Filepath    string       `displayname:"File Location" comment:"Unmatched file location"`
	ParsedData  string       `db:"parsed_data" displayname:"Parse Results" comment:"File parsing results"`
	LastChecked sql.NullTime `db:"last_checked" displayname:"Last Check" comment:"Last check timestamp"`
	CreatedAt   time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt   time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID          uint         `displayname:"Record ID" comment:"Unique record identifier"`
}
type SerieEpisodeFile struct {
	Location         string    `displayname:"File Path" comment:"File storage path"`
	Filename         string    `displayname:"File Name" comment:"File name only"`
	Extension        string    `displayname:"File Type" comment:"File extension type"`
	QualityProfile   string    `db:"quality_profile" displayname:"Quality Settings" comment:"File quality settings"`
	CreatedAt        time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt        time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID               uint      `displayname:"File ID" comment:"Unique file identifier"`
	ResolutionID     uint      `db:"resolution_id" displayname:"Video Resolution" comment:"Video resolution reference"`
	QualityID        uint      `db:"quality_id" displayname:"Source Quality" comment:"Quality type reference"`
	CodecID          uint      `db:"codec_id" displayname:"Video Codec" comment:"Video codec reference"`
	AudioID          uint      `db:"audio_id" displayname:"Audio Codec" comment:"Audio codec reference"`
	SerieID          uint      `db:"serie_id" displayname:"Parent Series" comment:"Parent series reference"`
	SerieEpisodeID   uint      `db:"serie_episode_id" displayname:"Episode Link" comment:"Episode reference"`
	DbserieEpisodeID uint      `db:"dbserie_episode_id" displayname:"Episode Reference" comment:"Database episode reference"`
	DbserieID        uint      `db:"dbserie_id" displayname:"Series Reference" comment:"Database series reference"`
	Height           uint16    `displayname:"Video Height" comment:"Video height pixels"`
	Width            uint16    `displayname:"Video Width" comment:"Video width pixels"`
	Proper           bool      `displayname:"Proper Release" comment:"Proper release flag"`
	Extended         bool      `displayname:"Extended Cut" comment:"Extended cut flag"`
	Repack           bool      `displayname:"Repack Release" comment:"Repack release flag"`
}
type SerieEpisodeHistory struct {
	Title            string    `displayname:"Release Title" comment:"Release title name"`
	URL              string    `displayname:"Download URL" comment:"Download source URL"`
	Indexer          string    `displayname:"Source Indexer" comment:"Source indexer name"`
	SerieType        string    `db:"type" displayname:"Media Type" comment:"Series category type"`
	Target           string    `displayname:"Target Path" comment:"Download target path"`
	QualityProfile   string    `db:"quality_profile" displayname:"Quality Settings" comment:"Quality settings used"`
	CreatedAt        time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt        time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	DownloadedAt     time.Time `db:"downloaded_at" displayname:"Download Date" comment:"Download completion timestamp"`
	ID               uint      `displayname:"History ID" comment:"Unique history identifier"`
	ResolutionID     uint      `db:"resolution_id" displayname:"Video Resolution" comment:"Video resolution reference"`
	QualityID        uint      `db:"quality_id" displayname:"Source Quality" comment:"Quality type reference"`
	CodecID          uint      `db:"codec_id" displayname:"Video Codec" comment:"Video codec reference"`
	AudioID          uint      `db:"audio_id" displayname:"Audio Codec" comment:"Audio codec reference"`
	SerieID          uint      `db:"serie_id" displayname:"Parent Series" comment:"Parent series reference"`
	SerieEpisodeID   uint      `db:"serie_episode_id" displayname:"Episode Link" comment:"Episode reference"`
	DbserieEpisodeID uint      `db:"dbserie_episode_id" displayname:"Episode Reference" comment:"Database episode reference"`
	DbserieID        uint      `db:"dbserie_id" displayname:"Series Reference" comment:"Database series reference"`
	Blacklisted      bool      `displayname:"Is Blacklisted" comment:"Entry is blacklisted"`
}

type ResultSeries struct {
	Dbserie
	Listname  string `displayname:"Configuration List" comment:"Configuration list name"`
	Rootpath  string `displayname:"Storage Path" comment:"Series storage directory"`
	DbserieID uint   `db:"dbserie_id" displayname:"Series Reference" comment:"Database series reference"`
}
type Dbserie struct {
	Seriename       string    `displayname:"Series Title" comment:"Primary series title"`
	Aliases         string    `displayname:"Alternative Names" comment:"Alternative series names"`
	Season          string    `displayname:"Current Season" comment:"Current season info"`
	Status          string    `displayname:"Airing Status" comment:"Series airing status"`
	Firstaired      string    `displayname:"First Air Date" comment:"Original air date"`
	Network         string    `displayname:"TV Network" comment:"Broadcasting network name"`
	Runtime         string    `displayname:"Episode Runtime" comment:"Episode duration minutes"`
	Language        string    `displayname:"Primary Language" comment:"Primary series language"`
	Genre           string    `displayname:"Genre Category" comment:"Series genre classification"`
	Overview        string    `displayname:"Plot Summary" comment:"Series plot summary"`
	Rating          string    `displayname:"Content Rating" comment:"Content rating level"`
	Siterating      string    `displayname:"User Rating" comment:"External site rating"`
	SiteratingCount string    `db:"siterating_count" displayname:"Rating Votes" comment:"Rating vote count"`
	Slug            string    `displayname:"URL Slug" comment:"URL friendly identifier"`
	ImdbID          string    `db:"imdb_id" displayname:"IMDB Identifier" comment:"IMDB database identifier"`
	FreebaseMID     string    `db:"freebase_m_id" displayname:"Freebase Machine ID" comment:"Freebase machine identifier"`
	FreebaseID      string    `db:"freebase_id" displayname:"Freebase Identifier" comment:"Freebase database identifier"`
	Facebook        string    `displayname:"Facebook Page" comment:"Facebook page URL"`
	Instagram       string    `displayname:"Instagram Profile" comment:"Instagram profile URL"`
	Twitter         string    `displayname:"Twitter Profile" comment:"Twitter profile URL"`
	Banner          string    `displayname:"Banner Image" comment:"Series banner image"`
	Poster          string    `displayname:"Poster Image" comment:"Series poster image"`
	Fanart          string    `displayname:"Fanart Image" comment:"Series fanart image"`
	Identifiedby    string    `displayname:"ID Method" comment:"Episode identification method"`
	CreatedAt       time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt       time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	TraktID         int       `db:"trakt_id" displayname:"Trakt Identifier" comment:"Trakt database identifier"`
	ThetvdbID       int       `db:"thetvdb_id" displayname:"TVDB Identifier" comment:"TheTVDB database identifier"`
	TvrageID        int       `db:"tvrage_id" displayname:"TVRage Identifier" comment:"TVRage database identifier"`
	ID              uint      `displayname:"Series ID" comment:"Unique series identifier"`
}

type DbserieAlternate struct {
	Title     string    `displayname:"Alternative Title" comment:"Alternative series title"`
	Slug      string    `displayname:"URL Slug" comment:"URL friendly identifier"`
	Region    string    `displayname:"Regional Code" comment:"Title regional variant"`
	CreatedAt time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID        uint      `displayname:"Alternate ID" comment:"Unique alternate identifier"`
	DbserieID uint      `db:"dbserie_id" displayname:"Series Reference" comment:"Parent series reference"`
}

type ResultSerieEpisodes struct {
	DbserieEpisode
	Listname         string       `displayname:"Configuration List" comment:"Configuration list name"`
	Rootpath         string       `displayname:"Storage Path" comment:"Series storage directory"`
	QualityProfile   string       `db:"quality_profile" displayname:"Quality Settings" comment:"Episode quality settings"`
	Lastscan         sql.NullTime `displayname:"Last Scanned" comment:"Last scan timestamp"`
	DbserieEpisodeID uint         `db:"dbserie_episode_id" displayname:"Episode Reference" comment:"Database episode reference"`
	Blacklisted      bool         `displayname:"Is Blacklisted" comment:"Episode is blacklisted"`
	QualityReached   bool         `db:"quality_reached" displayname:"Quality Target Met" comment:"Target quality achieved"`
	Missing          bool         `displayname:"Is Missing" comment:"Episode is missing"`
}

type DbserieEpisode struct {
	Episode    string       `displayname:"Episode Number" comment:"Episode number identifier"`
	Season     string       `displayname:"Season Number" comment:"Season number identifier"`
	Identifier string       `displayname:"Episode Identifier" comment:"Unique episode identifier"`
	Title      string       `displayname:"Episode Title" comment:"Episode title name"`
	Overview   string       `displayname:"Episode Summary" comment:"Episode plot summary"`
	Poster     string       `displayname:"Episode Poster" comment:"Episode poster image"`
	FirstAired sql.NullTime `db:"first_aired" json:"first_aired" time_format:"2006-01-02" time_utc:"1" displayname:"Original Air Date" comment:"Original air date"`
	CreatedAt  time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt  time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	Runtime    int          `displayname:"Episode Duration" comment:"Episode duration minutes"`
	ID         uint         `displayname:"Episode ID" comment:"Unique episode identifier"`
	DbserieID  uint         `db:"dbserie_id" displayname:"Series Reference" comment:"Parent series reference"`
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
