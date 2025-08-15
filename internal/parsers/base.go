// Package parsers provides robust CSV parsing capabilities for financial data.
//
// This package handles the complexities of parsing real-world financial CSV files,
// including various formats, error handling, streaming for large files, and
// concurrent processing capabilities.
//
// Key features:
//   - Configurable CSV parsing for different bank and system formats
//   - Streaming parsers for memory-efficient processing of large files
//   - Concurrent parsing for multiple files
//   - Comprehensive error handling and validation
//   - Progress reporting for long-running operations
//   - Memory monitoring and management
//
// Parser Types:
//   - TransactionParser: for internal system transaction files
//   - BankStatementParser: for external bank statement files
//   - StreamingTransactionParser: memory-efficient version for large files
//   - StreamingBankStatementParser: memory-efficient version for large files
//   - ConcurrentParser: for processing multiple files simultaneously
//
// Example usage:
//
//	// Basic parsing
//	config := &TransactionParserConfig{HasHeader: true}
//	parser, err := NewTransactionParser(config)
//	transactions, stats, err := parser.ParseTransactions("transactions.csv")
//	
//	// Streaming for large files
//	streamConfig := DefaultStreamingConfig()
//	streamParser, err := NewStreamingTransactionParser(config, streamConfig)
//	err = streamParser.ParseTransactionsStream(ctx, "large_file.csv", batchSize, callback)
//
// The package handles common CSV variations found in financial systems:
//   - Different date formats (ISO, US, European)
//   - Various amount representations (with/without currency symbols)
//   - Different transaction type encodings
//   - Header presence/absence variations
//   - Encoding issues (UTF-8, Latin-1)
package parsers

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang-reconciliation-service/pkg/errors"
	"golang-reconciliation-service/pkg/logger"
)

// ParseError represents an error that occurred during CSV parsing
type ParseError struct {
	Line    int
	Column  int
	Field   string
	Value   string
	Message string
	Err     error
}

func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("parse error at line %d, column %d (%s='%s'): %s: %v",
			e.Line, e.Column, e.Field, e.Value, e.Message, e.Err)
	}
	return fmt.Sprintf("parse error at line %d, column %d (%s='%s'): %s",
		e.Line, e.Column, e.Field, e.Value, e.Message)
}

func (e *ParseError) Unwrap() error {
	return e.Err
}

// ValidationError represents a validation error for a specific record
type ValidationError struct {
	Line   int
	Record interface{}
	Errors []error
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("validation error at line %d: %v", e.Line, e.Errors[0])
	}
	
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("validation errors at line %d: %s", e.Line, strings.Join(msgs, "; "))
}

// ParseConfig holds configuration for CSV parsing
type ParseConfig struct {
	HasHeader         bool
	Delimiter         rune
	Comment           rune
	TrimLeadingSpace  bool
	SkipEmptyRows     bool
	MaxFieldSize      int
	ValidateEncoding  bool
}

// DefaultParseConfig returns a configuration with sensible defaults
func DefaultParseConfig() *ParseConfig {
	return &ParseConfig{
		HasHeader:         true,
		Delimiter:         ',',
		Comment:           0,
		TrimLeadingSpace:  true,
		SkipEmptyRows:     true,
		MaxFieldSize:      1000000, // 1MB per field
		ValidateEncoding:  true,
	}
}

// BaseParser provides common CSV parsing functionality
type BaseParser struct {
	config *ParseConfig
	logger logger.Logger
}

// NewBaseParser creates a new BaseParser with the given configuration
func NewBaseParser(config *ParseConfig) *BaseParser {
	if config == nil {
		config = DefaultParseConfig()
	}
	
	log := logger.GetGlobalLogger().WithComponent("base_parser")
	log.WithFields(logger.Fields{
		"has_header":        config.HasHeader,
		"delimiter":         string(config.Delimiter),
		"validate_encoding": config.ValidateEncoding,
		"max_field_size":    config.MaxFieldSize,
	}).Debug("Created base parser")
	
	return &BaseParser{
		config: config,
		logger: log,
	}
}

// ParseContext holds state during parsing operations
type ParseContext struct {
	LineNumber   int
	Headers      []string
	HeaderMap    map[string]int
	RecordCount  int
	ErrorCount   int
	Errors       []*ParseError
	ctx          context.Context
}

// NewParseContext creates a new parsing context
func NewParseContext(ctx context.Context) *ParseContext {
	if ctx == nil {
		ctx = context.Background()
	}
	return &ParseContext{
		LineNumber:  0,
		Headers:     make([]string, 0),
		HeaderMap:   make(map[string]int),
		RecordCount: 0,
		ErrorCount:  0,
		Errors:      make([]*ParseError, 0),
		ctx:         ctx,
	}
}

// IsCancelled checks if the parsing context has been cancelled
func (pc *ParseContext) IsCancelled() bool {
	select {
	case <-pc.ctx.Done():
		return true
	default:
		return false
	}
}

// AddError adds a parsing error to the context
func (pc *ParseContext) AddError(column int, field, value, message string, err error) {
	parseErr := &ParseError{
		Line:    pc.LineNumber,
		Column:  column,
		Field:   field,
		Value:   value,
		Message: message,
		Err:     err,
	}
	pc.Errors = append(pc.Errors, parseErr)
	pc.ErrorCount++
}

// GetColumnIndex returns the index of a column by name, or -1 if not found
func (pc *ParseContext) GetColumnIndex(name string) int {
	if index, exists := pc.HeaderMap[name]; exists {
		return index
	}
	
	// Try case-insensitive lookup
	lowerName := strings.ToLower(name)
	for header, index := range pc.HeaderMap {
		if strings.ToLower(header) == lowerName {
			return index
		}
	}
	
	return -1
}

// OpenFile opens a CSV file and returns a csv.Reader
func (bp *BaseParser) OpenFile(filePath string) (*os.File, *csv.Reader, error) {
	bp.logger.WithField("file_path", filePath).Debug("Opening CSV file")
	
	file, err := os.Open(filePath)
	if err != nil {
		bp.logger.WithError(err).WithField("file_path", filePath).Error("Failed to open CSV file")
		
		// Determine the specific type of file error
		if os.IsNotExist(err) {
			return nil, nil, errors.FileError(errors.CodeFileNotFound, filePath, err)
		}
		if os.IsPermission(err) {
			return nil, nil, errors.FileError(errors.CodeFilePermission, filePath, err)
		}
		
		// Generic file error
		return nil, nil, errors.FileError(errors.CodeDirectoryError, filePath, err)
	}
	
	// Validate encoding if required
	if bp.config.ValidateEncoding {
		bp.logger.WithField("file_path", filePath).Debug("Validating file encoding")
		
		if err := bp.validateEncoding(file, filePath); err != nil {
			file.Close()
			bp.logger.WithError(err).WithField("file_path", filePath).Error("File encoding validation failed")
			return nil, nil, err // Already wrapped by validateEncoding
		}
		
		// Seek back to beginning after validation
		if _, err := file.Seek(0, 0); err != nil {
			file.Close()
			bp.logger.WithError(err).WithField("file_path", filePath).Error("Failed to seek to beginning of file")
			return nil, nil, errors.FileError(errors.CodeFileCorrupted, filePath, err)
		}
	}
	
	reader := csv.NewReader(file)
	bp.configureReader(reader)
	
	bp.logger.WithField("file_path", filePath).Debug("Successfully opened CSV file")
	return file, reader, nil
}

// configureReader sets up the CSV reader with our configuration
func (bp *BaseParser) configureReader(reader *csv.Reader) {
	reader.Comma = bp.config.Delimiter
	reader.Comment = bp.config.Comment
	reader.TrimLeadingSpace = bp.config.TrimLeadingSpace
	reader.FieldsPerRecord = -1 // Variable number of fields
	
	// Set field size limit if specified
	if bp.config.MaxFieldSize > 0 {
		// Note: csv.Reader doesn't have a direct field size limit,
		// but we can implement this during field processing
	}
}

// validateEncoding checks if the file contains valid UTF-8 text
func (bp *BaseParser) validateEncoding(file *os.File, filePath string) error {
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() && lineNum < 100 { // Check first 100 lines
		lineNum++
		if !utf8.Valid(scanner.Bytes()) {
			return errors.ParseError(
				errors.CodeEncodingError,
				filePath,
				lineNum,
				"encoding",
				"",
				fmt.Errorf("invalid UTF-8 encoding detected"),
			).WithSuggestion("Save the file in UTF-8 encoding and try again")
		}
	}
	
	if err := scanner.Err(); err != nil {
		return errors.FileError(errors.CodeFileCorrupted, filePath, err)
	}
	
	return nil
}

// ReadHeaders reads and validates the header row
func (bp *BaseParser) ReadHeaders(reader *csv.Reader, parseCtx *ParseContext, requiredHeaders []string) error {
	bp.logger.WithFields(logger.Fields{
		"has_header":        bp.config.HasHeader,
		"required_headers":  requiredHeaders,
	}).Debug("Reading CSV headers")
	
	if !bp.config.HasHeader {
		// Generate default headers if no header row
		if len(requiredHeaders) > 0 {
			parseCtx.Headers = make([]string, len(requiredHeaders))
			copy(parseCtx.Headers, requiredHeaders)
			bp.logger.WithField("default_headers", parseCtx.Headers).Debug("Using default headers")
		}
		bp.buildHeaderMap(parseCtx)
		return nil
	}
	
	// Read header row
	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			bp.logger.Error("File is empty or contains no data")
			return errors.ValidationError(
				errors.CodeMissingField,
				"file_content",
				"empty",
				nil,
			).WithSuggestion("Ensure the file contains header and data rows")
		}
		
		bp.logger.WithError(err).Error("Failed to read header row")
		return errors.ParseError(
			errors.CodeInvalidFormat,
			"",
			1,
			"headers",
			"",
			err,
		).WithSuggestion("Check the file format and ensure it's a valid CSV")
	}
	
	parseCtx.LineNumber++
	parseCtx.Headers = bp.cleanHeaders(headers)
	bp.buildHeaderMap(parseCtx)
	
	bp.logger.WithField("headers", parseCtx.Headers).Debug("Successfully read headers")
	
	// Validate required headers are present
	if len(requiredHeaders) > 0 {
		missing := bp.findMissingHeaders(parseCtx, requiredHeaders)
		if len(missing) > 0 {
			bp.logger.WithFields(logger.Fields{
				"missing_headers":   missing,
				"available_headers": parseCtx.Headers,
			}).Error("Required headers are missing")
			
			return errors.ParseError(
				errors.CodeMissingColumn,
				"",
				parseCtx.LineNumber,
				"headers",
				strings.Join(missing, ", "),
				nil,
			).WithSuggestion(fmt.Sprintf("Ensure the CSV file contains these headers: %s", strings.Join(missing, ", ")))
		}
	}
	
	return nil
}

// cleanHeaders removes whitespace and normalizes header names
func (bp *BaseParser) cleanHeaders(headers []string) []string {
	cleaned := make([]string, len(headers))
	for i, header := range headers {
		cleaned[i] = strings.TrimSpace(header)
	}
	return cleaned
}

// buildHeaderMap creates a map from header names to column indices
func (bp *BaseParser) buildHeaderMap(parseCtx *ParseContext) {
	parseCtx.HeaderMap = make(map[string]int)
	for i, header := range parseCtx.Headers {
		parseCtx.HeaderMap[header] = i
	}
}

// findMissingHeaders returns a list of required headers that are not present
func (bp *BaseParser) findMissingHeaders(parseCtx *ParseContext, required []string) []string {
	var missing []string
	for _, header := range required {
		if parseCtx.GetColumnIndex(header) == -1 {
			missing = append(missing, header)
		}
	}
	return missing
}

// ReadRecord reads and validates a single CSV record
func (bp *BaseParser) ReadRecord(reader *csv.Reader, parseCtx *ParseContext) ([]string, error) {
	for {
		if parseCtx.IsCancelled() {
			bp.logger.Debug("Record reading cancelled by context")
			return nil, errors.InternalError(
				errors.CodeUnexpectedError,
				"csv_parsing",
				fmt.Errorf("parsing cancelled"),
			)
		}
		
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				return nil, err // Normal end of file
			}
			
			bp.logger.WithError(err).WithField("line_number", parseCtx.LineNumber+1).Warn("Failed to read CSV record")
			return nil, err // Pass through other read errors
		}
		
		parseCtx.LineNumber++
		
		// Skip empty rows if configured
		if bp.config.SkipEmptyRows && bp.isEmptyRecord(record) {
			bp.logger.WithField("line_number", parseCtx.LineNumber).Debug("Skipping empty record")
			continue
		}
		
		// Validate field sizes if configured
		if bp.config.MaxFieldSize > 0 {
			for i, field := range record {
				if len(field) > bp.config.MaxFieldSize {
					bp.logger.WithFields(logger.Fields{
						"line_number": parseCtx.LineNumber,
						"column":      i,
						"field_size":  len(field),
						"max_size":    bp.config.MaxFieldSize,
					}).Warn("Field exceeds maximum size limit")
					
					parseCtx.AddError(i, fmt.Sprintf("field_%d", i), field[:50]+"...", 
						fmt.Sprintf("field exceeds maximum size of %d bytes", bp.config.MaxFieldSize), nil)
					
					return nil, errors.ParseError(
						errors.CodeInvalidData,
						"",
						parseCtx.LineNumber,
						fmt.Sprintf("field_%d", i),
						field[:50]+"...",
						fmt.Errorf("field size limit exceeded"),
					).WithSuggestion(fmt.Sprintf("Reduce field size to under %d bytes", bp.config.MaxFieldSize))
				}
			}
		}
		
		return record, nil
	}
}

// isEmptyRecord checks if all fields in a record are empty or whitespace
func (bp *BaseParser) isEmptyRecord(record []string) bool {
	for _, field := range record {
		if strings.TrimSpace(field) != "" {
			return false
		}
	}
	return true
}

// GetFieldValue safely retrieves a field value by name
func (bp *BaseParser) GetFieldValue(record []string, parseCtx *ParseContext, fieldName string) (string, error) {
	index := parseCtx.GetColumnIndex(fieldName)
	if index == -1 {
		bp.logger.WithFields(logger.Fields{
			"field_name":       fieldName,
			"available_headers": parseCtx.Headers,
		}).Debug("Field not found in headers")
		
		return "", errors.ParseError(
			errors.CodeMissingColumn,
			"",
			parseCtx.LineNumber,
			fieldName,
			"",
			fmt.Errorf("field '%s' not found in headers", fieldName),
		).WithSuggestion(fmt.Sprintf("Check the CSV headers. Available headers: %v", parseCtx.Headers))
	}
	
	if index >= len(record) {
		bp.logger.WithFields(logger.Fields{
			"field_name":   fieldName,
			"field_index":  index,
			"record_length": len(record),
			"line_number":  parseCtx.LineNumber,
		}).Warn("Field index exceeds record length")
		
		return "", errors.ParseError(
			errors.CodeInvalidData,
			"",
			parseCtx.LineNumber,
			fieldName,
			"",
			fmt.Errorf("field '%s' (index %d) not present in record with %d fields", fieldName, index, len(record)),
		).WithSuggestion("Check that all rows have the same number of columns as the header")
	}
	
	value := strings.TrimSpace(record[index])
	return value, nil
}

// ParseStats holds statistics about a parsing operation
type ParseStats struct {
	TotalLines    int
	RecordsParsed int
	RecordsValid  int
	ErrorCount    int
	Errors        []*ParseError
}

// NewParseStats creates a new ParseStats instance
func NewParseStats() *ParseStats {
	return &ParseStats{
		Errors: make([]*ParseError, 0),
	}
}

// AddError adds an error to the parsing statistics
func (ps *ParseStats) AddError(err *ParseError) {
	ps.Errors = append(ps.Errors, err)
	ps.ErrorCount++
}

// HasErrors returns true if there were any parsing errors
func (ps *ParseStats) HasErrors() bool {
	return ps.ErrorCount > 0
}

// String returns a human-readable summary of parsing statistics
func (ps *ParseStats) String() string {
	return fmt.Sprintf("Parsed %d lines, %d records (%d valid), %d errors",
		ps.TotalLines, ps.RecordsParsed, ps.RecordsValid, ps.ErrorCount)
}

// GetSampleErrors returns a sample of the parsing errors for logging/debugging
func (ps *ParseStats) GetSampleErrors(maxSamples int) []string {
	if len(ps.Errors) == 0 {
		return nil
	}
	
	var samples []string
	limit := len(ps.Errors)
	if maxSamples > 0 && maxSamples < limit {
		limit = maxSamples
	}
	
	for i := 0; i < limit; i++ {
		samples = append(samples, ps.Errors[i].Error())
	}
	
	return samples
}