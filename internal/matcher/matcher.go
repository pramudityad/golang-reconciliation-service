package matcher

import (
	"fmt"
	"math"
	"sort"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// MatchingEngine is the core engine responsible for transaction matching
type MatchingEngine struct {
	Config                *MatchingConfig
	TransactionIndex      *TransactionIndex
	BankStatementIndex    *BankStatementIndex
}

// MatchResult represents the result of matching a transaction with bank statements
type MatchResult struct {
	Transaction      *models.Transaction
	BankStatement    *models.BankStatement
	MatchType        MatchType
	ConfidenceScore  float64
	AmountDifference decimal.Decimal
	DateDifference   time.Duration
	Reasons          []string
}

// ReconciliationResult represents the complete result of a reconciliation process
type ReconciliationResult struct {
	Matches              []*MatchResult
	UnmatchedTransactions []*models.Transaction
	UnmatchedStatements   []*models.BankStatement
	Summary              ReconciliationSummary
}

// ReconciliationSummary provides aggregate statistics about the reconciliation
type ReconciliationSummary struct {
	TotalTransactions     int
	TotalBankStatements   int
	MatchedTransactions   int
	MatchedStatements     int
	UnmatchedTransactions int
	UnmatchedStatements   int
	ExactMatches          int
	CloseMatches          int
	FuzzyMatches          int
	PossibleMatches       int
	TotalAmountMatched    decimal.Decimal
	TotalAmountUnmatched  decimal.Decimal
}

// NewMatchingEngine creates a new matching engine with the specified configuration
func NewMatchingEngine(config *MatchingConfig) *MatchingEngine {
	if config == nil {
		config = DefaultMatchingConfig()
	}
	
	return &MatchingEngine{
		Config: config,
	}
}

// LoadTransactions loads transactions into the engine and builds indexes
func (me *MatchingEngine) LoadTransactions(transactions []*models.Transaction) {
	me.TransactionIndex = NewTransactionIndex(transactions)
}

// LoadBankStatements loads bank statements into the engine and builds indexes
func (me *MatchingEngine) LoadBankStatements(statements []*models.BankStatement) {
	me.BankStatementIndex = NewBankStatementIndex(statements)
}

// Reconcile performs the complete reconciliation process between transactions and bank statements
func (me *MatchingEngine) Reconcile() (*ReconciliationResult, error) {
	if me.TransactionIndex == nil {
		return nil, fmt.Errorf("transactions must be loaded before reconciliation")
	}
	
	if me.BankStatementIndex == nil {
		return nil, fmt.Errorf("bank statements must be loaded before reconciliation")
	}
	
	var matches []*MatchResult
	matchedTransactionIDs := make(map[string]bool)
	matchedStatementIDs := make(map[string]bool)
	
	// Find matches for each transaction
	for _, tx := range me.TransactionIndex.AllTransactions {
		if matchedTransactionIDs[tx.TrxID] {
			continue // Already matched
		}
		
		candidates := me.BankStatementIndex.GetCandidates(tx, me.Config)
		if len(candidates) == 0 {
			continue // No candidates found
		}
		
		// Score and rank candidates
		scores := me.scoreTransactionCandidates(tx, candidates)
		
		// Find best match above confidence threshold
		if len(scores) > 0 && scores[0].ConfidenceScore >= me.Config.MinConfidenceScore {
			bestMatch := scores[0]
			
			// Check if bank statement is already matched
			if !matchedStatementIDs[bestMatch.BankStatement.UniqueIdentifier] {
				matches = append(matches, bestMatch)
				matchedTransactionIDs[tx.TrxID] = true
				matchedStatementIDs[bestMatch.BankStatement.UniqueIdentifier] = true
			}
		}
	}
	
	// Collect unmatched transactions and statements
	var unmatchedTransactions []*models.Transaction
	var unmatchedStatements []*models.BankStatement
	
	for _, tx := range me.TransactionIndex.AllTransactions {
		if !matchedTransactionIDs[tx.TrxID] {
			unmatchedTransactions = append(unmatchedTransactions, tx)
		}
	}
	
	for _, stmt := range me.BankStatementIndex.AllStatements {
		if !matchedStatementIDs[stmt.UniqueIdentifier] {
			unmatchedStatements = append(unmatchedStatements, stmt)
		}
	}
	
	// Calculate summary statistics
	summary := me.calculateSummary(matches, unmatchedTransactions, unmatchedStatements)
	
	return &ReconciliationResult{
		Matches:              matches,
		UnmatchedTransactions: unmatchedTransactions,
		UnmatchedStatements:   unmatchedStatements,
		Summary:              summary,
	}, nil
}

// FindMatches finds potential matches for a specific transaction
func (me *MatchingEngine) FindMatches(tx *models.Transaction) ([]*MatchResult, error) {
	if me.BankStatementIndex == nil {
		return nil, fmt.Errorf("bank statements must be loaded before finding matches")
	}
	
	candidates := me.BankStatementIndex.GetCandidates(tx, me.Config)
	if len(candidates) == 0 {
		return []*MatchResult{}, nil
	}
	
	return me.scoreTransactionCandidates(tx, candidates), nil
}

// FindMatchesForStatement finds potential matches for a specific bank statement
func (me *MatchingEngine) FindMatchesForStatement(stmt *models.BankStatement) ([]*MatchResult, error) {
	if me.TransactionIndex == nil {
		return nil, fmt.Errorf("transactions must be loaded before finding matches")
	}
	
	candidates := me.TransactionIndex.GetCandidates(stmt, me.Config)
	if len(candidates) == 0 {
		return []*MatchResult{}, nil
	}
	
	return me.scoreStatementCandidates(stmt, candidates), nil
}

// scoreTransactionCandidates scores bank statement candidates for a transaction
func (me *MatchingEngine) scoreTransactionCandidates(tx *models.Transaction, candidates []*models.BankStatement) []*MatchResult {
	var results []*MatchResult
	
	for _, stmt := range candidates {
		result := me.scoreMatch(tx, stmt)
		if result.ConfidenceScore >= me.Config.MinConfidenceScore {
			results = append(results, result)
		}
	}
	
	// Sort by confidence score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ConfidenceScore > results[j].ConfidenceScore
	})
	
	return results
}

// scoreStatementCandidates scores transaction candidates for a bank statement
func (me *MatchingEngine) scoreStatementCandidates(stmt *models.BankStatement, candidates []*models.Transaction) []*MatchResult {
	var results []*MatchResult
	
	for _, tx := range candidates {
		result := me.scoreMatch(tx, stmt)
		if result.ConfidenceScore >= me.Config.MinConfidenceScore {
			results = append(results, result)
		}
	}
	
	// Sort by confidence score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].ConfidenceScore > results[j].ConfidenceScore
	})
	
	return results
}

// scoreMatch calculates the match score between a transaction and bank statement
func (me *MatchingEngine) scoreMatch(tx *models.Transaction, stmt *models.BankStatement) *MatchResult {
	result := &MatchResult{
		Transaction:   tx,
		BankStatement: stmt,
		Reasons:       []string{},
	}
	
	// Normalize times for comparison
	normalizedTxTime := me.Config.NormalizeTime(tx.TransactionTime)
	normalizedStmtTime := me.Config.NormalizeTime(stmt.Date)
	
	// Calculate amount score
	amountScore := me.calculateAmountScore(tx, stmt)
	result.AmountDifference = me.calculateAmountDifference(tx, stmt)
	
	// Calculate date score
	dateScore := me.calculateDateScore(normalizedTxTime, normalizedStmtTime)
	result.DateDifference = me.calculateDateDifference(normalizedTxTime, normalizedStmtTime)
	
	// Calculate type score
	typeScore := me.calculateTypeScore(tx, stmt)
	
	// Calculate weighted confidence score
	weights := me.Config.Weights
	result.ConfidenceScore = (amountScore * weights.AmountWeight) + 
							(dateScore * weights.DateWeight) + 
							(typeScore * weights.TypeWeight)
	
	// Determine match type and add reasons
	result.MatchType = me.determineMatchType(result.ConfidenceScore, amountScore, dateScore, typeScore)
	result.Reasons = me.generateMatchReasons(tx, stmt, amountScore, dateScore, typeScore)
	
	return result
}

// calculateAmountScore calculates the score based on amount matching
func (me *MatchingEngine) calculateAmountScore(tx *models.Transaction, stmt *models.BankStatement) float64 {
	txAmount := tx.Amount.Abs()
	stmtAmount := stmt.Amount.Abs()
	
	// Check for exact match first
	if txAmount.Equal(stmtAmount) {
		return 1.0
	}
	
	// Calculate percentage difference
	tolerance := me.Config.GetAmountTolerance(txAmount)
	if tolerance.IsZero() {
		return 0.0 // No tolerance, must be exact
	}
	
	difference := txAmount.Sub(stmtAmount).Abs()
	if difference.LessThanOrEqual(tolerance) {
		// Linear decay based on difference relative to tolerance
		diffRatio := difference.Div(tolerance).InexactFloat64()
		return math.Max(0.0, 1.0-diffRatio)
	}
	
	return 0.0
}

// calculateDateScore calculates the score based on date proximity
func (me *MatchingEngine) calculateDateScore(txTime, stmtTime time.Time) float64 {
	if me.Config.DateToleranceDays == 0 {
		// Exact date match required
		if txTime.Format("2006-01-02") == stmtTime.Format("2006-01-02") {
			return 1.0
		}
		return 0.0
	}
	
	if me.Config.IsWithinDateTolerance(txTime, stmtTime) {
		// Calculate decay based on distance from exact match
		diff := txTime.Sub(stmtTime)
		if diff < 0 {
			diff = -diff
		}
		
		maxDiff := time.Duration(me.Config.DateToleranceDays) * 24 * time.Hour
		diffRatio := float64(diff) / float64(maxDiff)
		
		return math.Max(0.0, 1.0-diffRatio)
	}
	
	return 0.0
}

// calculateTypeScore calculates the score based on transaction type compatibility
func (me *MatchingEngine) calculateTypeScore(tx *models.Transaction, stmt *models.BankStatement) float64 {
	if !me.Config.EnableTypeMatching {
		return 1.0 // Type matching disabled, full score
	}
	
	// Determine expected transaction type based on bank statement amount
	expectedType := models.TransactionTypeCredit
	if stmt.Amount.IsNegative() {
		expectedType = models.TransactionTypeDebit
	}
	
	if tx.Type == expectedType {
		return 1.0
	}
	
	return 0.0
}

// calculateAmountDifference calculates the absolute difference between amounts
func (me *MatchingEngine) calculateAmountDifference(tx *models.Transaction, stmt *models.BankStatement) decimal.Decimal {
	txAmount := tx.Amount
	stmtAmount := stmt.Amount
	
	// Handle different conventions (positive vs negative for debits)
	if tx.Type == models.TransactionTypeDebit && stmt.Amount.IsPositive() {
		stmtAmount = stmt.Amount.Neg()
	} else if tx.Type == models.TransactionTypeCredit && stmt.Amount.IsNegative() {
		stmtAmount = stmt.Amount.Neg()
	}
	
	return txAmount.Sub(stmtAmount).Abs()
}

// calculateDateDifference calculates the time difference between dates
func (me *MatchingEngine) calculateDateDifference(txTime, stmtTime time.Time) time.Duration {
	diff := txTime.Sub(stmtTime)
	if diff < 0 {
		diff = -diff
	}
	return diff
}

// determineMatchType determines the type of match based on scores
func (me *MatchingEngine) determineMatchType(confidenceScore, amountScore, dateScore, typeScore float64) MatchType {
	if confidenceScore >= 0.95 && amountScore == 1.0 && dateScore >= 0.9 {
		return MatchExact
	}
	
	if confidenceScore >= 0.85 {
		return MatchClose
	}
	
	if confidenceScore >= 0.7 && me.Config.EnableFuzzyMatching {
		return MatchFuzzy
	}
	
	if confidenceScore >= me.Config.MinConfidenceScore {
		return MatchPossible
	}
	
	return MatchNone
}

// generateMatchReasons generates human-readable reasons for the match
func (me *MatchingEngine) generateMatchReasons(tx *models.Transaction, stmt *models.BankStatement, amountScore, dateScore, typeScore float64) []string {
	var reasons []string
	
	// Amount reasons
	if amountScore == 1.0 {
		reasons = append(reasons, "Exact amount match")
	} else if amountScore > 0.8 {
		reasons = append(reasons, "Close amount match")
	} else if amountScore > 0.0 {
		reasons = append(reasons, "Amount within tolerance")
	}
	
	// Date reasons
	if dateScore == 1.0 {
		reasons = append(reasons, "Same date")
	} else if dateScore > 0.8 {
		reasons = append(reasons, "Close date match")
	} else if dateScore > 0.0 {
		reasons = append(reasons, "Date within tolerance")
	}
	
	// Type reasons
	if typeScore == 1.0 && me.Config.EnableTypeMatching {
		reasons = append(reasons, "Transaction type matches")
	} else if typeScore == 0.0 && me.Config.EnableTypeMatching {
		reasons = append(reasons, "Transaction type mismatch")
	}
	
	return reasons
}

// calculateSummary calculates summary statistics for the reconciliation result
func (me *MatchingEngine) calculateSummary(matches []*MatchResult, unmatchedTx []*models.Transaction, unmatchedStmt []*models.BankStatement) ReconciliationSummary {
	summary := ReconciliationSummary{
		TotalTransactions:     len(me.TransactionIndex.AllTransactions),
		TotalBankStatements:   len(me.BankStatementIndex.AllStatements),
		MatchedTransactions:   len(matches),
		MatchedStatements:     len(matches),
		UnmatchedTransactions: len(unmatchedTx),
		UnmatchedStatements:   len(unmatchedStmt),
		TotalAmountMatched:    decimal.Zero,
		TotalAmountUnmatched:  decimal.Zero,
	}
	
	// Count match types and calculate amounts
	for _, match := range matches {
		switch match.MatchType {
		case MatchExact:
			summary.ExactMatches++
		case MatchClose:
			summary.CloseMatches++
		case MatchFuzzy:
			summary.FuzzyMatches++
		case MatchPossible:
			summary.PossibleMatches++
		}
		
		summary.TotalAmountMatched = summary.TotalAmountMatched.Add(match.Transaction.Amount.Abs())
	}
	
	// Calculate unmatched amounts
	for _, tx := range unmatchedTx {
		summary.TotalAmountUnmatched = summary.TotalAmountUnmatched.Add(tx.Amount.Abs())
	}
	
	return summary
}

// ValidateConfiguration validates the matching engine configuration
func (me *MatchingEngine) ValidateConfiguration() error {
	return me.Config.Validate()
}

// GetConfiguration returns a copy of the current configuration
func (me *MatchingEngine) GetConfiguration() *MatchingConfig {
	return me.Config.Clone()
}

// UpdateConfiguration updates the matching configuration
func (me *MatchingEngine) UpdateConfiguration(config *MatchingConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	me.Config = config.Clone()
	return nil
}

// GetStats returns statistics about the loaded data and indexes
func (me *MatchingEngine) GetStats() (IndexStats, IndexStats) {
	var txStats, stmtStats IndexStats
	
	if me.TransactionIndex != nil {
		txStats = me.TransactionIndex.GetIndexStats()
	}
	
	if me.BankStatementIndex != nil {
		stmtStats = me.BankStatementIndex.GetIndexStats()
	}
	
	return txStats, stmtStats
}