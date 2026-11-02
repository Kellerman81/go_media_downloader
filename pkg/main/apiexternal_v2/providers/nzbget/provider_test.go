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
