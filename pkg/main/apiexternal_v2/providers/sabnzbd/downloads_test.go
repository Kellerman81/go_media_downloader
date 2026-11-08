package sabnzbd

import (
	"context"
	"net/http"
	"testing"
)

func TestAddNZBURLPaused(t *testing.T) {
	for _, tc := range []struct {
		priority int
		paused   bool
		want     string
	}{
		{1, true, "-2"},
		{0, true, "-2"},
		{1, false, "1"},
		{0, false, ""},
	} {
		var got string

		p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
			got = r.URL.Query().Get("priority")
			_, _ = w.Write([]byte(`{"status":true,"nzo_ids":["SABnzbd_nzo_1"]}`))
		})

		id, err := p.AddNZBURL(context.Background(), "http://x/a.nzb", "", "n", tc.priority, tc.paused)
		if err != nil || id != "SABnzbd_nzo_1" {
			t.Fatalf("add: %q %v", id, err)
		}

		if got != tc.want {
			t.Errorf("priority %d paused %t: sent %q, want %q", tc.priority, tc.paused, got, tc.want)
		}
	}
}
