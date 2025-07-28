package config

import (
	"os"
	"testing"
)

func TestParseVFRange(t *testing.T) {
	tests := []struct {
		name     string
		rangeStr string
		expected []int
		wantErr  bool
	}{
		{
			name:     "single index",
			rangeStr: "5",
			expected: []int{5},
			wantErr:  false,
		},
		{
			name:     "range",
			rangeStr: "0-3",
			expected: []int{0, 1, 2, 3},
			wantErr:  false,
		},
		{
			name:     "mixed range and single",
			rangeStr: "0-2,5,7-9",
			expected: []int{0, 1, 2, 5, 7, 8, 9},
			wantErr:  false,
		},
		{
			name:     "invalid range format",
			rangeStr: "0-1-2",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid start index",
			rangeStr: "abc-5",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "invalid end index",
			rangeStr: "0-abc",
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "empty string",
			rangeStr: "",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseVFRange(tt.rangeStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVFRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != len(tt.expected) {
				t.Errorf("ParseVFRange() length = %v, want %v", len(got), len(tt.expected))
				return
			}
			if !tt.wantErr {
				for i, v := range got {
					if v != tt.expected[i] {
						t.Errorf("ParseVFRange()[%d] = %v, want %v", i, v, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `
discovery:
  allowed_vendor_ids:
    - "0x15b3"
    - "0x8086"
  excluded_vendor_ids:
    - "0x1234"
  enable_representor_discovery: true
  enable_switchdev_mode_check: true

pools:
  - name: "test-pool"
    pf_pci: "0000:01:00.0"
    vf_range: "0-3,5"
    mask: false
    required_features: ["rx_checksum"]
    numa: "0"
  - name: "reserved-pool"
    pf_pci: "0000:01:00.0"
    vf_range: "4"
    mask: true
    mask_reason: "Reserved for testing"
    required_features: []
    numa: "0"
`
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading the config
	config, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Check discovery configuration
	if len(config.Discovery.AllowedVendorIDs) != 2 {
		t.Errorf("Expected 2 allowed vendor IDs, got %d", len(config.Discovery.AllowedVendorIDs))
	}
	if len(config.Discovery.ExcludedVendorIDs) != 1 {
		t.Errorf("Expected 1 excluded vendor ID, got %d", len(config.Discovery.ExcludedVendorIDs))
	}
	if !config.Discovery.EnableRepresentorDiscovery {
		t.Error("Expected representor discovery to be enabled")
	}
	if !config.Discovery.EnableSwitchdevModeCheck {
		t.Error("Expected switchdev mode check to be enabled")
	}

	if len(config.Pools) != 2 {
		t.Errorf("Expected 2 pools, got %d", len(config.Pools))
	}

	// Check first pool
	if config.Pools[0].Name != "test-pool" {
		t.Errorf("Expected pool name 'test-pool', got '%s'", config.Pools[0].Name)
	}
	if config.Pools[0].PfPCI != "0000:01:00.0" {
		t.Errorf("Expected PF PCI '0000:01:00.0', got '%s'", config.Pools[0].PfPCI)
	}
	if config.Pools[0].VFRange != "0-3,5" {
		t.Errorf("Expected VF range '0-3,5', got '%s'", config.Pools[0].VFRange)
	}
	if config.Pools[0].Mask {
		t.Error("Expected mask to be false")
	}
	if len(config.Pools[0].RequiredFeatures) != 1 {
		t.Errorf("Expected 1 required feature, got %d", len(config.Pools[0].RequiredFeatures))
	}
	if config.Pools[0].RequiredFeatures[0] != "rx_checksum" {
		t.Errorf("Expected required feature 'rx_checksum', got '%s'", config.Pools[0].RequiredFeatures[0])
	}

	// Check second pool
	if config.Pools[1].Name != "reserved-pool" {
		t.Errorf("Expected pool name 'reserved-pool', got '%s'", config.Pools[1].Name)
	}
	if !config.Pools[1].Mask {
		t.Error("Expected mask to be true")
	}
	if config.Pools[1].MaskReason != "Reserved for testing" {
		t.Errorf("Expected mask reason 'Reserved for testing', got '%s'", config.Pools[1].MaskReason)
	}
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("nonexistent-file.yaml")
	if err == nil {
		t.Error("Expected error when loading nonexistent file")
	}
}

func TestLoadConfigInvalidYAML(t *testing.T) {
	// Create a temporary config file with invalid YAML
	configContent := `
pools:
  - name: "test-pool"
    pf_pci: "0000:01:00.0"
    vf_range: "0-3,5"
    mask: false
    required_features: ["rx_checksum"
    numa: "0"
`
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test loading the invalid config
	_, err = LoadConfig(tmpfile.Name())
	if err == nil {
		t.Error("Expected error when loading invalid YAML")
	}
}

func TestIsVendorAllowed(t *testing.T) {
	// Test with empty configuration (allow all)
	config := &Config{}

	if !config.IsVendorAllowed("0x15b3") {
		t.Error("Expected vendor 0x15b3 to be allowed with empty config")
	}
	if !config.IsVendorAllowed("0x8086") {
		t.Error("Expected vendor 0x8086 to be allowed with empty config")
	}

	// Test with allowed vendor IDs
	config.Discovery.AllowedVendorIDs = []string{"0x15b3", "0x8086"}

	if !config.IsVendorAllowed("0x15b3") {
		t.Error("Expected vendor 0x15b3 to be allowed")
	}
	if !config.IsVendorAllowed("0x8086") {
		t.Error("Expected vendor 0x8086 to be allowed")
	}
	if config.IsVendorAllowed("0x1234") {
		t.Error("Expected vendor 0x1234 to be disallowed")
	}

	// Test with excluded vendor IDs
	config.Discovery.ExcludedVendorIDs = []string{"0x15b3"}

	if config.IsVendorAllowed("0x15b3") {
		t.Error("Expected vendor 0x15b3 to be excluded")
	}
	if !config.IsVendorAllowed("0x8086") {
		t.Error("Expected vendor 0x8086 to be allowed")
	}

	// Test with both allowed and excluded (excluded takes precedence)
	config.Discovery.AllowedVendorIDs = []string{"0x15b3", "0x8086"}
	config.Discovery.ExcludedVendorIDs = []string{"0x15b3"}

	if config.IsVendorAllowed("0x15b3") {
		t.Error("Expected vendor 0x15b3 to be excluded (excluded takes precedence)")
	}
	if !config.IsVendorAllowed("0x8086") {
		t.Error("Expected vendor 0x8086 to be allowed")
	}
}
