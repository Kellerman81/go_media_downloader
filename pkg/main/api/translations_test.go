package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// libGetLang is libGet for a browser in language lang.
func libGetLang(t *testing.T, h gin.HandlerFunc, query, lang string) string {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	c.Request.Header.Set("Accept-Language", lang)
	h(c)

	return plain(w.Body.String())
}

// TestMetadataTranslationsShown: the Library and the calendar show the
// translations into the account's language, the stored texts otherwise.
func TestMetadataTranslationsShown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTestLanguages(t)
	libraryTestData(t)

	database.SetTranslation("dbmovie", 1, "title", "de", "Film 01", "Movie 01")
	database.SetTranslation("dbmovie", 1, "overview", "de", "Handlung 1", "Plot 1")
	database.SetTranslation("dbserie", 1, "title", "de", "Die Show", "Site Show")
	database.SetTranslation("dbserie_episode", 1, "title", "de", "Pilotfolge", "Pilot")

	if out := libGetLang(t, HandleLibraryGrid, "group=movie_EN&filter=all&sort=title", "de"); !strings.Contains(out, "Film 01") || strings.Contains(out, "Movie 01") {
		t.Errorf("German grid:\n%.1500s", out)
	}

	if out := libGetLang(t, HandleLibraryGrid, "group=movie_EN&filter=all&sort=title", "en"); !strings.Contains(out, "Movie 01") || strings.Contains(out, "Film 01") {
		t.Errorf("English grid shows the translation:\n%.1500s", out)
	}

	if out := libGetLang(t, HandleLibraryItem, "type=movie&id=1", "de"); !strings.Contains(out, "Handlung 1") || !strings.Contains(out, `data-title="Film 01"`) {
		t.Errorf("German movie:\n%.1500s", out)
	}

	if out := libGetLang(t, HandleLibraryItem, "type=series&id=1", "de"); !strings.Contains(out, "Pilotfolge") || !strings.Contains(out, `data-title="Die Show"`) {
		t.Errorf("German series:\n%.2500s", out)
	}

	if out := libGetLang(t, HandleLibraryGrid, "group=serie_X&view=episodes&filter=all", "de"); !strings.Contains(out, "Pilotfolge") || !strings.Contains(out, "Die Show") {
		t.Errorf("German episode view:\n%.1500s", out)
	}

	events := []calendarEvent{
		{Type: "movie", Title: "Movie 01", Overview: "Plot 1", dbKey: "1"},
		{Type: "series", Title: "Site Show - Pilot", Show: "Site Show", EpTitle: "Pilot", dbKey: "1", coverKey: "1"},
		{Type: "movie", Title: "Movie 02", dbKey: "2"},
	}
	translateCalendarEvents(events, "de")

	if events[0].Title != "Film 01" || events[0].Overview != "Handlung 1" || events[1].Title != "Die Show - Pilotfolge" ||
		events[1].Show != "Die Show" || events[2].Title != "Movie 02" {
		t.Errorf("calendar %+v", events)
	}
}
