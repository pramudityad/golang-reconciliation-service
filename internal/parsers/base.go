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
}

// NewBaseParser creates a new BaseParser with the given configuration
func NewBaseParser(config *ParseConfig) *BaseParser {
	if config == nil {
		config = DefaultParseConfig()
	}
	return &BaseParser{
		config: config,
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
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file '%s': %w", filePath, err)
	}
	
	// Validate encoding if required
	if bp.config.ValidateEncoding {
		if err := bp.validateEncoding(file); err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("encoding validation failed for '%s': %w", filePath, err)
		}
		
		// Seek back to beginning after validation
		if _, err := file.Seek(0, 0); err != nil {
			file.Close()
			return nil, nil, fmt.Errorf("failed to seek to beginning of file '%s': %w", filePath, err)
		}
	}
	
	reader := csv.NewReader(file)
	bp.configureReader(reader)
	
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
func (bp *BaseParser) validateEncoding(file *os.File) error {
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() && lineNum < 100 { // Check first 100 lines
		lineNum++
		if !utf8.Valid(scanner.Bytes()) {
			return fmt.Errorf("invalid UTF-8 encoding detected at line %d", lineNum)
		}
	}
	
	return scanner.Err()
}

// ReadHeaders reads and validates the header row
func (bp *BaseParser) ReadHeaders(reader *csv.Reader, parseCtx *ParseContext, requiredHeaders []string) error {
	if !bp.config.HasHeader {
		// Generate default headers if no header row
		if len(requiredHeaders) > 0 {
			parseCtx.Headers = make([]string, len(requiredHeaders))
			copy(parseCtx.Headers, requiredHeaders)
		}
		bp.buildHeaderMap(parseCtx)
		return nil
	}
	
	// Read header row
	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return fmt.Errorf("file is empty or contains only headers")
		}
		return fmt.Errorf("failed to read headers: %w", err)
	}
	
	parseCtx.LineNumber++
	parseCtx.Headers = bp.cleanHeaders(headers)
	bp.buildHeaderMap(parseCtx)
	
	// Validate required headers are present
	if len(requiredHeaders) > 0 {
		missing := bp.findMissingHeaders(parseCtx, requiredHeaders)
		if len(missing) > 0 {
			return fmt.Errorf("missing required headers: %s", strings.Join(missing, ", "))
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
			return nil, fmt.Errorf("parsing cancelled")
		}
		
		record, err := reader.Read()
		if err != nil {
			return nil, err // EOF or other read error
		}
		
		parseCtx.LineNumber++
		
		// Skip empty rows if configured
		if bp.config.SkipEmptyRows && bp.isEmptyRecord(record) {
			continue
		}
		
		// Validate field sizes if configured
		if bp.config.MaxFieldSize > 0 {
			for i, field := range record {
				if len(field) > bp.config.MaxFieldSize {
					parseCtx.AddError(i, fmt.Sprintf("field_%d", i), field[:50]+"...", 
						fmt.Sprintf("field exceeds maximum size of %d bytes", bp.config.MaxFieldSize), nil)
					return nil, fmt.Errorf("field size limit exceeded at line %d, column %d", 
						parseCtx.LineNumber, i)
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
		return "", fmt.Errorf("field '%s' not found in headers", fieldName)
	}
	
	if index >= len(record) {
		return "", fmt.Errorf("field '%s' (index %d) not present in record with %d fields", 
			fieldName, index, len(record))
	}
	
	return strings.TrimSpace(record[index]), nil
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