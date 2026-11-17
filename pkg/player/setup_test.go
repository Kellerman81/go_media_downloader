package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestSetup: a wrong or unreachable address shows the page again with the
// problem; a working one is saved and the window goes on to the portal.
func TestSetup(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer portal.Close()

	dir := t.TempDir()

	page, saved, stop, err := setup(dir, config{}, "the portal can't be reached")
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	get := func(resp *http.Response, err error) string {
		t.Helper()

		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		b, _ := io.ReadAll(resp.Body)

		return string(b)
	}

	if body := get(http.Get(page)); !strings.Contains(body, "the portal can&#39;t be reached") {
		t.Errorf("setup page without the problem: %s", body)
	}

	if body := get(http.PostForm(page+"save", url.Values{"url": {"ftp://nope"}})); !strings.Contains(body, "enter the portal address") {
		t.Errorf("bad address: %s", body)
	}

	if body := get(http.PostForm(page+"save", url.Values{"url": {"http://127.0.0.1:1"}})); !strings.Contains(body, "can&#39;t be reached") {
		t.Errorf("unreachable address: %s", body)
	}

	body := get(http.PostForm(page+"save", url.Values{"url": {portal.URL + "/"}}))
	if !strings.Contains(body, portal.URL+"/api/admin/library?app=1") {
		t.Errorf("no redirect to the portal: %s", body)
	}

	select {
	case got := <-saved:
		if got != portal.URL || loadConfig(dir).URL != portal.URL {
			t.Errorf("saved %q, config %+v", got, loadConfig(dir))
		}
	case <-time.After(time.Second):
		t.Error("not saved")
	}
}
