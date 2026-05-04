package parser_v2

import (
	"regexp"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// PatternType defines the type of quality pattern.
type PatternType int

const (
	PatternTypeResolution PatternType = 1
	PatternTypeQuality    PatternType = 2
	PatternTypeCodec      PatternType = 3
	PatternTypeAudio      PatternType = 4
)

// DBPattern represents a pattern loaded from the database.
type DBPattern struct {
	Name       string
	Regex      *regexp.Regexp
	RegexStr   string
	Strings    []string // Lowercase match strings
	Type       PatternType
	Priority   int
	RegexGroup int
	ID         uint
	UseRegex   bool
}

// DBPatternStore holds patterns loaded from the database.
// It provides thread-safe access to patterns for parsing.
type DBPatternStore struct {
	mu sync.RWMutex

	// Patterns by type
	Resolutions []DBPattern
	Qualities   []DBPattern
	Codecs      []DBPattern
	Audios      []DBPattern

	// Flattened string arrays for fast matching
	ResolutionStrings []string
	QualityStrings    []string
	CodecStrings      []string
	AudioStrings      []string

	// Loaded flag
	loaded bool
}

// defaultPatternStore is the global pattern store.
var defaultPatternStore = &DBPatternStore{}

// GetPatternStore returns the default pattern store.
func GetPatternStore() *DBPatternStore {
	return defaultPatternStore
}

// LoadDBPatterns loads patterns from the database into the store.
// It uses double-check locking for thread-safe initialization.
func (ps *DBPatternStore) LoadDBPatterns() {
	// Fast path: check if already loaded
	ps.mu.RLock()

	if ps.loaded {
		ps.mu.RUnlock()
		return
	}

	ps.mu.RUnlock()

	// Slow path: acquire write lock
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Double-check after acquiring write lock
	if ps.loaded {
		return
	}

	// Load all pattern types from database.DBConnect
	ps.Resolutions, ps.ResolutionStrings = loadPatternType(
		database.DBConnect.GetresolutionsIn,
		database.DBConnect.ResolutionStrIn,
		PatternTypeResolution,
	)
	ps.Qualities, ps.QualityStrings = loadPatternType(
		database.DBConnect.GetqualitiesIn,
		database.DBConnect.QualityStrIn,
		PatternTypeQuality,
	)
	ps.Codecs, ps.CodecStrings = loadPatternType(
		database.DBConnect.GetcodecsIn,
		database.DBConnect.CodecStrIn,
		PatternTypeCodec,
	)
	ps.Audios, ps.AudioStrings = loadPatternType(
		database.DBConnect.GetaudiosIn,
		database.DBConnect.AudioStrIn,
		PatternTypeAudio,
	)

	ps.loaded = true
}

// loadPatternType converts database.Qualities slice to DBPattern slice.
func loadPatternType(
	dbPatterns []database.Qualities,
	stringsIn []string,
	pType PatternType,
) ([]DBPattern, []string) {
	patterns := make([]DBPattern, 0, len(dbPatterns))

	for i := range dbPatterns {
		pattern := DBPattern{
			Name:       dbPatterns[i].Name,
			RegexStr:   dbPatterns[i].Regex,
			Strings:    dbPatterns[i].StringsLowerSplitted,
			Type:       pType,
			Priority:   dbPatterns[i].Priority,
			RegexGroup: dbPatterns[i].Regexgroup,
			ID:         dbPatterns[i].ID,
			UseRegex:   dbPatterns[i].UseRegex,
		}

		if dbPatterns[i].UseRegex && dbPatterns[i].Regex != "" {
			if re, err := regexp.Compile(dbPatterns[i].Regex); err == nil {
				pattern.Regex = re
			}
		}

		patterns = append(patterns, pattern)
	}

	// Copy the string slice
	strs := make([]string, len(stringsIn))
	copy(strs, stringsIn)

	return patterns, strs
}

// Reload reloads patterns from the database.
// Call this after database.SetVars() is called to refresh patterns.
func (ps *DBPatternStore) Reload() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.loaded = false

	ps.Resolutions, ps.ResolutionStrings = loadPatternType(
		database.DBConnect.GetresolutionsIn,
		database.DBConnect.ResolutionStrIn,
		PatternTypeResolution,
	)
	ps.Qualities, ps.QualityStrings = loadPatternType(
		database.DBConnect.GetqualitiesIn,
		database.DBConnect.QualityStrIn,
		PatternTypeQuality,
	)
	ps.Codecs, ps.CodecStrings = loadPatternType(
		database.DBConnect.GetcodecsIn,
		database.DBConnect.CodecStrIn,
		PatternTypeCodec,
	)
	ps.Audios, ps.AudioStrings = loadPatternType(
		database.DBConnect.GetaudiosIn,
		database.DBConnect.AudioStrIn,
		PatternTypeAudio,
	)

	ps.loaded = true
}

// IsLoaded returns whether patterns have been loaded.
func (ps *DBPatternStore) IsLoaded() bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.loaded
}

// getPatterns returns the pattern slice for the given type.
func (ps *DBPatternStore) getPatterns(patternType PatternType) []DBPattern {
	switch patternType {
	case PatternTypeResolution:
		return ps.Resolutions
	case PatternTypeQuality:
		return ps.Qualities
	case PatternTypeCodec:
		return ps.Codecs
	case PatternTypeAudio:
		return ps.Audios
	default:
		return nil
	}
}

// MatchString performs fast string matching (similar to Parsegroup).
// It returns the matched pattern name and the match indices, or empty string and -1 if no match.
func (ps *DBPatternStore) MatchString(
	input string,
	patternType PatternType,
) (name string, matchIdx int, matchEnd int) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	inputLower := strings.ToLower(input)
	patterns := ps.getPatterns(patternType)

	for i := range patterns {
		for j := range patterns[i].Strings {
			idx := strings.Index(inputLower, patterns[i].Strings[j])
			if idx == -1 {
				continue
			}

			endIdx := idx + len(patterns[i].Strings[j])

			// Word boundary check (similar to checkDigitLetter in original)
			if endIdx < len(inputLower) && isDigitOrLetter(inputLower[endIdx]) {
				continue
			}

			if idx > 0 && isDigitOrLetter(inputLower[idx-1]) {
				continue
			}

			return patterns[i].Name, idx, endIdx
		}
	}

	return "", -1, -1
}

// MatchRegex performs regex matching on the input.
// It returns the matched pattern name, capture group value, and match indices.
func (ps *DBPatternStore) MatchRegex(
	input string,
	patternType PatternType,
) (name string, value string, matchIdx int, matchEnd int) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	patterns := ps.getPatterns(patternType)

	for i := range patterns {
		if !patterns[i].UseRegex || patterns[i].Regex == nil {
			continue
		}

		loc := patterns[i].Regex.FindStringSubmatchIndex(input)
		if loc == nil {
			continue
		}

		// Get the capture group value
		groupIdx := patterns[i].RegexGroup
		if groupIdx == 0 {
			groupIdx = 1 // Default to first capture group
		}

		startIdx := loc[0]
		endIdx := loc[1]
		capturedValue := ""

		// Get the captured group if available
		groupStart := groupIdx * 2

		groupEnd := groupStart + 1
		if groupEnd < len(loc) && loc[groupStart] != -1 {
			capturedValue = input[loc[groupStart]:loc[groupEnd]]
		}

		return patterns[i].Name, capturedValue, startIdx, endIdx
	}

	return "", "", -1, -1
}

// GetPatternByName returns a pattern by name and type.
func (ps *DBPatternStore) GetPatternByName(name string, patternType PatternType) *DBPattern {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	patterns := ps.getPatterns(patternType)

	for i := range patterns {
		if strings.EqualFold(patterns[i].Name, name) {
			return &patterns[i]
		}
	}

	return nil
}

// GetPatternID returns the database ID for a matched pattern name.
func (ps *DBPatternStore) GetPatternID(name string, patternType PatternType) uint {
	if p := ps.GetPatternByName(name, patternType); p != nil {
		return p.ID
	}

	return 0
}

// GetPatternPriority returns the priority for a matched pattern name.
func (ps *DBPatternStore) GetPatternPriority(name string, patternType PatternType) int {
	if p := ps.GetPatternByName(name, patternType); p != nil {
		return p.Priority
	}

	return 0
}

// isDigitOrLetter checks if a byte is a digit or letter (for word boundary check).
func isDigitOrLetter(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// LoadDBPatterns is a convenience function to load patterns using the default store.
func LoadDBPatterns() {
	defaultPatternStore.LoadDBPatterns()
}

// ReloadDBPatterns reloads patterns in the default store.
func ReloadDBPatterns() {
	defaultPatternStore.Reload()
}
