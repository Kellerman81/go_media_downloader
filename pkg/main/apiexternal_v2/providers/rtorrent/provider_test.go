package rtorrent

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

const faultBody = `<?xml version="1.0"?><methodResponse><fault><value><struct>
<member><name>faultCode</name><value><i4>-501</i4></value></member>
<member><name>faultString</name><value><string>Could not find info-hash.</string></value></member>
</struct></value></fault></methodResponse>`

const okBody = `<?xml version="1.0"?><methodResponse><params><param><value><i4>0</i4></value></param></params></methodResponse>`

func newTestProvider(t *testing.T, h http.HandlerFunc) *Provider {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	p, err := NewProvider(u.Hostname(), port, "", "", false, "")
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestFaultStringIsReported(t *testing.T) {
	p := newTestProvider(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(faultBody))
	})

	err := p.PauseTorrent(context.Background(), "H")
	if err == nil || !strings.Contains(err.Error(), "-501") ||
		!strings.Contains(err.Error(), "Could not find info-hash.") {
		t.Fatalf("err = %v, want faultCode and faultString", err)
	}
}

func TestRawTorrentIsBase64AndHashReturned(t *testing.T) {
	var body string

	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte(okBody))
	})

	data := []byte("d4:info\x00\xffe")

	if _, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{TorrentData: data}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(body, "<base64>"+base64.StdEncoding.EncodeToString(data)+"</base64>") {
		t.Fatalf("request body lacks base64 value: %s", body)
	}

	resp, err := p.AddTorrent(context.Background(), apiexternal_v2.TorrentAddRequest{
		URL: "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=x",
	})
	if err != nil || resp.Hash != "0123456789ABCDEF0123456789ABCDEF01234567" {
		t.Fatalf("resp = %+v, %v", resp, err)
	}
}

func TestBtihFromURL(t *testing.T) {
	for in, want := range map[string]string{
		"magnet:?xt=urn:btih:AEBAGBAFAYDQQCIKBMGA2DQPCAIREEYU": "0102030405060708090A0B0C0D0E0F1011121314",
		"http://x/file.torrent":                                "",
	} {
		if got := btihFromURL(in); got != want {
			t.Errorf("btihFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoveTorrent(t *testing.T) {
	var methods []string

	p := newTestProvider(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		methods = append(methods, string(b))
		_, _ = w.Write([]byte(okBody))
	})

	if err := p.RemoveTorrent(context.Background(), "H", false); err != nil {
		t.Fatal(err)
	}

	if len(methods) != 1 || !strings.Contains(methods[0], "<methodName>d.erase</methodName>") {
		t.Fatalf("calls = %v, want one d.erase", methods)
	}

	if err := p.RemoveTorrent(context.Background(), "H", true); err == nil {
		t.Fatal("deleteFiles=true: expected unsupported error")
	}

	if len(methods) != 1 {
		t.Fatalf("deleteFiles=true sent a request: %v", methods)
	}
}
