package api

import (
	"context"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestSyncWatched: watched on the server becomes watched here, watched
// here is marked there, a position there is taken over without history.
func TestSyncWatched(t *testing.T) {
	clientTestData(t)

	if err := database.ExecNErr("update dbseries set thetvdb_id = 555 where id = 1"); err != nil {
		t.Fatal(err)
	}

	fid := uint(22)
	if err := database.SetWatched("admin", "episode", &fid, true); err != nil {
		t.Fatal(err)
	}

	items := []syncItem{
		{Key: "m1", Imdb: "tt0078748", Watched: true},
		{Key: "e1", Episode: true, Tvdb: 555, Season: 1, Number: 1, Position: 300, Total: 1200},
		{Key: "e2", Episode: true, Tvdb: 555, Season: 1, Number: 2},
		{Key: "x", Imdb: "tt9999999", Watched: true},
	}

	var marked []string

	in, out := syncWatched(context.Background(), "admin", groupsOf(adminAccess), items, func(_ context.Context, key string) error {
		marked = append(marked, key)
		return nil
	})

	if in != 2 || out != 1 || len(marked) != 1 || marked[0] != "e2" {
		t.Fatalf("imported %d, exported %d, marked %v", in, out, marked)
	}

	if c := database.PlayCompletions("admin", "movie", "movie_files", "movie_id", new(uint64(1))); c[11] < database.Watched {
		t.Fatalf("movie not watched: %v", c)
	}

	if p := database.PlayResume("admin", "episode", new(uint(21))); p != 300 {
		t.Fatalf("position: %v", p)
	}

	// Again: nothing left to do, the position isn't taken over twice.
	in, out = syncWatched(context.Background(), "admin", groupsOf(adminAccess), items, func(context.Context, string) error { return nil })
	if in != 0 || out != 1 {
		t.Fatalf("second run: %d %d", in, out)
	}
}
