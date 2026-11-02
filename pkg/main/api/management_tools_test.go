package api

import (
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestPreviewCombinedReorder(t *testing.T) {
	qual := &config.QualityConfig{QualityReorder: []config.QualityReorderConfig{
		{Name: "1080p", ReorderType: "resolution", Newpriority: 5},
		{Name: "1080p,BluRay", ReorderType: "Combined_Res_Qual", Newpriority: 900},
		{Name: "1080p,bluray", ReorderType: "combined_res_qual", Newpriority: 100},
		{Name: "720p, WEB-DL", ReorderType: "combined_res_qual", Newpriority: 50},
	}}

	tests := []struct {
		name, reso, quality string
		wantReso, wantQual  int
		wantMatch           bool
	}{
		{"first match wins", "1080p", "BLURAY", 900, 0, true},
		{"no trimming like parser", "720p", "WEB-DL", 10, 20, false},
		{"no rule", "2160p", "BluRay", 10, 20, false},
	}

	for _, tt := range tests {
		reso, q, matched := previewCombinedReorder(qual, tt.reso, tt.quality, 10, 20)
		if reso != tt.wantReso || q != tt.wantQual || matched != tt.wantMatch {
			t.Errorf("%s: got (%d,%d,%v), want (%d,%d,%v)",
				tt.name, reso, q, matched, tt.wantReso, tt.wantQual, tt.wantMatch)
		}
	}
}

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
