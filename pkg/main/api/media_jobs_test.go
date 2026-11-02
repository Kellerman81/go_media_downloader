package api

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
