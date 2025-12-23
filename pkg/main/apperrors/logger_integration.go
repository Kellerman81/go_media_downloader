package apperrors

// LoggerEvent represents a minimal interface for logging events
// This is compatible with zerolog.Event and logger.Event interfaces.
type LoggerEvent interface {
	Err(err error) LoggerEvent
	Str(key, val string) LoggerEvent
	Bool(key string, b bool) LoggerEvent
	Interface(key string, i any) LoggerEvent
}

// LogClassifiedError enhances a logger event with error classification metadata
// This function extracts classification information from a ClassifiedError and
// adds it to the log event for better observability.
func LogClassifiedError(event LoggerEvent, err error) LoggerEvent {
	if err == nil {
		return event
	}

	// Add the base error
	event = event.Err(err)

	// Extract classification metadata
	class := GetClass(err)
	operation := GetOperation(err)
	context := GetContext(err)

	// Add classification fields
	event = event.
		Str("error_class", string(class))

	// Add operation if available
	if operation != "" {
		event = event.Str("operation", operation)
	}

	// Add context if available
	if len(context) > 0 {
		event = event.Interface("error_context", context)
	}

	return event
}

// WithClassification is a convenience function that wraps LogClassifiedError
// for use in logging chains.
func WithClassification(err error) func(LoggerEvent) LoggerEvent {
	return func(event LoggerEvent) LoggerEvent {
		return LogClassifiedError(event, err)
	}
}
