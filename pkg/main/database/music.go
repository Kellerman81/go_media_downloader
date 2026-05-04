package database

import (
	"database/sql"
	"slices"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Dbartist represents a music artist in the database (metadata table).
type Dbartist struct {
	Name           string    `comment:"Primary artist name"             displayname:"Artist Name"`
	SortName       string    `comment:"Name for sorting"                displayname:"Sort Name"      db:"sort_name"`
	MusicbrainzID  string    `comment:"MusicBrainz identifier"          displayname:"MusicBrainz ID" db:"musicbrainz_id"`
	DiscogsID      string    `comment:"Discogs identifier"              displayname:"Discogs ID"     db:"discogs_id"`
	SpotifyID      string    `comment:"Spotify identifier"              displayname:"Spotify ID"     db:"spotify_id"`
	ArtistType     string    `comment:"Type (person, group, orchestra)" displayname:"Artist Type"    db:"artist_type"`
	Country        string    `comment:"Artist country of origin"        displayname:"Country"`
	Disambiguation string    `comment:"Disambiguation text"             displayname:"Disambiguation"`
	Bio            string    `comment:"Artist biography"                displayname:"Biography"`
	ImageURL       string    `comment:"Artist image URL"                displayname:"Artist Image"   db:"image_url"`
	Genres         string    `comment:"Artist genres (JSON array)"      displayname:"Genres"`
	BeginDate      string    `comment:"Career/formation start date"     displayname:"Begin Date"     db:"begin_date"`
	EndDate        string    `comment:"Career/dissolution end date"     displayname:"End Date"       db:"end_date"`
	CreatedAt      time.Time `comment:"Record creation timestamp"       displayname:"Date Created"   db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"     displayname:"Last Updated"   db:"updated_at"`
	ID             uint      `comment:"Unique artist identifier"        displayname:"Artist ID"`
}

// DbartistAlias represents an alias for an artist.
type DbartistAlias struct {
	Alias      string    `comment:"Artist alias/alternate name"         displayname:"Alias Name"`
	Locale     string    `comment:"Alias locale/language"               displayname:"Locale"`
	AliasType  string    `comment:"Type (legal_name, stage_name, etc.)" displayname:"Alias Type"       db:"alias_type"`
	CreatedAt  time.Time `comment:"Record creation timestamp"           displayname:"Date Created"     db:"created_at"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"         displayname:"Last Updated"     db:"updated_at"`
	ID         uint      `comment:"Unique alias identifier"             displayname:"Alias ID"`
	DbartistID uint      `comment:"Parent artist reference"             displayname:"Artist Reference" db:"dbartist_id"`
	IsPrimary  bool      `comment:"Primary alias flag"                  displayname:"Is Primary"       db:"is_primary"`
}

// Dbalbum represents a music album in the database (metadata table).
type Dbalbum struct {
	Title                     string       `comment:"Primary album title"            displayname:"Album Title"`
	MusicbrainzReleaseGroupID string       `comment:"MusicBrainz release group ID"   displayname:"MB Release Group" db:"musicbrainz_release_group_id"`
	MusicbrainzReleaseID      string       `comment:"MusicBrainz release ID"         displayname:"MB Release ID"    db:"musicbrainz_release_id"`
	DiscogsMasterID           string       `comment:"Discogs master ID"              displayname:"Discogs Master"   db:"discogs_master_id"`
	DiscogsReleaseID          string       `comment:"Discogs release ID"             displayname:"Discogs Release"  db:"discogs_release_id"`
	SpotifyID                 string       `comment:"Spotify identifier"             displayname:"Spotify ID"       db:"spotify_id"`
	UPC                       string       `comment:"Universal Product Code"         displayname:"UPC/Barcode"`
	ReleaseType               string       `comment:"Type (album, ep, single, etc.)" displayname:"Release Type"     db:"release_type"`
	Format                    string       `comment:"Format (cd, vinyl, digital)"    displayname:"Release Format"`
	Label                     string       `comment:"Record label name"              displayname:"Record Label"`
	Country                   string       `comment:"Release country"                displayname:"Release Country"`
	Genres                    string       `comment:"Album genres (JSON array)"      displayname:"Genres"`
	Styles                    string       `comment:"Album styles (JSON array)"      displayname:"Styles"`
	CoverURL                  string       `comment:"Album cover image URL"          displayname:"Cover Image"      db:"cover_url"`
	SeriesName                string       `comment:"Series this album belongs to"   displayname:"Series Name"      db:"series_name"`
	Slug                      string       `comment:"URL friendly identifier"        displayname:"URL Slug"`
	ReleaseDate               sql.NullTime `comment:"Album release date"             displayname:"Release Date"     db:"release_date"                 json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt                 time.Time    `comment:"Record creation timestamp"      displayname:"Date Created"     db:"created_at"`
	UpdatedAt                 time.Time    `comment:"Last modification timestamp"    displayname:"Last Updated"     db:"updated_at"`
	TotalTracks               int          `comment:"Number of tracks"               displayname:"Total Tracks"     db:"total_tracks"`
	TotalRuntimeMs            int64        `comment:"Total runtime in milliseconds"  displayname:"Total Runtime"    db:"total_runtime_ms"`
	ID                        uint         `comment:"Unique album identifier"        displayname:"Album ID"`
	Year                      uint16       `comment:"Release year"                   displayname:"Release Year"`
}

// DbalbumTitle represents an alternate title for an album.
type DbalbumTitle struct {
	Title     string    `comment:"Alternative album title"     displayname:"Alternative Title"`
	Slug      string    `comment:"URL friendly identifier"     displayname:"URL Slug"`
	Region    string    `comment:"Title regional variant"      displayname:"Regional Code"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID        uint      `comment:"Unique title identifier"     displayname:"Title ID"`
	DbalbumID uint      `comment:"Parent album reference"      displayname:"Album Reference"   db:"dbalbum_id"`
}

// DbalbumArtist represents the many-to-many relationship between albums and artists.
type DbalbumArtist struct {
	JoinPhrase string    `comment:"Join phrase (feat., &, etc.)" db:"join_phrase" displayname:"Join Phrase"`
	CreatedAt  time.Time `comment:"Record creation timestamp"    db:"created_at"  displayname:"Date Created"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"  db:"updated_at"  displayname:"Last Updated"`
	ID         uint      `comment:"Unique relation identifier"                    displayname:"Relation ID"`
	DbalbumID  uint      `comment:"Album reference"              db:"dbalbum_id"  displayname:"Album Reference"`
	DbartistID uint      `comment:"Artist reference"             db:"dbartist_id" displayname:"Artist Reference"`
	Position   uint8     `comment:"Display order position"                        displayname:"Display Order"`
}

// Dbtrack represents a music track in the database (metadata table).
type Dbtrack struct {
	Title                  string    `comment:"Primary track title"                   displayname:"Track Title"`
	MusicbrainzRecordingID string    `comment:"MusicBrainz recording ID"              displayname:"MB Recording ID" db:"musicbrainz_recording_id"`
	ISRC                   string    `comment:"International Standard Recording Code" displayname:"ISRC"`
	AcoustID               string    `comment:"Audio fingerprint ID"                  displayname:"AcoustID"        db:"acoustid"`
	CreatedAt              time.Time `comment:"Record creation timestamp"             displayname:"Date Created"    db:"created_at"`
	UpdatedAt              time.Time `comment:"Last modification timestamp"           displayname:"Last Updated"    db:"updated_at"`
	RuntimeMs              int64     `comment:"Track runtime in milliseconds"         displayname:"Track Runtime"   db:"runtime_ms"`
	ID                     uint      `comment:"Unique track identifier"               displayname:"Track ID"`
	DbalbumID              uint      `comment:"Parent album reference"                displayname:"Album Reference" db:"dbalbum_id"`
	DiscNumber             uint16    `comment:"Disc number"                           displayname:"Disc Number"     db:"disc_number"`
	TrackNumber            uint16    `comment:"Track number on disc"                  displayname:"Track Number"    db:"track_number"`
	Explicit               bool      `comment:"Explicit content flag"                 displayname:"Is Explicit"`
}

// DbtrackArtist represents the many-to-many relationship between tracks and artists (for featured artists).
type DbtrackArtist struct {
	Role       string    `comment:"Role (main, featured, remixer)" displayname:"Artist Role"`
	CreatedAt  time.Time `comment:"Record creation timestamp"      displayname:"Date Created"     db:"created_at"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"    displayname:"Last Updated"     db:"updated_at"`
	ID         uint      `comment:"Unique relation identifier"     displayname:"Relation ID"`
	DbtrackID  uint      `comment:"Track reference"                displayname:"Track Reference"  db:"dbtrack_id"`
	DbartistID uint      `comment:"Artist reference"               displayname:"Artist Reference" db:"dbartist_id"`
	Position   uint8     `comment:"Display order position"         displayname:"Display Order"`
}

// Artist represents a tracked artist (user tracking table).
type Artist struct {
	Listname   string    `comment:"Configuration list name"                  displayname:"Configuration List"`
	TrackMode  string    `comment:"Tracking mode (all, albums_only, manual)" displayname:"Track Mode"         db:"track_mode"`
	CreatedAt  time.Time `comment:"Record creation timestamp"                displayname:"Date Created"       db:"created_at"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"              displayname:"Last Updated"       db:"updated_at"`
	ID         uint      `comment:"Unique artist identifier"                 displayname:"Artist ID"`
	DbartistID uint      `comment:"Database artist reference"                displayname:"Database Reference" db:"dbartist_id"`
	DontSearch bool      `comment:"Disable new searches"                     displayname:"Search Disabled"    db:"dont_search"`
}

// Album represents a tracked album (user tracking table).
type Album struct {
	QualityProfile string       `comment:"Album quality settings"      db:"quality_profile" displayname:"Quality Settings"`
	Listname       string       `comment:"Configuration list name"                          displayname:"Configuration List"`
	Rootpath       string       `comment:"Album storage directory"                          displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"                              displayname:"Last Scanned"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"   db:"created_at"      displayname:"Date Created"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp" db:"updated_at"      displayname:"Last Updated"`
	ID             uint         `comment:"Unique album identifier"                          displayname:"Album ID"`
	DbalbumID      uint         `comment:"Database album reference"    db:"dbalbum_id"      displayname:"Database Reference"`
	ArtistID       uint         `comment:"Tracked artist reference"    db:"artist_id"       displayname:"Artist Reference"`
	Blacklisted    bool         `comment:"Album is blacklisted"                             displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"     db:"quality_reached" displayname:"Quality Target Met"`
	Missing        bool         `comment:"Album is missing"                                 displayname:"Is Missing"`
	DontUpgrade    bool         `comment:"Disable quality upgrades"    db:"dont_upgrade"    displayname:"Upgrades Disabled"`
	DontSearch     bool         `comment:"Disable new searches"        db:"dont_search"     displayname:"Search Disabled"`
}

// AlbumFileUnmatched represents an unmatched album/music file.
type AlbumFileUnmatched struct {
	Listname    string       `comment:"Configuration list name"     displayname:"Configuration List"`
	Filepath    string       `comment:"Unmatched file location"     displayname:"File Location"`
	ParsedData  string       `comment:"File parsing results"        displayname:"Parse Results"      db:"parsed_data"`
	LastChecked sql.NullTime `comment:"Last check timestamp"        displayname:"Last Check"         db:"last_checked"`
	CreatedAt   time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID          uint         `comment:"Unique record identifier"    displayname:"Record ID"`
}

// ResultAlbums combines Dbalbum metadata with user tracking data.
type ResultAlbums struct {
	Dbalbum
	Listname       string       `comment:"Configuration list name"  displayname:"Configuration List"`
	QualityProfile string       `comment:"Album quality settings"   displayname:"Quality Settings"   db:"quality_profile"`
	Rootpath       string       `comment:"Album storage directory"  displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"      displayname:"Last Scanned"`
	DbalbumID      uint         `comment:"Database album reference" displayname:"Album Reference"    db:"dbalbum_id"`
	Blacklisted    bool         `comment:"Album is blacklisted"     displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"  displayname:"Quality Target Met" db:"quality_reached"`
	Missing        bool         `comment:"Album is missing"         displayname:"Is Missing"`
}

// AlbumFile represents a downloaded album track file.
type AlbumFile struct {
	Location       string    `comment:"File storage path"              displayname:"File Path"`
	Filename       string    `comment:"File name only"                 displayname:"File Name"`
	Extension      string    `comment:"File extension type"            displayname:"File Type"`
	Format         string    `comment:"Audio format (flac, mp3, etc.)" displayname:"Audio Format"`
	QualityProfile string    `comment:"File quality settings"          displayname:"Quality Settings" db:"quality_profile"`
	AcoustID       string    `comment:"Audio fingerprint ID"           displayname:"AcoustID"         db:"acoustid"`
	CreatedAt      time.Time `comment:"Record creation timestamp"      displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"    displayname:"Last Updated"     db:"updated_at"`
	AlbumID        uint      `comment:"Parent album reference"         displayname:"Parent Album"     db:"album_id"`
	DbalbumID      uint      `comment:"Database album reference"       displayname:"Album Reference"  db:"dbalbum_id"`
	DbtrackID      uint      `comment:"Database track reference"       displayname:"Track Reference"  db:"dbtrack_id"`
	ID             uint      `comment:"Unique file identifier"         displayname:"File ID"`
	FileSize       int64     `comment:"File size in bytes"             displayname:"File Size"        db:"file_size"`
	Bitrate        int       `comment:"Audio bitrate (kbps)"           displayname:"Audio Bitrate"`
	SampleRate     int       `comment:"Audio sample rate (Hz)"         displayname:"Sample Rate"      db:"sample_rate"`
	BitDepth       int       `comment:"Audio bit depth"                displayname:"Bit Depth"        db:"bit_depth"`
	RuntimeMs      int64     `comment:"Track runtime (ms)"             displayname:"Runtime"          db:"runtime_ms"`
	DiscNumber     uint16    `comment:"Disc number"                    displayname:"Disc Number"      db:"disc_number"`
	TrackNumber    uint16    `comment:"Track number"                   displayname:"Track Number"     db:"track_number"`
}

// AlbumHistory represents download history for albums.
type AlbumHistory struct {
	Title          string    `comment:"Release title name"            displayname:"Release Title"`
	URL            string    `comment:"Download source URL"           displayname:"Download URL"`
	Indexer        string    `comment:"Source indexer name"           displayname:"Source Indexer"`
	HistoryType    string    `comment:"Album category type"           displayname:"Media Type"       db:"type"`
	Target         string    `comment:"Download target path"          displayname:"Target Path"`
	QualityProfile string    `comment:"Quality settings used"         displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"     displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"     db:"updated_at"`
	DownloadedAt   time.Time `comment:"Download completion timestamp" displayname:"Download Date"    db:"downloaded_at"`
	ID             uint      `comment:"Unique history identifier"     displayname:"History ID"`
	AlbumID        uint      `comment:"Parent album reference"        displayname:"Parent Album"     db:"album_id"`
	DbalbumID      uint      `comment:"Database album reference"      displayname:"Album Reference"  db:"dbalbum_id"`
	Blacklisted    bool      `comment:"Entry is blacklisted"          displayname:"Is Blacklisted"`
}

// GetDbalbumByIDP retrieves a Dbalbum by ID.
func (album *Dbalbum) GetDbalbumByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,title,musicbrainz_release_group_id,musicbrainz_release_id,discogs_master_id,discogs_release_id,spotify_id,upc,release_date,release_type,format,label,country,total_tracks,total_runtime_ms,genres,styles,cover_url,year,slug from dbalbums where id = ?",
		album,
		id,
	)
}

// GetAlbumsByIDP retrieves an Album by ID.
func (u *Album) GetAlbumsByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbalbum_id,artist_id from albums where id = ?",
		u,
		id,
	)
}

// GetDbartistByIDP retrieves a Dbartist by ID.
func (artist *Dbartist) GetDbartistByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,name,sort_name,musicbrainz_id,discogs_id,spotify_id,artist_type,country,begin_date,end_date,disambiguation,bio,image_url,genres from dbartists where id = ?",
		artist,
		id,
	)
}

// GetDbtrackByIDP retrieves a Dbtrack by ID.
func (track *Dbtrack) GetDbtrackByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,dbalbum_id,musicbrainz_recording_id,isrc,acoustid,title,disc_number,track_number,runtime_ms,explicit from dbtracks where id = ?",
		track,
		id,
	)
}

// GetDbtracksByAlbumID returns all tracks for a given album ID,
// ordered by disc_number, track_number.
func GetDbtracksByAlbumID(albumID uint) []Dbtrack {
	return StructscanT[Dbtrack](
		false,
		Getdatarow[uint](false, "select count() from dbtracks where dbalbum_id = ?", albumID),
		"select id,created_at,updated_at,dbalbum_id,musicbrainz_recording_id,isrc,acoustid,title,disc_number,track_number,runtime_ms,explicit from dbtracks where dbalbum_id = ? order by disc_number, track_number",
		albumID,
	)
}

// DbtrackWithArtist extends Dbtrack with the primary artist name resolved from
// the dbtrack_artists / dbartists join tables.
type DbtrackWithArtist struct {
	Dbtrack
	Artist string `db:"artist"`
}

// GetDbtracksByAlbumIDWithArtist returns all tracks for an album enriched with
// their primary artist name (position=1 in dbtrack_artists), ordered by disc
// then track number. Artist is empty string when no artist row is found.
func GetDbtracksByAlbumIDWithArtist(albumID uint) []DbtrackWithArtist {
	return StructscanT[DbtrackWithArtist](
		false,
		Getdatarow[uint](false, "select count() from dbtracks where dbalbum_id = ?", albumID),
		`SELECT t.id, t.created_at, t.updated_at, t.dbalbum_id,
		        t.musicbrainz_recording_id, t.isrc, t.acoustid,
		        t.title, t.disc_number, t.track_number, t.runtime_ms, t.explicit,
		        COALESCE(a.name, '') AS artist
		   FROM dbtracks t
		   LEFT JOIN dbtrack_artists ta ON ta.dbtrack_id = t.id AND ta.position = 1
		   LEFT JOIN dbartists a ON a.id = ta.dbartist_id
		  WHERE t.dbalbum_id = ?
		  ORDER BY t.disc_number, t.track_number`,
		albumID,
	)
}

// AlbumSearchResult holds the result of searching for an album in the database.
type AlbumSearchResult struct {
	MusicBrainzReleaseID string
	Label                string
	Country              string
	ID                   uint
	TotalTracks          int
	TotalRuntime         int // Runtime in milliseconds
	Year                 int
	Title                string
	Artist               string
}

// FindAlbumByArtistTitle searches for an album by artist and title.
// Returns the best matching album or an error if not found.
// Uses artist-first lookup for better accuracy, with fallback to stripped title.
func FindAlbumByArtistTitle(artist, title *string) (*AlbumSearchResult, error) {
	if title == nil || *title == "" {
		return nil, logger.ErrNotFoundDbalbum
	}

	slug := logger.StringToSlug(*title)
	// Hoist stripped variants — used in both fallback branches; compute once.
	strippedTitle := stripReleaseType(*title)

	var strippedSlug string
	if strippedTitle != *title && strippedTitle != "" {
		strippedSlug = logger.StringToSlug(strippedTitle)
	}

	// If we have an artist, try artist-first lookup (more accurate)
	if artist != nil && *artist != "" {
		artistSlug := logger.StringToSlug(*artist)

		// Find artist IDs
		artistIDs := Getrowssize[uint](
			false,
			"SELECT count() FROM dbartists WHERE name = ? COLLATE NOCASE OR sort_name = ? COLLATE NOCASE OR slug = ?",
			"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR sort_name = ? COLLATE NOCASE OR slug = ?",
			artist,
			artist,
			&artistSlug,
		)

		// Also check artist aliases
		aliasIDs := Getrowssize[uint](
			false,
			"SELECT count() FROM dbartist_aliases WHERE alias = ? COLLATE NOCASE OR slug = ?",
			"SELECT dbartist_id FROM dbartist_aliases WHERE alias = ? COLLATE NOCASE OR slug = ?",
			artist, &artistSlug)

		artistIDs = append(artistIDs, aliasIDs...)

		// Try to find album by this artist
		for artistid := range artistIDs {
			var result AlbumSearchResult

			// Exact match
			Scanrowsdyn(false,
				`SELECT a.id FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
				 LIMIT 1`,
				&result.ID, &artistIDs[artistid], title, &slug)

			if result.ID > 0 {
				// logger.Logtype("debug", 0).
				// 	Str("artist", artist).
				// 	Str("title", title).
				// 	Uint("dbID", result.ID).
				// 	Msg("DEBUG: FindAlbumByArtistTitle - found by artist+title exact match")
				return fillAlbumResult(&result), nil
			}

			// Try alternate titles
			Scanrowsdyn(false,
				`SELECT at.dbalbum_id FROM dbalbum_titles at
				 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
				 LIMIT 1`,
				&result.ID, &artistIDs[artistid], title, &slug)

			if result.ID > 0 {
				// logger.Logtype("debug", 0).
				// 	Str("artist", artist).
				// 	Str("title", title).
				// 	Uint("dbID", result.ID).
				// 	Msg("DEBUG: FindAlbumByArtistTitle - found by artist+alternate title")
				return fillAlbumResult(&result), nil
			}
		}

		// Fallback: try with stripped title (remove release type, year, etc.)
		if strippedTitle != *title && strippedTitle != "" {
			for artistID := range artistIDs {
				var result AlbumSearchResult

				Scanrowsdyn(false,
					`SELECT a.id FROM dbalbums a
					 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
					 LIMIT 1`,
					&result.ID, &artistIDs[artistID], &strippedTitle, &strippedSlug)

				if result.ID > 0 {
					// logger.Logtype("debug", 0).
					// 	Str("artist", artist).
					// 	Str("originalTitle", title).
					// 	Str("strippedTitle", strippedTitle).
					// 	Uint("dbID", result.ID).
					// 	Msg("DEBUG: FindAlbumByArtistTitle - found by artist+stripped title")
					return fillAlbumResult(&result), nil
				}

				// Try alternate titles with stripped version
				Scanrowsdyn(false,
					`SELECT at.dbalbum_id FROM dbalbum_titles at
					 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
					 LIMIT 1`,
					&result.ID, &artistIDs[artistID], &strippedTitle, &strippedSlug)

				if result.ID > 0 {
					// logger.Logtype("debug", 0).
					// 	Str("artist", artist).
					// 	Str("originalTitle", title).
					// 	Str("strippedTitle", strippedTitle).
					// 	Uint("dbID", result.ID).
					// 	Msg("DEBUG: FindAlbumByArtistTitle - found by artist+stripped alternate title")
					return fillAlbumResult(&result), nil
				}
			}
		}

		// Log why we couldn't find it
		// logger.Logtype("debug", 0).
		// 	Str("artist", artist).
		// 	Str("title", title).
		// 	Str("strippedTitle", stripReleaseType(title)).
		// 	Int("artistIDsFound", len(artistIDs)).
		// 	Msg("DEBUG: FindAlbumByArtistTitle - not found with artist")
	}

	// Fallback: search by title only (less accurate)
	var result AlbumSearchResult

	Scanrowsdyn(false,
		"SELECT id FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ? LIMIT 1",
		&result.ID, title, &slug)

	if result.ID > 0 {
		// logger.Logtype("debug", 0).
		// 	Str("title", title).
		// 	Uint("dbID", result.ID).
		// 	Msg("DEBUG: FindAlbumByArtistTitle - found by title only")
		return fillAlbumResult(&result), nil
	}

	// Try stripped title without artist
	if strippedTitle != *title && strippedTitle != "" {
		Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ? LIMIT 1",
			&result.ID, &strippedTitle, &strippedSlug)

		if result.ID > 0 {
			// logger.Logtype("debug", 0).
			// 	Str("originalTitle", title).
			// 	Str("strippedTitle", strippedTitle).
			// 	Uint("dbID", result.ID).
			// 	Msg("DEBUG: FindAlbumByArtistTitle - found by stripped title only")
			return fillAlbumResult(&result), nil
		}
	}

	// logger.Logtype("debug", 0).
	// 	Str("artist", artist).
	// 	Str("title", title).
	// 	Str("strippedTitle", strippedTitle).
	// 	Msg("DEBUG: FindAlbumByArtistTitle - album not found")

	return nil, logger.ErrNotFoundDbalbum
}

// albumResultDedup tracks unique album IDs seen by FindAlbumsByArtistTitle.
// Using a []uint slice sized to maxResults replaces the map[uint]bool (one
// heap allocation instead of two) and is correct for any maxResults value.
type albumResultDedup struct {
	seen []uint
}

func (d *albumResultDedup) add(id uint) bool {
	if slices.Contains(d.seen, id) {
		return false
	}

	d.seen = append(d.seen, id)

	return true
}

// FindAlbumsByArtistTitle returns ALL potential album matches for an artist/title combination.
// This allows callers to try multiple matches and pick the best one (e.g., by track count).
// Returns up to maxResults matches, ordered by best match quality.
func FindAlbumsByArtistTitle(artist, title *string, maxResults int) ([]*AlbumSearchResult, error) {
	if title == nil || *title == "" {
		return nil, logger.ErrNotFoundDbalbum
	}

	if maxResults <= 0 {
		maxResults = 10
	}

	slug := logger.StringToSlug(*title)
	// Hoist stripped variants — used in both fallback branches; compute once.
	strippedTitle := stripReleaseType(*title)

	var strippedSlug string
	if strippedTitle != *title && strippedTitle != "" {
		strippedSlug = logger.StringToSlug(strippedTitle)
	}

	results := make([]*AlbumSearchResult, 0, maxResults)
	dedup := albumResultDedup{seen: make([]uint, 0, maxResults)}

	// If we have an artist, try artist-first lookup (more accurate)
	if artist != nil && *artist != "" {
		artistSlug := logger.StringToSlug(*artist)

		// Find artist IDs
		artistIDs := Getrowssize[uint](
			false,
			"SELECT count() FROM dbartists WHERE name = ? COLLATE NOCASE OR sort_name = ? COLLATE NOCASE OR slug = ?",
			"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR sort_name = ? COLLATE NOCASE OR slug = ?",
			artist,
			artist,
			&artistSlug,
		)

		// Also check artist aliases
		aliasIDs := Getrowssize[uint](
			false,
			"SELECT count() FROM dbartist_aliases WHERE alias = ? COLLATE NOCASE OR slug = ?",
			"SELECT dbartist_id FROM dbartist_aliases WHERE alias = ? COLLATE NOCASE OR slug = ?",
			artist, &artistSlug)

		artistIDs = append(artistIDs, aliasIDs...)

		// Find albums by these artists with matching title
		for artistID := range artistIDs {
			if len(results) >= maxResults {
				break
			}

			// Exact title matches
			ids := Getrowssize[uint](
				false,
				`SELECT count() FROM dbalbums a JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id WHERE aa.dbartist_id = ? AND (a.title = ? COLLATE NOCASE OR a.slug = ?)`,
				`SELECT a.id FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)`,
				&artistIDs[artistID],
				title,
				&slug,
			)

			for i := range ids {
				if ids[i] > 0 && dedup.add(ids[i]) {
					results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
				}
			}

			// Alternate title matches
			if len(results) >= maxResults {
				continue
			}

			ids = Getrowssize[uint](
				false,
				`SELECT count() FROM dbalbum_titles at JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id WHERE aa.dbartist_id = ? AND (at.title = ? COLLATE NOCASE OR at.slug = ?)`,
				`SELECT at.dbalbum_id FROM dbalbum_titles at
				 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)`,
				&artistIDs[artistID],
				title,
				&slug,
			)

			for i := range ids {
				if ids[i] > 0 && dedup.add(ids[i]) {
					results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
				}
			}
		}

		// Try with stripped title (remove release type, year, etc.)
		if strippedTitle != *title && strippedTitle != "" && len(results) < maxResults {
			for artistID := range artistIDs {
				if len(results) >= maxResults {
					break
				}

				ids := Getrowssize[uint](
					false,
					`SELECT count() FROM dbalbums a JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id WHERE aa.dbartist_id = ? AND (a.title = ? COLLATE NOCASE OR a.slug = ?)`,
					`SELECT a.id FROM dbalbums a
					 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)`,
					&artistIDs[artistID],
					&strippedTitle,
					&strippedSlug,
				)

				for i := range ids {
					if ids[i] > 0 && dedup.add(ids[i]) {
						results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
					}
				}

				// Alternate titles with stripped version
				if len(results) >= maxResults {
					continue
				}

				ids = Getrowssize[uint](
					false,
					`SELECT count() FROM dbalbum_titles at JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id WHERE aa.dbartist_id = ? AND (at.title = ? COLLATE NOCASE OR at.slug = ?)`,
					`SELECT at.dbalbum_id FROM dbalbum_titles at
					 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)`,
					&artistIDs[artistID],
					&strippedTitle,
					&strippedSlug,
				)

				for i := range ids {
					if ids[i] > 0 && dedup.add(ids[i]) {
						results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
					}
				}
			}
		}
	}

	// Fallback: search by title only (less accurate)
	if len(results) < maxResults {
		ids := Getrowssize[uint](false,
			"SELECT count() FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ?",
			"SELECT id FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ?",
			title, &slug)

		for i := range ids {
			if ids[i] > 0 && dedup.add(ids[i]) {
				results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
			}
		}
	}

	// Try stripped title without artist
	if strippedTitle != *title && strippedTitle != "" && len(results) < maxResults {
		ids := Getrowssize[uint](false,
			"SELECT count() FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ?",
			"SELECT id FROM dbalbums WHERE title = ? COLLATE NOCASE OR slug = ?",
			&strippedTitle, &strippedSlug)

		for i := range ids {
			if ids[i] > 0 && dedup.add(ids[i]) {
				results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
			}
		}
	}

	// LIKE-based fallback: match partial titles
	// This handles cases like:
	// - "Gods Of War" matching "Gods Of War (Live)"
	// - "Holy Bible" matching "The Holy Bible"
	if len(results) < maxResults && *title != "" {
		likeTitle := logger.JoinStrings(*title, "%")

		// Title starts with our search term
		ids := Getrowssize[uint](false,
			"SELECT count() FROM dbalbums WHERE title LIKE ? COLLATE NOCASE",
			"SELECT id FROM dbalbums WHERE title LIKE ? COLLATE NOCASE",
			&likeTitle)

		for i := range ids {
			if ids[i] > 0 && dedup.add(ids[i]) {
				results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
			}
		}

		// Our search term starts with the database title (parsed has extra junk)
		if len(results) < maxResults {
			ids = Getrowssize[uint](
				false,
				`SELECT count() FROM dbalbums WHERE ? LIKE title || '%' COLLATE NOCASE AND length(title) > 5`,
				`SELECT id FROM dbalbums WHERE ? LIKE title || '%' COLLATE NOCASE AND length(title) > 5`,
				title,
			)

			for i := range ids {
				if ids[i] > 0 && dedup.add(ids[i]) {
					results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
				}
			}
		}

		// Also try with stripped title for LIKE matches
		if strippedTitle != "" && strippedTitle != *title && len(results) < maxResults {
			likeStripped := strippedTitle + "%"

			ids = Getrowssize[uint](false,
				"SELECT count() FROM dbalbums WHERE title LIKE ? COLLATE NOCASE",
				"SELECT id FROM dbalbums WHERE title LIKE ? COLLATE NOCASE",
				&likeStripped)

			for i := range ids {
				if ids[i] > 0 && dedup.add(ids[i]) {
					results = append(results, fillAlbumResult(&AlbumSearchResult{ID: ids[i]}))
				}
			}
		}
	}

	if len(results) == 0 {
		return nil, logger.ErrNotFoundDbalbum
	}

	// logger.Logtype("debug", 0).
	// 	Str("artist", artist).
	// 	Str("title", title).
	// 	Int("matchCount", len(results)).
	// 	Msg("DEBUG: FindAlbumsByArtistTitle - found multiple potential matches")

	return results, nil
}

// FillAlbumResult fills in the AlbumSearchResult with full album data.
func FillAlbumResult(result *AlbumSearchResult) *AlbumSearchResult {
	return fillAlbumResult(result)
}

// fillAlbumResult fills in the AlbumSearchResult with full album data.
func fillAlbumResult(result *AlbumSearchResult) *AlbumSearchResult {
	if result.ID == 0 {
		return result
	}

	var ab Dbalbum
	if err := ab.GetDbalbumByIDP(&result.ID); err == nil {
		result.Title = ab.Title
		result.TotalTracks = ab.TotalTracks
		result.TotalRuntime = int(ab.TotalRuntimeMs)
		result.Year = int(ab.Year)
		result.MusicBrainzReleaseID = ab.MusicbrainzReleaseID
		result.Label = ab.Label
		result.Country = ab.Country
	}

	Scanrowsdyn(
		false,
		"SELECT da.name FROM dbartists da JOIN dbalbum_artists daa ON da.id = daa.dbartist_id WHERE daa.dbalbum_id = ? LIMIT 1",
		&result.Artist,
		&result.ID,
	)

	return result
}

// TrackFileInfo holds the per-track fields required by InsertAlbumFile.
// It mirrors the relevant fields of parser_v2.TrackInfo so that the database
// package does not need to import parser_v2 (which itself imports database).
type TrackFileInfo struct {
	Filepath       string
	Filename       string
	Extension      string
	Format         string
	QualityProfile string
	FileSize       int64
	Bitrate        int
	SampleRate     int
	BitDepth       int
	RuntimeMS      int64
	TrackNumber    int
	DiscNumber     int
}

// InsertAlbumFile records an album file in the database.
func InsertAlbumFile(
	albumID uint,
	track *TrackFileInfo,
	acoustid string,
	dbAlbumID uint,
	dbTrackID uint,
) error {
	if albumID == 0 {
		return nil // Skip if no database ID
	}

	ExecN(
		"INSERT INTO [album_files] ([location],[filename],[extension],[format],[quality_profile],[file_size],[bitrate],[sample_rate],[bit_depth],[runtime_ms],[disc_number],[track_number],[acoustid],[album_id],[dbalbum_id],[dbtrack_id]) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&track.Filepath,
		&track.Filename,
		&track.Extension,
		&track.Format,
		&track.QualityProfile,
		&track.FileSize,
		&track.Bitrate,
		&track.SampleRate,
		&track.BitDepth,
		&track.RuntimeMS,
		&track.DiscNumber,
		&track.TrackNumber,
		&acoustid,
		&albumID,
		&dbAlbumID,
		&dbTrackID,
	)

	return nil
}

// AlbumExistsByRootpath checks if an album with the given rootpath exists
// and has files in the album_files table.
func AlbumExistsByRootpath(rootpath *string) bool {
	if rootpath == nil || *rootpath == "" {
		return false
	}

	if SlicesCacheContainsIFast(config.MediaTypeMusic, logger.CacheRootpath, rootpath) {
		return true
	}

	return Getdatarow[uint](
		false,
		"SELECT a.id FROM albums a WHERE a.rootpath = ? AND EXISTS (SELECT 1 FROM album_files af WHERE af.album_id = a.id) LIMIT 1",
		rootpath,
	) > 0
}

// GetAlbumByRootpath returns album info if it exists with the given rootpath.
func GetAlbumByRootpath(rootpath string) (*Album, error) {
	if rootpath == "" {
		return nil, logger.ErrNotFoundAlbum
	}

	var ab Album
	Scanrowsdyn(
		false,
		"SELECT id, dbalbum_id, listname, rootpath, quality_profile FROM albums WHERE rootpath = ? LIMIT 1",
		&ab.ID,
		&ab.DbalbumID,
		&ab.Listname,
		&ab.Rootpath,
		&ab.QualityProfile,
		&rootpath,
	)

	if ab.ID == 0 {
		return nil, logger.ErrNotFoundAlbum
	}

	return &ab, nil
}

// UpdateAlbumRootpath updates the rootpath for an album.
func UpdateAlbumRootpath(albumID uint, rootpath string) error {
	if albumID == 0 {
		return nil
	}

	ExecN("UPDATE albums SET rootpath = ? WHERE id = ?", &rootpath, &albumID)

	return nil
}

// AlbumFileExists checks if a file already exists in album_files.
func AlbumFileExists(filepath *string) bool {
	if filepath == nil || *filepath == "" {
		return false
	}

	if SlicesCacheContainsIFast(config.MediaTypeMusic, logger.CacheFiles, filepath) {
		return true
	}

	return Getdatarow[uint](false,
		"SELECT id FROM album_files WHERE location = ? LIMIT 1",
		filepath) > 0
}

// GetAlbumListEntryID looks up the correct albums.id for a given dbalbum_id and listname.
// This is needed because album.DatabaseID stores dbalbum_id, not albums.id.
// Multiple list entries can reference the same dbalbum_id, so listname is required.
func GetAlbumListEntryID(dbalbumID uint, listname string) uint {
	if dbalbumID == 0 || listname == "" {
		return 0
	}

	return ScanRowVal2[uint, string, uint](
		"SELECT id FROM albums WHERE dbalbum_id = ? AND listname = ?",
		dbalbumID, listname)
}

// FindAlbumByMusicBrainzID searches for an album by its MusicBrainz release ID.
// Returns the best matching album or an error if not found.
func FindAlbumByMusicBrainzID(musicBrainzID *string) (*AlbumSearchResult, error) {
	if musicBrainzID == nil || *musicBrainzID == "" {
		return nil, logger.ErrNotFoundDbalbum
	}

	var result AlbumSearchResult

	// Query for exact MusicBrainz release ID match
	Scanrowsdyn(
		false,
		"SELECT id FROM dbalbums WHERE musicbrainz_release_id = ? OR musicbrainz_release_group_id = ? LIMIT 1",
		&result.ID,
		musicBrainzID,
		musicBrainzID,
	)

	if result.ID > 0 {
		var ab Dbalbum
		if err := ab.GetDbalbumByIDP(&result.ID); err == nil {
			result.Title = ab.Title
			result.TotalTracks = ab.TotalTracks
			result.TotalRuntime = int(ab.TotalRuntimeMs)
			result.Year = int(ab.Year)
			result.MusicBrainzReleaseID = ab.MusicbrainzReleaseID
			result.Label = ab.Label
			result.Country = ab.Country
		}

		return &result, nil
	}

	return nil, logger.ErrNotFoundDbalbum
}

// FindAlbumByUPC searches for an album by its UPC/barcode.
// Returns the best matching album or an error if not found.
func FindAlbumByUPC(upc *string) (*AlbumSearchResult, error) {
	if upc == nil || *upc == "" {
		return nil, logger.ErrNotFoundDbalbum
	}

	var result AlbumSearchResult

	// Query for exact UPC match
	result.ID = Getdatarow[uint](false,
		"SELECT id FROM dbalbums WHERE upc = ? LIMIT 1",
		upc)

	if result.ID > 0 {
		var ab Dbalbum
		if err := ab.GetDbalbumByIDP(&result.ID); err == nil {
			result.Title = ab.Title
			result.TotalTracks = ab.TotalTracks
			result.TotalRuntime = int(ab.TotalRuntimeMs)
			result.Year = int(ab.Year)
			result.MusicBrainzReleaseID = ab.MusicbrainzReleaseID
			result.Label = ab.Label
			result.Country = ab.Country
		}

		return &result, nil
	}

	return nil, logger.ErrNotFoundDbalbum
}
