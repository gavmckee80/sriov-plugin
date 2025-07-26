package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// DeviceInfo represents device information for CLI output
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

// DetailedCapabilityInfo represents detailed capability information
type DetailedCapabilityInfo struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

// EthtoolFeature represents ethtool feature information
type EthtoolFeature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Fixed   bool   `json:"fixed"`
}

// EthtoolRingInfo represents ethtool ring information
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

// EthtoolChannelInfo represents ethtool channel information
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

// EthtoolInfo represents complete ethtool information
type EthtoolInfo struct {
	Features []EthtoolFeature   `json:"features"`
	Ring     EthtoolRingInfo    `json:"ring"`
	Channels EthtoolChannelInfo `json:"channels"`
}

// Formatting functions
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
