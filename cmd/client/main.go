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
	EthtoolInfo          *EthtoolInfo                      `json:"ethtool_info,omitempty"`
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
		format     = flag.String("format", "table", "Output format: table, json, simple")
		server     = flag.String("server", "localhost:50051", "gRPC server address")
		timeout    = flag.Duration("timeout", 5*time.Second, "Connection timeout")
		deviceName = flag.String("device-name", "", "Filter by device name (exact match)")
		refresh    = flag.Bool("refresh", false, "Trigger manual refresh of device list")
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

	log.Printf("Connecting to SR-IOV server at %s...", *server)
	conn, err := grpc.Dial(*server, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Error: Failed to connect: %v", err)
	}
	defer conn.Close()

	log.Printf("Connected successfully")

	c := pb.NewSRIOVManagerClient(conn)

	// Handle refresh command
	if *refresh {
		log.Printf("Triggering manual refresh...")
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		resp, err := c.RefreshDevices(ctx, &pb.RefreshDevicesRequest{})
		if err != nil {
			log.Fatalf("Error: Failed to refresh devices: %v", err)
		}

		if resp.Success {
			log.Printf("Success: %s", resp.Message)
		} else {
			log.Printf("Error: Refresh failed: %s", resp.Message)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	log.Printf("Requesting device list...")
	r, err := c.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		log.Fatalf("Error: Could not list devices: %v", err)
	}

	log.Printf("Received %d devices", len(r.Devices))

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
			log.Printf("Warning: No devices found with name: %s", *deviceName)
			return
		}
		log.Printf("Found %d device(s) matching name: %s", len(devices), *deviceName)
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
