package config

import "testing"

func TestLanguageCode(t *testing.T) {
	for in, want := range map[string]string{
		"German": "de", "ger": "de", "de-DE": "de", "de": "de", " English ": "en", "und": "", "": "", "xx": "xx",
	} {
		if got := LanguageCode(in); got != want {
			t.Errorf("LanguageCode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestNormalizeMetadataLanguages: language names in metadata_language
// become codes, and only then is the config reported as changed.
func TestNormalizeMetadataLanguages(t *testing.T) {
	cfg := MainConfig{Media: MediaConfig{
		Movies: []MediaTypeConfig{{Name: "EN", MetadataLanguage: "English"}, {Name: "DE", MetadataLanguage: "German"}},
		Series: []MediaTypeConfig{{Name: "DE", MetadataLanguage: "de"}, {Name: "X"}},
	}}

	if !normalizeMetadataLanguages(&cfg) {
		t.Fatal("names not reported as changed")
	}

	if got := cfg.Media.Movies[0].MetadataLanguage + cfg.Media.Movies[1].MetadataLanguage + cfg.Media.Series[0].MetadataLanguage; got != "endede" {
		t.Errorf("normalized to %q", got)
	}

	if normalizeMetadataLanguages(&cfg) {
		t.Error("codes reported as changed")
	}
}

// TestLanguageNames: the ISO 639-1 list for the metadata language: two
// lower-case letters each, the common ones in, the withdrawn aliases out.
func TestLanguageNames(t *testing.T) {
	if len(LanguageNames) < 180 {
		t.Errorf("%d languages", len(LanguageNames))
	}

	for code, name := range LanguageNames {
		if len(code) != 2 || code[0] < 'a' || code[0] > 'z' || code[1] < 'a' || code[1] > 'z' || name == "" {
			t.Errorf("%q: %q", code, name)
		}
	}

	for code, name := range map[string]string{"de": "German", "en": "English", "fr": "French", "ja": "Japanese", "no": "Norwegian"} {
		if LanguageNames[code] != name {
			t.Errorf("%s = %q, want %q", code, LanguageNames[code], name)
		}
	}

	for _, withdrawn := range []string{"iw", "in", "ji", "jw", "mo"} {
		if _, ok := LanguageNames[withdrawn]; ok {
			t.Errorf("withdrawn code %s listed", withdrawn)
		}
	}
}
