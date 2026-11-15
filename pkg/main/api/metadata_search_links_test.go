package api

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"maragu.dev/gomponents"
)

func renderNode(t *testing.T, n gomponents.Node) string {
	t.Helper()

	var b strings.Builder
	if err := n.Render(&b); err != nil {
		t.Fatal(err)
	}

	return b.String()
}

func TestMetadataResultCardsCoversAndLinks(t *testing.T) {
	for name, tc := range map[string]struct {
		card gomponents.Node
		want []string
	}{
		"movie": {
			createMovieResultCard(apiexternal_v2.MovieSearchResult{ID: 27205, Title: "Inception", PosterPath: "/p.jpg", ProviderName: "tmdb", IMDbID: "tt1375666"}, false),
			[]string{`src="https://image.tmdb.org/t/p/w154/p.jpg"`, `href="https://www.themoviedb.org/movie/27205"`, `href="https://www.imdb.com/title/tt1375666"`},
		},
		"series": {
			createSeriesResultCard(apiexternal_v2.SeriesSearchResult{ID: 81189, Name: "Breaking Bad", PosterPath: "https://artworks.thetvdb.com/p.jpg", ProviderName: "tvdb", TVDbID: 81189}, false),
			[]string{`src="https://artworks.thetvdb.com/p.jpg"`, `href="https://thetvdb.com/dereferrer/series/81189"`},
		},
		"album": {
			createMusicResultCard(&apiexternal_v2.ReleaseSearchResult{Title: "The Wall", ReleaseGroupID: "rg", DeezerID: 5, CoverURL: "https://c.example/a.jpg",
				Artists: []apiexternal_v2.ArtistRef{{Name: "Pink Floyd"}}}, false),
			[]string{`src="https://c.example/a.jpg"`, `href="https://musicbrainz.org/release-group/rg"`, `href="https://www.deezer.com/album/5"`, `last.fm/music/Pink+Floyd/The+Wall`},
		},
		"book": {
			createBookResultCard(apiexternal_v2.BookSearchResult{ID: "OL1W", Title: "Rumo", ISBN13: "9783492045698", Authors: []string{"Walter Moers"}}, false),
			[]string{`href="https://openlibrary.org/works/OL1W"`, `goodreads.com/search?q=9783492045698`},
		},
		"audiobook": {
			createAudiobookResultCard(apiexternal_v2.AudiobookSearchResult{ASIN: "B1", Title: "Rumo", CoverURL: "javascript:x"}, false),
			[]string{`href="https://www.audible.com/pd/B1"`},
		},
	} {
		out := renderNode(t, tc.card)
		for _, w := range tc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s card lacks %s\n%s", name, w, out)
			}
		}
	}

	// Only http(s) covers are shown.
	if strings.Contains(renderNode(t, createAudiobookResultCard(apiexternal_v2.AudiobookSearchResult{CoverURL: "javascript:x"}, false)), "<img") {
		t.Error("non-http cover rendered")
	}
}
