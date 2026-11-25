package api

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestDropNestedPathJobs(t *testing.T) {
	type job struct {
		path string
		typ  uint
	}

	jobs := []job{{"/data/series", 1}, {"/data/series/anime", 1}, {"/data/series/books", 2}, {"/data/movies", 0}}

	got := dropNestedPathJobs(jobs, func(j job) (string, uint) { return j.path, j.typ })
	want := []job{{"/data/series", 1}, {"/data/series/books", 2}, {"/data/movies", 0}}

	if !slices.Equal(got, want) {
		t.Fatalf("dropNestedPathJobs = %v, want %v", got, want)
	}
}

func TestWalkPath(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a/movie.mkv", "a/movie.nfo", "b/other.MP4"} {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("12345"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var progress atomic.Int64

	st := walkPath(context.Background(), dir, config.MediaTypeMovie, &progress)
	if st.FileCount != 2 || st.TotalSize != 10 || st.FolderCount != 3 || progress.Load() != 2 || st.Error != "" {
		t.Fatalf("walk: %+v, progress %d", st, progress.Load())
	}

	// A missing folder says so instead of vanishing.
	if st := walkPath(context.Background(), filepath.Join(dir, "gone"), config.MediaTypeMovie, &progress); st.Error != "folder not found" {
		t.Fatalf("missing folder: %+v", st)
	}
}

// TestStorageScan: the scan only runs when started, in the background, and
// the details show its state.
func TestStorageScan(t *testing.T) {
	render := func(st StorageStatistics) string {
		var b strings.Builder
		_ = renderStorageDetails(st).Render(&b)

		return b.String()
	}

	if out := render(StorageStatistics{}); !strings.Contains(out, "Not scanned yet") || !strings.Contains(out, "Scan now") {
		t.Errorf("never scanned:\n%s", out)
	}

	if out := render(StorageStatistics{Scanning: true, ScannedFiles: 42}); !strings.Contains(out, "42 media files so far") ||
		!strings.Contains(out, "load delay:3s") || !strings.Contains(out, "disabled") {
		t.Errorf("scanning:\n%s", out)
	}

	done := StorageStatistics{ScannedAt: time.Now(), Took: time.Minute, TotalPaths: 1,
		ByPath: map[string]PathStatistic{"movie_EN (Movies)": {Path: "/data/movies", Error: "folder not found"}}}
	if out := render(done); !strings.Contains(out, "folder not found") || !strings.Contains(out, "Scan again") || strings.Contains(out, "load delay") {
		t.Errorf("scanned:\n%s", out)
	}

	if !startStorageScan() {
		t.Fatal("scan didn't start")
	}

	for deadline := time.Now().Add(10 * time.Second); storageStatus().Scanning; {
		if time.Now().After(deadline) {
			t.Fatal("scan didn't finish")
		}

		time.Sleep(10 * time.Millisecond)
	}

	if st := storageStatus(); st.ScannedAt.IsZero() || st.ByPath == nil {
		t.Fatalf("scan result: %+v", st)
	}
}
