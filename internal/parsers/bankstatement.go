package parsers

import (
	"context"
	"fmt"
	"io"
	"strings"

	"golang-reconciliation-service/internal/models"
)

// BankStatementParser handles parsing of bank statement CSV files with multi-format support
type BankStatementParser struct {
	*BaseParser
	bankConfig *BankConfig
}

// NewBankStatementParser creates a new BankStatementParser with the given bank configuration
func NewBankStatementParser(bankConfig *BankConfig) (*BankStatementParser, error) {
	if bankConfig == nil {
		bankConfig = StandardBankConfig
	}
	
	if err := bankConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bank configuration: %w", err)
	}
	
	parseConfig := &ParseConfig{
		HasHeader:         bankConfig.HasHeader,
		Delimiter:         bankConfig.Delimiter,
		Comment:           0,
		TrimLeadingSpace:  true,
		SkipEmptyRows:     true,
		MaxFieldSize:      1000000,
		ValidateEncoding:  true,
	}
	
	return &BankStatementParser{
		BaseParser: NewBaseParser(parseConfig),
		bankConfig: bankConfig,
	}, nil
}

// ParseBankStatements parses a CSV file containing bank statements
func (bsp *BankStatementParser) ParseBankStatements(filePath string) ([]*models.BankStatement, *ParseStats, error) {
	return bsp.ParseBankStatementsWithContext(context.Background(), filePath)
}

// ParseBankStatementsWithContext parses bank statements with cancellation support
func (bsp *BankStatementParser) ParseBankStatementsWithContext(ctx context.Context, filePath string) ([]*models.BankStatement, *ParseStats, error) {
	file, reader, err := bsp.OpenFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(ctx)
	stats := NewParseStats()
	
	// Read headers
	requiredHeaders := bsp.getRequiredHeaders()
	if err := bsp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return nil, stats, fmt.Errorf("failed to read headers: %w", err)
	}
	
	var bankStatements []*models.BankStatement
	
	// Parse records
	for {
		if parseCtx.IsCancelled() {
			return bankStatements, stats, fmt.Errorf("parsing cancelled")
		}
		
		record, err := bsp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				break
			}
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "failed to read record",
				Err:     err,
			})
			continue
		}
		
		stats.RecordsParsed++
		
		// Parse bank statement from record
		bankStatement, parseErr := bsp.parseBankStatementFromRecord(record, parseCtx)
		if parseErr != nil {
			stats.AddError(parseErr)
			continue
		}
		
		// Validate bank statement
		if err := bankStatement.Validate(); err != nil {
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "bank statement validation failed",
				Err:     err,
			})
			continue
		}
		
		bankStatements = append(bankStatements, bankStatement)
		stats.RecordsValid++
	}
	
	stats.TotalLines = parseCtx.LineNumber
	
	return bankStatements, stats, nil
}

// getRequiredHeaders returns the list of required header names for the configured bank
func (bsp *BankStatementParser) getRequiredHeaders() []string {
	return []string{
		bsp.bankConfig.GetColumnName("identifier"),
		bsp.bankConfig.GetColumnName("amount"),
		bsp.bankConfig.GetColumnName("date"),
	}
}

// parseBankStatementFromRecord creates a BankStatement from a CSV record
func (bsp *BankStatementParser) parseBankStatementFromRecord(record []string, parseCtx *ParseContext) (*models.BankStatement, *ParseError) {
	// Extract field values
	identifier, err := bsp.GetFieldValue(record, parseCtx, bsp.bankConfig.GetColumnName("identifier"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   bsp.bankConfig.GetColumnName("identifier"),
			Message: "failed to get unique identifier",
			Err:     err,
		}
	}
	
	amountStr, err := bsp.GetFieldValue(record, parseCtx, bsp.bankConfig.GetColumnName("amount"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   bsp.bankConfig.GetColumnName("amount"),
			Message: "failed to get amount",
			Err:     err,
		}
	}
	
	dateStr, err := bsp.GetFieldValue(record, parseCtx, bsp.bankConfig.GetColumnName("date"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   bsp.bankConfig.GetColumnName("date"),
			Message: "failed to get date",
			Err:     err,
		}
	}
	
	// Parse using bank-specific date format if specified
	if strings.TrimSpace(bsp.bankConfig.DateFormat) != "" {
		// Try bank-specific format first, then fall back to standard parsing
		bankStatement, err := bsp.createBankStatementWithFormat(identifier, amountStr, dateStr)
		if err == nil {
			return bankStatement, nil
		}
		// If bank-specific format fails, continue with standard parsing
	}
	
	// Use models helper to create bank statement from CSV values
	bankStatement, err := models.CreateBankStatementFromCSV(identifier, amountStr, dateStr)
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Message: "failed to create bank statement from CSV data",
			Err:     err,
		}
	}
	
	return bankStatement, nil
}

// createBankStatementWithFormat creates a bank statement using bank-specific date format
func (bsp *BankStatementParser) createBankStatementWithFormat(identifier, amountStr, dateStr string) (*models.BankStatement, error) {
	// Parse amount
	amount, err := models.ParseDecimalFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}
	
	// Parse date with bank-specific format
	date, err := models.ParseTimeWithFormats(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}
	
	bankStatement := models.NewBankStatement(strings.TrimSpace(identifier), amount, date)
	
	// Validate the created bank statement
	if err := bankStatement.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bank statement data: %w", err)
	}
	
	return bankStatement, nil
}

// ParseBankStatementsCallback defines a callback function for streaming bank statement parsing
type ParseBankStatementsCallback func([]*models.BankStatement) error

// ParseBankStatementsStream parses bank statements in streaming mode with batching
func (bsp *BankStatementParser) ParseBankStatementsStream(
	filePath string,
	batchSize int,
	callback ParseBankStatementsCallback,
) (*ParseStats, error) {
	return bsp.ParseBankStatementsStreamWithContext(context.Background(), filePath, batchSize, callback)
}

// ParseBankStatementsStreamWithContext parses bank statements in streaming mode with context support
func (bsp *BankStatementParser) ParseBankStatementsStreamWithContext(
	ctx context.Context,
	filePath string,
	batchSize int,
	callback ParseBankStatementsCallback,
) (*ParseStats, error) {
	if batchSize <= 0 {
		batchSize = 1000 // Default batch size
	}
	
	file, reader, err := bsp.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(ctx)
	stats := NewParseStats()
	
	// Read headers
	requiredHeaders := bsp.getRequiredHeaders()
	if err := bsp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return stats, fmt.Errorf("failed to read headers: %w", err)
	}
	
	batch := make([]*models.BankStatement, 0, batchSize)
	
	// Parse records in batches
	for {
		if parseCtx.IsCancelled() {
			return stats, fmt.Errorf("parsing cancelled")
		}
		
		record, err := bsp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				// Process remaining batch
				if len(batch) > 0 {
					if callbackErr := callback(batch); callbackErr != nil {
						return stats, fmt.Errorf("callback error: %w", callbackErr)
					}
				}
				break
			}
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "failed to read record",
				Err:     err,
			})
			continue
		}
		
		stats.RecordsParsed++
		
		// Parse bank statement from record
		bankStatement, parseErr := bsp.parseBankStatementFromRecord(record, parseCtx)
		if parseErr != nil {
			stats.AddError(parseErr)
			continue
		}
		
		// Validate bank statement
		if err := bankStatement.Validate(); err != nil {
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "bank statement validation failed",
				Err:     err,
			})
			continue
		}
		
		batch = append(batch, bankStatement)
		stats.RecordsValid++
		
		// Process batch when it's full
		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return stats, fmt.Errorf("callback error: %w", err)
			}
			batch = batch[:0] // Reset batch
		}
	}
	
	stats.TotalLines = parseCtx.LineNumber
	
	return stats, nil
}

// DetectBankFormat attempts to detect the bank format from the CSV file
func (bsp *BankStatementParser) DetectBankFormat(filePath string) (*BankConfig, error) {
	file, reader, err := bsp.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	// Read headers
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers for format detection: %w", err)
	}
	
	// Auto-detect format based on headers
	detectedConfig := AutoDetectBankConfig(headers)
	
	return detectedConfig, nil
}

// ValidateBankStatementFile validates that a CSV file has the correct format for bank statements
func (bsp *BankStatementParser) ValidateBankStatementFile(filePath string) error {
	file, reader, err := bsp.OpenFile(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(context.Background())
	
	// Validate headers
	requiredHeaders := bsp.getRequiredHeaders()
	if err := bsp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return fmt.Errorf("header validation failed: %w", err)
	}
	
	// Validate first few records
	recordCount := 0
	maxValidation := 10
	
	for recordCount < maxValidation {
		record, err := bsp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read record %d: %w", recordCount+1, err)
		}
		
		recordCount++
		
		// Try to parse the record
		_, parseErr := bsp.parseBankStatementFromRecord(record, parseCtx)
		if parseErr != nil {
			return fmt.Errorf("failed to parse record %d: %w", recordCount, parseErr)
		}
	}
	
	if recordCount == 0 {
		return fmt.Errorf("file contains no data records")
	}
	
	return nil
}

// GetBankConfig returns the current bank configuration
func (bsp *BankStatementParser) GetBankConfig() *BankConfig {
	return bsp.bankConfig
}

// SetBankConfig updates the bank configuration and reinitializes the parser
func (bsp *BankStatementParser) SetBankConfig(config *BankConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid bank configuration: %w", err)
	}
	
	bsp.bankConfig = config
	
	// Update parse configuration
	parseConfig := &ParseConfig{
		HasHeader:         config.HasHeader,
		Delimiter:         config.Delimiter,
		Comment:           0,
		TrimLeadingSpace:  true,
		SkipEmptyRows:     true,
		MaxFieldSize:      1000000,
		ValidateEncoding:  true,
	}
	
	bsp.BaseParser = NewBaseParser(parseConfig)
	
	return nil
}

// GetSampleBankStatement returns a sample bank statement for testing/validation
func (bsp *BankStatementParser) GetSampleBankStatement() *models.BankStatement {
	amount, _ := models.ParseDecimalFromString("-250.75")
	date, _ := models.ParseTimeWithFormats("2024-01-15")
	return models.NewBankStatement("BS001", amount, date)
}

// NewBankStatementParserWithAutoDetect creates a parser by auto-detecting the bank format
func NewBankStatementParserWithAutoDetect(filePath string) (*BankStatementParser, error) {
	// Create temporary parser for detection
	tempParser, err := NewBankStatementParser(StandardBankConfig)
	if err != nil {
		return nil, err
	}
	
	// Detect bank format
	config, err := tempParser.DetectBankFormat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect bank format: %w", err)
	}
	
	// Create parser with detected configuration
	return NewBankStatementParser(config)
}

// ParseMultipleBankFiles parses multiple bank statement files with different formats
func ParseMultipleBankFiles(files map[string]string) (map[string][]*models.BankStatement, map[string]*ParseStats, error) {
	results := make(map[string][]*models.BankStatement)
	stats := make(map[string]*ParseStats)
	
	for bankName, filePath := range files {
		// Get bank configuration
		config := GetBankConfig(bankName)
		if config == nil {
			return nil, nil, fmt.Errorf("unsupported bank: %s", bankName)
		}
		
		// Create parser for this bank
		parser, err := NewBankStatementParser(config)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create parser for %s: %w", bankName, err)
		}
		
		// Parse the file
		statements, parseStats, err := parser.ParseBankStatements(filePath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse file for %s: %w", bankName, err)
		}
		
		results[bankName] = statements
		stats[bankName] = parseStats
	}
	
	return results, stats, nil
}