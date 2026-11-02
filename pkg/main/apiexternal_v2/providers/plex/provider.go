package plex

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

var errMissingServer = errors.New("plex server URL and token are required")

// Provider reads watchlists from Plex servers. The server is given per call,
// since every list config may point at another one.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a Plex provider.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "plex"

	return &Provider{BaseClient: base.NewBaseClient(cfg)}
}

// WatchlistItem is an entry of a Plex watchlist.
type WatchlistItem struct {
	RatingKey             string `json:"ratingKey"`
	Key                   string `json:"key"`
	GUID                  string `json:"guid"`
	Type                  string `json:"type"`
	Title                 string `json:"title"`
	OriginalTitle         string `json:"originalTitle,omitempty"`
	Year                  int    `json:"year"`
	Summary               string `json:"summary,omitempty"`
	Rating                string `json:"rating,omitempty"`
	UserRating            string `json:"userRating,omitempty"`
	Duration              int    `json:"duration,omitempty"`
	AddedAt               int64  `json:"addedAt"`
	UpdatedAt             int64  `json:"updatedAt"`
	Thumb                 string `json:"thumb,omitempty"`
	Art                   string `json:"art,omitempty"`
	ContentRating         string `json:"contentRating,omitempty"`
	OriginallyAvailableAt string `json:"originallyAvailableAt,omitempty"`
	GUIDS                 []GUID `json:"Guid,omitempty"`
}

// GUID is an external ID of a Plex item ("imdb://tt…", "tvdb://123").
type GUID struct {
	ID string `json:"id"`
}

type watchlistResponse struct {
	MediaContainer struct {
		Metadata []WatchlistItem `json:"Metadata"`
	} `json:"MediaContainer"`
}

// GetWatchlist retrieves the watchlist of the account owning the token.
// The endpoint has no per-user filter, so no username is taken.
func (p *Provider) GetWatchlist(ctx context.Context, serverURL, token string) ([]WatchlistItem, error) {
	if serverURL == "" || token == "" {
		return nil, errMissingServer
	}

	params := url.Values{}
	params.Set("X-Plex-Token", token)
	params.Set("type", "15") // watchlist
	params.Set("includeGuids", "1")

	var response watchlistResponse

	err := p.MakeRequest(
		ctx,
		http.MethodGet,
		strings.TrimSuffix(serverURL, "/")+"/playlists/all?"+params.Encode(),
		nil,
		&response,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return response.MediaContainer.Metadata, nil
}

// guidValue returns the values of the GUIDs with the given scheme, the
// item's main GUID last.
func (item *WatchlistItem) guidValue(scheme string) []string {
	var out []string

	for _, guid := range item.GUIDS {
		if after, ok := strings.CutPrefix(guid.ID, scheme); ok {
			out = append(out, after)
		}
	}

	if after, ok := strings.CutPrefix(item.GUID, scheme); ok {
		out = append(out, after)
	}

	return out
}

// IMDbID returns the item's IMDB ID or "".
func (item *WatchlistItem) IMDbID() string {
	if ids := item.guidValue("imdb://"); len(ids) > 0 {
		return ids[0]
	}

	return ""
}

// TVDbID returns the item's TVDB ID or 0.
func (item *WatchlistItem) TVDbID() int {
	for _, id := range item.guidValue("tvdb://") {
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}

	return 0
}

// IsMovie reports whether the item is a movie.
func (item *WatchlistItem) IsMovie() bool { return item.Type == "movie" }

// IsShow reports whether the item is a TV show.
func (item *WatchlistItem) IsShow() bool { return item.Type == "show" }
