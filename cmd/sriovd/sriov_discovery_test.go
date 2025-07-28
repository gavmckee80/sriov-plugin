package main

import (
	"testing"
)

func TestSupportsEswitchMode(t *testing.T) {
	tests := []struct {
		name     string
		vendorID string
		deviceID string
		expected bool
	}{
		{
			name:     "Mellanox device should support e-switch",
			vendorID: "0x15b3",
			deviceID: "0x1017",
			expected: true,
		},
		{
			name:     "Intel E810 device should support e-switch",
			vendorID: "0x8086",
			deviceID: "0x37d0",
			expected: true,
		},
		{
			name:     "Intel X710 device should support e-switch",
			vendorID: "0x8086",
			deviceID: "0x1572",
			expected: true,
		},
		{
			name:     "Unknown vendor should not support e-switch",
			vendorID: "0x1234",
			deviceID: "0x5678",
			expected: false,
		},
		{
			name:     "Intel device not in e-switch list should not support e-switch",
			vendorID: "0x8086",
			deviceID: "0x1234",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := supportsEswitchMode(tt.vendorID, tt.deviceID)
			if result != tt.expected {
				t.Errorf("supportsEswitchMode(%s, %s) = %v, expected %v", tt.vendorID, tt.deviceID, result, tt.expected)
			}
		})
	}
}

func TestSupportsRepresentors(t *testing.T) {
	tests := []struct {
		name          string
		vendorID      string
		deviceID      string
		interfaceName string
		expected      bool
	}{
		{
			name:          "Mellanox device without representor driver should not support representors",
			vendorID:      "0x15b3",
			deviceID:      "0x1017",
			interfaceName: "mlx5_0",
			expected:      false, // Will be false if mlx5e_rep driver is not loaded
		},
		{
			name:          "Intel E810 device without representor sysfs entries should not support representors",
			vendorID:      "0x8086",
			deviceID:      "0x37d0",
			interfaceName: "enp0s0",
			expected:      false, // Will be false if representor sysfs entries don't exist
		},
		{
			name:          "Unknown vendor should not support representors",
			vendorID:      "0x1234",
			deviceID:      "0x5678",
			interfaceName: "eth0",
			expected:      false,
		},
		{
			name:          "Intel device not in representor list should not support representors",
			vendorID:      "0x8086",
			deviceID:      "0x1234",
			interfaceName: "eth0",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := supportsRepresentors(tt.vendorID, tt.deviceID, tt.interfaceName)
			if result != tt.expected {
				t.Errorf("supportsRepresentors(%s, %s, %s) = %v, expected %v", tt.vendorID, tt.deviceID, tt.interfaceName, result, tt.expected)
			}
		})
	}
}

func TestIsRegularNetworkInterface(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		expected      bool
	}{
		{
			name:          "eth0 should be regular interface",
			interfaceName: "eth0",
			expected:      true,
		},
		{
			name:          "bond0 should be regular interface",
			interfaceName: "bond0",
			expected:      true,
		},
		{
			name:          "br0 should be regular interface",
			interfaceName: "br0",
			expected:      true,
		},
		{
			name:          "veth123 should be regular interface",
			interfaceName: "veth123",
			expected:      true,
		},
		{
			name:          "enp0s0 could be representor, so not regular",
			interfaceName: "enp0s0",
			expected:      false,
		},
		{
			name:          "p0s0 could be representor, so not regular",
			interfaceName: "p0s0",
			expected:      false,
		},
		{
			name:          "mlx5_0 should not be regular interface",
			interfaceName: "mlx5_0",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRegularNetworkInterface(tt.interfaceName)
			if result != tt.expected {
				t.Errorf("isRegularNetworkInterface(%s) = %v, expected %v", tt.interfaceName, result, tt.expected)
			}
		})
	}
}

func TestHasRepresentorSysfsEntries(t *testing.T) {
	tests := []struct {
		name          string
		interfaceName string
		expected      bool
	}{
		{
			name:          "Empty interface name should return false",
			interfaceName: "",
			expected:      false,
		},
		{
			name:          "Non-existent interface should return false",
			interfaceName: "nonexistent_interface",
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRepresentorSysfsEntries(tt.interfaceName)
			if result != tt.expected {
				t.Errorf("hasRepresentorSysfsEntries(%s) = %v, expected %v", tt.interfaceName, result, tt.expected)
			}
		})
	}
}
