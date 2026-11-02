package importfeed

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestPickReleaseIDForGroup(t *testing.T) {
	rels := []apiexternal_v2.ReleaseSearchResult{
		{MusicBrainzID: "other-group", ReleaseGroupID: "rg2", Status: "Official"},
		{MusicBrainzID: "bootleg", ReleaseGroupID: "rg1", Status: "Bootleg"},
		{MusicBrainzID: "official", ReleaseGroupID: "rg1", Status: "Official"},
	}

	if got := pickReleaseIDForGroup(rels, "rg1"); got != "official" {
		t.Errorf("got %q, want official", got)
	}

	if got := pickReleaseIDForGroup(rels[:2], "rg1"); got != "bootleg" {
		t.Errorf("fallback got %q, want bootleg", got)
	}

	if got := pickReleaseIDForGroup(rels[:1], "rg1"); got != "" {
		t.Errorf("foreign group got %q, want empty", got)
	}
}
