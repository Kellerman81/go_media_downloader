package api

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// TestCoverThumb: a large cover gets a 400 px thumbnail made once, a small
// one is served as it is, and the index finds what the cache holds.
func TestCoverThumb(t *testing.T) {
	g := config.GetSettingsGeneral()
	oldDir := g.CoverCacheDir
	g.CoverCacheDir = filepath.Join(t.TempDir(), "covers")
	t.Cleanup(func() { g.CoverCacheDir = oldDir })

	dir := coverCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	big, small := filepath.Join(dir, "movie-1-aaa.png"), filepath.Join(dir, "album-2-bbb.png")
	writePNG(t, big, 1000, 1500)
	writePNG(t, small, 300, 300)

	if got, ok := coverIndex.lookup(dir, "movie-1-aaa"); !ok || got != big {
		t.Fatalf("index: %q %v", got, ok)
	}

	thumb := coverThumb(big)
	if !strings.HasSuffix(thumb, "movie-1-aaa-t400.jpg") {
		t.Fatalf("thumbnail %q", thumb)
	}

	f, err := os.Open(thumb)
	if err != nil {
		t.Fatal(err)
	}

	cfg, _, err := image.DecodeConfig(f)
	f.Close()

	if err != nil || cfg.Width != 400 || cfg.Height != 600 {
		t.Errorf("thumbnail %dx%d, %v", cfg.Width, cfg.Height, err)
	}

	if again := coverThumb(big); again != thumb {
		t.Errorf("second call made %q", again)
	}

	if got := coverThumb(small); got != small {
		t.Errorf("small cover served as %q", got)
	}
}
