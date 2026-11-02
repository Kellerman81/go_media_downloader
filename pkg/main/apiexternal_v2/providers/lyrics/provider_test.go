package lyrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripLRC(t *testing.T) {
	in := "[ar:Artist]\n[ti:Title]\n[00:01.00]First line\n[00:05.50][01:05.50]Chorus\n[00:09.00]\n[00:10.00] [bracketed] text"
	want := "\n\nFirst line\nChorus\n\n[bracketed] text"

	if got := stripLRC(in); got != want {
		t.Errorf("stripLRC = %q, want %q", got, want)
	}
}

func TestFromLRCLIBFallsBackToSynced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get" || r.URL.Query().Get("artist_name") != "A" {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plainLyrics":"","syncedLyrics":"[00:01.00]Hello\r\n[00:02.00]World","instrumental":false}`))
	}))
	defer srv.Close()

	c := newClient("lrclib-test", srv.URL)
	if got := fromLRCLIB(context.Background(), c, "A", "T", ""); got != "Hello\nWorld" {
		t.Errorf("fromLRCLIB = %q", got)
	}
}
