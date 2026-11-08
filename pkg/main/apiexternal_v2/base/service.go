package base

import (
	"net/url"
	"time"
)

// ServiceConfig is the ClientConfig the small notification and
// download-client providers start from: no auth, a generous hourly limit and
// a circuit breaker that opens after five failures. Callers set auth on top.
func ServiceConfig(name, baseURL string) ClientConfig {
	return ClientConfig{
		Name:                      name,
		BaseURL:                   baseURL,
		Timeout:                   30 * time.Second,
		AuthType:                  AuthNone,
		RateLimitCalls:            600,
		RateLimitSeconds:          3600,
		CircuitBreakerThreshold:   5,
		CircuitBreakerTimeout:     60 * time.Second,
		CircuitBreakerHalfOpenMax: 1,
		MaxRetries:                2,
		RetryBackoff:              2 * time.Second,
	}
}

// WithURLCredentials returns cfg with basic auth taken from the userinfo of
// rawURL ("https://user:pass@host/..."), and rawURL without it. Without
// userinfo both come back unchanged.
func WithURLCredentials(cfg ClientConfig, rawURL string) (ClientConfig, string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.User == nil {
		return cfg, rawURL
	}

	cfg.AuthType = AuthBasic
	cfg.Username = u.User.Username()
	cfg.Password, _ = u.User.Password()
	u.User = nil

	return cfg, u.String()
}
