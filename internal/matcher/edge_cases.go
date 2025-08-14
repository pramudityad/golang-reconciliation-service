package matcher

import (
	"fmt"
	"sort"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// EdgeCaseHandler provides advanced handling for complex matching scenarios
type EdgeCaseHandler struct {
	Config *MatchingConfig
}

// NewEdgeCaseHandler creates a new edge case handler
func NewEdgeCaseHandler(config *MatchingConfig) *EdgeCaseHandler {
	return &EdgeCaseHandler{
		Config: config,
	}
}

// DuplicateDetectionResult represents the result of duplicate detection
type DuplicateDetectionResult struct {
	Groups []DuplicateGroup
}

// DuplicateGroup represents a group of potentially duplicate transactions
type DuplicateGroup struct {
	Transactions  []*models.Transaction
	Statements    []*models.BankStatement
	GroupID       string
	Confidence    float64
	Reason        string
}

// SameDayMatch represents transactions that occur on the same day
type SameDayMatch struct {
	Date                  time.Time
	Transactions          []*models.Transaction
	Statements            []*models.BankStatement
	PotentialMatches      []*MatchResult
	AmbiguityScore        float64
	ResolutionStrategy    string
}

// PartialMatchResult represents a partial amount match scenario
type PartialMatchResult struct {
	Transaction         *models.Transaction
	PartialStatements   []*models.BankStatement
	TotalAmount         decimal.Decimal
	AmountDifference    decimal.Decimal
	MatchRatio          float64
	Confidence          float64
}

// DetectDuplicates identifies potential duplicate transactions within the same dataset
func (ech *EdgeCaseHandler) DetectDuplicates(transactions []*models.Transaction) *DuplicateDetectionResult {
	var groups []DuplicateGroup
	processed := make(map[string]bool)
	
	for i, tx1 := range transactions {
		if processed[tx1.TrxID] {
			continue
		}
		
		var duplicates []*models.Transaction
		duplicates = append(duplicates, tx1)
		
		for j := i + 1; j < len(transactions); j++ {
			tx2 := transactions[j]
			
			if processed[tx2.TrxID] {
				continue
			}
			
			// Check for potential duplicates based on amount, date, and type
			if ech.isPotentialDuplicate(tx1, tx2) {
				duplicates = append(duplicates, tx2)
				processed[tx2.TrxID] = true
			}
		}
		
		if len(duplicates) > 1 {
			confidence := ech.calculateDuplicateConfidence(duplicates)
			groups = append(groups, DuplicateGroup{
				Transactions: duplicates,
				GroupID:      fmt.Sprintf("DUP_%s", tx1.TrxID),
				Confidence:   confidence,
				Reason:       ech.generateDuplicateReason(duplicates),
			})
		}
		
		processed[tx1.TrxID] = true
	}
	
	return &DuplicateDetectionResult{
		Groups: groups,
	}
}

// HandleSameDayTransactions resolves ambiguity when multiple transactions occur on the same day
func (ech *EdgeCaseHandler) HandleSameDayTransactions(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	engine *MatchingEngine,
) ([]*SameDayMatch, error) {
	
	// Group transactions and statements by date
	txByDate := make(map[string][]*models.Transaction)
	stmtByDate := make(map[string][]*models.BankStatement)
	
	for _, tx := range transactions {
		dateKey := ech.Config.NormalizeTime(tx.TransactionTime).Format("2006-01-02")
		txByDate[dateKey] = append(txByDate[dateKey], tx)
	}
	
	for _, stmt := range statements {
		dateKey := ech.Config.NormalizeTime(stmt.Date).Format("2006-01-02")
		stmtByDate[dateKey] = append(stmtByDate[dateKey], stmt)
	}
	
	var sameDayMatches []*SameDayMatch
	
	// Process dates with multiple transactions or statements
	for dateStr, dayTransactions := range txByDate {
		dayStatements := stmtByDate[dateStr]
		
		if len(dayTransactions) <= 1 && len(dayStatements) <= 1 {
			continue // No ambiguity
		}
		
		date, _ := time.Parse("2006-01-02", dateStr)
		
		// Find potential matches for this day
		var potentialMatches []*MatchResult
		for _, tx := range dayTransactions {
			matches, err := engine.FindMatches(tx)
			if err != nil {
				continue
			}
			
			// Filter matches to only include statements from the same day
			for _, match := range matches {
				matchDate := ech.Config.NormalizeTime(match.BankStatement.Date).Format("2006-01-02")
				if matchDate == dateStr {
					potentialMatches = append(potentialMatches, match)
				}
			}
		}
		
		ambiguityScore := ech.calculateAmbiguityScore(dayTransactions, dayStatements, potentialMatches)
		strategy := ech.determineResolutionStrategy(dayTransactions, dayStatements, potentialMatches)
		
		sameDayMatches = append(sameDayMatches, &SameDayMatch{
			Date:               date,
			Transactions:       dayTransactions,
			Statements:         dayStatements,
			PotentialMatches:   potentialMatches,
			AmbiguityScore:     ambiguityScore,
			ResolutionStrategy: strategy,
		})
	}
	
	return sameDayMatches, nil
}

// HandlePartialMatches finds scenarios where one transaction might match multiple smaller bank statements
func (ech *EdgeCaseHandler) HandlePartialMatches(
	transaction *models.Transaction,
	candidates []*models.BankStatement,
) []*PartialMatchResult {
	
	if !ech.Config.EnablePartialMatching {
		return nil
	}
	
	var results []*PartialMatchResult
	
	// Try different combinations of statements to see if they sum to the transaction amount
	combinations := ech.generateCombinations(candidates, 2, 4) // Max 4 statements per combination
	
	for _, combo := range combinations {
		total := decimal.Zero
		for _, stmt := range combo {
			total = total.Add(stmt.Amount.Abs())
		}
		
		targetAmount := transaction.Amount.Abs()
		difference := total.Sub(targetAmount).Abs()
		tolerance := ech.Config.GetAmountTolerance(targetAmount)
		
		if difference.LessThanOrEqual(tolerance) {
			ratio := total.Div(targetAmount).InexactFloat64()
			
			if ratio >= (1.0 - ech.Config.MaxPartialMatchRatio) {
				confidence := ech.calculatePartialMatchConfidence(transaction, combo, total, difference)
				
				results = append(results, &PartialMatchResult{
					Transaction:       transaction,
					PartialStatements: combo,
					TotalAmount:       total,
					AmountDifference:  difference,
					MatchRatio:        ratio,
					Confidence:        confidence,
				})
			}
		}
	}
	
	// Sort by confidence score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})
	
	return results
}

// NormalizeTimezones handles timezone differences across transaction data
func (ech *EdgeCaseHandler) NormalizeTimezones(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
) {
	
	// Normalize transaction times
	for _, tx := range transactions {
		tx.TransactionTime = ech.Config.NormalizeTime(tx.TransactionTime)
	}
	
	// Normalize statement dates
	for _, stmt := range statements {
		stmt.Date = ech.Config.NormalizeTime(stmt.Date)
	}
}

// ResolveTimezoneMismatch attempts to resolve timezone-related matching issues
func (ech *EdgeCaseHandler) ResolveTimezoneMismatch(
	tx *models.Transaction,
	stmt *models.BankStatement,
) (*TimezoneResolution, error) {
	
	resolution := &TimezoneResolution{
		Transaction:   tx,
		Statement:     stmt,
		OriginalTxTime: tx.TransactionTime,
		OriginalStmtTime: stmt.Date,
	}
	
	// Try different timezone interpretations
	timezones := []string{"UTC", "Local", "US/Eastern", "US/Central", "US/Mountain", "US/Pacific", "Europe/London"}
	
	var bestMatch *TimezoneMatch
	bestScore := 0.0
	
	for _, tzName := range timezones {
		tz, err := time.LoadLocation(tzName)
		if err != nil {
			continue
		}
		
		// Try converting transaction time to this timezone
		adjustedTxTime := tx.TransactionTime.In(tz)
		adjustedStmtTime := stmt.Date.In(tz)
		
		score := ech.calculateTimezoneMatchScore(adjustedTxTime, adjustedStmtTime)
		
		if score > bestScore {
			bestScore = score
			bestMatch = &TimezoneMatch{
				Timezone:           tzName,
				AdjustedTxTime:     adjustedTxTime,
				AdjustedStmtTime:   adjustedStmtTime,
				Score:              score,
			}
		}
	}
	
	resolution.BestMatch = bestMatch
	resolution.Confidence = bestScore
	
	return resolution, nil
}

// HandleCurrencyMismatch deals with potential currency conversion scenarios
func (ech *EdgeCaseHandler) HandleCurrencyMismatch(
	tx *models.Transaction,
	stmt *models.BankStatement,
	exchangeRate decimal.Decimal,
) *CurrencyMatchResult {
	
	convertedAmount := tx.Amount.Mul(exchangeRate)
	difference := convertedAmount.Sub(stmt.Amount.Abs()).Abs()
	tolerance := ech.Config.GetAmountTolerance(stmt.Amount.Abs())
	
	isMatch := difference.LessThanOrEqual(tolerance)
	confidence := 0.0
	
	if isMatch {
		// Calculate confidence based on how close the converted amount is
		if tolerance.IsPositive() {
			diffRatio := difference.Div(tolerance).InexactFloat64()
			confidence = 1.0 - diffRatio
		} else if difference.IsZero() {
			confidence = 1.0
		}
	}
	
	return &CurrencyMatchResult{
		Transaction:      tx,
		Statement:        stmt,
		ExchangeRate:     exchangeRate,
		ConvertedAmount:  convertedAmount,
		Difference:       difference,
		IsMatch:          isMatch,
		Confidence:       confidence,
	}
}

// isPotentialDuplicate checks if two transactions are potentially duplicates
func (ech *EdgeCaseHandler) isPotentialDuplicate(tx1, tx2 *models.Transaction) bool {
	// Same amount
	if !tx1.Amount.Equal(tx2.Amount) {
		return false
	}
	
	// Same type
	if tx1.Type != tx2.Type {
		return false
	}
	
	// Close in time (within 1 hour by default)
	timeDiff := tx1.TransactionTime.Sub(tx2.TransactionTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}
	
	return timeDiff <= time.Hour
}

// calculateDuplicateConfidence calculates confidence score for duplicate detection
func (ech *EdgeCaseHandler) calculateDuplicateConfidence(transactions []*models.Transaction) float64 {
	if len(transactions) < 2 {
		return 0.0
	}
	
	// Base confidence on how similar the transactions are
	reference := transactions[0]
	totalScore := 0.0
	
	for i := 1; i < len(transactions); i++ {
		tx := transactions[i]
		score := 0.0
		
		// Amount match
		if reference.Amount.Equal(tx.Amount) {
			score += 0.4
		}
		
		// Type match
		if reference.Type == tx.Type {
			score += 0.3
		}
		
		// Time proximity
		timeDiff := reference.TransactionTime.Sub(tx.TransactionTime)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		
		if timeDiff <= 5*time.Minute {
			score += 0.3
		} else if timeDiff <= 30*time.Minute {
			score += 0.2
		} else if timeDiff <= time.Hour {
			score += 0.1
		}
		
		totalScore += score
	}
	
	return totalScore / float64(len(transactions)-1)
}

// generateDuplicateReason creates a human-readable reason for duplicate detection
func (ech *EdgeCaseHandler) generateDuplicateReason(transactions []*models.Transaction) string {
	if len(transactions) < 2 {
		return "Single transaction"
	}
	
	return fmt.Sprintf("Found %d transactions with same amount (%s) and type (%s) within short time period",
		len(transactions), transactions[0].Amount.String(), transactions[0].Type.String())
}

// calculateAmbiguityScore calculates how ambiguous same-day matching is
func (ech *EdgeCaseHandler) calculateAmbiguityScore(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	matches []*MatchResult,
) float64 {
	
	if len(transactions) <= 1 && len(statements) <= 1 {
		return 0.0 // No ambiguity
	}
	
	// Higher score = more ambiguous
	txCount := float64(len(transactions))
	stmtCount := float64(len(statements))
	matchCount := float64(len(matches))
	
	// Calculate potential combinations
	maxCombinations := txCount * stmtCount
	actualMatches := matchCount
	
	if maxCombinations == 0 {
		return 0.0
	}
	
	// Ambiguity increases with more potential combinations and fewer clear matches
	ambiguityRatio := (maxCombinations - actualMatches) / maxCombinations
	
	return ambiguityRatio
}

// determineResolutionStrategy determines the best strategy for resolving same-day ambiguity
func (ech *EdgeCaseHandler) determineResolutionStrategy(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	matches []*MatchResult,
) string {
	
	if len(matches) == 0 {
		return "no_matches_found"
	}
	
	if len(transactions) == len(statements) && len(matches) == len(transactions) {
		return "one_to_one_mapping"
	}
	
	if len(transactions) < len(statements) {
		return "consolidation_needed"
	}
	
	if len(transactions) > len(statements) {
		return "splitting_needed"
	}
	
	return "manual_review_required"
}

// generateCombinations generates combinations of bank statements for partial matching
func (ech *EdgeCaseHandler) generateCombinations(
	statements []*models.BankStatement,
	minSize, maxSize int,
) [][]*models.BankStatement {
	
	var combinations [][]*models.BankStatement
	
	for size := minSize; size <= maxSize && size <= len(statements); size++ {
		combos := ech.getCombinations(statements, size)
		combinations = append(combinations, combos...)
	}
	
	return combinations
}

// getCombinations generates all combinations of given size
func (ech *EdgeCaseHandler) getCombinations(
	statements []*models.BankStatement,
	size int,
) [][]*models.BankStatement {
	
	if size > len(statements) || size <= 0 {
		return nil
	}
	
	if size == 1 {
		var result [][]*models.BankStatement
		for _, stmt := range statements {
			result = append(result, []*models.BankStatement{stmt})
		}
		return result
	}
	
	var result [][]*models.BankStatement
	
	for i := 0; i <= len(statements)-size; i++ {
		smaller := ech.getCombinations(statements[i+1:], size-1)
		for _, combo := range smaller {
			newCombo := append([]*models.BankStatement{statements[i]}, combo...)
			result = append(result, newCombo)
		}
	}
	
	return result
}

// calculatePartialMatchConfidence calculates confidence for partial matches
func (ech *EdgeCaseHandler) calculatePartialMatchConfidence(
	tx *models.Transaction,
	statements []*models.BankStatement,
	total, difference decimal.Decimal,
) float64 {
	
	// Base confidence on amount accuracy
	targetAmount := tx.Amount.Abs()
	tolerance := ech.Config.GetAmountTolerance(targetAmount)
	
	amountScore := 0.0
	if tolerance.IsPositive() {
		diffRatio := difference.Div(tolerance).InexactFloat64()
		amountScore = 1.0 - diffRatio
	} else if difference.IsZero() {
		amountScore = 1.0
	}
	
	// Reduce confidence based on number of statements involved
	complexityPenalty := 1.0 - (float64(len(statements)-1) * 0.1)
	if complexityPenalty < 0.1 {
		complexityPenalty = 0.1
	}
	
	return amountScore * complexityPenalty
}

// calculateTimezoneMatchScore calculates how well times match after timezone adjustment
func (ech *EdgeCaseHandler) calculateTimezoneMatchScore(txTime, stmtTime time.Time) float64 {
	// Normalize to date-only comparison for timezone matching
	txDate := time.Date(txTime.Year(), txTime.Month(), txTime.Day(), 0, 0, 0, 0, time.UTC)
	stmtDate := time.Date(stmtTime.Year(), stmtTime.Month(), stmtTime.Day(), 0, 0, 0, 0, time.UTC)
	
	if txDate.Equal(stmtDate) {
		return 1.0 // Same date
	}
	
	diff := txDate.Sub(stmtDate)
	if diff < 0 {
		diff = -diff
	}
	
	// Score decreases with larger date differences
	days := diff.Hours() / 24
	if days <= 1 {
		return 0.8
	} else if days <= 2 {
		return 0.6
	} else if days <= 3 {
		return 0.4
	}
	
	return 0.0
}

// Supporting types for edge case handling

type TimezoneResolution struct {
	Transaction      *models.Transaction
	Statement        *models.BankStatement
	OriginalTxTime   time.Time
	OriginalStmtTime time.Time
	BestMatch        *TimezoneMatch
	Confidence       float64
}

type TimezoneMatch struct {
	Timezone         string
	AdjustedTxTime   time.Time
	AdjustedStmtTime time.Time
	Score            float64
}

type CurrencyMatchResult struct {
	Transaction     *models.Transaction
	Statement       *models.BankStatement
	ExchangeRate    decimal.Decimal
	ConvertedAmount decimal.Decimal
	Difference      decimal.Decimal
	IsMatch         bool
	Confidence      float64
}