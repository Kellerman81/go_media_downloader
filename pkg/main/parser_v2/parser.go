package parser_v2

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Parser is the unified parser that handles all media types.
type Parser struct {
	videoParser     *VideoParser
	bookParser      *BookParser
	audiobookParser *AudiobookParser
	musicParser     *MusicParser
	nzbPreprocessor *NZBPreprocessor

	// Configuration
	defaultVideoType    MediaType
	audiobookExtensions map[string]bool
}

// ParserConfig contains configuration options for the unified parser.
type ParserConfig struct {
	// DefaultVideoType is the default type when video type cannot be determined.
	DefaultVideoType MediaType
	// RuntimeMatcher provides custom runtime matching settings.
	RuntimeMatcher *RuntimeMatcher
}

// NewParser creates a new unified Parser with default configuration.
func NewParser() *Parser {
	return NewParserWithConfig(ParserConfig{})
}

// NewParserWithConfig creates a new unified Parser with custom configuration.
func NewParserWithConfig(cfg ParserConfig) *Parser {
	p := &Parser{
		videoParser:      NewVideoParser(),
		bookParser:       NewBookParser(),
		audiobookParser:  NewAudiobookParser(),
		musicParser:      NewMusicParser(),
		nzbPreprocessor:  NewNZBPreprocessor(),
		defaultVideoType: cfg.DefaultVideoType,
	}

	// Set runtime matcher if provided
	if cfg.RuntimeMatcher != nil {
		p.audiobookParser = NewAudiobookParserWithMatcher(cfg.RuntimeMatcher)
		p.musicParser = NewMusicParserWithMatcher(cfg.RuntimeMatcher)
	}

	// Build audiobook extensions map for quick lookup
	p.audiobookExtensions = make(map[string]bool)
	for _, ext := range AudiobookExtensions {
		p.audiobookExtensions[ext] = true
	}

	return p
}

// UnifiedParseResult is a union type for all parser results.
type UnifiedParseResult struct {
	MediaType MediaType
	Video     *VideoParseResult
	Book      *BookParseResult
	Audiobook *AudiobookParseResult
	Music     *MusicParseResult
}

// GetTitle returns the title from whichever result is populated.
func (u *UnifiedParseResult) GetTitle() string {
	switch u.MediaType {
	case MediaTypeMovie, MediaTypeSeries:
		if u.Video != nil {
			return u.Video.Title
		}

	case MediaTypeBook:
		if u.Book != nil {
			return u.Book.Title
		}

	case MediaTypeAudiobook:
		if u.Audiobook != nil {
			return u.Audiobook.Title
		}

	case MediaTypeMusic:
		if u.Music != nil {
			return u.Music.Title
		}

	default:
		// MediaTypeUnknown: no title available
	}

	return ""
}

// GetYear returns the year from whichever result is populated.
func (u *UnifiedParseResult) GetYear() int {
	switch u.MediaType {
	case MediaTypeMovie, MediaTypeSeries:
		if u.Video != nil {
			return u.Video.Year
		}

	case MediaTypeBook:
		if u.Book != nil {
			return u.Book.Year
		}

	case MediaTypeAudiobook:
		if u.Audiobook != nil {
			return u.Audiobook.Year
		}

	case MediaTypeMusic:
		if u.Music != nil {
			return u.Music.Year
		}

	default:
		// MediaTypeUnknown: no year available
	}

	return 0
}

// GetConfidence returns the confidence from whichever result is populated.
func (u *UnifiedParseResult) GetConfidence() float64 {
	switch u.MediaType {
	case MediaTypeMovie, MediaTypeSeries:
		if u.Video != nil {
			return u.Video.Confidence
		}

	case MediaTypeBook:
		if u.Book != nil {
			return u.Book.Confidence
		}

	case MediaTypeAudiobook:
		if u.Audiobook != nil {
			return u.Audiobook.Confidence
		}

	case MediaTypeMusic:
		if u.Music != nil {
			return u.Music.Confidence
		}

	default:
		// MediaTypeUnknown: no confidence available
	}

	return 0
}

// Parse parses a file and auto-detects the media type based on extension.
// Automatically handles NZB/Usenet-style subject lines by preprocessing them first.
func (p *Parser) Parse(filename string) *UnifiedParseResult {
	// Preprocess NZB-style filenames
	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	ext := strings.ToLower(filepath.Ext(cleanFilename))

	result := &UnifiedParseResult{}

	switch {
	case IsVideoExtension(ext):
		result.Video = p.videoParser.Parse(cleanFilename)
		result.MediaType = result.Video.MediaType

	case IsBookExtension(ext):
		result.Book = p.bookParser.Parse(cleanFilename)
		result.MediaType = MediaTypeBook

	case p.isAudiobookFile(cleanFilename, ext):
		result.Audiobook = p.audiobookParser.Parse(cleanFilename)
		result.MediaType = MediaTypeAudiobook

	case IsAudioExtension(ext):
		result.Music = p.musicParser.Parse(cleanFilename)
		result.MediaType = MediaTypeMusic

	default:
		// Unknown type, try video parsing as fallback
		result.Video = p.videoParser.Parse(cleanFilename)
		result.MediaType = MediaTypeUnknown
	}

	return result
}

// ParseWithPath parses a file with its full path for additional context.
func (p *Parser) ParseWithPath(fullpath string) *UnifiedParseResult {
	filename := filepath.Base(fullpath)
	ext := strings.ToLower(filepath.Ext(filename))

	result := &UnifiedParseResult{}

	switch {
	case IsVideoExtension(ext):
		result.Video = p.videoParser.ParseWithPath(fullpath)
		result.MediaType = result.Video.MediaType

	case IsBookExtension(ext):
		result.Book = p.bookParser.ParseWithPath(fullpath)
		result.MediaType = MediaTypeBook

	case p.isAudiobookFile(filename, ext):
		result.Audiobook = p.audiobookParser.Parse(filename)
		result.Audiobook.SourcePath = fullpath
		result.MediaType = MediaTypeAudiobook

	case IsAudioExtension(ext):
		result.Music = p.musicParser.Parse(filename)
		result.MediaType = MediaTypeMusic

	default:
		result.Video = p.videoParser.ParseWithPath(fullpath)
		result.MediaType = MediaTypeUnknown
	}

	return result
}

// ParseAsType parses a file as a specific media type.
func (p *Parser) ParseAsType(filename string, mediaType MediaType) *UnifiedParseResult {
	result := &UnifiedParseResult{MediaType: mediaType}

	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	switch mediaType {
	case MediaTypeMovie:
		p.videoParser.SetMovieMode()

		result.Video = p.videoParser.Parse(cleanFilename)

	case MediaTypeSeries:
		p.videoParser.SetSeriesMode()

		result.Video = p.videoParser.Parse(cleanFilename)

	case MediaTypeBook:
		result.Book = p.bookParser.Parse(cleanFilename)
	case MediaTypeAudiobook:
		result.Audiobook = p.audiobookParser.Parse(cleanFilename)
	case MediaTypeMusic:
		result.Music = p.musicParser.Parse(cleanFilename)
	default:
		return p.Parse(cleanFilename)
	}

	return result
}

// ParseVideo parses a file as a video (movie or series).
// Automatically handles NZB/Usenet-style subject lines.
func (p *Parser) ParseVideo(filename string) *VideoParseResult {
	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.videoParser.Parse(cleanFilename)
}

// ParseVideoPath parses a video file with full path.
func (p *Parser) ParseVideoPath(fullpath string) *VideoParseResult {
	return p.videoParser.ParseWithPath(fullpath)
}

// ParseMovie parses a file as a movie.
func (p *Parser) ParseMovie(filename string) *VideoParseResult {
	p.videoParser.SetMovieMode()

	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.videoParser.Parse(cleanFilename)
}

// ParseSeries parses a file as a TV series episode.
func (p *Parser) ParseSeries(filename string) *VideoParseResult {
	p.videoParser.SetSeriesMode()

	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.videoParser.Parse(cleanFilename)
}

// ParseBook parses a file as an ebook.
func (p *Parser) ParseBook(filename string) *BookParseResult {
	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.bookParser.Parse(cleanFilename)
}

// ParseBookPath parses an ebook file with full path.
func (p *Parser) ParseBookPath(fullpath string) *BookParseResult {
	cleanFilename := fullpath
	if p.nzbPreprocessor.IsNZBFormat(fullpath) {
		cleanFilename = p.nzbPreprocessor.Clean(fullpath)
	}

	return p.bookParser.ParseWithPath(cleanFilename)
}

// ParseAudiobook parses a file as an audiobook.
func (p *Parser) ParseAudiobook(filename string) *AudiobookParseResult {
	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.audiobookParser.Parse(cleanFilename)
}

// ParseAudiobookDirectory parses an audiobook from a directory of files.
func (p *Parser) ParseAudiobookDirectory(dirPath string) (*AudiobookParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.audiobookParser.ParseDirectory(dirPath, files), nil
}

// ParseMusic parses a file as a music track.
func (p *Parser) ParseMusic(filename string) *MusicParseResult {
	cleanFilename := filename
	if p.nzbPreprocessor.IsNZBFormat(filename) {
		cleanFilename = p.nzbPreprocessor.Clean(filename)
	}

	return p.musicParser.Parse(cleanFilename)
}

// ParseMusicAlbum parses a music album from a directory of files.
func (p *Parser) ParseMusicAlbum(dirPath string) (*MusicParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.musicParser.ParseAlbum(dirPath, files), nil
}

// isAudiobookFile determines if an audio file is likely an audiobook.
func (p *Parser) isAudiobookFile(filename, ext string) bool {
	// M4B is definitively audiobook
	if ext == ".m4b" || ext == ".aax" {
		return true
	}

	// Check filename patterns that suggest audiobook
	audiobookIndicators := []string{
		"audiobook", "audio book", "audiobook",
		"read by", "narrated by", "narrator",
		"unabridged", "abridged",
		"chapter", "ch.", "part",
	}

	for _, indicator := range audiobookIndicators {
		if logger.ContainsI(filename, indicator) {
			return true
		}
	}

	return false
}

// collectAudioFiles collects audio files from a directory.
func (p *Parser) collectAudioFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for i := range entries {
		if entries[i].IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entries[i].Name()))
		if IsAudioExtension(ext) {
			files = append(files, filepath.Join(dirPath, entries[i].Name()))
		}
	}

	return files, nil
}

// DetectMediaType attempts to detect the media type from a file path.
func (p *Parser) DetectMediaType(fullpath string) MediaType {
	ext := strings.ToLower(filepath.Ext(fullpath))
	filename := filepath.Base(fullpath)

	switch {
	case IsVideoExtension(ext):
		// Parse to detect movie vs series
		result := p.videoParser.Parse(filename)
		return result.MediaType

	case IsBookExtension(ext):
		return MediaTypeBook
	case p.isAudiobookFile(filename, ext):
		return MediaTypeAudiobook
	case IsAudioExtension(ext):
		return MediaTypeMusic
	default:
		return MediaTypeUnknown
	}
}

// DetectMediaTypeFromDirectory detects media type from directory contents.
func (p *Parser) DetectMediaTypeFromDirectory(dirPath string) MediaType {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return MediaTypeUnknown
	}

	var (
		hasVideo     bool
		hasAudio     bool
		hasBooks     bool
		audioCount   int
		videoCount   int
		audiobookish bool
	)

	for i := range entries {
		if entries[i].IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entries[i].Name()))

		switch {
		case IsVideoExtension(ext):
			hasVideo = true
			videoCount++

		case IsBookExtension(ext):
			hasBooks = true
		case IsAudioExtension(ext):
			hasAudio = true
			audioCount++

			if p.isAudiobookFile(entries[i].Name(), ext) {
				audiobookish = true
			}
		}
	}

	// Prioritize based on content
	switch {
	case hasVideo && !hasAudio:
		if videoCount == 1 {
			return MediaTypeMovie
		}

		return MediaTypeSeries // Multiple video files suggest series

	case hasBooks && !hasAudio && !hasVideo:
		return MediaTypeBook
	case hasAudio && audiobookish:
		return MediaTypeAudiobook
	case hasAudio:
		return MediaTypeMusic
	case hasVideo:
		return MediaTypeMovie
	default:
		return MediaTypeUnknown
	}
}

// MatchAudiobookByRuntime matches an audiobook to a database entry by runtime.
func (p *Parser) MatchAudiobookByRuntime(
	expectedRuntimeMS int64,
	files []AudiobookFileInfo,
) (bool, float64) {
	return p.audiobookParser.MatchByRuntime(expectedRuntimeMS, files)
}

// MatchMusicAlbumByRuntime matches a music album to a database entry by runtime.
func (p *Parser) MatchMusicAlbumByRuntime(
	expectedRuntimeMS int64,
	tracks []TrackInfo,
) (bool, float64) {
	return p.musicParser.MatchByRuntime(expectedRuntimeMS, tracks)
}

// SetQualityDatabase sets the quality database for video parsing.
// func (p *Parser) SetQualityDatabase(db QualityDatabase) {
// 	p.qualityDB = db
// 	p.videoParser = NewVideoParserWithDB(db)
// }

// SetRuntimeMatcher sets a custom runtime matcher for audio parsing.
func (p *Parser) SetRuntimeMatcher(rm *RuntimeMatcher) {
	p.audiobookParser = NewAudiobookParserWithMatcher(rm)
	p.musicParser = NewMusicParserWithMatcher(rm)
}

// ParseMusicAlbumWithTags parses a music album with audio tag reading.
func (p *Parser) ParseMusicAlbumWithTags(dirPath string) (*MusicParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.musicParser.ParseAlbumWithTags(dirPath, files), nil
}

func Gettypeids(inval string, qualitytype []database.Qualities) uint {
	for idx := range qualitytype {
		qual := &qualitytype[idx]
		if qual.Strings != "" && !config.GetSettingsGeneral().DisableParserStringMatch &&
			logger.SlicesContainsI(qual.StringsLowerSplitted, inval) {
			if qual.ID != 0 {
				return qual.ID
			}
		}

		if qual.UseRegex && qual.Regex != "" &&
			database.RegexGetMatchesFind(qual.Regex, inval, 2) {
			return qual.ID
		}
	}

	return 0
}

// ParseMusicAlbumWithAnalysis parses a music album with full media analysis.
func (p *Parser) ParseMusicAlbumWithAnalysis(
	dirPath string,
	analyzer *MediaAnalyzer,
) (*MusicParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.musicParser.ParseAlbumWithAnalysis(dirPath, files, analyzer), nil
}

// ParseAudiobookDirectoryWithTags parses an audiobook directory with audio tag reading.
func (p *Parser) ParseAudiobookDirectoryWithTags(dirPath string) (*AudiobookParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.audiobookParser.ParseDirectoryWithTags(dirPath, files), nil
}

// ParseAudiobookDirectoryWithAnalysis parses an audiobook directory with full media analysis.
func (p *Parser) ParseAudiobookDirectoryWithAnalysis(
	dirPath string,
	analyzer *MediaAnalyzer,
) (*AudiobookParseResult, error) {
	files, err := p.collectAudioFiles(dirPath)
	if err != nil {
		return nil, err
	}

	return p.audiobookParser.ParseDirectoryWithAnalysis(dirPath, files, analyzer), nil
}

// AnalyzeVideoFile analyzes a video file and updates the parse result with technical info.
func (p *Parser) AnalyzeVideoFile(
	filePath string,
	result *VideoParseResult,
	analyzer *MediaAnalyzer,
) error {
	if analyzer == nil {
		analyzer = DefaultMediaAnalyzer
	}

	return analyzer.AnalyzeVideo(filePath, result)
}

// NewMediaAnalyzerFromConfig creates a MediaAnalyzer from parser config.
func NewMediaAnalyzerFromConfig(cfg ParserConfig) *MediaAnalyzer {
	return NewMediaAnalyzer()
}
