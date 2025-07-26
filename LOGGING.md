# SR-IOV Plugin Logging System

This document describes the logging system used in the SR-IOV plugin, which is built on top of [Logrus](https://github.com/sirupsen/logrus) - a structured, pluggable logging library for Go.

## Overview

The logging system provides structured logging with different levels, formatters, and hooks. It maintains backward compatibility with existing code while offering advanced features for production environments.

## Features

### Log Levels
- **DEBUG**: Detailed information for debugging
- **INFO**: General information about program execution
- **WARN**: Warning messages for potentially harmful situations
- **ERROR**: Error messages for error conditions

### Structured Logging
Log entries can include structured fields for better context and filtering:

```go
pkg.WithFields(logrus.Fields{
    "device":  "ens60f0np0",
    "pci":     "0000:09:00.0",
    "vendor":  "Mellanox",
    "product": "ConnectX-7",
}).Info("Found SR-IOV device")
```

### Error Context
Errors can be attached to log entries with full context:

```go
pkg.WithFields(logrus.Fields{
    "device": device.Name,
    "action": "enable_sriov",
}).WithError(err).Error("Failed to enable SR-IOV")
```

## Usage

### Basic Logging

```go
import "example.com/sriov-plugin/pkg"

// Simple logging
pkg.Info("Starting SR-IOV Manager")
pkg.Warn("Warning message")
pkg.Error("Error message")
pkg.Debug("Debug information")
```

### Structured Logging

```go
import (
    "example.com/sriov-plugin/pkg"
    log "github.com/sirupsen/logrus"
)

// Single field
pkg.WithField("component", "device_discovery").Info("Starting discovery")

// Multiple fields
pkg.WithFields(log.Fields{
    "device":  "ens60f0np0",
    "pci":     "0000:09:00.0",
    "vendor":  "Mellanox",
    "product": "ConnectX-7",
}).Info("Found SR-IOV device")
```

### Error Logging

```go
err := someOperation()
if err != nil {
    pkg.WithFields(log.Fields{
        "device": device.Name,
        "action": "configure",
    }).WithError(err).Error("Operation failed")
}
```

## Configuration

### Setting Log Level

```go
// Set level from string
pkg.SetLogLevelFromString("debug")  // debug, info, warn, error

// Set level programmatically
pkg.SetLogLevel(pkg.LogLevelDebug)
```

### Formatters

#### Text Formatter (Default)
```go
pkg.SetFormatter(&log.TextFormatter{
    FullTimestamp:   true,
    TimestampFormat: "2006-01-02 15:04:05",
    DisableColors:   false,
})
```

#### JSON Formatter (Production)
```go
pkg.SetFormatter(&log.JSONFormatter{})
```

### Output Configuration

```go
// Set output to file
file, _ := os.OpenFile("sriov.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
pkg.SetOutput(file)

// Set output to multiple destinations
multiWriter := io.MultiWriter(os.Stdout, file)
pkg.SetOutput(multiWriter)
```

## Examples

### Device Discovery Logging

```go
func (m *SRIOVManager) DiscoverDevices() ([]Device, error) {
    pkg.Info("Discovering SR-IOV capable devices...")

    devices, err := ParseLshwDynamic()
    if err != nil {
        return nil, fmt.Errorf("failed to discover devices: %v", err)
    }

    // Enrich with PCI information
    devices, err = AttachPciInfo(devices)
    if err != nil {
        pkg.WithError(err).Warn("Failed to attach PCI info")
    }

    // Filter for SR-IOV capable devices
    var sriovDevices []Device
    for _, device := range devices {
        if device.SRIOVCapable {
            sriovDevices = append(sriovDevices, device)
            pkg.WithFields(log.Fields{
                "device":  device.Name,
                "pci":     device.PCIAddress,
                "vendor":  device.Vendor,
                "product": device.Product,
            }).Info("Found SR-IOV device")
        }
    }

    pkg.WithField("count", len(sriovDevices)).Info("Discovered SR-IOV capable devices")
    return sriovDevices, nil
}
```

### Device Configuration Logging

```go
func (m *SRIOVManager) configureDevice(device Device) error {
    pkg.WithFields(log.Fields{
        "device": device.Name,
        "pci":    device.PCIAddress,
    }).Info("Configuring device")

    // Extract vendor and device IDs
    vendorID, deviceID := m.extractDeviceIDs(device)
    if vendorID == "" || deviceID == "" {
        return fmt.Errorf("could not determine vendor/device IDs for %s", device.Name)
    }

    // Find applicable policy
    policy := m.config.GetDevicePolicy(vendorID, deviceID)
    if policy == nil {
        pkg.WithFields(log.Fields{
            "device":    device.Name,
            "vendor":    vendorID,
            "device_id": deviceID,
        }).Info("No policy found for device")
        return nil
    }

    pkg.WithFields(log.Fields{
        "device":  device.Name,
        "policy":  policy.Description,
        "mode":    string(policy.Mode),
        "num_vfs": policy.NumVFs,
    }).Info("Applying policy for device")

    return nil
}
```

## Production Configuration

For production environments, consider using JSON formatting for better log aggregation:

```go
func init() {
    // Set JSON formatter for production
    pkg.SetFormatter(&log.JSONFormatter{
        TimestampFormat: "2006-01-02T15:04:05.000Z",
    })
    
    // Set appropriate log level
    pkg.SetLogLevelFromString("info")
}
```

## Log Aggregation

The structured logging output is compatible with log aggregation systems like:

- **ELK Stack** (Elasticsearch, Logstash, Kibana)
- **Fluentd**
- **Splunk**
- **Grafana Loki**

### Example JSON Output
```json
{
  "level": "info",
  "msg": "Found SR-IOV device",
  "time": "2025-07-26T16:30:08Z",
  "device": "ens60f0np0",
  "pci": "0000:09:00.0",
  "vendor": "Mellanox",
  "product": "ConnectX-7"
}
```

## Testing

For testing, you can use Logrus's test hook:

```go
import (
    "github.com/sirupsen/logrus"
    "github.com/sirupsen/logrus/hooks/test"
    "github.com/stretchr/testify/assert"
    "testing"
)

func TestLogging(t *testing.T) {
    logger, hook := test.NewNullLogger()
    
    // Replace the default logger for testing
    pkg.SetLogger(logger)
    
    pkg.Info("Test message")
    
    assert.Equal(t, 1, len(hook.Entries))
    assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
    assert.Equal(t, "Test message", hook.LastEntry().Message)
}
```

## Migration from Previous System

The new logging system maintains backward compatibility with the previous simple logging implementation. Existing code using `pkg.Info()`, `pkg.Warn()`, `pkg.Error()`, and `pkg.Debug()` will continue to work without changes.

### Before (Simple Logging)
```go
pkg.Info("Found device: %s", device.Name)
pkg.Warn("Warning: %v", err)
```

### After (Structured Logging)
```go
pkg.WithField("device", device.Name).Info("Found device")
pkg.WithError(err).Warn("Warning")
```

## Benefits

1. **Structured Data**: Log entries include structured fields for better filtering and analysis
2. **Error Context**: Errors are properly attached with full context
3. **Multiple Formatters**: Support for text and JSON output
4. **Production Ready**: Compatible with log aggregation systems
5. **Backward Compatible**: Existing code continues to work
6. **Thread Safe**: Safe for concurrent use
7. **Extensible**: Support for custom hooks and formatters

## References

- [Logrus Documentation](https://github.com/sirupsen/logrus)
- [Structured Logging Best Practices](https://www.thoughtworks.com/insights/blog/structured-logging)
- [Log Aggregation with ELK Stack](https://www.elastic.co/elk-stack) 