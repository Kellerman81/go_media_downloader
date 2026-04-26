package parser_v2

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// discIDBase64Replacer applies MusicBrainz base64 substitutions: + → .  / → _  = → -.
var discIDBase64Replacer = strings.NewReplacer("+", ".", "/", "_", "=", "-")

// CalculateDiscID computes a MusicBrainz DiscID from a list of audio files.
// It reads the duration from each file's tags and derives sector offsets using
// the standard CD sector rate (75 sectors/second) with a 150-sector lead-in.
//
// The algorithm is specified at https://musicbrainz.org/doc/Disc_ID_Calculation
//
// Note: This is an approximation — sector offsets computed from encoded audio
// durations may differ slightly from the original CD TOC. It works best for
// lossless rips (FLAC via EAC/dBpoweramp) where durations are accurate.
// Files must be provided in track order (or have TrackNumber tags set).
func CalculateDiscID(files []string) (string, error) {
	if len(files) < 2 {
		return "", fmt.Errorf("DiscID requires at least 2 tracks (got %d)", len(files))
	}

	if len(files) > 99 {
		return "", fmt.Errorf("too many tracks: %d (max 99 for DiscID)", len(files))
	}

	type trackEntry struct {
		trackNum int
		sectors  int64 // duration in CD sectors (75/sec)
	}

	entries := make([]trackEntry, 0, len(files))

	for _, f := range files {
		info := ReadTagsForFirstFile([]string{f})
		if info == nil || info.Runtime == 0 {
			return "", fmt.Errorf("could not read duration from %s", f)
		}

		sectors := int64(math.Round(info.Runtime.Seconds() * 75))
		if sectors <= 0 {
			return "", fmt.Errorf("invalid duration for %s", f)
		}

		entries = append(entries, trackEntry{
			trackNum: info.TrackNumber,
			sectors:  sectors,
		})
	}

	// Sort by track number when tags provide it; otherwise keep file order.
	if entries[0].trackNum > 0 {
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].trackNum < entries[j].trackNum
		})
	}

	// Compute cumulative sector offsets.
	// Track 1 starts at sector 150 (2-second standard CD lead-in).
	const leadIn int64 = 150

	offsets := make([]int64, len(entries))

	offsets[0] = leadIn

	for i := 1; i < len(entries); i++ {
		offsets[i] = offsets[i-1] + entries[i-1].sectors
	}

	// Leadout = last track offset + last track length.
	leadout := offsets[len(entries)-1] + entries[len(entries)-1].sectors

	// Build the 804-character SHA1 input string:
	//   2 hex  — first track number
	//   2 hex  — last track number
	//   8 hex  — offset[0] = leadout
	//   8 hex × 99 — offsets[1..N] for existing tracks, "00000000" for the rest
	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	buf.WriteHex2(1)
	buf.WriteHex2(byte(len(entries)))
	buf.WriteHex8(uint32(leadout))

	for i := range 99 {
		if i < len(entries) {
			buf.WriteHex8(uint32(offsets[i]))
		} else {
			buf.WriteString("00000000")
		}
	}

	// SHA1 → base64 with MusicBrainz substitutions: + → .   / → _   = → -
	h := sha1.Sum(buf.Bytes())
	enc := base64.StdEncoding.EncodeToString(h[:])

	enc = discIDBase64Replacer.Replace(enc)

	return enc, nil
}
