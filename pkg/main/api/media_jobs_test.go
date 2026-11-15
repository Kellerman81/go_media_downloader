package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
)

func TestMediaJobQueue(t *testing.T) {
	tests := []struct {
		job  string
		want string
	}{
		{logger.StrSearchMissingInc, "Search"},
		{logger.StrSearchUpgradeFullTitle, "Search"},
		{logger.StrRss, "RSS"},
		{logger.StrRssSeasons, "RSS"},
		{logger.StrRssSeasonsAll, "RSS"},
		{logger.StrFeeds, "Feeds"},
		{"refresh", "Feeds"},
		{"refreshinc", "Feeds"},
		{logger.StrData, "Data"},
		{logger.StrClearHistory, "Data"},
		{logger.StrCheckMissing, "Data"},
		{logger.StrRssAuthors, "Data"},
	}

	for _, tt := range tests {
		if got := mediaJobQueue(tt.job); got != tt.want {
			t.Errorf("mediaJobQueue(%q) = %q, want %q", tt.job, got, tt.want)
		}
	}
}

func TestQueryFlag(t *testing.T) {
	for query, want := range map[string][2]bool{ // default false, default true
		"":                {false, true},
		"download=true":   {true, true},
		"download=1":      {true, true},
		"download=on":     {true, true},
		"download=FALSE":  {false, false},
		"download=0":      {false, false},
		"download=no":     {false, false},
		"download=banana": {false, true},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)

		if got := [2]bool{queryFlag(c, "download", false), queryFlag(c, "download", true)}; got != want {
			t.Errorf("%q: %v, want %v", query, got, want)
		}
	}
}
