package lastfm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Last.fm Provider - Music metadata and charts
// API: https://www.last.fm/api
// Rate limit: ~5 requests/second per API key
//

const (
	defaultBaseURL = "http://ws.audioscrobbler.com/2.0/"
)

// Provider implements the Last.fm music metadata and chart provider.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new Last.fm provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "lastfm"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 5
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 1
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	// Last.fm uses api_key as a URL parameter
	cfg.AuthType = base.AuthAPIKeyURL
	cfg.APIKeyParam = "api_key"

	return &Provider{
		BaseClient: base.NewBaseClient(cfg),
	}
}

// NewProvider creates a new Last.fm provider using API key and rate limits from
// the application config.
func NewProvider() *Provider {
	cfg := config.GetSettingsGeneral()
	lfmKey := cfg.LastFMAPIKey

	rateSec := cfg.LastFMLimiterSeconds
	rateCalls := cfg.LastFMLimiterCalls

	if rateSec == 0 {
		rateSec = 1
	}

	if rateCalls == 0 {
		rateCalls = 5
	}

	return NewProviderWithConfig(base.ClientConfig{
		APIKey:           lfmKey,
		RateLimitSeconds: int(rateSec),
		RateLimitCalls:   rateCalls,
	})
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderLastFM
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "lastfm"
}

// endpoint builds a query string for the Last.fm API.
// All calls share the same base URL; method, format, and api_key are always
// present — api_key is appended automatically by the base client's AuthAPIKeyURL
// logic, so we only add method and format here plus any extra params.
// Extra params are key/value pairs: endpoint("method.name", "key1", "val1", "key2", "val2").
// Values are URL-encoded so strings with spaces (e.g. country names, artist names) are safe.
func endpoint(method string, extra ...string) string {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("?method=")
	buf.WriteString(method)
	buf.WriteString("&format=json")

	for i := 0; i+1 < len(extra); i += 2 {
		buf.WriteByte('&')
		buf.WriteString(extra[i])
		buf.WriteByte('=')
		buf.WriteURL(extra[i+1]) // URL-encode the value
	}

	ep := buf.String()
	logger.PlAddBuffer.Put(buf)

	return ep
}

//
// Chart Methods
//

// GetTopArtists returns the global top artists chart.
// page is 1-based; limit is capped at 1000 by Last.fm (default 50).
func (p *Provider) GetTopArtists(ctx context.Context, page, limit int) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("chart.gettopartists",
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	var resp lfmChartTopArtistsResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertChartArtists(resp.Artists.Artist), nil
}

// GetTopTracks returns the global top tracks chart.
func (p *Provider) GetTopTracks(ctx context.Context, page, limit int) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("chart.gettoptracks",
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	var resp lfmChartTopTracksResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertChartTracks(resp.Tracks.Track), nil
}

// GetTopArtistsByCountry returns the top artists for a given country.
// country should be an ISO 3166-1 country name (e.g. "germany", "united states").
func (p *Provider) GetTopArtistsByCountry(
	ctx context.Context,
	country string,
	page, limit int,
) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("geo.gettopartists",
		"country", country,
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	var resp lfmGeoTopArtistsResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertChartArtists(resp.TopArtists.Artist), nil
}

// GetTopTracksByCountry returns the top tracks for a given country.
func (p *Provider) GetTopTracksByCountry(
	ctx context.Context,
	country string,
	page, limit int,
) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("geo.gettoptracks",
		"country", country,
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	var resp lfmGeoTopTracksResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertChartTracks(resp.Tracks.Track), nil
}

// GetTopAlbumsByTag returns the top albums for a given tag/genre.
// tag examples: "rock", "electronic", "hip-hop".
func (p *Provider) GetTopAlbumsByTag(
	ctx context.Context,
	tag string,
	page, limit int,
) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("tag.gettopalbums",
		"tag", tag,
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	var resp lfmTagTopAlbumsResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertTagAlbums(resp.Albums.Album), nil
}

// GetTopArtistsByTag returns the top artists for a given tag/genre.
func (p *Provider) GetTopArtistsByTag(
	ctx context.Context,
	tag string,
	page, limit int,
) ([]ChartEntry, error) {
	if limit <= 0 {
		limit = 100
	}

	if page <= 0 {
		page = 1
	}

	ep := endpoint("tag.gettopartists",
		"tag", tag,
		"page", fmt.Sprint(page),
		"limit", fmt.Sprint(limit),
	)

	// tag.gettopartists returns the same shape as chart.gettopartists
	var resp lfmChartTopArtistsResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertChartArtists(resp.Artists.Artist), nil
}

//
// Metadata Methods
//

// GetArtistInfo retrieves full artist metadata.
// Provide either artistName or mbid (or both). mbid takes priority when present.
func (p *Provider) GetArtistInfo(
	ctx context.Context,
	artistName, mbid string,
) (*apiexternal_v2.ArtistDetails, error) {
	if artistName == "" && mbid == "" {
		return nil, errors.New("lastfm: GetArtistInfo requires artist name or mbid")
	}

	var extra []string
	if mbid != "" {
		extra = append(extra, "mbid", mbid)
	} else {
		extra = append(extra, "artist", artistName)
	}

	ep := endpoint("artist.getinfo", extra...)

	var resp lfmArtistInfoResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertArtistInfoToDetails(&resp.Artist), nil
}

// GetAlbumInfo retrieves full album metadata.
// Provide either (artistName + albumName) or mbid. mbid takes priority when present.
func (p *Provider) GetAlbumInfo(
	ctx context.Context,
	artistName, albumName, mbid string,
) (*apiexternal_v2.ReleaseDetails, error) {
	if mbid == "" && (artistName == "" || albumName == "") {
		return nil, errors.New("lastfm: GetAlbumInfo requires (artist+album) or mbid")
	}

	var extra []string
	if mbid != "" {
		extra = append(extra, "mbid", mbid)
	} else {
		extra = append(extra, "artist", artistName, "album", albumName)
	}

	ep := endpoint("album.getinfo", extra...)

	var resp lfmAlbumInfoResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertAlbumInfoToRelease(&resp.Album), nil
}

//
// Search Methods
//

// SearchArtists searches for artists by name.
func (p *Provider) SearchArtists(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	if limit <= 0 {
		limit = 30
	}

	ep := endpoint("artist.search",
		"artist", query,
		"limit", fmt.Sprint(limit),
	)

	var resp lfmArtistSearchResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertArtistSearchResults(resp.Results.ArtistMatches.Artist), nil
}

// SearchAlbums searches for albums by title.
func (p *Provider) SearchAlbums(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 30
	}

	ep := endpoint("album.search",
		"album", query,
		"limit", fmt.Sprint(limit),
	)

	var resp lfmAlbumSearchResponse
	if err := p.MakeRequest(ctx, "GET", ep, nil, &resp, nil); err != nil {
		return nil, err
	}

	return convertAlbumSearchResults(resp.Results.AlbumMatches.Album), nil
}
