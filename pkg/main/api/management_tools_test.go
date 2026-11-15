package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
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

func TestHandleMissingEpisodesFilters(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	// first_aired is stored as Go's time text, which SQLite's DATE() can't parse.
	aired := func(d time.Time) string { return d.UTC().Truncate(24 * time.Hour).String() }
	now := time.Now()

	for _, q := range []string{
		"insert into dbseries (id, seriename) values (1, 'Show'), (2, 'Other')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'EN'), (2, 2, 'EN')",
		"insert into dbserie_episodes (id, dbserie_id, season, episode, identifier, first_aired) values " +
			"(1, 1, '1', '1', 'S01E01', '" + aired(now.AddDate(-5, 0, 0)) + "'), " +
			"(2, 1, '1', '2', 'S01E02', '" + aired(now.AddDate(0, 0, -5)) + "'), " +
			"(3, 1, '1', '3', 'S01E03', '" + aired(now.AddDate(1, 0, 0)) + "'), " +
			"(4, 1, '1', '4', 'S01E04', NULL), " +
			"(5, 1, '1', '5', 'S01E05', '0001-01-01 00:00:00 +0000 UTC'), " +
			"(6, 2, '1', '1', 'S01E01', '" + aired(now.AddDate(-5, 0, 0)) + "')",
		"insert into serie_episodes (dbserie_id, serie_id, dbserie_episode_id, missing) values " +
			"(1, 1, 1, 1), (1, 1, 2, 1), (1, 1, 3, 1), (1, 1, 4, 1), (1, 1, 5, 1), (2, 2, 6, 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name, form string
		want       int
	}{
		{"one series, aired, any date", "missing_SerieID=1&missing_OnlyAired=on&missing_DateRange=all", 4},
		{"one series, aired, last 30 days", "missing_SerieID=1&missing_OnlyAired=on&missing_DateRange=30", 3},
		{"one series, not only aired", "missing_SerieID=1&missing_DateRange=all", 5},
		{"all series, aired, any date", "missing_SerieID=&missing_OnlyAired=on&missing_DateRange=all", 5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/missing-episodes", strings.NewReader(tc.form))
			ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			HandleMissingEpisodes(ctx)

			want := fmt.Sprintf("<td>Missing Episodes:</td><td>%d</td>", tc.want)
			if !strings.Contains(w.Body.String(), want) {
				i := strings.Index(w.Body.String(), "Missing Episodes:")
				t.Errorf("want %q, got %q", want, w.Body.String()[max(i, 0):min(max(i, 0)+60, w.Body.Len())])
			}
		})
	}
}
