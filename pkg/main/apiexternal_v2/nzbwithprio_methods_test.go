package apiexternal_v2

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestGetregexcfgAndIndexerContext(t *testing.T) {
	rgx := &config.RegexConfig{Name: "strict"}
	ind := &config.IndexersConfig{Name: "geek"}
	qual := &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "GEEK", CfgRegex: rgx}}}

	results := make([]Nzbwithprio, 2)
	PopulateIndexerContext(results, ind, qual)

	if results[1].NZB.Indexer != ind || results[1].NZB.Quality != qual {
		t.Fatal("PopulateIndexerContext did not set the context")
	}

	if got := results[0].Getregexcfg(qual); got != rgx {
		t.Errorf("Getregexcfg = %v, want the quality's regex", got)
	}

	if results[0].Getregexcfg(nil) != nil || results[0].Getregexcfg(&config.QualityConfig{}) != nil {
		t.Error("regex returned without indexers in the quality")
	}

	other := &config.QualityConfig{Indexer: []config.QualityIndexerConfig{{TemplateIndexer: "slug", CfgRegex: rgx}}}
	if results[0].Getregexcfg(other) != nil {
		t.Error("regex returned for an indexer not in the quality")
	}

	var noIndexer Nzbwithprio
	if noIndexer.Getregexcfg(qual) != nil {
		t.Error("regex returned for a result without an indexer")
	}
}
