package parser_v2

import (
	"regexp"
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
	// A left boundary keeps the markers from matching inside words ("Cloud 9",
	// "Road 66" read as disc 9/66); the bare "d"
	// form must additionally be a standalone "d1"/"d01" token. Group 1 holds
	// the disc/disk/cd number, group 2 the bare "d" number.
	reMusicDiscNumber  = `(?i)(?:^|[^a-z0-9])[\[\(]?(?:(?:disc|disk|cd)[\s._-]?(\d+)|d(\d{1,2}))[\]\)]?(?:$|[^a-z0-9])`
	reMusicDiscTrack   = `^(\d{1,2})[-.](\d{2,3})\s`
	reMusicCatalogNum  = `(?i)[\[\(]([A-Z]{2,}[\s-]?\d+(?:[A-Z])?|[A-Z]+\d{2,}[A-Z]*)[\]\)]`
	reMusicFormat      = `(?i)(?:[\[\(\s-])(flac|mp3|m4a|ogg|opus|wav|alac|aac|ape|wma|aiff|wv|320|256|192|128|v0|v2|24bit|16bit|24-bit|16-bit)(?:[\]\)\s-]|$)`
	reMusicBitrate     = `(?i)(\d{2,4})\s*(?:kbps|kb\/s|kbit)`
	reMusicSampleRate  = `(?i)(\d{2,3}(?:\.\d)?)\s*(?:khz)`
	reMusicBitDepth    = `(?i)(\d{2})[\s-]?bit`
	reMusicReleaseType = `(?i)[\[\(\s](album|ep|single|compilation|soundtrack|ost|live|bootleg|demo|mixtape|remix|deluxe|remaster(?:ed)?|limited\s*edition)(?:[\]\)\s]|$)`
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

var validWords = map[string]bool{
	"I": true, "II": true, "III": true, "IV": true, "V": true,
	"VI": true, "VII": true, "VIII": true, "IX": true, "X": true,
	"EP": true, "LP": true, "DJ": true, "MC": true, "MR": true,
	"VS": true, "USA": true, "NYC": true, "LA": true, "UK": true,
	"OK": true, "TV": true, "CD": true, "DNA": true, "UFO": true,
	"AC": true, "DC": true,
}

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
	name, ext := splitKnownExt(filename)

	result.Format = logger.ExtToFormat(ext)

	// Check if lossless
	result.IsLossless = IsLosslessAudioExtension(ext)

	// Clean underscores
	name = strings.ReplaceAll(name, "_", " ")

	cleanedName := name

	// Extract disc-track combined pattern first (e.g., "1-01 Track.mp3")
	var trackInfo TrackInfo
	if loc := mp.patterns.discTrack.FindStringSubmatchIndex(name); len(loc) > 4 {
		trackInfo.DiscNumber = parseInt(name[loc[2]:loc[3]])
		trackInfo.TrackNumber = parseInt(name[loc[4]:loc[5]])
		cleanedName = mp.patterns.discTrack.ReplaceAllLiteralString(cleanedName, "")
	}

	// Extract track number if not already found
	if trackInfo.TrackNumber == 0 {
		if loc := mp.patterns.trackNumber.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
			trackInfo.TrackNumber = parseInt(cleanedName[loc[2]:loc[3]])
			cleanedName = mp.patterns.trackNumber.ReplaceAllLiteralString(cleanedName, "")
		}
	}

	// Extract disc number if not already found
	if trackInfo.DiscNumber == 0 {
		if loc := mp.patterns.discNumber.FindStringSubmatchIndex(cleanedName); len(loc) > 5 {
			if loc[2] != -1 {
				trackInfo.DiscNumber = parseInt(cleanedName[loc[2]:loc[3]])
			} else {
				trackInfo.DiscNumber = parseInt(cleanedName[loc[4]:loc[5]])
			}

			cleanedName = mp.patterns.discNumber.ReplaceAllLiteralString(cleanedName, " ")
		}
	}

	// Check for vinyl side notation
	if loc := mp.patterns.vinyl.FindStringSubmatchIndex(name); len(loc) > 4 {
		// Convert vinyl side to disc (A=1, B=2, etc.) — loc[2] is the byte offset of the side letter.
		trackInfo.DiscNumber = int(name[loc[2]] - 'A' + 1)
		trackInfo.TrackNumber = parseInt(name[loc[4]:loc[5]])
	}

	// Extract ISRC
	if loc := mp.patterns.isrc.FindStringSubmatchIndex(name); len(loc) > 2 {
		trackInfo.ISRC = name[loc[2]:loc[3]]
	}

	// Extract featured artist
	if loc := mp.patterns.feat.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		trackInfo.Artist = strings.TrimSpace(cleanedName[loc[2]:loc[3]])
		cleanedName = mp.patterns.feat.ReplaceAllLiteralString(cleanedName, " ")
	}

	// The remaining name is the track title
	trackInfo.Title = cleanTitle(cleanedName)
	trackInfo.Filename = filename

	result.Tracks = []TrackInfo{trackInfo}

	// Calculate basic confidence
	result.Confidence = mp.calculateTrackConfidence(trackInfo)

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
	if loc := mp.patterns.year.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.Year = parseInt(cleanedName[loc[2]:loc[3]])

		fullMatch := cleanedName[loc[0]:loc[1]]
		if strings.Contains(fullMatch, "(") || strings.Contains(fullMatch, "[") {
			cleanedName = strings.Replace(cleanedName, fullMatch, "", 1)
		}
	}

	// Extract release type - only extract for metadata, don't strip from title
	// if it's in parentheses (e.g., "(Live)" is often part of the album name)
	// Release-type words are ordinary English ("Live and Let Die", "Album"),
	// so a bare one only counts as a tag when it ends the name.
	for _, loc := range mp.patterns.releaseType.FindAllStringSubmatchIndex(cleanedName, -1) {
		matchStr := cleanedName[loc[0]:loc[1]]
		bracketed := matchStr[0] == '(' || matchStr[0] == '['
		if !bracketed && strings.TrimSpace(cleanedName[loc[3]:]) != "" {
			continue
		}

		result.ReleaseType = normalizeReleaseType(cleanedName[loc[2]:loc[3]])
		// Only strip release type if it's NOT in parentheses/brackets
		if !bracketed {
			cleanedName = strings.Replace(cleanedName, matchStr, " ", 1)
		}

		break
	}

	// Extract catalog number
	if loc := mp.patterns.catalogNum.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.CatalogNumber = cleanedName[loc[2]:loc[3]]
		cleanedName = mp.patterns.catalogNum.ReplaceAllLiteralString(cleanedName, " ")
	}

	// Extract format indicators
	if loc := mp.findFormatTag(cleanedName); len(loc) > 2 {
		formatIndicator := strings.ToLower(cleanedName[loc[2]:loc[3]])
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

		for loc != nil {
			cleanedName = cleanedName[:loc[0]] + " " + cleanedName[loc[1]:]
			loc = mp.findFormatTag(cleanedName)
		}
	}

	// Extract bitrate
	if loc := mp.patterns.bitrate.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.Bitrate = parseInt(cleanedName[loc[2]:loc[3]])
		cleanedName = mp.patterns.bitrate.ReplaceAllLiteralString(cleanedName, " ")
	}

	// Extract sample rate
	if loc := mp.patterns.sampleRate.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		rate := parseFloat(cleanedName[loc[2]:loc[3]])
		if rate < 1000 {
			result.SampleRate = int(rate * 1000)
		} else {
			result.SampleRate = int(rate)
		}
	}

	// Extract bit depth
	if loc := mp.patterns.bitDepth.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.BitDepth = parseInt(cleanedName[loc[2]:loc[3]])
	}

	// Extract UPC
	if loc := mp.patterns.upc.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.UPC = cleanedName[loc[2]:loc[3]]
		cleanedName = strings.Replace(cleanedName, cleanedName[loc[0]:loc[1]], "", 1)
	}

	// Extract MusicBrainz ID
	if loc := mp.patterns.mbid.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.MusicBrainzReleaseID = cleanedName[loc[2]:loc[3]]
		cleanedName = strings.Replace(cleanedName, cleanedName[loc[0]:loc[1]], "", 1)
	}

	// Extract release group
	if loc := mp.patterns.group.FindStringSubmatchIndex(cleanedName); len(loc) > 2 {
		result.ReleaseGroup = cleanedName[loc[2]:loc[3]]
		cleanedName = mp.patterns.group.ReplaceAllLiteralString(cleanedName, "")
	}

	// Strip scene release tags (formats, sources, quality indicators)
	// This handles names like "Artist-Album-OST-CD-FLAC-1986-KINDA"
	// Loop to handle overlapping matches (pattern includes trailing dash)
	for {
		newName := mp.patterns.sceneTags.ReplaceAllLiteralString(cleanedName, "-")
		if newName == cleanedName {
			break
		}

		cleanedName = newName
	}

	// Strip release group at end
	if mp.patterns.sceneGroup.MatchString(cleanedName) {
		cleanedName = mp.patterns.sceneGroup.ReplaceAllLiteralString(cleanedName, "")
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

// findFormatTag returns the submatch indices of the first format indicator in
// name that is really a tag. "opus" and "ape" are also ordinary title words
// ("Magnum Opus"), so outside brackets they only count when written in
// uppercase the way scene/NZB names spell tags.
func (mp *MusicParser) findFormatTag(name string) []int {
	for _, loc := range mp.patterns.format.FindAllStringSubmatchIndex(name, -1) {
		word := name[loc[2]:loc[3]]
		if name[loc[0]] == '(' || name[loc[0]] == '[' {
			return loc
		}

		if (strings.EqualFold(word, "opus") || strings.EqualFold(word, "ape")) &&
			word != strings.ToUpper(word) {
			continue
		}

		return loc
	}

	return nil
}

// extractArtistAlbum attempts to separate artist from album title.
func (mp *MusicParser) extractArtistAlbum(name string, result *MusicParseResult) {
	name = strings.TrimSpace(name)

	// Try "Artist - Album" pattern (with spaces around dash)
	if loc := mp.patterns.artistDash.FindStringSubmatchIndex(name); len(loc) > 4 {
		g1, g2 := name[loc[2]:loc[3]], name[loc[4]:loc[5]]
		// Skip this split if the "album" side starts with a bare year like "2007 - FLAC - GROUP".
		// That pattern means the whole string is "Title - Year - Format", not "Artist - Album".
		rhsTrimmed := strings.TrimSpace(g2)

		yearStart := len(rhsTrimmed) >= 4 &&
			(rhsTrimmed[0] == '1' || rhsTrimmed[0] == '2') &&
			rhsTrimmed[1] >= '0' && rhsTrimmed[1] <= '9' &&
			rhsTrimmed[2] >= '0' && rhsTrimmed[2] <= '9' &&
			rhsTrimmed[3] >= '0' && rhsTrimmed[3] <= '9' &&
			(len(rhsTrimmed) == 4 || rhsTrimmed[4] == ' ' || rhsTrimmed[4] == '-')

		// Skip this split if the LHS is a 4-digit year that was already extracted
		// (e.g. "1976 - I'd Rather Believe in You" → year=1976, not artist).
		lhsTrimmed := strings.TrimSpace(g1)
		lhsIsYear := result.Year > 0 &&
			len(lhsTrimmed) == 4 &&
			parseInt(lhsTrimmed) == result.Year

		// Skip if LHS is a bare integer (not a year) — it's a library index or
		// catalog prefix like "19 - Stef Bos - Album". Re-parse the RHS instead.
		lhsIsNumericPrefix := !lhsIsYear && isNumericOnly(lhsTrimmed)

		if !yearStart && !lhsIsYear && !lhsIsNumericPrefix {
			mp.setArtistAlbumFromMatches(g1, g2, result)
			return
		}

		if !yearStart && lhsIsNumericPrefix {
			mp.extractArtistAlbum(g2, result)
			return
		}
	}

	// Try "Artist-Album" or "Artist_Album" pattern (scene releases without spaces)
	// Only use this if we have at least one dash/underscore
	if strings.ContainsAny(name, "-_") {
		if loc := mp.patterns.artistTitle.FindStringSubmatchIndex(name); len(loc) > 4 {
			potentialArtist := strings.TrimSpace(name[loc[2]:loc[3]])
			potentialAlbum := strings.TrimSpace(name[loc[4]:loc[5]])

			if len(potentialArtist) >= 2 && len(potentialAlbum) >= 2 {
				// normalizeArtistName (inside setArtistAlbumFromMatches) handles ./_→space for artist.
				// Album needs '-'→' ' too since cleanAlbumTitle doesn't do it; use normalizeScenePart.
				mp.setArtistAlbumFromMatches(
					potentialArtist,
					normalizeScenePart(potentialAlbum, true),
					result,
				)

				return
			}
		}
	}

	// No split found, treat as album only
	result.Album = cleanTitle(name)
	result.Title = result.Album
}

// setArtistAlbumFromMatches sets artist and album from matched strings.
func (*MusicParser) setArtistAlbumFromMatches(artist, album string, result *MusicParseResult) {
	result.Artist = normalizeArtistName(artist)
	result.Album = cleanAlbumTitle(strings.TrimSpace(album))
	result.Title = result.Album

	// Check for multiple artists — ContainsI covers "and"/"And"/"AND" in one call.
	if !strings.Contains(result.Artist, " & ") &&
		!strings.Contains(result.Artist, " x ") &&
		!logger.ContainsI(result.Artist, " and ") {
		return
	}

	artists := splitArtists(result.Artist)

	result.Artists = artists
	if len(artists) > 0 {
		result.AlbumArtist = result.Artist
		result.Artist = artists[0]
	}
}

// normalizeScenePart replaces scene separators and collapses spaces in a
// single pooled-buffer pass. Set replaceDash=true to also convert '-' → ' '
// (needed for album parts; artists may legitimately contain dashes like "AC-DC").
func normalizeScenePart(s string, replaceDash bool) string {
	needsClean := false

	prevWasSpace := false
	for i := range len(s) {
		c := s[i]
		if c == '.' || c == '_' || (replaceDash && c == '-') || (c == ' ' && prevWasSpace) {
			needsClean = true
			break
		}

		prevWasSpace = c == ' '
	}

	if !needsClean {
		return strings.TrimSpace(s)
	}

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	lastWasSpace := false
	for i := range len(s) {
		c := s[i]
		if c == '.' || c == '_' || (replaceDash && c == '-') {
			c = ' '
		}

		if c == ' ' {
			if !lastWasSpace {
				buf.WriteByte(' ')
			}

			lastWasSpace = true
		} else {
			buf.WriteByte(c)

			lastWasSpace = false
		}
	}

	return strings.TrimSpace(buf.String())
}

// normalizeArtistName cleans scene separators from an artist name.
func normalizeArtistName(s string) string { return normalizeScenePart(s, false) }

// cleanAlbumTitle removes format indicators and scene tags from album titles.
func cleanAlbumTitle(album string) string {
	// Replace dots and underscores with spaces (scene format cleanup)
	album = strings.ReplaceAll(album, ".", " ")
	album = strings.ReplaceAll(album, "_", " ")

	// Remove trailing year + release group pattern (e.g., "2003 CGPABN INT", "1990 EMG INT")
	// This must be done first before other patterns
	if loc := database.GetCachedRegexp(`\s+\d{4}\s+[A-Z0-9]+(?:\s+INT)?$`).
		FindStringIndex(album); loc != nil {
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
				album = p.ReplaceAllLiteralString(album, " ")
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	// Normalize "Vol N" volume indicators: "Vol.58" or "Vol 58" → "58"
	// Scene releases often use "Artist Vol.NN" while the DB title uses "Artist NN"
	if loc := database.GetCachedRegexp(`(?i)\bVol\.?\s+(\d+)\b`).
		FindStringSubmatchIndex(album); loc != nil {
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
			// Single pass: check for at least one letter and no lowercase letters.
			// Pure numbers like "58" are volume/track numbers, not scene groups.
			hasLetter, hasLower := false, false
			for i := range len(lastWord) {
				c := lastWord[i]
				if c >= 'A' && c <= 'Z' {
					hasLetter = true
				} else if c >= 'a' && c <= 'z' {
					hasLetter = true
					hasLower = true
					break
				}
			}

			if hasLetter && !hasLower && !looksLikeAlbumWord(lastWord) {
				album = trailingGroup.ReplaceAllLiteralString(album, "")
			}
		}
	}

	return strings.TrimSpace(album)
}

// looksLikeAlbumWord returns true if the word could be part of an album title.
func looksLikeAlbumWord(word string) bool {
	// Common uppercase words that are valid album title parts
	return validWords[strings.ToUpper(word)]
}

// StripReleaseType removes release type indicators (Deluxe Edition, Remastered, etc.) from album titles.
// This is used for fallback matching when exact title match fails.
func StripReleaseType(album *string) string {
	// Remove catalog numbers in parentheses like (824 150-2 M-1) or (B60F350E)
	if re := database.GetCachedRegexp(`\s*\([A-Z0-9][A-Z0-9\s\-]*\)`); re.MatchString(*album) {
		*album = re.ReplaceAllLiteralString(*album, "")
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
			if p.MatchString(*album) {
				*album = p.ReplaceAllLiteralString(*album, " ")
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	// Clean up multiple spaces
	for strings.Contains(*album, "  ") {
		*album = strings.ReplaceAll(*album, "  ", " ")
	}

	return strings.TrimSpace(*album)
}

// calculateTrackConfidence calculates confidence for a single track parse.
func (*MusicParser) calculateTrackConfidence(track TrackInfo) float64 {
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

// MatchByRuntime attempts to match an album to a database entry by total runtime.
func (mp *MusicParser) MatchByRuntime(expectedRuntimeMS int64, tracks []TrackInfo) (bool, float64) {
	return mp.runtimeMatcher.MatchTotalRuntime(expectedRuntimeMS, tracks)
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
	for i := range parts {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			artists = append(artists, p)
		}
	}

	return artists
}
