package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sriov-plugin/pkg/types"
	pb "sriov-plugin/proto"
)

func main() {
	var format string
	var pciAddr string
	var interfaceName string
	flag.StringVar(&format, "format", "table", "Output format: table, json, or json-pretty")
	flag.StringVar(&pciAddr, "pci", "", "Filter by PCI address (e.g., 0000:31:00.0)")
	flag.StringVar(&interfaceName, "interface", "", "Filter by interface name (e.g., ens60f0np0)")
	flag.Parse()

	// Connect to the server
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logrus.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)

	// Set timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Request dump from server
	resp, err := client.DumpInterfaces(ctx, &pb.Empty{})
	if err != nil {
		logrus.WithError(err).Fatal("failed to get interface dump")
	}

	// Parse the JSON response
	var sriovData types.SRIOVData
	if err := json.Unmarshal([]byte(resp.JsonData), &sriovData); err != nil {
		logrus.WithError(err).Fatal("failed to parse JSON response")
	}

	// Apply filters if specified
	if pciAddr != "" || interfaceName != "" {
		sriovData = filterSRIOVData(sriovData, pciAddr, interfaceName)
	}

	// Display based on format
	switch format {
	case "table":
		displaySRIOVTable(sriovData)
	case "json":
		displayJSON(sriovData, false)
	case "json-pretty":
		displayJSON(sriovData, true)
	default:
		logrus.Fatal("invalid format. Use: table, json, or json-pretty")
	}
}

// filterSRIOVData filters the SR-IOV data based on PCI address or interface name
func filterSRIOVData(data types.SRIOVData, pciAddr, interfaceName string) types.SRIOVData {
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
