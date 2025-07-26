package pkg

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SRIOVMode represents the SR-IOV configuration mode
type SRIOVMode string

const (
	ModeSingleHome SRIOVMode = "single-home"
	ModeVFLag      SRIOVMode = "vf-lag"
)

// DevicePolicy defines how to configure a specific device
type DevicePolicy struct {
	VendorID     string    `json:"vendor_id"`
	DeviceID     string    `json:"device_id"`
	NumVFs       int       `json:"num_vfs"`
	Mode         SRIOVMode `json:"mode,omitempty"`
	EnableSwitch bool      `json:"enable_switch,omitempty"`
	Description  string    `json:"description,omitempty"`
}

// BondConfig defines VF-LAG bonding configuration
type BondConfig struct {
	BondName        string   `json:"bond_name"`
	SlaveInterfaces []string `json:"slave_interfaces"`
	Mode            string   `json:"mode,omitempty"`
	MIIMonitor      int      `json:"mii_monitor,omitempty"`
}

// SRIOVConfig represents the main configuration for SR-IOV management
type SRIOVConfig struct {
	Version        string         `json:"version"`
	Description    string         `json:"description"`
	DevicePolicies []DevicePolicy `json:"device_policies"`
	BondConfigs    []BondConfig   `json:"bond_configs,omitempty"`
	LogLevel       string         `json:"log_level,omitempty"`
	DryRun         bool           `json:"dry_run,omitempty"`
}

// LoadConfig loads SR-IOV configuration from a JSON file
func LoadConfig(configPath string) (*SRIOVConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config SRIOVConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Set defaults
	if config.Version == "" {
		config.Version = "1.0"
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}

	return &config, nil
}

// SaveConfig saves SR-IOV configuration to a JSON file
func SaveConfig(config *SRIOVConfig, configPath string) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// GetDevicePolicy finds the appropriate policy for a device
func (c *SRIOVConfig) GetDevicePolicy(vendorID, deviceID string) *DevicePolicy {
	for _, policy := range c.DevicePolicies {
		if strings.EqualFold(policy.VendorID, vendorID) &&
			strings.EqualFold(policy.DeviceID, deviceID) {
			return &policy
		}
	}
	return nil
}

// ValidateConfig validates the configuration
func (c *SRIOVConfig) ValidateConfig() error {
	for i, policy := range c.DevicePolicies {
		if policy.VendorID == "" {
			return fmt.Errorf("device policy %d: vendor_id is required", i)
		}
		if policy.DeviceID == "" {
			return fmt.Errorf("device policy %d: device_id is required", i)
		}
		if policy.NumVFs <= 0 {
			return fmt.Errorf("device policy %d: num_vfs must be > 0", i)
		}
		if policy.Mode != "" && policy.Mode != ModeSingleHome && policy.Mode != ModeVFLag {
			return fmt.Errorf("device policy %d: invalid mode %s", i, policy.Mode)
		}
	}
	return nil
}

// CreateDefaultConfig creates a default configuration with common devices
func CreateDefaultConfig() *SRIOVConfig {
	return &SRIOVConfig{
		Version:     "1.0",
		Description: "SR-IOV Manager Configuration",
		LogLevel:    "info",
		DevicePolicies: []DevicePolicy{
			{
				VendorID:     "15b3", // Mellanox
				DeviceID:     "101e", // ConnectX-7
				NumVFs:       4,
				Mode:         ModeSingleHome,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7 in single-home mode",
			},
			{
				VendorID:     "15b3", // Mellanox
				DeviceID:     "101e", // ConnectX-7
				NumVFs:       4,
				Mode:         ModeVFLag,
				EnableSwitch: true,
				Description:  "Mellanox ConnectX-7 in VF-LAG mode",
			},
			{
				VendorID:     "1dd8", // Pensando
				DeviceID:     "1003", // DSC
				NumVFs:       1,
				Mode:         ModeSingleHome,
				EnableSwitch: false,
				Description:  "Pensando DSC for ROCE",
			},
		},
		BondConfigs: []BondConfig{
			{
				BondName:        "bond0",
				SlaveInterfaces: []string{"ens60f0np0", "ens60f1np1"},
				Mode:            "active-backup",
				MIIMonitor:      100,
			},
		},
	}
}
