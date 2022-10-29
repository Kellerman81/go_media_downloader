package logger

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"html"
	"io"
	"net/http"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/alitto/pond"
	"github.com/rainycape/unidecode"
)

var DisableVariableCleanup bool
var DisableParserStringMatch bool

var GlobalCache *cache.Cache = cache.New(0)
var GlobalConfigCache *cache.Cache = cache.New(0)
var GlobalRegexCache *cache.CacheRegex = cache.NewRegex(20 * time.Minute)
var GlobalStmtCache *cache.CacheStmt = cache.NewStmt(20 * time.Minute)
var GlobalStmtNamedCache *cache.CacheStmtNamed = cache.NewStmtNamed(20 * time.Minute)
var WorkerPools map[string]*pond.WorkerPool
var GlobalCounter map[string]int = map[string]int{}

func ParseStringTemplate(message string, messagedata interface{}) (string, error) {
	tmplmessage, err := template.New("tmplfile").Parse(message)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	var doc bytes.Buffer
	err = tmplmessage.Execute(&doc, messagedata)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	return doc.String(), err
}
func StringBuild(str ...string) string {
	var bld strings.Builder
	for idx := range str {
		bld.WriteString(str[idx])
	}
	str = nil
	return bld.String()
}

type InStringArrayStruct struct {
	Arr []string
}

func (s *InStringArrayStruct) Close() {
	if s != nil {
		s.Arr = nil
		s = nil
	}
}

type InIntArrayStruct struct {
	Arr []int
}

func (s *InIntArrayStruct) Close() {
	if s != nil {
		s.Arr = nil
		s = nil
	}
}

func InStringArray(target string, arr *InStringArrayStruct) bool {
	for idx := range arr.Arr {
		if strings.EqualFold(target, arr.Arr[idx]) {
			return true
		}
	}
	return false
}
func InStringArrayCaseSensitive(target string, arr *InStringArrayStruct) bool {
	for idx := range arr.Arr {
		if target == arr.Arr[idx] {
			return true
		}
	}
	return false
}
func InStringArrayContainsTarget(target string, arr *InStringArrayStruct) bool {
	defer arr.Close()
	for idx := range arr.Arr {
		if strings.Contains(target, arr.Arr[idx]) {
			return true
		}
	}
	return false
}
func InStringArrayContainsCaseSensitive(target string, arr *InStringArrayStruct) bool {
	for idx := range arr.Arr {
		if strings.Contains(target, arr.Arr[idx]) {
			return true
		}
	}
	return false
}
func InStringArrayContainsCaseInSensitive(target string, arr *InStringArrayStruct) bool {
	for idx := range arr.Arr {
		if ContainsIa(target, arr.Arr[idx]) {
			return true
		}
	}
	return false
}

func InIntArray(target int, arr *InIntArrayStruct) bool {
	for idx := range arr.Arr {
		if target == arr.Arr[idx] {
			return true
		}
	}
	return false
}

func InitWorkerPools(workerindexer int, workerparse int, workersearch int, workerfiles int, workermeta int, workerdefault int) {
	WorkerPools = make(map[string]*pond.WorkerPool, 5)
	if workerdefault == 0 {
		workerdefault = 1
	}
	if workerindexer == 0 {
		workerindexer = 1
	}
	if workerparse == 0 {
		workerparse = 1
	}
	if workersearch == 0 {
		workersearch = 1
	}
	if workerfiles == 0 {
		workerfiles = 1
	}
	if workermeta == 0 {
		workermeta = 1
	}
	panicHandler := func(p interface{}) {
		Log.Errorln("Task panicked: ", p)
	}
	WorkerPools["Indexer"] = pond.New(workerindexer, 1000, pond.IdleTimeout(10*time.Second), pond.PanicHandler(panicHandler), pond.Strategy(pond.Lazy()))
	WorkerPools["Parse"] = pond.New(workerparse, 1000, pond.IdleTimeout(10*time.Second), pond.PanicHandler(panicHandler), pond.Strategy(pond.Lazy()))
	WorkerPools["Search"] = pond.New(workersearch, 1000, pond.IdleTimeout(10*time.Second), pond.PanicHandler(panicHandler), pond.Strategy(pond.Lazy()))
	WorkerPools["Files"] = pond.New(workerfiles, 1000, pond.IdleTimeout(10*time.Second), pond.PanicHandler(panicHandler), pond.Strategy(pond.Lazy()))
	WorkerPools["Metadata"] = pond.New(workermeta, 1000, pond.IdleTimeout(10*time.Second), pond.PanicHandler(panicHandler), pond.Strategy(pond.Lazy()))
}

func CloseWorkerPools() {
	WorkerPools["Indexer"].Stop()
	WorkerPools["Parse"].Stop()
	WorkerPools["Search"].Stop()
	WorkerPools["Files"].Stop()
	WorkerPools["Metadata"].Stop()
}

func makeSlug(s string) (slug string) {
	defer func() { // recovers panic
		if e := recover(); e != nil {
			Log.GlobalLogger.Error("Recovered from panic (makeslug) ")
		}
	}()
	slug = replaceUnwantedChars(strings.ToLower(unidecode.Unidecode(substituteRuneF(s))))
	slug = strings.Replace(slug, "--", "-", -1)
	slug = strings.Replace(slug, "--", "-", -1)
	slug = strings.Replace(slug, "--", "-", -1)
	slug = strings.Trim(slug, "- ")
	return
}

var substituteRune = map[rune]string{
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

// SubstituteRune substitutes string chars with provided rune
// substitution map. One pass.
func substituteRuneF(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))

	for _, c := range s {
		if repl, ok := substituteRune[c]; ok {
			buf.WriteString(repl)
		} else {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

var subRune = map[rune]bool{
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

// Make sure to do this with lowercase strings
func replaceUnwantedChars(s string) string {
	var buf bytes.Buffer
	buf.Grow(len(s))

	for _, c := range s {
		if _, ok := subRune[c]; ok {
			buf.WriteString(string(c))
		} else {
			buf.WriteRune('-')
		}
	}
	return buf.String()
}

// no chinese or cyrilic supported
func StringToSlug(instr string) string {
	if strings.Contains(instr, "&") || strings.Contains(instr, "%") {
		instr = html.UnescapeString(instr)
	}
	if strings.Contains(instr, "\\u") {
		instr2, err := strconv.Unquote("\"" + instr + "\"")
		if err == nil {
			instr = instr2
		}
	}
	instr = makeSlug(instr)
	instr = strings.TrimSuffix(instr, "-")

	return instr
}

func Path(s string, allowslash bool) string {
	// Start with lowercase string
	filePath := ""
	if strings.Contains(s, "&") || strings.Contains(s, "%") {
		filePath = html.UnescapeString(s)
	} else {
		filePath = s
	}
	if strings.Contains(filePath, "\\u") {
		filePath2, err := strconv.Unquote("\"" + filePath + "\"")
		if err == nil {
			filePath = filePath2
		}
	}

	filePath = strings.Replace(filePath, "..", "", -1)
	filePath = path.Clean(filePath)
	if allowslash {
		for _, line := range []string{":", "*", "?", "\"", "<", ">", "|"} {
			filePath = strings.Replace(filePath, line, "", -1)
		}
	} else {
		for _, line := range []string{"\\", "/", ":", "*", "?", "\"", "<", ">", "|"} {
			filePath = strings.Replace(filePath, line, "", -1)
		}
	}
	filePath = strings.Trim(filePath, " ")

	// NB this may be of length 0, caller must check
	return filePath
}

var WebClient *http.Client = &http.Client{Timeout: 120 * time.Second,
	Transport: &http.Transport{
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          20,
		MaxConnsPerHost:       10,
		DisableCompression:    false,
		DisableKeepAlives:     true,
		IdleConnTimeout:       120 * time.Second}}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(saveIn string, fileprefix string, filename string, url string) error {
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := WebClient.Do(req)
	if err != nil || resp == nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	if len(filename) == 0 {
		filename = path.Base(resp.Request.URL.String())
	}
	var filepath string
	if len(fileprefix) >= 1 {
		filename = fileprefix + filename
	}
	filepath = path.Join(saveIn, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	out.Sync()
	return err
}

func ClearVar(i interface{}) {
	if DisableVariableCleanup {
		return
	}
	v := reflect.ValueOf(i)
	if !v.IsZero() && v.Kind() == reflect.Pointer {
		v.Elem().Set(reflect.Zero(v.Elem().Type()))
	} else {
		if !v.IsZero() {
			Log.Warningln("Couldn't cleanup: ", v.Kind(), " Type ", v.Elem().Type(), " value ", v.Interface())
		}
	}
}
func ClearMap(s interface{}) {
	if DisableVariableCleanup {
		return
	}
	v := reflect.ValueOf(s)
	if v.Elem().Kind() == reflect.Map {
		if !v.IsZero() && v.Kind() == reflect.Pointer {
			v.Elem().Set(reflect.Zero(v.Elem().Type()))
		}
	}
}

func ParseDate(date string, layout string) sql.NullTime {
	if date == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02", date)
	return sql.NullTime{Time: t, Valid: err == nil}
}

func TrimStringInclAfterString(s string, search string) string {
	if idx := strings.Index(s, search); idx != -1 {
		return s[:idx]
	}
	return s
}
func TrimStringInclAfterStringInsensitive(s string, search string) string {
	if strings.Contains(s, search) {
		return strings.TrimRight(s[:strings.Index(s, search)], "-. ")
	} else if idx := strings.Index(strings.ToLower(s), strings.ToLower(search)); idx != -1 {
		return strings.TrimRight(s[:idx], "-. ")
	}
	return s
}
func TrimStringPrefixInsensitive(s string, search string) string {
	if strings.HasPrefix(s, search) {
		return strings.TrimLeft(s[(strings.Index(s, search)+len(search)):], "-. ")
	} else if StringHasPrefixCaseInsensitive(s, search) {
		return strings.TrimLeft(s[(len(search)):], "-. ")
	}
	return s
}

var substituteDiacritics = map[rune]string{
	'ä': "ae",
	'ö': "oe",
	'ü': "ue",
	'Ä': "Ae",
	'Ö': "Oe",
	'Ü': "Ue",
	'ß': "ss",
}

func StringReplaceDiacritics(instr string) string {
	var buf bytes.Buffer
	buf.Grow(len(instr))

	for _, c := range instr {
		if repl, ok := substituteDiacritics[c]; ok {
			buf.WriteString(repl)
		} else {
			buf.WriteRune(c)
		}
	}
	return unidecode.Unidecode(buf.String())
}

func Getrootpath(foldername string) (string, string) {
	var folders []string = make([]string, 0, 10)

	if strings.Contains(foldername, "/") {
		folders = strings.Split(foldername, "/")
	}
	if strings.Contains(foldername, "\\") {
		folders = strings.Split(foldername, "\\")
	}
	if !strings.Contains(foldername, "/") && !strings.Contains(foldername, "\\") {
		folders = []string{foldername}
	}
	foldername = strings.TrimPrefix(foldername, strings.TrimRight(folders[0], "/"))
	foldername = strings.Trim(foldername, "/")
	//Log.Debug("Removed ", folders[0], " from ", foldername)
	folderfirst := strings.TrimRight(folders[0], "/")
	return foldername, folderfirst
}

// Filter any Slice
// ex.
//
//	b := Filter(a.Elements, func(e Element) bool {
//		return strings.Contains(strings.ToLower(e.Name), strings.ToLower("woman"))
//	})
func Filter[T any](s []T, cond func(t T) bool) []T {
	res := []T{}
	for i := range s {
		if cond(s[i]) {
			res = append(res, s[i])
		}
	}
	return res
}

// Copy one struct array to a different type one
// a := []A{{"Batman"}, {"Diana"}}
//
//	b := CopyFunc[A, B](a, func(elem A) B {
//		return B{
//			Name: elem.Name,
//		}
//	})
func CopyFunc[T any, U any](src []T, copyFunc func(elem T) U) []U {
	dst := make([]U, len(src))
	for i := range src {
		dst[i] = copyFunc(src[i])
	}
	return dst
}

func GrowSliceBy[T any](src []T, i int) []T {
	t := make([]T, len(src), len(src)+i)
	copy(t, src)
	return t
}

func StringArrayToLower(src []string) []string {
	for idx := range src {
		src[idx] = strings.ToLower(src[idx])
	}
	return src
}

func StringHasPrefixCaseInsensitive(str string, prefix string) bool {
	if len(prefix) > len(str) {
		return false
	}
	return strings.EqualFold(str[:len(prefix)], prefix)
}
func StringHasSuffixCaseInsensitive(str string, suffix string) bool {
	if len(suffix) > len(str) {
		return false
	}
	return strings.EqualFold(str[len(suffix)+1:], suffix)
}

func StringTrimAfterCaseInsensitive(str string, search string) string {
	if len(search) > len(str) {
		return str
	}
	if strings.Contains(str, search) {
		return strings.TrimRight(str[:strings.Index(str, search)], "-. ")
	}
	for i := 0; i < len(str)-len(search); i++ {
		if strings.EqualFold(str[i:i+len(search)], search) {
			return str[i+len(search):]
		}
	}
	return str
}

func cCheckFunc[T any](src []T, checkFunc func(elem T) bool) bool {
	for i := range src {
		if checkFunc(src[i]) {
			return true
		}
	}
	return false
}

func RunFunc[T any](src []T, runFunc func(elem T), breakFunc func(elem T) bool) {
	for i := range src {
		runFunc(src[i])
		if breakFunc(src[i]) {
			break
		}
	}
}

func ContainsI(a string, b string) bool {
	if strings.Contains(a, b) {
		return true
	}
	return strings.Contains(
		strings.ToLower(a),
		strings.ToLower(b),
	)
}
func ContainsIa(a string, b string) bool {
	if strings.Contains(a, b) {
		return true
	}
	return strings.Contains(
		strings.ToLower(a),
		b,
	)
}
func ContainsIb(a string, b string) bool {
	if strings.Contains(a, b) {
		return true
	}
	return strings.Contains(
		a,
		strings.ToLower(b),
	)
}
