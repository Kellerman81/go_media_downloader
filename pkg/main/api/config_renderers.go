package api

import (
	"encoding/json"
	"fmt"
	html2 "html"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// templateConfigInfo maps template field names to their configuration page URLs and types.
type templateConfigInfo struct {
	URL  string // The config page URL
	Type string // The config type for the preview endpoint
}

// getTemplateConfigInfo returns the config URL and type for a template field name.
// Returns empty struct if the field is not a template reference field.
func getTemplateConfigInfo(fieldName string) templateConfigInfo {
	mapping := map[string]templateConfigInfo{
		"TemplatePath":       {URL: "/api/admin/config/paths", Type: "path"},
		"TemplatePathNzb":    {URL: "/api/admin/config/paths", Type: "path"},
		"TemplateList":       {URL: "/api/admin/config/lists", Type: "list"},
		"TemplateQuality":    {URL: "/api/admin/config/quality", Type: "quality"},
		"TemplateScheduler":  {URL: "/api/admin/config/scheduler", Type: "scheduler"},
		"TemplateIndexer":    {URL: "/api/admin/config/indexers", Type: "indexer"},
		"TemplateDownloader": {URL: "/api/admin/config/downloader", Type: "downloader"},
		"TemplateRegex":      {URL: "/api/admin/config/regex", Type: "regex"},
		"MapNotification":    {URL: "/api/admin/config/notifications", Type: "notification"},
	}

	return mapping[fieldName]
}

// isTemplateField returns true if the field name is a template reference field.
// func isTemplateField(fieldName string) bool {
// 	info := getTemplateConfigInfo(fieldName)
// 	return info.URL != ""
// }

// renderConfigPreviewReadonly wraps a config form in a readonly preview container.
// This is used for the modal preview of template configurations.
// Note: The JavaScript to expand collapsed sections is in page_frame.go (expandAllCollapsedSections)
// and runs after the fetch completes, since inline scripts don't execute when inserted via innerHTML.
// CSS disables form inputs while keeping collapse buttons and scrolling interactive.
func renderConfigPreviewReadonly(title string, content gomponents.Node) gomponents.Node {
	return html.Div(
		html.Class("config-preview-readonly"),
		html.Div(
			html.Class("preview-header mb-3 pb-2 border-bottom"),
			html.H5(
				html.Class("text-primary mb-0"),
				html.I(html.Class("fas fa-eye me-2")),
				gomponents.Text(title),
			),
			html.Small(
				html.Class("text-muted"),
				gomponents.Text("Read-only preview - click headers to expand/collapse"),
			),
		),
		html.Div(
			html.Class("preview-content"),
			html.Style("max-height: 60vh; overflow-y: auto;"),
			content,
		),
		// CSS to disable form inputs but keep collapse buttons and scrolling working
		html.StyleEl(
			gomponents.Raw(`
				.config-preview-readonly input,
				.config-preview-readonly select,
				.config-preview-readonly textarea,
				.config-preview-readonly .btn-danger,
				.config-preview-readonly .btn-primary,
				.config-preview-readonly .btn-secondary:not([data-bs-toggle="collapse"]),
				.config-preview-readonly .selectwithpreview button {
					pointer-events: none;
					opacity: 0.7;
				}
				.config-preview-readonly .accordion-button,
				.config-preview-readonly [data-bs-toggle="collapse"] {
					pointer-events: auto;
					cursor: pointer;
				}
			`),
		),
	)
}

// renderEnhancedPageHeader creates a standardized enhanced page header with gradient background.
func renderEnhancedPageHeader(iconClass, title, subtitle string) gomponents.Node {
	return html.Div(
		html.Class("page-header-enhanced"),
		html.Div(
			html.Class("header-content"),
			html.Div(
				html.Class("header-icon-wrapper"),
				html.I(html.Class(iconClass+" header-icon")),
			),
			html.Div(
				html.Class("header-text"),
				html.H2(html.Class("header-title"), gomponents.Text(title)),
				html.P(html.Class("header-subtitle"), gomponents.Text(subtitle)),
			),
		),
	)
}

// renderHTMXSubmitButton creates a standardized HTMX form submit button.
func renderHTMXSubmitButton(
	buttonText, targetID, endpoint, formID, csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("form-group submit-group"),
		html.Button(
			html.Class("btn btn-primary"),
			gomponents.Text(buttonText),
			html.Type("button"),
			hx.Target("#"+targetID),
			hx.Swap("innerHTML"),
			hx.Post(endpoint),
			hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
			hx.Include("#"+formID),
		),
	)
}

// renderGenericConfigSection creates a standardized config section.
func renderGenericConfigSection[T any](
	configs []T,
	csrfToken string,
	options RenderConfigOptions,
	renderItemFunc func(int, T, string) gomponents.Node,
) gomponents.Node {
	items := getNodeSlice()
	defer putNodeSlice(items)

	for i, config := range configs {
		items = append(items, renderItemFunc(i, config, csrfToken))
	}

	containerContent := getNodeSlice()
	defer putNodeSlice(containerContent)

	// Add existing items
	containerContent = append(containerContent, items...)

	// Create the complete container
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-"+options.Icon+" header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(options.Title)),
					html.P(html.Class("header-subtitle"), gomponents.Text(options.Subtitle)),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.Div(
				append([]gomponents.Node{html.ID(options.FormContainer)}, containerContent...)...),
			createAddButton(
				options.AddButtonText,
				"#"+options.FormContainer,
				options.AddEndpoint,
				csrfToken,
			),

			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				options.SubmitEndpoint,
				csrfToken,
			),
			html.Div(html.ID("addalert")),
		),
	)
}

// renderConfigSection creates a generic config section with form elements.
func renderConfigSection[T any](
	configList []T,
	csrfToken string,
	options ConfigSectionOptions,
	renderForm func(int, *T) gomponents.Node,
) gomponents.Node {
	elements := make([]gomponents.Node, 0, len(configList))
	for i, config := range configList {
		elements = append(elements, renderForm(i, &config))
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-"+options.SectionIcon+" header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(options.SectionTitle)),
					html.P(html.Class("header-subtitle"), gomponents.Text(options.SectionSubtitle)),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.Div(
				html.ID(options.ContainerID),
				gomponents.Group(elements),
			),
			createAddButton(
				options.AddButtonText,
				"#"+options.ContainerID,
				options.AddFormPath,
				csrfToken,
			),
			createFormSubmitGroup(
				"Save Configuration",
				"#addalert",
				options.UpdatePath,
				csrfToken,
			),
			html.Div(html.ID("addalert")),
			html.Script(
				gomponents.Raw(`
					document.addEventListener('DOMContentLoaded', function() {
						// Update collapse indicators when cards are toggled
						document.querySelectorAll('[data-bs-toggle="collapse"]').forEach(function(trigger) {
							var targetId = trigger.getAttribute('data-bs-target');
							var indicator = trigger.querySelector('.badge');
							var targetEl = document.querySelector(targetId);
							
							if (targetEl && indicator) {
								targetEl.addEventListener('show.bs.collapse', function() {
									indicator.textContent = '▼';
								});
								targetEl.addEventListener('hide.bs.collapse', function() {
									indicator.textContent = '▶';
								});
							}
						});
					});
				`),
			),
		),
	)
}

func renderArrayItemFormWithIndex(
	prefix string,
	i int,
	headerText string,
	config any,
	fields []FormFieldDefinition,
) gomponents.Node {
	group := prefix + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px; padding: 10px;"),
		html.Div(
			html.Class(ClassCardHeader),
			gomponents.Attr(
				"style",
				"cursor: pointer; display: flex; justify-content: space-between; align-items: center;",
			),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			gomponents.Text(headerText),
			html.Span(
				html.Class(ClassBadge),
				gomponents.Text("▼"),
			),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// createRemoveButton(true),
				gomponents.Group(formGroups),
			),
		),
	)
}

func renderArrayItemFormWithNameAndIndex(
	namePrefix string,
	mainname string,
	i int,
	headerText string,
	config any,
	fields []FormFieldDefinition,
) gomponents.Node {
	group := namePrefix + "_" + mainname + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px; padding: 10px;"),
		html.Div(
			html.Class(ClassCardHeader),
			gomponents.Attr(
				"style",
				"cursor: pointer; display: flex; justify-content: space-between; align-items: center;",
			),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			gomponents.Text(headerText+" "+mainname),
			html.Span(
				html.Class(ClassBadge),
				gomponents.Text("▼"),
			),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// createRemoveButton(true),
				gomponents.Group(formGroups),
			),
		),
	)
}

// renderConfigGroup creates a collapsible group of configuration fields.
func renderConfigGroup(
	title, id string,
	expanded bool,
	fields []FormFieldDefinition,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	return renderConfigGroupWithParent(
		title,
		id,
		expanded,
		fields,
		group,
		comments,
		displayNames,
		"generalConfigAccordion",
	)
}

// renderConfigGroupWithParent creates a collapsible group with specified parent accordion.
func renderConfigGroupWithParent(
	title, id string,
	expanded bool,
	fields []FormFieldDefinition,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	parentAccordion string,
) gomponents.Node {
	collapseClass := "accordion-collapse collapse"
	if expanded {
		collapseClass += " show"
	}

	return html.Div(
		html.Class("accordion-item"),
		html.Style("border: 1px solid #dee2e6; border-radius: 8px; margin-bottom: 0.5rem;"),
		html.H2(
			html.Class("accordion-header"),
			html.ID("heading"+id),
			html.Button(
				html.Class("accordion-button"),
				gomponents.If(!expanded, gomponents.Attr("class", "accordion-button collapsed")),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: none; padding: 0.75rem 1rem; font-weight: 600;",
				),
				html.Type("button"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#collapse"+id),
				gomponents.Attr("aria-expanded", fmt.Sprintf("%t", expanded)),
				gomponents.Attr("aria-controls", "collapse"+id),
				html.I(html.Class("fas fa-cog me-2 text-primary")),
				gomponents.Text(title),
				html.Span(
					html.Class("badge bg-primary ms-2"),
					gomponents.Text(fmt.Sprintf("%d", len(fields))),
				),
			),
		),
		html.Div(
			html.ID("collapse"+id),
			html.Class(collapseClass),
			gomponents.Attr("aria-labelledby", "heading"+id),
			gomponents.Attr("data-bs-parent", "#"+parentAccordion),
			html.Div(
				html.Class("accordion-body p-3"),
				html.Style("background-color: #fdfdfe;"),
				// Use compact grid layout for fields
				renderCompactFormFields(group, comments, displayNames, fields),
			),
		),
	)
}

// renderCompactFormFields creates a more compact grid layout for configuration fields.
func renderCompactFormFields(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	fields []FormFieldDefinition,
) gomponents.Node {
	var formRows []gomponents.Node

	// Group fields into rows of 2 for better space utilization
	for i := 0; i < len(fields); i += 2 {
		var rowFields []gomponents.Node

		// First field in row
		if i < len(fields) {
			field := fields[i]

			rowFields = append(rowFields, html.Div(
				html.Class("col-md-6 mb-3"),
				renderFormGroup(
					group,
					comments,
					displayNames,
					field.Name,
					field.Type,
					field.Value,
					convertSelectOptionsToMap(field.Options),
				),
			))
		}

		// Second field in row (if exists)
		if i+1 < len(fields) {
			field := fields[i+1]

			rowFields = append(rowFields, html.Div(
				html.Class("col-md-6 mb-3"),
				renderFormGroup(
					group,
					comments,
					displayNames,
					field.Name,
					field.Type,
					field.Value,
					convertSelectOptionsToMap(field.Options),
				),
			))
		}

		// Create row
		formRows = append(formRows, html.Div(
			html.Class("row"),
			gomponents.Group(rowFields),
		))
	}

	return gomponents.Group(formRows)
}

// renderAlert creates a dismissible alert with icons using optimized createAlert.
func renderAlert(message string, typev string) string {
	return renderComponentToString(createAlert(message, typev))
}

// jsString returns s as a JavaScript string literal. The result is meant for
// attribute values (onclick etc.), which gomponents HTML-escapes on output.
func jsString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// selectOptionsWithCurrent returns opts plus the current value when it is not
// one of them, so a saved value outside the hard-coded list is shown and
// posted back unchanged instead of the browser submitting the first option.
// An empty current value is not added; call sites where empty is a legal value
// list "" among their options instead.
func selectOptionsWithCurrent(opts []string, value any) []string {
	current, _ := value.(string)
	if current == "" || slices.Contains(opts, current) {
		return slices.Clone(opts)
	}

	return append(slices.Clone(opts), current)
}

// renderLanguageSelect is a dropdown of the ISO 639-1 languages ("German
// (de)", sorted by name) saving the two-letter code. A value that isn't a
// code (an older "de-DE") stays selectable, so saving doesn't drop it.
func renderLanguageSelect(fieldName string, value any) gomponents.Node {
	current, _ := value.(string)

	codes := make([]string, 0, len(config.LanguageNames)+1)
	for code := range config.LanguageNames {
		codes = append(codes, code)
	}

	slices.SortFunc(codes, func(a, b string) int { return strings.Compare(config.LanguageNames[a], config.LanguageNames[b]) })

	opts := []gomponents.Node{html.Option(html.Value(""), gomponents.If(current == "", html.Selected()), gomponents.Text("— not set —"))}

	if current != "" && config.LanguageNames[current] == "" {
		opts = append(opts, html.Option(html.Value(current), html.Selected(), gomponents.Text(current+" (not an ISO 639-1 code)")))
	}

	for _, code := range codes {
		opts = append(opts, html.Option(html.Value(code), gomponents.If(code == current, html.Selected()),
			gomponents.Text(config.LanguageNames[code]+" ("+code+")")))
	}

	return html.Select(html.Class("form-select"), html.ID(fieldName), html.Name(fieldName), gomponents.Group(opts))
}

// arrayPresentMarker renders the hidden marker that tells the parser an array
// widget was on the form even when all of its entries were removed.
func arrayPresentMarker(fieldName string) gomponents.Node {
	return html.Input(
		html.Type("hidden"),
		html.Name(fieldName+arrayPresentSuffix),
		html.Value("1"),
	)
}

func arrayRemoveRowButton() gomponents.Node {
	return html.Button(
		html.Class(ClassBtnDanger),
		html.Type("button"),
		gomponents.Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
		html.I(html.Class("fa fa-trash")),
	)
}

// arrayAddRowButton renders the "Add Item" button of an array widget. The new
// row's markup travels in the button's own data attribute, so every widget
// instance adds inputs with its own field name (no shared global function).
// rowInputs must already be HTML-escaped where it embeds values.
func arrayAddRowButton(rowInputs string) gomponents.Node {
	return html.Button(
		html.Class(ClassBtnPrimary),
		html.Type("button"),
		html.Data(
			"row-html",
			rowInputs+`<button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()"><i class="fa fa-trash"></i></button>`,
		),
		gomponents.Attr(
			"onclick",
			"var r=document.createElement('div');r.className='d-flex mb-2';r.innerHTML=this.dataset.rowHtml;this.parentElement.insertBefore(r,this);",
		),
		gomponents.Text("Add Item"),
	)
}

// renderFormGroup renders a form group with label and input.
func renderFormGroup(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	name, inputType string,
	value any,
	options map[string][]string,
) gomponents.Node {
	var (
		input     gomponents.Node
		iconClass string
	)

	// Add icons based on input type

	switch inputType {
	case "text":
		iconClass = "fa-solid fa-font"
	case "password":
		iconClass = "fa-solid fa-key"
	case "number":
		iconClass = "fa-solid fa-hashtag"
	case "select", "selectarray":
		iconClass = "fa-solid fa-list"
	case "language":
		iconClass = "fa-solid fa-language"
	case "checkbox":
		iconClass = "fa-solid fa-toggle-on"
	case "textarea":
		iconClass = "fa-solid fa-align-left"
	case "array", "arrayselect", "arrayselectarray", "arrayint":
		iconClass = "fa-solid fa-layer-group"
	case "floatmap":
		iconClass = "fa-solid fa-sliders"
	default:
		iconClass = "fa-solid fa-cog"
	}

	switch inputType {
	case "removebutton":
		return createRemoveButton(false)
	case "custom":
		// Value carries a pre-built gomponents.Node through the generic
		// FormFieldDefinition pipeline (e.g. renderPathCheckControl) instead
		// of a scalar - the caller's col-md-6 wrapper still applies.
		if node, ok := value.(gomponents.Node); ok {
			return node
		}

		return gomponents.Text("")
	case "selectarray":
		values, _ := value.([]string)
		input = html.Div(
			renderChoiceChips(group+"_"+name, choiceOptionsFrom(options, true), values),
			arrayPresentMarker(group+"_"+name),
		)

	case "select":
		var optionElements []gomponents.Node

		// An unset value gets its own empty option. Without it the browser
		// preselects the first option, and saving the form writes that value.
		if current, _ := value.(string); current == "" {
			optionElements = append(optionElements, html.Option(html.Value(""), html.Selected(), gomponents.Text("— not set —")))
		}

		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(selectOptionsWithCurrent(opts, value))
			opts2.Sort()

			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}
		}

		input = html.Select(
			html.Class("form-select"),
			html.ID(group+"_"+name),
			html.Name(group+"_"+name),
			gomponents.Group(optionElements),
		)

	case "language":
		input = renderLanguageSelect(group+"_"+name, value)

	case "selectwithpreview":
		// Select with preview link and modal - used for template reference fields
		var optionElements []gomponents.Node
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(selectOptionsWithCurrent(opts, value))
			opts2.Sort()

			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}
		}

		// Get template config info for the link and preview
		templateInfo := getTemplateConfigInfo(name)
		selectID := group + "_" + name
		previewBtnID := selectID + "_preview_btn"

		// Create the select with preview buttons
		input = html.Div(
			html.Class("input-group"),
			html.Select(
				html.Class("form-select"),
				html.ID(selectID),
				html.Name(selectID),
				gomponents.Group(optionElements),
			),
			// Preview button - opens modal with readonly preview
			html.Button(
				html.Type("button"),
				html.Class("btn btn-outline-info"),
				html.ID(previewBtnID),
				html.Title("Preview configuration"),
				gomponents.Attr(
					"onclick",
					fmt.Sprintf(
						"showConfigPreview(%s, document.getElementById(%s).value, %s)",
						jsString(templateInfo.Type),
						jsString(selectID),
						jsString(name),
					),
				),
				html.I(html.Class("fas fa-eye")),
			),
			// Link button - opens config page in new tab with hash anchor to auto-expand
			html.Button(
				html.Type("button"),
				html.Class("btn btn-outline-secondary"),
				html.Title("Open configuration page"),
				gomponents.Attr(
					"onclick",
					fmt.Sprintf(
						"openConfigPage(%s, document.getElementById(%s).value)",
						jsString(templateInfo.URL),
						jsString(selectID),
					),
				),
				html.I(html.Class("fas fa-external-link-alt")),
			),
		)

	case "checkbox":
		var addnode gomponents.Node
		switch val := value.(type) {
		case bool:
			if val {
				addnode = html.Checked()
			}
		}

		// Use createFormField for checkbox with custom checkbox-specific styling
		if addnode != nil {
			input = html.Input(
				html.Class("form-check-input-modern"),
				html.Type("checkbox"),
				html.Role("switch"),
				html.ID(group+"_"+name),
				html.Name(group+"_"+name), addnode,
			)
		} else {
			input = html.Input(
				html.Class("form-check-input-modern"),
				html.Type("checkbox"),
				html.Role("switch"),
				html.ID(group+"_"+name),
				html.Name(group+"_"+name),
			)
		}

	case "textarea":
		input = html.Textarea(
			html.Class(ClassFormControl),
			html.ID(group+"_"+name),
			html.Name(group+"_"+name),
			html.Rows("3"),
			gomponents.Text(fmt.Sprintf("%v", value)),
		)

	case "text":
		// Options of a text field are suggestions; any text stays allowed.
		if suggestions := options["values"]; len(suggestions) > 0 {
			listID := group + "_" + name + "_suggestions"
			opts := make([]gomponents.Node, 0, len(suggestions))
			for _, v := range suggestions {
				opts = append(opts, html.Option(html.Value(v)))
			}

			input = html.Div(
				createFormField("text", group+"_"+name, value.(string), "", []gomponents.Node{gomponents.Attr("list", listID)}),
				html.DataList(html.ID(listID), gomponents.Group(opts)),
			)
		} else {
			input = createFormField("text", group+"_"+name, value.(string), "", nil)
		}
	case "password":
		input = createFormField("password", group+"_"+name, value.(string), "", nil)
	case "number":
		var setvalue string
		switch val := value.(type) {
		case int:
			setvalue = fmt.Sprintf("%d", val)
		case int64:
			setvalue = fmt.Sprintf("%d", val)
		case int32:
			setvalue = fmt.Sprintf("%d", val)
		case int16:
			setvalue = fmt.Sprintf("%d", val)
		case int8:
			setvalue = fmt.Sprintf("%d", val)
		case uint:
			setvalue = fmt.Sprintf("%d", val)
		case uint64:
			setvalue = fmt.Sprintf("%d", val)
		case uint32:
			setvalue = fmt.Sprintf("%d", val)
		case uint16:
			setvalue = fmt.Sprintf("%d", val)
		case uint8:
			setvalue = fmt.Sprintf("%d", val)
		case float64:
			setvalue = fmt.Sprintf("%f", val)
		case float32:
			setvalue = fmt.Sprintf("%f", val)
		}

		input = createFormField("number", group+"_"+name, setvalue, "", nil)

	case "array":
		fieldName := group + "_" + name
		nodes := []gomponents.Node{
			html.ID(fieldName + "-container"),
			arrayPresentMarker(fieldName),
		}

		toolButton := ""
		if tool := helperToolFor(displayNames[name]); tool != "" {
			toolButton = helperToolRowButton(tool)
		}

		for _, v := range value.([]string) {
			nodes = append(nodes, html.Div(
				html.Class(ClassDFlex),
				html.Input(
					html.Class(ClassFormControl+" me-2"),
					html.Type("text"),
					html.Name(fieldName),
					html.Value(v),
				),
				gomponents.Raw(toolButton),
				arrayRemoveRowButton(),
			))
		}

		input = html.Div(append(nodes, arrayAddRowButton(
			`<input class="form-control me-2" type="text" name="`+html2.EscapeString(fieldName)+`">`+toolButton,
		))...)

	case "arrayselectarray":
		fieldName := group + "_" + name

		var opts2 sort.StringSlice
		if opts, ok := options["options"]; ok {
			opts2 = sort.StringSlice(slices.Clone(opts))
			opts2.Sort()
		}

		var optionString strings.Builder
		for _, opt := range opts2 {
			optionString.WriteString("<option value=\"")
			optionString.WriteString(html2.EscapeString(opt))
			optionString.WriteString("\">")
			optionString.WriteString(html2.EscapeString(opt))
			optionString.WriteString("</option>")
		}

		nodes := []gomponents.Node{
			html.ID(fieldName + "-container"),
			arrayPresentMarker(fieldName),
		}

		for _, v := range value.([]string) {
			optionElements := make([]gomponents.Node, 0, len(opts2)+1)
			for _, opt := range selectOptionsWithCurrent(opts2, v) {
				if opt == v {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))
				}
			}

			nodes = append(nodes, html.Div(
				html.Class(ClassDFlex),
				html.Select(
					html.Class(ClassFormSelect),
					html.Name(fieldName),
					gomponents.Group(optionElements),
				),
				arrayRemoveRowButton(),
			))
		}

		input = html.Div(append(nodes, arrayAddRowButton(
			`<select class="form-select me-2" name="`+html2.EscapeString(fieldName)+`">`+
				optionString.String()+`</select>`,
		))...)

	case "arrayselect":
		var (
			optionElements []gomponents.Node
			optionString   string
		)

		if opts, ok := options["options"]; ok {
			var values []string
			switch val := value.(type) {
			case []string:
				values = val
			}

			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for _, opt := range opts2 {
				var selected bool
				switch val := value.(type) {
				case []string:
					selected = slices.Contains(values, opt)
				case string:
					selected = opt == val
				}

				if selected {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
						html.Selected(),
					))

					optionString += "<option value=\"" + opt + "\" selected=\"\">" + opt + "</option>"
				} else {
					optionElements = append(optionElements, html.Option(
						html.Value(opt),
						gomponents.Text(opt),
					))

					optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
				}
			}
		}

		input = html.Div(
			html.ID(group+"_"+name+"-container"),
			html.Div(
				html.Class(ClassDFlex),
				html.Select(
					html.Class(ClassFormSelect),
					html.Name(group+"_"+name),
					gomponents.Group(optionElements),
				),
			),
		)

	case "arrayint":
		fieldName := group + "_" + name
		nodes := []gomponents.Node{
			html.ID(fieldName + "-container"),
			arrayPresentMarker(fieldName),
		}

		for _, v := range value.([]int) {
			nodes = append(nodes, html.Div(
				html.Class(ClassDFlex),
				html.Input(
					html.Class(ClassFormControl+" me-2"),
					html.Type("number"),
					html.Name(fieldName),
					html.Value(strconv.Itoa(v)),
				),
				arrayRemoveRowButton(),
			))
		}

		input = html.Div(append(nodes, arrayAddRowButton(
			`<input class="form-control me-2" type="number" name="`+html2.EscapeString(fieldName)+`">`,
		))...)

	case "floatmap":
		fieldName := group + "_" + name
		m, _ := value.(map[string]float64)

		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		nodes := []gomponents.Node{
			html.ID(fieldName + "-container"),
			arrayPresentMarker(fieldName + "_key"),
		}

		for _, k := range keys {
			nodes = append(nodes, html.Div(
				html.Class(ClassDFlex+" mb-2"),
				html.Input(
					html.Class(ClassFormControl+" me-2"),
					html.Type("text"),
					html.Placeholder("source"),
					html.Name(fieldName+"_key"),
					html.Value(k),
				),
				html.Input(
					html.Class(ClassFormControl+" me-2"),
					html.Type("number"),
					gomponents.Attr("step", "any"),
					html.Placeholder("penalty"),
					html.Name(fieldName+"_value"),
					html.Value(strconv.FormatFloat(m[k], 'f', -1, 64)),
				),
				arrayRemoveRowButton(),
			))
		}

		input = html.Div(append(nodes, arrayAddRowButton(
			`<input class="form-control me-2" type="text" placeholder="source" name="`+
				html2.EscapeString(fieldName+"_key")+
				`"><input class="form-control me-2" type="number" step="any" placeholder="penalty" name="`+
				html2.EscapeString(fieldName+"_value")+`">`,
		))...)

	default:
		input = createFormField(inputType, group+"_"+name, "", "", nil)
	}

	var commentNode gomponents.Node
	if comment := comments[name]; comment != "" {
		commentNode = renderFieldHelp(group+"-"+name, comment)
	}

	// Get the display name from the displayNames map, fallback to fieldNameToUserFriendly
	displayName := displayNames[name]
	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	if tool := helperToolFor(displayName); tool != "" && inputType == "text" {
		input = withHelperTool(tool, group+"_"+name, input)
	}

	if inputType == "checkbox" {
		return html.Div(
			html.Class("form-group-enhanced mb-4"),
			html.Div(
				html.Class("form-check-wrapper p-3 border rounded-3 bg-light"),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6 !important; transition: all 0.2s ease;",
				),
				html.Div(
					html.Class("form-check form-switch"),
					html.I(html.Class(iconClass+" text-primary me-2")),
					input,
					createFormLabel(group+"_"+name, displayName, true),
				),
				commentNode,
			),
		)
	}

	return html.Div(
		html.Class("form-group mb-2"),
		html.Div(
			html.Class("form-field-compact p-2 border rounded"),
			html.Style(
				"background: #ffffff; border: 1px solid #e3e6ea !important; transition: border-color 0.15s ease-in-out;",
			),
			html.Div(
				html.Class("d-flex align-items-center mb-1"),
				html.I(
					html.Class(iconClass+" text-primary me-2"),
					html.Style("font-size: 0.85em;"),
				),
				createFormLabel(group+"_"+name, displayName, false),
			),
			input,
			commentNode,
		),
	)
}

// renderFormFields renders multiple form fields using FormFieldDefinition array.
func renderFormFields(
	group string,
	comments map[string]string,
	displayNames map[string]string,
	fields []FormFieldDefinition,
) []gomponents.Node {
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		formGroups = append(
			formGroups,
			renderFormGroup(
				group,
				comments,
				displayNames,
				field.Name,
				field.Type,
				field.Value,
				convertSelectOptionsToMap(field.Options),
			),
		)
	}

	return formGroups
}

// renderComponentToString renders a gomponents Node to a string using pooled string builder.
func renderComponentToString(node gomponents.Node) string {
	sb := getStringBuilder()
	defer putStringBuilder(sb)

	node.Render(sb)

	return sb.String()
}
