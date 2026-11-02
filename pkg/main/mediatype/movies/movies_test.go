package movies

import (
	"slices"
	"testing"
)

func TestImdbPaddingCandidates(t *testing.T) {
	tests := []struct {
		src  string
		want []string
	}{
		{"123456", []string{"123456", "0123456"}},
		{"12345", []string{"12345", "012345", "0012345"}},
		{"1234567", []string{"1234567"}},
		{"12345678", []string{"12345678"}},
		{"1", []string{"1", "01", "001", "0001", "00001", "000001", "0000001"}},
	}

	for _, tt := range tests {
		if got := imdbPaddingCandidates(tt.src); !slices.Equal(got, tt.want) {
			t.Errorf("imdbPaddingCandidates(%q) = %v, want %v", tt.src, got, tt.want)
		}
	}
}
