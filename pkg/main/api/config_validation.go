package api

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

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

// validateConfigStructure validates a slice of configs using common patterns
func validateConfigStructure[T any](configs []T, configType string, validator func(T) error) error {
	if len(configs) == 0 {
		return nil // Empty slice is valid
	}

	// Track names for uniqueness check
	nameMap := make(map[string]int)

	for i, config := range configs {
		// Apply specific validator
		if err := validator(config); err != nil {
			return fmt.Errorf("%s config %d: %v", configType, i, err)
		}

		// Generic name uniqueness check using reflection
		configValue := reflect.ValueOf(config)
		if configValue.Kind() == reflect.Ptr {
			configValue = configValue.Elem()
		}

		if configValue.Kind() == reflect.Struct {
			nameField := configValue.FieldByName("Name")
			if nameField.IsValid() && nameField.Kind() == reflect.String {
				name := nameField.String()
				if name != "" {
					if prevIndex, exists := nameMap[name]; exists {
						return fmt.Errorf("%s config %d: duplicate name '%s' (already used in config %d)", configType, i, name, prevIndex)
					}
					nameMap[name] = i
				}
			}
		}
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

// Batch validation helper
func validateBatch[T any](validator *ConfigValidator[T], configs []T) error {
	return validator.ValidateAll(configs)
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

// validateRange checks if a numeric value is within a valid range
func validateRange(value int, min int, max int, fieldName string) error {
	if value < min || value > max {
		return fmt.Errorf("invalid %s: %d (must be between %d and %d)", fieldName, value, min, max)
	}
	return nil
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

// Example validation using the example validator with forbidden values
func ValidateListsConfigWithExampleValidator(configs []config.ListsConfig) error {
	exampleValidator := createExampleValidatorWithForbiddenValues()
	return exampleValidator.ValidateAll(configs)
}

// validateDownloaderConfig validates downloader configuration using generic validator
func validateDownloaderConfig(configs []config.DownloaderConfig) error {
	return validateBatch(downloaderValidator, configs)
}

// validateListsConfig validates lists configuration using generic validator
func validateListsConfig(configs []config.ListsConfig) error {
	return validateBatch(listsValidator, configs)
}

// validateIndexersConfig validates indexers configuration using generic validator
func validateIndexersConfig(configs []config.IndexersConfig) error {
	return validateBatch(indexerValidator, configs)
}

// validatePathsConfig validates paths configuration using generic validator
func validatePathsConfig(configs []config.PathsConfig) error {
	return validateBatch(pathsValidator, configs)
}

// validateNotificationConfig validates notification configuration using generic validator
func validateNotificationConfig(configs []config.NotificationConfig) error {
	return validateBatch(notificationValidator, configs)
}

// validateRegexConfig validates regex configuration using generic validator
func validateRegexConfig(configs []config.RegexConfig) error {
	return validateBatch(regexValidator, configs)
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

// validateSchedulerConfig validates scheduler configuration
func validateSchedulerConfig(configs []config.SchedulerConfig) error {
	return validateBatch(schedulerValidator, configs)
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

// Common validator factory functions
func requireNonEmptyString[T any](fieldName string, getValue func(T) string) func(T) error {
	return func(config T) error {
		value := getValue(config)
		return validateRequiredField(value, fieldName)
	}
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
