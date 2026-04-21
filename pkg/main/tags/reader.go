// Package tags provides audio file tag reading and writing functionality
// for various audio formats including MP3, FLAC, and OGG.
package tags

import (
	"time"
)

// AudioTags represents the metadata tags from an audio file.
type AudioTags struct {
	// Standard tags
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	TrackNumber int
	TotalTracks int
	DiscNumber  int
	TotalDiscs  int
	Comment     string

	// Extended tags
	Composer   string
	Conductor  string
	Label      string
	CatalogNum string
	ISRC       string
	Lyrics     string
	Copyright  string

	// MusicBrainz IDs
	MBRecordingID    string
	MBReleaseID      string
	MBReleaseGroupID string
	MBArtistID       string
	MBAlbumArtistID  string

	// AcoustID fingerprint
	AcoustID string

	// ReplayGain values
	ReplayGainTrack     float64
	ReplayGainTrackPeak float64
	ReplayGainAlbum     float64
	ReplayGainAlbumPeak float64

	// Technical audio properties
	Duration   time.Duration
	Bitrate    int // kbps
	SampleRate int // Hz
	BitDepth   int // bits per sample
	Channels   int

	// Cover art
	CoverData []byte
	CoverMIME string
}

// TagReader defines the interface for reading audio tags from files.
type TagReader interface {
	// ReadTags reads all metadata tags from the specified audio file.
	ReadTags(filepath string) (*AudioTags, error)

	// SupportedFormats returns a list of file extensions this reader supports.
	// Extensions should include the leading dot (e.g., ".mp3", ".flac").
	SupportedFormats() []string
}

// TagWriter defines the interface for writing audio tags to files.
type TagWriter interface {
	// WriteTags writes metadata tags to the specified audio file.
	WriteTags(filepath string, tags *AudioTags) error

	// SupportedFormats returns a list of file extensions this writer supports.
	// Extensions should include the leading dot (e.g., ".mp3", ".flac").
	SupportedFormats() []string
}

// TagHandler combines both reading and writing capabilities.
type TagHandler interface {
	TagReader
	TagWriter
}

// CoverTagReader is an optional interface implemented by handlers that can read
// cover art separately from text metadata. When a handler implements this,
// CopyTags will use it to preserve cover art without paying the cost on the
// normal ReadTags path.
type CoverTagReader interface {
	ReadTagsWithCover(filepath string) (*AudioTags, error)
}

// ErrUnsupportedFormat is returned when attempting to read/write tags
// for an unsupported audio format.
type ErrUnsupportedFormat struct {
	Format string
}

func (e *ErrUnsupportedFormat) Error() string {
	return "unsupported audio format: " + e.Format
}

// ErrReadFailed is returned when tag reading fails.
type ErrReadFailed struct {
	Path   string
	Reason string
}

func (e *ErrReadFailed) Error() string {
	return "failed to read tags from " + e.Path + ": " + e.Reason
}

// ErrWriteFailed is returned when tag writing fails.
type ErrWriteFailed struct {
	Path   string
	Reason string
}

func (e *ErrWriteFailed) Error() string {
	return "failed to write tags to " + e.Path + ": " + e.Reason
}
