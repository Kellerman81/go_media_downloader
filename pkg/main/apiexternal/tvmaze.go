package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tvmaze"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

type TVmazeSearchResults []TVmazeShow

type TVmazeShow struct {
	ID             int              `json:"id"`
	URL            string           `json:"url"`
	Name           string           `json:"name"`
	Type           string           `json:"type"`
	Language       string           `json:"language"`
	Genres         []string         `json:"genres"`
	Status         string           `json:"status"`
	Runtime        int              `json:"runtime"`
	AverageRuntime int              `json:"averageRuntime"`
	Premiered      string           `json:"premiered"`
	Ended          string           `json:"ended"`
	OfficialSite   string           `json:"officialSite"`
	Schedule       TVmazeSchedule   `json:"schedule"`
	Rating         TVmazeRating     `json:"rating"`
	Weight         int              `json:"weight"`
	Network        TVmazeNetwork    `json:"network"`
	WebChannel     TVmazeWebChannel `json:"webChannel"`
	DVDCountry     TVmazeDVDCountry `json:"dvdCountry"`
	Externals      TVmazeExternals  `json:"externals"`
	Image          TVmazeImage      `json:"image"`
	Summary        string           `json:"summary"`
	Updated        int64            `json:"updated"`
}

type TVmazeSchedule struct {
	Time string   `json:"time"`
	Days []string `json:"days"`
}

type TVmazeRating struct {
	Average float64 `json:"average"`
}

type TVmazeNetwork struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Country      TVmazeCountry `json:"country"`
	OfficialSite string        `json:"officialSite"`
}

type TVmazeWebChannel struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Country      TVmazeCountry `json:"country"`
	OfficialSite string        `json:"officialSite"`
}

type TVmazeCountry struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Timezone string `json:"timezone"`
}

type TVmazeDVDCountry struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Timezone string `json:"timezone"`
}

type TVmazeExternals struct {
	TVDB int    `json:"tvdb"`
	IMDB string `json:"imdb"`
	TMDb int    `json:"thetvdb"`
}

type TVmazeImage struct {
	Medium   string `json:"medium"`
	Original string `json:"original"`
}

type TVmazeEpisode struct {
	ID       int          `json:"id"`
	URL      string       `json:"url"`
	Name     string       `json:"name"`
	Season   int          `json:"season"`
	Number   int          `json:"number"`
	Type     string       `json:"type"`
	Airdate  string       `json:"airdate"`
	Airtime  string       `json:"airtime"`
	Airstamp string       `json:"airstamp"`
	Runtime  int          `json:"runtime"`
	Rating   TVmazeRating `json:"rating"`
	Image    TVmazeImage  `json:"image"`
	Summary  string       `json:"summary"`
}

type TVmazeSeasons []TVmazeSeason

type TVmazeSeason struct {
	ID           int              `json:"id"`
	URL          string           `json:"url"`
	Number       int              `json:"number"`
	Name         string           `json:"name"`
	EpisodeOrder int              `json:"episodeOrder"`
	PremiereDate string           `json:"premiereDate"`
	EndDate      string           `json:"endDate"`
	Network      TVmazeNetwork    `json:"network"`
	WebChannel   TVmazeWebChannel `json:"webChannel"`
	Image        TVmazeImage      `json:"image"`
	Summary      string           `json:"summary"`
}

// NewTVmazeClient creates a new TVmaze client for making API requests.
// TVmaze has no API key requirement and allows reasonable rate limiting.
func NewTVmazeClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	general := config.GetSettingsGeneral()

	tvmazeConfig := base.ClientConfig{
		Name:                      "tvmaze",
		BaseURL:                   "https://api.tvmaze.com",
		Timeout:                   time.Duration(general.TvmazeTimeoutSeconds) * time.Second,
		AuthType:                  base.AuthNone,
		RateLimitCalls:            general.TvmazeLimiterCalls,
		RateLimitSeconds:          int(general.TvmazeLimiterSeconds),
		RateLimitPer24h:           20000, // TVMaze has generous limits
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 2,
		EnableStats:               true,
		UserAgent:                 "go-media-downloader/2.0",
		DisableTLSVerify:          general.TvmazeDisableTLSVerify,
	}
	if provider := tvmaze.NewProviderWithConfig(tvmazeConfig); provider != nil {
		// Store in direct providers registry
		providers.SetTVMaze(provider)

		logger.Logtype(logger.StatusDebug, 0).
			Msg("Registered TVMaze metadata provider with rate limiting")
	}
}

// SearchTVmaze searches for TV shows on TVmaze API by name.
// Returns a slice of shows that match the search query.
func SearchTVmaze(name string) ([]apiexternal_v2.SeriesSearchResult, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().SearchSeries(context.Background(), name, 0)
}

// GetTVmazeShowByID gets a TV show from TVmaze API by its TVmaze ID.
func GetTVmazeShowByID(id int) (*apiexternal_v2.SeriesDetails, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().GetSeriesByID(context.Background(), id)
}

// GetTVmazeShowByTVDBID gets a TV show from TVmaze API by TVDB ID.
func GetTVmazeShowByTVDBID(tvdbID int) (*apiexternal_v2.SeriesDetails, error) {
	if tvdbID == 0 {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().FindSeriesByTVDbID(context.Background(), tvdbID)
}

// GetTVmazeShowByIMDBID gets a TV show from TVmaze API by IMDB ID.
func GetTVmazeShowByIMDBID(imdbID string) (*apiexternal_v2.FindByIMDbResult, error) {
	if imdbID == "" {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().FindSeriesByIMDbID(context.Background(), imdbID)
}

// GetTVmazeEpisodes gets all episodes for a TV show from TVmaze API.
func GetTVmazeEpisodes(showID int) ([]*apiexternal_v2.Episode, error) {
	if showID == 0 {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().GetEpisodes(context.Background(), showID)
}

// GetTVmazeSeasons gets all seasons for a TV show from TVmaze API.
func GetTVmazeSeasons(showID int) ([]*apiexternal_v2.Season, error) {
	if showID == 0 {
		return nil, logger.ErrNotFound
	}
	return providers.GetTVMaze().GetSeasons(context.Background(), showID)
}
