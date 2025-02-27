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

	scanpatterns       []regexpattern
	globalscanpatterns = []regexpattern{
		{name: "season", last: false, re: `(?i)(s?(\d{1,4}))(?: )?[ex]`, getgroup: 2},
		{name: "episode", last: false, re: `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`, getgroup: 2},
		{name: "identifier", last: false, re: `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, getgroup: 2},
		{name: logger.StrDate, last: false, re: `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, getgroup: 2},
		{name: "year", last: true, re: `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, getgroup: 2},
		{name: "audio", last: false, re: `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`, getgroup: 2},
		{name: "imdb", last: false, re: `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, getgroup: 2},
		{name: "tvdb", last: false, re: `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, getgroup: 2},
	}
)

// getmatchesroot finds all substring matches in m.Str using the provided regular
// expression pattern. It returns a slice of integer indices indicating the start
// and end positions of the matched substring(s). For regex capture groups, the
// even indices are start positions and odd indices are end positions.
func (pattern *regexpattern) getmatchesroot(m *database.ParseInfo, cfgp *config.MediaTypeConfig) (int, int) {
	matchest := database.RunRetRegex(pattern.re, m.Str, (pattern.last && pattern.name == "year" && cfgp.Useseries))
	if len(matchest) == 0 {
		return -1, -1
	}

	lensubmatches := len(matchest)
	matchdata := false
	switch {
	case lensubmatches >= 4 && ((lensubmatches/2)-1) >= pattern.getgroup && matchest[3] != -1 && matchest[(pattern.getgroup*2)+1] != -1:
		matchdata = true
	case lensubmatches <= 2 && ((lensubmatches*2)-1) >= pattern.getgroup && matchest[(pattern.getgroup*2)+1] != -1:
		matchdata = true
	case (lensubmatches*2) >= 4 && matchest[3] != -1:
		return -1, -1
	}
	if !matchdata {
		return -1, -1
	}
	return matchest[pattern.getgroup*2], matchest[(pattern.getgroup*2)+1]
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
	n := 8
	for _, val := range database.DBConnect.GetaudiosIn {
		if val.UseRegex {
			n++
		}
	}
	for _, val := range database.DBConnect.GetcodecsIn {
		if val.UseRegex {
			n++
		}
	}
	for _, val := range database.DBConnect.GetqualitiesIn {
		if val.UseRegex {
			n++
		}
	}
	for _, val := range database.DBConnect.GetresolutionsIn {
		if val.UseRegex {
			n++
		}
	}
	scanpatterns = make([]regexpattern, 0, n)
	scanpatterns = append(scanpatterns, globalscanpatterns...)

	for _, val := range scanpatterns {
		database.SetStaticRegexp(val.re)
	}
	for _, val := range database.DBConnect.GetaudiosIn {
		if val.UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: val.Regex, getgroup: val.Regexgroup})
			database.SetStaticRegexp(val.Regex)
		}
	}
	for _, val := range database.DBConnect.GetresolutionsIn {
		if val.UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: val.Regex, getgroup: val.Regexgroup})
			database.SetStaticRegexp(val.Regex)
		}
	}
	for _, val := range database.DBConnect.GetqualitiesIn {
		if val.UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: val.Regex, getgroup: val.Regexgroup})
			database.SetStaticRegexp(val.Regex)
		}
	}
	for _, val := range database.DBConnect.GetcodecsIn {
		if val.UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: val.Regex, getgroup: val.Regexgroup})
			database.SetStaticRegexp(val.Regex)
		}
	}
}

// GenerateCutoffPriorities iterates through the media type and list
// configurations, and sets the CutoffPriority field for any list that
// does not already have it set. It calls NewCutoffPrio to calculate
// the priority value based on the cutoff quality and resolution.
func GenerateCutoffPriorities() {
	m := new(database.ParseInfo)
	for _, media := range config.SettingsMedia {
		for _, lst := range media.ListsMap {
			if lst.CfgQuality.CutoffPriority != 0 {
				continue
			}
			m = &database.ParseInfo{}
			m.Quality = lst.CfgQuality.CutoffQuality
			m.Resolution = lst.CfgQuality.CutoffResolution
			GetPriorityMapQual(m, media, lst.CfgQuality, true, false) // newCutoffPrio(media, idxi)
			lst.CfgQuality.CutoffPriority = m.Priority
		}
	}
}

// newFileParserP reuses a FileParser instance. It sets the filename,
// media config, list ID, and allow title search flag. It runs the main parsing
// logic like splitting the filename on delimiters, running regex matches,
// and cleaning up the parsed title and identifier.
func newFileParserP(cleanName string, onlyifempty bool, cfgp *config.MediaTypeConfig, listid int, m *database.ParseInfo) {
	m.ListID = listid
	if !onlyifempty || m.File == "" {
		m.File = cleanName
	}
	m.Str = m.File
	logger.StringReplaceWithP(&m.Str, '_', ' ')
	m.Str = logger.Trim(m.Str, '[', ']')

	if !config.SettingsGeneral.DisableParserStringMatch {
		m.Parsegroup("audio", onlyifempty)
		m.Parsegroup("codec", onlyifempty)
		m.Parsegroup("quality", onlyifempty)
		m.Parsegroup("resolution", onlyifempty)
	}

	m.Parsegroup("extended", onlyifempty)
	m.ParsegroupEntry("proper")
	m.ParsegroupEntry("repack")

	var (
		start, strStart, strEnd, index int
		end                            = len(m.Str)
		conttt                         = logger.ContainsI(m.Str, logger.StrTt)
		conttvdb                       = logger.ContainsI(m.Str, logger.StrTvdb)
	)

	for idx := range scanpatterns {
		switch scanpatterns[idx].name {
		case "imdb":
			if cfgp.Useseries || !conttt {
				continue
			}
			if onlyifempty && m.Imdb != "" {
				continue
			}
		case "tvdb":
			if !cfgp.Useseries || !conttvdb {
				continue
			}
			if onlyifempty && m.Tvdb != "" {
				continue
			}
		case "season":
			if !cfgp.Useseries {
				continue
			}
			if onlyifempty && m.Season != 0 {
				continue
			}
		case "identifier":
			if !cfgp.Useseries {
				continue
			}
			if onlyifempty && m.Identifier != "" {
				continue
			}
		case "episode":
			if !cfgp.Useseries {
				continue
			}
			if onlyifempty && m.Episode != 0 {
				continue
			}
		case "date":
			if !cfgp.Useseries {
				continue
			}
			if onlyifempty && m.Date != "" {
				continue
			}
		case "audio":
			if m.Audio != "" {
				continue
			}
		case "codec":
			if m.Codec != "" {
				continue
			}
		case "quality":
			if m.Quality != "" {
				continue
			}
		case "resolution":
			if m.Resolution != "" {
				continue
			}
		}

		strStart, strEnd = scanpatterns[idx].getmatchesroot(m, cfgp)
		if strStart == -1 || strEnd == -1 {
			continue
		}

		if !cfgp.Useseries || (cfgp.Useseries && scanpatterns[idx].name != "year") {
			index = strings.Index(m.Str, m.Str[strStart:strEnd])
			if index == 0 {
				if len(m.Str[strStart:strEnd]) != len(m.Str) && len(m.Str[strStart:strEnd]) < end {
					start = len(m.Str[strStart:strEnd])
				}
			} else if index < end && index > start {
				end = index
			}
		}

		switch scanpatterns[idx].name {
		case "imdb", "tvdb", "year", "season", "episode", "identifier", "date", "audio", "resolution", "quality", "codec":
		default:
			continue
		}
		if m.FirstIDX == 0 || strStart < m.FirstIDX {
			m.FirstIDX = strStart
		}
		switch scanpatterns[idx].name {
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
	if end < start {
		logger.Logtype("debug", 0).Str(logger.StrPath, m.File).Int("start", start).Int("end", end).Msg("EndIndex < startindex")
		if strings.ContainsRune(m.File[start:], '(') {
			m.Str = splitByFullP(m.File[start:], '(')
		} else {
			m.Str = m.File[start:]
		}
	} else {
		if strings.ContainsRune(m.File[start:end], '(') {
			m.Str = splitByFullP(m.File[start:end], '(')
		} else {
			m.Str = m.File[start:end]
		}
	}

	if onlyifempty && m.Title != "" {
		return
	}
	m.Title = m.Str
	if strings.ContainsRune(m.Str, '.') && !strings.ContainsRune(m.Str, ' ') {
		logger.StringReplaceWithP(&m.Title, '.', ' ')
	}
	m.Title = logger.TrimSpace(logger.TrimRight(logger.TrimSpace(m.Title), '-', '.', ' '))
}

// SplitByFullP splits a string into two parts by the first
// occurrence of the split rune. It returns the part before the split.
// If the split rune is not found, it returns the original string.
func splitByFullP(str string, splitby rune) string {
	idx := strings.IndexRune(str, splitby)
	if idx != -1 {
		return str[:idx]
	}
	return str
}

// ParseFile parses the given video file to extract metadata.
// It accepts a video file path, booleans to indicate whether to use the path and folder
// to extract metadata, a media type config, a list ID, and a FileParser to populate.
// It calls ParseFileP to parse the file and populate the FileParser, which is then returned.
func ParseFile(videofile string, usepath, usefolder bool, cfgp *config.MediaTypeConfig, listid int) *database.ParseInfo {
	m := database.PLParseInfo.Get()
	ParseFileP(videofile, usepath, usefolder, cfgp, listid, m)
	return m
}

// ParseFileP parses a video file to extract metadata.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing parser to populate.
// It returns the populated parser after attempting to extract metadata.
func ParseFileP(videofile string, usepath, usefolder bool, cfgp *config.MediaTypeConfig, listid int, m *database.ParseInfo) {
	if usepath {
		newFileParserP(filepath.Base(videofile), false, cfgp, listid, m)
	} else {
		newFileParserP(videofile, false, cfgp, listid, m)
	}

	if m.Quality != "" && m.Resolution != "" {
		return
	}
	if !usefolder || !usepath {
		return
	}
	newFileParserP(filepath.Base(filepath.Dir(videofile)), true, cfgp, listid, m)
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
		if m.Imdb != "" {
			if !logger.HasPrefixI(m.Imdb, logger.StrTt) {
				sourceimdb := m.Imdb
				m.MovieFindDBIDByImdbParser()
				if m.DbmovieID == 0 && len(sourceimdb) < 7 {
					m.Imdb = ("0" + sourceimdb)
					m.MovieFindDBIDByImdbParser()
					if m.DbmovieID == 0 && len(sourceimdb) < 6 {
						m.Imdb = ("00" + sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
					if m.DbmovieID == 0 && len(sourceimdb) < 5 {
						m.Imdb = ("000" + sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
					if m.DbmovieID == 0 && len(sourceimdb) < 4 {
						m.Imdb = ("0000" + sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
				}
			} else {
				m.MovieFindDBIDByImdbParser()
			}
		}
		if m.DbmovieID == 0 && m.Title != "" && allowsearchtitle && cfgp.Name != "" {
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
		if m.DbmovieID != 0 && m.ListID != -1 {
			database.Scanrows2dyn(false, database.QueryMoviesGetIDByDBIDListname, &m.MovieID, &m.DbmovieID, &cfgp.Lists[m.ListID].Name)
		}

		if m.DbmovieID != 0 && m.MovieID == 0 && cfgp.Name != "" && m.ListID == -1 {
			for idx := range cfgp.Lists {
				if config.SettingsGeneral.UseMediaCache {
					m.MovieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheMovie, m.DbmovieID, cfgp.Lists[idx].Name)
				} else {
					database.Scanrows2dyn(false, "select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", &m.MovieID, &cfgp.Lists[idx].Name, &m.DbmovieID)
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
	if m.Tvdb != "" {
		database.Scanrows1dyn(false, database.QueryDbseriesGetIDByTvdb, &m.DbserieID, &m.Tvdb)
	}
	if m.DbserieID == 0 && m.Title != "" && (allowsearchtitle || m.Tvdb == "") {
		if m.Year != 0 {
			m.FindDbserieByNameWithSlug(logger.JoinStrings(m.Title, " (", logger.IntToString(m.Year), ")"))
		}
		if m.DbserieID == 0 {
			m.FindDbserieByNameWithSlug(m.Title)
		}
	}
	if m.DbserieID == 0 && m.File != "" {
		m.RegexGetMatchesStr1(cfgp)
	}

	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}
	m.SetDBEpisodeIDfromM()
	if m.DbserieEpisodeID == 0 {
		return errNotFoundDBEpisode
	}
	if m.ListID != -1 {
		database.Scanrows2dyn(false, database.QuerySeriesGetIDByDBIDListname, &m.SerieID, &m.DbserieID, &cfgp.Lists[m.ListID].Name)
	}
	if m.SerieID == 0 && cfgp != nil && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.SettingsGeneral.UseMediaCache {
				m.SerieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheSeries, m.DbserieID, cfgp.Lists[idx].Name)
			} else {
				database.Scanrows2dyn(false, database.QuerySeriesGetIDByDBIDListname, &m.SerieID, &m.DbserieID, &cfgp.Lists[idx].Name)
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

	err := parsemedia(!config.SettingsGeneral.UseMediainfo, m, quality)
	if err == nil {
		return nil
	}
	if !config.SettingsGeneral.UseMediaFallback {
		return err
	}
	return parsemedia(config.SettingsGeneral.UseMediainfo, m, quality)
}

// parsemedia attempts to parse the metadata of a video file using either ffprobe or MediaInfo.
// If ffprobe is enabled, it first tries to parse the file using ffprobe. If that fails or ffprobe is
// not enabled, it falls back to using MediaInfo to parse the file.
// It takes a boolean indicating whether to use ffprobe, a pointer to a ParseInfo struct to populate
// with the parsed metadata, and a pointer to a QualityConfig struct.
// Returns an error if both parsing methods fail.
func parsemedia(ffprobe bool, m *database.ParseInfo, quality *config.QualityConfig) error {
	if ffprobe {
		if m.File == "" {
			return logger.ErrNotFound
		}

		if ExecCmdJson[ffProbeJSON](m.File, "ffprobe", m, quality) == nil {
			return nil
		}
	}
	if m.File == "" {
		return logger.ErrNotFound
	}
	return ExecCmdJson[mediaInfoJSON](m.File, "mediainfo", m, quality)
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
		return errors.New("ffprobe error code " + strconv.Itoa(result.Error.Code) + " " + result.Error.String)
	}
	duration, err := strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.Runtime = int(math.Round(duration))
	}

	var redetermineprio bool

	var n int
	for idx := range result.Streams {
		if result.Streams[idx].Tags.Language != "" && (result.Streams[idx].CodecType == "audio" || strings.EqualFold(result.Streams[idx].CodecType, "audio")) {
			n++
		}
	}

	if n > 1 {
		m.Languages = make([]string, 0, n)
	}
	for idx := range result.Streams {
		if result.Streams[idx].CodecType == "audio" || strings.EqualFold(result.Streams[idx].CodecType, "audio") {
			if result.Streams[idx].Tags.Language != "" {
				m.Languages = append(m.Languages, result.Streams[idx].Tags.Language)
			}
			if m.Audio == "" || (result.Streams[idx].CodecName != "" && !strings.EqualFold(result.Streams[idx].CodecName, m.Audio)) {
				m.Audio = result.Streams[idx].CodecName
				m.AudioID = m.Gettypeids(3)
				redetermineprio = true
			}
			continue
		}
		if result.Streams[idx].CodecType != "video" {
			if !strings.EqualFold(result.Streams[idx].CodecType, "video") {
				continue
			}
		}
		m.Height = result.Streams[idx].Height
		m.Width = result.Streams[idx].Width

		if (result.Streams[idx].CodecName == "mpeg4" || strings.EqualFold(result.Streams[idx].CodecName, "mpeg4")) && (result.Streams[idx].CodecTagString == "xvid" || strings.EqualFold(result.Streams[idx].CodecTagString, "xvid")) {
			if m.Codec == "" || (result.Streams[idx].CodecTagString != "" && !strings.EqualFold(result.Streams[idx].CodecTagString, m.Codec)) {
				m.Codec = result.Streams[idx].CodecTagString
				m.CodecID = m.Gettypeids(4)
				redetermineprio = true
			}
		} else if m.Codec == "" || (result.Streams[idx].CodecName != "" && !strings.EqualFold(result.Streams[idx].CodecName, m.Codec)) {
			m.Codec = result.Streams[idx].CodecName
			m.CodecID = m.Gettypeids(4)
			redetermineprio = true
		}
		if m.Height > m.Width {
			m.Height, m.Width = m.Width, m.Height
		}
		getreso := m.Parseresolution()

		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = m.Gettypeids(1)
			redetermineprio = true
		}
	}
	if redetermineprio {
		intid := Findpriorityidxwanted(m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}

// parsemediainfo parses media information from a mediaInfoJSON object and updates the
// provided ParseInfo with the extracted data. It handles audio and video tracks,
// extracting codec, resolution, and other relevant information. It also determines
// the priority of the media based on the provided QualityConfig.
func parsemediainfo(m *database.ParseInfo, quality *config.QualityConfig, info *mediaInfoJSON) error {
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
		if info.Media.Track[idx].Type == "Audio" {
			if info.Media.Track[idx].Language != "" {
				m.Languages = append(m.Languages, info.Media.Track[idx].Language)
			}
			if m.Audio == "" || (info.Media.Track[idx].Format != "" && !strings.EqualFold(info.Media.Track[idx].CodecID, m.Audio)) {
				m.Audio = info.Media.Track[idx].Format
				m.AudioID = m.Gettypeids(3)
				redetermineprio = true
			}
			continue
		}
		if info.Media.Track[idx].Type != "video" {
			continue
		}

		if (info.Media.Track[idx].Format == "mpeg4" || strings.EqualFold(info.Media.Track[idx].Format, "mpeg4")) && (info.Media.Track[idx].CodecID == "xvid" || strings.EqualFold(info.Media.Track[idx].CodecID, "xvid")) {
			if m.Codec == "" || (info.Media.Track[idx].CodecID != "" && !strings.EqualFold(info.Media.Track[idx].CodecID, m.Codec)) {
				m.Codec = info.Media.Track[idx].CodecID
				m.CodecID = m.Gettypeids(4)
				redetermineprio = true
			}
		} else if m.Codec == "" || (info.Media.Track[idx].Format != "" && !strings.EqualFold(info.Media.Track[idx].Format, m.Codec)) {
			m.Codec = info.Media.Track[idx].Format
			m.CodecID = m.Gettypeids(4)
			redetermineprio = true
		}
		m.Height = logger.StringToInt(info.Media.Track[idx].Height)
		m.Width = logger.StringToInt(info.Media.Track[idx].Width)
		m.Runtime = logger.StringToInt(splitByFullP(info.Media.Track[idx].Duration, '.'))

		if m.Height > m.Width {
			m.Height, m.Width = m.Width, m.Height
		}
		getreso := m.Parseresolution()

		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = m.Gettypeids(1)
			redetermineprio = true
		}
	}
	if redetermineprio {
		intid := Findpriorityidxwanted(m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	return nil
}

// GetPriorityMapQual calculates priority for a ParseInfo based on its resolution,
// quality, codec, and audio IDs. It looks up missing IDs, applies defaults if configured,
// and maps IDs to names. It then calls getIDPriority to calculate the priority value.
func GetPriorityMapQual(m *database.ParseInfo, cfgp *config.MediaTypeConfig, quality *config.QualityConfig, useall, checkwanted bool) {
	if m.ResolutionID == 0 {
		m.ResolutionID = m.Gettypeids(1)
	}

	if m.QualityID == 0 {
		m.QualityID = m.Gettypeids(2)
	}

	if m.CodecID == 0 {
		m.CodecID = m.Gettypeids(4)
	}

	if m.AudioID == 0 {
		m.AudioID = m.Gettypeids(3)
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

	if m.ResolutionID != 0 {
		idx := m.Getqualityidxbyid(database.DBConnect.GetresolutionsIn, 1)
		if idx != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[idx].Name
		}
	}
	if m.QualityID != 0 {
		idx := m.Getqualityidxbyid(database.DBConnect.GetqualitiesIn, 2)
		if idx != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[idx].Name
		}
	}
	if m.AudioID != 0 {
		idx := m.Getqualityidxbyid(database.DBConnect.GetaudiosIn, 3)
		if idx != -1 {
			m.Audio = database.DBConnect.GetaudiosIn[idx].Name
		}
	}
	if m.CodecID != 0 {
		idx := m.Getqualityidxbyid(database.DBConnect.GetcodecsIn, 4)
		if idx != -1 {
			m.Codec = database.DBConnect.GetcodecsIn[idx].Name
		}
	}

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
	var intid int
	if checkwanted {
		intid = Findpriorityidxwanted(reso, qual, codec, aud, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		} else if m.Priority == 0 {
			intid = Findpriorityidx(reso, qual, codec, aud, quality)
			if intid != -1 {
				m.Priority = allQualityPrioritiesWantedT[intid].Priority
			}
		}
	} else {
		intid = Findpriorityidx(reso, qual, codec, aud, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}

	if intid == -1 {
		m.TempTitle = BuildPrioStr(reso, qual, codec, aud)
		logger.LogDynamicany2StrAny("debug", "prio not found", "in", quality.Name, "searched for", &m.TempTitle)
		m.Priority = 0
		return
	}
	if !quality.UseForPriorityOther && !useall {
		return
	}
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
		if allQualityPrioritiesWantedT[idx].ResolutionID == reso && allQualityPrioritiesWantedT[idx].QualityID == qual && allQualityPrioritiesWantedT[idx].CodecID == codec && allQualityPrioritiesWantedT[idx].AudioID == aud && strings.EqualFold(allQualityPrioritiesWantedT[idx].QualityGroup, quality.Name) {
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
		if allQualityPrioritiesT[idx].ResolutionID == reso && allQualityPrioritiesT[idx].QualityID == qual && allQualityPrioritiesT[idx].CodecID == codec && allQualityPrioritiesT[idx].AudioID == aud && strings.EqualFold(allQualityPrioritiesT[idx].QualityGroup, quality.Name) {
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
	getresolutions := append(database.DBConnect.GetresolutionsIn, regex0)

	getqualities := append(database.DBConnect.GetqualitiesIn, regex0)

	getaudios := append(database.DBConnect.GetaudiosIn, regex0)

	getcodecs := append(database.DBConnect.GetcodecsIn, regex0)

	allQualityPrioritiesT = make([]Prioarr, 0, len(config.SettingsQuality)*len(getresolutions)*len(getqualities)*len(getaudios)*len(getcodecs))
	allQualityPrioritiesWantedT = make([]Prioarr, 0, len(config.SettingsQuality)*len(getresolutions)*len(getqualities)*len(getaudios)*len(getcodecs))
	var addwanted bool

	for _, qual := range config.SettingsQuality {
		target := Prioarr{QualityGroup: qual.Name}
		for idxreso := range getresolutions {
			addwanted = true
			if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(qual.WantedResolution, getresolutions[idxreso].Name) {
				logger.LogDynamicany2Str("debug", "unwanted res", logger.StrQuality, qual.Name, "Resolution Parse", getresolutions[idxreso].Name)
				addwanted = false
			}
			target.ResolutionID = getresolutions[idxreso].ID
			prioreso := getresolutions[idxreso].Gettypeidprioritysingle("resolution", qual)
			prioresoorg := prioreso
			for idxqual := range getqualities {
				if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(qual.WantedQuality, getqualities[idxqual].Name) {
					logger.LogDynamicany2Str("debug", "unwanted qual", logger.StrQuality, qual.Name, "Quality Parse", getqualities[idxqual].Name)
					addwanted = false
				}

				target.QualityID = getqualities[idxqual].ID
				prioqual := getqualities[idxqual].Gettypeidprioritysingle("quality", qual)
				prioqualorg := prioqual
				for idxcodec := range getcodecs {
					target.CodecID = getcodecs[idxcodec].ID
					priocod := getcodecs[idxcodec].Gettypeidprioritysingle("codec", qual)
					for idxaudio := range getaudios {
						prioaud := getaudios[idxaudio].Gettypeidprioritysingle("audio", qual)

						target.AudioID = getaudios[idxaudio].ID
						for idxreorder := range qual.QualityReorder {
							if qual.QualityReorder[idxreorder].ReorderType != "combined_res_qual" {
								if !strings.EqualFold(qual.QualityReorder[idxreorder].ReorderType, "combined_res_qual") {
									continue
								}
							}
							if strings.ContainsRune(qual.QualityReorder[idxreorder].Name, ',') {
								continue
							}
							idxcomma := strings.IndexRune(qual.QualityReorder[idxreorder].Name, ',')

							if (qual.QualityReorder[idxreorder].Name[:idxcomma] == getresolutions[idxreso].Name || strings.EqualFold(qual.QualityReorder[idxreorder].Name[:idxcomma], getresolutions[idxreso].Name)) && (qual.QualityReorder[idxreorder].Name[idxcomma+1:] == getqualities[idxqual].Name || strings.EqualFold(qual.QualityReorder[idxreorder].Name[idxcomma+1:], getqualities[idxqual].Name)) {
								prioreso = qual.QualityReorder[idxreorder].Newpriority
								prioqual = 0
							}
						}
						target.Priority = prioreso + prioqual + priocod + prioaud
						prioreso = prioresoorg
						prioqual = prioqualorg
						allQualityPrioritiesT = append(allQualityPrioritiesT, target)
						if addwanted {
							allQualityPrioritiesWantedT = append(allQualityPrioritiesWantedT, target)
						}
					}
				}
			}
		}
	}
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
