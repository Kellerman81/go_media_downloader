package tags

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// mp4FreeformField maps an AudioTags field to the iTunes freeform ("----",
// mean "com.apple.iTunes") name parseFreeform reads it from. names[0] is
// written; all names are removed from the existing atoms when the field is set.
type mp4FreeformField struct {
	names []string
	value func(*AudioTags) string
}

var mp4FreeformFields = []mp4FreeformField{
	{[]string{"MusicBrainz Track Id", "MusicBrainz Recording Id"}, func(t *AudioTags) string { return t.MBRecordingID }},
	{[]string{"MusicBrainz Album Id", "MusicBrainz Release Id"}, func(t *AudioTags) string { return t.MBReleaseID }},
	{[]string{"MusicBrainz Release Group Id"}, func(t *AudioTags) string { return t.MBReleaseGroupID }},
	{[]string{"MusicBrainz Artist Id"}, func(t *AudioTags) string { return t.MBArtistID }},
	{[]string{"MusicBrainz Album Artist Id"}, func(t *AudioTags) string { return t.MBAlbumArtistID }},
	{[]string{"Acoustid Id"}, func(t *AudioTags) string { return t.AcoustID }},
	{[]string{"CATALOGNUMBER"}, func(t *AudioTags) string { return t.CatalogNum }},
	{[]string{"LABEL"}, func(t *AudioTags) string { return t.Label }},
	{[]string{"ISRC"}, func(t *AudioTags) string { return t.ISRC }},
	{[]string{"CONDUCTOR"}, func(t *AudioTags) string { return t.Conductor }},
	{[]string{"replaygain_track_gain"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrack, "%.2f dB") }},
	{[]string{"replaygain_track_peak"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrackPeak, "%.6f") }},
	{[]string{"replaygain_album_gain"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbum, "%.2f dB") }},
	{[]string{"replaygain_album_peak"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbumPeak, "%.6f") }},
}

// buildMP4Box serializes a box with a 32-bit size header.
func buildMP4Box(typ string, body ...[]byte) []byte {
	size := 8
	for _, b := range body {
		size += len(b)
	}

	out := make([]byte, 8, size)
	binary.BigEndian.PutUint32(out[0:4], uint32(size)) //nolint:gosec // moov-level boxes are far below 4 GB
	copy(out[4:8], typ)

	for _, b := range body {
		out = append(out, b...)
	}

	return out
}

// mp4FreeformAtom builds a "----" atom with a UTF-8 data value.
func mp4FreeformAtom(name, value string) []byte {
	return buildMP4Box("----",
		buildMP4Box("mean", []byte{0, 0, 0, 0}, []byte("com.apple.iTunes")),
		buildMP4Box("name", []byte{0, 0, 0, 0}, []byte(name)),
		buildMP4Box("data", []byte{0, 0, 0, 1, 0, 0, 0, 0}, []byte(value)),
	)
}

// mp4FreeformName returns the name of a "----" atom body.
func mp4FreeformName(body []byte) string {
	for _, b := range mp4children(body) {
		if b.typ == "name" && len(b.body) >= 4 {
			return string(b.body[4:])
		}
	}

	return ""
}

// mp4Ilst returns the ilst body of a moov body (udta > meta > ilst).
func mp4Ilst(moov []byte) ([]byte, bool) {
	udta, ok := findChild(mp4children(moov), "udta")
	if !ok {
		return nil, false
	}

	meta, ok := findChild(mp4children(udta), "meta")
	if !ok || len(meta) < 4 {
		return nil, false
	}

	return findChild(mp4children(meta[4:]), "ilst")
}

// readMP4IlstAtoms returns the existing ilst entries of an MP4 file as
// complete boxes: the freeform ("----") atoms and all other atoms.
func readMP4IlstAtoms(path string) (freeform, other [][]byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	moov, err := readMoov(f)
	if err != nil || moov == nil {
		return nil, nil, err
	}

	ilst, ok := mp4Ilst(moov)
	if !ok {
		return nil, nil, nil
	}

	for _, b := range mp4children(ilst) {
		if b.typ == "----" {
			freeform = append(freeform, buildMP4Box("----", b.body))
		} else {
			other = append(other, buildMP4Box(b.typ, b.body))
		}
	}

	return freeform, other, nil
}

// droppedMP4Atoms returns the atoms of original whose type is missing from
// the ilst of the remuxed file at path. ffmpeg's mov muxer only writes the
// keys it knows and silently drops the rest (rtng, soar, tvsh, purd, ...);
// keys WriteTags sets are written by ffmpeg, so they are never "missing".
func droppedMP4Atoms(path string, original [][]byte) ([][]byte, error) {
	if len(original) == 0 {
		return nil, nil
	}

	_, remuxed, err := readMP4IlstAtoms(path)
	if err != nil {
		return nil, err
	}

	present := make(map[string]struct{}, len(remuxed))
	for _, a := range remuxed {
		present[string(a[4:8])] = struct{}{}
	}

	var dropped [][]byte

	for _, a := range original {
		if _, ok := present[string(a[4:8])]; !ok {
			dropped = append(dropped, a)
		}
	}

	return dropped, nil
}

// mergeMP4Freeform returns the freeform atoms to write: the new values tags
// sets, plus every existing atom whose name tags doesn't set (custom keys,
// ReplayGain from another tool, ...).
func mergeMP4Freeform(existing [][]byte, tags *AudioTags) [][]byte {
	var (
		out     [][]byte
		replace []string
	)

	for i := range mp4FreeformFields {
		v := mp4FreeformFields[i].value(tags)
		if v == "" {
			continue
		}

		replace = append(replace, mp4FreeformFields[i].names...)
		out = append(out, mp4FreeformAtom(mp4FreeformFields[i].names[0], v))
	}

	for _, atom := range existing {
		name := mp4FreeformName(atom[8:])

		keep := true

		for _, r := range replace {
			if strings.EqualFold(name, r) {
				keep = false
				break
			}
		}

		if keep {
			out = append(out, atom)
		}
	}

	return out
}

// mp4ChildrenStrict splits a box body into children and reports whether they
// cover it exactly, so a body can be re-serialized without losing bytes.
// Children with 64-bit size headers make it report false.
func mp4ChildrenStrict(data []byte) ([]mp4box, bool) {
	boxes := mp4children(data)

	consumed := 0
	for i := range boxes {
		consumed += len(boxes[i].body) + 8
	}

	return boxes, consumed == len(data)
}

// replaceChild re-serializes boxes with the first box of type typ replaced
// by body (or appended if absent).
func replaceChild(boxes []mp4box, typ string, body []byte) []byte {
	var out []byte

	found := false

	for i := range boxes {
		if boxes[i].typ == typ && !found {
			out = append(out, buildMP4Box(typ, body)...)
			found = true

			continue
		}

		out = append(out, buildMP4Box(boxes[i].typ, boxes[i].body)...)
	}

	if !found {
		out = append(out, buildMP4Box(typ, body)...)
	}

	return out
}

// mp4MetaHdlr is the handler box of an iTunes metadata "meta" box.
var mp4MetaHdlr = buildMP4Box("hdlr",
	[]byte{0, 0, 0, 0, 0, 0, 0, 0}, []byte("mdirappl"), make([]byte, 9))

// rebuildMoovFreeform returns a new moov body whose udta > meta > ilst holds
// atoms as its only freeform entries, creating the boxes when missing. atoms
// may also carry non-freeform entries to append (see droppedMP4Atoms). All
// other boxes are kept byte for byte (re-serialized with 32-bit sizes).
func rebuildMoovFreeform(moov []byte, atoms [][]byte) ([]byte, error) {
	moovBoxes, ok := mp4ChildrenStrict(moov)
	if !ok {
		return nil, errors.New("unparseable moov box")
	}

	udta, _ := findChild(moovBoxes, "udta")

	udtaBoxes, ok := mp4ChildrenStrict(udta)
	if !ok {
		return nil, errors.New("unparseable udta box")
	}

	meta, hasMeta := findChild(udtaBoxes, "meta")
	if !hasMeta {
		meta = append([]byte{0, 0, 0, 0}, mp4MetaHdlr...)
	}

	if len(meta) < 4 {
		return nil, errors.New("truncated meta box")
	}

	metaBoxes, ok := mp4ChildrenStrict(meta[4:])
	if !ok {
		return nil, errors.New("unparseable meta box")
	}

	ilst, _ := findChild(metaBoxes, "ilst")

	ilstBoxes, ok := mp4ChildrenStrict(ilst)
	if !ok {
		return nil, errors.New("unparseable ilst box")
	}

	var newIlst []byte

	for i := range ilstBoxes {
		if ilstBoxes[i].typ != "----" {
			newIlst = append(newIlst, buildMP4Box(ilstBoxes[i].typ, ilstBoxes[i].body)...)
		}
	}

	for _, a := range atoms {
		newIlst = append(newIlst, a...)
	}

	newMeta := append(append([]byte(nil), meta[:4]...), replaceChild(metaBoxes, "ilst", newIlst)...)
	newUdta := replaceChild(udtaBoxes, "meta", newMeta)

	return replaceChild(moovBoxes, "udta", newUdta), nil
}

// writeMP4Freeform rewrites the moov box of the MP4 file at path so its ilst
// carries atoms (see rebuildMoovFreeform). The moov must be the last top-level box
// (ffmpeg's default layout without faststart), so resizing it shifts no
// sample data and no chunk offsets need patching. Anything else is refused.
func writeMP4Freeform(path string, atoms [][]byte) error {
	if len(atoms) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}

	var (
		hdr     [16]byte
		off     int64
		moovOff = int64(-1)
		moovLen int64
		hdrLen  int64
	)

	for off < st.Size() {
		if _, err := f.ReadAt(hdr[:8], off); err != nil {
			return err
		}

		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		hl := int64(8)

		switch size {
		case 1:
			if _, err := f.ReadAt(hdr[8:16], off+8); err != nil {
				return err
			}

			size = int64(binary.BigEndian.Uint64(hdr[8:16])) //nolint:gosec // validated below
			hl = 16
		case 0:
			size = st.Size() - off
		}

		if size < hl || off+size > st.Size() {
			return fmt.Errorf("malformed top-level box at offset %d", off)
		}

		if string(hdr[4:8]) == "moov" {
			moovOff, moovLen, hdrLen = off, size, hl
		}

		off += size
	}

	if moovOff < 0 {
		return errors.New("no moov box")
	}

	if moovOff+moovLen != st.Size() {
		return errors.New("moov is not the last box; refusing to shift sample data")
	}

	if moovLen-hdrLen > maxMoovBytes {
		return errors.New("moov atom too large")
	}

	moov := make([]byte, moovLen-hdrLen)
	if _, err := f.ReadAt(moov, moovOff+hdrLen); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	body, err := rebuildMoovFreeform(moov, atoms)
	if err != nil {
		return err
	}

	if _, err := f.WriteAt(buildMP4Box("moov", body), moovOff); err != nil {
		return err
	}

	if err := f.Truncate(moovOff + 8 + int64(len(body))); err != nil {
		return err
	}

	return f.Sync()
}
