package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestDecodeManualConfig(t *testing.T) {
	cfg, err := DecodeManualConfig([]byte(`
[global]
identifiedby = "date"
search = false

[[config]]
name = "Unset"

[[config]]
name = "Overrides"
identifiedby = "ep"
dont_search = false
dont_upgrade = true
`))
	if err != nil {
		t.Fatal(err)
	}

	unset, over := cfg.Config[0], cfg.Config[1]

	// Unset fields take [global]; upgrade has no global default, so it stays.
	if unset.Identifiedby != "date" || !unset.DontSearch || unset.DontUpgrade || unset.Explicit.Has("dont_search") {
		t.Errorf("unset entry = %+v", unset)
	}

	// Explicit values win, also an explicit false.
	if over.Identifiedby != "ep" || over.DontSearch || !over.DontUpgrade || !over.Explicit.Has("dont_search") {
		t.Errorf("explicit entry = %+v", over)
	}

	if v, ok := cfg.Global.DefaultFor("dont_search"); !ok || v != true {
		t.Errorf("DefaultFor(dont_search) = %v, %v", v, ok)
	}

	if _, ok := cfg.Global.DefaultFor("dont_upgrade"); ok {
		t.Error("dont_upgrade has no default here")
	}

	// Without [global] nothing changes.
	plain, err := DecodeManualConfig([]byte("[[config]]\nname = \"A\"\n"))
	if err != nil || plain.Config[0].Identifiedby != "" || plain.Config[0].DontSearch {
		t.Errorf("no global: %+v, %v", plain, err)
	}
}

// manualConfigBenchData is shaped like a production series file: a
// [global] section and 250 entries, some with more keys.
func manualConfigBenchData() []byte {
	var b strings.Builder

	b.WriteString("[global]\nidentifiedby = \"ep\"\nupgrade = true\nsearch = true\n")

	for i := range 250 {
		fmt.Fprintf(&b, "[[config]]\nname = \"Series %d\"\ntvdb_id = %d\n", i, 100000+i)

		if i%5 == 0 {
			fmt.Fprintf(&b, "alternatename = [\"Series %d (2013)\", \"The Series %d\"]\nidentifiedby = \"date\"\n", i, i)
		}
	}

	return []byte(b.String())
}

func BenchmarkDecodeManualConfig(b *testing.B) {
	data := manualConfigBenchData()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := DecodeManualConfig(data); err != nil {
			b.Fatal(err)
		}
	}
}

// TestScanManualKeys: the expression scan finds the same keys per entry as
// a generic decode, and documents it can't follow use the generic decode.
func TestScanManualKeys(t *testing.T) {
	for name, doc := range map[string]string{
		"plain":          "[[config]]\nname = 'A'\ntvdb_id = 1\n[[config]]\nname = 'B'\ndont_search = false\n",
		"quoted keys":    "[[config]]\n\"name\" = 'A'\n'identifiedby' = 'date'\n",
		"dotted key":     "[[config]]\nname = 'A'\nsource.x = 1\n",
		"global after":   "[[config]]\nname = 'A'\n[global]\nidentifiedby = 'date'\n[[config]]\nname = 'B'\n",
		"sub table":      "[[config]]\nname = 'A'\n[config.target]\nx = 1\n[[config]]\nname = 'B'\n",
		"unknown key":    "[[config]]\nname = 'A'\nnot_a_field = 1\n",
		"comments, crlf": "# c\r\n[[config]] # c\r\nname = 'A' # c\r\n\r\ndont_upgrade = true\r\n",
	} {
		scanned, ok := scanManualKeys([]byte(doc))
		decoded, err := decodeManualKeys([]byte(doc))

		if !ok || err != nil || fmt.Sprint(scanned) != fmt.Sprint(decoded) {
			t.Errorf("%s: scan %v (ok %v), decode %v (%v)", name, scanned, ok, decoded, err)
		}
	}

	// An inline array of entries isn't followed by the scan: the generic
	// decode still finds the keys.
	cfg, err := DecodeManualConfig([]byte("config = [{name = 'A', dont_search = false}, {name = 'B'}]\n[global]\nsearch = false\n"))
	if err != nil || len(cfg.Config) != 2 || !cfg.Config[0].Explicit.Has("dont_search") || cfg.Config[0].DontSearch ||
		cfg.Config[1].Explicit.Has("dont_search") || !cfg.Config[1].DontSearch {
		t.Fatalf("inline array: %+v, %v", cfg, err)
	}
}
