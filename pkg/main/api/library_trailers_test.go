package api

import (
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// TestPickTrailer: the group's language first, else English; trailers
// before teasers, official and newer ones first; other sites and
// languages never.
func TestPickTrailer(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 1, d, 0, 0, 0, 0, time.UTC) }
	videos := []apiexternal_v2.Video{
		{Key: "en-trailer", Site: "YouTube", Type: "Trailer", Language: "en", Official: true, PublishedAt: day(1)},
		{Key: "de-teaser", Site: "YouTube", Type: "Teaser", Language: "de", PublishedAt: day(2)},
		{Key: "de-old", Site: "YouTube", Type: "Trailer", Language: "de", Official: true, PublishedAt: day(1)},
		{Key: "de-new", Site: "YouTube", Type: "Trailer", Language: "de", Official: true, PublishedAt: day(5)},
		{Key: "de-vimeo", Site: "Vimeo", Type: "Trailer", Language: "de", Official: true, PublishedAt: day(9)},
		{Key: "fr", Site: "YouTube", Type: "Trailer", Language: "fr", Official: true, PublishedAt: day(9)},
		{Key: "de-clip", Site: "YouTube", Type: "Clip", Language: "de", Official: true, PublishedAt: day(9)},
	}

	for _, tc := range []struct{ lang, want string }{{"de", "de-new"}, {"en", "en-trailer"}, {"es", "en-trailer"}} {
		got := pickTrailer(videos, tc.lang)
		if got == nil || got.Key != tc.want {
			t.Errorf("%s: %+v, want %s", tc.lang, got, tc.want)
		}
	}

	if got := pickTrailer(videos[4:6], "de"); got != nil {
		t.Errorf("only Vimeo and French: %+v", got)
	}
}

// TestListLanguage: a list's group language as ISO 639-1 - from a code, a
// region tag or a language name (as config.toml has them) - English
// without one.
func TestListLanguage(t *testing.T) {
	groups := []config.MediaTypeConfig{}
	for name, lang := range map[string]string{"A": "de-DE", "B": "German", "C": "ger", "D": "English", "E": "", "F": "Klingon"} {
		g := mediaGroup(name)
		g[0].MetadataLanguage = lang
		groups = append(groups, g...)
	}

	useMediaConfig(t, config.MediaConfig{Movies: groups})

	for list, want := range map[string]string{"a list": "de", "B List": "de", "C List": "de", "D List": "en", "E List": "en", "F List": "en"} {
		if got := listLanguage(list); got != want {
			t.Errorf("%s: %q, want %q", list, got, want)
		}
	}
}
