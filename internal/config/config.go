package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the SR-IOV configuration
type Config struct {
	Discovery DiscoveryConfig `yaml:"discovery"`
	Pools     []Pool          `yaml:"pools"`
}

// DiscoveryConfig represents discovery configuration options
type DiscoveryConfig struct {
	// AllowedVendorIDs is a list of vendor IDs to discover and manage
	// If empty, all vendors are allowed
	AllowedVendorIDs []string `yaml:"allowed_vendor_ids"`

	// ExcludedVendorIDs is a list of vendor IDs to exclude from discovery
	// This takes precedence over AllowedVendorIDs
	ExcludedVendorIDs []string `yaml:"excluded_vendor_ids"`

	// EnableRepresentorDiscovery enables/disables representor discovery
	EnableRepresentorDiscovery bool `yaml:"enable_representor_discovery"`

	// EnableSwitchdevModeCheck enables/disables switchdev mode checking
	EnableSwitchdevModeCheck bool `yaml:"enable_switchdev_mode_check"`
}

// Pool represents a VF pool configuration
type Pool struct {
	Name             string   `yaml:"name"`
	PfPCI            string   `yaml:"pf_pci"`
	VFRange          string   `yaml:"vf_range"`
	Mask             bool     `yaml:"mask"`
	MaskReason       string   `yaml:"mask_reason"`
	RequiredFeatures []string `yaml:"required_features"`
	NUMA             string   `yaml:"numa"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	return &config, nil
}

// ParseVFRange parses a VF range string like "0-3,5,7-9"
func ParseVFRange(rangeStr string) ([]int, error) {
	var indices []int
	parts := strings.Split(rangeStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			// Range like "0-3"
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start index: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end index: %s", rangeParts[1])
			}
			for i := start; i <= end; i++ {
				indices = append(indices, i)
			}
		} else {
			// Single index
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid index: %s", part)
			}
			indices = append(indices, index)
		}
	}

	return indices, nil
}

// IsVendorAllowed checks if a vendor ID is allowed based on the discovery configuration
func (c *Config) IsVendorAllowed(vendorID string) bool {
	// If no discovery config, allow all vendors
	if len(c.Discovery.AllowedVendorIDs) == 0 && len(c.Discovery.ExcludedVendorIDs) == 0 {
		return true
	}

	// Check if vendor is explicitly excluded
	for _, excludedID := range c.Discovery.ExcludedVendorIDs {
		if vendorID == excludedID {
			return false
		}
	}

	// If allowed vendor IDs are specified, check if vendor is in the list
	if len(c.Discovery.AllowedVendorIDs) > 0 {
		for _, allowedID := range c.Discovery.AllowedVendorIDs {
			if vendorID == allowedID {
				return true
			}
		}
		return false
	}

	// If no allowed vendors specified but vendor is not excluded, allow it
	return true
}

// GetDiscoveryConfig returns the discovery configuration
func (c *Config) GetDiscoveryConfig() DiscoveryConfig {
	return c.Discovery
}
