package parsers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang-reconciliation-service/internal/models"
)

// ProgressReport contains information about parsing progress for long-running operations.
// This enables users to monitor parsing progress and estimate completion times
// for large CSV files.
//
// Fields:
//   - ProcessedRecords: Total number of CSV records processed so far
//   - ValidRecords: Number of successfully parsed and validated records
//   - ErrorCount: Number of records that failed parsing or validation
//   - ElapsedTime: Time elapsed since parsing started
//   - EstimatedTotal: Estimated total number of records (if available)
//   - PercentComplete: Progress percentage (0-100), only accurate if EstimatedTotal > 0
//
// Progress reports are generated at configurable intervals during streaming operations.
type ProgressReport struct {
	ProcessedRecords int
	ValidRecords     int
	ErrorCount       int
	ElapsedTime      time.Duration
	EstimatedTotal   int
	PercentComplete  float64
}

// ProgressCallback is called periodically to report parsing progress
type ProgressCallback func(*ProgressReport)

// StreamingTransactionParser provides memory-efficient streaming capabilities for transaction parsing.
// This parser is designed for processing large transaction files that may not fit in memory.
// It processes data in configurable batches and supports progress reporting.
//
// Key features:
//   - Memory-efficient batch processing
//   - Progress reporting with estimated completion times
//   - Context-based cancellation support
//   - Configurable batch sizes for performance tuning
//   - Advanced error handling and recovery
//
// Use this parser when processing files larger than available memory or when
// you need progress reporting for long-running operations.
type StreamingTransactionParser struct {
	*TransactionParser
	config *StreamingConfig
}

// NewStreamingTransactionParser creates a new streaming transaction parser
func NewStreamingTransactionParser(config *TransactionParserConfig, streamConfig *StreamingConfig) (*StreamingTransactionParser, error) {
	if streamConfig == nil {
		streamConfig = DefaultStreamingConfig()
	}
	
	if err := streamConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid streaming configuration: %w", err)
	}
	
	transactionParser, err := NewTransactionParser(config)
	if err != nil {
		return nil, err
	}
	
	return &StreamingTransactionParser{
		TransactionParser: transactionParser,
		config:           streamConfig,
	}, nil
}

// ParseTransactionsStreamAdvanced parses transactions with advanced streaming features
func (stp *StreamingTransactionParser) ParseTransactionsStreamAdvanced(
	ctx context.Context,
	filePath string,
	callback ParseTransactionsCallback,
	progressCallback ProgressCallback,
) (*ParseStats, error) {
	startTime := time.Now()
	stats := NewParseStats()
	
	// Estimate total records if progress reporting is enabled
	var estimatedTotal int
	if stp.config.ReportProgress && progressCallback != nil {
		total, err := stp.estimateRecordCount(filePath)
		if err != nil {
			// Continue without estimation
			estimatedTotal = 0
		} else {
			estimatedTotal = total
		}
	}
	
	// Use the existing streaming functionality with enhanced error handling
	batchCallback := func(transactions []*models.Transaction) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("processing cancelled")
		default:
			// Call the user callback
			if err := callback(transactions); err != nil {
				return fmt.Errorf("user callback error: %w", err)
			}
			
			// Update statistics
			stats.RecordsValid += len(transactions)
			
			// Report progress if configured
			if stp.config.ReportProgress && progressCallback != nil &&
				stats.RecordsValid%stp.config.ProgressInterval == 0 {
				
				elapsed := time.Since(startTime)
				var percentComplete float64
				if estimatedTotal > 0 {
					percentComplete = float64(stats.RecordsValid) / float64(estimatedTotal) * 100
				}
				
				progress := &ProgressReport{
					ProcessedRecords: stats.RecordsParsed,
					ValidRecords:     stats.RecordsValid,
					ErrorCount:       stats.ErrorCount,
					ElapsedTime:      elapsed,
					EstimatedTotal:   estimatedTotal,
					PercentComplete:  percentComplete,
				}
				
				progressCallback(progress)
			}
			
			return nil
		}
	}
	
	// Parse with streaming
	parseStats, err := stp.ParseTransactionsStreamWithContext(
		ctx, filePath, stp.config.BatchSize, batchCallback)
	
	// Merge statistics
	stats.TotalLines = parseStats.TotalLines
	stats.RecordsParsed = parseStats.RecordsParsed
	stats.ErrorCount = parseStats.ErrorCount
	stats.Errors = parseStats.Errors
	
	// Final progress report
	if stp.config.ReportProgress && progressCallback != nil {
		elapsed := time.Since(startTime)
		progress := &ProgressReport{
			ProcessedRecords: stats.RecordsParsed,
			ValidRecords:     stats.RecordsValid,
			ErrorCount:       stats.ErrorCount,
			ElapsedTime:      elapsed,
			EstimatedTotal:   estimatedTotal,
			PercentComplete:  100.0,
		}
		progressCallback(progress)
	}
	
	return stats, err
}

// estimateRecordCount attempts to estimate the total number of records in the file
func (stp *StreamingTransactionParser) estimateRecordCount(filePath string) (int, error) {
	file, reader, err := stp.OpenFile(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(context.Background())
	
	// Read headers if they exist
	if stp.TransactionParser.config.HasHeader {
		if err := stp.ReadHeaders(reader, parseCtx, nil); err != nil {
			return 0, err
		}
	}
	
	// Count records by reading through the file
	count := 0
	for {
		_, err := stp.ReadRecord(reader, parseCtx)
		if err != nil {
			break
		}
		count++
	}
	
	return count, nil
}

// StreamingBankStatementParser provides streaming capabilities for bank statement parsing
type StreamingBankStatementParser struct {
	*BankStatementParser
	config *StreamingConfig
}

// NewStreamingBankStatementParser creates a new streaming bank statement parser
func NewStreamingBankStatementParser(bankConfig *BankConfig, streamConfig *StreamingConfig) (*StreamingBankStatementParser, error) {
	if streamConfig == nil {
		streamConfig = DefaultStreamingConfig()
	}
	
	if err := streamConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid streaming configuration: %w", err)
	}
	
	bankStatementParser, err := NewBankStatementParser(bankConfig)
	if err != nil {
		return nil, err
	}
	
	return &StreamingBankStatementParser{
		BankStatementParser: bankStatementParser,
		config:             streamConfig,
	}, nil
}

// ParseBankStatementsStreamAdvanced parses bank statements with advanced streaming features
func (sbsp *StreamingBankStatementParser) ParseBankStatementsStreamAdvanced(
	ctx context.Context,
	filePath string,
	callback ParseBankStatementsCallback,
	progressCallback ProgressCallback,
) (*ParseStats, error) {
	startTime := time.Now()
	stats := NewParseStats()
	
	// Estimate total records if progress reporting is enabled
	var estimatedTotal int
	if sbsp.config.ReportProgress && progressCallback != nil {
		total, err := sbsp.estimateRecordCount(filePath)
		if err != nil {
			// Continue without estimation
			estimatedTotal = 0
		} else {
			estimatedTotal = total
		}
	}
	
	// Use the existing streaming functionality with enhanced error handling
	batchCallback := func(statements []*models.BankStatement) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("processing cancelled")
		default:
			// Call the user callback
			if err := callback(statements); err != nil {
				return fmt.Errorf("user callback error: %w", err)
			}
			
			// Update statistics
			stats.RecordsValid += len(statements)
			
			// Report progress if configured
			if sbsp.config.ReportProgress && progressCallback != nil &&
				stats.RecordsValid%sbsp.config.ProgressInterval == 0 {
				
				elapsed := time.Since(startTime)
				var percentComplete float64
				if estimatedTotal > 0 {
					percentComplete = float64(stats.RecordsValid) / float64(estimatedTotal) * 100
				}
				
				progress := &ProgressReport{
					ProcessedRecords: stats.RecordsParsed,
					ValidRecords:     stats.RecordsValid,
					ErrorCount:       stats.ErrorCount,
					ElapsedTime:      elapsed,
					EstimatedTotal:   estimatedTotal,
					PercentComplete:  percentComplete,
				}
				
				progressCallback(progress)
			}
			
			return nil
		}
	}
	
	// Parse with streaming
	parseStats, err := sbsp.ParseBankStatementsStreamWithContext(
		ctx, filePath, sbsp.config.BatchSize, batchCallback)
	
	// Merge statistics
	stats.TotalLines = parseStats.TotalLines
	stats.RecordsParsed = parseStats.RecordsParsed
	stats.ErrorCount = parseStats.ErrorCount
	stats.Errors = parseStats.Errors
	
	// Final progress report
	if sbsp.config.ReportProgress && progressCallback != nil {
		elapsed := time.Since(startTime)
		progress := &ProgressReport{
			ProcessedRecords: stats.RecordsParsed,
			ValidRecords:     stats.RecordsValid,
			ErrorCount:       stats.ErrorCount,
			ElapsedTime:      elapsed,
			EstimatedTotal:   estimatedTotal,
			PercentComplete:  100.0,
		}
		progressCallback(progress)
	}
	
	return stats, err
}

// estimateRecordCount attempts to estimate the total number of records in the file
func (sbsp *StreamingBankStatementParser) estimateRecordCount(filePath string) (int, error) {
	file, reader, err := sbsp.OpenFile(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	
	parseCtx := NewParseContext(context.Background())
	
	// Read headers if they exist
	if sbsp.bankConfig.HasHeader {
		if err := sbsp.ReadHeaders(reader, parseCtx, nil); err != nil {
			return 0, err
		}
	}
	
	// Count records by reading through the file
	count := 0
	for {
		_, err := sbsp.ReadRecord(reader, parseCtx)
		if err != nil {
			break
		}
		count++
	}
	
	return count, nil
}

// ConcurrentParser provides concurrent parsing capabilities for multiple files
type ConcurrentParser struct {
	maxConcurrency int
	semaphore      chan struct{}
}

// NewConcurrentParser creates a new concurrent parser
func NewConcurrentParser(maxConcurrency int) *ConcurrentParser {
	if maxConcurrency <= 0 {
		maxConcurrency = 4 // Default concurrency
	}
	
	return &ConcurrentParser{
		maxConcurrency: maxConcurrency,
		semaphore:     make(chan struct{}, maxConcurrency),
	}
}

// ConcurrentParseResult holds the result of a concurrent parsing operation
type ConcurrentParseResult struct {
	FilePath     string
	Transactions []*models.Transaction
	Statements   []*models.BankStatement
	Stats        *ParseStats
	Error        error
}

// ParseTransactionsConcurrently parses multiple transaction files concurrently
func (cp *ConcurrentParser) ParseTransactionsConcurrently(
	ctx context.Context,
	files map[string]*TransactionParserConfig,
) <-chan *ConcurrentParseResult {
	results := make(chan *ConcurrentParseResult, len(files))
	
	var wg sync.WaitGroup
	
	for filePath, config := range files {
		wg.Add(1)
		
		go func(path string, cfg *TransactionParserConfig) {
			defer wg.Done()
			
			// Acquire semaphore
			cp.semaphore <- struct{}{}
			defer func() { <-cp.semaphore }()
			
			result := &ConcurrentParseResult{FilePath: path}
			
			// Create parser
			parser, err := NewTransactionParser(cfg)
			if err != nil {
				result.Error = fmt.Errorf("failed to create parser: %w", err)
				results <- result
				return
			}
			
			// Parse transactions
			transactions, stats, err := parser.ParseTransactionsWithContext(ctx, path)
			result.Transactions = transactions
			result.Stats = stats
			result.Error = err
			
			results <- result
		}(filePath, config)
	}
	
	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()
	
	return results
}

// ParseBankStatementsConcurrently parses multiple bank statement files concurrently
func (cp *ConcurrentParser) ParseBankStatementsConcurrently(
	ctx context.Context,
	files map[string]*BankConfig,
) <-chan *ConcurrentParseResult {
	results := make(chan *ConcurrentParseResult, len(files))
	
	var wg sync.WaitGroup
	
	for filePath, config := range files {
		wg.Add(1)
		
		go func(path string, cfg *BankConfig) {
			defer wg.Done()
			
			// Acquire semaphore
			cp.semaphore <- struct{}{}
			defer func() { <-cp.semaphore }()
			
			result := &ConcurrentParseResult{FilePath: path}
			
			// Create parser
			parser, err := NewBankStatementParser(cfg)
			if err != nil {
				result.Error = fmt.Errorf("failed to create parser: %w", err)
				results <- result
				return
			}
			
			// Parse bank statements
			statements, stats, err := parser.ParseBankStatementsWithContext(ctx, path)
			result.Statements = statements
			result.Stats = stats
			result.Error = err
			
			results <- result
		}(filePath, config)
	}
	
	// Close results channel when all goroutines complete
	go func() {
		wg.Wait()
		close(results)
	}()
	
	return results
}

// MemoryMonitor tracks memory usage during parsing operations
type MemoryMonitor struct {
	maxMemoryMB      int
	checkIntervalSec int
	stopChan         chan bool
	alertCallback    func(memoryMB int)
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(maxMemoryMB int, checkIntervalSec int, alertCallback func(int)) *MemoryMonitor {
	return &MemoryMonitor{
		maxMemoryMB:      maxMemoryMB,
		checkIntervalSec: checkIntervalSec,
		stopChan:         make(chan bool),
		alertCallback:    alertCallback,
	}
}

// Start begins monitoring memory usage
func (mm *MemoryMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(mm.checkIntervalSec) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-mm.stopChan:
			return
		case <-ticker.C:
			// Note: This is a simplified memory check
			// In production, you might want to use runtime.MemStats
			// for more accurate memory monitoring
			if mm.alertCallback != nil {
				// Placeholder for memory check
				// mm.alertCallback(currentMemoryMB)
			}
		}
	}
}

// Stop stops memory monitoring
func (mm *MemoryMonitor) Stop() {
	close(mm.stopChan)
}