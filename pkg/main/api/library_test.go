package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

func libraryTestData(t *testing.T) {
	t.Helper()

	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN"), Series: mediaGroup("X")})

	q := []string{"insert into dbseries (id, seriename, overview) values (1, 'Site Show', 'Show plot')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'X List')",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (1, 1, 'S01E01', 'Pilot', '1', '1')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id, missing) values (1, 1, 1, 1, 1)"}

	for i := 1; i <= 50; i++ {
		q = append(q,
			fmt.Sprintf("insert into dbmovies (id, title, year, overview) values (%d, 'Movie %02d', %d, 'Plot %d')", i, i, 2000+i, i),
			fmt.Sprintf("insert into movies (id, dbmovie_id, listname, missing) values (%d, %d, 'EN List', %d)", i, i, i%2))
	}

	for _, s := range q {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}
}

func libGet(t *testing.T, h gin.HandlerFunc, query string) (int, string) {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
	h(c)

	return w.Code, w.Body.String()
}

func TestLibraryGrid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	groups := libraryGroups()
	if len(groups) != 2 || groups[0].Name != "movie_EN" || groups[1].Name != "serie_X" {
		t.Fatalf("groups = %+v", groups)
	}

	// Without a filter only downloaded entries show; none are here.
	if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN"); !strings.Contains(out, "Nothing found") {
		t.Errorf("default filter isn't downloaded:\n%.300s", out)
	}

	// The default page size (100) holds all 50; so does "all".
	for _, size := range []string{"", "&size=all"} {
		if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all"+size); strings.Count(out, `class="lib-card"`) != 50 ||
			strings.Contains(out, "Page 1 of") || !strings.Contains(out, `id="libPageSize"`) {
			t.Errorf("one page%s: %d cards", size, strings.Count(out, `class="lib-card"`))
		}
	}

	_, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&size=48")
	if !strings.Contains(out, "50 entries") || strings.Count(out, `class="lib-card"`) != 48 ||
		!strings.Contains(out, "Page 1 of 2") || !strings.Contains(out, `src="/api/admin/library/cover/movie/1"`) {
		t.Errorf("first page:\n%.600s", out)
	}

	_, out = libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&page=2&sort=year&size=48")
	if strings.Count(out, `class="lib-card"`) != 2 || !strings.Contains(out, "Movie 02") {
		t.Errorf("second page by year lacks the oldest movies")
	}

	_, out = libGet(t, HandleLibraryGrid, "group=movie_EN&q="+url.QueryEscape(`"Movie 1"`)+"&filter=missing&view=list")
	// Movie 11, 13, 15, 17, 19 match the phrase and are missing (odd ids); titles are zero-padded.
	if !strings.Contains(out, "5 entries") || !strings.Contains(out, "Plot 11") || strings.Contains(out, "Movie 12") {
		t.Errorf("search + filter + list:\n%.600s", out)
	}

	// Search syntax: words all match, | and OR are alternatives, -excludes.
	for search, want := range map[string]string{
		`Movie 1`:                       "14 entries", // "Movie" and "1": 01, 10-19, 21, 31, 41
		`"Movie 07" | "Movie 09"`:       "2 entries",
		`"Movie 07" OR "Movie 09" OR x`: "2 entries",
		`Movie & 07 | Movie AND 09`:     "2 entries",
		`"Movie 1" -"Movie 11"`:         "9 entries",
		`-Movie`:                        "Nothing found",
	} {
		if _, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&q="+url.QueryEscape(search)); !strings.Contains(out, want) {
			t.Errorf("search %s: want %q in\n%.300s", search, want, out)
		}
	}

	// Covers with descriptions: the overview under each cover.
	_, out = libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&view=cards&q=Movie+07")
	if !strings.Contains(out, `class="lib-grid lib-desc"`) || !strings.Contains(out, `<p class="lib-overview">Plot 7</p>`) ||
		!strings.Contains(out, "2007") {
		t.Errorf("cards view:\n%.600s", out)
	}

	// The plot is searched too ("Plot 3" is movie 03's overview).
	if _, out = libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&q=Plot+3"); !strings.Contains(out, "Movie 03") || strings.Contains(out, "Movie 04") {
		t.Errorf("plot search:\n%.400s", out)
	}

	_, out = libGet(t, HandleLibraryGrid, "group=serie_X&filter=missing")
	if !strings.Contains(out, "1 entries") || !strings.Contains(out, "Site Show") {
		t.Errorf("series missing filter:\n%.400s", out)
	}

	if _, out = libGet(t, HandleLibraryGrid, "group=nope"); !strings.Contains(out, "Unknown media group") {
		t.Errorf("unknown group: %s", out)
	}
}

func TestLibraryItemAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	dir := t.TempDir()
	keep, gone := filepath.Join(dir, "keep.mkv"), filepath.Join(dir, "gone.mkv")

	for _, f := range []string{keep, gone} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range []string{
		"insert into movie_files (movie_id, dbmovie_id, location, quality_profile) values (2, 2, '" + gone + "', 'HD')",
		"insert into movie_files (movie_id, dbmovie_id, location) values (4, 4, '" + keep + "')",
		"insert into movie_histories (movie_id, dbmovie_id, title, indexer) values (2, 2, 'Movie.02.1080p', 'nzbgeek')",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryItem, "type=movie&id=2")
	for _, want := range []string{`data-title="Movie 02"`, gone, "Movie.02.1080p", "Delete entry and file",
		`libAction(&#39;refresh&#39;, &#34;/api/movies/refresh/2&#34;)`,
		`data-entry-table="movies" data-entry-id="2" data-db-table="dbmovies" data-db-id="2"`} {
		if !strings.Contains(out, want) {
			t.Errorf("item lacks %s", want)
		}
	}

	_, out = libGet(t, HandleLibraryItem, "type=series&id=1")
	if !strings.Contains(out, "S01E01") || !strings.Contains(out, "/api/series/search/id/1") ||
		!strings.Contains(out, "/api/series/episodes/search/list/1") ||
		!strings.Contains(out, `libEdit(&#34;serie_episodes&#34;, 1, &#34;dbserie_episodes&#34;, 1, &#34;S01E01&#34;)`) {
		t.Errorf("series item:\n%.800s", out)
	}

	// The grid filter applies to the episodes; S01E01 is missing, not downloaded.
	if _, out = libGet(t, HandleLibraryItem, "type=series&id=1&filter=have"); strings.Contains(out, "Pilot") ||
		!strings.Contains(out, "Showing the 0 downloaded of 1 episodes") {
		t.Errorf("series item, downloaded episodes:\n%.800s", out)
	}

	if _, out = libGet(t, HandleLibraryItem, "type=series&id=1&filter=missing"); !strings.Contains(out, "Pilot") {
		t.Error("series item, missing episodes: no Pilot")
	}

	del := func(form url.Values) (int, string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		HandleLibraryDelete(c)

		return w.Code, w.Body.String()
	}

	// With files: the file and the entry go, other entries' files stay.
	if code, body := del(url.Values{"type": {"movie"}, "id": {"2"}, "files": {"1"}}); code != http.StatusOK || !strings.Contains(body, `"files":1`) {
		t.Fatalf("delete with files: %d %s", code, body)
	}

	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Error("file not deleted")
	}

	if _, err := os.Stat(keep); err != nil {
		t.Error("other file touched")
	}

	if n := database.Getdatarow[int](false, "select count() from movie_histories where movie_id = 2"); n != 0 ||
		database.Getdatarow[int](false, "select count() from movies where id = 2") != 0 {
		t.Error("entry or history kept")
	}

	// Without files: the file on disk stays.
	if code, _ := del(url.Values{"type": {"movie"}, "id": {"4"}}); code != http.StatusOK {
		t.Fatal("delete entry failed")
	}

	if _, err := os.Stat(keep); err != nil {
		t.Error("file deleted without files=1")
	}

	if code, _ := del(url.Values{"type": {"movie"}, "id": {"999"}}); code != http.StatusNotFound {
		t.Error("unknown entry accepted")
	}
}

func TestLibraryDeletePart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	dir := t.TempDir()
	ep, track := filepath.Join(dir, "ep.mkv"), filepath.Join(dir, "track.mp3")

	for _, f := range []string{ep, track} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	for _, s := range []string{
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (2, 1, 'S01E02', 'Second', '1', '2')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id) values (2, 1, 1, 2)",
		"insert into serie_episode_files (serie_id, serie_episode_id, dbserie_id, dbserie_episode_id, location) values (1, 2, 1, 2, '" + ep + "')",
		"insert into serie_episode_histories (serie_id, serie_episode_id, dbserie_id, dbserie_episode_id, title) values (1, 2, 1, 2, 'S01E02.rel')",
		"insert into album_files (id, location) values (7, '" + track + "')",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryItem, "type=series&id=1")
	if !strings.Contains(out, `libDeletePart(&#34;episode&#34;, &#34;series&#34;, 2, true, this)`) ||
		!strings.Contains(out, `libDeletePart(&#34;episode&#34;, &#34;series&#34;, 1, false, this)`) ||
		strings.Contains(out, `libDeletePart(&#34;episode&#34;, &#34;series&#34;, 1, true, this)`) {
		t.Errorf("episode delete buttons:\n%.1500s", out)
	}

	del := func(form url.Values) (int, string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		HandleLibraryDeletePart(c)

		return w.Code, w.Body.String()
	}

	// An episode with its file: rows and file gone, the other episode stays.
	if code, body := del(url.Values{"part": {"episode"}, "type": {"series"}, "id": {"2"}, "files": {"1"}}); code != http.StatusOK ||
		!strings.Contains(body, `"files":1`) {
		t.Fatalf("delete episode: %d %s", code, body)
	}

	if _, err := os.Stat(ep); !os.IsNotExist(err) {
		t.Error("episode file not deleted")
	}

	if database.Getdatarow[int](false, "select count() from serie_episodes where serie_id = 1") != 1 ||
		database.Getdatarow[int](false, "select count() from serie_episode_histories") != 0 ||
		database.Getdatarow[int](false, "select count() from serie_episode_files") != 0 {
		t.Error("episode rows kept or wrong episode deleted")
	}

	// A track row without its file: the file on disk stays.
	if code, _ := del(url.Values{"part": {"file"}, "type": {"album"}, "id": {"7"}}); code != http.StatusOK {
		t.Fatal("delete track row failed")
	}

	if _, err := os.Stat(track); err != nil {
		t.Error("track deleted without files=1")
	}

	if database.Getdatarow[int](false, "select count() from album_files where id = 7") != 0 {
		t.Error("track row kept")
	}

	for _, form := range []url.Values{
		{"part": {"file"}, "type": {"album"}, "id": {"7"}},
		{"part": {"episode"}, "type": {"movie"}, "id": {"1"}},
		{"part": {"entry"}, "type": {"series"}, "id": {"1"}},
	} {
		if code, _ := del(form); code != http.StatusNotFound {
			t.Errorf("%v accepted", form)
		}
	}
}

func TestLibraryRandom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	// Only odd movies are missing; every pick must match the filter.
	for range 20 {
		code, body := libGet(t, HandleLibraryRandom, "group=movie_EN&filter=missing")

		var got struct {
			Type string `json:"type"`
			ID   int    `json:"id"`
		}

		if err := json.Unmarshal([]byte(body), &got); err != nil || code != http.StatusOK || got.Type != "movie" || got.ID%2 != 1 {
			t.Fatalf("random missing movie: %d %s", code, body)
		}
	}

	if code, _ := libGet(t, HandleLibraryRandom, "group=movie_EN&filter=have"); code != http.StatusNotFound {
		t.Errorf("nothing downloaded, got %d", code)
	}

	if code, _ := libGet(t, HandleLibraryRandom, "group=nope"); code != http.StatusNotFound {
		t.Errorf("unknown group, got %d", code)
	}
}

func TestLibrarySorts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, q := range []string{
		"insert into movie_histories (movie_id, dbmovie_id, title, downloaded_at) values (7, 7, 'a', '2026-01-01'), (9, 9, 'b', '2026-03-01'), (7, 7, 'c', '2026-02-01')",
		"insert into movie_files (movie_id, dbmovie_id, location, created_at) values (12, 12, '/x/12.mkv', '2026-05-01'), (3, 3, '/x/3.mkv', '2026-04-01')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	titles := func(query string) []string {
		_, out := libGet(t, HandleLibraryGrid, query)

		var got []string
		for _, part := range strings.Split(out, `class="lib-title text-truncate" title="`)[1:] {
			got = append(got, part[:strings.Index(part, `"`)])
		}

		return got
	}

	// Newest download first, entries without one at the end.
	if got := titles("group=movie_EN&filter=all&sort=downloaded"); len(got) < 3 || got[0] != "Movie 09" || got[1] != "Movie 07" {
		t.Errorf("by download: %v", got[:min(3, len(got))])
	}

	if got := titles("group=movie_EN&filter=all&sort=filed"); len(got) < 2 || got[0] != "Movie 12" || got[1] != "Movie 03" {
		t.Errorf("by file: %v", got[:min(3, len(got))])
	}

	// Random: a seed gives a stable order, and the pages hold every entry once.
	seen := map[string]bool{}
	for _, page := range []string{"1", "2"} {
		for _, title := range titles("group=movie_EN&filter=all&sort=random&seed=4242&page=" + page) {
			if seen[title] {
				t.Errorf("%s on two pages", title)
			}

			seen[title] = true
		}
	}

	if len(seen) != 50 {
		t.Errorf("random pages show %d of 50 entries", len(seen))
	}

	a, b := titles("group=movie_EN&filter=all&sort=random&seed=1"), titles("group=movie_EN&filter=all&sort=random&seed=2")
	if strings.Join(a, ",") == strings.Join(b, ",") || strings.Join(a, ",") != strings.Join(titles("group=movie_EN&filter=all&sort=random&seed=1"), ",") {
		t.Error("random order doesn't follow the seed")
	}

	// The series grid still knows missing and downloaded (grouped joins).
	if err := database.ExecNErr("insert into serie_episode_files (serie_id, serie_episode_id, location) values (1, 1, '/x/e.mkv')"); err != nil {
		t.Fatal(err)
	}

	if _, out := libGet(t, HandleLibraryGrid, "group=serie_X&filter=have"); !strings.Contains(out, "Site Show") || !strings.Contains(out, "Missing") {
		t.Errorf("series downloaded + missing:\n%.400s", out)
	}
}

func TestLibraryExtraFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, q := range []string{
		"update dbmovies set genres = 'Action, Horror' where id in (3, 5)",
		"update dbmovies set genres = 'Horror' where id = 8",
		"update dbmovies set genres = 'Comedy,Action' where id = 9",
		"update dbmovies set overview = '' where id = 10",
		"insert into movie_files (movie_id, dbmovie_id, location, resolution_id) values (5, 5, '/x/5.mkv', 6), (8, 8, '/x/8.mkv', 5)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	count := func(query string) string {
		_, out := libGet(t, HandleLibraryGrid, "group=movie_EN&filter=all&"+query)
		if strings.Contains(out, "Nothing found") {
			return "0"
		}

		return out[strings.Index(out, `mb-2">`)+6 : strings.Index(out, " entries")]
	}

	for query, want := range map[string]string{
		"genre=Horror":                     "3",
		"genre=Action":                     "3", // "Action, Horror" and "Comedy,Action"
		"genre=Act":                        "0", // whole genres only
		"genre=Horror&year_from=2006":      "1", // movie 8 (2008)
		"year_from=2010&year_to=2019":      "10",
		"res=6":                            "1",
		"list=EN+List":                     "50",
		"list=Other":                       "50", // not a list of the group: ignored
		"filter=incomplete&year_from=2010": "1",
	} {
		q := query
		if strings.HasPrefix(q, "filter=incomplete") {
			_, out := libGet(t, HandleLibraryGrid, "group=movie_EN&"+q)
			if !strings.Contains(out, "1 entries") || !strings.Contains(out, "Movie 10") {
				t.Errorf("%s:\n%.300s", q, out)
			}

			continue
		}

		if got := count(q); got != want {
			t.Errorf("%s: %s entries, want %s", q, got, want)
		}
	}

	// The second filter row offers the genres and the resolutions in use.
	g := libGroupByName("movie_EN")
	out := renderComponentToString(renderLibraryExtraFilters(g, libQuery{Genre: "Horror"}))

	for _, want := range []string{`name="genre"`, `<option value="Horror" selected>`, `<option value="Comedy">`, `name="res"`, `name="year_from"`} {
		if !strings.Contains(out, want) {
			t.Errorf("filter row lacks %s:\n%s", want, out)
		}
	}
}

func TestLibraryFolderTitle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Music: mediaGroup("Music")})

	for _, q := range []string{
		"insert into dbalbums (id, title) values (1, ''), (2, 'Named')",
		"insert into albums (id, dbalbum_id, listname) values (1, 1, 'Music List'), (2, 2, 'Music List')",
		"insert into album_files (album_id, dbalbum_id, location) values (1, 1, '/music/Artist - Lost Album (1999)/01.flac')",
		"insert into dbartists (id, name) values (1, 'Someone')",
		"insert into dbalbum_artists (dbalbum_id, dbartist_id, position) values (2, 1, 0)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryGrid, "group=music_Music&filter=all")

	// The folder stands in for the missing title, in italics, and sorts last.
	named, folder := strings.Index(out, ">Named<"), strings.Index(out, "Artist - Lost Album (1999)")
	if folder < 0 || named < 0 || folder < named || !strings.Contains(out, "fst-italic") {
		t.Errorf("folder title:\n%.1500s", out)
	}

	if _, out = libGet(t, HandleLibraryGrid, "group=music_Music&filter=incomplete"); !strings.Contains(out, "Lost Album") || strings.Contains(out, ">Named<") {
		t.Errorf("incomplete albums:\n%.800s", out)
	}
}

func TestLibraryExtLinks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, q := range []string{
		"update dbmovies set imdb_id = 'tt1375666', moviedb_id = 27205, trakt_id = 16662, url = 'https://inception.example/' where id = 1",
		"update dbseries set thetvdb_id = 81189, trakt_id = 1388 where id = 1",
		"update dbserie_episodes set scraper_url = 'https://site.example/video/1/Pilot' where id = 1",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryItem, "type=movie&id=1")
	for _, want := range []string{"https://www.imdb.com/title/tt1375666", "https://www.themoviedb.org/movie/27205",
		"https://trakt.tv/movies/16662", `href="https://inception.example/"`, "fa-brands fa-imdb"} {
		if !strings.Contains(out, want) {
			t.Errorf("movie lacks %s", want)
		}
	}

	_, out = libGet(t, HandleLibraryItem, "type=series&id=1&filter=all")
	for _, want := range []string{"https://thetvdb.com/dereferrer/series/81189", "https://trakt.tv/shows/1388",
		`href="https://site.example/"`, `href="https://site.example/video/1/Pilot"`} {
		if !strings.Contains(out, want) {
			t.Errorf("series lacks %s", want)
		}
	}

	// Movie 2 has no IDs: no links, no empty row.
	if _, out = libGet(t, HandleLibraryItem, "type=movie&id=2"); strings.Contains(out, "imdb.com") || strings.Contains(out, "themoviedb.org") {
		t.Error("links without IDs")
	}
}

// TestCommonGenres: spellings that differ only in case are one genre, shown
// capitalised when any spelling is, the most common first within the limit,
// sorted by name.
func TestCommonGenres(t *testing.T) {
	got := commonGenres([]string{"Romance, Drama", "romance", "Comedy, drama", "drama,Romance", "Horror", "", " , "}, 3)

	if want := []string{"Comedy", "Drama", "Romance"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Tie: the capitalised spelling.
	if got := commonGenres([]string{"sci-fi", "Sci-Fi"}, 5); !slices.Equal(got, []string{"Sci-Fi"}) {
		t.Fatalf("tie: %v", got)
	}

	// Capitalised wins even when used less; among lower case the more used.
	if got := commonGenres([]string{"thriller", "thriller", "Thriller", "dokU", "doku", "doku"}, 5); !slices.Equal(got, []string{"doku", "Thriller"}) {
		t.Fatalf("spelling: %v", got)
	}
}

func TestLibSearchGroups(t *testing.T) {
	got := libSearchGroups(`bravo hits | "top 100" OR chart -xmas | -"x y" | a & b&c AND d | "Simon & Garfunkel" |`)
	want := [][]libTerm{
		{{text: "bravo"}, {text: "hits"}},
		{{text: "top 100"}},
		{{text: "chart"}, {text: "xmas", not: true}},
		{{text: "x y", not: true}},
		{{text: "a"}, {text: "b"}, {text: "c"}, {text: "d"}},
		{{text: "Simon & Garfunkel"}},
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	if w, _ := libSearchWhere("  |  "); w != "" {
		t.Fatalf("empty search: %q", w)
	}
}

// TestLibraryPlayAll: everything found, all pages, as one playlist.
func TestLibraryPlayAll(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, q := range []string{
		"insert into movie_files (movie_id, dbmovie_id, location) values (7, 7, '/x/7.mkv'), (9, 9, '/x/9.mkv'), (8, 8, '')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	var res struct {
		List   []libPlaylistItem
		Entry  string
		Capped bool
	}

	code, body := libGet(t, HandleLibraryPlayAll, "group=movie_EN&filter=all&size=1&q="+url.QueryEscape(`"Movie 07" | "Movie 08" | "Movie 09"`))
	if err := json.Unmarshal([]byte(body), &res); err != nil || code != http.StatusOK {
		t.Fatalf("%d %s", code, body)
	}

	// Both pages (size=1), in the grid's order; Movie 08's empty location is skipped.
	if len(res.List) != 2 || res.List[0].Label != "Movie 07" || res.List[1].Label != "Movie 09" || res.List[0].Kind != "movie" ||
		!strings.Contains(res.Entry, "movie_EN") || res.Capped {
		t.Fatalf("play all: %+v", res)
	}

	if code, _ := libGet(t, HandleLibraryPlayAll, "group=nope"); code != http.StatusNotFound {
		t.Fatalf("unknown group: %d", code)
	}
}

// TestLibraryWatched: the grid marks what the account watched (from 95%),
// audiobooks as heard, and the watched filters follow the account.
func TestLibraryWatched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN"), AudioBooks: mediaGroup("AB")})

	for _, q := range []string{
		"insert into dbmovies (id, title) values (1, 'Seen'), (2, 'Almost'), (3, 'Not yet')",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List'), (2, 2, 'EN List'), (3, 3, 'EN List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location) values (11, 1, 1, '/a'), (12, 2, 2, '/b'), (13, 3, 3, '/c')",
		"insert into dbaudiobooks (id, title) values (1, 'Book')",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'AB List')",
		"insert into audiobook_files (id, audiobook_id, dbaudiobook_id, location) values (21, 1, 1, '/p')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	for _, h := range []database.PlayHistory{
		{Username: "anna", Kind: "movie", FileID: 11, Position: 5720, Duration: 6000}, // 95.3%: the credits skipped
		{Username: "anna", Kind: "movie", FileID: 12, Position: 5640, Duration: 6000}, // 94%
		{Username: "anna", Kind: "audiobook", FileID: 21, Position: 3600, Duration: 3600},
		{Username: "bob", Kind: "movie", FileID: 13, Position: 6000, Duration: 6000},
	} {
		if err := database.PlayProgress(&h, true); err != nil {
			t.Fatal(err)
		}
	}

	groups := libraryGroups()
	group := func(name string) *libGroup {
		for i := range groups {
			if groups[i].Name == name {
				return &groups[i]
			}
		}

		t.Fatalf("no group %s", name)

		return nil
	}

	titles := func(filter string) string {
		rows, _ := libraryRows(group("movie_EN"), libQuery{User: "anna", Filter: filter, Sort: "title"})

		var out []string
		for _, r := range rows {
			out = append(out, r.Title+map[bool]string{true: "+"}[r.Watched])
		}

		return strings.Join(out, ",")
	}

	if got := titles("all"); got != "Almost,Not yet,Seen+" {
		t.Errorf("all: %s", got)
	}

	if got := titles("watched"); got != "Seen+" {
		t.Errorf("watched: %s", got)
	}

	if got := titles("unwatched"); got != "Almost,Not yet" {
		t.Errorf("unwatched: %s", got)
	}

	rows, _ := libraryRows(group("audiobook_AB"), libQuery{User: "anna"})
	if len(rows) != 1 || !rows[0].Watched || !strings.Contains(renderComponentToString(libStatusBadge(&rows[0])), "Heard") {
		t.Errorf("audiobook: %+v", rows)
	}
}
