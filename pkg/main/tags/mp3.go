package tags

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
)

// mp3ParseFrames lists every ID3v2 frame ID we actually use in ReadTags,
// intentionally excluding APIC (cover art) which can be several MB per file.
// The id3v2 library skips the frame body entirely for IDs not in this list,
// eliminating the dominant allocation source on the matching hot-path.
var mp3ParseFrames = []string{
	"TIT2", "TPE1", "TALB", "TCON", "TYER", "TDRC",
	"TPE2", "TCOM", "TPE3", "TRCK", "TPOS",
	"COMM", "TSRC", "TPUB", "TCOP", "USLT",
	"TXXX", "UFID", "TLEN",
}

// MP3Handler handles reading and writing ID3v2 tags for MP3 files.
type MP3Handler struct{}

// NewMP3Handler creates a new MP3 tag handler.
func NewMP3Handler() *MP3Handler {
	return &MP3Handler{}
}

// SupportedFormats returns the file extensions supported by this handler.
func (h *MP3Handler) SupportedFormats() []string {
	return []string{".mp3"}
}

// ReadTagsWithCover reads ID3v2 tags including embedded cover art (APIC).
// Use this only when cover art is needed (e.g. CopyTags); prefer ReadTags
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
// Use ReadTagsWithCover when cover art must be preserved (e.g. CopyTags).
func (h *MP3Handler) ReadTags(filepath string) (*AudioTags, error) {
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true, ParseFrames: mp3ParseFrames})
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
func (h *MP3Handler) extractTags(tag *id3v2.Tag) (*AudioTags, error) {
	tags := &AudioTags{
		Title:       tag.Title(),
		Artist:      tag.Artist(),
		Album:       tag.Album(),
		Genre:       tag.Genre(),
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
	for _, frame := range tag.GetFrames("UFID") {
		ufid, ok := frame.(id3v2.UnknownFrame)
		if !ok {
			continue
		}

		// Check if this is a MusicBrainz UFID
		data := string(ufid.Body)
		if !strings.Contains(data, "musicbrainz.org") || tags.MBRecordingID != "" {
			continue
		}

		// Extract UUID after the null terminator
		parts := strings.SplitN(data, "\x00", 2)
		if len(parts) == 2 {
			tags.MBRecordingID = parts[1]
		}
	}

	// Duration (TLEN) - in milliseconds
	if s := getTextFrame(tag, "TLEN"); s != "" {
		if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
			tags.Duration = time.Duration(ms) * time.Millisecond
		}
	}

	return tags, nil
}

// WriteTags writes ID3v2 tags to an MP3 file.
// Cover art (APIC) is only written when tags.CoverData is set.
// Opening with ParseFrames avoids loading existing APIC into memory —
// callers that need to preserve cover art should use ReadTagsWithCover
// to populate tags.CoverData before calling WriteTags.
func (h *MP3Handler) WriteTags(ctx context.Context, filepath string, tags *AudioTags) error {
	tag, err := id3v2.Open(filepath, id3v2.Options{Parse: true, ParseFrames: mp3ParseFrames})
	if err != nil {
		return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
	}
	defer tag.Close()

	// Set ID3v2.4 version for better Unicode support
	tag.SetVersion(4)

	// Standard tags
	tag.SetTitle(tags.Title)
	tag.SetArtist(tags.Artist)
	tag.SetAlbum(tags.Album)
	tag.SetGenre(tags.Genre)

	// Remove legacy TYER (v2.3) before writing TDRC (v2.4) to avoid dual year frames.
	tag.DeleteFrames("TYER")
	if tags.Year > 0 {
		tag.SetYear(strconv.Itoa(tags.Year))
	}

	// Album artist (TPE2)
	if tags.AlbumArtist != "" {
		tag.AddFrame("TPE2", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.AlbumArtist,
		})
	}

	// Composer (TCOM)
	if tags.Composer != "" {
		tag.AddFrame("TCOM", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.Composer,
		})
	}

	// Conductor (TPE3)
	if tags.Conductor != "" {
		tag.AddFrame("TPE3", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.Conductor,
		})
	}

	// Track number (TRCK)
	if tags.TrackNumber > 0 {
		tag.AddFrame("TRCK", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     formatTrackNum(tags.TrackNumber, tags.TotalTracks),
		})
	}

	// Disc number (TPOS)
	if tags.DiscNumber > 0 {
		tag.AddFrame("TPOS", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     formatTrackNum(tags.DiscNumber, tags.TotalDiscs),
		})
	}

	// Comment (COMM)
	if tags.Comment != "" {
		tag.AddFrame("COMM", id3v2.CommentFrame{
			Encoding:    id3v2.EncodingUTF8,
			Language:    "eng",
			Description: "",
			Text:        tags.Comment,
		})
	}

	// ISRC (TSRC)
	if tags.ISRC != "" {
		tag.AddFrame("TSRC", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.ISRC,
		})
	}

	// Publisher/Label (TPUB)
	if tags.Label != "" {
		tag.AddFrame("TPUB", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.Label,
		})
	}

	// Copyright (TCOP)
	if tags.Copyright != "" {
		tag.AddFrame("TCOP", id3v2.TextFrame{
			Encoding: id3v2.EncodingUTF8,
			Text:     tags.Copyright,
		})
	}

	// Lyrics (USLT)
	if tags.Lyrics != "" {
		tag.AddFrame("USLT", id3v2.UnsynchronisedLyricsFrame{
			Encoding:          id3v2.EncodingUTF8,
			Language:          "eng",
			ContentDescriptor: "",
			Lyrics:            tags.Lyrics,
		})
	}

	// MusicBrainz IDs as TXXX frames
	h.writeTXXX(tag, "MusicBrainz Album Id", tags.MBReleaseID)
	h.writeTXXX(tag, "MusicBrainz Artist Id", tags.MBArtistID)
	h.writeTXXX(tag, "MusicBrainz Album Artist Id", tags.MBAlbumArtistID)
	h.writeTXXX(tag, "MusicBrainz Recording Id", tags.MBRecordingID)
	h.writeTXXX(tag, "MusicBrainz Release Group Id", tags.MBReleaseGroupID)

	// AcoustID
	h.writeTXXX(tag, "ACOUSTID_ID", tags.AcoustID)

	// Catalog number
	h.writeTXXX(tag, "CATALOGNUMBER", tags.CatalogNum)

	// ReplayGain
	if tags.ReplayGainTrack != 0 {
		h.writeTXXX(tag, "REPLAYGAIN_TRACK_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainTrack))
	}

	if tags.ReplayGainTrackPeak != 0 {
		h.writeTXXX(tag, "REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainTrackPeak))
	}

	if tags.ReplayGainAlbum != 0 {
		h.writeTXXX(tag, "REPLAYGAIN_ALBUM_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainAlbum))
	}

	if tags.ReplayGainAlbumPeak != 0 {
		h.writeTXXX(tag, "REPLAYGAIN_ALBUM_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainAlbumPeak))
	}

	// Cover art (APIC)
	if len(tags.CoverData) > 0 {
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

// writeTXXX is a helper to add TXXX frames only if the value is non-empty.
func (h *MP3Handler) writeTXXX(tag *id3v2.Tag, description, value string) {
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
	s = strings.TrimSuffix(s, " dB")

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
