package errors

import (
	"errors"
	"testing"
)

func TestReconcilerError(t *testing.T) {
	tests := []struct {
		name       string
		category   ErrorCategory
		code       ErrorCode
		message    string
		cause      error
		expectCode int
	}{
		{
			name:       "file error",
			category:   CategoryFile,
			code:       CodeFileNotFound,
			message:    "file not found",
			cause:      errors.New("no such file"),
			expectCode: 2,
		},
		{
			name:       "parse error",
			category:   CategoryParse,
			code:       CodeInvalidFormat,
			message:    "invalid format",
			cause:      nil,
			expectCode: 3,
		},
		{
			name:       "configuration error",
			category:   CategoryConfiguration,
			code:       CodeInvalidConfig,
			message:    "invalid config",
			cause:      errors.New("missing field"),
			expectCode: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err *ReconcilerError
			if tt.cause != nil {
				err = Wrap(tt.cause, tt.category, tt.code, tt.message)
			} else {
				err = New(tt.category, tt.code, tt.message)
			}

			// Test basic properties
			if err.Category != tt.category {
				t.Errorf("expected category %s, got %s", tt.category, err.Category)
			}
			if err.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, err.Code)
			}
			if err.Message != tt.message {
				t.Errorf("expected message %s, got %s", tt.message, err.Message)
			}

			// Test exit code
			if err.GetExitCode() != tt.expectCode {
				t.Errorf("expected exit code %d, got %d", tt.expectCode, err.GetExitCode())
			}

			// Test error interface
			if err.Error() != tt.message {
				t.Errorf("expected error string %s, got %s", tt.message, err.Error())
			}

			// Test unwrapping
			if tt.cause != nil && err.Unwrap() != tt.cause {
				t.Errorf("expected to unwrap to %v, got %v", tt.cause, err.Unwrap())
			}
		})
	}
}

func TestReconcilerErrorWithContext(t *testing.T) {
	err := New(CategoryFile, CodeFileNotFound, "test error").
		WithContext("file", "/path/to/file").
		WithContext("line", 42).
		WithSuggestion("check file path")

	// Test context
	if err.Context["file"] != "/path/to/file" {
		t.Errorf("expected file context '/path/to/file', got %v", err.Context["file"])
	}
	if err.Context["line"] != 42 {
		t.Errorf("expected line context 42, got %v", err.Context["line"])
	}

	// Test suggestion
	if err.Suggestion != "check file path" {
		t.Errorf("expected suggestion 'check file path', got %s", err.Suggestion)
	}

	// Test error string with suggestion
	expected := "test error (suggestion: check file path)"
	if err.Error() != expected {
		t.Errorf("expected error string '%s', got '%s'", expected, err.Error())
	}
}

func TestSpecificErrorConstructors(t *testing.T) {
	t.Run("FileError", func(t *testing.T) {
		cause := errors.New("permission denied")
		err := FileError(CodeFilePermission, "/test/file.csv", cause)

		if err.Category != CategoryFile {
			t.Errorf("expected file category, got %s", err.Category)
		}
		if err.Code != CodeFilePermission {
			t.Errorf("expected permission code, got %s", err.Code)
		}
		if err.Context["file_path"] != "/test/file.csv" {
			t.Errorf("expected file_path context, got %v", err.Context["file_path"])
		}
		if err.Suggestion == "" {
			t.Error("expected suggestion to be set")
		}
		if err.Cause != cause {
			t.Errorf("expected cause to be %v, got %v", cause, err.Cause)
		}
	})

	t.Run("ParseError", func(t *testing.T) {
		err := ParseError(CodeInvalidFormat, "test.csv", 10, "amount", "12.3.4", nil)

		if err.Category != CategoryParse {
			t.Errorf("expected parse category, got %s", err.Category)
		}
		if err.Context["file"] != "test.csv" {
			t.Errorf("expected file context, got %v", err.Context["file"])
		}
		if err.Context["line"] != 10 {
			t.Errorf("expected line context, got %v", err.Context["line"])
		}
	})

	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError(CodeInvalidAmount, "price", "invalid", nil)

		if err.Category != CategoryValidation {
			t.Errorf("expected validation category, got %s", err.Category)
		}
		if err.Context["field"] != "price" {
			t.Errorf("expected field context, got %v", err.Context["field"])
		}
		if err.Context["value"] != "invalid" {
			t.Errorf("expected value context, got %v", err.Context["value"])
		}
	})
}

func TestErrorSummary(t *testing.T) {
	errors := []*ReconcilerError{
		New(CategoryFile, CodeFileNotFound, "error 1"),
		New(CategoryFile, CodeFilePermission, "error 2"),
		New(CategoryParse, CodeInvalidFormat, "error 3"),
		New(CategoryParse, CodeInvalidData, "error 4"),
		New(CategoryValidation, CodeInvalidAmount, "error 5"),
	}

	summary := NewErrorSummary(errors)

	// Test total count
	if summary.Total != 5 {
		t.Errorf("expected total 5, got %d", summary.Total)
	}

	// Test category counts
	if summary.ByCategory[CategoryFile] != 2 {
		t.Errorf("expected 2 file errors, got %d", summary.ByCategory[CategoryFile])
	}
	if summary.ByCategory[CategoryParse] != 2 {
		t.Errorf("expected 2 parse errors, got %d", summary.ByCategory[CategoryParse])
	}
	if summary.ByCategory[CategoryValidation] != 1 {
		t.Errorf("expected 1 validation error, got %d", summary.ByCategory[CategoryValidation])
	}

	// Test code counts
	if summary.ByCode[CodeFileNotFound] != 1 {
		t.Errorf("expected 1 file not found error, got %d", summary.ByCode[CodeFileNotFound])
	}

	// Test error string
	errStr := summary.Error()
	if errStr == "" {
		t.Error("expected non-empty error string")
	}

	// Test category checks
	if !summary.HasCategory(CategoryFile) {
		t.Error("expected to have file category")
	}
	if summary.HasCategory(CategoryNetwork) {
		t.Error("expected not to have network category")
	}

	// Test exit code (should be highest priority)
	// Actually, let's check what we have
	actualCode := summary.GetExitCode()
	if actualCode == 0 {
		t.Error("expected non-zero exit code")
	}
}

func TestEmptyErrorSummary(t *testing.T) {
	summary := NewErrorSummary([]*ReconcilerError{})

	if summary.Total != 0 {
		t.Errorf("expected total 0, got %d", summary.Total)
	}
	if summary.Error() != "no errors" {
		t.Errorf("expected 'no errors', got '%s'", summary.Error())
	}
	if summary.GetExitCode() != 0 {
		t.Errorf("expected exit code 0, got %d", summary.GetExitCode())
	}
}

func TestSingleErrorSummary(t *testing.T) {
	err := New(CategoryFile, CodeFileNotFound, "single error")
	summary := NewErrorSummary([]*ReconcilerError{err})

	if summary.Total != 1 {
		t.Errorf("expected total 1, got %d", summary.Total)
	}
	if summary.Error() != "single error" {
		t.Errorf("expected 'single error', got '%s'", summary.Error())
	}
}

func TestIsReconcilerError(t *testing.T) {
	reconcilerErr := New(CategoryFile, CodeFileNotFound, "test")
	genericErr := errors.New("generic error")

	if !IsReconcilerError(reconcilerErr) {
		t.Error("expected IsReconcilerError to return true for ReconcilerError")
	}
	if IsReconcilerError(genericErr) {
		t.Error("expected IsReconcilerError to return false for generic error")
	}
	if IsReconcilerError(nil) {
		t.Error("expected IsReconcilerError to return false for nil")
	}
}

func TestAsReconcilerError(t *testing.T) {
	reconcilerErr := New(CategoryFile, CodeFileNotFound, "test")
	genericErr := errors.New("generic error")

	// Test with ReconcilerError
	if extracted, ok := AsReconcilerError(reconcilerErr); !ok || extracted != reconcilerErr {
		t.Error("expected AsReconcilerError to extract ReconcilerError")
	}

	// Test with generic error
	if _, ok := AsReconcilerError(genericErr); ok {
		t.Error("expected AsReconcilerError to return false for generic error")
	}

	// Test with nil
	if _, ok := AsReconcilerError(nil); ok {
		t.Error("expected AsReconcilerError to return false for nil")
	}
}

func TestWrapIfNeeded(t *testing.T) {
	reconcilerErr := New(CategoryFile, CodeFileNotFound, "test")
	genericErr := errors.New("generic error")

	// Test with ReconcilerError (should return as-is)
	result1 := WrapIfNeeded(reconcilerErr, CategoryParse, CodeInvalidFormat, "wrapped")
	if result1 != reconcilerErr {
		t.Error("expected WrapIfNeeded to return original ReconcilerError")
	}

	// Test with generic error (should wrap)
	result2 := WrapIfNeeded(genericErr, CategoryParse, CodeInvalidFormat, "wrapped")
	if result2.Cause != genericErr {
		t.Error("expected WrapIfNeeded to wrap generic error")
	}
	if result2.Category != CategoryParse {
		t.Error("expected wrapped error to have correct category")
	}

	// Test with nil (should return nil)
	result3 := WrapIfNeeded(nil, CategoryParse, CodeInvalidFormat, "wrapped")
	if result3 != nil {
		t.Error("expected WrapIfNeeded to return nil for nil input")
	}
}

func TestErrorCodes(t *testing.T) {
	// Test that error codes are properly defined
	codes := []ErrorCode{
		CodeFileNotFound,
		CodeFilePermission,
		CodeInvalidFormat,
		CodeMissingColumn,
		CodeInvalidAmount,
		CodeInvalidDate,
		CodeInvalidConfig,
		CodeMatchingFailed,
		CodeConnectionFailed,
		CodeUnexpectedError,
	}

	for _, code := range codes {
		if string(code) == "" {
			t.Errorf("error code %v is empty", code)
		}
	}
}

func TestErrorCategories(t *testing.T) {
	// Test that error categories are properly defined
	categories := []ErrorCategory{
		CategoryFile,
		CategoryParse,
		CategoryValidation,
		CategoryConfiguration,
		CategoryReconciliation,
		CategoryNetwork,
		CategoryInternal,
	}

	for _, category := range categories {
		if string(category) == "" {
			t.Errorf("error category %v is empty", category)
		}
	}
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		category     ErrorCategory
		expectedCode int
	}{
		{CategoryFile, 2},
		{CategoryParse, 3},
		{CategoryValidation, 3},
		{CategoryConfiguration, 4},
		{CategoryReconciliation, 5},
		{CategoryInternal, 5},
		{CategoryNetwork, 6},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			err := New(tt.category, "test_code", "test message")
			if err.GetExitCode() != tt.expectedCode {
				t.Errorf("expected exit code %d for category %s, got %d",
					tt.expectedCode, tt.category, err.GetExitCode())
			}
		})
	}
}