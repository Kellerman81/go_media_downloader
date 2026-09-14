// Package sitethrottle enforces a minimum delay between HTTP requests to the
// same site (host), shared across every scraper and scraper config. Several
// scraper configs can target one site - e.g. multiple galleries on
// nubilefilms.com - and each used to wait only between its own pages, so running
// together they could still hammer the site. Keying the delay on the host makes
// the wait apply per site_url globally instead of per config.
package sitethrottle

import (
	"context"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	next = make(map[string]time.Time)
)

// Wait blocks until the next request to key (typically a host) is allowed, then
// reserves the following slot minInterval later. Concurrent callers for the same
// key are serialised in arrival order, each spaced by minInterval. A zero/
// negative interval or empty key is a no-op, and the wait is abandoned if ctx is
// cancelled.
func Wait(ctx context.Context, key string, minInterval time.Duration) {
	if key == "" || minInterval <= 0 {
		return
	}

	mu.Lock()
	now := time.Now()

	start := now
	if n, ok := next[key]; ok && n.After(now) {
		start = n
	}

	next[key] = start.Add(minInterval)
	mu.Unlock()

	sleep := time.Until(start)
	if sleep <= 0 {
		return
	}

	timer := time.NewTimer(sleep)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
