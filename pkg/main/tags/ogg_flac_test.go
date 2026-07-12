package tags

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const (
	testOGGSample  = "testdata/sample.ogg"
	testFLACSample = "testdata/sample.flac"
)

// workingCopy copies a read-only testdata sample into a temp dir so write tests
// don't mutate the master, and returns the copy's path. Skips if absent.
func workingCopy(tb testing.TB, sample string) string {
	tb.Helper()

	src, err := os.Open(sample)
	if err != nil {
		tb.Skipf("sample %s not available (%v) - skipping", sample, err)
	}
	defer src.Close()

	dst := filepath.Join(tb.TempDir(), filepath.Base(sample))

	out, err := os.Create(dst)
	if err != nil {
		tb.Fatalf("create copy: %v", err)
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		tb.Fatalf("copy: %v", err)
	}

	if err := out.Close(); err != nil {
		tb.Fatalf("close copy: %v", err)
	}

	return dst
}

func TestOGGWriteReadRoundTrip(t *testing.T) {
	path := workingCopy(t, testOGGSample)
	h := NewOGGHandler()

	orig, err := h.ReadTags(path)
	if err != nil {
		t.Fatalf("initial ReadTags: %v", err)
	}

	t.Logf("original ogg tags: title=%q artist=%q album=%q ch=%d rate=%d",
		orig.Title, orig.Artist, orig.Album, orig.Channels, orig.SampleRate)

	want := &AudioTags{
		Title:       "New Title",
		Artist:      "New Artist",
		Album:       "New Album",
		AlbumArtist: "New AlbumArtist",
		Genre:       "Rock",
		Year:        2020,
		TrackNumber: 3,
		TotalTracks: 9,
		DiscNumber:  1,
		ISRC:        "DEABC0000001",
		MBReleaseID: "rel-xyz",
	}

	// Write twice to confirm the rewrite is repeatable and produces a valid file.
	for i := range 2 {
		if err := h.WriteTags(context.Background(), path, want); err != nil {
			t.Fatalf("WriteTags #%d: %v", i+1, err)
		}
	}

	got, err := h.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags after write: %v", err)
	}

	checks := []struct{ name, got, want string }{
		{"Title", got.Title, want.Title},
		{"Artist", got.Artist, want.Artist},
		{"Album", got.Album, want.Album},
		{"AlbumArtist", got.AlbumArtist, want.AlbumArtist},
		{"Genre", got.Genre, want.Genre},
		{"ISRC", got.ISRC, want.ISRC},
		{"MBReleaseID", got.MBReleaseID, want.MBReleaseID},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	if got.Year != want.Year || got.TrackNumber != want.TrackNumber {
		t.Errorf("year/track = %d/%d, want %d/%d",
			got.Year, got.TrackNumber, want.Year, want.TrackNumber)
	}

	// Audio pages must survive the streaming rewrite unchanged.
	if got.Channels != orig.Channels || got.SampleRate != orig.SampleRate {
		t.Errorf("audio props changed after write: %d ch / %d Hz, want %d ch / %d Hz",
			got.Channels, got.SampleRate, orig.Channels, orig.SampleRate)
	}
}

// TestOGGWriteTagsRAM confirms WriteTags streams instead of buffering the whole
// file: allocations stay far below the file size (the old code buffered ~2x).
func TestOGGWriteTagsRAM(t *testing.T) {
	path := workingCopy(t, testOGGSample)
	h := NewOGGHandler()
	want := &AudioTags{Title: "X", Artist: "Y", Album: "Z"}

	got := bytesPerOp(t, 20, func() {
		if err := h.WriteTags(context.Background(), path, want); err != nil {
			t.Fatalf("WriteTags: %v", err)
		}
	})

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	t.Logf("OGG WriteTags on %d-byte file: %d bytes/op", fi.Size(), got)

	if int64(got) > 1<<20 {
		t.Errorf("WriteTags allocated %d bytes/op; expected << file size %d (not streaming?)",
			got, fi.Size())
	}
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
