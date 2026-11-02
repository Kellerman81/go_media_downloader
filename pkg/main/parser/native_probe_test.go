package parser

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Real sample files supplied for verification. Tests skip when absent.
const (
	sampleAVI  = `C:\Users\mkell\Downloads\Free_Test_Data_5MB_AVI.avi`
	sampleMP4  = `C:\Users\mkell\Downloads\Free_Test_Data_7MB_MP4.mp4`
	sampleMKV  = `C:\Users\mkell\Downloads\Free_Test_Data_3.67_WKV.mkv`
	sampleMPEG = `C:\Users\mkell\Downloads\FTD_5MB.mpeg`
)

func dumpStreams(t *testing.T, r *ffProbeJSON) {
	t.Helper()
	t.Logf("duration=%q", r.Format.Duration)

	for i := range r.Streams {
		s := &r.Streams[i]
		t.Logf("  %-5s codec=%-12s %dx%d ch=%d rate=%s lang=%q",
			s.CodecType, s.CodecName, s.Width, s.Height, s.Channels, s.SampleRate, s.language())
	}
}

func wantVideo(t *testing.T, r *ffProbeJSON, codec string, w, h int, dur float64) {
	t.Helper()

	var v *ffProbeStream

	for i := range r.Streams {
		if r.Streams[i].CodecType == "video" {
			v = &r.Streams[i]
			break
		}
	}

	if v == nil {
		t.Fatal("no video stream")
	}

	if v.CodecName != codec {
		t.Errorf("video codec = %q, want %q", v.CodecName, codec)
	}

	if v.Width != w || v.Height != h {
		t.Errorf("resolution = %dx%d, want %dx%d", v.Width, v.Height, w, h)
	}

	got, err := strconv.ParseFloat(r.Format.Duration, 64)
	if err != nil {
		t.Errorf("duration %q not numeric: %v", r.Format.Duration, err)
	} else if math.Abs(got-dur) > 1.0 {
		t.Errorf("duration = %.3f, want ~%.3f", got, dur)
	}
}

func TestNativeProbeAVI(t *testing.T) {
	if _, err := os.Stat(sampleAVI); err != nil {
		t.Skip("sample AVI not available")
	}

	r, err := nativeProbe(sampleAVI)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 960, 540, 55.121733)
}

func TestNativeProbeMP4(t *testing.T) {
	if _, err := os.Stat(sampleMP4); err != nil {
		t.Skip("sample MP4 not available")
	}

	r, err := nativeProbe(sampleMP4)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 960, 540, 55.121733)
}

func TestNativeProbeMKV(t *testing.T) {
	if _, err := os.Stat(sampleMKV); err != nil {
		t.Skip("sample MKV not available")
	}

	r, err := nativeProbe(sampleMKV)
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	dumpStreams(t, r)
	wantVideo(t, r, "h264", 1280, 720, 23.789)

	// Audio stream present with codec + channels.
	var a *ffProbeStream

	for i := range r.Streams {
		if r.Streams[i].CodecType == "audio" {
			a = &r.Streams[i]
			break
		}
	}

	if a == nil {
		t.Fatal("no audio stream")
	}

	if a.CodecName != "aac" || a.Channels != 2 {
		t.Errorf("audio = %q/%dch, want aac/2ch", a.CodecName, a.Channels)
	}
}

// TestNativeProbeMPEG confirms MPEG-PS is not claimed by the native prober, so
// it falls back to ffprobe.
func TestNativeProbeMPEG(t *testing.T) {
	if _, err := os.Stat(sampleMPEG); err != nil {
		t.Skip("sample MPEG not available")
	}

	if _, err := nativeProbe(sampleMPEG); err == nil {
		t.Error("expected native probe to decline MPEG-PS (so ffprobe handles it)")
	}
}

// Synthetic container headers, built byte by byte, so the native prober is
// exercised without the sample files TestNativeProbe* depend on.

func be16(v uint16) []byte { return binary.BigEndian.AppendUint16(nil, v) }

func be32(v uint32) []byte { return binary.BigEndian.AppendUint32(nil, v) }

func le16(v uint16) []byte { return binary.LittleEndian.AppendUint16(nil, v) }

func le32(v uint32) []byte { return binary.LittleEndian.AppendUint32(nil, v) }

func cat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}

	return out
}

func mkBox(typ string, body ...[]byte) []byte {
	b := cat(body...)
	return cat(be32(uint32(8+len(b))), []byte(typ), b)
}

// packMdhdLang packs a 3-letter ISO-639-2 code the way mdhd stores it.
func packMdhdLang(lang string) []byte {
	return be16(uint16(lang[0]-0x60)<<10 | uint16(lang[1]-0x60)<<5 | uint16(lang[2]-0x60))
}

func mp4Trak(handler, lang string, entry []byte) []byte {
	hdlr := cat(make([]byte, 8), []byte(handler), make([]byte, 12))
	mdhd := cat(make([]byte, 20), packMdhdLang(lang), make([]byte, 2))
	stsd := cat(make([]byte, 4), be32(1), entry)

	return mkBox("trak", mkBox("mdia",
		mkBox("hdlr", hdlr),
		mkBox("mdhd", mdhd),
		mkBox("minf", mkBox("stbl", mkBox("stsd", stsd))),
	))
}

func buildMP4() []byte {
	video := make([]byte, 78)
	copy(video[24:], be16(1920))
	copy(video[26:], be16(800))

	audio := make([]byte, 28)
	copy(audio[16:], be16(6))
	copy(audio[24:], be32(48000<<16))

	mvhd := cat(make([]byte, 12), be32(1000), be32(5_400_500), make([]byte, 80))

	return cat(
		mkBox("ftyp", []byte("isom"), be32(0x200), []byte("isomavc1")),
		mkBox("mdat", make([]byte, 4096)), // skipped by seeking, not read
		mkBox("moov",
			mkBox("mvhd", mvhd),
			mp4Trak("vide", "und", mkBox("avc1", video)),
			mp4Trak("soun", "ger", mkBox("ec-3", audio)),
		),
	)
}

func ebmlElem(id uint32, content ...[]byte) []byte {
	c := cat(content...)

	var idBytes []byte

	switch {
	case id > 0xFFFFFF:
		idBytes = be32(id)
	case id > 0xFFFF:
		idBytes = be32(id)[1:]
	case id > 0xFF:
		idBytes = be16(uint16(id))
	default:
		idBytes = []byte{byte(id)}
	}

	// 8-byte size vint: marker 0x01 followed by 7 length bytes.
	size := binary.BigEndian.AppendUint64(nil, uint64(len(c)))
	size[0] = 0x01

	return cat(idBytes, size, c)
}

func ebmlU(v uint64) []byte {
	return binary.BigEndian.AppendUint64(nil, v)
}

func ebmlF64(v float64) []byte { return binary.BigEndian.AppendUint64(nil, math.Float64bits(v)) }

func ebmlF32(v float32) []byte { return binary.BigEndian.AppendUint32(nil, math.Float32bits(v)) }

func buildMKV() []byte {
	return cat(
		ebmlElem(0x1A45DFA3, ebmlElem(0x4282, []byte("matroska"))),
		ebmlElem(ebmlSegment,
			ebmlElem(ebmlInfo,
				ebmlElem(ebmlTimecodeScale, ebmlU(1_000_000)),
				ebmlElem(ebmlDuration, ebmlF64(7_200_000)),
			),
			ebmlElem(ebmlTracks,
				ebmlElem(ebmlTrackEntry,
					ebmlElem(ebmlTrackType, ebmlU(1)),
					ebmlElem(ebmlCodecID, []byte("V_MPEGH/ISO/HEVC")),
					ebmlElem(ebmlVideo,
						ebmlElem(ebmlPixelWidth, ebmlU(3840)),
						ebmlElem(ebmlPixelHeight, ebmlU(2160)),
					),
				),
				ebmlElem(ebmlTrackEntry,
					ebmlElem(ebmlTrackType, ebmlU(2)),
					ebmlElem(ebmlCodecID, []byte("A_EAC3")),
					ebmlElem(ebmlLanguage, []byte("ger")),
					ebmlElem(ebmlAudio,
						ebmlElem(ebmlChannels, ebmlU(6)),
						ebmlElem(ebmlSampleFreq, ebmlF32(48000)),
					),
				),
				// Subtitle tracks are ignored.
				ebmlElem(ebmlTrackEntry, ebmlElem(ebmlTrackType, ebmlU(17))),
			),
			ebmlElem(ebmlCluster, make([]byte, 64)),
			// Anything after the first cluster must not be parsed.
			ebmlElem(ebmlTracks, ebmlElem(ebmlTrackEntry, ebmlElem(ebmlTrackType, ebmlU(1)))),
		),
	)
}

func riffChunk(fourcc string, body ...[]byte) []byte {
	b := cat(body...)
	out := cat([]byte(fourcc), le32(uint32(len(b))), b)

	if len(b)%2 == 1 {
		out = append(out, 0)
	}

	return out
}

func buildAVI() []byte {
	avih := cat(le32(40_000), make([]byte, 12), le32(250), make([]byte, 36)) // 25 fps, 250 frames

	bih := cat(le32(40), le32(720), le32(uint32(0xFFFFFE20)), make([]byte, 4), []byte("XVID"), make([]byte, 20)) // height -480
	wfx := cat(le16(0x2000), le16(6), le32(48000), make([]byte, 8))

	hdrl := riffChunk("LIST", []byte("hdrl"),
		riffChunk("avih", avih),
		riffChunk("LIST", []byte("strl"), riffChunk("strh", []byte("vids"), make([]byte, 52)), riffChunk("strf", bih)),
		riffChunk("LIST", []byte("strl"), riffChunk("strh", []byte("auds"), make([]byte, 52)), riffChunk("strf", wfx)),
		riffChunk("JUNK", []byte{1, 2, 3}), // odd size exercises word alignment
	)

	body := cat([]byte("AVI "), hdrl, riffChunk("LIST", []byte("movi")))

	return cat([]byte("RIFF"), le32(uint32(len(body))), body)
}

func writeSample(t *testing.T, name string, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func streamsByType(r *ffProbeJSON) (video, audio *ffProbeStream) {
	for i := range r.Streams {
		switch r.Streams[i].CodecType {
		case "video":
			video = &r.Streams[i]
		case "audio":
			audio = &r.Streams[i]
		}
	}

	return video, audio
}

func TestNativeProbeSyntheticMP4(t *testing.T) {
	r, err := nativeProbe(writeSample(t, "movie.mp4", buildMP4()))
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	if r.Format.Duration != "5400.500" {
		t.Errorf("duration = %q", r.Format.Duration)
	}

	v, a := streamsByType(r)
	if v == nil || v.CodecName != "h264" || v.Width != 1920 || v.Height != 800 || v.language() != "" {
		t.Errorf("video = %+v", v)
	}

	if a == nil || a.CodecName != "eac3" || a.Channels != 6 || a.SampleRate != "48000" || a.language() != "ger" {
		t.Errorf("audio = %+v", a)
	}
}

func TestNativeProbeSyntheticMKV(t *testing.T) {
	r, err := nativeProbe(writeSample(t, "movie.mkv", buildMKV()))
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	if r.Format.Duration != "7200.000" {
		t.Errorf("duration = %q", r.Format.Duration)
	}

	if len(r.Streams) != 2 {
		t.Fatalf("streams = %d, want 2 (subtitles and post-cluster data ignored)", len(r.Streams))
	}

	v, a := streamsByType(r)
	if v == nil || v.CodecName != "hevc" || v.Width != 3840 || v.Height != 2160 {
		t.Errorf("video = %+v", v)
	}

	if a == nil || a.CodecName != "eac3" || a.Channels != 6 || a.SampleRate != "48000" || a.language() != "ger" {
		t.Errorf("audio = %+v", a)
	}
}

func TestNativeProbeSyntheticAVI(t *testing.T) {
	r, err := nativeProbe(writeSample(t, "movie.avi", buildAVI()))
	if err != nil {
		t.Fatalf("nativeProbe: %v", err)
	}

	if r.Format.Duration != "10.000" {
		t.Errorf("duration = %q", r.Format.Duration)
	}

	v, a := streamsByType(r)
	if v == nil || v.CodecName != "xvid" || v.Width != 720 || v.Height != 480 {
		t.Errorf("video = %+v", v)
	}

	if a == nil || a.CodecName != "ac3" || a.Channels != 6 || a.SampleRate != "48000" {
		t.Errorf("audio = %+v", a)
	}
}

func TestNativeProbeRejectsBadInput(t *testing.T) {
	mp4 := buildMP4()
	mkv := buildMKV()
	avi := buildAVI()

	cases := map[string][]byte{
		"tiny.mp4":      []byte("short"),
		"unknown.mp4":   make([]byte, 64),
		"truncated.mp4": mp4[:len(mp4)-200],
		"nomoov.mp4":    mkBox("ftyp", []byte("isom"), make([]byte, 8)),
		"noseg.mkv":     mkv[:20],
		"nohdrl.avi":    cat([]byte("RIFF"), le32(4), []byte("AVI ")),
		"noaudio.avi":   avi[:12], // header only
	}

	for name, data := range cases {
		if _, err := nativeProbe(writeSample(t, name, data)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}

	if _, err := nativeProbe(filepath.Join(t.TempDir(), "missing.mkv")); err == nil {
		t.Error("missing file: expected an error")
	}
}

func TestMP4Boxes(t *testing.T) {
	data := cat(mkBox("free", []byte("ab")), mkBox("skip"))

	boxes := mp4Boxes(data)
	if len(boxes) != 2 || boxes[0].typ != "free" || string(boxes[0].body) != "ab" || boxes[1].typ != "skip" {
		t.Errorf("boxes = %+v", boxes)
	}

	// 64-bit "largesize" header.
	large := cat(be32(1), []byte("wide"), binary.BigEndian.AppendUint64(nil, 19), []byte("xyz"))
	if boxes := mp4Boxes(large); len(boxes) != 1 || string(boxes[0].body) != "xyz" {
		t.Errorf("largesize boxes = %+v", boxes)
	}

	// Sizes pointing past the end stop the walk instead of panicking.
	for _, bad := range [][]byte{
		cat(be32(100), []byte("trak")),
		cat(be32(4), []byte("trak")),
		cat(be32(1), []byte("wide"), binary.BigEndian.AppendUint64(nil, 99)),
		cat(be32(1), []byte("wide")),
	} {
		if boxes := mp4Boxes(bad); len(boxes) != 0 {
			t.Errorf("mp4Boxes(%x) = %+v, want none", bad, boxes)
		}
	}
}

func TestParseMvhd(t *testing.T) {
	v0 := cat(make([]byte, 12), be32(600), be32(1200))
	if ts, d := parseMvhd(v0); ts != 600 || d != 1200 {
		t.Errorf("v0 = %d, %d", ts, d)
	}

	v1 := cat([]byte{1}, make([]byte, 19), be32(90000), binary.BigEndian.AppendUint64(nil, 1<<33))
	if ts, d := parseMvhd(v1); ts != 90000 || d != 1<<33 {
		t.Errorf("v1 = %d, %d", ts, d)
	}

	for _, short := range [][]byte{nil, {0}, {1, 0, 0}} {
		if ts, d := parseMvhd(short); ts != 0 || d != 0 {
			t.Errorf("parseMvhd(%x) = %d, %d", short, ts, d)
		}
	}
}

func TestParseMdhdLanguage(t *testing.T) {
	v0 := cat(make([]byte, 20), packMdhdLang("eng"))
	if got := parseMdhdLanguage(v0); got != "eng" {
		t.Errorf("v0 = %q", got)
	}

	v1 := cat([]byte{1}, make([]byte, 31), packMdhdLang("fra"))
	if got := parseMdhdLanguage(v1); got != "fra" {
		t.Errorf("v1 = %q", got)
	}

	if got := parseMdhdLanguage(cat(make([]byte, 20), be16(0))); got != "" {
		t.Errorf("zero language = %q", got)
	}

	if got := parseMdhdLanguage(make([]byte, 10)); got != "" {
		t.Errorf("short box = %q", got)
	}
}

func TestEbmlVints(t *testing.T) {
	ids := []struct {
		in   []byte
		id   uint64
		size int
	}{
		{[]byte{0xAE}, 0xAE, 1},
		{[]byte{0x42, 0x86}, 0x4286, 2},
		{[]byte{0x2A, 0xD7, 0xB1}, 0x2AD7B1, 3},
		{[]byte{0x18, 0x53, 0x80, 0x67}, 0x18538067, 4},
		{[]byte{0x08, 0, 0, 0, 0}, 0, 0}, // 5-byte IDs are invalid
		{[]byte{0x42}, 0, 0},             // truncated
		{nil, 0, 0},
	}

	for _, tt := range ids {
		if id, n := readEbmlID(tt.in); id != tt.id || n != tt.size {
			t.Errorf("readEbmlID(%x) = %x, %d, want %x, %d", tt.in, id, n, tt.id, tt.size)
		}
	}

	sizes := []struct {
		in   []byte
		v    uint64
		size int
	}{
		{[]byte{0x85}, 5, 1},
		{[]byte{0x40, 0x02}, 2, 2},
		{[]byte{0x01, 0, 0, 0, 0, 0, 1, 0}, 256, 8},
		{[]byte{0x00}, 0, 0},
		{[]byte{0x40}, 0, 0},
	}

	for _, tt := range sizes {
		if v, n := readEbmlSize(tt.in); v != tt.v || n != tt.size {
			t.Errorf("readEbmlSize(%x) = %d, %d, want %d, %d", tt.in, v, n, tt.v, tt.size)
		}
	}

	if got := ebmlFloat(ebmlF32(1.5)); got != 1.5 {
		t.Errorf("float32 = %v", got)
	}

	if got := ebmlFloat([]byte{1, 2}); got != 0 {
		t.Errorf("2-byte float = %v", got)
	}
}

func TestEbmlReaderTruncatedTail(t *testing.T) {
	// Declared size 10, only 3 bytes present: content is clipped, not a panic.
	r := &ebmlReader{data: []byte{0xAE, 0x8A, 1, 2, 3}}

	id, c, ok := r.elem()
	if !ok || id != 0xAE || len(c) != 3 {
		t.Errorf("elem = %x, %x, %t", id, c, ok)
	}

	if _, _, ok := r.elem(); ok {
		t.Error("expected end of data")
	}
}

func TestCodecMaps(t *testing.T) {
	mp4 := map[string]string{"avc3": "h264", "hev1": "hevc", "vp09": "vp9", "av01": "av1", "mp4v": "mpeg4", "xyz ": "xyz"}
	for in, want := range mp4 {
		if got := mp4VideoCodec(in); got != want {
			t.Errorf("mp4VideoCodec(%q) = %q, want %q", in, got, want)
		}
	}

	mp4a := map[string]string{"mp4a": "aac", "ac-3": "ac3", "dtsh": "dts", ".mp3": "mp3", "fLaC": "flac", "Opus": "opus", "abc ": "abc"}
	for in, want := range mp4a {
		if got := mp4AudioCodec(in); got != want {
			t.Errorf("mp4AudioCodec(%q) = %q, want %q", in, got, want)
		}
	}

	mkv := map[string]string{
		"V_MPEG4/ISO/AVC": "h264", "V_MPEG4/ISO/ASP": "mpeg4", "V_MPEG4/MS/V3": "msmpeg4",
		"V_VP9": "vp9", "V_VP8": "vp8", "V_AV1": "av1", "V_MPEG2": "mpeg2", "V_THEORA": "",
	}
	for in, want := range mkv {
		if got := mkvVideoCodec(in); got != want {
			t.Errorf("mkvVideoCodec(%q) = %q, want %q", in, got, want)
		}
	}

	mkva := map[string]string{
		"A_AAC/MPEG4/LC": "aac", "A_AC3": "ac3", "A_DTS/MA": "dts", "A_MPEG/L3": "mp3", "A_MPEG/L2": "mp2",
		"A_FLAC": "flac", "A_OPUS": "opus", "A_VORBIS": "vorbis", "A_TRUEHD": "truehd", "A_PCM/INT/LIT": "pcm", "A_MS/ACM": "",
	}
	for in, want := range mkva {
		if got := mkvAudioCodec(in); got != want {
			t.Errorf("mkvAudioCodec(%q) = %q, want %q", in, got, want)
		}
	}

	avi := map[string]string{"x264": "h264", "HEVC": "hevc", "DX50": "divx", "MP43": "msmpeg4", "FMP4": "mpeg4", "WVC1": "vc1", "MJPG": "mjpg"}
	for in, want := range avi {
		if got := aviVideoCodec(in); got != want {
			t.Errorf("aviVideoCodec(%q) = %q, want %q", in, got, want)
		}
	}

	avia := map[uint16]string{0x0055: "mp3", 0x2001: "ac3", 0x0001: "pcm", 0x00FF: "aac", 0x0161: "wmav2", 0x9999: ""}
	for in, want := range avia {
		if got := aviAudioCodec(in); got != want {
			t.Errorf("aviAudioCodec(%#x) = %q, want %q", in, got, want)
		}
	}
}

func TestHasUsableVideo(t *testing.T) {
	cases := []struct {
		streams []ffProbeStream
		want    bool
	}{
		{nil, false},
		{[]ffProbeStream{{CodecType: "audio", CodecName: "aac"}}, false},
		{[]ffProbeStream{{CodecType: "video", CodecName: "h264"}}, false},
		{[]ffProbeStream{{CodecType: "video", Height: 720}}, false},
		{[]ffProbeStream{{CodecType: "Video", CodecName: "h264", Height: 720}}, true},
	}

	for _, tt := range cases {
		if got := hasUsableVideo(tt.streams); got != tt.want {
			t.Errorf("hasUsableVideo(%+v) = %t, want %t", tt.streams, got, tt.want)
		}
	}

	if s := langStream("audio", "aac", "UND"); s.Tags != nil {
		t.Errorf("undetermined language tagged: %v", s.Tags)
	}
}

func TestNativeProbeSupportsExt(t *testing.T) {
	for in, want := range map[string]bool{"a.MKV": true, "a.mp4": true, "a.webm": true, "a.ts": false, "a": false} {
		if got := nativeProbeSupportsExt(in); got != want {
			t.Errorf("nativeProbeSupportsExt(%q) = %t, want %t", in, got, want)
		}
	}
}
