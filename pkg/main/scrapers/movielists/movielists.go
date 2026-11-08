// Package movielists reads movie lists from Rotten Tomatoes and Letterboxd
// pages: title and year of every entry, plus the IMDb ID where the page
// links one. Matching titles to IMDb IDs is up to the caller.
package movielists

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/scrapers/sitethrottle"
	"github.com/goccy/go-json"
)

// Movie is one list entry.
type Movie struct {
	Title  string
	Year   int
	ImdbID string // set when the page links the movie's IMDb page
}

const maxBody = 8 << 20

// pageInterval spaces the page requests to one site.
var pageInterval = 2 * time.Second

var (
	errNoEntries = errors.New("no movies found on the page - is it a list page?")
	reImdbLink   = regexp.MustCompile(`imdb\.com/title/(tt\d{7,9})`)
	reNameYear   = regexp.MustCompile(`^(.*?)\s*\((\d{4})\)\s*$`)
	// Rotten Tomatoes pages describe their list as JSON-LD.
	reLDJSON = regexp.MustCompile(`(?s)<script type="application/ld\+json">(.*?)</script>`)
	// Editorial guides (editorial.rottentomatoes.com/guide/...).
	reRTEditorial = regexp.MustCompile(
		`(?s)<a class="meta-title"[^>]*>(.*?)</a>\s*<span class="meta-year">\s*\((\d{4})\)`)
	// Letterboxd posters: data-item-name="Black Bag (2025)".
	reLBItem = regexp.MustCompile(`data-item-name="([^"]+)"`)
)

// Client fetches list pages.
type Client struct {
	HTTP      *http.Client
	UserAgent string
}

// NewClient returns a client with a 30 second timeout.
func NewClient(userAgent string) *Client {
	if userAgent == "" {
		userAgent = "go_media_downloader"
	}

	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}, UserAgent: userAgent}
}

func (c *Client) get(ctx context.Context, pageURL string) (string, int, error) {
	u, err := url.Parse(pageURL)
	if err != nil || u.Host == "" {
		return "", 0, fmt.Errorf("invalid list URL %q", pageURL)
	}

	sitethrottle.Wait(ctx, u.Host, pageInterval)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", 0, err
	}

	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", resp.StatusCode, err
	}

	if resp.StatusCode != http.StatusOK {
		return string(body), resp.StatusCode, fmt.Errorf("%s: HTTP %d", pageURL, resp.StatusCode)
	}

	return string(body), resp.StatusCode, nil
}

// RottenTomatoes reads a Rotten Tomatoes browse page
// (www.rottentomatoes.com/browse/...) or editorial guide
// (editorial.rottentomatoes.com/guide/...). Browse pages show about 30
// movies; "?page=N" in the URL shows more.
func (c *Client) RottenTomatoes(ctx context.Context, pageURL string) ([]Movie, error) {
	page, _, err := c.get(ctx, pageURL)
	if err != nil {
		return nil, err
	}

	movies := ParseRottenTomatoes(page)
	if len(movies) == 0 {
		return nil, errNoEntries
	}

	return movies, nil
}

type ldMovie struct {
	Type        string `json:"@type"`
	Name        string `json:"name"`
	DateCreated string `json:"dateCreated"`
	SameAs      any    `json:"sameAs"`
}

type ldList struct {
	Type            string          `json:"@type"`
	ItemListElement json.RawMessage `json:"itemListElement"`
}

// ldItems collects the Movie entries of a (possibly nested) ItemList.
func ldItems(raw json.RawMessage, out *[]ldMovie) {
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		var nested ldList
		if json.Unmarshal(raw, &nested) == nil && nested.ItemListElement != nil {
			ldItems(nested.ItemListElement, out)
		}

		return
	}

	for _, it := range items {
		var m ldMovie
		if json.Unmarshal(it, &m) == nil && m.Type == "Movie" && m.Name != "" {
			*out = append(*out, m)
			continue
		}

		var nested ldList
		if json.Unmarshal(it, &nested) == nil && nested.ItemListElement != nil {
			ldItems(nested.ItemListElement, out)
		}
	}
}

// cleanTitle unescapes a title and normalizes its spaces (sites use
// non-breaking ones), so it compares equal to the IMDb title.
func cleanTitle(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(s)), " ")
}

func year(s string) int {
	if len(s) >= 4 {
		if y, err := strconv.Atoi(s[:4]); err == nil {
			return y
		}
	}

	return 0
}

// ParseRottenTomatoes extracts the movies of a Rotten Tomatoes page.
func ParseRottenTomatoes(page string) []Movie {
	var out []Movie

	for _, m := range reLDJSON.FindAllStringSubmatch(page, -1) {
		var list ldList
		if json.Unmarshal([]byte(m[1]), &list) != nil || list.Type != "ItemList" {
			continue
		}

		var items []ldMovie
		ldItems(list.ItemListElement, &items)

		for _, it := range items {
			mv := Movie{Title: cleanTitle(it.Name), Year: year(it.DateCreated)}
			if same, ok := it.SameAs.(string); ok {
				if id := reImdbLink.FindStringSubmatch(same); id != nil {
					mv.ImdbID = id[1]
				}
			}

			out = append(out, mv)
		}
	}

	if len(out) > 0 {
		return out
	}

	for _, m := range reRTEditorial.FindAllStringSubmatch(page, -1) {
		y, _ := strconv.Atoi(m[2])
		out = append(out, Movie{Title: cleanTitle(m[1]), Year: y})
	}

	return out
}

// Letterboxd reads a Letterboxd watchlist or list (letterboxd.com/<user>/
// watchlist/ or letterboxd.com/<user>/list/<slug>/), following its pages up
// to maxPages (0: 50).
func (c *Client) Letterboxd(ctx context.Context, listURL string, maxPages int) ([]Movie, error) {
	if maxPages <= 0 {
		maxPages = 50
	}

	base := strings.TrimRight(strings.SplitN(listURL, "?", 2)[0], "/") + "/"
	base = regexp.MustCompile(`page/\d+/$`).ReplaceAllString(base, "")

	var (
		out  []Movie
		seen = map[string]bool{}
	)

	for p := 1; p <= maxPages; p++ {
		pageURL := base
		if p > 1 {
			pageURL += "page/" + strconv.Itoa(p) + "/"
		}

		page, status, err := c.get(ctx, pageURL)
		if err != nil {
			if p > 1 && status == http.StatusNotFound {
				break
			}

			return out, err
		}

		movies := ParseLetterboxd(page)

		added := 0

		for _, m := range movies {
			key := m.Title + "|" + strconv.Itoa(m.Year)
			if !seen[key] {
				seen[key] = true

				out = append(out, m)
				added++
			}
		}

		if added == 0 {
			break
		}
	}

	if len(out) == 0 {
		return nil, errNoEntries
	}

	return out, nil
}

// ParseLetterboxd extracts the films of one Letterboxd list page. An IMDb
// link inside a film's own list item is used as its ID; Letterboxd list
// pages normally have none, so title and year are what the caller matches.
func ParseLetterboxd(page string) []Movie {
	var out []Movie

	items := strings.Split(page, "<li class=\"griditem")
	if len(items) == 1 {
		items = []string{"", page}
	}

	for _, item := range items[1:] {
		for _, m := range reLBItem.FindAllStringSubmatch(item, -1) {
			name := cleanTitle(m[1])

			mv := Movie{Title: name}
			if ny := reNameYear.FindStringSubmatch(name); ny != nil {
				mv.Title = ny[1]
				mv.Year, _ = strconv.Atoi(ny[2])
			}

			if id := reImdbLink.FindStringSubmatch(item); id != nil {
				mv.ImdbID = id[1]
			}

			out = append(out, mv)
		}
	}

	return out
}
