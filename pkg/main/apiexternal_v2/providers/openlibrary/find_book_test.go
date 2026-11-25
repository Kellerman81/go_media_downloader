package openlibrary

import "testing"

// TestPickFoundBook: the hit by the author whose title or matching
// edition's title is the file's; one that only contains it when nothing
// is equal; none without the author.
func TestPickFoundBook(t *testing.T) {
	doc := func(title, author string, editions ...string) olFindDoc {
		d := olFindDoc{olSearchDoc: olSearchDoc{Title: title, AuthorName: []string{author}}}
		for _, e := range editions {
			d.Editions.Docs = append(d.Editions.Docs, struct {
				Title string `json:"title"`
			}{e})
		}

		return d
	}

	docs := []olFindDoc{
		doc("Death of a Salesman - Tod eines Handlungsreisenden von Arthur Miller.", "Arthur Miller"),
		doc("The Pillars of the Earth", "Someone Else", "Die Säulen der Erde"),
		doc("The Pillars of the Earth", "Ken Follett", "Die Saulen der Erde."),
		doc("Das vertauschte Gesicht", "Åke Edwardson"),
	}

	for _, tc := range []struct {
		title, author string
		want          int
	}{
		{"Die Säulen der Erde", "Ken Follett", 2},
		{"Das vertauschte Gesicht", "Ake Edwardson", 3},
		{"Tod eines Handlungsreisenden", "Arthur Miller", 0},
		{"Die Säulen der Erde", "Nobody", -1},
		{"Tod", "Arthur Miller", -1},
	} {
		if got := pickFoundBook(docs, tc.title, tc.author); got != tc.want {
			t.Errorf("%s / %s: %d, want %d", tc.title, tc.author, got, tc.want)
		}
	}
}
