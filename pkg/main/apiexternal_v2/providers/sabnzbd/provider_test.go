package sabnzbd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func newTestProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p, err := NewProvider(u.Hostname(), port, "key", false, "")
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestAddNZBPriority(t *testing.T) {
	for prio, want := range map[int]string{-2: "-2", -1: "-1", 0: "", 1: "1", 2: "2", 5: ""} {
		var got *string

		p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			if v, ok := r.URL.Query()["priority"]; ok {
				got = &v[0]
			}

			_, _ = w.Write([]byte(`{"status":true,"nzo_ids":["SABnzbd_nzo_1"]}`))
		})

		if err := p.AddNZB(context.Background(), "http://x/a.nzb", "", prio); err != nil {
			t.Fatalf("priority %d: %v", prio, err)
		}

		switch {
		case want == "" && got != nil:
			t.Errorf("priority %d: sent %q, want omitted", prio, *got)
		case want != "" && (got == nil || *got != want):
			t.Errorf("priority %d: sent %v, want %q", prio, got, want)
		}
	}
}

func TestTestConnectionRejectsBadKey(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("mode") != "queue" {
			t.Errorf("mode = %q, want queue", r.URL.Query().Get("mode"))
		}

		_, _ = w.Write([]byte(`{"status":false,"error":"API Key Incorrect"}`))
	})

	if err := p.TestConnection(context.Background()); err == nil {
		t.Fatal("expected error for incorrect API key")
	}
}

func TestTestConnectionOK(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"queue":{"version":"4.3.2","slots":[]}}`))
	})

	status, err := p.GetStatus(context.Background())
	if err != nil || !status.Connected || status.Version != "4.3.2" {
		t.Fatalf("status = %+v, %v", status, err)
	}
}

func TestQueuePriorityNumberOrString(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"queue":{"slots":[
			{"nzo_id":"a","filename":"A","priority":1},
			{"nzo_id":"b","filename":"B","priority":"-1"},
			{"nzo_id":"c","filename":"C","priority":"Normal"}]}}`))
	})

	list, err := p.ListTorrents(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]int{"a": 1, "b": -1, "c": 0}
	for _, it := range list.Torrents {
		if it.Priority != want[it.Hash] {
			t.Errorf("%s: priority %d, want %d", it.Hash, it.Priority, want[it.Hash])
		}
	}

	if len(list.Torrents) != 3 {
		t.Fatalf("got %d slots, want 3", len(list.Torrents))
	}
}

func TestParseTimeLeft(t *testing.T) {
	p := &Provider{}
	for in, want := range map[string]int{"0:00:00": 0, "1:02:03": 3723, "26:00:01": 93601, "05:30": 0, "": 0} {
		if got := p.parseTimeLeft(in); got != want {
			t.Errorf("parseTimeLeft(%q) = %d, want %d", in, got, want)
		}
	}
}
