package matcher

import (
	"testing"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

// TestFullReconciliationWorkflow tests the complete end-to-end reconciliation process
func TestFullReconciliationWorkflow(t *testing.T) {
	// Create comprehensive test dataset
	transactions := createComprehensiveTransactionDataset()
	statements := createComprehensiveBankStatementDataset()
	
	// Test with different configurations
	configs := []*MatchingConfig{
		StrictMatchingConfig(),
		DefaultMatchingConfig(),
		RelaxedMatchingConfig(),
	}
	
	for _, config := range configs {
		t.Run("Config_"+config.String(), func(t *testing.T) {
			engine := NewMatchingEngine(config)
			engine.LoadTransactions(transactions)
			engine.LoadBankStatements(statements)
			
			// Perform reconciliation
			result, err := engine.Reconcile()
			if err != nil {
				t.Fatalf("Reconciliation failed: %v", err)
			}
			
			validateReconciliationResult(t, result, len(transactions), len(statements))
		})
	}
}

// TestEdgeCaseIntegration tests integration of edge case handling with the main engine
func TestEdgeCaseIntegration(t *testing.T) {
	config := DefaultMatchingConfig()
	config.EnablePartialMatching = true
	config.MaxPartialMatchRatio = 0.2
	
	engine := NewMatchingEngine(config)
	edgeHandler := NewEdgeCaseHandler(config)
	
	// Create test data with edge cases
	transactions := createEdgeCaseTransactionData()
	statements := createEdgeCaseBankStatementData()
	
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	// Test duplicate detection
	duplicates := edgeHandler.DetectDuplicates(transactions)
	if len(duplicates.Groups) == 0 {
		t.Error("Expected to detect duplicate groups in edge case data")
	}
	
	// Test same-day transaction handling
	sameDayMatches, err := edgeHandler.HandleSameDayTransactions(transactions, statements, engine)
	if err != nil {
		t.Fatalf("Same-day handling failed: %v", err)
	}
	
	if len(sameDayMatches) == 0 {
		t.Error("Expected to find same-day matches in edge case data")
	}
	
	// Test partial matching for specific transaction
	partialTx := findLargeTransaction(transactions)
	if partialTx != nil {
		candidates := engine.BankStatementIndex.GetCandidates(partialTx, config)
		partialResults := edgeHandler.HandlePartialMatches(partialTx, candidates)
		
		if len(partialResults) == 0 {
			t.Log("No partial matches found (this may be expected depending on test data)")
		}
	}
	
	// Perform full reconciliation
	result, err := engine.Reconcile()
	if err != nil {
		t.Fatalf("Edge case reconciliation failed: %v", err)
	}
	
	validateReconciliationResult(t, result, len(transactions), len(statements))
}

// TestPerformanceWithLargeDataset tests performance with a large dataset
func TestPerformanceWithLargeDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	// Create large dataset
	transactions := createLargeTransactionDataset(10000)
	statements := createLargeBankStatementDataset(10000)
	
	engine := NewMatchingEngine(DefaultMatchingConfig())
	
	startTime := time.Now()
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	loadTime := time.Since(startTime)
	
	t.Logf("Data loading time for 10k+10k records: %v", loadTime)
	
	startTime = time.Now()
	result, err := engine.Reconcile()
	reconcileTime := time.Since(startTime)
	
	if err != nil {
		t.Fatalf("Large dataset reconciliation failed: %v", err)
	}
	
	t.Logf("Reconciliation time for 10k+10k records: %v", reconcileTime)
	t.Logf("Matched %d/%d transactions", result.Summary.MatchedTransactions, result.Summary.TotalTransactions)
	t.Logf("Match types - Exact: %d, Close: %d, Fuzzy: %d, Possible: %d",
		result.Summary.ExactMatches, result.Summary.CloseMatches,
		result.Summary.FuzzyMatches, result.Summary.PossibleMatches)
	
	// Performance requirements
	if reconcileTime > 10*time.Second {
		t.Errorf("Reconciliation took too long: %v (expected < 10s)", reconcileTime)
	}
}

// TestAccuracyWithKnownMatches tests matching accuracy with predetermined correct matches
func TestAccuracyWithKnownMatches(t *testing.T) {
	// Create dataset with known correct matches
	transactions, statements, expectedMatches := createKnownMatchDataset()
	
	engine := NewMatchingEngine(DefaultMatchingConfig())
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	result, err := engine.Reconcile()
	if err != nil {
		t.Fatalf("Accuracy test reconciliation failed: %v", err)
	}
	
	// Analyze accuracy
	correctMatches := 0
	for _, match := range result.Matches {
		if isExpectedMatch(match, expectedMatches) {
			correctMatches++
		}
	}
	
	accuracy := float64(correctMatches) / float64(len(expectedMatches))
	t.Logf("Matching accuracy: %.2f%% (%d/%d correct)", accuracy*100, correctMatches, len(expectedMatches))
	
	// Expect at least 90% accuracy for known matches
	if accuracy < 0.9 {
		t.Errorf("Matching accuracy too low: %.2f%% (expected >= 90%%)", accuracy*100)
	}
}

// TestConcurrentReconciliation tests thread safety and concurrent operations
func TestConcurrentReconciliation(t *testing.T) {
	transactions := createComprehensiveTransactionDataset()
	statements := createComprehensiveBankStatementDataset()
	
	// Test multiple concurrent reconciliations
	numGoroutines := 5
	results := make(chan *ReconciliationResult, numGoroutines)
	errors := make(chan error, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			engine := NewMatchingEngine(DefaultMatchingConfig())
			engine.LoadTransactions(transactions)
			engine.LoadBankStatements(statements)
			
			result, err := engine.Reconcile()
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}
	
	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errors:
			t.Fatalf("Concurrent reconciliation failed: %v", err)
		case result := <-results:
			validateReconciliationResult(t, result, len(transactions), len(statements))
		case <-time.After(30 * time.Second):
			t.Fatal("Concurrent reconciliation timed out")
		}
	}
}

// Helper functions for creating test datasets

func createComprehensiveTransactionDataset() []*models.Transaction {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	return []*models.Transaction{
		// Exact matches
		{TrxID: "TX001", Amount: decimal.NewFromFloat(100.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime},
		{TrxID: "TX002", Amount: decimal.NewFromFloat(250.00), Type: models.TransactionTypeDebit, TransactionTime: baseTime.Add(1 * time.Hour)},
		{TrxID: "TX003", Amount: decimal.NewFromFloat(75.50), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(2 * time.Hour)},
		
		// Close matches (small amount differences)
		{TrxID: "TX004", Amount: decimal.NewFromFloat(199.95), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(3 * time.Hour)},
		{TrxID: "TX005", Amount: decimal.NewFromFloat(50.02), Type: models.TransactionTypeDebit, TransactionTime: baseTime.Add(4 * time.Hour)},
		
		// Date tolerance matches
		{TrxID: "TX006", Amount: decimal.NewFromFloat(300.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(24 * time.Hour)},
		
		// Unmatched transactions
		{TrxID: "TX007", Amount: decimal.NewFromFloat(999.99), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(5 * time.Hour)},
		{TrxID: "TX008", Amount: decimal.NewFromFloat(123.45), Type: models.TransactionTypeDebit, TransactionTime: baseTime.Add(48 * time.Hour)},
	}
}

func createComprehensiveBankStatementDataset() []*models.BankStatement {
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	
	return []*models.BankStatement{
		// Exact matches
		{UniqueIdentifier: "BS001", Amount: decimal.NewFromFloat(100.00), Date: baseTime},
		{UniqueIdentifier: "BS002", Amount: decimal.NewFromFloat(-250.00), Date: baseTime.Add(1 * time.Hour)},
		{UniqueIdentifier: "BS003", Amount: decimal.NewFromFloat(75.50), Date: baseTime.Add(2 * time.Hour)},
		
		// Close matches
		{UniqueIdentifier: "BS004", Amount: decimal.NewFromFloat(200.00), Date: baseTime.Add(3 * time.Hour)},
		{UniqueIdentifier: "BS005", Amount: decimal.NewFromFloat(-50.00), Date: baseTime.Add(4 * time.Hour)},
		
		// Date tolerance matches
		{UniqueIdentifier: "BS006", Amount: decimal.NewFromFloat(300.00), Date: baseTime.Add(25 * time.Hour)},
		
		// Unmatched statements
		{UniqueIdentifier: "BS007", Amount: decimal.NewFromFloat(500.00), Date: baseTime.Add(6 * time.Hour)},
		{UniqueIdentifier: "BS008", Amount: decimal.NewFromFloat(-777.77), Date: baseTime.Add(72 * time.Hour)},
	}
}

func createEdgeCaseTransactionData() []*models.Transaction {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	return []*models.Transaction{
		// Potential duplicates
		{TrxID: "TX001", Amount: decimal.NewFromFloat(100.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime},
		{TrxID: "TX002", Amount: decimal.NewFromFloat(100.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(5 * time.Minute)},
		
		// Same-day transactions
		{TrxID: "TX003", Amount: decimal.NewFromFloat(50.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(1 * time.Hour)},
		{TrxID: "TX004", Amount: decimal.NewFromFloat(75.00), Type: models.TransactionTypeDebit, TransactionTime: baseTime.Add(2 * time.Hour)},
		{TrxID: "TX005", Amount: decimal.NewFromFloat(25.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(3 * time.Hour)},
		
		// Large transaction for partial matching
		{TrxID: "TX006", Amount: decimal.NewFromFloat(500.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(4 * time.Hour)},
	}
}

func createEdgeCaseBankStatementData() []*models.BankStatement {
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	
	return []*models.BankStatement{
		// Potential matches for duplicates
		{UniqueIdentifier: "BS001", Amount: decimal.NewFromFloat(100.00), Date: baseTime},
		
		// Same-day statements
		{UniqueIdentifier: "BS002", Amount: decimal.NewFromFloat(50.00), Date: baseTime},
		{UniqueIdentifier: "BS003", Amount: decimal.NewFromFloat(-75.00), Date: baseTime},
		{UniqueIdentifier: "BS004", Amount: decimal.NewFromFloat(25.00), Date: baseTime},
		
		// Partial amounts for large transaction
		{UniqueIdentifier: "BS005", Amount: decimal.NewFromFloat(200.00), Date: baseTime},
		{UniqueIdentifier: "BS006", Amount: decimal.NewFromFloat(150.00), Date: baseTime},
		{UniqueIdentifier: "BS007", Amount: decimal.NewFromFloat(100.00), Date: baseTime},
		{UniqueIdentifier: "BS008", Amount: decimal.NewFromFloat(50.00), Date: baseTime},
	}
}

func createLargeTransactionDataset(size int) []*models.Transaction {
	transactions := make([]*models.Transaction, size)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < size; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i%1000) + 0.01),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.AddDate(0, 0, i%365),
		}
	}
	
	return transactions
}

func createLargeBankStatementDataset(size int) []*models.BankStatement {
	statements := make([]*models.BankStatement, size)
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < size; i++ {
		// Create some matching and some non-matching statements
		amount := decimal.NewFromFloat(float64(i%1000) + 0.01)
		if i%3 == 0 {
			amount = amount.Add(decimal.NewFromFloat(0.50)) // Slight difference
		}
		
		statements[i] = &models.BankStatement{
			UniqueIdentifier: "BS" + string(rune(i)),
			Amount:           amount,
			Date:             baseTime.AddDate(0, 0, i%365),
		}
	}
	
	return statements
}

func createKnownMatchDataset() ([]*models.Transaction, []*models.BankStatement, []ExpectedMatch) {
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	transactions := []*models.Transaction{
		{TrxID: "TX001", Amount: decimal.NewFromFloat(100.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime},
		{TrxID: "TX002", Amount: decimal.NewFromFloat(250.00), Type: models.TransactionTypeDebit, TransactionTime: baseTime.Add(1 * time.Hour)},
		{TrxID: "TX003", Amount: decimal.NewFromFloat(75.00), Type: models.TransactionTypeCredit, TransactionTime: baseTime.Add(2 * time.Hour)},
	}
	
	statements := []*models.BankStatement{
		{UniqueIdentifier: "BS001", Amount: decimal.NewFromFloat(100.00), Date: baseTime},
		{UniqueIdentifier: "BS002", Amount: decimal.NewFromFloat(-250.00), Date: baseTime.Add(1 * time.Hour)},
		{UniqueIdentifier: "BS003", Amount: decimal.NewFromFloat(75.00), Date: baseTime.Add(2 * time.Hour)},
	}
	
	expectedMatches := []ExpectedMatch{
		{TransactionID: "TX001", StatementID: "BS001"},
		{TransactionID: "TX002", StatementID: "BS002"},
		{TransactionID: "TX003", StatementID: "BS003"},
	}
	
	return transactions, statements, expectedMatches
}

func findLargeTransaction(transactions []*models.Transaction) *models.Transaction {
	for _, tx := range transactions {
		if tx.Amount.GreaterThan(decimal.NewFromFloat(400.00)) {
			return tx
		}
	}
	return nil
}

func validateReconciliationResult(t *testing.T, result *ReconciliationResult, expectedTxCount, expectedStmtCount int) {
	if result == nil {
		t.Fatal("Expected reconciliation result")
	}
	
	// Validate summary totals
	if result.Summary.TotalTransactions != expectedTxCount {
		t.Errorf("Expected %d total transactions, got %d", expectedTxCount, result.Summary.TotalTransactions)
	}
	
	if result.Summary.TotalBankStatements != expectedStmtCount {
		t.Errorf("Expected %d total statements, got %d", expectedStmtCount, result.Summary.TotalBankStatements)
	}
	
	// Validate that matched + unmatched = total
	if result.Summary.MatchedTransactions+result.Summary.UnmatchedTransactions != result.Summary.TotalTransactions {
		t.Error("Matched + unmatched transactions should equal total transactions")
	}
	
	if result.Summary.MatchedStatements+result.Summary.UnmatchedStatements != result.Summary.TotalBankStatements {
		t.Error("Matched + unmatched statements should equal total statements")
	}
	
	// Validate match type counts
	matchTypeTotal := result.Summary.ExactMatches + result.Summary.CloseMatches + 
					 result.Summary.FuzzyMatches + result.Summary.PossibleMatches
	
	if matchTypeTotal != result.Summary.MatchedTransactions {
		t.Error("Sum of match types should equal total matched transactions")
	}
	
	// Validate that amounts are non-negative
	if result.Summary.TotalAmountMatched.IsNegative() {
		t.Error("Total matched amount should not be negative")
	}
	
	if result.Summary.TotalAmountUnmatched.IsNegative() {
		t.Error("Total unmatched amount should not be negative")
	}
}

func isExpectedMatch(match *MatchResult, expectedMatches []ExpectedMatch) bool {
	for _, expected := range expectedMatches {
		if match.Transaction.TrxID == expected.TransactionID &&
		   match.BankStatement.UniqueIdentifier == expected.StatementID {
			return true
		}
	}
	return false
}

type ExpectedMatch struct {
	TransactionID string
	StatementID   string
}