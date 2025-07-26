package pkg

import (
	"bytes"
	"strings"
	"testing"

	log "github.com/sirupsen/logrus"
)

func TestLogrusIntegration(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(log.StandardLogger().Out) // Restore default output

	// Test basic logging
	Info("Test info message")
	Warn("Test warning message")
	Error("Test error message")

	output := buf.String()

	// Verify log messages are present
	if !strings.Contains(output, "Test info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "Test warning message") {
		t.Error("Warning message not found in output")
	}
	if !strings.Contains(output, "Test error message") {
		t.Error("Error message not found in output")
	}
}

func TestStructuredLogging(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(log.StandardLogger().Out)

	// Test structured logging
	WithField("device", "test-device").Info("Device found")
	WithFields(log.Fields{
		"pci":     "0000:01:00.0",
		"vendor":  "Test Vendor",
		"product": "Test Product",
	}).Info("Device details")

	output := buf.String()

	// Verify structured fields are present
	if !strings.Contains(output, "device=test-device") {
		t.Error("Device field not found in structured log")
	}
	if !strings.Contains(output, "pci=\"0000:01:00.0\"") {
		t.Error("PCI field not found in structured log")
	}
}

func TestLogLevels(t *testing.T) {
	// Test log level setting
	defer SetLogLevel(LogLevelInfo) // Restore original level

	// Set to debug level
	SetLogLevelFromString("debug")
	if !IsDebugEnabled() {
		t.Error("Debug level should be enabled")
	}

	// Set to warn level
	SetLogLevelFromString("warn")
	if IsDebugEnabled() {
		t.Error("Debug level should be disabled")
	}
	// Info level should be disabled when set to warn level
	if IsInfoEnabled() {
		t.Error("Info level should be disabled when set to warn level")
	}
}

func TestErrorLogging(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	SetOutput(&buf)
	defer SetOutput(log.StandardLogger().Out)

	// Test error logging with context
	err := &testError{message: "test error"}
	WithError(err).Error("Operation failed")

	output := buf.String()

	// Verify error is included
	if !strings.Contains(output, "test error") {
		t.Error("Error message not found in log output")
	}
}

// testError implements error interface for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}
