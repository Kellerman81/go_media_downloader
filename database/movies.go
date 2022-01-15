// movies
package database

import (
	"database/sql"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/logger"
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
type MovieJson struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Lastscan       time.Time `db:"lastscan" json:"lastscan" time_format:"2006-01-02 22:00" time_utc:"1"`
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
type MovieFileUnmatchedJson struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Listname    string
	Filepath    string
	LastChecked time.Time `db:"last_checked" json:"last_checked" time_format:"2006-01-02 22:00" time_utc:"1"`
	ParsedData  string    `db:"parsed_data"`
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

type ResultMoviesJson struct {
	DbmovieJson
	Listname       string
	Lastscan       time.Time `db:"lastscan" json:"lastscan" time_format:"2006-01-02 22:00" time_utc:"1"`
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
	Height         int
	Width          int
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
	Type           string
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
	Year             int
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
	VoteCount        int     `db:"vote_count"`
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
	//Dbmovies_titles_id []Dbmovies_titles `gorm:"foreignKey:Dbmovies_id"`
}
type DbmovieJson struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Title            string
	ReleaseDate      time.Time `db:"release_date" json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	Year             int
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
	VoteCount        int     `db:"vote_count"`
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

func (movie *Dbmovie) GetTitles(allowed []string, queryimdb bool, querytmdb bool, querytrakt bool) []DbmovieTitle {
	c := []DbmovieTitle{}

	processed := []string{}
	if queryimdb {
		queryimdbid := movie.ImdbID
		if !strings.HasPrefix(movie.ImdbID, "tt") {
			queryimdbid = "tt" + movie.ImdbID
		}
		imdbakadata, _ := QueryImdbAka(Query{Where: "tconst=? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})
		for _, akarow := range imdbakadata {
			regionok := false
			for idxallow := range allowed {
				if strings.EqualFold(allowed[idxallow], akarow.Region) {
					regionok = true
					break
				}
			}
			logger.Log.Debug("Title: ", akarow.Title, " Region: ", akarow.Region, " ok: ", regionok)
			if !regionok && len(allowed) >= 1 {
				continue
			}
			c = append(c, DbmovieTitle{DbmovieID: movie.ID, Title: akarow.Title, Slug: akarow.Slug, Region: akarow.Region})
			processed = append(processed, akarow.Title)
		}
	}
	if querytmdb {
		moviedbtitles, foundtitles := apiexternal.TmdbApi.GetMovieTitles(movie.MoviedbID)
		if foundtitles == nil {
			for _, row := range moviedbtitles.Titles {
				regionok := false
				for idxallow := range allowed {
					if strings.EqualFold(allowed[idxallow], row.Iso31661) {
						regionok = true
						break
					}
				}
				logger.Log.Debug("Title: ", row.Title, " Region: ", row.Iso31661, " ok: ", regionok)
				if !regionok && len(allowed) >= 1 {
					continue
				}
				if !logger.CheckStringArray(processed, row.Title) {
					c = append(c, DbmovieTitle{DbmovieID: movie.ID, Title: row.Title, Slug: logger.StringToSlug(row.Title), Region: row.Iso31661})

					processed = append(processed, row.Title)
				}
			}
		} else {
			logger.Log.Warning("Titles for Movie not found for: ", movie.ImdbID)
		}
	}

	if querytrakt {
		traktaliases, err := apiexternal.TraktApi.GetMovieAliases(movie.ImdbID)
		if err == nil {
			for _, row := range traktaliases {
				regionok := false
				for idxallow := range allowed {
					if strings.EqualFold(allowed[idxallow], row.Country) {
						regionok = true
						break
					}
				}
				logger.Log.Debug("Title: ", row.Title, " Region: ", row.Country, " ok: ", regionok)
				if !regionok && len(allowed) >= 1 {
					continue
				}
				if !logger.CheckStringArray(processed, row.Title) {
					c = append(c, DbmovieTitle{DbmovieID: movie.ID, Title: row.Title, Slug: logger.StringToSlug(row.Title), Region: row.Country})

					processed = append(processed, row.Title)
				}
			}
		}
	}
	return c
}

func (movie *Dbmovie) ClearAndGetTitles(allowed []string, queryimdb bool, querytmdb bool, querytrakt bool) []DbmovieTitle {
	DeleteRow("dbmovie_titles", Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.ID}})
	return movie.GetTitles(allowed, queryimdb, querytmdb, querytrakt)
}

func (movie *Dbmovie) GetImdbMetadata(overwrite bool) {
	queryimdbid := movie.ImdbID
	if !strings.HasPrefix(movie.ImdbID, "tt") {
		queryimdbid = "tt" + movie.ImdbID
	}
	imdbdata, imdbdataerr := GetImdbTitle(Query{Where: "tconst=? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})
	if imdbdataerr == nil {
		if movie.Title == "" || overwrite {
			movie.Title = imdbdata.PrimaryTitle
		}
		if movie.Year == 0 || overwrite {
			movie.Year = imdbdata.StartYear
		}
		if (!movie.Adult && imdbdata.IsAdult) || overwrite {
			movie.Adult = imdbdata.IsAdult
		}
		if movie.Genres == "" || overwrite {
			movie.Genres = imdbdata.Genres
		}
		if movie.OriginalTitle == "" || overwrite {
			movie.OriginalTitle = imdbdata.OriginalTitle
		}
		if movie.Runtime == 0 || overwrite {
			movie.Runtime = imdbdata.RuntimeMinutes
		}
		if movie.Slug == "" || overwrite {
			movie.Slug = imdbdata.Slug
		}
		if movie.URL == "" || overwrite {
			movie.URL = "https://www.imdb.com/title/" + queryimdbid
		}
	}
	imdbratedata, imdbratedataerr := GetImdbRating(Query{Where: "tconst=? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})

	if imdbratedataerr == nil {
		if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 || overwrite {
			movie.VoteAverage = imdbratedata.AverageRating
		}
		if movie.VoteCount == 0 || overwrite {
			movie.VoteCount = imdbratedata.NumVotes
		}
	}
}

func (movie *Dbmovie) GetTmdbMetadata(overwrite bool) {
	moviedb, found := apiexternal.TmdbApi.FindImdb(movie.ImdbID)
	if found == nil {
		if len(moviedb.MovieResults) >= 1 {
			logger.Log.Debug("Get the moviedb: ", movie.ImdbID)
			moviedbdetails, founddetail := apiexternal.TmdbApi.GetMovie(moviedb.MovieResults[0].ID)
			if founddetail == nil {
				if (!movie.Adult && moviedbdetails.Adult) || overwrite {
					movie.Adult = moviedbdetails.Adult
				}
				if movie.Title == "" || overwrite {
					if strings.Contains(moviedbdetails.Title, "&") || strings.Contains(moviedbdetails.Title, "%") {
						movie.Title = html.UnescapeString(moviedbdetails.Title)
					} else {
						movie.Title = moviedbdetails.Title
					}
				}
				if movie.Slug == "" || overwrite {
					movie.Slug = logger.StringToSlug(movie.Title)
				}
				movie.Budget = moviedbdetails.Budget
				if moviedbdetails.ReleaseDate != "" {
					layout := "2006-01-02" //year-month-day
					t, terr := time.Parse(layout, moviedbdetails.ReleaseDate)
					if terr == nil {
						movie.ReleaseDate = sql.NullTime{Time: t, Valid: true}
					}
				}
				if movie.Genres == "" || overwrite {
					var genrebuilder strings.Builder
					for _, v := range moviedbdetails.Genres {
						if genrebuilder.Len() >= 1 {
							genrebuilder.WriteString(",")
						}
						genrebuilder.WriteString(v.Name)
					}
					movie.Genres = genrebuilder.String()
					genrebuilder.Reset()
				}
				movie.OriginalLanguage = moviedbdetails.OriginalLanguage
				if movie.OriginalTitle == "" || overwrite {
					movie.OriginalTitle = moviedbdetails.OriginalTitle
				}
				movie.Overview = moviedbdetails.Overview
				movie.Popularity = moviedbdetails.Popularity
				movie.Revenue = moviedbdetails.Revenue
				if movie.Runtime == 0 {
					movie.Runtime = moviedbdetails.Runtime
				}
				var languagebuilder strings.Builder
				for _, v := range moviedbdetails.SpokenLanguages {
					if languagebuilder.Len() >= 1 {
						languagebuilder.WriteString(",")
					}
					languagebuilder.WriteString(v.EnglishName)
				}
				movie.SpokenLanguages = languagebuilder.String()
				languagebuilder.Reset()
				movie.Status = moviedbdetails.Status
				movie.Tagline = moviedbdetails.Tagline
				if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 || overwrite {
					movie.VoteAverage = moviedbdetails.VoteAverage
				}
				if movie.VoteCount == 0 || overwrite {
					movie.VoteCount = moviedbdetails.VoteCount
				}
				movie.Poster = moviedbdetails.Poster
				movie.Backdrop = moviedbdetails.Backdrop
				movie.MoviedbID = moviedbdetails.ID

				logger.Log.Debug("Get External the moviedb: ", movie.Title)
				moviedbexternal, foundexternal := apiexternal.TmdbApi.GetMovieExternal(moviedb.MovieResults[0].ID)
				if foundexternal == nil {
					movie.FreebaseMID = moviedbexternal.FreebaseMID
					movie.FreebaseID = moviedbexternal.FreebaseID
					movie.FacebookID = moviedbexternal.FacebookID
					movie.InstagramID = moviedbexternal.InstagramID
					movie.TwitterID = moviedbexternal.TwitterID
				} else {
					logger.Log.Warning("Externals for Movie not found for: ", movie.ImdbID)
				}
			} else {
				logger.Log.Warning("MovieDB Movie not found for: ", movie.ImdbID)
			}
		}
	}
}

func (movie *Dbmovie) GetOmdbMetadata(overwrite bool) {
	omdbdetails, founddetail := apiexternal.OmdbApi.GetMovie(movie.ImdbID)
	if founddetail == nil {
		if movie.Title == "" || overwrite {
			if strings.Contains(omdbdetails.Title, "&") || strings.Contains(omdbdetails.Title, "%") {
				movie.Title = html.UnescapeString(omdbdetails.Title)
			} else {
				movie.Title = omdbdetails.Title
			}
		}
		if movie.Slug == "" || overwrite {
			movie.Slug = logger.StringToSlug(movie.Title)
		}
		if movie.Genres == "" || overwrite {
			movie.Genres = omdbdetails.Genre
		}
		if movie.VoteCount == 0 || overwrite {
			movie.VoteCount, _ = strconv.Atoi(omdbdetails.ImdbVotes)
		}
		if (movie.VoteAverage == 0 || movie.VoteAverage == 0.0) || overwrite {
			rating, _ := strconv.Atoi(omdbdetails.ImdbRating)
			movie.VoteAverage = float32(rating)
		}
		if movie.Year == 0 || overwrite {
			movie.Year, _ = strconv.Atoi(omdbdetails.Year)
		}
		if movie.URL == "" || overwrite {
			movie.URL = omdbdetails.Website
		}
		if movie.Overview == "" || overwrite {
			movie.Overview = omdbdetails.Plot
		}
	}
}

func (movie *Dbmovie) GetTraktMetadata(overwrite bool) {
	traktdetails, err := apiexternal.TraktApi.GetMovie(movie.ImdbID)
	if err == nil {
		if movie.Title == "" || overwrite {
			if strings.Contains(traktdetails.Title, "&") || strings.Contains(traktdetails.Title, "%") {
				movie.Title = html.UnescapeString(traktdetails.Title)
			} else {
				movie.Title = traktdetails.Title
			}
		}
		if movie.Slug == "" || overwrite {
			movie.Slug = traktdetails.Ids.Slug
		}
		if movie.Genres == "" || overwrite {
			var genrebuilder strings.Builder
			for _, v := range traktdetails.Genres {
				if genrebuilder.Len() >= 1 {
					genrebuilder.WriteString(",")
				}
				genrebuilder.WriteString(v)
			}
			movie.Genres = genrebuilder.String()
			genrebuilder.Reset()
		}
		if movie.VoteCount == 0 || overwrite {
			movie.VoteCount = traktdetails.Votes
		}
		if (movie.VoteAverage == 0 || movie.VoteAverage == 0.0) || overwrite {
			movie.VoteAverage = traktdetails.Rating
		}
		if movie.Year == 0 || overwrite {
			movie.Year = traktdetails.Year
		}
		if movie.Overview == "" || overwrite {
			movie.Overview = traktdetails.Overview
		}
		if movie.Runtime == 0 || overwrite {
			movie.Runtime = traktdetails.Runtime
		}
		if movie.Status == "" || overwrite {
			movie.Status = traktdetails.Status
		}
		if movie.MoviedbID == 0 || overwrite {
			movie.MoviedbID = traktdetails.Ids.Tmdb
		}
		if movie.TraktID == 0 || overwrite {
			movie.TraktID = traktdetails.Ids.Trakt
		}
		if !movie.ReleaseDate.Valid || overwrite {
			if traktdetails.Released != "" {
				layout := "2006-01-02" //year-month-day
				t, terr := time.Parse(layout, traktdetails.Released)
				if terr == nil {
					movie.ReleaseDate = sql.NullTime{Time: t, Valid: true}
				}
			}
		}
		if movie.OriginalLanguage == "" || overwrite {
			movie.OriginalLanguage = traktdetails.Language
		}
		if movie.Tagline == "" || overwrite {
			movie.Tagline = traktdetails.Tagline
		}
	}
}
func (movie *Dbmovie) GetMetadata(queryimdb bool, querytmdb bool, queryomdb bool, querytrakt bool) {
	logger.Log.Debug("Find the moviedb: ", movie.ImdbID)

	if queryimdb {
		queryimdbid := movie.ImdbID
		if !strings.HasPrefix(movie.ImdbID, "tt") {
			queryimdbid = "tt" + movie.ImdbID
		}

		imdbdata, imdbdataerr := GetImdbTitle(Query{Where: "tconst=? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})
		if imdbdataerr == nil {
			if movie.Title == "" {
				movie.Title = imdbdata.PrimaryTitle
			}
			if movie.Year == 0 {
				movie.Year = imdbdata.StartYear
			}
			if !movie.Adult && imdbdata.IsAdult {
				movie.Adult = imdbdata.IsAdult
			}
			if movie.Genres == "" {
				movie.Genres = imdbdata.Genres
			}
			if movie.OriginalTitle == "" {
				movie.OriginalTitle = imdbdata.OriginalTitle
			}
			if movie.Runtime == 0 {
				movie.Runtime = imdbdata.RuntimeMinutes
			}
			if movie.Slug == "" {
				movie.Slug = imdbdata.Slug
			}
			if movie.URL == "" {
				movie.URL = "https://www.imdb.com/title/" + queryimdbid
			}
		}
		imdbratedata, imdbratedataerr := GetImdbRating(Query{Where: "tconst=? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})
		if imdbratedataerr == nil {
			if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 {
				movie.VoteAverage = imdbratedata.AverageRating
			}
			if movie.VoteCount == 0 {
				movie.VoteCount = imdbratedata.NumVotes
			}
		}
	}
	if querytmdb {
		moviedb, found := apiexternal.TmdbApi.FindImdb(movie.ImdbID)
		if found == nil {
			if len(moviedb.MovieResults) >= 1 {
				logger.Log.Debug("Get the moviedb: ", movie.ImdbID)
				moviedbdetails, founddetail := apiexternal.TmdbApi.GetMovie(moviedb.MovieResults[0].ID)
				if founddetail == nil {
					if !movie.Adult && moviedbdetails.Adult {
						movie.Adult = moviedbdetails.Adult
					}
					if movie.Title == "" {
						if strings.Contains(moviedbdetails.Title, "&") || strings.Contains(moviedbdetails.Title, "%") {
							movie.Title = html.UnescapeString(moviedbdetails.Title)
						} else {
							movie.Title = moviedbdetails.Title
						}
					}
					if movie.Slug == "" {
						movie.Slug = logger.StringToSlug(movie.Title)
					}
					movie.Budget = moviedbdetails.Budget
					if moviedbdetails.ReleaseDate != "" {
						layout := "2006-01-02" //year-month-day
						t, terr := time.Parse(layout, moviedbdetails.ReleaseDate)
						if terr == nil {
							movie.ReleaseDate = sql.NullTime{Time: t, Valid: true}
						}
					}
					if movie.Genres == "" {
						var genrebuilder strings.Builder
						for _, v := range moviedbdetails.Genres {
							if genrebuilder.Len() >= 1 {
								genrebuilder.WriteString(",")
							}
							genrebuilder.WriteString(v.Name)
						}
						movie.Genres = genrebuilder.String()
						genrebuilder.Reset()
					}
					movie.OriginalLanguage = moviedbdetails.OriginalLanguage
					if movie.OriginalTitle == "" {
						movie.OriginalTitle = moviedbdetails.OriginalTitle
					}
					movie.Overview = moviedbdetails.Overview
					movie.Popularity = moviedbdetails.Popularity
					movie.Revenue = moviedbdetails.Revenue
					if movie.Runtime == 0 {
						movie.Runtime = moviedbdetails.Runtime
					}
					var languagebuilder strings.Builder
					for _, v := range moviedbdetails.SpokenLanguages {
						if languagebuilder.Len() >= 1 {
							languagebuilder.WriteString(",")
						}
						languagebuilder.WriteString(v.EnglishName)
					}
					movie.SpokenLanguages = languagebuilder.String()
					languagebuilder.Reset()
					movie.Status = moviedbdetails.Status
					movie.Tagline = moviedbdetails.Tagline
					if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 {
						movie.VoteAverage = moviedbdetails.VoteAverage
					}
					if movie.VoteCount == 0 {
						movie.VoteCount = moviedbdetails.VoteCount
					}
					movie.Poster = moviedbdetails.Poster
					movie.Backdrop = moviedbdetails.Backdrop
					movie.MoviedbID = moviedbdetails.ID

					logger.Log.Debug("Get External the moviedb: ", movie.Title)
					moviedbexternal, foundexternal := apiexternal.TmdbApi.GetMovieExternal(moviedb.MovieResults[0].ID)
					if foundexternal == nil {
						movie.FreebaseMID = moviedbexternal.FreebaseMID
						movie.FreebaseID = moviedbexternal.FreebaseID
						movie.FacebookID = moviedbexternal.FacebookID
						movie.InstagramID = moviedbexternal.InstagramID
						movie.TwitterID = moviedbexternal.TwitterID
					} else {
						logger.Log.Warning("Externals for Movie not found for: ", movie.ImdbID)
					}
				} else {
					logger.Log.Warning("MovieDB Movie not found for: ", movie.ImdbID)
				}
			}
		}
	}
	if queryomdb {
		if movie.Title == "" || movie.Year == 0 {
			omdbdetails, founddetail := apiexternal.OmdbApi.GetMovie(movie.ImdbID)
			if founddetail == nil {
				if movie.Title == "" {
					if strings.Contains(omdbdetails.Title, "&") || strings.Contains(omdbdetails.Title, "%") {
						movie.Title = html.UnescapeString(omdbdetails.Title)
					} else {
						movie.Title = omdbdetails.Title
					}
				}
				if movie.Slug == "" {
					movie.Slug = logger.StringToSlug(movie.Title)
				}
				if movie.Genres == "" {
					movie.Genres = omdbdetails.Genre
				}
				if movie.VoteCount == 0 {
					movie.VoteCount, _ = strconv.Atoi(omdbdetails.ImdbVotes)
				}
				if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 {
					rating, _ := strconv.Atoi(omdbdetails.ImdbRating)
					movie.VoteAverage = float32(rating)
				}
				if movie.Year == 0 {
					movie.Year, _ = strconv.Atoi(omdbdetails.Year)
				}
				if movie.URL == "" {
					movie.URL = omdbdetails.Website
				}
				if movie.Overview == "" {
					movie.Overview = omdbdetails.Plot
				}
			}
		}
	}
	if querytrakt {
		traktdetails, err := apiexternal.TraktApi.GetMovie(movie.ImdbID)
		if err == nil {
			if movie.Title == "" {
				if strings.Contains(traktdetails.Title, "&") || strings.Contains(traktdetails.Title, "%") {
					movie.Title = html.UnescapeString(traktdetails.Title)
				} else {
					movie.Title = traktdetails.Title
				}
			}
			if movie.Slug == "" {
				movie.Slug = traktdetails.Ids.Slug
			}
			if movie.Genres == "" {
				var genrebuilder strings.Builder
				for _, v := range traktdetails.Genres {
					if genrebuilder.Len() >= 1 {
						genrebuilder.WriteString(",")
					}
					genrebuilder.WriteString(v)
				}
				movie.Genres = genrebuilder.String()
				genrebuilder.Reset()
			}
			if movie.VoteCount == 0 {
				movie.VoteCount = traktdetails.Votes
			}
			if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 {
				movie.VoteAverage = traktdetails.Rating
			}
			if movie.Year == 0 {
				movie.Year = traktdetails.Year
			}
			if movie.Overview == "" {
				movie.Overview = traktdetails.Overview
			}
			if movie.Runtime == 0 {
				movie.Runtime = traktdetails.Runtime
			}
			if movie.Status == "" {
				movie.Status = traktdetails.Status
			}
			if movie.MoviedbID == 0 {
				movie.MoviedbID = traktdetails.Ids.Tmdb
			}
			if movie.TraktID == 0 {
				movie.TraktID = traktdetails.Ids.Trakt
			}
			if !movie.ReleaseDate.Valid {
				if traktdetails.Released != "" {
					layout := "2006-01-02" //year-month-day
					t, terr := time.Parse(layout, traktdetails.Released)
					if terr == nil {
						movie.ReleaseDate = sql.NullTime{Time: t, Valid: true}
					}
				}
			}
			if movie.OriginalLanguage == "" {
				movie.OriginalLanguage = traktdetails.Language
			}
			if movie.Tagline == "" {
				movie.Tagline = traktdetails.Tagline
			}
		}
	}
}

func (dbmovie *Dbmovie) AddMissingMoviesMapping(listname string, quality string) []Movie {
	counter, _ := CountRows("movies", Query{Where: "listname = ? and dbmovie_id = ?", WhereArgs: []interface{}{listname, dbmovie.ID}})
	if counter == 0 {
		return []Movie{{DbmovieID: dbmovie.ID, Blacklisted: false, QualityReached: false, QualityProfile: quality, Missing: true, Listname: listname}}
	}
	return []Movie{}
}

func (movie *Movie) UpdateMoviesMapping(listname string, quality string) {
	UpdateArray("movies", []string{"listname", "quality_profile"}, []interface{}{listname, quality}, Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
}
