# Test Data Documentation

This directory contains comprehensive test datasets for the reconciliation service, designed to cover various matching scenarios and edge cases.

## Directory Structure

```
testdata/
├── csv/                           # Main test data files
│   ├── system_transactions.csv    # Primary transaction dataset
│   ├── bank_statement_bank1.csv   # Standard bank format
│   ├── bank_statement_bank2.csv   # Alternative bank format
│   └── edge_cases/               # Specialized edge case datasets
│       ├── duplicate_transactions.csv
│       ├── duplicate_statements.csv
│       ├── same_day_multiple.csv
│       ├── large_amounts.csv
│       ├── boundary_dates.csv
│       ├── timezone_variations.csv
│       ├── partial_matches.csv
│       └── partial_match_statements.csv
├── generators/                   # Data generation scripts (upcoming)
├── validators/                   # Data validation tools (upcoming)
└── README.md                    # This file
```

## Main Dataset Files

### system_transactions.csv
**Format**: `trxID,amount,type,transactionTime`
- **Records**: 113 transactions covering Jan-Mar 2024
- **Amount Range**: $0.01 to $50,000.00
- **Transaction Types**: CREDIT and DEBIT
- **Time Format**: RFC3339 (2024-01-15T10:30:00Z)

**Key Features**:
- Perfect matches with bank statements
- Close matches (small amount differences)
- Date tolerance scenarios (1-3 day differences)
- Large amount transactions for stress testing
- Boundary case amounts (0.01, 999.999, etc.)
- Realistic transaction patterns and frequencies

### bank_statement_bank1.csv
**Format**: `unique_identifier,amount,date`
- **Records**: 122 statements
- **Convention**: Negative amounts for debits, positive for credits
- **Date Format**: YYYY-MM-DD
- **Identifier Pattern**: BS001, BS002, etc.

**Matching Characteristics**:
- ~85% direct matches with system transactions
- ~10% close matches (small amount differences)
- ~5% unmatched statements (orphaned bank records)

### bank_statement_bank2.csv
**Format**: `transaction_id,transaction_amount,posting_date,transaction_description`
- **Records**: 112 statements
- **Convention**: Same as bank1 but different column names
- **Date Format**: MM/DD/YYYY
- **Identifier Pattern**: TXN_001, TXN_002, etc.
- **Additional Field**: Transaction descriptions

**Purpose**: Tests parser flexibility with different bank formats and column naming conventions.

## Edge Case Datasets

### duplicate_transactions.csv / duplicate_statements.csv
**Purpose**: Test duplicate detection and handling
- **Scenarios**: Same amount, same day, slightly different times
- **Use Case**: Identifying potential duplicate entries that need manual review

### same_day_multiple.csv
**Purpose**: Test multiple transactions on the same date
- **Scenarios**: 7 transactions on 2024-01-15 with different times
- **Use Case**: Testing matching algorithms when multiple candidates exist

### large_amounts.csv
**Purpose**: Test handling of large monetary values
- **Scenarios**: $100K to $50M transactions
- **Use Case**: Performance and precision testing with large numbers

### boundary_dates.csv
**Purpose**: Test date boundary conditions
- **Scenarios**: End of year, leap year, month boundaries
- **Use Case**: Date parsing and comparison edge cases

### timezone_variations.csv
**Purpose**: Test timezone handling
- **Scenarios**: Same logical time in different timezone formats
- **Use Case**: Ensuring timezone normalization works correctly

### partial_matches.csv / partial_match_statements.csv
**Purpose**: Test partial matching scenarios
- **Scenarios**: Large transactions that might be split across multiple bank entries
- **Use Case**: Advanced matching algorithms for complex scenarios

## Test Scenarios Covered

### Perfect Matches (85%)
- Exact amount and date matches
- Transaction type alignment (CREDIT/DEBIT with positive/negative amounts)
- Same-day transactions with exact amounts

### Close Matches (10%)
- Amount differences within tolerance (typically 0.1% to 2%)
- Date differences within tolerance (1-3 days)
- Rounding differences in amounts

### Unmatched Records (5%)
- System transactions without corresponding bank statements
- Bank statements without corresponding system transactions
- Large discrepancies that fall outside tolerance ranges

### Edge Cases
- **Duplicate Detection**: Multiple similar transactions requiring careful analysis
- **Same-Day Matching**: Multiple transactions on the same date requiring smart pairing
- **Large Amounts**: Testing precision and performance with significant monetary values
- **Date Boundaries**: Year-end, month-end, and leap year scenarios
- **Timezone Handling**: Cross-timezone transaction matching
- **Partial Matching**: Complex scenarios where amounts might be split or combined

## Usage Examples

### Basic Reconciliation Test
```bash
./reconciler reconcile \
  --system-file testdata/csv/system_transactions.csv \
  --bank-files testdata/csv/bank_statement_bank1.csv \
  --output-format console
```

### Multi-Bank Format Test
```bash
./reconciler reconcile \
  --system-file testdata/csv/system_transactions.csv \
  --bank-files testdata/csv/bank_statement_bank1.csv,testdata/csv/bank_statement_bank2.csv \
  --output-format json
```

### Edge Case Testing
```bash
./reconciler reconcile \
  --system-file testdata/csv/edge_cases/duplicate_transactions.csv \
  --bank-files testdata/csv/edge_cases/duplicate_statements.csv \
  --output-format console
```

## Expected Outcomes

### Match Rate Expectations
- **Perfect Matches**: ~70-80% of transactions
- **Close Matches**: ~10-15% of transactions  
- **Fuzzy Matches**: ~5-10% of transactions
- **Unmatched**: ~5-10% of transactions

### Performance Expectations
- **Load Time**: <1 second for main datasets
- **Processing Time**: <2 seconds for reconciliation
- **Memory Usage**: <50MB for main datasets

## Data Quality Standards

### Transaction Data
- All amounts are positive decimal values
- Transaction types are either CREDIT or DEBIT
- Times are valid RFC3339 format
- Transaction IDs are unique within file

### Bank Statement Data
- Amounts may be negative (debit convention)
- Dates are valid and within reasonable range
- Identifiers are unique within file
- No missing required fields

### File Format
- Valid CSV format with proper headers
- UTF-8 encoding
- Unix line endings (LF)
- No trailing spaces or empty lines

## Validation

The test data has been validated to ensure:
1. **Format Compliance**: All CSV files parse correctly
2. **Data Integrity**: No missing or malformed fields
3. **Scenario Coverage**: All reconciliation features are testable
4. **Realistic Patterns**: Data represents realistic banking scenarios
5. **Edge Case Coverage**: Comprehensive edge case representation

## Maintenance

When updating test data:
1. Maintain the existing file formats and naming conventions
2. Ensure new data covers additional scenarios without breaking existing ones
3. Update expected match rates if significant changes are made
4. Run validation scripts to ensure data quality
5. Update this documentation to reflect changes

For questions or issues with test data, refer to the validation scripts in the `validators/` directory or contact the development team.