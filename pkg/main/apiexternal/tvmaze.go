package apiexternal

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
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

type tvmazeClient struct {
	DefaultHeaders map[string][]string
	Lim            *slidingwindow.Limiter
	Client         rlHTTPClient
}

// NewTVmazeClient creates a new TVmaze client for making API requests.
// TVmaze has no API key requirement and allows reasonable rate limiting.
func NewTVmazeClient(seconds uint8, calls int, disabletls bool, timeoutseconds uint16) {
	if seconds == 0 {
		seconds = 1
	}
	if calls == 0 {
		calls = 1
	}
	lim := slidingwindow.NewLimiter(time.Duration(seconds)*time.Second, int64(calls))
	client := &tvmazeClient{
		DefaultHeaders: map[string][]string{
			"User-Agent": {"go-media-downloader/1.0"},
		},
		Lim: &lim,
		Client: newClient(
			"tvmaze",
			disabletls,
			true,
			&lim,
			false, nil, timeoutseconds),
	}
	setTvmazeAPI(client)
}

// Helper functions for thread-safe access to tvmazeAPI
func getTvmazeClient() *rlHTTPClient {
	api := getTvmazeAPI()
	if api == nil {
		return nil
	}
	return &api.Client
}

func getTvmazeHeaders() map[string][]string {
	api := getTvmazeAPI()
	if api == nil {
		return nil
	}
	return api.DefaultHeaders
}

// Helper function to check rate limit with timeout for TVmaze
func checkTvmazeRateLimit(ctx context.Context) (bool, error) {
	api := getTvmazeAPI()
	if api == nil {
		return false, logger.ErrNotFound
	}
	return api.Client.checkLimiter(ctx, true)
}

// Helper function to get timeout context for TVmaze
func getTvmazeTimeoutContext() (context.Context, context.CancelFunc) {
	api := getTvmazeAPI()
	if api == nil {
		return context.Background(), func() {}
	}
	return context.WithTimeout(api.Client.Ctx, api.Client.Timeout5)
}

// SearchTVmaze searches for TV shows on TVmaze API by name.
// Returns a slice of shows that match the search query.
func SearchTVmaze(name string) (TVmazeSearchResults, error) {
	if name == "" {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONType[TVmazeSearchResults](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/search/shows?q=",
			url.QueryEscape(name),
		),
		getTvmazeHeaders(),
	)
}

// GetTVmazeShowByID gets a TV show from TVmaze API by its TVmaze ID.
func GetTVmazeShowByID(id int) (*TVmazeShow, error) {
	if id == 0 {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONTypeP[TVmazeShow](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/shows/",
			strconv.Itoa(id),
		),
		getTvmazeHeaders(),
	)
}

// GetTVmazeShowByTVDBID gets a TV show from TVmaze API by TVDB ID.
func GetTVmazeShowByTVDBID(tvdbID int) (*TVmazeShow, error) {
	if tvdbID == 0 {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONTypeP[TVmazeShow](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/lookup/shows?thetvdb=",
			strconv.Itoa(tvdbID),
		),
		getTvmazeHeaders(),
	)
}

// GetTVmazeShowByIMDBID gets a TV show from TVmaze API by IMDB ID.
func GetTVmazeShowByIMDBID(imdbID string) (*TVmazeShow, error) {
	if imdbID == "" {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONTypeP[TVmazeShow](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/lookup/shows?imdb=",
			url.QueryEscape(imdbID),
		),
		getTvmazeHeaders(),
	)
}

// GetTVmazeEpisodes gets all episodes for a TV show from TVmaze API.
func GetTVmazeEpisodes(showID int) ([]TVmazeEpisode, error) {
	if showID == 0 {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONType[[]TVmazeEpisode](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/shows/",
			strconv.Itoa(showID),
			"/episodes",
		),
		getTvmazeHeaders(),
	)
}

// GetTVmazeSeasons gets all seasons for a TV show from TVmaze API.
func GetTVmazeSeasons(showID int) (TVmazeSeasons, error) {
	if showID == 0 {
		return nil, logger.ErrNotFound
	}

	ctx, ctxcancel := getTvmazeTimeoutContext()
	defer ctxcancel()
	ok, err := checkTvmazeRateLimit(ctx)
	if !ok {
		if err == nil {
			return nil, logger.ErrToWait
		}
		return nil, err
	}

	return doJSONType[TVmazeSeasons](
		getTvmazeClient(),
		logger.JoinStrings(
			"https://api.tvmaze.com/shows/",
			strconv.Itoa(showID),
			"/seasons",
		),
		getTvmazeHeaders(),
	)
}
