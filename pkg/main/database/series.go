package database

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
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
type SerieJson struct {
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
type SerieEpisodeJson struct {
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
type SerieFileUnmatchedJson struct {
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
type ResultSerieEpisodesJson struct {
	DbserieEpisodeJson
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
type DbserieEpisodeJson struct {
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

func (serie *Dbserie) GetMetadata(language string, querytmdb bool, querytrakt bool, overwrite bool) []string {
	var aliases []string
	defer logger.ClearVar(&aliases)

	if querytmdb {
		moviedb, found := apiexternal.TmdbApi.FindTvdb(serie.ThetvdbID)
		defer logger.ClearVar(&moviedb)
		if found == nil {
			if len(moviedb.TvResults) >= 1 {
				moviedbexternal, foundexternal := apiexternal.TmdbApi.GetTVExternal(moviedb.TvResults[0].ID)
				defer logger.ClearVar(&moviedbexternal)
				if foundexternal == nil {
					serie.FreebaseMID = moviedbexternal.FreebaseMID
					serie.FreebaseID = moviedbexternal.FreebaseID
					serie.Facebook = moviedbexternal.FacebookID
					serie.Instagram = moviedbexternal.InstagramID
					serie.Twitter = moviedbexternal.TwitterID
				} else {
					logger.Log.Warning("Serie externals not found for: ", serie.ThetvdbID)
				}
			} else {
				logger.Log.Warning("Serie data not found for: ", serie.ThetvdbID)
			}
		}
	}
	tvdbdetails, founddetail := apiexternal.TvdbApi.GetSeries(serie.ThetvdbID, language)
	defer logger.ClearVar(&tvdbdetails)

	if founddetail == nil {

		serie.Seriename = tvdbdetails.Data.SeriesName
		serie.Aliases = strings.Join(tvdbdetails.Data.Aliases, ",")
		serie.Season = tvdbdetails.Data.Season
		serie.Status = tvdbdetails.Data.Status
		serie.Firstaired = tvdbdetails.Data.FirstAired
		serie.Network = tvdbdetails.Data.Network
		serie.Runtime = tvdbdetails.Data.Runtime
		serie.Language = tvdbdetails.Data.Language
		serie.Genre = strings.Join(tvdbdetails.Data.Genre, ",")
		serie.Overview = tvdbdetails.Data.Overview
		serie.Rating = tvdbdetails.Data.Rating
		serie.Siterating = strconv.FormatFloat(float64(tvdbdetails.Data.SiteRating), 'f', 1, 32)
		serie.SiteratingCount = strconv.Itoa(tvdbdetails.Data.SiteRatingCount)
		serie.Slug = tvdbdetails.Data.Slug
		serie.Banner = tvdbdetails.Data.Banner
		serie.Poster = tvdbdetails.Data.Poster
		serie.Fanart = tvdbdetails.Data.Fanart
		serie.ImdbID = tvdbdetails.Data.ImdbID
		aliases = append(aliases, tvdbdetails.Data.Aliases...)
	} else {
		logger.Log.Warning("Serie tvdb data not found for: ", serie.ThetvdbID)
		return []string{}
	}
	if querytrakt && serie.ImdbID != "" {
		traktdetails, trakterr := apiexternal.TraktApi.GetSerie(serie.ImdbID)
		defer logger.ClearVar(&traktdetails)
		if trakterr == nil {
			if serie.Genre == "" || overwrite {
				serie.Genre = strings.Join(traktdetails.Genres, ",")
			}
			if serie.Language == "" || overwrite {
				serie.Language = traktdetails.Language
			}
			if serie.Network == "" || overwrite {
				serie.Network = traktdetails.Network
			}
			if serie.Overview == "" || overwrite {
				serie.Overview = traktdetails.Overview
			}
			if serie.Rating == "" || overwrite {
				serie.Rating = fmt.Sprintf("%f", traktdetails.Rating)
			}
			if serie.Runtime == "" || overwrite {
				serie.Runtime = strconv.Itoa(traktdetails.Runtime)
			}
			if serie.Seriename == "" || overwrite {
				serie.Seriename = traktdetails.Title
			}
			if serie.Slug == "" || overwrite {
				serie.Slug = traktdetails.Ids.Slug
			}
			if serie.Status == "" || overwrite {
				serie.Status = traktdetails.Status
			}
			if serie.ThetvdbID == 0 || overwrite {
				serie.ThetvdbID = traktdetails.Ids.Tvdb
			}
			if serie.TraktID == 0 || overwrite {
				serie.TraktID = traktdetails.Ids.Trakt
			}
			if serie.TvrageID == 0 || overwrite {
				serie.TvrageID = traktdetails.Ids.Tvrage
			}
			if serie.Firstaired == "" || overwrite {
				serie.Firstaired = traktdetails.FirstAired.String()
			}
		}
		traktaliases, trakterr := apiexternal.TraktApi.GetSerieAliases(serie.ImdbID)
		defer logger.ClearVar(&traktaliases)

		cfg_imdb := config.ConfigGet("imdb").Data.(config.ImdbConfig)
		if trakterr == nil {
			for idxalias := range traktaliases {
				regionok := false
				for idxlang := range cfg_imdb.Indexedlanguages {
					if strings.EqualFold(cfg_imdb.Indexedlanguages[idxlang], traktaliases[idxalias].Country) {
						regionok = true
						break
					}
				}
				if !regionok && len(cfg_imdb.Indexedlanguages) >= 1 {
					continue
				}
				aliases = append(aliases, traktaliases[idxalias].Title)
			}
		}
	}
	return aliases
}

func (serie *Dbserie) GetTitles(configTemplate string, queryimdb bool, querytrakt bool) []DbserieAlternate {

	var c []DbserieAlternate
	defer logger.ClearVar(&c)
	var processed []string
	defer logger.ClearVar(&processed)
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if queryimdb {
		queryimdbid := serie.ImdbID
		if !strings.HasPrefix(serie.ImdbID, "tt") {
			queryimdbid = "tt" + serie.ImdbID
		}
		imdbakadata, _ := QueryImdbAka(Query{Where: "tconst = ? COLLATE NOCASE", WhereArgs: []interface{}{queryimdbid}})
		defer logger.ClearVar(&imdbakadata)
		for idxaka := range imdbakadata {
			regionok := false
			for idxlang := range configEntry.Metadata_title_languages {
				if strings.EqualFold(configEntry.Metadata_title_languages[idxlang], imdbakadata[idxaka].Region) {
					regionok = true
					break
				}
			}
			logger.Log.Debug("Title: ", imdbakadata[idxaka].Title, " Region: ", imdbakadata[idxaka].Region, " ok: ", regionok)
			if !regionok && len(configEntry.Metadata_title_languages) >= 1 {
				continue
			}
			c = append(c, DbserieAlternate{DbserieID: serie.ID, Title: imdbakadata[idxaka].Title, Slug: imdbakadata[idxaka].Slug, Region: imdbakadata[idxaka].Region})
			processed = append(processed, imdbakadata[idxaka].Title)
		}
	}
	if querytrakt {
		queryid := serie.ImdbID
		if queryid == "" {
			queryid = strconv.Itoa(serie.TraktID)
		}
		traktaliases, err := apiexternal.TraktApi.GetSerieAliases(queryid)
		defer logger.ClearVar(&traktaliases)
		if err == nil {
			for idxalias := range traktaliases {
				regionok := false
				for idxlang := range configEntry.Metadata_title_languages {
					if strings.EqualFold(configEntry.Metadata_title_languages[idxlang], traktaliases[idxalias].Country) {
						regionok = true
						break
					}
				}
				logger.Log.Debug("Title: ", traktaliases[idxalias].Title, " Region: ", traktaliases[idxalias].Country, " ok: ", regionok)
				if !regionok && len(configEntry.Metadata_title_languages) >= 1 {
					continue
				}
				foundentry := false
				for idxproc := range processed {
					if processed[idxproc] == traktaliases[idxalias].Title {
						foundentry = true
						break
					}
				}
				if !foundentry {
					c = append(c, DbserieAlternate{DbserieID: serie.ID, Title: traktaliases[idxalias].Title, Slug: logger.StringToSlug(traktaliases[idxalias].Title), Region: traktaliases[idxalias].Country})
					processed = append(processed, traktaliases[idxalias].Title)
				}
			}
		}
	}
	return c
}

func (serie *Dbserie) GetEpisodes(language string, querytrakt bool) []DbserieEpisode {
	var epi []DbserieEpisode
	defer logger.ClearVar(&epi)

	if serie.ThetvdbID != 0 {
		tvdbdetails, founddetail := apiexternal.TvdbApi.GetSeriesEpisodes(serie.ThetvdbID, language)
		defer logger.ClearVar(&tvdbdetails)

		if founddetail == nil {
			var episode DbserieEpisode
			var layout string
			var t time.Time
			var terr error
			for idx := range tvdbdetails.Data {
				episode = DbserieEpisode{}
				episode.Episode = strconv.Itoa(tvdbdetails.Data[idx].AiredEpisodeNumber)
				episode.Season = strconv.Itoa(tvdbdetails.Data[idx].AiredSeason)
				episode.Identifier = "S" + padNumberWithZero(tvdbdetails.Data[idx].AiredSeason) + "E" + padNumberWithZero(tvdbdetails.Data[idx].AiredEpisodeNumber)
				episode.Title = tvdbdetails.Data[idx].EpisodeName
				if tvdbdetails.Data[idx].FirstAired != "" {
					layout = "2006-01-02" //year-month-day
					t, terr = time.Parse(layout, tvdbdetails.Data[idx].FirstAired)
					if terr == nil {
						episode.FirstAired = sql.NullTime{Time: t, Valid: true}
					}
				}
				episode.Overview = tvdbdetails.Data[idx].Overview
				episode.Poster = tvdbdetails.Data[idx].Poster
				episode.DbserieID = serie.ID
				epi = append(epi, episode)
			}
		} else {
			logger.Log.Warning("Serie episode not found for: ", serie.ThetvdbID, founddetail)
		}
	}
	if querytrakt && serie.ImdbID != "" {
		seasons, err := apiexternal.TraktApi.GetSerieSeasons(serie.ImdbID)
		defer logger.ClearVar(&seasons)
		if err == nil {
			var episodes []apiexternal.TraktSerieSeasonEpisodes
			defer logger.ClearVar(&episodes)
			var breakloop bool
			var episode DbserieEpisode
			for idxseason := range seasons {
				episodes, err = apiexternal.TraktApi.GetSerieSeasonEpisodes(serie.ImdbID, seasons[idxseason].Number)
				if err == nil {
					for idxepi := range episodes {
						breakloop = false
						for idxadded, added := range epi {
							if added.Season == strconv.Itoa(episodes[idxepi].Season) && added.Episode == strconv.Itoa(episodes[idxepi].Episode) {
								breakloop = true
								if added.Title == "" {
									epi[idxadded].Title = episodes[idxepi].Title
								}
								if added.FirstAired.Time.IsZero() {
									epi[idxadded].FirstAired = sql.NullTime{Time: episodes[idxepi].FirstAired, Valid: true}
								}
								if added.Overview == "" {
									epi[idxadded].Overview = episodes[idxepi].Overview
								}
								if added.Runtime == 0 {
									epi[idxadded].Runtime = episodes[idxepi].Runtime
								}
								break
							}
						}
						if breakloop {
							continue
						}
						episode = DbserieEpisode{}
						episode.Episode = strconv.Itoa(episodes[idxepi].Episode)
						episode.Season = strconv.Itoa(episodes[idxepi].Season)
						episode.Identifier = "S" + padNumberWithZero(episodes[idxepi].Season) + "E" + padNumberWithZero(episodes[idxepi].Episode)
						episode.Title = episodes[idxepi].Title
						episode.FirstAired = sql.NullTime{Time: episodes[idxepi].FirstAired, Valid: true}
						episode.Overview = episodes[idxepi].Overview
						episode.DbserieID = serie.ID
						episode.Runtime = episodes[idxepi].Runtime
						epi = append(epi, episode)
					}
				}
			}
		}
	}
	return epi
}
func padNumberWithZero(value int) string {
	return fmt.Sprintf("%02d", value)
}
