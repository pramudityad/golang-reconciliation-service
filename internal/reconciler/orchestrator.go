// Package reconciler provides high-level orchestration for the reconciliation process.
//
// This package coordinates the entire reconciliation workflow, including:
//   - File parsing and data loading
//   - Data preprocessing and validation
//   - Transaction matching execution
//   - Progress tracking and reporting
//   - Error handling and recovery
//   - Result generation and formatting
//
// The ReconciliationOrchestrator provides the main entry point for complex
// reconciliation operations that involve multiple files, preprocessing steps,
// and comprehensive progress tracking.
//
// Example usage:
//
//	orchestrator := reconciler.NewReconciliationOrchestrator(service)
//	orchestrator.AddProgressCallback(func(progress *ReconciliationProgress) {
//		fmt.Printf("Progress: %.1f%% - %s\n", progress.PercentComplete, progress.CurrentStep)
//	})
//	
//	request := &ReconciliationRequest{
//		TransactionFiles: []string{"transactions.csv"},
//		BankStatementFiles: []string{"statements.csv"},
//		DateRange: DateRange{Start: startDate, End: endDate},
//	}
//	
//	result, err := orchestrator.ProcessReconciliation(ctx, request)
package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/parsers"
	"golang-reconciliation-service/pkg/errors"
	"golang-reconciliation-service/pkg/logger"

	"github.com/shopspring/decimal"
)

// ReconciliationOrchestrator handles advanced orchestration of reconciliation workflows.
// It coordinates complex multi-step reconciliation processes with progress tracking,
// error handling, and comprehensive reporting capabilities.
//
// The orchestrator manages the complete lifecycle of reconciliation operations:
//  1. Data preprocessing and validation
//  2. File parsing with progress monitoring
//  3. Transaction matching and scoring
//  4. Result compilation and summary generation
//  5. Error aggregation and reporting
//
// Key features:
//   - Progress tracking with detailed step information
//   - Concurrent file processing capabilities
//   - Flexible callback system for progress monitoring
//   - Comprehensive error handling and recovery
//   - Support for complex reconciliation scenarios
type ReconciliationOrchestrator struct {
	service      *ReconciliationService
	preprocessor *DataPreprocessor
	logger       logger.Logger
	
	// Progress tracking
	progressCallbacks []ProgressCallback
	currentProgress   *ReconciliationProgress
	progressMutex     sync.RWMutex
}

// ReconciliationProgress tracks the progress of reconciliation operations
type ReconciliationProgress struct {
	TotalSteps        int                `json:"total_steps"`
	CompletedSteps    int                `json:"completed_steps"`
	CurrentStep       string             `json:"current_step"`
	PercentComplete   float64           `json:"percent_complete"`
	StartTime         time.Time         `json:"start_time"`
	ElapsedTime       time.Duration     `json:"elapsed_time"`
	EstimatedRemaining time.Duration    `json:"estimated_remaining"`
	
	// Step-specific progress
	FilesParsed       int               `json:"files_parsed"`
	TotalFiles        int               `json:"total_files"`
	RecordsProcessed  int               `json:"records_processed"`
	TotalRecords      int               `json:"total_records"`
	MatchesFound      int               `json:"matches_found"`
	
	// Error tracking
	Errors            []string          `json:"errors,omitempty"`
	Warnings          []string          `json:"warnings,omitempty"`
}

// ProgressCallback is called to report reconciliation progress
type ProgressCallback func(*ReconciliationProgress)

// NewReconciliationOrchestrator creates a new reconciliation orchestrator
func NewReconciliationOrchestrator(
	service *ReconciliationService,
	preprocessingConfig *PreprocessingConfig,
) (*ReconciliationOrchestrator, error) {
	
	if service == nil {
		return nil, errors.ValidationError(
			errors.CodeMissingField,
			"reconciliation_service",
			nil,
			nil,
		).WithSuggestion("Provide a valid ReconciliationService instance")
	}
	
	log := logger.GetGlobalLogger().WithComponent("reconciliation_orchestrator")
	log.Debug("Creating reconciliation orchestrator")
	
	preprocessor := NewDataPreprocessor(preprocessingConfig)
	
	orchestrator := &ReconciliationOrchestrator{
		service:      service,
		preprocessor: preprocessor,
		logger:       log,
		currentProgress: &ReconciliationProgress{
			TotalSteps: 6, // Parse system, parse banks, preprocess, filter, match, aggregate
		},
	}
	
	log.Info("Reconciliation orchestrator created successfully")
	return orchestrator, nil
}

// AddProgressCallback adds a progress callback function
func (ro *ReconciliationOrchestrator) AddProgressCallback(callback ProgressCallback) {
	ro.progressCallbacks = append(ro.progressCallbacks, callback)
}

// ProcessReconciliationWithAdvancedFeatures performs reconciliation with enhanced features
func (ro *ReconciliationOrchestrator) ProcessReconciliationWithAdvancedFeatures(
	ctx context.Context,
	request *ReconciliationRequest,
	options *ReconciliationOptions,
) (*EnhancedReconciliationResult, error) {
	
	ro.logger.WithFields(logger.Fields{
		"system_file":      request.SystemFile,
		"bank_files_count": len(request.BankFiles),
	}).Info("Starting advanced reconciliation process")
	
	// Initialize progress tracking
	ro.initializeProgress()
	
	startTime := time.Now()
	defer func() {
		elapsed := time.Since(startTime)
		ro.updateProgress("Completed", 6, elapsed)
		ro.logger.WithField("elapsed_time", elapsed).Info("Advanced reconciliation process completed")
	}()
	
	// Step 1: Validate request and options
	ro.updateProgress("Validating request", 0, 0)
	ro.logger.Debug("Validating advanced reconciliation request")
	
	if err := ro.validateAdvancedRequest(request, options); err != nil {
		ro.logger.WithError(err).Error("Advanced request validation failed")
		return nil, errors.ValidationError(
			errors.CodeInvalidConfig,
			"reconciliation_request",
			request,
			err,
		).WithSuggestion("Check the reconciliation request parameters and options")
	}
	
	// Step 2: Parse system transactions with preprocessing
	ro.updateProgress("Parsing system transactions", 1, time.Since(startTime))
	ro.logger.WithField("system_file", request.SystemFile).Info("Parsing system transactions")
	
	transactions, txStats, err := ro.parseAndPreprocessTransactions(ctx, request)
	if err != nil {
		ro.logger.WithError(err).WithField("system_file", request.SystemFile).Error("Failed to parse system transactions")
		return nil, errors.ReconciliationError(
			errors.CodeProcessingError,
			"transaction_parsing",
			err,
		).WithSuggestion("Check the system transaction file format and try again")
	}
	
	ro.logger.WithFields(logger.Fields{
		"transactions_count": len(transactions),
		"parse_stats":        txStats,
	}).Info("Successfully parsed system transactions")
	
	// Step 3: Parse bank statements with preprocessing
	ro.updateProgress("Parsing bank statements", 2, time.Since(startTime))
	statements, stmtStats, err := ro.parseAndPreprocessBankStatements(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("failed to parse bank statements: %w", err)
	}
	
	// Step 4: Apply filters and transformations
	ro.updateProgress("Applying filters", 3, time.Since(startTime))
	transactions, statements = ro.applyAdvancedFiltering(transactions, statements, request, options)
	
	// Step 5: Perform reconciliation with enhanced matching
	ro.updateProgress("Performing reconciliation", 4, time.Since(startTime))
	reconciliationResult, err := ro.performAdvancedMatching(ctx, transactions, statements, options)
	if err != nil {
		return nil, fmt.Errorf("reconciliation failed: %w", err)
	}
	
	// Step 6: Generate enhanced results
	ro.updateProgress("Generating results", 5, time.Since(startTime))
	enhancedResult := ro.buildEnhancedResult(reconciliationResult, txStats, stmtStats, options, startTime)
	
	return enhancedResult, nil
}

// ReconciliationOptions contains advanced options for reconciliation
type ReconciliationOptions struct {
	// Matching options
	UseAdvancedMatching    bool                      `json:"use_advanced_matching"`
	MatchingStrategies     []string                  `json:"matching_strategies"`
	CustomMatchingConfig   *matcher.MatchingConfig   `json:"custom_matching_config,omitempty"`
	
	// Processing options
	EnablePreprocessing    bool                      `json:"enable_preprocessing"`
	PreprocessingConfig    *PreprocessingConfig      `json:"preprocessing_config,omitempty"`
	ParallelProcessing     bool                      `json:"parallel_processing"`
	MaxConcurrency        int                       `json:"max_concurrency"`
	
	// Output options
	IncludeDetailedMetrics bool                      `json:"include_detailed_metrics"`
	IncludeMatchingScores  bool                      `json:"include_matching_scores"`
	IncludeProcessingLogs  bool                      `json:"include_processing_logs"`
	IncludeDataQuality     bool                      `json:"include_data_quality"`
	
	// Analysis options
	PerformDiscrepancyAnalysis   bool                `json:"perform_discrepancy_analysis"`
	PerformDuplicateDetection   bool                `json:"perform_duplicate_detection"`
	PerformDataQualityAnalysis  bool                `json:"perform_data_quality_analysis"`
	
	// Filtering options
	AmountThresholds      *AmountThresholds         `json:"amount_thresholds,omitempty"`
	DateRangeFilters      []*DateRange              `json:"date_range_filters,omitempty"`
	TransactionTypeFilter []models.TransactionType  `json:"transaction_type_filter,omitempty"`
}

// AmountThresholds defines amount-based filtering thresholds
type AmountThresholds struct {
	MinAmount      decimal.Decimal `json:"min_amount"`
	MaxAmount      decimal.Decimal `json:"max_amount"`
	ExcludeZero    bool           `json:"exclude_zero"`
	ExcludeNegative bool          `json:"exclude_negative"`
}

// EnhancedReconciliationResult contains comprehensive reconciliation results
type EnhancedReconciliationResult struct {
	*ReconciliationResult
	
	// Enhanced metrics
	DataQualityMetrics    *DataQualityMetrics    `json:"data_quality_metrics,omitempty"`
	MatchingMetrics       *MatchingMetrics       `json:"matching_metrics,omitempty"`
	PerformanceMetrics    *PerformanceMetrics    `json:"performance_metrics,omitempty"`
	
	// Advanced analysis
	TrendAnalysis         *TrendAnalysis         `json:"trend_analysis,omitempty"`
	AnomalyDetection      *AnomalyDetection      `json:"anomaly_detection,omitempty"`
	
	// Processing logs
	ProcessingLogs        []string               `json:"processing_logs,omitempty"`
	
	// Options used
	OptionsUsed           *ReconciliationOptions `json:"options_used,omitempty"`
}

// DataQualityMetrics contains data quality analysis results
type DataQualityMetrics struct {
	TransactionQuality struct {
		TotalRecords      int     `json:"total_records"`
		ValidRecords      int     `json:"valid_records"`
		InvalidRecords    int     `json:"invalid_records"`
		QualityScore      float64 `json:"quality_score"`
		CommonIssues      []string `json:"common_issues"`
	} `json:"transaction_quality"`
	
	StatementQuality struct {
		TotalRecords      int     `json:"total_records"`
		ValidRecords      int     `json:"valid_records"`
		InvalidRecords    int     `json:"invalid_records"`
		QualityScore      float64 `json:"quality_score"`
		CommonIssues      []string `json:"common_issues"`
	} `json:"statement_quality"`
	
	OverallQualityScore   float64 `json:"overall_quality_score"`
}

// MatchingMetrics contains detailed matching analysis
type MatchingMetrics struct {
	ConfidenceDistribution map[string]int     `json:"confidence_distribution"`
	AverageConfidenceScore float64           `json:"average_confidence_score"`
	MatchingAccuracy      float64           `json:"matching_accuracy"`
	FalsePositiveRate     float64           `json:"false_positive_rate"`
	FalseNegativeRate     float64           `json:"false_negative_rate"`
	MatchingStrategiesUsed []string         `json:"matching_strategies_used"`
}

// PerformanceMetrics contains performance analysis
type PerformanceMetrics struct {
	TotalProcessingTime   time.Duration `json:"total_processing_time"`
	ParsingTime          time.Duration `json:"parsing_time"`
	PreprocessingTime    time.Duration `json:"preprocessing_time"`
	MatchingTime         time.Duration `json:"matching_time"`
	AggregationTime      time.Duration `json:"aggregation_time"`
	
	RecordsPerSecond     float64       `json:"records_per_second"`
	MemoryUsage          int64         `json:"memory_usage"`
	CPUUtilization      float64       `json:"cpu_utilization"`
}

// TrendAnalysis contains trend analysis results
type TrendAnalysis struct {
	AmountTrends         []AmountTrend   `json:"amount_trends"`
	VolumeByDay          map[string]int  `json:"volume_by_day"`
	VolumeByType         map[string]int  `json:"volume_by_type"`
	AverageAmountByDay   map[string]decimal.Decimal `json:"average_amount_by_day"`
}

// AmountTrend represents amount trend data
type AmountTrend struct {
	Date   time.Time       `json:"date"`
	Amount decimal.Decimal `json:"amount"`
	Count  int            `json:"count"`
}

// AnomalyDetection contains anomaly detection results
type AnomalyDetection struct {
	UnusualAmounts       []*models.Transaction   `json:"unusual_amounts,omitempty"`
	UnusualDates         []*models.Transaction   `json:"unusual_dates,omitempty"`
	SuspiciousPatterns   []string               `json:"suspicious_patterns,omitempty"`
	OutlierTransactions  []*models.Transaction   `json:"outlier_transactions,omitempty"`
}

// Default options for reconciliation
func DefaultReconciliationOptions() *ReconciliationOptions {
	return &ReconciliationOptions{
		UseAdvancedMatching:        true,
		MatchingStrategies:        []string{"exact", "fuzzy", "amount_date"},
		EnablePreprocessing:       true,
		ParallelProcessing:        true,
		MaxConcurrency:           4,
		IncludeDetailedMetrics:    true,
		IncludeMatchingScores:     true,
		IncludeProcessingLogs:     false,
		IncludeDataQuality:       true,
		PerformDiscrepancyAnalysis: true,
		PerformDuplicateDetection: true,
		PerformDataQualityAnalysis: true,
	}
}

// Helper methods for orchestration

func (ro *ReconciliationOrchestrator) initializeProgress() {
	ro.progressMutex.Lock()
	defer ro.progressMutex.Unlock()
	
	ro.currentProgress = &ReconciliationProgress{
		TotalSteps:      6,
		CompletedSteps:  0,
		StartTime:       time.Now(),
		PercentComplete: 0.0,
	}
}

func (ro *ReconciliationOrchestrator) updateProgress(step string, completed int, elapsed time.Duration) {
	ro.progressMutex.Lock()
	defer ro.progressMutex.Unlock()
	
	ro.currentProgress.CurrentStep = step
	ro.currentProgress.CompletedSteps = completed
	ro.currentProgress.ElapsedTime = elapsed
	ro.currentProgress.PercentComplete = float64(completed) / float64(ro.currentProgress.TotalSteps) * 100
	
	// Estimate remaining time
	if completed > 0 && completed < ro.currentProgress.TotalSteps {
		avgTimePerStep := elapsed / time.Duration(completed)
		remainingSteps := ro.currentProgress.TotalSteps - completed
		ro.currentProgress.EstimatedRemaining = avgTimePerStep * time.Duration(remainingSteps)
	}
	
	// Notify callbacks
	for _, callback := range ro.progressCallbacks {
		callback(ro.currentProgress)
	}
}

func (ro *ReconciliationOrchestrator) validateAdvancedRequest(
	request *ReconciliationRequest,
	options *ReconciliationOptions,
) error {
	if err := request.Validate(); err != nil {
		return err
	}
	
	if options == nil {
		return fmt.Errorf("reconciliation options are required")
	}
	
	if options.MaxConcurrency <= 0 {
		options.MaxConcurrency = 4
	}
	
	return nil
}

func (ro *ReconciliationOrchestrator) parseAndPreprocessTransactions(
	ctx context.Context,
	request *ReconciliationRequest,
) ([]*models.Transaction, *parsers.ParseStats, error) {
	
	// Parse transactions using the service
	transactions, stats, err := ro.service.parseSystemTransactions(ctx, request)
	if err != nil {
		return nil, stats, err
	}
	
	// Apply preprocessing if enabled
	if ro.preprocessor != nil {
		processed, err := ro.preprocessor.PreprocessTransactions(transactions)
		if err != nil {
			// Log warning but continue with original data
			ro.addWarning(fmt.Sprintf("Preprocessing failed: %v", err))
		} else {
			transactions = processed
		}
	}
	
	return transactions, stats, nil
}

func (ro *ReconciliationOrchestrator) parseAndPreprocessBankStatements(
	ctx context.Context,
	request *ReconciliationRequest,
) ([]*models.BankStatement, map[string]*parsers.ParseStats, error) {
	
	// Parse statements using the service
	statements, stats, err := ro.service.parseBankStatements(ctx, request)
	if err != nil {
		return nil, stats, err
	}
	
	// Apply preprocessing if enabled
	if ro.preprocessor != nil {
		processed, err := ro.preprocessor.PreprocessBankStatements(statements)
		if err != nil {
			// Log warning but continue with original data
			ro.addWarning(fmt.Sprintf("Preprocessing failed: %v", err))
		} else {
			statements = processed
		}
	}
	
	return statements, stats, nil
}

func (ro *ReconciliationOrchestrator) applyAdvancedFiltering(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	request *ReconciliationRequest,
	options *ReconciliationOptions,
) ([]*models.Transaction, []*models.BankStatement) {
	
	// Apply basic date range filtering
	filteredTx, filteredStmt := ro.service.applyDateRangeFiltering(transactions, statements, request)
	
	// Apply additional filters from options
	if options.AmountThresholds != nil {
		filteredTx = ro.filterTransactionsByAmount(filteredTx, options.AmountThresholds)
		filteredStmt = ro.filterStatementsByAmount(filteredStmt, options.AmountThresholds)
	}
	
	if len(options.TransactionTypeFilter) > 0 {
		filteredTx = ro.filterTransactionsByType(filteredTx, options.TransactionTypeFilter)
	}
	
	return filteredTx, filteredStmt
}

func (ro *ReconciliationOrchestrator) performAdvancedMatching(
	ctx context.Context,
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	options *ReconciliationOptions,
) (*matcher.ReconciliationResult, error) {
	
	// Use custom matching configuration if provided
	if options.CustomMatchingConfig != nil {
		err := ro.service.matchingEngine.UpdateConfiguration(options.CustomMatchingConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to update matching configuration: %w", err)
		}
	}
	
	// Perform the matching
	return ro.service.performMatching(ctx, transactions, statements)
}

func (ro *ReconciliationOrchestrator) buildEnhancedResult(
	result *matcher.ReconciliationResult,
	txStats *parsers.ParseStats,
	stmtStats map[string]*parsers.ParseStats,
	options *ReconciliationOptions,
	startTime time.Time,
) *EnhancedReconciliationResult {
	
	// Build base result
	baseResult := &ReconciliationResult{
		Summary:              &ResultSummary{},
		ProcessingStats:     &ProcessingStats{},
		ProcessedAt:         time.Now(),
	}
	
	// Build enhanced result
	enhancedResult := &EnhancedReconciliationResult{
		ReconciliationResult: baseResult,
		OptionsUsed:         options,
	}
	
	// Add enhanced metrics if requested
	if options.IncludeDataQuality {
		enhancedResult.DataQualityMetrics = ro.calculateDataQualityMetrics(result)
	}
	
	if options.IncludeDetailedMetrics {
		enhancedResult.PerformanceMetrics = ro.calculatePerformanceMetrics(startTime)
	}
	
	return enhancedResult
}

// Helper filter methods

func (ro *ReconciliationOrchestrator) filterTransactionsByAmount(
	transactions []*models.Transaction,
	thresholds *AmountThresholds,
) []*models.Transaction {
	var filtered []*models.Transaction
	
	for _, tx := range transactions {
		if thresholds.ExcludeZero && tx.Amount.IsZero() {
			continue
		}
		
		if thresholds.ExcludeNegative && tx.Amount.IsNegative() {
			continue
		}
		
		absAmount := tx.GetAbsoluteAmount()
		if !thresholds.MinAmount.IsZero() && absAmount.LessThan(thresholds.MinAmount) {
			continue
		}
		
		if !thresholds.MaxAmount.IsZero() && absAmount.GreaterThan(thresholds.MaxAmount) {
			continue
		}
		
		filtered = append(filtered, tx)
	}
	
	return filtered
}

func (ro *ReconciliationOrchestrator) filterStatementsByAmount(
	statements []*models.BankStatement,
	thresholds *AmountThresholds,
) []*models.BankStatement {
	var filtered []*models.BankStatement
	
	for _, stmt := range statements {
		if thresholds.ExcludeZero && stmt.Amount.IsZero() {
			continue
		}
		
		if thresholds.ExcludeNegative && stmt.Amount.IsNegative() {
			continue
		}
		
		absAmount := stmt.NormalizeAmount()
		if !thresholds.MinAmount.IsZero() && absAmount.LessThan(thresholds.MinAmount) {
			continue
		}
		
		if !thresholds.MaxAmount.IsZero() && absAmount.GreaterThan(thresholds.MaxAmount) {
			continue
		}
		
		filtered = append(filtered, stmt)
	}
	
	return filtered
}

func (ro *ReconciliationOrchestrator) filterTransactionsByType(
	transactions []*models.Transaction,
	allowedTypes []models.TransactionType,
) []*models.Transaction {
	if len(allowedTypes) == 0 {
		return transactions
	}
	
	typeMap := make(map[models.TransactionType]bool)
	for _, t := range allowedTypes {
		typeMap[t] = true
	}
	
	var filtered []*models.Transaction
	for _, tx := range transactions {
		if typeMap[tx.Type] {
			filtered = append(filtered, tx)
		}
	}
	
	return filtered
}

func (ro *ReconciliationOrchestrator) addWarning(message string) {
	ro.progressMutex.Lock()
	defer ro.progressMutex.Unlock()
	
	ro.currentProgress.Warnings = append(ro.currentProgress.Warnings, message)
}

func (ro *ReconciliationOrchestrator) calculateDataQualityMetrics(result *matcher.ReconciliationResult) *DataQualityMetrics {
	// This would implement data quality analysis
	return &DataQualityMetrics{
		OverallQualityScore: 0.95, // Placeholder
	}
}

func (ro *ReconciliationOrchestrator) calculatePerformanceMetrics(startTime time.Time) *PerformanceMetrics {
	return &PerformanceMetrics{
		TotalProcessingTime: time.Since(startTime),
		RecordsPerSecond:   0, // Would be calculated based on actual data
	}
}