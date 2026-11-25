package openlibrary

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// workTitlesLimit is how many editions of a work are read (one request):
// the most edited works have a few hundred.
const workTitlesLimit = 1000

// WorkTitles returns the title of a work in each of languages (ISO 639-1)
// it has an edition in: "de" -> "Die Säulen der Erde".
func (p *Provider) WorkTitles(ctx context.Context, workID string, languages []string) (map[string]string, error) {
	endpoint := fmt.Sprintf("/works/%s/editions.json?limit=%d", url.PathEscape(extractOLID(workID)), workTitlesLimit)

	var response olEditionsResponse
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	return editionTitles(response.Entries, languages), nil
}

// editionTitles picks a title per language from a work's editions: the one
// most editions in the language carry. Spellings of one title count
// together ("Die Brucken Der Freiheit", "Die Brücken der Freiheit."); of
// them the most common wins, on a tie the one with umlauts and accents
// (ASCII spellings are often typed without them). A mistagged edition
// ("les pilliers de la terre" as German) loses to the real ones.
func editionTitles(entries []olEditionEntry, languages []string) map[string]string {
	type spelling struct {
		title string
		n     int
	}

	counts := map[string]map[string][]spelling{} // language -> normalized title -> spellings

	for i := range entries {
		title := strings.TrimRight(strings.TrimSpace(entries[i].Title), " .,;:/")
		slug := normalizeWorkTitle(title)

		if slug == "" {
			continue
		}

		for _, l := range entries[i].Languages {
			lang := config.LanguageCode(strings.TrimPrefix(l.Key, "/languages/"))
			if !slices.Contains(languages, lang) {
				continue
			}

			if counts[lang] == nil {
				counts[lang] = map[string][]spelling{}
			}

			sp := counts[lang][slug]

			found := false

			for j := range sp {
				if sp[j].title == title {
					sp[j].n++
					found = true

					break
				}
			}

			if !found {
				sp = append(sp, spelling{title: title, n: 1})
			}

			counts[lang][slug] = sp
		}
	}

	out := make(map[string]string, len(counts))

	for lang, slugs := range counts {
		bestTotal, best := 0, ""

		for _, sp := range slugs {
			total, pick := 0, sp[0]

			for _, s := range sp {
				total += s.n
				if s.n > pick.n || (s.n == pick.n && nonASCII(s.title) > nonASCII(pick.title)) {
					pick = s
				}
			}

			if total > bestTotal || (total == bestTotal && pick.title < best) {
				bestTotal, best = total, pick.title
			}
		}

		out[lang] = best
	}

	return out
}

func nonASCII(s string) int {
	n := 0

	for _, r := range s {
		if r > unicode.MaxASCII {
			n++
		}
	}

	return n
}
