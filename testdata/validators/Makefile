# Test Data Validators Makefile

.PHONY: help all quick full performance edge-cases clean install-deps validate

# Default target
help:
	@echo "Test Data Validators"
	@echo "==================="
	@echo ""
	@echo "Available targets:"
	@echo "  help          - Show this help message"
	@echo "  all           - Run complete validation suite"
	@echo "  quick         - Run quick validation for development"
	@echo "  full          - Run comprehensive validation"
	@echo "  performance   - Run performance-focused validation"
	@echo "  edge-cases    - Run edge case validation"
	@echo "  data-only     - Validate data format only"
	@echo "  scenarios-only - Validate scenario coverage only"
	@echo "  e2e-only      - Run end-to-end validation only"
	@echo "  clean         - Remove validation reports"
	@echo "  install-deps  - Install required dependencies"
	@echo ""
	@echo "Configuration:"
	@echo "  DATA_DIR      - Test data directory (default: ../csv)"
	@echo "  OUTPUT_DIR    - Output directory for reports (default: reports)"
	@echo "  VERBOSE       - Verbose output (default: false)"
	@echo "  STRICT        - Strict validation mode (default: false)"
	@echo ""
	@echo "Examples:"
	@echo "  make all                    # Run complete validation"
	@echo "  make quick VERBOSE=true     # Quick validation with verbose output"
	@echo "  make full STRICT=true       # Strict comprehensive validation"
	@echo "  make data-only DATA_DIR=../generated  # Validate generated data"

# Configuration
DATA_DIR ?= ../csv
OUTPUT_DIR ?= reports
VERBOSE ?= false
STRICT ?= false

# Derived flags
VERBOSE_FLAG := $(if $(filter true,$(VERBOSE)),-verbose,)
STRICT_FLAG := $(if $(filter true,$(STRICT)),-strict,)
COMMON_FLAGS := -data-dir=$(DATA_DIR) $(VERBOSE_FLAG) $(STRICT_FLAG)

# Check dependencies
check-deps:
	@command -v go >/dev/null 2>&1 || { echo "Go is required but not installed."; exit 1; }

# Install dependencies
install-deps: check-deps
	@echo "Installing Go dependencies..."
	@cd .. && go mod download
	@echo "Dependencies installed successfully"

# Create output directory
setup:
	@mkdir -p $(OUTPUT_DIR)

# Run all validations using master validator
all: check-deps setup
	@echo "Running complete validation suite..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=full $(VERBOSE_FLAG) $(STRICT_FLAG) -report=$(OUTPUT_DIR)/master_report.md
	@echo "✅ Complete validation finished. See $(OUTPUT_DIR)/master_report.md for full report."

# Quick validation for development
quick: check-deps setup
	@echo "Running quick validation..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=quick $(VERBOSE_FLAG) $(STRICT_FLAG)
	@echo "✅ Quick validation finished."

# Full comprehensive validation
full: check-deps setup
	@echo "Running comprehensive validation..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=full $(VERBOSE_FLAG) $(STRICT_FLAG) -report=$(OUTPUT_DIR)/comprehensive_report.md
	@echo "✅ Comprehensive validation finished."

# Performance validation
performance: check-deps setup
	@echo "Running performance validation..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=performance $(VERBOSE_FLAG) $(STRICT_FLAG)
	@echo "✅ Performance validation finished."

# Edge case validation
edge-cases: check-deps setup
	@echo "Running edge case validation..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=edge_cases $(VERBOSE_FLAG) $(STRICT_FLAG)
	@echo "✅ Edge case validation finished."

# Individual validator targets

# Data format validation only
data-only: check-deps setup
	@echo "Running data format validation..."
	@go run data_validator.go -input=$(DATA_DIR) -recursive -output=$(OUTPUT_DIR)/data_validation.txt $(VERBOSE_FLAG) $(STRICT_FLAG)
	@echo "✅ Data format validation finished. See $(OUTPUT_DIR)/data_validation.txt"

# Scenario coverage validation only
scenarios-only: check-deps setup
	@echo "Running scenario coverage validation..."
	@go run scenario_validator.go $(COMMON_FLAGS) -output=$(OUTPUT_DIR)/scenario_validation.txt
	@echo "✅ Scenario validation finished. See $(OUTPUT_DIR)/scenario_validation.txt"

# End-to-end reconciliation validation only
e2e-only: check-deps setup
	@echo "Running end-to-end reconciliation validation..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -output=$(OUTPUT_DIR)/reconciliation_validation.txt
	@echo "✅ End-to-end validation finished. See $(OUTPUT_DIR)/reconciliation_validation.txt"

# Specific scenario validations
validate-duplicates: check-deps setup
	@echo "Validating duplicate handling..."
	@go run scenario_validator.go $(COMMON_FLAGS) -scenario=duplicate_detection -output=$(OUTPUT_DIR)/duplicates_validation.txt

validate-same-day: check-deps setup
	@echo "Validating same-day transactions..."
	@go run scenario_validator.go $(COMMON_FLAGS) -scenario=same_day_multiple -output=$(OUTPUT_DIR)/same_day_validation.txt

validate-large-amounts: check-deps setup
	@echo "Validating large amounts..."
	@go run scenario_validator.go $(COMMON_FLAGS) -scenario=large_amounts -output=$(OUTPUT_DIR)/large_amounts_validation.txt

validate-formats: check-deps setup
	@echo "Validating format variations..."
	@go run scenario_validator.go $(COMMON_FLAGS) -scenario=format_variations -output=$(OUTPUT_DIR)/formats_validation.txt

# Specific reconciliation tests
test-basic: check-deps setup
	@echo "Testing basic reconciliation..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -test=basic_reconciliation -output=$(OUTPUT_DIR)/basic_test.txt

test-multi-bank: check-deps setup
	@echo "Testing multi-bank reconciliation..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -test=multi_bank_reconciliation -output=$(OUTPUT_DIR)/multi_bank_test.txt

test-performance: check-deps setup
	@echo "Testing reconciliation performance..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -test=performance -output=$(OUTPUT_DIR)/performance_test.txt

# Configuration-specific validations
validate-strict: check-deps setup
	@echo "Running validation with strict configuration..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -config=strict -output=$(OUTPUT_DIR)/strict_config_validation.txt

validate-relaxed: check-deps setup
	@echo "Running validation with relaxed configuration..."
	@go run reconciliation_validator.go $(COMMON_FLAGS) -config=relaxed -output=$(OUTPUT_DIR)/relaxed_config_validation.txt

# File-specific validations
validate-file:
	@if [ -z "$(FILE)" ]; then echo "Usage: make validate-file FILE=path/to/file.csv"; exit 1; fi
	@echo "Validating file: $(FILE)"
	@go run data_validator.go -input=$(FILE) $(VERBOSE_FLAG) $(STRICT_FLAG)

validate-directory:
	@if [ -z "$(DIR)" ]; then echo "Usage: make validate-directory DIR=path/to/directory"; exit 1; fi
	@echo "Validating directory: $(DIR)"
	@go run data_validator.go -input=$(DIR) -recursive $(VERBOSE_FLAG) $(STRICT_FLAG)

# Report generation
report: setup
	@echo "Generating comprehensive validation report..."
	@go run validate_all.go -data-dir=$(DATA_DIR) -output-dir=$(OUTPUT_DIR) -suite=full -report=$(OUTPUT_DIR)/validation_report.md $(VERBOSE_FLAG) $(STRICT_FLAG)
	@echo "✅ Report generated: $(OUTPUT_DIR)/validation_report.md"

# Summary targets
summary:
	@echo "Validation Summary"
	@echo "=================="
	@if [ -d "$(OUTPUT_DIR)" ]; then \
		echo "Reports available in $(OUTPUT_DIR):"; \
		ls -la $(OUTPUT_DIR)/*.txt $(OUTPUT_DIR)/*.md 2>/dev/null || echo "  No reports found"; \
	else \
		echo "No validation reports found. Run 'make all' to generate reports."; \
	fi

status:
	@echo "Validation Status"
	@echo "================="
	@echo "Data directory: $(DATA_DIR)"
	@echo "Output directory: $(OUTPUT_DIR)"
	@echo "Verbose mode: $(VERBOSE)"
	@echo "Strict mode: $(STRICT)"
	@echo ""
	@if [ -d "$(DATA_DIR)" ]; then \
		echo "Data files found:"; \
		find $(DATA_DIR) -name "*.csv" -type f | wc -l | xargs echo "  CSV files:"; \
	else \
		echo "❌ Data directory not found: $(DATA_DIR)"; \
	fi

# Continuous integration targets
ci-quick: clean quick
	@echo "✅ CI quick validation completed"

ci-full: clean all
	@echo "✅ CI full validation completed"

# Watch for changes (requires fswatch or inotify-tools)
watch:
	@echo "Watching $(DATA_DIR) for changes..."
	@if command -v fswatch >/dev/null 2>&1; then \
		fswatch -o $(DATA_DIR) | xargs -n1 -I{} make quick; \
	elif command -v inotifywait >/dev/null 2>&1; then \
		while inotifywait -r -e modify,create,delete $(DATA_DIR); do make quick; done; \
	else \
		echo "❌ Neither fswatch nor inotifywait found. Install one to use watch functionality."; \
	fi

# Benchmarking
benchmark: check-deps setup
	@echo "Running validation benchmarks..."
	@time make data-only VERBOSE=false > $(OUTPUT_DIR)/benchmark_data.log 2>&1
	@time make scenarios-only VERBOSE=false > $(OUTPUT_DIR)/benchmark_scenarios.log 2>&1
	@time make e2e-only VERBOSE=false > $(OUTPUT_DIR)/benchmark_e2e.log 2>&1
	@echo "✅ Benchmark results saved to $(OUTPUT_DIR)/benchmark_*.log"

# Clean up generated reports
clean:
	@echo "Cleaning validation reports..."
	@rm -rf $(OUTPUT_DIR)
	@echo "✅ Validation reports cleaned"

# Clean everything including temp files
clean-all: clean
	@echo "Cleaning all temporary files..."
	@rm -f *.tmp *.log
	@echo "✅ All temporary files cleaned"

# Development helpers
dev-setup: install-deps
	@echo "Setting up development environment..."
	@mkdir -p $(OUTPUT_DIR)
	@echo "✅ Development environment ready"

# Validate generated test data
validate-generated:
	@echo "Validating generated test data..."
	@make data-only DATA_DIR=../generated VERBOSE=true

# Integration with test data generators
generate-and-validate:
	@echo "Generating test data and validating..."
	@cd ../generators && make all
	@make validate-generated
	@echo "✅ Test data generated and validated"

# Help for specific validators
help-data:
	@go run data_validator.go -help

help-scenarios:
	@go run scenario_validator.go -help

help-reconciliation:
	@go run reconciliation_validator.go -help

help-master:
	@go run validate_all.go -help

# Create validation configuration
config:
	@echo "Creating validation configuration..."
	@printf '# Validation Configuration\n' > validation_config.env
	@printf 'DATA_DIR=../csv\n' >> validation_config.env
	@printf 'OUTPUT_DIR=reports\n' >> validation_config.env
	@printf 'VERBOSE=false\n' >> validation_config.env
	@printf 'STRICT=false\n\n' >> validation_config.env
	@printf '# Timeout settings (seconds)\n' >> validation_config.env
	@printf 'DATA_VALIDATION_TIMEOUT=120\n' >> validation_config.env
	@printf 'SCENARIO_VALIDATION_TIMEOUT=180\n' >> validation_config.env
	@printf 'E2E_VALIDATION_TIMEOUT=300\n\n' >> validation_config.env
	@printf '# Performance thresholds\n' >> validation_config.env
	@printf 'MAX_PROCESSING_TIME_1K=2\n' >> validation_config.env
	@printf 'MAX_PROCESSING_TIME_10K=10\n' >> validation_config.env
	@printf 'MIN_TRANSACTION_RATE=100\n\n' >> validation_config.env
	@printf '# Quality thresholds\n' >> validation_config.env
	@printf 'MIN_MATCH_RATE=80\n' >> validation_config.env
	@printf 'MAX_ERROR_RATE=5\n' >> validation_config.env
	@printf 'MIN_COVERAGE_PERCENT=95\n' >> validation_config.env
	@echo "Configuration created: validation_config.env"

# Load configuration if it exists
-include validation_config.env