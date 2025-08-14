package reporter

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/reconciler"

	"github.com/shopspring/decimal"
)

func TestNewReportGenerator(t *testing.T) {
	tests := []struct {
		name        string
		config      *ReportConfig
		expectError bool
	}{
		{
			name:        "default config",
			config:      nil,
			expectError: false,
		},
		{
			name:        "valid config",
			config:      DefaultReportConfig(),
			expectError: false,
		},
		{
			name: "invalid format",
			config: &ReportConfig{
				Format:        "invalid",
				TableMaxWidth: 120,
			},
			expectError: true,
		},
		{
			name: "table width too small",
			config: &ReportConfig{
				Format:        FormatConsole,
				TableMaxWidth: 30,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewReportGenerator(tt.config)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if generator == nil {
					t.Errorf("expected generator but got nil")
				}
			}
		})
	}
}

func TestOutputFormatValidation(t *testing.T) {
	tests := []struct {
		format OutputFormat
		valid  bool
	}{
		{FormatConsole, true},
		{FormatJSON, true},
		{FormatCSV, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			if tt.format.IsValid() != tt.valid {
				t.Errorf("expected IsValid() = %v for format %s", tt.valid, tt.format)
			}
		})
	}
}

func TestReportConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *ReportConfig
		expectError bool
	}{
		{
			name:        "valid config",
			config:      DefaultReportConfig(),
			expectError: false,
		},
		{
			name: "invalid format",
			config: &ReportConfig{
				Format:        "invalid",
				TableMaxWidth: 120,
			},
			expectError: true,
		},
		{
			name: "table width too small",
			config: &ReportConfig{
				Format:        FormatConsole,
				TableMaxWidth: 30,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.expectError && err == nil {
				t.Errorf("expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestGenerateReport(t *testing.T) {
	// Create sample reconciliation result
	result := createSampleReconciliationResult()

	tests := []struct {
		name        string
		config      *ReportConfig
		result      *reconciler.ReconciliationResult
		expectError bool
		checkOutput func(t *testing.T, output string)
	}{
		{
			name: "console format",
			config: &ReportConfig{
				Format:                       FormatConsole,
				IncludeUnmatchedTransactions: true,
				IncludeUnmatchedStatements:   true,
				IncludeDiscrepancies:         true,
				IncludeProcessingStats:       true,
				TableMaxWidth:                120,
			},
			result:      result,
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				if !strings.Contains(output, "RECONCILIATION REPORT") {
					t.Errorf("console output should contain report header")
				}
				if !strings.Contains(output, "SUMMARY") {
					t.Errorf("console output should contain summary section")
				}
				if !strings.Contains(output, "FINANCIAL SUMMARY") {
					t.Errorf("console output should contain financial summary")
				}
				if !strings.Contains(output, "UNMATCHED TRANSACTIONS") {
					t.Errorf("console output should contain unmatched transactions section")
				}
				if !strings.Contains(output, "UNMATCHED BANK STATEMENTS") {
					t.Errorf("console output should contain unmatched statements section")
				}
			},
		},
		{
			name: "JSON format",
			config: &ReportConfig{
				Format:                       FormatJSON,
				IncludeUnmatchedTransactions: true,
				IncludeUnmatchedStatements:   true,
				IncludeProcessingStats:       true,
				TableMaxWidth:                120,
			},
			result:      result,
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(output), &jsonData); err != nil {
					t.Errorf("output should be valid JSON: %v", err)
				}
				
				if _, exists := jsonData["summary"]; !exists {
					t.Errorf("JSON output should contain summary")
				}
				if _, exists := jsonData["unmatched_transactions"]; !exists {
					t.Errorf("JSON output should contain unmatched_transactions")
				}
				if _, exists := jsonData["unmatched_statements"]; !exists {
					t.Errorf("JSON output should contain unmatched_statements")
				}
			},
		},
		{
			name: "CSV format",
			config: &ReportConfig{
				Format:                       FormatCSV,
				IncludeUnmatchedTransactions: true,
				IncludeUnmatchedStatements:   true,
				CSVHeaders:                   true,
				CSVDelimiter:                 ',',
				TableMaxWidth:                120,
			},
			result:      result,
			expectError: false,
			checkOutput: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				if len(lines) < 2 {
					t.Errorf("CSV output should have at least header and one data row")
				}
				// Check headers
				if !strings.Contains(lines[0], "Type,ID,Amount") {
					t.Errorf("CSV should contain expected headers")
				}
				// Check for data rows
				dataRows := 0
				for _, line := range lines[1:] {
					if strings.TrimSpace(line) != "" {
						dataRows++
					}
				}
				if dataRows == 0 {
					t.Errorf("CSV should contain data rows")
				}
			},
		},
		{
			name:        "nil result",
			config:      DefaultReportConfig(),
			result:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewReportGenerator(tt.config)
			if err != nil {
				t.Fatalf("failed to create report generator: %v", err)
			}

			var buffer bytes.Buffer
			err = generator.GenerateReport(tt.result, &buffer)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				
				output := buffer.String()
				if tt.checkOutput != nil {
					tt.checkOutput(t, output)
				}
			}
		})
	}
}

func TestCalculatePercentage(t *testing.T) {
	generator, _ := NewReportGenerator(DefaultReportConfig())

	tests := []struct {
		name     string
		part     int
		total    int
		expected float64
	}{
		{"normal case", 25, 100, 25.0},
		{"zero total", 10, 0, 0.0},
		{"zero part", 0, 100, 0.0},
		{"equal parts", 50, 50, 100.0},
		{"fractional result", 1, 3, float64(1)/float64(3)*100.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generator.calculatePercentage(tt.part, tt.total)
			if result != tt.expected {
				t.Errorf("calculatePercentage(%d, %d) = %f, expected %f",
					tt.part, tt.total, result, tt.expected)
			}
		})
	}
}

func TestFilterResultForOutput(t *testing.T) {
	generator, _ := NewReportGenerator(&ReportConfig{
		Format:                       FormatJSON,
		IncludeUnmatchedTransactions: true,
		IncludeProcessingStats:       false,
		TableMaxWidth:                120,
	})

	result := createSampleReconciliationResult()
	filtered := generator.filterResultForOutput(result)

	// Check that summary and processed_at are always included
	if _, exists := filtered["summary"]; !exists {
		t.Errorf("filtered result should always include summary")
	}
	if _, exists := filtered["processed_at"]; !exists {
		t.Errorf("filtered result should always include processed_at")
	}

	// Check that unmatched transactions are included based on config
	if _, exists := filtered["unmatched_transactions"]; !exists {
		t.Errorf("filtered result should include unmatched_transactions when configured")
	}

	// Check that processing stats are excluded based on config
	if _, exists := filtered["processing_stats"]; exists {
		t.Errorf("filtered result should not include processing_stats when not configured")
	}
}

func TestUpdateConfiguration(t *testing.T) {
	generator, _ := NewReportGenerator(DefaultReportConfig())

	// Test valid configuration update
	newConfig := &ReportConfig{
		Format:        FormatJSON,
		TableMaxWidth: 80,
	}

	err := generator.UpdateConfiguration(newConfig)
	if err != nil {
		t.Errorf("unexpected error updating configuration: %v", err)
	}

	if !reflect.DeepEqual(generator.GetConfiguration(), newConfig) {
		t.Errorf("configuration was not updated correctly")
	}

	// Test invalid configuration update
	invalidConfig := &ReportConfig{
		Format:        "invalid",
		TableMaxWidth: 80,
	}

	err = generator.UpdateConfiguration(invalidConfig)
	if err == nil {
		t.Errorf("expected error for invalid configuration but got none")
	}
}

func TestConsoleOutputSections(t *testing.T) {
	result := createSampleReconciliationResult()

	tests := []struct {
		name         string
		config       *ReportConfig
		shouldContain []string
		shouldNotContain []string
	}{
		{
			name: "all sections enabled",
			config: &ReportConfig{
				Format:                       FormatConsole,
				IncludeUnmatchedTransactions: true,
				IncludeUnmatchedStatements:   true,
				IncludeDiscrepancies:         true,
				IncludeProcessingStats:       true,
				TableMaxWidth:                120,
			},
			shouldContain: []string{
				"=== SUMMARY ===",
				"=== FINANCIAL SUMMARY ===",
				"=== MATCH QUALITY BREAKDOWN ===",
				"=== UNMATCHED TRANSACTIONS ===",
				"=== UNMATCHED BANK STATEMENTS ===",
				"=== DISCREPANCIES ===",
				"=== PROCESSING STATISTICS ===",
			},
		},
		{
			name: "minimal sections",
			config: &ReportConfig{
				Format:                       FormatConsole,
				IncludeUnmatchedTransactions: false,
				IncludeUnmatchedStatements:   false,
				IncludeDiscrepancies:         false,
				IncludeProcessingStats:       false,
				TableMaxWidth:                120,
			},
			shouldContain: []string{
				"=== SUMMARY ===",
				"=== FINANCIAL SUMMARY ===",
				"=== MATCH QUALITY BREAKDOWN ===",
			},
			shouldNotContain: []string{
				"=== UNMATCHED TRANSACTIONS ===",
				"=== UNMATCHED BANK STATEMENTS ===",
				"=== DISCREPANCIES ===",
				"=== PROCESSING STATISTICS ===",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewReportGenerator(tt.config)
			if err != nil {
				t.Fatalf("failed to create report generator: %v", err)
			}

			var buffer bytes.Buffer
			err = generator.GenerateReport(result, &buffer)
			if err != nil {
				t.Fatalf("failed to generate report: %v", err)
			}

			output := buffer.String()

			for _, section := range tt.shouldContain {
				if !strings.Contains(output, section) {
					t.Errorf("output should contain section: %s", section)
				}
			}

			for _, section := range tt.shouldNotContain {
				if strings.Contains(output, section) {
					t.Errorf("output should not contain section: %s", section)
				}
			}
		})
	}
}

func TestCSVFormatting(t *testing.T) {
	result := createSampleReconciliationResult()

	tests := []struct {
		name      string
		config    *ReportConfig
		checkFunc func(t *testing.T, output string)
	}{
		{
			name: "with headers",
			config: &ReportConfig{
				Format:                       FormatCSV,
				IncludeUnmatchedTransactions: true,
				CSVHeaders:                   true,
				CSVDelimiter:                 ',',
				TableMaxWidth:                120,
			},
			checkFunc: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				if len(lines) < 1 || !strings.Contains(lines[0], "Type") {
					t.Errorf("CSV should start with headers when enabled")
				}
			},
		},
		{
			name: "without headers",
			config: &ReportConfig{
				Format:                       FormatCSV,
				IncludeUnmatchedTransactions: true,
				CSVHeaders:                   false,
				CSVDelimiter:                 ',',
				TableMaxWidth:                120,
			},
			checkFunc: func(t *testing.T, output string) {
				lines := strings.Split(output, "\n")
				if len(lines) >= 1 && strings.Contains(lines[0], "Type") {
					t.Errorf("CSV should not start with headers when disabled")
				}
			},
		},
		{
			name: "custom delimiter",
			config: &ReportConfig{
				Format:                       FormatCSV,
				IncludeUnmatchedTransactions: true,
				CSVHeaders:                   true,
				CSVDelimiter:                 ';',
				TableMaxWidth:                120,
			},
			checkFunc: func(t *testing.T, output string) {
				if !strings.Contains(output, ";") {
					t.Errorf("CSV should use custom delimiter")
				}
				if strings.Count(output, ";") < strings.Count(output, ",") {
					t.Errorf("CSV should primarily use semicolon delimiter")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			generator, err := NewReportGenerator(tt.config)
			if err != nil {
				t.Fatalf("failed to create report generator: %v", err)
			}

			var buffer bytes.Buffer
			err = generator.GenerateReport(result, &buffer)
			if err != nil {
				t.Fatalf("failed to generate report: %v", err)
			}

			tt.checkFunc(t, buffer.String())
		})
	}
}

// Helper function to create sample reconciliation result for testing
func createSampleReconciliationResult() *reconciler.ReconciliationResult {
	now := time.Now()

	// Create sample transactions
	tx1 := &models.Transaction{
		TrxID:           "TXN001",
		Amount:          decimal.NewFromFloat(100.50),
		Type:            models.TransactionTypeDebit,
		TransactionTime: now.Add(-24 * time.Hour),
	}

	tx2 := &models.Transaction{
		TrxID:           "TXN002", 
		Amount:          decimal.NewFromFloat(250.00),
		Type:            models.TransactionTypeCredit,
		TransactionTime: now.Add(-12 * time.Hour),
	}

	// Create sample bank statements
	stmt1 := &models.BankStatement{
		UniqueIdentifier: "STMT001",
		Amount:           decimal.NewFromFloat(-75.25),
		Date:             now.Add(-18 * time.Hour),
	}

	stmt2 := &models.BankStatement{
		UniqueIdentifier: "STMT002",
		Amount:           decimal.NewFromFloat(150.00),
		Date:             now.Add(-6 * time.Hour),
	}

	// Create sample matched transactions
	matchedTx := &matcher.MatchResult{
		Transaction:      tx1,
		BankStatement:    stmt1,
		MatchType:        matcher.MatchExact,
		ConfidenceScore:  0.95,
		AmountDifference: decimal.NewFromFloat(25.25),
		DateDifference:   6 * time.Hour,
		Reasons:          []string{"Amount match", "Date proximity"},
	}

	// Create sample discrepancies
	discrepancy := &reconciler.Discrepancy{
		Type:        reconciler.DiscrepancyAmountDifference,
		Transaction: tx2,
		Statement:   stmt2,
		Description: "Amount discrepancy between transaction and statement",
		Amount:      decimal.NewFromFloat(100.00),
		Severity:    reconciler.SeverityMedium,
	}

	// Create processing stats
	processingStats := &reconciler.ProcessingStats{
		FilesProcessed:      2,
		ParseErrors:        0,
		ValidationErrors:   0,
		RecordsPerSecond:   1500.0,
		TotalProcessingTime: 2 * time.Second,
		ParsingTime:        500 * time.Millisecond,
		MatchingTime:       1200 * time.Millisecond,
		PeakMemoryUsage:    50 * 1024 * 1024, // 50 MB
	}

	// Create summary
	summary := &reconciler.ResultSummary{
		TotalTransactions:      2,
		MatchedTransactions:    1,
		UnmatchedTransactions:  1,
		TotalBankStatements:    2,
		MatchedStatements:      1,
		UnmatchedStatements:    1,
		ExactMatches:          1,
		CloseMatches:          0,
		FuzzyMatches:          0,
		PossibleMatches:       0,
		TotalTransactionAmount: decimal.NewFromFloat(350.50),
		TotalStatementAmount:   decimal.NewFromFloat(74.75),
		NetDiscrepancy:        decimal.NewFromFloat(275.75),
		ProcessingDuration:    2 * time.Second,
	}

	return &reconciler.ReconciliationResult{
		Summary:               summary,
		MatchedTransactions:   []*matcher.MatchResult{matchedTx},
		UnmatchedTransactions: []*models.Transaction{tx2},
		UnmatchedStatements:   []*models.BankStatement{stmt2},
		ProcessingStats:       processingStats,
		Discrepancies:         []*reconciler.Discrepancy{discrepancy},
		ProcessedAt:           now,
	}
}

func TestEmptyResultHandling(t *testing.T) {
	// Test with empty reconciliation result
	emptyResult := &reconciler.ReconciliationResult{
		Summary: &reconciler.ResultSummary{
			TotalTransactions:      0,
			MatchedTransactions:    0,
			UnmatchedTransactions:  0,
			TotalBankStatements:    0,
			MatchedStatements:      0,
			UnmatchedStatements:    0,
			ExactMatches:          0,
			CloseMatches:          0,
			FuzzyMatches:          0,
			PossibleMatches:       0,
			TotalTransactionAmount: decimal.Zero,
			TotalStatementAmount:   decimal.Zero,
			NetDiscrepancy:        decimal.Zero,
			ProcessingDuration:    0,
		},
		MatchedTransactions:   []*matcher.MatchResult{},
		UnmatchedTransactions: []*models.Transaction{},
		UnmatchedStatements:   []*models.BankStatement{},
		ProcessingStats:       &reconciler.ProcessingStats{},
		Discrepancies:         []*reconciler.Discrepancy{},
		ProcessedAt:           time.Now(),
	}

	tests := []OutputFormat{FormatConsole, FormatJSON, FormatCSV}

	for _, format := range tests {
		t.Run(string(format), func(t *testing.T) {
			config := DefaultReportConfig()
			config.Format = format

			generator, err := NewReportGenerator(config)
			if err != nil {
				t.Fatalf("failed to create report generator: %v", err)
			}

			var buffer bytes.Buffer
			err = generator.GenerateReport(emptyResult, &buffer)
			if err != nil {
				t.Errorf("should handle empty result without error: %v", err)
			}

			output := buffer.String()
			if len(output) == 0 {
				t.Errorf("should produce some output even for empty results")
			}
		})
	}
}

func BenchmarkGenerateConsoleReport(b *testing.B) {
	result := createSampleReconciliationResult()
	generator, _ := NewReportGenerator(DefaultReportConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buffer bytes.Buffer
		_ = generator.GenerateReport(result, &buffer)
	}
}

func BenchmarkGenerateJSONReport(b *testing.B) {
	result := createSampleReconciliationResult()
	config := DefaultReportConfig()
	config.Format = FormatJSON
	generator, _ := NewReportGenerator(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buffer bytes.Buffer
		_ = generator.GenerateReport(result, &buffer)
	}
}

func BenchmarkGenerateCSVReport(b *testing.B) {
	result := createSampleReconciliationResult()
	config := DefaultReportConfig()
	config.Format = FormatCSV
	generator, _ := NewReportGenerator(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buffer bytes.Buffer
		_ = generator.GenerateReport(result, &buffer)
	}
}