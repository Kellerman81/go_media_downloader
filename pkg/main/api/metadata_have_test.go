package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestHaveAlready: a search result counts as had when an entry of it is in
// a list of the chosen list's media group, and only there.
func TestHaveAlready(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	database.ExecN("update dbmovies set moviedb_id = 200 where id = 2")
	database.ExecN("update dbseries set firstaired = '2019-03-01', thetvdb_id = 77 where id = 1")

	lists := func(form string) []string {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		return haveLists(c, "movie")
	}

	de, all := lists("have_list=DE+List"), lists("")
	if strings.Join(de, ",") != "DE List" || len(all) != 2 {
		t.Fatalf("lists: %v / %v", de, all)
	}

	if !haveMovie(de, &apiexternal_v2.MovieSearchResult{ID: 200}) {
		t.Error("the DE movie isn't had in DE")
	}

	if haveMovie([]string{"EN List"}, &apiexternal_v2.MovieSearchResult{ID: 200}) {
		t.Error("the DE movie counts in EN")
	}

	if haveMovie(de, &apiexternal_v2.MovieSearchResult{ID: 999}) {
		t.Error("an unknown movie is had")
	}

	x := []string{"X List"}
	for name, tc := range map[string]struct {
		s    apiexternal_v2.SeriesSearchResult
		want bool
	}{
		"name and year": {apiexternal_v2.SeriesSearchResult{Name: "show", FirstAirDate: time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)}, true},
		"other year":    {apiexternal_v2.SeriesSearchResult{Name: "Show", FirstAirDate: time.Date(2005, 1, 1, 0, 0, 0, 0, time.UTC)}, false},
		"original name": {apiexternal_v2.SeriesSearchResult{Name: "Die Show", OriginalName: "Show"}, true},
		"tvdb id":       {apiexternal_v2.SeriesSearchResult{Name: "renamed", TVDbID: 77}, true},
		"other tvdb id": {apiexternal_v2.SeriesSearchResult{Name: "Show", TVDbID: 78}, false},
	} {
		if got := haveSeries(x, &tc.s); got != tc.want {
			t.Errorf("series %s: %v, want %v", name, got, tc.want)
		}
	}

	card := renderComponentToString(createMovieResultCard(apiexternal_v2.MovieSearchResult{ID: 200, Title: "Deutsch"}, true))
	if !strings.Contains(card, "Have already") || !strings.Contains(card, "disabled") || strings.Contains(card, "add-movie-btn") {
		t.Errorf("had card: %s", card)
	}
}
