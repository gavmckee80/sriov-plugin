package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestNewSRIOVManager tests SR-IOV manager creation
func TestNewSRIOVManager(t *testing.T) {
	config := &SRIOVConfig{
		Version:     "1.0",
		Description: "Test Configuration",
		LogLevel:    "info",
	}

	manager := NewSRIOVManager(config)
	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
	if manager.config != config {
		t.Error("Expected config to be set")
	}
	// Logger is now handled by the global logging system, no need to check manager.logger
}

// TestExtractDeviceIDs tests device ID extraction
func TestExtractDeviceIDs(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	testCases := []struct {
		name           string
		device         Device
		expectedVendor string
		expectedDevice string
	}{
		{
			name: "Mellanox device",
			device: Device{
				Vendor:  "Mellanox Technologies",
				Product: "MT2910 Family [ConnectX-7]",
			},
			expectedVendor: "15b3",
			expectedDevice: "101e",
		},
		{
			name: "Pensando device",
			device: Device{
				Vendor:  "Pensando Systems",
				Product: "DSC Ethernet Controller",
			},
			expectedVendor: "1dd8",
			expectedDevice: "1003",
		},
		{
			name: "Intel device",
			device: Device{
				Vendor:  "Intel Corporation",
				Product: "I350 Gigabit Network Connection",
			},
			expectedVendor: "8086",
			expectedDevice: "1520",
		},
		{
			name: "Unknown device",
			device: Device{
				Vendor:  "Unknown Vendor",
				Product: "Unknown Product",
			},
			expectedVendor: "",
			expectedDevice: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vendor, device := manager.extractDeviceIDs(tc.device)
			if vendor != tc.expectedVendor {
				t.Errorf("Expected vendor %s, got %s", tc.expectedVendor, vendor)
			}
			if device != tc.expectedDevice {
				t.Errorf("Expected device %s, got %s", tc.expectedDevice, device)
			}
		})
	}
}

// TestConfigureDevice tests device configuration
func TestConfigureDevice(t *testing.T) {
	config := &SRIOVConfig{
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3",
				DeviceID:     "101e",
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7",
			},
			{
				VendorID:    "1dd8",
				DeviceID:    "1003",
				NumVFs:      1,
				Mode:        ModeSingleHome,
				Description: "Pensando DSC",
			},
		},
	}

	manager := NewSRIOVManager(config)

	testCases := []struct {
		name           string
		device         Device
		expectError    bool
		expectedPolicy *DevicePolicy
	}{
		{
			name: "Mellanox device with policy",
			device: Device{
				Name:         "ens60f0np0",
				PCIAddress:   "0000:31:00.0",
				Vendor:       "Mellanox Technologies",
				Product:      "MT2910 Family [ConnectX-7]",
				SRIOVCapable: true,
				SRIOVInfo: &SRIOVInfo{
					TotalVFs: 16,
				},
			},
			expectError:    false,
			expectedPolicy: &config.DevicePolicies[0],
		},
		{
			name: "Pensando device with policy",
			device: Device{
				Name:         "enp9s0np0",
				PCIAddress:   "0000:09:00.0",
				Vendor:       "Pensando Systems",
				Product:      "DSC Ethernet Controller",
				SRIOVCapable: true,
				SRIOVInfo: &SRIOVInfo{
					TotalVFs: 1,
				},
			},
			expectError:    false,
			expectedPolicy: &config.DevicePolicies[1],
		},
		{
			name: "Device without policy",
			device: Device{
				Name:         "ens61f0",
				PCIAddress:   "0000:32:00.0",
				Vendor:       "Intel Corporation",
				Product:      "I350 Gigabit Network Connection",
				SRIOVCapable: true,
			},
			expectError:    false,
			expectedPolicy: nil,
		},
		{
			name: "Device without vendor info",
			device: Device{
				Name:         "unknown",
				PCIAddress:   "0000:99:00.0",
				SRIOVCapable: true,
			},
			expectError:    true,
			expectedPolicy: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.configureDevice(tc.device)
			if tc.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestEnableSRIOV tests SR-IOV enabling
func TestEnableSRIOV(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	device := Device{
		Name:       "test-device",
		PCIAddress: "0000:01:00.0",
	}

	// Test with mock sysfs path
	tempDir := t.TempDir()
	sysfsPath := filepath.Join(tempDir, "0000:01:00.0")
	sriovPath := filepath.Join(sysfsPath, "sriov_numvfs")

	// Create directory structure
	if err := os.MkdirAll(sysfsPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test enabling SR-IOV
	err := manager.enableSRIOV(device, 4)
	if err == nil {
		t.Error("Expected error when sysfs is not writable")
	}

	// Test with existing SR-IOV configuration
	if err := os.WriteFile(sriovPath, []byte("4"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	err = manager.enableSRIOV(device, 4)
	if err != nil {
		t.Errorf("Expected no error when SR-IOV already enabled: %v", err)
	}
}

// TestEnableSwitchMode tests switchdev mode enabling
func TestEnableSwitchMode(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	testCases := []struct {
		name        string
		device      Device
		expectError bool
	}{
		{
			name: "Mellanox device",
			device: Device{
				Name:   "ens60f0np0",
				Vendor: "Mellanox Technologies",
			},
			expectError: true, // mlxconfig not available in test environment
		},
		{
			name: "Non-Mellanox device",
			device: Device{
				Name:   "ens61f0",
				Vendor: "Intel Corporation",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := manager.enableSwitchMode(tc.device)
			if tc.expectError && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestConfigureVFLagMode tests VF-LAG mode configuration
func TestConfigureVFLagMode(t *testing.T) {
	config := &SRIOVConfig{
		BondConfigs: []BondConfig{
			{
				BondName:        "bond0",
				SlaveInterfaces: []string{"ens60f0np0", "ens60f1np1"},
				Mode:            "active-backup",
				MIIMonitor:      100,
			},
		},
	}

	manager := NewSRIOVManager(config)

	device := Device{
		Name: "ens60f0np0",
	}

	policy := &DevicePolicy{
		Mode: ModeVFLag,
	}

	err := manager.configureVFLagMode(device, policy)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestCreateBondInterface tests bond interface creation
func TestCreateBondInterface(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	bond := &BondConfig{
		BondName:        "test-bond",
		SlaveInterfaces: []string{"ens60f0np0", "ens60f1np1"},
		Mode:            "active-backup",
		MIIMonitor:      100,
	}

	err := manager.createBondInterface(bond)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestValidateConfiguration tests configuration validation
func TestValidateConfiguration(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	err := manager.ValidateConfiguration()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
}

// TestDiscoverDevices tests device discovery
func TestDiscoverDevices(t *testing.T) {
	manager := NewSRIOVManager(&SRIOVConfig{})

	devices, err := manager.DiscoverDevices()
	if err != nil {
		t.Fatalf("DiscoverDevices failed: %v", err)
	}

	// Should find at least some devices
	if len(devices) == 0 {
		t.Skip("No SR-IOV devices found")
	}

	// Verify device properties
	for _, device := range devices {
		if device.Name == "" {
			t.Error("Device missing name")
		}
		if !device.SRIOVCapable {
			t.Errorf("Device %s should be SR-IOV capable", device.Name)
		}
	}
}

// TestConfigureDevices tests device configuration
func TestConfigureDevices(t *testing.T) {
	config := &SRIOVConfig{
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3",
				DeviceID:     "101e",
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7",
			},
		},
	}

	manager := NewSRIOVManager(config)

	devices := []Device{
		{
			Name:         "ens60f0np0",
			PCIAddress:   "0000:31:00.0",
			Vendor:       "Mellanox Technologies",
			Product:      "MT2910 Family [ConnectX-7]",
			SRIOVCapable: true,
			SRIOVInfo: &SRIOVInfo{
				TotalVFs: 16,
			},
		},
		{
			Name:         "ens61f0",
			PCIAddress:   "0000:32:00.0",
			Vendor:       "Intel Corporation",
			Product:      "I350 Gigabit Network Connection",
			SRIOVCapable: true,
		},
	}

	err := manager.ConfigureDevices(devices)
	if err != nil {
		t.Errorf("ConfigureDevices failed: %v", err)
	}
}

// TestRun tests the complete SR-IOV manager run
func TestRun(t *testing.T) {
	config := &SRIOVConfig{
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3",
				DeviceID:     "101e",
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7",
			},
		},
	}

	manager := NewSRIOVManager(config)

	err := manager.Run()
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

// TestDryRunMode tests dry-run mode
func TestDryRunMode(t *testing.T) {
	config := &SRIOVConfig{
		DryRun: true,
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3",
				DeviceID:     "101e",
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7",
			},
		},
	}

	manager := NewSRIOVManager(config)

	devices := []Device{
		{
			Name:         "ens60f0np0",
			PCIAddress:   "0000:31:00.0",
			Vendor:       "Mellanox Technologies",
			Product:      "MT2910 Family [ConnectX-7]",
			SRIOVCapable: true,
			SRIOVInfo: &SRIOVInfo{
				TotalVFs: 16,
			},
		},
	}

	err := manager.ConfigureDevices(devices)
	if err != nil {
		t.Errorf("ConfigureDevices in dry-run mode failed: %v", err)
	}
}

// TestDevicePolicyMatching tests device policy matching
func TestDevicePolicyMatching(t *testing.T) {
	config := &SRIOVConfig{
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3",
				DeviceID:     "101e",
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7",
			},
			{
				VendorID:    "1dd8",
				DeviceID:    "1003",
				NumVFs:      1,
				Mode:        ModeSingleHome,
				Description: "Pensando DSC",
			},
		},
	}

	// manager := NewSRIOVManager(config) // Not needed for this test

	testCases := []struct {
		name           string
		vendorID       string
		deviceID       string
		expectedPolicy *DevicePolicy
	}{
		{
			name:           "Mellanox device",
			vendorID:       "15b3",
			deviceID:       "101e",
			expectedPolicy: &config.DevicePolicies[0],
		},
		{
			name:           "Pensando device",
			vendorID:       "1dd8",
			deviceID:       "1003",
			expectedPolicy: &config.DevicePolicies[1],
		},
		{
			name:           "Unknown device",
			vendorID:       "8086",
			deviceID:       "1520",
			expectedPolicy: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy := config.GetDevicePolicy(tc.vendorID, tc.deviceID)
			if tc.expectedPolicy == nil {
				if policy != nil {
					t.Errorf("Expected nil policy for %s/%s, got %v", tc.vendorID, tc.deviceID, policy)
				}
			} else {
				if policy == nil {
					t.Errorf("Expected policy for %s/%s, got nil", tc.vendorID, tc.deviceID)
				} else if policy.VendorID != tc.expectedPolicy.VendorID || policy.DeviceID != tc.expectedPolicy.DeviceID {
					t.Errorf("Policy mismatch for %s/%s: expected %s/%s, got %s/%s",
						tc.vendorID, tc.deviceID,
						tc.expectedPolicy.VendorID, tc.expectedPolicy.DeviceID,
						policy.VendorID, policy.DeviceID)
				}
			}
		})
	}
}
