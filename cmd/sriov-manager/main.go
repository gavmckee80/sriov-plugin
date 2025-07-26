package main

import (
	"flag"
	"fmt"
	"log"
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
		fmt.Println("SR-IOV Manager v1.0.0")
		return
	}

	// Create default config if requested
	if *createConfig {
		if err := createDefaultConfig(*configPath); err != nil {
			log.Fatalf("Failed to create default config: %v", err)
		}
		fmt.Printf("Default configuration created at: %s\n", *configPath)
		return
	}

	// Load configuration
	config, err := pkg.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Set dry-run mode if requested
	if *dryRun {
		config.DryRun = true
		log.Println("Running in dry-run mode")
	}

	// Create SR-IOV manager
	manager := pkg.NewSRIOVManager(config)

	// Validate configuration if requested
	if *validate {
		if err := manager.ValidateConfiguration(); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
		}
		fmt.Println("Configuration validation passed")
		return
	}

	// Discover devices only if requested
	if *discover {
		devices, err := manager.DiscoverDevices()
		if err != nil {
			log.Fatalf("Device discovery failed: %v", err)
		}
		fmt.Printf("Discovered %d SR-IOV capable devices:\n", len(devices))
		for _, device := range devices {
			fmt.Printf("  - %s (PCI: %s, Vendor: %s, Product: %s)\n",
				device.Name, device.PCIAddress, device.Vendor, device.Product)
		}
		return
	}

	// Run as a service
	if err := runAsService(manager); err != nil {
		log.Fatalf("Service failed: %v", err)
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
	log.Println("Starting SR-IOV Manager service...")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Run the manager
	go func() {
		if err := manager.Run(); err != nil {
			log.Printf("SR-IOV Manager failed: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutting down SR-IOV Manager service...")

	return nil
}
