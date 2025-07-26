package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"example.com/sriov-plugin/pkg"
)

func main() {
	var (
		configPath   = flag.String("config", "/etc/sriov-manager/config.json", "Path to configuration file")
		dryRun       = flag.Bool("dry-run", false, "Run in dry-run mode (don't make changes)")
		validate     = flag.Bool("validate", false, "Validate configuration only")
		discover     = flag.Bool("discover", false, "Discover devices only")
		createConfig = flag.Bool("create-config", false, "Create default configuration file")
		version      = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	// Show version
	if *version {
		pkg.Info("SR-IOV Manager v1.0.0")
		return
	}

	// Create default config if requested
	if *createConfig {
		if err := createDefaultConfig(*configPath); err != nil {
			pkg.WithError(err).Fatal("Failed to create default config")
		}
		pkg.WithField("config_path", *configPath).Info("Default configuration created")
		return
	}

	// Load configuration
	config, err := pkg.LoadConfig(*configPath)
	if err != nil {
		pkg.WithError(err).Fatal("Failed to load configuration")
	}

	// Set dry-run mode if requested
	if *dryRun {
		config.DryRun = true
		pkg.Info("Running in dry-run mode")
	}

	// Create SR-IOV manager
	manager := pkg.NewSRIOVManager(config)

	// Validate configuration if requested
	if *validate {
		if err := manager.ValidateConfiguration(); err != nil {
			pkg.WithError(err).Fatal("Configuration validation failed")
		}
		pkg.Info("Configuration validation passed")
		return
	}

	// Discover devices only if requested
	if *discover {
		devices, err := manager.DiscoverDevices()
		if err != nil {
			pkg.WithError(err).Fatal("Device discovery failed")
		}
		pkg.WithField("device_count", len(devices)).Info("Discovered SR-IOV capable devices")
		for _, device := range devices {
			pkg.WithFields(map[string]interface{}{
				"device":  device.Name,
				"pci":     device.PCIAddress,
				"vendor":  device.Vendor,
				"product": device.Product,
			}).Info("Found SR-IOV device")
		}
		return
	}

	// Run as a service
	if err := runAsService(manager); err != nil {
		pkg.WithError(err).Fatal("Service failed")
	}
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(configPath string) error {
	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Create default configuration
	config := pkg.CreateDefaultConfig()

	// Save configuration
	if err := pkg.SaveConfig(config, configPath); err != nil {
		return fmt.Errorf("failed to save config: %v", err)
	}

	return nil
}

// runAsService runs the SR-IOV manager as a systemd service
func runAsService(manager *pkg.SRIOVManager) error {
	pkg.Info("Starting SR-IOV Manager service...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the manager
	go func() {
		if err := manager.Run(); err != nil {
			pkg.WithError(err).Error("SR-IOV Manager failed")
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	pkg.Info("Shutting down SR-IOV Manager service...")

	return nil
}
