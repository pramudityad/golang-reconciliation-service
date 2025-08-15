package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// MasterValidator orchestrates all validation tools
type MasterValidator struct {
	DataDir     string
	OutputDir   string
	Verbose     bool
	Strict      bool
	ReportFile  string
}

// ValidationSuite represents a complete validation suite
type ValidationSuite struct {
	Name        string
	Description string
	Validators  []ValidatorSpec
}

// ValidatorSpec specifies a validator to run
type ValidatorSpec struct {
	Name        string
	Command     string
	Args        []string
	Required    bool
	Timeout     time.Duration
}

func main() {
	var (
		dataDir   = flag.String("data-dir", "../csv", "Directory containing test data")
		outputDir = flag.String("output-dir", "validation_reports", "Directory for validation reports")
		verbose   = flag.Bool("verbose", false, "Verbose output")
		strict    = flag.Bool("strict", false, "Strict validation mode")
		suite     = flag.String("suite", "full", "Validation suite: quick, full, or custom")
		report    = flag.String("report", "", "Master report file (optional)")
	)
	flag.Parse()

	validator := &MasterValidator{
		DataDir:    *dataDir,
		OutputDir:  *outputDir,
		Verbose:    *verbose,
		Strict:     *strict,
		ReportFile: *report,
	}

	fmt.Println("Master Test Data Validator")
	fmt.Println("=========================")
	fmt.Printf("Data directory: %s\n", *dataDir)
	fmt.Printf("Output directory: %s\n", *outputDir)
	fmt.Printf("Validation suite: %s\n", *suite)
	fmt.Printf("Strict mode: %v\n", *strict)
	fmt.Println()

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Define validation suites
	suites := validator.DefineValidationSuites()

	// Find and run the specified suite
	var selectedSuite *ValidationSuite
	for _, s := range suites {
		if s.Name == *suite {
			selectedSuite = &s
			break
		}
	}

	if selectedSuite == nil {
		fmt.Printf("Available validation suites:\n")
		for _, s := range suites {
			fmt.Printf("  %-10s %s\n", s.Name, s.Description)
		}
		log.Fatalf("Unknown validation suite: %s", *suite)
	}

	// Run the validation suite
	success := validator.RunValidationSuite(*selectedSuite)

	// Generate master report if requested
	if validator.ReportFile != "" {
		validator.GenerateMasterReport()
	}

	if !success {
		os.Exit(1)
	}
}

// DefineValidationSuites defines the available validation suites
func (mv *MasterValidator) DefineValidationSuites() []ValidationSuite {
	return []ValidationSuite{
		{
			Name:        "quick",
			Description: "Quick validation for development",
			Validators: []ValidatorSpec{
				{
					Name:     "data_format",
					Command:  "data_validator",
					Args:     []string{"-input=" + mv.DataDir, "-output=" + filepath.Join(mv.OutputDir, "data_validation.txt")},
					Required: true,
					Timeout:  30 * time.Second,
				},
				{
					Name:     "basic_scenarios",
					Command:  "scenario_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-scenario=exact_matches", "-output=" + filepath.Join(mv.OutputDir, "scenario_validation.txt")},
					Required: true,
					Timeout:  60 * time.Second,
				},
			},
		},
		{
			Name:        "full",
			Description: "Complete validation suite",
			Validators: []ValidatorSpec{
				{
					Name:     "data_format",
					Command:  "data_validator",
					Args:     []string{"-input=" + mv.DataDir, "-recursive", "-output=" + filepath.Join(mv.OutputDir, "data_validation.txt")},
					Required: true,
					Timeout:  2 * time.Minute,
				},
				{
					Name:     "scenario_coverage",
					Command:  "scenario_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-output=" + filepath.Join(mv.OutputDir, "scenario_validation.txt")},
					Required: true,
					Timeout:  3 * time.Minute,
				},
				{
					Name:     "reconciliation_e2e",
					Command:  "reconciliation_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-output=" + filepath.Join(mv.OutputDir, "reconciliation_validation.txt")},
					Required: true,
					Timeout:  5 * time.Minute,
				},
			},
		},
		{
			Name:        "performance",
			Description: "Performance-focused validation",
			Validators: []ValidatorSpec{
				{
					Name:     "data_format",
					Command:  "data_validator",
					Args:     []string{"-input=" + mv.DataDir, "-output=" + filepath.Join(mv.OutputDir, "data_validation.txt")},
					Required: true,
					Timeout:  30 * time.Second,
				},
				{
					Name:     "reconciliation_performance",
					Command:  "reconciliation_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-test=performance", "-output=" + filepath.Join(mv.OutputDir, "performance_validation.txt")},
					Required: false,
					Timeout:  10 * time.Minute,
				},
			},
		},
		{
			Name:        "edge_cases",
			Description: "Edge case and stress testing",
			Validators: []ValidatorSpec{
				{
					Name:     "data_format",
					Command:  "data_validator",
					Args:     []string{"-input=" + filepath.Join(mv.DataDir, "edge_cases"), "-recursive", "-output=" + filepath.Join(mv.OutputDir, "edge_case_data_validation.txt")},
					Required: true,
					Timeout:  60 * time.Second,
				},
				{
					Name:     "edge_case_scenarios",
					Command:  "scenario_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-scenario=duplicates", "-output=" + filepath.Join(mv.OutputDir, "edge_case_scenarios.txt")},
					Required: true,
					Timeout:  2 * time.Minute,
				},
				{
					Name:     "edge_case_reconciliation",
					Command:  "reconciliation_validator",
					Args:     []string{"-data-dir=" + mv.DataDir, "-test=duplicate_handling", "-output=" + filepath.Join(mv.OutputDir, "edge_case_reconciliation.txt")},
					Required: false,
					Timeout:  3 * time.Minute,
				},
			},
		},
	}
}

// RunValidationSuite runs a complete validation suite
func (mv *MasterValidator) RunValidationSuite(suite ValidationSuite) bool {
	fmt.Printf("Running validation suite: %s\n", suite.Name)
	fmt.Printf("Description: %s\n\n", suite.Description)

	allSuccess := true
	results := make(map[string]ValidationResult)

	for _, validator := range suite.Validators {
		fmt.Printf("Running %s validator...\n", validator.Name)
		
		result := mv.RunValidator(validator)
		results[validator.Name] = result

		if result.Success {
			fmt.Printf("✅ %s validation passed\n", validator.Name)
		} else {
			fmt.Printf("❌ %s validation failed\n", validator.Name)
			if validator.Required {
				allSuccess = false
			}
		}

		if mv.Verbose {
			fmt.Printf("   Duration: %v\n", result.Duration)
			if result.Output != "" {
				fmt.Printf("   Output: %s\n", result.Output)
			}
			if result.Error != "" {
				fmt.Printf("   Error: %s\n", result.Error)
			}
		}
		fmt.Println()
	}

	// Print suite summary
	mv.PrintSuiteSummary(suite, results)

	return allSuccess
}

// ValidationResult represents the result of running a validator
type ValidationResult struct {
	Success  bool
	Duration time.Duration
	Output   string
	Error    string
}

// RunValidator runs a single validator
func (mv *MasterValidator) RunValidator(spec ValidatorSpec) ValidationResult {
	result := ValidationResult{}
	startTime := time.Now()

	// Add common arguments
	args := append([]string{"run", spec.Command + ".go"}, spec.Args...)
	
	if mv.Verbose {
		args = append(args, "-verbose")
	}
	
	if mv.Strict {
		args = append(args, "-strict")
	}

	// Execute validator
	cmd := exec.Command("go", args...)
	cmd.Dir = "." // Run in validators directory

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(startTime)
	result.Output = string(output)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result
}

// PrintSuiteSummary prints a summary of the validation suite results
func (mv *MasterValidator) PrintSuiteSummary(suite ValidationSuite, results map[string]ValidationResult) {
	fmt.Printf("Validation Suite Summary: %s\n", suite.Name)
	fmt.Printf("========================%s\n", "="*len(suite.Name))

	totalValidators := len(suite.Validators)
	passedValidators := 0
	requiredPassed := 0
	requiredTotal := 0
	var totalDuration time.Duration

	for _, spec := range suite.Validators {
		result := results[spec.Name]
		totalDuration += result.Duration

		if result.Success {
			passedValidators++
		}

		if spec.Required {
			requiredTotal++
			if result.Success {
				requiredPassed++
			}
		}
	}

	fmt.Printf("Validators run: %d\n", totalValidators)
	fmt.Printf("Validators passed: %d\n", passedValidators)
	fmt.Printf("Required validators: %d/%d passed\n", requiredPassed, requiredTotal)
	fmt.Printf("Total duration: %v\n", totalDuration)

	overallSuccess := requiredPassed == requiredTotal
	if overallSuccess {
		fmt.Printf("Overall result: ✅ SUITE PASSED\n")
	} else {
		fmt.Printf("Overall result: ❌ SUITE FAILED\n")
	}

	// Detailed results
	fmt.Printf("\nDetailed Results:\n")
	for _, spec := range suite.Validators {
		result := results[spec.Name]
		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
		}
		
		required := ""
		if spec.Required {
			required = " (required)"
		}

		fmt.Printf("  %-20s %s %8v%s\n", spec.Name, status, result.Duration, required)
	}
	fmt.Println()
}

// GenerateMasterReport generates a comprehensive master report
func (mv *MasterValidator) GenerateMasterReport() {
	fmt.Printf("Generating master validation report...\n")

	reportPath := mv.ReportFile
	if reportPath == "" {
		reportPath = filepath.Join(mv.OutputDir, "master_validation_report.md")
	}

	file, err := os.Create(reportPath)
	if err != nil {
		log.Printf("Failed to create master report: %v", err)
		return
	}
	defer file.Close()

	// Write markdown report
	fmt.Fprintf(file, "# Test Data Validation Report\n\n")
	fmt.Fprintf(file, "**Generated**: %s  \n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**Data Directory**: %s  \n", mv.DataDir)
	fmt.Fprintf(file, "**Validation Mode**: %s  \n\n", map[bool]string{true: "Strict", false: "Standard"}[mv.Strict])

	// Include individual reports
	fmt.Fprintf(file, "## Individual Validation Reports\n\n")
	
	reportFiles := []struct {
		name string
		file string
		desc string
	}{
		{"Data Format Validation", "data_validation.txt", "CSV format and data quality validation"},
		{"Scenario Coverage", "scenario_validation.txt", "Test scenario coverage analysis"},
		{"End-to-End Reconciliation", "reconciliation_validation.txt", "Complete reconciliation workflow testing"},
	}

	for _, report := range reportFiles {
		reportFile := filepath.Join(mv.OutputDir, report.file)
		fmt.Fprintf(file, "### %s\n\n", report.name)
		fmt.Fprintf(file, "%s\n\n", report.desc)
		
		if content, err := os.ReadFile(reportFile); err == nil {
			fmt.Fprintf(file, "```\n%s\n```\n\n", string(content))
		} else {
			fmt.Fprintf(file, "*Report file not found: %s*\n\n", report.file)
		}
	}

	// Data quality summary
	fmt.Fprintf(file, "## Data Quality Summary\n\n")
	fmt.Fprintf(file, "- **File Format Compliance**: All CSV files must parse correctly\n")
	fmt.Fprintf(file, "- **Data Integrity**: No missing required fields or invalid data types\n")
	fmt.Fprintf(file, "- **Scenario Coverage**: All required reconciliation scenarios covered\n")
	fmt.Fprintf(file, "- **Performance**: Processing targets met for all dataset sizes\n\n")

	// Recommendations
	fmt.Fprintf(file, "## Recommendations\n\n")
	fmt.Fprintf(file, "1. **Regular Validation**: Run validation suite before each release\n")
	fmt.Fprintf(file, "2. **Performance Monitoring**: Track processing time trends\n")
	fmt.Fprintf(file, "3. **Data Updates**: Keep test data current with production patterns\n")
	fmt.Fprintf(file, "4. **Edge Case Testing**: Regularly add new edge cases based on production issues\n\n")

	// File inventory
	fmt.Fprintf(file, "## Test Data File Inventory\n\n")
	err = filepath.Walk(mv.DataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && filepath.Ext(path) == ".csv" {
			relPath, _ := filepath.Rel(mv.DataDir, path)
			fmt.Fprintf(file, "- `%s` (%d bytes)\n", relPath, info.Size())
		}
		
		return nil
	})

	fmt.Printf("Master report generated: %s\n", reportPath)
}

// Additional utility functions for enhanced validation

// ValidateGeneratedData ensures generated test data meets requirements
func (mv *MasterValidator) ValidateGeneratedData() bool {
	fmt.Println("Validating generated test data...")

	// Check for required files
	requiredFiles := []string{
		"system_transactions.csv",
		"bank_statement_bank1.csv",
		"bank_statement_bank2.csv",
	}

	for _, file := range requiredFiles {
		path := filepath.Join(mv.DataDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("❌ Required file missing: %s\n", file)
			return false
		}
		fmt.Printf("✅ Found required file: %s\n", file)
	}

	return true
}

// CheckSystemDependencies verifies that all required tools are available
func (mv *MasterValidator) CheckSystemDependencies() bool {
	fmt.Println("Checking system dependencies...")

	dependencies := []string{"go"}

	for _, dep := range dependencies {
		if _, err := exec.LookPath(dep); err != nil {
			fmt.Printf("❌ Missing dependency: %s\n", dep)
			return false
		}
		fmt.Printf("✅ Found dependency: %s\n", dep)
	}

	return true
}

// CleanupValidationResults removes old validation results
func (mv *MasterValidator) CleanupValidationResults() {
	fmt.Println("Cleaning up old validation results...")
	
	if err := os.RemoveAll(mv.OutputDir); err != nil {
		fmt.Printf("Warning: Failed to clean output directory: %v\n", err)
	}
	
	if err := os.MkdirAll(mv.OutputDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to recreate output directory: %v\n", err)
	}
}