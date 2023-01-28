package database

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
)

type Serie struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Listname       string
	Rootpath       string
	DbserieID      uint `db:"dbserie_id"`
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
	SearchSpecials bool `db:"search_specials"`
	IgnoreRuntime  bool `db:"ignore_runtime"`
}
type SerieJSON struct {
	ID             uint
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Listname       string
	Rootpath       string
	DbserieID      uint `db:"dbserie_id"`
	DontUpgrade    bool `db:"dont_upgrade"`
	DontSearch     bool `db:"dont_search"`
	SearchSpecials bool `db:"search_specials"`
	IgnoreRuntime  bool `db:"ignore_runtime"`
}
type SerieEpisode struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Lastscan         sql.NullTime
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DontUpgrade      bool `db:"dont_upgrade"`
	DontSearch       bool `db:"dont_search"`
	IgnoreRuntime    bool `db:"ignore_runtime"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	SerieID          uint `db:"serie_id"`
	DbserieID        uint `db:"dbserie_id"`
}
type SerieEpisodeJSON struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Lastscan         time.Time `db:"lastscan" json:"lastscan" time_format:"2006-01-02 22:00" time_utc:"1"`
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DontUpgrade      bool `db:"dont_upgrade"`
	DontSearch       bool `db:"dont_search"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	SerieID          uint `db:"serie_id"`
	DbserieID        uint `db:"dbserie_id"`
}
type SerieFileUnmatched struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Listname    string
	Filepath    string
	LastChecked sql.NullTime `db:"last_checked"`
	ParsedData  string       `db:"parsed_data"`
}
type SerieFileUnmatchedJSON struct {
	ID          uint
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
	Listname    string
	Filepath    string
	LastChecked time.Time `db:"last_checked" json:"last_checked" time_format:"2006-01-02 22:00" time_utc:"1"`
	ParsedData  string    `db:"parsed_data"`
}
type SerieEpisodeFile struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Location         string
	Filename         string
	Extension        string
	QualityProfile   string `db:"quality_profile"`
	Proper           bool
	Extended         bool
	Repack           bool
	Height           int
	Width            int
	ResolutionID     uint `db:"resolution_id"`
	QualityID        uint `db:"quality_id"`
	CodecID          uint `db:"codec_id"`
	AudioID          uint `db:"audio_id"`
	SerieID          uint `db:"serie_id"`
	SerieEpisodeID   uint `db:"serie_episode_id"`
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
	DbserieID        uint `db:"dbserie_id"`
}
type SerieEpisodeHistory struct {
	ID               uint
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
	Title            string
	URL              string
	Indexer          string
	SerieType        string `db:"type"`
	Target           string
	DownloadedAt     time.Time `db:"downloaded_at"`
	Blacklisted      bool
	QualityProfile   string `db:"quality_profile"`
	ResolutionID     uint   `db:"resolution_id"`
	QualityID        uint   `db:"quality_id"`
	CodecID          uint   `db:"codec_id"`
	AudioID          uint   `db:"audio_id"`
	SerieID          uint   `db:"serie_id"`
	SerieEpisodeID   uint   `db:"serie_episode_id"`
	DbserieEpisodeID uint   `db:"dbserie_episode_id"`
	DbserieID        uint   `db:"dbserie_id"`
}

type ResultSeries struct {
	Dbserie
	Listname  string
	Rootpath  string
	DbserieID uint `db:"dbserie_id"`
}
type Dbserie struct {
	ID              uint
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
	Seriename       string
	Aliases         string
	Season          string
	Status          string
	Firstaired      string
	Network         string
	Runtime         string
	Language        string
	Genre           string
	Overview        string
	Rating          string
	Siterating      string
	SiteratingCount string `db:"siterating_count"`
	Slug            string
	TraktID         int    `db:"trakt_id"`
	ImdbID          string `db:"imdb_id"`
	ThetvdbID       int    `db:"thetvdb_id"`
	FreebaseMID     string `db:"freebase_m_id"`
	FreebaseID      string `db:"freebase_id"`
	TvrageID        int    `db:"tvrage_id"`
	Facebook        string
	Instagram       string
	Twitter         string
	Banner          string
	Poster          string
	Fanart          string
	Identifiedby    string
}

type DbserieAlternate struct {
	ID        uint
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
	Title     string
	Slug      string
	Region    string
	DbserieID uint `db:"dbserie_id"`
}

type ResultSerieEpisodes struct {
	DbserieEpisode
	Listname         string
	Rootpath         string
	Lastscan         sql.NullTime
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
}
type ResultSerieEpisodesJSON struct {
	DbserieEpisodeJSON
	Listname         string
	Rootpath         string
	Lastscan         time.Time `db:"lastscan" json:"lastscan" time_format:"2006-01-02 22:00" time_utc:"1"`
	Blacklisted      bool
	QualityReached   bool   `db:"quality_reached"`
	QualityProfile   string `db:"quality_profile"`
	Missing          bool
	DbserieEpisodeID uint `db:"dbserie_episode_id"`
}

type DbserieEpisode struct {
	ID         uint
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
	Episode    string
	Season     string
	Identifier string
	Title      string
	FirstAired sql.NullTime `db:"first_aired" json:"first_aired" time_format:"2006-01-02" time_utc:"1"`
	Overview   string
	Poster     string
	Runtime    int
	DbserieID  uint `db:"dbserie_id"`
}
type DbserieEpisodeJSON struct {
	ID         uint
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
	Episode    string
	Season     string
	Identifier string
	Title      string
	FirstAired time.Time `db:"first_aired" json:"first_aired" time_format:"2006-01-02" time_utc:"1"`
	Overview   string
	Poster     string
	DbserieID  uint `db:"dbserie_id"`
}

func (serie *Dbserie) getMetadataTmdb(overwrite bool) {
	if serie.ThetvdbID == 0 || (serie.Seriename != "" && !overwrite) {
		return
	}
	moviedb, err := apiexternal.TmdbAPI.FindTvdb(serie.ThetvdbID)
	if err != nil {
		return
	}
	defer moviedb.Close()
	if len(moviedb.TvResults) == 0 {
		logger.Log.GlobalLogger.Warn("Serie tmdb data not found for", zap.Int("tvdb", serie.ThetvdbID))
		return
	}
	if (serie.Seriename == "" || overwrite) && moviedb.TvResults[0].Name != "" {
		serie.Seriename = moviedb.TvResults[0].Name
	}
	// var moviedbexternal apiexternal.TheMovieDBTVExternal
	// err := apiexternal.TmdbApi.GetTVExternal(moviedb.TvResults[0].ID, moviedbexternal)
	// if err == nil {
	// 	serie.FreebaseMID = moviedbexternal.FreebaseMID
	// 	serie.FreebaseID = moviedbexternal.FreebaseID
	// 	serie.Facebook = moviedbexternal.FacebookID
	// 	serie.Instagram = moviedbexternal.InstagramID
	// 	serie.Twitter = moviedbexternal.TwitterID
	// } else {
	// 	logger.Log.GlobalLogger.Warn("Serie tmdb externals not found for: ", serie.ThetvdbID)
	// }
}
func (serie *Dbserie) getMetadataTrakt(overwrite bool) {
	if serie.ImdbID == "" {
		return
	}
	traktdetails, err := apiexternal.TraktAPI.GetSerie(serie.ImdbID)
	if err != nil {
		logger.Log.GlobalLogger.Warn("Serie trakt data not found for", zap.Int("tvdb", serie.ThetvdbID))
		return
	}
	if (serie.Genre == "" || overwrite) && len(traktdetails.Genres) >= 1 {
		serie.Genre = strings.Join(traktdetails.Genres, ",")
	}
	if (serie.Language == "" || overwrite) && traktdetails.Language != "" {
		serie.Language = traktdetails.Language
	}
	if (serie.Network == "" || overwrite) && traktdetails.Network != "" {
		serie.Network = traktdetails.Network
	}
	if (serie.Overview == "" || overwrite) && traktdetails.Overview != "" {
		serie.Overview = traktdetails.Overview
	}
	if (serie.Rating == "" || overwrite) && traktdetails.Rating != 0 {
		serie.Rating = strconv.FormatFloat(float64(traktdetails.Rating), 'f', 4, 64) //fmt.Sprintf("%f", traktdetails.Rating)
	}
	if (serie.Runtime == "" || overwrite) && traktdetails.Runtime != 0 {
		serie.Runtime = logger.IntToString(traktdetails.Runtime)
	}
	if (serie.Seriename == "" || overwrite) && traktdetails.Title != "" {
		serie.Seriename = traktdetails.Title
	}
	if (serie.Slug == "" || overwrite) && traktdetails.Ids.Slug != "" {
		serie.Slug = traktdetails.Ids.Slug
	}
	if (serie.Status == "" || overwrite) && traktdetails.Status != "" {
		serie.Status = traktdetails.Status
	}
	if (serie.ThetvdbID == 0 || overwrite) && traktdetails.Ids.Tvdb != 0 {
		serie.ThetvdbID = traktdetails.Ids.Tvdb
	}
	if (serie.TraktID == 0 || overwrite) && traktdetails.Ids.Trakt != 0 {
		serie.TraktID = traktdetails.Ids.Trakt
	}
	if (serie.TvrageID == 0 || overwrite) && traktdetails.Ids.Tvrage != 0 {
		serie.TvrageID = traktdetails.Ids.Tvrage
	}
	if (serie.Firstaired == "" || overwrite) && traktdetails.FirstAired.String() != "" {
		serie.Firstaired = traktdetails.FirstAired.String()
	}
	traktdetails.Close()
}
func (serie *Dbserie) getMetadataTvdb(language string, overwrite bool) []string {
	if serie.ThetvdbID == 0 {
		return []string{}
	}
	tvdbdetails, err := apiexternal.TvdbAPI.GetSeries(serie.ThetvdbID, language)
	if err != nil {
		logger.Log.GlobalLogger.Warn("Serie tvdb data not found for", zap.Int("tvdb", serie.ThetvdbID), zap.Error(err))
		return []string{}
	}
	if (serie.Seriename == "" || overwrite) && tvdbdetails.Data.SeriesName != "" {
		serie.Seriename = tvdbdetails.Data.SeriesName
	}
	if (serie.Aliases == "" || overwrite) && len(tvdbdetails.Data.Aliases) >= 1 {
		serie.Aliases = strings.Join(tvdbdetails.Data.Aliases, ",")
	}
	if (serie.Season == "" || overwrite) && tvdbdetails.Data.Season != "" {
		serie.Season = tvdbdetails.Data.Season
	}
	if (serie.Status == "" || overwrite) && tvdbdetails.Data.Status != "" {
		serie.Status = tvdbdetails.Data.Status
	}
	if (serie.Firstaired == "" || overwrite) && tvdbdetails.Data.FirstAired != "" {
		serie.Firstaired = tvdbdetails.Data.FirstAired
	}
	if (serie.Network == "" || overwrite) && tvdbdetails.Data.Network != "" {
		serie.Network = tvdbdetails.Data.Network
	}
	if (serie.Runtime == "" || overwrite) && tvdbdetails.Data.Runtime != "" {
		serie.Runtime = tvdbdetails.Data.Runtime
	}
	if (serie.Language == "" || overwrite) && tvdbdetails.Data.Language != "" {
		serie.Language = tvdbdetails.Data.Language
	}
	if (serie.Genre == "" || overwrite) && len(tvdbdetails.Data.Genre) >= 1 {
		serie.Genre = strings.Join(tvdbdetails.Data.Genre, ",")
	}
	if (serie.Overview == "" || overwrite) && tvdbdetails.Data.Overview != "" {
		serie.Overview = tvdbdetails.Data.Overview
	}
	if (serie.Rating == "" || overwrite) && tvdbdetails.Data.Rating != "" {
		serie.Rating = tvdbdetails.Data.Rating
	}
	if (serie.Siterating == "" || overwrite) && tvdbdetails.Data.SiteRating != 0 {
		serie.Siterating = strconv.FormatFloat(float64(tvdbdetails.Data.SiteRating), 'f', 1, 32)
	}
	if (serie.SiteratingCount == "" || overwrite) && tvdbdetails.Data.SiteRatingCount != 0 {
		serie.SiteratingCount = logger.IntToString(tvdbdetails.Data.SiteRatingCount)
	}
	if (serie.Slug == "" || overwrite) && tvdbdetails.Data.Slug != "" {
		serie.Slug = tvdbdetails.Data.Slug
	}
	if (serie.Banner == "" || overwrite) && tvdbdetails.Data.Banner != "" {
		serie.Banner = tvdbdetails.Data.Banner
	}
	if (serie.Poster == "" || overwrite) && tvdbdetails.Data.Poster != "" {
		serie.Poster = tvdbdetails.Data.Poster
	}
	if (serie.Fanart == "" || overwrite) && tvdbdetails.Data.Fanart != "" {
		serie.Fanart = tvdbdetails.Data.Fanart
	}
	if (serie.ImdbID == "" || overwrite) && tvdbdetails.Data.ImdbID != "" {
		serie.ImdbID = tvdbdetails.Data.ImdbID
	}
	defer tvdbdetails.Close()
	return tvdbdetails.Data.Aliases
}
func (serie *Dbserie) GetMetadata(language string, querytmdb bool, querytrakt bool, overwrite bool, returnaliases bool) []string {
	aliases := serie.getMetadataTvdb(language, overwrite)
	if querytmdb {
		serie.getMetadataTmdb(false)
	}
	if querytrakt && serie.ImdbID != "" {
		serie.getMetadataTrakt(false)
		if returnaliases {
			traktaliases, err := apiexternal.TraktAPI.GetSerieAliases(serie.ImdbID)

			if err == nil {
				arrcfglang := logger.InStringArrayStruct{Arr: config.Cfg.Imdbindexer.Indexedlanguages}
				lenarr := len(arrcfglang.Arr)
				for idxalias := range traktaliases.Aliases {
					if logger.InStringArray(traktaliases.Aliases[idxalias].Country, &arrcfglang) && lenarr >= 1 {
						aliases = append(aliases, traktaliases.Aliases[idxalias].Title)
					}
				}
				arrcfglang.Close()
				traktaliases.Close()
			}
		}
	}
	return aliases
}
