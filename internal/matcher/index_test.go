package matcher

import (
	"testing"
	"time"

	"golang-reconciliation-service/internal/models"

	"github.com/shopspring/decimal"
)

func createTestTransactions() []*models.Transaction {
	return []*models.Transaction{
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
			Amount:          decimal.NewFromFloat(100.50), // Same amount as TX001
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 16, 9, 15, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX004",
			Amount:          decimal.NewFromFloat(75.25),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 16, 16, 45, 0, 0, time.UTC),
		},
	}
}

func createTestBankStatements() []*models.BankStatement {
	return []*models.BankStatement{
		{
			UniqueIdentifier: "BS001",
			Amount:           decimal.NewFromFloat(100.50),
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS002",
			Amount:           decimal.NewFromFloat(-250.00), // Negative for debit
			Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS003",
			Amount:           decimal.NewFromFloat(75.25),
			Date:             time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS004",
			Amount:           decimal.NewFromFloat(100.50), // Same amount as BS001
			Date:             time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC),
		},
	}
}

func TestNewTransactionIndex(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	if index == nil {
		t.Fatal("Expected transaction index to be created")
	}
	
	if len(index.AllTransactions) != len(transactions) {
		t.Errorf("Expected %d transactions, got %d", len(transactions), len(index.AllTransactions))
	}
	
	// Check that indexes are built
	if len(index.ExactAmountIndex) == 0 {
		t.Error("Expected exact amount index to be populated")
	}
	
	if len(index.DateIndex) == 0 {
		t.Error("Expected date index to be populated")
	}
	
	if len(index.TypeIndex) == 0 {
		t.Error("Expected type index to be populated")
	}
	
	if len(index.AmountRangeIndex) == 0 {
		t.Error("Expected amount range index to be populated")
	}
}

func TestNewBankStatementIndex(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	if index == nil {
		t.Fatal("Expected bank statement index to be created")
	}
	
	if len(index.AllStatements) != len(statements) {
		t.Errorf("Expected %d statements, got %d", len(statements), len(index.AllStatements))
	}
	
	// Check that indexes are built
	if len(index.ExactAmountIndex) == 0 {
		t.Error("Expected exact amount index to be populated")
	}
	
	if len(index.DateIndex) == 0 {
		t.Error("Expected date index to be populated")
	}
	
	if len(index.AmountRangeIndex) == 0 {
		t.Error("Expected amount range index to be populated")
	}
}

func TestTransactionIndex_GetByExactAmount(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Test exact amount lookup
	amount := decimal.NewFromFloat(100.50)
	results := index.GetByExactAmount(amount)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 transactions with amount 100.50, got %d", len(results))
	}
	
	// Verify the returned transactions have the correct amount
	for _, tx := range results {
		if !tx.Amount.Equal(amount) {
			t.Errorf("Expected amount %s, got %s", amount.String(), tx.Amount.String())
		}
	}
	
	// Test non-existent amount
	nonExistentAmount := decimal.NewFromFloat(999.99)
	results = index.GetByExactAmount(nonExistentAmount)
	if len(results) != 0 {
		t.Errorf("Expected 0 transactions for non-existent amount, got %d", len(results))
	}
}

func TestBankStatementIndex_GetByExactAmount(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	// Test exact amount lookup
	amount := decimal.NewFromFloat(100.50)
	results := index.GetByExactAmount(amount)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 statements with amount 100.50, got %d", len(results))
	}
	
	// Verify the returned statements have the correct amount
	for _, stmt := range results {
		if !stmt.Amount.Equal(amount) {
			t.Errorf("Expected amount %s, got %s", amount.String(), stmt.Amount.String())
		}
	}
}

func TestTransactionIndex_GetByAmountRange(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Test range that includes multiple transactions
	minAmount := decimal.NewFromFloat(75.00)
	maxAmount := decimal.NewFromFloat(150.00)
	results := index.GetByAmountRange(minAmount, maxAmount)
	
	if len(results) != 3 {
		t.Errorf("Expected 3 transactions in range 75-150, got %d", len(results))
	}
	
	// Verify all results are within range
	for _, tx := range results {
		if tx.Amount.LessThan(minAmount) || tx.Amount.GreaterThan(maxAmount) {
			t.Errorf("Transaction amount %s is outside range %s - %s", 
				tx.Amount.String(), minAmount.String(), maxAmount.String())
		}
	}
	
	// Test narrow range
	minAmount = decimal.NewFromFloat(100.00)
	maxAmount = decimal.NewFromFloat(101.00)
	results = index.GetByAmountRange(minAmount, maxAmount)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 transactions in narrow range, got %d", len(results))
	}
}

func TestBankStatementIndex_GetByAmountRange(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	// Test range that includes both positive and negative amounts
	minAmount := decimal.NewFromFloat(-300.00)
	maxAmount := decimal.NewFromFloat(200.00)
	results := index.GetByAmountRange(minAmount, maxAmount)
	
	if len(results) != 4 {
		t.Errorf("Expected 4 statements in range -300 to 200, got %d", len(results))
	}
	
	// Verify all results are within range
	for _, stmt := range results {
		if stmt.Amount.LessThan(minAmount) || stmt.Amount.GreaterThan(maxAmount) {
			t.Errorf("Statement amount %s is outside range %s - %s", 
				stmt.Amount.String(), minAmount.String(), maxAmount.String())
		}
	}
}

func TestTransactionIndex_GetByDate(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Test date with multiple transactions
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	results := index.GetByDate(date)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 transactions on 2024-01-15, got %d", len(results))
	}
	
	// Verify all results have the correct date
	for _, tx := range results {
		if tx.TransactionTime.Format("2006-01-02") != "2024-01-15" {
			t.Errorf("Expected date 2024-01-15, got %s", 
				tx.TransactionTime.Format("2006-01-02"))
		}
	}
	
	// Test date with no transactions
	emptyDate := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	results = index.GetByDate(emptyDate)
	
	if len(results) != 0 {
		t.Errorf("Expected 0 transactions on empty date, got %d", len(results))
	}
}

func TestBankStatementIndex_GetByDate(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	// Test date with multiple statements
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	results := index.GetByDate(date)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 statements on 2024-01-15, got %d", len(results))
	}
	
	// Verify all results have the correct date
	for _, stmt := range results {
		if stmt.Date.Format("2006-01-02") != "2024-01-15" {
			t.Errorf("Expected date 2024-01-15, got %s", 
				stmt.Date.Format("2006-01-02"))
		}
	}
}

func TestTransactionIndex_GetByDateRange(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Test range covering all transactions
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	results := index.GetByDateRange(startDate, endDate)
	
	if len(results) != 4 {
		t.Errorf("Expected 4 transactions in date range, got %d", len(results))
	}
	
	// Test single day range
	singleDay := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	results = index.GetByDateRange(singleDay, singleDay)
	
	if len(results) != 2 {
		t.Errorf("Expected 2 transactions on single day, got %d", len(results))
	}
}

func TestBankStatementIndex_GetByDateRange(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	// Test range covering multiple days
	startDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC)
	results := index.GetByDateRange(startDate, endDate)
	
	if len(results) != 4 {
		t.Errorf("Expected 4 statements in date range, got %d", len(results))
	}
}

func TestTransactionIndex_GetByType(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Test credit transactions
	creditResults := index.GetByType(models.TransactionTypeCredit)
	if len(creditResults) != 3 {
		t.Errorf("Expected 3 credit transactions, got %d", len(creditResults))
	}
	
	// Test debit transactions
	debitResults := index.GetByType(models.TransactionTypeDebit)
	if len(debitResults) != 1 {
		t.Errorf("Expected 1 debit transaction, got %d", len(debitResults))
	}
	
	// Verify transaction types
	for _, tx := range creditResults {
		if tx.Type != models.TransactionTypeCredit {
			t.Errorf("Expected credit transaction, got %s", tx.Type)
		}
	}
	
	for _, tx := range debitResults {
		if tx.Type != models.TransactionTypeDebit {
			t.Errorf("Expected debit transaction, got %s", tx.Type)
		}
	}
}

func TestTransactionIndex_GetCandidates(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	// Create test bank statement
	stmt := &models.BankStatement{
		UniqueIdentifier: "BS001",
		Amount:           decimal.NewFromFloat(100.50),
		Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	
	// Test with default config
	config := DefaultMatchingConfig()
	candidates := index.GetCandidates(stmt, config)
	
	// Should find transactions with matching amount
	if len(candidates) == 0 {
		t.Error("Expected to find candidate transactions")
	}
	
	// Test with strict config (no tolerance)
	strictConfig := StrictMatchingConfig()
	candidates = index.GetCandidates(stmt, strictConfig)
	
	// Should still find exact matches
	if len(candidates) == 0 {
		t.Error("Expected to find exact match candidates")
	}
	
	// Test with negative amount (should match debit transactions)
	debitStmt := &models.BankStatement{
		UniqueIdentifier: "BS002",
		Amount:           decimal.NewFromFloat(-250.00),
		Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	
	candidates = index.GetCandidates(debitStmt, config)
	if len(candidates) == 0 {
		t.Error("Expected to find debit transaction candidates")
	}
}

func TestBankStatementIndex_GetCandidates(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	// Create test transaction
	tx := &models.Transaction{
		TrxID:           "TX001",
		Amount:          decimal.NewFromFloat(100.50),
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}
	
	// Test with default config
	config := DefaultMatchingConfig()
	candidates := index.GetCandidates(tx, config)
	
	// Should find bank statements with matching amount
	if len(candidates) == 0 {
		t.Error("Expected to find candidate bank statements")
	}
	
	// Test debit transaction (should find negative bank statement)
	debitTx := &models.Transaction{
		TrxID:           "TX002",
		Amount:          decimal.NewFromFloat(250.00),
		Type:            models.TransactionTypeDebit,
		TransactionTime: time.Date(2024, 1, 15, 14, 20, 0, 0, time.UTC),
	}
	
	candidates = index.GetCandidates(debitTx, config)
	if len(candidates) == 0 {
		t.Error("Expected to find negative bank statement candidates")
	}
}

func TestTransactionIndex_GetIndexStats(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	stats := index.GetIndexStats()
	
	if stats.TotalTransactions != 4 {
		t.Errorf("Expected 4 total transactions, got %d", stats.TotalTransactions)
	}
	
	if stats.UniqueAmounts != 3 { // 100.50, 250.00, 75.25
		t.Errorf("Expected 3 unique amounts, got %d", stats.UniqueAmounts)
	}
	
	if stats.UniqueDates != 2 { // 2024-01-15, 2024-01-16
		t.Errorf("Expected 2 unique dates, got %d", stats.UniqueDates)
	}
	
	if stats.UniqueTypes != 2 { // CREDIT, DEBIT
		t.Errorf("Expected 2 unique types, got %d", stats.UniqueTypes)
	}
}

func TestBankStatementIndex_GetIndexStats(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	stats := index.GetIndexStats()
	
	if stats.TotalTransactions != 4 {
		t.Errorf("Expected 4 total statements, got %d", stats.TotalTransactions)
	}
	
	if stats.UniqueAmounts != 3 { // 100.50, -250.00, 75.25
		t.Errorf("Expected 3 unique amounts, got %d", stats.UniqueAmounts)
	}
	
	if stats.UniqueDates != 3 { // 2024-01-15, 2024-01-16, 2024-01-17
		t.Errorf("Expected 3 unique dates, got %d", stats.UniqueDates)
	}
	
	if stats.UniqueTypes != 0 { // Bank statements don't have types
		t.Errorf("Expected 0 unique types for bank statements, got %d", stats.UniqueTypes)
	}
}

func TestTransactionIndex_AddTransaction(t *testing.T) {
	transactions := createTestTransactions()
	index := NewTransactionIndex(transactions)
	
	initialCount := len(index.AllTransactions)
	
	// Add new transaction
	newTx := &models.Transaction{
		TrxID:           "TX005",
		Amount:          decimal.NewFromFloat(200.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
	}
	
	index.AddTransaction(newTx)
	
	if len(index.AllTransactions) != initialCount+1 {
		t.Errorf("Expected %d transactions after add, got %d", 
			initialCount+1, len(index.AllTransactions))
	}
	
	// Verify the transaction can be found in indexes
	results := index.GetByExactAmount(decimal.NewFromFloat(200.00))
	if len(results) != 1 {
		t.Errorf("Expected 1 transaction with amount 200.00, got %d", len(results))
	}
	
	// Verify date index
	dateResults := index.GetByDate(time.Date(2024, 1, 17, 0, 0, 0, 0, time.UTC))
	if len(dateResults) != 1 {
		t.Errorf("Expected 1 transaction on 2024-01-17, got %d", len(dateResults))
	}
	
	// Verify type index
	creditResults := index.GetByType(models.TransactionTypeCredit)
	found := false
	for _, tx := range creditResults {
		if tx.TrxID == "TX005" {
			found = true
			break
		}
	}
	if !found {
		t.Error("New transaction not found in type index")
	}
}

func TestBankStatementIndex_AddStatement(t *testing.T) {
	statements := createTestBankStatements()
	index := NewBankStatementIndex(statements)
	
	initialCount := len(index.AllStatements)
	
	// Add new statement
	newStmt := &models.BankStatement{
		UniqueIdentifier: "BS005",
		Amount:           decimal.NewFromFloat(300.00),
		Date:             time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC),
	}
	
	index.AddStatement(newStmt)
	
	if len(index.AllStatements) != initialCount+1 {
		t.Errorf("Expected %d statements after add, got %d", 
			initialCount+1, len(index.AllStatements))
	}
	
	// Verify the statement can be found in indexes
	results := index.GetByExactAmount(decimal.NewFromFloat(300.00))
	if len(results) != 1 {
		t.Errorf("Expected 1 statement with amount 300.00, got %d", len(results))
	}
	
	// Verify date index
	dateResults := index.GetByDate(time.Date(2024, 1, 18, 0, 0, 0, 0, time.UTC))
	if len(dateResults) != 1 {
		t.Errorf("Expected 1 statement on 2024-01-18, got %d", len(dateResults))
	}
}

// Benchmark tests
func BenchmarkTransactionIndex_GetByExactAmount(b *testing.B) {
	// Create large dataset
	transactions := make([]*models.Transaction, 10000)
	for i := 0; i < 10000; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i) * 1.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}
	}
	
	index := NewTransactionIndex(transactions)
	lookupAmount := decimal.NewFromFloat(5000 * 1.5)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.GetByExactAmount(lookupAmount)
	}
}

func BenchmarkTransactionIndex_GetByAmountRange(b *testing.B) {
	// Create large dataset
	transactions := make([]*models.Transaction, 10000)
	for i := 0; i < 10000; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i) * 1.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}
	}
	
	index := NewTransactionIndex(transactions)
	minAmount := decimal.NewFromFloat(5000.0)
	maxAmount := decimal.NewFromFloat(6000.0)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.GetByAmountRange(minAmount, maxAmount)
	}
}

func BenchmarkTransactionIndex_GetCandidates(b *testing.B) {
	// Create large dataset
	transactions := make([]*models.Transaction, 10000)
	for i := 0; i < 10000; i++ {
		transactions[i] = &models.Transaction{
			TrxID:           "TX" + string(rune(i)),
			Amount:          decimal.NewFromFloat(float64(i) * 1.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		}
	}
	
	index := NewTransactionIndex(transactions)
	config := DefaultMatchingConfig()
	
	stmt := &models.BankStatement{
		UniqueIdentifier: "BS001",
		Amount:           decimal.NewFromFloat(5000.0),
		Date:             time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.GetCandidates(stmt, config)
	}
}