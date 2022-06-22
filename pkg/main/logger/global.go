package logger

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/rainycape/unidecode"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var DisableVariableCleanup bool

func StringReplaceArray(instr string, what []string, with string) string {
	defer ClearVar(&what)
	for idx := range what {
		instr = strings.Replace(instr, what[idx], with, -1)
	}
	return instr
}

var unavailableMapping = map[rune]rune{
	'\u0181': 'B',
	'\u1d81': 'd',
	'\u1d85': 'l',
	'\u1d89': 'r',
	'\u028b': 'v',
	'\u1d8d': 'x',
	'\u1d83': 'g',
	'\u0191': 'F',
	'\u0199': 'k',
	'\u019d': 'N',
	'\u0220': 'N',
	'\u01a5': 'p',
	'\u0224': 'Z',
	'\u0126': 'H',
	'\u01ad': 't',
	'\u01b5': 'Z',
	'\u0234': 'l',
	'\u023c': 'c',
	'\u0240': 'z',
	'\u0142': 'l',
	'\u0244': 'U',
	'\u2c60': 'L',
	'\u0248': 'J',
	'\ua74a': 'O',
	'\u024c': 'R',
	'\ua752': 'P',
	'\ua756': 'Q',
	'\ua75a': 'R',
	'\ua75e': 'V',
	'\u0260': 'g',
	'\u01e5': 'g',
	'\u2c64': 'R',
	'\u0166': 'T',
	'\u0268': 'i',
	'\u2c66': 't',
	'\u026c': 'l',
	'\u1d6e': 'f',
	'\u1d87': 'n',
	'\u1d72': 'r',
	'\u2c74': 'v',
	'\u1d76': 'z',
	'\u2c78': 'e',
	'\u027c': 'r',
	'\u1eff': 'y',
	'\ua741': 'k',
	'\u0182': 'B',
	'\u1d86': 'm',
	'\u0288': 't',
	'\u018a': 'D',
	'\u1d8e': 'z',
	'\u0111': 'd',
	'\u0290': 'z',
	'\u0192': 'f',
	'\u1d96': 'i',
	'\u019a': 'l',
	'\u019e': 'n',
	'\u1d88': 'p',
	'\u02a0': 'q',
	'\u01ae': 'T',
	'\u01b2': 'V',
	'\u01b6': 'z',
	'\u023b': 'C',
	'\u023f': 's',
	'\u0141': 'L',
	'\u0243': 'B',
	'\ua745': 'k',
	'\u0247': 'e',
	'\ua749': 'l',
	'\u024b': 'q',
	'\ua74d': 'o',
	'\u024f': 'y',
	'\ua751': 'p',
	'\u0253': 'b',
	'\ua755': 'p',
	'\u0257': 'd',
	'\ua759': 'q',
	'\u00d8': 'O',
	'\u2c63': 'P',
	'\u2c67': 'H',
	'\u026b': 'l',
	'\u1d6d': 'd',
	'\u1d71': 'p',
	'\u0273': 'n',
	'\u1d75': 't',
	'\u1d91': 'd',
	'\u00f8': 'o',
	'\u2c7e': 'S',
	'\u1d7d': 'p',
	'\u2c7f': 'Z',
	'\u0183': 'b',
	'\u0187': 'C',
	'\u1d80': 'b',
	'\u0289': 'u',
	'\u018b': 'D',
	'\u1d8f': 'a',
	'\u0291': 'z',
	'\u0110': 'D',
	'\u0193': 'G',
	'\u1d82': 'f',
	'\u0197': 'I',
	'\u029d': 'j',
	'\u019f': 'O',
	'\u2c6c': 'z',
	'\u01ab': 't',
	'\u01b3': 'Y',
	'\u0236': 't',
	'\u023a': 'A',
	'\u023e': 'T',
	'\ua740': 'K',
	'\u1d8a': 's',
	'\ua744': 'K',
	'\u0246': 'E',
	'\ua748': 'L',
	'\ua74c': 'O',
	'\u024e': 'Y',
	'\ua750': 'P',
	'\ua754': 'P',
	'\u0256': 'd',
	'\ua758': 'Q',
	'\u2c62': 'L',
	'\u0266': 'h',
	'\u2c73': 'w',
	'\u2c6a': 'k',
	'\u1d6c': 'b',
	'\u2c6e': 'M',
	'\u1d70': 'n',
	'\u0272': 'n',
	'\u1d92': 'e',
	'\u1d74': 's',
	'\u2c7a': 'o',
	'\u2c6b': 'Z',
	'\u027e': 'r',
	'\u0180': 'b',
	'\u0282': 's',
	'\u1d84': 'k',
	'\u0188': 'c',
	'\u018c': 'd',
	'\ua742': 'K',
	'\u1d99': 'u',
	'\u0198': 'K',
	'\u1d8c': 'v',
	'\u0221': 'd',
	'\u2c71': 'v',
	'\u0225': 'z',
	'\u01a4': 'P',
	'\u0127': 'h',
	'\u01ac': 'T',
	'\u0235': 'n',
	'\u01b4': 'y',
	'\u2c72': 'W',
	'\u023d': 'L',
	'\ua743': 'k',
	'\u0249': 'j',
	'\ua74b': 'o',
	'\u024d': 'r',
	'\ua753': 'p',
	'\u0255': 'c',
	'\ua757': 'q',
	'\u2c68': 'h',
	'\ua75b': 'r',
	'\ua75f': 'v',
	'\u2c61': 'l',
	'\u2c65': 'a',
	'\u01e4': 'G',
	'\u0167': 't',
	'\u2c69': 'K',
	'\u026d': 'l',
	'\u1d6f': 'm',
	'\u0271': 'm',
	'\u1d73': 'r',
	'\u027d': 'r',
	'\u1efe': 'Y',
}

func mapDecomposeUnavailable(r rune) rune {
	if v, ok := unavailableMapping[r]; ok {
		return v
	}
	return r
}

var transformer transform.Transformer = transform.Chain(runes.Map(mapDecomposeUnavailable), norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

var subRune = map[rune]string{
	'&':  "and",
	'@':  "at",
	'"':  "",
	'\'': "",
	'’':  "",
	'_':  "",
	'‒':  "-", // figure dash
	'–':  "-", // en dash
	'—':  "-", // em dash
	'―':  "-", // horizontal bar
	'ä':  "ae",
	'Ä':  "Ae",
	'ö':  "oe",
	'Ö':  "Oe",
	'ü':  "ue",
	'Ü':  "Ue",
	'ß':  "ss",
}

func makeSlug(s string) (slug string) {
	slug = strings.TrimSpace(s)
	slug = substituteRune(slug)
	slug = unidecode.Unidecode(slug)

	defer func() { // recovers panic
		if e := recover(); e != nil {
			fmt.Println("Recovered from panic (makeslug) ", e)
		}
	}()
	slug = strings.ToLower(slug)
	slug = replaceUnwantedChars(slug)
	slug = strings.Replace(slug, "--", "-", -1)
	slug = strings.Replace(slug, "--", "-", -1)
	slug = strings.Replace(slug, "--", "-", -1)
	return
}

// SubstituteRune substitutes string chars with provided rune
// substitution map. One pass.
func substituteRune(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		if d, ok := subRune[c]; ok {
			buf.WriteString(d)
		} else {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

var wantedChars = map[rune]bool{
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
	'A': true,
	'B': true,
	'C': true,
	'D': true,
	'E': true,
	'F': true,
	'G': true,
	'H': true,
	'I': true,
	'J': true,
	'K': true,
	'L': true,
	'M': true,
	'N': true,
	'O': true,
	'P': true,
	'Q': true,
	'R': true,
	'S': true,
	'T': true,
	'U': true,
	'V': true,
	'W': true,
	'X': true,
	'Y': true,
	'Z': true,
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

func replaceUnwantedChars(s string) string {
	var buf bytes.Buffer
	for _, c := range s {
		if _, ok := wantedChars[c]; ok {
			buf.WriteString(string(c))
		} else {
			buf.WriteRune('-')
		}
	}
	return buf.String()
}

//no chinese or cyrilic supported
func StringToSlug(instr string) string {
	instr = strings.Replace(instr, "\u00df", "ss", -1) // ß to ss handling
	if strings.Contains(instr, "&") || strings.Contains(instr, "%") {
		instr = strings.ToLower(html.UnescapeString(instr))
	} else {
		instr = strings.ToLower(instr)
	}
	if strings.Contains(instr, "\\u") {
		instr2, err := strconv.Unquote("\"" + instr + "\"")
		if err != nil {
			instr = instr2
		}
	}
	instr = makeSlug(instr)
	instr = strings.TrimSuffix(instr, "-")

	defer func() { // recovers panic
		if e := recover(); e != nil {
			fmt.Println("Recovered from panic (slugger) ", e)
		}
	}()

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
		if err != nil {
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

func GetUrlResponse(url string) (*http.Response, error) {
	webClient := &http.Client{Timeout: 120 * time.Second,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   20 * time.Second,
			ResponseHeaderTimeout: 20 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:          20,
			MaxConnsPerHost:       10,
			DisableCompression:    false,
			DisableKeepAlives:     true,
			IdleConnTimeout:       120 * time.Second}}
	req, _ := http.NewRequest("GET", url, nil)
	defer ClearVar(req)
	// Get the data
	return webClient.Do(req)
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(saveIn string, fileprefix string, filename string, url string) error {
	resp, err := GetUrlResponse(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer ClearVar(resp)

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

func CheckStringArray(array []string, find string) bool {
	defer ClearVar(&array)
	for idx := range array {
		if array[idx] == find {
			return true
		}
	}
	return false
}

func FindAndDeleteStringArray(array []string, item string) []string {
	defer ClearVar(&array)
	new := array[:0]
	defer ClearVar(&new)
	for idx := range array {
		if array[idx] != item {
			new = append(new, array[idx])
		}
	}
	return new
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

func TrimStringInclAfterString(s string, search string) string {
	if idx := strings.Index(s, search); idx != -1 {
		return strings.Repeat(s[:idx], 1)
	}
	return s
}
func TrimStringInclAfterStringInsensitive(s string, search string) string {
	if idx := strings.Index(strings.ToLower(s), strings.ToLower(search)); idx != -1 {
		s = strings.Repeat(s[:idx], 1)
	}
	s = strings.TrimRight(s, "-. ")
	return s
}
func TrimStringAfterString(s string, search string) string {
	if idx := strings.Index(s, search); idx != -1 {
		idn := idx + len(search)
		if idn >= len(s) {
			idn = len(s) - 1
		}
		return strings.Repeat(s[:idn], 1)
	}
	return s
}
func TrimStringAfterStringInsensitive(s string, search string) string {
	if idx := strings.Index(strings.ToLower(s), strings.ToLower(search)); idx != -1 {
		idn := idx + len(search)
		if idn >= len(s) {
			idn = len(s) - 1
		}
		return strings.Repeat(s[:idn], 1)
	}
	return s
}
func TrimStringPrefixInsensitive(s string, search string) string {
	if idx := strings.Index(strings.ToLower(s), strings.ToLower(search)); idx != -1 {
		idn := idx + len(search)
		s = strings.Repeat(s[idn:], 1)
		s = strings.TrimLeft(s, "-. ")
		return s
	}
	return s
}

func StringReplaceDiacritics(instr string) string {
	instr = strings.Replace(instr, "ß", "ss", -1)
	instr = strings.Replace(instr, "ä", "ae", -1)
	instr = strings.Replace(instr, "ö", "oe", -1)
	instr = strings.Replace(instr, "ü", "ue", -1)
	instr = strings.Replace(instr, "Ä", "Ae", -1)
	instr = strings.Replace(instr, "Ö", "Oe", -1)
	instr = strings.Replace(instr, "Ü", "Ue", -1)
	result, _, _ := transform.String(transformer, instr)
	return result
}

func Getrootpath(foldername string) (string, string) {
	var folders []string
	defer ClearVar(&folders)

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
	Log.Debug("Removed ", folders[0], " from ", foldername)
	return foldername, strings.TrimRight(folders[0], "/")
}
