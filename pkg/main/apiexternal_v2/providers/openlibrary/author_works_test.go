package openlibrary

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
)

// TestSelectAuthorWorks: of an author search only the author's own works
// stay, each title once, with an edition in the wanted languages.
func TestSelectAuthorWorks(t *testing.T) {
	ken := func(title string, year, editions int, langs ...string) olSearchDoc {
		return olSearchDoc{Title: title, FirstPublishYear: year, EditionCount: editions, Language: langs,
			AuthorName: []string{"Ken Follett"}, AuthorKey: []string{"OL229268A"}}
	}

	other := ken("A Book About Follett", 2001, 3, "eng")
	other.AuthorName, other.AuthorKey = []string{"Someone Else"}, []string{"OL1A"}

	anthology := ken("Thrillers", 1999, 2, "eng")
	anthology.AuthorName = []string{"Ken Follett", "A", "B", "C"}
	anthology.AuthorKey = []string{"OL229268A", "OL2A", "OL3A", "OL4A"}

	docs := []olSearchDoc{
		ken("The Pillars of the Earth", 1989, 111, "fre", "ger"),
		ken("Pillars of the Earth (1/3)", 1989, 2, "eng"),
		ken("Never", 2021, 4, "eng"),
		ken("Never", 2021, 1),
		ken("Edge of Eternity [Paperback] [Jan 01, 2014] Follett, Ken", 2014, 1, "eng"),
		ken("Edge of Eternity", 2014, 37, "rus", "ger"),
		ken("Triple by Ken Follett (November 15,2012)", 2012, 1, "eng"),
		ken("Les Piliers de la terre - Tome 1", 1999, 1, "fre"),
		ken("Reader's Digest Condensed Books--Volume 1 1996", 1996, 4, "eng"),
		ken("Century Trilogy Boxed Set", 2014, 1, "eng"),
		ken("Eye of the Needle; Whiteout", 2008, 1, "eng"),
		ken("Die Nadel", 1979, 1, "ger"),
		other,
		anthology,
	}

	got := selectAuthorWorks(docs, "Ken Follett", []string{"en", "de"})

	var titles []string
	for _, d := range got {
		titles = append(titles, d.Title+"/"+strconv.Itoa(d.EditionCount))
	}

	want := []string{"Die Nadel/1", "The Pillars of the Earth/111", "Triple/1", "Edge of Eternity/37", "Never/4"}
	if !slices.Equal(titles, want) {
		t.Errorf("works:\n got %v\nwant %v", titles, want)
	}

	if got := selectAuthorWorks(docs, "Nobody", nil); got != nil {
		t.Errorf("unknown author: %v", got)
	}
}

// TestAuthorWorksPages: all pages of the search are read, not just the
// first 100 hits.
func TestAuthorWorksPages(t *testing.T) {
	var offsets []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offsets = append(offsets, r.URL.Query().Get("offset"))
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		n := authorWorksPage
		if off > 0 {
			n = 30
		}

		docs := make([]olSearchDoc, n)
		for i := range docs {
			docs[i] = olSearchDoc{Key: "/works/OL" + strconv.Itoa(off+i) + "W", Title: "Book " + strconv.Itoa(off+i),
				FirstPublishYear: 1970 + i, EditionCount: 1, Language: []string{"eng"},
				AuthorName: []string{"Ken Follett"}, AuthorKey: []string{"OL229268A"}}
		}

		_ = json.NewEncoder(w).Encode(olSearchResponse{NumFound: authorWorksPage + 30, Docs: docs})
	}))
	defer srv.Close()

	p := NewProviderWithConfig(base.ClientConfig{BaseURL: srv.URL, RateLimitCalls: 100, RateLimitSeconds: 1})

	books, err := p.AuthorWorks(context.Background(), "Ken Follett", []string{"en"})
	if err != nil {
		t.Fatal(err)
	}

	if len(books) != authorWorksPage+30 || !slices.Equal(offsets, []string{"0", "100"}) {
		t.Errorf("%d books, requests at offsets %v", len(books), offsets)
	}
}
