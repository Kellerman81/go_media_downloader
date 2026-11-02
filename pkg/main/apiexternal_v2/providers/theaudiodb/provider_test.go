package theaudiodb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestSearchAlbumsIncludesAPIKeySegment(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"album":[]}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL}, "")
	if _, err := p.SearchAlbums(context.Background(), "Artist", "Album"); err != nil {
		t.Fatalf("SearchAlbums: %v", err)
	}

	if gotPath != "/123/searchalbum.php" {
		t.Fatalf("path = %q, want /123/searchalbum.php", gotPath)
	}
}
