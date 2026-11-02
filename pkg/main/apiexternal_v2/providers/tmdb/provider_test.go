package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestGetTMDBListMixedItemsAndStringID(t *testing.T) {
	tests := map[string]string{
		"string id": `"509ec17b19c2950a0600050d"`,
		"number id": `8204`,
	}

	for name, id := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id":` + id + `,"item_count":2,"page":1,"total_pages":1,"items":[
{"id":603,"media_type":"movie","title":"The Matrix","release_date":"1999-03-30"},
{"id":1396,"media_type":"tv","name":"Breaking Bad","first_air_date":"2008-01-20"}]}`))
			}))
			defer srv.Close()

			p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL + "/3", Timeout: 5 * time.Second}, "key")

			res, err := p.GetTMDBList(context.Background(), 1, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(res.Items) != 2 {
				t.Fatalf("got %d items", len(res.Items))
			}

			if res.Items[0].Title != "The Matrix" || res.Items[1].Name != "Breaking Bad" ||
				res.Items[1].FirstAirDate != "2008-01-20" || res.Items[1].MediaType != "tv" {
				t.Errorf("unexpected items %+v", res.Items)
			}
		})
	}
}
