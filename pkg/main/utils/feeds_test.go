package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"golang.org/x/net/html"
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

	if err := d.getbookbestsellers(context.Background(), nil, list); err != nil {
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

// TestGetBookBestsellersAudiobookConfig checks that a bestseller list under an
// audiobook config fills Audiobooks (the only feed the audiobook import reads)
// with single-title entries, so the author doesn't pull in a whole catalogue.
func TestGetBookBestsellersAudiobookConfig(t *testing.T) {
	// Trimmed from bestsellerliste.de, 2026-11-12.
	const page = `<html><body><ol class="list" data-type="Buch">` +
		`<li data-ean="9783426281765"><div class="position">1</div><div class="description">` +
		`<span class="author">Sebastian Fitzek</span> <span class="title"><cite>Der Nachtzug</cite></span></div></li>` +
		`<li data-ean="9783423285858"><div class="position">2</div><div class="description">` +
		`<span class="author">Sarah J. Maas</span> <span class="title"><cite>A Court of Splintered Harmony</cite></span></div></li>` +
		`</ol></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(page))
	}))
	defer srv.Close()

	list := &config.MediaListsConfig{
		Name: "spiegel-hardcover-belletristik",
		CfgList: &config.ListsConfig{
			ListType:            "bookbestsellers",
			URL:                 srv.URL,
			ChartEntryNodeXPath: "//ol[contains(@class,'list')]/li",
			ChartTitleXPath:     ".//span[@class='title']",
			ChartArtistXPath:    ".//span[@class='author']",
		},
	}

	want := []config.ManualConfig{
		{Name: "Der Nachtzug", AuthorName: "Sebastian Fitzek", SingleTitle: true},
		{Name: "A Court of Splintered Harmony", AuthorName: "Sarah J. Maas", SingleTitle: true},
	}

	for _, tc := range []struct {
		isType            uint
		audiobooks, books int
	}{
		{config.MediaTypeAudiobook, 2, 0},
		{config.MediaTypeBook, 0, 2},
	} {
		d := &feedResults{}
		if err := d.getbookbestsellers(context.Background(), &config.MediaTypeConfig{IsType: tc.isType}, list); err != nil {
			t.Fatalf("type %d: %v", tc.isType, err)
		}

		if len(d.Audiobooks) != tc.audiobooks || len(d.Books) != tc.books {
			t.Fatalf("type %d: got %d audiobooks, %d books; want %d, %d",
				tc.isType, len(d.Audiobooks), len(d.Books), tc.audiobooks, tc.books)
		}

		got := append(d.Audiobooks, d.Books...)
		for i := range want {
			if got[i].Name != want[i].Name || got[i].AuthorName != want[i].AuthorName || !got[i].SingleTitle {
				t.Errorf("type %d entry %d = %+v, want %+v", tc.isType, i,
					config.ManualConfig{Name: got[i].Name, AuthorName: got[i].AuthorName, SingleTitle: got[i].SingleTitle}, want[i])
			}
		}
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

func TestChartPageRequest(t *testing.T) {
	const page = "https://charts.example/album"

	req, err := chartPageRequest(context.Background(), page, "http://chrome.example:9222")
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodPost || req.URL.String() != "http://chrome.example:9222/content" {
		t.Errorf("chrome request = %s %s, want POST http://chrome.example:9222/content", req.Method, req.URL)
	}

	req, err = chartPageRequest(context.Background(), page, "")
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodGet || req.URL.String() != page || req.Header.Get("User-Agent") == "" {
		t.Errorf("direct request = %s %s UA=%q, want GET %s with a user agent",
			req.Method, req.URL, req.Header.Get("User-Agent"), page)
	}
}

func TestFetchAndParseChartPageErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "TimeoutError: Navigation timeout of 60000 ms exceeded", http.StatusBadRequest)
	}))
	defer srv.Close()

	err := fetchAndParseChartPage(context.Background(), srv.URL, func(*html.Node) {})
	if err == nil || !strings.Contains(err.Error(), "status 400") ||
		!strings.Contains(err.Error(), "Navigation timeout of 60000 ms exceeded") {
		t.Fatalf("error = %v, want status and body", err)
	}
}
