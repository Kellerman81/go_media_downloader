package database

import (
	"database/sql"
	"time"
)

type Serie struct {
	Listname       string    `comment:"Configuration list name"                             displayname:"Configuration List"`
	Rootpath       string    `comment:"Series storage directory"                            displayname:"Storage Path"`
	Aliases        string    `comment:"Alternative series names (language/config specific)" displayname:"Alternative Names"`
	CreatedAt      time.Time `comment:"Record creation timestamp"                           displayname:"Date Created"       db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"                         displayname:"Last Updated"       db:"updated_at"`
	ID             uint      `comment:"Unique series identifier"                            displayname:"Series ID"`
	DbserieID      uint      `comment:"Database series reference"                           displayname:"Database Reference" db:"dbserie_id"`
	DontUpgrade    bool      `comment:"Disable quality upgrades"                            displayname:"Upgrades Disabled"  db:"dont_upgrade"`
	DontSearch     bool      `comment:"Disable new searches"                                displayname:"Search Disabled"    db:"dont_search"`
	SearchSpecials bool      `comment:"Include season zero"                                 displayname:"Include Specials"   db:"search_specials"`
	IgnoreRuntime  bool      `comment:"Skip runtime validation"                             displayname:"Skip Runtime Check" db:"ignore_runtime"`
}
type SerieEpisode struct {
	QualityProfile   string       `comment:"Episode quality settings"    db:"quality_profile"    displayname:"Quality Settings"`
	Lastscan         sql.NullTime `comment:"Last scan timestamp"                                 displayname:"Last Scanned"`
	CreatedAt        time.Time    `comment:"Record creation timestamp"   db:"created_at"         displayname:"Date Created"`
	UpdatedAt        time.Time    `comment:"Last modification timestamp" db:"updated_at"         displayname:"Last Updated"`
	DbserieEpisodeID uint         `comment:"Database episode reference"  db:"dbserie_episode_id" displayname:"Episode Reference"`
	SerieID          uint         `comment:"Parent series reference"     db:"serie_id"           displayname:"Parent Series"`
	DbserieID        uint         `comment:"Database series reference"   db:"dbserie_id"         displayname:"Series Reference"`
	ID               uint         `comment:"Unique episode identifier"                           displayname:"Episode ID"`
	Blacklisted      bool         `comment:"Episode is blacklisted"                              displayname:"Is Blacklisted"`
	QualityReached   bool         `comment:"Target quality achieved"     db:"quality_reached"    displayname:"Quality Target Met"`
	Missing          bool         `comment:"Episode is missing"                                  displayname:"Is Missing"`
	DontUpgrade      bool         `comment:"Disable quality upgrades"    db:"dont_upgrade"       displayname:"Upgrades Disabled"`
	DontSearch       bool         `comment:"Disable new searches"        db:"dont_search"        displayname:"Search Disabled"`
	IgnoreRuntime    bool         `comment:"Skip runtime validation"     db:"ignore_runtime"     displayname:"Skip Runtime Check"`
}
type SerieFileUnmatched struct {
	Listname    string       `comment:"Configuration list name"     displayname:"Configuration List"`
	Filepath    string       `comment:"Unmatched file location"     displayname:"File Location"`
	ParsedData  string       `comment:"File parsing results"        displayname:"Parse Results"      db:"parsed_data"`
	LastChecked sql.NullTime `comment:"Last check timestamp"        displayname:"Last Check"         db:"last_checked"`
	CreatedAt   time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID          uint         `comment:"Unique record identifier"    displayname:"Record ID"`
}
type SerieEpisodeFile struct {
	Location         string    `comment:"File storage path"           displayname:"File Path"`
	Filename         string    `comment:"File name only"              displayname:"File Name"`
	Extension        string    `comment:"File extension type"         displayname:"File Type"`
	QualityProfile   string    `comment:"File quality settings"       displayname:"Quality Settings"  db:"quality_profile"`
	CreatedAt        time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt        time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID               uint      `comment:"Unique file identifier"      displayname:"File ID"`
	ResolutionID     uint      `comment:"Video resolution reference"  displayname:"Video Resolution"  db:"resolution_id"`
	QualityID        uint      `comment:"Quality type reference"      displayname:"Source Quality"    db:"quality_id"`
	CodecID          uint      `comment:"Video codec reference"       displayname:"Video Codec"       db:"codec_id"`
	AudioID          uint      `comment:"Audio codec reference"       displayname:"Audio Codec"       db:"audio_id"`
	SerieID          uint      `comment:"Parent series reference"     displayname:"Parent Series"     db:"serie_id"`
	SerieEpisodeID   uint      `comment:"Episode reference"           displayname:"Episode Link"      db:"serie_episode_id"`
	DbserieEpisodeID uint      `comment:"Database episode reference"  displayname:"Episode Reference" db:"dbserie_episode_id"`
	DbserieID        uint      `comment:"Database series reference"   displayname:"Series Reference"  db:"dbserie_id"`
	Height           uint16    `comment:"Video height pixels"         displayname:"Video Height"`
	Width            uint16    `comment:"Video width pixels"          displayname:"Video Width"`
	Proper           bool      `comment:"Proper release flag"         displayname:"Proper Release"`
	Extended         bool      `comment:"Extended cut flag"           displayname:"Extended Cut"`
	Repack           bool      `comment:"Repack release flag"         displayname:"Repack Release"`
}
type SerieEpisodeHistory struct {
	Title            string    `comment:"Release title name"            displayname:"Release Title"`
	URL              string    `comment:"Download source URL"           displayname:"Download URL"`
	Indexer          string    `comment:"Source indexer name"           displayname:"Source Indexer"`
	SerieType        string    `comment:"Series category type"          displayname:"Media Type"        db:"type"`
	Target           string    `comment:"Download target path"          displayname:"Target Path"`
	QualityProfile   string    `comment:"Quality settings used"         displayname:"Quality Settings"  db:"quality_profile"`
	CreatedAt        time.Time `comment:"Record creation timestamp"     displayname:"Date Created"      db:"created_at"`
	UpdatedAt        time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"      db:"updated_at"`
	DownloadedAt     time.Time `comment:"Download completion timestamp" displayname:"Download Date"     db:"downloaded_at"`
	ID               uint      `comment:"Unique history identifier"     displayname:"History ID"`
	ResolutionID     uint      `comment:"Video resolution reference"    displayname:"Video Resolution"  db:"resolution_id"`
	QualityID        uint      `comment:"Quality type reference"        displayname:"Source Quality"    db:"quality_id"`
	CodecID          uint      `comment:"Video codec reference"         displayname:"Video Codec"       db:"codec_id"`
	AudioID          uint      `comment:"Audio codec reference"         displayname:"Audio Codec"       db:"audio_id"`
	SerieID          uint      `comment:"Parent series reference"       displayname:"Parent Series"     db:"serie_id"`
	SerieEpisodeID   uint      `comment:"Episode reference"             displayname:"Episode Link"      db:"serie_episode_id"`
	DbserieEpisodeID uint      `comment:"Database episode reference"    displayname:"Episode Reference" db:"dbserie_episode_id"`
	DbserieID        uint      `comment:"Database series reference"     displayname:"Series Reference"  db:"dbserie_id"`
	Blacklisted      bool      `comment:"Entry is blacklisted"          displayname:"Is Blacklisted"`
}

type ResultSeries struct {
	Dbserie
	Listname  string `comment:"Configuration list name"   displayname:"Configuration List"`
	Rootpath  string `comment:"Series storage directory"  displayname:"Storage Path"`
	Aliases   string `comment:"Alternative series names"  displayname:"Alternative Names"`
	DbserieID uint   `comment:"Database series reference" displayname:"Series Reference"   db:"dbserie_id"`
}
type Dbserie struct {
	Seriename       string    `comment:"Primary series title"          displayname:"Series Title"`
	Season          string    `comment:"Current season info"           displayname:"Current Season"`
	Status          string    `comment:"Series airing status"          displayname:"Airing Status"`
	Firstaired      string    `comment:"Original air date"             displayname:"First Air Date"`
	Network         string    `comment:"Broadcasting network name"     displayname:"TV Network"`
	Runtime         string    `comment:"Episode duration minutes"      displayname:"Episode Runtime"`
	Language        string    `comment:"Primary series language"       displayname:"Primary Language"`
	Genre           string    `comment:"Series genre classification"   displayname:"Genre Category"`
	Overview        string    `comment:"Series plot summary"           displayname:"Plot Summary"`
	Rating          string    `comment:"Content rating level"          displayname:"Content Rating"`
	Siterating      string    `comment:"External site rating"          displayname:"User Rating"`
	SiteratingCount string    `comment:"Rating vote count"             displayname:"Rating Votes"        db:"siterating_count"`
	Slug            string    `comment:"URL friendly identifier"       displayname:"URL Slug"`
	ImdbID          string    `comment:"IMDB database identifier"      displayname:"IMDB Identifier"     db:"imdb_id"`
	FreebaseMID     string    `comment:"Freebase machine identifier"   displayname:"Freebase Machine ID" db:"freebase_m_id"`
	FreebaseID      string    `comment:"Freebase database identifier"  displayname:"Freebase Identifier" db:"freebase_id"`
	Facebook        string    `comment:"Facebook page URL"             displayname:"Facebook Page"`
	Instagram       string    `comment:"Instagram profile URL"         displayname:"Instagram Profile"`
	Twitter         string    `comment:"Twitter profile URL"           displayname:"Twitter Profile"`
	Banner          string    `comment:"Series banner image"           displayname:"Banner Image"`
	Poster          string    `comment:"Series poster image"           displayname:"Poster Image"`
	Fanart          string    `comment:"Series fanart image"           displayname:"Fanart Image"`
	Identifiedby    string    `comment:"Episode identification method" displayname:"ID Method"`
	CreatedAt       time.Time `comment:"Record creation timestamp"     displayname:"Date Created"        db:"created_at"`
	UpdatedAt       time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"        db:"updated_at"`
	TraktID         int       `comment:"Trakt database identifier"     displayname:"Trakt Identifier"    db:"trakt_id"`
	ThetvdbID       int       `comment:"TheTVDB database identifier"   displayname:"TVDB Identifier"     db:"thetvdb_id"`
	TvrageID        int       `comment:"TVRage database identifier"    displayname:"TVRage Identifier"   db:"tvrage_id"`
	ID              uint      `comment:"Unique series identifier"      displayname:"Series ID"`
}

type DbserieAlternate struct {
	Title     string    `comment:"Alternative series title"    displayname:"Alternative Title"`
	Slug      string    `comment:"URL friendly identifier"     displayname:"URL Slug"`
	Region    string    `comment:"Title regional variant"      displayname:"Regional Code"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID        uint      `comment:"Unique alternate identifier" displayname:"Alternate ID"`
	DbserieID uint      `comment:"Parent series reference"     displayname:"Series Reference"  db:"dbserie_id"`
}

type ResultSerieEpisodes struct {
	DbserieEpisode
	Listname         string       `comment:"Configuration list name"    displayname:"Configuration List"`
	Rootpath         string       `comment:"Series storage directory"   displayname:"Storage Path"`
	QualityProfile   string       `comment:"Episode quality settings"   displayname:"Quality Settings"   db:"quality_profile"`
	Lastscan         sql.NullTime `comment:"Last scan timestamp"        displayname:"Last Scanned"`
	DbserieEpisodeID uint         `comment:"Database episode reference" displayname:"Episode Reference"  db:"dbserie_episode_id"`
	Blacklisted      bool         `comment:"Episode is blacklisted"     displayname:"Is Blacklisted"`
	QualityReached   bool         `comment:"Target quality achieved"    displayname:"Quality Target Met" db:"quality_reached"`
	Missing          bool         `comment:"Episode is missing"         displayname:"Is Missing"`
}

type DbserieEpisode struct {
	Episode         string       `comment:"Episode number identifier"   displayname:"Episode Number"`
	Season          string       `comment:"Season number identifier"    displayname:"Season Number"`
	Identifier      string       `comment:"Unique episode identifier"   displayname:"Episode Identifier"`
	Title           string       `comment:"Episode title name"          displayname:"Episode Title"`
	Overview        string       `comment:"Episode plot summary"        displayname:"Episode Summary"`
	Poster          string       `comment:"Episode poster image"        displayname:"Episode Poster"`
	FirstAired      sql.NullTime `comment:"Original air date"           displayname:"Original Air Date"  db:"first_aired"      json:"first_aired" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt       time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt       time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	Runtime         int          `comment:"Episode duration minutes"    displayname:"Episode Duration"`
	AbsoluteEpisode int          `comment:"Absolute episode number"     displayname:"Absolute Episode"   db:"absolute_episode"`
	ID              uint         `comment:"Unique episode identifier"   displayname:"Episode ID"`
	DbserieID       uint         `comment:"Parent series reference"     displayname:"Series Reference"   db:"dbserie_id"`
}

// GetDbserieByIDP retrieves a Dbserie by ID. It takes a uint ID
// and a pointer to a Dbserie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// dbserie data and scan it into the Dbserie struct.
// Returns an error if there was a problem retrieving the data.
func (s *Dbserie) GetDbserieByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,seriename,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id from dbseries where id = ?",
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
		"select id,created_at,updated_at,listname,rootpath,aliases,dbserie_id,dont_upgrade,dont_search from series where id = ?",
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

// InsertEpisodeFile inserts an episode file record into the database.
func InsertEpisodeFile(
	serieEpisodeID uint,
	location, filename, extension string,
	resolutionID, qualityID, codecID, audioID uint,
) error {
	if serieEpisodeID == 0 {
		return nil
	}

	var (
		proper, repack, extended             bool
		serieID, dbserieID, dbserieEpisodeID uint
		height, width                        int
		qualityProfile                       string
	)

	// Get related IDs from serie_episode

	Scanrowsdyn(
		false,
		"SELECT serie_id, dbserie_id, dbserie_episode_id FROM serie_episodes WHERE id = ?",
		&serieID,
		&serieEpisodeID,
	)
	Scanrowsdyn(false, "SELECT dbserie_id, dbserie_episode_id FROM serie_episodes WHERE id = ?",
		&dbserieID, &serieEpisodeID)
	Scanrowsdyn(false, "SELECT dbserie_episode_id FROM serie_episodes WHERE id = ?",
		&dbserieEpisodeID, &serieEpisodeID)

	ExecN(
		"insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&location,
		&filename,
		&extension,
		&qualityProfile,
		&resolutionID,
		&qualityID,
		&codecID,
		&audioID,
		&proper,
		&repack,
		&extended,
		&serieID,
		&serieEpisodeID,
		&dbserieEpisodeID,
		&dbserieID,
		&height,
		&width,
	)

	return nil
}
