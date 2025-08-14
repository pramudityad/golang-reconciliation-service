package logger

import (
	"fmt"
	"sync"
	"time"
)

// ProgressTracker tracks progress of long-running operations
type ProgressTracker struct {
	logger      Logger
	operation   string
	total       int64
	current     int64
	startTime   time.Time
	lastLogTime time.Time
	logInterval time.Duration
	mutex       sync.RWMutex
}

// ProgressConfig configures progress tracking behavior
type ProgressConfig struct {
	Operation   string        `json:"operation"`
	Total       int64         `json:"total"`
	LogInterval time.Duration `json:"log_interval"`
	Logger      Logger        `json:"-"`
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(config ProgressConfig) *ProgressTracker {
	if config.Logger == nil {
		config.Logger = GetGlobalLogger()
	}
	if config.LogInterval == 0 {
		config.LogInterval = 5 * time.Second // Default to logging every 5 seconds
	}

	tracker := &ProgressTracker{
		logger:      config.Logger.WithComponent("progress"),
		operation:   config.Operation,
		total:       config.Total,
		startTime:   time.Now(),
		lastLogTime: time.Now(),
		logInterval: config.LogInterval,
	}

	tracker.logger.WithFields(Fields{
		"operation": config.Operation,
		"total":     config.Total,
	}).Info("Starting operation")

	return tracker
}

// Update updates the progress counter
func (p *ProgressTracker) Update(current int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.current = current
	now := time.Now()

	// Log progress at intervals
	if now.Sub(p.lastLogTime) >= p.logInterval {
		p.logProgress(now)
		p.lastLogTime = now
	}
}

// Increment increments the progress counter by 1
func (p *ProgressTracker) Increment() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.current++
	now := time.Now()

	// Log progress at intervals
	if now.Sub(p.lastLogTime) >= p.logInterval {
		p.logProgress(now)
		p.lastLogTime = now
	}
}

// Add increments the progress counter by the given amount
func (p *ProgressTracker) Add(delta int64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.current += delta
	now := time.Now()

	// Log progress at intervals
	if now.Sub(p.lastLogTime) >= p.logInterval {
		p.logProgress(now)
		p.lastLogTime = now
	}
}

// Complete marks the operation as complete and logs final statistics
func (p *ProgressTracker) Complete() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	duration := now.Sub(p.startTime)
	rate := float64(p.current) / duration.Seconds()

	p.logger.WithFields(Fields{
		"operation": p.operation,
		"total":     p.total,
		"processed": p.current,
		"duration":  duration.String(),
		"rate":      fmt.Sprintf("%.2f/sec", rate),
	}).Info("Operation completed")
}

// CompleteWithError marks the operation as complete with error
func (p *ProgressTracker) CompleteWithError(err error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	now := time.Now()
	duration := now.Sub(p.startTime)
	rate := float64(p.current) / duration.Seconds()

	p.logger.WithError(err).WithFields(Fields{
		"operation": p.operation,
		"total":     p.total,
		"processed": p.current,
		"duration":  duration.String(),
		"rate":      fmt.Sprintf("%.2f/sec", rate),
	}).Error("Operation completed with error")
}

// GetStats returns current progress statistics
func (p *ProgressTracker) GetStats() ProgressStats {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	now := time.Now()
	duration := now.Sub(p.startTime)
	var rate float64
	if duration.Seconds() > 0 {
		rate = float64(p.current) / duration.Seconds()
	}

	var percentage float64
	if p.total > 0 {
		percentage = float64(p.current) / float64(p.total) * 100
	}

	var eta time.Duration
	if p.total > 0 && p.current > 0 && rate > 0 {
		remaining := p.total - p.current
		eta = time.Duration(float64(remaining)/rate) * time.Second
	}

	return ProgressStats{
		Operation:  p.operation,
		Total:      p.total,
		Current:    p.current,
		Percentage: percentage,
		Duration:   duration,
		Rate:       rate,
		ETA:        eta,
	}
}

// logProgress logs the current progress
func (p *ProgressTracker) logProgress(now time.Time) {
	duration := now.Sub(p.startTime)
	var rate float64
	if duration.Seconds() > 0 {
		rate = float64(p.current) / duration.Seconds()
	}

	var percentage float64
	if p.total > 0 {
		percentage = float64(p.current) / float64(p.total) * 100
	}

	fields := Fields{
		"operation": p.operation,
		"processed": p.current,
		"rate":      fmt.Sprintf("%.2f/sec", rate),
	}

	if p.total > 0 {
		fields["total"] = p.total
		fields["percentage"] = fmt.Sprintf("%.1f%%", percentage)

		if p.current > 0 && rate > 0 {
			remaining := p.total - p.current
			eta := time.Duration(float64(remaining)/rate) * time.Second
			fields["eta"] = eta.String()
		}
	}

	p.logger.WithFields(fields).Info("Progress update")
}

// ProgressStats contains progress statistics
type ProgressStats struct {
	Operation  string        `json:"operation"`
	Total      int64         `json:"total"`
	Current    int64         `json:"current"`
	Percentage float64       `json:"percentage"`
	Duration   time.Duration `json:"duration"`
	Rate       float64       `json:"rate"`
	ETA        time.Duration `json:"eta,omitempty"`
}

// String returns a human-readable representation of the progress
func (ps ProgressStats) String() string {
	if ps.Total > 0 {
		return fmt.Sprintf("%s: %d/%d (%.1f%%) at %.2f/sec, ETA: %v",
			ps.Operation, ps.Current, ps.Total, ps.Percentage, ps.Rate, ps.ETA)
	}
	return fmt.Sprintf("%s: %d processed at %.2f/sec, elapsed: %v",
		ps.Operation, ps.Current, ps.Rate, ps.Duration)
}

// OperationLogger provides structured logging for operations with timing
type OperationLogger struct {
	logger    Logger
	operation string
	fields    Fields
	startTime time.Time
}

// NewOperationLogger creates a new operation logger
func NewOperationLogger(operation string, logger Logger) *OperationLogger {
	if logger == nil {
		logger = GetGlobalLogger()
	}

	ol := &OperationLogger{
		logger:    logger.WithComponent("operation"),
		operation: operation,
		fields:    make(Fields),
		startTime: time.Now(),
	}

	ol.logger.WithField("operation", operation).Info("Starting operation")
	return ol
}

// WithField adds a field to the operation context
func (ol *OperationLogger) WithField(key string, value interface{}) *OperationLogger {
	ol.fields[key] = value
	return ol
}

// WithFields adds multiple fields to the operation context
func (ol *OperationLogger) WithFields(fields Fields) *OperationLogger {
	for k, v := range fields {
		ol.fields[k] = v
	}
	return ol
}

// Step logs a step within the operation
func (ol *OperationLogger) Step(step string) {
	fields := Fields{
		"operation": ol.operation,
		"step":      step,
	}
	for k, v := range ol.fields {
		fields[k] = v
	}

	ol.logger.WithFields(fields).Info("Operation step")
}

// Progress logs progress information
func (ol *OperationLogger) Progress(message string, processed, total int64) {
	fields := Fields{
		"operation": ol.operation,
		"processed": processed,
		"total":     total,
	}
	if total > 0 {
		fields["percentage"] = fmt.Sprintf("%.1f%%", float64(processed)/float64(total)*100)
	}
	for k, v := range ol.fields {
		fields[k] = v
	}

	ol.logger.WithFields(fields).Info(message)
}

// Success completes the operation successfully
func (ol *OperationLogger) Success(message string) {
	duration := time.Since(ol.startTime)
	fields := Fields{
		"operation": ol.operation,
		"duration":  duration.String(),
		"status":    "success",
	}
	for k, v := range ol.fields {
		fields[k] = v
	}

	ol.logger.WithFields(fields).Info(message)
}

// Error completes the operation with an error
func (ol *OperationLogger) Error(err error, message string) {
	duration := time.Since(ol.startTime)
	fields := Fields{
		"operation": ol.operation,
		"duration":  duration.String(),
		"status":    "error",
	}
	for k, v := range ol.fields {
		fields[k] = v
	}

	ol.logger.WithError(err).WithFields(fields).Error(message)
}

// Warning logs a warning during the operation
func (ol *OperationLogger) Warning(message string) {
	fields := Fields{
		"operation": ol.operation,
	}
	for k, v := range ol.fields {
		fields[k] = v
	}

	ol.logger.WithFields(fields).Warn(message)
}

// TimedOperation executes a function and logs timing information
func TimedOperation(operation string, logger Logger, fn func() error) error {
	ol := NewOperationLogger(operation, logger)
	
	err := fn()
	
	if err != nil {
		ol.Error(err, "Operation failed")
	} else {
		ol.Success("Operation completed successfully")
	}
	
	return err
}