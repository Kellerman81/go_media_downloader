package api

// setup_wizard_recommend.go holds what the wizard's Finalize step suggests per
// media group: naming, release language with its regex rules, quality search
// settings and a schedule. The values come from a configuration that has run
// for years (personal choices like a resolution cutoff left out); the user
// edits them in the step, with the naming generator, regex tester and cron
// generator next to the fields.

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"regexp"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// wizardLangs are the release languages a video group can prefer.
var wizardLangs = [][2]string{{"any", "Any language"}, {"en", "English"}, {"de", "German"}}

// Regex building blocks.
const (
	reJunk     = `(?i)(\b|_)(bonus|trailer|sample)(\b|_)`
	reExtras   = `(?i)(\b|_)extra(s)?(\b|_)`
	reSpam     = `(?i)(\b|_)(imageset|getb8|1bet|buymore)(\b|_)`
	reDubbed   = `(?i)(\b|_)(ac3)?md(\b|_)`
	reMic      = `(?i)(\b|_)mic(dubbed)?(\b|_)`
	reSplit    = `(?i)(\b|_)(cd|disc|disk|dvd|part)[ .\-)]?(1|2|3|4|5|9|r)(\b|_)`
	reCam      = `(?i)(\b|_)cam(rip)?(\b|_)`
	reScreener = `(?i)(\b|_)(webscr|screener|webscreener|ts|r5|hdcam|hqcam)(\b|_)`
	reSeason   = `(?i)(\b|_)season( )?[0-9]+(\b|_)`
	reEpisode  = `(?i)(\b|_)s[0-9]+e[0-9]+(\b|_)`
	reSubbed   = `(?i)(\b|_)sub(s|bed)?(\b|_)`
	reGerman   = `(?i)(\b|_)ger(man)?(\b|_)`
	reGerman2  = `(?i)(\b|_)(deu|und|wie|der|das|ein|eine)(tsch)?(\b|_)`
)

// wizardForeign rejects releases in other languages than English and German.
var wizardForeign = []string{
	`(?i)(\b|_)(sub)?(fre|fr|truefre)(nch|sub|subs)?(\b|_)`,
	`(?i)(\b|_)ita(lian)?(\b|_)`,
	`(?i)(\b|_)spa(nish)?(\b|_)`,
	`(?i)(\b|_)rus(sian)?(\b|_)`,
	`(?i)(\b|_)kor(ean|sub|subs)?(\b|_)`,
	`(?i)(\b|_)(dan|dk|pol)(ish|sub|subs)?(\b|_)`,
	`(?i)(\b|_)swe(dish|sub|subs)?(\b|_)`,
	`(?i)(\b|_)heb(rew|sub|subs|dub)?(\b|_)`,
	`(?i)(\b|_)(tur|tr)(kish)?(\b|_)`,
	`(?i)(\b|_)(dut|nl)(ch|sub|subs)?(\b|_)`,
	`(?i)(\b|_)(castellano|tamil|hindi|nordic|japanese|chinese)(sub|subs)?(\b|_)`,
}

// wizardRecommendedRegex is the regex suggestion for a group: required (at
// least one must match) and rejected (none may match) patterns.
func wizardRecommendedRegex(key, lang string) (required, rejected []string) {
	switch key {
	case "music":
		return nil, []string{reJunk, reSpam, `(?i)(\b|_)karaoke(\b|_)`}
	case "audiobooks", "books":
		return nil, []string{reJunk, reSpam}
	}

	rejected = []string{reJunk, reExtras, reSpam, reDubbed, reMic, reSplit, reCam, reScreener}
	if key == "movies" {
		// A movie release naming a season or episode is a series.
		rejected = append(rejected, reSeason, reEpisode)
	}

	switch lang {
	case "en":
		rejected = append(append(rejected, wizardForeign...), reSubbed, reGerman, reGerman2)
	case "de":
		required = []string{reGerman, reGerman2}
		rejected = append(append(rejected, wizardForeign...), reSubbed)
	}

	return required, rejected
}

// wizardRecommendedNaming is the naming suggestion. Field names follow
// structure.parsertype (the struct the template runs against).
func wizardRecommendedNaming(key, lang string) string {
	german := map[bool]string{true: " German"}[lang == "de"]

	switch key {
	case "movies":
		return `{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}})` + german +
			` [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}{{if eq .Source.Extended true}} extended{{end}}] ({{.Source.Imdb}})`
	case "series":
		return `{{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}/{{.Dbserie.Seriename}} - S{{printf "%02s" .DbserieEpisode.Season}}{{range .Episodes}}E{{printf "%02d" . }}{{end}} - {{.DbserieEpisode.Title}}` +
			map[bool]string{true: " -"}[german != ""] + german +
			` [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}] ({{.Source.Tvdb}})`
	case "music":
		// Letter folder, compilations under "Various Artists", disc prefix
		// only for multi-disc albums.
		return `{{if .AlbumArtist.Name}}{{firstLetter .AlbumArtist.Name}}{{else}}_{{end}}/{{if or (eq .AlbumArtist.Name "Various Artists") (eq .AlbumArtist.Name "VA") (eq .AlbumArtist.Name "Various")}}Various Artists{{else}}{{.AlbumArtist.Name}}{{end}}/{{.Dbalbum.Title}}{{if .Dbalbum.Year}} ({{.Dbalbum.Year}}){{end}}/{{if gt .TotalDiscs 1}}{{printf "%02d" .Dbtrack.DiscNumber}}-{{end}}{{printf "%02d" .Track}} - {{.Title}}`
	case "audiobooks":
		// Letter folder, author, book series and position, ASIN.
		return `{{if .Author.Name}}{{firstLetter .Author.Name}}{{else}}_{{end}}/{{if .Author.Name}}{{.Author.Name}}{{else}}{{.Dbaudiobook.Title}}{{end}}{{if .BookSeries.Name}}/{{.BookSeries.Name}}{{end}}/{{if and .BookSeries.Name .Dbbook.SeriesPosition}}{{.Dbbook.SeriesPosition}} - {{end}}{{.Dbaudiobook.Title}}{{if .Dbaudiobook.ASIN}} ({{.Dbaudiobook.ASIN}}){{end}}/{{if gt .TotalDiscs 1}}{{.Disc}}-{{end}}{{.Track}} - {{.Title}}`
	case "books":
		return `{{.Author.Name}}/{{.Dbbook.Title}} ({{.Dbbook.Year}})/{{.Dbbook.Title}}`
	}

	return ""
}

// wizardRecommendedQuality is the quality suggestion: what to download and
// how to search (title searches as backup, title/year checks, suffixes to
// strip). Music, audiobooks and books have no IDs indexers know, so they are
// searched by title.
func wizardRecommendedQuality(key, lang string) config.QualityConfig {
	switch key {
	case "music":
		return config.QualityConfig{
			WantedAudioFormats: []string{"flac", "mp3", "aac", "ogg", "alac", "wma"}, PreferLossless: true,
			UseForPriorityAudioFormat: true, UseForPriorityAudioBitrate: true,
			SearchForTitleIfEmpty: true, BackupSearchForTitle: true, CheckTitle: true,
			TitleStripSuffixForSearch: []string{"remaster", "deluxe", "edition"},
		}
	case "audiobooks":
		return config.QualityConfig{
			WantedAudioFormats:        []string{"m4b", "mp3", "flac", "aac", "ogg"},
			UseForPriorityAudioFormat: true, UseForPriorityAudioBitrate: true,
			SearchForTitleIfEmpty: true, BackupSearchForTitle: true, CheckTitle: true, ExcludeYearFromTitleSearch: true,
			TitleStripSuffixForSearch: []string{"unabridged", "abridged", "audiobook"},
		}
	case "books":
		return config.QualityConfig{SearchForTitleIfEmpty: true, BackupSearchForTitle: true, CheckTitle: true}
	}

	q := config.QualityConfig{
		WantedResolution:            []string{"480p", "576p", "720p", "1080p"},
		WantedQuality:               []string{"sdtv", "hdtv", "webdl", "web-dl", "web", "webrip", "dvdrip", "bluray"},
		CutoffResolution:            "1080p",
		CutoffQuality:               "bluray",
		UseForPriorityResolution:    true,
		UseForPriorityQuality:       true,
		UseForPriorityMinDifference: 20,
		BackupSearchForTitle:        true,
		CheckTitle:                  true,
		CheckYear:                   true,
		CheckYear1:                  true,
		TitleStripSuffixForSearch:   []string{" dc", "repack", "unrated", "directors cut", "theatrical", "remastered", "uncut"},
	}

	if lang == "de" {
		// German titles differ from the original: alternate titles help.
		q.TitleStripSuffixForSearch = append(q.TitleStripSuffixForSearch, "german")
		q.BackupSearchForAlternateTitle = true
		q.SearchForAlternateTitleIfEmpty = true
		q.ExcludeYearFromTitleSearch = true
	}

	return q
}

// wizardSchedule is the part of a schedule the Finalize step shows.
type wizardSchedule struct {
	RSS, Missing, Upgrade, Scan string
}

var wizardScheduleFields = []struct{ key, label, hint string }{
	{"rss", "New releases (RSS)", "Checks the indexers' newest releases"},
	{"missing", "Search missing", "Searches what is wanted and not there yet"},
	{"upgrade", "Search upgrades", "Searches better versions until the cutoff"},
	{"scan", "Scan library", "Picks up files added or removed outside the app"},
}

func (s wizardSchedule) get(key string) string {
	return map[string]string{"rss": s.RSS, "missing": s.Missing, "upgrade": s.Upgrade, "scan": s.Scan}[key]
}

// wizardRecommendedSchedule is the schedule suggestion (cron with seconds).
// Searches are spread over the hour so groups don't search at the same time.
func wizardRecommendedSchedule(key string) wizardSchedule {
	switch key {
	case "movies":
		return wizardSchedule{RSS: "0 */15 * * * *", Missing: "0 */30 * * * *", Upgrade: "0 */45 * * * *", Scan: "0 0 * * * *"}
	case "series":
		return wizardSchedule{RSS: "0 */15 * * * *", Missing: "0 */33 * * * *", Upgrade: "0 */50 * * * *", Scan: "0 0 * * * *"}
	}

	return wizardSchedule{RSS: "0 */15 * * * *", Missing: "0 */33 * * * *", Upgrade: "0 */50 * * * *", Scan: "0 0 0 */2 * *"}
}

// wizardBuildScheduler turns a group's schedule into a scheduler: the shown
// jobs plus the ones that belong to the media type - title searches for
// series, music and books, season RSS for series, artist/author RSS for
// music and books, list refreshes and full searches.
func wizardBuildScheduler(key, name string, s wizardSchedule) config.SchedulerConfig {
	sc := config.SchedulerConfig{
		Name:                name,
		CronIndexerRss:      s.RSS,
		CronScanData:        s.Scan,
		CronScanDataMissing: "0 0 5 */3 * *",
		CronScanDataimport:  "0 */15 * * * *",
		CronFeeds:           "0 0 */8 * * *",
	}

	switch key {
	case "movies":
		// Movies are found by their IMDb ID.
		sc.CronIndexerMissing, sc.CronIndexerUpgrade = s.Missing, s.Upgrade
		sc.CronIndexerMissingFull, sc.CronIndexerUpgradeFull = "0 0 8 2,18 * *", "0 0 8 3 * *"
		sc.CronImdb = "0 0 6 */3 * *"
		sc.CronFeedsRefreshMovies, sc.CronFeedsRefreshMoviesFull = "0 0 */8 * * *", "0 0 8 1,11,30 * *"
	case "series":
		sc.CronIndexerMissingTitle, sc.CronIndexerUpgradeTitle = s.Missing, s.Upgrade
		sc.CronIndexerMissingFullTitle, sc.CronIndexerUpgradeFullTitle = "0 0 8 5,21 * *", "0 0 8 4 * *"
		sc.CronIndexerRssSeasons = "0 0 */2 * * *"
		sc.CronFeedsRefreshSeries, sc.CronFeedsRefreshSeriesFull = "0 0 */12 * * *", "0 0 2 */7 * *"
	default:
		sc.CronIndexerMissingTitle, sc.CronIndexerUpgradeTitle = s.Missing, s.Upgrade
		sc.CronIndexerMissingFullTitle, sc.CronIndexerUpgradeFullTitle = "0 0 8 5,21 * *", "0 0 8 4 * *"
		sc.CronFeeds = "0 0 1 * * *"
		sc.CronScanDataimport = "0 0 */8 * * *"

		if key == "music" {
			sc.CronIndexerRssArtists, sc.CronIndexerRssArtistsUpgrade = "0 0 * * * *", "0 0 * * * *"
		} else {
			sc.CronIndexerRssAuthors, sc.CronIndexerRssAuthorsUpgrade = "0 0 * * * *", "0 0 * * * *"
		}
	}

	return sc
}

// wizardCronParser accepts what the scheduler accepts (worker.go).
var wizardCronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// wizardCheckPatterns lists the regexes that don't compile.
func wizardCheckPatterns(patterns []string) []string {
	var bad []string

	for _, p := range patterns {
		if _, err := regexp.Compile(p); err != nil {
			bad = append(bad, p)
		}
	}

	return bad
}

// wizardRecommendJSON is the suggestion per language for the step's script,
// which swaps naming and patterns when the language changes.
func wizardRecommendJSON(key string) string {
	type rec struct {
		Naming   string   `json:"naming"`
		Required []string `json:"required"`
		Rejected []string `json:"rejected"`
		Cats     string   `json:"cats"`
		DlCat    string   `json:"dlcat"`
	}

	out := map[string]rec{}
	for _, l := range wizardLangs {
		req, rej := wizardRecommendedRegex(key, l[0])
		out[l[0]] = rec{Naming: wizardRecommendedNaming(key, l[0]), Required: req, Rejected: rej,
			Cats: wizardRecommendedCategories(key, l[0]), DlCat: wizardRecommendedDlCategory(key, l[0])}
	}

	b, _ := json.Marshal(out)

	return string(b)
}

// wizardDefaultLang is the language suggested for a group: German for a
// group named like it (DE), else any.
func wizardDefaultLang(d wizardMediaGroupDef, name string) string {
	if d.qualityKind != "video" {
		return "any"
	}

	n := strings.ToLower(name)
	if strings.HasSuffix(n, "de") || strings.Contains(n, "german") || strings.Contains(n, "deutsch") {
		return "de"
	}

	if strings.HasSuffix(n, "en") || strings.Contains(n, "english") {
		return "en"
	}

	return "any"
}

// renderWizardPatternRows are the inputs of a pattern list, each with the
// regex tester and a remove button.
func renderWizardPatternRows(field string, patterns []string) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(patterns))
	for _, p := range patterns {
		rows = append(rows, gomponents.Raw(wizardPatternRow(field, p)))
	}

	return gomponents.Group(rows)
}

func wizardPatternRow(field, value string) string {
	return `<div class="input-group input-group-sm mb-1"><input type="text" class="form-control font-monospace" name="` + field +
		`" value="` + stdhtml.EscapeString(value) + `">` + helperToolRowButton("regex") +
		`<button class="btn btn-outline-danger" type="button" title="Remove" onclick="this.parentElement.remove()"><i class="fa-solid fa-xmark"></i></button></div>`
}

// renderWizardGroupRecommend is a group's recommended settings in the
// Finalize step.
func renderWizardGroupRecommend(d wizardMediaGroupDef, g wizardGroupState) gomponents.Node {
	lang := wizardDefaultLang(d, g.Name)
	required, rejected := wizardRecommendedRegex(d.key, lang)
	sched := wizardRecommendedSchedule(d.key)
	prefix := "wizard_rec_" + d.key + "_"
	namingField := "media_main_" + d.key + "_wizard_naming"

	langSel := gomponents.Node(nil)
	if d.qualityKind == "video" {
		opts := make([]gomponents.Node, 0, len(wizardLangs))
		for _, l := range wizardLangs {
			opts = append(opts, html.Option(html.Value(l[0]), gomponents.If(l[0] == lang, html.Selected()), gomponents.Text(l[1])))
		}

		langSel = html.Div(html.Class("mb-2"),
			html.Label(html.Class("form-label small mb-1"), gomponents.Text("Release language")),
			html.Select(html.Class("form-select form-select-sm w-auto"), html.Name(prefix+"lang"),
				gomponents.Attr("onchange", "wizardRecLang(this)"), gomponents.Group(opts)),
			html.Div(html.Class("form-text"), gomponents.Text("Fills the regex rules and naming below: German requires a German release, English rejects German and other languages.")))
	}

	schedRows := make([]gomponents.Node, 0, len(wizardScheduleFields))
	for _, f := range wizardScheduleFields {
		name := prefix + "cron_" + f.key
		schedRows = append(schedRows, html.Div(html.Class("col-sm-6 mb-2"),
			html.Label(html.Class("form-label small mb-1"), html.Title(f.hint), gomponents.Text(f.label)),
			withHelperTool("cron", name, html.Input(html.Type("text"), html.Class("form-control form-control-sm font-monospace"),
				html.Name(name), html.Value(sched.get(f.key)))),
		))
	}

	add := func(field string) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary mb-2"),
			gomponents.Attr("onclick", "wizardRecAddRow(this, "+jsString(field)+")"),
			html.I(html.Class("fa-solid fa-plus me-1")), gomponents.Text("Add pattern"))
	}

	return html.Div(html.Class("wizard-rec mt-2"), html.Data("rec", wizardRecommendJSON(d.key)), html.Data("prefix", prefix),
		html.Input(html.Type("hidden"), html.Name(prefix+"shown"), html.Value("1")),
		html.Details(html.Class("mb-2"), html.Open(),
			html.Summary(html.Class("fw-semibold small"), gomponents.Text("Recommended settings (edit as needed)")),
			langSel,
			html.Div(html.Class("mb-2"),
				html.Label(html.Class("form-label small mb-1"), gomponents.Text("File and folder naming")),
				withHelperTool("naming", namingField, html.Input(html.Type("text"), html.Class("form-control form-control-sm font-monospace"),
					html.Name(namingField), html.Value(wizardRecommendedNaming(d.key, lang)), html.Data("rec-naming", "1")))),
			html.Div(html.Class("mb-1 small fw-semibold"), gomponents.Text("Required patterns (at least one must match; none: no requirement)")),
			html.Div(html.Class("wizard-rec-required"), renderWizardPatternRows(prefix+"required", required)),
			add(prefix+"required"),
			html.Div(html.Class("mb-1 small fw-semibold"), gomponents.Text("Rejected patterns (none may match)")),
			html.Div(html.Class("wizard-rec-rejected"), renderWizardPatternRows(prefix+"rejected", rejected)),
			add(prefix+"rejected"),
			renderWizardAudioOpts(d.key, prefix),
			renderWizardDownloadFields(d.key, lang, prefix),
			renderWizardNotify(prefix),
			html.Div(html.Class("small fw-semibold mt-2 mb-1"), gomponents.Text("Schedule (cron: second minute hour day month weekday)")),
			html.Div(html.Class("row"), gomponents.Group(schedRows)),
		))
}

// wizardRecommendScript swaps a group's suggestions when its language
// changes; a naming the user edited stays.
const wizardRecommendScript = `
function wizardRecRow(field, value) {
	var d = document.createElement('div');
	d.innerHTML = %s.replace('__FIELD__', field).replace('__VALUE__', value.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;'));
	return d.firstChild;
}
function wizardRecAddRow(btn, field) {
	var box = btn.previousElementSibling;
	var row = wizardRecRow(field, '');
	box.appendChild(row);
	row.querySelector('input').focus();
}
function wizardRecLang(sel) {
	var box = sel.closest('.wizard-rec'), rec = JSON.parse(box.dataset.rec), p = box.dataset.prefix;
	var old = rec[box.dataset.lang || 'any'] || {}, now = rec[sel.value];
	box.dataset.lang = sel.value;
	var naming = box.querySelector('[data-rec-naming]');
	if (naming && naming.value === old.naming) { naming.value = now.naming; }
	box.querySelectorAll('[data-rec-field]').forEach(function (f) {
		var k = f.dataset.recField;
		if (f.value === (old[k] || '')) { f.value = now[k] || ''; }
	});
	[['required', now.required], ['rejected', now.rejected]].forEach(function (x) {
		var list = box.querySelector('.wizard-rec-' + x[0]);
		list.innerHTML = '';
		(x[1] || []).forEach(function (v) { list.appendChild(wizardRecRow(p + x[0], v)); });
	});
}
document.querySelectorAll('.wizard-rec').forEach(function (b) {
	var s = b.querySelector('select'); b.dataset.lang = s ? s.value : 'any';
});
`

func wizardRecommendScriptNode() gomponents.Node {
	return html.Script(gomponents.Raw(fmt.Sprintf(wizardRecommendScript, jsString(wizardPatternRow("__FIELD__", "__VALUE__")))))
}

// wizardGroupSettings are a group's recommended settings as the user left
// them in the Finalize step.
type wizardGroupSettings struct {
	lang, naming       string
	required, rejected []string
	schedule           wizardSchedule
	audio              wizardAudioOpts
	cats, dlcat        string
	notify             []config.MediaNotificationConfig
}

// wizardReadGroupSettings reads a group's settings from the Finalize form;
// the recommendation where the fields weren't shown. bad lists invalid
// patterns and schedules.
func wizardReadGroupSettings(c *gin.Context, d wizardMediaGroupDef, g wizardGroupState) (gs wizardGroupSettings, bad []string) {
	prefix := "wizard_rec_" + d.key + "_"

	gs.lang = c.PostForm(prefix + "lang")
	if d.qualityKind != "video" || (gs.lang != "en" && gs.lang != "de" && gs.lang != "any") {
		gs.lang = wizardDefaultLang(d, g.Name)
	}

	gs.required, gs.rejected = wizardRecommendedRegex(d.key, gs.lang)
	gs.naming = wizardRecommendedNaming(d.key, gs.lang)
	gs.schedule = wizardRecommendedSchedule(d.key)
	gs.audio = wizardReadAudioOpts(c, d.key, prefix)
	gs.cats = wizardRecommendedCategories(d.key, gs.lang)
	gs.dlcat = wizardRecommendedDlCategory(d.key, gs.lang)
	gs.notify = wizardReadNotify(c, d.key, prefix)

	if c.PostForm(prefix+"shown") == "" {
		return gs, nil
	}

	if v, ok := c.GetPostForm(prefix + "cats"); ok {
		gs.cats = strings.ReplaceAll(strings.TrimSpace(v), " ", "")
		if !wizardCheckCategories(gs.cats) {
			bad = append(bad, fmt.Sprintf("%s: indexer categories %q must be numbers separated by commas", g.Name, v))
		}
	}

	if v, ok := c.GetPostForm(prefix + "dlcat"); ok {
		gs.dlcat = strings.TrimSpace(v)
	}

	if v := strings.TrimSpace(c.PostForm("media_main_" + d.key + "_wizard_naming")); v != "" {
		gs.naming = v
	}

	gs.required = wizardNonEmpty(trimAll(c.PostFormArray(prefix + "required")))
	gs.rejected = wizardNonEmpty(trimAll(c.PostFormArray(prefix + "rejected")))

	for _, p := range wizardCheckPatterns(append(append([]string{}, gs.required...), gs.rejected...)) {
		bad = append(bad, g.Name+": invalid pattern "+p)
	}

	for _, f := range wizardScheduleFields {
		// Not sent: the suggestion; sent empty: the job is off.
		v, ok := c.GetPostForm(prefix + "cron_" + f.key)
		if !ok {
			continue
		}

		if v = strings.TrimSpace(v); v != "" {
			if _, err := wizardCronParser.Parse(v); err != nil {
				bad = append(bad, fmt.Sprintf("%s: %s %q is not a valid schedule", g.Name, f.label, v))
			}
		}

		switch f.key {
		case "rss":
			gs.schedule.RSS = v
		case "missing":
			gs.schedule.Missing = v
		case "upgrade":
			gs.schedule.Upgrade = v
		case "scan":
			gs.schedule.Scan = v
		}
	}

	return gs, bad
}

func trimAll(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = strings.TrimSpace(v)
	}

	return out
}

// wizardAudioOpts are the matching and organizing options of music and
// audiobook groups the Finalize step shows; the values were tuned on large
// real libraries.
type wizardAudioOpts struct {
	RenameLog, EmbedArt, AltReleases, CountMismatch, ReplaceChapters bool
	Tolerance                                                        int
}

func wizardRecommendedAudio(key string) wizardAudioOpts {
	if key == "audiobooks" {
		// Audiobook parts vary in length between editions: a track count
		// may differ when the total runtime matches.
		return wizardAudioOpts{RenameLog: true, EmbedArt: true, AltReleases: true, CountMismatch: true, ReplaceChapters: true, Tolerance: 3}
	}

	return wizardAudioOpts{RenameLog: true, EmbedArt: true, AltReleases: true, Tolerance: 10}
}

// wizardApplyMediaDefaults fills a group's library and import path entries
// with the recommended options of its kind (the shown audio options plus
// fixed ones: unpacking, release types and formats, series handling).
func wizardApplyMediaDefaults(key string, o wizardAudioOpts, data *config.MediaDataConfig, imp *config.MediaDataImportConfig) {
	if key != "music" && key != "audiobooks" {
		return
	}

	if data != nil {
		data.EnableUnpacking = true
		data.WriteRenameLog, data.EmbedArt, data.AllowAlternativeReleases = o.RenameLog, o.EmbedArt, o.AltReleases
		data.PerTrackToleranceSeconds, data.PerTrackToleranceSecondsMax = o.Tolerance, 30
		data.AllowCountMismatchIfTotalMatch, data.ReplaceEmbeddedChapters = o.CountMismatch, o.ReplaceChapters

		if key == "music" {
			data.AllowedReleaseTypes = []string{"Album", "Compilation"}
			data.MBMediaFormats = []string{"CD", "Digital Media"}
			data.AllowAllFormatsWhenStructuring = true
			data.DiscoverSeriesAlbums = true
		} else {
			data.SkipSeriesTrackMatch = true
		}
	}

	if imp != nil {
		imp.EnableUnpacking = true
		imp.AllowAlternativeReleases = o.AltReleases
		imp.PerTrackToleranceSeconds, imp.PerTrackToleranceSecondsMax = o.Tolerance, 30
		imp.AllowCountMismatchIfTotalMatch, imp.ReplaceEmbeddedChapters = o.CountMismatch, o.ReplaceChapters

		if key == "music" {
			imp.AllowAllFormatsWhenStructuring = true
			imp.DiscoverSeriesAlbums = true
		}
	}
}

// renderWizardAudioOpts are the audio options of the Finalize step.
func renderWizardAudioOpts(key, prefix string) gomponents.Node {
	if key != "music" && key != "audiobooks" {
		return nil
	}

	o := wizardRecommendedAudio(key)
	check := func(name, label, hint string, on bool) gomponents.Node {
		return html.Div(html.Class("col-md-6 mb-1"), html.Div(html.Class("form-check"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.ID(prefix+name), html.Name(prefix+name),
				html.Value("1"), gomponents.If(on, html.Checked())),
			html.Label(html.Class("form-check-label small"), html.For(prefix+name), html.Title(hint), gomponents.Text(label))))
	}

	nodes := []gomponents.Node{
		check("renamelog", "Write a rename log", "A _rename_log.txt in each organized folder: original and new file names", o.RenameLog),
		check("embedart", "Embed cover art", "Writes the cover into the files' tags while organizing", o.EmbedArt),
		check("altreleases", "Try alternative releases", "Look for another edition (MusicBrainz/Audible) when the files don't match the one in the database", o.AltReleases),
	}

	if key == "audiobooks" {
		nodes = append(nodes,
			check("countmismatch", "Accept a different number of parts", "When the total runtime matches, a release with more or fewer files still counts", o.CountMismatch),
			check("chapters", "Replace embedded chapters", "Single-file audiobooks (m4b) get the matched book's chapter list", o.ReplaceChapters))
	}

	return html.Div(html.Class("mb-2"),
		html.Div(html.Class("small fw-semibold mb-1"), gomponents.Text("Matching & organizing")),
		html.Div(html.Class("row"), gomponents.Group(nodes)),
		html.Div(html.Class("d-flex align-items-center gap-2 mt-1"),
			html.Label(html.Class("small"), html.For(prefix+"tolerance"), gomponents.Text("Per-track runtime tolerance (seconds)")),
			html.Input(html.Type("number"), html.Min("0"), html.Max("120"), html.Class("form-control form-control-sm"), html.Style("max-width:6rem"),
				html.ID(prefix+"tolerance"), html.Name(prefix+"tolerance"), html.Value(strconv.Itoa(o.Tolerance)))),
		html.Div(html.Class("form-text"), gomponents.Text("Tracks may differ this much from the database's length without counting against a match.")))
}

// wizardReadAudioOpts reads the audio options of the form (the
// recommendation when they weren't shown).
func wizardReadAudioOpts(c *gin.Context, key, prefix string) wizardAudioOpts {
	o := wizardRecommendedAudio(key)
	if _, ok := c.GetPostForm(prefix + "tolerance"); !ok {
		return o
	}

	on := func(name string) bool { return c.PostForm(prefix+name) != "" }
	o.RenameLog, o.EmbedArt, o.AltReleases = on("renamelog"), on("embedart"), on("altreleases")

	if key == "audiobooks" {
		o.CountMismatch, o.ReplaceChapters = on("countmismatch"), on("chapters")
	}

	if n, err := strconv.Atoi(strings.TrimSpace(c.PostForm(prefix + "tolerance"))); err == nil {
		o.Tolerance = min(max(n, 0), 120)
	}

	return o
}

// wizardRecommendedCategories are the Newznab categories a group searches,
// as in production: the foreign categories for German releases (2010 movies,
// 5020 TV), the SD/WEB/HD/UHD ones for English, the parents for any
// language. Music leaves out 3030 (audiobooks) and 3020 (music videos).
func wizardRecommendedCategories(key, lang string) string {
	video := map[string][3]string{ // German, English, any
		"movies": {"2010", "2030,2035,2040,2045", "2000"},
		"series": {"5020", "5030,5040", "5000"},
	}
	if cats, ok := video[key]; ok {
		switch lang {
		case "de":
			return cats[0]
		case "en":
			return cats[1]
		}

		return cats[2]
	}

	return map[string]string{"music": "3010,3040", "audiobooks": "3030", "books": "7020"}[key]
}

// wizardRecommendedDlCategory is the download client category of a group:
// one per group, named like the media kind plus DE for German (Movies,
// MoviesDE, Series, ...), as in production.
func wizardRecommendedDlCategory(key, lang string) string {
	name := map[string]string{"movies": "Movies", "series": "Series", "music": "Music", "audiobooks": "Audiobooks", "books": "Books"}[key]
	if lang == "de" {
		name += "DE"
	}

	return name
}

// wizardCheckCategories reports whether cats is a Newznab category list
// (numbers separated by commas, or empty).
func wizardCheckCategories(cats string) bool {
	if cats == "" {
		return true
	}

	for _, c := range strings.Split(cats, ",") {
		if _, err := strconv.Atoi(c); err != nil {
			return false
		}
	}

	return true
}

// wizardNotifyEvents are the events the Finalize step offers per group, and
// whether they are on by default.
var wizardNotifyEvents = []struct {
	event, label string
	on           bool
}{
	{"added_data", "New file in the library", true},
	{"upgraded_data", "File replaced by a better one", true},
	{"added_download", "Release sent to the download client", false},
}

// wizardNotification is a group's notification entry for an event: a CSV
// line for csv targets (as in production), else a title and a message.
func wizardNotification(key, target, notifType, event string) config.MediaNotificationConfig {
	n := config.MediaNotificationConfig{MapNotification: target, Event: event}

	noun := map[string]string{"movies": "Movie", "series": "Episode", "music": "Album", "audiobooks": "Audiobook", "books": "Book"}[key]

	if event == "added_download" {
		if notifType == "csv" {
			n.Message = "{{.Nzb.NZB.Title}};{{.Cfgp.Name}}"
		} else {
			n.Title = "Download started in {{.Cfgp.Name}}"
			n.Message = "{{.Nzb.NZB.Title}}"
		}

		return n
	}

	if notifType == "csv" {
		switch key {
		case "movies":
			n.Message = "{{.Time}};{{.Title}};{{.Year}};{{.Imdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}"
		case "series":
			n.Message = "{{.Time}};{{.Title}};{{.Season}};{{.Episode}};{{.Tvdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}"
		default:
			n.Message = "{{.Time}};{{.Title}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}"
		}

		return n
	}

	n.Title = "New " + noun + " added in {{.Configuration}}"
	if event == "upgraded_data" {
		n.Title = "Replaced " + noun + " in {{.Configuration}}"
		n.ReplacedPrefix = "replaced"
	}

	n.Message = "{{.Title}}{{if .Identifier}} {{.Identifier}}{{end}} - moved from {{.SourcePath}} to {{.Targetpath}}{{if .Replaced }} Replaced: {{ range .Replaced }}{{.}},{{end}}{{end}}"

	return n
}

// renderWizardDownloadFields are a group's indexer and client categories.
func renderWizardDownloadFields(key, lang, prefix string) gomponents.Node {
	input := func(name, rec, value, label, hint string) gomponents.Node {
		return html.Div(html.Class("col-sm-6 mb-2"),
			html.Label(html.Class("form-label small mb-1"), html.For(prefix+name), gomponents.Text(label)),
			html.Input(html.Type("text"), html.Class("form-control form-control-sm font-monospace"), html.ID(prefix+name),
				html.Name(prefix+name), html.Value(value), html.Data("rec-field", rec)),
			html.Div(html.Class("form-text"), gomponents.Text(hint)))
	}

	return html.Div(
		html.Div(html.Class("small fw-semibold mt-2 mb-1"), gomponents.Text("Search & download")),
		html.Div(html.Class("row"),
			input("cats", "cats", wizardRecommendedCategories(key, lang), "Indexer categories",
				"Newznab category numbers, comma separated. Without them every search and RSS pass returns releases of all kinds."),
			input("dlcat", "dlcat", wizardRecommendedDlCategory(key, lang), "Download client category",
				"Sorts this group's downloads into their own folder in the client; create it there if it doesn't exist. Use \"Check download folders\" below.")))
}

// renderWizardNotify is a group's notification choice; nothing when no
// notification target is configured.
func renderWizardNotify(prefix string) gomponents.Node {
	targets := config.GetSettingsNotificationAll()
	if len(targets) == 0 {
		return nil
	}

	nodes := make([]gomponents.Node, 0, len(targets)+len(wizardNotifyEvents))
	for _, t := range targets {
		id := prefix + "notify_" + t.Name
		nodes = append(nodes, html.Div(html.Class("form-check form-check-inline"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.ID(id), html.Name(prefix+"notify"), html.Value(t.Name)),
			html.Label(html.Class("form-check-label small"), html.For(id), gomponents.Text(t.Name+" ("+t.NotificationType+")"))))
	}

	events := make([]gomponents.Node, 0, len(wizardNotifyEvents))
	for _, e := range wizardNotifyEvents {
		id := prefix + "ev_" + e.event
		events = append(events, html.Div(html.Class("form-check form-check-inline"),
			html.Input(html.Type("checkbox"), html.Class("form-check-input"), html.ID(id), html.Name(id), html.Value("1"), gomponents.If(e.on, html.Checked())),
			html.Label(html.Class("form-check-label small"), html.For(id), gomponents.Text(e.label))))
	}

	return html.Div(html.Class("mb-2"),
		html.Div(html.Class("small fw-semibold mt-2 mb-1"), gomponents.Text("Notifications")),
		html.Div(html.Class("small text-muted mb-1"), gomponents.Text("Send to:")),
		html.Div(gomponents.Group(nodes)),
		html.Div(html.Class("small text-muted mt-1 mb-1"), gomponents.Text("When:")),
		html.Div(gomponents.Group(events)))
}

// wizardReadNotify builds a group's notification entries from the form.
func wizardReadNotify(c *gin.Context, key, prefix string) []config.MediaNotificationConfig {
	var out []config.MediaNotificationConfig

	for _, target := range wizardNonEmpty(c.PostFormArray(prefix + "notify")) {
		t := config.GetSettingsNotification(target)
		if t == nil {
			continue
		}

		for _, e := range wizardNotifyEvents {
			if c.PostForm(prefix+"ev_"+e.event) != "" {
				out = append(out, wizardNotification(key, target, t.NotificationType, e.event))
			}
		}
	}

	return out
}
