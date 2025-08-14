package matcher

import (
	"testing"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

func createTestMatchingData() ([]*models.Transaction, []*models.BankStatement) {
	transactions := []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.50),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX002",
			Amount:          decimal.NewFromFloat(250.00),
			Type:            models.TransactionTypeDebit,
			TransactionTime: time.Date(2024, 1, 15, 14, 20, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX003",
			Amount:          decimal.NewFromFloat(75.25),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 16, 9, 15, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX004",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 17, 11, 0, 0, 0, time.UTC),
		},
	}
	
	statements := []*models.BankStatement{
		{
			UniqueIdentifier: "BS001",
			Amount:           decimal.NewFromFloat(100.50), // Exact match with TX001
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS002",
			Amount:           decimal.NewFromFloat(-250.00), // Exact match with TX002 (debit)
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS003",
			Amount:           decimal.NewFromFloat(75.30), // Close match with TX003 (small difference)
			Date:             time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS004",
			Amount:           decimal.NewFromFloat(500.00), // No match
			Date:             time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
		},
	}
	
	return transactions, statements
}

func TestNewMatchingEngine(t *testing.T) {
	// Test with nil config
	engine := NewMatchingEngine(nil)
	if engine == nil {
		t.Fatal("Expected matching engine to be created")
	}
	
	if engine.Config == nil {
		t.Fatal("Expected default config to be set")
	}
	
	// Test with custom config
	config := StrictMatchingConfig()
	engine = NewMatchingEngine(config)
	
	if engine.Config != config {
		t.Error("Expected custom config to be set")
	}
}

func TestMatchingEngine_LoadTransactions(t *testing.T) {
	engine := NewMatchingEngine(nil)
	transactions, _ := createTestMatchingData()
	
	engine.LoadTransactions(transactions)
	
	if engine.TransactionIndex == nil {
		t.Fatal("Expected transaction index to be created")
	}
	
	if len(engine.TransactionIndex.AllTransactions) != len(transactions) {
		t.Errorf("Expected %d transactions, got %d", 
			len(transactions), len(engine.TransactionIndex.AllTransactions))
	}
}

func TestMatchingEngine_LoadBankStatements(t *testing.T) {
	engine := NewMatchingEngine(nil)
	_, statements := createTestMatchingData()
	
	engine.LoadBankStatements(statements)
	
	if engine.BankStatementIndex == nil {
		t.Fatal("Expected bank statement index to be created")
	}
	
	if len(engine.BankStatementIndex.AllStatements) != len(statements) {
		t.Errorf("Expected %d statements, got %d", 
			len(statements), len(engine.BankStatementIndex.AllStatements))
	}
}

func TestMatchingEngine_Reconcile(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	transactions, statements := createTestMatchingData()
	
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	result, err := engine.Reconcile()
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}
	
	if result == nil {
		t.Fatal("Expected reconciliation result")
	}
	
	// Should find some matches
	if len(result.Matches) == 0 {
		t.Error("Expected to find at least some matches")
	}
	
	// Check summary
	if result.Summary.TotalTransactions != len(transactions) {
		t.Errorf("Expected %d total transactions, got %d", 
			len(transactions), result.Summary.TotalTransactions)
	}
	
	if result.Summary.TotalBankStatements != len(statements) {
		t.Errorf("Expected %d total statements, got %d", 
			len(statements), result.Summary.TotalBankStatements)
	}
	
	// Total should equal matched + unmatched
	totalMatched := result.Summary.MatchedTransactions
	totalUnmatched := result.Summary.UnmatchedTransactions
	if totalMatched+totalUnmatched != result.Summary.TotalTransactions {
		t.Errorf("Matched (%d) + Unmatched (%d) should equal Total (%d)", 
			totalMatched, totalUnmatched, result.Summary.TotalTransactions)
	}
}

func TestMatchingEngine_Reconcile_WithoutData(t *testing.T) {
	engine := NewMatchingEngine(nil)
	
	// Test without transactions
	_, err := engine.Reconcile()
	if err == nil {
		t.Error("Expected error when reconciling without transactions")
	}
	
	// Load transactions but not statements
	transactions, _ := createTestMatchingData()
	engine.LoadTransactions(transactions)
	
	_, err = engine.Reconcile()
	if err == nil {
		t.Error("Expected error when reconciling without bank statements")
	}
}

func TestMatchingEngine_FindMatches(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	transactions, statements := createTestMatchingData()
	
	engine.LoadBankStatements(statements)
	
	// Test finding matches for a transaction
	tx := transactions[0] // TX001 - should match BS001
	matches, err := engine.FindMatches(tx)
	if err != nil {
		t.Fatalf("FindMatches failed: %v", err)
	}
	
	if len(matches) == 0 {
		t.Error("Expected to find matches for TX001")
	}
	
	// Best match should be high confidence
	if len(matches) > 0 && matches[0].ConfidenceScore < 0.8 {
		t.Errorf("Expected high confidence match, got %f", matches[0].ConfidenceScore)
	}
}

func TestMatchingEngine_FindMatchesForStatement(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	transactions, statements := createTestMatchingData()
	
	engine.LoadTransactions(transactions)
	
	// Test finding matches for a bank statement
	stmt := statements[0] // BS001 - should match TX001
	matches, err := engine.FindMatchesForStatement(stmt)
	if err != nil {
		t.Fatalf("FindMatchesForStatement failed: %v", err)
	}
	
	if len(matches) == 0 {
		t.Error("Expected to find matches for BS001")
	}
	
	// Best match should be high confidence
	if len(matches) > 0 && matches[0].ConfidenceScore < 0.8 {
		t.Errorf("Expected high confidence match, got %f", matches[0].ConfidenceScore)
	}
}

func TestMatchingEngine_scoreMatch(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	// Test exact match
	tx := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.50),
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}
	
	stmt := &models.BankStatement{
		UniqueIdentifier: "BS001",
		Amount:           decimal.NewFromFloat(100.50),
		Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	
	result := engine.scoreMatch(tx, stmt)
	
	if result.ConfidenceScore < 0.9 {
		t.Errorf("Expected high confidence for exact match, got %f", result.ConfidenceScore)
	}
	
	if result.MatchType != MatchExact && result.MatchType != MatchClose {
		t.Errorf("Expected exact or close match type, got %s", result.MatchType.String())
	}
	
	if len(result.Reasons) == 0 {
		t.Error("Expected match reasons to be generated")
	}
}

func TestMatchingEngine_calculateAmountScore(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	tx := &models.Transaction{
		Amount: decimal.NewFromFloat(100.00),
		Type:   models.TransactionTypeCredit,
	}
	
	// Test exact match
	stmt := &models.BankStatement{
		Amount: decimal.NewFromFloat(100.00),
	}
	
	score := engine.calculateAmountScore(tx, stmt)
	if score != 1.0 {
		t.Errorf("Expected score 1.0 for exact match, got %f", score)
	}
	
	// Test with tolerance
	engine.Config.AmountTolerancePercent = 1.0 // 1% tolerance
	stmt.Amount = decimal.NewFromFloat(100.50) // 0.5% difference
	
	score = engine.calculateAmountScore(tx, stmt)
	if score <= 0.0 {
		t.Errorf("Expected positive score within tolerance, got %f", score)
	}
	
	// Test outside tolerance
	stmt.Amount = decimal.NewFromFloat(105.00) // 5% difference
	score = engine.calculateAmountScore(tx, stmt)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 outside tolerance, got %f", score)
	}
}

func TestMatchingEngine_calculateDateScore(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	txTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	
	// Test exact date match (normalize to handle timezone issues)
	stmtTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	normalizedTxTime := engine.Config.NormalizeTime(txTime)
	normalizedStmtTime := engine.Config.NormalizeTime(stmtTime)
	score := engine.calculateDateScore(normalizedTxTime, normalizedStmtTime)
	if score != 1.0 {
		t.Errorf("Expected score 1.0 for same date, got %f", score)
	}
	
	// Test within tolerance
	engine.Config.DateToleranceDays = 2
	stmtTime = time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC) // 1 day later
	normalizedStmtTime = engine.Config.NormalizeTime(stmtTime)
	score = engine.calculateDateScore(normalizedTxTime, normalizedStmtTime)
	if score <= 0.0 {
		t.Errorf("Expected positive score within tolerance, got %f", score)
	}
	
	// Test outside tolerance
	stmtTime = time.Date(2024, 1, 20, 0, 0, 0, 0, time.UTC) // 5 days later
	normalizedStmtTime = engine.Config.NormalizeTime(stmtTime)
	score = engine.calculateDateScore(normalizedTxTime, normalizedStmtTime)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 outside tolerance, got %f", score)
	}
}

func TestMatchingEngine_calculateTypeScore(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	// Test with type matching disabled
	engine.Config.EnableTypeMatching = false
	
	tx := &models.Transaction{
		Type: models.TransactionTypeCredit,
	}
	
	stmt := &models.BankStatement{
		Amount: decimal.NewFromFloat(-100.00), // Should be debit
	}
	
	score := engine.calculateTypeScore(tx, stmt)
	if score != 1.0 {
		t.Errorf("Expected score 1.0 when type matching disabled, got %f", score)
	}
	
	// Test with type matching enabled
	engine.Config.EnableTypeMatching = true
	
	// Test matching types (credit transaction, positive amount)
	stmt.Amount = decimal.NewFromFloat(100.00)
	score = engine.calculateTypeScore(tx, stmt)
	if score != 1.0 {
		t.Errorf("Expected score 1.0 for matching types, got %f", score)
	}
	
	// Test mismatched types (credit transaction, negative amount)
	stmt.Amount = decimal.NewFromFloat(-100.00)
	score = engine.calculateTypeScore(tx, stmt)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 for mismatched types, got %f", score)
	}
}

func TestMatchingEngine_determineMatchType(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	// Test exact match
	matchType := engine.determineMatchType(0.98, 1.0, 1.0, 1.0)
	if matchType != MatchExact {
		t.Errorf("Expected MatchExact, got %s", matchType.String())
	}
	
	// Test close match
	matchType = engine.determineMatchType(0.87, 0.9, 0.8, 1.0)
	if matchType != MatchClose {
		t.Errorf("Expected MatchClose, got %s", matchType.String())
	}
	
	// Test fuzzy match
	matchType = engine.determineMatchType(0.75, 0.7, 0.8, 1.0)
	if matchType != MatchFuzzy {
		t.Errorf("Expected MatchFuzzy, got %s", matchType.String())
	}
	
	// Test possible match (using score above minimum confidence threshold)
	engine.Config.MinConfidenceScore = 0.6 // Set lower threshold for test
	matchType = engine.determineMatchType(0.65, 0.6, 0.7, 1.0)
	if matchType != MatchPossible {
		t.Errorf("Expected MatchPossible, got %s", matchType.String())
	}
	
	// Test no match
	matchType = engine.determineMatchType(0.5, 0.4, 0.6, 1.0)
	if matchType != MatchNone {
		t.Errorf("Expected MatchNone, got %s", matchType.String())
	}
}

func TestMatchingEngine_ValidateConfiguration(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	// Valid configuration should pass
	err := engine.ValidateConfiguration()
	if err != nil {
		t.Errorf("Valid configuration should pass validation: %v", err)
	}
	
	// Invalid configuration should fail
	engine.Config.AmountTolerancePercent = -1.0 // Invalid
	err = engine.ValidateConfiguration()
	if err == nil {
		t.Error("Invalid configuration should fail validation")
	}
}

func TestMatchingEngine_UpdateConfiguration(t *testing.T) {
	engine := NewMatchingEngine(DefaultMatchingConfig())
	originalConfig := engine.GetConfiguration()
	
	// Update with valid config
	newConfig := StrictMatchingConfig()
	err := engine.UpdateConfiguration(newConfig)
	if err != nil {
		t.Errorf("Valid config update should succeed: %v", err)
	}
	
	// Verify config was updated
	if engine.Config.DateToleranceDays == originalConfig.DateToleranceDays {
		t.Error("Configuration should have been updated")
	}
	
	// Update with invalid config should fail
	invalidConfig := &MatchingConfig{
		AmountTolerancePercent: -1.0, // Invalid
	}
	
	err = engine.UpdateConfiguration(invalidConfig)
	if err == nil {
		t.Error("Invalid config update should fail")
	}
}

func TestMatchingEngine_GetStats(t *testing.T) {
	engine := NewMatchingEngine(nil)
	transactions, statements := createTestMatchingData()
	
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	txStats, stmtStats := engine.GetStats()
	
	if txStats.TotalTransactions != len(transactions) {
		t.Errorf("Expected %d transaction stats, got %d", 
			len(transactions), txStats.TotalTransactions)
	}
	
	if stmtStats.TotalTransactions != len(statements) {
		t.Errorf("Expected %d statement stats, got %d", 
			len(statements), stmtStats.TotalTransactions)
	}
}

func TestReconciliationResult_Summary(t *testing.T) {
	engine := NewMatchingEngine(RelaxedMatchingConfig())
	transactions, statements := createTestMatchingData()
	
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	result, err := engine.Reconcile()
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}
	
	summary := result.Summary
	
	// Verify summary totals
	if summary.MatchedTransactions+summary.UnmatchedTransactions != summary.TotalTransactions {
		t.Error("Summary transaction totals don't add up")
	}
	
	if summary.MatchedStatements+summary.UnmatchedStatements != summary.TotalBankStatements {
		t.Error("Summary statement totals don't add up")
	}
	
	// Verify match type counts
	matchTypeTotal := summary.ExactMatches + summary.CloseMatches + 
					 summary.FuzzyMatches + summary.PossibleMatches
	
	if matchTypeTotal != summary.MatchedTransactions {
		t.Errorf("Match type totals (%d) don't equal matched transactions (%d)", 
			matchTypeTotal, summary.MatchedTransactions)
	}
}

// Benchmark tests
func BenchmarkMatchingEngine_Reconcile(b *testing.B) {
	// Create larger dataset
	transactions := make([]*models.Transaction, 1000)
	statements := make([]*models.BankStatement, 1000)
	
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < 1000; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i) * 1.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.AddDate(0, 0, i%30),
		}
		
		statements[i] = &models.BankStatement{
			UniqueIdentifier: "BS" + string(rune(i)),
			Amount:           decimal.NewFromFloat(float64(i) * 1.5),
			Date:             baseTime.AddDate(0, 0, i%30),
		}
	}
	
	engine := NewMatchingEngine(DefaultMatchingConfig())
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Reconcile()
	}
}

func BenchmarkMatchingEngine_FindMatches(b *testing.B) {
	// Create test data
	transactions := make([]*models.Transaction, 100)
	statements := make([]*models.BankStatement, 100)
	
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < 100; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i) * 1.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.AddDate(0, 0, i%10),
		}
		
		statements[i] = &models.BankStatement{
			UniqueIdentifier: "BS" + string(rune(i)),
			Amount:           decimal.NewFromFloat(float64(i) * 1.5),
			Date:             baseTime.AddDate(0, 0, i%10),
		}
	}
	
	engine := NewMatchingEngine(DefaultMatchingConfig())
	engine.LoadBankStatements(statements)
	
	testTx := transactions[50]
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.FindMatches(testTx)
	}
}