package config

import (
	"testing"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/parsers"
	"golang-reconciliation-service/internal/reporter"
)

func TestCreateTransactionParserConfig(t *testing.T) {
	config, err := CreateTransactionParserConfig()
	if err != nil {
		t.Fatalf("failed to create transaction parser config: %v", err)
	}

	if config.TrxIDColumn != "trxID" {
		t.Errorf("expected TrxIDColumn 'trxID', got '%s'", config.TrxIDColumn)
	}
	if config.AmountColumn != "amount" {
		t.Errorf("expected AmountColumn 'amount', got '%s'", config.AmountColumn)
	}
	if config.TypeColumn != "type" {
		t.Errorf("expected TypeColumn 'type', got '%s'", config.TypeColumn)
	}
	if config.TransactionTimeColumn != "transactionTime" {
		t.Errorf("expected TransactionTimeColumn 'transactionTime', got '%s'", config.TransactionTimeColumn)
	}
	if !config.HasHeader {
		t.Error("expected HasHeader to be true")
	}
	if config.Delimiter != ',' {
		t.Errorf("expected Delimiter ',', got '%c'", config.Delimiter)
	}

	// Test aliases
	if len(config.ColumnAliases) == 0 {
		t.Error("expected column aliases to be set")
	}
	if config.ColumnAliases["id"] != "trxID" {
		t.Error("expected 'id' alias to map to 'trxID'")
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		t.Errorf("transaction parser config should be valid: %v", err)
	}
}

func TestCreateBankConfigs(t *testing.T) {
	tests := []struct {
		name      string
		bankFiles []string
		expected  int
	}{
		{
			name:      "single bank file",
			bankFiles: []string{"/path/to/statements.csv"},
			expected:  1,
		},
		{
			name:      "multiple bank files",
			bankFiles: []string{"/path/to/bank1.csv", "/path/to/bank2.csv", "/path/to/bank3.csv"},
			expected:  3,
		},
		{
			name:      "empty list",
			bankFiles: []string{},
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configs, err := CreateBankConfigs(tt.bankFiles)
			if err != nil {
				t.Fatalf("failed to create bank configs: %v", err)
			}

			if len(configs) != tt.expected {
				t.Errorf("expected %d configs, got %d", tt.expected, len(configs))
			}

			// Validate all configurations
			for file, config := range configs {
				if config.IdentifierColumn != "unique_identifier" {
					t.Errorf("expected IdentifierColumn 'unique_identifier', got '%s'", config.IdentifierColumn)
				}
				if config.AmountColumn != "amount" {
					t.Errorf("expected AmountColumn 'amount', got '%s'", config.AmountColumn)
				}
				if config.DateColumn != "date" {
					t.Errorf("expected DateColumn 'date', got '%s'", config.DateColumn)
				}
				if !config.HasHeader {
					t.Error("expected HasHeader to be true")
				}
				if config.Delimiter != ',' {
					t.Errorf("expected Delimiter ',', got '%c'", config.Delimiter)
				}

				// Validate the configuration
				if err := config.Validate(); err != nil {
					t.Errorf("bank config for file %s should be valid: %v", file, err)
				}
			}
		})
	}
}

func TestCreateMatchingConfig(t *testing.T) {
	tests := []struct {
		name            string
		dateTolerance   int
		amountTolerance float64
	}{
		{"default tolerances", 1, 0.0},
		{"custom tolerances", 3, 2.5},
		{"zero tolerances", 0, 0.0},
		{"high tolerances", 7, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateMatchingConfig(tt.dateTolerance, tt.amountTolerance)

			if config.DateToleranceDays != tt.dateTolerance {
				t.Errorf("expected DateToleranceDays %d, got %d", tt.dateTolerance, config.DateToleranceDays)
			}
			if config.AmountTolerancePercent != tt.amountTolerance {
				t.Errorf("expected AmountTolerancePercent %f, got %f", tt.amountTolerance, config.AmountTolerancePercent)
			}

			// Test default settings
			if !config.EnableFuzzyMatching {
				t.Error("expected EnableFuzzyMatching to be true")
			}
			if config.MinConfidenceScore != 0.7 {
				t.Errorf("expected MinConfidenceScore 0.7, got %f", config.MinConfidenceScore)
			}

			// Validate the configuration
			if err := config.Validate(); err != nil {
				t.Errorf("matching config should be valid: %v", err)
			}
		})
	}
}

func TestCreateReconcilerConfig(t *testing.T) {
	tests := []struct {
		name         string
		showProgress bool
	}{
		{"with progress", true},
		{"without progress", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateReconcilerConfig(tt.showProgress)

			if config.ProgressReporting != tt.showProgress {
				t.Errorf("expected ProgressReporting %v, got %v", tt.showProgress, config.ProgressReporting)
			}

			// Test default settings
			if !config.IncludeStatistics {
				t.Error("expected IncludeStatistics to be true")
			}
			if !config.DetailedBreakdown {
				t.Error("expected DetailedBreakdown to be true")
			}

			// Validate the configuration
			if err := config.Validate(); err != nil {
				t.Errorf("reconciler config should be valid: %v", err)
			}
		})
	}
}

func TestCreateReportConfig(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		expectedType reporter.OutputFormat
	}{
		{"console format", "console", reporter.FormatConsole},
		{"json format", "json", reporter.FormatJSON},
		{"csv format", "csv", reporter.FormatCSV},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CreateReportConfig(tt.format)

			if config.Format != tt.expectedType {
				t.Errorf("expected Format %s, got %s", tt.expectedType, config.Format)
			}

			// Test format-specific settings
			switch tt.format {
			case "console":
				if !config.UseColors {
					t.Error("console format should use colors")
				}
			case "json":
				if config.IncludeMatchedTransactions {
					t.Error("JSON format should not include matched transactions by default")
				}
			case "csv":
				if !config.CSVHeaders {
					t.Error("CSV format should include headers")
				}
				if config.CSVDelimiter != ',' {
					t.Error("CSV format should use comma delimiter")
				}
				if config.IncludeDiscrepancies {
					t.Error("CSV format should not include discrepancies")
				}
			}

			// Validate the configuration
			if err := config.Validate(); err != nil {
				t.Errorf("report config should be valid: %v", err)
			}
		})
	}
}

func TestGetCommonBankProfiles(t *testing.T) {
	profiles := GetCommonBankProfiles()

	if len(profiles) == 0 {
		t.Fatal("expected at least one bank profile")
	}

	expectedProfiles := []string{"Standard", "Chase", "Wells Fargo", "Bank of America"}
	for _, expected := range expectedProfiles {
		found := false
		for _, profile := range profiles {
			if profile.Name == expected {
				found = true
				// Validate the profile configuration
				if err := profile.Config.Validate(); err != nil {
					t.Errorf("profile %s should have valid config: %v", expected, err)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected to find profile '%s'", expected)
		}
	}
}

func TestGetBankProfile(t *testing.T) {
	tests := []struct {
		name        string
		profileName string
		expectError bool
	}{
		{"valid profile", "Standard", false},
		{"another valid profile", "Chase", false},
		{"invalid profile", "NonExistent", true},
		{"empty name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetBankProfile(tt.profileName)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for profile '%s'", tt.profileName)
				}
				if config != nil {
					t.Error("expected nil config on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if config == nil {
					t.Error("expected valid config")
				}
				// Validate the configuration
				if config != nil {
					if err := config.Validate(); err != nil {
						t.Errorf("profile config should be valid: %v", err)
					}
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	// Create valid configurations
	transactionConfig, _ := CreateTransactionParserConfig()
	bankConfigs, _ := CreateBankConfigs([]string{"test.csv"})
	matchingConfig := CreateMatchingConfig(1, 0.0)

	tests := []struct {
		name               string
		transactionConfig  func() *parsers.TransactionParserConfig
		bankConfigs        func() map[string]*parsers.BankConfig
		matchingConfig     func() *matcher.MatchingConfig
		expectError        bool
	}{
		{
			name: "all valid",
			transactionConfig: func() *parsers.TransactionParserConfig { return transactionConfig },
			bankConfigs: func() map[string]*parsers.BankConfig { return bankConfigs },
			matchingConfig: func() *matcher.MatchingConfig { return matchingConfig },
			expectError: false,
		},
		{
			name: "invalid transaction config",
			transactionConfig: func() *parsers.TransactionParserConfig {
				invalid := *transactionConfig
				invalid.TrxIDColumn = "" // Invalid
				return &invalid
			},
			bankConfigs: func() map[string]*parsers.BankConfig { return bankConfigs },
			matchingConfig: func() *matcher.MatchingConfig { return matchingConfig },
			expectError: true,
		},
		{
			name: "invalid bank config",
			transactionConfig: func() *parsers.TransactionParserConfig { return transactionConfig },
			bankConfigs: func() map[string]*parsers.BankConfig {
				invalid := make(map[string]*parsers.BankConfig)
				config := *bankConfigs["test.csv"]
				config.IdentifierColumn = "" // Invalid
				invalid["test.csv"] = &config
				return invalid
			},
			matchingConfig: func() *matcher.MatchingConfig { return matchingConfig },
			expectError: true,
		},
		{
			name: "invalid matching config",
			transactionConfig: func() *parsers.TransactionParserConfig { return transactionConfig },
			bankConfigs: func() map[string]*parsers.BankConfig { return bankConfigs },
			matchingConfig: func() *matcher.MatchingConfig {
				invalid := *matchingConfig
				invalid.DateToleranceDays = -1 // Invalid
				return &invalid
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(
				tt.transactionConfig(),
				tt.bankConfigs(),
				tt.matchingConfig(),
			)

			if tt.expectError && err == nil {
				t.Error("expected validation error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}