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
	QualityProfile string       `db:"quality_profile" displayname:"Quality Settings" comment:"Movie quality settings"`
	Listname       string       `displayname:"Configuration List" comment:"Configuration list name"`
	Rootpath       string       `displayname:"Storage Path" comment:"Movie storage directory"`
	Lastscan       sql.NullTime `displayname:"Last Scanned" comment:"Last scan timestamp"`
	CreatedAt      time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt      time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID             uint         `displayname:"Movie ID" comment:"Unique movie identifier"`
	DbmovieID      uint         `db:"dbmovie_id" displayname:"Database Reference" comment:"Database movie reference"`
	Blacklisted    bool         `displayname:"Is Blacklisted" comment:"Movie is blacklisted"`
	QualityReached bool         `db:"quality_reached" displayname:"Quality Target Met" comment:"Target quality achieved"`
	Missing        bool         `displayname:"Is Missing" comment:"Movie is missing"`
	DontUpgrade    bool         `db:"dont_upgrade" displayname:"Upgrades Disabled" comment:"Disable quality upgrades"`
	DontSearch     bool         `db:"dont_search" displayname:"Search Disabled" comment:"Disable new searches"`
}

type MovieFileUnmatched struct {
	Listname    string       `displayname:"Configuration List" comment:"Configuration list name"`
	Filepath    string       `displayname:"File Location" comment:"Unmatched file location"`
	ParsedData  string       `db:"parsed_data" displayname:"Parse Results" comment:"File parsing results"`
	LastChecked sql.NullTime `db:"last_checked" displayname:"Last Check" comment:"Last check timestamp"`
	CreatedAt   time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt   time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID          uint         `displayname:"Record ID" comment:"Unique record identifier"`
}

type ResultMovies struct {
	Dbmovie
	Listname       string       `displayname:"Configuration List" comment:"Configuration list name"`
	QualityProfile string       `db:"quality_profile" displayname:"Quality Settings" comment:"Movie quality settings"`
	Rootpath       string       `displayname:"Storage Path" comment:"Movie storage directory"`
	Lastscan       sql.NullTime `displayname:"Last Scanned" comment:"Last scan timestamp"`
	DbmovieID      uint         `db:"dbmovie_id" displayname:"Movie Reference" comment:"Database movie reference"`
	Blacklisted    bool         `displayname:"Is Blacklisted" comment:"Movie is blacklisted"`
	QualityReached bool         `db:"quality_reached" displayname:"Quality Target Met" comment:"Target quality achieved"`
	Missing        bool         `displayname:"Is Missing" comment:"Movie is missing"`
}

type MovieFile struct {
	Location       string    `displayname:"File Path" comment:"File storage path"`
	Filename       string    `displayname:"File Name" comment:"File name only"`
	Extension      string    `displayname:"File Type" comment:"File extension type"`
	QualityProfile string    `db:"quality_profile" displayname:"Quality Settings" comment:"File quality settings"`
	CreatedAt      time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt      time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ResolutionID   uint      `db:"resolution_id" displayname:"Video Resolution" comment:"Video resolution reference"`
	QualityID      uint      `db:"quality_id" displayname:"Source Quality" comment:"Quality type reference"`
	CodecID        uint      `db:"codec_id" displayname:"Video Codec" comment:"Video codec reference"`
	AudioID        uint      `db:"audio_id" displayname:"Audio Codec" comment:"Audio codec reference"`
	MovieID        uint      `db:"movie_id" displayname:"Parent Movie" comment:"Parent movie reference"`
	DbmovieID      uint      `db:"dbmovie_id" displayname:"Movie Reference" comment:"Database movie reference"`
	ID             uint      `displayname:"File ID" comment:"Unique file identifier"`
	Height         uint16    `displayname:"Video Height" comment:"Video height pixels"`
	Width          uint16    `displayname:"Video Width" comment:"Video width pixels"`
	Proper         bool      `displayname:"Proper Release" comment:"Proper release flag"`
	Extended       bool      `displayname:"Extended Cut" comment:"Extended cut flag"`
	Repack         bool      `displayname:"Repack Release" comment:"Repack release flag"`
}

type MovieHistory struct {
	Title          string    `displayname:"Release Title" comment:"Release title name"`
	URL            string    `displayname:"Download URL" comment:"Download source URL"`
	Indexer        string    `displayname:"Source Indexer" comment:"Source indexer name"`
	HistoryType    string    `db:"type" displayname:"Media Type" comment:"Movie category type"`
	Target         string    `displayname:"Target Path" comment:"Download target path"`
	QualityProfile string    `db:"quality_profile" displayname:"Quality Settings" comment:"Quality settings used"`
	CreatedAt      time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt      time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	DownloadedAt   time.Time `db:"downloaded_at" displayname:"Download Date" comment:"Download completion timestamp"`
	ID             uint      `displayname:"History ID" comment:"Unique history identifier"`
	ResolutionID   uint      `db:"resolution_id" displayname:"Video Resolution" comment:"Video resolution reference"`
	QualityID      uint      `db:"quality_id" displayname:"Source Quality" comment:"Quality type reference"`
	CodecID        uint      `db:"codec_id" displayname:"Video Codec" comment:"Video codec reference"`
	AudioID        uint      `db:"audio_id" displayname:"Audio Codec" comment:"Audio codec reference"`
	MovieID        uint      `db:"movie_id" displayname:"Parent Movie" comment:"Parent movie reference"`
	DbmovieID      uint      `db:"dbmovie_id" displayname:"Movie Reference" comment:"Database movie reference"`
	Blacklisted    bool      `displayname:"Is Blacklisted" comment:"Entry is blacklisted"`
}

type Dbmovie struct {
	Genres           string       `displayname:"Genre Categories" comment:"Movie genre classification"`
	OriginalLanguage string       `db:"original_language" displayname:"Original Language" comment:"Movie original language"`
	OriginalTitle    string       `db:"original_title" displayname:"Original Title" comment:"Movie original title"`
	Overview         string       `displayname:"Plot Summary" comment:"Movie plot summary"`
	Title            string       `displayname:"Movie Title" comment:"Primary movie title"`
	SpokenLanguages  string       `db:"spoken_languages" displayname:"Spoken Languages" comment:"Languages spoken in movie"`
	Status           string       `displayname:"Release Status" comment:"Movie release status"`
	Tagline          string       `displayname:"Movie Tagline" comment:"Movie promotional tagline"`
	ImdbID           string       `db:"imdb_id" displayname:"IMDB Identifier" comment:"IMDB database identifier"`
	FreebaseMID      string       `db:"freebase_m_id" displayname:"Freebase Machine ID" comment:"Freebase machine identifier"`
	FreebaseID       string       `db:"freebase_id" displayname:"Freebase Identifier" comment:"Freebase database identifier"`
	FacebookID       string       `db:"facebook_id" displayname:"Facebook ID" comment:"Facebook page identifier"`
	InstagramID      string       `db:"instagram_id" displayname:"Instagram ID" comment:"Instagram profile identifier"`
	TwitterID        string       `db:"twitter_id" displayname:"Twitter ID" comment:"Twitter profile identifier"`
	URL              string       `displayname:"Movie URL" comment:"Movie information URL"`
	Backdrop         string       `displayname:"Backdrop Image" comment:"Movie backdrop image"`
	Poster           string       `displayname:"Poster Image" comment:"Movie poster image"`
	Slug             string       `displayname:"URL Slug" comment:"URL friendly identifier"`
	ReleaseDate      sql.NullTime `db:"release_date" json:"release_date" time_format:"2006-01-02" time_utc:"1" displayname:"Release Date" comment:"Movie release date"`
	CreatedAt        time.Time    `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt        time.Time    `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	Popularity       float32      `displayname:"Popularity Score" comment:"Movie popularity rating"`
	VoteAverage      float32      `db:"vote_average" displayname:"User Rating" comment:"Average user rating"`
	Budget           int          `displayname:"Production Budget" comment:"Movie production budget"`
	Revenue          int          `displayname:"Box Office" comment:"Movie box office revenue"`
	Runtime          int          `displayname:"Movie Duration" comment:"Movie runtime minutes"`
	TraktID          int          `db:"trakt_id" displayname:"Trakt Identifier" comment:"Trakt database identifier"`
	MoviedbID        int          `db:"moviedb_id" displayname:"MovieDB Identifier" comment:"MovieDB database identifier"`
	ID               uint         `displayname:"Movie ID" comment:"Unique movie identifier"`
	VoteCount        int32        `db:"vote_count" displayname:"Rating Votes" comment:"Number of user votes"`
	Year             uint16       `displayname:"Release Year" comment:"Movie release year"`
	Adult            bool         `displayname:"Adult Content" comment:"Adult content flag"`
}

type DbmovieTitle struct {
	Title     string    `displayname:"Alternative Title" comment:"Alternative movie title"`
	Slug      string    `displayname:"URL Slug" comment:"URL friendly identifier"`
	Region    string    `displayname:"Regional Code" comment:"Title regional variant"`
	CreatedAt time.Time `db:"created_at" displayname:"Date Created" comment:"Record creation timestamp"`
	UpdatedAt time.Time `db:"updated_at" displayname:"Last Updated" comment:"Last modification timestamp"`
	ID        uint      `displayname:"Title ID" comment:"Unique title identifier"`
	DbmovieID uint      `db:"dbmovie_id" displayname:"Movie Reference" comment:"Parent movie reference"`
}

// movieGetImdbMetadata fetches movie metadata from IMDB.
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
			logger.LogDynamicany1String(
				"debug",
				"skipped imdb movie runtime for",
				logger.StrImdb,
				movie.ImdbID,
			)
		} else {
			logger.LogDynamicany1String("debug", "set imdb movie runtime for", logger.StrImdb, movie.ImdbID)
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
	return structscan1(logger.DBMovieDetails, movie, id)
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
	if strings.Contains(strings.ToLower(nzbtitle), strings.ToLower(movietitle)) {
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
