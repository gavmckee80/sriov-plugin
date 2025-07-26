package pkg

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// SRIOVManager handles SR-IOV device configuration
type SRIOVManager struct {
	config *SRIOVConfig
	logger *log.Logger
}

// NewSRIOVManager creates a new SR-IOV manager instance
func NewSRIOVManager(config *SRIOVConfig) *SRIOVManager {
	return &SRIOVManager{
		config: config,
		logger: log.New(os.Stdout, "[SR-IOV-MANAGER] ", log.LstdFlags),
	}
}

// DiscoverDevices discovers all SR-IOV capable devices
func (m *SRIOVManager) DiscoverDevices() ([]Device, error) {
	m.logger.Printf("Discovering SR-IOV capable devices...")

	devices, err := ParseLshwDynamic()
	if err != nil {
		return nil, fmt.Errorf("failed to discover devices: %v", err)
	}

	// Enrich with PCI and ethtool information
	devices, err = AttachPciInfo(devices)
	if err != nil {
		m.logger.Printf("Warning: failed to attach PCI info: %v", err)
	}

	devices, err = AttachEthtoolInfo(devices)
	if err != nil {
		m.logger.Printf("Warning: failed to attach ethtool info: %v", err)
	}

	// Filter for SR-IOV capable devices
	var sriovDevices []Device
	for _, device := range devices {
		if device.SRIOVCapable {
			sriovDevices = append(sriovDevices, device)
			m.logger.Printf("Found SR-IOV device: %s (PCI: %s, Vendor: %s, Product: %s)",
				device.Name, device.PCIAddress, device.Vendor, device.Product)
		}
	}

	m.logger.Printf("Discovered %d SR-IOV capable devices", len(sriovDevices))
	return sriovDevices, nil
}

// ConfigureDevices configures SR-IOV on all discovered devices according to policies
func (m *SRIOVManager) ConfigureDevices(devices []Device) error {
	m.logger.Printf("Configuring SR-IOV devices...")

	for _, device := range devices {
		if err := m.configureDevice(device); err != nil {
			m.logger.Printf("Error configuring device %s: %v", device.Name, err)
			continue
		}
	}

	return nil
}

// configureDevice configures a single SR-IOV device
func (m *SRIOVManager) configureDevice(device Device) error {
	m.logger.Printf("Configuring device: %s (PCI: %s)", device.Name, device.PCIAddress)

	// Extract vendor and device IDs from PCI address or use lshw data
	vendorID, deviceID := m.extractDeviceIDs(device)
	if vendorID == "" || deviceID == "" {
		return fmt.Errorf("could not determine vendor/device IDs for %s", device.Name)
	}

	// Find applicable policy
	policy := m.config.GetDevicePolicy(vendorID, deviceID)
	if policy == nil {
		m.logger.Printf("No policy found for device %s (Vendor: %s, Device: %s)",
			device.Name, vendorID, deviceID)
		return nil
	}

	m.logger.Printf("Applying policy for %s: %s", device.Name, policy.Description)

	// Check if device supports the requested number of VFs
	if device.SRIOVInfo != nil && policy.NumVFs > device.SRIOVInfo.TotalVFs {
		m.logger.Printf("Warning: Requested %d VFs but device only supports %d",
			policy.NumVFs, device.SRIOVInfo.TotalVFs)
		policy.NumVFs = device.SRIOVInfo.TotalVFs
	}

	// Configure switchdev mode if required
	if policy.EnableSwitch {
		if err := m.enableSwitchMode(device); err != nil {
			m.logger.Printf("Warning: Failed to enable switch mode for %s: %v", device.Name, err)
		}
	}

	// Enable SR-IOV
	if err := m.enableSRIOV(device, policy.NumVFs); err != nil {
		return fmt.Errorf("failed to enable SR-IOV: %v", err)
	}

	// Configure mode-specific settings
	switch policy.Mode {
	case ModeVFLag:
		if err := m.configureVFLagMode(device, policy); err != nil {
			m.logger.Printf("Warning: Failed to configure VF-LAG mode: %v", err)
		}
	case ModeSingleHome:
		// Single-home mode requires no additional configuration
		m.logger.Printf("Device %s configured in single-home mode", device.Name)
	}

	return nil
}

// extractDeviceIDs extracts vendor and device IDs from device information
func (m *SRIOVManager) extractDeviceIDs(device Device) (string, string) {
	// Try to extract from PCI address first
	if device.PCIAddress != "" && strings.Contains(device.PCIAddress, ":") {
		// Parse PCI address to get vendor/device IDs
		// This would require additional parsing logic
	}

	// Use lshw data if available
	// For now, we'll use a simple mapping based on known devices
	switch {
	case strings.Contains(strings.ToLower(device.Vendor), "mellanox"):
		return "15b3", "101e" // ConnectX-7
	case strings.Contains(strings.ToLower(device.Vendor), "pensando"):
		return "1dd8", "1003" // DSC
	case strings.Contains(strings.ToLower(device.Vendor), "intel"):
		return "8086", "1520" // I350
	default:
		return "", ""
	}
}

// enableSwitchMode enables switchdev mode for Mellanox devices
func (m *SRIOVManager) enableSwitchMode(device Device) error {
	m.logger.Printf("Enabling switch mode for %s", device.Name)

	// For Mellanox ConnectX-7, enable switchdev mode
	if strings.Contains(strings.ToLower(device.Vendor), "mellanox") {
		// Set device to switchdev mode
		cmd := exec.Command("mlxconfig", "-d", device.PCIAddress, "set", "SWITCH_DEVICE=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to enable switch mode: %s, %v", string(output), err)
		}
		m.logger.Printf("Switch mode enabled for %s", device.Name)
	}

	return nil
}

// enableSRIOV enables SR-IOV on a device
func (m *SRIOVManager) enableSRIOV(device Device, numVFs int) error {
	m.logger.Printf("Enabling SR-IOV on %s with %d VFs", device.Name, numVFs)

	// Find the sysfs path for the device
	sysfsPath := filepath.Join("/sys/bus/pci/devices", device.PCIAddress)
	sriovPath := filepath.Join(sysfsPath, "sriov_numvfs")

	// Check if SR-IOV is already enabled
	if data, err := os.ReadFile(sriovPath); err == nil {
		if currentVFs := strings.TrimSpace(string(data)); currentVFs != "0" {
			m.logger.Printf("SR-IOV already enabled on %s with %s VFs", device.Name, currentVFs)
			return nil
		}
	}

	// Enable SR-IOV by writing the number of VFs
	if err := os.WriteFile(sriovPath, []byte(strconv.Itoa(numVFs)), 0644); err != nil {
		return fmt.Errorf("failed to enable SR-IOV: %v", err)
	}

	m.logger.Printf("SR-IOV enabled on %s with %d VFs", device.Name, numVFs)
	return nil
}

// configureVFLagMode configures VF-LAG mode for bonding
func (m *SRIOVManager) configureVFLagMode(device Device, policy *DevicePolicy) error {
	m.logger.Printf("Configuring VF-LAG mode for %s", device.Name)

	// Find bond configuration for this device
	var bondConfig *BondConfig
	for _, bond := range m.config.BondConfigs {
		for _, slave := range bond.SlaveInterfaces {
			if slave == device.Name {
				bondConfig = &bond
				break
			}
		}
		if bondConfig != nil {
			break
		}
	}

	if bondConfig == nil {
		return fmt.Errorf("no bond configuration found for device %s", device.Name)
	}

	// Create bond interface
	if err := m.createBondInterface(bondConfig); err != nil {
		return fmt.Errorf("failed to create bond interface: %v", err)
	}

	return nil
}

// createBondInterface creates a bond interface
func (m *SRIOVManager) createBondInterface(bond *BondConfig) error {
	m.logger.Printf("Creating bond interface %s", bond.BondName)

	// Create bond interface using ip command
	cmd := exec.Command("ip", "link", "add", bond.BondName, "type", "bond")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Bond might already exist
		m.logger.Printf("Bond interface %s might already exist: %s", bond.BondName, string(output))
	}

	// Set bond mode
	if bond.Mode != "" {
		modePath := fmt.Sprintf("/sys/class/net/%s/bonding/mode", bond.BondName)
		if err := os.WriteFile(modePath, []byte(bond.Mode), 0644); err != nil {
			m.logger.Printf("Warning: Failed to set bond mode: %v", err)
		}
	}

	// Set MII monitor interval
	if bond.MIIMonitor > 0 {
		monitorPath := fmt.Sprintf("/sys/class/net/%s/bonding/miimon", bond.BondName)
		if err := os.WriteFile(monitorPath, []byte(strconv.Itoa(bond.MIIMonitor)), 0644); err != nil {
			m.logger.Printf("Warning: Failed to set MII monitor: %v", err)
		}
	}

	// Add slave interfaces
	for _, slave := range bond.SlaveInterfaces {
		cmd := exec.Command("ip", "link", "set", slave, "master", bond.BondName)
		if output, err := cmd.CombinedOutput(); err != nil {
			m.logger.Printf("Warning: Failed to add slave %s to bond: %s", slave, string(output))
		}
	}

	// Bring up bond interface
	cmd = exec.Command("ip", "link", "set", bond.BondName, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Printf("Warning: Failed to bring up bond interface: %s", string(output))
	}

	m.logger.Printf("Bond interface %s created successfully", bond.BondName)
	return nil
}

// Run performs the complete SR-IOV configuration process
func (m *SRIOVManager) Run() error {
	m.logger.Printf("Starting SR-IOV Manager...")

	// Discover devices
	devices, err := m.DiscoverDevices()
	if err != nil {
		return fmt.Errorf("device discovery failed: %v", err)
	}

	// Configure devices
	if err := m.ConfigureDevices(devices); err != nil {
		return fmt.Errorf("device configuration failed: %v", err)
	}

	m.logger.Printf("SR-IOV Manager completed successfully")
	return nil
}

// ValidateConfiguration validates the current system configuration
func (m *SRIOVManager) ValidateConfiguration() error {
	m.logger.Printf("Validating SR-IOV configuration...")

	// Validate config file
	if err := m.config.ValidateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %v", err)
	}

	// Check if required tools are available
	requiredTools := []string{"ip", "mlxconfig"}
	for _, tool := range requiredTools {
		if _, err := exec.LookPath(tool); err != nil {
			m.logger.Printf("Warning: Required tool %s not found", tool)
		}
	}

	// Check if SR-IOV is supported by kernel
	if _, err := os.Stat("/sys/bus/pci/devices"); os.IsNotExist(err) {
		return fmt.Errorf("PCI sysfs not available - SR-IOV not supported")
	}

	m.logger.Printf("Configuration validation completed")
	return nil
}
