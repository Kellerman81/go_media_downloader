package openlibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestOLKeyPath(t *testing.T) {
	tests := []struct{ id, fallback, want string }{
		{"/works/OL1W", "works", "/works/OL1W"},
		{"OL1W", "works", "/works/OL1W"},
		{"OL2M", "works", "/books/OL2M"},
		{"OL3A", "authors", "/authors/OL3A"},
		{"/authors/OL3A", "authors", "/authors/OL3A"},
		{"weird", "authors", "/authors/weird"},
	}

	for _, tt := range tests {
		if got := olKeyPath(tt.id, tt.fallback); got != tt.want {
			t.Errorf("olKeyPath(%q, %q) = %q, want %q", tt.id, tt.fallback, got, tt.want)
		}
	}
}

func TestSearchBooksEscapesQuery(t *testing.T) {
	var gotQ, gotAuthor string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQ = r.URL.Query().Get("q")
		gotAuthor = r.URL.Query().Get("author")
		_, _ = w.Write([]byte(`{"numFound":0,"docs":[]}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, RateLimitCalls: 100, RateLimitSeconds: 1})

	if _, err := p.SearchBooks(context.Background(), `Dune: Messiah (Book 2) AC/DC`, "Frank Herbert", 5); err != nil {
		t.Fatalf("SearchBooks: %v", err)
	}

	if want := `Dune\: Messiah \(Book 2\) AC\/DC`; gotQ != want {
		t.Errorf("q = %q, want %q", gotQ, want)
	}

	if gotAuthor != "Frank Herbert" {
		t.Errorf("author = %q", gotAuthor)
	}
}
