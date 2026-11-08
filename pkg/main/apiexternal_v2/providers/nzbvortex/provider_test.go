package nzbvortex

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestLoginHash(t *testing.T) {
	// sha256("n:c:k") in base64, as NZBVortex expects.
	if got := loginHash("n", "c", "k"); got != "N+V1u9de+4TR4tKMRiKTVaTA5i1wLPyZKQPf2RAP0Vg=" {
		t.Errorf("got %s", got)
	}
}

func TestAddNZBURL(t *testing.T) {
	nzbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "<nzb/>")
	}))
	defer nzbSrv.Close()

	var (
		upload, group string
		logins        int
	)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		switch r.URL.Path {
		case "/api/auth/nonce":
			io.WriteString(w, `{"authNonce":"n"}`)
		case "/api/auth/login":
			logins++

			if q.Get("hash") != loginHash("n", q.Get("cnonce"), "k") {
				io.WriteString(w, `{"loginResult":"failed"}`)
				return
			}

			io.WriteString(w, `{"loginResult":"successful","sessionID":"s`+strconv.Itoa(logins)+`"}`)
		case "/api/nzb/add":
			if q.Get("sessionid") == "s1" {
				io.WriteString(w, `{"result":"not_logged_in"}`)
				return
			}

			f, h, err := r.FormFile("name")
			if err != nil {
				t.Error(err)
				return
			}

			b, _ := io.ReadAll(f)
			upload, group = h.Filename+":"+string(b), q.Get("groupname")
			io.WriteString(w, `{"result":"ok","add_uuid":"u-7"}`)
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p := NewProvider(u.Hostname(), port, "k", "")
	id, err := p.AddNZBURL(context.Background(), nzbSrv.URL+"/get", "tv", "Show.S01E01", 0, false)
	if err != nil || id != "u-7" {
		t.Fatalf("add: %q %v", id, err)
	}

	if logins != 2 || upload != "Show.S01E01.nzb:<nzb/>" || group != "tv" {
		t.Errorf("logins %d, upload %q, group %q", logins, upload, group)
	}

	if err := NewProvider(u.Hostname(), port, "wrong", "").TestConnection(context.Background()); err == nil {
		t.Error("wrong API key accepted")
	}
}
