package matcher

import (
	"testing"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

func createTestDuplicateTransactions() []*models.Transaction {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	
	return []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime,
		},
		{
			TrxID:           "TX002", // Potential duplicate of TX001
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.Add(5 * time.Minute),
		},
		{
			TrxID:           "TX003", // Different amount, not a duplicate
			Amount:          decimal.NewFromFloat(200.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime,
		},
		{
			TrxID:           "TX004", // Another potential duplicate of TX001
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.Add(10 * time.Minute),
		},
	}
}

func createTestSameDayData() ([]*models.Transaction, []*models.BankStatement) {
	baseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	
	transactions := []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseDate.Add(10 * time.Hour),
		},
		{
			TrxID:           "TX002",
			Amount:          decimal.NewFromFloat(250.00),
			Type:            models.TransactionTypeDebit,
			TransactionTime: baseDate.Add(14 * time.Hour),
		},
		{
			TrxID:           "TX003",
			Amount:          decimal.NewFromFloat(75.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseDate.Add(16 * time.Hour),
		},
	}
	
	statements := []*models.BankStatement{
		{
			UniqueIdentifier: "BS001",
			Amount:           decimal.NewFromFloat(100.00),
			Date:             baseDate,
		},
		{
			UniqueIdentifier: "BS002",
			Amount:           decimal.NewFromFloat(-250.00),
			Date:             baseDate,
		},
		{
			UniqueIdentifier: "BS003",
			Amount:           decimal.NewFromFloat(75.00),
			Date:             baseDate,
		},
	}
	
	return transactions, statements
}

func createTestPartialMatchData() (*models.Transaction, []*models.BankStatement) {
	// Transaction for $100.00
	transaction := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}
	
	// Bank statements that could add up to $100.00
	statements := []*models.BankStatement{
		{
			UniqueIdentifier: "BS001",
			Amount:           decimal.NewFromFloat(60.00),
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS002",
			Amount:           decimal.NewFromFloat(40.00),
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS003",
			Amount:           decimal.NewFromFloat(25.00),
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS004",
			Amount:           decimal.NewFromFloat(200.00), // Too large
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	
	return transaction, statements
}

func TestNewEdgeCaseHandler(t *testing.T) {
	config := DefaultMatchingConfig()
	handler := NewEdgeCaseHandler(config)
	
	if handler == nil {
		t.Fatal("Expected edge case handler to be created")
	}
	
	if handler.Config != config {
		t.Error("Expected config to be set correctly")
	}
}

func TestEdgeCaseHandler_DetectDuplicates(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	transactions := createTestDuplicateTransactions()
	
	result := handler.DetectDuplicates(transactions)
	
	if result == nil {
		t.Fatal("Expected duplicate detection result")
	}
	
	// Should find one group with TX001, TX002, and TX004 (all same amount and close in time)
	if len(result.Groups) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(result.Groups))
	}
	
	if len(result.Groups) > 0 {
		group := result.Groups[0]
		if len(group.Transactions) != 3 {
			t.Errorf("Expected 3 transactions in duplicate group, got %d", len(group.Transactions))
		}
		
		if group.Confidence <= 0.0 {
			t.Errorf("Expected positive confidence score, got %f", group.Confidence)
		}
		
		if group.Reason == "" {
			t.Error("Expected duplicate reason to be generated")
		}
	}
}

func TestEdgeCaseHandler_HandleSameDayTransactions(t *testing.T) {
	config := DefaultMatchingConfig()
	handler := NewEdgeCaseHandler(config)
	engine := NewMatchingEngine(config)
	
	transactions, statements := createTestSameDayData()
	
	engine.LoadTransactions(transactions)
	engine.LoadBankStatements(statements)
	
	sameDayMatches, err := handler.HandleSameDayTransactions(transactions, statements, engine)
	if err != nil {
		t.Fatalf("HandleSameDayTransactions failed: %v", err)
	}
	
	// Should find one same-day match group (all transactions and statements on same date)
	if len(sameDayMatches) != 1 {
		t.Errorf("Expected 1 same-day match, got %d", len(sameDayMatches))
	}
	
	if len(sameDayMatches) > 0 {
		match := sameDayMatches[0]
		
		if len(match.Transactions) != 3 {
			t.Errorf("Expected 3 transactions in same-day match, got %d", len(match.Transactions))
		}
		
		if len(match.Statements) != 3 {
			t.Errorf("Expected 3 statements in same-day match, got %d", len(match.Statements))
		}
		
		if match.ResolutionStrategy == "" {
			t.Error("Expected resolution strategy to be determined")
		}
	}
}

func TestEdgeCaseHandler_HandlePartialMatches(t *testing.T) {
	config := DefaultMatchingConfig()
	config.EnablePartialMatching = true
	config.MaxPartialMatchRatio = 0.1
	
	handler := NewEdgeCaseHandler(config)
	transaction, statements := createTestPartialMatchData()
	
	results := handler.HandlePartialMatches(transaction, statements)
	
	// Should find at least one partial match (BS001 + BS002 = $100.00)
	if len(results) == 0 {
		t.Error("Expected to find partial matches")
	}
	
	// Check the best result
	if len(results) > 0 {
		bestResult := results[0]
		
		if bestResult.Transaction != transaction {
			t.Error("Expected transaction to match")
		}
		
		if len(bestResult.PartialStatements) < 2 {
			t.Errorf("Expected at least 2 partial statements, got %d", len(bestResult.PartialStatements))
		}
		
		if bestResult.Confidence <= 0.0 {
			t.Errorf("Expected positive confidence, got %f", bestResult.Confidence)
		}
		
		// Check that total amount is close to transaction amount
		expectedAmount := transaction.Amount.Abs()
		if bestResult.TotalAmount.Sub(expectedAmount).Abs().GreaterThan(decimal.NewFromFloat(1.0)) {
			t.Errorf("Expected total amount close to %s, got %s", 
				expectedAmount.String(), bestResult.TotalAmount.String())
		}
	}
}

func TestEdgeCaseHandler_HandlePartialMatches_Disabled(t *testing.T) {
	config := DefaultMatchingConfig()
	config.EnablePartialMatching = false
	
	handler := NewEdgeCaseHandler(config)
	transaction, statements := createTestPartialMatchData()
	
	results := handler.HandlePartialMatches(transaction, statements)
	
	// Should return nil when partial matching is disabled
	if results != nil {
		t.Error("Expected nil result when partial matching is disabled")
	}
}

func TestEdgeCaseHandler_NormalizeTimezones(t *testing.T) {
	config := DefaultMatchingConfig()
	config.TimezoneHandling = TimezoneUTC
	
	handler := NewEdgeCaseHandler(config)
	
	// Create test data with different timezones
	localTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.Local)
	
	transactions := []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: localTime,
		},
	}
	
	statements := []*models.BankStatement{
		{
			UniqueIdentifier: "BS001",
			Amount:           decimal.NewFromFloat(100.00),
			Date:             localTime,
		},
	}
	
	handler.NormalizeTimezones(transactions, statements)
	
	// Check that times were normalized to UTC
	if transactions[0].TransactionTime.Location() != time.UTC {
		t.Error("Expected transaction time to be normalized to UTC")
	}
	
	if statements[0].Date.Location() != time.UTC {
		t.Error("Expected statement date to be normalized to UTC")
	}
}

func TestEdgeCaseHandler_ResolveTimezoneMismatch(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	// Create transaction and statement with potential timezone issues
	tx := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 15, 23, 30, 0, 0, time.UTC), // Late UTC time
	}
	
	stmt := &models.BankStatement{
		UniqueIdentifier: "BS001",
		Amount:           decimal.NewFromFloat(100.00),
		Date:             time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC), // Next day UTC
	}
	
	resolution, err := handler.ResolveTimezoneMismatch(tx, stmt)
	if err != nil {
		t.Fatalf("ResolveTimezoneMismatch failed: %v", err)
	}
	
	if resolution == nil {
		t.Fatal("Expected timezone resolution result")
	}
	
	if resolution.Transaction != tx {
		t.Error("Expected transaction to match")
	}
	
	if resolution.Statement != stmt {
		t.Error("Expected statement to match")
	}
	
	if resolution.Confidence < 0.0 {
		t.Errorf("Expected non-negative confidence, got %f", resolution.Confidence)
	}
}

func TestEdgeCaseHandler_HandleCurrencyMismatch(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	// Transaction in USD
	tx := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.00), // $100 USD
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}
	
	// Statement in EUR
	stmt := &models.BankStatement{
		UniqueIdentifier: "BS001",
		Amount:           decimal.NewFromFloat(85.00), // €85 EUR
		Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	
	// Exchange rate: 1 USD = 0.85 EUR
	exchangeRate := decimal.NewFromFloat(0.85)
	
	result := handler.HandleCurrencyMismatch(tx, stmt, exchangeRate)
	
	if result == nil {
		t.Fatal("Expected currency match result")
	}
	
	if result.Transaction != tx {
		t.Error("Expected transaction to match")
	}
	
	if result.Statement != stmt {
		t.Error("Expected statement to match")
	}
	
	if !result.ExchangeRate.Equal(exchangeRate) {
		t.Error("Expected exchange rate to match")
	}
	
	expectedConverted := decimal.NewFromFloat(85.00) // 100 * 0.85
	if !result.ConvertedAmount.Equal(expectedConverted) {
		t.Errorf("Expected converted amount %s, got %s", 
			expectedConverted.String(), result.ConvertedAmount.String())
	}
	
	if !result.IsMatch {
		t.Error("Expected currency conversion to result in match")
	}
	
	if result.Confidence <= 0.0 {
		t.Errorf("Expected positive confidence for currency match, got %f", result.Confidence)
	}
}

func TestEdgeCaseHandler_isPotentialDuplicate(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	
	tx1 := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: baseTime,
	}
	
	// Same amount, type, and close time - should be potential duplicate
	tx2 := &models.Transaction{
		TrxID:           "TX002",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: baseTime.Add(10 * time.Minute),
	}
	
	if !handler.isPotentialDuplicate(tx1, tx2) {
		t.Error("Expected transactions to be identified as potential duplicates")
	}
	
	// Different amount - should not be duplicate
	tx3 := &models.Transaction{
		TrxID:           "TX003",
		Amount:          decimal.NewFromFloat(200.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: baseTime,
	}
	
	if handler.isPotentialDuplicate(tx1, tx3) {
		t.Error("Expected transactions with different amounts to not be duplicates")
	}
	
	// Different type - should not be duplicate
	tx4 := &models.Transaction{
		TrxID:           "TX004",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeDebit,
		TransactionTime: baseTime,
	}
	
	if handler.isPotentialDuplicate(tx1, tx4) {
		t.Error("Expected transactions with different types to not be duplicates")
	}
	
	// Too far apart in time - should not be duplicate
	tx5 := &models.Transaction{
		TrxID:           "TX005",
		Amount:          decimal.NewFromFloat(100.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: baseTime.Add(2 * time.Hour),
	}
	
	if handler.isPotentialDuplicate(tx1, tx5) {
		t.Error("Expected transactions too far apart in time to not be duplicates")
	}
}

func TestEdgeCaseHandler_calculateDuplicateConfidence(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	
	// Single transaction - should return 0
	singleTx := []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime,
		},
	}
	
	confidence := handler.calculateDuplicateConfidence(singleTx)
	if confidence != 0.0 {
		t.Errorf("Expected 0 confidence for single transaction, got %f", confidence)
	}
	
	// Perfect duplicates - should return high confidence
	perfectDuplicates := []*models.Transaction{
		{
			TrxID:           "TX001",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime,
		},
		{
			TrxID:           "TX002",
			Amount:          decimal.NewFromFloat(100.00),
			Type:            models.TransactionTypeCredit,
			TransactionTime: baseTime.Add(1 * time.Minute),
		},
	}
	
	confidence = handler.calculateDuplicateConfidence(perfectDuplicates)
	if confidence < 0.8 {
		t.Errorf("Expected high confidence for perfect duplicates, got %f", confidence)
	}
}

func TestEdgeCaseHandler_generateCombinations(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	statements := []*models.BankStatement{
		{UniqueIdentifier: "BS001", Amount: decimal.NewFromFloat(25.00)},
		{UniqueIdentifier: "BS002", Amount: decimal.NewFromFloat(50.00)},
		{UniqueIdentifier: "BS003", Amount: decimal.NewFromFloat(75.00)},
	}
	
	combinations := handler.generateCombinations(statements, 2, 3)
	
	// Should generate combinations of size 2 and 3
	// Size 2: [BS001,BS002], [BS001,BS003], [BS002,BS003]
	// Size 3: [BS001,BS002,BS003]
	// Total: 4 combinations
	
	if len(combinations) != 4 {
		t.Errorf("Expected 4 combinations, got %d", len(combinations))
	}
	
	// Check that we have different sizes
	hasSizeTwo := false
	hasSizeThree := false
	
	for _, combo := range combinations {
		if len(combo) == 2 {
			hasSizeTwo = true
		} else if len(combo) == 3 {
			hasSizeThree = true
		}
	}
	
	if !hasSizeTwo {
		t.Error("Expected to find combinations of size 2")
	}
	
	if !hasSizeThree {
		t.Error("Expected to find combinations of size 3")
	}
}

func TestEdgeCaseHandler_calculateTimezoneMatchScore(t *testing.T) {
	handler := NewEdgeCaseHandler(DefaultMatchingConfig())
	
	// Same date should get score 1.0
	txTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	stmtTime := time.Date(2024, 1, 15, 14, 45, 0, 0, time.UTC) // Same date, different time
	
	score := handler.calculateTimezoneMatchScore(txTime, stmtTime)
	if score != 1.0 {
		t.Errorf("Expected score 1.0 for same date, got %f", score)
	}
	
	// One day apart should get lower score
	stmtTime = time.Date(2024, 1, 16, 10, 30, 0, 0, time.UTC)
	score = handler.calculateTimezoneMatchScore(txTime, stmtTime)
	if score <= 0.0 || score >= 1.0 {
		t.Errorf("Expected score between 0 and 1 for one day apart, got %f", score)
	}
	
	// Many days apart should get score 0.0
	stmtTime = time.Date(2024, 1, 25, 10, 30, 0, 0, time.UTC)
	score = handler.calculateTimezoneMatchScore(txTime, stmtTime)
	if score != 0.0 {
		t.Errorf("Expected score 0.0 for many days apart, got %f", score)
	}
}