// parser
package parser

import (
	"errors"
	"path/filepath"
	"regexp"
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
	"go.uber.org/zap/buffer"
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

var allQualityPriorities map[string]map[string]int
var allQualityPrioritiesMu *sync.RWMutex = &sync.RWMutex{}

func Getallprios() map[string]map[string]int {
	return allQualityPriorities
}
func loadDBPatterns() []regexpattern {
	value, found := logger.GlobalCache.Get("scanpatterns")
	if !found {
		var scanpatterns = []regexpattern{
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
			logger.GlobalRegexCache.SetRegexp(scanpatterns[idx].re, regexp.MustCompile(scanpatterns[idx].re), 0)
		}
		for idx := range database.DBConnect.Getaudios {
			if database.DBConnect.Getaudios[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.Getaudios[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getaudios[idx].Regex, regexp.MustCompile(database.DBConnect.Getaudios[idx].Regex), 0)
			}
		}
		for idx := range database.DBConnect.Getresolutions {
			if database.DBConnect.Getresolutions[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.Getresolutions[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getresolutions[idx].Regex, regexp.MustCompile(database.DBConnect.Getresolutions[idx].Regex), 0)
			}
		}
		for idx := range database.DBConnect.Getqualities {
			if database.DBConnect.Getqualities[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.Getqualities[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getqualities[idx].Regex, regexp.MustCompile(database.DBConnect.Getqualities[idx].Regex), 0)
			}
		}
		for idx := range database.DBConnect.Getcodecs {
			if database.DBConnect.Getcodecs[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.Getcodecs[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getcodecs[idx].Regex, regexp.MustCompile(database.DBConnect.Getcodecs[idx].Regex), 0)
			}
		}
		logger.GlobalCache.Set("scanpatterns", scanpatterns, 0)
		return scanpatterns
	} else {
		return value.Value.([]regexpattern)
	}
}

func NewCutoffPrio(cfg string, qualityTemplate string) int {
	m := apiexternal.ParseInfo{Quality: config.Cfg.Quality[qualityTemplate].Cutoff_quality, Resolution: config.Cfg.Quality[qualityTemplate].Cutoff_resolution}
	GetPriorityMap(&m, cfg, qualityTemplate, true)
	prio := m.Priority
	m.Close()
	return prio
}

func NewFileParser(filename string, includeYearInTitle bool, typegroup string) (*apiexternal.ParseInfo, error) {
	var m apiexternal.ParseInfo
	m.File = filename
	err := ParseFile(&m, includeYearInTitle, typegroup)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
func NewFileParserNoPt(filename string, includeYearInTitle bool, typegroup string) (apiexternal.ParseInfo, error) {
	m, err := NewFileParser(filename, includeYearInTitle, typegroup)
	return *m, err
}

var errNotAdded error = errors.New("not added")
var errNoMatch error = errors.New("no match")
var errNoRow error = errors.New("no row")
var errNotFound error = errors.New("not found")

func AddMovieIfNotFound(m *apiexternal.ParseInfo, listname string, cfg string) (movie uint, err error) {
	if len(m.Imdb) >= 1 {
		importfeed.JobImportMovies(m.Imdb, cfg, listname, true)
		movie, err = database.QueryColumnUint("select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", m.Imdb, listname)
		if err == nil {
			return
		}
	}
	configEntryData := config.Cfg.Media[cfg].Data[0]

	dbmovie, found, found1 := importfeed.MovieFindDbIdByTitle(m.Imdb, m.Title, strconv.Itoa(m.Year), "rss", configEntryData.AddFound)
	if found || found1 {
		if listname == configEntryData.AddFoundList && configEntryData.AddFound {
			_, err = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", dbmovie, listname)
			if err != nil {
				imdbID, _ := database.QueryColumnString("select imdb_id from dbmovies where id = ?", dbmovie)
				importfeed.JobImportMovies(imdbID, cfg, listname, true)
				movie, err = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", dbmovie, listname)
				if err == nil {
					return
				}
				return 0, errNotAdded
			}
		}
	} else if listname == configEntryData.AddFoundList && configEntryData.AddFound {
		var imdbID string
		imdbID, _, _ = importfeed.MovieFindImdbIDByTitle(m.Title, strconv.Itoa(m.Year), "rss", configEntryData.AddFound)
		importfeed.JobImportMovies(imdbID, cfg, listname, true)
		movie, err = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", dbmovie, listname)
		if err == nil {
			return
		}
		return 0, errNotAdded
	}
	return 0, errNoMatch
}

func ParseFile(m *apiexternal.ParseInfo, includeYearInTitle bool, typegroup string) error {
	patterns := loadDBPatterns()

	var startIndex, endIndex = 0, len(m.File)

	cleanName := strings.TrimRight(strings.TrimLeft(m.File, "["), "]")
	if strings.Contains(cleanName, "_") {
		cleanName = strings.Replace(cleanName, "_", " ", -1)
	}

	tolower := strings.ToLower(cleanName)
	if !logger.DisableParserStringMatch {
		m.Parsegroup(tolower, "audio", database.DBConnect.AudioStr)
		m.Parsegroup(tolower, "codec", database.DBConnect.CodecStr)
		m.Parsegroup(tolower, "quality", database.DBConnect.QualityStr)
		m.Parsegroup(tolower, "resolution", database.DBConnect.ResolutionStr)
	}
	m.Parsegroup(tolower, "extended", []string{"extended", "extended cut", "extended.cut", "extended-cut", "extended_cut"})
	m.Parsegroup(tolower, "proper", []string{"proper"})
	m.Parsegroup(tolower, "repack", []string{"repack"})

	var err error
	var matchentry []string = make([]string, 0, 2)
	defer logger.ClearVar(&matchentry)
	var index int
	var do_single bool

	for idx := range patterns {
		switch patterns[idx].name {
		case "imdb":
			if typegroup != "movie" {
				continue
			}
			if !strings.Contains(cleanName, "tt") {
				continue
			}
		case "tvdb":
			if typegroup != "series" {
				continue
			}
			if !strings.Contains(cleanName, "tvdb") {
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

		do_single = true
		if patterns[idx].last {
			if patterns[idx].name != "year" && typegroup != "series" {
			} else {
				do_single = false
				matchentry = config.RegexGetLastMatches(patterns[idx].re, cleanName, 10)
			}
		}
		if do_single {
			matchentry = config.RegexGetMatches(patterns[idx].re, cleanName)
		}
		if len(matchentry) == 0 {
			continue
		}
		index = strings.Index(cleanName, matchentry[1])
		if !includeYearInTitle || (includeYearInTitle && patterns[idx].name != "year") {
			if index == 0 {
				if len(matchentry[1]) != len(cleanName) && len(matchentry[1]) < endIndex {
					startIndex = len(matchentry[1])
				}
			} else if index < endIndex && index > startIndex {
				endIndex = index
			}
		}
		switch patterns[idx].name {
		case "imdb":
			m.Imdb = matchentry[patterns[idx].getgroup]
		case "tvdb":
			m.Tvdb = strings.TrimPrefix(matchentry[patterns[idx].getgroup], "tvdb")
		case "year":
			m.Year, err = strconv.Atoi(matchentry[patterns[idx].getgroup])
			if err != nil {
				continue
			}
		case "season":
			m.Season, err = strconv.Atoi(matchentry[patterns[idx].getgroup])
			if err != nil {
				continue
			}
			m.SeasonStr = matchentry[patterns[idx].getgroup]
		case "episode":
			m.Episode, err = strconv.Atoi(matchentry[patterns[idx].getgroup])
			if err != nil {
				continue
			}
			m.EpisodeStr = matchentry[patterns[idx].getgroup]
		case "identifier":
			m.Identifier = matchentry[patterns[idx].getgroup]
		case "date":
			m.Date = matchentry[patterns[idx].getgroup]
		case "audio":
			m.Audio = matchentry[patterns[idx].getgroup]
		case "resolution":
			m.Resolution = matchentry[patterns[idx].getgroup]
		case "quality":
			m.Quality = matchentry[patterns[idx].getgroup]
		case "codec":
			m.Codec = matchentry[patterns[idx].getgroup]
		}
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
		logger.Log.GlobalLogger.Debug("EndIndex < startindex", zap.Int("start", startIndex), zap.Int("end", endIndex), zap.String("Path", m.File))
		raw = m.File[startIndex:]
	} else {
		raw = m.File[startIndex:endIndex]
	}
	if strings.Contains(raw, "(") {
		raw = strings.Split(raw, "(")[0]
	}

	cleanName = strings.TrimPrefix(raw, "- ")
	if strings.ContainsRune(cleanName, '.') && !strings.ContainsRune(cleanName, ' ') {
		cleanName = strings.Replace(cleanName, ".", " ", -1)
	} else if strings.ContainsRune(cleanName, '.') && strings.ContainsRune(cleanName, '_') {
		cleanName = strings.Replace(cleanName, "_", " ", -1)
	}
	m.Title = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(cleanName), "-"))

	return nil
}

func getSubImdb(imdb string) (uint, error) {

	searchimdb := "tt%" + imdb
	imdblist, _ := database.QueryStaticColumnsOneStringOneInt("select imdb_id as str, id as num from dbmovies where imdb_id like ?", false, 0, searchimdb)
	if len(imdblist) == 0 {
		return 0, errNoRow
	} else {
		for key := range imdblist {
			if imdblist[key].Str == "tt"+imdb || imdblist[key].Str == "tt0"+imdb || imdblist[key].Str == "tt00"+imdb || imdblist[key].Str == "tt000"+imdb || imdblist[key].Str == "tt0000"+imdb {
				return uint(imdblist[key].Num), nil
			}
		}
		return 0, errNoRow
	}
}
func GetDbIDs(grouptype string, m *apiexternal.ParseInfo, cfg string, listname string, allowsearchtitle bool) {
	if grouptype == "movie" {
		if m.Imdb != "" {
			searchimdb := m.Imdb
			if !strings.HasPrefix(searchimdb, "tt") {
				searchimdb = "tt" + searchimdb
			}
			m.DbmovieID, _ = database.QueryColumnUint("select id from dbmovies where imdb_id = ?", searchimdb)
			if m.DbmovieID == 0 {
				if !strings.HasPrefix(m.Imdb, "tt") {
					m.DbmovieID, _ = getSubImdb(m.Imdb)
				}
			}
		}
		if m.DbmovieID == 0 && allowsearchtitle {
			for idx := range config.Cfg.Media[cfg].Lists {
				importfeed.StripTitlePrefixPostfix(&m.Title, config.Cfg.Media[cfg].Lists[idx].Template_quality)
			}
			m.DbmovieID, _, _ = importfeed.MovieFindDbIdByTitle(m.Imdb, m.Title, strconv.Itoa(m.Year), "", false)
		}
		if m.DbmovieID != 0 {
			if listname != "" {
				m.MovieID, _ = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", m.DbmovieID, listname)
			}

			if m.MovieID == 0 {
				for idx := range config.Cfg.Media[cfg].Lists {
					m.MovieID, _ = database.QueryColumnUint("select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", m.DbmovieID, config.Cfg.Media[cfg].Lists[idx].Name)
					if m.MovieID != 0 {
						break
					}
				}
			}
		}
		if m.MovieID == 0 {
			m.DbmovieID = 0
		} else {
			m.Listname, _ = database.QueryColumnString("select listname from movies where id = ?", m.MovieID)
		}
	} else {
		if m.Tvdb != "" {
			m.DbserieID, _ = database.QueryColumnUint("select id from dbseries where thetvdb_id = ?", m.Tvdb)
		}
		if m.DbserieID == 0 && (allowsearchtitle || m.Tvdb == "") {
			if m.Year != 0 {
				m.DbserieID, _ = importfeed.FindDbserieByName(m.Title + " (" + strconv.Itoa(m.Year) + ")")
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
		if m.DbserieID != 0 {
			if listname != "" {
				m.SerieID, _ = database.QueryColumnUint("select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE", m.DbserieID, listname)
			}
			if m.SerieID == 0 {
				for idx := range config.Cfg.Media[cfg].Lists {
					m.SerieID, _ = database.QueryColumnUint("select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE", m.DbserieID, config.Cfg.Media[cfg].Lists[idx].Name)
					if m.SerieID != 0 {
						break
					}
				}
			}
			if m.SerieID != 0 {
				m.DbserieEpisodeID, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, m.Identifier, m.SeasonStr, m.EpisodeStr)
				if m.DbserieEpisodeID != 0 {
					m.SerieEpisodeID, _ = database.QueryColumnUint("select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?", m.SerieID, m.DbserieEpisodeID)
				}
			} else {
				logger.Log.GlobalLogger.Debug("NOT Found serie for ", zap.String("path", m.File))
			}
		} else {
			logger.Log.GlobalLogger.Debug("NOT Found dbserie for ", zap.String("path", m.File))
		}
		if m.SerieID == 0 {
			m.DbserieEpisodeID = 0
			m.SerieEpisodeID = 0
		} else {
			m.Listname, _ = database.QueryColumnString("select listname from series where id = ?", m.SerieID)
		}
	}
}

func ParseVideoFile(m *apiexternal.ParseInfo, file string, cfg string, qualityTemplate string) error {
	if m.QualitySet == "" {
		m.QualitySet = qualityTemplate
	}
	return newVideoFile(m, getFFProbeFilename(), file, false, qualityTemplate)
}

type InQualitiesArray struct {
	Arr []database.QualitiesRegex
}

func InQualitiesRegexArray(target string, str_array *InQualitiesArray) uint {
	for idx := range str_array.Arr {
		if strings.EqualFold(str_array.Arr[idx].Name, target) {
			return str_array.Arr[idx].ID
		}
	}
	str_array = nil
	return 0
}

func GetPriorityMap(m *apiexternal.ParseInfo, cfg string, qualityTemplate string, useall bool) {

	m.QualitySet = qualityTemplate

	if m.ResolutionID == 0 {
		m.ResolutionID = gettypeids(m, logger.DisableParserStringMatch, m.Resolution, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getresolutions})
	}

	if m.QualityID == 0 {
		m.QualityID = gettypeids(m, logger.DisableParserStringMatch, m.Quality, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getqualities})
	}

	if m.CodecID == 0 {
		m.CodecID = gettypeids(m, logger.DisableParserStringMatch, m.Codec, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getcodecs})
	}

	if m.AudioID == 0 {
		m.AudioID = gettypeids(m, logger.DisableParserStringMatch, m.Audio, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getaudios})
	}

	if m.ResolutionID == 0 {
		m.ResolutionID = InQualitiesRegexArray(config.Cfg.Media[cfg].DefaultResolution, &InQualitiesArray{Arr: database.DBConnect.Getresolutions})
	}
	if m.QualityID == 0 {
		m.QualityID = InQualitiesRegexArray(config.Cfg.Media[cfg].DefaultQuality, &InQualitiesArray{Arr: database.DBConnect.Getqualities})
	}

	if m.ResolutionID != 0 {
		for idx := range database.DBConnect.Getresolutions {
			if database.DBConnect.Getresolutions[idx].ID == m.ResolutionID {
				m.Resolution = database.DBConnect.Getresolutions[idx].Name
				break
			}
		}
	}
	if m.QualityID != 0 {
		for idx := range database.DBConnect.Getqualities {
			if database.DBConnect.Getqualities[idx].ID == m.QualityID {
				m.Quality = database.DBConnect.Getqualities[idx].Name
				break
			}
		}
	}

	if len(allQualityPriorities) == 0 {
		GetAllQualityPriorities()
	}
	reso := "0"
	qual := "0"
	aud := "0"
	codec := "0"

	if config.Cfg.Quality[qualityTemplate].UseForPriorityResolution || useall {
		reso = strconv.Itoa(int(m.ResolutionID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityQuality || useall {
		qual = strconv.Itoa(int(m.QualityID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityAudio || useall {
		aud = strconv.Itoa(int(m.AudioID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityCodec || useall {
		codec = strconv.Itoa(int(m.CodecID))
	}

	allQualityPrioritiesMu.RLock()
	defer allQualityPrioritiesMu.RUnlock()
	title := reso + "_" + qual + "_" + codec + "_" + aud
	if prio, ok := allQualityPriorities[qualityTemplate][title]; ok {
		m.Priority = prio
		if config.Cfg.Quality[qualityTemplate].UseForPriorityOther || useall {
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
	} else {
		logger.Log.GlobalLogger.Error("Prio in Quality not found ", zap.String("wanted quality", qualityTemplate), zap.String("title", title))
	}
}
func GetIDPriorityMap(m *apiexternal.ParseInfo, cfg string, qualityTemplate string, useall bool) {
	if m.ResolutionID == 0 {
		m.ResolutionID = InQualitiesRegexArray(config.Cfg.Media[cfg].DefaultResolution, &InQualitiesArray{Arr: database.DBConnect.Getresolutions})
	}
	if m.QualityID == 0 {
		m.QualityID = InQualitiesRegexArray(config.Cfg.Media[cfg].DefaultQuality, &InQualitiesArray{Arr: database.DBConnect.Getqualities})
	}

	if len(allQualityPriorities) == 0 {
		GetAllQualityPriorities()
	}
	reso := "0"
	qual := "0"
	aud := "0"
	codec := "0"

	if config.Cfg.Quality[qualityTemplate].UseForPriorityResolution || useall {
		reso = strconv.Itoa(int(m.ResolutionID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityQuality || useall {
		qual = strconv.Itoa(int(m.QualityID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityAudio || useall {
		aud = strconv.Itoa(int(m.AudioID))
	}
	if config.Cfg.Quality[qualityTemplate].UseForPriorityCodec || useall {
		codec = strconv.Itoa(int(m.CodecID))
	}
	allQualityPrioritiesMu.RLock()
	defer allQualityPrioritiesMu.RUnlock()
	title := reso + "_" + qual + "_" + codec + "_" + aud
	if prio, ok := allQualityPriorities[qualityTemplate][title]; ok {
		m.Priority = prio
		if config.Cfg.Quality[qualityTemplate].UseForPriorityOther || useall {
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
	} else {
		logger.Log.GlobalLogger.Error("Prio in Quality not found ", zap.String("Quality", qualityTemplate), zap.String("title", title))
	}
}

func getIDPrioritySimple(m *apiexternal.ParseInfo, qualityTemplate string, reordergroup *config.QualityReorderConfigGroup) int {
	var priores, prioqual, prioaud, priocodec int
	if m.ResolutionID != 0 {
		priores = gettypeidpriority(m, m.ResolutionID, "resolution", qualityTemplate, true, reordergroup, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getresolutions})
	}
	if m.QualityID != 0 {
		prioqual = gettypeidpriority(m, m.QualityID, "quality", qualityTemplate, true, reordergroup, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getqualities})
	}
	if m.CodecID != 0 {
		priocodec = gettypeidpriority(m, m.CodecID, "codec", qualityTemplate, true, reordergroup, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getcodecs})
	}
	if m.AudioID != 0 {
		prioaud = gettypeidpriority(m, m.AudioID, "audio", qualityTemplate, true, reordergroup, &database.QualitiesRegexGroup{Arr: database.DBConnect.Getaudios})
	}
	for idxreorder := range reordergroup.Arr {
		if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "combined_res_qual") {
			namearr := strings.Split(reordergroup.Arr[idxreorder].Name, ",")

			if len(namearr) != 2 {
			} else if strings.EqualFold(namearr[0], m.Resolution) && strings.EqualFold(namearr[1], m.Quality) {
				priores = reordergroup.Arr[idxreorder].Newpriority
				prioqual = 0
			}
		}
	}

	return priores + prioqual + priocodec + prioaud
}

func GetAllQualityPriorities() {
	allQualityPrioritiesMu.Lock()
	defer allQualityPrioritiesMu.Unlock()
	allQualityPriorities = make(map[string]map[string]int)

	getresolutions := logger.GrowSliceBy(database.DBConnect.Getresolutions, 1)
	getresolutions = append(getresolutions, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getqualities := logger.GrowSliceBy(database.DBConnect.Getqualities, 1)
	getqualities = append(getqualities, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getaudios := logger.GrowSliceBy(database.DBConnect.Getaudios, 1)
	getaudios = append(getaudios, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getcodecs := logger.GrowSliceBy(database.DBConnect.Getcodecs, 1)
	getcodecs = append(getcodecs, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	var wanted bool
	var parse apiexternal.ParseInfo
	var reordergroup config.QualityReorderConfigGroup
	var mapPriorities map[string]int = make(map[string]int, len(getresolutions)*len(getqualities)*len(getaudios)*len(getcodecs))
	defer logger.ClearMap(&mapPriorities)
	var qualname string

	var buf buffer.Buffer
	var bytereso, byteQual, byteCodec []byte
	for idxqualroot := range config.Cfg.Quality {
		qualname = config.Cfg.Quality[idxqualroot].Name
		reordergroup = config.QualityReorderConfigGroup{Arr: config.Cfg.Quality[idxqualroot].QualityReorder}
		mapPriorities = map[string]int{}

		for idxreso := range getresolutions {
			wanted = apiexternal.Determinewanted(qualname, &logger.InStringArrayStruct{Arr: config.Cfg.Quality[idxqualroot].Wanted_resolution}, getresolutions[idxreso].Name)

			if !wanted {
				logger.Log.GlobalLogger.Debug("resolution unwanted", zap.String("Resolution", getresolutions[idxreso].Name))
				continue
			}
			bytereso = []byte(strconv.Itoa(int(getresolutions[idxreso].ID)))

			for idxqual := range getqualities {
				wanted = apiexternal.Determinewanted(qualname, &logger.InStringArrayStruct{Arr: config.Cfg.Quality[idxqualroot].Wanted_quality}, getqualities[idxqual].Name)

				if !wanted {
					logger.Log.GlobalLogger.Debug("quality unwanted", zap.String("Quality", getqualities[idxqual].Name))
					continue
				}
				byteQual = []byte("_" + strconv.Itoa(int(getqualities[idxqual].ID)))

				for idxcodec := range getcodecs {
					byteCodec = []byte("_" + strconv.Itoa(int(getcodecs[idxcodec].ID)))
					for idxaudio := range getaudios {
						parse = apiexternal.ParseInfo{
							Resolution:   getresolutions[idxreso].Name,
							Quality:      getqualities[idxqual].Name,
							Codec:        getcodecs[idxcodec].Name,
							Audio:        getaudios[idxaudio].Name,
							ResolutionID: getresolutions[idxreso].ID,
							QualityID:    getqualities[idxqual].ID,
							CodecID:      getcodecs[idxcodec].ID,
							AudioID:      getaudios[idxaudio].ID,
						}
						buf.Write(bytereso)
						buf.Write(byteQual)
						buf.Write(byteCodec)
						buf.AppendString("_" + strconv.Itoa(int(getaudios[idxaudio].ID)))
						mapPriorities[buf.String()] = getIDPrioritySimple(&parse, qualname, &reordergroup)
						buf.Reset()
					}
				}
			}
		}
		allQualityPriorities[qualname] = mapPriorities
	}
}

func gettypeids(m *apiexternal.ParseInfo, disableParserStringMatch bool, inval string, qualitytype *database.QualitiesRegexGroup) uint {
	defer qualitytype.Close()
	var id uint
	tolower := strings.ToLower(inval)
	var index, substrpost_len int
	var substrpre, substrpost string
	var isokpost, isokpre bool
	for idx := range qualitytype.Arr {
		id = 0
		if len(qualitytype.Arr[idx].Strings) >= 1 && !disableParserStringMatch {
			if strings.Contains(qualitytype.Arr[idx].StringsLower, tolower) {
				index = strings.Index(qualitytype.Arr[idx].StringsLower, tolower)
				substrpre = ""
				if index >= 1 {
					substrpre = qualitytype.Arr[idx].StringsLower[index-1 : index]
				}
				substrpost_len = index + len(inval) + 1
				if len(qualitytype.Arr[idx].Strings) < substrpost_len {
					substrpost_len = index + len(inval)
				}
				substrpost = qualitytype.Arr[idx].StringsLower[index+len(inval) : substrpost_len]
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
				if isokpre && isokpost && qualitytype.Arr[idx].ID != 0 {
					id = qualitytype.Arr[idx].ID
					break
				}
			}
		}
		if len(qualitytype.Arr[idx].Regex) >= 1 {
			if config.RegexGetMatchesFind(qualitytype.Arr[idx].Regex, tolower, 2) {
				id = qualitytype.Arr[idx].ID
				break
			}
		}
	}
	if id != 0 {
		return id
	}
	return 0
}
func gettypeidpriority(m *apiexternal.ParseInfo, id uint, qualitystringtype string, qualityTemplate string, setprioonly bool, reordergroup *config.QualityReorderConfigGroup, qualitytype *database.QualitiesRegexGroup) int {
	defer qualitytype.Close()
	for qualidx := range qualitytype.Arr {
		priority := 0
		name := ""
		if qualitytype.Arr[qualidx].ID == id {
			name = qualitytype.Arr[qualidx].Name
			priority = qualitytype.Arr[qualidx].Priority
			for idxreorder := range reordergroup.Arr {
				if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, qualitystringtype) && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualitytype.Arr[qualidx].Name) {
					priority = reordergroup.Arr[idxreorder].Newpriority
				}
				if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "position") && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualitystringtype) {
					priority = priority * reordergroup.Arr[idxreorder].Newpriority
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
	}
	return 0
}

// Path makes a string safe to use as a URL path,
// removing accents and replacing separators with -.
// The path may still start at / and is not intended
// for use as a file system path without prefix.

func FindSerieByParser(m *apiexternal.ParseInfo, titleyear string, seriestitle string, listname string) (uint, int, error) {
	var dbserie_id uint
	var counter int
	if m.Tvdb != "" {
		counter, _ = database.CountRowsStatic("select count() from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
		if counter == 1 {
			dbserie_id, _ = database.QueryColumnUint("select id from dbseries where thetvdb_id = ?", strings.Replace(m.Tvdb, "tvdb", "", -1))
		}
	}
	if dbserie_id == 0 && titleyear != "" {
		dbserie_id, _ = importfeed.FindDbserieByName(titleyear)
	}
	if dbserie_id == 0 && seriestitle != "" {
		dbserie_id, _ = importfeed.FindDbserieByName(seriestitle)
	}
	if dbserie_id == 0 && m.Title != "" {
		dbserie_id, _ = importfeed.FindDbserieByName(m.Title)
	}
	if dbserie_id != 0 {
		counter, _ = database.CountRowsStatic("select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE", dbserie_id, listname)
		if counter == 1 {
			serieid, _ := database.QueryColumnUint("select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE", dbserie_id, listname)
			return serieid, 1, nil
		}
	}
	return 0, 0, errNotFound
}

func FindDbmovieByFile(m *apiexternal.ParseInfo) (uint, error) {
	if len(m.Imdb) >= 1 {
		dbmovieid, err := database.QueryColumnUint("select id from dbmovies where imdb_id = ?", m.Imdb)
		if err == nil {
			if dbmovieid != 0 {
				return dbmovieid, nil
			}
		}
	}

	dbmovieid, found, found1 := importfeed.MovieFindDbIdByTitle(m.Imdb, m.Title, strconv.Itoa(m.Year), "parse", false)
	if found || found1 {
		return dbmovieid, nil
	}

	return 0, errNotFound
}
