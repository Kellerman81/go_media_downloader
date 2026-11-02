package tags

import "testing"

func TestNormalizeID3Genre(t *testing.T) {
	if len(id3v1Genres) != 192 {
		t.Fatalf("genre table has %d entries, want 192", len(id3v1Genres))
	}

	tests := map[string]string{
		"(17)":           "Rock",
		"(17)Rock":       "Rock",
		"(17)Hard Stuff": "Hard Stuff",
		"17":             "Rock",
		"0":              "Blues",
		"(191)":          "Psybient",
		"(4)(17)":        "Disco/Rock",
		"(RX)":           "Remix",
		"(CR)":           "Cover",
		"(RX)(17)":       "Remix/Rock",
		"Rock":           "Rock",
		"RX":             "RX",
		"Trip-Hop":       "Trip-Hop",
		"((Brackets)":    "(Brackets)",
		"(999)":          "(999)",
		"192":            "192",
		"1999":           "1999",
		"  (8) ":         "Jazz",
		"":               "",
		"17\x00Synthpop": "Rock/Synthpop",
		"(Live)":         "(Live)",
	}

	for in, want := range tests {
		if got := normalizeID3Genre(in); got != want {
			t.Errorf("normalizeID3Genre(%q) = %q, want %q", in, got, want)
		}
	}
}
