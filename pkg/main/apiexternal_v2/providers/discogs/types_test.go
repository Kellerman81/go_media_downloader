package discogs

import (
	"testing"
	"time"
)

func TestParseDiscogsDate(t *testing.T) {
	tests := map[string]time.Time{
		"1999-00-00": time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		"1999-05-00": time.Date(1999, 5, 1, 0, 0, 0, 0, time.UTC),
		"1999-05-17": time.Date(1999, 5, 17, 0, 0, 0, 0, time.UTC),
		"1999":       time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC),
		"1999-05":    time.Date(1999, 5, 1, 0, 0, 0, 0, time.UTC),
		"":           {},
		"unknown":    {},
		"1999-13-01": {},
		"1999-02-30": {},
	}

	for in, want := range tests {
		if got := parseDiscogsDate(in); !got.Equal(want) {
			t.Errorf("parseDiscogsDate(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseDuration(t *testing.T) {
	tests := map[string]time.Duration{
		"3:45":    3*time.Minute + 45*time.Second,
		"12:05":   12*time.Minute + 5*time.Second,
		"1:02:03": time.Hour + 2*time.Minute + 3*time.Second,
		"":        0,
		"abc":     0,
		"1:x":     0,
	}

	for in, want := range tests {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestConvertMasterSkipsHeadingRows(t *testing.T) {
	master := &discogsMasterResponse{
		ID: 1,
		Tracklist: []discogsTrack{
			{Type: "heading", Title: "Part One"},
			{Type: "track", Position: "", Title: "First"},
			{Type: "index", Title: "Suite"},
			{Type: "track", Position: "", Title: "Second"},
		},
	}

	d := convertMasterToDetails(master)
	if len(d.Tracks) != 2 {
		t.Fatalf("tracks = %d, want 2", len(d.Tracks))
	}

	if d.Tracks[1].Title != "Second" || d.Tracks[1].TrackNumber != 2 || d.Tracks[1].Position != 2 {
		t.Errorf("second track = %+v", d.Tracks[1])
	}
}

func TestParseVinylSidePosition(t *testing.T) {
	tests := []struct {
		in          string
		disc, track int
		ok          bool
	}{
		{"A1", 1, 1, true},
		{"B12", 2, 12, true},
		{"c-3", 3, 3, true},
		{"D.4", 4, 4, true},
		{"AA1", 27, 1, true},
		{"1", 0, 0, false},
		{"A", 0, 0, false},
		{"A0", 0, 0, false},
		{"A1a", 0, 0, false},
		{"", 0, 0, false},
	}

	for _, tt := range tests {
		disc, track, ok := parseVinylSidePosition(tt.in)
		if disc != tt.disc || track != tt.track || ok != tt.ok {
			t.Errorf("parseVinylSidePosition(%q) = %d, %d, %t, want %d, %d, %t",
				tt.in, disc, track, ok, tt.disc, tt.track, tt.ok)
		}
	}
}

func TestCountDiscs(t *testing.T) {
	tests := []struct {
		formats []discogsFormat
		want    int
	}{
		{nil, 1},
		{[]discogsFormat{{Name: "CD", Qty: "2"}}, 2},
		{[]discogsFormat{{Name: "CD", Qty: "2"}, {Name: "DVD", Qty: "1"}}, 3},
		{[]discogsFormat{{Name: "Vinyl", Qty: ""}}, 1},
		{[]discogsFormat{{Name: "Vinyl", Qty: "0"}, {Name: "CD", Qty: "x"}}, 2},
	}

	for _, tt := range tests {
		if got := countDiscs(tt.formats); got != tt.want {
			t.Errorf("countDiscs(%+v) = %d, want %d", tt.formats, got, tt.want)
		}
	}
}
