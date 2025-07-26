package main

import (
	"os"

	"example.com/sriov-plugin/pkg"
	log "github.com/sirupsen/logrus"
)

func main() {
	// Demonstrate different log levels
	pkg.Info("Starting SR-IOV Manager with Logrus logging")
	pkg.Debug("Debug information - only visible when debug level is set")
	pkg.Warn("Warning message - something to be aware of")
	pkg.Error("Error message - something went wrong")

	// Demonstrate structured logging with fields
	pkg.WithField("component", "device_discovery").Info("Starting device discovery")

	pkg.WithFields(log.Fields{
		"device":  "ens60f0np0",
		"pci":     "0000:09:00.0",
		"vendor":  "Mellanox",
		"product": "ConnectX-7",
		"driver":  "mlx5_core",
	}).Info("Found SR-IOV capable device")

	// Demonstrate error logging with context
	err := &os.PathError{
		Op:   "open",
		Path: "/sys/bus/pci/devices/0000:09:00.0/sriov_numvfs",
		Err:  os.ErrNotExist,
	}

	pkg.WithFields(log.Fields{
		"device": "ens60f0np0",
		"pci":    "0000:09:00.0",
		"action": "enable_sriov",
	}).WithError(err).Error("Failed to enable SR-IOV")

	// Demonstrate different formatters
	pkg.Info("Switching to JSON formatter for production logging")
	pkg.SetFormatter(&log.JSONFormatter{})

	pkg.WithFields(log.Fields{
		"operation": "device_configuration",
		"status":    "completed",
		"devices":   3,
		"success":   true,
	}).Info("Device configuration completed")

	// Switch back to text formatter
	pkg.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
		DisableColors:   false,
	})

	pkg.Info("Switched back to text formatter")
	pkg.WithField("final_status", "success").Info("Logrus migration completed successfully")
}
