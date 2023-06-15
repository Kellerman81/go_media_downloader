package logger

import (
	"bytes"
	"errors"
	"html"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/mozillazg/go-unidecode"
	//"github.com/rainycape/unidecode"
)

const (
	FilterByTvdb              = "thetvdb_id = ?"
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
	StrName                   = "Name"
	StrTitle                  = "Title"
	StrIndexer                = "Indexer"
	StrEntries                = "Entries"
	StrPriority               = "Priority"
	StrYear                   = "Year"
	StrData                   = "data"
	StrDataFull               = "datafull"
	StrFeeds                  = "feeds"
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
	StrMovieFileUnmatched     = "movie_file_unmatcheds_cached"
	StrSerieFileUnmatched     = "serie_file_unmatcheds_cached"
)

var (
	StrRegexSeriesIdentifier = "RegexSeriesIdentifier"
	StrRegexSeriesTitle      = "RegexSeriesTitle"
	StrRssSeasons            = "rssseasons"
	StrMovie                 = "movie"
	StrSeries                = "series"
	StrSerie                 = "serie"
	StrRss                   = "rss"
	Underscore               = "_"
	Empty                    = ""
	StrTvdb                  = "tvdb"
	StrTt                    = "tt"
	StrMissing               = "missing"
	StrUpgrade               = "upgrade"
	DisableVariableCleanup   bool
	DisableParserStringMatch bool
	GlobalCache              = cache.New(20*time.Minute, 20*time.Minute)
	GlobalCacheRegex         = cache.NewRegex(20*time.Minute, 20*time.Minute)
	GlobalCacheStmt          = cache.NewStmt(20*time.Minute, 20*time.Minute)
	GlobalCacheNamed         = cache.NewNamed(20*time.Minute, 20*time.Minute)
	GlobalCounter            = make(map[string]int)
	ErrWrongExtension        = errors.New("mismatched extension")
	ErrWrongPrefix           = errors.New("mismatched prefix")
	ErrNoGeneral             = errors.New("no general")
	ErrNoID                  = errors.New("no id")
	ErrNoFiles               = errors.New("no files")
	ErrNoPath                = errors.New("no path")
	ErrNotFound              = errors.New("not found")
	ErrNoListRead            = errors.New("list not readable")
	ErrNoListOther           = errors.New("list other error")
	Errwrongtype             = errors.New("wrong type")
	ErrNoUsername            = errors.New("no username")
	ErrNoShowOrMovie         = errors.New("not show or movie")
	ErrCsvImport             = errors.New("list csv import error")
	ErrRuntime               = errors.New("wrong runtime")
	ErrLanguage              = errors.New("wrong language")
	ErrNotAllowed            = errors.New("not allowed")
	ErrLowerQuality          = errors.New("lower quality")
	ErrNoIndexerSearched     = errors.New("no indexer searched")
	ErrOther                 = errors.New("other error")
	ErrDisabled              = errors.New("disabled")
	ErrToWait                = errors.New("please wait")
	ErrDailyLimit            = errors.New("daily limit reached")
	Errnoresults             = errors.New("no results")
	ErrInvalid               = errors.New("invalid")
	ErrNotFoundDbmovie       = errors.New("dbmovie not found")
	ErrNotFoundMovie         = errors.New("movie not found")
	ErrNotFoundDbserie       = errors.New("dbserie not found")
	ErrNotFoundSerie         = errors.New("serie not found")
	ErrNotEnabled            = errors.New("not enabled")
	ErrCfgpNotFound          = errors.New("cfgpstr not found")
	pathchars                = [7]rune{':', '*', '?', '"', '<', '>', '|'}
	pathcharsext             = [9]rune{'\\', '/', ':', '*', '?', '"', '<', '>', '|'}
	substituteRune           = map[rune]string{
		'&':  "and",
		'@':  "at",
		'"':  "",
		'\'': "",
		'’':  "",
		'_':  "",
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
	subRune = map[rune]bool{
		'a': true,
		'b': true,
		'c': true,
		'd': true,
		'e': true,
		'f': true,
		'g': true,
		'h': true,
		'i': true,
		'j': true,
		'k': true,
		'l': true,
		'm': true,
		'n': true,
		'o': true,
		'p': true,
		'q': true,
		'r': true,
		's': true,
		't': true,
		'u': true,
		'v': true,
		'w': true,
		'x': true,
		'y': true,
		'z': true,
		'0': true,
		'1': true,
		'2': true,
		'3': true,
		'4': true,
		'5': true,
		'6': true,
		'7': true,
		'8': true,
		'9': true,
		'-': true,
	}
	substituteDiacritics = map[rune]string{
		'ä': "ae",
		'ö': "oe",
		'ü': "ue",
		'Ä': "Ae",
		'Ö': "Oe",
		'Ü': "Ue",
		'ß': "ss",
	}
)

func ParseStringTemplate(message string, messagedata interface{}) (string, error) {
	tmplmessage, err := template.New("tmplfile").Parse(message)
	if err != nil {
		Logerror(err, "template")
		return "", err
	}
	var doc bytes.Buffer
	defer doc.Reset()
	err = tmplmessage.Execute(&doc, messagedata)
	if err != nil {
		Logerror(err, "template")
		return "", err
	}
	return doc.String(), err
}

// SubstituteRune substitutes string chars with provided rune
// substitution map. One pass.
// Changes the source string
func substituteRuneF(s *string) {
	var repl string
	var ok bool
	for _, c := range *s {
		if repl, ok = substituteRune[c]; ok {
			StringReplaceRunePP(s, c, &repl)
		}
	}
}

func replaceUnwantedChars(s string) string {
	if len(s) == 0 {
		return ""
	}
	var ok bool
	for _, r := range s {
		if _, ok = subRune[r]; !ok {
			break
		}
	}
	if ok {
		return s
	}
	out := []rune(s)
	defer Clear(&out)
	for idx, r := range out {
		if _, ok = subRune[r]; !ok {
			out[idx] = '-'
			//*s = (*s)[0:idx] + "-" + (*s)[idx+1:]
			//StringReplaceRuneP(s, r, "-")
		}
	}
	return string(out)
}

// no chinese or cyrilic supported
func StringToSlug(instr string) string {
	if len(instr) == 0 {
		return ""
	}
	HTMLUnescape(&instr)
	Unquote(&instr)
	substituteRuneF(&instr)
	// defer func() { // recovers panic
	// 	if e := recover(); e != nil {
	// 		Log.Error().Msg("Recovered from panic (makeslug)")
	// 	}
	// }()
	instr = strings.ReplaceAll(replaceUnwantedChars(unidecode.Unidecode(strings.ToLower(instr))), "--", "-")
	instr = strings.ReplaceAll(instr, "--", "-")
	instr = strings.ReplaceAll(instr, "--", "-")
	return strings.Trim(instr, "- ")
}

func AddImdbPrefix(str string) string {
	if len(str) == 0 {
		return ""
	}
	if !HasPrefixI(str, StrTt) {
		return StrTt + str
	}
	return str
}
func AddImdbPrefixP(str *string) {
	if str == nil || len(*str) == 0 {
		return
	}
	if !HasPrefixI(*str, StrTt) {
		*str = StrTt + *str
	}
}
func AddTvdbPrefix(str string) string {
	if len(str) == 0 {
		return ""
	}
	if !HasPrefixI(str, StrTvdb) {
		return StrTvdb + str
	}
	return str
}

// Changes the source string
func Path(s *string, allowslash bool) {
	if s == nil || len(*s) == 0 {
		return
	}
	HTMLUnescape(s)
	Unquote(s)
	*s = path.Clean(strings.ReplaceAll(*s, "..", ""))
	if allowslash && strings.ContainsAny(*s, ":*?\"><|") {
		for idx := range pathchars {
			StringDeleteRuneP(s, pathchars[idx])
		}
	}
	if !allowslash && strings.ContainsAny(*s, "\\/:*?\"><|") {
		for idx := range pathcharsext {
			StringDeleteRuneP(s, pathcharsext[idx])
		}
	}

	*s = strings.Trim(*s, " ")
}

// Changes the source string
func TrimStringInclAfterString(s *string, search string) {
	if idx := IndexI(*s, search); idx != -1 {
		*s = (*s)[:idx]
	}
}
func TrimStringInclAfterStringB(s string, search string) string {
	if idx := IndexI(s, search); idx != -1 {
		return s[:idx]
	}
	return s
}

// Changes the source string
func StringReplaceDiacritics(instr *string) {
	var repl string
	var ok bool
	for _, c := range *instr {
		if repl, ok = substituteDiacritics[c]; ok {
			StringReplaceRunePP(instr, c, &repl)
		}
	}
}

func StringReplaceRuneS(instr string, search rune, replace string) string {
	StringReplaceRunePP(&instr, search, &replace)
	return instr
}

func RuneReplaceRuneS(instr string, search rune, replace rune) string {
	RuneReplaceRuneP(&instr, search, replace)
	return instr
}

// Changes the source string
func RuneReplaceRuneP(instr *string, search rune, replace rune) {
	//var idx int
	*instr = strings.ReplaceAll(*instr, string(search), string(replace))
	i := StringRuneCount(*instr, search)
	if i == 0 {
		return
	}
	*instr = strings.Replace(*instr, string(search), string(replace), i)
	// return
	// out := []rune(*instr)
	// for idx, r := range out {
	// 	if r == search {
	// 		out[idx] = replace
	// 	}
	// }
	// *instr = string(out)
	// Clear(&out)
}

// Changes the source string
func StringReplaceRuneP(instr *string, search rune, replace string) {
	i := StringRuneCount(*instr, search)
	if i == 0 {
		return
	}
	*instr = strings.Replace(*instr, string(search), replace, i)
}

// Changes the source string
func StringReplaceRunePP(instr *string, search rune, replace *string) {
	i := StringRuneCount(*instr, search)
	if i == 0 {
		return
	}
	*instr = strings.Replace(*instr, string(search), *replace, i)
}

func StringRuneCount(instr string, search rune) int {
	i := 0
	for _, r := range instr {
		if r == search {
			i++
		}
	}
	return i
}

// Changes the source string
func StringDeleteRuneP(instr *string, search rune) {
	//var idx int
	i := StringRuneCount(*instr, search)
	if i == 0 {
		return
	}
	*instr = strings.Replace(*instr, string(search), "", i)
	// for i := 0; i <= StringRuneCount(*instr, search); i++ {
	// 	idx = strings.IndexRune(*instr, search)
	// 	if idx != -1 {
	// 		*instr = (*instr)[0:idx] + (*instr)[idx+1:]
	// 	}
	// 	if count != 0 && i == (count-1) {
	// 		break
	// 	}
	// }
}

func Getrootpath(foldername string) (string, string) {
	var splitby rune
	if strings.ContainsRune(foldername, '/') {
		splitby = '/'
	} else if strings.ContainsRune(foldername, '\\') {
		splitby = '\\'
	} else {
		return strings.Trim(foldername, "/"), ""
	}

	folders := strings.TrimRight(SplitBy(foldername, splitby), "/")
	foldername = strings.TrimPrefix(foldername, folders)
	foldername = strings.Trim(foldername, "/")
	return foldername, folders
}

// Filter any Slice
// ex.
//
//	b := Filter(a.Elements, func(e Element) bool {
//		return strings.Contains(strings.ToLower(e.Name), strings.ToLower("woman"))
//	})
// func Filter[T any](s []T, cond func(t T) bool) []T {
// 	res := s[:0]
// 	for i := range s {
// 		if cond(s[i]) {
// 			res = append(res, s[i])
// 		}
// 	}
// 	return res
// }

// Copy one struct array to a different type one
// a := []A{{"Batman"}, {"Diana"}}
//
//	b := CopyFunc[A, B](a, func(elem A) B {
//		return B{
//			Name: elem.Name,
//		}
//	})
// func CopyFunc[T any, U any](src []T, copyFunc func(elem T) U) []U {
// 	dst := make([]U, len(src))
// 	for i := range src {
// 		dst[i] = copyFunc(src[i])
// 	}
// 	return dst
// }

// func StringArrayToLower(src []string) []string {
// 	for idx := range src {
// 		src[idx] = strings.ToLower(src[idx])
// 	}
// 	return src
// }

// func CheckFunc[T any](src *[]T, checkFunc func(elem *T) bool) bool {
// 	for _, a := range *src {
// 		if checkFunc(&a) {
// 			return true
// 		}
// 	}
// 	return false
// }

// func RunFunc[T any](src *[]T, runFunc func(elem *T), breakFunc func(elem *T) bool) {
// 	for _, a := range *src {
// 		runFunc(&a)
// 		if breakFunc(&a) {
// 			break
// 		}
// 	}
// }

// func RunFuncSimple[T any](src *[]T, runFunc func(elem *T)) {
// 	for _, a := range *src {
// 		runFunc(&a)
// 	}
// }

func ContainsI(a string, b string) bool {
	return IndexI(a, b) != -1
}

// HasPrefix tests whether the string s begins with prefix.
func HasPrefixI(s, prefix string) bool {
	if strings.HasPrefix(s, prefix) {
		return true
	}
	return len(s) >= len(prefix) && strings.EqualFold(s[0:len(prefix)], prefix)
}

// HasSuffix tests whether the string s ends with suffix.
func HasSuffixI(s, suffix string) bool {
	if strings.HasSuffix(s, suffix) {
		return true
	}
	return len(s) >= len(suffix) && strings.EqualFold(s[len(s)-len(suffix):], suffix)
}
func IndexI(a string, b string) int {
	if len(a) < len(b) {
		return -1
	}
	j := strings.Index(a, b)
	if j >= 0 {
		return j
	}
	lb := len(b)
	la := len(a)
	for i := 0; i < (la - lb + 1); i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			return i
		}
	}
	return -1
}
func IndexILast(a string, b string) int {
	j := -1
	lb := len(b)
	la := len(a)
	for i := 0; i < (la - lb + 1); i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			j = i
		}
	}
	return j
}

func StringToInt(s string) int {
	if s == "" {
		return 0
	}
	in, err := strconv.ParseUint(s, 10, 0)
	//in, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return int(in)
}
func StringToInt64(s string) int64 {
	if s == "" {
		return 0
	}
	in, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return int64(in)
}
func StringToUint32(s string) uint32 {
	if s == "" {
		return 0
	}
	in, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0
	}
	return uint32(in)
}

func IntToString(i int) string {
	if i == 0 {
		return "0"
	}
	if i < 0 {
		return strconv.FormatInt(int64(i), 10)
	}
	return strconv.FormatUint(uint64(i), 10)
}
func UintToString(i uint) string {
	if i == 0 {
		return "0"
	}
	return strconv.FormatUint(uint64(i), 10)
}

func TimeGetNow() time.Time {
	return time.Now().In(&TimeZone)
}

func HTMLUnescape(s *string) {
	defer func() { // recovers panic
		if e := recover(); e != nil {
			Log.Error().Msgf("Recovered from panic (unescape) %v", e)
		}
	}()
	if strings.ContainsRune(*s, '&') || strings.ContainsRune(*s, '%') {
		*s = html.UnescapeString(*s)
	}
}

func HTMLUnescapeS(s string) string {
	HTMLUnescape(&s)
	return s
}
func Unquote(s *string) {
	if strings.Contains(*s, "\\u") {
		unquote, err := strconv.Unquote("\"" + *s + "\"")
		if err == nil {
			*s = unquote
		}
	}
}

func UnquoteS(s string) string {
	Unquote(&s)
	return s
}

// repeat ,? - start ? - count 1
func StringsRepeat(start string, repeat string, count int) string {
	return start + strings.Repeat(repeat, count)
}

func DeleteFromStringsCache(cachekey string, search string) {
	if !GlobalCache.CheckNoType(cachekey) {
		return
	}

	GlobalCache.DeleteString(cachekey, search)
}

func SplitBy(str string, splitby rune) string {
	idx := strings.IndexRune(str, splitby)
	if idx != -1 {
		return str[:idx]
	}
	return ""
}
func SplitByLR(str string, splitby rune) (string, string) {
	var str1, str2 = "", str
	var idx int
	for i, c := range str {
		if c == splitby {
			idx = i
		}
	}
	if idx != 0 {
		str1 = str[:idx]
		str2 = str[idx+1:]
	}
	return str1, str2
}
func Join[T any](str *[]T, getFunc func(elem *T) string, sep string) string {
	if str == nil || len(*str) == 0 || sep == "" {
		return ""
	}
	strs := make([]string, 0, (len(*str)))
	for idx := range *str {
		strs = append(strs, getFunc(&(*str)[idx]))
	}
	defer Clear(&strs)
	return strings.Join(strs, sep)
}
func SplitByRet(str string, splitby rune) string {
	idx := strings.IndexRune(str, splitby)
	if idx != -1 {
		return str[:idx]
	}
	return str
}

// func SplitByStr(str string, splitby string) string {
// 	idx := IndexI(str, splitby)
// 	if idx != -1 {
// 		return str[:idx]
// 	}
// 	return str
// }

// Changes source string
func SplitByStrMod(str *string, splitby string) {
	idx := IndexI(*str, splitby)
	if idx != -1 {
		*str = (*str)[:idx]
		if *str == "" {
			return
		}
		if (*str)[len(*str)-1:] == "-" || (*str)[len(*str)-1:] == "." || (*str)[len(*str)-1:] == " " {
			*str = strings.TrimRight(*str, "-. ")
		}
	}
}

// Changes source string
func SplitByStrModRight(str *string, splitby string) {
	idx := IndexI(*str, splitby)
	if idx != -1 {
		*str = (*str)[idx+len(splitby):]
		if *str == "" {
			return
		}
		if (*str)[:1] == "-" || (*str)[:1] == "." || (*str)[:1] == " " {
			*str = strings.TrimLeft(*str, "-. ")
		}
	}
}

func StringBuilder(str ...string) string {
	if len(str) == 0 {
		return ""
	}
	var strb strings.Builder
	for idx := range str {
		strb.WriteString(str[idx])
	}
	Clear(&str)
	defer strb.Reset()
	return strb.String()
}
func StringBuilderS(str ...string) string {
	if len(str) == 0 {
		return ""
	}
	var ret string
	for idx := range str {
		ret += str[idx]
	}
	Clear(&str)
	return ret
}
func StringBuilderSP(str ...string) *string {
	if len(str) == 0 {
		return new(string)
	}
	var ret string
	for idx := range str {
		ret += str[idx]
	}
	Clear(&str)
	return &ret
}
func StringBuilderPS(str ...*string) string {
	if len(str) == 0 {
		return ""
	}
	var ret string
	for idx := range str {
		ret += *(str[idx])
	}
	Clear(&str)
	return ret
}

func Getarray[T any](size int) []T {
	if size > 0 {
		return make([]T, 0, size)
	}
	return []T{}
}

// func GetarrayStatic[T any](size int) []T {
// 	return make([]T, size)
// }

func IndexFunc[T any](s *[]T, f func(T) bool) int {
	for i := range *s {
		if f((*s)[i]) {
			return i
		}
	}
	return -1
}

// func ContainsFunc[T any](s *[]T, f func(T) bool) bool {
// 	return IndexFunc(s, f) >= 0
// }

// func Index[T comparable](s *[]T, v T) int {
// 	for i := range *s {
// 		if v == (*s)[i] {
// 			return i
// 		}
// 	}
// 	return -1
// }
// func Contains[T comparable](s *[]T, v T) bool {
// 	return Index(s, v) >= 0
// }

func ContainsStringsI(s *[]string, v string) bool {
	for i := range *s {
		if strings.EqualFold(v, (*s)[i]) {
			return true
		}
	}
	return false
}

// func ContainsStringsContainI(s *[]string, v string) bool {
// 	for i := range *s {
// 		if ContainsI(v, (*s)[i]) {
// 			return true
// 		}
// 	}
// 	return false
// }

func Delete[T any](s *[]T, i int) {
	(*s)[i] = (*s)[len(*s)-1] //Replace found entry with last
	*s = (*s)[:len(*s)-1]
}

func Grow[S *[]T, T any](s S, n int) {
	if n < 1 {
		return
	}
	if n -= cap(*s) - len(*s); n > 0 {
		//*s = append(*s, make([]T, n)...)[:len(*s)]
		// TODO(https://go.dev/issue/53888): Make using []E instead of S
		// to workaround a compiler bug where the runtime.growslice optimization
		// does not take effect. Revert when the compiler is fixed.
		*s = append([]T(*s)[:cap(*s)], make([]T, n)...)[:len(*s)]
	}
}

func ClearVar(i interface{}) {
	if DisableVariableCleanup {
		return
	}
	v := reflect.ValueOf(i)
	if !v.IsZero() && v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	}
}
func ClearVarNew[T any](i *T) {
	if DisableVariableCleanup || i == nil {
		return
	}
	*i = *new(T)
}
func Clear[T any](t *[]T) {
	if DisableVariableCleanup || t == nil {
		return
	}
	*t = nil
}

func HTTPGetRequest(url *string) *http.Request {
	req, err := http.NewRequest("GET", *url, nil)
	if err != nil {
		LogerrorStr(err, StrURL, *url, "failed to get url")
		return nil
	}
	return req
}

func GetP[T any](s T) *T {
	return &s
}

func PathJoin(str1 string, str2 string) string {
	return filepath.Join(str1, str2)
}
