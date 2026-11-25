package jellyfin

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var errMissingLibraryServer = errors.New("jellyfin server URL and API key are required")

// LibraryItem is a movie, series or episode of a Jellyfin library.
type LibraryItem struct {
	WatchlistItem
	Taglines          []string `json:"Taglines,omitempty"`
	PremiereDate      string   `json:"PremiereDate,omitempty"`
	IndexNumber       int      `json:"IndexNumber,omitempty"`       // episode number
	ParentIndexNumber int      `json:"ParentIndexNumber,omitempty"` // season of an episode
	Genres            []string `json:"Genres,omitempty"`
	SeriesID          string   `json:"SeriesId,omitempty"` // series of an episode
}

// RuntimeMinutes converts the item's runtime ticks to minutes.
func (item *LibraryItem) RuntimeMinutes() int {
	return int(item.RunTimeTicks / 600_000_000)
}

type libraryResponse struct {
	Items            []LibraryItem `json:"Items"`
	TotalRecordCount int           `json:"TotalRecordCount"`
}

const libraryPageSize = 500

func (p *Provider) items(ctx context.Context, serverURL, apiKey, path string, params url.Values) ([]LibraryItem, error) {
	if serverURL == "" || apiKey == "" {
		return nil, errMissingLibraryServer
	}

	params.Set("api_key", apiKey)
	params.Set("Limit", "500")

	var all []LibraryItem

	for start := 0; ; start += libraryPageSize {
		params.Set("StartIndex", strconv.Itoa(start))

		var response libraryResponse

		err := p.MakeRequest(ctx, http.MethodGet, strings.TrimSuffix(serverURL, "/")+path+"?"+params.Encode(), nil, &response, nil)
		if err != nil {
			return nil, err
		}

		all = append(all, response.Items...)
		if len(response.Items) < libraryPageSize || len(all) >= response.TotalRecordCount {
			return all, nil
		}
	}
}

// LibraryItems returns all items of a type ("Movie" or "Series") with their
// provider IDs.
func (p *Provider) LibraryItems(ctx context.Context, serverURL, apiKey, itemType string) ([]LibraryItem, error) {
	return p.items(ctx, serverURL, apiKey, "/Items", url.Values{
		"Recursive":        {"true"},
		"IncludeItemTypes": {itemType},
		"Fields":           {"ProviderIds,Overview,Genres,Taglines,PremiereDate"},
	})
}

// SeriesEpisodes returns all episodes of a series (by its Jellyfin ID).
func (p *Provider) SeriesEpisodes(ctx context.Context, serverURL, apiKey, seriesID string) ([]LibraryItem, error) {
	return p.items(ctx, serverURL, apiKey, "/Shows/"+url.PathEscape(seriesID)+"/Episodes", url.Values{
		"Fields": {"Overview,PremiereDate"},
	})
}

// ServerID returns the server's ID, which Jellyfin Web links
// (#/details?id=...&serverId=...) need.
func (p *Provider) ServerID(ctx context.Context, serverURL, apiKey string) (string, error) {
	if serverURL == "" || apiKey == "" {
		return "", errMissingLibraryServer
	}

	var info struct {
		ID string `json:"Id"`
	}

	err := p.MakeRequest(ctx, http.MethodGet, strings.TrimSuffix(serverURL, "/")+"/System/Info/Public?api_key="+url.QueryEscape(apiKey), nil, &info, nil)
	if err != nil {
		return "", err
	}

	return info.ID, nil
}

// UserItems returns all items of a type ("Movie", "Series", "Episode")
// with the user's state (UserData) and provider IDs.
func (p *Provider) UserItems(ctx context.Context, serverURL, apiKey, username, itemType string) ([]LibraryItem, error) {
	uid, err := p.userID(ctx, strings.TrimSuffix(serverURL, "/"), apiKey, username)
	if err != nil {
		return nil, err
	}

	return p.items(ctx, serverURL, apiKey, "/Users/"+url.PathEscape(uid)+"/Items", url.Values{
		"Recursive":        {"true"},
		"IncludeItemTypes": {itemType},
		"Fields":           {"ProviderIds"},
	})
}

// MarkPlayed marks an item played for a user.
func (p *Provider) MarkPlayed(ctx context.Context, serverURL, apiKey, username, itemID string) error {
	serverURL = strings.TrimSuffix(serverURL, "/")

	uid, err := p.userID(ctx, serverURL, apiKey, username)
	if err != nil {
		return err
	}

	return p.MakeRequest(ctx, http.MethodPost, serverURL+"/Users/"+url.PathEscape(uid)+"/PlayedItems/"+url.PathEscape(itemID)+
		"?api_key="+url.QueryEscape(apiKey), nil, nil, nil)
}
