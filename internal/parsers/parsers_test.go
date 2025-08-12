package parsers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang-reconciliation-service/internal/models"
)

const testDataDir = "../../test/examples"

// Helper function to get test file path
func getTestFilePath(filename string) string {
	return filepath.Join(testDataDir, filename)
}

// Helper function to create temporary CSV file
func createTempCSVFile(t *testing.T, content string) string {
	tmpFile, err := os.CreateTemp("", "test_*.csv")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	
	_, err = tmpFile.WriteString(content)
	if err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()
	
	// Clean up after test
	t.Cleanup(func() {
		os.Remove(tmpFile.Name())
	})
	
	return tmpFile.Name()
}

func TestDefaultParseConfig(t *testing.T) {
	config := DefaultParseConfig()
	
	if !config.HasHeader {
		t.Error("Expected HasHeader to be true")
	}
	
	if config.Delimiter != ',' {
		t.Errorf("Expected delimiter to be ',', got %q", config.Delimiter)
	}
	
	if !config.TrimLeadingSpace {
		t.Error("Expected TrimLeadingSpace to be true")
	}
	
	if !config.SkipEmptyRows {
		t.Error("Expected SkipEmptyRows to be true")
	}
}

func TestParseError(t *testing.T) {
	err := &ParseError{
		Line:    5,
		Column:  3,
		Field:   "amount",
		Value:   "invalid",
		Message: "invalid format",
	}
	
	expected := "parse error at line 5, column 3 (amount='invalid'): invalid format"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestTransactionParserConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *TransactionParserConfig
		wantError bool
	}{
		{
			name:      "Valid config",
			config:    DefaultTransactionParserConfig(),
			wantError: false,
		},
		{
			name: "Empty transaction ID column",
			config: &TransactionParserConfig{
				TrxIDColumn:           "",
				AmountColumn:          "amount",
				TypeColumn:            "type",
				TransactionTimeColumn: "transactionTime",
			},
			wantError: true,
		},
		{
			name: "Empty amount column",
			config: &TransactionParserConfig{
				TrxIDColumn:           "trxID",
				AmountColumn:          "",
				TypeColumn:            "type",
				TransactionTimeColumn: "transactionTime",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestBankConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *BankConfig
		wantError bool
	}{
		{
			name:      "Valid config",
			config:    StandardBankConfig,
			wantError: false,
		},
		{
			name: "Empty name",
			config: &BankConfig{
				Name:             "",
				IdentifierColumn: "id",
				AmountColumn:     "amount",
				DateColumn:       "date",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestNewTransactionParser(t *testing.T) {
	// Test with nil config (should use defaults)
	parser, err := NewTransactionParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser with nil config: %v", err)
	}
	if parser == nil {
		t.Fatal("Expected parser to be created")
	}
	
	// Test with valid config
	config := DefaultTransactionParserConfig()
	parser, err = NewTransactionParser(config)
	if err != nil {
		t.Fatalf("Failed to create parser with valid config: %v", err)
	}
	if parser == nil {
		t.Fatal("Expected parser to be created")
	}
	
	// Test with invalid config
	invalidConfig := &TransactionParserConfig{
		TrxIDColumn: "", // Invalid
	}
	_, err = NewTransactionParser(invalidConfig)
	if err == nil {
		t.Error("Expected error with invalid config")
	}
}

func TestTransactionParser_ParseTransactions(t *testing.T) {
	parser, err := NewTransactionParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	// Create test CSV content
	csvContent := `trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z
TX002,250.00,DEBIT,2024-01-15T14:20:00Z`
	
	filePath := createTempCSVFile(t, csvContent)
	
	transactions, stats, err := parser.ParseTransactions(filePath)
	if err != nil {
		t.Fatalf("Failed to parse transactions: %v", err)
	}
	
	if len(transactions) != 2 {
		t.Errorf("Expected 2 transactions, got %d", len(transactions))
	}
	
	if stats.RecordsValid != 2 {
		t.Errorf("Expected 2 valid records, got %d", stats.RecordsValid)
	}
	
	// Validate first transaction
	tx1 := transactions[0]
	if tx1.TrxID != "TX001" {
		t.Errorf("Expected TrxID 'TX001', got %s", tx1.TrxID)
	}
	
	expectedAmount, _ := models.ParseDecimalFromString("100.50")
	if !tx1.Amount.Equal(expectedAmount) {
		t.Errorf("Expected amount %s, got %s", expectedAmount.String(), tx1.Amount.String())
	}
}

func TestTransactionParser_ParseTransactions_Malformed(t *testing.T) {
	parser, err := NewTransactionParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	// Test with malformed data
	csvContent := `trxID,amount,type,transactionTime
TX001,invalid_amount,CREDIT,2024-01-15T10:30:00Z
TX002,250.00,INVALID_TYPE,2024-01-15T14:20:00Z`
	
	filePath := createTempCSVFile(t, csvContent)
	
	transactions, stats, err := parser.ParseTransactions(filePath)
	if err != nil {
		t.Fatalf("Failed to parse transactions: %v", err)
	}
	
	// Should have 0 valid transactions due to malformed data
	if len(transactions) != 0 {
		t.Errorf("Expected 0 valid transactions, got %d", len(transactions))
	}
	
	if stats.ErrorCount == 0 {
		t.Error("Expected parsing errors for malformed data")
	}
}

func TestNewBankStatementParser(t *testing.T) {
	// Test with nil config (should use standard)
	parser, err := NewBankStatementParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser with nil config: %v", err)
	}
	if parser == nil {
		t.Fatal("Expected parser to be created")
	}
	
	// Test with valid config
	parser, err = NewBankStatementParser(StandardBankConfig)
	if err != nil {
		t.Fatalf("Failed to create parser with valid config: %v", err)
	}
	if parser == nil {
		t.Fatal("Expected parser to be created")
	}
	
	// Test with invalid config
	invalidConfig := &BankConfig{
		Name: "", // Invalid
	}
	_, err = NewBankStatementParser(invalidConfig)
	if err == nil {
		t.Error("Expected error with invalid config")
	}
}

func TestBankStatementParser_ParseBankStatements(t *testing.T) {
	parser, err := NewBankStatementParser(StandardBankConfig)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	// Create test CSV content
	csvContent := `unique_identifier,amount,date
BS001,100.50,2024-01-15
BS002,-250.00,2024-01-15`
	
	filePath := createTempCSVFile(t, csvContent)
	
	statements, stats, err := parser.ParseBankStatements(filePath)
	if err != nil {
		t.Fatalf("Failed to parse bank statements: %v", err)
	}
	
	if len(statements) != 2 {
		t.Errorf("Expected 2 bank statements, got %d", len(statements))
	}
	
	if stats.RecordsValid != 2 {
		t.Errorf("Expected 2 valid records, got %d", stats.RecordsValid)
	}
	
	// Validate first statement
	bs1 := statements[0]
	if bs1.UniqueIdentifier != "BS001" {
		t.Errorf("Expected UniqueIdentifier 'BS001', got %s", bs1.UniqueIdentifier)
	}
	
	expectedAmount, _ := models.ParseDecimalFromString("100.50")
	if !bs1.Amount.Equal(expectedAmount) {
		t.Errorf("Expected amount %s, got %s", expectedAmount.String(), bs1.Amount.String())
	}
}

func TestAutoDetectBankConfig(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected string
	}{
		{
			name:     "Standard format",
			headers:  []string{"unique_identifier", "amount", "date"},
			expected: "Standard",
		},
		{
			name:     "Bank1 format",
			headers:  []string{"transaction_id", "transaction_amount", "posting_date"},
			expected: "Bank1",
		},
		{
			name:     "Unknown format",
			headers:  []string{"id", "value", "timestamp"},
			expected: "Standard", // Should fallback to standard
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := AutoDetectBankConfig(tt.headers)
			if config.Name != tt.expected {
				t.Errorf("Expected config name %s, got %s", tt.expected, config.Name)
			}
		})
	}
}

func TestTransactionParser_ParseTransactionsStream(t *testing.T) {
	parser, err := NewTransactionParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	csvContent := `trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z
TX002,250.00,DEBIT,2024-01-15T14:20:00Z
TX003,75.25,CREDIT,2024-01-16T09:15:00Z`
	
	filePath := createTempCSVFile(t, csvContent)
	
	var processedTransactions []*models.Transaction
	callback := func(transactions []*models.Transaction) error {
		processedTransactions = append(processedTransactions, transactions...)
		return nil
	}
	
	stats, err := parser.ParseTransactionsStream(filePath, 2, callback)
	if err != nil {
		t.Fatalf("Failed to parse transactions stream: %v", err)
	}
	
	if len(processedTransactions) != 3 {
		t.Errorf("Expected 3 transactions, got %d", len(processedTransactions))
	}
	
	if stats.RecordsValid != 3 {
		t.Errorf("Expected 3 valid records, got %d", stats.RecordsValid)
	}
}

func TestBankStatementParser_ParseBankStatementsStream(t *testing.T) {
	parser, err := NewBankStatementParser(StandardBankConfig)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	csvContent := `unique_identifier,amount,date
BS001,100.50,2024-01-15
BS002,-250.00,2024-01-15
BS003,75.25,2024-01-16`
	
	filePath := createTempCSVFile(t, csvContent)
	
	var processedStatements []*models.BankStatement
	callback := func(statements []*models.BankStatement) error {
		processedStatements = append(processedStatements, statements...)
		return nil
	}
	
	stats, err := parser.ParseBankStatementsStream(filePath, 2, callback)
	if err != nil {
		t.Fatalf("Failed to parse bank statements stream: %v", err)
	}
	
	if len(processedStatements) != 3 {
		t.Errorf("Expected 3 statements, got %d", len(processedStatements))
	}
	
	if stats.RecordsValid != 3 {
		t.Errorf("Expected 3 valid records, got %d", stats.RecordsValid)
	}
}

func TestTransactionParser_ValidateTransactionFile(t *testing.T) {
	parser, err := NewTransactionParser(nil)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	// Test with valid file
	validContent := `trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z`
	
	validFile := createTempCSVFile(t, validContent)
	if err := parser.ValidateTransactionFile(validFile); err != nil {
		t.Errorf("Valid file should pass validation: %v", err)
	}
	
	// Test with invalid file
	invalidContent := `wrong,headers,here
TX001,invalid,data,format`
	
	invalidFile := createTempCSVFile(t, invalidContent)
	if err := parser.ValidateTransactionFile(invalidFile); err == nil {
		t.Error("Invalid file should fail validation")
	}
}

func TestBankStatementParser_ValidateBankStatementFile(t *testing.T) {
	parser, err := NewBankStatementParser(StandardBankConfig)
	if err != nil {
		t.Fatalf("Failed to create parser: %v", err)
	}
	
	// Test with valid file
	validContent := `unique_identifier,amount,date
BS001,100.50,2024-01-15`
	
	validFile := createTempCSVFile(t, validContent)
	if err := parser.ValidateBankStatementFile(validFile); err != nil {
		t.Errorf("Valid file should pass validation: %v", err)
	}
	
	// Test with invalid file
	invalidContent := `wrong,headers
BS001,invalid`
	
	invalidFile := createTempCSVFile(t, invalidContent)
	if err := parser.ValidateBankStatementFile(invalidFile); err == nil {
		t.Error("Invalid file should fail validation")
	}
}

func TestParseMultipleBankFiles(t *testing.T) {
	// Create test files for different banks
	standardContent := `unique_identifier,amount,date
BS001,100.50,2024-01-15
BS002,-250.00,2024-01-15`
	
	standardFile := createTempCSVFile(t, standardContent)
	
	files := map[string]string{
		"standard": standardFile,
	}
	
	results, stats, err := ParseMultipleBankFiles(files)
	if err != nil {
		t.Fatalf("Failed to parse multiple files: %v", err)
	}
	
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}
	
	if len(stats) != 1 {
		t.Errorf("Expected 1 stats entry, got %d", len(stats))
	}
	
	standardResults := results["standard"]
	if len(standardResults) != 2 {
		t.Errorf("Expected 2 statements for standard bank, got %d", len(standardResults))
	}
}

func TestStreamingConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *StreamingConfig
		wantError bool
	}{
		{
			name:      "Valid config",
			config:    DefaultStreamingConfig(),
			wantError: false,
		},
		{
			name: "Invalid batch size",
			config: &StreamingConfig{
				BatchSize:      -1,
				MaxConcurrency: 1,
				BufferSize:     1,
				MaxErrors:      1,
				ProgressInterval: 1,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestParseContext_GetColumnIndex(t *testing.T) {
	parseCtx := NewParseContext(context.Background())
	parseCtx.Headers = []string{"trxID", "amount", "type"}
	parseCtx.HeaderMap = map[string]int{
		"trxID":  0,
		"amount": 1,
		"type":   2,
	}
	
	// Test exact match
	index := parseCtx.GetColumnIndex("amount")
	if index != 1 {
		t.Errorf("Expected index 1 for 'amount', got %d", index)
	}
	
	// Test case insensitive match
	index = parseCtx.GetColumnIndex("AMOUNT")
	if index != 1 {
		t.Errorf("Expected index 1 for 'AMOUNT' (case insensitive), got %d", index)
	}
	
	// Test not found
	index = parseCtx.GetColumnIndex("nonexistent")
	if index != -1 {
		t.Errorf("Expected -1 for nonexistent column, got %d", index)
	}
}

// Benchmark tests
func BenchmarkTransactionParser_ParseTransactions(b *testing.B) {
	parser, err := NewTransactionParser(nil)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}
	
	// Create larger test dataset
	var csvLines []string
	csvLines = append(csvLines, "trxID,amount,type,transactionTime")
	for i := 0; i < 1000; i++ {
		csvLines = append(csvLines, 
			fmt.Sprintf("TX%03d,100.50,CREDIT,2024-01-15T10:30:00Z", i))
	}
	csvContent := strings.Join(csvLines, "\n")
	
	tmpFile, err := os.CreateTemp("", "benchmark_*.csv")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	tmpFile.WriteString(csvContent)
	tmpFile.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := parser.ParseTransactions(tmpFile.Name())
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkBankStatementParser_ParseBankStatements(b *testing.B) {
	parser, err := NewBankStatementParser(StandardBankConfig)
	if err != nil {
		b.Fatalf("Failed to create parser: %v", err)
	}
	
	// Create larger test dataset
	var csvLines []string
	csvLines = append(csvLines, "unique_identifier,amount,date")
	for i := 0; i < 1000; i++ {
		csvLines = append(csvLines, 
			fmt.Sprintf("BS%03d,100.50,2024-01-15", i))
	}
	csvContent := strings.Join(csvLines, "\n")
	
	tmpFile, err := os.CreateTemp("", "benchmark_*.csv")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	
	tmpFile.WriteString(csvContent)
	tmpFile.Close()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := parser.ParseBankStatements(tmpFile.Name())
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}