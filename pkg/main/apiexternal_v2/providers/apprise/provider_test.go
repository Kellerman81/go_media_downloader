package apprise

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

func TestKeyModeWithoutURLs(t *testing.T) {
	var (
		gotPath string
		gotBody map[string]any
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	host, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	p := NewProvider(host, port, "mykey", nil, false)

	resp, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{
		Message: "m",
		Options: map[string]string{"tag": "admins"},
	})
	if err != nil || !resp.Success {
		t.Fatalf("SendNotification = %+v, %v", resp, err)
	}

	if gotPath != "/notify/mykey" || gotBody["tag"] != "admins" {
		t.Errorf("path=%q body=%v", gotPath, gotBody)
	}

	if _, ok := gotBody["urls"]; ok {
		t.Errorf("urls sent in key mode: %v", gotBody["urls"])
	}
}

func TestStatelessModeRequiresURLs(t *testing.T) {
	p := NewProvider("127.0.0.1", 1, "", nil, false)

	_, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{
		Message: "m",
		Options: map[string]string{"urls": " , "},
	})
	if err == nil {
		t.Fatal("expected error without urls and key")
	}

	if got := splitURLs(" a://x , ,b://y "); len(got) != 2 || got[0] != "a://x" || got[1] != "b://y" {
		t.Errorf("splitURLs = %q", got)
	}
}
