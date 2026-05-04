package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"path"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/mozillazg/go-unidecode/table"
	"github.com/rs/zerolog"
)

type AddBuffer struct {
	bytes.Buffer
}

const (
	StatusAll     = "all"
	StatusSuccess = "success"
	StatusInfo    = "info"
	StatusWarning = "warning"
	StatusError   = "error"
	StatusDanger  = "danger"
	StatusDebug   = "debug"
	StatusFatal   = "fatal"
	StatusPanic   = "panic"
)

const (
	ParseFailedIDs             = "parse failed ids"
	FilterByID                 = "id = ?"
	StrRefreshMovies           = "Refresh Movies"
	StrRefreshMoviesInc        = "Refresh Movies Incremental"
	StrRefreshSeries           = "Refresh Series"
	StrRefreshSeriesInc        = "Refresh Series Incremental"
	StrRefreshAudiobooks       = "Refresh Audiobooks"
	StrRefreshAudiobooksInc    = "Refresh Audiobooks Incremental"
	StrRefreshMusic            = "Refresh Music"
	StrRefreshMusicInc         = "Refresh Music Incremental"
	StrDebug                   = "debug"
	StrDate                    = "date"
	StrSearchMissingInc        = "searchmissinginc"
	StrSearchMissingFull       = "searchmissingfull"
	StrSearchMissingIncTitle   = "searchmissinginctitle"
	StrSearchMissingFullTitle  = "searchmissingfulltitle"
	StrSearchUpgradeInc        = "searchupgradeinc"
	StrSearchUpgradeFull       = "searchupgradefull"
	StrSearchUpgradeIncTitle   = "searchupgradeinctitle"
	StrSearchUpgradeFullTitle  = "searchupgradefulltitle"
	StrCheckMissing            = "checkmissing"
	StrCheckMissingFlag        = "checkmissingflag"
	StrUpgradeFlag             = "checkupgradeflag"
	StrReachedFlag             = "checkreachedflag"
	StrClearHistory            = "clearhistory"
	StrRssSeasonsAll           = "rssseasonsall"
	StrSerie                   = "serie"
	StrRssSeasons              = "rssseasons"
	StrRssArtists              = "rssartists"
	StrRssArtistsUpgrade       = "rssartistsupgrade"
	StrRssAuthors              = "rssauthors"
	StrRssAuthorsUpgrade       = "rssauthorsupgrade"
	StrRss                     = "rss"
	Underscore                 = "_"
	StrTt                      = "tt"
	CacheDBMedia               = "CacheDBMedia"
	DBCountDBMedia             = "DBCountDBMedia"
	DBCacheDBMedia             = "DBCacheDBMedia"
	CacheMedia                 = "CacheMedia"
	DBCountMedia               = "DBCountMedia"
	DBCacheMedia               = "DBCacheMedia"
	CacheRootpath              = "CacheRootpath"
	DBCountRootpath            = "DBCountRootpath"
	DBCacheRootpath            = "DBCacheRootpath"
	CacheHistoryTitle          = "CacheHistoryTitle"
	CacheHistoryURL            = "CacheHistoryUrl"
	DBHistoriesURL             = "DBHistoriesUrl"
	DBHistoriesTitle           = "DBHistoriesTitle"
	DBCountHistoriesURL        = "DBCountHistoriesUrl"
	DBCountHistoriesTitle      = "DBCountHistoriesTitle"
	CacheMediaTitles           = "CacheMediaTitles"
	DBCountDBTitles            = "DBCountDBTitles"
	DBCacheDBTitles            = "DBCacheDBTitles"
	CacheFiles                 = "CacheFiles"
	DBCountFiles               = "DBCountFiles"
	DBCacheFiles               = "DBCacheFiles"
	CacheUnmatched             = "CacheUnmatched"
	DBCountUnmatched           = "DBCountUnmatched"
	DBCacheUnmatched           = "DBCacheUnmatched"
	DBCountFilesLocation       = "DBCountFilesLocation"
	DBCountUnmatchedPath       = "DBCountUnmatchedPath"
	DBCountDBTitlesDBID        = "DBCountDBTitlesDBID"
	DBDistinctDBTitlesDBID     = "DBDistinctDBTitlesDBID"
	DBMediaTitlesID            = "DBMediaTitlesID"
	DBFilesQuality             = "DBFilesQuality"
	DBCountFilesByList         = "DBCountFilesByList"
	DBLocationFilesByList      = "DBLocationFilesByList"
	DBIDsFilesByLocation       = "DBIDsFilesByLocation"
	DBCountFilesByMediaID      = "DBCountFilesByMediaID"
	DBCountFilesByLocation     = "DBCountFilesByLocation"
	TableFiles                 = "TableFiles"
	TableMedia                 = "TableMedia"
	DBCountMediaByList         = "DBCountMediaByList"
	DBIDMissingMediaByList     = "DBIDMissingMediaByList"
	DBUpdateMissing            = "DBUpdateMissing"
	DBListnameByMediaID        = "DBListnameByMediaID"
	DBRootPathFromMediaID      = "DBRootPathFromMediaID"
	DBDeleteFileByIDLocation   = "DBDeleteFileByIDLocation"
	DBCountHistoriesByTitle    = "DBCountHistoriesByTitle"
	DBCountHistoriesByURL      = "DBCountHistoriesByUrl"
	DBLocationIDFilesByID      = "DBLocationIDFilesByID"
	DBFilePrioFilesByID        = "DBFilePrioFilesByID"
	DBAudioFilePrioFilesByID   = "DBAudioFilePrioFilesByID"
	UpdateMediaLastscan        = "UpdateMediaLastscan"
	DBQualityMediaByID         = "DBQualityMediaByID"
	SearchGenSelect            = "SearchGenSelect"
	SearchGenTable             = "SearchGenTable"
	SearchGenMissing           = "SearchGenMissing"
	SearchGenMissingEnd        = "SearchGenMissingEnd"
	SearchGenReached           = "SearchGenReached"
	SearchGenLastScan          = "SearchGenLastScan"
	SearchGenDate              = "SearchGenDate"
	SearchGenOrder             = "SearchGenOrder"
	CacheMovie                 = "CacheMovie"
	CacheSeries                = "CacheSeries"
	CacheDBMovie               = "CacheDBMovie"
	CacheDBSeries              = "CacheDBSeries"
	CacheDBSeriesAlt           = "CacheDBSeriesAlt"
	CacheTitlesMovie           = "CacheTitlesMovie"
	CacheUnmatchedMovie        = "CacheUnmatchedMovie"
	CacheUnmatchedSeries       = "CacheUnmatchedSeries"
	CacheFilesMovie            = "CacheFilesMovie"
	CacheFilesSeries           = "CacheFilesSeries"
	CacheHistoryURLMovie       = "CacheHistoryUrlMovie"
	CacheHistoryTitleMovie     = "CacheHistoryTitleMovie"
	CacheHistoryURLSeries      = "CacheHistoryUrlSeries"
	CacheHistoryTitleSeries    = "CacheHistoryTitleSeries"
	CacheBook                  = "CacheBook"
	CacheDBBook                = "CacheDBBook"
	CacheTitlesBook            = "CacheTitlesBook"
	CacheUnmatchedBook         = "CacheUnmatchedBook"
	CacheFilesBook             = "CacheFilesBook"
	CacheHistoryURLBook        = "CacheHistoryUrlBook"
	CacheHistoryTitleBook      = "CacheHistoryTitleBook"
	CacheAudiobook             = "CacheAudiobook"
	CacheDBAudiobook           = "CacheDBAudiobook"
	CacheTitlesAudiobook       = "CacheTitlesAudiobook"
	CacheUnmatchedAudiobook    = "CacheUnmatchedAudiobook"
	CacheFilesAudiobook        = "CacheFilesAudiobook"
	CacheHistoryURLAudiobook   = "CacheHistoryUrlAudiobook"
	CacheHistoryTitleAudiobook = "CacheHistoryTitleAudiobook"
	CacheAlbum                 = "CacheAlbum"
	CacheDBAlbum               = "CacheDBAlbum"
	CacheTitlesAlbum           = "CacheTitlesAlbum"
	CacheUnmatchedAlbum        = "CacheUnmatchedAlbum"
	CacheFilesAlbum            = "CacheFilesAlbum"
	CacheHistoryURLAlbum       = "CacheHistoryUrlAlbum"
	CacheHistoryTitleAlbum     = "CacheHistoryTitleAlbum"
	CacheRootpathAlbum         = "CacheRootpathAlbum"
	CacheRootpathAudiobook     = "CacheRootpathAudiobook"
	DBIDUnmatchedPathList      = "DBIDUnmatchedPathList"
	DBMovieDetails             = "select id,created_at,updated_at,title,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?"

	Strstructure = "structure"
	StrStatus    = "status"
	StrRow       = "row"
	StrDot       = "."
	StrDash      = "-"
	StrSpace     = " "
	StrIndexer   = "indexer"
	StrSize      = "Size"
)

type Arrany struct {
	Arr []any
}

// global vars.
var (
	StrFeeds        = "feeds"
	StrDataFull     = "datafull"
	StrStructure    = "structure"
	V0              = 0
	StrMovie        = "movie"
	StrSeries       = "series"
	StrBook         = "book"
	StrAudiobook    = "audiobook"
	StrAlbum        = "album"
	StrID           = "id"
	StrWaitfor      = "waitfor"
	StrURL          = "Url"
	StrImdb         = "imdb"
	StrFound        = "found"
	StrWanted       = "wanted"
	StrTitle        = "Title"
	StrAccepted     = "accepted"
	StrDenied       = "denied"
	StrJob          = "Job"
	StrPath         = "Path"
	StrFile         = "File"
	StrListname     = "Listname"
	StrData         = "data"
	StrTvdb         = "tvdb"
	StrSeason       = "season"
	StrConfig       = "config"
	StrReason       = "reason"
	StrPriority     = "Priority"
	StrMinPrio      = "minimum prio"
	StrQuality      = "Quality"
	ArrHTMLEntities = []string{
		"&AElig",
		"&AMP",
		"&Aacute",
		"&Acirc",
		"&Agrave",
		"&Aring",
		"&Atilde",
		"&Auml",
		"&COPY",
		"&Ccedil",
		"&ETH",
		"&Eacute",
		"&Ecirc",
		"&Egrave",
		"&Euml",
		"&GT",
		"&Iacute",
		"&Icirc",
		"&Igrave",
		"&Iuml",
		"&LT",
		"&Ntilde",
		"&Oacute",
		"&Ocirc",
		"&Ograve",
		"&Oslash",
		"&Otilde",
		"&Ouml",
		"&QUOT",
		"&REG",
		"&THORN",
		"&Uacute",
		"&Ucirc",
		"&Ugrave",
		"&Uuml",
		"&Yacute",
		"&aacute",
		"&acirc",
		"&acute",
		"&aelig",
		"&agrave",
		"&amp",
		"&aring",
		"&atilde",
		"&auml",
		"&brvbar",
		"&ccedil",
		"&cedil",
		"&cent",
		"&copy",
		"&curren",
		"&deg",
		"&divide",
		"&eacute",
		"&ecirc",
		"&egrave",
		"&eth",
		"&euml",
		"&gt",
		"&iacute",
		"&icirc",
		"&iexcl",
		"&igrave",
		"&iquest",
		"&iuml",
		"&laquo",
		"&lt",
		"&macr",
		"&micro",
		"&middot",
		"&nbsp",
		"&not",
		"&ntilde",
		"&oacute",
		"&ocirc",
		"&ograve",
		"&ordf",
		"&ordm",
		"&oslash",
		"&otilde",
		"&ouml",
		"&para",
		"&plusmn",
		"&pound",
		"&quot",
		"&raquo",
		"&reg",
		"&sect",
		"&shy",
		"&szlig",
		"&thorn",
		"&times",
		"&uacute",
		"&ucirc",
		"&ugrave",
		"&uml",
		"&uuml",
		"&yacute",
		"&yen",
		"&yuml",
	}
	ErrNoID                   = errors.New("no id")
	ErrNotFound               = errors.New("not found")
	ErrNotAllowed             = errors.New("not allowed")
	ErrDisabled               = errors.New("disabled")
	ErrToWait                 = errors.New("please wait")
	ErrContextCanceled        = errors.New("context canceled")
	Errnoresults              = errors.New("no results")
	ErrNotFoundDbmovie        = errors.New("dbmovie not found")
	ErrNotFoundMovie          = errors.New("movie not found")
	ErrNotFoundDbserie        = errors.New("dbserie not found")
	ErrNotFoundSerie          = errors.New("serie not found")
	ErrNotFoundDbserieEpisode = errors.New("dbserie episode not found")
	ErrCfgpNotFound           = errors.New("cfgpstr not found")
	ErrNotFoundEpisode        = errors.New("episode not found")
	ErrNotFoundDbbook         = errors.New("dbbook not found")
	ErrNotFoundBook           = errors.New("book not found")
	ErrNotFoundDbaudiobook    = errors.New("dbaudiobook not found")
	ErrNotFoundAudiobook      = errors.New("audiobook not found")
	ErrNotFoundDbalbum        = errors.New("dbalbum not found")
	ErrNotFoundAlbum          = errors.New("album not found")
	ErrListnameEmpty          = errors.New("listname empty")
	ErrListnameTemplateEmpty  = errors.New("listname template empty")
	ErrTvdbEmpty              = errors.New("tvdb empty")
	ErrImdbEmpty              = errors.New("imdb empty")
	ErrTracksEmpty            = errors.New("tracks empty")
	ErrYearEmpty              = errors.New("year empty")
	PlAddBuffer               pool.Poolobj[AddBuffer]
	PLArrAny                  pool.Poolobj[Arrany]
)

// local vars.
var (
	timeFormat = time.RFC3339Nano
	log        zerolog.Logger
	timeZone   = *time.UTC
	// textparser is a template engine instance used for parsing and rendering text templates.
	textparser = template.New("master").Funcs(template.FuncMap{
		"firstLetter": firstLetter,
		"pad":         pad,
		"pad3":        pad3,
		"discTrack":   discTrack,
		"se":          se,
		"seNoPad":     seNoPad,
		"xe":          xe,
		"seMulti":     seMulti,
		"titleThe":    titleThe,
	})
	poolsOnce sync.Once
	// subRuneSet is a pre-computed boolean array that efficiently checks if a rune is an allowed character
	// for filename or path generation, including lowercase letters, numbers, and hyphen.
	subRuneSet = [256]bool{
		'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true, 'h': true,
		'i': true, 'j': true, 'k': true, 'l': true, 'm': true, 'n': true, 'o': true, 'p': true,
		'q': true, 'r': true, 's': true, 't': true, 'u': true, 'v': true, 'w': true, 'x': true,
		'y': true, 'z': true, '0': true, '1': true, '2': true, '3': true, '4': true, '5': true,
		'6': true, '7': true, '8': true, '9': true, '-': true,
	}
	// substituteRuneSpace is a mapping of special characters to their replacement strings.
	// It handles character substitutions for filename or path sanitization, including:
	// - Removing or replacing punctuation and whitespace
	// - Converting diacritical characters to their base ASCII equivalents
	// - Replacing special symbols with readable text.
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
	// diacriticsmap is a mapping of diacritical characters to their ASCII equivalent replacements.
	// It provides a standardized way to convert special characters with diacritical marks
	// to their base letter representations, useful for text normalization and sanitization.
	diacriticsmap = map[rune]string{
		'ä': "ae",
		'ö': "oe",
		'ü': "ue",
		'Ä': "Ae",
		'Ö': "Oe",
		'Ü': "Ue",
		'ß': "ss",
	}
	// diacriticslowermap is a mapping of diacritical characters to their lowercase equivalents.
	// It provides a standardized way to convert special characters with diacritical marks
	// to their lowercase representations, useful for text normalization and case-insensitive comparisons.
	diacriticslowermap = map[rune]rune{
		'ä': 'ä',
		'ö': 'ö',
		'ü': 'ü',
		'Ä': 'ä',
		'Ö': 'ö',
		'Ü': 'ü',
		'ß': 'ß',
	}
	// pathmap is a set of special characters that are considered invalid or problematic in file paths.
	// It contains characters that typically need to be sanitized or escaped when working with file system paths.
	// These characters include reserved symbols like colons, asterisks, question marks, backslashes, angle brackets, and pipe symbols.
	pathmap = map[rune]struct{}{
		':':  {},
		'*':  {},
		'?':  {},
		'\\': {},
		'<':  {},
		'>':  {},
		'|':  {},
	}
)

// initializePools initializes the object pools safely with sync.Once.
func initializePools() {
	poolsOnce.Do(func() {
		PlAddBuffer.Init(200, 10, func(b *AddBuffer) {
			if b.Cap() < 900 {
				b.Grow(900)
			}

			if b.Len() > 1 {
				b.Reset()
			}
		}, func(b *AddBuffer) bool {
			b.Reset()
			return false
		})
		PLArrAny.Init(200, 5, func(a *Arrany) { a.Arr = make([]any, 0, 20) }, func(a *Arrany) bool {
			clear(a.Arr)

			if cap(a.Arr) > 200 {
				return true
			}

			a.Arr = a.Arr[:0]

			return false
		})
	})
}

// IntToString converts any numeric type to a string.
// It handles all integer types, including signed and unsigned.
func IntToString(a uint16) string {
	return strconv.Itoa(int(a))
}

// WriteInt writes the string representation of the given integer i to the buffer.
func (b *AddBuffer) WriteInt(i int) {
	b.WriteString(strconv.Itoa(i))
}

// WriteUInt16 writes the given uint16 value to the buffer as a string.
func (b *AddBuffer) WriteUInt16(i uint16) {
	b.WriteInt(int(i))
}

// WriteUInt writes the string representation of the given unsigned integer to the buffer.
func (b *AddBuffer) WriteUInt(i uint) {
	b.WriteInt(int(i)) //nolint:gosec // safe: value within target type range
}

// WriteStringMap writes a string to the buffer based on the provided boolean and string parameters.
// If isType is true, it writes the value from the Mapstringsseries map using the typestr key.
// If isType is false, it writes the value from the Mapstringsmovies map using the typestr key.
func (b *AddBuffer) WriteStringMap(isType uint, typestr string) {
	b.WriteString(mtstrings.GetStringsMap(isType, typestr))
}

// WriteURL writes the given string to the buffer after escaping it for use in a URL.
func (b *AddBuffer) WriteURL(s string) {
	b.WriteString(url.QueryEscape(s))
}

// hexUpperTable maps nibble → uppercase hex character.
const hexUpperTable = "0123456789ABCDEF"

// WriteHex2 writes v as exactly 2 uppercase hex digits (zero-padded).
// Equivalent to fmt.Fprintf(w, "%02X", v) but allocation-free.
func (b *AddBuffer) WriteHex2(v byte) {
	b.WriteByte(hexUpperTable[v>>4])
	b.WriteByte(hexUpperTable[v&0xF])
}

// WriteHex8 writes v as exactly 8 uppercase hex digits (zero-padded).
// Equivalent to fmt.Fprintf(w, "%08X", v) but allocation-free.
func (b *AddBuffer) WriteHex8(v uint32) {
	b.WriteByte(hexUpperTable[(v>>28)&0xF])
	b.WriteByte(hexUpperTable[(v>>24)&0xF])
	b.WriteByte(hexUpperTable[(v>>20)&0xF])
	b.WriteByte(hexUpperTable[(v>>16)&0xF])
	b.WriteByte(hexUpperTable[(v>>12)&0xF])
	b.WriteByte(hexUpperTable[(v>>8)&0xF])
	b.WriteByte(hexUpperTable[(v>>4)&0xF])
	b.WriteByte(hexUpperTable[v&0xF])
}

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

func firstLetter(s string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(s))
	return strings.ToUpper(string(r))
}

// pad zero-pads an integer to 2 digits: pad 1 → "01", pad 12 → "12".
func pad(n int) string {
	return fmt.Sprintf("%02d", n)
}

// pad3 zero-pads an integer to 3 digits: pad3 1 → "001".
func pad3(n int) string {
	return fmt.Sprintf("%03d", n)
}

// discTrack formats disc-track as "D-TT": discTrack 2 5 → "2-05".
func discTrack(disc, track int) string {
	return fmt.Sprintf("%d-%02d", disc, track)
}

// se formats season/episode as S01E01. Season is a string, episode is an int.
func se(season string, episode int) string {
	s, _ := strconv.Atoi(season)
	return fmt.Sprintf("S%02dE%02d", s, episode)
}

// seNoPad formats season/episode without zero-padding: S1E1.
func seNoPad(season string, episode int) string {
	return fmt.Sprintf("S%sE%d", season, episode)
}

// xe formats season/episode as 1x01 (season unpadded, episode padded).
func xe(season string, episode int) string {
	return fmt.Sprintf("%sx%02d", season, episode)
}

// seMulti formats a season with multiple episodes as S01E01-E03.
// Episodes slice must be sorted.
func seMulti(season string, episodes []int) string {
	s, _ := strconv.Atoi(season)
	if len(episodes) == 0 {
		return fmt.Sprintf("S%02d", s)
	}

	if len(episodes) == 1 {
		return fmt.Sprintf("S%02dE%02d", s, episodes[0])
	}

	var b strings.Builder
	fmt.Fprintf(&b, "S%02d", s)

	for _, ep := range episodes {
		fmt.Fprintf(&b, "E%02d", ep)
	}

	return b.String()
}

// titleThe moves leading articles ("The ", "A ", "An ") to the end:
// "The Hobbit" → "Hobbit, The", "A Song" → "Song, A".
func titleThe(s string) string {
	s = strings.TrimSpace(s)
	for _, article := range []string{"The ", "A ", "An "} {
		if strings.HasPrefix(s, article) {
			return s[len(article):] + ", " + strings.TrimSpace(article)
		}
	}

	return s
}

// ParseStringTemplate parses a text/template string into a template.Template, caches it, and executes it with the given data.
// It returns whether an error occurred, the executed template string, and any error encountered.
func ParseStringTemplate(message string, messagedata any) (bool, string, error) {
	if message == "" {
		return false, "", nil
	}

	tmplmessage := textparser.Lookup(message)
	if tmplmessage == nil {
		var err error

		tmplmessage, err = textparser.New(message).Parse(message)
		if err != nil {
			Logtype("error", 1).Err(err).Msg("template")
			return true, "", err
		}
	}

	initializePools()

	doc := PlAddBuffer.Get()
	defer PlAddBuffer.Put(doc)

	if err := tmplmessage.Execute(doc, messagedata); err != nil {
		Logtype("error", 1).Err(err).Msg("template")
		return true, "", err
	}

	return false, doc.String(), nil
}

func BytesToString(b []byte) string {
	initializePools()

	bld := PlAddBuffer.Get()
	defer PlAddBuffer.Put(bld)

	bld.Write(b)

	return bld.String()
}

// StringToSlug converts the given string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlug(instr string) string {
	initializePools()

	ret := PlAddBuffer.Get()
	defer PlAddBuffer.Put(ret)

	stringToSlugBuffer(ret, Checkhtmlentities(instr))

	return string(bytes.Trim(ret.Bytes(), "- "))
}

// Checkhtmlentities checks if the input string contains HTML entities, and if so,
// unescapes the entities using html.UnescapeString. If the input string does not
// contain any HTML entities, it is returned as-is.
func Checkhtmlentities(instr string) string {
	if !strings.ContainsRune(instr, '&') {
		return instr
	}

	if strings.ContainsRune(instr, ';') {
		return html.UnescapeString(instr)
	}

	for idx := range ArrHTMLEntities {
		if strings.Contains(instr, ArrHTMLEntities[idx]) {
			return html.UnescapeString(instr)
		}
	}

	return instr
}

// StringToSlugWild converts the given string to a %slug% wildcard in one allocation.
// The caller can derive the plain slug as a free subslice: wild[1:len(wild)-1].
func StringToSlugWild(instr string) string {
	ret := PlAddBuffer.Get()
	defer PlAddBuffer.Put(ret)

	stringToSlugBuffer(ret, Checkhtmlentities(instr))

	trimmed := bytes.Trim(ret.Bytes(), "- ")

	var sb strings.Builder
	sb.Grow(len(trimmed) + 2)
	sb.WriteByte('%')
	sb.Write(trimmed)
	sb.WriteByte('%')

	return sb.String()
}

// StringToSlugBytes converts the given string to a slug byte slice by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlugBytes(instr string) []byte {
	initializePools()

	ret := PlAddBuffer.Get()
	defer PlAddBuffer.Put(ret)

	stringToSlugBuffer(ret, Checkhtmlentities(instr))
	// Create a copy to avoid referencing pool memory after Put()
	trimmed := bytes.Trim(ret.Bytes(), "- ")
	result := make([]byte, len(trimmed))
	copy(result, trimmed)

	return result
}

// stringToSlugBuffer converts the given input string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens. The result
// is written to the provided bytes.Buffer.
func stringToSlugBuffer(ret *AddBuffer, instr string) {
	if len(instr) == 0 {
		return
	}

	var (
		lastrune, section, position rune
		laststr                     string
	)
	for _, r := range instr {
		if val, ok := substituteRuneSpace[r]; ok {
			if (laststr == "" || val != laststr) && (lastrune != '-' || val != StrDash) {
				ret.WriteString(val)

				laststr = val

				if val == StrDash {
					lastrune = '-'
				} else {
					lastrune = ' '
				}
			}

			continue
		}

		if laststr != "" {
			laststr = ""
		}

		switch {
		case r < unicode.MaxASCII:
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}

			// if _, ok = subRune[r]; ok {
			if r < 256 && subRuneSet[r] {
				if lastrune != '-' || r != '-' {
					ret.WriteRune(r)

					lastrune = r
				}
			} else if lastrune != '-' {
				ret.WriteByte('-')

				lastrune = '-'
			}

		case r <= 0xeffff:
			section = r >> 8

			position = r % 256

			tb, ok := table.Tables[section]
			if !ok || len(tb) <= int(position) {
				break
			}

			if len(tb[position]) >= 1 && tb[position][0] > unicode.MaxASCII && lastrune != '-' {
				ret.WriteByte('-')

				lastrune = '-'
			} else if lastrune != '-' || tb[position] != StrDash {
				ret.WriteString(tb[position])
			}
		}
	}
}

// AddImdbPrefix adds the "tt" prefix to the given string if it doesn't already have the prefix.
// If the string is nil or has a length less than 1, this function does nothing.
func AddImdbPrefix(str string) string {
	if len(str) >= 1 && !HasPrefixI(str, StrTt) {
		return JoinStrings(StrTt, str)
	}

	return str
}

// Path sanitizes the given string by cleaning it with path.Clean, unquoting
// and unescaping it, optionally removing slashes, and replacing invalid
// characters. It returns the cleaned path string.
func Path(s *string, allowslash bool) {
	if s == nil || *s == "" {
		return
	}

	// Read once to avoid concurrent modification issues
	original := *s

	newpath := path.Clean(UnquoteUnescape(original))
	if !allowslash {
		StringRemoveAllRunesP(&newpath, '\\', '/')
	}

	initializePools()

	bld := PlAddBuffer.Get()
	defer PlAddBuffer.Put(bld)

	bl := newpath != original
	for _, z := range newpath {
		if r, ok := diacriticsmap[z]; ok {
			bld.WriteString(r)

			bl = true
		} else if _, ok := pathmap[z]; !ok {
			bld.WriteRune(z)
		} else {
			bl = true
		}
	}

	if bl {
		*s = TrimSpace(bld.String())
	}
}

// TrimSpace returns a slice of the string s, with all leading and trailing
// Unicode code points that are considered whitespace removed.
// If s is empty, TrimSpace returns s.
func TrimSpace(s string) string {
	if len(s) == 0 {
		return s
	}

	return Trim(s, ' ')
}

// Trim returns a slice of the string s, with all leading and trailing Unicode code points contained in cutset removed.
// If s is empty or cutset is empty, Trim returns s.
func Trim(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}

	i := getfirstinstring(s, cutset)

	j := getlastinstring(s, cutset)
	if i == -1 && j == -1 {
		return s
	}

	if i == -1 {
		return s[:j]
	}

	if j == -1 {
		return s[i:]
	}

	return s[i:j]
}

// TrimLeft returns a slice of the string s, with all leading Unicode code points contained in cutset removed.
// If s is empty or cutset is empty, TrimLeft returns s.
func TrimLeft(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}

	if i := getfirstinstring(s, cutset); i != -1 {
		return s[i:]
	}

	return s
}

// getfirstinstring returns the index of the first character in the string s that is not in the cutset.
// If no such character is found, it returns -1.
func getfirstinstring(s string, cutset []rune) int {
	runeIdx := 0
	byteIdx := 0

	for _, y := range s {
		found := slices.Contains(cutset, y)

		if !found {
			if runeIdx == 0 {
				return -1
			}

			return byteIdx
		}

		runeIdx++

		byteIdx += len(string(y))
	}

	return -1
}

// func getfirstinstring(s string, cutset []rune) int {
// 	runeIdx := 0
// 	byteIdx := 0
// 	for _, y := range s {
// 		found := false
// 		for _, z := range cutset {
// 			if y == z {
// 				found = true
// 				break
// 			}
// 		}

// 		if !found {
// 			if runeIdx == 0 {
// 				return -1
// 			}
// 			return byteIdx
// 		}
// 		runeIdx++
// 		byteIdx += len(string(y))
// 	}
// 	return -1
// }

// getlastinstring returns the index of the last character in the string s that is not in the cutset.
// If no such character is found, it returns -1.
func getlastinstring(s string, cutset []rune) int {
	for idx := len(s) - 1; idx >= 0; idx-- {
		found := false

		var x rune

		for idx2, y := range s {
			if idx2 == idx {
				x = y
				break
			}
		}

		if slices.Contains(cutset, x) {
			found = true
		}

		if !found && idx == 0 {
			return -1
		}

		if !found && idx > 0 {
			return idx + 1
		}
	}

	return -1
}

// func getlastinstring(s string, cutset []rune) int {
// 	runes := []rune(s)
// 	for idx := len(runes) - 1; idx >= 0; idx-- {
// 		found := slices.Contains(cutset, runes[idx])
// 		if !found && idx == 0 {
// 			return -1
// 		}
// 		if !found && idx > 0 {
// 			// Convert rune index back to byte index
// 			return len(string(runes[:idx+1]))
// 		}
// 	}
// 	return -1
// }

// TrimRight returns a slice of the string s, with all trailing
// Unicode code points contained in cutset removed.
func TrimRight(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}

	if i := getlastinstring(s, cutset); i != -1 {
		return s[:i]
	}

	return s
}

// ContainsI checks if string a contains string b, ignoring case.
// It first checks for a direct match with strings.Contains.
// If not found, it does a case-insensitive search by looping through a
// and comparing substrings with EqualFold.
func ContainsI(a, b string) bool {
	return IndexI(a, b) != -1
}

// ContainsByteI checks if the byte slice a contains the byte slice b, ignoring case.
// It first checks for a direct match using bytes.Contains.
// If not found, it does a case-insensitive search by converting both a and b to lowercase
// and then checking if the lowercase a contains the lowercase b.
func ContainsByteI(a, b []byte) bool {
	if bytes.Contains(a, b) {
		return true
	}

	if len(a) < len(b) {
		return false
	}

	return bytes.Contains(bytes.ToLower(a), bytes.ToLower(b))
}

// ContainsInt checks if the string a contains the string representation of
// the integer b. It converts b to a string using strconv.Itoa and calls
// strings.Contains to check for a match.
func ContainsInt(a string, b uint16) bool {
	return strings.Contains(a, strconv.Itoa(int(b)))
}

// HasPrefixI checks if string s starts with prefix, ignoring case.
// It first checks for a direct match with strings.HasPrefix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from 0 to len(prefix) with prefix using EqualFold.
func HasPrefixI(s, prefix string) bool {
	return len(s) >= len(prefix) &&
		(s[0:len(prefix)] == prefix || strings.EqualFold(s[0:len(prefix)], prefix))
}

// TimeAfter returns true if the time a is after the time b.
// If the Unix timestamps of a and b are equal, it compares the
// nanosecond parts of the times to determine the order.
func TimeAfter(a, b time.Time) bool {
	as := a.Unix()

	bs := b.Unix()
	if as == bs {
		return a.UnixNano() > b.UnixNano()
	}

	return as > bs
}

// HasSuffixI checks if string s ends with suffix, ignoring case.
// It first checks for a direct match with strings.HasSuffix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from len(s)-len(suffix) to len(s) with suffix
// using EqualFold.
func HasSuffixI(s, suffix string) bool {
	return len(s) >= len(suffix) &&
		(s[len(s)-len(suffix):] == suffix || strings.EqualFold(s[len(s)-len(suffix):], suffix))
}

// ExtToFormat converts a file extension to a lowercase format string without
// the leading dot, e.g. ".MP3" → "mp3", "FLAC" → "flac".
func ExtToFormat(ext string) string {
	return strings.ToLower(strings.TrimPrefix(ext, "."))
}

// TrimPrefixI returns s with prefix removed using a case-insensitive match,
// preserving the original case of the remaining string.
// If s does not start with prefix, s is returned unchanged.
func TrimPrefixI(s, prefix string) string {
	if HasPrefixI(s, prefix) {
		return s[len(prefix):]
	}

	return s
}

// TrimSuffixI returns s with suffix removed using a case-insensitive match,
// preserving the original case of the remaining string.
// If s does not end with suffix, s is returned unchanged.
func TrimSuffixI(s, suffix string) string {
	if HasSuffixI(s, suffix) {
		return s[:len(s)-len(suffix)]
	}

	return s
}

// JoinStrings concatenates any number of strings together.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStrings(elems ...string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	case 2:
		if elems[0] == "" {
			return elems[1]
		}

		if elems[1] == "" {
			return elems[0]
		}
	}

	initializePools()

	b := PlAddBuffer.Get()
	defer PlAddBuffer.Put(b)

	for idx := range elems {
		if elems[idx] != "" {
			b.WriteString(elems[idx])
		}
	}

	return b.String()
}

// JoinStringsSep concatenates the elements of the provided slice of strings
// into a single string, separated by the provided separator string.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStringsSep(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}

	initializePools()

	b := PlAddBuffer.Get()
	defer PlAddBuffer.Put(b)

	l := len(elems)
	for idx, val := range elems {
		if val == "" {
			continue
		}

		b.WriteString(val)

		if idx < l-1 {
			b.WriteString(sep)
		}
	}

	return b.String()
}

// IndexI searches for the first case-insensitive instance of b in a.
// It returns the index of the first match, or -1 if no match is found.
func IndexI(a, b string) int {
	if i := strings.Index(a, b); i != -1 {
		return i
	}

	if len(b) > len(a) {
		return -1
	}

	hasUppera, hasUpperb := false, false
	isASCIIb := true

	for _, c := range a {
		if c >= utf8.RuneSelf {
			if _, ok := diacriticslowermap[c]; !ok {
				// isASCIIa = false
				break
			}
		}

		hasUppera = hasUppera || ('A' <= c && c <= 'Z') || c == 'Ö' || c == 'Ü' || c == 'Ä'
	}

	for _, c := range b {
		if c >= utf8.RuneSelf {
			_, ok := diacriticslowermap[c]
			if !ok {
				isASCIIb = false
				break
			}
		}

		hasUpperb = hasUpperb || ('A' <= c && c <= 'Z') || c == 'Ö' || c == 'Ü' || c == 'Ä'
	}

	if !isASCIIb {
		return strings.Index(strings.Map(unicode.ToLower, a), strings.Map(unicode.ToLower, b))
	}

	if !hasUppera && !hasUpperb {
		return strings.Index(a, b)
	}

	initializePools()

	bufa := PlAddBuffer.Get()
	defer PlAddBuffer.Put(bufa)

	for _, c := range a {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		} else if c >= utf8.RuneSelf {
			d, ok := diacriticslowermap[c]
			if ok {
				c = d
			}
		}

		bufa.WriteRune(c)
	}

	bufb := PlAddBuffer.Get()
	defer PlAddBuffer.Put(bufb)

	for _, c := range b {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		} else if c >= utf8.RuneSelf {
			d, ok := diacriticslowermap[c]
			if ok {
				c = d
			}
		}

		bufb.WriteRune(c)
	}

	return bytes.Index(bufa.Bytes(), bufb.Bytes())
}

// Int64ToUint converts an int64 to a uint, returning 0 if the input is negative.
func Int64ToUint(in int64) uint {
	if in < 0 {
		return 0
	}

	return uint(in)
}

// IntToUint converts an int to a uint, returning 0 if the input is negative.
func IntToUint(in int) uint {
	if in < 0 {
		return 0
	}

	return uint(in)
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

	return fs.FileMode(uint32(in)) //nolint:gosec // safe: value within target type range
}

// StringToInt converts the given string to an int.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int.
func StringToInt(s string) int {
	if s == "" || s == "0" {
		return 0
	}

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

// StringToDuration converts the given string to a time.Duration.
// It first tries to parse the string as a float and then cast it to time.Duration.
// If that fails, it tries to parse the string directly as an int and then cast it to time.Duration.
// If both attempts fail, it returns 0.
func StringToDuration(s string) int {
	if s == "" || s == "0" {
		return 0
	}

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

// StringToInt32 converts the given string to an int32.
// It first tries to parse the string as a float and then cast it to int32.
// If that fails, it tries to parse the string directly as an int32.
// If both attempts fail, it returns 0.
func StringToInt32(s string) int32 {
	if s == "" || s == "0" {
		return 0
	}

	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}

		return int32(in)
	}

	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}

	return int32(i)
}

// StringToUInt16 converts a string to a uint16 value.
// It returns 0 for empty strings, "0", invalid strings, negative values,
// or values that exceed the uint16 range (0-65535).
func StringToUInt16(s string) uint16 {
	if s == "" || s == "0" {
		return 0
	}

	i, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0
	}

	return uint16(i)
}

// StringToInt64 converts the given string to an int64.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int64.
func StringToInt64(s string) int64 {
	if s == "" || s == "0" {
		return 0
	}

	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return i
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
		u, err := strconv.Unquote(s)
		if err == nil {
			return Checkhtmlentities(u)
		}
	}

	return Checkhtmlentities(s)
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

// SlicesContainsI reports whether v is present in s - case insensitive.
func SlicesContainsI(s []string, v string) bool {
	for idx := range s {
		if v == s[idx] || strings.EqualFold(v, s[idx]) {
			return true
		}
	}

	return false
}

// SlicesIndexI returns the index of the first occurrence of v in s, using case-insensitive comparison.
// If v is not found in s, it returns -1.
func SlicesIndexI(s []string, v string) int {
	for idx := range s {
		if v == s[idx] || strings.EqualFold(v, s[idx]) {
			return idx
		}
	}

	return -1
}

// SlicesContainsPart2I reports whether any element of s is contained in v - case insensitive.
func SlicesContainsPart2I(s []string, v string) bool {
	for idx := range s {
		if ContainsI(v, s[idx]) {
			return true
		}
	}

	return false
}

// StringRemoveAllRunesP removes all occurrences of the given runes from the string pointed to by s.
// It modifies the string in-place.
// If s is nil or an empty string, or if the slice of runes r is empty, this function does nothing.
// If the slice of runes r contains only one rune and that rune is not present in the string, this function does nothing.
// If the slice of runes r contains more than one rune and none of them are present in the string, this function does nothing.
// Otherwise, this function creates a new buffer, writes all the characters from the string that are not in the slice of runes r, and updates the string pointed to by s with the new content.
func StringRemoveAllRunesP(s *string, r ...byte) {
	if s == nil || *s == "" {
		return
	}

	if len(r) == 0 {
		return
	}

	if len(r) == 1 && !strings.ContainsRune(*s, rune(r[0])) {
		return
	}

	if len(r) > 1 {
		var bl bool
		for idx := range r {
			if strings.ContainsRune(*s, rune(r[idx])) {
				bl = true
				break
			}
		}

		if !bl {
			return
		}
	}

	initializePools()

	out := PlAddBuffer.Get()
	defer PlAddBuffer.Put(out)

	for idx := range *s {
		if isruneinbyteslice((*s)[idx], r) {
			continue
		}

		out.WriteByte((*s)[idx])
	}

	*s = out.String()
}

// isruneinbyteslice checks if the given byte r is present in the slice of bytes rs.
// It returns true if r is found in rs, false otherwise.
func isruneinbyteslice(r byte, rs []byte) bool {
	return slices.Contains(rs, r)
}

// StringReplaceWith replaces all occurrences of the byte r in s with the byte t.
// It returns a new string with the replacements.
func StringReplaceWith(s string, r, t byte) string {
	if s == "" {
		return s
	}

	if !strings.ContainsRune(s, rune(r)) {
		return s
	}

	initializePools()

	buf := PlAddBuffer.Get()
	defer PlAddBuffer.Put(buf)

	for idx := range s {
		if s[idx] == r {
			buf.WriteByte(t)
		} else {
			buf.WriteByte(s[idx])
		}
	}

	return buf.String()
}

// StringReplaceWithP replaces all occurrences of the rune r in the given string pointer s with the rune t.
// It modifies the string in-place.
func StringReplaceWithP(s *string, r, t byte) {
	if s == nil || *s == "" {
		return
	}

	if !strings.ContainsRune(*s, rune(r)) {
		return
	}

	initializePools()

	buf := PlAddBuffer.Get()
	defer PlAddBuffer.Put(buf)

	for idx := range *s {
		if (*s)[idx] == r {
			buf.WriteByte(t)
		} else {
			buf.WriteByte((*s)[idx])
		}
	}

	*s = buf.String()
}

// StringReplaceWithStr replaces all occurrences of the string r in s with the string t.
// It returns a new string with the replacements.
func StringReplaceWithStr(s, r, t string) string {
	if s == "" || r == "" {
		return s
	}

	if !strings.Contains(s, r) {
		return s
	}

	// Compute number of replacements.
	n := strings.Count(s, r)
	if n == 0 {
		return s // avoid allocation
	}

	// Apply replacements to buffer.
	initializePools()

	buf := PlAddBuffer.Get()
	defer PlAddBuffer.Put(buf)

	start := 0

	lenr := len(r)
	for i := range n {
		j := start
		if lenr == 0 {
			if i > 0 {
				_, wid := utf8.DecodeRuneInString(s[start:])

				j += wid
			}
		} else {
			idx := strings.Index(s[start:], r)
			if idx == -1 {
				// This shouldn't happen given our Count check, but handle gracefully
				break
			}

			j = start + idx
		}

		buf.WriteString(s[start:j])
		buf.WriteString(t)

		start = j + lenr
	}

	buf.WriteString(s[start:])

	return buf.String()
}

// CheckContextEnded checks if the provided context has been canceled or has expired.
// If the context has been canceled or has expired, it returns the context's error.
// Otherwise, it returns nil.
func CheckContextEnded(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Abort / return early
		return ctx.Err()
	default:
		return nil
	}
}

// HandlePanic recovers from a panic and logs the recovered value along with the stack trace.
func HandlePanic() {
	// detect if panic occurs or not
	a := recover()
	if a != nil {
		Logtype("error", 1).Str("RECOVER", Stack()).Any("vap", a).Msg("Recovered from panic")
	}
}

func Stack() string {
	buf := make([]byte, 1024)

	maxSize := 64 * 1024 // Prevent runaway memory usage
	for len(buf) <= maxSize {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}

		buf = make([]byte, 2*len(buf))
	}

	// Fallback if stack is exceptionally large
	return "Stack trace too large"
}

// TryTimeParse attempts to parse the given string `s` using the provided time layout `layout`.
// It returns the parsed time.Time value and a boolean indicating whether the parsing was successful.
func TryTimeParse(layout string, s string) (time.Time, bool) {
	sleeptime, err := time.Parse(layout, s)
	return sleeptime, err == nil
}

// GetFieldComments returns a map of field names to their comment values.
func GetFieldComments(v any) map[string]string {
	comments := make(map[string]string)

	if v == nil {
		return comments
	}

	t := reflect.TypeOf(v)
	if t == nil {
		return comments
	}

	// If it's a pointer, get the underlying type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == nil {
			return comments
		}
	}

	// Ensure it's a struct
	if t.Kind() != reflect.Struct {
		return comments
	}

	// Iterate through all fields
	for i := range t.NumField() {
		field := t.Field(i)

		comment := field.Tag.Get("longcomment")
		if comment != "" {
			comments[field.Name] = comment
		}
	}

	return comments
}

// GetFieldDisplayNames returns a map of field names to their displayname tag values.
func GetFieldDisplayNames(v any) map[string]string {
	displayNames := make(map[string]string)

	if v == nil {
		return displayNames
	}

	t := reflect.TypeOf(v)
	if t == nil {
		return displayNames
	}

	// If it's a pointer, get the underlying type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == nil {
			return displayNames
		}
	}

	// Ensure it's a struct
	if t.Kind() != reflect.Struct {
		return displayNames
	}

	// Iterate through all fields
	for i := range t.NumField() {
		field := t.Field(i)

		displayName := field.Tag.Get("displayname")
		if displayName != "" {
			displayNames[field.Name] = displayName
		}
	}

	return displayNames
}

// GetFieldCommentByName returns the comment for a specific field.
func GetFieldCommentByName(v any, fieldName string) string {
	if v == nil || fieldName == "" {
		return ""
	}

	t := reflect.TypeOf(v)
	if t == nil {
		return ""
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		if t == nil {
			return ""
		}
	}

	if t.Kind() != reflect.Struct {
		return ""
	}

	field, found := t.FieldByName(fieldName)
	if !found {
		return ""
	}

	return field.Tag.Get("comment")
}

// GetStringsMap returns the map of strings for the given type based on
// whether to use the series or movies map. If isType is true, it returns
// the mapstringsseries map, otherwise it returns the mapstringsmovies map.
// Deprecated: Use mtstrings.GetStringsMap directly instead.
func GetStringsMap(isType uint, typestr string) string {
	return mtstrings.GetStringsMap(isType, typestr)
}
