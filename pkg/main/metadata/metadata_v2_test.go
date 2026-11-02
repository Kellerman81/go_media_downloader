package metadata

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestParseDateString(t *testing.T) {
	if got := ParseDateString("2024-02-29"); !got.Valid || !got.Time.Equal(time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("valid date = %+v", got)
	}

	for _, bad := range []string{"", "2023-02-29", "29.02.2024", "2024"} {
		if got := ParseDateString(bad); got.Valid {
			t.Errorf("ParseDateString(%q) = %+v, want invalid", bad, got)
		}
	}
}

func TestCleanGoodreadsTitle(t *testing.T) {
	for in, want := range map[string]string{
		"The Way of Kings (The Stormlight Archive, #1)": "The Way of Kings",
		"  Dune  ":   "Dune",
		"(Untitled)": "",
	} {
		if got := cleanGoodreadsTitle(in); got != want {
			t.Errorf("cleanGoodreadsTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"same", "same", 0},
		{"Dune", "dune", 1},
	}

	for _, tt := range tests {
		if got := levenshteinDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}

	// Pooled buffers must not leak state between a long and a short call.
	_ = levenshteinDistance("a much longer string here", "another long string there")
	if got := levenshteinDistance("ab", "ac"); got != 1 {
		t.Errorf("after pool reuse = %d, want 1", got)
	}
}

func TestChapterValues(t *testing.T) {
	tests := []struct {
		name              string
		ch                apiexternal_v2.AudiobookChapter
		num               int
		startMs, lengthMs int64
	}{
		{
			name:     "ms fields set",
			ch:       apiexternal_v2.AudiobookChapter{ChapterNumber: 2, Number: 9, StartOffsetMs: 1000, LengthMs: 500},
			num:      2,
			startMs:  1000,
			lengthMs: 500,
		},
		{
			name: "duration fields only",
			ch: apiexternal_v2.AudiobookChapter{
				Number:      3,
				StartOffset: 90 * time.Second,
				Duration:    2 * time.Minute,
			},
			num:      3,
			startMs:  90000,
			lengthMs: 120000,
		},
		{
			name: "first chapter at zero offset",
			ch:   apiexternal_v2.AudiobookChapter{Number: 1, Duration: time.Second},
			num:  1, startMs: 0, lengthMs: 1000,
		},
	}

	for _, tt := range tests {
		num, start, length := chapterValues(&tt.ch)
		if num != tt.num || start != tt.startMs || length != tt.lengthMs {
			t.Errorf("%s: got (%d, %d, %d), want (%d, %d, %d)",
				tt.name, num, start, length, tt.num, tt.startMs, tt.lengthMs)
		}
	}
}

func TestApplyAudiobookDetailsSeriesAndChapters(t *testing.T) {
	var ab database.Dbaudiobook
	applyAudiobookDetails(&ab, &apiexternal_v2.AudiobookDetails{
		RuntimeMinutes: 43,
		SeriesName:     "Benjamin Blümchen",
		SeriesPosition: "38",
		Chapters:       make([]apiexternal_v2.AudiobookChapter, 9),
	}, true)

	if ab.RuntimeMinutes != 43 || ab.ChapterCount != 9 ||
		ab.SeriesName != "Benjamin Blümchen" || ab.SeriesPosition != "38" {
		t.Errorf("got runtime %d, chapters %d, series %q #%q",
			ab.RuntimeMinutes, ab.ChapterCount, ab.SeriesName, ab.SeriesPosition)
	}
}

func TestUpdateHelpers(t *testing.T) {
	s := ""
	if !UpdateString(&s, "Heat", false, strings.ToUpper) || s != "HEAT" {
		t.Errorf("UpdateString fill = %q", s)
	}

	if UpdateString(&s, "Other", false, nil) || s != "HEAT" {
		t.Error("UpdateString overwrote without overwrite")
	}

	if UpdateString(&s, "", true, nil) || s != "HEAT" {
		t.Error("UpdateString cleared a value with an empty one")
	}

	if !UpdateString(&s, "Heat", true, nil) || s != "Heat" {
		t.Error("UpdateString overwrite failed")
	}

	var i64 int64
	var u uint
	var u16 uint16

	if !UpdateInt64(&i64, 5, false) || UpdateInt64(&i64, 6, false) || UpdateInt64(&i64, 0, true) || !UpdateInt64(&i64, 7, true) || i64 != 7 {
		t.Errorf("UpdateInt64 = %d", i64)
	}

	if !UpdateUint(&u, 5, false) || UpdateUint(&u, 6, false) || UpdateUint(&u, 0, true) || !UpdateUint(&u, 7, true) || u != 7 {
		t.Errorf("UpdateUint = %d", u)
	}

	if !UpdateUint16(&u16, 1999, false) || UpdateUint16(&u16, 2000, false) || UpdateUint16(&u16, 0, true) || u16 != 1999 {
		t.Errorf("UpdateUint16 = %d", u16)
	}

	// Booleans are real answers, not "missing": overwrite applies false too.
	b := false
	if UpdateBool(&b, false, false) || !UpdateBool(&b, true, false) || !b {
		t.Error("UpdateBool fill")
	}

	if UpdateBool(&b, false, false) || !b {
		t.Error("UpdateBool cleared without overwrite")
	}

	if !UpdateBool(&b, false, true) || b {
		t.Error("UpdateBool overwrite with false")
	}

	day := sql.NullTime{Time: time.Date(1995, 12, 15, 0, 0, 0, 0, time.UTC), Valid: true}
	var nt sql.NullTime

	if !UpdateNullTime(&nt, day, false) || UpdateNullTime(&nt, sql.NullTime{}, true) || !nt.Valid {
		t.Errorf("UpdateNullTime = %+v", nt)
	}
}

func TestTimeConversions(t *testing.T) {
	if TimeToNullTime(time.Time{}).Valid {
		t.Error("zero time became valid")
	}

	now := time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC)
	if nt := TimeToNullTime(now); !nt.Valid || !nt.Time.Equal(now) {
		t.Errorf("TimeToNullTime = %+v", nt)
	}

	if ExtractYearFromDate(sql.NullTime{}) != 0 || ExtractYearFromDate(sql.NullTime{Time: now, Valid: true}) != 2024 {
		t.Error("ExtractYearFromDate")
	}

	if ExtractYearFromTime(time.Time{}) != 0 || ExtractYearFromTime(now) != 2024 {
		t.Error("ExtractYearFromTime")
	}

	if GenerateSlug("") != "" || GenerateSlug("The Dark Knight") != "the-dark-knight" {
		t.Errorf("GenerateSlug = %q", GenerateSlug("The Dark Knight"))
	}

	for in, want := range map[string]string{"": "", "a": "a", "a,b,c": "a,b,c"} {
		var items []string
		if in != "" {
			items = strings.Split(in, ",")
		}

		if got := BuildCommaSeparatedString(items); got != want {
			t.Errorf("BuildCommaSeparatedString(%v) = %q, want %q", items, got, want)
		}
	}
}
