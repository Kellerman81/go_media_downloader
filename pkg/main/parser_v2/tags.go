package parser_v2

import (
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

// TagReader provides audio tag reading functionality for parser_v2.
type TagReader struct {
	manager *tags.Manager
}

// NewTagReader creates a new TagReader using the tags package.
func NewTagReader() *TagReader {
	return &TagReader{
		manager: tags.NewManager(),
	}
}

// ReadMusicTags reads audio tags from a music file and returns track info.
func (tr *TagReader) ReadMusicTags(filePath string) (*TrackInfo, error) {
	audioTags, err := tr.manager.ReadTags(filePath)
	if err != nil {
		return nil, err
	}

	track := &TrackInfo{
		Filename:               filepath.Base(filePath),
		Title:                  audioTags.Title,
		TrackNumber:            audioTags.TrackNumber,
		DiscNumber:             audioTags.DiscNumber,
		Artist:                 audioTags.Artist,
		ISRC:                   audioTags.ISRC,
		AcoustID:               audioTags.AcoustID,
		MusicBrainzRecordingID: audioTags.MBRecordingID,
	}

	// Duration in milliseconds
	if audioTags.Duration > 0 {
		track.RuntimeMS = audioTags.Duration.Milliseconds()
	}

	return track, nil
}

// ReadAlbumTags reads audio tags from multiple files and extracts album info.
func (tr *TagReader) ReadAlbumTags(filePaths []string) (*MusicParseResult, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	result := &MusicParseResult{
		ParseResult: ParseResult{
			MediaType: MediaTypeMusic,
		},
		Tracks: make([]TrackInfo, 0, len(filePaths)),
	}

	var (
		totalRuntime int64
		firstTags    *tags.AudioTags
	)

	for _, filePath := range filePaths {
		audioTags, err := tr.manager.ReadTags(filePath)
		if err != nil {
			// Still add a basic track entry even if we can't read tags
			result.Tracks = append(result.Tracks, TrackInfo{
				Filename: filepath.Base(filePath),
			})
			continue
		}

		track := TrackInfo{
			Filename:               filepath.Base(filePath),
			Title:                  audioTags.Title,
			TrackNumber:            audioTags.TrackNumber,
			DiscNumber:             audioTags.DiscNumber,
			Artist:                 audioTags.Artist,
			ISRC:                   audioTags.ISRC,
			AcoustID:               audioTags.AcoustID,
			MusicBrainzRecordingID: audioTags.MBRecordingID,
		}

		if audioTags.Duration > 0 {
			track.RuntimeMS = audioTags.Duration.Milliseconds()

			totalRuntime += track.RuntimeMS
		}

		result.Tracks = append(result.Tracks, track)

		// Capture first file's tags for album-level info
		if firstTags == nil {
			firstTags = audioTags
		}

		// Update disc count
		if audioTags.DiscNumber > result.TotalDiscs {
			result.TotalDiscs = audioTags.DiscNumber
		}

		if audioTags.TotalDiscs > result.TotalDiscs {
			result.TotalDiscs = audioTags.TotalDiscs
		}

		// Update track count from tags
		if audioTags.TotalTracks > result.TotalTracks {
			result.TotalTracks = audioTags.TotalTracks
		}
	}

	// Populate album-level info from first file's tags
	if firstTags != nil {
		result.Title = firstTags.Album
		result.Artist = firstTags.Artist
		result.AlbumArtist = firstTags.AlbumArtist
		result.Album = firstTags.Album
		result.Year = firstTags.Year
		result.Genre = firstTags.Genre
		result.Label = firstTags.Label
		result.CatalogNumber = firstTags.CatalogNum

		// MusicBrainz IDs
		result.MusicBrainzReleaseID = firstTags.MBReleaseID
		result.MusicBrainzReleaseGroupID = firstTags.MBReleaseGroupID

		// Set primary artist
		if result.AlbumArtist != "" {
			result.Artist = result.AlbumArtist
		}

		// Determine format from file extension
		if len(filePaths) > 0 {
			ext := strings.ToLower(filepath.Ext(filePaths[0]))

			result.Format = logger.ExtToFormat(ext)
			result.IsLossless = IsLosslessAudioExtension(ext)
		}

		// Audio properties
		result.Bitrate = firstTags.Bitrate
		result.SampleRate = firstTags.SampleRate
		result.BitDepth = firstTags.BitDepth
	}

	result.TotalRuntimeMS = totalRuntime

	// If we couldn't get total tracks from tags, use file count
	if result.TotalTracks == 0 {
		result.TotalTracks = len(result.Tracks)
	}

	// Check completeness
	result.IsComplete = len(result.Tracks) >= result.TotalTracks
	if !result.IsComplete {
		result.MissingTracks = findMissingTracks(result.Tracks, result.TotalTracks)
	}

	// Calculate confidence
	result.Confidence = calculateMusicTagConfidence(result, firstTags)

	return result, nil
}

// ReadAudiobookTags reads audio tags from audiobook files for metadata.
func (tr *TagReader) ReadAudiobookTags(filePaths []string) (*AudiobookParseResult, error) {
	if len(filePaths) == 0 {
		return nil, nil
	}

	result := &AudiobookParseResult{
		ParseResult: ParseResult{
			MediaType: MediaTypeAudiobook,
		},
		IsMultiFile: len(filePaths) > 1,
		Files:       make([]AudiobookFileInfo, 0, len(filePaths)),
	}

	var (
		totalRuntime int64
		firstTags    *tags.AudioTags
	)

	for i, filePath := range filePaths {
		audioTags, err := tr.manager.ReadTags(filePath)
		if err != nil {
			result.Files = append(result.Files, AudiobookFileInfo{
				Filename:   filepath.Base(filePath),
				PartNumber: i + 1,
			})

			continue
		}

		fileInfo := AudiobookFileInfo{
			Filename:     filepath.Base(filePath),
			PartNumber:   audioTags.TrackNumber,
			DiscNumber:   audioTags.DiscNumber,
			ChapterTitle: audioTags.Title,
		}

		if fileInfo.PartNumber == 0 {
			fileInfo.PartNumber = i + 1
		}

		if audioTags.Duration > 0 {
			fileInfo.RuntimeMS = audioTags.Duration.Milliseconds()

			totalRuntime += fileInfo.RuntimeMS
		}

		result.Files = append(result.Files, fileInfo)

		if firstTags == nil {
			firstTags = audioTags
		}
	}

	// Populate audiobook info from first file
	if firstTags != nil {
		result.Title = firstTags.Album
		result.Year = firstTags.Year

		// Artist is typically the author for audiobooks
		result.Author = firstTags.Artist

		// Album artist might be narrator
		if firstTags.AlbumArtist != "" && firstTags.AlbumArtist != firstTags.Artist {
			result.Narrator = firstTags.AlbumArtist
		}

		// Determine format from file extension
		if len(filePaths) > 0 {
			result.Format = logger.ExtToFormat(filepath.Ext(filePaths[0]))
		}

		// Audio properties
		result.Bitrate = firstTags.Bitrate
		result.SampleRate = firstTags.SampleRate
	}

	result.RuntimeMS = totalRuntime
	result.TotalParts = len(result.Files)

	// Calculate confidence
	result.Confidence = calculateAudiobookTagConfidence(result, firstTags)

	return result, nil
}

// IsSupported checks if the file format is supported for tag reading.
func (tr *TagReader) IsSupported(filePath string) bool {
	return tr.manager.IsSupported(filePath)
}

// findMissingTracks identifies missing track numbers.
func findMissingTracks(tracks []TrackInfo, totalTracks int) []int {
	present := make(map[int]bool)
	for i := range tracks {
		if tracks[i].TrackNumber > 0 {
			present[tracks[i].TrackNumber] = true
		}
	}

	var missing []int
	for i := 1; i <= totalTracks; i++ {
		if !present[i] {
			missing = append(missing, i)
		}
	}

	return missing
}

// calculateMusicTagConfidence calculates confidence based on available tag data.
func calculateMusicTagConfidence(result *MusicParseResult, tags *tags.AudioTags) float64 {
	if tags == nil {
		return 0.2
	}

	var conf float64

	// Album and artist are essential
	if result.Album != "" {
		conf += 0.25
	}

	if result.Artist != "" || result.AlbumArtist != "" {
		conf += 0.2
	}

	// MusicBrainz IDs are strong identifiers
	if result.MusicBrainzReleaseID != "" {
		conf += 0.25
	}

	// Year adds context
	if result.Year > 0 {
		conf += 0.05
	}

	// Track information quality
	tracksWithNumbers := 0

	tracksWithTitles := 0
	for i := range result.Tracks {
		if result.Tracks[i].TrackNumber > 0 {
			tracksWithNumbers++
		}

		if result.Tracks[i].Title != "" {
			tracksWithTitles++
		}
	}

	if len(result.Tracks) > 0 {
		trackNumberRatio := float64(tracksWithNumbers) / float64(len(result.Tracks))
		trackTitleRatio := float64(tracksWithTitles) / float64(len(result.Tracks))

		conf += 0.15 * trackNumberRatio
		conf += 0.1 * trackTitleRatio
	}

	// Cap at 1.0
	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// calculateAudiobookTagConfidence calculates confidence for audiobook tags.
func calculateAudiobookTagConfidence(result *AudiobookParseResult, tags *tags.AudioTags) float64 {
	if tags == nil {
		return 0.2
	}

	var conf float64

	// Title is essential
	if result.Title != "" {
		conf += 0.3
	}

	// Author is important
	if result.Author != "" {
		conf += 0.25
	}

	// Narrator adds context
	if result.Narrator != "" {
		conf += 0.15
	}

	// Year adds context
	if result.Year > 0 {
		conf += 0.05
	}

	// Runtime is important for matching
	if result.RuntimeMS > 0 {
		conf += 0.15
	}

	// File part numbers
	partsWithNumbers := 0
	for i := range result.Files {
		if result.Files[i].PartNumber > 0 {
			partsWithNumbers++
		}
	}

	if len(result.Files) > 0 {
		partNumberRatio := float64(partsWithNumbers) / float64(len(result.Files))

		conf += 0.1 * partNumberRatio
	}

	// Cap at 1.0
	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// DefaultTagReader is a package-level TagReader instance.
var DefaultTagReader = NewTagReader()

// ReadMusicTags uses the default reader to read music tags.
func ReadMusicTags(filePath string) (*TrackInfo, error) {
	return DefaultTagReader.ReadMusicTags(filePath)
}

// ReadAlbumTags uses the default reader to read album tags from multiple files.
func ReadAlbumTags(filePaths []string) (*MusicParseResult, error) {
	return DefaultTagReader.ReadAlbumTags(filePaths)
}

// ReadAudiobookTags uses the default reader to read audiobook tags.
func ReadAudiobookTags(filePaths []string) (*AudiobookParseResult, error) {
	return DefaultTagReader.ReadAudiobookTags(filePaths)
}
