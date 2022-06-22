// parser
package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"

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

var patterns = []regexpattern{
	{"season", false, `(?i)(s?(\d{1,4}))(?: )?[ex]`, 2},
	{"episode", false, `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`, 2},
	//{"episode", false, reflect.Int, regexp.MustCompile(`(-\s+([0-9]{1,})(?:[^0-9]|$))`)},
	{"identifier", false, `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex]\d{2,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
	{"date", false, `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
	{"year", true, `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, 2},

	//{"resolution", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((\d{3,4}[pi]))(?:\b|_)`, 0)},
	//{"quality", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((workprint|cam|webcam|hdts|ts|telesync|tc|telecine|r[2-8]|preair|sdtv|hdtv|pdtv|(?:(?:dvd|web|bd)\W?)?scr(?:eener)?|(?:web|dvd|hdtv|bd|br|dvb|dsr|ds|tv|ppv|hd)\W?rip|web\W?(?:dl|hd)?|hddvd|remux|(?:blu\W?ray)))(?:\b|_)`, 0)},
	//{"codec", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((xvid|divx|hevc|vp9|10bit|hi10p|h\.?264|h\.?265|x\.?264|x\.?265))(?:\b|_)`, 0)},
	//{"audio", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((mp3|aac|dd[0-9\\.]+|ac3|ac3d|ac3md|dd[p+][0-9\\.]+|flac|dts\W?hd(?:\W?ma)?|dts|truehd|mic|micdubbed))(?:\b|_)`, 0)},
	{"audio", false, `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`, 2},
	//{"region", false, reflect.String, regexp.MustCompile(`(?i)\b(R([0-9]))\b`)},
	//{"size", false, reflect.String, regexp.MustCompile(`(?i)\b((\d+(?:\.\d+)?(?:GB|MB)))\b`)},
	//{"website", false, reflect.String, regexp.MustCompile(`^(\[ ?([^\]]+?) ?\])`)},
	//{"language", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((english|eng|deutsch|german|ger|deu|french|italian|dutch|polish|fre|truefre|ita|dut|spa|spanish|rus|russian|tur|turkish|pol|nordic|kor|korean|hindi|swedish|hebrew|heb))(?:\b|_)`)},
	//{"subs", false, reflect.String, regexp.MustCompile(`(?i)(?:\b|_)((korsub|korsubs|swesub|swesubs|hebsub|hebsubs|sup|sups|subbed))(?:\b|_)`)},
	//{"sbs", false, reflect.String, regexp.MustCompile(`(?i)\b(((?:Half-)?SBS))\b`)},
	//{"container", false, reflect.String, regexp.MustCompile(`(?i)\b((MKV|AVI|MP4))\b`)},

	//{"group", false, reflect.String, regexp.MustCompile(`\b(- ?([^-]+(?:-={[^-]+-?$)?))$`)},

	//{"extended", false, reflect.Bool, regexp.MustCompile(`(?i)(?:\b|_)(EXTENDED(:?.CUT)?)(?:\b|_)`, 0)},
	//{"hardcoded", false, reflect.Bool, regexp.MustCompile(`(?i)\b((HC))\b`)},
	//{"proper", false, reflect.Bool, regexp.MustCompile(`(?i)(?:\b|_)((PROPER))(?:\b|_)`, 0)},
	{"imdb", false, `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, 2},
	{"tvdb", false, `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, 2},
	//{"repack", false, reflect.Bool, regexp.MustCompile(`(?i)(?:\b|_)((REPACK))(?:\b|_)`, 0)},
	//{"widescreen", false, reflect.Bool, regexp.MustCompile(`(?i)\b((WS))\b`)},
	//{"unrated", false, reflect.Bool, regexp.MustCompile(`(?i)\b((UNRATED))\b`)},
	//{"threeD", false, reflect.Bool, regexp.MustCompile(`(?i)\b((3D))\b`)},
}

var scanpatterns []regexpattern

func LoadDBPatterns() {
	for idx := range patterns {
		if !config.ConfigCheck(patterns[idx].re) {
			config.RegexAdd(patterns[idx].re, *regexp.MustCompile(patterns[idx].re))
			logger.Log.Infoln("Config added for: ", patterns[idx].re)
		}
	}
	scanpatterns = patterns
	for idx := range database.Getaudios {
		if database.Getaudios[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.Getaudios[idx].Regex, getgroup: 0})
			config.RegexDelete(database.Getaudios[idx].Regex)
			config.RegexAdd(database.Getaudios[idx].Regex, *regexp.MustCompile(database.Getaudios[idx].Regex))
		}
	}
	for idx := range database.Getresolutions {
		if database.Getresolutions[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.Getresolutions[idx].Regex, getgroup: 0})
			config.RegexDelete(database.Getresolutions[idx].Regex)
			config.RegexAdd(database.Getresolutions[idx].Regex, *regexp.MustCompile(database.Getresolutions[idx].Regex))
		}
	}
	for idx := range database.Getqualities {
		if database.Getqualities[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.Getqualities[idx].Regex, getgroup: 0})
			config.RegexDelete(database.Getqualities[idx].Regex)
			config.RegexAdd(database.Getqualities[idx].Regex, *regexp.MustCompile(database.Getqualities[idx].Regex))
		}
	}
	for idx := range database.Getcodecs {
		if database.Getcodecs[idx].UseRegex {
			scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.Getcodecs[idx].Regex, getgroup: 0})
			config.RegexDelete(database.Getcodecs[idx].Regex)
			config.RegexAdd(database.Getcodecs[idx].Regex, *regexp.MustCompile(database.Getcodecs[idx].Regex))
		}
	}
}

type ParseInfo struct {
	File         string
	Title        string
	Season       int    `json:"season,omitempty"`
	Episode      int    `json:"episode,omitempty"`
	SeasonStr    string `json:"seasonstr,omitempty"`
	EpisodeStr   string `json:"episodestr,omitempty"`
	Year         int    `json:"year,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	ResolutionID uint   `json:"resolutionid,omitempty"`
	Quality      string `json:"quality,omitempty"`
	QualityID    uint   `json:"qualityid,omitempty"`
	Codec        string `json:"codec,omitempty"`
	CodecID      uint   `json:"codecid,omitempty"`
	Audio        string `json:"audio,omitempty"`
	AudioID      uint   `json:"audioid,omitempty"`
	Priority     int    `json:"priority,omitempty"`
	//Group           string   `json:"group,omitempty"`
	//Region          string   `json:"region,omitempty"`
	Identifier string `json:"identifier,omitempty"`
	Date       string `json:"date,omitempty"`
	Extended   bool   `json:"extended,omitempty"`
	//Hardcoded       bool     `json:"hardcoded,omitempty"`
	Proper bool `json:"proper,omitempty"`
	Repack bool `json:"repack,omitempty"`
	//Container       string   `json:"container,omitempty"`
	//Widescreen      bool     `json:"widescreen,omitempty"`
	//Website         string   `json:"website,omitempty"`
	Language string `json:"language,omitempty"`
	//Sbs             string   `json:"sbs,omitempty"`
	//Unrated         bool     `json:"unrated,omitempty"`
	//Subs            string   `json:"subs,omitempty"`
	Imdb string `json:"imdb,omitempty"`
	Tvdb string `json:"tvdb,omitempty"`
	Size string `json:"size,omitempty"`
	//ThreeD          bool     `json:"3d,omitempty"`
	QualitySet      string   `json:"qualityset,omitempty"`
	Prio_audio      int      `json:"Prio_audio,omitempty"`
	Prio_codec      int      `json:"Prio_codec,omitempty"`
	Prio_resolution int      `json:"Prio_resolution,omitempty"`
	Prio_quality    int      `json:"Prio_quality,omitempty"`
	Languages       []string `json:"languages,omitempty"`
	Runtime         int      `json:"runtime,omitempty"`
	Height          int      `json:"height,omitempty"`
	Width           int      `json:"width,omitempty"`
}

func NewDefaultPrio(configTemplate string, qualityTemplate string) ParseInfo {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	m := ParseInfo{Quality: configEntry.DefaultQuality, Resolution: configEntry.DefaultResolution}
	m.GetPriority(configTemplate, qualityTemplate)
	return m
}

func NewCutoffPrio(configTemplate string, qualityTemplate string) ParseInfo {
	quality := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)

	m := ParseInfo{Quality: quality.Cutoff_quality, Resolution: quality.Cutoff_resolution}
	m.GetPriority(configTemplate, qualityTemplate)
	return m
}

func NewFileParser(filename string, includeYearInTitle bool, typegroup string) (ParseInfo, error) {
	m := ParseInfo{File: filename}
	err := m.ParseFile(includeYearInTitle, typegroup)
	return m, err
}

func (s *ParseInfo) Close() {
	if s == nil {
		return
	}
	if len(s.Languages) >= 1 {
		s.Languages = nil
	}
	s = nil
}

func (m *ParseInfo) AddMovieIfNotFound(listname string, configTemplate string) (movie uint, err error) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if len(m.Imdb) >= 1 {
		importfeed.JobImportMovies(m.Imdb, configTemplate, listname)
		movie, err = database.QueryColumnUint("Select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and  listname = ? COLLATE NOCASE", m.Imdb, listname)
		if err == nil {
			return
		}
	}

	dbmovie, found, found1 := importfeed.MovieFindDbIdByTitle(m.Title, strconv.Itoa(m.Year), "rss", configEntry.Data[0].AddFound)
	//getlist, _, entriesfound, dbmovie := importfeed.MovieFindListByTitle(configTemplate, m.Title, strconv.Itoa(m.Year), "rss")
	if found || found1 {
		movie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and  listname = ? COLLATE NOCASE", dbmovie, listname)
		if err != nil {
			if listname == configEntry.Data[0].AddFoundList && configEntry.Data[0].AddFound {
				var imdbID string
				imdbID, err = database.QueryColumnString("Select imdb_id from dbmovies where id = ?", dbmovie)
				importfeed.JobImportMovies(imdbID, configTemplate, listname)
				movie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and  listname = ? COLLATE NOCASE", dbmovie, listname)
				if err == nil {
					return
				}
				return 0, errors.New("not added")
			}
		}
	} else if listname == configEntry.Data[0].AddFoundList && configEntry.Data[0].AddFound {
		var imdbID string
		imdbID, found, found1 = importfeed.MovieFindImdbIDByTitle(m.Title, strconv.Itoa(m.Year), "rss", configEntry.Data[0].AddFound)
		importfeed.JobImportMovies(imdbID, configTemplate, listname)
		movie, err = database.QueryColumnUint("Select id from movies where dbmovie_id = ? and  listname = ? COLLATE NOCASE", dbmovie, listname)
		if err == nil {
			return
		}
		return 0, errors.New("not added")
	}
	return 0, errors.New("no match")
}

func (m *ParseInfo) Filter_test_quality_wanted(qualityTemplate string, title string) bool {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	wanted_release_resolution := false
	for idxqual := range qualityconfig.Wanted_resolution {
		if strings.EqualFold(qualityconfig.Wanted_resolution[idxqual], m.Resolution) {
			wanted_release_resolution = true
			break
		}
	}

	if len(qualityconfig.Wanted_resolution) >= 1 && !wanted_release_resolution {
		logger.Log.Debug("Skipped - unwanted resolution: ", title, " ", qualityTemplate, " ", m.Resolution)
		return false
	}
	wanted_release_quality := false
	for idxqual := range qualityconfig.Wanted_quality {
		if strings.EqualFold(qualityconfig.Wanted_quality[idxqual], m.Quality) {
			wanted_release_quality = true
			break
		}
	}
	if len(qualityconfig.Wanted_quality) >= 1 && !wanted_release_quality {
		logger.Log.Debug("Skipped - unwanted quality: ", title, " ", qualityTemplate, " ", m.Quality)
		return false
	}
	wanted_release_audio := false
	for idxqual := range qualityconfig.Wanted_audio {
		if strings.EqualFold(qualityconfig.Wanted_audio[idxqual], m.Audio) {
			wanted_release_audio = true
			break
		}
	}
	if len(qualityconfig.Wanted_audio) >= 1 && !wanted_release_audio {
		logger.Log.Debug("Skipped - unwanted audio: ", title, " ", qualityTemplate)
		return false
	}
	wanted_release_codec := false
	for idxqual := range qualityconfig.Wanted_codec {
		if strings.EqualFold(qualityconfig.Wanted_codec[idxqual], m.Codec) {
			wanted_release_codec = true
			break
		}
	}
	if len(qualityconfig.Wanted_codec) >= 1 && !wanted_release_codec {
		logger.Log.Debug("Skipped - unwanted codec: ", title, " ", qualityTemplate)
		return false
	}
	return true
}

func (n *ParseInfo) old_StripTitlePrefixPostfix(qualityTemplate string) {
	quality := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)

	lowertitle := strings.ToLower(n.Title)
	for idxstrip := range quality.TitleStripSuffixForSearch {
		if strings.HasSuffix(lowertitle, strings.ToLower(quality.TitleStripSuffixForSearch[idxstrip])) {
			n.Title = logger.TrimStringInclAfterStringInsensitive(n.Title, quality.TitleStripSuffixForSearch[idxstrip])
			n.Title = strings.Trim(n.Title, " ")
		}
	}
	for idxstrip := range quality.TitleStripPrefixForSearch {
		if strings.HasPrefix(lowertitle, strings.ToLower(quality.TitleStripPrefixForSearch[idxstrip])) {
			n.Title = logger.TrimStringPrefixInsensitive(n.Title, quality.TitleStripPrefixForSearch[idxstrip])
			n.Title = strings.Trim(n.Title, " ")
		}
	}
}

func StripTitlePrefixPostfix(n *ParseInfo, qualityTemplate string) {
	if qualityTemplate == "" {
		logger.Log.Error("missing quality information")
		return
	}
	quality := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	//lowertitle := strings.ToLower(n.Title)
	for idxstrip := range quality.TitleStripSuffixForSearch {
		//if strings.HasSuffix(lowertitle, strings.ToLower(quality.TitleStripSuffixForSearch[idxstrip])) {
		n.Title = logger.TrimStringInclAfterStringInsensitive(n.Title, quality.TitleStripSuffixForSearch[idxstrip])
		n.Title = strings.Trim(n.Title, " ")
		//}
	}
	for idxstrip := range quality.TitleStripPrefixForSearch {
		//if strings.HasPrefix(lowertitle, strings.ToLower(quality.TitleStripPrefixForSearch[idxstrip])) {
		n.Title = logger.TrimStringPrefixInsensitive(n.Title, quality.TitleStripPrefixForSearch[idxstrip])
		n.Title = strings.Trim(n.Title, " ")
		//}
	}
}

func before(value string, index int) string {
	if index <= 0 {
		return ""
	}
	return strings.Repeat(value[index-1:index], 1)
}

func after(value string, index int) string {
	if index >= len(value) {
		return ""
	}
	return strings.Repeat(value[index:index+1], 1)
}

func (m *ParseInfo) parsegroup(cleanName string, name string, startIndex int, endIndex int) (int, int) {
	var group []string
	switch name {
	case "audio":
		group = database.AudioStr
	case "codec":
		group = database.CodecStr
	case "quality":
		group = database.QualityStr
	case "resolution":
		group = database.ResolutionStr
	case "extended":
		group = []string{"extended", "extended cut", "extended.cut", "extended-cut", "extended_cut"}
	case "proper":
		group = []string{"proper"}
	case "repack":
		group = []string{"repack"}
	}

	defer logger.ClearVar(&group)
	tolower := strings.ToLower(cleanName)
	var index int
	var substr, substrpre, substrpost string
	for idx := range group {
		if strings.Contains(tolower, group[idx]) {
			index = strings.Index(tolower, group[idx])
			substr = strings.Repeat(cleanName[index:index+len(group[idx])], 1)
			substrpre = before(cleanName, index)
			substrpost = after(cleanName, index+len(group[idx]))
			if len(substrpost) >= 1 {
				if unicode.IsDigit([]rune(substrpost)[0]) || unicode.IsLetter([]rune(substrpost)[0]) {
					continue
				}
			}
			if len(substrpre) >= 1 {
				if unicode.IsDigit([]rune(substrpre)[0]) || unicode.IsLetter([]rune(substrpre)[0]) {
					continue
				}
			}
			switch name {
			case "audio":
				m.Audio = substr
			case "codec":
				m.Codec = substr
			case "quality":
				m.Quality = substr
			case "resolution":
				m.Resolution = substr
			case "extended":
				if len(substr) >= 1 {
					m.Extended = true
				}
			case "proper":
				if len(substr) >= 1 {
					m.Proper = true
				}
			case "repack":
				if len(substr) >= 1 {
					m.Repack = true
				}
			}
			break
		}
	}
	return startIndex, endIndex
}

func (m *ParseInfo) ParseFile(includeYearInTitle bool, typegroup string) error {
	var startIndex, endIndex = 0, len(m.File)
	cleanName := strings.Replace(m.File, "_", " ", -1)

	cleanName = strings.TrimLeft(cleanName, "[")
	cleanName = strings.TrimRight(cleanName, "]")

	if !config.ConfigCheck("general") {
		return errors.New("no general")
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !cfg_general.DisableParserStringMatch {

		for _, val := range []string{"audio", "codec", "quality", "resolution"} {
			startIndex, endIndex = m.parsegroup(cleanName, val, startIndex, endIndex)
		}
	}
	for _, val := range []string{"extended", "proper", "repack"} {
		startIndex, endIndex = m.parsegroup(cleanName, val, startIndex, endIndex)
	}

	tolower := strings.ToLower(cleanName)
	// fmt.Println(scanpatterns)
	for idxpattern := range scanpatterns {
		switch scanpatterns[idxpattern].name {
		case "imdb":
			if typegroup != "movie" {
				continue
			}
			if !strings.Contains(tolower, "tt") {
				continue
			}
		case "tvdb":
			if typegroup != "series" {
				continue
			}
			if !strings.Contains(tolower, "tvdb") {
				continue
			}
		case "season":
			if typegroup != "series" {
				continue
			}
		case "episode":
			if typegroup != "series" {
				continue
			}
		case "identifier":
			if typegroup != "series" {
				continue
			}
		case "date":
			if typegroup != "series" {
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

		matches := config.RegexGet(scanpatterns[idxpattern].re).FindAllStringSubmatch(cleanName, -1)

		if len(matches) == 0 {
			continue
		}
		matchIdx := 0
		if scanpatterns[idxpattern].last {
			// Take last occurence of element.
			matchIdx = len(matches) - 1
		}

		index := strings.Index(cleanName, matches[matchIdx][1])
		if !includeYearInTitle || (includeYearInTitle && scanpatterns[idxpattern].name != "year") {
			if index == 0 {
				if len(matches[matchIdx][1]) != len(cleanName) && len(matches[matchIdx][1]) < endIndex {
					startIndex = len(matches[matchIdx][1])
				}
			} else if index < endIndex && index > startIndex {
				endIndex = index
			}
		}
		switch scanpatterns[idxpattern].name {
		case "imdb":
			m.Imdb = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "tvdb":
			m.Tvdb = matches[matchIdx][scanpatterns[idxpattern].getgroup]
			m.Tvdb = strings.TrimPrefix(m.Tvdb, "tvdb")
		case "year":
			mint, err := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
			if err != nil {
				continue
			}
			m.Year = mint
		case "season":
			mint, err := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
			if err != nil {
				continue
			}
			m.Season = mint
			m.SeasonStr = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "episode":
			mint, err := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
			if err != nil {
				continue
			}
			m.Episode = mint
			m.EpisodeStr = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "identifier":
			m.Identifier = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "date":
			m.Date = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "audio":
			m.Audio = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "resolution":
			m.Resolution = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "quality":
			m.Quality = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "codec":
			m.Codec = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		}
		matches = nil
	}
	if len(m.Date) >= 1 {
		m.Identifier = m.Date
	} else {
		if len(m.Identifier) == 0 && m.SeasonStr != "" && m.EpisodeStr != "" {
			m.Identifier = "S" + m.SeasonStr + "E" + m.EpisodeStr
		}
	}
	raw := ""
	if endIndex < startIndex {
		logger.Log.Debug("EndIndex < startindex", startIndex, endIndex, m.File)
		strstart := strings.Repeat(m.File[startIndex:], 1)
		if strings.Contains(strstart, "(") {
			rawarr := strings.Split(strstart, "(")
			if len(rawarr) >= 1 {
				raw = rawarr[0]
			} else {
				raw = strstart
			}
			rawarr = nil
		} else {
			raw = strstart
		}
	} else {
		if strings.Contains(strings.Repeat(m.File[startIndex:endIndex], 1), "(") {
			rawarr := strings.Split(strings.Repeat(m.File[startIndex:endIndex], 1), "(")
			if len(rawarr) >= 1 {
				raw = rawarr[0]
			} else {
				raw = strings.Repeat(m.File[startIndex:endIndex], 1)
			}
			rawarr = nil
		} else {
			raw = strings.Repeat(m.File[startIndex:endIndex], 1)
		}
	}

	cleanName = strings.TrimPrefix(raw, "- ")
	if strings.ContainsRune(cleanName, '.') && !strings.ContainsRune(cleanName, ' ') {
		cleanName = strings.Replace(cleanName, ".", " ", -1)
	}
	if strings.ContainsRune(cleanName, '.') {
		cleanName = strings.Replace(cleanName, "_", " ", -1)
	}
	cleanName = strings.TrimSpace(cleanName)
	cleanName = strings.TrimSuffix(cleanName, "-")
	m.Title = strings.TrimSpace(cleanName)

	return nil
}

func (m *ParseInfo) GetPriority(configTemplate string, qualityTemplate string) {
	if m.Priority != 0 && m.Resolution != "" && m.ResolutionID != 0 && m.Prio_resolution != 0 && m.Quality != "" && m.QualityID != 0 && m.Prio_quality != 0 {
		return
	}
	m.QualitySet = qualityTemplate

	m.gettypepriority(m.Resolution, "resolution", qualityTemplate, true)

	m.gettypepriority(m.Quality, "quality", qualityTemplate, true)

	m.gettypepriority(m.Codec, "codec", qualityTemplate, true)

	m.gettypepriority(m.Audio, "audio", qualityTemplate, true)

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if m.QualityID == 0 {
		m.getdefaulttypepriority(configEntry.DefaultQuality, "quality", m.QualityID, qualityTemplate, false)
	}
	if m.ResolutionID == 0 {
		m.getdefaulttypepriority(configEntry.DefaultResolution, "resolution", m.ResolutionID, qualityTemplate, false)
	}
	m.getcombinedpriority(qualityTemplate)

	m.Priority = m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
	if m.Proper {
		m.Priority = m.Priority + 5
	}
	if m.Extended {
		m.Priority = m.Priority + 2
	}
	if m.Repack {
		m.Priority = m.Priority + 1
	}
}

func (m *ParseInfo) ParseVideoFile(file string, configTemplate string, qualityTemplate string) error {
	if m.QualitySet == "" {
		m.QualitySet = qualityTemplate
	}
	video, err := NewVideoFile(getFFProbeFilename(), file, false)
	if err == nil {
		defer logger.ClearVar(&video)
		logger.Log.Debug("Parsed Video as Audio: ", video.AudioCodec)
		logger.Log.Debug("Parsed Video as Codec: ", video.VideoCodec)
		logger.Log.Debug("Parsed Video as Height: ", video.Height)
		m.Runtime = int(video.Duration)
		if m.Audio == "" || (!strings.EqualFold(video.AudioCodec, m.Audio) && video.AudioCodec != "") {
			m.gettypepriority(video.AudioCodec, "audio", qualityTemplate, false)
		}
		if strings.EqualFold(video.VideoCodec, "mpeg4") && strings.EqualFold(video.VideoCodecTagString, "XVID") {
			video.VideoCodec = video.VideoCodecTagString
		}
		if m.Codec == "" || (!strings.EqualFold(video.VideoCodec, m.Codec) && video.VideoCodec != "") {
			m.gettypepriority(video.VideoCodec, "codec", qualityTemplate, false)
		}
		getreso := ""
		if video.Height == 360 {
			getreso = "360p"
		}
		if video.Height > 360 {
			getreso = "368p"
		}
		if video.Height > 368 || video.Width == 720 {
			getreso = "480p"
		}
		if video.Height > 480 {
			getreso = "576p"
		}
		if video.Height > 576 || video.Width == 1280 {
			getreso = "720p"
		}
		if video.Height > 720 || video.Width == 1920 {
			getreso = "1080p"
		}
		if video.Height > 1080 || video.Width == 3840 {
			getreso = "2160p"
		}
		m.Height = video.Height
		m.Width = video.Width
		if m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution) {
			m.gettypepriority(getreso, "resolution", qualityTemplate, false)
		}
		m.Languages = video.AudioLanguages

		//m.GetIDPriority(configTemplate, qualityTemplate)
		// m.Priority = m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
		// if m.Proper {
		// 	m.Priority = m.Priority + 5
		// }
		// if m.Extended {
		// 	m.Priority = m.Priority + 2
		// }
		// if m.Repack {
		// 	m.Priority = m.Priority + 1
		// }
		return nil
	} else {

		return err
	}
}

func (m *ParseInfo) GetIDPriority(configTemplate string, qualityTemplate string) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if m.ResolutionID != 0 {
		m.gettypeidpriority(m.ResolutionID, "resolution", qualityTemplate, true)
	} else {
		m.getdefaulttypepriority(configEntry.DefaultResolution, "resolution", m.ResolutionID, qualityTemplate, true)
	}
	if m.QualityID != 0 {
		m.gettypeidpriority(m.QualityID, "quality", qualityTemplate, true)
	} else {
		m.getdefaulttypepriority(configEntry.DefaultQuality, "quality", m.QualityID, qualityTemplate, true)
	}
	if m.CodecID != 0 {
		m.gettypeidpriority(m.CodecID, "codec", qualityTemplate, true)
	}
	if m.AudioID != 0 {
		m.gettypeidpriority(m.AudioID, "audio", qualityTemplate, true)
	}

	m.getcombinedpriority(qualityTemplate)

	m.Priority = m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
	if m.Proper {
		m.Priority = m.Priority + 5
	}
	if m.Extended {
		m.Priority = m.Priority + 2
	}
	if m.Repack {
		m.Priority = m.Priority + 1
	}
}

func qualityreorder(qualityReorder []config.QualityReorderConfig, priority int, qualitystringtype string, qualitytypename string) (bool, int) {
	defer logger.ClearVar(&qualityReorder)
	found := false
	for idx := range qualityReorder {
		if strings.EqualFold(qualityReorder[idx].ReorderType, qualitystringtype) && strings.EqualFold(qualityReorder[idx].Name, qualitytypename) {
			found = true
			priority = qualityReorder[idx].Newpriority
		}
		if strings.EqualFold(qualityReorder[idx].ReorderType, "position") && strings.EqualFold(qualityReorder[idx].Name, qualitystringtype) {
			found = true
			priority = priority * qualityReorder[idx].Newpriority
		}
	}
	return found, priority
}

func (m *ParseInfo) gettypepriority(inval string, qualitystringtype string, qualityTemplate string, clearname bool) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	var qualitytype []database.QualitiesRegex
	defer logger.ClearVar(&qualitytype)
	switch qualitystringtype {
	case "resolution":
		qualitytype = database.Getresolutions
	case "quality":
		qualitytype = database.Getqualities
	case "codec":
		qualitytype = database.Getcodecs
	case "audio":
		qualitytype = database.Getaudios
	}
	tolower := strings.ToLower(inval)
	var id uint
	var priority, index, substrpost_len int
	var name, substrpre, substrpost string
	var isokpost, isokpre bool
	var teststr []string
	defer logger.ClearVar(&teststr)
	for idxqual := range qualitytype {
		if len(qualitytype[idxqual].Strings) >= 1 {
			if strings.Contains(qualitytype[idxqual].Strings, tolower) {
				index = strings.Index(qualitytype[idxqual].Strings, tolower)
				substrpre = ""
				if index >= 1 {
					substrpre = strings.Repeat(qualitytype[idxqual].Strings[index-1:index], 1)
				}
				substrpost_len = index + len(inval) + 1
				if len(qualitytype[idxqual].Strings) < substrpost_len {
					substrpost_len = index + len(inval)
				}
				substrpost = strings.Repeat(qualitytype[idxqual].Strings[index+len(inval):substrpost_len], 1)
				isokpost = true
				isokpre = true
				if len(substrpost) >= 1 {
					if unicode.IsDigit([]rune(substrpost)[0]) || unicode.IsLetter([]rune(substrpost)[0]) {
						isokpost = false
					}
				}
				if len(substrpre) >= 1 {
					if unicode.IsDigit([]rune(substrpre)[0]) || unicode.IsLetter([]rune(substrpre)[0]) {
						isokpre = false
					}
				}
				if isokpre && isokpost {
					id = qualitytype[idxqual].ID
					name = qualitytype[idxqual].Name
					priority = qualitytype[idxqual].Priority
					if len(qualityconfig.QualityReorder) >= 1 {
						for idx := range qualityconfig.QualityReorder {
							if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitytype[idxqual].Name) {
								priority = qualityconfig.QualityReorder[idx].Newpriority
							}
							if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, "position") && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitystringtype) {
								priority = priority * qualityconfig.QualityReorder[idx].Newpriority
							}
						}
					}
					break
				}
			}
		} else {
			if config.RegexCheck(qualitytype[idxqual].Regex) {
				teststr = config.RegexGet(qualitytype[idxqual].Regex).FindStringSubmatch(tolower)
				if len(teststr) >= 2 {
					id = qualitytype[idxqual].ID
					name = qualitytype[idxqual].Name
					priority = qualitytype[idxqual].Priority
					if len(qualityconfig.QualityReorder) >= 1 {
						for idx := range qualityconfig.QualityReorder {
							if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitytype[idxqual].Name) {
								priority = qualityconfig.QualityReorder[idx].Newpriority
							}
							if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, "position") && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitystringtype) {
								priority = priority * qualityconfig.QualityReorder[idx].Newpriority
							}
						}
					}
					break
				}
			}
		}

	}
	if id != 0 {
		switch qualitystringtype {
		case "resolution":
			m.Resolution = name
			m.ResolutionID = id
			m.Prio_resolution = priority
		case "quality":

			m.Quality = name
			m.QualityID = id
			m.Prio_quality = priority
		case "codec":

			m.Codec = name
			m.CodecID = id
			m.Prio_codec = priority
		case "audio":

			m.Audio = name
			m.AudioID = id
			m.Prio_audio = priority
		}
	} else {
		if clearname {
			switch qualitystringtype {
			case "resolution":
				m.Resolution = ""
			case "quality":
				m.Quality = ""
			case "codec":
				m.Codec = ""
			case "audio":
				m.Audio = ""
			}
		}
	}
	return
}
func (m *ParseInfo) gettypeidpriority(id uint, qualitystringtype string, qualityTemplate string, setprioonly bool) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	var qualitytype []database.QualitiesRegex
	defer logger.ClearVar(&qualitytype)
	switch qualitystringtype {
	case "resolution":
		qualitytype = database.Getresolutions
	case "quality":
		qualitytype = database.Getqualities
	case "codec":
		qualitytype = database.Getcodecs
	case "audio":
		qualitytype = database.Getaudios
	}
	for idxqual := range qualitytype {
		if qualitytype[idxqual].ID == id {
			name := qualitytype[idxqual].Name
			priority := qualitytype[idxqual].Priority
			if len(qualityconfig.QualityReorder) >= 1 {
				for idx := range qualityconfig.QualityReorder {
					if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitytype[idxqual].Name) {
						priority = qualityconfig.QualityReorder[idx].Newpriority
					}
					if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, "position") && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitystringtype) {
						priority = priority * qualityconfig.QualityReorder[idx].Newpriority
					}
				}
			}
			switch qualitystringtype {
			case "resolution":
				if !setprioonly {
					m.Resolution = name
				}
				m.Prio_resolution = priority
			case "quality":
				if !setprioonly {
					m.Quality = name
				}
				m.Prio_quality = priority
			case "codec":
				if !setprioonly {
					m.Codec = name
				}
				m.Prio_codec = priority
			case "audio":
				if !setprioonly {
					m.Audio = name
				}
				m.Prio_audio = priority
			}

			break
		}
	}
	return
}

func (m *ParseInfo) getdefaulttypepriority(qualitystring string, qualitystringtype string, qualityid uint, qualityTemplate string, setprioonly bool) (id uint, priority int, name string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	var qualitytype []database.QualitiesRegex
	defer logger.ClearVar(&qualitytype)
	switch qualitystringtype {
	case "resolution":
		qualitytype = database.Getresolutions
	case "quality":
		qualitytype = database.Getqualities
	case "codec":
		qualitytype = database.Getcodecs
	case "audio":
		qualitytype = database.Getaudios
	}
	if qualitystring != "" && qualityid == 0 {
		for idxqual := range qualitytype {
			if strings.EqualFold(qualitytype[idxqual].Name, qualitystring) {
				logger.Log.Debug("use default qual: ", qualitystring)
				id = qualitytype[idxqual].ID
				name = qualitytype[idxqual].Name

				priority = qualitytype[idxqual].Priority
				if len(qualityconfig.QualityReorder) >= 1 {
					for idx := range qualityconfig.QualityReorder {
						if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitytype[idxqual].Name) {
							priority = qualityconfig.QualityReorder[idx].Newpriority
						}
						if strings.EqualFold(qualityconfig.QualityReorder[idx].ReorderType, "position") && strings.EqualFold(qualityconfig.QualityReorder[idx].Name, qualitystringtype) {
							priority = priority * qualityconfig.QualityReorder[idx].Newpriority
						}
					}
				}
			}
		}
	}
	if id != 0 {
		switch qualitystringtype {
		case "resolution":
			if !setprioonly {
				m.Resolution = name
				m.ResolutionID = id
			}
			m.Prio_resolution = priority
		case "quality":
			if !setprioonly {

				m.Quality = name
				m.QualityID = id
			}
			m.Prio_quality = priority
		}
	}
	return
}

func (m *ParseInfo) getcombinedpriority(qualityTemplate string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	if len(qualityconfig.QualityReorder) >= 1 {
		var namearr []string
		defer logger.ClearVar(&namearr)
		for idxreorder := range qualityconfig.QualityReorder {
			if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].ReorderType, "combined_res_qual") {
				namearr = strings.Split(qualityconfig.QualityReorder[idxreorder].Name, ",")

				if len(namearr) != 2 {
					continue
				}

				if strings.EqualFold(namearr[0], m.Resolution) && strings.EqualFold(namearr[1], m.Quality) {
					m.Prio_resolution = qualityconfig.QualityReorder[idxreorder].Newpriority
					m.Prio_quality = 0
				}
			}
		}
	}
}

func GetSerieDBPriorityById(episodefileid uint, configTemplate string, qualityTemplate string) int {
	serieepisodefile, err := database.GetSerieEpisodeFiles(database.Query{Where: "id = ?", WhereArgs: []interface{}{episodefileid}})
	if err != nil {
		return 0
	}
	m := &ParseInfo{
		File:         serieepisodefile.Location,
		Title:        serieepisodefile.Filename,
		ResolutionID: serieepisodefile.ResolutionID,
		QualityID:    serieepisodefile.QualityID,
		CodecID:      serieepisodefile.CodecID,
		AudioID:      serieepisodefile.AudioID,
		Proper:       serieepisodefile.Proper,
		Extended:     serieepisodefile.Extended,
		Repack:       serieepisodefile.Repack}

	defer logger.ClearVar(&m)
	m.GetIDPriority(configTemplate, qualityTemplate)
	return m.Priority
}

func GetMovieDBPriorityById(moviefileid uint, configTemplate string, qualityTemplate string) int {
	moviefile, err := database.GetMovieFiles(database.Query{Where: "id = ?", WhereArgs: []interface{}{moviefileid}})
	if err != nil {
		return 0
	}
	m := ParseInfo{}
	defer logger.ClearVar(&m)
	m.ResolutionID = moviefile.ResolutionID
	m.QualityID = moviefile.QualityID
	m.CodecID = moviefile.CodecID
	m.AudioID = moviefile.AudioID
	m.Proper = moviefile.Proper
	m.Extended = moviefile.Extended
	m.Repack = moviefile.Repack
	m.GetIDPriority(configTemplate, qualityTemplate)
	return m.Priority
}

// Path makes a string safe to use as a URL path,
// removing accents and replacing separators with -.
// The path may still start at / and is not intended
// for use as a file system path without prefix.

func filteronestringoneint(rows []database.Dbstatic_OneStringOneInt, search string, casesensitive bool) int {
	defer logger.ClearVar(&rows)
	for idx := range rows {
		if casesensitive {
			if rows[idx].Str == search {
				return rows[idx].Num
			}
		} else {
			if strings.EqualFold(rows[idx].Str, search) {
				return rows[idx].Num
			}
		}
	}
	return 0
}

func findparsefilteron(title string, listname string, alternate bool) (uint, error) {
	var dbserietitles []database.Dbstatic_OneStringOneInt
	var dbserietitlesslug []database.Dbstatic_OneStringOneInt

	if !alternate {
		dbserietitles, _ = database.QueryStaticColumnsOneStringOneInt("Select seriename, id from dbseries where seriename = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "Select count(id) from dbseries where seriename = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, title)
		dbserietitlesslug, _ = database.QueryStaticColumnsOneStringOneInt("Select slug, id from dbseries where seriename = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "Select count(id) from dbseries where seriename = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, title)
	} else {
		dbserietitles, _ = database.QueryStaticColumnsOneStringOneInt("Select title, dbserie_id from dbserie_alternates where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "Select count(id) from dbserie_alternates where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, title)
		dbserietitlesslug, _ = database.QueryStaticColumnsOneStringOneInt("Select slug, dbserie_id from dbserie_alternates where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", "Select count(id) from dbserie_alternates where title = ? COLLATE NOCASE or slug = ? COLLATE NOCASE", title, title)
	}
	defer logger.ClearVar(&dbserietitles)
	defer logger.ClearVar(&dbserietitlesslug)
	var dbmovieid int

	for idx := range dbserietitles {
		if strings.EqualFold(dbserietitles[idx].Str, title) {
			dbmovieid = dbserietitles[idx].Num
			break
		}
	}
	if dbmovieid == 0 {
		for idx := range dbserietitlesslug {
			if strings.EqualFold(dbserietitlesslug[idx].Str, title) {
				dbmovieid = dbserietitlesslug[idx].Num
				break
			}
		}
	}
	if dbmovieid != 0 {
		findseries, _ := database.QueryStaticColumnsOneInt("Select id from series where dbserie_id = ? AND listname = ? COLLATE NOCASE", "", dbmovieid, listname)
		defer logger.ClearVar(&findseries)
		if len(findseries) == 1 {
			return uint(findseries[0].Num), nil
		}
	}
	return 0, errors.New("no serie")
}
func (m *ParseInfo) FindSerieByParser(titleyear string, seriestitle string, listname string) (uint, int, error) {
	var dbserie_id uint
	if m.Tvdb != "" {
		counter, _ := database.CountRowsStatic("Select count(id) from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
		if counter == 1 {
			dbserie_id, _ = database.QueryColumnUint("Select id from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
		}
	}
	if dbserie_id == 0 {
		dbserie_id, _ = importfeed.FindDbserieByName(titleyear)
	}
	if dbserie_id == 0 {
		dbserie_id, _ = importfeed.FindDbserieByName(seriestitle)
	}
	if dbserie_id == 0 {
		dbserie_id, _ = importfeed.FindDbserieByName(m.Title)
	}
	if dbserie_id != 0 {
		counter, _ := database.CountRowsStatic("Select count(id) from series where dbserie_id = ? AND  listname = ? COLLATE NOCASE", dbserie_id, listname)
		if counter == 1 {
			serieid, _ := database.QueryColumnUint("Select id from series where dbserie_id = ? AND  listname = ? COLLATE NOCASE", dbserie_id, listname)
			return serieid, 1, nil
		}
	}
	return 0, 0, errors.New("not found")
}

func (m *ParseInfo) FindMovieByFile(listnames []string) (uint, string, string, error) {
	defer logger.ClearVar(&listnames)
	entriesfound := 0
	argslist := config.StringArrayToInterfaceArray(listnames)
	defer logger.ClearVar(&argslist)
	if entriesfound == 0 && len(m.Imdb) >= 1 {
		entriesfound, _ = database.CountRowsStatic("Select count(id) from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", m.Imdb, argslist)
		if entriesfound == 1 {
			id, _ := database.QueryColumnUint("Select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", m.Imdb, argslist)
			listname, _ := database.QueryColumnString("Select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", m.Imdb, argslist)
			return id, "", listname, nil
		}
	}

	if entriesfound == 0 {
		dbmovieid, found, found1 := importfeed.MovieFindDbIdByTitle(m.Title, strconv.Itoa(m.Year), "parse", false)
		if found || found1 {
			entriesfound, _ = database.CountRowsStatic("Select count(id) from movies where dbmovie_id = ? and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", dbmovieid, argslist)
			if entriesfound == 1 {
				id, _ := database.QueryColumnUint("Select id from movies where dbmovie_id = ? and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", dbmovieid, argslist)
				listname, _ := database.QueryColumnString("Select listname from movies where dbmovie_id = ? and listname in (?"+strings.Repeat(",?", len(argslist)-1)+")", dbmovieid, argslist)
				return id, "", listname, nil
			}
		}
	}

	return 0, "", "", errors.New("no movie found")
}
func (m *ParseInfo) FindDbmovieByFile() (uint, error) {
	var dbmovieid uint
	var err error
	if len(m.Imdb) >= 1 {
		dbmovieid, err = database.QueryColumnUint("Select id from dbmovies where imdb_id = ?", m.Imdb)
		if err == nil {
			if dbmovieid != 0 {
				return dbmovieid, nil
			}
		}
	}

	dbmovieid, found, found1 := importfeed.MovieFindDbIdByTitle(m.Title, strconv.Itoa(m.Year), "parse", false)
	if found || found1 {
		return dbmovieid, nil
	}

	return 0, errors.New("no movie found")
}
