package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

// DeviceInfo holds formatted device information for output
type DeviceInfo struct {
	PCIAddress string `json:"pci_address"`
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Vendor     string `json:"vendor"`
	Product    string `json:"product"`
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
		devices[i] = DeviceInfo{
			PCIAddress: d.PciAddress,
			Name:       d.Name,
			Driver:     d.Driver,
			Vendor:     d.Vendor,
			Product:    d.Product,
		}
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
	fmt.Println("\n📊 SR-IOV Network Devices")
	fmt.Println(strings.Repeat("=", 80))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PCI Address\tName\tDriver\tVendor\tProduct")
	fmt.Fprintln(w, "-----------\t----\t------\t------\t-------")

	for _, d := range devices {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			d.PCIAddress,
			d.Name,
			d.Driver,
			d.Vendor,
			d.Product)
	}
	w.Flush()

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
