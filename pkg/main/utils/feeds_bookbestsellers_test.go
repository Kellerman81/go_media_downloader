package utils

import (
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

	if err := d.getbookbestsellers(list); err != nil {
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
