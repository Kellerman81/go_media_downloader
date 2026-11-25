package config

import (
	"slices"
	"strings"
)

// languageCodes maps three-letter codes and language names to ISO 639-1.
var languageCodes = map[string]string{
	"eng": "en", "english": "en", "ger": "de", "deu": "de", "german": "de", "deutsch": "de",
	"fre": "fr", "fra": "fr", "french": "fr", "spa": "es", "spanish": "es", "ita": "it", "italian": "it",
	"jpn": "ja", "japanese": "ja", "dut": "nl", "nld": "nl", "dutch": "nl", "por": "pt", "portuguese": "pt",
	"rus": "ru", "russian": "ru", "pol": "pl", "polish": "pl", "swe": "sv", "swedish": "sv", "dan": "da", "danish": "da",
	"nor": "no", "nob": "no", "norwegian": "no", "fin": "fi", "finnish": "fi", "tur": "tr", "turkish": "tr",
	"kor": "ko", "korean": "ko", "chi": "zh", "zho": "zh", "chinese": "zh", "cze": "cs", "ces": "cs", "czech": "cs",
	"hun": "hu", "hungarian": "hu", "gre": "el", "ell": "el", "greek": "el", "ara": "ar", "arabic": "ar",
	"heb": "he", "hebrew": "he", "hin": "hi", "hindi": "hi",
}

// LanguageNames are the ISO 639-1 language codes with their English names:
// the languages metadata_language can be (TMDB, TVDB and Audible use these
// codes). Generated from golang.org/x/text (CLDR) on 2026-11-25, without
// the withdrawn aliases (in, iw, ji, jw, mo); sh and bh stay, TMDB uses them.
var LanguageNames = map[string]string{
	"aa": "Afar", "ab": "Abkhazian", "ae": "Avestan", "af": "Afrikaans", "ak": "Akan", "am": "Amharic",
	"an": "Aragonese", "ar": "Arabic", "as": "Assamese", "av": "Avaric", "ay": "Aymara", "az": "Azerbaijani",
	"ba": "Bashkir", "be": "Belarusian", "bg": "Bulgarian", "bh": "Bihari", "bi": "Bislama", "bm": "Bambara",
	"bn": "Bengali", "bo": "Tibetan", "br": "Breton", "bs": "Bosnian", "ca": "Catalan", "ce": "Chechen",
	"ch": "Chamorro", "co": "Corsican", "cr": "Cree", "cs": "Czech", "cu": "Church Slavic", "cv": "Chuvash",
	"cy": "Welsh", "da": "Danish", "de": "German", "dv": "Divehi", "dz": "Dzongkha", "ee": "Ewe", "el": "Greek",
	"en": "English", "eo": "Esperanto", "es": "Spanish", "et": "Estonian", "eu": "Basque", "fa": "Persian",
	"ff": "Fulah", "fi": "Finnish", "fj": "Fijian", "fo": "Faroese", "fr": "French", "fy": "Western Frisian",
	"ga": "Irish", "gd": "Scottish Gaelic", "gl": "Galician", "gn": "Guarani", "gu": "Gujarati", "gv": "Manx",
	"ha": "Hausa", "he": "Hebrew", "hi": "Hindi", "ho": "Hiri Motu", "hr": "Croatian", "ht": "Haitian Creole",
	"hu": "Hungarian", "hy": "Armenian", "hz": "Herero", "ia": "Interlingua", "id": "Indonesian", "ie": "Interlingue",
	"ig": "Igbo", "ii": "Sichuan Yi", "ik": "Inupiaq", "io": "Ido", "is": "Icelandic", "it": "Italian",
	"iu": "Inuktitut", "ja": "Japanese", "jv": "Javanese", "ka": "Georgian", "kg": "Kongo", "ki": "Kikuyu",
	"kj": "Kuanyama", "kk": "Kazakh", "kl": "Kalaallisut", "km": "Khmer", "kn": "Kannada", "ko": "Korean",
	"kr": "Kanuri", "ks": "Kashmiri", "ku": "Kurdish", "kv": "Komi", "kw": "Cornish", "ky": "Kyrgyz", "la": "Latin",
	"lb": "Luxembourgish", "lg": "Ganda", "li": "Limburgish", "ln": "Lingala", "lo": "Lao", "lt": "Lithuanian",
	"lu": "Luba-Katanga", "lv": "Latvian", "mg": "Malagasy", "mh": "Marshallese", "mi": "Maori", "mk": "Macedonian",
	"ml": "Malayalam", "mn": "Mongolian", "mr": "Marathi", "ms": "Malay", "mt": "Maltese", "my": "Burmese",
	"na": "Nauru", "nb": "Norwegian Bokmål", "nd": "North Ndebele", "ne": "Nepali", "ng": "Ndonga", "nl": "Dutch",
	"nn": "Norwegian Nynorsk", "no": "Norwegian", "nr": "South Ndebele", "nv": "Navajo", "ny": "Nyanja",
	"oc": "Occitan", "oj": "Ojibwa", "om": "Oromo", "or": "Odia", "os": "Ossetic", "pa": "Punjabi", "pi": "Pali",
	"pl": "Polish", "ps": "Pashto", "pt": "Portuguese", "qu": "Quechua", "rm": "Romansh", "rn": "Rundi",
	"ro": "Romanian", "ru": "Russian", "rw": "Kinyarwanda", "sa": "Sanskrit", "sc": "Sardinian", "sd": "Sindhi",
	"se": "Northern Sami", "sg": "Sango", "sh": "Serbo-Croatian", "si": "Sinhala", "sk": "Slovak", "sl": "Slovenian",
	"sm": "Samoan", "sn": "Shona", "so": "Somali", "sq": "Albanian", "sr": "Serbian", "ss": "Swati",
	"st": "Southern Sotho", "su": "Sundanese", "sv": "Swedish", "sw": "Swahili", "ta": "Tamil", "te": "Telugu",
	"tg": "Tajik", "th": "Thai", "ti": "Tigrinya", "tk": "Turkmen", "tl": "Tagalog", "tn": "Tswana", "to": "Tongan",
	"tr": "Turkish", "ts": "Tsonga", "tt": "Tatar", "tw": "Twi", "ty": "Tahitian", "ug": "Uyghur", "uk": "Ukrainian",
	"ur": "Urdu", "uz": "Uzbek", "ve": "Venda", "vi": "Vietnamese", "vo": "Volapük", "wa": "Walloon", "wo": "Wolof",
	"xh": "Xhosa", "yi": "Yiddish", "yo": "Yoruba", "za": "Zhuang", "zh": "Chinese", "zu": "Zulu",
}

// IsLanguageName reports whether s is a language name or three-letter
// code LanguageCode knows.
func IsLanguageName(s string) bool {
	return languageCodes[strings.ToLower(strings.TrimSpace(s))] != ""
}

// LanguageCode normalises a language ("German", "ger", "de-DE", "de") to
// its ISO 639-1 code; "" for none or undetermined ("und"). A value it
// doesn't know comes back lower-cased as it is.
func LanguageCode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "und" {
		return ""
	}

	if c, ok := languageCodes[s]; ok {
		return c
	}

	if base, _, ok := strings.Cut(s, "-"); ok && len(base) == 2 {
		return base
	}

	return s
}

// MetadataLanguages are the languages (ISO 639-1) the media groups of a
// type fetch metadata texts in: their metadata_language, without groups
// that have none.
func MetadataLanguages(isType uint) []string {
	var langs []string

	RangeSettingsMediaBreak(func(_ string, cfgp *MediaTypeConfig) bool {
		if cfgp.IsType != isType {
			return false
		}

		if l := LanguageCode(cfgp.MetadataLanguage); len(l) == 2 && !slices.Contains(langs, l) {
			langs = append(langs, l)
		}

		return false
	})

	slices.Sort(langs)

	return langs
}

// normalizeMetadataLanguages rewrites the media groups' metadata_language
// to ISO 639-1 codes ("German" to "de"); it reports whether one changed.
func normalizeMetadataLanguages(cfg *MainConfig) bool {
	changed := false

	for _, groups := range [][]MediaTypeConfig{cfg.Media.Movies, cfg.Media.Series, cfg.Media.Books, cfg.Media.AudioBooks, cfg.Media.Music} {
		for i := range groups {
			if l := LanguageCode(groups[i].MetadataLanguage); l != groups[i].MetadataLanguage {
				groups[i].MetadataLanguage = l
				changed = true
			}
		}
	}

	return changed
}
