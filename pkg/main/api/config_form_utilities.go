package api

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// Alert creation utilities

// createAlert creates a standardized alert component.
func createAlert(message, alertType string) gomponents.Node {
	var icon string
	switch alertType {
	case "success":
		icon = "fas fa-check-circle"
	case "danger":
		icon = "fas fa-exclamation-triangle"
	case "warning":
		icon = "fas fa-exclamation-circle"
	case "info":
		icon = "fas fa-info-circle"
	default:
		icon = "fas fa-bell"
	}

	return html.Div(
		html.Class("alert alert-"+alertType+" alert-column alert-dismissible shadow-sm"),
		html.Style("border-radius: 10px; padding: 1rem 1.25rem;"),
		html.Role("alert"),
		html.Button(
			html.Type("button"),
			html.Class("btn-close"),
			html.Data("bs-dismiss", "alert"),
			html.Aria("label", "Close"),
		),
		html.Div(
			html.Class("d-flex align-items-center"),
			html.Div(
				html.Class("alert-icon me-3"),
				html.Style("font-size: 1.25rem;"),
				html.I(html.Class(icon)),
			),
			html.Div(
				html.Class("alert-message flex-grow-1"),
				html.Style("font-weight: 500; line-height: 1.4;"),
				gomponents.Text(message),
			),
		),
	)
}

// Form field creation utilities

// createFormField creates a standardized form field.
func createFormField(
	fieldType, name, value, placeholder string,
	options []gomponents.Node,
) gomponents.Node {
	switch fieldType {
	case "email", "password", "text", "url":
		baseAttrs := []gomponents.Node{
			html.Type(fieldType),
			html.Name(name),
			html.Class(ClassFormControl),
		}
		if value != "" {
			baseAttrs = append(baseAttrs, html.Value(value))
		}

		if placeholder != "" {
			baseAttrs = append(baseAttrs, html.Placeholder(placeholder))
		}

		return html.Input(append(baseAttrs, options...)...)

	case "number":
		baseAttrs := []gomponents.Node{
			html.Type(fieldTypeText),
			html.Name(name),
			html.Class(ClassFormControl),
			html.Value(value),
		}
		if placeholder != "" {
			baseAttrs = append(baseAttrs, html.Placeholder(placeholder))
		}

		return html.Input(append(baseAttrs, options...)...)

	case "textarea":
		baseAttrs := []gomponents.Node{
			html.Name(name),
			html.Class(ClassFormControl),
			html.Rows("3"),
		}
		if placeholder != "" {
			baseAttrs = append(baseAttrs, html.Placeholder(placeholder))
		}

		if value != "" {
			baseAttrs = append(baseAttrs, gomponents.Text(value))
		}

		return html.Textarea(append(baseAttrs, options...)...)

	case "checkbox":
		baseAttrs := []gomponents.Node{
			html.Type("checkbox"),
			html.Name(name),
			html.Class("form-check-input"),
		}
		if value == "true" || value == "on" {
			baseAttrs = append(baseAttrs, html.Checked())
		}

		return html.Input(append(baseAttrs, options...)...)

	default:
		return html.Input(
			html.Type("text"),
			html.Name(name),
			html.Class(ClassFormControl),
			html.Value(value),
		)
	}
}

// Button creation utilities

// createButton creates a standardized button component.
func createButton(text, buttonType, cssClass string, attrs ...gomponents.Node) gomponents.Node {
	buttonAttrs := []gomponents.Node{
		html.Type(buttonType),
		html.Class(cssClass),
		gomponents.Text(text),
	}

	buttonAttrs = append(buttonAttrs, attrs...)

	return html.Button(buttonAttrs...)
}

// Parsing utilities

// parseIntOrDefault parses a string to int with a default value.
func parseIntOrDefault(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}

	if parsed, err := strconv.Atoi(s); err == nil {
		return parsed
	}

	return defaultValue
}

// parseUintOrDefault parses a string to uint with a default value.
func parseUintOrDefault(s string, defaultValue uint) uint {
	if s == "" {
		return defaultValue
	}

	if parsed, err := strconv.ParseUint(s, 10, 32); err == nil {
		return uint(parsed)
	}

	return defaultValue
}

// Button utilities

// createRemoveButton creates a standardized remove button.
func createRemoveButton(float bool) gomponents.Node {
	floatText := ""
	if float {
		floatText = " float: right;"
	}

	return html.Button(
		html.Class(ClassBtnDanger+" btn-lg shadow-sm mt-2"),
		html.Style(
			"background: linear-gradient(135deg, #dc3545 0%, #c82333 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;"+floatText,
		),
		html.Type("button"),
		html.I(html.Class("fa-solid fa-trash me-2")),
		gomponents.Text("Remove Item"),
		// Attr("onclick", "if(confirm('Are you sure you want to remove this item?') && this.parentElement.parentElement.parentElement) this.parentElement.parentElement.parentElement.remove()"),
		// gomponents.Attr("onclick", "this.parentElement.parentElement.parentElement.remove()"),
		gomponents.Attr("onclick", `
			// For array items, remove the card container
			const arrayItem = this.closest('.card');
			if (arrayItem) {
				arrayItem.remove();
				return;
			}
			
			// For main forms, remove the card container (ID ends with _card)
			const cardElement = this.closest('[id$="_card"]');
			if (cardElement) {
				cardElement.remove();
				return;
			}
			
			// Fallback to parent element removal
			this.parentElement.remove();
		`),
		// Attr("onmouseover", "this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(220, 53, 69, 0.4)'"),
		// Attr("onmouseout", "this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'"),
	)
}

// createAddButton creates a standardized add button with HTMX.
func createAddButton(text, target, endpoint, csrfToken string) gomponents.Node {
	return html.Button(
		html.Class(ClassBtnSuccess+" btn-lg shadow-sm mt-2"),
		html.Style(
			"background: linear-gradient(135deg, #28a745 0%, #20c997 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;",
		),
		html.Type("button"),
		html.I(html.Class("fa-solid fa-plus me-2")),
		gomponents.Text(text),
		hx.Target(target),
		hx.Swap("beforeend"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
		gomponents.Attr(
			"onmouseover",
			"this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(40, 167, 69, 0.4)'",
		),
		gomponents.Attr(
			"onmouseout",
			"this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'",
		),
	)
}

// Form label creation

// createFormLabel creates a standardized form label.
func createFormLabel(forID, text string, checkbox bool) gomponents.Node {
	cssClass := ClassFormLabel
	if checkbox {
		cssClass = ClassFormCheckLabel
	}

	return html.Label(
		html.Class(cssClass),
		html.For(forID),
		gomponents.Text(text),
	)
}

// Form button utilities

// createSubmitButton creates a standardized submit button with HTMX.
func createSubmitButton(text, target, endpoint, csrfToken string) gomponents.Node {
	return html.Button(
		html.Class(ClassBtnPrimary+" btn-lg shadow-sm"),
		html.Style(
			"background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;",
		),
		html.I(html.Class("fa-solid fa-save me-2")),
		gomponents.Text(text),
		html.Type("submit"),
		hx.Target(target),
		hx.Swap("innerHTML"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
		gomponents.Attr(
			"onmouseover",
			"this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(13, 110, 253, 0.4)'",
		),
		gomponents.Attr(
			"onmouseout",
			"this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'",
		),
	)
}

// createBackupSubmitButton creates a submit button that first copies the
// current config.toml to ./backup/ (see config.BackupConfig) before applying
// the same save the plain submit button would. Posts to the same endpoint
// with ?backup=1, which HandleConfigUpdate checks for.
func createBackupSubmitButton(text, target, endpoint, csrfToken string) gomponents.Node {
	return html.Button(
		html.Class(ClassBtnSecondary+" btn-lg shadow-sm ms-3"),
		html.Style(
			"background: linear-gradient(135deg, #6f42c1 0%, #4b2e83 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem; color: #fff;",
		),
		html.I(html.Class("fa-solid fa-shield-halved me-2")),
		gomponents.Text(text),
		html.Type("submit"),
		hx.Target(target),
		hx.Swap("innerHTML"),
		hx.Post(endpoint+"?backup=1"),
		hx.Headers(createHTMXHeaders(csrfToken)),
		gomponents.Attr(
			"onmouseover",
			"this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(111, 66, 193, 0.4)'",
		),
		gomponents.Attr(
			"onmouseout",
			"this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'",
		),
	)
}

// createResetButton creates a standardized reset button.
func createResetButton(text string) gomponents.Node {
	return html.Button(
		html.Type("button"),
		html.Class(ClassBtnSecondary+" btn-lg shadow-sm ms-3"),
		html.Style(
			"background: linear-gradient(135deg, #6c757d 0%, #495057 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;",
		),
		html.I(html.Class("fa-solid fa-undo me-2")),
		gomponents.Text(text),
		gomponents.Attr(
			"onclick",
			"confirmAction('Reset fields', 'Reset all fields? Unsaved changes will be lost.', function(){ window.location.reload(); });",
		),
		gomponents.Attr(
			"onmouseover",
			"this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(108, 117, 125, 0.4)'",
		),
		gomponents.Attr(
			"onmouseout",
			"this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'",
		),
	)
}

// createHTMXHeaders creates standardized HTMX headers with CSRF token.
func createHTMXHeaders(csrfToken string) string {
	return "{\"X-CSRF-Token\": \"" + csrfToken + "\"}"
}

// Form group utilities

// createFormSubmitGroup creates a standardized form submit group with HTMX.
func createFormSubmitGroup(text, target, endpoint, csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("form-group submit-group-enhanced mt-5 p-4 rounded-3"),
		html.Style(
			"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6; text-align: center;",
		),
		html.H5(
			html.Class("mb-3 text-muted"),
			html.I(html.Class("fa-solid fa-cogs me-2")),
			gomponents.Text("Configuration Actions"),
		),
		html.Div(
			html.Class("d-flex justify-content-center align-items-center flex-wrap gap-0"),
			createSubmitButton(text, target, endpoint, csrfToken),
			createBackupSubmitButton("Backup & Save Configuration", target, endpoint, csrfToken),
			createResetButton("Reset"),
		),
	)
}

// fieldHelpLabels are line prefixes rendered as bold labels in field help.
var fieldHelpLabels = []string{
	"Default", "Example", "Examples", "Recommended", "Range", "Typical range", "Note", "Format", "Warning", "Tip",
}

var fieldHelpURL = regexp.MustCompile(`https?://[^\s'")]+`)

// renderFieldHelp renders a config field's help text: the first line as a
// short hint and, if there is more, a "More" toggle that opens the rest as a
// formatted box (lists, bold labels, links). id must be unique on the page.
func renderFieldHelp(id, text string) gomponents.Node {
	text = strings.NewReplacer("\r\n", "\n", "\r", "\n", `\n`, "\n").Replace(text)

	var lines []string

	for l := range strings.SplitSeq(text, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}

	if len(lines) == 0 {
		return nil
	}

	hint, rest := lines[0], lines[1:]
	if utf8.RuneCountInString(hint) > fieldHelpHintLen {
		hint, rest = truncateText(hint, fieldHelpHintLen), lines
	}

	hintNode := html.Small(html.Class("form-text text-muted field-help-hint"), gomponents.Text(hint))
	if len(rest) == 0 {
		return html.Div(html.Class("mt-1"), hintNode)
	}

	targetID := "help-" + id

	return html.Div(
		html.Class("mt-1"),
		html.Div(
			html.Class("d-flex align-items-baseline flex-wrap gap-1"),
			hintNode,
			html.Button(
				html.Type("button"),
				html.Class("btn btn-link btn-sm p-0 field-help-toggle collapsed"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#"+targetID),
				gomponents.Attr("aria-expanded", "false"),
				gomponents.Attr("aria-controls", targetID),
				html.Span(html.Class("when-collapsed"), gomponents.Text("More")),
				html.Span(html.Class("when-open"), gomponents.Text("Less")),
				html.I(html.Class("fas fa-chevron-down ms-1")),
			),
		),
		html.Div(
			html.ID(targetID),
			html.Class("collapse"),
			html.Div(html.Class("field-help mt-2"), gomponents.Group(fieldHelpBody(rest))),
		),
	)
}

// fieldHelpHintLen is the longest first line shown as the hint as is.
const fieldHelpHintLen = 120

// fieldHelpBody turns help lines into paragraphs, "- " lists, headings
// (lines ending in ':') and bold "Label: value" lines.
func fieldHelpBody(lines []string) []gomponents.Node {
	var (
		out  []gomponents.Node
		list []gomponents.Node
	)

	flush := func() {
		if len(list) > 0 {
			out = append(out, html.Ul(gomponents.Group(list)))
			list = nil
		}
	}

	for _, l := range lines {
		if item, ok := strings.CutPrefix(l, "- "); ok {
			list = append(list, html.Li(fieldHelpText(item)...))
			continue
		}

		flush()

		if strings.HasSuffix(l, ":") {
			out = append(out, html.Div(html.Class("field-help-heading"), gomponents.Text(l)))
			continue
		}

		if label, value, ok := strings.Cut(l, ":"); ok && slices.ContainsFunc(fieldHelpLabels, func(p string) bool {
			return strings.EqualFold(p, strings.TrimSpace(label))
		}) {
			out = append(out, html.P(append([]gomponents.Node{html.Strong(gomponents.Text(label + ":"))},
				fieldHelpText(value)...)...))

			continue
		}

		out = append(out, html.P(fieldHelpText(l)...))
	}

	flush()

	return out
}

// fieldHelpText renders s with its URLs as links.
func fieldHelpText(s string) []gomponents.Node {
	var (
		nodes []gomponents.Node
		last  int
	)

	for _, m := range fieldHelpURL.FindAllStringIndex(s, -1) {
		nodes = append(nodes, gomponents.Text(s[last:m[0]]),
			html.A(html.Href(s[m[0]:m[1]]), html.Target("_blank"), html.Rel("noopener"), gomponents.Text(s[m[0]:m[1]])))
		last = m[1]
	}

	return append(nodes, gomponents.Text(s[last:]))
}

// choiceOption is one choice of a multi-choice field.
type choiceOption struct {
	Value, Label, Note string
}

// renderChoiceChips renders a multi-choice field as toggle chips (checkboxes
// named name, posted like a multiple select). Selected values that aren't
// among opts stay visible and selected, marked as unknown, so saving doesn't
// silently drop them.
func renderChoiceChips(name string, opts []choiceOption, selected []string) gomponents.Node {
	known := make(map[string]bool, len(opts))
	for _, o := range opts {
		known[o.Value] = true
	}

	for _, v := range selected {
		if v != "" && !known[v] {
			opts = append(opts, choiceOption{Value: v, Label: v, Note: "not a known value"})
			known[v] = true
		}
	}

	chips := make([]gomponents.Node, 0, 2*len(opts))
	for i, o := range opts {
		id := fmt.Sprintf("%s-chip-%d", name, i)

		label := []gomponents.Node{gomponents.Text(o.Label)}
		if o.Label != o.Value {
			label = append(label, html.Span(html.Class("choice-chip-value"), gomponents.Text(o.Value)))
		}

		class := "btn btn-sm btn-outline-primary rounded-pill choice-chip"
		if o.Note != "" {
			class = "btn btn-sm btn-outline-warning rounded-pill choice-chip"

			label = append(label, html.Span(html.Class("choice-chip-value"), gomponents.Text("– "+o.Note)))
		}

		chips = append(chips,
			html.Input(html.Type("checkbox"), html.Class("btn-check"), html.ID(id), html.Name(name), html.Value(o.Value),
				gomponents.Attr("autocomplete", "off"), gomponents.If(slices.Contains(selected, o.Value), html.Checked())),
			html.Label(html.Class(class), html.For(id), gomponents.Group(label)),
		)
	}

	return html.Div(html.Class("d-flex flex-wrap gap-2 choice-chips"), gomponents.Attr("role", "group"), gomponents.Group(chips))
}

// choiceOptionsFrom builds chip options from a form field's option map:
// "values" with "options" as labels (convertSelectOptionsToMap), or
// "options" with optional "labels".
func choiceOptionsFrom(options map[string][]string, sortByValue bool) []choiceOption {
	values := options["options"]
	labels := options["labels"]

	if v := options["values"]; len(v) == len(values) && len(v) > 0 {
		values, labels = v, options["options"]
	}

	out := make([]choiceOption, 0, len(values))
	for i, v := range values {
		label := v
		if len(labels) == len(values) {
			label = labels[i]
		}

		out = append(out, choiceOption{Value: v, Label: label})
	}

	if sortByValue && len(labels) != len(values) {
		slices.SortFunc(out, func(a, b choiceOption) int { return strings.Compare(a.Value, b.Value) })
	}

	return out
}

// imdbTitleTypes are IMDb's title types (title.basics.tsv titleType), the
// values init_imdb's indexed_types matches exactly (case-sensitive).
var imdbTitleTypes = []choiceOption{
	{Value: "movie", Label: "Movie"},
	{Value: "tvMovie", Label: "TV movie"},
	{Value: "tvSeries", Label: "TV series"},
	{Value: "tvMiniSeries", Label: "TV mini series"},
	{Value: "tvSpecial", Label: "TV special"},
	{Value: "video", Label: "Video"},
	{Value: "short", Label: "Short"},
	{Value: "tvShort", Label: "TV short"},
	{Value: "tvEpisode", Label: "TV episode"},
	{Value: "videoGame", Label: "Video game"},
}

// helperToolFor names the helper tool for a config field, by display name:
// "naming" for media naming templates, "notification" for notification
// title/message templates, "cron" for scheduler cron fields, "regex" for
// regex patterns, else "".
func helperToolFor(displayName string) string {
	switch {
	case displayName == "File Naming Template":
		return "naming"
	case displayName == "Notification Title Template", displayName == "Notification Message Template":
		return "notification"
	case strings.HasSuffix(displayName, "Cron Schedule"):
		return "cron"
	case displayName == "Required Pattern Matches", displayName == "Rejected Pattern Matches", displayName == "IRC Announce Regex":
		return "regex"
	}

	return ""
}

// withHelperTool puts a button next to a config input that opens its helper
// tool in the overlay (helperToolScript), started from the field's value.
func withHelperTool(tool, fieldName string, input gomponents.Node) gomponents.Node {
	label := helperToolLabel(tool)

	return html.Div(
		html.Class("input-group"),
		input,
		html.Button(
			html.Type("button"),
			html.Class("btn btn-outline-primary"),
			html.Title(label),
			gomponents.Attr("aria-label", label),
			gomponents.Attr("onclick", "openHelperTool("+jsString(tool)+", "+jsString(fieldName)+")"),
			html.I(html.Class("fa-solid fa-wand-magic-sparkles")),
		),
	)
}

func helperToolLabel(tool string) string {
	switch tool {
	case "cron":
		return "Open the cron generator"
	case "regex":
		return "Open the regex tester"
	}

	return "Open the naming generator"
}

// helperToolRowButton is the helper button of one row of an array field
// (all rows share the field name, so the button passes its row's input).
func helperToolRowButton(tool string) string {
	label := helperToolLabel(tool)

	return `<button class="btn btn-outline-primary me-2" type="button" title="` + label + `" aria-label="` + label +
		`" onclick="openHelperTool('` + tool + `', this.parentElement.querySelector('input'))"><i class="fa-solid fa-wand-magic-sparkles"></i></button>`
}

// helperToolScript defines openHelperTool: it shows a helper tool page
// (?embed=1) in a modal and writes the result back into the config field.
func helperToolScript() gomponents.Node {
	return gomponents.Group([]gomponents.Node{
		html.StyleEl(gomponents.Raw(`.helper-embed{background:#f5f7fb}.helper-embed .page-header-enhanced{display:none}` +
			`#helperToolModal iframe{width:100%;height:75vh;border:0;display:block}`)),
		html.Script(gomponents.Raw(helperToolJS)),
	})
}

const helperToolJS = `
(function () {
	var tools = {
		naming: { url: '/api/admin/naming-generator', title: 'Naming Generator', result: 'ngTemplate' },
		notification: { url: '/api/admin/naming-generator', title: 'Naming Generator: notification template', result: 'ngTemplate' },
		cron: { url: '/api/admin/crongen', title: 'Cron Expression Generator', result: 'cronExpression' },
		regex: { url: '/api/admin/regex-tester', title: 'Regex Tester', result: 'rtPattern' }
	};
	// The naming field sits in media_main_<type>_<index>, notifications in
	// media_<type>_<index>_notification_<n>.
	var mediaTypes = { movies: 'movie', series: 'series', music: 'music', books: 'book', audiobooks: 'audiobook' };

	function modal() {
		var el = document.getElementById('helperToolModal');
		if (el) { return el; }
		el = document.createElement('div');
		el.className = 'modal fade';
		el.id = 'helperToolModal';
		el.tabIndex = -1;
		el.setAttribute('aria-labelledby', 'helperToolTitle');
		el.innerHTML =
			'<div class="modal-dialog modal-xl modal-dialog-centered modal-fullscreen-lg-down"><div class="modal-content">' +
			'<div class="modal-header py-2"><h5 class="modal-title" id="helperToolTitle"></h5>' +
			'<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button></div>' +
			'<div class="modal-body p-0"><iframe title="Helper tool"></iframe></div>' +
			'<div class="modal-footer py-2"><span class="small text-muted me-auto" id="helperToolField"></span>' +
			'<a class="btn btn-outline-secondary" id="helperToolTab" target="_blank" rel="noopener"><i class="fa-solid fa-up-right-from-square me-1"></i>Open as page</a>' +
			'<button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>' +
			'<button type="button" class="btn btn-primary" id="helperToolApply"><i class="fa-solid fa-check me-1"></i>Use this value</button>' +
			'</div></div></div>';
		document.body.appendChild(el);
		// Unload the tool when closed so the next field starts fresh.
		el.addEventListener('hidden.bs.modal', function () { el.querySelector('iframe').src = 'about:blank'; });
		return el;
	}

	// field is the input's name, or the input itself (rows of array fields
	// share one name).
	window.openHelperTool = function (tool, field) {
		var cfg = tools[tool];
		if (typeof field === 'string') { field = document.querySelector('[name="' + CSS.escape(field) + '"]'); }
		if (!cfg || !field) { return; }
		var fieldName = field.name;

		var params = new URLSearchParams({ value: field.value });
		if (tool === 'notification') {
			params.set('type', 'notification');
		} else if (tool === 'regex') {
			params.set('tab', 'pattern');
		} else if (tool === 'naming') {
			var m = /^media_(?:main_)?([a-z]+)_/.exec(fieldName);
			params.set('type', (m && mediaTypes[m[1]]) || 'movie');
		}

		var el = modal();
		el.querySelector('#helperToolTitle').textContent = cfg.title;
		el.querySelector('#helperToolField').textContent = 'Result goes to ' + fieldName.replace(/_/g, ' › ');
		el.querySelector('#helperToolTab').href = cfg.url + '?' + params.toString();
		params.set('embed', '1');
		var frame = el.querySelector('iframe');
		frame.src = cfg.url + '?' + params.toString();

		el.querySelector('#helperToolApply').onclick = function () {
			var doc = frame.contentDocument;
			var out = doc && doc.getElementById(cfg.result);
			if (!out) { return; }
			field.value = out.value.trim();
			field.dispatchEvent(new Event('input', { bubbles: true }));
			field.dispatchEvent(new Event('change', { bubbles: true }));
			bootstrap.Modal.getOrCreateInstance(el).hide();
			field.focus();
			if (typeof showToaster === 'function') { showToaster('success', 'Value applied - save the form to keep it'); }
		};

		bootstrap.Modal.getOrCreateInstance(el).show();
	};

	// openPriorityEditor shows the quality priority editor for a profile. The
	// editor saves the reorder rules itself; once it has, closing the overlay
	// runs onSaved (the quality page refreshes its Reorder Rules from it).
	window.openPriorityEditor = function (profile, onSaved) {
		var el = modal(), apply = el.querySelector('#helperToolApply'), close = el.querySelector('.modal-footer [data-bs-dismiss]');
		var url = '/api/admin/quality-priorities?profile=' + encodeURIComponent(profile);
		el.querySelector('#helperToolTitle').textContent = 'Quality priorities: ' + profile;
		el.querySelector('#helperToolField').textContent = 'Drag into the wanted order, Generate Rules, then save them in the editor.';
		el.querySelector('#helperToolTab').href = url;
		apply.classList.add('d-none');
		close.textContent = 'Close';
		var frame = el.querySelector('iframe');
		frame.src = url + '&embed=1';
		el.addEventListener('hide.bs.modal', function done() {
			el.removeEventListener('hide.bs.modal', done);
			apply.classList.remove('d-none');
			close.textContent = 'Cancel';
			var doc = frame.contentDocument;
			if (doc && doc.querySelector('[data-qp-saved]') && typeof onSaved === 'function') { onSaved(); }
		});
		bootstrap.Modal.getOrCreateInstance(el).show();
	};

	// refreshReorderRules reloads a quality profile's Reorder Rules section
	// from the server, leaving the page's other (maybe unsaved) fields alone.
	window.refreshReorderRules = function (profile) {
		var id = 'collapsereorder-quality-' + profile;
		fetch(location.pathname + location.search, { credentials: 'same-origin' })
			.then(function (r) { return r.text(); })
			.then(function (text) {
				var doc = new DOMParser().parseFromString(text, 'text/html');
				var fresh = doc.getElementById(id), cur = document.getElementById(id);
				if (!fresh || !cur) { return; }
				cur.innerHTML = fresh.innerHTML;
				cur.classList.add('show');
				var head = document.getElementById('headingreorder-quality-' + profile), freshHead = doc.getElementById('headingreorder-quality-' + profile);
				if (head && freshHead) { head.innerHTML = freshHead.innerHTML; }
				if (window.htmx) { htmx.process(cur); }
				if (typeof showToaster === 'function') { showToaster('success', 'Reorder rules of ' + profile + ' updated'); }
			});
	};
})();
`
