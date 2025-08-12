package parsers

import (
	"fmt"
	"strings"
)

// BankConfig represents configuration for parsing bank-specific CSV formats
type BankConfig struct {
	Name             string            `json:"name"`
	IdentifierColumn string            `json:"identifier_column"`
	AmountColumn     string            `json:"amount_column"`
	DateColumn       string            `json:"date_column"`
	DateFormat       string            `json:"date_format"`
	HasHeader        bool              `json:"has_header"`
	Delimiter        rune              `json:"delimiter"`
	ColumnAliases    map[string]string `json:"column_aliases,omitempty"`
	Description      string            `json:"description,omitempty"`
}

// Validate checks if the bank configuration is valid
func (bc *BankConfig) Validate() error {
	if strings.TrimSpace(bc.Name) == "" {
		return fmt.Errorf("bank name cannot be empty")
	}
	
	if strings.TrimSpace(bc.IdentifierColumn) == "" {
		return fmt.Errorf("identifier column cannot be empty")
	}
	
	if strings.TrimSpace(bc.AmountColumn) == "" {
		return fmt.Errorf("amount column cannot be empty")
	}
	
	if strings.TrimSpace(bc.DateColumn) == "" {
		return fmt.Errorf("date column cannot be empty")
	}
	
	return nil
}

// GetColumnName returns the actual column name, checking aliases first
func (bc *BankConfig) GetColumnName(standardName string) string {
	if alias, exists := bc.ColumnAliases[standardName]; exists {
		return alias
	}
	
	switch standardName {
	case "identifier":
		return bc.IdentifierColumn
	case "amount":
		return bc.AmountColumn
	case "date":
		return bc.DateColumn
	default:
		return standardName
	}
}

// TransactionParserConfig holds configuration for parsing transaction CSV files
type TransactionParserConfig struct {
	TrxIDColumn           string            `json:"trx_id_column"`
	AmountColumn          string            `json:"amount_column"`
	TypeColumn            string            `json:"type_column"`
	TransactionTimeColumn string            `json:"transaction_time_column"`
	HasHeader             bool              `json:"has_header"`
	Delimiter             rune              `json:"delimiter"`
	ColumnAliases         map[string]string `json:"column_aliases,omitempty"`
}

// Validate checks if the transaction parser configuration is valid
func (tpc *TransactionParserConfig) Validate() error {
	if strings.TrimSpace(tpc.TrxIDColumn) == "" {
		return fmt.Errorf("transaction ID column cannot be empty")
	}
	
	if strings.TrimSpace(tpc.AmountColumn) == "" {
		return fmt.Errorf("amount column cannot be empty")
	}
	
	if strings.TrimSpace(tpc.TypeColumn) == "" {
		return fmt.Errorf("type column cannot be empty")
	}
	
	if strings.TrimSpace(tpc.TransactionTimeColumn) == "" {
		return fmt.Errorf("transaction time column cannot be empty")
	}
	
	return nil
}

// GetColumnName returns the actual column name, checking aliases first
func (tpc *TransactionParserConfig) GetColumnName(standardName string) string {
	if alias, exists := tpc.ColumnAliases[standardName]; exists {
		return alias
	}
	
	switch standardName {
	case "trx_id":
		return tpc.TrxIDColumn
	case "amount":
		return tpc.AmountColumn
	case "type":
		return tpc.TypeColumn
	case "transaction_time":
		return tpc.TransactionTimeColumn
	default:
		return standardName
	}
}

// DefaultTransactionParserConfig returns a configuration with standard defaults
func DefaultTransactionParserConfig() *TransactionParserConfig {
	return &TransactionParserConfig{
		TrxIDColumn:           "trxID",
		AmountColumn:          "amount",
		TypeColumn:            "type",
		TransactionTimeColumn: "transactionTime",
		HasHeader:             true,
		Delimiter:             ',',
		ColumnAliases:         make(map[string]string),
	}
}

// Predefined bank configurations for common banks
var (
	// StandardBankConfig represents a generic bank statement format
	StandardBankConfig = &BankConfig{
		Name:             "Standard",
		IdentifierColumn: "unique_identifier",
		AmountColumn:     "amount",
		DateColumn:       "date",
		DateFormat:       "2006-01-02",
		HasHeader:        true,
		Delimiter:        ',',
		Description:      "Standard bank statement format",
	}
	
	// SampleBank1Config represents Bank1's specific format
	SampleBank1Config = &BankConfig{
		Name:             "Bank1",
		IdentifierColumn: "transaction_id",
		AmountColumn:     "transaction_amount",
		DateColumn:       "posting_date",
		DateFormat:       "01/02/2006",
		HasHeader:        true,
		Delimiter:        ',',
		ColumnAliases: map[string]string{
			"description": "transaction_description",
		},
		Description: "Bank1 statement format with MM/DD/YYYY dates",
	}
	
	// SampleBank2Config represents Bank2's specific format
	SampleBank2Config = &BankConfig{
		Name:             "Bank2",
		IdentifierColumn: "ref_number",
		AmountColumn:     "debit_credit_amount",
		DateColumn:       "value_date",
		DateFormat:       "2006-01-02",
		HasHeader:        true,
		Delimiter:        ';',
		ColumnAliases: map[string]string{
			"type":        "debit_credit_indicator",
			"description": "transaction_details",
		},
		Description: "Bank2 statement format with semicolon delimiter",
	}
)

// GetBankConfig returns a predefined bank configuration by name
func GetBankConfig(name string) *BankConfig {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "standard":
		return StandardBankConfig
	case "bank1":
		return SampleBank1Config
	case "bank2":
		return SampleBank2Config
	default:
		return nil
	}
}

// ListAvailableBankConfigs returns all available predefined bank configurations
func ListAvailableBankConfigs() []*BankConfig {
	return []*BankConfig{
		StandardBankConfig,
		SampleBank1Config,
		SampleBank2Config,
	}
}

// AutoDetectBankConfig attempts to detect the bank format from headers
func AutoDetectBankConfig(headers []string) *BankConfig {
	headerMap := make(map[string]bool)
	for _, header := range headers {
		headerMap[strings.ToLower(strings.TrimSpace(header))] = true
	}
	
	configs := ListAvailableBankConfigs()
	
	for _, config := range configs {
		score := 0
		totalFields := 3 // identifier, amount, date
		
		// Check if key columns are present
		if headerMap[strings.ToLower(config.IdentifierColumn)] {
			score++
		}
		if headerMap[strings.ToLower(config.AmountColumn)] {
			score++
		}
		if headerMap[strings.ToLower(config.DateColumn)] {
			score++
		}
		
		// If all key fields match, this is likely the right config
		if score == totalFields {
			return config
		}
	}
	
	// Return standard config as fallback
	return StandardBankConfig
}

// StreamingConfig holds configuration for streaming operations
type StreamingConfig struct {
	BatchSize         int  `json:"batch_size"`
	MaxConcurrency    int  `json:"max_concurrency"`
	BufferSize        int  `json:"buffer_size"`
	ContinueOnError   bool `json:"continue_on_error"`
	MaxErrors         int  `json:"max_errors"`
	ReportProgress    bool `json:"report_progress"`
	ProgressInterval  int  `json:"progress_interval"`
}

// DefaultStreamingConfig returns a configuration with sensible defaults for streaming
func DefaultStreamingConfig() *StreamingConfig {
	return &StreamingConfig{
		BatchSize:         1000,
		MaxConcurrency:    4,
		BufferSize:        8192,
		ContinueOnError:   true,
		MaxErrors:         100,
		ReportProgress:    false,
		ProgressInterval:  10000,
	}
}

// Validate checks if the streaming configuration is valid
func (sc *StreamingConfig) Validate() error {
	if sc.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive, got %d", sc.BatchSize)
	}
	
	if sc.MaxConcurrency <= 0 {
		return fmt.Errorf("max concurrency must be positive, got %d", sc.MaxConcurrency)
	}
	
	if sc.BufferSize <= 0 {
		return fmt.Errorf("buffer size must be positive, got %d", sc.BufferSize)
	}
	
	if sc.MaxErrors < 0 {
		return fmt.Errorf("max errors cannot be negative, got %d", sc.MaxErrors)
	}
	
	if sc.ProgressInterval <= 0 {
		return fmt.Errorf("progress interval must be positive, got %d", sc.ProgressInterval)
	}
	
	return nil
}