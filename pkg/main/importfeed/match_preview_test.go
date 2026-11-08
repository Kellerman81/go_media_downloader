package importfeed

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

func TestPreviewTrackRows(t *testing.T) {
	local := []parser_v2.TrackInfo{
		{Filename: "01.mp3", Title: "Intro", TrackNumber: 1, RuntimeMS: 61000},
		{Filename: "02.mp3", Title: "Song", TrackNumber: 2, RuntimeMS: 200000},
		{Filename: "99.mp3", Title: "Bonus", TrackNumber: 9, RuntimeMS: 999000},
	}

	dbTrack := func(title string, num uint16, ms int64) database.DbtrackWithArtist {
		var d database.DbtrackWithArtist
		d.Title = title
		d.TrackNumber = num
		d.DiscNumber = 1
		d.RuntimeMs = ms

		return d
	}

	db := []database.DbtrackWithArtist{
		dbTrack("Intro", 1, 60000),
		dbTrack("Song", 2, 201000),
		dbTrack("Outro", 3, 90000),
	}

	rows, missing, extra := previewTrackRows(local, db, false, false, &config.MediaDataConfig{})
	if missing != 1 || extra != 1 {
		t.Fatalf("missing=%d extra=%d; want 1 and 1", missing, extra)
	}

	if len(rows) != 4 {
		t.Fatalf("got %d rows; want 4 (2 pairs, 1 missing, 1 extra)", len(rows))
	}

	for i, want := range []struct {
		local, wanted    string
		missing, isExtra bool
	}{
		{"01.mp3", "Intro", false, false},
		{"02.mp3", "Song", false, false},
		{"", "Outro", true, false},
		{"99.mp3", "", false, true},
	} {
		r := rows[i]
		if r.LocalFile != want.local || r.WantedTitle != want.wanted ||
			r.Missing != want.missing || r.Extra != want.isExtra {
			t.Errorf("row %d = %+v; want %+v", i, r, want)
		}
	}

	if rows[0].Dist >= 0.5 {
		t.Errorf("identical track scored distance %.2f", rows[0].Dist)
	}
}

func TestFinishPreviewEntrySortsAndCaps(t *testing.T) {
	e := &PreviewEntry{}
	for i := range previewMaxCandidates + 3 {
		e.Candidates = append(e.Candidates, &PreviewCandidate{Dist: 1 - float64(i)/100})
	}

	finishPreviewEntry(e)

	if len(e.Candidates) != previewMaxCandidates {
		t.Fatalf("kept %d candidates; want %d", len(e.Candidates), previewMaxCandidates)
	}

	for i := 1; i < len(e.Candidates); i++ {
		if e.Candidates[i-1].Dist > e.Candidates[i].Dist {
			t.Fatalf("candidates not sorted by distance at %d", i)
		}
	}

	empty := &PreviewEntry{}
	finishPreviewEntry(empty)

	if empty.Err != "no_match" {
		t.Errorf("entry without candidates: Err = %q; want no_match", empty.Err)
	}
}
