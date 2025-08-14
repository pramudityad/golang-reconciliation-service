package errors

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// ErrorCategory represents different categories of errors
type ErrorCategory string

const (
	CategoryFile          ErrorCategory = "file"
	CategoryParse         ErrorCategory = "parse"
	CategoryValidation    ErrorCategory = "validation"
	CategoryConfiguration ErrorCategory = "configuration"
	CategoryReconciliation ErrorCategory = "reconciliation"
	CategoryNetwork       ErrorCategory = "network"
	CategoryInternal      ErrorCategory = "internal"
)

// ErrorCode represents specific error codes within categories
type ErrorCode string

const (
	// File errors
	CodeFileNotFound    ErrorCode = "file_not_found"
	CodeFilePermission  ErrorCode = "file_permission"
	CodeFileCorrupted   ErrorCode = "file_corrupted"
	CodeDirectoryError  ErrorCode = "directory_error"

	// Parse errors
	CodeInvalidFormat   ErrorCode = "invalid_format"
	CodeMissingColumn   ErrorCode = "missing_column"
	CodeInvalidData     ErrorCode = "invalid_data"
	CodeEncodingError   ErrorCode = "encoding_error"

	// Validation errors
	CodeInvalidAmount   ErrorCode = "invalid_amount"
	CodeInvalidDate     ErrorCode = "invalid_date"
	CodeMissingField    ErrorCode = "missing_field"
	CodeOutOfRange      ErrorCode = "out_of_range"

	// Configuration errors
	CodeInvalidConfig   ErrorCode = "invalid_config"
	CodeMissingConfig   ErrorCode = "missing_config"
	CodeConfigConflict  ErrorCode = "config_conflict"

	// Reconciliation errors
	CodeMatchingFailed  ErrorCode = "matching_failed"
	CodeDataInconsistent ErrorCode = "data_inconsistent"
	CodeProcessingError ErrorCode = "processing_error"

	// Network errors
	CodeConnectionFailed ErrorCode = "connection_failed"
	CodeTimeout         ErrorCode = "timeout"
	CodeServiceUnavailable ErrorCode = "service_unavailable"

	// Internal errors
	CodeUnexpectedError ErrorCode = "unexpected_error"
	CodeResourceExhausted ErrorCode = "resource_exhausted"
)

// ReconcilerError is the base error type for all application errors
type ReconcilerError struct {
	Category    ErrorCategory `json:"category"`
	Code        ErrorCode     `json:"code"`
	Message     string        `json:"message"`
	Suggestion  string        `json:"suggestion,omitempty"`
	Context     Context       `json:"context,omitempty"`
	Cause       error         `json:"-"`
	StackTrace  errors.StackTrace `json:"-"`
}

// Context provides additional information about the error
type Context map[string]interface{}

// Error implements the error interface
func (e *ReconcilerError) Error() string {
	if e.Suggestion != "" {
		return fmt.Sprintf("%s (suggestion: %s)", e.Message, e.Suggestion)
	}
	return e.Message
}

// Unwrap returns the underlying cause error
func (e *ReconcilerError) Unwrap() error {
	return e.Cause
}

// GetExitCode returns an appropriate exit code for the error
func (e *ReconcilerError) GetExitCode() int {
	switch e.Category {
	case CategoryFile:
		return 2
	case CategoryParse, CategoryValidation:
		return 3
	case CategoryConfiguration:
		return 4
	case CategoryReconciliation, CategoryInternal:
		return 5
	case CategoryNetwork:
		return 6
	default:
		return 1
	}
}

// WithContext adds context information to the error
func (e *ReconcilerError) WithContext(key string, value interface{}) *ReconcilerError {
	if e.Context == nil {
		e.Context = make(Context)
	}
	e.Context[key] = value
	return e
}

// WithSuggestion adds a suggestion for fixing the error
func (e *ReconcilerError) WithSuggestion(suggestion string) *ReconcilerError {
	e.Suggestion = suggestion
	return e
}

// New creates a new ReconcilerError
func New(category ErrorCategory, code ErrorCode, message string) *ReconcilerError {
	return &ReconcilerError{
		Category:   category,
		Code:       code,
		Message:    message,
		StackTrace: errors.New("").(stackTracer).StackTrace(),
	}
}

// Wrap wraps an existing error with ReconcilerError context
func Wrap(err error, category ErrorCategory, code ErrorCode, message string) *ReconcilerError {
	if err == nil {
		return nil
	}

	return &ReconcilerError{
		Category:   category,
		Code:       code,
		Message:    message,
		Cause:      err,
		StackTrace: errors.WithStack(err).(stackTracer).StackTrace(),
	}
}

// stackTracer interface for extracting stack traces
type stackTracer interface {
	StackTrace() errors.StackTrace
}

// Specific error constructors

// FileError creates a file-related error
func FileError(code ErrorCode, path string, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeFileNotFound:
		message = fmt.Sprintf("file not found: %s", path)
		suggestion = "check if the file path is correct and the file exists"
	case CodeFilePermission:
		message = fmt.Sprintf("permission denied accessing file: %s", path)
		suggestion = "check file permissions and ensure you have read access"
	case CodeFileCorrupted:
		message = fmt.Sprintf("file appears to be corrupted: %s", path)
		suggestion = "verify the file integrity and try using a backup copy"
	case CodeDirectoryError:
		message = fmt.Sprintf("directory error: %s", path)
		suggestion = "ensure the directory exists and is accessible"
	default:
		message = fmt.Sprintf("file error: %s", path)
		suggestion = "check the file and try again"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryFile, code, message)
	} else {
		result = New(CategoryFile, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("file_path", path)
}

// ParseError creates a parsing-related error
func ParseError(code ErrorCode, file string, line int, column string, value string, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeInvalidFormat:
		message = fmt.Sprintf("invalid format in file %s at line %d, column '%s': '%s'", file, line, column, value)
		suggestion = "check the data format and ensure it matches the expected structure"
	case CodeMissingColumn:
		message = fmt.Sprintf("missing required column '%s' in file %s", column, file)
		suggestion = "verify the file has all required columns with correct headers"
	case CodeInvalidData:
		message = fmt.Sprintf("invalid data in file %s at line %d, column '%s': '%s'", file, line, column, value)
		suggestion = "correct the data format or remove the invalid entry"
	case CodeEncodingError:
		message = fmt.Sprintf("encoding error in file %s at line %d", file, line)
		suggestion = "ensure the file is saved in UTF-8 encoding"
	default:
		message = fmt.Sprintf("parse error in file %s at line %d", file, line)
		suggestion = "check the file format and data integrity"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryParse, code, message)
	} else {
		result = New(CategoryParse, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("file", file).
		WithContext("line", line).
		WithContext("column", column).
		WithContext("value", value)
}

// ValidationError creates a validation-related error
func ValidationError(code ErrorCode, field string, value interface{}, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeInvalidAmount:
		message = fmt.Sprintf("invalid amount in field '%s': %v", field, value)
		suggestion = "ensure amounts are valid decimal numbers (e.g., '12.34')"
	case CodeInvalidDate:
		message = fmt.Sprintf("invalid date in field '%s': %v", field, value)
		suggestion = "use date format YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"
	case CodeMissingField:
		message = fmt.Sprintf("required field '%s' is missing or empty", field)
		suggestion = "provide a value for this required field"
	case CodeOutOfRange:
		message = fmt.Sprintf("value out of range in field '%s': %v", field, value)
		suggestion = "ensure the value is within the acceptable range"
	default:
		message = fmt.Sprintf("validation error in field '%s': %v", field, value)
		suggestion = "check the field value and format"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryValidation, code, message)
	} else {
		result = New(CategoryValidation, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("field", field).
		WithContext("value", value)
}

// ConfigurationError creates a configuration-related error
func ConfigurationError(code ErrorCode, setting string, value interface{}, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeInvalidConfig:
		message = fmt.Sprintf("invalid configuration for '%s': %v", setting, value)
		suggestion = "check the configuration documentation for valid values"
	case CodeMissingConfig:
		message = fmt.Sprintf("missing required configuration: %s", setting)
		suggestion = "provide this configuration setting or use a config file"
	case CodeConfigConflict:
		message = fmt.Sprintf("configuration conflict with setting '%s': %v", setting, value)
		suggestion = "resolve the conflicting settings or use default values"
	default:
		message = fmt.Sprintf("configuration error: %s", setting)
		suggestion = "check your configuration and try again"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryConfiguration, code, message)
	} else {
		result = New(CategoryConfiguration, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("setting", setting).
		WithContext("value", value)
}

// ReconciliationError creates a reconciliation-related error
func ReconciliationError(code ErrorCode, operation string, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeMatchingFailed:
		message = fmt.Sprintf("matching failed during %s", operation)
		suggestion = "try adjusting matching tolerances or check data quality"
	case CodeDataInconsistent:
		message = fmt.Sprintf("data inconsistency detected during %s", operation)
		suggestion = "verify data integrity and resolve inconsistencies"
	case CodeProcessingError:
		message = fmt.Sprintf("processing error during %s", operation)
		suggestion = "check system resources and try again"
	default:
		message = fmt.Sprintf("reconciliation error during %s", operation)
		suggestion = "review the data and configuration"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryReconciliation, code, message)
	} else {
		result = New(CategoryReconciliation, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("operation", operation)
}

// NetworkError creates a network-related error
func NetworkError(code ErrorCode, endpoint string, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeConnectionFailed:
		message = fmt.Sprintf("connection failed to %s", endpoint)
		suggestion = "check network connectivity and endpoint availability"
	case CodeTimeout:
		message = fmt.Sprintf("timeout connecting to %s", endpoint)
		suggestion = "increase timeout setting or check network speed"
	case CodeServiceUnavailable:
		message = fmt.Sprintf("service unavailable: %s", endpoint)
		suggestion = "try again later or contact service administrator"
	default:
		message = fmt.Sprintf("network error: %s", endpoint)
		suggestion = "check network connection and try again"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryNetwork, code, message)
	} else {
		result = New(CategoryNetwork, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("endpoint", endpoint)
}

// InternalError creates an internal error
func InternalError(code ErrorCode, operation string, err error) *ReconcilerError {
	var message string
	var suggestion string

	switch code {
	case CodeUnexpectedError:
		message = fmt.Sprintf("unexpected error during %s", operation)
		suggestion = "this is likely a bug - please report it with the error details"
	case CodeResourceExhausted:
		message = fmt.Sprintf("resource exhausted during %s", operation)
		suggestion = "try reducing batch size or increasing system resources"
	default:
		message = fmt.Sprintf("internal error during %s", operation)
		suggestion = "try again or contact support if the problem persists"
	}

	var result *ReconcilerError
	if err != nil {
		result = Wrap(err, CategoryInternal, code, message)
	} else {
		result = New(CategoryInternal, code, message)
	}
	
	return result.
		WithSuggestion(suggestion).
		WithContext("operation", operation)
}

// ErrorSummary provides a summary of multiple errors
type ErrorSummary struct {
	Total        int                 `json:"total"`
	ByCategory   map[ErrorCategory]int `json:"by_category"`
	ByCode       map[ErrorCode]int   `json:"by_code"`
	Errors       []*ReconcilerError  `json:"errors"`
	SampleErrors []*ReconcilerError  `json:"sample_errors,omitempty"`
}

// NewErrorSummary creates a new error summary
func NewErrorSummary(errors []*ReconcilerError) *ErrorSummary {
	if len(errors) == 0 {
		return &ErrorSummary{
			Total:      0,
			ByCategory: make(map[ErrorCategory]int),
			ByCode:     make(map[ErrorCode]int),
			Errors:     []*ReconcilerError{},
		}
	}

	summary := &ErrorSummary{
		Total:      len(errors),
		ByCategory: make(map[ErrorCategory]int),
		ByCode:     make(map[ErrorCode]int),
		Errors:     errors,
	}

	// Count by category and code
	for _, err := range errors {
		summary.ByCategory[err.Category]++
		summary.ByCode[err.Code]++
	}

	// Include sample errors (max 5)
	maxSamples := 5
	if len(errors) > maxSamples {
		summary.SampleErrors = errors[:maxSamples]
	} else {
		summary.SampleErrors = errors
	}

	return summary
}

// Error returns a formatted error message for the summary
func (es *ErrorSummary) Error() string {
	if es.Total == 0 {
		return "no errors"
	}

	if es.Total == 1 {
		return es.Errors[0].Error()
	}

	var categories []string
	for category, count := range es.ByCategory {
		categories = append(categories, fmt.Sprintf("%s: %d", category, count))
	}

	return fmt.Sprintf("%d errors occurred (%s)", es.Total, strings.Join(categories, ", "))
}

// HasCategory checks if the summary contains errors of the given category
func (es *ErrorSummary) HasCategory(category ErrorCategory) bool {
	count, exists := es.ByCategory[category]
	return exists && count > 0
}

// HasCode checks if the summary contains errors with the given code
func (es *ErrorSummary) HasCode(code ErrorCode) bool {
	count, exists := es.ByCode[code]
	return exists && count > 0
}

// GetExitCode returns the highest priority exit code from all errors
func (es *ErrorSummary) GetExitCode() int {
	if es.Total == 0 {
		return 0
	}

	maxCode := 1
	for _, err := range es.Errors {
		if code := err.GetExitCode(); code > maxCode {
			maxCode = code
		}
	}

	return maxCode
}

// Utility functions

// IsReconcilerError checks if an error is a ReconcilerError
func IsReconcilerError(err error) bool {
	_, ok := err.(*ReconcilerError)
	return ok
}

// AsReconcilerError extracts a ReconcilerError from an error chain
func AsReconcilerError(err error) (*ReconcilerError, bool) {
	var reconcilerErr *ReconcilerError
	if errors.As(err, &reconcilerErr) {
		return reconcilerErr, true
	}
	return nil, false
}

// WrapIfNeeded wraps an error if it's not already a ReconcilerError
func WrapIfNeeded(err error, category ErrorCategory, code ErrorCode, message string) *ReconcilerError {
	if err == nil {
		return nil
	}

	if reconcilerErr, ok := AsReconcilerError(err); ok {
		return reconcilerErr
	}

	return Wrap(err, category, code, message)
}