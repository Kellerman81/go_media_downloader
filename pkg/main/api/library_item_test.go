package api

import (
	"maps"
	"maragu.dev/gomponents/html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

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
		"update dbmovies set genres = 'Action, Science Fiction' where id = 2",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	_, out := libGet(t, HandleLibraryItem, "type=movie&id=2")
	for _, want := range []string{`data-title="Movie 02"`, gone, "Movie.02.1080p", "Delete entry and its file", "don&#39;t add again",
		`libAction(&#39;refresh&#39;, &#34;/api/movies/refresh/2&#34;)`,
		`data-entry-table="movies" data-entry-id="2" data-db-table="dbmovies" data-db-id="2"`,
		`libShowGenre(&#34;movie&#34;, &#34;Science Fiction&#34;)`, ">Action</button>"} {
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

// TestLibraryAlbumTracks: the Tracks tab matches files by dbtrack_id, by
// disc and number, then by title in the file name, and the status badge
// says what the album lacks.
func TestLibraryAlbumTracks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Music: mediaGroup("Music")})

	for _, q := range []string{
		"insert into dbalbums (id, title) values (1, 'Album')",
		"insert into albums (id, dbalbum_id, listname, missing) values (1, 1, 'Music List', 1)",
		`insert into dbtracks (id, dbalbum_id, disc_number, track_number, title) values
			(11, 1, 1, 1, 'Intro'), (12, 1, 1, 2, 'Song'), (13, 1, 1, 3, 'Gone'), (14, 1, 2, 1, 'Other Side')`,
		`insert into album_files (id, album_id, dbalbum_id, dbtrack_id, disc_number, track_number, filename, location) values
			(1, 1, 1, 11, 1, 1, '01 - Intro.flac', '/m/01 - Intro.flac'),
			(2, 1, 1, 0, 0, 2, '02 - Song.flac', '/m/02 - Song.flac'),
			(3, 1, 1, 0, 1, 9, '09 - Other Side.flac', '/m/09 - Other Side.flac')`,
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	files := map[string]uint{}
	for _, tr := range libAlbumTracks(1, 1) {
		files[tr.Title] = tr.FileID
	}

	if want := map[string]uint{"Intro": 1, "Song": 2, "Gone": 0, "Other Side": 3}; !maps.Equal(files, want) {
		t.Errorf("tracks %v, want %v", files, want)
	}

	if _, out := libGet(t, HandleLibraryItem, "type=album&id=1"); !strings.Contains(out, "1 of 4 tracks missing") || !strings.Contains(out, `id="libTabTracks"`) {
		t.Errorf("missing track:\n%.3000s", out)
	}

	// The grid filter applies to the tracks; the search looks in the file names.
	_, out := libGet(t, HandleLibraryItem, "type=album&id=1&filter=missing")
	if !strings.Contains(out, `data-title="Gone"`) || strings.Contains(out, `data-title="Intro"`) || !strings.Contains(out, "Show all tracks") {
		t.Errorf("missing filter:\n%.3000s", out)
	}

	if _, out = libGet(t, HandleLibraryItem, "type=album&id=1"); !strings.Contains(out, `data-q="1-01 intro 01 - intro.flac"`) ||
		!strings.Contains(out, "libFilterList(this)") || !strings.Contains(out, "libSortList(this)") || strings.Contains(out, "Show all tracks") ||
		!strings.Contains(out, "lib-list-ws") || !strings.Contains(out, `data-ws="unwatched"`) {
		t.Errorf("track search:\n%.3000s", out)
	}

	// The note only shows when the filter hides something.
	if libFilterNote("tracks", "have", 13, 13) != nil || libFilterNote("episodes", "have", 12, 13) == nil {
		t.Error("filter note: shown for a filter that hides nothing, or missing")
	}

	// A file no track matches: the track without one is unclear.
	if err := database.ExecNErr("insert into album_files (id, album_id, dbalbum_id, track_number, filename, location) values (4, 1, 1, 30, 'x.flac', '/m/x.flac')"); err != nil {
		t.Fatal(err)
	}

	if _, out := libGet(t, HandleLibraryItem, "type=album&id=1"); !strings.Contains(out, "1 tracks without a matching file, 1 files without a track") ||
		!strings.Contains(out, "Not matched") {
		t.Errorf("unmatched file:\n%.3000s", out)
	}

	// Every track there, the flag still set.
	if err := database.ExecNErr("update album_files set filename = '03 - Gone.flac', track_number = 3, disc_number = 1 where id = 4"); err != nil {
		t.Fatal(err)
	}

	if _, out := libGet(t, HandleLibraryItem, "type=album&id=1"); !strings.Contains(out, "All tracks are there - the missing flag is out of date") {
		t.Errorf("outdated flag:\n%.3000s", out)
	}
}

// TestLibWatchState: rows carry their file's watched state for the list's
// watched filter; rows without a file count as not watched.
func TestLibWatchState(t *testing.T) {
	watched := map[uint]float64{1: 1, 2: 0.4}

	for id, want := range map[uint]string{1: "watched", 2: "progress", 3: "unwatched", 0: "unwatched"} {
		var b strings.Builder
		_ = html.Tr(libWatchState(watched, id)).Render(&b)

		if !strings.Contains(b.String(), `data-ws="`+want+`"`) {
			t.Errorf("file %d: %s, want %s", id, b.String(), want)
		}
	}
}

// TestLibrarySeriesSearchButton: "Search missing episodes" shows only when
// the missing search would take an episode; a missing special counts only
// when the series searches specials.
func TestLibrarySeriesSearchButton(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	shown := func() bool {
		t.Helper()

		_, out := libGet(t, HandleLibraryItem, "type=series&id=1")
		if strings.Contains(out, "Search missing episodes") && !strings.Contains(out, "missing=true") {
			t.Errorf("search without missing=true:\n%.2000s", out)
		}

		return strings.Contains(out, "Search missing episodes")
	}

	if !shown() {
		t.Error("missing episode: no search button")
	}

	for _, s := range []string{
		"update serie_episodes set missing = 0 where id = 1",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode) values (3, 1, 'S00E01', 'Special', '0', '1')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id, missing) values (3, 1, 1, 3, 1)",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	if shown() {
		t.Error("only an unwanted special missing: search button shown")
	}

	if err := database.ExecNErr("update series set search_specials = 1 where id = 1"); err != nil {
		t.Fatal(err)
	}

	if !shown() {
		t.Error("wanted special missing: no search button")
	}
}
