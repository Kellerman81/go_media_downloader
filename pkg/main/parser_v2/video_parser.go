package parser_v2

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// VideoParser handles parsing of video filenames for movies and TV series.
// It maintains compatibility with the original parser package functionality.
type VideoParser struct {
	patterns     *videoPatterns
	patternStore *DBPatternStore // Database patterns (optional)
	isMovie      bool
	strictMode   bool
	useDBPattern bool // Whether to use database patterns
}

// videoPatterns contains compiled regex patterns for video parsing.
type videoPatterns struct {
	year              *regexp.Regexp
	imdb              *regexp.Regexp
	tvdb              *regexp.Regexp
	seasonEpisode     *regexp.Regexp
	seasonEpisodeAlt  *regexp.Regexp
	seasonEpisodeDate *regexp.Regexp
	episodeOnly       *regexp.Regexp
	resolution        *regexp.Regexp
	quality           *regexp.Regexp
	codec             *regexp.Regexp
	audio             *regexp.Regexp
	group             *regexp.Regexp
	extended          *regexp.Regexp
	proper            *regexp.Regexp
	repack            *regexp.Regexp
	remux             *regexp.Regexp
	hdr               *regexp.Regexp
	complete          *regexp.Regexp
	multiSeason       *regexp.Regexp
	language          *regexp.Regexp
}

// NewVideoParser creates a new VideoParser.
func NewVideoParser() *VideoParser {
	return &VideoParser{
		patterns:     compileVideoPatterns(),
		strictMode:   false,
		useDBPattern: true,
	}
}

// NewVideoParserWithPatternStore creates a new VideoParser that uses database patterns.
func NewVideoParserWithPatternStore(ps *DBPatternStore) *VideoParser {
	return &VideoParser{
		patterns:     compileVideoPatterns(),
		patternStore: ps,
		useDBPattern: true,
	}
}

// SetPatternStore sets the database pattern store for this parser.
func (vp *VideoParser) SetPatternStore(ps *DBPatternStore) {
	vp.patternStore = ps
	vp.useDBPattern = ps != nil && ps.IsLoaded()
}

// SetMovieMode sets the parser to expect movie files.
func (vp *VideoParser) SetMovieMode() {
	vp.isMovie = true
}

// SetSeriesMode sets the parser to expect TV series files.
func (vp *VideoParser) SetSeriesMode() {
	vp.isMovie = false
}

// SetStrictMode enables stricter pattern matching.
func (vp *VideoParser) SetStrictMode(strict bool) {
	vp.strictMode = strict
}

// Video pattern strings as constants so they can be used as cache keys.
const (
	reVideoYear              = `(?:[\(\[]|\s|\.|_)((?:19|20)\d{2})(?:[\)\]]|\s|\.|_|$)`
	reVideoIMDB              = `(?i)(?:imdb[:\s-]?)?tt\d{7,8}`
	reVideoTVDB              = `(?i)tvdb[\s-]?(\d+)`
	reVideoSeasonEpisode     = `(?i)(?:s|season\s?)(\d{1,2})[\s._-]*(?:e|episode\s?|x)(\d{1,3})(?:-?(?:e|x)?(\d{1,3}))?|(\d{1,2})x(\d{2,3})`
	reVideoSeasonEpisodeAlt  = `(?i)season\s*(\d{1,2})\s*episode\s*(\d{1,3})`
	reVideoSeasonEpisodeDate = `(\d{2,4})[\s._-](\d{2})[\s._-](\d{2})`
	reVideoEpisodeOnly       = `(?i)(?:e|ep|episode\s?)(\d{1,3})`
	reVideoResolution        = `(?i)(?:(\d{3,4})(?:p|i)|4k|uhd|hd|sd)`
	reVideoQuality           = `(?i)(blu[\s-]?ray|bdrip|brrip|web[\s-]?dl|web[\s-]?rip|webrip|web|hdtv|dvd[\s-]?rip|dvd[\s-]?scr|hdcam|hdrip|hd[\s-]?ts|tele[\s-]?sync|ts|cam|r5|ppv[\s-]?rip|pdtv|dsr|sat[\s-]?rip|vod[\s-]?rip|amazon|amzn|nf|netflix|dsnp|disney\+?|hmax|hulu|atvp|atv\+?|pcok|peacock|hbo[\s-]?max|itunes)`
	reVideoCodec             = `(?i)(x264|x\.264|h\.?264|avc|x265|x\.265|h\.?265|hevc|xvid|divx|av1|mpeg[\s-]?2|vc[\s-]?1)`
	reVideoAudio             = `(?i)(dts[\s-]?hd[\s-]?ma|dts[\s-]?hd|dts[\s-]?x|dts|dolby[\s-]?atmos|atmos|truehd|ddp?\+?|dd[\s-]?5\.1|dd|ac[\s-]?3|eac[\s-]?3|aac[\s-]?2\.0|aac|flac|mp3|lpcm|pcm|opus|vorbis)`
	reVideoGroup             = `(?:-|\[)([a-zA-Z0-9]+)(?:\])?$`
	reVideoExtended          = `(?i)(?:[\[\(\s]|\.)(extended|uncut|unrated|directors?[\s._-]?cut|theatrical)(?:[\]\)\s]|\.)`
	reVideoProper            = `(?i)(?:[\[\(\s]|\.)(proper|real)(?:[\]\)\s]|\.)`
	reVideoRepack            = `(?i)(?:[\[\(\s]|\.)(repack|rerip)(?:[\]\)\s]|\.)`
	reVideoRemux             = `(?i)(remux)`
	reVideoHDR               = `(?i)(hdr10\+?|dolby[\s-]?vision|(?:^|[\s._-])dv(?:$|[\s._-])|hlg|hdr)`
	reVideoComplete          = `(?i)(?:[\[\(\s]|\.)(complete|full[\s._-]?series)(?:[\]\)\s]|\.)`
	reVideoMultiSeason       = `(?i)s(\d{1,2})[\s._-]*[-–][\s._-]*s(\d{1,2})`
	reVideoLanguage          = `(?i)[\s._](german|deutsch|french|francais|spanish|espanol|italiano|portuguese|portuguese|russian|japanese|korean|chinese|mandarin|hindi|arabic|dutch|polish|swedish|norwegian|danish|finnish|turkish|greek|hebrew|czech|hungarian|romanian|thai|vietnamese|indonesian|malay|tagalog|multi|dual[\s._-]?audio|dubbed|subbed|subs?)[\s._](?:(?:19|20)\d{2}[\s._]|$)`
)

var (
	sharedVideoPatterns    *videoPatterns
	sharedVideoPatternOnce sync.Once
)

// compileVideoPatterns returns the shared video pattern set.
// The struct is built once and reused; the individual regexps inside are
// already cached by the database package, so this eliminates the per-call
// struct allocation that showed up in heap profiles.
func compileVideoPatterns() *videoPatterns {
	sharedVideoPatternOnce.Do(func() {
		sharedVideoPatterns = &videoPatterns{
			year:              database.GetCachedRegexp(reVideoYear),
			imdb:              database.GetCachedRegexp(reVideoIMDB),
			tvdb:              database.GetCachedRegexp(reVideoTVDB),
			seasonEpisode:     database.GetCachedRegexp(reVideoSeasonEpisode),
			seasonEpisodeAlt:  database.GetCachedRegexp(reVideoSeasonEpisodeAlt),
			seasonEpisodeDate: database.GetCachedRegexp(reVideoSeasonEpisodeDate),
			episodeOnly:       database.GetCachedRegexp(reVideoEpisodeOnly),
			resolution:        database.GetCachedRegexp(reVideoResolution),
			quality:           database.GetCachedRegexp(reVideoQuality),
			codec:             database.GetCachedRegexp(reVideoCodec),
			audio:             database.GetCachedRegexp(reVideoAudio),
			group:             database.GetCachedRegexp(reVideoGroup),
			extended:          database.GetCachedRegexp(reVideoExtended),
			proper:            database.GetCachedRegexp(reVideoProper),
			repack:            database.GetCachedRegexp(reVideoRepack),
			remux:             database.GetCachedRegexp(reVideoRemux),
			hdr:               database.GetCachedRegexp(reVideoHDR),
			complete:          database.GetCachedRegexp(reVideoComplete),
			multiSeason:       database.GetCachedRegexp(reVideoMultiSeason),
			language:          database.GetCachedRegexp(reVideoLanguage),
		}
	})

	return sharedVideoPatterns
}

// Parse parses a video filename and returns extracted information.
func (vp *VideoParser) Parse(filename string) *VideoParseResult {
	result := &VideoParseResult{
		ParseResult: ParseResult{
			SourceFile: filename,
		},
	}

	// Get extension and base name - only strip if it's a valid video extension
	ext := filepath.Ext(filename)

	name := filename
	if IsVideoExtension(ext) {
		name = strings.TrimSuffix(filename, ext)
	}

	// Replace common separators with spaces for parsing
	cleanName := strings.ReplaceAll(name, ".", " ")

	cleanName = strings.ReplaceAll(cleanName, "_", " ")

	// Preserve original for pattern matching
	originalName := name

	// Extract IMDB ID first
	if loc := vp.patterns.imdb.FindStringSubmatchIndex(originalName); len(loc) > 0 {
		result.Imdb = normalizeIMDB(originalName[loc[0]:loc[1]])
	}

	// Extract TVDB ID
	if loc := vp.patterns.tvdb.FindStringSubmatchIndex(originalName); len(loc) > 2 {
		result.Tvdb = originalName[loc[2]:loc[3]]
	}

	// Detect media type and extract season/episode info
	result.MediaType = vp.detectMediaType(originalName, result)

	// Extract year
	if loc := vp.patterns.year.FindStringSubmatchIndex(originalName); len(loc) > 2 {
		result.Year = parseInt(originalName[loc[2]:loc[3]])
	}

	// Extract quality attributes
	vp.extractQualityInfo(originalName, result)

	// Extract release group
	if loc := vp.patterns.group.FindStringSubmatchIndex(originalName); len(loc) > 2 {
		result.ReleaseGroup = originalName[loc[2]:loc[3]]
	}

	// Extract title
	result.Title = vp.extractTitle(cleanName, result)

	// Set quality IDs if database available
	result.ResolutionID = Gettypeids(result.Resolution, database.DBConnect.GetresolutionsIn)
	result.QualityID = Gettypeids(result.Quality, database.DBConnect.GetqualitiesIn)
	result.CodecID = Gettypeids(result.Codec, database.DBConnect.GetcodecsIn)
	result.AudioID = Gettypeids(result.Audio, database.DBConnect.GetaudiosIn)

	// Calculate confidence
	result.Confidence = vp.calculateConfidence(result)

	return result
}

// ParseWithPath parses a video file with its full path for additional context.
func (vp *VideoParser) ParseWithPath(fullpath string) *VideoParseResult {
	filename := filepath.Base(fullpath)
	result := vp.Parse(filename)

	result.SourcePath = fullpath

	// Try to extract additional info from parent directory
	dir := filepath.Dir(fullpath)
	parentDir := filepath.Base(dir)

	// If we didn't find a year, check parent directory
	if result.Year == 0 {
		if loc := vp.patterns.year.FindStringSubmatchIndex(parentDir); len(loc) > 2 {
			result.Year = parseInt(parentDir[loc[2]:loc[3]])
		}
	}

	// For series, parent directory might be series name
	if result.MediaType == MediaTypeSeries {
		if result.Title == "" || len(result.Title) < len(parentDir) {
			// Clean parent directory name
			dirName := strings.ReplaceAll(parentDir, ".", " ")

			dirName = strings.ReplaceAll(dirName, "_", " ")

			// Remove year if present
			if loc := vp.patterns.year.FindStringSubmatchIndex(dirName); len(loc) > 0 {
				dirName = strings.Replace(dirName, dirName[loc[0]:loc[1]], "", 1)
			}

			cleaned := cleanTitle(dirName)
			if len(cleaned) > len(result.Title) {
				result.Title = cleaned
			}
		}
	}

	return result
}

// defaultVideoParser is a shared VideoParser instance for package-level functions.
// var defaultVideoParser = NewVideoParser()

// ParseFile parses a video file and returns parsed information.
// It retrieves a ParseInfo object from the pool, populates it with parsed data,
// and returns the populated ParseInfo.
//
// Parameters:
//   - videofile: the path to the video file to be parsed
//   - usepath: whether to use the file path in parsing
//   - usefolder: whether to use the folder name in parsing
//   - cfgp: pointer to MediaTypeConfig containing parsing configuration
//   - listid: the ID of the list associated with this parse operation
//
// Returns:
//   - a pointer to database.ParseInfo containing the parsed video file information
func ParseFile(
	videofile string,
	usepath, usefolder bool,
	cfgp *config.MediaTypeConfig,
	listid int,
) *database.ParseInfo {
	m := database.PLParseInfo.Get()
	ParseFileP(videofile, usepath, usefolder, cfgp, listid, m)
	return m
}

// ParseFileP parses a video file to extract metadata, populating an existing ParseInfo struct.
// This is a drop-in replacement for parser.ParseFileP with the same signature.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing ParseInfo to populate.
func ParseFileP(
	videofile string,
	usepath, usefolder bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	if m == nil {
		return
	}

	// Handle audiobooks specially - they need ASIN from folder path
	if cfgp != nil && cfgp.IsType == config.MediaTypeAudiobook {
		parseAudiobookFileToParseInfo(videofile, usepath, usefolder, cfgp, listid, m)
		return
	}

	// Handle books specially
	if cfgp != nil && cfgp.IsType == config.MediaTypeBook {
		parseBookFileToParseInfo(videofile, usepath, usefolder, cfgp, listid, m)
		return
	}

	// Handle music specially
	if cfgp != nil && cfgp.IsType == config.MediaTypeMusic {
		parseMusicFileToParseInfo(videofile, usepath, usefolder, cfgp, listid, m)
		return
	}

	filename := videofile
	if usepath {
		filename = filepath.Base(videofile)
	}

	// Parse the filename
	parseFileToParseInfo(filename, false, cfgp, listid, m)

	// If quality and resolution are already set, we're done
	if m.Quality != "" && m.Resolution != "" {
		return
	}

	// Try folder name if enabled
	if usefolder && usepath {
		parseFileToParseInfo(filepath.Base(filepath.Dir(videofile)), true, cfgp, listid, m)
	}
}

// parseAudiobookFileToParseInfo parses an audiobook file to extract metadata.
// It extracts ASIN from folder paths and uses AudiobookParser for proper parsing.
func parseAudiobookFileToParseInfo(
	filepath_ string,
	usepath, usefolder bool,
	_ *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid

	filename := filepath_
	if usepath {
		filename = filepath.Base(filepath_)
	}

	m.File = filename

	// Create audiobook parser
	ap := NewAudiobookParser()

	// Parse the filename
	result := ap.Parse(filename)

	// If usefolder and usepath, also parse the folder name for additional info
	if usefolder && usepath {
		dirPath := filepath.Dir(filepath_)
		dirName := filepath.Base(dirPath)
		dirResult := ap.Parse(dirName)

		// Merge directory info (prefer directory for ASIN and title if filename lacks them)
		if dirResult.ASIN != "" && result.ASIN == "" {
			result.ASIN = dirResult.ASIN
		}

		if dirResult.Title != "" &&
			(result.Title == "" || len(dirResult.Title) > len(result.Title)) {
			result.Title = dirResult.Title
		}

		if dirResult.Author != "" && result.Author == "" {
			result.Author = dirResult.Author
		}

		if dirResult.Series != "" && result.Series == "" {
			result.Series = dirResult.Series
			result.SeriesPosition = dirResult.SeriesPosition
		}

		// Also check parent directories for ASIN (audiobooks often have ASIN in folder name) //nolint:gosec // safe: value within target type range
		if result.ASIN == "" {
			result.ASIN = extractASINFromPath(filepath_)
		}
	}

	// Populate ParseInfo from AudiobookParseResult
	m.Title = result.Title
	m.Artist = result.Author

	m.ASIN = result.ASIN
	if result.Year > 0 {
		m.Year = uint16(result.Year)
	}
}

// extractASINFromPath attempts to extract ASIN from an audiobook folder path.
// ASINs are 10-character alphanumeric codes starting with 'B' (for Audible).
func extractASINFromPath(folderPath string) string {
	// Split path into parts (handle both Unix and Windows paths)
	var parts []string
	if strings.Contains(folderPath, "/") {
		parts = strings.Split(folderPath, "/")
	} else {
		parts = strings.Split(folderPath, "\\")
	}

	// Check each path component for ASIN (starting from the end)
	for _, part := range slices.Backward(parts) {
		if asin := extractASINFromString(part); asin != "" {
			return asin
		}
	}

	return ""
}

// extractASINFromString finds an ASIN pattern in a string.
// func extractASINFromString(s string) string {
// 	// ASIN pattern: starts with B, followed by 9 alphanumeric characters
// 	asinPattern := regexp.MustCompile(`\bB[0-9A-Z]{9}\b`)
// 	if match := asinPattern.FindString(s); match != "" {
// 		return match
// 	}
// 	return ""
// }

// parseBookFileToParseInfo parses a book file to extract metadata.
func parseBookFileToParseInfo(
	filepath_ string,
	usepath, usefolder bool,
	_ *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid

	filename := filepath_
	if usepath {
		filename = filepath.Base(filepath_)
	}

	m.File = filename

	// Create book parser
	bp := NewBookParser()

	// Parse the filename
	result := bp.Parse(filename)

	// If usefolder and usepath, also parse the folder name for additional info
	if usefolder && usepath {
		dirPath := filepath.Dir(filepath_)
		dirName := filepath.Base(dirPath)
		dirResult := bp.Parse(dirName)

		// Merge directory info
		if dirResult.ISBN13 != "" && result.ISBN13 == "" {
			result.ISBN13 = dirResult.ISBN13
		}

		if dirResult.ISBN10 != "" && result.ISBN10 == "" {
			result.ISBN10 = dirResult.ISBN10
		}

		if dirResult.ASIN != "" && result.ASIN == "" {
			result.ASIN = dirResult.ASIN
		}

		if dirResult.Title != "" &&
			(result.Title == "" || len(dirResult.Title) > len(result.Title)) {
			result.Title = dirResult.Title
		}

		if dirResult.Author != "" && result.Author == "" {
			result.Author = dirResult.Author
		}
	}

	// Populate ParseInfo from BookParseResult //nolint:gosec // safe: value within target type range
	m.Title = result.Title
	m.Artist = result.Author

	m.ASIN = result.ASIN
	if result.ISBN13 != "" {
		m.ISBN = result.ISBN13
	} else if result.ISBN10 != "" {
		m.ISBN = result.ISBN10
	}

	if result.Year > 0 {
		m.Year = uint16(result.Year)
	}
}

// parseMusicFileToParseInfo parses a music file to extract metadata.
// For NZB titles like "Alabama Shakes - At The Loveless Barn (2014) FLAC",
// this uses ParseAlbumTitle to properly extract artist, album, year, and format.
func parseMusicFileToParseInfo(
	filepath_ string,
	usepath, usefolder bool,
	_ *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid

	filename := filepath_
	if usepath {
		filename = filepath.Base(filepath_)
	}

	m.File = filename

	// Create music parser
	mp := NewMusicParser()

	// Parse the filename as an album title (handles "Artist - Album (Year) Format" patterns)
	result := mp.ParseAlbumTitle(filename)

	// If usefolder and usepath, also parse the folder name for additional info
	if usefolder && usepath {
		dirPath := filepath.Dir(filepath_)
		dirName := filepath.Base(dirPath)
		dirResult := mp.ParseAlbumTitle(dirName)

		// Merge directory info (folder often has album/artist info)
		if dirResult.Album != "" && result.Album == "" {
			result.Album = dirResult.Album
		}

		if dirResult.Artist != "" && result.Artist == "" {
			result.Artist = dirResult.Artist
		}

		if dirResult.Year > 0 && result.Year == 0 {
			result.Year = dirResult.Year
		}

		if dirResult.MusicBrainzReleaseID != "" && result.MusicBrainzReleaseID == "" {
			result.MusicBrainzReleaseID = dirResult.MusicBrainzReleaseID
		}

		if dirResult.UPC != "" && result.UPC == "" {
			result.UPC = dirResult.UPC
		}
	}

	// Populate ParseInfo from MusicParseResult
	m.Title = result.Album
	m.Artist = result.Artist

	if result.Year > 0 {
		m.Year = uint16(result.Year)
	}

	// Set MusicBrainz ID and UPC if parsed
	m.MusicBrainzID = result.MusicBrainzReleaseID
	m.UPC = result.UPC
}

// parseFileToParseInfo parses a filename and populates a ParseInfo struct.
// If onlyIfEmpty is true, only empty fields will be populated.
func parseFileToParseInfo(
	filename string,
	onlyIfEmpty bool,
	_ *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid
	if !onlyIfEmpty || m.File == "" {
		m.File = filename
	}

	ps := GetPatternStore()
	ps.LoadDBPatterns()

	// Parse using VideoParser
	vp := NewVideoParserWithPatternStore(ps)

	// Get extension and base name
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Replace common separators with spaces for parsing
	cleanName := strings.ReplaceAll(name, ".", " ")

	cleanName = strings.ReplaceAll(cleanName, "_", " ")

	// Preserve original for pattern matching
	originalName := name

	// Extract IMDB ID
	if !onlyIfEmpty || m.Imdb == "" {
		if loc := vp.patterns.imdb.FindStringSubmatchIndex(originalName); len(loc) > 0 {
			m.Imdb = normalizeIMDB(originalName[loc[0]:loc[1]])
		}
	}

	// Extract TVDB ID
	if !onlyIfEmpty || m.Tvdb == "" {
		if loc := vp.patterns.tvdb.FindStringSubmatchIndex(originalName); len(loc) > 2 {
			m.Tvdb = originalName[loc[2]:loc[3]]
		}
	}

	// Extract season/episode info
	if !onlyIfEmpty || (m.Season == 0 && m.Episode == 0) {
		extractEpisodeInfo(vp, originalName, m)
	}

	// Extract year
	if !onlyIfEmpty || m.Year == 0 {
		if loc := vp.patterns.year.FindStringSubmatchIndex(originalName); len(loc) > 2 {
			m.Year = uint16(parseInt(originalName[loc[2]:loc[3]]))
		}
	}

	// Extract quality attributes using database patterns if available
	extractQualityToParseInfo(vp, originalName, m, onlyIfEmpty)

	// Extract title
	if onlyIfEmpty && m.Title != "" {
		return
	}

	// Build a temporary result to use extractTitle
	tempResult := &VideoParseResult{
		Season:     m.Season,
		Episode:    m.Episode,
		Identifier: m.Identifier,
	}

	tempResult.Year = int(m.Year)
	m.Title = vp.extractTitle(cleanName, tempResult)
}

// extractEpisodeInfo extracts season/episode information from a filename.
func extractEpisodeInfo(vp *VideoParser, name string, m *database.ParseInfo) {
	// Check for season/episode pattern
	if loc := vp.patterns.seasonEpisode.FindStringSubmatchIndex(name); len(loc) > 0 {
		if loc[2] != -1 && loc[4] != -1 {
			// Standard SxxExx format
			m.SeasonStr = name[loc[2]:loc[3]]
			m.EpisodeStr = name[loc[4]:loc[5]]
			m.Season = parseInt(m.SeasonStr)
			m.Episode = parseInt(m.EpisodeStr)
			m.Identifier = formatIdentifier(m.Season, m.Episode)
		} else if len(loc) > 10 && loc[8] != -1 && loc[10] != -1 {
			// NxNN format
			m.SeasonStr = name[loc[8]:loc[9]]
			m.EpisodeStr = name[loc[10]:loc[11]]
			m.Season = parseInt(m.SeasonStr)
			m.Episode = parseInt(m.EpisodeStr)
			m.Identifier = formatIdentifier(m.Season, m.Episode)
		}

		return
	}

	// Check alternative format
	if loc := vp.patterns.seasonEpisodeAlt.FindStringSubmatchIndex(name); len(loc) > 4 {
		m.SeasonStr = name[loc[2]:loc[3]]
		m.EpisodeStr = name[loc[4]:loc[5]]
		m.Season = parseInt(m.SeasonStr)
		m.Episode = parseInt(m.EpisodeStr)
		m.Identifier = formatIdentifier(m.Season, m.Episode)

		return
	}

	// Check date-based episodes
	if loc := vp.patterns.seasonEpisodeDate.FindStringSubmatchIndex(name); len(loc) > 6 {
		year := name[loc[2]:loc[3]]
		month := name[loc[4]:loc[5]]
		day := name[loc[6]:loc[7]]

		m.Date = logger.JoinStrings(year, "-", month, "-", day)
		m.Identifier = m.Date

		return
	}

	// Check for episode-only pattern (e.g., "E02", "E643")
	loc := vp.patterns.episodeOnly.FindStringSubmatchIndex(name)
	if len(loc) <= 2 {
		return
	}

	epStr := name[loc[2]:loc[3]]
	epNum := parseInt(epStr)

	m.Episode = epNum
	m.EpisodeStr = epStr
	// Only populate AbsoluteEpisode when no season was found
	if m.Season == 0 {
		m.AbsoluteEpisode = epNum
	}
}

// extractQualityToParseInfo extracts quality info from a filename to ParseInfo.
func extractQualityToParseInfo(
	vp *VideoParser,
	name string,
	m *database.ParseInfo,
	onlyIfEmpty bool,
) {
	// Extract resolution
	if !onlyIfEmpty || m.Resolution == "" {
		if loc := vp.patterns.resolution.FindStringSubmatchIndex(name); len(loc) > 0 {
			m.Resolution = normalizeResolution(name[loc[0]:loc[1]])
		}
	}

	// Extract quality source
	if !onlyIfEmpty || m.Quality == "" {
		if loc := vp.patterns.quality.FindStringSubmatchIndex(name); len(loc) > 2 {
			m.Quality = normalizeQuality(name[loc[2]:loc[3]])
		}
	}

	// Check for REMUX
	if vp.patterns.remux.MatchString(name) {
		m.Quality = "REMUX"
	}

	// Extract codec
	if !onlyIfEmpty || m.Codec == "" {
		if loc := vp.patterns.codec.FindStringSubmatchIndex(name); len(loc) > 2 {
			m.Codec = normalizeCodec(name[loc[2]:loc[3]])
		}
	}

	// Extract audio
	if !onlyIfEmpty || m.Audio == "" {
		if loc := vp.patterns.audio.FindStringSubmatchIndex(name); len(loc) > 2 {
			m.Audio = normalizeAudio(name[loc[2]:loc[3]])
		}
	}

	// Extract extended/proper/repack
	if !onlyIfEmpty || !m.Extended {
		m.Extended = vp.patterns.extended.MatchString(name)
	}

	if !onlyIfEmpty || !m.Proper {
		m.Proper = vp.patterns.proper.MatchString(name)
	}

	if !onlyIfEmpty || !m.Repack {
		m.Repack = vp.patterns.repack.MatchString(name)
	}
}

// detectMediaType determines if content is movie or series and extracts episode info.
func (vp *VideoParser) detectMediaType(name string, result *VideoParseResult) MediaType {
	// Check for season/episode pattern
	if loc := vp.patterns.seasonEpisode.FindStringSubmatchIndex(name); len(loc) > 0 {
		// Pattern has two capture groups: standard S01E02 format OR NxNN format
		if loc[2] != -1 && loc[4] != -1 {
			// Standard SxxExx format
			result.Season = parseInt(name[loc[2]:loc[3]])
			result.Episode = parseInt(name[loc[4]:loc[5]])
		} else if len(loc) > 10 && loc[8] != -1 && loc[10] != -1 {
			// NxNN format (e.g., 1x01)
			result.Season = parseInt(name[loc[8]:loc[9]])
			result.Episode = parseInt(name[loc[10]:loc[11]])
		}

		if result.Season > 0 || result.Episode > 0 {
			result.Identifier = vp.buildIdentifier(result.Season, result.Episode)

			// Check for multi-episode
			if len(loc) > 6 && loc[6] != -1 {
				endEp := parseInt(name[loc[6]:loc[7]])
				if endEp > result.Episode {
					result.Identifier = vp.buildMultiIdentifier(
						result.Season,
						result.Episode,
						endEp,
					)
				}
			}

			return MediaTypeSeries
		}
	}

	// Check alternative pattern
	if loc := vp.patterns.seasonEpisodeAlt.FindStringSubmatchIndex(name); len(loc) > 4 {
		result.Season = parseInt(name[loc[2]:loc[3]])
		result.Episode = parseInt(name[loc[4]:loc[5]])
		result.Identifier = vp.buildIdentifier(result.Season, result.Episode)
		return MediaTypeSeries
	}

	// Check for date-based episode
	if loc := vp.patterns.seasonEpisodeDate.FindStringSubmatchIndex(name); len(loc) > 6 {
		result.Identifier = name[loc[2]:loc[3]] + "-" + name[loc[4]:loc[5]] + "-" + name[loc[6]:loc[7]]
		return MediaTypeSeries
	}

	// Check for TVDB ID (strong series indicator)
	if result.Tvdb != "" {
		return MediaTypeSeries
	}

	// Check for multi-season indicator
	if vp.patterns.multiSeason.MatchString(name) {
		return MediaTypeSeries
	}

	// Check for complete series indicator
	if vp.patterns.complete.MatchString(name) {
		return MediaTypeSeries
	}

	// Check for episode-only pattern (less reliable)
	if loc := vp.patterns.episodeOnly.FindStringSubmatchIndex(name); len(loc) > 2 {
		epNum := parseInt(name[loc[2]:loc[3]])

		result.Episode = epNum
		// Only populate AbsoluteEpisode when no season was found
		if result.Season == 0 {
			result.AbsoluteEpisode = epNum
		}

		return MediaTypeSeries
	}

	// Default to movie if explicit mode set
	if vp.isMovie {
		return MediaTypeMovie
	}

	// Heuristics for detection when no clear indicators
	if result.Imdb != "" && strings.HasPrefix(result.Imdb, "tt") {
		return MediaTypeMovie // IMDB ID more common for movies
	}

	// Default based on no episode info
	return MediaTypeMovie
}

// extractQualityInfo extracts resolution, quality, codec, and audio information.
func (vp *VideoParser) extractQualityInfo(name string, result *VideoParseResult) {
	// Use database patterns if available, otherwise use hardcoded patterns
	if vp.useDBPattern && vp.patternStore != nil {
		vp.extractQualityInfoFromDB(name, result)
	} else {
		vp.extractQualityInfoBuiltin(name, result)
	}

	// Check for HDR (always use builtin pattern)
	if loc := vp.patterns.hdr.FindStringSubmatchIndex(name); len(loc) > 2 {
		hdr := normalizeHDR(name[loc[2]:loc[3]])
		if hdr != "" && result.Resolution != "" {
			result.Resolution = result.Resolution + " " + hdr
		}
	}

	// Check for extended/proper/repack (always use builtin patterns)
	result.Extended = vp.patterns.extended.MatchString(name)
	result.Proper = vp.patterns.proper.MatchString(name)
	result.Repack = vp.patterns.repack.MatchString(name)
}

// extractQualityInfoBuiltin uses the hardcoded regex patterns.
func (vp *VideoParser) extractQualityInfoBuiltin(name string, result *VideoParseResult) {
	// Extract resolution
	if loc := vp.patterns.resolution.FindStringSubmatchIndex(name); len(loc) > 0 {
		result.Resolution = normalizeResolution(name[loc[0]:loc[1]])
	}

	// Extract quality source
	if loc := vp.patterns.quality.FindStringSubmatchIndex(name); len(loc) > 2 {
		result.Quality = normalizeQuality(name[loc[2]:loc[3]])
	}

	// Check for REMUX (high quality indicator)
	if vp.patterns.remux.MatchString(name) {
		result.Quality = "REMUX"
	}

	// Extract codec
	if loc := vp.patterns.codec.FindStringSubmatchIndex(name); len(loc) > 2 {
		result.Codec = normalizeCodec(name[loc[2]:loc[3]])
	}

	// Extract audio
	if loc := vp.patterns.audio.FindStringSubmatchIndex(name); len(loc) > 2 {
		result.Audio = normalizeAudio(name[loc[2]:loc[3]])
	}
}

// extractQualityInfoFromDB uses the database patterns (similar to Parsegroup).
func (vp *VideoParser) extractQualityInfoFromDB(name string, result *VideoParseResult) {
	ps := vp.patternStore

	// Try string matching first (fast path)
	if resName, _, _ := ps.MatchString(name, PatternTypeResolution); resName != "" {
		result.Resolution = resName
		result.ResolutionID = ps.GetPatternID(resName, PatternTypeResolution)
	}

	if qualName, _, _ := ps.MatchString(name, PatternTypeQuality); qualName != "" {
		result.Quality = qualName
		result.QualityID = ps.GetPatternID(qualName, PatternTypeQuality)
	}

	if codecName, _, _ := ps.MatchString(name, PatternTypeCodec); codecName != "" {
		result.Codec = codecName
		result.CodecID = ps.GetPatternID(codecName, PatternTypeCodec)
	}

	if audioName, _, _ := ps.MatchString(name, PatternTypeAudio); audioName != "" {
		result.Audio = audioName
		result.AudioID = ps.GetPatternID(audioName, PatternTypeAudio)
	}

	// Try regex matching for any empty fields
	if result.Resolution == "" {
		if resName, _, _, _ := ps.MatchRegex(name, PatternTypeResolution); resName != "" {
			result.Resolution = resName
			result.ResolutionID = ps.GetPatternID(resName, PatternTypeResolution)
		}
	}

	if result.Quality == "" {
		if qualName, _, _, _ := ps.MatchRegex(name, PatternTypeQuality); qualName != "" {
			result.Quality = qualName
			result.QualityID = ps.GetPatternID(qualName, PatternTypeQuality)
		}
	}

	if result.Codec == "" {
		if codecName, _, _, _ := ps.MatchRegex(name, PatternTypeCodec); codecName != "" {
			result.Codec = codecName
			result.CodecID = ps.GetPatternID(codecName, PatternTypeCodec)
		}
	}

	if result.Audio == "" {
		if audioName, _, _, _ := ps.MatchRegex(name, PatternTypeAudio); audioName != "" {
			result.Audio = audioName
			result.AudioID = ps.GetPatternID(audioName, PatternTypeAudio)
		}
	}

	// Check for REMUX
	if vp.patterns.remux.MatchString(name) {
		result.Quality = "REMUX"
	}
}

// extractTitle attempts to extract the title from the cleaned name.
func (vp *VideoParser) extractTitle(cleanName string, _ *VideoParseResult) string {
	// Find the first quality/episode indicator to determine title boundary
	indicatorPatterns := []*regexp.Regexp{
		vp.patterns.seasonEpisode,
		vp.patterns.seasonEpisodeAlt,
		vp.patterns.seasonEpisodeDate,
		vp.patterns.year,
		vp.patterns.resolution,
		vp.patterns.quality,
		vp.patterns.language, // Language codes also indicate end of title
	}

	minIdx := len(cleanName)

	for _, pattern := range indicatorPatterns {
		if loc := pattern.FindStringIndex(cleanName); loc != nil && loc[0] < minIdx {
			minIdx = loc[0]
		}
	}

	title := cleanName
	if minIdx > 0 && minIdx < len(cleanName) {
		title = cleanName[:minIdx]
	}

	// Strip any remaining language codes from the title
	title = vp.patterns.language.ReplaceAllLiteralString(title, " ")

	return cleanTitle(title)
}

// buildIdentifier creates an episode identifier string.
func (vp *VideoParser) buildIdentifier(season, episode int) string {
	return formatIdentifier(season, episode)
}

// formatIdentifier creates an episode identifier string (e.g., "S01E05").
func formatIdentifier(season, episode int) string {
	return logger.JoinStrings("S", padInt(season), "E", padInt(episode))
}

// buildMultiIdentifier creates a multi-episode identifier string.
func (vp *VideoParser) buildMultiIdentifier(season, startEp, endEp int) string {
	return logger.JoinStrings("S", padInt(season), "E", padInt(startEp), "-E", padInt(endEp))
}

// calculateConfidence calculates the confidence score for the parse.
func (vp *VideoParser) calculateConfidence(result *VideoParseResult) float64 {
	var conf float64

	// Title is essential
	if result.Title != "" {
		conf += 0.25
	}

	// External identifiers are strong
	if result.Imdb != "" {
		conf += 0.2
	}

	if result.Tvdb != "" {
		conf += 0.15
	}

	// Year adds confidence
	if result.Year > 0 {
		conf += 0.1
	}

	// Quality info adds confidence
	if result.Resolution != "" {
		conf += 0.1
	}

	if result.Quality != "" {
		conf += 0.05
	}

	// For series, episode info is important
	if result.MediaType == MediaTypeSeries {
		if result.Season > 0 || result.Episode > 0 || result.Identifier != "" {
			conf += 0.15
		}
	}

	if conf > 1.0 {
		conf = 1.0
	}

	return conf
}

// normalizeIMDB normalizes an IMDB ID to the tt0000000 format.
func normalizeIMDB(imdb string) string {
	// Extract just the ttNNNNNNN part
	if loc := database.GetCachedRegexp(`(?i)(tt\d{7,8})`).
		FindStringSubmatchIndex(imdb); len(loc) > 2 {
		m := imdb[loc[2]:loc[3]]
		// Only the leading two bytes can differ in case ('t' vs 'T').
		// Digits that follow have no case, so check directly instead of strings.ToLower.
		if m[0] == 't' && m[1] == 't' {
			return m // already canonical, no allocation
		}

		return "tt" + m[2:]
	}

	return strings.ToLower(strings.TrimSpace(imdb))
}

// normalizeResolution normalizes resolution strings.
func normalizeResolution(res string) string {
	switch {
	case logger.ContainsI(res, "2160") || logger.ContainsI(res, "4k") || logger.ContainsI(res, "uhd"):
		return "2160p"
	case logger.ContainsI(res, "1080"):
		return "1080p"
	case logger.ContainsI(res, "720"):
		return "720p"
	case logger.ContainsI(res, "576"):
		return "576p"
	case logger.ContainsI(res, "480"):
		return "480p"
	case logger.ContainsI(res, "sd"):
		return "SD"
	case logger.ContainsI(res, "hd"):
		return "HD"
	default:
		return strings.ToUpper(strings.TrimSpace(res))
	}
}

// normalizeQuality normalizes quality source strings.
func normalizeQuality(quality string) string {
	switch {
	case logger.ContainsI(quality, "bluray") || logger.ContainsI(quality, "blu-ray") ||
		logger.ContainsI(quality, "blu ray") || logger.ContainsI(quality, "blu_ray"):
		return "BluRay"
	case logger.ContainsI(quality, "bdrip") || logger.ContainsI(quality, "brrip"):
		return "BDRip"
	case logger.ContainsI(quality, "web-dl") || logger.ContainsI(quality, "webdl") ||
		logger.ContainsI(quality, "web dl") || logger.ContainsI(quality, "web_dl"):
		return "WEB-DL"
	case logger.ContainsI(quality, "webrip"):
		return "WEBRip"
	case logger.ContainsI(quality, "hdtv"):
		return "HDTV"
	case logger.ContainsI(quality, "dvdrip"):
		return "DVDRip"
	case logger.ContainsI(quality, "amzn") || logger.ContainsI(quality, "amazon"):
		return "AMZN"
	case logger.ContainsI(quality, "netflix") || logger.ContainsI(quality, "nf"):
		return "NF"
	case logger.ContainsI(quality, "dsnp") || logger.ContainsI(quality, "disney"):
		return "DSNP"
	case logger.ContainsI(quality, "hmax") || logger.ContainsI(quality, "hbo"):
		return "HMAX"
	default:
		q := strings.ToUpper(strings.TrimSpace(quality))

		q = strings.ReplaceAll(q, " ", "-")
		q = strings.ReplaceAll(q, "_", "-")

		return q
	}
}

// normalizeCodec normalizes codec strings.
func normalizeCodec(codec string) string {
	switch {
	case logger.ContainsI(codec, "x264") || logger.ContainsI(codec, "x.264") || logger.ContainsI(codec, "x 264") ||
		logger.ContainsI(
			codec,
			"h264",
		) || logger.ContainsI(codec, "h.264") || logger.ContainsI(codec, "h 264") ||
		logger.ContainsI(codec, "avc"):
		return "x264"
	case logger.ContainsI(codec, "x265") || logger.ContainsI(codec, "x.265") || logger.ContainsI(codec, "x 265") ||
		logger.ContainsI(
			codec,
			"h265",
		) || logger.ContainsI(codec, "h.265") || logger.ContainsI(codec, "h 265") ||
		logger.ContainsI(codec, "hevc"):
		return "x265"
	case logger.ContainsI(codec, "xvid"):
		return "XviD"
	case logger.ContainsI(codec, "divx"):
		return "DivX"
	case logger.ContainsI(codec, "av1"):
		return "AV1"
	default:
		c := strings.ToUpper(strings.TrimSpace(codec))

		c = strings.ReplaceAll(c, ".", "")
		c = strings.ReplaceAll(c, " ", "")

		return c
	}
}

// normalizeAudio normalizes audio codec strings.
func normalizeAudio(audio string) string {
	switch {
	case logger.ContainsI(audio, "dts-hd-ma") || logger.ContainsI(audio, "dtshdma") ||
		logger.ContainsI(audio, "dts hd ma") || logger.ContainsI(audio, "dts-hd ma") ||
		logger.ContainsI(audio, "dts hd-ma"):
		return "DTS-HD MA"
	case logger.ContainsI(audio, "dts-hd") || logger.ContainsI(audio, "dtshd") ||
		logger.ContainsI(audio, "dts hd"):
		return "DTS-HD"
	case logger.ContainsI(audio, "dts-x") || logger.ContainsI(audio, "dtsx") ||
		logger.ContainsI(audio, "dts x"):
		return "DTS:X"
	case logger.ContainsI(audio, "dts"):
		return "DTS"
	case logger.ContainsI(audio, "atmos"):
		return "Atmos"
	case logger.ContainsI(audio, "truehd"):
		return "TrueHD"
	case logger.ContainsI(audio, "dd+") || logger.ContainsI(audio, "ddp") || logger.ContainsI(audio, "eac3"):
		return "DD+"
	case logger.ContainsI(audio, "dd") || logger.ContainsI(audio, "ac3"):
		return "DD"
	case logger.ContainsI(audio, "aac"):
		return "AAC"
	case logger.ContainsI(audio, "flac"):
		return "FLAC"
	default:
		a := strings.ToUpper(strings.TrimSpace(audio))

		a = strings.ReplaceAll(a, " ", "-")
		return a
	}
}

// normalizeHDR normalizes HDR format strings.
func normalizeHDR(hdr string) string {
	switch {
	case logger.ContainsI(hdr, "hdr10+") || logger.ContainsI(hdr, "hdr10plus"):
		return "HDR10+"
	case logger.ContainsI(hdr, "hdr10"):
		return "HDR10"
	case logger.ContainsI(hdr, "dolby") || logger.ContainsI(hdr, "vision") ||
		strings.EqualFold(strings.Trim(strings.TrimSpace(hdr), "._- "), "DV"):
		return "DV"
	case logger.ContainsI(hdr, "hlg"):
		return "HLG"
	case logger.ContainsI(hdr, "hdr"):
		return "HDR"
	default:
		return strings.ToUpper(strings.Trim(strings.TrimSpace(hdr), "._- "))
	}
}

// padInt pads an integer to 2 digits with a leading zero.
func padInt(n int) string {
	if n >= 10 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}

	return "0" + string(rune('0'+n))
}
