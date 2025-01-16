package database

import (
	"iter"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// ParseInfo is a struct containing parsed information about media files.
type ParseInfo struct {
	Episodes []DbstaticTwoUint `json:"-"`
	// Languages is a list of language codes
	Languages []string `json:"languages,omitempty"`
	Str       string   // used internally
	// File is the path to the media file
	File string
	// SeasonStr is the season number as a string, if applicable
	SeasonStr string `json:"seasonstr,omitempty"`
	// EpisodeStr is the episode number as a string, if applicable
	EpisodeStr string `json:"episodestr,omitempty"`
	// Title is the title of the media
	Title string
	// Resolution is the video resolution
	Resolution string `json:"resolution,omitempty"`
	// Quality is the video quality description
	Quality string `json:"quality,omitempty"`
	// Codec is the video codec
	Codec string `json:"codec,omitempty"`
	// Audio is the audio description
	Audio      string `json:"audio,omitempty"`
	RuntimeStr string `json:"-"`
	TempTitle  string
	// Identifier is an identifier string
	Identifier string `json:"identifier,omitempty"`
	// Date is the release date
	Date string `json:"date,omitempty"`
	// Imdb is the IMDB ID
	Imdb string `json:"imdb,omitempty"`
	// Tvdb is the TVDB ID
	Tvdb string `json:"tvdb,omitempty"`
	// Priority is the priority for downloading
	Priority int `json:"priority,omitempty"`
	// Season is the season number, if applicable
	Season int `json:"season,omitempty"`
	// Episode is the episode number, if applicable
	Episode int `json:"episode,omitempty"`
	// Runtime is the runtime in minutes
	Runtime int `json:"runtime,omitempty"`
	// ListID is the ID of the list this came from
	ListID       int
	FirstIDX     int
	FirstYearIDX int
	// Height is the video height in pixels
	Height int `json:"height,omitempty"`
	// Width is the video width in pixels
	Width  int `json:"width,omitempty"`
	TempID uint
	// ResolutionID is the database ID of the resolution
	ResolutionID uint `json:"resolutionid,omitempty"`
	// QualityID is the database ID of the quality
	QualityID uint `json:"qualityid,omitempty"`
	// CodecID is the database ID of the codec
	CodecID uint `json:"codecid,omitempty"`
	// AudioID is the database ID of the audio
	AudioID uint `json:"audioid,omitempty"`
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
	// Year is the year of release
	Year uint16 `json:"year,omitempty"`
	// Extended is a flag indicating if it is an extended version
	Extended bool `json:"extended,omitempty"`
	// Proper is a flag indicating if it is a proper release
	Proper bool `json:"proper,omitempty"`
	// Repack is a flag indicating if it is a repack release
	Repack bool `json:"repack,omitempty"`

	// SluggedTitle     string
	// Listname         string   `json:"listname,omitempty"`
	// ListCfg *config.ListsConfig
	// Group           string   `json:"group,omitempty"`
	// Region          string   `json:"region,omitempty"`
	// Hardcoded       bool     `json:"hardcoded,omitempty"`
	// Container       string   `json:"container,omitempty"`
	// Widescreen      bool     `json:"widescreen,omitempty"`
	// Website         string   `json:"website,omitempty"`
	// Sbs             string   `json:"sbs,omitempty"`
	// Unrated         bool     `json:"unrated,omitempty"`
	// Subs            string   `json:"subs,omitempty"`
	// ThreeD          bool     `json:"3d,omitempty"`
}
type mapslugged struct {
	Slugged string
	Default string
}

var PLParseInfo = pool.NewPool(100, 10, nil, func(b *ParseInfo) bool {
	clear(b.Languages)
	clear(b.Episodes)
	*b = ParseInfo{ListID: -1}
	return false
})
var arrExtended = []string{"extended", "extended cut", "extended.cut", "extended-cut"}

var mapSlugged = map[string]mapslugged{
	"dbmovies": {
		Slugged: "select id from dbmovies where slug = ?",
		Default: "select id from dbmovies where title = ? COLLATE NOCASE",
	},
	"dbmoviesalt": {
		Slugged: "select dbmovie_id from dbmovie_titles where slug = ?",
		Default: "select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE",
	},
	"dbseries": {
		Slugged: "select id from dbseries where slug = ?",
		Default: "select id from dbseries where seriename = ? COLLATE NOCASE",
	},
	"dbseriesalt": {
		Slugged: "select dbserie_id from dbserie_alternates where slug = ?",
		Default: "select dbserie_id from dbserie_alternates where title = ? COLLATE NOCASE",
	},
}

// GetSluggedMap returns the appropriate SQL query string based on whether the
// caller wants to use a slugged or default lookup. The returned string can be
// used to query the database for a record matching the provided type string.
func GetSluggedMap(slugged bool, typestr string) string {
	if slugged {
		return mapSlugged[typestr].Slugged
	}
	return mapSlugged[typestr].Default
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
				switch m.Title[:idx2][len(m.Title[:idx2])-1] {
				case '-', '.', ' ':
					m.Title = logger.TrimRight(m.Title[:idx2], '-', '.', ' ')
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
			if m.Title[idx2+len(quality.TitleStripPrefixForSearch[idx]):] != "" {
				switch m.Title[idx2+len(quality.TitleStripPrefixForSearch[idx]):][0] {
				case '-', '.', ' ':
					m.Title = logger.TrimLeft(m.Title[idx2+len(quality.TitleStripPrefixForSearch[idx]):], '-', '.', ' ')
				}
			}
		}
	}
}

// moviegetimdbtitle checks if the movie year in the ParseInfo struct matches the year
// retrieved from the database or cache. It returns true if the years match or are
// within one year of each other, and false otherwise.
func (m *ParseInfo) moviegetimdbtitle(dbid *uint) bool {
	if config.SettingsGeneral.UseMediaCache {
		year := CacheThreeStringIntIndexFuncGetYear(logger.CacheDBMovie, *dbid)
		if year != 0 {
			if m.Year == 0 {
				return false
			}
			if m.Year == year || m.Year == year+1 || m.Year == year-1 {
				return true
			}
			return false
		}
		return false
	}
	year := Getdatarow1[uint16](false, "select year from dbmovies where id = ?", dbid)
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
func (m *ParseInfo) Findmoviedbidbytitle(slugged bool) {
	if m == nil {
		return
	}
	if m.TempTitle == "" {
		m.DbmovieID = 0
		return
	}
	if slugged {
		m.TempTitle = logger.StringToSlug(m.Title)
	}
	if config.SettingsGeneral.UseMediaCache {
		c := GetCachedThreeStringArr(logger.CacheDBMovie, false, true)
		for idx := range c {
			if c[idx].Str1 == m.TempTitle || c[idx].Str2 == m.TempTitle || strings.EqualFold(c[idx].Str1, m.TempTitle) || strings.EqualFold(c[idx].Str2, m.TempTitle) {
				if m.moviegetimdbtitle(&c[idx].Num2) {
					m.DbmovieID = c[idx].Num2
					return
				}
			}
		}
		d := GetCachedTwoStringArr(logger.CacheTitlesMovie, false, true)
		for idx := range d {
			if d[idx].Str1 == m.TempTitle || d[idx].Str2 == m.TempTitle || strings.EqualFold(d[idx].Str1, m.TempTitle) || strings.EqualFold(d[idx].Str2, m.TempTitle) {
				if m.moviegetimdbtitle(&d[idx].Num) {
					m.DbmovieID = d[idx].Num
					return
				}
			}
		}
		m.DbmovieID = 0
		return
	}
	Scanrows1dyn(false, GetSluggedMap(slugged, "dbmovies"), &m.DbmovieID, &m.TempTitle)
	if m.DbmovieID != 0 && m.moviegetimdbtitle(&m.DbmovieID) {
		return
	}
	Scanrows1dyn(false, GetSluggedMap(slugged, "dbmoviesalt"), &m.DbmovieID, &m.TempTitle)
	if m.DbmovieID != 0 && m.moviegetimdbtitle(&m.DbmovieID) {
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
// - If the height and width do not match any of the above cases, the resolution is "Unknown Resolution".
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
	m.Imdb = logger.AddImdbPrefixP(m.Imdb)
	if config.SettingsGeneral.UseMediaCache {
		m.DbmovieID = CacheThreeStringIntIndexFunc(logger.CacheDBMovie, &m.Imdb)
		return
	}
	Scanrows1dyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &m.Imdb)
}

// Getepisodestoimport retrieves a slice of DbstaticTwoUint values representing the episode IDs to import for the given series ID and database series ID.
// If the episode array is empty, it returns an ErrNotFoundEpisode error.
// If there is only one episode and the SerieEpisodeID and DbserieEpisodeID are set, it returns a single-element slice with those values.
// Otherwise, it populates the episode IDs into the returned slice.
func (m *ParseInfo) Getepisodestoimport() error {
	if Getdatarow1[string](false, QueryDbseriesGetIdentifiedByID, &m.DbserieID) == logger.StrDate {
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
	var err error
	for idx := range episodeArray {
		m.Episode, err = strconv.Atoi(logger.TrimLeft(logger.Trim(episodeArray[idx], '-', '.', ' ', '_', 'E', 'X'), '0'))
		if err != nil {
			m.Episode = 0
			return logger.ErrNotFoundEpisode
		}
		m.SetDBEpisodeIDfromM()
		if m.DbserieEpisodeID != 0 {
			m.SetEpisodeIDfromM()
			if m.SerieEpisodeID != 0 {
				if idx == 0 {
					m.Episodes = []DbstaticTwoUint{{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID}}
				} else {
					m.Episodes = append(m.Episodes, DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID})
				}
			}
		}
	}
	return nil
}

// Checktitle checks if the given wanted title and year match the parsed title and year
// from the media file. It compares the wanted title against any alternate titles for the
// media entry from the database. Returns true if the title is unwanted and should be skipped.
func (m *ParseInfo) Checktitle(cfgp *config.MediaTypeConfig, qualcfg *config.QualityConfig, title string) bool {
	if qualcfg == nil {
		logger.LogDynamicany0("debug", "qualcfg empty")
		return true
	}
	if !qualcfg.CheckTitle {
		return false
	}
	var (
		wantedTitle string
		wantedslug  string
		year        uint16
		id          uint
	)
	if cfgp.Useseries {
		id = m.DbserieID
	} else {
		id = m.DbmovieID
	}
	GetdatarowArgs(logger.GetStringsMap(cfgp.Useseries, logger.DBMediaTitlesID), &id, &year, &wantedTitle, &wantedslug)

	if wantedTitle == "" {
		logger.LogDynamicany0("debug", "wanttitle empty")
		return true
	}
	if qualcfg.Name != "" {
		m.StripTitlePrefixPostfixGetQual(qualcfg)
	}
	if m.Title == "" {
		logger.LogDynamicany0("debug", "m Title empty")
		return true
	}

	if m.Year != 0 && year != 0 {
		if (m.Year != year && !qualcfg.CheckYear1) || (qualcfg.CheckYear1 && (m.Year != year && m.Year != year+1 && m.Year != year-1)) {
			logger.LogDynamicany("debug", "year different", &logger.StrFound, &m.Year, &logger.StrWanted, &year)
			return true
		}
	}
	if wantedTitle != "" {
		if qualcfg.CheckTitle && ChecknzbtitleB(wantedTitle, wantedslug, title, qualcfg.CheckYear1, m.Year) {
			return false
		}
	}
	if !qualcfg.CheckTitle {
		logger.LogDynamicany1String("debug", "no alternate title check allowed", logger.StrTitle, m.Title) // , "checked", arr - better use string array
		return true
	}

	var arr []DbstaticTwoStringOneInt

	if config.SettingsGeneral.UseMediaCache {
		arr = GetCachedTwoStringArr(logger.GetStringsMap(cfgp.Useseries, logger.CacheMediaTitles), false, true)
	} else {
		arr = Getentryalternatetitlesdirect(&id, cfgp.Useseries)
	}
	if len(arr) == 0 {
		logger.LogDynamicany1String("debug", "no alternate titles found", logger.StrTitle, m.Title) // , "checked", arr - better use string array
		return true
	}
	for idx := range FilterDbstaticTwoStringOneInt(arr, id) {
		if idx.Str1 == "" {
			continue
		}
		if ChecknzbtitleB(idx.Str1, idx.Str2, title, qualcfg.CheckYear1, m.Year) {
			return false
		}
	}
	logger.LogDynamicany("debug", "no alternate title match found", &logger.StrTitle, &m.Title, "Year", &m.Year, "Titles", GetDbstaticTwoStringOneInt(arr, id))
	return true
}

// FilterDbstaticTwoStringOneInt filters a slice of DbstaticTwoStringOneInt structs by the provided id. It returns a sequence that yields the filtered elements.
func FilterDbstaticTwoStringOneInt(s []DbstaticTwoStringOneInt, id uint) iter.Seq[DbstaticTwoStringOneInt] {
	return func(yield func(DbstaticTwoStringOneInt) bool) {
		for idx := range s {
			if s[idx].Num != id {
				continue
			}
			if !yield(s[idx]) {
				return
			}
		}
	}
}

// AddUnmatched adds an unmatched file to the database. If the file is already in the cache, it returns without adding it. Otherwise, it inserts a new record into the appropriate table (movie_file_unmatcheds or serie_file_unmatcheds) with the file path, list name, and parsed data.
func (m *ParseInfo) AddUnmatched(cfgp *config.MediaTypeConfig, listname *string, err error) {
	if config.SettingsGeneral.UseFileCache {
		if slices.Contains(GetCachedStringArr(logger.GetStringsMap(cfgp.Useseries, logger.CacheUnmatched), false, true), m.TempTitle) {
			return
		}
	}
	m.ExecParsed(cfgp, err, listname)
}

// ExecParsed adds an unmatched file to the database or updates an existing unmatched file record. It constructs a string representation of the parsed file information and inserts a new record or updates an existing record in the appropriate table (movie_file_unmatcheds or serie_file_unmatcheds).
func (m *ParseInfo) ExecParsed(cfgp *config.MediaTypeConfig, err error, listname *string) {
	id := Getdatarow2[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBIDUnmatchedPathList), &m.TempTitle, listname) // testing
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
		bld.WriteInt(m.ListID)
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
	if err != nil {
		bld.WriteString(" Error: ")
		bld.WriteString(err.Error())
	}

	str := bld.String()

	if id == 0 {
		if config.SettingsGeneral.UseFileCache {
			AppendCacheMap(cfgp.Useseries, logger.CacheUnmatched, m.TempTitle)
		}
		exec3(logger.GetStringsMap(cfgp.Useseries, "InsertUnmatched"), &str, listname, &m.TempTitle)
	} else {
		exec(logger.GetStringsMap(cfgp.Useseries, "UpdateUnmatched"), &str, &id, nil)
	}
}

// FindDbserieByName looks up the database series ID by the title of the media.
// It first checks the media cache for the series ID, and if not found, it
// attempts to find the series ID by the title or a slugged version of the title.
// If the series ID is still not found, it checks the alternate titles in the
// database. This function is used to populate the DbserieID field on the
// ParseInfo struct.
func (m *ParseInfo) FindDbserieByName(slugged bool) {
	if m.TempTitle == "" {
		return
	}
	if slugged {
		m.TempTitle = logger.StringToSlug(m.TempTitle)
	}
	if config.SettingsGeneral.UseMediaCache {
		for _, a := range GetCachedThreeStringArr(logger.CacheDBSeries, false, true) {
			if a.Str1 == m.TempTitle || a.Str2 == m.TempTitle || strings.EqualFold(a.Str1, m.TempTitle) || strings.EqualFold(a.Str2, m.TempTitle) {
				m.DbserieID = a.Num2
				return
			}
		}
		for _, b := range GetCachedTwoStringArr(logger.CacheDBSeriesAlt, false, true) {
			if b.Str1 == m.TempTitle || b.Str2 == m.TempTitle || strings.EqualFold(b.Str1, m.TempTitle) || strings.EqualFold(b.Str2, m.TempTitle) {
				m.DbserieID = b.Num
				return
			}
		}
		m.DbserieID = 0
		return
	}
	if m.DbserieID == 0 {
		Scanrows1dyn(false, GetSluggedMap(slugged, "dbseries"), &m.DbserieID, &m.TempTitle)
		if m.DbserieID != 0 {
			return
		}
		Scanrows1dyn(false, GetSluggedMap(slugged, "dbseriesalt"), &m.DbserieID, &m.TempTitle)
	}
}

// RegexGetMatchesStr1 extracts the series name from the filename
// by using a regular expression match. It looks for the series name substring
// in the filename, trims extra characters, and calls findDbserieByName
// to look up the series ID.
func (m *ParseInfo) RegexGetMatchesStr1(cfgp *config.MediaTypeConfig) {
	matchfor := filepath.Base(m.File)

	runrgx := strRegexSeriesTitle
	if cfgp.Useseries && m.Date != "" {
		runrgx = strRegexSeriesTitleDate
	}
	matches := RunRetRegex(runrgx, matchfor, false)
	if len(matches) == 0 && cfgp.Useseries && m.Date != "" {
		matches = RunRetRegex(strRegexSeriesTitle, matchfor, false)
	}
	if len(matches) == 0 {
		return
	}
	if len(matches) < 4 || matches[3] == -1 {
		return
	}
	if strings.ContainsRune(matchfor[matches[2]:matches[3]], '.') {
		title := logger.TrimRight(logger.StringReplaceWith(matchfor[matches[2]:matches[3]], '.', ' '), '-', '.', ' ')
		if title == m.Title {
			return
		}
		m.FindDbserieByNameWithSlug(title)
		return
	}
	title := logger.TrimRight(matchfor[matches[2]:matches[3]], '-', '.', ' ')
	if title == m.Title {
		return
	}
	m.FindDbserieByNameWithSlug(title)
}

// FindDbserieByNameWithSlug attempts to find a database series by the provided title string.
// It first trims any leading or trailing whitespace from the title, then calls FindDbserieByName
// with the trimmed title. If no series is found, it calls FindDbserieByName again with the
// slugged version of the title.
func (m *ParseInfo) FindDbserieByNameWithSlug(title string) {
	m.TempTitle = logger.TrimSpace(title)
	m.FindDbserieByName(false)
	if m.DbserieID == 0 {
		m.FindDbserieByName(true)
	}
}

// SetEpisodeIDfromM sets the SerieEpisodeID field of the ParseInfo struct based on the SerieID and DbserieEpisodeID fields.
// If SerieID or DbserieEpisodeID is 0, SerieEpisodeID is set to 0.
// Otherwise, it queries the database to find the corresponding serie_episodes record and sets SerieEpisodeID.
func (m *ParseInfo) SetEpisodeIDfromM() {
	if m.SerieID == 0 || m.DbserieEpisodeID == 0 {
		m.SerieEpisodeID = 0
		return
	}
	Scanrows2dyn(false, "select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", &m.SerieEpisodeID, &m.DbserieEpisodeID, &m.SerieID)
}

// SetDBEpisodeIDfromM sets the DbserieEpisodeID field on the FileParser struct by looking
// up the episode ID in the database based on the season, episode, and identifier fields.
// It first tries looking up by season and episode number strings, then falls back to the identifier.
func (m *ParseInfo) SetDBEpisodeIDfromM() {
	if m.DbserieID == 0 {
		m.DbserieEpisodeID = 0
		return
	}
	if m.SeasonStr != "" && m.EpisodeStr != "" {
		Scanrows3dyn(false, "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?", &m.DbserieEpisodeID, &m.DbserieID, &m.SeasonStr, &m.EpisodeStr)
		if m.DbserieEpisodeID != 0 {
			return
		}
	}

	if m.Identifier != "" {
		Scanrows2dyn(false, "select id from dbserie_episodes where dbserie_id = ? and identifier = ? COLLATE NOCASE", &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier)
		if m.DbserieEpisodeID != 0 {
			return
		}
		if strings.ContainsRune(m.Identifier, '.') {
			Scanrows2dyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifierDot, &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier)
			if m.DbserieEpisodeID != 0 {
				return
			}
		}
		if strings.ContainsRune(m.Identifier, ' ') {
			Scanrows2dyn(false, QueryDBSerieEpisodeGetIDByDBSerieIDIdentifierDash, &m.DbserieEpisodeID, &m.DbserieID, &m.Identifier)
		}
	}
}

// GenerateIdentifierString generates an identifier string for a movie or episode
// in the format "S{season}E{episode}", where {season} and {episode} are the
// season and episode numbers formatted as strings.
func (m *ParseInfo) GenerateIdentifierString() {
	m.Identifier = ("S" + m.SeasonStr + "E" + m.EpisodeStr)
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
	PLParseInfo.Put(m)
}

// Ceanimdbdbmovie clears the Imdb and DbmovieID fields in the FileParser struct to empty values.
// This is used to reset the state when a lookup fails.
func (m *ParseInfo) Cleanimdbdbmovie() {
	m.Imdb = ""
	m.DbmovieID = 0
}

// CacheThreeStringIntIndexFuncGetImdb retrieves the IMDB value from a cached array of DbstaticThreeStringTwoInt objects that match the provided string and uint values. If a matching object is found, the IMDB value is stored in the ParseInfo struct. If no matching object is found, this method does nothing.
func (m *ParseInfo) CacheThreeStringIntIndexFuncGetImdb() {
	for _, a := range GetCachedThreeStringArr(logger.CacheDBMovie, false, true) {
		if a.Num2 == m.DbmovieID {
			m.Imdb = a.Str3
			return
		}
	}
}

// Getqualityidxbyname searches the given quality table tbl by name
// and returns the index of the matching entry, or -1 if no match is found.
func Getqualityidxbyname(tbl []Qualities, cfgp *config.MediaTypeConfig, reso bool) int {
	var str string
	switch reso {
	case true:
		str = cfgp.DefaultResolution
	case false:
		str = cfgp.DefaultQuality
	}
	for idx := range tbl {
		if tbl[idx].Name == str || strings.EqualFold(tbl[idx].Name, str) {
			return idx
		}
	}
	return -1
}

// Getqualityidxbyid searches the given quality table tbl by ID
// and returns the index of the matching entry, or -1 if no match is found.
func (m *ParseInfo) Getqualityidxbyid(tbl []Qualities, i uint8) int {
	var id uint
	switch i {
	case 1:
		id = m.ResolutionID
	case 2:
		id = m.QualityID
	case 3:
		id = m.AudioID
	case 4:
		id = m.CodecID
	}
	for idx := range tbl {
		if tbl[idx].ID == id {
			return idx
		}
	}
	return -1
}

// Gettypeids searches through the provided qualitytype slice to find a match for
// the given input string inval. It checks the Strings and Regex fields of each
// QualitiesRegex struct, returning the ID if a match is found. 0 is returned if no
// match is found.
func (m *ParseInfo) Gettypeids(id uint8) uint {
	var inval string
	var qualitytype []Qualities
	switch id {
	case 1:
		inval = m.Resolution
		qualitytype = DBConnect.GetresolutionsIn
	case 2:
		inval = m.Quality
		qualitytype = DBConnect.GetqualitiesIn
	case 3:
		inval = m.Audio
		qualitytype = DBConnect.GetaudiosIn
	case 4:
		inval = m.Codec
		qualitytype = DBConnect.GetcodecsIn
	}
	lenval := len(inval)
	var index, indexmax int
	for idx := range qualitytype {
		if qualitytype[idx].Strings != "" && !config.SettingsGeneral.DisableParserStringMatch && logger.ContainsI(qualitytype[idx].StringsLower, inval) {
			index = logger.IndexI(qualitytype[idx].StringsLower, inval)

			indexmax = index + lenval
			if indexmax < len(qualitytype[idx].StringsLower) && !checkDigitLetter((qualitytype[idx].StringsLower[indexmax])) {
				return 0
			}
			if index > 0 && !checkDigitLetter((qualitytype[idx].StringsLower[index-1])) {
				return 0
			}
			if qualitytype[idx].ID != 0 {
				return qualitytype[idx].ID
			}
		}
		if qualitytype[idx].UseRegex && qualitytype[idx].Regex != "" && RegexGetMatchesFind(qualitytype[idx].Regex, inval, 2) {
			return qualitytype[idx].ID
		}
	}
	return 0
}

// CheckDigitLetter returns true if the given rune is a digit or letter.
func checkDigitLetter(b byte) bool {
	return !((b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z'))
}

// Parsegroup parses a group of strings from the input string and updates the corresponding fields in the ParseInfo struct.
// The function takes a name string, a boolean onlyifempty, and a slice of group strings as input. It searches for each group string in the input string and extracts the matched substring.
// If the matched substring is not empty and is not part of a larger word, the function updates the corresponding field in the ParseInfo struct based on the name parameter. If onlyifempty is true, the function will only update the field if it is currently empty.
// The function supports the following names: "audio", "codec", "quality", "resolution", "extended", "proper", and "repack".
func (m *ParseInfo) Parsegroup(name string, onlyifempty bool) {
	var group []string
	switch name {
	case "audio":
		group = DBConnect.AudioStrIn
	case "codec":
		group = DBConnect.CodecStrIn
	case "quality":
		group = DBConnect.QualityStrIn
	case "resolution":
		group = DBConnect.ResolutionStrIn
	case "extended":
		group = arrExtended
	}
	for idx := range group {
		index := logger.IndexI(m.Str, group[idx])
		if index == -1 {
			continue
		}
		indexmax := index + len(group[idx])

		if m.Str[index:indexmax] == "" {
			continue
		}
		if indexmax < len(m.Str) && !checkDigitLetter((m.Str[indexmax])) {
			continue
		}
		if index > 0 && !checkDigitLetter((m.Str[index-1])) {
			continue
		}
		if m.FirstIDX == 0 || index < m.FirstIDX {
			m.FirstIDX = index
		}
		switch name {
		case "audio":
			if onlyifempty && m.Audio != "" {
				continue
			}

			m.Audio = m.getstrvalue(index, indexmax)
		case "codec":
			if onlyifempty && m.Codec != "" {
				continue
			}
			m.Codec = m.getstrvalue(index, indexmax)
		case "quality":
			if onlyifempty && m.Quality != "" {
				continue
			}
			m.Quality = m.getstrvalue(index, indexmax)
		case "resolution":
			if onlyifempty && m.Resolution != "" {
				continue
			}
			m.Resolution = m.getstrvalue(index, indexmax)
		case "extended":
			m.Extended = true
		case "proper":
			m.Proper = true
		case "repack":
			m.Repack = true
		}
	}
}

// getstrvalue returns the substring of m.Str between the given index and indexmax.
func (m *ParseInfo) getstrvalue(index, indexmax int) string {
	return m.Str[index:indexmax]
}

// ParsegroupEntry parses a group of characters from the input string and updates the corresponding fields in the ParseInfo struct.
// The function takes a name string and a group string as input. It searches for the group string in the input string and extracts the matched substring.
// If the matched substring is not empty and is not part of a larger word, the function updates the corresponding field in the ParseInfo struct based on the name parameter.
// The function supports the following names: "audio", "codec", "quality", "resolution", "extended", "proper", and "repack".
func (m *ParseInfo) ParsegroupEntry(group string) {
	index := logger.IndexI(m.Str, group)
	if index == -1 {
		return
	}
	indexmax := index + len(group)
	if indexmax < len(m.Str) && !checkDigitLetter((m.Str[indexmax])) {
		return
	}
	if index > 0 && !checkDigitLetter((m.Str[index-1])) {
		return
	}

	if m.Str[index:indexmax] == "" {
		return
	}
	switch group {
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

	if m.FirstIDX == 0 || index < m.FirstIDX {
		m.FirstIDX = index
	}
}
