package reporter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang-reconciliation-service/internal/reconciler"
	"golang-reconciliation-service/pkg/errors"
	"golang-reconciliation-service/pkg/logger"
)

// SafeReportGenerator wraps ReportGenerator with enhanced error handling
type SafeReportGenerator struct {
	*ReportGenerator
	logger logger.Logger
}

// NewSafeReportGenerator creates a new safe report generator with error handling
func NewSafeReportGenerator(config *ReportConfig, log logger.Logger) (*SafeReportGenerator, error) {
	if log == nil {
		log = logger.GetGlobalLogger()
	}

	generator, err := NewReportGenerator(config)
	if err != nil {
		return nil, errors.ConfigurationError(
			errors.CodeInvalidConfig,
			"report_config",
			config,
			err,
		).WithSuggestion("Check the report configuration values")
	}

	return &SafeReportGenerator{
		ReportGenerator: generator,
		logger:          log.WithComponent("reporter"),
	}, nil
}

// GenerateReportSafely generates a report with comprehensive error handling and fallbacks
func (srg *SafeReportGenerator) GenerateReportSafely(result interface{}, writer io.Writer) error {
	// Log operation start
	srg.logger.WithFields(logger.Fields{
		"format": srg.config.Format,
		"output": getWriterDescription(writer),
	}).Info("Starting report generation")

	// Validate inputs
	if err := srg.validateInputs(result, writer); err != nil {
		srg.logger.WithError(err).Error("Report generation failed: input validation")
		return err
	}

	// Cast result to expected type
	reconcilerResult, ok := result.(*reconciler.ReconciliationResult)
	if !ok {
		err := errors.ValidationError(
			errors.CodeInvalidData,
			"result_type",
			fmt.Sprintf("%T", result),
			nil,
		).WithSuggestion("Provide a valid ReconciliationResult")
		
		srg.logger.WithError(err).Error("Invalid result type for report generation")
		return err
	}

	// Generate report with error handling
	err := srg.generateWithFallback(reconcilerResult, writer)
	if err != nil {
		srg.logger.WithError(err).Error("Report generation failed")
		return err
	}

	srg.logger.Info("Report generation completed successfully")
	return nil
}

// validateInputs validates the inputs for report generation
func (srg *SafeReportGenerator) validateInputs(result interface{}, writer io.Writer) error {
	if result == nil {
		return errors.ValidationError(
			errors.CodeMissingField,
			"result",
			nil,
			nil,
		).WithSuggestion("Provide a valid reconciliation result")
	}

	if writer == nil {
		return errors.ValidationError(
			errors.CodeMissingField,
			"writer",
			nil,
			nil,
		).WithSuggestion("Provide a valid output writer")
	}

	return nil
}

// generateWithFallback attempts to generate the report with fallback strategies
func (srg *SafeReportGenerator) generateWithFallback(result *reconciler.ReconciliationResult, writer io.Writer) error {
	// Try primary generation method
	err := srg.GenerateReport(result, writer)
	if err == nil {
		return nil
	}

	// Log the primary error
	srg.logger.WithError(err).Warn("Primary report generation failed, attempting fallback")

	// Check if it's a format-specific error and try fallback format
	if srg.shouldAttemptFormatFallback(err) {
		return srg.generateWithFormatFallback(result, writer, err)
	}

	// Check if it's an output error and try alternative output
	if srg.shouldAttemptOutputFallback(err, writer) {
		return srg.generateWithOutputFallback(result, writer, err)
	}

	// If no fallback is possible, wrap and return the original error
	return srg.wrapGenerationError(err)
}

// shouldAttemptFormatFallback determines if a format fallback should be attempted
func (srg *SafeReportGenerator) shouldAttemptFormatFallback(err error) bool {
	// Try fallback for JSON/CSV format errors
	return srg.config.Format != FormatConsole
}

// generateWithFormatFallback attempts to generate with a fallback format
func (srg *SafeReportGenerator) generateWithFormatFallback(result *reconciler.ReconciliationResult, writer io.Writer, originalErr error) error {
	// Create fallback config with console format
	fallbackConfig := *srg.config
	fallbackConfig.Format = FormatConsole
	
	srg.logger.WithField("fallback_format", FormatConsole).Info("Attempting format fallback")

	// Create temporary generator with fallback config
	fallbackGenerator, err := NewReportGenerator(&fallbackConfig)
	if err != nil {
		return srg.wrapGenerationError(originalErr)
	}

	// Add fallback notice to the output
	fmt.Fprintf(writer, "NOTE: Report generated in fallback format due to error with requested format\n")
	fmt.Fprintf(writer, "Original error: %v\n\n", originalErr)

	// Generate with fallback format
	if err := fallbackGenerator.GenerateReport(result, writer); err != nil {
		return errors.InternalError(
			errors.CodeUnexpectedError,
			"report_fallback",
			fmt.Errorf("both primary and fallback generation failed: primary=%v, fallback=%v", originalErr, err),
		)
	}

	srg.logger.Info("Report generated successfully using format fallback")
	return nil
}

// shouldAttemptOutputFallback determines if an output fallback should be attempted
func (srg *SafeReportGenerator) shouldAttemptOutputFallback(err error, writer io.Writer) bool {
	// Check if the writer is a file (has a name) and the error is file-related
	if file, ok := writer.(*os.File); ok && file.Name() != "" {
		return srg.isFileError(err)
	}
	return false
}

// generateWithOutputFallback attempts to generate with a fallback output
func (srg *SafeReportGenerator) generateWithOutputFallback(result *reconciler.ReconciliationResult, writer io.Writer, originalErr error) error {
	file, ok := writer.(*os.File)
	if !ok {
		return srg.wrapGenerationError(originalErr)
	}

	// Try creating a backup file
	originalPath := file.Name()
	backupPath := srg.generateBackupPath(originalPath)
	
	srg.logger.WithFields(logger.Fields{
		"original_file": originalPath,
		"backup_file":   backupPath,
	}).Info("Attempting output fallback")

	backupFile, err := os.Create(backupPath)
	if err != nil {
		return srg.wrapGenerationError(originalErr)
	}
	defer backupFile.Close()

	// Add fallback notice
	fmt.Fprintf(backupFile, "NOTE: Report saved to backup location due to error with original output\n")
	fmt.Fprintf(backupFile, "Original file: %s\n", originalPath)
	fmt.Fprintf(backupFile, "Original error: %v\n\n", originalErr)

	// Generate to backup file
	if err := srg.GenerateReport(result, backupFile); err != nil {
		return errors.InternalError(
			errors.CodeUnexpectedError,
			"report_output_fallback",
			fmt.Errorf("both primary and backup output failed: primary=%v, backup=%v", originalErr, err),
		)
	}

	srg.logger.WithField("backup_file", backupPath).Info("Report generated successfully using output fallback")
	
	// Also try to write error message to stderr for user awareness
	fmt.Fprintf(os.Stderr, "Warning: Could not write to %s, report saved to %s\n", originalPath, backupPath)
	
	return nil
}

// isFileError checks if the error is file-related
func (srg *SafeReportGenerator) isFileError(err error) bool {
	return os.IsPermission(err) || 
		   os.IsNotExist(err) || 
		   os.IsExist(err) ||
		   isSpaceError(err)
}

// generateBackupPath creates a backup file path
func (srg *SafeReportGenerator) generateBackupPath(originalPath string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := base[:len(base)-len(ext)]
	
	return filepath.Join(dir, fmt.Sprintf("%s_backup%s", name, ext))
}

// wrapGenerationError wraps generation errors with context
func (srg *SafeReportGenerator) wrapGenerationError(err error) error {
	if reconcilerErr, ok := errors.AsReconcilerError(err); ok {
		return reconcilerErr
	}

	return errors.InternalError(
		errors.CodeProcessingError,
		"report_generation",
		err,
	).WithSuggestion("Check the output destination and report format settings")
}

// Validation methods for different report formats

// ValidateJSONOutput validates that JSON output can be generated
func (srg *SafeReportGenerator) ValidateJSONOutput(result *reconciler.ReconciliationResult) error {
	if result == nil {
		return errors.ValidationError(errors.CodeMissingField, "result", nil, nil)
	}

	// Check for any issues that might prevent JSON serialization
	if result.Summary == nil {
		return errors.ValidationError(
			errors.CodeMissingField, 
			"summary", 
			nil, 
			nil,
		).WithSuggestion("Ensure the reconciliation result includes a summary")
	}

	return nil
}

// ValidateCSVOutput validates that CSV output can be generated
func (srg *SafeReportGenerator) ValidateCSVOutput(result *reconciler.ReconciliationResult) error {
	if result == nil {
		return errors.ValidationError(errors.CodeMissingField, "result", nil, nil)
	}

	// Check that we have data to write to CSV
	if len(result.MatchedTransactions) == 0 && len(result.UnmatchedTransactions) == 0 && len(result.UnmatchedStatements) == 0 {
		srg.logger.Warn("No transaction data available for CSV output")
	}

	return nil
}

// ValidateConsoleOutput validates that console output can be generated
func (srg *SafeReportGenerator) ValidateConsoleOutput(result *reconciler.ReconciliationResult) error {
	if result == nil {
		return errors.ValidationError(errors.CodeMissingField, "result", nil, nil)
	}

	// Console output is the most flexible, so minimal validation needed
	return nil
}

// Utility functions

func getWriterDescription(writer io.Writer) string {
	switch w := writer.(type) {
	case *os.File:
		if w.Name() != "" {
			return fmt.Sprintf("file:%s", w.Name())
		}
		return "file:unnamed"
	default:
		return fmt.Sprintf("writer:%T", writer)
	}
}

func isSpaceError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "no space left") || 
		   contains(errStr, "disk full") || 
		   contains(errStr, "device full")
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > len(substr) && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || containsSubstring(str, substr)))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}