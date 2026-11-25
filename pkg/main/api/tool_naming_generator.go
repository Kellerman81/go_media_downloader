package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

type TemplateField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Template    string `json:"template"`
	Category    string `json:"category"`
}

type TemplateStructure struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// ngTypes are the template types of the naming generator: one per media
// type for the naming field of a media group, plus notification messages.
var ngTypes = []struct{ Value, Label string }{
	{"movie", "Movies (media group → naming)"},
	{"series", "Series (media group → naming)"},
	{"music", "Music (media group → naming)"},
	{"book", "Books (media group → naming)"},
	{"audiobook", "Audiobooks (media group → naming)"},
	{"notification", "Notification title and message"},
}

// ngTypeFields lists the naming data each media type fills: struct names
// (matching their nested fields) and top-level fields. Source is the parsed
// release or file.
var ngTypeFields = map[string][]string{
	"movie":     {"Dbmovie", "Source", "TitleSource", "Title", "Identifier"},
	"series":    {"Dbserie", "Serie", "DbserieEpisode", "Source", "TitleSource", "EpisodeTitleSource", "Identifier", "Episodes", "Title"},
	"music":     {"Dbalbum", "Dbtrack", "Artist", "AlbumArtist", "Source", "Title", "Track", "Disc", "TotalDiscs"},
	"book":      {"Dbbook", "Author", "BookSeries", "Source", "Title"},
	"audiobook": {"Dbaudiobook", "DbaudiobookChapter", "Dbbook", "Author", "BookSeries", "Source", "Title", "Track", "Disc", "TotalDiscs"},
}

// ngKnownType reports whether dataType is one of ngTypes.
func ngKnownType(dataType string) bool {
	for _, t := range ngTypes {
		if t.Value == dataType {
			return true
		}
	}

	return false
}

// ngKind maps a template type to the data it is rendered with: "notification"
// or "parser" (naming). Unknown types are naming templates.
func ngKind(dataType string) string {
	if dataType == "notification" {
		return "notification"
	}

	return "parser"
}

// ngExamples are the examples offered on the naming generator page.
var ngExamples = []struct{ Label, DataType, Template string }{
	{
		"Series: folder per season", "series",
		`{{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}/{{.Dbserie.Seriename}} - S{{printf "%02s" .DbserieEpisode.Season}}{{range .Episodes}}E{{printf "%02d" . }}{{end}} - {{.DbserieEpisode.Title}} [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}] ({{.Source.Tvdb}})`,
	},
	{
		"Series: one folder per series", "series",
		`{{.Dbserie.Seriename}}/{{.Dbserie.Seriename}} {{.DbserieEpisode.Identifier}} {{.EpisodeTitleSource}} - [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}]`,
	},
	{
		"Movie: folder per movie", "movie",
		`{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}{{if eq .Source.Extended true}} extended{{end}}] ({{.Source.Imdb}})`,
	},
	{
		"Music: letter / artist / album / track", "music",
		`{{if .AlbumArtist.Name}}{{firstLetter .AlbumArtist.Name}}{{else}}_{{end}}/{{if or (eq .AlbumArtist.Name "Various Artists") (eq .AlbumArtist.Name "VA") (eq .AlbumArtist.Name "Various")}}Various Artists{{else}}{{.AlbumArtist.Name}}{{end}}/{{.Dbalbum.Title}}{{if .Dbalbum.Year}} ({{.Dbalbum.Year}}){{end}}/{{if gt .TotalDiscs 1}}{{printf "%02d" .Dbtrack.DiscNumber}}-{{end}}{{printf "%02d" .Track}} - {{.Title}}`,
	},
	{
		"Music: artist / album / track", "music",
		`{{.AlbumArtist.Name}}/{{.Dbalbum.Title}}{{if .Dbalbum.Year}} ({{.Dbalbum.Year}}){{end}}/{{printf "%02d" .Track}} - {{.Artist.Name}} - {{.Title}}`,
	},
	{
		"Book: author / series / title", "book",
		`{{if .Author.Name}}{{.Author.Name}}{{else}}Unknown Author{{end}}/{{if .BookSeries.Name}}{{.BookSeries.Name}}/{{if .Dbbook.SeriesPosition}}{{.Dbbook.SeriesPosition}} - {{end}}{{end}}{{.Dbbook.Title}}{{if .Dbbook.Year}} ({{.Dbbook.Year}}){{end}}/{{.Author.Name}} - {{.Dbbook.Title}}`,
	},
	{
		"Audiobook: letter / author / series / title / part", "audiobook",
		`{{if .Author.Name}}{{firstLetter .Author.Name}}{{else}}_{{end}}/{{if .Author.Name}}{{.Author.Name}}{{else}}{{.Dbaudiobook.Title}}{{end}}{{if .BookSeries.Name}}/{{.BookSeries.Name}}{{end}}/{{if and .BookSeries.Name .Dbbook.SeriesPosition}}{{.Dbbook.SeriesPosition}} - {{end}}{{.Dbaudiobook.Title}}{{if .Dbaudiobook.ASIN}} ({{.Dbaudiobook.ASIN}}){{end}}/{{if gt .TotalDiscs 1}}{{.Disc}}-{{end}}{{.Track}} - {{.Title}}`,
	},
	{
		"Audiobook: author / title / chapter", "audiobook",
		`{{.Author.Name}}/{{.Dbaudiobook.Title}}{{if .Dbaudiobook.Year}} ({{.Dbaudiobook.Year}}){{end}}/{{printf "%03d" .Track}} - {{.Title}}`,
	},
	{
		"Notification: CSV line", "notification",
		`{{.Time}};{{.Title}};{{.Identifier}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}`,
	},
	{
		"Notification: moved message", "notification",
		`{{.Title}} - moved from {{.SourcePath}} to {{.Targetpath}}{{if .Replaced }} Replaced: {{ range .Replaced }}{{.}},{{end}}{{end}}`,
	},
}

// renderNamingGeneratorPage renders the generator for a template type, with
// an optional template to start from (both from the URL, so config fields can
// open it prefilled).
func renderNamingGeneratorPage(csrfToken, dataType, template string) gomponents.Node {
	if !ngKnownType(dataType) {
		dataType = "movie"
	}

	typeOptions := make([]gomponents.Node, 0, len(ngTypes))
	for _, t := range ngTypes {
		typeOptions = append(typeOptions, html.Option(html.Value(t.Value), gomponents.If(t.Value == dataType, html.Selected()),
			gomponents.Text(t.Label)))
	}

	placeholder := ""
	for _, e := range ngExamples {
		if e.DataType == dataType {
			placeholder = e.Template
			break
		}
	}

	examples := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("Start from an example…"))}
	for _, t := range ngTypes {
		var opts []gomponents.Node
		for _, e := range ngExamples {
			if e.DataType == t.Value {
				opts = append(opts, html.Option(html.Value(e.Template), html.Data("type", e.DataType), gomponents.Text(e.Label)))
			}
		}

		if len(opts) > 0 {
			examples = append(examples, html.OptGroup(gomponents.Attr("label", t.Label), gomponents.Group(opts)))
		}
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader(
			"fa-solid fa-code",
			"Naming Generator",
			"Build the Go templates for file and folder names and for notification messages. "+
				"Every change is checked and previewed with sample data.",
		),
		html.Form(
			html.ID("namingForm"),
			hx.Post("/api/admin/naming-generator/check"),
			hx.Target("#namingResult"),
			hx.Trigger("input delay:500ms from:#ngTemplate, change from:#ngType"),
			hx.Headers(createHTMXHeaders(csrfToken)),
			html.Div(
				html.Class("card border-0 shadow-sm mb-4"),
				html.Div(
					html.Class("card-body row g-3 align-items-end"),
					html.Div(
						html.Class("col-md-5"),
						html.Label(html.Class("form-label fw-semibold"), html.For("ngType"), gomponents.Text("Template for")),
						html.Select(
							html.Class("form-select"), html.ID("ngType"), html.Name("data_type"),
							hx.Get("/api/admin/naming-generator/fields"), hx.Target("#ngFields"), hx.Trigger("change"),
							gomponents.Group(typeOptions),
						),
					),
					html.Div(
						html.Class("col-md-4"),
						html.Label(html.Class("form-label"), html.For("ngExample"), gomponents.Text("Example")),
						html.Select(html.Class("form-select"), html.ID("ngExample"), gomponents.Group(examples)),
					),
					html.Div(
						html.Class("col-md-3 text-md-end"),
						html.Button(html.Type("button"), html.Class("btn btn-outline-secondary me-2"), html.ID("ngCopy"),
							html.I(html.Class("fas fa-copy me-1")), gomponents.Text("Copy")),
						html.Button(html.Type("button"), html.Class("btn btn-outline-danger"), html.ID("ngClear"),
							html.I(html.Class("fas fa-eraser me-1")), gomponents.Text("Clear")),
					),
				),
			),
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-4"),
					html.Div(
						html.Class("card border-0 shadow-sm h-100"),
						html.Div(html.Class("card-header bg-transparent fw-semibold"),
							html.I(html.Class("fa-solid fa-list me-2 text-primary")), gomponents.Text("Fields")),
						html.Div(
							html.Class("card-body"),
							html.Input(html.Class("form-control form-control-sm mb-2"), html.Type("search"), html.ID("ngFieldSearch"),
								html.Placeholder("Search fields…")),
							html.P(html.Class("small text-muted"), gomponents.Text("Click a field to insert it at the cursor.")),
							html.Div(html.ID("ngFields"), html.Style("max-height: 560px; overflow-y: auto;"), renderNamingFields(dataType)),
						),
					),
				),
				html.Div(
					html.Class("col-lg-8"),
					html.Div(
						html.Class("card border-0 shadow-sm mb-4"),
						html.Div(html.Class("card-header bg-transparent fw-semibold"),
							html.I(html.Class("fa-solid fa-pen me-2 text-primary")), gomponents.Text("Template")),
						html.Div(
							html.Class("card-body"),
							html.Textarea(
								html.Class("form-control font-monospace"), html.ID("ngTemplate"), html.Name("template"), html.Rows("6"),
								html.Placeholder(placeholder),
								gomponents.Text(template),
							),
						),
					),
					html.Div(html.ID("namingResult"),
						gomponents.If(template == "",
							html.P(html.Class("text-muted"), gomponents.Text("Type a template or pick an example to see the preview."))),
						// A prefilled template is checked right away.
						gomponents.If(template != "", html.Div(hx.Post("/api/admin/naming-generator/check"), hx.Trigger("load"),
							hx.Include("#namingForm"), hx.Target("#namingResult"), hx.Headers(createHTMXHeaders(csrfToken)))),
					),
				),
			),
		),
		html.Script(gomponents.Raw(ngScript)),
	)
}

// renderNamingFields renders the insertable fields and template structures
// of a template type, grouped by category.
func renderNamingFields(dataType string) gomponents.Node {
	fields, structures := getFieldsForDataType(dataType)

	item := func(name, template, description, example, kind string) gomponents.Node {
		search := strings.ToLower(name + " " + description)
		nodes := []gomponents.Node{
			html.Class("list-group-item list-group-item-action py-1 px-2 ng-field"),
			html.Type("button"),
			html.Data("ng-insert", template),
			html.Data("search", search),
			html.Div(html.Class("d-flex justify-content-between"),
				html.Code(gomponents.Text(name)),
				gomponents.If(kind != "", html.Span(html.Class("badge bg-light text-dark border"), gomponents.Text(kind))),
			),
			html.Div(html.Class("small text-muted"), gomponents.Text(description)),
		}

		if example != "" {
			nodes = append(nodes, html.Div(html.Class("small text-success text-truncate"), gomponents.Text("e.g. "+example)))
		}

		return html.Button(nodes...)
	}

	section := func(title string, items []gomponents.Node) gomponents.Node {
		return html.Details(html.Class("mb-2 ng-category"),
			html.Summary(html.Class("fw-semibold small"), gomponents.Textf("%s (%d)", title, len(items))),
			html.Div(html.Class("list-group list-group-flush mt-1"), gomponents.Group(items)),
		)
	}

	sections := make([]gomponents.Node, 0)

	structItems := make([]gomponents.Node, 0, len(structures))
	for _, st := range structures {
		structItems = append(structItems, item(st.Name, st.Template, st.Description, st.Example, ""))
	}

	sections = append(sections, section("Template structures", structItems))

	var order []string

	byCategory := map[string][]gomponents.Node{}
	for _, f := range fields {
		if _, ok := byCategory[f.Category]; !ok {
			order = append(order, f.Category)
		}

		byCategory[f.Category] = append(byCategory[f.Category], item(f.Name, f.Template, f.Description, f.Example, f.Type))
	}

	for _, c := range order {
		sections = append(sections, section(c, byCategory[c]))
	}

	return gomponents.Group(sections)
}

// HandleNamingFieldsForType renders the fields panel for ?data_type=.
func HandleNamingFieldsForType(ctx *gin.Context) {
	ctx.String(http.StatusOK, renderComponentToString(renderNamingFields(ctx.Query("data_type"))))
}

// HandleNamingCheck verifies the posted template and renders it with sample data.
func HandleNamingCheck(ctx *gin.Context) {
	template := strings.TrimSpace(ctx.PostForm("template"))
	dataType := strings.TrimSpace(ctx.PostForm("data_type"))

	if template == "" {
		ctx.String(http.StatusOK, renderComponentToString(
			html.P(html.Class("text-muted"), gomponents.Text("Type a template or pick an example to see the preview."))))

		return
	}

	var (
		output string
		err    error
	)

	if !ngKnownType(dataType) {
		ctx.String(http.StatusOK, renderAlert("Unknown template type "+dataType, "danger"))
		return
	}

	if ngKind(dataType) == "notification" {
		output, err = structure.TestInputnotifier(template)
	} else {
		output, err = structure.TestParsertype(template, dataType)
	}

	verification := verifyGoTemplate(template, dataType)

	var preview gomponents.Node
	if err != nil {
		preview = html.Div(html.Class("alert alert-danger mb-0 small"), gomponents.Text(err.Error()))
	} else {
		preview = html.Pre(html.Class("bg-light border rounded p-3 mb-0 small"), html.Style("white-space: pre-wrap;"),
			gomponents.Text(output))
	}

	list := func(items []string) gomponents.Node {
		lis := make([]gomponents.Node, 0, len(items))
		for _, it := range items {
			lis = append(lis, html.Li(gomponents.Text(it)))
		}

		return html.Ul(html.Class("mb-0 small"), gomponents.Group(lis))
	}

	var check gomponents.Node
	if verification.Valid {
		check = html.Div(html.Class("text-success small"),
			html.I(html.Class("fas fa-check-circle me-1")), gomponents.Text(verification.Message),
			gomponents.If(len(verification.Suggestions) > 0, html.Div(html.Class("mt-2"), html.Strong(gomponents.Text("Suggestions:")), list(verification.Suggestions))),
		)
	} else {
		check = html.Div(html.Class("alert alert-warning mb-0 small"),
			html.I(html.Class("fas fa-exclamation-triangle me-1")), gomponents.Text(verification.Message),
			gomponents.If(len(verification.Errors) > 0, list(verification.Errors)),
		)
	}

	ctx.String(http.StatusOK, renderComponentToString(html.Div(
		html.Class("card border-0 shadow-sm"),
		html.Div(html.Class("card-header bg-transparent fw-semibold"),
			html.I(html.Class("fa-solid fa-eye me-2 text-primary")), gomponents.Text("Preview with sample data")),
		html.Div(html.Class("card-body"),
			preview,
			html.Div(html.Class("mt-3"), check),
		),
	)))
}

// ngScript inserts fields at the cursor, filters the fields panel, loads
// examples and handles copy/clear. Changes to the template fire an input
// event, which the form turns into a check request.
const ngScript = `
(function () {
	function tpl() { return document.getElementById('ngTemplate'); }
	function changed() { tpl().dispatchEvent(new Event('input', { bubbles: true })); }
	function notify(kind, msg) { if (typeof showToaster === 'function') { showToaster(kind, msg); } }

	document.addEventListener('click', function (e) {
		var field = e.target.closest && e.target.closest('[data-ng-insert]');
		if (field) {
			var t = tpl(), v = field.getAttribute('data-ng-insert');
			var start = t.selectionStart, end = t.selectionEnd;
			t.value = t.value.slice(0, start) + v + t.value.slice(end);
			t.focus();
			t.setSelectionRange(start + v.length, start + v.length);
			changed();
			return;
		}
		if (e.target.closest && e.target.closest('#ngClear')) {
			tpl().value = '';
			changed();
			tpl().focus();
		}
		if (e.target.closest && e.target.closest('#ngCopy')) {
			if (!tpl().value.trim()) { notify('warning', 'The template is empty'); return; }
			navigator.clipboard.writeText(tpl().value).then(
				function () { notify('success', 'Template copied'); },
				function () { notify('error', 'Copying failed'); });
		}
	});

	document.addEventListener('change', function (e) {
		// The placeholder shows the first example of the selected type.
		if (e.target.id === 'ngType') {
			var first = document.querySelector('#ngExample option[data-type="' + e.target.value + '"]');
			tpl().placeholder = first ? first.value : '';
			return;
		}
		if (e.target.id !== 'ngExample' || !e.target.value) { return; }
		var opt = e.target.selectedOptions[0];
		var type = document.getElementById('ngType');
		tpl().value = e.target.value;
		e.target.value = '';
		if (type.value !== opt.dataset.type) {
			type.value = opt.dataset.type;
			type.dispatchEvent(new Event('change', { bubbles: true }));
		} else {
			changed();
		}
	});

	document.addEventListener('input', function (e) {
		if (e.target.id !== 'ngFieldSearch') { return; }
		var q = e.target.value.toLowerCase().trim();
		document.querySelectorAll('#ngFields .ng-category').forEach(function (cat) {
			var any = false;
			cat.querySelectorAll('.ng-field').forEach(function (f) {
				var hit = !q || f.dataset.search.indexOf(q) >= 0;
				f.classList.toggle('d-none', !hit);
				any = any || hit;
			});
			cat.classList.toggle('d-none', !any);
			cat.open = !!q && any;
		});
	});
})();
`

func getFieldsForDataType(dataType string) ([]TemplateField, []TemplateStructure) {
	structures := []TemplateStructure{
		{
			Name:        "If Statement",
			Template:    "{{if .FIELD}}...{{end}}",
			Description: "Conditional block - shows content only if field exists",
			Example:     "{{if .Dbmovie.Year}}Released in {{.Dbmovie.Year}}{{end}}",
		},
		{
			Name:        "If-Else Statement",
			Template:    "{{if .FIELD}}...{{else}}...{{end}}",
			Description: "Conditional with fallback - shows different content based on condition",
			Example:     "{{if .Dbmovie.Year}}{{.Dbmovie.Year}}{{else}}Unknown Year{{end}}",
		},
		{
			Name:        "Range Loop",
			Template:    "{{range .ARRAY}}{{.}}{{end}}",
			Description: "Loop through array/slice - repeats content for each item",
			Example:     "{{range .Replaced}}{{.}}, {{end}}",
		},
		{
			Name:        "Range with Index",
			Template:    "{{range $index, $element := .ARRAY}}{{$index}}: {{$element}}{{end}}",
			Description: "Loop with index access",
			Example:     "{{range $i, $file := .Replaced}}{{$i}}: {{$file}}{{end}}",
		},
		{
			Name:        "With Statement",
			Template:    "{{with .FIELD}}...{{end}}",
			Description: "Execute block only if field is not empty/nil",
			Example:     "{{with .Dbmovie.ImdbID}}IMDB: {{.}}{{end}}",
		},
		{
			Name:        "Variable Assignment",
			Template:    "{{$var := .FIELD}}{{$var}}",
			Description: "Assign field to variable for reuse",
			Example:     "{{$title := .Dbmovie.Title}}Title: {{$title}}",
		},
	}

	if dataType == "notification" {
		return getNotificationFields(), structures
	}

	fields := getParserFields()

	prefixes, ok := ngTypeFields[dataType]
	if !ok {
		return fields, structures
	}

	filtered := fields[:0]
	for _, f := range fields {
		for _, p := range prefixes {
			if f.Name == p || strings.HasPrefix(f.Name, p+".") {
				filtered = append(filtered, f)
				break
			}
		}
	}

	return filtered, structures
}

func getParserFields() []TemplateField {
	return []TemplateField{
		// Dbmovie fields (nested struct)
		{
			Name:        "Dbmovie.Title",
			Type:        "string",
			Template:    "{{.Dbmovie.Title}}",
			Description: "Movie title",
			Example:     "Inception",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Year",
			Type:        "int",
			Template:    "{{.Dbmovie.Year}}",
			Description: "Movie year",
			Example:     "2000",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.OriginalTitle",
			Type:        "string",
			Template:    "{{.Dbmovie.OriginalTitle}}",
			Description: "Original movie title",
			Example:     "Inception",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Overview",
			Type:        "string",
			Template:    "{{.Dbmovie.Overview}}",
			Description: "Movie plot summary",
			Example:     "A thief who steals secrets...",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Tagline",
			Type:        "string",
			Template:    "{{.Dbmovie.Tagline}}",
			Description: "Movie tagline",
			Example:     "Your mind is the scene of the crime",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Genres",
			Type:        "string",
			Template:    "{{.Dbmovie.Genres}}",
			Description: "Movie genres",
			Example:     "Action, Sci-Fi, Thriller",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Runtime",
			Type:        "int",
			Template:    "{{.Dbmovie.Runtime}}",
			Description: "Movie runtime in minutes",
			Example:     "148",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.ReleaseDate",
			Type:        "time",
			Template:    "{{.Dbmovie.ReleaseDate}}",
			Description: "Movie release date",
			Example:     "2010-07-16",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Status",
			Type:        "string",
			Template:    "{{.Dbmovie.Status}}",
			Description: "Release status",
			Example:     "Released",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.OriginalLanguage",
			Type:        "string",
			Template:    "{{.Dbmovie.OriginalLanguage}}",
			Description: "Original language",
			Example:     "en",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.SpokenLanguages",
			Type:        "string",
			Template:    "{{.Dbmovie.SpokenLanguages}}",
			Description: "Spoken languages",
			Example:     "English, Japanese",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.ImdbID",
			Type:        "string",
			Template:    "{{.Dbmovie.ImdbID}}",
			Description: "IMDB ID",
			Example:     "tt1375666",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.MoviedbID",
			Type:        "int",
			Template:    "{{.Dbmovie.MoviedbID}}",
			Description: "MovieDB ID",
			Example:     "27205",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.TraktID",
			Type:        "int",
			Template:    "{{.Dbmovie.TraktID}}",
			Description: "Trakt ID",
			Example:     "1390",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.FacebookID",
			Type:        "string",
			Template:    "{{.Dbmovie.FacebookID}}",
			Description: "Facebook page ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.InstagramID",
			Type:        "string",
			Template:    "{{.Dbmovie.InstagramID}}",
			Description: "Instagram ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.TwitterID",
			Type:        "string",
			Template:    "{{.Dbmovie.TwitterID}}",
			Description: "Twitter ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.VoteAverage",
			Type:        "float32",
			Template:    "{{.Dbmovie.VoteAverage}}",
			Description: "Average user rating",
			Example:     "8.3",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.VoteCount",
			Type:        "int32",
			Template:    "{{.Dbmovie.VoteCount}}",
			Description: "Number of votes",
			Example:     "31280",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.Popularity",
			Type:        "float32",
			Template:    "{{.Dbmovie.Popularity}}",
			Description: "Popularity score",
			Example:     "108.6",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.Budget",
			Type:        "int",
			Template:    "{{.Dbmovie.Budget}}",
			Description: "Production budget",
			Example:     "160000000",
			Category:    "Movie Financial",
		},
		{
			Name:        "Dbmovie.Revenue",
			Type:        "int",
			Template:    "{{.Dbmovie.Revenue}}",
			Description: "Box office revenue",
			Example:     "825532764",
			Category:    "Movie Financial",
		},
		{
			Name:        "Dbmovie.Poster",
			Type:        "string",
			Template:    "{{.Dbmovie.Poster}}",
			Description: "Poster image path",
			Example:     "/poster.jpg",
			Category:    "Movie Images",
		},
		{
			Name:        "Dbmovie.Backdrop",
			Type:        "string",
			Template:    "{{.Dbmovie.Backdrop}}",
			Description: "Backdrop image path",
			Example:     "/backdrop.jpg",
			Category:    "Movie Images",
		},
		{
			Name:        "Dbmovie.Slug",
			Type:        "string",
			Template:    "{{.Dbmovie.Slug}}",
			Description: "URL slug",
			Example:     "inception-2010",
			Category:    "Movie Information",
		},

		// Dbserie fields (nested struct)
		{
			Name:        "Dbserie.Seriename",
			Type:        "string",
			Template:    "{{.Dbserie.Seriename}}",
			Description: "Series name",
			Example:     "Breaking Bad",
			Category:    "Series Information",
		},
		{
			Name:        "Serie.Aliases",
			Type:        "string",
			Template:    "{{.Serie.Aliases}}",
			Description: "Alternative series names",
			Example:     "Breaking Bad, BB",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Overview",
			Type:        "string",
			Template:    "{{.Dbserie.Overview}}",
			Description: "Series plot summary",
			Example:     "A high school chemistry teacher...",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Status",
			Type:        "string",
			Template:    "{{.Dbserie.Status}}",
			Description: "Series status",
			Example:     "Ended",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Firstaired",
			Type:        "string",
			Template:    "{{.Dbserie.Firstaired}}",
			Description: "First air date",
			Example:     "2008-01-20",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Network",
			Type:        "string",
			Template:    "{{.Dbserie.Network}}",
			Description: "Broadcasting network",
			Example:     "AMC",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Runtime",
			Type:        "string",
			Template:    "{{.Dbserie.Runtime}}",
			Description: "Episode runtime",
			Example:     "47",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Language",
			Type:        "string",
			Template:    "{{.Dbserie.Language}}",
			Description: "Primary language",
			Example:     "en",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Genre",
			Type:        "string",
			Template:    "{{.Dbserie.Genre}}",
			Description: "Series genres",
			Example:     "Crime, Drama, Thriller",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Rating",
			Type:        "string",
			Template:    "{{.Dbserie.Rating}}",
			Description: "Content rating",
			Example:     "TV-MA",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Siterating",
			Type:        "string",
			Template:    "{{.Dbserie.Siterating}}",
			Description: "Site user rating",
			Example:     "9.5",
			Category:    "Series Ratings",
		},
		{
			Name:        "Dbserie.SiteratingCount",
			Type:        "string",
			Template:    "{{.Dbserie.SiteratingCount}}",
			Description: "Rating vote count",
			Example:     "1,654,876",
			Category:    "Series Ratings",
		},
		{
			Name:        "Dbserie.ImdbID",
			Type:        "string",
			Template:    "{{.Dbserie.ImdbID}}",
			Description: "IMDB ID",
			Example:     "tt0903747",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.ThetvdbID",
			Type:        "int",
			Template:    "{{.Dbserie.ThetvdbID}}",
			Description: "TVDB ID",
			Example:     "81189",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.TraktID",
			Type:        "int",
			Template:    "{{.Dbserie.TraktID}}",
			Description: "Trakt ID",
			Example:     "1388",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.TvrageID",
			Type:        "int",
			Template:    "{{.Dbserie.TvrageID}}",
			Description: "TVRage ID",
			Example:     "18164",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Facebook",
			Type:        "string",
			Template:    "{{.Dbserie.Facebook}}",
			Description: "Facebook page URL",
			Example:     "https://www.facebook.com/BreakingBad",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Instagram",
			Type:        "string",
			Template:    "{{.Dbserie.Instagram}}",
			Description: "Instagram URL",
			Example:     "https://www.instagram.com/breakingbad",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Twitter",
			Type:        "string",
			Template:    "{{.Dbserie.Twitter}}",
			Description: "Twitter URL",
			Example:     "https://twitter.com/BreakingBad_AMC",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Banner",
			Type:        "string",
			Template:    "{{.Dbserie.Banner}}",
			Description: "Banner image path",
			Example:     "/banner.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Poster",
			Type:        "string",
			Template:    "{{.Dbserie.Poster}}",
			Description: "Poster image path",
			Example:     "/poster.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Fanart",
			Type:        "string",
			Template:    "{{.Dbserie.Fanart}}",
			Description: "Fanart image path",
			Example:     "/fanart.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Identifiedby",
			Type:        "string",
			Template:    "{{.Dbserie.Identifiedby}}",
			Description: "Episode ID method",
			Example:     "ep",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Slug",
			Type:        "string",
			Template:    "{{.Dbserie.Slug}}",
			Description: "URL slug",
			Example:     "breaking-bad",
			Category:    "Series Information",
		},

		// DbserieEpisode fields (nested struct)
		{
			Name:        "DbserieEpisode.Title",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Title}}",
			Description: "Episode title",
			Example:     "Pilot",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Season",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Season}}",
			Description: "Season number",
			Example:     "1",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Episode",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Episode}}",
			Description: "Episode number",
			Example:     "1",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Identifier",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Identifier}}",
			Description: "Episode identifier",
			Example:     "S01E01",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Overview",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Overview}}",
			Description: "Episode summary",
			Example:     "Walter White begins cooking...",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.FirstAired",
			Type:        "time",
			Template:    "{{.DbserieEpisode.FirstAired}}",
			Description: "Original air date",
			Example:     "2008-01-20",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Runtime",
			Type:        "int",
			Template:    "{{.DbserieEpisode.Runtime}}",
			Description: "Episode runtime in minutes",
			Example:     "58",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Poster",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Poster}}",
			Description: "Episode poster image",
			Example:     "/episode_poster.jpg",
			Category:    "Episode Images",
		},

		// Source (ParseInfo) fields (nested struct pointer)
		{
			Name:        "Source.Title",
			Type:        "string",
			Template:    "{{.Source.Title}}",
			Description: "Parsed media title",
			Example:     "Breaking Bad",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Year",
			Type:        "uint16",
			Template:    "{{.Source.Year}}",
			Description: "Parsed release year",
			Example:     "2008",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Season",
			Type:        "int",
			Template:    "{{.Source.Season}}",
			Description: "Parsed season number",
			Example:     "1",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Episode",
			Type:        "int",
			Template:    "{{.Source.Episode}}",
			Description: "Parsed episode number",
			Example:     "1",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Quality",
			Type:        "string",
			Template:    "{{.Source.Quality}}",
			Description: "Video quality",
			Example:     "bluray",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Resolution",
			Type:        "string",
			Template:    "{{.Source.Resolution}}",
			Description: "Video resolution",
			Example:     "1080p",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Codec",
			Type:        "string",
			Template:    "{{.Source.Codec}}",
			Description: "Video codec",
			Example:     "x264",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Audio",
			Type:        "string",
			Template:    "{{.Source.Audio}}",
			Description: "Audio codec",
			Example:     "AC3",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.File",
			Type:        "string",
			Template:    "{{.Source.File}}",
			Description: "File path",
			Example:     "/path/to/file.mkv",
			Category:    "Source File",
		},
		{
			Name:        "Source.Runtime",
			Type:        "int",
			Template:    "{{.Source.Runtime}}",
			Description: "Runtime in minutes",
			Example:     "58",
			Category:    "Source File",
		},
		{
			Name:        "Source.Height",
			Type:        "int",
			Template:    "{{.Source.Height}}",
			Description: "Video height",
			Example:     "1080",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Width",
			Type:        "int",
			Template:    "{{.Source.Width}}",
			Description: "Video width",
			Example:     "1920",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Imdb",
			Type:        "string",
			Template:    "{{.Source.Imdb}}",
			Description: "IMDB ID from source",
			Example:     "tt0903747",
			Category:    "Source External IDs",
		},
		{
			Name:        "Source.Tvdb",
			Type:        "string",
			Template:    "{{.Source.Tvdb}}",
			Description: "TVDB ID from source",
			Example:     "tvdb81189",
			Category:    "Source External IDs",
		},
		{
			Name:        "Source.Identifier",
			Type:        "string",
			Template:    "{{.Source.Identifier}}",
			Description: "Source identifier",
			Example:     "S01E01",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Date",
			Type:        "string",
			Template:    "{{.Source.Date}}",
			Description: "Source release date",
			Example:     "2008-01-20",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Proper",
			Type:        "bool",
			Template:    "{{.Source.Proper}}",
			Description: "Is proper release",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Extended",
			Type:        "bool",
			Template:    "{{.Source.Extended}}",
			Description: "Is extended version",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Repack",
			Type:        "bool",
			Template:    "{{.Source.Repack}}",
			Description: "Is repack release",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Languages",
			Type:        "[]string",
			Template:    "{{range .Source.Languages}}{{.}}{{end}}",
			Description: "Available languages",
			Example:     "English, Spanish",
			Category:    "Source Information",
		},

		// Top-level parsertype fields
		{
			Name:        "TitleSource",
			Type:        "string",
			Template:    "{{.TitleSource}}",
			Description: "Source title from filename",
			Example:     "Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS",
			Category:    "Parser Information",
		},
		{
			Name:        "EpisodeTitleSource",
			Type:        "string",
			Template:    "{{.EpisodeTitleSource}}",
			Description: "Episode title from source",
			Example:     "Pilot",
			Category:    "Parser Information",
		},
		{
			Name:        "Identifier",
			Type:        "string",
			Template:    "{{.Identifier}}",
			Description: "Parsed identifier",
			Example:     "S01E01",
			Category:    "Parser Information",
		},
		{
			Name:        "Episodes",
			Type:        "[]int",
			Template:    "{{range .Episodes}}{{.}}{{end}}",
			Description: "Episode numbers array",
			Example:     "1, 2, 3",
			Category:    "Parser Information",
		},
		{
			Name:        "Title",
			Type:        "string",
			Template:    "{{.Title}}",
			Description: "Resolved media title",
			Example:     "The Fellowship of the Ring",
			Category:    "Parser Information",
		},
		{
			Name:        "Track",
			Type:        "int",
			Template:    "{{.Track}}",
			Description: "Track number (music)",
			Example:     "1",
			Category:    "Parser Information",
		},
		{
			Name:        "Disc",
			Type:        "int",
			Template:    "{{.Disc}}",
			Description: "Disc number (music)",
			Example:     "1",
			Category:    "Parser Information",
		},
		{
			Name:        "TotalDiscs",
			Type:        "int",
			Template:    "{{.TotalDiscs}}",
			Description: "Total number of discs (music)",
			Example:     "2",
			Category:    "Parser Information",
		},

		// ---- Music: Album (Dbalbum) ----
		{
			Name:        "Dbalbum.Title",
			Type:        "string",
			Template:    "{{.Dbalbum.Title}}",
			Description: "Album title",
			Example:     "Dark Side of the Moon",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Year",
			Type:        "uint16",
			Template:    "{{.Dbalbum.Year}}",
			Description: "Album release year",
			Example:     "1973",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.ReleaseType",
			Type:        "string",
			Template:    "{{.Dbalbum.ReleaseType}}",
			Description: "Release type (album, ep, single)",
			Example:     "album",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Format",
			Type:        "string",
			Template:    "{{.Dbalbum.Format}}",
			Description: "Release format (cd, vinyl, digital)",
			Example:     "vinyl",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Label",
			Type:        "string",
			Template:    "{{.Dbalbum.Label}}",
			Description: "Record label",
			Example:     "Harvest",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Genres",
			Type:        "string",
			Template:    "{{.Dbalbum.Genres}}",
			Description: "Album genres",
			Example:     "Progressive Rock",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.TotalTracks",
			Type:        "int",
			Template:    "{{.Dbalbum.TotalTracks}}",
			Description: "Number of tracks",
			Example:     "10",
			Category:    "Music - Album",
		},

		// ---- Music: Artist (Artist / AlbumArtist = database.Dbartist) ----
		{
			Name:        "Artist.Name",
			Type:        "string",
			Template:    "{{.Artist.Name}}",
			Description: "Primary artist name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},
		{
			Name:        "Artist.SortName",
			Type:        "string",
			Template:    "{{.Artist.SortName}}",
			Description: "Artist sort name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},
		{
			Name:        "AlbumArtist.Name",
			Type:        "string",
			Template:    "{{.AlbumArtist.Name}}",
			Description: "Album artist name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},

		// ---- Music: Track (Dbtrack) ----
		{
			Name:        "Dbtrack.Title",
			Type:        "string",
			Template:    "{{.Dbtrack.Title}}",
			Description: "Track title",
			Example:     "Time",
			Category:    "Music - Track",
		},
		{
			Name:        "Dbtrack.TrackNumber",
			Type:        "uint16",
			Template:    "{{.Dbtrack.TrackNumber}}",
			Description: "Track number on disc",
			Example:     "4",
			Category:    "Music - Track",
		},
		{
			Name:        "Dbtrack.DiscNumber",
			Type:        "uint16",
			Template:    "{{.Dbtrack.DiscNumber}}",
			Description: "Disc number",
			Example:     "1",
			Category:    "Music - Track",
		},

		// ---- Book (Dbbook) ----
		{
			Name:        "Dbbook.Title",
			Type:        "string",
			Template:    "{{.Dbbook.Title}}",
			Description: "Book title",
			Example:     "The Hobbit",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.OriginalTitle",
			Type:        "string",
			Template:    "{{.Dbbook.OriginalTitle}}",
			Description: "Original book title",
			Example:     "The Hobbit",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Year",
			Type:        "uint16",
			Template:    "{{.Dbbook.Year}}",
			Description: "Publication year",
			Example:     "1937",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.ISBN13",
			Type:        "string",
			Template:    "{{.Dbbook.ISBN13}}",
			Description: "ISBN-13 identifier",
			Example:     "9780261103344",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Publisher",
			Type:        "string",
			Template:    "{{.Dbbook.Publisher}}",
			Description: "Publisher",
			Example:     "George Allen & Unwin",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Language",
			Type:        "string",
			Template:    "{{.Dbbook.Language}}",
			Description: "Book language",
			Example:     "English",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.SeriesPosition",
			Type:        "string",
			Template:    "{{.Dbbook.SeriesPosition}}",
			Description: "Position in series",
			Example:     "1",
			Category:    "Book",
		},

		// ---- Book: Author (Author = database.Dbauthor) ----
		{
			Name:        "Author.Name",
			Type:        "string",
			Template:    "{{.Author.Name}}",
			Description: "Author name",
			Example:     "J.R.R. Tolkien",
			Category:    "Book - Author",
		},

		// ---- Book: Series (BookSeries = database.DbbookSeries) ----
		{
			Name:        "BookSeries.Name",
			Type:        "string",
			Template:    "{{.BookSeries.Name}}",
			Description: "Book series name",
			Example:     "Middle-earth",
			Category:    "Book - Series",
		},

		// ---- Audiobook (Dbaudiobook) ----
		{
			Name:        "Dbaudiobook.Title",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Title}}",
			Description: "Audiobook title",
			Example:     "The Hobbit",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Year",
			Type:        "uint16",
			Template:    "{{.Dbaudiobook.Year}}",
			Description: "Audiobook release year",
			Example:     "2012",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.ASIN",
			Type:        "string",
			Template:    "{{.Dbaudiobook.ASIN}}",
			Description: "Amazon/Audible ASIN",
			Example:     "B007978PI2",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Publisher",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Publisher}}",
			Description: "Publisher",
			Example:     "Recorded Books",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Language",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Language}}",
			Description: "Audiobook language",
			Example:     "English",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.SeriesName",
			Type:        "string",
			Template:    "{{.Dbaudiobook.SeriesName}}",
			Description: "Audiobook series name",
			Example:     "The Lord of the Rings",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.SeriesPosition",
			Type:        "string",
			Template:    "{{.Dbaudiobook.SeriesPosition}}",
			Description: "Position in series",
			Example:     "1",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.RuntimeMinutes",
			Type:        "int",
			Template:    "{{.Dbaudiobook.RuntimeMinutes}}",
			Description: "Total runtime in minutes",
			Example:     "660",
			Category:    "Audiobook",
		},

		// ---- Audiobook: Chapter (DbaudiobookChapter) ----
		{
			Name:        "DbaudiobookChapter.Title",
			Type:        "string",
			Template:    "{{.DbaudiobookChapter.Title}}",
			Description: "Chapter title",
			Example:     "An Unexpected Party",
			Category:    "Audiobook - Chapter",
		},
		{
			Name:        "DbaudiobookChapter.ChapterNumber",
			Type:        "uint16",
			Template:    "{{.DbaudiobookChapter.ChapterNumber}}",
			Description: "Chapter sequence number",
			Example:     "1",
			Category:    "Audiobook - Chapter",
		},
	}
}

func getNotificationFields() []TemplateField {
	return []TemplateField{
		// Dbmovie fields (nested struct) - same as parser
		{
			Name:        "Dbmovie.Title",
			Type:        "string",
			Template:    "{{.Dbmovie.Title}}",
			Description: "Movie title",
			Example:     "Inception",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Year",
			Type:        "int",
			Template:    "{{.Dbmovie.Year}}",
			Description: "Movie year",
			Example:     "2000",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.OriginalTitle",
			Type:        "string",
			Template:    "{{.Dbmovie.OriginalTitle}}",
			Description: "Original movie title",
			Example:     "Inception",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Overview",
			Type:        "string",
			Template:    "{{.Dbmovie.Overview}}",
			Description: "Movie plot summary",
			Example:     "A thief who steals secrets...",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Tagline",
			Type:        "string",
			Template:    "{{.Dbmovie.Tagline}}",
			Description: "Movie tagline",
			Example:     "Your mind is the scene of the crime",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Genres",
			Type:        "string",
			Template:    "{{.Dbmovie.Genres}}",
			Description: "Movie genres",
			Example:     "Action, Sci-Fi, Thriller",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Runtime",
			Type:        "int",
			Template:    "{{.Dbmovie.Runtime}}",
			Description: "Movie runtime in minutes",
			Example:     "148",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.ReleaseDate",
			Type:        "time",
			Template:    "{{.Dbmovie.ReleaseDate}}",
			Description: "Movie release date",
			Example:     "2010-07-16",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.Status",
			Type:        "string",
			Template:    "{{.Dbmovie.Status}}",
			Description: "Release status",
			Example:     "Released",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.OriginalLanguage",
			Type:        "string",
			Template:    "{{.Dbmovie.OriginalLanguage}}",
			Description: "Original language",
			Example:     "en",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.SpokenLanguages",
			Type:        "string",
			Template:    "{{.Dbmovie.SpokenLanguages}}",
			Description: "Spoken languages",
			Example:     "English, Japanese",
			Category:    "Movie Information",
		},
		{
			Name:        "Dbmovie.ImdbID",
			Type:        "string",
			Template:    "{{.Dbmovie.ImdbID}}",
			Description: "IMDB ID",
			Example:     "tt1375666",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.MoviedbID",
			Type:        "int",
			Template:    "{{.Dbmovie.MoviedbID}}",
			Description: "MovieDB ID",
			Example:     "27205",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.TraktID",
			Type:        "int",
			Template:    "{{.Dbmovie.TraktID}}",
			Description: "Trakt ID",
			Example:     "1390",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.FacebookID",
			Type:        "string",
			Template:    "{{.Dbmovie.FacebookID}}",
			Description: "Facebook page ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.InstagramID",
			Type:        "string",
			Template:    "{{.Dbmovie.InstagramID}}",
			Description: "Instagram ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.TwitterID",
			Type:        "string",
			Template:    "{{.Dbmovie.TwitterID}}",
			Description: "Twitter ID",
			Example:     "inception",
			Category:    "Movie External IDs",
		},
		{
			Name:        "Dbmovie.VoteAverage",
			Type:        "float32",
			Template:    "{{.Dbmovie.VoteAverage}}",
			Description: "Average user rating",
			Example:     "8.3",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.VoteCount",
			Type:        "int32",
			Template:    "{{.Dbmovie.VoteCount}}",
			Description: "Number of votes",
			Example:     "31280",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.Popularity",
			Type:        "float32",
			Template:    "{{.Dbmovie.Popularity}}",
			Description: "Popularity score",
			Example:     "108.6",
			Category:    "Movie Ratings",
		},
		{
			Name:        "Dbmovie.Budget",
			Type:        "int",
			Template:    "{{.Dbmovie.Budget}}",
			Description: "Production budget",
			Example:     "160000000",
			Category:    "Movie Financial",
		},
		{
			Name:        "Dbmovie.Revenue",
			Type:        "int",
			Template:    "{{.Dbmovie.Revenue}}",
			Description: "Box office revenue",
			Example:     "825532764",
			Category:    "Movie Financial",
		},
		{
			Name:        "Dbmovie.Poster",
			Type:        "string",
			Template:    "{{.Dbmovie.Poster}}",
			Description: "Poster image path",
			Example:     "/poster.jpg",
			Category:    "Movie Images",
		},
		{
			Name:        "Dbmovie.Backdrop",
			Type:        "string",
			Template:    "{{.Dbmovie.Backdrop}}",
			Description: "Backdrop image path",
			Example:     "/backdrop.jpg",
			Category:    "Movie Images",
		},
		{
			Name:        "Dbmovie.Slug",
			Type:        "string",
			Template:    "{{.Dbmovie.Slug}}",
			Description: "URL slug",
			Example:     "inception-2010",
			Category:    "Movie Information",
		},

		// Dbserie fields (nested struct)
		{
			Name:        "Dbserie.Seriename",
			Type:        "string",
			Template:    "{{.Dbserie.Seriename}}",
			Description: "Series name",
			Example:     "Breaking Bad",
			Category:    "Series Information",
		},
		{
			Name:        "Serie.Aliases",
			Type:        "string",
			Template:    "{{.Serie.Aliases}}",
			Description: "Alternative series names",
			Example:     "Breaking Bad, BB",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Overview",
			Type:        "string",
			Template:    "{{.Dbserie.Overview}}",
			Description: "Series plot summary",
			Example:     "A high school chemistry teacher...",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Status",
			Type:        "string",
			Template:    "{{.Dbserie.Status}}",
			Description: "Series status",
			Example:     "Ended",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Firstaired",
			Type:        "string",
			Template:    "{{.Dbserie.Firstaired}}",
			Description: "First air date",
			Example:     "2008-01-20",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Network",
			Type:        "string",
			Template:    "{{.Dbserie.Network}}",
			Description: "Broadcasting network",
			Example:     "AMC",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Runtime",
			Type:        "string",
			Template:    "{{.Dbserie.Runtime}}",
			Description: "Episode runtime",
			Example:     "47",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Language",
			Type:        "string",
			Template:    "{{.Dbserie.Language}}",
			Description: "Primary language",
			Example:     "en",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Genre",
			Type:        "string",
			Template:    "{{.Dbserie.Genre}}",
			Description: "Series genres",
			Example:     "Crime, Drama, Thriller",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Rating",
			Type:        "string",
			Template:    "{{.Dbserie.Rating}}",
			Description: "Content rating",
			Example:     "TV-MA",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Siterating",
			Type:        "string",
			Template:    "{{.Dbserie.Siterating}}",
			Description: "Site user rating",
			Example:     "9.5",
			Category:    "Series Ratings",
		},
		{
			Name:        "Dbserie.SiteratingCount",
			Type:        "string",
			Template:    "{{.Dbserie.SiteratingCount}}",
			Description: "Rating vote count",
			Example:     "1,654,876",
			Category:    "Series Ratings",
		},
		{
			Name:        "Dbserie.ImdbID",
			Type:        "string",
			Template:    "{{.Dbserie.ImdbID}}",
			Description: "IMDB ID",
			Example:     "tt0903747",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.ThetvdbID",
			Type:        "int",
			Template:    "{{.Dbserie.ThetvdbID}}",
			Description: "TVDB ID",
			Example:     "81189",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.TraktID",
			Type:        "int",
			Template:    "{{.Dbserie.TraktID}}",
			Description: "Trakt ID",
			Example:     "1388",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.TvrageID",
			Type:        "int",
			Template:    "{{.Dbserie.TvrageID}}",
			Description: "TVRage ID",
			Example:     "18164",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Facebook",
			Type:        "string",
			Template:    "{{.Dbserie.Facebook}}",
			Description: "Facebook page URL",
			Example:     "https://www.facebook.com/BreakingBad",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Instagram",
			Type:        "string",
			Template:    "{{.Dbserie.Instagram}}",
			Description: "Instagram URL",
			Example:     "https://www.instagram.com/breakingbad",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Twitter",
			Type:        "string",
			Template:    "{{.Dbserie.Twitter}}",
			Description: "Twitter URL",
			Example:     "https://twitter.com/BreakingBad_AMC",
			Category:    "Series External IDs",
		},
		{
			Name:        "Dbserie.Banner",
			Type:        "string",
			Template:    "{{.Dbserie.Banner}}",
			Description: "Banner image path",
			Example:     "/banner.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Poster",
			Type:        "string",
			Template:    "{{.Dbserie.Poster}}",
			Description: "Poster image path",
			Example:     "/poster.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Fanart",
			Type:        "string",
			Template:    "{{.Dbserie.Fanart}}",
			Description: "Fanart image path",
			Example:     "/fanart.jpg",
			Category:    "Series Images",
		},
		{
			Name:        "Dbserie.Identifiedby",
			Type:        "string",
			Template:    "{{.Dbserie.Identifiedby}}",
			Description: "Episode ID method",
			Example:     "ep",
			Category:    "Series Information",
		},
		{
			Name:        "Dbserie.Slug",
			Type:        "string",
			Template:    "{{.Dbserie.Slug}}",
			Description: "URL slug",
			Example:     "breaking-bad",
			Category:    "Series Information",
		},

		// DbserieEpisode fields (nested struct)
		{
			Name:        "DbserieEpisode.Title",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Title}}",
			Description: "Episode title",
			Example:     "Pilot",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Season",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Season}}",
			Description: "Season number",
			Example:     "1",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Episode",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Episode}}",
			Description: "Episode number",
			Example:     "1",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Identifier",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Identifier}}",
			Description: "Episode identifier",
			Example:     "S01E01",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Overview",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Overview}}",
			Description: "Episode summary",
			Example:     "Walter White begins cooking...",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.FirstAired",
			Type:        "time",
			Template:    "{{.DbserieEpisode.FirstAired}}",
			Description: "Original air date",
			Example:     "2008-01-20",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Runtime",
			Type:        "int",
			Template:    "{{.DbserieEpisode.Runtime}}",
			Description: "Episode runtime in minutes",
			Example:     "58",
			Category:    "Episode Information",
		},
		{
			Name:        "DbserieEpisode.Poster",
			Type:        "string",
			Template:    "{{.DbserieEpisode.Poster}}",
			Description: "Episode poster image",
			Example:     "/episode_poster.jpg",
			Category:    "Episode Images",
		},

		// Top-level inputNotifier fields
		{
			Name:        "Title",
			Type:        "string",
			Template:    "{{.Title}}",
			Description: "Media title",
			Example:     "Inception",
			Category:    "Basic Information",
		},
		{
			Name:        "Year",
			Type:        "string",
			Template:    "{{.Year}}",
			Description: "Release year",
			Example:     "2010",
			Category:    "Basic Information",
		},
		{
			Name:        "Season",
			Type:        "string",
			Template:    "{{.Season}}",
			Description: "Season number",
			Example:     "1",
			Category:    "Basic Information",
		},
		{
			Name:        "Episode",
			Type:        "string",
			Template:    "{{.Episode}}",
			Description: "Episode number",
			Example:     "5",
			Category:    "Basic Information",
		},
		{
			Name:        "Identifier",
			Type:        "string",
			Template:    "{{.Identifier}}",
			Description: "Media identifier",
			Example:     "S01E05",
			Category:    "Basic Information",
		},
		{
			Name:        "Series",
			Type:        "string",
			Template:    "{{.Series}}",
			Description: "Series name",
			Example:     "Breaking Bad",
			Category:    "Basic Information",
		},
		{
			Name:        "EpisodeTitle",
			Type:        "string",
			Template:    "{{.EpisodeTitle}}",
			Description: "Episode title",
			Example:     "Pilot",
			Category:    "Basic Information",
		},
		{
			Name:        "Configuration",
			Type:        "string",
			Template:    "{{.Configuration}}",
			Description: "Configuration name",
			Example:     "movies-4k",
			Category:    "Basic Information",
		},

		// Paths & Locations
		{
			Name:        "SourcePath",
			Type:        "string",
			Template:    "{{.SourcePath}}",
			Description: "Original file path",
			Example:     "/downloads/movie.mkv",
			Category:    "Paths & Locations",
		},
		{
			Name:        "Targetpath",
			Type:        "string",
			Template:    "{{.Targetpath}}",
			Description: "Final organized path",
			Example:     "/media/movies/Inception (2010)/Inception.mkv",
			Category:    "Paths & Locations",
		},
		{
			Name:        "Serie.Rootpath",
			Type:        "string",
			Template:    "{{.Serie.Rootpath}}",
			Description: "Root media path (series only)",
			Example:     "/media/tv/Breaking Bad",
			Category:    "Paths & Locations",
		},

		// External IDs
		{
			Name:        "Imdb",
			Type:        "string",
			Template:    "{{.Imdb}}",
			Description: "IMDB ID",
			Example:     "tt1375666",
			Category:    "External IDs",
		},
		{
			Name:        "Tvdb",
			Type:        "string",
			Template:    "{{.Tvdb}}",
			Description: "TVDB ID",
			Example:     "290434",
			Category:    "External IDs",
		},

		// Dates & Times
		{
			Name:        "Time",
			Type:        "string",
			Template:    "{{.Time}}",
			Description: "Processing timestamp",
			Example:     "2024-01-15 14:30:00",
			Category:    "Dates & Times",
		},
		// Processing Information
		{
			Name:        "ReplacedPrefix",
			Type:        "string",
			Template:    "{{.ReplacedPrefix}}",
			Description: "Prefix for replaced files",
			Example:     "Replaced: ",
			Category:    "Processing Information",
		},

		// Arrays & Lists
		{
			Name:        "Replaced",
			Type:        "[]string",
			Template:    "{{range .Replaced}}{{.}}{{end}}",
			Description: "List of replaced files",
			Example:     "old_file1.mkv, old_file2.mkv",
			Category:    "Arrays & Lists",
		},

		// Source (ParseInfo) fields (nested struct pointer) - subset for notifications
		{
			Name:        "Source.Title",
			Type:        "string",
			Template:    "{{.Source.Title}}",
			Description: "Parsed media title",
			Example:     "Breaking Bad",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Year",
			Type:        "uint16",
			Template:    "{{.Source.Year}}",
			Description: "Parsed release year",
			Example:     "2008",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Season",
			Type:        "int",
			Template:    "{{.Source.Season}}",
			Description: "Parsed season number",
			Example:     "1",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Episode",
			Type:        "int",
			Template:    "{{.Source.Episode}}",
			Description: "Parsed episode number",
			Example:     "1",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Quality",
			Type:        "string",
			Template:    "{{.Source.Quality}}",
			Description: "Video quality",
			Example:     "bluray",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Resolution",
			Type:        "string",
			Template:    "{{.Source.Resolution}}",
			Description: "Video resolution",
			Example:     "1080p",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Codec",
			Type:        "string",
			Template:    "{{.Source.Codec}}",
			Description: "Video codec",
			Example:     "x264",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Audio",
			Type:        "string",
			Template:    "{{.Source.Audio}}",
			Description: "Audio codec",
			Example:     "AC3",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.File",
			Type:        "string",
			Template:    "{{.Source.File}}",
			Description: "File path",
			Example:     "/path/to/file.mkv",
			Category:    "Source File",
		},
		{
			Name:        "Source.Runtime",
			Type:        "int",
			Template:    "{{.Source.Runtime}}",
			Description: "Runtime in minutes",
			Example:     "58",
			Category:    "Source File",
		},
		{
			Name:        "Source.Height",
			Type:        "int",
			Template:    "{{.Source.Height}}",
			Description: "Video height",
			Example:     "1080",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Width",
			Type:        "int",
			Template:    "{{.Source.Width}}",
			Description: "Video width",
			Example:     "1920",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Imdb",
			Type:        "string",
			Template:    "{{.Source.Imdb}}",
			Description: "IMDB ID from source",
			Example:     "tt0903747",
			Category:    "Source External IDs",
		},
		{
			Name:        "Source.Tvdb",
			Type:        "string",
			Template:    "{{.Source.Tvdb}}",
			Description: "TVDB ID from source",
			Example:     "tvdb81189",
			Category:    "Source External IDs",
		},
		{
			Name:        "Source.Identifier",
			Type:        "string",
			Template:    "{{.Source.Identifier}}",
			Description: "Source identifier",
			Example:     "S01E01",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Date",
			Type:        "string",
			Template:    "{{.Source.Date}}",
			Description: "Source release date",
			Example:     "2008-01-20",
			Category:    "Source Information",
		},
		{
			Name:        "Source.Proper",
			Type:        "bool",
			Template:    "{{.Source.Proper}}",
			Description: "Is proper release",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Extended",
			Type:        "bool",
			Template:    "{{.Source.Extended}}",
			Description: "Is extended version",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Repack",
			Type:        "bool",
			Template:    "{{.Source.Repack}}",
			Description: "Is repack release",
			Example:     "false",
			Category:    "Source Quality",
		},
		{
			Name:        "Source.Languages",
			Type:        "[]string",
			Template:    "{{range .Source.Languages}}{{.}}{{end}}",
			Description: "Available languages",
			Example:     "English, Spanish",
			Category:    "Source Information",
		},

		// ---- Music: Album (Dbalbum) ----
		{
			Name:        "Dbalbum.Title",
			Type:        "string",
			Template:    "{{.Dbalbum.Title}}",
			Description: "Album title",
			Example:     "Dark Side of the Moon",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Year",
			Type:        "uint16",
			Template:    "{{.Dbalbum.Year}}",
			Description: "Album release year",
			Example:     "1973",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Label",
			Type:        "string",
			Template:    "{{.Dbalbum.Label}}",
			Description: "Record label",
			Example:     "Harvest",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Genres",
			Type:        "string",
			Template:    "{{.Dbalbum.Genres}}",
			Description: "Album genres",
			Example:     "Progressive Rock",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.ReleaseType",
			Type:        "string",
			Template:    "{{.Dbalbum.ReleaseType}}",
			Description: "Release type (album, ep, single)",
			Example:     "album",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.Format",
			Type:        "string",
			Template:    "{{.Dbalbum.Format}}",
			Description: "Release format (cd, vinyl, digital)",
			Example:     "vinyl",
			Category:    "Music - Album",
		},
		{
			Name:        "Dbalbum.TotalTracks",
			Type:        "int",
			Template:    "{{.Dbalbum.TotalTracks}}",
			Description: "Number of tracks",
			Example:     "10",
			Category:    "Music - Album",
		},

		// ---- Music: Artist (Artist / AlbumArtist) ----
		{
			Name:        "Artist.Name",
			Type:        "string",
			Template:    "{{.Artist.Name}}",
			Description: "Primary artist name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},
		{
			Name:        "Artist.SortName",
			Type:        "string",
			Template:    "{{.Artist.SortName}}",
			Description: "Artist sort name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},
		{
			Name:        "AlbumArtist.Name",
			Type:        "string",
			Template:    "{{.AlbumArtist.Name}}",
			Description: "Album artist name",
			Example:     "Pink Floyd",
			Category:    "Music - Artist",
		},

		// ---- Music: Track (Dbtrack) ----
		{
			Name:        "Dbtrack.Title",
			Type:        "string",
			Template:    "{{.Dbtrack.Title}}",
			Description: "Track title",
			Example:     "Time",
			Category:    "Music - Track",
		},
		{
			Name:        "Dbtrack.TrackNumber",
			Type:        "uint16",
			Template:    "{{.Dbtrack.TrackNumber}}",
			Description: "Track number on disc",
			Example:     "4",
			Category:    "Music - Track",
		},
		{
			Name:        "Dbtrack.DiscNumber",
			Type:        "uint16",
			Template:    "{{.Dbtrack.DiscNumber}}",
			Description: "Disc number",
			Example:     "1",
			Category:    "Music - Track",
		},

		// ---- Book (Dbbook) ----
		{
			Name:        "Dbbook.Title",
			Type:        "string",
			Template:    "{{.Dbbook.Title}}",
			Description: "Book title",
			Example:     "The Hobbit",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.OriginalTitle",
			Type:        "string",
			Template:    "{{.Dbbook.OriginalTitle}}",
			Description: "Original book title",
			Example:     "The Hobbit",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Year",
			Type:        "uint16",
			Template:    "{{.Dbbook.Year}}",
			Description: "Publication year",
			Example:     "1937",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.ISBN13",
			Type:        "string",
			Template:    "{{.Dbbook.ISBN13}}",
			Description: "ISBN-13 identifier",
			Example:     "9780261103344",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Publisher",
			Type:        "string",
			Template:    "{{.Dbbook.Publisher}}",
			Description: "Publisher",
			Example:     "George Allen & Unwin",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.Language",
			Type:        "string",
			Template:    "{{.Dbbook.Language}}",
			Description: "Book language",
			Example:     "English",
			Category:    "Book",
		},
		{
			Name:        "Dbbook.SeriesPosition",
			Type:        "string",
			Template:    "{{.Dbbook.SeriesPosition}}",
			Description: "Position in series",
			Example:     "1",
			Category:    "Book",
		},

		// ---- Book: Author / Series ----
		{
			Name:        "Author.Name",
			Type:        "string",
			Template:    "{{.Author.Name}}",
			Description: "Author name",
			Example:     "J.R.R. Tolkien",
			Category:    "Book - Author",
		},
		{
			Name:        "BookSeries.Name",
			Type:        "string",
			Template:    "{{.BookSeries.Name}}",
			Description: "Book series name",
			Example:     "Middle-earth",
			Category:    "Book - Series",
		},

		// ---- Audiobook (Dbaudiobook) ----
		{
			Name:        "Dbaudiobook.Title",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Title}}",
			Description: "Audiobook title",
			Example:     "The Hobbit",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Year",
			Type:        "uint16",
			Template:    "{{.Dbaudiobook.Year}}",
			Description: "Audiobook release year",
			Example:     "2012",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.SeriesName",
			Type:        "string",
			Template:    "{{.Dbaudiobook.SeriesName}}",
			Description: "Audiobook series name",
			Example:     "The Lord of the Rings",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.SeriesPosition",
			Type:        "string",
			Template:    "{{.Dbaudiobook.SeriesPosition}}",
			Description: "Position in series",
			Example:     "1",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.ASIN",
			Type:        "string",
			Template:    "{{.Dbaudiobook.ASIN}}",
			Description: "Audible ASIN identifier",
			Example:     "B007978PI2",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Publisher",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Publisher}}",
			Description: "Publisher",
			Example:     "Recorded Books",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.Language",
			Type:        "string",
			Template:    "{{.Dbaudiobook.Language}}",
			Description: "Audiobook language",
			Example:     "English",
			Category:    "Audiobook",
		},
		{
			Name:        "Dbaudiobook.RuntimeMinutes",
			Type:        "int",
			Template:    "{{.Dbaudiobook.RuntimeMinutes}}",
			Description: "Runtime in minutes",
			Example:     "660",
			Category:    "Audiobook",
		},
		{
			Name:        "DbaudiobookChapter.Title",
			Type:        "string",
			Template:    "{{.DbaudiobookChapter.Title}}",
			Description: "Chapter title",
			Example:     "An Unexpected Party",
			Category:    "Audiobook - Chapter",
		},
		{
			Name:        "DbaudiobookChapter.ChapterNumber",
			Type:        "uint16",
			Template:    "{{.DbaudiobookChapter.ChapterNumber}}",
			Description: "Chapter number",
			Example:     "1",
			Category:    "Audiobook - Chapter",
		},
	}
}

type TemplateVerification struct {
	Valid       bool     `json:"valid"`
	Message     string   `json:"message"`
	Errors      []string `json:"errors,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

func verifyGoTemplate(templateStr, dataType string) TemplateVerification {
	verification := TemplateVerification{
		Valid:       true,
		Message:     "Template syntax appears valid",
		Errors:      []string{},
		Suggestions: []string{},
	}

	// The type only narrows the fields panel: every naming field is filled
	// (possibly empty) for every media type, so all of them are valid.
	kind := ngKind(dataType)
	fields, _ := getFieldsForDataType(kind)

	// Build a map of valid field references
	validFields := make(map[string]bool)

	// Add top-level fields based on data type
	switch kind {
	case "parser":
		{
			validFields["TitleSource"] = true
			validFields["EpisodeTitleSource"] = true
			validFields["Identifier"] = true
			validFields["Episodes"] = true
		}

	case "notification":
		{
			validFields["Title"] = true
			validFields["Year"] = true
			validFields["Season"] = true
			validFields["Episode"] = true
			validFields["Series"] = true
			validFields["EpisodeTitle"] = true
			validFields["Configuration"] = true
			validFields["SourcePath"] = true
			validFields["Targetpath"] = true
			validFields["Imdb"] = true
			validFields["Tvdb"] = true
			validFields["Time"] = true
			validFields["ReplacedPrefix"] = true
			validFields["Replaced"] = true
		}
	}

	// Add nested fields
	for _, field := range fields {
		validFields[field.Name] = true
	}

	// Check for common template syntax issues
	if strings.Contains(templateStr, "{{") && strings.Contains(templateStr, "}}") {
		// Check for unmatched braces
		openCount := strings.Count(templateStr, "{{")

		closeCount := strings.Count(templateStr, "}}")
		if openCount != closeCount {
			verification.Valid = false
			verification.Errors = append(
				verification.Errors,
				"Unmatched template braces - ensure every {{ has a matching }}",
			)
		}

		// Check for nested template syntax errors
		if strings.Contains(templateStr, "{{{") || strings.Contains(templateStr, "}}}") {
			verification.Valid = false
			verification.Errors = append(
				verification.Errors,
				"Invalid template syntax - avoid triple braces {{{ or }}}",
			)
		}

		// Find all template references
		templateRefs := findTemplateReferences(templateStr)

		// Verify field references
		for _, ref := range templateRefs {
			if ref == "" {
				continue
			}

			// Skip template functions and structures
			if isTemplateFunction(ref) {
				continue
			}

			// Extract field name from reference (handle range, if, with, etc.)
			fieldName := extractFieldName(ref)
			if fieldName != "" && !validFields[fieldName] {
				verification.Valid = false
				verification.Errors = append(
					verification.Errors,
					"Unknown field reference: "+fieldName,
				)

				// Suggest similar field names
				suggestion := findSimilarField(fieldName, validFields)
				if suggestion != "" {
					verification.Suggestions = append(
						verification.Suggestions,
						"Did you mean '"+suggestion+"' instead of '"+fieldName+"'?",
					)
				}
			}
		}

		// Add helpful suggestions
		if (dataType == "movie" && !strings.Contains(templateStr, ".Dbmovie.")) ||
			(dataType == "series" && !strings.Contains(templateStr, ".Dbserie.")) {
			verification.Suggestions = append(
				verification.Suggestions,
				"Consider using nested fields like {{.Dbmovie.Title}} or {{.Dbserie.Seriename}} for richer data",
			)
		}

		if !strings.Contains(templateStr, "{{if") && len(templateRefs) > 3 {
			verification.Suggestions = append(
				verification.Suggestions,
				"Consider using conditional statements like {{if .Field}}...{{end}} to handle optional fields",
			)
		}

		if kind == "notification" && !strings.Contains(templateStr, ".Replaced") &&
			!strings.Contains(templateStr, "range") {
			verification.Suggestions = append(
				verification.Suggestions,
				"For notifications, consider showing replaced files using {{range .Replaced}}{{.}}{{end}}",
			)
		}
	} else {
		verification.Valid = false
		verification.Errors = append(
			verification.Errors,
			"Template appears to be plain text - use Go template syntax with {{ }} for dynamic content",
		)
	}

	// The brace/field heuristics above miss real parse and execution errors
	// (e.g. an unterminated {{if}}), so render against the same sample data the
	// preview uses.
	if verification.Valid {
		var err error
		if kind == "notification" {
			_, err = structure.TestInputnotifier(templateStr)
		} else {
			_, err = structure.TestParsertype(templateStr, dataType)
		}

		if err != nil {
			verification.Valid = false
			verification.Errors = append(verification.Errors, "Template error: "+err.Error())
		}
	}

	if verification.Valid && len(verification.Errors) == 0 {
		verification.Message = "Template syntax is valid and references valid fields"
	} else if !verification.Valid {
		verification.Message = "Template has syntax or reference errors that need to be fixed"
	}

	return verification
}

func findTemplateReferences(template string) []string {
	var refs []string

	// Find all {{ ... }} patterns
	start := 0
	for {
		startIdx := strings.Index(template[start:], "{{")
		if startIdx == -1 {
			break
		}

		startIdx += start

		endIdx := strings.Index(template[startIdx:], "}}")
		if endIdx == -1 {
			break
		}

		endIdx += startIdx

		ref := strings.TrimSpace(template[startIdx+2 : endIdx])
		if ref != "" {
			refs = append(refs, ref)
		}

		start = endIdx + 2
	}

	return refs
}

func isTemplateFunction(ref string) bool {
	// Common Go template functions and structures
	functions := []string{
		"range",
		"if",
		"else",
		"end",
		"with",
		"printf",
		"eq",
		"ne",
		"lt",
		"le",
		"gt",
		"ge",
	}

	for _, fn := range functions {
		if strings.HasPrefix(ref, fn+" ") || ref == fn {
			return true
		}
	}

	// Variable assignments like $var := .Field
	if strings.Contains(ref, ":=") || strings.HasPrefix(ref, "$") {
		return true
	}

	return false
}

func extractFieldName(ref string) string {
	// Handle different template patterns
	ref = strings.TrimSpace(ref)

	// Skip template functions
	if isTemplateFunction(ref) {
		return ""
	}

	// Extract field from patterns like ".Field", "if .Field", "range .Array", etc.
	words := strings.FieldsSeq(ref)
	for word := range words {
		if strings.HasPrefix(word, ".") && len(word) > 1 {
			// Remove leading dot and extract the field path
			field := word[1:]
			// For nested fields like "Dbmovie.Title", we want the full path
			return field
		}
	}

	return ""
}

func findSimilarField(target string, validFields map[string]bool) string {
	target = strings.ToLower(target)

	bestMatch := ""
	bestScore := 0

	for field := range validFields {
		fieldLower := strings.ToLower(field)

		score := calculateSimilarity(target, fieldLower)
		if score > bestScore && score > 50 { // Only suggest if > 50% similar
			bestScore = score
			bestMatch = field
		}
	}

	return bestMatch
}

func calculateSimilarity(s1, s2 string) int {
	// Simple similarity calculation based on common characters
	if s1 == s2 {
		return 100
	}

	// Check if one contains the other
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 80
	}

	// Count common characters
	common := 0
	for _, char := range s1 {
		if strings.ContainsRune(s2, char) {
			common++
		}
	}

	maxLen := max(len(s2), len(s1))

	if maxLen == 0 {
		return 0
	}

	return (common * 100) / maxLen
}
