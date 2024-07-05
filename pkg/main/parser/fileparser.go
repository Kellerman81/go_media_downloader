// parser
package parser

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

type regexpattern struct {
	name string
	// Use the last matching pattern. E.g. Year.
	last bool
	// REs need to have 2 sub expressions (groups), the first one is "raw", and
	// the second one for the "clean" value.
	// E.g. Epiode matching on "S01E18" will result in: raw = "E18", clean = "18".
	re       string
	getgroup int
}

type Prioarr struct {
	QualityGroup string
	ResolutionID uint
	QualityID    uint
	CodecID      uint
	AudioID      uint
	Priority     int
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
		{"season", false, `(?i)(s?(\d{1,4}))(?: )?[ex]`, 2},
		{"episode", false, `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`, 2},
		{"identifier", false, `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
		{logger.StrDate, false, `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
		{"year", true, `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, 2},
		{"audio", false, `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`, 2},
		{"imdb", false, `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, 2},
		{"tvdb", false, `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, 2},
	}
)

// getmatchesroot finds all substring matches in m.Str using the provided regular
// expression pattern. It returns a slice of integer indices indicating the start
// and end positions of the matched substring(s). For regex capture groups, the
// even indices are start positions and odd indices are end positions.
func (pattern *regexpattern) getmatchesroot(m *database.ParseInfo, cfgp *config.MediaTypeConfig) []int {
	if pattern.last && pattern.name == "year" && cfgp.Useseries {
		matches := database.Getallsubmatchindex(pattern.re, m.Str)
		if !(len(matches) >= 1 && len(matches[len(matches)-1]) >= 1) {
			return nil
		}
		return matches[len(matches)-1]
	}
	return database.Getfirstsubmatchindex(pattern.re, m.Str)
}

// getmatches extracts metadata from the file path or contents
// by matching regular expressions. It accepts a regexpattern struct,
// FileParser, and indices to track the substring match location.
// Returns bool indicating if matchdata was found.
func (pattern *regexpattern) getmatches(m *database.ParseInfo, cfgp *config.MediaTypeConfig, start *int, end *int, leng int) bool {
	matchest := pattern.getmatchesroot(m, cfgp)
	//defer clear(matchest)
	lensubmatches := len(matchest)
	if lensubmatches == 0 {
		return true
	}

	var matchdata bool
	if lensubmatches >= 4 && ((lensubmatches/2)-1) >= pattern.getgroup && matchest[3] != -1 && matchest[(pattern.getgroup*2)+1] != -1 {
		if !cfgp.Useseries || (cfgp.Useseries && pattern.name != "year") {
			index := strings.Index(m.Str, m.Str[matchest[2]:matchest[3]])
			if index == 0 {
				if len(m.Str[matchest[2]:matchest[3]]) != leng && len(m.Str[matchest[2]:matchest[3]]) < *end {
					*start = len(m.Str[matchest[2]:matchest[3]])
				}
			} else if index < *end && index > *start {
				*end = index
			}
		}
		matchdata = true
	} else if lensubmatches <= 2 && ((lensubmatches*2)-1) >= pattern.getgroup && matchest[(pattern.getgroup*2)+1] != -1 {
		matchdata = true
	} else if (lensubmatches*2) >= 4 && matchest[3] != -1 {
		return true
	}
	if !matchdata {
		return false
	}
	matchgroup := m.Str[matchest[pattern.getgroup*2]:matchest[(pattern.getgroup*2)+1]]
	switch pattern.name {
	case logger.StrImdb:
		m.Imdb = matchgroup
	case "tvdb":
		m.Tvdb, _ = strings.CutPrefix(matchgroup, logger.StrTvdb)
		if logger.HasPrefixI(matchgroup, logger.StrTvdb) {
			m.Tvdb = matchgroup[4:]
		}
	case "year":
		m.Year = logger.StringToUInt16(matchgroup)
	case "season":
		m.SeasonStr = matchgroup
		m.Season = logger.StringToInt(matchgroup)
	case "episode":
		m.EpisodeStr = matchgroup
		m.Episode = logger.StringToInt(matchgroup)
	case "identifier":
		m.Identifier = matchgroup
	case logger.StrDate:
		m.Date = matchgroup
	case "audio":
		m.Audio = matchgroup
	case "resolution":
		m.Resolution = matchgroup
	case "quality":
		m.Quality = matchgroup
	case "codec":
		m.Codec = matchgroup
	}
	return false
}

// Getallprios returns all quality priorities in descending order of quality. This is a copy
func Getallprios() []Prioarr {
	return allQualityPrioritiesWantedT
}

// Getcompleteallprios returns all quality priorities in descending order of quality. This is useful for testing
func Getcompleteallprios() []Prioarr {
	return allQualityPrioritiesT
}

// LoadDBPatterns loads patterns from database if not already loaded.
func LoadDBPatterns() {
	if len(scanpatterns) >= 1 {
		return
	}
	n := 8
	for idx := range database.DBConnect.GetaudiosIn {
		if database.DBConnect.GetaudiosIn[idx].UseRegex {
			n++
		}
	}
	for idx := range database.DBConnect.GetcodecsIn {
		if database.DBConnect.GetcodecsIn[idx].UseRegex {
			n++
		}
	}
	for idx := range database.DBConnect.GetqualitiesIn {
		if database.DBConnect.GetqualitiesIn[idx].UseRegex {
			n++
		}
	}
	for idx := range database.DBConnect.GetresolutionsIn {
		if database.DBConnect.GetresolutionsIn[idx].UseRegex {
			n++
		}
	}
	scanpatterns = make([]regexpattern, 0, n)
	scanpatterns = append(scanpatterns, globalscanpatterns...)

	for idx := range scanpatterns {
		database.SetRegexp(scanpatterns[idx].re, 0)
	}
	for idx := range database.DBConnect.GetaudiosIn {
		if database.DBConnect.GetaudiosIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.GetaudiosIn[idx].Regex, getgroup: database.DBConnect.GetaudiosIn[idx].Regexgroup})
			database.SetRegexp(database.DBConnect.GetaudiosIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetresolutionsIn {
		if database.DBConnect.GetresolutionsIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.GetresolutionsIn[idx].Regex, getgroup: database.DBConnect.GetresolutionsIn[idx].Regexgroup})
			database.SetRegexp(database.DBConnect.GetresolutionsIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetqualitiesIn {
		if database.DBConnect.GetqualitiesIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.GetqualitiesIn[idx].Regex, getgroup: database.DBConnect.GetqualitiesIn[idx].Regexgroup})
			database.SetRegexp(database.DBConnect.GetqualitiesIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetcodecsIn {
		if database.DBConnect.GetcodecsIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.GetcodecsIn[idx].Regex, getgroup: database.DBConnect.GetcodecsIn[idx].Regexgroup})
			database.SetRegexp(database.DBConnect.GetcodecsIn[idx].Regex, 0)
		}
	}
}

// GenerateCutoffPriorities iterates through the media type and list
// configurations, and sets the CutoffPriority field for any list that
// does not already have it set. It calls NewCutoffPrio to calculate
// the priority value based on the cutoff quality and resolution.
func GenerateCutoffPriorities() {
	for _, media := range config.SettingsMedia {
		for idxi := range media.Lists {
			if media.Lists[idxi].CfgQuality.CutoffPriority != 0 {
				continue
			}
			m := database.ParseInfo{Quality: media.Lists[idxi].CfgQuality.CutoffQuality, Resolution: media.Lists[idxi].CfgQuality.CutoffResolution}
			GetPriorityMapQual(&m, media, media.Lists[idxi].CfgQuality, true, false) //newCutoffPrio(media, idxi)
			media.Lists[idxi].CfgQuality.CutoffPriority = m.Priority
		}
	}
}

// NewFileParser creates a new FileParser instance with the given clean filename,
// media type config, list ID, and allow title search flag. It initializes the
// parser and returns it.
func NewFileParser(cleanName string, cfgp *config.MediaTypeConfig, onlyifempty bool, listid int8) *database.ParseInfo {
	//var m database.ParseInfo
	m := database.PLParseInfo.Get()
	newFileParserP(cleanName, onlyifempty, cfgp, listid, m)
	return m
}

// newFileParserP reuses a FileParser instance. It sets the filename,
// media config, list ID, and allow title search flag. It runs the main parsing
// logic like splitting the filename on delimiters, running regex matches,
// and cleaning up the parsed title and identifier.
func newFileParserP(cleanName string, onlyifempty bool, cfgp *config.MediaTypeConfig, listid int8, m *database.ParseInfo) {
	var orgstr string
	if m.Str != "" {
		orgstr = m.Str
	}
	m.Str = cleanName
	m.ListID = listid
	if !onlyifempty || m.File == "" {
		m.File = m.Str
	}
	if m.File != "" && (m.File[:1] == "[" || m.File[len(m.File)-1:] == "]") {
		m.Str = strings.TrimRight(strings.TrimLeft(m.File, "["), "]")
	} else if m.File != "" {
		m.Str = m.File
	}
	logger.StringReplaceWithP(&m.Str, '_', ' ')
	if !config.SettingsGeneral.DisableParserStringMatch {
		m.Parsegroup("audio", onlyifempty, database.DBConnect.AudioStrIn)
		m.Parsegroup("codec", onlyifempty, database.DBConnect.CodecStrIn)
		m.Parsegroup("quality", onlyifempty, database.DBConnect.QualityStrIn)
		m.Parsegroup("resolution", onlyifempty, database.DBConnect.ResolutionStrIn)
	}

	m.Parsegroup("extended", onlyifempty, []string{"extended", "extended cut", "extended.cut", "extended-cut"})
	m.ParsegroupEntry("proper", "proper")
	m.ParsegroupEntry("repack", "repack")

	var start int
	end := len(m.Str)
	leng := end
	//se := startendindex{start: 0, end: len(m.Str), length: len(m.Str)}
	conttt := !logger.ContainsI(m.Str, logger.StrTt)
	conttvdb := !logger.ContainsI(m.Str, logger.StrTvdb)

	for idx := range scanpatterns {
		switch scanpatterns[idx].name {
		case logger.StrImdb:
			if cfgp.Useseries || conttt {
				continue
			}
			if onlyifempty && m.Imdb != "" {
				continue
			}
		case "tvdb":
			if !cfgp.Useseries || conttvdb {
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
		case logger.StrDate:
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

		if scanpatterns[idx].getmatches(m, cfgp, &start, &end, leng) {
			continue
		}
	}
	if cfgp.Useseries {
		if onlyifempty && m.Identifier != "" {

		} else {
			if m.Identifier == "" && m.SeasonStr != "" && m.EpisodeStr != "" {
				m.GenerateIdentifierString()
			}

			if m.Date != "" && m.Identifier == "" {
				m.Identifier = m.Date
			}
		}
	}
	if end < start {
		logger.LogDynamicany("debug", "EndIndex < startindex", &logger.StrPath, &m.File, "start", start, "end", end)
		m.Str = m.File[start:]
	} else {
		m.Str = m.File[start:end]
	}
	if strings.ContainsRune(m.Str, '(') {
		m.Str = splitByFullP(m.Str, '(')
	}
	if !onlyifempty || m.Title == "" {
		if strings.ContainsRune(m.Str, '.') && !strings.ContainsRune(m.Str, ' ') {
			logger.StringReplaceWithP(&m.Str, '.', ' ')
		}
		if m.Str != "" && (m.Str[:1] == logger.StrSpace || m.Str[len(m.Str)-1:] == logger.StrSpace) {
			m.Title = strings.TrimSpace(m.Str)
		} else {
			m.Title = m.Str
		}
		if m.Title != "" && (m.Title[len(m.Title)-1:] == logger.StrDot || m.Title[len(m.Title)-1:] == logger.StrDash || m.Title[len(m.Title)-1:] == logger.StrSpace) {
			m.Title = strings.TrimRight(m.Title, "-. ")
		}
		if m.Title != "" && (m.Title[:1] == logger.StrSpace || m.Title[len(m.Title)-1:] == logger.StrSpace) {
			m.Title = strings.TrimSpace(m.Title)
		}
	}
	if !onlyifempty || m.Identifier == "" {
		if m.Identifier != "" && (m.Identifier[len(m.Identifier)-1:] == logger.StrDot || m.Identifier[len(m.Identifier)-1:] == logger.StrDash || m.Identifier[len(m.Identifier)-1:] == logger.StrSpace) {
			m.Identifier = strings.TrimRight(m.Identifier, " .-")
		}
		if m.Identifier != "" && (m.Identifier[:1] == logger.StrDot || m.Identifier[:1] == logger.StrDash || m.Identifier[:1] == logger.StrSpace) {
			m.Identifier = strings.TrimLeft(m.Identifier, " .-")
		}
	}
	if orgstr != "" {
		m.Str = orgstr
	}
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
func ParseFile(videofile string, usepath bool, usefolder bool, cfgp *config.MediaTypeConfig, listid int8) *database.ParseInfo {
	parsefor := videofile
	if usepath {
		parsefor = filepath.Base(videofile)
	}
	m := NewFileParser(parsefor, cfgp, false, listid)
	if m.Quality != "" && m.Resolution != "" {
		return m
	}
	if !usefolder || !usepath {
		return m
	}
	newFileParserP(filepath.Base(filepath.Dir(videofile)), true, cfgp, listid, m)
	return m
}

// ParseFileP parses a video file to extract metadata.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing parser to populate.
// It returns the populated parser after attempting to extract metadata.
func ParseFileP(videofile string, usepath bool, usefolder bool, cfgp *config.MediaTypeConfig, listid int8, m *database.ParseInfo) {
	parsefor := videofile
	if usepath {
		parsefor = filepath.Base(videofile)
	}
	newFileParserP(parsefor, false, cfgp, listid, m)

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
	m.ListID = -1
	if !cfgp.Useseries {
		if m.Imdb != "" {
			if !logger.HasPrefixI(m.Imdb, logger.StrTt) {
				sourceimdb := m.Imdb
				m.MovieFindDBIDByImdbParser()
				if m.DbmovieID == 0 && len(m.Imdb) < 7 {
					m.Imdb = logger.JoinStrings("0", sourceimdb)
					m.MovieFindDBIDByImdbParser()
					if m.DbmovieID == 0 && len(m.Imdb) < 6 {
						m.Imdb = logger.JoinStrings("00", sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
					if m.DbmovieID == 0 && len(m.Imdb) < 5 {
						m.Imdb = logger.JoinStrings("000", sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
					if m.DbmovieID == 0 && len(m.Imdb) < 4 {
						m.Imdb = logger.JoinStrings("0000", sourceimdb)
						m.MovieFindDBIDByImdbParser()
					}
				}
			} else {
				m.MovieFindDBIDByImdbParser()
			}
		}
		if m.DbmovieID == 0 && m.Title != "" && allowsearchtitle && cfgp.Name != "" {
			for idx := range cfgp.Lists {
				if cfgp.Lists[idx].TemplateQuality != "" {
					m.StripTitlePrefixPostfixGetQual(cfgp.Lists[idx].CfgQuality)
				}
			}
			if m.Title != "" && (m.Title[:1] == logger.StrSpace || m.Title[len(m.Title)-1:] == logger.StrSpace) {
				m.Title = strings.TrimSpace(m.Title)
			}
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
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.MovieID, &m.DbmovieID, &cfgp.Lists[m.ListID].Name)
		}

		if m.DbmovieID != 0 && m.MovieID == 0 && cfgp.Name != "" && m.ListID == -1 {
			for idx := range cfgp.Lists {
				if config.SettingsGeneral.UseMediaCache {
					m.MovieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheMovie, m.DbmovieID, cfgp.Lists[idx].Name)
					if m.MovieID != 0 {
						m.ListID = int8(idx)
						break
					}
				} else {
					database.ScanrowsNdyn(false, "select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", &m.MovieID, &cfgp.Lists[idx].Name, &m.DbmovieID)
					if m.MovieID != 0 {
						m.ListID = int8(idx)
						break
					}
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
		_ = database.Scanrows1dyn(false, database.QueryDbseriesGetIDByTvdb, &m.DbserieID, &m.Tvdb)
	}
	if m.DbserieID == 0 && m.Title != "" && (allowsearchtitle || m.Tvdb == "") {
		if m.Year != 0 {
			m.FindDbserieByName(logger.JoinStrings(m.Title, " (", logger.IntToString(m.Year), ")"))
		}
		if m.DbserieID == 0 {
			m.FindDbserieByName(m.Title)
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
		database.ScanrowsNdyn(false, database.QuerySeriesGetIDByDBIDListname, &m.SerieID, &m.DbserieID, &cfgp.Lists[m.ListID].Name)
	}
	if m.SerieID == 0 && cfgp != nil && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.SettingsGeneral.UseMediaCache {
				m.SerieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheSeries, m.DbserieID, cfgp.Lists[idx].Name)
				if m.SerieID != 0 {
					m.ListID = int8(idx)
					break
				}
			} else {
				database.ScanrowsNdyn(false, database.QuerySeriesGetIDByDBIDListname, &m.SerieID, &m.DbserieID, &cfgp.Lists[idx].Name)
				if m.SerieID != 0 {
					m.ListID = int8(idx)
					break
				}
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
func ParseVideoFile(m *database.ParseInfo, file string, quality *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}
	var err error
	switch config.SettingsGeneral.UseMediainfo {
	case true:
		err = parsemediainfo(m, file, quality)
	default:
		err = probeURL(m, file, quality)
	}
	if err == nil {
		return nil
	}
	if !config.SettingsGeneral.UseMediaFallback {
		return err
	}
	switch config.SettingsGeneral.UseMediainfo {
	case true:
		return probeURL(m, file, quality)
	default:
		return parsemediainfo(m, file, quality)
	}
}

// GetPriorityMapQual calculates priority for a ParseInfo based on its resolution,
// quality, codec, and audio IDs. It looks up missing IDs, applies defaults if configured,
// and maps IDs to names. It then calls getIDPriority to calculate the priority value.
func GetPriorityMapQual(m *database.ParseInfo, cfgp *config.MediaTypeConfig, quality *config.QualityConfig, useall, checkwanted bool) {
	if m.ResolutionID == 0 {
		m.ResolutionID = database.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = database.Gettypeids(m.Quality, database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = database.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = database.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
	}

	//var intid int
	if m.ResolutionID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetresolutionsIn, cfgp.DefaultResolution)
		if idx != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[idx].ID
		}
	}
	if m.QualityID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetqualitiesIn, cfgp.DefaultQuality)
		if idx != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[idx].ID
		}
	}

	if m.ResolutionID != 0 {
		idx := database.Getqualityidxbyid(database.DBConnect.GetresolutionsIn, m.ResolutionID)
		if idx != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[idx].Name
		}
		//m.Resolution = gettypeidloop(database.DBConnect.GetresolutionsIn, m.ResolutionID)
	}
	if m.QualityID != 0 {
		idx := database.Getqualityidxbyid(database.DBConnect.GetqualitiesIn, m.QualityID)
		if idx != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[idx].Name
		}
		//m.Quality = gettypeidloop(database.DBConnect.GetqualitiesIn, m.QualityID)
	}
	if m.AudioID != 0 {
		idx := database.Getqualityidxbyid(database.DBConnect.GetaudiosIn, m.AudioID)
		if idx != -1 {
			m.Audio = database.DBConnect.GetaudiosIn[idx].Name
		}
		//m.Resolution = gettypeidloop(database.DBConnect.GetresolutionsIn, m.ResolutionID)
	}
	if m.CodecID != 0 {
		idx := database.Getqualityidxbyid(database.DBConnect.GetcodecsIn, m.CodecID)
		if idx != -1 {
			m.Codec = database.DBConnect.GetcodecsIn[idx].Name
		}
		//m.Quality = gettypeidloop(database.DBConnect.GetqualitiesIn, m.QualityID)
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
		logger.LogDynamicany("debug", "prio not found", "searched for", BuildPrioStr(reso, qual, codec, aud), "in", &quality.Name)
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

	var prioreso, prioresoorg, prioqual, prioqualorg, priocod, prioaud int
	var target Prioarr
	for _, qual := range config.SettingsQuality {
		target.QualityGroup = qual.Name
		target.QualityID = 0
		target.ResolutionID = 0
		target.CodecID = 0
		target.AudioID = 0
		target.Priority = 0
		for idxreso := range getresolutions {
			addwanted = true
			if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(qual.WantedResolution, getresolutions[idxreso].Name) {
				logger.LogDynamicany("debug", "unwanted res", "Quality", &qual.Name, "Resolution Parse", &getresolutions[idxreso].Name)
				addwanted = false
			}
			target.ResolutionID = getresolutions[idxreso].ID
			prioreso = getresolutions[idxreso].Gettypeidprioritysingle("resolution", qual.QualityReorder)
			prioresoorg = prioreso
			for idxqual := range getqualities {
				if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(qual.WantedQuality, getqualities[idxqual].Name) {
					logger.LogDynamicany("debug", "unwanted qual", "Quality", &qual.Name, "Quality Parse", &getqualities[idxqual].Name)
					addwanted = false
				}

				target.QualityID = getqualities[idxqual].ID
				prioqual = getqualities[idxqual].Gettypeidprioritysingle("quality", qual.QualityReorder)
				prioqualorg = prioqual
				for idxcodec := range getcodecs {
					target.CodecID = getcodecs[idxcodec].ID
					priocod = getcodecs[idxcodec].Gettypeidprioritysingle("codec", qual.QualityReorder)
					for idxaudio := range getaudios {
						prioaud = getaudios[idxaudio].Gettypeidprioritysingle("audio", qual.QualityReorder)

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
						//target.Priority = getIDPrioritySimple(prioreso, prioqual, priocod, prioaud, getresolutions[idxreso].Name, getqualities[idxqual].Name, qual.QualityReorder)
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
func BuildPrioStr(r, q, c, a uint) []byte {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	//bld.Grow(12)
	bld.WriteUInt(r)
	bld.WriteRune('_')
	bld.WriteUInt(q)
	bld.WriteRune('_')
	bld.WriteUInt(c)
	bld.WriteRune('_')
	bld.WriteUInt(a)
	return bld.Bytes()
}
