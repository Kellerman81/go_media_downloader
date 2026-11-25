package tvdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// GetSeriesTranslation is a series' name and overview in language. TVDB
// answers in English where it has no translation; such texts are left out.
func (p *Provider) GetSeriesTranslation(ctx context.Context, id int, language string) (apiexternal_v2.Translation, error) {
	var response tvdbV2SeriesResponse
	if err := p.getWithLanguage(ctx, fmt.Sprintf("/series/%d", id), language, &response); err != nil {
		return apiexternal_v2.Translation{}, err
	}

	tr := apiexternal_v2.Translation{Language: language}
	if response.Data.Language == "" || strings.EqualFold(response.Data.Language, language) {
		tr.Title, tr.Overview = response.Data.SeriesName, response.Data.Overview
	}

	return tr, nil
}

// GetEpisodeTranslations are the names and overviews of a series' episodes
// in language, each field only where TVDB has it in that language (its
// "language" map names the language of each field).
func (p *Provider) GetEpisodeTranslations(ctx context.Context, id int, language string) ([]apiexternal_v2.EpisodeTranslation, error) {
	out := make([]apiexternal_v2.EpisodeTranslation, 0, 100)

	for page := 1; page <= 50; page++ {
		var response tvdbV2EpisodesResponse
		if err := p.getWithLanguage(ctx, fmt.Sprintf("/series/%d/episodes?page=%d", id, page), language, &response); err != nil {
			return nil, err
		}

		for i := range response.Data {
			ep := &response.Data[i]
			in := func(field string) bool {
				l, ok := ep.Language[field]
				return !ok || strings.EqualFold(l, language)
			}

			tr := apiexternal_v2.EpisodeTranslation{Season: ep.AiredSeason, Episode: ep.AiredEpisodeNumber}
			if in("episodeName") {
				tr.Title = ep.EpisodeName
			}

			if in("overview") {
				tr.Overview = ep.Overview
			}

			if tr.Title != "" || tr.Overview != "" {
				out = append(out, tr)
			}
		}

		if response.Links.Next == nil {
			break
		}
	}

	return out, nil
}
