package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestTransactionType_String(t *testing.T) {
	tests := []struct {
		txType   TransactionType
		expected string
	}{
		{TransactionTypeDebit, "DEBIT"},
		{TransactionTypeCredit, "CREDIT"},
	}

	for _, tt := range tests {
		t.Run(string(tt.txType), func(t *testing.T) {
			if got := tt.txType.String(); got != tt.expected {
				t.Errorf("TransactionType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		txType TransactionType
		valid  bool
	}{
		{TransactionTypeDebit, true},
		{TransactionTypeCredit, true},
		{"INVALID", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.txType), func(t *testing.T) {
			if got := tt.txType.IsValid(); got != tt.valid {
				t.Errorf("TransactionType.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestNewTransaction(t *testing.T) {
	amount := decimal.NewFromFloat(100.50)
	txTime := time.Now()
	
	tx := NewTransaction("TX001", amount, TransactionTypeCredit, txTime)
	
	if tx.TrxID != "TX001" {
		t.Errorf("Expected TrxID 'TX001', got %s", tx.TrxID)
	}
	if !tx.Amount.Equal(amount) {
		t.Errorf("Expected amount %s, got %s", amount.String(), tx.Amount.String())
	}
	if tx.Type != TransactionTypeCredit {
		t.Errorf("Expected type %s, got %s", TransactionTypeCredit, tx.Type)
	}
	if !tx.TransactionTime.Equal(txTime) {
		t.Errorf("Expected time %s, got %s", txTime, tx.TransactionTime)
	}
}

func TestTransaction_Validate(t *testing.T) {
	validAmount := decimal.NewFromFloat(100.50)
	validTime := time.Now()

	tests := []struct {
		name        string
		transaction Transaction
		wantError   bool
	}{
		{
			name: "Valid transaction",
			transaction: Transaction{
				TrxID:           "TX001",
				Amount:          validAmount,
				Type:            TransactionTypeCredit,
				TransactionTime: validTime,
			},
			wantError: false,
		},
		{
			name: "Empty transaction ID",
			transaction: Transaction{
				TrxID:           "",
				Amount:          validAmount,
				Type:            TransactionTypeCredit,
				TransactionTime: validTime,
			},
			wantError: true,
		},
		{
			name: "Zero amount",
			transaction: Transaction{
				TrxID:           "TX001",
				Amount:          decimal.Zero,
				Type:            TransactionTypeCredit,
				TransactionTime: validTime,
			},
			wantError: true,
		},
		{
			name: "Invalid transaction type",
			transaction: Transaction{
				TrxID:           "TX001",
				Amount:          validAmount,
				Type:            "INVALID",
				TransactionTime: validTime,
			},
			wantError: true,
		},
		{
			name: "Zero transaction time",
			transaction: Transaction{
				TrxID:           "TX001",
				Amount:          validAmount,
				Type:            TransactionTypeCredit,
				TransactionTime: time.Time{},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.transaction.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Transaction.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestTransaction_JSONMarshaling(t *testing.T) {
	amount := decimal.NewFromFloat(100.50)
	txTime, _ := time.Parse(time.RFC3339, "2024-01-15T10:30:00Z")
	
	tx := NewTransaction("TX001", amount, TransactionTypeCredit, txTime)
	
	// Test marshaling
	jsonData, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("Failed to marshal transaction: %v", err)
	}
	
	// Test unmarshaling
	var unmarshaled Transaction
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal transaction: %v", err)
	}
	
	// Verify fields
	if !tx.Equals(&unmarshaled) {
		t.Errorf("Original and unmarshaled transactions are not equal")
	}
}

func TestTransaction_Equals(t *testing.T) {
	amount := decimal.NewFromFloat(100.50)
	txTime := time.Now()
	
	tx1 := NewTransaction("TX001", amount, TransactionTypeCredit, txTime)
	tx2 := NewTransaction("TX001", amount, TransactionTypeCredit, txTime)
	tx3 := NewTransaction("TX002", amount, TransactionTypeCredit, txTime)
	
	if !tx1.Equals(tx2) {
		t.Error("Expected equal transactions to be equal")
	}
	
	if tx1.Equals(tx3) {
		t.Error("Expected different transactions to be not equal")
	}
	
	if tx1.Equals(nil) {
		t.Error("Expected transaction to not equal nil")
	}
}

func TestTransaction_HelperMethods(t *testing.T) {
	amount := decimal.NewFromFloat(-100.50)
	tx := NewTransaction("TX001", amount, TransactionTypeDebit, time.Now())
	
	// Test GetAbsoluteAmount
	absAmount := tx.GetAbsoluteAmount()
	expected := decimal.NewFromFloat(100.50)
	if !absAmount.Equal(expected) {
		t.Errorf("Expected absolute amount %s, got %s", expected.String(), absAmount.String())
	}
	
	// Test IsDebit
	if !tx.IsDebit() {
		t.Error("Expected transaction to be debit")
	}
	
	// Test IsCredit
	if tx.IsCredit() {
		t.Error("Expected transaction to not be credit")
	}
}

func TestNewBankStatement(t *testing.T) {
	amount := decimal.NewFromFloat(-250.75)
	date := time.Now()
	
	bs := NewBankStatement("BS001", amount, date)
	
	if bs.UniqueIdentifier != "BS001" {
		t.Errorf("Expected UniqueIdentifier 'BS001', got %s", bs.UniqueIdentifier)
	}
	if !bs.Amount.Equal(amount) {
		t.Errorf("Expected amount %s, got %s", amount.String(), bs.Amount.String())
	}
	if !bs.Date.Equal(date) {
		t.Errorf("Expected date %s, got %s", date, bs.Date)
	}
}

func TestBankStatement_Validate(t *testing.T) {
	validAmount := decimal.NewFromFloat(-100.50)
	validDate := time.Now()

	tests := []struct {
		name          string
		bankStatement BankStatement
		wantError     bool
	}{
		{
			name: "Valid bank statement",
			bankStatement: BankStatement{
				UniqueIdentifier: "BS001",
				Amount:           validAmount,
				Date:             validDate,
			},
			wantError: false,
		},
		{
			name: "Empty identifier",
			bankStatement: BankStatement{
				UniqueIdentifier: "",
				Amount:           validAmount,
				Date:             validDate,
			},
			wantError: true,
		},
		{
			name: "Zero amount",
			bankStatement: BankStatement{
				UniqueIdentifier: "BS001",
				Amount:           decimal.Zero,
				Date:             validDate,
			},
			wantError: true,
		},
		{
			name: "Zero date",
			bankStatement: BankStatement{
				UniqueIdentifier: "BS001",
				Amount:           validAmount,
				Date:             time.Time{},
			},
			wantError: true,
		},
		{
			name: "Future date",
			bankStatement: BankStatement{
				UniqueIdentifier: "BS001",
				Amount:           validAmount,
				Date:             time.Now().Add(48 * time.Hour),
			},
			wantError: true,
		},
		{
			name: "Date too far in past",
			bankStatement: BankStatement{
				UniqueIdentifier: "BS001",
				Amount:           validAmount,
				Date:             time.Now().AddDate(-15, 0, 0),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bankStatement.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("BankStatement.Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestBankStatement_JSONMarshaling(t *testing.T) {
	amount := decimal.NewFromFloat(-250.75)
	date, _ := time.Parse("2006-01-02", "2024-01-15")
	
	bs := NewBankStatement("BS001", amount, date)
	
	// Test marshaling
	jsonData, err := json.Marshal(bs)
	if err != nil {
		t.Fatalf("Failed to marshal bank statement: %v", err)
	}
	
	// Test unmarshaling
	var unmarshaled BankStatement
	if err := json.Unmarshal(jsonData, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal bank statement: %v", err)
	}
	
	// Verify fields
	if !bs.Equals(&unmarshaled) {
		t.Errorf("Original and unmarshaled bank statements are not equal")
	}
}

func TestBankStatement_HelperMethods(t *testing.T) {
	negativeAmount := decimal.NewFromFloat(-250.75)
	positiveAmount := decimal.NewFromFloat(250.75)
	
	// Test with negative amount (debit)
	bs1 := NewBankStatement("BS001", negativeAmount, time.Now())
	
	if !bs1.IsDebit() {
		t.Error("Expected bank statement with negative amount to be debit")
	}
	
	if bs1.IsCredit() {
		t.Error("Expected bank statement with negative amount to not be credit")
	}
	
	if bs1.GetTransactionType() != TransactionTypeDebit {
		t.Errorf("Expected transaction type %s, got %s", TransactionTypeDebit, bs1.GetTransactionType())
	}
	
	// Test with positive amount (credit)
	bs2 := NewBankStatement("BS002", positiveAmount, time.Now())
	
	if bs2.IsDebit() {
		t.Error("Expected bank statement with positive amount to not be debit")
	}
	
	if !bs2.IsCredit() {
		t.Error("Expected bank statement with positive amount to be credit")
	}
	
	if bs2.GetTransactionType() != TransactionTypeCredit {
		t.Errorf("Expected transaction type %s, got %s", TransactionTypeCredit, bs2.GetTransactionType())
	}
	
	// Test NormalizeAmount
	normalizedAmount := bs1.NormalizeAmount()
	expectedAmount := decimal.NewFromFloat(250.75)
	if !normalizedAmount.Equal(expectedAmount) {
		t.Errorf("Expected normalized amount %s, got %s", expectedAmount.String(), normalizedAmount.String())
	}
}

func TestParseDecimalFromString(t *testing.T) {
	tests := []struct {
		input     string
		expected  decimal.Decimal
		wantError bool
	}{
		{"100.50", decimal.NewFromFloat(100.50), false},
		{"$1,250.75", decimal.NewFromFloat(1250.75), false},
		{"-500.25", decimal.NewFromFloat(-500.25), false},
		{"", decimal.Zero, true},
		{"   ", decimal.Zero, true},
		{"invalid", decimal.Zero, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseDecimalFromString(tt.input)
			
			if (err != nil) != tt.wantError {
				t.Errorf("ParseDecimalFromString() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if !tt.wantError && !result.Equal(tt.expected) {
				t.Errorf("ParseDecimalFromString() = %s, want %s", result.String(), tt.expected.String())
			}
		})
	}
}

func TestParseTransactionType(t *testing.T) {
	tests := []struct {
		input     string
		expected  TransactionType
		wantError bool
	}{
		{"DEBIT", TransactionTypeDebit, false},
		{"debit", TransactionTypeDebit, false},
		{"D", TransactionTypeDebit, false},
		{"DR", TransactionTypeDebit, false},
		{"CREDIT", TransactionTypeCredit, false},
		{"credit", TransactionTypeCredit, false},
		{"C", TransactionTypeCredit, false},
		{"CR", TransactionTypeCredit, false},
		{"INVALID", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseTransactionType(tt.input)
			
			if (err != nil) != tt.wantError {
				t.Errorf("ParseTransactionType() error = %v, wantError %v", err, tt.wantError)
				return
			}
			
			if !tt.wantError && result != tt.expected {
				t.Errorf("ParseTransactionType() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestParseTimeWithFormats(t *testing.T) {
	tests := []struct {
		input     string
		wantError bool
	}{
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15 10:30:00", false},
		{"2024-01-15", false},
		{"01/15/2024", false},
		{"Jan 15, 2024", false},
		{"", true},
		{"invalid-date", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseTimeWithFormats(tt.input)
			
			if (err != nil) != tt.wantError {
				t.Errorf("ParseTimeWithFormats() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCompareAmountsWithTolerance(t *testing.T) {
	amount1 := decimal.NewFromFloat(100.50)
	amount2 := decimal.NewFromFloat(100.52)
	tolerance := decimal.NewFromFloat(0.05)
	
	if !CompareAmountsWithTolerance(amount1, amount2, tolerance) {
		t.Error("Expected amounts to be within tolerance")
	}
	
	amount3 := decimal.NewFromFloat(101.00)
	if CompareAmountsWithTolerance(amount1, amount3, tolerance) {
		t.Error("Expected amounts to be outside tolerance")
	}
}

func TestCompareDatesWithTolerance(t *testing.T) {
	date1 := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	date2 := date1.Add(12 * time.Hour)
	
	if !CompareDatesWithTolerance(date1, date2, 1) {
		t.Error("Expected dates to be within 1 day tolerance")
	}
	
	date3 := date1.Add(48 * time.Hour)
	if CompareDatesWithTolerance(date1, date3, 1) {
		t.Error("Expected dates to be outside 1 day tolerance")
	}
}

func TestCreateTransactionFromCSV(t *testing.T) {
	tests := []struct {
		name      string
		trxID     string
		amount    string
		txType    string
		time      string
		wantError bool
	}{
		{
			name:      "Valid CSV data",
			trxID:     "TX001",
			amount:    "100.50",
			txType:    "CREDIT",
			time:      "2024-01-15T10:30:00Z",
			wantError: false,
		},
		{
			name:      "Invalid amount",
			trxID:     "TX001",
			amount:    "invalid",
			txType:    "CREDIT",
			time:      "2024-01-15T10:30:00Z",
			wantError: true,
		},
		{
			name:      "Invalid transaction type",
			trxID:     "TX001",
			amount:    "100.50",
			txType:    "INVALID",
			time:      "2024-01-15T10:30:00Z",
			wantError: true,
		},
		{
			name:      "Invalid time",
			trxID:     "TX001",
			amount:    "100.50",
			txType:    "CREDIT",
			time:      "invalid-time",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateTransactionFromCSV(tt.trxID, tt.amount, tt.txType, tt.time)
			
			if (err != nil) != tt.wantError {
				t.Errorf("CreateTransactionFromCSV() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCreateBankStatementFromCSV(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		amount     string
		date       string
		wantError  bool
	}{
		{
			name:       "Valid CSV data",
			identifier: "BS001",
			amount:     "-250.75",
			date:       "2024-01-15",
			wantError:  false,
		},
		{
			name:       "Invalid amount",
			identifier: "BS001",
			amount:     "invalid",
			date:       "2024-01-15",
			wantError:  true,
		},
		{
			name:       "Invalid date",
			identifier: "BS001",
			amount:     "-250.75",
			date:       "invalid-date",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CreateBankStatementFromCSV(tt.identifier, tt.amount, tt.date)
			
			if (err != nil) != tt.wantError {
				t.Errorf("CreateBankStatementFromCSV() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkTransaction_Validate(b *testing.B) {
	tx := NewTransaction("TX001", decimal.NewFromFloat(100.50), TransactionTypeCredit, time.Now())
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tx.Validate()
	}
}

func BenchmarkParseDecimalFromString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ParseDecimalFromString("1234.56")
	}
}

func BenchmarkCompareAmountsWithTolerance(b *testing.B) {
	amount1 := decimal.NewFromFloat(100.50)
	amount2 := decimal.NewFromFloat(100.52)
	tolerance := decimal.NewFromFloat(0.05)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CompareAmountsWithTolerance(amount1, amount2, tolerance)
	}
}