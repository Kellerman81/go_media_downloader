package theaudiodb

import (
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// TheAudioDB Internal Types - Used for JSON unmarshaling
//

// tadSearchAlbumResponse is the top-level response from searchalbum.php.
type tadSearchAlbumResponse struct {
	Album []tadAlbum `json:"album"`
}

// tadTrackListResponse is the top-level response from track.php.
type tadTrackListResponse struct {
	Track []tadTrack `json:"track"`
}

// tadAlbum represents a single album entry from TheAudioDB.
type tadAlbum struct {
	IDAlbum           string `json:"idAlbum"`
	StrAlbum          string `json:"strAlbum"`
	StrArtist         string `json:"strArtist"`
	IntYearReleased   string `json:"intYearReleased"`
	StrAlbumThumb     string `json:"strAlbumThumb"`
	StrMusicBrainzID  string `json:"strMusicBrainzID"`
	IntScore          string `json:"intScore"`
	StrReleaseFormat  string `json:"strReleaseFormat"`
	StrLabel          string `json:"strLabel"`
}

// tadTrack represents a single track entry from TheAudioDB.
type tadTrack struct {
	IDTrack          string `json:"idTrack"`
	IDAlbum          string `json:"idAlbum"`
	IDArtist         string `json:"idArtist"`
	StrTrack         string `json:"strTrack"`
	StrAlbum         string `json:"strAlbum"`
	StrArtist        string `json:"strArtist"`
	IntTrackNumber   string `json:"intTrackNumber"`
	IntCD            string `json:"intCD"` // disc number
	IntDuration      string `json:"intDuration"` // milliseconds
	StrMusicBrainzID string `json:"strMusicBrainzID"`
}

//
// Conversion Functions
//

func convertAlbumSearchToReleases(albums []tadAlbum) []apiexternal_v2.ReleaseSearchResult {
	out := make([]apiexternal_v2.ReleaseSearchResult, 0, len(albums))
	for i := range albums {
		a := &albums[i]
		rel := apiexternal_v2.ReleaseSearchResult{
			ID:           a.IDAlbum,
			TheAudioDBID: a.IDAlbum,
			Title:        a.StrAlbum,
			MusicBrainzID: a.StrMusicBrainzID,
			CoverURL:     a.StrAlbumThumb,
			ProviderType: apiexternal_v2.ProviderTheAudioDB,
		}
		if a.StrArtist != "" {
			rel.Artists = []apiexternal_v2.ArtistRef{{Name: a.StrArtist}}
		}
		if y, err := strconv.Atoi(a.IntYearReleased); err == nil {
			rel.ReleaseYear = y
		}
		out = append(out, rel)
	}
	return out
}

func convertTracksToDetails(albumID string, tracks []tadTrack) *apiexternal_v2.ReleaseDetails {
	details := &apiexternal_v2.ReleaseDetails{
		ID:           albumID,
		TheAudioDBID: albumID,
		ProviderType: apiexternal_v2.ProviderTheAudioDB,
	}

	if len(tracks) > 0 {
		details.Title = tracks[0].StrAlbum
		if tracks[0].StrArtist != "" {
			details.Artists = []apiexternal_v2.ArtistRef{{Name: tracks[0].StrArtist}}
		}
	}

	converted := make([]apiexternal_v2.Track, 0, len(tracks))
	for i, t := range tracks {
		tn, _ := strconv.Atoi(t.IntTrackNumber)
		if tn == 0 {
			tn = i + 1
		}
		dn, _ := strconv.Atoi(t.IntCD)
		if dn == 0 {
			dn = 1
		}
		var dur time.Duration
		if ms, err := strconv.ParseInt(t.IntDuration, 10, 64); err == nil && ms > 0 {
			dur = time.Duration(ms) * time.Millisecond
		}
		converted = append(converted, apiexternal_v2.Track{
			Title:        t.StrTrack,
			Position:     i + 1,
			TrackNumber:  tn,
			DiscNumber:   dn,
			Duration:     dur,
			MusicBrainzID: t.StrMusicBrainzID,
		})
	}
	details.Tracks = converted
	details.TrackCount = len(converted)

	return details
}
