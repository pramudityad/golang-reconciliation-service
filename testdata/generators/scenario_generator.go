package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/shopspring/decimal"
)

// ScenarioGenerator creates specific test scenarios
type ScenarioGenerator struct {
	Seed      int64
	OutputDir string
}

func main() {
	var (
		outputDir = flag.String("output-dir", "generated_scenarios", "Output directory for scenario files")
		seed      = flag.Int64("seed", time.Now().UnixNano(), "Random seed for reproducible generation")
		scenario  = flag.String("scenario", "all", "Scenario to generate: all, duplicates, same-day, partial, edge-cases")
	)
	flag.Parse()

	// Create output directory
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	generator := &ScenarioGenerator{
		Seed:      *seed,
		OutputDir: *outputDir,
	}

	switch *scenario {
	case "duplicates":
		generator.GenerateDuplicateScenario()
	case "same-day":
		generator.GenerateSameDayScenario()
	case "partial":
		generator.GeneratePartialMatchScenario()
	case "edge-cases":
		generator.GenerateEdgeCaseScenarios()
	case "all":
		generator.GenerateAllScenarios()
	default:
		log.Fatalf("Unknown scenario: %s", *scenario)
	}

	fmt.Printf("Generated scenarios in %s\n", *outputDir)
	fmt.Printf("Seed used: %d\n", *seed)
}

// GenerateAllScenarios generates all predefined scenarios
func (sg *ScenarioGenerator) GenerateAllScenarios() {
	fmt.Println("Generating all scenarios...")
	sg.GenerateDuplicateScenario()
	sg.GenerateSameDayScenario()
	sg.GeneratePartialMatchScenario()
	sg.GenerateEdgeCaseScenarios()
	sg.GeneratePerformanceScenario()
	sg.GenerateAccuracyTestScenario()
}

// GenerateDuplicateScenario creates duplicate detection test data
func (sg *ScenarioGenerator) GenerateDuplicateScenario() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating duplicate detection scenario...")
	
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	// Create transactions with potential duplicates
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		// Exact duplicates
		{"DUP001", "100.00", "CREDIT", baseTime.Format(time.RFC3339)},
		{"DUP002", "100.00", "CREDIT", baseTime.Format(time.RFC3339)},
		{"DUP003", "100.00", "CREDIT", baseTime.Format(time.RFC3339)},
		// Time-based duplicates (same amount, close times)
		{"DUP004", "250.50", "DEBIT", baseTime.Add(5 * time.Minute).Format(time.RFC3339)},
		{"DUP005", "250.50", "DEBIT", baseTime.Add(7 * time.Minute).Format(time.RFC3339)},
		// Near duplicates (small amount differences)
		{"DUP006", "75.25", "CREDIT", baseTime.Add(1 * time.Hour).Format(time.RFC3339)},
		{"DUP007", "75.27", "CREDIT", baseTime.Add(1*time.Hour + 2*time.Minute).Format(time.RFC3339)},
		// False positives (same amount, different days)
		{"DUP008", "500.00", "DEBIT", baseTime.Add(24 * time.Hour).Format(time.RFC3339)},
		{"DUP009", "500.00", "DEBIT", baseTime.Add(48 * time.Hour).Format(time.RFC3339)},
	}
	
	sg.writeCSV("duplicate_transactions.csv", transactions)
	
	// Corresponding bank statements
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"DUPS001", "100.00", baseTime.Format("2006-01-02")},
		{"DUPS002", "100.00", baseTime.Format("2006-01-02")},
		{"DUPS003", "-250.50", baseTime.Format("2006-01-02")},
		{"DUPS004", "75.25", baseTime.Add(1 * time.Hour).Format("2006-01-02")},
		{"DUPS005", "-500.00", baseTime.Add(24 * time.Hour).Format("2006-01-02")},
		{"DUPS006", "-500.00", baseTime.Add(48 * time.Hour).Format("2006-01-02")},
	}
	
	sg.writeCSV("duplicate_statements.csv", statements)
}

// GenerateSameDayScenario creates same-day transaction test data
func (sg *ScenarioGenerator) GenerateSameDayScenario() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating same-day transaction scenario...")
	
	baseTime := time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)
	
	// Multiple transactions on the same day
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
	}
	
	amounts := []string{"50.00", "75.00", "25.00", "100.00", "200.00", "150.00", "300.00", "125.00"}
	types := []string{"CREDIT", "DEBIT", "CREDIT", "DEBIT", "CREDIT", "DEBIT", "CREDIT", "DEBIT"}
	
	for i := 0; i < len(amounts); i++ {
		txTime := baseTime.Add(time.Duration(i*2) * time.Hour)
		transactions = append(transactions, []string{
			fmt.Sprintf("SD%03d", i+1),
			amounts[i],
			types[i],
			txTime.Format(time.RFC3339),
		})
	}
	
	sg.writeCSV("same_day_transactions.csv", transactions)
	
	// Corresponding statements (some might be missing or combined)
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"SDS001", "50.00", baseTime.Format("2006-01-02")},
		{"SDS002", "-75.00", baseTime.Format("2006-01-02")},
		{"SDS003", "25.00", baseTime.Format("2006-01-02")},
		{"SDS004", "-100.00", baseTime.Format("2006-01-02")},
		{"SDS005", "200.00", baseTime.Format("2006-01-02")},
		// Missing 150.00 debit statement
		{"SDS006", "300.00", baseTime.Format("2006-01-02")},
		{"SDS007", "-125.00", baseTime.Format("2006-01-02")},
		// Extra statement not in transactions
		{"SDS008", "99.99", baseTime.Format("2006-01-02")},
	}
	
	sg.writeCSV("same_day_statements.csv", statements)
}

// GeneratePartialMatchScenario creates partial matching test data
func (sg *ScenarioGenerator) GeneratePartialMatchScenario() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating partial match scenario...")
	
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	// Large transactions that might be split
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		{"PM001", "1000.00", "CREDIT", baseTime.Format(time.RFC3339)},
		{"PM002", "2000.00", "DEBIT", baseTime.Add(1 * time.Hour).Format(time.RFC3339)},
		{"PM003", "500.00", "CREDIT", baseTime.Add(2 * time.Hour).Format(time.RFC3339)},
		{"PM004", "1500.00", "DEBIT", baseTime.Add(3 * time.Hour).Format(time.RFC3339)},
	}
	
	sg.writeCSV("partial_match_transactions.csv", transactions)
	
	// Bank statements that might represent partial amounts
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		// PM001 (1000.00) split into multiple deposits
		{"PMS001", "300.00", baseTime.Format("2006-01-02")},
		{"PMS002", "400.00", baseTime.Format("2006-01-02")},
		{"PMS003", "300.00", baseTime.Format("2006-01-02")},
		// PM002 (2000.00) split into multiple withdrawals
		{"PMS004", "-800.00", baseTime.Add(1 * time.Hour).Format("2006-01-02")},
		{"PMS005", "-600.00", baseTime.Add(1 * time.Hour).Format("2006-01-02")},
		{"PMS006", "-600.00", baseTime.Add(1 * time.Hour).Format("2006-01-02")},
		// PM003 (500.00) split into two
		{"PMS007", "250.00", baseTime.Add(2 * time.Hour).Format("2006-01-02")},
		{"PMS008", "250.00", baseTime.Add(2 * time.Hour).Format("2006-01-02")},
		// PM004 (1500.00) split unevenly
		{"PMS009", "-700.00", baseTime.Add(3 * time.Hour).Format("2006-01-02")},
		{"PMS010", "-800.00", baseTime.Add(3 * time.Hour).Format("2006-01-02")},
	}
	
	sg.writeCSV("partial_match_statements.csv", statements)
}

// GenerateEdgeCaseScenarios creates various edge case test data
func (sg *ScenarioGenerator) GenerateEdgeCaseScenarios() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating edge case scenarios...")
	
	// Large amounts
	sg.generateLargeAmountScenario()
	
	// Boundary dates
	sg.generateBoundaryDateScenario()
	
	// Timezone variations
	sg.generateTimezoneScenario()
	
	// Precision edge cases
	sg.generatePrecisionScenario()
}

func (sg *ScenarioGenerator) generateLargeAmountScenario() {
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		{"LA001", "100000.00", "CREDIT", "2024-01-15T10:00:00Z"},
		{"LA002", "250000.50", "DEBIT", "2024-01-16T14:00:00Z"},
		{"LA003", "999999.99", "CREDIT", "2024-01-17T09:00:00Z"},
		{"LA004", "1000000.00", "DEBIT", "2024-01-18T16:00:00Z"},
		{"LA005", "50000000.00", "CREDIT", "2024-01-19T11:00:00Z"},
	}
	
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"LAS001", "100000.00", "2024-01-15"},
		{"LAS002", "-250000.50", "2024-01-16"},
		{"LAS003", "999999.99", "2024-01-17"},
		{"LAS004", "-1000000.00", "2024-01-18"},
		{"LAS005", "50000000.00", "2024-01-19"},
	}
	
	sg.writeCSV("large_amount_transactions.csv", transactions)
	sg.writeCSV("large_amount_statements.csv", statements)
}

func (sg *ScenarioGenerator) generateBoundaryDateScenario() {
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		{"BD001", "100.00", "CREDIT", "2023-12-31T23:59:59Z"},
		{"BD002", "200.00", "DEBIT", "2024-01-01T00:00:00Z"},
		{"BD003", "300.00", "CREDIT", "2024-02-28T23:59:59Z"},
		{"BD004", "400.00", "DEBIT", "2024-02-29T00:00:00Z"},
		{"BD005", "500.00", "CREDIT", "2024-02-29T23:59:59Z"},
		{"BD006", "600.00", "DEBIT", "2024-03-01T00:00:00Z"},
		{"BD007", "700.00", "CREDIT", "2024-12-31T23:59:59Z"},
	}
	
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"BDS001", "100.00", "2023-12-31"},
		{"BDS002", "-200.00", "2024-01-01"},
		{"BDS003", "300.00", "2024-02-28"},
		{"BDS004", "-400.00", "2024-02-29"},
		{"BDS005", "500.00", "2024-02-29"},
		{"BDS006", "-600.00", "2024-03-01"},
		{"BDS007", "700.00", "2024-12-31"},
	}
	
	sg.writeCSV("boundary_date_transactions.csv", transactions)
	sg.writeCSV("boundary_date_statements.csv", statements)
}

func (sg *ScenarioGenerator) generateTimezoneScenario() {
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		{"TZ001", "100.00", "CREDIT", "2024-01-15T10:00:00Z"},
		{"TZ002", "200.00", "DEBIT", "2024-01-15T10:00:00-05:00"},
		{"TZ003", "300.00", "CREDIT", "2024-01-15T10:00:00+05:00"},
		{"TZ004", "400.00", "DEBIT", "2024-01-15T10:00:00-08:00"},
		{"TZ005", "500.00", "CREDIT", "2024-01-15T10:00:00+09:00"},
	}
	
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"TZS001", "100.00", "2024-01-15"},
		{"TZS002", "-200.00", "2024-01-15"},
		{"TZS003", "300.00", "2024-01-14"}, // Different day due to timezone
		{"TZS004", "-400.00", "2024-01-15"},
		{"TZS005", "500.00", "2024-01-15"},
	}
	
	sg.writeCSV("timezone_transactions.csv", transactions)
	sg.writeCSV("timezone_statements.csv", statements)
}

func (sg *ScenarioGenerator) generatePrecisionScenario() {
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
		{"PR001", "0.01", "CREDIT", "2024-01-15T10:00:00Z"},
		{"PR002", "0.99", "DEBIT", "2024-01-15T11:00:00Z"},
		{"PR003", "999.999", "CREDIT", "2024-01-15T12:00:00Z"},
		{"PR004", "123.456789", "DEBIT", "2024-01-15T13:00:00Z"},
		{"PR005", "1000000.001", "CREDIT", "2024-01-15T14:00:00Z"},
	}
	
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
		{"PRS001", "0.01", "2024-01-15"},
		{"PRS002", "-0.99", "2024-01-15"},
		{"PRS003", "999.999", "2024-01-15"},
		{"PRS004", "-123.46", "2024-01-15"}, // Rounded to 2 decimal places
		{"PRS005", "1000000.00", "2024-01-15"}, // Rounded
	}
	
	sg.writeCSV("precision_transactions.csv", transactions)
	sg.writeCSV("precision_statements.csv", statements)
}

// GeneratePerformanceScenario creates large datasets for performance testing
func (sg *ScenarioGenerator) GeneratePerformanceScenario() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating performance test scenario...")
	
	// Generate 10,000 transactions
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
	}
	
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	
	for i := 0; i < 10000; i++ {
		// Random time within year
		randomDays := rand.Intn(365)
		randomHours := rand.Intn(24)
		randomMinutes := rand.Intn(60)
		
		txTime := baseTime.AddDate(0, 0, randomDays).
			Add(time.Duration(randomHours) * time.Hour).
			Add(time.Duration(randomMinutes) * time.Minute)
		
		// Random amount
		amount := decimal.NewFromFloat(rand.Float64() * 10000).Round(2)
		
		txType := "CREDIT"
		if rand.Float64() < 0.4 {
			txType = "DEBIT"
		}
		
		transactions = append(transactions, []string{
			fmt.Sprintf("PERF%06d", i+1),
			amount.String(),
			txType,
			txTime.Format(time.RFC3339),
		})
	}
	
	sg.writeCSV("performance_transactions.csv", transactions)
	
	// Generate corresponding statements (with some variations)
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
	}
	
	for i := 1; i < len(transactions); i++ { // Skip header
		tx := transactions[i]
		amount, _ := decimal.NewFromString(tx[1])
		
		if tx[2] == "DEBIT" {
			amount = amount.Neg()
		}
		
		// 90% exact matches, 10% variations
		if rand.Float64() < 0.1 {
			variation := decimal.NewFromFloat((rand.Float64() - 0.5) * 0.02) // ±1%
			amount = amount.Add(amount.Mul(variation))
		}
		
		txTime, _ := time.Parse(time.RFC3339, tx[3])
		
		statements = append(statements, []string{
			fmt.Sprintf("PERFS%06d", i),
			amount.Round(2).String(),
			txTime.Format("2006-01-02"),
		})
	}
	
	sg.writeCSV("performance_statements.csv", statements)
}

// GenerateAccuracyTestScenario creates datasets with known correct matches
func (sg *ScenarioGenerator) GenerateAccuracyTestScenario() {
	rand.Seed(sg.Seed)
	
	fmt.Println("Generating accuracy test scenario...")
	
	baseTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	
	// Create transactions with known expected matches
	transactions := [][]string{
		{"trxID", "amount", "type", "transactionTime"},
	}
	
	statements := [][]string{
		{"unique_identifier", "amount", "date"},
	}
	
	expectedMatches := [][]string{
		{"transaction_id", "statement_id", "match_type", "confidence"},
	}
	
	// Generate 100 transaction pairs with known match types
	for i := 0; i < 100; i++ {
		txTime := baseTime.Add(time.Duration(i) * time.Hour)
		amount := decimal.NewFromFloat(100.0 + float64(i))
		
		txType := "CREDIT"
		stmtAmount := amount
		if i%3 == 0 {
			txType = "DEBIT"
			stmtAmount = amount.Neg()
		}
		
		txID := fmt.Sprintf("ACC%03d", i+1)
		stmtID := fmt.Sprintf("ACCS%03d", i+1)
		
		matchType := "exact"
		confidence := "1.0"
		
		// Introduce variations for different match types
		if i%10 == 1 { // Close matches (amount difference)
			variation := decimal.NewFromFloat(0.50)
			stmtAmount = stmtAmount.Add(variation)
			matchType = "close"
			confidence = "0.95"
		} else if i%10 == 2 { // Date differences
			txTime = txTime.Add(24 * time.Hour)
			matchType = "close"
			confidence = "0.90"
		} else if i%10 == 3 { // Fuzzy matches
			variation := decimal.NewFromFloat(2.00)
			stmtAmount = stmtAmount.Add(variation)
			txTime = txTime.Add(48 * time.Hour)
			matchType = "fuzzy"
			confidence = "0.75"
		}
		
		transactions = append(transactions, []string{
			txID,
			amount.String(),
			txType,
			txTime.Format(time.RFC3339),
		})
		
		statements = append(statements, []string{
			stmtID,
			stmtAmount.Round(2).String(),
			txTime.Format("2006-01-02"),
		})
		
		expectedMatches = append(expectedMatches, []string{
			txID,
			stmtID,
			matchType,
			confidence,
		})
	}
	
	sg.writeCSV("accuracy_transactions.csv", transactions)
	sg.writeCSV("accuracy_statements.csv", statements)
	sg.writeCSV("accuracy_expected_matches.csv", expectedMatches)
}

// writeCSV is a helper function to write CSV data
func (sg *ScenarioGenerator) writeCSV(filename string, data [][]string) {
	filepath := fmt.Sprintf("%s/%s", sg.OutputDir, filename)
	
	file, err := os.Create(filepath)
	if err != nil {
		log.Printf("Failed to create %s: %v", filepath, err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for _, record := range data {
		if err := writer.Write(record); err != nil {
			log.Printf("Failed to write record to %s: %v", filepath, err)
			return
		}
	}
	
	fmt.Printf("  Created %s with %d records\n", filename, len(data)-1) // -1 for header
}