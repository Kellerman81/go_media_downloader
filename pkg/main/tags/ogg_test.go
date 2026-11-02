package tags

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOGGNativeDuration(t *testing.T) {
	if _, err := os.Stat(testOGGSample); err != nil {
		t.Skipf("sample %s not available - skipping", testOGGSample)
	}

	d, err := (&OGGHandler{}).GetDuration(testOGGSample)
	if err != nil {
		t.Fatalf("GetDuration: %v", err)
	}

	t.Logf("OGG native duration: %s", d)

	if d <= 0 {
		t.Errorf("expected positive duration, got %s", d)
	}
}

func TestOGGWriteTagsFFmpegVerified(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	cover := genCover(t, dir)

	for _, c := range []struct{ name, codec string }{
		{"vorbis.ogg", "libvorbis"},
		{"opus.opus", "libopus"},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(dir, c.name)
			genFixture(t, "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:a", c.codec,
				"-metadata", "TITLE=Old", "-metadata", "CUSTOMKEY=keepme", path)

			h := NewOGGHandler()
			want := fullTags(cover)

			if err := h.WriteTags(context.Background(), path, want); err != nil {
				t.Fatal(err)
			}

			assertFFmpegClean(t, path)

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			verifyOggPages(t, data)

			got, err := h.ReadTags(path)
			if err != nil {
				t.Fatal(err)
			}

			assertTagsEqual(t, got, want)

			hd, err := h.readOggHeaders(bufio.NewReader(bytes.NewReader(data)), true)
			if err != nil {
				t.Fatal(err)
			}

			off, _ := commentHeaderPayload(hd.packets[1])
			if _, comments, _, _ := parseVorbisCommentList(hd.packets[1][off:]); !strings.Contains(
				strings.Join(comments, "\n"), "CUSTOMKEY=keepme") {
				t.Error("unmodelled comment CUSTOMKEY lost")
			}

			// Re-tag without a cover: the multi-page cover must survive.
			if err := h.WriteTags(context.Background(), path, &AudioTags{Title: "Again"}); err != nil {
				t.Fatal(err)
			}

			assertFFmpegClean(t, path)

			got, err = h.ReadTags(path)
			if err != nil {
				t.Fatal(err)
			}

			want.Title = "Again"
			assertTagsEqual(t, got, want)
		})
	}
}

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

// clickOggFirstPage is the BOS page (Vorbis identification header) of a
// libvorbis-encoded file, as written by ffmpeg — a known-good page whose CRC
// field was produced by libogg.
const clickOggFirstPage = "4f6767530002000000000000000011106f5300000000c7e79f6b011e01766f726269730000" +
	"00000244ac00000000000080b5010000000000b801"

// bitwiseOggCRC is an independent, table-free reference implementation.
func bitwiseOggCRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc ^= uint32(b) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = crc<<1 ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
	}

	return crc
}

func TestOggCRCKnownPage(t *testing.T) {
	page, err := hex.DecodeString(clickOggFirstPage)
	if err != nil {
		t.Fatal(err)
	}

	want := binary.LittleEndian.Uint32(page[22:26])

	binary.LittleEndian.PutUint32(page[22:26], 0)

	if got := oggCRC(page); got != want {
		t.Fatalf("oggCRC = %08x, want %08x (libogg)", got, want)
	}

	if got := bitwiseOggCRC(page); got != want {
		t.Fatalf("reference CRC = %08x, want %08x", got, want)
	}

	data := bytes.Repeat([]byte("The quick brown fox \x00\xff"), 500)
	if oggCRC(data) != bitwiseOggCRC(data) {
		t.Fatal("table CRC disagrees with bitwise reference")
	}
}

// verifyOggPages walks every page of an Ogg file and checks capture pattern,
// CRC and per-stream contiguous sequence numbers.
func verifyOggPages(t *testing.T, data []byte) int {
	t.Helper()

	seqs := map[uint32]uint32{}
	pages := 0

	for off := 0; off < len(data); {
		if off+27 > len(data) || string(data[off:off+4]) != "OggS" {
			t.Fatalf("no page at offset %d", off)
		}

		nseg := int(data[off+26])
		size := 27 + nseg

		for _, l := range data[off+27 : off+27+nseg] {
			size += int(l)
		}

		page := append([]byte(nil), data[off:off+size]...)
		want := binary.LittleEndian.Uint32(page[22:26])

		binary.LittleEndian.PutUint32(page[22:26], 0)

		if got := bitwiseOggCRC(page); got != want {
			t.Fatalf("page %d: CRC %08x, stored %08x", pages, got, want)
		}

		serial := binary.LittleEndian.Uint32(page[14:18])
		seq := binary.LittleEndian.Uint32(page[18:22])

		if prev, ok := seqs[serial]; ok && seq != prev+1 {
			t.Fatalf("page %d: sequence %d after %d", pages, seq, prev)
		}

		seqs[serial] = seq
		off += size
		pages++
	}

	return pages
}

// synthOpus builds a minimal structurally valid Ogg Opus stream: BOS page with
// OpusHead, an OpusTags packet of the given comments (paginated), and audio
// pages.
func synthOpus(t *testing.T, comments []string, audioPages int) []byte {
	t.Helper()

	head := make([]byte, 19)
	copy(head, "OpusHead")
	head[8] = 1
	head[9] = 2
	binary.LittleEndian.PutUint32(head[12:16], 44100)

	var tagsPkt bytes.Buffer

	tagsPkt.WriteString("OpusTags")

	var n [4]byte

	binary.LittleEndian.PutUint32(n[:], 6)
	tagsPkt.Write(n[:])
	tagsPkt.WriteString("vendor")
	binary.LittleEndian.PutUint32(n[:], uint32(len(comments)))
	tagsPkt.Write(n[:])

	for _, c := range comments {
		binary.LittleEndian.PutUint32(n[:], uint32(len(c)))
		tagsPkt.Write(n[:])
		tagsPkt.WriteString(c)
	}

	const serial = 0x1234

	var out bytes.Buffer

	bos := buildOggPages([][]byte{head}, 0, serial, 0, false)[0]
	bos[5] = 0x02
	binary.LittleEndian.PutUint32(bos[22:26], 0)
	binary.LittleEndian.PutUint32(bos[22:26], oggCRC(bos))
	out.Write(bos)

	hdrPages := buildOggPages([][]byte{tagsPkt.Bytes()}, 0, serial, 1, false)
	for _, p := range hdrPages {
		out.Write(p)
	}

	seq := uint32(1 + len(hdrPages))

	for i := range audioPages {
		p := buildOggPages([][]byte{bytes.Repeat([]byte{byte(i)}, 300)}, 0, serial, seq, i == audioPages-1)[0]
		binary.LittleEndian.PutUint64(p[6:14], uint64(960*(i+1)))
		binary.LittleEndian.PutUint32(p[22:26], 0)
		binary.LittleEndian.PutUint32(p[22:26], oggCRC(p))
		out.Write(p)

		seq++
	}

	return out.Bytes()
}

func TestBuildOggPagesMultiPage(t *testing.T) {
	big := bytes.Repeat([]byte{0xAB}, 70000) // > 255*255: needs two pages
	small := []byte("setup")

	pages := buildOggPages([][]byte{big, small}, 0, 7, 1, false)
	if len(pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(pages))
	}

	// Page 1: 255 full segments, packet continues -> granule -1, no continuation flag.
	if pages[0][26] != 255 || pages[0][5] != 0 ||
		int64(binary.LittleEndian.Uint64(pages[0][6:14])) != -1 {
		t.Fatalf("page 1 header: segs=%d flags=%d", pages[0][26], pages[0][5])
	}

	// Page 2 continues the packet, ends both packets -> granule 0.
	if pages[1][5]&0x01 == 0 || binary.LittleEndian.Uint64(pages[1][6:14]) != 0 {
		t.Fatalf("page 2 header: flags=%d", pages[1][5])
	}

	if binary.LittleEndian.Uint32(pages[1][18:22]) != 2 {
		t.Fatal("sequence numbers not incremented")
	}

	var stream []byte
	for _, p := range pages {
		stream = append(stream, p...)
	}

	verifyOggPages(t, stream)

	// A packet that is an exact multiple of 255 needs a terminating 0 lacing value.
	p := buildOggPages([][]byte{bytes.Repeat([]byte{1}, 510)}, 0, 7, 0, false)
	if len(p) != 1 || p[0][26] != 3 || p[0][27+2] != 0 {
		t.Fatalf("exact-multiple packet lacing wrong")
	}
}

func TestOGGWriteTagsMultiPageCover(t *testing.T) {
	cover := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF, 0xE0, 1, 2, 3, 4}, 30000) // 240 KB
	picBlock := (&OGGHandler{}).buildPictureBlock(&AudioTags{CoverData: cover, CoverMIME: "image/jpeg"})
	orig := synthOpus(t, []string{
		"TITLE=Old",
		"ARTISTS=Keep A; Keep B",
		"CUSTOMKEY=keepme",
		"TRACKNUMBER=3/12",
		"METADATA_BLOCK_PICTURE=" + base64.StdEncoding.EncodeToString(picBlock),
	}, 5)

	path := filepath.Join(t.TempDir(), "t.opus")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatal(err)
	}

	h := NewOGGHandler()

	got, err := h.ReadTags(path)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got.CoverData, cover) || got.TrackNumber != 3 || got.TotalTracks != 12 {
		t.Fatalf("multi-page read: cover=%d track=%d/%d", len(got.CoverData), got.TrackNumber, got.TotalTracks)
	}

	// Write without a cover: existing cover and unknown keys must survive.
	if err := h.WriteTags(context.Background(), path, &AudioTags{Title: "New", Album: "Alb"}); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	pagesBefore := verifyOggPages(t, data)

	got, err = h.ReadTags(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "New" || got.Album != "Alb" || !bytes.Equal(got.CoverData, cover) {
		t.Fatalf("after write: title=%q album=%q cover=%d", got.Title, got.Album, len(got.CoverData))
	}

	hd, err := h.readOggHeaders(bufio.NewReader(bytes.NewReader(data)), true)
	if err != nil {
		t.Fatal(err)
	}

	_, comments, _, _ := parseVorbisCommentList(hd.packets[1][8:])
	joined := strings.Join(comments, "\n")

	for _, want := range []string{"ARTISTS=Keep A; Keep B", "CUSTOMKEY=keepme", "TITLE=New"} {
		if !strings.Contains(joined, want) {
			t.Errorf("comment %q missing after write", want)
		}
	}

	if strings.Contains(joined, "TITLE=Old") {
		t.Error("old TITLE not replaced")
	}

	// Replace the cover with a tiny one: header shrinks, audio pages get renumbered.
	if err := h.WriteTags(context.Background(), path, &AudioTags{CoverData: []byte{0xFF, 0xD8, 0xFF, 0xE0}}); err != nil {
		t.Fatal(err)
	}

	data, _ = os.ReadFile(path)
	if pagesAfter := verifyOggPages(t, data); pagesAfter >= pagesBefore {
		t.Fatalf("page count %d did not shrink from %d", pagesAfter, pagesBefore)
	}

	got, err = h.ReadTags(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.CoverData) != 4 || got.Title != "New" {
		t.Fatalf("after shrink: cover=%d title=%q", len(got.CoverData), got.Title)
	}

	// Audio payload must be byte-identical.
	if !bytes.HasSuffix(data, orig[len(orig)-27-1-300:][27+1:]) {
		t.Error("last audio packet changed")
	}
}

func TestParsePictureBlockMalformed(t *testing.T) {
	h := &OGGHandler{}

	crafted := func(mimeLen, descLen, picLen uint32) string {
		b := make([]byte, 64)
		binary.BigEndian.PutUint32(b[0:4], 3)
		binary.BigEndian.PutUint32(b[4:8], mimeLen)
		binary.BigEndian.PutUint32(b[8:12], descLen)
		binary.BigEndian.PutUint32(b[28:32], picLen)

		return base64.StdEncoding.EncodeToString(b)
	}

	cases := []string{
		crafted(0xFFFFFFFF, 0, 0), // 8+mimeLen+4 wraps in uint32
		crafted(0xFFFFFFF8, 0, 0), // wraps to a small value
		crafted(0, 0xFFFFFFF0, 0), // description length wraps
		crafted(0, 0, 0xFFFFFFFF), // picture length past end
		crafted(40, 0, 0),         // mime runs past end
		"!!!not base64!!!",
		base64.StdEncoding.EncodeToString(make([]byte, 31)),
	}

	for i, c := range cases {
		var tags AudioTags

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d panicked: %v", i, r)
				}
			}()

			h.parsePictureBlock(c, &tags)
		}()

		if tags.CoverData != nil {
			t.Errorf("case %d: expected no cover, got %d bytes", i, len(tags.CoverData))
		}
	}

	// Fuzz-style: random truncations/garbles of a valid block must never panic.
	valid := h.buildPictureBlock(&AudioTags{CoverData: []byte("img"), CoverMIME: "image/png"})
	for cut := range len(valid) + 1 {
		for _, flip := range []byte{0x00, 0xFF} {
			b := append([]byte(nil), valid[:cut]...)
			if cut > 6 {
				b[6] = flip
			}

			var tags AudioTags

			h.parsePictureBlock(base64.StdEncoding.EncodeToString(b), &tags)
		}
	}

	var ok AudioTags

	h.parsePictureBlock(base64.StdEncoding.EncodeToString(valid), &ok)

	if string(ok.CoverData) != "img" || ok.CoverMIME != "image/png" {
		t.Fatalf("valid block: %q %q", ok.CoverData, ok.CoverMIME)
	}
}

func TestCommentHeaderPayload(t *testing.T) {
	if off, ok := commentHeaderPayload([]byte("OpusTags....")); !ok || off != 8 {
		t.Errorf("Opus: off=%d ok=%v", off, ok)
	}

	if off, ok := commentHeaderPayload(append([]byte{0x03}, []byte("vorbisXYZ")...)); !ok ||
		off != 7 {
		t.Errorf("Vorbis: off=%d ok=%v", off, ok)
	}

	if _, ok := commentHeaderPayload([]byte("not a header")); ok {
		t.Error("expected non-header to return false")
	}
}

func TestParseIDHeaderOpus(t *testing.T) {
	data := make([]byte, 19)
	copy(data, "OpusHead")
	data[9] = 2 // channels
	binary.LittleEndian.PutUint32(data[12:16], 48000)

	var tags AudioTags

	parseIDHeader(data, &tags)

	if tags.Channels != 2 || tags.SampleRate != 48000 {
		t.Errorf("channels=%d sampleRate=%d", tags.Channels, tags.SampleRate)
	}
}
