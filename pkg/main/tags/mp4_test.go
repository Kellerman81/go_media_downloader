package tags

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFfmpegBesideFfprobe(t *testing.T) {
	tests := []struct {
		in      string
		windows bool
		want    string
	}{
		// ffprobe_path is the folder (as parser/ffmpeg.go uses it).
		{"/usr/bin", false, "/usr/bin/ffmpeg"},
		{"/usr/bin/", false, "/usr/bin/ffmpeg"},
		{"/opt/ff", true, "/opt/ff/ffmpeg.exe"},
		// A path to the ffprobe binary itself still works.
		{"/usr/bin/ffprobe", false, "/usr/bin/ffmpeg"},
		{"/opt/ff/ffprobe.exe", true, "/opt/ff/ffmpeg.exe"},
	}

	for _, tt := range tests {
		if got := ffmpegBesideFfprobe(tt.in, tt.windows); got != tt.want {
			t.Errorf("ffmpegBesideFfprobe(%q, %v) = %q, want %q", tt.in, tt.windows, got, tt.want)
		}
	}
}

// These tests generate their fixtures with ffmpeg and verify written files
// with ffprobe/ffmpeg, so they skip when the tools are not installed.

func requireFFmpegTools(t *testing.T) {
	t.Helper()

	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed - skipping", tool)
		}
	}
}

func runTool(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()

	out, err := exec.Command(name, args...).CombinedOutput()

	return strings.TrimSpace(string(out)), err
}

// genFixture runs ffmpeg to create a fixture; it skips the test when the
// encoder is unavailable in this ffmpeg build.
func genFixture(t *testing.T, args ...string) {
	t.Helper()

	if out, err := runTool(t, "ffmpeg", append([]string{"-v", "error", "-y"}, args...)...); err != nil {
		t.Skipf("ffmpeg cannot generate fixture (%v): %s", err, out)
	}
}

// assertFFmpegClean fails when ffprobe or a full ffmpeg decode reports errors.
func assertFFmpegClean(t *testing.T, path string) {
	t.Helper()

	if out, err := runTool(t, "ffprobe", "-v", "error", path); err != nil || out != "" {
		t.Fatalf("ffprobe -v error: err=%v output=%q", err, out)
	}

	if out, err := runTool(t, "ffmpeg", "-v", "error", "-i", path, "-f", "null", "-"); err != nil || out != "" {
		t.Fatalf("ffmpeg decode: err=%v output=%q", err, out)
	}
}

func genCover(t *testing.T, dir string) []byte {
	t.Helper()

	path := filepath.Join(dir, "cover.jpg")
	genFixture(t, "-f", "lavfi", "-i", "nullsrc=s=720x720,geq=random(1)*255:random(1)*255:128",
		"-frames:v", "1", "-q:v", "1", path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Must exceed one Ogg page (65025 bytes) to exercise multi-page headers.
	if len(data) < 70000 {
		t.Fatalf("cover only %d bytes", len(data))
	}

	return data
}

func fullTags(cover []byte) *AudioTags {
	return &AudioTags{
		Title: "New Title", Artist: "Artist", Album: "Album", AlbumArtist: "AA", Genre: "Rock",
		Year: 2004, TrackNumber: 3, TotalTracks: 12, DiscNumber: 1, TotalDiscs: 2,
		Composer: "Comp", Conductor: "Cond", Label: "Lbl", CatalogNum: "CAT-9", ISRC: "USX9P0000001",
		MBRecordingID: "rec-id", MBReleaseID: "rel-id", MBReleaseGroupID: "rg-id",
		MBArtistID: "art-id", MBAlbumArtistID: "aart-id", AcoustID: "acoust-id",
		ReplayGainTrack: -6.5, ReplayGainTrackPeak: 0.98,
		CoverData: cover, CoverMIME: "image/jpeg",
	}
}

func assertTagsEqual(t *testing.T, got, want *AudioTags) {
	t.Helper()

	check := func(name string, g, w any) {
		if g != w {
			t.Errorf("%s = %v, want %v", name, g, w)
		}
	}

	check("Title", got.Title, want.Title)
	check("Artist", got.Artist, want.Artist)
	check("Album", got.Album, want.Album)
	check("AlbumArtist", got.AlbumArtist, want.AlbumArtist)
	check("Year", got.Year, want.Year)
	check("TrackNumber", got.TrackNumber, want.TrackNumber)
	check("TotalTracks", got.TotalTracks, want.TotalTracks)
	check("DiscNumber", got.DiscNumber, want.DiscNumber)
	check("TotalDiscs", got.TotalDiscs, want.TotalDiscs)
	check("Conductor", got.Conductor, want.Conductor)
	check("Label", got.Label, want.Label)
	check("CatalogNum", got.CatalogNum, want.CatalogNum)
	check("ISRC", got.ISRC, want.ISRC)
	check("MBRecordingID", got.MBRecordingID, want.MBRecordingID)
	check("MBReleaseID", got.MBReleaseID, want.MBReleaseID)
	check("MBReleaseGroupID", got.MBReleaseGroupID, want.MBReleaseGroupID)
	check("MBArtistID", got.MBArtistID, want.MBArtistID)
	check("MBAlbumArtistID", got.MBAlbumArtistID, want.MBAlbumArtistID)
	check("AcoustID", got.AcoustID, want.AcoustID)
	check("ReplayGainTrack", got.ReplayGainTrack, want.ReplayGainTrack)
	check("ReplayGainTrackPeak", got.ReplayGainTrackPeak, want.ReplayGainTrackPeak)

	if !bytes.Equal(got.CoverData, want.CoverData) {
		t.Errorf("cover: got %d bytes, want %d", len(got.CoverData), len(want.CoverData))
	}
}

func TestMP4WriteTagsFFmpegVerified(t *testing.T) {
	requireFFmpegTools(t)

	dir := t.TempDir()
	cover := genCover(t, dir)
	path := filepath.Join(dir, "a.m4a")

	genFixture(t, "-f", "lavfi", "-i", "sine=frequency=440:duration=3", "-c:a", "aac",
		"-metadata", "title=Old", path)

	h := NewMP4Handler()
	want := fullTags(cover)

	if err := h.WriteTags(context.Background(), path, want); err != nil {
		t.Fatal(err)
	}

	assertFFmpegClean(t, path)

	got, err := h.ReadTagsWithCover(path)
	if err != nil {
		t.Fatal(err)
	}

	assertTagsEqual(t, got, want)

	// Re-tag with only a title: freeform atoms and the cover must survive.
	if err := h.WriteTags(context.Background(), path, &AudioTags{Title: "Again"}); err != nil {
		t.Fatal(err)
	}

	assertFFmpegClean(t, path)

	got, err = h.ReadTagsWithCover(path)
	if err != nil {
		t.Fatal(err)
	}

	want.Title = "Again"
	assertTagsEqual(t, got, want)
}

// mp4Box builds a minimal MP4 box: [4-byte size][4-byte type][body].
func mp4Box(typ string, body []byte) []byte {
	b := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(b[0:4], uint32(8+len(body)))
	copy(b[4:8], typ)
	copy(b[8:], body)

	return b
}

// dataBox builds an iTunes 'data' box with the given type indicator and value.
func dataBox(typeIndicator uint32, value []byte) []byte {
	body := make([]byte, 8+len(value))
	binary.BigEndian.PutUint32(body[0:4], typeIndicator)
	// body[4:8] = locale (zero)
	copy(body[8:], value)

	return mp4Box("data", body)
}

func TestMP4ChildrenAndIlstValue(t *testing.T) {
	// An ilst entry body contains a 'data' box; ilstValue locates it.
	val, dt, ok := ilstValue(dataBox(1, []byte("Hello")))
	if !ok || string(val) != "Hello" || dt != 1 {
		t.Fatalf("ilstValue = %q,%d,%v; want Hello,1,true", val, dt, ok)
	}

	// Two siblings parsed in order.
	data := append(mp4Box("aaaa", []byte("x")), mp4Box("bbbb", []byte("yy"))...)

	boxes := mp4children(data)
	if len(boxes) != 2 || boxes[0].typ != "aaaa" || boxes[1].typ != "bbbb" {
		t.Fatalf("mp4children = %+v", boxes)
	}

	if string(boxes[1].body) != "yy" {
		t.Fatalf("second body = %q", boxes[1].body)
	}
}

func TestParseIlst(t *testing.T) {
	trkn := make([]byte, 8)
	binary.BigEndian.PutUint16(trkn[2:4], 3)  // track 3
	binary.BigEndian.PutUint16(trkn[4:6], 12) // of 12

	ilst := mp4Box("\xa9nam", dataBox(1, []byte("Song Title")))
	ilst = append(ilst, mp4Box("\xa9ART", dataBox(1, []byte("The Artist")))...)
	ilst = append(ilst, mp4Box("trkn", dataBox(0, trkn))...)

	// Freeform MusicBrainz Album Id.
	mean := mp4Box("mean", append([]byte{0, 0, 0, 0}, []byte("com.apple.iTunes")...))
	name := mp4Box("name", append([]byte{0, 0, 0, 0}, []byte("MusicBrainz Album Id")...))
	free := append(mean, name...)
	free = append(free, dataBox(1, []byte("abc-123"))...)
	ilst = append(ilst, mp4Box("----", free)...)

	var tags AudioTags

	parseIlst(ilst, &tags, false)

	if tags.Title != "Song Title" {
		t.Errorf("Title = %q", tags.Title)
	}

	if tags.Artist != "The Artist" {
		t.Errorf("Artist = %q", tags.Artist)
	}

	if tags.TrackNumber != 3 || tags.TotalTracks != 12 {
		t.Errorf("track = %d/%d", tags.TrackNumber, tags.TotalTracks)
	}

	if tags.MBReleaseID != "abc-123" {
		t.Errorf("MBReleaseID = %q", tags.MBReleaseID)
	}
}

// trakBox builds a trak with the given handler, mdhd timescale and sample entry.
func trakBox(handler string, timescale uint32, sampleEntry []byte) []byte {
	hdlr := make([]byte, 24)
	copy(hdlr[8:12], handler)

	mdhd := make([]byte, 24)
	binary.BigEndian.PutUint32(mdhd[12:16], timescale)

	stsd := append(make([]byte, 8), buildMP4Box("mp4a", sampleEntry)...)
	stbl := buildMP4Box("stbl", buildMP4Box("stsd", stsd))
	minf := buildMP4Box("minf", stbl)
	mdia := buildMP4Box("mdia", buildMP4Box("mdhd", mdhd), buildMP4Box("hdlr", hdlr), minf)

	return buildMP4Box("trak", mdia)
}

func TestParseMoovAudioTrackSelection(t *testing.T) {
	se := make([]byte, 28)
	binary.BigEndian.PutUint16(se[16:18], 2)
	binary.BigEndian.PutUint16(se[18:20], 24)
	binary.BigEndian.PutUint32(se[24:28], uint32(uint64(96000)<<16&0xFFFFFFFF)) // truncated, as muxers do

	video := make([]byte, 28)
	binary.BigEndian.PutUint16(video[16:18], 9)

	moov := append(trakBox("vide", 1000, video), trakBox("soun", 96000, se)...)

	var tags AudioTags

	parseMoov(moov, &tags, false)

	if tags.Channels != 2 || tags.BitDepth != 24 || tags.SampleRate != 96000 {
		t.Fatalf("channels=%d depth=%d rate=%d", tags.Channels, tags.BitDepth, tags.SampleRate)
	}
}
