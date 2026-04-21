package parser_v2

import (
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// MusicParser handles parsing of music filenames and album collections.
type MusicParser struct {
	patterns       *musicPatterns
	runtimeMatcher *RuntimeMatcher
}

// musicPatterns contains compiled regex patterns for music parsing.
type musicPatterns struct {
	year        *regexp.Regexp
	trackNumber *regexp.Regexp
	discNumber  *regexp.Regexp
	discTrack   *regexp.Regexp
	catalogNum  *regexp.Regexp
	format      *regexp.Regexp
	bitrate     *regexp.Regexp
	sampleRate  *regexp.Regexp
	bitDepth    *regexp.Regexp
	releaseType *regexp.Regexp
	group       *regexp.Regexp
	artistDash  *regexp.Regexp
	artistTitle *regexp.Regexp
	vinyl       *regexp.Regexp
	upc         *regexp.Regexp
	isrc        *regexp.Regexp
	feat        *regexp.Regexp
	mbid        *regexp.Regexp
	sceneTags   *regexp.Regexp // Scene release tags like -FLAC-, -WEB-, -OST-
	sceneGroup  *regexp.Regexp // Release group at end of string
}

// NewMusicParser creates a new MusicParser with compiled patterns.
func NewMusicParser() *MusicParser {
	return &MusicParser{
		patterns:       compileMusicPatterns(),
		runtimeMatcher: DefaultRuntimeMatcher(),
	}
}

// NewMusicParserWithMatcher creates a new MusicParser with a custom runtime matcher.
func NewMusicParserWithMatcher(rm *RuntimeMatcher) *MusicParser {
	return &MusicParser{
		patterns:       compileMusicPatterns(),
		runtimeMatcher: rm,
	}
}

// Music pattern strings as constants so they can be used as cache keys.
const (
	reMusicYear        = `[\(\[]?((?:19|20)\d{2})[\)\]]?`
	reMusicTrackNumber = `(?i)(?:track[\s._-]?)?(\d{1,3})[\s._-]`
	reMusicDiscNumber  = `(?i)(?:\[|\()?(?:disc|disk|cd|d)[\s._-]?(\d+)(?:\]|\))?`
	reMusicDiscTrack   = `^(\d{1,2})[-.](\d{2,3})\s`
	reMusicCatalogNum  = `(?i)[\[\(]([A-Z]{2,}[\s-]?\d+(?:[A-Z])?|[A-Z]+\d{2,}[A-Z]*)[\]\)]`
	reMusicFormat      = `(?i)(?:[\[\(\s-])(flac|mp3|m4a|ogg|opus|wav|alac|aac|ape|wma|aiff|wv|320|256|192|128|v0|v2|24bit|16bit|24-bit|16-bit)(?:[\]\)\s-]|$)`
	reMusicBitrate     = `(?i)(\d{2,4})\s*(?:kbps|kb\/s|kbit)`
	reMusicSampleRate  = `(?i)(\d{2,3}(?:\.\d)?)\s*(?:khz)`
	reMusicBitDepth    = `(?i)(\d{2})[\s-]?bit`
	reMusicReleaseType = `(?i)[\[\(\s](album|ep|single|compilation|soundtrack|ost|live|bootleg|demo|mixtape|remix|deluxe|remaster(?:ed)?|limited\s*edition)[\]\)\s]`
	reMusicGroup       = `(?i)-([a-z0-9_]+)$`
	reMusicArtistDash  = `^(.+?)\s+-\s+(.+)$`
	reMusicArtistTitle = `^(.+?)[-_](.+)$`
	reMusicVinyl       = `^([A-D])(\d{1,2})[\s._-]`
	reMusicUPC         = `(?:UPC[:\s-]?)?(\d{12,13})`
	reMusicISRC        = `(?i)(?:ISRC[:\s-]?)?([A-Z]{2}[A-Z0-9]{3}\d{7})`
	reMusicFeat        = `(?i)[\(\[\s](?:feat\.?|featuring|ft\.?|with)\s+([^\)\]]+)[\)\]]?`
	reMusicMBID        = `(?i)(?:mbid[:\s-]?)?([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`
	reMusicSceneTags   = `(?i)-(?:FLAC|MP3|AAC|OGG|OPUS|WAV|ALAC|APE|WMA|AIFF|M4A|WEB|CD|VINYL|TAPE|CABLE|HDTV|SAT|DVDRip|BDRip|16BIT|24BIT|320|256|192|128|96|88|48|44|V0|V2|OST|EP|LP|SINGLE|ALBUM|LIVE|BOOTLEG|DEMO|REMIX|DELUXE|REMASTERED?|REISSUE|PROPER|REPACK|READNFO|INT|iNT|RETAIL|ADVANCE|PROMO|LIMITED|19\d{2}|20\d{2})(?:-|$)`
	reMusicSceneGroup  = `(?i)-[A-Z0-9_]{2,12}(?:\s+INT)?$`
)

// compileMusicPatterns returns the shared music pattern set, fetching each
// compiled regexp from the global cache (compiled once, reused on every call).
func compileMusicPatterns() *musicPatterns {
	return &musicPatterns{
		year:        database.GetCachedRegexp(reMusicYear),
		trackNumber: database.GetCachedRegexp(reMusicTrackNumber),
		discNumber:  database.GetCachedRegexp(reMusicDiscNumber),
		discTrack:   database.GetCachedRegexp(reMusicDiscTrack),
		catalogNum:  database.GetCachedRegexp(reMusicCatalogNum),
		format:      database.GetCachedRegexp(reMusicFormat),
		bitrate:     database.GetCachedRegexp(reMusicBitrate),
		sampleRate:  database.GetCachedRegexp(reMusicSampleRate),
		bitDepth:    database.GetCachedRegexp(reMusicBitDepth),
		releaseType: database.GetCachedRegexp(reMusicReleaseType),
		group:       database.GetCachedRegexp(reMusicGroup),
		artistDash:  database.GetCachedRegexp(reMusicArtistDash),
		artistTitle: database.GetCachedRegexp(reMusicArtistTitle),
		vinyl:       database.GetCachedRegexp(reMusicVinyl),
		upc:         database.GetCachedRegexp(reMusicUPC),
		isrc:        database.GetCachedRegexp(reMusicISRC),
		feat:        database.GetCachedRegexp(reMusicFeat),
		mbid:        database.GetCachedRegexp(reMusicMBID),
		sceneTags:   database.GetCachedRegexp(reMusicSceneTags),
		sceneGroup:  database.GetCachedRegexp(reMusicSceneGroup),
	}
}

// Parse parses a single music filename (typically a track).
func (mp *MusicParser) Parse(filename string) *MusicParseResult {
	result := &MusicParseResult{
		ParseResult: ParseResult{
			SourceFile: filename,
			MediaType:  MediaTypeMusic,
		},
	}

	// Get extension and base name
	ext := filepath.Ext(filename)

	result.Format = logger.ExtToFormat(ext)

	name := strings.TrimSuffix(filename, ext)

	// Check if lossless
	result.IsLossless = IsLosslessAudioExtension(ext)

	// Clean underscores
	name = strings.ReplaceAll(name, "_", " ")

	cleanedName := name

	// Extract disc-track combined pattern first (e.g., "1-01 Track.mp3")
	var trackInfo TrackInfo
	if matches := mp.patterns.discTrack.FindStringSubmatch(name); len(matches) > 2 {
		trackInfo.DiscNumber = parseInt(matches[1])
		trackInfo.TrackNumber = parseInt(matches[2])
		cleanedName = mp.patterns.discTrack.ReplaceAllString(cleanedName, "")
	}

	// Extract track number if not already found
	if trackInfo.TrackNumber == 0 {
		if matches := mp.patterns.trackNumber.FindStringSubmatch(cleanedName); len(matches) > 1 {
			trackInfo.TrackNumber = parseInt(matches[1])
			cleanedName = mp.patterns.trackNumber.ReplaceAllString(cleanedName, "")
		}
	}

	// Extract disc number if not already found
	if trackInfo.DiscNumber == 0 {
		if matches := mp.patterns.discNumber.FindStringSubmatch(cleanedName); len(matches) > 1 {
			trackInfo.DiscNumber = parseInt(matches[1])
			cleanedName = mp.patterns.discNumber.ReplaceAllString(cleanedName, "")
		}
	}

	// Check for vinyl side notation
	if matches := mp.patterns.vinyl.FindStringSubmatch(name); len(matches) > 2 {
		// Convert vinyl side to disc (A=1, B=2, etc.)
		trackInfo.DiscNumber = int(matches[1][0] - 'A' + 1)
		trackInfo.TrackNumber = parseInt(matches[2])
	}

	// Extract ISRC
	if matches := mp.patterns.isrc.FindStringSubmatch(name); len(matches) > 1 {
		trackInfo.ISRC = matches[1]
	}

	// Extract featured artist
	if matches := mp.patterns.feat.FindStringSubmatch(cleanedName); len(matches) > 1 {
		trackInfo.Artist = strings.TrimSpace(matches[1])
		cleanedName = mp.patterns.feat.ReplaceAllString(cleanedName, " ")
	}

	// The remaining name is the track title
	trackInfo.Title = cleanTitle(cleanedName)
	trackInfo.Filename = filename

	result.Tracks = []TrackInfo{trackInfo}

	// Calculate basic confidence
	result.Confidence = mp.calculateTrackConfidence(trackInfo)

	return result
}

// ParseAlbum parses an album from a directory of files.
func (mp *MusicParser) ParseAlbum(dirPath string, files []string) *MusicParseResult {
	if len(files) == 0 {
		return nil
	}

	result := &MusicParseResult{
		ParseResult: ParseResult{
			SourcePath: dirPath,
			MediaType:  MediaTypeMusic,
		},
	}

	// Parse directory name for album info
	dirName := filepath.Base(dirPath)
	mp.parseAlbumName(dirName, result)

	// Parse parent directory for artist info if needed
	if result.Artist == "" {
		parentDir := filepath.Base(filepath.Dir(dirPath))
		if parentDir != "." && parentDir != "" && looksLikeArtistName(parentDir) {
			result.Artist = cleanTitle(parentDir)
		}
	}

	// Parse all track files
	result.Tracks = make([]TrackInfo, 0, len(files))

	discSet := make(map[int]bool)
	formatSet := make(map[string]bool)

	for _, file := range files {
		ext := filepath.Ext(file)
		if !IsAudioExtension(ext) {
			continue
		}

		trackResult := mp.Parse(filepath.Base(file))
		if len(trackResult.Tracks) == 0 {
			continue
		}

		track := trackResult.Tracks[0]

		track.Filename = file
		result.Tracks = append(result.Tracks, track)

		if track.DiscNumber > 0 {
			discSet[track.DiscNumber] = true
		}

		formatSet[trackResult.Format] = true
	}

	// Sort tracks by disc and track number
	sort.Slice(result.Tracks, func(i, j int) bool {
		if result.Tracks[i].DiscNumber != result.Tracks[j].DiscNumber {
			return result.Tracks[i].DiscNumber < result.Tracks[j].DiscNumber
		}

		return result.Tracks[i].TrackNumber < result.Tracks[j].TrackNumber
	})

	// Determine format (use most common or first found)
	if len(formatSet) > 0 {
		for format := range formatSet {
			result.Format = format
			result.IsLossless = IsLosslessAudioExtension("." + format)
			break
		}
	}

	// Calculate totals
	result.TotalTracks = len(result.Tracks)

	result.TotalDiscs = len(discSet)
	if result.TotalDiscs == 0 && result.TotalTracks > 0 {
		result.TotalDiscs = 1
	}

	// Check completeness
	mp.checkCompleteness(result)

	// Calculate confidence
	result.Confidence = mp.calculateAlbumConfidence(result)

	return result
}

// ParseAlbumTitle parses an album title string (e.g., from NZB names like
// "Alabama Shakes - At The Loveless Barn (2014) FLAC") and extracts artist,
// album, year, and format information.
func (mp *MusicParser) ParseAlbumTitle(title string) *MusicParseResult {
	result := &MusicParseResult{
		ParseResult: ParseResult{
			SourceFile: title,
			MediaType:  MediaTypeMusic,
		},
	}

	mp.parseAlbumName(title, result)

	return result
}

// parseAlbumName extracts album information from a directory name.
func (mp *MusicParser) parseAlbumName(name string, result *MusicParseResult) {
	cleanedName := name

	// Clean underscores and dots
	cleanedName = strings.ReplaceAll(cleanedName, "_", " ")

	// Extract year
	if matches := mp.patterns.year.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.Year = parseInt(matches[1])
		if strings.Contains(matches[0], "(") || strings.Contains(matches[0], "[") {
			cleanedName = strings.Replace(cleanedName, matches[0], "", 1)
		}
	}

	// Extract release type - only extract for metadata, don't strip from title
	// if it's in parentheses (e.g., "(Live)" is often part of the album name)
	if matches := mp.patterns.releaseType.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.ReleaseType = normalizeReleaseType(matches[1])
		// Only strip release type if it's NOT in parentheses/brackets
		// "(Live)" should stay, but "-LIVE-" can be stripped by sceneTags later
		matchStr := matches[0]
		if !strings.HasPrefix(matchStr, "(") && !strings.HasPrefix(matchStr, "[") {
			cleanedName = strings.Replace(cleanedName, matchStr, " ", 1)
		}
	}

	// Extract catalog number
	if matches := mp.patterns.catalogNum.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.CatalogNumber = matches[1]
		cleanedName = mp.patterns.catalogNum.ReplaceAllString(cleanedName, " ")
	}

	// Extract format indicators
	if matches := mp.patterns.format.FindStringSubmatch(cleanedName); len(matches) > 1 {
		formatIndicator := strings.ToLower(matches[1])
		switch formatIndicator {
		case "flac", "wav", "alac", "aiff":
			result.IsLossless = true
			result.Format = formatIndicator

		case "24bit", "24-bit":
			result.BitDepth = 24
		case "16bit", "16-bit":
			result.BitDepth = 16
		case "320":
			result.Bitrate = 320
		case "v0":
			result.Bitrate = 245 // Approximate VBR V0
		}

		cleanedName = mp.patterns.format.ReplaceAllString(cleanedName, " ")
	}

	// Extract bitrate
	if matches := mp.patterns.bitrate.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.Bitrate = parseInt(matches[1])
		cleanedName = mp.patterns.bitrate.ReplaceAllString(cleanedName, " ")
	}

	// Extract sample rate
	if matches := mp.patterns.sampleRate.FindStringSubmatch(cleanedName); len(matches) > 1 {
		rate := parseFloat(matches[1])
		if rate < 1000 {
			result.SampleRate = int(rate * 1000)
		} else {
			result.SampleRate = int(rate)
		}
	}

	// Extract bit depth
	if matches := mp.patterns.bitDepth.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.BitDepth = parseInt(matches[1])
	}

	// Extract UPC
	if matches := mp.patterns.upc.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.UPC = matches[1]
		cleanedName = strings.Replace(cleanedName, matches[0], "", 1)
	}

	// Extract MusicBrainz ID
	if matches := mp.patterns.mbid.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.MusicBrainzReleaseID = matches[1]
		cleanedName = strings.Replace(cleanedName, matches[0], "", 1)
	}

	// Extract release group
	if matches := mp.patterns.group.FindStringSubmatch(cleanedName); len(matches) > 1 {
		result.ReleaseGroup = matches[1]
		cleanedName = mp.patterns.group.ReplaceAllString(cleanedName, "")
	}

	// Strip scene release tags (formats, sources, quality indicators)
	// This handles names like "Artist-Album-OST-CD-FLAC-1986-KINDA"
	// Loop to handle overlapping matches (pattern includes trailing dash)
	for {
		newName := mp.patterns.sceneTags.ReplaceAllString(cleanedName, "-")
		if newName == cleanedName {
			break
		}

		cleanedName = newName
	}

	// Strip release group at end
	if mp.patterns.sceneGroup.MatchString(cleanedName) {
		cleanedName = mp.patterns.sceneGroup.ReplaceAllString(cleanedName, "")
	}
	// Clean up multiple consecutive dashes and trailing dashes
	for strings.Contains(cleanedName, "--") {
		cleanedName = strings.ReplaceAll(cleanedName, "--", "-")
	}

	cleanedName = strings.Trim(cleanedName, "- ")

	// Try to split artist and album
	mp.extractArtistAlbum(cleanedName, result)

	// Set album title (use Title from ParseResult)
	if result.Title != "" {
		result.Album = result.Title
	}

	result.Title = result.Album
}

// extractArtistAlbum attempts to separate artist from album title.
func (mp *MusicParser) extractArtistAlbum(name string, result *MusicParseResult) {
	name = strings.TrimSpace(name)

	// Try "Artist - Album" pattern (with spaces around dash)
	if matches := mp.patterns.artistDash.FindStringSubmatch(name); len(matches) > 2 {
		// Skip this split if the "album" side starts with a bare year like "2007 - FLAC - GROUP".
		// That pattern means the whole string is "Title - Year - Format", not "Artist - Album".
		rhsTrimmed := strings.TrimSpace(matches[2])

		yearStart := len(rhsTrimmed) >= 4 &&
			(rhsTrimmed[0] == '1' || rhsTrimmed[0] == '2') &&
			rhsTrimmed[1] >= '0' && rhsTrimmed[1] <= '9' &&
			rhsTrimmed[2] >= '0' && rhsTrimmed[2] <= '9' &&
			rhsTrimmed[3] >= '0' && rhsTrimmed[3] <= '9' &&
			(len(rhsTrimmed) == 4 || rhsTrimmed[4] == ' ' || rhsTrimmed[4] == '-')

		// Skip this split if the LHS is a 4-digit year that was already extracted
		// (e.g. "1976 - I'd Rather Believe in You" → year=1976, not artist).
		lhsTrimmed := strings.TrimSpace(matches[1])
		lhsIsYear := result.Year > 0 &&
			len(lhsTrimmed) == 4 &&
			parseInt(lhsTrimmed) == result.Year

		// Skip if LHS is a bare integer (not a year) — it's a library index or
		// catalog prefix like "19 - Stef Bos - Album". Re-parse the RHS instead.
		lhsIsNumericPrefix := !lhsIsYear && isNumericOnly(lhsTrimmed)

		if !yearStart && !lhsIsYear && !lhsIsNumericPrefix {
			mp.setArtistAlbumFromMatches(matches[1], matches[2], result)
			return
		}
		if !yearStart && lhsIsNumericPrefix {
			mp.extractArtistAlbum(matches[2], result)
			return
		}
	}

	// Try "Artist-Album" or "Artist_Album" pattern (scene releases without spaces)
	// Only use this if we have at least one dash/underscore
	if strings.ContainsAny(name, "-_") {
		if matches := mp.patterns.artistTitle.FindStringSubmatch(name); len(matches) > 2 {
			potentialArtist := strings.TrimSpace(matches[1])
			potentialAlbum := strings.TrimSpace(matches[2])

			// Validate: artist should have reasonable length and look like a name/band
			// Album shouldn't be empty after cleaning
			if len(potentialArtist) >= 2 && len(potentialAlbum) >= 2 {
				// Clean up scene separators (dots, dashes, underscores) to spaces
				potentialArtist = strings.ReplaceAll(potentialArtist, ".", " ")
				potentialArtist = strings.ReplaceAll(potentialArtist, "_", " ")
				potentialArtist = cleanTitle(potentialArtist)

				potentialAlbum = strings.ReplaceAll(potentialAlbum, ".", " ")
				potentialAlbum = strings.ReplaceAll(potentialAlbum, "-", " ")
				potentialAlbum = strings.ReplaceAll(potentialAlbum, "_", " ")
				potentialAlbum = cleanTitle(potentialAlbum)

				mp.setArtistAlbumFromMatches(potentialArtist, potentialAlbum, result)

				return
			}
		}
	}

	// No split found, treat as album only
	result.Album = cleanTitle(name)
	result.Title = result.Album
}

// setArtistAlbumFromMatches sets artist and album from matched strings.
func (mp *MusicParser) setArtistAlbumFromMatches(artist, album string, result *MusicParseResult) {
	// Clean up scene separators (dots, underscores) in artist name
	artist = strings.ReplaceAll(artist, ".", " ")
	artist = strings.ReplaceAll(artist, "_", " ")
	// Clean up multiple spaces
	for strings.Contains(artist, "  ") {
		artist = strings.ReplaceAll(artist, "  ", " ")
	}

	result.Artist = strings.TrimSpace(artist)
	result.Album = cleanAlbumTitle(strings.TrimSpace(album))
	result.Title = result.Album

	// Check for multiple artists
	if !strings.Contains(result.Artist, " & ") && !strings.Contains(result.Artist, " x ") &&
		!strings.Contains(result.Artist, " and ") && !strings.Contains(result.Artist, " And ") {
		return
	}

	artists := splitArtists(result.Artist)

	result.Artists = artists
	if len(artists) > 0 {
		result.AlbumArtist = result.Artist
		result.Artist = artists[0]
	}
}

// cleanAlbumTitle removes format indicators and scene tags from album titles.
func cleanAlbumTitle(album string) string {
	// Replace dots and underscores with spaces (scene format cleanup)
	album = strings.ReplaceAll(album, ".", " ")
	album = strings.ReplaceAll(album, "_", " ")

	// Remove trailing year + release group pattern (e.g., "2003 CGPABN INT", "1990 EMG INT")
	// This must be done first before other patterns
	if loc := database.GetCachedRegexp(`\s+\d{4}\s+[A-Z0-9]+(?:\s+INT)?$`).FindStringIndex(album); loc != nil {
		album = album[:loc[0]]
	}

	// Remove trailing release group (short uppercase string at end, e.g., "EMG", "OBZEN", "r35")
	trailingGroup := database.GetCachedRegexp(`\s+[A-Za-z0-9]{2,10}$`)

	// Patterns to remove (run multiple times to catch all)
	patterns := []*regexp.Regexp{
		database.GetCachedRegexp(
			`(?i)(?:^|\s)(FLAC|MP3|M4A|OGG|OPUS|WAV|ALAC|AAC|APE|WMA|AIFF|WV)(?:\s|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s)(320|256|192|128|96|88|48|44|24|16|V0|V2|24BIT|16BIT|24-BIT|16-BIT|\d+KHZ|24BIT-\d+KHZ|\d+\s*kbps)(?:\s|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s)(WEB|CD|VINYL|TAPE|DVD|BLURAY|BLU-RAY|HDTV|SAT|OST|LP)(?:\s|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s)(INT|INTERNAL|PROPER|REPACK|RETAIL|ADVANCE|PROMO)(?:\s|$)`,
		),
		database.GetCachedRegexp(`(?i)(?:^|\s)(\d*CD\d*|\d*DISC\d*|CD\d+of\d+|Cd\d+of\d+)(?:\s|$)`),
	}

	// Run cleanup patterns multiple times to catch overlapping matches
	for range 3 {
		changed := false
		for _, p := range patterns {
			if p.MatchString(album) {
				album = p.ReplaceAllString(album, " ")
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Normalize "Vol N" volume indicators: "Vol.58" or "Vol 58" → "58"
	// Scene releases often use "Artist Vol.NN" while the DB title uses "Artist NN"
	if loc := database.GetCachedRegexp(`(?i)\bVol\.?\s+(\d+)\b`).FindStringSubmatchIndex(album); loc != nil {
		album = album[:loc[0]] + album[loc[2]:loc[3]] + album[loc[1]:]
	}

	// Remove trailing year patterns that might remain
	if loc := database.GetCachedRegexp(`\s+\d{4}$`).FindStringIndex(album); loc != nil {
		album = album[:loc[0]]
	}

	// Clean up multiple spaces
	for strings.Contains(album, "  ") {
		album = strings.ReplaceAll(album, "  ", " ")
	}

	album = strings.TrimSpace(album)

	// Check if trailing group looks like a scene group (all caps or common patterns)
	// Only remove if remaining album is at least 2 words
	words := strings.Fields(album)
	if len(words) >= 2 {
		lastWord := words[len(words)-1]
		// Check if last word looks like a scene group (short uppercase, or known patterns)
		if len(lastWord) >= 2 && len(lastWord) <= 10 {
			// Require at least one letter: pure numbers like "58" are volume/track numbers,
			// not scene groups, even though strings.ToUpper("58") == "58".
			hasLetter := strings.ContainsAny(
				lastWord,
				"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
			)

			isAllUpper := hasLetter && strings.ToUpper(lastWord) == lastWord
			if isAllUpper && !looksLikeAlbumWord(lastWord) {
				album = trailingGroup.ReplaceAllString(album, "")
			}
		}
	}

	return strings.TrimSpace(album)
}

// looksLikeAlbumWord returns true if the word could be part of an album title.
func looksLikeAlbumWord(word string) bool {
	// Common uppercase words that are valid album title parts
	validWords := map[string]bool{
		"I": true, "II": true, "III": true, "IV": true, "V": true,
		"VI": true, "VII": true, "VIII": true, "IX": true, "X": true,
		"EP": true, "LP": true, "DJ": true, "MC": true, "MR": true,
		"VS": true, "USA": true, "NYC": true, "LA": true, "UK": true,
		"OK": true, "TV": true, "CD": true, "DNA": true, "UFO": true,
		"AC": true, "DC": true,
	}

	return validWords[strings.ToUpper(word)]
}

// StripReleaseType removes release type indicators (Deluxe Edition, Remastered, etc.) from album titles.
// This is used for fallback matching when exact title match fails.
func StripReleaseType(album string) string {
	// Remove catalog numbers in parentheses like (824 150-2 M-1) or (B60F350E)
	if re := database.GetCachedRegexp(`\s*\([A-Z0-9][A-Z0-9\s\-]*\)`); re.MatchString(album) {
		album = re.ReplaceAllString(album, "")
	}

	patterns := []*regexp.Regexp{
		database.GetCachedRegexp(
			`(?i)(?:^|\s|-|_)(DELUXE\s*EDITION|DELUXE|REMASTERED|REMASTER|EXPANDED|LIMITED\s*EDITION|SPECIAL\s*EDITION|BONUS\s*TRACKS?)(?:\s|-|_|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s|-|_)(REISSUE|RETAIL|ADVANCE|PROMO|PROPER|REPACK|INT|INTERNAL)(?:\s|-|_|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s|-|_)(\d+(?:st|nd|rd|th)\s*Anniversary\s*Edition)(?:\s|-|_|$)`,
		),
		database.GetCachedRegexp(
			`(?i)(?:^|\s)(DE|US|UK|EU|JP|AU|CA|FR|IT|ES|NL|SE|NO|DK|FI|AT|CH|BE)(?:\s|$)`,
		),
		database.GetCachedRegexp(`(?:^|\s)(19\d{2}|20\d{2})(?:\s|$)`),
	}

	for range 3 {
		changed := false
		for _, p := range patterns {
			if p.MatchString(album) {
				album = p.ReplaceAllString(album, " ")
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Clean up multiple spaces
	for strings.Contains(album, "  ") {
		album = strings.ReplaceAll(album, "  ", " ")
	}

	return strings.TrimSpace(album)
}

// checkCompleteness determines if an album has all expected tracks.
func (mp *MusicParser) checkCompleteness(result *MusicParseResult) {
	if len(result.Tracks) == 0 {
		return
	}

	// Group tracks by disc
	discTracks := make(map[int][]int)
	for i := range result.Tracks {
		disc := result.Tracks[i].DiscNumber
		if disc == 0 {
			disc = 1
		}

		discTracks[disc] = append(discTracks[disc], result.Tracks[i].TrackNumber)
	}

	// Check each disc for missing tracks
	result.MissingTracks = nil
	for disc, tracks := range discTracks {
		if len(tracks) == 0 {
			continue
		}

		// Find max track number
		maxTrack := 0

		trackSet := make(map[int]bool)
		for _, t := range tracks {
			if t > maxTrack {
				maxTrack = t
			}

			trackSet[t] = true
		}

		// Check for gaps (only if reasonable track count)
		if maxTrack > 50 {
			continue
		}

		for i := 1; i <= maxTrack; i++ {
			if !trackSet[i] {
				// Encode as disc*100 + track for multi-disc support
				if result.TotalDiscs > 1 {
					result.MissingTracks = append(result.MissingTracks, disc*100+i)
				} else {
					result.MissingTracks = append(result.MissingTracks, i)
				}
			}
		}
	}

	result.IsComplete = len(result.MissingTracks) == 0
}

// calculateTrackConfidence calculates confidence for a single track parse.
func (mp *MusicParser) calculateTrackConfidence(track TrackInfo) float64 {
	var conf float64

	if track.Title != "" {
		conf += 0.3
	}

	if track.TrackNumber > 0 {
		conf += 0.3
	}

	if track.Artist != "" {
		conf += 0.2
	}

	if track.ISRC != "" {
		conf += 0.1
	}

	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// calculateAlbumConfidence calculates confidence for an album parse.
func (mp *MusicParser) calculateAlbumConfidence(result *MusicParseResult) float64 {
	var conf float64

	// Album title
	if result.Album != "" {
		conf += 0.2
	}

	// Artist
	if result.Artist != "" {
		conf += 0.2
	}

	// Year
	if result.Year > 0 {
		conf += 0.1
	}

	// Track information
	if len(result.Tracks) > 0 {
		conf += 0.15

		// Tracks have numbers
		hasNumbers := false
		for _, t := range result.Tracks {
			if t.TrackNumber > 0 {
				hasNumbers = true
				break
			}
		}

		if hasNumbers {
			conf += 0.1
		}
	}

	// Completeness
	if result.IsComplete && len(result.Tracks) > 0 {
		conf += 0.1
	}

	// External identifiers
	if result.MusicBrainzReleaseID != "" {
		conf += 0.1
	}

	if result.CatalogNumber != "" {
		conf += 0.05
	}

	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// MatchByRuntime attempts to match an album to a database entry by total runtime.
func (mp *MusicParser) MatchByRuntime(expectedRuntimeMS int64, tracks []TrackInfo) (bool, float64) {
	return mp.runtimeMatcher.MatchTotalRuntime(expectedRuntimeMS, tracks)
}

// UpdateTrackRuntimes updates the runtime information for tracks.
func (mp *MusicParser) UpdateTrackRuntimes(result *MusicParseResult, runtimes map[string]int64) {
	var totalRuntime int64
	for i := range result.Tracks {
		if runtime, ok := runtimes[result.Tracks[i].Filename]; ok {
			result.Tracks[i].RuntimeMS = runtime

			totalRuntime += runtime
		}
	}

	result.TotalRuntimeMS = totalRuntime
}

// ParseAlbumWithTags parses an album using both filename parsing and audio tags.
// Tags take precedence over filename-derived information.
func (mp *MusicParser) ParseAlbumWithTags(dirPath string, files []string) *MusicParseResult {
	// First parse from filenames
	result := mp.ParseAlbum(dirPath, files)
	if result == nil {
		return nil
	}

	// Read tags from files
	tagResult, err := ReadAlbumTags(files)
	if err != nil || tagResult == nil {
		return result
	}

	// Merge tag info, preferring tag data over filename data
	mp.mergeTagInfo(result, tagResult)

	return result
}

// ParseAlbumWithAnalysis parses an album with full media analysis.
func (mp *MusicParser) ParseAlbumWithAnalysis(
	dirPath string,
	files []string,
	analyzer *MediaAnalyzer,
) *MusicParseResult {
	// Parse with tags first
	result := mp.ParseAlbumWithTags(dirPath, files)
	if result == nil {
		return nil
	}

	// Analyze files for runtime info
	if analyzer != nil {
		_ = analyzer.AnalyzeMusic(files, result)
	}

	return result
}

// mergeTagInfo merges tag information into the parse result.
func (mp *MusicParser) mergeTagInfo(result, tagResult *MusicParseResult) {
	// Album-level info from tags takes precedence
	if tagResult.Album != "" {
		result.Album = tagResult.Album
		result.Title = tagResult.Album
	}

	if tagResult.Artist != "" {
		result.Artist = tagResult.Artist
	}

	if tagResult.AlbumArtist != "" {
		result.AlbumArtist = tagResult.AlbumArtist
	}

	if tagResult.Year > 0 {
		result.Year = tagResult.Year
	}

	if tagResult.Genre != "" {
		result.Genre = tagResult.Genre
	}

	if tagResult.Label != "" {
		result.Label = tagResult.Label
	}

	if tagResult.CatalogNumber != "" {
		result.CatalogNumber = tagResult.CatalogNumber
	}

	if tagResult.MusicBrainzReleaseID != "" {
		result.MusicBrainzReleaseID = tagResult.MusicBrainzReleaseID
	}

	if tagResult.MusicBrainzReleaseGroupID != "" {
		result.MusicBrainzReleaseGroupID = tagResult.MusicBrainzReleaseGroupID
	}

	if tagResult.TotalTracks > 0 {
		result.TotalTracks = tagResult.TotalTracks
	}

	if tagResult.TotalDiscs > 0 {
		result.TotalDiscs = tagResult.TotalDiscs
	}

	// Merge track info
	tagTrackMap := make(map[string]*TrackInfo)
	for i := range tagResult.Tracks {
		tagTrackMap[tagResult.Tracks[i].Filename] = &tagResult.Tracks[i]
	}

	for i := range result.Tracks {
		tagTrack, ok := tagTrackMap[result.Tracks[i].Filename]
		if !ok {
			continue
		}

		// Merge track-level info
		if tagTrack.Title != "" {
			result.Tracks[i].Title = tagTrack.Title
		}

		if tagTrack.TrackNumber > 0 {
			result.Tracks[i].TrackNumber = tagTrack.TrackNumber
		}

		if tagTrack.DiscNumber > 0 {
			result.Tracks[i].DiscNumber = tagTrack.DiscNumber
		}

		if tagTrack.Artist != "" {
			result.Tracks[i].Artist = tagTrack.Artist
		}

		if tagTrack.ISRC != "" {
			result.Tracks[i].ISRC = tagTrack.ISRC
		}

		if tagTrack.AcoustID != "" {
			result.Tracks[i].AcoustID = tagTrack.AcoustID
		}

		if tagTrack.MusicBrainzRecordingID != "" {
			result.Tracks[i].MusicBrainzRecordingID = tagTrack.MusicBrainzRecordingID
		}

		if tagTrack.RuntimeMS > 0 {
			result.Tracks[i].RuntimeMS = tagTrack.RuntimeMS
		}
	}

	// Recalculate totals from merged data
	mp.checkCompleteness(result)

	// Update confidence based on merged data
	result.Confidence = mp.calculateAlbumConfidence(result)

	if tagResult.MusicBrainzReleaseID == "" {
		return
	}

	result.Confidence += 0.1
	if result.Confidence > 1.0 {
		result.Confidence = 1.0
	}
}

// normalizeReleaseType normalizes release type strings.
func normalizeReleaseType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "ost":
		return "soundtrack"
	case "remastered":
		return "remaster"
	case "limited edition":
		return "limited"
	default:
		return s
	}
}

// looksLikeArtistName checks if a string looks like an artist name.
func looksLikeArtistName(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 {
		return false
	}

	// Exclude common non-artist directory names
	excludes := []string{
		"music", "albums", "flac", "mp3", "lossless", "cd", "disc",
		"downloads", "torrents", "complete", "collection",
	}
	if slices.ContainsFunc(excludes, func(e string) bool { return strings.EqualFold(s, e) }) {
		return false
	}

	// Should have at least one letter
	hasLetter := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			hasLetter = true
			break
		}
	}

	return hasLetter
}

// splitArtists splits an artist string by common separators.
func splitArtists(s string) []string {
	// Replace common separators
	s = strings.ReplaceAll(s, " & ", ", ")
	s = strings.ReplaceAll(s, " x ", ", ")
	s = strings.ReplaceAll(s, " X ", ", ")
	s = strings.ReplaceAll(s, " vs ", ", ")
	s = strings.ReplaceAll(s, " vs. ", ", ")
	s = strings.ReplaceAll(s, " and ", ", ")

	parts := strings.Split(s, ",")

	var artists []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			artists = append(artists, p)
		}
	}

	return artists
}
