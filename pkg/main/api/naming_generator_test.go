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

func TestHandleNamingPreviewUnknownDataType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	form := url.Values{"template": {"{{.Title}}"}, "data_type": {"bogus"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	HandleNamingPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body %s)", w.Code, w.Body.String())
	}
}

func TestMetadataLookupPreserveQueriesCompile(t *testing.T) {
	db := openMigratedSchema(t)

	explainOK(t, db, "book author", "SELECT dbauthor_id FROM dbbooks WHERE id = ?", 1)
	explainOK(t, db, "book series", "SELECT dbbook_series_id FROM dbbooks WHERE id = ?", 1)
	explainOK(t, db, "audiobook book", "SELECT dbbook_id FROM dbaudiobooks WHERE id = ?", 1)
}
