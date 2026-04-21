package database

import (
	"bytes"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

type Movie struct {
	QualityProfile string       `comment:"Movie quality settings"      db:"quality_profile" displayname:"Quality Settings"`
	Listname       string       `comment:"Configuration list name"                          displayname:"Configuration List"`
	Rootpath       string       `comment:"Movie storage directory"                          displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"                              displayname:"Last Scanned"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"   db:"created_at"      displayname:"Date Created"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp" db:"updated_at"      displayname:"Last Updated"`
	ID             uint         `comment:"Unique movie identifier"                          displayname:"Movie ID"`
	DbmovieID      uint         `comment:"Database movie reference"    db:"dbmovie_id"      displayname:"Database Reference"`
	Blacklisted    bool         `comment:"Movie is blacklisted"                             displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"     db:"quality_reached" displayname:"Quality Target Met"`
	Missing        bool         `comment:"Movie is missing"                                 displayname:"Is Missing"`
	DontUpgrade    bool         `comment:"Disable quality upgrades"    db:"dont_upgrade"    displayname:"Upgrades Disabled"`
	DontSearch     bool         `comment:"Disable new searches"        db:"dont_search"     displayname:"Search Disabled"`
}

type MovieFileUnmatched struct {
	Listname    string       `comment:"Configuration list name"     displayname:"Configuration List"`
	Filepath    string       `comment:"Unmatched file location"     displayname:"File Location"`
	ParsedData  string       `comment:"File parsing results"        displayname:"Parse Results"      db:"parsed_data"`
	LastChecked sql.NullTime `comment:"Last check timestamp"        displayname:"Last Check"         db:"last_checked"`
	CreatedAt   time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID          uint         `comment:"Unique record identifier"    displayname:"Record ID"`
}

type ResultMovies struct {
	Dbmovie
	Listname       string       `comment:"Configuration list name"  displayname:"Configuration List"`
	QualityProfile string       `comment:"Movie quality settings"   displayname:"Quality Settings"   db:"quality_profile"`
	Rootpath       string       `comment:"Movie storage directory"  displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"      displayname:"Last Scanned"`
	DbmovieID      uint         `comment:"Database movie reference" displayname:"Movie Reference"    db:"dbmovie_id"`
	Blacklisted    bool         `comment:"Movie is blacklisted"     displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"  displayname:"Quality Target Met" db:"quality_reached"`
	Missing        bool         `comment:"Movie is missing"         displayname:"Is Missing"`
}

type MovieFile struct {
	Location       string    `comment:"File storage path"           displayname:"File Path"`
	Filename       string    `comment:"File name only"              displayname:"File Name"`
	Extension      string    `comment:"File extension type"         displayname:"File Type"`
	QualityProfile string    `comment:"File quality settings"       displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"   displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp" displayname:"Last Updated"     db:"updated_at"`
	ResolutionID   uint      `comment:"Video resolution reference"  displayname:"Video Resolution" db:"resolution_id"`
	QualityID      uint      `comment:"Quality type reference"      displayname:"Source Quality"   db:"quality_id"`
	CodecID        uint      `comment:"Video codec reference"       displayname:"Video Codec"      db:"codec_id"`
	AudioID        uint      `comment:"Audio codec reference"       displayname:"Audio Codec"      db:"audio_id"`
	MovieID        uint      `comment:"Parent movie reference"      displayname:"Parent Movie"     db:"movie_id"`
	DbmovieID      uint      `comment:"Database movie reference"    displayname:"Movie Reference"  db:"dbmovie_id"`
	ID             uint      `comment:"Unique file identifier"      displayname:"File ID"`
	Height         uint16    `comment:"Video height pixels"         displayname:"Video Height"`
	Width          uint16    `comment:"Video width pixels"          displayname:"Video Width"`
	Proper         bool      `comment:"Proper release flag"         displayname:"Proper Release"`
	Extended       bool      `comment:"Extended cut flag"           displayname:"Extended Cut"`
	Repack         bool      `comment:"Repack release flag"         displayname:"Repack Release"`
}

type MovieHistory struct {
	Title          string    `comment:"Release title name"            displayname:"Release Title"`
	URL            string    `comment:"Download source URL"           displayname:"Download URL"`
	Indexer        string    `comment:"Source indexer name"           displayname:"Source Indexer"`
	HistoryType    string    `comment:"Movie category type"           displayname:"Media Type"       db:"type"`
	Target         string    `comment:"Download target path"          displayname:"Target Path"`
	QualityProfile string    `comment:"Quality settings used"         displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"     displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"     db:"updated_at"`
	DownloadedAt   time.Time `comment:"Download completion timestamp" displayname:"Download Date"    db:"downloaded_at"`
	ID             uint      `comment:"Unique history identifier"     displayname:"History ID"`
	ResolutionID   uint      `comment:"Video resolution reference"    displayname:"Video Resolution" db:"resolution_id"`
	QualityID      uint      `comment:"Quality type reference"        displayname:"Source Quality"   db:"quality_id"`
	CodecID        uint      `comment:"Video codec reference"         displayname:"Video Codec"      db:"codec_id"`
	AudioID        uint      `comment:"Audio codec reference"         displayname:"Audio Codec"      db:"audio_id"`
	MovieID        uint      `comment:"Parent movie reference"        displayname:"Parent Movie"     db:"movie_id"`
	DbmovieID      uint      `comment:"Database movie reference"      displayname:"Movie Reference"  db:"dbmovie_id"`
	Blacklisted    bool      `comment:"Entry is blacklisted"          displayname:"Is Blacklisted"`
}

type Dbmovie struct {
	Genres           string       `comment:"Movie genre classification"   displayname:"Genre Categories"`
	OriginalLanguage string       `comment:"Movie original language"      displayname:"Original Language"   db:"original_language"`
	OriginalTitle    string       `comment:"Movie original title"         displayname:"Original Title"      db:"original_title"`
	Overview         string       `comment:"Movie plot summary"           displayname:"Plot Summary"`
	Title            string       `comment:"Primary movie title"          displayname:"Movie Title"`
	SpokenLanguages  string       `comment:"Languages spoken in movie"    displayname:"Spoken Languages"    db:"spoken_languages"`
	Status           string       `comment:"Movie release status"         displayname:"Release Status"`
	Tagline          string       `comment:"Movie promotional tagline"    displayname:"Movie Tagline"`
	ImdbID           string       `comment:"IMDB database identifier"     displayname:"IMDB Identifier"     db:"imdb_id"`
	FreebaseMID      string       `comment:"Freebase machine identifier"  displayname:"Freebase Machine ID" db:"freebase_m_id"`
	FreebaseID       string       `comment:"Freebase database identifier" displayname:"Freebase Identifier" db:"freebase_id"`
	FacebookID       string       `comment:"Facebook page identifier"     displayname:"Facebook ID"         db:"facebook_id"`
	InstagramID      string       `comment:"Instagram profile identifier" displayname:"Instagram ID"        db:"instagram_id"`
	TwitterID        string       `comment:"Twitter profile identifier"   displayname:"Twitter ID"          db:"twitter_id"`
	URL              string       `comment:"Movie information URL"        displayname:"Movie URL"`
	Backdrop         string       `comment:"Movie backdrop image"         displayname:"Backdrop Image"`
	Poster           string       `comment:"Movie poster image"           displayname:"Poster Image"`
	Slug             string       `comment:"URL friendly identifier"      displayname:"URL Slug"`
	ReleaseDate      sql.NullTime `comment:"Movie release date"           displayname:"Release Date"        db:"release_date"      json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt        time.Time    `comment:"Record creation timestamp"    displayname:"Date Created"        db:"created_at"`
	UpdatedAt        time.Time    `comment:"Last modification timestamp"  displayname:"Last Updated"        db:"updated_at"`
	Popularity       float32      `comment:"Movie popularity rating"      displayname:"Popularity Score"`
	VoteAverage      float32      `comment:"Average user rating"          displayname:"User Rating"         db:"vote_average"`
	Budget           int          `comment:"Movie production budget"      displayname:"Production Budget"`
	Revenue          int          `comment:"Movie box office revenue"     displayname:"Box Office"`
	Runtime          int          `comment:"Movie runtime minutes"        displayname:"Movie Duration"`
	TraktID          int          `comment:"Trakt database identifier"    displayname:"Trakt Identifier"    db:"trakt_id"`
	MoviedbID        int          `comment:"MovieDB database identifier"  displayname:"MovieDB Identifier"  db:"moviedb_id"`
	ID               uint         `comment:"Unique movie identifier"      displayname:"Movie ID"`
	VoteCount        int32        `comment:"Number of user votes"         displayname:"Rating Votes"        db:"vote_count"`
	Year             uint16       `comment:"Movie release year"           displayname:"Release Year"`
	Adult            bool         `comment:"Adult content flag"           displayname:"Adult Content"`
}

type DbmovieTitle struct {
	Title     string    `comment:"Alternative movie title"     displayname:"Alternative Title"`
	Slug      string    `comment:"URL friendly identifier"     displayname:"URL Slug"`
	Region    string    `comment:"Title regional variant"      displayname:"Regional Code"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID        uint      `comment:"Unique title identifier"     displayname:"Title ID"`
	DbmovieID uint      `comment:"Parent movie reference"      displayname:"Movie Reference"   db:"dbmovie_id"`
}

// MovieGetImdbMetadata fetches movie metadata from IMDB.
// It takes a pointer to a Dbmovie struct and a bool indicating
// whether to overwrite existing data.
// It adds the "tt" prefix to the IMDB ID if missing, fetches
// the title from the IMDB API, and clears temporary variables.
func (movie *Dbmovie) MovieGetImdbMetadata(overwrite bool) error {
	if movie.ImdbID == "" {
		return nil
	}

	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)

	return movie.GetImdbTitle(overwrite)
}

// GetImdbTitle queries the imdb_titles table to populate movie details from IMDb.
// It takes the IMDb id pointer, movie struct pointer, and a boolean overwrite flag.
// It will populate the movie struct with data from IMDb if fields are empty or overwrite is true.
// This handles setting the title, year, adult flag, genres, original title, runtime, slug, url,
// vote average, and vote count.
func (movie *Dbmovie) GetImdbTitle(overwrite bool) error {
	if movie.ImdbID == "" {
		return nil
	}

	var imdbdata ImdbTitle

	err := GetdatarowArgsImdb(
		"select primary_title, start_year, is_adult, genres, original_title, runtime_minutes, slug from imdb_titles where tconst = ?",
		&movie.ImdbID,
		&imdbdata.PrimaryTitle,
		&imdbdata.StartYear,
		&imdbdata.IsAdult,
		&imdbdata.Genres,
		&imdbdata.OriginalTitle,
		&imdbdata.RuntimeMinutes,
		&imdbdata.Slug,
	)
	if err != nil {
		return err
	}

	if (movie.Title == "" || overwrite) && imdbdata.PrimaryTitle != "" {
		movie.Title = imdbdata.PrimaryTitle
	}

	if (movie.Year == 0 || overwrite) && imdbdata.StartYear != 0 {
		movie.Year = imdbdata.StartYear
	}

	if (!movie.Adult && imdbdata.IsAdult) || overwrite {
		movie.Adult = imdbdata.IsAdult
	}

	if (movie.Genres == "" || overwrite) && imdbdata.Genres != "" {
		movie.Genres = imdbdata.Genres
	}

	if (movie.OriginalTitle == "" || overwrite) && imdbdata.OriginalTitle != "" {
		movie.OriginalTitle = imdbdata.OriginalTitle
	}

	if (movie.Runtime == 0 || movie.Runtime == 1 || movie.Runtime == 2 || movie.Runtime == 3 || movie.Runtime == 4 || movie.Runtime == 60 || movie.Runtime == 90 || movie.Runtime == 120 || overwrite) &&
		imdbdata.RuntimeMinutes != 0 {
		if movie.Runtime != 0 &&
			(imdbdata.RuntimeMinutes == 1 || imdbdata.RuntimeMinutes == 2 || imdbdata.RuntimeMinutes == 3 || imdbdata.RuntimeMinutes == 4) {
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, movie.ImdbID).
				Msg("skipped imdb movie runtime for")
		} else {
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, movie.ImdbID).
				Msg("set imdb movie runtime for")

			movie.Runtime = imdbdata.RuntimeMinutes
		}
	}

	if (movie.Slug == "" || overwrite) && imdbdata.Slug != "" {
		movie.Slug = imdbdata.Slug
	}

	if movie.URL == "" || overwrite {
		movie.URL = logger.JoinStrings("https://www.imdb.com/title/", movie.ImdbID)
	}

	movie.GetImdbRating(overwrite)

	return nil
}

// GetDbmovieByIDP retrieves a Dbmovie by ID. It takes a uint ID and a
// pointer to a Dbmovie struct to scan the result into. It executes a SQL query
// using the structscan function to select the dbmovie data and scan it into
// the Dbmovie struct. Returns an error if there was a problem retrieving the data.
func (movie *Dbmovie) GetDbmovieByIDP(id *uint) error {
	err := structscan1(logger.DBMovieDetails, movie, id)
	if err != nil {
		logSQLError(err, logger.DBMovieDetails)
	}

	return err
}

// MovieFindDBIDByImdbParser sets the ID field of the Dbmovie struct based on the ImdbID field.
// If the ImdbID is empty, the ID is set to 0 and the function returns.
// If the UseMediaCache setting is true, the ID is set by calling CacheThreeStringIntIndexFunc with the ImdbID.
// Otherwise, the ID is set by executing a SQL query to select the id from the dbmovies table where the imdb_id matches the ImdbID.
func (movie *Dbmovie) MovieFindDBIDByImdbParser() {
	if movie.ImdbID == "" {
		movie.ID = 0
		return
	}

	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)
	if config.GetSettingsGeneral().UseMediaCache {
		movie.ID = CacheThreeStringIntIndexFunc(logger.CacheDBMovie, &movie.ImdbID)
		return
	}

	Scanrowsdyn(false, "select id from dbmovies where imdb_id = ?", &movie.ID, &movie.ImdbID)
}

// GetImdbRating queries the imdb_ratings table to get the average rating and number of votes for the given IMDb ID.
// It populates the rating fields on the Dbmovie struct if they are empty or overwrite is true.
func (movie *Dbmovie) GetImdbRating(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}

	var imdbratedata ImdbRatings
	GetdatarowArgsImdb(
		"select num_votes, average_rating from imdb_ratings where tconst = ?",
		&movie.ImdbID,
		&imdbratedata.NumVotes,
		&imdbratedata.AverageRating,
	)

	if (movie.VoteAverage == 0 || overwrite) && imdbratedata.AverageRating != 0 {
		movie.VoteAverage = imdbratedata.AverageRating
	}

	if (movie.VoteCount == 0 || overwrite) && imdbratedata.NumVotes != 0 {
		movie.VoteCount = imdbratedata.NumVotes
	}
}

// GetMoviesByIDP retrieves a Movie by ID. It takes a uint ID
// and a pointer to a Movie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// movie data and scan it into the Movie struct.
// Returns an error if there was a problem retrieving the data.
func (u *Movie) GetMoviesByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id from movies where id = ?",
		u,
		id,
	)
}

// InsertMovieFile inserts a movie file record into the database.
func InsertMovieFile(
	movieID uint,
	location, filename, extension string,
	resolutionID, qualityID, codecID, audioID uint,
) error {
	if movieID == 0 {
		return nil
	}

	var (
		proper, repack, extended bool
		dbmovieID                uint
		height, width            int
		qualityProfile           string
	)

	// Get dbmovie_id from movie

	Scanrowsdyn(false, "SELECT dbmovie_id FROM movies WHERE id = ?", &dbmovieID, &movieID)

	ExecN(
		"insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
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
		&movieID,
		&dbmovieID,
		&height,
		&width,
	)

	return nil
}

// ChecknzbtitleB checks if the nzbtitle matches the movietitle and year.
// It compares the movietitle and nzbtitle directly, and also tries
// appending/removing the year, converting to slugs, etc.
// It is used to fuzzy match nzb titles to movie info during parsing.
func ChecknzbtitleB(
	movietitle, movietitlesluga, nzbtitle string,
	allowpm1 bool,
	yearu uint16,
) bool {
	if movietitle == "" || nzbtitle == "" {
		return false
	}

	// Quick exact match first
	if strings.EqualFold(movietitle, nzbtitle) {
		return yearu == 0 || checkYearInTitle(nzbtitle, yearu, allowpm1)
	}

	// Check if movie title is contained in nzb title
	if logger.ContainsI(nzbtitle, movietitle) {
		return yearu == 0 || checkYearInTitle(nzbtitle, yearu, allowpm1)
	}

	// Slug comparison (more expensive, do last)
	return compareSluggedTitles(movietitle, movietitlesluga, nzbtitle, yearu, allowpm1)
}

// checkYearInTitle checks if a given year is present in the title.
// It checks for an exact match of the year, and optionally allows matching years
// that are one year before or after the specified year.
// Returns true if the year is found in the title, false otherwise.
func checkYearInTitle(title string, year uint16, allowpm1 bool) bool {
	yearStr := logger.IntToString(year)
	if strings.Contains(title, yearStr) {
		return true
	}

	if allowpm1 {
		if strings.Contains(title, logger.IntToString(year+1)) ||
			strings.Contains(title, logger.IntToString(year-1)) {
			return true
		}
	}

	return false
}

// compareSluggedTitles compares movie titles by converting them to slugs and checking for matches.
// It handles cases where the slugged movie title is a substring or exact match of the slugged NZB title.
// If a year is provided, it also verifies the year's presence in the slugged title.
// Returns true if a match is found, false otherwise.
func compareSluggedTitles(
	movietitle, movietitlesluga, nzbtitle string,
	yearu uint16,
	allowpm1 bool,
) bool {
	var movietitleslug []byte
	if movietitlesluga != "" {
		movietitleslug = []byte(movietitlesluga)
	} else {
		movietitleslug = logger.StringToSlugBytes(movietitle)
	}

	slugged := logger.StringToSlugBytes(nzbtitle)
	if len(slugged) == 0 {
		return false
	}

	// Remove dashes for comparison
	movietitleslug = bytes.ReplaceAll(movietitleslug, []byte{'-'}, nil)
	slugged = bytes.ReplaceAll(slugged, []byte{'-'}, nil)

	if bytes.Equal(movietitleslug, slugged) || bytes.Contains(slugged, movietitleslug) {
		return yearu == 0 || checkYearInSluggedTitle(slugged, yearu, allowpm1)
	}

	return false
}

// checkYearInSluggedTitle checks if a given year or its adjacent years (when allowed) are present in a slugged title.
// It converts the year to a string and searches for it within the slugged bytes.
// If allowpm1 is true, it also checks for years one before or after the specified year.
// Returns true if the year is found, false otherwise.
func checkYearInSluggedTitle(slugged []byte, yearu uint16, allowpm1 bool) bool {
	if bytes.Contains(slugged, []byte(strconv.Itoa(int(yearu)))) {
		return true
	}

	if allowpm1 {
		if bytes.Contains(slugged, []byte(strconv.Itoa(int(yearu+1)))) ||
			bytes.Contains(slugged, []byte(strconv.Itoa(int(yearu-1)))) {
			return true
		}
	}

	return false
}
