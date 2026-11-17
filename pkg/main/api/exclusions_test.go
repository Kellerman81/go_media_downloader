package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestDeleteWithExclusion: deleting with "don't add again" excludes the
// title for the entry's media group, and the collection check sees it.
func TestDeleteWithExclusion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	if err := database.ExecNErr("update dbmovies set moviedb_id = 77 where id = 2"); err != nil {
		t.Fatal(err)
	}

	cfgp := mediaConfigOfList("DE List")
	if !groupHasTMDB(cfgp, 77) || groupExcludesTMDB(cfgp, 77) {
		t.Fatal("before: the group should have the movie and not exclude it")
	}

	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}
	if w := callAs(admin, HandleLibraryDelete, url.Values{"type": {"movie"}, "id": {"2"}, "exclude": {"1"}}); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body)
	}

	if !database.ImportExcluded("movie", "movie_DE", new(uint(2))) || database.ImportExcluded("movie", "movie_EN", new(uint(2))) {
		t.Error("not excluded for DE only")
	}

	if groupHasTMDB(cfgp, 77) || !groupExcludesTMDB(cfgp, 77) {
		t.Error("after: the group should miss and exclude the movie")
	}
}
