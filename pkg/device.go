package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// parsePciDevices is defined as a variable so it can be
// overridden in tests.
// Default to sysfs implementation for better performance and reliability
var parsePciDevices = ParseSysfsPciDevices

// SetParsePciDevices allows overriding the parsePciDevices function for testing
func SetParsePciDevices(fn func() ([]SysfsPciDevice, error)) {
	parsePciDevices = fn
}

// Device holds information about a network device
// parsed from lshw and enriched with sysfs PCI information
type Device struct {
	PCIAddress string
	Name       string
	Driver     string
	Vendor     string
	Product    string
	// Enhanced fields for SR-IOV information
	SRIOVCapable bool
	SRIOVInfo    *SRIOVInfo
	// Additional context from lshw
	Description  string
	Serial       string
	Size         string
	Capacity     string
	Clock        string
	Width        string
	Capabilities []string
	// Device classification
	Class    string
	SubClass string
	// Network-specific information
	LogicalName string
	BusInfo     string
	// Configuration details
	Configuration map[string]interface{}
	// Detailed PCI capabilities
	DetailedCapabilities map[string]DetailedCapability
	// Ethtool information
	EthtoolInfo *EthtoolInfo
	// NUMA topology information
	NUMANode     int
	NUMADistance map[int]int // Distance to other NUMA nodes
}

// GetDetailedCapabilities returns formatted detailed capability information
func (d *Device) GetDetailedCapabilities() string {
	if len(d.DetailedCapabilities) == 0 {
		return "No detailed capabilities available"
	}

	var result []string
	for _, cap := range d.DetailedCapabilities {
		result = append(result, fmt.Sprintf("Capabilities: [%s] %s", cap.ID, cap.Description))
	}
	return strings.Join(result, "\n")
}

// GetCapabilityInfo returns specific capability information
func (d *Device) GetCapabilityInfo(capabilityName string) *DetailedCapability {
	if cap, exists := d.DetailedCapabilities[capabilityName]; exists {
		return &cap
	}
	return nil
}

// GetNUMANode returns the NUMA node this device belongs to
func (d *Device) GetNUMANode() int {
	return d.NUMANode
}

// GetNUMADistance returns the distance to a specific NUMA node
func (d *Device) GetNUMADistance(node int) (int, bool) {
	if d.NUMADistance == nil {
		return -1, false
	}
	distance, exists := d.NUMADistance[node]
	return distance, exists
}

// IsNUMALocal returns true if the device is on the local NUMA node (distance 10)
func (d *Device) IsNUMALocal() bool {
	if d.NUMADistance == nil {
		return false
	}
	distance, exists := d.NUMADistance[d.NUMANode]
	return exists && distance == 10
}

// GetNUMATopologyInfo returns formatted NUMA topology information
func (d *Device) GetNUMATopologyInfo() string {
	if d.NUMANode == -1 {
		return "No NUMA affinity"
	}

	var info []string
	info = append(info, fmt.Sprintf("NUMA Node: %d", d.NUMANode))

	if d.NUMADistance != nil && len(d.NUMADistance) > 0 {
		var distances []string
		for node, distance := range d.NUMADistance {
			distances = append(distances, fmt.Sprintf("Node %d: %d", node, distance))
		}
		info = append(info, fmt.Sprintf("NUMA Distances: %s", strings.Join(distances, ", ")))
	}

	return strings.Join(info, " | ")
}

// ParseLshwFromFile parses a lshw -class network -json output file
func ParseLshwFromFile(path string) ([]Device, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw []map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}

	return parseLshwData(raw)
}

// ParseLshwDynamic runs lshw -class network -json and parses the output
func ParseLshwDynamic() ([]Device, error) {
	cmd := exec.Command("lshw", "-class", "network", "-json")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var raw []map[string]any
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, err
	}

	return parseLshwData(raw)
}

// parseLshwData parses the raw lshw JSON data into Device structs
func parseLshwData(raw []map[string]any) ([]Device, error) {
	var devices []Device
	for _, item := range raw {
		businfo, _ := item["businfo"].(string)
		// businfo is like pci@0000:09:00.0
		pciAddr := strings.TrimPrefix(businfo, "pci@")
		conf, _ := item["configuration"].(map[string]any)
		driver, _ := conf["driver"].(string)
		logicalName, _ := item["logicalname"].(string)
		vendor, _ := item["vendor"].(string)
		product, _ := item["product"].(string)
		description, _ := item["description"].(string)
		serial, _ := item["serial"].(string)
		size, _ := item["size"].(string)
		capacity, _ := item["capacity"].(string)
		clock, _ := item["clock"].(string)
		width, _ := item["width"].(string)
		class, _ := item["class"].(string)
		subclass, _ := item["subclass"].(string)

		// Parse capabilities if available
		var capabilities []string
		if caps, ok := item["capabilities"].(map[string]any); ok {
			for capName := range caps {
				capabilities = append(capabilities, capName)
			}
		}

		devices = append(devices, Device{
			PCIAddress:    pciAddr,
			Name:          logicalName,
			Driver:        driver,
			Vendor:        vendor,
			Product:       product,
			Description:   description,
			Serial:        serial,
			Size:          size,
			Capacity:      capacity,
			Clock:         clock,
			Width:         width,
			Capabilities:  capabilities,
			Class:         class,
			SubClass:      subclass,
			LogicalName:   logicalName,
			BusInfo:       businfo,
			Configuration: conf,
		})
	}
	return devices, nil
}

// AttachPciInfo enriches devices with data from enhanced PCI parsing
func AttachPciInfo(devices []Device) ([]Device, error) {
	pciDevs, err := parsePciDevices()
	if err != nil {
		return devices, err
	}
	// index by bus (pci address) to find kernel driver etc
	info := make(map[string]SysfsPciDevice)
	for _, d := range pciDevs {
		info[d.Bus] = d
	}
	for i, dev := range devices {
		if p, ok := info[dev.PCIAddress]; ok {
			if dev.Driver == "" {
				dev.Driver = p.KernelDriver
			}
			if dev.Vendor == "" {
				dev.Vendor = p.VendorName
			}
			if dev.Product == "" {
				dev.Product = p.DeviceName
			}
			// Add enhanced SR-IOV information
			dev.SRIOVCapable = p.SRIOVCapable
			dev.SRIOVInfo = p.SRIOVInfo
			// Add detailed capabilities
			dev.DetailedCapabilities = p.DetailedCapabilities
			// Add NUMA topology information
			dev.NUMANode = p.NUMANode
			dev.NUMADistance = p.NUMADistance
			devices[i] = dev
		}
	}
	return devices, nil
}

// AttachEthtoolInfo enriches device information with ethtool details
func AttachEthtoolInfo(devices []Device) ([]Device, error) {
	Debug("Processing %d devices for ethtool information", len(devices))

	for i := range devices {
		// Only get ethtool info for devices with a logical name (network interfaces)
		if devices[i].LogicalName != "" {
			// Skip USB devices and other non-Ethernet interfaces
			if isEthernetInterface(devices[i].LogicalName, devices[i].Class, devices[i].SubClass) {
				Debug("Processing device %d: LogicalName=%s, Name=%s", i, devices[i].LogicalName, devices[i].Name)
				ethtoolInfo, err := GetEthtoolInfo(devices[i].LogicalName)
				if err != nil {
					// Log error but continue with other devices
					WithField("device", devices[i].LogicalName).WithError(err).Warn("Failed to get ethtool info")
					continue
				}
				devices[i].EthtoolInfo = ethtoolInfo
				Debug("Successfully added ethtool info for %s", devices[i].LogicalName)
			} else {
				Debug("Skipping non-Ethernet device %d: LogicalName=%s, Class=%s, SubClass=%s", i, devices[i].LogicalName, devices[i].Class, devices[i].SubClass)
			}
		} else {
			Debug("Skipping device %d: no logical name", i)
		}
	}

	return devices, nil
}

// isEthernetInterface checks if a device is an Ethernet interface that supports ethtool
func isEthernetInterface(logicalName, class, subClass string) bool {
	// Skip USB devices
	if strings.HasPrefix(logicalName, "usb") {
		return false
	}

	// Skip loopback interfaces
	if strings.HasPrefix(logicalName, "lo") {
		return false
	}

	// Skip wireless interfaces (they may not support all ethtool commands)
	if strings.HasPrefix(logicalName, "wlan") || strings.HasPrefix(logicalName, "wifi") {
		return false
	}

	// Skip virtual interfaces that don't support ethtool
	if strings.HasPrefix(logicalName, "veth") || strings.HasPrefix(logicalName, "docker") {
		return false
	}

	// Check if it's a network interface class
	if class == "network" {
		// For network devices, check if it's Ethernet (not wireless, USB, etc.)
		if subClass == "ethernet" || subClass == "" {
			return true
		}
	}

	// If we can't determine from class/subclass, check if the interface name looks like Ethernet
	// Common Ethernet interface patterns
	ethernetPatterns := []string{
		"en", "eth", "ens", "eno", "enp", "ens", "ens60", "ens61", "ens70", "ens80",
	}

	for _, pattern := range ethernetPatterns {
		if strings.HasPrefix(logicalName, pattern) {
			return true
		}
	}

	return false
}
