package audnex

import (
	"testing"
	"time"
)

func TestParseAudnexDate(t *testing.T) {
	want := time.Date(2021, 1, 26, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"2021-01-26T00:00:00.000Z", "2021-01-26T00:00:00Z", "2021-01-26"} {
		got, ok := parseAudnexDate(in)
		if !ok || !got.Equal(want) {
			t.Errorf("parseAudnexDate(%q) = %v, %v", in, got, ok)
		}
	}

	if _, ok := parseAudnexDate("soon"); ok {
		t.Error("garbage parsed")
	}
}

func TestConvertBookToDetailsRuntimeAndSeries(t *testing.T) {
	d := convertBookToDetails(&audnexBookResponse{
		ASIN:           "B004JVIRVK",
		Title:          "Der Zoo zieht um",
		RuntimeMinutes: 43,
		Series:         flexibleSeriesList{{Name: "Benjamin Blümchen", Position: "38"}},
	})
	if d.RuntimeMinutes != 43 || d.SeriesName != "Benjamin Blümchen" || d.SeriesPosition != "38" {
		t.Errorf("got runtime %d, series %q #%q", d.RuntimeMinutes, d.SeriesName, d.SeriesPosition)
	}
}
