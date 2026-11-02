package utils

import (
	"context"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// TestGetBookBestsellers_SpiegelHardcoverBelletristik exercises getbookbestsellers
// with the bestsellerliste.de Spiegel hardcover fiction chart config.
// It makes a real HTTP request — run with -v to see the scraped entries.
// Skip with -short if network is unavailable.
func TestGetBookBestsellers_SpiegelHardcoverBelletristik(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	cfgList := &config.ListsConfig{
		Name:     "spiegel-hardcover-belletristik",
		ListType: "bookbestsellers",
		Enabled:  true,
		URL:      "https://www.bestsellerliste.de/spiegel-bestseller-hardcover-belletristik/",

		ChartEntryNodeXPath: "//ol[contains(@class,'list')]/li",
		ChartTitleXPath:     ".//span[@class='title']",
		ChartArtistXPath:    ".//span[@class='author']",
	}

	list := &config.MediaListsConfig{
		Name:    cfgList.Name,
		CfgList: cfgList,
	}

	d := &feedResults{
		Books: make([]config.ManualConfig, 0, 100),
	}

	if err := d.getbookbestsellers(context.Background(), list); err != nil {
		t.Fatalf("getbookbestsellers returned error: %v", err)
	}

	if len(d.Books) == 0 {
		t.Fatal("expected at least one book entry, got 0")
	}

	t.Logf("scraped %d bestseller entries", len(d.Books))
	for i, b := range d.Books {
		t.Logf("  [%2d] title=%q  author=%q", i+1, b.Name, b.AuthorName)
	}
}

// TestGetMusicCharts_OffizielleChartsCompilations exercises getmusiccharts with
// the offiziellecharts.de compilation chart config.
// It makes a real HTTP request — run with -v to see the scraped entries.
// Skip with -short if network is unavailable.
func TestGetMusicCharts_OffizielleChartsCompilations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	cfgList := &config.ListsConfig{
		Name:     "offiziellecharts-compilations",
		ListType: "musiccharts",
		Enabled:  true,
		URL:      "https://www.offiziellecharts.de/charts/compilation/",

		ChartEntryNodeXPath: "//tr[contains(@class,'drill-down-link')]",
		ChartTitleXPath:     ".//span[@class='info-artist']",
		ChartArtistXPath:    "",
		ChartDefaultArtist:  "Various Artists",

		ChartDateURLPattern: "https://www.offiziellecharts.de/charts/compilation/for-date-{date}",
		ChartDateFormat:     "timestamp_ms",
	}

	list := &config.MediaListsConfig{
		Name:    cfgList.Name,
		CfgList: cfgList,
	}

	d := &feedResults{
		Albums: make([]config.ManualConfig, 0, 100),
	}

	if err := d.getmusiccharts(context.Background(), list); err != nil {
		t.Fatalf("getmusiccharts returned error: %v", err)
	}

	if len(d.Albums) == 0 {
		t.Fatal("expected at least one album entry, got 0")
	}

	t.Logf("scraped %d compilation chart entries", len(d.Albums))
	for i, a := range d.Albums {
		t.Logf("  [%2d] title=%q  artist=%q", i+1, a.Name, a.ArtistName)
	}
}

func TestFeedResultsSettleIRC(t *testing.T) {
	s := &ircSession{key: "cfg\x00irc-settle"}

	ircSessionsMu.Lock()
	ircSessions[s.key] = s
	ircSessionsMu.Unlock()

	t.Cleanup(func() {
		ircSessionsMu.Lock()
		delete(ircSessions, s.key)
		ircSessionsMu.Unlock()
	})

	d := &feedResults{ircKey: s.key, ircLines: []string{"l1"}}
	d.requeueIRC(false)

	if d.ircKey != "" || d.ircLines != nil {
		t.Fatal("requeueIRC did not release the lines")
	}

	if got := s.drain(); !slices.Equal(got, []string{"l1"}) {
		t.Fatalf("requeued = %v, want [l1]", got)
	}

	d = &feedResults{ircKey: s.key, ircLines: []string{"l2"}}
	d.commitIRC()

	if got := s.drain(); len(got) != 0 {
		t.Fatalf("committed lines came back: %v", got)
	}

	(&feedResults{}).requeueIRC(true) // no IRC lines: must be a no-op
}
