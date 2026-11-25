package api

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestValidateImdbConfigTitleTypes(t *testing.T) {
	ok := &config.ImdbConfig{ImdbIDSize: 1, Indexedtypes: []string{"movie", "tvMiniSeries", "tvSpecial"}}
	if err := validateImdbConfig(ok); err != nil {
		t.Errorf("valid IMDb types rejected: %v", err)
	}

	err := validateImdbConfig(&config.ImdbConfig{ImdbIDSize: 1, Indexedtypes: []string{"movie", "tvmovie"}})
	if err == nil || !strings.Contains(err.Error(), `IMDb's type is "tvMovie"`) {
		t.Errorf("wrongly cased type: %v", err)
	}

	if err := validateImdbConfig(&config.ImdbConfig{ImdbIDSize: 1, Indexedtypes: []string{"cartoon"}}); err == nil {
		t.Error("unknown type accepted")
	}
}

// TestValidateGeneralConfigLogFileSize: 0 is the documented default (10 MB)
// and must not block saving the general settings; a negative size does.
func TestValidateGeneralConfigLogFileSize(t *testing.T) {
	g := config.GeneralConfig{TimeFormat: "rfc3339", LogLevel: "info", WebPort: "9090"}
	if err := validateGeneralConfig(&g); err != nil {
		t.Errorf("log_file_size 0: %v", err)
	}

	g.LogFileSize = -1
	if err := validateGeneralConfig(&g); err == nil {
		t.Error("negative log_file_size accepted")
	}
}
