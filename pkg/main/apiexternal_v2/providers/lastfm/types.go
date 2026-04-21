package lastfm

import (
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// Last.fm Internal Types - Used for JSON unmarshaling
//

// lfmImage represents a Last.fm image at a given size.
type lfmImage struct {
	URL  string `json:"#text"`
	Size string `json:"size"`
}

// lfmAttr holds pagination metadata returned in @attr fields.
type lfmAttr struct {
	Page       string `json:"page"`
	PerPage    string `json:"perPage"`
	Total      string `json:"total"`
	TotalPages string `json:"totalPages"`
	Country    string `json:"country,omitempty"`
	Tag        string `json:"tag,omitempty"`
}

// lfmTag represents a Last.fm tag (genre).
type lfmTag struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// lfmTagList is the container for a list of tags.
// Last.fm sends "" (empty string) instead of {} when there are no tags,
// so we need a custom unmarshaler to handle both forms gracefully.
type lfmTagList struct {
	Tag []lfmTag `json:"tag"`
}

func (t *lfmTagList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || data[0] != '{' {
		// Empty string, null, or any non-object — treat as no tags.
		return nil
	}
	type lfmTagListAlias lfmTagList
	return json.Unmarshal(data, (*lfmTagListAlias)(t))
}

// lfmDuration accepts Last.fm's duration field which may arrive as a bare
// number (e.g. 327) or a quoted string (e.g. "327").
type lfmDuration int

func (d *lfmDuration) UnmarshalJSON(data []byte) error {
	// Strip surrounding quotes if present, then parse as int.
	s := string(data)
	if len(s) >= 2 && s[0] == '"' {
		s = s[1 : len(s)-1]
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		*d = 0
		return nil
	}
	*d = lfmDuration(n)
	return nil
}

// lfmBio holds artist biography text.
type lfmBio struct {
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// lfmWiki holds album wiki text.
type lfmWiki struct {
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// -------- chart.getTopArtists --------

type lfmChartArtist struct {
	Name       string     `json:"name"`
	Playcount  string     `json:"playcount"`
	Listeners  string     `json:"listeners"`
	MBID       string     `json:"mbid"`
	URL        string     `json:"url"`
	Streamable string     `json:"streamable"`
	Image      []lfmImage `json:"image"`
}

type lfmChartArtistsWrapper struct {
	Artist []lfmChartArtist `json:"artist"`
	Attr   lfmAttr          `json:"@attr"`
}

type lfmChartTopArtistsResponse struct {
	Artists lfmChartArtistsWrapper `json:"artists"`
}

// -------- chart.getTopTracks --------

type lfmChartTrack struct {
	Name       string     `json:"name"`
	Duration   string     `json:"duration"`
	Playcount  string     `json:"playcount"`
	Listeners  string     `json:"listeners"`
	MBID       string     `json:"mbid"`
	URL        string     `json:"url"`
	Streamable struct {
		Text      string `json:"#text"`
		Fulltrack string `json:"fulltrack"`
	} `json:"streamable"`
	Artist lfmChartArtist `json:"artist"`
	Image  []lfmImage     `json:"image"`
}

type lfmChartTracksWrapper struct {
	Track []lfmChartTrack `json:"track"`
	Attr  lfmAttr         `json:"@attr"`
}

type lfmChartTopTracksResponse struct {
	Tracks lfmChartTracksWrapper `json:"tracks"`
}

// -------- geo.getTopArtists --------

type lfmGeoArtistsWrapper struct {
	Artist []lfmChartArtist `json:"artist"`
	Attr   lfmAttr          `json:"@attr"`
}

type lfmGeoTopArtistsResponse struct {
	TopArtists lfmGeoArtistsWrapper `json:"topartists"`
}

// -------- geo.getTopTracks --------

type lfmGeoTracksWrapper struct {
	Track []lfmChartTrack `json:"track"`
	Attr  lfmAttr         `json:"@attr"`
}

type lfmGeoTopTracksResponse struct {
	Tracks lfmGeoTracksWrapper `json:"tracks"`
}

// -------- tag.getTopAlbums --------

type lfmTagAlbum struct {
	Name   string     `json:"name"`
	MBID   string     `json:"mbid"`
	URL    string     `json:"url"`
	Image  []lfmImage `json:"image"`
	Artist struct {
		Name string `json:"name"`
		MBID string `json:"mbid"`
		URL  string `json:"url"`
	} `json:"artist"`
}

type lfmTagAlbumsWrapper struct {
	Album []lfmTagAlbum `json:"album"`
	Attr  lfmAttr       `json:"@attr"`
}

type lfmTagTopAlbumsResponse struct {
	Albums lfmTagAlbumsWrapper `json:"albums"`
}

// -------- artist.getInfo --------

type lfmArtistInfo struct {
	Name       string     `json:"name"`
	MBID       string     `json:"mbid"`
	URL        string     `json:"url"`
	Image      []lfmImage `json:"image"`
	Streamable string     `json:"streamable"`
	OnTour     string     `json:"ontour"`
	Stats      struct {
		Listeners string `json:"listeners"`
		Playcount string `json:"playcount"`
	} `json:"stats"`
	Similar struct {
		Artist []struct {
			Name  string     `json:"name"`
			URL   string     `json:"url"`
			Image []lfmImage `json:"image"`
		} `json:"artist"`
	} `json:"similar"`
	Tags lfmTagList `json:"tags"`
	Bio  lfmBio     `json:"bio"`
}

type lfmArtistInfoResponse struct {
	Artist lfmArtistInfo `json:"artist"`
}

// -------- album.getInfo --------

type lfmAlbumTrack struct {
	Name       string      `json:"name"`
	URL        string      `json:"url"`
	Duration   lfmDuration `json:"duration"` // bare number or quoted string of seconds
	Streamable struct {
		Text      string `json:"#text"`
		Fulltrack string `json:"fulltrack"`
	} `json:"streamable"`
	Attr struct {
		Rank lfmDuration `json:"rank"` // bare number or quoted string
	} `json:"@attr"`
	Artist struct {
		Name string `json:"name"`
		MBID string `json:"mbid"`
		URL  string `json:"url"`
	} `json:"artist"`
}

type lfmAlbumInfo struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	MBID   string `json:"mbid"`
	URL    string `json:"url"`
	Image  []lfmImage
	Tracks struct {
		Track []lfmAlbumTrack `json:"track"`
	} `json:"tracks"`
	Tags      lfmTagList `json:"tags"`
	Wiki      lfmWiki    `json:"wiki"`
	Listeners string     `json:"listeners"`
	Playcount string     `json:"playcount"`
}

type lfmAlbumInfoResponse struct {
	Album lfmAlbumInfo `json:"album"`
}

// -------- artist.search --------

type lfmArtistSearchResult struct {
	Name       string     `json:"name"`
	Listeners  string     `json:"listeners"`
	MBID       string     `json:"mbid"`
	URL        string     `json:"url"`
	Image      []lfmImage `json:"image"`
	Streamable string     `json:"streamable"`
}

type lfmArtistSearchResponse struct {
	Results struct {
		ArtistMatches struct {
			Artist []lfmArtistSearchResult `json:"artist"`
		} `json:"artistmatches"`
		Attr struct {
			For string `json:"for"`
		} `json:"@attr"`
		OpenSearchTotalResults string `json:"opensearch:totalResults"`
		OpenSearchStartIndex   string `json:"opensearch:startIndex"`
		OpenSearchItemsPerPage string `json:"opensearch:itemsPerPage"`
	} `json:"results"`
}

// -------- album.search --------

type lfmAlbumSearchResult struct {
	Name       string     `json:"name"`
	Artist     string     `json:"artist"`
	MBID       string     `json:"mbid"`
	URL        string     `json:"url"`
	Image      []lfmImage `json:"image"`
	Streamable string     `json:"streamable"`
}

type lfmAlbumSearchResponse struct {
	Results struct {
		AlbumMatches struct {
			Album []lfmAlbumSearchResult `json:"album"`
		} `json:"albummatches"`
		OpenSearchTotalResults string `json:"opensearch:totalResults"`
		OpenSearchStartIndex   string `json:"opensearch:startIndex"`
		OpenSearchItemsPerPage string `json:"opensearch:itemsPerPage"`
	} `json:"results"`
}

// -------- ChartEntry (shared result type) --------

// ChartEntry is a provider-neutral chart entry returned by the chart methods.
type ChartEntry struct {
	Name      string
	Artist    string
	MBID      string
	Playcount int
	Listeners int
	ImageURL  string
	Rank      int
}

//
// Conversion helpers
//

func bestImage(images []lfmImage) string {
	// Prefer "extralarge" → "large" → "medium" → first available
	prefer := []string{"extralarge", "large", "medium", "small"}
	for _, size := range prefer {
		for i := range images {
			if images[i].Size == size && images[i].URL != "" {
				return images[i].URL
			}
		}
	}
	for i := range images {
		if images[i].URL != "" {
			return images[i].URL
		}
	}
	return ""
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func convertChartArtists(artists []lfmChartArtist) []ChartEntry {
	out := make([]ChartEntry, 0, len(artists))
	for i := range artists {
		out = append(out, ChartEntry{
			Name:      artists[i].Name,
			MBID:      artists[i].MBID,
			Playcount: atoiOrZero(artists[i].Playcount),
			Listeners: atoiOrZero(artists[i].Listeners),
			ImageURL:  bestImage(artists[i].Image),
			Rank:      i + 1,
		})
	}
	return out
}

func convertChartTracks(tracks []lfmChartTrack) []ChartEntry {
	out := make([]ChartEntry, 0, len(tracks))
	for i := range tracks {
		out = append(out, ChartEntry{
			Name:      tracks[i].Name,
			Artist:    tracks[i].Artist.Name,
			MBID:      tracks[i].MBID,
			Playcount: atoiOrZero(tracks[i].Playcount),
			Listeners: atoiOrZero(tracks[i].Listeners),
			ImageURL:  bestImage(tracks[i].Image),
			Rank:      i + 1,
		})
	}
	return out
}

func convertTagAlbums(albums []lfmTagAlbum) []ChartEntry {
	out := make([]ChartEntry, 0, len(albums))
	for i := range albums {
		out = append(out, ChartEntry{
			Name:     albums[i].Name,
			Artist:   albums[i].Artist.Name,
			MBID:     albums[i].MBID,
			ImageURL: bestImage(albums[i].Image),
			Rank:     i + 1,
		})
	}
	return out
}

func convertArtistInfoToDetails(a *lfmArtistInfo) *apiexternal_v2.ArtistDetails {
	details := &apiexternal_v2.ArtistDetails{
		ID:            a.MBID,
		Name:          a.Name,
		MusicBrainzID: a.MBID,
		Website:       a.URL,
		ProviderType:  apiexternal_v2.ProviderLastFM,
	}

	if img := bestImage(a.Image); img != "" {
		details.ImageURL = img
	}

	if len(a.Tags.Tag) > 0 {
		genres := make([]string, 0, len(a.Tags.Tag))
		for i := range a.Tags.Tag {
			genres = append(genres, a.Tags.Tag[i].Name)
		}
		details.Genres = genres
	}

	if a.Bio.Summary != "" {
		details.Bio = a.Bio.Summary
	}

	return details
}

func convertAlbumInfoToRelease(al *lfmAlbumInfo) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:            al.MBID,
		Title:         al.Name,
		MusicBrainzID: al.MBID,
		ProviderType:  apiexternal_v2.ProviderLastFM,
	}

	if al.Artist != "" {
		details.Artists = []apiexternal_v2.ArtistRef{{Name: al.Artist}}
	}

	if len(al.Tags.Tag) > 0 {
		genres := make([]string, 0, len(al.Tags.Tag))
		for i := range al.Tags.Tag {
			genres = append(genres, al.Tags.Tag[i].Name)
		}
		details.Genres = genres
	}

	if al.Wiki.Summary != "" {
		details.Notes = al.Wiki.Summary
	}

	// Tracks
	if len(al.Tracks.Track) > 0 {
		tracks := make([]apiexternal_v2.Track, 0, len(al.Tracks.Track))
		for i := range al.Tracks.Track {
			t := al.Tracks.Track[i]
			rank := int(t.Attr.Rank)
			if rank == 0 {
				rank = i + 1
			}
			secs := int(t.Duration)
			dur := time.Duration(secs) * time.Second
			tracks = append(tracks, apiexternal_v2.Track{
				Title:      t.Name,
				Position:   rank,
				Duration:   dur,
				DurationMs: secs * 1000,
			})
		}
		details.Tracks = tracks
		details.TrackCount = len(tracks)
	}

	return details
}

func convertArtistSearchResults(artists []lfmArtistSearchResult) []apiexternal_v2.ArtistSearchResult {
	out := make([]apiexternal_v2.ArtistSearchResult, 0, len(artists))
	for i := range artists {
		out = append(out, apiexternal_v2.ArtistSearchResult{
			ID:            artists[i].MBID,
			Name:          artists[i].Name,
			MusicBrainzID: artists[i].MBID,
			ImageURL:      bestImage(artists[i].Image),
			ProviderType:  apiexternal_v2.ProviderLastFM,
		})
	}
	return out
}

func convertAlbumSearchResults(albums []lfmAlbumSearchResult) []apiexternal_v2.ReleaseSearchResult {
	out := make([]apiexternal_v2.ReleaseSearchResult, 0, len(albums))
	for i := range albums {
		out = append(out, apiexternal_v2.ReleaseSearchResult{
			ID:            albums[i].MBID,
			Title:         albums[i].Name,
			MusicBrainzID: albums[i].MBID,
			ProviderType:  apiexternal_v2.ProviderLastFM,
		})
		if albums[i].Artist != "" {
			out[len(out)-1].Artists = []apiexternal_v2.ArtistRef{{Name: albums[i].Artist}}
		}
	}
	return out
}
