package tags

import (
	"encoding/binary"
	"errors"
	"os"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Chapter is one entry of the chapter list stored in an audio file.
type Chapter struct {
	Title     string
	StartMS   int64
	RuntimeMS int64
}

const (
	// maxChapters bounds the chapter track's sample count against a corrupt
	// table; audiobooks have a few hundred at most.
	maxChapters = 10000
	// maxChapterTitleBytes bounds one chapter sample read.
	maxChapterTitleBytes = 4096
	// maxChapterSpanBytes is the largest file range read at once for all
	// chapter titles; scattered titles are read one by one.
	maxChapterSpanBytes = 1 << 20
)

// ReadMP4Chapters returns the chapter list of an MP4 file (m4b/m4a): the
// QuickTime chapter track the audio track refers to (what Apple players show;
// no count limit), else the Nero chpl list (at most 255 entries). nil when
// the file has neither.
func ReadMP4Chapters(path string) ([]Chapter, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, &ErrReadFailed{Path: path, Reason: err.Error()}
	}
	defer f.Close()

	moov, err := readMoov(f)
	if err != nil {
		return nil, &ErrReadFailed{Path: path, Reason: err.Error()}
	}

	if moov == nil {
		return nil, &ErrReadFailed{Path: path, Reason: "no moov atom found"}
	}

	boxes := mp4children(moov)

	if chapters, err := qtChapters(f, boxes); err == nil && len(chapters) > 0 {
		return chapters, nil
	}

	var movie AudioTags
	if b, ok := findChild(boxes, "mvhd"); ok {
		parseMvhd(b, &movie)
	}

	if udta, ok := findChild(boxes, "udta"); ok {
		if chpl, ok := findChild(mp4children(udta), "chpl"); ok {
			return neroChapters(chpl, movie.Duration.Milliseconds()), nil
		}
	}

	return nil, nil
}

// qtChapters reads the QuickTime chapter track: the text track named by the
// audio track's tref > chap. Each sample is one chapter title, its duration
// the chapter's.
func qtChapters(f *os.File, boxes []mp4box) ([]Chapter, error) {
	var chapID uint32

	for i := range boxes {
		if boxes[i].typ != "trak" || trakHandler(boxes[i].body) != "soun" {
			continue
		}

		if tref, ok := findChild(mp4children(boxes[i].body), "tref"); ok {
			if chap, ok := findChild(mp4children(tref), "chap"); ok && len(chap) >= 4 {
				chapID = binary.BigEndian.Uint32(chap[0:4])
			}
		}

		break
	}

	if chapID == 0 {
		return nil, nil
	}

	for i := range boxes {
		if boxes[i].typ == "trak" && trakID(boxes[i].body) == chapID {
			return readChapterTrack(f, boxes[i].body)
		}
	}

	return nil, nil
}

// trakID returns the track ID from trak > tkhd, or 0.
func trakID(trak []byte) uint32 {
	tkhd, ok := findChild(mp4children(trak), "tkhd")
	if !ok || len(tkhd) < 4 {
		return 0
	}

	off := 12 // version 0: 4-byte creation and modification times
	if tkhd[0] == 1 {
		off = 20
	}

	if len(tkhd) < off+4 {
		return 0
	}

	return binary.BigEndian.Uint32(tkhd[off : off+4])
}

var errBadChapterTrack = errors.New("malformed chapter track")

// readChapterTrack reads the samples of a QuickTime text track as chapters.
func readChapterTrack(f *os.File, trak []byte) ([]Chapter, error) {
	mdia, ok := findChild(mp4children(trak), "mdia")
	if !ok {
		return nil, errBadChapterTrack
	}

	timescale := mdhdTimescale(mdia)

	minf, ok := findChild(mp4children(mdia), "minf")
	if !ok || timescale == 0 {
		return nil, errBadChapterTrack
	}

	stblBody, ok := findChild(mp4children(minf), "stbl")
	if !ok {
		return nil, errBadChapterTrack
	}

	stbl := mp4children(stblBody)

	sizes, err := sampleSizes(stbl)
	if err != nil {
		return nil, err
	}

	durations, err := sampleDurations(stbl, len(sizes))
	if err != nil {
		return nil, err
	}

	offsets, err := sampleOffsets(stbl, sizes)
	if err != nil {
		return nil, err
	}

	ms := func(t uint64) int64 {
		return int64((t*1000 + uint64(timescale)/2) / uint64(timescale)) //nolint:gosec // bounded by the track duration
	}

	read := sampleReader(f, offsets, sizes)
	chapters := make([]Chapter, len(sizes))

	var start uint64

	for i := range sizes {
		sample, err := read(i)
		if err != nil {
			return nil, err
		}

		chapters[i] = Chapter{
			Title:     chapterText(sample),
			StartMS:   ms(start),
			RuntimeMS: ms(start+durations[i]) - ms(start),
		}
		start += durations[i]
	}

	return chapters, nil
}

// sampleReader returns a function reading sample i (at most
// maxChapterTitleBytes of it). The titles of a chapter track usually sit
// together in one chunk; then they are read with one request instead of one
// per chapter, which is what costs time on a network share.
func sampleReader(f *os.File, offsets []int64, sizes []uint32) func(i int) ([]byte, error) {
	lo, hi := offsets[0], int64(0)
	for i := range offsets {
		lo = min(lo, offsets[i])
		hi = max(hi, offsets[i]+int64(min(sizes[i], maxChapterTitleBytes)))
	}

	if hi-lo <= maxChapterSpanBytes {
		span := make([]byte, hi-lo)
		if _, err := f.ReadAt(span, lo); err == nil {
			return func(i int) ([]byte, error) {
				from := offsets[i] - lo

				return span[from : from+int64(min(sizes[i], maxChapterTitleBytes))], nil
			}
		}
	}

	buf := make([]byte, maxChapterTitleBytes)

	return func(i int) ([]byte, error) {
		n := min(int(sizes[i]), len(buf))
		if _, err := f.ReadAt(buf[:n], offsets[i]); err != nil {
			return nil, err
		}

		return buf[:n], nil
	}
}

// sampleSizes reads stsz: a constant size or one size per sample.
func sampleSizes(stbl []mp4box) ([]uint32, error) {
	stsz, ok := findChild(stbl, "stsz")
	if !ok || len(stsz) < 12 {
		return nil, errBadChapterTrack
	}

	constant := binary.BigEndian.Uint32(stsz[4:8])
	count := int(binary.BigEndian.Uint32(stsz[8:12]))

	if count > maxChapters || (constant == 0 && len(stsz) < 12+4*count) {
		return nil, errBadChapterTrack
	}

	sizes := make([]uint32, count)
	for i := range sizes {
		if constant > 0 {
			sizes[i] = constant
		} else {
			sizes[i] = binary.BigEndian.Uint32(stsz[12+4*i:])
		}
	}

	return sizes, nil
}

// sampleDurations expands stts (runs of sample count and duration) to one
// duration per sample.
func sampleDurations(stbl []mp4box, count int) ([]uint64, error) {
	stts, ok := findChild(stbl, "stts")
	if !ok || len(stts) < 8 {
		return nil, errBadChapterTrack
	}

	entries := int(binary.BigEndian.Uint32(stts[4:8]))
	if len(stts) < 8+8*entries {
		return nil, errBadChapterTrack
	}

	out := make([]uint64, 0, count)

	for i := 0; i < entries && len(out) < count; i++ {
		n := int(binary.BigEndian.Uint32(stts[8+8*i:]))
		delta := uint64(binary.BigEndian.Uint32(stts[12+8*i:]))

		for j := 0; j < n && len(out) < count; j++ {
			out = append(out, delta)
		}
	}

	if len(out) != count {
		return nil, errBadChapterTrack
	}

	return out, nil
}

// sampleOffsets returns each sample's file offset from the chunk offsets
// (stco/co64) and the samples per chunk (stsc).
func sampleOffsets(stbl []mp4box, sizes []uint32) ([]int64, error) {
	var chunks []int64

	if stco, ok := findChild(stbl, "stco"); ok && len(stco) >= 8 {
		n := int(binary.BigEndian.Uint32(stco[4:8]))
		if n > maxChapters || len(stco) < 8+4*n {
			return nil, errBadChapterTrack
		}

		for i := range n {
			chunks = append(chunks, int64(binary.BigEndian.Uint32(stco[8+4*i:])))
		}
	} else if co64, ok := findChild(stbl, "co64"); ok && len(co64) >= 8 {
		n := int(binary.BigEndian.Uint32(co64[4:8]))
		if n > maxChapters || len(co64) < 8+8*n {
			return nil, errBadChapterTrack
		}

		for i := range n {
			chunks = append(chunks, int64(binary.BigEndian.Uint64(co64[8+8*i:]))) //nolint:gosec // file offset
		}
	}

	stsc, ok := findChild(stbl, "stsc")
	if !ok || len(stsc) < 8 || len(chunks) == 0 {
		return nil, errBadChapterTrack
	}

	entries := int(binary.BigEndian.Uint32(stsc[4:8]))
	if entries == 0 || len(stsc) < 8+12*entries {
		return nil, errBadChapterTrack
	}

	offsets := make([]int64, 0, len(sizes))

	// stsc entry e covers chunks firstChunk(e) .. firstChunk(e+1)-1 (1-based).
	for e := 0; e < entries && len(offsets) < len(sizes); e++ {
		first := int(binary.BigEndian.Uint32(stsc[8+12*e:]))
		perChunk := int(binary.BigEndian.Uint32(stsc[12+12*e:]))

		last := len(chunks)
		if e+1 < entries {
			last = int(binary.BigEndian.Uint32(stsc[8+12*(e+1):])) - 1
		}

		for c := first; c <= last && c <= len(chunks) && len(offsets) < len(sizes); c++ {
			if c < 1 {
				return nil, errBadChapterTrack
			}

			off := chunks[c-1]
			for range perChunk {
				if len(offsets) == len(sizes) {
					break
				}

				offsets = append(offsets, off)
				off += int64(sizes[len(offsets)-1])
			}
		}
	}

	if len(offsets) != len(sizes) {
		return nil, errBadChapterTrack
	}

	return offsets, nil
}

// chapterText decodes a QuickTime text sample: a 2-byte length, then UTF-8
// text, or UTF-16 after a byte-order mark.
func chapterText(sample []byte) string {
	if len(sample) < 2 {
		return ""
	}

	text := sample[2:]
	if n := int(binary.BigEndian.Uint16(sample[0:2])); n < len(text) {
		text = text[:n]
	}

	if len(text) >= 2 && (text[0] == 0xFE && text[1] == 0xFF || text[0] == 0xFF && text[1] == 0xFE) {
		order := binary.ByteOrder(binary.BigEndian)
		if text[0] == 0xFF {
			order = binary.LittleEndian
		}

		units := make([]uint16, (len(text)-2)/2)
		for i := range units {
			units[i] = order.Uint16(text[2+2*i:])
		}

		return strings.TrimSpace(string(utf16.Decode(units)))
	}

	return strings.TrimSpace(string(text))
}

// neroChapters reads a Nero chpl box: version/flags, 4 reserved bytes
// (version 1), a 1-byte count, then per chapter an 8-byte start in 100 ns
// units and a length-prefixed title. The last chapter runs to movieMs.
func neroChapters(chpl []byte, movieMs int64) []Chapter {
	if len(chpl) < 5 {
		return nil
	}

	p := 4
	if chpl[0] == 1 {
		p += 4
	}

	if len(chpl) <= p {
		return nil
	}

	count := int(chpl[p])
	p++

	chapters := make([]Chapter, 0, count)

	for range count {
		if len(chpl) < p+9 {
			break
		}

		start := int64(binary.BigEndian.Uint64(chpl[p:p+8]) / 10000) //nolint:gosec // 100 ns units to ms
		n := int(chpl[p+8])
		p += 9

		if len(chpl) < p+n {
			break
		}

		chapters = append(chapters, Chapter{Title: strings.TrimSpace(string(chpl[p : p+n])), StartMS: start})
		p += n
	}

	for i := range chapters {
		end := movieMs
		if i+1 < len(chapters) {
			end = chapters[i+1].StartMS
		}

		chapters[i].RuntimeMS = max(end-chapters[i].StartMS, 0)
	}

	if len(chapters) == 0 {
		return nil
	}

	return chapters
}

// ffmetadataChapters renders chapters as an ffmpeg FFMETADATA file, the input
// WriteTags maps chapters from when AudioTags.Chapters is set.
func ffmetadataChapters(chapters []Chapter) string {
	escape := strings.NewReplacer(`\`, `\\`, "=", `\=`, ";", `\;`, "#", `\#`, "\n", `\`+"\n")

	var b strings.Builder

	b.WriteString(";FFMETADATA1\n")

	for i := range chapters {
		b.WriteString("[CHAPTER]\nTIMEBASE=1/1000\nSTART=")
		b.WriteString(strconv.FormatInt(chapters[i].StartMS, 10))
		b.WriteString("\nEND=")
		b.WriteString(strconv.FormatInt(chapters[i].StartMS+chapters[i].RuntimeMS, 10))
		b.WriteString("\ntitle=")
		b.WriteString(escape.Replace(chapters[i].Title))
		b.WriteString("\n")
	}

	return b.String()
}
