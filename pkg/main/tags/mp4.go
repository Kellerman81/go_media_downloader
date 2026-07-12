package tags

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// MP4Handler reads and writes metadata for MP4-family audio containers
// (.m4a, .m4b, .mp4 — including AAC and ALAC streams).
//
// Reading is native: the moov atom is parsed directly, so no external tool is
// required for the matching / enrichment hot-path. Writing shells out to ffmpeg
// (remux with -c copy), mirroring the FLAC handler's use of metaflac, because
// in-place MP4 atom rewriting requires chunk-offset patching that ffmpeg already
// does correctly and safely.
type MP4Handler struct{}

// NewMP4Handler creates a new MP4 tag handler.
func NewMP4Handler() *MP4Handler {
	return &MP4Handler{}
}

// SupportedFormats returns the file extensions supported by this handler.
func (*MP4Handler) SupportedFormats() []string {
	return []string{".m4a", ".m4b", ".mp4"}
}

// maxMoovBytes caps how much of a moov atom is read into memory, guarding against
// a corrupt/hostile size field. Real moov atoms are at most a few MB.
const maxMoovBytes = 256 << 20

// ReadTags reads metadata from an MP4 file, skipping embedded cover art.
func (h *MP4Handler) ReadTags(filepath string) (*AudioTags, error) {
	return h.readTags(filepath, false)
}

// ReadTagsWithCover reads metadata including embedded cover art (covr atom).
func (h *MP4Handler) ReadTagsWithCover(filepath string) (*AudioTags, error) {
	return h.readTags(filepath, true)
}

func (*MP4Handler) readTags(filepath string, withCover bool) (*AudioTags, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer f.Close()

	moov, err := readMoov(f)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}

	if moov == nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: "no moov atom found"}
	}

	tags := &AudioTags{}
	parseMoov(moov, tags, withCover)

	return tags, nil
}

// mp4box is one MP4 box (atom): its 4-char type and its body (children/payload).
type mp4box struct {
	typ  string
	body []byte
}

// mp4children splits an MP4 box body into its immediate child boxes. It tolerates
// truncation by stopping at the first malformed length.
func mp4children(data []byte) []mp4box {
	var out []mp4box

	for off := 0; off+8 <= len(data); {
		size := int(binary.BigEndian.Uint32(data[off : off+4]))
		typ := string(data[off+4 : off+8])

		switch {
		case size == 1: // 64-bit extended size
			if off+16 > len(data) {
				return out
			}

			sz64 := binary.BigEndian.Uint64(data[off+8 : off+16])
			if sz64 < 16 || sz64 > uint64(len(data)-off) {
				return out
			}

			size = int(sz64)
			out = append(out, mp4box{typ, data[off+16 : off+size]})

			off += size

		case size < 8 || off+size > len(data):
			return out

		default:
			out = append(out, mp4box{typ, data[off+8 : off+size]})

			off += size
		}
	}

	return out
}

// findChild returns the body of the first child box with the given type.
func findChild(boxes []mp4box, typ string) ([]byte, bool) {
	for i := range boxes {
		if boxes[i].typ == typ {
			return boxes[i].body, true
		}
	}

	return nil, false
}

// readMoov walks the top-level boxes of an MP4 file and returns the moov body,
// seeking past large boxes (mdat) without reading them. Returns nil if absent.
func readMoov(f *os.File) ([]byte, error) {
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
		case 1: // 64-bit size follows
			var ext [8]byte
			if _, err := io.ReadFull(f, ext[:]); err != nil {
				return nil, err
			}

			size = int64(binary.BigEndian.Uint64(ext[:])) //nolint:gosec // bounded below
			headerLen = 16

		case 0: // box extends to EOF
			if typ == "moov" {
				return io.ReadAll(io.LimitReader(f, maxMoovBytes))
			}

			return nil, nil
		}

		if size < headerLen {
			return nil, nil
		}

		bodyLen := size - headerLen

		if typ == "moov" {
			if bodyLen > maxMoovBytes {
				return nil, errors.New("moov atom too large")
			}

			body := make([]byte, bodyLen)
			if _, err := io.ReadFull(f, body); err != nil {
				return nil, err
			}

			return body, nil
		}

		if _, err := f.Seek(bodyLen, io.SeekCurrent); err != nil {
			return nil, err
		}
	}
}

// parseMoov extracts audio properties and metadata from a moov atom body.
func parseMoov(moov []byte, tags *AudioTags, withCover bool) {
	boxes := mp4children(moov)

	if b, ok := findChild(boxes, "mvhd"); ok {
		parseMvhd(b, tags)
	}

	for i := range boxes {
		if boxes[i].typ == "trak" {
			parseTrakAudio(boxes[i].body, tags)
			break
		}
	}

	// Metadata lives under udta > meta > ilst. meta is a full box (4-byte
	// version/flags before its children).
	udta, ok := findChild(boxes, "udta")
	if !ok {
		return
	}

	meta, ok := findChild(mp4children(udta), "meta")
	if !ok || len(meta) < 4 {
		return
	}

	if ilst, ok := findChild(mp4children(meta[4:]), "ilst"); ok {
		parseIlst(ilst, tags, withCover)
	}
}

// parseMvhd reads the movie header to derive duration.
func parseMvhd(b []byte, tags *AudioTags) {
	if len(b) < 1 {
		return
	}

	var timescale, duration uint64

	if b[0] == 1 { // version 1: 64-bit times
		if len(b) < 32 {
			return
		}

		timescale = uint64(binary.BigEndian.Uint32(b[20:24]))
		duration = binary.BigEndian.Uint64(b[24:32])
	} else {
		if len(b) < 20 {
			return
		}

		timescale = uint64(binary.BigEndian.Uint32(b[12:16]))
		duration = uint64(binary.BigEndian.Uint32(b[16:20]))
	}

	if timescale > 0 {
		tags.Duration = time.Duration(float64(duration) / float64(timescale) * float64(time.Second))
	}
}

// parseTrakAudio best-effort reads channel/sample-rate/bit-depth from the first
// audio sample entry (trak > mdia > minf > stbl > stsd).
func parseTrakAudio(trak []byte, tags *AudioTags) {
	mdia, ok := findChild(mp4children(trak), "mdia")
	if !ok {
		return
	}

	minf, ok := findChild(mp4children(mdia), "minf")
	if !ok {
		return
	}

	stbl, ok := findChild(mp4children(minf), "stbl")
	if !ok {
		return
	}

	stsd, ok := findChild(mp4children(stbl), "stsd")
	if !ok || len(stsd) < 8 {
		return
	}

	// stsd: 4-byte version/flags + 4-byte entry count, then sample entry boxes.
	entries := mp4children(stsd[8:])
	if len(entries) == 0 {
		return
	}

	se := entries[0].body
	// AudioSampleEntry: 6 reserved, 2 data-ref, 8 reserved, 2 channels, 2 sample
	// size, 2 predefined, 2 reserved, 4 sample rate (16.16 fixed point).
	if len(se) >= 28 {
		tags.Channels = int(binary.BigEndian.Uint16(se[16:18]))
		tags.BitDepth = int(binary.BigEndian.Uint16(se[18:20]))
		tags.SampleRate = int(binary.BigEndian.Uint32(se[24:28]) >> 16)
	}
}

// parseIlst walks the iTunes metadata item list.
func parseIlst(ilst []byte, tags *AudioTags, withCover bool) {
	for _, e := range mp4children(ilst) {
		if e.typ == "----" {
			parseFreeform(e.body, tags)
			continue
		}

		value, dataType, ok := ilstValue(e.body)
		if !ok {
			continue
		}

		switch e.typ {
		case "\xa9nam":
			tags.Title = string(value)
		case "\xa9ART":
			tags.Artist = string(value)
		case "aART":
			tags.AlbumArtist = string(value)
		case "\xa9alb":
			tags.Album = string(value)
		case "\xa9gen":
			tags.Genre = string(value)
		case "\xa9day":
			tags.Year = parseYear(string(value))
		case "\xa9wrt":
			tags.Composer = string(value)
		case "\xa9cmt":
			tags.Comment = string(value)
		case "cprt":
			tags.Copyright = string(value)
		case "\xa9lyr":
			tags.Lyrics = string(value)
		case "trkn":
			if len(value) >= 6 {
				tags.TrackNumber = int(binary.BigEndian.Uint16(value[2:4]))
				tags.TotalTracks = int(binary.BigEndian.Uint16(value[4:6]))
			}

		case "disk":
			if len(value) >= 6 {
				tags.DiscNumber = int(binary.BigEndian.Uint16(value[2:4]))
				tags.TotalDiscs = int(binary.BigEndian.Uint16(value[4:6]))
			}

		case "covr":
			if withCover && len(value) > 0 {
				tags.CoverData = value

				switch dataType {
				case 13:
					tags.CoverMIME = "image/jpeg"
				case 14:
					tags.CoverMIME = "image/png"
				default:
					tags.CoverMIME = detectImageMIME(value)
				}
			}
		}
	}
}

// ilstValue extracts the payload of the 'data' sub-box of an ilst entry, along
// with its type indicator (1=UTF-8, 13=JPEG, 14=PNG, 0=binary, …).
func ilstValue(entryBody []byte) (value []byte, dataType uint32, ok bool) {
	for _, b := range mp4children(entryBody) {
		if b.typ != "data" || len(b.body) < 8 {
			continue
		}

		// data box body: 4-byte type indicator, 4-byte locale, then value.
		return b.body[8:], binary.BigEndian.Uint32(b.body[0:4]), true
	}

	return nil, 0, false
}

// parseFreeform handles "----" custom atoms (mean / name / data), used for
// MusicBrainz IDs, AcoustID, ReplayGain, catalog number, etc.
func parseFreeform(body []byte, tags *AudioTags) {
	var (
		name  string
		value []byte
	)

	for _, b := range mp4children(body) {
		switch b.typ {
		case "name":
			if len(b.body) >= 4 {
				name = string(b.body[4:])
			}

		case "data":
			if len(b.body) >= 8 {
				value = b.body[8:]
			}
		}
	}

	if name == "" || len(value) == 0 {
		return
	}

	sval := string(value)

	switch {
	case strings.EqualFold(name, "MusicBrainz Track Id"),
		strings.EqualFold(name, "MusicBrainz Recording Id"):
		tags.MBRecordingID = sval
	case strings.EqualFold(name, "MusicBrainz Album Id"),
		strings.EqualFold(name, "MusicBrainz Release Id"):
		tags.MBReleaseID = sval
	case strings.EqualFold(name, "MusicBrainz Release Group Id"):
		tags.MBReleaseGroupID = sval
	case strings.EqualFold(name, "MusicBrainz Artist Id"):
		tags.MBArtistID = sval
	case strings.EqualFold(name, "MusicBrainz Album Artist Id"):
		tags.MBAlbumArtistID = sval
	case strings.EqualFold(name, "Acoustid Id"):
		tags.AcoustID = sval
	case strings.EqualFold(name, "CATALOGNUMBER"):
		tags.CatalogNum = sval
	case strings.EqualFold(name, "LABEL"):
		tags.Label = sval
	case strings.EqualFold(name, "ISRC"):
		tags.ISRC = sval
	case strings.EqualFold(name, "replaygain_track_gain"):
		tags.ReplayGainTrack = parseReplayGain(sval)
	case strings.EqualFold(name, "replaygain_track_peak"):
		tags.ReplayGainTrackPeak = parseReplayGainPeak(sval)
	case strings.EqualFold(name, "replaygain_album_gain"):
		tags.ReplayGainAlbum = parseReplayGain(sval)
	case strings.EqualFold(name, "replaygain_album_peak"):
		tags.ReplayGainAlbumPeak = parseReplayGainPeak(sval)
	}
}

// WriteTags writes metadata to an MP4 file by remuxing with ffmpeg (-c copy).
// Streams and chapters are preserved; the original is atomically replaced on
// success. Requires ffmpeg to be available.
func (*MP4Handler) WriteTags(ctx context.Context, filepath string, tags *AudioTags) error {
	ffmpeg, err := resolveFFmpeg()
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	args := []string{"-y", "-loglevel", "error", "-i", filepath}

	var coverPath string

	if len(tags.CoverData) > 0 {
		if cp, cerr := writeTempCover(tags.CoverData); cerr == nil {
			coverPath = cp
			defer os.Remove(coverPath)

			args = append(args, "-i", coverPath)
		}
	}

	if coverPath != "" {
		args = append(args, "-map", "0:a", "-map", "1:v")
	} else {
		args = append(args, "-map", "0")
	}

	args = append(args, "-c", "copy", "-map_metadata", "0", "-map_chapters", "0")

	if coverPath != "" {
		args = append(args, "-disposition:v:0", "attached_pic")
	}

	addMeta := func(key, val string) {
		if val != "" {
			args = append(args, "-metadata", key+"="+val)
		}
	}

	addMeta("title", tags.Title)
	addMeta("artist", tags.Artist)
	addMeta("album", tags.Album)
	addMeta("album_artist", tags.AlbumArtist)
	addMeta("genre", tags.Genre)

	if tags.Year > 0 {
		addMeta("date", strconv.Itoa(tags.Year))
	}

	if tags.TrackNumber > 0 {
		if tags.TotalTracks > 0 {
			addMeta("track", fmt.Sprintf("%d/%d", tags.TrackNumber, tags.TotalTracks))
		} else {
			addMeta("track", strconv.Itoa(tags.TrackNumber))
		}
	}

	if tags.DiscNumber > 0 {
		if tags.TotalDiscs > 0 {
			addMeta("disc", fmt.Sprintf("%d/%d", tags.DiscNumber, tags.TotalDiscs))
		} else {
			addMeta("disc", strconv.Itoa(tags.DiscNumber))
		}
	}

	addMeta("composer", tags.Composer)
	addMeta("comment", tags.Comment)
	addMeta("copyright", tags.Copyright)
	addMeta("lyrics", tags.Lyrics)
	addMeta("ISRC", tags.ISRC)

	// Output to a temp file with the original extension so ffmpeg selects the
	// correct muxer, then atomically replace the original.
	ext := ".m4a"
	if i := strings.LastIndexByte(filepath, '.'); i >= 0 {
		ext = filepath[i:]
	}

	out := filepath + ".tagtmp" + ext

	args = append(args, out)

	cmd := exec.CommandContext(ctx, ffmpeg, args...)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(out)

		return &ErrWriteFailed{
			Path:   filepath,
			Reason: "ffmpeg: " + strings.TrimSpace(stderr.String()),
		}
	}

	if err := os.Rename(out, filepath); err != nil {
		os.Remove(out)

		return &ErrWriteFailed{Path: filepath, Reason: "replacing original: " + err.Error()}
	}

	return nil
}

// resolveFFmpeg locates an ffmpeg binary: first by deriving it from the
// configured ffprobe path, then from PATH.
func resolveFFmpeg() (string, error) {
	if p := config.GetSettingsGeneral().FfprobePath; p != "" {
		if cand := strings.Replace(p, "ffprobe", "ffmpeg", 1); cand != p {
			if _, err := exec.LookPath(cand); err == nil {
				return cand, nil
			}
		}
	}

	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return "ffmpeg", nil
	}

	return "", errors.New(
		"ffmpeg not found in PATH - install FFmpeg to enable MP4/M4A/M4B tagging",
	)
}

// writeTempCover writes cover bytes to a temp file and returns its path.
func writeTempCover(data []byte) (string, error) {
	ext := ".jpg"
	if detectImageMIME(data) == "image/png" {
		ext = ".png"
	}

	f, err := os.CreateTemp("", "mp4-cover-*"+ext)
	if err != nil {
		return "", err
	}

	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())

		return "", err
	}

	f.Close()

	return f.Name(), nil
}
