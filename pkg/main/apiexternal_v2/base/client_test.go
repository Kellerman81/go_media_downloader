package base

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		in   string
		want time.Time
	}{
		{"", time.Time{}},
		{"garbage", time.Time{}},
		{"120", now.Add(120 * time.Second)},
		{"-5", now},
		{"Fri, 02 Jan 2026 04:00:00 GMT", time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC)},
		{"Friday, 02-Jan-26 04:00:00 GMT", time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC)},
		{"Fri Jan  2 04:00:00 2026", time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC)},
		{"2026-01-02T04:00:00Z", time.Date(2026, 1, 2, 4, 0, 0, 0, time.UTC)},
		{"Request limit reached. Retry in 3 minutes.", now.Add(3 * time.Minute)},
		{"Request limit reached. Retry in 2 hours", now.Add(2 * time.Hour)},
		{"Request limit reached. Retry in soon", time.Time{}},
	}

	for _, tt := range tests {
		if got := parseRetryAfter(tt.in, now); !got.Equal(tt.want) {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestHandleRateLimitResponseFallbackBlock(t *testing.T) {
	bc := NewBaseClient(ClientConfig{Name: "t"})

	resp := &http.Response{StatusCode: 429, Header: http.Header{}}
	bc.handleRateLimitResponse(resp, nil)

	if err := bc.checkServerRateLimit(); err == nil {
		t.Fatal("429 without Retry-After must block further requests")
	}
}

func TestIsRetryableMethod(t *testing.T) {
	for _, m := range []string{"", "GET", "HEAD", "OPTIONS", "PUT", "DELETE"} {
		if !isRetryableMethod(m) {
			t.Errorf("%q should be retryable", m)
		}
	}

	for _, m := range []string{"POST", "PATCH"} {
		if isRetryableMethod(m) {
			t.Errorf("%q must not be retryable", m)
		}
	}
}

func TestApplyAuthAppendsWithoutReencoding(t *testing.T) {
	bc := NewBaseClient(ClientConfig{
		Name:        "t",
		AuthType:    AuthAPIKeyURL,
		APIKey:      "a b",
		APIKeyParam: "apikey",
	})

	req, _ := http.NewRequest(http.MethodGet, "http://h/api?z=1&q=foo%20bar", nil)
	if err := bc.applyAuth(req); err != nil {
		t.Fatal(err)
	}

	if req.URL.RawQuery != "z=1&q=foo%20bar&apikey=a+b" {
		t.Errorf("got %q", req.URL.RawQuery)
	}
}

func TestPostNotRetriedGetRetried(t *testing.T) {
	var hits atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	bc := NewBaseClient(ClientConfig{
		Name:                    "t",
		BaseURL:                 srv.URL,
		MaxRetries:              2,
		RetryBackoff:            time.Millisecond,
		CircuitBreakerThreshold: 100,
	})

	_ = bc.MakeRequest(context.Background(), http.MethodPost, "/add", strings.NewReader("{}"), nil, nil)

	if got := hits.Load(); got != 1 {
		t.Errorf("POST sent %d times, want 1", got)
	}

	hits.Store(0)

	_ = bc.MakeRequest(context.Background(), http.MethodGet, "/get", nil, nil, nil)

	if got := hits.Load(); got != 3 {
		t.Errorf("GET sent %d times, want 3", got)
	}
}

func TestCancelledRequestNotABreakerFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	bc := NewBaseClient(ClientConfig{
		Name:                    "t",
		BaseURL:                 srv.URL + "/x?apikey=secret",
		CircuitBreakerThreshold: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := bc.MakeRequest(ctx, http.MethodGet, "", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error should wrap the context error: %v", err)
	}

	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error leaks API key: %v", err)
	}

	if bc.circuitBreaker.GetFailureCount() != 0 {
		t.Error("caller cancellation was recorded as a breaker failure")
	}
}

func TestResponseBodyCap(t *testing.T) {
	data, err := readLimited(strings.NewReader("12345"), 5)
	if err != nil || string(data) != "12345" {
		t.Fatalf("got %q, %v", data, err)
	}

	if _, err := readLimited(strings.NewReader("123456"), 5); !errors.Is(err, errResponseTooLarge) {
		t.Fatalf("expected errResponseTooLarge, got %v", err)
	}
}

func TestReserveRateLimitConsumesOnce(t *testing.T) {
	bc := NewBaseClient(ClientConfig{
		Name:              "t",
		RateLimitCalls:    2,
		RateLimitSeconds:  60,
		RateLimitPerTotal: 10,
	})

	for range 2 {
		if err := bc.reserveRateLimit(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
	}

	if err := bc.reserveRateLimit(context.Background(), 0); err == nil {
		t.Fatal("third request within the window should be rejected")
	}

	if got := bc.rateLimiterTotal.Load(); got != 2 {
		t.Errorf("total counter = %d, want 2 (rejected request must not count)", got)
	}
}

func TestMakeRequestAcceptStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Token", "t1")
		if r.URL.Path == "/conflict" {
			w.WriteHeader(http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	bc := NewBaseClient(ClientConfig{Name: "accept_test", BaseURL: srv.URL, Timeout: 5 * time.Second})

	var gotStatus int

	var gotHeader string

	err := bc.MakeRequestAcceptStatus(context.Background(), http.MethodPost, "/conflict", nil,
		func(resp *http.Response) error {
			gotStatus = resp.StatusCode
			gotHeader = resp.Header.Get("X-Token")

			return nil
		}, nil, http.StatusConflict)
	if err != nil || gotStatus != http.StatusConflict || gotHeader != "t1" {
		t.Fatalf("accepted 409: err=%v status=%d header=%q", err, gotStatus, gotHeader)
	}

	called := false
	err = bc.MakeRequestAcceptStatus(context.Background(), http.MethodPost, "/other", nil,
		func(*http.Response) error {
			called = true
			return nil
		}, nil, http.StatusConflict)
	if err == nil || called || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("non-accepted 404: err=%v called=%v", err, called)
	}
}

func TestOAuthTokenStorageKeyedByProviderIdentity(t *testing.T) {
	newClient := func(tokenURL, clientID string) *BaseClient {
		return NewBaseClient(ClientConfig{
			Name:          "shared",
			AuthType:      AuthOAuth,
			OAuthTokenURL: tokenURL,
			OAuthClientID: clientID,
		})
	}

	first := newClient("https://a.invalid/oauth/token", "app1")
	if err := first.SetOAuthToken(&oauth2.Token{AccessToken: "tok-a"}); err != nil {
		t.Fatal(err)
	}

	if tok := newClient("https://b.invalid/oauth/token", "app1").GetOAuthToken(); tok != nil {
		t.Fatalf("client for another token endpoint loaded %q", tok.AccessToken)
	}

	if tok := newClient("https://a.invalid/oauth/token", "app2").GetOAuthToken(); tok != nil {
		t.Fatalf("client for another OAuth app loaded %q", tok.AccessToken)
	}

	tok := newClient("https://a.invalid/oauth/token", "app1").GetOAuthToken()
	if tok == nil || tok.AccessToken != "tok-a" {
		t.Fatalf("same identity did not load the stored token: %v", tok)
	}
}

func TestTokenNeedsRefresh(t *testing.T) {
	cases := map[string]struct {
		tok  *oauth2.Token
		want bool
	}{
		"nil":            {nil, true},
		"empty":          {&oauth2.Token{}, true},
		"expired":        {&oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(-time.Minute)}, true},
		"expiring soon":  {&oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(3 * time.Minute)}, true},
		"valid":          {&oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)}, false},
		"never expiring": {&oauth2.Token{AccessToken: "a"}, false},
	}

	for name, c := range cases {
		if got := tokenNeedsRefresh(c.tok); got != c.want {
			t.Errorf("%s: tokenNeedsRefresh = %t, want %t", name, got, c.want)
		}
	}
}
