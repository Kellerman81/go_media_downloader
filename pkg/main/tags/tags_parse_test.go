package tags

import (
	"encoding/binary"
	"testing"
)

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
	binary.BigEndian.PutUint16(trkn[2:4], 3) // track 3
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

func TestOGGBuildPageOverflowGuard(t *testing.T) {
	h := &OGGHandler{}

	// >65 KB needs more than 255 lacing segments and must be rejected.
	if _, err := h.buildPage(&oggPage{Data: make([]byte, 70000)}); err == nil {
		t.Error("expected error for oversize packet")
	}

	// A small packet must succeed and round-trip its data.
	page, err := h.buildPage(&oggPage{Data: []byte("hello")})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(page[len(page)-5:]) != "hello" {
		t.Errorf("page tail = %q", page[len(page)-5:])
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
