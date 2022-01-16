// parser
package parser

import (
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

func (n *ParseInfo) StripTitlePrefixPostfix(qualityTemplate string) {
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

func before(value string, index int) string {
	if index <= 0 {
		return ""
	}
	return value[index-1 : index]
}

func after(value string, index int) string {
	if index >= len(value) {
		return ""
	}
	return value[index : index+1]
}

func (m *ParseInfo) parsegroup(cleanName string, name string, group []string, startIndex int, endIndex int) (int, int) {
	tolower := strings.ToLower(cleanName)
	for idx := range group {
		if strings.Contains(tolower, group[idx]) {
			index := strings.Index(tolower, group[idx])
			substr := cleanName[index : index+len(group[idx])]
			substrpre := before(cleanName, index)
			substrpost := after(cleanName, index+len(group[idx]))
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
		return nil
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !cfg_general.DisableParserStringMatch {
		audio := []string{}
		for idx := range database.Getaudios {
			audio = append(audio, strings.Split(database.Getaudios[idx].Strings, ",")...)
		}
		startIndex, endIndex = m.parsegroup(cleanName, "audio", audio, startIndex, endIndex)

		codec := []string{}
		for idx := range database.Getcodecs {
			codec = append(codec, strings.Split(database.Getcodecs[idx].Strings, ",")...)
		}
		startIndex, endIndex = m.parsegroup(cleanName, "codec", codec, startIndex, endIndex)

		quality := []string{}
		for idx := range database.Getqualities {
			quality = append(quality, strings.Split(database.Getqualities[idx].Strings, ",")...)
		}
		startIndex, endIndex = m.parsegroup(cleanName, "quality", quality, startIndex, endIndex)

		resolution := []string{}
		for idx := range database.Getresolutions {
			resolution = append(resolution, strings.Split(database.Getresolutions[idx].Strings, ",")...)
		}
		startIndex, endIndex = m.parsegroup(cleanName, "resolution", resolution, startIndex, endIndex)
	}
	extended := []string{"extended", "extended cut", "extended.cut", "extended-cut", "extended_cut"}
	startIndex, endIndex = m.parsegroup(cleanName, "extended", extended, startIndex, endIndex)

	proper := []string{"proper"}
	startIndex, endIndex = m.parsegroup(cleanName, "proper", proper, startIndex, endIndex)

	repack := []string{"repack"}
	startIndex, endIndex = m.parsegroup(cleanName, "repack", repack, startIndex, endIndex)

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
			mint, _ := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
			m.Year = mint
		case "season":
			mint, _ := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
			m.Season = mint
			m.SeasonStr = matches[matchIdx][scanpatterns[idxpattern].getgroup]
		case "episode":
			mint, _ := strconv.Atoi(matches[matchIdx][scanpatterns[idxpattern].getgroup])
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
		} else {
			raw = strstart
		}
	} else {
		if strings.Contains(m.File[startIndex:endIndex], "(") {
			rawarr := strings.Split(m.File[startIndex:endIndex], "(")
			if len(rawarr) >= 1 {
				raw = rawarr[0]
			} else {
				raw = strings.Repeat(m.File[startIndex:endIndex], 1)
			}
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
	cleanName = strings.TrimSuffix(cleanName, " -")
	m.Title = strings.TrimSpace(cleanName)

	return nil
}

func (m *ParseInfo) GetPriority(configTemplate string, qualityTemplate string) {
	if m.Priority != 0 && m.Resolution != "" && m.ResolutionID != 0 && m.Prio_resolution != 0 && m.Quality != "" && m.QualityID != 0 && m.Prio_quality != 0 {
		return
	}
	m.QualitySet = qualityTemplate

	resolution_priority := 0
	quality_priority := 0
	codec_priority := 0
	audio_priority := 0

	typeid, resolution_priority, newname := gettypepriority(m.Resolution, "resolution", qualityTemplate)
	if typeid != 0 {
		m.Resolution = newname
		m.ResolutionID = typeid
		m.Prio_resolution = resolution_priority
	} else {
		m.Resolution = ""
	}

	typeid, quality_priority, newname = gettypepriority(m.Quality, "quality", qualityTemplate)
	if typeid != 0 {
		m.Quality = newname
		m.QualityID = typeid
		m.Prio_quality = quality_priority
	} else {
		m.Quality = ""
	}

	typeid, codec_priority, newname = gettypepriority(m.Codec, "codec", qualityTemplate)
	if typeid != 0 {
		m.Codec = newname
		m.CodecID = typeid
		m.Prio_codec = codec_priority
	} else {
		m.Codec = ""
	}

	typeid, audio_priority, newname = gettypepriority(m.Audio, "audio", qualityTemplate)
	if typeid != 0 {
		m.Audio = newname
		m.AudioID = typeid
		m.Prio_audio = audio_priority
	} else {
		m.Audio = ""
	}

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	typeid, type_priority, newname := getdefaulttypepriority(configEntry.DefaultQuality, "quality", m.QualityID, qualityTemplate)
	if typeid != 0 {
		m.Quality = newname
		m.QualityID = typeid
		quality_priority = type_priority
		m.Prio_quality = type_priority
	}

	typeid, type_priority, newname = getdefaulttypepriority(configEntry.DefaultResolution, "resolution", m.ResolutionID, qualityTemplate)
	if typeid != 0 {
		m.Resolution = newname
		m.ResolutionID = typeid
		resolution_priority = type_priority
		m.Prio_resolution = type_priority
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
		logger.Log.Debug("Parsed Video as Audio: ", video.AudioCodec)
		logger.Log.Debug("Parsed Video as Codec: ", video.VideoCodec)
		logger.Log.Debug("Parsed Video as Height: ", video.Height)
		m.Runtime = int(video.Duration)
		if m.Audio == "" || (!strings.EqualFold(video.AudioCodec, m.Audio) && video.AudioCodec != "") {
			typeid, audio_priority, newname := gettypepriority(video.AudioCodec, "audio", qualityTemplate)
			if typeid != 0 {
				logger.Log.Debug("Changed Audio from ", m.Audio, " to ", video.AudioCodec)
				m.Audio = newname
				m.AudioID = typeid
				m.Prio_audio = audio_priority
			}
		}
		if strings.EqualFold(video.VideoCodec, "mpeg4") && strings.EqualFold(video.VideoCodecTagString, "XVID") {
			video.VideoCodec = video.VideoCodecTagString
		}
		if m.Codec == "" || (!strings.EqualFold(video.VideoCodec, m.Codec) && video.VideoCodec != "") {
			typeid, codec_priority, newname := gettypepriority(video.VideoCodec, "codec", qualityTemplate)
			if typeid != 0 {
				logger.Log.Debug("Changed Codec from ", m.Codec, " to ", video.VideoCodec)
				m.Codec = newname
				m.CodecID = typeid
				m.Prio_codec = codec_priority
			}
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
			typeid, resolution_priority, newname := gettypepriority(getreso, "resolution", qualityTemplate)
			if typeid != 0 {
				logger.Log.Debug("Changed Resolution from ", m.Resolution, " to ", getreso)
				m.Resolution = newname
				m.ResolutionID = typeid
				m.Prio_resolution = resolution_priority
			}
		}
		m.Languages = video.AudioLanguages

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

		return nil
	} else {

		return err
	}
}

func (m *ParseInfo) GetIDPriority(configTemplate string, qualityTemplate string) {
	resolution_priority := 0
	quality_priority := 0
	codec_priority := 0
	audio_priority := 0

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if m.ResolutionID != 0 {
		resolution_priority, _ = gettypeidpriority(m.ResolutionID, "resolution", qualityTemplate)
		m.Prio_resolution = resolution_priority
	} else {
		typeid, type_priority, _ := getdefaulttypepriority(configEntry.DefaultResolution, "resolution", m.ResolutionID, qualityTemplate)
		if typeid != 0 {
			resolution_priority = type_priority
			m.Prio_resolution = type_priority
		}
	}
	if m.QualityID != 0 {
		quality_priority, _ = gettypeidpriority(m.QualityID, "quality", qualityTemplate)
		m.Prio_quality = quality_priority
	} else {
		typeid, type_priority, _ := getdefaulttypepriority(configEntry.DefaultQuality, "quality", m.QualityID, qualityTemplate)
		if typeid != 0 {
			quality_priority = type_priority
			m.Prio_quality = type_priority
		}
	}
	if m.CodecID != 0 {
		codec_priority, _ = gettypeidpriority(m.CodecID, "codec", qualityTemplate)
		m.Prio_codec = codec_priority
	}
	if m.AudioID != 0 {
		audio_priority, _ = gettypeidpriority(m.AudioID, "audio", qualityTemplate)
		m.Prio_audio = audio_priority
	}

	m.getcombinedpriority(qualityTemplate)

	Priority := m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
	if m.Proper {
		Priority = Priority + 5
	}
	if m.Extended {
		Priority = Priority + 2
	}
	if m.Repack {
		Priority = Priority + 1
	}

	m.Priority = Priority
}
func gettypepriority(inval string, qualitystringtype string, qualityTemplate string) (id uint, priority int, name string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)

	var qualitytype []database.QualitiesRegex
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
	for idxqual := range qualitytype {
		if len(qualitytype[idxqual].Strings) >= 1 {
			if strings.Contains(qualitytype[idxqual].Strings, tolower) {
				index := strings.Index(qualitytype[idxqual].Strings, tolower)
				substrpre := ""
				if index >= 1 {
					substrpre = qualitytype[idxqual].Strings[index-1 : index]
				}
				substrpost_len := index + len(inval) + 1
				if len(qualitytype[idxqual].Strings) < substrpost_len {
					substrpost_len = index + len(inval)
				}
				substrpost := qualitytype[idxqual].Strings[index+len(inval) : substrpost_len]
				isokpost := true
				isokpre := true
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
						for idxreorder := range qualityconfig.QualityReorder {
							if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitytype[idxqual].Name) {
								priority = qualityconfig.QualityReorder[idxreorder].Newpriority
							}
							if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, "position") && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitystringtype) {
								priority = priority * qualityconfig.QualityReorder[idxreorder].Newpriority
							}
						}
					}
					break
				}
			}
		} else {
			if config.RegexCheck(qualitytype[idxqual].Regex) {
				teststr := config.RegexGet(qualitytype[idxqual].Regex).FindStringSubmatch(tolower)
				if len(teststr) >= 2 {
					id = qualitytype[idxqual].ID
					name = qualitytype[idxqual].Name
					priority = qualitytype[idxqual].Priority
					if len(qualityconfig.QualityReorder) >= 1 {
						for idxreorder := range qualityconfig.QualityReorder {
							if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitytype[idxqual].Name) {
								priority = qualityconfig.QualityReorder[idxreorder].Newpriority
							}
							if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, "position") && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitystringtype) {
								priority = priority * qualityconfig.QualityReorder[idxreorder].Newpriority
							}
						}
					}
					break
				}
			}
		}

	}
	return
}
func gettypeidpriority(id uint, qualitystringtype string, qualityTemplate string) (priority int, name string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	var qualitytype []database.QualitiesRegex
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
			name = qualitytype[idxqual].Name
			priority = qualitytype[idxqual].Priority
			if len(qualityconfig.QualityReorder) >= 1 {
				for idxreorder := range qualityconfig.QualityReorder {
					if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitytype[idxqual].Name) {
						priority = qualityconfig.QualityReorder[idxreorder].Newpriority
					}
					if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, "position") && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitystringtype) {
						priority = priority * qualityconfig.QualityReorder[idxreorder].Newpriority
					}
				}
			}
			break
		}
	}
	return
}

func getdefaulttypepriority(qualitystring string, qualitystringtype string, qualityid uint, qualityTemplate string) (id uint, priority int, name string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	var qualitytype []database.QualitiesRegex
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
					for idxreorder := range qualityconfig.QualityReorder {
						if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, qualitystringtype) && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitytype[idxqual].Name) {
							priority = qualityconfig.QualityReorder[idxreorder].Newpriority
						}
						if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, "position") && strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Name, qualitystringtype) {
							priority = priority * qualityconfig.QualityReorder[idxreorder].Newpriority
						}
					}
				}
			}
		}
	}
	return
}

func (m *ParseInfo) getcombinedpriority(qualityTemplate string) {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	if len(qualityconfig.QualityReorder) >= 1 {
		for idxreorder := range qualityconfig.QualityReorder {
			if strings.EqualFold(qualityconfig.QualityReorder[idxreorder].Type, "combined_res_qual") {
				namearr := strings.Split(qualityconfig.QualityReorder[idxreorder].Name, ",")

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

func GetSerieDBPriority(serieepisodefile database.SerieEpisodeFile, configTemplate string, qualityTemplate string) int {
	resolution_priority := 0
	quality_priority := 0
	codec_priority := 0
	audio_priority := 0
	resolution_name := ""
	quality_name := ""
	audio_name := ""
	codec_name := ""

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if serieepisodefile.ResolutionID != 0 {
		resolution_priority, resolution_name = gettypeidpriority(serieepisodefile.ResolutionID, "resolution", qualityTemplate)
	} else {
		typeid, type_priority, type_name := getdefaulttypepriority(configEntry.DefaultResolution, "resolution", serieepisodefile.ResolutionID, qualityTemplate)
		if typeid != 0 {
			resolution_priority = type_priority
			resolution_name = type_name
		}
	}
	if serieepisodefile.QualityID != 0 {
		quality_priority, quality_name = gettypeidpriority(serieepisodefile.QualityID, "quality", qualityTemplate)
	} else {
		typeid, type_priority, type_name := getdefaulttypepriority(configEntry.DefaultQuality, "quality", serieepisodefile.QualityID, qualityTemplate)
		if typeid != 0 {
			quality_priority = type_priority
			quality_name = type_name
		}
	}
	if serieepisodefile.CodecID != 0 {
		codec_priority, codec_name = gettypeidpriority(serieepisodefile.CodecID, "codec", qualityTemplate)
	}
	if serieepisodefile.AudioID != 0 {
		audio_priority, audio_name = gettypeidpriority(serieepisodefile.AudioID, "audio", qualityTemplate)
	}

	m := ParseInfo{Resolution: resolution_name, Prio_resolution: resolution_priority, Quality: quality_name, Prio_quality: quality_priority, Codec: codec_name, Prio_codec: codec_priority, Audio: audio_name, Prio_audio: audio_priority}

	m.getcombinedpriority(qualityTemplate)
	Priority := m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
	if serieepisodefile.Proper {
		Priority = Priority + 5
	}
	if serieepisodefile.Extended {
		Priority = Priority + 2
	}
	if serieepisodefile.Repack {
		Priority = Priority + 1
	}
	return Priority
}

func GetMovieDBPriority(moviefile database.MovieFile, configTemplate string, qualityTemplate string) int {
	resolution_priority := 0
	quality_priority := 0
	codec_priority := 0
	audio_priority := 0
	resolution_name := ""
	quality_name := ""
	audio_name := ""
	codec_name := ""

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if moviefile.ResolutionID != 0 {
		resolution_priority, resolution_name = gettypeidpriority(moviefile.ResolutionID, "resolution", qualityTemplate)
	} else {
		typeid, type_priority, type_name := getdefaulttypepriority(configEntry.DefaultResolution, "resolution", moviefile.ResolutionID, qualityTemplate)
		if typeid != 0 {
			resolution_priority = type_priority
			resolution_name = type_name
		}
	}
	if moviefile.QualityID != 0 {
		quality_priority, quality_name = gettypeidpriority(moviefile.QualityID, "quality", qualityTemplate)
	} else {
		typeid, type_priority, type_name := getdefaulttypepriority(configEntry.DefaultQuality, "quality", moviefile.QualityID, qualityTemplate)
		if typeid != 0 {
			quality_priority = type_priority
			quality_name = type_name
		}
	}
	if moviefile.CodecID != 0 {
		codec_priority, codec_name = gettypeidpriority(moviefile.CodecID, "codec", qualityTemplate)
	}
	if moviefile.AudioID != 0 {
		audio_priority, audio_name = gettypeidpriority(moviefile.AudioID, "audio", qualityTemplate)
	}

	m := ParseInfo{Resolution: resolution_name, Prio_resolution: resolution_priority, Quality: quality_name, Prio_quality: quality_priority, Codec: codec_name, Prio_codec: codec_priority, Audio: audio_name, Prio_audio: audio_priority}

	m.getcombinedpriority(qualityTemplate)
	Priority := m.Prio_resolution + m.Prio_quality + m.Prio_codec + m.Prio_audio
	if moviefile.Proper {
		Priority = Priority + 5
	}
	if moviefile.Extended {
		Priority = Priority + 2
	}
	if moviefile.Repack {
		Priority = Priority + 1
	}
	return Priority
}

// Path makes a string safe to use as a URL path,
// removing accents and replacing separators with -.
// The path may still start at / and is not intended
// for use as a file system path without prefix.

func (m *ParseInfo) FindSerieByParser(titleyear string, seriestitle string, listname string) (database.Serie, int) {
	var entriesfound int

	if m.Tvdb != "" {
		counter, _ := database.CountRowsStatic("Select count(id) from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
		if counter == 1 {
			id, _ := database.QueryColumnStatic("Select id from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
			findseries, _ := database.QuerySeries(database.Query{Where: "dbserie_id = ? AND listname = ?", WhereArgs: []interface{}{id, listname}})

			id = nil
			if len(findseries) == 1 {
				entriesfound = len(findseries)
				return findseries[0], entriesfound
			}
		}
	}
	if entriesfound == 0 && titleyear != "" {
		foundserie, foundentries := importfeed.Findseriebyname(titleyear, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		foundserie, foundentries := importfeed.Findseriebyname(seriestitle, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && m.Title != "" {
		foundserie, foundentries := importfeed.Findseriebyname(m.Title, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && titleyear != "" {
		foundserie, foundentries := importfeed.Findseriebyalternatename(titleyear, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		foundserie, foundentries := importfeed.Findseriebyalternatename(seriestitle, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && m.Title != "" {
		foundserie, foundentries := importfeed.Findseriebyalternatename(m.Title, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	return database.Serie{}, 0
}
