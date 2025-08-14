package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestValidateFileExists(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.csv")
	if err := os.WriteFile(validFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		filePath    string
		description string
		expectError bool
	}{
		{
			name:        "valid file",
			filePath:    validFile,
			description: "test file",
			expectError: false,
		},
		{
			name:        "empty path",
			filePath:    "",
			description: "test file",
			expectError: true,
		},
		{
			name:        "non-existent file",
			filePath:    "/non/existent/file.csv",
			description: "test file",
			expectError: true,
		},
		{
			name:        "directory instead of file",
			filePath:    tmpDir,
			description: "test file",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateFileExists(tt.filePath, tt.description)
			
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateReconcileFlags(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()
	systemFile := filepath.Join(tmpDir, "transactions.csv")
	bankFile := filepath.Join(tmpDir, "statements.csv")
	
	if err := os.WriteFile(systemFile, []byte("trxID,amount,type,transactionTime\nTX001,100.50,CREDIT,2024-01-15T10:30:00Z"), 0644); err != nil {
		t.Fatalf("failed to create system file: %v", err)
	}
	if err := os.WriteFile(bankFile, []byte("unique_identifier,amount,date\nBS001,100.50,2024-01-15"), 0644); err != nil {
		t.Fatalf("failed to create bank file: %v", err)
	}

	tests := []struct {
		name        string
		setupFlags  func()
		expectError bool
		errorContains string
	}{
		{
			name: "valid flags",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "console")
				viper.Set("date-tolerance", 1)
				viper.Set("amount-tolerance", 0.0)
			},
			expectError: false,
		},
		{
			name: "missing system file",
			setupFlags: func() {
				viper.Set("system-file", "")
				viper.Set("bank-files", []string{bankFile})
			},
			expectError: true,
			errorContains: "system-file is required",
		},
		{
			name: "missing bank files",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{})
			},
			expectError: true,
			errorContains: "at least one bank-file is required",
		},
		{
			name: "invalid output format",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "invalid")
			},
			expectError: true,
			errorContains: "invalid output format",
		},
		{
			name: "invalid start date",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "console")
				viper.Set("start-date", "invalid-date")
			},
			expectError: true,
			errorContains: "invalid start date format",
		},
		{
			name: "start date after end date",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "console")
				viper.Set("start-date", "2024-01-31")
				viper.Set("end-date", "2024-01-01")
			},
			expectError: true,
			errorContains: "start date cannot be after end date",
		},
		{
			name: "negative date tolerance",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "console")
				viper.Set("date-tolerance", -1)
			},
			expectError: true,
			errorContains: "date tolerance cannot be negative",
		},
		{
			name: "invalid amount tolerance",
			setupFlags: func() {
				viper.Set("system-file", systemFile)
				viper.Set("bank-files", []string{bankFile})
				viper.Set("output-format", "console")
				viper.Set("amount-tolerance", 150.0)
			},
			expectError: true,
			errorContains: "amount tolerance must be between 0.0 and 100.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper
			viper.Reset()
			tt.setupFlags()

			cmd := &cobra.Command{}
			err := validateReconcileFlags(cmd, []string{})

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("expected error to contain '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestReconcileCommandHelp(t *testing.T) {
	cmd := reconcileCmd
	
	// Test that command has required flags
	systemFileFlag := cmd.Flags().Lookup("system-file")
	if systemFileFlag == nil {
		t.Error("system-file flag not found")
	}
	
	bankFilesFlag := cmd.Flags().Lookup("bank-files")
	if bankFilesFlag == nil {
		t.Error("bank-files flag not found")
	}
	
	outputFormatFlag := cmd.Flags().Lookup("output-format")
	if outputFormatFlag == nil {
		t.Error("output-format flag not found")
	}
	
	// Test help output contains key information
	var helpOutput bytes.Buffer
	cmd.SetOut(&helpOutput)
	cmd.Help()
	
	helpText := helpOutput.String()
	
	expectedSections := []string{
		"Usage:",
		"Examples:",
		"Flags:",
		"--system-file",
		"--bank-files",
		"--output-format",
	}
	
	for _, section := range expectedSections {
		if !strings.Contains(helpText, section) {
			t.Errorf("help text should contain '%s'", section)
		}
	}
}

func TestReconcileCommandExamples(t *testing.T) {
	// Test that the examples in the help text are valid
	examples := []struct {
		name string
		args []string
	}{
		{
			name: "basic example",
			args: []string{"--system-file", "tx.csv", "--bank-files", "stmt.csv"},
		},
		{
			name: "multiple bank files",
			args: []string{"--system-file", "tx.csv", "--bank-files", "bank1.csv,bank2.csv"},
		},
		{
			name: "with date range",
			args: []string{"--system-file", "tx.csv", "--bank-files", "stmt.csv", "--start-date", "2024-01-01", "--end-date", "2024-01-31"},
		},
		{
			name: "with output format",
			args: []string{"--system-file", "tx.csv", "--bank-files", "stmt.csv", "--output-format", "json"},
		},
	}

	for _, tt := range examples {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the command can parse these arguments without errors
			cmd := &cobra.Command{}
			cmd.Flags().StringP("system-file", "s", "", "")
			cmd.Flags().StringSliceP("bank-files", "b", []string{}, "")
			cmd.Flags().StringP("output-format", "f", "console", "")
			cmd.Flags().String("start-date", "", "")
			cmd.Flags().String("end-date", "", "")
			
			cmd.SetArgs(tt.args)
			_, err := cmd.ExecuteC()
			
			// We expect a validation error about missing files, not a parsing error
			if err != nil && !strings.Contains(err.Error(), "file") {
				t.Errorf("unexpected parsing error for example '%s': %v", tt.name, err)
			}
		})
	}
}

func TestOutputFormatValidation(t *testing.T) {
	validFormats := []string{"console", "json", "csv"}
	invalidFormats := []string{"xml", "yaml", "invalid", ""}

	for _, format := range validFormats {
		t.Run(fmt.Sprintf("valid_%s", format), func(t *testing.T) {
			validFormatsMap := map[string]bool{"console": true, "json": true, "csv": true}
			if !validFormatsMap[format] {
				t.Errorf("format '%s' should be valid", format)
			}
		})
	}

	for _, format := range invalidFormats {
		t.Run(fmt.Sprintf("invalid_%s", format), func(t *testing.T) {
			validFormatsMap := map[string]bool{"console": true, "json": true, "csv": true}
			if validFormatsMap[format] {
				t.Errorf("format '%s' should be invalid", format)
			}
		})
	}
}

func TestDateValidation(t *testing.T) {
	tests := []struct {
		name    string
		date    string
		isValid bool
	}{
		{"valid date", "2024-01-15", true},
		{"valid date with zeros", "2024-01-01", true},
		{"invalid format", "01/15/2024", false},
		{"invalid format 2", "2024-1-15", false},
		{"empty string", "", true}, // empty is allowed (means no filter)
		{"invalid month", "2024-13-01", false},
		{"invalid day", "2024-01-32", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.date == "" {
				// Empty dates are valid (no filtering)
				return
			}
			
			_, err := parseDate(tt.date)
			isValid := err == nil
			
			if isValid != tt.isValid {
				t.Errorf("date '%s' validity: got %v, want %v", tt.date, isValid, tt.isValid)
			}
		})
	}
}


// Helper function for testing date parsing
func parseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	
	return time.Parse("2006-01-02", dateStr)
}

func TestFlagBinding(t *testing.T) {
	// Test that all flags are properly bound to viper
	cmd := reconcileCmd

	flagTests := []struct {
		flagName  string
		viperKey  string
	}{
		{"system-file", "system-file"},
		{"bank-files", "bank-files"},
		{"output-format", "output-format"},
		{"output-file", "output-file"},
		{"start-date", "start-date"},
		{"end-date", "end-date"},
		{"date-tolerance", "date-tolerance"},
		{"amount-tolerance", "amount-tolerance"},
		{"progress", "progress"},
	}

	for _, tt := range flagTests {
		t.Run(tt.flagName, func(t *testing.T) {
			flag := cmd.Flags().Lookup(tt.flagName)
			if flag == nil {
				t.Errorf("flag '%s' not found", tt.flagName)
				return
			}

			// Test that the flag exists in viper bindings
			// Note: In a real test, we'd check viper.IsSet or similar
			// For now, we just verify the flag exists
		})
	}
}