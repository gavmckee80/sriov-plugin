package pkg

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestMockPciDevices tests the mock PCI device parsing
func TestMockPciDevices(t *testing.T) {
	devices, err := MockParseSysfsPciDevices()
	if err != nil {
		t.Fatalf("Failed to parse mock PCI devices: %v", err)
	}

	// Updated expectation: we now have 2 devices in the mock data
	if len(devices) != 2 {
		t.Errorf("expected 2 mock devices, got %d", len(devices))
	}

	// Check first device (Mellanox)
	device1 := devices[0]
	if device1.Bus != "0000:01:00.0" {
		t.Errorf("expected bus 0000:01:00.0, got %s", device1.Bus)
	}
	if device1.VendorName != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", device1.VendorName)
	}
	if !device1.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device1.NUMANode != 0 {
		t.Errorf("expected NUMA node 0, got %d", device1.NUMANode)
	}

	// Check second device (Pensando)
	device2 := devices[1]
	if device2.Bus != "0000:02:00.0" {
		t.Errorf("expected bus 0000:02:00.0, got %s", device2.Bus)
	}
	if device2.VendorName != "Pensando Systems" {
		t.Errorf("expected vendor Pensando Systems, got %s", device2.VendorName)
	}
	if !device2.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device2.NUMANode != 1 {
		t.Errorf("expected NUMA node 1, got %d", device2.NUMANode)
	}
}

// TestMockPciDevicesWithFilter tests filtering by driver
func TestMockPciDevicesWithFilter(t *testing.T) {
	devices, err := MockParseSysfsPciDevices()
	if err != nil {
		t.Fatalf("Failed to parse mock PCI devices: %v", err)
	}

	// Filter for Mellanox devices
	var mellanoxDevices []SysfsPciDevice
	for _, device := range devices {
		if strings.Contains(strings.ToLower(device.VendorName), "mellanox") {
			mellanoxDevices = append(mellanoxDevices, device)
		}
	}

	// Updated expectation: we now have 1 Mellanox device in the mock data
	if len(mellanoxDevices) != 1 {
		t.Errorf("expected 1 Mellanox device, got %d", len(mellanoxDevices))
	}

	if mellanoxDevices[0].VendorName != "Mellanox Technologies" {
		t.Errorf("expected Mellanox Technologies, got %s", mellanoxDevices[0].VendorName)
	}
}

// TestMockPciDevicesByVendor tests filtering by vendor
func TestMockPciDevicesByVendor(t *testing.T) {
	devices, err := MockParseSysfsPciDevices()
	if err != nil {
		t.Fatalf("Failed to parse mock PCI devices: %v", err)
	}

	// Filter for Pensando devices
	var pensandoDevices []SysfsPciDevice
	for _, device := range devices {
		if strings.Contains(strings.ToLower(device.VendorName), "pensando") {
			pensandoDevices = append(pensandoDevices, device)
		}
	}

	// Updated expectation: we now have 1 Pensando device in the mock data
	if len(pensandoDevices) != 1 {
		t.Errorf("expected 1 Pensando device, got %d", len(pensandoDevices))
	}

	if pensandoDevices[0].VendorName != "Pensando Systems" {
		t.Errorf("expected Pensando Systems, got %s", pensandoDevices[0].VendorName)
	}
}

// TestAttachPciInfoWithMock tests device enrichment with mock PCI data
func TestAttachPciInfoWithMock(t *testing.T) {
	// Create mock devices
	devices := []Device{
		{
			PCIAddress: "0000:01:00.0",
			Name:       "eth0",
			Vendor:     "",
			Product:    "",
		},
		{
			PCIAddress: "0000:02:00.0",
			Name:       "eth1",
			Vendor:     "",
			Product:    "",
		},
	}

	// Attach PCI info
	enrichedDevices, err := AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("AttachPciInfo failed: %v", err)
	}

	if len(enrichedDevices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(enrichedDevices))
	}

	// Check first device (Mellanox)
	device1 := enrichedDevices[0]
	if device1.Driver != "mlx5_core" {
		t.Errorf("expected driver mlx5_core, got %s", device1.Driver)
	}
	if device1.Vendor != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", device1.Vendor)
	}
	if !device1.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device1.NUMANode != 0 {
		t.Errorf("expected NUMA node 0, got %d", device1.NUMANode)
	}

	// Check second device (Pensando)
	device2 := enrichedDevices[1]
	if device2.Driver != "pensando_dsc" {
		t.Errorf("expected driver pensando_dsc, got %s", device2.Driver)
	}
	if device2.Vendor != "Pensando Systems" {
		t.Errorf("expected vendor Pensando Systems, got %s", device2.Vendor)
	}
	if !device2.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device2.NUMANode != 1 {
		t.Errorf("expected NUMA node 1, got %d", device2.NUMANode)
	}
}

// TestParseLshwWithMockFile tests parsing lshw JSON with mock file
func TestParseLshwWithMockFile(t *testing.T) {
	// Create mock lshw JSON data
	mockData := []map[string]interface{}{
		{
			"businfo":     "pci@0000:01:00.0",
			"logicalname": "eth0",
			"vendor":      "Mellanox Technologies",
			"product":     "MT2910 Family [ConnectX-7]",
			"description": "Ethernet interface",
			"serial":      "00:1b:21:0a:8b:2a",
			"size":        "10Gbit/s",
			"capacity":    "10Gbit/s",
			"clock":       "33MHz",
			"width":       "8 bits",
			"class":       "network",
			"subclass":    "ethernet",
			"configuration": map[string]interface{}{
				"driver": "mlx5_core",
				"speed":  "10Gbit/s",
			},
			"capabilities": map[string]interface{}{
				"pm":         "Power Management",
				"msi":        "Message Signalled Interrupts",
				"pciexpress": "PCI Express",
				"msix":       "MSI-X",
				"bus_master": "bus mastering",
				"cap_list":   "PCI capabilities listing",
			},
		},
		{
			"businfo":     "pci@0000:02:00.0",
			"logicalname": "eth1",
			"vendor":      "Intel Corporation",
			"product":     "I350 Gigabit Network Connection",
			"description": "Ethernet interface",
			"serial":      "00:1b:21:0a:8b:2b",
			"size":        "1Gbit/s",
			"capacity":    "1Gbit/s",
			"clock":       "33MHz",
			"width":       "8 bits",
			"class":       "network",
			"subclass":    "ethernet",
			"configuration": map[string]interface{}{
				"driver": "igb",
				"speed":  "1Gbit/s",
			},
			"capabilities": map[string]interface{}{
				"pm":         "Power Management",
				"msi":        "Message Signalled Interrupts",
				"pciexpress": "PCI Express",
				"msix":       "MSI-X",
				"bus_master": "bus mastering",
				"cap_list":   "PCI capabilities listing",
			},
		},
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "mock-lshw-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write mock data to file
	if err := json.NewEncoder(tmpFile).Encode(mockData); err != nil {
		t.Fatalf("Failed to write mock data: %v", err)
	}

	// Parse the mock file
	devices, err := ParseLshwFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseLshwFromFile returned error: %v", err)
	}

	// Verify results
	if len(devices) != 2 {
		t.Fatalf("Expected 2 devices, got %d", len(devices))
	}

	// Check first device (Mellanox)
	device1 := devices[0]
	if device1.PCIAddress != "0000:01:00.0" {
		t.Errorf("Expected PCI address 0000:01:00.0, got %s", device1.PCIAddress)
	}
	if device1.Name != "eth0" {
		t.Errorf("Expected name eth0, got %s", device1.Name)
	}
	if device1.Vendor != "Mellanox Technologies" {
		t.Errorf("Expected vendor Mellanox Technologies, got %s", device1.Vendor)
	}
	if device1.Product != "MT2910 Family [ConnectX-7]" {
		t.Errorf("Expected product MT2910 Family [ConnectX-7], got %s", device1.Product)
	}
	if device1.Driver != "mlx5_core" {
		t.Errorf("Expected driver mlx5_core, got %s", device1.Driver)
	}
	if device1.Description != "Ethernet interface" {
		t.Errorf("Expected description Ethernet interface, got %s", device1.Description)
	}
	if device1.Serial != "00:1b:21:0a:8b:2a" {
		t.Errorf("Expected serial 00:1b:21:0a:8b:2a, got %s", device1.Serial)
	}
	if device1.Size != "10Gbit/s" {
		t.Errorf("Expected size 10Gbit/s, got %s", device1.Size)
	}
	if device1.Capacity != "10Gbit/s" {
		t.Errorf("Expected capacity 10Gbit/s, got %s", device1.Capacity)
	}
	if device1.Clock != "33MHz" {
		t.Errorf("Expected clock 33MHz, got %s", device1.Clock)
	}
	if device1.Width != "8 bits" {
		t.Errorf("Expected width 8 bits, got %s", device1.Width)
	}
	if device1.Class != "network" {
		t.Errorf("Expected class network, got %s", device1.Class)
	}
	if device1.SubClass != "ethernet" {
		t.Errorf("Expected subclass ethernet, got %s", device1.SubClass)
	}
	if len(device1.Capabilities) != 6 {
		t.Errorf("Expected 6 capabilities, got %d", len(device1.Capabilities))
	}

	// Check second device (Intel)
	device2 := devices[1]
	if device2.PCIAddress != "0000:02:00.0" {
		t.Errorf("Expected PCI address 0000:02:00.0, got %s", device2.PCIAddress)
	}
	if device2.Name != "eth1" {
		t.Errorf("Expected name eth1, got %s", device2.Name)
	}
	if device2.Vendor != "Intel Corporation" {
		t.Errorf("Expected vendor Intel Corporation, got %s", device2.Vendor)
	}
	if device2.Product != "I350 Gigabit Network Connection" {
		t.Errorf("Expected product I350 Gigabit Network Connection, got %s", device2.Product)
	}
	if device2.Driver != "igb" {
		t.Errorf("Expected driver igb, got %s", device2.Driver)
	}
}

// TestEndToEndMock tests the complete end-to-end flow with mock data
func TestEndToEndMock(t *testing.T) {
	// Create a mock lshw output file
	mockLshwData := `{
		"id": "network",
		"children": [
			{
				"id": "network:0",
				"class": "network",
				"logicalname": "eth0",
				"businfo": "pci@0000:01:00.0",
				"vendor": "Mellanox Technologies",
				"product": "MT2910 Family [ConnectX-7]",
				"driver": "mlx5_core"
			},
			{
				"id": "network:1",
				"class": "network",
				"logicalname": "eth1",
				"businfo": "pci@0000:02:00.0",
				"vendor": "Pensando Systems",
				"product": "DSC Ethernet Controller",
				"driver": "pensando_dsc"
			}
		]
	}`

	// Write mock data to temporary file
	tmpFile, err := os.CreateTemp("", "lshw_mock_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(mockLshwData); err != nil {
		t.Fatalf("Failed to write mock data: %v", err)
	}
	tmpFile.Close()

	// Parse the mock lshw data
	devices, err := ParseLshwFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseLshwFromFile failed: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}

	// Enrich with PCI information
	enrichedDevices, err := AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("AttachPciInfo failed: %v", err)
	}

	// Verify first device (Mellanox)
	device1 := enrichedDevices[0]
	if device1.PCIAddress != "0000:01:00.0" {
		t.Errorf("expected PCI address 0000:01:00.0, got %s", device1.PCIAddress)
	}
	if device1.Name != "eth0" {
		t.Errorf("expected name eth0, got %s", device1.Name)
	}
	if device1.Driver != "mlx5_core" {
		t.Errorf("expected driver mlx5_core, got %s", device1.Driver)
	}
	if device1.Vendor != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", device1.Vendor)
	}
	if !device1.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device1.SRIOVInfo == nil {
		t.Errorf("expected SR-IOV info to be present")
	}
	if device1.NUMANode != 0 {
		t.Errorf("expected NUMA node 0, got %d", device1.NUMANode)
	}

	// Verify second device (Pensando)
	device2 := enrichedDevices[1]
	if device2.PCIAddress != "0000:02:00.0" {
		t.Errorf("expected PCI address 0000:02:00.0, got %s", device2.PCIAddress)
	}
	if device2.Name != "eth1" {
		t.Errorf("expected name eth1, got %s", device2.Name)
	}
	if device2.Driver != "pensando_dsc" {
		t.Errorf("expected driver pensando_dsc, got %s", device2.Driver)
	}
	if device2.Vendor != "Pensando Systems" {
		t.Errorf("expected vendor Pensando Systems, got %s", device2.Vendor)
	}
	if !device2.SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if device2.SRIOVInfo == nil {
		t.Errorf("expected SR-IOV info to be present")
	}
	if device2.NUMANode != 1 {
		t.Errorf("expected NUMA node 1, got %d", device2.NUMANode)
	}

	// Test NUMA topology methods
	if device1.GetNUMANode() != 0 {
		t.Errorf("GetNUMANode() expected 0, got %d", device1.GetNUMANode())
	}
	if device2.GetNUMANode() != 1 {
		t.Errorf("GetNUMANode() expected 1, got %d", device2.GetNUMANode())
	}

	// Test NUMA distance
	distance1, exists1 := device1.GetNUMADistance(1)
	if !exists1 {
		t.Errorf("Expected NUMA distance to node 1 to exist for device 1")
	}
	if distance1 != 20 {
		t.Errorf("Expected NUMA distance 20 from node 0 to node 1, got %d", distance1)
	}

	distance2, exists2 := device2.GetNUMADistance(0)
	if !exists2 {
		t.Errorf("Expected NUMA distance to node 0 to exist for device 2")
	}
	if distance2 != 20 {
		t.Errorf("Expected NUMA distance 20 from node 1 to node 0, got %d", distance2)
	}

	// Test NUMA topology info
	topologyInfo1 := device1.GetNUMATopologyInfo()
	if !strings.Contains(topologyInfo1, "NUMA Node: 0") {
		t.Errorf("Expected topology info to contain 'NUMA Node: 0', got: %s", topologyInfo1)
	}

	topologyInfo2 := device2.GetNUMATopologyInfo()
	if !strings.Contains(topologyInfo2, "NUMA Node: 1") {
		t.Errorf("Expected topology info to contain 'NUMA Node: 1', got: %s", topologyInfo2)
	}
}
