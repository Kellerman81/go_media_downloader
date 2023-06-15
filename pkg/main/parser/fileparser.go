// parser
package parser

import (
	"path/filepath"
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

var (
	allQualityPrioritiesT       = make([]Prioarr, 0, 100000)
	allQualityPrioritiesWantedT = make([]Prioarr, 0, 100000)
	scanpatterns                = make([]regexpattern, 0, 100)
	varextended                 = []string{"extended", "extended cut", "extended.cut", "extended-cut"}
	varproper                   = []string{"proper"}
	varrepack                   = []string{"repack"}
)

func Getallprios() []Prioarr {
	return allQualityPrioritiesWantedT
}

func Getcompleteallprios() []Prioarr {
	return allQualityPrioritiesT
}

func LoadDBPatterns() {
	if len(scanpatterns) == 0 {
		scanpatterns = []regexpattern{
			{"season", false, `(?i)(s?(\d{1,4}))(?: )?[ex]`, 2},
			{"episode", false, `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`, 2},
			{"identifier", false, `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
			{logger.StrDate, false, `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`, 2},
			{"year", true, `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, 2},
			{"audio", false, `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`, 2},
			{"imdb", false, `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, 2},
			{"tvdb", false, `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, 2},
		}
		//logger.Grow(&scanpatterns, len(database.DBConnect.GetaudiosIn)+len(database.DBConnect.GetresolutionsIn)+len(database.DBConnect.GetqualitiesIn)+len(database.DBConnect.GetcodecsIn))
		for idx := range scanpatterns {
			logger.GlobalCacheRegex.SetRegexp(&scanpatterns[idx].re, 0)
		}
		for idx := range database.DBConnect.GetaudiosIn {
			if database.DBConnect.GetaudiosIn[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "audio", last: false, re: database.DBConnect.GetaudiosIn[idx].Regex, getgroup: database.DBConnect.GetaudiosIn[idx].Regexgroup})
				logger.GlobalCacheRegex.SetRegexp(&database.DBConnect.GetaudiosIn[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetresolutionsIn {
			if database.DBConnect.GetresolutionsIn[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "resolution", last: false, re: database.DBConnect.GetresolutionsIn[idx].Regex, getgroup: database.DBConnect.GetresolutionsIn[idx].Regexgroup})
				logger.GlobalCacheRegex.SetRegexp(&database.DBConnect.GetresolutionsIn[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetqualitiesIn {
			if database.DBConnect.GetqualitiesIn[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "quality", last: false, re: database.DBConnect.GetqualitiesIn[idx].Regex, getgroup: database.DBConnect.GetqualitiesIn[idx].Regexgroup})
				logger.GlobalCacheRegex.SetRegexp(&database.DBConnect.GetqualitiesIn[idx].Regex, 0)
			}
		}
		for idx := range database.DBConnect.GetcodecsIn {
			if database.DBConnect.GetcodecsIn[idx].UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{name: "codec", last: false, re: database.DBConnect.GetcodecsIn[idx].Regex, getgroup: database.DBConnect.GetcodecsIn[idx].Regexgroup})
				logger.GlobalCacheRegex.SetRegexp(&database.DBConnect.GetcodecsIn[idx].Regex, 0)
			}
		}
	}
}

func NewCutoffPrio(cfgpstr string, templatequality string) int {
	return GetPriorityMapQualAutoClose(&apiexternal.ParseInfo{Quality: config.SettingsQuality["quality_"+templatequality].CutoffQuality, Resolution: config.SettingsQuality["quality_"+templatequality].CutoffResolution}, cfgpstr, templatequality, true, false)
}

func getparsermatches(pat *regexpattern, m *apiexternal.FileParser) (string, string) {
	// if pat.getgroup == 0 {
	// 	pat.getgroup = logger.GlobalRegexCache.GetRegexpDirect(pat.re).NumSubexp() - 1
	// }
	var matchest *[]int
	if pat.last && pat.name == "year" && m.TypeGroup == logger.StrSeries {
		matches := logger.GlobalCacheRegex.GetRegexpDirect(&pat.re).FindAllStringSubmatchIndex(m.Str, 10)
		//matchest first slice is the match - second slice submatches - 1 mostly empty 2 wanted 3 empty
		if len(matches) >= 1 {
			//result was found
			lensubmatches := len(matches[len(matches)-1])
			if lensubmatches >= 1 {
				matchest = &matches[len(matches)-1]
				defer logger.Clear(&matches)
				//last result also has submatches
				// if ((lensubmatches*2)-1) >= pat.getgroup && lensubmatches >= 4 && matches[lenmatches-1][3] != -1 && matches[lenmatches-1][(pat.getgroup*2)+1] != -1 {
				// 	return cleanName[matches[lenmatches-1][2]:matches[lenmatches-1][3]], cleanName[matches[lenmatches-1][pat.getgroup*2]:matches[lenmatches-1][(pat.getgroup*2)+1]]
				// }
				// if ((lensubmatches*2)-1) >= pat.getgroup && lensubmatches <= 2 && matches[lenmatches-1][(pat.getgroup*2)+1] != -1 {
				// 	return "", cleanName[matches[lenmatches-1][pat.getgroup*2]:matches[lenmatches-1][(pat.getgroup*2)+1]]
				// }
				// if (lensubmatches*2) >= 4 && matches[lenmatches-1][3] != -1 {
				// 	return cleanName[matches[lenmatches-1][2]:matches[lenmatches-1][3]], ""
				// }
				// return "", ""
			} else {
				logger.Clear(&matches)
			}
		} else {
			logger.Clear(&matches)
		}
		//return "", ""

		//matchentry = config.RegexGetLastMatches(scanpatterns[idx].re, cleanName, 10)
	} else {
		matchest = config.Getmatches(true, &pat.re, &m.Str)
		//matchest = logger.GlobalCacheRegex.GetRegexpDirect(&pat.re).FindStringSubmatchIndex(*cleanName)
	}
	if matchest == nil {
		return "", ""
	}
	lensubmatches := len(*matchest)
	if lensubmatches == 0 {
		return "", ""
	}

	var ret1, ret2 string
	if ((lensubmatches/2)-1) >= pat.getgroup && lensubmatches >= 4 && (*matchest)[3] != -1 && (*matchest)[(pat.getgroup*2)+1] != -1 {
		ret1 = m.Str[(*matchest)[2]:(*matchest)[3]]
		ret2 = m.Str[(*matchest)[pat.getgroup*2]:(*matchest)[(pat.getgroup*2)+1]]
	} else if ((lensubmatches*2)-1) >= pat.getgroup && lensubmatches <= 2 && (*matchest)[(pat.getgroup*2)+1] != -1 {
		ret2 = m.Str[(*matchest)[pat.getgroup*2]:(*matchest)[(pat.getgroup*2)+1]]
	} else if (lensubmatches*2) >= 4 && (*matchest)[3] != -1 {
		ret1 = m.Str[(*matchest)[2]:(*matchest)[3]]
	}
	//logger.Log.Debug().Any("found", matchest).Str("search", cleanName).Str("key", pat.re).Int("count", len(matchest)).Msg("matchest")
	logger.Clear(matchest)
	return ret1, ret2
}

func NewFileParser(cleanName string, includeYearInTitle bool, typegroup string) *apiexternal.FileParser {
	m := apiexternal.FileParser{Str: cleanName, M: apiexternal.ParseInfo{File: cleanName}, TypeGroup: typegroup, IncludeYearInTitle: includeYearInTitle}
	//m := apiexternal.ParseInfo{File: cleanName}
	parsestatic(&m)
	parseadditional(&m)

	return &m
}

// Parses - uses fprobe and checks language
func ParseFile(videofile *string, usepath bool, includeYearInTitle bool, typegroup string, usefolder bool) *apiexternal.FileParser {
	var parse string
	if usepath {
		parse = filepath.Base(*videofile)
	} else {
		parse = *videofile
	}
	m := NewFileParser(parse, includeYearInTitle, typegroup)
	if m.M.Quality != "" && m.M.Resolution != "" {
		return m
	}
	if !usefolder || !usepath {
		return m
	}
	mf := NewFileParser(filepath.Base(filepath.Dir(*videofile)), includeYearInTitle, typegroup)
	if m.M.Quality == "" {
		m.M.Quality = mf.M.Quality
	}
	if m.M.Resolution == "" {
		m.M.Resolution = mf.M.Resolution
	}
	if m.M.Title == "" {
		m.M.Title = mf.M.Title
	}
	if m.M.Year == 0 {
		m.M.Year = mf.M.Year
	}
	if m.M.Identifier == "" {
		m.M.Identifier = mf.M.Identifier
	}
	if m.M.Audio == "" {
		m.M.Audio = mf.M.Audio
	}
	if m.M.Codec == "" {
		m.M.Codec = mf.M.Codec
	}
	if m.M.Imdb == "" {
		m.M.Imdb = mf.M.Imdb
	}
	mf.Close()
	return m
}

func parsestatic(m *apiexternal.FileParser) {
	m.Str = strings.TrimRight(strings.TrimLeft(m.M.File, "["), "]")
	logger.StringReplaceRuneP(&m.Str, '_', " ")

	if !logger.DisableParserStringMatch {
		m.M.Parsegroup(m.Str, "audio", &database.DBConnect.AudioStrIn)
		m.M.Parsegroup(m.Str, "codec", &database.DBConnect.CodecStrIn)
		m.M.Parsegroup(m.Str, "quality", &database.DBConnect.QualityStrIn)
		m.M.Parsegroup(m.Str, "resolution", &database.DBConnect.ResolutionStrIn)
	}
	m.M.Parsegroup(m.Str, "extended", &varextended)
	m.M.Parsegroup(m.Str, "proper", &varproper)
	m.M.Parsegroup(m.Str, "repack", &varrepack)
}

func parseadditional(m *apiexternal.FileParser) {
	startIndex, endIndex := parsepatterns(m)
	if m.M.Date != "" {
		m.M.Identifier = m.M.Date
	} else if m.M.Identifier == "" && m.M.SeasonStr != "" && m.M.EpisodeStr != "" {
		m.M.Identifier = "S" + m.M.SeasonStr + "E" + m.M.EpisodeStr
	}
	if endIndex < startIndex {
		logger.Log.Debug().Str(logger.StrPath, m.M.File).Int("start", startIndex).Int("end", endIndex).Msg("EndIndex < startindex")
		m.Str = m.M.File[startIndex:]
	} else {
		m.Str = m.M.File[startIndex:endIndex]
	}

	m.Str = logger.SplitByRet(m.Str, '(')

	m.Str = strings.TrimPrefix(m.Str, "- ")
	if strings.ContainsRune(m.Str, '.') && !strings.ContainsRune(m.Str, ' ') {
		logger.StringReplaceRuneP(&m.Str, '.', " ")
	} else if strings.ContainsRune(m.Str, '.') && strings.ContainsRune(m.Str, '_') {
		logger.StringReplaceRuneP(&m.Str, '_', " ")
	}
	m.M.Title = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(m.Str), "-"), "."))
}

func parsepatterns(m *apiexternal.FileParser) (int, int) {
	var startIndex, endIndex = 0, len(m.M.File)
	conttt := !logger.ContainsI(m.Str, logger.StrTt)
	conttvdb := !logger.ContainsI(m.Str, logger.StrTvdb)
	lenclean := len(m.Str)
	// var match1, matchgroup string
	var index int
	var match1, matchgroup string
	for idx := range scanpatterns {
		switch scanpatterns[idx].name {
		case "imdb":
			if m.TypeGroup != logger.StrMovie || conttt {
				continue
			}
		case "tvdb":
			if m.TypeGroup != logger.StrSeries || conttvdb {
				continue
			}
		case "season":
		case "episode":
		case "identifier":
		case logger.StrDate:
			if m.TypeGroup != logger.StrSeries {
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

		match1, matchgroup = getparsermatches(&scanpatterns[idx], m)

		if len(match1) >= 1 {
			index = strings.Index(m.Str, match1)
			if !m.IncludeYearInTitle || (m.IncludeYearInTitle && scanpatterns[idx].name != "year") {
				if index == 0 {
					if len(match1) != lenclean && len(match1) < endIndex {
						startIndex = len(match1)
					}
				} else if index < endIndex && index > startIndex {
					endIndex = index
				}
			}
		}

		if len(matchgroup) == 0 {
			continue
		}
		switch scanpatterns[idx].name {
		case "imdb":
			m.M.Imdb = matchgroup
		case "tvdb":
			if logger.HasPrefixI(matchgroup, logger.StrTvdb) {
				m.M.Tvdb = matchgroup[:4]
			} else {
				m.M.Tvdb = matchgroup
			}
		case "year":
			m.M.Year = logger.StringToInt(matchgroup)
		case "season":
			m.M.SeasonStr = matchgroup
			m.M.Season = logger.StringToInt(m.M.SeasonStr)
		case "episode":
			m.M.EpisodeStr = matchgroup
			m.M.Episode = logger.StringToInt(m.M.EpisodeStr)
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
	}
	return startIndex, endIndex
}

func getdbmovieidbyimdb(m *apiexternal.ParseInfo, imdbid *string) {
	if config.SettingsGeneral.UseMediaCache {
		ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, *imdbid) })
		if ti != -1 {
			m.DbmovieID = uint(database.CacheDBMovie[ti].Num1)
		}
	} else {
		m.DbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, imdbid)
	}
}

func GetDBIDs(m *apiexternal.ParseInfo, cfgpstr string, listname string, allowsearchtitle bool) error {
	if logger.HasPrefixI(cfgpstr, logger.StrMovie) {
		if m.Imdb != "" {
			run2 := logger.HasPrefixI(m.Imdb, logger.StrTt)
			if !run2 {
				imdbid := logger.AddImdbPrefix(m.Imdb)
				getdbmovieidbyimdb(m, &imdbid)
			} else {
				getdbmovieidbyimdb(m, &m.Imdb)
			}

			// m.DbmovieID = uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool {
			// 	return strings.EqualFold(elem.Str3, logger.AddImdbPrefix(m.Imdb))
			// }).Num1)

			//database.QueryColumn(database.QueryDbmoviesGetIDByImdb, &m.DbmovieID, logger.AddImdbPrrefix(&m.Imdb))
			if m.DbmovieID == 0 && !run2 {
				imdbid := logger.AddImdbPrefix(m.Imdb)
				imdbid0 := logger.AddImdbPrefix("0" + m.Imdb)
				imdbid00 := logger.AddImdbPrefix("00" + m.Imdb)
				imdbid000 := logger.AddImdbPrefix("000" + m.Imdb)
				imdbid0000 := logger.AddImdbPrefix("0000" + m.Imdb)

				if config.SettingsGeneral.UseMediaCache {
					ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool {
						return strings.EqualFold(elem.Str3, imdbid) || elem.Str3 == imdbid0 || elem.Str3 == imdbid00 || elem.Str3 == imdbid000 || elem.Str3 == imdbid0000
					})
					if ti != -1 {
						m.DbmovieID = uint(database.CacheDBMovie[ti].Num1)
					}
				} else {
					m.DbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid0)
					if m.DbmovieID == 0 {
						m.DbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid00)
					}
					if m.DbmovieID == 0 {
						m.DbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid000)
					}
					if m.DbmovieID == 0 {
						m.DbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid0000)
					}
				}
				// m.DbmovieID = uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool {
				// 	return elem.Str3 == logger.AddImdbPrefix(m.Imdb) || elem.Str3 == logger.AddImdbPrefix("0"+m.Imdb) || elem.Str3 == logger.AddImdbPrefix("00"+m.Imdb) || elem.Str3 == logger.AddImdbPrefix("000"+m.Imdb) || elem.Str3 == logger.AddImdbPrefix("0000"+m.Imdb)
				// }).Num1)
			}
		}
		if m.DbmovieID == 0 && m.Title != "" && allowsearchtitle && cfgpstr != "" {
			var err error
			for idx := range config.SettingsMedia[cfgpstr].Lists {
				err = importfeed.StripTitlePrefixPostfixGetQual(m.Title, config.SettingsMedia[cfgpstr].Lists[idx].TemplateQuality)
				if err != nil {
					logger.Logerror(err, "Strip Failed")
				}
			}
			if m.Imdb == "" {
				m.Imdb, _, _ = importfeed.MovieFindImdbIDByTitle(&m.Title, m.Year, "", false)
			}
			if m.Imdb != "" {
				getdbmovieidbyimdb(m, &m.Imdb)
			}
			//m.DbmovieID = importfeed.MovieFindDBIDByTitleSimple(m.Imdb, &m.Title, m.Year, false)
		}
		if m.DbmovieID == 0 {
			return logger.ErrNotFoundDbmovie
		}
		if m.DbmovieID != 0 && listname != "" {
			m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, &m.DbmovieID, &listname)
		}

		if m.DbmovieID != 0 && m.MovieID == 0 && cfgpstr != "" {
			for idx := range config.SettingsMedia[cfgpstr].Lists {
				m.MovieID = database.QueryUintColumn(database.QueryMoviesGetIDByDBIDListname, &m.DbmovieID, &config.SettingsMedia[cfgpstr].Lists[idx].Name)
				if m.MovieID != 0 {
					m.Listname = config.SettingsMedia[cfgpstr].Lists[idx].Name
					break
				}
			}
		}
		if m.MovieID == 0 {
			m.DbmovieID = 0
			return logger.ErrNotFoundMovie
		}
		m.Listname = database.QueryStringColumn(database.QueryMoviesGetListnameByID, m.MovieID)
		return nil
	}
	if m.Tvdb != "" {
		m.DbserieID = database.QueryUintColumn(database.QueryDbseriesGetIDByTvdb, &m.Tvdb)
	}
	if m.DbserieID == 0 && m.Title != "" && (allowsearchtitle || m.Tvdb == "") {
		if m.Year != 0 {
			m.DbserieID = importfeed.FindDbserieByName(m.Title + " (" + logger.IntToString(m.Year) + ")")
		}
		if m.DbserieID == 0 {
			m.DbserieID = importfeed.FindDbserieByName(m.Title)
		}
	}
	if m.DbserieID == 0 && m.File != "" {
		basepath := filepath.Base(m.File)
		matched, _ := config.RegexGetMatchesStr1Str2(true, &logger.StrRegexSeriesTitle, &basepath)
		if matched != "" {
			logger.StringReplaceRuneP(&matched, '.', " ")
			matched = strings.TrimRight(matched, ".- ")
			m.DbserieID = importfeed.FindDbserieByName(matched)
		}
	}

	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}
	if listname != "" {
		m.SerieID = database.QueryUintColumn(database.QuerySeriesGetIDByDBIDListname, &m.DbserieID, &listname)
	}
	if m.SerieID == 0 && cfgpstr != "" {
		for idx := range config.SettingsMedia[cfgpstr].Lists {
			m.SerieID = database.QueryUintColumn(database.QuerySeriesGetIDByDBIDListname, &m.DbserieID, &config.SettingsMedia[cfgpstr].Lists[idx].Name)
			if m.SerieID != 0 {
				m.Listname = config.SettingsMedia[cfgpstr].Lists[idx].Name
				break
			}
		}
	}
	if m.SerieID == 0 {
		m.DbserieEpisodeID = 0
		m.SerieEpisodeID = 0
		return logger.ErrNotFoundSerie
	}
	m.DbserieEpisodeID = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, &m.Identifier, m.SeasonStr, m.EpisodeStr)
	if m.DbserieEpisodeID != 0 {
		m.SerieEpisodeID = database.QueryUintColumn(database.QuerySerieEpisodesGetIDBySerieDBEpisode, m.SerieID, m.DbserieEpisodeID)
	}
	m.Listname = database.QueryStringColumn(database.QuerySeriesGetListnameByID, m.SerieID)
	return nil
}

func ParseVideoFile(m *apiexternal.ParseInfo, file *string, qualityTemplate string) error {
	if m.QualitySet == "" {
		m.QualitySet = qualityTemplate
	}
	if !config.SettingsGeneral.UseMediainfo {
		err := probeURL(m, file, qualityTemplate)
		if err != nil && config.SettingsGeneral.UseMediaFallback {
			return parsemediainfo(m, file, qualityTemplate)
		}
		return err
	}
	err := parsemediainfo(m, file, qualityTemplate)
	if err != nil && config.SettingsGeneral.UseMediaFallback {
		return probeURL(m, file, qualityTemplate)
	}
	return err
}

//	func GetPriorityMapAutoClose(m *apiexternal.ParseInfo, cfgpstr string, qualityTemplate string, useall bool, checkwanted bool) int {
//		i := GetPriorityMap(m, cfgpstr, qualityTemplate, useall, checkwanted)
//		m.Close()
//		return i
//	}
func GetPriorityMapQualAutoClose(m *apiexternal.ParseInfo, cfgpstr string, qualityTemplate string, useall bool, checkwanted bool) int {
	i := GetPriorityMapQual(m, cfgpstr, qualityTemplate, useall, checkwanted)
	m.Close()
	return i
}

//	func GetPriorityMap(m *apiexternal.ParseInfo, cfgpstr string, qualityTemplate string, useall bool, checkwanted bool) int {
//		return GetPriorityMapQual(m, cfgpstr, qualityTemplate, useall, checkwanted)
//	}
func GetPriorityMapQual(m *apiexternal.ParseInfo, cfgpstr string, templatequality string, useall bool, checkwanted bool) int {

	m.QualitySet = config.SettingsQuality["quality_"+templatequality].Name

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

	if m.ResolutionID == 0 && cfgpstr != "" {
		intid := -1
		for idxi := range database.DBConnect.GetresolutionsIn {
			if strings.EqualFold(database.DBConnect.GetresolutionsIn[idxi].Name, config.SettingsMedia[cfgpstr].DefaultResolution) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetresolutionsIn, func(e database.QualitiesRegex) bool {
		//	return strings.EqualFold(e.Name, config.SettingsMedia[cfgpstr].DefaultResolution)
		//})
		if intid != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[intid].ID
		}
	}
	if m.QualityID == 0 && cfgpstr != "" {
		intid := -1
		for idxi := range database.DBConnect.GetqualitiesIn {
			if strings.EqualFold(database.DBConnect.GetqualitiesIn[idxi].Name, config.SettingsMedia[cfgpstr].DefaultQuality) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetqualitiesIn, func(e database.QualitiesRegex) bool {
		//	return strings.EqualFold(e.Name, config.SettingsMedia[cfgpstr].DefaultQuality)
		//})
		if intid != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[intid].ID
		}
	}

	if m.ResolutionID != 0 {
		intid := -1
		for idxi := range database.DBConnect.GetresolutionsIn {
			if database.DBConnect.GetresolutionsIn[idxi].ID == m.ResolutionID {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetresolutionsIn, func(c database.QualitiesRegex) bool {
		//	return c.ID == m.ResolutionID
		//})
		if intid != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[intid].Name
		}
	}
	if m.QualityID != 0 {
		intid := -1
		for idxi := range database.DBConnect.GetqualitiesIn {
			if database.DBConnect.GetqualitiesIn[idxi].ID == m.QualityID {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetqualitiesIn, func(c database.QualitiesRegex) bool {
		//	return c.ID == m.QualityID
		//})
		if intid != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[intid].Name
		}
	}

	m.Priority = GetIDPriority(m, templatequality, useall, checkwanted)
	return m.Priority
}
func Getdbidsfromfiles(useall bool, checkwanted bool, id uint, querycount string, querysql string, templatequality string) int {
	if database.QueryIntColumn(querycount, id) == 0 {
		return 0
	}
	//var m apiexternal.ParseInfo
	//database.Queryfilesprio(querysql, &m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID, &m.Proper, &m.Extended, &m.Repack, id)
	//defer m.Close()
	var r, q, c, a uint
	var p, e, re bool
	database.Queryfilesprio(querysql, &r, &q, &c, &a, &p, &e, &re, id)
	//return GetIDPriority(&m, templatequality, useall, checkwanted)
	return GetIDPriority(&apiexternal.ParseInfo{ResolutionID: r, QualityID: q, AudioID: a, CodecID: c, Proper: p, Extended: e, Repack: re}, templatequality, useall, checkwanted)
}
func GetIDPriorityMap(m *apiexternal.ParseInfo, cfgpstr string, templatequality string, useall bool, checkwanted bool) int {
	if m.ResolutionID == 0 && m.Resolution == "" && cfgpstr != "" {
		m.Resolution = config.SettingsMedia[cfgpstr].DefaultResolution
	}
	if m.QualityID == 0 && m.Quality == "" && cfgpstr != "" {
		m.Quality = config.SettingsMedia[cfgpstr].DefaultQuality
	}
	if m.ResolutionID == 0 && m.Resolution != "" {
		m.ResolutionID = gettypeids(logger.DisableParserStringMatch, m.Resolution, &database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 && m.Quality != "" {
		m.QualityID = gettypeids(logger.DisableParserStringMatch, m.Quality, &database.DBConnect.GetqualitiesIn)
	}
	if m.ResolutionID == 0 && cfgpstr != "" {
		intid := -1
		for idxi := range database.DBConnect.GetresolutionsIn {
			if strings.EqualFold(database.DBConnect.GetresolutionsIn[idxi].Name, config.SettingsMedia[cfgpstr].DefaultResolution) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetresolutionsIn, func(e database.QualitiesRegex) bool {
		//	return strings.EqualFold(e.Name, config.SettingsMedia[cfgpstr].DefaultResolution)
		//})
		if intid != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[intid].ID
		}
	}
	if m.QualityID == 0 && cfgpstr != "" {
		intid := -1
		for idxi := range database.DBConnect.GetqualitiesIn {
			if strings.EqualFold(database.DBConnect.GetqualitiesIn[idxi].Name, config.SettingsMedia[cfgpstr].DefaultQuality) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFunc(&database.DBConnect.GetqualitiesIn, func(e database.QualitiesRegex) bool {
		//	return strings.EqualFold(e.Name, config.SettingsMedia[cfgpstr].DefaultQuality)
		//})
		if intid != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[intid].ID
		}
	}

	return GetIDPriority(m, templatequality, useall, checkwanted)
}

func GetIDPriority(m *apiexternal.ParseInfo, templatequality string, useall bool, checkwanted bool) int {
	var reso, qual, aud, codec uint

	if config.SettingsQuality["quality_"+templatequality].UseForPriorityResolution || useall {
		reso = m.ResolutionID
	}
	if config.SettingsQuality["quality_"+templatequality].UseForPriorityQuality || useall {
		qual = m.QualityID
	}
	if config.SettingsQuality["quality_"+templatequality].UseForPriorityAudio || useall {
		aud = m.AudioID
	}
	if config.SettingsQuality["quality_"+templatequality].UseForPriorityCodec || useall {
		codec = m.CodecID
	}
	var prio int
	var intid int
	if checkwanted {
		intid = -1
		for idxi := range allQualityPrioritiesWantedT {
			if strings.EqualFold(allQualityPrioritiesWantedT[idxi].QualityGroup, config.SettingsQuality["quality_"+templatequality].Name) && allQualityPrioritiesWantedT[idxi].ResolutionID == reso && allQualityPrioritiesWantedT[idxi].QualityID == qual && allQualityPrioritiesWantedT[idxi].CodecID == codec && allQualityPrioritiesWantedT[idxi].AudioID == aud {
				intid = idxi
				break
			}
		}
		//intid = logger.IndexFunc(&allQualityPrioritiesWantedT, func(e Prioarr) bool {
		//	return strings.EqualFold(e.QualityGroup, config.SettingsQuality["quality_"+templatequality].Name) && e.ResolutionID == reso && e.QualityID == qual && e.CodecID == codec && e.AudioID == aud
		//})
		if intid != -1 {
			prio = allQualityPrioritiesWantedT[intid].Priority
		}
	} else {
		intid = -1
		for idxi := range allQualityPrioritiesT {
			if strings.EqualFold(allQualityPrioritiesT[idxi].QualityGroup, config.SettingsQuality["quality_"+templatequality].Name) && allQualityPrioritiesT[idxi].ResolutionID == reso && allQualityPrioritiesT[idxi].QualityID == qual && allQualityPrioritiesT[idxi].CodecID == codec && allQualityPrioritiesT[idxi].AudioID == aud {
				intid = idxi
				break
			}
		}
		//intid = logger.IndexFunc(&allQualityPrioritiesT, func(e Prioarr) bool {
		//	return strings.EqualFold(e.QualityGroup, config.SettingsQuality["quality_"+templatequality].Name) && e.ResolutionID == reso && e.QualityID == qual && e.CodecID == codec && e.AudioID == aud
		//})
		if intid != -1 {
			prio = allQualityPrioritiesWantedT[intid].Priority
		}
	}

	//allQualityPrioritiesMu.Unlock()
	if intid == -1 {
		logger.Log.Debug().Str("searched for", buildPrioStr(reso, qual, codec, aud)).Str("in", config.SettingsQuality["quality_"+templatequality].Name).Bool("wanted", checkwanted).Msg("prio not found")
		return 0
	}
	if !config.SettingsQuality["quality_"+templatequality].UseForPriorityOther && !useall {
		//cfgqual.Close()
		return prio
	}
	if m.Proper {
		prio += 5
	}
	if m.Extended {
		prio += 2
	}
	if m.Repack {
		prio++
	}
	return prio
}

func getIDPrioritySimple(m *apiexternal.ParseInfo, reordergroup config.QualityReorderConfigGroup) int {
	var priores, prioqual, prioaud, priocodec int
	if m.ResolutionID != 0 {
		priores = gettypeidpriority(m.ResolutionID, "resolution", reordergroup, &database.DBConnect.GetresolutionsIn)
	}
	if m.QualityID != 0 {
		prioqual = gettypeidpriority(m.QualityID, "quality", reordergroup, &database.DBConnect.GetqualitiesIn)
	}
	if m.CodecID != 0 {
		priocodec = gettypeidpriority(m.CodecID, "codec", reordergroup, &database.DBConnect.GetcodecsIn)
	}
	if m.AudioID != 0 {
		prioaud = gettypeidpriority(m.AudioID, "audio", reordergroup, &database.DBConnect.GetaudiosIn)
	}

	var idxcomma int
	for idxreorder := range reordergroup.Arr {
		if !strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "combined_res_qual") {
			continue
		}
		if strings.ContainsRune(reordergroup.Arr[idxreorder].Name, ',') {
			continue
		}
		idxcomma = strings.IndexRune(reordergroup.Arr[idxreorder].Name, ',')

		if strings.EqualFold(reordergroup.Arr[idxreorder].Name[:idxcomma], m.Resolution) && strings.EqualFold(reordergroup.Arr[idxreorder].Name[idxcomma+1:], m.Quality) {
			priores = reordergroup.Arr[idxreorder].Newpriority
			prioqual = 0
		}
	}

	return priores + prioqual + priocodec + prioaud
}

type Prioarr struct {
	QualityGroup string
	ResolutionID uint
	QualityID    uint
	CodecID      uint
	AudioID      uint
	Priority     int
}

func GetAllQualityPriorities() {
	//allQualityPrioritiesMu.Lock()

	getresolutions := append(database.DBConnect.GetresolutionsIn, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getqualities := append(database.DBConnect.GetqualitiesIn, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getaudios := append(database.DBConnect.GetaudiosIn, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	getcodecs := append(database.DBConnect.GetcodecsIn, database.QualitiesRegex{Qualities: database.Qualities{Name: "", ID: 0, Priority: 0}})

	allQualityPrioritiesT = allQualityPrioritiesT[:0]
	allQualityPrioritiesWantedT = allQualityPrioritiesWantedT[:0]
	var reordergroup config.QualityReorderConfigGroup
	var target Prioarr
	var cfgreso []string
	var cfgqual []string
	var addwanted bool
	var parse apiexternal.ParseInfo
	for idx := range config.SettingsQuality {
		reordergroup = config.QualityReorderConfigGroup{Arr: config.SettingsQuality[idx].QualityReorder}
		//submap := make(map[string]int, lenmap)
		//submapwanted := make(map[string]int, lenmap)
		target = Prioarr{QualityGroup: config.SettingsQuality[idx].Name}
		cfgreso = config.SettingsQuality[idx].WantedResolution
		cfgqual = config.SettingsQuality[idx].WantedQuality
		for idxreso := range getresolutions {
			addwanted = true
			if !logger.ContainsStringsI(&cfgreso, getresolutions[idxreso].Name) && database.DBLogLevel == logger.StrDebug {
				logger.Log.Debug().Str("Quality", config.SettingsQuality[idx].Name).Str("Resolution Parse", getresolutions[idxreso].Name).Msg("unwanted res")
				addwanted = false
			}
			parse = apiexternal.ParseInfo{
				Resolution:   getresolutions[idxreso].Name,
				ResolutionID: getresolutions[idxreso].ID,
			}
			target.ResolutionID = parse.ResolutionID
			//r = logger.UintToString(parse.ResolutionID)

			for idxqual := range getqualities {
				if !logger.ContainsStringsI(&cfgqual, getqualities[idxqual].Name) && database.DBLogLevel == logger.StrDebug {
					logger.Log.Debug().Str("Quality", config.SettingsQuality[idx].Name).Str("Resolution Parse", parse.Resolution).Str("Quality Parse", parse.Quality).Msg("unwanted qual")
					addwanted = false
				}

				parse.Quality = getqualities[idxqual].Name
				parse.QualityID = getqualities[idxqual].ID
				target.QualityID = parse.QualityID
				//q = logger.UintToString(parse.QualityID)
				for idxcodec := range getcodecs {
					parse.Codec = getcodecs[idxcodec].Name
					parse.CodecID = getcodecs[idxcodec].ID
					target.CodecID = parse.CodecID
					//c = logger.UintToString(parse.CodecID)
					for idxaudio := range getaudios {
						parse.Audio = getaudios[idxaudio].Name
						parse.AudioID = getaudios[idxaudio].ID

						target.AudioID = parse.AudioID
						target.Priority = getIDPrioritySimple(&parse, reordergroup)
						allQualityPrioritiesT = append(allQualityPrioritiesT, target)
						//submap[str4] = setprio
						if addwanted {
							allQualityPrioritiesWantedT = append(allQualityPrioritiesWantedT, target)
							//submapwanted[str4] = setprio
						}
					}
				}
			}
		}
	}
}

func gettypeids(disableParserStringMatch bool, inval string, qualitytype *[]database.QualitiesRegex) uint {
	lenval := len(inval)
	var lenstr, index int
	for idx := range *qualitytype {
		lenstr = len((*qualitytype)[idx].Strings)
		if lenstr >= 1 && !disableParserStringMatch && logger.ContainsI((*qualitytype)[idx].StringsLower, inval) {
			index = logger.IndexI((*qualitytype)[idx].StringsLower, inval)

			if !apiexternal.CheckDigitLetter(apiexternal.After((*qualitytype)[idx].StringsLower, index+lenval)) {
				continue
			}
			if !apiexternal.CheckDigitLetter(apiexternal.Before((*qualitytype)[idx].StringsLower, index)) {
				continue
			}
			if (*qualitytype)[idx].ID != 0 {
				return (*qualitytype)[idx].ID
			}
		}
		if len((*qualitytype)[idx].Regex) >= 1 && (*qualitytype)[idx].UseRegex && config.RegexGetMatchesFind(&(*qualitytype)[idx].Regex, inval, 2) {
			return (*qualitytype)[idx].ID
		}
	}
	return 0
}

func reorderpriority(reordergroup config.QualityReorderConfigGroup, qualitystringtype string, qualityname string, priority *int) {

	for idxreorder := range reordergroup.Arr {
		if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, qualitystringtype) && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualityname) {
			*priority = reordergroup.Arr[idxreorder].Newpriority
		}
		if strings.EqualFold(reordergroup.Arr[idxreorder].ReorderType, "position") && strings.EqualFold(reordergroup.Arr[idxreorder].Name, qualitystringtype) {
			*priority *= reordergroup.Arr[idxreorder].Newpriority
		}
	}
}
func gettypeidpriority(id uint, qualitystringtype string, reordergroup config.QualityReorderConfigGroup, qualitytype *[]database.QualitiesRegex) int {
	var priority int
	for qualidx := range *qualitytype {
		if (*qualitytype)[qualidx].ID != id {
			continue
		}
		priority = (*qualitytype)[qualidx].Priority

		reorderpriority(reordergroup, qualitystringtype, (*qualitytype)[qualidx].Name, &priority)

		switch qualitystringtype {
		case "resolution", "quality", "codec", "audio":
			return priority
		}

		return 0
	}
	return 0
}

func buildPrioStr(r uint, q uint, c uint, a uint) string {
	return logger.UintToString(r) + logger.Underscore + logger.UintToString(q) + logger.Underscore + logger.UintToString(c) + logger.Underscore + logger.UintToString(a)
}
