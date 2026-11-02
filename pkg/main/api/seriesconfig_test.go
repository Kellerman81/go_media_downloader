package api

import (
	"testing"
)

func TestSeriesConfigPath(t *testing.T) {
	for _, bad := range []string{
		"", "../config.toml", "config.toml", "sub/series.toml", `..\x.toml`, "series.txt", "/etc/x.toml",
	} {
		if _, err := seriesConfigPath(bad); err == nil {
			t.Errorf("seriesConfigPath(%q) accepted", bad)
		}
	}

	if _, err := seriesConfigPath("series.toml"); err != nil {
		t.Errorf("seriesConfigPath(series.toml) rejected: %v", err)
	}
}
