package tags

import (
	"bufio"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mewkiz/flac/meta"
)

type flacPic struct {
	typ  uint32
	data []byte
}

func flacPictures(t *testing.T, path string) []flacPic {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	if _, err := io.ReadFull(br, make([]byte, 4)); err != nil {
		t.Fatal(err)
	}

	var pics []flacPic

	for {
		block, err := meta.New(br)
		if err != nil {
			t.Fatal(err)
		}

		if block.Type == meta.TypePicture {
			if err := block.Parse(); err != nil {
				t.Fatal(err)
			}

			pic := block.Body.(*meta.Picture)
			pics = append(pics, flacPic{typ: pic.Type, data: pic.Data})
		} else {
			block.Skip() //nolint:errcheck
		}

		if block.IsLast {
			return pics
		}
	}
}

// tinyPNG returns a distinct 1x1 PNG per shade (metaflac needs a real image
// to read dimensions from).
func tinyPNG(t *testing.T, shade uint8) []byte {
	t.Helper()

	img := image.NewGray(image.Rect(0, 0, 1, 1))
	img.SetGray(0, 0, color.Gray{Y: shade})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}

	return p
}

// TestFLACCoverKeepsOtherPictures: a new cover replaces only the front cover,
// and passing the existing front cover back does not touch the pictures.
func TestFLACCoverKeepsOtherPictures(t *testing.T) {
	if _, err := exec.LookPath("metaflac"); err != nil {
		t.Skip("metaflac not installed")
	}

	src, err := os.ReadFile("testdata/sample.flac")
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := writeFile(t, dir, "x.flac", src)

	front := tinyPNG(t, 10)
	back := tinyPNG(t, 20)
	frontPath := writeFile(t, dir, "front.bin", front)
	backPath := writeFile(t, dir, "back.bin", back)

	if out, err := exec.Command("metaflac", "--remove", "--block-type=PICTURE", path).CombinedOutput(); err != nil {
		t.Fatalf("metaflac remove: %v %s", err, out)
	}

	out, err := exec.Command("metaflac",
		"--import-picture-from=3|image/png|Front||"+frontPath,
		"--import-picture-from=4|image/png|Back||"+backPath,
		path).CombinedOutput()
	if err != nil {
		t.Fatalf("metaflac import: %v %s", err, out)
	}

	h := NewFLACHandler()

	before, _ := os.ReadFile(path)
	if err := h.WriteTags(context.Background(), path, &AudioTags{CoverData: front, CoverMIME: "image/png"}); err != nil {
		t.Fatal(err)
	}

	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("re-passing the existing front cover rewrote the file")
	}

	newFront := tinyPNG(t, 30)
	if err := h.WriteTags(context.Background(), path,
		&AudioTags{Title: "T", CoverData: newFront, CoverMIME: "image/png"}); err != nil {
		t.Fatal(err)
	}

	pics := flacPictures(t, path)

	var gotFront, gotBack int

	for _, p := range pics {
		switch {
		case p.typ == 3 && bytes.Equal(p.data, newFront):
			gotFront++
		case p.typ == 4 && bytes.Equal(p.data, back):
			gotBack++
		default:
			t.Fatalf("unexpected picture type %d (%d bytes)", p.typ, len(p.data))
		}
	}

	if gotFront != 1 || gotBack != 1 {
		t.Fatalf("front=%d back=%d, want 1/1", gotFront, gotBack)
	}

	tags, err := h.ReadTags(path)
	if err != nil || tags.Title != "T" {
		t.Fatalf("title not written: %v %+v", err, tags)
	}
}

// bytesPerOp returns the average bytes allocated per call of fn over iters runs.
func bytesPerOp(tb testing.TB, iters int, fn func()) uint64 {
	tb.Helper()

	runtime.GC()

	var m0, m1 runtime.MemStats

	runtime.ReadMemStats(&m0)

	for range iters {
		fn()
	}

	runtime.ReadMemStats(&m1)

	return (m1.TotalAlloc - m0.TotalAlloc) / uint64(iters)
}

func TestFLACReadTags(t *testing.T) {
	if _, err := os.Stat(testFLACSample); err != nil {
		t.Skipf("sample %s not available (%v) - skipping", testFLACSample, err)
	}

	h := NewFLACHandler()

	tags, err := h.ReadTags(testFLACSample)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	t.Logf("flac tags: title=%q artist=%q album=%q ch=%d rate=%d depth=%d dur=%s",
		tags.Title, tags.Artist, tags.Album,
		tags.Channels, tags.SampleRate, tags.BitDepth, tags.Duration)

	// StreamInfo must always yield audio properties.
	if tags.SampleRate == 0 || tags.Channels == 0 {
		t.Errorf("expected non-zero sample rate/channels from StreamInfo, got %d/%d",
			tags.SampleRate, tags.Channels)
	}

	// ReadTags must not carry cover art (that's ReadTagsWithCover's job).
	if len(tags.CoverData) != 0 {
		t.Errorf("ReadTags returned %d bytes of cover data, want 0", len(tags.CoverData))
	}
}

// TestFLACReadTagsRAM shows the PICTURE-skip optimization: metadata-only ReadTags
// should allocate far less than ReadTagsWithCover when the file embeds cover art.
func TestFLACReadTagsRAM(t *testing.T) {
	if _, err := os.Stat(testFLACSample); err != nil {
		t.Skipf("sample %s not available (%v) - skipping", testFLACSample, err)
	}

	h := NewFLACHandler()

	metaBytes := bytesPerOp(t, 50, func() {
		if _, err := h.ReadTags(testFLACSample); err != nil {
			t.Fatalf("ReadTags: %v", err)
		}
	})

	coverBytes := bytesPerOp(t, 50, func() {
		if _, err := h.ReadTagsWithCover(testFLACSample); err != nil {
			t.Fatalf("ReadTagsWithCover: %v", err)
		}
	})

	t.Logf("FLAC ReadTags (no cover): %d bytes/op", metaBytes)
	t.Logf("FLAC ReadTagsWithCover:  %d bytes/op", coverBytes)
}

func TestFLACBuildTagArgsPreservesUnset(t *testing.T) {
	args := (&FLACHandler{}).buildTagArgs(&AudioTags{Title: "T", TrackNumber: 2, TotalTracks: 10})
	joined := strings.Join(args, " ")

	for _, want := range []string{
		"--remove-tag=TITLE", "--remove-tag=TRACKNUMBER", "--remove-tag=TRACK",
		"--remove-tag=TRACKTOTAL", "--set-tag=TITLE=T", "--set-tag=TRACKNUMBER=02",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %v", want, args)
		}
	}

	for _, unwanted := range []string{"--remove-all-tags", "ARTIST", "REPLAYGAIN", "COMMENT"} {
		if strings.Contains(joined, unwanted) {
			t.Errorf("unexpected %q in %v", unwanted, args)
		}
	}

	// Removes must precede sets (metaflac applies shorthand ops in order).
	lastRemove, firstSet := -1, len(args)

	for i, a := range args {
		if strings.HasPrefix(a, "--remove-tag=") {
			lastRemove = i
		} else if firstSet == len(args) {
			firstSet = i
		}
	}

	if lastRemove > firstSet {
		t.Error("remove after set")
	}
}
