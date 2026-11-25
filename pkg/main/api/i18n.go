package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
)

// Translation of the end-user pages (Library, Calendar, Add Media).
//
// Go code marks a text with t("English") or tf("English %d", n); the marks
// are Unicode private-use characters that HTML escaping and JSON leave
// alone. JavaScript marks a constant text with _t('English') or a format
// with _tf('English %s', x). i18nMiddleware translates an HTML or JSON
// response as a whole into the account's language: the render code needs
// no language, also not in the fragments htmx and fetch load.
//
// A text's key is its English. The other languages come from a JSON file
// each in static/i18n, named by the language (de.json):
//
//	{"name": "Deutsch", "texts": {"Play": "Abspielen", "%d of %d": "%[1]s von %[2]s"}}
//
// A new language is a new file there; the files are read at start.
// en.json, which lists every text as itself, changes the English ones.
// A format names its arguments %s, %d or %v, or %[2]s to change the order.
// Translations hold no ' " \ < or > (they may end up in JavaScript and
// attributes): use „…“ and ’. A translation that breaks a rule is left
// out (logged), its text stays English.

const (
	trOpen  = '\ue000'
	trSep   = '\ue002'
	trClose = '\ue001'
)

// i18nDir holds the language files.
const i18nDir = "./static/i18n"

// languages are the languages the pages come in, English first; set by
// loadLanguages like languageNames and catalogs.
var (
	languages     = []string{"en"}
	languageNames = map[string]string{"en": "English"}
	// catalogs hold the translations of each language but English.
	catalogs = map[string]map[string]string{}
)

// langFile is a language file of i18nDir.
type langFile struct {
	Name  string            `json:"name"`
	Texts map[string]string `json:"texts"`
}

// loadLanguages reads the language files of dir. Run it before serving:
// the maps are read without a lock.
func loadLanguages(dir string) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))

	langs, names, cats := []string{"en"}, map[string]string{"en": "English"}, map[string]map[string]string{}

	for _, f := range files {
		lang := strings.ToLower(strings.TrimSuffix(filepath.Base(f), ".json"))
		if strings.ContainsAny(lang, " '\"<>") {
			continue
		}

		var lf langFile

		data, err := os.ReadFile(f)
		if err == nil {
			err = json.Unmarshal(data, &lf)
		}

		if err != nil {
			logger.Logtype("error", 0).Err(err).Str("file", f).Msg("language file")
			continue
		}

		texts := make(map[string]string, len(lf.Texts))

		for key, tr := range lf.Texts {
			// en.json lists every text as itself, to be changed one by one.
			if tr == key || tr == "" {
				continue
			}

			if err := trCheck(key, tr); err != nil {
				logger.Logtype("warn", 0).Err(err).Str("file", f).Str("text", key).Msg("language file")
				continue
			}

			texts[key] = tr
		}

		if lf.Name == "" {
			lf.Name = lang
		}

		if lang != "en" {
			langs = append(langs, lang)
		}

		names[lang] = lf.Name
		cats[lang] = texts
	}

	slices.Sort(langs[1:])

	languages, languageNames, catalogs = langs, names, cats
}

var (
	errTrChars = errors.New("translation holds ' \" \\ < or >")
	errTrVerbs = errors.New("translation has other format verbs than its text")
)

// trCheck reports whether a translation can go into the pages: none of the
// characters that break JavaScript or attributes, the same number of
// format verbs as its text.
func trCheck(key, tr string) error {
	if strings.ContainsAny(tr, `'"\<>`) {
		return errTrChars
	}

	if len(trVerbRe.FindAllString(key, -1)) != len(trVerbRe.FindAllString(tr, -1)) {
		return errTrVerbs
	}

	return nil
}

// t marks a text for translation.
func t(s string) string {
	return string(trOpen) + s + string(trClose)
}

// tf marks a format and its arguments for translation; the arguments are
// put in after translating the format.
func tf(format string, args ...any) string {
	var b strings.Builder

	b.WriteRune(trOpen)
	b.WriteString(format)

	for _, a := range args {
		b.WriteRune(trSep)
		b.WriteString(fmt.Sprint(a))
	}

	b.WriteRune(trClose)

	return b.String()
}

var trVerbRe = regexp.MustCompile(`%(\[\d+\])?[dsv]`)

// trFormat puts the arguments into a format (all verbs as strings).
func trFormat(format string, args []string) string {
	if len(args) == 0 {
		return format
	}

	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = a
	}

	return fmt.Sprintf(trVerbRe.ReplaceAllString(format, "%${1}s"), vals...)
}

var trJSONEsc = strings.NewReplacer(`\u0026`, "&", `\u003c`, "<", `\u003e`, ">", `\"`, `"`, `\\`, `\`)

// trLookup translates a marked text. It reaches here escaped like its
// surroundings (HTML: &amp; &#34; &#39;, JSON: \u0026 \"), so a text
// with such characters is looked up unescaped and its translation escaped
// the same way.
func trLookup(cat map[string]string, key string) string {
	if v, ok := cat[key]; ok && v != "" {
		return v
	}

	if strings.Contains(key, "&") {
		if v, ok := cat[html.UnescapeString(key)]; ok && v != "" {
			return html.EscapeString(v)
		}
	}

	if strings.Contains(key, `\`) {
		if v, ok := cat[trJSONEsc.Replace(key)]; ok && v != "" {
			return v
		}
	}

	return key
}

var (
	trJSRe  = regexp.MustCompile(`_t\('((?:[^'\\\n]|\\.)*)'\)`)
	trJSfRe = regexp.MustCompile(`_tf\('((?:[^'\\\n]|\\.)*)'`)
)

// translateOutput translates the marked texts of a response into lang
// (English: the marks are only taken out).
func translateOutput(out []byte, lang string) []byte {
	cat := catalogs[lang]

	open, sep, closeMark := []byte(string(trOpen)), []byte(string(trSep)), []byte(string(trClose))

	// Innermost first: a format's argument may be marked itself.
	for {
		end := bytes.Index(out, closeMark)
		if end < 0 {
			break
		}

		start := bytes.LastIndex(out[:end], open)
		if start < 0 {
			// A stray end mark: drop it.
			out = append(out[:end], out[end+len(closeMark):]...)
			continue
		}

		parts := bytes.Split(out[start+len(open):end], sep)
		args := make([]string, 0, len(parts)-1)

		for _, a := range parts[1:] {
			args = append(args, string(a))
		}

		text := trFormat(trLookup(cat, string(parts[0])), args)

		var b bytes.Buffer

		b.Grow(len(out) + len(text))
		b.Write(out[:start])
		b.WriteString(text)
		b.Write(out[end+len(closeMark):])
		out = b.Bytes()
	}

	out = bytes.ReplaceAll(out, open, nil)
	out = bytes.ReplaceAll(out, sep, nil)

	if cat == nil {
		return out
	}

	jsText := func(re *regexp.Regexp, prefix, suffix string) {
		out = re.ReplaceAllFunc(out, func(m []byte) []byte {
			key := string(re.FindSubmatch(m)[1])
			plain := strings.ReplaceAll(key, `\'`, `'`)

			tr, ok := cat[plain]
			if !ok || tr == "" {
				return m
			}

			return []byte(prefix + strings.ReplaceAll(tr, `'`, `\'`) + suffix)
		})
	}

	jsText(trJSRe, "_t('", "')")
	jsText(trJSfRe, "_tf('", "'")

	return bytes.Replace(out, []byte(`<html lang="en"`), []byte(`<html lang="`+lang+`"`), 1)
}

// trJS defines _t and _tf for the pages' scripts; the texts are already
// translated in the page, so _t only returns its text and _tf puts the
// arguments in.
const trJS = `function _t(s) { return s; }
function _tf(f) { var a = arguments, i = 1; return f.replace(/%(?:\[(\d+)\])?[dsv]/g, function (m, p) { return String(a[p ? +p : i++]); }); }`

// requestLanguage is the account's language: its setting, else the
// browser's first language there is a translation for, else English.
func requestLanguage(c *gin.Context) string {
	if _, ok := c.Get("session"); ok {
		var s struct {
			Lang string `json:"lang"`
		}

		if json.Unmarshal([]byte(database.GetUserSettings(accName(accessOf(c)))), &s) == nil &&
			s.Lang != "" && s.Lang != "auto" && languageNames[s.Lang] != "" {
			return s.Lang
		}
	}

	return browserLanguage(c.GetHeader("Accept-Language"))
}

// browserLanguage picks from an Accept-Language header ("de-DE,de;q=0.9,
// en;q=0.8") the first language there are pages in (the order the browser
// lists them in is its preference).
func browserLanguage(header string) string {
	for part := range strings.SplitSeq(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")

		if languageNames[base] != "" {
			return base
		}
	}

	return "en"
}

// trWriter holds back an HTML, JSON or plain text response to translate it
// once the handler is done; anything else (video, downloads, event
// streams) goes straight through.
type trWriter struct {
	gin.ResponseWriter

	decided, hold bool
	buf           bytes.Buffer
}

func (w *trWriter) decide() {
	if w.decided {
		return
	}

	w.decided = true
	ct := w.Header().Get("Content-Type")
	w.hold = strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/plain")
}

func (w *trWriter) Write(p []byte) (int, error) {
	w.decide()

	if w.hold {
		return w.buf.Write(p)
	}

	return w.ResponseWriter.Write(p)
}

func (w *trWriter) WriteString(s string) (int, error) {
	w.decide()

	if w.hold {
		return w.buf.WriteString(s)
	}

	return w.ResponseWriter.WriteString(s)
}

// Flush sends a held response only at the end.
func (w *trWriter) Flush() {
	if w.decided && w.hold {
		return
	}

	w.ResponseWriter.Flush()
}

// Written reports a held response as written, as gin expects after a
// handler wrote one.
func (w *trWriter) Written() bool {
	return w.ResponseWriter.Written() || w.buf.Len() > 0
}

func (w *trWriter) finish(c *gin.Context) {
	if !w.hold {
		return
	}

	out := w.buf.Bytes()
	if bytes.ContainsRune(out, trOpen) || bytes.Contains(out, []byte("_t")) {
		out = translateOutput(out, requestLanguage(c))
		w.Header().Del("Content-Length")
	}

	if _, err := w.ResponseWriter.Write(out); err != nil && c.Request.Context().Err() == nil {
		c.Error(err) //nolint:errcheck // recorded for the logger
	}
}

// i18nMiddleware translates the marked texts of the responses.
func i18nMiddleware(c *gin.Context) {
	if c.Request.Method == http.MethodHead {
		c.Next()
		return
	}

	w := &trWriter{ResponseWriter: c.Writer}
	c.Writer = w

	c.Next()

	c.Writer = w.ResponseWriter
	w.finish(c)
}
