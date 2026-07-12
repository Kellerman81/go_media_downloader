package parser

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// nativeProbeExts are the container extensions the native prober handles. Other
// extensions (e.g. .mpg/.ts/.wmv/.flv) skip straight to ffprobe without an open.
var nativeProbeExts = map[string]struct{}{
	".mp4":  {},
	".m4v":  {},
	".mov":  {},
	".mkv":  {},
	".webm": {},
	".avi":  {},
}

// nativeProbeSupportsExt reports whether file's extension is one the native
// prober can parse, so callers can avoid opening unsupported files.
func nativeProbeSupportsExt(file string) bool {
	_, ok := nativeProbeExts[logger.FileExt(file)]
	return ok
}

// Native container probing extracts the same fields ffprobe provides
// (video/audio codec, resolution, channels, language, duration) directly from
// MP4/MOV, Matroska/WebM and AVI headers, avoiding an ffprobe subprocess per
// file. It is best-effort: on any malformed/unsupported input it returns an
// error so the caller falls back to ffprobe/mediainfo. The result is shaped as
// an *ffProbeJSON so the existing parseffprobe mapping is reused unchanged.

var errNativeUnsupported = errors.New("native probe: unsupported or insufficient data")

// maxHeaderProbe caps how much of a container's header region is read into
// memory (moov atom / EBML head / AVI hdrl). Real headers are well under this.
const maxHeaderProbe = 64 << 20

// nativeProbe analyzes file by sniffing the container magic and dispatching to
// the matching parser. Returns errNativeUnsupported when it cannot handle the
// file so the caller can fall back to ffprobe.
func nativeProbe(file string) (*ffProbeJSON, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var magic [12]byte

	n, _ := io.ReadFull(f, magic[:])
	if n < 12 {
		return nil, errNativeUnsupported
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	switch {
	case magic[4] == 'f' && magic[5] == 't' && magic[6] == 'y' && magic[7] == 'p':
		return probeMP4(f)
	case magic[0] == 0x1A && magic[1] == 0x45 && magic[2] == 0xDF && magic[3] == 0xA3:
		return probeMKV(f)
	case string(magic[0:4]) == "RIFF" && string(magic[8:12]) == "AVI ":
		return probeAVI(f)
	}

	return nil, errNativeUnsupported
}

// hasUsableVideo reports whether a video stream was found with both a known
// codec and a resolution. When false the caller treats the probe as a miss and
// falls back to ffprobe, so native only "wins" when it is confident.
func hasUsableVideo(streams []ffProbeStream) bool {
	for i := range streams {
		if strings.EqualFold(streams[i].CodecType, "video") &&
			streams[i].CodecName != "" && streams[i].Height > 0 {
			return true
		}
	}

	return false
}

func langStream(codecType, codecName, lang string) ffProbeStream {
	s := ffProbeStream{CodecType: codecType, CodecName: codecName}
	if lang != "" && !strings.EqualFold(lang, "und") {
		s.Tags = map[string]string{"language": lang}
	}

	return s
}

// ---------------------------------------------------------------------------
// MP4 / MOV (ISO base media file format)
// ---------------------------------------------------------------------------

type mp4Box struct {
	typ  string
	body []byte
}

func mp4Boxes(data []byte) []mp4Box {
	var out []mp4Box

	for off := 0; off+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])

		switch {
		case size == 1:
			if off+16 > len(data) {
				return out
			}

			sz := binary.BigEndian.Uint64(data[off+8 : off+16])
			if sz < 16 || sz > uint64(len(data)-off) {
				return out
			}

			out = append(out, mp4Box{typ, data[off+16 : off+int(sz)]})

			off += int(sz)

		case size < 8 || off+size > len(data):
			return out
		default:
			out = append(out, mp4Box{typ, data[off+8 : off+size]})

			off += size
		}
	}

	return out
}

func findMP4Box(boxes []mp4Box, typ string) ([]byte, bool) {
	for i := range boxes {
		if boxes[i].typ == typ {
			return boxes[i].body, true
		}
	}

	return nil, false
}

func probeMP4(f *os.File) (*ffProbeJSON, error) {
	moov, err := readTopLevelBox(f, "moov")
	if err != nil || moov == nil {
		return nil, errNativeUnsupported
	}

	out := &ffProbeJSON{}
	boxes := mp4Boxes(moov)

	if b, ok := findMP4Box(boxes, "mvhd"); ok {
		if ts, dur := parseMvhd(b); ts > 0 && dur > 0 {
			out.Format.Duration = strconv.FormatFloat(float64(dur)/float64(ts), 'f', 3, 64)
		}
	}

	for i := range boxes {
		if boxes[i].typ != "trak" {
			continue
		}

		if st, ok := mp4TrakStream(boxes[i].body); ok {
			out.Streams = append(out.Streams, st)
		}
	}

	if !hasUsableVideo(out.Streams) {
		return nil, errNativeUnsupported
	}

	return out, nil
}

// readTopLevelBox walks the top-level boxes of an ISO BMFF file and returns the
// body of the named box, seeking past large boxes (mdat) without reading them.
func readTopLevelBox(f *os.File, want string) ([]byte, error) {
	var hdr [8]byte

	for {
		if _, err := io.ReadFull(f, hdr[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil, nil
			}

			return nil, err
		}

		size := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		headerLen := int64(8)

		switch size {
		case 1:
			var ext [8]byte
			if _, err := io.ReadFull(f, ext[:]); err != nil {
				return nil, err
			}

			size = int64(binary.BigEndian.Uint64(ext[:])) //nolint:gosec // bounded below
			headerLen = 16

		case 0:
			if typ == want {
				return io.ReadAll(io.LimitReader(f, maxHeaderProbe))
			}

			return nil, nil
		}

		if size < headerLen {
			return nil, nil
		}

		body := size - headerLen
		if typ == want {
			if body > maxHeaderProbe {
				return nil, errNativeUnsupported
			}

			buf := make([]byte, body)
			if _, err := io.ReadFull(f, buf); err != nil {
				return nil, err
			}

			return buf, nil
		}

		if _, err := f.Seek(body, io.SeekCurrent); err != nil {
			return nil, err
		}
	}
}

// parseMvhd returns (timescale, duration) from a movie header box.
func parseMvhd(b []byte) (uint64, uint64) {
	if len(b) < 1 {
		return 0, 0
	}

	if b[0] == 1 {
		if len(b) < 32 {
			return 0, 0
		}

		return uint64(binary.BigEndian.Uint32(b[20:24])), binary.BigEndian.Uint64(b[24:32])
	}

	if len(b) < 20 {
		return 0, 0
	}

	return uint64(binary.BigEndian.Uint32(b[12:16])), uint64(binary.BigEndian.Uint32(b[16:20]))
}

// mp4TrakStream extracts a stream entry from a trak box (mdia > {hdlr, mdhd, minf > stbl > stsd}).
func mp4TrakStream(trak []byte) (ffProbeStream, bool) {
	mdia, ok := findMP4Box(mp4Boxes(trak), "mdia")
	if !ok {
		return ffProbeStream{}, false
	}

	mboxes := mp4Boxes(mdia)

	handler := ""
	if hdlr, ok := findMP4Box(mboxes, "hdlr"); ok && len(hdlr) >= 12 {
		handler = string(hdlr[8:12]) // 'vide' / 'soun'
	}

	lang := ""
	if mdhd, ok := findMP4Box(mboxes, "mdhd"); ok {
		lang = parseMdhdLanguage(mdhd)
	}

	minf, ok := findMP4Box(mboxes, "minf")
	if !ok {
		return ffProbeStream{}, false
	}

	stbl, ok := findMP4Box(mp4Boxes(minf), "stbl")
	if !ok {
		return ffProbeStream{}, false
	}

	stsd, ok := findMP4Box(mp4Boxes(stbl), "stsd")
	if !ok || len(stsd) < 8 {
		return ffProbeStream{}, false
	}

	entries := mp4Boxes(stsd[8:]) // skip version/flags + entry count
	if len(entries) == 0 {
		return ffProbeStream{}, false
	}

	se := entries[0]

	switch handler {
	case "vide":
		st := langStream("video", mp4VideoCodec(se.typ), lang)
		if len(se.body) >= 28 {
			st.Width = int(binary.BigEndian.Uint16(se.body[24:26]))
			st.Height = int(binary.BigEndian.Uint16(se.body[26:28]))
		}

		return st, true

	case "soun":
		st := langStream("audio", mp4AudioCodec(se.typ), lang)
		if len(se.body) >= 28 {
			st.Channels = int(binary.BigEndian.Uint16(se.body[16:18]))
			st.SampleRate = strconv.Itoa(int(binary.BigEndian.Uint32(se.body[24:28]) >> 16))
		}

		return st, true
	}

	return ffProbeStream{}, false
}

// parseMdhdLanguage decodes the 3-letter ISO-639-2 language packed in mdhd.
func parseMdhdLanguage(b []byte) string {
	if len(b) < 1 {
		return ""
	}

	var off int
	if b[0] == 1 {
		off = 4 + 8 + 8 + 4 + 8 // version/flags, ctime, mtime, timescale, duration
	} else {
		off = 4 + 4 + 4 + 4 + 4
	}

	if off+2 > len(b) {
		return ""
	}

	packed := binary.BigEndian.Uint16(b[off : off+2])
	c1 := byte((packed>>10)&0x1f) + 0x60
	c2 := byte((packed>>5)&0x1f) + 0x60
	c3 := byte(packed&0x1f) + 0x60

	if c1 < 'a' || c1 > 'z' {
		return ""
	}

	return string([]byte{c1, c2, c3})
}

func mp4VideoCodec(typ string) string {
	switch typ {
	case "avc1", "avc3":
		return "h264"
	case "hvc1", "hev1", "dvh1", "dvhe":
		return "hevc"
	case "vp09":
		return "vp9"
	case "av01":
		return "av1"
	case "mp4v":
		return "mpeg4"
	case "mp2v", "mpg2":
		return "mpeg2"
	}

	return strings.TrimSpace(typ)
}

func mp4AudioCodec(typ string) string {
	switch typ {
	case "mp4a":
		return "aac"
	case "ac-3":
		return "ac3"
	case "ec-3":
		return "eac3"
	case "dtsc", "dtsh", "dtse", "dtsl":
		return "dts"
	case ".mp3", "mp3 ":
		return "mp3"
	case "fLaC":
		return "flac"
	case "Opus":
		return "opus"
	case "alac":
		return "alac"
	}

	return strings.TrimSpace(typ)
}

// ---------------------------------------------------------------------------
// Matroska / WebM (EBML)
// ---------------------------------------------------------------------------

// EBML element IDs (with their length-descriptor leading bits).
const (
	ebmlSegment       = 0x18538067
	ebmlInfo          = 0x1549A966
	ebmlTimecodeScale = 0x2AD7B1
	ebmlDuration      = 0x4489
	ebmlTracks        = 0x1654AE6B
	ebmlTrackEntry    = 0xAE
	ebmlTrackType     = 0x83
	ebmlCodecID       = 0x86
	ebmlLanguage      = 0x22B59C
	ebmlVideo         = 0xE0
	ebmlPixelWidth    = 0xB0
	ebmlPixelHeight   = 0xBA
	ebmlAudio         = 0xE1
	ebmlChannels      = 0x9F
	ebmlSampleFreq    = 0xB5
	ebmlCluster       = 0x1F43B675
)

// ebmlReader walks EBML elements from a byte slice.
type ebmlReader struct {
	data []byte
	pos  int
}

// elem reads the next (id, content) element. ok is false at end/error.
func (r *ebmlReader) elem() (id uint64, content []byte, ok bool) {
	id, n := readEbmlID(r.data[r.pos:])
	if n == 0 {
		return 0, nil, false
	}

	r.pos += n

	size, sn := readEbmlSize(r.data[r.pos:])
	if sn == 0 {
		return 0, nil, false
	}

	r.pos += sn

	if size > uint64(len(r.data)-r.pos) {
		size = uint64(len(r.data) - r.pos) // tolerate truncated tail
	}

	content = r.data[r.pos : r.pos+int(size)]

	r.pos += int(size)

	return id, content, true
}

// readEbmlID reads an EBML element ID keeping its length-marker bits.
func readEbmlID(b []byte) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}

	length := ebmlVintLength(b[0])
	if length == 0 || length > 4 || length > len(b) {
		return 0, 0
	}

	var v uint64
	for i := range length {
		v = v<<8 | uint64(b[i])
	}

	return v, length
}

// readEbmlSize reads an EBML size, stripping the length-marker bit.
func readEbmlSize(b []byte) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}

	length := ebmlVintLength(b[0])
	if length == 0 || length > 8 || length > len(b) {
		return 0, 0
	}

	v := uint64(b[0] & (0xFF >> length))
	for i := 1; i < length; i++ {
		v = v<<8 | uint64(b[i])
	}

	return v, length
}

func ebmlVintLength(first byte) int {
	for i := range 8 {
		if first&(0x80>>i) != 0 {
			return i + 1
		}
	}

	return 0
}

func ebmlUint(b []byte) uint64 {
	var v uint64
	for i := range b {
		v = v<<8 | uint64(b[i])
	}

	return v
}

func ebmlFloat(b []byte) float64 {
	switch len(b) {
	case 4:
		return float64(math.Float32frombits(uint32(ebmlUint(b))))
	case 8:
		return math.Float64frombits(ebmlUint(b))
	}

	return 0
}

func probeMKV(f *os.File) (*ffProbeJSON, error) {
	head, err := io.ReadAll(io.LimitReader(f, maxHeaderProbe))
	if err != nil {
		return nil, err
	}

	r := &ebmlReader{data: head}

	// Top level: EBML header then Segment.
	var segment []byte

	for {
		id, content, ok := r.elem()
		if !ok {
			break
		}

		if id == ebmlSegment {
			segment = content
			break
		}
	}

	if segment == nil {
		return nil, errNativeUnsupported
	}

	out := &ffProbeJSON{}

	var (
		timecodeScale uint64 = 1_000_000 // default 1ms
		durationTicks float64
	)

	seg := &ebmlReader{data: segment}
	for {
		id, content, ok := seg.elem()
		if !ok {
			break
		}

		switch id {
		case ebmlInfo:
			timecodeScale, durationTicks = parseMKVInfo(content, timecodeScale)
		case ebmlTracks:
			out.Streams = parseMKVTracks(content)
		case ebmlCluster:
			// Media data begins; header parsing is complete.
			goto done
		}
	}

done:
	if durationTicks > 0 {
		secs := durationTicks * float64(timecodeScale) / 1e9

		out.Format.Duration = strconv.FormatFloat(secs, 'f', 3, 64)
	}

	if !hasUsableVideo(out.Streams) {
		return nil, errNativeUnsupported
	}

	return out, nil
}

func parseMKVInfo(content []byte, defScale uint64) (uint64, float64) {
	scale := defScale

	var dur float64

	r := &ebmlReader{data: content}
	for {
		id, c, ok := r.elem()
		if !ok {
			break
		}

		switch id {
		case ebmlTimecodeScale:
			if v := ebmlUint(c); v > 0 {
				scale = v
			}

		case ebmlDuration:
			dur = ebmlFloat(c)
		}
	}

	return scale, dur
}

func parseMKVTracks(content []byte) []ffProbeStream {
	var streams []ffProbeStream

	r := &ebmlReader{data: content}
	for {
		id, c, ok := r.elem()
		if !ok {
			break
		}

		if id == ebmlTrackEntry {
			if st, ok := parseMKVTrackEntry(c); ok {
				streams = append(streams, st)
			}
		}
	}

	return streams
}

func parseMKVTrackEntry(content []byte) (ffProbeStream, bool) {
	var (
		trackType uint64
		codecID   string
		lang      string // only set from an explicit Language element (matches ffprobe)
		width     int
		height    int
		channels  int
		freq      int
	)

	r := &ebmlReader{data: content}
	for {
		id, c, ok := r.elem()
		if !ok {
			break
		}

		switch id {
		case ebmlTrackType:
			trackType = ebmlUint(c)
		case ebmlCodecID:
			codecID = string(c)
		case ebmlLanguage:
			lang = string(c)
		case ebmlVideo:
			width, height = parseMKVVideo(c)
		case ebmlAudio:
			channels, freq = parseMKVAudio(c)
		}
	}

	switch trackType {
	case 1: // video
		st := langStream("video", mkvVideoCodec(codecID), lang)

		st.Width = width
		st.Height = height

		return st, true

	case 2: // audio
		st := langStream("audio", mkvAudioCodec(codecID), lang)

		st.Channels = channels

		if freq > 0 {
			st.SampleRate = strconv.Itoa(freq)
		}

		return st, true
	}

	return ffProbeStream{}, false
}

func parseMKVVideo(content []byte) (int, int) {
	var w, h int

	r := &ebmlReader{data: content}
	for {
		id, c, ok := r.elem()
		if !ok {
			break
		}

		switch id {
		case ebmlPixelWidth:
			w = int(ebmlUint(c))
		case ebmlPixelHeight:
			h = int(ebmlUint(c))
		}
	}

	return w, h
}

func parseMKVAudio(content []byte) (int, int) {
	var ch, freq int

	r := &ebmlReader{data: content}
	for {
		id, c, ok := r.elem()
		if !ok {
			break
		}

		switch id {
		case ebmlChannels:
			ch = int(ebmlUint(c))
		case ebmlSampleFreq:
			freq = int(ebmlFloat(c))
		}
	}

	return ch, freq
}

func mkvVideoCodec(id string) string {
	switch {
	case strings.HasPrefix(id, "V_MPEG4/ISO/AVC"):
		return "h264"
	case strings.HasPrefix(id, "V_MPEGH/ISO/HEVC"):
		return "hevc"
	case strings.HasPrefix(id, "V_MPEG4/ISO"):
		return "mpeg4"
	case strings.HasPrefix(id, "V_MPEG4/MS"):
		return "msmpeg4"
	case id == "V_VP9":
		return "vp9"
	case id == "V_VP8":
		return "vp8"
	case id == "V_AV1":
		return "av1"
	case strings.HasPrefix(id, "V_MPEG2"):
		return "mpeg2"
	}

	return ""
}

func mkvAudioCodec(id string) string {
	switch {
	case strings.HasPrefix(id, "A_AAC"):
		return "aac"
	case id == "A_AC3":
		return "ac3"
	case id == "A_EAC3":
		return "eac3"
	case strings.HasPrefix(id, "A_DTS"):
		return "dts"
	case id == "A_MPEG/L3":
		return "mp3"
	case id == "A_MPEG/L2":
		return "mp2"
	case id == "A_FLAC":
		return "flac"
	case id == "A_OPUS":
		return "opus"
	case id == "A_VORBIS":
		return "vorbis"
	case id == "A_TRUEHD":
		return "truehd"
	case strings.HasPrefix(id, "A_PCM"):
		return "pcm"
	}

	return ""
}

// ---------------------------------------------------------------------------
// AVI (RIFF)
// ---------------------------------------------------------------------------

func probeAVI(f *os.File) (*ffProbeJSON, error) {
	head, err := io.ReadAll(io.LimitReader(f, maxHeaderProbe))
	if err != nil {
		return nil, err
	}

	if len(head) < 12 || string(head[0:4]) != "RIFF" || string(head[8:12]) != "AVI " {
		return nil, errNativeUnsupported
	}

	// Find the hdrl LIST.
	hdrl := findRiffList(head[12:], "hdrl")
	if hdrl == nil {
		return nil, errNativeUnsupported
	}

	out := &ffProbeJSON{}

	var microPerFrame, totalFrames uint32

	// hdrl: avih chunk + one strl LIST per stream.
	walkRiff(hdrl, func(fourcc string, body []byte) {
		switch fourcc {
		case "avih":
			if len(body) >= 20 {
				microPerFrame = binary.LittleEndian.Uint32(body[0:4])
				totalFrames = binary.LittleEndian.Uint32(body[16:20])
			}

		case "LIST":
			if len(body) >= 4 && string(body[0:4]) == "strl" {
				if st, ok := parseAVIStream(body[4:]); ok {
					out.Streams = append(out.Streams, st)
				}
			}
		}
	})

	if microPerFrame > 0 && totalFrames > 0 {
		secs := float64(totalFrames) * float64(microPerFrame) / 1e6

		out.Format.Duration = strconv.FormatFloat(secs, 'f', 3, 64)
	}

	if !hasUsableVideo(out.Streams) {
		return nil, errNativeUnsupported
	}

	return out, nil
}

// walkRiff iterates RIFF chunks in data, calling fn with each chunk's fourcc and
// body. For a LIST chunk the body includes the list type fourcc as its prefix.
func walkRiff(data []byte, fn func(fourcc string, body []byte)) {
	for off := 0; off+8 <= len(data); {
		fourcc := string(data[off : off+4])
		size := int(binary.LittleEndian.Uint32(data[off+4 : off+8]))

		start := off + 8
		if size < 0 || start+size > len(data) {
			return
		}

		fn(fourcc, data[start:start+size])

		off = start + size
		if size%2 == 1 { // chunks are word-aligned
			off++
		}
	}
}

func findRiffList(data []byte, listType string) []byte {
	var found []byte

	walkRiff(data, func(fourcc string, body []byte) {
		if found == nil && fourcc == "LIST" && len(body) >= 4 && string(body[0:4]) == listType {
			found = body[4:]
		}
	})

	return found
}

// parseAVIStream reads a strl list (strh + strf chunks) into a stream entry.
func parseAVIStream(strl []byte) (ffProbeStream, bool) {
	var (
		streamType string
		strf       []byte
	)

	walkRiff(strl, func(fourcc string, body []byte) {
		switch fourcc {
		case "strh":
			if len(body) >= 4 {
				streamType = string(body[0:4]) // 'vids' / 'auds'
			}

		case "strf":
			strf = body
		}
	})

	switch streamType {
	case "vids":
		st := ffProbeStream{CodecType: "video"}
		if len(strf) >= 20 { // BITMAPINFOHEADER
			st.Width = int(int32(binary.LittleEndian.Uint32(strf[4:8])))
			st.Height = abs32(int32(binary.LittleEndian.Uint32(strf[8:12])))
			st.CodecName = aviVideoCodec(string(strf[16:20]))
		}

		return st, true

	case "auds":
		st := ffProbeStream{CodecType: "audio"}
		if len(strf) >= 16 { // WAVEFORMATEX
			st.CodecName = aviAudioCodec(binary.LittleEndian.Uint16(strf[0:2]))
			st.Channels = int(binary.LittleEndian.Uint16(strf[2:4]))
			st.SampleRate = strconv.Itoa(int(binary.LittleEndian.Uint32(strf[4:8])))
		}

		return st, true
	}

	return ffProbeStream{}, false
}

func abs32(v int32) int {
	if v < 0 {
		return int(-v)
	}

	return int(v)
}

func aviVideoCodec(fourcc string) string {
	switch strings.ToUpper(strings.TrimSpace(fourcc)) {
	case "H264", "X264", "AVC1", "DAVC", "VSSH":
		return "h264"
	case "HEVC", "H265", "HVC1", "X265", "HEV1":
		return "hevc"
	case "XVID":
		return "xvid"
	case "DIVX", "DX50", "DIV3", "DIV4", "DIV5", "DIV6", "DIVF":
		return "divx"
	case "MP42", "MP43", "MPG4", "MP41":
		return "msmpeg4"
	case "FMP4", "MP4V":
		return "mpeg4"
	case "VP90", "VP9":
		return "vp9"
	case "AV01":
		return "av1"
	case "MPG2", "MPEG", "MPG1", "MP2V":
		return "mpeg2"
	case "WVC1", "VC-1", "WMV3":
		return "vc1"
	}

	return strings.ToLower(strings.TrimSpace(fourcc))
}

func aviAudioCodec(tag uint16) string {
	switch tag {
	case 0x0055:
		return "mp3"
	case 0x2000, 0x2001:
		return "ac3"
	case 0x0001:
		return "pcm"
	case 0x00FF, 0x1600, 0x1601, 0x1602, 0x1610:
		return "aac"
	case 0x000A, 0x0161, 0x0162, 0x0163:
		return "wmav2"
	}

	return ""
}
