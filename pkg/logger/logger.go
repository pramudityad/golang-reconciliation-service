package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger interface defines the logging contract
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields Fields) Logger
	WithError(err error) Logger
	WithComponent(component string) Logger
}

// Fields represents a map of key-value pairs for structured logging
type Fields map[string]interface{}

// Config holds configuration options for the logger
type Config struct {
	Level       Level         `json:"level"`
	Format      Format        `json:"format"`
	Output      Output        `json:"output"`
	File        string        `json:"file,omitempty"`
	MaxSize     int           `json:"max_size,omitempty"`     // MB
	MaxBackups  int           `json:"max_backups,omitempty"`
	MaxAge      int           `json:"max_age,omitempty"`      // days
	Compress    bool          `json:"compress,omitempty"`
	DisableTimestamp bool     `json:"disable_timestamp,omitempty"`
	CallerInfo  bool          `json:"caller_info,omitempty"`
}

// Level represents log levels
type Level string

const (
	DebugLevel Level = "debug"
	InfoLevel  Level = "info"
	WarnLevel  Level = "warn"
	ErrorLevel Level = "error"
	FatalLevel Level = "fatal"
)

// Format represents log output formats
type Format string

const (
	JSONFormat Format = "json"
	TextFormat Format = "text"
)

// Output represents log output destinations
type Output string

const (
	StdoutOutput Output = "stdout"
	StderrOutput Output = "stderr"
	FileOutput   Output = "file"
)

// logrusLogger wraps logrus.Logger to implement our Logger interface
type logrusLogger struct {
	logger *logrus.Logger
	config *Config
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config *Config) (Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid logger configuration: %w", err)
	}

	logger := logrus.New()

	// Set log level
	level, err := logrus.ParseLevel(string(config.Level))
	if err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", config.Level, err)
	}
	logger.SetLevel(level)

	// Set output destination
	writer, err := getOutputWriter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to set log output: %w", err)
	}
	logger.SetOutput(writer)

	// Set formatter
	formatter := getFormatter(config)
	logger.SetFormatter(formatter)

	// Enable caller info if requested
	logger.SetReportCaller(config.CallerInfo)

	return &logrusLogger{
		logger: logger,
		config: config,
	}, nil
}

// DefaultConfig returns a default logger configuration
func DefaultConfig() *Config {
	return &Config{
		Level:       InfoLevel,
		Format:      TextFormat,
		Output:      StderrOutput,
		CallerInfo:  false,
		DisableTimestamp: false,
	}
}

// DebugConfig returns a configuration suitable for debugging
func DebugConfig() *Config {
	return &Config{
		Level:       DebugLevel,
		Format:      TextFormat,
		Output:      StderrOutput,
		CallerInfo:  true,
		DisableTimestamp: false,
	}
}

// ProductionConfig returns a configuration suitable for production
func ProductionConfig() *Config {
	return &Config{
		Level:       InfoLevel,
		Format:      JSONFormat,
		Output:      FileOutput,
		File:        "reconciler.log",
		MaxSize:     100,
		MaxBackups:  5,
		MaxAge:      30,
		Compress:    true,
		CallerInfo:  false,
		DisableTimestamp: false,
	}
}

// Validate validates the logger configuration
func (c *Config) Validate() error {
	// Validate level
	validLevels := map[Level]bool{
		DebugLevel: true,
		InfoLevel:  true,
		WarnLevel:  true,
		ErrorLevel: true,
		FatalLevel: true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("invalid log level: %s", c.Level)
	}

	// Validate format
	validFormats := map[Format]bool{
		JSONFormat: true,
		TextFormat: true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("invalid log format: %s", c.Format)
	}

	// Validate output
	validOutputs := map[Output]bool{
		StdoutOutput: true,
		StderrOutput: true,
		FileOutput:   true,
	}
	if !validOutputs[c.Output] {
		return fmt.Errorf("invalid log output: %s", c.Output)
	}

	// Validate file output settings
	if c.Output == FileOutput {
		if strings.TrimSpace(c.File) == "" {
			return fmt.Errorf("log file path is required for file output")
		}
		if c.MaxSize < 0 {
			return fmt.Errorf("max size cannot be negative")
		}
		if c.MaxBackups < 0 {
			return fmt.Errorf("max backups cannot be negative")
		}
		if c.MaxAge < 0 {
			return fmt.Errorf("max age cannot be negative")
		}
	}

	return nil
}

func getOutputWriter(config *Config) (io.Writer, error) {
	switch config.Output {
	case StdoutOutput:
		return os.Stdout, nil
	case StderrOutput:
		return os.Stderr, nil
	case FileOutput:
		// For now, just return a file writer
		// In a real implementation, you might use lumberjack for log rotation
		if err := os.MkdirAll(filepath.Dir(config.File), 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		file, err := os.OpenFile(config.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		return file, nil
	default:
		return os.Stderr, nil
	}
}

func getFormatter(config *Config) logrus.Formatter {
	switch config.Format {
	case JSONFormat:
		return &logrus.JSONFormatter{
			DisableTimestamp: config.DisableTimestamp,
			TimestampFormat:  time.RFC3339,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				filename := filepath.Base(f.File)
				return fmt.Sprintf("%s()", f.Function), fmt.Sprintf("%s:%d", filename, f.Line)
			},
		}
	case TextFormat:
		return &logrus.TextFormatter{
			DisableTimestamp: config.DisableTimestamp,
			TimestampFormat:  "2006-01-02 15:04:05",
			FullTimestamp:    !config.DisableTimestamp,
			ForceColors:      true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				filename := filepath.Base(f.File)
				return "", fmt.Sprintf("%s:%d", filename, f.Line)
			},
		}
	default:
		return &logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		}
	}
}

// Implement Logger interface

func (l *logrusLogger) Debug(args ...interface{}) {
	l.logger.Debug(args...)
}

func (l *logrusLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debugf(format, args...)
}

func (l *logrusLogger) Info(args ...interface{}) {
	l.logger.Info(args...)
}

func (l *logrusLogger) Infof(format string, args ...interface{}) {
	l.logger.Infof(format, args...)
}

func (l *logrusLogger) Warn(args ...interface{}) {
	l.logger.Warn(args...)
}

func (l *logrusLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warnf(format, args...)
}

func (l *logrusLogger) Error(args ...interface{}) {
	l.logger.Error(args...)
}

func (l *logrusLogger) Errorf(format string, args ...interface{}) {
	l.logger.Errorf(format, args...)
}

func (l *logrusLogger) Fatal(args ...interface{}) {
	l.logger.Fatal(args...)
}

func (l *logrusLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatalf(format, args...)
}

func (l *logrusLogger) WithField(key string, value interface{}) Logger {
	return &logrusLogger{
		logger: l.logger.WithField(key, value).Logger,
		config: l.config,
	}
}

func (l *logrusLogger) WithFields(fields Fields) Logger {
	logrusFields := logrus.Fields(fields)
	return &logrusLogger{
		logger: l.logger.WithFields(logrusFields).Logger,
		config: l.config,
	}
}

func (l *logrusLogger) WithError(err error) Logger {
	return &logrusLogger{
		logger: l.logger.WithError(err).Logger,
		config: l.config,
	}
}

func (l *logrusLogger) WithComponent(component string) Logger {
	return l.WithField("component", component)
}

// Global logger instance
var globalLogger Logger

// Initialize the global logger with default configuration
func init() {
	var err error
	globalLogger, err = NewLogger(DefaultConfig())
	if err != nil {
		// Fallback to basic logging if initialization fails
		logrus.WithError(err).Fatal("Failed to initialize logger")
	}
}

// SetGlobalLogger sets the global logger instance
func SetGlobalLogger(logger Logger) {
	globalLogger = logger
}

// GetGlobalLogger returns the global logger instance
func GetGlobalLogger() Logger {
	return globalLogger
}

// Global logging functions

func Debug(args ...interface{}) {
	globalLogger.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	globalLogger.Debugf(format, args...)
}

func Info(args ...interface{}) {
	globalLogger.Info(args...)
}

func Infof(format string, args ...interface{}) {
	globalLogger.Infof(format, args...)
}

func Warn(args ...interface{}) {
	globalLogger.Warn(args...)
}

func Warnf(format string, args ...interface{}) {
	globalLogger.Warnf(format, args...)
}

func Error(args ...interface{}) {
	globalLogger.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	globalLogger.Errorf(format, args...)
}

func Fatal(args ...interface{}) {
	globalLogger.Fatal(args...)
}

func Fatalf(format string, args ...interface{}) {
	globalLogger.Fatalf(format, args...)
}

func WithField(key string, value interface{}) Logger {
	return globalLogger.WithField(key, value)
}

func WithFields(fields Fields) Logger {
	return globalLogger.WithFields(fields)
}

func WithError(err error) Logger {
	return globalLogger.WithError(err)
}

func WithComponent(component string) Logger {
	return globalLogger.WithComponent(component)
}