// Package reporter provides comprehensive reporting capabilities for reconciliation results.
//
// This package generates various types of reports from reconciliation results,
// supporting multiple output formats and providing detailed analysis of matches,
// discrepancies, and summary statistics.
//
// Supported output formats:
//   - Console: Human-readable tabular output for terminal display
//   - JSON: Structured data format for programmatic consumption
//   - CSV: Comma-separated format for spreadsheet applications
//
// Report types available:
//   - Reconciliation reports: complete match and discrepancy analysis
//   - Summary reports: high-level statistics and totals
//   - Unmatched transaction reports: items requiring investigation
//   - Match detail reports: in-depth analysis of specific matches
//
// Example usage:
//
//	reporter := reporter.NewReconciliationReporter()
//	report, err := reporter.GenerateReport(result, reporter.FormatJSON)
//	
//	// Generate console report with custom options
//	options := &ReportOptions{
//		IncludeMatchDetails: true,
//		SortBy: "amount",
//		MaxItems: 100,
//	}
//	consoleReport, err := reporter.GenerateConsoleReport(result, options)
package reporter

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/reconciler"

	"github.com/shopspring/decimal"
)

// OutputFormat represents the supported report output formats.
// Each format is optimized for different use cases and audiences.
type OutputFormat string

const (
	FormatConsole OutputFormat = "console"
	FormatJSON    OutputFormat = "json"
	FormatCSV     OutputFormat = "csv"
)

// IsValid checks if the output format is supported
func (f OutputFormat) IsValid() bool {
	switch f {
	case FormatConsole, FormatJSON, FormatCSV:
		return true
	default:
		return false
	}
}

// ReportConfig holds configuration options for report generation
type ReportConfig struct {
	// Output format
	Format OutputFormat `json:"format"`
	
	// Detail level options
	IncludeMatchedTransactions   bool `json:"include_matched_transactions"`
	IncludeUnmatchedTransactions bool `json:"include_unmatched_transactions"`
	IncludeUnmatchedStatements   bool `json:"include_unmatched_statements"`
	IncludeDiscrepancies         bool `json:"include_discrepancies"`
	IncludeProcessingStats       bool `json:"include_processing_stats"`
	
	// Console formatting options
	UseColors         bool `json:"use_colors"`
	ShowProgressBars  bool `json:"show_progress_bars"`
	TableMaxWidth     int  `json:"table_max_width"`
	
	// CSV options
	CSVDelimiter rune   `json:"csv_delimiter"`
	CSVHeaders   bool   `json:"csv_headers"`
	
	// Grouping options
	GroupUnmatchedByBank bool `json:"group_unmatched_by_bank"`
	SortByAmount         bool `json:"sort_by_amount"`
}

// DefaultReportConfig returns a default report configuration
func DefaultReportConfig() *ReportConfig {
	return &ReportConfig{
		Format:                       FormatConsole,
		IncludeMatchedTransactions:   false,
		IncludeUnmatchedTransactions: true,
		IncludeUnmatchedStatements:   true,
		IncludeDiscrepancies:         true,
		IncludeProcessingStats:       true,
		UseColors:                    true,
		ShowProgressBars:             false,
		TableMaxWidth:                120,
		CSVDelimiter:                 ',',
		CSVHeaders:                   true,
		GroupUnmatchedByBank:         true,
		SortByAmount:                 false,
	}
}

// Validate validates the report configuration
func (c *ReportConfig) Validate() error {
	if !c.Format.IsValid() {
		return fmt.Errorf("invalid output format: %s", c.Format)
	}
	
	if c.TableMaxWidth < 50 {
		return fmt.Errorf("table max width must be at least 50 characters, got %d", c.TableMaxWidth)
	}
	
	return nil
}

// ReportGenerator generates reconciliation reports in various formats
type ReportGenerator struct {
	config *ReportConfig
}

// NewReportGenerator creates a new report generator with the specified configuration
func NewReportGenerator(config *ReportConfig) (*ReportGenerator, error) {
	if config == nil {
		config = DefaultReportConfig()
	}
	
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid report configuration: %w", err)
	}
	
	return &ReportGenerator{
		config: config,
	}, nil
}

// GenerateReport generates a report from reconciliation results and writes it to the provided writer
func (rg *ReportGenerator) GenerateReport(result *reconciler.ReconciliationResult, writer io.Writer) error {
	if result == nil {
		return fmt.Errorf("reconciliation result cannot be nil")
	}
	
	switch rg.config.Format {
	case FormatConsole:
		return rg.generateConsoleReport(result, writer)
	case FormatJSON:
		return rg.generateJSONReport(result, writer)
	case FormatCSV:
		return rg.generateCSVReport(result, writer)
	default:
		return fmt.Errorf("unsupported output format: %s", rg.config.Format)
	}
}

// generateConsoleReport generates a human-readable console report
func (rg *ReportGenerator) generateConsoleReport(result *reconciler.ReconciliationResult, writer io.Writer) error {
	// Report header
	fmt.Fprintf(writer, "RECONCILIATION REPORT\n")
	fmt.Fprintf(writer, "Generated: %s\n", result.ProcessedAt.Format(time.RFC3339))
	fmt.Fprintf(writer, "Processing Duration: %v\n\n", result.Summary.ProcessingDuration)
	
	// Summary section
	fmt.Fprintf(writer, "=== SUMMARY ===\n")
	rg.printSummaryTable(result.Summary, writer)
	fmt.Fprintf(writer, "\n")
	
	// Financial summary
	fmt.Fprintf(writer, "=== FINANCIAL SUMMARY ===\n")
	rg.printFinancialSummary(result.Summary, writer)
	fmt.Fprintf(writer, "\n")
	
	// Match quality breakdown
	fmt.Fprintf(writer, "=== MATCH QUALITY BREAKDOWN ===\n")
	rg.printMatchQualityTable(result.Summary, writer)
	fmt.Fprintf(writer, "\n")
	
	// Unmatched transactions
	if rg.config.IncludeUnmatchedTransactions && len(result.UnmatchedTransactions) > 0 {
		fmt.Fprintf(writer, "=== UNMATCHED TRANSACTIONS ===\n")
		rg.printUnmatchedTransactions(result.UnmatchedTransactions, writer)
		fmt.Fprintf(writer, "\n")
	}
	
	// Unmatched bank statements
	if rg.config.IncludeUnmatchedStatements && len(result.UnmatchedStatements) > 0 {
		fmt.Fprintf(writer, "=== UNMATCHED BANK STATEMENTS ===\n")
		rg.printUnmatchedStatements(result.UnmatchedStatements, writer)
		fmt.Fprintf(writer, "\n")
	}
	
	// Discrepancies
	if rg.config.IncludeDiscrepancies && len(result.Discrepancies) > 0 {
		fmt.Fprintf(writer, "=== DISCREPANCIES ===\n")
		rg.printDiscrepancies(result.Discrepancies, writer)
		fmt.Fprintf(writer, "\n")
	}
	
	// Processing statistics
	if rg.config.IncludeProcessingStats && result.ProcessingStats != nil {
		fmt.Fprintf(writer, "=== PROCESSING STATISTICS ===\n")
		rg.printProcessingStats(result.ProcessingStats, writer)
	}
	
	return nil
}

// generateJSONReport generates a structured JSON report
func (rg *ReportGenerator) generateJSONReport(result *reconciler.ReconciliationResult, writer io.Writer) error {
	// Create a filtered result based on configuration
	filteredResult := rg.filterResultForOutput(result)
	
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	
	return encoder.Encode(filteredResult)
}

// generateCSVReport generates a CSV report with transaction details
func (rg *ReportGenerator) generateCSVReport(result *reconciler.ReconciliationResult, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	csvWriter.Comma = rg.config.CSVDelimiter
	defer csvWriter.Flush()
	
	// Write headers if enabled
	if rg.config.CSVHeaders {
		headers := []string{
			"Type",
			"ID",
			"Amount",
			"Transaction_Type",
			"Date",
			"Status",
			"Bank_File",
			"Match_Type",
			"Confidence_Score",
			"Amount_Difference",
			"Date_Difference",
			"Notes",
		}
		if err := csvWriter.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV headers: %w", err)
		}
	}
	
	// Write matched transactions if requested
	if rg.config.IncludeMatchedTransactions {
		for _, match := range result.MatchedTransactions {
			record := []string{
				"Matched Transaction",
				match.Transaction.TrxID,
				match.Transaction.Amount.String(),
				string(match.Transaction.Type),
				match.Transaction.TransactionTime.Format("2006-01-02 15:04:05"),
				"Matched",
				"", // Bank file would need to be tracked separately
				match.MatchType.String(),
				fmt.Sprintf("%.2f", match.ConfidenceScore),
				match.AmountDifference.String(),
				match.DateDifference.String(),
				strings.Join(match.Reasons, "; "),
			}
			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("failed to write matched transaction record: %w", err)
			}
		}
	}
	
	// Write unmatched transactions
	if rg.config.IncludeUnmatchedTransactions {
		for _, tx := range result.UnmatchedTransactions {
			record := []string{
				"Unmatched Transaction",
				tx.TrxID,
				tx.Amount.String(),
				string(tx.Type),
				tx.TransactionTime.Format("2006-01-02 15:04:05"),
				"Unmatched",
				"",
				"",
				"",
				"",
				"",
				"No matching bank statement found",
			}
			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("failed to write unmatched transaction record: %w", err)
			}
		}
	}
	
	// Write unmatched bank statements
	if rg.config.IncludeUnmatchedStatements {
		for _, stmt := range result.UnmatchedStatements {
			record := []string{
				"Unmatched Bank Statement",
				stmt.UniqueIdentifier,
				stmt.Amount.String(),
				string(stmt.GetTransactionType()),
				stmt.Date.Format("2006-01-02"),
				"Unmatched",
				"", // Bank file would need to be tracked separately
				"",
				"",
				"",
				"",
				"No matching system transaction found",
			}
			if err := csvWriter.Write(record); err != nil {
				return fmt.Errorf("failed to write unmatched statement record: %w", err)
			}
		}
	}
	
	return nil
}

// Helper methods for console output formatting

func (rg *ReportGenerator) printSummaryTable(summary *reconciler.ResultSummary, writer io.Writer) {
	fmt.Fprintf(writer, "Transactions:\n")
	fmt.Fprintf(writer, "  Total:     %d\n", summary.TotalTransactions)
	fmt.Fprintf(writer, "  Matched:   %d (%.1f%%)\n", 
		summary.MatchedTransactions, 
		rg.calculatePercentage(summary.MatchedTransactions, summary.TotalTransactions))
	fmt.Fprintf(writer, "  Unmatched: %d (%.1f%%)\n", 
		summary.UnmatchedTransactions,
		rg.calculatePercentage(summary.UnmatchedTransactions, summary.TotalTransactions))
	
	fmt.Fprintf(writer, "\nBank Statements:\n")
	fmt.Fprintf(writer, "  Total:     %d\n", summary.TotalBankStatements)
	fmt.Fprintf(writer, "  Matched:   %d (%.1f%%)\n", 
		summary.MatchedStatements,
		rg.calculatePercentage(summary.MatchedStatements, summary.TotalBankStatements))
	fmt.Fprintf(writer, "  Unmatched: %d (%.1f%%)\n", 
		summary.UnmatchedStatements,
		rg.calculatePercentage(summary.UnmatchedStatements, summary.TotalBankStatements))
}

func (rg *ReportGenerator) printFinancialSummary(summary *reconciler.ResultSummary, writer io.Writer) {
	fmt.Fprintf(writer, "Total Transaction Amount: %s\n", summary.TotalTransactionAmount.StringFixed(2))
	fmt.Fprintf(writer, "Total Statement Amount:   %s\n", summary.TotalStatementAmount.StringFixed(2))
	fmt.Fprintf(writer, "Net Discrepancy:          %s\n", summary.NetDiscrepancy.StringFixed(2))
	
	if !summary.NetDiscrepancy.IsZero() {
		discrepancyPct := summary.NetDiscrepancy.Abs().Div(summary.TotalTransactionAmount).Mul(decimal.NewFromInt(100))
		fmt.Fprintf(writer, "Discrepancy Percentage:   %s%%\n", discrepancyPct.StringFixed(2))
	}
}

func (rg *ReportGenerator) printMatchQualityTable(summary *reconciler.ResultSummary, writer io.Writer) {
	total := summary.ExactMatches + summary.CloseMatches + summary.FuzzyMatches + summary.PossibleMatches
	
	fmt.Fprintf(writer, "Exact Matches:    %d (%.1f%%)\n", 
		summary.ExactMatches, rg.calculatePercentage(summary.ExactMatches, total))
	fmt.Fprintf(writer, "Close Matches:    %d (%.1f%%)\n", 
		summary.CloseMatches, rg.calculatePercentage(summary.CloseMatches, total))
	fmt.Fprintf(writer, "Fuzzy Matches:    %d (%.1f%%)\n", 
		summary.FuzzyMatches, rg.calculatePercentage(summary.FuzzyMatches, total))
	fmt.Fprintf(writer, "Possible Matches: %d (%.1f%%)\n", 
		summary.PossibleMatches, rg.calculatePercentage(summary.PossibleMatches, total))
}

func (rg *ReportGenerator) printUnmatchedTransactions(transactions []*models.Transaction, writer io.Writer) {
	// Sort transactions if requested
	if rg.config.SortByAmount {
		sort.Slice(transactions, func(i, j int) bool {
			return transactions[i].Amount.GreaterThan(transactions[j].Amount)
		})
	}
	
	fmt.Fprintf(writer, "Total Unmatched Transactions: %d\n\n", len(transactions))
	
	// Group by type if helpful
	debitTxns := make([]*models.Transaction, 0)
	creditTxns := make([]*models.Transaction, 0)
	
	for _, tx := range transactions {
		if tx.IsDebit() {
			debitTxns = append(debitTxns, tx)
		} else {
			creditTxns = append(creditTxns, tx)
		}
	}
	
	if len(debitTxns) > 0 {
		fmt.Fprintf(writer, "Debit Transactions (%d):\n", len(debitTxns))
		rg.printTransactionList(debitTxns, writer)
		fmt.Fprintf(writer, "\n")
	}
	
	if len(creditTxns) > 0 {
		fmt.Fprintf(writer, "Credit Transactions (%d):\n", len(creditTxns))
		rg.printTransactionList(creditTxns, writer)
	}
}

func (rg *ReportGenerator) printUnmatchedStatements(statements []*models.BankStatement, writer io.Writer) {
	// Sort statements if requested
	if rg.config.SortByAmount {
		sort.Slice(statements, func(i, j int) bool {
			return statements[i].Amount.Abs().GreaterThan(statements[j].Amount.Abs())
		})
	}
	
	fmt.Fprintf(writer, "Total Unmatched Bank Statements: %d\n\n", len(statements))
	
	// Group by debit/credit
	debitStmts := make([]*models.BankStatement, 0)
	creditStmts := make([]*models.BankStatement, 0)
	
	for _, stmt := range statements {
		if stmt.IsDebit() {
			debitStmts = append(debitStmts, stmt)
		} else {
			creditStmts = append(creditStmts, stmt)
		}
	}
	
	if len(debitStmts) > 0 {
		fmt.Fprintf(writer, "Debit Statements (%d):\n", len(debitStmts))
		rg.printStatementList(debitStmts, writer)
		fmt.Fprintf(writer, "\n")
	}
	
	if len(creditStmts) > 0 {
		fmt.Fprintf(writer, "Credit Statements (%d):\n", len(creditStmts))
		rg.printStatementList(creditStmts, writer)
	}
}

func (rg *ReportGenerator) printDiscrepancies(discrepancies []*reconciler.Discrepancy, writer io.Writer) {
	fmt.Fprintf(writer, "Total Discrepancies Found: %d\n\n", len(discrepancies))
	
	// Group by severity
	severityGroups := make(map[reconciler.Severity][]*reconciler.Discrepancy)
	for _, disc := range discrepancies {
		severityGroups[disc.Severity] = append(severityGroups[disc.Severity], disc)
	}
	
	// Print in severity order
	severities := []reconciler.Severity{
		reconciler.SeverityCritical,
		reconciler.SeverityHigh,
		reconciler.SeverityMedium,
		reconciler.SeverityLow,
		reconciler.SeverityInfo,
	}
	
	for _, severity := range severities {
		discs := severityGroups[severity]
		if len(discs) == 0 {
			continue
		}
		
		fmt.Fprintf(writer, "%s Severity (%d):\n", strings.ToUpper(string(severity)), len(discs))
		for _, disc := range discs {
			fmt.Fprintf(writer, "  - %s: %s", disc.Type, disc.Description)
			if !disc.Amount.IsZero() {
				fmt.Fprintf(writer, " (Amount: %s)", disc.Amount.StringFixed(2))
			}
			fmt.Fprintf(writer, "\n")
		}
		fmt.Fprintf(writer, "\n")
	}
}

func (rg *ReportGenerator) printProcessingStats(stats *reconciler.ProcessingStats, writer io.Writer) {
	fmt.Fprintf(writer, "Files Processed:      %d\n", stats.FilesProcessed)
	fmt.Fprintf(writer, "Parse Errors:         %d\n", stats.ParseErrors)
	fmt.Fprintf(writer, "Validation Errors:    %d\n", stats.ValidationErrors)
	fmt.Fprintf(writer, "Records/Second:       %.2f\n", stats.RecordsPerSecond)
	fmt.Fprintf(writer, "Total Processing:     %v\n", stats.TotalProcessingTime)
	fmt.Fprintf(writer, "Parsing Time:         %v\n", stats.ParsingTime)
	fmt.Fprintf(writer, "Matching Time:        %v\n", stats.MatchingTime)
	
	if stats.PeakMemoryUsage > 0 {
		fmt.Fprintf(writer, "Peak Memory Usage:    %d MB\n", stats.PeakMemoryUsage/(1024*1024))
	}
}

func (rg *ReportGenerator) printTransactionList(transactions []*models.Transaction, writer io.Writer) {
	for i, tx := range transactions {
		fmt.Fprintf(writer, "  %d. ID: %s, Amount: %s, Type: %s, Time: %s\n",
			i+1,
			tx.TrxID,
			tx.Amount.StringFixed(2),
			tx.Type,
			tx.TransactionTime.Format("2006-01-02 15:04:05"))
		
		// Limit output for very long lists
		if i >= 9 && len(transactions) > 10 {
			fmt.Fprintf(writer, "  ... and %d more\n", len(transactions)-10)
			break
		}
	}
}

func (rg *ReportGenerator) printStatementList(statements []*models.BankStatement, writer io.Writer) {
	for i, stmt := range statements {
		fmt.Fprintf(writer, "  %d. ID: %s, Amount: %s, Date: %s\n",
			i+1,
			stmt.UniqueIdentifier,
			stmt.Amount.StringFixed(2),
			stmt.Date.Format("2006-01-02"))
		
		// Limit output for very long lists
		if i >= 9 && len(statements) > 10 {
			fmt.Fprintf(writer, "  ... and %d more\n", len(statements)-10)
			break
		}
	}
}

// Helper methods

func (rg *ReportGenerator) calculatePercentage(part, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return float64(part) / float64(total) * 100.0
}

func (rg *ReportGenerator) filterResultForOutput(result *reconciler.ReconciliationResult) map[string]interface{} {
	output := map[string]interface{}{
		"summary":      result.Summary,
		"processed_at": result.ProcessedAt,
	}
	
	if rg.config.IncludeMatchedTransactions && result.MatchedTransactions != nil {
		output["matched_transactions"] = result.MatchedTransactions
	}
	
	if rg.config.IncludeUnmatchedTransactions && result.UnmatchedTransactions != nil {
		output["unmatched_transactions"] = result.UnmatchedTransactions
	}
	
	if rg.config.IncludeUnmatchedStatements && result.UnmatchedStatements != nil {
		output["unmatched_statements"] = result.UnmatchedStatements
	}
	
	if rg.config.IncludeDiscrepancies && result.Discrepancies != nil {
		output["discrepancies"] = result.Discrepancies
	}
	
	if rg.config.IncludeProcessingStats && result.ProcessingStats != nil {
		output["processing_stats"] = result.ProcessingStats
	}
	
	return output
}

// UpdateConfiguration updates the report generator configuration
func (rg *ReportGenerator) UpdateConfiguration(config *ReportConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid report configuration: %w", err)
	}
	
	rg.config = config
	return nil
}

// GetConfiguration returns the current configuration
func (rg *ReportGenerator) GetConfiguration() *ReportConfig {
	return rg.config
}