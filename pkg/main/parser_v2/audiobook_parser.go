package parser_v2

import (
	"context"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// AudiobookParser handles parsing of audiobook filenames and multi-file collections.
type AudiobookParser struct {
	patterns       *audiobookPatterns
	runtimeMatcher *RuntimeMatcher
}

// audiobookPatterns contains compiled regex patterns for audiobook parsing.
type audiobookPatterns struct {
	asin            *regexp.Regexp
	isbn            *regexp.Regexp
	year            *regexp.Regexp
	partNumber      *regexp.Regexp
	discNumber      *regexp.Regexp
	chapterNum      *regexp.Regexp
	series          *regexp.Regexp
	seriesNum       *regexp.Regexp
	narrator        *regexp.Regexp
	author          *regexp.Regexp
	abridged        *regexp.Regexp
	unabridged      *regexp.Regexp
	group           *regexp.Regexp
	authorDash      *regexp.Regexp
	bitrate         *regexp.Regexp
	sampleRate      *regexp.Regexp
	titleClean      *regexp.Regexp
	sceneAuthorDash *regexp.Regexp
}

// NewAudiobookParser creates a new AudiobookParser with compiled patterns.
func NewAudiobookParser() *AudiobookParser {
	return &AudiobookParser{
		patterns:       compileAudiobookPatterns(),
		runtimeMatcher: DefaultRuntimeMatcher(),
	}
}

// NewAudiobookParserWithMatcher creates a new AudiobookParser with a custom runtime matcher.
func NewAudiobookParserWithMatcher(rm *RuntimeMatcher) *AudiobookParser {
	return &AudiobookParser{
		patterns:       compileAudiobookPatterns(),
		runtimeMatcher: rm,
	}
}

// Audiobook pattern strings as constants so they can be used as cache keys.
const (
	reAudioASIN            = `(?i)(?:asin[:\s-]?)?B0[A-Z0-9]{8}`
	reAudioISBN            = `(?i)(?:isbn[:\s-]?)?(?:978|979)[\s-]?\d[\s-]?\d{2}[\s-]?\d{5}[\s-]?\d{3}[\s-]?\d`
	reAudioYear            = `[\(\[]?((?:19|20)\d{2})[\)\]]?`
	reAudioPartNumber      = `(?i)(?:part|pt\.?|p)[\s._-]?(\d+)|(\d+)\s*(?:of|/)[\s._-]?(\d+)|(?:[\(\[]|\s|^)(\d{1,3})(?:[\)\]]|\s|$)`
	reAudioDiscNumber      = `(?i)(?:disc|disk|cd|d)[\s._-]?(\d+)`
	reAudioChapterNum      = `(?i)(?:chapter|ch\.?|chap\.?)[\s._-]?(\d+)`
	reAudioSeries          = `(?i)(?:\(|\[)?\s*([^()\[\]]+?)\s*(?:book|#|,?\s*no\.?|,?\s*vol\.?|,?\s*volume)\s*(\d+(?:\.\d+)?)\s*(?:\)|\])?`
	reAudioSeriesNum       = `(?i)(?:book|#|no\.?|vol\.?|volume)\s*(\d+(?:\.\d+)?)`
	reAudioNarrator        = `(?i)(?:read\s+by|narrated\s+by|narrator[:\s]+)[\s,:]*([^,\[\]\(\)]+)`
	reAudioAuthor          = `(?i)(?:by|author[:\s]+)[\s,:]*([^,\[\]\(\)-]+)`
	reAudioAbridged        = `(?i)[\[\(\s]abr(?:idged)?[\]\)\s]`
	reAudioUnabridged      = `(?i)[\[\(\s](?:un)?abr(?:idged)?[\]\)\s]`
	reAudioGroup           = `(?i)[\[\(]([a-z0-9_-]+)[\]\)]$`
	reAudioAuthorDash      = `^(.+?)\s+-\s+(.+)$`
	reAudioBitrate         = `(?i)(\d+)\s*(?:kbps|kb\/s|kbit)`
	reAudioSampleRate      = `(?i)(\d+(?:\.\d+)?)\s*(?:khz|k?hz)`
	reAudioTitleClean      = `(?i)[\[\(](?:audiobook|audio\s*book|unabridged|abridged)[\]\)]`
	reAudioSceneAuthorDash = `^([A-Za-z][A-Za-z\s\.]+)-([^-].*)$`
	reAudioNumInName       = `(?:^|\D)(\d{1,2})(?:\D|$)`
	reAudioStripASIN       = `(?i)asin[:\s-]*`
	reAudioStripISBN       = `(?i)isbn[:\s-]*`
)

// compileAudiobookPatterns returns the shared audiobook pattern set, fetching each
// compiled regexp from the global cache (compiled once, reused on every call).
func compileAudiobookPatterns() *audiobookPatterns {
	return &audiobookPatterns{
		asin:            database.GetCachedRegexp(reAudioASIN),
		isbn:            database.GetCachedRegexp(reAudioISBN),
		year:            database.GetCachedRegexp(reAudioYear),
		partNumber:      database.GetCachedRegexp(reAudioPartNumber),
		discNumber:      database.GetCachedRegexp(reAudioDiscNumber),
		chapterNum:      database.GetCachedRegexp(reAudioChapterNum),
		series:          database.GetCachedRegexp(reAudioSeries),
		seriesNum:       database.GetCachedRegexp(reAudioSeriesNum),
		narrator:        database.GetCachedRegexp(reAudioNarrator),
		author:          database.GetCachedRegexp(reAudioAuthor),
		abridged:        database.GetCachedRegexp(reAudioAbridged),
		unabridged:      database.GetCachedRegexp(reAudioUnabridged),
		group:           database.GetCachedRegexp(reAudioGroup),
		authorDash:      database.GetCachedRegexp(reAudioAuthorDash),
		bitrate:         database.GetCachedRegexp(reAudioBitrate),
		sampleRate:      database.GetCachedRegexp(reAudioSampleRate),
		titleClean:      database.GetCachedRegexp(reAudioTitleClean),
		sceneAuthorDash: database.GetCachedRegexp(reAudioSceneAuthorDash),
	}
}

// Parse parses a single audiobook filename.
func (ap *AudiobookParser) Parse(filename string) *AudiobookParseResult {
	result := &AudiobookParseResult{
		ParseResult: ParseResult{
			SourceFile: filename,
			MediaType:  MediaTypeAudiobook,
		},
	}

	// Get extension and base name
	ext := filepath.Ext(filename)

	result.Format = logger.ExtToFormat(ext)

	name := strings.TrimSuffix(filename, ext)

	// Clean common separators
	name = strings.ReplaceAll(name, "_", " ")

	// Normalize scene format separator ".-." to " - " for proper author/title splitting
	// This handles titles like "Paul.K.Lunow.-.Riaru.-.Willkommen.im.modernsten.Unternehmen.der.Welt"
	name = strings.ReplaceAll(name, ".-.", " - ")

	cleanedName := name

	// Extract ASIN (also remove surrounding brackets if present)
	if match := ap.patterns.asin.FindString(name); match != "" {
		result.ASIN = ap.extractASIN(match)
		// Remove ASIN with surrounding brackets/parens if present
		cleanedName = strings.Replace(cleanedName, "["+match+"]", "", 1)
		cleanedName = strings.Replace(cleanedName, "("+match+")", "", 1)
		cleanedName = strings.Replace(cleanedName, match, "", 1)
	}

	// Extract ISBN
	if match := ap.patterns.isbn.FindString(name); match != "" {
		result.ISBN13 = normalizeISBN(match)
		cleanedName = strings.Replace(cleanedName, match, "", 1)
	}

	// Check for abridged/unabridged
	if ap.patterns.abridged.MatchString(name) {
		result.Abridged = true
		cleanedName = ap.patterns.abridged.ReplaceAllString(cleanedName, " ")
	}

	if ap.patterns.unabridged.MatchString(name) {
		result.Abridged = false
		cleanedName = ap.patterns.unabridged.ReplaceAllString(cleanedName, " ")
	}

	// Extract year BEFORE release group (group pattern matches digits too)
	if matches := ap.patterns.year.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.Year = parseInt(matches[1])
		// Remove the year pattern from cleaned name (including surrounding parens/brackets)
		cleanedName = ap.patterns.year.ReplaceAllString(cleanedName, " ")
	}

	// Extract release group
	if matches := ap.patterns.group.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.ReleaseGroup = matches[1]
		cleanedName = ap.patterns.group.ReplaceAllString(cleanedName, "")
	}

	// Extract narrator
	if matches := ap.patterns.narrator.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.Narrator = strings.TrimSpace(matches[1])
		cleanedName = ap.patterns.narrator.ReplaceAllString(cleanedName, " ")
	}

	// Extract series information
	if matches := ap.patterns.series.FindStringSubmatch(cleanedName); len(matches) > 2 {
		result.Series = strings.TrimSpace(matches[1])
		result.SeriesPosition = matches[2]
		cleanedName = ap.patterns.series.ReplaceAllString(cleanedName, "")
	}

	// Extract bitrate
	if matches := ap.patterns.bitrate.FindStringSubmatch(name); len(matches) > 1 {
		result.Bitrate = parseInt(matches[1])
	}

	// Extract sample rate
	if matches := ap.patterns.sampleRate.FindStringSubmatch(name); len(matches) > 1 {
		rate := parseFloat(matches[1])
		// Convert kHz to Hz if needed
		if rate < 1000 {
			result.SampleRate = int(rate * 1000)
		} else {
			result.SampleRate = int(rate)
		}
	}

	// Clean up audiobook-specific tags
	if ap.patterns.titleClean.MatchString(cleanedName) {
		cleanedName = ap.patterns.titleClean.ReplaceAllString(cleanedName, "")
	}

	// Try to extract author and title
	ap.extractAuthorTitle(cleanedName, result)

	// Clean up title
	result.Title = cleanTitle(result.Title)

	// Calculate confidence
	result.Confidence = ap.calculateConfidence(result)

	return result
}

// ParseDirectory parses an audiobook directory containing multiple files.
func (ap *AudiobookParser) ParseDirectory(dirPath string, files []string) *AudiobookParseResult {
	if len(files) == 0 {
		return nil
	}

	// Parse the first file for base information
	result := ap.Parse(filepath.Base(files[0]))

	result.SourcePath = dirPath
	result.IsMultiFile = len(files) > 1

	// Try to extract info from directory name
	dirName := filepath.Base(dirPath)
	dirResult := ap.Parse(dirName)

	// Prefer directory name info if it seems more complete
	if dirResult.Title != "" && len(dirResult.Title) > len(result.Title) {
		result.Title = dirResult.Title
	}

	if dirResult.Author != "" && result.Author == "" {
		result.Author = dirResult.Author
	}

	if dirResult.Series != "" && result.Series == "" {
		result.Series = dirResult.Series
		result.SeriesPosition = dirResult.SeriesPosition
	}

	if dirResult.Narrator != "" && result.Narrator == "" {
		result.Narrator = dirResult.Narrator
	}

	if dirResult.ASIN != "" && result.ASIN == "" {
		result.ASIN = dirResult.ASIN
	}

	// Parse all files to get part information
	result.Files = make([]AudiobookFileInfo, 0, len(files))
	for i := range files {
		fileInfo := ap.parseFileInfo(files[i])

		result.Files = append(result.Files, fileInfo)
	}

	// Sort files by part/disc number
	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].DiscNumber != result.Files[j].DiscNumber {
			return result.Files[i].DiscNumber < result.Files[j].DiscNumber
		}

		return result.Files[i].PartNumber < result.Files[j].PartNumber
	})

	// Determine total parts and check for missing parts
	ap.analyzeParts(result)

	// Recalculate confidence with multi-file info
	result.Confidence = ap.calculateConfidence(result)

	return result
}

// parseFileInfo extracts file-level information from a filename.
func (ap *AudiobookParser) parseFileInfo(filename string) AudiobookFileInfo {
	info := AudiobookFileInfo{
		Filename: filepath.Base(filename),
	}

	name := strings.TrimSuffix(info.Filename, filepath.Ext(info.Filename))

	// Extract disc number
	if matches := ap.patterns.discNumber.FindStringSubmatch(name); len(matches) > 1 {
		info.DiscNumber = parseInt(matches[1])
	}

	// Extract part/track number
	if matches := ap.patterns.partNumber.FindStringSubmatch(name); len(matches) > 0 {
		for i := 1; i < len(matches); i++ {
			if matches[i] != "" {
				info.PartNumber = parseInt(matches[i])
				break
			}
		}
	}

	// Extract chapter number as alternative
	if info.PartNumber == 0 {
		if matches := ap.patterns.chapterNum.FindStringSubmatch(name); len(matches) > 1 {
			info.PartNumber = parseInt(matches[1])
			info.ChapterTitle = "Chapter " + matches[1]
		}
	}

	// If still no part number, try to extract from just numbers in filename
	if info.PartNumber == 0 {
		if matches := database.GetCachedRegexp(reAudioNumInName).
			FindStringSubmatch(name); len(
			matches,
		) > 1 {
			info.PartNumber = parseInt(matches[1])
		}
	}

	return info
}

// analyzeParts determines total parts and identifies missing parts.
func (ap *AudiobookParser) analyzeParts(result *AudiobookParseResult) {
	if len(result.Files) == 0 {
		return
	}

	// Find the highest part number
	maxPart := 0
	partMap := make(map[int]bool)

	for i := range result.Files {
		if result.Files[i].PartNumber > maxPart {
			maxPart = result.Files[i].PartNumber
		}

		if result.Files[i].PartNumber > 0 {
			partMap[result.Files[i].PartNumber] = true
		}
	}

	result.TotalParts = maxPart

	// Check for missing parts (only if we have numbered parts)
	if maxPart == 0 || maxPart > 100 { // Reasonable limit for part checking
		return
	}

	for i := 1; i <= maxPart; i++ {
		if !partMap[i] {
			result.MissingParts = append(result.MissingParts, i)
		}
	}
}

// extractAuthorTitle attempts to separate author from title.
func (ap *AudiobookParser) extractAuthorTitle(name string, result *AudiobookParseResult) {
	name = strings.TrimSpace(name)

	// Try "Author - Title" pattern
	if matches := ap.patterns.authorDash.FindStringSubmatch(name); len(matches) > 2 {
		potentialAuthor := strings.TrimSpace(matches[1])
		potentialTitle := strings.TrimSpace(matches[2])

		// If author or title contains dots without spaces, it's likely scene format
		// (e.g., "Paul.K.Lunow - Riaru - Willkommen.im.modernsten")
		isSceneFormat := (strings.Contains(potentialAuthor, ".") && !strings.Contains(potentialAuthor, " ")) ||
			(strings.Contains(potentialTitle, ".") && !strings.Contains(potentialTitle, " "))

		if isSceneFormat {
			potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
			potentialAuthor = logger.JoinStringsSep(strings.Fields(potentialAuthor), " ")
			potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")
			potentialTitle = logger.JoinStringsSep(strings.Fields(potentialTitle), " ")
		}

		// Clean scene tags from title (DE, AUDIOBOOK, FLAC, year, release group, etc.)
		potentialTitle = cleanAudiobookSceneTags(potentialTitle)

		// Validate it looks like an author name
		if looksLikePersonName(potentialAuthor) {
			result.Author = potentialAuthor
			result.Title = potentialTitle

			// Check for multiple authors
			if strings.Contains(result.Author, ",") || strings.Contains(result.Author, "&") {
				authors := splitAuthors(result.Author)

				result.Authors = authors
				if len(authors) > 0 {
					result.Author = authors[0]
				}
			}

			return
		}
	}

	// Try scene format "Author-Title-..." pattern (no spaces around dash)
	if matches := ap.patterns.sceneAuthorDash.FindStringSubmatch(name); len(matches) > 2 {
		potentialAuthor := strings.TrimSpace(matches[1])
		potentialTitle := strings.TrimSpace(matches[2])

		// For scene format, replace dots with spaces (e.g., "Paul.K.Lunow" -> "Paul K Lunow")
		potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
		potentialAuthor = logger.JoinStringsSep(
			strings.Fields(potentialAuthor),
			" ",
		) // Clean up extra spaces

		// Clean scene tags first (before replacing dots/dashes, to preserve structure)
		potentialTitle = cleanAudiobookSceneTags(potentialTitle)
		potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")
		potentialTitle = strings.ReplaceAll(potentialTitle, "-", " ")
		potentialTitle = logger.JoinStringsSep(
			strings.Fields(potentialTitle),
			" ",
		) // Clean up extra spaces

		// Validate it looks like an author name
		if looksLikePersonName(potentialAuthor) {
			result.Author = potentialAuthor
			result.Title = potentialTitle

			return
		}
	}

	// Try "by Author" pattern
	if matches := ap.patterns.author.FindStringSubmatch(name); len(matches) > 1 {
		result.Author = strings.TrimSpace(matches[1])
		// Remove the author part from name for title
		result.Title = ap.patterns.author.ReplaceAllString(name, "")
		result.Title = strings.TrimSpace(result.Title)

		return
	}

	// No author found, treat entire name as title
	result.Title = name
}

// extractASIN extracts the ASIN from a match.
func (ap *AudiobookParser) extractASIN(match string) string {
	return strings.ToUpper(
		strings.TrimSpace(database.GetCachedRegexp(reAudioStripASIN).ReplaceAllString(match, "")),
	)
}

// calculateConfidence calculates the confidence score.
func (ap *AudiobookParser) calculateConfidence(result *AudiobookParseResult) float64 {
	var conf float64

	// Title is essential
	if result.Title != "" {
		conf += 0.25
	}

	// Author is very helpful
	if result.Author != "" {
		conf += 0.2
	}

	// ASIN is a strong identifier
	if result.ASIN != "" {
		conf += 0.25
	}

	// Narrator adds confidence
	if result.Narrator != "" {
		conf += 0.1
	}

	// Multi-file completeness
	if result.IsMultiFile {
		if len(result.MissingParts) == 0 && result.TotalParts > 0 {
			conf += 0.1 // Complete set
		}

		if len(result.Files) > 1 {
			conf += 0.05
		}
	}

	// Year adds context
	if result.Year > 0 {
		conf += 0.05
	}

	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// MatchByRuntime attempts to match an audiobook to a database entry by total runtime.
func (ap *AudiobookParser) MatchByRuntime(
	expectedRuntimeMS int64,
	files []AudiobookFileInfo,
) (bool, float64) {
	return ap.runtimeMatcher.MatchTotalRuntime(expectedRuntimeMS, filesToTracks(files))
}

// UpdateFileRuntimes updates the runtime information for audiobook files.
func (ap *AudiobookParser) UpdateFileRuntimes(
	result *AudiobookParseResult,
	runtimes map[string]int64,
) {
	var totalRuntime int64
	for i := range result.Files {
		if runtime, ok := runtimes[result.Files[i].Filename]; ok {
			result.Files[i].RuntimeMS = runtime

			totalRuntime += runtime
		}
	}

	result.RuntimeMS = totalRuntime
}

// ParseDirectoryWithTags parses an audiobook directory using both filename parsing and audio tags.
func (ap *AudiobookParser) ParseDirectoryWithTags(
	dirPath string,
	files []string,
) *AudiobookParseResult {
	// First parse from filenames
	result := ap.ParseDirectory(dirPath, files)
	if result == nil {
		return nil
	}

	// Read tags from files
	tagResult, err := ReadAudiobookTags(files)
	if err != nil || tagResult == nil {
		return result
	}

	// Merge tag info
	ap.mergeTagInfo(result, tagResult)

	return result
}

// ParseDirectoryWithAnalysis parses an audiobook directory with full media analysis.
func (ap *AudiobookParser) ParseDirectoryWithAnalysis(
	ctx context.Context,
	dirPath string,
	files []string,
	analyzer *MediaAnalyzer,
) *AudiobookParseResult {
	// Parse with tags first
	result := ap.ParseDirectoryWithTags(dirPath, files)
	if result == nil {
		return nil
	}

	// Analyze files for runtime info
	if analyzer != nil {
		_ = analyzer.AnalyzeAudiobook(ctx, files, result)
	}

	return result
}

// mergeTagInfo merges tag information into the audiobook result.
func (ap *AudiobookParser) mergeTagInfo(result, tagResult *AudiobookParseResult) {
	// Book-level info from tags takes precedence
	if tagResult.Title != "" {
		result.Title = tagResult.Title
	}

	if tagResult.Author != "" {
		result.Author = tagResult.Author
	}

	if len(tagResult.Authors) > 0 {
		result.Authors = tagResult.Authors
	}

	if tagResult.Narrator != "" {
		result.Narrator = tagResult.Narrator
	}

	if tagResult.Year > 0 {
		result.Year = tagResult.Year
	}

	// Merge file info
	tagFileMap := make(map[string]*AudiobookFileInfo)
	for i := range tagResult.Files {
		tagFileMap[tagResult.Files[i].Filename] = &tagResult.Files[i]
	}

	for i := range result.Files {
		tagFile, ok := tagFileMap[result.Files[i].Filename]
		if !ok {
			continue
		}

		if tagFile.PartNumber > 0 {
			result.Files[i].PartNumber = tagFile.PartNumber
		}

		if tagFile.DiscNumber > 0 {
			result.Files[i].DiscNumber = tagFile.DiscNumber
		}

		if tagFile.ChapterTitle != "" {
			result.Files[i].ChapterTitle = tagFile.ChapterTitle
		}

		if tagFile.RuntimeMS > 0 {
			result.Files[i].RuntimeMS = tagFile.RuntimeMS
		}
	}

	// Recalculate totals and confidence
	ap.analyzeParts(result)

	result.Confidence = ap.calculateConfidence(result)
}

// filesToTracks converts AudiobookFileInfo slice to TrackInfo slice for runtime matching.
func filesToTracks(files []AudiobookFileInfo) []TrackInfo {
	tracks := make([]TrackInfo, len(files))
	for i, f := range files {
		tracks[i] = TrackInfo{
			Filename:    f.Filename,
			TrackNumber: f.PartNumber,
			RuntimeMS:   f.RuntimeMS,
		}
	}

	return tracks
}

// looksLikePersonName checks if a string looks like a person's name.
// This is case-insensitive to handle NZB titles with lowercase names.
func looksLikePersonName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}

	// Should contain at least one space (first + last name) typically
	wordCount := len(strings.Fields(s))
	if wordCount >= 2 && wordCount <= 5 {
		// Check first character is a letter (case-insensitive)
		first := s[0]
		return (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')
	}

	// Single word could still be valid (mononyms like "Plato")
	if wordCount == 1 {
		first := s[0]
		return (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z')
	}

	return false
}

// splitAuthors splits an author string by common separators.
func splitAuthors(s string) []string {
	// Replace & with comma for uniform splitting
	s = strings.ReplaceAll(s, " & ", ", ")
	s = strings.ReplaceAll(s, " and ", ", ")

	parts := strings.Split(s, ",")

	var authors []string
	for i := range parts {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			authors = append(authors, p)
		}
	}

	return authors
}

// audiobookSceneTags contains common scene tags to strip from audiobook titles.
var audiobookSceneTags = map[string]bool{
	// Audio formats
	"FLAC": true, "MP3": true, "AAC": true, "OGG": true, "OPUS": true,
	"WAV": true, "ALAC": true, "APE": true, "WMA": true, "M4A": true, "M4B": true,
	// Country codes
	"DE": true, "US": true, "UK": true, "EU": true, "JP": true, "AU": true,
	"CA": true, "FR": true, "IT": true, "ES": true, "NL": true, "SE": true,
	"NO": true, "DK": true, "FI": true, "AT": true, "CH": true, "BE": true,
	// Media types
	"AUDIOBOOK": true, "EBOOK": true, "CD": true, "DVD": true,
	// Scene tags
	"RETAIL": true, "WEB": true, "PROPER": true, "REPACK": true, "INT": true,
	"INTERNAL": true, "READNFO": true,
}

// cleanAudiobookSceneTags removes common scene tags from audiobook titles.
// It handles formats like "Title-DE-AUDIOBOOK-CD-FLAC-2001-oNePiEcE".
func cleanAudiobookSceneTags(title string) string {
	// Split by common separators
	parts := strings.FieldsFunc(title, func(r rune) bool {
		return r == '-' || r == '_'
	})

	if len(parts) <= 1 {
		return title
	}

	// Find where the actual title ends and scene tags begin
	// Work backwards from the end, removing known tags
	lastValidIdx := len(parts) - 1

	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.ToUpper(strings.TrimSpace(parts[i]))

		// Check if it's a known scene tag
		if audiobookSceneTags[part] {
			lastValidIdx = i - 1
			continue
		}

		// Check if it's a year (4 digits starting with 19 or 20)
		if len(part) == 4 && (strings.HasPrefix(part, "19") || strings.HasPrefix(part, "20")) {
			if _, err := strconv.Atoi(part); err == nil {
				lastValidIdx = i - 1
				continue
			}
		}

		// Check if it looks like a release group (all caps/mixed, short, at the end)
		if i == len(parts)-1 && len(part) <= 12 && isAlphanumeric(part) {
			// Likely a release group - skip it
			lastValidIdx = i - 1
			continue
		}

		// This part looks like actual title content, stop here
		break
	}

	if lastValidIdx < 0 {
		lastValidIdx = 0
	}

	return logger.JoinStringsSep(parts[:lastValidIdx+1], " ")
}

// isAlphanumeric checks if a string contains only alphanumeric characters.
func isAlphanumeric(s string) bool {
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return false
		}
	}

	return len(s) > 0
}

// normalizeISBN removes hyphens and spaces from ISBN.
func normalizeISBN(isbn string) string {
	isbn = database.GetCachedRegexp(reAudioStripISBN).ReplaceAllString(isbn, "")
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	return strings.ToUpper(isbn)
}

// parseFloat parses a string to float64, returning 0 on error.
func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return f
}
