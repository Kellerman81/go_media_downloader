package audible

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestSearchByASINFetchesChaptersSeparately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/1.0/catalog/products/B1":
			if strings.Contains(r.URL.RawQuery, "chapter_info") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"Invalid response group(s) requested: chapter_info"}`))

				return
			}

			_, _ = w.Write([]byte(`{"product":{"asin":"B1","title":"Book"}}`))
		case "/1.0/content/B1/metadata":
			_, _ = w.Write([]byte(`{"content_metadata":{"chapter_info":{"chapters":[{"length_ms":1000,"start_offset_ms":0,"title":"One"},{"length_ms":2000,"start_offset_ms":1000,"title":"Two"}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL}, RegionUS)

	d, err := p.SearchByASIN(context.Background(), "B1")
	if err != nil {
		t.Fatalf("SearchByASIN: %v", err)
	}

	if d.Title != "Book" || len(d.Chapters) != 2 || d.Chapters[1].StartOffsetMs != 1000 {
		t.Errorf("details = %+v", d)
	}
}
