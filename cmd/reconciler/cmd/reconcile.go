package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang-reconciliation-service/cmd/reconciler/config"
	"golang-reconciliation-service/internal/parsers"
	"golang-reconciliation-service/internal/reconciler"
	"golang-reconciliation-service/internal/reporter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Flags for the reconcile command
var (
	systemFile      string
	bankFiles       []string
	outputFormat    string
	outputFile      string
	startDate       string
	endDate         string
	dateTolerance   int
	amountTolerance float64
	showProgress    bool
	
	// Edge case handling flags
	enableEdgeCases         bool
	enableDuplicateDetection bool
	enableTimezoneNorm      bool
	enableSameDayMatching   bool
	enablePartialMatching   bool
	enableCurrencyConversion bool
)

// reconcileCmd represents the reconcile command
var reconcileCmd = &cobra.Command{
	Use:   "reconcile",
	Short: "Reconcile system transactions with bank statements",
	Long: `Reconcile compares system transaction records with bank statement records
to identify matches, discrepancies, and unmatched entries.

This command requires:
- A system transaction file (CSV format)
- One or more bank statement files (CSV format)

Examples:
  # Basic reconciliation
  reconciler reconcile --system-file transactions.csv --bank-files statements.csv
  
  # Multiple bank files with date filtering
  reconciler reconcile --system-file tx.csv --bank-files bank1.csv,bank2.csv \
    --start-date 2024-01-01 --end-date 2024-01-31
  
  # Custom output format and tolerances
  reconciler reconcile --system-file tx.csv --bank-files stmt.csv \
    --output-format json --output-file report.json \
    --date-tolerance 2 --amount-tolerance 0.1
  
  # With progress indicators
  reconciler reconcile --system-file tx.csv --bank-files stmt.csv --progress
  
  # Disable edge case handling for faster processing
  reconciler reconcile --system-file tx.csv --bank-files stmt.csv --enable-edge-cases=false
  
  # Enable partial matching for complex scenarios
  reconciler reconcile --system-file tx.csv --bank-files stmt.csv --partial-matching`,
	
	PreRunE: validateReconcileFlags,
	RunE:    runReconcile,
}

func init() {
	rootCmd.AddCommand(reconcileCmd)

	// Required flags
	reconcileCmd.Flags().StringVarP(&systemFile, "system-file", "s", "", "path to system transaction CSV file (required)")
	reconcileCmd.Flags().StringSliceVarP(&bankFiles, "bank-files", "b", []string{}, "comma-separated paths to bank statement CSV files (required)")
	
	// Output flags
	reconcileCmd.Flags().StringVarP(&outputFormat, "output-format", "f", "console", "output format: console, json, csv")
	reconcileCmd.Flags().StringVarP(&outputFile, "output-file", "o", "", "output file path (default: stdout)")
	
	// Date filtering flags
	reconcileCmd.Flags().StringVar(&startDate, "start-date", "", "filter start date (YYYY-MM-DD)")
	reconcileCmd.Flags().StringVar(&endDate, "end-date", "", "filter end date (YYYY-MM-DD)")
	
	// Matching configuration flags
	reconcileCmd.Flags().IntVarP(&dateTolerance, "date-tolerance", "d", 1, "date matching tolerance in days")
	reconcileCmd.Flags().Float64VarP(&amountTolerance, "amount-tolerance", "a", 0.0, "amount tolerance percentage (0.0-100.0)")
	
	// UI flags
	reconcileCmd.Flags().BoolVar(&showProgress, "progress", false, "show progress indicators")
	
	// Edge case handling flags
	reconcileCmd.Flags().BoolVar(&enableEdgeCases, "enable-edge-cases", true, "enable advanced edge case handling")
	reconcileCmd.Flags().BoolVar(&enableDuplicateDetection, "detect-duplicates", true, "detect duplicate transactions")
	reconcileCmd.Flags().BoolVar(&enableTimezoneNorm, "normalize-timezones", true, "normalize timezone differences")
	reconcileCmd.Flags().BoolVar(&enableSameDayMatching, "same-day-matching", true, "handle same-day transaction ambiguity")
	reconcileCmd.Flags().BoolVar(&enablePartialMatching, "partial-matching", false, "enable partial amount matching (resource intensive)")
	reconcileCmd.Flags().BoolVar(&enableCurrencyConversion, "currency-conversion", false, "enable currency conversion handling")

	// Mark required flags
	reconcileCmd.MarkFlagRequired("system-file")
	reconcileCmd.MarkFlagRequired("bank-files")

	// Bind flags to viper
	viper.BindPFlag("system-file", reconcileCmd.Flags().Lookup("system-file"))
	viper.BindPFlag("bank-files", reconcileCmd.Flags().Lookup("bank-files"))
	viper.BindPFlag("output-format", reconcileCmd.Flags().Lookup("output-format"))
	viper.BindPFlag("output-file", reconcileCmd.Flags().Lookup("output-file"))
	viper.BindPFlag("start-date", reconcileCmd.Flags().Lookup("start-date"))
	viper.BindPFlag("end-date", reconcileCmd.Flags().Lookup("end-date"))
	viper.BindPFlag("date-tolerance", reconcileCmd.Flags().Lookup("date-tolerance"))
	viper.BindPFlag("amount-tolerance", reconcileCmd.Flags().Lookup("amount-tolerance"))
	viper.BindPFlag("progress", reconcileCmd.Flags().Lookup("progress"))
	
	// Bind edge case handling flags
	viper.BindPFlag("enable-edge-cases", reconcileCmd.Flags().Lookup("enable-edge-cases"))
	viper.BindPFlag("detect-duplicates", reconcileCmd.Flags().Lookup("detect-duplicates"))
	viper.BindPFlag("normalize-timezones", reconcileCmd.Flags().Lookup("normalize-timezones"))
	viper.BindPFlag("same-day-matching", reconcileCmd.Flags().Lookup("same-day-matching"))
	viper.BindPFlag("partial-matching", reconcileCmd.Flags().Lookup("partial-matching"))
	viper.BindPFlag("currency-conversion", reconcileCmd.Flags().Lookup("currency-conversion"))
}

func validateReconcileFlags(cmd *cobra.Command, args []string) error {
	// Get values from viper (allows override from config file)
	systemFile = viper.GetString("system-file")
	bankFiles = viper.GetStringSlice("bank-files")
	outputFormat = viper.GetString("output-format")
	outputFile = viper.GetString("output-file")
	startDate = viper.GetString("start-date")
	endDate = viper.GetString("end-date")
	dateTolerance = viper.GetInt("date-tolerance")
	amountTolerance = viper.GetFloat64("amount-tolerance")
	showProgress = viper.GetBool("progress")

	// Validate required flags
	if systemFile == "" {
		return fmt.Errorf("system-file is required")
	}
	if len(bankFiles) == 0 {
		return fmt.Errorf("at least one bank-file is required")
	}

	// Validate file existence
	if err := validateFileExists(systemFile, "system transaction file"); err != nil {
		return err
	}

	for i, bankFile := range bankFiles {
		if err := validateFileExists(bankFile, fmt.Sprintf("bank file %d", i+1)); err != nil {
			return err
		}
	}

	// Validate output format
	validFormats := map[string]bool{"console": true, "json": true, "csv": true}
	if !validFormats[outputFormat] {
		return fmt.Errorf("invalid output format '%s'. Valid formats: console, json, csv", outputFormat)
	}

	// Validate dates
	if startDate != "" {
		if _, err := time.Parse("2006-01-02", startDate); err != nil {
			return fmt.Errorf("invalid start date format. Use YYYY-MM-DD: %w", err)
		}
	}
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil {
			return fmt.Errorf("invalid end date format. Use YYYY-MM-DD: %w", err)
		}
	}

	// Validate date range
	if startDate != "" && endDate != "" {
		start, _ := time.Parse("2006-01-02", startDate)
		end, _ := time.Parse("2006-01-02", endDate)
		if start.After(end) {
			return fmt.Errorf("start date cannot be after end date")
		}
	}

	// Validate tolerances
	if dateTolerance < 0 {
		return fmt.Errorf("date tolerance cannot be negative")
	}
	if amountTolerance < 0.0 || amountTolerance > 100.0 {
		return fmt.Errorf("amount tolerance must be between 0.0 and 100.0")
	}

	// Validate output file directory exists if specified
	if outputFile != "" {
		dir := filepath.Dir(outputFile)
		if dir != "." {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				return fmt.Errorf("output directory does not exist: %s", dir)
			}
		}
	}

	return nil
}

func validateFileExists(filePath, description string) error {
	if filePath == "" {
		return fmt.Errorf("%s path cannot be empty", description)
	}

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist: %s", description, filePath)
	}
	if err != nil {
		return fmt.Errorf("error accessing %s: %w", description, err)
	}

	if info.IsDir() {
		return fmt.Errorf("%s is a directory, expected a file: %s", description, filePath)
	}

	// Check if file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s is not readable: %w", description, err)
	}
	file.Close()

	return nil
}

func runReconcile(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "Starting reconciliation...\n")
		fmt.Fprintf(os.Stderr, "System file: %s\n", systemFile)
		fmt.Fprintf(os.Stderr, "Bank files: %s\n", strings.Join(bankFiles, ", "))
		fmt.Fprintf(os.Stderr, "Output format: %s\n", outputFormat)
		if outputFile != "" {
			fmt.Fprintf(os.Stderr, "Output file: %s\n", outputFile)
		}
	}

	// Create configurations
	transactionConfig, err := config.CreateTransactionParserConfig()
	if err != nil {
		return fmt.Errorf("failed to create transaction parser config: %w", err)
	}

	bankConfigs, err := config.CreateBankConfigs(bankFiles)
	if err != nil {
		return fmt.Errorf("failed to create bank configs: %w", err)
	}

	matchingConfig := config.CreateMatchingConfig(dateTolerance, amountTolerance)
	reconcilerConfig := config.CreateReconcilerConfig(showProgress)

	// Create reconciliation service
	// Note: We need to create a service for each bank file since the current API expects one bank config
	// For now, we'll use the first bank file's config as the default
	var firstBankConfig *parsers.BankConfig
	for _, bankConfig := range bankConfigs {
		firstBankConfig = bankConfig
		break
	}

	service, err := reconciler.NewReconciliationService(
		transactionConfig,
		firstBankConfig,
		matchingConfig,
		reconcilerConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create reconciliation service: %w", err)
	}
	
	// Create orchestrator for advanced reconciliation if edge cases are enabled
	var orchestrator *reconciler.ReconciliationOrchestrator
	if enableEdgeCases {
		orchestrator, err = reconciler.NewReconciliationOrchestrator(service, nil)
		if err != nil {
			return fmt.Errorf("failed to create reconciliation orchestrator: %w", err)
		}
		
		// Add progress callback if requested
		if showProgress {
			orchestrator.AddProgressCallback(func(progress *reconciler.ReconciliationProgress) {
				fmt.Fprintf(os.Stderr, "\r[%d/%d] %s (%.1f%% complete)", 
					progress.CompletedSteps, progress.TotalSteps, 
					progress.CurrentStep, progress.PercentComplete)
			})
		}
	}

	// Parse date range
	var startTime, endTime *time.Time
	if startDate != "" {
		t, _ := time.Parse("2006-01-02", startDate)
		startTime = &t
	}
	if endDate != "" {
		t, _ := time.Parse("2006-01-02", endDate)
		// Set to end of day
		t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		endTime = &t
	}

	// Create reconciliation request
	request := &reconciler.ReconciliationRequest{
		SystemFile:        systemFile,
		BankFiles:         bankFiles,
		StartDate:         startTime,
		EndDate:           endTime,
		TransactionConfig: transactionConfig,
		BankConfigs:       bankConfigs,
	}

	// Show progress if requested
	if showProgress {
		fmt.Fprintf(os.Stderr, "Processing reconciliation...\n")
	}

	// Perform reconciliation with edge case handling if enabled
	var result *reconciler.ReconciliationResult
	if enableEdgeCases && orchestrator != nil {
		// Create reconciliation options based on flags
		options := &reconciler.ReconciliationOptions{
			UseAdvancedMatching:           true,
			MatchingStrategies:           []string{"exact", "fuzzy", "amount_date"},
			EnablePreprocessing:          true,
			ParallelProcessing:           true,
			MaxConcurrency:              4,
			IncludeDetailedMetrics:       true,
			IncludeMatchingScores:        true,
			IncludeProcessingLogs:        viper.GetBool("verbose"),
			IncludeDataQuality:          true,
			PerformDiscrepancyAnalysis:   true,
			PerformDuplicateDetection:    enableDuplicateDetection,
			PerformDataQualityAnalysis:   true,
			EnableEdgeCaseHandling:       enableEdgeCases,
			EnableTimezoneNormalization:  enableTimezoneNorm,
			EnableSameDayMatching:        enableSameDayMatching,
			EnablePartialMatching:        enablePartialMatching,
			EnableCurrencyConversion:     enableCurrencyConversion,
		}
		
		enhancedResult, err := orchestrator.ProcessReconciliationWithAdvancedFeatures(ctx, request, options)
		if err != nil {
			return fmt.Errorf("advanced reconciliation failed: %w", err)
		}
		result = enhancedResult.ReconciliationResult
		
		// Show edge case results if verbose
		if viper.GetBool("verbose") && enhancedResult.EdgeCaseResults != nil {
			fmt.Fprintf(os.Stderr, "\nEdge case handling results:\n")
			fmt.Fprintf(os.Stderr, "  Duplicate groups detected: %d\n", len(enhancedResult.EdgeCaseResults.DuplicateGroups))
			fmt.Fprintf(os.Stderr, "  Same-day ambiguities: %d\n", len(enhancedResult.EdgeCaseResults.SameDayMatches))
			fmt.Fprintf(os.Stderr, "  Partial matches found: %d\n", len(enhancedResult.EdgeCaseResults.PartialMatches))
			fmt.Fprintf(os.Stderr, "  Edge cases detected: %d\n", enhancedResult.EdgeCaseResults.EdgeCasesDetected)
			fmt.Fprintf(os.Stderr, "  Edge cases resolved: %d\n", enhancedResult.EdgeCaseResults.EdgeCasesResolved)
		}
		
		if showProgress {
			fmt.Fprintf(os.Stderr, "\n") // New line after progress
		}
	} else {
		// Use basic reconciliation
		basicResult, err := service.ProcessReconciliation(ctx, request)
		if err != nil {
			return fmt.Errorf("reconciliation failed: %w", err)
		}
		result = basicResult
	}

	// Generate report
	reportConfig := config.CreateReportConfig(outputFormat)
	reportGenerator, err := reporter.NewReportGenerator(reportConfig)
	if err != nil {
		return fmt.Errorf("failed to create report generator: %w", err)
	}

	// Determine output destination
	var output *os.File
	if outputFile != "" {
		output, err = os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer output.Close()
	} else {
		output = os.Stdout
	}

	// Generate report
	if err := reportGenerator.GenerateReport(result, output); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Show completion message
	if viper.GetBool("verbose") {
		fmt.Fprintf(os.Stderr, "\nReconciliation completed successfully.\n")
		fmt.Fprintf(os.Stderr, "Processed %d transactions and %d bank statements.\n",
			result.Summary.TotalTransactions, result.Summary.TotalBankStatements)
		fmt.Fprintf(os.Stderr, "Found %d matches, %d unmatched transactions, %d unmatched statements.\n",
			result.Summary.MatchedTransactions, result.Summary.UnmatchedTransactions, result.Summary.UnmatchedStatements)
		if len(result.Discrepancies) > 0 {
			fmt.Fprintf(os.Stderr, "Detected %d discrepancies.\n", len(result.Discrepancies))
		}
		fmt.Fprintf(os.Stderr, "Processing time: %v\n", result.Summary.ProcessingDuration)
	}

	return nil
}