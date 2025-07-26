package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseSysfsPciDevices tests the sysfs PCI device parsing
func TestParseSysfsPciDevices(t *testing.T) {
	// Skip if not running as root or sysfs not available
	if _, err := os.Stat("/sys/bus/pci/devices"); os.IsNotExist(err) {
		t.Skip("sysfs not available, skipping test")
	}

	devices, err := ParseSysfsPciDevices()
	if err != nil {
		t.Fatalf("ParseSysfsPciDevices returned error: %v", err)
	}

	// Should find at least some devices
	if len(devices) == 0 {
		t.Skip("No PCI devices found in sysfs")
	}

	// Test that we can parse basic device information
	for _, device := range devices {
		if device.Bus == "" {
			t.Errorf("device missing bus address")
		}
		if device.VendorID == "" {
			t.Errorf("device %s missing vendor ID", device.Bus)
		}
		if device.DeviceID == "" {
			t.Errorf("device %s missing device ID", device.Bus)
		}
		if device.Class == "" {
			t.Errorf("device %s missing class", device.Bus)
		}
	}

	t.Logf("Successfully parsed %d PCI devices from sysfs", len(devices))
}

// TestSysfsPciDeviceConversion tests conversion to EnhancedPciDevice
func TestSysfsPciDeviceConversion(t *testing.T) {
	sysfsDevice := SysfsPciDevice{
		Bus:          "0000:01:00.0",
		KernelDriver: "mlx5_core",
		VendorName:   "Mellanox Technologies",
		DeviceName:   "MT2910 Family [ConnectX-7]",
		VendorID:     "15b3",
		DeviceID:     "101e",
		Class:        "0200",
		SRIOVCapable: true,
		SRIOVInfo: &SRIOVInfo{
			TotalVFs:    16,
			NumberOfVFs: 4,
		},
		Capabilities: map[string]string{
			"MSI-X": "Available: 128",
		},
	}

	enhanced := sysfsDevice.ToEnhancedPciDevice()

	if enhanced.Bus != sysfsDevice.Bus {
		t.Errorf("bus mismatch: expected %s, got %s", sysfsDevice.Bus, enhanced.Bus)
	}
	if enhanced.KernelDriver != sysfsDevice.KernelDriver {
		t.Errorf("driver mismatch: expected %s, got %s", sysfsDevice.KernelDriver, enhanced.KernelDriver)
	}
	if enhanced.VendorName != sysfsDevice.VendorName {
		t.Errorf("vendor mismatch: expected %s, got %s", sysfsDevice.VendorName, enhanced.VendorName)
	}
	if enhanced.SRIOVCapable != sysfsDevice.SRIOVCapable {
		t.Errorf("SR-IOV capability mismatch: expected %t, got %t", sysfsDevice.SRIOVCapable, enhanced.SRIOVCapable)
	}
	if enhanced.SRIOVInfo.TotalVFs != sysfsDevice.SRIOVInfo.TotalVFs {
		t.Errorf("total VFs mismatch: expected %d, got %d", sysfsDevice.SRIOVInfo.TotalVFs, enhanced.SRIOVInfo.TotalVFs)
	}
}

// TestIsPciAddress tests PCI address validation
func TestIsPciAddress(t *testing.T) {
	testCases := []struct {
		name     string
		expected bool
	}{
		{"0000:01:00.0", true},
		{"0000:01:00.1", true},
		{"0000:0a:0b.c", true},
		{"0000:01:00", false},     // Missing function
		{"0000:01:00.0.1", false}, // Too many parts
		{"0000:01:00", false},     // Missing function
		{"0000:01:00.0", true},    // Valid
		{"invalid", false},        // Invalid format
		{"0000:01:00.g", false},   // Invalid hex
		{"0000:01:00.0", true},    // Valid
	}

	for _, tc := range testCases {
		result := isPciAddress(tc.name)
		if result != tc.expected {
			t.Errorf("isPciAddress(%s): expected %t, got %t", tc.name, tc.expected, result)
		}
	}
}

// TestSysfsDeviceParsing tests individual device parsing functions
func TestSysfsDeviceParsing(t *testing.T) {
	// Create a temporary sysfs-like structure for testing
	tmpDir, err := os.MkdirTemp("", "sysfs-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a mock PCI device directory
	deviceDir := filepath.Join(tmpDir, "0000:01:00.0")
	if err := os.Mkdir(deviceDir, 0755); err != nil {
		t.Fatalf("failed to create device dir: %v", err)
	}

	// Create mock device files
	mockFiles := map[string]string{
		"vendor":   "15b3",
		"device":   "101e",
		"class":    "0x020000",
		"revision": "00",
	}

	for filename, content := range mockFiles {
		filePath := filepath.Join(deviceDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create mock file %s: %v", filename, err)
		}
	}

	// Test device ID parsing
	device := &SysfsPciDevice{}
	if err := parseDeviceIds(deviceDir, device); err != nil {
		t.Fatalf("parseDeviceIds failed: %v", err)
	}

	if device.VendorID != "15b3" {
		t.Errorf("expected vendor ID 15b3, got %s", device.VendorID)
	}
	if device.DeviceID != "101e" {
		t.Errorf("expected device ID 101e, got %s", device.DeviceID)
	}
	if device.Revision != "00" {
		t.Errorf("expected revision 00, got %s", device.Revision)
	}

	// Test class parsing
	if err := parseDeviceClass(deviceDir, device); err != nil {
		t.Fatalf("parseDeviceClass failed: %v", err)
	}

	if device.Class != "0200" {
		t.Errorf("expected class 0200, got %s", device.Class)
	}
}
