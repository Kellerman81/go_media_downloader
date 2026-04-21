package utils

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// TestGetMusicCharts_OffizielleChartsCompilations exercises getmusiccharts with
// the offiziellecharts.de compilation chart config.
// It makes a real HTTP request — run with -v to see the scraped entries.
// Skip with -short if network is unavailable.
func TestGetMusicCharts_OffizielleChartsCompilations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	cfgList := &config.ListsConfig{
		Name:    "offiziellecharts-compilations",
		ListType: "musiccharts",
		Enabled: true,
		URL:     "https://www.offiziellecharts.de/charts/compilation/",

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

	if err := d.getmusiccharts(list); err != nil {
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
