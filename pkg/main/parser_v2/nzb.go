package parser_v2

import (
	"regexp"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// NZBPreprocessor handles cleanup of NZB/Usenet-style filenames.
// It strips metadata like part numbers, channel tags, and yEnc suffixes
// to extract the actual filename from complex Usenet subject lines.
type NZBPreprocessor struct {
	patterns *nzbPatterns
}

// nzbPatterns contains compiled regex patterns for NZB filename preprocessing.
type nzbPatterns struct {
	// [001/279] - NZB part number prefix
	partNumberPrefix *regexp.Regexp
	// - [01/58] - Part number suffix
	partNumberSuffix *regexp.Regexp
	// [22569]-[FULL]-[#a.b.hdtv.x264@EFNet]-[ ... ] - Usenet metadata brackets
	usenetMetaPrefix *regexp.Regexp
	// "filename.ext" - Quoted filename extraction
	quotedFile *regexp.Regexp
	// yEnc suffix at end of line
	yenc *regexp.Regexp
	// (TR-1080p) - Quality prefix in parentheses at start
	qualityPrefix *regexp.Regexp
	// <kere.ws> - Site tags
	siteTag *regexp.Regexp
	// Trailing .par2, .vol000+01.par2, etc. for repair files
	par2Suffix *regexp.Regexp
	// Trailing .rar, .r00, .001 for archive files
	archiveSuffix *regexp.Regexp
}

// defaultNZBPreprocessor is a shared instance for convenience.
var defaultNZBPreprocessor = NewNZBPreprocessor()

// NewNZBPreprocessor creates a new NZB filename preprocessor.
func NewNZBPreprocessor() *NZBPreprocessor {
	return &NZBPreprocessor{
		patterns: compileNZBPatterns(),
	}
}

// NZB pattern strings as constants so they can be used as cache keys.
const (
	reNZBPartNumberPrefix = `^\s*\[\d{1,4}/\d{1,4}\]\s*`
	reNZBPartNumberSuffix = `\s*-?\s*\[\d{1,4}/\d{1,4}\]\s*`
	reNZBUsenetMetaPrefix = `^\s*(?:\[\d+\]\s*-?\s*|\[FULL\]\s*-?\s*|\[#[^\]]+\]\s*-?\s*|\[[^\]]*[^\]A-Za-z0-9.][^\]]*\]\s*-?\s*)+`
	reNZBQuotedFile       = `"([^"]+)"`
	reNZBYenc             = `\s+yEnc\s*$`
	reNZBQualityPrefix    = `^\s*\([A-Z]{1,3}-?\d{3,4}p?\)\s*`
	reNZBSiteTag          = `<[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}>`
	reNZBPar2Suffix       = `\.vol\d+\+\d+\.par2$|\.par2$`
	reNZBArchiveSuffix    = `\.(rar|r\d{2}|\d{3}|zip|7z)$`
	reNZBYear             = `(?:19|20)\d{2}`
	reNZBResolution       = `(?i)(1080p|720p|2160p|4k|uhd|480p|576p)`
	reNZBSeasonEpisode    = `(?i)S\d{1,2}E\d{1,2}|\d{1,2}x\d{2}`
)

// compileNZBPatterns returns the shared NZB pattern set, fetching each
// compiled regexp from the global cache (compiled once, reused on every call).
func compileNZBPatterns() *nzbPatterns {
	return &nzbPatterns{
		partNumberPrefix: database.GetCachedRegexp(reNZBPartNumberPrefix),
		partNumberSuffix: database.GetCachedRegexp(reNZBPartNumberSuffix),
		usenetMetaPrefix: database.GetCachedRegexp(reNZBUsenetMetaPrefix),
		quotedFile:       database.GetCachedRegexp(reNZBQuotedFile),
		yenc:             database.GetCachedRegexp(reNZBYenc),
		qualityPrefix:    database.GetCachedRegexp(reNZBQualityPrefix),
		siteTag:          database.GetCachedRegexp(reNZBSiteTag),
		par2Suffix:       database.GetCachedRegexp(reNZBPar2Suffix),
		archiveSuffix:    database.GetCachedRegexp(reNZBArchiveSuffix),
	}
}

// Clean preprocesses an NZB-style filename to extract the actual filename.
// It handles various Usenet subject line formats:
//   - [001/279] "The.Matrix.1999.1080p.BluRay.x264.mkv" yEnc
//   - [22569]-[FULL]-[#a.b.hdtv.x264@EFNet]-[ The.Matrix.1999 ]-[136/143] - "file.par2" yEnc
//   - (TR-1080p)[01/80] - "The.Matrix.1999.1080p.par2" yEnc
//   - <kere.ws> - Filme - Title - [01/58] - "file.par2" yEnc
func (n *NZBPreprocessor) Clean(input string) string {
	result := input

	// First, try to extract a quoted filename - this is the most reliable
	if matches := n.patterns.quotedFile.FindStringSubmatch(result); len(matches) > 1 {
		// Found quoted filename, extract it
		quotedName := matches[1]

		// Clean up .par2/.rar suffix if present (these are repair/archive files)
		cleanedQuotedName := n.patterns.par2Suffix.ReplaceAllLiteralString(quotedName, "")
		// cleanedQuotedName = n.patterns.archiveSuffix.ReplaceAllLiteralString(cleanedQuotedName, "")

		// If the quoted filename looks like a short par2/rar reference (not the actual content name),
		// we need to extract the content name from elsewhere in the subject line
		// Check if the cleaned quoted name looks like a meaningful media filename (has year or resolution patterns)
		if n.looksLikeMediaFilename(cleanedQuotedName) && len(cleanedQuotedName) > 10 {
			return strings.TrimSpace(cleanedQuotedName)
		}

		// Otherwise, fall through to extract from rest of subject line
	}

	// Remove yEnc suffix
	if n.patterns.yenc.MatchString(result) {
		result = n.patterns.yenc.ReplaceAllLiteralString(result, "")
	}

	// Remove site tags like <kere.ws>
	if n.patterns.siteTag.MatchString(result) {
		result = n.patterns.siteTag.ReplaceAllLiteralString(result, "")
	}

	// Remove quality prefix like (TR-1080p)
	if n.patterns.qualityPrefix.MatchString(result) {
		result = n.patterns.qualityPrefix.ReplaceAllLiteralString(result, "")
	}

	// Remove part number prefix [001/279]
	if n.patterns.partNumberPrefix.MatchString(result) {
		result = n.patterns.partNumberPrefix.ReplaceAllLiteralString(result, "")
	}

	// Remove Usenet metadata brackets
	if n.patterns.usenetMetaPrefix.MatchString(result) {
		result = n.patterns.usenetMetaPrefix.ReplaceAllLiteralString(result, "")
	}

	// Remove part number suffix - [01/58] -
	if n.patterns.partNumberSuffix.MatchString(result) {
		result = n.patterns.partNumberSuffix.ReplaceAllLiteralString(result, " ")
	}

	// Remove .par2/.rar suffixes
	if n.patterns.par2Suffix.MatchString(result) {
		result = n.patterns.par2Suffix.ReplaceAllLiteralString(result, "")
	}

	// result = n.patterns.archiveSuffix.ReplaceAllLiteralString(result, "")

	// Remove quoted filenames (we already tried to use them, they weren't useful)
	if n.patterns.quotedFile.MatchString(result) {
		result = n.patterns.quotedFile.ReplaceAllLiteralString(result, "")
	}

	// Clean up any remaining artifacts
	// Remove " - " sections that are just metadata markers
	result = strings.ReplaceAll(result, " - - ", " - ")

	// Try to find a media filename pattern in the remaining content
	// Split by " - " and find the segment that looks like a media filename
	segments := strings.SplitSeq(result, " - ")
	for seg := range segments {
		seg = strings.TrimSpace(seg)
		if n.looksLikeMediaFilename(seg) && len(seg) > 10 {
			return seg
		}
	}

	// Remove leading/trailing dashes and spaces
	result = strings.Trim(result, " -")

	// Collapse multiple spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	return strings.TrimSpace(result)
}

// IsNZBFormat checks if the input appears to be an NZB-style subject line.
func (n *NZBPreprocessor) IsNZBFormat(input string) bool {
	// Check for common NZB patterns
	if n.patterns.partNumberPrefix.MatchString(input) {
		return true
	}

	if n.patterns.quotedFile.MatchString(input) {
		return true
	}

	if n.patterns.yenc.MatchString(input) {
		return true
	}

	if n.patterns.usenetMetaPrefix.MatchString(input) {
		return true
	}

	if n.patterns.siteTag.MatchString(input) {
		return true
	}

	return false
}

// CleanNZB is a convenience function using the default preprocessor.
func CleanNZB(input string) string {
	return defaultNZBPreprocessor.Clean(input)
}

// IsNZBFormatString is a convenience function using the default preprocessor.
func IsNZBFormatString(input string) bool {
	return defaultNZBPreprocessor.IsNZBFormat(input)
}

// looksLikeMediaFilename checks if a string looks like an actual media filename
// rather than just a short par2/rar reference name.
func (*NZBPreprocessor) looksLikeMediaFilename(name string) bool {
	// Check for year pattern (1900-2099)
	if database.GetCachedRegexp(reNZBYear).MatchString(name) {
		return true
	}

	// Check for resolution patterns
	if database.GetCachedRegexp(reNZBResolution).MatchString(name) {
		return true
	}

	// Check for season/episode patterns
	if database.GetCachedRegexp(reNZBSeasonEpisode).MatchString(name) {
		return true
	}

	return false
}
