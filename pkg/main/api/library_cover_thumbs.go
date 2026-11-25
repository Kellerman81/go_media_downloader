package api

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"image"
	"image/draw"
	_ "image/gif" // GIF covers are stored as JPEG
	"image/jpeg"
	_ "image/png" // PNG covers are stored as JPEG
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Covers are shown small everywhere (grid, details, player, calendar), so
// the cover URL serves a 400 px wide thumbnail, made next to the cached
// cover (<base>-t400.jpg, again when the cover is newer); ?size=full serves
// the cover itself.

const coverThumbWidth = 400

// coverExts are the extensions cached images are stored with, one per
// image type (coverExt), most common first.
var coverExts = []string{".jpg", ".png", ".webp", ".gif"}

// coverExt is the extension an image of a content type or file extension
// is stored with (JPEG as .jpg, whatever the source called it).
func coverExt(typeOrExt string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(typeOrExt, ";")[0])) {
	case "image/png", ".png":
		return ".png"
	case "image/webp", ".webp":
		return ".webp"
	case "image/gif", ".gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// cachedFile returns the cached image named base in dir.
func cachedFile(dir, base string) (string, bool) {
	p, _, ok := cachedFileStat(dir, base)
	return p, ok
}

// cachedFileStat is cachedFile with the file's info.
func cachedFileStat(dir, base string) (string, os.FileInfo, bool) {
	for _, ext := range coverExts {
		p := filepath.Join(dir, base+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, st, true
		}
	}

	return "", nil, false
}

// Covers are kept at most coverMaxWidth wide: pictures embedded in music
// files and some providers' covers are full-size scans of several MB.
const (
	coverMaxWidth = 1000
	coverMaxBytes = 1 << 20
)

// The source of a cached cover is named in a JPEG comment right after the
// image's start: coverSrcPrefix and the first 12 hex digits of the SHA-1
// of the source key (URL, "local:"+file, "frame:"+file, server name).
const coverSrcPrefix = "gmd-src:"

// coverSrcTag is the tag of a source key.
func coverSrcTag(key string) string {
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:6])
}

// coverSrcs remembers the tags read from cached covers (by path, valid
// while the file's time and size stay), so each file is read once.
var coverSrcs sync.Map

type coverSrc struct {
	mod  time.Time
	size int64
	tag  string
}

// coverSrcOf returns the source tag a cached cover names ("" none).
func coverSrcOf(path string, st os.FileInfo) string {
	if v, ok := coverSrcs.Load(path); ok {
		if c := v.(coverSrc); c.mod.Equal(st.ModTime()) && c.size == st.Size() {
			return c.tag
		}
	}

	tag := ""

	if f, err := os.Open(path); err == nil {
		head := make([]byte, 4+2+len(coverSrcPrefix)+12)
		if _, err := io.ReadFull(f, head); err == nil {
			tag = jpegSrcTag(head)
		}

		f.Close()
	}

	coverSrcs.Store(path, coverSrc{mod: st.ModTime(), size: st.Size(), tag: tag})

	return tag
}

// jpegSrcTag reads the tag from the start of a JPEG ("" none).
func jpegSrcTag(head []byte) string {
	if len(head) < 6 || head[0] != 0xFF || head[1] != 0xD8 || head[2] != 0xFF || head[3] != 0xFE {
		return ""
	}

	comment, ok := bytes.CutPrefix(head[6:], []byte(coverSrcPrefix))
	if !ok || len(comment) < 12 {
		return ""
	}

	return string(comment[:12])
}

// jpegWithSrc puts the source tag into a JPEG as its first segment.
func jpegWithSrc(data []byte, tag string) []byte {
	payload := coverSrcPrefix + tag
	out := make([]byte, 0, len(data)+4+len(payload))
	out = append(out, 0xFF, 0xD8, 0xFF, 0xFE, byte((len(payload)+2)>>8), byte(len(payload)+2))
	out = append(out, payload...)

	return append(out, data[2:]...)
}

// saveCover stores an item's cover from the source tag: a JPEG (PNG and
// GIF converted) of at most coverMaxWidth naming its source, WebP as it is.
// Another extension and the thumbnail of the cover it replaces go.
func saveCover(dir, base, tag string, data []byte, typeOrExt string) (string, error) {
	if http.DetectContentType(data) == "image/jpeg" {
		typeOrExt = ".jpg"
	}

	ext := coverExt(typeOrExt)

	if ext != ".webp" {
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil &&
			(ext != ".jpg" || cfg.Width > coverMaxWidth || len(data) > coverMaxBytes) {
			if img, _, err := image.Decode(bytes.NewReader(data)); err == nil {
				if cfg.Width > coverMaxWidth {
					img = shrinkImage(img, coverMaxWidth)
				}

				var buf bytes.Buffer
				if err := jpeg.Encode(&buf, onWhite(img), &jpeg.Options{Quality: 85}); err == nil &&
					(ext != ".jpg" || buf.Len() < len(data)) {
					data, ext = buf.Bytes(), ".jpg"
				}
			}
		}

		if ext == ".jpg" && tag != "" && bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
			// A cover stored before names its old source: replaced.
			if jpegSrcTag(data[:min(len(data), 64)]) != "" {
				data = append([]byte{0xFF, 0xD8}, data[6+len(coverSrcPrefix)+12:]...)
			}

			data = jpegWithSrc(data, tag)
		}
	}

	path, err := storeCover(dir, base, ext, bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	for _, e := range coverExts {
		if other := filepath.Join(dir, base+e); other != path {
			os.Remove(other)
		}
	}

	os.Remove(filepath.Join(dir, base+"-t400.jpg"))

	if st, err := os.Stat(path); err == nil {
		coverSrcs.Store(path, coverSrc{mod: st.ModTime(), size: st.Size(), tag: jpegSrcTag(data[:min(len(data), 64)])})
	}

	return path, nil
}

// onWhite puts an image with transparency onto white (JPEG has none).
func onWhite(img image.Image) image.Image {
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		return img
	}

	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Over)

	return out
}

// keptCovers are large covers saveCover couldn't make smaller: not tried
// again.
var keptCovers sync.Map

// smallCovers are covers no wider than a thumbnail (by path, with the
// cover's time): served as they are.
var smallCovers sync.Map

// coverThumb returns the thumbnail of a cached cover, making it when
// missing or older than the cover; the cover itself when it is small
// already or can't be decoded.
func coverThumb(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return path
	}

	if mod, small := smallCovers.Load(path); small && mod.(time.Time).Equal(st.ModTime()) {
		return path
	}

	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)) + "-t400"

	fresh := func() (string, bool) {
		p, tst, ok := cachedFileStat(dir, base)
		return p, ok && !tst.ModTime().Before(st.ModTime())
	}

	if p, ok := fresh(); ok {
		return p
	}

	lock, _ := coverLocks.LoadOrStore(base, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	if p, ok := fresh(); ok {
		return p
	}

	f, err := os.Open(path)
	if err != nil {
		return path
	}

	src, _, err := image.Decode(f)
	f.Close()

	if err != nil || src.Bounds().Dx() <= coverThumbWidth*5/4 {
		smallCovers.Store(path, st.ModTime())
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
