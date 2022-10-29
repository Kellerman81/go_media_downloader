// movies
package database

import (
	"database/sql"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
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

func (movie *Dbmovie) GetTitles(cfg string, queryimdb bool, querytmdb bool, querytrakt bool) []DbmovieTitle {
	arrcfglang := &logger.InStringArrayStruct{Arr: config.Cfg.Media[cfg].Metadata_title_languages}
	defer arrcfglang.Close()

	var result []DbmovieTitle = make([]DbmovieTitle, 0, 10)
	if queryimdb && movie.ImdbID != "" {
		if !strings.HasPrefix(movie.ImdbID, "tt") {
			movie.ImdbID = "tt" + movie.ImdbID
		}
		imdbakadata, _ := QueryStaticColumnsThreeString("select region, title, slug from imdb_akas where tconst = ?", true, movie.ImdbID)

		for idxaka := range imdbakadata {
			if logger.InStringArray(imdbakadata[idxaka].Str1, arrcfglang) || len(arrcfglang.Arr) == 0 {
				result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: imdbakadata[idxaka].Str2, Slug: imdbakadata[idxaka].Str3, Region: imdbakadata[idxaka].Str1})
			}
		}
	}
	if querytmdb && movie.MoviedbID != 0 {
		moviedbtitles, err := apiexternal.TmdbApi.GetMovieTitles(strconv.Itoa(movie.MoviedbID))
		if err == nil {
			for idx := range moviedbtitles.Titles {
				if ok := logger.InStringArray(moviedbtitles.Titles[idx].Iso31661, arrcfglang); !ok && len(arrcfglang.Arr) >= 1 {
					continue
				}
				result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: moviedbtitles.Titles[idx].Title, Slug: logger.StringToSlug(moviedbtitles.Titles[idx].Title), Region: moviedbtitles.Titles[idx].Iso31661})
			}
			moviedbtitles = nil
		} else {
			logger.Log.GlobalLogger.Warn("Movie tmdb titles not found for", zap.String("imdb", movie.ImdbID))
		}
	}
	if querytrakt && movie.ImdbID != "" {
		traktaliases, err := apiexternal.TraktApi.GetMovieAliases(movie.ImdbID)
		if err == nil {
			for idxalias := range traktaliases.Aliases {
				if logger.InStringArray(traktaliases.Aliases[idxalias].Country, arrcfglang) || len(arrcfglang.Arr) == 0 {
					result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: traktaliases.Aliases[idxalias].Title, Slug: logger.StringToSlug(traktaliases.Aliases[idxalias].Title), Region: traktaliases.Aliases[idxalias].Country})
				}
			}
			traktaliases = nil
		} else {
			logger.Log.GlobalLogger.Warn("Movie trakt titles not found for", zap.String("imdb", movie.ImdbID))
		}
	}
	return result
}

func (movie *Dbmovie) GetImdbMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	queryimdbid := movie.ImdbID
	if !strings.HasPrefix(movie.ImdbID, "tt") {
		queryimdbid = "tt" + movie.ImdbID
	}
	imdbdata, err := GetImdbTitle(&Query{Where: "tconst = ?"}, queryimdbid)
	if err == nil {
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

		imdbratedata, err := GetImdbRating(&Query{Where: "tconst = ?"}, queryimdbid)
		if err == nil {
			if movie.VoteAverage == 0 || movie.VoteAverage == 0.0 || overwrite {
				movie.VoteAverage = imdbratedata.AverageRating
			}
			if movie.VoteCount == 0 || overwrite {
				movie.VoteCount = imdbratedata.NumVotes
			}
		} else {
			logger.Log.GlobalLogger.Warn("Movie imdb rating not found for", zap.String("Title", movie.ImdbID))
		}
	} else {
		logger.Log.GlobalLogger.Warn("Movie imdb data not found for", zap.String("Title", movie.ImdbID))
	}
}

func (movie *Dbmovie) GetTmdbMetadata(overwrite bool) {
	if movie.MoviedbID == 0 {
		if movie.ImdbID == "" {
			return
		}
		moviedb, err := apiexternal.TmdbApi.FindImdb(movie.ImdbID)
		if err != nil {
			return
		}
		if len(moviedb.MovieResults) >= 1 {
			movie.MoviedbID = moviedb.MovieResults[0].ID
		} else {
			return
		}
	}
	moviedbdetails, err := apiexternal.TmdbApi.GetMovie(strconv.Itoa(movie.MoviedbID))
	if err == nil {
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
		if moviedbdetails.ReleaseDate != "" && !movie.ReleaseDate.Valid {
			movie.ReleaseDate = logger.ParseDate(moviedbdetails.ReleaseDate, "2006-01-02")
		}
		if movie.Genres == "" || overwrite {
			movie.Genres = ""
			for idxgenre := range moviedbdetails.Genres {
				if len(movie.Genres) >= 1 {
					movie.Genres += ","
				}
				movie.Genres += moviedbdetails.Genres[idxgenre].Name
			}
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
		movie.SpokenLanguages = ""
		for idxlang := range moviedbdetails.SpokenLanguages {
			if len(movie.SpokenLanguages) >= 1 {
				movie.SpokenLanguages += ","
			}
			movie.SpokenLanguages += moviedbdetails.SpokenLanguages[idxlang].EnglishName
		}
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
	} else {
		logger.Log.GlobalLogger.Warn("Movie tmdb movie entry not found for", zap.String("Title", movie.ImdbID))
	}
}

func (movie *Dbmovie) GetOmdbMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	var omdbdetails apiexternal.OmDBMovie
	err := apiexternal.OmdbApi.GetMovie(movie.ImdbID, &omdbdetails)
	if err == nil {
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
	} else {
		logger.Log.GlobalLogger.Warn("Movie omdb data not found for", zap.String("Title", movie.ImdbID))
	}
}

func (movie *Dbmovie) GetTraktMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
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
			movie.Genres = strings.Join(traktdetails.Genres, ",")
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
				movie.ReleaseDate = logger.ParseDate(traktdetails.Released, "2006-01-02")
			}
		}
		if movie.OriginalLanguage == "" || overwrite {
			movie.OriginalLanguage = traktdetails.Language
		}
		if movie.Tagline == "" || overwrite {
			movie.Tagline = traktdetails.Tagline
		}
	} else {
		logger.Log.GlobalLogger.Warn("Movie trakt data not found for", zap.String("Title", movie.ImdbID))
	}
}
func (movie *Dbmovie) GetMetadata(queryimdb bool, querytmdb bool, queryomdb bool, querytrakt bool) {
	logger.Log.GlobalLogger.Info("Get Metadata for", zap.String("Title", movie.ImdbID))

	if queryimdb {
		movie.GetImdbMetadata(false)
	}
	if querytmdb {
		movie.GetTmdbMetadata(false)
	}
	if queryomdb {
		movie.GetOmdbMetadata(false)
	}
	if querytrakt {
		movie.GetTraktMetadata(false)
	}
	logger.Log.GlobalLogger.Info("ENDED Get Metadata for", zap.String("Title", movie.ImdbID))
}

func (dbmovie *Dbmovie) Getmoviemetadata(refresh bool) {
	prios := config.Cfg.General.MovieMetaSourcePriority
	if len(prios) >= 1 {
		for idxmeta := range prios {
			switch prios[idxmeta] {
			case "imdb":
				dbmovie.GetImdbMetadata(refresh)
			case "tmdb":
				dbmovie.GetTmdbMetadata(false)
			case "omdb":
				dbmovie.GetOmdbMetadata(false)
			case "trakt":
				dbmovie.GetTraktMetadata(false)
			}
		}
	} else {
		dbmovie.GetMetadata(config.Cfg.General.MovieMetaSourceImdb, config.Cfg.General.MovieMetaSourceTmdb, config.Cfg.General.MovieMetaSourceOmdb, config.Cfg.General.MovieMetaSourceTrakt)
	}
	prios = nil
}

func (dbmovie *Dbmovie) Getmoviemetatitles(cfg string) {
	if cfg == "" {
		tmpl, _ := QueryColumnString("select listname from movies where dbmovie_id = ?", dbmovie.ID)
		if tmpl != "" {
			cfg = config.FindconfigTemplateOnList("movie_", tmpl)
		}
	}
	if cfg == "" {
		return
	}

	titles := &logger.InStringArrayStruct{Arr: QueryStaticStringArray("select title from dbmovie_titles where dbmovie_id = ?", false, 0, dbmovie.ID)}
	defer titles.Close()
	titlegroup := dbmovie.GetTitles(cfg, config.Cfg.General.MovieAlternateTitleMetaSourceImdb, config.Cfg.General.MovieAlternateTitleMetaSourceTmdb, config.Cfg.General.MovieAlternateTitleMetaSourceTrakt)
	for idx := range titlegroup {
		if titlegroup[idx].Title == "" {
			continue
		}
		if !logger.InStringArray(titlegroup[idx].Title, titles) {
			InsertNamed("Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (:title, :slug, :dbmovie_id, :region)", DbmovieTitle{Title: titlegroup[idx].Title, Slug: titlegroup[idx].Slug, DbmovieID: dbmovie.ID, Region: titlegroup[idx].Region})
			titles.Arr = append(titles.Arr, titlegroup[idx].Title)
		}
	}
	titlegroup = nil
}
