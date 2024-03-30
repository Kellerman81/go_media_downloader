package logger

import (
	"bytes"
	"errors"
	"html"
	"io"
	"io/fs"
	"net/url"
	"path"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/goccy/go-json"
	"github.com/pelletier/go-toml/v2"

	"github.com/mozillazg/go-unidecode/table"
	"github.com/rs/zerolog"
)

const (
	FilterByID                = "id = ?"
	StrRefreshMovies          = "Refresh Movies"
	StrRefreshMoviesInc       = "Refresh Movies Incremental"
	StrRefreshSeries          = "Refresh Series"
	StrRefreshSeriesInc       = "Refresh Series Incremental"
	StrDebug                  = "debug"
	StrDate                   = "date"
	StrID                     = "id"
	StrURL                    = "Url"
	StrQuery                  = "Query"
	StrJob                    = "Job"
	StrJobLower               = "job"
	StrFile                   = "File"
	StrPath                   = "Path"
	StrListname               = "Listname"
	StrImdb                   = "imdb"
	StrTitle                  = "Title"
	StrPriority               = "Priority"
	StrSearchMissingInc       = "searchmissinginc"
	StrSearchMissingFull      = "searchmissingfull"
	StrSearchMissingIncTitle  = "searchmissinginctitle"
	StrSearchMissingFullTitle = "searchmissingfulltitle"
	StrSearchUpgradeInc       = "searchupgradeinc"
	StrSearchUpgradeFull      = "searchupgradefull"
	StrSearchUpgradeIncTitle  = "searchupgradeinctitle"
	StrSearchUpgradeFullTitle = "searchupgradefulltitle"
	StrStructure              = "structure"
	StrCheckMissing           = "checkmissing"
	StrCheckMissingFlag       = "checkmissingflag"
	StrUpgradeFlag            = "checkupgradeflag"
	StrReachedFlag            = "checkreachedflag"
	StrClearHistory           = "clearhistory"
	StrRssSeasonsAll          = "rssseasonsall"
	StrSerie                  = "serie"
	StrRssSeasons             = "rssseasons"
	StrRss                    = "rss"
	Underscore                = "_"
	StrTvdb                   = "tvdb"
	StrTt                     = "tt"
	StrSeries                 = "series"
	CacheDBMedia              = "CacheDBMedia"
	DBCountDBMedia            = "DBCountDBMedia"
	DBCacheDBMedia            = "DBCacheDBMedia"
	CacheMedia                = "CacheMedia"
	DBCountMedia              = "DBCountMedia"
	DBCacheMedia              = "DBCacheMedia"
	CacheHistoryTitle         = "CacheHistoryTitle"
	CacheHistoryUrl           = "CacheHistoryUrl"
	DBHistoriesUrl            = "DBHistoriesUrl"
	DBHistoriesTitle          = "DBHistoriesTitle"
	DBCountHistoriesUrl       = "DBCountHistoriesUrl"
	DBCountHistoriesTitle     = "DBCountHistoriesTitle"
	CacheMediaTitles          = "CacheMediaTitles"
	DBCountDBTitles           = "DBCountDBTitles"
	DBCacheDBTitles           = "DBCacheDBTitles"
	CacheFiles                = "CacheFiles"
	DBCountFiles              = "DBCountFiles"
	DBCacheFiles              = "DBCacheFiles"
	CacheUnmatched            = "CacheUnmatched"
	DBCountUnmatched          = "DBCountUnmatched"
	DBCacheUnmatched          = "DBCacheUnmatched"
	DBCountFilesLocation      = "DBCountFilesLocation"
	DBCountUnmatchedPath      = "DBCountUnmatchedPath"
	DBCountDBTitlesDBID       = "DBCountDBTitlesDBID"
	DBDistinctDBTitlesDBID    = "DBDistinctDBTitlesDBID"
	DBMediaTitlesID           = "DBMediaTitlesID"
	DBFilesQuality            = "DBFilesQuality"
	DBCountFilesByList        = "DBCountFilesByList"
	DBLocationFilesByList     = "DBLocationFilesByList"
	DBIDsFilesByLocation      = "DBIDsFilesByLocation"
	DBCountFilesByMediaID     = "DBCountFilesByMediaID"
	DBCountFilesByLocation    = "DBCountFilesByLocation"
	TableFiles                = "TableFiles"
	TableMedia                = "TableMedia"
	DBCountMediaByList        = "DBCountMediaByList"
	DBIDMissingMediaByList    = "DBIDMissingMediaByList"
	DBUpdateMissing           = "DBUpdateMissing"
	DBListnameByMediaID       = "DBListnameByMediaID"
	DBRootPathFromMediaID     = "DBRootPathFromMediaID"
	DBDeleteFileByIDLocation  = "DBDeleteFileByIDLocation"
	DBCountHistoriesByTitle   = "DBCountHistoriesByTitle"
	DBCountHistoriesByUrl     = "DBCountHistoriesByUrl"
	DBLocationIDFilesByID     = "DBLocationIDFilesByID"
	DBFilePrioFilesByID       = "DBFilePrioFilesByID"
	UpdateMediaLastscan       = "UpdateMediaLastscan"
	DBQualityMediaByID        = "DBQualityMediaByID"
	SearchGenSelect           = "SearchGenSelect"
	SearchGenTable            = "SearchGenTable"
	SearchGenMissing          = "SearchGenMissing"
	SearchGenMissingEnd       = "SearchGenMissingEnd"
	SearchGenReached          = "SearchGenReached"
	SearchGenLastScan         = "SearchGenLastScan"
	SearchGenDate             = "SearchGenDate"
	SearchGenOrder            = "SearchGenOrder"

	CacheMovie              = "CacheMovie"
	CacheSeries             = "CacheSeries"
	CacheDBMovie            = "CacheDBMovie"
	CacheDBSeries           = "CacheDBSeries"
	CacheDBSeriesAlt        = "CacheDBSeriesAlt"
	CacheTitlesMovie        = "CacheTitlesMovie"
	CacheUnmatchedMovie     = "CacheUnmatchedMovie"
	CacheUnmatchedSeries    = "CacheUnmatchedSeries"
	CacheFilesMovie         = "CacheFilesMovie"
	CacheFilesSeries        = "CacheFilesSeries"
	CacheHistoryUrlMovie    = "CacheHistoryUrlMovie"
	CacheHistoryTitleMovie  = "CacheHistoryTitleMovie"
	CacheHistoryUrlSeries   = "CacheHistoryUrlSeries"
	CacheHistoryTitleSeries = "CacheHistoryTitleSeries"
)

var (
	V0          uint = 0
	StrFeeds         = "feeds"
	StrData          = "data"
	StrDataFull      = "datafull"
	StrMovie         = "movie"
	timeFormat       = time.RFC3339Nano
	Log         zerolog.Logger
	timeZone    = *time.UTC
	//diacriticsReplacer   = strings.NewReplacer("ä", "ae", "ö", "oe", "ü", "ue", "Ä", "Ae", "Ö", "Oe", "Ü", "Ue", "ß", "ss")
	//pathReplacer         = strings.NewReplacer(":", "", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "")
	ErrNoID              = errors.New("no id")
	ErrNoFiles           = errors.New("no files")
	ErrNotFound          = errors.New("not found")
	Errwrongtype         = errors.New("wrong type")
	ErrRuntime           = errors.New("wrong runtime")
	ErrNotAllowed        = errors.New("not allowed")
	ErrLowerQuality      = errors.New("lower quality")
	ErrOther             = errors.New("other error")
	ErrDisabled          = errors.New("disabled")
	ErrToWait            = errors.New("please wait")
	ErrTooSmall          = errors.New("too small")
	ErrDailyLimit        = errors.New("daily limit reached")
	Errnoresults         = errors.New("no results")
	ErrNotFoundDbmovie   = errors.New("dbmovie not found")
	ErrNotFoundMovie     = errors.New("movie not found")
	ErrNotFoundDbserie   = errors.New("dbserie not found")
	ErrNotFoundSerie     = errors.New("serie not found")
	ErrNotEnabled        = errors.New("not enabled")
	ErrCfgpNotFound      = errors.New("cfgpstr not found")
	ErrNotFoundDBEpisode = errors.New("dbepisode not found")
	ErrNotFoundEpisode   = errors.New("episode not found")
	ErrListnameEmpty     = errors.New("listname empty")
	ErrTvdbEmpty         = errors.New("tvdb empty")
	ErrImdbEmpty         = errors.New("imdb empty")
	ErrSearchvarEmpty    = errors.New("searchvar empty")
	ErrTracksEmpty       = errors.New("tracks empty")
	PlBuffer             = pool.NewPool(100, 0, func(b *bytes.Buffer) {}, func(b *bytes.Buffer) {
		b.Reset()
	})

	textparser = template.New("master")
	subRune    = map[rune]struct{}{
		'a': {},
		'b': {},
		'c': {},
		'd': {},
		'e': {},
		'f': {},
		'g': {},
		'h': {},
		'i': {},
		'j': {},
		'k': {},
		'l': {},
		'm': {},
		'n': {},
		'o': {},
		'p': {},
		'q': {},
		'r': {},
		's': {},
		't': {},
		'u': {},
		'v': {},
		'w': {},
		'x': {},
		'y': {},
		'z': {},
		'0': {},
		'1': {},
		'2': {},
		'3': {},
		'4': {},
		'5': {},
		'6': {},
		'7': {},
		'8': {},
		'9': {},
		'-': {},
	}
	substituteRuneSpace = map[rune]string{
		'&':  "and",
		'@':  "at",
		'"':  "",
		'\'': "",
		'’':  "",
		'_':  "",
		' ':  "-",
		'‒':  "-",
		'–':  "-",
		'—':  "-",
		'―':  "-",
		'ä':  "ae",
		'ö':  "oe",
		'ü':  "ue",
		'Ä':  "Ae",
		'Ö':  "Oe",
		'Ü':  "Ue",
		'ß':  "ss",
	}
	diacriticsmap = map[rune]string{
		'ä': "ae",
		'ö': "oe",
		'ü': "ue",
		'Ä': "Ae",
		'Ö': "Oe",
		'Ü': "Ue",
		'ß': "ss",
	}
	pathmap = map[rune]string{
		':':  "",
		'*':  "",
		'?':  "",
		'\\': "",
		'<':  "",
		'>':  "",
		'|':  "",
	}

	mapstringsmovies = map[string]string{
		CacheDBMedia:             CacheDBMovie,
		DBCountDBMedia:           "select count() from dbmovies",
		DBCacheDBMedia:           "select title, slug, imdb_id, year, id from dbmovies",
		CacheMedia:               CacheMovie,
		DBCountMedia:             "select count() from movies",
		DBCacheMedia:             "select lower(listname), dbmovie_id, id from movies",
		CacheHistoryTitle:        CacheHistoryTitleMovie,
		CacheHistoryUrl:          CacheHistoryUrlMovie,
		DBHistoriesUrl:           "select distinct url from movie_histories",
		DBHistoriesTitle:         "select distinct title from movie_histories",
		DBCountHistoriesUrl:      "select count() from (select distinct url from movie_histories)",
		DBCountHistoriesTitle:    "select count() from (select distinct title from movie_histories)",
		CacheMediaTitles:         CacheTitlesMovie,
		DBCountDBTitles:          "select count() from dbmovie_titles",
		DBCacheDBTitles:          "select title, slug, dbmovie_id from dbmovie_titles",
		CacheFiles:               CacheFilesMovie,
		DBCountFiles:             "select count() from movie_files",
		DBCacheFiles:             "select location from movie_files",
		CacheUnmatched:           CacheUnmatchedMovie,
		DBCountUnmatched:         "select count() from movie_file_unmatcheds where (last_checked > ? or last_checked is null)",
		DBCacheUnmatched:         "select filepath from movie_file_unmatcheds where (last_checked > ? or last_checked is null)",
		DBCountFilesLocation:     "select count() from movie_files where location = ?",
		DBCountUnmatchedPath:     "select count() from movie_file_unmatcheds where filepath = ?",
		DBCountDBTitlesDBID:      "select count() from (select distinct title, slug from dbmovie_titles where dbmovie_id = ? and title != '')",
		DBDistinctDBTitlesDBID:   "select distinct title, slug, 0 from dbmovie_titles where dbmovie_id = ? and title != ''",
		DBMediaTitlesID:          "select year, title, slug from dbmovies where id = ?",
		DBFilesQuality:           "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?",
		DBCountFilesByList:       "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		DBLocationFilesByList:    "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		DBIDsFilesByLocation:     "select id, movie_id from movie_files where location = ?",
		DBCountFilesByMediaID:    "select count() from movie_files where movie_id = ?",
		DBCountFilesByLocation:   "select count() from movie_files where location = ?",
		TableFiles:               "movie_files",
		TableMedia:               "movies",
		DBCountMediaByList:       "select count() from movies where listname = ? COLLATE NOCASE",
		DBIDMissingMediaByList:   "select id,missing from movies where listname = ? COLLATE NOCASE",
		DBUpdateMissing:          "update movies set missing = ? where id = ?",
		DBListnameByMediaID:      "select listname from movies where id = ?",
		DBRootPathFromMediaID:    "select rootpath from movies where id = ?",
		DBDeleteFileByIDLocation: "delete from movie_files where movie_id = ? and location = ?",
		DBCountHistoriesByTitle:  "select count() from movie_histories where title = ?",
		DBCountHistoriesByUrl:    "select count() from movie_histories where url = ?",
		DBLocationIDFilesByID:    "select location, id from movie_files where movie_id = ?",
		DBFilePrioFilesByID:      "select location, movie_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from movie_files where movie_id = ?",
		UpdateMediaLastscan:      "update movies set lastscan = datetime('now','localtime') where id = ?",
		DBQualityMediaByID:       "select quality_profile from movies where id = ?",
		SearchGenSelect:          "select movies.quality_profile, movies.id ",
		SearchGenTable:           " from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ",
		SearchGenMissing:         "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?",
		SearchGenMissingEnd:      ")",
		SearchGenReached:         "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
		SearchGenLastScan:        " and (movies.lastscan is null or movies.Lastscan < ?)",
		SearchGenDate:            " and (dbmovies.release_date < ? or dbmovies.release_date is null)",
		SearchGenOrder:           " order by movies.Lastscan asc",
	}
	mapstringsseries = map[string]string{
		CacheDBMedia:             CacheDBSeries,
		DBCountDBMedia:           "select count() from dbseries",
		DBCacheDBMedia:           "select seriename, slug, id from dbseries",
		CacheMedia:               CacheSeries,
		DBCountMedia:             "select count() from series",
		DBCacheMedia:             "select lower(listname), dbserie_id, id from series",
		CacheHistoryTitle:        CacheHistoryTitleSeries,
		CacheHistoryUrl:          CacheHistoryUrlSeries,
		DBHistoriesUrl:           "select distinct url from serie_episode_histories",
		DBHistoriesTitle:         "select distinct title from serie_episode_histories",
		DBCountHistoriesUrl:      "select count() from (select distinct url from serie_episode_histories)",
		DBCountHistoriesTitle:    "select count() from (select distinct title from serie_episode_histories)",
		CacheMediaTitles:         CacheDBSeriesAlt,
		DBCountDBTitles:          "select count() from dbserie_alternates",
		DBCacheDBTitles:          "select title, slug, dbserie_id from dbserie_alternates",
		CacheFiles:               CacheFilesSeries,
		DBCountFiles:             "select count() from serie_episode_files",
		DBCacheFiles:             "select location from serie_episode_files",
		CacheUnmatched:           CacheUnmatchedSeries,
		DBCountUnmatched:         "select count() from serie_file_unmatcheds where (last_checked > ? or last_checked is null)",
		DBCacheUnmatched:         "select filepath from serie_file_unmatcheds where (last_checked > ? or last_checked is null)",
		DBCountFilesLocation:     "select count() from serie_episode_files where location = ?",
		DBCountUnmatchedPath:     "select count() from serie_file_unmatcheds where filepath = ?",
		DBCountDBTitlesDBID:      "select count() from (select distinct title, slug from dbserie_alternates where dbserie_id = ? and title != '')",
		DBDistinctDBTitlesDBID:   "select distinct title, slug, 0 from dbserie_alternates where dbserie_id = ? and title != ''",
		DBMediaTitlesID:          "select 0, seriename, slug from dbseries where id = ?",
		DBFilesQuality:           "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?",
		DBCountFilesByList:       "select count() from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		DBLocationFilesByList:    "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		DBIDsFilesByLocation:     "select id, serie_episode_id from serie_episode_files where location = ?",
		DBCountFilesByMediaID:    "select count() from serie_episode_files where serie_episode_id = ?",
		DBCountFilesByLocation:   "select count() from serie_episode_files where location = ?",
		TableFiles:               "serie_episode_files",
		TableMedia:               "serie_episodes",
		DBCountMediaByList:       "select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		DBIDMissingMediaByList:   "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		DBUpdateMissing:          "update serie_episodes set missing = ? where id = ?",
		DBListnameByMediaID:      "select listname from series where id = ?",
		DBRootPathFromMediaID:    "select rootpath from series where id = ?",
		DBDeleteFileByIDLocation: "delete from serie_episode_files where serie_id = ? and location = ?",
		DBCountHistoriesByTitle:  "select count() from serie_episode_histories where title = ?",
		DBCountHistoriesByUrl:    "select count() from serie_episode_histories where url = ?",
		DBLocationIDFilesByID:    "select location, id from serie_episode_files where serie_episode_id = ?",
		DBFilePrioFilesByID:      "select location, serie_episode_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from serie_episode_files where serie_episode_id = ?",
		UpdateMediaLastscan:      "update serie_episodes set lastscan = datetime('now','localtime') where id = ?",
		DBQualityMediaByID:       "select quality_profile from serie_episodes where id = ?",
		SearchGenSelect:          "select serie_episodes.quality_profile, serie_episodes.id ",
		SearchGenTable:           " from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where ",
		SearchGenMissing:         "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
		SearchGenMissingEnd:      ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)",
		SearchGenReached:         "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
		SearchGenLastScan:        " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)",
		SearchGenDate:            " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)",
		SearchGenOrder:           " order by serie_episodes.Lastscan asc",
	}
)

// GetTimeZone returns a pointer to the time.Location representing the
// timezone used for formatting logs. This allows checking the current
// timezone.
func GetTimeZone() *time.Location {
	return &timeZone
}

// GetTimeFormat returns the time format string used for formatting logs.
// This allows checking the current time format.
func GetTimeFormat() string {
	return timeFormat
}

// ParseJson decodes the JSON encoded data from the provided reader into
// the given object. Returns any error encountered during decoding.
func ParseJson(r io.Reader, obj any) error {
	return json.NewDecoder(r).Decode(obj)
}

// ParseToml decodes the TOML encoded data from the provided reader into
// the given object. Returns any error encountered during decoding.
func ParseToml(r io.Reader, obj any) error {
	return toml.NewDecoder(r).Decode(obj)
}

// ParseStringTemplate parses a text/template string into a template.Template, caches it, and executes it with the given data.
// It returns the executed template string and any error encountered.
func ParseStringTemplate(message string, messagedata any) (bool, string) {
	tmplmessage := textparser.Lookup(message)
	if tmplmessage == nil {
		var err error
		tmplmessage, err = textparser.New(message).Parse(message)
		if err != nil {
			LogDynamic("error", "template", NewLogFieldValue(err))
			return true, ""
		}
	}
	doc := PlBuffer.Get()
	if err := tmplmessage.Execute(doc, messagedata); err != nil {
		LogDynamic("error", "template", NewLogFieldValue(err))
		return true, ""
	}
	defer PlBuffer.Put(doc)
	return false, doc.String()
}

// unidecode2 converts a unicode string to an ASCII transliteration by
// replacing each unicode rune with its best ASCII approximation. It handles
// special cases like converting to lowercase and inserting separators between
// contiguous substitutions. This allows sanitizing unicode strings into
// a more filesystem-friendly ASCII format.
func unidecode2(s string) []byte {
	ret := PlBuffer.Get()
	var laststr string
	var lastrune rune
	//var c byte
	if strings.ContainsRune(s, '&') {
		s = html.UnescapeString(s)
	}
	ret.Grow(len(s) + 10)
	for _, r := range s {
		if val, ok := substituteRuneSpace[r]; ok {
			if laststr != "" && val == laststr {
				continue
			}
			if lastrune == '-' && val == "-" {
				continue
			}
			ret.WriteString(val)
			laststr = val
			if val == "-" {
				lastrune = '-'
			} else {
				lastrune = ' '
			}
			continue
		}
		if laststr != "" {
			laststr = ""
		}

		if r < unicode.MaxASCII {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			if _, ok := subRune[r]; !ok {
				if lastrune == '-' {
					continue
				}
				lastrune = '-'
				ret.WriteRune('-')
			} else {
				if lastrune == '-' && r == '-' {
					continue
				}
				lastrune = r
				ret.WriteRune(r)
			}
			continue
		}
		if r > 0xeffff {
			continue
		}

		section := r >> 8   // Chop off the last two hex digits
		position := r % 256 // Last two hex digits
		if tb, ok := table.Tables[section]; ok {
			if len(tb) > int(position) {
				if len(tb[position]) >= 1 {
					if tb[position][0] > unicode.MaxASCII && lastrune != '-' {
						lastrune = '-'
						ret.WriteRune('-')
						continue
					}
				}
				if lastrune == '-' && tb[position] == "-" {
					continue
				}
				ret.WriteString(tb[position])
			}
		}
	}
	defer PlBuffer.Put(ret)
	return ret.Bytes()
}

// StringToSlug converts the given string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlug(instr string) string {
	if instr == "" {
		return ""
	}
	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}
	inbyte = bytes.TrimRight(inbyte, "- ")
	inbyte = bytes.TrimLeft(inbyte, "- ")
	return string(inbyte)
}

// AddImdbPrefix prepends the imdb prefix if it doesn't already exist.
func AddImdbPrefix(str string) string {
	if len(str) >= 1 && !HasPrefixI(str, StrTt) {
		return JoinStrings(StrTt, str)
	}
	return str
}

// AddTvdbPrefix prepends the tvdb prefix if it doesn't already exist.
func AddTvdbPrefix(str string) string {
	if len(str) >= 1 && !HasPrefixI(str, StrTvdb) {
		//return JoinStrings(StrTvdb, str)
		return StrTvdb + str
	}
	return str
}

// Path sanitizes the given string by cleaning it with path.Clean, unquoting
// and unescaping it, optionally removing slashes, and replacing invalid
// characters. It returns the cleaned path string.
func Path(s string, allowslash bool) string {
	if s == "" {
		return ""
	}
	s = UnquoteUnescape(s)
	s = path.Clean(s)
	if !allowslash {
		s = StringRemoveAllRunesMulti(s, '\\', '/')
		//s = StringRemoveAllRunes(s, '\\')
		//s = StringRemoveAllRunes(s, '/')
	}
	s = pathReplacer(s)

	if s == "" {
		return ""
	}
	if s != "" && (s[:1] == " " || s[len(s)-1:] == " ") {
		return strings.TrimSpace(s)
	}
	return s
}

// TrimStringInclAfterString truncates the given string s after the first
// occurrence of the search string. It returns the truncated string.
func TrimStringInclAfterString(s string, search string) string {
	if idx := IndexI(s, search); idx != -1 {
		return s[:idx]
	}
	return s
}

// Getrootpath returns the root path of the given folder name by splitting on '/' or '\'
// and trimming any trailing slashes. If no slashes, it just trims trailing slashes.
func Getrootpath(foldername string) string {
	if !strings.ContainsRune(foldername, '/') && !strings.ContainsRune(foldername, '\\') {
		return strings.Trim(foldername, "/")
	}
	splitby := '/'
	if !strings.ContainsRune(foldername, '/') {
		splitby = '\\'
	}
	idx := strings.IndexRune(foldername, splitby)
	if idx != -1 {
		foldername = foldername[:idx]
	}
	//foldername = SplitBy(foldername, splitby)
	if foldername != "" && foldername[len(foldername)-1:] == "/" {
		return strings.TrimRight(foldername, "/")
	}

	return foldername
}

// ContainsI checks if string a contains string b, ignoring case.
// It first checks for a direct match with strings.Contains.
// If not found, it does a case-insensitive search by looping through a
// and comparing substrings with EqualFold.
func ContainsI(a string, b string) bool {
	if strings.Contains(a, b) {
		return true
	}
	if len(a) < len(b) {
		return false
	}
	lb := len(b)
	for i := 0; i < len(a)-lb+1; i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			return true
		}
	}
	return false
}

// ContainsInt checks if the string a contains the string representation of
// the integer b. It converts b to a string using strconv.Itoa and calls
// strings.Contains to check for a match.
func ContainsInt(a string, b int) bool {
	return strings.Contains(a, strconv.Itoa(b))
}

// HasPrefixI checks if string s starts with prefix, ignoring case.
// It first checks for a direct match with strings.HasPrefix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from 0 to len(prefix) with prefix using EqualFold.
func HasPrefixI(s, prefix string) bool {
	//if strings.HasPrefix(s, prefix) {
	//	return true
	//}
	return len(s) >= len(prefix) && strings.EqualFold(s[0:len(prefix)], prefix)
}

// HasSuffixI checks if string s ends with suffix, ignoring case.
// It first checks for a direct match with strings.HasSuffix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from len(s)-len(suffix) to len(s) with suffix
// using EqualFold.
func HasSuffixI(s, suffix string) bool {
	//if strings.HasSuffix(s, suffix) {
	//	return true
	//}
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}

// JoinStrings concatenates any number of strings together.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStrings(elems ...string) string {
	if len(elems) == 0 {
		return ""
	}
	if len(elems) == 1 {
		return elems[0]
	}

	n := Getstringarrlength(elems)

	b := PlBuffer.Get()
	b.Grow(n)
	for idx := range elems {
		if elems[idx] != "" {
			b.WriteString(elems[idx])
		}
	}
	defer PlBuffer.Put(b)
	return b.String()
}

// Getstringarrlength returns the total length of all strings in the given
// string slice elems. It iterates through the slice and sums the individual
// string lengths.
func Getstringarrlength(elems []string) int {
	var n int
	for idx := range elems {
		n += len(elems[idx])
	}
	return n
}

// URLJoinPath joins a base URL path with any number of path elements.
// It uses url.JoinPath under the hood and discards any errors.
func URLJoinPath(base string, elem ...string) string {
	str, _ := url.JoinPath(base, elem...)
	return str
}

// IndexI searches for the first case-insensitive instance of b in a.
// It returns the index of the first match, or -1 if no match is found.
func IndexI(a string, b string) int {
	i := strings.Index(a, b)
	if i != -1 {
		return i
	}

	if len(b) > len(a) {
		return -1
	}

	lb := len(b)
	for i := 0; i < len(a)-lb+1; i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			return i
		}
	}

	return -1
}

// IndexILast searches for the last case-insensitive instance of b in a.
// It returns the index of the last match, or -1 if no match is found.
func IndexILast(a string, b string) int {
	lb := len(b)
	for i := len(a) - 1; i >= 0; i-- {
		if strings.EqualFold(a[i:i+lb], b) {
			return i
		}
	}
	return -1
}

// StringToFileMode converts a string representing a file mode in octal to a uint32.
// It returns 0 if the string is empty or cannot be parsed.
func StringToFileMode(s string) fs.FileMode {
	if s == "" {
		return 0
	}
	in, err := strconv.ParseUint(s, 8, 0)
	if err != nil {
		return 0
	}
	return fs.FileMode(uint32(in))
}

// StringToInt converts the given string to an int.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int.
func StringToInt(s string) int {
	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}
		return int(in)
	}
	in, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return in
}

// StringToInt64 converts the given string to an int64.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int64.
func StringToInt64(s string) int64 {
	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}
		return int64(in)
	}
	in, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return int64(in)
}

// TimeGetNow returns the current time in the time zone specified by the
// global timeZone variable.
func TimeGetNow() time.Time {
	return time.Now().In(&timeZone)
}

// UnquoteUnescape unquotes a quoted string and unescapes HTML entities.
// It first tries to unquote the string as a quoted string literal.
// If that succeeds and the unquoted string contains HTML entities,
// it unescapes the HTML entities.
// If unquoting fails, it just unescapes any HTML entities in the original string.
func UnquoteUnescape(s string) string {
	if strings.Contains(s, "\\u") {
		if u, err := strconv.Unquote(s); err == nil {
			s = u
		}
	}
	if strings.ContainsRune(s, '&') {
		s = html.UnescapeString(s)
	}
	return s
}

// SplitByFullP splits a string into two parts by the first
// occurrence of the split rune. It returns the part before the split.
// If the split rune is not found, it returns the original string.
func SplitByFullP(str string, splitby rune) string {
	idx := strings.IndexRune(str, splitby)
	if idx != -1 {
		return str[:idx]
	}
	return str
}

// SplitByLR splits str into left and right substrings by the last occurrence of splitby byte.
// It returns the left substring before the split byte and the right substring after.
// If splitby byte is not found or invalid, an empty string and the original str are returned.
func SplitByLR(str string, splitby byte) (string, string) { // left, right
	idx := strings.LastIndexByte(str, splitby)
	if idx == -1 || idx == 0 || idx == len(str) {
		return "", str
	}
	return str[:idx], str[idx+1:]
}

// SplitByStrMod splits str into two strings by removing splitby from the right side of str.
// It returns the left substring before the removed splitby string.
// func SplitByStrMod(str string, splitby string) string {
// 	idx := IndexI(str, splitby)
// 	if idx != -1 {
// 		str2 := str[:idx]
// 		if str2 == "" {
// 			return ""
// 		}
// 		switch str2[len(str2)-1:] {
// 		case "-", ".", " ":
// 			return strings.TrimRight(str2, "-. ")
// 		}
// 	}
// 	return str
// }

// SplitByStrModRight splits str into two strings by removing splitby from the right side of str.
// It trims any trailing spaces from the right side and returns the right string.
// func SplitByStrModRight(str string, splitby string) string {
// 	idx := IndexI(str, splitby)
// 	if idx != -1 {
// 		str2 := str[idx+len(splitby):]
// 		if str2 == "" {
// 			return ""
// 		}
// 		switch str2[0] {
// 		case '-', '.', ' ':
// 			return strings.TrimLeft(str2, "-. ")
// 		}
// 	}
// 	return str
// }

// Contains reports whether v is present in s - case insensitive.
func SlicesContainsI(s []string, v string) bool {
	for idx := range s {
		if strings.EqualFold(v, s[idx]) {
			return true
		}
	}
	return false
}

// Contains reports whether v is contained in s - case insensitive.
func SlicesContainsPart2I(s []string, v string) bool {
	for idx := range s {
		if ContainsI(v, s[idx]) {
			return true
		}
	}
	return false
}

// Contains reports whether v is contained in s.
func Contains[S ~[]E, E comparable](s S, v E) bool {
	for idx := range s {
		if v == s[idx] {
			return true
		}
	}
	return false
}

// BuilderAddInt writes the string representation of the given int to the given bytes.Buffer.
func BuilderAddInt(bld *bytes.Buffer, i int) {
	bld.WriteString(strconv.Itoa(i))
}

// BuilderAddUint writes the string representation of the unsigned integer i to the buffer bld.
func BuilderAddUint(bld *bytes.Buffer, i uint) {
	bld.WriteString(strconv.Itoa(int(i)))
}

// StringRemoveAllRunes removes all occurrences of the rune r from s.
func StringRemoveAllRunes(s string, r rune) string {
	if !strings.ContainsRune(s, r) {
		return s
	}

	out := PlBuffer.Get()
	out.Grow(len(s))
	for _, z := range s {
		if r != z {
			out.WriteRune(z)
		}
	}
	defer PlBuffer.Put(out)
	return out.String()
}

// StringRemoveAllRunes removes all occurrences of the rune r from s.
func StringRemoveAllRunesMulti(s string, r ...rune) string {
	out := PlBuffer.Get()
	out.Grow(len(s))
contloop:
	for _, z := range s {
		for _, y := range r {
			if y == z {
				continue contloop
			}
		}
		out.WriteRune(z)
	}
	defer PlBuffer.Put(out)
	return out.String()
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func StringReplaceWith(s string, r rune, t rune) string {
	if !strings.ContainsRune(s, r) {
		return s
	}
	buf := PlBuffer.Get()
	buf.Grow(len(s))
	for _, z := range s {
		if z == r {
			buf.WriteRune(t)
		} else {
			buf.WriteRune(z)
		}
	}
	defer PlBuffer.Put(buf)
	return buf.String()
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func StringReplaceWithByte(s string, r rune, t rune) []byte {
	if !strings.ContainsRune(s, r) {
		return []byte(s)
	}
	buf := PlBuffer.Get()
	buf.Grow(len(s))
	for _, z := range s {
		if z == r {
			buf.WriteRune(t)
		} else {
			buf.WriteRune(z)
		}
	}
	defer PlBuffer.Put(buf)
	return buf.Bytes()
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func ByteReplaceWithByte(s []byte, r rune, t rune) []byte {
	if !bytes.ContainsRune(s, r) {
		return []byte(s)
	}
	buf := PlBuffer.Get()
	buf.Grow(len(s))
	for _, z := range s {
		if rune(z) == r {
			buf.WriteRune(t)
		} else {
			buf.WriteByte(z)
		}
	}
	defer PlBuffer.Put(buf)
	return buf.Bytes()
}

// GetStringsMap returns the map of strings for the given type based on
// whether to use the series or movies map. If useseries is true, it returns
// the mapstringsseries map, otherwise it returns the mapstringsmovies map.
func GetStringsMap(useseries bool, typestr string) string {
	if useseries {
		return mapstringsseries[typestr]
	}
	return mapstringsmovies[typestr]
}

// diacriticsReplacer replaces diacritic marks in the input string s
// with their ASCII equivalents, based on the provided diacriticsmap.
// It calls the replacer function to perform the replacements.
func DiacriticsReplacer(s string) string {
	bld := PlBuffer.Get()
	bld.Grow(len(s) + 2)
	for _, z := range s {
		if r, ok := diacriticsmap[z]; ok {
			bld.WriteString(r)
		} else {
			bld.WriteRune(z)
		}
	}
	defer PlBuffer.Put(bld)
	return bld.String()
	//return replacer(diacriticsmap, s)
}

// pathReplacer replaces characters in s that are keys in pathmap with the
// corresponding values. It is used to replace problematic characters in paths.
func pathReplacer(s string) string {
	bld := PlBuffer.Get()
	bld.Grow(len(s) + 2)
	for _, z := range s {
		if r, ok := pathmap[z]; ok {
			bld.WriteString(r)
		} else {
			bld.WriteRune(z)
		}
	}
	defer PlBuffer.Put(bld)
	return bld.String()
	//return replacer(pathmap, s)
}

// replacer replaces characters in s with corresponding strings from mapping m.
// It allocates a new byte buffer to build the replaced string to avoid mutations.
func replacer(m map[rune]string, s string) string {
	bld := PlBuffer.Get()
	bld.Grow(len(s) + 2)
	for _, z := range s {
		if r, ok := m[z]; ok {
			bld.WriteString(r)
		} else {
			bld.WriteRune(z)
		}
	}
	defer PlBuffer.Put(bld)
	return bld.String()
}
