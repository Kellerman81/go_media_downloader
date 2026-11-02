package base

import (
	"net/url"
	"strconv"
	"strings"
)

// SplitHostURL parses a configured hostname that may be bare ("nas.local",
// "nas.local:8080") or a full URL ("https://nas.local:9091/sub"). It returns
// the host (bracketed when IPv6, ready for "host:port"), the port, whether
// the scheme is https, and the URL path without a trailing slash. A port in
// the hostname wins over cfgPort; with neither, defaultPort is used, and with
// no default either, 80 or 443.
func SplitHostURL(raw string, cfgPort, defaultPort int) (host string, port int, useSSL bool, path string) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		// Not parseable as a URL: use it verbatim as the host.
		host = strings.TrimPrefix(raw, "http://")
	} else {
		useSSL = strings.EqualFold(u.Scheme, "https")
		host = u.Hostname()
		path = strings.TrimSuffix(u.EscapedPath(), "/")

		if p, err := strconv.Atoi(u.Port()); err == nil {
			port = p
		}
	}

	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}

	if port == 0 {
		port = cfgPort
	}

	// An explicit https:// URL without any port means 443; the clients'
	// default ports serve plain HTTP, so they can't be right for it. Plain
	// http:// keeps the client default for existing "http://nas/sab" setups.
	if port == 0 && useSSL {
		port = 443
	}

	if port == 0 {
		port = defaultPort
	}

	if port == 0 {
		port = 80
		if useSSL {
			port = 443
		}
	}

	return host, port, useSSL, path
}
