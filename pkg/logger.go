package pkg

import (
	"fmt"
	"io"
	"strings"

	log "github.com/sirupsen/logrus"
)

// LogLevel represents the logging level (compatibility with existing code)
type LogLevel int

const (
	// LogLevelError represents error level logging
	LogLevelError LogLevel = iota
	// LogLevelWarn represents warning level logging
	LogLevelWarn
	// LogLevelInfo represents info level logging
	LogLevelInfo
	// LogLevelDebug represents debug level logging
	LogLevelDebug
)

// Logger provides structured logging with different levels
// This is now a wrapper around logrus.Logger for compatibility
type Logger struct {
	logger *log.Logger
}

var (
	// Default logger instance
	defaultLogger *Logger
)

func init() {
	// Initialize default logger with info level
	defaultLogger = NewLogger(LogLevelInfo)

	// Configure logrus defaults
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   false,
	})
}

// NewLogger creates a new logger with the specified level
func NewLogger(level LogLevel) *Logger {
	logger := log.New()
	logger.SetLevel(logrusLevelFromLogLevel(level))

	return &Logger{
		logger: logger,
	}
}

// logrusLevelFromLogLevel converts our LogLevel to logrus.Level
func logrusLevelFromLogLevel(level LogLevel) log.Level {
	switch level {
	case LogLevelDebug:
		return log.DebugLevel
	case LogLevelInfo:
		return log.InfoLevel
	case LogLevelWarn:
		return log.WarnLevel
	case LogLevelError:
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}

// SetLogLevel sets the log level for the default logger
func SetLogLevel(level LogLevel) {
	defaultLogger.logger.SetLevel(logrusLevelFromLogLevel(level))
}

// SetLogLevelFromString sets the log level from a string
func SetLogLevelFromString(levelStr string) error {
	level := LogLevelInfo
	switch strings.ToLower(levelStr) {
	case "debug":
		level = LogLevelDebug
	case "info":
		level = LogLevelInfo
	case "warn", "warning":
		level = LogLevelWarn
	case "error":
		level = LogLevelError
	default:
		return fmt.Errorf("invalid log level: %s", levelStr)
	}
	SetLogLevel(level)
	return nil
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	defaultLogger.logger.Debugf(format, args...)
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	defaultLogger.logger.Infof(format, args...)
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	defaultLogger.logger.Warnf(format, args...)
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	defaultLogger.logger.Errorf(format, args...)
}

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	return defaultLogger
}

// IsDebugEnabled returns true if debug logging is enabled
func IsDebugEnabled() bool {
	return defaultLogger.logger.GetLevel() >= log.DebugLevel
}

// IsInfoEnabled returns true if info logging is enabled
func IsInfoEnabled() bool {
	return defaultLogger.logger.GetLevel() >= log.InfoLevel
}

// SetFormatter sets the formatter for the default logger
func SetFormatter(formatter log.Formatter) {
	defaultLogger.logger.SetFormatter(formatter)
}

// SetOutput sets the output for the default logger
func SetOutput(output io.Writer) {
	defaultLogger.logger.SetOutput(output)
}

// WithField adds a field to the logger
func WithField(key string, value interface{}) *log.Entry {
	return defaultLogger.logger.WithField(key, value)
}

// WithFields adds multiple fields to the logger
func WithFields(fields log.Fields) *log.Entry {
	return defaultLogger.logger.WithFields(fields)
}

// WithError adds an error field to the logger
func WithError(err error) *log.Entry {
	return defaultLogger.logger.WithError(err)
}
