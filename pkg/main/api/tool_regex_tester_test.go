package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

func TestRtWordIndex(t *testing.T) {
	for _, tc := range []struct {
		s, word    string
		start, end int
	}{
		{"Movie.2024.German.WEB.x264", "web", 18, 21},
		{"Movie.2024.WEBRip.x264", "web", -1, -1}, // part of a longer word
		{"Movie.WEB2.WEB", "web", 11, 14},         // first occurrence is followed by a digit
		{"web-dl", "web", 0, 3},
		{"x", "", -1, -1},
	} {
		if start, end := rtWordIndex(tc.s, tc.word); start != tc.start || end != tc.end {
			t.Errorf("rtWordIndex(%q, %q) = %d, %d; want %d, %d", tc.s, tc.word, start, end, tc.start, tc.end)
		}
	}
}

func TestRtFind(t *testing.T) {
	// The year pattern takes the last match, group 2 is the value.
	m := rtFind(`(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, "2012.Remake.2009.1080p", true, 2)
	if !m.matched() || m.Value != "2009" {
		t.Errorf("year = %+v, want 2009", m)
	}

	// Compiled as written: case-sensitive without (?i).
	if m := rtFind(`German`, "movie.german.1080p", false, 0); m.matched() {
		t.Error("pattern without (?i) matched case-insensitively")
	}

	if m := rtFind(`(`, "x", false, 0); m.Err == "" {
		t.Error("invalid pattern: no error")
	}
}

func postRegexTest(t *testing.T, form url.Values) string {
	t.Helper()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/admin/regex-tester/test", strings.NewReader(form.Encode()))
	ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	HandleRegexTesting(ctx)

	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}

	return w.Body.String()
}

func TestHandleRegexTesting(t *testing.T) {
	useTestConfig(t, config.MainConfig{
		Regex: []config.RegexConfig{
			{Name: "de", Required: []string{`(?i)(\b|_)ger(man)?(\b|_)`}, Rejected: []string{`(?i)(\b|_)cam(\b|_)`}},
			{Name: "en", Rejected: []string{`(?i)(\b|_)sample(\b|_)`}},
		},
		Quality: []config.QualityConfig{{Name: "SD", Indexer: []config.QualityIndexerConfig{{TemplateRegex: "de"}}}},
		Media:   config.MediaConfig{Movies: mediaGroup("EN")},
	})

	prev := database.DBConnect
	t.Cleanup(func() { database.DBConnect = prev })

	database.DBConnect.GetresolutionsIn = []database.Qualities{
		{ID: 1, Name: "1080p", StringsLowerSplitted: []string{"1080p", "1080"}},
	}
	database.DBConnect.GetqualitiesIn = []database.Qualities{
		{ID: 2, Name: "webdl", StringsLowerSplitted: []string{"webdl", "web-dl"}},
		{ID: 3, Name: "web", UseRegex: true, Regex: `(?i)(\b|_)(web)(\b|_)`, Regexgroup: 2},
	}
	database.DBConnect.GetcodecsIn, database.DBConnect.GetaudiosIn, database.DBConnect.GetaudioformatsIn = nil, nil, nil

	all := url.Values{"rt_TestConfig": {"on"}, "rt_TestQualities": {"on"}, "rt_TestGlobal": {"on"}}

	all.Set("test_string", "Film.2024.German.1080p.WEB.x264-GRP")
	body := postRegexTest(t, all)

	for _, want := range []string{
		`<strong>de</strong></td><td style="vertical-align: top;" class="small">SD</td><td style="vertical-align: top;"><span class="badge bg-success">Accepted</span>`,
		`<strong>en</strong></td><td style="vertical-align: top;" class="small">—</td><td style="vertical-align: top;"><span class="badge bg-success">Accepted</span>`,
		"1080p</span><span class=\"small text-muted mx-2\">string “1080p”",
		"regex → “WEB”",
		"<td class=\"fw-semibold\">year</td><td><strong>2024</strong>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("accepted name: body lacks %s", want)
		}
	}

	if strings.Contains(body, "webdl</span>") {
		t.Error("webdl matched although the name has no webdl/web-dl word")
	}

	all.Set("test_string", "Film.2024.English.CAM.x264")
	body = postRegexTest(t, all)

	if !strings.Contains(body, "No required pattern matches.") {
		t.Error("de: missing required not reported")
	}

	all.Set("test_string", "Film.2024.German.CAM.x264")
	all.Set("regex_config", "de")
	body = postRegexTest(t, all)

	if !strings.Contains(body, "Rejected by ") || strings.Contains(body, "<strong>en</strong>") {
		t.Errorf("de only, rejected by cam:\n%s", body)
	}

	if body := postRegexTest(t, url.Values{"test_string": {" "}}); !strings.Contains(body, "Enter a release") {
		t.Error("empty input")
	}
}

func TestHandleRegexPattern(t *testing.T) {
	gin.SetMode(gin.TestMode)

	post := func(pattern, samples string) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		form := url.Values{"pattern": {pattern}, "samples": {samples}}
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		HandleRegexPattern(c)

		return w.Body.String()
	}

	out := post(`(?i)\b(?P<src>cam|ts)\b`, "Movie.2024.CAM.x264-GRP\r\nMovie.2024.1080p.WEB-GRP\n\n")
	for _, want := range []string{
		"Matches 1 of 2 sample names",
		`Movie.2024.<mark class="px-0">CAM</mark>.x264-GRP`,
		"src: CAM",
		"no match",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("result lacks %s:\n%s", want, out)
		}
	}

	if out := post(`(?<=x)y`, "xy"); !strings.Contains(out, "Invalid pattern") {
		t.Errorf("lookbehind accepted: %s", out)
	}

	if out := post(" ", "x"); !strings.Contains(out, "Type a pattern") {
		t.Errorf("empty pattern: %s", out)
	}
}

func TestRenderRegexTesterPagePatternTab(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	// An empty history gives the fixed examples.
	var b strings.Builder
	if err := renderRegexTesterPage("tok", `\bCAM\b`, true).Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()
	for _, want := range []string{
		`id="rtPattern" name="pattern" value="\bCAM\b"`,
		`class="tab-pane fade show active" id="rtPanePattern"`,
		`class="tab-pane fade" id="rtPaneName"`,
		`hx-post="/api/admin/regex-tester/pattern"`,
		"Movie.Title.2023.CAM.x264-GROUP",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}
