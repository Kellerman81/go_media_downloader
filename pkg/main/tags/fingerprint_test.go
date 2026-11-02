package tags

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestBestAcoustIDMatch(t *testing.T) {
	matches := []apiexternal_v2.RecordingMatch{
		{Score: 0.3, AcoustID: "low"},
		{Score: 0.7, AcoustID: "mid"},
		{Score: 0.9, AcoustID: "best"},
	}

	if b := bestAcoustIDMatch(matches); b == nil || b.AcoustID != "best" {
		t.Fatalf("best = %+v", b)
	}

	if b := bestAcoustIDMatch(matches[:1]); b != nil {
		t.Fatalf("expected nil below threshold, got %+v", b)
	}
}
