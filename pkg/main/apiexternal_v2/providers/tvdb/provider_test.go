package tvdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestTVDBLanguageAndV2Search(t *testing.T) {
	langs := map[string]string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		langs[r.URL.Path] = r.Header.Get("Accept-Language")

		switch r.URL.Path {
		case "/search/series":
			_, _ = w.Write([]byte(`{"data":[{"id":81189,"seriesName":"Breaking Bad","firstAired":"2008-01-20","poster":"/p.jpg"}]}`))
		case "/series/81189":
			_, _ = w.Write([]byte(`{"data":{"id":81189,"seriesName":"Breaking Bad"}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[],"links":{"next":null}}`))
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second}, "", "", "")

	res, err := p.SearchSeries(context.Background(), "breaking", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 || res[0].ID != 81189 || res[0].TVDbID != 81189 || res[0].Name != "Breaking Bad" ||
		res[0].FirstAirDate.Year() != 2008 {
		t.Errorf("unexpected results %+v", res)
	}

	if _, err := p.GetSeriesByID(context.Background(), 81189, "de"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := p.GetAllEpisodes(context.Background(), 81189, "de"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if langs["/series/81189"] != "de" || langs["/series/81189/episodes"] != "de" {
		t.Errorf("Accept-Language not sent: %v", langs)
	}

	if langs["/search/series"] != "" {
		t.Errorf("unexpected Accept-Language on search: %q", langs["/search/series"])
	}
}
