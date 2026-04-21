package spotify

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// convertAlbumToSearchResult converts a Spotify album to a search result.
func convertAlbumToSearchResult(album *spotifyAlbum) apiexternal_v2.ReleaseSearchResult {
	artists := make([]apiexternal_v2.ArtistRef, len(album.Artists))
	for i, artist := range album.Artists {
		artists[i] = apiexternal_v2.ArtistRef{Name: artist.Name, ID: artist.ID}
	}

	releaseDate := parseSpotifyDate(album.ReleaseDate, album.ReleaseDatePrecision)

	releaseYear := 0
	if !releaseDate.IsZero() {
		releaseYear = releaseDate.Year()
	}

	coverURL := ""
	if len(album.Images) > 0 {
		coverURL = album.Images[0].URL
	}

	return apiexternal_v2.ReleaseSearchResult{
		ID:           album.ID,
		Title:        album.Name,
		Artists:      artists,
		ReleaseDate:  releaseDate,
		ReleaseYear:  releaseYear,
		Type:         album.AlbumType,
		CoverURL:     coverURL,
		TrackCount:   album.TotalTracks,
		Label:        album.Label,
		ProviderType: apiexternal_v2.ProviderSpotify,
	}
}

// convertAlbumToDetails converts a Spotify album to detailed release info.
func convertAlbumToDetails(album *spotifyAlbum) *apiexternal_v2.ReleaseDetails {
	artists := make([]apiexternal_v2.ArtistRef, len(album.Artists))
	for i, artist := range album.Artists {
		artists[i] = apiexternal_v2.ArtistRef{Name: artist.Name, ID: artist.ID}
	}

	releaseDate := parseSpotifyDate(album.ReleaseDate, album.ReleaseDatePrecision)

	releaseYear := 0
	if !releaseDate.IsZero() {
		releaseYear = releaseDate.Year()
	}

	coverURL := ""
	if len(album.Images) > 0 {
		// Spotify returns images sorted by size, take the largest
		coverURL = album.Images[0].URL
	}

	// Determine format (always Digital for Spotify)
	format := "Digital"

	details := &apiexternal_v2.ReleaseDetails{
		ID:           album.ID,
		Title:        album.Name,
		Artists:      artists,
		ReleaseDate:  releaseDate,
		ReleaseYear:  releaseYear,
		Type:         album.AlbumType,
		CoverURL:     coverURL,
		TrackCount:   album.TotalTracks,
		Format:       format,
		Label:        album.Label,
		Genres:       album.Genres,
		SpotifyID:    album.ID,
		ProviderType: apiexternal_v2.ProviderSpotify,
	}

	// Extract UPC if available
	if album.ExternalIDs.UPC != "" {
		details.Barcode = album.ExternalIDs.UPC
	} else if album.ExternalIDs.EAN != "" {
		details.Barcode = album.ExternalIDs.EAN
	}

	// Convert tracks if present
	if album.Tracks != nil && len(album.Tracks.Items) > 0 {
		details.Tracks = make([]apiexternal_v2.Track, len(album.Tracks.Items))
		for i, track := range album.Tracks.Items {
			details.Tracks[i] = convertSpotifyTrackToTrack(&track, i+1)
		}
	}

	return details
}

// convertTrackToSearchResult converts a Spotify track to a search result.
func convertTrackToSearchResult(track *spotifyTrack) apiexternal_v2.TrackSearchResult {
	artists := make([]string, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = artist.Name
	}

	albumName := ""
	if track.Album != nil {
		albumName = track.Album.Name
	}

	return apiexternal_v2.TrackSearchResult{
		ID:           track.ID,
		Title:        track.Name,
		Artists:      artists,
		Album:        albumName,
		TrackNumber:  track.TrackNumber,
		DiscNumber:   track.DiscNumber,
		DurationMs:   track.DurationMs,
		ProviderType: apiexternal_v2.ProviderSpotify,
	}
}

// convertTrackToDetails converts a Spotify track to detailed track info.
func convertTrackToDetails(track *spotifyTrack) *apiexternal_v2.TrackDetails {
	artists := make([]string, len(track.Artists))

	artistIDs := make([]string, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = artist.Name
		artistIDs[i] = artist.ID
	}

	albumID := ""
	albumName := ""
	releaseDate := time.Time{}
	releaseYear := 0
	coverURL := ""

	if track.Album != nil {
		albumID = track.Album.ID
		albumName = track.Album.Name

		releaseDate = parseSpotifyDate(track.Album.ReleaseDate, track.Album.ReleaseDatePrecision)
		if !releaseDate.IsZero() {
			releaseYear = releaseDate.Year()
		}

		if len(track.Album.Images) > 0 {
			coverURL = track.Album.Images[0].URL
		}
	}

	details := &apiexternal_v2.TrackDetails{
		ID:           track.ID,
		Title:        track.Name,
		Artists:      artists,
		ArtistIDs:    artistIDs,
		Album:        albumName,
		AlbumID:      albumID,
		TrackNumber:  track.TrackNumber,
		DiscNumber:   track.DiscNumber,
		DurationMs:   track.DurationMs,
		ReleaseDate:  releaseDate,
		ReleaseYear:  releaseYear,
		CoverURL:     coverURL,
		Explicit:     track.Explicit,
		ProviderType: apiexternal_v2.ProviderSpotify,
	}

	// Extract ISRC if available
	if track.ExternalIDs.ISRC != "" {
		details.ISRC = track.ExternalIDs.ISRC
	}

	return details
}

// convertSpotifyTrackToTrack converts a Spotify track to the generic Track type.
func convertSpotifyTrackToTrack(track *spotifyTrack, position int) apiexternal_v2.Track {
	artists := make([]apiexternal_v2.ArtistRef, len(track.Artists))
	for i, artist := range track.Artists {
		artists[i] = apiexternal_v2.ArtistRef{Name: artist.Name, ID: artist.ID}
	}

	return apiexternal_v2.Track{
		ID:          track.ID,
		Title:       track.Name,
		Position:    position,
		TrackNumber: track.TrackNumber,
		DiscNumber:  track.DiscNumber,
		Duration:    time.Duration(track.DurationMs) * time.Millisecond,
		Artists:     artists,
		ISRC:        track.ExternalIDs.ISRC,
	}
}

// convertArtistToDetails converts a Spotify artist to detailed artist info.
func convertArtistToDetails(artist *spotifyArtist) *apiexternal_v2.ArtistDetails {
	imageURL := ""
	if len(artist.Images) > 0 {
		imageURL = artist.Images[0].URL
	}

	return &apiexternal_v2.ArtistDetails{
		ID:           artist.ID,
		Name:         artist.Name,
		Genres:       artist.Genres,
		ImageURL:     imageURL,
		ProviderType: apiexternal_v2.ProviderSpotify,
	}
}

// parseSpotifyDate parses Spotify's date format based on precision.
// Spotify returns dates in different precisions: "year", "month", or "day".
func parseSpotifyDate(dateStr, precision string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	var layout string
	switch precision {
	case "day":
		layout = "2006-01-02"
	case "month":
		layout = "2006-01"
	case "year":
		layout = "2006"
	default:
		// Try to auto-detect based on string length
		parts := strings.Split(dateStr, "-")
		switch len(parts) {
		case 3:
			layout = "2006-01-02"
		case 2:
			layout = "2006-01"
		case 1:
			layout = "2006"
		default:
			return time.Time{}
		}
	}

	parsed, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Time{}
	}

	return parsed
}
