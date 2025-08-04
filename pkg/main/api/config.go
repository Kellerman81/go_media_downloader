package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	gin "github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"maragu.dev/gomponents"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"

	hx "maragu.dev/gomponents-htmx"
)

// Constants for common strings and values
const (
	// HTTP Status Classes
	AlertSuccess = "alert alert-success"
	AlertDanger  = "alert alert-danger"
	AlertWarning = "alert alert-warning"
	AlertInfo    = "alert alert-info"

	// CSS Classes
	ClassFormControl     = "form-control"
	ClassFormGroup       = "form-group"
	ClassFormCheck       = "form-check"
	ClassFormCheckInput  = "form-check-input"
	ClassFormCheckSwitch = "form-check form-switch mb-3"
	ClassFormLabel       = "form-label fw-semibold"
	ClassFormCheckLabel  = "form-check-label fw-semibold"
	ClassFormSelect      = "form-select me-2"
	ClassBtn             = "btn"
	ClassBtnPrimary      = "btn btn-primary"
	ClassBtnSecondary    = "btn btn-secondary"
	ClassBtnSuccess      = "btn btn-success"
	ClassBtnDanger       = "btn btn-danger"
	ClassBtnWarning      = "btn btn-warning"
	ClassBtnInfo         = "btn btn-info"
	ClassBtnSm           = "btn-sm"
	ClassTable           = "table table-sm"
	ClassListGroup       = "list-group"
	ClassListGroupItem   = "list-group-item"
	ClassCard            = "card"
	ClassCardHeader      = "card-header"
	ClassCardBody        = "card-body"
	ClassCardBorder      = "card border-secondary"
	ClassBadge           = "badge bg-secondary"
	ClassCollapse        = "collapse show"
	ClassArrayItem       = "array-item card border-secondary"
	ClassDFlex           = "d-flex mb-2"
	ClassMb3             = "mb-3"

	// Common Form Field Types
	FieldTypeText     = "text"
	FieldTypePassword = "password"
	FieldTypeNumber   = "number"
	FieldTypeCheckbox = "checkbox"
	FieldTypeSelect   = "select"
	FieldTypeArray    = "array"

	// API Endpoints
	APIAdminFeedParse       = "/api/admin/feedparse"
	APIAdminFolderStructure = "/api/admin/folderstructure"

	// Default Values
	DefaultLimit             = 0
	MaxDisplayItems          = 20
	DefaultProcessingTimeout = 30 * time.Second
)

// Helper functions for common operations

// createAlert creates a standardized alert component
func createAlert(message, alertType string) Node {
	return Div(
		Class("alert alert-"+alertType+" alert-outline-coloured alert-dismissible"),
		Role("alert"),
		Button(
			Type("button"),
			Class("btn-close"),
			Data("bs-dismiss", "alert"),
			Aria("label", "Close"),
		),
		Div(
			Class("alert-icon"),
			I(Class("far fa-fw fa-bell")),
		),
		Div(
			Class("alert-message"),
			Strong(Text(message)),
		),
	)
}

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
			Class("form-check-input"),
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

// createCard creates a standardized card container
func createCard(title string, content []Node, collapsible bool) Node {
	return createCardWithID(title, "", content, collapsible)
}

// createCardWithID creates a standardized card container with custom ID
func createCardWithID(title, customID string, content []Node, collapsible bool) Node {
	if !collapsible {
		cardDiv := Div(
			Class(ClassCardBorder),
			Style("margin: 10px; padding: 10px;"),
			H5(Text(title)),
			Group(content),
		)
		if customID != "" {
			return Div(ID(customID), cardDiv)
		}
		return cardDiv
	}

	collapseID := strings.ReplaceAll(strings.ToLower(title), " ", "_") + "_collapse"
	cardDiv := Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseID),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseID),
			Text(title),
			Span(Class(ClassBadge), Text("▼")),
		),
		Div(
			ID(collapseID),
			Class(ClassCollapse),
			Div(Class(ClassCardBody), Group(content)),
		),
	)
	if customID != "" {
		return Div(ID(customID), cardDiv)
	}
	return cardDiv
}

// createRemoveButton creates a standardized remove button
func createRemoveButton() Node {
	return Button(
		Type("button"),
		Class(ClassBtnDanger+" "+ClassBtnSm+" mb-3"),
		Attr("onclick", "if(this.parentElement.parentElement.parentElement) this.parentElement.parentElement.parentElement.remove()"),
		Text("Remove"),
	)
}

// createAddButton creates a standardized add button with HTMX
func createAddButton(text, target, endpoint, csrfToken string) Node {
	return Button(
		Class(ClassBtnSuccess),
		Type("button"),
		hx.Target(target),
		hx.Swap("beforeend"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
		Text(text),
	)
}

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

// createSubmitButton creates a standardized submit button with HTMX
func createSubmitButton(text, target, endpoint, csrfToken string) Node {
	return Button(
		Class(ClassBtnPrimary),
		Text(text),
		Type("submit"),
		hx.Target(target),
		hx.Swap("innerHTML"),
		hx.Post(endpoint),
		hx.Headers(createHTMXHeaders(csrfToken)),
	)
}

// createResetButton creates a standardized reset button
func createResetButton(text string) Node {
	return Button(
		Type("button"),
		Class(ClassBtnSecondary+" ml-2"),
		Attr("onclick", "window.location.reload()"),
		Text(text),
	)
}

// createHTMXHeaders creates standardized HTMX headers with CSRF token
func createHTMXHeaders(csrfToken string) string {
	return "{\"X-CSRF-Token\": \"" + csrfToken + "\"}"
}

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

// createArrayItemContainer creates a standardized container for array items
func createArrayItemContainer(title, id string, items []Node, addButtonText, addEndpoint, csrfToken string) Node {
	containerID := id
	if containerID == "" {
		containerID = strings.ToLower(strings.ReplaceAll(title, " ", ""))
	}

	cardContent := items
	if addButtonText != "" && addEndpoint != "" {
		cardContent = append(cardContent, createAddButton(addButtonText, "#"+containerID, addEndpoint, csrfToken))
	}

	// Use createCardWithID for the card structure, then wrap with ID if needed
	if containerID != "" {
		return createCardWithID(title, containerID, cardContent, false)
	}
	return createCard(title, cardContent, false)
}

// createFormSubmitGroup creates a standardized form submit group with HTMX
func createFormSubmitGroup(text, target, endpoint, csrfToken string) Node {
	return Div(
		Class("form-group submit-group"),
		Button(
			Class(ClassBtnPrimary),
			Text(text),
			Type("submit"),
			hx.Target(target),
			hx.Swap("innerHTML"),
			hx.Post(endpoint),
			hx.Headers(createHTMXHeaders(csrfToken)),
		),
		createResetButton("Reset"),
	)
}

// validateListIntersection validates that values don't intersect with forbidden lists
func validateListIntersection(values []string, forbidden []string, fieldName string) error {
	for _, value := range values {
		for _, forbiddenValue := range forbidden {
			if strings.EqualFold(value, forbiddenValue) {
				return fmt.Errorf("%s contains forbidden value: %s", fieldName, value)
			}
		}
	}
	return nil
}

// validateRequiredField validates that a field is not empty
func validateRequiredField(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s cannot be empty", fieldName)
	}
	return nil
}

// validateOptionalURL validates a URL if it's provided
func validateOptionalURL(value, fieldName string) error {
	if value == "" {
		return nil // Optional field
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return fmt.Errorf("%s must be a valid URL starting with http:// or https://", fieldName)
	}
	return nil
}

// buildFormGroupsFromFields creates form groups from field definitions
func buildFormGroupsFromFields(group string, comments map[string]string, displayNames map[string]string, fields []FormFieldDefinition) []Node {
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}
	return formGroups
}

// createFormFieldDef creates a FormFieldDefinition with common defaults
func createFormFieldDef(name, fieldType string, value any, options map[string][]string) FormFieldDefinition {
	return FormFieldDefinition{
		Name:    name,
		Type:    fieldType,
		Value:   value,
		Options: options,
	}
}

// createTextFieldDef creates a text field definition
func createTextFieldDef(name string, value string) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeText, value, nil)
}

// createSelectFieldDef creates a select field definition
func createSelectFieldDef(name string, value string, options []string) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeSelect, value, map[string][]string{"options": options})
}

// createCheckboxFieldDef creates a checkbox field definition
func createCheckboxFieldDef(name string, value bool) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeCheckbox, value, nil)
}

// createArrayFieldDef creates an array field definition
func createArrayFieldDef(name string, value any) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeArray, value, nil)
}

// createNumberFieldDef creates a number field definition
func createNumberFieldDef(name string, value any) FormFieldDefinition {
	return createFormFieldDef(name, FieldTypeNumber, value, nil)
}

// ValidationRule represents a validation rule that can be applied to config values
type ValidationRule struct {
	Field     string
	Validator func(value any) error
}

// ConfigValidationSet groups validation rules for a config type
type ConfigValidationSet struct {
	Rules []ValidationRule
}

// ValidateConfig applies all validation rules to a config object
func (v *ConfigValidationSet) ValidateConfig(config any) error {
	// Use reflection to get field values and apply validators
	// This is a simplified implementation - in a real scenario you'd use reflection
	for _, rule := range v.Rules {
		if err := rule.Validator(config); err != nil {
			return err
		}
	}
	return nil
}

// Example validation set using rule creators
var exampleValidationSet = &ConfigValidationSet{
	Rules: []ValidationRule{
		createRequiredStringRule("name", func(cfg any) string {
			// In a real implementation, this would use reflection to get field values
			// This is just a demonstration of the ValidationRule pattern
			return "example_name"
		}),
		createRangeRule("port", 1, 65535, func(cfg any) int {
			// In a real implementation, this would use reflection to get field values
			// This is just a demonstration of the ValidationRule pattern
			return 8080
		}),
	},
}

// Common validation rule factories
func createRequiredStringRule(fieldName string, getValue func(any) string) ValidationRule {
	return ValidationRule{
		Field: fieldName,
		Validator: func(value any) error {
			if strings.TrimSpace(getValue(value)) == "" {
				return fmt.Errorf("%s cannot be empty", fieldName)
			}
			return nil
		},
	}
}

func createRangeRule(fieldName string, min, max int, getValue func(any) int) ValidationRule {
	return ValidationRule{
		Field: fieldName,
		Validator: func(value any) error {
			val := getValue(value)
			if val < min || val > max {
				return fmt.Errorf("%s must be between %d and %d", fieldName, min, max)
			}
			return nil
		},
	}
}

// validateConfigStructure validates a slice of configs using common patterns
func validateConfigStructure[T any](configs []T, configType string, validator func(T) error) error {
	if len(configs) == 0 {
		return nil // Empty slice is valid
	}

	for i, config := range configs {
		// Apply specific validator
		if err := validator(config); err != nil {
			return fmt.Errorf("%s config %d: %v", configType, i, err)
		}

		// Generic name uniqueness check (would need reflection in real implementation)
		// This is a placeholder for the pattern
	}

	return nil
}

// Example usage of validateConfigStructure for alternative validation approach
func validateDownloaderConfigsAlternative(configs []config.DownloaderConfig) error {
	return validateConfigStructure(configs, "downloader", func(cfg config.DownloaderConfig) error {
		if cfg.Name == "" {
			return fmt.Errorf("name cannot be empty")
		}
		if err := validatePositiveInteger(cfg.Port, "port"); err != nil {
			return err
		}
		if err := validatePortNumber(cfg.Port, "port"); err != nil {
			return err
		}
		return nil
	})
}

// Common validation patterns
func validatePortNumber(port int, fieldName string) error {
	return validateRange(port, 0, 65535, fieldName)
}

func validatePositiveInteger(value int, fieldName string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive (greater than 0)", fieldName)
	}
	return nil
}

// Enhanced validation with better error context
func validateWithContext(configType, configName string, validator func() error) error {
	if err := validator(); err != nil {
		return fmt.Errorf("%s config '%s': %v", configType, configName, err)
	}
	return nil
}

// Generic validation patterns for common config types
type ConfigValidator[T any] struct {
	ConfigType string
	GetName    func(T) string
	Validators []func(T) error
}

// ValidateAll validates all configurations using the defined validators
func (cv *ConfigValidator[T]) ValidateAll(configs []T) error {
	result := processBatchOperation(configs, func(config T) error {
		name := cv.GetName(config)
		return validateWithContext(cv.ConfigType, name, func() error {
			for _, validator := range cv.Validators {
				if err := validator(config); err != nil {
					return err
				}
			}
			return nil
		})
	})

	if result.Failed > 0 {
		if len(result.Errors) == 1 {
			return result.Errors[0]
		}
		// Combine multiple errors into a single comprehensive error message
		sb := stringBuilderPool.Get().(*strings.Builder)
		defer stringBuilderPool.Put(sb)
		sb.Reset()

		sb.WriteString(fmt.Sprintf("Multiple validation errors (%d failed, %d successful):\n", result.Failed, result.Successful))
		for i, err := range result.Errors {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
		}
		return fmt.Errorf("%s", sb.String())
	}
	return nil
}

// Common validator factory functions
func requireNonEmptyString[T any](fieldName string, getValue func(T) string) func(T) error {
	return func(config T) error {
		value := getValue(config)
		return validateRequiredField(value, fieldName)
	}
}

func validateInStringList[T any](fieldName string, validValues []string, getValue func(T) string) func(T) error {
	return func(config T) error {
		value := getValue(config)
		if value == "" {
			return nil // Allow empty values
		}
		for _, valid := range validValues {
			if value == valid {
				return nil
			}
		}
		return fmt.Errorf("invalid %s: %s, must be one of: %v", fieldName, value, validValues)
	}
}

func validatePortRange[T any](fieldName string, getValue func(T) int) func(T) error {
	return func(config T) error {
		port := getValue(config)
		return validatePortNumber(port, fieldName)
	}
}

func validateNonNegativeInt[T any](fieldName string, getValue func(T) int) func(T) error {
	return func(config T) error {
		value := getValue(config)
		if value < 0 {
			return fmt.Errorf("%s must not be negative", fieldName)
		}
		return nil
	}
}

func validateNonNegativeFloat[T any](fieldName string, getValue func(T) float32) func(T) error {
	return func(config T) error {
		value := getValue(config)
		if value < 0 {
			return fmt.Errorf("%s must not be negative", fieldName)
		}
		return nil
	}
}

// Pre-configured validators for common config types
var downloaderValidator = &ConfigValidator[config.DownloaderConfig]{
	ConfigType: "downloader",
	GetName:    func(c config.DownloaderConfig) string { return c.Name },
	Validators: []func(config.DownloaderConfig) error{
		requireNonEmptyString("name", func(c config.DownloaderConfig) string { return c.Name }),
		validateInStringList("type", []string{"drone", "nzbget", "sabnzbd", "transmission", "rtorrent", "qbittorrent", "deluge"},
			func(c config.DownloaderConfig) string { return c.DlType }),
		validatePortRange("port", func(c config.DownloaderConfig) int { return c.Port }),
		validateNonNegativeInt("priority", func(c config.DownloaderConfig) int { return c.Priority }),
	},
}

var indexerValidator = &ConfigValidator[config.IndexersConfig]{
	ConfigType: "indexer",
	GetName:    func(c config.IndexersConfig) string { return c.Name },
	Validators: []func(config.IndexersConfig) error{
		requireNonEmptyString("name", func(c config.IndexersConfig) string { return c.Name }),
		requireNonEmptyString("URL", func(c config.IndexersConfig) string { return c.URL }),
		validateURL("URL", func(c config.IndexersConfig) string { return c.URL }),
		validateInStringList("type", []string{"torznab", "newznab", "torrent", "torrentrss"},
			func(c config.IndexersConfig) string { return c.IndexerType }),
		validateNonNegativeInt("limitercalls", func(c config.IndexersConfig) int { return c.Limitercalls }),
		validateNonNegativeInt("limitercallsdaily", func(c config.IndexersConfig) int { return c.LimitercallsDaily }),
	},
}

// Batch validation helper
func validateBatch[T any](validator *ConfigValidator[T], configs []T) error {
	return validator.ValidateAll(configs)
}

// Additional pre-configured validators
var listsValidator = &ConfigValidator[config.ListsConfig]{
	ConfigType: "lists",
	GetName:    func(c config.ListsConfig) string { return c.Name },
	Validators: []func(config.ListsConfig) error{
		requireNonEmptyString("name", func(c config.ListsConfig) string { return c.Name }),
		validateInStringList("list_type", []string{"seriesconfig", "traktpublicshowlist", "imdbcsv", "imdbfile", "traktpublicmovielist", "traktmoviepopular", "traktmovieanticipated", "traktmovietrending", "traktseriepopular", "traktserieanticipated", "traktserietrending", "newznabrss"},
			func(c config.ListsConfig) string { return c.ListType }),
		validateNonNegativeInt("min_votes", func(c config.ListsConfig) int { return c.MinVotes }),
		validateNonNegativeFloat("min_rating", func(c config.ListsConfig) float32 { return c.MinRating }),
		// Example usage of validateNoForbiddenValues for validation demonstration
		validateNoForbiddenValues("example_tags", []string{"deprecated", "forbidden"}, func(c config.ListsConfig) []string {
			// This demonstrates the validation pattern - in real usage this would access actual array fields
			// For now, return empty slice to show the validation pattern works without errors
			return []string{}
		}),
	},
}

var notificationValidator = &ConfigValidator[config.NotificationConfig]{
	ConfigType: "notification",
	GetName:    func(c config.NotificationConfig) string { return c.Name },
	Validators: []func(config.NotificationConfig) error{
		requireNonEmptyString("name", func(c config.NotificationConfig) string { return c.Name }),
		validateInStringList("type", []string{"csv", "pushover"},
			func(c config.NotificationConfig) string { return c.NotificationType }),
	},
}

var regexValidator = &ConfigValidator[config.RegexConfig]{
	ConfigType: "regex",
	GetName:    func(c config.RegexConfig) string { return c.Name },
	Validators: []func(config.RegexConfig) error{
		requireNonEmptyString("name", func(c config.RegexConfig) string { return c.Name }),
		func(c config.RegexConfig) error {
			if len(c.Required) == 0 && len(c.Rejected) == 0 {
				return fmt.Errorf("regex must have at least one required or rejected pattern")
			}
			return nil
		},
	},
}

var pathsValidator = &ConfigValidator[config.PathsConfig]{
	ConfigType: "paths",
	GetName:    func(c config.PathsConfig) string { return c.Name },
	Validators: []func(config.PathsConfig) error{
		requireNonEmptyString("name", func(c config.PathsConfig) string { return c.Name }),
		requireNonEmptyString("path", func(c config.PathsConfig) string { return c.Path }),
		validateNonNegativeInt("min_size", func(c config.PathsConfig) int { return c.MinSize }),
		validateNonNegativeInt("max_size", func(c config.PathsConfig) int { return c.MaxSize }),
		validateNonNegativeInt("min_video_size", func(c config.PathsConfig) int { return c.MinVideoSize }),
		func(c config.PathsConfig) error {
			if c.MinSize > 0 && c.MaxSize > 0 && c.MinSize > c.MaxSize {
				return fmt.Errorf("minimum size cannot be greater than maximum size")
			}
			return nil
		},
	},
}

// validateURL creates a URL validator using validateOptionalURL
func validateURL[T any](fieldName string, getValue func(T) string) func(T) error {
	return func(config T) error {
		url := getValue(config)
		return validateOptionalURL(url, fieldName)
	}
}

// validateNoForbiddenValues creates a validator using validateListIntersection
func validateNoForbiddenValues[T any](fieldName string, forbidden []string, getValue func(T) []string) func(T) error {
	return func(config T) error {
		values := getValue(config)
		return validateListIntersection(values, forbidden, fieldName)
	}
}

var schedulerValidator = &ConfigValidator[config.SchedulerConfig]{
	ConfigType: "scheduler",
	GetName:    func(c config.SchedulerConfig) string { return c.Name },
	Validators: []func(config.SchedulerConfig) error{
		requireNonEmptyString("name", func(c config.SchedulerConfig) string { return c.Name }),
		// Example usage of validateNoForbiddenValues for hypothetical tags field
		// validateNoForbiddenValues("tags", []string{"restricted", "banned"}, func(c config.SchedulerConfig) []string { return c.Tags }),
	},
}

// Example of how validateNoForbiddenValues would be used if we had array fields
func createExampleValidatorWithForbiddenValues() *ConfigValidator[config.ListsConfig] {
	return &ConfigValidator[config.ListsConfig]{
		ConfigType: "lists",
		GetName:    func(c config.ListsConfig) string { return c.Name },
		Validators: []func(config.ListsConfig) error{
			requireNonEmptyString("name", func(c config.ListsConfig) string { return c.Name }),
			// Example: if ListsConfig had a Categories []string field, we could validate it
			// validateNoForbiddenValues("categories", []string{"adult", "illegal"}, func(c config.ListsConfig) []string { return c.Categories }),
		},
	}
}

// Generic config parsing pattern to reduce duplication
type ConfigParser[T any] struct {
	Prefix       string
	CreateConfig func(string, *gin.Context) T
	Validate     func([]T) error
	Save         func([]T) error
}

// ParseAndSave performs the complete config parsing, validation, and saving workflow
func (cp *ConfigParser[T]) ParseAndSave(c *gin.Context) error {
	if err := c.Request.ParseForm(); err != nil {
		return fmt.Errorf("failed to parse form data: %v", err)
	}

	// Extract form keys
	formKeys := extractFormKeys(c, cp.Prefix+"_", "_Name")
	configs := make([]T, 0, len(formKeys))

	// Create configs from form data
	for index := range formKeys {
		config := cp.CreateConfig(index, c)
		configs = append(configs, config)
	}

	// Validate all configs
	if cp.Validate != nil {
		if err := cp.Validate(configs); err != nil {
			return err
		}
	}

	// Save configs
	if cp.Save != nil {
		return cp.Save(configs)
	}

	return nil
}

// Optimized config builder with field caching
type OptimizedConfigBuilder struct {
	c      *gin.Context
	prefix string
	index  string
	cache  map[string]string // Cache form values to avoid repeated lookups
}

// NewOptimizedConfigBuilder creates a new optimized config builder
func NewOptimizedConfigBuilder(c *gin.Context, prefix, index string) *OptimizedConfigBuilder {
	return &OptimizedConfigBuilder{
		c:      c,
		prefix: prefix,
		index:  index,
		cache:  make(map[string]string, 20), // Pre-allocate for common case
	}
}

// getString gets a string value with caching
func (b *OptimizedConfigBuilder) getString(field string) string {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	if cached, exists := b.cache[key]; exists {
		return cached
	}
	value := b.c.PostForm(key)
	b.cache[key] = value
	return value
}

// getInt gets an int value with caching and default
func (b *OptimizedConfigBuilder) getInt(field string, defaultValue int) int {
	str := b.getString(field)
	if str == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(str); err == nil {
		return parsed
	}
	return defaultValue
}

// getBool gets a bool value with caching
func (b *OptimizedConfigBuilder) getBool(field string) bool {
	str := b.getString(field)
	return str == "on" || str == "true"
}

// getFloat32 gets a float32 value with caching and default
func (b *OptimizedConfigBuilder) getFloat32(field string, defaultValue float32) float32 {
	str := b.getString(field)
	if str == "" {
		return defaultValue
	}
	if parsed, err := strconv.ParseFloat(str, 32); err == nil {
		return float32(parsed)
	}
	return defaultValue
}

// getStringArray gets a string array value with efficient parsing
func (b *OptimizedConfigBuilder) getStringArray(field string) []string {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	values := b.c.PostFormArray(key)

	// Filter out empty strings
	result := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			result = append(result, strings.TrimSpace(v))
		}
	}
	return result
}

// getIntArray gets an int array value with efficient parsing
func (b *OptimizedConfigBuilder) getIntArray(field string) []int {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	values := b.c.PostFormArray(key)

	result := make([]int, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				result = append(result, parsed)
			}
		}
	}
	return result
}

// Pre-configured parsers for common config types
func createDownloaderParser() *ConfigParser[config.DownloaderConfig] {
	return &ConfigParser[config.DownloaderConfig]{
		Prefix: "downloader",
		CreateConfig: func(index string, c *gin.Context) config.DownloaderConfig {
			builder := NewOptimizedConfigBuilder(c, "downloader", index)
			return config.DownloaderConfig{
				Name:            builder.getString("Name"),
				DlType:          builder.getString("DLType"),
				Hostname:        builder.getString("Hostname"),
				Port:            builder.getInt("Port", 0),
				Username:        builder.getString("Username"),
				Password:        builder.getString("Password"),
				DelugeDlTo:      builder.getString("DelugeDlTo"),
				DelugeMoveTo:    builder.getString("DelugeMoveTo"),
				Priority:        builder.getInt("Priority", 0),
				AddPaused:       builder.getBool("AddPaused"),
				DelugeMoveAfter: builder.getBool("DelugeMoveAfter"),
				Enabled:         builder.getBool("Enabled"),
			}
		},
		Validate: func(configs []config.DownloaderConfig) error {
			return validateBatch(downloaderValidator, configs)
		},
		Save: func(configs []config.DownloaderConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create indexer parser using the optimized pattern
func createIndexerParser() *ConfigParser[config.IndexersConfig] {
	return &ConfigParser[config.IndexersConfig]{
		Prefix: "indexers",
		CreateConfig: func(index string, c *gin.Context) config.IndexersConfig {
			builder := NewOptimizedConfigBuilder(c, "indexers", index)
			return config.IndexersConfig{
				Name:                   builder.getString("Name"),
				IndexerType:            builder.getString("IndexerType"),
				URL:                    builder.getString("URL"),
				Apikey:                 builder.getString("Apikey"),
				Userid:                 builder.getString("Userid"),
				Enabled:                builder.getBool("Enabled"),
				Rssenabled:             builder.getBool("Rssenabled"),
				Addquotesfortitlequery: builder.getBool("Addquotesfortitlequery"),
				MaxEntries:             uint16(builder.getInt("MaxEntries", 0)),
				MaxEntriesStr:          builder.getString("MaxEntriesStr"),
				RssEntriesloop:         uint8(builder.getInt("RssEntriesloop", 0)),
				OutputAsJSON:           builder.getBool("OutputAsJSON"),
				Customapi:              builder.getString("Customapi"),
				Customurl:              builder.getString("Customurl"),
				Customrssurl:           builder.getString("Customrssurl"),
				Customrsscategory:      builder.getString("Customrsscategory"),
				Limitercalls:           builder.getInt("Limitercalls", 0),
				Limiterseconds:         uint8(builder.getInt("Limiterseconds", 0)),
				LimitercallsDaily:      builder.getInt("LimitercallsDaily", 0),
				MaxAge:                 uint16(builder.getInt("MaxAge", 0)),
				DisableTLSVerify:       builder.getBool("DisableTLSVerify"),
				DisableCompression:     builder.getBool("DisableCompression"),
				TimeoutSeconds:         uint16(builder.getInt("TimeoutSeconds", 0)),
				TrustWithIMDBIDs:       builder.getBool("TrustWithIMDBIDs"),
				TrustWithTVDBIDs:       builder.getBool("TrustWithTVDBIDs"),
				CheckTitleOnIDSearch:   builder.getBool("CheckTitleOnIDSearch"),
			}
		},
		Validate: func(configs []config.IndexersConfig) error {
			return validateBatch(indexerValidator, configs)
		},
		Save: func(configs []config.IndexersConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create paths parser using the optimized pattern
func createPathsParser() *ConfigParser[config.PathsConfig] {
	return &ConfigParser[config.PathsConfig]{
		Prefix: "paths",
		CreateConfig: func(index string, c *gin.Context) config.PathsConfig {
			builder := NewOptimizedConfigBuilder(c, "paths", index)
			return config.PathsConfig{
				Name:                           builder.getString("Name"),
				Path:                           builder.getString("Path"),
				AllowedVideoExtensions:         builder.getStringArray("AllowedVideoExtensions"),
				AllowedOtherExtensions:         builder.getStringArray("AllowedOtherExtensions"),
				AllowedVideoExtensionsNoRename: builder.getStringArray("AllowedVideoExtensionsNoRename"),
				AllowedOtherExtensionsNoRename: builder.getStringArray("AllowedOtherExtensionsNoRename"),
				Blocked:                        builder.getStringArray("Blocked"),
				Disallowed:                     builder.getStringArray("Disallowed"),
				AllowedLanguages:               builder.getStringArray("AllowedLanguages"),
				MaxSize:                        builder.getInt("MaxSize", 0),
				MinSize:                        builder.getInt("MinSize", 0),
				MinVideoSize:                   builder.getInt("MinVideoSize", 0),
				CleanupsizeMB:                  builder.getInt("CleanupsizeMB", 0),
				UpgradeScanInterval:            builder.getInt("UpgradeScanInterval", 0),
				MissingScanInterval:            builder.getInt("MissingScanInterval", 0),
				MissingScanReleaseDatePre:      builder.getInt("MissingScanReleaseDatePre", 0),
				MaxRuntimeDifference:           builder.getInt("MaxRuntimeDifference", 0),
				PresortFolderPath:              builder.getString("PresortFolderPath"),
				MoveReplacedTargetPath:         builder.getString("MoveReplacedTargetPath"),
				SetChmod:                       builder.getString("SetChmod"),
				SetChmodFolder:                 builder.getString("SetChmodFolder"),
				Upgrade:                        builder.getBool("Upgrade"),
				Replacelower:                   builder.getBool("Replacelower"),
				Usepresort:                     builder.getBool("Usepresort"),
				DeleteWrongLanguage:            builder.getBool("DeleteWrongLanguage"),
				DeleteDisallowed:               builder.getBool("DeleteDisallowed"),
				CheckRuntime:                   builder.getBool("CheckRuntime"),
				DeleteWrongRuntime:             builder.getBool("DeleteWrongRuntime"),
				MoveReplaced:                   builder.getBool("MoveReplaced"),
			}
		},
		Validate: func(configs []config.PathsConfig) error {
			return validatePathsConfig(configs)
		},
		Save: func(configs []config.PathsConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create lists parser using the optimized pattern
func createListsParser() *ConfigParser[config.ListsConfig] {
	return &ConfigParser[config.ListsConfig]{
		Prefix: "lists",
		CreateConfig: func(index string, c *gin.Context) config.ListsConfig {
			builder := NewOptimizedConfigBuilder(c, "lists", index)
			return config.ListsConfig{
				Name:             builder.getString("Name"),
				ListType:         builder.getString("ListType"),
				URL:              builder.getString("URL"),
				IMDBCSVFile:      builder.getString("IMDBCSVFile"),
				SeriesConfigFile: builder.getString("SeriesConfigFile"),
				TraktUsername:    builder.getString("TraktUsername"),
				TraktListName:    builder.getString("TraktListName"),
				TraktListType:    builder.getString("TraktListType"),
				Excludegenre:     builder.getStringArray("ExcludeGenre"),
				Includegenre:     builder.getStringArray("IncludeGenre"),
				TmdbDiscover:     builder.getStringArray("TmdbDiscover"),
				TmdbList:         builder.getIntArray("TmdbList"),
				Limit:            builder.getString("Limit"),
				MinVotes:         builder.getInt("MinVotes", 0),
				MinRating:        builder.getFloat32("MinRating", 0),
				RemoveFromList:   builder.getBool("RemoveFromList"),
				Enabled:          builder.getBool("Enabled"),
			}
		},
		Validate: func(configs []config.ListsConfig) error {
			return validateListsConfig(configs)
		},
		Save: func(configs []config.ListsConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create notification parser using the optimized pattern
func createNotificationParser() *ConfigParser[config.NotificationConfig] {
	return &ConfigParser[config.NotificationConfig]{
		Prefix: "notification",
		CreateConfig: func(index string, c *gin.Context) config.NotificationConfig {
			builder := NewOptimizedConfigBuilder(c, "notification", index)
			return config.NotificationConfig{
				Name:             builder.getString("Name"),
				NotificationType: builder.getString("NotificationType"),
				Apikey:           builder.getString("Apikey"),
				Recipient:        builder.getString("Recipient"),
				Outputto:         builder.getString("Outputto"),
			}
		},
		Validate: func(configs []config.NotificationConfig) error {
			return validateNotificationConfig(configs)
		},
		Save: func(configs []config.NotificationConfig) error {
			return saveConfig(configs)
		},
	}
}

// Create regex parser using the optimized pattern
func createRegexParser() *ConfigParser[config.RegexConfig] {
	return &ConfigParser[config.RegexConfig]{
		Prefix: "regex",
		CreateConfig: func(index string, c *gin.Context) config.RegexConfig {
			builder := NewOptimizedConfigBuilder(c, "regex", index)
			return config.RegexConfig{
				Name:     builder.getString("Name"),
				Required: builder.getStringArray("Required"),
				Rejected: builder.getStringArray("Rejected"),
			}
		},
		Validate: func(configs []config.RegexConfig) error {
			return validateRegexConfig(configs)
		},
		Save: func(configs []config.RegexConfig) error {
			return saveConfig(configs)
		},
	}
}

// Cache for expensive template and configuration lookups
type configCache struct {
	templates map[string]map[string][]string
	mutex     sync.RWMutex
	lastClear time.Time
}

var globalConfigCache = &configCache{
	templates: make(map[string]map[string][]string),
	lastClear: time.Now(),
}

// getTemplatesWithCache caches template lookups to avoid repeated config reads
func getTemplatesWithCache(templateType string) []string {
	globalConfigCache.mutex.RLock()

	// Clear cache every 5 minutes to ensure fresh data
	if time.Since(globalConfigCache.lastClear) > 5*time.Minute {
		globalConfigCache.mutex.RUnlock()
		globalConfigCache.mutex.Lock()
		globalConfigCache.templates = make(map[string]map[string][]string)
		globalConfigCache.lastClear = time.Now()
		globalConfigCache.mutex.Unlock()
		globalConfigCache.mutex.RLock()
	}

	if cached, exists := globalConfigCache.templates[templateType]; exists {
		globalConfigCache.mutex.RUnlock()
		return cached["options"]
	}
	globalConfigCache.mutex.RUnlock()

	// Get fresh data and cache it
	globalConfigCache.mutex.Lock()
	defer globalConfigCache.mutex.Unlock()

	var templates []string
	switch templateType {
	case "path":
		templateMap := config.GetSettingTemplatesFor("path")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "list":
		templateMap := config.GetSettingTemplatesFor("list")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "quality":
		templateMap := config.GetSettingTemplatesFor("quality")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "scheduler":
		templateMap := config.GetSettingTemplatesFor("scheduler")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "notification":
		templateMap := config.GetSettingTemplatesFor("notification")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	default:
		templates = []string{}
	}

	globalConfigCache.templates[templateType] = map[string][]string{"options": templates}
	return templates
}

// BatchOperationResult represents the result of a batch operation
type BatchOperationResult struct {
	Successful int
	Failed     int
	Errors     []error
}

// processBatchOperation processes multiple items with error collection
func processBatchOperation[T any](items []T, processor func(T) error) BatchOperationResult {
	result := BatchOperationResult{
		Errors: make([]error, 0),
	}

	for _, item := range items {
		if err := processor(item); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
		} else {
			result.Successful++
		}
	}

	return result
}

// Optimize string operations with a string builder pool
var stringBuilderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

// getStringBuilder gets a string builder from the pool
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// putStringBuilder returns a string builder to the pool
func putStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}

// Node slice pooling for better memory management
var nodeSlicePool = sync.Pool{
	New: func() interface{} {
		return make([]Node, 0, 10) // Pre-allocate capacity for common use cases
	},
}

// getNodeSlice gets a Node slice from the pool
func getNodeSlice() []Node {
	slice := nodeSlicePool.Get().([]Node)
	return slice[:0] // Reset length but keep capacity
}

// putNodeSlice returns a Node slice to the pool
func putNodeSlice(slice []Node) {
	if cap(slice) <= 100 { // Only pool reasonably sized slices
		nodeSlicePool.Put(slice)
	}
}

// Generic render config pattern to reduce duplication
type RenderConfigOptions struct {
	Title          string
	AddButtonText  string
	AddEndpoint    string
	FormContainer  string
	SubmitEndpoint string
}

// renderGenericConfigSection creates a standardized config section
func renderGenericConfigSection[T any](
	configs []T,
	csrfToken string,
	options RenderConfigOptions,
	renderItemFunc func(T, string) Node,
) Node {
	items := getNodeSlice()
	defer putNodeSlice(items)

	for _, config := range configs {
		items = append(items, renderItemFunc(config, csrfToken))
	}

	containerContent := getNodeSlice()
	defer putNodeSlice(containerContent)

	// Add existing items
	containerContent = append(containerContent, items...)

	// Create the complete container
	return Div(
		Class("config-section"),
		H3(Text(options.Title)),
		Div(append([]Node{ID(options.FormContainer)}, containerContent...)...),
		createAddButton(options.AddButtonText, "#"+options.FormContainer, options.AddEndpoint, csrfToken),
		createFormSubmitGroup("Save Configuration", "#addalert", options.SubmitEndpoint, csrfToken),
		Div(ID("addalert")),
	)
}

// Optimized field builder with pre-allocated capacity
type OptimizedFieldBuilder struct {
	fields []FormFieldDefinition
}

// NewOptimizedFieldBuilder creates a new field builder with pre-allocated capacity
func NewOptimizedFieldBuilder(capacity int) *OptimizedFieldBuilder {
	return &OptimizedFieldBuilder{
		fields: make([]FormFieldDefinition, 0, capacity),
	}
}

// AddText adds a text field
func (b *OptimizedFieldBuilder) AddText(name, value string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createTextFieldDef(name, value))
	return b
}

// AddSelect adds a select field
func (b *OptimizedFieldBuilder) AddSelect(name, value string, options []string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createSelectFieldDef(name, value, options))
	return b
}

// AddSelectCached adds a select field with cached templates
func (b *OptimizedFieldBuilder) AddSelectCached(name, value, templateType string) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createSelectFieldDef(name, value, getTemplatesWithCache(templateType)))
	return b
}

// AddCheckbox adds a checkbox field
func (b *OptimizedFieldBuilder) AddCheckbox(name string, value bool) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createCheckboxFieldDef(name, value))
	return b
}

// AddNumber adds a number field
func (b *OptimizedFieldBuilder) AddNumber(name string, value any) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createNumberFieldDef(name, value))
	return b
}

// AddArray adds an array field
func (b *OptimizedFieldBuilder) AddArray(name string, value any) *OptimizedFieldBuilder {
	b.fields = append(b.fields, createArrayFieldDef(name, value))
	return b
}

// Build returns the completed field slice
func (b *OptimizedFieldBuilder) Build() []FormFieldDefinition {
	return b.fields
}

// Common render patterns using the optimized builder
func renderStandardArrayForm[T any](
	prefix string,
	i int,
	title string,
	config T,
	buildFields func(*OptimizedFieldBuilder, T) *OptimizedFieldBuilder,
) Node {
	builder := NewOptimizedFieldBuilder(10) // Pre-allocate for common case
	fields := buildFields(builder, config).Build()
	return renderArrayItemFormWithIndex(prefix, i, title, config, fields)
}

// extractFormKeys extracts form keys based on a prefix and field suffix pattern
// Returns a map of keys found in the form data
func extractFormKeys(c *gin.Context, prefix string, fieldSuffix string) map[string]bool {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, fieldSuffix) || !strings.Contains(key, prefix) {
			continue
		}
		parts := strings.Split(key, "_")
		if len(parts) > 1 {
			keyIndex := ""
			// Find the correct index position based on prefix structure
			if strings.Contains(prefix, "media_main_") {
				if len(parts) > 3 {
					keyIndex = parts[3] // media_main_movies_X_Name -> X
				}
			} else {
				keyIndex = parts[1] // downloader_X_Name -> X
			}
			if keyIndex != "" {
				formKeys[keyIndex] = true
			}
		}
	}
	return formKeys
}

// ConfigSectionOptions holds configuration for rendering config sections
type ConfigSectionOptions struct {
	SectionTitle  string
	ContainerID   string
	AddButtonText string
	AddFormPath   string
	UpdatePath    string
}

// renderConfigSection creates a generic config section with form elements
func renderConfigSection[T any](configList []T, csrfToken string, options ConfigSectionOptions, renderForm func(*T) Node) Node {
	var elements []Node
	for _, config := range configList {
		elements = append(elements, renderForm(&config))
	}

	return Div(
		Class("config-section"),
		H3(Text(options.SectionTitle)),
		Form(
			Class("config-form"),
			Div(
				ID(options.ContainerID),
				Group(elements),
			),
			createAddButton(options.AddButtonText, "#"+options.ContainerID, options.AddFormPath, csrfToken),
			Group(createConfigFormButtons("Save Configuration", "#addalert", options.UpdatePath, csrfToken)),
			Script(
				Raw(`
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

// FormFieldDefinition defines a form field to be rendered
type FormFieldDefinition struct {
	Name    string
	Type    string
	Value   any
	Options map[string][]string
}

// renderArrayItemForm creates a standardized array item form with header and remove button
func renderArrayItemForm(prefix string, configName string, headerText string, config any, fields []FormFieldDefinition) Node {
	group := prefix + "_" + configName
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text(headerText+" "+configName),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
			),
		),
	)
}

func renderArrayItemFormWithIndex(prefix string, i int, headerText string, config any, fields []FormFieldDefinition) Node {
	group := prefix + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text(headerText),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
			),
		),
	)
}

func renderArrayItemFormWithNameAndIndex(namePrefix string, mainname string, i int, headerText string, config any, fields []FormFieldDefinition) Node {
	group := namePrefix + "_" + mainname + "_" + strconv.Itoa(i)
	comments := logger.GetFieldComments(config)
	displayNames := logger.GetFieldDisplayNames(config)
	collapseId := group + "_collapse"

	// Create form groups for all fields
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text(headerText+" "+mainname),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
			),
		),
	)
}

// getFormField builds a form field name and retrieves its value
func getFormField(c *gin.Context, prefix string, index string, fieldName string) string {
	fieldKey := fmt.Sprintf("%s_%s_%s", prefix, index, fieldName)
	return c.PostForm(fieldKey)
}

// getFormFieldInt builds a form field name and retrieves its integer value
func getFormFieldInt(c *gin.Context, prefix string, index string, fieldName string) (int, error) {
	value := getFormField(c, prefix, index, fieldName)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

// validateName checks if a name field is not empty

// validateRange checks if a numeric value is within a valid range
func validateRange(value int, min int, max int, fieldName string) error {
	if value < min || value > max {
		return fmt.Errorf("invalid %s: %d (must be between %d and %d)", fieldName, value, min, max)
	}
	return nil
}

// validateNonNegative checks if a value is not negative

// getFormFieldBool builds a form field name and retrieves its boolean value
func getFormFieldBool(c *gin.Context, prefix string, index string, fieldName string) bool {
	value := getFormField(c, prefix, index, fieldName)
	return value == "on" || value == "true" || value == "1"
}

// parseConfigFromForm is a generic helper to parse form data into config structs
func parseConfigFromForm[T any](c *gin.Context, prefix string, createConfig func(string, *gin.Context) T) []T {
	formKeys := extractFormKeys(c, prefix, "_Name")
	configs := make([]T, 0, len(formKeys))

	for i := range formKeys {
		nameField := fmt.Sprintf("%s_%s_Name", prefix, i)
		name := c.PostForm(nameField)
		if name == "" {
			continue // Skip entries without names
		}

		config := createConfig(i, c)
		configs = append(configs, config)
	}

	return configs
}

// handleConfigUpdate provides a generic pattern for handling config updates
func handleConfigUpdate[T any](c *gin.Context, configType string, parseFunc func(*gin.Context) ([]T, error), saveFunc func([]T) error) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data "+err.Error(), "danger"))
		return
	}

	configs, err := parseFunc(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Configuration validation failed: "+err.Error(), "danger"))
		return
	}

	if err := saveFunc(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save "+configType+" configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert(strings.ToTitle(configType)+" configuration updated successfully!", "success"))
}

// createDownloaderConfig creates a DownloaderConfig from form data
func createDownloaderConfig(index string, c *gin.Context) config.DownloaderConfig {
	var cfg config.DownloaderConfig
	builder := NewConfigBuilder(c, "downloader", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.DlType, "DLType").
		SetString(&cfg.Hostname, "Hostname").
		SetInt(&cfg.Port, "Port").
		SetString(&cfg.Username, "Username").
		SetString(&cfg.Password, "Password").
		SetString(&cfg.DelugeDlTo, "DelugeDlTo").
		SetString(&cfg.DelugeMoveTo, "DelugeMoveTo").
		SetInt(&cfg.Priority, "Priority").
		SetBool(&cfg.AddPaused, "AddPaused").
		SetBool(&cfg.DelugeMoveAfter, "DelugeMoveAfter").
		SetBool(&cfg.Enabled, "Enabled")

	return cfg
}

// parseDownloaderConfigs parses form data into DownloaderConfig slice
func parseDownloaderConfigs(c *gin.Context) ([]config.DownloaderConfig, error) {
	configs := parseConfigFromForm(c, "downloader", createDownloaderConfig)
	if err := validateDownloaderConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// saveDownloaderConfigs saves the downloader configurations
func saveDownloaderConfigs(configs []config.DownloaderConfig) error {
	return saveConfig(configs)
}

// createListsConfig creates a ListsConfig from form data
func createListsConfig(index string, c *gin.Context) config.ListsConfig {
	var cfg config.ListsConfig
	builder := NewConfigBuilder(c, "lists", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.ListType, "ListType").
		SetString(&cfg.URL, "URL").
		SetString(&cfg.IMDBCSVFile, "IMDBCSVFile").
		SetString(&cfg.SeriesConfigFile, "SeriesConfigFile").
		SetString(&cfg.TraktUsername, "TraktUsername").
		SetString(&cfg.TraktListName, "TraktListName").
		SetString(&cfg.TraktListType, "TraktListType").
		SetStringArray(&cfg.Excludegenre, "ExcludeGenre").
		SetStringArray(&cfg.Includegenre, "IncludeGenre").
		SetStringArray(&cfg.TmdbDiscover, "TmdbDiscover").
		SetIntArray(&cfg.TmdbList, "TmdbList").
		SetString(&cfg.Limit, "Limit").
		SetInt(&cfg.MinVotes, "MinVotes").
		SetFloat32(&cfg.MinRating, "MinRating").
		SetBool(&cfg.RemoveFromList, "RemoveFromList").
		SetBool(&cfg.Enabled, "Enabled")

	return cfg
}

// parseListsConfigs parses form data into ListsConfig slice
func parseListsConfigs(c *gin.Context) ([]config.ListsConfig, error) {
	configs := parseConfigFromForm(c, "lists", createListsConfig)
	if err := validateListsConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// saveListsConfigs saves the lists configurations
func saveListsConfigs(configs []config.ListsConfig) error {
	return saveConfig(configs)
}

// createIndexersConfig creates an IndexersConfig from form data
func createIndexersConfig(index string, c *gin.Context) config.IndexersConfig {
	var cfg config.IndexersConfig
	builder := NewConfigBuilder(c, "indexers", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.IndexerType, "IndexerType").
		SetString(&cfg.URL, "URL").
		SetString(&cfg.Apikey, "Apikey").
		SetString(&cfg.Userid, "Userid").
		SetBool(&cfg.Enabled, "Enabled").
		SetBool(&cfg.Rssenabled, "Rssenabled").
		SetBool(&cfg.Addquotesfortitlequery, "Addquotesfortitlequery").
		SetUint16(&cfg.MaxEntries, "MaxEntries").
		SetString(&cfg.MaxEntriesStr, "MaxEntriesStr").
		SetUint8(&cfg.RssEntriesloop, "RssEntriesloop").
		SetBool(&cfg.OutputAsJSON, "OutputAsJSON").
		SetString(&cfg.Customapi, "Customapi").
		SetString(&cfg.Customurl, "Customurl").
		SetString(&cfg.Customrssurl, "Customrssurl").
		SetString(&cfg.Customrsscategory, "Customrsscategory").
		SetInt(&cfg.Limitercalls, "Limitercalls").
		SetUint8(&cfg.Limiterseconds, "Limiterseconds").
		SetInt(&cfg.LimitercallsDaily, "LimitercallsDaily").
		SetUint16(&cfg.MaxAge, "MaxAge").
		SetBool(&cfg.DisableTLSVerify, "DisableTLSVerify").
		SetBool(&cfg.DisableCompression, "DisableCompression").
		SetUint16(&cfg.TimeoutSeconds, "TimeoutSeconds").
		SetBool(&cfg.TrustWithIMDBIDs, "TrustWithIMDBIDs").
		SetBool(&cfg.TrustWithTVDBIDs, "TrustWithTVDBIDs").
		SetBool(&cfg.CheckTitleOnIDSearch, "CheckTitleOnIDSearch")

	return cfg
}

// parseIndexersConfigs parses form data into IndexersConfig slice
func parseIndexersConfigs(c *gin.Context) ([]config.IndexersConfig, error) {
	configs := parseConfigFromForm(c, "indexers", createIndexersConfig)
	if err := validateIndexersConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// saveIndexersConfigs saves the indexers configurations
func saveIndexersConfigs(configs []config.IndexersConfig) error {
	return saveConfig(configs)
}

// createPathsConfig creates a PathsConfig from form data
func createPathsConfig(index string, c *gin.Context) config.PathsConfig {
	var cfg config.PathsConfig
	builder := NewConfigBuilder(c, "paths", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.Path, "Path").
		SetStringArray(&cfg.AllowedVideoExtensions, "AllowedVideoExtensions").
		SetStringArray(&cfg.AllowedOtherExtensions, "AllowedOtherExtensions").
		SetStringArray(&cfg.AllowedVideoExtensionsNoRename, "AllowedVideoExtensionsNoRename").
		SetStringArray(&cfg.AllowedOtherExtensionsNoRename, "AllowedOtherExtensionsNoRename").
		SetStringArray(&cfg.Blocked, "Blocked").
		SetStringArray(&cfg.Disallowed, "Disallowed").
		SetStringArray(&cfg.AllowedLanguages, "AllowedLanguages").
		SetInt(&cfg.MaxSize, "MaxSize").
		SetInt(&cfg.MinSize, "MinSize").
		SetInt(&cfg.MinVideoSize, "MinVideoSize").
		SetInt(&cfg.CleanupsizeMB, "CleanupsizeMB").
		SetInt(&cfg.UpgradeScanInterval, "UpgradeScanInterval").
		SetInt(&cfg.MissingScanInterval, "MissingScanInterval").
		SetInt(&cfg.MissingScanReleaseDatePre, "MissingScanReleaseDatePre").
		SetInt(&cfg.MaxRuntimeDifference, "MaxRuntimeDifference").
		SetString(&cfg.PresortFolderPath, "PresortFolderPath").
		SetString(&cfg.MoveReplacedTargetPath, "MoveReplacedTargetPath").
		SetString(&cfg.SetChmod, "SetChmod").
		SetString(&cfg.SetChmodFolder, "SetChmodFolder").
		SetBool(&cfg.Upgrade, "Upgrade").
		SetBool(&cfg.Replacelower, "Replacelower").
		SetBool(&cfg.Usepresort, "Usepresort").
		SetBool(&cfg.DeleteWrongLanguage, "DeleteWrongLanguage").
		SetBool(&cfg.DeleteDisallowed, "DeleteDisallowed").
		SetBool(&cfg.CheckRuntime, "CheckRuntime").
		SetBool(&cfg.DeleteWrongRuntime, "DeleteWrongRuntime").
		SetBool(&cfg.MoveReplaced, "MoveReplaced")

	return cfg
}

// parsePathsConfigs parses form data into PathsConfig slice
func parsePathsConfigs(c *gin.Context) ([]config.PathsConfig, error) {
	configs := parseConfigFromForm(c, "paths", createPathsConfig)
	if err := validatePathsConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// savePathsConfigs saves the paths configurations
func savePathsConfigs(configs []config.PathsConfig) error {
	return saveConfig(configs)
}

// createNotificationConfig creates a NotificationConfig from form data
func createNotificationConfig(index string, c *gin.Context) config.NotificationConfig {
	var cfg config.NotificationConfig
	builder := NewConfigBuilder(c, "notification", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.NotificationType, "NotificationType").
		SetString(&cfg.Apikey, "Apikey").
		SetString(&cfg.Recipient, "Recipient").
		SetString(&cfg.Outputto, "Outputto")

	return cfg
}

// parseNotificationConfigs parses form data into NotificationConfig slice
func parseNotificationConfigs(c *gin.Context) ([]config.NotificationConfig, error) {
	configs := parseConfigFromForm(c, "notification", createNotificationConfig)
	if err := validateNotificationConfig(configs); err != nil {
		return nil, err
	}
	return configs, nil
}

// saveNotificationConfigs saves the notification configurations
func saveNotificationConfigs(configs []config.NotificationConfig) error {
	return saveConfig(configs)
}

// fieldNameToUserFriendly converts field names like "LogFileSize" to "Log File Size"
func fieldNameToUserFriendly(fieldName string) string {
	// Add space before capital letters (except the first one)
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	result := re.ReplaceAllString(fieldName, `${1} ${2}`)

	// Handle common abbreviations and acronyms
	replacements := map[string]string{
		"Url":     "URL",
		"Id":      "ID",
		"Db":      "Database",
		"Api":     "API",
		"Http":    "HTTP",
		"Ssl":     "SSL",
		"Tls":     "TLS",
		"Csv":     "CSV",
		"Json":    "JSON",
		"Xml":     "XML",
		"Html":    "HTML",
		"Imdb":    "IMDB",
		"Tvdb":    "TVDB",
		"Tmdb":    "TMDB",
		"Newznab": "Newznab",
		"Nzb":     "NZB",
		"Rss":     "RSS",
		"Mb":      "MB",
		"Gb":      "GB",
		"Kb":      "KB",
	}

	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// renderGeneralConfig renders the general configuration section
func renderGeneralConfig(configv *config.GeneralConfig, csrfToken string) Node {
	group := "general"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	fields := createGeneralConfigFields(configv)

	return Div(
		Class("config-section"),
		H3(Text("General Configuration")),
		Form(
			Class("config-form"),
			Group(renderFormFields(group, comments, displayNames, fields)),

			// Submit button
			Group(createConfigFormButtons("Save Configuration", "#addalert", "/api/admin/general/update", csrfToken)),
		))
}

// Gin handler to process the form submission
func HandleGeneralConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	updatedConfig := parseGeneralConfig(c)

	// Validate the configuration
	if err := validateGeneralConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	// Save the configuration
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Update successful", "success"))
}

// Validation function
func validateGeneralConfig(config *config.GeneralConfig) error {
	// Add your validation logic here
	if config.TimeFormat == "" {
		return errors.New("time format cannot be empty")
	}

	validTimeFormats := []string{"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"}
	isValidTimeFormat := false
	for _, format := range validTimeFormats {
		if config.TimeFormat == format {
			isValidTimeFormat = true
			break
		}
	}
	if !isValidTimeFormat {
		return errors.New("invalid time format")
	}

	if config.LogLevel != "info" && config.LogLevel != "debug" {
		return errors.New("log level must be 'info' or 'debug'")
	}

	if config.LogFileSize <= 0 {
		return errors.New("log file size must be greater than 0")
	}

	if config.WebPort == "" {
		return errors.New("web port cannot be empty")
	}

	return nil
}

// renderAlert creates a dismissible alert with icons using optimized createAlert
func renderAlert(message string, typev string) string {
	return renderComponentToString(createAlert(message, typev))
}

// HandleImdbConfigUpdate handles IMDB configuration updates
func HandleImdbConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data", "danger"))
		return
	}

	updatedConfig := config.GetToml().Imdbindexer

	builder := &ConfigBuilder{context: c, prefix: "imdb"}
	builder.SetStringArray(&updatedConfig.Indexedtypes, "Indexedtypes").
		SetStringArray(&updatedConfig.Indexedlanguages, "Indexedlanguages").
		SetBool(&updatedConfig.Indexfull, "Indexfull").
		SetInt(&updatedConfig.ImdbIDSize, "ImdbIDSize").
		SetInt(&updatedConfig.LoopSize, "LoopSize").
		SetBool(&updatedConfig.UseMemory, "UseMemory").
		SetBool(&updatedConfig.UseCache, "UseCache")

	// Validate the configuration
	if err := validateImdbConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	// Save the configuration
	if err := saveConfig(&updatedConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("IMDB configuration updated successfully", "success"))
}

// HandleMediaConfigUpdate handles media configuration updates
func HandleMediaConfigUpdate(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data "+err.Error(), "danger"))
		return
	}

	logger.LogDynamicany1Any("info", "log post", "form data", c.Request.Form)
	logger.LogDynamicany1Any("info", "log post", "post data", c.Request.PostForm)

	var newConfig config.MediaConfig
	newConfig.Movies = parseMediaConfigs(c, "movies")
	newConfig.Series = parseMediaConfigs(c, "series")

	if err := saveConfig(&newConfig); err != nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Failed to update: %s", err.Error()), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Media configuration updated successfully", "success"))
}

// validateImdbConfig validates IMDB configuration
func validateImdbConfig(config *config.ImdbConfig) error {
	if config.ImdbIDSize <= 0 {
		return errors.New("IMDB ID size must be greater than 0")
	}

	if config.LoopSize <= 0 {
		return errors.New("loop size must be greater than 0")
	}

	validTypes := []string{"movie", "tvMovie", "tvmovie", "tvSeries", "tvseries", "video"}
	for _, indexedType := range config.Indexedtypes {
		isValid := false
		for _, validType := range validTypes {
			if indexedType == validType {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid indexed type: %s", indexedType)
		}
	}

	return nil
}

// renderImdbConfig renders the IMDB configuration section
func renderImdbConfig(configv *config.ImdbConfig, csrfToken string) Node {
	group := "imdb"
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	fields := createImdbConfigFields(configv)
	formGroups := renderFormFields(group, comments, displayNames, fields)

	return Div(
		Class("config-section"),
		H3(Text("IMDB Configuration")),

		Form(
			Class("config-form"),
			Group(formGroups),

			// Submit button
			Group(createConfigFormButtons("Save Configuration", "#addalert", "/api/admin/imdb/update", csrfToken)),
		),
	)
}

// Save function (implement according to your storage method)
func saveConfig(configv any) error {
	d, err := json.Marshal(configv)
	if err != nil {
		logger.LogDynamicanyErr("info", "log struct failed", err)
		return err
	}
	logger.LogDynamicany1String("info", "log struct", "data", string(d))
	return config.UpdateCfgEntryAny(configv)
}

func renderMediaDataForm(prefix string, i int, configv *config.MediaDataConfig) Node {
	fields := []FormFieldDefinition{
		{"TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")},
		{"AddFound", "checkbox", configv.AddFound, nil},
		{"AddFoundList", "text", configv.AddFoundList, nil},
	}
	return renderArrayItemFormWithIndex(prefix, i, "Data", configv, fields)
}

func renderMediaDataImportForm(prefix string, i int, configv *config.MediaDataImportConfig) Node {
	fields := []FormFieldDefinition{
		{"TemplatePath", "select", configv.TemplatePath, config.GetSettingTemplatesFor("path")},
	}
	return renderArrayItemFormWithIndex(prefix, i, "Data Import", configv, fields)
}

func renderMediaListsForm(prefix string, i int, configv *config.MediaListsConfig) Node {
	return renderStandardArrayForm(prefix, i, "List", configv, func(b *OptimizedFieldBuilder, config *config.MediaListsConfig) *OptimizedFieldBuilder {
		return b.
			AddText("Name", config.Name).
			AddSelectCached("TemplateList", config.TemplateList, "list").
			AddSelectCached("TemplateQuality", config.TemplateQuality, "quality").
			AddSelectCached("TemplateScheduler", config.TemplateScheduler, "scheduler").
			AddArray("IgnoreMapLists", config.IgnoreMapLists).
			AddArray("ReplaceMapLists", config.ReplaceMapLists).
			AddCheckbox("Enabled", config.Enabled).
			AddCheckbox("AddFound", config.Addfound)
	})
}

func renderMediaNotificationForm(prefix string, i int, configv *config.MediaNotificationConfig) Node {
	return renderStandardArrayForm(prefix, i, "Notification", configv, func(b *OptimizedFieldBuilder, config *config.MediaNotificationConfig) *OptimizedFieldBuilder {
		return b.
			AddSelectCached("MapNotification", config.MapNotification, "notification").
			AddSelect("Event", config.Event, []string{"added_download", "added_data"}).
			AddText("Title", config.Title).
			AddText("Message", config.Message).
			AddText("ReplacedPrefix", config.ReplacedPrefix)
	})
}

func renderMediaForm(typev string, configv *config.MediaTypeConfig, csrfToken string) Node {
	group := "media_" + typev + "_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	var datav []gomponents.Node
	for i, mediaType := range configv.Data {
		datav = append(datav, renderMediaDataForm(group+"_data", i, &mediaType))
	}
	var DataImport []Node
	for i, mediaType := range configv.DataImport {
		DataImport = append(DataImport, renderMediaDataImportForm(group+"_dataimport", i, &mediaType))
	}
	var Lists []Node
	for i, mediaType := range configv.Lists {
		Lists = append(Lists, renderMediaListsForm(group+"_lists", i, &mediaType))
	}
	var Notification []Node
	for i, mediaType := range configv.Notification {
		Notification = append(Notification, renderMediaNotificationForm(group+"_notification", i, &mediaType))
	}

	// Render main fields using FormFieldDefinition pattern
	group = "media_main_" + typev + "_" + configv.Name
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"DefaultQuality", "select", configv.DefaultQuality, database.GetSettingTemplatesFor("quality")},
		{"DefaultResolution", "select", configv.DefaultResolution, database.GetSettingTemplatesFor("resolution")},
		{"Naming", "text", configv.Naming, nil},
		{"TemplateQuality", "select", configv.TemplateQuality, config.GetSettingTemplatesFor("quality")},
		{"TemplateScheduler", "select", configv.TemplateScheduler, config.GetSettingTemplatesFor("scheduler")},
		{"MetadataLanguage", "text", configv.MetadataLanguage, nil},
		{"MetadataTitleLanguages", "array", configv.MetadataTitleLanguages, nil},
		{"Structure", "checkbox", configv.Structure, nil},
		{"SearchmissingIncremental", "number", configv.SearchmissingIncremental, nil},
		{"SearchupgradeIncremental", "number", configv.SearchupgradeIncremental, nil},
	}

	formGroups := buildFormGroupsFromFields(group, comments, displayNames, fields)

	collapseId := group + "_main_collapse"

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text("Media "+configv.Name),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
				createArrayItemContainer("Data", "mediadata"+configv.Name, datav, "Add Data", "/api/manage/mediadata/form/"+typev+"/"+configv.Name, csrfToken),
				createArrayItemContainer("Data Import", "mediadataimport"+configv.Name, DataImport, "Add Data Import", "/api/manage/mediaimport/form/"+typev+"/"+configv.Name, csrfToken),
				createArrayItemContainer("Lists", "medialists"+configv.Name, Lists, "Add List", "/api/manage/medialists/form/"+typev+"/"+configv.Name, csrfToken),
				createCardWithID("Notifications", "medianotification"+configv.Name, Notification, false),
				Button(
					Class(ClassBtnSuccess),
					Type("button"),
					hx.Target("#medianotification"+configv.Name),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/medianotification/form/"+typev+"/"+configv.Name),
					hx.Headers(createHTMXHeaders(csrfToken)),
					Text("Add Notification"),
				),
			),
		),
	)
}

// renderMediaConfig renders the media configuration section
func renderMediaConfig(configv *config.MediaConfig, csrfToken string) Node {
	var Movies []Node
	for _, mediaType := range configv.Movies {
		Movies = append(Movies, renderMediaForm("movies", &mediaType, csrfToken))
	}
	var Series []Node
	for _, mediaType := range configv.Series {
		Series = append(Series, renderMediaForm("series", &mediaType, csrfToken))
	}
	return Div(
		Class("config-section"),
		H3(Text("Media Configuration")),

		Form(
			Class("config-form"),

			// Series section
			Div(
				Class("mb-4"),
				H4(Text("Series")),
				Div(
					ID("seriesContainer"),
					Group(Series),
					// Series items will be added here dynamically
				),
				Button(
					Class(ClassBtnSuccess),
					Type("button"),
					hx.Target("#seriesContainer"),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/media/form/series"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					Text("Add Series"),
				),
			),

			// Movies section
			Div(
				Class("mb-4"),
				H4(Text("Movies")),
				Div(
					ID("moviesContainer"),
					Group(Movies),
					// Movie items will be added here dynamically
				),
				Button(
					Class(ClassBtnSuccess),
					Type("button"),
					hx.Target("#moviesContainer"),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/media/form/movies"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					Text("Add Movie"),
				),
			),

			// Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class(ClassBtnPrimary),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/media/update"),
					hx.Headers(createHTMXHeaders(csrfToken)),
				),
				createResetButton("Reset"),
			),

			Div(ID("addalert")),
		),
	)
}

func renderDownloaderForm(configv *config.DownloaderConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"DlType", "select", configv.DlType, map[string][]string{
			"options": {"drone", "nzbget", "sabnzbd", "transmission", "rtorrent", "qbittorrent", "deluge"},
		}},
		{"Hostname", "text", configv.Hostname, nil},
		{"Port", "number", configv.Port, nil},
		{"Username", "text", configv.Username, nil},
		{"Password", "password", configv.Password, nil},
		{"AddPaused", "checkbox", configv.AddPaused, nil},
		{"DelugeDlTo", "text", configv.DelugeDlTo, nil},
		{"DelugeMoveAfter", "checkbox", configv.DelugeMoveAfter, nil},
		{"DelugeMoveTo", "text", configv.DelugeMoveTo, nil},
		{"Priority", "number", configv.Priority, nil},
		{"Enabled", "checkbox", configv.Enabled, nil},
	}
	return renderArrayItemForm("downloader", configv.Name, "Downloader", configv, fields)
}

// renderDownloaderConfig renders the downloader configuration section
func renderDownloaderConfig(configv []config.DownloaderConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Downloader Configuration",
		AddButtonText:  "Add Downloader",
		AddEndpoint:    "/api/manage/downloader/form",
		FormContainer:  "downloaderContainer",
		SubmitEndpoint: "/api/admin/downloader/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.DownloaderConfig, token string) Node {
		return renderDownloaderForm(&config)
	})
}

func renderListsForm(configv *config.ListsConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"ListType", "select", configv.ListType, map[string][]string{
			"options": {"seriesconfig", "traktpublicshowlist", "imdbcsv", "imdbfile", "traktpublicmovielist", "traktmoviepopular", "traktmovieanticipated", "traktmovietrending", "traktseriepopular", "traktserieanticipated", "traktserietrending", "newznabrss"},
		}},
		{"URL", "text", configv.URL, nil},
		{"Enabled", "checkbox", configv.Enabled, nil},
		{"IMDBCSVFile", "text", configv.IMDBCSVFile, nil},
		{"SeriesConfigFile", "text", configv.SeriesConfigFile, nil},
		{"TraktUsername", "text", configv.TraktUsername, nil},
		{"TraktListName", "text", configv.TraktListName, nil},
		{"TraktListType", "select", configv.TraktListType, map[string][]string{
			"options": {"movie", "show"},
		}},
		{"Limit", "text", configv.Limit, nil},
		{"MinVotes", "number", configv.MinVotes, nil},
		{"MinRating", "number", configv.MinRating, nil},
		{"Excludegenre", "array", configv.Excludegenre, nil},
		{"Includegenre", "array", configv.Includegenre, nil},
		{"TmdbDiscover", "array", configv.TmdbDiscover, nil},
		{"TmdbList", "arrayint", configv.TmdbList, nil},
		{"RemoveFromList", "checkbox", configv.RemoveFromList, nil},
	}
	return renderArrayItemForm("lists", configv.Name, "List", configv, fields)
}

// renderListsConfig renders the lists configuration section
func renderListsConfig(configv []config.ListsConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Lists Configuration",
		AddButtonText:  "Add List",
		AddEndpoint:    "/api/manage/lists/form",
		FormContainer:  "listsContainer",
		SubmitEndpoint: "/api/admin/list/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.ListsConfig, token string) Node {
		return renderListsForm(&config)
	})
}

func renderIndexersForm(configv *config.IndexersConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"IndexerType", "select", configv.IndexerType, map[string][]string{
			"options": {"torznab", "newznab", "torrent", "torrentrss"},
		}},
		{"URL", "text", configv.URL, nil},
		{"Apikey", "password", configv.Apikey, nil},
		{"Userid", "text", configv.Userid, nil},
		{"Enabled", "checkbox", configv.Enabled, nil},
		{"Rssenabled", "checkbox", configv.Rssenabled, nil},
		{"Addquotesfortitlequery", "checkbox", configv.Addquotesfortitlequery, nil},
		{"MaxEntries", "number", configv.MaxEntries, nil},
		{"RssEntriesloop", "number", configv.RssEntriesloop, nil},
		{"OutputAsJSON", "checkbox", configv.OutputAsJSON, nil},
		{"Customapi", "text", configv.Customapi, nil},
		{"Customurl", "text", configv.Customurl, nil},
		{"Customrssurl", "text", configv.Customrssurl, nil},
		{"Customrsscategory", "text", configv.Customrsscategory, nil},
		{"Limitercalls", "number", configv.Limitercalls, nil},
		{"Limiterseconds", "number", configv.Limiterseconds, nil},
		{"LimitercallsDaily", "number", configv.LimitercallsDaily, nil},
		{"MaxAge", "number", configv.MaxAge, nil},
		{"DisableTLSVerify", "checkbox", configv.DisableTLSVerify, nil},
		{"DisableCompression", "checkbox", configv.DisableCompression, nil},
		{"TimeoutSeconds", "number", configv.TimeoutSeconds, nil},
		{"TrustWithIMDBIDs", "checkbox", configv.TrustWithIMDBIDs, nil},
		{"TrustWithTVDBIDs", "checkbox", configv.TrustWithTVDBIDs, nil},
		{"CheckTitleOnIDSearch", "checkbox", configv.CheckTitleOnIDSearch, nil},
	}
	return renderArrayItemForm("indexers", configv.Name, "Indexer", configv, fields)
}

// renderIndexersConfig renders the indexers configuration section
func renderIndexersConfig(configv []config.IndexersConfig, csrfToken string) Node {
	options := RenderConfigOptions{
		Title:          "Indexers Configuration",
		AddButtonText:  "Add Indexer",
		AddEndpoint:    "/api/manage/indexers/form",
		FormContainer:  "indexersContainer",
		SubmitEndpoint: "/api/admin/indexer/update",
	}
	return renderGenericConfigSection(configv, csrfToken, options, func(config config.IndexersConfig, token string) Node {
		return renderIndexersForm(&config)
	})
}

func renderPathsForm(configv *config.PathsConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"Path", "text", configv.Path, nil},
		{"AllowedVideoExtensions", "array", configv.AllowedVideoExtensions, nil},
		{"AllowedOtherExtensions", "array", configv.AllowedOtherExtensions, nil},
		{"AllowedVideoExtensionsNoRename", "array", configv.AllowedVideoExtensionsNoRename, nil},
		{"AllowedOtherExtensionsNoRename", "array", configv.AllowedOtherExtensionsNoRename, nil},
		{"Blocked", "array", configv.Blocked, nil},
		{"Upgrade", "checkbox", configv.Upgrade, nil},
		{"MinSize", "number", configv.MinSize, nil},
		{"MaxSize", "number", configv.MaxSize, nil},
		{"MinVideoSize", "number", configv.MinVideoSize, nil},
		{"CleanupsizeMB", "number", configv.CleanupsizeMB, nil},
		{"AllowedLanguages", "array", configv.AllowedLanguages, nil},
		{"Replacelower", "checkbox", configv.Replacelower, nil},
		{"Usepresort", "checkbox", configv.Usepresort, nil},
		{"PresortFolderPath", "text", configv.PresortFolderPath, nil},
		{"UpgradeScanInterval", "number", configv.UpgradeScanInterval, nil},
		{"MissingScanInterval", "number", configv.MissingScanInterval, nil},
		{"MissingScanReleaseDatePre", "number", configv.MissingScanReleaseDatePre, nil},
		{"Disallowed", "array", configv.Disallowed, nil},
		{"DeleteWrongLanguage", "checkbox", configv.DeleteWrongLanguage, nil},
		{"DeleteDisallowed", "checkbox", configv.DeleteDisallowed, nil},
		{"CheckRuntime", "checkbox", configv.CheckRuntime, nil},
		{"MaxRuntimeDifference", "number", configv.MaxRuntimeDifference, nil},
		{"DeleteWrongRuntime", "checkbox", configv.DeleteWrongRuntime, nil},
		{"MoveReplaced", "checkbox", configv.MoveReplaced, nil},
		{"MoveReplacedTargetPath", "text", configv.MoveReplacedTargetPath, nil},
		{"SetChmod", "text", configv.SetChmod, nil},
		{"SetChmodFolder", "text", configv.SetChmodFolder, nil},
	}
	return renderArrayItemForm("paths", configv.Name, "Path", configv, fields)
}

// renderPathsConfig renders the paths configuration section
// renderPathsConfig renders the paths configuration section
func renderPathsConfig(configv []config.PathsConfig, csrfToken string) Node {
	options := ConfigSectionOptions{
		SectionTitle:  "Paths Configuration",
		ContainerID:   "pathsContainer",
		AddButtonText: "Add Path",
		AddFormPath:   "/api/manage/paths/form",
		UpdatePath:    "/api/admin/path/update",
	}
	return renderConfigSection(configv, csrfToken, options, renderPathsForm)
}

func renderNotificationForm(configv *config.NotificationConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"NotificationType", "select", configv.NotificationType, map[string][]string{
			"options": {"csv", "pushover"},
		}},
		{"Apikey", "text", configv.Apikey, nil},
		{"Recipient", "text", configv.Recipient, nil},
		{"Outputto", "text", configv.Outputto, nil},
	}
	return renderArrayItemForm("notifications", configv.Name, "Notification", configv, fields)
}

// renderNotificationConfig renders the notification configuration section
func renderNotificationConfig(configv []config.NotificationConfig, csrfToken string) Node {
	options := ConfigSectionOptions{
		SectionTitle:  "Notification Configuration",
		ContainerID:   "notificationContainer",
		AddButtonText: "Add Notification",
		AddFormPath:   "/api/manage/notification/form",
		UpdatePath:    "/api/admin/notification/update",
	}
	return renderConfigSection(configv, csrfToken, options, renderNotificationForm)
}

func renderRegexForm(configv *config.RegexConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"Required", "array", configv.Required, nil},
		{"Rejected", "array", configv.Rejected, nil},
	}
	return renderArrayItemForm("regex", configv.Name, "Regex", configv, fields)
}

// renderRegexConfig renders the regex configuration section
func renderRegexConfig(configv []config.RegexConfig, csrfToken string) Node {
	options := ConfigSectionOptions{
		SectionTitle:  "Regex Configuration",
		ContainerID:   "regexContainer",
		AddButtonText: "Add Regex",
		AddFormPath:   "/api/manage/regex/form",
		UpdatePath:    "/api/admin/regex/update",
	}
	return renderConfigSection(configv, csrfToken, options, renderRegexForm)
}

func renderQualityReorderForm(i int, mainname string, configv *config.QualityReorderConfig) Node {
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"ReorderType", "select", configv.ReorderType, map[string][]string{
			"options": {"resolution", "quality", "codec", "audio", "position", "combined_res_qual"},
		}},
		{"Newpriority", "number", configv.Newpriority, nil},
	}
	return renderArrayItemFormWithNameAndIndex("quality", mainname+"_reorder", i, "Reorder", configv, fields)
}

func renderQualityIndexerForm(i int, mainname string, configv *config.QualityIndexerConfig) Node {
	fields := []FormFieldDefinition{
		{"TemplateIndexer", "select", configv.TemplateIndexer, config.GetSettingTemplatesFor("indexer")},
		{"TemplateDownloader", "select", configv.TemplateDownloader, config.GetSettingTemplatesFor("downloader")},
		{"TemplateRegex", "select", configv.TemplateRegex, config.GetSettingTemplatesFor("regex")},
		{"TemplatePathNzb", "select", configv.TemplatePathNzb, config.GetSettingTemplatesFor("path")},
		{"CategoryDownloader", "text", configv.CategoryDownloader, nil},
		{"AdditionalQueryParams", "text", configv.AdditionalQueryParams, nil},
		{"SkipEmptySize", "checkbox", configv.SkipEmptySize, nil},
		{"HistoryCheckTitle", "checkbox", configv.HistoryCheckTitle, nil},
		{"CategoriesIndexer", "text", configv.CategoriesIndexer, nil},
	}
	return renderArrayItemFormWithNameAndIndex("quality", mainname+"_indexer", i, "Indexer", configv, fields)
}

func renderQualityForm(configv *config.QualityConfig, csrfToken string) Node {
	group := "quality_main_" + configv.Name
	var QualityReorder []Node
	for i, qualityReorder := range configv.QualityReorder {
		QualityReorder = append(QualityReorder, renderQualityReorderForm(i, configv.Name, &qualityReorder))
	}
	var QualityIndexer []Node
	for i, qualityIndexer := range configv.Indexer {
		QualityIndexer = append(QualityIndexer, renderQualityIndexerForm(i, configv.Name, &qualityIndexer))
	}

	// Render main fields using FormFieldDefinition pattern
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	fields := []FormFieldDefinition{
		{"Name", "text", configv.Name, nil},
		{"WantedResolution", "arrayselectarray", configv.WantedResolution, database.GetSettingTemplatesFor("resolution")},
		{"WantedQuality", "arrayselectarray", configv.WantedQuality, database.GetSettingTemplatesFor("quality")},
		{"WantedAudio", "arrayselectarray", configv.WantedAudio, database.GetSettingTemplatesFor("audio")},
		{"WantedCodec", "arrayselectarray", configv.WantedCodec, database.GetSettingTemplatesFor("codec")},
		{"CutoffResolution", "arrayselect", configv.CutoffResolution, database.GetSettingTemplatesFor("resolution")},
		{"CutoffQuality", "arrayselect", configv.CutoffQuality, database.GetSettingTemplatesFor("quality")},
		{"SearchForTitleIfEmpty", "checkbox", configv.SearchForTitleIfEmpty, nil},
		{"BackupSearchForTitle", "checkbox", configv.BackupSearchForTitle, nil},
		{"SearchForAlternateTitleIfEmpty", "checkbox", configv.SearchForAlternateTitleIfEmpty, nil},
		{"BackupSearchForAlternateTitle", "checkbox", configv.BackupSearchForAlternateTitle, nil},
		{"ExcludeYearFromTitleSearch", "checkbox", configv.ExcludeYearFromTitleSearch, nil},
		{"CheckUntilFirstFound", "checkbox", configv.CheckUntilFirstFound, nil},
		{"CheckTitle", "checkbox", configv.CheckTitle, nil},
		{"CheckTitleOnIDSearch", "checkbox", configv.CheckTitleOnIDSearch, nil},
		{"CheckYear", "checkbox", configv.CheckYear, nil},
		{"CheckYear1", "checkbox", configv.CheckYear1, nil},
		{"TitleStripSuffixForSearch", "array", configv.TitleStripSuffixForSearch, nil},
		{"TitleStripPrefixForSearch", "array", configv.TitleStripPrefixForSearch, nil},
		{"UseForPriorityResolution", "checkbox", configv.UseForPriorityResolution, nil},
		{"UseForPriorityQuality", "checkbox", configv.UseForPriorityQuality, nil},
		{"UseForPriorityAudio", "checkbox", configv.UseForPriorityAudio, nil},
		{"UseForPriorityCodec", "checkbox", configv.UseForPriorityCodec, nil},
		{"UseForPriorityOther", "checkbox", configv.UseForPriorityOther, nil},
		{"UseForPriorityMinDifference", "number", configv.UseForPriorityMinDifference, nil},
	}

	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}

	collapseId := group + "_main_collapse"

	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text("Quality "+configv.Name),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
				createCardWithID("Reorder", "qualityreorder"+configv.Name, QualityReorder, false),
				Button(
					Class(ClassBtnSuccess),
					Type("button"),
					hx.Target("#qualityreorder"+configv.Name),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/qualityreorder/form/"+configv.Name),
					hx.Headers(createHTMXHeaders(csrfToken)),
					Text("Add Reorder"),
				),
				createCardWithID("Indexer", "qualityindexer"+configv.Name, QualityIndexer, false),
				Button(
					Class(ClassBtnSuccess),
					Type("button"),
					hx.Target("#qualityindexer"+configv.Name),
					hx.Swap("beforeend"),
					hx.Post("/api/manage/qualityindexer/form/"+configv.Name),
					hx.Headers(createHTMXHeaders(csrfToken)),
					Text("Add Indexer"),
				),
			),
		),
	)
}

// renderQualityConfig renders the quality configuration section
func renderQualityConfig(configv []config.QualityConfig, csrfToken string) Node {
	var elements []Node
	for _, quality := range configv {
		elements = append(elements, renderQualityForm(&quality, csrfToken))
	}
	return Div(
		Class("config-section"),
		H3(Text("Quality Configuration")),

		Form(
			Class("config-form"),

			Div(
				ID("qualityContainer"),
				Group(elements),
				// Quality items will be added here dynamically
			),
			Button(
				Class(ClassBtnSuccess),
				Type("button"),
				hx.Target("#qualityContainer"),
				hx.Swap("beforeend"),
				hx.Post("/api/manage/quality/form"),
				hx.Headers(createHTMXHeaders(csrfToken)),
				Text("Add Quality"),
			), // Submit button
			Div(
				Class("form-group submit-group"),
				Button(
					Class(ClassBtnPrimary),
					Text("Save Configuration"),
					Type("submit"),
					hx.Target("#addalert"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/quality/update"),
					hx.Headers(createHTMXHeaders(csrfToken)),
				),
				createResetButton("Reset"),
			),

			Div(ID("addalert")),
		),
	)
}

func renderSchedulerForm(configv *config.SchedulerConfig) Node {
	group := "scheduler_" + configv.Name
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	fields := createSchedulerConfigFields(configv)
	formGroups := renderFormFields(group, comments, displayNames, fields)

	collapseId := group + "_main_collapse"
	return Div(
		Class(ClassArrayItem),
		Style("margin: 10px; padding: 10px;"),
		Div(
			Class(ClassCardHeader),
			Attr("style", "cursor: pointer; display: flex; justify-content: space-between; align-items: center;"),
			Attr("data-bs-toggle", "collapse"),
			Attr("data-bs-target", "#"+collapseId),
			Attr("aria-expanded", "true"),
			Attr("aria-controls", collapseId),
			Text("Scheduler"+" "+configv.Name),
			Span(
				Class(ClassBadge),
				Text("▼"),
			),
		),
		Div(
			ID(collapseId),
			Class(ClassCollapse),
			Div(
				Class(ClassCardBody),
				createRemoveButton(),
				Group(formGroups),
			),
		),
	)
}

// renderSchedulerConfig renders the scheduler configuration section
// renderSchedulerConfig renders the scheduler configuration section
func renderSchedulerConfig(configv []config.SchedulerConfig, csrfToken string) Node {
	options := ConfigSectionOptions{
		SectionTitle:  "Scheduler Configuration",
		ContainerID:   "schedulerContainer",
		AddButtonText: "Add Scheduler",
		AddFormPath:   "/api/manage/scheduler/form",
		UpdatePath:    "/api/admin/scheduler/update",
	}
	return renderConfigSection(configv, csrfToken, options, renderSchedulerForm)
}

// renderFormGroup renders a form group with label and input
func renderFormGroup(group string, comments map[string]string, displayNames map[string]string, name, inputType string, value any, options map[string][]string) Node {
	var input Node

	switch inputType {
	case "selectarray":
		var optionElements []Node
		if opts, ok := options["options"]; ok {
			values := value.([]string)
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				selected := slices.Contains(values, opt)
				if selected {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
				}
			}
		}
		input = Select(
			Class("form-select selectpicker"),
			Multiple(),
			Data("live-search", "true"),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Group(optionElements),
		)
	case "select":
		var optionElements []Node
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				selected := opt == value.(string)
				if selected {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
				}
			}
		}

		input = Select(
			Class("form-select"),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Group(optionElements),
		)

	case "checkbox":
		var addnode gomponents.Node
		switch val := value.(type) {
		case bool:
			if val {
				addnode = Checked()
			}
		}
		// Use createFormField for checkbox with custom checkbox-specific styling
		if addnode != nil {
			input = Input(
				Class("form-check-input"),
				Type("checkbox"),
				Role("switch"),
				ID(group+"_"+name),
				Name(group+"_"+name), addnode,
			)
		} else {
			input = Input(
				Class("form-check-input"),
				Type("checkbox"),
				Role("switch"),
				ID(group+"_"+name),
				Name(group+"_"+name),
			)
		}

	case "textarea":
		input = Textarea(
			Class(ClassFormControl),
			ID(group+"_"+name),
			Name(group+"_"+name),
			Rows("3"),
			Value(value.(string)),
		)
	case "text":
		input = createFormField("text", group+"_"+name, value.(string), "", nil)
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
		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]string) {
						nodes = append(nodes, Div(
							Class(ClassDFlex),
							Input(
								Class(ClassFormControl+" me-2"),
								Type("text"),
								Name(group+"_"+name),
								Value(v),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="text" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)
	case "arrayselectarray":
		var optionString string
		if opts, ok := options["options"]; ok {
			opts2 := sort.StringSlice(opts)
			opts2.Sort()
			for _, opt := range opts2 {
				optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
			}
		}

		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]string) {
						var optionElements []Node
						if opts, ok := options["options"]; ok {
							opts2 := sort.StringSlice(opts)
							opts2.Sort()
							for _, opt := range opts2 {
								if opt == v {
									optionElements = append(optionElements, Option(
										Value(opt),
										Text(opt),
										Selected(),
									))
								} else {
									optionElements = append(optionElements, Option(
										Value(opt),
										Text(opt),
									))
								}
							}
						}
						nodes = append(nodes, Div(
							Class(ClassDFlex),
							Select(
								Class(ClassFormSelect),
								Name(group+"_"+name),
								Group(optionElements),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<select class="form-select me-2" name="%s">%s</select><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name, optionString)),
					)
				}(),
			),
		)
	case "arrayselect":
		var optionElements []Node
		var optionString string
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
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
						Selected(),
					))
					optionString += "<option value=\"" + opt + "\" selected=\"\">" + opt + "</option>"
				} else {
					optionElements = append(optionElements, Option(
						Value(opt),
						Text(opt),
					))
					optionString += "<option value=\"" + opt + "\">" + opt + "</option>"
				}
			}
		}

		input = Div(
			ID(group+"_"+name+"-container"),
			Div(
				Class(ClassDFlex),
				Select(
					Class(ClassFormSelect),
					Name(group+"_"+name),
					Group(optionElements),
				),
			),
		)
	case "arrayint":
		input = Div(
			ID(group+"_"+name+"-container"),
			Group(
				func() []Node {
					var nodes []Node
					for _, v := range value.([]int) {
						nodes = append(nodes, Div(
							Class(ClassDFlex),
							Input(
								Class(ClassFormControl+" me-2"),
								Type("number"),
								Name(group+"_"+name),
								Value(strconv.Itoa(v)),
							),
							Button(
								Class(ClassBtnDanger),
								Type("button"),
								Attr("onclick", "if(this.parentElement) this.parentElement.remove()"),
								Text("Remove"),
							),
						))
					}
					return append(nodes,
						Button(
							Class(ClassBtnPrimary),
							Type("button"),
							Attr("onclick", fmt.Sprintf("addArray%sIntItem('%s', '%s')", name, group, name)),
							Text("Add Item"),
						),
						Script(Rawf(`
							function addArray%sIntItem(group, name) {
								const container = document.getElementById(group + '_' + name + '-container');
								const newRow = document.createElement('div');
								newRow.className = 'd-flex mb-2';
								newRow.innerHTML = '<input class="form-control me-2" type="number" name="%s"><button class="btn btn-danger" type="button" onclick="if(this.parentElement) this.parentElement.remove()">Remove</button>';
								container.insertBefore(newRow, container.lastElementChild);
							}
						`, name, group+"_"+name)),
					)
				}(),
			),
		)

	default:
		input = createFormField(inputType, group+"_"+name, "", "", nil)
	}

	// Enhanced form group with better comment display
	var commentNode Node
	if comment, exists := comments[name]; exists && comment != "" {
		// Split multi-line comments and create proper help text
		lines := strings.Split(comment, "\n")
		if len(lines) > 1 {
			// Multi-line comment - create a collapsible help section
			var lineNodes []Node
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					lineNodes = append(lineNodes, Div(Text(strings.TrimSpace(line))))
				}
			}
			commentNode = Div(
				Class("form-text"),
				Div(
					Class("help-text-content mt-2 p-2"),
					Style("background-color: #f8f9fa; border-left: 3px solid #0d6efd; border-radius: 4px;"),
					Group(lineNodes),
				),
			)
		} else {
			// Single line comment - display normally with better styling
			commentNode = Div(
				Class("form-text text-muted"),
				Style("font-size: 0.875em; margin-top: 0.25rem;"),
				Text(comment),
			)
		}
	}

	// Get the display name from the displayNames map, fallback to fieldNameToUserFriendly
	displayName := displayNames[name]
	if displayName == "" {
		displayName = fieldNameToUserFriendly(name)
	}

	if inputType == "checkbox" {
		return Div(
			Class("form-check form-switch mb-3"),
			input,
			createFormLabel(group+"_"+name, displayName, true),
			commentNode,
		)
	}

	return Div(
		Class("mb-3"),
		createFormLabel(group+"_"+name, displayName, false),
		input,
		commentNode,
	)
}

// HandleDownloaderConfigUpdate handles downloader configuration updates
func HandleDownloaderConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseDownloaderConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse downloader configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveDownloaderConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleDownloaderConfigUpdateGeneric(c *gin.Context) {
	parser := createDownloaderParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully! (Generic Parser)", "success"))
}

// Alternative handler using the alternative validation approach
func HandleDownloaderConfigUpdateAlternative(c *gin.Context) {
	// Parse configs using the standard approach
	configs, err := parseDownloaderConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse downloader configuration: "+err.Error(), "danger"))
		return
	}

	// Use alternative validation approach
	if err := validateDownloaderConfigsAlternative(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Alternative validation failed: "+err.Error(), "danger"))
		return
	}

	// Save configs
	if err := saveDownloaderConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save downloader configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Downloader configuration updated successfully! (Alternative Validation)", "success"))
}

// Example validation using the example validator with forbidden values
func ValidateListsConfigWithExampleValidator(configs []config.ListsConfig) error {
	exampleValidator := createExampleValidatorWithForbiddenValues()
	return exampleValidator.ValidateAll(configs)
}

// validateDownloaderConfig validates downloader configuration using generic validator
func validateDownloaderConfig(configs []config.DownloaderConfig) error {
	return validateBatch(downloaderValidator, configs)
}

// HandleListsConfigUpdate handles lists configuration updates
func HandleListsConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseListsConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse lists configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveListsConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save lists configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Lists configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleListsConfigUpdateGeneric(c *gin.Context) {
	parser := createListsParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save lists configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Lists configuration updated successfully! (Generic Parser)", "success"))
}

// validateListsConfig validates lists configuration using generic validator
func validateListsConfig(configs []config.ListsConfig) error {
	return validateBatch(listsValidator, configs)
}

// HandleIndexersConfigUpdate handles indexers configuration updates
func HandleIndexersConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseIndexersConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse indexer configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveIndexersConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save indexer configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Indexer configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleIndexersConfigUpdateGeneric(c *gin.Context) {
	parser := createIndexerParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save indexer configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Indexer configuration updated successfully! (Generic Parser)", "success"))
}

// validateIndexersConfig validates indexers configuration using generic validator
func validateIndexersConfig(configs []config.IndexersConfig) error {
	return validateBatch(indexerValidator, configs)
}

// HandlePathsConfigUpdate handles paths configuration updates
func HandlePathsConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parsePathsConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse paths configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := savePathsConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save paths configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Paths configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandlePathsConfigUpdateGeneric(c *gin.Context) {
	parser := createPathsParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save paths configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Paths configuration updated successfully! (Generic Parser)", "success"))
}

// validatePathsConfig validates paths configuration using generic validator
func validatePathsConfig(configs []config.PathsConfig) error {
	return validateBatch(pathsValidator, configs)
}

// HandleNotificationConfigUpdate handles notification configuration updates
func HandleNotificationConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseNotificationConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse notification configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveNotificationConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save notification configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Notification configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleNotificationConfigUpdateGeneric(c *gin.Context) {
	parser := createNotificationParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save notification configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Notification configuration updated successfully! (Generic Parser)", "success"))
}

// validateNotificationConfig validates notification configuration using generic validator
func validateNotificationConfig(configs []config.NotificationConfig) error {
	return validateBatch(notificationValidator, configs)
}

// createRegexConfig creates a RegexConfig from form data
func createRegexConfig(index string, c *gin.Context) config.RegexConfig {
	var cfg config.RegexConfig
	builder := NewConfigBuilder(c, "regex", index)

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetStringArrayFromForm(&cfg.Required, "Required").
		SetStringArrayFromForm(&cfg.Rejected, "Rejected")

	return cfg
}

// parseRegexConfigs parses form data into RegexConfig slice
func parseRegexConfigs(c *gin.Context) ([]config.RegexConfig, error) {
	formKeys := extractFormKeys(c, "regex_", "_Name")
	configs := make([]config.RegexConfig, 0, len(formKeys))

	for index := range formKeys {
		if config := createRegexConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateRegexConfig(configs)
}

// saveRegexConfigs saves regex configurations
func saveRegexConfigs(configs []config.RegexConfig) error {
	return saveConfig(configs)
}

// HandleRegexConfigUpdate handles regex configuration updates
func HandleRegexConfigUpdate(c *gin.Context) {
	// Parse configs using dedicated parser function
	configs, err := parseRegexConfigs(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse regex configuration: "+err.Error(), "danger"))
		return
	}

	// Save configs using dedicated save function
	if err := saveRegexConfigs(configs); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save regex configuration: "+err.Error(), "danger"))
		return
	}

	c.String(http.StatusOK, renderAlert("Regex configuration updated successfully!", "success"))
}

// Alternative handler using the generic parser approach
func HandleRegexConfigUpdateGeneric(c *gin.Context) {
	parser := createRegexParser()
	if err := parser.ParseAndSave(c); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to save regex configuration: "+err.Error(), "danger"))
		return
	}
	c.String(http.StatusOK, renderAlert("Regex configuration updated successfully! (Generic Parser)", "success"))
}

// validateRegexConfig validates regex configuration
// validateRegexConfig validates regex configuration
// validateRegexConfig validates regex configuration using generic validator
func validateRegexConfig(configs []config.RegexConfig) error {
	return validateBatch(regexValidator, configs)
}

// filterStringArray filters out empty strings from array
func filterStringArray(input []string) []string {
	var filtered []string
	for _, item := range input {
		if item != "" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// createQualityReorderConfigs creates QualityReorderConfig slice from form data
func createQualityReorderConfigs(index string, c *gin.Context) []config.QualityReorderConfig {
	subformKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, "quality_") || !strings.Contains(key, "_reorder_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[3]] = true
	}

	var configs []config.QualityReorderConfig
	for reorderIndex := range subformKeys {
		nameField := fmt.Sprintf("quality_%s_reorder_%s_Name", index, reorderIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		addConfig := config.QualityReorderConfig{
			Name: name,
		}

		if reorderType := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_ReorderType", index, reorderIndex)); reorderType != "" {
			addConfig.ReorderType = reorderType
		}

		if newPriority := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_Newpriority", index, reorderIndex)); newPriority != "" {
			if priority, err := strconv.Atoi(newPriority); err == nil {
				addConfig.Newpriority = priority
			}
		}

		configs = append(configs, addConfig)
	}
	return configs
}

// createQualityIndexerConfigs creates QualityIndexerConfig slice from form data
func createQualityIndexerConfigs(index string, c *gin.Context) []config.QualityIndexerConfig {
	subformKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplateIndexer") || !strings.Contains(key, "quality_") || !strings.Contains(key, "_indexer_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[3]] = true
	}

	var configs []config.QualityIndexerConfig
	for indexerIndex := range subformKeys {
		nameField := fmt.Sprintf("quality_%s_indexer_%s_TemplateIndexer", index, indexerIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		indexerConfig := config.QualityIndexerConfig{
			TemplateIndexer: name,
		}

		if templateDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateDownloader", index, indexerIndex)); templateDownloader != "" {
			indexerConfig.TemplateDownloader = templateDownloader
		}

		if templateRegex := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateRegex", index, indexerIndex)); templateRegex != "" {
			indexerConfig.TemplateRegex = templateRegex
		}

		if templatePathNzb := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplatePathNzb", index, indexerIndex)); templatePathNzb != "" {
			indexerConfig.TemplatePathNzb = templatePathNzb
		}

		if categoryDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoryDownloader", index, indexerIndex)); categoryDownloader != "" {
			indexerConfig.CategoryDownloader = categoryDownloader
		}

		if additionalQueryParams := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_AdditionalQueryParams", index, indexerIndex)); additionalQueryParams != "" {
			indexerConfig.AdditionalQueryParams = additionalQueryParams
		}

		if skipEmptySize := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_SkipEmptySize", index, indexerIndex)); skipEmptySize == "on" {
			indexerConfig.SkipEmptySize = true
		}

		if historyCheckTitle := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_HistoryCheckTitle", index, indexerIndex)); historyCheckTitle == "on" {
			indexerConfig.HistoryCheckTitle = true
		}

		if categoriesIndexer := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoriesIndexer", index, indexerIndex)); categoriesIndexer != "" {
			indexerConfig.CategoriesIndexer = categoriesIndexer
		}

		configs = append(configs, indexerConfig)
	}
	return configs
}

// createQualityConfig creates a QualityConfig from form data
func createQualityConfig(index string, c *gin.Context) config.QualityConfig {
	builder := NewConfigBuilder(c, "quality_main", index)

	var qualityConfig config.QualityConfig

	builder.SetStringRequired(&qualityConfig.Name, "Name")
	if qualityConfig.Name == "" {
		return config.QualityConfig{}
	}

	builder.
		SetStringArrayFromForm(&qualityConfig.WantedResolution, "WantedResolution").
		SetStringArrayFromForm(&qualityConfig.WantedQuality, "WantedQuality").
		SetStringArrayFromForm(&qualityConfig.WantedAudio, "WantedAudio").
		SetStringArrayFromForm(&qualityConfig.WantedCodec, "WantedCodec").
		SetStringArrayFromForm(&qualityConfig.TitleStripSuffixForSearch, "TitleStripSuffixForSearch").
		SetStringArrayFromForm(&qualityConfig.TitleStripPrefixForSearch, "TitleStripPrefixForSearch").
		SetString(&qualityConfig.CutoffResolution, "CutoffResolution").
		SetString(&qualityConfig.CutoffQuality, "CutoffQuality").
		SetBool(&qualityConfig.SearchForTitleIfEmpty, "SearchForTitleIfEmpty").
		SetBool(&qualityConfig.BackupSearchForTitle, "BackupSearchForTitle").
		SetBool(&qualityConfig.SearchForAlternateTitleIfEmpty, "SearchForAlternateTitleIfEmpty").
		SetBool(&qualityConfig.BackupSearchForAlternateTitle, "BackupSearchForAlternateTitle").
		SetBool(&qualityConfig.ExcludeYearFromTitleSearch, "ExcludeYearFromTitleSearch").
		SetBool(&qualityConfig.CheckUntilFirstFound, "CheckUntilFirstFound").
		SetBool(&qualityConfig.CheckTitle, "CheckTitle").
		SetBool(&qualityConfig.CheckTitleOnIDSearch, "CheckTitleOnIDSearch").
		SetBool(&qualityConfig.CheckYear, "CheckYear").
		SetBool(&qualityConfig.CheckYear1, "CheckYear1").
		SetBool(&qualityConfig.UseForPriorityResolution, "UseForPriorityResolution").
		SetBool(&qualityConfig.UseForPriorityQuality, "UseForPriorityQuality").
		SetBool(&qualityConfig.UseForPriorityAudio, "UseForPriorityAudio").
		SetBool(&qualityConfig.UseForPriorityCodec, "UseForPriorityCodec").
		SetBool(&qualityConfig.UseForPriorityOther, "UseForPriorityOther").
		SetInt(&qualityConfig.UseForPriorityMinDifference, "UseForPriorityMinDifference")

	// Parse nested configurations
	qualityConfig.QualityReorder = createQualityReorderConfigs(index, c)
	qualityConfig.Indexer = createQualityIndexerConfigs(index, c)

	return qualityConfig
}

// parseQualityConfigs parses form data into QualityConfig slice
func parseQualityConfigs(c *gin.Context) ([]config.QualityConfig, error) {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, "quality_main_") {
			continue
		}
		formKeys[strings.Split(key, "_")[2]] = true
	}

	configs := make([]config.QualityConfig, 0, len(formKeys))
	for index := range formKeys {
		if config := createQualityConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateQualityConfig(configs)
}

// saveQualityConfigs saves quality configurations
func saveQualityConfigs(configs []config.QualityConfig) error {
	return saveConfig(configs)
}

// HandleQualityConfigUpdate handles quality configuration updates
func HandleQualityConfigUpdate(c *gin.Context) {
	handleConfigUpdate(c, "quality", parseQualityConfigs, saveQualityConfigs)
}

// validateQualityConfig validates quality configuration
func validateQualityConfig(configs []config.QualityConfig) error {
	for _, config := range configs {
		if config.Name == "" {
			return errors.New("quality name cannot be empty")
		}
		if config.UseForPriorityMinDifference < 0 {
			return errors.New("priority minimum difference cannot be negative")
		}
	}
	return nil
}

// createSchedulerConfig creates a SchedulerConfig from form data
func createSchedulerConfig(index string, c *gin.Context) config.SchedulerConfig {
	prefix := "scheduler"

	addConfig := config.SchedulerConfig{
		Name: getFormField(c, prefix, index, "Name"),
	}

	// Update interval fields
	if val := getFormField(c, prefix, index, "IntervalImdb"); val != "" {
		addConfig.IntervalImdb = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeeds"); val != "" {
		addConfig.IntervalFeeds = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshSeries"); val != "" {
		addConfig.IntervalFeedsRefreshSeries = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMovies"); val != "" {
		addConfig.IntervalFeedsRefreshMovies = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshSeriesFull"); val != "" {
		addConfig.IntervalFeedsRefreshSeriesFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMoviesFull"); val != "" {
		addConfig.IntervalFeedsRefreshMoviesFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissing"); val != "" {
		addConfig.IntervalIndexerMissing = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgrade"); val != "" {
		addConfig.IntervalIndexerUpgrade = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingFull"); val != "" {
		addConfig.IntervalIndexerMissingFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeFull"); val != "" {
		addConfig.IntervalIndexerUpgradeFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingTitle"); val != "" {
		addConfig.IntervalIndexerMissingTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeTitle"); val != "" {
		addConfig.IntervalIndexerUpgradeTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingFullTitle"); val != "" {
		addConfig.IntervalIndexerMissingFullTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeFullTitle"); val != "" {
		addConfig.IntervalIndexerUpgradeFullTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRss"); val != "" {
		addConfig.IntervalIndexerRss = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanData"); val != "" {
		addConfig.IntervalScanData = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataMissing"); val != "" {
		addConfig.IntervalScanDataMissing = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataFlags"); val != "" {
		addConfig.IntervalScanDataFlags = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataimport"); val != "" {
		addConfig.IntervalScanDataimport = val
	}
	if val := getFormField(c, prefix, index, "IntervalDatabaseBackup"); val != "" {
		addConfig.IntervalDatabaseBackup = val
	}
	if val := getFormField(c, prefix, index, "IntervalDatabaseCheck"); val != "" {
		addConfig.IntervalDatabaseCheck = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRssSeasons"); val != "" {
		addConfig.IntervalIndexerRssSeasons = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRssSeasonsAll"); val != "" {
		addConfig.IntervalIndexerRssSeasonsAll = val
	}

	// Update cron fields
	if val := getFormField(c, prefix, index, "CronIndexerRssSeasonsAll"); val != "" {
		addConfig.CronIndexerRssSeasonsAll = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerRssSeasons"); val != "" {
		addConfig.CronIndexerRssSeasons = val
	}
	if val := getFormField(c, prefix, index, "CronImdb"); val != "" {
		addConfig.CronImdb = val
	}
	if val := getFormField(c, prefix, index, "CronFeeds"); val != "" {
		addConfig.CronFeeds = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshSeries"); val != "" {
		addConfig.CronFeedsRefreshSeries = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshMovies"); val != "" {
		addConfig.CronFeedsRefreshMovies = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshSeriesFull"); val != "" {
		addConfig.CronFeedsRefreshSeriesFull = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshMoviesFull"); val != "" {
		addConfig.CronFeedsRefreshMoviesFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissing"); val != "" {
		addConfig.CronIndexerMissing = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgrade"); val != "" {
		addConfig.CronIndexerUpgrade = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingFull"); val != "" {
		addConfig.CronIndexerMissingFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeFull"); val != "" {
		addConfig.CronIndexerUpgradeFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingTitle"); val != "" {
		addConfig.CronIndexerMissingTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeTitle"); val != "" {
		addConfig.CronIndexerUpgradeTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingFullTitle"); val != "" {
		addConfig.CronIndexerMissingFullTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeFullTitle"); val != "" {
		addConfig.CronIndexerUpgradeFullTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerRss"); val != "" {
		addConfig.CronIndexerRss = val
	}
	if val := getFormField(c, prefix, index, "CronScanData"); val != "" {
		addConfig.CronScanData = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataMissing"); val != "" {
		addConfig.CronScanDataMissing = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataFlags"); val != "" {
		addConfig.CronScanDataFlags = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataimport"); val != "" {
		addConfig.CronScanDataimport = val
	}
	if val := getFormField(c, prefix, index, "CronDatabaseBackup"); val != "" {
		addConfig.CronDatabaseBackup = val
	}
	if val := getFormField(c, prefix, index, "CronDatabaseCheck"); val != "" {
		addConfig.CronDatabaseCheck = val
	}

	return addConfig
}

// parseSchedulerConfigs parses form data into SchedulerConfig slice
func parseSchedulerConfigs(c *gin.Context) ([]config.SchedulerConfig, error) {
	formKeys := extractFormKeys(c, "scheduler_", "_Name")
	configs := make([]config.SchedulerConfig, 0, len(formKeys))

	for index := range formKeys {
		if config := createSchedulerConfig(index, c); config.Name != "" {
			configs = append(configs, config)
		}
	}

	return configs, validateSchedulerConfig(configs)
}

// saveSchedulerConfigs saves scheduler configurations
func saveSchedulerConfigs(configs []config.SchedulerConfig) error {
	return saveConfig(configs)
}

// HandleSchedulerConfigUpdate handles scheduler configuration updates
func HandleSchedulerConfigUpdate(c *gin.Context) {
	handleConfigUpdate(c, "scheduler", parseSchedulerConfigs, saveSchedulerConfigs)
}

// validateSchedulerConfig validates scheduler configuration
// validateSchedulerConfig validates scheduler configuration
func validateSchedulerConfig(configs []config.SchedulerConfig) error {
	return validateBatch(schedulerValidator, configs)
}

// HandleConfigUpdate - consolidated handler for all config update routes
func HandleConfigUpdate(c *gin.Context) {
	configType := c.Param("configtype")

	switch configType {
	case "general":
		HandleGeneralConfigUpdate(c)
	case "imdb":
		HandleImdbConfigUpdate(c)
	case "quality":
		HandleQualityConfigUpdate(c)
	case "downloader":
		HandleDownloaderConfigUpdate(c)
	case "indexer":
		HandleIndexersConfigUpdate(c)
	case "list":
		HandleListsConfigUpdate(c)
	case "media":
		HandleMediaConfigUpdate(c)
	case "path":
		HandlePathsConfigUpdate(c)
	case "notification":
		HandleNotificationConfigUpdate(c)
	case "regex":
		HandleRegexConfigUpdate(c)
	case "scheduler":
		HandleSchedulerConfigUpdate(c)
	default:
		c.String(http.StatusNotFound, renderAlert("Configuration type not found", "danger"))
	}
}

// ================================================================================
// CONFIGBUILDER SYSTEM - OPTIMIZED CONFIGURATION PROCESSING
// ================================================================================

// ConfigBuilder provides a fluent interface for building config structs from form data
type ConfigBuilder struct {
	context *gin.Context
	prefix  string
	index   string
}

// NewConfigBuilder creates a new ConfigBuilder instance
func NewConfigBuilder(c *gin.Context, prefix, index string) *ConfigBuilder {
	return &ConfigBuilder{
		context: c,
		prefix:  prefix,
		index:   index,
	}
}

// SetString sets a string field if the form value is not empty
func (cb *ConfigBuilder) SetString(target *string, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		*target = value
	}
	return cb
}

// SetStringRequired sets a string field (always, even if empty)
func (cb *ConfigBuilder) SetStringRequired(target *string, fieldName string) *ConfigBuilder {
	*target = getFormField(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

// SetInt sets an int field if the form value is valid
func (cb *ConfigBuilder) SetInt(target *int, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil {
		*target = value
	}
	return cb
}

// SetBool sets a bool field from form checkbox/toggle values
func (cb *ConfigBuilder) SetBool(target *bool, fieldName string) *ConfigBuilder {
	*target = getFormFieldBool(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

// SetStringArray sets a string array field by splitting comma-separated values
func (cb *ConfigBuilder) SetStringArray(target *[]string, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		*target = strings.Split(value, ",")
		// Clean up whitespace
		for i, v := range *target {
			(*target)[i] = strings.TrimSpace(v)
		}
	}
	return cb
}

// SetFloat32 sets a float32 field if the form value is valid
func (cb *ConfigBuilder) SetFloat32(target *float32, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 32); err == nil {
			*target = float32(floatVal)
		}
	}
	return cb
}

// SetUint8 sets a uint8 field if the form value is valid
func (cb *ConfigBuilder) SetUint8(target *uint8, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil && value >= 0 && value <= 255 {
		*target = uint8(value)
	}
	return cb
}

// SetStringArrayFromForm sets a string array field from PostFormArray
func (cb *ConfigBuilder) SetStringArrayFromForm(target *[]string, fieldName string) *ConfigBuilder {
	fieldKey := fmt.Sprintf("%s_%s_%s", cb.prefix, cb.index, fieldName)
	if values := cb.context.PostFormArray(fieldKey); len(values) > 0 {
		*target = filterStringArray(values)
	}
	return cb
}

// SetUint16 sets a uint16 field if the form value is valid
func (cb *ConfigBuilder) SetUint16(target *uint16, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil && value >= 0 && value <= 65535 {
		*target = uint16(value)
	}
	return cb
}

// SetIntArray sets an int array field by parsing comma-separated values
func (cb *ConfigBuilder) SetIntArray(target *[]int, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		parts := strings.Split(value, ",")
		var intArray []int
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				if intVal, err := strconv.Atoi(trimmed); err == nil {
					intArray = append(intArray, intVal)
				}
			}
		}
		if len(intArray) > 0 {
			*target = intArray
		}
	}
	return cb
}

// renderFormFields renders multiple form fields using FormFieldDefinition array
func renderFormFields(group string, comments map[string]string, displayNames map[string]string, fields []FormFieldDefinition) []Node {
	var formGroups []Node
	for _, field := range fields {
		formGroups = append(formGroups, renderFormGroup(group, comments, displayNames, field.Name, field.Type, field.Value, field.Options))
	}
	return formGroups
}

// createGeneralConfigFields creates field definitions for General config
func createGeneralConfigFields(configv *config.GeneralConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{Name: "TimeFormat", Type: "select", Value: configv.TimeFormat, Options: map[string][]string{
			"options": {"rfc3339", "iso8601", "rfc1123", "rfc822", "rfc850"},
		}},
		{Name: "TimeZone", Type: "text", Value: configv.TimeZone},
		{Name: "LogLevel", Type: "select", Value: configv.LogLevel, Options: map[string][]string{
			"options": {"info", "debug"},
		}},
		{Name: "DBLogLevel", Type: "select", Value: configv.DBLogLevel, Options: map[string][]string{
			"options": {"info", "debug"},
		}},
		{Name: "LogFileSize", Type: "number", Value: configv.LogFileSize},
		{Name: "LogFileCount", Type: "number", Value: configv.LogFileCount},
		{Name: "LogCompress", Type: "checkbox", Value: configv.LogCompress},
		{Name: "LogToFileOnly", Type: "checkbox", Value: configv.LogToFileOnly},
		{Name: "LogColorize", Type: "checkbox", Value: configv.LogColorize},
		{Name: "LogZeroValues", Type: "checkbox", Value: configv.LogZeroValues},
		{Name: "WorkerMetadata", Type: "number", Value: configv.WorkerMetadata},
		{Name: "WorkerFiles", Type: "number", Value: configv.WorkerFiles},
		{Name: "WorkerParse", Type: "number", Value: configv.WorkerParse},
		{Name: "WorkerSearch", Type: "number", Value: configv.WorkerSearch},
		{Name: "WorkerRSS", Type: "number", Value: configv.WorkerRSS},
		{Name: "WorkerIndexer", Type: "number", Value: configv.WorkerIndexer},
		{Name: "OmdbAPIKey", Type: "text", Value: configv.OmdbAPIKey},
		{Name: "UseMediaCache", Type: "checkbox", Value: configv.UseMediaCache},
		{Name: "UseFileCache", Type: "checkbox", Value: configv.UseFileCache},
		{Name: "UseHistoryCache", Type: "checkbox", Value: configv.UseHistoryCache},
		{Name: "CacheDuration", Type: "number", Value: configv.CacheDuration},
		{Name: "CacheAutoExtend", Type: "checkbox", Value: configv.CacheAutoExtend},
		{Name: "SearcherSize", Type: "number", Value: configv.SearcherSize},
		{Name: "MovieMetaSourceImdb", Type: "checkbox", Value: configv.MovieMetaSourceImdb},
		{Name: "MovieMetaSourceTmdb", Type: "checkbox", Value: configv.MovieMetaSourceTmdb},
		{Name: "MovieMetaSourceOmdb", Type: "checkbox", Value: configv.MovieMetaSourceOmdb},
		{Name: "MovieMetaSourceTrakt", Type: "checkbox", Value: configv.MovieMetaSourceTrakt},
		{Name: "MovieAlternateTitleMetaSourceImdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceImdb},
		{Name: "MovieAlternateTitleMetaSourceTmdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceTmdb},
		{Name: "MovieAlternateTitleMetaSourceOmdb", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceOmdb},
		{Name: "MovieAlternateTitleMetaSourceTrakt", Type: "checkbox", Value: configv.MovieAlternateTitleMetaSourceTrakt},
		{Name: "SerieAlternateTitleMetaSourceImdb", Type: "checkbox", Value: configv.SerieAlternateTitleMetaSourceImdb},
		{Name: "SerieAlternateTitleMetaSourceTrakt", Type: "checkbox", Value: configv.SerieAlternateTitleMetaSourceTrakt},
		{Name: "MovieMetaSourcePriority", Type: "array", Value: configv.MovieMetaSourcePriority},
		{Name: "MovieRSSMetaSourcePriority", Type: "array", Value: configv.MovieRSSMetaSourcePriority},
		{Name: "MovieParseMetaSourcePriority", Type: "array", Value: configv.MovieParseMetaSourcePriority},
		{Name: "SerieMetaSourceTmdb", Type: "checkbox", Value: configv.SerieMetaSourceTmdb},
		{Name: "SerieMetaSourceTrakt", Type: "checkbox", Value: configv.SerieMetaSourceTrakt},
		{Name: "MoveBufferSizeKB", Type: "number", Value: configv.MoveBufferSizeKB},
		{Name: "WebPort", Type: "text", Value: configv.WebPort},
		{Name: "WebAPIKey", Type: "text", Value: configv.WebAPIKey},
		{Name: "WebPortalEnabled", Type: "checkbox", Value: configv.WebPortalEnabled},
		{Name: "TheMovieDBApiKey", Type: "text", Value: configv.TheMovieDBApiKey},
		{Name: "TraktClientID", Type: "text", Value: configv.TraktClientID},
		{Name: "TraktClientSecret", Type: "text", Value: configv.TraktClientSecret},
		{Name: "TraktRedirectUrl", Type: "text", Value: configv.TraktRedirectUrl},
		{Name: "SchedulerDisabled", Type: "checkbox", Value: configv.SchedulerDisabled},
		{Name: "DisableParserStringMatch", Type: "checkbox", Value: configv.DisableParserStringMatch},
		{Name: "UseCronInsteadOfInterval", Type: "checkbox", Value: configv.UseCronInsteadOfInterval},
		{Name: "UseFileBufferCopy", Type: "checkbox", Value: configv.UseFileBufferCopy},
		{Name: "DisableSwagger", Type: "checkbox", Value: configv.DisableSwagger},
		{Name: "TraktLimiterSeconds", Type: "number", Value: configv.TraktLimiterSeconds},
		{Name: "TraktLimiterCalls", Type: "number", Value: configv.TraktLimiterCalls},
		{Name: "TvdbLimiterSeconds", Type: "number", Value: configv.TvdbLimiterSeconds},
		{Name: "TvdbLimiterCalls", Type: "number", Value: configv.TvdbLimiterCalls},
		{Name: "TmdbLimiterSeconds", Type: "number", Value: configv.TmdbLimiterSeconds},
		{Name: "TmdbLimiterCalls", Type: "number", Value: configv.TmdbLimiterCalls},
		{Name: "OmdbLimiterSeconds", Type: "number", Value: configv.OmdbLimiterSeconds},
		{Name: "OmdbLimiterCalls", Type: "number", Value: configv.OmdbLimiterCalls},
		{Name: "TheMovieDBDisableTLSVerify", Type: "checkbox", Value: configv.TheMovieDBDisableTLSVerify},
		{Name: "TraktDisableTLSVerify", Type: "checkbox", Value: configv.TraktDisableTLSVerify},
		{Name: "OmdbDisableTLSVerify", Type: "checkbox", Value: configv.OmdbDisableTLSVerify},
		{Name: "TvdbDisableTLSVerify", Type: "checkbox", Value: configv.TvdbDisableTLSVerify},
		{Name: "FfprobePath", Type: "text", Value: configv.FfprobePath},
		{Name: "MediainfoPath", Type: "text", Value: configv.MediainfoPath},
		{Name: "UseMediainfo", Type: "checkbox", Value: configv.UseMediainfo},
		{Name: "UseMediaFallback", Type: "checkbox", Value: configv.UseMediaFallback},
		{Name: "FailedIndexerBlockTime", Type: "number", Value: configv.FailedIndexerBlockTime},
		{Name: "MaxDatabaseBackups", Type: "number", Value: configv.MaxDatabaseBackups},
		{Name: "DatabaseBackupStopTasks", Type: "checkbox", Value: configv.DatabaseBackupStopTasks},
		{Name: "DisableVariableCleanup", Type: "checkbox", Value: configv.DisableVariableCleanup},
		{Name: "OmdbTimeoutSeconds", Type: "number", Value: configv.OmdbTimeoutSeconds},
		{Name: "TmdbTimeoutSeconds", Type: "number", Value: configv.TmdbTimeoutSeconds},
		{Name: "TvdbTimeoutSeconds", Type: "number", Value: configv.TvdbTimeoutSeconds},
		{Name: "TraktTimeoutSeconds", Type: "number", Value: configv.TraktTimeoutSeconds},
		{Name: "EnableFileWatcher", Type: "checkbox", Value: configv.EnableFileWatcher},
	}
}

func createImdbConfigFields(configv *config.ImdbConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{Name: "Indexedtypes", Type: "selectarray", Value: configv.Indexedtypes, Options: map[string][]string{
			"options": {"movie", "tvMovie", "tvmovie", "tvSeries", "tvseries", "video"},
		}},
		{Name: "Indexedlanguages", Type: "array", Value: configv.Indexedlanguages},
		{Name: "Indexfull", Type: "checkbox", Value: configv.Indexfull},
		{Name: "ImdbIDSize", Type: "number", Value: configv.ImdbIDSize},
		{Name: "LoopSize", Type: "number", Value: configv.LoopSize},
		{Name: "UseMemory", Type: "checkbox", Value: configv.UseMemory},
		{Name: "UseCache", Type: "checkbox", Value: configv.UseCache},
	}
}

func createSchedulerConfigFields(configv *config.SchedulerConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{Name: "Name", Type: "text", Value: configv.Name},
		{Name: "IntervalImdb", Type: "text", Value: configv.IntervalImdb},
		{Name: "IntervalFeeds", Type: "text", Value: configv.IntervalFeeds},
		{Name: "IntervalFeedsRefreshSeries", Type: "text", Value: configv.IntervalFeedsRefreshSeries},
		{Name: "IntervalFeedsRefreshMovies", Type: "text", Value: configv.IntervalFeedsRefreshMovies},
		{Name: "IntervalFeedsRefreshSeriesFull", Type: "text", Value: configv.IntervalFeedsRefreshSeriesFull},
		{Name: "IntervalFeedsRefreshMoviesFull", Type: "text", Value: configv.IntervalFeedsRefreshMoviesFull},
		{Name: "IntervalIndexerMissing", Type: "text", Value: configv.IntervalIndexerMissing},
		{Name: "IntervalIndexerUpgrade", Type: "text", Value: configv.IntervalIndexerUpgrade},
		{Name: "IntervalIndexerMissingFull", Type: "text", Value: configv.IntervalIndexerMissingFull},
		{Name: "IntervalIndexerUpgradeFull", Type: "text", Value: configv.IntervalIndexerUpgradeFull},
		{Name: "IntervalIndexerMissingTitle", Type: "text", Value: configv.IntervalIndexerMissingTitle},
		{Name: "IntervalIndexerUpgradeTitle", Type: "text", Value: configv.IntervalIndexerUpgradeTitle},
		{Name: "IntervalIndexerMissingFullTitle", Type: "text", Value: configv.IntervalIndexerMissingFullTitle},
		{Name: "IntervalIndexerUpgradeFullTitle", Type: "text", Value: configv.IntervalIndexerUpgradeFullTitle},
		{Name: "IntervalIndexerRss", Type: "text", Value: configv.IntervalIndexerRss},
		{Name: "IntervalScanData", Type: "text", Value: configv.IntervalScanData},
		{Name: "IntervalScanDataMissing", Type: "text", Value: configv.IntervalScanDataMissing},
		{Name: "IntervalScanDataFlags", Type: "text", Value: configv.IntervalScanDataFlags},
		{Name: "IntervalScanDataimport", Type: "text", Value: configv.IntervalScanDataimport},
		{Name: "IntervalDatabaseBackup", Type: "text", Value: configv.IntervalDatabaseBackup},
		{Name: "IntervalDatabaseCheck", Type: "text", Value: configv.IntervalDatabaseCheck},
		{Name: "IntervalIndexerRssSeasons", Type: "text", Value: configv.IntervalIndexerRssSeasons},
		{Name: "IntervalIndexerRssSeasonsAll", Type: "text", Value: configv.IntervalIndexerRssSeasonsAll},
		{Name: "CronIndexerRssSeasonsAll", Type: "text", Value: configv.CronIndexerRssSeasonsAll},
		{Name: "CronIndexerRssSeasons", Type: "text", Value: configv.CronIndexerRssSeasons},
		{Name: "CronImdb", Type: "text", Value: configv.CronImdb},
		{Name: "CronFeeds", Type: "text", Value: configv.CronFeeds},
		{Name: "CronFeedsRefreshSeries", Type: "text", Value: configv.CronFeedsRefreshSeries},
		{Name: "CronFeedsRefreshMovies", Type: "text", Value: configv.CronFeedsRefreshMovies},
		{Name: "CronFeedsRefreshSeriesFull", Type: "text", Value: configv.CronFeedsRefreshSeriesFull},
		{Name: "CronFeedsRefreshMoviesFull", Type: "text", Value: configv.CronFeedsRefreshMoviesFull},
		{Name: "CronIndexerMissing", Type: "text", Value: configv.CronIndexerMissing},
		{Name: "CronIndexerUpgrade", Type: "text", Value: configv.CronIndexerUpgrade},
		{Name: "CronIndexerMissingFull", Type: "text", Value: configv.CronIndexerMissingFull},
		{Name: "CronIndexerUpgradeFull", Type: "text", Value: configv.CronIndexerUpgradeFull},
		{Name: "CronIndexerMissingTitle", Type: "text", Value: configv.CronIndexerMissingTitle},
		{Name: "CronIndexerUpgradeTitle", Type: "text", Value: configv.CronIndexerUpgradeTitle},
		{Name: "CronIndexerMissingFullTitle", Type: "text", Value: configv.CronIndexerMissingFullTitle},
		{Name: "CronIndexerUpgradeFullTitle", Type: "text", Value: configv.CronIndexerUpgradeFullTitle},
		{Name: "CronIndexerRss", Type: "text", Value: configv.CronIndexerRss},
		{Name: "CronScanData", Type: "text", Value: configv.CronScanData},
		{Name: "CronScanDataMissing", Type: "text", Value: configv.CronScanDataMissing},
		{Name: "CronScanDataFlags", Type: "text", Value: configv.CronScanDataFlags},
		{Name: "CronScanDataimport", Type: "text", Value: configv.CronScanDataimport},
		{Name: "CronDatabaseBackup", Type: "text", Value: configv.CronDatabaseBackup},
		{Name: "CronDatabaseCheck", Type: "text", Value: configv.CronDatabaseCheck},
	}
}

func parseMediaTypeConfig[T any](c *gin.Context, mediaType, index string, builder *ConfigBuilder) config.MediaTypeConfig {
	var cfg config.MediaTypeConfig
	group := fmt.Sprintf("media_main_%s_%s", mediaType, index)

	builder.context = c
	builder.prefix = group
	builder.index = index

	builder.SetString(&cfg.Name, "Name").
		SetString(&cfg.DefaultQuality, "DefaultQuality").
		SetString(&cfg.DefaultResolution, "DefaultResolution").
		SetString(&cfg.Naming, "Naming").
		SetString(&cfg.TemplateQuality, "TemplateQuality").
		SetString(&cfg.TemplateScheduler, "TemplateScheduler").
		SetString(&cfg.MetadataLanguage, "MetadataLanguage").
		SetStringArray(&cfg.MetadataTitleLanguages, "MetadataTitleLanguages").
		SetBool(&cfg.Structure, "Structure").
		SetUint16(&cfg.SearchmissingIncremental, "SearchmissingIncremental").
		SetUint16(&cfg.SearchupgradeIncremental, "SearchupgradeIncremental")

	return cfg
}

func parseMediaDataConfigs(c *gin.Context, mediaType, index string) []config.MediaDataConfig {
	prefix := fmt.Sprintf("media_%s_%s_data", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplatePath") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaDataConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaDataConfig{TemplatePath: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_AddFound", prefix, subIndex)); val != "" {
			cfg.AddFound, _ = strconv.ParseBool(val)
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_AddFoundList", prefix, subIndex)); val != "" {
			cfg.AddFoundList = val
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaDataImportConfigs(c *gin.Context, mediaType, index string) []config.MediaDataImportConfig {
	prefix := fmt.Sprintf("media_%s_%s_dataimport", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplatePath") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaDataImportConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_TemplatePath", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		configs = append(configs, config.MediaDataImportConfig{TemplatePath: name})
	}
	return configs
}

func parseMediaListsConfigs(c *gin.Context, mediaType, index string) []config.MediaListsConfig {
	prefix := fmt.Sprintf("media_%s_%s_lists", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaListsConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_Name", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaListsConfig{Name: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateList", prefix, subIndex)); val != "" {
			cfg.TemplateList = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateQuality", prefix, subIndex)); val != "" {
			cfg.TemplateQuality = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_TemplateScheduler", prefix, subIndex)); val != "" {
			cfg.TemplateScheduler = val
		}
		if val := c.PostFormArray(fmt.Sprintf("%s_%s_IgnoreMapLists", prefix, subIndex)); len(val) != 0 {
			cfg.IgnoreMapLists = val
		}
		if val := c.PostFormArray(fmt.Sprintf("%s_%s_ReplaceMapLists", prefix, subIndex)); len(val) != 0 {
			cfg.ReplaceMapLists = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Enabled", prefix, subIndex)); val != "" {
			cfg.Enabled, _ = strconv.ParseBool(val)
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Addfound", prefix, subIndex)); val != "" {
			cfg.Addfound, _ = strconv.ParseBool(val)
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaNotificationConfigs(c *gin.Context, mediaType, index string) []config.MediaNotificationConfig {
	prefix := fmt.Sprintf("media_%s_%s_notification", mediaType, index)
	subformKeys := make(map[string]bool)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_MapNotification") || !strings.Contains(key, prefix+"_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[4]] = true
	}

	var configs []config.MediaNotificationConfig
	for subIndex := range subformKeys {
		nameField := fmt.Sprintf("%s_%s_MapNotification", prefix, subIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		cfg := config.MediaNotificationConfig{MapNotification: name}

		if val := c.PostForm(fmt.Sprintf("%s_%s_Event", prefix, subIndex)); val != "" {
			cfg.Event = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Title", prefix, subIndex)); val != "" {
			cfg.Title = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_Message", prefix, subIndex)); val != "" {
			cfg.Message = val
		}
		if val := c.PostForm(fmt.Sprintf("%s_%s_ReplacedPrefix", prefix, subIndex)); val != "" {
			cfg.ReplacedPrefix = val
		}

		configs = append(configs, cfg)
	}
	return configs
}

func parseMediaConfigs(c *gin.Context, mediaType string) []config.MediaTypeConfig {
	formKeys := make(map[string]bool)
	searchKey := fmt.Sprintf("media_main_%s_", mediaType)

	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, searchKey) {
			continue
		}
		formKeys[strings.Split(key, "_")[3]] = true
	}

	var configs []config.MediaTypeConfig
	var builder ConfigBuilder

	for index := range formKeys {
		cfg := parseMediaTypeConfig[config.MediaTypeConfig](c, mediaType, index, &builder)
		if cfg.Name == "" {
			continue
		}

		cfg.Data = parseMediaDataConfigs(c, mediaType, index)
		cfg.DataImport = parseMediaDataImportConfigs(c, mediaType, index)
		cfg.Lists = parseMediaListsConfigs(c, mediaType, index)
		cfg.Notification = parseMediaNotificationConfigs(c, mediaType, index)

		configs = append(configs, cfg)
	}
	return configs
}

func parseGeneralConfig(c *gin.Context) config.GeneralConfig {
	updatedConfig := config.GetToml().General

	builder := &ConfigBuilder{context: c, prefix: "general"}
	builder.SetString(&updatedConfig.TimeFormat, "TimeFormat").
		SetString(&updatedConfig.TimeZone, "TimeZone").
		SetString(&updatedConfig.LogLevel, "LogLevel").
		SetString(&updatedConfig.DBLogLevel, "DBLogLevel").
		SetInt(&updatedConfig.LogFileSize, "LogFileSize").
		SetUint8(&updatedConfig.LogFileCount, "LogFileCount").
		SetInt(&updatedConfig.WorkerMetadata, "WorkerMetadata").
		SetInt(&updatedConfig.WorkerFiles, "WorkerFiles").
		SetString(&updatedConfig.WebPort, "WebPort").
		SetBool(&updatedConfig.DisableVariableCleanup, "DisableVariableCleanup").
		SetBool(&updatedConfig.DisableParserStringMatch, "DisableParserStringMatch").
		SetBool(&updatedConfig.UseMediaCache, "UseMediaCache").
		SetInt(&updatedConfig.CacheDuration, "CacheDuration").
		SetBool(&updatedConfig.DisableSwagger, "DisableSwagger").
		SetString(&updatedConfig.WebAPIKey, "WebAPIKey").
		SetBool(&updatedConfig.LogCompress, "LogCompress").
		SetBool(&updatedConfig.LogToFileOnly, "LogToFileOnly").
		SetBool(&updatedConfig.LogColorize, "LogColorize").
		SetBool(&updatedConfig.LogZeroValues, "LogZeroValues").
		SetInt(&updatedConfig.WorkerParse, "WorkerParse").
		SetInt(&updatedConfig.WorkerSearch, "WorkerSearch").
		SetInt(&updatedConfig.WorkerIndexer, "WorkerIndexer").
		SetBool(&updatedConfig.WebPortalEnabled, "WebPortalEnabled").
		SetString(&updatedConfig.TheMovieDBApiKey, "TheMovieDBApiKey").
		SetString(&updatedConfig.TraktClientID, "TraktClientID").
		SetString(&updatedConfig.TraktClientSecret, "TraktClientSecret").
		SetString(&updatedConfig.TraktRedirectUrl, "TraktRedirectUrl").
		SetBool(&updatedConfig.EnableFileWatcher, "EnableFileWatcher").
		SetUint16(&updatedConfig.TraktTimeoutSeconds, "TraktTimeoutSeconds").
		SetUint16(&updatedConfig.TvdbTimeoutSeconds, "TvdbTimeoutSeconds").
		SetUint16(&updatedConfig.TmdbTimeoutSeconds, "TmdbTimeoutSeconds").
		SetUint16(&updatedConfig.OmdbTimeoutSeconds, "OmdbTimeoutSeconds").
		SetBool(&updatedConfig.DatabaseBackupStopTasks, "DatabaseBackupStopTasks").
		SetInt(&updatedConfig.MaxDatabaseBackups, "MaxDatabaseBackups").
		SetInt(&updatedConfig.FailedIndexerBlockTime, "FailedIndexerBlockTime").
		SetBool(&updatedConfig.UseMediaFallback, "UseMediaFallback").
		SetBool(&updatedConfig.UseMediainfo, "UseMediainfo").
		SetString(&updatedConfig.MediainfoPath, "MediainfoPath").
		SetString(&updatedConfig.FfprobePath, "FfprobePath").
		SetBool(&updatedConfig.TvdbDisableTLSVerify, "TvdbDisableTLSVerify").
		SetBool(&updatedConfig.OmdbDisableTLSVerify, "OmdbDisableTLSVerify").
		SetBool(&updatedConfig.TraktDisableTLSVerify, "TraktDisableTLSVerify").
		SetBool(&updatedConfig.TheMovieDBDisableTLSVerify, "TheMovieDBDisableTLSVerify").
		SetInt(&updatedConfig.OmdbLimiterCalls, "OmdbLimiterCalls").
		SetUint8(&updatedConfig.OmdbLimiterSeconds, "OmdbLimiterSeconds").
		SetInt(&updatedConfig.TmdbLimiterCalls, "TmdbLimiterCalls").
		SetUint8(&updatedConfig.TmdbLimiterSeconds, "TmdbLimiterSeconds").
		SetInt(&updatedConfig.TvdbLimiterCalls, "TvdbLimiterCalls").
		SetUint8(&updatedConfig.TvdbLimiterSeconds, "TvdbLimiterSeconds").
		SetInt(&updatedConfig.TraktLimiterCalls, "TraktLimiterCalls").
		SetUint8(&updatedConfig.TraktLimiterSeconds, "TraktLimiterSeconds").
		SetBool(&updatedConfig.UseFileBufferCopy, "UseFileBufferCopy").
		SetBool(&updatedConfig.UseCronInsteadOfInterval, "UseCronInsteadOfInterval").
		SetBool(&updatedConfig.SchedulerDisabled, "SchedulerDisabled").
		SetInt(&updatedConfig.MoveBufferSizeKB, "MoveBufferSizeKB").
		SetBool(&updatedConfig.SerieMetaSourceTrakt, "SerieMetaSourceTrakt").
		SetBool(&updatedConfig.SerieMetaSourceTmdb, "SerieMetaSourceTmdb").
		SetStringArray(&updatedConfig.MovieParseMetaSourcePriority, "MovieParseMetaSourcePriority").
		SetStringArray(&updatedConfig.MovieRSSMetaSourcePriority, "MovieRSSMetaSourcePriority").
		SetStringArray(&updatedConfig.MovieMetaSourcePriority, "MovieMetaSourcePriority").
		SetBool(&updatedConfig.SerieAlternateTitleMetaSourceTrakt, "SerieAlternateTitleMetaSourceTrakt").
		SetBool(&updatedConfig.SerieAlternateTitleMetaSourceImdb, "SerieAlternateTitleMetaSourceImdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceTrakt, "MovieAlternateTitleMetaSourceTrakt").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceOmdb, "MovieAlternateTitleMetaSourceOmdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceTmdb, "MovieAlternateTitleMetaSourceTmdb").
		SetBool(&updatedConfig.MovieAlternateTitleMetaSourceImdb, "MovieAlternateTitleMetaSourceImdb").
		SetBool(&updatedConfig.MovieMetaSourceTrakt, "MovieMetaSourceTrakt").
		SetBool(&updatedConfig.MovieMetaSourceOmdb, "MovieMetaSourceOmdb").
		SetBool(&updatedConfig.MovieMetaSourceTmdb, "MovieMetaSourceTmdb").
		SetBool(&updatedConfig.MovieMetaSourceImdb, "MovieMetaSourceImdb").
		SetInt(&updatedConfig.SearcherSize, "SearcherSize").
		SetBool(&updatedConfig.CacheAutoExtend, "CacheAutoExtend").
		SetBool(&updatedConfig.UseHistoryCache, "UseHistoryCache").
		SetBool(&updatedConfig.UseFileCache, "UseFileCache").
		SetString(&updatedConfig.OmdbAPIKey, "OmdbAPIKey").
		SetInt(&updatedConfig.WorkerRSS, "WorkerRSS")

	return updatedConfig
}

// renderTestParsePage renders a page for testing string parsing
func renderTestParsePage(csrfToken string) Node {
	media := config.GetSettingsMediaAll()
	lists := make([]string, 0, len(media.Movies)+len(media.Series))
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}
	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
	}
	qualitycfg := config.GetSettingsQualityAll()
	qualities := make([]string, 0, len(qualitycfg))
	for i := range qualitycfg {
		qualities = append(qualities, qualitycfg[i].Name)
	}
	return Div(
		Class("config-section"),
		H3(Text("String Parse Test")),
		P(Text("Test the file parser with different filename patterns. This tool helps you understand how the parser extracts metadata from filenames.")),

		Form(
			Class("config-form"),
			ID("parseTestForm"),

			renderFormGroup("testparse", map[string]string{
				"Filename": "Enter a filename to parse (e.g., 'The.Matrix.1999.1080p.BluRay.x264-RARBG')",
			}, map[string]string{
				"Filename": "Filename",
			}, "Filename", "text", "", nil),

			renderFormGroup("testparse", map[string]string{
				"ConfigKey": "Select the media configuration to use for parsing",
			}, map[string]string{
				"ConfigKey": "Media Config",
			}, "ConfigKey", "select", "", map[string][]string{
				"options": lists,
			}),

			renderFormGroup("testparse", map[string]string{
				"QualityKey": "Select the quality configuration to use",
			}, map[string]string{
				"QualityKey": "Quality Config",
			}, "QualityKey", "select", "", map[string][]string{
				"options": qualities,
			}),

			renderFormGroup("testparse", map[string]string{
				"UsePath": "Parse as full path instead of just filename",
			}, map[string]string{
				"UsePath": "Use Path",
			}, "UsePath", "checkbox", false, nil),

			renderFormGroup("testparse", map[string]string{
				"UseFolder": "Use folder information in parsing",
			}, map[string]string{
				"UseFolder": "Use Folder",
			}, "UseFolder", "checkbox", false, nil),

			Div(
				Class("form-group submit-group"),
				Button(
					Class(ClassBtnPrimary),
					Text("Parse"),
					Type("button"),
					hx.Target("#parseResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/testparse"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#parseTestForm"),
				),
				createButton("Reset", "button", "btn btn-secondary ml-2",
					Attr("onclick", "document.getElementById('parseTestForm').reset(); document.getElementById('parseResults').innerHTML = '';")),
			),
		),

		Div(
			ID("parseResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),
	)
}

// HandleTestParse handles test parsing requests
func HandleTestParse(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	filename := c.PostForm("testparse_Filename")
	configKey := c.PostForm("testparse_ConfigKey")
	qualityKey := c.PostForm("testparse_QualityKey")
	usePath, _ := strconv.ParseBool(c.PostForm("testparse_UsePath"))
	useFolder, _ := strconv.ParseBool(c.PostForm("testparse_UseFolder"))

	if filename == "" {
		c.String(http.StatusOK, renderAlert("Please enter a filename to parse", "warning"))
		return
	}

	// Get configuration objects
	cfgp := config.GetSettingsMedia(configKey)
	if cfgp == nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Config key %s not found", configKey), "danger"))
		return
	}

	quality := config.GetSettingsQuality(qualityKey)
	if quality == nil {
		c.String(http.StatusOK, renderAlert(fmt.Sprintf("Quality key %s not found", qualityKey), "danger"))
		return
	}

	// Parse the file
	m := parser.ParseFile(filename, usePath, useFolder, cfgp, -1)
	if m == nil {
		c.String(http.StatusOK, renderAlert("ParseFile returned nil - parsing failed", "danger"))
		return
	}

	// Get database IDs and quality mapping
	parser.GetDBIDs(m, cfgp, true)
	parser.GetPriorityMapQual(m, cfgp, quality, false, true)

	// Render results
	c.String(http.StatusOK, renderParseResults(m, filename, configKey, qualityKey))
}

// renderParseResults renders the parsing results in a formatted table
func renderParseResults(m *database.ParseInfo, originalFilename, configKey, qualityKey string) string {
	resultRows := []Node{
		// Header information
		Tr(Td(Strong(Text("Original Filename:"))), Td(Text(originalFilename))),
		Tr(Td(Strong(Text("Config Used:"))), Td(Text(configKey))),
		Tr(Td(Strong(Text("Quality Used:"))), Td(Text(qualityKey))),
		Tr(Td(Attr("colspan", "2"), Hr())),

		// Basic media information
		Tr(Td(Strong(Text("Title:"))), Td(Text(m.Title))),
		Tr(Td(Strong(Text("File:"))), Td(Text(m.File))),
		Tr(Td(Strong(Text("Year:"))), Td(Text(fmt.Sprintf("%d", m.Year)))),
		Tr(Td(Strong(Text("Date:"))), Td(Text(m.Date))),
		Tr(Td(Strong(Text("Identifier:"))), Td(Text(m.Identifier))),

		// Episode/Season information
		Tr(Td(Strong(Text("Season:"))), Td(Text(fmt.Sprintf("%d", m.Season)))),
		Tr(Td(Strong(Text("Season String:"))), Td(Text(m.SeasonStr))),
		Tr(Td(Strong(Text("Episode:"))), Td(Text(fmt.Sprintf("%d", m.Episode)))),
		Tr(Td(Strong(Text("Episode String:"))), Td(Text(m.EpisodeStr))),

		// Quality and technical information
		Tr(Td(Strong(Text("Quality:"))), Td(Text(m.Quality))),
		Tr(Td(Strong(Text("Resolution:"))), Td(Text(m.Resolution))),
		Tr(Td(Strong(Text("Codec:"))), Td(Text(m.Codec))),
		Tr(Td(Strong(Text("Audio:"))), Td(Text(m.Audio))),
		Tr(Td(Strong(Text("Width x Height:"))), Td(Text(fmt.Sprintf("%d x %d", m.Width, m.Height)))),
		Tr(Td(Strong(Text("Runtime:"))), Td(Text(fmt.Sprintf("%d min", m.Runtime)))),
		Tr(Td(Strong(Text("Languages:"))), Td(Text(strings.Join(m.Languages, ", ")))),

		// Flags and properties
		Tr(Td(Strong(Text("Extended:"))), Td(Text(fmt.Sprintf("%t", m.Extended)))),
		Tr(Td(Strong(Text("Proper:"))), Td(Text(fmt.Sprintf("%t", m.Proper)))),
		Tr(Td(Strong(Text("Repack:"))), Td(Text(fmt.Sprintf("%t", m.Repack)))),
		Tr(Td(Strong(Text("Priority:"))), Td(Text(fmt.Sprintf("%d", m.Priority)))),

		// External IDs
		Tr(Td(Strong(Text("IMDB ID:"))), Td(Text(m.Imdb))),
		Tr(Td(Strong(Text("TVDB ID:"))), Td(Text(m.Tvdb))),

		// Internal information
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("List ID:"))), Td(Text(fmt.Sprintf("%d", m.ListID)))),
		Tr(Td(Strong(Text("First IDX:"))), Td(Text(fmt.Sprintf("%d", m.FirstIDX)))),
		Tr(Td(Strong(Text("First Year IDX:"))), Td(Text(fmt.Sprintf("%d", m.FirstYearIDX)))),
		Tr(Td(Strong(Text("Temp ID:"))), Td(Text(fmt.Sprintf("%d", m.TempID)))),

		// Database IDs
		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Quality ID:"))), Td(Text(fmt.Sprintf("%d", m.QualityID)))),
		Tr(Td(Strong(Text("Resolution ID:"))), Td(Text(fmt.Sprintf("%d", m.ResolutionID)))),
		Tr(Td(Strong(Text("Codec ID:"))), Td(Text(fmt.Sprintf("%d", m.CodecID)))),
		Tr(Td(Strong(Text("Audio ID:"))), Td(Text(fmt.Sprintf("%d", m.AudioID)))),

		// Movie IDs
		Tr(Td(Strong(Text("DB Movie ID:"))), Td(Text(fmt.Sprintf("%d", m.DbmovieID)))),
		Tr(Td(Strong(Text("Movie ID:"))), Td(Text(fmt.Sprintf("%d", m.MovieID)))),

		// Series IDs
		Tr(Td(Strong(Text("DB Serie ID:"))), Td(Text(fmt.Sprintf("%d", m.DbserieID)))),
		Tr(Td(Strong(Text("DB Serie Episode ID:"))), Td(Text(fmt.Sprintf("%d", m.DbserieEpisodeID)))),
		Tr(Td(Strong(Text("Serie ID:"))), Td(Text(fmt.Sprintf("%d", m.SerieID)))),
		Tr(Td(Strong(Text("Serie Episode ID:"))), Td(Text(fmt.Sprintf("%d", m.SerieEpisodeID)))),

		// Internal strings (if not empty)
		func() Node {
			if m.Str != "" {
				return Tr(Td(Strong(Text("Internal String:"))), Td(Text(m.Str)))
			}
			return Text("")
		}(),
		func() Node {
			if m.RuntimeStr != "" {
				return Tr(Td(Strong(Text("Runtime String:"))), Td(Text(m.RuntimeStr)))
			}
			return Text("")
		}(),
		func() Node {
			if m.TempTitle != "" {
				return Tr(Td(Strong(Text("Temp Title:"))), Td(Text(m.TempTitle)))
			}
			return Text("")
		}(),
	}

	results := Div(
		Class("alert alert-success"),
		H5(Text("Parse Results")),
		Table(
			Class("table table-striped table-sm"),
			TBody(Group(resultRows)),
		),
	)

	return renderComponentToString(results)
}

// renderComponentToString renders a gomponents Node to a string using pooled string builder
func renderComponentToString(node Node) string {
	sb := getStringBuilder()
	defer putStringBuilder(sb)
	node.Render(sb)
	return sb.String()
}

// renderTraktAuthPage renders a page for Trakt authentication
func renderTraktAuthPage(csrfToken string) Node {
	// Check current token status
	token := apiexternal.GetTraktToken()
	hasValidToken := token != nil && token.AccessToken != ""

	return Div(
		Class("config-section"),
		H3(Text("Trakt Authentication")),
		P(Text("Authenticate with Trakt.tv to enable synchronized watchlists, ratings, and recommendations. This uses OAuth2 for secure authentication.")),

		// Current status display
		Div(
			func() Node {
				if hasValidToken {
					return Class("alert alert-success")
				}
				return Class("alert alert-warning")
			}(),
			H5(Text("Current Authentication Status:")),
			P(func() Node {
				if hasValidToken {
					return Text("✓ Authenticated - Trakt API access is enabled")
				}
				return Text("✗ Not authenticated - Trakt API access is disabled")
			}()),
			func() Node {
				if hasValidToken {
					return Div(
						P(Text("Token Details:")),
						Ul(
							Li(Text("Access Token: "+token.AccessToken[:20]+"...")),
							Li(Text("Token Type: "+token.TokenType)),
							Li(Text("Expiry: "+func() string {
								if token.Expiry.IsZero() {
									return "Never"
								}
								return token.Expiry.Format("2006-01-02 15:04:05")
							}())),
						),
					)
				}
				return Text("")
			}(),
		),

		// Authentication form
		Form(
			Class("config-form"),
			ID("traktAuthForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Text("Step 1: Get Authorization URL")),
					P(Text("Click the button below to generate an authorization URL. This will open Trakt.tv in a new tab where you can authorize this application.")),
					Br(),
					Button(
						Class("btn btn-info"),
						Text("Get Trakt Authorization URL"),
						Type("button"),
						hx.Target("#authUrlResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"get_url\"}"),
					),
					Div(
						ID("authUrlResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				),

				Div(
					Class("col-md-6"),
					H5(Text("Step 2: Enter Authorization Code")),
					P(Text("After authorizing on Trakt.tv, you'll receive a code. Enter it below to complete the authentication process.")),

					renderFormGroup("trakt", map[string]string{
						"AuthCode": "Enter the authorization code from Trakt.tv",
					}, map[string]string{
						"AuthCode": "Authorization Code",
					}, "AuthCode", "text", "", nil),

					Button(
						Class(ClassBtnSuccess),
						Text("Store Trakt Token"),
						Type("button"),
						hx.Target("#tokenResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Include("#traktAuthForm"),
						hx.Vals("{\"action\": \"store_token\"}"),
					),
					Div(
						ID("tokenResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				),
			),
		),

		// Token management section
		func() Node {
			if hasValidToken {
				return Div(
					Class("mt-4"),
					H5(Text("Token Management")),
					Div(
						Class("btn-group"),
						Button(
							Class("btn btn-warning"),
							Text("Refresh Token"),
							Type("button"),
							hx.Target("#refreshResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"refresh_token\"}"),
						),
						Button(
							Class("btn btn-danger ml-2"),
							Text("Revoke Token"),
							Type("button"),
							hx.Target("#revokeResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"revoke_token\"}"),
							Attr("onclick", "return confirm('Are you sure you want to revoke the Trakt authentication? This will disable Trakt API access.')"),
						),
					),
					Div(
						ID("refreshResult"),
						Class("mt-3"),
					),
					Div(
						ID("revokeResult"),
						Class("mt-3"),
					),
				)
			}
			return Text("")
		}(),

		// API Test section
		func() Node {
			if hasValidToken {
				return Div(
					Class("mt-4"),
					H5(Text("API Test")),
					P(Text("Test the Trakt API connection by fetching popular movies:")),
					Button(
						Class(ClassBtnSecondary),
						Text("Test Trakt API"),
						Type("button"),
						hx.Target("#apiTestResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"test_api\"}"),
					),
					Div(
						ID("apiTestResult"),
						Class("mt-3"),
						Style("min-height: 30px;"),
					),
				)
			}
			return Text("")
		}(),

		// Instructions
		Div(
			Class("mt-4 alert alert-info"),
			H5(Text("Setup Instructions:")),
			Ol(
				Li(Text("Make sure your Trakt Client ID and Client Secret are configured in the General settings")),
				Li(Text("Click 'Get Trakt Authorization URL' to generate the authorization link")),
				Li(Text("Visit the generated URL and authorize this application on Trakt.tv")),
				Li(Text("Copy the authorization code from the redirect URL")),
				Li(Text("Paste the code in the form above and click 'Store Trakt Token'")),
				Li(Text("Your Trakt authentication is now complete and will be stored securely")),
			),
		),
	)
}

// HandleTraktAuth handles Trakt authentication requests
func HandleTraktAuth(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	action := c.PostForm("action")
	if action == "" {
		// Try to get from JSON body
		var reqData map[string]string
		if err := c.ShouldBindJSON(&reqData); err == nil {
			action = reqData["action"]
		}
	}

	switch action {
	case "get_url":
		handleGetTraktAuthURL(c)
	case "store_token":
		handleStoreTraktToken(c)
	case "refresh_token":
		handleRefreshTraktToken(c)
	case "revoke_token":
		handleRevokeTraktToken(c)
	case "test_api":
		handleTestTraktAPI(c)
	default:
		c.String(http.StatusOK, renderAlert("Invalid action specified", "danger"))
	}
}

// handleGetTraktAuthURL generates and returns the Trakt authorization URL
func handleGetTraktAuthURL(c *gin.Context) {
	// Check if Trakt is configured
	generalConfig := config.GetSettingsGeneral()
	if generalConfig.TraktClientID == "" || generalConfig.TraktClientSecret == "" {
		c.String(http.StatusOK, renderAlert("Trakt Client ID and Secret must be configured in General settings first", "danger"))
		return
	}

	// Get the authorization URL
	authURL := apiexternal.GetTraktAuthURL()
	if authURL == "" {
		c.String(http.StatusOK, renderAlert("Failed to generate authorization URL", "danger"))
		return
	}

	result := Div(
		Class("alert alert-info"),
		H5(Text("Authorization URL Generated")),
		P(Text("Click the link below to authorize this application on Trakt.tv:")),
		P(
			A(
				Href(authURL),
				Target("_blank"),
				Class(ClassBtnPrimary),
				Text("Open Trakt Authorization Page"),
			),
		),
		Div(
			Class("mt-3"),
			Label(Text("Or copy this URL manually:")),
			Textarea(
				Class(ClassFormControl),
				Attr("readonly", "true"),
				Attr("rows", "3"),
				Text(authURL),
			),
		),
		P(
			Class("mt-2 text-muted"),
			Small(Text("After authorization, copy the code from the redirect URL and use it in Step 2.")),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleStoreTraktToken stores the Trakt token using the authorization code
func handleStoreTraktToken(c *gin.Context) {
	authCode := c.PostForm("trakt_AuthCode")
	if authCode == "" {
		c.String(http.StatusOK, renderAlert("Please enter the authorization code", "warning"))
		return
	}

	// Exchange code for token
	token := apiexternal.GetTraktAuthToken(authCode)
	if token == nil || token.AccessToken == "" {
		c.String(http.StatusOK, renderAlert("Failed to exchange authorization code for token. Please check the code and try again.", "danger"))
		return
	}

	// Store the token
	apiexternal.SetTraktToken(token)
	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})

	result := Div(
		Class("alert alert-success"),
		H5(Text("Trakt Authentication Successful!")),
		P(Text("Your Trakt token has been stored successfully. The application now has access to your Trakt account.")),
		Ul(
			Li(Text("Access Token: "+token.AccessToken[:20]+"...")),
			Li(Text("Token Type: "+token.TokenType)),
			Li(Text("Expiry: "+func() string {
				if token.Expiry.IsZero() {
					return "Never"
				}
				return token.Expiry.Format("2006-01-02 15:04:05")
			}())),
		),
		P(
			Class("mt-2"),
			Button(
				Class("btn btn-info"),
				Text("Reload Page"),
				Attr("onclick", "window.location.reload()"),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleRefreshTraktToken refreshes the current Trakt token
func handleRefreshTraktToken(c *gin.Context) {
	currentToken := apiexternal.GetTraktToken()
	if currentToken == nil || currentToken.RefreshToken == "" {
		c.String(http.StatusOK, renderAlert("No refresh token available. Please re-authenticate.", "danger"))
		return
	}

	// Note: Trakt API token refresh would need to be implemented in the apiexternal package
	// For now, we'll just indicate that refresh is not yet implemented
	c.String(http.StatusOK, renderAlert("Token refresh functionality is not yet implemented. Please re-authenticate if needed.", "info"))
}

// handleRevokeTraktToken revokes the current Trakt token
func handleRevokeTraktToken(c *gin.Context) {
	// Clear the token
	apiexternal.SetTraktToken(&oauth2.Token{})
	config.UpdateCfgEntry(config.Conf{Name: "trakt_token", Data: apiexternal.GetTraktToken()})

	result := Div(
		Class("alert alert-success"),
		H5(Text("Trakt Token Revoked")),
		P(Text("Your Trakt authentication has been revoked successfully. The application no longer has access to your Trakt account.")),
		P(
			Class("mt-2"),
			Button(
				Class("btn btn-info"),
				Text("Reload Page"),
				Attr("onclick", "window.location.reload()"),
			),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}

// handleTestTraktAPI tests the Trakt API connection
func handleTestTraktAPI(c *gin.Context) {
	token := apiexternal.GetTraktToken()
	if token == nil || token.AccessToken == "" {
		c.String(http.StatusOK, renderAlert("No valid Trakt token available", "danger"))
		return
	}

	// Test the API by getting popular movies (limit to 5 for testing)
	limit := "5"
	movies := apiexternal.GetTraktMoviePopular(&limit)

	if len(movies) == 0 {
		c.String(http.StatusOK, renderAlert("API test failed - no movies returned. Check your network connection and token validity.", "danger"))
		return
	}

	var movieRows []Node
	for i, movie := range movies {
		movieRows = append(movieRows,
			Tr(
				Td(Text(fmt.Sprintf("%d", i+1))),
				Td(Text(movie.Title)),
				Td(Text(fmt.Sprintf("%d", movie.Year))),
				Td(Text(fmt.Sprintf("tt%s", movie.IDs.Imdb))),
				Td(Text(fmt.Sprintf("%d", movie.IDs.Trakt))),
			),
		)
	}

	result := Div(
		Class("alert alert-success"),
		H5(Text("Trakt API Test Successful!")),
		P(Text(fmt.Sprintf("Successfully retrieved %d popular movies from Trakt:", len(movies)))),
		Table(
			Class("table table-striped table-sm"),
			THead(
				Tr(
					Th(Text("#")),
					Th(Text("Title")),
					Th(Text("Year")),
					Th(Text("IMDB ID")),
					Th(Text("Trakt ID")),
				),
			),
			TBody(Group(movieRows)),
		),
	)

	c.String(http.StatusOK, renderComponentToString(result))
}
