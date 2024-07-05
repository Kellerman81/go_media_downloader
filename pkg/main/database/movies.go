// movies
package database

import (
	"bytes"
	"database/sql"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

type Movie struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Lastscan       sql.NullTime
	Blacklisted    bool
	QualityReached bool   `db:"quality_reached"`
	QualityProfile string `db:"quality_profile"`
	Missing        bool
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
	Listname       string
	Rootpath       string
	DbmovieID      uint `db:"dbmovie_id"`
}

type MovieFileUnmatched struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Listname    string
	Filepath    string
	LastChecked sql.NullTime `db:"last_checked"`
	ParsedData  string       `db:"parsed_data"`
}

type ResultMovies struct {
	Dbmovie
	Listname       string
	Lastscan       sql.NullTime
	Blacklisted    bool
	QualityReached bool   `db:"quality_reached"`
	QualityProfile string `db:"quality_profile"`
	Rootpath       string
	Missing        bool
	DbmovieID      uint `db:"dbmovie_id"`
}

type MovieFile struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Location       string
	Filename       string
	Extension      string
	QualityProfile string `db:"quality_profile"`
	Proper         bool
	Extended       bool
	Repack         bool
	Height         uint16
	Width          uint16
	ResolutionID   uint `db:"resolution_id"`
	QualityID      uint `db:"quality_id"`
	CodecID        uint `db:"codec_id"`
	AudioID        uint `db:"audio_id"`
	MovieID        uint `db:"movie_id"`
	DbmovieID      uint `db:"dbmovie_id"`
}

type MovieHistory struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Title          string
	URL            string
	Indexer        string
	HistoryType    string `db:"type"`
	Target         string
	DownloadedAt   time.Time `db:"downloaded_at"`
	Blacklisted    bool
	QualityProfile string `db:"quality_profile"`
	ResolutionID   uint   `db:"resolution_id"`
	QualityID      uint   `db:"quality_id"`
	CodecID        uint   `db:"codec_id"`
	AudioID        uint   `db:"audio_id"`
	MovieID        uint   `db:"movie_id"`
	DbmovieID      uint   `db:"dbmovie_id"`
}

type Dbmovie struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Title            string
	ReleaseDate      sql.NullTime `db:"release_date" json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	Year             uint16
	Adult            bool
	Budget           int
	Genres           string
	OriginalLanguage string `db:"original_language"`
	OriginalTitle    string `db:"original_title"`
	Overview         string
	Popularity       float32
	Revenue          int
	Runtime          int
	SpokenLanguages  string `db:"spoken_languages"`
	Status           string
	Tagline          string
	VoteAverage      float32 `db:"vote_average"`
	VoteCount        int32   `db:"vote_count"`
	TraktID          int     `db:"trakt_id"`
	MoviedbID        int     `db:"moviedb_id"`
	ImdbID           string  `db:"imdb_id"`
	FreebaseMID      string  `db:"freebase_m_id"`
	FreebaseID       string  `db:"freebase_id"`
	FacebookID       string  `db:"facebook_id"`
	InstagramID      string  `db:"instagram_id"`
	TwitterID        string  `db:"twitter_id"`
	URL              string
	Backdrop         string
	Poster           string
	Slug             string
}

type DbmovieTitle struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	DbmovieID uint      `db:"dbmovie_id"`
	Title     string
	Slug      string
	Region    string
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
	logger.AddImdbPrefixP(&movie.ImdbID)
	movie.GetImdbTitle(&movie.ImdbID, overwrite)
}

// GetImdbTitle queries the imdb_titles table to populate movie details from IMDb.
// It takes the IMDb id pointer, movie struct pointer, and a boolean overwrite flag.
// It will populate the movie struct with data from IMDb if fields are empty or overwrite is true.
// This handles setting the title, year, adult flag, genres, original title, runtime, slug, url,
// vote average, and vote count.
func (movie *Dbmovie) GetImdbTitle(arg *string, overwrite bool) {
	imdbdata, err := structscanG[ImdbTitle]("select * from imdb_titles where tconst = ?", true, arg)
	if err != nil {
		return
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
	if (movie.Runtime == 0 || movie.Runtime == 1 || movie.Runtime == 2 || movie.Runtime == 3 || movie.Runtime == 4 || movie.Runtime == 60 || movie.Runtime == 90 || movie.Runtime == 120 || overwrite) && imdbdata.RuntimeMinutes != 0 {
		if movie.Runtime != 0 && (imdbdata.RuntimeMinutes == 1 || imdbdata.RuntimeMinutes == 2 || imdbdata.RuntimeMinutes == 3 || imdbdata.RuntimeMinutes == 4) {
			logger.LogDynamicany("debug", "skipped imdb movie runtime for", &logger.StrImdb, &movie.ImdbID)
		} else {
			logger.LogDynamicany("debug", "set imdb movie runtime for", &logger.StrImdb, &movie.ImdbID)
			movie.Runtime = imdbdata.RuntimeMinutes
		}
	}
	if (movie.Slug == "" || overwrite) && imdbdata.Slug != "" {
		movie.Slug = imdbdata.Slug
	}
	if movie.URL == "" || overwrite {
		movie.URL = logger.JoinStrings("https://www.imdb.com/title/", movie.ImdbID)
	}

	movie.GetImdbRating(&movie.ImdbID, overwrite)
	//imdbdata.Close()
}

// GetDbmovieByIDP retrieves a Dbmovie by ID. It takes a uint ID and a
// pointer to a Dbmovie struct to scan the result into. It executes a SQL query
// using the structscan function to select the dbmovie data and scan it into
// the Dbmovie struct. Returns an error if there was a problem retrieving the data.
func (movie *Dbmovie) GetDbmovieByIDP(id *uint) error {
	return structscan1(logger.DBMovieDetails, false, movie, id)
}

// GetImdbRating queries the imdb_ratings table to get the average rating and number of votes for the given IMDb ID.
// It populates the rating fields on the Dbmovie struct if they are empty or overwrite is true.
func (movie *Dbmovie) GetImdbRating(arg *string, overwrite bool) {
	imdbratedata, err := structscanG[ImdbRatings]("select * from imdb_ratings where tconst = ?", true, arg)
	if err == nil {
		if (movie.VoteAverage == 0 || overwrite) && imdbratedata.AverageRating != 0 {
			movie.VoteAverage = imdbratedata.AverageRating
		}
		if (movie.VoteCount == 0 || overwrite) && imdbratedata.NumVotes != 0 {
			movie.VoteCount = imdbratedata.NumVotes
		}
		//imdbratedata.Close()
	}
}

// GetMoviesByIDP retrieves a Movie by ID. It takes a uint ID
// and a pointer to a Movie struct to scan the result into.
// It executes a SQL query using the structscan function to select the
// movie data and scan it into the Movie struct.
// Returns an error if there was a problem retrieving the data.
func (u *Movie) GetMoviesByIDP(id *uint) error {
	return structscan1("select id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id from movies where id = ?", false, u, id)
}

// ChecknzbtitleB checks if the nzbtitle matches the movietitle and year.
// It compares the movietitle and nzbtitle directly, and also tries
// appending/removing the year, converting to slugs, etc.
// It is used to fuzzy match nzb titles to movie info during parsing.
func ChecknzbtitleB(movietitle string, movietitlesluga any, nzbtitle string, allowpm1 bool, yearu uint16) bool {
	if movietitle == "" {
		return false
	}
	if movietitle == nzbtitle || strings.EqualFold(movietitle, nzbtitle) {
		return true
	}
	if logger.ContainsI(nzbtitle, movietitle) {
		if yearu != 0 {
			year := logger.IntToString(yearu)
			checkstr1 := logger.JoinStrings(movietitle, logger.StrSpace, year)
			checkstr2 := logger.JoinStrings(movietitle, " (", year, ")")
			if checkstr1 == nzbtitle ||
				checkstr2 == nzbtitle ||
				strings.EqualFold(checkstr1, nzbtitle) ||
				strings.EqualFold(checkstr2, nzbtitle) {
				return true
			}
			if allowpm1 {
				yearp := logger.IntToString(yearu + 1)
				checkstr1 = logger.JoinStrings(movietitle, logger.StrSpace, yearp) //JoinStrings
				checkstr2 = logger.JoinStrings(movietitle, " (", yearp, ")")       //JoinStrings
				if checkstr1 == nzbtitle ||
					checkstr2 == nzbtitle ||
					strings.EqualFold(checkstr1, nzbtitle) ||
					strings.EqualFold(checkstr2, nzbtitle) {
					return true
				}

				yearm := logger.IntToString(yearu - 1)
				checkstr1 = logger.JoinStrings(movietitle, logger.StrSpace, yearm) //JoinStrings
				checkstr2 = logger.JoinStrings(movietitle, " (", yearm, ")")       //JoinStrings
				if checkstr1 == nzbtitle ||
					checkstr2 == nzbtitle ||
					strings.EqualFold(checkstr1, nzbtitle) ||
					strings.EqualFold(checkstr2, nzbtitle) {
					return true
				}
			}
		}
	}

	var movietitleslug []byte
	switch tt := movietitlesluga.(type) {
	case string:
		if tt != "" {
			movietitleslug = logger.StringToByteArr(tt)
		}
	case []byte:
		movietitleslug = tt
	}
	if len(movietitleslug) == 0 {
		movietitleslug = logger.StringToSlugBytes(movietitle)
	}
	slugged := logger.StringToSlugBytes(nzbtitle)
	//defer clear(slugged)
	if len(slugged) == 0 {
		return false
	}
	if bytes.Equal(movietitleslug, slugged) {
		return true
	}

	movietitleslug = logger.BytesRemoveAllRunesP(movietitleslug, '-')
	slugged = logger.BytesRemoveAllRunesP(slugged, '-')
	if bytes.Equal(movietitleslug, slugged) {
		return true
	}
	if !bytes.Contains(slugged, movietitleslug) {
		return false
	}

	if yearu != 0 {
		bld := logger.PlAddBuffer.Get()
		defer logger.PlAddBuffer.Put(bld)
		bld.WriteUInt16(yearu)
		if bytes.Contains(slugged, bld.Bytes()) && bytes.Equal(append(movietitleslug, bld.Bytes()...), slugged) {
			return true
		}

		if allowpm1 {
			bld.Reset()
			bld.WriteUInt16(yearu + 1)
			if bytes.Contains(slugged, bld.Bytes()) && bytes.Equal(append(movietitleslug, bld.Bytes()...), slugged) {
				return true
			}
			bld.Reset()
			bld.WriteUInt16(yearu - 1)
			if bytes.Contains(slugged, bld.Bytes()) && bytes.Equal(append(movietitleslug, bld.Bytes()...), slugged) {
				return true
			}
		}
	}

	return false
}
