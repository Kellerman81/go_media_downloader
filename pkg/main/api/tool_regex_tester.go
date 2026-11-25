package api

import (
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The regex tester checks a release or file name the way the app does:
//   - regex configs as the searcher's filter (filterRegexNzbs): patterns are
//     compiled as written (case-sensitive unless they contain (?i)); at least
//     one required pattern must match, no rejected pattern may match;
//   - qualities by their string lists (whole words, as the parser cuts them
//     out of a name) and, with use_regex, their regex;
//   - the parser's own scan patterns (season, episode, year, ...).

// renderRegexTesterPage renders the tester with two tabs: testing a name
// against the configured patterns, and building one pattern against sample
// names (started from pattern; the tab the overlay of regex fields opens).
func renderRegexTesterPage(csrfToken, pattern string, patternTab bool) gomponents.Node {
	regexOptions := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("All regex configs"))}

	usedBy := regexUsedBy()
	for _, rc := range config.GetSettingsRegexAll() {
		label := rc.Name
		if profiles := usedBy[rc.Name]; len(profiles) > 0 {
			label += " (" + strings.Join(profiles, ", ") + ")"
		}

		regexOptions = append(regexOptions, html.Option(html.Value(rc.Name), gomponents.Text(label)))
	}

	option := func(name, label, comment string, on bool) gomponents.Node {
		return html.Div(html.Class("col-md-6 col-xl-3"),
			renderFormGroup("rt", map[string]string{name: comment}, map[string]string{name: label}, name, "checkbox", on, nil))
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader(
			"fa-solid fa-magnifying-glass",
			"Regex Tester",
			"Paste a release or file name to see whether your regex configs accept it, which resolution, source, codec and audio "+
				"patterns match, and what the parser's own patterns pick up.",
		),
		html.Ul(
			html.Class("nav nav-tabs mb-3"), html.Role("tablist"),
			rtTab("rtTabName", "rtPaneName", "fa-file-lines", "Test a name", !patternTab),
			rtTab("rtTabPattern", "rtPanePattern", "fa-wand-magic-sparkles", "Build a pattern", patternTab),
		),
		html.Div(
			html.Class("tab-content"),
			html.Div(
				html.Class("tab-pane fade"+rtShow(!patternTab)), html.ID("rtPaneName"), html.Role("tabpanel"),
				renderRegexNameTest(csrfToken, regexOptions, option),
			),
			html.Div(
				html.Class("tab-pane fade"+rtShow(patternTab)), html.ID("rtPanePattern"), html.Role("tabpanel"),
				renderRegexPatternBuilder(csrfToken, pattern),
			),
		),
	)
}

func rtShow(active bool) string {
	if active {
		return " show active"
	}

	return ""
}

func rtTab(id, pane, icon, label string, active bool) gomponents.Node {
	class := "nav-link"
	if active {
		class += " active"
	}

	return html.Li(html.Class("nav-item"), html.Role("presentation"),
		html.Button(html.Class(class), html.ID(id), html.Type("button"), html.Role("tab"),
			gomponents.Attr("data-bs-toggle", "tab"), gomponents.Attr("data-bs-target", "#"+pane),
			gomponents.Attr("aria-controls", pane), gomponents.Attr("aria-selected", fmt.Sprint(active)),
			html.I(html.Class("fa-solid "+icon+" me-1")), gomponents.Text(label)),
	)
}

// renderRegexNameTest renders the "Test a name" tab.
func renderRegexNameTest(csrfToken string, regexOptions []gomponents.Node, option func(name, label, comment string, on bool) gomponents.Node) gomponents.Node {
	return gomponents.Group([]gomponents.Node{
		html.Div(
			html.Class("card border-0 shadow-sm mb-4"),
			html.Div(
				html.Class("card-body"),
				html.Form(
					html.ID("regexTestForm"),
					hx.Post("/api/admin/regex-tester/test"),
					hx.Target("#regexResults"),
					hx.Trigger("submit, input delay:400ms, change"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					html.Div(
						html.Class("mb-3"),
						html.Label(html.Class("form-label fw-semibold"), html.For("rtString"), gomponents.Text("Release or file name")),
						html.Textarea(
							html.Class("form-control font-monospace"),
							html.ID("rtString"),
							html.Name("test_string"),
							html.Rows("2"),
							html.Placeholder("e.g. Movie.Title.2024.German.DL.1080p.WEB.x264-GROUP"),
						),
						html.Small(html.Class("form-text text-muted"),
							gomponents.Text("Results update while you type. For the complete parse (title, year, episode, quality IDs) use "),
							html.A(html.Href("/api/admin/testparse"), gomponents.Text("Test Parsing")),
							gomponents.Text("."),
						),
					),
					html.Div(
						html.Class("mb-3"),
						html.Label(html.Class("form-label fw-semibold"), html.For("rtRegexConfig"), gomponents.Text("Regex config")),
						html.Select(html.Class("form-select"), html.ID("rtRegexConfig"), html.Name("regex_config"), gomponents.Group(regexOptions)),
						html.Small(html.Class("form-text text-muted"), gomponents.Text("In brackets: the quality profiles that use the regex config.")),
					),
					html.Div(
						html.Class("row"),
						option("TestConfig", "Regex configs", "Would the regex config accept the release?", true),
						option("TestQualities", "Qualities", "Resolution, source, codec and audio entries that match", true),
						option("TestGlobal", "Parser patterns", "Season, episode, year, IDs found by the parser", true),
						option("ShowAll", "Show non-matching", "Also list patterns that don't match", false),
					),
				),
			),
		),
		html.Div(html.ID("regexResults")),
	})
}

// rtSnippets are the building blocks the pattern builder inserts. Patterns
// are Go regular expressions (RE2): no lookahead, lookbehind or backrefs.
var rtSnippets = []struct{ Label, Insert, Title string }{
	{"(?i)", "(?i)", "Ignore case (put it at the start)"},
	{`\b`, `\b`, "Word boundary"},
	{"[ ._-]", "[ ._-]", "One separator: space, dot, underscore or dash"},
	{"(a|b)", "(|)", "Alternatives"},
	{".*", ".*", "Anything"},
	{`\d{4}`, `\d{4}`, "Four digits, e.g. a year"},
	{"^", "^", "Start of the name"},
	{"$", "$", "End of the name"},
	{"(?P<name>…)", "(?P<name>)", "Named group"},
}

// renderRegexPatternBuilder renders the "Build a pattern" tab: one pattern
// tested against sample names while typing.
func renderRegexPatternBuilder(csrfToken, pattern string) gomponents.Node {
	snippets := make([]gomponents.Node, 0, len(rtSnippets))
	for _, sn := range rtSnippets {
		snippets = append(snippets, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary font-monospace"),
			html.Title(sn.Title), html.Data("rt-insert", sn.Insert), gomponents.Text(sn.Label)))
	}

	return gomponents.Group([]gomponents.Node{
		html.Form(
			html.ID("rtPatternForm"),
			hx.Post("/api/admin/regex-tester/pattern"),
			hx.Target("#rtPatternResults"),
			hx.Trigger("load, input delay:300ms"),
			hx.Headers(createHTMXHeaders(csrfToken)),
			html.Div(
				html.Class("card border-0 shadow-sm mb-4"),
				html.Div(
					html.Class("card-body"),
					html.Label(html.Class("form-label fw-semibold"), html.For("rtPattern"), gomponents.Text("Pattern")),
					html.Input(html.Class("form-control font-monospace"), html.ID("rtPattern"), html.Name("pattern"),
						html.Value(pattern), html.Placeholder(`e.g. (?i)\b(cam|ts|telesync)\b`), gomponents.Attr("spellcheck", "false")),
					html.Div(html.Class("d-flex flex-wrap gap-1 mt-2"), gomponents.Group(snippets)),
					html.Small(html.Class("form-text text-muted d-block mt-2"),
						gomponents.Text("Go regular expressions (RE2): matching is case-sensitive unless the pattern starts with (?i); "+
							"lookahead, lookbehind and backreferences are not supported.")),
					html.Label(html.Class("form-label fw-semibold mt-3"), html.For("rtSamples"), gomponents.Text("Sample names (one per line)")),
					html.Textarea(html.Class("form-control font-monospace small"), html.ID("rtSamples"), html.Name("samples"),
						html.Rows("6"), gomponents.Attr("spellcheck", "false"), gomponents.Text(strings.Join(rtSampleNames(), "\n"))),
					html.Small(html.Class("form-text text-muted"), gomponents.Text("Prefilled with recent releases from the download history.")),
				),
			),
		),
		html.Div(html.ID("rtPatternResults")),
		html.Script(gomponents.Raw(`
document.addEventListener('click', function (e) {
	var b = e.target.closest && e.target.closest('[data-rt-insert]');
	if (!b) { return; }
	var t = document.getElementById('rtPattern'), v = b.getAttribute('data-rt-insert');
	var start = t.selectionStart, end = t.selectionEnd;
	if (v === '(?i)') { if (t.value.indexOf('(?i)') !== 0) { t.value = '(?i)' + t.value; } }
	else { t.value = t.value.slice(0, start) + v + t.value.slice(end); t.setSelectionRange(start + v.length, start + v.length); }
	t.focus();
	t.dispatchEvent(new Event('input', { bubbles: true }));
});`)),
	})
}

// rtSampleNames returns recent release names from the download history, or
// fixed examples when there are none.
func rtSampleNames() []string {
	var names []string

	for _, table := range []string{"movie_histories", "serie_episode_histories"} {
		names = append(names, database.GetrowsN[string](false, 4, "select title from "+table+" order by id desc limit 4")...)
	}

	if len(names) == 0 {
		names = []string{
			"Movie.Title.2024.German.DL.1080p.WEB.x264-GROUP",
			"Series.Name.S01E02.720p.HDTV.x264-GROUP",
			"Movie.Title.2023.CAM.x264-GROUP",
		}
	}

	return names
}

// HandleRegexPattern tests the posted pattern against the sample names.
func HandleRegexPattern(ctx *gin.Context) {
	pattern := ctx.PostForm("pattern")
	if strings.TrimSpace(pattern) == "" {
		ctx.String(http.StatusOK, renderComponentToString(
			html.P(html.Class("text-muted"), gomponents.Text("Type a pattern to see which sample names it matches."))))

		return
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		ctx.String(http.StatusOK, renderComponentToString(html.Div(html.Class("alert alert-danger"),
			html.I(html.Class("fa-solid fa-triangle-exclamation me-2")), gomponents.Text("Invalid pattern: "+err.Error()))))

		return
	}

	var (
		rows    []gomponents.Node
		matched int
	)

	names := re.SubexpNames()

	for line := range strings.SplitSeq(ctx.PostForm("samples"), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		idx := re.FindStringSubmatchIndex(line)
		if idx == nil {
			rows = append(rows, html.Tr(
				html.Td(rtBadge("bg-secondary", "no match")),
				html.Td(html.Code(html.Class("text-muted"), gomponents.Text(line))),
				html.Td(),
			))

			continue
		}

		matched++

		var groups []gomponents.Node

		for g := 1; 2*g+1 < len(idx); g++ {
			if idx[2*g] < 0 {
				continue
			}

			label := fmt.Sprint(g)
			if names[g] != "" {
				label = names[g]
			}

			groups = append(groups, html.Span(html.Class("badge bg-light text-dark border me-1 font-monospace"),
				gomponents.Text(label+": "+line[idx[2*g]:idx[2*g+1]])))
		}

		rows = append(rows, html.Tr(
			html.Td(rtBadge("bg-success", "match")),
			html.Td(rtHighlight(line, idx[0], idx[1])),
			html.Td(gomponents.Group(groups)),
		))
	}

	ctx.String(http.StatusOK, renderComponentToString(rtCard(
		fmt.Sprintf("Matches %d of %d sample names", matched, len(rows)), "fa-list-check",
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(html.Style("width:7rem")), html.Th(gomponents.Text("Name")), html.Th(gomponents.Text("Groups")))),
			html.TBody(gomponents.Group(rows)),
		)),
	)))
}

// regexUsedBy maps regex config names to the quality profiles that use them.
func regexUsedBy() map[string][]string {
	out := make(map[string][]string)

	config.RangeSettingsQuality(func(name string, q *config.QualityConfig) {
		for i := range q.Indexer {
			r := q.Indexer[i].TemplateRegex
			if r != "" && !slices.Contains(out[r], name) {
				out[r] = append(out[r], name)
			}
		}
	})

	for k := range out {
		slices.Sort(out[k])
	}

	return out
}

// rtMatch is one pattern tested against the input.
type rtMatch struct {
	Pattern    string
	Start, End int    // matched span, -1 if no match
	Value      string // captured value, if the pattern has one
	Err        string
}

func (m *rtMatch) matched() bool { return m.Start >= 0 }

// rtFind matches pattern (compiled as written) against s. With last, the last
// match counts; group selects the captured value (0 = none).
func rtFind(pattern, s string, last bool, group int) rtMatch {
	m := rtMatch{Pattern: pattern, Start: -1, End: -1}

	re, err := regexp.Compile(pattern)
	if err != nil {
		m.Err = err.Error()
		return m
	}

	all := re.FindAllStringSubmatchIndex(s, -1)
	if len(all) == 0 {
		return m
	}

	idx := all[0]
	if last {
		idx = all[len(all)-1]
	}

	m.Start, m.End = idx[0], idx[1]
	if group > 0 && len(idx) > 2*group+1 && idx[2*group] >= 0 {
		m.Value = s[idx[2*group]:idx[2*group+1]]
	}

	return m
}

// rtWordIndex finds word as a whole word in s, case-insensitively: not
// directly preceded or followed by a letter or digit, as the parser cuts
// quality tokens out of a name.
func rtWordIndex(s, word string) (int, int) {
	if word == "" {
		return -1, -1
	}

	lower := strings.ToLower(s)
	for from := 0; from < len(lower); {
		i := strings.Index(lower[from:], word)
		if i < 0 {
			return -1, -1
		}

		start, end := from+i, from+i+len(word)
		if (start == 0 || !rtAlnum(lower[start-1])) && (end == len(lower) || !rtAlnum(lower[end])) {
			return start, end
		}

		from = start + 1
	}

	return -1, -1
}

func rtAlnum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// rtHighlight shows s with the span [start, end) marked.
func rtHighlight(s string, start, end int) gomponents.Node {
	if start < 0 || end > len(s) || start >= end {
		return html.Code(gomponents.Text(s))
	}

	return html.Code(
		html.Class("text-body"),
		gomponents.Text(s[:start]),
		html.Mark(html.Class("px-0"), gomponents.Text(s[start:end])),
		gomponents.Text(s[end:]),
	)
}

func rtBadge(class, text string) gomponents.Node {
	return html.Span(html.Class("badge "+class), gomponents.Text(text))
}

func rtCard(title, icon string, body ...gomponents.Node) gomponents.Node {
	return html.Div(
		html.Class("card border-0 shadow-sm mb-4"),
		html.Div(html.Class("card-header bg-transparent fw-semibold"),
			html.I(html.Class("fa-solid "+icon+" me-2 text-primary")), gomponents.Text(title)),
		html.Div(html.Class("card-body"), gomponents.Group(body)),
	)
}

// HandleRegexTesting renders the results for the posted name.
func HandleRegexTesting(ctx *gin.Context) {
	s := strings.TrimSpace(ctx.PostForm("test_string"))
	if s == "" {
		ctx.String(http.StatusOK, renderComponentToString(
			html.P(html.Class("text-muted"), gomponents.Text("Enter a release or file name to test."))))

		return
	}

	showAll := ctx.PostForm("rt_ShowAll") == "on"

	var sections []gomponents.Node

	if ctx.PostForm("rt_TestConfig") == "on" {
		sections = append(sections, rtRegexConfigs(s, ctx.PostForm("regex_config"), showAll))
	}

	if ctx.PostForm("rt_TestQualities") == "on" {
		sections = append(sections, rtQualities(s))
	}

	if ctx.PostForm("rt_TestGlobal") == "on" {
		sections = append(sections, rtParserPatterns(s, showAll))
	}

	if len(sections) == 0 {
		sections = append(sections, html.P(html.Class("text-muted"), gomponents.Text("Select at least one kind of pattern to test.")))
	}

	ctx.String(http.StatusOK, renderComponentToString(gomponents.Group(sections)))
}

// rtRegexConfigs shows each regex config's verdict for s.
func rtRegexConfigs(s, only string, showAll bool) gomponents.Node {
	usedBy := regexUsedBy()
	rows := []gomponents.Node{}

	for _, rc := range config.GetSettingsRegexAll() {
		if only != "" && rc.Name != only {
			continue
		}

		var required, rejected []rtMatch
		for _, p := range rc.Required {
			if p != "" {
				required = append(required, rtFind(p, s, false, 0))
			}
		}

		for _, p := range rc.Rejected {
			if p != "" {
				rejected = append(rejected, rtFind(p, s, false, 0))
			}
		}

		verdict := rtBadge("bg-success", "Accepted")
		var reason []gomponents.Node

		matchedRequired := slices.IndexFunc(required, func(m rtMatch) bool { return m.matched() })
		matchedRejected := slices.IndexFunc(rejected, func(m rtMatch) bool { return m.matched() })

		switch {
		case len(required) > 0 && matchedRequired < 0:
			verdict = rtBadge("bg-danger", "Rejected")
			reason = append(reason, html.Div(gomponents.Text("No required pattern matches.")))
		case matchedRejected >= 0:
			m := rejected[matchedRejected]
			verdict = rtBadge("bg-danger", "Rejected")
			reason = append(reason,
				html.Div(gomponents.Text("Rejected by "), html.Code(gomponents.Text(m.Pattern))),
				html.Div(rtHighlight(s, m.Start, m.End)),
			)
		case matchedRequired >= 0:
			m := required[matchedRequired]
			reason = append(reason,
				html.Div(gomponents.Text("Required "), html.Code(gomponents.Text(m.Pattern)), gomponents.Text(" matches")),
				html.Div(rtHighlight(s, m.Start, m.End)),
			)
		default:
			reason = append(reason, html.Div(html.Class("text-muted"), gomponents.Text("No rejected pattern matches.")))
		}

		for _, list := range [][]rtMatch{required, rejected} {
			for _, m := range list {
				if m.Err != "" {
					reason = append(reason, html.Div(html.Class("text-danger small"),
						gomponents.Text("Invalid pattern "), html.Code(gomponents.Text(m.Pattern)), gomponents.Text(": "+m.Err)))
				}
			}
		}

		if showAll {
			reason = append(reason, rtPatternList("Required", required, s), rtPatternList("Rejected", rejected, s))
		}

		profiles := strings.Join(usedBy[rc.Name], ", ")
		if profiles == "" {
			profiles = "—"
		}

		// The theme centres table cells; long "why" cells need the name on top.
		top := html.Style("vertical-align: top;")
		rows = append(rows, html.Tr(
			html.Td(top, html.Strong(gomponents.Text(rc.Name))),
			html.Td(top, html.Class("small"), gomponents.Text(profiles)),
			html.Td(top, verdict),
			html.Td(top, html.Class("small"), gomponents.Group(reason)),
		))
	}

	if len(rows) == 0 {
		return rtCard("Regex Configs", "fa-filter", html.P(html.Class("text-muted mb-0"), gomponents.Text("No regex configs.")))
	}

	return rtCard("Regex Configs", "fa-filter",
		html.Div(html.Class("table-responsive"),
			html.Table(html.Class("table table-sm align-top mb-2"),
				html.THead(html.Tr(
					html.Th(gomponents.Text("Regex config")), html.Th(gomponents.Text("Used by")),
					html.Th(gomponents.Text("Result")), html.Th(gomponents.Text("Why")),
				)),
				html.TBody(gomponents.Group(rows)),
			)),
		html.Small(html.Class("text-muted"), gomponents.Text(
			"As in a search: at least one required pattern must match and no rejected pattern may match. "+
				"A search also ignores a rejected pattern that matches the wanted title itself, which can't be checked here.")),
	)
}

// rtPatternList lists every pattern of a regex config with its result.
func rtPatternList(label string, list []rtMatch, s string) gomponents.Node {
	if len(list) == 0 {
		return gomponents.Group(nil)
	}

	items := make([]gomponents.Node, 0, len(list))
	for _, m := range list {
		state := rtBadge("bg-secondary", "no match")
		detail := gomponents.Node(gomponents.Group(nil))

		switch {
		case m.Err != "":
			state = rtBadge("bg-warning text-dark", "invalid")
		case m.matched():
			state = rtBadge("bg-info text-dark", "match")
			detail = html.Div(rtHighlight(s, m.Start, m.End))
		}

		items = append(items, html.Li(html.Class("mb-1"), state, gomponents.Text(" "), html.Code(gomponents.Text(m.Pattern)), detail))
	}

	return html.Details(html.Class("mt-2"),
		html.Summary(gomponents.Text(fmt.Sprintf("%s patterns (%d)", label, len(list)))),
		html.Ul(html.Class("list-unstyled ms-2 mt-1"), gomponents.Group(items)),
	)
}

// rtQualities shows which resolution, source, codec, audio and audio format
// entries match s, by string or by regex.
func rtQualities(s string) gomponents.Node {
	stringMatch := true
	if g := config.GetSettingsGeneral(); g != nil && g.DisableParserStringMatch {
		stringMatch = false
	}

	groups := []struct {
		label   string
		entries []database.Qualities
	}{
		{"Resolution", database.DBConnect.GetresolutionsIn},
		{"Source", database.DBConnect.GetqualitiesIn},
		{"Video codec", database.DBConnect.GetcodecsIn},
		{"Audio codec", database.DBConnect.GetaudiosIn},
		{"Audio format", database.DBConnect.GetaudioformatsIn},
	}

	rows := make([]gomponents.Node, 0, len(groups))
	for _, g := range groups {
		var found []gomponents.Node

		for i := range g.entries {
			q := &g.entries[i]

			if stringMatch {
				for _, word := range q.StringsLowerSplitted {
					if start, end := rtWordIndex(s, word); start >= 0 {
						found = append(found, html.Div(rtBadge("bg-primary", q.Name),
							html.Span(html.Class("small text-muted mx-2"), gomponents.Text("string “"+word+"”")), rtHighlight(s, start, end)))

						break
					}
				}
			}

			// The parser scans the whole name with these, keeping regexgroup.
			if q.UseRegex && q.Regex != "" {
				if m := rtFind(q.Regex, s, false, q.Regexgroup); m.matched() {
					how := "regex"
					if m.Value != "" {
						how += " → “" + m.Value + "”"
					}

					found = append(found, html.Div(rtBadge("bg-primary", q.Name),
						html.Span(html.Class("small text-muted mx-2"), gomponents.Text(how)), rtHighlight(s, m.Start, m.End)))
				} else if m.Err != "" {
					found = append(found, html.Div(html.Class("text-danger small"),
						gomponents.Text(q.Name+": invalid regex: "+m.Err)))
				}
			}
		}

		var cell gomponents.Node = html.Span(html.Class("text-muted"), gomponents.Text("nothing found"))
		if len(found) > 0 {
			cell = gomponents.Group(found)
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("fw-semibold text-nowrap"), gomponents.Text(g.label)),
			html.Td(cell),
		))
	}

	note := "Entries are listed in the order the parser checks them; for a name part, the first match wins."
	if !stringMatch {
		note += " String matching is switched off (disable_parser_string_match), so only regex entries count."
	}

	return rtCard("Qualities", "fa-star",
		html.Table(html.Class("table table-sm align-top mb-2"), html.TBody(gomponents.Group(rows))),
		html.Small(html.Class("text-muted"), gomponents.Text(note)),
	)
}

// rtParserPatterns shows what the parser's own scan patterns find in s.
func rtParserPatterns(s string, showAll bool) gomponents.Node {
	rows := []gomponents.Node{}

	for _, p := range parser.GlobalScanPatterns() {
		m := rtFind(p.Regex, s, p.Last, p.Group)
		if !m.matched() && !showAll {
			continue
		}

		var value, match gomponents.Node = html.Span(html.Class("text-muted"), gomponents.Text("no match")), gomponents.Group(nil)
		if m.matched() {
			value = html.Strong(gomponents.Text(m.Value))
			match = rtHighlight(s, m.Start, m.End)
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("fw-semibold"), gomponents.Text(p.Name)),
			html.Td(value),
			html.Td(match),
		))
	}

	if len(rows) == 0 {
		return rtCard("Parser Patterns", "fa-code", html.P(html.Class("text-muted mb-0"), gomponents.Text("No parser pattern matches.")))
	}

	return rtCard("Parser Patterns", "fa-code",
		html.Table(html.Class("table table-sm align-top mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Pattern")), html.Th(gomponents.Text("Value")), html.Th(gomponents.Text("Match")))),
			html.TBody(gomponents.Group(rows)),
		),
	)
}
