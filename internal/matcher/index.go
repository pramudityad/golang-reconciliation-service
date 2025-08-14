package matcher

import (
	"sort"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// TransactionIndex provides efficient indexing for transaction matching operations
type TransactionIndex struct {
	// ExactAmountIndex maps exact amounts to transaction slices
	ExactAmountIndex map[string][]*models.Transaction
	
	// DateIndex maps date strings (YYYY-MM-DD) to transaction slices
	DateIndex map[string][]*models.Transaction
	
	// AmountRangeIndex provides sorted amounts for range-based lookups
	AmountRangeIndex []*AmountIndexEntry
	
	// TypeIndex maps transaction types to transaction slices
	TypeIndex map[models.TransactionType][]*models.Transaction
	
	// AllTransactions holds all indexed transactions
	AllTransactions []*models.Transaction
}

// AmountIndexEntry represents an entry in the sorted amount index
type AmountIndexEntry struct {
	Amount       decimal.Decimal
	Transactions []*models.Transaction
}

// BankStatementIndex provides efficient indexing for bank statement matching operations
type BankStatementIndex struct {
	// ExactAmountIndex maps exact amounts to bank statement slices
	ExactAmountIndex map[string][]*models.BankStatement
	
	// DateIndex maps date strings (YYYY-MM-DD) to bank statement slices
	DateIndex map[string][]*models.BankStatement
	
	// AmountRangeIndex provides sorted amounts for range-based lookups
	AmountRangeIndex []*BankAmountIndexEntry
	
	// AllStatements holds all indexed bank statements
	AllStatements []*models.BankStatement
}

// BankAmountIndexEntry represents an entry in the sorted bank statement amount index
type BankAmountIndexEntry struct {
	Amount     decimal.Decimal
	Statements []*models.BankStatement
}

// NewTransactionIndex creates a new transaction index from a slice of transactions
func NewTransactionIndex(transactions []*models.Transaction) *TransactionIndex {
	index := &TransactionIndex{
		ExactAmountIndex: make(map[string][]*models.Transaction),
		DateIndex:        make(map[string][]*models.Transaction),
		TypeIndex:        make(map[models.TransactionType][]*models.Transaction),
		AllTransactions:  transactions,
	}
	
	index.buildIndexes()
	return index
}

// NewBankStatementIndex creates a new bank statement index from a slice of statements
func NewBankStatementIndex(statements []*models.BankStatement) *BankStatementIndex {
	index := &BankStatementIndex{
		ExactAmountIndex: make(map[string][]*models.BankStatement),
		DateIndex:        make(map[string][]*models.BankStatement),
		AllStatements:    statements,
	}
	
	index.buildIndexes()
	return index
}

// buildIndexes constructs all internal indexes for transactions
func (ti *TransactionIndex) buildIndexes() {
	// Build amount range index with unique amounts
	amountMap := make(map[string]*AmountIndexEntry)
	
	for _, tx := range ti.AllTransactions {
		amountKey := tx.Amount.String()
		dateKey := tx.TransactionTime.Format("2006-01-02")
		
		// Exact amount index
		ti.ExactAmountIndex[amountKey] = append(ti.ExactAmountIndex[amountKey], tx)
		
		// Date index
		ti.DateIndex[dateKey] = append(ti.DateIndex[dateKey], tx)
		
		// Type index
		ti.TypeIndex[tx.Type] = append(ti.TypeIndex[tx.Type], tx)
		
		// Amount range index
		if entry, exists := amountMap[amountKey]; exists {
			entry.Transactions = append(entry.Transactions, tx)
		} else {
			amountMap[amountKey] = &AmountIndexEntry{
				Amount:       tx.Amount,
				Transactions: []*models.Transaction{tx},
			}
		}
	}
	
	// Convert amount map to sorted slice
	ti.AmountRangeIndex = make([]*AmountIndexEntry, 0, len(amountMap))
	for _, entry := range amountMap {
		ti.AmountRangeIndex = append(ti.AmountRangeIndex, entry)
	}
	
	// Sort by amount for efficient range queries
	sort.Slice(ti.AmountRangeIndex, func(i, j int) bool {
		return ti.AmountRangeIndex[i].Amount.LessThan(ti.AmountRangeIndex[j].Amount)
	})
}

// buildIndexes constructs all internal indexes for bank statements
func (bsi *BankStatementIndex) buildIndexes() {
	// Build amount range index with unique amounts
	amountMap := make(map[string]*BankAmountIndexEntry)
	
	for _, stmt := range bsi.AllStatements {
		amountKey := stmt.Amount.String()
		dateKey := stmt.Date.Format("2006-01-02")
		
		// Exact amount index
		bsi.ExactAmountIndex[amountKey] = append(bsi.ExactAmountIndex[amountKey], stmt)
		
		// Date index
		bsi.DateIndex[dateKey] = append(bsi.DateIndex[dateKey], stmt)
		
		// Amount range index
		if entry, exists := amountMap[amountKey]; exists {
			entry.Statements = append(entry.Statements, stmt)
		} else {
			amountMap[amountKey] = &BankAmountIndexEntry{
				Amount:     stmt.Amount,
				Statements: []*models.BankStatement{stmt},
			}
		}
	}
	
	// Convert amount map to sorted slice
	bsi.AmountRangeIndex = make([]*BankAmountIndexEntry, 0, len(amountMap))
	for _, entry := range amountMap {
		bsi.AmountRangeIndex = append(bsi.AmountRangeIndex, entry)
	}
	
	// Sort by amount for efficient range queries
	sort.Slice(bsi.AmountRangeIndex, func(i, j int) bool {
		return bsi.AmountRangeIndex[i].Amount.LessThan(bsi.AmountRangeIndex[j].Amount)
	})
}

// GetByExactAmount returns transactions with the exact amount
func (ti *TransactionIndex) GetByExactAmount(amount decimal.Decimal) []*models.Transaction {
	return ti.ExactAmountIndex[amount.String()]
}

// GetByExactAmount returns bank statements with the exact amount
func (bsi *BankStatementIndex) GetByExactAmount(amount decimal.Decimal) []*models.BankStatement {
	return bsi.ExactAmountIndex[amount.String()]
}

// GetByAmountRange returns transactions within the specified amount range (inclusive)
func (ti *TransactionIndex) GetByAmountRange(minAmount, maxAmount decimal.Decimal) []*models.Transaction {
	var result []*models.Transaction
	
	// Find starting index using binary search
	startIdx := sort.Search(len(ti.AmountRangeIndex), func(i int) bool {
		return ti.AmountRangeIndex[i].Amount.GreaterThanOrEqual(minAmount)
	})
	
	// Collect all transactions in range
	for i := startIdx; i < len(ti.AmountRangeIndex); i++ {
		entry := ti.AmountRangeIndex[i]
		if entry.Amount.GreaterThan(maxAmount) {
			break
		}
		result = append(result, entry.Transactions...)
	}
	
	return result
}

// GetByAmountRange returns bank statements within the specified amount range (inclusive)
func (bsi *BankStatementIndex) GetByAmountRange(minAmount, maxAmount decimal.Decimal) []*models.BankStatement {
	var result []*models.BankStatement
	
	// Find starting index using binary search
	startIdx := sort.Search(len(bsi.AmountRangeIndex), func(i int) bool {
		return bsi.AmountRangeIndex[i].Amount.GreaterThanOrEqual(minAmount)
	})
	
	// Collect all statements in range
	for i := startIdx; i < len(bsi.AmountRangeIndex); i++ {
		entry := bsi.AmountRangeIndex[i]
		if entry.Amount.GreaterThan(maxAmount) {
			break
		}
		result = append(result, entry.Statements...)
	}
	
	return result
}

// GetByDate returns transactions for the specified date
func (ti *TransactionIndex) GetByDate(date time.Time) []*models.Transaction {
	dateKey := date.Format("2006-01-02")
	return ti.DateIndex[dateKey]
}

// GetByDate returns bank statements for the specified date
func (bsi *BankStatementIndex) GetByDate(date time.Time) []*models.BankStatement {
	dateKey := date.Format("2006-01-02")
	return bsi.DateIndex[dateKey]
}

// GetByDateRange returns transactions within the specified date range (inclusive)
func (ti *TransactionIndex) GetByDateRange(startDate, endDate time.Time) []*models.Transaction {
	var result []*models.Transaction
	
	current := startDate
	for !current.After(endDate) {
		dateKey := current.Format("2006-01-02")
		if transactions, exists := ti.DateIndex[dateKey]; exists {
			result = append(result, transactions...)
		}
		current = current.AddDate(0, 0, 1)
	}
	
	return result
}

// GetByDateRange returns bank statements within the specified date range (inclusive)
func (bsi *BankStatementIndex) GetByDateRange(startDate, endDate time.Time) []*models.BankStatement {
	var result []*models.BankStatement
	
	current := startDate
	for !current.After(endDate) {
		dateKey := current.Format("2006-01-02")
		if statements, exists := bsi.DateIndex[dateKey]; exists {
			result = append(result, statements...)
		}
		current = current.AddDate(0, 0, 1)
	}
	
	return result
}

// GetByType returns transactions of the specified type
func (ti *TransactionIndex) GetByType(txType models.TransactionType) []*models.Transaction {
	return ti.TypeIndex[txType]
}

// GetCandidates returns potential matching transactions for a bank statement
// based on amount tolerance and date range
func (ti *TransactionIndex) GetCandidates(stmt *models.BankStatement, config *MatchingConfig) []*models.Transaction {
	var candidates []*models.Transaction
	
	// Calculate amount tolerance
	tolerance := config.GetAmountTolerance(stmt.Amount.Abs())
	minAmount := stmt.Amount.Abs().Sub(tolerance)
	maxAmount := stmt.Amount.Abs().Add(tolerance)
	
	// Get transactions by amount range
	amountCandidates := ti.GetByAmountRange(minAmount, maxAmount)
	
	// Filter by date tolerance if specified
	if config.DateToleranceDays > 0 {
		var dateCandidates []*models.Transaction
		
		// Filter amount candidates by date range
		for _, tx := range amountCandidates {
			normalizedTxDate := config.NormalizeTime(tx.TransactionTime)
			normalizedStmtDate := config.NormalizeTime(stmt.Date)
			
			if config.IsWithinDateTolerance(normalizedTxDate, normalizedStmtDate) {
				dateCandidates = append(dateCandidates, tx)
			}
		}
		
		candidates = dateCandidates
	} else {
		candidates = amountCandidates
	}
	
	// Filter by transaction type if enabled
	if config.EnableTypeMatching {
		var typeCandidates []*models.Transaction
		
		// Bank statement negative amounts typically correspond to DEBIT transactions
		// Bank statement positive amounts typically correspond to CREDIT transactions
		expectedType := models.TransactionTypeCredit
		if stmt.Amount.IsNegative() {
			expectedType = models.TransactionTypeDebit
		}
		
		for _, tx := range candidates {
			if tx.Type == expectedType {
				typeCandidates = append(typeCandidates, tx)
			}
		}
		
		candidates = typeCandidates
	}
	
	// Limit number of candidates if specified
	if config.MaxCandidatesPerTransaction > 0 && len(candidates) > config.MaxCandidatesPerTransaction {
		candidates = candidates[:config.MaxCandidatesPerTransaction]
	}
	
	return candidates
}

// GetCandidates returns potential matching bank statements for a transaction
// based on amount tolerance and date range
func (bsi *BankStatementIndex) GetCandidates(tx *models.Transaction, config *MatchingConfig) []*models.BankStatement {
	var candidates []*models.BankStatement
	
	// Calculate amount tolerance - need to consider both positive and negative amounts
	tolerance := config.GetAmountTolerance(tx.Amount.Abs())
	
	// Bank statements may have negative amounts for debits
	var minAmount, maxAmount decimal.Decimal
	if tx.Type == models.TransactionTypeDebit {
		// Look for negative amounts in bank statements
		negAmount := tx.Amount.Neg()
		minAmount = negAmount.Sub(tolerance)
		maxAmount = negAmount.Add(tolerance)
	} else {
		// Look for positive amounts in bank statements
		minAmount = tx.Amount.Sub(tolerance)
		maxAmount = tx.Amount.Add(tolerance)
	}
	
	// Get statements by amount range
	amountCandidates := bsi.GetByAmountRange(minAmount, maxAmount)
	
	// Filter by date tolerance if specified
	if config.DateToleranceDays > 0 {
		var dateCandidates []*models.BankStatement
		
		// Filter amount candidates by date range
		for _, stmt := range amountCandidates {
			normalizedTxDate := config.NormalizeTime(tx.TransactionTime)
			normalizedStmtDate := config.NormalizeTime(stmt.Date)
			
			if config.IsWithinDateTolerance(normalizedTxDate, normalizedStmtDate) {
				dateCandidates = append(dateCandidates, stmt)
			}
		}
		
		candidates = dateCandidates
	} else {
		candidates = amountCandidates
	}
	
	// Limit number of candidates if specified
	if config.MaxCandidatesPerTransaction > 0 && len(candidates) > config.MaxCandidatesPerTransaction {
		candidates = candidates[:config.MaxCandidatesPerTransaction]
	}
	
	return candidates
}

// GetIndexStats returns statistics about the transaction index
func (ti *TransactionIndex) GetIndexStats() IndexStats {
	return IndexStats{
		TotalTransactions:    len(ti.AllTransactions),
		UniqueAmounts:        len(ti.AmountRangeIndex),
		UniqueDates:         len(ti.DateIndex),
		UniqueTypes:         len(ti.TypeIndex),
	}
}

// GetIndexStats returns statistics about the bank statement index
func (bsi *BankStatementIndex) GetIndexStats() IndexStats {
	return IndexStats{
		TotalTransactions:    len(bsi.AllStatements),
		UniqueAmounts:        len(bsi.AmountRangeIndex),
		UniqueDates:         len(bsi.DateIndex),
		UniqueTypes:         0, // Bank statements don't have transaction types
	}
}

// IndexStats provides statistics about index usage and efficiency
type IndexStats struct {
	TotalTransactions int
	UniqueAmounts     int
	UniqueDates       int
	UniqueTypes       int
}

// AddTransaction adds a new transaction to the index and rebuilds necessary indexes
func (ti *TransactionIndex) AddTransaction(tx *models.Transaction) {
	ti.AllTransactions = append(ti.AllTransactions, tx)
	
	// Update exact amount index
	amountKey := tx.Amount.String()
	ti.ExactAmountIndex[amountKey] = append(ti.ExactAmountIndex[amountKey], tx)
	
	// Update date index
	dateKey := tx.TransactionTime.Format("2006-01-02")
	ti.DateIndex[dateKey] = append(ti.DateIndex[dateKey], tx)
	
	// Update type index
	ti.TypeIndex[tx.Type] = append(ti.TypeIndex[tx.Type], tx)
	
	// Rebuild amount range index (could be optimized for better performance)
	ti.buildAmountRangeIndex()
}

// AddStatement adds a new bank statement to the index and rebuilds necessary indexes
func (bsi *BankStatementIndex) AddStatement(stmt *models.BankStatement) {
	bsi.AllStatements = append(bsi.AllStatements, stmt)
	
	// Update exact amount index
	amountKey := stmt.Amount.String()
	bsi.ExactAmountIndex[amountKey] = append(bsi.ExactAmountIndex[amountKey], stmt)
	
	// Update date index
	dateKey := stmt.Date.Format("2006-01-02")
	bsi.DateIndex[dateKey] = append(bsi.DateIndex[dateKey], stmt)
	
	// Rebuild amount range index (could be optimized for better performance)
	bsi.buildAmountRangeIndex()
}

// buildAmountRangeIndex rebuilds only the amount range index for transactions
func (ti *TransactionIndex) buildAmountRangeIndex() {
	amountMap := make(map[string]*AmountIndexEntry)
	
	for _, tx := range ti.AllTransactions {
		amountKey := tx.Amount.String()
		if entry, exists := amountMap[amountKey]; exists {
			entry.Transactions = append(entry.Transactions, tx)
		} else {
			amountMap[amountKey] = &AmountIndexEntry{
				Amount:       tx.Amount,
				Transactions: []*models.Transaction{tx},
			}
		}
	}
	
	// Convert to sorted slice
	ti.AmountRangeIndex = make([]*AmountIndexEntry, 0, len(amountMap))
	for _, entry := range amountMap {
		ti.AmountRangeIndex = append(ti.AmountRangeIndex, entry)
	}
	
	sort.Slice(ti.AmountRangeIndex, func(i, j int) bool {
		return ti.AmountRangeIndex[i].Amount.LessThan(ti.AmountRangeIndex[j].Amount)
	})
}

// buildAmountRangeIndex rebuilds only the amount range index for bank statements
func (bsi *BankStatementIndex) buildAmountRangeIndex() {
	amountMap := make(map[string]*BankAmountIndexEntry)
	
	for _, stmt := range bsi.AllStatements {
		amountKey := stmt.Amount.String()
		if entry, exists := amountMap[amountKey]; exists {
			entry.Statements = append(entry.Statements, stmt)
		} else {
			amountMap[amountKey] = &BankAmountIndexEntry{
				Amount:     stmt.Amount,
				Statements: []*models.BankStatement{stmt},
			}
		}
	}
	
	// Convert to sorted slice
	bsi.AmountRangeIndex = make([]*BankAmountIndexEntry, 0, len(amountMap))
	for _, entry := range amountMap {
		bsi.AmountRangeIndex = append(bsi.AmountRangeIndex, entry)
	}
	
	sort.Slice(bsi.AmountRangeIndex, func(i, j int) bool {
		return bsi.AmountRangeIndex[i].Amount.LessThan(bsi.AmountRangeIndex[j].Amount)
	})
}