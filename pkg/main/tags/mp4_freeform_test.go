package tags

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mp4DataAtom builds an ilst entry with one data child of the given type
// indicator (1 = UTF-8, 21 = signed int).
func mp4DataAtom(typ string, dataType byte, value []byte) []byte {
	return buildMP4Box(typ, buildMP4Box("data", []byte{0, 0, 0, dataType, 0, 0, 0, 0}, value))
}

// TestMP4WriteTagsKeepsStreamsAndUnknownAtoms: replacing the cover keeps the
// subtitle stream and the chapters, and ilst atoms ffmpeg doesn't know
// survive the remux byte for byte.
func TestMP4WriteTagsKeepsStreamsAndUnknownAtoms(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	oldCover := genCover(t, dir)
	path := filepath.Join(dir, "a.m4b")

	meta := filepath.Join(dir, "meta.txt")
	if err := os.WriteFile(meta, []byte(";FFMETADATA1\ntitle=Old\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=1500\ntitle=Ch1\n"+
		"[CHAPTER]\nTIMEBASE=1/1000\nSTART=1500\nEND=3000\ntitle=Ch2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	srt := filepath.Join(dir, "s.srt")
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	coverPath := filepath.Join(dir, "old.jpg")
	if err := os.WriteFile(coverPath, oldCover, 0o644); err != nil {
		t.Fatal(err)
	}

	genFixture(t, "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-i", srt, "-i", meta,
		"-i", coverPath, "-map", "0:a", "-map", "1", "-map", "3",
		"-map_metadata", "2", "-map_chapters", "2",
		"-c:a", "aac", "-c:s", "mov_text", "-c:v", "copy", "-disposition:v:0", "attached_pic",
		"-f", "ipod", path)

	unknown := [][]byte{
		mp4DataAtom("rtng", 21, []byte{1}),
		mp4DataAtom("purd", 1, []byte("2024-01-02 03:04:05")),
		mp4DataAtom("tvsh", 1, []byte("Some Show")),
	}

	if err := writeMP4Freeform(path, unknown); err != nil {
		t.Fatalf("injecting atoms: %v", err)
	}

	newCover := genCover(t, t.TempDir())
	if bytes.Equal(newCover, oldCover) {
		// genCover is deterministic; make the new cover distinguishable.
		newCover = append(append([]byte(nil), newCover...), 0)
	}

	h := NewMP4Handler()
	if err := h.WriteTags(context.Background(), path,
		&AudioTags{Title: "New", CoverData: newCover}); err != nil {
		t.Fatal(err)
	}

	streams, err := runTool(t, "ffprobe", "-v", "error", "-show_entries",
		"stream=codec_type:stream_disposition=attached_pic", "-of", "csv=p=0", path)
	if err != nil {
		t.Fatalf("ffprobe: %v %s", err, streams)
	}

	lines := strings.Fields(streams)
	if !strings.Contains(streams, "subtitle") || !strings.Contains(streams, "audio") {
		t.Fatalf("streams lost: %q", streams)
	}

	var pics int

	for _, l := range lines {
		if strings.HasPrefix(l, "video") {
			pics++
		}
	}

	if pics != 1 {
		t.Fatalf("want exactly one (new) cover stream, got %q", streams)
	}

	chapters, err := runTool(t, "ffprobe", "-v", "error", "-show_entries", "chapter_tags=title",
		"-of", "csv=p=0", path)
	if err != nil || !strings.Contains(chapters, "Ch1") || !strings.Contains(chapters, "Ch2") {
		t.Fatalf("chapters lost: %v %q", err, chapters)
	}

	got, err := h.ReadTagsWithCover(path)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "New" {
		t.Errorf("title = %q", got.Title)
	}

	if !bytes.Equal(got.CoverData, newCover) {
		t.Errorf("cover not replaced (%d bytes)", len(got.CoverData))
	}

	_, other, err := readMP4IlstAtoms(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range unknown {
		var found bool

		for _, a := range other {
			if bytes.Equal(a, want) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("atom %q lost or changed", want[4:8])
		}
	}
}

func TestRebuildMoovFreeform(t *testing.T) {
	existing := [][]byte{
		mp4FreeformAtom("MusicBrainz Release Id", "old"),
		mp4FreeformAtom("CustomKey", "keep"),
	}

	atoms := mergeMP4Freeform(existing, &AudioTags{MBReleaseID: "new", ISRC: "US123"})

	// moov without udta: everything must be created.
	moov := buildMP4Box("mvhd", make([]byte, 100))

	body, err := rebuildMoovFreeform(moov, atoms)
	if err != nil {
		t.Fatal(err)
	}

	var tags AudioTags

	parseMoov(body, &tags, false)

	if tags.MBReleaseID != "new" || tags.ISRC != "US123" {
		t.Fatalf("rel=%q isrc=%q", tags.MBReleaseID, tags.ISRC)
	}

	ilst, ok := mp4Ilst(body)
	if !ok {
		t.Fatal("no ilst")
	}

	names := map[string]bool{}

	for _, b := range mp4children(ilst) {
		names[mp4FreeformName(b.body)] = true
	}

	if !names["CustomKey"] || names["MusicBrainz Release Id"] {
		t.Fatalf("freeform names: %v", names)
	}

	if !bytes.HasPrefix(body, moov) {
		t.Fatal("mvhd not preserved")
	}
}
