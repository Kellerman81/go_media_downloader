package api

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestComputeViewStats: overlapping plays count as concurrent, back to back
// ones don't; titles are ranked by plays, modes counted, starts placed by
// weekday and hour.
func TestComputeViewStats(t *testing.T) {
	s := func(title, mode, start, end string, secs float64) database.PlaySession {
		return database.PlaySession{Title: title, Mode: mode, StartedAt: start, EndedAt: end, Seconds: secs}
	}

	v := computeViewStats([]database.PlaySession{
		s("A", "direct", "2026-11-16 20:00:00", "2026-11-16 21:00:00", 3600), // Monday
		s("B", "transcode", "2026-11-16 20:30:00", "2026-11-16 20:45:00", 900),
		s("A", "direct", "2026-11-16 20:40:00", "2026-11-16 20:50:00", 600),
		s("C", "", "2026-11-16 21:00:00", "2026-11-16 21:30:00", 1800), // starts as A ends
	})

	if v.Peak != 3 || v.PeakAt != "2026-11-16 20:40" {
		t.Errorf("peak %d at %q, want 3 at 20:40", v.Peak, v.PeakAt)
	}

	if len(v.Top) != 3 || v.Top[0].Title != "A" || v.Top[0].Plays != 2 || v.Top[0].Hours != 7.0/6 {
		t.Errorf("top: %+v", v.Top)
	}

	if v.Modes["direct"] != 2 || v.Modes["transcode"] != 1 || v.Modes["unknown"] != 1 {
		t.Errorf("modes: %v", v.Modes)
	}

	if v.Heat[1][20] != 3 || v.Heat[1][21] != 1 {
		t.Errorf("heat Monday 20h %d, 21h %d", v.Heat[1][20], v.Heat[1][21])
	}
}
