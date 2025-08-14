package config

import (
	"fmt"
	"path/filepath"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/parsers"
	"golang-reconciliation-service/internal/reconciler"
	"golang-reconciliation-service/internal/reporter"
)

// CreateTransactionParserConfig creates a default transaction parser configuration
func CreateTransactionParserConfig() (*parsers.TransactionParserConfig, error) {
	return &parsers.TransactionParserConfig{
		TrxIDColumn:           "trxID",
		AmountColumn:          "amount",
		TypeColumn:            "type",
		TransactionTimeColumn: "transactionTime",
		HasHeader:             true,
		Delimiter:             ',',
		ColumnAliases:         map[string]string{
			// Common aliases for transaction columns
			"id":     "trxID",
			"tx_id":  "trxID",
			"txn_id": "trxID",
			"transaction_id": "trxID",
			"amt":    "amount",
			"value":  "amount",
			"sum":    "amount",
			"transaction_type": "type",
			"tx_type": "type",
			"debit_credit": "type",
			"time":   "transactionTime",
			"datetime": "transactionTime",
			"timestamp": "transactionTime",
			"date":   "transactionTime",
		},
	}, nil
}

// CreateBankConfigs creates bank configurations for the provided file paths
func CreateBankConfigs(bankFiles []string) (map[string]*parsers.BankConfig, error) {
	bankConfigs := make(map[string]*parsers.BankConfig)
	
	for i, bankFile := range bankFiles {
		bankName := fmt.Sprintf("Bank_%d", i+1)
		if len(bankFiles) == 1 {
			bankName = "Bank"
		} else {
			// Try to derive name from filename
			base := filepath.Base(bankFile)
			ext := filepath.Ext(base)
			if ext != "" {
				base = base[:len(base)-len(ext)]
			}
			bankName = fmt.Sprintf("Bank_%s", base)
		}
		
		bankConfig := &parsers.BankConfig{
			Name:             bankName,
			IdentifierColumn: "unique_identifier",
			AmountColumn:     "amount",
			DateColumn:       "date",
			DateFormat:       "2006-01-02",
			HasHeader:        true,
			Delimiter:        ',',
			ColumnAliases: map[string]string{
				// Common aliases for bank statement columns
				"id":           "unique_identifier",
				"identifier":   "unique_identifier",
				"ref":          "unique_identifier",
				"reference":    "unique_identifier",
				"transaction_id": "unique_identifier",
				"statement_id": "unique_identifier",
				"amt":          "amount",
				"value":        "amount",
				"sum":          "amount",
				"balance":      "amount",
				"transaction_date": "date",
				"statement_date": "date",
				"posting_date": "date",
				"value_date":   "date",
			},
			Description: fmt.Sprintf("Configuration for %s", bankFile),
		}
		
		bankConfigs[bankFile] = bankConfig
	}
	
	return bankConfigs, nil
}

// CreateMatchingConfig creates a matching configuration with the specified tolerances
func CreateMatchingConfig(dateTolerance int, amountTolerance float64) *matcher.MatchingConfig {
	config := matcher.DefaultMatchingConfig()
	
	// Apply CLI overrides
	config.DateToleranceDays = dateTolerance
	config.AmountTolerancePercent = amountTolerance
	
	// Set sensible defaults for CLI usage
	config.EnableFuzzyMatching = true
	config.MinConfidenceScore = 0.7
	config.TimezoneHandling = matcher.TimezoneIgnore
	
	return config
}

// CreateReconcilerConfig creates a reconciler configuration
func CreateReconcilerConfig(showProgress bool) *reconciler.Config {
	config := reconciler.DefaultConfig()
	
	// Apply CLI overrides
	config.ProgressReporting = showProgress
	config.IncludeStatistics = true
	config.DetailedBreakdown = true
	
	return config
}

// CreateReportConfig creates a report configuration for the specified output format
func CreateReportConfig(format string) *reporter.ReportConfig {
	config := reporter.DefaultReportConfig()
	
	// Set output format
	switch format {
	case "console":
		config.Format = reporter.FormatConsole
		config.UseColors = true
		config.IncludeUnmatchedTransactions = true
		config.IncludeUnmatchedStatements = true
		config.IncludeDiscrepancies = true
		config.IncludeProcessingStats = true
	case "json":
		config.Format = reporter.FormatJSON
		config.IncludeUnmatchedTransactions = true
		config.IncludeUnmatchedStatements = true
		config.IncludeDiscrepancies = true
		config.IncludeProcessingStats = true
		config.IncludeMatchedTransactions = false // Keep JSON output focused
	case "csv":
		config.Format = reporter.FormatCSV
		config.CSVHeaders = true
		config.CSVDelimiter = ','
		config.IncludeUnmatchedTransactions = true
		config.IncludeUnmatchedStatements = true
		config.IncludeMatchedTransactions = true
		config.IncludeDiscrepancies = false // CSV is for transaction data
		config.IncludeProcessingStats = false
	}
	
	return config
}

// BankProfile represents a pre-configured bank profile
type BankProfile struct {
	Name   string
	Config *parsers.BankConfig
}

// GetCommonBankProfiles returns configurations for common bank CSV formats
func GetCommonBankProfiles() []BankProfile {
	return []BankProfile{
		{
			Name: "Standard",
			Config: &parsers.BankConfig{
				Name:             "Standard Bank Format",
				IdentifierColumn: "unique_identifier",
				AmountColumn:     "amount",
				DateColumn:       "date",
				DateFormat:       "2006-01-02",
				HasHeader:        true,
				Delimiter:        ',',
				Description:      "Standard CSV format with identifier, amount, and date columns",
			},
		},
		{
			Name: "Chase",
			Config: &parsers.BankConfig{
				Name:             "Chase Bank",
				IdentifierColumn: "transaction_id",
				AmountColumn:     "amount",
				DateColumn:       "posting_date",
				DateFormat:       "01/02/2006",
				HasHeader:        true,
				Delimiter:        ',',
				Description:      "Chase Bank statement format",
			},
		},
		{
			Name: "Wells Fargo",
			Config: &parsers.BankConfig{
				Name:             "Wells Fargo",
				IdentifierColumn: "reference_number",
				AmountColumn:     "amount",
				DateColumn:       "date",
				DateFormat:       "01/02/2006",
				HasHeader:        true,
				Delimiter:        ',',
				Description:      "Wells Fargo statement format",
			},
		},
		{
			Name: "Bank of America",
			Config: &parsers.BankConfig{
				Name:             "Bank of America",
				IdentifierColumn: "reference_id",
				AmountColumn:     "amount",
				DateColumn:       "posted_date",
				DateFormat:       "01/02/2006",
				HasHeader:        true,
				Delimiter:        ',',
				Description:      "Bank of America statement format",
			},
		},
	}
}

// GetBankProfile returns a bank configuration by profile name
func GetBankProfile(profileName string) (*parsers.BankConfig, error) {
	profiles := GetCommonBankProfiles()
	
	for _, profile := range profiles {
		if profile.Name == profileName {
			return profile.Config, nil
		}
	}
	
	return nil, fmt.Errorf("unknown bank profile: %s", profileName)
}

// ValidateConfig validates that all required configurations are valid
func ValidateConfig(transactionConfig *parsers.TransactionParserConfig, bankConfigs map[string]*parsers.BankConfig, matchingConfig *matcher.MatchingConfig) error {
	// Validate transaction config
	if err := transactionConfig.Validate(); err != nil {
		return fmt.Errorf("invalid transaction config: %w", err)
	}
	
	// Validate bank configs
	for file, bankConfig := range bankConfigs {
		if err := bankConfig.Validate(); err != nil {
			return fmt.Errorf("invalid bank config for file %s: %w", file, err)
		}
	}
	
	// Validate matching config
	if err := matchingConfig.Validate(); err != nil {
		return fmt.Errorf("invalid matching config: %w", err)
	}
	
	return nil
}