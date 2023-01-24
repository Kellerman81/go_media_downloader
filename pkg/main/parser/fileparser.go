// parser
package parser

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
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

const queryidmoviesbylistname = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
const queryiddbmoviesbyimdb = "select id from dbmovies where imdb_id = ?"
const querylistnamemoviesbyid = "select listname from movies where id = ?"
const queryiddbseriesbytvdbid = "select id from dbseries where thetvdb_id = ?"
const queryidseriesbylistname = "select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE"
const queryidepisodebydbid = "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?"
const querylistnameseriesbyid = "select listname from series where id = ?"

var allQualityPriorities map[string]map[string]int
var allQualityPrioritiesWanted map[string]map[string]int
var allQualityPrioritiesMu = &sync.RWMutex{}
var scanpatterns []regexpattern
var varextended logger.InStringArrayStruct = logger.InStringArrayStruct{Arr: []string{"extended", "extended cut", "extended.cut", "extended-cut"}}
var varproper logger.InStringArrayStruct = logger.InStringArrayStruct{Arr: []string{"proper"}}
var varrepack logger.InStringArrayStruct = logger.InStringArrayStruct{Arr: []string{"repack"}}

func Getallprios() map[string]map[string]int {
	return allQualityPrioritiesWanted
}

func Getcompleteallprios() map[string]map[string]int {
	return allQualityPriorities
}

func loadDBPatterns() {
	if len(scanpatterns) == 0 {
		scanpatterns = []regexpattern{
			{"season", false, `(?i)(s?(\d{1,4}))(?: )?[ex]`, 2},
			{"episode", false, `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`, 2},
			{"identifier", false, `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex]\d{2,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
			{"date", false, `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
			{"year", true, `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, 2},
			{"audio", false, `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`, 2},
			{"imdb", false, `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, 2},
			{"tvdb", false, `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, 2},
		}
		for idx := range scanpatterns {
			logger.GlobalRegexCache.SetRegexp(scanpatterns[idx].re, 0)
		}
		for idx := range database.DBConnect.GetaudiosIn.Arr {
			if database.DBConnect.GetaudiosIn.Arr[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.GetaudiosIn.Arr[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.GetaudiosIn.Arr[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetresolutionsIn.Arr {
			if database.DBConnect.GetresolutionsIn.Arr[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.GetresolutionsIn.Arr[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.GetresolutionsIn.Arr[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetqualitiesIn.Arr {
			if database.DBConnect.GetqualitiesIn.Arr[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.GetqualitiesIn.Arr[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.GetqualitiesIn.Arr[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetcodecsIn.Arr {
			if database.DBConnect.GetcodecsIn.Arr[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.GetcodecsIn.Arr[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.GetcodecsIn.Arr[idx].Regex, 0)
			}
		}
	}
}

func NewCutoffPrio(cfgp *config.MediaTypeConfig, qualityTemplate string) int {
	m := apiexternal.ParseInfo{Quality: config.Cfg.Quality[qualityTemplate].CutoffQuality, Resolution: config.Cfg.Quality[qualityTemplate].CutoffResolution}
	GetPriorityMap(&m, cfgp, qualityTemplate, true, false)
	prio := m.Priority
	m.Close()
	return prio
}

func NewFileParser(filename string, includeYearInTitle bool, typegroup string) (*apiexternal.ParseInfo, error) {
	m := apiexternal.ParseInfo{File: filename}
	var startIndex, endIndex = 0, len(m.File)
	loadDBPatterns()
	cleanName := strings.TrimRight(strings.TrimLeft(m.File, "["), "]")
	cleanName = strings.ReplaceAll(cleanName, "_", " ")

	tolower := strings.ToLower(cleanName)
	if !logger.DisableParserStringMatch {
		m.Parsegroup(tolower, "audio", &database.DBConnect.AudioStrIn)
		m.Parsegroup(tolower, "codec", &database.DBConnect.CodecStrIn)
		m.Parsegroup(tolower, "quality", &database.DBConnect.QualityStrIn)
		m.Parsegroup(tolower, "resolution", &database.DBConnect.ResolutionStrIn)
	}
	m.Parsegroup(tolower, "extended", &varextended)
	m.Parsegroup(tolower, "proper", &varproper)
	m.Parsegroup(tolower, "repack", &varrepack)

	var matchentry = make([]string, 0, 2)
	var index int
	var doSingle bool

	conttt := !strings.Contains(cleanName, "tt")
	conttvdb := !strings.Contains(cleanName, "tvdb")
	lenclean := len(cleanName)
	var matchest [][]string
	for idx := range scanpatterns {
		switch scanpatterns[idx].name {
		case "imdb":
			if typegroup != "movie" {
				continue
			}
			if conttt {
				continue
			}
		case "tvdb":
			if typegroup != "series" {
				continue
			}
			if conttvdb {
				continue
			}
		case "season":
		case "episode":
		case "identifier":
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

		doSingle = true
		if scanpatterns[idx].last && scanpatterns[idx].name == "year" && typegroup == "series" {
			doSingle = false
			matchest = logger.GlobalRegexCache.GetRegexpDirect(scanpatterns[idx].re).FindAllStringSubmatch(cleanName, 10)
			if len(matchest) >= 1 {
				matchentry = matchest[len(matchest)-1]
				// matchest = nil
			} else {
				matchentry = nil
			}
			//matchentry = config.RegexGetLastMatches(scanpatterns[idx].re, cleanName, 10)
		}
		if doSingle {
			matchentry = logger.GlobalRegexCache.GetRegexpDirect(scanpatterns[idx].re).FindStringSubmatch(cleanName)
		}
		if len(matchentry) == 0 {
			continue
		}
		index = strings.Index(cleanName, matchentry[1])
		if !includeYearInTitle || (includeYearInTitle && scanpatterns[idx].name != "year") {
			if index == 0 {
				if len(matchentry[1]) != lenclean && len(matchentry[1]) < endIndex {
					startIndex = len(matchentry[1])
				}
			} else if index < endIndex && index > startIndex {
				endIndex = index
			}
		}
		switch scanpatterns[idx].name {
		case "imdb":
			m.Imdb = matchentry[scanpatterns[idx].getgroup]
		case "tvdb":
			m.Tvdb = strings.TrimPrefix(matchentry[scanpatterns[idx].getgroup], "tvdb")
		case "year":
			m.Year = logger.StringToInt(matchentry[scanpatterns[idx].getgroup])
		case "season":
			m.SeasonStr = matchentry[scanpatterns[idx].getgroup]
			m.Season = logger.StringToInt(m.SeasonStr)
		case "episode":
			m.EpisodeStr = matchentry[scanpatterns[idx].getgroup]
			m.Episode = logger.StringToInt(m.EpisodeStr)
		case "identifier":
			m.Identifier = matchentry[scanpatterns[idx].getgroup]
		case "date":
			m.Date = matchentry[scanpatterns[idx].getgroup]
		case "audio":
			m.Audio = matchentry[scanpatterns[idx].getgroup]
		case "resolution":
			m.Resolution = matchentry[scanpatterns[idx].getgroup]
		case "quality":
			m.Quality = matchentry[scanpatterns[idx].getgroup]
		case "codec":
			m.Codec = matchentry[scanpatterns[idx].getgroup]
		}
	}
	matchest = nil
	matchentry = nil
	if m.Date != "" {
		m.Identifier = m.Date
	} else if m.Identifier == "" && m.SeasonStr != "" && m.EpisodeStr != "" {
		m.Identifier = fmt.Sprintf("S%sE%s", m.SeasonStr, m.EpisodeStr)
	}
	if endIndex < startIndex {
		logger.Log.GlobalLogger.Debug("EndIndex < startindex", zap.Int("start", startIndex), zap.Int("end", endIndex), zap.String("Path", m.File))
		cleanName = m.File[startIndex:]
	} else {
		cleanName = m.File[startIndex:endIndex]
	}
	if strings.Contains(cleanName, "(") {
		cleanName = strings.Split(cleanName, "(")[0]
	}

	cleanName = strings.TrimPrefix(cleanName, "- ")
	if strings.ContainsRune(cleanName, '.') && !strings.ContainsRune(cleanName, ' ') {
		cleanName = strings.ReplaceAll(cleanName, ".", " ")
	} else if strings.ContainsRune(cleanName, '.') && strings.ContainsRune(cleanName, '_') {
		cleanName = strings.ReplaceAll(cleanName, "_", " ")
	}
	m.Title = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cleanName), "-"))

	return &m, nil
}
func NewFileParserNoPt(filename string, includeYearInTitle bool, typegroup string) (apiexternal.ParseInfo, error) {
	m, err := NewFileParser(filename, includeYearInTitle, typegroup)
	return *m, err
}

func GetDbIDs(grouptype string, m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, listname string, allowsearchtitle bool) {
	if grouptype == "movie" {
		if m.Imdb != "" {
			searchimdb := m.Imdb
			if !strings.HasPrefix(searchimdb, "tt") {
				searchimdb = "tt" + searchimdb
			}
			database.QueryColumn(&database.Querywithargs{QueryString: queryiddbmoviesbyimdb, Args: []interface{}{searchimdb}}, &m.DbmovieID)
			if m.DbmovieID == 0 && !strings.HasPrefix(m.Imdb, "tt") {
				var imdblist []database.DbstaticOneStringOneInt
				database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: "select imdb_id, id from dbmovies where imdb_id like ?", Args: []interface{}{"tt%" + m.Imdb}}, &imdblist)
				if len(imdblist) >= 1 {
					for key := range imdblist {
						if imdblist[key].Str == "tt"+m.Imdb || imdblist[key].Str == "tt0"+m.Imdb || imdblist[key].Str == "tt00"+m.Imdb || imdblist[key].Str == "tt000"+m.Imdb || imdblist[key].Str == "tt0000"+m.Imdb {
							m.DbmovieID = uint(imdblist[key].Num)
							break
						}
					}
					imdblist = nil
				}
			}
		}
		if m.DbmovieID == 0 && allowsearchtitle {
			for idx := range cfgp.Lists {
				m.Title = importfeed.StripTitlePrefixPostfix(m.Title, cfgp.Lists[idx].TemplateQuality)
			}
			m.DbmovieID, _, _ = importfeed.MovieFindDbIDByTitle(m.Imdb, m.Title, m.Year, "", false)
		}
		if m.DbmovieID != 0 && listname != "" {
			database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{m.DbmovieID, listname}}, &m.MovieID)
		}

		if m.DbmovieID != 0 && m.MovieID == 0 {
			for idx := range cfgp.Lists {
				database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{m.DbmovieID, cfgp.Lists[idx].Name}}, &m.MovieID)
				if m.MovieID != 0 {
					break
				}
			}
		}
		if m.MovieID == 0 {
			m.DbmovieID = 0
		} else {
			database.QueryColumn(&database.Querywithargs{QueryString: querylistnamemoviesbyid, Args: []interface{}{m.MovieID}}, &m.Listname)
		}
		return
	}

	//Parse series
	if m.Tvdb != "" {
		database.QueryColumn(&database.Querywithargs{QueryString: queryiddbseriesbytvdbid, Args: []interface{}{m.Tvdb}}, &m.DbserieID)
	}
	if m.DbserieID == 0 && (allowsearchtitle || m.Tvdb == "") {
		if m.Year != 0 {
			m.DbserieID, _ = importfeed.FindDbserieByName(fmt.Sprintf("%s (%d)", m.Title, m.Year))
		}
		if m.DbserieID == 0 {
			m.DbserieID, _ = importfeed.FindDbserieByName(m.Title)
		}
	}
	if m.DbserieID == 0 && m.File != "" {
		matched, matched2 := config.RegexGetMatchesStr1Str2("RegexSeriesTitle", filepath.Base(m.File))
		if matched2 != "" {
			m.DbserieID, _ = importfeed.FindDbserieByName(matched)
		}
	}

	if m.DbserieID == 0 {
		logger.Log.GlobalLogger.Debug("NOT Found dbserie for ", zap.Stringp("path", &m.File))
		return
	}
	if listname != "" {
		database.QueryColumn(&database.Querywithargs{QueryString: queryidseriesbylistname, Args: []interface{}{m.DbserieID, listname}}, &m.SerieID)
	}
	if m.SerieID == 0 {
		for idx := range cfgp.Lists {
			database.QueryColumn(&database.Querywithargs{QueryString: queryidseriesbylistname, Args: []interface{}{m.DbserieID, cfgp.Lists[idx].Name}}, &m.SerieID)
			if m.SerieID != 0 {
				break
			}
		}
	}
	if m.SerieID == 0 {
		m.DbserieEpisodeID = 0
		m.SerieEpisodeID = 0
		logger.Log.GlobalLogger.Debug("NOT Found serie for ", zap.Stringp("path", &m.File))
		return
	}
	m.DbserieEpisodeID, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, m.Identifier, m.SeasonStr, m.EpisodeStr)
	if m.DbserieEpisodeID != 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: queryidepisodebydbid, Args: []interface{}{m.SerieID, m.DbserieEpisodeID}}, &m.SerieEpisodeID)
	}
	database.QueryColumn(&database.Querywithargs{QueryString: querylistnameseriesbyid, Args: []interface{}{m.SerieID}}, &m.Listname)
}

func ParseVideoFile(m *apiexternal.ParseInfo, file string, cfgp *config.MediaTypeConfig, qualityTemplate string) error {
	if m.QualitySet == "" {
		m.QualitySet = qualityTemplate
	}
	result, err := probeURL(file)
	if err != nil {
		return err
	}

	if len(result.Streams) == 0 {
		result.Close()
		return fmt.Errorf("failed to get ffprobe json for <%s> %s", file, err.Error())
	}

	if result.Error.Code != 0 {
		err = fmt.Errorf("ffprobe error code %d: %s", result.Error.Code, result.Error.String)
		result.Close()
		return err
	}
	var duration float64
	duration, err = strconv.ParseFloat(result.Format.Duration, 64)
	if err == nil {
		m.Runtime = int(math.Round(duration))
	}

	m.Languages = []string{}
	var getreso string
	var redetermineprio bool
	for idxstream := range result.Streams {
		if result.Streams[idxstream].CodecType == "audio" {
			if result.Streams[idxstream].Tags.Language != "" {
				m.Languages = append(m.Languages, result.Streams[idxstream].Tags.Language)
			}
			if m.Audio == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Audio) && result.Streams[idxstream].CodecName != "") {
				m.Audio = result.Streams[idxstream].CodecName
				m.AudioID = gettypeids(logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
				redetermineprio = true
			}
			continue
		}
		if result.Streams[idxstream].CodecType != "video" {
			continue
		}
		if result.Streams[idxstream].Height > result.Streams[idxstream].Width {
			result.Streams[idxstream].Height, result.Streams[idxstream].Width = result.Streams[idxstream].Width, result.Streams[idxstream].Height
		}

		if strings.EqualFold(result.Streams[idxstream].CodecName, "mpeg4") && strings.EqualFold(result.Streams[idxstream].CodecTagString, "xvid") {
			result.Streams[idxstream].CodecName = result.Streams[idxstream].CodecTagString
		}
		if m.Codec == "" || (!strings.EqualFold(result.Streams[idxstream].CodecName, m.Codec) && result.Streams[idxstream].CodecName != "") {
			m.Codec = result.Streams[idxstream].CodecName
			m.CodecID = gettypeids(logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
			redetermineprio = true
		}
		getreso = ""
		if result.Streams[idxstream].Height == 360 {
			getreso = "360p"
		} else if result.Streams[idxstream].Height > 1080 {
			getreso = "2160p"
		} else if result.Streams[idxstream].Height > 720 {
			getreso = "1080p"
		} else if result.Streams[idxstream].Height > 576 {
			getreso = "720p"
		} else if result.Streams[idxstream].Height > 480 {
			getreso = "576p"
		} else if result.Streams[idxstream].Height > 368 {
			getreso = "480p"
		} else if result.Streams[idxstream].Height > 360 {
			getreso = "368p"
		}
		if result.Streams[idxstream].Width == 720 {
			getreso = "480p"
		}
		if result.Streams[idxstream].Width == 1280 {
			getreso = "720p"
		}
		if result.Streams[idxstream].Width == 1920 {
			getreso = "1080p"
		}
		if result.Streams[idxstream].Width == 3840 {
			getreso = "2160p"
		}
		m.Height = result.Streams[idxstream].Height
		m.Width = result.Streams[idxstream].Width
		if getreso != "" && (m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
			m.Resolution = getreso
			m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
			redetermineprio = true
		}
	}
	if redetermineprio {
		allQualityPrioritiesMu.Lock()
		prio, ok := allQualityPrioritiesWanted[qualityTemplate][fmt.Sprintf("%d_%d_%d_%d", m.ResolutionID, m.QualityID, m.CodecID, m.AudioID)]
		if ok {
			m.Priority = prio
		}
		allQualityPrioritiesMu.Unlock()
	}
	result.Close()
	return nil
}

func GetPriorityMap(m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, qualityTemplate string, useall bool, checkwanted bool) {

	m.QualitySet = qualityTemplate

	if m.ResolutionID == 0 {
		m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = gettypeids(logger.DisableParserStringMatch, m.Quality, &database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = gettypeids(logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = gettypeids(logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
	}

	if m.ResolutionID == 0 {
		m.ResolutionID = database.InQualitiesRegexArray(cfgp.DefaultResolution, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID == 0 {
		m.QualityID = database.InQualitiesRegexArray(cfgp.DefaultQuality, &database.DBConnect.GetqualitiesIn)
	}

	if m.ResolutionID != 0 {
		for idx := range database.DBConnect.GetresolutionsIn.Arr {
			if database.DBConnect.GetresolutionsIn.Arr[idx].ID == m.ResolutionID {
				m.Resolution = database.DBConnect.GetresolutionsIn.Arr[idx].Name
				break
			}
		}
	}
	if m.QualityID != 0 {
		for idx := range database.DBConnect.GetqualitiesIn.Arr {
			if database.DBConnect.GetqualitiesIn.Arr[idx].ID == m.QualityID {
				m.Quality = database.DBConnect.GetqualitiesIn.Arr[idx].Name
				break
			}
		}
	}

	if len(allQualityPriorities) == 0 {
		GetAllQualityPriorities()
	}
	var reso, qual, aud, codec uint

	if config.Cfg.Quality[qualityTemplate].UseForPriorityResolution || useall {
		reso = m.ResolutionID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityQuality || useall {
		qual = m.QualityID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityAudio || useall {
		aud = m.AudioID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityCodec || useall {
		codec = m.CodecID
	}

	allQualityPrioritiesMu.Lock()
	title := fmt.Sprintf("%d_%d_%d_%d", reso, qual, codec, aud)

	var ok bool
	var prio int
	if checkwanted {
		prio, ok = allQualityPrioritiesWanted[qualityTemplate][title]
	} else {
		prio, ok = allQualityPriorities[qualityTemplate][title]
	}
	allQualityPrioritiesMu.Unlock()
	if !ok {
		m.Priority = 0
		return
	}
	m.Priority = prio
	if !config.Cfg.Quality[qualityTemplate].UseForPriorityOther && !useall {
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
func GetIDPriorityMap(m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, qualityTemplate string, useall bool, checkwanted bool) {
	if m.ResolutionID == 0 && m.Resolution == "" {
		m.Resolution = cfgp.DefaultResolution
	}
	if m.QualityID == 0 && m.Quality == "" {
		m.Quality = cfgp.DefaultQuality
	}
	if m.ResolutionID == 0 && m.Resolution != "" {
		m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 && m.Quality != "" {
		m.QualityID = gettypeids(logger.DisableParserStringMatch, m.Quality, &database.DBConnect.GetqualitiesIn)
	}
	if m.ResolutionID == 0 {
		m.ResolutionID = database.InQualitiesRegexArray(cfgp.DefaultResolution, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID == 0 {
		m.QualityID = database.InQualitiesRegexArray(cfgp.DefaultQuality, &database.DBConnect.GetqualitiesIn)
	}

	if len(allQualityPriorities) == 0 {
		GetAllQualityPriorities()
	}
	var reso, qual, aud, codec uint

	if config.Cfg.Quality[qualityTemplate].UseForPriorityResolution || useall {
		reso = m.ResolutionID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityQuality || useall {
		qual = m.QualityID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityAudio || useall {
		aud = m.AudioID
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityCodec || useall {
		codec = m.CodecID
	}
	allQualityPrioritiesMu.Lock()
	title := fmt.Sprintf("%d_%d_%d_%d", reso, qual, codec, aud)

	var ok bool
	var prio int
	if checkwanted {
		prio, ok = allQualityPrioritiesWanted[qualityTemplate][title]
	} else {
		prio, ok = allQualityPriorities[qualityTemplate][title]
	}
	allQualityPrioritiesMu.Unlock()
	if !ok {
		m.Priority = 0
		logger.Log.GlobalLogger.Debug("prio not found", zap.String("searched for", title), zap.String("in", qualityTemplate), zap.Bool("wanted", checkwanted))
		return
	}
	m.Priority = prio
	if !config.Cfg.Quality[qualityTemplate].UseForPriorityOther && !useall {
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

func getIDPrioritySimple(m *apiexternal.ParseInfo, qualityTemplate string, reordergroup *config.QualityReorderConfigGroup) int {
	var priores, prioqual, prioaud, priocodec int
	if m.ResolutionID != 0 {
		priores = gettypeidpriority(m, m.ResolutionID, "resolution", true, reordergroup, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID != 0 {
		prioqual = gettypeidpriority(m, m.QualityID, "quality", true, reordergroup, &database.DBConnect.GetqualitiesIn)
	}
	if m.CodecID != 0 {
		priocodec = gettypeidpriority(m, m.CodecID, "codec", true, reordergroup, &database.DBConnect.GetcodecsIn)
	}
	if m.AudioID != 0 {
		prioaud = gettypeidpriority(m, m.AudioID, "audio", true, reordergroup, &database.DBConnect.GetaudiosIn)
	}
	var namearr []string
	for idxreorder := range reordergroup.Arr {
		if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "combined_res_qual") {
			namearr = strings.Split(reordergroup.Arr[idxreorder].Name, ",")

			if len(namearr) != 2 {
				continue
			} else if strings.EqualFold(namearr[0], m.Resolution) && strings.EqualFold(namearr[1], m.Quality) {
				priores = reordergroup.Arr[idxreorder].Newpriority
				prioqual = 0
			}
		}
	}
	namearr = nil

	return priores + prioqual + priocodec + prioaud
}

func GetAllQualityPriorities() {
	allQualityPrioritiesMu.Lock()

	getresolutions := append(database.DBConnect.GetresolutionsIn.Arr, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getqualities := append(database.DBConnect.GetqualitiesIn.Arr, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getaudios := append(database.DBConnect.GetaudiosIn.Arr, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getcodecs := append(database.DBConnect.GetcodecsIn.Arr, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	var parse apiexternal.ParseInfo
	reordergroup := config.QualityReorderConfigGroup{}
	lenmap := len(getresolutions) * len(getqualities) * len(getaudios) * len(getcodecs)
	allQualityPriorities = make(map[string]map[string]int, len(config.Cfg.Quality))
	allQualityPrioritiesWanted = make(map[string]map[string]int, len(config.Cfg.Quality))

	//var mapPriorities map[string]int
	//var mapPrioritieswanted map[string]int

	var qualconf config.QualityConfig
	var setprio int
	var str4 string
	var addwanted bool
	for idxqualroot := range config.Cfg.Quality {
		qualconf = config.Cfg.Quality[idxqualroot]
		reordergroup.Arr = qualconf.QualityReorder
		allQualityPriorities[qualconf.Name] = make(map[string]int, lenmap)
		allQualityPrioritiesWanted[qualconf.Name] = make(map[string]int, lenmap)
		for idxreso := range getresolutions {
			addwanted = true
			if !logger.InStringArray(getresolutions[idxreso].Name, &qualconf.WantedResolutionIn) && database.DBLogLevel == "debug" {
				logger.Log.Debug("unwanted res: ", qualconf.Name, " ", parse.Resolution, " ", qualconf.WantedResolutionIn)
				addwanted = false
			}
			parse = apiexternal.ParseInfo{}
			parse.Resolution = getresolutions[idxreso].Name
			parse.ResolutionID = getresolutions[idxreso].ID

			for idxqual := range getqualities {
				if !logger.InStringArray(getqualities[idxqual].Name, &qualconf.WantedQualityIn) && database.DBLogLevel == "debug" {
					logger.Log.Debug("unwanted qual: ", qualconf.Name, " ", parse.Resolution, " ", parse.Quality, " ", qualconf.WantedQualityIn)
					addwanted = false
				}

				parse.Quality = getqualities[idxqual].Name
				parse.QualityID = getqualities[idxqual].ID
				for idxcodec := range getcodecs {
					parse.Codec = getcodecs[idxcodec].Name
					parse.CodecID = getcodecs[idxcodec].ID
					for idxaudio := range getaudios {
						parse.Audio = getaudios[idxaudio].Name
						parse.AudioID = getaudios[idxaudio].ID

						str4 = fmt.Sprintf("%d_%d_%d_%d", getresolutions[idxreso].ID, getqualities[idxqual].ID, getcodecs[idxcodec].ID, getaudios[idxaudio].ID)
						setprio = getIDPrioritySimple(&parse, qualconf.Name, &reordergroup)
						allQualityPriorities[qualconf.Name][str4] = setprio
						if addwanted {
							allQualityPrioritiesWanted[qualconf.Name][str4] = setprio
						}
					}
				}
			}
		}
	}

	parse.Close()
	qualconf.Close()
	//tempmap = nil
	reordergroup.Close()
	getaudios = nil
	getcodecs = nil
	getqualities = nil
	getresolutions = nil
	allQualityPrioritiesMu.Unlock()
}

func gettypeids(disableParserStringMatch bool, inval string, qualitytype *database.InQualitiesArray) uint {
	var id uint
	tolower := strings.ToLower(inval)
	var index, substrpostLen, lenstr int
	var substrpre, substrpost string
	var isokpost, isokpre bool
	lenval := len(inval)
	var runev rune
	for idx := range qualitytype.Arr {
		id = 0
		lenstr = len(qualitytype.Arr[idx].Strings)
		if lenstr >= 1 && !disableParserStringMatch && strings.Contains(qualitytype.Arr[idx].StringsLower, tolower) {
			index = strings.Index(qualitytype.Arr[idx].StringsLower, tolower)
			substrpre = ""
			if index >= 1 {
				substrpre = qualitytype.Arr[idx].StringsLower[index-1 : index]
			}
			substrpostLen = index + lenval + 1
			if lenstr < substrpostLen {
				substrpostLen = index + lenval
			}
			substrpost = qualitytype.Arr[idx].StringsLower[index+lenval : substrpostLen]
			isokpost = true
			isokpre = true
			if substrpost != "" {
				runev = []rune(substrpost)[0]
				if unicode.IsDigit(runev) || unicode.IsLetter(runev) {
					isokpost = false
				}
			}
			if substrpre != "" {
				runev = []rune(substrpre)[0]
				if unicode.IsDigit(runev) || unicode.IsLetter(runev) {
					isokpre = false
				}
			}
			if isokpre && isokpost && qualitytype.Arr[idx].ID != 0 {
				id = qualitytype.Arr[idx].ID
				break
			}
		}
		if len(qualitytype.Arr[idx].Regex) >= 1 && qualitytype.Arr[idx].UseRegex && config.RegexGetMatchesFind(qualitytype.Arr[idx].Regex, tolower, 2) {
			id = qualitytype.Arr[idx].ID
			break
		}
	}
	return id
}
func gettypeidpriority(m *apiexternal.ParseInfo, id uint, qualitystringtype string, setprioonly bool, reordergroup *config.QualityReorderConfigGroup, qualitytype *database.InQualitiesArray) int {
	var priority int
	var name string
	for qualidx := range qualitytype.Arr {
		if qualitytype.Arr[qualidx].ID != id {
			continue
		}
		name = qualitytype.Arr[qualidx].Name
		priority = qualitytype.Arr[qualidx].Priority
		for idxreorder := range reordergroup.Arr {
			if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, qualitystringtype) && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualitytype.Arr[qualidx].Name) {
				priority = reordergroup.Arr[idxreorder].Newpriority
			}
			if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "position") && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualitystringtype) {
				priority *= reordergroup.Arr[idxreorder].Newpriority
			}
		}

		switch qualitystringtype {
		case "resolution":
			if !setprioonly {
				m.Resolution = name
			}
			return priority
		case "quality":
			if !setprioonly {
				m.Quality = name
			}
			return priority
		case "codec":
			if !setprioonly {
				m.Codec = name
			}
			return priority
		case "audio":
			if !setprioonly {
				m.Audio = name
			}
			return priority
		}

		return 0
	}
	return 0
}
