package api

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()

	writePNGTo(t, f, w, h)
}

func writePNGTo(t *testing.T, out io.Writer, w, h int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}

	if err := png.Encode(out, img); err != nil {
		t.Fatal(err)
	}
}

// TestCoverThumb: a large cover gets a 400 px thumbnail made once, a small
// one is served as it is, and the lookup finds what the cache holds.
func TestCoverThumb(t *testing.T) {
	// Without a loaded config the cache is the default folder: in a temp
	// folder, not in the package.
	t.Chdir(t.TempDir())

	dir := coverCacheDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	big, small := filepath.Join(dir, "movie-1-aaa.png"), filepath.Join(dir, "album-2-bbb.png")
	writePNG(t, big, 1000, 1500)
	writePNG(t, small, 300, 300)

	if got, ok := cachedFile(dir, "movie-1-aaa"); !ok || got != big {
		t.Fatalf("lookup: %q %v", got, ok)
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

func TestCoverExt(t *testing.T) {
	for in, want := range map[string]string{
		"image/jpeg": ".jpg", "image/jpeg; charset=binary": ".jpg", ".JPEG": ".jpg", ".jfif": ".jpg", "": ".jpg",
		"image/png": ".png", ".PNG": ".png", "image/webp": ".webp", ".gif": ".gif",
	} {
		if got := coverExt(in); got != want {
			t.Errorf("coverExt(%q) = %q, want %q", in, got, want)
		}
	}

	dir := t.TempDir()

	path, err := storeCover(dir, "movie-3-ccc", "image/jpeg", strings.NewReader("jpeg data"))
	if err != nil || filepath.Base(path) != "movie-3-ccc.jpg" {
		t.Fatalf("stored as %q: %v", path, err)
	}

	if got, ok := cachedFile(dir, "movie-3-ccc"); !ok || got != path {
		t.Errorf("lookup: %q %v", got, ok)
	}
}

// noiseJPEG returns a w×h JPEG of noise (large like a photo).
func noiseJPEG(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(1)

	for i := range img.Pix {
		seed = seed*1664525 + 1013904223
		img.Pix[i] = uint8(seed >> 24)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func imageWidth(t *testing.T, p string) int {
	t.Helper()

	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}

	return cfg.Width
}

// TestCoverSource: a cover is <kind>-<id>.jpg naming its source inside;
// the same source is served from the cache, a new one replaces the file
// (and its thumbnail); large covers are shrunk, PNGs become JPEGs.
func TestCoverSource(t *testing.T) {
	t.Chdir(t.TempDir())

	dir := coverCacheDir()
	loads := 0
	load := func(data []byte, typ string) func() ([]byte, string, error) {
		return func() ([]byte, string, error) {
			loads++
			return data, typ, nil
		}
	}

	big := noiseJPEG(t, 1600, 2400)

	p, err := cachedCoverWith("movie", 1, "https://a/poster.jpg", load(big, "image/jpeg"))
	if err != nil || filepath.Base(p) != "movie-1.jpg" || imageWidth(t, p) != coverMaxWidth {
		t.Fatalf("stored %q: %v", p, err)
	}

	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", st.Mode().Perm())
	}

	if got := coverSrcOf(p, st); got != coverSrcTag("https://a/poster.jpg") {
		t.Fatalf("source tag %q", got)
	}

	coverSrcs.Delete(p) // read from the file, not remembered

	if got := coverSrcOf(p, st); got != coverSrcTag("https://a/poster.jpg") {
		t.Fatalf("source tag from the file %q", got)
	}

	if thumb := coverThumb(p); !strings.HasSuffix(thumb, "movie-1-t400.jpg") || imageWidth(t, thumb) != coverThumbWidth {
		t.Fatalf("thumbnail %q", thumb)
	}

	if again, _ := cachedCoverWith("movie", 1, "https://a/poster.jpg", load(big, "image/jpeg")); again != p || loads != 1 {
		t.Fatalf("same source loaded again: %q, %d loads", again, loads)
	}

	// A new source: the same file, the old thumbnail gone.
	small := noiseJPEG(t, 300, 450)

	p2, err := cachedCoverWith("movie", 1, "https://b/new.jpg", load(small, "image/jpeg"))
	if err != nil || p2 != p || loads != 2 || imageWidth(t, p2) != 300 {
		t.Fatalf("new source: %q %v, %d loads", p2, err, loads)
	}

	if _, err := os.Stat(filepath.Join(dir, "movie-1-t400.jpg")); err == nil {
		t.Error("the old thumbnail is left")
	}

	// A PNG is stored as JPEG.
	var png bytes.Buffer
	writePNGTo(t, &png, 1200, 1200)

	p3, err := cachedCoverWith("album", 2, "local:/m/a.mp3", load(png.Bytes(), "image/png"))
	if err != nil || filepath.Base(p3) != "album-2.jpg" || imageWidth(t, p3) != coverMaxWidth {
		t.Fatalf("PNG stored as %q: %v", p3, err)
	}

	// WebP is stored as it is; the JPEG it replaces goes.
	p5, err := cachedCoverWith("album", 2, "https://c/cover.webp", load([]byte("RIFF....WEBPVP8 "), "image/webp"))
	if err != nil || filepath.Base(p5) != "album-2.webp" {
		t.Fatalf("WebP stored as %q: %v", p5, err)
	}

	if _, err := os.Stat(p3); err == nil {
		t.Error("the replaced JPEG is left")
	}

	// Cached before covers were shrunk (no source inside): any source
	// matches, and it is shrunk when looked up.
	if err := os.WriteFile(filepath.Join(dir, "album-3.jpg"), big, 0o644); err != nil {
		t.Fatal(err)
	}

	p4, err := cachedCoverWith("album", 3, "whatever", func() ([]byte, string, error) {
		t.Fatal("loaded again")
		return nil, "", nil
	})
	if err != nil || imageWidth(t, p4) != coverMaxWidth {
		t.Fatalf("old large cover: %q %v", p4, err)
	}
}
