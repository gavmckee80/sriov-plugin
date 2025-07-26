package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig tests configuration loading
func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test_config.json")

	// Create test configuration
	testConfig := `{
		"version": "1.0",
		"description": "Test Configuration",
		"device_policies": [
			{
				"vendor_id": "15b3",
				"device_id": "101e",
				"num_vfs": 4,
				"mode": "single-home",
				"enable_switch": true,
				"description": "Mellanox ConnectX-7"
			}
		],
		"bond_configs": [
			{
				"bond_name": "bond0",
				"slave_interfaces": ["ens60f0np0", "ens60f1np1"],
				"mode": "active-backup",
				"mii_monitor": 100
			}
		],
		"log_level": "debug"
	}`

	if err := os.WriteFile(configPath, []byte(testConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify configuration
	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}
	if config.Description != "Test Configuration" {
		t.Errorf("Expected description 'Test Configuration', got %s", config.Description)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", config.LogLevel)
	}
	if len(config.DevicePolicies) != 1 {
		t.Errorf("Expected 1 device policy, got %d", len(config.DevicePolicies))
	}
	if len(config.BondConfigs) != 1 {
		t.Errorf("Expected 1 bond config, got %d", len(config.BondConfigs))
	}

	// Test device policy
	policy := config.DevicePolicies[0]
	if policy.VendorID != "15b3" {
		t.Errorf("Expected vendor ID '15b3', got %s", policy.VendorID)
	}
	if policy.DeviceID != "101e" {
		t.Errorf("Expected device ID '101e', got %s", policy.DeviceID)
	}
	if policy.NumVFs != 4 {
		t.Errorf("Expected 4 VFs, got %d", policy.NumVFs)
	}
	if policy.Mode != ModeSingleHome {
		t.Errorf("Expected mode 'single-home', got %s", policy.Mode)
	}
	if !policy.EnableSwitch {
		t.Error("Expected enable_switch to be true")
	}

	// Test bond config
	bond := config.BondConfigs[0]
	if bond.BondName != "bond0" {
		t.Errorf("Expected bond name 'bond0', got %s", bond.BondName)
	}
	if len(bond.SlaveInterfaces) != 2 {
		t.Errorf("Expected 2 slave interfaces, got %d", len(bond.SlaveInterfaces))
	}
	if bond.Mode != "active-backup" {
		t.Errorf("Expected mode 'active-backup', got %s", bond.Mode)
	}
	if bond.MIIMonitor != 100 {
		t.Errorf("Expected MII monitor 100, got %d", bond.MIIMonitor)
	}
}

// TestLoadConfigWithDefaults tests configuration loading with defaults
func TestLoadConfigWithDefaults(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "minimal_config.json")

	// Create minimal configuration
	minimalConfig := `{
		"device_policies": [
			{
				"vendor_id": "15b3",
				"device_id": "101e",
				"num_vfs": 4
			}
		]
	}`

	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify defaults are set
	if config.Version != "1.0" {
		t.Errorf("Expected default version 1.0, got %s", config.Version)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", config.LogLevel)
	}
}

// TestLoadConfigInvalid tests invalid configuration handling
func TestLoadConfigInvalid(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "invalid_config.json")

	// Create invalid JSON
	invalidConfig := `{
		"version": "1.0",
		"device_policies": [
			{
				"vendor_id": "15b3",
				"device_id": "101e",
				"num_vfs": 4
			}
		]
		"invalid": syntax
	}`

	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}

// TestLoadConfigNotFound tests missing file handling
func TestLoadConfigNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

// TestSaveConfig tests configuration saving
func TestSaveConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "save_test_config.json")

	// Create test configuration
	config := &SRIOVConfig{
		Version:     "1.0",
		Description: "Test Save Configuration",
		LogLevel:    "debug",
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

	// Save configuration
	if err := SaveConfig(config, configPath); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify the saved configuration
	loadedConfig, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Version mismatch: expected %s, got %s", config.Version, loadedConfig.Version)
	}
	if loadedConfig.Description != config.Description {
		t.Errorf("Description mismatch: expected %s, got %s", config.Description, loadedConfig.Description)
	}
	if len(loadedConfig.DevicePolicies) != len(config.DevicePolicies) {
		t.Errorf("Device policies count mismatch: expected %d, got %d", len(config.DevicePolicies), len(loadedConfig.DevicePolicies))
	}
}

// TestGetDevicePolicy tests device policy lookup
func TestGetDevicePolicy(t *testing.T) {
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

	// Test exact match
	policy := config.GetDevicePolicy("15b3", "101e")
	if policy == nil {
		t.Error("Expected to find policy for Mellanox device")
	}
	if policy.VendorID != "15b3" {
		t.Errorf("Expected vendor ID '15b3', got %s", policy.VendorID)
	}

	// Test case-insensitive match
	policy = config.GetDevicePolicy("15B3", "101E")
	if policy == nil {
		t.Error("Expected to find policy with case-insensitive match")
	}

	// Test no match
	policy = config.GetDevicePolicy("8086", "1520")
	if policy != nil {
		t.Error("Expected no policy for Intel device")
	}
}

// TestValidateConfig tests configuration validation
func TestValidateConfig(t *testing.T) {
	testCases := []struct {
		name        string
		config      *SRIOVConfig
		expectError bool
	}{
		{
			name: "valid configuration",
			config: &SRIOVConfig{
				DevicePolicies: []DevicePolicy{
					{
						VendorID: "15b3",
						DeviceID: "101e",
						NumVFs:   4,
					},
				},
			},
			expectError: false,
		},
		{
			name: "missing vendor ID",
			config: &SRIOVConfig{
				DevicePolicies: []DevicePolicy{
					{
						DeviceID: "101e",
						NumVFs:   4,
					},
				},
			},
			expectError: true,
		},
		{
			name: "missing device ID",
			config: &SRIOVConfig{
				DevicePolicies: []DevicePolicy{
					{
						VendorID: "15b3",
						NumVFs:   4,
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid VF count",
			config: &SRIOVConfig{
				DevicePolicies: []DevicePolicy{
					{
						VendorID: "15b3",
						DeviceID: "101e",
						NumVFs:   0,
					},
				},
			},
			expectError: true,
		},
		{
			name: "invalid mode",
			config: &SRIOVConfig{
				DevicePolicies: []DevicePolicy{
					{
						VendorID: "15b3",
						DeviceID: "101e",
						NumVFs:   4,
						Mode:     "invalid-mode",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.ValidateConfig()
			if tc.expectError && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tc.expectError && err != nil {
				t.Errorf("Expected no validation error, got: %v", err)
			}
		})
	}
}

// TestCreateDefaultConfig tests default configuration creation
func TestCreateDefaultConfig(t *testing.T) {
	config := CreateDefaultConfig()

	// Verify basic structure
	if config.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", config.Version)
	}
	if config.Description != "SR-IOV Manager Configuration" {
		t.Errorf("Expected description 'SR-IOV Manager Configuration', got %s", config.Description)
	}
	if config.LogLevel != "info" {
		t.Errorf("Expected log level 'info', got %s", config.LogLevel)
	}

	// Verify device policies
	if len(config.DevicePolicies) == 0 {
		t.Error("Expected at least one device policy")
	}

	// Verify bond configs
	if len(config.BondConfigs) == 0 {
		t.Error("Expected at least one bond config")
	}

	// Verify the configuration is valid
	if err := config.ValidateConfig(); err != nil {
		t.Errorf("Default configuration validation failed: %v", err)
	}
}

// TestSRIOVModeConstants tests SR-IOV mode constants
func TestSRIOVModeConstants(t *testing.T) {
	if ModeSingleHome != "single-home" {
		t.Errorf("Expected ModeSingleHome to be 'single-home', got %s", ModeSingleHome)
	}
	if ModeVFLag != "vf-lag" {
		t.Errorf("Expected ModeVFLag to be 'vf-lag', got %s", ModeVFLag)
	}
}
