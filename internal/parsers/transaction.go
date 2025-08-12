package parsers

import (
	"context"
	"fmt"
	"io"

	"golang-reconciliation-service/internal/models"
)

// TransactionParser handles parsing of system transaction CSV files
type TransactionParser struct {
	*BaseParser
	config *TransactionParserConfig
}

// NewTransactionParser creates a new TransactionParser with the given configuration
func NewTransactionParser(config *TransactionParserConfig) (*TransactionParser, error) {
	if config == nil {
		config = DefaultTransactionParserConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction parser configuration: %w", err)
	}
	
	parseConfig := &ParseConfig{
		HasHeader:         config.HasHeader,
		Delimiter:         config.Delimiter,
		Comment:           0,
		TrimLeadingSpace:  true,
		SkipEmptyRows:     true,
		MaxFieldSize:      1000000,
		ValidateEncoding:  true,
	}
	
	return &TransactionParser{
		BaseParser: NewBaseParser(parseConfig),
		config:     config,
	}, nil
}

// ParseTransactions parses a CSV file containing system transactions
func (tp *TransactionParser) ParseTransactions(filePath string) ([]*models.Transaction, *ParseStats, error) {
	return tp.ParseTransactionsWithContext(context.Background(), filePath)
}

// ParseTransactionsWithContext parses transactions with cancellation support
func (tp *TransactionParser) ParseTransactionsWithContext(ctx context.Context, filePath string) ([]*models.Transaction, *ParseStats, error) {
	file, reader, err := tp.OpenFile(filePath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(ctx)
	stats := NewParseStats()
	
	// Read headers
	requiredHeaders := tp.getRequiredHeaders()
	if err := tp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return nil, stats, fmt.Errorf("failed to read headers: %w", err)
	}
	
	var transactions []*models.Transaction
	
	// Parse records
	for {
		if parseCtx.IsCancelled() {
			return transactions, stats, fmt.Errorf("parsing cancelled")
		}
		
		record, err := tp.ReadRecord(reader, parseCtx)
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
		
		// Parse transaction from record
		transaction, parseErr := tp.parseTransactionFromRecord(record, parseCtx)
		if parseErr != nil {
			stats.AddError(parseErr)
			continue
		}
		
		// Validate transaction
		if err := transaction.Validate(); err != nil {
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "transaction validation failed",
				Err:     err,
			})
			continue
		}
		
		transactions = append(transactions, transaction)
		stats.RecordsValid++
	}
	
	stats.TotalLines = parseCtx.LineNumber
	
	return transactions, stats, nil
}

// getRequiredHeaders returns the list of required header names
func (tp *TransactionParser) getRequiredHeaders() []string {
	return []string{
		tp.config.GetColumnName("trx_id"),
		tp.config.GetColumnName("amount"),
		tp.config.GetColumnName("type"),
		tp.config.GetColumnName("transaction_time"),
	}
}

// parseTransactionFromRecord creates a Transaction from a CSV record
func (tp *TransactionParser) parseTransactionFromRecord(record []string, parseCtx *ParseContext) (*models.Transaction, *ParseError) {
	// Extract field values
	trxID, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("trx_id"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("trx_id"),
			Message: "failed to get transaction ID",
			Err:     err,
		}
	}
	
	amountStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("amount"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("amount"),
			Message: "failed to get amount",
			Err:     err,
		}
	}
	
	typeStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("type"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("type"),
			Message: "failed to get transaction type",
			Err:     err,
		}
	}
	
	timeStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("transaction_time"))
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("transaction_time"),
			Message: "failed to get transaction time",
			Err:     err,
		}
	}
	
	// Use models helper to create transaction from CSV values
	transaction, err := models.CreateTransactionFromCSV(trxID, amountStr, typeStr, timeStr)
	if err != nil {
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Message: "failed to create transaction from CSV data",
			Err:     err,
		}
	}
	
	return transaction, nil
}

// ParseTransactionsCallback defines a callback function for streaming parsing
type ParseTransactionsCallback func([]*models.Transaction) error

// ParseTransactionsStream parses transactions in streaming mode with batching
func (tp *TransactionParser) ParseTransactionsStream(
	filePath string,
	batchSize int,
	callback ParseTransactionsCallback,
) (*ParseStats, error) {
	return tp.ParseTransactionsStreamWithContext(context.Background(), filePath, batchSize, callback)
}

// ParseTransactionsStreamWithContext parses transactions in streaming mode with context support
func (tp *TransactionParser) ParseTransactionsStreamWithContext(
	ctx context.Context,
	filePath string,
	batchSize int,
	callback ParseTransactionsCallback,
) (*ParseStats, error) {
	if batchSize <= 0 {
		batchSize = 1000 // Default batch size
	}
	
	file, reader, err := tp.OpenFile(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(ctx)
	stats := NewParseStats()
	
	// Read headers
	requiredHeaders := tp.getRequiredHeaders()
	if err := tp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return stats, fmt.Errorf("failed to read headers: %w", err)
	}
	
	batch := make([]*models.Transaction, 0, batchSize)
	
	// Parse records in batches
	for {
		if parseCtx.IsCancelled() {
			return stats, fmt.Errorf("parsing cancelled")
		}
		
		record, err := tp.ReadRecord(reader, parseCtx)
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
		
		// Parse transaction from record
		transaction, parseErr := tp.parseTransactionFromRecord(record, parseCtx)
		if parseErr != nil {
			stats.AddError(parseErr)
			continue
		}
		
		// Validate transaction
		if err := transaction.Validate(); err != nil {
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: "transaction validation failed",
				Err:     err,
			})
			continue
		}
		
		batch = append(batch, transaction)
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

// ValidateTransactionFile validates that a CSV file has the correct format for transactions
func (tp *TransactionParser) ValidateTransactionFile(filePath string) error {
	file, reader, err := tp.OpenFile(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(context.Background())
	
	// Validate headers
	requiredHeaders := tp.getRequiredHeaders()
	if err := tp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		return fmt.Errorf("header validation failed: %w", err)
	}
	
	// Validate first few records
	recordCount := 0
	maxValidation := 10
	
	for recordCount < maxValidation {
		record, err := tp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read record %d: %w", recordCount+1, err)
		}
		
		recordCount++
		
		// Try to parse the record
		_, parseErr := tp.parseTransactionFromRecord(record, parseCtx)
		if parseErr != nil {
			return fmt.Errorf("failed to parse record %d: %w", recordCount, parseErr)
		}
	}
	
	if recordCount == 0 {
		return fmt.Errorf("file contains no data records")
	}
	
	return nil
}

// GetSampleTransaction returns a sample transaction for testing/validation
func (tp *TransactionParser) GetSampleTransaction() *models.Transaction {
	amount, _ := models.ParseDecimalFromString("100.50")
	txTime, _ := models.ParseTimeWithFormats("2024-01-15T10:30:00Z")
	return models.NewTransaction("TX001", amount, models.TransactionTypeCredit, txTime)
}