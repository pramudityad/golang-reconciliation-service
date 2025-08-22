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

// StatementGenerator generates bank statement CSV files
type StatementGenerator struct {
	Count         int
	StartDate     time.Time
	EndDate       time.Time
	MinAmount     decimal.Decimal
	MaxAmount     decimal.Decimal
	MatchRatio    float64 // Ratio of statements that should match transactions
	Format        string  // bank1, bank2, or custom format
	Seed          int64
	BaseTransactionFile string // Optional: base transaction file to create matches for
}

// StatementTemplate represents a bank statement record
type StatementTemplate struct {
	UniqueIdentifier string
	Amount           decimal.Decimal
	Date             time.Time
	Description      string // Optional, used in some formats
}

// TransactionData for matching purposes
type TransactionData struct {
	TrxID           string
	Amount          decimal.Decimal
	Type            string
	TransactionTime time.Time
}

func main() {
	var (
		output     = flag.String("output", "generated_statements.csv", "Output CSV file path")
		count      = flag.Int("count", 1000, "Number of statements to generate")
		startDate  = flag.String("start-date", "2024-01-01", "Start date (YYYY-MM-DD)")
		endDate    = flag.String("end-date", "2024-12-31", "End date (YYYY-MM-DD)")
		minAmount  = flag.Float64("min-amount", 0.01, "Minimum statement amount")
		maxAmount  = flag.Float64("max-amount", 50000.00, "Maximum statement amount")
		matchRatio = flag.Float64("match-ratio", 0.8, "Ratio of statements that should match transactions (0.0-1.0)")
		format     = flag.String("format", "bank1", "Output format: bank1, bank2, or custom")
		seed       = flag.Int64("seed", time.Now().UnixNano(), "Random seed for reproducible generation")
		baseFile   = flag.String("base-transactions", "", "Base transaction CSV file to create matches for")
		pattern    = flag.String("pattern", "random", "Generation pattern: random, matching, mismatched")
	)
	flag.Parse()

	// Parse dates
	start, err := time.Parse("2006-01-02", *startDate)
	if err != nil {
		log.Fatalf("Invalid start date: %v", err)
	}

	end, err := time.Parse("2006-01-02", *endDate)
	if err != nil {
		log.Fatalf("Invalid end date: %v", err)
	}

	generator := &StatementGenerator{
		Count:               *count,
		StartDate:           start,
		EndDate:             end,
		MinAmount:           decimal.NewFromFloat(*minAmount),
		MaxAmount:           decimal.NewFromFloat(*maxAmount),
		MatchRatio:          *matchRatio,
		Format:              *format,
		Seed:                *seed,
		BaseTransactionFile: *baseFile,
	}

	// Load base transactions if provided
	var baseTransactions []TransactionData
	if *baseFile != "" {
		baseTransactions, err = generator.LoadBaseTransactions(*baseFile)
		if err != nil {
			log.Fatalf("Failed to load base transactions: %v", err)
		}
		fmt.Printf("Loaded %d base transactions from %s\n", len(baseTransactions), *baseFile)
	}

	// Generate statements based on pattern
	var statements []StatementTemplate
	switch *pattern {
	case "matching":
		if len(baseTransactions) == 0 {
			log.Fatal("matching pattern requires base-transactions file")
		}
		statements = generator.GenerateMatching(baseTransactions)
	case "mismatched":
		statements = generator.GenerateMismatched()
	default:
		if len(baseTransactions) > 0 {
			statements = generator.GenerateWithMatches(baseTransactions)
		} else {
			statements = generator.GenerateRandom()
		}
	}

	// Write to CSV
	if err := generator.WriteToCSV(*output, statements); err != nil {
		log.Fatalf("Failed to write CSV: %v", err)
	}

	fmt.Printf("Generated %d statements in %s\n", len(statements), *output)
	fmt.Printf("Format: %s\n", *format)
	fmt.Printf("Date range: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Amount range: $%.2f to $%.2f\n", *minAmount, *maxAmount)
	fmt.Printf("Match ratio: %.1f%%\n", *matchRatio*100)
	fmt.Printf("Seed used: %d\n", *seed)
}

// LoadBaseTransactions loads transactions from CSV file
func (sg *StatementGenerator) LoadBaseTransactions(filename string) ([]TransactionData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("file must have header and at least one data row")
	}

	// Skip header
	var transactions []TransactionData
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue // Skip malformed records
		}

		amount, err := decimal.NewFromString(record[1])
		if err != nil {
			continue // Skip invalid amounts
		}

		txTime, err := time.Parse(time.RFC3339, record[3])
		if err != nil {
			continue // Skip invalid times
		}

		transactions = append(transactions, TransactionData{
			TrxID:           record[0],
			Amount:          amount,
			Type:            record[2],
			TransactionTime: txTime,
		})
	}

	return transactions, nil
}

// GenerateRandom creates random bank statements
func (sg *StatementGenerator) GenerateRandom() []StatementTemplate {
	rand.Seed(sg.Seed)
	statements := make([]StatementTemplate, sg.Count)

	duration := sg.EndDate.Sub(sg.StartDate)
	
	for i := 0; i < sg.Count; i++ {
		// Random date within range
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		stmtDate := sg.StartDate.Add(randomDuration)
		
		// Random amount within range
		amountRange := sg.MaxAmount.Sub(sg.MinAmount)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(sg.MinAmount)
		
		// Apply debit convention (negative for debits)
		if rand.Float64() < 0.4 { // 40% debits
			randomAmount = randomAmount.Neg()
		}

		statements[i] = StatementTemplate{
			UniqueIdentifier: fmt.Sprintf("BS%06d", i+1),
			Amount:          randomAmount.Round(2),
			Date:            stmtDate,
			Description:     sg.generateDescription(randomAmount),
		}
	}

	return statements
}

// GenerateWithMatches creates statements with specified match ratio to base transactions
func (sg *StatementGenerator) GenerateWithMatches(baseTransactions []TransactionData) []StatementTemplate {
	rand.Seed(sg.Seed)
	
	matchCount := int(float64(sg.Count) * sg.MatchRatio)
	unmatchedCount := sg.Count - matchCount
	
	var statements []StatementTemplate
	
	// Generate matching statements
	for i := 0; i < matchCount && i < len(baseTransactions); i++ {
		tx := baseTransactions[i%len(baseTransactions)]
		
		// Convert to bank statement format
		amount := tx.Amount
		if tx.Type == "DEBIT" {
			amount = amount.Neg() // Bank convention: negative for debits
		}
		
		// Add small variations for realistic matching
		if rand.Float64() < 0.1 { // 10% have small amount differences
			variation := decimal.NewFromFloat((rand.Float64() - 0.5) * 0.02) // ±1%
			amount = amount.Add(amount.Mul(variation))
		}
		
		// Date might be slightly different
		stmtDate := tx.TransactionTime
		if rand.Float64() < 0.2 { // 20% have date differences
			dayOffset := rand.Intn(3) - 1 // -1, 0, or +1 day
			stmtDate = stmtDate.AddDate(0, 0, dayOffset)
		}
		
		statements = append(statements, StatementTemplate{
			UniqueIdentifier: fmt.Sprintf("BS%06d", i+1),
			Amount:          amount.Round(2),
			Date:            stmtDate,
			Description:     sg.generateDescription(amount),
		})
	}
	
	// Generate unmatched statements
	duration := sg.EndDate.Sub(sg.StartDate)
	for i := 0; i < unmatchedCount; i++ {
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		stmtDate := sg.StartDate.Add(randomDuration)
		
		amountRange := sg.MaxAmount.Sub(sg.MinAmount)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(sg.MinAmount)
		
		if rand.Float64() < 0.4 {
			randomAmount = randomAmount.Neg()
		}

		statements = append(statements, StatementTemplate{
			UniqueIdentifier: fmt.Sprintf("BS%06d", matchCount+i+1),
			Amount:          randomAmount.Round(2),
			Date:            stmtDate,
			Description:     sg.generateDescription(randomAmount),
		})
	}

	return statements
}

// GenerateMatching creates statements that closely match base transactions
func (sg *StatementGenerator) GenerateMatching(baseTransactions []TransactionData) []StatementTemplate {
	rand.Seed(sg.Seed)
	statements := make([]StatementTemplate, 0, len(baseTransactions))
	
	for i, tx := range baseTransactions {
		amount := tx.Amount
		if tx.Type == "DEBIT" {
			amount = amount.Neg()
		}
		
		// Minimal variations for high match confidence
		if rand.Float64() < 0.05 { // Only 5% have any variation
			variation := decimal.NewFromFloat((rand.Float64() - 0.5) * 0.001) // ±0.05%
			amount = amount.Add(amount.Mul(variation))
		}
		
		statements = append(statements, StatementTemplate{
			UniqueIdentifier: fmt.Sprintf("MATCH%06d", i+1),
			Amount:          amount.Round(2),
			Date:            tx.TransactionTime,
			Description:     sg.generateDescription(amount),
		})
	}

	return statements
}

// GenerateMismatched creates statements that intentionally don't match well
func (sg *StatementGenerator) GenerateMismatched() []StatementTemplate {
	rand.Seed(sg.Seed)
	statements := make([]StatementTemplate, sg.Count)

	duration := sg.EndDate.Sub(sg.StartDate)
	
	for i := 0; i < sg.Count; i++ {
		// Random date with potential large offsets
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		stmtDate := sg.StartDate.Add(randomDuration)
		
		// Add additional date offset to reduce matches
		dayOffset := rand.Intn(30) - 15 // ±15 days
		stmtDate = stmtDate.AddDate(0, 0, dayOffset)
		
		// Amounts that are less likely to match
		amountRange := sg.MaxAmount.Sub(sg.MinAmount)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(sg.MinAmount)
		
		// Add significant amount variation
		variation := decimal.NewFromFloat((rand.Float64() - 0.5) * 0.2) // ±10%
		randomAmount = randomAmount.Add(randomAmount.Mul(variation))
		
		if rand.Float64() < 0.6 { // 60% debits (higher than normal)
			randomAmount = randomAmount.Neg()
		}

		statements[i] = StatementTemplate{
			UniqueIdentifier: fmt.Sprintf("MISMATCH%06d", i+1),
			Amount:          randomAmount.Round(2),
			Date:            stmtDate,
			Description:     sg.generateDescription(randomAmount),
		}
	}

	return statements
}

// generateDescription creates realistic transaction descriptions
func (sg *StatementGenerator) generateDescription(amount decimal.Decimal) string {
	if amount.IsPositive() {
		creditDescriptions := []string{
			"Deposit", "Transfer In", "Direct Deposit", "Check Deposit",
			"Payroll", "Interest", "Refund", "Cashback", "Dividend",
		}
		return creditDescriptions[rand.Intn(len(creditDescriptions))]
	} else {
		debitDescriptions := []string{
			"Withdrawal", "Transfer Out", "Payment", "ATM Withdrawal",
			"Online Payment", "Bill Payment", "Purchase", "Service Charge",
			"Maintenance Fee", "Auto Payment",
		}
		return debitDescriptions[rand.Intn(len(debitDescriptions))]
	}
}

// WriteToCSV writes statements to a CSV file in the specified format
func (sg *StatementGenerator) WriteToCSV(filename string, statements []StatementTemplate) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header based on format
	switch sg.Format {
	case "bank1":
		if err := writer.Write([]string{"unique_identifier", "amount", "date"}); err != nil {
			return err
		}
		
		for _, stmt := range statements {
			record := []string{
				stmt.UniqueIdentifier,
				stmt.Amount.String(),
				stmt.Date.Format("2006-01-02"),
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		
	case "bank2":
		if err := writer.Write([]string{"transaction_id", "transaction_amount", "posting_date", "transaction_description"}); err != nil {
			return err
		}
		
		for _, stmt := range statements {
			record := []string{
				"TXN_" + stmt.UniqueIdentifier[2:], // Convert BS123456 to TXN_123456
				stmt.Amount.String(),
				stmt.Date.Format("01/02/2006"),
				stmt.Description,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		
	case "custom":
		if err := writer.Write([]string{"id", "value", "transaction_date", "type", "notes"}); err != nil {
			return err
		}
		
		for _, stmt := range statements {
			txType := "CREDIT"
			if stmt.Amount.IsNegative() {
				txType = "DEBIT"
			}
			
			record := []string{
				stmt.UniqueIdentifier,
				stmt.Amount.Abs().String(),
				stmt.Date.Format("2006/01/02"),
				txType,
				stmt.Description,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		
	default:
		return fmt.Errorf("unsupported format: %s", sg.Format)
	}

	return nil
}