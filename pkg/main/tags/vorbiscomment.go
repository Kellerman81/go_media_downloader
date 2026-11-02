package tags

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

// applyVorbisComment maps one Vorbis comment (KEY=value) onto tags. FLAC and
// Ogg share the exact same comment format, so both readers use this.
// Unmodelled keys are ignored.
func applyVorbisComment(key, value string, tags *AudioTags) {
	switch strings.ToUpper(key) {
	// Standard tags
	case "TITLE":
		tags.Title = value
	case "ARTIST":
		tags.Artist = value
	case "ALBUM":
		tags.Album = value
	case "ALBUMARTIST", "ALBUM ARTIST":
		tags.AlbumArtist = value
	case "GENRE":
		tags.Genre = value
	case "DATE", "YEAR":
		tags.Year = parseYear(value)
	case "TRACKNUMBER", "TRACK":
		// "3/12" carries the total too; an explicit TRACKTOTAL (in any order) wins.
		var total int

		tags.TrackNumber, total = parseTrackNum(value)
		if total > 0 && tags.TotalTracks == 0 {
			tags.TotalTracks = total
		}

	case "TRACKTOTAL", "TOTALTRACKS", "TRACKC":
		if n, _ := strconv.Atoi(strings.TrimSpace(value)); n > 0 {
			tags.TotalTracks = n
		}

	case "DISCNUMBER", "DISC":
		var total int

		tags.DiscNumber, total = parseTrackNum(value)
		if total > 0 && tags.TotalDiscs == 0 {
			tags.TotalDiscs = total
		}

	case "DISCTOTAL", "TOTALDISCS", "DISCC":
		if n, _ := strconv.Atoi(strings.TrimSpace(value)); n > 0 {
			tags.TotalDiscs = n
		}

	case "COMMENT", "DESCRIPTION":
		tags.Comment = value

	// Extended tags
	case "COMPOSER":
		tags.Composer = value
	case "CONDUCTOR":
		tags.Conductor = value
	case "LABEL", "ORGANIZATION", "PUBLISHER":
		tags.Label = value
	case "CATALOGNUMBER", "CATALOG", "LABELNO":
		tags.CatalogNum = value
	case "ISRC":
		tags.ISRC = value
	case "LYRICS", "UNSYNCEDLYRICS":
		tags.Lyrics = value
	case "COPYRIGHT":
		tags.Copyright = value

	// MusicBrainz IDs
	case "MUSICBRAINZ_TRACKID", "MUSICBRAINZ_RECORDINGID":
		tags.MBRecordingID = value
	case "MUSICBRAINZ_ALBUMID", "MUSICBRAINZ_RELEASEID":
		tags.MBReleaseID = value
	case "MUSICBRAINZ_RELEASEGROUPID":
		tags.MBReleaseGroupID = value
	case "MUSICBRAINZ_ARTISTID":
		tags.MBArtistID = value
	case "MUSICBRAINZ_ALBUMARTISTID":
		tags.MBAlbumArtistID = value

	// AcoustID
	case "ACOUSTID_ID":
		tags.AcoustID = value

	// ReplayGain
	case "REPLAYGAIN_TRACK_GAIN":
		tags.ReplayGainTrack = parseReplayGain(value)
	case "REPLAYGAIN_TRACK_PEAK":
		tags.ReplayGainTrackPeak = parseReplayGainPeak(value)
	case "REPLAYGAIN_ALBUM_GAIN":
		tags.ReplayGainAlbum = parseReplayGain(value)
	case "REPLAYGAIN_ALBUM_PEAK":
		tags.ReplayGainAlbumPeak = parseReplayGainPeak(value)
	}
}

// vorbisField describes one AudioTags field as written to Vorbis comments.
// keys[0] is the key written; every key in keys (the aliases the reader
// accepts) is removed from the existing comments when the field is written,
// so a re-tag never leaves a stale alias behind. Fields whose value is empty
// are not written and their existing comments are preserved.
type vorbisField struct {
	keys  []string
	value func(*AudioTags) string
}

func vorbisFloat(v float64, format string) string {
	if v == 0 {
		return ""
	}

	return fmt.Sprintf(format, v)
}

func vorbisPositive(v int) string {
	if v <= 0 {
		return ""
	}

	return strconv.Itoa(v)
}

var vorbisFields = []vorbisField{
	{[]string{"TITLE"}, func(t *AudioTags) string { return t.Title }},
	{[]string{"ARTIST"}, func(t *AudioTags) string { return t.Artist }},
	{[]string{"ALBUM"}, func(t *AudioTags) string { return t.Album }},
	{[]string{"ALBUMARTIST", "ALBUM ARTIST"}, func(t *AudioTags) string { return t.AlbumArtist }},
	{[]string{"GENRE"}, func(t *AudioTags) string { return t.Genre }},
	{[]string{"DATE", "YEAR"}, func(t *AudioTags) string { return vorbisPositive(t.Year) }},
	{[]string{"TRACKNUMBER", "TRACK"}, func(t *AudioTags) string {
		if t.TrackNumber <= 0 {
			return ""
		}

		return formatPaddedNum(t.TrackNumber, t.TotalTracks)
	}},
	{[]string{"TRACKTOTAL", "TOTALTRACKS", "TRACKC"}, func(t *AudioTags) string {
		if t.TotalTracks <= 0 {
			return ""
		}

		return formatPaddedNum(t.TotalTracks, t.TotalTracks)
	}},
	{[]string{"DISCNUMBER", "DISC"}, func(t *AudioTags) string { return vorbisPositive(t.DiscNumber) }},
	{[]string{"DISCTOTAL", "TOTALDISCS", "DISCC"}, func(t *AudioTags) string { return vorbisPositive(t.TotalDiscs) }},
	{[]string{"COMMENT", "DESCRIPTION"}, func(t *AudioTags) string { return t.Comment }},
	{[]string{"COMPOSER"}, func(t *AudioTags) string { return t.Composer }},
	{[]string{"CONDUCTOR"}, func(t *AudioTags) string { return t.Conductor }},
	{[]string{"LABEL", "ORGANIZATION", "PUBLISHER"}, func(t *AudioTags) string { return t.Label }},
	{[]string{"CATALOGNUMBER", "CATALOG", "LABELNO"}, func(t *AudioTags) string { return t.CatalogNum }},
	{[]string{"ISRC"}, func(t *AudioTags) string { return t.ISRC }},
	{[]string{"LYRICS", "UNSYNCEDLYRICS"}, func(t *AudioTags) string { return t.Lyrics }},
	{[]string{"COPYRIGHT"}, func(t *AudioTags) string { return t.Copyright }},
	{[]string{"MUSICBRAINZ_TRACKID", "MUSICBRAINZ_RECORDINGID"}, func(t *AudioTags) string { return t.MBRecordingID }},
	{[]string{"MUSICBRAINZ_ALBUMID", "MUSICBRAINZ_RELEASEID"}, func(t *AudioTags) string { return t.MBReleaseID }},
	{[]string{"MUSICBRAINZ_RELEASEGROUPID"}, func(t *AudioTags) string { return t.MBReleaseGroupID }},
	{[]string{"MUSICBRAINZ_ARTISTID"}, func(t *AudioTags) string { return t.MBArtistID }},
	{[]string{"MUSICBRAINZ_ALBUMARTISTID"}, func(t *AudioTags) string { return t.MBAlbumArtistID }},
	{[]string{"ACOUSTID_ID"}, func(t *AudioTags) string { return t.AcoustID }},
	{[]string{"REPLAYGAIN_TRACK_GAIN"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrack, "%.2f dB") }},
	{[]string{"REPLAYGAIN_TRACK_PEAK"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainTrackPeak, "%.6f") }},
	{[]string{"REPLAYGAIN_ALBUM_GAIN"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbum, "%.2f dB") }},
	{[]string{"REPLAYGAIN_ALBUM_PEAK"}, func(t *AudioTags) string { return vorbisFloat(t.ReplayGainAlbumPeak, "%.6f") }},
}

// vorbisCommentUpdate returns the comments to write for tags as "KEY=value"
// and the (upper-case) keys that must be removed from the existing comments
// before writing them.
func vorbisCommentUpdate(tags *AudioTags) (set []string, remove map[string]struct{}) {
	remove = make(map[string]struct{})

	for i := range vorbisFields {
		v := vorbisFields[i].value(tags)
		if v == "" {
			continue
		}

		for _, k := range vorbisFields[i].keys {
			remove[k] = struct{}{}
		}

		set = append(set, vorbisFields[i].keys[0]+"="+v)
	}

	return set, remove
}

// vorbisCommentKey returns the upper-cased key of a "KEY=value" comment.
func vorbisCommentKey(comment string) string {
	key, _, _ := strings.Cut(comment, "=")
	return strings.ToUpper(key)
}

// parseVorbisCommentList parses a Vorbis comment body (vendor length onward,
// i.e. without the "\x03vorbis"/"OpusTags" magic) into the vendor string, the
// raw "KEY=value" comments and whatever bytes follow the last comment (the
// Vorbis framing bit, or Opus's optional binary extension data). ok is false
// when the body is truncated or a length field is out of bounds.
func parseVorbisCommentList(data []byte) (vendor string, comments []string, rest []byte, ok bool) {
	n := uint64(len(data))
	if n < 4 {
		return "", nil, nil, false
	}

	vendorLen := uint64(binary.LittleEndian.Uint32(data[0:4]))
	off := uint64(4)

	if off+vendorLen+4 > n {
		return "", nil, nil, false
	}

	vendor = string(data[off : off+vendorLen])
	off += vendorLen

	count := uint64(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4

	// Each comment needs at least its 4-byte length; this also bounds the
	// allocation against a hostile count.
	if count > (n-off)/4 {
		return vendor, nil, nil, false
	}

	comments = make([]string, 0, count)

	for range count {
		if off+4 > n {
			return vendor, comments, nil, false
		}

		l := uint64(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4

		if off+l > n {
			return vendor, comments, nil, false
		}

		comments = append(comments, string(data[off:off+l]))
		off += l
	}

	return vendor, comments, data[off:], true
}
