package reconciler

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// DataPreprocessor handles data normalization and preprocessing
type DataPreprocessor struct {
	config *PreprocessingConfig
}

// PreprocessingConfig contains configuration for data preprocessing
type PreprocessingConfig struct {
	// Date normalization options
	NormalizeTimezone   bool
	DefaultTimezone     *time.Location
	DateFormats         []string
	
	// Amount normalization options
	RemoveCurrencySymbols bool
	NormalizeDecimalPlaces int
	AmountFormats         []string
	
	// String normalization options
	TrimWhitespace       bool
	NormalizeCase        bool
	RemoveSpecialChars   bool
	
	// Validation options
	ValidateAmounts      bool
	ValidateDates        bool
	ValidateIDs          bool
	
	// Data cleaning options
	RemoveDuplicates     bool
	FixCommonErrors      bool
}

// DefaultPreprocessingConfig returns a default preprocessing configuration
func DefaultPreprocessingConfig() *PreprocessingConfig {
	return &PreprocessingConfig{
		NormalizeTimezone:      true,
		DefaultTimezone:        time.UTC,
		DateFormats:           []string{
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
			"01/02/2006",
			"Jan 2, 2006",
			"January 2, 2006",
		},
		RemoveCurrencySymbols:  true,
		NormalizeDecimalPlaces: -1, // -1 means no normalization
		AmountFormats:          []string{
			"$#,###.##",
			"#,###.##",
			"#.##",
		},
		TrimWhitespace:         true,
		NormalizeCase:          false,
		RemoveSpecialChars:     false,
		ValidateAmounts:        true,
		ValidateDates:          true,
		ValidateIDs:           true,
		RemoveDuplicates:       false,
		FixCommonErrors:       true,
	}
}

// NewDataPreprocessor creates a new data preprocessor
func NewDataPreprocessor(config *PreprocessingConfig) *DataPreprocessor {
	if config == nil {
		config = DefaultPreprocessingConfig()
	}
	
	return &DataPreprocessor{
		config: config,
	}
}

// PreprocessTransactions normalizes and validates transaction data
func (dp *DataPreprocessor) PreprocessTransactions(transactions []*models.Transaction) ([]*models.Transaction, error) {
	var processed []*models.Transaction
	var errors []error
	
	for i, tx := range transactions {
		processedTx, err := dp.preprocessTransaction(tx)
		if err != nil {
			if dp.config.FixCommonErrors {
				// Try to fix the transaction
				fixedTx, fixErr := dp.fixTransactionErrors(tx, err)
				if fixErr == nil {
					processed = append(processed, fixedTx)
					continue
				}
			}
			errors = append(errors, fmt.Errorf("transaction %d (%s): %w", i, tx.TrxID, err))
			continue
		}
		
		processed = append(processed, processedTx)
	}
	
	// Remove duplicates if configured
	if dp.config.RemoveDuplicates {
		processed = dp.removeDuplicateTransactions(processed)
	}
	
	if len(errors) > 0 {
		return processed, fmt.Errorf("preprocessing errors: %v", errors)
	}
	
	return processed, nil
}

// PreprocessBankStatements normalizes and validates bank statement data
func (dp *DataPreprocessor) PreprocessBankStatements(statements []*models.BankStatement) ([]*models.BankStatement, error) {
	var processed []*models.BankStatement
	var errors []error
	
	for i, stmt := range statements {
		processedStmt, err := dp.preprocessBankStatement(stmt)
		if err != nil {
			if dp.config.FixCommonErrors {
				// Try to fix the statement
				fixedStmt, fixErr := dp.fixStatementErrors(stmt, err)
				if fixErr == nil {
					processed = append(processed, fixedStmt)
					continue
				}
			}
			errors = append(errors, fmt.Errorf("statement %d (%s): %w", i, stmt.UniqueIdentifier, err))
			continue
		}
		
		processed = append(processed, processedStmt)
	}
	
	// Remove duplicates if configured
	if dp.config.RemoveDuplicates {
		processed = dp.removeDuplicateStatements(processed)
	}
	
	if len(errors) > 0 {
		return processed, fmt.Errorf("preprocessing errors: %v", errors)
	}
	
	return processed, nil
}

// preprocessTransaction processes a single transaction
func (dp *DataPreprocessor) preprocessTransaction(tx *models.Transaction) (*models.Transaction, error) {
	// Create a copy to avoid modifying the original
	processed := &models.Transaction{
		TrxID:           dp.normalizeString(tx.TrxID),
		Amount:          tx.Amount,
		Type:            tx.Type,
		TransactionTime: tx.TransactionTime,
	}
	
	// Normalize amount
	if dp.config.RemoveCurrencySymbols || dp.config.NormalizeDecimalPlaces >= 0 {
		normalizedAmount, err := dp.normalizeAmount(tx.Amount)
		if err != nil {
			return nil, fmt.Errorf("amount normalization failed: %w", err)
		}
		processed.Amount = normalizedAmount
	}
	
	// Normalize date/time
	if dp.config.NormalizeTimezone {
		processed.TransactionTime = dp.normalizeDateTime(tx.TransactionTime)
	}
	
	// Validate the processed transaction
	if dp.config.ValidateAmounts || dp.config.ValidateDates || dp.config.ValidateIDs {
		if err := dp.validateTransaction(processed); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}
	
	return processed, nil
}

// preprocessBankStatement processes a single bank statement
func (dp *DataPreprocessor) preprocessBankStatement(stmt *models.BankStatement) (*models.BankStatement, error) {
	// Create a copy to avoid modifying the original
	processed := &models.BankStatement{
		UniqueIdentifier: dp.normalizeString(stmt.UniqueIdentifier),
		Amount:          stmt.Amount,
		Date:            stmt.Date,
	}
	
	// Normalize amount
	if dp.config.RemoveCurrencySymbols || dp.config.NormalizeDecimalPlaces >= 0 {
		normalizedAmount, err := dp.normalizeAmount(stmt.Amount)
		if err != nil {
			return nil, fmt.Errorf("amount normalization failed: %w", err)
		}
		processed.Amount = normalizedAmount
	}
	
	// Normalize date
	if dp.config.NormalizeTimezone {
		processed.Date = dp.normalizeDateTime(stmt.Date)
	}
	
	// Validate the processed statement
	if dp.config.ValidateAmounts || dp.config.ValidateDates || dp.config.ValidateIDs {
		if err := dp.validateBankStatement(processed); err != nil {
			return nil, fmt.Errorf("validation failed: %w", err)
		}
	}
	
	return processed, nil
}

// normalizeString applies string normalization rules
func (dp *DataPreprocessor) normalizeString(s string) string {
	result := s
	
	if dp.config.TrimWhitespace {
		result = strings.TrimSpace(result)
	}
	
	if dp.config.NormalizeCase {
		result = strings.ToUpper(result)
	}
	
	if dp.config.RemoveSpecialChars {
		// Remove common special characters but keep alphanumeric and basic punctuation
		reg := regexp.MustCompile(`[^\w\s\-_.]`)
		result = reg.ReplaceAllString(result, "")
	}
	
	return result
}

// normalizeAmount applies amount normalization rules
func (dp *DataPreprocessor) normalizeAmount(amount decimal.Decimal) (decimal.Decimal, error) {
	result := amount
	
	// Normalize decimal places if configured
	if dp.config.NormalizeDecimalPlaces >= 0 {
		result = result.Round(int32(dp.config.NormalizeDecimalPlaces))
	}
	
	return result, nil
}

// normalizeDateTime applies date/time normalization rules
func (dp *DataPreprocessor) normalizeDateTime(t time.Time) time.Time {
	if dp.config.NormalizeTimezone && dp.config.DefaultTimezone != nil {
		return t.In(dp.config.DefaultTimezone)
	}
	
	return t
}

// validateTransaction validates a transaction according to configuration
func (dp *DataPreprocessor) validateTransaction(tx *models.Transaction) error {
	if dp.config.ValidateIDs {
		if strings.TrimSpace(tx.TrxID) == "" {
			return fmt.Errorf("transaction ID cannot be empty")
		}
	}
	
	if dp.config.ValidateAmounts {
		if tx.Amount.IsZero() {
			return fmt.Errorf("transaction amount cannot be zero")
		}
	}
	
	if dp.config.ValidateDates {
		if tx.TransactionTime.IsZero() {
			return fmt.Errorf("transaction time cannot be zero")
		}
		
		// Check if date is too far in the future (more than 1 day)
		if tx.TransactionTime.After(time.Now().Add(24 * time.Hour)) {
			return fmt.Errorf("transaction time is in the future: %s", tx.TransactionTime)
		}
		
		// Check if date is too far in the past (more than 20 years)
		if tx.TransactionTime.Before(time.Now().AddDate(-20, 0, 0)) {
			return fmt.Errorf("transaction time is too far in the past: %s", tx.TransactionTime)
		}
	}
	
	return nil
}

// validateBankStatement validates a bank statement according to configuration
func (dp *DataPreprocessor) validateBankStatement(stmt *models.BankStatement) error {
	if dp.config.ValidateIDs {
		if strings.TrimSpace(stmt.UniqueIdentifier) == "" {
			return fmt.Errorf("bank statement identifier cannot be empty")
		}
	}
	
	if dp.config.ValidateAmounts {
		if stmt.Amount.IsZero() {
			return fmt.Errorf("bank statement amount cannot be zero")
		}
	}
	
	if dp.config.ValidateDates {
		if stmt.Date.IsZero() {
			return fmt.Errorf("bank statement date cannot be zero")
		}
		
		// Check if date is in the future
		if stmt.Date.After(time.Now().Add(24 * time.Hour)) {
			return fmt.Errorf("bank statement date is in the future: %s", stmt.Date)
		}
		
		// Check if date is too far in the past (more than 20 years)
		if stmt.Date.Before(time.Now().AddDate(-20, 0, 0)) {
			return fmt.Errorf("bank statement date is too far in the past: %s", stmt.Date)
		}
	}
	
	return nil
}

// fixTransactionErrors attempts to fix common transaction errors
func (dp *DataPreprocessor) fixTransactionErrors(tx *models.Transaction, err error) (*models.Transaction, error) {
	fixed := &models.Transaction{
		TrxID:           tx.TrxID,
		Amount:          tx.Amount,
		Type:            tx.Type,
		TransactionTime: tx.TransactionTime,
	}
	
	errStr := err.Error()
	
	// Fix empty transaction ID
	if strings.Contains(errStr, "transaction ID cannot be empty") {
		fixed.TrxID = fmt.Sprintf("AUTO_%d", time.Now().UnixNano())
	}
	
	// Fix zero amount (convert to absolute minimum)
	if strings.Contains(errStr, "amount cannot be zero") && tx.Amount.IsZero() {
		fixed.Amount = decimal.NewFromFloat(0.01) // Set minimum amount
	}
	
	// Fix zero transaction time
	if strings.Contains(errStr, "time cannot be zero") && tx.TransactionTime.IsZero() {
		fixed.TransactionTime = time.Now()
	}
	
	// Validate the fixed transaction
	if err := dp.validateTransaction(fixed); err != nil {
		return nil, fmt.Errorf("unable to fix transaction: %w", err)
	}
	
	return fixed, nil
}

// fixStatementErrors attempts to fix common statement errors
func (dp *DataPreprocessor) fixStatementErrors(stmt *models.BankStatement, err error) (*models.BankStatement, error) {
	fixed := &models.BankStatement{
		UniqueIdentifier: stmt.UniqueIdentifier,
		Amount:          stmt.Amount,
		Date:            stmt.Date,
	}
	
	errStr := err.Error()
	
	// Fix empty identifier
	if strings.Contains(errStr, "identifier cannot be empty") {
		fixed.UniqueIdentifier = fmt.Sprintf("AUTO_%d", time.Now().UnixNano())
	}
	
	// Fix zero amount
	if strings.Contains(errStr, "amount cannot be zero") && stmt.Amount.IsZero() {
		fixed.Amount = decimal.NewFromFloat(0.01) // Set minimum amount
	}
	
	// Fix zero date
	if strings.Contains(errStr, "date cannot be zero") && stmt.Date.IsZero() {
		fixed.Date = time.Now()
	}
	
	// Validate the fixed statement
	if err := dp.validateBankStatement(fixed); err != nil {
		return nil, fmt.Errorf("unable to fix statement: %w", err)
	}
	
	return fixed, nil
}

// removeDuplicateTransactions removes duplicate transactions based on key fields
func (dp *DataPreprocessor) removeDuplicateTransactions(transactions []*models.Transaction) []*models.Transaction {
	seen := make(map[string]bool)
	var unique []*models.Transaction
	
	for _, tx := range transactions {
		// Create a key based on ID, amount, type, and date
		key := fmt.Sprintf("%s_%s_%s_%s", 
			tx.TrxID, 
			tx.Amount.String(), 
			tx.Type,
			tx.TransactionTime.Format("2006-01-02"))
		
		if !seen[key] {
			seen[key] = true
			unique = append(unique, tx)
		}
	}
	
	return unique
}

// removeDuplicateStatements removes duplicate statements based on key fields
func (dp *DataPreprocessor) removeDuplicateStatements(statements []*models.BankStatement) []*models.BankStatement {
	seen := make(map[string]bool)
	var unique []*models.BankStatement
	
	for _, stmt := range statements {
		// Create a key based on identifier, amount, and date
		key := fmt.Sprintf("%s_%s_%s", 
			stmt.UniqueIdentifier, 
			stmt.Amount.String(),
			stmt.Date.Format("2006-01-02"))
		
		if !seen[key] {
			seen[key] = true
			unique = append(unique, stmt)
		}
	}
	
	return unique
}

// GetStatistics returns preprocessing statistics
func (dp *DataPreprocessor) GetStatistics() *PreprocessingStats {
	return &PreprocessingStats{
		Config: dp.config,
	}
}

// PreprocessingStats contains statistics about preprocessing operations
type PreprocessingStats struct {
	Config                *PreprocessingConfig `json:"config"`
	TotalRecordsProcessed int                  `json:"total_records_processed"`
	RecordsFixed          int                  `json:"records_fixed"`
	RecordsRemoved        int                  `json:"records_removed"`
	ValidationErrors      int                  `json:"validation_errors"`
	ProcessingTime        time.Duration        `json:"processing_time"`
}