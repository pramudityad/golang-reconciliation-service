package reconciler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/parsers"

	"github.com/shopspring/decimal"
)

// Test fixtures and test data setup

func createTestDataFiles(t *testing.T) (string, []string, func()) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "reconciliation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	// Create system transactions CSV
	systemFile := filepath.Join(tmpDir, "transactions.csv")
	systemCSV := `trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z
TX002,250.00,DEBIT,2024-01-15T14:20:00Z
TX003,75.25,CREDIT,2024-01-16T09:15:00Z
TX004,100.00,CREDIT,2024-01-17T11:00:00Z
TX005,50.75,DEBIT,2024-01-18T16:30:00Z
TX006,1000.00,CREDIT,2024-01-19T12:00:00Z
TX007,25.50,DEBIT,2024-01-20T08:45:00Z`
	
	if err := os.WriteFile(systemFile, []byte(systemCSV), 0644); err != nil {
		t.Fatalf("Failed to write system file: %v", err)
	}
	
	// Create bank statement files
	bank1File := filepath.Join(tmpDir, "bank1_statements.csv")
	bank1CSV := `unique_identifier,amount,date
BS001,100.50,2024-01-15
BS002,-250.00,2024-01-15
BS003,75.30,2024-01-16
BS004,50.00,2024-01-19`
	
	if err := os.WriteFile(bank1File, []byte(bank1CSV), 0644); err != nil {
		t.Fatalf("Failed to write bank1 file: %v", err)
	}
	
	bank2File := filepath.Join(tmpDir, "bank2_statements.csv")
	bank2CSV := `unique_identifier,amount,date
BS101,100.00,2024-01-17
BS102,-50.75,2024-01-18
BS103,1000.00,2024-01-19
BS104,-25.50,2024-01-20`
	
	if err := os.WriteFile(bank2File, []byte(bank2CSV), 0644); err != nil {
		t.Fatalf("Failed to write bank2 file: %v", err)
	}
	
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	
	return systemFile, []string{bank1File, bank2File}, cleanup
}

func createTestConfigs() (*parsers.TransactionParserConfig, map[string]*parsers.BankConfig) {
	txConfig := &parsers.TransactionParserConfig{
		HasHeader:             true,
		Delimiter:             ',',
		TrxIDColumn:           "trxID",
		AmountColumn:          "amount",
		TypeColumn:            "type",
		TransactionTimeColumn: "transactionTime",
	}
	
	bankConfigs := map[string]*parsers.BankConfig{
		"bank1_statements.csv": {
			Name:             "Bank1",
			HasHeader:        true,
			Delimiter:        ',',
			IdentifierColumn: "unique_identifier",
			AmountColumn:     "amount",
			DateColumn:       "date",
		},
		"bank2_statements.csv": {
			Name:             "Bank2",
			HasHeader:        true,
			Delimiter:        ',',
			IdentifierColumn: "unique_identifier",
			AmountColumn:     "amount",
			DateColumn:       "date",
		},
	}
	
	return txConfig, bankConfigs
}

func TestReconciliationService_BasicReconciliation(t *testing.T) {
	// Setup test data
	systemFile, bankFiles, cleanup := createTestDataFiles(t)
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	// Create reconciliation service
	service, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base(bankFiles[0])], // Use first bank config for initialization
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	// Create request with proper file-to-config mapping
	fileConfigs := make(map[string]*parsers.BankConfig)
	for _, file := range bankFiles {
		baseFile := filepath.Base(file)
		if config, exists := bankConfigs[baseFile]; exists {
			fileConfigs[file] = config
		}
	}
	
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        bankFiles,
		TransactionConfig: txConfig,
		BankConfigs:      fileConfigs,
	}
	
	// Process reconciliation
	ctx := context.Background()
	result, err := service.ProcessReconciliation(ctx, request)
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}
	
	// Validate results
	if result == nil {
		t.Fatal("Expected reconciliation result, got nil")
	}
	
	if result.Summary == nil {
		t.Fatal("Expected summary in result")
	}
	
	// Check that we have some transactions and statements
	if result.Summary.TotalTransactions == 0 {
		t.Error("Expected to find transactions")
	}
	
	if result.Summary.TotalBankStatements == 0 {
		t.Error("Expected to find bank statements")
	}
	
	// Check that some matches were found
	if result.Summary.MatchedTransactions == 0 {
		t.Error("Expected to find some matched transactions")
	}
	
	t.Logf("Reconciliation completed: %d transactions, %d statements, %d matches",
		result.Summary.TotalTransactions,
		result.Summary.TotalBankStatements,
		result.Summary.MatchedTransactions)
}

func TestReconciliationService_WithDateRange(t *testing.T) {
	// Setup test data
	systemFile, bankFiles, cleanup := createTestDataFiles(t)
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	// Create reconciliation service
	service, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base(bankFiles[0])],
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	// Set date range to filter transactions
	startDate := time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 18, 23, 59, 59, 0, time.UTC)
	
	// Create request with date filtering
	fileConfigs := make(map[string]*parsers.BankConfig)
	for _, file := range bankFiles {
		baseFile := filepath.Base(file)
		if config, exists := bankConfigs[baseFile]; exists {
			fileConfigs[file] = config
		}
	}
	
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        bankFiles,
		StartDate:        &startDate,
		EndDate:          &endDate,
		TransactionConfig: txConfig,
		BankConfigs:      fileConfigs,
	}
	
	// Process reconciliation
	ctx := context.Background()
	result, err := service.ProcessReconciliation(ctx, request)
	if err != nil {
		t.Fatalf("Reconciliation with date range failed: %v", err)
	}
	
	// Validate that date range was applied
	if result.Summary.DateRange == nil {
		t.Error("Expected date range in summary")
	}
	
	if !result.Summary.DateRange.Start.Equal(startDate) {
		t.Errorf("Expected start date %v, got %v", startDate, result.Summary.DateRange.Start)
	}
	
	if !result.Summary.DateRange.End.Equal(endDate) {
		t.Errorf("Expected end date %v, got %v", endDate, result.Summary.DateRange.End)
	}
	
	// Should have fewer transactions due to date filtering
	if result.Summary.TotalTransactions >= 7 {
		t.Errorf("Expected fewer transactions due to date filtering, got %d", result.Summary.TotalTransactions)
	}
	
	t.Logf("Date-filtered reconciliation: %d transactions in range %s to %s",
		result.Summary.TotalTransactions, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
}

func TestReconciliationService_ErrorHandling(t *testing.T) {
	txConfig, _ := createTestConfigs()
	
	// Create service
	service, err := NewReconciliationService(
		txConfig,
		&parsers.BankConfig{Name: "Test", HasHeader: true},
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	ctx := context.Background()
	
	// Test with invalid system file
	request := &ReconciliationRequest{
		SystemFile:        "/nonexistent/file.csv",
		BankFiles:        []string{"/nonexistent/bank.csv"},
		TransactionConfig: txConfig,
		BankConfigs:      map[string]*parsers.BankConfig{"/nonexistent/bank.csv": {Name: "Test"}},
	}
	
	_, err = service.ProcessReconciliation(ctx, request)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	
	// Test with invalid request
	invalidRequest := &ReconciliationRequest{
		SystemFile: "", // Empty system file
		BankFiles:  []string{},
	}
	
	_, err = service.ProcessReconciliation(ctx, invalidRequest)
	if err == nil {
		t.Error("Expected error for invalid request")
	}
}

func TestReconciliationService_LargeDataset(t *testing.T) {
	// Create larger test dataset
	tmpDir, err := os.MkdirTemp("", "large_reconciliation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// Generate large system file
	systemFile := filepath.Join(tmpDir, "large_transactions.csv")
	systemCSV := "trxID,amount,type,transactionTime\n"
	
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 1000; i++ {
		txTime := baseTime.Add(time.Duration(i) * time.Hour)
		amount := decimal.NewFromFloat(float64(i%1000) * 1.5)
		txType := "CREDIT"
		if i%2 == 0 {
			txType = "DEBIT"
		}
		systemCSV += fmt.Sprintf("TX%04d,%s,%s,%s\n", i, amount.String(), txType, txTime.Format(time.RFC3339))
	}
	
	if err := os.WriteFile(systemFile, []byte(systemCSV), 0644); err != nil {
		t.Fatalf("Failed to write large system file: %v", err)
	}
	
	// Generate matching bank file (subset of transactions)
	bankFile := filepath.Join(tmpDir, "large_bank_statements.csv")
	bankCSV := "unique_identifier,amount,date\n"
	
	for i := 0; i < 500; i += 2 { // Every other transaction
		txTime := baseTime.Add(time.Duration(i) * time.Hour)
		amount := decimal.NewFromFloat(float64(i%1000) * 1.5)
		if i%2 == 0 { // Debit transactions become negative in bank statements
			amount = amount.Neg()
		}
		bankCSV += fmt.Sprintf("BS%04d,%s,%s\n", i, amount.String(), txTime.Format("2006-01-02"))
	}
	
	if err := os.WriteFile(bankFile, []byte(bankCSV), 0644); err != nil {
		t.Fatalf("Failed to write large bank file: %v", err)
	}
	
	// Create configurations
	txConfig := &parsers.TransactionParserConfig{
		HasHeader:             true,
		Delimiter:             ',',
		TrxIDColumn:           "trxID",
		AmountColumn:          "amount",
		TypeColumn:            "type",
		TransactionTimeColumn: "transactionTime",
	}
	
	bankConfig := &parsers.BankConfig{
		Name:             "LargeBank",
		HasHeader:        true,
		Delimiter:        ',',
		IdentifierColumn: "unique_identifier",
		AmountColumn:     "amount",
		DateColumn:       "date",
	}
	
	// Create service
	service, err := NewReconciliationService(
		txConfig,
		bankConfig,
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	// Create request
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        []string{bankFile},
		TransactionConfig: txConfig,
		BankConfigs:      map[string]*parsers.BankConfig{bankFile: bankConfig},
	}
	
	// Process reconciliation and measure performance
	ctx := context.Background()
	startTime := time.Now()
	
	result, err := service.ProcessReconciliation(ctx, request)
	if err != nil {
		t.Fatalf("Large dataset reconciliation failed: %v", err)
	}
	
	duration := time.Since(startTime)
	
	// Validate results
	if result.Summary.TotalTransactions != 1000 {
		t.Errorf("Expected 1000 transactions, got %d", result.Summary.TotalTransactions)
	}
	
	if result.Summary.TotalBankStatements != 250 {
		t.Errorf("Expected 250 bank statements, got %d", result.Summary.TotalBankStatements)
	}
	
	// Check performance (should complete in reasonable time)
	if duration > 30*time.Second {
		t.Errorf("Large dataset processing took too long: %v", duration)
	}
	
	// Calculate records per second
	totalRecords := result.Summary.TotalTransactions + result.Summary.TotalBankStatements
	recordsPerSecond := float64(totalRecords) / duration.Seconds()
	
	t.Logf("Large dataset performance: %d records processed in %v (%.2f records/sec)",
		totalRecords, duration, recordsPerSecond)
	
	// Should find some matches
	if result.Summary.MatchedTransactions == 0 {
		t.Error("Expected to find matches in large dataset")
	}
	
	matchRate := float64(result.Summary.MatchedTransactions) / float64(result.Summary.TotalTransactions) * 100
	t.Logf("Match rate: %.2f%% (%d/%d)", matchRate, result.Summary.MatchedTransactions, result.Summary.TotalTransactions)
}

func TestReconciliationService_MultipleMatchingConfigurations(t *testing.T) {
	// Setup test data
	systemFile, bankFiles, cleanup := createTestDataFiles(t)
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	// Test different matching configurations
	configurations := []struct {
		name   string
		config *matcher.MatchingConfig
	}{
		{"Strict", matcher.StrictMatchingConfig()},
		{"Default", matcher.DefaultMatchingConfig()},
		{"Relaxed", matcher.RelaxedMatchingConfig()},
	}
	
	for _, cfg := range configurations {
		t.Run(cfg.name, func(t *testing.T) {
			// Create service with specific matching config
			service, err := NewReconciliationService(
				txConfig,
				bankConfigs[filepath.Base(bankFiles[0])],
				cfg.config,
				DefaultConfig(),
			)
			if err != nil {
				t.Fatalf("Failed to create reconciliation service: %v", err)
			}
			
			// Create request
			fileConfigs := make(map[string]*parsers.BankConfig)
			for _, file := range bankFiles {
				baseFile := filepath.Base(file)
				if config, exists := bankConfigs[baseFile]; exists {
					fileConfigs[file] = config
				}
			}
			
			request := &ReconciliationRequest{
				SystemFile:        systemFile,
				BankFiles:        bankFiles,
				TransactionConfig: txConfig,
				BankConfigs:      fileConfigs,
			}
			
			// Process reconciliation
			ctx := context.Background()
			result, err := service.ProcessReconciliation(ctx, request)
			if err != nil {
				t.Fatalf("Reconciliation failed for %s config: %v", cfg.name, err)
			}
			
			// Log results for comparison
			t.Logf("%s config: %d matches, %d exact, %d close, %d fuzzy",
				cfg.name,
				result.Summary.MatchedTransactions,
				result.Summary.ExactMatches,
				result.Summary.CloseMatches,
				result.Summary.FuzzyMatches)
		})
	}
}

func TestReconciliationService_ConfigurationValidation(t *testing.T) {
	txConfig, bankConfigs := createTestConfigs()
	
	// Test invalid configurations
	invalidConfigs := []*Config{
		{BatchSize: 0},                    // Invalid batch size
		{MaxConcurrentFiles: 0},          // Invalid concurrency
		{BatchSize: -1},                  // Negative batch size
		{MaxConcurrentFiles: -1},         // Negative concurrency
	}
	
	for i, invalidConfig := range invalidConfigs {
		t.Run(fmt.Sprintf("InvalidConfig%d", i), func(t *testing.T) {
			_, err := NewReconciliationService(
				txConfig,
				bankConfigs[filepath.Base("bank1_statements.csv")],
				matcher.DefaultMatchingConfig(),
				invalidConfig,
			)
			if err == nil {
				t.Error("Expected error for invalid configuration")
			}
		})
	}
	
	// Test valid date range
	validConfig := DefaultConfig()
	startDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	validConfig.StartDate = &startDate
	validConfig.EndDate = &endDate
	
	_, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base("bank1_statements.csv")],
		matcher.DefaultMatchingConfig(),
		validConfig,
	)
	if err != nil {
		t.Errorf("Valid configuration should not produce error: %v", err)
	}
	
	// Test invalid date range (start after end)
	invalidDateConfig := DefaultConfig()
	invalidDateConfig.StartDate = &endDate
	invalidDateConfig.EndDate = &startDate
	
	_, err = NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base("bank1_statements.csv")],
		matcher.DefaultMatchingConfig(),
		invalidDateConfig,
	)
	if err == nil {
		t.Error("Expected error for invalid date range")
	}
}

func TestReconciliationService_DiscrepancyAnalysis(t *testing.T) {
	// Create test data with known discrepancies
	tmpDir, err := os.MkdirTemp("", "discrepancy_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// System transactions
	systemFile := filepath.Join(tmpDir, "transactions.csv")
	systemCSV := `trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z
TX002,100.50,CREDIT,2024-01-15T10:30:00Z
TX003,250.00,DEBIT,2024-01-16T14:20:00Z`
	
	if err := os.WriteFile(systemFile, []byte(systemCSV), 0644); err != nil {
		t.Fatalf("Failed to write system file: %v", err)
	}
	
	// Bank statements with discrepancies
	bankFile := filepath.Join(tmpDir, "bank_statements.csv")
	bankCSV := `unique_identifier,amount,date
BS001,100.45,2024-01-15
BS002,100.55,2024-01-15
BS003,-250.10,2024-01-16`
	
	if err := os.WriteFile(bankFile, []byte(bankCSV), 0644); err != nil {
		t.Fatalf("Failed to write bank file: %v", err)
	}
	
	// Create configurations
	txConfig := &parsers.TransactionParserConfig{
		HasHeader:             true,
		Delimiter:             ',',
		TrxIDColumn:           "trxID",
		AmountColumn:          "amount",
		TypeColumn:            "type",
		TransactionTimeColumn: "transactionTime",
	}
	
	bankConfig := &parsers.BankConfig{
		Name:             "TestBank",
		HasHeader:        true,
		Delimiter:        ',',
		IdentifierColumn: "unique_identifier",
		AmountColumn:     "amount",
		DateColumn:       "date",
	}
	
	// Create service with relaxed matching to catch fuzzy matches
	service, err := NewReconciliationService(
		txConfig,
		bankConfig,
		matcher.RelaxedMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	// Create request
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        []string{bankFile},
		TransactionConfig: txConfig,
		BankConfigs:      map[string]*parsers.BankConfig{bankFile: bankConfig},
	}
	
	// Process reconciliation
	ctx := context.Background()
	result, err := service.ProcessReconciliation(ctx, request)
	if err != nil {
		t.Fatalf("Reconciliation failed: %v", err)
	}
	
	// Check for discrepancies
	if len(result.Discrepancies) == 0 {
		t.Error("Expected to find discrepancies")
	}
	
	// Check discrepancy types
	foundAmountDiscrepancy := false
	foundDuplicateDiscrepancy := false
	
	for _, discrepancy := range result.Discrepancies {
		switch discrepancy.Type {
		case DiscrepancyAmountDifference:
			foundAmountDiscrepancy = true
		case DiscrepancyDuplicateTransaction:
			foundDuplicateDiscrepancy = true
		}
	}
	
	if !foundAmountDiscrepancy {
		t.Error("Expected to find amount discrepancy")
	}
	
	if !foundDuplicateDiscrepancy {
		t.Error("Expected to find duplicate transaction discrepancy")
	}
	
	t.Logf("Found %d discrepancies", len(result.Discrepancies))
	for _, d := range result.Discrepancies {
		t.Logf("Discrepancy: %s - %s", d.Type, d.Description)
	}
}

// Benchmark tests for performance validation
func BenchmarkReconciliationService_SmallDataset(b *testing.B) {
	// Setup
	systemFile, bankFiles, cleanup := createTestDataFiles(&testing.T{})
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	service, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base(bankFiles[0])],
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		b.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	fileConfigs := make(map[string]*parsers.BankConfig)
	for _, file := range bankFiles {
		baseFile := filepath.Base(file)
		if config, exists := bankConfigs[baseFile]; exists {
			fileConfigs[file] = config
		}
	}
	
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        bankFiles,
		TransactionConfig: txConfig,
		BankConfigs:      fileConfigs,
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.ProcessReconciliation(ctx, request)
		if err != nil {
			b.Fatalf("Reconciliation failed: %v", err)
		}
	}
}