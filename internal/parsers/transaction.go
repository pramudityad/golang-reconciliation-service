package parsers

import (
	"context"
	"fmt"
	"io"

	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/pkg/errors"
	"golang-reconciliation-service/pkg/logger"
)

// TransactionParser handles parsing of system transaction CSV files
type TransactionParser struct {
	*BaseParser
	config *TransactionParserConfig
	logger logger.Logger
}

// NewTransactionParser creates a new TransactionParser with the given configuration
func NewTransactionParser(config *TransactionParserConfig) (*TransactionParser, error) {
	if config == nil {
		config = DefaultTransactionParserConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, errors.ConfigurationError(
			errors.CodeInvalidConfig,
			"transaction_parser_config",
			config,
			err,
		).WithSuggestion("Check the transaction parser configuration values")
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
	
	baseParser := NewBaseParser(parseConfig)
	log := logger.GetGlobalLogger().WithComponent("transaction_parser")
	
	log.WithFields(logger.Fields{
		"has_header": config.HasHeader,
		"delimiter":  string(config.Delimiter),
	}).Debug("Created transaction parser")
	
	return &TransactionParser{
		BaseParser: baseParser,
		config:     config,
		logger:     log,
	}, nil
}

// ParseTransactions parses a CSV file containing system transactions
func (tp *TransactionParser) ParseTransactions(filePath string) ([]*models.Transaction, *ParseStats, error) {
	return tp.ParseTransactionsWithContext(context.Background(), filePath)
}

// ParseTransactionsWithContext parses transactions with cancellation support
func (tp *TransactionParser) ParseTransactionsWithContext(ctx context.Context, filePath string) ([]*models.Transaction, *ParseStats, error) {
	// Log operation start
	tp.logger.WithFields(logger.Fields{
		"file_path": filePath,
		"operation": "parse_transactions",
	}).Info("Starting transaction parsing")
	
	file, reader, err := tp.OpenFile(filePath)
	if err != nil {
		tp.logger.WithError(err).WithField("file_path", filePath).Error("Failed to open transaction file")
		return nil, nil, errors.FileError(errors.CodeFileNotFound, filePath, err)
	}
	defer file.Close()
	
	parseCtx := NewParseContext(ctx)
	stats := NewParseStats()
	
	// Read headers
	requiredHeaders := tp.getRequiredHeaders()
	if err := tp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		tp.logger.WithError(err).WithFields(logger.Fields{
			"file_path":        filePath,
			"required_headers": requiredHeaders,
		}).Error("Failed to read or validate headers")
		
		return nil, stats, errors.ParseError(
			errors.CodeMissingColumn,
			filePath,
			parseCtx.LineNumber,
			"headers",
			"",
			err,
		).WithSuggestion("Ensure the CSV file has the required headers: " + fmt.Sprintf("%v", requiredHeaders))
	}
	
	var transactions []*models.Transaction
	
	// Parse records
	for {
		if parseCtx.IsCancelled() {
			tp.logger.Warn("Transaction parsing was cancelled")
			return transactions, stats, errors.InternalError(
				errors.CodeUnexpectedError,
				"transaction_parsing",
				fmt.Errorf("parsing cancelled by context"),
			)
		}
		
		record, err := tp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				break
			}
			
			tp.logger.WithError(err).WithField("line_number", parseCtx.LineNumber).Warn("Failed to read record")
			
			parseError := errors.ParseError(
				errors.CodeInvalidFormat,
				filePath,
				parseCtx.LineNumber,
				"record",
				"",
				err,
			)
			
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: parseError.Message,
				Err:     parseError,
			})
			continue
		}
		
		stats.RecordsParsed++
		
		// Parse transaction from record
		transaction, parseErr := tp.parseTransactionFromRecord(record, parseCtx, filePath)
		if parseErr != nil {
			stats.AddError(parseErr)
			continue
		}
		
		// Validate transaction
		if err := transaction.Validate(); err != nil {
			tp.logger.WithError(err).WithFields(logger.Fields{
				"line_number": parseCtx.LineNumber,
				"trx_id":      transaction.TrxID,
			}).Warn("Transaction validation failed")
			
			validationError := errors.ValidationError(
				errors.CodeInvalidData,
				"transaction",
				transaction.TrxID,
				err,
			)
			
			stats.AddError(&ParseError{
				Line:    parseCtx.LineNumber,
				Message: validationError.Message,
				Err:     validationError,
			})
			continue
		}
		
		transactions = append(transactions, transaction)
		stats.RecordsValid++
	}
	
	stats.TotalLines = parseCtx.LineNumber
	
	// Log completion with summary
	tp.logger.WithFields(logger.Fields{
		"file_path":      filePath,
		"total_lines":    stats.TotalLines,
		"records_parsed": stats.RecordsParsed,
		"records_valid":  stats.RecordsValid,
		"error_count":    len(stats.Errors),
	}).Info("Transaction parsing completed")
	
	if len(stats.Errors) > 0 {
		tp.logger.WithField("sample_errors", stats.GetSampleErrors(3)).Warn("Encountered errors during parsing")
	}
	
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
func (tp *TransactionParser) parseTransactionFromRecord(record []string, parseCtx *ParseContext, filePath string) (*models.Transaction, *ParseError) {
	// Extract field values with comprehensive error handling
	trxID, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("trx_id"))
	if err != nil {
		parseError := errors.ParseError(
			errors.CodeMissingField,
			filePath,
			parseCtx.LineNumber,
			tp.config.GetColumnName("trx_id"),
			"",
			err,
		).WithSuggestion("Ensure the transaction ID column exists and has a value")
		
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("trx_id"),
			Message: parseError.Message,
			Err:     parseError,
		}
	}
	
	amountStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("amount"))
	if err != nil {
		parseError := errors.ParseError(
			errors.CodeMissingField,
			filePath,
			parseCtx.LineNumber,
			tp.config.GetColumnName("amount"),
			"",
			err,
		).WithSuggestion("Ensure the amount column exists and has a valid decimal value")
		
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("amount"),
			Message: parseError.Message,
			Err:     parseError,
		}
	}
	
	typeStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("type"))
	if err != nil {
		parseError := errors.ParseError(
			errors.CodeMissingField,
			filePath,
			parseCtx.LineNumber,
			tp.config.GetColumnName("type"),
			"",
			err,
		).WithSuggestion("Ensure the type column exists and has a valid transaction type (credit/debit)")
		
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("type"),
			Message: parseError.Message,
			Err:     parseError,
		}
	}
	
	timeStr, err := tp.GetFieldValue(record, parseCtx, tp.config.GetColumnName("transaction_time"))
	if err != nil {
		parseError := errors.ParseError(
			errors.CodeMissingField,
			filePath,
			parseCtx.LineNumber,
			tp.config.GetColumnName("transaction_time"),
			"",
			err,
		).WithSuggestion("Ensure the transaction_time column exists and has a valid date/time value")
		
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Field:   tp.config.GetColumnName("transaction_time"),
			Message: parseError.Message,
			Err:     parseError,
		}
	}
	
	// Use models helper to create transaction from CSV values
	transaction, err := models.CreateTransactionFromCSV(trxID, amountStr, typeStr, timeStr)
	if err != nil {
		tp.logger.WithError(err).WithFields(logger.Fields{
			"line_number": parseCtx.LineNumber,
			"trx_id":      trxID,
			"amount":      amountStr,
			"type":        typeStr,
			"time":        timeStr,
		}).Warn("Failed to create transaction from CSV data")
		
		var errorCode errors.ErrorCode
		var suggestion string
		
		// Categorize the error based on which field likely caused it
		switch {
		case err.Error() == "invalid amount" || err.Error() == "decimal parsing failed":
			errorCode = errors.CodeInvalidAmount
			suggestion = "Check the amount format - use decimal numbers like '123.45'"
		case err.Error() == "invalid transaction type":
			errorCode = errors.CodeInvalidData
			suggestion = "Use 'credit' or 'debit' for transaction type"
		case err.Error() == "invalid date format" || err.Error() == "time parsing failed":
			errorCode = errors.CodeInvalidDate
			suggestion = "Use ISO 8601 date format like '2024-01-15T10:30:00Z'"
		default:
			errorCode = errors.CodeInvalidData
			suggestion = "Check the data format for all fields"
		}
		
		parseError := errors.ParseError(
			errorCode,
			filePath,
			parseCtx.LineNumber,
			"transaction_data",
			fmt.Sprintf("trx_id=%s, amount=%s, type=%s, time=%s", trxID, amountStr, typeStr, timeStr),
			err,
		).WithSuggestion(suggestion)
		
		return nil, &ParseError{
			Line:    parseCtx.LineNumber,
			Message: parseError.Message,
			Err:     parseError,
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
		transaction, parseErr := tp.parseTransactionFromRecord(record, parseCtx, filePath)
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
	tp.logger.WithField("file_path", filePath).Info("Validating transaction file format")
	
	file, reader, err := tp.OpenFile(filePath)
	if err != nil {
		tp.logger.WithError(err).WithField("file_path", filePath).Error("Failed to open file for validation")
		return errors.FileError(errors.CodeFileNotFound, filePath, err)
	}
	defer file.Close()
	
	parseCtx := NewParseContext(context.Background())
	
	// Validate headers
	requiredHeaders := tp.getRequiredHeaders()
	if err := tp.ReadHeaders(reader, parseCtx, requiredHeaders); err != nil {
		tp.logger.WithError(err).WithFields(logger.Fields{
			"file_path":        filePath,
			"required_headers": requiredHeaders,
		}).Error("Header validation failed")
		
		return errors.ParseError(
			errors.CodeMissingColumn,
			filePath,
			parseCtx.LineNumber,
			"headers",
			"",
			err,
		).WithSuggestion("Ensure the CSV file has the required headers: " + fmt.Sprintf("%v", requiredHeaders))
	}
	
	// Validate first few records
	recordCount := 0
	maxValidation := 10
	var validationErrors []error
	
	for recordCount < maxValidation {
		record, err := tp.ReadRecord(reader, parseCtx)
		if err != nil {
			if err == io.EOF {
				break
			}
			
			validationError := errors.ParseError(
				errors.CodeInvalidFormat,
				filePath,
				parseCtx.LineNumber,
				"record",
				"",
				err,
			)
			validationErrors = append(validationErrors, validationError)
			
			tp.logger.WithError(err).WithField("line_number", parseCtx.LineNumber).Warn("Failed to read record during validation")
			continue
		}
		
		recordCount++
		
		// Try to parse the record
		_, parseErr := tp.parseTransactionFromRecord(record, parseCtx, filePath)
		if parseErr != nil {
			validationErrors = append(validationErrors, parseErr.Err)
			tp.logger.WithError(parseErr.Err).WithField("line_number", parseCtx.LineNumber).Warn("Failed to parse record during validation")
		}
	}
	
	if recordCount == 0 {
		err := errors.ValidationError(
			errors.CodeMissingField,
			"data_records",
			0,
			nil,
		).WithSuggestion("Ensure the file contains data rows after the header")
		
		tp.logger.WithField("file_path", filePath).Error("File contains no data records")
		return err
	}
	
	if len(validationErrors) > 0 {
		tp.logger.WithFields(logger.Fields{
			"file_path":      filePath,
			"error_count":    len(validationErrors),
			"records_tested": recordCount,
		}).Error("File validation failed with errors")
		
		return errors.ValidationError(
			errors.CodeInvalidData,
			"file_format",
			fmt.Sprintf("%d validation errors out of %d records tested", len(validationErrors), recordCount),
			validationErrors[0], // Return first error as cause
		).WithSuggestion("Fix the data format issues and try again")
	}
	
	tp.logger.WithFields(logger.Fields{
		"file_path":      filePath,
		"records_tested": recordCount,
	}).Info("Transaction file validation completed successfully")
	
	return nil
}

// GetSampleTransaction returns a sample transaction for testing/validation
func (tp *TransactionParser) GetSampleTransaction() *models.Transaction {
	amount, _ := models.ParseDecimalFromString("100.50")
	txTime, _ := models.ParseTimeWithFormats("2024-01-15T10:30:00Z")
	return models.NewTransaction("TX001", amount, models.TransactionTypeCredit, txTime)
}