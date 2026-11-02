package musicbrainz

import (
	"testing"
)

func TestClampLimit(t *testing.T) {
	for in, want := range map[int]int{0: 25, -1: 25, 50: 50, 100: 100, 500: 100} {
		if got := clampLimit(in); got != want {
			t.Errorf("clampLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
