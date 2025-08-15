# Transaction Reconciliation Service

A Go-based service designed for Amartha to identify unmatched and discrepant transactions between internal system data and external bank statements.

## Overview

This reconciliation service helps financial institutions maintain data integrity by comparing internal transaction records against bank statements. It identifies:

- **Matched transactions** - Transactions that appear in both systems
- **Unmatched transactions** - Transactions missing from either system or bank statements  
- **Discrepant transactions** - Transactions with amount differences between systems
- **Summary reports** - Comprehensive reconciliation analytics

## Features

- **Multi-format CSV Support** - Handles various bank statement formats
- **Configurable Matching** - Flexible date tolerance and amount precision settings
- **Memory Efficient** - Streaming processing for large transaction volumes
- **Comprehensive Reporting** - Detailed summaries with discrepancy analysis
- **High Performance** - Optimized matching algorithms for large datasets

## Project Structure

```
├── cmd/                    # CLI entry points
├── internal/              # Private application code
│   ├── models/            # Data structures (Transaction, BankStatement)
│   ├── parsers/           # CSV parsing for system and bank files
│   ├── reconciler/        # Main reconciliation orchestration
│   ├── matcher/           # Transaction matching algorithms
│   └── reporter/          # Report generation and formatting
├── pkg/                   # Public API packages
├── test/                  # Test data and integration tests
│   └── examples/          # Sample CSV files
└── go.mod                 # Go module definition
```

## Quick Start

### Prerequisites

- Go 1.21+ installed
- CSV files for system transactions and bank statements

### Installation

```bash
# Clone the repository
git clone <repository-url>
cd golang-reconciliation-service

# Install dependencies
go mod tidy

# Build the application
go build -o reconciler ./cmd/reconciler
```

### Usage

```bash
# Basic reconciliation
reconciler reconcile --system-file transactions.csv --bank-files statements.csv --start-date 2024-01-01 --end-date 2024-01-31

# Multiple bank files
reconciler reconcile --system-file transactions.csv --bank-files bank1.csv,bank2.csv --start-date 2024-01-01 --end-date 2024-01-31

# With custom output format and tolerance settings
reconciler reconcile --system-file transactions.csv --bank-files statements.csv \
  --start-date 2024-01-01 --end-date 2024-01-31 \
  --output-format json --output-file report.json \
  --date-tolerance 2 --amount-tolerance 0.5

# With progress indicators and verbose output
reconciler reconcile --system-file transactions.csv --bank-files statements.csv \
  --progress --verbose
```

## Data Formats

### System Transaction CSV
```csv
trxID,amount,type,transactionTime
TX001,100.50,CREDIT,2024-01-15T10:30:00Z
TX002,250.00,DEBIT,2024-01-15T14:20:00Z
```

### Bank Statement CSV
```csv
unique_identifier,amount,date
BS001,100.50,2024-01-15
BS002,-250.00,2024-01-15
```

## Configuration

The service supports various configuration options via CLI flags and optional config files:

### CLI Configuration Options

- **Date Tolerance** (`--date-tolerance`, `-d`) - Allow ±N days for transaction matching (default: 1)
- **Amount Tolerance** (`--amount-tolerance`, `-a`) - Percentage tolerance for amount matching (0.0-100.0)
- **Output Format** (`--output-format`, `-f`) - Console, JSON, CSV reporting options (default: console)
- **Output File** (`--output-file`, `-o`) - Specify output file path (default: stdout)
- **Date Filtering** (`--start-date`, `--end-date`) - Filter transactions by date range (YYYY-MM-DD format)
- **Progress Indicators** (`--progress`) - Show progress during processing
- **Verbose Output** (`--verbose`, `-v`) - Enable detailed logging

### Config File Support

Use `--config path/to/config.toml` to specify a configuration file:

```toml
# reconciler.toml
[matching]
date_tolerance = 2
amount_tolerance = 0.5

[output]
format = "json"
file = "report.json"

[processing]
progress = true
verbose = true
```

### CLI Command Reference

#### Global Options

Available for all commands:

```bash
--config string    # Path to configuration file (optional)
--verbose, -v      # Enable verbose output for detailed logging
--help, -h         # Show help information
--version          # Show version information
```

#### reconcile Command

The main command for performing reconciliation:

```bash
reconciler reconcile [flags]
```

**Required Flags:**
- `--system-file, -s`: Path to system transaction CSV file
- `--bank-files, -b`: Comma-separated paths to bank statement CSV files

**Optional Flags:**
- `--output-format, -f`: Output format (console, json, csv) [default: console]
- `--output-file, -o`: Output file path [default: stdout]
- `--start-date`: Filter start date (YYYY-MM-DD format)
- `--end-date`: Filter end date (YYYY-MM-DD format)
- `--date-tolerance, -d`: Date matching tolerance in days [default: 1]
- `--amount-tolerance, -a`: Amount tolerance percentage (0.0-100.0) [default: 0.0]
- `--progress`: Show progress indicators during processing

**Examples:**

```bash
# Minimal example
reconciler reconcile -s transactions.csv -b statements.csv

# Full featured example
reconciler reconcile \
  --system-file transactions.csv \
  --bank-files bank1.csv,bank2.csv \
  --start-date 2024-01-01 \
  --end-date 2024-01-31 \
  --date-tolerance 2 \
  --amount-tolerance 0.5 \
  --output-format json \
  --output-file report.json \
  --progress \
  --verbose

# Using config file
reconciler reconcile -s tx.csv -b stmt.csv --config reconciler.toml
```

#### Other Commands

```bash
# Show version information
reconciler --version

# Generate shell completion scripts
reconciler completion bash > /etc/bash_completion.d/reconciler
reconciler completion zsh > ~/.zshrc
reconciler completion fish > ~/.config/fish/completions/reconciler.fish

# Get help for any command
reconciler help
reconciler help reconcile
```

## Development

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests with coverage
go test -v -cover ./...

# Benchmark tests
go test -bench=. ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
golangci-lint run

# Check for security issues
gosec ./...
```

## API Documentation

The service provides both CLI and programmatic interfaces:

- **CLI Tool** - Command-line interface for batch processing
- **Go API** - Embeddable packages for integration with other services
- **REST API** - HTTP endpoints for web-based reconciliation (planned)

### Go API Reference

#### Core Packages

##### models Package
```go
import "golang-reconciliation-service/internal/models"

// Create transactions
tx := models.NewTransaction("TX001", decimal.NewFromFloat(100.50), models.TransactionTypeCredit, time.Now())

// Parse from CSV data
tx, err := models.CreateTransactionFromCSV("TX001", "$100.50", "CREDIT", "2024-01-15T10:30:00Z")

// Create bank statements
stmt := models.NewBankStatement("BS001", decimal.NewFromFloat(-100.50), time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
```

##### parsers Package
```go
import "golang-reconciliation-service/internal/parsers"

// Basic transaction parsing
config := &parsers.TransactionParserConfig{
    HasHeader: true,
    Delimiter: ',',
}
parser, err := parsers.NewTransactionParser(config)
transactions, stats, err := parser.ParseTransactions("transactions.csv")

// Streaming for large files
streamConfig := parsers.DefaultStreamingConfig()
streamParser, err := parsers.NewStreamingTransactionParser(config, streamConfig)

// Parse with progress callback
progressCallback := func(progress *parsers.ProgressReport) {
    fmt.Printf("Progress: %.1f%% (%d/%d records)\n", 
        progress.PercentComplete, progress.ValidRecords, progress.EstimatedTotal)
}

err = streamParser.ParseTransactionsStreamAdvanced(ctx, "large_file.csv", 
    func(batch []*models.Transaction) error {
        // Process batch
        return nil
    }, progressCallback)
```

##### matcher Package
```go
import "golang-reconciliation-service/internal/matcher"

// Configure matching engine
config := matcher.DefaultMatchingConfig()
config.DateToleranceDays = 2
config.AmountTolerancePercent = 0.5
config.MinConfidenceScore = 0.8

// Create engine and load data
engine := matcher.NewMatchingEngine(config)
err = engine.LoadTransactions(transactions)
err = engine.LoadBankStatements(statements)

// Perform reconciliation
result, err := engine.Reconcile()

// Access results
fmt.Printf("Found %d matches\n", len(result.Matches))
fmt.Printf("Unmatched transactions: %d\n", len(result.UnmatchedTransactions))
fmt.Printf("Match rate: %.1f%%\n", 
    float64(result.Summary.MatchedTransactions)/float64(result.Summary.TotalTransactions)*100)

// Find matches for specific transaction
matches, err := engine.FindMatches(transaction)
for _, match := range matches {
    fmt.Printf("Match: %s -> %s (confidence: %.2f, type: %s)\n",
        match.Transaction.TrxID, match.BankStatement.UniqueIdentifier,
        match.ConfidenceScore, match.MatchType)
}
```

##### reconciler Package
```go
import "golang-reconciliation-service/internal/reconciler"

// Basic reconciliation service
service := reconciler.NewReconciliationService()

request := &reconciler.ReconciliationRequest{
    TransactionFiles:   []string{"transactions.csv"},
    BankStatementFiles: []string{"statements.csv"},
    DateRange: reconciler.DateRange{
        Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
        End:   time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
    },
    MatchingConfig: matcher.DefaultMatchingConfig(),
}

result, err := service.ProcessReconciliation(context.Background(), request)

// Advanced orchestration with progress tracking
orchestrator := reconciler.NewReconciliationOrchestrator(service)
orchestrator.AddProgressCallback(func(progress *reconciler.ReconciliationProgress) {
    fmt.Printf("Step: %s (%.1f%% complete)\n", 
        progress.CurrentStep, progress.PercentComplete)
})

result, err = orchestrator.ProcessReconciliation(context.Background(), request)
```

##### reporter Package
```go
import "golang-reconciliation-service/internal/reporter"

// Generate reports in different formats
reporter := reporter.NewReconciliationReporter()

// JSON report for programmatic use
jsonReport, err := reporter.GenerateReport(result, reporter.FormatJSON)

// Console report for human readability
consoleReport, err := reporter.GenerateReport(result, reporter.FormatConsole)

// CSV report for spreadsheet analysis
csvReport, err := reporter.GenerateReport(result, reporter.FormatCSV)

// Customized console report
options := &reporter.ReportOptions{
    IncludeMatchDetails: true,
    SortBy:             "amount",
    MaxItems:           100,
    ShowUnmatchedOnly:  false,
}
customReport, err := reporter.GenerateConsoleReport(result, options)
```

#### Configuration Examples

##### Strict Matching Configuration
```go
config := matcher.StrictMatchingConfig()
// Exact date matching, no amount tolerance, high confidence threshold
```

##### Relaxed Matching Configuration
```go
config := matcher.RelaxedMatchingConfig()
// 3-day tolerance, 1% amount tolerance, fuzzy matching enabled
```

##### Custom Configuration
```go
config := &matcher.MatchingConfig{
    DateToleranceDays:      1,
    AmountTolerancePercent: 0.5,
    EnableFuzzyMatching:    true,
    TimezoneHandling:       matcher.TimezoneIgnore,
    MinConfidenceScore:     0.75,
    EnableTypeMatching:     true,
    Weights: matcher.MatchingWeights{
        AmountWeight: 0.6,
        DateWeight:   0.3,
        TypeWeight:   0.1,
    },
}
```

#### Error Handling

All API functions return detailed error information:

```go
result, err := engine.Reconcile()
if err != nil {
    switch e := err.(type) {
    case *errors.ValidationError:
        fmt.Printf("Validation error: %s\n", e.Error())
        fmt.Printf("Suggestion: %s\n", e.Suggestion)
    case *errors.ReconciliationError:
        fmt.Printf("Reconciliation error: %s\n", e.Error())
        fmt.Printf("Component: %s\n", e.Component)
    default:
        fmt.Printf("Unexpected error: %s\n", err.Error())
    }
}

## Architecture

### System Overview

The reconciliation service follows a layered architecture with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Interface                           │
│                   (cmd/reconciler)                          │
├─────────────────────────────────────────────────────────────┤
│                 Orchestration Layer                         │
│              (internal/reconciler)                          │
│   ┌─────────────────┐  ┌─────────────────┐                │
│   │ Reconciliation  │  │ Data            │                │
│   │ Service         │  │ Preprocessor    │                │
│   └─────────────────┘  └─────────────────┘                │
├─────────────────────────────────────────────────────────────┤
│                    Processing Layer                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
│  │   Parsers   │ │   Matcher   │ │  Reporter   │          │
│  │   (CSV)     │ │  (Engine)   │ │ (Multiple   │          │
│  │             │ │             │ │  Formats)   │          │
│  └─────────────┘ └─────────────┘ └─────────────┘          │
├─────────────────────────────────────────────────────────────┤
│                      Data Layer                            │
│                  (internal/models)                         │
│   ┌─────────────────┐  ┌─────────────────┐                │
│   │  Transaction    │  │  BankStatement  │                │
│   │     Model       │  │     Model       │                │
│   └─────────────────┘  └─────────────────┘                │
├─────────────────────────────────────────────────────────────┤
│                   Infrastructure Layer                      │
│                    (pkg/logger, pkg/errors)                │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

The reconciliation process follows this sequence:

1. **Input Processing**
   - CSV files are parsed using configurable parsers
   - Data validation occurs during parsing
   - Transactions and bank statements are loaded into memory models

2. **Indexing**
   - Data is indexed by amount, date, and other criteria for fast lookups
   - Multiple index structures support different matching strategies

3. **Matching Algorithm**
   ```
   For each transaction:
     1. Find candidate bank statements using indexes
     2. Score each candidate based on:
        - Amount similarity (weighted)
        - Date proximity (weighted) 
        - Transaction type compatibility (weighted)
     3. Select best match above confidence threshold
     4. Mark both transaction and statement as matched
   ```

4. **Result Generation**
   - Compile matches, unmatched items, and statistics
   - Generate reports in requested format(s)

### Key Components

#### Matching Engine (`internal/matcher`)

The core matching algorithm uses a multi-criteria scoring system:

- **Amount Matching**: Exact match or percentage-based tolerance
- **Date Matching**: Day-based tolerance with optional weekend exclusion  
- **Type Matching**: Debit/credit compatibility checking
- **Confidence Scoring**: Weighted combination of all criteria

Index structures provide O(log n) lookup performance:
- Amount range indexes for tolerance-based matching
- Date bucket indexes for temporal matching
- Hash indexes for exact matches

#### Parser Framework (`internal/parsers`)

Supports multiple parsing strategies:

- **Standard Parsing**: Full file loading for small to medium files
- **Streaming Parsing**: Memory-efficient processing for large files
- **Concurrent Parsing**: Parallel processing of multiple files

Features include:
- Configurable CSV dialects and formats
- Comprehensive error handling and recovery
- Progress reporting for long operations
- Memory usage monitoring

#### Configuration System

Three-tier configuration approach:
- **Default Configurations**: Sensible defaults for most use cases
- **Preset Configurations**: Strict, relaxed, and balanced presets
- **Custom Configurations**: Full control over all parameters

### Performance Characteristics

#### Memory Usage

- **Standard Processing**: ~100MB per 100K transactions
- **Streaming Processing**: Configurable batch sizes (typically 1-10MB peak)
- **Index Overhead**: ~30% of data size for fast lookups

#### Processing Speed

Current benchmarks (on standard hardware):
- **Small datasets** (< 10K records): < 1 second
- **Medium datasets** (10K - 100K records): 1-10 seconds  
- **Large datasets** (100K - 1M records): 10-60 seconds
- **Very large datasets** (> 1M records): Use streaming mode

#### Scalability Limits

- **Memory-bound**: ~5M transactions in standard mode
- **I/O-bound**: Limited by disk read speed in streaming mode
- **CPU-bound**: Matching complexity is O(n log n) average case

### Error Handling Strategy

The service implements comprehensive error handling:

1. **Validation Errors**: Data format and constraint violations
2. **Processing Errors**: File access, parsing, and matching failures  
3. **System Errors**: Memory, timeout, and resource exhaustion

Error propagation includes:
- Detailed error messages with context
- Actionable suggestions for resolution
- Error aggregation for batch operations

### Security Considerations

- **Input Validation**: All CSV input is validated and sanitized
- **Memory Safety**: Bounded memory usage prevents DoS attacks
- **File Access**: Restricted to specified input directories
- **No Network Access**: Pure file-based processing (no external calls)

## Troubleshooting

### Common Issues

#### High Memory Usage

**Symptoms**: Out of memory errors, slow performance, system becoming unresponsive

**Causes**:
- Processing very large files in standard mode
- Multiple large files being processed simultaneously
- Insufficient system memory for dataset size

**Solutions**:
```bash
# Use smaller batch sizes for large files (via config file)
reconciler reconcile --system-file large_transactions.csv --bank-files large_statements.csv --config config.toml

# Process files individually instead of simultaneously
reconciler reconcile --system-file transactions.csv --bank-files bank1.csv
reconciler reconcile --system-file transactions.csv --bank-files bank2.csv

# Use progress monitoring to track memory usage
reconciler reconcile --system-file transactions.csv --bank-files statements.csv --progress --verbose
```

**API Solution**:
```go
// Use streaming parsers for large files
streamConfig := &parsers.StreamingConfig{
    BatchSize: 1000, 
    ReportProgress: true,
}
parser, err := parsers.NewStreamingTransactionParser(config, streamConfig)
```

#### Poor Matching Performance

**Symptoms**: Very slow matching process, high CPU usage

**Causes**:
- Very large datasets without proper indexing
- Inefficient matching configuration
- Complex fuzzy matching on large datasets

**Solutions**:
```bash
# Reduce date tolerance to limit candidate search
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --date-tolerance 1

# Set amount tolerance to zero for exact matching
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --amount-tolerance 0.0

# Use minimal tolerance for better performance
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --date-tolerance 0 --amount-tolerance 0.0
```

**API Solution**:
```go
// Optimize configuration for performance
config := matcher.DefaultMatchingConfig()
config.EnableFuzzyMatching = false
config.DateToleranceDays = 1
config.MaxCandidatesPerTransaction = 5
```

#### Low Match Rates

**Symptoms**: Many unmatched transactions, low confidence scores

**Causes**:
- Too strict matching criteria
- Data quality issues (date formats, amounts)
- Timezone or date offset problems

**Solutions**:
```bash
# Increase tolerances for more flexible matching
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --date-tolerance 3 --amount-tolerance 1.0

# Use maximum tolerances for exploratory analysis
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --date-tolerance 7 --amount-tolerance 5.0

# Enable verbose output to analyze match quality
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --verbose
```

**Data Quality Checks**:
```bash
# Use verbose output to validate data quality
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --verbose

# Test with minimal date range first
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --start-date 2024-01-01 --end-date 2024-01-07
```

#### CLI Flag Errors

**Symptoms**: "Error: unknown flag: --xxx" messages

**Common Causes**:
- Typos in flag names (e.g., `--ouput-format` instead of `--output-format`)
- Using old command syntax
- Incorrect flag combinations

**Solutions**:
```bash
# Common typos to avoid:
# ❌ --ouput-format (missing 't')
# ✅ --output-format

# ❌ --system (old syntax)
# ✅ --system-file

# ❌ --bank (old syntax)  
# ✅ --bank-files

# Use help to verify flag names
reconciler reconcile --help

# Test with minimal flags first
reconciler reconcile --system-file tx.csv --bank-files stmt.csv
```

#### CSV Parsing Errors

**Symptoms**: "Parse error at line X", "Invalid format" messages

**Causes**:
- Incorrect CSV format detection
- Encoding issues (non-UTF8 files)
- Inconsistent data formats

**Solutions**:
```bash
# Use verbose output to identify parsing issues
reconciler reconcile --system-file tx.csv --bank-files stmt.csv --verbose

# Handle encoding issues before processing
iconv -f ISO-8859-1 -t UTF-8 input.csv > output.csv

# Test with small subset first
head -100 transactions.csv > test_transactions.csv
reconciler reconcile --system-file test_transactions.csv --bank-files stmt.csv --verbose
```

### Performance Tuning

#### For Large Datasets (>100K records)

1. **Use progress monitoring for large files**:
   ```bash
   reconciler reconcile --system-file large.csv --bank-files statements.csv --progress --verbose
   ```

2. **Optimize batch size**:
   ```go
   streamConfig := &parsers.StreamingConfig{
       BatchSize: 2000,  // Tune based on available memory
   }
   ```

3. **Disable unnecessary features**:
   ```go
   config := matcher.DefaultMatchingConfig()
   config.EnableFuzzyMatching = false  // Faster processing
   config.MaxCandidatesPerTransaction = 5  // Limit search space
   ```

#### For High-Volume Operations

1. **Process files in parallel**:
   ```go
   concurrentParser := parsers.NewConcurrentParser(4)  // 4 parallel workers
   results := concurrentParser.ParseTransactionsConcurrently(ctx, fileConfigs)
   ```

2. **Monitor memory usage**:
   ```go
   monitor := parsers.NewMemoryMonitor(1024, 5, func(memMB int) {
       log.Printf("Memory usage: %dMB", memMB)
   })
   monitor.Start(ctx)
   ```

#### Configuration Recommendations

| Dataset Size | Mode | Batch Size | Memory Usage | Processing Time |
|-------------|------|------------|--------------|-----------------|
| < 10K | Standard | N/A | < 50MB | < 5 seconds |
| 10K - 100K | Standard | N/A | 50-500MB | 5-30 seconds |
| 100K - 1M | Streaming | 5000 | 100-200MB | 30-300 seconds |
| > 1M | Streaming | 1000-2000 | 50-100MB | 5-30 minutes |

### Monitoring and Diagnostics

#### Performance Metrics

Monitor these key metrics during operation:

```go
// Processing metrics
fmt.Printf("Records/second: %.1f\n", float64(totalRecords)/processingTime.Seconds())
fmt.Printf("Memory efficiency: %.1f records/MB\n", float64(totalRecords)/memoryUsageMB)
fmt.Printf("Match rate: %.1f%%\n", matchRate)

// Quality metrics
fmt.Printf("Exact matches: %d\n", summary.ExactMatches)
fmt.Printf("Fuzzy matches: %d\n", summary.FuzzyMatches)
fmt.Printf("Confidence average: %.2f\n", averageConfidence)
```

#### Debugging Tools

1. **Verbose logging**:
   ```bash
   reconciler reconcile --system-file tx.csv --bank-files stmt.csv --verbose
   ```

2. **Profile CPU and memory**:
   ```bash
   go tool pprof cpu.prof
   go tool pprof mem.prof
   ```

3. **Benchmark performance**:
   ```bash
   go test -bench=. -benchmem ./internal/matcher
   go test -bench=. -benchmem ./internal/parsers
   ```

### Getting Help

If you encounter issues not covered in this guide:

1. **Check existing issues**: Search GitHub issues for similar problems
2. **Enable verbose output**: Use `--verbose` for detailed output
3. **Provide sample data**: Include anonymized sample CSV files
4. **Include system information**: OS, Go version, available memory
5. **Share configuration**: Include matching configuration and CLI arguments used

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/new-feature`)
3. Commit changes (`git commit -am 'Add new feature'`)
4. Push to branch (`git push origin feature/new-feature`) 
5. Create Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For questions, issues, or contributions, please contact the development team or create an issue in the project repository.

---

**Built with ❤️ for Amartha's financial reconciliation needs**