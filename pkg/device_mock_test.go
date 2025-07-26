package pkg

import (
	"encoding/json"
	"os"
	"testing"
)

// TestMockPciDevices tests the mock PCI device parsing
func TestMockPciDevices(t *testing.T) {
	devices, err := MockParseSysfsPciDevices()
	if err != nil {
		t.Fatalf("MockParseSysfsPciDevices returned error: %v", err)
	}

	if len(devices) != 6 {
		t.Fatalf("expected 6 mock devices, got %d", len(devices))
	}

	// Test first device (Mellanox)
	expected := SysfsPciDevice{
		Bus:          "0000:01:00.0",
		KernelDriver: "mlx5_core",
		VendorName:   "Mellanox Technologies",
		DeviceName:   "MT2910 Family [ConnectX-7]",
		VendorID:     "15b3",
		DeviceID:     "101e",
		SRIOVCapable: true,
	}

	if devices[0].Bus != expected.Bus || devices[0].KernelDriver != expected.KernelDriver {
		t.Errorf("first mock device mismatch\nexpected: %#v\nactual:   %#v", expected, devices[0])
	}
}

// TestMockPciDevicesWithFilter tests filtering by driver
func TestMockPciDevicesWithFilter(t *testing.T) {
	// Test filtering by Mellanox driver
	devices, err := MockParseSysfsPciDevicesByDriver("mlx5_core")
	if err != nil {
		t.Fatalf("MockParseSysfsPciDevicesByDriver returned error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 Mellanox devices, got %d", len(devices))
	}

	// Test filtering by Intel driver
	devices, err = MockParseSysfsPciDevicesByDriver("igb")
	if err != nil {
		t.Fatalf("MockParseSysfsPciDevicesByDriver returned error: %v", err)
	}

	if len(devices) != 1 {
		t.Fatalf("expected 1 Intel device, got %d", len(devices))
	}

	if devices[0].VendorName != "Intel Corporation" {
		t.Errorf("expected Intel vendor, got %s", devices[0].VendorName)
	}
}

// TestMockPciDevicesByVendor tests filtering by vendor
func TestMockPciDevicesByVendor(t *testing.T) {
	// Test filtering by Intel vendor
	devices, err := MockParseSysfsPciDevicesByVendor("Intel")
	if err != nil {
		t.Fatalf("MockParseSysfsPciDevicesByVendor returned error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 Intel devices, got %d", len(devices))
	}

	// Test filtering by Mellanox vendor
	devices, err = MockParseSysfsPciDevicesByVendor("Mellanox")
	if err != nil {
		t.Fatalf("MockParseSysfsPciDevicesByVendor returned error: %v", err)
	}

	if len(devices) != 2 {
		t.Fatalf("expected 2 Mellanox devices, got %d", len(devices))
	}
}

// TestAttachPciInfoWithMock tests device enrichment with mock PCI data
func TestAttachPciInfoWithMock(t *testing.T) {
	// Create test devices that match our mock PCI data
	devices := []Device{
		{PCIAddress: "0000:01:00.0", Name: "test1"},
		{PCIAddress: "0000:02:00.0", Name: "test2"},
		{PCIAddress: "0000:99:00.0", Name: "test3"}, // Non-existent PCI address
	}

	// Override the parsePciDevices function with our sysfs mock
	old := parsePciDevices
	defer func() { parsePciDevices = old }()
	parsePciDevices = MockParseSysfsPciDevices

	enriched, err := AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("AttachPciInfo returned error: %v", err)
	}

	if len(enriched) != 3 {
		t.Fatalf("expected 3 enriched devices, got %d", len(enriched))
	}

	// Test first device (should be enriched with Mellanox data)
	if enriched[0].Driver != "mlx5_core" {
		t.Errorf("expected driver mlx5_core, got %s", enriched[0].Driver)
	}
	if enriched[0].Vendor != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", enriched[0].Vendor)
	}
	if enriched[0].Product != "MT2910 Family [ConnectX-7]" {
		t.Errorf("expected product MT2910 Family [ConnectX-7], got %s", enriched[0].Product)
	}
	if !enriched[0].SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if enriched[0].SRIOVInfo == nil {
		t.Errorf("expected SR-IOV info to be present")
	}

	// Test second device (should be enriched with Pensando data)
	if enriched[1].Driver != "ionic" {
		t.Errorf("expected driver ionic, got %s", enriched[1].Driver)
	}
	if enriched[1].Vendor != "Pensando Systems" {
		t.Errorf("expected vendor Pensando Systems, got %s", enriched[1].Vendor)
	}
	if enriched[1].Product != "DSC Ethernet Controller" {
		t.Errorf("expected product DSC Ethernet Controller, got %s", enriched[1].Product)
	}
	if enriched[1].SRIOVCapable {
		t.Errorf("expected non-SR-IOV capable device")
	}

	// Test third device (should not be enriched - non-existent PCI address)
	if enriched[2].Driver != "" {
		t.Errorf("expected empty driver for non-existent device, got %s", enriched[2].Driver)
	}
	if enriched[2].Vendor != "" {
		t.Errorf("expected empty vendor for non-existent device, got %s", enriched[2].Vendor)
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
	// Create a temporary mock lshw JSON file
	mockLshwData := `[
		{
			"businfo": "pci@0000:01:00.0",
			"logicalname": "ens1f0",
			"configuration": {
				"driver": "mlx5_core"
			},
			"vendor": "Mellanox Technologies",
			"product": "MT2910 Family [ConnectX-7]"
		},
		{
			"businfo": "pci@0000:04:00.0",
			"logicalname": "ens4f0",
			"configuration": {
				"driver": "i40e"
			},
			"vendor": "Intel Corporation",
			"product": "Ethernet Controller X710 for 10GbE SFP+"
		}
	]`

	tmpFile, err := os.CreateTemp("", "lshw-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(mockLshwData); err != nil {
		t.Fatalf("failed to write mock data: %v", err)
	}
	tmpFile.Close()

	// Override the parsePciDevices function with our sysfs mock
	old := parsePciDevices
	defer func() { parsePciDevices = old }()
	parsePciDevices = MockParseSysfsPciDevices

	// Parse lshw and enrich with PCI data
	devices, err := ParseLshwFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseLshwFromFile returned error: %v", err)
	}

	enriched, err := AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("AttachPciInfo returned error: %v", err)
	}

	if len(enriched) != 2 {
		t.Fatalf("expected 2 enriched devices, got %d", len(enriched))
	}

	// Test first device (Mellanox with SR-IOV)
	if enriched[0].PCIAddress != "0000:01:00.0" {
		t.Errorf("expected PCI address 0000:01:00.0, got %s", enriched[0].PCIAddress)
	}
	if enriched[0].Name != "ens1f0" {
		t.Errorf("expected name ens1f0, got %s", enriched[0].Name)
	}
	if enriched[0].Driver != "mlx5_core" {
		t.Errorf("expected driver mlx5_core, got %s", enriched[0].Driver)
	}
	if enriched[0].Vendor != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", enriched[0].Vendor)
	}
	if enriched[0].Product != "MT2910 Family [ConnectX-7]" {
		t.Errorf("expected product MT2910 Family [ConnectX-7], got %s", enriched[0].Product)
	}
	if !enriched[0].SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if enriched[0].SRIOVInfo == nil {
		t.Errorf("expected SR-IOV info to be present")
	}
	if enriched[0].SRIOVInfo.TotalVFs != 16 {
		t.Errorf("expected 16 total VFs, got %d", enriched[0].SRIOVInfo.TotalVFs)
	}

	// Test second device (Intel with SR-IOV)
	if enriched[1].PCIAddress != "0000:04:00.0" {
		t.Errorf("expected PCI address 0000:04:00.0, got %s", enriched[1].PCIAddress)
	}
	if enriched[1].Name != "ens4f0" {
		t.Errorf("expected name ens4f0, got %s", enriched[1].Name)
	}
	if enriched[1].Driver != "i40e" {
		t.Errorf("expected driver i40e, got %s", enriched[1].Driver)
	}
	if enriched[1].Vendor != "Intel Corporation" {
		t.Errorf("expected vendor Intel Corporation, got %s", enriched[1].Vendor)
	}
	if enriched[1].Product != "Ethernet Controller X710 for 10GbE SFP+" {
		t.Errorf("expected product Ethernet Controller X710 for 10GbE SFP+, got %s", enriched[1].Product)
	}
	if !enriched[1].SRIOVCapable {
		t.Errorf("expected SR-IOV capable device")
	}
	if enriched[1].SRIOVInfo == nil {
		t.Errorf("expected SR-IOV info to be present")
	}
	if enriched[1].SRIOVInfo.TotalVFs != 32 {
		t.Errorf("expected 32 total VFs, got %d", enriched[1].SRIOVInfo.TotalVFs)
	}
}
