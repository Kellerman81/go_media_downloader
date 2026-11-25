package api

import (
	"crypto/md5" //nolint:gosec // the protocol's token
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestSubsonic(t *testing.T) {
	f := clientTestData(t)

	call := func(method, query string) string {
		t.Helper()

		w := f.do(t, http.MethodGet, "/subsonic/rest/"+method+".view?u=admin&p=adminpass&v=1.16.1&c=test&"+query, nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", method, w.Code)
		}

		return w.Body.String()
	}

	if body := call("ping", ""); !strings.Contains(body, `status="ok"`) || !strings.Contains(body, `xmlns="http://subsonic.org/restapi"`) {
		t.Fatalf("ping: %s", body)
	}

	// Logins: a wrong password, the hex form, the admin's token, an account's token.
	if w := f.do(t, http.MethodGet, "/subsonic/rest/ping.view?u=admin&p=wrong", nil, nil); !strings.Contains(w.Body.String(), `code="40"`) {
		t.Fatalf("wrong password: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/subsonic/rest/ping.view?u=anna&p=enc:"+hex.EncodeToString([]byte("secret12")), nil, nil); !strings.Contains(w.Body.String(), `status="ok"`) {
		t.Fatalf("enc password: %s", w.Body)
	}

	sum := md5.Sum([]byte("adminpass" + "salt1")) //nolint:gosec // the protocol's token
	if w := f.do(t, http.MethodGet, "/subsonic/rest/ping?u=admin&s=salt1&t="+hex.EncodeToString(sum[:]), nil, nil); !strings.Contains(w.Body.String(), `status="ok"`) {
		t.Fatalf("admin token: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/subsonic/rest/ping?u=anna&s=salt1&t=00", nil, nil); !strings.Contains(w.Body.String(), `code="41"`) {
		t.Fatalf("account token: %s", w.Body)
	}

	var res struct {
		R struct {
			Status  string `json:"status"`
			Artists struct {
				Index []struct {
					Name   string `json:"name"`
					Artist []struct {
						ID, Name string
					} `json:"artist"`
				} `json:"index"`
			} `json:"artists"`
		} `json:"subsonic-response"`
	}

	if err := json.Unmarshal([]byte(call("getArtists", "f=json")), &res); err != nil || res.R.Status != "ok" ||
		len(res.R.Artists.Index) != 1 || res.R.Artists.Index[0].Name != "B" || res.R.Artists.Index[0].Artist[0].ID != "ar1" {
		t.Fatalf("getArtists: %+v %v", res, err)
	}

	album := call("getAlbum", "id=al1")
	for _, want := range []string{`name="Record"`, `artist="Band"`, `songCount="1"`, `id="tr31"`, `title="Song"`, `suffix="mp3"`, `duration="180"`, `genre="rock"`} {
		if !strings.Contains(album, want) {
			t.Errorf("getAlbum lacks %s: %s", want, album)
		}
	}

	if body := call("getMusicDirectory", "id=ar1"); !strings.Contains(body, `id="al1"`) || !strings.Contains(body, `isDir="true"`) {
		t.Errorf("artist folder: %s", body)
	}

	for _, typ := range []string{"newest", "random", "alphabeticalByName", "alphabeticalByArtist", "byGenre&genre=pop", "byYear&fromYear=2000&toYear=2002"} {
		if body := call("getAlbumList2", "type="+typ); !strings.Contains(body, `id="al1"`) {
			t.Errorf("getAlbumList2 %s: %s", typ, body)
		}
	}

	if body := call("getAlbumList2", "type=byGenre&genre=jazz"); strings.Contains(body, `id="al1"`) {
		t.Errorf("wrong genre: %s", body)
	}

	if body := call("search3", "query=song"); !strings.Contains(body, `id="tr31"`) {
		t.Errorf("search3: %s", body)
	}

	if body := call("getGenres", ""); !strings.Contains(body, `>rock<`) || !strings.Contains(body, `>pop<`) {
		t.Errorf("getGenres: %s", body)
	}

	if w := f.do(t, http.MethodGet, "/subsonic/rest/stream.view?u=admin&p=adminpass&id=tr31", nil, nil); w.Code != http.StatusOK || w.Body.String() != "data of 01 - Song.mp3" {
		t.Fatalf("stream: %d %q", w.Code, w.Body)
	}

	// A played song: history, then "frequent" and "recent" lists it.
	if body := call("scrobble", "id=tr31&submission=true"); !strings.Contains(body, `status="ok"`) {
		t.Fatalf("scrobble: %s", body)
	}

	if n := database.Getdatarow[int](false, "select plays from play_history where username = 'admin' and kind = 'track' and file_id = 31"); n != 1 {
		t.Fatalf("plays after scrobble: %d", n)
	}

	for _, typ := range []string{"frequent", "recent"} {
		if body := call("getAlbumList2", "type="+typ); !strings.Contains(body, `id="al1"`) {
			t.Errorf("getAlbumList2 %s after a play: %s", typ, body)
		}
	}

	// anna has no music group.
	if w := f.do(t, http.MethodGet, "/subsonic/rest/getAlbum.view?u=anna&p=secret12&id=al1", nil, nil); !strings.Contains(w.Body.String(), `code="70"`) {
		t.Fatalf("anna's album: %s", w.Body)
	}

	if w := f.do(t, http.MethodGet, "/subsonic/rest/stream.view?u=anna&p=secret12&id=tr31", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("anna's stream: %d", w.Code)
	}
}

func TestSubIndexKey(t *testing.T) {
	for name, want := range map[string]string{"The Band": "B", "abba": "A", "2Pac": "#", "Ärzte": "Ä", "The": "T"} {
		if got := subIndexKey(name); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
}

// TestSubsonicTranscode: a lower maxBitRate than the file's converts it to
// MP3; the file's own rate or more sends it as it is.
func TestSubsonicTranscode(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	f := clientTestData(t)

	song := filepath.Join(f.dir, "01 - Song.mp3")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=duration=5",
		"-c:a", "libmp3lame", "-b:a", "320k", song).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}

	st, _ := os.Stat(song)
	if err := database.ExecNErr("update album_files set file_size = ?, runtime_ms = 5000 where id = 31", st.Size()); err != nil {
		t.Fatal(err)
	}

	orig := f.do(t, http.MethodGet, "/subsonic/rest/stream.view?u=admin&p=adminpass&id=tr31&maxBitRate=400", nil, nil)
	if orig.Code != http.StatusOK || int64(orig.Body.Len()) != st.Size() {
		t.Fatalf("own rate: %d, %d bytes of %d", orig.Code, orig.Body.Len(), st.Size())
	}

	low := f.do(t, http.MethodGet, "/subsonic/rest/stream.view?u=admin&p=adminpass&id=tr31&maxBitRate=64", nil, nil)
	if low.Code != http.StatusOK || low.Body.Len() == 0 || int64(low.Body.Len()) >= st.Size()/2 || low.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("64 kbit/s: %d, %d bytes of %d, %s", low.Code, low.Body.Len(), st.Size(), low.Header().Get("Content-Type"))
	}
}

// TestSubsonicStarsAndPlaylists: stars are the favorites playlist; the
// Library's playlists with music are Subsonic playlists.
func TestSubsonicStarsAndPlaylists(t *testing.T) {
	f := clientTestData(t)

	call := func(method, query string) string {
		t.Helper()
		return f.do(t, http.MethodGet, "/subsonic/rest/"+method+".view?u=admin&p=adminpass&"+query, nil, nil).Body.String()
	}

	call("star", "id=tr31&albumId=al1")

	if body := call("getStarred2", ""); !strings.Contains(body, `<song id="tr31"`) || !strings.Contains(body, `<album id="al1"`) {
		t.Fatalf("starred: %s", body)
	}

	if body := call("getAlbumList2", "type=starred"); !strings.Contains(body, `id="al1"`) {
		t.Fatalf("starred albums: %s", body)
	}

	if !isFavorite("admin", "track", 1, 31) || !isFavorite("admin", "album", 1, 0) {
		t.Fatal("not in the favorites playlist")
	}

	call("unstar", "id=tr31")

	if body := call("getStarred2", ""); strings.Contains(body, `<song id="tr31"`) {
		t.Fatalf("unstarred: %s", body)
	}

	// A playlist of movies only isn't a music playlist.
	movies := database.Playlist{Username: "admin", Name: "Films"}
	if err := database.SavePlaylist(&movies); err != nil {
		t.Fatal(err)
	}

	if _, err := database.AddPlaylistItems("admin", &movies.ID, []database.PlaylistItem{{Kind: "movie", EntryID: 1, Title: "Alien"}}); err != nil {
		t.Fatal(err)
	}

	body := call("createPlaylist", "name=Mix&songId=tr31")
	if !strings.Contains(body, `name="Mix"`) || !strings.Contains(body, `songCount="1"`) || !strings.Contains(body, `<entry id="tr31"`) {
		t.Fatalf("create: %s", body)
	}

	id := regexp.MustCompile(`<playlist id="(\d+)" name="Mix"`).FindStringSubmatch(body)[1]

	lists := call("getPlaylists", "")
	if !strings.Contains(lists, `name="Mix"`) || !strings.Contains(lists, `name="Favorites"`) || strings.Contains(lists, `name="Films"`) {
		t.Fatalf("playlists: %s", lists)
	}

	call("updatePlaylist", "playlistId="+id+"&name=Mix2&songIndexToRemove=0")

	if body := call("getPlaylist", "id="+id); !strings.Contains(body, `name="Mix2"`) || !strings.Contains(body, `songCount="0"`) {
		t.Fatalf("after update: %s", body)
	}

	call("deletePlaylist", "id="+id)

	if body := call("getPlaylist", "id="+id); !strings.Contains(body, `code="70"`) {
		t.Fatalf("after delete: %s", body)
	}
}
