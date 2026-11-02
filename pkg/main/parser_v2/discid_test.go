package parser_v2

import (
	"path/filepath"
	"testing"
)

// libdiscid reference TOC: offsets 150 15363 32314 46592 63414 80489, leadout 95462.
func discIDReferenceTracks(disc int) []discIDTrack {
	sectors := []int64{15213, 16951, 14278, 16822, 17075, 14973}

	out := make([]discIDTrack, len(sectors))
	for i, s := range sectors {
		out[i] = discIDTrack{discNum: disc, trackNum: i + 1, sectors: s}
	}

	return out
}

func TestDiscIDFromTracksReference(t *testing.T) {
	got, err := discIDFromTracks(discIDReferenceTracks(1))
	if err != nil {
		t.Fatal(err)
	}

	if want := "49HHV7Eb8UKF3aQiNmu1GR8vKTY-"; got != want {
		t.Errorf("discIDFromTracks = %q; want %q", got, want)
	}
}

// Files of a second disc with per-disc track numbers must not be interleaved
// into the first disc's TOC.
func TestDiscIDFromTracksMultiDiscUsesFirstDisc(t *testing.T) {
	disc2 := []discIDTrack{
		{discNum: 2, trackNum: 1, sectors: 20000},
		{discNum: 2, trackNum: 2, sectors: 21000},
		{discNum: 2, trackNum: 3, sectors: 22000},
	}

	mixed := append(append([]discIDTrack{}, disc2...), discIDReferenceTracks(1)...)

	got, err := discIDFromTracks(mixed)
	if err != nil {
		t.Fatal(err)
	}

	if want := "49HHV7Eb8UKF3aQiNmu1GR8vKTY-"; got != want {
		t.Errorf("multi-disc discIDFromTracks = %q; want first disc id %q", got, want)
	}
}

func TestDiscIDFromTracksMultiDiscTooFewOnFirstDisc(t *testing.T) {
	_, err := discIDFromTracks([]discIDTrack{
		{discNum: 1, trackNum: 1, sectors: 20000},
		{discNum: 2, trackNum: 1, sectors: 21000},
		{discNum: 2, trackNum: 2, sectors: 22000},
	})
	if err == nil {
		t.Error("expected an error for a single-track first disc")
	}
}

func TestCalculateDiscIDRejectsBadInput(t *testing.T) {
	if _, err := CalculateDiscID([]string{"one.flac"}); err == nil {
		t.Error("single track accepted")
	}

	missing := filepath.Join(t.TempDir(), "missing.flac")
	if _, err := CalculateDiscID([]string{missing, missing}); err == nil {
		t.Error("unreadable files accepted")
	}
}
