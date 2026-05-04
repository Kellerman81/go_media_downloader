// Package parser_v2 provides an extended file parsing system that handles
// multiple media types including movies, series, books, audiobooks, and music.
// It extends the original parser package with support for single-file media (books),
// multi-file collections (music albums, audiobooks), and enhanced metadata extraction.
package parser_v2

import "time"

// MediaType represents the type of media being parsed.
type MediaType uint8

const (
	// MediaTypeUnknown indicates the media type could not be determined.
	MediaTypeUnknown MediaType = iota
	// MediaTypeMovie indicates a movie file.
	MediaTypeMovie
	// MediaTypeSeries indicates a TV series episode.
	MediaTypeSeries
	// MediaTypeBook indicates an ebook file.
	MediaTypeBook
	// MediaTypeAudiobook indicates an audiobook (single or multi-file).
	MediaTypeAudiobook
	// MediaTypeMusic indicates a music album or track.
	MediaTypeMusic
)

// String returns the string representation of the media type.
func (mt MediaType) String() string {
	switch mt {
	case MediaTypeMovie:
		return "movie"
	case MediaTypeSeries:
		return "series"
	case MediaTypeBook:
		return "book"
	case MediaTypeAudiobook:
		return "audiobook"
	case MediaTypeMusic:
		return "music"
	default:
		return "unknown"
	}
}

// ParseResult contains the common result fields for all media types.
type ParseResult struct {
	// Title is the extracted title of the media.
	Title string `json:"title,omitempty"`
	// Year is the release year.
	Year int `json:"year,omitempty"`
	// MediaType indicates what type of media was detected.
	MediaType MediaType `json:"media_type"`
	// Confidence indicates how confident the parser is (0.0-1.0).
	Confidence float64 `json:"confidence,omitempty"`
	// SourceFile is the original filename parsed.
	SourceFile string `json:"source_file,omitempty"`
	// SourcePath is the full path to the file.
	SourcePath string `json:"source_path,omitempty"`
}

// VideoParseResult extends ParseResult with video-specific fields.
type VideoParseResult struct {
	ParseResult

	// Resolution is the video resolution (e.g., "1080p", "4k").
	Resolution string `json:"resolution,omitempty"`
	// Quality is the video quality (e.g., "BluRay", "WEB-DL").
	Quality string `json:"quality,omitempty"`
	// Codec is the video codec (e.g., "x264", "x265").
	Codec string `json:"codec,omitempty"`
	// Audio is the audio codec/format (e.g., "DTS", "AAC").
	Audio string `json:"audio,omitempty"`

	// Extended indicates if this is an extended version.
	Extended bool `json:"extended,omitempty"`
	// Proper indicates if this is a proper release.
	Proper bool `json:"proper,omitempty"`
	// Repack indicates if this is a repack release.
	Repack bool `json:"repack,omitempty"`

	// Imdb is the IMDB ID (for movies).
	Imdb string `json:"imdb,omitempty"`
	// Tvdb is the TVDB ID (for series).
	Tvdb string `json:"tvdb,omitempty"`

	// Season is the season number (for series).
	Season int `json:"season,omitempty"`
	// Episode is the episode number (for series).
	Episode int `json:"episode,omitempty"`
	// AbsoluteEpisode is the absolute episode number (e.g., for "E643").
	AbsoluteEpisode int `json:"absolute_episode,omitempty"`
	// Identifier is the episode identifier (e.g., "S01E05").
	Identifier string `json:"identifier,omitempty"`

	// Runtime is the video duration in seconds.
	Runtime int `json:"runtime,omitempty"`
	// Height is the video height in pixels.
	Height int `json:"height,omitempty"`
	// Width is the video width in pixels.
	Width int `json:"width,omitempty"`

	// ResolutionID is the database ID for resolution.
	ResolutionID uint `json:"resolution_id,omitempty"`
	// QualityID is the database ID for quality.
	QualityID uint `json:"quality_id,omitempty"`
	// CodecID is the database ID for codec.
	CodecID uint `json:"codec_id,omitempty"`
	// AudioID is the database ID for audio.
	AudioID uint `json:"audio_id,omitempty"`

	// ReleaseGroup is the release group name.
	ReleaseGroup string `json:"release_group,omitempty"`
}

// BookParseResult contains parsed information for book files.
type BookParseResult struct {
	ParseResult

	// Author is the extracted author name.
	Author string `json:"author,omitempty"`
	// Authors is a list of authors for multi-author works.
	Authors []string `json:"authors,omitempty"`
	// ISBN13 is the 13-digit ISBN.
	ISBN13 string `json:"isbn_13,omitempty"`
	// ISBN10 is the 10-digit ISBN.
	ISBN10 string `json:"isbn_10,omitempty"`
	// ASIN is the Amazon Standard Identification Number.
	ASIN string `json:"asin,omitempty"`

	// Publisher is the book publisher.
	Publisher string `json:"publisher,omitempty"`
	// Language is the book language code.
	Language string `json:"language,omitempty"`

	// Series is the series name if part of a series.
	Series string `json:"series,omitempty"`
	// SeriesPosition is the position in the series (e.g., "1", "2.5").
	SeriesPosition string `json:"series_position,omitempty"`

	// Format is the ebook format (e.g., "epub", "pdf", "mobi").
	Format string `json:"format,omitempty"`

	// IsRetail indicates if this is a retail (non-scene) release.
	IsRetail bool `json:"is_retail,omitempty"`
	// ReleaseGroup is the release group for scene releases.
	ReleaseGroup string `json:"release_group,omitempty"`
}

// AudiobookParseResult contains parsed information for audiobook files.
type AudiobookParseResult struct {
	ParseResult

	// Author is the book author.
	Author string `json:"author,omitempty"`
	// Authors is a list of authors for multi-author works.
	Authors []string `json:"authors,omitempty"`
	// Narrator is the audiobook narrator.
	Narrator string `json:"narrator,omitempty"`
	// Narrators is a list of narrators for multi-narrator works.
	Narrators []string `json:"narrators,omitempty"`

	// ASIN is the Audible ASIN.
	ASIN string `json:"asin,omitempty"`
	// ISBN13 is the 13-digit ISBN of the print edition.
	ISBN13 string `json:"isbn_13,omitempty"`

	// Series is the series name if part of a series.
	Series string `json:"series,omitempty"`
	// SeriesPosition is the position in the series.
	SeriesPosition string `json:"series_position,omitempty"`

	// Format is the audio format (e.g., "m4b", "mp3", "flac").
	Format string `json:"format,omitempty"`
	// Bitrate is the audio bitrate in kbps.
	Bitrate int `json:"bitrate,omitempty"`
	// SampleRate is the audio sample rate in Hz.
	SampleRate int `json:"sample_rate,omitempty"`

	// RuntimeMS is the total runtime in milliseconds.
	RuntimeMS int64 `json:"runtime_ms,omitempty"`

	// IsMultiFile indicates if this is a multi-file audiobook.
	IsMultiFile bool `json:"is_multi_file,omitempty"`
	// Files contains information about individual files.
	Files []AudiobookFileInfo `json:"files,omitempty"`
	// TotalParts is the expected number of parts/files.
	TotalParts int `json:"total_parts,omitempty"`
	// MissingParts lists missing part numbers.
	MissingParts []int `json:"missing_parts,omitempty"`

	// Abridged indicates if this is an abridged version.
	Abridged bool `json:"abridged,omitempty"`
	// ReleaseGroup is the release group name.
	ReleaseGroup string `json:"release_group,omitempty"`
}

// AudiobookFileInfo contains information about a single audiobook file.
type AudiobookFileInfo struct {
	// Filename is the name of the file.
	Filename string `json:"filename"`
	// PartNumber is the part/track number.
	PartNumber int `json:"part_number,omitempty"`
	// DiscNumber is the disc number for multi-disc sets.
	DiscNumber int `json:"disc_number,omitempty"`
	// RuntimeMS is the runtime in milliseconds.
	RuntimeMS int64 `json:"runtime_ms,omitempty"`
	// ChapterTitle is the chapter title if available.
	ChapterTitle string `json:"chapter_title,omitempty"`
}

// MusicParseResult contains parsed information for music files/albums.
type MusicParseResult struct {
	ParseResult

	// Artist is the primary artist name.
	Artist string `json:"artist,omitempty"`
	// Artists is a list of artists for collaborations.
	Artists []string `json:"artists,omitempty"`
	// AlbumArtist is the album artist (may differ from track artist).
	AlbumArtist string `json:"album_artist,omitempty"`
	// Album is the album name.
	Album string `json:"album,omitempty"`

	// ReleaseType is the type of release (album, ep, single, compilation).
	ReleaseType string `json:"release_type,omitempty"`
	// Label is the record label.
	Label string `json:"label,omitempty"`
	// CatalogNumber is the label catalog number.
	CatalogNumber string `json:"catalog_number,omitempty"`

	// MusicBrainzReleaseID is the MusicBrainz release UUID.
	MusicBrainzReleaseID string `json:"musicbrainz_release_id,omitempty"`
	// MusicBrainzReleaseGroupID is the MusicBrainz release group UUID.
	MusicBrainzReleaseGroupID string `json:"musicbrainz_release_group_id,omitempty"`
	// DiscogsReleaseID is the Discogs release ID.
	DiscogsReleaseID int `json:"discogs_release_id,omitempty"`
	// UPC is the Universal Product Code.
	UPC string `json:"upc,omitempty"`

	// Format is the audio format (e.g., "flac", "mp3", "m4a").
	Format string `json:"format,omitempty"`
	// Bitrate is the audio bitrate in kbps.
	Bitrate int `json:"bitrate,omitempty"`
	// SampleRate is the audio sample rate in Hz.
	SampleRate int `json:"sample_rate,omitempty"`
	// BitDepth is the audio bit depth.
	BitDepth int `json:"bit_depth,omitempty"`

	// TotalRuntimeMS is the total album runtime in milliseconds.
	TotalRuntimeMS int64 `json:"total_runtime_ms,omitempty"`
	// TotalTracks is the total number of tracks.
	TotalTracks int `json:"total_tracks,omitempty"`
	// TotalDiscs is the total number of discs.
	TotalDiscs int `json:"total_discs,omitempty"`

	// Tracks contains information about individual tracks.
	Tracks []TrackInfo `json:"tracks,omitempty"`
	// IsComplete indicates if all tracks are present.
	IsComplete bool `json:"is_complete,omitempty"`
	// MissingTracks lists missing track numbers.
	MissingTracks []int `json:"missing_tracks,omitempty"`

	// Genre is the primary genre.
	Genre string `json:"genre,omitempty"`
	// Genres is a list of genres.
	Genres []string `json:"genres,omitempty"`

	// ReleaseGroup is the release group name.
	ReleaseGroup string `json:"release_group,omitempty"`
	// IsLossless indicates if the audio is lossless.
	IsLossless bool `json:"is_lossless,omitempty"`
}

// TrackInfo contains information about a single music track.
type TrackInfo struct {
	Filepath string // Full path to the audio file
	// Filename is the name of the file.
	Filename  string `json:"filename"`
	Extension string // File extension (e.g., ".mp3", ".flac")
	// Title is the track title.
	Title string `json:"title,omitempty"`
	// TrackNumber is the track number.
	TrackNumber int `json:"track_number,omitempty"`
	// DiscNumber is the disc number.
	DiscNumber int `json:"disc_number,omitempty"`
	// RuntimeMS is the track runtime in milliseconds.
	RuntimeMS int64 `json:"runtime_ms,omitempty"`
	// Artist is the track artist (for featured artists).
	Artist         string        `json:"artist,omitempty"`
	Album          string        // Album or book title
	AlbumArtist    string        // Album artist (may differ from track artist)
	Year           int           // Release year
	Genre          string        // Genre
	Runtime        time.Duration // Track duration
	Format         string        // Audio format (mp3, flac, m4a, etc.)
	Bitrate        int           // Bitrate in kbps
	SampleRate     int           // Sample rate in Hz
	BitDepth       int           // Bit depth (16, 24, 32)
	Channels       int           // Number of audio channels
	FileSize       int64         // File size in bytes
	QualityProfile string        // Quality profile identifier
	MusicBrainzID  string        // MusicBrainz recording ID
	// ISRC is the International Standard Recording Code.
	ISRC string `json:"isrc,omitempty"`
	// AcoustID is the audio fingerprint ID.
	AcoustID string `json:"acoustid,omitempty"`
	// DiscID is the MusicBrainz disc ID embedded by ripping software.
	DiscID string `json:"discid,omitempty"`
	// MusicBrainzRecordingID is the MusicBrainz recording UUID.
	MusicBrainzRecordingID string `json:"musicbrainz_recording_id,omitempty"`
	// For audiobooks specifically
	Narrator   string // Narrator name
	Series     string // Book series name
	SeriesNum  string // Position in series
	ASIN       string // Amazon Standard Identification Number
	AudibleID  string // Audible-specific ID
	IsChapter  bool   // True if this represents a chapter marker
	ChapterNum int    // Chapter number if IsChapter

	// Set during matching / filename generation — reused downstream to avoid
	// re-querying the DB.
	ExpectedRuntimeMS int64   // Expected track runtime from DB (0 = unknown)
	TrackDist         float64 // Distance score from matching (0 = unknown)
	GeneratedFilename string  // Target filename after rename (empty = unchanged)
}

// regexPattern represents a compiled regex pattern for parsing.
// type regexPattern struct {
// 	name     string
// 	re       *regexp.Regexp
// 	getgroup int
// 	last     bool
// }

// patternCache caches compiled regex patterns.
// type patternCache struct {
// 	mu       sync.RWMutex
// 	patterns map[string]*regexp.Regexp
// }

// globalPatternCache is the global pattern cache instance.
// var globalPatternCache = &patternCache{
// 	patterns: make(map[string]*regexp.Regexp),
// }

// getOrCompile retrieves a cached pattern or compiles and caches a new one.
// func (pc *patternCache) getOrCompile(pattern string) (*regexp.Regexp, error) {
// 	pc.mu.RLock()
// 	if re, ok := pc.patterns[pattern]; ok {
// 		pc.mu.RUnlock()
// 		return re, nil
// 	}
// 	pc.mu.RUnlock()

// 	pc.mu.Lock()
// 	defer pc.mu.Unlock()

// 	// Double-check after acquiring write lock
// 	if re, ok := pc.patterns[pattern]; ok {
// 		return re, nil
// 	}

// 	re, err := regexp.Compile(pattern)
// 	if err != nil {
// 		return nil, err
// 	}

// 	pc.patterns[pattern] = re
// 	return re, nil
// }

// RuntimeMatcher handles runtime-based matching with configurable tolerance.
type RuntimeMatcher struct {
	// TolerancePercent is the percentage tolerance (e.g., 0.03 for 3%).
	TolerancePercent float64
	// ToleranceMinMS is the minimum tolerance in milliseconds.
	ToleranceMinMS int64
	// ToleranceMaxMS is the maximum tolerance in milliseconds.
	ToleranceMaxMS int64
}

// DefaultRuntimeMatcher returns a RuntimeMatcher with sensible defaults.
func DefaultRuntimeMatcher() *RuntimeMatcher {
	return &RuntimeMatcher{
		TolerancePercent: 0.03,   // 3% tolerance
		ToleranceMinMS:   5000,   // At least 5 seconds
		ToleranceMaxMS:   300000, // At most 5 minutes
	}
}

// MatchRuntime checks if the actual runtime matches the expected within tolerance.
func (rm *RuntimeMatcher) MatchRuntime(expectedMS, actualMS int64) bool {
	diff := abs64(expectedMS - actualMS)
	tolerance := rm.calculateTolerance(expectedMS)
	return diff <= tolerance
}

// MatchTotalRuntime checks if total runtime from tracks matches expected.
// Returns match status and confidence (0.0-1.0).
func (rm *RuntimeMatcher) MatchTotalRuntime(expectedMS int64, tracks []TrackInfo) (bool, float64) {
	var totalActual int64
	for i := range tracks {
		totalActual += tracks[i].RuntimeMS
	}

	diff := abs64(expectedMS - totalActual)
	tolerance := rm.calculateTolerance(expectedMS)

	if diff <= tolerance {
		confidence := 1.0 - (float64(diff) / float64(tolerance))
		return true, confidence
	}

	return false, 0
}

// calculateTolerance determines the tolerance value for a given runtime.
func (rm *RuntimeMatcher) calculateTolerance(runtimeMS int64) int64 {
	percentTolerance := int64(float64(runtimeMS) * rm.TolerancePercent)

	if percentTolerance < rm.ToleranceMinMS {
		return rm.ToleranceMinMS
	}

	if percentTolerance > rm.ToleranceMaxMS {
		return rm.ToleranceMaxMS
	}

	return percentTolerance
}

// abs64 returns the absolute value of an int64.
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}

	return x
}

// File extension constants for different media types.
var (
	// VideoExtensions are common video file extensions.
	VideoExtensions = []string{
		".mkv", ".mp4", ".avi", ".wmv", ".mov", ".m4v", ".mpg", ".mpeg",
		".ts", ".m2ts", ".webm", ".flv", ".vob", ".ogv", ".divx",
	}

	// BookExtensions are common ebook file extensions.
	BookExtensions = []string{
		".epub", ".pdf", ".mobi", ".azw", ".azw3", ".fb2", ".lit",
		".djvu", ".cbz", ".cbr", ".txt", ".rtf", ".doc", ".docx",
	}

	// AudioExtensions are common audio file extensions.
	AudioExtensions = []string{
		".mp3", ".flac", ".m4a", ".m4b", ".aac", ".ogg", ".opus",
		".wav", ".wma", ".ape", ".alac", ".aiff", ".wv",
	}

	// AudiobookExtensions are common audiobook file extensions.
	AudiobookExtensions = []string{
		".m4b", ".mp3", ".m4a", ".flac", ".ogg", ".opus", ".wma", ".aax",
	}

	// LosslessAudioExtensions are lossless audio formats.
	LosslessAudioExtensions = []string{
		".flac", ".alac", ".wav", ".aiff", ".ape", ".wv",
	}
)

// IsVideoExtension checks if the extension is a video extension.
func IsVideoExtension(ext string) bool {
	return containsIgnoreCase(VideoExtensions, ext)
}

// IsBookExtension checks if the extension is an ebook extension.
func IsBookExtension(ext string) bool {
	return containsIgnoreCase(BookExtensions, ext)
}

// IsAudioExtension checks if the extension is an audio extension.
func IsAudioExtension(ext string) bool {
	return containsIgnoreCase(AudioExtensions, ext)
}

// IsAudiobookExtension checks if the extension is an audiobook extension.
func IsAudiobookExtension(ext string) bool {
	return containsIgnoreCase(AudiobookExtensions, ext)
}

// IsLosslessAudioExtension checks if the extension is a lossless audio extension.
func IsLosslessAudioExtension(ext string) bool {
	return containsIgnoreCase(LosslessAudioExtensions, ext)
}

// containsIgnoreCase checks if the slice contains the string (case-insensitive).
func containsIgnoreCase(slice []string, str string) bool {
	for i := range slice {
		if len(slice[i]) == len(str) && (slice[i] == str || equalFoldASCII(slice[i], str)) {
			return true
		}
	}

	return false
}

// equalFoldASCII is a fast ASCII-only case-insensitive comparison.
func equalFoldASCII(s, t string) bool {
	if len(s) != len(t) {
		return false
	}

	for i := range len(s) {
		sc := s[i]

		tc := t[i]
		if sc == tc {
			continue
		}

		// Make lowercase
		if sc >= 'A' && sc <= 'Z' {
			sc += 'a' - 'A'
		}

		if tc >= 'A' && tc <= 'Z' {
			tc += 'a' - 'A'
		}

		if sc != tc {
			return false
		}
	}

	return true
}
