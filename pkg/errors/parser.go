package errors

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ParseContext provides context information for parsing operations
type ParseContext struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   string `json:"column"`
	Value    string `json:"value"`
	Expected string `json:"expected,omitempty"`
	Row      int    `json:"row,omitempty"`
}

// EnhancedParseError extends the base ParseError with better context and suggestions
type EnhancedParseError struct {
	*ReconcilerError
	Context     *ParseContext     `json:"context"`
	Recoverable bool             `json:"recoverable"`
	LineContent string           `json:"line_content,omitempty"`
	Examples    []string         `json:"examples,omitempty"`
}

// Error implements the error interface with enhanced formatting
func (e *EnhancedParseError) Error() string {
	var parts []string
	
	// Basic error message
	parts = append(parts, e.ReconcilerError.Error())
	
	// Location information
	if e.Context != nil {
		location := fmt.Sprintf("at %s", filepath.Base(e.Context.File))
		if e.Context.Line > 0 {
			location += fmt.Sprintf(":%d", e.Context.Line)
		}
		if e.Context.Column != "" {
			location += fmt.Sprintf(" column '%s'", e.Context.Column)
		}
		parts = append(parts, location)
	}
	
	return strings.Join(parts, " ")
}

// GetDetailedError returns a detailed multi-line error description
func (e *EnhancedParseError) GetDetailedError() string {
	var lines []string
	
	// Error header
	lines = append(lines, fmt.Sprintf("ERROR: %s", e.Message))
	
	// Location information
	if e.Context != nil {
		lines = append(lines, fmt.Sprintf("  → File: %s", e.Context.File))
		if e.Context.Line > 0 {
			lines = append(lines, fmt.Sprintf("  → Line: %d", e.Context.Line))
		}
		if e.Context.Column != "" {
			lines = append(lines, fmt.Sprintf("  → Column: %s", e.Context.Column))
		}
		if e.Context.Value != "" {
			lines = append(lines, fmt.Sprintf("  → Value: '%s'", e.Context.Value))
		}
		if e.Context.Expected != "" {
			lines = append(lines, fmt.Sprintf("  → Expected: %s", e.Context.Expected))
		}
	}
	
	// Line content if available
	if e.LineContent != "" {
		lines = append(lines, fmt.Sprintf("  → Content: %s", e.LineContent))
	}
	
	// Suggestion
	if e.Suggestion != "" {
		lines = append(lines, fmt.Sprintf("  → Suggestion: %s", e.Suggestion))
	}
	
	// Examples if available
	if len(e.Examples) > 0 {
		lines = append(lines, "  → Examples:")
		for _, example := range e.Examples {
			lines = append(lines, fmt.Sprintf("    • %s", example))
		}
	}
	
	return strings.Join(lines, "\n")
}

// NewEnhancedParseError creates a new enhanced parse error
func NewEnhancedParseError(code ErrorCode, context *ParseContext, message string, cause error) *EnhancedParseError {
	baseError := Wrap(cause, CategoryParse, code, message)
	
	// Add context to base error
	if context != nil {
		baseError.WithContext("file", context.File).
			WithContext("line", context.Line).
			WithContext("column", context.Column).
			WithContext("value", context.Value)
	}
	
	return &EnhancedParseError{
		ReconcilerError: baseError,
		Context:         context,
		Recoverable:     true, // Most parse errors are recoverable by default
	}
}

// WithLineContent adds the actual line content to the error
func (e *EnhancedParseError) WithLineContent(content string) *EnhancedParseError {
	e.LineContent = content
	return e
}

// WithExamples adds example values to help fix the error
func (e *EnhancedParseError) WithExamples(examples ...string) *EnhancedParseError {
	e.Examples = examples
	return e
}

// WithSuggestion adds a suggestion and returns the EnhancedParseError
func (e *EnhancedParseError) WithSuggestion(suggestion string) *EnhancedParseError {
	e.ReconcilerError.WithSuggestion(suggestion)
	return e
}

// WithRecoverable sets whether this error is recoverable
func (e *EnhancedParseError) WithRecoverable(recoverable bool) *EnhancedParseError {
	e.Recoverable = recoverable
	return e
}

// Common parse error constructors

// InvalidAmountError creates an error for invalid amount format
func InvalidAmountError(file string, line int, column string, value string) *EnhancedParseError {
	context := &ParseContext{
		File:     file,
		Line:     line,
		Column:   column,
		Value:    value,
		Expected: "decimal number",
	}
	
	message := "invalid amount format"
	err := NewEnhancedParseError(CodeInvalidAmount, context, message, nil).
		WithExamples("12.34", "1250.50", "-500.00").
		WithSuggestion("Remove currency symbols and use decimal format")
	
	return err
}

// InvalidDateError creates an error for invalid date format
func InvalidDateError(file string, line int, column string, value string) *EnhancedParseError {
	context := &ParseContext{
		File:     file,
		Line:     line,
		Column:   column,
		Value:    value,
		Expected: "date in YYYY-MM-DD format",
	}
	
	message := "invalid date format"
	err := NewEnhancedParseError(CodeInvalidDate, context, message, nil).
		WithExamples("2024-01-15", "2024-12-31", "2023-06-01").
		WithSuggestion("Use YYYY-MM-DD format or YYYY-MM-DD HH:MM:SS for timestamps")
	
	return err
}

// MissingColumnError creates an error for missing required columns
func MissingColumnError(file string, expectedColumns []string, actualColumns []string) *EnhancedParseError {
	missing := findMissingColumns(expectedColumns, actualColumns)
	
	context := &ParseContext{
		File:     file,
		Line:     1, // Header line
		Expected: fmt.Sprintf("columns: %s", strings.Join(expectedColumns, ", ")),
	}
	
	message := fmt.Sprintf("missing required columns: %s", strings.Join(missing, ", "))
	err := NewEnhancedParseError(CodeMissingColumn, context, message, nil).
		WithSuggestion("Add the missing columns to your CSV file header")
	
	err.Recoverable = false // Missing columns typically can't be recovered from
	return err
}

// InvalidTransactionTypeError creates an error for invalid transaction types
func InvalidTransactionTypeError(file string, line int, column string, value string) *EnhancedParseError {
	context := &ParseContext{
		File:     file,
		Line:     line,
		Column:   column,
		Value:    value,
		Expected: "CREDIT or DEBIT",
	}
	
	message := "invalid transaction type"
	err := NewEnhancedParseError(CodeInvalidData, context, message, nil).
		WithExamples("CREDIT", "DEBIT", "C", "D").
		WithSuggestion("Use CREDIT/DEBIT or C/D for transaction types")
	
	return err
}

// EmptyValueError creates an error for empty required values
func EmptyValueError(file string, line int, column string) *EnhancedParseError {
	context := &ParseContext{
		File:     file,
		Line:     line,
		Column:   column,
		Value:    "",
		Expected: "non-empty value",
	}
	
	message := "required field is empty"
	err := NewEnhancedParseError(CodeMissingField, context, message, nil).
		WithSuggestion("Provide a value for this required field")
	
	return err
}

// EncodingError creates an error for file encoding issues
func EncodingError(file string, line int, cause error) *EnhancedParseError {
	context := &ParseContext{
		File: file,
		Line: line,
	}
	
	message := "file encoding error"
	err := NewEnhancedParseError(CodeEncodingError, context, message, cause).
		WithSuggestion("Save the file in UTF-8 encoding")
	
	err.Recoverable = false
	return err
}

// OutOfRangeError creates an error for values outside acceptable ranges
func OutOfRangeError(file string, line int, column string, value string, min, max interface{}) *EnhancedParseError {
	context := &ParseContext{
		File:     file,
		Line:     line,
		Column:   column,
		Value:    value,
		Expected: fmt.Sprintf("value between %v and %v", min, max),
	}
	
	message := "value out of acceptable range"
	err := NewEnhancedParseError(CodeOutOfRange, context, message, nil).
		WithSuggestion(fmt.Sprintf("Ensure the value is between %v and %v", min, max))
	
	return err
}

// ParseErrorCollector collects multiple parse errors during processing
type ParseErrorCollector struct {
	errors        []*EnhancedParseError
	maxErrors     int
	continueOnError bool
}

// NewParseErrorCollector creates a new error collector
func NewParseErrorCollector(maxErrors int, continueOnError bool) *ParseErrorCollector {
	return &ParseErrorCollector{
		errors:          make([]*EnhancedParseError, 0),
		maxErrors:       maxErrors,
		continueOnError: continueOnError,
	}
}

// Add adds an error to the collector
func (c *ParseErrorCollector) Add(err *EnhancedParseError) bool {
	if err == nil {
		return true
	}
	
	c.errors = append(c.errors, err)
	
	// Check if we should continue processing
	if len(c.errors) >= c.maxErrors {
		return false
	}
	
	return c.continueOnError || err.Recoverable
}

// HasErrors returns true if any errors have been collected
func (c *ParseErrorCollector) HasErrors() bool {
	return len(c.errors) > 0
}

// GetErrors returns all collected errors
func (c *ParseErrorCollector) GetErrors() []*EnhancedParseError {
	return c.errors
}

// GetReconcilerErrors converts all errors to base ReconcilerError type
func (c *ParseErrorCollector) GetReconcilerErrors() []*ReconcilerError {
	result := make([]*ReconcilerError, len(c.errors))
	for i, err := range c.errors {
		result[i] = err.ReconcilerError
	}
	return result
}

// GetSummary returns an error summary for all collected errors
func (c *ParseErrorCollector) GetSummary() *ErrorSummary {
	return NewErrorSummary(c.GetReconcilerErrors())
}

// Clear clears all collected errors
func (c *ParseErrorCollector) Clear() {
	c.errors = c.errors[:0]
}

// Helper functions

func findMissingColumns(expected, actual []string) []string {
	actualSet := make(map[string]bool)
	for _, col := range actual {
		actualSet[strings.ToLower(strings.TrimSpace(col))] = true
	}
	
	var missing []string
	for _, col := range expected {
		if !actualSet[strings.ToLower(strings.TrimSpace(col))] {
			missing = append(missing, col)
		}
	}
	
	return missing
}

// FormatParseErrorsForUser formats multiple parse errors in a user-friendly way
func FormatParseErrorsForUser(errors []*EnhancedParseError) string {
	if len(errors) == 0 {
		return "No parse errors"
	}
	
	if len(errors) == 1 {
		return errors[0].GetDetailedError()
	}
	
	var lines []string
	lines = append(lines, fmt.Sprintf("Found %d parse errors:", len(errors)))
	lines = append(lines, "")
	
	// Group errors by file
	errorsByFile := make(map[string][]*EnhancedParseError)
	for _, err := range errors {
		file := "unknown"
		if err.Context != nil {
			file = filepath.Base(err.Context.File)
		}
		errorsByFile[file] = append(errorsByFile[file], err)
	}
	
	// Display errors grouped by file
	for file, fileErrors := range errorsByFile {
		lines = append(lines, fmt.Sprintf("File: %s (%d errors)", file, len(fileErrors)))
		
		// Show first few errors in detail, summarize the rest
		maxDetailedErrors := 3
		for i, err := range fileErrors {
			if i < maxDetailedErrors {
				lines = append(lines, "")
				lines = append(lines, err.GetDetailedError())
			} else if i == maxDetailedErrors {
				remaining := len(fileErrors) - maxDetailedErrors
				lines = append(lines, "")
				lines = append(lines, fmt.Sprintf("... and %d more errors in this file", remaining))
				break
			}
		}
		lines = append(lines, "")
	}
	
	return strings.Join(lines, "\n")
}

// SuggestionsForCommonErrors provides suggestions for common parsing issues
func SuggestionsForCommonErrors() string {
	return `Common solutions for parse errors:

• Invalid amounts: Remove currency symbols ($, €, etc.) and use decimal format (12.34)
• Invalid dates: Use YYYY-MM-DD format (2024-01-15) or YYYY-MM-DD HH:MM:SS
• Missing columns: Ensure your CSV has all required headers
• Encoding issues: Save your file in UTF-8 encoding
• Empty values: Provide values for all required fields
• Wrong separators: Use commas (,) to separate CSV fields

For more help, check the documentation or use --help flag.`
}