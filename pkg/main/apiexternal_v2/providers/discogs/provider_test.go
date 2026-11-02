package discogs

import (
	"context"
	"errors"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func TestSearchWithoutTokenFailsClearly(t *testing.T) {
	p := NewProviderWithConfig(base.ClientConfig{BaseURL: "http://127.0.0.1:1", UserAgent: "test"}, "")

	_, err := p.Search(context.Background(), "x", "release", 1, 1)
	if !errors.Is(err, logger.ErrDisabled) {
		t.Errorf("err = %v, want ErrDisabled", err)
	}
}
