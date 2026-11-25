package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// useSavedViews stores saved views as the Library page does.
func useSavedViews(t *testing.T, user string, views []savedView) {
	t.Helper()

	b, _ := json.Marshal(views)
	settings, _ := json.Marshal(map[string]string{"lib-saved-views": string(b)})

	if err := database.SaveUserSettings(user, string(settings)); err != nil {
		t.Fatal(err)
	}
}

// TestSmartPlaylists: saved views are playlists of what they find, in the
// Jellyfin API, Subsonic and DLNA.
func TestSmartPlaylists(t *testing.T) {
	f := clientTestData(t)

	useSavedViews(t, "admin", []savedView{
		{Name: "Space films", Params: "group=movie_EN&filter=all&q=alien"},
		{Name: "My music", Params: "group=music_M&filter=all"},
	})

	h := jfLogin(t, f, DefaultUsername, "adminpass")

	var lists jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Items?ParentId="+jfPlaylistsViewID, &lists)

	if len(lists.Items) != 2 || lists.Items[1].Name != "My music" {
		t.Fatalf("playlists: %+v", lists)
	}

	var films jfTestResult
	jfGetJSON(t, f, h, "/jellyfin/Playlists/"+lists.Items[0].ID+"/Items", &films)

	if len(films.Items) != 1 || films.Items[0].Name != "Alien" {
		t.Fatalf("smart playlist: %+v", films)
	}

	w := f.do(t, http.MethodGet, "/subsonic/rest/getPlaylists.view?u=admin&p=adminpass", nil, nil)
	if body := w.Body.String(); !strings.Contains(body, `name="My music"`) || strings.Contains(body, "Space films") {
		t.Fatalf("subsonic playlists: %s", body)
	}

	id := "sv" + strconv.FormatUint(smartID(&savedView{Name: "My music"}), 10)
	if body := f.do(t, http.MethodGet, "/subsonic/rest/getPlaylist.view?u=admin&p=adminpass&id="+id, nil, nil).Body.String(); !strings.Contains(body, `<entry id="tr31"`) {
		t.Fatalf("subsonic smart playlist: %s", body)
	}

	sv := "sv/" + strconv.FormatUint(smartID(&savedView{Name: "Space films"}), 10)
	if items, total := dlnaBrowse(t, f, sv, "BrowseDirectChildren"); total != "1" || !strings.Contains(items, ">Alien<") {
		t.Fatalf("dlna (%s): %s", total, items)
	}

	// A view of a group the account can't see finds nothing.
	useSavedViews(t, "anna", []savedView{{Name: "Not mine", Params: "group=serie_X&filter=all"}})

	if code, ids := smartEntries(&userAccess{Name: "anna", Role: database.RoleUser, Groups: []string{"movie_EN"}}, "en",
		&savedView{Name: "Not mine", Params: "group=serie_X&filter=all"}); code != 0 || len(ids) != 0 {
		t.Fatalf("foreign group: %d %v", code, ids)
	}
}
