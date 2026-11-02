package tags

import (
	"bytes"
	"context"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

// testMP3Sample is a real MP3 copied once into the package testdata dir.
// Tests skip automatically when it is not present (e.g. CI or other machines).
const testMP3Sample = "testdata/sample.mp3"

// workingCopyMP3 copies the read-only testdata sample into a temp dir so write
// tests don't mutate the master, and returns the working copy's path. Skips the
// test if the sample is absent.
func workingCopyMP3(tb testing.TB) string {
	tb.Helper()

	src, err := os.Open(testMP3Sample)
	if err != nil {
		tb.Skipf("testdata sample not available (%v) - skipping", err)
	}
	defer src.Close()

	dst := filepath.Join(tb.TempDir(), "test.mp3")

	out, err := os.Create(dst)
	if err != nil {
		tb.Fatalf("create temp copy: %v", err)
	}

	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		tb.Fatalf("copy mp3: %v", err)
	}

	if err := out.Close(); err != nil {
		tb.Fatalf("close temp copy: %v", err)
	}

	return dst
}

// requireSample skips the test if the read-only testdata sample is missing.
func requireSample(tb testing.TB) string {
	tb.Helper()

	if _, err := os.Stat(testMP3Sample); err != nil {
		tb.Skipf("testdata sample not available (%v) - skipping", err)
	}

	return testMP3Sample
}

// readWithFrames reads tags using an explicit ID3v2 frame set, so the RAM test
// can compare the minimal read set against the full write set.
func readWithFrames(path string, frames []string) (*AudioTags, error) {
	tag, err := id3v2.Open(path, id3v2.Options{Parse: true, ParseFrames: frames})
	if err != nil {
		return nil, err
	}
	defer tag.Close()

	return (&MP3Handler{}).extractTags(tag)
}

func TestMP3ReadWriteRoundTrip(t *testing.T) {
	path := workingCopyMP3(t)
	h := NewMP3Handler()

	orig, err := h.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	t.Logf("original tags: title=%q artist=%q album=%q year=%d track=%d/%d duration=%s",
		orig.Title, orig.Artist, orig.Album, orig.Year,
		orig.TrackNumber, orig.TotalTracks, orig.Duration)

	want := &AudioTags{
		Title:         "Round Trip Title",
		Artist:        "Round Trip Artist",
		Album:         "Round Trip Album",
		AlbumArtist:   "Round Trip AlbumArtist",
		Genre:         "Electronic",
		Year:          1994,
		TrackNumber:   1,
		TotalTracks:   10,
		DiscNumber:    1,
		TotalDiscs:    1,
		ISRC:          "GBABC1234567",
		Label:         "Stealth Sonic",
		CatalogNum:    "SSX001",
		MBReleaseID:   "11111111-2222-3333-4444-555555555555",
		MBRecordingID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		MBArtistID:    "99999999-8888-7777-6666-555555555555",
	}

	if err := h.WriteTags(context.Background(), path, want); err != nil {
		t.Fatalf("WriteTags: %v", err)
	}

	got, err := h.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags after write: %v", err)
	}

	strChecks := []struct {
		name, got, want string
	}{
		{"Title", got.Title, want.Title},
		{"Artist", got.Artist, want.Artist},
		{"Album", got.Album, want.Album},
		{"AlbumArtist", got.AlbumArtist, want.AlbumArtist},
		{"Genre", got.Genre, want.Genre},
		{"ISRC", got.ISRC, want.ISRC},
		{"Label", got.Label, want.Label},
		{"CatalogNum", got.CatalogNum, want.CatalogNum},
		{"MBReleaseID", got.MBReleaseID, want.MBReleaseID},
		{"MBRecordingID", got.MBRecordingID, want.MBRecordingID},
		{"MBArtistID", got.MBArtistID, want.MBArtistID},
	}
	for _, c := range strChecks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	intChecks := []struct {
		name      string
		got, want int
	}{
		{"Year", got.Year, want.Year},
		{"TrackNumber", got.TrackNumber, want.TrackNumber},
		{"TotalTracks", got.TotalTracks, want.TotalTracks},
		{"DiscNumber", got.DiscNumber, want.DiscNumber},
		{"TotalDiscs", got.TotalDiscs, want.TotalDiscs},
	}
	for _, c := range intChecks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

// TestMP3WriteIdempotent guards against multi-value frame (TXXX/COMM/USLT/UFID)
// duplication on repeated tagging — the failure mode that bloats files and RAM
// during batch folder processing.
func TestMP3WriteIdempotent(t *testing.T) {
	path := workingCopyMP3(t)
	h := NewMP3Handler()

	tg := &AudioTags{
		Title:         "T",
		Artist:        "A",
		Album:         "Al",
		Comment:       "hello",
		MBReleaseID:   "rel-1",
		MBRecordingID: "rec-1",
		MBArtistID:    "art-1",
		AcoustID:      "ac-1",
		CatalogNum:    "cat-1",
	}

	// Write the same tags three times; counts must not grow.
	txxxAfterFirst := 0

	for i := range 3 {
		if err := h.WriteTags(context.Background(), path, tg); err != nil {
			t.Fatalf("WriteTags #%d: %v", i+1, err)
		}

		if i == 0 {
			first, err := id3v2.Open(path, id3v2.Options{Parse: true})
			if err != nil {
				t.Fatalf("open: %v", err)
			}

			txxxAfterFirst = len(first.GetFrames("TXXX"))
			first.Close()
		}
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer tag.Close()

	comm := tag.GetFrames("COMM")
	if len(comm) != 1 {
		t.Errorf("COMM frames = %d, want 1 (duplicated on re-tag)", len(comm))
	} else if cf, ok := comm[0].(id3v2.CommentFrame); !ok || cf.Text != "hello" {
		t.Errorf("COMM text = %q, want %q", cf.Text, "hello")
	}

	// 5 non-empty TXXX written: MBReleaseID, MBArtistID, MBRecordingID, AcoustID,
	// CatalogNum. Unrelated TXXX already in the sample (album-artist /
	// release-group IDs) are preserved, so the count is >= 5 and must not grow.
	if n := len(tag.GetFrames("TXXX")); n < 5 || n != txxxAfterFirst {
		t.Errorf("TXXX frames = %d, want >= 5 and == %d (duplicated on re-tag)", n, txxxAfterFirst)
	}

	// TXXX-backed values are read back (COMM is intentionally not on the read path).
	got, err := h.ReadTags(path)
	if err != nil {
		t.Fatalf("ReadTags: %v", err)
	}

	if got.MBReleaseID != "rel-1" || got.MBRecordingID != "rec-1" {
		t.Errorf("wrong values after rewrite: MBReleaseID=%q MBRecordingID=%q",
			got.MBReleaseID, got.MBRecordingID)
	}
}

// TestMP3ReadTagsRAM reports allocations for ReadTags and compares the minimal
// read frame set against the full write frame set to quantify the RAM saved by
// skipping unused frames (lyrics/comment/composer/conductor/copyright).
func TestMP3ReadTagsRAM(t *testing.T) {
	path := requireSample(t)
	h := NewMP3Handler()

	readAllocs := testing.AllocsPerRun(50, func() {
		if _, err := h.ReadTags(path); err != nil {
			t.Fatalf("ReadTags: %v", err)
		}
	})

	readBytes := bytesPerOp(t, 200, func() {
		_, _ = h.ReadTags(path)
	})

	fullAllocs := testing.AllocsPerRun(50, func() {
		if _, err := readWithFrames(path, nil); err != nil {
			t.Fatalf("readWithFrames(full): %v", err)
		}
	})

	fullBytes := bytesPerOp(t, 200, func() {
		_, _ = readWithFrames(path, nil)
	})

	t.Logf("ReadTags minimal frames: %.0f allocs/op, %d bytes/op", readAllocs, readBytes)
	t.Logf("ReadTags full frame set: %.0f allocs/op, %d bytes/op", fullAllocs, fullBytes)
	t.Logf("saved by trimming frames: %.0f allocs/op, %d bytes/op",
		fullAllocs-readAllocs, int64(fullBytes)-int64(readBytes))
}

func BenchmarkMP3ReadTags(b *testing.B) {
	path := requireSample(b)
	h := NewMP3Handler()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := h.ReadTags(path); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMP3WriteTags(b *testing.B) {
	path := workingCopyMP3(b)
	h := NewMP3Handler()

	tg := &AudioTags{
		Title: "Bench", Artist: "Bench", Album: "Bench",
		Year: 1994, TrackNumber: 1, TotalTracks: 10,
	}

	b.ReportAllocs()

	for b.Loop() {
		if err := h.WriteTags(context.Background(), path, tg); err != nil {
			b.Fatal(err)
		}
	}
}

// mpegFrame returns one MPEG-1 Layer III, 128 kbps, 44.1 kHz, stereo frame
// (417 bytes) with a zero payload.
func mpegFrame() []byte {
	f := make([]byte, 417)
	copy(f, []byte{0xFF, 0xFB, 0x90, 0x00})

	return f
}

// TestMP3WritePreservesUnmodelledFrames covers A6/A7/A8: frames outside the
// modelled set, unrelated TXXX, the year and the cover must survive a re-tag
// that doesn't set them.
func TestMP3WritePreservesUnmodelledFrames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.mp3")
	if err := os.WriteFile(path, bytes.Repeat(mpegFrame(), 4), 0o644); err != nil {
		t.Fatal(err)
	}

	tag, err := id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatal(err)
	}

	tag.SetVersion(3)
	tag.AddFrame("TYER", id3v2.TextFrame{Encoding: id3v2.EncodingISO, Text: "1987"})
	tag.AddFrame("POPM", id3v2.PopularimeterFrame{Email: "a@b", Rating: 200, Counter: big.NewInt(5)})
	tag.AddFrame("PRIV", id3v2.UnknownFrame{Body: []byte("owner\x00secret")})
	tag.AddFrame("TXXX", id3v2.UserDefinedTextFrame{Encoding: id3v2.EncodingUTF8, Description: "MyKey", Value: "keep"})
	tag.AddFrame("TXXX", id3v2.UserDefinedTextFrame{
		Encoding: id3v2.EncodingUTF8, Description: "REPLAYGAIN_TRACK_GAIN", Value: "-4.20 dB",
	})
	tag.AddFrame("TXXX", id3v2.UserDefinedTextFrame{
		Encoding: id3v2.EncodingUTF8, Description: "MUSICBRAINZ_ALBUMID", Value: "old-rel",
	})
	tag.AddFrame("UFID", id3v2.UFIDFrame{OwnerIdentifier: "http://example.org", Identifier: []byte("x1")})
	tag.AddFrame("APIC", id3v2.PictureFrame{
		Encoding: id3v2.EncodingUTF8, MimeType: "image/jpeg", PictureType: id3v2.PTBackCover,
		Description: "back", Picture: []byte{0xFF, 0xD8, 0xFF, 1},
	})

	if err := tag.Save(); err != nil {
		t.Fatal(err)
	}

	tag.Close()

	h := NewMP3Handler()

	if err := h.WriteTags(context.Background(), path, &AudioTags{Title: "New", MBReleaseID: "new-rel"}); err != nil {
		t.Fatal(err)
	}

	got, err := h.ReadTags(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "New" || got.Year != 1987 || got.ReplayGainTrack != -4.2 || got.MBReleaseID != "new-rel" {
		t.Fatalf("title=%q year=%d rg=%v rel=%q", got.Title, got.Year, got.ReplayGainTrack, got.MBReleaseID)
	}

	tag, err = id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"POPM", "PRIV", "UFID", "APIC", "TYER"} {
		if len(tag.GetFrames(id)) == 0 {
			t.Errorf("%s frame lost on write", id)
		}
	}

	descs := map[string]int{}

	for _, f := range tag.GetFrames("TXXX") {
		if u, ok := f.(id3v2.UserDefinedTextFrame); ok {
			descs[u.Description]++
		}
	}

	if descs["MyKey"] != 1 || descs["MUSICBRAINZ_ALBUMID"] != 0 || descs["MusicBrainz Album Id"] != 1 {
		t.Errorf("TXXX after write: %v", descs)
	}

	tag.Close()

	// Writing a year replaces TYER with TDRC.
	if err := h.WriteTags(context.Background(), path, &AudioTags{Year: 2001}); err != nil {
		t.Fatal(err)
	}

	tag, err = id3v2.Open(path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tag.Close()

	if len(tag.GetFrames("TYER")) != 0 || tag.GetTextFrame("TDRC").Text != "2001" {
		t.Errorf("year frames: TYER=%d TDRC=%q", len(tag.GetFrames("TYER")), tag.GetTextFrame("TDRC").Text)
	}

	if got, _ := h.ReadTags(path); got.MBRecordingID != "" {
		t.Errorf("unexpected recording id %q", got.MBRecordingID)
	}
}
