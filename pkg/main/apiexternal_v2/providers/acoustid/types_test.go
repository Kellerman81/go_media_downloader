package acoustid

import (
	"strings"
	"testing"
)

func TestConvertResultPicksReleaseOfChosenGroup(t *testing.T) {
	res := &acoustidResult{
		ID:    "aid",
		Score: 0.9,
		Recordings: []acoustidRecording{{
			ID:       "rec1",
			Title:    "Song",
			Duration: 180000,
			Artists:  []acoustidArtist{{ID: "a1", Name: "One"}, {ID: "a2", Name: "Two"}},
			ReleaseGroups: []acoustidReleaseGroup{
				{ID: "rg1", Title: "Album", Releases: []acoustidRelease{{
					ID: "rel-of-rg1", Title: "Album", MediumCount: 1,
					Date: acoustidDate{Year: 2001},
					Mediums: []acoustidMedium{{Position: 1, TrackCount: 10, Tracks: []acoustidTrack{
						{Position: 3, Title: "Song"},
					}}},
				}}},
				{ID: "rg2", Title: "Other"},
			},
			// Flat list belongs to an unknown group and must not be used.
			Releases: []acoustidRelease{{ID: "rel-other"}},
		}},
	}

	m := convertResult(res)[0]
	if m.RecordingTitle != "Song" || m.Title != "Song" || m.RecordingID != "rec1" {
		t.Errorf("title/id fields not filled: %+v", m)
	}

	if m.ArtistName != "One, Two" || m.ArtistID != "a1" {
		t.Errorf("ArtistName/ArtistID = %q/%q", m.ArtistName, m.ArtistID)
	}

	if m.AlbumID != "rg1" || m.ReleaseID != "rel-of-rg1" {
		t.Errorf("AlbumID/ReleaseID = %q/%q", m.AlbumID, m.ReleaseID)
	}

	if m.TrackNumber != 3 || m.ReleaseYear != 2001 || m.DurationMs != 180000 {
		t.Errorf("track/year/duration = %d/%d/%d", m.TrackNumber, m.ReleaseYear, m.DurationMs)
	}
}

func TestConvertResultFlatReleasesOnlyWithSingleGroup(t *testing.T) {
	rec := acoustidRecording{
		ID:            "r",
		ReleaseGroups: []acoustidReleaseGroup{{ID: "rg1"}, {ID: "rg2"}},
		Releases:      []acoustidRelease{{ID: "rel"}},
	}

	if got := convertResult(&acoustidResult{Recordings: []acoustidRecording{rec}})[0].ReleaseID; got != "" {
		t.Errorf("ambiguous flat release used: %q", got)
	}

	rec.ReleaseGroups = rec.ReleaseGroups[:1]
	if got := convertResult(&acoustidResult{Recordings: []acoustidRecording{rec}})[0].ReleaseID; got != "rel" {
		t.Errorf("single-group flat release = %q, want rel", got)
	}
}

func TestStatusErrorIncludesMessage(t *testing.T) {
	err := statusError("acoustid error: ", "error", &acoustidError{Code: 4, Message: "invalid API key"})
	if !strings.Contains(err.Error(), "invalid API key") {
		t.Errorf("error = %q", err)
	}

	if got := statusError("acoustid error: ", "error", nil).Error(); got != "acoustid error: error" {
		t.Errorf("nil error = %q", got)
	}
}
