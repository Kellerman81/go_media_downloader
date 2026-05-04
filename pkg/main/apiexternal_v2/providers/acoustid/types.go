package acoustid

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// AcoustID Internal Types - Used for JSON unmarshaling
//

// acoustidLookupResponse represents the lookup API response.
type acoustidLookupResponse struct {
	Status  string           `json:"status"`
	Results []acoustidResult `json:"results"`
}

// acoustidResult represents a single lookup result.
type acoustidResult struct {
	ID         string              `json:"id"` // AcoustID track ID
	Score      float64             `json:"score"`
	Recordings []acoustidRecording `json:"recordings"`
	Index      int                 `json:"index"` // For batch requests
}

// acoustidRecording represents a matched recording.
type acoustidRecording struct {
	ID            string                 `json:"id"` // MusicBrainz recording ID
	Title         string                 `json:"title"`
	Duration      int                    `json:"duration"` // milliseconds
	Artists       []acoustidArtist       `json:"artists"`
	ReleaseGroups []acoustidReleaseGroup `json:"releasegroups"`
	Releases      []acoustidRelease      `json:"releases"`
	Sources       int                    `json:"sources"` // Number of submissions
}

// acoustidArtist represents an artist.
type acoustidArtist struct {
	ID   string `json:"id"` // MusicBrainz artist ID
	Name string `json:"name"`
}

// acoustidReleaseGroup represents a release group (album).
type acoustidReleaseGroup struct {
	ID             string           `json:"id"` // MusicBrainz release group ID
	Title          string           `json:"title"`
	Type           string           `json:"type"`
	SecondaryTypes []string         `json:"secondarytypes"`
	Artists        []acoustidArtist `json:"artists"`
}

// acoustidRelease represents a release (specific edition).
type acoustidRelease struct {
	ID          string           `json:"id"` // MusicBrainz release ID
	Title       string           `json:"title"`
	Country     string           `json:"country"`
	Date        acoustidDate     `json:"date"`
	TrackCount  int              `json:"track_count"`
	MediumCount int              `json:"medium_count"`
	Mediums     []acoustidMedium `json:"mediums"`
	Artists     []acoustidArtist `json:"artists"`
}

// acoustidMedium represents a medium (disc) in a release.
type acoustidMedium struct {
	Position   int             `json:"position"`
	Format     string          `json:"format"`
	TrackCount int             `json:"track_count"`
	Tracks     []acoustidTrack `json:"tracks"`
}

// acoustidTrack represents a track on a medium.
type acoustidTrack struct {
	Position int              `json:"position"`
	Title    string           `json:"title"`
	Artists  []acoustidArtist `json:"artists"`
}

// acoustidDate represents a date in AcoustID format.
type acoustidDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

// acoustidSubmitResponse represents the submit API response.
type acoustidSubmitResponse struct {
	Status      string               `json:"status"`
	Submissions []acoustidSubmission `json:"submissions"`
}

// acoustidSubmission represents a submission result.
type acoustidSubmission struct {
	ID     string `json:"id"`
	Index  int    `json:"index"`
	Status string `json:"status"`
	Result struct {
		ID string `json:"id"` // AcoustID track ID
	} `json:"result"`
}

//
// Conversion Functions
//

func convertLookupResults(results []acoustidResult) []apiexternal_v2.RecordingMatch {
	matches := make([]apiexternal_v2.RecordingMatch, 0)

	for i := range results {
		matches = append(matches, convertResult(&results[i])...)
	}

	return matches
}

func convertResult(result *acoustidResult) []apiexternal_v2.RecordingMatch {
	matches := make([]apiexternal_v2.RecordingMatch, 0, len(result.Recordings))

	for i := range result.Recordings {
		match := apiexternal_v2.RecordingMatch{
			AcoustID:      result.ID,
			Score:         result.Score,
			MusicBrainzID: result.Recordings[i].ID,
			Title:         result.Recordings[i].Title,
			Duration:      time.Duration(result.Recordings[i].Duration) * time.Millisecond,
			Sources:       result.Recordings[i].Sources,
			ProviderType:  apiexternal_v2.ProviderAcoustID,
		}

		// Artists
		if len(result.Recordings[i].Artists) > 0 {
			artists := make([]string, 0, len(result.Recordings[i].Artists))

			artistIDs := make([]string, 0, len(result.Recordings[i].Artists))
			for j := range result.Recordings[i].Artists {
				artists = append(artists, result.Recordings[i].Artists[j].Name)
				artistIDs = append(artistIDs, result.Recordings[i].Artists[j].ID)
			}

			match.Artists = artists
			match.ArtistIDs = artistIDs
		}

		// Release groups (albums)
		if len(result.Recordings[i].ReleaseGroups) > 0 {
			rg := result.Recordings[i].ReleaseGroups[0]

			match.Album = rg.Title
			match.AlbumID = rg.ID
			match.AlbumType = rg.Type

			// Album artists if different from track artists
			if len(rg.Artists) > 0 {
				albumArtists := make([]string, 0, len(rg.Artists))
				for j := range rg.Artists {
					albumArtists = append(albumArtists, rg.Artists[j].Name)
				}

				match.AlbumArtists = albumArtists
			}
		}

		// Releases (specific editions)
		if len(result.Recordings[i].Releases) > 0 {
			match.ReleaseID = result.Recordings[i].Releases[0].ID
			match.Country = result.Recordings[i].Releases[0].Country

			// Release date
			if result.Recordings[i].Releases[0].Date.Year > 0 {
				match.ReleaseYear = result.Recordings[i].Releases[0].Date.Year
				match.ReleaseDate = convertDate(result.Recordings[i].Releases[0].Date)
			}

			// Track position from mediums
			for k := range result.Recordings[i].Releases[0].Mediums {
				for j := range result.Recordings[i].Releases[0].Mediums[k].Tracks {
					// Find matching track by title or position
					if result.Recordings[i].Releases[0].Mediums[k].Tracks[j].Title != result.Recordings[i].Title &&
						len(result.Recordings[i].Releases[0].Mediums[k].Tracks) != 1 {
						continue
					}

					match.TrackNumber = result.Recordings[i].Releases[0].Mediums[k].Tracks[j].Position
					match.DiscNumber = result.Recordings[i].Releases[0].Mediums[k].Position
					match.TotalTracks = result.Recordings[i].Releases[0].Mediums[k].TrackCount

					break
				}

				if match.TrackNumber > 0 {
					break
				}
			}

			match.TotalDiscs = result.Recordings[i].Releases[0].MediumCount
		}

		matches = append(matches, match)
	}

	return matches
}

// convertDate converts AcoustID date to time.Time.
func convertDate(d acoustidDate) time.Time {
	if d.Year == 0 {
		return time.Time{}
	}

	month := d.Month
	if month == 0 {
		month = 1
	}

	day := d.Day
	if day == 0 {
		day = 1
	}

	return time.Date(d.Year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
