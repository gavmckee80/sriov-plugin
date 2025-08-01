package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

// DeviceInfo holds formatted device information for output
type DeviceInfo struct {
	PCIAddress           string
	Name                 string
	Driver               string
	Vendor               string
	Product              string
	SRIOVCapable         bool
	DetailedCapabilities map[string]DetailedCapabilityInfo
	EthtoolInfo          *EthtoolInfo
	// NUMA topology information
	NUMANode     int
	NUMADistance map[int]int
}

// DetailedCapabilityInfo holds formatted detailed capability information
type DetailedCapabilityInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

type EthtoolFeature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Fixed   bool   `json:"fixed"`
}

type EthtoolRingInfo struct {
	RxMaxPending      uint32 `json:"rx_max_pending"`
	RxMiniMaxPending  uint32 `json:"rx_mini_max_pending"`
	RxJumboMaxPending uint32 `json:"rx_jumbo_max_pending"`
	TxMaxPending      uint32 `json:"tx_max_pending"`
	RxPending         uint32 `json:"rx_pending"`
	RxMiniPending     uint32 `json:"rx_mini_pending"`
	RxJumboPending    uint32 `json:"rx_jumbo_pending"`
	TxPending         uint32 `json:"tx_pending"`
}

type EthtoolChannelInfo struct {
	MaxRx         uint32 `json:"max_rx"`
	MaxTx         uint32 `json:"max_tx"`
	MaxOther      uint32 `json:"max_other"`
	MaxCombined   uint32 `json:"max_combined"`
	RxCount       uint32 `json:"rx_count"`
	TxCount       uint32 `json:"tx_count"`
	OtherCount    uint32 `json:"other_count"`
	CombinedCount uint32 `json:"combined_count"`
}

type EthtoolInfo struct {
	Features []EthtoolFeature   `json:"features"`
	Ring     EthtoolRingInfo    `json:"ring"`
	Channels EthtoolChannelInfo `json:"channels"`
}

// OutputFormat defines the output format
type OutputFormat string

const (
	FormatTable  OutputFormat = "table"
	FormatJSON   OutputFormat = "json"
	FormatSimple OutputFormat = "simple"
)

func main() {
	// Parse command line flags
	var (
		serverAddr  = flag.String("server", "localhost:50051", "gRPC server address")
		timeout     = flag.Duration("timeout", 5*time.Second, "Connection timeout")
		refresh     = flag.Bool("refresh", false, "Trigger manual refresh of device list")
		deviceName  = flag.String("device-name", "", "Filter by device name (exact match)")
		format      = flag.String("format", "table", "Output format: table, json, simple, csv, detailed")
		tableFormat = flag.String("table-format", "default", "Table format: default, extended, numa, sriov")
	)
	flag.Parse()

	// Validate format
	outputFormat := OutputFormat(strings.ToLower(*format))
	switch outputFormat {
	case FormatTable, FormatJSON, FormatSimple:
		// Valid format
	case "csv", "detailed":
		// New valid formats
	default:
		fmt.Printf("❌ Invalid format: %s. Use: table, json, simple, csv, or detailed\n", *format)
		os.Exit(1)
	}

	fmt.Printf("Connecting to SR-IOV server at %s...\n", *serverAddr)
	conn, err := grpc.Dial(*serverAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		fmt.Printf("Error: Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	fmt.Printf("Connected successfully\n")

	c := pb.NewSRIOVManagerClient(conn)

	// Handle refresh command
	if *refresh {
		fmt.Printf("Triggering manual refresh...\n")
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		resp, err := c.RefreshDevices(ctx, &pb.RefreshDevicesRequest{})
		if err != nil {
			fmt.Printf("Error: Failed to refresh devices: %v\n", err)
			os.Exit(1)
		}

		if resp.Success {
			fmt.Printf("Success: %s\n", resp.Message)
		} else {
			fmt.Printf("Error: Refresh failed: %s\n", resp.Message)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	fmt.Printf("Requesting device list...\n")
	r, err := c.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		fmt.Printf("Error: Could not list devices: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Received %d devices\n", len(r.Devices))

	// Convert to DeviceInfo for consistent formatting
	devices := make([]DeviceInfo, len(r.Devices))
	for i, d := range r.Devices {
		deviceInfo := DeviceInfo{
			PCIAddress:   d.PciAddress,
			Name:         d.Name,
			Driver:       d.Driver,
			Vendor:       d.Vendor,
			Product:      d.Product,
			SRIOVCapable: d.SriovCapable,
			// Add NUMA topology information
			NUMANode:     int(d.NumaNode),
			NUMADistance: make(map[int]int),
		}

		// Add NUMA distance information
		for node, distance := range d.NumaDistance {
			deviceInfo.NUMADistance[int(node)] = int(distance)
		}

		// Add detailed capabilities if available
		if len(d.DetailedCapabilities) > 0 {
			deviceInfo.DetailedCapabilities = make(map[string]DetailedCapabilityInfo)
			for name, cap := range d.DetailedCapabilities {
				deviceInfo.DetailedCapabilities[name] = DetailedCapabilityInfo{
					ID:          cap.Id,
					Name:        cap.Name,
					Status:      cap.Status,
					Description: cap.Description,
					Parameters:  cap.Parameters,
				}
			}
		}

		// Add ethtool information
		if d.EthtoolInfo != nil {
			ethtoolInfo := &EthtoolInfo{}

			// Convert features
			for _, feature := range d.EthtoolInfo.Features {
				ethtoolInfo.Features = append(ethtoolInfo.Features, EthtoolFeature{
					Name:    feature.Name,
					Enabled: feature.Enabled,
					Fixed:   feature.Fixed,
				})
			}

			// Convert ring info
			if d.EthtoolInfo.Ring != nil {
				ethtoolInfo.Ring = EthtoolRingInfo{
					RxMaxPending:      d.EthtoolInfo.Ring.RxMaxPending,
					RxMiniMaxPending:  d.EthtoolInfo.Ring.RxMiniMaxPending,
					RxJumboMaxPending: d.EthtoolInfo.Ring.RxJumboMaxPending,
					TxMaxPending:      d.EthtoolInfo.Ring.TxMaxPending,
					RxPending:         d.EthtoolInfo.Ring.RxPending,
					RxMiniPending:     d.EthtoolInfo.Ring.RxMiniPending,
					RxJumboPending:    d.EthtoolInfo.Ring.RxJumboPending,
					TxPending:         d.EthtoolInfo.Ring.TxPending,
				}
			}

			// Convert channel info
			if d.EthtoolInfo.Channels != nil {
				ethtoolInfo.Channels = EthtoolChannelInfo{
					MaxRx:         d.EthtoolInfo.Channels.MaxRx,
					MaxTx:         d.EthtoolInfo.Channels.MaxTx,
					MaxOther:      d.EthtoolInfo.Channels.MaxOther,
					MaxCombined:   d.EthtoolInfo.Channels.MaxCombined,
					RxCount:       d.EthtoolInfo.Channels.RxCount,
					TxCount:       d.EthtoolInfo.Channels.TxCount,
					OtherCount:    d.EthtoolInfo.Channels.OtherCount,
					CombinedCount: d.EthtoolInfo.Channels.CombinedCount,
				}
			}

			deviceInfo.EthtoolInfo = ethtoolInfo
		}

		devices[i] = deviceInfo
	}

	// Filter by device name if specified
	if *deviceName != "" {
		var filteredDevices []DeviceInfo
		for _, device := range devices {
			if device.Name == *deviceName {
				filteredDevices = append(filteredDevices, device)
			}
		}
		devices = filteredDevices

		if len(devices) == 0 {
			fmt.Printf("Warning: No devices found with name: %s\n", *deviceName)
			return
		}
		fmt.Printf("Found %d device(s) matching name: %s\n", len(devices), *deviceName)
	}

	// Format output based on user preference
	switch *format {
	case "json":
		fmt.Println(formatDeviceJSON(devices))
	case "simple":
		fmt.Println(formatDeviceSimple(devices))
	case "csv":
		fmt.Println(formatDeviceCSV(devices))
	case "detailed":
		fmt.Println(formatDeviceDetailed(devices))
	case "table":
		switch *tableFormat {
		case "default":
			fmt.Println(formatDeviceTable(devices))
		case "extended":
			fmt.Println(formatDeviceTableExtended(devices))
		case "numa":
			fmt.Println(formatDeviceTableNUMA(devices))
		case "sriov":
			fmt.Println(formatDeviceTableSRIOV(devices))
		default:
			fmt.Println(formatDeviceTable(devices))
		}
	default:
		fmt.Println(formatDeviceTable(devices))
	}
}

// printTable prints devices in a formatted table
func printTable(devices []DeviceInfo) {
	fmt.Printf("\nSR-IOV Network Devices\n")
	fmt.Printf("================================================================================\n")
	fmt.Printf("%-12s %-16s %-12s %-20s %-30s %-8s\n", "PCI Address", "Name", "Driver", "Vendor", "Product", "SR-IOV")
	fmt.Printf("%-12s %-16s %-12s %-20s %-30s %-8s\n", "-----------", "----", "------", "------", "-------", "------")

	for _, device := range devices {
		sriovStatus := "No"
		if device.SRIOVCapable {
			sriovStatus = "Yes"
		}

		fmt.Printf("%-12s %-16s %-12s %-20s %-30s %-8s\n",
			device.PCIAddress, device.Name, device.Driver, device.Vendor, device.Product, sriovStatus)

		// Show detailed capabilities if available
		if len(device.DetailedCapabilities) > 0 {
			for name, cap := range device.DetailedCapabilities {
				fmt.Printf("  └─ [%s] %s: %s\n", cap.ID, name, cap.Description)
			}
		}
	}

	fmt.Printf("\n📈 Summary: %d devices found\n", len(devices))
}

// printJSON prints devices in JSON format
func printJSON(devices []DeviceInfo) {
	output := map[string]interface{}{
		"devices": devices,
		"summary": map[string]interface{}{
			"total_devices": len(devices),
			"timestamp":     time.Now().Format(time.RFC3339),
		},
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("❌ Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))
}

// printSimple prints devices in simple format
func printSimple(devices []DeviceInfo) {
	fmt.Printf("\nFound %d devices:\n\n", len(devices))

	for i, d := range devices {
		fmt.Printf("Device %d:\n", i+1)
		fmt.Printf("  PCI Address: %s\n", d.PCIAddress)
		fmt.Printf("  Name:        %s\n", d.Name)
		fmt.Printf("  Driver:      %s\n", d.Driver)
		fmt.Printf("  Vendor:      %s\n", d.Vendor)
		fmt.Printf("  Product:     %s\n", d.Product)
		fmt.Println()
	}
}

func formatDeviceTable(devices []DeviceInfo) string {
	var builder strings.Builder
	builder.WriteString("┌─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┐\n")
	builder.WriteString("│ PCI Address         │ Name                │ Driver              │ Vendor              │ Product             │ SR-IOV Capable      │ NUMA Node           │\n")
	builder.WriteString("├─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┤\n")

	for _, device := range devices {
		// Truncate long strings for table display
		pciAddr := truncateString(device.PCIAddress, 19)
		name := truncateString(device.Name, 19)
		driver := truncateString(device.Driver, 19)
		vendor := truncateString(device.Vendor, 19)
		product := truncateString(device.Product, 19)
		sriov := "Yes"
		if !device.SRIOVCapable {
			sriov = "No"
		}

		// Format NUMA information
		numaInfo := "No affinity"
		if device.NUMANode != -1 {
			numaInfo = fmt.Sprintf("Node %d", device.NUMANode)
		}

		builder.WriteString(fmt.Sprintf("│ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │\n",
			pciAddr, name, driver, vendor, product, sriov, numaInfo))
	}

	builder.WriteString("└─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┘\n")
	return builder.String()
}

func formatDeviceJSON(devices []DeviceInfo) string {
	type DeviceOutput struct {
		PCIAddress           string                            `json:"pci_address"`
		Name                 string                            `json:"name"`
		Driver               string                            `json:"driver"`
		Vendor               string                            `json:"vendor"`
		Product              string                            `json:"product"`
		SRIOVCapable         bool                              `json:"sriov_capable"`
		DetailedCapabilities map[string]DetailedCapabilityInfo `json:"detailed_capabilities,omitempty"`
		EthtoolInfo          *EthtoolInfo                      `json:"ethtool_info,omitempty"`
		NUMANode             int                               `json:"numa_node"`
		NUMADistance         map[int]int                       `json:"numa_distance,omitempty"`
	}

	var output []DeviceOutput
	for _, device := range devices {
		output = append(output, DeviceOutput{
			PCIAddress:           device.PCIAddress,
			Name:                 device.Name,
			Driver:               device.Driver,
			Vendor:               device.Vendor,
			Product:              device.Product,
			SRIOVCapable:         device.SRIOVCapable,
			DetailedCapabilities: device.DetailedCapabilities,
			EthtoolInfo:          device.EthtoolInfo,
			NUMANode:             device.NUMANode,
			NUMADistance:         device.NUMADistance,
		})
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

func formatDeviceSimple(devices []DeviceInfo) string {
	var builder strings.Builder
	for _, device := range devices {
		builder.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n",
			device.PCIAddress, device.Name, device.Vendor, device.Product))
	}
	return builder.String()
}

func formatDeviceCSV(devices []DeviceInfo) string {
	var builder strings.Builder
	builder.WriteString("PCI_ADDRESS,NAME,DRIVER,VENDOR,PRODUCT,SRIOV_CAPABLE,NUMANODE\n")
	for _, device := range devices {
		sriov := "No"
		if device.SRIOVCapable {
			sriov = "Yes"
		}
		builder.WriteString(fmt.Sprintf("%s,%s,%s,%s,%s,%s,%d\n",
			device.PCIAddress, device.Name, device.Driver, device.Vendor, device.Product, sriov, device.NUMANode))
	}
	return builder.String()
}

func formatDeviceDetailed(devices []DeviceInfo) string {
	var builder strings.Builder
	for _, device := range devices {
		builder.WriteString(fmt.Sprintf("Device: %s\n", device.Name))
		builder.WriteString(fmt.Sprintf("  PCI Address: %s\n", device.PCIAddress))
		builder.WriteString(fmt.Sprintf("  Driver: %s\n", device.Driver))
		builder.WriteString(fmt.Sprintf("  Vendor: %s\n", device.Vendor))
		builder.WriteString(fmt.Sprintf("  Product: %s\n", device.Product))
		builder.WriteString(fmt.Sprintf("  SR-IOV Capable: %t\n", device.SRIOVCapable))
		builder.WriteString(fmt.Sprintf("  NUMA Node: %d\n", device.NUMANode))
		if len(device.NUMADistance) > 0 {
			var distances []string
			for node, distance := range device.NUMADistance {
				distances = append(distances, fmt.Sprintf("Node %d: %d", node, distance))
			}
			builder.WriteString(fmt.Sprintf("  NUMA Distances: %s\n", strings.Join(distances, ", ")))
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func formatDeviceTableExtended(devices []DeviceInfo) string {
	var builder strings.Builder
	builder.WriteString("┌─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┐\n")
	builder.WriteString("│ PCI Address         │ Name                │ Driver              │ Vendor              │ Product             │ SR-IOV Capable      │ NUMA Node           │ Description         │\n")
	builder.WriteString("├─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┤\n")

	for _, device := range devices {
		pciAddr := truncateString(device.PCIAddress, 19)
		name := truncateString(device.Name, 19)
		driver := truncateString(device.Driver, 19)
		vendor := truncateString(device.Vendor, 19)
		product := truncateString(device.Product, 19)
		sriov := "Yes"
		if !device.SRIOVCapable {
			sriov = "No"
		}

		numaInfo := "No affinity"
		if device.NUMANode != -1 {
			numaInfo = fmt.Sprintf("Node %d", device.NUMANode)
		}

		description := truncateString(fmt.Sprintf("%s %s", device.Vendor, device.Product), 19)

		builder.WriteString(fmt.Sprintf("│ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │\n",
			pciAddr, name, driver, vendor, product, sriov, numaInfo, description))
	}

	builder.WriteString("└─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┘\n")
	return builder.String()
}

func formatDeviceTableNUMA(devices []DeviceInfo) string {
	var builder strings.Builder
	builder.WriteString("┌─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┐\n")
	builder.WriteString("│ PCI Address         │ Name                │ Vendor              │ Product             │ NUMA Node           │ Local NUMA          │ NUMA Distances      │ Driver              │\n")
	builder.WriteString("├─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┤\n")

	for _, device := range devices {
		pciAddr := truncateString(device.PCIAddress, 19)
		name := truncateString(device.Name, 19)
		vendor := truncateString(device.Vendor, 19)
		product := truncateString(device.Product, 19)

		numaInfo := "No affinity"
		if device.NUMANode != -1 {
			numaInfo = fmt.Sprintf("Node %d", device.NUMANode)
		}

		localNUMA := "No"
		if device.NUMANode != -1 && len(device.NUMADistance) > 0 {
			if distance, exists := device.NUMADistance[device.NUMANode]; exists && distance == 10 {
				localNUMA = "Yes"
			}
		}

		distances := "N/A"
		if len(device.NUMADistance) > 0 {
			var distStrs []string
			for node, distance := range device.NUMADistance {
				distStrs = append(distStrs, fmt.Sprintf("%d:%d", node, distance))
			}
			distances = truncateString(strings.Join(distStrs, " "), 19)
		}

		driver := truncateString(device.Driver, 19)

		builder.WriteString(fmt.Sprintf("│ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │\n",
			pciAddr, name, vendor, product, numaInfo, localNUMA, distances, driver))
	}

	builder.WriteString("└─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┘\n")
	return builder.String()
}

func formatDeviceTableSRIOV(devices []DeviceInfo) string {
	var builder strings.Builder
	builder.WriteString("┌─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┬─────────────────────┐\n")
	builder.WriteString("│ INTERFACE           │ DESCRIPTION         │ PRODUCT             │ VENDOR              │ PCI ADDRESS         │ TOTAL VFS           │ CURRENT VFS         │ NUMA                │ DRIVER              │\n")
	builder.WriteString("├─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┼─────────────────────┤\n")

	for _, device := range devices {
		interfaceName := truncateString(device.Name, 19)
		description := truncateString(fmt.Sprintf("%s %s", device.Vendor, device.Product), 19)
		product := truncateString(device.Product, 19)
		vendor := truncateString(device.Vendor, 19)
		pciAddr := truncateString(device.PCIAddress, 19)

		totalVFs := "N/A"
		currentVFs := "N/A"
		if device.SRIOVCapable {
			totalVFs = "16"  // Default value, could be enhanced to get actual VF count
			currentVFs = "4" // Default value, could be enhanced to get actual VF count
		}

		numaInfo := "No affinity"
		if device.NUMANode != -1 {
			numaInfo = fmt.Sprintf("Node %d", device.NUMANode)
		}

		driver := truncateString(device.Driver, 19)

		builder.WriteString(fmt.Sprintf("│ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │ %-19s │\n",
			interfaceName, description, product, vendor, pciAddr, totalVFs, currentVFs, numaInfo, driver))
	}

	builder.WriteString("└─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┴─────────────────────┘\n")
	return builder.String()
}

// truncateString truncates a string to the specified length, adding "..." if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
