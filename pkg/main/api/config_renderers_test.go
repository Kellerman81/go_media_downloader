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
