package musicbrainz

import (
	"context"
	"errors"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// MusicBrainz Provider - Music metadata database
// API: https://musicbrainz.org/doc/MusicBrainz_API
// Rate limit: 1 request per second (with proper User-Agent)
//

const (
	defaultBaseURL = "https://musicbrainz.org/ws/2"
)

// Provider implements the music metadata provider for MusicBrainz.
type Provider struct {
	*base.BaseClient
}

// NewProviderWithConfig creates a new MusicBrainz provider with custom config.
func NewProviderWithConfig(cfg base.ClientConfig) *Provider {
	cfg.Name = "musicbrainz"
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// MusicBrainz requires 1 request per second rate limit
	if cfg.RateLimitCalls == 0 {
		cfg.RateLimitCalls = 3
	}

	if cfg.RateLimitSeconds == 0 {
		cfg.RateLimitSeconds = 4
	}

	// MusicBrainz requires a descriptive User-Agent
	if cfg.UserAgent == "" {
		cfg.UserAgent = config.GetSettingsGeneral().UserAgent
	}

	return &Provider{
		BaseClient: base.NewBaseClient(cfg),
	}
}

// NewProvider creates a new MusicBrainz provider with default configuration.
func NewProvider() *Provider {
	return NewProviderWithConfig(base.ClientConfig{})
}

// GetProviderType returns the provider type.
func (*Provider) GetProviderType() apiexternal_v2.ProviderType {
	return apiexternal_v2.ProviderMusicBrainz
}

// GetProviderName returns the provider name.
func (*Provider) GetProviderName() string {
	return "musicbrainz"
}

//
// Artist Methods
//

// SearchArtists searches for artists by name.
func (p *Provider) SearchArtists(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	if limit <= 0 {
		limit = 25
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/artist?query=")
	buf.WriteURL(query)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)
	buf.WriteString("&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbArtistSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertArtistSearchResults(response.Artists), nil
}

// GetArtistByID retrieves artist details by MusicBrainz ID.
func (p *Provider) GetArtistByID(
	ctx context.Context,
	mbid string,
) (*apiexternal_v2.ArtistDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/artist/")
	buf.WriteString(mbid)
	buf.WriteString("?inc=aliases%2Btags%2Bratings%2Burl-rels&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbArtistResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertArtistToDetails(&response), nil
}

// GetArtistReleases retrieves releases for an artist.
func (p *Provider) GetArtistReleases(
	ctx context.Context,
	mbid string,
	limit int,
	offset int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 25
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release?artist=")
	buf.WriteString(mbid)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)
	buf.WriteString("&offset=")
	buf.WriteInt(offset)
	buf.WriteString("&inc=labels%2Bmedia&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertReleaseSearchResults(response.Releases), nil
}

//
// Release Methods
//

// SearchReleases searches for releases (albums).
func (p *Provider) SearchReleases(
	ctx context.Context,
	query string,
	limit int,
	offset int,
) ([]apiexternal_v2.ReleaseSearchResult, int, error) {
	if limit <= 0 {
		limit = 25
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release?query=")
	buf.WriteURL(query)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)
	buf.WriteString("&offset=")
	buf.WriteInt(offset)
	buf.WriteString("&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, 0, err
	}

	return convertReleaseSearchResults(response.Releases), response.Count, nil
}

// GetReleaseByID retrieves release details by MusicBrainz ID.
func (p *Provider) GetReleaseByID(
	ctx context.Context,
	mbid string,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release/")
	buf.WriteString(mbid)
	buf.WriteString(
		"?inc=artists%2Blabels%2Brecordings%2Brelease-groups%2Bmedia%2Btags%2Bratings&fmt=json",
	)

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertReleaseToDetails(&response), nil
}

// GetReleaseByBarcode retrieves a release by its barcode (UPC/EAN).
func (p *Provider) GetReleaseByBarcode(
	ctx context.Context,
	barcode string,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release?query=barcode%3A")
	buf.WriteString(barcode)
	buf.WriteString("&limit=1&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if len(response.Releases) == 0 {
		return nil, errors.New(logger.JoinStrings("no release found with barcode: ", barcode))
	}

	// Get full details
	return p.GetReleaseByID(ctx, response.Releases[0].ID)
}

//
// Recording Methods
//

// SearchRecordings searches for recordings (tracks).
func (p *Provider) SearchRecordings(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.Track, error) {
	if limit <= 0 {
		limit = 25
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/recording?query=")
	buf.WriteURL(query)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)
	buf.WriteString("&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbRecordingSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertRecordingSearchResults(response.Recordings), nil
}

// GetRecordingByID retrieves recording details by MusicBrainz ID.
func (p *Provider) GetRecordingByID(
	ctx context.Context,
	mbid string,
) (*apiexternal_v2.Track, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/recording/")
	buf.WriteString(mbid)
	buf.WriteString("?inc=artists%2Breleases%2Btags%2Bratings%2Bisrcs&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbRecordingResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertRecordingToTrack(&response), nil
}

// GetRecordingByISRC retrieves a recording by its ISRC.
func (p *Provider) GetRecordingByISRC(
	ctx context.Context,
	isrc string,
) (*apiexternal_v2.Track, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/isrc/")
	buf.WriteString(isrc)
	buf.WriteString("?inc=artists%2Breleases%2Btags&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbISRCResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if len(response.Recordings) == 0 {
		return nil, errors.New(logger.JoinStrings("no recording found with ISRC: ", isrc))
	}

	return convertRecordingToTrack(&response.Recordings[0]), nil
}

// GetReleaseIDsByDiscID returns all MusicBrainz release IDs associated with a disc ID.
// The disc ID can be the pre-computed MusicBrainz DiscID (SHA1-based) or one embedded
// in file tags by ripping software (MUSICBRAINZ_DISCID).
func (p *Provider) GetReleaseIDsByDiscID(
	ctx context.Context,
	discID string,
) ([]string, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/discid/")
	buf.WriteString(discID)
	buf.WriteString("?inc=releases&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response struct {
		Releases []struct {
			ID string `json:"id"`
		} `json:"releases"`
	}

	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(response.Releases))
	for i := range response.Releases {
		if response.Releases[i].ID != "" {
			ids = append(ids, response.Releases[i].ID)
		}
	}

	return ids, nil
}

// GetReleaseIDsByISRC returns all MusicBrainz release IDs associated with an ISRC.
// An ISRC identifies a recording; each recording may appear on multiple releases.
// The returned slice contains deduplicated release IDs across all matched recordings.
func (p *Provider) GetReleaseIDsByISRC(
	ctx context.Context,
	isrc string,
) ([]string, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/isrc/")
	buf.WriteString(isrc)
	buf.WriteString("?inc=releases&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbISRCResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	if len(response.Recordings) == 0 {
		return nil, errors.New(logger.JoinStrings("no recording found with ISRC: ", isrc))
	}

	seen := make(map[string]struct{})
	ids := make([]string, 0)

	for i := range response.Recordings {
		for j := range response.Recordings[i].Releases {
			id := response.Recordings[i].Releases[j].ID
			if id != "" {
				if _, ok := seen[id]; !ok {
					seen[id] = struct{}{}
					ids = append(ids, id)
				}
			}
		}
	}

	return ids, nil
}

//
// Release Group Methods
//

// SearchReleaseGroups searches for release groups (albums across editions).
func (p *Provider) SearchReleaseGroups(
	ctx context.Context,
	query string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	if limit <= 0 {
		limit = 25
	}

	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release-group?query=")
	buf.WriteURL(query)
	buf.WriteString("&limit=")
	buf.WriteInt(limit)
	buf.WriteString("&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseGroupSearchResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertReleaseGroupSearchResults(response.ReleaseGroups), nil
}

// GetReleaseGroupByID retrieves release group details by MusicBrainz ID.
func (p *Provider) GetReleaseGroupByID(
	ctx context.Context,
	mbid string,
) (*apiexternal_v2.ReleaseDetails, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/release-group/")
	buf.WriteString(mbid)
	buf.WriteString("?inc=artists%2Breleases%2Btags%2Bratings&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbReleaseGroupResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return convertReleaseGroupToDetails(&response), nil
}

//
// Label Methods
//

// GetLabelByID retrieves label details by MusicBrainz ID.
func (p *Provider) GetLabelByID(ctx context.Context, mbid string) (*mbLabelResponse, error) {
	buf := logger.PlAddBuffer.Get()
	buf.WriteString("/label/")
	buf.WriteString(mbid)
	buf.WriteString("?inc=tags%2Bratings&fmt=json")

	endpoint := buf.String()
	logger.PlAddBuffer.Put(buf)

	var response mbLabelResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return &response, nil
}
