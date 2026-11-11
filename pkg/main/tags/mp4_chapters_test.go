package tags

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// genChapterFixture writes a 6 s AAC file at path with the given chapters
// (FFMETADATA text) via ffmpeg, in the given muxer.
func genChapterFixture(t *testing.T, path, format string, chapters []Chapter) {
	t.Helper()

	meta := filepath.Join(t.TempDir(), "chapters.txt")
	if err := os.WriteFile(meta, []byte(ffmetadataChapters(chapters)), 0o600); err != nil {
		t.Fatal(err)
	}

	genFixture(t, "-f", "lavfi", "-i", "sine=duration=6", "-f", "ffmetadata", "-i", meta,
		"-map", "0", "-map_chapters", "1", "-c:a", "aac", "-f", format, path)
}

func TestReadMP4Chapters(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	want := []Chapter{
		{Title: "Kapitel 1: Räuber", StartMS: 0, RuntimeMS: 2500},
		{Title: "Kapitel 2 = Teil; #2", StartMS: 2500, RuntimeMS: 1500},
		{Title: `Kapitel 3 \ Ende`, StartMS: 4000, RuntimeMS: 2000},
	}

	for _, format := range []string{"ipod", "mp4"} {
		path := filepath.Join(dir, "book-"+format+".m4b")
		genChapterFixture(t, path, format, want)

		got, err := ReadMP4Chapters(path)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}

		if !chaptersClose(got, want) {
			t.Errorf("%s: chapters = %+v, want %+v", format, got, want)
		}
	}

	plain := filepath.Join(dir, "plain.m4a")
	genFixture(t, "-f", "lavfi", "-i", "sine=duration=2", "-c:a", "aac", plain)

	if got, err := ReadMP4Chapters(plain); err != nil || got != nil {
		t.Errorf("file without chapters: %+v, %v; want nil, nil", got, err)
	}

	if _, err := ReadMP4Chapters(filepath.Join(dir, "missing.m4b")); err == nil {
		t.Error("missing file: want an error")
	}
}

// chaptersClose compares chapter lists, allowing for the AAC frame rounding
// of chapter boundaries (a frame is 1024 samples, 23 ms at 44.1 kHz).
func chaptersClose(got, want []Chapter) bool {
	return slices.EqualFunc(got, want, func(a, b Chapter) bool {
		near := func(x, y int64) bool { return x-y <= 25 && y-x <= 25 }
		return a.Title == b.Title && near(a.StartMS, b.StartMS) && near(a.RuntimeMS, b.RuntimeMS)
	})
}

func TestNeroChapters(t *testing.T) {
	chpl := []byte{1, 0, 0, 0, 0, 0, 0, 0, 2}

	for _, c := range []struct {
		start uint64 // 100 ns units
		title string
	}{{0, "Eins"}, {25_000_000, "Zwei"}} {
		var start [8]byte
		binary.BigEndian.PutUint64(start[:], c.start)
		chpl = append(chpl, start[:]...)
		chpl = append(chpl, byte(len(c.title)))
		chpl = append(chpl, c.title...)
	}

	want := []Chapter{{Title: "Eins", RuntimeMS: 2500}, {Title: "Zwei", StartMS: 2500, RuntimeMS: 3500}}
	if got := neroChapters(chpl, 6000); !slices.Equal(got, want) {
		t.Errorf("chapters = %+v, want %+v", got, want)
	}

	// A truncated entry ends the list.
	if got := neroChapters(chpl[:len(chpl)-2], 6000); len(got) != 1 || got[0].RuntimeMS != 6000 {
		t.Errorf("truncated: %+v, want only the first chapter running to the end", got)
	}

	if got := neroChapters([]byte{0, 0, 0, 0}, 6000); got != nil {
		t.Errorf("empty box: %+v, want nil", got)
	}
}

func TestChapterText(t *testing.T) {
	utf16BE := []byte{0, 8, 0xFE, 0xFF, 0, 'A', 0, 0xE4, 0, 'b'}

	tests := []struct {
		name   string
		sample []byte
		want   string
	}{
		{"utf-8", append([]byte{0, 7}, "Räuber"...), "Räuber"},
		{"length shorter than sample", append([]byte{0, 3}, "Abc trailing"...), "Abc"},
		{"utf-16 with BOM", utf16BE, "Aäb"},
		{"too short", []byte{0}, ""},
	}

	for _, tt := range tests {
		if got := chapterText(tt.sample); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestMP4WriteTagsChapters(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "book.m4b")
	old := []Chapter{
		{Title: "001 - Kapitel 1 & Kapitel 2.1 - Buch", StartMS: 0, RuntimeMS: 2500},
		{Title: "002 - Kapitel 2.2 - Buch", StartMS: 2500, RuntimeMS: 3500},
	}
	genChapterFixture(t, path, "ipod", old)

	h := NewMP4Handler()

	// Without Chapters the file keeps its own.
	if err := h.WriteTags(context.Background(), path, &AudioTags{Title: "Buch"}); err != nil {
		t.Fatal(err)
	}

	if got, _ := ReadMP4Chapters(path); !chaptersClose(got, old) {
		t.Fatalf("chapters changed without AudioTags.Chapters: %+v", got)
	}

	renamed := []Chapter{
		{Title: "Kapitel 1 = Anfang; #1", StartMS: old[0].StartMS, RuntimeMS: old[0].RuntimeMS},
		{Title: `Kapitel 2 \ Ende`, StartMS: old[1].StartMS, RuntimeMS: old[1].RuntimeMS},
	}

	// With a new cover as well, so the chapter input is ffmpeg input 2.
	cover := genCover(t, dir)
	if err := h.WriteTags(context.Background(), path, &AudioTags{
		Title: "Buch", CoverData: cover, CoverMIME: "image/jpeg", Chapters: renamed,
	}); err != nil {
		t.Fatal(err)
	}

	assertFFmpegClean(t, path)

	got, err := ReadMP4Chapters(path)
	if err != nil || !chaptersClose(got, renamed) {
		t.Errorf("chapters after write: %+v, %v; want %+v", got, err, renamed)
	}

	out, err := runTool(t, "ffprobe", "-v", "error", "-show_entries", "chapter_tags=title",
		"-of", "csv=p=0", path)
	if err != nil || !strings.Contains(out, "Kapitel 1 = Anfang; #1") {
		t.Errorf("ffprobe chapter titles: %q, %v", out, err)
	}

	if tags, err := h.ReadTagsWithCover(path); err != nil || tags.Title != "Buch" || len(tags.CoverData) == 0 {
		t.Errorf("tags after write: %+v, %v", tags, err)
	}
}
