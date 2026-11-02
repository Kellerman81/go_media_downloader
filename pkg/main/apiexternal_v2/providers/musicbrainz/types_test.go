package musicbrainz

import (
	"testing"
	"time"
)

func TestConvertReleaseGroupToDetailsUsesReleaseID(t *testing.T) {
	group := &mbReleaseGroupResponse{
		ID: "group-id",
		Releases: []mbRelease{
			{ID: "bootleg", Status: "Bootleg"},
			{ID: "official", Status: "Official"},
		},
	}

	d := convertReleaseGroupToDetails(group)
	if d.ReleaseGroupID != "group-id" || d.MusicBrainzID != "official" {
		t.Errorf("ReleaseGroupID/MusicBrainzID = %q/%q", d.ReleaseGroupID, d.MusicBrainzID)
	}

	if got := convertReleaseGroupToDetails(&mbReleaseGroupResponse{ID: "g"}).MusicBrainzID; got != "" {
		t.Errorf("group without releases: MusicBrainzID = %q, want empty", got)
	}

	if got := pickGroupReleaseID([]mbRelease{{ID: "x", Status: "Promotion"}}); got != "x" {
		t.Errorf("fallback = %q", got)
	}
}

func TestParseMBDate(t *testing.T) {
	for in, want := range map[string]time.Time{
		"1973-03-01": time.Date(1973, 3, 1, 0, 0, 0, 0, time.UTC),
		"1973-03":    time.Date(1973, 3, 1, 0, 0, 0, 0, time.UTC),
		"1973":       time.Date(1973, 1, 1, 0, 0, 0, 0, time.UTC),
		"":           {},
		"1973-13-01": {},
		"73":         {},
	} {
		if got := parseMBDate(in); !got.Equal(want) {
			t.Errorf("parseMBDate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExtractWikidataID(t *testing.T) {
	for in, want := range map[string]string{
		"https://www.wikidata.org/wiki/Q2306": "Q2306",
		"https://example.org/wiki/Pink":       "",
		"":                                    "",
	} {
		if got := extractWikidataID(in); got != want {
			t.Errorf("extractWikidataID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractYear(t *testing.T) {
	for in, want := range map[string]int{"1973-03-01": 1973, "1973": 1973, "": 0, "197": 0, "????-01": 0} {
		if got := extractYear(in); got != want {
			t.Errorf("extractYear(%q) = %d, want %d", in, got, want)
		}
	}
}
