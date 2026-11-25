package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestParseJFID(t *testing.T) {
	for _, tc := range []struct {
		kind byte
		n    uint64
	}{{jfMovie, 1}, {jfSeason, 7<<16 | 2}, {jfUser, 0}, {jfTrack, 1 << 40}} {
		id := jfID(tc.kind, tc.n)
		if len(id) != 32 {
			t.Fatalf("%s: length %d", id, len(id))
		}

		dashed := id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
		for _, s := range []string{id, strings.ToUpper(dashed)} {
			if k, n, ok := parseJFID(s); !ok || k != tc.kind || n != tc.n {
				t.Errorf("%s: %d %d %v", s, k, n, ok)
			}
		}
	}

	if _, _, ok := parseJFID("xyz"); ok {
		t.Error("short ID accepted")
	}
}

// jfLogin logs in through the API and returns the Authorization header.
func jfLogin(t *testing.T, f *clientFixture, user, pass string) http.Header {
	t.Helper()

	h := http.Header{}
	h.Set("Authorization", `MediaBrowser Client="Test", Device="Box", DeviceId="dev1", Version="1"`)
	h.Set("Content-Type", "application/json")

	w := f.do(t, http.MethodPost, "/jellyfin/Users/AuthenticateByName", strings.NewReader(`{"Username":"`+user+`","Pw":"`+pass+`"}`), h)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: %d %s", user, w.Code, w.Body)
	}

	var res struct {
		AccessToken string
		User        struct{ ID string }
	}

	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res.AccessToken == "" || res.User.ID != jfUserID(user) {
		t.Fatalf("login answer: %s", w.Body)
	}

	h.Set("Authorization", `MediaBrowser Client="Test", Device="Box", DeviceId="dev1", Version="1", Token="`+res.AccessToken+`"`)

	return h
}

type jfTestResult struct {
	Items            []jfItem
	TotalRecordCount int
}

func jfGetJSON(t *testing.T, f *clientFixture, h http.Header, target string, out any) {
	t.Helper()

	w := f.do(t, http.MethodGet, target, nil, h)
	if w.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", target, w.Code, w.Body)
	}

	if err := json.Unmarshal(w.Body.Bytes(), out); err != nil {
		t.Fatalf("%s: %v %s", target, err, w.Body)
	}
}

func TestJellyfin(t *testing.T) {
	f := clientTestData(t)

	var info map[string]any
	jfGetJSON(t, f, nil, "/jellyfin/System/Info/Public", &info)

	if info["Version"] != jfVersion || info["Id"] != jfServerID() || info["ServerName"] != defaultServerName {
		t.Fatalf("public info: %v", info)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/UserViews", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("views without login: %d", w.Code)
	}

	h := jfLogin(t, f, DefaultUsername, "adminpass")

	var views jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Users/"+jfUserID("admin")+"/Views", &views)

	if len(views.Items) != 4 {
		t.Fatalf("views: %+v", views)
	}

	byType := map[string]jfItem{}
	for _, v := range views.Items {
		byType[v.CollectionType] = v
	}

	// Movies: only the one with a file, with its IDs and the translation.
	loadTestLanguages(t)
	database.SetTranslation("dbmovie", 1, "title", "de", "Fremd", "Alien")

	var movies jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+byType["movies"].ID+"&Recursive=true&IncludeItemTypes=Movie&Fields=ProviderIds", &movies)

	if movies.TotalRecordCount != 1 || movies.Items[0].Name != "Alien" || movies.Items[0].ProviderIDs["Imdb"] != "tt0078748" ||
		movies.Items[0].ProviderIDs["Tmdb"] != "348" || movies.Items[0].ProductionYear != 1979 || len(movies.Items[0].Genres) != 2 ||
		movies.Items[0].ImageTags["Primary"] == "" || movies.Items[0].UserData == nil {
		t.Fatalf("movies: %+v", movies)
	}

	movieID := movies.Items[0].ID

	hDE := h.Clone()
	hDE.Set("Accept-Language", "de-DE")

	var moviesDE jfTestResult
	jfGetJSON(t, f, hDE, "/jellyfin/Items?ParentId="+byType["movies"].ID, &moviesDE)

	if moviesDE.Items[0].Name != "Fremd" || moviesDE.Items[0].OriginalTitle != "Alien" {
		t.Fatalf("translated: %+v", moviesDE.Items[0])
	}

	var found jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?searchTerm=fremd&IncludeItemTypes=Movie,Series&Recursive=true", &found)

	if found.TotalRecordCount != 1 || found.Items[0].ID != movieID {
		t.Fatalf("search by translation: %+v", found)
	}

	// Series: seasons, episodes.
	var series jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+byType["tvshows"].ID, &series)

	if series.TotalRecordCount != 1 || series.Items[0].Type != "Series" || series.Items[0].RecursiveItemCount != 2 {
		t.Fatalf("series: %+v", series)
	}

	var seasons jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Shows/"+series.Items[0].ID+"/Seasons", &seasons)

	if len(seasons.Items) != 1 || seasons.Items[0].Name != "Season 1" || *seasons.Items[0].UserData.UnplayedItemCount != 2 {
		t.Fatalf("seasons: %+v", seasons)
	}

	var eps jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Shows/"+series.Items[0].ID+"/Episodes?SeasonId="+seasons.Items[0].ID, &eps)

	if len(eps.Items) != 2 || eps.Items[0].Name != "Pilot" || *eps.Items[1].IndexNumber != 2 || eps.Items[0].SeriesName != "Show" ||
		eps.Items[0].RunTimeTicks != 1200*1e7 {
		t.Fatalf("episodes: %+v", eps)
	}

	// One item with its file, played as it is.
	var item jfItem
	jfGetJSON(t, f, h, "/jellyfin/Users/"+jfUserID("admin")+"/Items/"+movieID, &item)

	if len(item.MediaSources) != 1 || item.MediaSources[0].ID != movieID || item.MediaSources[0].Container != "mkv" ||
		!item.MediaSources[0].SupportsDirectPlay {
		t.Fatalf("item: %+v", item)
	}

	token := strings.TrimSuffix(h.Get("Authorization")[strings.Index(h.Get("Authorization"), `Token="`)+7:], `"`)
	if w := f.do(t, http.MethodGet, "/jellyfin/Videos/"+movieID+"/stream.mkv?static=true&mediaSourceId="+item.MediaSources[0].ID+"&api_key="+token, nil, nil); w.Code != http.StatusOK || w.Body.String() != "data of movie.mkv" {
		t.Fatalf("stream: %d %q", w.Code, w.Body)
	}

	// Progress: in the middle (Resume), then to the end (played, NextUp).
	report := func(path, item string, pos int64) {
		t.Helper()

		body := `{"ItemId":"` + item + `","PositionTicks":` + itoa64(pos) + `}`
		if w := f.do(t, http.MethodPost, "/jellyfin/Sessions/Playing"+path, strings.NewReader(body), h); w.Code != http.StatusNoContent {
			t.Fatalf("report %s: %d %s", path, w.Code, w.Body)
		}
	}

	resetNowPlaying()
	report("", eps.Items[0].ID, 0)

	playing := nowPlayingList()
	if len(playing) != 1 || playing[0].Kind != "episode" || playing[0].ID != "21" || playing[0].Title != "Show - Pilot" ||
		!strings.HasPrefix(playing[0].Client, "Test (Box, ") {
		t.Fatalf("now playing: %+v", playing)
	}

	playedFor(t, playing[0].SID, 50*time.Second)
	report("/Progress", eps.Items[0].ID, 30*1e7)
	playedFor(t, playing[0].SID, 50*time.Second)
	report("/Progress", eps.Items[0].ID, 300*1e7)

	var resume jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/UserItems/Resume?MediaTypes=Video", &resume)

	if len(resume.Items) != 1 || resume.Items[0].ID != eps.Items[0].ID || resume.Items[0].UserData.PlaybackPositionTicks != 300*1e7 {
		t.Fatalf("resume: %+v", resume)
	}

	if w := f.do(t, http.MethodPost, "/jellyfin/UserPlayedItems/"+eps.Items[0].ID, nil, h); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"Played":true`) {
		t.Fatalf("mark played: %d %s", w.Code, w.Body)
	}

	var next jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Shows/NextUp?UserId="+jfUserID("admin"), &next)

	if len(next.Items) != 1 || next.Items[0].ID != eps.Items[1].ID {
		t.Fatalf("next up: %+v", next)
	}

	var unplayed jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+series.Items[0].ID+"&Recursive=true&IncludeItemTypes=Episode&Filters=IsUnplayed", &unplayed)

	if unplayed.TotalRecordCount != 1 || unplayed.Items[0].ID != eps.Items[1].ID {
		t.Fatalf("unplayed: %+v", unplayed)
	}

	// Music and audiobooks.
	var albums jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+byType["music"].ID+"&IncludeItemTypes=MusicAlbum&Recursive=true", &albums)

	if albums.TotalRecordCount != 1 || albums.Items[0].Name != "Record" || albums.Items[0].AlbumArtist != "Band" {
		t.Fatalf("albums: %+v", albums)
	}

	var tracks jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+albums.Items[0].ID, &tracks)

	if tracks.TotalRecordCount != 1 || tracks.Items[0].Name != "Song" || tracks.Items[0].Type != "Audio" {
		t.Fatalf("tracks: %+v", tracks)
	}

	var artists jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Artists/AlbumArtists?ParentId="+byType["music"].ID, &artists)

	if artists.TotalRecordCount != 1 || artists.Items[0].Name != "Band" {
		t.Fatalf("artists: %+v", artists)
	}

	var books jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+byType["books"].ID, &books)

	// An audiobook is a MusicAlbum of its parts: the web client lists and
	// plays only those (a Folder showed neither parts nor a play button).
	if books.TotalRecordCount != 1 || books.Items[0].Name != "Tale" || !books.Items[0].IsFolder || books.Items[0].Type != "MusicAlbum" {
		t.Fatalf("audiobooks: %+v", books)
	}

	var byAlbumType jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+byType["books"].ID+"&IncludeItemTypes=MusicAlbum&Recursive=true", &byAlbumType)

	if byAlbumType.TotalRecordCount != 1 || byAlbumType.Items[0].ID != books.Items[0].ID {
		t.Fatalf("audiobooks as MusicAlbum: %+v", byAlbumType)
	}

	var parts jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+books.Items[0].ID, &parts)

	if parts.TotalRecordCount == 0 || parts.Items[0].Type != "AudioBook" || parts.Items[0].MediaType != "Audio" {
		t.Fatalf("audiobook parts: %+v", parts)
	}

	// Images: a wrong tag is refused without a login.
	if w := f.do(t, http.MethodGet, "/jellyfin/Items/"+movieID+"/Images/Primary?tag=wrong", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("image with a wrong tag: %d", w.Code)
	}

	// anna: only the movie group; series and their files stay hidden.
	ha := jfLogin(t, f, "anna", "secret12")

	var annaViews jfTestResult
	jfGetJSON(t, f, ha, "/jellyfin/UserViews", &annaViews)

	if len(annaViews.Items) != 1 || annaViews.Items[0].CollectionType != "movies" {
		t.Fatalf("anna's views: %+v", annaViews)
	}

	for _, target := range []string{"/jellyfin/Items/" + eps.Items[0].ID, "/jellyfin/Videos/" + eps.Items[0].ID + "/stream?static=true"} {
		if w := f.do(t, http.MethodGet, target, nil, ha); w.Code != http.StatusNotFound {
			t.Errorf("anna reads %s: %d", target, w.Code)
		}
	}

	// A new password ends the app's login.
	u, _ := database.GetUserByName(new("anna"))
	if err := database.SetUserPassword(&u.ID, "another12"); err != nil {
		t.Fatal(err)
	}

	jfTokens.Clear()

	if w := f.do(t, http.MethodGet, "/jellyfin/UserViews", nil, ha); w.Code != http.StatusUnauthorized {
		t.Fatalf("old token after a new password: %d", w.Code)
	}
}

func itoa64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestJFOrder(t *testing.T) {
	for _, tc := range []struct{ by, order, want string }{
		{"SortName", "Ascending", "x.name COLLATE NOCASE"},
		{"DateCreated,SortName", "Descending", "x.created DESC, x.name COLLATE NOCASE DESC"},
		{"Random", "", "random()"},
		{"Unknown", "", "def"},
	} {
		if got := jfOrder(tc.by, tc.order, "def"); got != tc.want {
			t.Errorf("%s %s: %q, want %q", tc.by, tc.order, got, tc.want)
		}
	}
}

// TestJellyfinPlaylistsAndFavorites: favorites are kept (the favorites
// playlist), playlists can be made, listed, changed and deleted.
func TestJellyfinPlaylistsAndFavorites(t *testing.T) {
	f := clientTestData(t)
	h := jfLogin(t, f, DefaultUsername, "adminpass")
	movie, episode := jfID(jfMovie, 1), jfID(jfEpisode, 5)

	if w := f.do(t, http.MethodPost, "/jellyfin/UserFavoriteItems/"+movie, nil, h); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"IsFavorite":true`) {
		t.Fatalf("favorite: %d %s", w.Code, w.Body)
	}

	var favs jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?Recursive=true&IncludeItemTypes=Movie,Series&Filters=IsFavorite", &favs)

	if favs.TotalRecordCount != 1 || favs.Items[0].ID != movie || !favs.Items[0].UserData.IsFavorite {
		t.Fatalf("favorites: %+v", favs)
	}

	h.Set("Content-Type", "application/json")

	w := f.do(t, http.MethodPost, "/jellyfin/Playlists", strings.NewReader(`{"Name":"Watch","Ids":["`+movie+`","`+episode+`"]}`), h)
	if w.Code != http.StatusOK {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}

	var created struct{ ID string }
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	var views jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/UserViews", &views)

	if last := views.Items[len(views.Items)-1]; last.CollectionType != "playlists" {
		t.Fatalf("views: %+v", views)
	}

	var lists jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+jfPlaylistsViewID, &lists)

	if len(lists.Items) != 2 { // Favorites and Watch
		t.Fatalf("playlists: %+v", lists)
	}

	var entries jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Playlists/"+created.ID+"/Items", &entries)

	if len(entries.Items) != 2 || entries.Items[0].ID != movie || entries.Items[1].ID != episode || entries.Items[1].PlaylistItemID == "" {
		t.Fatalf("entries: %+v", entries)
	}

	if w := f.do(t, http.MethodDelete, "/jellyfin/Playlists/"+created.ID+"/Items?EntryIds="+entries.Items[0].PlaylistItemID, nil, h); w.Code != http.StatusNoContent {
		t.Fatalf("remove: %d", w.Code)
	}

	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+created.ID, &entries)

	if len(entries.Items) != 1 || entries.Items[0].ID != episode {
		t.Fatalf("after remove: %+v", entries)
	}

	// anna neither sees nor deletes the admin's playlist.
	ha := jfLogin(t, f, "anna", "secret12")
	if w := f.do(t, http.MethodDelete, "/jellyfin/Items/"+created.ID, nil, ha); w.Code != http.StatusForbidden {
		t.Fatalf("anna deletes: %d", w.Code)
	}

	if w := f.do(t, http.MethodDelete, "/jellyfin/Items/"+created.ID, nil, h); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/Items/"+created.ID, nil, h); w.Code != http.StatusNotFound {
		t.Fatalf("deleted playlist: %d", w.Code)
	}

	if w := f.do(t, http.MethodDelete, "/jellyfin/UserFavoriteItems/"+movie, nil, h); !strings.Contains(w.Body.String(), `"IsFavorite":false`) {
		t.Fatalf("unfavorite: %s", w.Body)
	}
}
