package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestCleanupFileCache: covers and subtitles nothing refers to any more go,
// with stale temporary files; everything in use (and unknown files) stays.
func TestCleanupFileCache(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	g := config.GetSettingsGeneral()
	oldDir := g.CoverCacheDir
	g.CoverCacheDir = filepath.Join(t.TempDir(), "covers")
	t.Cleanup(func() { g.CoverCacheDir = oldDir })

	media := t.TempDir()
	video := filepath.Join(media, "movie.mkv")
	loose := filepath.Join(media, "loose.mkv")
	sidecar := filepath.Join(media, "movie.de.srt")

	for _, f := range []string{video, loose, sidecar} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, q := range []string{
		"insert into dbmovies (id, title) values (1, 'Kept'), (2, 'Not in a list')",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN')",
		"insert into dbseries (id, seriename) values (1, 'S')",
		"insert into series (id, dbserie_id, listname) values (1, 1, 'EN')",
		"insert into serie_episode_files (id, location, dbserie_id) values (7, '/v/e.mkv', 1)",
		"insert into movie_files (id, location, dbmovie_id, movie_id) values (1, ?, 1, 1)",
	} {
		args := []any{}
		if strings.Contains(q, "?") {
			args = append(args, video)
		}

		if err := database.ExecNErr(q, args...); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-2 * time.Hour)
	write := func(dir, name string, mod time.Time) {
		t.Helper()

		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}

		path := filepath.Join(dir, name)
		if strings.HasPrefix(name, "convert-") {
			if err := os.Mkdir(path, 0o755); err != nil {
				t.Fatal(err)
			}
		} else if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}

		if !mod.IsZero() {
			os.Chtimes(path, mod, mod)
		}
	}

	covers := coverCacheDir()
	for _, n := range []string{
		"movie-1-aaa.jpg", "movie-2-bbb.jpg", "movie-3-ccc.jpg", "series-1-ddd.png",
		"epthumb-7-eee.jpg", "epthumb-8-fff.jpg", "audiobook-5-ggg.jpg", "notes.txt", "movie-1-hhh-123.tmp",
	} {
		write(covers, n, time.Time{})
	}

	write(covers, "movie-9-iii-456.tmp", old)

	st, _ := os.Stat(video)
	subs := subCacheDir()

	current := subCacheName(video, st.ModTime(), "0:s:0")
	sideSt, _ := os.Stat(sidecar)
	side := subCacheName(sidecar, sideSt.ModTime(), "0:s:0")

	for _, n := range []string{
		current,
		subCacheName(video, st.ModTime().Add(-time.Hour), "0:s:0"),   // replaced file
		subCacheName(filepath.Join(media, "gone.mkv"), old, "0:s:0"), // removed file
		subCacheName(loose, st.ModTime(), "0:s:0"),                   // not downloaded
		side,                               // subtitle file
		strings.Repeat("ab", 20) + ".json", // old naming
		subCacheName(filepath.Join(media, "nosrc.mkv"), old, "0:s:1"), // without note
		"convert-1", "notes.txt",
	} {
		write(subs, n, time.Time{})
	}

	write(subs, "convert-2", old)

	// The source notes; orphan.mkv's is old and has no conversions.
	orphan := filepath.Join(media, "orphan.mkv")
	for _, src := range []string{video, filepath.Join(media, "gone.mkv"), loose, sidecar, orphan} {
		if err := os.WriteFile(filepath.Join(subs, subSourceKey(src)+subSourceExt), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	os.Chtimes(filepath.Join(subs, subSourceKey(orphan)+subSourceExt), old, old)

	res, err := CleanupFileCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	names := func(dir string) []string {
		var out []string

		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			out = append(out, e.Name())
		}

		return out
	}

	wantCovers := []string{"epthumb-7-eee.jpg", "movie-1-aaa.jpg", "movie-1-hhh-123.tmp", "notes.txt", "series-1-ddd.png"}
	if got := names(covers); !slices.Equal(got, wantCovers) {
		t.Errorf("covers left: %v, want %v", got, wantCovers)
	}

	wantSubs := []string{
		current, side, "convert-1", "notes.txt",
		subSourceKey(video) + subSourceExt, subSourceKey(sidecar) + subSourceExt,
	}
	slices.Sort(wantSubs)

	if got := names(subs); !slices.Equal(got, wantSubs) {
		t.Errorf("subtitles left: %v, want %v", got, wantSubs)
	}

	if res.Covers != 4 || res.Subtitles != 5 || res.Temp != 2 || res.Bytes == 0 {
		t.Errorf("result %+v", res)
	}
}
