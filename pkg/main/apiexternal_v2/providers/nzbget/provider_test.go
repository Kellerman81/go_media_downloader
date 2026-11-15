package nzbget

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestNzbgetPriority(t *testing.T) {
	for in, want := range map[int]int{-2: -100, -1: -50, 0: 0, 1: 50, 2: 100, 900: 900, -100: -100} {
		if got := nzbgetPriority(in); got != want {
			t.Errorf("nzbgetPriority(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestEditQueueFalseIsError(t *testing.T) {
	var gotPriority float64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/a.nzb" {
			_, _ = w.Write([]byte("<nzb/>"))
			return
		}

		var req struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		switch req.Method {
		case "editqueue":
			_, _ = w.Write([]byte(`{"result":false}`))
		case "append":
			gotPriority, _ = req.Params[3].(float64)
			_, _ = w.Write([]byte(`{"result":7}`))
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p, err := NewProvider(u.Hostname(), port, "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}

	if err := p.PauseTorrent(context.Background(), "42"); err == nil {
		t.Error("PauseTorrent: expected error for result false")
	}

	if err := p.RemoveTorrent(context.Background(), "42", false); err == nil {
		t.Error("RemoveTorrent: expected error for result false")
	}

	id, err := p.AddNZBExtended(context.Background(), srv.URL+"/a.nzb", "", "n", 1, false)
	if err != nil || id != 7 {
		t.Fatalf("AddNZBExtended = %d, %v", id, err)
	}

	if gotPriority != 50 {
		t.Errorf("append priority = %v, want 50", gotPriority)
	}
}

func TestNzbgetCategoryDir(t *testing.T) {
	opts := map[string]string{
		"maindir": "/downloads", "destdir": "${MainDir}/completed",
		"category1.name": "Movies", "category1.destdir": "${DestDir}/Film",
		"category2.name": "Series", "category2.destdir": "",
	}

	for _, tc := range []struct{ category, want string }{
		{"movies", "/downloads/completed/Film"},
		{"Series", "/downloads/completed/Series"}, // AppendCategoryDir is on by default
		{"", "/downloads/completed"},
	} {
		if got := nzbgetCategoryDir(opts, tc.category); got != tc.want {
			t.Errorf("%q: got %q, want %q", tc.category, got, tc.want)
		}
	}

	opts["appendcategorydir"] = "no"
	if got := nzbgetCategoryDir(opts, "Series"); got != "/downloads/completed" {
		t.Errorf("without AppendCategoryDir: %q", got)
	}
}
