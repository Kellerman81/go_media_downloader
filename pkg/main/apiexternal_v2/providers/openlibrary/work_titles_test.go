package openlibrary

import (
	"testing"
)

// TestEditionTitles: per language the title most editions carry, spellings
// of one title counted together and the one with umlauts preferred; other
// languages and mistagged editions don't win.
func TestEditionTitles(t *testing.T) {
	ed := func(title string, langs ...string) olEditionEntry {
		e := olEditionEntry{Title: title}
		for _, l := range langs {
			e.Languages = append(e.Languages, olRef{Key: "/languages/" + l})
		}

		return e
	}

	pillars := []olEditionEntry{
		ed("The Pillars of the Earth", "eng"),
		ed("Die Säulen der Erde", "ger"),
		ed("Die Säulen der Erde", "ger"),
		ed("Die Saulen der Erde.", "ger"),
		ed("les pilliers de la terre", "ger"),
		ed("Los pilares de la Tierra", "spa"),
		ed("Ohne Sprache"),
	}

	got := editionTitles(pillars, []string{"de"})
	if len(got) != 1 || got["de"] != "Die Säulen der Erde" {
		t.Errorf("pillars: %v", got)
	}

	bridges := []olEditionEntry{ed("Die Brucken Der Freiheit", "ger"), ed("Die Brücken der Freiheit.", "ger"), ed("A Place Called Freedom", "eng")}
	if got := editionTitles(bridges, []string{"de", "en"}); got["de"] != "Die Brücken der Freiheit" || got["en"] != "A Place Called Freedom" {
		t.Errorf("bridges: %v", got)
	}

	if got := editionTitles(bridges, []string{"fr"}); len(got) != 0 {
		t.Errorf("no French edition: %v", got)
	}
}
