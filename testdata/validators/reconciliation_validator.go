package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/parsers"

	"github.com/shopspring/decimal"
)

// ReconciliationValidator validates end-to-end reconciliation scenarios
type ReconciliationValidator struct {
	Verbose     bool
	DataDir     string
	TempDir     string
	Engine      *matcher.MatchingEngine
}

// ValidationTest represents a reconciliation validation test
type ValidationTest struct {
	Name                string
	TransactionFile     string
	StatementFiles      []string
	ExpectedMatches     int
	ExpectedUnmatched   int
	ExpectedMatchRate   float64
	MaxProcessingTime   time.Duration
	Config              *matcher.MatchingConfig
}

// TestResult represents the result of a validation test
type TestResult struct {
	Test                ValidationTest
	Success             bool
	ActualMatches       int
	ActualUnmatched     int
	ActualMatchRate     float64
	ProcessingTime      time.Duration
	MatchTypes          map[matcher.MatchType]int
	Errors              []string
	Warnings            []string
	PerformanceMetrics  PerformanceMetrics
}

// PerformanceMetrics captures performance-related metrics
type PerformanceMetrics struct {
	LoadTime        time.Duration
	IndexTime       time.Duration
	MatchingTime    time.Duration
	TotalTime       time.Duration
	MemoryUsage     int64
	TransactionRate float64 // Transactions per second
}

func main() {
	var (
		dataDir   = flag.String("data-dir", "../csv", "Directory containing test data")
		output    = flag.String("output", "", "Output file for validation report")
		verbose   = flag.Bool("verbose", false, "Verbose output")
		testName  = flag.String("test", "all", "Specific test to run (or 'all')")
		configType = flag.String("config", "default", "Matching configuration: default, strict, relaxed")
	)
	flag.Parse()

	validator := &ReconciliationValidator{
		Verbose: *verbose,
		DataDir: *dataDir,
		TempDir: "/tmp/reconciliation_validation",
	}

	// Create temp directory for test outputs
	if err := os.MkdirAll(validator.TempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(validator.TempDir)

	// Configure matching engine
	var config *matcher.MatchingConfig
	switch *configType {
	case "strict":
		config = matcher.StrictMatchingConfig()
	case "relaxed":
		config = matcher.RelaxedMatchingConfig()
	default:
		config = matcher.DefaultMatchingConfig()
	}
	validator.Engine = matcher.NewMatchingEngine(config)

	fmt.Println("Reconciliation End-to-End Validator")
	fmt.Println("===================================")
	fmt.Printf("Data directory: %s\n", *dataDir)
	fmt.Printf("Configuration: %s\n", *configType)
	fmt.Printf("Target test: %s\n\n", *testName)

	// Define validation tests
	tests := validator.DefineValidationTests()

	var selectedTests []ValidationTest
	if *testName == "all" {
		selectedTests = tests
	} else {
		for _, test := range tests {
			if test.Name == *testName {
				selectedTests = []ValidationTest{test}
				break
			}
		}
		if len(selectedTests) == 0 {
			log.Fatalf("Unknown test: %s", *testName)
		}
	}

	// Run validation tests
	var results []TestResult
	for _, test := range selectedTests {
		result := validator.RunValidationTest(test)
		results = append(results, result)
	}

	// Print results
	validator.PrintResults(results)

	// Write report if requested
	if *output != "" {
		if err := validator.WriteReport(*output, results); err != nil {
			log.Printf("Failed to write report: %v", err)
		} else {
			fmt.Printf("\nReconciliation validation report written to: %s\n", *output)
		}
	}

	// Check if all tests passed
	allPassed := true
	for _, result := range results {
		if !result.Success {
			allPassed = false
			break
		}
	}

	if !allPassed {
		os.Exit(1)
	}
}

// DefineValidationTests defines the set of validation tests to run
func (rv *ReconciliationValidator) DefineValidationTests() []ValidationTest {
	return []ValidationTest{
		{
			Name:                "basic_reconciliation",
			TransactionFile:     "system_transactions.csv",
			StatementFiles:      []string{"bank_statement_bank1.csv"},
			ExpectedMatches:     90,
			ExpectedUnmatched:   20,
			ExpectedMatchRate:   80.0,
			MaxProcessingTime:   5 * time.Second,
		},
		{
			Name:                "multi_bank_reconciliation",
			TransactionFile:     "system_transactions.csv",
			StatementFiles:      []string{"bank_statement_bank1.csv", "bank_statement_bank2.csv"},
			ExpectedMatches:     100,
			ExpectedUnmatched:   15,
			ExpectedMatchRate:   85.0,
			MaxProcessingTime:   8 * time.Second,
		},
		{
			Name:                "duplicate_handling",
			TransactionFile:     "edge_cases/duplicate_transactions.csv",
			StatementFiles:      []string{"edge_cases/duplicate_statements.csv"},
			ExpectedMatches:     5,
			ExpectedUnmatched:   3,
			ExpectedMatchRate:   60.0,
			MaxProcessingTime:   3 * time.Second,
		},
		{
			Name:                "same_day_transactions",
			TransactionFile:     "edge_cases/same_day_multiple.csv",
			StatementFiles:      []string{"edge_cases/same_day_statements.csv"},
			ExpectedMatches:     6,
			ExpectedUnmatched:   2,
			ExpectedMatchRate:   75.0,
			MaxProcessingTime:   3 * time.Second,
		},
		{
			Name:                "large_amounts",
			TransactionFile:     "edge_cases/large_amounts.csv",
			StatementFiles:      []string{"edge_cases/large_amount_statements.csv"},
			ExpectedMatches:     4,
			ExpectedUnmatched:   1,
			ExpectedMatchRate:   80.0,
			MaxProcessingTime:   2 * time.Second,
		},
		{
			Name:                "boundary_dates",
			TransactionFile:     "edge_cases/boundary_dates.csv",
			StatementFiles:      []string{"edge_cases/boundary_date_statements.csv"},
			ExpectedMatches:     6,
			ExpectedUnmatched:   2,
			ExpectedMatchRate:   75.0,
			MaxProcessingTime:   3 * time.Second,
		},
		{
			Name:                "partial_matches",
			TransactionFile:     "edge_cases/partial_matches.csv",
			StatementFiles:      []string{"edge_cases/partial_match_statements.csv"},
			ExpectedMatches:     2,
			ExpectedUnmatched:   6,
			ExpectedMatchRate:   25.0, // Low match rate expected for partial scenarios
			MaxProcessingTime:   4 * time.Second,
		},
	}
}

// RunValidationTest runs a single validation test
func (rv *ReconciliationValidator) RunValidationTest(test ValidationTest) TestResult {
	result := TestResult{
		Test:       test,
		MatchTypes: make(map[matcher.MatchType]int),
		Errors:     []string{},
		Warnings:   []string{},
	}

	if rv.Verbose {
		fmt.Printf("Running test: %s\n", test.Name)
	}

	startTime := time.Now()

	// Load transaction file
	transactionPath := filepath.Join(rv.DataDir, test.TransactionFile)
	if _, err := os.Stat(transactionPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Sprintf("Transaction file not found: %s", transactionPath))
		return result
	}

	loadStart := time.Now()
	transactions, err := rv.loadTransactions(transactionPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to load transactions: %v", err))
		return result
	}
	result.PerformanceMetrics.LoadTime = time.Since(loadStart)

	// Load statement files
	var allStatements []*models.BankStatement
	for _, stmtFile := range test.StatementFiles {
		stmtPath := filepath.Join(rv.DataDir, stmtFile)
		if _, err := os.Stat(stmtPath); os.IsNotExist(err) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Statement file not found: %s", stmtPath))
			continue
		}

		statements, err := rv.loadStatements(stmtPath)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to load statements from %s: %v", stmtFile, err))
			continue
		}
		allStatements = append(allStatements, statements...)
	}

	if len(allStatements) == 0 {
		result.Errors = append(result.Errors, "No statement data loaded")
		return result
	}

	// Configure engine if test specifies custom config
	if test.Config != nil {
		rv.Engine.UpdateConfiguration(test.Config)
	}

	// Load data into matching engine
	indexStart := time.Now()
	if err := rv.Engine.LoadTransactions(transactions); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to load transactions into engine: %v", err))
		return result
	}

	if err := rv.Engine.LoadBankStatements(allStatements); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to load statements into engine: %v", err))
		return result
	}
	result.PerformanceMetrics.IndexTime = time.Since(indexStart)

	// Run reconciliation
	matchingStart := time.Now()
	reconciliationResult, err := rv.Engine.Reconcile()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Reconciliation failed: %v", err))
		return result
	}
	result.PerformanceMetrics.MatchingTime = time.Since(matchingStart)
	result.PerformanceMetrics.TotalTime = time.Since(startTime)

	// Analyze results
	result.ActualMatches = len(reconciliationResult.Matches)
	result.ActualUnmatched = len(reconciliationResult.UnmatchedTransactions)
	
	if len(transactions) > 0 {
		result.ActualMatchRate = float64(result.ActualMatches) / float64(len(transactions)) * 100
	}

	// Count match types
	for _, match := range reconciliationResult.Matches {
		result.MatchTypes[match.MatchType]++
	}

	// Calculate performance metrics
	if result.PerformanceMetrics.TotalTime > 0 {
		result.PerformanceMetrics.TransactionRate = float64(len(transactions)) / result.PerformanceMetrics.TotalTime.Seconds()
	}

	result.ProcessingTime = result.PerformanceMetrics.TotalTime

	// Validate expectations
	result.Success = rv.validateExpectations(test, result)

	if rv.Verbose {
		fmt.Printf("  Completed in %v\n", result.ProcessingTime)
		fmt.Printf("  Matches: %d (expected: %d)\n", result.ActualMatches, test.ExpectedMatches)
		fmt.Printf("  Match rate: %.1f%% (expected: %.1f%%)\n", result.ActualMatchRate, test.ExpectedMatchRate)
	}

	return result
}

// loadTransactions loads transactions from CSV file
func (rv *ReconciliationValidator) loadTransactions(filename string) ([]*models.Transaction, error) {
	parser := parsers.NewTransactionParser(nil)
	return parser.ParseFile(filename)
}

// loadStatements loads bank statements from CSV file
func (rv *ReconciliationValidator) loadStatements(filename string) ([]*models.BankStatement, error) {
	parser := parsers.NewBankStatementParser(nil)
	return parser.ParseFile(filename)
}

// validateExpectations validates test results against expectations
func (rv *ReconciliationValidator) validateExpectations(test ValidationTest, result TestResult) bool {
	success := true

	// Check processing time
	if result.ProcessingTime > test.MaxProcessingTime {
		result.Errors = append(result.Errors, 
			fmt.Sprintf("Processing time exceeded limit: %v > %v", result.ProcessingTime, test.MaxProcessingTime))
		success = false
	}

	// Check match count (allow 10% tolerance)
	expectedMin := int(float64(test.ExpectedMatches) * 0.9)
	expectedMax := int(float64(test.ExpectedMatches) * 1.1)
	if result.ActualMatches < expectedMin || result.ActualMatches > expectedMax {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Match count outside expected range: %d (expected: %d±10%%)", result.ActualMatches, test.ExpectedMatches))
	}

	// Check match rate (allow 5% tolerance)
	expectedMinRate := test.ExpectedMatchRate - 5.0
	expectedMaxRate := test.ExpectedMatchRate + 5.0
	if result.ActualMatchRate < expectedMinRate || result.ActualMatchRate > expectedMaxRate {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Match rate outside expected range: %.1f%% (expected: %.1f%%±5%%)", result.ActualMatchRate, test.ExpectedMatchRate))
	}

	// Performance thresholds
	if result.PerformanceMetrics.TransactionRate < 100 {
		result.Warnings = append(result.Warnings, 
			fmt.Sprintf("Low transaction processing rate: %.1f tx/sec", result.PerformanceMetrics.TransactionRate))
	}

	return success && len(result.Errors) == 0
}

// PrintResults prints validation results to console
func (rv *ReconciliationValidator) PrintResults(results []TestResult) {
	fmt.Println("Reconciliation Validation Results")
	fmt.Println("================================")

	passedTests := 0
	totalTests := len(results)

	for _, result := range results {
		fmt.Printf("\nTest: %s\n", result.Test.Name)
		
		if result.Success {
			fmt.Printf("Status: ✅ PASSED\n")
			passedTests++
		} else {
			fmt.Printf("Status: ❌ FAILED\n")
		}

		fmt.Printf("Processing Time: %v\n", result.ProcessingTime)
		fmt.Printf("Matches: %d\n", result.ActualMatches)
		fmt.Printf("Unmatched: %d\n", result.ActualUnmatched)
		fmt.Printf("Match Rate: %.1f%%\n", result.ActualMatchRate)

		// Show match type breakdown
		if len(result.MatchTypes) > 0 {
			fmt.Printf("Match Types:\n")
			for matchType, count := range result.MatchTypes {
				fmt.Printf("  %s: %d\n", matchType.String(), count)
			}
		}

		// Show performance metrics
		if rv.Verbose {
			fmt.Printf("Performance:\n")
			fmt.Printf("  Load Time: %v\n", result.PerformanceMetrics.LoadTime)
			fmt.Printf("  Index Time: %v\n", result.PerformanceMetrics.IndexTime)
			fmt.Printf("  Matching Time: %v\n", result.PerformanceMetrics.MatchingTime)
			fmt.Printf("  Transaction Rate: %.1f tx/sec\n", result.PerformanceMetrics.TransactionRate)
		}

		// Show errors
		if len(result.Errors) > 0 {
			fmt.Printf("Errors:\n")
			for _, err := range result.Errors {
				fmt.Printf("  ❌ %s\n", err)
			}
		}

		// Show warnings
		if len(result.Warnings) > 0 {
			fmt.Printf("Warnings:\n")
			for _, warning := range result.Warnings {
				fmt.Printf("  ⚠️  %s\n", warning)
			}
		}
	}

	// Overall summary
	fmt.Printf("\nOverall Test Results\n")
	fmt.Printf("===================\n")
	fmt.Printf("Tests passed: %d/%d (%.1f%%)\n", 
		passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)

	if passedTests == totalTests {
		fmt.Printf("Result: ✅ ALL TESTS PASSED\n")
	} else {
		fmt.Printf("Result: ❌ SOME TESTS FAILED\n")
	}
}

// WriteReport writes a detailed validation report
func (rv *ReconciliationValidator) WriteReport(filename string, results []TestResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "Reconciliation End-to-End Validation Report\n")
	fmt.Fprintf(file, "===========================================\n")
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Data Directory: %s\n\n", rv.DataDir)

	// Write summary
	passedTests := 0
	totalTests := len(results)
	var totalProcessingTime time.Duration
	var totalTransactions int
	
	for _, result := range results {
		if result.Success {
			passedTests++
		}
		totalProcessingTime += result.ProcessingTime
		totalTransactions += result.ActualMatches + result.ActualUnmatched
	}

	fmt.Fprintf(file, "Summary\n")
	fmt.Fprintf(file, "-------\n")
	fmt.Fprintf(file, "Tests run: %d\n", totalTests)
	fmt.Fprintf(file, "Tests passed: %d\n", passedTests)
	fmt.Fprintf(file, "Tests failed: %d\n", totalTests-passedTests)
	fmt.Fprintf(file, "Overall success rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)
	fmt.Fprintf(file, "Total processing time: %v\n", totalProcessingTime)
	fmt.Fprintf(file, "Total transactions processed: %d\n", totalTransactions)
	if totalProcessingTime > 0 {
		fmt.Fprintf(file, "Average processing rate: %.1f tx/sec\n\n", 
			float64(totalTransactions)/totalProcessingTime.Seconds())
	}

	// Write detailed results
	for _, result := range results {
		fmt.Fprintf(file, "Test: %s\n", result.Test.Name)
		fmt.Fprintf(file, "Status: %s\n", map[bool]string{true: "PASSED", false: "FAILED"}[result.Success])
		fmt.Fprintf(file, "Transaction File: %s\n", result.Test.TransactionFile)
		fmt.Fprintf(file, "Statement Files: %v\n", result.Test.StatementFiles)
		
		fmt.Fprintf(file, "\nResults:\n")
		fmt.Fprintf(file, "  Processing Time: %v\n", result.ProcessingTime)
		fmt.Fprintf(file, "  Matches Found: %d (expected: %d)\n", result.ActualMatches, result.Test.ExpectedMatches)
		fmt.Fprintf(file, "  Unmatched: %d (expected: %d)\n", result.ActualUnmatched, result.Test.ExpectedUnmatched)
		fmt.Fprintf(file, "  Match Rate: %.1f%% (expected: %.1f%%)\n", result.ActualMatchRate, result.Test.ExpectedMatchRate)

		if len(result.MatchTypes) > 0 {
			fmt.Fprintf(file, "\nMatch Type Breakdown:\n")
			for matchType, count := range result.MatchTypes {
				fmt.Fprintf(file, "  %s: %d\n", matchType.String(), count)
			}
		}

		fmt.Fprintf(file, "\nPerformance Metrics:\n")
		fmt.Fprintf(file, "  Load Time: %v\n", result.PerformanceMetrics.LoadTime)
		fmt.Fprintf(file, "  Index Time: %v\n", result.PerformanceMetrics.IndexTime)
		fmt.Fprintf(file, "  Matching Time: %v\n", result.PerformanceMetrics.MatchingTime)
		fmt.Fprintf(file, "  Total Time: %v\n", result.PerformanceMetrics.TotalTime)
		fmt.Fprintf(file, "  Transaction Rate: %.1f tx/sec\n", result.PerformanceMetrics.TransactionRate)

		if len(result.Errors) > 0 {
			fmt.Fprintf(file, "\nErrors:\n")
			for _, err := range result.Errors {
				fmt.Fprintf(file, "  - %s\n", err)
			}
		}

		if len(result.Warnings) > 0 {
			fmt.Fprintf(file, "\nWarnings:\n")
			for _, warning := range result.Warnings {
				fmt.Fprintf(file, "  - %s\n", warning)
			}
		}

		fmt.Fprintf(file, "\n%s\n\n", "="*80)
	}

	return nil
}