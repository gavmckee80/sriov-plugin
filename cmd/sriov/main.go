package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sriov",
	Short: "SR-IOV Plugin - A comprehensive tool for managing SR-IOV devices",
	Long: `SR-IOV Plugin is a powerful CLI tool for discovering, monitoring, and managing 
SR-IOV (Single Root I/O Virtualization) devices on Linux systems.

Features:
  • Device discovery and enumeration
  • NUMA topology detection
  • Multiple output formats (table, JSON, CSV)
  • Real-time device monitoring
  • SR-IOV capability detection
  • Detailed device information

Examples:
  sriov list                    # List all devices
  sriov list --format json     # List devices in JSON format
  sriov list --table-format sriov  # SR-IOV specific table format
  sriov server                  # Start the gRPC server
  sriov monitor                 # Monitor devices in real-time`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
