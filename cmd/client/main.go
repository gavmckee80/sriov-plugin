package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

// DeviceInfo holds formatted device information for output
type DeviceInfo struct {
	PCIAddress           string                            `json:"pci_address"`
	Name                 string                            `json:"name"`
	Driver               string                            `json:"driver"`
	Vendor               string                            `json:"vendor"`
	Product              string                            `json:"product"`
	SRIOVCapable         bool                              `json:"sriov_capable"`
	DetailedCapabilities map[string]DetailedCapabilityInfo `json:"detailed_capabilities,omitempty"`
}

// DetailedCapabilityInfo holds formatted detailed capability information
type DetailedCapabilityInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
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
		format  = flag.String("format", "table", "Output format: table, json, simple")
		server  = flag.String("server", "localhost:50051", "gRPC server address")
		timeout = flag.Duration("timeout", 5*time.Second, "Connection timeout")
	)
	flag.Parse()

	// Validate format
	outputFormat := OutputFormat(strings.ToLower(*format))
	switch outputFormat {
	case FormatTable, FormatJSON, FormatSimple:
		// Valid format
	default:
		log.Fatalf("❌ Invalid format: %s. Use: table, json, or simple", *format)
	}

	log.Printf("🔌 Connecting to SR-IOV server at %s...", *server)
	conn, err := grpc.Dial(*server, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Printf("✅ Connected successfully")

	c := pb.NewSRIOVManagerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("📋 Requesting device list...")
	r, err := c.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		log.Fatalf("❌ Could not list devices: %v", err)
	}

	log.Printf("✅ Received %d devices", len(r.Devices))

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

		devices[i] = deviceInfo
	}

	// Output based on format
	switch outputFormat {
	case FormatTable:
		printTable(devices)
	case FormatJSON:
		printJSON(devices)
	case FormatSimple:
		printSimple(devices)
	}
}

// printTable prints devices in a formatted table
func printTable(devices []DeviceInfo) {
	fmt.Printf("\n📊 SR-IOV Network Devices\n")
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
		log.Fatalf("❌ Failed to marshal JSON: %v", err)
	}

	fmt.Println(string(jsonData))
}

// printSimple prints devices in simple format
func printSimple(devices []DeviceInfo) {
	fmt.Printf("\n📋 Found %d devices:\n\n", len(devices))

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
