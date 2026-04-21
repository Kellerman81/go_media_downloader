package spotify

import "time"

// Authentication response from Spotify token endpoint.
type spotifyAuthResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// Search response structure.
type spotifySearchResponse struct {
	Albums *spotifyAlbumsPaging `json:"albums,omitempty"`
	Tracks *spotifyTracksPaging `json:"tracks,omitempty"`
}

type spotifyAlbumsPaging struct {
	Href     string         `json:"href"`
	Items    []spotifyAlbum `json:"items"`
	Limit    int            `json:"limit"`
	Next     string         `json:"next"`
	Offset   int            `json:"offset"`
	Previous string         `json:"previous"`
	Total    int            `json:"total"`
}

type spotifyTracksPaging struct {
	Href     string         `json:"href"`
	Items    []spotifyTrack `json:"items"`
	Limit    int            `json:"limit"`
	Next     string         `json:"next"`
	Offset   int            `json:"offset"`
	Previous string         `json:"previous"`
	Total    int            `json:"total"`
}

// Album structure.
type spotifyAlbum struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	AlbumType            string                `json:"album_type"` // album, single, compilation
	Artists              []spotifyArtistSimple `json:"artists"`
	AvailableMarkets     []string              `json:"available_markets"`
	ExternalURLs         spotifyExternalURLs   `json:"external_urls"`
	Href                 string                `json:"href"`
	Images               []spotifyImage        `json:"images"`
	ReleaseDate          string                `json:"release_date"`
	ReleaseDatePrecision string                `json:"release_date_precision"`
	TotalTracks          int                   `json:"total_tracks"`
	Type                 string                `json:"type"`
	URI                  string                `json:"uri"`
	Genres               []string              `json:"genres,omitempty"`
	Label                string                `json:"label,omitempty"`
	Popularity           int                   `json:"popularity,omitempty"`
	Tracks               *spotifyTracksPaging  `json:"tracks,omitempty"`
	ExternalIDs          spotifyExternalIDs    `json:"external_ids"`
}

// Track structure.
type spotifyTrack struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Artists          []spotifyArtistSimple `json:"artists"`
	Album            *spotifyAlbum         `json:"album,omitempty"`
	AvailableMarkets []string              `json:"available_markets"`
	DiscNumber       int                   `json:"disc_number"`
	DurationMs       int                   `json:"duration_ms"`
	Explicit         bool                  `json:"explicit"`
	ExternalIDs      spotifyExternalIDs    `json:"external_ids"`
	ExternalURLs     spotifyExternalURLs   `json:"external_urls"`
	Href             string                `json:"href"`
	IsLocal          bool                  `json:"is_local"`
	Popularity       int                   `json:"popularity,omitempty"`
	TrackNumber      int                   `json:"track_number"`
	Type             string                `json:"type"`
	URI              string                `json:"uri"`
}

// Artist structure (simplified version in album/track objects).
type spotifyArtistSimple struct {
	ExternalURLs spotifyExternalURLs `json:"external_urls"`
	Href         string              `json:"href"`
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Type         string              `json:"type"`
	URI          string              `json:"uri"`
}

// Artist full details.
type spotifyArtist struct {
	ExternalURLs spotifyExternalURLs `json:"external_urls"`
	Followers    spotifyFollowers    `json:"followers"`
	Genres       []string            `json:"genres"`
	Href         string              `json:"href"`
	ID           string              `json:"id"`
	Images       []spotifyImage      `json:"images"`
	Name         string              `json:"name"`
	Popularity   int                 `json:"popularity"`
	Type         string              `json:"type"`
	URI          string              `json:"uri"`
}

type spotifyFollowers struct {
	Href  string `json:"href"`
	Total int    `json:"total"`
}

type spotifyImage struct {
	Height int    `json:"height"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
}

type spotifyExternalURLs struct {
	Spotify string `json:"spotify"`
}

type spotifyExternalIDs struct {
	ISRC string `json:"isrc,omitempty"`
	EAN  string `json:"ean,omitempty"`
	UPC  string `json:"upc,omitempty"`
}

// Audio features structure.
type spotifyAudioFeatures struct {
	Acousticness     float64 `json:"acousticness"`
	AnalysisURL      string  `json:"analysis_url"`
	Danceability     float64 `json:"danceability"`
	DurationMs       int     `json:"duration_ms"`
	Energy           float64 `json:"energy"`
	ID               string  `json:"id"`
	Instrumentalness float64 `json:"instrumentalness"`
	Key              int     `json:"key"`
	Liveness         float64 `json:"liveness"`
	Loudness         float64 `json:"loudness"`
	Mode             int     `json:"mode"`
	Speechiness      float64 `json:"speechiness"`
	Tempo            float64 `json:"tempo"`
	TimeSignature    int     `json:"time_signature"`
	Valence          float64 `json:"valence"`
}

// Token cache structure for storing authentication tokens.
type tokenCache struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}
