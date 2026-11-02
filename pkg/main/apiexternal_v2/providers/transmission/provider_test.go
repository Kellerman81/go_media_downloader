package transmission

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestAddTorrentSessionHandshakeAndDuplicate(t *testing.T) {
	var calls atomic.Int32

	var gotArgs map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.URL.Path != "/transmission/rpc" {
			t.Errorf("path = %q", r.URL.Path)
		}

		if r.Header.Get("X-Transmission-Session-Id") != "sid-1" {
			w.Header().Set("X-Transmission-Session-Id", "sid-1")
			w.WriteHeader(http.StatusConflict)

			return
		}

		var req struct {
			Method    string         `json:"method"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}

		gotArgs = req.Arguments

		_, _ = w.Write([]byte(`{"result":"success","arguments":{"torrent-duplicate":{"hashString":"abc123","id":1}}}`))
	}))
	defer srv.Close()

	p := NewProvider(srv.URL+"/", "", "")

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		TorrentData: []byte("d4:infoe"),
	})
	if err != nil {
		t.Fatalf("AddTorrent: %v", err)
	}

	if !resp.Success || resp.Hash != "abc123" {
		t.Fatalf("resp = %+v, want success with duplicate hash", resp)
	}

	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2 (409 handshake + retry)", calls.Load())
	}

	if _, ok := gotArgs["download-dir"]; ok {
		t.Errorf("download-dir sent for empty SavePath")
	}

	if gotArgs["metainfo"] != base64.StdEncoding.EncodeToString([]byte("d4:infoe")) {
		t.Errorf("metainfo = %v, want base64", gotArgs["metainfo"])
	}
}

func TestPersistent409IsReported(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := calls.Add(1)
		w.Header().Set("X-Transmission-Session-Id", "rotating-"+string(rune('a'+n)))
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	p := NewProvider(srv.URL, "", "")
	if err := p.TestConnection(context.Background()); err == nil {
		t.Fatal("expected error for persistent 409")
	}

	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

func TestRPCURL(t *testing.T) {
	for in, want := range map[string]string{
		"http://h:9091":                   "http://h:9091/transmission/rpc",
		"http://h:9091/":                  "http://h:9091/transmission/rpc",
		"http://h:9091/transmission/rpc/": "http://h:9091/transmission/rpc",
	} {
		if got := rpcURL(in); got != want {
			t.Errorf("rpcURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBandwidthPriority(t *testing.T) {
	for in, want := range map[int]int{-3: -1, -2: -1, -1: -1, 0: 0, 1: 1, 2: 1, 5: 1} {
		if got := bandwidthPriority(in); got != want {
			t.Errorf("bandwidthPriority(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestAddTorrentSendsBandwidthPriority(t *testing.T) {
	tests := []struct {
		priority int
		want     any // nil = not sent
	}{
		{-2, float64(-1)},
		{0, nil},
		{2, float64(1)},
	}

	for _, tt := range tests {
		var gotArgs map[string]any

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			gotArgs = req.Arguments

			_, _ = w.Write([]byte(`{"result":"success","arguments":{"torrent-added":{"hashString":"h"}}}`))
		}))

		p := NewProvider(srv.URL+"/", "", "")

		if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
			URL:      "magnet:?xt=urn:btih:abc",
			Priority: tt.priority,
		}); err != nil {
			t.Fatalf("priority %d: %v", tt.priority, err)
		}

		srv.Close()

		got, sent := gotArgs["bandwidthPriority"]
		if tt.want == nil {
			if sent {
				t.Errorf("priority %d: bandwidthPriority %v sent, want omitted", tt.priority, got)
			}
		} else if got != tt.want {
			t.Errorf("priority %d: bandwidthPriority = %v, want %v", tt.priority, got, tt.want)
		}
	}
}

func TestGetStatusString(t *testing.T) {
	want := []string{"stopped", "check_waiting", "checking", "download_waiting", "downloading", "seed_waiting", "seeding"}
	for i, w := range want {
		if got := getStatusString(i); got != w {
			t.Errorf("getStatusString(%d) = %q, want %q", i, got, w)
		}
	}

	if got := getStatusString(7); got != "unknown" {
		t.Errorf("getStatusString(7) = %q", got)
	}
}
