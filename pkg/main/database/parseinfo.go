package database

import (
	"iter"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// Package-level constants for music/audiobook matching — allocated once, not per call.
var (
	variousArtistNames = []string{
		"Various Artists",
		"Various",
		"VA",
		"V.A.",
		"V/A",
		"Soundtrack",
		"Original Soundtrack",
		"OST",
	}

	variousAuthorNames = []string{
		"Various Authors",
		"Various",
		"VA",
		"V.A.",
		"Anthology",
		"Multiple Authors",
	}

	// multiArtistSeparators are already lowercase — no ToLower needed at runtime.
	multiArtistSeparators = []string{
		" & ",
		" and ",
		" + ",
		" feat. ",
		" feat ",
		" ft. ",
		" ft ",
		" featuring ",
		" with ",
		" vs ",
		" vs. ",
		" x ",
		"  ", // double space (missing separator)
	}

	vaForms = []string{"va", "v.a.", "v.a"}

	// Pre-computed slugs — parallel to the name slices above, populated in init().
	variousArtistSlugs []string
	variousAuthorSlugs []string
)

func init() {
	variousArtistSlugs = make([]string, len(variousArtistNames))
	for i, n := range variousArtistNames {
		variousArtistSlugs[i] = logger.StringToSlug(n)
	}

	variousAuthorSlugs = make([]string, len(variousAuthorNames))
	for i, n := range variousAuthorNames {
		variousAuthorSlugs[i] = logger.StringToSlug(n)
	}
}

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
	// AbsoluteEpisode is the absolute episode number (for anime and shows with continuous numbering)
	AbsoluteEpisode int `json:"absolute_episode,omitempty"`
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
	// DbbookID is the database ID of the book
	DbbookID uint `json:"dbbookid,omitempty"`
	// BookID is the application ID of the book
	BookID uint `json:"bookid,omitempty"`
	// DbaudiobookID is the database ID of the audiobook
	DbaudiobookID uint `json:"dbaudiobookid,omitempty"`
	// AudiobookID is the application ID of the audiobook
	AudiobookID uint `json:"audiobookid,omitempty"`
	// DbalbumID is the database ID of the music album
	DbalbumID uint `json:"dbalbumid,omitempty"`
	// AlbumID is the application ID of the music album
	AlbumID uint `json:"albumid,omitempty"`
	// Year is the year of release
	Year uint16 `json:"year,omitempty"`
	// ISBN is the ISBN-13 or ISBN-10 identifier for books
	ISBN string `json:"isbn,omitempty"`
	// ASIN is the Amazon ASIN identifier for audiobooks
	ASIN string `json:"asin,omitempty"`
	// MusicBrainzID is the MusicBrainz release ID for music
	MusicBrainzID string `json:"musicbrainz_id,omitempty"`
	// UPC is the Universal Product Code for music
	UPC string `json:"upc,omitempty"`
	// Artist is the artist name for music
	Artist string `json:"artist,omitempty"`
	// AudioFormat is the audio codec/format (mp3, flac, aac, etc.) for music/audiobooks
	AudioFormat string `json:"audio_format,omitempty"`
	// AudioBitrate is the audio bitrate in kbps for music/audiobooks
	AudioBitrate int `json:"audio_bitrate,omitempty"`
	// AudioSampleRate is the sample rate in Hz for music/audiobooks
	AudioSampleRate int `json:"audio_sample_rate,omitempty"`
	// AudioBitDepth is the bit depth (16, 24, 32) for lossless audio
	AudioBitDepth int `json:"audio_bit_depth,omitempty"`
	// AudioFormatID is the database ID for the audio format
	AudioFormatID uint `json:"audio_format_id,omitempty"`
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

// StripTitlePrefixPostfixGetQual removes any prefix and suffix from the title
// string that match the configured title strip patterns, and returns the
// resulting title. This is used to normalize the title for search and
// matching purposes.
func (m *ParseInfo) StripTitlePrefixPostfixGetQual(quality *config.QualityConfig) {
	if m.Title == "" {
		return
	}

	for _, suffix := range quality.TitleStripSuffixForSearch {
		if idx := logger.IndexI(m.Title, suffix); idx != -1 {
			if newTitle := m.Title[:idx]; newTitle != "" {
				switch newTitle[len(newTitle)-1] {
				case '-', '.', ' ':
					m.Title = logger.TrimRight(newTitle, '-', '.', ' ')
				default:
					m.Title = newTitle
				}
			}

			break // Only process first match
		}
	}

	for _, prefix := range quality.TitleStripPrefixForSearch {
		if logger.HasPrefixI(m.Title, prefix) {
			if idx := logger.IndexI(m.Title, prefix); idx != -1 {
				if newTitle := m.Title[idx+len(prefix):]; newTitle != "" {
					switch newTitle[0] {
					case '-', '.', ' ':
						m.Title = logger.TrimLeft(newTitle, '-', '.', ' ')
					default:
						m.Title = newTitle
					}
				}
			}

			break // Only process first match
		}
	}
}

// moviegetimdbtitle checks if the movie year in the ParseInfo struct matches the year
// retrieved from the database or cache. It returns true if the years match or are
// within one year of each other, and false otherwise.
func (m *ParseInfo) moviegetimdbtitle(dbid *uint) bool {
	if m.Year == 0 {
		return false
	}

	var year uint16
	if config.GetSettingsGeneral().UseMediaCache {
		year = CacheThreeStringIntIndexFuncGetYearFast(logger.CacheDBMovie, *dbid)
	} else {
		year = Getdatarow[uint16](false, "select year from dbmovies where id = ?", dbid)
	}

	if year == 0 {
		return false
	}

	// Check if years match within ±1 year
	return m.Year >= year-1 && m.Year <= year+1
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

	if config.GetSettingsGeneral().UseMediaCache {
		m.findMovieInCache()
		return
	}

	m.findMovieInDB(slugged)
}

// findMovieInCache searches for a movie in the media cache by checking both the main movie cache
// and the movie titles cache. It attempts to match the movie title and verify the year.
// If a matching movie is found, it sets the DbmovieID to the found movie's ID.
// If no match is found, it sets DbmovieID to 0.
func (m *ParseInfo) findMovieInCache() {
	// Search in main movie cache
	c := GetCachedThreeStringArr(logger.CacheDBMovie, false, true)
	for idx := range c {
		if m.matchesTitle(c[idx].Str1, c[idx].Str2) && m.moviegetimdbtitle(&c[idx].Num2) {
			m.DbmovieID = c[idx].Num2
			return
		}
	}

	// Search in movie titles cache
	d := GetCachedTwoStringArr(logger.CacheTitlesMovie, false, true)
	for idx := range d {
		if m.matchesTitle(d[idx].Str1, d[idx].Str2) && m.moviegetimdbtitle(&d[idx].Num) {
			m.DbmovieID = d[idx].Num
			return
		}
	}

	m.DbmovieID = 0
}

// findMovieInDB searches for a movie ID in the database by checking the main movies table and alternate titles table.
// It uses the provided slugged parameter to determine how to match the title.
// If a movie ID is found and its IMDB title can be retrieved, it sets the DbmovieID.
// If no matching movie is found, it sets DbmovieID to 0.
func (m *ParseInfo) findMovieInDB(slugged bool) {
	// Try main movies table
	Scanrowsdyn(false, GetSluggedMap(slugged, "dbmovies"), &m.DbmovieID, &m.TempTitle)

	if m.DbmovieID != 0 && m.moviegetimdbtitle(&m.DbmovieID) {
		return
	}

	// Try alternate titles
	Scanrowsdyn(false, GetSluggedMap(slugged, "dbmoviesalt"), &m.DbmovieID, &m.TempTitle)

	if m.DbmovieID != 0 && m.moviegetimdbtitle(&m.DbmovieID) {
		return
	}

	m.DbmovieID = 0
}

// matchesTitle checks if temp title matches either string (case-insensitive).
func (m *ParseInfo) matchesTitle(str1, str2 string) bool {
	return strings.EqualFold(m.TempTitle, str1) || strings.EqualFold(m.TempTitle, str2)
}

// Parseresolution determines the video resolution based on the height and width of the media.
// It returns a string representation of the resolution (e.g., "4k", "1080p", "720p").
// If the resolution cannot be determined, it returns "Unknown Resolution".
func (m *ParseInfo) Parseresolution() string {
	height := m.Height

	width := m.Width
	if m.Height > m.Width {
		height = m.Width
		width = m.Height
	}

	aspectRatio := float64(width) / float64(height)

	// Ultra-wide content (aspect > 2.5, e.g. 32:9 or cinema scope).
	// Width determines the resolution class because these formats have
	// disproportionately wide pixels relative to their height.
	if aspectRatio > 2.5 {
		switch {
		case width >= 7680:
			return "4320p"
		case width >= 5120:
			return "2880p"
		case width >= 3840:
			return "2160p"
		case width >= 2560:
			return "1440p"
		case width >= 1920:
			return "1080p"
		case width >= 1280:
			return "720p"
		case width >= 720:
			return "480p"
		default:
			return "SD"
		}
	}

	// Standard and widescreen content (aspect ≤ 2.5): use height.
	// A video missing 1 pixel from a standard (e.g. 1079 instead of 1080)
	// is classified at the lower standard; 1081 stays at 1080p since it is
	// still closer to 1080 than to 1440 (midpoint 1260).
	switch {
	case height >= 4320:
		return "4320p"
	case height >= 2880:
		return "2880p"
	case height >= 2160:
		return "2160p"
	case height >= 1440:
		return "1440p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 576:
		return "576p"
	case height >= 480:
		return "480p"
	case height >= 360:
		return "360p"
	case height >= 240:
		return "240p"
	default:
		return "SD"
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

	m.Imdb = logger.AddImdbPrefix(m.Imdb)
	if config.GetSettingsGeneral().UseMediaCache {
		m.DbmovieID = CacheThreeStringIntIndexFuncFast(logger.CacheDBMovie, &m.Imdb)
		return
	}

	Scanrowsdyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &m.Imdb)
}

// Getepisodestoimport retrieves a slice of DbstaticTwoUint values representing the episode IDs to import for the given series ID and database series ID.
// If the episode array is empty, it returns an ErrNotFoundEpisode error.
// If there is only one episode and the SerieEpisodeID and DbserieEpisodeID are set, it returns a single-element slice with those values.
// Otherwise, it populates the episode IDs into the returned slice.
func (m *ParseInfo) Getepisodestoimport() error {
	if Getdatarow[string](false, QueryDbseriesGetIdentifiedByID, &m.DbserieID) == logger.StrDate {
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

	splitby := m.determineSplitChar(str1)
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
		m.Episode, err = strconv.Atoi(
			logger.TrimLeft(logger.Trim(episodeArray[idx], '-', '.', ' ', '_', 'E', 'X'), '0'),
		)
		if err != nil {
			m.Episode = 0
			return logger.ErrNotFoundEpisode
		}

		m.SetDBEpisodeIDfromM()

		if m.DbserieEpisodeID == 0 {
			continue
		}

		m.SetEpisodeIDfromM()

		if m.SerieEpisodeID == 0 {
			continue
		}

		if idx == 0 {
			m.Episodes = []DbstaticTwoUint{
				{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID},
			}
		} else {
			m.Episodes = append(
				m.Episodes,
				DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID},
			)
		}
	}

	return nil
}

// determineSplitChar identifies the character used to separate episode numbers in a string.
// It checks for common episode separators like 'E', 'e', 'X', 'x', or a dash.
// Returns the first matching separator character, or an empty string if no separator is found.
func (m *ParseInfo) determineSplitChar(str1 string) string {
	for _, char := range []string{"E", "e", "X", "x", logger.StrDash} {
		if strings.ContainsRune(str1, rune(char[0])) {
			return char
		}
	}

	return ""
}

// Checktitle checks if the given wanted title and year match the parsed title and year
// from the media file. It compares the wanted title against any alternate titles for the
// media entry from the database. Returns true if the title is unwanted and should be skipped.
func (m *ParseInfo) Checktitle(
	cfgp *config.MediaTypeConfig,
	qualcfg *config.QualityConfig,
	title string,
) bool {
	if qualcfg == nil {
		logger.Logtype("debug", 0).
			Msg("qualcfg empty")
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

	id = GetDBIDofType(cfgp.IsType, m)

	GetdatarowArgs(
		mtstrings.GetStringsMap(cfgp.IsType, logger.DBMediaTitlesID),
		&id,
		&year,
		&wantedTitle,
		&wantedslug,
	)

	if wantedTitle == "" {
		logger.Logtype("debug", 0).
			Msg("wanttitle empty")
		return true
	}

	if qualcfg.Name != "" {
		m.StripTitlePrefixPostfixGetQual(qualcfg)
	}

	if m.Title == "" {
		logger.Logtype("debug", 0).
			Msg("m Title empty")
		return true
	}

	if m.Year != 0 && year != 0 {
		if (m.Year != year && !qualcfg.CheckYear1) ||
			(qualcfg.CheckYear1 && (m.Year != year && m.Year != year+1 && m.Year != year-1)) {
			logger.Logtype("debug", 4).
				Uint16(logger.StrFound, m.Year).
				Uint16(logger.StrWanted, year).
				Msg("year different")

			return true
		}
	}

	if wantedTitle != "" {
		if qualcfg.CheckTitle &&
			ChecknzbtitleB(wantedTitle, wantedslug, title, qualcfg.CheckYear1, m.Year) {
			return false
		}
	}

	if !qualcfg.CheckTitle {
		logger.Logtype("debug", 1).
			Str(logger.StrTitle, m.Title).
			Msg("no alternate title check allowed") // , "checked", arr - better use string array
		return true
	}

	if config.GetSettingsGeneral().UseMediaCache {
		return m.checkalternatetitles(GetCachedTwoStringArr(
			mtstrings.GetStringsMap(cfgp.IsType, logger.CacheMediaTitles),
			false,
			true,
		), id, qualcfg, title)
	}

	return m.checkalternatetitles(
		Getentryalternatetitlesdirect(&id, cfgp.IsType),
		id,
		qualcfg,
		title,
	)
}

func GetDBIDofType(isType uint, m *ParseInfo) uint {
	switch isType {
	case config.MediaTypeMovie:
		return m.DbmovieID
	case config.MediaTypeSeries:
		return m.DbserieID
	case config.MediaTypeBook:
		return m.DbbookID
	case config.MediaTypeAudiobook:
		return m.DbaudiobookID
	case config.MediaTypeMusic:
		return m.DbalbumID
	}

	return 0
}

// checkalternatetitles checks if the given title matches any alternate titles for a specific media item.
// It takes an array of alternate titles, an ID, quality configuration, and the title to check.
// Returns true if no matching alternate title is found, false otherwise.
func (m *ParseInfo) checkalternatetitles(
	arr []syncops.DbstaticTwoStringOneInt,
	id uint,
	qualcfg *config.QualityConfig,
	title string,
) bool {
	if len(arr) == 0 {
		logger.Logtype("debug", 1).
			Str(logger.StrTitle, m.Title).
			Msg("no alternate titles found") // , "checked", arr - better use string array
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

	logger.Logtype("debug", 3).
		Str(logger.StrTitle, m.Title).
		Uint16("Year", m.Year).
		Interface("Titles", GetDbstaticTwoStringOneInt(arr, id)).
		Msg("no alternate title match found")

	return true
}

// AddUnmatched adds an unmatched file to the database. If the file is already in the cache, it returns without adding it. Otherwise, it inserts a new record into the appropriate table (movie_file_unmatcheds or serie_file_unmatcheds) with the file path, list name, and parsed data.
func (m *ParseInfo) AddUnmatched(cfgp *config.MediaTypeConfig, listname *string, err error) {
	// logger.Logtype("info", 1).Str("File", m.File).Msg("Pre Add Unmatched")
	if config.GetSettingsGeneral().UseFileCache {
		if slices.Contains(
			GetCachedStringArr(
				mtstrings.GetStringsMap(cfgp.IsType, logger.CacheUnmatched),
				false,
				true,
			),
			m.TempTitle,
		) {
			return
		}
	}

	// logger.Logtype("info", 1).Str("File", m.File).Msg("Pre Set Unmatched")
	m.ExecParsed(cfgp, err, listname)
	// logger.Logtype("info", 1).Str("File", m.File).Msg("Post Add Unmatched")
}

// ExecParsed adds an unmatched file to the database or updates an existing unmatched file record. It constructs a string representation of the parsed file information and inserts a new record or updates an existing record in the appropriate table (movie_file_unmatcheds or serie_file_unmatcheds).
func (m *ParseInfo) ExecParsed(cfgp *config.MediaTypeConfig, err error, listname *string) {
	id := Getdatarow[uint](
		false,
		mtstrings.GetStringsMap(cfgp.IsType, logger.DBIDUnmatchedPathList),
		&m.TempTitle,
		listname,
	) // testing

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
		if config.GetSettingsGeneral().UseFileCache {
			AppendCacheMap(cfgp.IsType, logger.CacheUnmatched, m.TempTitle)
		}

		ExecN(mtstrings.GetStringsMap(cfgp.IsType, "InsertUnmatched"), &str, listname, &m.TempTitle)
	} else {
		ExecN(mtstrings.GetStringsMap(cfgp.IsType, "UpdateUnmatched"), &str, &id)
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

	if config.GetSettingsGeneral().UseMediaCache {
		// Try indexed lookup first (O(1) if enabled)
		if config.GetSettingsGeneral().UseIndexedCache {
			if id := FindSeriesIDByTitleFast(m.TempTitle); id != 0 {
				m.DbserieID = id
				return
			}
		}

		// Fallback to linear search if indexed lookup not enabled or not found
		for a := range GetCachedThreeStringSeq(logger.CacheDBSeries, false, true) {
			if strings.EqualFold(a.Str1, m.TempTitle) ||
				strings.EqualFold(a.Str2, m.TempTitle) {
				m.DbserieID = a.Num2
				return
			}
		}

		for b := range GetCachedTwoStringSeq(logger.CacheDBSeriesAlt, false, true) {
			if strings.EqualFold(b.Str1, m.TempTitle) ||
				strings.EqualFold(b.Str2, m.TempTitle) {
				m.DbserieID = b.Num
				return
			}
		}

		m.DbserieID = 0

		return
	}

	if m.DbserieID != 0 {
		return
	}

	Scanrowsdyn(false, GetSluggedMap(slugged, "dbseries"), &m.DbserieID, &m.TempTitle)

	if m.DbserieID != 0 {
		return
	}

	Scanrowsdyn(false, GetSluggedMap(slugged, "dbseriesalt"), &m.DbserieID, &m.TempTitle)
}

// RegexGetMatchesStr1 extracts the series name from the filename
// by using a regular expression match. It looks for the series name substring
// in the filename, trims extra characters, and calls findDbserieByName
// to look up the series ID.
func (m *ParseInfo) RegexGetMatchesStr1(cfgp *config.MediaTypeConfig) {
	matchfor := filepath.Base(m.File)

	runrgx := strRegexSeriesTitle
	switch cfgp.IsType {
	case config.MediaTypeSeries:
		{
			if m.Date != "" {
				runrgx = strRegexSeriesTitleDate
			}
		}
	}

	matches := RunRetRegex(runrgx, matchfor, false)

	if len(matches) == 0 {
		switch cfgp.IsType {
		case config.MediaTypeSeries:
			{
				if m.Date != "" {
					matches = RunRetRegex(strRegexSeriesTitle, matchfor, false)
				}
			}
		}
	}

	if len(matches) == 0 || len(matches) < 4 || matches[3] == -1 {
		return
	}

	titleStr := matchfor[matches[2]:matches[3]]

	var title string
	if strings.ContainsRune(titleStr, '.') {
		title = logger.TrimRight(
			logger.StringReplaceWith(titleStr, '.', ' '),
			'-',
			'.',
			' ',
		)
	} else {
		title = logger.TrimRight(titleStr, '-', '.', ' ')
	}

	if title != m.Title {
		m.FindDbserieByNameWithSlug(title)
	}
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

// FindDbbookByAuthorFirst implements author-first lookup for books.
// Similar to FindDbalbumByArtistFirst, it tries to find an author in the database
// first, then searches for books by that author.
func (m *ParseInfo) FindDbbookByAuthorFirst() {
	// Use the original raw title (m.File) before parsing modified it
	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbbookID = 0
		return
	}

	// Clean up common NZB formatting from raw title
	rawTitle = cleanRawNZBTitle(rawTitle)

	authorCache := make(map[string][]resolvedAuthor)

	// Try " - " first (standard format: "Author - Title")
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialAuthor := strings.TrimSpace(before)

		potentialTitle := strings.TrimSpace(after)
		if m.tryFindAuthorAndBook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format: "Author-Title-Quality-Year-Group")
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialAuthor := strings.TrimSpace(before)
		potentialTitle := strings.TrimSpace(after)

		potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
		potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")
		// Clean up scene group from title
		potentialTitle = cleanSceneGroupFromAlbum(potentialTitle)
		if m.tryFindAuthorAndBook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
		) {
			return
		}
	}

	// Try comma separator (some formats use "Author, Title")
	if before, after, ok := strings.Cut(rawTitle, ","); ok {
		potentialAuthor := strings.TrimSpace(before)

		potentialTitle := strings.TrimSpace(after)
		if m.tryFindAuthorAndBook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
		) {
			return
		}
	}

	// If no split worked, try treating first two words as author (common for "FirstName LastName Title")
	words := strings.Fields(rawTitle)
	if len(words) >= 3 {
		// Try "FirstName LastName" as author, rest as title
		potentialAuthor := logger.JoinStrings(words[0], " ", words[1])

		potentialTitle := logger.JoinStringsSep(words[2:], " ")
		if m.tryFindAuthorAndBook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
		) {
			return
		}
	}

	// Try "Various Authors" for anthologies
	if m.tryFindVariousAuthorsBook(rawTitle) {
		return
	}

	m.DbbookID = 0
}

// resolvedAuthor pairs a single resolved author name with its matching dbauthor IDs.
type resolvedAuthor struct {
	name string
	ids  []uint
}

// resolvedArtist pairs a single resolved artist name with its matching dbartist IDs.
type resolvedArtist struct {
	name string
	ids  []uint
}

// resolveArtistNamesForAlbum splits, expands VA forms, slugs, and resolves dbartist IDs
// cachedResolveArtist wraps resolveArtistNamesForAlbum with a per-call cache so that
// repeated lookups for the same name within one search do not hit the DB twice.
func cachedResolveArtist(cache map[string][]resolvedArtist, name string) []resolvedArtist {
	if r, ok := cache[name]; ok {
		return r
	}

	r := resolveArtistNamesForAlbum(name)

	cache[name] = r

	return r
}

// cachedResolveAuthor wraps resolveAuthorNames with a per-call cache.
func cachedResolveAuthor(cache map[string][]resolvedAuthor, name string) []resolvedAuthor {
	if r, ok := cache[name]; ok {
		return r
	}

	r := resolveAuthorNames(name)

	cache[name] = r

	return r
}

// for a (possibly multi-artist) name. Shared by tryFindArtistAndAlbum and
// tryFindArtistAndAlbumFromWantedList so that the artist DB lookups are not repeated
// when they fall back to their respective Stripped variants with the same artistName.
func resolveArtistNamesForAlbum(artistName string) []resolvedArtist {
	single, names := splitMultiArtist(artistName)
	if single != "" {
		names = []string{single}
	}

	names = expandVANames(names)

	out := make([]resolvedArtist, 0, len(names))

	var (
		slug          string
		withPeriods   string
		ids, aliasIDs []uint
	)

	for i := range names {
		slug = logger.StringToSlug(names[i])
		withPeriods = addPeriodsToInitials(names[i])
		ids = Getrowssize[uint](
			false,
			"select count() from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			"select id from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			&names[i],
			&names[i],
			&slug,
			&withPeriods,
		)
		aliasIDs = Getrowssize[uint](
			false,
			"select count() from dbartist_aliases where alias = ? COLLATE NOCASE or slug = ?",
			"select dbartist_id from dbartist_aliases where alias = ? COLLATE NOCASE or slug = ?",
			&names[i], &slug)

		ids = append(ids, aliasIDs...)
		if len(ids) > 0 {
			out = append(out, resolvedArtist{name: names[i], ids: ids})
		}
	}

	return out
}

// resolveAuthorNames splits a (possibly multi-author) string, computes slug and
// period-initial variant for each part, queries dbauthors, and returns only the
// parts that were actually found in the database.
// Shared by tryFindAuthorAndBook, tryFindAuthorAndBookFromWantedList,
// tryFindAuthorAndAudiobook, and tryFindAuthorAndAudiobookFromWantedList so that
// StringToSlug + addPeriodsToInitials + GetrowsN are not repeated per function.
func resolveAuthorNames(authorName string) []resolvedAuthor {
	single, parts := splitMultiArtist(authorName)

	if single != "" {
		slug := logger.StringToSlug(single)
		withPeriods := addPeriodsToInitials(single)

		ids := Getrowssize[uint](
			false,
			"select count() from dbauthors where name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			"select id from dbauthors where name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			&single,
			&slug,
			&withPeriods,
		)
		if len(ids) > 0 {
			return []resolvedAuthor{{name: single, ids: ids}}
		}

		return nil
	}

	out := make([]resolvedAuthor, 0, len(parts))

	var (
		slug, withPeriods string
		ids               []uint
	)

	for i := range parts {
		slug = logger.StringToSlug(parts[i])
		withPeriods = addPeriodsToInitials(parts[i])

		ids = Getrowssize[uint](
			false,
			"select count() from dbauthors where name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			"select id from dbauthors where name = ? COLLATE NOCASE or slug = ? or name = ? COLLATE NOCASE",
			&parts[i],
			&slug,
			&withPeriods,
		)
		if len(ids) > 0 {
			out = append(out, resolvedAuthor{name: parts[i], ids: ids})
		}
	}

	return out
}

// tryFindVariousAuthorsBook attempts to find a book with "Various Authors" or similar.
// This is a fallback for anthology collections where no author is in the title.
func (m *ParseInfo) tryFindVariousAuthorsBook(bookTitle string) bool {
	if bookTitle == "" {
		return false
	}

	sluggedTitle := logger.StringToSlug(bookTitle)

	for i := range variousAuthorNames {
		var authorID uint

		// Try to find this author variation
		Scanrowsdyn(
			false,
			"select id from dbauthors where name = ? COLLATE NOCASE or slug = ?",
			&authorID,
			&variousAuthorNames[i],
			&variousAuthorSlugs[i],
		)

		if authorID == 0 {
			continue
		}

		// Try to find book by this author
		Scanrowsdyn(false,
			`SELECT b.id FROM dbbooks b
			 JOIN dbbook_authors ba ON b.id = ba.dbbook_id
			 WHERE ba.dbauthor_id = ?
			 AND (b.title = ? COLLATE NOCASE OR b.slug = ?)
			 LIMIT 1`,
			&m.DbbookID, &authorID, &bookTitle, &sluggedTitle)

		if m.DbbookID != 0 {
			m.Artist = variousAuthorNames[i]
			return true
		}

		// Try partial match
		Scanrowsdyn(false,
			`SELECT b.id FROM dbbooks b
			 JOIN dbbook_authors ba ON b.id = ba.dbbook_id
			 WHERE ba.dbauthor_id = ?
			 AND (b.title LIKE ? COLLATE NOCASE OR b.slug LIKE ?)
			 LIMIT 1`,
			&m.DbbookID, &authorID, "%"+bookTitle+"%", "%"+sluggedTitle+"%")

		if m.DbbookID != 0 {
			m.Artist = variousAuthorNames[i]
			return true
		}
	}

	return false
}

// tryFindAuthorAndBook attempts to find an author in the database and then find their book.
// Also handles multi-author names by splitting and trying each author individually.
func (m *ParseInfo) tryFindAuthorAndBook(resolved []resolvedAuthor, bookTitle string) bool {
	if len(resolved) == 0 || bookTitle == "" {
		return false
	}

	sluggedTitle := logger.StringToSlug(bookTitle)
	bookWords := strings.Fields(bookTitle)

	for _, ra := range resolved {
		for _, authorID := range ra.ids {
			// Try main books table with author filter
			Scanrowsdyn(false,
				`SELECT b.id FROM dbbooks b
				 JOIN dbbook_authors ba ON b.id = ba.dbbook_id
				 WHERE ba.dbauthor_id = ?
				 AND (b.title = ? COLLATE NOCASE OR b.slug = ?)
				 LIMIT 1`,
				&m.DbbookID, &authorID, &bookTitle, &sluggedTitle)

			if m.DbbookID != 0 {
				m.Artist = ra.name
				return true
			}

			// Try alternate titles
			Scanrowsdyn(false,
				`SELECT bt.dbbook_id FROM dbbook_titles bt
				 JOIN dbbook_authors ba ON bt.dbbook_id = ba.dbbook_id
				 WHERE ba.dbauthor_id = ?
				 AND (bt.title = ? COLLATE NOCASE OR bt.slug = ?)
				 LIMIT 1`,
				&m.DbbookID, &authorID, &bookTitle, &sluggedTitle)

			if m.DbbookID != 0 {
				m.Artist = ra.name
				return true
			}

			// Try word-skipping: remove one word at a time from the end
			for wordsToKeep := len(bookWords) - 1; wordsToKeep >= 2; wordsToKeep-- {
				shorterTitle := logger.JoinStringsSep(bookWords[:wordsToKeep], " ")
				shorterSlug := logger.StringToSlug(shorterTitle)

				Scanrowsdyn(false,
					`SELECT b.id FROM dbbooks b
					 JOIN dbbook_authors ba ON b.id = ba.dbbook_id
					 WHERE ba.dbauthor_id = ?
					 AND (b.title = ? COLLATE NOCASE OR b.slug = ?)
					 LIMIT 1`,
					&m.DbbookID, &authorID, &shorterTitle, &shorterSlug)

				if m.DbbookID != 0 {
					m.Artist = ra.name
					return true
				}

				// Also try alternate titles with shorter version
				Scanrowsdyn(false,
					`SELECT bt.dbbook_id FROM dbbook_titles bt
					 JOIN dbbook_authors ba ON bt.dbbook_id = ba.dbbook_id
					 WHERE ba.dbauthor_id = ?
					 AND (bt.title = ? COLLATE NOCASE OR bt.slug = ?)
					 LIMIT 1`,
					&m.DbbookID, &authorID, &shorterTitle, &shorterSlug)

				if m.DbbookID != 0 {
					m.Artist = ra.name
					return true
				}
			}
		}
	}

	return false
}

// FindDbbookByAuthorFirstFromWantedList searches for a book by author name,
// prioritizing dbbooks that are already in the user's wanted list (books table).
func (m *ParseInfo) FindDbbookByAuthorFirstFromWantedList(listnames []string) {
	if len(listnames) == 0 {
		m.DbbookID = 0
		return
	}

	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbbookID = 0
		return
	}

	rawTitle = cleanRawNZBTitle(rawTitle)

	authorCache := make(map[string][]resolvedAuthor)

	// Try " - " first (standard format: "Author - Title")
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialAuthor := strings.TrimSpace(before)

		potentialTitle := strings.TrimSpace(after)
		if m.tryFindAuthorAndBookFromWantedList(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
			listnames,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format)
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialAuthor := strings.TrimSpace(before)
		potentialTitle := strings.TrimSpace(after)

		potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
		potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")

		potentialTitle = cleanSceneGroupFromAlbum(potentialTitle)
		if m.tryFindAuthorAndBookFromWantedList(
			cachedResolveAuthor(authorCache, potentialAuthor),
			potentialTitle,
			listnames,
		) {
			return
		}
	}

	m.DbbookID = 0
}

// tryFindAuthorAndBookFromWantedList attempts to find a book by author name,
// prioritizing dbbooks that are in the user's wanted list.
func (m *ParseInfo) tryFindAuthorAndBookFromWantedList(
	resolved []resolvedAuthor,
	bookTitle string,
	listnames []string,
) bool {
	if len(resolved) == 0 || bookTitle == "" || len(listnames) == 0 {
		return false
	}

	sluggedTitle := logger.StringToSlug(bookTitle)
	bookWords := strings.Fields(bookTitle)

	for _, ra := range resolved {
		for _, authorID := range ra.ids {
			for j := range listnames {
				// Try exact match in wanted list
				Scanrowsdyn(false,
					`SELECT bk.dbbook_id FROM books bk
					 JOIN dbbook_authors ba ON bk.dbbook_id = ba.dbbook_id
					 JOIN dbbooks db ON db.id = bk.dbbook_id
					 WHERE ba.dbauthor_id = ?
					 AND bk.listname = ? COLLATE NOCASE
					 AND (db.title = ? COLLATE NOCASE OR db.slug = ?)
					 LIMIT 1`,
					&m.DbbookID, &authorID, &listnames[j], &bookTitle, &sluggedTitle)

				if m.DbbookID != 0 {
					m.Artist = ra.name
					m.BookID = m.getBookIDByDbbookAndList(listnames[j])
					return true
				}

				// Try word-skipping for wanted list
				for wordsToKeep := len(bookWords) - 1; wordsToKeep >= 2; wordsToKeep-- {
					shorterTitle := logger.JoinStringsSep(bookWords[:wordsToKeep], " ")
					shorterSlug := logger.StringToSlug(shorterTitle)

					Scanrowsdyn(false,
						`SELECT bk.dbbook_id FROM books bk
						 JOIN dbbook_authors ba ON bk.dbbook_id = ba.dbbook_id
						 JOIN dbbooks db ON db.id = bk.dbbook_id
						 WHERE ba.dbauthor_id = ?
						 AND bk.listname = ? COLLATE NOCASE
						 AND (db.title = ? COLLATE NOCASE OR db.slug = ?)
						 LIMIT 1`,
						&m.DbbookID, &authorID, &listnames[j], &shorterTitle, &shorterSlug)

					if m.DbbookID == 0 {
						continue
					}

					m.Artist = ra.name
					m.BookID = m.getBookIDByDbbookAndList(listnames[j])

					return true
				}
			}
		}
	}

	return false
}

// getBookIDByDbbookAndList retrieves the book ID from the books table.
func (m *ParseInfo) getBookIDByDbbookAndList(listname string) uint {
	var bookID uint
	Scanrowsdyn(false,
		"SELECT id FROM books WHERE dbbook_id = ? AND listname = ? COLLATE NOCASE",
		&bookID, &m.DbbookID, &listname)

	return bookID
}

// FindDbbookByTitle searches for a book by title in the database or cache.
// It looks for matches in both the main dbbooks table and dbbook_titles table.
// If Artist field is populated, it also filters by author name.
func (m *ParseInfo) FindDbbookByTitle() {
	if m.Title == "" {
		m.DbbookID = 0
		return
	}

	sluggedTitle := logger.StringToSlug(m.Title)

	// If artist (author) is provided, search by both title and author
	if m.Artist != "" {
		if config.GetSettingsGeneral().UseMediaCache {
			// Search in main book cache with author filtering
			for a := range GetCachedThreeStringSeq(logger.CacheDBBook, false, true) {
				if !strings.EqualFold(a.Str1, m.Title) && !strings.EqualFold(a.Str2, sluggedTitle) {
					continue
				}

				// Verify author matches
				var authorMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbauthors
						WHERE id = (SELECT dbauthor_id FROM dbbooks WHERE id = ?)
						AND name = ? COLLATE NOCASE
					)`,
					&authorMatches, &a.Num2, &m.Artist)

				if authorMatches {
					m.DbbookID = a.Num2
					return
				}
			}

			// Search in book titles cache with author filtering
			for b := range GetCachedTwoStringSeq(logger.CacheTitlesBook, false, true) {
				if !strings.EqualFold(b.Str1, m.Title) && !strings.EqualFold(b.Str2, sluggedTitle) {
					continue
				}

				// Verify author matches
				var authorMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbauthors
						WHERE id = (SELECT dbauthor_id FROM dbbooks WHERE id = ?)
						AND name = ? COLLATE NOCASE
					)`,
					&authorMatches, &b.Num, &m.Artist)

				if authorMatches {
					m.DbbookID = b.Num
					return
				}
			}

			m.DbbookID = 0

			return
		}

		// Try main books table with author join
		Scanrowsdyn(false,
			`SELECT b.id FROM dbbooks b
			 JOIN dbauthors a ON b.dbauthor_id = a.id
			 WHERE (b.title = ? COLLATE NOCASE OR b.slug = ?)
			 AND a.name = ? COLLATE NOCASE
			 LIMIT 1`,
			&m.DbbookID, &m.Title, &sluggedTitle, &m.Artist)

		if m.DbbookID != 0 {
			return
		}

		// Try alternate titles with author join
		Scanrowsdyn(false,
			`SELECT bt.dbbook_id FROM dbbook_titles bt
			 JOIN dbbooks b ON bt.dbbook_id = b.id
			 JOIN dbauthors a ON b.dbauthor_id = a.id
			 WHERE (bt.title = ? COLLATE NOCASE OR bt.slug = ?)
			 AND a.name = ? COLLATE NOCASE
			 LIMIT 1`,
			&m.DbbookID, &m.Title, &sluggedTitle, &m.Artist)

		return
	}

	// Fallback to title-only search if no artist provided
	if config.GetSettingsGeneral().UseMediaCache {
		// Search in main book cache
		for a := range GetCachedThreeStringSeq(logger.CacheDBBook, false, true) {
			if strings.EqualFold(a.Str1, m.Title) || strings.EqualFold(a.Str2, sluggedTitle) {
				m.DbbookID = a.Num2
				return
			}
		}

		// Search in book titles cache
		for b := range GetCachedTwoStringSeq(logger.CacheTitlesBook, false, true) {
			if strings.EqualFold(b.Str1, m.Title) || strings.EqualFold(b.Str2, sluggedTitle) {
				m.DbbookID = b.Num
				return
			}
		}

		m.DbbookID = 0

		return
	}

	// Try main books table
	Scanrowsdyn(
		false,
		"select id from dbbooks where title = ? COLLATE NOCASE or slug = ?",
		&m.DbbookID,
		&m.Title,
		&sluggedTitle,
	)

	if m.DbbookID != 0 {
		return
	}

	// Try alternate titles
	Scanrowsdyn(
		false,
		"select dbbook_id from dbbook_titles where title = ? COLLATE NOCASE or slug = ?",
		&m.DbbookID,
		&m.Title,
		&sluggedTitle,
	)
}

// FindDbaudiobookByTitle searches for an audiobook by title in the database or cache.
// It looks for matches in both the main dbaudiobooks table and dbaudiobook_titles table.
// If Artist field is populated, it also filters by author name.
func (m *ParseInfo) FindDbaudiobookByTitle() {
	if m.Title == "" {
		m.DbaudiobookID = 0
		return
	}

	sluggedTitle := logger.StringToSlug(m.Title)

	// If artist (author) is provided, search by both title and author
	if m.Artist != "" {
		if config.GetSettingsGeneral().UseMediaCache {
			// Search in main audiobook cache with author filtering
			for a := range GetCachedThreeStringSeq(logger.CacheDBAudiobook, false, true) {
				if !strings.EqualFold(a.Str1, m.Title) && !strings.EqualFold(a.Str2, sluggedTitle) {
					continue
				}

				// Verify author matches
				var authorMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbaudiobook_authors aa
						JOIN dbauthors a ON aa.dbauthor_id = a.id
						WHERE aa.dbaudiobook_id = ? AND a.name = ? COLLATE NOCASE
					)`,
					&authorMatches, &a.Num2, &m.Artist)

				if authorMatches {
					m.DbaudiobookID = a.Num2
					return
				}
			}

			// Search in audiobook titles cache with author filtering
			for b := range GetCachedTwoStringSeq(logger.CacheTitlesAudiobook, false, true) {
				if !strings.EqualFold(b.Str1, m.Title) && !strings.EqualFold(b.Str2, sluggedTitle) {
					continue
				}

				// Verify author matches
				var authorMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbaudiobook_authors aa
						JOIN dbauthors a ON aa.dbauthor_id = a.id
						WHERE aa.dbaudiobook_id = ? AND a.name = ? COLLATE NOCASE
					)`,
					&authorMatches, &b.Num, &m.Artist)

				if authorMatches {
					m.DbaudiobookID = b.Num
					return
				}
			}

			m.DbaudiobookID = 0

			return
		}

		// Try main audiobooks table with author join
		Scanrowsdyn(false,
			`SELECT ab.id FROM dbaudiobooks ab
			 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
			 JOIN dbauthors a ON aa.dbauthor_id = a.id
			 WHERE (ab.title = ? COLLATE NOCASE OR ab.slug = ?)
			 AND a.name = ? COLLATE NOCASE
			 LIMIT 1`,
			&m.DbaudiobookID, &m.Title, &sluggedTitle, &m.Artist)

		if m.DbaudiobookID != 0 {
			return
		}

		// Try alternate titles with author join
		Scanrowsdyn(false,
			`SELECT abt.dbaudiobook_id FROM dbaudiobook_titles abt
			 JOIN dbaudiobook_authors aa ON abt.dbaudiobook_id = aa.dbaudiobook_id
			 JOIN dbauthors a ON aa.dbauthor_id = a.id
			 WHERE (abt.title = ? COLLATE NOCASE OR abt.slug = ?)
			 AND a.name = ? COLLATE NOCASE
			 LIMIT 1`,
			&m.DbaudiobookID, &m.Title, &sluggedTitle, &m.Artist)

		return
	}

	// Fallback to title-only search if no artist provided
	if config.GetSettingsGeneral().UseMediaCache {
		// Search in main audiobook cache
		for a := range GetCachedThreeStringSeq(logger.CacheDBAudiobook, false, true) {
			if strings.EqualFold(a.Str1, m.Title) || strings.EqualFold(a.Str2, sluggedTitle) {
				m.DbaudiobookID = a.Num2
				return
			}
		}

		// Search in audiobook titles cache
		for b := range GetCachedTwoStringSeq(logger.CacheTitlesAudiobook, false, true) {
			if strings.EqualFold(b.Str1, m.Title) || strings.EqualFold(b.Str2, sluggedTitle) {
				m.DbaudiobookID = b.Num
				return
			}
		}

		m.DbaudiobookID = 0

		return
	}

	// Try main audiobooks table
	Scanrowsdyn(
		false,
		"select id from dbaudiobooks where title = ? COLLATE NOCASE or slug = ?",
		&m.DbaudiobookID,
		&m.Title,
		&sluggedTitle,
	)

	if m.DbaudiobookID != 0 {
		return
	}

	// Try alternate titles
	Scanrowsdyn(
		false,
		"select dbaudiobook_id from dbaudiobook_titles where title = ? COLLATE NOCASE or slug = ?",
		&m.DbaudiobookID,
		&m.Title,
		&sluggedTitle,
	)
}

// FindDbalbumByTitle searches for an album by title in the database or cache.
// It looks for matches in both the main dbalbums table and dbalbum_titles table.
func (m *ParseInfo) FindDbalbumByTitle() {
	if m.Title == "" {
		m.DbalbumID = 0
		return
	}

	sluggedTitle := logger.StringToSlug(m.Title)

	// If artist is provided, search by both artist and album
	if m.Artist != "" {
		if config.GetSettingsGeneral().UseMediaCache {
			// Search in main album cache with artist filtering
			for a := range GetCachedThreeStringSeq(logger.CacheDBAlbum, false, true) {
				if !strings.EqualFold(a.Str1, m.Title) && !strings.EqualFold(a.Str2, sluggedTitle) {
					continue
				}

				// Verify artist matches
				var artistMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbalbum_artists aa
						JOIN dbartists ar ON aa.dbartist_id = ar.id
						WHERE aa.dbalbum_id = ? AND (ar.name = ? COLLATE NOCASE OR ar.sort_name = ? COLLATE NOCASE)
					)`,
					&artistMatches, &a.Num2, &m.Artist, &m.Artist)

				if artistMatches {
					m.DbalbumID = a.Num2
					return
				}
			}

			// Search in album titles cache with artist filtering
			for b := range GetCachedTwoStringSeq(logger.CacheTitlesAlbum, false, true) {
				if !strings.EqualFold(b.Str1, m.Title) && !strings.EqualFold(b.Str2, sluggedTitle) {
					continue
				}

				// Verify artist matches
				var artistMatches bool
				Scanrowsdyn(false,
					`SELECT EXISTS(
						SELECT 1 FROM dbalbum_artists aa
						JOIN dbartists ar ON aa.dbartist_id = ar.id
						WHERE aa.dbalbum_id = ? AND (ar.name = ? COLLATE NOCASE OR ar.sort_name = ? COLLATE NOCASE)
					)`,
					&artistMatches, &b.Num, &m.Artist, &m.Artist)

				if artistMatches {
					m.DbalbumID = b.Num
					return
				}
			}

			m.DbalbumID = 0

			return
		}

		// Try main albums table with artist join
		Scanrowsdyn(false,
			`SELECT a.id FROM dbalbums a
			 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
			 JOIN dbartists ar ON aa.dbartist_id = ar.id
			 WHERE (a.title = ? COLLATE NOCASE OR a.slug = ?)
			 AND (ar.name = ? COLLATE NOCASE OR ar.sort_name = ? COLLATE NOCASE)
			 LIMIT 1`,
			&m.DbalbumID, &m.Title, &sluggedTitle, &m.Artist, &m.Artist)

		if m.DbalbumID != 0 {
			return
		}

		// Try alternate titles with artist join
		Scanrowsdyn(false,
			`SELECT at.dbalbum_id FROM dbalbum_titles at
			 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
			 JOIN dbartists ar ON aa.dbartist_id = ar.id
			 WHERE (at.title = ? COLLATE NOCASE OR at.slug = ?)
			 AND (ar.name = ? COLLATE NOCASE OR ar.sort_name = ? COLLATE NOCASE)
			 LIMIT 1`,
			&m.DbalbumID, &m.Title, &sluggedTitle, &m.Artist, &m.Artist)

		// Try artist aliases as well
		if m.DbalbumID == 0 {
			Scanrowsdyn(false,
				`SELECT a.id FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 JOIN dbartist_aliases aal ON aa.dbartist_id = aal.dbartist_id
				 WHERE (a.title = ? COLLATE NOCASE OR a.slug = ?)
				 AND aal.alias = ? COLLATE NOCASE
				 LIMIT 1`,
				&m.DbalbumID, &m.Title, &sluggedTitle, &m.Artist)
		}

		return
	}

	// Fallback to title-only search if no artist provided
	if config.GetSettingsGeneral().UseMediaCache {
		// Search in main album cache
		for a := range GetCachedThreeStringSeq(logger.CacheDBAlbum, false, true) {
			if strings.EqualFold(a.Str1, m.Title) || strings.EqualFold(a.Str2, sluggedTitle) {
				m.DbalbumID = a.Num2
				return
			}
		}

		// Search in album titles cache
		for b := range GetCachedTwoStringSeq(logger.CacheTitlesAlbum, false, true) {
			if strings.EqualFold(b.Str1, m.Title) || strings.EqualFold(b.Str2, sluggedTitle) {
				m.DbalbumID = b.Num
				return
			}
		}

		m.DbalbumID = 0

		return
	}

	// Try main albums table
	Scanrowsdyn(
		false,
		"select id from dbalbums where title = ? COLLATE NOCASE or slug = ?",
		&m.DbalbumID,
		&m.Title,
		&sluggedTitle,
	)

	if m.DbalbumID != 0 {
		return
	}

	// Try alternate titles
	Scanrowsdyn(
		false,
		"select dbalbum_id from dbalbum_titles where title = ? COLLATE NOCASE or slug = ?",
		&m.DbalbumID,
		&m.Title,
		&sluggedTitle,
	)
}

// decodeHTMLEntities decodes common HTML entities found in NZB titles.
func decodeHTMLEntities(s string) string {
	replacements := []struct {
		entity string
		char   string
	}{
		{"&amp;", "&"},
		{"&nbsp;", " "},
		{"&quot;", "\""},
		{"&apos;", "'"},
		{"&lt;", "<"},
		{"&gt;", ">"},
		{"&#39;", "'"},
		{"&#34;", "\""},
		{"&#x27;", "'"},
		{"&#x22;", "\""},
	}

	for i := range replacements {
		s = strings.ReplaceAll(s, replacements[i].entity, replacements[i].char)
	}

	// Handle common broken entities (missing semicolon or space after)
	s = strings.ReplaceAll(s, " amp; ", " & ")
	s = strings.ReplaceAll(s, " amp ", " & ")

	return s
}

// fixMojibake fixes common UTF-8 characters that were incorrectly decoded as Latin-1.
// This happens when UTF-8 bytes are interpreted as ISO-8859-1/Windows-1252.
func fixMojibake(s string) string {
	// Common mojibake patterns (UTF-8 → Latin-1 → displayed)
	replacements := []struct {
		broken  string
		correct string
	}{
		{"Ã©", "é"}, // é
		{"Ã¨", "è"}, // è
		{"Ã ", "à"}, // à
		{"Ã¢", "â"}, // â
		{"Ã§", "ç"}, // ç
		{"Ã´", "ô"}, // ô
		{"Ã»", "û"}, // û
		{"Ã¼", "ü"}, // ü
		{"Ã¶", "ö"}, // ö
		{"Ã¤", "ä"}, // ä
		{"Ã±", "ñ"}, // ñ
		{"Ã­", "í"}, // í
		{"Ã³", "ó"}, // ó
		{"Ãº", "ú"}, // ú
		{"Ã¡", "á"}, // á
		{"Ã¯", "ï"}, // ï
		{"Ã«", "ë"}, // ë
		{"Ã¿", "ÿ"}, // ÿ
	}

	for _, r := range replacements {
		s = strings.ReplaceAll(s, r.broken, r.correct)
	}

	return s
}

// cleanRawNZBTitle cleans up common NZB formatting from a raw title string.
// It removes quality indicators, format tags, year patterns, and scene tags
// to get a cleaner "Artist - Album" string for database lookup.
func cleanRawNZBTitle(title string) string {
	// Decode HTML entities (common in NZB titles)
	title = decodeHTMLEntities(title)

	// Fix common mojibake/encoding issues (UTF-8 displayed as Latin-1)
	title = fixMojibake(title)

	// Replace underscores and dots with spaces (scene format)
	if !strings.Contains(title, " ") {
		title = strings.ReplaceAll(title, "_", " ")
		title = strings.ReplaceAll(title, ".", " ")
	}

	// Remove common quality/format indicators at the end
	// These patterns are case-insensitive
	cleanPatterns := []string{
		// Audio formats
		"FLAC", "MP3", "AAC", "OGG", "OPUS", "WAV", "ALAC", "APE", "WMA", "M4A",
		// Quality indicators
		"320", "256", "192", "128", "V0", "V2", "24BIT", "16BIT", "24-BIT", "16-BIT",
		// Sources
		"WEB", "CD", "VINYL", "TAPE", "CABLE", "HDTV", "SAT",
		// Release types
		"OST", "EP", "LP", "SINGLE", "ALBUM", "LIVE", "BOOTLEG", "DEMO", "REMIX",
		"DELUXE", "REMASTERED", "REMASTER", "LIMITED", "EDITION",
		// Scene tags
		"PROPER", "REPACK", "READNFO", "INT",
	}

	upperTitle := strings.ToUpper(title)
	for _, pattern := range cleanPatterns {
		// Remove pattern surrounded by spaces, dashes, or at end
		for _, sep := range []string{" ", "-"} {
			// Pattern at end with separator before
			idx := strings.LastIndex(upperTitle, sep+pattern)
			if idx == -1 {
				continue
			}

			// Check if it's at the end or followed by separator/end
			endIdx := idx + len(sep) + len(pattern)
			if endIdx == len(title) ||
				(endIdx < len(title) && (title[endIdx] == ' ' || title[endIdx] == '-')) {
				title = title[:idx]
				upperTitle = strings.ToUpper(title)
			}
		}
	}

	// Remove year pattern at end like (2020) or [2020] or -2020
	if loc := globalCache.setRegexp(`[\s\-\[\(]*(19|20)\d{2}[\]\)]*\s*$`, 0).
		FindStringIndex(title); loc != nil {
		title = title[:loc[0]]
	}

	// Remove common audiobook/music markers
	if loc := globalCache.setRegexp(`(?i)\s*\[(?:audiobook|audio\s*book|ebook|e-book)\]\s*$`, 0).
		FindStringIndex(title); loc != nil {
		title = title[:loc[0]]
	}

	// Normalize "Vol.N" / "Vol N" / "Volume N" volume indicators (scene releases often use "Vol.58"
	// while the DB stores "58" as the album number). Replace with just the number.
	if loc := globalCache.setRegexp(`(?i)\bVol(?:ume)?\.?\s*(\d+)\b`, 0).
		FindStringSubmatchIndex(title); loc != nil {
		title = title[:loc[0]] + title[loc[2]:loc[3]] + title[loc[1]:]
	}

	// Note: Don't remove scene groups here - they're handled during artist/album splitting
	// The tryFindArtistAndAlbum function uses partial matching for album titles

	// Clean up multiple spaces and trim
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}

	return strings.TrimSpace(title)
}

// FindDbalbumByArtistFirst implements artist-first lookup for music albums.
// Instead of relying on perfect parsing, it tries to find an artist in the database
// first using different parts of the title string, then searches for albums by that artist.
// This approach is more robust for NZB titles that may not parse cleanly.
func (m *ParseInfo) FindDbalbumByArtistFirst() {
	// Use the original raw title (m.File) before parsing modified it
	// Fall back to m.Title if m.File is empty
	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbalbumID = 0
		return
	}

	// Clean up common NZB formatting from raw title
	rawTitle = cleanRawNZBTitle(rawTitle)

	artistCache := make(map[string][]resolvedArtist)

	// Try " - " first (standard format: "Artist - Album")
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialArtist := strings.TrimSpace(before)

		potentialAlbum := strings.TrimSpace(after)
		if m.tryFindArtistAndAlbum(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format: "Artist-Album-Quality-Year-Group")
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialArtist := strings.TrimSpace(before)
		potentialAlbum := strings.TrimSpace(after)
		// Replace dots with spaces for scene format
		potentialArtist = strings.ReplaceAll(potentialArtist, ".", " ")
		potentialAlbum = strings.ReplaceAll(potentialAlbum, ".", " ")
		// Clean up scene group from album (short alphanumeric string after last dash)
		potentialAlbum = cleanSceneGroupFromAlbum(potentialAlbum)
		if m.tryFindArtistAndAlbum(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
		) {
			return
		}
	}

	// Try comma separator (some formats use "Artist, Album")
	if before, after, ok := strings.Cut(rawTitle, ","); ok {
		potentialArtist := strings.TrimSpace(before)

		potentialAlbum := strings.TrimSpace(after)
		if m.tryFindArtistAndAlbum(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
		) {
			return
		}
	}

	// If no split worked, try treating first two words as artist
	words := strings.Fields(rawTitle)
	if len(words) >= 3 {
		// Try "FirstName LastName" as artist, rest as album
		potentialArtist := logger.JoinStrings(words[0], " ", words[1])

		potentialAlbum := logger.JoinStringsSep(words[2:], " ")
		if m.tryFindArtistAndAlbum(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
		) {
			return
		}
	}

	// If no artist found, try with "Various Artists" (for compilations/soundtracks)
	// Use the entire title as album name
	if m.tryFindVariousArtistsAlbum(&rawTitle) {
		return
	}

	m.DbalbumID = 0
}

// tryFindVariousArtistsAlbum attempts to find an album with "Various Artists" or similar as artist.
// This is a fallback for compilation albums, soundtracks, etc. where no artist is in the title.
func (m *ParseInfo) tryFindVariousArtistsAlbum(albumTitle *string) bool {
	if albumTitle == nil || *albumTitle == "" {
		return false
	}

	sluggedAlbumWild := logger.StringToSlugWild(*albumTitle)
	sluggedAlbum := sluggedAlbumWild[1 : len(sluggedAlbumWild)-1]
	albumTitleWild := logger.JoinStrings("%", *albumTitle, "%")

	var artistID uint
	for i := range variousArtistNames {
		artistID = 0
		// Try to find this artist variation
		Scanrowsdyn(
			false,
			"select id from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE or slug = ?",
			&artistID,
			&variousArtistNames[i],
			&variousArtistNames[i],
			&variousArtistSlugs[i],
		)

		if artistID == 0 {
			continue
		}

		// Try to find album by this artist
		Scanrowsdyn(false,
			`SELECT a.id FROM dbalbums a
			 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
			 WHERE aa.dbartist_id = ?
			 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
			 LIMIT 1`,
			&m.DbalbumID, &artistID, albumTitle, &sluggedAlbum)

		if m.DbalbumID != 0 {
			m.Artist = variousArtistNames[i]
			return true
		}

		// Try alternate album titles
		Scanrowsdyn(false,
			`SELECT at.dbalbum_id FROM dbalbum_titles at
			 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
			 WHERE aa.dbartist_id = ?
			 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
			 LIMIT 1`,
			&m.DbalbumID, &artistID, albumTitle, &sluggedAlbum)

		if m.DbalbumID != 0 {
			m.Artist = variousArtistNames[i]
			return true
		}

		// Try partial match
		Scanrowsdyn(false,
			`SELECT a.id FROM dbalbums a
			 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
			 WHERE aa.dbartist_id = ?
			 AND (a.title LIKE ? COLLATE NOCASE OR a.slug LIKE ?)
			 LIMIT 1`,
			&m.DbalbumID, &artistID, &albumTitleWild, &sluggedAlbumWild)

		if m.DbalbumID != 0 {
			m.Artist = variousArtistNames[i]
			return true
		}
	}

	return false
}

// splitMultiArtist splits a multi-artist string into individual artist names.
// Handles common separators like "&", " and ", "+", "feat.", "ft.", and double spaces.
// Returns (single, nil) when there is exactly one artist (no heap allocation),
// or ("", many) when multiple artists are found.
func splitMultiArtist(artistStr string) (string, []string) {
	// Normalize separators to a common delimiter
	normalized := artistStr

	// Fast path: skip replacement loop if no separator is present.
	hasSep := false
	for _, sep := range multiArtistSeparators {
		if logger.ContainsI(normalized, sep) {
			hasSep = true
			break
		}
	}

	if !hasSep {
		trimmed := strings.TrimSpace(artistStr)
		if len(trimmed) >= 2 {
			return trimmed, nil
		}

		return "", nil
	}

	// Search case-insensitively (lowerNormalized) but replace in original-case (normalized).
	// Both strings are kept in sync so lowerNormalized is only computed once.
	lowerNormalized := strings.ToLower(normalized)
	for _, sep := range multiArtistSeparators {
		// All separators are already lowercase, so no ToLower(sep) needed.
		for {
			idx := strings.Index(lowerNormalized, sep)
			if idx == -1 {
				break
			}

			normalized = logger.JoinStrings(normalized[:idx], "|", normalized[idx+len(sep):])
			lowerNormalized = logger.JoinStrings(
				lowerNormalized[:idx],
				"|",
				lowerNormalized[idx+len(sep):],
			)
		}
	}

	// Split by pipe and clean up
	var artists []string

	parts := strings.SplitSeq(normalized, "|")
	for part := range parts {
		part = strings.TrimSpace(part)
		if part != "" && len(part) >= 2 {
			artists = append(artists, part)
		}
	}

	if len(artists) == 0 {
		artists = append(artists, strings.TrimSpace(artistStr))
	}

	return "", artists
}

// expandVANames expands VA/V.A. to "Various Artists" and vice versa in an artist name list.
// This enables bidirectional matching between abbreviated and full forms.
func expandVANames(artists []string) []string {
	const fullForm = "Various Artists"

	expanded := make([]string, 0, len(artists)*2)
	for _, a := range artists {
		expanded = append(expanded, a)

		trimmed := strings.TrimSpace(a)
		for _, vf := range vaForms {
			if strings.EqualFold(trimmed, vf) {
				expanded = append(expanded, fullForm)
				break
			}
		}

		if strings.EqualFold(a, fullForm) {
			expanded = append(expanded, "VA")
		}
	}

	return expanded
}

// tryFindArtistAndAlbum attempts to find an artist in the database and then find their album.
// Returns true if both artist and album were found.
// Note: Gets ALL matching artists (case variations) and tries each one until album is found.
// Also handles multi-artist names by splitting and trying each artist individually.
func (m *ParseInfo) tryFindArtistAndAlbum(resolved []resolvedArtist, albumTitle *string) bool {
	if len(resolved) == 0 || albumTitle == nil || *albumTitle == "" {
		return false
	}

	sluggedAlbum := logger.StringToSlug(*albumTitle)
	doPrefixMatch := strings.Count(*albumTitle, " ") >= 2

	for resolveid := range resolved {
		for raid := range resolved[resolveid].ids {
			// Try main albums table with artist filter
			Scanrowsdyn(false,
				`SELECT a.id FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
				 LIMIT 1`,
				&m.DbalbumID, &resolved[resolveid].ids[raid], albumTitle, &sluggedAlbum)

			if m.DbalbumID != 0 {
				m.Artist = resolved[resolveid].name
				return true
			}

			// Try alternate album titles
			Scanrowsdyn(false,
				`SELECT at.dbalbum_id FROM dbalbum_titles at
				 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
				 LIMIT 1`,
				&m.DbalbumID, &resolved[resolveid].ids[raid], albumTitle, &sluggedAlbum)

			if m.DbalbumID != 0 {
				m.Artist = resolved[resolveid].name
				return true
			}

			// Try word-skipping: fetch all album titles for this artist once,
			// then match word-prefixes of albumTitle in Go (replaces O(Nx2) SQL queries).
			if doPrefixMatch {
				prefixCandidates := Getrowssize[DbstaticOneStringOneUInt](
					false,
					`SELECT count() FROM dbalbums a JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id WHERE aa.dbartist_id = ?`,
					`SELECT a.title, a.id FROM dbalbums a
					 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?`,
					&resolved[resolveid].ids[raid],
				)
				altPrefixCandidates := Getrowssize[DbstaticOneStringOneUInt](
					false,
					`SELECT count() FROM dbalbum_titles at JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id WHERE aa.dbartist_id = ?`,
					`SELECT at.title, at.dbalbum_id FROM dbalbum_titles at
					 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
					 WHERE aa.dbartist_id = ?`,
					&resolved[resolveid].ids[raid],
				)

				prefixCandidates = append(prefixCandidates, altPrefixCandidates...)
				for _, cand := range prefixCandidates {
					if strings.Count(cand.Str, " ") < 1 {
						continue
					}

					cs := logger.StringToSlug(cand.Str)

					titleMatch := logger.HasPrefixI(*albumTitle, cand.Str) &&
						(len(*albumTitle) == len(cand.Str) || (*albumTitle)[len(cand.Str)] == ' ')

					slugMatch := strings.HasPrefix(sluggedAlbum, cs) &&
						(len(sluggedAlbum) == len(cs) || sluggedAlbum[len(cs)] == '-')
					if titleMatch || slugMatch {
						m.DbalbumID = cand.Num
						m.Artist = resolved[resolveid].name
						return true
					}
				}
			}
		}
	}

	// Fallback with stripped title — reuse already-resolved artists, no repeated DB lookups.
	strippedAlbum := stripReleaseType(*albumTitle)
	if strippedAlbum != *albumTitle && strippedAlbum != "" {
		return m.tryFindArtistAndAlbumStripped(resolved, &strippedAlbum)
	}

	return false
}

// tryFindArtistAndAlbumStripped is like tryFindArtistAndAlbum but for stripped titles.
// It accepts pre-resolved artists so the DB lookups are not repeated.
func (m *ParseInfo) tryFindArtistAndAlbumStripped(
	resolved []resolvedArtist,
	albumTitle *string,
) bool {
	if len(resolved) == 0 || albumTitle == nil || *albumTitle == "" {
		return false
	}

	sluggedAlbum := logger.StringToSlug(*albumTitle)

	for _, ra := range resolved {
		for artistID := range ra.ids {
			Scanrowsdyn(false,
				`SELECT a.id FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
				 LIMIT 1`,
				&m.DbalbumID, &ra.ids[artistID], albumTitle, &sluggedAlbum)

			if m.DbalbumID != 0 {
				m.Artist = ra.name
				return true
			}

			Scanrowsdyn(false,
				`SELECT at.dbalbum_id FROM dbalbum_titles at
				 JOIN dbalbum_artists aa ON at.dbalbum_id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
				 LIMIT 1`,
				&m.DbalbumID, &ra.ids[artistID], albumTitle, &sluggedAlbum)

			if m.DbalbumID != 0 {
				m.Artist = ra.name
				return true
			}
		}
	}

	return false
}

// FindDbalbumByArtistFirstFromWantedList searches for an album by artist name,
// prioritizing dbalbums that are already in the user's wanted list (albums table).
// This ensures that when there are multiple releases of the same album, we find
// the one the user actually wants.
func (m *ParseInfo) FindDbalbumByArtistFirstFromWantedList(listnames []string) {
	if len(listnames) == 0 {
		m.DbalbumID = 0
		return
	}

	// Use the original raw title (m.File) before parsing modified it
	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbalbumID = 0
		return
	}

	// Clean up common NZB formatting from raw title
	rawTitle = cleanRawNZBTitle(rawTitle)

	artistCache := make(map[string][]resolvedArtist)

	// Try " - " first (standard format: "Artist - Album")
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialArtist := strings.TrimSpace(before)

		potentialAlbum := strings.TrimSpace(after)
		if m.tryFindArtistAndAlbumFromWantedList(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
			listnames,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format: "Artist-Album-Quality-Year-Group")
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialArtist := strings.TrimSpace(before)
		potentialAlbum := strings.TrimSpace(after)

		potentialArtist = strings.ReplaceAll(potentialArtist, ".", " ")
		potentialAlbum = strings.ReplaceAll(potentialAlbum, ".", " ")

		potentialAlbum = cleanSceneGroupFromAlbum(potentialAlbum)
		if m.tryFindArtistAndAlbumFromWantedList(
			cachedResolveArtist(artistCache, potentialArtist),
			&potentialAlbum,
			listnames,
		) {
			return
		}
	}

	m.DbalbumID = 0
}

// tryFindArtistAndAlbumFromWantedList attempts to find an album by artist name,
// prioritizing dbalbums that are in the user's wanted list.
func (m *ParseInfo) tryFindArtistAndAlbumFromWantedList(
	resolved []resolvedArtist,
	albumTitle *string,
	listnames []string,
) bool {
	if len(resolved) == 0 || albumTitle == nil || *albumTitle == "" || len(listnames) == 0 {
		return false
	}

	sluggedAlbum := logger.StringToSlug(*albumTitle)
	// Hoist constants out of the triple loop — albumTitle never changes.
	doPrefixMatch := strings.Count(*albumTitle, " ") >= 2

	for _, ra := range resolved {
		// Try each artist ID until we find an album in the wanted list.
		// Use index-based range so &ra.ids[k] points into the backing array already
		// on the heap, avoiding the range-copy variable escaping to heap.
		for k := range ra.ids {
			for j := range listnames {
				// Try exact match first
				Scanrowsdyn(false,
					`SELECT a.dbalbum_id FROM albums a
					 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
					 JOIN dbalbums db ON db.id = a.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND a.listname = ? COLLATE NOCASE
					 AND (db.title = ? COLLATE NOCASE OR db.slug = ?)
					 LIMIT 1`,
					&m.DbalbumID, &ra.ids[k], &listnames[j], albumTitle, &sluggedAlbum)

				if m.DbalbumID != 0 {
					m.Artist = ra.name
					m.AlbumID = m.getAlbumIDByDbalbumAndList(listnames[j])
					return true
				}

				// Try alternate album titles
				Scanrowsdyn(false,
					`SELECT a.dbalbum_id FROM albums a
					 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
					 JOIN dbalbum_titles at ON at.dbalbum_id = a.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND a.listname = ? COLLATE NOCASE
					 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
					 LIMIT 1`,
					&m.DbalbumID, &ra.ids[k], &listnames[j], albumTitle, &sluggedAlbum)

				if m.DbalbumID != 0 {
					m.Artist = ra.name
					m.AlbumID = m.getAlbumIDByDbalbumAndList(listnames[j])
					return true
				}

				// Try word-skipping approach: fetch all album titles for this artist+listname
				// once (2 queries), then match word-prefixes of albumTitle in Go.
				// Replaces O(N×2) SQL queries (one per word-prefix) with Go comparisons.
				if doPrefixMatch {
					prefixCandidates := Getrowssize[syncops.DbstaticTwoStringOneInt](
						false,
						`SELECT count() FROM albums a JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id JOIN dbalbums db ON db.id = a.dbalbum_id WHERE aa.dbartist_id = ? AND a.listname = ? COLLATE NOCASE`,
						`SELECT db.title, db.slug, db.id FROM albums a
						 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
						 JOIN dbalbums db ON db.id = a.dbalbum_id
						 WHERE aa.dbartist_id = ? AND a.listname = ? COLLATE NOCASE`,
						&ra.ids[k],
						&listnames[j],
					)
					altPrefixCandidates := Getrowssize[syncops.DbstaticTwoStringOneInt](
						false,
						`SELECT count() FROM albums a JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id JOIN dbalbum_titles at ON at.dbalbum_id = a.dbalbum_id WHERE aa.dbartist_id = ? AND a.listname = ? COLLATE NOCASE`,
						`SELECT at.title, at.slug, at.dbalbum_id FROM albums a
						 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
						 JOIN dbalbum_titles at ON at.dbalbum_id = a.dbalbum_id
						 WHERE aa.dbartist_id = ? AND a.listname = ? COLLATE NOCASE`,
						&ra.ids[k],
						&listnames[j],
					)

					prefixCandidates = append(prefixCandidates, altPrefixCandidates...)
					for _, cand := range prefixCandidates {
						if !strings.Contains(cand.Str1, " ") {
							continue // stored title is a single word, skip
						}

						cl := len(cand.Str1)
						titleMatch := len(*albumTitle) >= cl &&
							strings.EqualFold((*albumTitle)[:cl], cand.Str1) &&
							(len(*albumTitle) == cl || (*albumTitle)[cl] == ' ')

						slugMatch := strings.HasPrefix(sluggedAlbum, cand.Str2) &&
							(len(sluggedAlbum) == len(cand.Str2) || sluggedAlbum[len(cand.Str2)] == '-')
						if titleMatch || slugMatch {
							m.DbalbumID = cand.Num
							m.Artist = ra.name
							m.AlbumID = m.getAlbumIDByDbalbumAndList(listnames[j])
							return true
						}
					}
				}
			}
		}
	}

	// Fallback: try with release type stripped (e.g., "Deluxe Edition" removed)
	strippedAlbum := stripReleaseType(*albumTitle)
	if strippedAlbum != *albumTitle && strippedAlbum != "" {
		return m.tryFindArtistAndAlbumFromWantedListStripped(resolved, &strippedAlbum, listnames)
	}

	return false
}

// tryFindArtistAndAlbumFromWantedListStripped is like tryFindArtistAndAlbumFromWantedList but for stripped titles.
// It accepts pre-resolved artists so the DB lookups are not repeated.
func (m *ParseInfo) tryFindArtistAndAlbumFromWantedListStripped(
	resolved []resolvedArtist,
	albumTitle *string,
	listnames []string,
) bool {
	if len(resolved) == 0 || albumTitle == nil || *albumTitle == "" || len(listnames) == 0 {
		return false
	}

	sluggedAlbum := logger.StringToSlug(*albumTitle)

	for _, ra := range resolved {
		for raid := range ra.ids {
			for j := range listnames {
				Scanrowsdyn(false,
					`SELECT a.dbalbum_id FROM albums a
					 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
					 JOIN dbalbums db ON db.id = a.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND a.listname = ? COLLATE NOCASE
					 AND (db.title = ? COLLATE NOCASE OR db.slug = ?)
					 LIMIT 1`,
					&m.DbalbumID, ra.ids[raid], &listnames[j], albumTitle, &sluggedAlbum)

				if m.DbalbumID != 0 {
					m.Artist = ra.name
					m.AlbumID = m.getAlbumIDByDbalbumAndList(listnames[j])
					return true
				}

				Scanrowsdyn(false,
					`SELECT a.dbalbum_id FROM albums a
					 JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id
					 JOIN dbalbum_titles at ON at.dbalbum_id = a.dbalbum_id
					 WHERE aa.dbartist_id = ?
					 AND a.listname = ? COLLATE NOCASE
					 AND (at.title = ? COLLATE NOCASE OR at.slug = ?)
					 LIMIT 1`,
					&m.DbalbumID, ra.ids[raid], &listnames[j], albumTitle, &sluggedAlbum)

				if m.DbalbumID == 0 {
					continue
				}

				m.Artist = ra.name
				m.AlbumID = m.getAlbumIDByDbalbumAndList(listnames[j])

				return true
			}
		}
	}

	return false
}

// getAlbumIDByDbalbumAndList retrieves the album ID from the albums table for the current DbalbumID and listname.
func (m *ParseInfo) getAlbumIDByDbalbumAndList(listname string) uint {
	return ScanRowVal2[uint, string, uint](
		"SELECT id FROM albums WHERE dbalbum_id = ? AND listname = ? COLLATE NOCASE",
		m.DbalbumID, listname)
}

// FindDbaudiobookByAuthorFirst implements author-first lookup for audiobooks.
// Similar to FindDbalbumByArtistFirst, it tries to find an author in the database
// first, then searches for audiobooks by that author.
func (m *ParseInfo) FindDbaudiobookByAuthorFirst() {
	// Use the original raw title (m.File) before parsing modified it
	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbaudiobookID = 0
		return
	}

	// Clean up common NZB formatting from raw title
	rawTitle = cleanRawNZBTitle(rawTitle)

	authorCache := make(map[string][]resolvedAuthor)

	// Try " - " first (standard format: "Author - Title")
	var potentialAuthor, potentialTitle string
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialAuthor = strings.TrimSpace(before)

		potentialTitle = strings.TrimSpace(after)
		if m.tryFindAuthorAndAudiobook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format: "Author-Title-Quality-Year-Group")
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialAuthor = strings.TrimSpace(before)
		potentialTitle = strings.TrimSpace(after)

		potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
		potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")
		// Clean up scene group from title
		potentialTitle = cleanSceneGroupFromAlbum(potentialTitle)
		if m.tryFindAuthorAndAudiobook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
		) {
			return
		}
	}

	// Try comma separator (some formats use "Author, Title")
	if before, after, ok := strings.Cut(rawTitle, ","); ok {
		potentialAuthor = strings.TrimSpace(before)

		potentialTitle = strings.TrimSpace(after)
		if m.tryFindAuthorAndAudiobook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
		) {
			return
		}
	}

	// If no split worked, try treating first two words as author (common for "FirstName LastName Title")
	words := strings.Fields(rawTitle)
	if len(words) >= 3 {
		// Try "FirstName LastName" as author, rest as title
		potentialAuthor = logger.JoinStrings(words[0], " ", words[1])

		potentialTitle = logger.JoinStringsSep(words[2:], " ")
		if m.tryFindAuthorAndAudiobook(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
		) {
			return
		}
	}

	// Try "Various Authors" for anthologies
	if m.tryFindVariousAuthorsAudiobook(&rawTitle) {
		return
	}

	m.DbaudiobookID = 0
}

// tryFindVariousAuthorsAudiobook attempts to find an audiobook with "Various Authors" or similar.
// This is a fallback for anthology collections where no author is in the title.
func (m *ParseInfo) tryFindVariousAuthorsAudiobook(bookTitle *string) bool {
	if bookTitle == nil || *bookTitle == "" {
		return false
	}

	sluggedTitle := logger.StringToSlug(*bookTitle)

	var authorID uint
	for i := range variousAuthorNames {
		Scanrowsdyn(false,
			"select id from dbauthors where name = ? COLLATE NOCASE or slug = ?",
			&authorID, &variousAuthorNames[i], &variousAuthorSlugs[i],
		)

		if authorID == 0 {
			continue
		}

		// Try to find audiobook by this author
		Scanrowsdyn(false,
			`SELECT ab.id FROM dbaudiobooks ab
			 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
			 WHERE aa.dbauthor_id = ?
			 AND (ab.title = ? COLLATE NOCASE OR ab.slug = ?)
			 LIMIT 1`,
			&m.DbaudiobookID, &authorID, bookTitle, &sluggedTitle)

		if m.DbaudiobookID != 0 {
			m.Artist = variousAuthorNames[i]
			return true
		}

		// Try partial match
		bookTitleWild := logger.JoinStrings("%", *bookTitle, "%")
		sluggedTitleWild := logger.JoinStrings("%", sluggedTitle, "%")
		Scanrowsdyn(false,
			`SELECT ab.id FROM dbaudiobooks ab
			 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
			 WHERE aa.dbauthor_id = ?
			 AND (ab.title LIKE ? COLLATE NOCASE OR ab.slug LIKE ?)
			 LIMIT 1`,
			&m.DbaudiobookID, &authorID, &bookTitleWild, &sluggedTitleWild)

		if m.DbaudiobookID != 0 {
			m.Artist = variousAuthorNames[i]
			return true
		}
	}

	return false
}

// tryFindAuthorAndAudiobook attempts to find an author in the database and then find their audiobook.
// Also handles multi-author names by splitting and trying each author individually.
func (m *ParseInfo) tryFindAuthorAndAudiobook(resolved []resolvedAuthor, bookTitle *string) bool {
	if len(resolved) == 0 || bookTitle == nil || *bookTitle == "" {
		return false
	}

	sluggedTitle := logger.StringToSlug(*bookTitle)
	doPrefixMatch := strings.Count(*bookTitle, " ") >= 2

	var (
		prefixCandidates      []syncops.DbstaticTwoStringOneInt
		altPrefixCandidates   []syncops.DbstaticTwoStringOneInt
		cl                    int
		titleMatch, slugMatch bool
	)

	for resolvedid := range resolved {
		for k := range resolved[resolvedid].ids {
			// Try main audiobooks table with author filter
			Scanrowsdyn(false,
				`SELECT ab.id FROM dbaudiobooks ab
				 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
				 WHERE aa.dbauthor_id = ?
				 AND (ab.title = ? COLLATE NOCASE OR ab.slug = ?)
				 LIMIT 1`,
				&m.DbaudiobookID, &resolved[resolvedid].ids[k], bookTitle, &sluggedTitle)

			if m.DbaudiobookID != 0 {
				m.Artist = resolved[resolvedid].name
				return true
			}

			// Try alternate titles
			Scanrowsdyn(false,
				`SELECT abt.dbaudiobook_id FROM dbaudiobook_titles abt
				 JOIN dbaudiobook_authors aa ON abt.dbaudiobook_id = aa.dbaudiobook_id
				 WHERE aa.dbauthor_id = ?
				 AND (abt.title = ? COLLATE NOCASE OR abt.slug = ?)
				 LIMIT 1`,
				&m.DbaudiobookID, &resolved[resolvedid].ids[k], bookTitle, &sluggedTitle)

			if m.DbaudiobookID != 0 {
				m.Artist = resolved[resolvedid].name
				return true
			}

			// Try word-skipping: fetch all audiobook titles for this author once,
			// then match word-prefixes of bookTitle in Go (replaces O(Nx2) SQL queries).
			if doPrefixMatch {
				prefixCandidates = Getrowssize[syncops.DbstaticTwoStringOneInt](
					false,
					`SELECT count() FROM dbaudiobooks ab JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id WHERE aa.dbauthor_id = ?`,
					`SELECT ab.title, ab.slug, ab.id FROM dbaudiobooks ab
					 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
					 WHERE aa.dbauthor_id = ?`,
					&resolved[resolvedid].ids[k],
				)
				altPrefixCandidates = Getrowssize[syncops.DbstaticTwoStringOneInt](
					false,
					`SELECT count() FROM dbaudiobook_titles abt JOIN dbaudiobook_authors aa ON abt.dbaudiobook_id = aa.dbaudiobook_id WHERE aa.dbauthor_id = ?`,
					`SELECT abt.title, abt.slug, abt.dbaudiobook_id FROM dbaudiobook_titles abt
					 JOIN dbaudiobook_authors aa ON abt.dbaudiobook_id = aa.dbaudiobook_id
					 WHERE aa.dbauthor_id = ?`,
					&resolved[resolvedid].ids[k],
				)

				prefixCandidates = append(prefixCandidates, altPrefixCandidates...)
				for prefixCandidatesid := range prefixCandidates {
					if !strings.Contains(prefixCandidates[prefixCandidatesid].Str1, " ") {
						continue
					}

					cl = len(prefixCandidates[prefixCandidatesid].Str1)
					titleMatch = len(*bookTitle) >= cl &&
						strings.EqualFold(
							(*bookTitle)[:cl],
							prefixCandidates[prefixCandidatesid].Str1,
						) &&
						(len(*bookTitle) == cl || (*bookTitle)[cl] == ' ')

					slugMatch = strings.HasPrefix(
						sluggedTitle,
						prefixCandidates[prefixCandidatesid].Str2,
					) &&
						(len(sluggedTitle) == len(prefixCandidates[prefixCandidatesid].Str2) || sluggedTitle[len(prefixCandidates[prefixCandidatesid].Str2)] == '-')
					if titleMatch || slugMatch {
						m.DbaudiobookID = prefixCandidates[prefixCandidatesid].Num
						m.Artist = resolved[resolvedid].name
						return true
					}
				}
			}
		}
	}

	return false
}

// FindDbaudiobookByAuthorFirstFromWantedList searches for an audiobook by author name,
// prioritizing dbaudiobooks that are already in the user's wanted list (audiobooks table).
func (m *ParseInfo) FindDbaudiobookByAuthorFirstFromWantedList(listnames []string) {
	if len(listnames) == 0 {
		m.DbaudiobookID = 0
		return
	}

	rawTitle := m.File
	if rawTitle == "" {
		rawTitle = m.Title
	}

	if rawTitle == "" {
		m.DbaudiobookID = 0
		return
	}

	rawTitle = cleanRawNZBTitle(rawTitle)

	authorCache := make(map[string][]resolvedAuthor)

	var potentialAuthor, potentialTitle string

	// Try " - " first (standard format: "Author - Title")
	if before, after, ok := strings.Cut(rawTitle, " - "); ok {
		potentialAuthor = strings.TrimSpace(before)

		potentialTitle = strings.TrimSpace(after)
		if m.tryFindAuthorAndAudiobookFromWantedList(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
			listnames,
		) {
			return
		}
	}

	// Try "-" without spaces (scene format)
	if before, after, ok := strings.Cut(rawTitle, "-"); ok {
		potentialAuthor = strings.TrimSpace(before)
		potentialTitle = strings.TrimSpace(after)

		potentialAuthor = strings.ReplaceAll(potentialAuthor, ".", " ")
		potentialTitle = strings.ReplaceAll(potentialTitle, ".", " ")

		potentialTitle = cleanSceneGroupFromAlbum(potentialTitle)
		if m.tryFindAuthorAndAudiobookFromWantedList(
			cachedResolveAuthor(authorCache, potentialAuthor),
			&potentialTitle,
			listnames,
		) {
			return
		}
	}

	m.DbaudiobookID = 0
}

// tryFindAuthorAndAudiobookFromWantedList attempts to find an audiobook by author name,
// prioritizing dbaudiobooks that are in the user's wanted list.
func (m *ParseInfo) tryFindAuthorAndAudiobookFromWantedList(
	resolved []resolvedAuthor,
	bookTitle *string,
	listnames []string,
) bool {
	if len(resolved) == 0 || bookTitle == nil || *bookTitle == "" || len(listnames) == 0 {
		return false
	}

	sluggedTitle := logger.StringToSlug(*bookTitle)
	doPrefixMatch := strings.Count(*bookTitle, " ") >= 2

	for _, ra := range resolved {
		for raid := range ra.ids {
			for j := range listnames {
				// Try exact match in wanted list
				Scanrowsdyn(false,
					`SELECT a.dbaudiobook_id FROM audiobooks a
					 JOIN dbaudiobook_authors aa ON a.dbaudiobook_id = aa.dbaudiobook_id
					 JOIN dbaudiobooks db ON db.id = a.dbaudiobook_id
					 WHERE aa.dbauthor_id = ?
					 AND a.listname = ? COLLATE NOCASE
					 AND (db.title = ? COLLATE NOCASE OR db.slug = ?)
					 LIMIT 1`,
					&m.DbaudiobookID, &ra.ids[raid], &listnames[j], bookTitle, &sluggedTitle)

				if m.DbaudiobookID != 0 {
					m.Artist = ra.name
					m.AudiobookID = m.getAudiobookIDByDbaudiobookAndList(listnames[j])
					return true
				}

				// Try word-skipping: fetch all audiobook titles for this author+listname once,
				// then match word-prefixes of bookTitle in Go (replaces O(N) SQL queries).
				if doPrefixMatch {
					prefixCandidates := Getrowssize[syncops.DbstaticTwoStringOneInt](
						false,
						`SELECT count() FROM audiobooks a JOIN dbaudiobook_authors aa ON a.dbaudiobook_id = aa.dbaudiobook_id JOIN dbaudiobooks db ON db.id = a.dbaudiobook_id WHERE aa.dbauthor_id = ? AND a.listname = ? COLLATE NOCASE`,
						`SELECT db.title, db.slug, db.id FROM audiobooks a
						 JOIN dbaudiobook_authors aa ON a.dbaudiobook_id = aa.dbaudiobook_id
						 JOIN dbaudiobooks db ON db.id = a.dbaudiobook_id
						 WHERE aa.dbauthor_id = ? AND a.listname = ? COLLATE NOCASE`,
						&ra.ids[raid],
						&listnames[j],
					)
					for _, cand := range prefixCandidates {
						if !strings.Contains(cand.Str1, " ") {
							continue
						}

						cl := len(cand.Str1)
						titleMatch := len(*bookTitle) >= cl &&
							strings.EqualFold((*bookTitle)[:cl], cand.Str1) &&
							(len(*bookTitle) == cl || (*bookTitle)[cl] == ' ')

						slugMatch := strings.HasPrefix(sluggedTitle, cand.Str2) &&
							(len(sluggedTitle) == len(cand.Str2) || sluggedTitle[len(cand.Str2)] == '-')
						if titleMatch || slugMatch {
							m.DbaudiobookID = cand.Num
							m.Artist = ra.name
							m.AudiobookID = m.getAudiobookIDByDbaudiobookAndList(listnames[j])
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// getAudiobookIDByDbaudiobookAndList retrieves the audiobook ID from the audiobooks table.
func (m *ParseInfo) getAudiobookIDByDbaudiobookAndList(listname string) uint {
	return ScanRowVal2[uint, string, uint](
		"SELECT id FROM audiobooks WHERE dbaudiobook_id = ? AND listname = ? COLLATE NOCASE",
		m.DbaudiobookID, listname)
}

// SetEpisodeIDfromM sets the SerieEpisodeID field of the ParseInfo struct based on the SerieID and DbserieEpisodeID fields.
// If SerieID or DbserieEpisodeID is 0, SerieEpisodeID is set to 0.
// Otherwise, it queries the database to find the corresponding serie_episodes record and sets SerieEpisodeID.
func (m *ParseInfo) SetEpisodeIDfromM() {
	if m.SerieID == 0 || m.DbserieEpisodeID == 0 {
		m.SerieEpisodeID = 0
		return
	}

	Scanrowsdyn(
		false,
		"select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?",
		&m.SerieEpisodeID,
		&m.DbserieEpisodeID,
		&m.SerieID,
	)
}

// SetDBEpisodeIDfromM sets the DbserieEpisodeID field on the FileParser struct by looking
// up the episode ID in the database based on the season, episode, and identifier fields.
func (m *ParseInfo) SetDBEpisodeIDfromM() {
	if m.DbserieID == 0 {
		m.DbserieEpisodeID = 0
		return
	}

	ident2 := logger.StringReplaceWith(m.Identifier, '.', ' ')
	ident3 := logger.StringReplaceWith(
		m.Identifier,
		'.',
		'-',
	) // scraper stores "YY-MM-DD" (dots→dashes)
	ident4 := logger.StringReplaceWith(m.Identifier, ' ', '-')

	// Only match by season+episode when they were actually parsed from the title.
	// Matching on empty SeasonStr/EpisodeStr would hit every scraper-imported episode
	// (which has season='' and episode='' by default), returning a wrong random episode.
	if m.SeasonStr != "" && m.EpisodeStr != "" {
		Scanrowsdyn(
			false,
			`select id from dbserie_episodes where dbserie_id = ? and (
				(season = ? and episode = ?) or
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE
			) limit 1`,
			&m.DbserieEpisodeID,
			&m.DbserieID,
			&m.SeasonStr, &m.EpisodeStr,
			&m.Identifier, &ident2, &ident3, &ident4,
		)
	} else {
		Scanrowsdyn(
			false,
			`select id from dbserie_episodes where dbserie_id = ? and (
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE or
				identifier = ? COLLATE NOCASE
			) limit 1`,
			&m.DbserieEpisodeID,
			&m.DbserieID,
			&m.Identifier, &ident2, &ident3, &ident4,
		)
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

// Cleanimdbdbmovie clears the Imdb and DbmovieID fields in the FileParser struct to empty values.
// This is used to reset the state when a lookup fails.
func (m *ParseInfo) Cleanimdbdbmovie() {
	m.Imdb = ""
	m.DbmovieID = 0
}

// CacheThreeStringIntIndexFuncGetImdb retrieves the IMDB value from a cached array of DbstaticThreeStringTwoInt objects that match the provided string and uint values. If a matching object is found, the IMDB value is stored in the ParseInfo struct. If no matching object is found, this method does nothing.
func (m *ParseInfo) CacheThreeStringIntIndexFuncGetImdb() {
	for a := range GetCachedThreeStringSeq(logger.CacheDBMovie, false, true) {
		if a.Num2 == m.DbmovieID {
			m.Imdb = a.Str3
			return
		}
	}
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
	case 5:
		id = m.AudioFormatID
	}

	for idx := range tbl {
		if tbl[idx].ID == id {
			return idx
		}
	}

	return -1
}

// IsAudioFormatWanted checks if the audio format is in the wanted list.
// Returns true if the format is wanted or if the wanted list is empty (allow all).
func (m *ParseInfo) IsAudioFormatWanted(quality *config.QualityConfig) bool {
	if quality == nil || quality.WantedAudioFormatsLen == 0 {
		return true // No filter, allow all
	}

	if m.AudioFormat == "" {
		return false // No format detected, reject
	}

	for _, wanted := range quality.WantedAudioFormats {
		if strings.EqualFold(m.AudioFormat, wanted) {
			return true
		}
	}

	return false
}

// IsAudioBitrateAcceptable checks if the audio bitrate meets the minimum requirement.
// Returns true if bitrate >= minimum or if no minimum is set (0).
func (m *ParseInfo) IsAudioBitrateAcceptable(quality *config.QualityConfig) bool {
	if quality == nil || quality.MinAudioBitrate <= 0 {
		return true // No minimum, allow all
	}

	// For lossless formats, always accept (bitrate varies with content)
	format := strings.ToLower(m.AudioFormat)
	switch format {
	case "flac", "alac", "wav", "aiff", "ape", "wv", "wavpack", "dsd", "dsf":
		return true
	}

	return m.AudioBitrate >= quality.MinAudioBitrate
}

// Gettypeids searches through the provided qualitytype slice to find a match for
// the given input string inval. It checks the Strings and Regex fields of each
// QualitiesRegex struct, returning the ID if a match is found. 0 is returned if no
// match is found.
func (m *ParseInfo) Gettypeids(inval string, qualitytype []Qualities) uint {
	for idx := range qualitytype {
		qual := &qualitytype[idx]
		if qual.Strings != "" && !config.GetSettingsGeneral().DisableParserStringMatch &&
			logger.SlicesContainsI(qual.StringsLowerSplitted, inval) {
			if qual.ID != 0 {
				return qual.ID
			}
		}

		if qual.UseRegex && qual.Regex != "" &&
			RegexGetMatchesFind(qual.Regex, inval, 2) {
			return qual.ID
		}
	}

	return 0
}

// Parsegroup parses a group of strings from the input string and updates the corresponding fields in the ParseInfo struct.
// The function takes a name string, a boolean onlyifempty, and a slice of group strings as input. It searches for each group string in the input string and extracts the matched substring.
// If the matched substring is not empty and is not part of a larger word, the function updates the corresponding field in the ParseInfo struct based on the name parameter. If onlyifempty is true, the function will only update the field if it is currently empty.
// The function supports the following names: "audio", "codec", "quality", "resolution", "extended", "proper", and "repack".
func (m *ParseInfo) Parsegroup(name string, onlyifempty bool, group []string) {
	for _, groupItem := range group {
		index := logger.IndexI(m.Str, groupItem)
		if index == -1 {
			continue
		}

		indexmax := index + len(groupItem)

		if m.Str[index:indexmax] == "" {
			continue
		}

		if indexmax < len(m.Str) && checkDigitLetter((m.Str[indexmax])) {
			continue
		}

		if index > 0 && checkDigitLetter((m.Str[index-1])) {
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
	if indexmax < len(m.Str) && checkDigitLetter((m.Str[indexmax])) {
		return
	}

	if index > 0 && checkDigitLetter((m.Str[index-1])) {
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

// GetSluggedMap returns the appropriate SQL query string based on whether the
// caller wants to use a slugged or default lookup. The returned string can be
// used to query the database for a record matching the provided type string.
func GetSluggedMap(slugged bool, typestr string) string {
	if slugged {
		return mapSlugged[typestr].Slugged
	}

	return mapSlugged[typestr].Default
}

// FilterDbstaticTwoStringOneInt filters a slice of DbstaticTwoStringOneInt structs by the provided id. It returns a sequence that yields the filtered elements.
func FilterDbstaticTwoStringOneInt(
	s []syncops.DbstaticTwoStringOneInt,
	id uint,
) iter.Seq[syncops.DbstaticTwoStringOneInt] {
	return func(yield func(syncops.DbstaticTwoStringOneInt) bool) {
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
		if strings.EqualFold(tbl[idx].Name, str) {
			return idx
		}
	}

	return -1
}

// addPeriodsToInitials converts names like "A Zavarelli" to "A. Zavarelli"
// by adding periods after single-letter words (initials).
// This helps match author names that lost their periods during scene format conversion.
func addPeriodsToInitials(name string) string {
	// Fast path: scan raw string for an isolated single letter before allocating.
	found := false
	for i := range len(name) {
		b := name[i]
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') {
			before := i == 0 || name[i-1] == ' '

			after := i+1 == len(name) || name[i+1] == ' '
			if before && after {
				found = true
				break
			}
		}
	}

	if !found {
		return name
	}

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	first := true
	for word := range strings.FieldsSeq(name) {
		if !first {
			buf.WriteByte(' ')
		}

		buf.WriteString(word)

		if len(word) == 1 &&
			((word[0] >= 'A' && word[0] <= 'Z') || (word[0] >= 'a' && word[0] <= 'z')) {
			buf.WriteByte('.')
		}

		first = false
	}

	return buf.String()
}

// cleanSceneGroupFromAlbum removes scene group names from album titles.
// Scene format: "Album Title-Quality-Year-GROUP" → "Album Title"
// It removes short alphanumeric segments after dashes that look like scene tags.
func cleanSceneGroupFromAlbum(album string) string {
	// Split by dash and analyze each part
	parts := strings.Split(album, "-")
	if len(parts) <= 1 {
		return album
	}

	// Keep parts that look like album title words, remove scene tags
	var cleanParts []string
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// First part is always kept (start of album title)
		if i == 0 {
			cleanParts = append(cleanParts, part)
			continue
		}

		// Check if this part looks like a scene tag (short, alphanumeric, all caps or common tags)
		if looksLikeSceneTag(part) {
			// Stop here - everything after is likely scene metadata
			break
		}

		cleanParts = append(cleanParts, part)
	}

	return logger.JoinStringsSep(cleanParts, " - ")
}

// looksLikeSceneTag checks if a string looks like a scene release tag.
// Scene tags are typically short (2-10 chars), alphanumeric, and often all uppercase.
func looksLikeSceneTag(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 2 || len(s) > 12 {
		return false
	}

	// Common scene tags (case insensitive)
	commonTags := []string{
		"FLAC", "MP3", "AAC", "OGG", "WAV", "ALAC", "APE", "WMA",
		"WEB", "CD", "VINYL", "HDTV", "SAT", "DVDRip", "BDRip",
		"OST", "EP", "LP", "SINGLE", "ALBUM", "LIVE", "BOOTLEG",
		"PROPER", "REPACK", "INT", "INTERNAL", "RETAiL",
		"DELUXE", "REMASTERED", "LIMITED",
	}

	upperS := strings.ToUpper(s)
	if slices.Contains(commonTags, upperS) {
		return true
	}

	// Check if it's a year (4 digits starting with 19 or 20)
	if len(s) == 4 && (strings.HasPrefix(s, "19") || strings.HasPrefix(s, "20")) {
		allDigits := true
		for _, c := range s {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}

		if allDigits {
			return true
		}
	}

	// Check if it's a short all-uppercase alphanumeric string (likely a group name)
	// Group names are typically 2-10 chars, all uppercase letters/numbers
	if len(s) >= 2 && len(s) <= 10 {
		allUpperAlnum := true

		hasLetter := false
		for _, c := range s {
			if c >= 'A' && c <= 'Z' {
				hasLetter = true
			} else if c >= '0' && c <= '9' {
				// digits are ok
			} else if c >= 'a' && c <= 'z' {
				// lowercase means it's probably not a scene tag
				allUpperAlnum = false
				break
			} else {
				allUpperAlnum = false
				break
			}
		}

		if allUpperAlnum && hasLetter {
			return true
		}
	}

	return false
}

// checkDigitLetter checks if the given byte is an alphanumeric character.
// It returns true if the byte is a digit (0-9) or a letter (uppercase or lowercase), otherwise false.
func checkDigitLetter(b byte) bool {
	return ((b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z'))
}

// Patterns for stripping release type indicators from album titles (for fallback matching).
var (
	catalogPattern     = regexp.MustCompile(`\s*\([A-Z0-9][A-Z0-9\s\-]*\)`)
	releaseTypePattern = regexp.MustCompile(
		`(?i)(?:^|\s|-|_)(DELUXE\s*EDITION|DELUXE|REMASTERED|REMASTER|EXPANDED|LIMITED\s*EDITION|SPECIAL\s*EDITION|BONUS\s*TRACKS?)(?:\s|-|_|$)`,
	)
	sceneReleasePattern = regexp.MustCompile(
		`(?i)(?:^|\s|-|_)(REISSUE|RETAIL|ADVANCE|PROMO|PROPER|REPACK|INT|INTERNAL)(?:\s|-|_|$)`,
	)
	anniversaryPattern = regexp.MustCompile(
		`(?i)(?:^|\s|-|_)(\d+(?:st|nd|rd|th)\s*Anniversary\s*Edition)(?:\s|-|_|$)`,
	)
	countryCodePattern = regexp.MustCompile(
		`(?i)(?:^|\s)(DE|US|UK|EU|JP|AU|CA|FR|IT|ES|NL|SE|NO|DK|FI|AT|CH|BE)(?:\s|$)`,
	)
	yearPattern = regexp.MustCompile(`(?:^|\s)(19\d{2}|20\d{2})(?:\s|$)`)
)

// stripReleaseType removes release type indicators from album titles.
// Used for fallback matching when exact title match fails.
func stripReleaseType(album string) string {
	// Remove catalog numbers in parentheses
	if catalogPattern.MatchString(album) {
		album = catalogPattern.ReplaceAllString(album, "")
	}

	patterns := []*regexp.Regexp{
		releaseTypePattern,
		sceneReleasePattern,
		anniversaryPattern,
		countryCodePattern,
		yearPattern,
	}

	for range 3 {
		changed := false
		for _, p := range patterns {
			if p.MatchString(album) {
				album = p.ReplaceAllString(album, " ")
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	for strings.Contains(album, "  ") {
		album = strings.ReplaceAll(album, "  ", " ")
	}

	return strings.TrimSpace(album)
}
