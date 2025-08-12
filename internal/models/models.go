package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	// TransactionTypeDebit represents a debit transaction
	TransactionTypeDebit TransactionType = "DEBIT"
	// TransactionTypeCredit represents a credit transaction
	TransactionTypeCredit TransactionType = "CREDIT"
)

// String returns the string representation of TransactionType
func (t TransactionType) String() string {
	return string(t)
}

// IsValid checks if the transaction type is valid
func (t TransactionType) IsValid() bool {
	return t == TransactionTypeDebit || t == TransactionTypeCredit
}

// Transaction represents a system transaction record
type Transaction struct {
	TrxID           string          `json:"trxID" csv:"trxID"`
	Amount          decimal.Decimal `json:"amount" csv:"amount"`
	Type            TransactionType `json:"type" csv:"type"`
	TransactionTime time.Time       `json:"transactionTime" csv:"transactionTime"`
}

// NewTransaction creates a new Transaction instance
func NewTransaction(trxID string, amount decimal.Decimal, txType TransactionType, txTime time.Time) *Transaction {
	return &Transaction{
		TrxID:           trxID,
		Amount:          amount,
		Type:            txType,
		TransactionTime: txTime,
	}
}

// Validate performs basic validation on the Transaction
func (t *Transaction) Validate() error {
	if strings.TrimSpace(t.TrxID) == "" {
		return fmt.Errorf("transaction ID cannot be empty")
	}
	
	if t.Amount.IsZero() {
		return fmt.Errorf("transaction amount cannot be zero")
	}
	
	if !t.Type.IsValid() {
		return fmt.Errorf("invalid transaction type: %s", t.Type)
	}
	
	if t.TransactionTime.IsZero() {
		return fmt.Errorf("transaction time cannot be zero")
	}
	
	return nil
}

// String returns a string representation of the Transaction
func (t *Transaction) String() string {
	return fmt.Sprintf("Transaction{ID: %s, Amount: %s, Type: %s, Time: %s}",
		t.TrxID, t.Amount.String(), t.Type, t.TransactionTime.Format(time.RFC3339))
}

// MarshalJSON implements custom JSON marshaling for Transaction
func (t *Transaction) MarshalJSON() ([]byte, error) {
	type Alias Transaction
	return json.Marshal(&struct {
		Amount          string `json:"amount"`
		TransactionTime string `json:"transactionTime"`
		*Alias
	}{
		Amount:          t.Amount.String(),
		TransactionTime: t.TransactionTime.Format(time.RFC3339),
		Alias:           (*Alias)(t),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Transaction
func (t *Transaction) UnmarshalJSON(data []byte) error {
	type Alias Transaction
	aux := &struct {
		Amount          string `json:"amount"`
		TransactionTime string `json:"transactionTime"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var err error
	t.Amount, err = decimal.NewFromString(aux.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount format: %w", err)
	}

	t.TransactionTime, err = time.Parse(time.RFC3339, aux.TransactionTime)
	if err != nil {
		return fmt.Errorf("invalid transaction time format: %w", err)
	}

	return nil
}

// Equals compares two Transaction instances for equality
func (t *Transaction) Equals(other *Transaction) bool {
	if other == nil {
		return false
	}
	
	return t.TrxID == other.TrxID &&
		t.Amount.Equal(other.Amount) &&
		t.Type == other.Type &&
		t.TransactionTime.Equal(other.TransactionTime)
}

// GetAbsoluteAmount returns the absolute value of the transaction amount
func (t *Transaction) GetAbsoluteAmount() decimal.Decimal {
	return t.Amount.Abs()
}

// IsDebit returns true if the transaction is a debit
func (t *Transaction) IsDebit() bool {
	return t.Type == TransactionTypeDebit
}

// IsCredit returns true if the transaction is a credit  
func (t *Transaction) IsCredit() bool {
	return t.Type == TransactionTypeCredit
}

// BankStatement represents a bank statement transaction record
type BankStatement struct {
	UniqueIdentifier string          `json:"unique_identifier" csv:"unique_identifier"`
	Amount           decimal.Decimal `json:"amount" csv:"amount"`
	Date             time.Time       `json:"date" csv:"date"`
}

// NewBankStatement creates a new BankStatement instance
func NewBankStatement(identifier string, amount decimal.Decimal, date time.Time) *BankStatement {
	return &BankStatement{
		UniqueIdentifier: identifier,
		Amount:           amount,
		Date:             date,
	}
}

// Validate performs basic validation on the BankStatement
func (bs *BankStatement) Validate() error {
	if err := bs.ValidateIdentifier(); err != nil {
		return err
	}
	
	if err := bs.ValidateAmount(); err != nil {
		return err
	}
	
	if err := bs.ValidateDate(); err != nil {
		return err
	}
	
	return nil
}

// ValidateIdentifier checks if the unique identifier is valid
func (bs *BankStatement) ValidateIdentifier() error {
	if strings.TrimSpace(bs.UniqueIdentifier) == "" {
		return fmt.Errorf("bank statement identifier cannot be empty")
	}
	return nil
}

// ValidateAmount checks if the amount is valid
func (bs *BankStatement) ValidateAmount() error {
	if bs.Amount.IsZero() {
		return fmt.Errorf("bank statement amount cannot be zero")
	}
	return nil
}

// ValidateDate checks if the date is valid
func (bs *BankStatement) ValidateDate() error {
	if bs.Date.IsZero() {
		return fmt.Errorf("bank statement date cannot be zero")
	}
	
	// Check if date is not in the future (with some tolerance)
	now := time.Now()
	if bs.Date.After(now.Add(24 * time.Hour)) {
		return fmt.Errorf("bank statement date cannot be more than 1 day in the future")
	}
	
	// Check if date is not too far in the past (reasonable limit)
	tenYearsAgo := now.AddDate(-10, 0, 0)
	if bs.Date.Before(tenYearsAgo) {
		return fmt.Errorf("bank statement date cannot be more than 10 years in the past")
	}
	
	return nil
}

// String returns a string representation of the BankStatement
func (bs *BankStatement) String() string {
	return fmt.Sprintf("BankStatement{ID: %s, Amount: %s, Date: %s}",
		bs.UniqueIdentifier, bs.Amount.String(), bs.Date.Format("2006-01-02"))
}

// MarshalJSON implements custom JSON marshaling for BankStatement
func (bs *BankStatement) MarshalJSON() ([]byte, error) {
	type Alias BankStatement
	return json.Marshal(&struct {
		Amount string `json:"amount"`
		Date   string `json:"date"`
		*Alias
	}{
		Amount: bs.Amount.String(),
		Date:   bs.Date.Format("2006-01-02"),
		Alias:  (*Alias)(bs),
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for BankStatement
func (bs *BankStatement) UnmarshalJSON(data []byte) error {
	type Alias BankStatement
	aux := &struct {
		Amount string `json:"amount"`
		Date   string `json:"date"`
		*Alias
	}{
		Alias: (*Alias)(bs),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var err error
	bs.Amount, err = decimal.NewFromString(aux.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount format: %w", err)
	}

	// Try multiple date formats commonly used in bank statements
	dateFormats := []string{
		"2006-01-02",
		"01/02/2006",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
	}
	
	for _, format := range dateFormats {
		if bs.Date, err = time.Parse(format, aux.Date); err == nil {
			break
		}
	}
	
	if err != nil {
		return fmt.Errorf("invalid date format: %w", err)
	}

	return nil
}

// Equals compares two BankStatement instances for equality
func (bs *BankStatement) Equals(other *BankStatement) bool {
	if other == nil {
		return false
	}
	
	return bs.UniqueIdentifier == other.UniqueIdentifier &&
		bs.Amount.Equal(other.Amount) &&
		bs.Date.Format("2006-01-02") == other.Date.Format("2006-01-02")
}

// GetAbsoluteAmount returns the absolute value of the bank statement amount
func (bs *BankStatement) GetAbsoluteAmount() decimal.Decimal {
	return bs.Amount.Abs()
}

// IsDebit returns true if the bank statement amount represents a debit (negative amount)
func (bs *BankStatement) IsDebit() bool {
	return bs.Amount.IsNegative()
}

// IsCredit returns true if the bank statement amount represents a credit (positive amount)
func (bs *BankStatement) IsCredit() bool {
	return bs.Amount.IsPositive()
}

// GetTransactionType returns the transaction type based on amount sign
func (bs *BankStatement) GetTransactionType() TransactionType {
	if bs.IsDebit() {
		return TransactionTypeDebit
	}
	return TransactionTypeCredit
}

// NormalizeAmount converts bank statement amount to match transaction amount format
// (bank statements may have negative amounts for debits)
func (bs *BankStatement) NormalizeAmount() decimal.Decimal {
	return bs.GetAbsoluteAmount()
}

// Utility functions for type conversion and validation

// ParseDecimalFromString parses a decimal value from string with validation
func ParseDecimalFromString(s string) (decimal.Decimal, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return decimal.Zero, fmt.Errorf("amount string cannot be empty")
	}
	
	// Remove common currency symbols and thousand separators
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid decimal format '%s': %w", s, err)
	}
	
	return d, nil
}

// ParseTransactionType parses and validates a transaction type from string
func ParseTransactionType(s string) (TransactionType, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	
	switch s {
	case "DEBIT", "D", "DR":
		return TransactionTypeDebit, nil
	case "CREDIT", "C", "CR":
		return TransactionTypeCredit, nil
	default:
		return "", fmt.Errorf("invalid transaction type '%s': must be DEBIT or CREDIT", s)
	}
}

// ParseTimeWithFormats attempts to parse time from string using multiple common formats
func ParseTimeWithFormats(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("time string cannot be empty")
	}
	
	// Common time formats used in CSV files
	formats := []string{
		time.RFC3339,                // "2006-01-02T15:04:05Z07:00"
		"2006-01-02 15:04:05",      // "2006-01-02 15:04:05"
		"2006-01-02T15:04:05",      // "2006-01-02T15:04:05"
		"2006-01-02",               // "2006-01-02"
		"01/02/2006 15:04:05",      // "01/02/2006 15:04:05"
		"01/02/2006",               // "01/02/2006"
		"02-01-2006",               // "02-01-2006"
		"2006/01/02",               // "2006/01/02"
		"Jan 2, 2006",              // "Jan 2, 2006"
		"January 2, 2006",          // "January 2, 2006"
	}
	
	var lastErr error
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse time '%s': %w", s, lastErr)
}

// ValidateAmountRange checks if a decimal amount is within reasonable bounds
func ValidateAmountRange(amount decimal.Decimal, min, max decimal.Decimal) error {
	if amount.LessThan(min) {
		return fmt.Errorf("amount %s is below minimum allowed %s", amount.String(), min.String())
	}
	
	if amount.GreaterThan(max) {
		return fmt.Errorf("amount %s exceeds maximum allowed %s", amount.String(), max.String())
	}
	
	return nil
}

// ValidateDateRange checks if a date is within reasonable bounds
func ValidateDateRange(date time.Time, minDate, maxDate time.Time) error {
	if date.Before(minDate) {
		return fmt.Errorf("date %s is before minimum allowed date %s", 
			date.Format("2006-01-02"), minDate.Format("2006-01-02"))
	}
	
	if date.After(maxDate) {
		return fmt.Errorf("date %s is after maximum allowed date %s", 
			date.Format("2006-01-02"), maxDate.Format("2006-01-02"))
	}
	
	return nil
}

// CompareAmountsWithTolerance compares two decimal amounts with a tolerance
func CompareAmountsWithTolerance(a, b, tolerance decimal.Decimal) bool {
	diff := a.Sub(b).Abs()
	return diff.LessThanOrEqual(tolerance)
}

// CompareDatesWithTolerance compares two dates within a day tolerance
func CompareDatesWithTolerance(a, b time.Time, toleranceDays int) bool {
	diff := a.Sub(b)
	if diff < 0 {
		diff = -diff
	}
	
	maxDiff := time.Duration(toleranceDays) * 24 * time.Hour
	return diff <= maxDiff
}

// NormalizeIdentifier cleans and normalizes identifier strings
func NormalizeIdentifier(id string) string {
	// Trim whitespace and convert to uppercase for consistency
	normalized := strings.ToUpper(strings.TrimSpace(id))
	
	// Remove common prefixes that banks might add
	prefixes := []string{"TXN", "TRANS", "REF", "ID"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			normalized = strings.TrimLeft(normalized, "-_:")
			break
		}
	}
	
	return normalized
}

// CreateTransactionFromCSV creates a Transaction from CSV field values
func CreateTransactionFromCSV(trxID, amountStr, typeStr, timeStr string) (*Transaction, error) {
	// Parse amount
	amount, err := ParseDecimalFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount in CSV: %w", err)
	}
	
	// Parse transaction type
	txType, err := ParseTransactionType(typeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction type in CSV: %w", err)
	}
	
	// Parse time
	txTime, err := ParseTimeWithFormats(timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction time in CSV: %w", err)
	}
	
	transaction := NewTransaction(strings.TrimSpace(trxID), amount, txType, txTime)
	
	// Validate the created transaction
	if err := transaction.Validate(); err != nil {
		return nil, fmt.Errorf("invalid transaction data: %w", err)
	}
	
	return transaction, nil
}

// CreateBankStatementFromCSV creates a BankStatement from CSV field values
func CreateBankStatementFromCSV(identifier, amountStr, dateStr string) (*BankStatement, error) {
	// Parse amount
	amount, err := ParseDecimalFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount in CSV: %w", err)
	}
	
	// Parse date
	date, err := ParseTimeWithFormats(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date in CSV: %w", err)
	}
	
	bankStatement := NewBankStatement(strings.TrimSpace(identifier), amount, date)
	
	// Validate the created bank statement
	if err := bankStatement.Validate(); err != nil {
		return nil, fmt.Errorf("invalid bank statement data: %w", err)
	}
	
	return bankStatement, nil
}