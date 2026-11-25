package openlibrary

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"golang.org/x/text/unicode/norm"
)

// An author's works: OpenLibrary lists far more works under an author than
// the author wrote (363 for Ken Follett, who wrote about 40): translations
// entered as works of their own, the same book once per spelling or shop
// title ("Edge of Eternity [Paperback] [Jan 01, 2014] Follett, Ken"),
// anthologies (Reader's Digest), boxed sets and shop displays. AuthorWorks
// pages through all of them and keeps the author's own books, once each.

const (
	authorWorksPage     = 100
	authorWorksMaxPages = 10 // 1000 works
	authorWorksMaxCount = 3  // more authors: an anthology
)

var (
	// authorWorksJunk are titles that are no book of their own: collections,
	// sets and shop displays.
	authorWorksJunk = regexp.MustCompile(`(?i)reader'?s digest|select ?editions|box(ed)? set|gift pack|\bdisplay\b|\bpack\b|s/wx|` +
		`\bomnibus\b|condensed|best ?sellers|dumpbin|collection \d|\bbooks? set\b|trilog(y|ía) `)
	// authorWorksMulti are volumes of several titles ("X; Y", "X / Y").
	authorWorksMulti = regexp.MustCompile(`;|/`)
	// authorWorksBrackets are notes in a title: "(French Edition)", "[Paperback]".
	authorWorksBrackets = regexp.MustCompile(`\(.*?\)|\[.*?\]|\{.*?\}`)
	authorWorksArticle  = regexp.MustCompile(`^(the|a|an|der|die|das|ein|eine)\s+|,\s*(the|a|an)$`)
)

// AuthorWorks returns the works of an author: all pages of the author
// search, only works of the author (by the author's OpenLibrary key, three
// authors at most), only titles with an edition in one of languages
// (ISO 639-1; empty: any), without collections and sets, each title once
// (the work with the most editions).
func (p *Provider) AuthorWorks(ctx context.Context, author string, languages []string) ([]apiexternal_v2.BookSearchResult, error) {
	var docs []olSearchDoc

	for page := range authorWorksMaxPages {
		endpoint := fmt.Sprintf("/search.json?author=%s&limit=%d&offset=%d", url.QueryEscape(author), authorWorksPage, page*authorWorksPage)

		var response olSearchResponse
		if err := p.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
			if page == 0 {
				return nil, err
			}

			break
		}

		docs = append(docs, response.Docs...)

		if len(response.Docs) < authorWorksPage {
			break
		}
	}

	return convertSearchResults(selectAuthorWorks(docs, author, languages)), nil
}

// selectAuthorWorks keeps the author's own works from a search, once each.
func selectAuthorWorks(docs []olSearchDoc, author string, languages []string) []olSearchDoc {
	key := authorKeyOf(docs, author)
	if key == "" {
		return nil
	}

	best := map[string]int{} // normalized title -> index in out

	var out []olSearchDoc

	for i := range docs {
		d := &docs[i]
		if !slices.Contains(d.AuthorKey, key) || len(d.AuthorKey) > authorWorksMaxCount {
			continue
		}

		d.Title = cleanWorkTitle(d.Title, author)
		if d.Title == "" || authorWorksJunk.MatchString(d.Title) || authorWorksMulti.MatchString(d.Title) || !inLanguages(d.Language, languages) {
			continue
		}

		norm := normalizeWorkTitle(d.Title)
		if norm == "" {
			continue
		}

		if j, ok := best[norm]; ok {
			if d.EditionCount > out[j].EditionCount {
				out[j] = *d
			}

			continue
		}

		best[norm] = len(out)
		out = append(out, *d)
	}

	slices.SortStableFunc(out, func(a, b olSearchDoc) int { return a.FirstPublishYear - b.FirstPublishYear })

	return out
}

// authorKeyOf is the OpenLibrary key of the author: the one most of the
// works named after the author carry.
func authorKeyOf(docs []olSearchDoc, author string) string {
	counts := map[string]int{}

	for i := range docs {
		for j, name := range docs[i].AuthorName {
			if j < len(docs[i].AuthorKey) && strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(author)) {
				counts[docs[i].AuthorKey[j]]++
			}
		}
	}

	var key string
	for k, n := range counts {
		if n > counts[key] || (n == counts[key] && k < key) {
			key = k
		}
	}

	return key
}

// inLanguages reports whether a work has an edition in one of languages.
func inLanguages(editions, languages []string) bool {
	if len(languages) == 0 {
		return true
	}

	for _, l := range editions {
		if slices.Contains(languages, config.LanguageCode(l)) {
			return true
		}
	}

	return false
}

// cleanWorkTitle drops a shop listing's additions from a title: "by
// <author> ...", notes in brackets and a trailing author name, also as
// "Last, First".
func cleanWorkTitle(title, author string) string {
	title = strings.TrimSpace(title)

	if i := strings.Index(strings.ToLower(title), " by "+strings.ToLower(author)); i > 0 {
		title = strings.TrimSpace(title[:i])
	}

	if t := strings.Join(strings.Fields(authorWorksBrackets.ReplaceAllString(title, " ")), " "); t != "" {
		title = t
	}

	names := []string{strings.ToLower(author)}
	if i := strings.LastIndex(author, " "); i > 0 {
		names = append(names, strings.ToLower(author[i+1:]+", "+author[:i]))
	}

	for _, n := range names {
		if strings.HasSuffix(strings.ToLower(title), " "+n) {
			title = strings.TrimRight(strings.TrimSpace(title[:len(title)-len(n)]), " -,:")
		}
	}

	return title
}

// normalizeWorkTitle is the title works are compared by: lower case,
// without notes in brackets, leading article, accents and punctuation
// ("Die Brücken der Freiheit." and "Die Brucken Der Freiheit" are one).
func normalizeWorkTitle(title string) string {
	t := strings.ToLower(authorWorksBrackets.ReplaceAllString(title, ""))
	t = authorWorksArticle.ReplaceAllString(strings.TrimSpace(t), "")

	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}

		return -1
	}, norm.NFD.String(t))
}
