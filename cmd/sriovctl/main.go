package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sriov-plugin/pkg/types"
	pb "sriov-plugin/proto"
)

var (
	// Global flags
	format        string
	pciAddr       string
	interfaceName string
	vendorID      string
	serverAddr    string

	// Vendor ID to name mapping for help text
	vendorMapping = map[string]string{
		"0x1dd8": "Pensando Systems",
		"0x15b3": "Mellanox Technologies",
		"0x8086": "Intel Corporation",
		"0x1002": "Advanced Micro Devices, Inc. [AMD/ATI]",
		"0x10ee": "Xilinx Corporation",
		"0x19e5": "Huawei Technologies Co., Ltd.",
		"0x14e4": "Broadcom Inc.",
		"0x10df": "Emulex Corporation",
		"0x1077": "QLogic Corp.",
		"0x1924": "Solarflare Communications",
	}

	// Root command
	rootCmd = &cobra.Command{
		Use:   "sriovctl",
		Short: "SR-IOV device management and monitoring tool",
		Long: `sriovctl is a command-line tool for managing and monitoring SR-IOV devices.
It connects to the sriovd server to retrieve real-time information about Physical Functions (PFs) 
and Virtual Functions (VFs) on your system.`,
	}

	// List command
	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List SR-IOV devices",
		Long: `List all SR-IOV devices or filter by various criteria.
Examples:
  sriovctl list                           # List all devices
  sriovctl list --vendor=0x1dd8          # List Pensando devices
  sriovctl list --pci=0000:31:00.0       # List specific device
  sriovctl list --interface=ens60f0np0   # List by interface name`,
		Run: runList,
	}

	// Get command
	getCmd = &cobra.Command{
		Use:   "get [pci-address]",
		Short: "Get detailed information about a specific device",
		Long: `Get detailed information about a specific SR-IOV device by PCI address.
Examples:
  sriovctl get 0000:31:00.0              # Get specific device
  sriovctl get 0000:31:00.0 --json       # Get in JSON format`,
		Args: cobra.ExactArgs(1),
		Run:  runGet,
	}

	// Version command
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("sriovctl version 1.0.0")
		},
	}

	// Status command
	statusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show SR-IOV system status",
		Long: `Show overall SR-IOV system status including:
- Total number of PFs and VFs
- Enabled vs disabled SR-IOV devices
- Vendor distribution
- System health summary`,
		Run: runStatus,
	}

	// Vendors command
	vendorsCmd = &cobra.Command{
		Use:   "vendors",
		Short: "List all vendors with SR-IOV devices",
		Long: `List all vendors that have SR-IOV capable devices on the system.
This helps identify what types of network hardware are present.`,
		Run: runVendors,
	}

	// Stats command
	statsCmd = &cobra.Command{
		Use:   "stats",
		Short: "Show SR-IOV statistics",
		Long: `Show detailed statistics about SR-IOV devices including:
- Device counts by vendor
- SR-IOV enablement rates
- VF utilization statistics
- Driver distribution`,
		Run: runStats,
	}
)

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVar(&serverAddr, "server", "localhost:50051", "gRPC server address")
	rootCmd.PersistentFlags().StringVar(&format, "format", "table", "Output format: table, json, or json-pretty")

	// List command flags
	listCmd.Flags().StringVar(&pciAddr, "pci", "", "Filter by PCI address (e.g., 0000:31:00.0)")
	listCmd.Flags().StringVar(&interfaceName, "interface", "", "Filter by interface name (e.g., ens60f0np0)")
	listCmd.Flags().StringVar(&vendorID, "vendor", "", generateVendorHelpText())

	// Get command flags
	getCmd.Flags().StringVar(&interfaceName, "interface", "", "Filter by interface name (e.g., ens60f0np0)")
	getCmd.Flags().StringVar(&vendorID, "vendor", "", generateVendorHelpText())

	// Add commands to root
	rootCmd.AddCommand(listCmd, getCmd, versionCmd, statusCmd, vendorsCmd, statsCmd)
}

// generateVendorHelpText creates a help text string with vendor ID to name mappings
func generateVendorHelpText() string {
	var examples []string
	for id, name := range vendorMapping {
		examples = append(examples, fmt.Sprintf("%s (%s)", id, name))
	}

	// Sort for consistent output
	sort.Strings(examples)

	helpText := "Filter by vendor ID (e.g., 0x1dd8). Common vendors:\n"
	for _, example := range examples {
		helpText += "  " + example + "\n"
	}

	return helpText
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// runList handles the list command
func runList(cmd *cobra.Command, args []string) {
	sriovData, err := getSRIOVData()
	if err != nil {
		logrus.WithError(err).Fatal("failed to get SR-IOV data")
	}

	// Apply filters if specified
	if pciAddr != "" || interfaceName != "" || vendorID != "" {
		sriovData = filterSRIOVData(sriovData, pciAddr, interfaceName, vendorID)
	}

	// Display based on format
	displaySRIOVData(sriovData, format)
}

// runGet handles the get command
func runGet(cmd *cobra.Command, args []string) {
	pciAddr = args[0] // Use the provided PCI address

	sriovData, err := getSRIOVData()
	if err != nil {
		logrus.WithError(err).Fatal("failed to get SR-IOV data")
	}

	// Apply filters
	sriovData = filterSRIOVData(sriovData, pciAddr, interfaceName, vendorID)

	// Display based on format
	displaySRIOVData(sriovData, format)
}

// runStatus handles the status command
func runStatus(cmd *cobra.Command, args []string) {
	sriovData, err := getSRIOVData()
	if err != nil {
		logrus.WithError(err).Fatal("failed to get SR-IOV data")
	}

	// Calculate statistics
	totalPFs := len(sriovData.PhysicalFunctions)
	totalVFs := len(sriovData.VirtualFunctions)
	enabledPFs := 0
	vendorCounts := make(map[string]int)

	for _, pf := range sriovData.PhysicalFunctions {
		if pf.SRIOVEnabled {
			enabledPFs++
		}
		vendorCounts[pf.VendorName]++
	}

	// Display status
	fmt.Println("SR-IOV System Status")
	fmt.Println("====================")
	fmt.Printf("Physical Functions: %d total, %d with SR-IOV enabled\n", totalPFs, enabledPFs)
	fmt.Printf("Virtual Functions: %d total\n", totalVFs)
	fmt.Printf("SR-IOV Enablement Rate: %.1f%%\n", float64(enabledPFs)/float64(totalPFs)*100)
	fmt.Println()
	fmt.Println("Vendors:")
	for vendor, count := range vendorCounts {
		fmt.Printf("  %s: %d devices\n", vendor, count)
	}
}

// runVendors handles the vendors command
func runVendors(cmd *cobra.Command, args []string) {
	sriovData, err := getSRIOVData()
	if err != nil {
		logrus.WithError(err).Fatal("failed to get SR-IOV data")
	}

	// Collect vendor information
	vendors := make(map[string]map[string]int)
	for _, pf := range sriovData.PhysicalFunctions {
		if vendors[pf.VendorName] == nil {
			vendors[pf.VendorName] = make(map[string]int)
		}
		vendors[pf.VendorName]["pfs"]++
		if pf.SRIOVEnabled {
			vendors[pf.VendorName]["enabled_pfs"]++
		}
		vendors[pf.VendorName]["vfs"] += len(pf.VFs)
	}

	// Display vendors
	fmt.Println("SR-IOV Vendors")
	fmt.Println("==============")
	for vendor, stats := range vendors {
		fmt.Printf("%s:\n", vendor)
		fmt.Printf("  Physical Functions: %d\n", stats["pfs"])
		fmt.Printf("  Enabled SR-IOV: %d\n", stats["enabled_pfs"])
		fmt.Printf("  Virtual Functions: %d\n", stats["vfs"])
		fmt.Println()
	}
}

// runStats handles the stats command
func runStats(cmd *cobra.Command, args []string) {
	sriovData, err := getSRIOVData()
	if err != nil {
		logrus.WithError(err).Fatal("failed to get SR-IOV data")
	}

	// Calculate detailed statistics
	stats := struct {
		TotalPFs       int
		EnabledPFs     int
		TotalVFs       int
		VendorStats    map[string]int
		DriverStats    map[string]int
		NUMAStats      map[string]int
		EnablementRate float64
		AvgVFsPerPF    float64
	}{
		VendorStats: make(map[string]int),
		DriverStats: make(map[string]int),
		NUMAStats:   make(map[string]int),
	}

	for _, pf := range sriovData.PhysicalFunctions {
		stats.TotalPFs++
		if pf.SRIOVEnabled {
			stats.EnabledPFs++
		}
		stats.TotalVFs += len(pf.VFs)
		stats.VendorStats[pf.VendorName]++
		stats.DriverStats[pf.Driver]++
		stats.NUMAStats[pf.NUMANode]++
	}

	if stats.TotalPFs > 0 {
		stats.EnablementRate = float64(stats.EnabledPFs) / float64(stats.TotalPFs) * 100
		stats.AvgVFsPerPF = float64(stats.TotalVFs) / float64(stats.TotalPFs)
	}

	// Display statistics
	fmt.Println("SR-IOV Statistics")
	fmt.Println("=================")
	fmt.Printf("Total PFs: %d\n", stats.TotalPFs)
	fmt.Printf("Enabled PFs: %d\n", stats.EnabledPFs)
	fmt.Printf("Total VFs: %d\n", stats.TotalVFs)
	fmt.Printf("SR-IOV Enablement Rate: %.1f%%\n", stats.EnablementRate)
	fmt.Printf("Average VFs per PF: %.1f\n", stats.AvgVFsPerPF)
	fmt.Println()

	fmt.Println("By Vendor:")
	for vendor, count := range stats.VendorStats {
		fmt.Printf("  %s: %d PFs\n", vendor, count)
	}
	fmt.Println()

	fmt.Println("By Driver:")
	for driver, count := range stats.DriverStats {
		fmt.Printf("  %s: %d PFs\n", driver, count)
	}
	fmt.Println()

	fmt.Println("By NUMA Node:")
	for numa, count := range stats.NUMAStats {
		fmt.Printf("  NUMA %s: %d PFs\n", numa, count)
	}
}

// getSRIOVData connects to the server and retrieves SR-IOV data
func getSRIOVData() (types.SRIOVData, error) {
	// Connect to the server
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return types.SRIOVData{}, fmt.Errorf("failed to connect to server: %v", err)
	}
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)

	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Request dump from server
	resp, err := client.DumpInterfaces(ctx, &pb.Empty{})
	if err != nil {
		return types.SRIOVData{}, fmt.Errorf("failed to get interface dump: %v", err)
	}

	// Parse the JSON response
	var sriovData types.SRIOVData
	if err := json.Unmarshal([]byte(resp.JsonData), &sriovData); err != nil {
		return types.SRIOVData{}, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return sriovData, nil
}

// displaySRIOVData displays the SR-IOV data in the specified format
func displaySRIOVData(data types.SRIOVData, format string) {
	switch format {
	case "table":
		displaySRIOVTable(data)
	case "json":
		displayJSON(data, false)
	case "json-pretty":
		displayJSON(data, true)
	default:
		logrus.Fatal("invalid format. Use: table, json, or json-pretty")
	}
}

// filterSRIOVData filters the SR-IOV data based on PCI address, interface name, or vendor ID
func filterSRIOVData(data types.SRIOVData, pciAddr, interfaceName, vendorID string) types.SRIOVData {
	filteredData := types.SRIOVData{
		PhysicalFunctions: make(map[string]*types.PFInfo),
		VirtualFunctions:  make(map[string]*types.VFInfo),
	}

	// Filter Physical Functions
	for pfPCI, pfInfo := range data.PhysicalFunctions {
		// Check if this PF matches our filters
		if pciAddr != "" && pfPCI != pciAddr {
			continue
		}
		if interfaceName != "" && pfInfo.InterfaceName != interfaceName {
			continue
		}
		if vendorID != "" && pfInfo.VendorID != vendorID {
			continue
		}

		// Add this PF to filtered data
		filteredData.PhysicalFunctions[pfPCI] = pfInfo

		// Also add all VFs for this PF to the VF map
		for vfPCI, vfInfo := range pfInfo.VFs {
			filteredData.VirtualFunctions[vfPCI] = vfInfo
		}
	}

	// If no PFs matched but we're looking for a VF by interface name
	if len(filteredData.PhysicalFunctions) == 0 && interfaceName != "" {
		// Search through all VFs to find one with matching interface
		for vfPCI, vfInfo := range data.VirtualFunctions {
			if vfInfo.InterfaceName == interfaceName {
				// Find the parent PF
				for pfPCI, pfInfo := range data.PhysicalFunctions {
					if _, exists := pfInfo.VFs[vfPCI]; exists {
						// Create a filtered version of the PF with only the matching VF
						filteredPF := *pfInfo
						filteredPF.VFs = make(map[string]*types.VFInfo)
						filteredPF.VFs[vfPCI] = vfInfo

						filteredData.PhysicalFunctions[pfPCI] = &filteredPF
						filteredData.VirtualFunctions[vfPCI] = vfInfo
						break
					}
				}
			}
		}
	}

	// If no PFs matched but we're looking for VFs by vendor ID
	if len(filteredData.PhysicalFunctions) == 0 && vendorID != "" {
		// Search through all PFs to find ones with matching vendor ID
		for pfPCI, pfInfo := range data.PhysicalFunctions {
			if pfInfo.VendorID == vendorID {
				filteredData.PhysicalFunctions[pfPCI] = pfInfo
				// Add all VFs for this PF
				for vfPCI, vfInfo := range pfInfo.VFs {
					filteredData.VirtualFunctions[vfPCI] = vfInfo
				}
			}
		}
	}

	return filteredData
}

func displaySRIOVTable(data types.SRIOVData) {
	fmt.Println("SR-IOV Device Information")
	fmt.Println("=" + strings.Repeat("=", 100))
	fmt.Println()

	// Display Physical Functions with their VFs
	for pfPCI, pfInfo := range data.PhysicalFunctions {
		fmt.Printf("Physical Function: %s\n", pfPCI)
		fmt.Printf("  Interface: %s\n", pfInfo.InterfaceName)
		fmt.Printf("  Driver: %s\n", pfInfo.Driver)
		fmt.Printf("  Class: %s\n", pfInfo.DeviceClass)
		fmt.Printf("  Description: %s\n", pfInfo.Description)
		fmt.Printf("  Vendor ID: %s, Device ID: %s\n", pfInfo.VendorID, pfInfo.DeviceID)
		if pfInfo.VendorName != "" {
			fmt.Printf("  Vendor: %s\n", pfInfo.VendorName)
		}
		if pfInfo.DeviceName != "" {
			fmt.Printf("  Device: %s\n", pfInfo.DeviceName)
		}
		if pfInfo.SubsysVendorName != "" {
			fmt.Printf("  Subsys Vendor: %s\n", pfInfo.SubsysVendorName)
		}
		if pfInfo.SubsysDeviceName != "" {
			fmt.Printf("  Subsys Device: %s\n", pfInfo.SubsysDeviceName)
		}
		fmt.Printf("  NUMA Node: %s\n", pfInfo.NUMANode)
		fmt.Printf("  Total VFs: %d, Enabled VFs: %d\n", pfInfo.TotalVFs, pfInfo.NumVFs)
		fmt.Printf("  SR-IOV Enabled: %t\n", pfInfo.SRIOVEnabled)
		fmt.Println()

		if len(pfInfo.VFs) > 0 {
			fmt.Println("  Virtual Functions:")
			fmt.Printf("  %-15s %-20s %-15s %-10s %-10s %-15s %-30s %-30s\n", "PCI Address", "Interface", "Driver", "NUMA Node", "VF Index", "Class", "Vendor", "Device")
			fmt.Printf("  %-15s %-20s %-15s %-10s %-10s %-15s %-30s %-30s\n",
				strings.Repeat("-", 15), strings.Repeat("-", 20), strings.Repeat("-", 15),
				strings.Repeat("-", 10), strings.Repeat("-", 10), strings.Repeat("-", 15), strings.Repeat("-", 30), strings.Repeat("-", 30))

			for vfPCI, vfInfo := range pfInfo.VFs {
				// Truncate vendor and device names if too long
				vendorName := vfInfo.VendorName
				if len(vendorName) > 27 {
					vendorName = vendorName[:24] + "..."
				}
				deviceName := vfInfo.DeviceName
				if len(deviceName) > 27 {
					deviceName = deviceName[:24] + "..."
				}

				fmt.Printf("  %-15s %-20s %-15s %-10s %-10d %-15s %-30s %-30s\n",
					vfPCI,
					vfInfo.InterfaceName,
					vfInfo.Driver,
					vfInfo.NUMANode,
					vfInfo.VFIndex,
					vfInfo.DeviceClass,
					vendorName,
					deviceName)
			}
		} else {
			fmt.Println("  No Virtual Functions")
		}
		fmt.Println()
		fmt.Println(strings.Repeat("-", 120))
		fmt.Println()
	}

	// Summary
	fmt.Printf("Summary: %d Physical Functions, %d Virtual Functions\n", len(data.PhysicalFunctions), len(data.VirtualFunctions))
}

func displayJSON(data types.SRIOVData, pretty bool) {
	var jsonData []byte
	var err error

	if pretty {
		jsonData, err = json.MarshalIndent(data, "", "  ")
	} else {
		jsonData, err = json.Marshal(data)
	}

	if err != nil {
		logrus.WithError(err).Fatal("failed to marshal JSON")
	}

	fmt.Println(string(jsonData))
}
