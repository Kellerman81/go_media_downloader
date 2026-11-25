package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

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
