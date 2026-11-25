package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestTranslationsEditor: the database edit form of a metadata row lists
// its translations; saved ones are manual, an emptied one goes, a field the
// table doesn't have is refused.
func TestTranslationsEditor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	database.SetTranslation("dbmovie", 1, "title", "de", "Film 01", "Movie 01")

	post := func(form url.Values) (int, string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		HandleTranslationSave(c)

		return w.Code, w.Body.String()
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Params = gin.Params{{Key: StrName, Value: "dbmovies"}, {Key: StrID, Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	apiAdminTableDataEditForm(c)

	if out := w.Body.String(); !strings.Contains(out, `id="gmdTranslations"`) || !strings.Contains(out, "Film 01") {
		t.Errorf("edit form without translations:\n%.800s", out)
	}

	if code, out := post(url.Values{"type": {"dbmovie"}, "id": {"1"}, "lang": {"German"}, "field": {"overview"}, "text": {"Handlung"}}); code != http.StatusOK ||
		!strings.Contains(out, "Handlung") {
		t.Fatalf("save: %d %.500s", code, out)
	}

	if tr := database.GetTranslations("dbmovie", 1); len(tr) != 2 || tr[0].Language != "de" || tr[0].Field != "overview" || !tr[0].Manual {
		t.Errorf("translations %+v", tr)
	}

	if code, _ := post(url.Values{"type": {"dbmovie"}, "id": {"1"}, "lang": {"de"}, "field": {"bio"}, "text": {"x"}}); code != http.StatusBadRequest {
		t.Errorf("unknown field: %d", code)
	}

	if code, _ := post(url.Values{"type": {"dbmovie"}, "id": {"1"}, "lang": {"de"}, "field": {"title"}, "text": {""}}); code != http.StatusOK {
		t.Errorf("remove: %d", code)
	}

	if tr := database.GetTranslations("dbmovie", 1); len(tr) != 1 || tr[0].Field != "overview" {
		t.Errorf("after removing the title: %+v", tr)
	}
}
