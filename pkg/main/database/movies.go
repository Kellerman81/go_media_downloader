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
	QualityProfile string `db:"quality_profile"`
	Listname       string
	Rootpath       string
	Lastscan       sql.NullTime
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	ID             uint
	DbmovieID      uint `db:"dbmovie_id"`
	Blacklisted    bool
	QualityReached bool `db:"quality_reached"`
	Missing        bool
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
}

type MovieFileUnmatched struct {
	Listname    string
	Filepath    string
	ParsedData  string       `db:"parsed_data"`
	LastChecked sql.NullTime `db:"last_checked"`
	CreatedAt   time.Time    `db:"created_at"`
	UpdatedAt   time.Time    `db:"updated_at"`
	ID          uint
}

type ResultMovies struct {
	Dbmovie
	Listname       string
	QualityProfile string `db:"quality_profile"`
	Rootpath       string
	Lastscan       sql.NullTime
	DbmovieID      uint `db:"dbmovie_id"`
	Blacklisted    bool
	QualityReached bool `db:"quality_reached"`
	Missing        bool
}

type MovieFile struct {
	Location       string
	Filename       string
	Extension      string
	QualityProfile string    `db:"quality_profile"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	ResolutionID   uint      `db:"resolution_id"`
	QualityID      uint      `db:"quality_id"`
	CodecID        uint      `db:"codec_id"`
	AudioID        uint      `db:"audio_id"`
	MovieID        uint      `db:"movie_id"`
	DbmovieID      uint      `db:"dbmovie_id"`
	ID             uint
	Height         uint16
	Width          uint16
	Proper         bool
	Extended       bool
	Repack         bool
}

type MovieHistory struct {
	Title          string
	URL            string
	Indexer        string
	HistoryType    string `db:"type"`
	Target         string
	QualityProfile string    `db:"quality_profile"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	DownloadedAt   time.Time `db:"downloaded_at"`
	ID             uint
	ResolutionID   uint `db:"resolution_id"`
	QualityID      uint `db:"quality_id"`
	CodecID        uint `db:"codec_id"`
	AudioID        uint `db:"audio_id"`
	MovieID        uint `db:"movie_id"`
	DbmovieID      uint `db:"dbmovie_id"`
	Blacklisted    bool
}

type Dbmovie struct {
	Genres           string
	OriginalLanguage string `db:"original_language"`
	OriginalTitle    string `db:"original_title"`
	Overview         string
	Title            string
	SpokenLanguages  string `db:"spoken_languages"`
	Status           string
	Tagline          string
	ImdbID           string `db:"imdb_id"`
	FreebaseMID      string `db:"freebase_m_id"`
	FreebaseID       string `db:"freebase_id"`
	FacebookID       string `db:"facebook_id"`
	InstagramID      string `db:"instagram_id"`
	TwitterID        string `db:"twitter_id"`
	URL              string
	Backdrop         string
	Poster           string
	Slug             string
	ReleaseDate      sql.NullTime `db:"release_date"      json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt        time.Time    `db:"created_at"`
	UpdatedAt        time.Time    `db:"updated_at"`
	Popularity       float32
	VoteAverage      float32 `db:"vote_average"`
	Budget           int
	Revenue          int
	Runtime          int
	TraktID          int `db:"trakt_id"`
	MoviedbID        int `db:"moviedb_id"`
	ID               uint
	VoteCount        int32 `db:"vote_count"`
	Year             uint16
	Adult            bool
}

type DbmovieTitle struct {
	Title     string
	Slug      string
	Region    string
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	ID        uint
	DbmovieID uint `db:"dbmovie_id"`
}

// movieGetImdbMetadata fetches movie metadata from IMDB.
// It takes a pointer to a Dbmovie struct and a bool indicating
// whether to overwrite existing data.
// It adds the "tt" prefix to the IMDB ID if missing, fetches
// the title from the IMDB API, and clears temporary variables.
func (movie *Dbmovie) MovieGetImdbMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)
	movie.GetImdbTitle(overwrite)
}

// GetImdbTitle queries the imdb_titles table to populate movie details from IMDb.
// It takes the IMDb id pointer, movie struct pointer, and a boolean overwrite flag.
// It will populate the movie struct with data from IMDb if fields are empty or overwrite is true.
// This handles setting the title, year, adult flag, genres, original title, runtime, slug, url,
// vote average, and vote count.
func (movie *Dbmovie) GetImdbTitle(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	var imdbdata ImdbTitle
	GetdatarowArgsImdb(
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
	if config.SettingsGeneral.UseMediaCache {
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
