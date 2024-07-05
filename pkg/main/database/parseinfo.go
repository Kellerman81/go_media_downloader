package database

import (
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// ParseInfo is a struct containing parsed information about media files
type ParseInfo struct {
	Str string //used internally
	// File is the path to the media file
	File string
	// Title is the title of the media
	Title     string
	TempTitle string
	TempID    uint
	// Season is the season number, if applicable
	Season int `json:"season,omitempty"`
	// Episode is the episode number, if applicable
	Episode int `json:"episode,omitempty"`
	// SeasonStr is the season number as a string, if applicable
	SeasonStr string `json:"seasonstr,omitempty"`
	// EpisodeStr is the episode number as a string, if applicable
	EpisodeStr string `json:"episodestr,omitempty"`
	// Year is the year of release
	Year uint16 `json:"year,omitempty"`
	// Resolution is the video resolution
	Resolution string `json:"resolution,omitempty"`
	// ResolutionID is the database ID of the resolution
	ResolutionID uint `json:"resolutionid,omitempty"`
	// Quality is the video quality description
	Quality string `json:"quality,omitempty"`
	// QualityID is the database ID of the quality
	QualityID uint `json:"qualityid,omitempty"`
	// Codec is the video codec
	Codec string `json:"codec,omitempty"`
	// CodecID is the database ID of the codec
	CodecID uint `json:"codecid,omitempty"`
	// Audio is the audio description
	Audio string `json:"audio,omitempty"`
	// AudioID is the database ID of the audio
	AudioID uint `json:"audioid,omitempty"`
	// Priority is the priority for downloading
	Priority int `json:"priority,omitempty"`
	// Identifier is an identifier string
	Identifier string `json:"identifier,omitempty"`
	// Date is the release date
	Date string `json:"date,omitempty"`
	// Extended is a flag indicating if it is an extended version
	Extended bool `json:"extended,omitempty"`
	// Proper is a flag indicating if it is a proper release
	Proper bool `json:"proper,omitempty"`
	// Repack is a flag indicating if it is a repack release
	Repack bool `json:"repack,omitempty"`
	// Imdb is the IMDB ID
	Imdb string `json:"imdb,omitempty"`
	// Tvdb is the TVDB ID
	Tvdb string `json:"tvdb,omitempty"`
	// Languages is a list of language codes
	Languages []string `json:"languages,omitempty"`
	// Runtime is the runtime in minutes
	Runtime    int    `json:"runtime,omitempty"`
	RuntimeStr string `json:"-"`
	// Height is the video height in pixels
	Height int `json:"height,omitempty"`
	// Width is the video width in pixels
	Width int `json:"width,omitempty"`
	// DbmovieID is the database ID of the movie
	DbmovieID uint `json:"dbmovieid,omitempty"`
	// MovieID is the application ID of the movie
	MovieID uint `json:"movieid,omitempty"`
	// DbserieID is the database ID of the TV series
	DbserieID uint `json:"dbserieid,omitempty"`
	// DbserieEpisodeID is the database ID of the episode
	DbserieEpisodeID uint `json:"dbserieepisodeid,omitempty"`
	// SerieID is the application ID of the TV series
	SerieID uint `json:"serieid,omitempty"`
	// SerieEpisodeID is the application ID of the episode
	SerieEpisodeID uint `json:"serieepisodeid,omitempty"`
	// ListID is the ID of the list this came from
	ListID   int8
	Episodes []DbstaticTwoUint `json:"-"`

	//SluggedTitle     string
	//Listname         string   `json:"listname,omitempty"`
	//ListCfg *config.ListsConfig
	//Group           string   `json:"group,omitempty"`
	//Region          string   `json:"region,omitempty"`
	//Hardcoded       bool     `json:"hardcoded,omitempty"`
	//Container       string   `json:"container,omitempty"`
	//Widescreen      bool     `json:"widescreen,omitempty"`
	//Website         string   `json:"website,omitempty"`
	//Sbs             string   `json:"sbs,omitempty"`
	//Unrated         bool     `json:"unrated,omitempty"`
	//Subs            string   `json:"subs,omitempty"`
	//ThreeD          bool     `json:"3d,omitempty"`
}

// var plparseclear = []ParseInfo{{}}
var PLParseInfo = pool.NewPool(100, 10, nil, func(b *ParseInfo) {
	clear(b.Languages)
	clear(b.Episodes)
	*b = ParseInfo{}
	b.ListID = -1
})

// getdbmovieidbytitleincache retrieves the database movie ID for the given title from the cache.
// If the movie ID is found in the cache, it sets the DbmovieID field of the ParseInfo struct.
// If the movie ID is not found in the cache, it sets the DbmovieID field to 0.
func (m *ParseInfo) getdbmovieidbytitleincache(title string) {
	if title == "" {
		return
	}
	title = strings.TrimSpace(title)
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](logger.CacheDBMovie, false)
	for idx := range a {
		if a[idx].Str1 == title || a[idx].Str2 == title || strings.EqualFold(a[idx].Str1, title) || strings.EqualFold(a[idx].Str2, title) {
			m.TempID = a[idx].Num2
			if m.moviegetimdbtitle() {
				m.DbmovieID = a[idx].Num2
				return
			}
		}
	}
	b := GetCachedTypeObjArr[DbstaticTwoStringOneInt](logger.CacheTitlesMovie, false)
	for idx := range b {
		if b[idx].Str1 == title || b[idx].Str2 == title || strings.EqualFold(b[idx].Str1, title) || strings.EqualFold(b[idx].Str2, title) {
			m.TempID = b[idx].Num
			if m.moviegetimdbtitle() {
				m.DbmovieID = b[idx].Num
				return
			}
		}
	}
	m.DbmovieID = 0
}

// StripTitlePrefixPostfixGetQual removes any prefix and suffix from the title
// string that match the configured title strip patterns, and returns the
// resulting title. This is used to normalize the title for search and
// matching purposes.
func (m *ParseInfo) StripTitlePrefixPostfixGetQual(quality *config.QualityConfig) {
	if m.Title == "" {
		return
	}

	var idx2 int
	for idx := range quality.TitleStripSuffixForSearch {
		if !logger.ContainsI(m.Title, quality.TitleStripSuffixForSearch[idx]) {
			continue
		}
		idx2 = logger.IndexI(m.Title, quality.TitleStripSuffixForSearch[idx])
		if idx2 != -1 {
			if m.Title[:idx2] != "" {
				str2 := m.Title[:idx2]
				switch str2[len(str2)-1:] {
				case logger.StrDash, logger.StrDot, logger.StrSpace:
					m.Title = strings.TrimRight(str2, "-. ")
				}
			}
		}
	}
	for idx := range quality.TitleStripPrefixForSearch {
		if !logger.HasPrefixI(m.Title, quality.TitleStripPrefixForSearch[idx]) {
			continue
		}
		idx2 = logger.IndexI(m.Title, quality.TitleStripPrefixForSearch[idx])
		if idx2 != -1 {
			str2 := m.Title[idx2+len(quality.TitleStripPrefixForSearch[idx]):]
			if str2 != "" {
				switch str2[0] {
				case '-', '.', ' ':
					m.Title = strings.TrimLeft(str2, "-. ")
				}
			}
		}
	}
}

// moviegetimdbtitle checks if the movie year in the ParseInfo struct matches the year
// retrieved from the database or cache. It returns true if the years match or are
// within one year of each other, and false otherwise.
func (m *ParseInfo) moviegetimdbtitle() bool {
	if config.SettingsGeneral.UseMediaCache {
		year := CacheThreeStringIntIndexFuncGetYear(logger.CacheDBMovie, m.TempID)
		if year != 0 {
			if m.Year == 0 {
				return false
			}
			if m.Year == year || m.Year == year+1 || m.Year == year-1 {
				return true
			}
		}
		return false
	}
	year := Getdatarow1[uint16](false, "select year from dbmovies where id = ?", &m.TempID)
	if year == 0 || (year != 0 && m.Year == 0) {
		return false
	}
	if m.Year == year || m.Year == year+1 || m.Year == year-1 {
		return true
	}
	return false
}

// Findmoviedbidbytitle queries the database to find the movie ID for the given title.
// If the UseMediaCache setting is enabled, it retrieves the movie ID from the cache using the Getdbmovieidbytitleincache method.
// Otherwise, it queries the dbmovies table directly to find the movie ID for the given title, and if not found, it queries the dbmovie_titles table.
// If a movie ID is found, it attempts to retrieve the IMDB title using the Moviegetimdbtitleparser method.
// If the IMDB title is not found, the DbmovieID is set to 0.
func (m *ParseInfo) Findmoviedbidbytitle() {
	m.TempTitle = strings.TrimSpace(m.TempTitle)
	if config.SettingsGeneral.UseMediaCache {
		m.getdbmovieidbytitleincache(m.TempTitle)
		return
	}
	_ = Scanrows1dyn(false, "select id from dbmovies where title = ? COLLATE NOCASE", &m.TempID, &m.TempTitle)
	if m.TempID != 0 && m.moviegetimdbtitle() {
		m.DbmovieID = m.TempID
		return
	}
	_ = Scanrows1dyn(false, "select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE", &m.TempID, &m.TempTitle)
	if m.TempID != 0 && m.moviegetimdbtitle() {
		m.DbmovieID = m.TempID
		return
	}
	m.DbmovieID = 0
}

// Parseresolution returns a string representation of the video resolution based on the height and width of the video.
// The resolution is determined by the following rules:
// - If the height is 360, the resolution is "360p"
// - If the height is greater than 1080, the resolution is "2160p"
// - If the height is greater than 720, the resolution is "1080p"
// - If the height is greater than 576, the resolution is "720p"
// - If the height is greater than 480, the resolution is "576p"
// - If the height is greater than 368, the resolution is "480p"
// - If the height is greater than 360, the resolution is "368p"
// - If the width is 720 and the height is at least 576, the resolution is "576p"
// - If the width is 720 and the height is less than 576, the resolution is "480p"
// - If the width is 1280, the resolution is "720p"
// - If the width is 1920, the resolution is "1080p"
// - If the width is 3840, the resolution is "2160p"
// - If the height and width do not match any of the above cases, the resolution is "Unknown Resolution"
func (m *ParseInfo) Parseresolution() string {
	switch {
	case m.Height == 360:
		return "360p"
	case m.Height > 1080:
		return "2160p"
	case m.Height > 720:
		return "1080p"
	case m.Height > 576:
		return "720p"
	case m.Height > 480:
		return "576p"
	case m.Height > 368:
		return "480p"
	case m.Height > 360:
		return "368p"
	case m.Width == 720:
		if m.Height >= 576 {
			return "576p"
		}
		return "480p"
	case m.Width == 1280:
		return "720p"
	case m.Width == 1920:
		return "1080p"
	case m.Width == 3840:
		return "2160p"
	default:
		return "Unknown Resolution"
	}
}

// MovieFindDBIDByImdbParser queries the database to find the movie ID for the given IMDB ID.
// If the IMDB ID is empty, it sets the DbmovieID to 0 and returns.
// If the UseMediaCache setting is enabled, it uses the CacheThreeStringIntIndexFunc to retrieve the movie ID from the cache.
// Otherwise, it queries the dbmovies table directly to find the movie ID for the given IMDB ID.
func (m *ParseInfo) MovieFindDBIDByImdbParser() {
	if m.Imdb == "" {
		m.DbmovieID = 0
		return
	}
	logger.AddImdbPrefixP(&m.Imdb)
	if config.SettingsGeneral.UseMediaCache {
		m.DbmovieID = CacheThreeStringIntIndexFunc(logger.CacheDBMovie, m.Imdb)
		return
	}
	Scanrows1dyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &m.Imdb)
}

// Getepisodestoimport retrieves a slice of DbstaticTwoUint values representing the episode IDs to import for the given series ID and database series ID.
// If the episode array is empty, it returns an ErrNotFoundEpisode error.
// If there is only one episode and the SerieEpisodeID and DbserieEpisodeID are set, it returns a single-element slice with those values.
// Otherwise, it populates the episode IDs into the returned slice.
func (m *ParseInfo) Getepisodestoimport() error {
	if Getdatarow1[string](false, QueryDbseriesGetIdentifiedByID, m.DbserieID) == logger.StrDate {
		if m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
			m.Episodes = []DbstaticTwoUint{{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID}}
			return nil
		}
		return logger.ErrNotFoundEpisode
	}
	str1, str2 := RegexGetMatchesStr1Str2(true, strRegexSeriesIdentifier, m.Identifier)
	if str1 == "" && str2 == "" {
		return logger.ErrNotFoundEpisode
	}
	var splitby string
	if strings.ContainsRune(str1, 'E') {
		splitby = "E"
	} else if strings.ContainsRune(str1, 'e') {
		splitby = "e"
	} else if strings.ContainsRune(str1, 'X') {
		splitby = "X"
	} else if strings.ContainsRune(str1, 'x') {
		splitby = "x"
	} else if strings.ContainsRune(str1, '-') {
		splitby = logger.StrDash
	}
	if splitby == "" {
		return logger.ErrNotFoundEpisode
	}
	episodeArray := strings.Split(str1, splitby)
	if episodeArray[0] == "" {
		episodeArray = episodeArray[1:]
	}
	if splitby != logger.StrDash && len(episodeArray) == 1 {
		if strings.ContainsRune(episodeArray[0], '-') {
			episodeArray = strings.Split(episodeArray[0], logger.StrDash)
		}
	}

	if m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 && len(episodeArray) == 1 {
		m.Episodes = []DbstaticTwoUint{{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID}}
		return nil
	}
	if len(episodeArray) == 0 {
		return logger.ErrNotFoundEpisode
	}
	m.Episodes = make([]DbstaticTwoUint, 0, len(episodeArray))
	for idx := range episodeArray {
		episodeArray[idx] = strings.TrimPrefix(strings.Trim(episodeArray[idx], "-EX_. "), "0")
		if episodeArray[idx] == "" {
			continue
		}
		m.Episode, _ = strconv.Atoi(episodeArray[idx])
		m.SetDBEpisodeIDfromM()
		if m.DbserieEpisodeID != 0 {
			m.SetEpisodeIDfromM()
			if m.SerieEpisodeID != 0 {
				m.Episodes = append(m.Episodes, DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID})
			}
		}
	}
	return nil
}

// Checktitle checks if the given wanted title and year match the parsed title and year
// from the media file. It compares the wanted title against any alternate titles for the
// media entry from the database. Returns true if the title is unwanted and should be skipped.
func (m *ParseInfo) Checktitle(useseries bool, qualcfg *config.QualityConfig) bool {
	if qualcfg == nil {
		logger.LogDynamicany("debug", "qualcfg empty")
		return true
	}
	if !qualcfg.CheckTitle {
		return false
	}
	var (
		wantedTitle string
		wantedslug  string
		year        uint16
	)
	if useseries {
		GetdatarowArgs(logger.GetStringsMap(useseries, logger.DBMediaTitlesID), &m.DbserieID, &year, &wantedTitle, &wantedslug)
	} else {
		GetdatarowArgs(logger.GetStringsMap(useseries, logger.DBMediaTitlesID), &m.DbmovieID, &year, &wantedTitle, &wantedslug)
	}

	if wantedTitle == "" {
		logger.LogDynamicany("debug", "wanttitle empty")
		return true
	}
	if qualcfg.Name != "" {
		m.StripTitlePrefixPostfixGetQual(qualcfg)
	}
	if m.Title == "" {
		logger.LogDynamicany("debug", "m Title empty")
		return true
	}

	if m.Year != 0 && year != 0 && m.Year != year && (!qualcfg.CheckYear1 || m.Year != year+1 && m.Year != year-1) {
		logger.LogDynamicany("debug", "year different", &logger.StrFound, &m.Year, &logger.StrWanted, &year)
		return true
	}
	if wantedTitle != "" {
		if qualcfg.CheckTitle && ChecknzbtitleB(wantedTitle, wantedslug, m.Title, qualcfg.CheckYear1, m.Year) {
			return false
		}
	}

	var arr []DbstaticTwoStringOneInt

	if config.SettingsGeneral.UseMediaCache {
		if useseries {
			arr = GetCachedTypeObjArr[DbstaticTwoStringOneInt](logger.CacheDBSeriesAlt, false)
		} else {
			arr = GetCachedTypeObjArr[DbstaticTwoStringOneInt](logger.CacheTitlesMovie, false)
		}
	} else {
		if useseries {
			arr = Getentryalternatetitlesdirect(&m.DbserieID, useseries)
		} else {
			arr = Getentryalternatetitlesdirect(&m.DbmovieID, useseries)
		}
	}
	//checked := arr[:0]
	for idx := range arr {
		if arr[idx].Str1 == "" {
			continue
		}
		//checked = append(checked, arr[idx])
		if arr[idx].ChecknzbtitleC(m.Title, qualcfg.CheckYear1, m.Year) {
			//logger.PLArrString.Put(checked)
			return false
		}
	}
	logger.LogDynamicany("debug", "no alternate title found", &logger.StrTitle, &m.Title) //, "checked", arr - better use string array
	//logger.PLArrString.Put(checked)
	return true
}

// AddUnmatched adds an unmatched file to the database. If the file is already in the cache, it returns without adding it. Otherwise, it inserts a new record into the appropriate table (movie_file_unmatcheds or serie_file_unmatcheds) with the file path, list name, and parsed data.
func (m *ParseInfo) AddUnmatched(cfgp *config.MediaTypeConfig, listname *string) {
	if config.SettingsGeneral.UseFileCache {
		if logger.Contains(GetCachedTypeObjArr[string](logger.GetStringsMap(cfgp.Useseries, logger.CacheUnmatched), false), m.TempTitle) {
			return
		}
	}

	ScanrowsNdyn(false, logger.GetStringsMap(cfgp.Useseries, logger.DBIDUnmatchedPathList), &m.TempID, &m.TempTitle, listname) //testing
	if m.TempID == 0 {
		if config.SettingsGeneral.UseFileCache {
			AppendCacheMap(cfgp.Useseries, logger.CacheUnmatched, m.TempTitle)
		}
		m.ExecParsedstring(cfgp.Useseries, "InsertUnmatched", listname, &m.TempTitle) //testing
	} else {
		m.ExecParsedstring(cfgp.Useseries, "UpdateUnmatched", &m.TempID) //testing
	}
}

func (m *ParseInfo) LogTempTitle(logt string, logm string, err error, title string) {
	m.TempTitle = title
	logger.LogDynamicany(logt, logm, err, &logger.StrFile, &m.TempTitle) //testing m.TempTitle
}
func (m *ParseInfo) LogTempTitleNoErr(logt string, logm string, title string) {
	m.TempTitle = title
	logger.LogDynamicany(logt, logm, &logger.StrFile, &m.TempTitle) //testing  m.TempTitle
}

// FindDbserieByName looks up the database series ID by the title of the media.
// It first checks the media cache for the series ID, and if not found, it
// attempts to find the series ID by the title or a slugged version of the title.
// If the series ID is still not found, it checks the alternate titles in the
// database. This function is used to populate the DbserieID field on the
// ParseInfo struct.
func (m *ParseInfo) FindDbserieByName(title string) {
	if title == "" {
		return
	}
	m.TempTitle = strings.TrimSpace(title)
	if config.SettingsGeneral.UseMediaCache {
		m.cacheTwoStringIntIndexFunc(logger.CacheDBSeries, true, m.TempTitle)
		if m.DbserieID != 0 {
			return
		}
		m.cacheTwoStringIntIndexFunc(logger.CacheDBSeriesAlt, true, m.TempTitle)
		if m.DbserieID != 0 {
			return
		}
		slugged := logger.StringToSlug(m.TempTitle)
		if slugged == "" {
			return
		}
		m.cacheTwoStringIntIndexFunc(logger.CacheDBSeries, false, slugged)
		if m.DbserieID != 0 {
			return
		}
		m.cacheTwoStringIntIndexFunc(logger.CacheDBSeriesAlt, false, slugged)
		if m.DbserieID != 0 {
			return
		}
		return
	}

	_ = Scanrows1dyn(false, QueryDbseriesGetIDByName, &m.DbserieID, &m.TempTitle)
	if m.DbserieID != 0 {
		return
	}
	slugged := logger.StringToSlug(m.TempTitle)
	if slugged == "" {
		return
	}
	_ = Scanrows1dyn(false, "select id from dbseries where slug = ?", &m.DbserieID, &slugged)
	if m.DbserieID != 0 {
		return
	}
	_ = Scanrows1dyn(false, "select dbserie_id from Dbserie_alternates where Title = ? COLLATE NOCASE", &m.DbserieID, &m.TempTitle)
	if m.DbserieID == 0 {
		_ = Scanrows1dyn(false, "select dbserie_id from Dbserie_alternates where Slug = ?", &m.DbserieID, &slugged)
	}
}

// RegexGetMatchesStr1 extracts the series name from the filename
// by using a regular expression match. It looks for the series name substring
// in the filename, trims extra characters, and calls findDbserieByName
// to look up the series ID.
func (m *ParseInfo) RegexGetMatchesStr1(cfgp *config.MediaTypeConfig) {
	matchfor := filepath.Base(m.File)
	var matches []int
	if cfgp.Useseries && m.Date != "" {
		matches = Getfirstsubmatchindex(strRegexSeriesTitleDate, matchfor)
	}
	if len(matches) == 0 {
		matches = Getfirstsubmatchindex(strRegexSeriesTitle, matchfor)
	}
	lenm := len(matches)
	if lenm == 0 {
		return
	}
	if lenm < 4 || matches[3] == -1 {
		return
	}
	if strings.ContainsRune(matchfor[matches[2]:matches[3]], '.') {
		m.FindDbserieByName(strings.TrimRight(logger.StringReplaceWith(matchfor[matches[2]:matches[3]], '.', ' '), ".- "))
		return
	}
	m.FindDbserieByName(strings.TrimRight(matchfor[matches[2]:matches[3]], ".- "))
}

// SetEpisodeIDfromM sets the SerieEpisodeID field of the ParseInfo struct based on the SerieID and DbserieEpisodeID fields.
// If SerieID or DbserieEpisodeID is 0, SerieEpisodeID is set to 0.
// Otherwise, it queries the database to find the corresponding serie_episodes record and sets SerieEpisodeID.
func (m *ParseInfo) SetEpisodeIDfromM() {
	if m.SerieID == 0 || m.DbserieEpisodeID == 0 {
		m.SerieEpisodeID = 0
		return
	}
	_ = ScanrowsNdyn(false, "select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", &m.SerieEpisodeID, &m.DbserieEpisodeID, &m.SerieID)
}

// SetDBEpisodeIDfromM sets the DbserieEpisodeID field on the FileParser struct by looking
// up the episode ID in the database based on the season, episode, and identifier fields.
// It first tries looking up by season and episode number strings, then falls back to the identifier.
func (m *ParseInfo) SetDBEpisodeIDfromM() {
	if m.SeasonStr != "" && m.EpisodeStr != "" {
		_ = ScanrowsNdyn(false, "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?", &m.DbserieEpisodeID, &m.DbserieID, &m.Season, &m.Episode)
		if m.DbserieEpisodeID != 0 {
			return
		}
	}

	if m.Identifier != "" {
		if m.DbserieID == 0 {
			return
		}
		err := ScanrowsNdyn(false, "select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier)
		if err != nil && strings.ContainsRune(m.Identifier, '.') {
			err = ScanrowsNdyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier2, &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier, &logger.StrDot, &logger.StrDash)
		}
		if err == nil {
			return
		}
		if strings.ContainsRune(m.Identifier, ' ') {
			_ = ScanrowsNdyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifier2, &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier, &logger.StrSpace, &logger.StrDash)
		}
	}
}

// GenerateIdentifierString generates an identifier string for a movie or episode
// in the format "S{season}E{episode}", where {season} and {episode} are the
// season and episode numbers formatted as strings.
func (m *ParseInfo) GenerateIdentifierString() {
	m.Identifier = logger.JoinStrings("S", m.SeasonStr, "E", m.EpisodeStr) //JoinStrings
}

// Buildparsedstring builds a string representation of the ParseInfo struct by concatenating various fields
// into a single string. The string is built using a buffer from the logger package, which is then returned
// as the result of this method.
func (m *ParseInfo) ExecParsedstring(useseries bool, query string, args ...any) {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)
	if m.AudioID != 0 {
		bld.WriteString(" Audioid: ")
		bld.WriteUInt(m.AudioID)
	}
	if m.CodecID != 0 {
		bld.WriteString(" Codecid: ")
		bld.WriteUInt(m.CodecID)
	}
	if m.QualityID != 0 {
		bld.WriteString(" Qualityid: ")
		bld.WriteUInt(m.QualityID)
	}
	if m.ResolutionID != 0 {
		bld.WriteString(" Resolutionid: ")
		bld.WriteUInt(m.ResolutionID)
	}
	if m.EpisodeStr != "" {
		bld.WriteString(" Episode: ")
		bld.WriteString(m.EpisodeStr)
	}
	if m.Identifier != "" {
		bld.WriteString(" Identifier: ")
		bld.WriteString(m.Identifier)
	}
	if m.ListID != -1 {
		bld.WriteString(" Listname: ")
		bld.WriteInt8(m.ListID)
	}
	if m.SeasonStr != "" {
		bld.WriteString(" Season: ")
		bld.WriteString(m.SeasonStr)
	}
	if m.Title != "" {
		bld.WriteString(" Title: ")
		bld.WriteString(m.Title)
	}
	if m.Tvdb != "" {
		bld.WriteString(" Tvdb: ")
		bld.WriteString(m.Tvdb)
	}
	if m.Imdb != "" {
		bld.WriteString(" Imdb: ")
		bld.WriteString(m.Imdb)
	}
	if m.Year != 0 {
		bld.WriteString(" Year: ")
		bld.WriteUInt16(m.Year)
	}

	//vals := make([]any, 0, len(args)+1)
	vals := logger.PLArrAny.Get()
	vals.Arr = append(vals.Arr, bld.String())
	if len(args) >= 1 {
		vals.Arr = append(vals.Arr, args...)
	}

	execNArg(logger.GetStringsMap(useseries, query), vals.Arr)
	logger.PLArrAny.Put(vals)
}

// ClearArr resets the Languages field of the ParseInfo struct to nil, effectively clearing the array.
func (m *ParseInfo) ClearArr() {
	if m == nil {
		return
	}
	clear(m.Languages)
	m.Languages = nil
	clear(m.Episodes)
	m.Episodes = nil
}

// Close resets the ParseInfo struct to its initial state by setting the Languages field to nil and
// initializing the struct to its zero value.
func (m *ParseInfo) Close() {
	if m == nil {
		return
	}
	PLParseInfo.Put(m)
}

// cleanimdbdbmovie clears the Imdb and DbmovieID fields in the FileParser struct to empty values.
// This is used to reset the state when a lookup fails.
func (m *ParseInfo) Cleanimdbdbmovie() {
	m.Imdb = ""
	m.DbmovieID = 0
}

// CacheTwoStringIntIndexFunc retrieves the DbserieID value from a cached array of DbstaticTwoStringOneInt objects that match the provided string and boolean values. If a matching object is found, the DbserieID value is stored in the ParseInfo struct. If no matching object is found, this method does nothing.
func (m *ParseInfo) cacheTwoStringIntIndexFunc(s string, usestr1 bool, t string) {
	a := GetCachedTypeObjArr[DbstaticTwoStringOneInt](s, false)
	if a == nil {
		return
	}
	for idx := range a {
		if usestr1 && strings.EqualFold(a[idx].Str1, t) {
			m.DbserieID = a[idx].Num
			return
		}
		if !usestr1 && strings.EqualFold(a[idx].Str2, t) {
			m.DbserieID = a[idx].Num
			return
		}
	}
}

// CacheThreeStringIntIndexFuncGetImdb retrieves the IMDB value from a cached array of DbstaticThreeStringTwoInt objects that match the provided string and uint values. If a matching object is found, the IMDB value is stored in the ParseInfo struct. If no matching object is found, this method does nothing.
func (m *ParseInfo) CacheThreeStringIntIndexFuncGetImdb(s string, ip uint) {
	a := GetCachedTypeObjArr[DbstaticThreeStringTwoInt](s, false)
	if a == nil {
		return
	}
	for idx := range a {
		if a[idx].Num2 == ip {
			m.Imdb = a[idx].Str3
			return
		}
	}
}

// getqualityidxbyname searches the given quality table tbl by name
// and returns the index of the matching entry, or -1 if no match is found.
func Getqualityidxbyname(tbl []Qualities, str string) int {
	for idx := range tbl {
		if tbl[idx].Name == str || strings.EqualFold(tbl[idx].Name, str) {
			return idx
		}
	}
	return -1
}

// getqualityidxbyid searches the given quality table tbl by ID
// and returns the index of the matching entry, or -1 if no match is found.
func Getqualityidxbyid(tbl []Qualities, id uint) int {
	for idx := range tbl {
		if tbl[idx].ID == id {
			return idx
		}
	}
	return -1
}

// gettypeids searches through the provided qualitytype slice to find a match for
// the given input string inval. It checks the Strings and Regex fields of each
// QualitiesRegex struct, returning the ID if a match is found. 0 is returned if no
// match is found.
func Gettypeids(inval string, qualitytype []Qualities) uint {
	lenval := len(inval)
	var index, indexmax int
	for idxtype := range qualitytype {
		if qualitytype[idxtype].Strings != "" && !config.SettingsGeneral.DisableParserStringMatch && logger.ContainsI(qualitytype[idxtype].StringsLower, inval) {
			index = logger.IndexI(qualitytype[idxtype].StringsLower, inval)

			indexmax = index + lenval
			if indexmax < len(qualitytype[idxtype].StringsLower) && !checkDigitLetter(rune(qualitytype[idxtype].StringsLower[indexmax : indexmax+1][0])) {
				return 0
			}
			if index > 0 && !checkDigitLetter(rune(qualitytype[idxtype].StringsLower[index-1 : index][0])) {
				return 0
			}
			if qualitytype[idxtype].ID != 0 {
				return qualitytype[idxtype].ID
			}
		}
		if qualitytype[idxtype].UseRegex && qualitytype[idxtype].Regex != "" && RegexGetMatchesFind(qualitytype[idxtype].Regex, inval, 2) {
			return qualitytype[idxtype].ID
		}
	}
	return 0
}

// CheckDigitLetter returns true if the given rune is a digit or letter.
func checkDigitLetter(runev rune) bool {
	if unicode.IsDigit(runev) || unicode.IsLetter(runev) {
		return false
	}
	return true
}

// Parsegroup parses a group of strings from the input string and updates the corresponding fields in the ParseInfo struct.
// The function takes a name string, a boolean onlyifempty, and a slice of group strings as input. It searches for each group string in the input string and extracts the matched substring.
// If the matched substring is not empty and is not part of a larger word, the function updates the corresponding field in the ParseInfo struct based on the name parameter. If onlyifempty is true, the function will only update the field if it is currently empty.
// The function supports the following names: "audio", "codec", "quality", "resolution", "extended", "proper", and "repack".
func (m *ParseInfo) Parsegroup(name string, onlyifempty bool, group []string) {
	var index, indexmax int
	for idx := range group {
		index = logger.IndexI(m.Str, group[idx])
		if index == -1 {
			continue
		}
		indexmax = index + len(group[idx])
		if m.Str[index:indexmax] == "" {
			continue
		}
		if indexmax < len(m.Str) && !checkDigitLetter(rune(m.Str[indexmax : indexmax+1][0])) {
			continue
		}
		if index > 0 && !checkDigitLetter(rune(m.Str[index-1 : index][0])) {
			continue
		}
		value := m.Str[index:indexmax]
		switch name {
		case "audio":
			if onlyifempty && m.Audio != "" {
				continue
			}
			m.Audio = value
		case "codec":
			if onlyifempty && m.Codec != "" {
				continue
			}
			m.Codec = value
		case "quality":
			if onlyifempty && m.Quality != "" {
				continue
			}
			m.Quality = value
		case "resolution":
			if onlyifempty && m.Resolution != "" {
				continue
			}
			m.Resolution = value
		case "extended":
			m.Extended = true
		case "proper":
			m.Proper = true
		case "repack":
			m.Repack = true
		}
		break
	}
}

// ParsegroupEntry parses a group of characters from the input string and updates the corresponding fields in the ParseInfo struct.
// The function takes a name string and a group string as input. It searches for the group string in the input string and extracts the matched substring.
// If the matched substring is not empty and is not part of a larger word, the function updates the corresponding field in the ParseInfo struct based on the name parameter.
// The function supports the following names: "audio", "codec", "quality", "resolution", "extended", "proper", and "repack".
func (m *ParseInfo) ParsegroupEntry(name string, group string) {
	index := logger.IndexI(m.Str, group)
	if index == -1 {
		return
	}

	indexmax := index + len(group)
	if indexmax < len(m.Str) && !checkDigitLetter(rune(m.Str[indexmax : indexmax+1][0])) {
		return
	}
	if index > 0 && !checkDigitLetter(rune(m.Str[index-1 : index][0])) {
		return
	}

	if m.Str[index:indexmax] == "" {
		return
	}
	switch name {
	case "audio":
		m.Audio = m.Str[index:indexmax]
	case "codec":
		m.Codec = m.Str[index:indexmax]
	case "quality":
		m.Quality = m.Str[index:indexmax]
	case "resolution":
		m.Resolution = m.Str[index:indexmax]
	case "extended":
		m.Extended = true
	case "proper":
		m.Proper = true
	case "repack":
		m.Repack = true
	}
}
