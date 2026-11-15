package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVerifyGoTemplateRendersTemplate(t *testing.T) {
	tests := []struct {
		name     string
		dataType string
		tpl      string
		valid    bool
	}{
		{"valid parser field", "parser", "{{.Dbmovie.Title}} ({{.Dbmovie.Year}})", true},
		{"valid notification field", "notification", "{{.Title}} -> {{.Targetpath}}", true},
		{"unterminated if", "parser", "{{if .Dbmovie.Title}}x", false},
		{"unknown nested field", "parser", "{{.Dbmovie.Nope}}", false},
		{"unknown function", "notification", "{{nosuchfunc .Title}}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := verifyGoTemplate(tt.tpl, tt.dataType)
			if v.Valid != tt.valid {
				t.Fatalf("Valid = %v, want %v (errors %v)", v.Valid, tt.valid, v.Errors)
			}
		})
	}
}

func TestHandleNamingCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct{ name, tpl, dataType, want string }{
		{"preview and valid", "{{.Dbmovie.Title}} ({{.Dbmovie.Year}})", "movie", "Inception (2000)"},
		{"invalid field", "{{.Dbmovie.Nope}}", "movie", "alert-warning"},
		{"music sample title", "{{printf \"%02d\" .Track}} - {{.Title}}", "music", "04 - Time"},
		{"notification", "{{.Title}}", "notification", "text-success"},
		{"unknown type", "{{.Title}}", "bogus", "Unknown template type bogus"},
		{"empty", " ", "parser", "Type a template"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			form := url.Values{"template": {tc.tpl}, "data_type": {tc.dataType}}
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
			c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			HandleNamingCheck(c)

			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("status %d, body lacks %q:\n%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}

func TestRenderNamingFields(t *testing.T) {
	var b strings.Builder
	if err := renderNamingFields("movie").Render(&b); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`data-ng-insert="{{.Dbmovie.Title}}"`, "Template structures", "Movie Information"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("fields panel lacks %s", want)
		}
	}
}

func TestMetadataLookupPreserveQueriesCompile(t *testing.T) {
	db := openMigratedSchema(t)

	explainOK(t, db, "book author", "SELECT dbauthor_id FROM dbbooks WHERE id = ?", 1)
	explainOK(t, db, "book series", "SELECT dbbook_series_id FROM dbbooks WHERE id = ?", 1)
	explainOK(t, db, "audiobook book", "SELECT dbbook_id FROM dbaudiobooks WHERE id = ?", 1)
}

func TestNamingFieldsPerType(t *testing.T) {
	all, _ := getFieldsForDataType("parser")

	has := func(fields []TemplateField, name string) bool {
		for _, f := range fields {
			if f.Name == name {
				return true
			}
		}

		return false
	}

	for _, tc := range []struct {
		dataType  string
		want, not []string
	}{
		{"movie", []string{"Dbmovie.Title", "Source.Resolution", "TitleSource"}, []string{"Dbserie.Seriename", "Dbalbum.Title", "Track"}},
		{"series", []string{"Dbserie.Seriename", "DbserieEpisode.Title", "Episodes"}, []string{"Dbmovie.Title", "Author.Name"}},
		{"music", []string{"Dbalbum.Title", "AlbumArtist.Name", "Track", "TotalDiscs"}, []string{"Dbmovie.Title", "Dbaudiobook.Title"}},
		{"book", []string{"Dbbook.Title", "Author.Name", "BookSeries.Name"}, []string{"Dbaudiobook.Title", "Track"}},
		{"audiobook", []string{"Dbaudiobook.Title", "DbaudiobookChapter.Title", "Dbbook.SeriesPosition", "Track"}, []string{"Dbmovie.Title", "Dbalbum.Title"}},
	} {
		fields, _ := getFieldsForDataType(tc.dataType)
		if len(fields) >= len(all) {
			t.Errorf("%s: %d fields, not filtered from %d", tc.dataType, len(fields), len(all))
		}

		for _, w := range tc.want {
			if !has(fields, w) {
				t.Errorf("%s lacks %s", tc.dataType, w)
			}
		}

		for _, n := range tc.not {
			if has(fields, n) {
				t.Errorf("%s has %s", tc.dataType, n)
			}
		}
	}

	// Filtering must not have touched the full list.
	if again, _ := getFieldsForDataType("parser"); len(again) != len(all) || !has(again, "Dbmovie.Title") {
		t.Error("full field list changed")
	}
}

func TestNamingExamplesValid(t *testing.T) {
	seen := map[string]bool{}

	for _, e := range ngExamples {
		if !ngKnownType(e.DataType) {
			t.Errorf("%s: unknown type %s", e.Label, e.DataType)
		}

		seen[e.DataType] = true

		if v := verifyGoTemplate(e.Template, e.DataType); !v.Valid {
			t.Errorf("%s: %v", e.Label, v.Errors)
		}
	}

	for _, typ := range []string{"movie", "series", "music", "book", "audiobook", "notification"} {
		if !seen[typ] {
			t.Errorf("no example for %s", typ)
		}
	}
}

func TestRenderNamingGeneratorPagePrefill(t *testing.T) {
	var b strings.Builder
	if err := renderNamingGeneratorPage("tok", "music", "{{.Title}}").Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()
	for _, want := range []string{
		`<option value="music" selected>`,
		`>{{.Title}}</textarea>`,
		`hx-trigger="load"`,
		`data-ng-insert="{{.Dbalbum.Title}}"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page lacks %s", want)
		}
	}

	if strings.Contains(out, `data-ng-insert="{{.Dbmovie.Title}}"`) {
		t.Error("music page lists movie fields")
	}

	b.Reset()

	if err := renderNamingGeneratorPage("tok", "bogus", "").Render(&b); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(b.String(), `<option value="movie" selected>`) || strings.Contains(b.String(), `hx-trigger="load"`) {
		t.Error("unknown type should fall back to movie without a check on load")
	}
}
