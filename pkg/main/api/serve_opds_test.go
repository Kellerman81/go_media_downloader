package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestOPDS(t *testing.T) {
	f := clientTestData(t)
	admin := basicHeader(DefaultUsername, "adminpass")

	if w := f.do(t, http.MethodGet, "/opds", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("without login: %d", w.Code)
	}

	w := f.do(t, http.MethodGet, "/opds", nil, admin)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `href="/opds/group/book_B"`) ||
		!strings.Contains(w.Body.String(), "Recently added") || strings.ContainsRune(w.Body.String(), trOpen) {
		t.Fatalf("root: %d %s", w.Code, w.Body)
	}

	w = f.do(t, http.MethodGet, "/opds/group/book_B", nil, admin)
	body := w.Body.String()

	for _, want := range []string{"<title>Novel</title>", "<name>Writer</name>", `href="/opds/file/51/novel.epub"`,
		`type="application/epub+zip"`, `href="/opds/cover/1"`, "A story", "<opensearch:totalResults>1<"} {
		if !strings.Contains(body, want) {
			t.Errorf("group feed lacks %s: %s", want, body)
		}
	}

	if w := f.do(t, http.MethodGet, "/opds/search?q=nov", nil, admin); !strings.Contains(w.Body.String(), "<title>Novel</title>") {
		t.Errorf("search: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/opds/search?q=nothing", nil, admin); strings.Contains(w.Body.String(), "<entry>") {
		t.Errorf("search without hits: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/opds/authors", nil, admin); !strings.Contains(w.Body.String(), "<title>Writer</title>") {
		t.Errorf("authors: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/opds/file/51/novel.epub", nil, admin); w.Code != http.StatusOK || w.Body.String() != "data of novel.epub" {
		t.Fatalf("download: %d %q", w.Code, w.Body)
	}

	// anna has no book group.
	anna := basicHeader("anna", "secret12")
	if w := f.do(t, http.MethodGet, "/opds/file/51/novel.epub", nil, anna); w.Code != http.StatusNotFound {
		t.Fatalf("anna downloads: %d", w.Code)
	}

	if w := f.do(t, http.MethodGet, "/opds/new", nil, anna); strings.Contains(w.Body.String(), "Novel") {
		t.Fatalf("anna sees the book: %s", w.Body)
	}
}
