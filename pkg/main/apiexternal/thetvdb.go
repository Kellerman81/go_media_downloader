package apiexternal

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

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
		UserAgent:                 "go-media-downloader/2.0",
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
		for _, g := range details.Genres {
			series.Data.Genre = append(series.Data.Genre, g.Name)
		}

		return series, nil
	}

	return nil, errors.New("client empty")
}

// GetTvdbSeriesEpisodes retrieves all episodes for the given TV series ID from
// TheTVDB API. It accepts the series ID, preferred language, and database series
// ID. It retrieves the episode data, checks for existing episodes to avoid
// duplicates, and inserts any missing episodes into the database. If there are
// multiple pages of results, it fetches additional pages.
func UpdateTvdbSeriesEpisodes(id int, language string, dbid *uint) {
	// Use v2 provider if available
	if provider := providers.GetTVDB(); provider != nil {
		episodes, err := provider.GetAllEpisodes(context.Background(), id)
		if err != nil {
			logger.Logtype("error", 1).
				Int("series_id", id).
				Err(err).
				Msg("Error getting episodes from v2 provider")

			return
		}

		// Get existing episodes to avoid duplicates
		tbl := database.Getrowssize[database.DbstaticTwoString](
			false,
			database.QueryDbserieEpisodesCountByDBID,
			database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
			dbid,
		)

		// Insert missing episodes
		for _, ep := range episodes {
			if checkdbtwostrings(tbl, ep.SeasonNumber, ep.EpisodeNumber) {
				continue
			}

			epi := strconv.Itoa(ep.EpisodeNumber)
			seas := strconv.Itoa(ep.SeasonNumber)
			ident := generateIdentifierStringFromInt(&ep.SeasonNumber, &ep.EpisodeNumber)

			database.ExecN(
				"insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
				&epi,
				&seas,
				&ident,
				&ep.Name,
				&ep.AirDate,
				&ep.Overview,
				&ep.StillPath,
				dbid,
			)
		}

		return
	}
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
