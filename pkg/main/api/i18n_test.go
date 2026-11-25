package api

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// plain is the English of a rendered text, as i18nMiddleware sends it:
// for tests that call a handler directly.
func plain(s string) string {
	return string(translateOutput([]byte(s), "en"))
}

// TestTranslateOutput: marked texts, formats (also reordered and with a
// marked argument) and the scripts' _t/_tf texts are translated; English
// only loses the marks; unknown texts stay English.
func TestTranslateOutput(test *testing.T) {
	old := catalogs["xx"]
	catalogs["xx"] = map[string]string{
		"Play":            "Spielen",
		"%d of %d":        "%[2]s von %[1]s",
		"Delete %s":       "%s löschen",
		"the entry":       "den Eintrag",
		"Can't play: ":    "Abspielen geht nicht: ",
		"%s items":        "%s Einträge",
		"Quote „inside“":  "x",
		"Unused in pages": "y",
		"Can't & won't":   "Geht & geht nicht",
	}

	test.Cleanup(func() {
		if old == nil {
			delete(catalogs, "xx")
		} else {
			catalogs["xx"] = old
		}
	})

	page := `<html lang="en"><b title="` + t("Play") + `">` + tf("%d of %d", 3, 10) + `</b>` + tf("Delete %s", t("the entry")) +
		t("Not translated") + `<script>libNotify('error', _t('Can\'t play: ') + e); x = _tf('%s items', n); y = _t('Other');</script>`

	want := `<html lang="xx"><b title="Spielen">10 von 3</b>den Eintrag löschen` +
		`Not translated<script>libNotify('error', _t('Abspielen geht nicht: ') + e); x = _tf('%s Einträge', n); y = _t('Other');</script>`
	if got := string(translateOutput([]byte(page), "xx")); got != want {
		test.Errorf("xx:\n got %s\nwant %s", got, want)
	}

	// Escaped by the HTML renderer or by JSON.
	escaped := `<i title="` + t("Can&#39;t &amp; won&#39;t") + `"></i>{"error":"` + t(`Can't \u0026 won't`) + `"}`
	if got, want := string(translateOutput([]byte(escaped), "xx")), `<i title="Geht &amp; geht nicht"></i>{"error":"Geht & geht nicht"}`; got != want {
		test.Errorf("escaped:\n got %s\nwant %s", got, want)
	}

	en := `<html lang="en"><b title="Play">3 of 10</b>Delete the entryNot translated` +
		`<script>libNotify('error', _t('Can\'t play: ') + e); x = _tf('%s items', n); y = _t('Other');</script>`
	if got := string(translateOutput([]byte(page), "en")); got != en {
		test.Errorf("en:\n got %s\nwant %s", got, en)
	}
}

func TestBrowserLanguage(t *testing.T) {
	loadTestLanguages(t)

	for header, want := range map[string]string{
		"de-DE,de;q=0.9,en;q=0.8": "de",
		"fr-FR,fr;q=0.9,de;q=0.5": "de",
		"en-US,en;q=0.9,de;q=0.8": "en",
		"fr":                      "en",
		"":                        "en",
	} {
		if got := browserLanguage(header); got != want {
			t.Errorf("%q: %s, want %s", header, got, want)
		}
	}
}

// testLanguagesDir holds the language files the tests read; the repo
// root's static/i18n (shipped in the image) is a copy.
const testLanguagesDir = "../static/i18n"

// loadTestLanguages loads the shipped language files.
func loadTestLanguages(test *testing.T) {
	test.Helper()
	loadLanguages(testLanguagesDir)

	if len(catalogs["de"]) == 0 {
		test.Fatal("no German texts in " + testLanguagesDir)
	}
}

// TestLoadLanguages: every file is a language, by its file name; a file
// that doesn't parse is left out, as are translations that break a rule.
func TestLoadLanguages(test *testing.T) {
	dir := test.TempDir()

	for name, body := range map[string]string{
		"xx.json":   `{"name": "Xish", "texts": {"Play": "Spielx", "Bad": "it's", "%d of %d": "%s"}}`,
		"yy.json":   `{"texts": {`,
		"en.json":   `{"name": "Other English", "texts": {"Play": "Start", "Stop": "Stop"}}`,
		"notes.txt": `ignored`,
		"Zz.json":   `{"texts": {}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			test.Fatal(err)
		}
	}

	test.Cleanup(func() { loadLanguages(testLanguagesDir) })
	loadLanguages(dir)

	if got := strings.Join(languages, ","); got != "en,xx,zz" {
		test.Errorf("languages %s, want en,xx,zz", got)
	}

	if languageNames["xx"] != "Xish" || languageNames["zz"] != "zz" || languageNames["en"] != "Other English" {
		test.Errorf("names %v", languageNames)
	}

	// en.json changes English texts; one listed as itself changes nothing.
	if want := map[string]string{"Play": "Start"}; !maps.Equal(catalogs["en"], want) {
		test.Errorf("en texts %v, want %v", catalogs["en"], want)
	}

	if got := string(translateOutput([]byte(t("Play")+t("Stop")), "en")); got != "StartStop" {
		test.Errorf("en output %q", got)
	}

	if want := map[string]string{"Play": "Spielx"}; !maps.Equal(catalogs["xx"], want) {
		test.Errorf("xx texts %v, want %v", catalogs["xx"], want)
	}
}

// TestI18nMiddleware: an HTML response comes in the browser's language,
// a video stream goes through untouched.
func TestI18nMiddleware(test *testing.T) {
	gin.SetMode(gin.TestMode)
	loadTestLanguages(test)

	r := gin.New()
	r.Use(i18nMiddleware)
	r.GET("/page", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<html lang="en">`+t("Play"))
	})
	r.GET("/video", func(c *gin.Context) {
		c.Header("Content-Type", "video/mp4")
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write([]byte(t("Play")))
	})

	for path, want := range map[string]string{"/page": `<html lang="de">Abspielen`, "/video": t("Play")} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Accept-Language", "de-DE,de;q=0.9")
		r.ServeHTTP(w, req)

		if w.Body.String() != want {
			test.Errorf("%s: %q, want %q", path, w.Body.String(), want)
		}
	}
}

// TestCatalogs: the shipped language files parse, every translation keeps
// the rules (loadLanguages would drop it), and the repo root's copy is the
// same.
func TestCatalogs(test *testing.T) {
	files, _ := filepath.Glob(filepath.Join(testLanguagesDir, "*.json"))
	if len(files) == 0 {
		test.Fatal("no language files in " + testLanguagesDir)
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			test.Fatal(err)
		}

		var lf langFile
		if err := json.Unmarshal(data, &lf); err != nil {
			test.Errorf("%s: %v", f, err)
			continue
		}

		if lf.Name == "" {
			test.Errorf("%s: no name", f)
		}

		for key, tr := range lf.Texts {
			if tr == key {
				continue
			}

			if err := trCheck(key, tr); err != nil {
				test.Errorf("%s %q: %q: %v", f, key, tr, err)
			}
		}

		root, err := os.ReadFile(filepath.Join("../../../static/i18n", filepath.Base(f)))
		if err != nil || !bytes.Equal(root, data) {
			test.Errorf("%s: the repo root's static/i18n has no identical copy", filepath.Base(f))
		}
	}
}

// trSourceFiles are the sources searched for marked texts: all of them, so
// moving code between files can't drop texts from the check (i18n.go's
// marks are examples in its comments).
var trSourceFiles = []string{"*.go"}

var trMarkRes = []*regexp.Regexp{
	regexp.MustCompile(`\btf?\("((?:[^"\\]|\\.)*)"`),
	regexp.MustCompile(`\b_tf?\('((?:[^'\\\n]|\\.)*)'`),
}

// TestCatalogsComplete: every text the pages mark is in every language
// file.
func TestCatalogsComplete(t *testing.T) {
	keys := map[string]string{}

	for _, pattern := range trSourceFiles {
		files, _ := filepath.Glob(pattern)
		for _, f := range files {
			if strings.HasSuffix(f, "_test.go") || f == "i18n.go" {
				continue
			}

			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			for _, re := range trMarkRes {
				for _, m := range re.FindAllStringSubmatch(string(src), -1) {
					key := strings.NewReplacer(`\"`, `"`, `\'`, `'`, `\\`, `\`, `\n`, "\n").Replace(m[1])
					keys[key] = f
				}
			}
		}
	}

	if len(keys) == 0 {
		t.Fatal("no marked texts found")
	}

	// The files as written: en.json lists its unchanged texts as well.
	files, _ := filepath.Glob(filepath.Join(testLanguagesDir, "*.json"))

	for _, f := range files {
		var lf langFile

		data, err := os.ReadFile(f)
		if err == nil {
			err = json.Unmarshal(data, &lf)
		}

		if err != nil {
			t.Fatal(err)
		}

		lang, missing := filepath.Base(f), 0

		for key, file := range keys {
			if _, ok := lf.Texts[key]; !ok {
				missing++
				if missing <= 50 {
					t.Errorf("%s: no translation of %q (%s)", lang, key, file)
				}
			}
		}

		if missing > 50 {
			t.Errorf("%s: %d texts without translation", lang, missing)
		}
	}
}
