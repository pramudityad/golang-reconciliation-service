package reconciler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang-reconciliation-service/internal/matcher"
	"golang-reconciliation-service/internal/models"
	"golang-reconciliation-service/internal/parsers"

	"github.com/shopspring/decimal"
)

// parseSystemTransactions parses the system transaction file
func (rs *ReconciliationService) parseSystemTransactions(
	ctx context.Context,
	request *ReconciliationRequest,
) ([]*models.Transaction, *parsers.ParseStats, error) {
	
	// Update parser configuration if needed
	if request.TransactionConfig != nil {
		parser, err := parsers.NewTransactionParser(request.TransactionConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create transaction parser: %w", err)
		}
		rs.transactionParser = parser
	}
	
	// Parse transactions with context
	transactions, stats, err := rs.transactionParser.ParseTransactionsWithContext(ctx, request.SystemFile)
	if err != nil {
		return nil, stats, fmt.Errorf("failed to parse transactions from %s: %w", request.SystemFile, err)
	}
	
	// Validate transactions if configured
	if rs.config.ValidateInputs {
		validTransactions := make([]*models.Transaction, 0, len(transactions))
		for _, tx := range transactions {
			if err := tx.Validate(); err != nil {
				// Log validation error but continue processing
				stats.ErrorCount++
				continue
			}
			validTransactions = append(validTransactions, tx)
		}
		transactions = validTransactions
	}
	
	return transactions, stats, nil
}

// parseBankStatements parses all bank statement files concurrently
func (rs *ReconciliationService) parseBankStatements(
	ctx context.Context,
	request *ReconciliationRequest,
) ([]*models.BankStatement, map[string]*parsers.ParseStats, error) {
	
	if len(request.BankFiles) == 0 {
		return nil, nil, fmt.Errorf("no bank files provided")
	}
	
	// For single file, use simple parsing
	if len(request.BankFiles) == 1 {
		return rs.parseSingleBankFile(ctx, request.BankFiles[0], request.BankConfigs)
	}
	
	// For multiple files, use concurrent parsing
	return rs.parseMultipleBankFiles(ctx, request.BankFiles, request.BankConfigs)
}

// parseSingleBankFile parses a single bank statement file
func (rs *ReconciliationService) parseSingleBankFile(
	ctx context.Context,
	filePath string,
	bankConfigs map[string]*parsers.BankConfig,
) ([]*models.BankStatement, map[string]*parsers.ParseStats, error) {
	
	config, exists := bankConfigs[filePath]
	if !exists {
		return nil, nil, fmt.Errorf("no configuration found for bank file: %s", filePath)
	}
	
	// Create parser for this specific file
	parser, err := parsers.NewBankStatementParser(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create parser for %s: %w", filePath, err)
	}
	
	// Parse statements
	statements, stats, err := parser.ParseBankStatementsWithContext(ctx, filePath)
	if err != nil {
		return nil, map[string]*parsers.ParseStats{filePath: stats}, 
			fmt.Errorf("failed to parse bank statements from %s: %w", filePath, err)
	}
	
	// Validate statements if configured
	if rs.config.ValidateInputs {
		validStatements := make([]*models.BankStatement, 0, len(statements))
		for _, stmt := range statements {
			if err := stmt.Validate(); err != nil {
				stats.ErrorCount++
				continue
			}
			validStatements = append(validStatements, stmt)
		}
		statements = validStatements
	}
	
	return statements, map[string]*parsers.ParseStats{filePath: stats}, nil
}

// parseMultipleBankFiles parses multiple bank statement files concurrently
func (rs *ReconciliationService) parseMultipleBankFiles(
	ctx context.Context,
	filePaths []string,
	bankConfigs map[string]*parsers.BankConfig,
) ([]*models.BankStatement, map[string]*parsers.ParseStats, error) {
	
	// Set up concurrent processing
	maxConcurrency := rs.config.MaxConcurrentFiles
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	allStatements := make([]*models.BankStatement, 0)
	allStats := make(map[string]*parsers.ParseStats)
	var parseErrors []error
	
	// Process each file concurrently
	for _, filePath := range filePaths {
		wg.Add(1)
		
		go func(path string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			config, exists := bankConfigs[path]
			if !exists {
				mu.Lock()
				parseErrors = append(parseErrors, fmt.Errorf("no configuration found for bank file: %s", path))
				mu.Unlock()
				return
			}
			
			// Create parser for this file
			parser, err := parsers.NewBankStatementParser(config)
			if err != nil {
				mu.Lock()
				parseErrors = append(parseErrors, fmt.Errorf("failed to create parser for %s: %w", path, err))
				mu.Unlock()
				return
			}
			
			// Parse statements
			statements, stats, err := parser.ParseBankStatementsWithContext(ctx, path)
			if err != nil {
				mu.Lock()
				parseErrors = append(parseErrors, fmt.Errorf("failed to parse %s: %w", path, err))
				allStats[path] = stats
				mu.Unlock()
				return
			}
			
			// Validate statements if configured
			if rs.config.ValidateInputs {
				validStatements := make([]*models.BankStatement, 0, len(statements))
				for _, stmt := range statements {
					if err := stmt.Validate(); err != nil {
						stats.ErrorCount++
						continue
					}
					validStatements = append(validStatements, stmt)
				}
				statements = validStatements
			}
			
			// Thread-safe aggregation
			mu.Lock()
			allStatements = append(allStatements, statements...)
			allStats[path] = stats
			mu.Unlock()
			
		}(filePath)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	
	// Check for errors
	if len(parseErrors) > 0 {
		return allStatements, allStats, fmt.Errorf("parsing errors occurred: %v", parseErrors)
	}
	
	return allStatements, allStats, nil
}

// applyDateRangeFiltering filters transactions and statements by date range
func (rs *ReconciliationService) applyDateRangeFiltering(
	transactions []*models.Transaction,
	statements []*models.BankStatement,
	request *ReconciliationRequest,
) ([]*models.Transaction, []*models.BankStatement) {
	
	startDate := request.StartDate
	endDate := request.EndDate
	
	// If no date filtering is requested, return original data
	if startDate == nil && endDate == nil {
		return transactions, statements
	}
	
	// Filter transactions
	filteredTransactions := make([]*models.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if rs.isWithinDateRange(tx.TransactionTime, startDate, endDate) {
			filteredTransactions = append(filteredTransactions, tx)
		}
	}
	
	// Filter statements
	filteredStatements := make([]*models.BankStatement, 0, len(statements))
	for _, stmt := range statements {
		if rs.isWithinDateRange(stmt.Date, startDate, endDate) {
			filteredStatements = append(filteredStatements, stmt)
		}
	}
	
	return filteredTransactions, filteredStatements
}

// isWithinDateRange checks if a date falls within the specified range
func (rs *ReconciliationService) isWithinDateRange(date time.Time, startDate, endDate *time.Time) bool {
	if startDate != nil && date.Before(*startDate) {
		return false
	}
	
	if endDate != nil && date.After(*endDate) {
		return false
	}
	
	return true
}

// performMatching executes the reconciliation matching process
func (rs *ReconciliationService) performMatching(
	ctx context.Context,
	transactions []*models.Transaction,
	statements []*models.BankStatement,
) (*matcher.ReconciliationResult, error) {
	
	// Load data into matching engine
	if err := rs.matchingEngine.LoadTransactions(transactions); err != nil {
		return nil, fmt.Errorf("failed to load transactions into matching engine: %w", err)
	}
	
	if err := rs.matchingEngine.LoadBankStatements(statements); err != nil {
		return nil, fmt.Errorf("failed to load bank statements into matching engine: %w", err)
	}
	
	// Perform reconciliation
	result, err := rs.matchingEngine.Reconcile()
	if err != nil {
		return nil, fmt.Errorf("matching engine reconciliation failed: %w", err)
	}
	
	return result, nil
}

// analyzeDiscrepancies identifies potential discrepancies in the reconciliation results
func (rs *ReconciliationService) analyzeDiscrepancies(
	matches []*matcher.MatchResult,
	transactions []*models.Transaction,
	statements []*models.BankStatement,
) []*Discrepancy {
	
	var discrepancies []*Discrepancy
	
	// Analyze matches for discrepancies
	for _, match := range matches {
		// Check for amount differences in fuzzy matches
		if match.MatchType == matcher.MatchFuzzy || match.MatchType == matcher.MatchPossible {
			if !match.Transaction.Amount.Equal(match.BankStatement.NormalizeAmount()) {
				discrepancy := &Discrepancy{
					Type:        DiscrepancyAmountDifference,
					Transaction: match.Transaction,
					Statement:   match.BankStatement,
					Description: fmt.Sprintf("Amount difference detected: transaction %s vs statement %s",
						match.Transaction.Amount.String(), match.BankStatement.NormalizeAmount().String()),
					Amount:      match.Transaction.Amount.Sub(match.BankStatement.NormalizeAmount()).Abs(),
					Severity:    rs.determineSeverity(match.ConfidenceScore),
				}
				discrepancies = append(discrepancies, discrepancy)
			}
		}
		
		// Check for date mismatches
		if rs.config.StrictDateMatching {
			txDate := match.Transaction.TransactionTime.Truncate(24 * time.Hour)
			stmtDate := match.BankStatement.Date.Truncate(24 * time.Hour)
			
			if !txDate.Equal(stmtDate) {
				discrepancy := &Discrepancy{
					Type:        DiscrepancyDateMismatch,
					Transaction: match.Transaction,
					Statement:   match.BankStatement,
					Description: fmt.Sprintf("Date mismatch: transaction %s vs statement %s",
						txDate.Format("2006-01-02"), stmtDate.Format("2006-01-02")),
					Severity:    SeverityMedium,
				}
				discrepancies = append(discrepancies, discrepancy)
			}
		}
		
		// Check for type mismatches
		if match.Transaction.Type != match.BankStatement.GetTransactionType() {
			discrepancy := &Discrepancy{
				Type:        DiscrepancyTypeMismatch,
				Transaction: match.Transaction,
				Statement:   match.BankStatement,
				Description: fmt.Sprintf("Transaction type mismatch: %s vs %s",
					match.Transaction.Type, match.BankStatement.GetTransactionType()),
				Severity:    SeverityHigh,
			}
			discrepancies = append(discrepancies, discrepancy)
		}
	}
	
	// Analyze for duplicate transactions
	discrepancies = append(discrepancies, rs.findDuplicateTransactions(transactions)...)
	
	// Analyze for duplicate statements
	discrepancies = append(discrepancies, rs.findDuplicateStatements(statements)...)
	
	return discrepancies
}

// findDuplicateTransactions identifies duplicate transactions
func (rs *ReconciliationService) findDuplicateTransactions(transactions []*models.Transaction) []*Discrepancy {
	var discrepancies []*Discrepancy
	seen := make(map[string]*models.Transaction)
	
	for _, tx := range transactions {
		// Create a key based on amount, type, and date (normalized to day)
		key := fmt.Sprintf("%s_%s_%s", 
			tx.Amount.String(), 
			tx.Type, 
			tx.TransactionTime.Format("2006-01-02"))
		
		if existingTx, exists := seen[key]; exists {
			discrepancy := &Discrepancy{
				Type:        DiscrepancyDuplicateTransaction,
				Transaction: tx,
				Description: fmt.Sprintf("Duplicate transaction detected: %s and %s", existingTx.TrxID, tx.TrxID),
				Severity:    SeverityHigh,
			}
			discrepancies = append(discrepancies, discrepancy)
		} else {
			seen[key] = tx
		}
	}
	
	return discrepancies
}

// findDuplicateStatements identifies duplicate bank statements
func (rs *ReconciliationService) findDuplicateStatements(statements []*models.BankStatement) []*Discrepancy {
	var discrepancies []*Discrepancy
	seen := make(map[string]*models.BankStatement)
	
	for _, stmt := range statements {
		// Create a key based on amount and date
		key := fmt.Sprintf("%s_%s", stmt.Amount.String(), stmt.Date.Format("2006-01-02"))
		
		if existingStmt, exists := seen[key]; exists {
			discrepancy := &Discrepancy{
				Type:        DiscrepancyDuplicateStatement,
				Statement:   stmt,
				Description: fmt.Sprintf("Duplicate statement detected: %s and %s", 
					existingStmt.UniqueIdentifier, stmt.UniqueIdentifier),
				Severity:    SeverityHigh,
			}
			discrepancies = append(discrepancies, discrepancy)
		} else {
			seen[key] = stmt
		}
	}
	
	return discrepancies
}

// determineSeverity determines the severity of a discrepancy based on confidence score
func (rs *ReconciliationService) determineSeverity(confidenceScore float64) Severity {
	switch {
	case confidenceScore >= 0.9:
		return SeverityLow
	case confidenceScore >= 0.7:
		return SeverityMedium
	case confidenceScore >= 0.5:
		return SeverityHigh
	default:
		return SeverityCritical
	}
}

// buildFinalResult constructs the final reconciliation result
func (rs *ReconciliationService) buildFinalResult(
	result *ReconciliationResult,
	matchingResult *matcher.ReconciliationResult,
	discrepancies []*Discrepancy,
	transactionStats *parsers.ParseStats,
	bankStats map[string]*parsers.ParseStats,
	matchingDuration time.Duration,
) {
	
	// Populate matched transactions
	if rs.config.DetailedBreakdown {
		result.MatchedTransactions = matchingResult.Matches
	}
	
	// Populate unmatched transactions and statements
	if rs.config.DetailedBreakdown {
		result.UnmatchedTransactions = matchingResult.UnmatchedTransactions
		result.UnmatchedStatements = matchingResult.UnmatchedStatements
	}
	
	// Set discrepancies
	result.Discrepancies = discrepancies
	
	// Build summary from matching result
	summary := matchingResult.Summary
	result.Summary.TotalTransactions = summary.TotalTransactions
	result.Summary.MatchedTransactions = summary.MatchedTransactions
	result.Summary.UnmatchedTransactions = summary.UnmatchedTransactions
	result.Summary.TotalBankStatements = summary.TotalBankStatements
	result.Summary.MatchedStatements = summary.MatchedStatements
	result.Summary.UnmatchedStatements = summary.UnmatchedStatements
	result.Summary.ExactMatches = summary.ExactMatches
	result.Summary.CloseMatches = summary.CloseMatches
	result.Summary.FuzzyMatches = summary.FuzzyMatches
	result.Summary.PossibleMatches = summary.PossibleMatches
	
	// Calculate financial summaries
	rs.calculateFinancialSummary(result, matchingResult)
	
	// Build processing statistics
	if rs.config.IncludeStatistics {
		rs.buildProcessingStats(result, transactionStats, bankStats, matchingDuration)
	}
}

// calculateFinancialSummary calculates financial summary information
func (rs *ReconciliationService) calculateFinancialSummary(
	result *ReconciliationResult,
	matchingResult *matcher.ReconciliationResult,
) {
	
	totalTxAmount := decimal.Zero
	totalStmtAmount := decimal.Zero
	
	// Calculate total transaction amount from all transactions
	allTransactions := append(matchingResult.UnmatchedTransactions, []*models.Transaction{}...)
	for _, match := range matchingResult.Matches {
		allTransactions = append(allTransactions, match.Transaction)
	}
	for _, tx := range allTransactions {
		totalTxAmount = totalTxAmount.Add(tx.GetAbsoluteAmount())
	}
	
	// Calculate total statement amount from all statements
	allStatements := append(matchingResult.UnmatchedStatements, []*models.BankStatement{}...)
	for _, match := range matchingResult.Matches {
		allStatements = append(allStatements, match.BankStatement)
	}
	for _, stmt := range allStatements {
		totalStmtAmount = totalStmtAmount.Add(stmt.NormalizeAmount())
	}
	
	result.Summary.TotalTransactionAmount = totalTxAmount
	result.Summary.TotalStatementAmount = totalStmtAmount
	result.Summary.NetDiscrepancy = totalTxAmount.Sub(totalStmtAmount)
}

// buildProcessingStats builds detailed processing statistics
func (rs *ReconciliationService) buildProcessingStats(
	result *ReconciliationResult,
	transactionStats *parsers.ParseStats,
	bankStats map[string]*parsers.ParseStats,
	matchingDuration time.Duration,
) {
	
	stats := result.ProcessingStats
	
	// File processing stats
	stats.FilesProcessed = 1 + len(bankStats) // System file + bank files
	
	// Aggregate parse errors
	if transactionStats != nil {
		stats.ParseErrors += transactionStats.ErrorCount
	}
	
	for _, bankStat := range bankStats {
		stats.ParseErrors += bankStat.ErrorCount
	}
	
	// Performance metrics
	stats.MatchingTime = matchingDuration
	
	if stats.TotalProcessingTime > 0 {
		totalRecords := float64(result.Summary.TotalTransactions + result.Summary.TotalBankStatements)
		stats.RecordsPerSecond = totalRecords / stats.TotalProcessingTime.Seconds()
	}
}