package apperrors

import (
	"bytes"

	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// ErrorClass represents the category of an error.
type ErrorClass string

const (
	// ErrClassConfig represents configuration-related errors.
	ErrClassConfig ErrorClass = "CONFIG"
	// ErrClassDatabase represents database-related errors.
	ErrClassDatabase ErrorClass = "DATABASE"
	// ErrClassAPI represents API-related errors.
	ErrClassAPI ErrorClass = "API"
	// ErrClassMetadata represents Metadata-related errors.
	ErrClassMetadata    ErrorClass = "METADATA"
	ErrClassImportfeed  ErrorClass = "IMPORTFEED"
	ErrClassAPIExternal ErrorClass = "APIEXTERNAL"
	ErrClassSearcher    ErrorClass = "SEARCHER"
	ErrClassStructure   ErrorClass = "STRUCTURE"
	// ErrClassNetwork represents network-related errors.
	ErrClassNetwork ErrorClass = "NETWORK"
	// ErrClassValidation represents validation-related errors.
	ErrClassValidation ErrorClass = "VALIDATION"
	// ErrClassFileSystem represents filesystem-related errors.
	ErrClassFileSystem ErrorClass = "FILESYSTEM"
	// ErrClassParsing represents parsing-related errors.
	ErrClassParsing ErrorClass = "PARSING"
	// ErrClassDownload represents download-related errors.
	ErrClassDownload ErrorClass = "DOWNLOAD"
	// ErrClassUnknown represents unknown or unclassified errors.
	ErrClassUnknown ErrorClass = "UNKNOWN"
)

// ClassifiedError wraps an error with classification metadata.
type ClassifiedError struct {
	// Class represents the category of the error
	Class ErrorClass
	// Operation describes the operation that failed
	Operation string
	// Message describes the failed operation in more detail
	Message string
	// MessageFor describes ‘for’ field for the error. It helps to identify the entity on which an operation failed.
	MessageFor string
	// Category is a subgroup of Class
	Category string
	// Err is the underlying error
	Err error
	// Context provides additional context about the error
	Context map[string]any
}

// Error implements the error interface.
func (e *ClassifiedError) Error() string {
	bld := errorBuilder.Get()
	defer errorBuilder.Put(bld)

	bld.WriteRune('[')
	bld.WriteString(string(e.Class))
	bld.WriteRune(']')

	if e.Category != "" {
		bld.WriteString(" (")
		bld.WriteString(e.Category)
		bld.WriteString(")")
	}

	if e.Operation != "" {
		bld.WriteRune(' ')
		bld.WriteString(e.Operation)
	}

	if e.Message != "" {
		bld.WriteRune(' ')
		bld.WriteString(e.Message)
	}

	if e.MessageFor != "" {
		bld.WriteString(" for: ")
		bld.WriteString(e.MessageFor)
	}

	if e.Err != nil {
		bld.WriteString(" Error: ")
		bld.WriteString(e.Err.Error())
	}
	// if e.Operation != "" {
	// 	return fmt.Sprintf("[%s] %s: %v", e.Class, e.Operation, e.Err)
	// }
	// return fmt.Sprintf("[%s] %v", e.Class, e.Err)
	return bld.String()
}

// Unwrap returns the wrapped error for errors.Is/As compatibility.
func (e *ClassifiedError) Unwrap() error {
	return e.Err
}

var errorBuilder = pool.NewPool(200, 10, func(b *bytes.Buffer) {
	if b.Cap() < 900 {
		b.Grow(900)
	}

	if b.Len() > 1 {
		b.Reset()
	}
}, func(b *bytes.Buffer) bool {
	b.Reset()
	return false
})
