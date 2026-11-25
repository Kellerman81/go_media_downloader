package api

import (
	"slices"
	"testing"
)

func TestCollapseNestedPaths(t *testing.T) {
	got := collapseNestedPaths([]string{"/data/b", "/data", "/data/", "/database", "/other/x", "/other/x/y"})
	want := []string{"/data", "/database", "/other/x"}

	if !slices.Equal(got, want) {
		t.Fatalf("collapseNestedPaths = %v, want %v", got, want)
	}
}

func TestPathWithinAny(t *testing.T) {
	roots := []string{"/data2/movies", "/data/EN_Series"}

	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/data2/movies", true},
		{"/data2/movies/Film (2020)", true},
		{"/data2/movies2", false},
		{"/data2", false},
		{"/data/EN_Series/../DE_Series", false},
		{"/etc", false},
	} {
		if got := pathWithinAny(cleanPathForTest(tc.path), roots); got != tc.want {
			t.Errorf("pathWithinAny(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func cleanPathForTest(p string) string {
	return collapseNestedPaths([]string{p})[0]
}
