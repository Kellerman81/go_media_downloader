package movielists

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

const rtBrowse = `<html><script type="application/ld+json">{"@context":"http://schema.org","@type":"ItemList",
"name":"Movies in Theaters","itemListElement":{"@type":"ItemList","name":"Movies","itemListElement":[
{"@type":"Movie","position":1,"name":"Birds of War","dateCreated":"2026","url":"https://www.rottentomatoes.com/m/birds_of_war"},
{"@type":"Movie","position":2,"name":"Tom &amp; Jerry\u00a0","dateCreated":"2025-07-01"}]}}</script></html>`

const rtEditorial = `<tr><td>1.</td><td><span class="meta-data-wrapper">
<a class="meta-title" href="https://www.rottentomatoes.com/m/the_godfather">The Godfather</a> <span class="meta-year">(1972)</span>
</span></td></tr><tr><td><a class="meta-title" href="https://www.rottentomatoes.com/m/x">L.A. Confidential</a>
<span class="meta-year">(1997)</span></td></tr>`

func lbPage(names ...string) string {
	page := `<ul class="grid">`
	for _, n := range names {
		page += `<li class="griditem"><div class="react-component" data-item-name="` + n + `" data-item-slug="x"></div></li>`
	}

	return page + "</ul>"
}

func TestParseRottenTomatoes(t *testing.T) {
	want := []Movie{{"Birds of War", 2026, ""}, {"Tom & Jerry", 2025, ""}}
	if got := ParseRottenTomatoes(rtBrowse); !slices.Equal(got, want) {
		t.Errorf("browse: %v", got)
	}

	want = []Movie{{"The Godfather", 1972, ""}, {"L.A. Confidential", 1997, ""}}
	if got := ParseRottenTomatoes(rtEditorial); !slices.Equal(got, want) {
		t.Errorf("editorial: %v", got)
	}
}

func TestParseLetterboxd(t *testing.T) {
	page := lbPage("Black Bag (2025)", "Am&eacute;lie (2001)", "No Year") +
		`<li class="griditem"><div data-item-name="Heat (1995)"></div><a href="https://www.imdb.com/title/tt0113277/">IMDb</a></li>`

	want := []Movie{{"Black Bag", 2025, ""}, {"Amélie", 2001, ""}, {"No Year", 0, ""}, {"Heat", 1995, "tt0113277"}}
	if got := ParseLetterboxd(page); !slices.Equal(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestLetterboxdPages(t *testing.T) {
	old := pageInterval
	pageInterval = 0
	t.Cleanup(func() { pageInterval = old })

	var paths []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)

		switch r.URL.Path {
		case "/u/watchlist/":
			fmt.Fprint(w, lbPage("A (2001)", "B (2002)"))
		case "/u/watchlist/page/2/":
			fmt.Fprint(w, lbPage("C (2003)"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	got, err := NewClient("").Letterboxd(context.Background(), srv.URL+"/u/watchlist/page/2/?x=1", 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 3 || got[2].Title != "C" || !slices.Equal(paths, []string{"/u/watchlist/", "/u/watchlist/page/2/", "/u/watchlist/page/3/"}) {
		t.Errorf("got %v via %v", got, paths)
	}
}
