package api

import (
	"strconv"

	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// Alert creation utilities

// createAlert creates a standardized alert component
func createAlert(message, alertType string) Node {
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

	return Div(
		Class("alert alert-"+alertType+" alert-column alert-dismissible shadow-sm"),
		Style("border-radius: 10px; padding: 1rem 1.25rem;"),
		Role("alert"),
		Button(
			Type("button"),
			Class("btn-close"),
			Data("bs-dismiss", "alert"),
			Aria("label", "Close"),
		),
		Div(
			Class("d-flex align-items-center"),
			Div(
				Class("alert-icon me-3"),
				Style("font-size: 1.25rem;"),
				I(Class(icon)),
			),
			Div(
				Class("alert-message flex-grow-1"),
				Style("font-weight: 500; line-height: 1.4;"),
				Text(message),
			),
		),
	)
}

// Form field creation utilities

// createFormField creates a standardized form field
func createFormField(fieldType, name, value, placeholder string, options map[string][]string) Node {
	switch fieldType {
	case FieldTypeText, FieldTypePassword, FieldTypeNumber:
		attrs := []Node{
			Type(fieldType),
			Name(name),
			Class(ClassFormControl),
		}
		if value != "" {
			attrs = append(attrs, Value(value))
		}
		if placeholder != "" {
			attrs = append(attrs, Placeholder(placeholder))
		}
		return Input(attrs...)

	case FieldTypeCheckbox:
		attrs := []Node{
			Type(FieldTypeCheckbox),
			Name(name),
			Class("form-check-input-modern"),
		}
		if value == "true" || value == "on" {
			attrs = append(attrs, Checked())
		}
		return Input(attrs...)

	case FieldTypeSelect:
		selectAttrs := []Node{
			Name(name),
			Class(ClassFormControl),
		}

		var optionNodes []Node
		if options != nil && options["options"] != nil {
			for _, option := range options["options"] {
				optAttrs := []Node{Value(option), Text(option)}
				if option == value {
					optAttrs = append(optAttrs, Selected())
				}
				optionNodes = append(optionNodes, Option(optAttrs...))
			}
		}

		return Select(append(selectAttrs, optionNodes...)...)

	default:
		return Input(
			Type(FieldTypeText),
			Name(name),
			Class(ClassFormControl),
			Value(value),
		)
	}
}

// Button creation utilities

// createButton creates a standardized button component
func createButton(text, buttonType, cssClass string, attrs ...Node) Node {
	buttonAttrs := []Node{
		Type(buttonType),
		Class(cssClass),
		Text(text),
	}
	buttonAttrs = append(buttonAttrs, attrs...)
	return Button(buttonAttrs...)
}

// Parsing utilities

// parseIntOrDefault parses a string to int with a default value
func parseIntOrDefault(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(s); err == nil {
		return parsed
	}
	return defaultValue
}

// parseUintOrDefault parses a string to uint with a default value
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

// createRemoveButton creates a standardized remove button
func createRemoveButton(float bool) Node {
	floatText := ""
	if float {
		floatText = " float: right;"
	}
	return Button(
		Class(ClassBtnDanger+" btn-lg shadow-sm mt-2"),
		Style("background: linear-gradient(135deg, #dc3545 0%, #c82333 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;"+floatText),
		Type("button"),
		I(Class("fa-solid fa-trash me-2")),
		Text("Remove Item"),
		// Attr("onclick", "if(confirm('Are you sure you want to remove this item?') && this.parentElement.parentElement.parentElement) this.parentElement.parentElement.parentElement.remove()"),
		Attr("onclick", "this.parentElement.parentElement.parentElement.remove()"),
		// Attr("onmouseover", "this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(220, 53, 69, 0.4)'"),
		// Attr("onmouseout", "this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'"),
	)
}

// createAddButton creates a standardized add button with HTMX
func createAddButton(text, target, endpoint, csrfToken string) Node {
	return Button(
		Class(ClassBtnSuccess+" btn-lg shadow-sm mt-2"),
		Style("background: linear-gradient(135deg, #28a745 0%, #20c997 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;"),
		Type("button"),
		I(Class("fa-solid fa-plus me-2")),
		Text(text),
		hx.Target(target),
		hx.Swap("beforeend"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
		Attr("onmouseover", "this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(40, 167, 69, 0.4)'"),
		Attr("onmouseout", "this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'"),
	)
}

// Form label creation

// createFormLabel creates a standardized form label
func createFormLabel(forID, text string, checkbox bool) Node {
	cssClass := ClassFormLabel
	if checkbox {
		cssClass = ClassFormCheckLabel
	}
	return Label(
		Class(cssClass),
		For(forID),
		Text(text),
	)
}

// Form button utilities

// createSubmitButton creates a standardized submit button with HTMX
func createSubmitButton(text, target, endpoint, csrfToken string) Node {
	return Button(
		Class(ClassBtnPrimary+" btn-lg shadow-sm"),
		Style("background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;"),
		I(Class("fa-solid fa-save me-2")),
		Text(text),
		Type("submit"),
		hx.Target(target),
		hx.Swap("innerHTML"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
		Attr("onmouseover", "this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(13, 110, 253, 0.4)'"),
		Attr("onmouseout", "this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'"),
	)
}

// createResetButton creates a standardized reset button
func createResetButton(text string) Node {
	return Button(
		Type("button"),
		Class(ClassBtnSecondary+" btn-lg shadow-sm ms-3"),
		Style("background: linear-gradient(135deg, #6c757d 0%, #495057 100%); border: none; transition: all 0.3s ease; padding: 0.75rem 2rem;"),
		I(Class("fa-solid fa-undo me-2")),
		Text(text),
		Attr("onclick", "if(confirm('Are you sure you want to reset all fields?')) { window.location.reload(); }"),
		Attr("onmouseover", "this.style.transform='translateY(-2px)'; this.style.boxShadow='0 6px 20px rgba(108, 117, 125, 0.4)'"),
		Attr("onmouseout", "this.style.transform='translateY(0)'; this.style.boxShadow='0 2px 4px rgba(0,0,0,0.1)'"),
	)
}

// createHTMXHeaders creates standardized HTMX headers with CSRF token
func createHTMXHeaders(csrfToken string) string {
	return "{\"X-CSRF-Token\": \"" + csrfToken + "\"}"
}

// Form group utilities

// createConfigFormButtons creates standardized form submit and reset buttons
func createConfigFormButtons(submitText, target, endpoint, csrfToken string) []Node {
	return []Node{
		Div(
			Class("form-group submit-group"),
			createSubmitButton(submitText, target, endpoint, csrfToken),
			createResetButton("Reset"),
		),
		Div(ID("addalert")),
	}
}

// createFormSubmitGroup creates a standardized form submit group with HTMX
func createFormSubmitGroup(text, target, endpoint, csrfToken string) Node {
	return Div(
		Class("form-group submit-group-enhanced mt-5 p-4 rounded-3"),
		Style("background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); border: 1px solid #dee2e6; text-align: center;"),
		H5(
			Class("mb-3 text-muted"),
			I(Class("fa-solid fa-cogs me-2")),
			Text("Configuration Actions"),
		),
		Div(
			Class("d-flex justify-content-center align-items-center"),
			createSubmitButton(text, target, endpoint, csrfToken),
			createResetButton("Reset"),
		),
	)
}
