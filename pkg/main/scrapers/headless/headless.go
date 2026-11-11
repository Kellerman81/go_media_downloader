// Package headless loads web pages through a headless Chrome with the
// browserless /content API (headless_chrome_url), for sites that refuse plain
// HTTP clients.
package headless

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Timeout is the HTTP client timeout for a request through Chrome, which
// waits for the page's scripts and network to settle before answering.
const Timeout = 90 * time.Second

// PageRequest returns the request that loads pageURL: a plain GET when
// chromeURL is empty, else a POST to chromeURL's /content endpoint, whose
// response body is the HTML Chrome rendered.
func PageRequest(ctx context.Context, chromeURL, pageURL string) (*http.Request, error) {
	if chromeURL == "" {
		return http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	}

	body, err := json.Marshal(map[string]any{
		"url":         pageURL,
		"gotoOptions": map[string]any{"waitUntil": "networkidle2", "timeout": 60000},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(chromeURL, "/")+"/content", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return req, nil
}
