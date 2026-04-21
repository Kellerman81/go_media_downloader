package itunes

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// iTunes Internal Types - Used for JSON unmarshaling
//

// itunesSearchResponse is the top-level response from /search.
type itunesSearchResponse struct {
	ResultCount int              `json:"resultCount"`
	Results     []itunesAlbum   `json:"results"`
}

// itunesLookupResponse is the top-level response from /lookup.
// The first result is always the album ("collection"); subsequent results are tracks.
type itunesLookupResponse struct {
	ResultCount int               `json:"resultCount"`
	Results     []itunesLookupEntry `json:"results"`
}

// itunesAlbum represents an album result from the search endpoint.
type itunesAlbum struct {
	WrapperType    string `json:"wrapperType"`    // "collection"
	CollectionType string `json:"collectionType"` // "Album"
	CollectionID   int    `json:"collectionId"`
	ArtistName     string `json:"artistName"`
	CollectionName string `json:"collectionName"`
	ArtworkURL100  string `json:"artworkUrl100"`
	ReleaseDate    string `json:"releaseDate"` // "YYYY-MM-DDTHH:MM:SSZ"
	TrackCount     int    `json:"trackCount"`
	PrimaryGenre   string `json:"primaryGenreName"`
}

// itunesLookupEntry covers both the album header and individual song entries
// returned by /lookup?id=...&entity=song.
type itunesLookupEntry struct {
	WrapperType     string `json:"wrapperType"`     // "collection" or "track"
	TrackID         int    `json:"trackId"`
	CollectionID    int    `json:"collectionId"`
	ArtistName      string `json:"artistName"`
	CollectionName  string `json:"collectionName"`
	TrackName       string `json:"trackName"`
	TrackNumber     int    `json:"trackNumber"`
	DiscNumber      int    `json:"discNumber"`
	TrackTimeMillis int64  `json:"trackTimeMillis"`
	ReleaseDate     string `json:"releaseDate"`
	ArtworkURL100   string `json:"artworkUrl100"`
	PrimaryGenre    string `json:"primaryGenreName"`
}

//
// Conversion Functions
//

func convertSearchToReleases(albums []itunesAlbum) []apiexternal_v2.ReleaseSearchResult {
	out := make([]apiexternal_v2.ReleaseSearchResult, 0, len(albums))
	for i := range albums {
		a := &albums[i]
		if a.WrapperType != "collection" {
			continue
		}
		rel := apiexternal_v2.ReleaseSearchResult{
			ID:           strconv.Itoa(a.CollectionID),
			ITunesID:     a.CollectionID,
			Title:        a.CollectionName,
			TrackCount:   a.TrackCount,
			CoverURL:     artworkURL(a.ArtworkURL100),
			ProviderType: apiexternal_v2.ProviderITunes,
		}
		if a.ArtistName != "" {
			rel.Artists = []apiexternal_v2.ArtistRef{{Name: a.ArtistName}}
		}
		if a.PrimaryGenre != "" {
			rel.Genres = []string{a.PrimaryGenre}
		}
		if y, t, ok := parseReleaseDate(a.ReleaseDate); ok {
			rel.ReleaseYear = y
			rel.ReleaseDate = t
		}
		out = append(out, rel)
	}
	return out
}

func convertLookupToDetails(entries []itunesLookupEntry) *apiexternal_v2.ReleaseDetails {
	if len(entries) == 0 {
		return nil
	}

	// First entry is always the collection header.
	hdr := &entries[0]
	if hdr.WrapperType != "collection" {
		return nil
	}

	details := &apiexternal_v2.ReleaseDetails{
		ID:           strconv.Itoa(hdr.CollectionID),
		ITunesID:     strconv.Itoa(hdr.CollectionID),
		Title:        hdr.CollectionName,
		CoverURL:     artworkURL(hdr.ArtworkURL100),
		ProviderType: apiexternal_v2.ProviderITunes,
	}
	if hdr.ArtistName != "" {
		details.Artists = []apiexternal_v2.ArtistRef{{Name: hdr.ArtistName}}
	}
	if hdr.PrimaryGenre != "" {
		details.Genres = []string{hdr.PrimaryGenre}
	}
	if y, t, ok := parseReleaseDate(hdr.ReleaseDate); ok {
		details.ReleaseYear = y
		details.ReleaseDate = t
	}

	tracks := make([]apiexternal_v2.Track, 0, len(entries)-1)
	for i := range entries[1:] {
		e := &entries[1:][i]
		if e.WrapperType != "track" {
			continue
		}
		tn := e.TrackNumber
		if tn == 0 {
			tn = i + 1
		}
		dn := e.DiscNumber
		if dn == 0 {
			dn = 1
		}
		tracks = append(tracks, apiexternal_v2.Track{
			Title:       e.TrackName,
			Position:    i + 1,
			TrackNumber: tn,
			DiscNumber:  dn,
			Duration:    time.Duration(e.TrackTimeMillis) * time.Millisecond,
			Artists:     []apiexternal_v2.ArtistRef{{Name: e.ArtistName}},
		})
	}
	details.Tracks = tracks
	details.TrackCount = len(tracks)

	return details
}

// artworkURL replaces the 100×100 thumbnail with the 600×600 version.
func artworkURL(u string) string {
	if u == "" {
		return ""
	}
	// Apple CDN pattern: .../{w}x{h}bb.jpg — swap 100x100bb for 600x600bb
	return strings.Replace(u, "100x100bb", "600x600bb", 1)
}

func parseReleaseDate(s string) (int, time.Time, bool) {
	if len(s) < 4 {
		return 0, time.Time{}, false
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, time.Time{}, false
	}
	t, _ := time.Parse(time.RFC3339, s)
	return year, t, true
}
