// parser
package parser

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
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
	allQualityPrioritiesT       []Prioarr
	allQualityPrioritiesWantedT []Prioarr
	mediainfopath               string
	ffprobepath                 string
	scanpatterns                []regexpattern
	globalscanpatterns          = []regexpattern{
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
		database.GlobalCache.SetRegexp(scanpatterns[idx].re, 0)
	}
	for idx := range database.DBConnect.GetaudiosIn {
		if database.DBConnect.GetaudiosIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.GetaudiosIn[idx].Regex, getgroup: database.DBConnect.GetaudiosIn[idx].Regexgroup})
			database.GlobalCache.SetRegexp(database.DBConnect.GetaudiosIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetresolutionsIn {
		if database.DBConnect.GetresolutionsIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.GetresolutionsIn[idx].Regex, getgroup: database.DBConnect.GetresolutionsIn[idx].Regexgroup})
			database.GlobalCache.SetRegexp(database.DBConnect.GetresolutionsIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetqualitiesIn {
		if database.DBConnect.GetqualitiesIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.GetqualitiesIn[idx].Regex, getgroup: database.DBConnect.GetqualitiesIn[idx].Regexgroup})
			database.GlobalCache.SetRegexp(database.DBConnect.GetqualitiesIn[idx].Regex, 0)
		}
	}
	for idx := range database.DBConnect.GetcodecsIn {
		if database.DBConnect.GetcodecsIn[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.GetcodecsIn[idx].Regex, getgroup: database.DBConnect.GetcodecsIn[idx].Regexgroup})
			database.GlobalCache.SetRegexp(database.DBConnect.GetcodecsIn[idx].Regex, 0)
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
func NewFileParser(cleanName string, cfgp *config.MediaTypeConfig, listid int, allowtitlesearch bool) *apiexternal.FileParser {
	m := apiexternal.ParserPool.Get()
	newFileParserP(cleanName, cfgp, listid, allowtitlesearch, m)
	return m
}

// newFileParserP reuses a FileParser instance. It sets the filename,
// media config, list ID, and allow title search flag. It runs the main parsing
// logic like splitting the filename on delimiters, running regex matches,
// and cleaning up the parsed title and identifier.
func newFileParserP(cleanName string, cfgp *config.MediaTypeConfig, listid int, allowtitlesearch bool, m *apiexternal.FileParser) {
	if m.Filled {
		m.Clear()
	}
	m.Filled = true
	m.Str = cleanName
	m.Cfgp = cfgp
	m.M.ListID = listid
	m.Allowsearchtitle = allowtitlesearch
	m.M.File = m.Str
	if m.M.File != "" && (m.M.File[:1] == "[" || m.M.File[len(m.M.File)-1:] == "]") {
		m.Str = strings.TrimRight(strings.TrimLeft(m.M.File, "["), "]")
	} else if m.M.File != "" {
		m.Str = m.M.File
	}
	m.Str = logger.StringReplaceWith(m.Str, '_', ' ')
	if !config.SettingsGeneral.DisableParserStringMatch {
		m.Parsegroup("audio", database.DBConnect.AudioStrIn)
		m.Parsegroup("codec", database.DBConnect.CodecStrIn)
		m.Parsegroup("quality", database.DBConnect.QualityStrIn)
		m.Parsegroup("resolution", database.DBConnect.ResolutionStrIn)
	}

	m.Parsegroup("extended", []string{"extended", "extended cut", "extended.cut", "extended-cut"})
	m.ParsegroupEntry("proper", "proper")
	m.ParsegroupEntry("repack", "repack")

	var startIndex, endIndex = 0, len(m.Str)
	conttt := !logger.ContainsI(m.Str, logger.StrTt)
	conttvdb := !logger.ContainsI(m.Str, logger.StrTvdb)
	lenclean := len(m.Str)

	for idx := range scanpatterns {
		switch scanpatterns[idx].name {
		case logger.StrImdb:
			if m.Cfgp.Useseries || conttt {
				continue
			}
		case "tvdb":
			if !m.Cfgp.Useseries || conttvdb {
				continue
			}
		case "season", "episode", "identifier", logger.StrDate:
			if !m.Cfgp.Useseries {
				continue
			}
		case "audio":
			if m.M.Audio != "" {
				continue
			}
		case "codec":
			if m.M.Codec != "" {
				continue
			}
		case "quality":
			if m.M.Quality != "" {
				continue
			}
		case "resolution":
			if m.M.Resolution != "" {
				continue
			}
		}

		if getmatches(&scanpatterns[idx], m, &startIndex, &endIndex, lenclean) {
			continue
		}
	}
	if m.Cfgp.Useseries {
		if m.M.Identifier == "" && m.M.SeasonStr != "" && m.M.EpisodeStr != "" {
			m.M.Identifier = apiexternal.GenerateIdentifierString(&m.M)
		}

		if m.M.Date != "" && m.M.Identifier == "" {
			m.M.Identifier = m.M.Date
		}
	}
	if endIndex < startIndex {
		logger.LogDynamic("debug", "EndIndex < startindex", logger.NewLogField(logger.StrPath, m.M.File), logger.NewLogField("start", startIndex), logger.NewLogField("end", endIndex))
		m.Str = m.M.File[startIndex:]
	} else {
		m.Str = m.M.File[startIndex:endIndex]
	}
	if strings.ContainsRune(m.Str, '(') {
		m.Str = logger.SplitByFullP(m.Str, '(')
	}

	if strings.ContainsRune(m.Str, '.') && !strings.ContainsRune(m.Str, ' ') {
		m.Str = logger.StringReplaceWith(m.Str, '.', ' ')
		//m.Str = logger.StringsReplaceRune(m.Str, '.', ' ')
	} //else if strings.ContainsRune(m.Str, '.') && strings.ContainsRune(m.Str, '_') {
	//	m.Str = strings.Replace(m.Str, "_", " ", -1)
	//}
	if m.Str != "" && (m.Str[:1] == " " || m.Str[len(m.Str)-1:] == " ") {
		m.M.Title = strings.TrimSpace(m.Str)
	} else {
		m.M.Title = m.Str
	}
	if m.M.Title != "" && (m.M.Title[len(m.M.Title)-1:] == "." || m.M.Title[len(m.M.Title)-1:] == "-") {
		m.M.Title = strings.TrimRight(m.M.Title, "-.")
	}
	if m.M.Title != "" && (m.M.Title[:1] == " " || m.M.Title[len(m.M.Title)-1:] == " ") {
		m.M.Title = strings.TrimSpace(m.M.Title)
	}
	if m.M.Identifier != "" && (m.M.Identifier[len(m.M.Identifier)-1:] == "." || m.M.Identifier[len(m.M.Identifier)-1:] == "-" || m.M.Identifier[len(m.M.Identifier)-1:] == " ") {
		m.M.Identifier = strings.TrimRight(m.M.Identifier, " .-")
	}
	if m.M.Identifier != "" && (m.M.Identifier[:1] == "." || m.M.Identifier[:1] == "-" || m.M.Identifier[:1] == " ") {
		m.M.Identifier = strings.TrimLeft(m.M.Identifier, " .-")
	}
}

// getmatchesroot finds all substring matches in m.Str using the provided regular
// expression pattern. It returns a slice of integer indices indicating the start
// and end positions of the matched substring(s). For regex capture groups, the
// even indices are start positions and odd indices are end positions.
func getmatchesroot(pattern *regexpattern, m *apiexternal.FileParser) []int {
	if pattern.last && pattern.name == "year" && m.Cfgp.Useseries {
		matches := database.GlobalCache.GetRegexpDirect(pattern.re).FindAllStringSubmatchIndex(m.Str, 10)
		if !(len(matches) >= 1 && len(matches[len(matches)-1]) >= 1) {
			return nil
		}
		return matches[len(matches)-1]
	}
	//return database.GlobalCache.GetRegexpDirect(pattern.re).FindStringSubmatchIndex(m.Str)
	return database.Getfirstsubmatchindex(pattern.re, m.Str)
}

// getmatches extracts metadata from the file path or contents
// by matching regular expressions. It accepts a regexpattern struct,
// FileParser, and indices to track the substring match location.
// Returns bool indicating if matchdata was found.
func getmatches(pattern *regexpattern, m *apiexternal.FileParser, startIndex *int, endIndex *int, lenclean int) bool {
	matchest := getmatchesroot(pattern, m)
	lensubmatches := len(matchest)
	if lensubmatches == 0 {
		return true
	}

	var matchdata bool
	if lensubmatches >= 4 && ((lensubmatches/2)-1) >= pattern.getgroup && matchest[3] != -1 && matchest[(pattern.getgroup*2)+1] != -1 {
		if !m.Cfgp.Useseries || (m.Cfgp.Useseries && pattern.name != "year") {
			index := strings.Index(m.Str, m.Str[matchest[2]:matchest[3]])
			if index == 0 {
				if len(m.Str[matchest[2]:matchest[3]]) != lenclean && len(m.Str[matchest[2]:matchest[3]]) < *endIndex {
					*startIndex = len(m.Str[matchest[2]:matchest[3]])
				}
			} else if index < *endIndex && index > *startIndex {
				*endIndex = index
			}
		}
		matchdata = true
	} else if lensubmatches <= 2 && ((lensubmatches*2)-1) >= pattern.getgroup && matchest[(pattern.getgroup*2)+1] != -1 {
		matchdata = true
	} else if (lensubmatches*2) >= 4 && matchest[3] != -1 {
		//clear(matchest)
		return true
	}
	if !matchdata {
		//clear(matchest)
		return false
	}
	matchgroup := m.Str[matchest[pattern.getgroup*2]:matchest[(pattern.getgroup*2)+1]]
	switch pattern.name {
	case logger.StrImdb:
		m.M.Imdb = matchgroup
	case "tvdb":
		m.M.Tvdb, _ = strings.CutPrefix(matchgroup, logger.StrTvdb)
		if logger.HasPrefixI(matchgroup, logger.StrTvdb) {
			m.M.Tvdb = matchgroup[4:]
		}
	case "year":
		m.M.Year = logger.StringToInt(matchgroup)
	case "season":
		m.M.SeasonStr = matchgroup
		m.M.Season = logger.StringToInt(matchgroup)
	case "episode":
		m.M.EpisodeStr = matchgroup
		m.M.Episode = logger.StringToInt(matchgroup)
	case "identifier":
		m.M.Identifier = matchgroup
	case logger.StrDate:
		m.M.Date = matchgroup
	case "audio":
		m.M.Audio = matchgroup
	case "resolution":
		m.M.Resolution = matchgroup
	case "quality":
		m.M.Quality = matchgroup
	case "codec":
		m.M.Codec = matchgroup
	}
	//clear(matchest)
	return false
}

// parsedir parses a folder path to extract metadata into the passed in fileParser.
// It accepts a folder path, media type config, list ID, boolean to allow title search,
// a fileParser to populate, and boolean to return the new parser.
// It returns the originally passed in fileParser after attempting to extract metadata from the folder path.
func parsedir(folder string, cfgp *config.MediaTypeConfig, listid int, allowtitlesearch bool, m *apiexternal.FileParser) {
	mf := NewFileParser(folder, cfgp, listid, allowtitlesearch)
	if m.M.Quality == "" && mf.M.Quality != "" {
		m.M.Quality = mf.M.Quality
	}
	if m.M.Resolution == "" && mf.M.Resolution != "" {
		m.M.Resolution = mf.M.Resolution
	}
	if m.M.Title == "" && mf.M.Title != "" {
		m.M.Title = mf.M.Title
	}
	if m.M.Year == 0 && mf.M.Year != 0 {
		m.M.Year = mf.M.Year
	}
	if m.M.Identifier == "" && mf.M.Identifier != "" {
		m.M.Identifier = mf.M.Identifier
	}
	if m.M.Audio == "" && mf.M.Audio != "" {
		m.M.Audio = mf.M.Audio
	}
	if m.M.Codec == "" && mf.M.Codec != "" {
		m.M.Codec = mf.M.Codec
	}
	if m.M.Imdb == "" && mf.M.Imdb != "" {
		m.M.Imdb = mf.M.Imdb
	}
	apiexternal.ParserPool.Put(mf)
}

// ParseFile parses the given video file to extract metadata.
// It accepts a video file path, booleans to indicate whether to use the path and folder
// to extract metadata, a media type config, a list ID, and a FileParser to populate.
// It calls ParseFileP to parse the file and populate the FileParser, which is then returned.
func ParseFile(videofile string, usepath bool, usefolder bool, cfgp *config.MediaTypeConfig, listid int) *apiexternal.FileParser {
	m := apiexternal.ParserPool.Get()
	ParseFileP(videofile, usepath, usefolder, cfgp, listid, m)
	return m
}

// ParseFileP parses a video file to extract metadata.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing parser to populate.
// It returns the populated parser after attempting to extract metadata.
func ParseFileP(videofile string, usepath bool, usefolder bool, cfgp *config.MediaTypeConfig, listid int, m *apiexternal.FileParser) {
	if usepath {
		newFileParserP(filepath.Base(videofile), cfgp, listid, true, m)
	} else {
		newFileParserP(videofile, cfgp, listid, true, m)
	}

	if m.M.Quality != "" && m.M.Resolution != "" {
		return
	}
	if !usefolder || !usepath {
		return
	}
	parsedir(filepath.Base(filepath.Dir(videofile)), cfgp, listid, true, m)
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
func GetDBIDs(m *apiexternal.FileParser) error {
	m.M.ListID = -1
	if !m.Cfgp.Useseries {
		var imdb importfeed.ImdbID
		if m.M.Imdb != "" {
			if !logger.HasPrefixI(m.M.Imdb, logger.StrTt) {
				imdb.Imdb = logger.AddImdbPrefix(m.M.Imdb)
				m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
				if m.M.DbmovieID == 0 && len(m.M.Imdb) < 7 {
					imdb.Imdb = logger.AddImdbPrefix("0" + m.M.Imdb)
					m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
					if m.M.DbmovieID == 0 && len(m.M.Imdb) < 6 {
						imdb.Imdb = logger.AddImdbPrefix("00" + m.M.Imdb)
						m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
					}
					if m.M.DbmovieID == 0 && len(m.M.Imdb) < 5 {
						imdb.Imdb = logger.AddImdbPrefix("000" + m.M.Imdb)
						m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
					}
					if m.M.DbmovieID == 0 && len(m.M.Imdb) < 4 {
						imdb.Imdb = logger.AddImdbPrefix("0000" + m.M.Imdb)
						m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
					}
				}
			} else {
				imdb.Imdb = m.M.Imdb
				m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
			}
		}
		if m.M.DbmovieID == 0 && m.M.Title != "" && m.Allowsearchtitle && m.Cfgp.Name != "" {
			for idx := range m.Cfgp.Lists {
				if m.Cfgp.Lists[idx].TemplateQuality != "" {
					importfeed.StripTitlePrefixPostfixGetQual(&m.M, m.Cfgp.Lists[idx].CfgQuality)
				}
			}
			if m.M.Title != "" && (m.M.Title[:1] == " " || m.M.Title[len(m.M.Title)-1:] == " ") {
				m.M.Title = strings.TrimSpace(m.M.Title)
			}
			if m.M.Imdb == "" {
				importfeed.MovieFindImdbIDByTitle(false, m)
				imdb.Imdb = m.M.Imdb
			}
			if m.M.Imdb != "" && m.M.DbmovieID == 0 {
				m.M.DbmovieID = importfeed.MovieFindDBIDByImdb(&imdb)
			}
		}
		if m.M.DbmovieID == 0 {
			return logger.ErrNotFoundDbmovie
		}
		if m.M.DbmovieID != 0 && m.M.ListID != -1 {
			database.ScanrowsNdyn(false, database.QueryMoviesGetIDByDBIDListname, &m.M.MovieID, &m.M.DbmovieID, &m.Cfgp.Lists[m.M.ListID].Name)
		}

		if m.M.DbmovieID != 0 && m.M.MovieID == 0 && m.Cfgp.Name != "" && m.M.ListID == -1 {
			for idx := range m.Cfgp.Lists {
				if config.SettingsGeneral.UseMediaCache {
					m.M.MovieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheMovie, int(m.M.DbmovieID), m.Cfgp.Lists[idx].Name)
					if m.M.MovieID != 0 {
						m.M.ListID = idx
						break
					}
				} else {
					database.ScanrowsNdyn(false, "select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?", &m.M.MovieID, &m.Cfgp.Lists[idx].Name, &m.M.DbmovieID)
					if m.M.MovieID != 0 {
						m.M.ListID = idx
						break
					}
				}
			}
		}
		if m.M.MovieID == 0 {
			return logger.ErrNotFoundMovie
		}
		if m.M.ListID == -1 {
			m.M.ListID = database.GetMediaListIDGetListname(m.Cfgp, m.M.MovieID)
		}
		return nil
	}
	if m.M.Tvdb != "" {
		_ = database.ScanrowsNdyn(false, database.QueryDbseriesGetIDByTvdb, &m.M.DbserieID, &m.M.Tvdb)
	}
	if m.M.DbserieID == 0 && m.M.Title != "" && (m.Allowsearchtitle || m.M.Tvdb == "") {
		if m.M.Year != 0 {
			testtitle := logger.JoinStrings(m.M.Title, " (", strconv.Itoa(m.M.Year), ")")
			findDbserieByName(testtitle, &m.M)
		}
		if m.M.DbserieID == 0 {
			findDbserieByName(m.M.Title, &m.M)
		}
	}
	if m.M.DbserieID == 0 && m.M.File != "" {
		RegexGetMatchesStr1(m)
	}

	if m.M.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}
	SetDBEpisodeIDfromM(m)
	if m.M.DbserieEpisodeID == 0 {
		return logger.ErrNotFoundDBEpisode
	}
	if m.M.ListID != -1 {
		database.ScanrowsNdyn(false, database.QuerySeriesGetIDByDBIDListname, &m.M.SerieID, &m.M.DbserieID, &m.Cfgp.Lists[m.M.ListID].Name)
	}
	if m.M.SerieID == 0 && m.Cfgp != nil && m.M.ListID == -1 {
		for idx := range m.Cfgp.Lists {
			if config.SettingsGeneral.UseMediaCache {
				m.M.SerieID = database.CacheOneStringTwoIntIndexFuncRet(logger.CacheSeries, int(m.M.DbserieID), m.Cfgp.Lists[idx].Name)
				if m.M.SerieID != 0 {
					m.M.ListID = idx
					break
				}
			} else {
				database.ScanrowsNdyn(false, database.QuerySeriesGetIDByDBIDListname, &m.M.SerieID, &m.M.DbserieID, &m.Cfgp.Lists[idx].Name)
				if m.M.SerieID != 0 {
					m.M.ListID = idx
					break
				}
			}
		}
	}
	if m.M.SerieID == 0 {
		m.M.DbserieEpisodeID = 0
		m.M.SerieEpisodeID = 0
		return logger.ErrNotFoundSerie
	}
	if m.M.DbserieEpisodeID != 0 && m.M.SerieID != 0 {
		database.ScanrowsNdyn(false, "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?", &m.M.SerieEpisodeID, &m.M.SerieID, &m.M.DbserieEpisodeID)
	}

	if m.M.SerieEpisodeID == 0 {
		return logger.ErrNotFoundEpisode
	}
	if m.M.ListID == -1 {
		m.M.ListID = database.GetMediaListIDGetListname(m.Cfgp, m.M.SerieID)
	}
	return nil
}

// SetDBEpisodeIDfromM sets the DbserieEpisodeID field on the FileParser struct by looking
// up the episode ID in the database based on the season, episode, and identifier fields.
// It first tries looking up by season and episode number strings, then falls back to the identifier.
func SetDBEpisodeIDfromM(m *apiexternal.FileParser) {
	if m.M.SeasonStr != "" && m.M.EpisodeStr != "" {
		database.QueryDBEpisodeID(&m.M.DbserieID, &m.M.Season, &m.M.Episode, &m.M.DbserieEpisodeID)
		if m.M.DbserieEpisodeID != 0 {
			return
		}
	}

	if m.M.Identifier != "" {
		database.SetDBEpisodeIDByIdentifier(&m.M.DbserieEpisodeID, &m.M.DbserieID, &m.M.Identifier)
	}
}

// GetDBEpisodeID queries the database to get the episode ID for the given file parser, season, and episode number.
// It first tries to find the episode ID based on season number, then falls back to identifier if needed.
// Returns the episode ID or 0 if not found.
func GetDBEpisodeID(m *apiexternal.FileParser, epi string, dbserieid *uint, outid *uint) {
	if m.M.SeasonStr != "" {
		database.QueryDBEpisodeID(dbserieid, &m.M.Season, &epi, outid)
	}

	if *outid == 0 && m.M.Identifier != "" {
		database.SetDBEpisodeIDByIdentifier(outid, dbserieid, &m.M.Identifier)
	}
}

// findDbserieByName searches for a dbserie by title and sets dbid.
// It first checks the cache, then falls back to a database query.
// It handles both the original and slugged title.
func findDbserieByName(title string, m *database.ParseInfo) {
	if title == "" {
		return
	}
	if config.SettingsGeneral.UseMediaCache {
		database.CacheTwoStringIntIndexFunc(logger.CacheDBSeries, true, title, m)
		if m.DbserieID != 0 {
			return
		}
		database.CacheTwoStringIntIndexFunc(logger.CacheDBSeriesAlt, true, title, m)
		if m.DbserieID != 0 {
			return
		}
		slugged := logger.StringToSlug(title)
		if slugged == "" {
			return
		}
		database.CacheTwoStringIntIndexFunc(logger.CacheDBSeries, false, slugged, m)
		if m.DbserieID != 0 {
			return
		}
		database.CacheTwoStringIntIndexFunc(logger.CacheDBSeriesAlt, false, slugged, m)
		if m.DbserieID != 0 {
			return
		}
		return
	}

	_ = database.ScanrowsNdyn(false, database.QueryDbseriesGetIDByName, &m.DbserieID, &title)
	if m.DbserieID != 0 {
		return
	}
	slugged := logger.StringToSlug(title)
	if slugged == "" {
		return
	}
	_ = database.ScanrowsNdyn(false, "select id from dbseries where slug = ?", &m.DbserieID, &slugged)
	if m.DbserieID != 0 {
		return
	}
	_ = database.ScanrowsNdyn(false, "select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE", &m.DbserieID, &title)
	if m.DbserieID == 0 {
		_ = database.ScanrowsNdyn(false, "select dbserie_id from Dbserie_alternates where Slug = ?", &m.DbserieID, &slugged)
	}
}

// RegexGetMatchesStr1 extracts the series name from the filename
// by using a regular expression match. It looks for the series name substring
// in the filename, trims extra characters, and calls findDbserieByName
// to look up the series ID.
func RegexGetMatchesStr1(m *apiexternal.FileParser) {
	matchfor := filepath.Base(m.M.File)
	//matches := database.GlobalCache.GetRegexpDirect("RegexSeriesTitle").FindStringSubmatchIndex(matchfor)
	var matches []int
	if m.Cfgp.Useseries && m.M.Date != "" {
		matches = database.Getfirstsubmatchindex("RegexSeriesTitleDate", matchfor)
	}
	if len(matches) == 0 {
		matches = database.Getfirstsubmatchindex("RegexSeriesTitle", matchfor)
	}
	lenm := len(matches)
	if lenm == 0 {
		return
	}
	if lenm < 4 || matches[3] == -1 {
		return
	}
	seriename := string(bytes.TrimRight(logger.StringReplaceWithByte(matchfor[matches[2]:matches[3]], '.', ' '), ".- "))
	if seriename == "" {
		return
	}
	findDbserieByName(seriename, &m.M)
}

// ParseVideoFile parses metadata for a video file using ffprobe or MediaInfo.
// It first tries ffprobe, then falls back to MediaInfo if enabled.
// It takes a FileParser, path to the video file, and quality settings.
// It populates the FileParser with metadata parsed from the file.
// Returns an error if both parsing methods fail.
func ParseVideoFile(m *apiexternal.FileParser, file string, quality *config.QualityConfig) error {
	if file == "" {
		return logger.ErrNotFound
	}
	if m.M.Qualityset == nil {
		m.M.Qualityset = quality
	}
	if !config.SettingsGeneral.UseMediainfo {
		err := probeURL(m, file, quality)
		if err != nil && config.SettingsGeneral.UseMediaFallback {
			return parsemediainfo(m, file, quality)
		}
		return err
	}
	err := parsemediainfo(m, file, quality)
	if err != nil && config.SettingsGeneral.UseMediaFallback {
		return probeURL(m, file, quality)
	}
	return err
}

// GetPriorityMapQual calculates priority for a ParseInfo based on its resolution,
// quality, codec, and audio IDs. It looks up missing IDs, applies defaults if configured,
// and maps IDs to names. It then calls getIDPriority to calculate the priority value.
func GetPriorityMapQual(m *database.ParseInfo, cfgp *config.MediaTypeConfig, quality *config.QualityConfig, useall, checkwanted bool) {
	if m.Qualityset == nil {
		m.Qualityset = quality
	}

	if m.ResolutionID == 0 {
		m.ResolutionID = gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = gettypeids(m.Quality, database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
	}

	//var intid int
	if m.ResolutionID == 0 && cfgp != nil {
		idx := getqualityidxbyname(database.DBConnect.GetresolutionsIn, cfgp.DefaultResolution)
		if idx != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[idx].ID
		}
	}
	if m.QualityID == 0 && cfgp != nil {
		idx := getqualityidxbyname(database.DBConnect.GetqualitiesIn, cfgp.DefaultQuality)
		if idx != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[idx].ID
		}
	}

	if m.ResolutionID != 0 {
		idx := getqualityidxbyid(database.DBConnect.GetresolutionsIn, m.ResolutionID)
		if idx != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[idx].Name
		}
		//m.Resolution = gettypeidloop(database.DBConnect.GetresolutionsIn, m.ResolutionID)
	}
	if m.QualityID != 0 {
		idx := getqualityidxbyid(database.DBConnect.GetqualitiesIn, m.QualityID)
		if idx != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[idx].Name
		}
		//m.Quality = gettypeidloop(database.DBConnect.GetqualitiesIn, m.QualityID)
	}
	if m.AudioID != 0 {
		idx := getqualityidxbyid(database.DBConnect.GetaudiosIn, m.AudioID)
		if idx != -1 {
			m.Audio = database.DBConnect.GetaudiosIn[idx].Name
		}
		//m.Resolution = gettypeidloop(database.DBConnect.GetresolutionsIn, m.ResolutionID)
	}
	if m.CodecID != 0 {
		idx := getqualityidxbyid(database.DBConnect.GetcodecsIn, m.CodecID)
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
		intid = findpriorityidxwanted(reso, qual, codec, aud, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	} else {
		intid = findpriorityidx(reso, qual, codec, aud, quality)
		if intid != -1 {
			m.Priority = allQualityPrioritiesWantedT[intid].Priority
		}
	}

	if intid == -1 {
		logger.LogDynamic("debug", "prio not found", logger.NewLogField("searched for", buildPrioStr(reso, qual, codec, aud)), logger.NewLogField("in", quality.Name), logger.NewLogField("wanted", checkwanted))
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

// getqualityidxbyname searches the given quality table tbl by name
// and returns the index of the matching entry, or -1 if no match is found.
func getqualityidxbyname(tbl []database.QualitiesRegex, str string) int {
	for idx := range tbl {
		if strings.EqualFold(tbl[idx].Name, str) {
			return idx
		}
	}
	return -1
}

// getqualityidxbyid searches the given quality table tbl by ID
// and returns the index of the matching entry, or -1 if no match is found.
func getqualityidxbyid(tbl []database.QualitiesRegex, id uint) int {
	for idx := range tbl {
		if tbl[idx].ID == id {
			return idx
		}
	}
	return -1
}

// GetIDPrioritySimpleParse calculates a priority value for a ParseInfoSimple struct
// based on its resolution, quality, audio, and codec IDs. It looks up the priority
// values for each ID, applies any priority reordering rules, and sums the priorities.
// The qualcfg config is used to control which IDs are used and priority reordering.
// The useall and checkwanted params control including all IDs vs just configured ones,
// and checking wanted vs all available priorities.
func GetIDPrioritySimpleParse(id int, useseries bool, qualcfg *config.QualityConfig, useall bool, checkwanted bool) int {
	if qualcfg == nil {
		return 0
	}
	var reso, qual, aud, codec uint
	var proper, extended, repack bool
	database.GetdatarowArgs(logger.GetStringsMap(useseries, logger.DBFilesQuality), &id, &reso, &qual, &codec, &aud, &proper, &extended, &repack)

	if !qualcfg.UseForPriorityResolution && !useall {
		reso = 0
	}
	if !qualcfg.UseForPriorityQuality && !useall {
		qual = 0
	}
	if !qualcfg.UseForPriorityAudio && !useall {
		aud = 0
	}
	if !qualcfg.UseForPriorityCodec && !useall {
		codec = 0
	}
	var intid, prio int
	if checkwanted {
		intid = findpriorityidxwanted(reso, qual, codec, aud, qualcfg)
		if intid != -1 {
			prio = allQualityPrioritiesWantedT[intid].Priority
		}
	}
	if prio == 0 {
		intid = findpriorityidx(reso, qual, codec, aud, qualcfg)
		if intid != -1 {
			prio = allQualityPrioritiesWantedT[intid].Priority
		}
	}

	if intid == -1 {
		logger.LogDynamic("debug", "prio not found", logger.NewLogField("searched for", buildPrioStr(reso, qual, codec, aud)), logger.NewLogField("in", qualcfg.Name), logger.NewLogField("wanted", checkwanted))

		return 0
	}
	if !qualcfg.UseForPriorityOther && !useall {
		return prio
	}
	if proper {
		prio += 5
	}
	if extended {
		prio += 2
	}
	if repack {
		prio++
	}
	return prio
}

// findpriorityidxwanted searches through the allQualityPrioritiesWantedT slice
// to find the index of the priority entry matching the given resolution,
// quality, codec, and audio IDs. It is used to look up the priority value
// for a video file's metadata when checking if it matches a wanted quality.
func findpriorityidxwanted(reso, qual, codec, aud uint, quality *config.QualityConfig) int {
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
func findpriorityidx(reso, qual, codec, aud uint, quality *config.QualityConfig) int {
	for idx := range allQualityPrioritiesT {
		if allQualityPrioritiesT[idx].ResolutionID == reso && allQualityPrioritiesT[idx].QualityID == qual && allQualityPrioritiesT[idx].CodecID == codec && allQualityPrioritiesT[idx].AudioID == aud && strings.EqualFold(allQualityPrioritiesT[idx].QualityGroup, quality.Name) {
			return idx
		}
	}
	return -1
}

// getIDPrioritySimple calculates priority values for a ParseInfo's resolution,
// quality, codec, and audio IDs by looking them up in the corresponding database
// slices. It applies any priority reordering rules from the config's
// QualityReorderConfig slice. Returns the sum of the individual priority values.
func getIDPrioritySimple(priores, prioqual, priocodec, prioaud int, resolution, quality string, reordergroup []config.QualityReorderConfig) int {
	var idxcomma int
	for idxreorder := range reordergroup {
		if !strings.EqualFold(reordergroup[idxreorder].ReorderType, "combined_res_qual") {
			continue
		}
		if strings.ContainsRune(reordergroup[idxreorder].Name, ',') {
			continue
		}
		idxcomma = strings.IndexRune(reordergroup[idxreorder].Name, ',')

		if strings.EqualFold(reordergroup[idxreorder].Name[:idxcomma], resolution) && strings.EqualFold(reordergroup[idxreorder].Name[idxcomma+1:], quality) {
			priores = reordergroup[idxreorder].Newpriority
			prioqual = 0
		}
	}

	return priores + prioqual + priocodec + prioaud
}

// GetAllQualityPriorities generates all possible quality priority combinations
// by iterating through resolutions, qualities, codecs and audios. It builds up
// a target Prioarr struct containing the ID and name for each, and calculates
// the priority value based on the quality group's reorder rules. The results
// are added to allQualityPrioritiesT and allQualityPrioritiesWantedT slices.
func GenerateAllQualityPriorities() {
	regex0 := database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}}
	getresolutions := append(database.DBConnect.GetresolutionsIn, regex0)

	getqualities := append(database.DBConnect.GetqualitiesIn, regex0)

	getaudios := append(database.DBConnect.GetaudiosIn, regex0)

	getcodecs := append(database.DBConnect.GetcodecsIn, regex0)

	allQualityPrioritiesT = make([]Prioarr, 0, len(config.SettingsQuality)*len(getresolutions)*len(getqualities)*len(getaudios)*len(getcodecs))
	allQualityPrioritiesWantedT = make([]Prioarr, 0, len(config.SettingsQuality)*len(getresolutions)*len(getqualities)*len(getaudios)*len(getcodecs))
	var addwanted bool

	var prioreso, prioqual, priocod, prioaud int
	var reordergroup []config.QualityReorderConfig
	var target Prioarr
	var cfgreso []string
	var cfgqual []string
	for _, qual := range config.SettingsQuality {
		reordergroup = qual.QualityReorder
		target = Prioarr{}
		target.QualityGroup = qual.Name
		cfgreso = qual.WantedResolution
		cfgqual = qual.WantedQuality
		for idxreso := range getresolutions {
			addwanted = true
			if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(cfgreso, getresolutions[idxreso].Name) {
				logger.LogDynamic("debug", "unwanted res", logger.NewLogField("Quality", qual.Name), logger.NewLogField("Resolution Parse", getresolutions[idxreso].Name))
				addwanted = false
			}
			target.ResolutionID = getresolutions[idxreso].ID
			prioreso = gettypeidprioritysingle(&getresolutions[idxreso], "resolution", reordergroup)

			for idxqual := range getqualities {
				if database.DBLogLevel == logger.StrDebug && !logger.SlicesContainsI(cfgqual, getqualities[idxqual].Name) {
					logger.LogDynamic("debug", "unwanted qual", logger.NewLogField("Quality", qual.Name), logger.NewLogField("Quality Parse", getqualities[idxqual].Name))
					addwanted = false
				}

				target.QualityID = getqualities[idxqual].ID
				prioqual = gettypeidprioritysingle(&getqualities[idxqual], "quality", reordergroup)
				for idxcodec := range getcodecs {
					target.CodecID = getcodecs[idxcodec].ID
					priocod = gettypeidprioritysingle(&getcodecs[idxcodec], "codec", reordergroup)
					for idxaudio := range getaudios {
						prioaud = gettypeidprioritysingle(&getaudios[idxaudio], "audio", reordergroup)

						target.AudioID = getaudios[idxaudio].ID
						target.Priority = getIDPrioritySimple(prioreso, prioqual, priocod, prioaud, getresolutions[idxreso].Name, getqualities[idxqual].Name, reordergroup)
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

// gettypeids searches through the provided qualitytype slice to find a match for
// the given input string inval. It checks the Strings and Regex fields of each
// QualitiesRegex struct, returning the ID if a match is found. 0 is returned if no
// match is found.
func gettypeids(inval string, qualitytype []database.QualitiesRegex) uint {
	lenval := len(inval)
	var index, indexmax int
	for idxtype := range qualitytype {
		if qualitytype[idxtype].Strings != "" && !config.SettingsGeneral.DisableParserStringMatch && logger.ContainsI(qualitytype[idxtype].StringsLower, inval) {
			index = logger.IndexI(qualitytype[idxtype].StringsLower, inval)

			indexmax = index + lenval
			if indexmax < len(qualitytype[idxtype].StringsLower) && !apiexternal.CheckDigitLetter(rune(qualitytype[idxtype].StringsLower[indexmax : indexmax+1][0])) {
				return 0
			}
			if index > 0 && !apiexternal.CheckDigitLetter(rune(qualitytype[idxtype].StringsLower[index-1 : index][0])) {
				return 0
			}
			if qualitytype[idxtype].ID != 0 {
				return qualitytype[idxtype].ID
			}
		}
		if qualitytype[idxtype].UseRegex && qualitytype[idxtype].Regex != "" && database.RegexGetMatchesFind(qualitytype[idxtype].Regex, inval, 2) {
			return qualitytype[idxtype].ID
		}
	}
	return 0
}

// gettypeidprioritysingle returns the priority for the given QualitiesRegex struct
// after applying any reorder rules that match the given quality string type and name.
// It checks each QualityReorderConfig in the reordergroup, looking for matches on
// ReorderType and Name. If found, it will update the priority based on Newpriority.
func gettypeidprioritysingle(qual *database.QualitiesRegex, qualitystringtype string, reordergroup []config.QualityReorderConfig) int {
	priority := qual.Priority
	for idxreorder := range reordergroup {
		if strings.EqualFold(reordergroup[idxreorder].ReorderType, qualitystringtype) && strings.EqualFold(reordergroup[idxreorder].Name, qual.Name) {
			priority = reordergroup[idxreorder].Newpriority
		}
		if strings.EqualFold(reordergroup[idxreorder].ReorderType, "position") && strings.EqualFold(reordergroup[idxreorder].Name, qualitystringtype) {
			priority *= reordergroup[idxreorder].Newpriority
		}
	}
	return priority
}

// buildPrioStr builds a priority string from the given resolution, quality, codec, and audio values.
// The priority string is in the format "r_q_c_a" where r is the resolution, q is the quality, c is the codec,
// and a is the audio value. This allows easy comparison of release priority.
func buildPrioStr(r, q, c, a uint) string {
	bld := logger.PlBuffer.Get()
	bld.Grow(12)
	logger.BuilderAddUint(bld, r)
	bld.WriteRune('_')
	logger.BuilderAddUint(bld, q)
	bld.WriteRune('_')
	logger.BuilderAddUint(bld, c)
	bld.WriteRune('_')
	logger.BuilderAddUint(bld, a)
	defer logger.PlBuffer.Put(bld)
	return bld.String()
}
