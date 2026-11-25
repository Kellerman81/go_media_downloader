package apiexternal_v2

// Translation are an entry's texts in one language (ISO 639-1); an empty
// field has no translation.
type Translation struct {
	Language string
	Title    string
	Overview string
	Tagline  string
}

// EpisodeTranslation are an episode's texts in one language.
type EpisodeTranslation struct {
	Season   int
	Episode  int
	Title    string
	Overview string
}
