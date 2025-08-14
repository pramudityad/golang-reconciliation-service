package reconciler

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/parsers"

	"github.com/shopspring/decimal"
)

func TestReconciliationOrchestrator_AdvancedFeatures(t *testing.T) {
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
	
	// Create orchestrator with preprocessing
	orchestrator, err := NewReconciliationOrchestrator(service, DefaultPreprocessingConfig())
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Add progress callback
	var progressUpdates []*ReconciliationProgress
	orchestrator.AddProgressCallback(func(progress *ReconciliationProgress) {
		progressUpdates = append(progressUpdates, progress)
	})
	
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
	
	// Create advanced options
	options := DefaultReconciliationOptions()
	options.EnablePreprocessing = true
	options.IncludeDataQuality = true
	options.PerformDiscrepancyAnalysis = true
	
	// Process reconciliation with advanced features
	ctx := context.Background()
	result, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
	if err != nil {
		t.Fatalf("Advanced reconciliation failed: %v", err)
	}
	
	// Validate results
	if result == nil {
		t.Fatal("Expected enhanced reconciliation result, got nil")
	}
	
	if result.ReconciliationResult == nil {
		t.Fatal("Expected base reconciliation result")
	}
	
	if result.OptionsUsed == nil {
		t.Fatal("Expected options to be recorded in result")
	}
	
	// Check that progress callbacks were called
	if len(progressUpdates) == 0 {
		t.Error("Expected progress updates")
	}
	
	// Validate final progress
	finalProgress := progressUpdates[len(progressUpdates)-1]
	if finalProgress.PercentComplete != 100.0 {
		t.Errorf("Expected 100%% completion, got %.1f%%", finalProgress.PercentComplete)
	}
	
	if finalProgress.CurrentStep != "Completed" {
		t.Errorf("Expected final step to be 'Completed', got '%s'", finalProgress.CurrentStep)
	}
	
	// Check enhanced features
	if options.IncludeDataQuality && result.DataQualityMetrics == nil {
		t.Error("Expected data quality metrics when requested")
	}
	
	t.Logf("Advanced reconciliation completed with %d progress updates", len(progressUpdates))
	t.Logf("Final result: %d matches found", result.Summary.MatchedTransactions)
}

func TestReconciliationOrchestrator_WithFiltering(t *testing.T) {
	// Create test data with various amounts
	tmpDir, err := os.MkdirTemp("", "filtering_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	// System transactions with various amounts
	systemFile := filepath.Join(tmpDir, "transactions.csv")
	systemCSV := `trxID,amount,type,transactionTime
TX001,5.00,CREDIT,2024-01-15T10:30:00Z
TX002,100.50,CREDIT,2024-01-15T14:20:00Z
TX003,1000.00,DEBIT,2024-01-16T09:15:00Z
TX004,50.75,CREDIT,2024-01-17T11:00:00Z
TX005,2500.00,CREDIT,2024-01-18T16:30:00Z`
	
	if err := os.WriteFile(systemFile, []byte(systemCSV), 0644); err != nil {
		t.Fatalf("Failed to write system file: %v", err)
	}
	
	// Bank statements
	bankFile := filepath.Join(tmpDir, "bank_statements.csv")
	bankCSV := `unique_identifier,amount,date
BS001,5.00,2024-01-15
BS002,100.50,2024-01-15
BS003,-1000.00,2024-01-16
BS004,50.75,2024-01-17
BS005,2500.00,2024-01-18`
	
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
	
	// Create service and orchestrator
	service, err := NewReconciliationService(
		txConfig,
		bankConfig,
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	orchestrator, err := NewReconciliationOrchestrator(service, nil)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Create request
	request := &ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:        []string{bankFile},
		TransactionConfig: txConfig,
		BankConfigs:      map[string]*parsers.BankConfig{bankFile: bankConfig},
	}
	
	// Test with amount filtering
	options := DefaultReconciliationOptions()
	options.AmountThresholds = &AmountThresholds{
		MinAmount:    decimal.NewFromFloat(50.0),
		MaxAmount:    decimal.NewFromFloat(1500.0),
		ExcludeZero:  true,
	}
	
	// Process reconciliation
	ctx := context.Background()
	result, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
	if err != nil {
		t.Fatalf("Filtered reconciliation failed: %v", err)
	}
	
	// Should have fewer transactions due to amount filtering
	// Expected: TX002 (100.50), TX003 (1000.00), TX004 (50.75)
	// Excluded: TX001 (5.00 < 50), TX005 (2500.00 > 1500)
	expectedTransactions := 3
	if result.Summary.TotalTransactions != expectedTransactions {
		t.Errorf("Expected %d transactions after filtering, got %d", 
			expectedTransactions, result.Summary.TotalTransactions)
	}
	
	t.Logf("Amount filtering: %d transactions remain after filtering", result.Summary.TotalTransactions)
}

func TestReconciliationOrchestrator_TypeFiltering(t *testing.T) {
	// Setup test data
	systemFile, bankFiles, cleanup := createTestDataFiles(t)
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	service, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base(bankFiles[0])],
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	orchestrator, err := NewReconciliationOrchestrator(service, nil)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
	}
	
	// Create request with proper file mapping
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
	
	// Test filtering by transaction type (CREDIT only)
	options := DefaultReconciliationOptions()
	options.TransactionTypeFilter = []models.TransactionType{models.TransactionTypeCredit}
	
	// Process reconciliation
	ctx := context.Background()
	result, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
	if err != nil {
		t.Fatalf("Type-filtered reconciliation failed: %v", err)
	}
	
	// Should have fewer transactions (only CREDIT transactions)
	totalWithoutFilter := 7  // From test data
	if result.Summary.TotalTransactions >= totalWithoutFilter {
		t.Errorf("Expected fewer transactions with type filtering, got %d", result.Summary.TotalTransactions)
	}
	
	t.Logf("Type filtering: %d CREDIT transactions found", result.Summary.TotalTransactions)
}

func TestReconciliationOrchestrator_CustomMatchingConfig(t *testing.T) {
	// Setup test data
	systemFile, bankFiles, cleanup := createTestDataFiles(t)
	defer cleanup()
	
	txConfig, bankConfigs := createTestConfigs()
	
	service, err := NewReconciliationService(
		txConfig,
		bankConfigs[filepath.Base(bankFiles[0])],
		matcher.DefaultMatchingConfig(),
		DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("Failed to create reconciliation service: %v", err)
	}
	
	orchestrator, err := NewReconciliationOrchestrator(service, nil)
	if err != nil {
		t.Fatalf("Failed to create orchestrator: %v", err)
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
	
	// Test with custom matching configuration
	customMatchingConfig := &matcher.MatchingConfig{
		DateToleranceDays:               1,     // Strict date matching
		AmountTolerancePercent:          0.5,   // Very strict amount matching
		MinConfidenceScore:              0.95,  // High confidence requirement
		EnableTypeMatching:              true,
		MaxCandidatesPerTransaction:     100,
	}
	
	options := DefaultReconciliationOptions()
	options.CustomMatchingConfig = customMatchingConfig
	options.UseAdvancedMatching = true
	
	// Process reconciliation
	ctx := context.Background()
	result, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
	if err != nil {
		t.Fatalf("Custom matching reconciliation failed: %v", err)
	}
	
	// With strict matching, we might have fewer but higher quality matches
	if result.Summary.MatchedTransactions > 0 {
		// All matches should be high confidence
		t.Logf("Custom matching: %d matches with strict configuration", result.Summary.MatchedTransactions)
		t.Logf("Match breakdown: %d exact, %d close, %d fuzzy", 
			result.Summary.ExactMatches, result.Summary.CloseMatches, result.Summary.FuzzyMatches)
	}
}

func TestDataPreprocessor_TransactionPreprocessing(t *testing.T) {
	preprocessor := NewDataPreprocessor(DefaultPreprocessingConfig())
	
	// Create test transactions with various issues
	transactions := []*models.Transaction{
		{
			TrxID:           "  TX001  ", // Extra whitespace
			Amount:          decimal.NewFromFloat(100.5),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX002",
			Amount:          decimal.NewFromFloat(250.123456), // Many decimal places
			Type:            models.TransactionTypeDebit,
			TransactionTime: time.Date(2024, 1, 15, 14, 20, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX003",
			Amount:          decimal.Zero, // Zero amount (should be flagged)
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Date(2024, 1, 16, 9, 15, 0, 0, time.UTC),
		},
		{
			TrxID:           "TX004",
			Amount:          decimal.NewFromFloat(100.0),
			Type:            models.TransactionTypeCredit,
			TransactionTime: time.Time{}, // Zero time (should be flagged)
		},
	}
	
	// Preprocess transactions
	processed, err := preprocessor.PreprocessTransactions(transactions)
	
	// Should have some errors but continue processing with fixes
	if err == nil {
		t.Error("Expected preprocessing errors for invalid data")
	}
	
	// Should have processed some transactions
	if len(processed) == 0 {
		t.Error("Expected some transactions to be processed successfully")
	}
	
	// Check that whitespace was trimmed
	for _, tx := range processed {
		if tx.TrxID != strings.TrimSpace(tx.TrxID) {
			t.Errorf("Expected trimmed transaction ID, got '%s'", tx.TrxID)
		}
	}
	
	t.Logf("Preprocessed %d/%d transactions successfully", len(processed), len(transactions))
}

func TestDataPreprocessor_BankStatementPreprocessing(t *testing.T) {
	config := DefaultPreprocessingConfig()
	config.FixCommonErrors = true
	
	preprocessor := NewDataPreprocessor(config)
	
	// Create test statements with various issues
	statements := []*models.BankStatement{
		{
			UniqueIdentifier: "  BS001  ", // Extra whitespace
			Amount:          decimal.NewFromFloat(100.5),
			Date:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "BS002",
			Amount:          decimal.Zero, // Zero amount (should be fixed)
			Date:            time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			UniqueIdentifier: "", // Empty identifier (should be fixed)
			Amount:          decimal.NewFromFloat(75.0),
			Date:            time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		},
	}
	
	// Preprocess statements
	processed, _ := preprocessor.PreprocessBankStatements(statements)
	
	// With error fixing enabled, should process all statements
	if len(processed) != len(statements) {
		t.Errorf("Expected %d processed statements, got %d", len(statements), len(processed))
	}
	
	// Check that fixes were applied
	foundAutoGeneratedID := false
	foundFixedAmount := false
	
	for _, stmt := range processed {
		if strings.HasPrefix(stmt.UniqueIdentifier, "AUTO_") {
			foundAutoGeneratedID = true
		}
		if stmt.Amount.Equal(decimal.NewFromFloat(0.01)) {
			foundFixedAmount = true
		}
	}
	
	if !foundAutoGeneratedID {
		t.Error("Expected auto-generated ID for empty identifier")
	}
	
	if !foundFixedAmount {
		t.Error("Expected fixed amount for zero amount")
	}
	
	t.Logf("Preprocessed %d statements with error fixing", len(processed))
}

// Benchmark test for orchestrator performance
func BenchmarkReconciliationOrchestrator_AdvancedProcessing(b *testing.B) {
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
	
	orchestrator, err := NewReconciliationOrchestrator(service, DefaultPreprocessingConfig())
	if err != nil {
		b.Fatalf("Failed to create orchestrator: %v", err)
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
	
	options := DefaultReconciliationOptions()
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
		if err != nil {
			b.Fatalf("Advanced reconciliation failed: %v", err)
		}
	}
}