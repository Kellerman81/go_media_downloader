package api

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png" // covers are JPEG or PNG
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Covers are shown small everywhere (grid, details, player, calendar), so
// the cover URL serves a 400 px wide thumbnail, made once next to the
// cached cover (<base>-t400.jpg); ?size=full serves the cover itself.
// Which cached files exist is kept in memory (coverIndex), so a grid of a
// hundred covers doesn't list the cache folder a hundred times.

const coverThumbWidth = 400

// coverIdx maps the cache file names without extension to their paths.
type coverIdx struct {
	mu    sync.RWMutex
	dir   string
	files map[string]string
}

var coverIndex coverIdx

// load reads the cache folder once (again after cover_cache_dir changed).
func (ci *coverIdx) load(dir string) {
	ci.mu.RLock()
	ok := ci.files != nil && ci.dir == dir
	ci.mu.RUnlock()

	if ok {
		return
	}

	files := map[string]string{}

	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if name := e.Name(); !e.IsDir() && filepath.Ext(name) != ".tmp" {
				files[strings.TrimSuffix(name, filepath.Ext(name))] = filepath.Join(dir, name)
			}
		}
	}

	ci.mu.Lock()
	ci.dir, ci.files = dir, files
	ci.mu.Unlock()
}

// lookup returns the cached file named base (any extension) in dir.
func (ci *coverIdx) lookup(dir, base string) (string, bool) {
	ci.load(filepath.Clean(dir))

	ci.mu.RLock()
	path, ok := ci.files[base]
	ci.mu.RUnlock()

	return path, ok
}

// add records a file stored into the cache folder.
func (ci *coverIdx) add(path string) {
	ci.mu.Lock()
	if ci.files != nil && ci.dir == filepath.Dir(path) {
		name := filepath.Base(path)
		ci.files[strings.TrimSuffix(name, filepath.Ext(name))] = path
	}
	ci.mu.Unlock()
}

// remove forgets a file removed from the cache folder.
func (ci *coverIdx) remove(path string) {
	ci.mu.Lock()
	if ci.files != nil {
		name := filepath.Base(path)
		if ci.files[strings.TrimSuffix(name, filepath.Ext(name))] == path {
			delete(ci.files, strings.TrimSuffix(name, filepath.Ext(name)))
		}
	}
	ci.mu.Unlock()
}

// smallCovers are covers no wider than a thumbnail: served as they are.
var smallCovers sync.Map

// coverThumb returns the thumbnail of a cached cover, making it on first
// use; the cover itself when it is small already or can't be decoded.
func coverThumb(path string) string {
	if _, small := smallCovers.Load(path); small {
		return path
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + "-t400"

	if p, ok := coverIndex.lookup(dir, base); ok {
		return p
	}

	lock, _ := coverLocks.LoadOrStore(base, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	if p, ok := coverIndex.lookup(dir, base); ok {
		return p
	}

	f, err := os.Open(path)
	if err != nil {
		return path
	}

	src, _, err := image.Decode(f)
	f.Close()

	if err != nil || src.Bounds().Dx() <= coverThumbWidth*5/4 {
		smallCovers.Store(path, struct{}{})
		return path
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, shrinkImage(src, coverThumbWidth), &jpeg.Options{Quality: 85}); err != nil {
		return path
	}

	thumb, err := storeCover(dir, base, ".jpg", &buf)
	if err != nil {
		return path
	}

	coverIndex.add(thumb)

	return thumb
}

// shrinkImage scales src down to width w (keeping its aspect ratio),
// averaging the source pixels each target pixel covers.
func shrinkImage(src image.Image, w int) *image.RGBA {
	b := src.Bounds()
	h := max(1, b.Dy()*w/b.Dx())

	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := b.Dx(), b.Dy()

	for y := range h {
		y0, y1 := y*sh/h, max((y+1)*sh/h, y*sh/h+1)

		for x := range w {
			x0, x1 := x*sw/w, max((x+1)*sw/w, x*sw/w+1)

			var r, g, bl, a, n uint32

			for sy := y0; sy < y1; sy++ {
				row := rgba.Pix[sy*rgba.Stride:]
				for sx := x0; sx < x1; sx++ {
					p := row[sx*4 : sx*4+4]
					r, g, bl, a = r+uint32(p[0]), g+uint32(p[1]), bl+uint32(p[2]), a+uint32(p[3])
					n++
				}
			}

			o := dst.Pix[y*dst.Stride+x*4:]
			o[0], o[1], o[2], o[3] = uint8(r/n), uint8(g/n), uint8(bl/n), uint8(a/n) //nolint:gosec // averages of bytes
		}
	}

	return dst
}
