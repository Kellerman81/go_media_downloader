package openlibrary

import (
	"context"
	"net/url"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// findBookFields: the work and the edition that matched the search (its
// title is the one the file is named by, often a translation).
const findBookFields = "key,title,subtitle,author_name,author_key,first_publish_year,cover_i,isbn,edition_count,language," +
	"editions,editions.title,editions.language"

// olFindDoc is a search hit with the edition that matched.
type olFindDoc struct {
	olSearchDoc
	Editions struct {
		Docs []struct {
			Title string `json:"title"`
		} `json:"docs"`
	} `json:"editions"`
}

// FindBook returns the work a book file names, by its title (also an
// edition's: "Die Säulen der Erde" is "The Pillars of the Earth") and
// author; nil when no hit has the author and the title.
func (p *Provider) FindBook(ctx context.Context, title, author string) (*apiexternal_v2.BookSearchResult, error) {
	if strings.TrimSpace(title) == "" || strings.TrimSpace(author) == "" {
		return nil, nil
	}

	endpoint := "/search.json?title=" + url.QueryEscape(title) + "&author=" + url.QueryEscape(author) +
		"&limit=10&fields=" + url.QueryEscape(findBookFields)

	var response struct {
		Docs []olFindDoc `json:"docs"`
	}
	if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, err
	}

	i := pickFoundBook(response.Docs, title, author)
	if i == -1 {
		return nil, nil
	}

	res := convertSearchResults([]olSearchDoc{response.Docs[i].olSearchDoc})

	return &res[0], nil
}

// pickFoundBook is the first hit by the author whose title or matching
// edition's title is the title; failing that, the first one where it
// contains it ("Death of a Salesman - Tod eines Handlungsreisenden").
// Compared without case, accents, articles and punctuation.
func pickFoundBook(docs []olFindDoc, title, author string) int {
	want, who := normalizeWorkTitle(title), normalizeWorkTitle(author)
	if want == "" {
		return -1
	}

	contains := -1

	for i := range docs {
		if !hasAuthor(docs[i].AuthorName, who) {
			continue
		}

		titles := []string{docs[i].Title}
		for _, e := range docs[i].Editions.Docs {
			titles = append(titles, e.Title)
		}

		for _, t := range titles {
			n := normalizeWorkTitle(t)
			if n == want {
				return i
			}

			if contains == -1 && len([]rune(want)) >= 8 && strings.Contains(n, want) {
				contains = i
			}
		}
	}

	return contains
}

// hasAuthor reports whether one of names is who (normalized): "Ake
// Edwardson" is "Åke Edwardson".
func hasAuthor(names []string, who string) bool {
	for _, n := range names {
		if normalizeWorkTitle(n) == who {
			return true
		}
	}

	return false
}
