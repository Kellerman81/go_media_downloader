package importfeed

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestImportQualityConfigAndYearCandidate(t *testing.T) {
	strict := &config.QualityConfig{Name: "strict"}
	lenient := &config.QualityConfig{Name: "lenient", CheckYear1: true}

	cfgp := &config.MediaTypeConfig{
		CfgQuality: strict,
		Lists: []config.MediaListsConfig{
			{Name: "a", CfgQuality: strict},
			{Name: "b", CfgQuality: lenient},
		},
	}

	m := &database.ParseInfo{ListID: 1, Year: 2000}
	if q := importQualityConfig(m, cfgp); q != lenient {
		t.Fatalf("importQualityConfig picked %v, want list quality", q)
	}

	m.ListID = -1
	if q := importQualityConfig(m, cfgp); q != strict {
		t.Fatalf("importQualityConfig fallback picked %v, want media default", q)
	}

	arr := []database.DbstaticOneStringOneInt{
		{Str: "tt1", Num: 2001},
		{Str: "tt2", Num: 2000},
	}

	m.ListID = 1
	if got := pickImdbYearCandidate(arr, m, cfgp); got != 1 {
		t.Errorf("exact year should win, got %d", got)
	}

	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != 0 {
		t.Errorf("year+1 with CheckYear1 should match, got %d", got)
	}

	m.ListID = 0
	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != -1 {
		t.Errorf("year+1 without CheckYear1 must not match, got %d", got)
	}

	m.Year = 0
	if got := pickImdbYearCandidate(arr[:1], m, cfgp); got != 0 {
		t.Errorf("single yearless candidate should match, got %d", got)
	}

	if got := pickImdbYearCandidate(arr, m, cfgp); got != -1 {
		t.Errorf("ambiguous yearless candidates must not match, got %d", got)
	}
}
