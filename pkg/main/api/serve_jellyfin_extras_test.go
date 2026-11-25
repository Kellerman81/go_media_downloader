package api

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestJellyfinGenres: the genres of a library, and items by genre ID.
func TestJellyfinGenres(t *testing.T) {
	f := clientTestData(t)
	h := jfLogin(t, f, DefaultUsername, "adminpass")

	var genres jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Genres?ParentId="+jfGroupID("movie_EN"), &genres)

	if len(genres.Items) != 2 || genres.Items[0].Name != "Horror" || genres.Items[1].Name != "Sci-Fi" {
		t.Fatalf("genres: %+v", genres)
	}

	jfGenreNames.Clear() // as after a restart: found again by name

	var movies jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?Recursive=true&IncludeItemTypes=Movie&GenreIds="+genres.Items[1].ID, &movies)

	if movies.TotalRecordCount != 1 || movies.Items[0].Name != "Alien" || len(movies.Items[0].GenreItems) != 2 {
		t.Fatalf("by genre: %+v", movies)
	}

	var music jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/MusicGenres", &music)

	if len(music.Items) != 2 || music.Items[0].Name != "pop" || music.Items[0].Type != "MusicGenre" {
		t.Fatalf("music genres: %+v", music)
	}
}

// TestJellyfinBoxSets: movies of one TMDB collection (dbmovies.collection_id)
// are a box set.
func TestJellyfinBoxSets(t *testing.T) {
	f := clientTestData(t)

	for _, s := range []string{
		"insert into dbmovies (id, title, year, moviedb_id) values (3, 'Aliens', 1986, 679)",
		"insert into movies (id, dbmovie_id, listname) values (3, 3, 'EN List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location) values (13, 3, 3, '" + filepath.Join(f.dir, "movie.mkv") + "')",
		"update dbmovies set collection_id = 8091, collection_name = 'Alien Collection' where moviedb_id in (348, 679)",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	h := jfLogin(t, f, DefaultUsername, "adminpass")

	var views jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/UserViews", &views)

	if !strings.Contains(viewTypes(views), "boxsets") {
		t.Fatalf("views: %s", viewTypes(views))
	}

	var sets jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+jfGroupID("movie_EN")+"&IncludeItemTypes=BoxSet&Recursive=true", &sets)

	if len(sets.Items) != 1 || sets.Items[0].Name != "Alien Collection" || sets.Items[0].ChildCount != 2 {
		t.Fatalf("box sets: %+v", sets)
	}

	var movies jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+sets.Items[0].ID, &movies)

	if len(movies.Items) != 2 || movies.Items[0].Name != "Alien" || movies.Items[1].Name != "Aliens" {
		t.Fatalf("box set movies: %+v", movies)
	}

	// With one movie left in the library it is no set.
	if err := database.ExecNErr("update movies set listname = 'other' where id = 3"); err != nil {
		t.Fatal(err)
	}

	jfGetJSON(t, f, h, "/jellyfin/Items?IncludeItemTypes=BoxSet", &sets)

	if len(sets.Items) != 0 {
		t.Fatalf("set of one movie: %+v", sets)
	}

}

func viewTypes(r jfTestResult) string {
	var out []string
	for _, v := range r.Items {
		out = append(out, v.CollectionType)
	}

	return strings.Join(out, ",")
}

// TestJellyfinNoWebClient: the web client's address explains instead of
// failing.
func TestJellyfinNoWebClient(t *testing.T) {
	f := clientTestData(t)

	for _, p := range []string{"/jellyfin/", "/jellyfin/web/index.html"} {
		w := f.do(t, "GET", p, nil, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "Swiftfin") || strings.ContainsRune(w.Body.String(), trOpen) {
			t.Errorf("%s: %d %s", p, w.Code, w.Body)
		}
	}
}

// TestJellyfinPerson: a cast member opens as a person, lists the account's
// titles with them (personIds) and is found by the people search.
func TestJellyfinPerson(t *testing.T) {
	f := clientTestData(t)

	if err := database.SaveMediaExtras("movie", 1, []database.MediaCast{{PersonID: 10205, Name: "Sigourney Weaver", Character: "Ripley", Profile: "/w.jpg"}}, nil, nil); err != nil {
		t.Fatal(err)
	}

	h := jfLogin(t, f, DefaultUsername, "adminpass")
	person := jfID(jfPerson, 10205)

	var it jfItem
	jfGetJSON(t, f, h, "/jellyfin/Items/"+person, &it)

	if it.Type != "Person" || it.Name != "Sigourney Weaver" || it.ImageTags["Primary"] == "" {
		t.Fatalf("person: %+v", it)
	}

	var titles jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?PersonIds="+person+"&IncludeItemTypes=Movie,Series&Recursive=true", &titles)

	if len(titles.Items) != 1 || titles.Items[0].Name != "Alien" {
		t.Fatalf("filmography: %+v", titles)
	}

	var found jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Persons?searchTerm=weav", &found)

	if len(found.Items) != 1 || found.Items[0].ID != person {
		t.Fatalf("people search: %+v", found)
	}

	if w := f.do(t, "GET", "/jellyfin/Items/"+jfID(jfPerson, 1), nil, h); w.Code != 404 {
		t.Fatalf("unknown person: %d", w.Code)
	}
}
