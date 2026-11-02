package tags

import (
	"bytes"
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
)

// mp3ReadFrames is the minimal frame set for the metadata read hot-path
// (matching / enrichment). It omits frames whose AudioTags fields no ReadTags
// caller consumes — USLT (lyrics, often several KB), COMM, TCOM, TPE3, TCOP — so
// the id3v2 parser streams past their bodies without allocating strings. This is
// the dominant allocation source when scanning large libraries. WriteTags and
// ReadTagsWithCover parse all frames.
var mp3ReadFrames = []string{
	"TIT2", "TPE1", "TALB", "TCON", "TYER", "TDRC",
	"TPE2", "TRCK", "TPOS", "TSRC", "TPUB",
	"TXXX", "UFID", "TLEN",
}

// MP3Handler handles reading and writing ID3v2 tags for MP3 files.
type MP3Handler struct{}

// NewMP3Handler creates a new MP3 tag handler.
func NewMP3Handler() *MP3Handler {
	return &MP3Handler{}
}

// SupportedFormats returns the file extensions supported by this handler.
func (*MP3Handler) SupportedFormats() []string {
	return []string{".mp3"}
}

// ReadTagsWithCover reads ID3v2 tags including embedded cover art (APIC).
// Use this only when cover art is needed (e.g. ReadCoverData); prefer ReadTags
// for metadata-only access since loading cover art is expensive.
func (h *MP3Handler) ReadTagsWithCover(filepath string) (*AudioTags, error) {
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true})
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer tag.Close()

	tags, err := h.extractTags(tag)
	if err != nil {
		return nil, err
	}

	if pictures := tag.GetFrames("APIC"); len(pictures) > 0 {
		if pic, ok := pictures[0].(id3v2.PictureFrame); ok {
			tags.CoverData = pic.Picture
			tags.CoverMIME = pic.MimeType
		}
	}

	return tags, nil
}

// ReadTags reads ID3v2 tags from an MP3 file, skipping embedded cover art.
// Cover art (APIC) is intentionally omitted — it can be several MB per file
// and is not needed for the matching / enrichment hot-path.
// Use ReadTagsWithCover when cover art must be preserved (e.g. ReadCoverData).
func (h *MP3Handler) ReadTags(filepath string) (*AudioTags, error) {
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true, ParseFrames: mp3ReadFrames})
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer tag.Close()

	return h.extractTags(tag)
}

// getTextFrame returns the text value of the first TextFrame for the given ID,
// or an empty string if the frame is absent or not a TextFrame.
func getTextFrame(tag *id3v2.Tag, id string) string {
	if frames := tag.GetFrames(id); len(frames) > 0 {
		if f, ok := frames[0].(id3v2.TextFrame); ok {
			return f.Text
		}
	}

	return ""
}

// extractTags populates an AudioTags from an already-opened id3v2.Tag.
// It does not touch APIC; callers that need cover art must read it separately.
func (*MP3Handler) extractTags(tag *id3v2.Tag) (*AudioTags, error) {
	tags := &AudioTags{
		Title:       tag.Title(),
		Artist:      tag.Artist(),
		Album:       tag.Album(),
		Genre:       normalizeID3Genre(tag.Genre()),
		Year:        parseYearFallback(tag),
		AlbumArtist: getTextFrame(tag, "TPE2"),
		Composer:    getTextFrame(tag, "TCOM"),
		Conductor:   getTextFrame(tag, "TPE3"),
		ISRC:        getTextFrame(tag, "TSRC"),
		Label:       getTextFrame(tag, "TPUB"),
		Copyright:   getTextFrame(tag, "TCOP"),
	}

	// Track number (TRCK)
	if s := getTextFrame(tag, "TRCK"); s != "" {
		tags.TrackNumber, tags.TotalTracks = parseTrackNum(s)
	}

	// Disc number (TPOS)
	if s := getTextFrame(tag, "TPOS"); s != "" {
		tags.DiscNumber, tags.TotalDiscs = parseTrackNum(s)
	}

	// Comment (COMM)
	for _, frame := range tag.GetFrames("COMM") {
		if comm, ok := frame.(id3v2.CommentFrame); ok {
			tags.Comment = comm.Text
			break
		}
	}

	// Unsynchronized lyrics (USLT)
	for _, frame := range tag.GetFrames("USLT") {
		if uslt, ok := frame.(id3v2.UnsynchronisedLyricsFrame); ok {
			tags.Lyrics = uslt.Lyrics
			break
		}
	}

	// User defined text frames (TXXX) for MusicBrainz IDs and other metadata
	for _, frame := range tag.GetFrames("TXXX") {
		txxx, ok := frame.(id3v2.UserDefinedTextFrame)
		if !ok {
			continue
		}

		desc := txxx.Description
		switch {
		case strings.EqualFold(desc, "MUSICBRAINZ ALBUM ID") || strings.EqualFold(desc, "MUSICBRAINZ_ALBUMID"):
			tags.MBReleaseID = txxx.Value
		case strings.EqualFold(desc, "MUSICBRAINZ ARTIST ID") || strings.EqualFold(desc, "MUSICBRAINZ_ARTISTID"):
			tags.MBArtistID = txxx.Value
		case strings.EqualFold(desc, "MUSICBRAINZ ALBUM ARTIST ID") || strings.EqualFold(desc, "MUSICBRAINZ_ALBUMARTISTID"):
			tags.MBAlbumArtistID = txxx.Value
		case strings.EqualFold(desc, "MUSICBRAINZ TRACK ID") || strings.EqualFold(desc, "MUSICBRAINZ_TRACKID") || strings.EqualFold(desc, "MUSICBRAINZ RECORDING ID"):
			tags.MBRecordingID = txxx.Value
		case strings.EqualFold(desc, "MUSICBRAINZ RELEASE GROUP ID") || strings.EqualFold(desc, "MUSICBRAINZ_RELEASEGROUPID"):
			tags.MBReleaseGroupID = txxx.Value
		case strings.EqualFold(desc, "ACOUSTID_ID") || strings.EqualFold(desc, "ACOUSTID ID"):
			tags.AcoustID = txxx.Value
		case strings.EqualFold(desc, "CATALOGNUMBER") || strings.EqualFold(desc, "CATALOG NUMBER"):
			tags.CatalogNum = txxx.Value
		case strings.EqualFold(desc, "REPLAYGAIN_TRACK_GAIN"):
			tags.ReplayGainTrack = parseReplayGain(txxx.Value)
		case strings.EqualFold(desc, "REPLAYGAIN_TRACK_PEAK"):
			tags.ReplayGainTrackPeak = parseReplayGainPeak(txxx.Value)
		case strings.EqualFold(desc, "REPLAYGAIN_ALBUM_GAIN"):
			tags.ReplayGainAlbum = parseReplayGain(txxx.Value)
		case strings.EqualFold(desc, "REPLAYGAIN_ALBUM_PEAK"):
			tags.ReplayGainAlbumPeak = parseReplayGainPeak(txxx.Value)
		}
	}

	// Unique file identifier (UFID) - sometimes used for MusicBrainz Recording ID
	// (id3v2 parses UFID into UFIDFrame.)
	for _, frame := range tag.GetFrames("UFID") {
		ufid, ok := frame.(id3v2.UFIDFrame)
		if !ok || tags.MBRecordingID != "" || !strings.Contains(ufid.OwnerIdentifier, "musicbrainz.org") {
			continue
		}

		tags.MBRecordingID = string(ufid.Identifier)
	}

	// Duration (TLEN) - in milliseconds. Some taggers write seconds here; a
	// value under one second is treated as absent so the duration is computed
	// from the audio instead.
	if s := getTextFrame(tag, "TLEN"); s != "" {
		if ms, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil && ms >= 1000 {
			tags.Duration = time.Duration(ms) * time.Millisecond
		}
	}

	return tags, nil
}

// mp3TXXXField is one AudioTags field stored as a TXXX frame. desc is written;
// frames whose description matches desc or any alias (case-insensitively, as
// the reader matches them) are replaced when the field is set.
type mp3TXXXField struct {
	desc    string
	aliases []string
	value   func(*AudioTags) string
}

var mp3TXXXFields = []mp3TXXXField{
	{"MusicBrainz Album Id", []string{"MUSICBRAINZ_ALBUMID"}, func(t *AudioTags) string { return t.MBReleaseID }},
	{"MusicBrainz Artist Id", []string{"MUSICBRAINZ_ARTISTID"}, func(t *AudioTags) string { return t.MBArtistID }},
	{"MusicBrainz Album Artist Id", []string{"MUSICBRAINZ_ALBUMARTISTID"}, func(t *AudioTags) string { return t.MBAlbumArtistID }},
	{"MusicBrainz Recording Id", []string{"MusicBrainz Track Id", "MUSICBRAINZ_TRACKID"}, func(t *AudioTags) string { return t.MBRecordingID }},
	{"MusicBrainz Release Group Id", []string{"MUSICBRAINZ_RELEASEGROUPID"}, func(t *AudioTags) string { return t.MBReleaseGroupID }},
	{"ACOUSTID_ID", []string{"ACOUSTID ID"}, func(t *AudioTags) string { return t.AcoustID }},
	{"CATALOGNUMBER", []string{"CATALOG NUMBER"}, func(t *AudioTags) string { return t.CatalogNum }},
	{"REPLAYGAIN_TRACK_GAIN", nil, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrack, "%.2f dB") }},
	{"REPLAYGAIN_TRACK_PEAK", nil, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrackPeak, "%.6f") }},
	{"REPLAYGAIN_ALBUM_GAIN", nil, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbum, "%.2f dB") }},
	{"REPLAYGAIN_ALBUM_PEAK", nil, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbumPeak, "%.6f") }},
}

// mp3MusicBrainzUFIDOwner is the UFID owner Picard uses for the recording ID.
const mp3MusicBrainzUFIDOwner = "http://musicbrainz.org"

// WriteTags writes ID3v2 tags to an MP3 file. Only the fields tags sets are
// written; every other existing frame (including ones this package doesn't
// model: POPM, PRIV, GEOB, WXXX, CHAP/CTOC, unrelated TXXX, ...) is kept.
// Existing cover art is kept unless tags.CoverData carries a new one.
func (h *MP3Handler) WriteTags(_ context.Context, filepath string, tags *AudioTags) error {
	// All frames must be parsed: id3v2 drops frames it did not parse on Save.
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true})
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}
	defer tag.Close()

	// Set ID3v2.4 version for better Unicode support
	tag.SetVersion(4)

	setText := func(id, value string) {
		if value != "" {
			tag.AddFrame(id, id3v2.TextFrame{Encoding: id3v2.EncodingUTF8, Text: value})
		}
	}

	setText("TIT2", tags.Title)
	setText("TPE1", tags.Artist)
	setText("TALB", tags.Album)
	setText("TCON", tags.Genre)

	if tags.Year > 0 {
		// Drop legacy TYER (v2.3) only when writing the new TDRC (v2.4), to
		// avoid dual year frames without losing the year when none is given.
		tag.DeleteFrames("TYER")
		setText("TDRC", strconv.Itoa(tags.Year))
	}

	setText("TPE2", tags.AlbumArtist)
	setText("TCOM", tags.Composer)
	setText("TPE3", tags.Conductor)

	if tags.TrackNumber > 0 {
		setText("TRCK", formatTrackNum(tags.TrackNumber, tags.TotalTracks))
	}

	if tags.DiscNumber > 0 {
		setText("TPOS", formatTrackNum(tags.DiscNumber, tags.TotalDiscs))
	}

	setText("TSRC", tags.ISRC)
	setText("TPUB", tags.Label)
	setText("TCOP", tags.Copyright)

	// Comment (COMM): replace the main (description-less) comment only, so
	// described comments such as iTunNORM survive.
	if tags.Comment != "" {
		keepFrames(tag, "COMM", func(f id3v2.Framer) bool {
			cf, ok := f.(id3v2.CommentFrame)
			return !ok || cf.Description != ""
		})
		tag.AddFrame("COMM", id3v2.CommentFrame{
			Encoding:    id3v2.EncodingUTF8,
			Language:    "eng",
			Description: "",
			Text:        tags.Comment,
		})
	}

	// Lyrics (USLT)
	if tags.Lyrics != "" {
		tag.DeleteFrames("USLT")
		tag.AddFrame("USLT", id3v2.UnsynchronisedLyricsFrame{
			Encoding:          id3v2.EncodingUTF8,
			ContentDescriptor: "",
			Lyrics:            tags.Lyrics,
			Language:          "eng",
		})
	}

	// MusicBrainz / AcoustID / catalog / ReplayGain TXXX frames: replace only
	// the descriptions being written (under every alias the reader accepts),
	// so re-tagging neither duplicates them nor drops unrelated TXXX frames.
	for i := range mp3TXXXFields {
		f := &mp3TXXXFields[i]

		value := f.value(tags)
		if value == "" {
			continue
		}

		keepFrames(tag, "TXXX", func(fr id3v2.Framer) bool {
			udtf, ok := fr.(id3v2.UserDefinedTextFrame)
			if !ok {
				return true
			}

			if strings.EqualFold(udtf.Description, f.desc) {
				return false
			}

			for _, a := range f.aliases {
				if strings.EqualFold(udtf.Description, a) {
					return false
				}
			}

			return true
		})

		h.writeTXXX(tag, f.desc, value)
	}

	// Keep the MusicBrainz UFID in step with the recording ID (it replaces
	// the frame of the same owner); other owners' UFIDs are untouched.
	if tags.MBRecordingID != "" {
		tag.AddFrame("UFID", id3v2.UFIDFrame{
			OwnerIdentifier: mp3MusicBrainzUFIDOwner,
			Identifier:      []byte(tags.MBRecordingID),
		})
	}

	// Cover art (APIC): a new cover replaces the front/other pictures (the
	// ones the readers treat as "the cover"); back covers etc. are kept.
	if len(tags.CoverData) > 0 && !hasPicture(tag, tags.CoverData) {
		keepFrames(tag, "APIC", func(f id3v2.Framer) bool {
			pf, ok := f.(id3v2.PictureFrame)
			return !ok || (pf.PictureType != id3v2.PTFrontCover && pf.PictureType != id3v2.PTOther)
		})
		tag.AddFrame("APIC", id3v2.PictureFrame{
			Encoding:    id3v2.EncodingUTF8,
			MimeType:    tags.CoverMIME,
			PictureType: id3v2.PTFrontCover,
			Description: "Front Cover",
			Picture:     tags.CoverData,
		})
	}

	if err := tag.Save(); err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}

	return nil
}

// keepFrames removes the frames with the given ID for which keep returns false.
func keepFrames(tag *id3v2.Tag, id string, keep func(id3v2.Framer) bool) {
	frames := append([]id3v2.Framer(nil), tag.GetFrames(id)...)

	tag.DeleteFrames(id)

	for _, f := range frames {
		if keep(f) {
			tag.AddFrame(id, f)
		}
	}
}

// hasPicture reports whether the tag already holds an APIC with exactly data
// (e.g. the existing cover carried over by the caller via ReadCoverData).
func hasPicture(tag *id3v2.Tag, data []byte) bool {
	for _, f := range tag.GetFrames("APIC") {
		if pf, ok := f.(id3v2.PictureFrame); ok && bytes.Equal(pf.Picture, data) {
			return true
		}
	}

	return false
}

// writeTXXX is a helper to add TXXX frames only if the value is non-empty.
func (*MP3Handler) writeTXXX(tag *id3v2.Tag, description, value string) {
	if value != "" {
		tag.AddFrame("TXXX", id3v2.UserDefinedTextFrame{
			Encoding:    id3v2.EncodingUTF8,
			Description: description,
			Value:       value,
		})
	}
}

// parseYear extracts a year from a date string.
func parseYear(s string) int {
	s = strings.TrimSpace(s)
	if len(s) >= 4 {
		if year, err := strconv.Atoi(s[:4]); err == nil {
			return year
		}
	}

	return 0
}

// parseYearFallback reads TDRC (v2.4) and falls back to TYER (v2.3) when
// TDRC is absent — handles files that were tagged by old software or that
// carry both frames.
func parseYearFallback(tag *id3v2.Tag) int {
	if y := parseYear(tag.Year()); y > 0 {
		return y
	}

	return parseYear(getTextFrame(tag, "TYER"))
}

// parseTrackNum parses "track/total" format strings.
func parseTrackNum(s string) (track, total int) {
	s = strings.TrimSpace(s)
	if before, after, ok := strings.Cut(s, "/"); ok {
		track, _ = strconv.Atoi(strings.TrimSpace(before))
		total, _ = strconv.Atoi(strings.TrimSpace(after))
	} else {
		track, _ = strconv.Atoi(s)
	}

	return
}

// formatTrackNum formats track/total for writing.
func formatTrackNum(track, total int) string {
	if total > 0 {
		return formatPaddedNum(track, total) + "/" + formatPaddedNum(total, total)
	}

	return formatPaddedNum(track, total)
}

// parseReplayGain parses ReplayGain value strings like "-3.45 dB".
func parseReplayGain(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "dB")
	s = strings.TrimSpace(s)

	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}

	return 0
}

// parseReplayGainPeak parses ReplayGain peak values.
func parseReplayGainPeak(s string) float64 {
	s = strings.TrimSpace(s)
	if val, err := strconv.ParseFloat(s, 64); err == nil {
		return val
	}

	return 0
}
