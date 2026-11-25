package tvdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

// TestEpisodeTranslations: texts TVDB only has in English (its "language"
// map says so) are left out; a series in another language than asked for
// gives no translation.
func TestEpisodeTranslations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/series/1":
			_, _ = w.Write([]byte(`{"data":{"id":1,"seriesName":"Die Serie","overview":"Deutsch.","language":"de"}}`))
		case "/series/2":
			_, _ = w.Write([]byte(`{"data":{"id":2,"seriesName":"The Show","overview":"English.","language":"en"}}`))
		case "/series/1/episodes":
			_, _ = w.Write([]byte(`{"links":{"next":null},"data":[
				{"airedSeason":1,"airedEpisodeNumber":1,"episodeName":"Pilot DE","overview":"Erste.","language":{"episodeName":"de","overview":"de"}},
				{"airedSeason":1,"airedEpisodeNumber":2,"episodeName":"Zweite","overview":"Only English.","language":{"episodeName":"de","overview":"en"}},
				{"airedSeason":1,"airedEpisodeNumber":3,"episodeName":"Third","overview":"English.","language":{"episodeName":"en","overview":"en"}}]}`))
		}
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, Timeout: 5 * time.Second}, "", "", "")

	eps, err := p.GetEpisodeTranslations(context.Background(), 1, "de")
	if err != nil {
		t.Fatal(err)
	}

	if len(eps) != 2 || eps[0].Title != "Pilot DE" || eps[0].Overview != "Erste." || eps[1].Title != "Zweite" || eps[1].Overview != "" {
		t.Errorf("episodes %+v", eps)
	}

	if tr, err := p.GetSeriesTranslation(context.Background(), 1, "de"); err != nil || tr.Title != "Die Serie" {
		t.Errorf("series %+v %v", tr, err)
	}

	if tr, err := p.GetSeriesTranslation(context.Background(), 2, "de"); err != nil || tr.Title != "" || tr.Overview != "" {
		t.Errorf("English fallback taken as German: %+v %v", tr, err)
	}
}
