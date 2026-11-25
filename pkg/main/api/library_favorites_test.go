package api

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestLibraryFavorite: the heart adds to the account's favorites
// playlist and takes out again; entries the account may not see can't be
// added.
func TestLibraryFavorite(t *testing.T) {
	clientTestData(t)

	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}
	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_EN"}}

	for _, tc := range []struct {
		s    *Session
		form url.Values
		code int
	}{
		{admin, url.Values{"kind": {"movie"}, "entry": {"1"}, "on": {"1"}}, http.StatusOK},
		{admin, url.Values{"kind": {"episode"}, "entry": {"1"}, "item": {"6"}, "on": {"1"}}, http.StatusOK},
		{admin, url.Values{"kind": {"nothing"}, "entry": {"1"}, "on": {"1"}}, http.StatusBadRequest},
		{anna, url.Values{"kind": {"series"}, "entry": {"1"}, "on": {"1"}}, http.StatusNotFound},
	} {
		if w := callAs(tc.s, HandleLibraryFavorite, tc.form); w.Code != tc.code {
			t.Errorf("%s %v: %d, want %d", tc.s.UserID, tc.form, w.Code, tc.code)
		}
	}

	favs := favoriteSet("admin")
	if !favs["movie"][1] || !favs["episode"][6] || len(favs["series"]) != 0 {
		t.Fatalf("favorites: %v", favs)
	}

	if lists := database.GetPlaylists("admin"); len(lists) != 1 || lists[0].Name != favoritesName || lists[0].Items != 2 {
		t.Fatalf("playlists: %+v", lists)
	}

	if len(database.GetPlaylists("anna")) != 0 {
		t.Fatal("anna got a favorites playlist for nothing")
	}

	callAs(admin, HandleLibraryFavorite, url.Values{"kind": {"movie"}, "entry": {"1"}, "on": {"0"}})

	if isFavorite("admin", "movie", 1, 0) || !isFavorite("admin", "episode", 1, 6) {
		t.Fatal("movie still a favorite, or the episode gone")
	}

	// The heart in the detail dialog shows the state.
	_, body := libGet(t, HandleLibraryItem, "type=series&id=1")
	if !strings.Contains(body, "lib-fav") {
		t.Fatalf("no favorite buttons: %.500s", body)
	}
}
