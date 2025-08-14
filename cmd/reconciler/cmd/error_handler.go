package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang-reconciliation-service/pkg/errors"
	"golang-reconciliation-service/pkg/logger"

	"github.com/spf13/viper"
)

// CLIErrorHandler provides user-friendly error handling for CLI operations
type CLIErrorHandler struct {
	logger logger.Logger
	verbose bool
}

// NewCLIErrorHandler creates a new CLI error handler
func NewCLIErrorHandler() *CLIErrorHandler {
	return &CLIErrorHandler{
		logger:  logger.GetGlobalLogger().WithComponent("cli"),
		verbose: viper.GetBool("verbose"),
	}
}

// HandleError handles errors and provides user-friendly messages
func (h *CLIErrorHandler) HandleError(err error) int {
	if err == nil {
		return 0
	}

	// Log the error
	h.logger.WithError(err).Error("Command failed")

	// Handle ReconcilerError with detailed information
	if reconcilerErr, ok := errors.AsReconcilerError(err); ok {
		return h.handleReconcilerError(reconcilerErr)
	}

	// Handle other error types
	return h.handleGenericError(err)
}

// handleReconcilerError handles ReconcilerError with detailed context
func (h *CLIErrorHandler) handleReconcilerError(err *errors.ReconcilerError) int {
	// Print the main error message
	fmt.Fprintf(os.Stderr, "Error: %s\n", err.Message)

	// Add context information if available
	if err.Context != nil && len(err.Context) > 0 {
		fmt.Fprintf(os.Stderr, "\nContext:\n")
		for key, value := range err.Context {
			fmt.Fprintf(os.Stderr, "  %s: %v\n", key, value)
		}
	}

	// Add suggestion if available
	if err.Suggestion != "" {
		fmt.Fprintf(os.Stderr, "\nSuggestion: %s\n", err.Suggestion)
	}

	// Add category-specific help
	fmt.Fprintf(os.Stderr, "\n%s\n", h.getCategoryHelp(err.Category))

	// Show underlying error in verbose mode
	if h.verbose && err.Cause != nil {
		fmt.Fprintf(os.Stderr, "\nUnderlying error: %v\n", err.Cause)
	}

	return err.GetExitCode()
}

// handleGenericError handles non-ReconcilerError types
func (h *CLIErrorHandler) handleGenericError(err error) int {
	// Check for common system errors and provide better messages
	if h.isFileNotFoundError(err) {
		fmt.Fprintf(os.Stderr, "Error: File not found\n")
		fmt.Fprintf(os.Stderr, "Suggestion: Check if the file path is correct and the file exists\n")
		return 2
	}

	if h.isPermissionError(err) {
		fmt.Fprintf(os.Stderr, "Error: Permission denied\n")
		fmt.Fprintf(os.Stderr, "Suggestion: Check file permissions and ensure you have read access\n")
		return 2
	}

	if h.isDiskFullError(err) {
		fmt.Fprintf(os.Stderr, "Error: Insufficient disk space\n")
		fmt.Fprintf(os.Stderr, "Suggestion: Free up disk space and try again\n")
		return 2
	}

	// Generic error handling
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	
	if h.verbose {
		fmt.Fprintf(os.Stderr, "\nFor more details, check the logs or run with --verbose flag\n")
	}

	return 1
}

// getCategoryHelp returns category-specific help text
func (h *CLIErrorHandler) getCategoryHelp(category errors.ErrorCategory) string {
	switch category {
	case errors.CategoryFile:
		return `File error help:
• Check if the file exists and is readable
• Verify the file path is correct (use absolute paths if needed)
• Ensure you have proper permissions to access the file
• Try using a different file or contact your system administrator`

	case errors.CategoryParse:
		return `Parse error help:
• Verify the CSV file format and structure
• Check for proper column headers and data types
• Ensure the file uses UTF-8 encoding
• Remove any special characters or formatting from the data
• Use 'reconciler --help' for examples of correct file formats`

	case errors.CategoryValidation:
		return `Validation error help:
• Check that all required fields have values
• Verify date formats use YYYY-MM-DD
• Ensure amounts are decimal numbers without currency symbols
• Check that all values are within acceptable ranges`

	case errors.CategoryConfiguration:
		return `Configuration error help:
• Check your command-line flags and arguments
• Verify configuration file syntax if using --config
• Use 'reconciler reconcile --help' to see all available options
• Try running with default settings first`

	case errors.CategoryReconciliation:
		return `Reconciliation error help:
• Check data quality in your input files
• Try adjusting matching tolerances (--date-tolerance, --amount-tolerance)
• Verify that your files contain matching transactions
• Check for data consistency between system and bank files`

	default:
		return `For more help:
• Use 'reconciler --help' for general help
• Use 'reconciler reconcile --help' for command-specific help
• Check the documentation for detailed examples
• Report bugs or ask for help on the project repository`
	}
}

// Error detection helpers

func (h *CLIErrorHandler) isFileNotFoundError(err error) bool {
	return os.IsNotExist(err) || strings.Contains(err.Error(), "no such file or directory")
}

func (h *CLIErrorHandler) isPermissionError(err error) bool {
	return os.IsPermission(err) || 
		   strings.Contains(err.Error(), "permission denied") ||
		   strings.Contains(err.Error(), "access denied")
}

func (h *CLIErrorHandler) isDiskFullError(err error) bool {
	if err == syscall.ENOSPC {
		return true
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no space left") ||
		   strings.Contains(errStr, "disk full") ||
		   strings.Contains(errStr, "device full")
}

// FormatValidationErrors formats validation errors in a user-friendly way
func FormatValidationErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}

	if len(errs) == 1 {
		return fmt.Sprintf("Validation error: %v", errs[0])
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Found %d validation errors:", len(errs)))

	for i, err := range errs {
		lines = append(lines, fmt.Sprintf("  %d. %v", i+1, err))
		// Limit the number of errors shown
		if i >= 9 && len(errs) > 10 {
			lines = append(lines, fmt.Sprintf("  ... and %d more errors", len(errs)-10))
			break
		}
	}

	return strings.Join(lines, "\n")
}

// FormatFileError formats file-related errors with helpful information
func FormatFileError(filePath string, err error) string {
	baseName := filepath.Base(filePath)
	dir := filepath.Dir(filePath)

	var message strings.Builder
	message.WriteString(fmt.Sprintf("Error with file '%s':\n", baseName))
	message.WriteString(fmt.Sprintf("  Path: %s\n", filePath))
	message.WriteString(fmt.Sprintf("  Error: %v\n", err))

	// Add specific suggestions based on error type
	if os.IsNotExist(err) {
		message.WriteString("  Suggestion: Check if the file exists in the specified location\n")
		
		// Try to suggest similar files in the directory
		if entries, dirErr := os.ReadDir(dir); dirErr == nil {
			var similar []string
			for _, entry := range entries {
				if !entry.IsDir() && strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(baseName[:min(len(baseName), 3)])) {
					similar = append(similar, entry.Name())
				}
			}
			if len(similar) > 0 {
				message.WriteString("  Similar files found:\n")
				for _, name := range similar[:min(len(similar), 3)] {
					message.WriteString(fmt.Sprintf("    - %s\n", name))
				}
			}
		}
	} else if os.IsPermission(err) {
		message.WriteString("  Suggestion: Check file permissions - you may need read access\n")
	}

	return message.String()
}

// ShowProgressError displays errors that occurred during progress tracking
func ShowProgressError(operation string, processed, total int64, err error) {
	fmt.Fprintf(os.Stderr, "\nOperation '%s' failed after processing %d", operation, processed)
	if total > 0 {
		percentage := float64(processed) / float64(total) * 100
		fmt.Fprintf(os.Stderr, "/%d items (%.1f%%)", total, percentage)
	}
	fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
	
	fmt.Fprintf(os.Stderr, "\nPartial results may be available.\n")
	fmt.Fprintf(os.Stderr, "Check the output and consider resuming from where it failed.\n")
}

// SuggestRecoveryActions suggests actions the user can take to recover from errors
func SuggestRecoveryActions(category errors.ErrorCategory) {
	fmt.Fprintf(os.Stderr, "\nRecovery suggestions:\n")
	
	switch category {
	case errors.CategoryFile:
		fmt.Fprintf(os.Stderr, "• Verify file paths and permissions\n")
		fmt.Fprintf(os.Stderr, "• Try copying files to a different location\n")
		fmt.Fprintf(os.Stderr, "• Check available disk space\n")
		
	case errors.CategoryParse:
		fmt.Fprintf(os.Stderr, "• Fix data format issues in the CSV files\n")
		fmt.Fprintf(os.Stderr, "• Remove or correct invalid entries\n")
		fmt.Fprintf(os.Stderr, "• Save files in UTF-8 encoding\n")
		
	case errors.CategoryValidation:
		fmt.Fprintf(os.Stderr, "• Correct the invalid data values\n")
		fmt.Fprintf(os.Stderr, "• Check date and amount formats\n")
		fmt.Fprintf(os.Stderr, "• Ensure all required fields are present\n")
		
	case errors.CategoryConfiguration:
		fmt.Fprintf(os.Stderr, "• Review command-line arguments\n")
		fmt.Fprintf(os.Stderr, "• Check configuration file syntax\n")
		fmt.Fprintf(os.Stderr, "• Try with default settings first\n")
		
	case errors.CategoryReconciliation:
		fmt.Fprintf(os.Stderr, "• Review data quality in input files\n")
		fmt.Fprintf(os.Stderr, "• Adjust matching tolerance settings\n")
		fmt.Fprintf(os.Stderr, "• Verify data consistency\n")
	}
	
	fmt.Fprintf(os.Stderr, "• Use --verbose flag for more detailed error information\n")
	fmt.Fprintf(os.Stderr, "• Check the documentation for examples and troubleshooting\n")
}

// Helper function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}