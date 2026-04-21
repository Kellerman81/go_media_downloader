package apiexternal

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// CollectedEpisode represents episode data collected from TVDB and/or Trakt.
// This struct is used to merge data from both sources before writing to the database.
type CollectedEpisode struct {
	Season         int
	Episode        int
	AbsoluteNumber int
	Title          string
	FirstAired     time.Time
	Overview       string
	Poster         string
}

type TheTVDBSeries struct {
	Data struct {
		Aliases         []string `json:"aliases"`
		Genre           []string `json:"genre"`
		SeriesName      string   `json:"seriesName"`
		Season          string   `json:"season"`
		Status          string   `json:"status"`
		FirstAired      string   `json:"firstAired"`
		Network         string   `json:"network"`
		Runtime         string   `json:"runtime"`
		Language        string   `json:"language"`
		Overview        string   `json:"overview"`
		Rating          string   `json:"rating"`
		ImdbID          string   `json:"imdbId"`
		Slug            string   `json:"slug"`
		Banner          string   `json:"banner"`
		Poster          string   `json:"poster"`
		Fanart          string   `json:"fanart"`
		SiteRating      float32  `json:"siteRating"`
		ID              int      `json:"id"`
		SiteRatingCount int      `json:"siteRatingCount"`
		// SeriesID        any      `json:"seriesId"`
		// NetworkID       string   `json:"networkId"`
	} `json:"data"`
}

type TheTVDBEpisodes struct {
	Data []struct {
		EpisodeName        string `json:"episodeName"`
		FirstAired         string `json:"firstAired"`
		Overview           string `json:"overview"`
		Poster             string `json:"filename"`
		AiredSeason        int    `json:"airedSeason"`
		AiredEpisodeNumber int    `json:"airedEpisodeNumber"`
		// ID                 int    `json:"id"`
		// Language           TheTVDBEpisodeLanguage `json:"language"`
		// ProductionCode     string                 `json:"productionCode"`
		// ShowURL            string                 `json:"showUrl"`
		// SeriesID           int                    `json:"seriesId"`
		// ImdbID string `json:"imdbId"`
		// ContentRating      string                 `json:"contentRating"`
		// SiteRating         float32                `json:"siteRating"`
		// SiteRatingCount    int                    `json:"siteRatingCount"`
		// IsMovie            int                    `json:"isMovie"`
	} `json:"data"`
	Links struct {
		First int `json:"first"`
		Last  int `json:"last"`
		Next  int `json:"next"`
	} `json:"links"`
}

// NewTvdbClient creates a new tvdbClient instance for making requests to
// the TheTVDB API. It configures rate limiting and TLS based on the
// provided parameters.
func NewTvdbClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}

	if calls == 0 {
		calls = 1
	}

	// Create v2 provider with empty credentials (will need to be set separately)
	tvdbConfig := base.ClientConfig{
		Name:                      "tvdb",
		BaseURL:                   "https://api.thetvdb.com",
		Timeout:                   time.Duration(timeoutseconds) * time.Second,
		AuthType:                  base.AuthNone, // TVDB uses JWT, handled by provider
		RateLimitCalls:            calls,
		RateLimitSeconds:          int(seconds),
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		EnableStats:               true,
		UserAgent:                 config.GetSettingsGeneral().UserAgent,
		DisableTLSVerify:          disabletls,
	}
	// Note: apiKey, userKey, username would need to be provided from config
	if provider := tvdb.NewProviderWithConfig(tvdbConfig, "", "", ""); provider != nil {
		providers.SetTVDB(provider)
	}
}

// GetTvdbSeries retrieves TV series data from the TheTVDB API for the given series ID.
// If a non-empty language is provided, it will be set in the API request headers.
// Returns the TV series data, or an error if one occurs.
func GetTvdbSeries(id int, language string) (*TheTVDBSeries, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	// Use v2 provider if available
	if provider := providers.GetTVDB(); provider != nil {
		details, err := provider.GetSeriesByID(context.Background(), id)
		if err != nil {
			return nil, err
		}

		// Convert v2 SeriesDetails to old TheTVDBSeries format
		series := &TheTVDBSeries{}

		series.Data.ID = details.ID
		series.Data.SeriesName = details.Name
		series.Data.Status = details.Status
		series.Data.Overview = details.Overview
		series.Data.ImdbID = details.IMDbID
		series.Data.Poster = details.PosterPath
		series.Data.Fanart = details.BackdropPath
		series.Data.SiteRating = float32(details.VoteAverage)
		series.Data.SiteRatingCount = details.VoteCount

		if !details.FirstAirDate.IsZero() {
			series.Data.FirstAired = details.FirstAirDate.Format("2006-01-02")
		}

		if len(details.EpisodeRunTime) > 0 {
			series.Data.Runtime = strconv.Itoa(details.EpisodeRunTime[0])
		}

		series.Data.Language = details.OriginalLanguage

		if len(details.Networks) > 0 {
			series.Data.Network = details.Networks[0].Name
		}

		// Convert genres
		for i := range details.Genres {
			series.Data.Genre = append(series.Data.Genre, details.Genres[i].Name)
		}

		return series, nil
	}

	return nil, errors.New("client empty")
}

// CollectTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API and returns them as a map keyed by "season-episode" for easy merging.
// This function does NOT write to the database - use WriteCollectedEpisodesToDB for that.
func CollectTvdbSeriesEpisodes(id int, language string) map[string]*CollectedEpisode {
	episodes := make(map[string]*CollectedEpisode)

	// Use v2 provider if available
	if provider := providers.GetTVDB(); provider != nil {
		apiEpisodes, err := provider.GetAllEpisodes(context.Background(), id)
		if err != nil {
			logger.Logtype("error", 1).
				Int("series_id", id).
				Err(err).
				Msg("Error getting episodes from v2 provider")

			return episodes
		}

		for i := range apiEpisodes {
			ep := apiEpisodes[i]

			episodes[strconv.Itoa(ep.SeasonNumber)+"-"+strconv.Itoa(ep.EpisodeNumber)] = &CollectedEpisode{
				Season:         ep.SeasonNumber,
				Episode:        ep.EpisodeNumber,
				AbsoluteNumber: ep.AbsoluteNumber,
				Title:          ep.Name,
				FirstAired:     ep.AirDate,
				Overview:       ep.Overview,
				Poster:         ep.StillPath,
			}
		}
	}

	return episodes
}

// WriteCollectedEpisodesToDB writes the collected episodes to the database.
// It checks for existing episodes and updates them, or inserts new ones.
func WriteCollectedEpisodesToDB(episodes map[string]*CollectedEpisode, dbid *uint) {
	if len(episodes) == 0 {
		return
	}

	// Get existing episodes to check for updates
	tbl := database.Getrowssize[database.DbstaticTwoString](
		false,
		database.QueryDbserieEpisodesCountByDBID,
		database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
		dbid,
	)

	var epi, seas, ident string
	for _, ep := range episodes {
		epi = strconv.Itoa(ep.Episode)
		seas = strconv.Itoa(ep.Season)
		ident = generateIdentifierStringFromInt(&ep.Season, &ep.Episode)

		if checkdbtwostrings(tbl, ep.Season, ep.Episode) {
			// Episode exists - update it
			database.ExecN(
				"UPDATE dbserie_episodes SET title = ?, first_aired = ?, overview = ?, poster = ?, absolute_episode = ?, updated_at = CURRENT_TIMESTAMP WHERE dbserie_id = ? AND season = ? AND episode = ?",
				&ep.Title,
				&ep.FirstAired,
				&ep.Overview,
				&ep.Poster,
				&ep.AbsoluteNumber,
				dbid,
				&seas,
				&epi,
			)
		} else {
			// Episode doesn't exist - insert it
			database.ExecN(
				"INSERT INTO dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, absolute_episode, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
				&epi,
				&seas,
				&ident,
				&ep.Title,
				&ep.FirstAired,
				&ep.Overview,
				&ep.Poster,
				&ep.AbsoluteNumber,
				dbid,
			)
		}
	}
}

// UpdateTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API and writes them to the database. This is a convenience function that
// combines CollectTvdbSeriesEpisodes and WriteCollectedEpisodesToDB.
// Deprecated: Use CollectTvdbSeriesEpisodes + MergeTraktIntoCollectedEpisodes + WriteCollectedEpisodesToDB instead.
func UpdateTvdbSeriesEpisodes(id int, language string, dbid *uint) {
	episodes := CollectTvdbSeriesEpisodes(id, language)
	WriteCollectedEpisodesToDB(episodes, dbid)
}

// TestTVDBConnectivity tests the connectivity to the TVDB API
// Note: timeout parameter is currently unused as ProcessHTTPNoRateCheck handles its own timeouts
// Returns status code and error if any.
func TestTVDBConnectivity(timeout time.Duration) (int, error) {
	// Use v2 provider if available
	if provider := providers.GetTVDB(); provider != nil {
		// Test with a simple series lookup
		_, err := provider.GetSeriesByID(context.Background(), 1)
		if err != nil {
			return 0, err
		}

		return 200, nil
	}

	return 400, errors.New("client empty")
}
