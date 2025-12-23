package apperrors

import (
	"errors"
)

// Wrap creates a classified error.
func Wrap(class ErrorClass, operation string, err error) *ClassifiedError {
	if err == nil {
		return nil
	}

	return &ClassifiedError{
		Class:     class,
		Operation: operation,
		Err:       err,
		Context:   make(map[string]any),
	}
}

// WithContext adds context to a classified error.
func (e *ClassifiedError) WithContext(key string, value any) *ClassifiedError {
	if e == nil {
		return nil
	}

	e.Context[key] = value

	return e
}

// GetClass extracts the error class from an error.
func GetClass(err error) ErrorClass {
	if err == nil {
		return ErrClassUnknown
	}

	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Class
	}

	return ErrClassUnknown
}

// GetOperation extracts the operation from an error.
func GetOperation(err error) string {
	if err == nil {
		return ""
	}

	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Operation
	}

	return ""
}

// GetContext extracts context from an error.
func GetContext(err error) map[string]any {
	if err == nil {
		return nil
	}

	var ce *ClassifiedError
	if errors.As(err, &ce) {
		return ce.Context
	}

	return nil
}

// New creates a new classified error with a message.
func New(class ErrorClass, operation string, message string) *ClassifiedError {
	return &ClassifiedError{
		Class:     class,
		Operation: operation,
		Err:       errors.New(message),
		Context:   make(map[string]any),
	}
}

// WrapWithContext creates a classified error and automatically adds context fields.
// This is a convenience function that wraps an error and adds context in one call.
//
// Example usage:
//
//	err := apperrors.WrapWithContext(
//	    apperrors.ErrClassFileSystem,
//	    "media_processing",
//	    cause,
//	    map[string]any{
//	        "file_path": "/path/to/file.mkv",
//	        "media_type": "movie",
//	        "list_name": "watchlist",
//	    },
//	)
func WrapWithContext(
	class ErrorClass,
	operation string,
	err error,
	context map[string]any,
) *ClassifiedError {
	if err == nil {
		return nil
	}

	classified := Wrap(class, operation, err)
	for key, value := range context {
		classified.WithContext(key, value)
	}

	return classified
}

// NewWithContext creates a new classified error with context fields.
// This is a convenience function that creates an error and adds context in one call.
//
// Example usage:
//
//	err := apperrors.NewWithContext(
//	    apperrors.ErrClassValidation,
//	    "job_validation",
//	    "job name cannot be empty",
//	    map[string]any{
//	        "job_type": "import",
//	        "job_id": 123,
//	    },
//	)
func NewWithContext(
	class ErrorClass,
	operation string,
	message string,
	context map[string]any,
) *ClassifiedError {
	classified := New(class, operation, message)
	for key, value := range context {
		classified.WithContext(key, value)
	}

	return classified
}

func WrapWithMessage(
	class ErrorClass,
	operation string,
	message string,
	err error,
) *ClassifiedError {
	if err == nil {
		return New(class, operation, message)
	}

	classified := Wrap(class, operation, err)

	classified.Message = message

	return classified
}

func WrapWithMessageContext(
	class ErrorClass,
	operation string,
	message string,
	err error,
	context map[string]any,
) *ClassifiedError {
	if err == nil {
		return NewWithContext(class, operation, message, context)
	}

	classified := Wrap(class, operation, err)

	classified.Message = message
	for key, value := range context {
		classified.WithContext(key, value)
	}

	return classified
}

func WrapWithMessageFor(
	class ErrorClass,
	operation string,
	message string,
	messageFor string,
	err error,
) *ClassifiedError {
	if err == nil {
		classified := New(class, operation, message)

		classified.MessageFor = messageFor
		return classified
	}

	classified := Wrap(class, operation, err)

	classified.Message = message
	classified.MessageFor = messageFor

	return classified
}

func WrapWithMessageForContext(
	class ErrorClass,
	operation string,
	message string,
	messageFor string,
	err error,
	context map[string]any,
) *ClassifiedError {
	if err == nil {
		classified := NewWithContext(class, operation, message, context)

		classified.MessageFor = messageFor
		return classified
	}

	classified := Wrap(class, operation, err)

	classified.Message = message

	classified.MessageFor = messageFor
	for key, value := range context {
		classified.WithContext(key, value)
	}

	return classified
}

func WrapWithOptions(
	class ErrorClass,
	operation string,
	err error,
	options ...errorOption,
) *ClassifiedError {
	classified := Wrap(class, operation, err)

	option := &ErrorOptions{}
	for _, loption := range options {
		loption(option)
	}

	if option.Message != "" {
		classified.Message = option.Message
	}

	if option.MessageFor != "" {
		classified.MessageFor = option.MessageFor
	}

	if option.Category != "" {
		classified.Category = option.Category
	}

	if option.Err != nil {
		classified.Err = option.Err
	}

	if len(option.Context) != 0 {
		for key, value := range option.Context {
			classified.WithContext(key, value)
		}
	}

	return classified
}

func NewWithOptions(class ErrorClass, operation string, options ...errorOption) *ClassifiedError {
	classified := &ClassifiedError{
		Class:     class,
		Operation: operation,
	}

	option := &ErrorOptions{}
	for _, loption := range options {
		loption(option)
	}

	if option.Message != "" {
		classified.Message = option.Message
	}

	if option.MessageFor != "" {
		classified.MessageFor = option.MessageFor
	}

	if option.Category != "" {
		classified.Category = option.Category
	}

	if option.Err != nil {
		classified.Err = option.Err
	}

	if len(option.Context) != 0 {
		for key, value := range option.Context {
			classified.WithContext(key, value)
		}
	}

	return classified
}

// Option types for configurable components.
type ErrorOptions struct {
	Message    string
	MessageFor string
	Category   string
	Err        error
	Context    map[string]any
}

type errorOption func(*ErrorOptions)

func WithMessage(message string) errorOption {
	return func(eo *ErrorOptions) { eo.Message = message }
}

func WithMessageFor(messageFor string) errorOption {
	return func(eo *ErrorOptions) { eo.MessageFor = messageFor }
}

func WithCategory(category string) errorOption {
	return func(eo *ErrorOptions) { eo.Category = category }
}

func WithError(err error) errorOption {
	return func(eo *ErrorOptions) { eo.Err = err }
}

func WithContext(context map[string]any) errorOption {
	return func(eo *ErrorOptions) { eo.Context = context }
}
