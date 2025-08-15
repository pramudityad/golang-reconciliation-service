// Package matcher provides the core transaction matching engine and configuration.
//
// This package implements sophisticated algorithms for matching system transactions
// against bank statements, handling various real-world scenarios including:
//   - Date tolerances for processing delays
//   - Amount tolerances for rounding differences
//   - Fuzzy matching for imperfect data
//   - Timezone handling for global operations
//   - Configurable matching weights and thresholds
//
// The matching engine uses a multi-stage approach:
//  1. Candidate selection using indexed lookups
//  2. Scoring based on amount, date, and type similarity
//  3. Confidence-based filtering and ranking
//  4. Best-match selection with conflict resolution
//
// Example usage:
//
//	config := matcher.DefaultMatchingConfig()
//	config.DateToleranceDays = 2
//	config.AmountTolerancePercent = 0.5
//	
//	engine := matcher.NewMatchingEngine(config)
//	engine.LoadTransactions(transactions)
//	engine.LoadBankStatements(statements)
//	
//	result, err := engine.Reconcile()
package matcher

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// TimezoneMode defines how timezone differences should be handled during matching.
// Different timezone modes accommodate various business requirements and data sources.
type TimezoneMode int

const (
	// TimezoneUTC normalizes all times to UTC before comparison.
	// Use this for globally distributed systems with consistent UTC timestamps.
	TimezoneUTC TimezoneMode = iota
	
	// TimezoneLocal uses the local system timezone for time normalization.
	// Use this when all data sources are in the same local timezone.
	TimezoneLocal
	
	// TimezoneIgnore ignores timezone differences by comparing dates only.
	// This is the most common mode for financial reconciliation where
	// transactions are typically processed on business days regardless of exact time.
	TimezoneIgnore
	
	// TimezoneBusiness uses a specified business timezone for normalization.
	// Use this when your business operates in a specific timezone different
	// from the system timezone.
	TimezoneBusiness
)

// String returns the string representation of TimezoneMode
func (tm TimezoneMode) String() string {
	switch tm {
	case TimezoneUTC:
		return "UTC"
	case TimezoneLocal:
		return "Local"
	case TimezoneIgnore:
		return "Ignore"
	case TimezoneBusiness:
		return "Business"
	default:
		return "Unknown"
	}
}

// MatchType represents the quality and confidence level of a transaction match.
// This classification helps determine how much manual review may be required.
type MatchType int

const (
	// MatchExact represents a perfect match with high confidence.
	// Typically: exact amount, same date, matching type.
	// These matches usually require no manual review.
	MatchExact MatchType = iota
	
	// MatchClose represents a close match within configured tolerances.
	// Typically: amounts within tolerance, dates within 1-2 days.
	// These matches may require minimal review.
	MatchClose
	
	// MatchFuzzy represents a fuzzy match that meets minimum confidence threshold.
	// Typically: larger date differences or amount variations.
	// These matches usually require manual review before acceptance.
	MatchFuzzy
	
	// MatchPossible represents a potential match with low confidence.
	// These matches require careful manual review and may be false positives.
	MatchPossible
	
	// MatchNone indicates no suitable match was found for the transaction.
	// These represent unmatched transactions requiring investigation.
	MatchNone
)

// String returns the string representation of MatchType
func (mt MatchType) String() string {
	switch mt {
	case MatchExact:
		return "Exact"
	case MatchClose:
		return "Close"
	case MatchFuzzy:
		return "Fuzzy"
	case MatchPossible:
		return "Possible"
	case MatchNone:
		return "None"
	default:
		return "Unknown"
	}
}

// MatchingConfig holds configuration parameters for transaction matching.
// This configuration controls all aspects of the matching algorithm including
// tolerances, weights, and behavioral options. Different configurations can be
// used for different scenarios (strict vs relaxed matching).
//
// Key configuration areas:
//   - Date/time handling: tolerances and timezone behavior
//   - Amount handling: precision and tolerance percentages
//   - Match quality: confidence thresholds and fuzzy matching
//   - Performance: candidate limits and indexing behavior
//   - Scoring: relative weights for different match criteria
//
// Use the provided factory functions for common scenarios:
//   - DefaultMatchingConfig(): balanced approach for most use cases
//   - StrictMatchingConfig(): tight tolerances for critical reconciliation
//   - RelaxedMatchingConfig(): loose tolerances for exploratory matching
type MatchingConfig struct {
	// DateToleranceDays defines the number of days tolerance for date matching
	DateToleranceDays int `json:"date_tolerance_days"`
	
	// AmountPrecision defines the number of decimal places for amount comparison
	AmountPrecision int `json:"amount_precision"`
	
	// AmountTolerancePercent defines percentage tolerance for amount matching (0.0 to 100.0)
	AmountTolerancePercent float64 `json:"amount_tolerance_percent"`
	
	// EnableFuzzyMatching enables fuzzy matching for potential matches
	EnableFuzzyMatching bool `json:"enable_fuzzy_matching"`
	
	// TimezoneHandling defines how to handle timezone differences
	TimezoneHandling TimezoneMode `json:"timezone_handling"`
	
	// BusinessTimezone defines the business timezone (used with TimezoneBusiness mode)
	BusinessTimezone string `json:"business_timezone"`
	
	// MaxCandidatesPerTransaction limits the number of candidates to consider per transaction
	MaxCandidatesPerTransaction int `json:"max_candidates_per_transaction"`
	
	// MinConfidenceScore defines the minimum confidence score for a match
	MinConfidenceScore float64 `json:"min_confidence_score"`
	
	// EnableTypeMatching requires transaction type compatibility
	EnableTypeMatching bool `json:"enable_type_matching"`
	
	// EnablePartialMatching allows matching of partial amounts
	EnablePartialMatching bool `json:"enable_partial_matching"`
	
	// MaxPartialMatchRatio defines the maximum ratio for partial matches (0.0 to 1.0)
	MaxPartialMatchRatio float64 `json:"max_partial_match_ratio"`
	
	// IgnoreWeekends excludes weekends from date tolerance calculations
	IgnoreWeekends bool `json:"ignore_weekends"`
	
	// Priority weights for different matching criteria
	Weights MatchingWeights `json:"weights"`
}

// MatchingWeights defines the relative importance of different matching criteria
type MatchingWeights struct {
	AmountWeight float64 `json:"amount_weight"`
	DateWeight   float64 `json:"date_weight"`
	TypeWeight   float64 `json:"type_weight"`
}

// DefaultMatchingConfig returns a configuration with sensible defaults
func DefaultMatchingConfig() *MatchingConfig {
	return &MatchingConfig{
		DateToleranceDays:              1,
		AmountPrecision:                2,
		AmountTolerancePercent:         0.0,
		EnableFuzzyMatching:            true,
		TimezoneHandling:              TimezoneIgnore,
		BusinessTimezone:              "UTC",
		MaxCandidatesPerTransaction:   10,
		MinConfidenceScore:            0.8,
		EnableTypeMatching:            true,
		EnablePartialMatching:         false,
		MaxPartialMatchRatio:          0.1,
		IgnoreWeekends:                false,
		Weights: MatchingWeights{
			AmountWeight: 0.6,
			DateWeight:   0.3,
			TypeWeight:   0.1,
		},
	}
}

// StrictMatchingConfig returns a configuration for strict matching
func StrictMatchingConfig() *MatchingConfig {
	return &MatchingConfig{
		DateToleranceDays:              0,
		AmountPrecision:                2,
		AmountTolerancePercent:         0.0,
		EnableFuzzyMatching:            false,
		TimezoneHandling:              TimezoneUTC,
		BusinessTimezone:              "UTC",
		MaxCandidatesPerTransaction:   5,
		MinConfidenceScore:            0.95,
		EnableTypeMatching:            true,
		EnablePartialMatching:         false,
		MaxPartialMatchRatio:          0.0,
		IgnoreWeekends:                false,
		Weights: MatchingWeights{
			AmountWeight: 0.7,
			DateWeight:   0.2,
			TypeWeight:   0.1,
		},
	}
}

// RelaxedMatchingConfig returns a configuration for relaxed matching
func RelaxedMatchingConfig() *MatchingConfig {
	return &MatchingConfig{
		DateToleranceDays:              3,
		AmountPrecision:                2,
		AmountTolerancePercent:         1.0,
		EnableFuzzyMatching:            true,
		TimezoneHandling:              TimezoneIgnore,
		BusinessTimezone:              "UTC",
		MaxCandidatesPerTransaction:   20,
		MinConfidenceScore:            0.6,
		EnableTypeMatching:            false,
		EnablePartialMatching:         true,
		MaxPartialMatchRatio:          0.2,
		IgnoreWeekends:                true,
		Weights: MatchingWeights{
			AmountWeight: 0.5,
			DateWeight:   0.4,
			TypeWeight:   0.1,
		},
	}
}

// Validate checks if the matching configuration is valid
func (mc *MatchingConfig) Validate() error {
	if mc.DateToleranceDays < 0 {
		return fmt.Errorf("date tolerance days cannot be negative: %d", mc.DateToleranceDays)
	}
	
	if mc.AmountPrecision < 0 || mc.AmountPrecision > 10 {
		return fmt.Errorf("amount precision must be between 0 and 10: %d", mc.AmountPrecision)
	}
	
	if mc.AmountTolerancePercent < 0.0 || mc.AmountTolerancePercent > 100.0 {
		return fmt.Errorf("amount tolerance percent must be between 0.0 and 100.0: %f", mc.AmountTolerancePercent)
	}
	
	if mc.MaxCandidatesPerTransaction <= 0 {
		return fmt.Errorf("max candidates per transaction must be positive: %d", mc.MaxCandidatesPerTransaction)
	}
	
	if mc.MinConfidenceScore < 0.0 || mc.MinConfidenceScore > 1.0 {
		return fmt.Errorf("minimum confidence score must be between 0.0 and 1.0: %f", mc.MinConfidenceScore)
	}
	
	if mc.MaxPartialMatchRatio < 0.0 || mc.MaxPartialMatchRatio > 1.0 {
		return fmt.Errorf("max partial match ratio must be between 0.0 and 1.0: %f", mc.MaxPartialMatchRatio)
	}
	
	// Validate weights
	if err := mc.Weights.Validate(); err != nil {
		return fmt.Errorf("invalid weights: %w", err)
	}
	
	// Validate business timezone if specified
	if mc.TimezoneHandling == TimezoneBusiness {
		if _, err := time.LoadLocation(mc.BusinessTimezone); err != nil {
			return fmt.Errorf("invalid business timezone '%s': %w", mc.BusinessTimezone, err)
		}
	}
	
	return nil
}

// Validate checks if the matching weights are valid
func (mw *MatchingWeights) Validate() error {
	if mw.AmountWeight < 0.0 || mw.AmountWeight > 1.0 {
		return fmt.Errorf("amount weight must be between 0.0 and 1.0: %f", mw.AmountWeight)
	}
	
	if mw.DateWeight < 0.0 || mw.DateWeight > 1.0 {
		return fmt.Errorf("date weight must be between 0.0 and 1.0: %f", mw.DateWeight)
	}
	
	if mw.TypeWeight < 0.0 || mw.TypeWeight > 1.0 {
		return fmt.Errorf("type weight must be between 0.0 and 1.0: %f", mw.TypeWeight)
	}
	
	// Weights should sum to approximately 1.0 (allow some tolerance)
	total := mw.AmountWeight + mw.DateWeight + mw.TypeWeight
	if total < 0.9 || total > 1.1 {
		return fmt.Errorf("weights should sum to approximately 1.0, got %f", total)
	}
	
	return nil
}

// Clone creates a deep copy of the matching configuration
func (mc *MatchingConfig) Clone() *MatchingConfig {
	if mc == nil {
		return nil
	}
	
	return &MatchingConfig{
		DateToleranceDays:              mc.DateToleranceDays,
		AmountPrecision:                mc.AmountPrecision,
		AmountTolerancePercent:         mc.AmountTolerancePercent,
		EnableFuzzyMatching:            mc.EnableFuzzyMatching,
		TimezoneHandling:              mc.TimezoneHandling,
		BusinessTimezone:              mc.BusinessTimezone,
		MaxCandidatesPerTransaction:   mc.MaxCandidatesPerTransaction,
		MinConfidenceScore:            mc.MinConfidenceScore,
		EnableTypeMatching:            mc.EnableTypeMatching,
		EnablePartialMatching:         mc.EnablePartialMatching,
		MaxPartialMatchRatio:          mc.MaxPartialMatchRatio,
		IgnoreWeekends:                mc.IgnoreWeekends,
		Weights: MatchingWeights{
			AmountWeight: mc.Weights.AmountWeight,
			DateWeight:   mc.Weights.DateWeight,
			TypeWeight:   mc.Weights.TypeWeight,
		},
	}
}

// GetAmountTolerance calculates the amount tolerance for a given amount
func (mc *MatchingConfig) GetAmountTolerance(amount decimal.Decimal) decimal.Decimal {
	if mc.AmountTolerancePercent == 0.0 {
		return decimal.Zero
	}
	
	percentage := decimal.NewFromFloat(mc.AmountTolerancePercent / 100.0)
	tolerance := amount.Abs().Mul(percentage)
	
	// Round to the configured precision
	precision := int32(mc.AmountPrecision)
	return tolerance.Round(precision)
}

// IsWithinDateTolerance checks if two dates are within the configured tolerance
func (mc *MatchingConfig) IsWithinDateTolerance(date1, date2 time.Time) bool {
	if mc.DateToleranceDays == 0 {
		return date1.Format("2006-01-02") == date2.Format("2006-01-02")
	}
	
	diff := date1.Sub(date2)
	if diff < 0 {
		diff = -diff
	}
	
	maxDiff := time.Duration(mc.DateToleranceDays) * 24 * time.Hour
	
	if !mc.IgnoreWeekends {
		return diff <= maxDiff
	}
	
	// Calculate business days difference when ignoring weekends
	return mc.isWithinBusinessDayTolerance(date1, date2)
}

// isWithinBusinessDayTolerance checks date tolerance excluding weekends
func (mc *MatchingConfig) isWithinBusinessDayTolerance(date1, date2 time.Time) bool {
	if date1.Equal(date2) {
		return true
	}
	
	// Ensure date1 is earlier than date2
	if date1.After(date2) {
		date1, date2 = date2, date1
	}
	
	businessDays := 0
	current := date1
	
	for businessDays <= mc.DateToleranceDays && current.Before(date2) {
		// Skip weekends
		if current.Weekday() != time.Saturday && current.Weekday() != time.Sunday {
			businessDays++
		}
		current = current.AddDate(0, 0, 1)
		
		if current.Format("2006-01-02") == date2.Format("2006-01-02") {
			return businessDays <= mc.DateToleranceDays
		}
	}
	
	return businessDays <= mc.DateToleranceDays
}

// NormalizeTime normalizes time according to the timezone handling configuration
func (mc *MatchingConfig) NormalizeTime(t time.Time) time.Time {
	switch mc.TimezoneHandling {
	case TimezoneUTC:
		return t.UTC()
	case TimezoneLocal:
		return t.Local()
	case TimezoneIgnore:
		// Return date-only time (midnight UTC)
		year, month, day := t.Date()
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	case TimezoneBusiness:
		if loc, err := time.LoadLocation(mc.BusinessTimezone); err == nil {
			return t.In(loc)
		}
		// Fallback to UTC if business timezone is invalid
		return t.UTC()
	default:
		return t
	}
}

// String returns a human-readable description of the configuration
func (mc *MatchingConfig) String() string {
	return fmt.Sprintf("MatchingConfig{DateTolerance: %d days, AmountPrecision: %d, AmountTolerance: %.2f%%, Timezone: %s, MinConfidence: %.2f}",
		mc.DateToleranceDays, mc.AmountPrecision, mc.AmountTolerancePercent, mc.TimezoneHandling.String(), mc.MinConfidenceScore)
}