package tags

import (
	"context"
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
