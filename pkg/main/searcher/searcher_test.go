package searcher

import (
	"errors"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func TestVaAlternateName(t *testing.T) {
	tests := map[string]string{
		"VA":              "Various Artists",
		" v.a. ":          "Various Artists",
		"V.A":             "Various Artists",
		"various artists": "VA",
		"Various":         "",
		"Vangelis":        "",
	}

	for in, want := range tests {
		if got := vaAlternateName(in); got != want {
			t.Errorf("vaAlternateName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCalcPrioFromFiles(t *testing.T) {
	prios := []int{120, -5, 300, 80}
	locs := []string{"a.mkv", "b.mkv", "c.mkv", "d.mkv"}
	prio := func(i int) int { return prios[i] }
	loc := func(i int) string { return locs[i] }

	if p, old := calcPrioFromFiles(0, 200, true, prio, loc); p != 0 || old != nil {
		t.Errorf("no files: got %d %v", p, old)
	}

	// The best existing file sets the bar a new release has to beat.
	p, old := calcPrioFromFiles(len(prios), 200, true, prio, loc)
	if p != 300 {
		t.Errorf("prio = %d, want 300", p)
	}

	if want := []string{"a.mkv", "b.mkv", "d.mkv"}; !slices.Equal(old, want) {
		t.Errorf("old files = %q, want %q", old, want)
	}

	if _, old := calcPrioFromFiles(len(prios), 200, false, prio, loc); old != nil {
		t.Errorf("getold=false returned %q", old)
	}

	if _, old := calcPrioFromFiles(len(prios), -1, true, prio, loc); old != nil {
		t.Errorf("wantedprio=-1 returned %q", old)
	}

	// Negative priorities must not be clamped to zero.
	neg := []int{-10, -20}
	if p, _ := calcPrioFromFiles(2, -1, false, func(i int) int { return neg[i] }, loc); p != -10 {
		t.Errorf("negative prio = %d, want -10", p)
	}
}

func TestCalculateAudioBitratePriorityFromFile(t *testing.T) {
	tests := []struct {
		bitrate int
		format  string
		want    int
	}{
		{0, "flac", 5},
		{1411, "FLAC", 5},
		{320, "mp3", 30},
		{256, "mp3", 25},
		{192, "aac", 20},
		{160, "mp3", 15},
		{128, "mp3", 10},
		{96, "mp3", 5},
		{0, "mp3", 0},
	}

	for _, tt := range tests {
		if got := calculateAudioBitratePriorityFromFile(tt.bitrate, tt.format); got != tt.want {
			t.Errorf("calculateAudioBitratePriorityFromFile(%d, %q) = %d, want %d",
				tt.bitrate, tt.format, got, tt.want)
		}
	}
}

func TestGetsearchtype(t *testing.T) {
	tests := []struct {
		prio        int
		dont, force bool
		want        string
		wantErr     error
	}{
		{0, true, false, "missing", nil},
		{100, false, false, "upgrade", nil},
		{100, true, false, "", logger.ErrDisabled},
		{100, true, true, "upgrade", nil},
	}

	for _, tt := range tests {
		got, err := getsearchtype(tt.prio, tt.dont, tt.force)
		if got != tt.want || !errors.Is(err, tt.wantErr) {
			t.Errorf("getsearchtype(%d, %t, %t) = %q, %v", tt.prio, tt.dont, tt.force, got, err)
		}
	}
}

func TestFindIndexerConfig(t *testing.T) {
	s := newTestSearcher(config.MediaTypeMovie)
	s.indexerConfigMap = map[string]int{}
	s.Quality = &config.QualityConfig{Indexer: []config.QualityIndexerConfig{
		{TemplateIndexer: "nzbgeek"}, {TemplateIndexer: "DrunkenSlug"},
	}}

	if got := s.findIndexerConfig("drunkenslug"); got != 1 {
		t.Errorf("case-insensitive lookup = %d, want 1", got)
	}

	if got := s.findIndexerConfig("missing"); got != -1 {
		t.Errorf("missing = %d", got)
	}

	// Both hits and misses are cached.
	s.Quality.Indexer = nil

	if s.findIndexerConfig("drunkenslug") != 1 || s.findIndexerConfig("missing") != -1 {
		t.Error("lookups were not served from the cache")
	}
}

func TestSearcherReset(t *testing.T) {
	s := newTestSearcher(config.MediaTypeMovie)
	s.downloadedMap = map[uint]struct{}{1: {}}
	s.indexerConfigMap = map[string]int{"x": 1}
	s.processedURLs["u"] = struct{}{}
	s.Quality = &config.QualityConfig{Name: "HD"}
	s.searchActionType = "missing"
	s.isSeasonSearch = true
	s.Done = 1
	s.Accepted = make([]apiexternal_v2.Nzbwithprio, 2)
	s.Denied = make([]apiexternal_v2.Nzbwithprio, 3)

	s.reset()

	if s.Quality != nil || s.searchActionType != "" || s.isSeasonSearch || s.Done != 0 {
		t.Errorf("state not cleared: quality=%v action=%q season=%t done=%d", s.Quality, s.searchActionType, s.isSeasonSearch, s.Done)
	}

	if len(s.Accepted) != 0 || len(s.Denied) != 0 || cap(s.Denied) != 3 {
		t.Errorf("slices not truncated in place: %d %d cap %d", len(s.Accepted), len(s.Denied), cap(s.Denied))
	}

	if len(s.downloadedMap) != 0 || len(s.indexerConfigMap) != 0 || len(s.processedURLs) != 0 {
		t.Error("maps not cleared")
	}
}

func TestSortAcceptedByPriority(t *testing.T) {
	mk := func(title string, prio int) apiexternal_v2.Nzbwithprio {
		var e apiexternal_v2.Nzbwithprio
		e.NZB.Title = title
		e.Info.Priority = prio
		return e
	}

	accepted := []apiexternal_v2.Nzbwithprio{
		mk("low", 10),
		mk("high-a", 50),
		mk("mid", 30),
		mk("high-b", 50),
	}
	sortAcceptedByPriority(accepted)

	want := []string{"high-a", "high-b", "mid", "low"}
	for i, w := range want {
		if accepted[i].NZB.Title != w {
			t.Fatalf("index %d: got %q, want %q", i, accepted[i].NZB.Title, w)
		}
	}
}

func TestAcceptedMediaID(t *testing.T) {
	var e apiexternal_v2.Nzbwithprio
	e.NzbmovieID = 3
	e.NzbepisodeID = 4

	if got := acceptedMediaID(config.MediaTypeMovie, &e); got != 3 {
		t.Fatalf("movie id = %d", got)
	}

	if got := acceptedMediaID(config.MediaTypeSeries, &e); got != 4 {
		t.Fatalf("episode id = %d", got)
	}

	if got := acceptedMediaID(config.MediaTypeBook, &e); got != 0 {
		t.Fatalf("book id = %d", got)
	}
}
