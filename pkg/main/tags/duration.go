package tags

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// NativeDuration computes an audio file's duration without spawning an external
// process (ffprobe), for the formats whose ReadTags does not already provide it:
// MP3 (no TLEN frame) and Ogg Vorbis/Opus (granule based). FLAC and MP4/M4A
// already yield duration during ReadTags, so they are not handled here.
//
// Returns ok=false when the extension isn't handled or parsing fails, letting the
// caller fall back to ffprobe.
func NativeDuration(path string) (time.Duration, bool) {
	switch logger.FileExt(path) {
	case ".mp3":
		d, err := mp3Duration(path)
		return d, err == nil && d > 0

	case ".ogg", ".oga", ".opus":
		d, err := (&OGGHandler{}).GetDuration(path)
		return d, err == nil && d > 0
	}

	return 0, false
}
