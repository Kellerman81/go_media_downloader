package api

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents/html"
)

func calDate(s string) time.Time {
	d, _ := time.Parse("2006-01-02", s)
	return d
}

func TestCalendarStateRange(t *testing.T) {
	for _, tc := range []struct {
		view, date, first, last, prev, next string
	}{
		{"month", "2026-12-17", "2026-12-01", "2026-12-31", "2026-11-01", "2027-01-01"},
		{"week", "2026-12-17", "2026-12-13", "2026-12-19", "2026-12-10", "2026-12-24"}, // Sunday to Saturday
		{"agenda", "2026-12-17", "2026-12-10", "2027-02-07", "2026-10-18", "2027-02-15"},
	} {
		st := calendarState{View: tc.view, Filter: "all", Date: calDate(tc.date)}

		first, last := st.Range()
		if first.Format("2006-01-02") != tc.first || last.Format("2006-01-02") != tc.last {
			t.Errorf("%s: range %s..%s, want %s..%s", tc.view, first.Format("2006-01-02"), last.Format("2006-01-02"), tc.first, tc.last)
		}

		if p, n := st.Step(-1).Format("2006-01-02"), st.Step(1).Format("2006-01-02"); p != tc.prev || n != tc.next {
			t.Errorf("%s: steps %s / %s, want %s / %s", tc.view, p, n, tc.prev, tc.next)
		}
	}
}

func TestMergeCalendarEvents(t *testing.T) {
	d := calDate("2025-12-01")
	merged := mergeCalendarEvents([]calendarEvent{
		{ID: 2, Type: "movie", Title: "B", Date: d, Listname: "EN Watchlist", dbKey: "7", Monitored: true},
		{ID: 1, Type: "movie", Title: "A", Date: d, Listname: "EN Hallmark", dbKey: "5"},
		{ID: 3, Type: "movie", Title: "B", Date: d, Listname: "EN Hallmark", dbKey: "7", Downloaded: true},
		{ID: 4, Type: "series", Title: "B", Date: d, Listname: "EN", dbKey: "7"}, // same id, other type
	})

	if len(merged) != 3 || merged[0].Title != "A" {
		t.Fatalf("merged = %+v", merged)
	}

	// Watchlist still wants the file Hallmark has: partly downloaded, and
	// Play and the ID go to the entry with the file.
	b := merged[1]
	if b.Type != "movie" || b.ID != 3 || b.Listname != "EN Watchlist, EN Hallmark" || b.Downloaded || !b.Partial ||
		b.Status != "Partly downloaded" || !b.Overdue || len(b.Entries) != 2 || b.Entries[0].ID != 2 {
		t.Errorf("merged movie = %+v", b)
	}

	if items, n := calendarSearchItems(merged); n != 1 || items != `[{"type":"movie","id":2}]` {
		t.Errorf("search items = %s", items)
	}

	// An unmonitored entry without the file doesn't make it partial.
	merged = mergeCalendarEvents([]calendarEvent{
		{ID: 1, Type: "movie", Date: d, Listname: "EN", dbKey: "7", Downloaded: true},
		{ID: 2, Type: "movie", Date: d, Listname: "DE", dbKey: "7"},
	})
	if m := merged[0]; !m.Downloaded || m.Partial || m.Overdue || m.Status != "Downloaded" {
		t.Errorf("unmonitored entry = %+v", m)
	}
}

func TestICalTextAndFolding(t *testing.T) {
	if got := escapeICalText("a,b;c\\d\r\ne\nf"); got != `a\,b\;c\\d\ne\nf` {
		t.Errorf("escape = %q", got)
	}

	long := "DESCRIPTION:" + strings.Repeat("Größe ", 40)

	folded := foldICalLine(long)
	for _, l := range strings.Split(folded, "\r\n") {
		if len(l) > 75 || !utf8.ValidString(l) {
			t.Fatalf("folded line %q: %d octets, valid UTF-8 %v", l, len(l), utf8.ValidString(l))
		}
	}

	if strings.ReplaceAll(folded, "\r\n ", "") != long {
		t.Error("unfolding doesn't restore the line")
	}
}

func TestCalendarICalHandler(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN"), Series: mediaGroup("TV")})

	day := time.Now().UTC().Format("2006-01-02")
	for _, q := range []string{
		"insert into dbmovies (id, title, release_date, overview) values (1, 'Upcoming, Part 1', '" + day + " 00:00:00+00:00', 'Line one\nLine two')",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List'), (2, 1, 'Other List')",
		"insert into dbseries (id, seriename) values (1, 'Show')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'TV List')",
		"insert into dbserie_episodes (id, dbserie_id, season, episode, identifier, first_aired) values (1, 1, '1', '2', 'S01E02', '" + day + " 00:00:00+00:00')",
		"insert into serie_episodes (id, dbserie_id, serie_id, dbserie_episode_id) values (1, 1, 1, 1)",
		"insert into serie_episode_files (location, serie_episode_id, serie_id) values ('/x.mkv', 1, 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	get := func(query string) (string, string) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		asAdmin(ctx)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/calendar/ical?"+query, nil)

		CalendarICalHandler(ctx)

		return w.Body.String(), w.Header().Get("Content-Disposition")
	}

	body, disposition := get("view=week&filter=all&date=" + day)
	if strings.Count(body, "BEGIN:VEVENT") != 2 || !strings.Contains(body, `SUMMARY:Upcoming\, Part 1`) ||
		!strings.Contains(body, "SUMMARY:Show S01E02") || !strings.Contains(body, `Lists: EN List\, Other List`) ||
		!strings.Contains(body, `Line one\nLine two`) || strings.Contains(body, `\\n`) || !strings.Contains(body, "DTEND;VALUE=DATE:") {
		t.Errorf("all:\n%s", body)
	}

	if !strings.Contains(disposition, "media-calendar-all-") {
		t.Errorf("filename %q", disposition)
	}

	if body, _ := get("view=week&filter=movies&date=" + day); strings.Count(body, "BEGIN:VEVENT") != 1 || strings.Contains(body, "Show") {
		t.Errorf("movies only:\n%s", body)
	}

	// The episode is downloaded, so "only missing" leaves the movie.
	if body, _ := get("view=week&filter=all&missing=1&date=" + day); strings.Count(body, "BEGIN:VEVENT") != 1 ||
		!strings.Contains(body, "X-WR-CALNAME:Media Calendar (missing)") {
		t.Errorf("only missing:\n%s", body)
	}

	for _, l := range strings.Split(body, "\r\n") {
		if len(l) > 75 {
			t.Errorf("unfolded line %q", l)
		}
	}
}

func TestCalendarImageURL(t *testing.T) {
	str := func(s string) *string { return &s }

	for _, tc := range []struct {
		path *string
		base string
		want string
	}{
		{str("/abc.jpg"), "https://image.tmdb.org/t/p/w342", "https://image.tmdb.org/t/p/w342/abc.jpg"},
		{str("posters/310882-2.jpg"), "https://artworks.thetvdb.com/banners/", "https://artworks.thetvdb.com/banners/posters/310882-2.jpg"},
		{str("https://m.media-amazon.com/x.jpg"), "", "https://m.media-amazon.com/x.jpg"},
		{str("https://artworks.thetvdb.com/banners/v4/x.jpg"), "https://artworks.thetvdb.com/banners/", "https://artworks.thetvdb.com/banners/v4/x.jpg"},
		{str("relative.jpg"), "", ""},
		{str(""), "https://x", ""},
		{nil, "https://x", ""},
	} {
		if got := calendarImageURL(tc.path, tc.base); got != tc.want {
			t.Errorf("calendarImageURL(%v, %q) = %q, want %q", tc.path, tc.base, got, tc.want)
		}
	}
}

func TestCalendarEventAttrs(t *testing.T) {
	var b strings.Builder
	if err := html.Div(calendarEventAttrs(calendarEvent{ID: 3, Title: `Tom's "Show" <b>`, Type: "movie", PosterURL: "https://x/p.jpg"})).Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()
	// The JSON is HTML-escaped inside the attribute, so quotes and tags can't break out.
	if !strings.Contains(out, `data-event="{&#34;id&#34;:3,&#34;title&#34;:&#34;Tom&#39;s \&#34;Show\&#34; \u003cb\u003e&#34;`) ||
		!strings.Contains(out, `onclick="showEventDetails(JSON.parse(this.dataset.event))"`) ||
		!strings.Contains(out, `&#34;posterUrl&#34;:&#34;https://x/p.jpg&#34;`) {
		t.Errorf("attrs:\n%s", out)
	}
}

func TestCalendarBooks(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Books: mediaGroup("Books")})

	day := time.Now().UTC().Format("2006-01-02")
	for _, q := range []string{
		"insert into dbauthors (id, name) values (1, 'Sebastian Fitzek')",
		"insert into dbbooks (id, title, publish_date, dbauthor_id, cover_url) values (1, 'Der Nachtzug', '" + day + " 00:00:00+00:00', 1, 'https://covers.example/1.jpg')",
		"insert into books (id, dbbook_id, listname) values (1, 1, 'Books List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	events := calendarState{View: "week", Filter: "books", Date: calDate(day)}.Events()
	if len(events) != 1 || events[0].Type != "book" || events[0].Network != "Sebastian Fitzek" ||
		events[0].PosterURL != "/api/admin/library/cover/book/1" || events[0].Status != "Monitored" ||
		events[0].LibraryURL != "/api/admin/library?group=book_Books#book/1" {
		t.Fatalf("book events = %+v", events)
	}
}

func TestCalendarExternalLinks(t *testing.T) {
	audiobooks := mediaGroup("Audiobooks")
	audiobooks[0].AudibleRegion = "de"
	useMediaConfig(t, config.MediaConfig{AudioBooks: audiobooks, Music: mediaGroup("Music"), Books: mediaGroup("Books")})

	day := time.Now().UTC().Format("2006-01-02")
	for _, q := range []string{
		"insert into dbartists (id, name) values (1, 'Pink Floyd')",
		"insert into dbalbums (id, title, release_date, musicbrainz_release_group_id, musicbrainz_release_id, discogs_master_id, deezer_id, itunes_id) " +
			"values (1, 'The Wall', '" + day + "', 'mb-group', 'mb-rel', '0', '302127', '')",
		"insert into dbalbum_artists (dbalbum_id, dbartist_id, position) values (1, 1, 0)",
		"insert into albums (id, dbalbum_id, listname) values (1, 1, 'Music List')",
		"insert into dbauthors (id, name) values (2, 'Walter Moers')",
		"insert into dbaudiobooks (id, title, release_date, asin) values (1, 'Die Stadt der Träumenden Bücher', '" + day + "', 'B00ABC')",
		"insert into dbaudiobook_authors (dbaudiobook_id, dbauthor_id, position) values (1, 2, 0)",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'Audiobooks List')",
		"insert into dbbooks (id, title, publish_date, dbauthor_id, isbn_13) values (1, 'Rumo', '" + day + "', 2, '9783492045698')",
		"insert into books (id, dbbook_id, listname) values (1, 1, 'Books List')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	byType := map[string]calendarEvent{}
	for _, e := range (calendarState{View: "week", Filter: "all", Date: calDate(day)}).Events() {
		byType[e.Type] = e
	}

	urls := func(e calendarEvent) []string {
		out := make([]string, 0, len(e.Links))
		for _, l := range e.Links {
			out = append(out, l.Label+" "+l.URL)
		}

		return out
	}

	for typ, want := range map[string][]string{
		// Release group before release; "0" and empty IDs give no link.
		"album": {
			"MusicBrainz https://musicbrainz.org/release-group/mb-group",
			"Deezer https://www.deezer.com/album/302127",
			"Last.fm https://www.last.fm/music/Pink+Floyd/The+Wall",
		},
		// The audible_region of the list's media group picks the store.
		"audiobook": {
			"Audible https://www.audible.de/pd/B00ABC",
			"Goodreads https://www.goodreads.com/search?q=Die+Stadt+der+Tr%C3%A4umenden+B%C3%BCcher+Walter+Moers",
		},
		"book": {
			"Open Library https://openlibrary.org/isbn/9783492045698",
			"Goodreads https://www.goodreads.com/search?q=9783492045698",
		},
	} {
		e, ok := byType[typ]
		if !ok {
			t.Fatalf("no %s event in %+v", typ, byType)
		}

		if got := urls(e); !reflect.DeepEqual(got, want) {
			t.Errorf("%s links = %q, want %q", typ, got, want)
		}
	}

	if byType["album"].Network != "Pink Floyd" || byType["audiobook"].Network != "Walter Moers" {
		t.Errorf("artist/author: %q, %q", byType["album"].Network, byType["audiobook"].Network)
	}
}

func TestBookLinksWithoutIDs(t *testing.T) {
	links := bookLinks(&BookCalendarQuery{Title: "Rumo"}, "Walter Moers")
	if len(links) != 1 || links[0].URL != "https://www.goodreads.com/search?q=Rumo+Walter+Moers" {
		t.Errorf("links = %+v", links)
	}

	if l := audiobookLinks(&AudiobookCalendarQuery{Title: "X", ASIN: "B1"}, "nowhere"); l[0].URL != "https://www.audible.com/pd/B1" {
		t.Errorf("unknown region: %+v", l)
	}
}

func TestCalendarSeriesScraperLink(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Series: mediaGroup("X")})

	day := time.Now().UTC().Format("2006-01-02")
	for _, q := range []string{
		"insert into dbseries (id, seriename) values (1, 'Site Show')",
		"insert into dbserie_episodes (id, dbserie_id, title, identifier, first_aired, scraper_url) values " +
			"(1, 1, 'Scene', '" + day + "', '" + day + "', 'https://site.example/videos/1'), " +
			"(2, 1, 'Other', 'x', '" + day + "', 'javascript:alert(1)')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'X List')",
		"insert into serie_episodes (id, serie_id, dbserie_episode_id, dbserie_id) values (1, 1, 1, 1), (2, 1, 2, 1)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	links := map[string][]calendarExtLink{}
	for _, e := range (calendarState{View: "week", Filter: "series", Date: calDate(day)}).Events() {
		for i := range e.Links {
			e.Links[i].Label = plain(e.Links[i].Label)
		}

		links[e.EpTitle] = e.Links
	}

	if want := []calendarExtLink{{Label: "Source", URL: "https://site.example/videos/1", Icon: "fa-solid fa-globe"}}; !reflect.DeepEqual(links["Scene"], want) {
		t.Errorf("links = %+v", links)
	}

	if _, ok := links["Other"]; !ok || links["Other"] != nil {
		t.Errorf("non-http scraper URL linked: %+v", links)
	}
}

func TestCalendarLibraryFeatures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: append(mediaGroup("EN"), mediaGroup("DE")...), Series: mediaGroup("X")})

	today := time.Now() // local, like the calendar: UTC is a day behind after midnight
	past, todayS := today.AddDate(0, 0, -2).Format("2006-01-02"), today.Format("2006-01-02")

	for _, q := range []string{
		// Movie 1: released two days ago, monitored, missing -> overdue.
		"insert into dbmovies (id, title, release_date, genres, imdb_id, moviedb_id) values (1, 'Late One', '" + past + "', 'Action, Horror', 'tt1', 5)",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List')",
		// Movie 2: today, downloaded just now -> new.
		"insert into dbmovies (id, title, release_date, genres) values (2, 'Fresh One', '" + todayS + "', 'Comedy')",
		"insert into movies (id, dbmovie_id, listname) values (2, 2, 'DE List')",
		"insert into movie_files (movie_id, dbmovie_id, location, created_at) values (2, 2, '/x/2.mkv', datetime('now'))",
		// Movie 3 is in both groups: DE has the file, EN still wants it.
		"insert into dbmovies (id, title, release_date) values (3, 'Both', '" + past + "')",
		"insert into movies (id, dbmovie_id, listname) values (3, 3, 'EN List'), (4, 3, 'DE List')",
		"insert into movie_files (movie_id, dbmovie_id, location) values (4, 3, '/x/4.mkv')",
		// Series: three episodes of one site series on one day, one downloaded.
		"insert into dbseries (id, seriename) values (1, 'girlsway')",
		"insert into series (id, dbserie_id, listname) values (4, 1, 'X List')",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title, season, episode, first_aired) values " +
			"(1, 1, 'A', 'Ep A', '1', '1', '" + todayS + "'), (2, 1, 'B', 'Ep B', '1', '2', '" + todayS + "'), (3, 1, 'C', 'Ep C', '1', '3', '" + todayS + "')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id) values (11, 4, 1, 1), (12, 4, 1, 2), (13, 4, 1, 3)",
		"insert into serie_episode_files (id, serie_id, serie_episode_id, dbserie_id, dbserie_episode_id, location) values (7, 4, 12, 1, 2, '/x/b.mkv')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	st := calendarState{View: "month", Filter: "all", Date: calDate(todayS)}
	byTitle := map[string]calendarEvent{}

	for _, e := range st.Events() {
		byTitle[e.Title] = e
	}

	late, fresh, epB := byTitle["Late One"], byTitle["Fresh One"], byTitle["girlsway - Ep B"]
	if !late.Overdue || late.New || late.PosterURL != "/api/admin/library/cover/movie/1" ||
		late.LibraryURL != "/api/admin/library?group=movie_EN#movie/1" || len(late.Links) != 2 || late.Links[0].Label != "IMDb" {
		t.Errorf("late = %+v", late)
	}

	if fresh.Overdue || !fresh.New || fresh.LibraryURL != "/api/admin/library?group=movie_DE#movie/2" {
		t.Errorf("fresh = %+v", fresh)
	}

	// Episodes open their series in the Library, with the series cover and a frame.
	if epB.LibraryURL != "/api/admin/library?group=serie_X#series/4" || epB.PosterURL != "/api/admin/library/cover/series/1" ||
		epB.ThumbURL != "/api/admin/library/thumb/episode/7" {
		t.Errorf("episode = %+v", epB)
	}

	both := byTitle["Both"]
	if !both.Partial || both.Downloaded || !both.Overdue || both.ID != 4 || len(both.Entries) != 2 ||
		both.Entries[0].Group != "movie_EN" || both.Entries[1].LibraryURL != "/api/admin/library?group=movie_DE#movie/4" {
		t.Errorf("both = %+v", both)
	}

	// Filters: media group and genre. Within a group an event has only that
	// group's entry and state.
	ev := (calendarState{View: "month", Filter: "all", Date: st.Date, Group: "movie_DE"}).Events()
	if len(ev) != 2 || ev[0].Title != "Both" || !ev[0].Downloaded || ev[0].Partial || len(ev[0].Entries) != 1 || ev[1].Title != "Fresh One" {
		t.Errorf("group filter DE: %+v", ev)
	}

	ev = (calendarState{View: "month", Filter: "all", Date: st.Date, Group: "movie_EN", OnlyMissing: true}).Events()
	if len(ev) != 2 || ev[0].Title != "Both" || ev[0].Downloaded || !ev[0].Overdue || ev[0].ID != 3 {
		t.Errorf("group filter EN: %+v", ev)
	}

	if ev := (calendarState{View: "month", Filter: "all", Date: st.Date, Genre: "horror"}).Events(); len(ev) != 1 || ev[0].Title != "Late One" {
		t.Errorf("genre filter: %+v", ev)
	}

	// The three episodes collapse into one group; movies stay single.
	groups := groupCalendarEvents(slices.DeleteFunc(st.Events(), func(e calendarEvent) bool { return e.Date.Format("2006-01-02") != todayS }))
	if len(groups) != 2 || calendarEventCount(groups) != 4 {
		t.Errorf("groups = %d of %d events", len(groups), calendarEventCount(groups))
	}

	// The page: group/genre selects, overdue search, the collapsed series.
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/?view=month&date="+todayS, nil)
	CalendarContentHandler(c)

	for _, want := range []string{"All media groups", "Any genre", `<option value="Horror">`, "Search overdue (2)", "girlsway ×3", "2 overdue", "1 partly downloaded", `{&#34;type&#34;:&#34;movie&#34;,&#34;id&#34;:3}`, "2 new since yesterday"} {
		if !strings.Contains(plain(w.Body.String()), want) {
			t.Errorf("page lacks %q", want)
		}
	}

	// Playlist of a downloaded episode and of a movie without files.
	play := func(query string) (int, string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodGet, "/?"+query, nil)
		HandleLibraryPlaylist(c)

		return w.Code, w.Body.String()
	}

	if code, body := play("type=series&id=12"); code != http.StatusOK || !strings.Contains(body, `{"kind":"episode","id":7,"label":"B Ep B"}`) ||
		!strings.Contains(body, `"entry":"girlsway"`) {
		t.Errorf("episode playlist: %d %s", code, body)
	}

	if code, body := play("type=movie&id=1"); code != http.StatusOK || !strings.Contains(body, `"list":[]`) {
		t.Errorf("movie without files: %d %s", code, body)
	}

	if code, _ := play("type=book&id=1"); code != http.StatusNotFound {
		t.Errorf("books play: %d", code)
	}
}

// TestMovieLinksWebsite: the website link is left out when it is a link
// shown already (often the IMDb page).
func TestMovieLinksWebsite(t *testing.T) {
	for _, site := range []string{"https://www.imdb.com/title/tt0092494", "http://imdb.com/title/tt0092494/"} {
		links := movieLinks("tt0092494", 11548, 0, site)
		if len(links) != 2 { // IMDb and TMDB
			t.Errorf("%s shown again: %+v", site, links)
		}
	}

	links := movieLinks("tt0092494", 0, 0, "https://example.com/film")
	if !slices.ContainsFunc(links, func(l calendarExtLink) bool { return l.URL == "https://example.com/film" }) {
		t.Errorf("own website missing: %+v", links)
	}
}
