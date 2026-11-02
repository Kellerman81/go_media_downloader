package gotify

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

func TestSendNotificationUsesHeaderToken(t *testing.T) {
	var gotKey, gotQuery, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Gotify-Key")
		gotQuery = r.URL.RawQuery
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7}`))
	}))
	defer srv.Close()

	host, portStr, _ := net.SplitHostPort(srv.Listener.Addr().String())
	port, _ := strconv.Atoi(portStr)

	p := NewProviderWithConfig(base.ClientConfig{}, host+"/sub", port, "secret", false)

	resp, err := p.SendNotification(context.Background(), apiexternal_v2.NotificationRequest{Title: "t", Message: "m"})
	if err != nil || !resp.Success || resp.MessageID != "7" {
		t.Fatalf("SendNotification = %+v, %v", resp, err)
	}

	if gotKey != "secret" || gotQuery != "" || gotPath != "/sub/message" {
		t.Errorf("key=%q query=%q path=%q", gotKey, gotQuery, gotPath)
	}
}
