package base

import "testing"

func TestSplitHostURL(t *testing.T) {
	tests := []struct {
		raw              string
		cfgPort, defPort int
		host             string
		port             int
		ssl              bool
		path             string
	}{
		{"nas.local", 0, 9091, "nas.local", 9091, false, ""},
		{"nas.local", 1234, 9091, "nas.local", 1234, false, ""},
		{"https://nas.local", 0, 0, "nas.local", 443, true, ""},
		{"https://nas.local:8443/sab/", 8080, 0, "nas.local", 8443, true, "/sab"},
		{"http://host", 0, 0, "host", 80, false, ""},
		{"host:8080/path", 0, 0, "host", 8080, false, "/path"},
		{"[::1]:9091", 0, 0, "[::1]", 9091, false, ""},
		{"https://host/sab", 0, 8080, "host", 443, true, "/sab"},
		{"https://host/sab", 9443, 8080, "host", 9443, true, "/sab"},
		{"http://nas/sabnzbd", 0, 8080, "nas", 8080, false, "/sabnzbd"},
	}

	for _, tt := range tests {
		host, port, ssl, path := SplitHostURL(tt.raw, tt.cfgPort, tt.defPort)
		if host != tt.host || port != tt.port || ssl != tt.ssl || path != tt.path {
			t.Errorf("SplitHostURL(%q, %d, %d) = (%q, %d, %v, %q), want (%q, %d, %v, %q)",
				tt.raw, tt.cfgPort, tt.defPort, host, port, ssl, path,
				tt.host, tt.port, tt.ssl, tt.path)
		}
	}
}
