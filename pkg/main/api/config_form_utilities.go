package api

import (
	"strconv"

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
		gomponents.Attr("onclick", "this.parentElement.parentElement.parentElement.remove()"),
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
			"if(confirm('Are you sure you want to reset all fields?')) { window.location.reload(); }",
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

// createConfigFormButtons creates standardized form submit and reset buttons.
func createConfigFormButtons(submitText, target, endpoint, csrfToken string) []gomponents.Node {
	return []gomponents.Node{
		html.Div(
			html.Class("form-group submit-group"),
			createSubmitButton(submitText, target, endpoint, csrfToken),
			createResetButton("Reset"),
		),
		html.Div(html.ID("addalert")),
	}
}

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
			html.Class("d-flex justify-content-center align-items-center"),
			createSubmitButton(text, target, endpoint, csrfToken),
			createResetButton("Reset"),
		),
	)
}
