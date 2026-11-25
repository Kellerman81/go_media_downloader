package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

func TestHandleJobManagementRejectsUnknownList(t *testing.T) {
	useWantedConfig(t) // movie_EN has "EN Hallmark", movie_DE "DE Hallmark"

	form := url.Values{"job_JobType": {"feeds"}, "job_MediaConfig": {"movie_DE"}, "job_ListName": {"EN Hallmark"}}

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/jobmanagement", strings.NewReader(form.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	HandleJobManagement(ctx)

	if !strings.Contains(w.Body.String(), "Media configuration movie_DE has no list EN Hallmark") {
		t.Errorf("unknown list not rejected: %s", w.Body.String())
	}
}

func TestJobTypesFor(t *testing.T) {
	useWantedConfig(t)

	types := jobTypesFor(config.GetSettingsMedia("movie_DE"))
	for _, want := range []string{"data", "datafull", "searchmissinginc", "feeds", "checkreachedflag", "clearhistory", "refresh"} {
		if !slices.Contains(types, want) {
			t.Errorf("movie jobs lack %q: %v", want, types)
		}
	}

	if slices.Contains(types, "rssseasons") {
		t.Error("movies offered the series-only rssseasons job")
	}
}
