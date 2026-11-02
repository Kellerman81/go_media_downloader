package omdb

import (
	"testing"
)

func TestParseVotes(t *testing.T) {
	tests := map[string]int{"1,234,567": 1234567, "42": 42, "N/A": 0, "": 0}

	for in, want := range tests {
		if got := parseVotes(in); got != want {
			t.Errorf("parseVotes(%q) = %d, want %d", in, got, want)
		}
	}
}
