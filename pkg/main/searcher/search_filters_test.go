package searcher

import (
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies"
)

func useTestDB(t *testing.T) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)
}

func release(title string, size int64) *apiexternal_v2.Nzbwithprio {
	e := &apiexternal_v2.Nzbwithprio{}
	e.NZB.Title = title
	e.NZB.Size = size

	return e
}

func TestValidateSize(t *testing.T) {
	ind := &config.IndexersConfig{Name: "geek"}
	s := newTestSearcher(config.MediaTypeMovie)
	s.Quality = &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "geek", SkipEmptySize: true}}}
	s.Cfgp.DataMap = map[int]*config.MediaDataConfig{0: {CfgPath: &config.PathsConfig{
		MinSize: 1, MinSizeByte: 500 << 20, MaxSize: 1, MaxSizeByte: 20 << 30,
	}}}

	cases := []struct {
		size   int64
		reject bool
		reason string
	}{
		{0, true, "no size"},
		{100 << 20, true, "too small"},
		{30 << 30, true, "too big"},
		{4 << 30, false, ""},
	}

	for _, c := range cases {
		e := release("Heat.1995.1080p", c.size)
		e.NZB.Indexer = ind

		if got := s.validateSize(e); got != c.reject || e.Reason != c.reason {
			t.Errorf("size %d: reject=%t reason=%q, want %t %q", c.size, got, e.Reason, c.reject, c.reason)
		}
	}

	// Without an indexer nothing is checked; without SkipEmptySize an unknown size passes.
	if s.validateSize(release("x", 0)) {
		t.Error("entry without indexer rejected")
	}

	s.Quality.Indexer[0].SkipEmptySize = false

	e := release("x", 0)
	e.NZB.Indexer = ind

	if s.validateSize(e) {
		t.Error("unknown size rejected without SkipEmptySize")
	}
}

func TestFilterRegexNzbs(t *testing.T) {
	useTestDB(t) // also sets up the regex cache
	ind := &config.IndexersConfig{Name: "geek"}
	rgx := &config.RegexConfig{Required: []string{`(?i)1080p|2160p`}, RequiredLen: 1, Rejected: []string{`(?i)\bcam\b`, `(?i)german`}}
	qual := &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "geek", CfgRegex: rgx}}}
	s := newTestSearcher(config.MediaTypeMovie)

	cases := []struct {
		title, wanted string
		reject        bool
	}{
		{"Heat.1995.1080p.BluRay", "Heat", false},
		{"Heat.1995.720p.BluRay", "Heat", true},         // required regex not matched
		{"Heat.1995.1080p.CAM", "Heat", true},           // rejected
		{"German.Psycho.1080p", "German Psycho", false}, // rejected word is part of the wanted title
	}

	for _, c := range cases {
		e := release(c.title, 0)
		e.NZB.Indexer = ind
		e.WantedTitle = c.wanted

		if got := s.filterRegexNzbs(e, qual); got != c.reject {
			t.Errorf("%q: reject = %t, want %t (%s)", c.title, got, c.reject, e.Reason)
		}
	}

	// An indexer the quality profile does not list has no regex: rejected.
	e := release("Heat.1995.1080p", 0)
	e.NZB.Indexer = &config.IndexersConfig{Name: "other"}

	if !s.filterRegexNzbs(e, qual) {
		t.Error("release from an indexer outside the quality profile accepted")
	}
}

func TestFilterTestQualityWanted(t *testing.T) {
	q := &config.QualityConfig{
		WantedResolution: []string{"1080p"}, WantedResolutionLen: 1,
		WantedQuality: []string{"BluRay"}, WantedQualityLen: 1,
		WantedAudioFormats: []string{"flac"}, WantedAudioFormatsLen: 1,
	}
	s := newTestSearcher(config.MediaTypeMovie)

	mk := func(res, qual, format string) *apiexternal_v2.Nzbwithprio {
		e := release("x", 0)
		e.Info.Resolution, e.Info.Quality, e.Info.AudioFormat = res, qual, format

		return e
	}

	for _, c := range []struct {
		e      *apiexternal_v2.Nzbwithprio
		reason string
	}{
		{mk("1080P", "bluray", ""), ""},
		{mk("720p", "BluRay", ""), "unwanted Resolution"},
		{mk("1080p", "WEBDL", ""), "unwanted Quality"},
		{mk("", "", "mp3"), "unwanted AudioFormat"},
		{mk("", "", ""), ""}, // unknown values are not rejected
	} {
		if got := s.filterTestQualityWanted(c.e, q); got != (c.reason != "") || c.e.Reason != c.reason {
			t.Errorf("%+v: reject=%t reason=%q, want %q", c.e.Info, got, c.e.Reason, c.reason)
		}
	}

	if s.filterTestQualityWanted(mk("480p", "", ""), nil) {
		t.Error("nil quality rejected")
	}
}

func TestCheckprocessed(t *testing.T) {
	s := newTestSearcher(config.MediaTypeMovie)
	s.processedURLs["http://a/1"] = struct{}{}
	s.processedTitles["Heat.1995.1080p"] = struct{}{}
	s.processedNorm[normalizedTitleKey("Dune.2021.2160p", 15<<30)] = struct{}{}

	for _, c := range []struct {
		url, title string
		size       int64
		want       bool
	}{
		{"http://a/1", "other", 0, true},
		{"", "Heat.1995.1080p", 0, true},
		{"http://b/2", " dune.2021.2160p ", 15<<30 + 1<<20, true}, // same release from another indexer
		{"http://b/3", "Dune.2021.2160p", 16 << 30, false},        // different size bucket
		{"http://c/4", "New.Release", 0, false},
	} {
		n := apiexternal_v2.Nzb{DownloadURL: c.url, Title: c.title, Size: c.size}
		if got := s.checkprocessed(&n); got != c.want {
			t.Errorf("checkprocessed(%q, %q) = %t, want %t", c.url, c.title, got, c.want)
		}
	}
}

func TestCheckyear(t *testing.T) {
	s := newTestSearcher(config.MediaTypeMovie)
	src := release("", 0)
	src.Info.Year = 1995

	strict := &config.QualityConfig{CheckYear: true}
	loose := &config.QualityConfig{CheckYear1: true}

	for _, c := range []struct {
		title  string
		qual   *config.QualityConfig
		reject bool
	}{
		{"Heat.1995.1080p", strict, false},
		{"Heat.1996.1080p", strict, true},
		{"Heat.1996.1080p", loose, false},
		{"Heat.1998.1080p", loose, true},
		{"Heat.1998.1080p", &config.QualityConfig{}, false}, // year check disabled
	} {
		if got := s.checkyear(src, release(c.title, 0), c.qual); got != c.reject {
			t.Errorf("%q: reject = %t, want %t", c.title, got, c.reject)
		}
	}

	noYear := release("", 0)
	if !s.checkyear(noYear, release("Heat.1995", 0), strict) {
		t.Error("wanted item without a year accepted under CheckYear")
	}

	s.searchActionType = "rss"
	if s.checkyear(src, release("Heat.2001", 0), strict) {
		t.Error("RSS search applied the year check")
	}

	audiobooks := newTestSearcher(config.MediaTypeAudiobook)
	if audiobooks.checkyear(src, release("Mythos.2001", 0), strict) {
		t.Error("year check applied to audiobooks")
	}
}

func TestCheckhistoryAndRuntimeSize(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into movie_histories (title, url) values ('Heat.1995.1080p-GRP', 'http://a/1')"); err != nil {
		t.Fatal(err)
	}

	if err := database.ExecNErr("insert into dbaudiobooks (id, title, runtime_minutes) values (1, 'Mythos', 900), (2, 'Short', 20)"); err != nil {
		t.Fatal(err)
	}

	ind := &config.IndexersConfig{Name: "geek"}
	checkTitle := &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "geek", HistoryCheckTitle: true}}}
	urlOnly := &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "geek"}}}
	s := newTestSearcher(config.MediaTypeMovie)

	mk := func(url, title string) *apiexternal_v2.Nzbwithprio {
		e := release(title, 0)
		e.NZB.DownloadURL = url
		e.NZB.Indexer = ind

		return e
	}

	if e := mk("http://a/1", "x"); !s.checkhistory(e, urlOnly) || e.Reason != "already downloaded url" {
		t.Errorf("known URL: %q", e.Reason)
	}

	if s.checkhistory(mk("http://b/2", "Heat.1995.1080p-GRP"), urlOnly) {
		t.Error("title checked although HistoryCheckTitle is off")
	}

	if e := mk("http://b/2", "Heat.1995.1080p-GRP"); !s.checkhistory(e, checkTitle) || e.Reason != "already downloaded title" {
		t.Errorf("known title: %q", e.Reason)
	}

	if s.checkhistory(mk("http://c/3", "New.Release"), checkTitle) {
		t.Error("new release rejected")
	}

	ab := newTestSearcher(config.MediaTypeAudiobook)

	for _, c := range []struct {
		id     uint
		size   int64
		reject bool
	}{
		{1, 20 << 20, true},   // 20 MiB for a 15 h book
		{1, 400 << 20, false}, // ~27 MiB/h
		{2, 1 << 20, false},   // runtimes under 30 min are not checked
		{0, 1 << 20, false},   // unmatched book
	} {
		e := release("Mythos", c.size)
		e.Info.DbaudiobookID = c.id

		if got := ab.checkruntimesize(e); got != c.reject {
			t.Errorf("book %d, %d bytes: reject = %t, want %t", c.id, c.size, got, c.reject)
		}
	}

	if s.checkruntimesize(release("Heat", 1)) {
		t.Error("runtime size check applied to movies")
	}
}

// TestValidateEntryPriorityRules runs releases through the whole validateEntry
// chain with a permissive profile, so only the priority rules decide.
func TestValidateEntryPriorityRules(t *testing.T) {
	useTestDB(t)

	ind := &config.IndexersConfig{Name: "geek"}
	qual := &config.QualityConfig{
		Name:                        "HD",
		UseForPriorityMinDifference: 10,
		Indexer:                     []config.QualityIndexerConfig{{TemplateIndexer: "geek", CfgRegex: &config.RegexConfig{}}},
	}

	cases := []struct {
		name       string
		prio, min  int
		dontSearch bool
		reason     string
	}{
		{"missing item", 100, 0, false, ""},
		{"missing item, search disabled", 100, 0, true, "disabled Search"},
		{"same priority as existing file", 100, 100, false, "same Prio"},
		{"upgrade below the minimum difference", 105, 100, false, "lower Prio"},
		{"worse than existing file", 80, 100, false, "lower Prio"},
		{"real upgrade", 120, 100, false, ""},
	}

	for _, c := range cases {
		s := newTestSearcher(config.MediaTypeMovie)
		e := release("Heat.1995.1080p.BluRay-GRP", 0)
		e.NZB.Indexer = ind
		e.NZB.DownloadURL = "http://new/" + c.name
		e.Info.Priority = c.prio
		e.MinimumPriority = c.min
		e.DontSearch = c.dontSearch

		rejected := s.validateEntry(nil, e, qual)
		if rejected != (c.reason != "") || e.Reason != c.reason {
			t.Errorf("%s: rejected=%t reason=%q, want %q", c.name, rejected, e.Reason, c.reason)
		}

		if rejected && len(s.Denied) != 1 {
			t.Errorf("%s: rejection not recorded in Denied (%d)", c.name, len(s.Denied))
		}
	}
}

func TestAudioFormatFromCategory(t *testing.T) {
	tests := map[string]string{"3010": "mp3", "3040": "flac", "3000": "", "": "", "2040": ""}
	for in, want := range tests {
		if got := audioFormatFromCategory(in); got != want {
			t.Errorf("audioFormatFromCategory(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolutionFromCategory(t *testing.T) {
	withUHD := []string{"2000", "2040", "2045"}
	withoutUHD := []string{"2000", "2040"}

	tests := []struct {
		cat       string
		supported []string
		want      string
	}{
		{"2045", nil, "2160p"},
		{"5045", nil, "2160p"},
		{"2030", nil, "480p"},
		{"5030", nil, "480p"},
		// HD only means 1080p when the indexer separates out UHD.
		{"2040", withUHD, "1080p"},
		{"5040", []string{"5045"}, "1080p"},
		{"2040", withoutUHD, ""},
		{"2040", nil, ""},
		{"2000", withUHD, ""},
	}

	for _, tt := range tests {
		if got := resolutionFromCategory(tt.cat, tt.supported); got != tt.want {
			t.Errorf("resolutionFromCategory(%q, %v) = %q, want %q", tt.cat, tt.supported, got, tt.want)
		}
	}
}

func TestTitleToWordSet(t *testing.T) {
	got := titleToWordSet("  The Dark  Knight the ")
	want := []string{"dark", "knight", "the", "the"}

	if !slices.Equal(got, want) {
		t.Errorf("titleToWordSet = %q, want %q", got, want)
	}
}

func TestSortedIsSubset(t *testing.T) {
	tests := []struct {
		sub, super []string
		want       bool
	}{
		{nil, nil, true},
		{nil, []string{"a"}, true},
		{[]string{"a"}, nil, false},
		{[]string{"a", "c"}, []string{"a", "b", "c"}, true},
		{[]string{"a", "d"}, []string{"a", "b", "c"}, false},
		{[]string{"b"}, []string{"a"}, false},
		{[]string{"a", "a"}, []string{"a"}, true},
	}

	for _, tt := range tests {
		if got := sortedIsSubset(tt.sub, tt.super); got != tt.want {
			t.Errorf("sortedIsSubset(%q, %q) = %t, want %t", tt.sub, tt.super, got, tt.want)
		}
	}
}

func TestIsWordSetRedundant(t *testing.T) {
	searched := [][]string{titleToWordSet("Star Wars Episode IV")}

	tests := []struct {
		title string
		want  bool
	}{
		{"Star Wars", true},
		{"episode iv star wars", true},
		{"Star Wars Episode IV A New Hope", true},
		{"Star Trek", false},
	}

	for _, tt := range tests {
		if got := isWordSetRedundant(titleToWordSet(tt.title), searched); got != tt.want {
			t.Errorf("isWordSetRedundant(%q) = %t, want %t", tt.title, got, tt.want)
		}
	}

	if isWordSetRedundant(titleToWordSet("anything"), nil) {
		t.Error("nothing searched yet must not be redundant")
	}
}

func TestCheckdownloaded(t *testing.T) {
	s := newTestSearcher(config.MediaTypeMovie)
	s.downloadedMap = map[uint]struct{}{5: {}, 9: {}}

	mk := func(movie, episode uint) apiexternal_v2.Nzbwithprio {
		var e apiexternal_v2.Nzbwithprio
		e.NzbmovieID = movie
		e.NzbepisodeID = episode
		return e
	}

	s.Accepted = []apiexternal_v2.Nzbwithprio{mk(5, 0), mk(0, 9), mk(6, 0), mk(0, 0)}

	for idx, want := range []bool{true, true, false, false} {
		if got := s.checkdownloaded(idx); got != want {
			t.Errorf("checkdownloaded(%d) = %t, want %t", idx, got, want)
		}
	}
}

func newTestSearcher(isType uint) *ConfigSearcher {
	return &ConfigSearcher{
		Cfgp:            &config.MediaTypeConfig{IsType: isType},
		processedURLs:   map[string]struct{}{},
		processedTitles: map[string]struct{}{},
		processedNorm:   map[string]struct{}{},
	}
}

func TestCheckYear(t *testing.T) {
	tests := []struct {
		name       string
		checkYear  bool
		checkYear1 bool
		wantedYear uint16
		title      string
		wantSkip   bool
	}{
		{"no check configured, yearless wanted", false, false, 0, "Movie.1080p", false},
		{"no check configured, other year", false, false, 2020, "Movie.1999.1080p", false},
		{"check, no wanted year", true, false, 0, "Movie.2020.1080p", true},
		{"check, exact year", true, false, 2020, "Movie.2020.1080p", false},
		{"check, wrong year", true, false, 2020, "Movie.2021.1080p", true},
		{"check1, year +1", false, true, 2020, "Movie.2021.1080p", false},
		{"check1, year +2", false, true, 2020, "Movie.2022.1080p", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSearcher(config.MediaTypeMovie)
			qual := &config.QualityConfig{CheckYear: tt.checkYear, CheckYear1: tt.checkYear1}

			var source, entry apiexternal_v2.Nzbwithprio
			source.Info.Year = tt.wantedYear
			entry.NZB.Title = tt.title

			if got := s.checkyear(&source, &entry, qual); got != tt.wantSkip {
				t.Fatalf("checkyear() = %v, want %v", got, tt.wantSkip)
			}
		})
	}
}

func TestCheckEpisodeFormatUsesRelease(t *testing.T) {
	tests := []struct {
		name      string
		season    string
		episode   string
		releaseID string
		wantSkip  bool
	}{
		{"same episode unpadded wanted", "1", "5", "S01E05", false},
		{"same episode padded wanted", "01", "05", "S01E05", false},
		{"multi-episode pack", "1", "5", "S01E05-E06", false},
		{"different episode", "1", "5", "S01E06", true},
		{"different episode same digit", "1", "5", "S01E15", true},
		{"season 1 vs season 10", "1", "5", "S10E05", true},
		{"different season", "1", "5", "S02E05", true},
		{"release without identifier", "1", "5", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSearcher(config.MediaTypeSeries)

			var source, entry apiexternal_v2.Nzbwithprio
			source.NZB.Season = tt.season
			source.NZB.Episode = tt.episode
			source.Info.Identifier = "S01E05"
			entry.NZB.Title = "Show.Name." + tt.releaseID + ".1080p"
			entry.Info.Identifier = tt.releaseID

			if got := s.checkEpisodeFormat(&source, &entry, entry.Info.Identifier); got != tt.wantSkip {
				t.Fatalf("checkEpisodeFormat(%q) = %v, want %v", tt.releaseID, got, tt.wantSkip)
			}
		})
	}
}

func TestCheckSingleTrack(t *testing.T) {
	tests := []struct {
		isType uint
		title  string
		want   bool
	}{
		{config.MediaTypeMusic, `Sigur Ros - Live 2013-09-21 - [06/17] - "06-SigurRos-Stormur.flac"`, true},
		{config.MediaTypeMusic, "Horner, James - The Mask Of Zorro - 05 - The Fencing Lesson.flac", true},
		{config.MediaTypeMusic, `The Platters-The Very Best Of The Platters(2000)(CD2 of 2)-2-12 Remember When.mp3"`, true},
		{config.MediaTypeMusic, "VA-Bravo.Hits.Vol.130.2CD-2025", false},
		{config.MediaTypeMusic, "Artist-Album-WEB-FLAC-2026-GRP", false},
		{config.MediaTypeAudiobook, "Dan Brown - The Lost Symbol - 002 of 135 01-02.mp3", true},
		{config.MediaTypeAudiobook, "Dan Brown - The Lost Symbol.m4b", false},
		{config.MediaTypeMovie, "Some.Movie.2020.mp3", false},
	}

	for _, tt := range tests {
		s := newTestSearcher(tt.isType)
		entry := apiexternal_v2.Nzbwithprio{}
		entry.NZB.Title = tt.title

		if got := s.checksingletrack(&entry); got != tt.want {
			t.Errorf("checksingletrack(%q) = %v, want %v", tt.title, got, tt.want)
		}
	}
}
