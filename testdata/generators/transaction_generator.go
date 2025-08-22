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

// TransactionGenerator generates system transaction CSV files
type TransactionGenerator struct {
	Count     int
	StartDate time.Time
	EndDate   time.Time
	MinAmount decimal.Decimal
	MaxAmount decimal.Decimal
	Seed      int64
}

// TransactionTemplate represents a transaction record
type TransactionTemplate struct {
	TrxID           string
	Amount          decimal.Decimal
	Type            string // CREDIT or DEBIT
	TransactionTime time.Time
}

func main() {
	var (
		output    = flag.String("output", "generated_transactions.csv", "Output CSV file path")
		count     = flag.Int("count", 1000, "Number of transactions to generate")
		startDate = flag.String("start-date", "2024-01-01", "Start date (YYYY-MM-DD)")
		endDate   = flag.String("end-date", "2024-12-31", "End date (YYYY-MM-DD)")
		minAmount = flag.Float64("min-amount", 0.01, "Minimum transaction amount")
		maxAmount = flag.Float64("max-amount", 50000.00, "Maximum transaction amount")
		seed      = flag.Int64("seed", time.Now().UnixNano(), "Random seed for reproducible generation")
		pattern   = flag.String("pattern", "random", "Generation pattern: random, business-hours, end-of-month")
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

	generator := &TransactionGenerator{
		Count:     *count,
		StartDate: start,
		EndDate:   end,
		MinAmount: decimal.NewFromFloat(*minAmount),
		MaxAmount: decimal.NewFromFloat(*maxAmount),
		Seed:      *seed,
	}

	// Generate transactions based on pattern
	var transactions []TransactionTemplate
	switch *pattern {
	case "business-hours":
		transactions = generator.GenerateBusinessHours()
	case "end-of-month":
		transactions = generator.GenerateEndOfMonth()
	case "large-amounts":
		transactions = generator.GenerateLargeAmounts()
	case "micro-transactions":
		transactions = generator.GenerateMicroTransactions()
	default:
		transactions = generator.GenerateRandom()
	}

	// Write to CSV
	if err := generator.WriteToCSV(*output, transactions); err != nil {
		log.Fatalf("Failed to write CSV: %v", err)
	}

	fmt.Printf("Generated %d transactions in %s\n", len(transactions), *output)
	fmt.Printf("Date range: %s to %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))
	fmt.Printf("Amount range: $%.2f to $%.2f\n", *minAmount, *maxAmount)
	fmt.Printf("Seed used: %d\n", *seed)
}

// GenerateRandom creates random transactions distributed evenly across the date range
func (tg *TransactionGenerator) GenerateRandom() []TransactionTemplate {
	rand.Seed(tg.Seed)
	transactions := make([]TransactionTemplate, tg.Count)

	duration := tg.EndDate.Sub(tg.StartDate)
	
	for i := 0; i < tg.Count; i++ {
		// Random time within the date range
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		txTime := tg.StartDate.Add(randomDuration)
		
		// Random amount within range
		amountRange := tg.MaxAmount.Sub(tg.MinAmount)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(tg.MinAmount)
		
		// Random transaction type
		txType := "CREDIT"
		if rand.Float64() < 0.4 { // 40% debit, 60% credit
			txType = "DEBIT"
		}

		transactions[i] = TransactionTemplate{
			TrxID:           fmt.Sprintf("TXG%06d", i+1),
			Amount:          randomAmount.Round(2),
			Type:            txType,
			TransactionTime: txTime,
		}
	}

	return transactions
}

// GenerateBusinessHours creates transactions concentrated during business hours
func (tg *TransactionGenerator) GenerateBusinessHours() []TransactionTemplate {
	rand.Seed(tg.Seed)
	transactions := make([]TransactionTemplate, tg.Count)

	duration := tg.EndDate.Sub(tg.StartDate)
	
	for i := 0; i < tg.Count; i++ {
		// Random day within the date range
		randomDays := rand.Int63n(int64(duration / (24 * time.Hour)))
		baseDate := tg.StartDate.AddDate(0, 0, int(randomDays))
		
		// Business hours: 9 AM to 5 PM on weekdays
		if baseDate.Weekday() == time.Saturday || baseDate.Weekday() == time.Sunday {
			// Weekend - fewer transactions, random times
			randomHour := rand.Intn(24)
			randomMinute := rand.Intn(60)
			baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 
								randomHour, randomMinute, rand.Intn(60), 0, baseDate.Location())
		} else {
			// Weekday - business hours focus
			if rand.Float64() < 0.8 { // 80% during business hours
				randomHour := 9 + rand.Intn(8) // 9 AM to 4:59 PM
				randomMinute := rand.Intn(60)
				baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 
									randomHour, randomMinute, rand.Intn(60), 0, baseDate.Location())
			} else { // 20% outside business hours
				randomHour := rand.Intn(24)
				randomMinute := rand.Intn(60)
				baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), 
									randomHour, randomMinute, rand.Intn(60), 0, baseDate.Location())
			}
		}
		
		// Business hour transactions tend to be larger
		var randomAmount decimal.Decimal
		if baseDate.Hour() >= 9 && baseDate.Hour() <= 17 {
			// Larger amounts during business hours
			minBusiness := tg.MinAmount.Mul(decimal.NewFromInt(10))
			maxBusiness := tg.MaxAmount
			amountRange := maxBusiness.Sub(minBusiness)
			randomAmount = decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(minBusiness)
		} else {
			// Smaller amounts outside business hours
			amountRange := tg.MaxAmount.Sub(tg.MinAmount)
			randomAmount = decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(tg.MinAmount)
		}
		
		// Transaction type
		txType := "CREDIT"
		if rand.Float64() < 0.35 { // 35% debit, 65% credit
			txType = "DEBIT"
		}

		transactions[i] = TransactionTemplate{
			TrxID:           fmt.Sprintf("TXB%06d", i+1),
			Amount:          randomAmount.Round(2),
			Type:            txType,
			TransactionTime: baseDate,
		}
	}

	return transactions
}

// GenerateEndOfMonth creates transactions concentrated at month-end
func (tg *TransactionGenerator) GenerateEndOfMonth() []TransactionTemplate {
	rand.Seed(tg.Seed)
	transactions := make([]TransactionTemplate, tg.Count)

	currentDate := tg.StartDate
	monthlyCount := tg.Count / int(tg.EndDate.Sub(tg.StartDate).Hours()/(24*30)) // Approximate months
	if monthlyCount < 1 {
		monthlyCount = 1
	}

	txIndex := 0
	for currentDate.Before(tg.EndDate) && txIndex < tg.Count {
		// Get last day of current month
		nextMonth := currentDate.AddDate(0, 1, 0)
		lastDayOfMonth := time.Date(nextMonth.Year(), nextMonth.Month(), 1, 0, 0, 0, 0, nextMonth.Location()).AddDate(0, 0, -1)
		
		// Distribute transactions in the last 5 days of the month
		endOfMonthStart := lastDayOfMonth.AddDate(0, 0, -4)
		
		for day := 0; day < 5 && txIndex < tg.Count; day++ {
			transactionsThisDay := rand.Intn(monthlyCount/2) + 1
			
			for i := 0; i < transactionsThisDay && txIndex < tg.Count; i++ {
				txDate := endOfMonthStart.AddDate(0, 0, day)
				randomHour := rand.Intn(24)
				randomMinute := rand.Intn(60)
				txDate = time.Date(txDate.Year(), txDate.Month(), txDate.Day(), 
								  randomHour, randomMinute, rand.Intn(60), 0, txDate.Location())
				
				// End of month transactions tend to be larger (payroll, bills)
				amountMultiplier := decimal.NewFromFloat(1.0 + rand.Float64()*2.0) // 1x to 3x
				amountRange := tg.MaxAmount.Sub(tg.MinAmount)
				randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(tg.MinAmount).Mul(amountMultiplier)
				if randomAmount.GreaterThan(tg.MaxAmount) {
					randomAmount = tg.MaxAmount
				}
				
				txType := "CREDIT"
				if rand.Float64() < 0.3 { // 30% debit, 70% credit (payday effect)
					txType = "DEBIT"
				}

				transactions[txIndex] = TransactionTemplate{
					TrxID:           fmt.Sprintf("TXE%06d", txIndex+1),
					Amount:          randomAmount.Round(2),
					Type:            txType,
					TransactionTime: txDate,
				}
				txIndex++
			}
		}
		
		// Move to next month
		currentDate = currentDate.AddDate(0, 1, 0)
	}

	// Fill remaining slots with random transactions if needed
	for txIndex < tg.Count {
		duration := tg.EndDate.Sub(tg.StartDate)
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		txTime := tg.StartDate.Add(randomDuration)
		
		amountRange := tg.MaxAmount.Sub(tg.MinAmount)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(tg.MinAmount)
		
		txType := "CREDIT"
		if rand.Float64() < 0.4 {
			txType = "DEBIT"
		}

		transactions[txIndex] = TransactionTemplate{
			TrxID:           fmt.Sprintf("TXE%06d", txIndex+1),
			Amount:          randomAmount.Round(2),
			Type:            txType,
			TransactionTime: txTime,
		}
		txIndex++
	}

	return transactions[:txIndex]
}

// GenerateLargeAmounts creates transactions with predominantly large amounts
func (tg *TransactionGenerator) GenerateLargeAmounts() []TransactionTemplate {
	rand.Seed(tg.Seed)
	transactions := make([]TransactionTemplate, tg.Count)

	// Override min amount to be larger
	minLarge := decimal.NewFromFloat(10000.0)
	maxLarge := decimal.NewFromFloat(1000000.0)

	duration := tg.EndDate.Sub(tg.StartDate)
	
	for i := 0; i < tg.Count; i++ {
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		txTime := tg.StartDate.Add(randomDuration)
		
		// Exponential distribution for large amounts
		randomFactor := rand.Float64()
		exponentialFactor := randomFactor * randomFactor // Skew towards smaller end of large range
		amountRange := maxLarge.Sub(minLarge)
		randomAmount := decimal.NewFromFloat(exponentialFactor).Mul(amountRange).Add(minLarge)
		
		txType := "CREDIT"
		if rand.Float64() < 0.25 { // 25% debit, 75% credit (large deposits more common)
			txType = "DEBIT"
		}

		transactions[i] = TransactionTemplate{
			TrxID:           fmt.Sprintf("TXL%06d", i+1),
			Amount:          randomAmount.Round(2),
			Type:            txType,
			TransactionTime: txTime,
		}
	}

	return transactions
}

// GenerateMicroTransactions creates transactions with very small amounts
func (tg *TransactionGenerator) GenerateMicroTransactions() []TransactionTemplate {
	rand.Seed(tg.Seed)
	transactions := make([]TransactionTemplate, tg.Count)

	// Override amounts to be very small
	minMicro := decimal.NewFromFloat(0.01)
	maxMicro := decimal.NewFromFloat(10.0)

	duration := tg.EndDate.Sub(tg.StartDate)
	
	for i := 0; i < tg.Count; i++ {
		randomDuration := time.Duration(rand.Int63n(int64(duration)))
		txTime := tg.StartDate.Add(randomDuration)
		
		amountRange := maxMicro.Sub(minMicro)
		randomAmount := decimal.NewFromFloat(rand.Float64()).Mul(amountRange).Add(minMicro)
		
		txType := "CREDIT"
		if rand.Float64() < 0.5 { // 50/50 split for micro transactions
			txType = "DEBIT"
		}

		transactions[i] = TransactionTemplate{
			TrxID:           fmt.Sprintf("TXM%06d", i+1),
			Amount:          randomAmount.Round(2),
			Type:            txType,
			TransactionTime: txTime,
		}
	}

	return transactions
}

// WriteToCSV writes transactions to a CSV file
func (tg *TransactionGenerator) WriteToCSV(filename string, transactions []TransactionTemplate) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"trxID", "amount", "type", "transactionTime"}); err != nil {
		return err
	}

	// Write transactions
	for _, tx := range transactions {
		record := []string{
			tx.TrxID,
			tx.Amount.String(),
			tx.Type,
			tx.TransactionTime.Format(time.RFC3339),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}