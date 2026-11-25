package api

import (
	"slices"
	"strings"
	"testing"
)

func TestSelectOptionsWithCurrent(t *testing.T) {
	opts := []string{"info", "debug"}

	if got := selectOptionsWithCurrent(opts, "Info"); !slices.Equal(got, []string{"info", "debug", "Info"}) {
		t.Errorf("unknown value not added: %v", got)
	}

	if got := selectOptionsWithCurrent(opts, "debug"); !slices.Equal(got, opts) {
		t.Errorf("known value duplicated: %v", got)
	}

	if got := selectOptionsWithCurrent(opts, ""); !slices.Equal(got, opts) {
		t.Errorf("empty value added: %v", got)
	}
}

func TestRenderFormGroupSelectUnset(t *testing.T) {
	opts := map[string][]string{"options": {"tvdb", "scraper", "none"}}

	render := func(value string) string {
		var b strings.Builder
		if err := renderFormGroup("serie", nil, nil, "Source", "select", value, opts).Render(&b); err != nil {
			t.Fatal(err)
		}

		return b.String()
	}

	// Unset: an empty option is selected, not the first real one.
	if out := render(""); !strings.Contains(out, `<option value="" selected>— not set —</option>`) ||
		strings.Contains(out, `value="none" selected`) {
		t.Errorf("unset select:\n%s", out)
	}

	// Set: no empty option, the value is selected.
	if out := render("tvdb"); strings.Contains(out, "not set") || !strings.Contains(out, `value="tvdb" selected`) {
		t.Errorf("set select:\n%s", out)
	}
}

// TestRenderLanguageSelect: the metadata language is picked from the ISO
// 639-1 codes by name; an unset value selects "not set", and a value that
// isn't a code stays selected instead of being lost on saving.
func TestRenderLanguageSelect(t *testing.T) {
	render := func(v string) string {
		return renderComponentToString(renderLanguageSelect("media_main_x_MetadataLanguage", v))
	}

	if out := render("de"); !strings.Contains(out, `<option value="de" selected>German (de)</option>`) ||
		strings.Count(out, "selected") != 1 || !strings.Contains(out, `name="media_main_x_MetadataLanguage"`) {
		t.Errorf("de: %.400s", out)
	}

	if out := render(""); !strings.Contains(out, `<option value="" selected>— not set —</option>`) || strings.Count(out, "selected") != 1 {
		t.Errorf("unset: %.300s", out)
	}

	if out := render("de-DE"); !strings.Contains(out, `<option value="de-DE" selected>de-DE (not an ISO 639-1 code)</option>`) {
		t.Errorf("de-DE: %.400s", out)
	}

	// Sorted by name: Afar (aa) comes before Zulu (zu), German after French.
	out := render("")
	if a, z := strings.Index(out, "Afar (aa)"), strings.Index(out, "Zulu (zu)"); a < 0 || z < a {
		t.Error("not sorted by name")
	}

	if f, g := strings.Index(out, "French (fr)"), strings.Index(out, "German (de)"); f < 0 || g < f {
		t.Error("German before French")
	}
}
