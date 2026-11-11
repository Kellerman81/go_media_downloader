package tags

import (
	"bytes"
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/bogem/id3v2/v2"
)

func TestManagerSupportedFormats(t *testing.T) {
	m := NewManager()

	formats := m.SupportedFormats()
	for _, ext := range []string{".mp3", ".flac", ".m4a", ".m4b", ".ogg"} {
		if !slices.Contains(formats, ext) {
			t.Errorf("SupportedFormats is missing %s: %v", ext, formats)
		}
	}

	for path, want := range map[string]bool{"/a/01.MP3": true, "b.flac": true, "c.m4b": true, "cover.jpg": false, "noext": false} {
		if got := m.IsSupported(path); got != want {
			t.Errorf("IsSupported(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestMP3ReadTagsIgnoresImplausibleTLEN(t *testing.T) {
	for _, tc := range []struct {
		tlen string
		want time.Duration
	}{
		{"245", 0}, // seconds written by a broken tagger
		{"245000", 245 * time.Second},
	} {
		p := writeTemp(t, synthMP3(10))

		tag, err := id3v2.Open(p, id3v2.Options{Parse: true})
		if err != nil {
			t.Fatal(err)
		}

		tag.SetVersion(4)
		tag.AddFrame("TLEN", id3v2.TextFrame{Encoding: id3v2.EncodingUTF8, Text: tc.tlen})

		if err := tag.Save(); err != nil {
			t.Fatal(err)
		}

		tag.Close()

		got, err := (&MP3Handler{}).ReadTags(p)
		if err != nil {
			t.Fatal(err)
		}

		if got.Duration != tc.want {
			t.Errorf("TLEN %q: Duration = %v; want %v", tc.tlen, got.Duration, tc.want)
		}
	}
}

type panicHandler struct{}

func (panicHandler) ReadTags(string) (*AudioTags, error) { panic("boom") }

func (panicHandler) WriteTags(context.Context, string, *AudioTags) error { return nil }

func (panicHandler) SupportedFormats() []string { return []string{".boom"} }

func TestManagerReadTagsRecoversPanic(t *testing.T) {
	m := &Manager{handlers: map[string]TagHandler{}}
	m.RegisterHandler(panicHandler{})

	tags, err := m.ReadTags("/x/file.boom")
	if err == nil || tags != nil {
		t.Fatalf("expected error from recovered panic, got %v, %v", tags, err)
	}
}

// TestWriteTagsKeepsCover: a write without CoverData keeps the embedded
// cover in every format, so callers need not read it first and pass it back.
func TestWriteTagsKeepsCover(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	cover := genCover(t, dir)

	fixtures := map[string][]string{
		".mp3":  {"-c:a", "libmp3lame"},
		".flac": {"-c:a", "flac"},
		".ogg":  {"-c:a", "libvorbis"},
		".m4a":  {"-c:a", "aac"},
		".m4b":  {"-c:a", "aac", "-f", "mp4"},
	}

	for ext, codec := range fixtures {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(dir, "a"+ext)
			genFixture(t, append(append([]string{"-f", "lavfi", "-i", "sine=frequency=440:duration=2"}, codec...), path)...)

			ctx := context.Background()
			if err := WriteTags(ctx, path, &AudioTags{Title: "First", CoverData: cover, CoverMIME: "image/jpeg"}); err != nil {
				t.Fatal(err)
			}

			if got, _ := ReadCoverData(path); !bytes.Equal(got, cover) {
				t.Fatalf("cover not written (%d bytes)", len(got))
			}

			if err := WriteTags(ctx, path, &AudioTags{Title: "Second", Artist: "Someone"}); err != nil {
				t.Fatal(err)
			}

			if got, _ := ReadCoverData(path); !bytes.Equal(got, cover) {
				t.Errorf("cover lost by a write without CoverData (%d bytes left)", len(got))
			}

			got, err := ReadTags(path)
			if err != nil || got.Title != "Second" || got.Artist != "Someone" {
				t.Errorf("second write: %+v %v", got, err)
			}
		})
	}
}
