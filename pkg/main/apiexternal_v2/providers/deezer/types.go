package deezer

import (
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// Deezer Internal Types - Used for JSON unmarshaling
//

type deezerSearchResponse struct {
	Data  []deezerAlbumResult `json:"data"`
	Total int                 `json:"total"`
	Next  string              `json:"next"`
}

type deezerAlbumResult struct {
	ID          int          `json:"id"`
	Title       string       `json:"title"`
	UPC         string       `json:"upc"`
	Link        string       `json:"link"`
	CoverSmall  string       `json:"cover_small"`
	CoverMedium string       `json:"cover_medium"`
	CoverBig    string       `json:"cover_big"`
	CoverXL     string       `json:"cover_xl"`
	GenreID     int          `json:"genre_id"`
	NbTracks    int          `json:"nb_tracks"`
	ReleaseDate string       `json:"release_date"` // "YYYY-MM-DD"
	RecordType  string       `json:"record_type"`
	Available   bool         `json:"available"`
	Artist      deezerArtist `json:"artist"`
	Type        string       `json:"type"`
}

type deezerArtist struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Link          string `json:"link"`
	Picture       string `json:"picture"`
	PictureSmall  string `json:"picture_small"`
	PictureMedium string `json:"picture_medium"`
	PictureBig    string `json:"picture_big"`
	PictureXL     string `json:"picture_xl"`
	Type          string `json:"type"`
}

type deezerAlbumDetail struct {
	ID          int              `json:"id"`
	Title       string           `json:"title"`
	UPC         string           `json:"upc"`
	Link        string           `json:"link"`
	CoverSmall  string           `json:"cover_small"`
	CoverMedium string           `json:"cover_medium"`
	CoverBig    string           `json:"cover_big"`
	CoverXL     string           `json:"cover_xl"`
	GenreID     int              `json:"genre_id"`
	Genres      deezerGenres     `json:"genres"`
	Label       string           `json:"label"`
	NbTracks    int              `json:"nb_tracks"`
	Duration    int              `json:"duration"` // total duration in seconds
	Fans        int              `json:"fans"`
	ReleaseDate string           `json:"release_date"`
	RecordType  string           `json:"record_type"`
	Available   bool             `json:"available"`
	Artist      deezerArtist     `json:"artist"`
	Tracks      deezerTracksList `json:"tracks"`
	Type        string           `json:"type"`
}

type deezerGenres struct {
	Data []deezerGenre `json:"data"`
}

type deezerGenre struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type deezerTracksList struct {
	Data []deezerTrack `json:"data"`
}

type deezerTrack struct {
	ID                    int          `json:"id"`
	Readable              bool         `json:"readable"`
	Title                 string       `json:"title"`
	TitleShort            string       `json:"title_short"`
	TitleVersion          string       `json:"title_version"`
	ISRC                  string       `json:"isrc"`
	Link                  string       `json:"link"`
	Duration              int          `json:"duration"` // seconds
	TrackPosition         int          `json:"track_position"`
	DiskNumber            int          `json:"disk_number"`
	Rank                  int          `json:"rank"`
	ExplicitLyrics        bool         `json:"explicit_lyrics"`
	ExplicitContentLyrics int          `json:"explicit_content_lyrics"`
	ExplicitContentCover  int          `json:"explicit_content_cover"`
	Preview               string       `json:"preview"`
	MD5Image              string       `json:"md5_image"`
	Artist                deezerArtist `json:"artist"`
	Type                  string       `json:"type"`
}

//
// Conversion Functions
//

func convertSearchToReleases(results []deezerAlbumResult) []apiexternal_v2.ReleaseSearchResult {
	out := make([]apiexternal_v2.ReleaseSearchResult, 0, len(results))
	for _, r := range results {
		rel := apiexternal_v2.ReleaseSearchResult{
			ID:           strconv.Itoa(r.ID),
			Title:        r.Title,
			TrackCount:   r.NbTracks,
			CoverURL:     r.CoverXL,
			DeezerID:     r.ID,
			ProviderType: apiexternal_v2.ProviderDeezer,
		}
		if rel.CoverURL == "" {
			rel.CoverURL = r.CoverBig
		}

		if r.Artist.Name != "" {
			rel.Artists = []apiexternal_v2.ArtistRef{{
				Name: r.Artist.Name,
				ID:   strconv.Itoa(r.Artist.ID),
			}}
		}

		if r.ReleaseDate != "" {
			if year, _, ok := parseYear(r.ReleaseDate); ok {
				rel.ReleaseYear = year
			}
		}

		out = append(out, rel)
	}

	return out
}

func convertAlbumToDetails(album *deezerAlbumDetail) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:           strconv.Itoa(album.ID),
		Title:        album.Title,
		Label:        album.Label,
		DeezerID:     strconv.Itoa(album.ID),
		ProviderType: apiexternal_v2.ProviderDeezer,
		CoverURL:     album.CoverXL,
	}
	if details.CoverURL == "" {
		details.CoverURL = album.CoverBig
	}

	if album.ReleaseDate != "" {
		if year, t, ok := parseYear(album.ReleaseDate); ok {
			details.ReleaseYear = year
			details.ReleaseDate = t
		}
	}

	if album.Artist.Name != "" {
		details.Artists = []apiexternal_v2.ArtistRef{{
			Name: album.Artist.Name,
			ID:   strconv.Itoa(album.Artist.ID),
		}}
	}

	for _, g := range album.Genres.Data {
		details.Genres = append(details.Genres, g.Name)
	}

	tracks := make([]apiexternal_v2.Track, 0, len(album.Tracks.Data))
	for i, t := range album.Tracks.Data {
		tn := t.TrackPosition
		if tn == 0 {
			tn = i + 1
		}

		dn := t.DiskNumber
		if dn == 0 {
			dn = 1
		}

		tracks = append(tracks, apiexternal_v2.Track{
			Title:       t.Title,
			Position:    i + 1,
			TrackNumber: tn,
			DiscNumber:  dn,
			Duration:    time.Duration(t.Duration) * time.Second,
			ISRC:        t.ISRC,
			Artists: []apiexternal_v2.ArtistRef{{
				Name: t.Artist.Name,
				ID:   strconv.Itoa(t.Artist.ID),
			}},
		})
	}

	details.Tracks = tracks
	details.TrackCount = len(tracks)

	return details
}

// parseYear extracts the year from a "YYYY-MM-DD" string.
func parseYear(s string) (int, time.Time, bool) {
	if len(s) < 4 {
		return 0, time.Time{}, false
	}

	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, time.Time{}, false
	}

	t, _ := time.Parse("2006-01-02", s)

	return year, t, true
}
