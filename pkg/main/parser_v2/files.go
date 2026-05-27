package parser_v2

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
	"github.com/goccy/go-json"
)

// AlbumInfo groups tracks together as an album or audiobook.
type AlbumInfo struct {
	Title         string        // Album or book title
	Artist        string        // Primary artist or author
	AlbumArtist   string        // Album artist
	Year          int           // Release year
	Genre         string        // Primary genre
	Tracks        []TrackInfo   // All tracks in order
	TotalRuntime  time.Duration // Sum of all track runtimes
	DiscCount     int           // Number of discs
	TrackCount    int           // Total number of tracks
	IsComplete    bool          // True if all expected tracks are present
	MissingTracks []int         // Track numbers that are missing
	SourceFolder  string        // Original folder path

	// For audiobooks
	Narrator  string // Primary narrator
	Series    string // Series name
	SeriesNum string // Position in series
	ASIN      string // Amazon ID
	Abridged  bool   // True if abridged version

	// For music
	Label         string // Record label
	CatalogNumber string // Catalog number
	ReleaseType   string // album, ep, single, compilation
	Country       string // Release country

	// Matching info
	ExpectedTracks  int           // Expected number of tracks from database
	ExpectedRuntime time.Duration // Expected total runtime from database
	MatchScore      float64       // How well this matches the expected data
	DatabaseID      uint          // Matched database ID (0 if unmatched)
}

// ffprobeAudioResult represents the JSON output from ffprobe for audio files
// Used only for formats not supported by the tags package (M4A, M4B, etc.)
type ffprobeAudioResult struct {
	Format struct {
		Filename string            `json:"filename"`
		Duration string            `json:"duration"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType  string            `json:"codec_type"`
		CodecName  string            `json:"codec_name"`
		SampleRate string            `json:"sample_rate"`
		Channels   int               `json:"channels"`
		BitRate    string            `json:"bit_rate"`
		Duration   string            `json:"duration"`
		Tags       map[string]string `json:"tags"`
	} `json:"streams"`
}

// Common errors.
var (
	plTrackInfo pool.Poolobj[TrackInfo]

	ErrFileTooSmall        = errors.New("file is too small")
	ErrFileTooLarge        = errors.New("file is too large")
	ErrFileTooNew          = errors.New("file was modified too recently")
	ErrInvalidPath         = errors.New("invalid path")
	ErrExtensionNotAllowed = errors.New("file extension not allowed")
	// Year in folder/album name.
	yearInNamePattern = regexp.MustCompile(`\((\d{4})\)`)
	// Track patterns for music/audiobooks
	// "01 - Track Title.mp3".
	trackPatternDashTitle = regexp.MustCompile(`(?i)^(\d{1,3})\s*[-._]\s*(.+?)\.(\w+)$`)

	// "01. Track Title.mp3".
	trackPatternDotTitle = regexp.MustCompile(`(?i)^(\d{1,3})\.\s*(.+?)\.(\w+)$`)

	// "1-01 Track Title.flac" (disc-track).
	trackPatternDiscTrack = regexp.MustCompile(`(?i)^(\d{1,2})[-.](\d{1,3})\s+(.+?)\.(\w+)$`)

	// "CD1/01 - Track Title.mp3" or "Disc 1/01 - Track.mp3".
	trackPatternCDFolder = regexp.MustCompile(`(?i)(?:CD|Disc|Part)\s*(\d+)`)

	// "Artist - Album - 01 - Track.mp3".
	trackPatternArtistAlbumTrack = regexp.MustCompile(
		`(?i)^(.+?)\s*-\s*(.+?)\s*-\s*(\d{1,3})\s*[-._]\s*(.+?)\.(\w+)$`,
	)

	// Audiobook specific patterns
	// "Author - Book Title - Chapter 01.m4b".
	audiobookPatternChapter = regexp.MustCompile(
		`(?i)^(.+?)\s*-\s*(.+?)\s*-\s*(?:Chapter|Ch\.?|Part|Pt\.?)\s*(\d+).*?\.(\w+)$`,
	)

	// "Book Title - Part 01.mp3".
	audiobookPatternPart = regexp.MustCompile(
		`(?i)^(.+?)\s*-\s*(?:Part|Pt\.?|Chapter|Ch\.?)\s*(\d+).*?\.(\w+)$`,
	)

	// "(01) Chapter Title.mp3" - common audiobook format.
	audiobookPatternParenNum = regexp.MustCompile(`(?i)^\((\d+)\)\s*(.+?)\.(\w+)$`)

	// losslessFormats lists all lossless audio format names (lowercase, without dot).
	losslessFormats = []string{"flac", "alac", "wav", "aiff", "ape", "wv", "wavpack", "dsd", "dsf"}

	// musicParserInstance is a singleton MusicParser for folder name parsing.
	musicParserInstance *MusicParser
)

// WalkFiles walks a directory and returns all files matching the given extensions.
// It skips subdirectories starting with "_" (like "_unpack"), but NOT the root folder.
// Returns files sorted by path.
func WalkFiles(folder string, extensions []string) ([]string, error) {
	if folder == "" {
		return nil, ErrInvalidPath
	}

	var files []string

	err := filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip subdirectories starting with underscore (but not the root folder)
		if d.IsDir() {
			name := d.Name()
			// Only skip if it's a subdirectory (not the root) and starts with underscore
			if len(name) > 0 && name[0] == '_' && path != folder {
				return filepath.SkipDir
			}

			return nil
		}

		// Check if file has an allowed extension
		if len(extensions) > 0 && !HasExtension(path, extensions) {
			return nil
		}

		files = append(files, path)

		return nil
	})

	return files, err
}

// CollectFilesOnly walks a folder and returns file paths without reading tags.
// This is the first step in the optimized matching process - we collect files first,
// parse metadata from filename/folder, and only read tags if needed.
func CollectFilesOnly(folder string, extensions []string) ([]string, error) {
	if len(extensions) == 0 {
		extensions = AudioExtensions
	}

	return WalkFiles(folder, extensions)
}

// HasExtension checks if a filename has one of the given extensions.
func HasExtension(filename string, extensions []string) bool {
	for i := range extensions {
		if len(filename) <= len(extensions[i]) {
			continue
		}

		suffix := filename[len(filename)-len(extensions[i]):]
		if equalFoldASCII(suffix, extensions[i]) {
			return true
		}
	}

	return false
}

// getMusicParser returns a singleton MusicParser instance.
func getMusicParser() *MusicParser {
	if musicParserInstance == nil {
		musicParserInstance = NewMusicParser()
	}

	return musicParserInstance
}

// ParseAudioFolder extracts album/book info from a folder name.
// Uses the same parsing logic as MusicParser.ParseAlbumTitle for consistency.
func ParseAudioFolder(folderPath string) (artist, album string, year int) {
	folderName := filepath.Base(folderPath)

	// Use MusicParser for consistent parsing with NZB title parsing
	mp := getMusicParser()
	result := mp.ParseAlbumTitle(folderName)

	artist = result.Artist
	album = result.Album
	year = result.Year

	return
}

// ParseASINFromPath attempts to extract ASIN from an audiobook folder path.
func ParseASINFromPath(folderPath string) string {
	// Split path into parts
	parts := strings.Split(folderPath, string('/'))
	if len(parts) < 2 {
		parts = strings.Split(folderPath, string('\\'))
	}

	// Check the last part (folder name) for ASIN
	if len(parts) > 0 {
		folderName := parts[len(parts)-1]
		return extractASINFromString(folderName)
	}

	return ""
}

// extractASINFromString finds an ASIN pattern in a string.
func extractASINFromString(s string) string {
	words := strings.FieldsSeq(s)
	for word := range words {
		// Clean up the word
		word = strings.Trim(word, "()[]")
		if isValidASIN(word) {
			return word
		}
	}

	return ""
}

// isValidASIN checks if a string looks like an ASIN.
func isValidASIN(s string) bool {
	if len(s) != 10 {
		return false
	}

	// ASINs starting with B are alphanumeric
	if s[0] == 'B' {
		for _, c := range s[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
				return false
			}
		}

		return true
	}

	// Numeric ASINs (10 digits)
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// ParseAudioFilename extracts metadata from an audio filename.
// It handles both music tracks and audiobook chapters.
func ParseAudioFilename(filename string) *TrackInfo {
	// Get just the filename and parent folder
	base := filepath.Base(filename)
	ext := filepath.Ext(base)

	info := &TrackInfo{
		Filepath:    filename,
		Filename:    base,
		Extension:   strings.ToLower(ext),
		DiscNumber:  1,
		TrackNumber: 0,
		Format:      logger.ExtToFormat(ext),
	}

	parentDir := filepath.Base(filepath.Dir(filename))

	// Check parent folder for disc number
	if matches := trackPatternCDFolder.FindStringSubmatch(parentDir); len(matches) > 1 {
		info.DiscNumber, _ = strconv.Atoi(matches[1])
	}

	// Try audiobook patterns first (more specific)
	if parseAudiobookChapterPattern(base, info) {
		return info
	}

	if parseAudiobookPartPattern(base, info) {
		return info
	}

	if parseAudiobookParenNumPattern(base, info) {
		return info
	}

	// Try music patterns
	if parseArtistAlbumTrackPattern(base, info) {
		return info
	}

	if parseDiscTrackPattern(base, info) {
		return info
	}

	if parseDashTitlePattern(base, info) {
		return info
	}

	if parseDotTitlePattern(base, info) {
		return info
	}

	// Fallback: try to extract just a track number from the start
	if matches := database.GetCachedRegexp(`^(\d{1,3})`).
		FindStringSubmatch(base); len(
		matches,
	) > 1 {
		info.TrackNumber, _ = strconv.Atoi(matches[1])
		// Remove track number and extension for title
		title := strings.TrimPrefix(base, matches[0])

		title = strings.TrimSuffix(title, filepath.Ext(title))
		title = strings.TrimLeft(title, " -._")
		info.Title = strings.TrimSpace(title)
	} else {
		// No track number found, use filename as title
		info.Title = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return info
}

// parseAudiobookChapterPattern handles "Author - Book Title - Chapter 01.m4b".
func parseAudiobookChapterPattern(filename string, info *TrackInfo) bool {
	matches := audiobookPatternChapter.FindStringSubmatch(filename)
	if len(matches) < 5 {
		return false
	}

	info.Artist = strings.TrimSpace(matches[1]) // Author
	info.Album = strings.TrimSpace(matches[2])  // Book title
	info.TrackNumber, _ = strconv.Atoi(matches[3])
	info.Title = "Chapter " + matches[3]
	info.IsChapter = true
	info.ChapterNum = info.TrackNumber

	return true
}

// parseAudiobookPartPattern handles "Book Title - Part 01.mp3".
func parseAudiobookPartPattern(filename string, info *TrackInfo) bool {
	matches := audiobookPatternPart.FindStringSubmatch(filename)
	if len(matches) < 4 {
		return false
	}

	info.Album = strings.TrimSpace(matches[1])
	info.TrackNumber, _ = strconv.Atoi(matches[2])
	info.Title = "Part " + matches[2]
	info.IsChapter = true
	info.ChapterNum = info.TrackNumber

	return true
}

// parseAudiobookParenNumPattern handles "(01) Chapter Title.mp3".
func parseAudiobookParenNumPattern(filename string, info *TrackInfo) bool {
	matches := audiobookPatternParenNum.FindStringSubmatch(filename)
	if len(matches) < 4 {
		return false
	}

	info.TrackNumber, _ = strconv.Atoi(matches[1])
	info.Title = strings.TrimSpace(matches[2])
	info.IsChapter = true
	info.ChapterNum = info.TrackNumber

	return true
}

// parseArtistAlbumTrackPattern handles "Artist - Album - 01 - Track.mp3".
func parseArtistAlbumTrackPattern(filename string, info *TrackInfo) bool {
	matches := trackPatternArtistAlbumTrack.FindStringSubmatch(filename)
	if len(matches) < 6 {
		return false
	}

	info.Artist = strings.TrimSpace(matches[1])
	info.Album = strings.TrimSpace(matches[2])
	info.TrackNumber, _ = strconv.Atoi(matches[3])
	info.Title = strings.TrimSpace(matches[4])

	// Extract year from album if present
	if yearMatches := yearInNamePattern.FindStringSubmatch(info.Album); len(yearMatches) > 1 {
		info.Year, _ = strconv.Atoi(yearMatches[1])
		info.Album = strings.TrimSpace(yearInNamePattern.ReplaceAllLiteralString(info.Album, ""))
	}

	return true
}

// parseDiscTrackPattern handles "1-01 Track Title.flac".
func parseDiscTrackPattern(filename string, info *TrackInfo) bool {
	matches := trackPatternDiscTrack.FindStringSubmatch(filename)
	if len(matches) < 5 {
		return false
	}

	info.DiscNumber, _ = strconv.Atoi(matches[1])
	info.TrackNumber, _ = strconv.Atoi(matches[2])
	info.Title = strings.TrimSpace(matches[3])

	return true
}

// parseDashTitlePattern handles "01 - Track Title.mp3".
func parseDashTitlePattern(filename string, info *TrackInfo) bool {
	matches := trackPatternDashTitle.FindStringSubmatch(filename)
	if len(matches) < 4 {
		return false
	}

	info.TrackNumber, _ = strconv.Atoi(matches[1])
	info.Title = strings.TrimSpace(matches[2])

	// Check if title contains artist info "Artist - Title"
	if parts := strings.SplitN(info.Title, " - ", 2); len(parts) == 2 {
		info.Artist = strings.TrimSpace(parts[0])
		info.Title = strings.TrimSpace(parts[1])
	}

	return true
}

// parseDotTitlePattern handles "01. Track Title.mp3".
func parseDotTitlePattern(filename string, info *TrackInfo) bool {
	matches := trackPatternDotTitle.FindStringSubmatch(filename)
	if len(matches) < 4 {
		return false
	}

	info.TrackNumber, _ = strconv.Atoi(matches[1])
	info.Title = strings.TrimSpace(matches[2])

	return true
}

// ReadTagsForFirstFile reads tags from files until it finds one with valid artist/album info.
// Tries up to 5 files to find one with non-empty tags.
// Returns the TrackInfo with tag data, or nil if no valid tags are found.
func ReadTagsForFirstFile(files []string) *TrackInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return readTagsForFirstFileCtx(ctx, files)
}

func readTagsForFirstFileCtx(ctx context.Context, files []string) *TrackInfo {
	if len(files) == 0 {
		return nil
	}

	// Try up to 5 files to find one with valid tags
	maxTries := min(len(files), 5)

	for i := range maxTries {
		track, err := readAudioTagsCtx(ctx, files[i])
		if err != nil {
			continue
		}

		// Check if we have useful tag data (artist or album)
		if track.Artist != "" || track.AlbumArtist != "" || track.Album != "" {
			return track
		}
	}

	// If no files have valid tags, return the first one anyway (might have MusicBrainzID)
	track, err := readAudioTagsCtx(ctx, files[0])
	if err != nil {
		return nil
	}

	return track
}

func init() {
	plTrackInfo.Init(50, 5, func(t *TrackInfo) {
		*t = TrackInfo{DiscNumber: 1}
	}, func(t *TrackInfo) bool {
		*t = TrackInfo{DiscNumber: 1}
		return false
	})
}

// PutTrackInfo returns a TrackInfo obtained from ReadAudioTags back to the pool.
// Call this after you have finished reading all fields you need from the struct.
func PutTrackInfo(t *TrackInfo) {
	if t != nil {
		plTrackInfo.Put(t)
	}
}

// ReadAudioTags reads metadata tags from an audio file.
// It uses the native tags package for supported formats (MP3, FLAC, OGG)
// and falls back to ffprobe for other formats (M4A, M4B, etc.)
func ReadAudioTags(filePath string) (*TrackInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return readAudioTagsCtx(ctx, filePath)
}

func readAudioTagsCtx(ctx context.Context, filePath string) (*TrackInfo, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	info := plTrackInfo.Get()

	*info = TrackInfo{
		Filepath:   filePath,
		Filename:   filepath.Base(filePath),
		Extension:  ext,
		DiscNumber: 1,
	}

	// Get file size
	if fileInfo, err := os.Stat(filePath); err == nil {
		info.FileSize = fileInfo.Size()
	}

	// Try using the native tags package first for supported formats
	if tags.IsSupported(filePath) {
		audioTags, err := tags.ReadTags(filePath)
		if err == nil {
			// Convert tags.AudioTags to TrackInfo
			info.Title = audioTags.Title
			info.Artist = audioTags.Artist
			info.Album = audioTags.Album
			info.AlbumArtist = audioTags.AlbumArtist
			info.Genre = audioTags.Genre
			info.Year = audioTags.Year
			info.TrackNumber = audioTags.TrackNumber

			info.DiscNumber = audioTags.DiscNumber
			if info.DiscNumber == 0 {
				info.DiscNumber = 1
			}

			info.Runtime = audioTags.Duration
			info.RuntimeMS = audioTags.Duration.Milliseconds()
			info.Bitrate = audioTags.Bitrate
			info.SampleRate = audioTags.SampleRate
			info.BitDepth = audioTags.BitDepth
			info.Channels = audioTags.Channels

			// Get format from extension
			info.Format = strings.TrimPrefix(ext, ".")

			// Derive quality profile
			info.QualityProfile = deriveQualityProfile(info)

			// If no tags found, fall back to filename parsing
			if info.Title == "" && info.Artist == "" && info.Album == "" {
				parsed := ParseAudioFilename(filePath)

				info.Title = parsed.Title
				info.Artist = parsed.Artist

				info.Album = parsed.Album
				if info.TrackNumber == 0 {
					info.TrackNumber = parsed.TrackNumber
				}

				if info.DiscNumber == 0 || info.DiscNumber == 1 {
					info.DiscNumber = parsed.DiscNumber
				}
			}

			// If native tags succeeded but duration is missing, get it from ffprobe
			if info.RuntimeMS == 0 {
				if ffResult, ffErr := runFFProbeAudio(
					ctx, filePath,
				); ffErr == nil &&
					ffResult.Format.Duration != "" {
					if dur, parseErr := strconv.ParseFloat(
						ffResult.Format.Duration,
						64,
					); parseErr == nil {
						info.Runtime = time.Duration(dur * float64(time.Second))
						info.RuntimeMS = info.Runtime.Milliseconds()
					}
				}
			}

			return info, nil
		}

		// Log error but continue to ffprobe fallback
		logger.Logtype("debug", 0).
			Str("file", filePath).
			Err(err).
			Msg("Native tag reading failed, trying ffprobe")
	}

	// Fall back to ffprobe for unsupported formats (M4A, M4B, AAC, etc.)
	result, err := runFFProbeAudio(ctx, filePath)
	if err != nil {
		return nil, err
	}

	// Parse format-level tags
	parseFFProbeTags(info, result.Format.Tags)

	// Parse duration
	if result.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			info.Runtime = time.Duration(dur * float64(time.Second))
			info.RuntimeMS = info.Runtime.Milliseconds()
		}
	}

	// Parse bitrate
	if result.Format.BitRate != "" {
		if br, err := strconv.Atoi(result.Format.BitRate); err == nil {
			info.Bitrate = br / 1000 // Convert to kbps
		}
	}

	// Parse stream-level info
	for i := range result.Streams {
		if result.Streams[i].CodecType != "audio" {
			continue
		}

		info.Format = result.Streams[i].CodecName

		if result.Streams[i].SampleRate != "" {
			info.SampleRate, _ = strconv.Atoi(result.Streams[i].SampleRate)
		}

		info.Channels = result.Streams[i].Channels

		// Stream tags can override format tags
		parseFFProbeTags(info, result.Streams[i].Tags)

		break
	}

	// If no tags found, fall back to filename parsing
	if info.Title == "" && info.Artist == "" && info.Album == "" {
		parsed := ParseAudioFilename(filePath)

		info.Title = parsed.Title
		info.Artist = parsed.Artist
		info.Album = parsed.Album
		info.TrackNumber = parsed.TrackNumber
		info.DiscNumber = parsed.DiscNumber
	}

	// Derive quality profile
	info.QualityProfile = deriveQualityProfile(info)

	return info, nil
}

// deriveQualityProfile determines a quality profile string based on audio properties.
func deriveQualityProfile(info *TrackInfo) string {
	// Lossless formats
	if IsLosslessFormat(info.Format) {
		if info.BitDepth >= 24 {
			return "lossless-hires"
		}

		return "lossless"
	}

	// Lossy formats - categorize by bitrate
	if info.Bitrate >= 320 {
		return "high"
	} else if info.Bitrate >= 256 {
		return "medium-high"
	} else if info.Bitrate >= 192 {
		return "medium"
	} else if info.Bitrate >= 128 {
		return "standard"
	}

	return "low"
}

// parseFFProbeTags extracts common audio tags from ffprobe's tag map.
func parseFFProbeTags(info *TrackInfo, tagMap map[string]string) {
	if tagMap == nil {
		return
	}

	// Helper to get tag with multiple possible keys
	getTag := func(keys ...string) string {
		for i := range keys {
			// Try exact match
			if val, ok := tagMap[keys[i]]; ok && val != "" {
				return val
			}

			// Try case-insensitive
			for k, v := range tagMap {
				if strings.EqualFold(k, keys[i]) && v != "" {
					return v
				}
			}
		}

		return ""
	}

	// Basic metadata
	if val := getTag("title", "TITLE"); val != "" {
		info.Title = val
	}

	if val := getTag("artist", "ARTIST"); val != "" {
		info.Artist = val
	}

	if val := getTag("album", "ALBUM"); val != "" {
		info.Album = val
	}

	if val := getTag("album_artist", "ALBUMARTIST", "ALBUM_ARTIST"); val != "" {
		info.AlbumArtist = val
	}

	if val := getTag("genre", "GENRE"); val != "" {
		info.Genre = val
	}

	// Track/Disc numbers
	if val := getTag("track", "TRACKNUMBER", "TRACK"); val != "" {
		// Handle "3/12" format
		if idx := strings.Index(val, "/"); idx != -1 {
			val = val[:idx]
		}

		info.TrackNumber, _ = strconv.Atoi(val)
	}

	if val := getTag("disc", "DISCNUMBER", "DISC"); val != "" {
		if idx := strings.Index(val, "/"); idx != -1 {
			val = val[:idx]
		}

		info.DiscNumber, _ = strconv.Atoi(val)
		if info.DiscNumber == 0 {
			info.DiscNumber = 1
		}
	}

	// Year/Date
	if val := getTag("date", "DATE", "year", "YEAR"); val != "" {
		// Extract year from date string (could be "2023", "2023-01-15", etc.)
		if len(val) >= 4 {
			yearStr := val[:4]
			if y, err := strconv.Atoi(yearStr); err == nil && y > 1900 && y < 2100 {
				info.Year = y
			}
		}
	}

	// Audiobook-specific tags
	if val := getTag("narrator", "NARRATOR", "composer", "COMPOSER"); val != "" {
		info.Narrator = val
	}

	if val := getTag("series", "SERIES", "grouping", "GROUPING"); val != "" {
		info.Series = val
	}

	if val := getTag("series-part", "SERIES_PART", "movement", "MOVEMENT"); val != "" {
		info.SeriesNum = val
	}

	if val := getTag("asin", "ASIN"); val != "" {
		info.ASIN = val
	}

	if val := getTag("audible_asin", "AUDIBLE_ASIN"); val != "" {
		info.AudibleID = val
	}

	// MusicBrainz IDs
	if val := getTag("musicbrainz_trackid", "MUSICBRAINZ_TRACKID"); val != "" {
		info.MusicBrainzID = val
	}

	if val := getTag("isrc", "ISRC"); val != "" {
		info.ISRC = val
	}

	if val := getTag("acoustid_id", "ACOUSTID_ID"); val != "" {
		info.AcoustID = val
	}

	if val := getTag("musicbrainz_discid", "MUSICBRAINZ_DISCID"); val != "" {
		info.DiscID = val
	}
}

// runFFProbeAudio runs ffprobe on an audio file and returns parsed JSON.
func runFFProbeAudio(ctx context.Context, filePath string) (*ffprobeAudioResult, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	cmd.Stdout = buf
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var result ffprobeAudioResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ReadAudioTagsBatch reads tags from multiple audio files efficiently.
func ReadAudioTagsBatch(filepaths []string) ([]*TrackInfo, error) {
	tracks := make([]*TrackInfo, 0, len(filepaths))

	for i := range filepaths {
		track, err := ReadAudioTags(filepaths[i])
		if err != nil {
			// Use filename parsing as fallback
			// track = ParseAudioFilename(filepaths[i])
			return nil, err
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}

// IsLosslessFormat returns true if the format is lossless audio.
func IsLosslessFormat(format string) bool {
	return logger.SlicesContainsI(losslessFormats, format)
}

// CollectTracksFromFiles creates TrackInfo from file paths using filename parsing only.
// Tags are NOT read - this is for the initial pass where we try to match by folder/filename.
func CollectTracksFromFiles(files []string) []TrackInfo {
	tracks := make([]TrackInfo, 0, len(files))

	for i := range files {
		track := ParseAudioFilename(files[i])

		track.Filepath = files[i]
		tracks = append(tracks, *track)
	}

	return tracks
}

// EnrichTracksWithTags reads tags for all tracks and updates their info.
// This is called when we need full tag data for duration/chapter matching.
func EnrichTracksWithTags(tracks []TrackInfo) []TrackInfo {
	enriched := make([]TrackInfo, len(tracks))

	for i, track := range tracks {
		taggedTrack, err := ReadAudioTags(track.Filepath)
		if err != nil {
			// Keep original parsed data
			enriched[i] = track
		} else {
			// Merge: prefer tag data but keep parsed data as fallback
			enriched[i] = *taggedTrack
			PutTrackInfo(taggedTrack)

			enriched[i].Filepath = track.Filepath
			// Keep parsed track number if tag didn't have one
			if enriched[i].TrackNumber == 0 && track.TrackNumber > 0 {
				enriched[i].TrackNumber = track.TrackNumber
			}

			if enriched[i].DiscNumber == 0 && track.DiscNumber > 0 {
				enriched[i].DiscNumber = track.DiscNumber
			}
		}
	}

	// Post-process: detect encoded disc+track numbers (e.g., 101=disc1/track01, 201=disc2/track01)
	NormalizeDiscTrackNumbers(enriched)

	return enriched
}

// NormalizeDiscTrackNumbers detects and splits encoded disc+track numbers.
// Scene releases commonly encode disc info in filenames: 101-124 for CD1,
// 201-224 for CD2, etc. (first digit = disc, remaining digits = track).
// This also works when tags have proper track numbers (1-24) but no disc info —
// it falls back to parsing the leading number from the filename.
func NormalizeDiscTrackNumbers(tracks []TrackInfo) {
	if len(tracks) < 2 {
		return
	}

	// Check if tags already provide multiple distinct disc numbers
	discSet := make(map[int]bool)
	for i := range tracks {
		discSet[tracks[i].DiscNumber] = true
	}

	if len(discSet) > 1 {
		return // Tags already have proper disc info
	}

	// Strategy 1: Check if track numbers themselves encode disc+track (100-999 range)
	if normalizeFromTrackNumbers(tracks) {
		return
	}

	// Strategy 2: Check filenames for encoded disc+track (e.g., "101_title.mp3", "201_title.mp3")
	// This handles the case where tags have proper track numbers (1-24) but no disc info.
	normalizeFromFilenames(tracks)
}

// normalizeFromTrackNumbers checks if track numbers encode disc info (e.g., TrackNumber=201 → disc 2, track 1).
func normalizeFromTrackNumbers(tracks []TrackInfo) bool {
	encodedDiscs := make(map[int]int)

	for i := range tracks {
		if tracks[i].TrackNumber < 100 || tracks[i].TrackNumber >= 1000 {
			return false
		}

		disc := tracks[i].TrackNumber / 100

		track := tracks[i].TrackNumber % 100
		if track == 0 || track > 99 {
			return false
		}

		encodedDiscs[disc]++
	}

	if len(encodedDiscs) < 2 {
		return false
	}

	for i := range encodedDiscs {
		if encodedDiscs[i] < 2 {
			return false
		}
	}

	for i := range tracks {
		origTrack := tracks[i].TrackNumber

		tracks[i].DiscNumber = origTrack / 100
		tracks[i].TrackNumber = origTrack % 100
	}

	logger.Logtype("info", 1).
		Int("trackCount", len(tracks)).
		Int("discCount", len(encodedDiscs)).
		Msg("Normalized encoded disc+track numbers from track numbers")

	return true
}

// normalizeFromFilenames parses leading numbers from filenames to detect disc+track encoding.
// Handles filenames like "101_artist_-_title.mp3" where tags have TrackNumber=1 but no disc.
func normalizeFromFilenames(tracks []TrackInfo) {
	type filenameDisc struct {
		disc, track int
	}

	parsed := make([]filenameDisc, len(tracks))
	encodedDiscs := make(map[int]int)

	for i, t := range tracks {
		base := filepath.Base(t.Filepath)

		// Extract leading number from filename
		var numStr strings.Builder
		for _, c := range base {
			if c < '0' || c > '9' {
				break
			}

			numStr.WriteString(string(c))
		}

		if len(numStr.String()) < 3 {
			return // Need at least 3 digits for disc+track encoding
		}

		num, err := strconv.Atoi(numStr.String())
		if err != nil || num < 100 || num >= 1000 {
			return
		}

		disc := num / 100

		track := num % 100
		if track == 0 || track > 99 {
			return
		}

		parsed[i] = filenameDisc{disc: disc, track: track}
		encodedDiscs[disc]++
	}

	if len(encodedDiscs) < 2 {
		return
	}

	for i := range encodedDiscs {
		if encodedDiscs[i] < 2 {
			return
		}
	}

	for i := range tracks {
		tracks[i].DiscNumber = parsed[i].disc
		tracks[i].TrackNumber = parsed[i].track
	}

	logger.Logtype("info", 1).
		Int("trackCount", len(tracks)).
		Int("discCount", len(encodedDiscs)).
		Msg("Normalized encoded disc+track numbers from filenames")
}

// SortTracksByFilename sorts tracks alphabetically by filename.
func SortTracksByFilename(tracks []TrackInfo) {
	sort.Slice(tracks, func(i, j int) bool {
		return filepath.Base(tracks[i].Filepath) < filepath.Base(tracks[j].Filepath)
	})
}

// SortTracksByDiscAndTrack sorts tracks by disc number, then track number.
func SortTracksByDiscAndTrack(tracks []TrackInfo) {
	sort.Slice(tracks, func(i, j int) bool {
		if tracks[i].DiscNumber != tracks[j].DiscNumber {
			return tracks[i].DiscNumber < tracks[j].DiscNumber
		}

		return tracks[i].TrackNumber < tracks[j].TrackNumber
	})
}

// CalculateTotalRuntime sums the runtime of all tracks.
func CalculateTotalRuntime(tracks []TrackInfo) time.Duration {
	var total time.Duration
	for i := range tracks {
		total += tracks[i].Runtime
	}

	return total
}

// ValidateAlbum checks if an album has all expected tracks.
// It returns whether the album is complete and a list of missing track numbers.
func ValidateAlbum(album *AlbumInfo, expectedTracks int) (bool, []int) {
	if expectedTracks <= 0 {
		// If we don't know the expected count, check for gaps in sequence
		return validateTrackSequence(album)
	}

	// Check if we have all tracks 1..expectedTracks
	trackMap := make(map[int]map[int]bool) // disc -> track -> exists

	for i := range album.Tracks {
		disc := album.Tracks[i].DiscNumber
		if disc == 0 {
			disc = 1
		}

		if trackMap[disc] == nil {
			trackMap[disc] = make(map[int]bool)
		}

		trackMap[disc][album.Tracks[i].TrackNumber] = true
	}

	// Single disc album
	if album.DiscCount <= 1 {
		var missing []int
		for i := 1; i <= expectedTracks; i++ {
			if !trackMap[1][i] {
				missing = append(missing, i)
			}
		}

		album.MissingTracks = missing
		album.IsComplete = len(missing) == 0

		return album.IsComplete, missing
	}

	// Multi-disc: we don't know tracks per disc, just check for gaps
	return validateTrackSequence(album)
}

// validateTrackSequence checks for gaps in track numbering.
func validateTrackSequence(album *AlbumInfo) (bool, []int) {
	if len(album.Tracks) == 0 {
		album.IsComplete = false
		return false, nil
	}

	// Group by disc
	discTracks := make(map[int][]int)
	for i := range album.Tracks {
		disc := album.Tracks[i].DiscNumber
		if disc == 0 {
			disc = 1
		}

		discTracks[disc] = append(discTracks[disc], album.Tracks[i].TrackNumber)
	}

	var allMissing []int

	for disc, tracks := range discTracks {
		// Sort tracks
		sort.Ints(tracks)

		if len(tracks) == 0 {
			continue
		}

		// Check if tracks start from 1 and have no gaps
		// Allow starting from 0 (some albums use 0 for intro)
		minTrack := tracks[0]
		if minTrack > 1 {
			// Missing tracks at the start
			for i := 1; i < minTrack; i++ {
				allMissing = append(allMissing, disc*100+i) // Encode disc in track number
			}
		}

		// Check for duplicates and gaps
		for i := 1; i < len(tracks); i++ {
			if tracks[i] == tracks[i-1] {
				// Duplicate track numbers within the same disc means
				// disc/track tags are unreliable (e.g., multi-disc audiobook
				// with all tracks tagged as disc 1)
				allMissing = append(allMissing, disc*100+tracks[i])
				continue
			}

			expected := tracks[i-1] + 1

			actual := tracks[i]
			if actual <= expected {
				continue
			}

			for j := expected; j < actual; j++ {
				allMissing = append(allMissing, disc*100+j)
			}
		}
	}

	album.MissingTracks = allMissing
	album.IsComplete = len(allMissing) == 0

	return album.IsComplete, allMissing
}

// IsMultiEpisodeAudiobookFolder detects whether a folder contains multiple
// distinct audiobook episodes (each file is a separate book/episode) rather
// than chapters of a single audiobook.
//
// Detection strategy:
//  1. Read tags from the first two files and compare Album tags —
//     if they differ, the folder contains separate episodes.
//  2. Fallback: check runtime ratios. Episodes are typically 20-60+ min each;
//     chapters are typically 5-15 min. If individual file runtime is long and
//     there are many files, it's likely multi-episode.
func IsMultiEpisodeAudiobookFolder(folder string, files []string) bool {
	if len(files) < 2 {
		return false
	}

	// Read tags from first two files
	tags1, err1 := ReadAudioTags(files[0])
	tags2, err2 := ReadAudioTags(files[1])

	defer PutTrackInfo(tags1)
	defer PutTrackInfo(tags2)

	// Compare Album tags — different albums means different episodes
	if err1 == nil && err2 == nil && tags1.Album != "" && tags2.Album != "" {
		if !equalFoldASCII(tags1.Album, tags2.Album) {
			logger.Logtype("info", 1).
				Str("folder", folder).
				Str("album1", tags1.Album).
				Str("album2", tags2.Album).
				Int("fileCount", len(files)).
				Msg("Multi-episode audiobook detected: different Album tags")

			return true
		}

		// Same album tags — this is a normal multi-chapter audiobook
		return false
	}

	// Fallback: runtime-based detection when tags are missing/empty
	// A typical audiobook chapter is 5-15 minutes; an episode is 20-60+ minutes.
	// If we have many files with long individual runtimes, it's likely episodes.
	var firstRuntime int64
	if err1 == nil && tags1.RuntimeMS > 0 {
		firstRuntime = tags1.RuntimeMS
	} else if err2 == nil && tags2.RuntimeMS > 0 {
		firstRuntime = tags2.RuntimeMS
	}

	if firstRuntime <= 0 {
		return false
	}

	const (
		episodeMinRuntimeMS = 20 * 60 * 1000      // 20 minutes — minimum for an episode
		minFilesForEpisode  = 10                  // need many files to suspect episodes
		maxTotalHoursNormal = 40 * 60 * 60 * 1000 // 40 hours — max for a single audiobook
	)

	fileCount := len(files)
	estimatedTotalMS := firstRuntime * int64(fileCount)

	if fileCount >= minFilesForEpisode &&
		firstRuntime >= episodeMinRuntimeMS &&
		estimatedTotalMS > maxTotalHoursNormal {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Int("fileCount", fileCount).
			Int64("fileRuntimeMS", firstRuntime).
			Int64("estimatedTotalMS", estimatedTotalMS).
			Msg("Multi-episode audiobook detected: runtime heuristic")

		return true
	}

	return false
}
