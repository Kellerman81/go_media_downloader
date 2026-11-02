package acoustid

import (
	"errors"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// AcoustID Internal Types - Used for JSON unmarshaling
//

// acoustidLookupResponse represents the lookup API response.
type acoustidLookupResponse struct {
	Status  string           `json:"status"`
	Error   *acoustidError   `json:"error"`
	Results []acoustidResult `json:"results"`
}

// acoustidError is the error object returned alongside status "error".
type acoustidError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func statusError(prefix, status string, e *acoustidError) error {
	if e != nil && e.Message != "" {
		return errors.New(logger.JoinStrings(prefix, status, ": ", e.Message))
	}

	return errors.New(logger.JoinStrings(prefix, status))
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
	// With meta=releasegroups+releases AcoustID nests the releases inside
	// their release group.
	Releases []acoustidRelease `json:"releases"`
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
	Error       *acoustidError       `json:"error"`
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
		rec := &result.Recordings[i]
		match := apiexternal_v2.RecordingMatch{
			AcoustID:       result.ID,
			Score:          result.Score,
			MusicBrainzID:  rec.ID,
			RecordingID:    rec.ID,
			Title:          rec.Title,
			RecordingTitle: rec.Title,
			Duration:       time.Duration(rec.Duration) * time.Millisecond,
			DurationMs:     rec.Duration,
			Sources:        rec.Sources,
			ProviderType:   apiexternal_v2.ProviderAcoustID,
		}

		// Artists
		if len(rec.Artists) > 0 {
			artists := make([]string, 0, len(rec.Artists))

			artistIDs := make([]string, 0, len(rec.Artists))
			for j := range rec.Artists {
				artists = append(artists, rec.Artists[j].Name)
				artistIDs = append(artistIDs, rec.Artists[j].ID)
			}

			match.Artists = artists
			match.ArtistIDs = artistIDs
			match.ArtistName = strings.Join(artists, ", ")
			match.ArtistID = artistIDs[0]
		}

		var release *acoustidRelease

		// Release groups (albums)
		if len(rec.ReleaseGroups) > 0 {
			rg := &rec.ReleaseGroups[0]

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

			if len(rg.Releases) > 0 {
				release = &rg.Releases[0]
			}
		}

		// Flat recording-level releases carry no release-group reference, so
		// they can only be attributed to the chosen group when there is no
		// more than one group to choose from.
		if release == nil && len(rec.Releases) > 0 && len(rec.ReleaseGroups) <= 1 {
			release = &rec.Releases[0]
		}

		// Releases (specific editions)
		if release != nil {
			match.ReleaseID = release.ID
			match.ReleaseTitle = release.Title
			match.Country = release.Country

			// Release date
			if release.Date.Year > 0 {
				match.ReleaseYear = release.Date.Year
				match.ReleaseDate = convertDate(release.Date)
			}

			// Track position from mediums
			for k := range release.Mediums {
				for j := range release.Mediums[k].Tracks {
					// Find matching track by title or position
					if !strings.EqualFold(release.Mediums[k].Tracks[j].Title, rec.Title) &&
						len(release.Mediums[k].Tracks) != 1 {
						continue
					}

					match.TrackNumber = release.Mediums[k].Tracks[j].Position
					match.DiscNumber = release.Mediums[k].Position
					match.TotalTracks = release.Mediums[k].TrackCount

					break
				}

				if match.TrackNumber > 0 {
					break
				}
			}

			match.TotalDiscs = release.MediumCount
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
