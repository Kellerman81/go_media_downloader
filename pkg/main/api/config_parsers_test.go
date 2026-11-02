package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newFormContext(t *testing.T, form url.Values) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request = req

	if err := c.Request.ParseForm(); err != nil {
		t.Fatal(err)
	}

	return c
}

func TestParseMediaListsCheckboxOn(t *testing.T) {
	c := newFormContext(t, url.Values{
		"media_movies_0_lists_1_Name":     {"second"},
		"media_movies_0_lists_1_Enabled":  {"on"},
		"media_movies_0_lists_0_Name":     {"first"},
		"media_movies_0_lists_0_Enabled":  {"on"},
		"media_movies_0_lists_0_Addfound": {"on"},
	})

	got := parseMediaListsConfigs(c, "movies", "0")

	if len(got) != 2 || got[0].Name != "first" || got[1].Name != "second" {
		t.Fatalf("unexpected lists/order: %+v", got)
	}

	if !got[0].Enabled || !got[0].Addfound {
		t.Errorf("checked boxes not parsed as true: %+v", got[0])
	}

	if !got[1].Enabled || got[1].Addfound {
		t.Errorf("unchecked Addfound must be false: %+v", got[1])
	}
}

func TestParseConfigFromFormNames(t *testing.T) {
	c := newFormContext(t, url.Values{
		"downloader_0_Name":   {"nzb_geek"},
		"downloader_0_DlType": {"sabnzbd"},
		"downloader_0_Port":   {"8080"},
		"downloader_1_Name":   {"second"},
	})

	got, err := parseConfigFromForm(c, "downloader", createDownloaderConfig)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 2 || got[0].Name != "nzb_geek" || got[0].Port != 8080 ||
		got[1].Name != "second" {
		t.Fatalf("unexpected result: %+v", got)
	}

	c = newFormContext(t, url.Values{"downloader_0_Name": {""}})
	if _, err := parseConfigFromForm(c, "downloader", createDownloaderConfig); err == nil {
		t.Fatal("cleared name must produce an error")
	}

	c = newFormContext(t, url.Values{
		"downloader_0_Name": {"x"},
		"downloader_0_Port": {"80a"},
	})
	if _, err := parseConfigFromForm(c, "downloader", createDownloaderConfig); err == nil {
		t.Fatal("unparseable port must produce an error")
	}
}
