// parser
package parser

import (
	"errors"
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

var allQualityPriorities map[string]map[string]int
var allQualityPrioritiesWanted map[string]map[string]int
var allQualityPrioritiesMu *sync.RWMutex = &sync.RWMutex{}

func getseriebydbidandlist(dbserieID uint, listname string) (uint, error) {
	return database.QueryColumnUint(database.Querywithargs{QueryString: "select id from series where dbserie_id = ? and listname = ? COLLATE NOCASE", Args: []interface{}{dbserieID, listname}})
}

func Getallprios() map[string]map[string]int {
	return allQualityPrioritiesWanted
}

var scanpatterns []regexpattern

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
			logger.GlobalRegexCache.SetRegexp(scanpatterns[idx].re, scanpatterns[idx].re, 0)
		}
		for idx := range database.DBConnect.Getaudios {
			if database.DBConnect.Getaudios[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.Getaudios[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getaudios[idx].Regex, database.DBConnect.Getaudios[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.Getresolutions {
			if database.DBConnect.Getresolutions[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.Getresolutions[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getresolutions[idx].Regex, database.DBConnect.Getresolutions[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.Getqualities {
			if database.DBConnect.Getqualities[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.Getqualities[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getqualities[idx].Regex, database.DBConnect.Getqualities[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.Getcodecs {
			if database.DBConnect.Getcodecs[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.Getcodecs[idx].Regex, getgroup: 0})
				logger.GlobalRegexCache.SetRegexp(database.DBConnect.Getcodecs[idx].Regex, database.DBConnect.Getcodecs[idx].Regex, 0)
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
	m := new(apiexternal.ParseInfo)
	m.File = filename
	err := ParseFile(m, includeYearInTitle, typegroup)
	if err != nil {
		m = nil
		return nil, err
	}
	return m, nil
}
func NewFileParserNoPt(filename string, includeYearInTitle bool, typegroup string) (apiexternal.ParseInfo, error) {
	m, err := NewFileParser(filename, includeYearInTitle, typegroup)
	return *m, err
}

var errNotAdded error = errors.New("not added")
var errNoMatch error = errors.New("no match")
var errNoRow error = errors.New("no row")
var errNotFound error = errors.New("not found")

func findmoviebyidandlist(dbmovieID uint, listname string) (uint, error) {
	return database.QueryColumnUint(database.Querywithargs{QueryString: "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE", Args: []interface{}{dbmovieID, listname}})
}
func AddMovieIfNotFound(m *apiexternal.ParseInfo, listname string, cfgp *config.MediaTypeConfig) (uint, error) {
	var movie uint
	var err error
	if m.Imdb != "" {
		importfeed.JobImportMovies(m.Imdb, cfgp, listname, true)
		movie, err = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE", Args: []interface{}{m.Imdb, listname}})
		if err == nil {
			return movie, err
		}
	}
	configEntryData := cfgp.Data[0]

	dbmovie, found, found1 := importfeed.MovieFindDbIdByTitle(m.Imdb, m.Title, strconv.Itoa(m.Year), "rss", configEntryData.AddFound)
	if found || found1 {
		if listname == configEntryData.AddFoundList && configEntryData.AddFound {
			_, err = findmoviebyidandlist(dbmovie, listname)
			if err != nil {
				imdbID, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select imdb_id from dbmovies where id = ?", Args: []interface{}{dbmovie}})
				importfeed.JobImportMovies(imdbID, cfgp, listname, true)
				movie, err = findmoviebyidandlist(dbmovie, listname)
				if err == nil {
					return movie, err
				}
				return 0, errNotAdded
			}
		}
	} else if listname == configEntryData.AddFoundList && configEntryData.AddFound {
		var imdbID string
		imdbID, _, _ = importfeed.MovieFindImdbIDByTitle(m.Title, strconv.Itoa(m.Year), "rss", configEntryData.AddFound)
		importfeed.JobImportMovies(imdbID, cfgp, listname, true)
		movie, err = findmoviebyidandlist(dbmovie, listname)
		if err == nil {
			return movie, err
		}
		return 0, errNotAdded
	}
	return 0, errNoMatch
}

func ParseFile(m *apiexternal.ParseInfo, includeYearInTitle bool, typegroup string) error {
	var startIndex, endIndex = 0, len(m.File)
	loadDBPatterns()
	cleanName := strings.TrimRight(strings.TrimLeft(m.File, "["), "]")
	cleanName = strings.Replace(cleanName, "_", " ", -1)

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
	var doSingle bool

	conttt := !strings.Contains(cleanName, "tt")
	conttvdb := !strings.Contains(cleanName, "tvdb")
	lenclean := len(cleanName)
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
		if scanpatterns[idx].last {
			if scanpatterns[idx].name == "year" && typegroup == "series" {
				doSingle = false
				matchentry = config.RegexGetLastMatches(scanpatterns[idx].re, cleanName, 10)
			}
		}
		if doSingle {
			matchentry = config.RegexGetMatches(scanpatterns[idx].re, cleanName)
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
			m.Year, err = strconv.Atoi(matchentry[scanpatterns[idx].getgroup])
			if err != nil {
				continue
			}
		case "season":
			m.Season, err = strconv.Atoi(matchentry[scanpatterns[idx].getgroup])
			if err != nil {
				continue
			}
			m.SeasonStr = matchentry[scanpatterns[idx].getgroup]
		case "episode":
			m.Episode, err = strconv.Atoi(matchentry[scanpatterns[idx].getgroup])
			if err != nil {
				continue
			}
			m.EpisodeStr = matchentry[scanpatterns[idx].getgroup]
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
	if m.Date != "" {
		m.Identifier = m.Date
	} else {
		if m.Identifier == "" && m.SeasonStr != "" && m.EpisodeStr != "" {
			m.Identifier = logger.StringBuild("S", m.SeasonStr, "E", m.EpisodeStr)
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
	imdblist, _ := database.QueryStaticColumnsOneStringOneInt(false, 0, database.Querywithargs{QueryString: "select imdb_id as str, id as num from dbmovies where imdb_id like ?", Args: []interface{}{searchimdb}})
	if len(imdblist) == 0 {
		return 0, errNoRow
	} else {
		defer logger.ClearVar(&imdblist)
		for key := range imdblist {
			if imdblist[key].Str == "tt"+imdb || imdblist[key].Str == "tt0"+imdb || imdblist[key].Str == "tt00"+imdb || imdblist[key].Str == "tt000"+imdb || imdblist[key].Str == "tt0000"+imdb {
				return uint(imdblist[key].Num), nil
			}
		}
		return 0, errNoRow
	}
}
func GetDbIDs(grouptype string, m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, listname string, allowsearchtitle bool) {
	if grouptype == "movie" {
		if m.Imdb != "" {
			searchimdb := m.Imdb
			if !strings.HasPrefix(searchimdb, "tt") {
				searchimdb = "tt" + searchimdb
			}
			m.DbmovieID, _ = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from dbmovies where imdb_id = ?", Args: []interface{}{searchimdb}})
			if m.DbmovieID == 0 {
				if !strings.HasPrefix(m.Imdb, "tt") {
					m.DbmovieID, _ = getSubImdb(m.Imdb)
				}
			}
		}
		if m.DbmovieID == 0 && allowsearchtitle {
			for idx := range cfgp.Lists {
				m.Title = importfeed.StripTitlePrefixPostfix(m.Title, cfgp.Lists[idx].TemplateQuality)
			}
			m.DbmovieID, _, _ = importfeed.MovieFindDbIdByTitle(m.Imdb, m.Title, strconv.Itoa(m.Year), "", false)
		}
		if m.DbmovieID != 0 {
			if listname != "" {
				m.MovieID, _ = findmoviebyidandlist(m.DbmovieID, listname)
			}

			if m.MovieID == 0 {
				for idx := range cfgp.Lists {
					m.MovieID, _ = findmoviebyidandlist(m.DbmovieID, cfgp.Lists[idx].Name)
					if m.MovieID != 0 {
						break
					}
				}
			}
		}
		if m.MovieID == 0 {
			m.DbmovieID = 0
		} else {
			m.Listname, _ = database.QueryColumnString(database.Querywithargs{QueryString: "select listname from movies where id = ?", Args: []interface{}{m.MovieID}})
		}
	} else {
		if m.Tvdb != "" {
			m.DbserieID, _ = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from dbseries where thetvdb_id = ?", Args: []interface{}{m.Tvdb}})
		}
		if m.DbserieID == 0 && (allowsearchtitle || m.Tvdb == "") {
			if m.Year != 0 {
				m.DbserieID, _ = importfeed.FindDbserieByName(logger.StringBuild(m.Title, " (", strconv.Itoa(m.Year), ")"))
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
				m.SerieID, _ = getseriebydbidandlist(m.DbserieID, listname)
			}
			if m.SerieID == 0 {
				for idx := range cfgp.Lists {
					m.SerieID, _ = getseriebydbidandlist(m.DbserieID, cfgp.Lists[idx].Name)
					if m.SerieID != 0 {
						break
					}
				}
			}
			if m.SerieID != 0 {
				m.DbserieEpisodeID, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, m.Identifier, m.SeasonStr, m.EpisodeStr)
				if m.DbserieEpisodeID != 0 {
					m.SerieEpisodeID, _ = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?", Args: []interface{}{m.SerieID, m.DbserieEpisodeID}})
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
			m.Listname, _ = database.QueryColumnString(database.Querywithargs{QueryString: "select listname from series where id = ?", Args: []interface{}{m.SerieID}})
		}
	}
}

func ParseVideoFile(m *apiexternal.ParseInfo, file string, cfgp *config.MediaTypeConfig, qualityTemplate string) error {
	if m.QualitySet == "" {
		m.QualitySet = qualityTemplate
	}
	return newVideoFile(m, getFFProbeFilename(), file, false, qualityTemplate)
}

func GetPriorityMap(m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, qualityTemplate string, useall bool, checkwanted bool) {

	m.QualitySet = qualityTemplate

	if m.ResolutionID == 0 {
		m.ResolutionID = gettypeids(m, logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = gettypeids(m, logger.DisableParserStringMatch, m.Quality, &database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = gettypeids(m, logger.DisableParserStringMatch, m.Codec, &database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = gettypeids(m, logger.DisableParserStringMatch, m.Audio, &database.DBConnect.GetaudiosIn)
	}

	if m.ResolutionID == 0 {
		m.ResolutionID = database.InQualitiesRegexArray(cfgp.DefaultResolution, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID == 0 {
		m.QualityID = database.InQualitiesRegexArray(cfgp.DefaultQuality, &database.DBConnect.GetqualitiesIn)
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
	title := logger.StringBuild(reso, "_", qual, "_", codec, "_", aud)

	ok := false
	prio := 0
	if checkwanted {
		prio, ok = allQualityPrioritiesWanted[qualityTemplate][title]
	} else {
		prio, ok = allQualityPriorities[qualityTemplate][title]
	}
	if ok {
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
	} //else {
	//	logger.Log.GlobalLogger.Error("Prio in Quality not found ", zap.String("wanted quality", qualityTemplate), zap.String("title", title))
	//}
}
func GetIDPriorityMap(m *apiexternal.ParseInfo, cfgp *config.MediaTypeConfig, qualityTemplate string, useall bool, checkwanted bool) {
	if m.ResolutionID == 0 {
		m.ResolutionID = database.InQualitiesRegexArray(cfgp.DefaultResolution, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID == 0 {
		m.QualityID = database.InQualitiesRegexArray(cfgp.DefaultQuality, &database.DBConnect.GetqualitiesIn)
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
	title := logger.StringBuild(reso, "_", qual, "_", codec, "_", aud)

	ok := false
	prio := 0
	if checkwanted {
		prio, ok = allQualityPrioritiesWanted[qualityTemplate][title]
	} else {
		prio, ok = allQualityPriorities[qualityTemplate][title]
	}
	if ok {
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
	} //else {
	//	logger.Log.GlobalLogger.Error("Prio in Quality not found ", zap.String("Quality", qualityTemplate), zap.String("title", title))
	//}
}

func getIDPrioritySimple(m *apiexternal.ParseInfo, qualityTemplate string, reordergroup *config.QualityReorderConfigGroup) int {
	var priores, prioqual, prioaud, priocodec int
	if m.ResolutionID != 0 {
		priores = gettypeidpriority(m, m.ResolutionID, "resolution", qualityTemplate, true, reordergroup, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID != 0 {
		prioqual = gettypeidpriority(m, m.QualityID, "quality", qualityTemplate, true, reordergroup, &database.DBConnect.GetqualitiesIn)
	}
	if m.CodecID != 0 {
		priocodec = gettypeidpriority(m, m.CodecID, "codec", qualityTemplate, true, reordergroup, &database.DBConnect.GetcodecsIn)
	}
	if m.AudioID != 0 {
		prioaud = gettypeidpriority(m, m.AudioID, "audio", qualityTemplate, true, reordergroup, &database.DBConnect.GetaudiosIn)
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
	defer allQualityPrioritiesMu.Unlock()

	getresolutions := logger.GrowSliceBy(database.DBConnect.Getresolutions, 1)
	getresolutions = append(getresolutions, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getqualities := logger.GrowSliceBy(database.DBConnect.Getqualities, 1)
	getqualities = append(getqualities, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getaudios := logger.GrowSliceBy(database.DBConnect.Getaudios, 1)
	getaudios = append(getaudios, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getcodecs := logger.GrowSliceBy(database.DBConnect.Getcodecs, 1)
	getcodecs = append(getcodecs, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	var parse *apiexternal.ParseInfo
	reordergroup := &config.QualityReorderConfigGroup{}
	lenmap := len(getresolutions) * len(getqualities) * len(getaudios) * len(getcodecs)
	allQualityPriorities = make(map[string]map[string]int, len(config.Cfg.Quality))
	allQualityPrioritiesWanted = make(map[string]map[string]int, len(config.Cfg.Quality))

	var mapPriorities map[string]int = make(map[string]int, lenmap)
	defer logger.ClearMap(&mapPriorities)
	var mapPrioritieswanted map[string]int = make(map[string]int, lenmap)
	defer logger.ClearMap(&mapPrioritieswanted)
	var qualname string

	var qualconf config.QualityConfig
	var addwantedres, addwantedqual bool
	setprio := 0
	var str1, str2, str3, str4 string
	parseclear := &apiexternal.ParseInfo{}
	for idxqualroot := range config.Cfg.Quality {
		qualconf = config.Cfg.Quality[idxqualroot]
		qualname = config.Cfg.Quality[idxqualroot].Name
		reordergroup.Arr = config.Cfg.Quality[idxqualroot].QualityReorder
		//mapPriorities = nil
		//mapPriorities = make(map[string]int, lenmap)
		//mapPrioritieswanted = nil
		//mapPrioritieswanted = make(map[string]int, lenmap)
		for re := range mapPriorities {
			delete(mapPriorities, re)
		}
		for re := range mapPrioritieswanted {
			delete(mapPrioritieswanted, re)
		}
		parse = parseclear
		for idxreso := range getresolutions {
			addwantedres = logger.InStringArray(getresolutions[idxreso].Name, &qualconf.WantedResolutionIn)

			str1 = strconv.Itoa(int(getresolutions[idxreso].ID))
			parse.Resolution = getresolutions[idxreso].Name
			parse.ResolutionID = getresolutions[idxreso].ID
			if !addwantedres {
				logger.Log.Debug("unwanted res: ", qualname, " ", parse.Resolution, " ", qualconf.WantedResolutionIn)
			}

			for idxqual := range getqualities {
				addwantedqual = logger.InStringArray(getqualities[idxqual].Name, &qualconf.WantedQualityIn)

				str2 = "_" + strconv.Itoa(int(getqualities[idxqual].ID))

				parse.Quality = getqualities[idxqual].Name
				parse.QualityID = getqualities[idxqual].ID
				if !addwantedqual {
					logger.Log.Debug("unwanted qual: ", qualname, " ", parse.Resolution, " ", parse.Quality, " ", qualconf.WantedQualityIn)
				}
				for idxcodec := range getcodecs {
					str3 = "_" + strconv.Itoa(int(getcodecs[idxcodec].ID))
					parse.Codec = getcodecs[idxcodec].Name
					parse.CodecID = getcodecs[idxcodec].ID
					for idxaudio := range getaudios {
						parse.Audio = getaudios[idxaudio].Name
						parse.AudioID = getaudios[idxaudio].ID

						str4 = logger.StringBuild(str1, str2, str3, "_", strconv.Itoa(int(getaudios[idxaudio].ID)))
						setprio = getIDPrioritySimple(parse, qualname, reordergroup)
						mapPriorities[str4] = setprio
						if addwantedres && addwantedqual {
							mapPrioritieswanted[str4] = setprio
						}
					}
				}
			}
		}
		allQualityPriorities[qualname] = mapPriorities
		allQualityPrioritiesWanted[qualname] = mapPrioritieswanted
	}
	//logger.Log.Debug(allQualityPriorities)
	//logger.Log.Debug(allQualityPrioritiesWanted)

	mapPriorities = nil
	mapPrioritieswanted = nil
	getaudios = nil
	getcodecs = nil
	getqualities = nil
	getresolutions = nil
}

func gettypeids(m *apiexternal.ParseInfo, disableParserStringMatch bool, inval string, qualitytype *database.InQualitiesArray) uint {
	var id uint
	tolower := strings.ToLower(inval)
	var index, substrpost_len int
	var substrpre, substrpost string
	var isokpost, isokpre bool
	lenval := len(inval)
	lenstr := 0
	for idx := range qualitytype.Arr {
		id = 0
		lenstr = len(qualitytype.Arr[idx].Strings)
		if lenstr >= 1 && !disableParserStringMatch {
			if strings.Contains(qualitytype.Arr[idx].StringsLower, tolower) {
				index = strings.Index(qualitytype.Arr[idx].StringsLower, tolower)
				substrpre = ""
				if index >= 1 {
					substrpre = qualitytype.Arr[idx].StringsLower[index-1 : index]
				}
				substrpost_len = index + lenval + 1
				if lenstr < substrpost_len {
					substrpost_len = index + lenval
				}
				substrpost = qualitytype.Arr[idx].StringsLower[index+lenval : substrpost_len]
				isokpost = true
				isokpre = true
				if substrpost != "" {
					if unicode.IsDigit([]rune(substrpost)[0]) || unicode.IsLetter([]rune(substrpost)[0]) {
						isokpost = false
					}
				}
				if substrpre != "" {
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
func gettypeidpriority(m *apiexternal.ParseInfo, id uint, qualitystringtype string, qualityTemplate string, setprioonly bool, reordergroup *config.QualityReorderConfigGroup, qualitytype *database.InQualitiesArray) int {
	var priority int
	var name string
	for qualidx := range qualitytype.Arr {
		priority = 0
		name = ""
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
	var dbserieId uint
	var counter int
	if m.Tvdb != "" {
		counter, _ = database.CountRowsStatic(database.Querywithargs{QueryString: "select count() from dbseries where thetvdb_id = ?", Args: []interface{}{strings.Replace(m.Tvdb, "tvdb", "", -1)}})
		if counter == 1 {
			dbserieId, _ = database.QueryColumnUint(database.Querywithargs{QueryString: "select id from dbseries where thetvdb_id = ?", Args: []interface{}{strings.Replace(m.Tvdb, "tvdb", "", -1)}})
		}
	}
	if dbserieId == 0 && titleyear != "" {
		dbserieId, _ = importfeed.FindDbserieByName(titleyear)
	}
	if dbserieId == 0 && seriestitle != "" {
		dbserieId, _ = importfeed.FindDbserieByName(seriestitle)
	}
	if dbserieId == 0 && m.Title != "" {
		dbserieId, _ = importfeed.FindDbserieByName(m.Title)
	}
	if dbserieId != 0 {
		counter, _ = database.CountRowsStatic(database.Querywithargs{QueryString: "select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE", Args: []interface{}{dbserieId, listname}})
		if counter == 1 {
			serieid, _ := getseriebydbidandlist(dbserieId, listname)
			return serieid, 1, nil
		}
	}
	return 0, 0, errNotFound
}

func FindDbmovieByFile(m *apiexternal.ParseInfo) (uint, error) {
	if m.Imdb != "" {
		dbmovieid, err := database.QueryColumnUint(database.Querywithargs{QueryString: "select id from dbmovies where imdb_id = ?", Args: []interface{}{m.Imdb}})
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
