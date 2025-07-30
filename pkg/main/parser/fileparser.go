package parser

import (
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

type regexpattern struct {
	name string
	// REs need to have 2 sub expressions (groups), the first one is "raw", and
	// the second one for the "clean" value.
	// E.g. Epiode matching on "S01E18" will result in: raw = "E18", clean = "18".
	re       string
	getgroup int
	// Use the last matching pattern. E.g. Year.
	last bool
}

type Prioarr struct {
	QualityGroup string
	Priority     int
	ResolutionID uint
	QualityID    uint
	CodecID      uint
	AudioID      uint
}

var (
	errNotFoundDBEpisode        = errors.New("dbepisode not found")
	errNotFoundSerie            = errors.New("serie not found")
	allQualityPrioritiesT       []Prioarr
	allQualityPrioritiesWantedT []Prioarr
	mediainfopath               string
	ffprobepath                 string
	arrExtended                 = [4]string{
		"extended",
		"extended cut",
		"extended.cut",
		"extended-cut",
	}

	scanpatterns       []regexpattern
	globalscanpatterns = [8]regexpattern{
		{name: "season", last: false, re: `(?i)(s?(\d{1,4}))(?: )?[ex]`, getgroup: 2},
		{
			name:     "episode",
			last:     false,
			re:       `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`,
			getgroup: 2,
		},
		{
			name:     "identifier",
			last:     false,
			re:       `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`,
			getgroup: 2,
		},
		{
			name:     logger.StrDate,
			last:     false,
			re:       `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`,
			getgroup: 2,
		},
		{name: "year", last: true, re: `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, getgroup: 2},
		{
			name:     "audio",
			last:     false,
			re:       `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`,
			getgroup: 2,
		},
		{name: "imdb", last: false, re: `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, getgroup: 2},
		{name: "tvdb", last: false, re: `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, getgroup: 2},
	}
)

// getmatchesroot finds all substring matches in m.Str using the provided regular
// expression pattern. It returns a slice of integer indices indicating the start
// and end positions of the matched substring(s). For regex capture groups, the
// even indices are start positions and odd indices are end positions.
func (pattern *regexpattern) getmatchesroot(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
) (int, int) {
	matchest := database.RunRetRegex(
		pattern.re,
		m.Str,
		(pattern.last && pattern.name == "year" && cfgp.Useseries),
	)
	if len(matchest) == 0 {
		return -1, -1
	}

	lensubmatches := len(matchest)
	groupIndex := pattern.getgroup * 2
	if groupIndex+1 >= lensubmatches || matchest[groupIndex+1] == -1 {
		return -1, -1
	}

	if lensubmatches >= 4 && matchest[3] != -1 && groupIndex >= 4 {
		return matchest[groupIndex], matchest[groupIndex+1]
	} else if lensubmatches <= 2 && groupIndex < lensubmatches*2 {
		return matchest[groupIndex], matchest[groupIndex+1]
	} else if lensubmatches >= 4 && matchest[3] != -1 {
		return -1, -1
	}

	return matchest[groupIndex], matchest[groupIndex+1]
}

// getImdbFilename returns the path to the init_imdb executable
// based on the current OS. For Windows it returns init_imdb.exe,
// for other OSes it returns ./init_imdb.
func getImdbFilename() string {
	if runtime.GOOS == "windows" {
		return "init_imdb.exe"
	}
	return "./init_imdb"
}

// Getallprios returns all quality priorities in descending order of quality. This is a copy.
func Getallprios() []Prioarr {
	return allQualityPrioritiesWantedT
}

// Getcompleteallprios returns all quality priorities in descending order of quality. This is useful for testing.
func Getcompleteallprios() []Prioarr {
	return allQualityPrioritiesT
}

// LoadDBPatterns loads patterns from database if not already loaded.
func LoadDBPatterns() {
	if len(scanpatterns) >= 1 {
		return
	}
	capacity := len(globalscanpatterns)
	for _, val := range database.DBConnect.GetaudiosIn {
		if val.UseRegex {
			capacity++
		}
	}
	for _, val := range database.DBConnect.GetcodecsIn {
		if val.UseRegex {
			capacity++
		}
	}
	for _, val := range database.DBConnect.GetqualitiesIn {
		if val.UseRegex {
			capacity++
		}
	}
	for _, val := range database.DBConnect.GetresolutionsIn {
		if val.UseRegex {
			capacity++
		}
	}
	scanpatterns = make([]regexpattern, 0, capacity)
	scanpatterns = append(scanpatterns, globalscanpatterns[:]...)

	for i := range globalscanpatterns {
		database.SetStaticRegexp(globalscanpatterns[i].re)
	}

	addPatterns := func(items []database.Qualities, patternName string) {
		for _, val := range items {
			if val.UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{
					name:     patternName,
					last:     false,
					re:       val.Regex,
					getgroup: val.Regexgroup,
				})
				database.SetStaticRegexp(val.Regex)
			}
		}
	}
	addPatterns(database.DBConnect.GetaudiosIn, "audio")
	addPatterns(database.DBConnect.GetresolutionsIn, "resolution")
	addPatterns(database.DBConnect.GetqualitiesIn, "quality")
	addPatterns(database.DBConnect.GetcodecsIn, "codec")
}

// GenerateCutoffPriorities iterates through the media type and list
// configurations, and sets the CutoffPriority field for any list that
// does not already have it set. It calls NewCutoffPrio to calculate
// the priority value based on the cutoff quality and resolution.
func GenerateCutoffPriorities() {
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		for _, lst := range media.ListsMap {
			if lst.CfgQuality.CutoffPriority != 0 {
				continue
			}
			m := database.ParseInfo{
				Quality:    lst.CfgQuality.CutoffQuality,
				Resolution: lst.CfgQuality.CutoffResolution,
			}
			GetPriorityMapQual(&m, media, lst.CfgQuality, true, false) // newCutoffPrio(media, idxi)
			lst.CfgQuality.CutoffPriority = m.Priority
		}
		return nil
	})
}

// processPatternMatch handles the processing of a matched regex pattern in file parsing.
// It updates the ParseInfo struct with matched information based on the pattern type,
// and manages the start and end indices of the matched substring within the original string.
// The function is used internally during file name parsing to extract metadata like
// IMDb ID, year, season, episode, and other media-related information.
func processPatternMatch(
	m *database.ParseInfo,
	pattern *regexpattern,
	strStart, strEnd int,
	cfgp *config.MediaTypeConfig,
	start, end *int,
) {
	if !cfgp.Useseries || pattern.name != "year" {
		if index := strings.Index(m.Str, m.Str[strStart:strEnd]); index == 0 {
			if matchLen := len(m.Str[strStart:strEnd]); matchLen != len(m.Str) && matchLen < *end {
				*start = matchLen
			}
		} else if index < *end && index > *start {
			*end = index
		}
	}

	if m.FirstIDX == 0 || strStart < m.FirstIDX {
		m.FirstIDX = strStart
	}

	// Use a map for better performance on pattern matching
	switch pattern.name {
	case "imdb":
		m.Imdb = m.Str[strStart:strEnd]
	case "tvdb":
		m.Tvdb, _ = strings.CutPrefix(m.Str[strStart:strEnd], logger.StrTvdb)
		if logger.HasPrefixI(m.Tvdb, logger.StrTvdb) {
			m.Tvdb = m.Tvdb[4:]
		}
	case "year":
		m.FirstYearIDX = strStart
		m.Year = logger.StringToUInt16(m.Str[strStart:strEnd])
	case "season":
		m.SeasonStr = m.Str[strStart:strEnd]
		m.Season = logger.StringToInt(m.SeasonStr)
	case "episode":
		m.EpisodeStr = m.Str[strStart:strEnd]
		m.Episode = logger.StringToInt(m.EpisodeStr)
	case "identifier":
		m.Identifier = m.Str[strStart:strEnd]
	case logger.StrDate:
		m.Date = m.Str[strStart:strEnd]
	case "audio":
		m.Audio = m.Str[strStart:strEnd]
	case "resolution":
		m.Resolution = m.Str[strStart:strEnd]
	case "quality":
		m.Quality = m.Str[strStart:strEnd]
	case "codec":
		m.Codec = m.Str[strStart:strEnd]
	}
}

// shouldSkipPattern determines whether a specific regex pattern should be skipped during file parsing.
// It checks various conditions based on the pattern type, media type configuration, and existing parsed information.
// Returns true if the pattern should be skipped, false otherwise.
func shouldSkipPattern(
	pattern *regexpattern,
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	onlyifempty bool,
	conttt, conttvdb bool,
) bool {
	switch pattern.name {
	case "imdb":
		return cfgp.Useseries || !conttt || (onlyifempty && m.Imdb != "")
	case "tvdb":
		return !cfgp.Useseries || !conttvdb || (onlyifempty && m.Tvdb != "")
	case "season":
		return !cfgp.Useseries || (onlyifempty && m.Season != 0)
	case "identifier":
		return !cfgp.Useseries || (onlyifempty && m.Identifier != "")
	case "episode":
		return !cfgp.Useseries || (onlyifempty && m.Episode != 0)
	case "date":
		return !cfgp.Useseries || (onlyifempty && m.Date != "")
	case "audio":
		return m.Audio != ""
	case "codec":
		return m.Codec != ""
	case "quality":
		return m.Quality != ""
	case "resolution":
		return m.Resolution != ""
	}
	return false
}

// newFileParser reuses a FileParser instance. It sets the filename,
// media config, list ID, and allow title search flag. It runs the main parsing
// logic like splitting the filename on delimiters, running regex matches,
// and cleaning up the parsed title and identifier.
func newFileParser(
	cleanName string,
	onlyifempty bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid
	if !onlyifempty || m.File == "" {
		m.File = cleanName
	}
	m.Str = m.File
	logger.StringReplaceWithP(&m.Str, '_', ' ')
	m.Str = logger.Trim(m.Str, '[', ']')

	if !config.GetSettingsGeneral().DisableParserStringMatch {
		m.Parsegroup("audio", onlyifempty, database.DBConnect.AudioStrIn)
		m.Parsegroup("codec", onlyifempty, database.DBConnect.CodecStrIn)
		m.Parsegroup("quality", onlyifempty, database.DBConnect.QualityStrIn)
		m.Parsegroup("resolution", onlyifempty, database.DBConnect.ResolutionStrIn)
	}

	m.Parsegroup("extended", onlyifempty, arrExtended[:])
	m.ParsegroupEntry("proper")
	m.ParsegroupEntry("repack")

	var (
		start, end = 0, len(m.Str)
		conttt     = logger.ContainsI(m.Str, logger.StrTt)
		conttvdb   = logger.ContainsI(m.Str, logger.StrTvdb)
	)

	for i := range scanpatterns {
		pattern := &scanpatterns[i]
		if shouldSkipPattern(pattern, m, cfgp, onlyifempty, conttt, conttvdb) {
			continue
		}
		if strStart, strEnd := pattern.getmatchesroot(m, cfgp); strStart != -1 && strEnd != -1 {
			processPatternMatch(m, pattern, strStart, strEnd, cfgp, &start, &end)
		}
	}
	if cfgp.Useseries && (!onlyifempty || m.Identifier == "") {
		if m.Identifier == "" && m.SeasonStr != "" && m.EpisodeStr != "" {
			m.GenerateIdentifierString()
		}

		if m.Date != "" && m.Identifier == "" {
			m.Identifier = m.Date
		}
		m.Identifier = logger.Trim(m.Identifier, '-', '.', ' ')
	}
	if m.FirstIDX != 0 && m.FirstIDX < m.FirstYearIDX {
		end = m.FirstIDX
	}
	var titleStr string
	if end < start {
		logger.Logtype("debug", 0).
			Str(logger.StrPath, m.File).
			Int("start", start).
			Int("end", end).
			Msg("EndIndex < startindex")
		titleStr = m.File[start:]
	} else {
		titleStr = m.File[start:end]
	}
	if idx := strings.IndexRune(titleStr, '('); idx != -1 {
		titleStr = titleStr[:idx]
	}

	m.Str = titleStr

	if onlyifempty && m.Title != "" {
		return
	}
	m.Title = titleStr
	if strings.ContainsRune(m.Title, '.') && !strings.ContainsRune(m.Title, ' ') {
		logger.StringReplaceWithP(&m.Title, '.', ' ')
	}
	m.Title = logger.TrimSpace(logger.TrimRight(logger.TrimSpace(m.Title), '-', '.', ' '))
}

// SplitByFull splits a string into two parts by the first
// occurrence of the split rune. It returns the part before the split.
// If the split rune is not found, it returns the original string.
func splitByFull(str string, splitby rune) string {
	if idx := strings.IndexRune(str, splitby); idx != -1 {
		return str[:idx]
	}
	return str
}

// ParseFile parses the given video file to extract metadata.
// It accepts a video file path, booleans to indicate whether to use the path and folder
// to extract metadata, a media type config, a list ID, and a FileParser to populate.
// It calls ParseFileP to parse the file and populate the FileParser, which is then returned.
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

// ParseFileP parses a video file to extract metadata.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing parser to populate.
// It returns the populated parser after attempting to extract metadata.
func ParseFileP(
	videofile string,
	usepath, usefolder bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	filename := videofile
	if usepath {
		filename = filepath.Base(videofile)
	}
	newFileParser(filename, false, cfgp, listid, m)

	if m.Quality != "" && m.Resolution != "" {
		return
	}
	if usefolder && usepath {
		newFileParser(filepath.Base(filepath.Dir(videofile)), true, cfgp, listid, m)
	}
}

// GetDBIDs retrieves the database IDs needed to locate a movie or TV episode in the database.
// It takes a FileParser struct pointer as input. This contains metadata about the media file.
// It first checks if it is a movie or TV show based on the config.
// For movies:
// It tries to lookup the movie by IMDb ID, trying prefixes if not found
// If still not found, it searches by title
// It gets the movie ID and list ID
// Returns error if not found
// For TV shows:
// Lookup by TVDB ID
// If not found, search by title and year
// Get the episode ID using other metadata
// Get the series ID and list ID
// Returns error if IDs not found
// The goal is to map the metadata from the file to the database IDs needed to locate that movie or episode. This allows further processing on the database data.
// It returns errors if it can't find the expected IDs.
func GetDBIDs(m *database.ParseInfo, cfgp *config.MediaTypeConfig, allowsearchtitle bool) error {
	if m == nil {
		return logger.ErrNotFound
	}
	m.ListID = -1

	if !cfgp.Useseries {
		return getMovieDBIDs(m, cfgp, allowsearchtitle)
	}
	return getSeriesDBIDs(m, cfgp, allowsearchtitle)
}

// getMovieDBIDs retrieves database IDs for a movie by attempting multiple lookup strategies.
// It first tries IMDb ID lookup with padding optimization, then falls back to title-based search.
// If an IMDb ID is found, it attempts to locate the movie in the database and configured media lists.
//
// Parameters:
//   - m: Pointer to ParseInfo containing movie metadata
//   - cfgp: Media type configuration
//   - allowsearchtitle: Flag to enable title-based search
//
// Returns an error if no movie database ID can be found.
func getMovieDBIDs(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowsearchtitle bool,
) error {
	// Handle IMDB lookup with padding optimization
	if m.Imdb != "" {
		if !logger.HasPrefixI(m.Imdb, logger.StrTt) {
			sourceimdb := m.Imdb
			// Try different padding levels efficiently
			paddings := []string{"", "0", "00", "000", "0000"}
			for _, padding := range paddings {
				if len(sourceimdb)+len(padding) >= 7 && padding != "" {
					break
				}
				m.Imdb = padding + sourceimdb
				m.MovieFindDBIDByImdbParser()
				if m.DbmovieID != 0 {
					break
				}
			}
		} else {
			m.MovieFindDBIDByImdbParser()
		}
	}

	// Title-based search if IMDB lookup failed
	if m.DbmovieID == 0 && m.Title != "" && allowsearchtitle && cfgp.Name != "" {
		// Strip title prefixes/postfixes
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}
		m.Title = logger.TrimSpace(m.Title)

		if m.Imdb == "" {
			importfeed.MovieFindImdbIDByTitle(false, m, cfgp)
		}
		if m.Imdb != "" && m.DbmovieID == 0 {
			m.MovieFindDBIDByImdbParser()
		}
	}

	if m.DbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}

	// Find movie in lists
	return findMovieInLists(m, cfgp)
}

// getSeriesDBIDs retrieves database IDs for a TV series by attempting multiple lookup strategies.
// It first tries TVDB lookup, then falls back to title-based search (with optional year),
// and uses regex matching as a final attempt. If a series database ID is found, it sets
// the corresponding episode ID and attempts to locate the series in configured media lists.
//
// Parameters:
//   - m: Pointer to ParseInfo containing series metadata
//   - cfgp: Media type configuration
//   - allowsearchtitle: Flag to enable title-based search
//
// Returns an error if no series or episode database ID can be found.
func getSeriesDBIDs(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowsearchtitle bool,
) error {
	// TVDB lookup
	if m.Tvdb != "" {
		database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &m.DbserieID, &m.Tvdb)
	}

	// Title-based search
	if m.DbserieID == 0 && m.Title != "" && (allowsearchtitle || m.Tvdb == "") {
		if m.Year != 0 {
			titleWithYear := logger.JoinStrings(m.Title, " (", logger.IntToString(m.Year), ")")
			m.FindDbserieByNameWithSlug(titleWithYear)
		}
		if m.DbserieID == 0 {
			m.FindDbserieByNameWithSlug(m.Title)
		}
	}

	// Regex fallback
	if m.DbserieID == 0 && m.File != "" {
		m.RegexGetMatchesStr1(cfgp)
	}

	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}

	// Set episode ID
	m.SetDBEpisodeIDfromM()
	if m.DbserieEpisodeID == 0 {
		return errNotFoundDBEpisode
	}

	// Find series in lists
	return findSeriesInLists(m, cfgp)
}

// findMovieInLists attempts to locate a movie in configured media lists by its database ID.
// It first checks if a list ID is already specified, then searches through available lists.
// If no movie is found, it returns an error. The function updates the ParseInfo
// with the found movie ID and list index.
func findMovieInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			database.QueryMoviesGetIDByDBIDListname,
			&m.MovieID,
			&m.DbmovieID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.MovieID == 0 && cfgp.Name != "" && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.MovieID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheMovie,
					m.DbmovieID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(false, "select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", &m.MovieID, &cfgp.Lists[idx].Name, &m.DbmovieID)
			}
			if m.MovieID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.MovieID == 0 {
		return logger.ErrNotFoundMovie
	}
	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.MovieID)
	}
	return nil
}

// findSeriesInLists attempts to locate a series in configured media lists by its database ID.
// It first checks if a list ID is already specified, then searches through available lists.
// If no series is found, it resets episode-related IDs and returns an error.
// The function updates the ParseInfo with the found series and episode IDs.
func findSeriesInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			database.QuerySeriesGetIDByDBIDListname,
			&m.SerieID,
			&m.DbserieID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.SerieID == 0 && cfgp != nil && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.SerieID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheSeries,
					m.DbserieID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(false, database.QuerySeriesGetIDByDBIDListname, &m.SerieID, &m.DbserieID, &cfgp.Lists[idx].Name)
			}
			if m.SerieID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.SerieID == 0 {
		m.DbserieEpisodeID = 0
		m.SerieEpisodeID = 0
		return errNotFoundSerie
	}

	m.SetEpisodeIDfromM()
	if m.SerieEpisodeID == 0 {
		return logger.ErrNotFoundEpisode
	}
	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.SerieID)
	}
	return nil
}

// ParseVideoFile parses metadata for a video file using ffprobe or MediaInfo.
// It first tries ffprobe, then falls back to MediaInfo if enabled.
// It takes a FileParser, path to the video file, and quality settings.
// It populates the FileParser with metadata parsed from the file.
// Returns an error if both parsing methods fail.
func ParseVideoFile(m *database.ParseInfo, quality *config.QualityConfig) error {
	if m.File == "" {
		return logger.ErrNotFound
	}

	err := parsemedia(!config.GetSettingsGeneral().UseMediainfo, m, quality)
	if err == nil {
		return nil
	}
	if !config.GetSettingsGeneral().UseMediaFallback {
		return err
	}
	return parsemedia(config.GetSettingsGeneral().UseMediainfo, m, quality)
}

// parsemedia attempts to parse the metadata of a video file using either ffprobe or MediaInfo.
// If ffprobe is enabled, it first tries to parse the file using ffprobe. If that fails or ffprobe is
// not enabled, it falls back to using MediaInfo to parse the file.
// It takes a boolean indicating whether to use ffprobe, a pointer to a ParseInfo struct to populate
// with the parsed metadata, and a pointer to a QualityConfig struct.
// Returns an error if both parsing methods fail.
func parsemedia(ffprobe bool, m *database.ParseInfo, quality *config.QualityConfig) error {
	if m.File == "" {
		return logger.ErrNotFound
	}
	if ffprobe {
		if ExecCmdJSON[ffProbeJSON](m.File, "ffprobe", m, quality) == nil {
			return nil
		}
	}
	return ExecCmdJSON[mediaInfoJSON](m.File, "mediainfo", m, quality)
}

// parseffprobe parses metadata from the ffprobe JSON output and updates the provided
// ParseInfo with the extracted data. It handles audio and video tracks, extracting
// codec, resolution, and other relevant information. It also determines the priority
// of the media based on the provided QualityConfig.
func parseffprobe(m *database.ParseInfo, quality *config.QualityConfig, result *ffProbeJSON) error {
	if len(result.Streams) == 0 {
		return logger.ErrTracksEmpty
	}

	if result.Error.Code != 0 {
		return errors.New(
			"ffprobe error code " + strconv.Itoa(result.Error.Code) + " " + result.Error.String,
		)
	}
	if duration, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		m.Runtime = int(math.Round(duration))
	}

	var redetermineprio bool

	var n int
	for idx := range result.Streams {
		if result.Streams[idx].Tags.Language != "" &&
			(result.Streams[idx].CodecType == "audio" || strings.EqualFold(result.Streams[idx].CodecType, "audio")) {
			n++
		}
	}

	if n > 1 {
		m.Languages = make([]string, 0, n)
	}
	for idx := range result.Streams {
		stream := &result.Streams[idx]
		if isAudioStream(stream) {
			if stream.Tags.Language != "" {
				m.Languages = append(m.Languages, stream.Tags.Language)
			}
			if updateAudio(m, stream) {
				redetermineprio = true
			}
		} else if isVideoStream(stream) {
			if updateVideo(m, stream) {
				redetermineprio = true
			}
		}
	}
	if redetermineprio {
		updatePriority(m, quality)
	}
	return nil
}

// isAudioStream checks if the given stream is an audio stream by comparing its codec type.
// It returns true if the stream's codec type is "audio" (case-insensitive), false otherwise.
func isAudioStream(stream *struct {
	Tags struct {
		Language string `json:"language"`
	} `json:"tags"`
	CodecName      string `json:"codec_name"`
	CodecTagString string `json:"codec_tag_string"`
	CodecType      string `json:"codec_type"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
},
) bool {
	return stream.CodecType == "audio" || strings.EqualFold(stream.CodecType, "audio")
}

// isVideoStream checks if the given stream is a video stream by comparing its codec type.
// It returns true if the stream's codec type is "video" (case-insensitive), false otherwise.
func isVideoStream(stream *struct {
	Tags struct {
		Language string `json:"language"`
	} `json:"tags"`
	CodecName      string `json:"codec_name"`
	CodecTagString string `json:"codec_tag_string"`
	CodecType      string `json:"codec_type"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
},
) bool {
	return stream.CodecType == "video" || strings.EqualFold(stream.CodecType, "video")
}

// updateAudio updates the audio metadata in the ParseInfo struct based on the provided stream information.
// It updates the audio codec and sets the corresponding audio ID using the Gettypeids method.
// Returns true if the audio codec has changed, false otherwise.
func updateAudio(m *database.ParseInfo, stream *struct {
	Tags struct {
		Language string `json:"language"`
	} `json:"tags"`
	CodecName      string `json:"codec_name"`
	CodecTagString string `json:"codec_tag_string"`
	CodecType      string `json:"codec_type"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
},
) bool {
	if m.Audio == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.Audio)) {
		m.Audio = stream.CodecName
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
		return true
	}
	return false
}

// updateVideo updates the video metadata in the ParseInfo struct based on the provided stream information.
// It updates the video resolution, codec, and dimensions. If the codec or resolution changes,
// it updates the corresponding IDs using the Gettypeids method. Returns true if either the
// codec or resolution has changed, false otherwise.
func updateVideo(m *database.ParseInfo, stream *struct {
	Tags struct {
		Language string `json:"language"`
	} `json:"tags"`
	CodecName      string `json:"codec_name"`
	CodecTagString string `json:"codec_tag_string"`
	CodecType      string `json:"codec_type"`
	Height         int    `json:"height,omitempty"`
	Width          int    `json:"width,omitempty"`
},
) bool {
	m.Height = stream.Height
	m.Width = stream.Width

	var codecChanged bool

	// Handle special case for MPEG4/XVID
	if (stream.CodecName == "mpeg4" || strings.EqualFold(stream.CodecName, "mpeg4")) &&
		(stream.CodecTagString == "xvid" || strings.EqualFold(stream.CodecTagString, "xvid")) {
		if m.Codec == "" ||
			(stream.CodecTagString != "" && !strings.EqualFold(stream.CodecTagString, m.Codec)) {
			m.Codec = stream.CodecTagString
			codecChanged = true
		}
	} else if m.Codec == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.Codec)) {
		m.Codec = stream.CodecName
		codecChanged = true
	}

	if codecChanged {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	// Normalize dimensions
	if m.Height > m.Width {
		m.Height, m.Width = m.Width, m.Height
	}

	var resolutionChanged bool
	if getreso := m.Parseresolution(); getreso != "" &&
		(m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
		m.Resolution = getreso
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
		resolutionChanged = true
	}

	return codecChanged || resolutionChanged
}

// updatePriority determines the priority of a media file based on its resolution, quality, codec, and audio characteristics.
// It uses the provided QualityConfig to find the appropriate priority index and sets the Priority field accordingly.
// If no matching priority is found, the priority remains unchanged.
func updatePriority(m *database.ParseInfo, quality *config.QualityConfig) {
	if intid := Findpriorityidxwanted(m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, quality); intid != -1 {
		m.Priority = allQualityPrioritiesWantedT[intid].Priority
	}
}

// parsemediainfo parses media information from a mediaInfoJSON object and updates the
// provided ParseInfo with the extracted data. It handles audio and video tracks,
// extracting codec, resolution, and other relevant information. It also determines
// the priority of the media based on the provided QualityConfig.
func parsemediainfo(
	m *database.ParseInfo,
	quality *config.QualityConfig,
	info *mediaInfoJSON,
) error {
	if len(info.Media.Track) == 0 {
		return logger.ErrTracksEmpty
	}
	var redetermineprio bool
	var n int
	for idx := range info.Media.Track {
		if info.Media.Track[idx].Type == "Audio" && info.Media.Track[idx].Language != "" {
			n++
		}
	}
	if n > 1 {
		m.Languages = make([]string, 0, n)
	}
	for idx := range info.Media.Track {
		track := &info.Media.Track[idx]
		switch track.Type {
		case "Audio":
			if track.Language != "" {
				m.Languages = append(m.Languages, track.Language)
			}
			if updateAudioFromMediaInfo(m, track) {
				redetermineprio = true
			}
		case "video":
			if updateVideoFromMediaInfo(m, track) {
				redetermineprio = true
			}
		}
	}
	if redetermineprio {
		updatePriority(m, quality)
	}
	return nil
}

// updateAudioFromMediaInfo updates the ParseInfo with audio track details from MediaInfo.
// It handles audio codec and sets the corresponding audio ID.
// Returns true if the audio codec changes, false otherwise.
func updateAudioFromMediaInfo(m *database.ParseInfo, track *struct {
	Type     string `json:"@type"`
	Format   string `json:"Format"`
	Duration string `json:"Duration"`
	CodecID  string `json:"CodecID,omitempty"`
	Width    string `json:"Width,omitempty"`
	Height   string `json:"Height,omitempty"`
	Language string `json:"Language,omitempty"`
},
) bool {
	if m.Audio == "" || (track.Format != "" && !strings.EqualFold(track.CodecID, m.Audio)) {
		m.Audio = track.Format
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
		return true
	}
	return false
}

// updateVideoFromMediaInfo updates the ParseInfo with video track details from MediaInfo.
// It handles codec, resolution, height, width, and runtime information.
// Returns true if codec or resolution changes, false otherwise.
func updateVideoFromMediaInfo(m *database.ParseInfo, track *struct {
	Type     string `json:"@type"`
	Format   string `json:"Format"`
	Duration string `json:"Duration"`
	CodecID  string `json:"CodecID,omitempty"`
	Width    string `json:"Width,omitempty"`
	Height   string `json:"Height,omitempty"`
	Language string `json:"Language,omitempty"`
},
) bool {
	var codecChanged bool

	// Handle special case for MPEG4/XVID
	if (track.Format == "mpeg4" || strings.EqualFold(track.Format, "mpeg4")) &&
		(track.CodecID == "xvid" || strings.EqualFold(track.CodecID, "xvid")) {
		if m.Codec == "" || (track.CodecID != "" && !strings.EqualFold(track.CodecID, m.Codec)) {
			m.Codec = track.CodecID
			codecChanged = true
		}
	} else if m.Codec == "" || (track.Format != "" && !strings.EqualFold(track.Format, m.Codec)) {
		m.Codec = track.Format
		codecChanged = true
	}

	if codecChanged {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	m.Height = logger.StringToInt(track.Height)
	m.Width = logger.StringToInt(track.Width)
	m.Runtime = logger.StringToInt(splitByFull(track.Duration, '.'))

	// Normalize dimensions
	if m.Height > m.Width {
		m.Height, m.Width = m.Width, m.Height
	}

	var resolutionChanged bool
	if getreso := m.Parseresolution(); getreso != "" &&
		(m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
		m.Resolution = getreso
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
		resolutionChanged = true
	}

	return codecChanged || resolutionChanged
}

// GetPriorityMapQual calculates priority for a ParseInfo based on its resolution,
// quality, codec, and audio IDs. It looks up missing IDs, applies defaults if configured,
// and maps IDs to names. It then calls getIDPriority to calculate the priority value.
func GetPriorityMapQual(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	useall, checkwanted bool,
) {
	if m.ResolutionID == 0 {
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = m.Gettypeids(m.Quality, database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
	}

	if m.ResolutionID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetresolutionsIn, cfgp, true)
		if idx != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[idx].ID
		}
	}
	if m.QualityID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetqualitiesIn, cfgp, false)
		if idx != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[idx].ID
		}
	}

	updateNamesFromIDs(m)

	var reso, qual, aud, codec uint

	if quality.UseForPriorityResolution || useall {
		reso = m.ResolutionID
	}
	if quality.UseForPriorityQuality || useall {
		qual = m.QualityID
	}
	if quality.UseForPriorityAudio || useall {
		aud = m.AudioID
	}
	if quality.UseForPriorityCodec || useall {
		codec = m.CodecID
	}

	intid, cwanted := findPriorityIndex(reso, qual, codec, aud, quality, checkwanted)
	if intid == -1 {
		m.TempTitle = BuildPrioStr(reso, qual, codec, aud)
		logger.LogDynamicany2StrAny(
			"debug",
			"prio not found",
			"in",
			quality.Name,
			"searched for",
			&m.TempTitle,
		)
		m.Priority = 0
		return
	}

	if cwanted {
		m.Priority = allQualityPrioritiesWantedT[intid].Priority
	} else {
		m.Priority = allQualityPrioritiesT[intid].Priority
	}

	if quality.UseForPriorityOther || useall {
		applyPriorityModifiers(m)
	}
}

// updateNamesFromIDs populates the name fields of a ParseInfo struct based on its corresponding ID fields.
// It retrieves names for resolution, quality, audio, and codec by matching IDs with predefined database entries.
// If an ID is non-zero, it attempts to find and set the corresponding name from the respective database slice.
func updateNamesFromIDs(m *database.ParseInfo) {
	if m.ResolutionID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetresolutionsIn, 1); idx != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[idx].Name
		}
	}
	if m.QualityID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetqualitiesIn, 2); idx != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[idx].Name
		}
	}
	if m.AudioID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetaudiosIn, 3); idx != -1 {
			m.Audio = database.DBConnect.GetaudiosIn[idx].Name
		}
	}
	if m.CodecID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetcodecsIn, 4); idx != -1 {
			m.Codec = database.DBConnect.GetcodecsIn[idx].Name
		}
	}
}

// findPriorityIndex determines the priority index for a media file by first checking wanted priorities
// if checkwanted is true, and falling back to the default priority index if no wanted priority is found.
// It returns the index of the matching priority entry or -1 if no match is found.
func findPriorityIndex(
	reso, qual, codec, aud uint,
	quality *config.QualityConfig,
	checkwanted bool,
) (int, bool) {
	if checkwanted {
		if intid := Findpriorityidxwanted(reso, qual, codec, aud, quality); intid != -1 {
			return intid, true
		}
	}
	return Findpriorityidx(reso, qual, codec, aud, quality), false
}

// applyPriorityModifiers adjusts the priority of a parsed media file based on specific attributes.
// It increases the priority for proper releases, extended versions, and repacks.
// The priority is incremented by 5 for proper releases, 2 for extended versions, and 1 for repacks.
func applyPriorityModifiers(m *database.ParseInfo) {
	if m.Proper {
		m.Priority += 5
	}
	if m.Extended {
		m.Priority += 2
	}
	if m.Repack {
		m.Priority++
	}
}

// GetwantedArrPrio returns the priority value from the allQualityPrioritiesWantedT slice
// at the given index.
func GetwantedArrPrio(intid int) int {
	return allQualityPrioritiesWantedT[intid].Priority
}

// findpriorityidxwanted searches through the allQualityPrioritiesWantedT slice
// to find the index of the priority entry matching the given resolution,
// quality, codec, and audio IDs. It is used to look up the priority value
// for a video file's metadata when checking if it matches a wanted quality.
func Findpriorityidxwanted(reso, qual, codec, aud uint, quality *config.QualityConfig) int {
	for idx := range allQualityPrioritiesWantedT {
		entry := &allQualityPrioritiesWantedT[idx]
		if entry.ResolutionID == reso &&
			entry.QualityID == qual &&
			entry.CodecID == codec &&
			entry.AudioID == aud &&
			strings.EqualFold(entry.QualityGroup, quality.Name) {
			return idx
		}
	}
	return -1
}

// findpriorityidx searches through the allQualityPrioritiesT slice
// to find the index of the priority entry matching the given resolution,
// quality, codec, and audio IDs. It is used internally to look up the
// priority value for a video file's metadata.
func Findpriorityidx(reso, qual, codec, aud uint, quality *config.QualityConfig) int {
	for idx := range allQualityPrioritiesT {
		entry := &allQualityPrioritiesT[idx]
		if entry.ResolutionID == reso &&
			entry.QualityID == qual &&
			entry.CodecID == codec &&
			entry.AudioID == aud &&
			strings.EqualFold(entry.QualityGroup, quality.Name) {
			return idx
		}
	}
	return -1
}

// GetAllQualityPriorities generates all possible quality priority combinations
// by iterating through resolutions, qualities, codecs and audios. It builds up
// a target Prioarr struct containing the ID and name for each, and calculates
// the priority value based on the quality group's reorder rules. The results
// are added to allQualityPrioritiesT and allQualityPrioritiesWantedT slices.
func GenerateAllQualityPriorities() {
	regex0 := database.Qualities{Name: "", ID: 0, Priority: 0}
	getresolutions := database.DBConnect.GetresolutionsIn
	getresolutions = append(getresolutions, regex0)

	getqualities := database.DBConnect.GetqualitiesIn
	getqualities = append(getqualities, regex0)

	getaudios := database.DBConnect.GetaudiosIn
	getaudios = append(getaudios, regex0)

	getcodecs := database.DBConnect.GetcodecsIn
	getcodecs = append(getcodecs, regex0)

	totalCombinations := config.GetSettingsQualityLen() * len(
		getresolutions,
	) * len(
		getqualities,
	) * len(
		getaudios,
	) * len(
		getcodecs,
	)
	allQualityPrioritiesT = make(
		[]Prioarr,
		0,
		totalCombinations,
	)
	allQualityPrioritiesWantedT = make(
		[]Prioarr,
		0,
		totalCombinations,
	)
	// var addwanted bool

	config.RangeSettingsQuality(func(_ string, qual *config.QualityConfig) {
		target := Prioarr{QualityGroup: qual.Name}

		for idxreso := range getresolutions {
			target.ResolutionID = getresolutions[idxreso].ID
			prioreso := getresolutions[idxreso].Gettypeidprioritysingle("resolution", qual)
			prioresoorg := prioreso

			for idxqual := range getqualities {
				target.QualityID = getqualities[idxqual].ID
				prioqual := getqualities[idxqual].Gettypeidprioritysingle("quality", qual)
				prioqualorg := prioqual

				for idxcodec := range getcodecs {
					target.CodecID = getcodecs[idxcodec].ID
					priocod := getcodecs[idxcodec].Gettypeidprioritysingle("codec", qual)

					for idxaudio := range getaudios {
						target.AudioID = getaudios[idxaudio].ID
						prioaud := getaudios[idxaudio].Gettypeidprioritysingle("audio", qual)

						// Handle combined resolution/quality reordering
						prioreso, prioqual = handleCombinedReorder(
							qual,
							getresolutions[idxreso].Name,
							getqualities[idxqual].Name,
							prioresoorg,
							prioqualorg,
						)

						target.Priority = prioreso + prioqual + priocod + prioaud

						// Reset for next iteration
						prioreso = prioresoorg
						prioqual = prioqualorg

						allQualityPrioritiesT = append(allQualityPrioritiesT, target)

						// Check if this combination is wanted
						if isWantedCombination(
							qual,
							getresolutions[idxreso].Name,
							getqualities[idxqual].Name,
						) {
							allQualityPrioritiesWantedT = append(
								allQualityPrioritiesWantedT,
								target,
							)
						}
					}
				}
			}
		}
	})
}

// handleCombinedReorder processes quality reordering for combined resolution and quality configurations.
// It checks if a specific resolution and quality combination matches a reordering rule and returns
// adjusted priority values. If no matching rule is found, it returns the original priority values.
// The function supports case-insensitive matching and handles combined resolution-quality reordering.
func handleCombinedReorder(
	qual *config.QualityConfig,
	resolutionName, qualityName string,
	prioresoorg, prioqualorg int,
) (int, int) {
	for idx := range qual.QualityReorder {
		reorder := &qual.QualityReorder[idx]
		if reorder.ReorderType != "combined_res_qual" &&
			!strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
			continue
		}

		if !strings.ContainsRune(reorder.Name, ',') {
			continue
		}

		commaIdx := strings.IndexRune(reorder.Name, ',')
		if commaIdx == -1 {
			continue
		}

		reorderRes := reorder.Name[:commaIdx]
		reorderQual := reorder.Name[commaIdx+1:]

		if (reorderRes == resolutionName || strings.EqualFold(reorderRes, resolutionName)) &&
			(reorderQual == qualityName || strings.EqualFold(reorderQual, qualityName)) {
			return reorder.Newpriority, 0
		}
	}
	return prioresoorg, prioqualorg
}

// isWantedCombination checks if a specific resolution and quality combination is desired
// based on the quality configuration. When debug logging is enabled, it logs details
// about unwanted resolutions or qualities. Returns true if both resolution and quality
// are wanted, false otherwise.
func isWantedCombination(qual *config.QualityConfig, resolutionName, qualityName string) bool {
	if database.DBLogLevel != logger.StrDebug {
		return true
	}

	resWanted := logger.SlicesContainsI(qual.WantedResolution, resolutionName)
	qualWanted := logger.SlicesContainsI(qual.WantedQuality, qualityName)

	if !resWanted {
		logger.LogDynamicany2Str(
			"debug",
			"unwanted res",
			logger.StrQuality,
			qual.Name,
			"Resolution Parse",
			resolutionName,
		)
	}
	if !qualWanted {
		logger.LogDynamicany2Str(
			"debug",
			"unwanted qual",
			logger.StrQuality,
			qual.Name,
			"Quality Parse",
			qualityName,
		)
	}

	return resWanted && qualWanted
}

// buildPrioStr builds a priority string from the given resolution, quality, codec, and audio values.
// The priority string is in the format "r_q_c_a" where r is the resolution, q is the quality, c is the codec,
// and a is the audio value. This allows easy comparison of release priority.
func BuildPrioStr(r, q, c, a uint) string {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	bld.WriteUInt(r)
	bld.WriteByte('_')
	bld.WriteUInt(q)
	bld.WriteByte('_')
	bld.WriteUInt(c)
	bld.WriteByte('_')
	bld.WriteUInt(a)
	return bld.String()
}
