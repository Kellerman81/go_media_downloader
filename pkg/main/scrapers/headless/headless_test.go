package headless

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPageRequest(t *testing.T) {
	const page = "https://site.example/list?page=2"

	var got struct {
		Method, Path, ContentType string
		Body                      map[string]any
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.Method, got.Path, got.ContentType = r.Method, r.URL.Path, r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got.Body)
		io.WriteString(w, "<html></html>")
	}))
	defer srv.Close()

	req, err := PageRequest(context.Background(), srv.URL+"/", page)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if got.Method != http.MethodPost || got.Path != "/content" || got.ContentType != "application/json" {
		t.Errorf("browserless request = %s %s (%s), want POST /content (application/json)",
			got.Method, got.Path, got.ContentType)
	}

	if got.Body["url"] != page {
		t.Errorf("body url = %v, want %s", got.Body["url"], page)
	}

	req, err = PageRequest(context.Background(), "", page)
	if err != nil {
		t.Fatal(err)
	}

	if req.Method != http.MethodGet || req.URL.String() != page {
		t.Errorf("direct request = %s %s, want GET %s", req.Method, req.URL, page)
	}
}
