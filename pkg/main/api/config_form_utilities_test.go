package api

import (
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func renderHelp(t *testing.T, text string) string {
	t.Helper()

	node := renderFieldHelp("general-TimeFormat", text)
	if node == nil {
		return ""
	}

	var b strings.Builder
	if err := node.Render(&b); err != nil {
		t.Fatal(err)
	}

	return b.String()
}

func TestRenderFieldHelp(t *testing.T) {
	out := renderHelp(t, "Specify the timestamp format.\nAvailable options:\n- 'rfc3339': 2023-01-15T14:30:45Z\n"+
		"- 'iso8601': <b>x</b>\nDefault: 'rfc3339'\nSee https://example.org/tz for zones")

	for _, want := range []string{
		`field-help-hint">Specify the timestamp format.</small>`,
		`data-bs-target="#help-general-TimeFormat"`,
		`<div class="field-help-heading">Available options:</div><ul><li>&#39;rfc3339&#39;: 2023-01-15T14:30:45Z</li>`,
		`&lt;b&gt;x&lt;/b&gt;`, // escaped
		`<p><strong>Default:</strong> &#39;rfc3339&#39;</p>`,
		`<a href="https://example.org/tz" target="_blank" rel="noopener">https://example.org/tz</a> for zones`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help lacks %s\n%s", want, out)
		}
	}

	// The first line is the hint and isn't repeated in the box.
	if strings.Count(out, "Specify the timestamp format.") != 1 {
		t.Error("first line repeated")
	}

	// One short line: no toggle. Literal \n from config comments splits lines.
	if out := renderHelp(t, "Just a hint."); strings.Contains(out, "field-help-toggle") || !strings.Contains(out, "Just a hint.") {
		t.Errorf("single line: %s", out)
	}

	if out := renderHelp(t, `First\nSecond`); !strings.Contains(out, "<p>Second</p>") {
		t.Errorf("literal \\n: %s", out)
	}

	// Several links in one line (this used to panic).
	if out := renderHelp(t, "Links:\nsee https://a.example/x and https://b.example/y now"); !strings.Contains(out,
		`see <a href="https://a.example/x" target="_blank" rel="noopener">https://a.example/x</a> and <a href="https://b.example/y" target="_blank" rel="noopener">https://b.example/y</a> now`) {
		t.Errorf("two links: %s", out)
	}

	if renderHelp(t, " \n ") != "" {
		t.Error("empty help rendered")
	}
}

func TestRenderChoiceChipsIMDbTypes(t *testing.T) {
	// The IMDb page's path: field options -> convertSelectOptionsToMap -> chips.
	field := createImdbConfigFields(&config.ImdbConfig{Indexedtypes: []string{"movie", "tvmovie"}, ImdbIDSize: 1})[0]

	var b strings.Builder
	if err := renderChoiceChips("imdb_Indexedtypes", choiceOptionsFrom(convertSelectOptionsToMap(field.Options), true),
		field.Value.([]string)).Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()

	for _, want := range []string{
		// Real value posted, readable label shown, configured value checked.
		`name="imdb_Indexedtypes" value="movie" autocomplete="off" checked>`,
		`name="imdb_Indexedtypes" value="tvMovie" autocomplete="off">`,
		`TV movie<span class="choice-chip-value">tvMovie</span>`,
		// The wrongly cased configured value stays selected and is marked.
		`value="tvmovie" autocomplete="off" checked>`,
		`btn-outline-warning`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("chips lack %s\n%s", want, out)
		}
	}

	// IMDb's order, not alphabetical.
	if strings.Index(out, `value="movie"`) > strings.Index(out, `value="tvMovie"`) ||
		strings.Index(out, `value="tvSeries"`) > strings.Index(out, `value="video"`) {
		t.Error("options not in imdbTitleTypes order")
	}
}

func TestHelperToolFields(t *testing.T) {
	for name, want := range map[string]string{
		"File Naming Template":          "naming",
		"Notification Title Template":   "notification",
		"Notification Message Template": "notification",
		"RSS Feed Cron Schedule":        "cron",
		"Required Pattern Matches":      "regex",
		"IRC Announce Regex":            "regex",
		"RSS Feed Check Interval":       "",
		"Name":                          "",
	} {
		if got := helperToolFor(name); got != want {
			t.Errorf("helperToolFor(%q) = %q, want %q", name, got, want)
		}
	}

	// The scheduler form puts the button next to every cron field.
	var b strings.Builder
	if err := renderSchedulerForm(0, &config.SchedulerConfig{Name: "Default", CronFeeds: "0 */15 * * * *"}).Render(&b); err != nil {
		t.Fatal(err)
	}

	out := b.String()
	if !strings.Contains(out, `openHelperTool(&#34;cron&#34;, &#34;scheduler_0_CronFeeds&#34;)`) {
		t.Error("CronFeeds lacks the cron generator button")
	}

	if strings.Contains(out, `scheduler_0_IntervalFeeds&#34;)`) {
		t.Error("interval field got a helper button")
	}

	// Rows of array fields pass their own input; new rows get the button too.
	b.Reset()

	if err := renderRegexForm(0, &config.RegexConfig{Name: "r", Required: []string{`x`}}).Render(&b); err != nil {
		t.Fatal(err)
	}

	rowButton := `openHelperTool('regex', this.parentElement.querySelector('input'))`
	if out := b.String(); !strings.Contains(out, rowButton) || !strings.Contains(out, "data-row-html=") ||
		strings.Count(out, "openHelperTool(&#39;regex&#39;") == 0 {
		t.Errorf("regex rows lack the helper button:\n%s", out)
	}
}
