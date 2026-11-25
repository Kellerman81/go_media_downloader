package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	htmx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The field types and input builders of the generated edit forms.
const (
	// Common Form Field Types.
	fieldTypeText        = "text"
	fieldTypeNumber      = "number"
	fieldTypeCheckbox    = "checkbox"
	fieldTypeSelect      = "select"
	fieldTypeSelectArray = "selectarray"

	// Common HTML Attributes and Elements.
	attrTypeButton = "button"
	attrTitle      = "title"
	keyOptions     = "options"

	// Separators and delimiters.
	separatorUnderscore = "_"

	// Array index position in field name parsing.
	fieldNameIndexPosition = 3

	// Number parsing base.
	decimalBase = 10
	trueValue   = "true"

	// AttrHxTarget is the HTMX attribute for specifying the target element.
	AttrHxTarget         = "hx-target"
	AttrHxPost           = "hx-post"
	AttrHxHeaders        = "hx-headers"
	AttrHxGet            = "hx-get"
	AttrHxTrigger        = "hx-trigger"
	AttrHxVals           = "hx-vals"
	AttrHxIndicator      = "hx-indicator"
	AttrHxDelete         = "hx-delete"
	AttrHxConfirm        = "hx-confirm"
	AttrHxOnAfterRequest = "hx-on--after-request"
	AttrHxPatch          = "hx-patch"
	AttrHxPut            = "hx-put"

	// IconTimes is the FontAwesome CSS class for the close/times icon.
	IconTimes = "fas fa-times"

	// IconCog is the FontAwesome CSS class for the settings/cog icon.
	IconCog = "fas fa-cog"

	// IconList is the FontAwesome CSS class for the list icon.
	IconList         = "fas fa-list"
	IconCalendarDays = "fas fa-calendar-days"

	// IconFont is the FontAwesome CSS class for the font icon.
	IconFont       = "fas fa-font"
	IconHashtag    = "fas fa-hashtag"
	IconToggleOn   = "fas fa-toggle-on"
	IconAlignLeft  = "fas fa-align-left"
	IconLayerGroup = "fas fa-layer-group"
)

// textFieldType is "textarea" for long text columns (plots, descriptions,
// notes, error messages) and for any value too long or multi-line for one
// input line, else "text".
func textFieldType(col, value string) string {
	switch {
	case col == "overview", col == "description", col == "notes",
		strings.HasSuffix(col, "_message"), strings.HasSuffix(col, "_errors"),
		len(value) > 100, strings.Contains(value, "\n"):
		return "textarea"
	}

	return fieldTypeText
}

// Helper function to generate form fields based on data type and column name.
func generateFormField(col, fieldName, displayName string, fieldData any) gomponents.Node {
	// Handle special config dropdown fields
	if col == "quality_profile" || col == "listname" || (col == "quality_type") {
		return generateConfigSelectField(col, fieldName, displayName, fieldData)
	}

	// Handle foreign key fields with AJAX
	if strings.HasSuffix(col, "_id") && col != "id" {
		refTable := getReferenceTable(col)
		if refTable != "" {
			return generateAjaxSelectField(col, fieldName, displayName, fieldData, refTable)
		}
	}

	// Handle different data types
	switch val := fieldData.(type) {
	case bool:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "checkbox", val, nil)

	case string:
		return RenderFormGroup("",
			"",
			displayName, fieldName, textFieldType(col, val), val, nil)

	case int, int64, uint, uint8, uint16, uint32, uint64:
		// Check if this should be a checkbox (boolean field stored as int)
		if isCheckboxFieldRefactored(col) {
			checked := parseCheckboxValue(fmt.Sprintf("%v", fieldData))
			return RenderFormGroup("",
				"",
				displayName, fieldName, "checkbox", checked, nil)
		}

		return RenderFormGroup("",
			"",
			displayName, fieldName, "number", fmt.Sprintf("%v", fieldData), nil)

	case float64, float32:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "number", fmt.Sprintf("%v", fieldData), nil)

	case []byte:
		// modernc/sqlite can return BLOB-affinity columns as raw bytes.
		return RenderFormGroup("",
			"",
			displayName, fieldName, textFieldType(col, string(val)), string(val), nil)

	case nil:
		return RenderFormGroup("",
			"",
			displayName, fieldName, textFieldType(col, ""), "", nil)

	case time.Time:
		valformat := val.Format("2006-01-02")

		return RenderFormGroup(
			"",
			"",
			displayName,
			fieldName,
			"date",
			valformat,
			map[string][]string{
				"class": {"form-control datepicker"},
			},
		)

	case sql.NullTime:
		valformat := ""
		if val.Valid {
			valformat = val.Time.Format("2006-01-02")
		}

		return RenderFormGroup(
			"",
			"",
			displayName,
			fieldName,
			"date",
			valformat,
			map[string][]string{
				"class": {"form-control datepicker"},
			},
		)

	default:
		return RenderFormGroup("",
			"",
			displayName, fieldName, "text", fieldData, nil)
	}
}

// Generate config select field using SelectField component.
func generateConfigSelectField(col, fieldName, displayName string, fieldData any) gomponents.Node {
	currentValue := ""
	if fieldData != nil {
		currentValue = fmt.Sprintf("%v", fieldData)
	}

	var options []string

	options = append(options, "")

	// Add config options based on field type
	switch col {
	case "quality_profile":
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			options = append(options, qc.Name)
		}

	case "listname":
		options = append(options, config.GetSettingsMediaListAll()...)
	case "quality_type":
		qualityTypes := map[string]string{
			"1": "Resolution",
			"2": "Quality",
			"3": "Codec",
			"4": "Audio",
		}
		for value := range qualityTypes {
			options = append(options, value)
		}
	}

	return RenderFormGroup("",
		"",
		displayName, fieldName, "select", currentValue, map[string][]string{
			"options": options,
		})
}

// AjaxSelectOptions for AJAX-powered select fields with preselection support.
type AjaxSelectOptions struct {
	ID              string         // HTML ID attribute
	Name            string         // HTML name attribute for form submission
	Label           string         // Label text
	Icon            string         // Icon class for label (e.g., "fas fa-film")
	AjaxURL         string         // URL for AJAX data loading
	SelectedValue   string         // Current selected value ID
	SelectedText    string         // Current selected value display text (if known)
	Placeholder     string         // Placeholder text
	HelpText        string         // Help text below the field
	WrapperClass    string         // CSS class for wrapper div
	StaticOptions   []SelectOption // Static options (e.g., for quality profiles)
	SearchThreshold int            // Minimum characters before search triggers
	AllowClear      bool           // Allow clearing selection
	AllowCustom     bool           // Allow custom values to be typed
	Required        bool           // HTML required attribute
	Disabled        bool           // HTML disabled attribute
}

// Generate AJAX select field using AjaxSelectField component.
func generateAjaxSelectField(
	col, fieldName, displayName string,
	fieldData any,
	refTable string,
) gomponents.Node {
	currentValue := ""
	if fieldData != nil {
		switch v := fieldData.(type) {
		case int, int64, uint, uint64:
			currentValue = fmt.Sprintf("%d", v)
		case float64:
			if v == float64(int64(v)) {
				currentValue = fmt.Sprintf("%.0f", v)
			} else {
				currentValue = fmt.Sprintf("%v", v)
			}

		case string:
			currentValue = v
		default:
			currentValue = fmt.Sprintf("%v", v)
		}
	}

	return AjaxSelectField(AjaxSelectOptions{
		ID:              fieldName,
		Name:            fieldName,
		Label:           displayName,
		AjaxURL:         "/api/admin/dropdown/" + refTable + "/" + col,
		SelectedValue:   currentValue,
		Placeholder:     "Select " + displayName + "...",
		AllowClear:      true,
		SearchThreshold: 0,
	})
}

// AjaxSelectField creates a select field with AJAX loading and preselection support
// Following the same pattern as renderFormGroup for consistency.
func AjaxSelectField(options AjaxSelectOptions) gomponents.Node {
	// Determine icon based on type or use provided icon
	iconClass := IconList
	if options.Icon != "" {
		iconClass = options.Icon
	}

	// Create select attributes
	selectAttrs := []gomponents.Node{
		html.ID(options.ID),
		html.Name(options.Name),
		html.Class("form-select choices-ajax"),
	}

	// Add AJAX URL if provided
	if options.AjaxURL != "" {
		selectAttrs = append(selectAttrs, html.Data("ajax-url", options.AjaxURL))
	}

	// Add optional attributes
	if options.Placeholder != "" {
		selectAttrs = append(selectAttrs, html.Data("placeholder", options.Placeholder))
	}

	if options.AllowClear {
		selectAttrs = append(selectAttrs, html.Data("allow-clear", trueValue))
	}

	if options.AllowCustom {
		selectAttrs = append(selectAttrs, html.Data("allow-custom", trueValue))
	}

	if options.SearchThreshold > 0 {
		selectAttrs = append(
			selectAttrs,
			html.Data("search-threshold", fmt.Sprintf("%d", options.SearchThreshold)),
		)
	}

	if options.SelectedValue != "" {
		selectAttrs = append(selectAttrs, html.Data("selected-value", options.SelectedValue))
	}

	if options.Required {
		selectAttrs = append(selectAttrs, html.Required())
	}

	if options.Disabled {
		selectAttrs = append(selectAttrs, html.Disabled())
	}

	// Create select options
	var selectOptions []gomponents.Node

	// Add default empty option
	emptyText := "-- Select --"
	if options.Placeholder != "" {
		emptyText = options.Placeholder
	}

	selectOptions = append(selectOptions, html.Option(
		html.Value(""),
		gomponents.Text(emptyText),
	))

	// Add preselected option if there's a current value
	if options.SelectedValue != "" && options.SelectedText != "" {
		selectOptions = append(selectOptions, html.Option(
			html.Value(options.SelectedValue),
			html.Selected(),
			gomponents.Text(options.SelectedText),
		))
	} else if options.SelectedValue != "" {
		// If we have a value but no text, show "Loading..." until AJAX loads
		selectOptions = append(selectOptions, html.Option(
			html.Value(options.SelectedValue),
			html.Selected(),
			gomponents.Text("Loading..."),
		))
	}

	// Add any static options
	for _, opt := range options.StaticOptions {
		var valueStr string
		if opt.Value != nil {
			valueStr = fmt.Sprintf("%v", opt.Value)
		}

		optAttrs := []gomponents.Node{
			html.Value(valueStr),
			gomponents.Text(opt.Label),
		}
		// Note: Selected field is commented out in shared.SelectOption
		selectOptions = append(selectOptions, html.Option(optAttrs...))
	}

	selectAttrs = append(selectAttrs, selectOptions...)

	input := html.Select(selectAttrs...)

	var commentNode gomponents.Node
	if options.HelpText != "" {
		commentNode = renderFieldHelp(options.ID, options.HelpText)
	}

	// Get display name, fallback to fieldNameToUserFriendly if not provided
	displayName := options.Label
	if displayName == "" {
		displayName = fieldNameToUserFriendly(options.Name)
	}

	// Return consistent form group structure like renderFormGroup
	// Include JavaScript initialization for AJAX functionality
	return html.Div(
		createFormFieldWrapper(options.ID, iconClass, displayName, input, commentNode),
		// Add CSS to fix remove button icon styling
		html.StyleEl(gomponents.Raw(`
			.choices__button {
				width: 20px !important;
				height: 20px !important;
				padding: 2px !important;
				display: flex !important;
				align-items: center !important;
				justify-content: center !important;
				border: 1px solid #ccc !important;
				border-radius: 2px !important;
				background-size: 12px 12px !important;
				background-position: center !important;
				background-repeat: no-repeat !important;
			}
			.choices__button:hover {
				background-color: #f8f9fa !important;
				border-color: #dc3545 !important;
			}
			.choices__button:focus {
				outline: 2px solid #007bff !important;
				outline-offset: 1px !important;
			}
		`)),
		html.Script(gomponents.Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				// Initialize Choices.js for enhanced selects
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
			});
		`)),
	)
}

// createFormFieldWrapper creates a compact form field wrapper with icon and label.
func createFormFieldWrapper(
	id, iconClass, displayName string,
	inputElement, commentNode gomponents.Node,
) gomponents.Node {
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
				createFormLabel(id, displayName, false),
			),
			inputElement,
			commentNode,
		),
	)
}

// Helper function to determine if an int field should be rendered as checkbox.
func isCheckboxFieldRefactored(col string) bool {
	checkboxFields := []string{
		"missing", "blacklisted", "dont_search", "dont_upgrade", "use_regex",
		"proper", "extended", "repack", "ignore_runtime", "adult",
		"search_specials", "quality_reached",
	}

	return slices.Contains(checkboxFields, col)
}

// RenderFormGroup creates form group with proper input type handling.
func RenderFormGroup(
	group, comment, displayName, name, inputType string,
	value any,
	options map[string][]string,
) gomponents.Node {
	classelement := ClassFormControl
	if inputType == fieldTypeSelect || inputType == fieldTypeSelectArray {
		classelement = "form-select"
		if inputType == fieldTypeSelectArray {
			classelement += " selectpicker"
		}
	}

	classelement, addDetails := parseFormOptions(options, classelement)

	id := name
	if group != "" {
		id = group + "_" + name
	}

	iconClass := getInputIcon(inputType)
	input := renderInput(inputType, group, id, classelement, value, options, addDetails)
	commentNode := createCommentNode(comment, group, name)

	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	return createFormLayout(inputType, id, displayName, iconClass, input, commentNode)
}

// parseCheckboxValue safely parses checkbox values, handling corrupted "on" strings.
func parseCheckboxValue(value string) bool {
	switch value {
	case "1", "true", "on", "yes":
		return true
	case "0", "false", "off", "no", "":
		return false
	default:
		// For any other value, try standard ParseBool
		result, _ := strconv.ParseBool(value)
		return result
	}
}

// parseFormOptions processes the options map and returns additional HTML attributes.
func parseFormOptions(options map[string][]string, baseClass string) (string, []gomponents.Node) {
	classelement := baseClass

	var addDetails []gomponents.Node

	if len(options) >= 1 {
		for i, val := range options {
			switch i {
			case "class":
				classelement = val[0]
			case "style":
				addDetails = append(addDetails, html.Style(val[0]))
			case "rows":
				addDetails = append(addDetails, html.Rows(val[0]))
			case "required":
				if strings.EqualFold(val[0], "True") {
					addDetails = append(addDetails, html.Required())
				}

			case "readonly":
				if strings.EqualFold(val[0], "True") {
					addDetails = append(addDetails, html.ReadOnly())
				}

			case "min":
				addDetails = append(addDetails, html.Min(val[0]))
			case "max":
				addDetails = append(addDetails, html.Max(val[0]))
			case "pattern":
				addDetails = append(addDetails, html.Pattern(val[0]))
			case "data-toggle":
				addDetails = append(addDetails, html.Data("toggle", val[0]))
			case "title":
				addDetails = append(addDetails, html.Title(val[0]))
			case AttrHxGet:
				addDetails = append(addDetails, htmx.Get(val[0]))
			case AttrHxTarget:
				addDetails = append(addDetails, htmx.Target(val[0]))
			case AttrHxTrigger:
				addDetails = append(addDetails, htmx.Trigger(val[0]))
			case AttrHxVals:
				addDetails = append(addDetails, htmx.Vals(val[0]))
			case AttrHxHeaders:
				addDetails = append(addDetails, htmx.Headers(val[0]))
			case AttrHxIndicator:
				addDetails = append(addDetails, htmx.Indicator(val[0]))
			case AttrHxDelete:
				addDetails = append(addDetails, htmx.Delete(val[0]))
			case AttrHxConfirm:
				addDetails = append(addDetails, htmx.Confirm(val[0]))
			case AttrHxOnAfterRequest:
				addDetails = append(addDetails, htmx.On("after-request", val[0]))
			case AttrHxPatch:
				addDetails = append(addDetails, htmx.Patch(val[0]))
			case AttrHxPost:
				addDetails = append(addDetails, htmx.Post(val[0]))
			case AttrHxPut:
				addDetails = append(addDetails, htmx.Put(val[0]))
			case "onchange":
				addDetails = append(addDetails, htmx.On("change", val[0]))
			case "placeholder":
				addDetails = append(addDetails, html.Placeholder(val[0]))
			}
		}
	}

	return classelement, addDetails
}

// getInputIcon returns the appropriate FontAwesome icon for the input type.
func getInputIcon(inputType string) string {
	switch inputType {
	case fieldTypeText:
		return IconFont
	case "date":
		return IconCalendarDays
	case "number":
		return IconHashtag
	case "select", "selectarray":
		return IconList
	case "checkbox":
		return IconToggleOn
	case "textarea":
		return IconAlignLeft
	case "array", "arrayselect", "arrayselectarray", "arrayint":
		return IconLayerGroup
	default:
		return IconCog
	}
}

// renderInput creates the appropriate input element based on type.
func renderInput(
	inputType, group, id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	switch inputType {
	case "removebutton":
		return createRemoveButton(false)
	case "selectarray":
		return createSelectArrayInput(id, classelement, value, options)
	case "select":
		return createSelectInput(id, classelement, value, options, addDetails)
	case fieldTypeCheckbox:
		return createCheckboxInput(id, value)
	case "textarea":
		return createTextareaInput(id, classelement, value, options, addDetails)
	case "date":
		return createDateInput(id, classelement, value, addDetails)
	case fieldTypeText:
		return createTextInput(id, classelement, value, addDetails)
	case "number":
		return createNumberInput(id, value, addDetails)
	case "array":
		return createArrayInput(group, id, classelement, value, options, addDetails)
	case "arrayselectarray":
		return createArraySelectArrayInput(id, value, options, addDetails)
	case "arrayselect":
		return createArraySelectInput(id, value, options, addDetails)
	case "arrayint":
		return createArrayIntInput(group, id, classelement, value, addDetails)
	default:
		return createFormField(inputType, id, "", "", addDetails)
	}
}

// createSelectArrayInput creates a multi-choice field as toggle chips.
func createSelectArrayInput(
	id, _ string,
	value any,
	options map[string][]string,
) gomponents.Node {
	values, _ := value.([]string)
	return renderChoiceChips(id, choiceOptionsFrom(options, true), values)
}

// createSelectInput creates a single select dropdown.
func createSelectInput(
	id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		labels, hasLabels := options["labels"]

		if hasLabels && len(labels) == len(opts) {
			for i, opt := range opts {
				selected := opt == fmt.Sprintf("%v", value)
				displayText := labels[i]

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		} else {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()

			for i, opt := range opts2 {
				selected := opt == fmt.Sprintf("%v", value)

				displayText := opt
				if hasLabels && i < len(labels) {
					displayText = labels[i]
				}

				optionElements = append(optionElements, createOption(opt, displayText, selected))
			}
		}
	}

	selectattr := append([]gomponents.Node{
		html.Class(classelement),
		html.ID(id),
		html.Name(id),
	}, addDetails...)

	selectattr = append(selectattr, gomponents.Group(optionElements))

	return html.Select(selectattr...)
}

// createCheckboxInput creates a checkbox input.
func createCheckboxInput(id string, value any) gomponents.Node {
	baseAttrs := []gomponents.Node{
		html.Class("form-check-input-modern"),
		html.Type(fieldTypeCheckbox),
		html.Role("switch"),
		html.ID(id),
		html.Name(id),
	}

	if val, ok := value.(bool); ok && val {
		baseAttrs = append(baseAttrs, html.Checked())
	}

	return html.Input(baseAttrs...)
}

// createTextareaInput creates a textarea input.
func createTextareaInput(
	id, classelement string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	baseAttrs := []gomponents.Node{
		html.Class(classelement),
		html.ID(id),
		html.Name(id),
	}

	if !hasRowsOption(options) {
		baseAttrs = append(baseAttrs, html.Rows("3"))
	}

	baseAttrs = append(baseAttrs, addDetails...)

	// A textarea's content is its child text; a value attribute is ignored.
	return html.Textarea(append(baseAttrs, gomponents.Text(fmt.Sprintf("%v", value)))...)
}

// hasRowsOption checks if rows option is specified.
func hasRowsOption(options map[string][]string) bool {
	for _, option := range options {
		if len(option) == 0 {
			continue
		}

		for key := range options {
			if key == "rows" {
				return true
			}
		}
	}

	return false
}

// createDateInput creates a date input.
func createDateInput(
	id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Input(
		append([]gomponents.Node{
			html.Class(classelement),
			html.ID(id),
			html.Name(id),
			html.Type("date"),
			html.Value(fmt.Sprintf("%v", value)),
		}, addDetails...)...,
	)
}

// createTextInput creates a text input.
func createTextInput(
	id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Input(
		append([]gomponents.Node{
			html.Class(classelement),
			html.Type("text"),
			html.ID(id),
			html.Name(id),
			html.Value(fmt.Sprintf("%v", value)),
		}, addDetails...)...,
	)
}

// createNumberInput creates a number input with proper type conversion.
func createNumberInput(id string, value any, addDetails []gomponents.Node) gomponents.Node {
	var setvalue string
	switch val := value.(type) {
	case int, int64, int32, int16, int8:
		setvalue = fmt.Sprintf("%d", val)
	case uint, uint64, uint32, uint16, uint8:
		setvalue = fmt.Sprintf("%d", val)
	case float64, float32:
		setvalue = fmt.Sprintf("%f", val)
	case string:
		setvalue = val
	}

	return createFormField(fieldTypeNumber, id, setvalue, "", addDetails)
}

// createArrayInput creates array text inputs with add/remove functionality.
func createArrayInput(
	group, id, classelement string,
	value any,
	_ map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	values, ok := value.([]string)
	if !ok {
		values = nil
	}

	nodes := make([]gomponents.Node, 0, len(values)+1)

	for _, v := range values {
		nodes = append(nodes, createArrayRow(id, classelement, fieldTypeText, v, addDetails))
	}

	nodes = append(nodes, createArrayAddButton(group, id, "addArrayItem", fieldTypeText))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArrayIntInput creates array number inputs.
func createArrayIntInput(
	group, id, classelement string,
	value any,
	addDetails []gomponents.Node,
) gomponents.Node {
	values, ok := value.([]int)
	if !ok {
		values = nil
	}

	nodes := make([]gomponents.Node, 0, len(values)+1)

	for _, v := range values {
		nodes = append(
			nodes,
			createArrayRow(id, classelement, fieldTypeNumber, strconv.Itoa(v), addDetails),
		)
	}

	nodes = append(nodes, createArrayAddButton(group, id, "addArrayIntItem", fieldTypeNumber))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArrayRow creates a single row in an array input with Bootstrap input group.
func createArrayRow(
	id, classelement, inputType, value string,
	addDetails []gomponents.Node,
) gomponents.Node {
	return html.Div(
		html.Class("input-group mb-2"),
		html.Input(
			append([]gomponents.Node{
				html.Class(classelement),
				html.Type(inputType),
				html.Name(id),
				html.Value(value),
			}, addDetails...)...,
		),
		html.Button(
			html.Class("btn btn-outline-danger"),
			html.Type("button"),
			gomponents.Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			gomponents.Attr("title", "Remove item"),
			html.I(html.Class(IconTimes)),
		),
	)
}

// createArrayAddButton creates add button for array inputs.
func createArrayAddButton(group, id, funcName, inputType string) gomponents.Node {
	parts := strings.Split(id, "_")
	name := parts[len(parts)-1]
	// group := ""
	// if len(parts) > 1 {
	// 	group = parts[0]
	// }

	return gomponents.Group([]gomponents.Node{
		html.Button(
			html.Class(ClassBtnPrimary),
			html.Type("button"),
			gomponents.Attr(
				"onclick",
				fmt.Sprintf("%s%s('%s', '%s')", funcName, name, group, name),
			),
			gomponents.Text("Add Item"),
		),
		html.Script(gomponents.Rawf(`
			function %s%s(group, name) {
				const container = document.getElementById(group + '_' + name + '-container');
				const newRow = document.createElement('div');
				newRow.className = 'input-group mb-2';
				newRow.innerHTML = '<input class="form-control" type="%s" name="%s"><button class="btn btn-outline-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()" title="Remove item"><i class=IconTimes></i></button>';
				container.insertBefore(newRow, container.lastElementChild);
			}
		`, funcName, name, inputType, id)),
	})
}

// createArraySelectArrayInput creates array of select inputs.
func createArraySelectArrayInput(
	id string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	optionString := buildOptionString(options)

	values, ok := value.([]string)
	if !ok {
		values = nil
	}

	nodes := make([]gomponents.Node, 0, len(values)+1)

	for _, v := range values {
		nodes = append(nodes, createArraySelectRow(id, v, options, addDetails))
	}

	nodes = append(nodes, createArraySelectAddButton(id, "addArraySelectArrayItem", optionString))

	return html.Div(
		html.ID(id+"-container"),
		gomponents.Group(nodes),
	)
}

// createArraySelectInput creates single array select input.
func createArraySelectInput(
	id string,
	value any,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		var values []string
		if val, ok := value.([]string); ok {
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

			optionElements = append(optionElements, createOption(opt, opt, selected))
		}
	}

	return html.Div(
		html.ID(id+"-container"),
		html.Div(
			html.Class(ClassDFlex),
			html.Select(
				append([]gomponents.Node{
					html.Class(ClassFormSelect),
					html.Name(id),
					gomponents.Group(optionElements),
				}, addDetails...)...,
			),
		),
	)
}

// buildOptionString builds HTML option string for JavaScript.
func buildOptionString(options map[string][]string) string {
	var optionString strings.Builder
	if opts, ok := options[keyOptions]; ok {
		opts2 := sort.StringSlice(opts)
		opts2.Sort()

		for _, opt := range opts2 {
			optionString.WriteString("<option value=\"")
			optionString.WriteString(opt)
			optionString.WriteString("\">")
			optionString.WriteString(opt)
			optionString.WriteString("</option>")
		}
	}

	return optionString.String()
}

// createArraySelectRow creates a row with select dropdown.
func createArraySelectRow(
	id, value string,
	options map[string][]string,
	addDetails []gomponents.Node,
) gomponents.Node {
	var optionElements []gomponents.Node
	if opts, ok := options[keyOptions]; ok {
		opts2 := sort.StringSlice(opts)
		opts2.Sort()

		for _, opt := range opts2 {
			selected := opt == value

			optionElements = append(optionElements, createOption(opt, opt, selected))
		}
	}

	return html.Div(
		html.Class("input-group mb-2"),
		html.Select(
			append([]gomponents.Node{
				html.Class(ClassFormSelect),
				html.Name(id),
				gomponents.Group(optionElements),
			}, addDetails...)...,
		),
		html.Button(
			html.Class("btn btn-outline-danger"),
			html.Type("button"),
			gomponents.Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
			gomponents.Attr("title", "Remove item"),
			html.I(html.Class(IconTimes)),
		),
	)
}

// createArraySelectAddButton creates add button for array select inputs.
func createArraySelectAddButton(id, funcName, optionString string) gomponents.Node {
	parts := strings.Split(id, "_")
	name := parts[len(parts)-1]

	group := ""
	if len(parts) > 1 {
		group = parts[0]
	}

	return gomponents.Group([]gomponents.Node{
		html.Button(
			html.Class(ClassBtnPrimary),
			html.Type("button"),
			gomponents.Attr(
				"onclick",
				fmt.Sprintf("%s%s('%s', '%s')", funcName, name, group, name),
			),
			gomponents.Text("Add Item"),
		),
		html.Script(gomponents.Rawf(`
			function %s%s(group, name) {
				const container = document.getElementById(group + '_' + name + '-container');
				const newRow = document.createElement('div');
				newRow.className = 'input-group mb-2';
				newRow.innerHTML = '<select class="form-select" name="%s">%s</select><button class="btn btn-outline-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()" title="Remove item"><i class=IconTimes></i></button>';
				container.insertBefore(newRow, container.lastElementChild);
			}
		`, funcName, name, id, optionString)),
	})
}

// createCommentNode creates comment display node.
func createCommentNode(comment, group, name string) gomponents.Node {
	if comment == "" {
		return nil
	}

	return renderFieldHelp(group+"-"+name, comment)
}

// createFormLayout creates the final form layout.
func createFormLayout(
	inputType, id, displayName, iconClass string,
	input, commentNode gomponents.Node,
) gomponents.Node {
	if inputType == "checkbox" {
		return html.Div(
			html.Class("form-group mb-4"),
			html.Div(
				html.Class("form-check-wrapper p-3 border rounded-3 bg-light"),
				html.Style(
					"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6 !important; transition: all 0.2s ease;",
				),
				html.Div(
					html.Class("form-check form-switch"),
					html.I(html.Class(iconClass+" text-primary me-2")),
					input,
					createFormLabel(id, displayName, true),
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
				createFormLabel(id, displayName, false),
			),
			input,
			commentNode,
		),
	)
}

// ParseIntOrDefault parses an integer from s, returning defaultVal if parsing fails.
func ParseIntOrDefault(s string, defaultVal int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}

	return defaultVal
}

func ExtractFormKeys(c *gin.Context, prefix, fieldSuffix string) map[string]bool {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, fieldSuffix) || !strings.Contains(key, prefix) {
			continue
		}

		parts := strings.Split(key, separatorUnderscore)
		if len(parts) <= 1 {
			continue
		}

		keyIndex := ""
		// Find the correct index position based on prefix structure
		if strings.Contains(prefix, "media_main_") {
			if len(parts) > fieldNameIndexPosition {
				keyIndex = parts[fieldNameIndexPosition] // media_main_movies_X_Name -> X
			}
		} else {
			keyIndex = parts[1] // downloader_X_Name -> X
		}

		if keyIndex != "" {
			formKeys[keyIndex] = true
		}
	}

	return formKeys
}

// CreateAddButton creates an add button for forms.
func CreateAddButton(
	addNewLabel, hxTarget, hxPost, hxHeaders string,
) gomponents.Node {
	attrs := []gomponents.Node{
		html.Type(attrTypeButton),
		html.Class("btn btn-success btn-sm mb-3"),
		html.Data("add-new", "true"),
		html.I(html.Class("fas fa-plus me-1")),
		gomponents.Text(addNewLabel),
	}

	if hxPost != "" {
		attrs = append(attrs, htmx.Post(hxPost))
	}

	if hxTarget != "" {
		attrs = append(attrs, htmx.Target(hxTarget))
		// Use beforeend to append new items inside the container instead of replacing existing content
		attrs = append(attrs, htmx.Swap("beforeend"))
	}

	if hxHeaders != "" {
		// Use the existing CreateHTMXHeaders function for proper CSRF token formatting
		attrs = append(attrs, htmx.Headers(CreateHTMXHeaders(hxHeaders)))
	}

	return html.Button(attrs...)
}

// CreateHTMXHeaders creates standardized HTMX headers with CSRF token.
func CreateHTMXHeaders(csrfToken string) string {
	return "{\"X-CSRF-Token\": \"" + csrfToken + "\"}"
}

// CreateFormSubmitGroup creates a submit button group for forms.
func CreateFormSubmitGroup(
	submitText, submitIcon, hxPost, hxHeaders string,
) gomponents.Node {
	btnAttrs := []gomponents.Node{
		html.Type("submit"),
		html.Class("btn btn-primary"),
	}

	if submitIcon != "" {
		btnAttrs = append(btnAttrs, html.I(html.Class(submitIcon+" me-1")))
	}

	if submitText != "" {
		btnAttrs = append(btnAttrs, gomponents.Text(submitText))
	} else {
		btnAttrs = append(btnAttrs, gomponents.Text("Save Configuration"))
	}

	if hxPost != "" {
		btnAttrs = append(btnAttrs, htmx.Swap("innerHTML"))
		btnAttrs = append(btnAttrs, htmx.Post(hxPost))
	}

	if hxHeaders != "" {
		// Use the existing CreateHTMXHeaders function for proper CSRF token formatting
		btnAttrs = append(btnAttrs, htmx.Headers(CreateHTMXHeaders(hxHeaders)))
	}

	return html.Div(
		html.Class("form-group mt-3"),
		html.Button(btnAttrs...),
	)
}

type SelectOption struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

// RenderFormFields renders a list of form fields into gomponents nodes.
func RenderFormFields(
	group string,
	comments, displayNames map[string]string,
	fields []FormFieldDefinition,
) []gomponents.Node {
	formGroups := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		// Convert SelectOption slice to map[string][]string for compatibility
		optionsMap := make(map[string][]string)
		if len(field.Options) > 0 {
			options := make([]string, len(field.Options))
			for i, opt := range field.Options {
				if strVal, ok := opt.Value.(string); ok {
					options[i] = strVal
				}
			}

			optionsMap["options"] = options
		}

		formGroups = append(
			formGroups,
			RenderFormGroup(
				group,
				comments[field.Name],
				displayNames[field.Name],
				field.Name,
				field.Type,
				field.DefaultValue,
				optionsMap,
			),
		)
	}

	return formGroups
}

// StringBuilderPool optimizes string operations via a reusable pool of string builders.
var StringBuilderPool = sync.Pool{
	New: func() any {
		return &strings.Builder{}
	},
}

// GetStringBuilder gets a string builder from the pool.
func GetStringBuilder() *strings.Builder {
	sb, ok := StringBuilderPool.Get().(*strings.Builder)
	if !ok {
		return &strings.Builder{}
	}

	sb.Reset()

	return sb
}

// PutStringBuilder returns a string builder to the pool.
func PutStringBuilder(builder *strings.Builder) {
	StringBuilderPool.Put(builder)
}

// ParseUintOrDefault parses a uint from s, returning defaultVal if parsing fails.
func ParseUintOrDefault(s string, defaultVal uint) uint {
	if s == "" {
		return defaultVal
	}

	if parsed, err := strconv.ParseUint(s, decimalBase, 32); err == nil {
		return uint(parsed)
	}

	return defaultVal
}

// GetCSRFToken extracts CSRF token from gin context if available.
func GetCSRFToken(c *gin.Context) string {
	if token, exists := c.Get("csrf_token"); exists {
		if str, ok := token.(string); ok {
			return str
		}
	}

	return ""
}

func BindJSONWithValidation(c *gin.Context, obj any) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{logger.StatusError: err.Error()})
		return false
	}

	return true
}
