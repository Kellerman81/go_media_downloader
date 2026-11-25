package plex

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// LibraryItem is a movie, show or episode of a Plex library.
type LibraryItem struct {
	WatchlistItem
	Tagline     string `json:"tagline,omitempty"`
	Studio      string `json:"studio,omitempty"`
	ParentIndex int    `json:"parentIndex,omitempty"` // season of an episode
	Index       int    `json:"index,omitempty"`       // episode number
	Genre       []Tag  `json:"Genre,omitempty"`
	// The token's account's state: plays, where it stopped (ms).
	ViewCount            int    `json:"viewCount,omitempty"`
	ViewOffset           int64  `json:"viewOffset,omitempty"`
	GrandparentRatingKey string `json:"grandparentRatingKey,omitempty"` // show of an episode
}

// Tag is a Plex tag such as a genre.
type Tag struct {
	Tag string `json:"tag"`
}

// TMDbID returns the item's TMDb ID or 0.
func (item *LibraryItem) TMDbID() int {
	for _, id := range item.guidValue("tmdb://") {
		if n, err := strconv.Atoi(id); err == nil {
			return n
		}
	}

	return 0
}

// Genres returns the item's genres comma-separated.
func (item *LibraryItem) Genres() string {
	names := make([]string, 0, len(item.Genre))
	for _, g := range item.Genre {
		names = append(names, g.Tag)
	}

	return strings.Join(names, ", ")
}

type libraryResponse struct {
	MediaContainer struct {
		MachineIdentifier string `json:"machineIdentifier"`
		Directory         []struct {
			Key  string `json:"key"`
			Type string `json:"type"`
		} `json:"Directory"`
		Metadata []LibraryItem `json:"Metadata"`
	} `json:"MediaContainer"`
}

func (p *Provider) get(ctx context.Context, serverURL, token, path string, params url.Values) (*libraryResponse, error) {
	if serverURL == "" || token == "" {
		return nil, errMissingServer
	}

	if params == nil {
		params = url.Values{}
	}

	params.Set("X-Plex-Token", token)

	var response libraryResponse

	err := p.MakeRequest(ctx, http.MethodGet, strings.TrimSuffix(serverURL, "/")+path+"?"+params.Encode(), nil, &response, nil)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// LibraryItems returns all items of the server's libraries of a type
// ("movie" or "show"), with their external IDs.
func (p *Provider) LibraryItems(ctx context.Context, serverURL, token, libraryType string) ([]LibraryItem, error) {
	sections, err := p.get(ctx, serverURL, token, "/library/sections", nil)
	if err != nil {
		return nil, err
	}

	var items []LibraryItem

	for _, dir := range sections.MediaContainer.Directory {
		if dir.Type != libraryType {
			continue
		}

		all, err := p.get(ctx, serverURL, token, "/library/sections/"+url.PathEscape(dir.Key)+"/all",
			url.Values{"includeGuids": {"1"}})
		if err != nil {
			return nil, err
		}

		items = append(items, all.MediaContainer.Metadata...)
	}

	return items, nil
}

// ShowEpisodes returns all episodes of a show (by its ratingKey).
func (p *Provider) ShowEpisodes(ctx context.Context, serverURL, token, ratingKey string) ([]LibraryItem, error) {
	resp, err := p.get(ctx, serverURL, token, "/library/metadata/"+url.PathEscape(ratingKey)+"/allLeaves", nil)
	if err != nil {
		return nil, err
	}

	return resp.MediaContainer.Metadata, nil
}

// MachineID returns the server's machine identifier, which Plex Web links
// (#!/server/<id>/details?key=...) need.
func (p *Provider) MachineID(ctx context.Context, serverURL, token string) (string, error) {
	response, err := p.get(ctx, serverURL, token, "/identity", nil)
	if err != nil {
		return "", err
	}

	return response.MediaContainer.MachineIdentifier, nil
}

// AllEpisodes returns the episodes of all show libraries, with their show
// (grandparentRatingKey) and the token's account's state.
func (p *Provider) AllEpisodes(ctx context.Context, serverURL, token string) ([]LibraryItem, error) {
	sections, err := p.get(ctx, serverURL, token, "/library/sections", nil)
	if err != nil {
		return nil, err
	}

	var items []LibraryItem

	for _, dir := range sections.MediaContainer.Directory {
		if dir.Type != "show" {
			continue
		}

		all, err := p.get(ctx, serverURL, token, "/library/sections/"+url.PathEscape(dir.Key)+"/all", url.Values{"type": {"4"}})
		if err != nil {
			return nil, err
		}

		items = append(items, all.MediaContainer.Metadata...)
	}

	return items, nil
}

// Scrobble marks an item (ratingKey) watched for the token's account.
func (p *Provider) Scrobble(ctx context.Context, serverURL, token, ratingKey string) error {
	if serverURL == "" || token == "" {
		return errMissingServer
	}

	params := url.Values{"key": {ratingKey}, "identifier": {"com.plexapp.plugins.library"}, "X-Plex-Token": {token}}

	return p.MakeRequest(ctx, http.MethodGet, strings.TrimSuffix(serverURL, "/")+"/:/scrobble?"+params.Encode(), nil, nil, nil)
}
