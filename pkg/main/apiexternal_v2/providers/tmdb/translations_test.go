package tmdb

import (
	"encoding/json"
	"testing"
)

// TestPickTranslations: one translation per language, the language's own
// country first; a series' name is its title; empty ones are left out.
func TestPickTranslations(t *testing.T) {
	var response tmdbTranslations
	if err := json.Unmarshal([]byte(`{"translations": [
		{"iso_3166_1": "AT", "iso_639_1": "de", "data": {"title": "Das Boot (AT)", "overview": "Ein U-Boot, lang beschrieben."}},
		{"iso_3166_1": "DE", "iso_639_1": "de", "data": {"title": "Das Boot", "overview": "Ein U-Boot."}},
		{"iso_3166_1": "GB", "iso_639_1": "en", "data": {"name": "The Boat (GB)", "overview": "A submarine, at length."}},
		{"iso_3166_1": "US", "iso_639_1": "en", "data": {"name": "The Boat", "overview": "A submarine.", "tagline": "Dive"}},
		{"iso_3166_1": "FR", "iso_639_1": "fr", "data": {"title": "", "overview": ""}}
	]}`), &response); err != nil {
		t.Fatal(err)
	}

	got := pickTranslations(&response)
	if len(got) != 2 {
		t.Fatalf("translations %+v", got)
	}

	if got[0].Language != "de" || got[0].Title != "Das Boot" || got[1].Title != "The Boat" || got[1].Tagline != "Dive" {
		t.Errorf("picked %+v", got)
	}
}
