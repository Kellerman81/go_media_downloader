package tags

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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
func (*FLACHandler) SupportedFormats() []string {
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
// Use ReadTagsWithCover when cover art must be preserved (e.g. ReadCoverData).
func (h *FLACHandler) ReadTags(filepath string) (*AudioTags, error) {
	return h.readTags(filepath, false)
}

// ReadTagsWithCover reads Vorbis Comment tags including embedded cover art.
// Use this only when cover art is needed (e.g. ReadCoverData); prefer ReadTags
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
		return nil, errors.New("not a FLAC file")
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
func (*FLACHandler) parseVorbisComments(comments [][2]string, tags *AudioTags) {
	for i := range comments {
		applyVorbisComment(comments[i][0], comments[i][1], tags)
	}
}

// WriteTags writes Vorbis Comment tags (and, if present, cover art) to a FLAC
// file using metaflac. Tag removal, tag setting, and picture import are all
// "shorthand operations" in metaflac's own terminology and combine into one
// atomic invocation; removing an existing picture block is a "major
// operation" and metaflac refuses to mix major and shorthand operations in
// the same invocation ("you may not mix shorthand and major operations"),
// so replacing an existing cover still needs a separate preliminary call to
// drop the old picture block first. This is still a meaningful improvement
// over the previous 2-3-fully-independent-invocation approach: the tag
// removal and tag/cover write now happen together, so a failure can no
// longer leave the file with all tags wiped and nothing written back - the
// residual risk window (old cover removed, then the main call fails before
// importing the new one) is smaller and loses only the cover, not all
// metadata.
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

	args := h.buildTagArgs(tags)

	writeCover := len(tags.CoverData) > 0

	var replace []string
	if writeCover {
		blocks, same, err := flacFrontCoverBlocks(filepath, tags.CoverData)
		if err != nil {
			return &ErrWriteFailed{Path: filepath, Reason: err.Error()}
		}

		// Callers pass the file's own front cover back on every re-tag;
		// rewriting it would only churn the file.
		writeCover = !same
		replace = blocks
	}

	if len(args) == 0 && !writeCover {
		return nil
	}

	if writeCover {
		// --remove is a major operation and cannot be combined with the
		// shorthand operations above in one invocation - drop the replaced
		// front-cover blocks first. Back covers, artist photos etc. are kept.
		if len(replace) > 0 {
			blocks := "--block-number=" + strings.Join(replace, ",")
			exec.CommandContext(ctx, metaflac, "--remove", blocks, filepath).Run() //nolint:errcheck
		}

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

		// Format: --import-picture-from=TYPE|MIME|DESC|WIDTHxHEIGHTxDEPTH/COLORS|FILE
		// Type 3 = Front Cover, we can use simplified format
		pictureSpec := logger.JoinStrings("3|", tags.CoverMIME, "|Front Cover||", tmpPath)
		args = append(args, logger.JoinStrings("--import-picture-from=", pictureSpec))
	}

	args = append(args, filepath)

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, metaflac, args...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return &ErrWriteFailed{
			Path:   filepath,
			Reason: fmt.Sprintf("failed to write tags: %s", stderr.String()),
		}
	}

	return nil
}

// flacFrontCoverBlocks returns the metadata block numbers (metaflac
// --block-number) of the pictures a new cover replaces: every front-cover
// (type 3) picture, or the "other" (type 0) pictures when there is no front
// cover. same reports that one of them already holds exactly cover.
func flacFrontCoverBlocks(path string, cover []byte) (blocks []string, same bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer file.Close()

	br := bufio.NewReader(file)

	var sig [4]byte
	if _, err := io.ReadFull(br, sig[:]); err != nil {
		return nil, false, fmt.Errorf("reading signature: %w", err)
	}

	if sig != [4]byte{'f', 'L', 'a', 'C'} {
		return nil, false, errors.New("not a FLAC file")
	}

	var front, other []string

	var sameFront, sameOther bool

	for num := 0; ; num++ {
		block, err := meta.New(br)
		if err != nil {
			return nil, false, fmt.Errorf("reading block header: %w", err)
		}

		if block.Type != meta.TypePicture {
			block.Skip() //nolint:errcheck
		} else {
			if err := block.Parse(); err != nil {
				return nil, false, fmt.Errorf("parsing Picture: %w", err)
			}

			if pic, ok := block.Body.(*meta.Picture); ok {
				switch pic.Type {
				case 3:
					front = append(front, strconv.Itoa(num))
					sameFront = sameFront || bytes.Equal(pic.Data, cover)
				case 0:
					other = append(other, strconv.Itoa(num))
					sameOther = sameOther || bytes.Equal(pic.Data, cover)
				}
			}
		}

		if block.IsLast {
			break
		}
	}

	if len(front) > 0 {
		return front, sameFront, nil
	}

	return other, sameOther, nil
}

// buildTagArgs builds the metaflac arguments for setting tags. Only the
// fields tags sets are touched: their existing values (under every alias the
// reader accepts) are removed and the new value written; all other comments
// (ARTISTS, ORIGINALDATE, custom keys, ReplayGain from another tool, ...)
// are preserved. metaflac applies shorthand operations in argument order.
func (*FLACHandler) buildTagArgs(tags *AudioTags) []string {
	set, remove := vorbisCommentUpdate(tags)

	args := make([]string, 0, len(remove)+len(set))

	for i := range vorbisFields {
		if _, ok := remove[vorbisFields[i].keys[0]]; !ok {
			continue
		}

		for _, k := range vorbisFields[i].keys {
			args = append(args, "--remove-tag="+k)
		}
	}

	for i := range set {
		args = append(args, "--set-tag="+set[i])
	}

	return args
}
