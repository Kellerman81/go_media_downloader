package lastfm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func TestLastFMErrorPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":6,"message":"Album not found"}`))
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL + "/", UserAgent: "test", APIKey: "k"})

	_, err := p.GetAlbumInfo(context.Background(), "a", "b", "")
	if !errors.Is(err, logger.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCheckLFMError(t *testing.T) {
	if err := checkLFMError([]byte(`{"results":{}}`)); err != nil {
		t.Errorf("success payload: %v", err)
	}

	if err := checkLFMError([]byte(`{"error":10,"message":"Invalid API key"}`)); err == nil ||
		errors.Is(err, logger.ErrNotFound) {
		t.Errorf("invalid key payload: %v", err)
	}
}
