# Test Data Generator Makefile

.PHONY: help all clean transactions statements scenarios performance install-deps

# Default target
help:
	@echo "Test Data Generator"
	@echo "==================="
	@echo ""
	@echo "Available targets:"
	@echo "  help        - Show this help message"
	@echo "  all         - Generate all test datasets"
	@echo "  transactions - Generate transaction datasets only"
	@echo "  statements  - Generate statement datasets only"
	@echo "  scenarios   - Generate scenario datasets only"
	@echo "  performance - Generate performance test datasets"
	@echo "  clean       - Remove all generated files"
	@echo "  install-deps - Install required Go dependencies"
	@echo ""
	@echo "Examples:"
	@echo "  make all                    # Generate complete test suite"
	@echo "  make transactions COUNT=5000 # Generate 5000 transactions"
	@echo "  make statements FORMAT=bank2 # Generate bank2 format statements"
	@echo "  make scenarios SCENARIO=duplicates # Generate duplicate scenarios only"
	@echo ""
	@echo "Environment variables:"
	@echo "  OUTPUT_DIR  - Output directory (default: ../generated)"
	@echo "  COUNT       - Number of records to generate (default: 1000)"
	@echo "  FORMAT      - Statement format: bank1, bank2, custom (default: bank1)"
	@echo "  SCENARIO    - Scenario type: all, duplicates, same-day, etc. (default: all)"
	@echo "  SEED        - Random seed for reproducible generation (default: current time)"

# Configuration
OUTPUT_DIR ?= ../generated
COUNT ?= 1000
FORMAT ?= bank1
SCENARIO ?= all
SEED ?= $(shell date +%s)

# Check for required dependencies
check-deps:
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed. Please install Go first."; exit 1; }

# Install Go dependencies
install-deps: check-deps
	@echo "Installing Go dependencies..."
	@go mod download
	@echo "Dependencies installed successfully"

# Generate all test data
all: check-deps
	@echo "Generating comprehensive test dataset..."
	@mkdir -p $(OUTPUT_DIR)
	@go run generate.go -generator=all -output-dir=$(OUTPUT_DIR)
	@echo "✓ All test data generated in $(OUTPUT_DIR)"

# Generate transaction datasets
transactions: check-deps
	@echo "Generating transaction datasets..."
	@mkdir -p $(OUTPUT_DIR)/transactions
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/transactions/transactions_$(COUNT).csv \
		-count=$(COUNT) \
		-seed=$(SEED)
	@echo "✓ Transaction dataset generated: $(OUTPUT_DIR)/transactions/transactions_$(COUNT).csv"

# Generate statement datasets
statements: check-deps
	@echo "Generating statement datasets..."
	@mkdir -p $(OUTPUT_DIR)/statements
	@go run statement_generator.go \
		-output=$(OUTPUT_DIR)/statements/statements_$(FORMAT)_$(COUNT).csv \
		-format=$(FORMAT) \
		-count=$(COUNT) \
		-seed=$(SEED)
	@echo "✓ Statement dataset generated: $(OUTPUT_DIR)/statements/statements_$(FORMAT)_$(COUNT).csv"

# Generate scenario datasets
scenarios: check-deps
	@echo "Generating scenario datasets..."
	@mkdir -p $(OUTPUT_DIR)/scenarios
	@go run scenario_generator.go \
		-output-dir=$(OUTPUT_DIR)/scenarios \
		-scenario=$(SCENARIO) \
		-seed=$(SEED)
	@echo "✓ Scenario datasets generated in $(OUTPUT_DIR)/scenarios"

# Generate performance test datasets
performance: check-deps
	@echo "Generating performance test datasets..."
	@mkdir -p $(OUTPUT_DIR)/performance
	@for size in 1000 5000 10000 25000 50000; do \
		echo "  Generating $$size record dataset..."; \
		go run transaction_generator.go \
			-output=$(OUTPUT_DIR)/performance/perf_transactions_$${size}.csv \
			-count=$$size \
			-seed=$(SEED); \
		go run statement_generator.go \
			-output=$(OUTPUT_DIR)/performance/perf_statements_$${size}.csv \
			-count=$$size \
			-format=bank1 \
			-match-ratio=0.85 \
			-seed=$(SEED); \
	done
	@echo "✓ Performance datasets generated in $(OUTPUT_DIR)/performance"

# Quick datasets for development
quick: check-deps
	@echo "Generating quick development datasets..."
	@mkdir -p $(OUTPUT_DIR)/quick
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/quick/dev_transactions.csv \
		-count=100 \
		-seed=$(SEED)
	@go run statement_generator.go \
		-output=$(OUTPUT_DIR)/quick/dev_statements.csv \
		-count=100 \
		-format=bank1 \
		-match-ratio=0.8 \
		-seed=$(SEED)
	@echo "✓ Quick development datasets generated in $(OUTPUT_DIR)/quick"

# Generate specific patterns
pattern-business: check-deps
	@mkdir -p $(OUTPUT_DIR)/patterns
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/patterns/business_hours_$(COUNT).csv \
		-count=$(COUNT) \
		-pattern=business-hours \
		-seed=$(SEED)

pattern-endmonth: check-deps
	@mkdir -p $(OUTPUT_DIR)/patterns
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/patterns/end_of_month_$(COUNT).csv \
		-count=$(COUNT) \
		-pattern=end-of-month \
		-seed=$(SEED)

pattern-large: check-deps
	@mkdir -p $(OUTPUT_DIR)/patterns
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/patterns/large_amounts_$(COUNT).csv \
		-count=$(COUNT) \
		-pattern=large-amounts \
		-seed=$(SEED)

pattern-micro: check-deps
	@mkdir -p $(OUTPUT_DIR)/patterns
	@go run transaction_generator.go \
		-output=$(OUTPUT_DIR)/patterns/micro_transactions_$(COUNT).csv \
		-count=$(COUNT) \
		-pattern=micro-transactions \
		-seed=$(SEED)

# Generate specific scenarios
scenario-duplicates: check-deps
	@mkdir -p $(OUTPUT_DIR)/scenarios
	@go run scenario_generator.go -output-dir=$(OUTPUT_DIR)/scenarios -scenario=duplicates -seed=$(SEED)

scenario-sameday: check-deps
	@mkdir -p $(OUTPUT_DIR)/scenarios
	@go run scenario_generator.go -output-dir=$(OUTPUT_DIR)/scenarios -scenario=same-day -seed=$(SEED)

scenario-partial: check-deps
	@mkdir -p $(OUTPUT_DIR)/scenarios
	@go run scenario_generator.go -output-dir=$(OUTPUT_DIR)/scenarios -scenario=partial -seed=$(SEED)

scenario-edge: check-deps
	@mkdir -p $(OUTPUT_DIR)/scenarios
	@go run scenario_generator.go -output-dir=$(OUTPUT_DIR)/scenarios -scenario=edge-cases -seed=$(SEED)

# Validation targets
validate: check-deps
	@echo "Validating generated data..."
	@if [ ! -d "$(OUTPUT_DIR)" ]; then \
		echo "No generated data found. Run 'make all' first."; \
		exit 1; \
	fi
	@echo "Checking file formats..."
	@for file in $(OUTPUT_DIR)/**/*.csv; do \
		if [ -f "$$file" ]; then \
			echo "  Validating $$file..."; \
			head -1 "$$file" | grep -q "," || { echo "Invalid CSV format in $$file"; exit 1; }; \
		fi; \
	done
	@echo "✓ All files validated successfully"

# Count records in generated files
count:
	@echo "Record counts in generated datasets:"
	@echo "===================================="
	@find $(OUTPUT_DIR) -name "*.csv" -type f | while read file; do \
		count=$$(tail -n +2 "$$file" | wc -l); \
		echo "$$(basename "$$file"): $$count records"; \
	done

# Show file sizes
sizes:
	@echo "File sizes of generated datasets:"
	@echo "================================="
	@find $(OUTPUT_DIR) -name "*.csv" -type f -exec ls -lh {} \; | awk '{print $$9 ": " $$5}'

# Test generators
test: check-deps
	@echo "Testing generators..."
	@mkdir -p test_output
	@go run transaction_generator.go -output=test_output/test_tx.csv -count=10
	@go run statement_generator.go -output=test_output/test_stmt.csv -count=10
	@go run scenario_generator.go -output-dir=test_output -scenario=duplicates
	@echo "✓ All generators working correctly"
	@rm -rf test_output

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@rm -rf $(OUTPUT_DIR)
	@echo "✓ Generated files cleaned"

# Clean test files
clean-test:
	@rm -rf test_output

# Show help for specific generators
help-transactions:
	@go run transaction_generator.go -help

help-statements:
	@go run statement_generator.go -help

help-scenarios:
	@go run scenario_generator.go -help

# Build standalone binaries
build: check-deps
	@echo "Building standalone generator binaries..."
	@mkdir -p bin
	@go build -o bin/transaction_generator transaction_generator.go
	@go build -o bin/statement_generator statement_generator.go
	@go build -o bin/scenario_generator scenario_generator.go
	@go build -o bin/generate generate.go
	@echo "✓ Binaries built in bin/ directory"

# Install binaries to GOPATH/bin
install: build
	@echo "Installing generators to GOPATH/bin..."
	@cp bin/* $(GOPATH)/bin/
	@echo "✓ Generators installed globally"

# Create sample configuration file
config:
	@echo "Creating sample configuration file..."
	@cat > generator_config.env << 'EOF'
# Test Data Generator Configuration
# Copy this file and modify as needed

# Output directory for generated files
OUTPUT_DIR=../generated

# Default record counts
TRANSACTION_COUNT=1000
STATEMENT_COUNT=1000

# Default formats
STATEMENT_FORMAT=bank1

# Random seed (leave empty for random)
SEED=

# Date ranges
START_DATE=2024-01-01
END_DATE=2024-12-31

# Amount ranges
MIN_AMOUNT=0.01
MAX_AMOUNT=50000.00

# Matching ratios
MATCH_RATIO=0.85
EOF
	@echo "✓ Configuration file created: generator_config.env"

# Show summary of what can be generated
summary:
	@echo "Test Data Generator Summary"
	@echo "=========================="
	@echo ""
	@echo "Transaction Patterns:"
	@echo "  - random: Evenly distributed random transactions"
	@echo "  - business-hours: Concentrated during 9-5 weekdays"
	@echo "  - end-of-month: Concentrated at month-end"
	@echo "  - large-amounts: High-value transactions"
	@echo "  - micro-transactions: Very small amounts"
	@echo ""
	@echo "Statement Formats:"
	@echo "  - bank1: unique_identifier, amount, date"
	@echo "  - bank2: transaction_id, transaction_amount, posting_date, transaction_description"
	@echo "  - custom: id, value, transaction_date, type, notes"
	@echo ""
	@echo "Scenarios:"
	@echo "  - duplicates: Duplicate detection testing"
	@echo "  - same-day: Multiple transactions per day"
	@echo "  - partial: Partial matching scenarios"
	@echo "  - edge-cases: Boundary conditions and edge cases"
	@echo ""
	@echo "Performance Datasets:"
	@echo "  - 1K, 5K, 10K, 25K, 50K record sets"
	@echo "  - Stress testing and load testing"