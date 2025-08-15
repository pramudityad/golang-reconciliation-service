package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// ValidationResult represents the result of validating a file
type ValidationResult struct {
	FilePath    string
	FileType    string // transaction, statement, or unknown
	IsValid     bool
	RecordCount int
	Errors      []ValidationError
	Warnings    []ValidationWarning
	Summary     ValidationSummary
}

// ValidationError represents a validation error
type ValidationError struct {
	Line    int
	Column  string
	Message string
	Value   string
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Line    int
	Column  string
	Message string
	Value   string
}

// ValidationSummary provides aggregate validation statistics
type ValidationSummary struct {
	TotalRecords    int
	ValidRecords    int
	ErrorRecords    int
	WarningRecords  int
	UniqueIDs       int
	DuplicateIDs    int
	AmountRange     AmountRange
	DateRange       DateRange
	TransactionTypes map[string]int
}

// AmountRange represents the range of amounts in the dataset
type AmountRange struct {
	Min decimal.Decimal
	Max decimal.Decimal
	Avg decimal.Decimal
}

// DateRange represents the range of dates in the dataset
type DateRange struct {
	Min time.Time
	Max time.Time
}

func main() {
	var (
		input     = flag.String("input", "", "Input CSV file or directory to validate")
		output    = flag.String("output", "", "Output file for validation report (optional)")
		recursive = flag.Bool("recursive", false, "Recursively validate files in directory")
		verbose   = flag.Bool("verbose", false, "Verbose output")
		strict    = flag.Bool("strict", false, "Strict validation mode (warnings become errors)")
	)
	flag.Parse()

	if *input == "" {
		fmt.Println("CSV Data Validator")
		fmt.Println("==================")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run data_validator.go -input=<file_or_directory> [options]")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("  -input=FILE        Input CSV file or directory")
		fmt.Println("  -output=FILE       Output report file (optional)")
		fmt.Println("  -recursive         Recursively validate directories")
		fmt.Println("  -verbose           Show detailed validation output")
		fmt.Println("  -strict            Treat warnings as errors")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run data_validator.go -input=../csv/system_transactions.csv")
		fmt.Println("  go run data_validator.go -input=../csv -recursive -output=validation_report.txt")
		fmt.Println("  go run data_validator.go -input=generated_data -recursive -verbose")
		return
	}

	validator := &DataValidator{
		Verbose: *verbose,
		Strict:  *strict,
	}

	var results []ValidationResult

	// Check if input is file or directory
	info, err := os.Stat(*input)
	if err != nil {
		log.Fatalf("Cannot access input: %v", err)
	}

	if info.IsDir() {
		results = validator.ValidateDirectory(*input, *recursive)
	} else {
		result := validator.ValidateFile(*input)
		results = []ValidationResult{result}
	}

	// Output results
	validator.PrintResults(results)

	// Write report if requested
	if *output != "" {
		if err := validator.WriteReport(*output, results); err != nil {
			log.Printf("Failed to write report: %v", err)
		} else {
			fmt.Printf("\nValidation report written to: %s\n", *output)
		}
	}

	// Exit with error code if validation failed
	hasErrors := false
	for _, result := range results {
		if !result.IsValid {
			hasErrors = true
			break
		}
	}

	if hasErrors {
		os.Exit(1)
	}
}

// DataValidator performs CSV data validation
type DataValidator struct {
	Verbose bool
	Strict  bool
}

// ValidateDirectory validates all CSV files in a directory
func (dv *DataValidator) ValidateDirectory(dirPath string, recursive bool) []ValidationResult {
	var results []ValidationResult

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			if !recursive && path != dirPath {
				return filepath.SkipDir
			}
			return nil
		}

		// Only validate CSV files
		if strings.ToLower(filepath.Ext(path)) == ".csv" {
			if dv.Verbose {
				fmt.Printf("Validating: %s\n", path)
			}
			result := dv.ValidateFile(path)
			results = append(results, result)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error walking directory: %v", err)
	}

	return results
}

// ValidateFile validates a single CSV file
func (dv *DataValidator) ValidateFile(filePath string) ValidationResult {
	result := ValidationResult{
		FilePath: filePath,
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
		Summary: ValidationSummary{
			TransactionTypes: make(map[string]int),
		},
	}

	// Open and read file
	file, err := os.Open(filePath)
	if err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Message: fmt.Sprintf("Cannot open file: %v", err),
		})
		return result
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Message: fmt.Sprintf("Cannot parse CSV: %v", err),
		})
		return result
	}

	if len(records) < 2 {
		result.Errors = append(result.Errors, ValidationError{
			Line:    0,
			Message: "File must have header and at least one data row",
		})
		return result
	}

	// Determine file type from header
	header := records[0]
	result.FileType = dv.detectFileType(header)

	// Validate based on file type
	switch result.FileType {
	case "transaction":
		dv.validateTransactionFile(records, &result)
	case "statement":
		dv.validateStatementFile(records, &result)
	default:
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    1,
			Message: "Unknown file format, performing basic validation only",
		})
		dv.validateGenericCSV(records, &result)
	}

	// Calculate summary
	result.RecordCount = len(records) - 1 // Exclude header
	result.Summary.TotalRecords = result.RecordCount
	result.Summary.ErrorRecords = len(result.Errors)
	result.Summary.WarningRecords = len(result.Warnings)
	result.Summary.ValidRecords = result.RecordCount - result.Summary.ErrorRecords

	// Determine if file is valid
	result.IsValid = len(result.Errors) == 0
	if dv.Strict && len(result.Warnings) > 0 {
		result.IsValid = false
	}

	return result
}

// detectFileType determines the type of CSV file from the header
func (dv *DataValidator) detectFileType(header []string) string {
	headerStr := strings.ToLower(strings.Join(header, ","))

	// Transaction file patterns
	if strings.Contains(headerStr, "trxid") && strings.Contains(headerStr, "transactiontime") {
		return "transaction"
	}

	// Bank statement patterns
	if (strings.Contains(headerStr, "unique_identifier") || strings.Contains(headerStr, "transaction_id")) &&
		strings.Contains(headerStr, "amount") {
		return "statement"
	}

	return "unknown"
}

// validateTransactionFile validates a transaction CSV file
func (dv *DataValidator) validateTransactionFile(records [][]string, result *ValidationResult) {
	header := records[0]

	// Validate header
	expectedColumns := []string{"trxID", "amount", "type", "transactionTime"}
	if !dv.validateHeader(header, expectedColumns, result) {
		return
	}

	// Track unique IDs and amounts for summary
	idMap := make(map[string]int)
	var amounts []decimal.Decimal
	var dates []time.Time

	// Validate each record
	for i := 1; i < len(records); i++ {
		record := records[i]
		lineNum := i + 1

		if len(record) != len(expectedColumns) {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Message: fmt.Sprintf("Expected %d columns, got %d", len(expectedColumns), len(record)),
			})
			continue
		}

		// Validate transaction ID
		trxID := strings.TrimSpace(record[0])
		if trxID == "" {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  "trxID",
				Message: "Transaction ID cannot be empty",
				Value:   record[0],
			})
		} else {
			if prevLine, exists := idMap[trxID]; exists {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					Column:  "trxID",
					Message: fmt.Sprintf("Duplicate transaction ID (first seen on line %d)", prevLine),
					Value:   trxID,
				})
			} else {
				idMap[trxID] = lineNum
			}
		}

		// Validate amount
		amount, err := decimal.NewFromString(strings.TrimSpace(record[1]))
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  "amount",
				Message: fmt.Sprintf("Invalid amount format: %v", err),
				Value:   record[1],
			})
		} else {
			if amount.IsZero() {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    lineNum,
					Column:  "amount",
					Message: "Amount is zero",
					Value:   record[1],
				})
			}
			if amount.IsNegative() {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    lineNum,
					Column:  "amount",
					Message: "Amount is negative (unusual for transaction files)",
					Value:   record[1],
				})
			}
			amounts = append(amounts, amount)
		}

		// Validate transaction type
		txType := strings.ToUpper(strings.TrimSpace(record[2]))
		if txType != "CREDIT" && txType != "DEBIT" {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  "type",
				Message: "Transaction type must be CREDIT or DEBIT",
				Value:   record[2],
			})
		} else {
			result.Summary.TransactionTypes[txType]++
		}

		// Validate transaction time
		txTime, err := time.Parse(time.RFC3339, strings.TrimSpace(record[3]))
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  "transactionTime",
				Message: fmt.Sprintf("Invalid time format (expected RFC3339): %v", err),
				Value:   record[3],
			})
		} else {
			dates = append(dates, txTime)
		}

		// Validate using models package
		if len(result.Errors) == 0 || dv.Verbose {
			_, err := models.CreateTransactionFromCSV(trxID, record[1], txType, record[3])
			if err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					Message: fmt.Sprintf("Model validation failed: %v", err),
				})
			}
		}
	}

	// Calculate summary statistics
	result.Summary.UniqueIDs = len(idMap)
	result.Summary.DuplicateIDs = result.RecordCount - result.Summary.UniqueIDs

	if len(amounts) > 0 {
		result.Summary.AmountRange = dv.calculateAmountRange(amounts)
	}

	if len(dates) > 0 {
		result.Summary.DateRange = dv.calculateDateRange(dates)
	}
}

// validateStatementFile validates a bank statement CSV file
func (dv *DataValidator) validateStatementFile(records [][]string, result *ValidationResult) {
	header := records[0]

	// Detect statement format
	var format string
	var expectedColumns []string

	headerStr := strings.ToLower(strings.Join(header, ","))
	if strings.Contains(headerStr, "unique_identifier") {
		format = "bank1"
		expectedColumns = []string{"unique_identifier", "amount", "date"}
	} else if strings.Contains(headerStr, "transaction_id") && strings.Contains(headerStr, "posting_date") {
		format = "bank2"
		expectedColumns = []string{"transaction_id", "transaction_amount", "posting_date", "transaction_description"}
	} else {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Line:    1,
			Message: "Unknown statement format, using generic validation",
		})
		dv.validateGenericCSV(records, result)
		return
	}

	// Validate header
	if !dv.validateHeader(header, expectedColumns, result) {
		return
	}

	// Track unique IDs and amounts for summary
	idMap := make(map[string]int)
	var amounts []decimal.Decimal
	var dates []time.Time

	// Validate each record
	for i := 1; i < len(records); i++ {
		record := records[i]
		lineNum := i + 1

		if len(record) != len(expectedColumns) {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Message: fmt.Sprintf("Expected %d columns, got %d", len(expectedColumns), len(record)),
			})
			continue
		}

		// Validate identifier
		identifier := strings.TrimSpace(record[0])
		if identifier == "" {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  expectedColumns[0],
				Message: "Identifier cannot be empty",
				Value:   record[0],
			})
		} else {
			if prevLine, exists := idMap[identifier]; exists {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					Column:  expectedColumns[0],
					Message: fmt.Sprintf("Duplicate identifier (first seen on line %d)", prevLine),
					Value:   identifier,
				})
			} else {
				idMap[identifier] = lineNum
			}
		}

		// Validate amount
		amount, err := decimal.NewFromString(strings.TrimSpace(record[1]))
		if err != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  expectedColumns[1],
				Message: fmt.Sprintf("Invalid amount format: %v", err),
				Value:   record[1],
			})
		} else {
			if amount.IsZero() {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    lineNum,
					Column:  expectedColumns[1],
					Message: "Amount is zero",
					Value:   record[1],
				})
			}
			amounts = append(amounts, amount)

			// Track transaction types based on amount sign
			if amount.IsNegative() {
				result.Summary.TransactionTypes["DEBIT"]++
			} else {
				result.Summary.TransactionTypes["CREDIT"]++
			}
		}

		// Validate date
		var dateFormats []string
		if format == "bank1" {
			dateFormats = []string{"2006-01-02"}
		} else {
			dateFormats = []string{"01/02/2006", "2006-01-02"}
		}

		var parsedDate time.Time
		var parseErr error
		for _, dateFormat := range dateFormats {
			parsedDate, parseErr = time.Parse(dateFormat, strings.TrimSpace(record[2]))
			if parseErr == nil {
				break
			}
		}

		if parseErr != nil {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Column:  expectedColumns[2],
				Message: fmt.Sprintf("Invalid date format: %v", parseErr),
				Value:   record[2],
			})
		} else {
			dates = append(dates, parsedDate)
		}

		// Validate using models package
		if len(result.Errors) == 0 || dv.Verbose {
			_, err := models.CreateBankStatementFromCSV(identifier, record[1], record[2])
			if err != nil {
				result.Errors = append(result.Errors, ValidationError{
					Line:    lineNum,
					Message: fmt.Sprintf("Model validation failed: %v", err),
				})
			}
		}
	}

	// Calculate summary statistics
	result.Summary.UniqueIDs = len(idMap)
	result.Summary.DuplicateIDs = result.RecordCount - result.Summary.UniqueIDs

	if len(amounts) > 0 {
		result.Summary.AmountRange = dv.calculateAmountRange(amounts)
	}

	if len(dates) > 0 {
		result.Summary.DateRange = dv.calculateDateRange(dates)
	}
}

// validateGenericCSV performs basic CSV validation
func (dv *DataValidator) validateGenericCSV(records [][]string, result *ValidationResult) {
	header := records[0]
	expectedColumns := len(header)

	for i := 1; i < len(records); i++ {
		record := records[i]
		lineNum := i + 1

		if len(record) != expectedColumns {
			result.Errors = append(result.Errors, ValidationError{
				Line:    lineNum,
				Message: fmt.Sprintf("Expected %d columns, got %d", expectedColumns, len(record)),
			})
		}

		// Check for empty required fields
		for j, value := range record {
			if strings.TrimSpace(value) == "" {
				result.Warnings = append(result.Warnings, ValidationWarning{
					Line:    lineNum,
					Column:  header[j],
					Message: "Empty field",
					Value:   value,
				})
			}
		}
	}
}

// validateHeader validates CSV header against expected columns
func (dv *DataValidator) validateHeader(header, expected []string, result *ValidationResult) bool {
	if len(header) != len(expected) {
		result.Errors = append(result.Errors, ValidationError{
			Line:    1,
			Message: fmt.Sprintf("Header mismatch: expected %v, got %v", expected, header),
		})
		return false
	}

	for i, expectedCol := range expected {
		if header[i] != expectedCol {
			result.Errors = append(result.Errors, ValidationError{
				Line:    1,
				Column:  header[i],
				Message: fmt.Sprintf("Expected column '%s', got '%s'", expectedCol, header[i]),
			})
		}
	}

	return len(result.Errors) == 0
}

// calculateAmountRange calculates min, max, and average amounts
func (dv *DataValidator) calculateAmountRange(amounts []decimal.Decimal) AmountRange {
	if len(amounts) == 0 {
		return AmountRange{}
	}

	min := amounts[0]
	max := amounts[0]
	sum := decimal.Zero

	for _, amount := range amounts {
		if amount.LessThan(min) {
			min = amount
		}
		if amount.GreaterThan(max) {
			max = amount
		}
		sum = sum.Add(amount)
	}

	avg := sum.Div(decimal.NewFromInt(int64(len(amounts))))

	return AmountRange{
		Min: min,
		Max: max,
		Avg: avg,
	}
}

// calculateDateRange calculates min and max dates
func (dv *DataValidator) calculateDateRange(dates []time.Time) DateRange {
	if len(dates) == 0 {
		return DateRange{}
	}

	min := dates[0]
	max := dates[0]

	for _, date := range dates {
		if date.Before(min) {
			min = date
		}
		if date.After(max) {
			max = date
		}
	}

	return DateRange{
		Min: min,
		Max: max,
	}
}

// PrintResults prints validation results to console
func (dv *DataValidator) PrintResults(results []ValidationResult) {
	fmt.Println("\nValidation Results")
	fmt.Println("==================")

	totalFiles := len(results)
	validFiles := 0
	totalRecords := 0
	totalErrors := 0
	totalWarnings := 0

	for _, result := range results {
		fmt.Printf("\nFile: %s\n", result.FilePath)
		fmt.Printf("Type: %s\n", result.FileType)
		fmt.Printf("Records: %d\n", result.RecordCount)

		if result.IsValid {
			fmt.Printf("Status: ✓ VALID\n")
			validFiles++
		} else {
			fmt.Printf("Status: ✗ INVALID\n")
		}

		if len(result.Errors) > 0 {
			fmt.Printf("Errors: %d\n", len(result.Errors))
			if dv.Verbose {
				for _, err := range result.Errors {
					fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
				}
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Printf("Warnings: %d\n", len(result.Warnings))
			if dv.Verbose {
				for _, warning := range result.Warnings {
					fmt.Printf("  Line %d: %s\n", warning.Line, warning.Message)
				}
			}
		}

		// Print summary if available
		if result.Summary.TotalRecords > 0 {
			fmt.Printf("Summary:\n")
			fmt.Printf("  Unique IDs: %d\n", result.Summary.UniqueIDs)
			if result.Summary.DuplicateIDs > 0 {
				fmt.Printf("  Duplicate IDs: %d\n", result.Summary.DuplicateIDs)
			}
			if !result.Summary.AmountRange.Min.IsZero() {
				fmt.Printf("  Amount Range: %s to %s (avg: %s)\n",
					result.Summary.AmountRange.Min.String(),
					result.Summary.AmountRange.Max.String(),
					result.Summary.AmountRange.Avg.String())
			}
			if !result.Summary.DateRange.Min.IsZero() {
				fmt.Printf("  Date Range: %s to %s\n",
					result.Summary.DateRange.Min.Format("2006-01-02"),
					result.Summary.DateRange.Max.Format("2006-01-02"))
			}
			if len(result.Summary.TransactionTypes) > 0 {
				fmt.Printf("  Transaction Types: ")
				for txType, count := range result.Summary.TransactionTypes {
					fmt.Printf("%s=%d ", txType, count)
				}
				fmt.Println()
			}
		}

		totalRecords += result.RecordCount
		totalErrors += len(result.Errors)
		totalWarnings += len(result.Warnings)
	}

	// Overall summary
	fmt.Printf("\nOverall Summary\n")
	fmt.Printf("===============\n")
	fmt.Printf("Files processed: %d\n", totalFiles)
	fmt.Printf("Valid files: %d\n", validFiles)
	fmt.Printf("Invalid files: %d\n", totalFiles-validFiles)
	fmt.Printf("Total records: %d\n", totalRecords)
	fmt.Printf("Total errors: %d\n", totalErrors)
	fmt.Printf("Total warnings: %d\n", totalWarnings)

	if validFiles == totalFiles {
		fmt.Printf("Result: ✓ ALL FILES VALID\n")
	} else {
		fmt.Printf("Result: ✗ VALIDATION FAILED\n")
	}
}

// WriteReport writes a detailed validation report to file
func (dv *DataValidator) WriteReport(filename string, results []ValidationResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "CSV Data Validation Report\n")
	fmt.Fprintf(file, "==========================\n")
	fmt.Fprintf(file, "Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Write summary
	totalFiles := len(results)
	validFiles := 0
	totalRecords := 0
	totalErrors := 0
	totalWarnings := 0

	for _, result := range results {
		if result.IsValid {
			validFiles++
		}
		totalRecords += result.RecordCount
		totalErrors += len(result.Errors)
		totalWarnings += len(result.Warnings)
	}

	fmt.Fprintf(file, "Summary\n")
	fmt.Fprintf(file, "-------\n")
	fmt.Fprintf(file, "Files processed: %d\n", totalFiles)
	fmt.Fprintf(file, "Valid files: %d\n", validFiles)
	fmt.Fprintf(file, "Invalid files: %d\n", totalFiles-validFiles)
	fmt.Fprintf(file, "Total records: %d\n", totalRecords)
	fmt.Fprintf(file, "Total errors: %d\n", totalErrors)
	fmt.Fprintf(file, "Total warnings: %d\n\n", totalWarnings)

	// Write detailed results
	for _, result := range results {
		fmt.Fprintf(file, "File: %s\n", result.FilePath)
		fmt.Fprintf(file, "Type: %s\n", result.FileType)
		fmt.Fprintf(file, "Records: %d\n", result.RecordCount)
		fmt.Fprintf(file, "Status: %s\n", map[bool]string{true: "VALID", false: "INVALID"}[result.IsValid])

		if len(result.Errors) > 0 {
			fmt.Fprintf(file, "\nErrors (%d):\n", len(result.Errors))
			for _, err := range result.Errors {
				fmt.Fprintf(file, "  Line %d", err.Line)
				if err.Column != "" {
					fmt.Fprintf(file, ", Column %s", err.Column)
				}
				fmt.Fprintf(file, ": %s", err.Message)
				if err.Value != "" {
					fmt.Fprintf(file, " (Value: %s)", err.Value)
				}
				fmt.Fprintf(file, "\n")
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Fprintf(file, "\nWarnings (%d):\n", len(result.Warnings))
			for _, warning := range result.Warnings {
				fmt.Fprintf(file, "  Line %d", warning.Line)
				if warning.Column != "" {
					fmt.Fprintf(file, ", Column %s", warning.Column)
				}
				fmt.Fprintf(file, ": %s", warning.Message)
				if warning.Value != "" {
					fmt.Fprintf(file, " (Value: %s)", warning.Value)
				}
				fmt.Fprintf(file, "\n")
			}
		}

		// Write summary statistics
		if result.Summary.TotalRecords > 0 {
			fmt.Fprintf(file, "\nStatistics:\n")
			fmt.Fprintf(file, "  Unique IDs: %d\n", result.Summary.UniqueIDs)
			if result.Summary.DuplicateIDs > 0 {
				fmt.Fprintf(file, "  Duplicate IDs: %d\n", result.Summary.DuplicateIDs)
			}
			if !result.Summary.AmountRange.Min.IsZero() {
				fmt.Fprintf(file, "  Amount Range: %s to %s (avg: %s)\n",
					result.Summary.AmountRange.Min.String(),
					result.Summary.AmountRange.Max.String(),
					result.Summary.AmountRange.Avg.String())
			}
			if !result.Summary.DateRange.Min.IsZero() {
				fmt.Fprintf(file, "  Date Range: %s to %s\n",
					result.Summary.DateRange.Min.Format("2006-01-02"),
					result.Summary.DateRange.Max.Format("2006-01-02"))
			}
			if len(result.Summary.TransactionTypes) > 0 {
				fmt.Fprintf(file, "  Transaction Types:\n")
				for txType, count := range result.Summary.TransactionTypes {
					fmt.Fprintf(file, "    %s: %d\n", txType, count)
				}
			}
		}

		fmt.Fprintf(file, "\n%s\n\n", strings.Repeat("-", 80))
	}

	return nil
}