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
type MovieJSON struct {
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
type MovieFileUnmatchedJSON struct {
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

type ResultMoviesJSON struct {
	DbmovieJSON
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

type DbmovieJSON struct {
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

func (movie *Dbmovie) getTitles(cfgp *config.MediaTypeConfig, queryimdb bool, querytmdb bool, querytrakt bool) []DbmovieTitle {
	arrcfglang := logger.InStringArrayStruct{Arr: cfgp.MetadataTitleLanguages}

	var result []DbmovieTitle
	if queryimdb && movie.ImdbID != "" {
		if !strings.HasPrefix(movie.ImdbID, "tt") {
			movie.ImdbID = "tt" + movie.ImdbID
		}
		var imdbakadata []DbstaticThreeString
		QueryStaticColumnsThreeString(true, &Querywithargs{QueryString: "select region, title, slug from imdb_akas where tconst = ?", Args: []interface{}{movie.ImdbID}}, &imdbakadata)

		result = make([]DbmovieTitle, 0, len(imdbakadata))
		lenarr := len(arrcfglang.Arr)
		for idxaka := range imdbakadata {
			if logger.InStringArray(imdbakadata[idxaka].Str1, &arrcfglang) || lenarr == 0 {
				result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: imdbakadata[idxaka].Str2, Slug: imdbakadata[idxaka].Str3, Region: imdbakadata[idxaka].Str1})
			}
		}
		imdbakadata = nil
	}
	if querytmdb && movie.MoviedbID != 0 {
		moviedbtitles, err := apiexternal.TmdbAPI.GetMovieTitles(movie.MoviedbID)
		if err == nil {
			result = logger.GrowSliceBy(result, len(moviedbtitles.Titles))
			lenarr := len(arrcfglang.Arr)
			for idx := range moviedbtitles.Titles {
				if ok := logger.InStringArray(moviedbtitles.Titles[idx].Iso31661, &arrcfglang); !ok && lenarr >= 1 {
					continue
				}
				result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: moviedbtitles.Titles[idx].Title, Slug: logger.StringToSlug(moviedbtitles.Titles[idx].Title), Region: moviedbtitles.Titles[idx].Iso31661})
			}
			moviedbtitles.Close()
		} //else {
		//	logger.Log.GlobalLogger.Warn("Movie tmdb titles not found for", zap.String("imdb", movie.ImdbID))
		//}
	}
	if querytrakt && movie.ImdbID != "" {
		traktaliases, err := apiexternal.TraktAPI.GetMovieAliases(movie.ImdbID)
		if err != nil {
			arrcfglang.Close()
			return result
		}
		result = logger.GrowSliceBy(result, len(traktaliases.Aliases))
		lenarr := len(arrcfglang.Arr)
		for idxalias := range traktaliases.Aliases {
			if logger.InStringArray(traktaliases.Aliases[idxalias].Country, &arrcfglang) || lenarr == 0 {
				result = append(result, DbmovieTitle{DbmovieID: movie.ID, Title: traktaliases.Aliases[idxalias].Title, Slug: logger.StringToSlug(traktaliases.Aliases[idxalias].Title), Region: traktaliases.Aliases[idxalias].Country})
			}
		}
		traktaliases.Close()
	}
	arrcfglang.Close()

	return result
}

func (movie *Dbmovie) getImdbMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	queryimdbid := movie.ImdbID
	if !strings.HasPrefix(movie.ImdbID, "tt") {
		queryimdbid = "tt" + movie.ImdbID
	}
	var imdbdata ImdbTitle
	if GetImdbTitle(&Querywithargs{Query: QueryFilterByTconst, Args: []interface{}{queryimdbid}}, &imdbdata) != nil {
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
	if (movie.Runtime == 0 || overwrite) && imdbdata.RuntimeMinutes != 0 {
		movie.Runtime = imdbdata.RuntimeMinutes
	}
	if (movie.Slug == "" || overwrite) && imdbdata.Slug != "" {
		movie.Slug = imdbdata.Slug
	}
	if (movie.URL == "" || overwrite) && queryimdbid != "" {
		movie.URL = "https://www.imdb.com/title/" + queryimdbid
	}

	var imdbratedata ImdbRatings
	if GetImdbRating(&Querywithargs{Query: QueryFilterByTconst, Args: []interface{}{queryimdbid}}, &imdbratedata) != nil {
		return
	}
	if (movie.VoteAverage == 0 || movie.VoteAverage == 0.0 || overwrite) && imdbratedata.AverageRating != 0 {
		movie.VoteAverage = imdbratedata.AverageRating
	}
	if (movie.VoteCount == 0 || overwrite) && imdbratedata.NumVotes != 0 {
		movie.VoteCount = imdbratedata.NumVotes
	}
}

func (movie *Dbmovie) getTmdbMetadata(overwrite bool) {
	if movie.MoviedbID == 0 {
		if movie.ImdbID == "" {
			return
		}
		moviedb, err := apiexternal.TmdbAPI.FindImdb(movie.ImdbID)
		if err != nil {
			return
		}
		if len(moviedb.MovieResults) >= 1 {
			movie.MoviedbID = moviedb.MovieResults[0].ID
			moviedb.Close()
		} else {
			moviedb.Close()
			return
		}
	}
	moviedbdetails, err := apiexternal.TmdbAPI.GetMovie(movie.MoviedbID)
	if err != nil {
		return
	}
	if (!movie.Adult && moviedbdetails.Adult) || overwrite {
		movie.Adult = moviedbdetails.Adult
	}
	if (movie.Title == "" || overwrite) && moviedbdetails.Title != "" {
		movie.Title = moviedbdetails.Title
		if strings.Contains(movie.Title, "&") || strings.Contains(movie.Title, "%") {
			movie.Title = html.UnescapeString(movie.Title)
		}
		if strings.Contains(movie.Title, "\\u") {
			unquote, err := strconv.Unquote("\"" + movie.Title + "\"")
			if err == nil {
				movie.Title = unquote
			}
		}
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}
	if (movie.Budget == 0 || overwrite) && moviedbdetails.Budget != 0 {
		movie.Budget = moviedbdetails.Budget
	}
	if moviedbdetails.ReleaseDate != "" && !movie.ReleaseDate.Valid {
		movie.ReleaseDate = logger.ParseDate(moviedbdetails.ReleaseDate, "2006-01-02")
		if (movie.Year == 0 || overwrite) && movie.ReleaseDate.Time.Year() != 0 {
			movie.Year = movie.ReleaseDate.Time.Year()
		}
	}
	if (movie.Genres == "" || overwrite) && len(moviedbdetails.Genres) != 0 {
		movie.Genres = ""
		for idxgenre := range moviedbdetails.Genres {
			if movie.Genres != "" {
				movie.Genres += ","
			}
			movie.Genres += moviedbdetails.Genres[idxgenre].Name
		}
	}
	if (movie.OriginalLanguage == "" || overwrite) && moviedbdetails.OriginalLanguage != "" {
		movie.OriginalLanguage = moviedbdetails.OriginalLanguage
	}
	if (movie.OriginalTitle == "" || overwrite) && moviedbdetails.OriginalTitle != "" {
		movie.OriginalTitle = moviedbdetails.OriginalTitle
	}
	if (movie.Overview == "" || overwrite) && moviedbdetails.Overview != "" {
		movie.Overview = moviedbdetails.Overview
	}
	if (movie.Popularity == 0 || overwrite) && moviedbdetails.Popularity != 0 {
		movie.Popularity = moviedbdetails.Popularity
	}
	if (movie.Revenue == 0 || overwrite) && moviedbdetails.Revenue != 0 {
		movie.Revenue = moviedbdetails.Revenue
	}
	if (movie.Runtime == 0) && moviedbdetails.Runtime != 0 {
		movie.Runtime = moviedbdetails.Runtime
	}
	if (movie.SpokenLanguages == "" || overwrite) && len(moviedbdetails.SpokenLanguages) != 0 {
		movie.SpokenLanguages = ""
		for idxlang := range moviedbdetails.SpokenLanguages {
			if movie.SpokenLanguages != "" {
				movie.SpokenLanguages += ","
			}
			movie.SpokenLanguages += moviedbdetails.SpokenLanguages[idxlang].EnglishName
		}
	}
	if (movie.Status == "" || overwrite) && moviedbdetails.Status != "" {
		movie.Status = moviedbdetails.Status
	}
	if (movie.Tagline == "" || overwrite) && moviedbdetails.Tagline != "" {
		movie.Tagline = moviedbdetails.Tagline
	}
	if (movie.VoteAverage == 0 || movie.VoteAverage == 0.0 || overwrite) && moviedbdetails.VoteAverage != 0 {
		movie.VoteAverage = moviedbdetails.VoteAverage
	}
	if (movie.VoteCount == 0 || overwrite) && moviedbdetails.VoteCount != 0 {
		movie.VoteCount = moviedbdetails.VoteCount
	}
	if (movie.Poster == "" || overwrite) && moviedbdetails.Poster != "" {
		movie.Poster = moviedbdetails.Poster
	}
	if (movie.Backdrop == "" || overwrite) && moviedbdetails.Backdrop != "" {
		movie.Backdrop = moviedbdetails.Backdrop
	}
	if (movie.MoviedbID == 0 || overwrite) && moviedbdetails.ID != 0 {
		movie.MoviedbID = moviedbdetails.ID
	}
	moviedbdetails.Close()
}

func (movie *Dbmovie) getOmdbMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	var omdbdetails apiexternal.OmDBMovie
	if apiexternal.OmdbAPI.GetMovie(movie.ImdbID, &omdbdetails) != nil {
		return
	}
	if (movie.Title == "" || overwrite) && omdbdetails.Title != "" {
		movie.Title = omdbdetails.Title
		if strings.Contains(movie.Title, "&") || strings.Contains(movie.Title, "%") {
			movie.Title = html.UnescapeString(movie.Title)
		}
		if strings.Contains(movie.Title, "\\u") {
			unquote, err := strconv.Unquote("\"" + movie.Title + "\"")
			if err == nil {
				movie.Title = unquote
			}
		}
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}
	if (movie.Genres == "" || overwrite) && omdbdetails.Genre != "" {
		movie.Genres = omdbdetails.Genre
	}
	if (movie.VoteCount == 0 || overwrite) && omdbdetails.ImdbVotes != "" {
		movie.VoteCount = logger.StringToInt(omdbdetails.ImdbVotes)
	}
	if ((movie.VoteAverage == 0 || movie.VoteAverage == 0.0) || overwrite) && omdbdetails.ImdbRating != "" {
		movie.VoteAverage = float32(logger.StringToInt(omdbdetails.ImdbRating))
	}
	if (movie.Year == 0 || overwrite) && omdbdetails.Year != "" {
		movie.Year = logger.StringToInt(omdbdetails.Year)
	}
	if (movie.URL == "" || overwrite) && omdbdetails.Website != "" {
		movie.URL = omdbdetails.Website
	}
	if (movie.Overview == "" || overwrite) && omdbdetails.Plot != "" {
		movie.Overview = omdbdetails.Plot
	}
}

func (movie *Dbmovie) getTraktMetadata(overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	traktdetails, err := apiexternal.TraktAPI.GetMovie(movie.ImdbID)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && traktdetails.Title != "" {
		movie.Title = traktdetails.Title
		if strings.Contains(movie.Title, "&") || strings.Contains(movie.Title, "%") {
			movie.Title = html.UnescapeString(movie.Title)
		}
		if strings.Contains(movie.Title, "\\u") {
			unquote, err := strconv.Unquote("\"" + movie.Title + "\"")
			if err == nil {
				movie.Title = unquote
			}
		}
	}
	if (movie.Slug == "" || overwrite) && traktdetails.Ids.Slug != "" {
		movie.Slug = traktdetails.Ids.Slug
	}
	if (movie.Genres == "" || overwrite) && len(traktdetails.Genres) != 0 {
		movie.Genres = strings.Join(traktdetails.Genres, ",")
	}
	if (movie.VoteCount == 0 || overwrite) && traktdetails.Votes != 0 {
		movie.VoteCount = traktdetails.Votes
	}
	if ((movie.VoteAverage == 0 || movie.VoteAverage == 0.0) || overwrite) && traktdetails.Rating != 0 {
		movie.VoteAverage = traktdetails.Rating
	}
	if (movie.Year == 0 || overwrite) && traktdetails.Year != 0 {
		movie.Year = traktdetails.Year
	}
	if (movie.Overview == "" || overwrite) && traktdetails.Overview != "" {
		movie.Overview = traktdetails.Overview
	}
	if (movie.Runtime == 0 || overwrite) && traktdetails.Runtime != 0 {
		movie.Runtime = traktdetails.Runtime
	}
	if (movie.Status == "" || overwrite) && traktdetails.Status != "" {
		movie.Status = traktdetails.Status
	}
	if (movie.MoviedbID == 0 || overwrite) && traktdetails.Ids.Tmdb != 0 {
		movie.MoviedbID = traktdetails.Ids.Tmdb
	}
	if (movie.TraktID == 0 || overwrite) && traktdetails.Ids.Trakt != 0 {
		movie.TraktID = traktdetails.Ids.Trakt
	}
	if (!movie.ReleaseDate.Valid || overwrite) && traktdetails.Released != "" {
		movie.ReleaseDate = logger.ParseDate(traktdetails.Released, "2006-01-02")
	}
	if (movie.OriginalLanguage == "" || overwrite) && traktdetails.Language != "" {
		movie.OriginalLanguage = traktdetails.Language
	}
	if (movie.Tagline == "" || overwrite) && traktdetails.Tagline != "" {
		movie.Tagline = traktdetails.Tagline
	}
	traktdetails.Close()
}
func (movie *Dbmovie) getMetadata(queryimdb bool, querytmdb bool, queryomdb bool, querytrakt bool) {
	logger.Log.GlobalLogger.Info("Get Metadata for", zap.Stringp("Title", &movie.ImdbID))

	if queryimdb {
		movie.getImdbMetadata(false)
	}
	if querytmdb {
		movie.getTmdbMetadata(false)
	}
	if queryomdb {
		movie.getOmdbMetadata(false)
	}
	if querytrakt {
		movie.getTraktMetadata(false)
	}
	logger.Log.GlobalLogger.Info("ENDED Get Metadata for", zap.Stringp("Title", &movie.ImdbID))
}

func (movie *Dbmovie) Getmoviemetadata(refresh bool) {
	if len(config.Cfg.General.MovieMetaSourcePriority) >= 1 {
		for idxmeta := range config.Cfg.General.MovieMetaSourcePriority {
			switch config.Cfg.General.MovieMetaSourcePriority[idxmeta] {
			case "imdb":
				movie.getImdbMetadata(refresh)
			case "tmdb":
				movie.getTmdbMetadata(false)
			case "omdb":
				movie.getOmdbMetadata(false)
			case "trakt":
				movie.getTraktMetadata(false)
			}
		}
	} else {
		movie.getMetadata(config.Cfg.General.MovieMetaSourceImdb, config.Cfg.General.MovieMetaSourceTmdb, config.Cfg.General.MovieMetaSourceOmdb, config.Cfg.General.MovieMetaSourceTrakt)
	}
}

func (movie *Dbmovie) Getmoviemetatitles(cfgp *config.MediaTypeConfig) {
	//var tmpl string
	//QueryColumn(Querywithargs{QueryString: "select listname from movies where dbmovie_id = ?", Args: []interface{}{dbmovie.ID}}, &tmpl)
	// if tmpl != "" && tmpl != cfgp.Name {
	// 	*cfgp = config.Cfg.Media[config.FindconfigTemplateOnList("movie_", tmpl)]
	// }
	if cfgp.Name == "" {
		return
	}
	var titles logger.InStringArrayStruct
	QueryStaticStringArray(false, 0, &Querywithargs{QueryString: "select title from dbmovie_titles where dbmovie_id = ?", Args: []interface{}{movie.ID}}, &titles.Arr)
	titlegroup := movie.getTitles(cfgp, config.Cfg.General.MovieAlternateTitleMetaSourceImdb, config.Cfg.General.MovieAlternateTitleMetaSourceTmdb, config.Cfg.General.MovieAlternateTitleMetaSourceTrakt)
	for idx := range titlegroup {
		if titlegroup[idx].Title == "" {
			continue
		}
		if !logger.InStringArray(titlegroup[idx].Title, &titles) {
			InsertNamed("Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (:title, :slug, :dbmovie_id, :region)", DbmovieTitle{Title: titlegroup[idx].Title, Slug: titlegroup[idx].Slug, DbmovieID: movie.ID, Region: titlegroup[idx].Region})
			titles.Arr = append(titles.Arr, titlegroup[idx].Title)
		}
	}
	titles.Close()
	titlegroup = nil
}
