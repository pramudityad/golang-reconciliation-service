package reconciler

import (
	"context"
	"fmt"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/parsers"

	"github.com/shopspring/decimal"
)

// ReconciliationService orchestrates the complete reconciliation process
type ReconciliationService struct {
	transactionParser   *parsers.TransactionParser
	bankStatementParser *parsers.BankStatementParser
	matchingEngine      *matcher.MatchingEngine
	config              *Config
}

// Config holds configuration options for the reconciliation service
type Config struct {
	// Date range filtering options
	StartDate *time.Time
	EndDate   *time.Time
	
	// Processing options
	BatchSize           int
	MaxConcurrentFiles  int
	ProgressReporting   bool
	
	// Validation options
	ValidateInputs      bool
	StrictDateMatching  bool
	
	// Output options
	IncludeStatistics   bool
	DetailedBreakdown   bool
}

// DefaultConfig returns a default configuration for the reconciliation service
func DefaultConfig() *Config {
	return &Config{
		BatchSize:          1000,
		MaxConcurrentFiles: 4,
		ProgressReporting:  false,
		ValidateInputs:     true,
		StrictDateMatching: false,
		IncludeStatistics:  true,
		DetailedBreakdown:  true,
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive, got %d", c.BatchSize)
	}
	
	if c.MaxConcurrentFiles <= 0 {
		return fmt.Errorf("max concurrent files must be positive, got %d", c.MaxConcurrentFiles)
	}
	
	if c.StartDate != nil && c.EndDate != nil && c.StartDate.After(*c.EndDate) {
		return fmt.Errorf("start date must be before end date")
	}
	
	return nil
}

// ReconciliationRequest represents a request for reconciliation
type ReconciliationRequest struct {
	SystemFile        string
	BankFiles         []string
	StartDate         *time.Time
	EndDate           *time.Time
	TransactionConfig *parsers.TransactionParserConfig
	BankConfigs       map[string]*parsers.BankConfig // File path -> config mapping
}

// Validate validates the reconciliation request
func (r *ReconciliationRequest) Validate() error {
	if r.SystemFile == "" {
		return fmt.Errorf("system file path is required")
	}
	
	if len(r.BankFiles) == 0 {
		return fmt.Errorf("at least one bank file is required")
	}
	
	if r.StartDate != nil && r.EndDate != nil && r.StartDate.After(*r.EndDate) {
		return fmt.Errorf("start date must be before end date")
	}
	
	if r.TransactionConfig == nil {
		return fmt.Errorf("transaction parser configuration is required")
	}
	
	// Validate bank configs for each file
	for _, bankFile := range r.BankFiles {
		if _, exists := r.BankConfigs[bankFile]; !exists {
			return fmt.Errorf("bank configuration missing for file: %s", bankFile)
		}
	}
	
	return nil
}

// ReconciliationResult contains the complete results of reconciliation
type ReconciliationResult struct {
	// Summary information
	Summary              *ResultSummary                    `json:"summary"`
	
	// Detailed results
	MatchedTransactions   []*matcher.MatchResult           `json:"matched_transactions,omitempty"`
	UnmatchedTransactions []*models.Transaction            `json:"unmatched_transactions,omitempty"`
	UnmatchedStatements   []*models.BankStatement          `json:"unmatched_statements,omitempty"`
	
	// Processing information
	ProcessingStats       *ProcessingStats                 `json:"processing_stats,omitempty"`
	
	// Additional analysis
	Discrepancies         []*Discrepancy                   `json:"discrepancies,omitempty"`
	
	// Metadata
	ProcessedAt          time.Time                         `json:"processed_at"`
	Request              *ReconciliationRequest            `json:"request,omitempty"`
}

// ResultSummary provides a high-level overview of reconciliation results
type ResultSummary struct {
	// Transaction counts
	TotalTransactions     int `json:"total_transactions"`
	MatchedTransactions   int `json:"matched_transactions"`
	UnmatchedTransactions int `json:"unmatched_transactions"`
	
	// Bank statement counts
	TotalBankStatements     int `json:"total_bank_statements"`
	MatchedStatements       int `json:"matched_statements"`
	UnmatchedStatements     int `json:"unmatched_statements"`
	
	// Match quality breakdown
	ExactMatches    int `json:"exact_matches"`
	CloseMatches    int `json:"close_matches"`
	FuzzyMatches    int `json:"fuzzy_matches"`
	PossibleMatches int `json:"possible_matches"`
	
	// Financial summary
	TotalTransactionAmount decimal.Decimal `json:"total_transaction_amount"`
	TotalStatementAmount   decimal.Decimal `json:"total_statement_amount"`
	NetDiscrepancy         decimal.Decimal `json:"net_discrepancy"`
	
	// Processing metadata
	ProcessingDuration time.Duration `json:"processing_duration"`
	DateRange          *DateRange    `json:"date_range,omitempty"`
}

// ProcessingStats contains detailed processing statistics
type ProcessingStats struct {
	// File processing
	FilesProcessed        int           `json:"files_processed"`
	ParseErrors          int           `json:"parse_errors"`
	ValidationErrors     int           `json:"validation_errors"`
	
	// Performance metrics
	RecordsPerSecond     float64       `json:"records_per_second"`
	TotalProcessingTime  time.Duration `json:"total_processing_time"`
	ParsingTime          time.Duration `json:"parsing_time"`
	MatchingTime         time.Duration `json:"matching_time"`
	
	// Memory usage (if available)
	PeakMemoryUsage      int64         `json:"peak_memory_usage,omitempty"`
}

// Discrepancy represents a detected discrepancy in the data
type Discrepancy struct {
	Type        DiscrepancyType     `json:"type"`
	Transaction *models.Transaction `json:"transaction,omitempty"`
	Statement   *models.BankStatement `json:"statement,omitempty"`
	Description string              `json:"description"`
	Amount      decimal.Decimal     `json:"amount,omitempty"`
	Severity    Severity            `json:"severity"`
}

// DiscrepancyType represents the type of discrepancy
type DiscrepancyType string

const (
	DiscrepancyAmountDifference    DiscrepancyType = "amount_difference"
	DiscrepancyDateMismatch        DiscrepancyType = "date_mismatch"
	DiscrepancyTypeMismatch        DiscrepancyType = "type_mismatch"
	DiscrepancyDuplicateTransaction DiscrepancyType = "duplicate_transaction"
	DiscrepancyDuplicateStatement   DiscrepancyType = "duplicate_statement"
	DiscrepancyMissingTransaction   DiscrepancyType = "missing_transaction"
	DiscrepancyMissingStatement     DiscrepancyType = "missing_statement"
)

// Severity represents the severity level of a discrepancy
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
	SeverityInfo     Severity = "info"
)

// DateRange represents a date range filter
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// NewReconciliationService creates a new reconciliation service
func NewReconciliationService(
	transactionConfig *parsers.TransactionParserConfig,
	bankConfig *parsers.BankConfig,
	matchingConfig *matcher.MatchingConfig,
	config *Config,
) (*ReconciliationService, error) {
	
	if config == nil {
		config = DefaultConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Create transaction parser
	transactionParser, err := parsers.NewTransactionParser(transactionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction parser: %w", err)
	}
	
	// Create bank statement parser
	bankStatementParser, err := parsers.NewBankStatementParser(bankConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create bank statement parser: %w", err)
	}
	
	// Create matching engine
	matchingEngine := matcher.NewMatchingEngine(matchingConfig)
	
	return &ReconciliationService{
		transactionParser:   transactionParser,
		bankStatementParser: bankStatementParser,
		matchingEngine:      matchingEngine,
		config:              config,
	}, nil
}

// ProcessReconciliation performs the complete reconciliation process
func (rs *ReconciliationService) ProcessReconciliation(
	ctx context.Context,
	request *ReconciliationRequest,
) (*ReconciliationResult, error) {
	
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	
	startTime := time.Now()
	result := &ReconciliationResult{
		ProcessedAt: startTime,
		Request:     request,
		Summary:     &ResultSummary{},
		ProcessingStats: &ProcessingStats{},
	}
	
	// Set up date range for filtering
	if request.StartDate != nil && request.EndDate != nil {
		result.Summary.DateRange = &DateRange{
			Start: *request.StartDate,
			End:   *request.EndDate,
		}
	}
	
	// Step 1: Parse system transactions
	transactions, parseStats, err := rs.parseSystemTransactions(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse system transactions: %w", err)
	}
	
	// Step 2: Parse bank statements from all files
	statements, bankParseStats, err := rs.parseBankStatements(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bank statements: %w", err)
	}
	
	// Step 3: Apply date range filtering
	transactions, statements = rs.applyDateRangeFiltering(transactions, statements, request)
	
	// Step 4: Perform reconciliation matching
	matchingStartTime := time.Now()
	reconciliationResult, err := rs.performMatching(ctx, transactions, statements)
	if err != nil {
		return nil, fmt.Errorf("failed to perform matching: %w", err)
	}
	matchingDuration := time.Since(matchingStartTime)
	
	// Step 5: Analyze discrepancies
	discrepancies := rs.analyzeDiscrepancies(reconciliationResult.Matches, transactions, statements)
	
	// Step 6: Build final result
	rs.buildFinalResult(result, reconciliationResult, discrepancies, parseStats, bankParseStats, matchingDuration)
	
	// Calculate total processing time
	result.Summary.ProcessingDuration = time.Since(startTime)
	result.ProcessingStats.TotalProcessingTime = result.Summary.ProcessingDuration
	
	return result, nil
}

// UpdateConfiguration updates the service configuration
func (rs *ReconciliationService) UpdateConfiguration(config *Config) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	rs.config = config
	return nil
}

// GetConfiguration returns the current configuration
func (rs *ReconciliationService) GetConfiguration() *Config {
	return rs.config
}

// GetStats returns current processing statistics
func (rs *ReconciliationService) GetStats() *ProcessingStats {
	// This would return real-time stats in a full implementation
	return &ProcessingStats{}
}