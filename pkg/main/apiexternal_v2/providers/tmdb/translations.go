package tmdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// tmdbTranslations is /movie/{id}/translations and /tv/{id}/translations:
// a movie's texts are title, a series' name.
type tmdbTranslations struct {
	Translations []struct {
		Country  string `json:"iso_3166_1"`
		Language string `json:"iso_639_1"`
		Data     struct {
			Title    string `json:"title"`
			Name     string `json:"name"`
			Overview string `json:"overview"`
			Tagline  string `json:"tagline"`
		} `json:"data"`
	} `json:"translations"`
}

// GetMovieTranslations are a movie's title, overview and tagline in every
// language TMDB has them in.
func (p *Provider) GetMovieTranslations(ctx context.Context, id int) ([]apiexternal_v2.Translation, error) {
	return p.getTranslations(ctx, fmt.Sprintf("%s/movie/%d/translations", p.baseURL, id))
}

// GetSeriesTranslations are a series' name, overview and tagline in every
// language TMDB has them in.
func (p *Provider) GetSeriesTranslations(ctx context.Context, id int) ([]apiexternal_v2.Translation, error) {
	return p.getTranslations(ctx, fmt.Sprintf("%s/tv/%d/translations", p.baseURL, id))
}

func (p *Provider) getTranslations(ctx context.Context, url string) ([]apiexternal_v2.Translation, error) {
	var response tmdbTranslations
	if err := p.makeRequest(ctx, url, &response); err != nil {
		return nil, err
	}

	return pickTranslations(&response), nil
}

// pickTranslations keeps one translation per language: TMDB lists one per
// country (de-DE, de-AT); the language's own country wins, else the one
// with the most text.
func pickTranslations(response *tmdbTranslations) []apiexternal_v2.Translation {
	type pick struct {
		tr    apiexternal_v2.Translation
		score int
	}

	byLang := map[string]*pick{}
	order := []string{}

	home := map[string]string{"en": "US", "ja": "JP", "ko": "KR", "zh": "CN", "sv": "SE", "da": "DK", "cs": "CZ", "el": "GR", "he": "IL", "hi": "IN", "ar": "SA"}

	for _, t := range response.Translations {
		lang := strings.ToLower(t.Language)
		if lang == "" {
			continue
		}

		tr := apiexternal_v2.Translation{Language: lang, Title: t.Data.Title, Overview: t.Data.Overview, Tagline: t.Data.Tagline}
		if tr.Title == "" {
			tr.Title = t.Data.Name
		}

		score := len(tr.Title) + len(tr.Overview) + len(tr.Tagline)
		if score == 0 {
			continue
		}

		country := home[lang]
		if country == "" {
			country = strings.ToUpper(lang)
		}

		if strings.EqualFold(t.Country, country) {
			score += 1 << 20
		}

		if cur, ok := byLang[lang]; !ok {
			byLang[lang] = &pick{tr, score}
			order = append(order, lang)
		} else if score > cur.score {
			*cur = pick{tr, score}
		}
	}

	out := make([]apiexternal_v2.Translation, 0, len(order))
	for _, l := range order {
		out = append(out, byLang[l].tr)
	}

	return out
}
