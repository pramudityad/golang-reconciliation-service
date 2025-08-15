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

// Generator represents a data generator command
type Generator struct {
	Name        string
	Command     string
	Description string
}

var generators = []Generator{
	{
		Name:        "transactions",
		Command:     "transaction_generator",
		Description: "Generate system transaction CSV files",
	},
	{
		Name:        "statements",
		Command:     "statement_generator",
		Description: "Generate bank statement CSV files",
	},
	{
		Name:        "scenarios",
		Command:     "scenario_generator",
		Description: "Generate specific test scenario datasets",
	},
}

func main() {
	var (
		generator = flag.String("generator", "", "Generator to run: transactions, statements, scenarios, or 'all'")
		list      = flag.Bool("list", false, "List available generators")
		outputDir = flag.String("output-dir", "../generated", "Output directory for generated files")
		help      = flag.Bool("help", false, "Show help for specific generator")
	)
	flag.Parse()

	if *list {
		listGenerators()
		return
	}

	if *generator == "" {
		fmt.Println("Test Data Generator CLI")
		fmt.Println("======================")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go run generate.go -generator=<name> [options]")
		fmt.Println()
		fmt.Println("Available generators:")
		for _, gen := range generators {
			fmt.Printf("  %-12s %s\n", gen.Name, gen.Description)
		}
		fmt.Println()
		fmt.Println("Use -list to see all generators")
		fmt.Println("Use -help -generator=<name> to see generator-specific options")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run generate.go -generator=transactions -count=1000 -output=large_transactions.csv")
		fmt.Println("  go run generate.go -generator=statements -format=bank2 -match-ratio=0.9")
		fmt.Println("  go run generate.go -generator=scenarios -scenario=all")
		fmt.Println("  go run generate.go -generator=all")
		return
	}

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	if *help {
		showGeneratorHelp(*generator)
		return
	}

	if *generator == "all" {
		generateAll(*outputDir)
		return
	}

	// Find and run specific generator
	for _, gen := range generators {
		if gen.Name == *generator {
			runGenerator(gen, *outputDir, flag.Args())
			return
		}
	}

	log.Fatalf("Unknown generator: %s", *generator)
}

func listGenerators() {
	fmt.Println("Available Test Data Generators:")
	fmt.Println("===============================")
	fmt.Println()
	
	for _, gen := range generators {
		fmt.Printf("Name: %s\n", gen.Name)
		fmt.Printf("Description: %s\n", gen.Description)
		fmt.Printf("Command: %s\n", gen.Command)
		fmt.Println()
	}
}

func showGeneratorHelp(generatorName string) {
	for _, gen := range generators {
		if gen.Name == generatorName {
			fmt.Printf("Help for %s generator:\n", generatorName)
			fmt.Printf("======================\n\n")
			
			// Run the generator with -help flag
			cmd := exec.Command("go", "run", gen.Command+".go", "-help")
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("Failed to get help for %s: %v", generatorName, err)
				return
			}
			
			fmt.Println(string(output))
			return
		}
	}
	
	log.Fatalf("Unknown generator: %s", generatorName)
}

func runGenerator(gen Generator, outputDir string, args []string) {
	fmt.Printf("Running %s generator...\n", gen.Name)
	
	// Prepare command arguments
	cmdArgs := []string{"run", gen.Command + ".go"}
	
	// Add output directory argument for scenarios generator
	if gen.Name == "scenarios" {
		cmdArgs = append(cmdArgs, "-output-dir="+outputDir)
	}
	
	// Add additional arguments passed from command line
	cmdArgs = append(cmdArgs, args...)
	
	// Execute the generator
	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to run %s generator: %v", gen.Name, err)
	}
	
	fmt.Printf("✓ %s generator completed successfully\n", gen.Name)
}

func generateAll(outputDir string) {
	fmt.Println("Generating comprehensive test dataset...")
	fmt.Println("======================================")
	fmt.Println()
	
	seed := time.Now().UnixNano()
	fmt.Printf("Using seed: %d\n\n", seed)
	
	// Create subdirectories
	dirs := []string{
		filepath.Join(outputDir, "transactions"),
		filepath.Join(outputDir, "statements"),
		filepath.Join(outputDir, "scenarios"),
		filepath.Join(outputDir, "performance"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	
	// Generate transactions
	fmt.Println("1. Generating transaction datasets...")
	generateTransactionSets(outputDir, seed)
	
	// Generate statements
	fmt.Println("\n2. Generating statement datasets...")
	generateStatementSets(outputDir, seed)
	
	// Generate scenarios
	fmt.Println("\n3. Generating scenario datasets...")
	generateScenarioSets(outputDir, seed)
	
	// Generate performance datasets
	fmt.Println("\n4. Generating performance datasets...")
	generatePerformanceSets(outputDir, seed)
	
	// Generate documentation
	fmt.Println("\n5. Generating documentation...")
	generateDocumentation(outputDir)
	
	fmt.Println("\n✓ All generators completed successfully!")
	fmt.Printf("Generated files are in: %s\n", outputDir)
}

func generateTransactionSets(outputDir string, seed int64) {
	txDir := filepath.Join(outputDir, "transactions")
	
	sets := []struct {
		name    string
		count   int
		pattern string
		desc    string
	}{
		{"small_random.csv", 100, "random", "Small random dataset"},
		{"medium_random.csv", 1000, "random", "Medium random dataset"},
		{"large_random.csv", 10000, "random", "Large random dataset"},
		{"business_hours.csv", 1000, "business-hours", "Business hours pattern"},
		{"end_of_month.csv", 500, "end-of-month", "End-of-month pattern"},
		{"large_amounts.csv", 200, "large-amounts", "Large amount transactions"},
		{"micro_transactions.csv", 1000, "micro-transactions", "Micro transactions"},
	}
	
	for _, set := range sets {
		fmt.Printf("  Generating %s (%s)...\n", set.name, set.desc)
		
		outputPath := filepath.Join(txDir, set.name)
		cmd := exec.Command("go", "run", "transaction_generator.go",
			"-output="+outputPath,
			"-count="+fmt.Sprintf("%d", set.count),
			"-pattern="+set.pattern,
			"-seed="+fmt.Sprintf("%d", seed),
		)
		
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to generate %s: %v", set.name, err)
		}
	}
}

func generateStatementSets(outputDir string, seed int64) {
	stmtDir := filepath.Join(outputDir, "statements")
	
	sets := []struct {
		name       string
		count      int
		format     string
		matchRatio float64
		desc       string
	}{
		{"bank1_format.csv", 1000, "bank1", 0.85, "Bank1 format with 85% matches"},
		{"bank2_format.csv", 1000, "bank2", 0.85, "Bank2 format with 85% matches"},
		{"custom_format.csv", 1000, "custom", 0.85, "Custom format with 85% matches"},
		{"high_match_rate.csv", 500, "bank1", 0.95, "High match rate dataset"},
		{"low_match_rate.csv", 500, "bank1", 0.60, "Low match rate dataset"},
		{"mismatched_only.csv", 200, "bank1", 0.0, "Intentionally mismatched"},
	}
	
	for _, set := range sets {
		fmt.Printf("  Generating %s (%s)...\n", set.name, set.desc)
		
		outputPath := filepath.Join(stmtDir, set.name)
		cmd := exec.Command("go", "run", "statement_generator.go",
			"-output="+outputPath,
			"-count="+fmt.Sprintf("%d", set.count),
			"-format="+set.format,
			"-match-ratio="+fmt.Sprintf("%.2f", set.matchRatio),
			"-seed="+fmt.Sprintf("%d", seed),
		)
		
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to generate %s: %v", set.name, err)
		}
	}
}

func generateScenarioSets(outputDir string, seed int64) {
	scenarioDir := filepath.Join(outputDir, "scenarios")
	
	fmt.Printf("  Generating all scenario datasets...\n")
	
	cmd := exec.Command("go", "run", "scenario_generator.go",
		"-output-dir="+scenarioDir,
		"-scenario=all",
		"-seed="+fmt.Sprintf("%d", seed),
	)
	
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to generate scenarios: %v", err)
	}
}

func generatePerformanceSets(outputDir string, seed int64) {
	perfDir := filepath.Join(outputDir, "performance")
	
	sets := []struct {
		name  string
		count int
		desc  string
	}{
		{"stress_test_10k.csv", 10000, "10K transactions for stress testing"},
		{"stress_test_50k.csv", 50000, "50K transactions for load testing"},
		{"stress_test_100k.csv", 100000, "100K transactions for extreme load testing"},
	}
	
	for _, set := range sets {
		fmt.Printf("  Generating %s (%s)...\n", set.name, set.desc)
		
		outputPath := filepath.Join(perfDir, set.name)
		cmd := exec.Command("go", "run", "transaction_generator.go",
			"-output="+outputPath,
			"-count="+fmt.Sprintf("%d", set.count),
			"-pattern=random",
			"-seed="+fmt.Sprintf("%d", seed),
		)
		
		if err := cmd.Run(); err != nil {
			log.Printf("Failed to generate %s: %v", set.name, err)
		}
	}
}

func generateDocumentation(outputDir string) {
	docContent := `# Generated Test Data

This directory contains automatically generated test data for the reconciliation service.

## Directory Structure

- **transactions/**: Various transaction datasets with different patterns
- **statements/**: Bank statement datasets in different formats
- **scenarios/**: Specific test scenarios (duplicates, edge cases, etc.)
- **performance/**: Large datasets for performance and stress testing

## File Descriptions

### Transactions
- small_random.csv: 100 random transactions
- medium_random.csv: 1,000 random transactions  
- large_random.csv: 10,000 random transactions
- business_hours.csv: Transactions concentrated during business hours
- end_of_month.csv: Transactions concentrated at month-end
- large_amounts.csv: High-value transactions
- micro_transactions.csv: Very small amount transactions

### Statements
- bank1_format.csv: Standard bank format (unique_identifier, amount, date)
- bank2_format.csv: Alternative format (transaction_id, transaction_amount, posting_date, transaction_description)
- custom_format.csv: Custom format example
- high_match_rate.csv: 95% match rate with transactions
- low_match_rate.csv: 60% match rate with transactions
- mismatched_only.csv: Intentionally non-matching statements

### Scenarios
- duplicate_*: Duplicate detection testing
- same_day_*: Multiple transactions on same date
- partial_match_*: Partial matching scenarios
- large_amount_*: Large value edge cases
- boundary_date_*: Date boundary conditions
- timezone_*: Timezone handling
- precision_*: Decimal precision edge cases
- performance_*: Large datasets for performance testing
- accuracy_*: Known match datasets for accuracy validation

### Performance
- stress_test_10k.csv: 10,000 transactions
- stress_test_50k.csv: 50,000 transactions
- stress_test_100k.csv: 100,000 transactions

## Usage

Use these datasets to test different aspects of the reconciliation engine:

1. **Functional Testing**: Use small and medium datasets
2. **Performance Testing**: Use large datasets and performance folder
3. **Edge Case Testing**: Use scenario-specific datasets
4. **Format Testing**: Use different statement formats
5. **Accuracy Testing**: Use accuracy datasets with known expected matches

## Regeneration

To regenerate all test data:
` + "```bash\ngo run generate.go -generator=all\n```" + `

To generate specific datasets:
` + "```bash\ngo run generate.go -generator=transactions -count=5000\ngo run generate.go -generator=statements -format=bank2\ngo run generate.go -generator=scenarios -scenario=duplicates\n```" + `

Generated on: ` + time.Now().Format("2006-01-02 15:04:05") + `
`

	docPath := filepath.Join(outputDir, "README.md")
	if err := os.WriteFile(docPath, []byte(docContent), 0644); err != nil {
		log.Printf("Failed to write documentation: %v", err)
	} else {
		fmt.Printf("  Generated README.md\n")
	}
}