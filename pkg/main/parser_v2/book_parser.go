package parser_v2

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// BookParser handles parsing of ebook filenames and metadata.
type BookParser struct {
	patterns *bookPatterns
}

// bookPatterns contains compiled regex patterns for book parsing.
type bookPatterns struct {
	isbn13     *regexp.Regexp
	isbn10     *regexp.Regexp
	asin       *regexp.Regexp
	year       *regexp.Regexp
	series     *regexp.Regexp
	seriesNum  *regexp.Regexp
	author     *regexp.Regexp
	retail     *regexp.Regexp
	group      *regexp.Regexp
	authorDash *regexp.Regexp
}

// NewBookParser creates a new BookParser with compiled patterns.
func NewBookParser() *BookParser {
	return &BookParser{
		patterns: compileBookPatterns(),
	}
}

// Book pattern strings as constants so they can be used as cache keys.
const (
	reBookISBN13     = `(?i)(?:isbn[:\s-]*)?((?:978|979)[\d\s-]{10,16})`
	reBookISBN10     = `(?i)(?:isbn[:\s-]?)?\d[\s-]?\d{2}[\s-]?\d{5}[\s-]?\d{2}[\s-]?[\dxX]`
	reBookASIN       = `(?i)(?:asin[:\s-]?)?B0[A-Z0-9]{8}`
	reBookYear       = `[\(\[]?((?:19|20)\d{2})[\)\]]?`
	reBookSeries     = `(?i)(?:\(|\[)?\s*([^()\[\]]+?)\s*(?:book|#|,?\s*no\.?|,?\s*vol\.?|,?\s*volume)\s*(\d+(?:\.\d+)?)\s*(?:\)|\])?`
	reBookSeriesNum  = `(?i)(?:book|#|no\.?|vol\.?|volume)\s*(\d+(?:\.\d+)?)`
	reBookAuthor     = `^([^-]+?)\s*-\s*`
	reBookRetail     = `(?i)[\[\(]?retail[\]\)]?`
	reBookGroup      = `(?i)[\[\(]([a-z0-9_-]+)[\]\)]$`
	reBookAuthorDash = `^(.+?)\s+-\s+(.+)$`
	reBookStripISBN  = `(?i)isbn[:\s-]*`
	reBookStripASIN  = `(?i)asin[:\s-]*`
)

// compileBookPatterns returns the shared book pattern set, fetching each
// compiled regexp from the global cache (compiled once, reused on every call).
func compileBookPatterns() *bookPatterns {
	return &bookPatterns{
		isbn13:     database.GetCachedRegexp(reBookISBN13),
		isbn10:     database.GetCachedRegexp(reBookISBN10),
		asin:       database.GetCachedRegexp(reBookASIN),
		year:       database.GetCachedRegexp(reBookYear),
		series:     database.GetCachedRegexp(reBookSeries),
		seriesNum:  database.GetCachedRegexp(reBookSeriesNum),
		author:     database.GetCachedRegexp(reBookAuthor),
		retail:     database.GetCachedRegexp(reBookRetail),
		group:      database.GetCachedRegexp(reBookGroup),
		authorDash: database.GetCachedRegexp(reBookAuthorDash),
	}
}

// Parse parses a book filename and returns the extracted information.
func (bp *BookParser) Parse(filename string) *BookParseResult {
	result := &BookParseResult{
		ParseResult: ParseResult{
			SourceFile: filename,
			MediaType:  MediaTypeBook,
		},
	}

	// Get extension and base name
	ext := filepath.Ext(filename)

	result.Format = logger.ExtToFormat(ext)

	name := strings.TrimSuffix(filename, ext)

	// Clean common separators
	name = strings.ReplaceAll(name, "_", " ")

	cleanedName := name

	// Extract ISBN-13
	if match := bp.patterns.isbn13.FindString(name); match != "" {
		result.ISBN13 = bp.normalizeISBN(match)
		cleanedName = strings.Replace(cleanedName, match, "", 1)

		result.Confidence += 0.3
	}

	// Extract ISBN-10
	if match := bp.patterns.isbn10.FindString(name); match != "" && result.ISBN13 == "" {
		result.ISBN10 = bp.normalizeISBN(match)
		cleanedName = strings.Replace(cleanedName, match, "", 1)

		result.Confidence += 0.2
	}

	// Extract ASIN
	if match := bp.patterns.asin.FindString(name); match != "" {
		result.ASIN = bp.extractASIN(match)
		cleanedName = strings.Replace(cleanedName, match, "", 1)

		result.Confidence += 0.2
	}

	// Check for retail indicator
	if bp.patterns.retail.MatchString(name) {
		result.IsRetail = true
		cleanedName = bp.patterns.retail.ReplaceAllString(cleanedName, "")
	}

	// Extract release group (typically at the end)
	if matches := bp.patterns.group.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.ReleaseGroup = matches[1]
		cleanedName = bp.patterns.group.ReplaceAllString(cleanedName, "")
	}

	// Extract year
	if matches := bp.patterns.year.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.Year = parseInt(matches[1])
		// Only remove year if it's in parentheses/brackets or standalone
		if strings.Contains(matches[0], "(") || strings.Contains(matches[0], "[") {
			cleanedName = strings.Replace(cleanedName, matches[0], "", 1)
		}
	}

	// Extract series information
	if matches := bp.patterns.series.FindStringSubmatch(cleanedName); len(matches) > 2 {
		result.Series = strings.TrimSpace(matches[1])
		result.SeriesPosition = matches[2]
		cleanedName = bp.patterns.series.ReplaceAllString(cleanedName, "")
	} else if matches := bp.patterns.seriesNum.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.SeriesPosition = matches[1]
		cleanedName = bp.patterns.seriesNum.ReplaceAllString(cleanedName, "")
	}

	// Try to split author from title
	bp.extractAuthorTitle(cleanedName, result)

	// Clean up title
	result.Title = cleanTitle(result.Title)

	// Set confidence based on extracted data
	result.Confidence = bp.calculateConfidence(result)

	return result
}

// ParseWithPath parses a book filename with its full path for additional context.
func (bp *BookParser) ParseWithPath(fullpath string) *BookParseResult {
	filename := filepath.Base(fullpath)
	result := bp.Parse(filename)

	result.SourcePath = fullpath

	// Try to extract additional info from parent directories
	dir := filepath.Dir(fullpath)
	parentDir := filepath.Base(dir)

	// If we didn't find an author, check parent directory
	if result.Author == "" && parentDir != "." && parentDir != "" {
		// Common pattern: Author/Book Title.epub
		if !isNumericOnly(parentDir) {
			// Check if parent looks like an author name
			if bp.looksLikeAuthor(parentDir) {
				result.Author = cleanTitle(parentDir)
			}
		}
	}

	// Check if grandparent directory could be author (Author/Series/Book.epub)
	if result.Author == "" {
		grandParent := filepath.Base(filepath.Dir(dir))
		if grandParent != "." && grandParent != "" && bp.looksLikeAuthor(grandParent) {
			result.Author = cleanTitle(grandParent)
			// Parent might be series name
			if result.Series == "" && parentDir != "." && !isNumericOnly(parentDir) {
				result.Series = cleanTitle(parentDir)
			}
		}
	}

	return result
}

// extractAuthorTitle attempts to separate author from title in the cleaned name.
func (bp *BookParser) extractAuthorTitle(name string, result *BookParseResult) {
	name = strings.TrimSpace(name)

	// Try "Author - Title" pattern
	if matches := bp.patterns.authorDash.FindStringSubmatch(name); len(matches) > 2 {
		potentialAuthor := strings.TrimSpace(matches[1])
		potentialTitle := strings.TrimSpace(matches[2])

		// Validate it looks like an author name
		if bp.looksLikeAuthor(potentialAuthor) {
			result.Author = potentialAuthor
			result.Title = potentialTitle

			// Check for multiple authors (comma-separated)
			if strings.Contains(result.Author, ",") {
				authors := strings.SplitSeq(result.Author, ",")
				for a := range authors {
					a = strings.TrimSpace(a)
					if a != "" {
						result.Authors = append(result.Authors, a)
					}
				}

				if len(result.Authors) > 0 {
					result.Author = result.Authors[0]
				}
			}

			return
		}
	}

	// No author found, treat entire name as title
	result.Title = name
}

// looksLikeAuthor checks if a string looks like an author name.
func (bp *BookParser) looksLikeAuthor(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}

	// Should contain at least one space (first + last name) or be a known format
	// Unless it's a single-name author like "Plato"
	wordCount := len(strings.Fields(s))
	if wordCount >= 2 {
		return true
	}

	// Single word - check if it's capitalized like a name
	if wordCount == 1 {
		// Should start with uppercase
		return s[0] >= 'A' && s[0] <= 'Z'
	}

	return false
}

// normalizeISBN removes hyphens and spaces from ISBN and returns clean format.
func (bp *BookParser) normalizeISBN(isbn string) string {
	// Remove "ISBN:" prefix if present
	isbn = database.GetCachedRegexp(reBookStripISBN).ReplaceAllString(isbn, "")
	// Remove hyphens and spaces
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")

	return strings.ToUpper(isbn)
}

// extractASIN extracts the ASIN from a match.
func (bp *BookParser) extractASIN(match string) string {
	// Remove "ASIN:" prefix if present
	return strings.ToUpper(
		strings.TrimSpace(database.GetCachedRegexp(reBookStripASIN).ReplaceAllString(match, "")),
	)
}

// calculateConfidence calculates the confidence score based on extracted data.
func (bp *BookParser) calculateConfidence(result *BookParseResult) float64 {
	var conf float64

	// Title is essential
	if result.Title != "" {
		conf += 0.3
	}

	// Author is very helpful
	if result.Author != "" {
		conf += 0.2
	}

	// ISBNs are strong identifiers
	if result.ISBN13 != "" {
		conf += 0.25
	} else if result.ISBN10 != "" {
		conf += 0.2
	}

	// ASIN is a good identifier
	if result.ASIN != "" {
		conf += 0.15
	}

	// Year adds context
	if result.Year > 0 {
		conf += 0.05
	}

	// Series info adds context
	if result.Series != "" {
		conf += 0.05
	}

	// Cap at 1.0
	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// ValidateISBN13 validates an ISBN-13 checksum.
func ValidateISBN13(isbn string) bool {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")

	if len(isbn) != 13 {
		return false
	}

	var sum int
	for i, c := range isbn {
		if c < '0' || c > '9' {
			return false
		}

		digit := int(c - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}

	return sum%10 == 0
}

// ValidateISBN10 validates an ISBN-10 checksum.
func ValidateISBN10(isbn string) bool {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	isbn = strings.ToUpper(isbn)

	if len(isbn) != 10 {
		return false
	}

	var sum int
	for i, c := range isbn {
		var digit int
		if c >= '0' && c <= '9' {
			digit = int(c - '0')
		} else if c == 'X' && i == 9 {
			digit = 10
		} else {
			return false
		}

		sum += (10 - i) * digit
	}

	return sum%11 == 0
}

// ISBN10toISBN13 converts an ISBN-10 to ISBN-13.
func ISBN10toISBN13(isbn10 string) string {
	isbn10 = strings.ReplaceAll(isbn10, "-", "")
	isbn10 = strings.ReplaceAll(isbn10, " ", "")

	if len(isbn10) != 10 {
		return ""
	}

	// Take first 9 digits and prepend 978
	isbn13 := "978" + isbn10[:9]

	// Calculate check digit
	var sum int
	for i, c := range isbn13 {
		digit := int(c - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}

	checkDigit := (10 - (sum % 10)) % 10

	return isbn13 + string('0'+byte(checkDigit))
}

// cleanTitle cleans up a title string.
func cleanTitle(title string) string {
	// Remove extra whitespace
	title = logger.JoinStringsSep(strings.Fields(title), " ")
	// Remove leading/trailing punctuation
	title = strings.Trim(title, ".-_,;: ")
	return title
}

// parseInt parses a string to int, returning 0 on error.
func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}

	return result
}

// isNumericOnly checks if a string contains only digits.
func isNumericOnly(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}
