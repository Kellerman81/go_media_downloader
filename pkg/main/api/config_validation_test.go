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
