package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// TestExtrasInLibrary: recommended titles the media group has are marked
// (movies by TMDB id, series by name and year) and open their entry; the
// tab shows cast, videos and reviews.
func TestExtrasInLibrary(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN"), Series: mediaGroup("TV")})

	for _, q := range []string{
		"insert into dbmovies (id, title, moviedb_id) values (1, 'Fight Club', 550)",
		"insert into movies (id, dbmovie_id, listname) values (7, 1, 'EN List')",
		"insert into dbseries (id, seriename, firstaired) values (1, 'Harbor Lights', '2021-02-01')",
		"insert into series (id, dbserie_id, listname) values (9, 1, 'TV List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	movies := []extraTitle{{ID: 550, Title: "Fight Club"}, {ID: 1, Title: "Other"}}
	markInLibrary("movie", []string{"EN List"}, movies)

	if movies[0].EntryID != 7 || movies[1].EntryID != 0 {
		t.Errorf("movies %+v", movies)
	}

	shows := []extraTitle{{ID: 3, Title: "harbor lights", Year: 2021}, {ID: 4, Title: "Harbor Lights", Year: 1999}}
	markInLibrary("series", []string{"TV List"}, shows)

	if shows[0].EntryID != 9 || shows[1].EntryID != 0 {
		t.Errorf("series %+v", shows)
	}

	e := &libExtras{Source: "TMDB",
		Cast:    []apiexternal_v2.CastMember{{Name: "Edward Norton", Character: "Narrator", ProfilePath: "/n.jpg"}},
		Videos:  []apiexternal_v2.Video{{Key: "abc", Name: "Clip", Site: "YouTube", Type: "Clip"}, {Key: "tr1", Name: "Trailer", Site: "YouTube", Type: "Trailer"}},
		Reviews: []tmdb.Review{{Author: "Jo", Rating: 8, Content: "Great."}},
	}

	out := renderComponentToString(renderExtras("movie", e, movies, nil))
	for _, want := range []string{"Edward Norton", "Narrator", "https://image.tmdb.org/t/p/w185/n.jpg", "i.ytimg.com/vi/tr1/",
		"In library", "openLibraryItem(&#34;movie&#34;, 7)", "themoviedb.org/movie/1", "Jo · ★ 8.0", "Great."} {
		if !strings.Contains(out, want) {
			t.Errorf("tab lacks %s", want)
		}
	}

	if strings.Index(out, "vi/tr1/") > strings.Index(out, "vi/abc/") {
		t.Error("the trailer is not listed before the clip")
	}
}

// TestStoredExtras: the More tab shows the cast and related titles the
// import saved, without TMDB; a cast member filters the grid.
func TestStoredExtras(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	for _, q := range []string{
		"insert into dbmovies (id, title, moviedb_id) values (41, 'Stored One', 4100), (42, 'Other One', 4200)",
		"insert into movies (id, dbmovie_id, listname) values (41, 41, 'EN List'), (42, 42, 'EN List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if err := database.SaveMediaExtras("movie", 41, []database.MediaCast{{PersonID: 7, Name: "Paul Walker", Character: "Brian"}},
		[]database.MediaRelated{{TMDB: 4200, Title: "Other One", Year: 2003}}, nil); err != nil {
		t.Fatal(err)
	}

	e := loadExtras(t.Context(), "movie", new(uint64(41)), "en")
	if e == nil || len(e.Cast) != 1 || e.Cast[0].Name != "Paul Walker" || len(e.Recommendations) != 1 || e.Recommendations[0].ID != 4200 {
		t.Fatalf("stored extras: %+v", e)
	}

	var b strings.Builder
	_ = renderExtras("movie", e, e.Recommendations, nil).Render(&b)

	if !strings.Contains(b.String(), `libShowCast(&#34;movie&#34;, &#34;Paul Walker&#34;)`) {
		t.Errorf("cast not clickable:\n%s", b.String())
	}

	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&cast=paul+walker"); !strings.Contains(out, "Stored One") ||
		strings.Contains(out, "Other One") || !strings.Contains(out, "libShowCast(&#39;&#39;, &#39;&#39;)") {
		t.Errorf("cast filter:\n%.3000s", out)
	}

	// The search finds the cast too.
	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&q=walker"); !strings.Contains(out, "Stored One") || strings.Contains(out, "Other One") {
		t.Errorf("cast search:\n%.3000s", out)
	}
}

// TestLiveExtrasSaved: a cast the More tab loads from TMDB (title not
// refreshed since) is saved, so the cast filter finds the title; a name
// no saved cast has explains why nothing shows.
func TestLiveExtrasSaved(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/credits"):
			fmt.Fprint(w, `{"cast":[{"id":58225,"name":"Ross Patterson","character":"Himself","profile_path":"/r.jpg"}]}`)
		case strings.HasSuffix(r.URL.Path, "/recommendations"):
			fmt.Fprint(w, `{"results":[{"id":77,"title":"Recommended"}]}`)
		default:
			fmt.Fprint(w, `{"results":[]}`)
		}
	}))
	defer srv.Close()

	providers.SetTMDB(tmdb.NewProviderWithConfig(base.ClientConfig{Name: "tmdb", BaseURL: srv.URL + "/3"}, "key"))
	t.Cleanup(func() { providers.SetTMDB(nil) })

	for _, q := range []string{
		"insert into dbmovies (id, title, moviedb_id) values (51, 'Live One', 5100), (52, 'Unrelated', 5200)",
		"insert into movies (id, dbmovie_id, listname) values (51, 51, 'EN List'), (52, 52, 'EN List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if e := loadExtras(t.Context(), "movie", new(uint64(51)), "en"); e == nil || len(e.Cast) != 1 {
		t.Fatalf("live extras: %+v", e)
	}

	if c := database.MediaCastOf("movie", 51); len(c) != 1 || c[0].PersonID != 58225 || len(database.MediaRelatedOf("movie", 51, database.RelationRecommended)) != 1 {
		t.Fatalf("not saved: %+v", c)
	}

	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&cast=Ross+Patterson"); !strings.Contains(out, "Live One") || strings.Contains(out, "Unrelated") {
		t.Errorf("cast filter after live load:\n%.3000s", out)
	}

	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&cast=Nobody+Known"); !strings.Contains(out, "The cast of a title is saved") ||
		!strings.Contains(out, "libShowCast(&#39;&#39;, &#39;&#39;)") || !strings.Contains(out, "Titles with Nobody Known") {
		t.Errorf("empty cast filter:\n%.3000s", out)
	}
}
