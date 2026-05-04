package tags

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/mewkiz/flac/meta"
)

// FLACHandler handles reading and writing Vorbis Comment tags for FLAC files.
// Reading uses the mewkiz/flac library directly.
// Writing uses the metaflac command-line tool (part of the FLAC reference implementation).
type FLACHandler struct {
	// MetaflacPath is the path to the metaflac binary.
	// If empty, "metaflac" will be searched in PATH.
	MetaflacPath string
}

// NewFLACHandler creates a new FLAC tag handler.
func NewFLACHandler() *FLACHandler {
	return &FLACHandler{}
}

// SupportedFormats returns the file extensions supported by this handler.
func (h *FLACHandler) SupportedFormats() []string {
	return []string{".flac"}
}

// getMetaflacPath returns the path to the metaflac binary.
func (h *FLACHandler) getMetaflacPath() string {
	if h.MetaflacPath != "" {
		return h.MetaflacPath
	}

	return "metaflac"
}

// ReadTags reads Vorbis Comment tags from a FLAC file, skipping embedded cover art.
// Cover art (PICTURE blocks) is intentionally omitted — it can be several MB per file
// and is not needed for the matching / enrichment hot-path.
// Use ReadTagsWithCover when cover art must be preserved (e.g. CopyTags).
func (h *FLACHandler) ReadTags(filepath string) (*AudioTags, error) {
	return h.readTags(filepath, false)
}

// ReadTagsWithCover reads Vorbis Comment tags including embedded cover art.
// Use this only when cover art is needed (e.g. CopyTags); prefer ReadTags
// for metadata-only access since loading cover art is expensive.
func (h *FLACHandler) ReadTagsWithCover(filepath string) (*AudioTags, error) {
	return h.readTags(filepath, true)
}

func (h *FLACHandler) readTags(filepath string, withCover bool) (*AudioTags, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}
	defer file.Close()

	tags, err := h.parseFLACMeta(file, withCover)
	if err != nil {
		return nil, &ErrReadFailed{Path: filepath, Reason: err.Error()}
	}

	return tags, nil
}

// parseFLACMeta reads FLAC metadata using meta.New to read only block headers,
// then selectively calls block.Parse() or block.Skip() per block type.
// PICTURE blocks are skipped without any body allocation when withCover is false.
func (h *FLACHandler) parseFLACMeta(r io.Reader, withCover bool) (*AudioTags, error) {
	br := bufio.NewReader(r)
	defer br.Reset(nil) // release buffer on return
	// Verify "fLaC" signature.
	var sig [4]byte
	if _, err := io.ReadFull(br, sig[:]); err != nil {
		return nil, fmt.Errorf("reading signature: %w", err)
	}

	if sig != [4]byte{'f', 'L', 'a', 'C'} {
		return nil, fmt.Errorf("not a FLAC file")
	}

	tags := &AudioTags{}

	for {
		// meta.New reads only the 4-byte block header — body is NOT yet allocated.
		block, err := meta.New(br)
		if err != nil {
			return nil, fmt.Errorf("reading block header: %w", err)
		}

		switch block.Type {
		case meta.TypeStreamInfo:
			if err := block.Parse(); err != nil {
				return nil, fmt.Errorf("parsing StreamInfo: %w", err)
			}

			if si, ok := block.Body.(*meta.StreamInfo); ok {
				tags.SampleRate = int(si.SampleRate)
				tags.BitDepth = int(si.BitsPerSample)
				tags.Channels = int(si.NChannels)

				if si.SampleRate > 0 && si.NSamples > 0 {
					tags.Duration = time.Duration(
						float64(si.NSamples) / float64(si.SampleRate) * float64(time.Second),
					)
				}
			}

		case meta.TypeVorbisComment:
			if err := block.Parse(); err != nil {
				return nil, fmt.Errorf("parsing VorbisComment: %w", err)
			}

			if vc, ok := block.Body.(*meta.VorbisComment); ok {
				h.parseVorbisComments(vc.Tags, tags)
			}

		case meta.TypePicture:
			if !withCover {
				// Skip body bytes without allocating — this is the key optimization.
				block.Skip() //nolint:errcheck
				break
			}

			if err := block.Parse(); err != nil {
				return nil, fmt.Errorf("parsing Picture: %w", err)
			}

			if pic, ok := block.Body.(*meta.Picture); ok {
				// Prefer front cover (3), fall back to first available (0).
				if pic.Type == 3 || (tags.CoverData == nil && pic.Type == 0) {
					tags.CoverData = pic.Data
					tags.CoverMIME = pic.MIME
				}
			}

		default:
			block.Skip() //nolint:errcheck
		}

		if block.IsLast {
			break
		}
	}

	return tags, nil
}

// parseVorbisComments extracts tag values from Vorbis Comment fields.
func (h *FLACHandler) parseVorbisComments(comments [][2]string, tags *AudioTags) {
	for i := range comments {
		key := strings.ToUpper(comments[i][0])
		value := comments[i][1]

		switch key {
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
			tags.TrackNumber, _ = parseTrackNum(value)
		case "TRACKTOTAL", "TOTALTRACKS", "TRACKC":
			tags.TotalTracks, _ = strconv.Atoi(value)
		case "DISCNUMBER", "DISC":
			tags.DiscNumber, _ = parseTrackNum(value)
		case "DISCTOTAL", "TOTALDISCS", "DISCC":
			tags.TotalDiscs, _ = strconv.Atoi(value)
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
}

// WriteTags writes Vorbis Comment tags to a FLAC file using metaflac.
// Requires the metaflac command-line tool to be installed and available in PATH.
func (h *FLACHandler) WriteTags(ctx context.Context, filepath string, tags *AudioTags) error {
	metaflac := h.getMetaflacPath()

	// Check if metaflac is available
	if _, err := exec.LookPath(metaflac); err != nil {
		return &ErrWriteFailed{
			Path:   filepath,
			Reason: "metaflac not found in PATH - install flac package to enable FLAC tagging",
		}
	}

	// First, remove all existing vorbis comments
	var stderr bytes.Buffer

	removeCmd := exec.CommandContext(ctx, metaflac, "--remove-all-tags", filepath)

	removeCmd.Stderr = &stderr
	if err := removeCmd.Run(); err != nil {
		return &ErrWriteFailed{
			Path:   filepath,
			Reason: fmt.Sprintf("failed to remove existing tags: %s", stderr.String()),
		}
	}

	// Build the list of tags to set
	tagArgs := h.buildTagArgs(tags)

	if len(tagArgs) > 0 {
		// Add all new tags
		args := append(tagArgs, filepath)
		setCmd := exec.CommandContext(ctx, metaflac, args...)

		stderr.Reset()

		setCmd.Stderr = &stderr
		if err := setCmd.Run(); err != nil {
			return &ErrWriteFailed{
				Path:   filepath,
				Reason: fmt.Sprintf("failed to set tags: %s", stderr.String()),
			}
		}
	}

	// Handle cover art
	if len(tags.CoverData) > 0 {
		if err := h.writeCoverArt(ctx, filepath, tags); err != nil {
			return err
		}
	}

	return nil
}

// buildTagArgs builds the metaflac arguments for setting tags.
func (h *FLACHandler) buildTagArgs(tags *AudioTags) []string {
	var args []string

	addTag := func(key, value string) {
		if value != "" {
			args = append(args, fmt.Sprintf("--set-tag=%s=%s", key, value))
		}
	}

	// Standard tags
	addTag("TITLE", tags.Title)
	addTag("ARTIST", tags.Artist)
	addTag("ALBUM", tags.Album)
	addTag("ALBUMARTIST", tags.AlbumArtist)
	addTag("GENRE", tags.Genre)

	if tags.Year > 0 {
		addTag("DATE", strconv.Itoa(tags.Year))
	}

	if tags.TrackNumber > 0 {
		addTag("TRACKNUMBER", formatPaddedNum(tags.TrackNumber, tags.TotalTracks))
	}

	if tags.TotalTracks > 0 {
		addTag("TRACKTOTAL", formatPaddedNum(tags.TotalTracks, tags.TotalTracks))
	}

	if tags.DiscNumber > 0 {
		addTag("DISCNUMBER", strconv.Itoa(tags.DiscNumber))
	}

	if tags.TotalDiscs > 0 {
		addTag("DISCTOTAL", strconv.Itoa(tags.TotalDiscs))
	}

	addTag("COMMENT", tags.Comment)

	// Extended tags
	addTag("COMPOSER", tags.Composer)
	addTag("CONDUCTOR", tags.Conductor)
	addTag("LABEL", tags.Label)
	addTag("CATALOGNUMBER", tags.CatalogNum)
	addTag("ISRC", tags.ISRC)
	addTag("LYRICS", tags.Lyrics)
	addTag("COPYRIGHT", tags.Copyright)

	// MusicBrainz IDs
	addTag("MUSICBRAINZ_TRACKID", tags.MBRecordingID)
	addTag("MUSICBRAINZ_ALBUMID", tags.MBReleaseID)
	addTag("MUSICBRAINZ_RELEASEGROUPID", tags.MBReleaseGroupID)
	addTag("MUSICBRAINZ_ARTISTID", tags.MBArtistID)
	addTag("MUSICBRAINZ_ALBUMARTISTID", tags.MBAlbumArtistID)

	// AcoustID
	addTag("ACOUSTID_ID", tags.AcoustID)

	// ReplayGain
	if tags.ReplayGainTrack != 0 {
		addTag("REPLAYGAIN_TRACK_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainTrack))
	}

	if tags.ReplayGainTrackPeak != 0 {
		addTag("REPLAYGAIN_TRACK_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainTrackPeak))
	}

	if tags.ReplayGainAlbum != 0 {
		addTag("REPLAYGAIN_ALBUM_GAIN", fmt.Sprintf("%.2f dB", tags.ReplayGainAlbum))
	}

	if tags.ReplayGainAlbumPeak != 0 {
		addTag("REPLAYGAIN_ALBUM_PEAK", fmt.Sprintf("%.6f", tags.ReplayGainAlbumPeak))
	}

	return args
}

// writeCoverArt writes cover art to a FLAC file using metaflac.
func (h *FLACHandler) writeCoverArt(ctx context.Context, filepath string, tags *AudioTags) error {
	metaflac := h.getMetaflacPath()

	// Write cover data to a temporary file
	tmpFile, err := os.CreateTemp("", "flac-cover-*")
	if err != nil {
		return &ErrWriteFailed{
			Path:   filepath,
			Reason: fmt.Sprintf("failed to create temp file for cover: %v", err),
		}
	}

	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(tags.CoverData); err != nil {
		tmpFile.Close()

		return &ErrWriteFailed{
			Path:   filepath,
			Reason: fmt.Sprintf("failed to write cover to temp file: %v", err),
		}
	}

	tmpFile.Close()

	// Remove existing pictures first
	exec.CommandContext(ctx, metaflac, "--remove", "--block-type=PICTURE", filepath).
		Run()

		//nolint:errcheck // ignore: might not have pictures

	// Import the new picture
	// Format: --import-picture-from=TYPE|MIME|DESC|WIDTHxHEIGHTxDEPTH/COLORS|FILE
	// Type 3 = Front Cover, we can use simplified format
	pictureSpec := logger.JoinStrings("3|", tags.CoverMIME, "|Front Cover||", tmpPath)
	importCmd := exec.CommandContext(ctx,
		metaflac,
		logger.JoinStrings("--import-picture-from=", pictureSpec),
		filepath,
	)

	var stderr bytes.Buffer

	importCmd.Stderr = &stderr
	if err := importCmd.Run(); err != nil {
		return &ErrWriteFailed{
			Path:   filepath,
			Reason: fmt.Sprintf("failed to import cover art: %s", stderr.String()),
		}
	}

	return nil
}
