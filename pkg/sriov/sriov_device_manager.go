package sriov

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// VF represents a Virtual Function
type VF struct {
	VFPCI       string          `json:"vf_pci"`
	PFPCI       string          `json:"pf_pci"`
	Interface   string          `json:"interface"`
	Representor string          `json:"representor"`
	NUMANode    string          `json:"numa_node"`
	LinkState   string          `json:"link_state"`
	LinkSpeed   string          `json:"link_speed"`
	Allocated   bool            `json:"allocated"`
	Masked      bool            `json:"masked"`
	Features    map[string]bool `json:"features"`
	RxRings     int             `json:"rx_rings"`
	TxRings     int             `json:"tx_rings"`
	RxMax       int             `json:"rx_max"`
	TxMax       int             `json:"tx_max"`
	RxChannels  int             `json:"rx_channels"`
	TxChannels  int             `json:"tx_channels"`
	LastUpdated string          `json:"last_updated"`
	Driver      string          `json:"driver"`
	Mode        string          `json:"mode"`
	State       string          `json:"state"`
	Pool        string          `json:"pool"`
}

// PF represents a Physical Function
type PF struct {
	PFPCI     string `json:"pf_pci"`
	Interface string `json:"interface"`
	VFs       []VF   `json:"vfs"`
	Pool      string `json:"pool"`
}

// DeviceManager manages SR-IOV device discovery
type DeviceManager struct{}

// NewDeviceManager creates a new device manager
func NewDeviceManager() *DeviceManager {
	return &DeviceManager{}
}

// getInterfaceNameForVF attempts to find the interface name for a VF
func getInterfaceNameForVF(vfPCI string) string {
	// Try to find interface name from sysfs
	// Look in /sys/bus/pci/devices/{vf_pci}/net/
	netPath := filepath.Join("/sys/bus/pci/devices", vfPCI, "net")

	if entries, err := os.ReadDir(netPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				return entry.Name()
			}
		}
	}

	// If no interface found, return empty string
	return ""
}

// getInterfaceNameForPF attempts to find the interface name for a PF
func getInterfaceNameForPF(pfPCI string) string {
	// Try to find interface name from sysfs
	// Look in /sys/bus/pci/devices/{pf_pci}/net/
	netPath := filepath.Join("/sys/bus/pci/devices", pfPCI, "net")

	if entries, err := os.ReadDir(netPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				return entry.Name()
			}
		}
	}

	// If no interface found, return empty string
	return ""
}

// formatVFName returns a user-friendly name for a VF
func formatVFName(vfPCI string, interfaceName string) string {
	if interfaceName != "" {
		// Extract VF number from PCI address
		if idx := strings.LastIndex(vfPCI, "-vf"); idx > 0 {
			vfNum := vfPCI[idx+3:] // Remove "-vf" prefix
			return fmt.Sprintf("%s vf %s", interfaceName, vfNum)
		}
	}
	// Fallback to PCI address if no interface name found
	return vfPCI
}

// formatPFName returns a user-friendly name for a PF
func formatPFName(pfPCI string, interfaceName string) string {
	if interfaceName != "" {
		return interfaceName
	}
	// Fallback to PCI address if no interface name found
	return pfPCI
}

// GetSRIOVDevices discovers SR-IOV capable devices
func (dm *DeviceManager) GetSRIOVDevices() ([]PF, error) {
	var pfs []PF
	// Scan /sys/bus/pci/devices for SR-IOV capable devices
	devices, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return nil, fmt.Errorf("failed to read PCI devices: %v", err)
	}

	for _, device := range devices {
		if !device.IsDir() {
			continue
		}

		pciAddr := device.Name()
		sriovPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "sriov_totalvfs")

		// Check if device supports SR-IOV
		if _, err := os.Stat(sriovPath); os.IsNotExist(err) {
			continue
		}

		// Read total VFs
		totalVFsData, err := os.ReadFile(sriovPath)
		if err != nil {
			continue
		}

		totalVFs, err := strconv.Atoi(strings.TrimSpace(string(totalVFsData)))
		if err != nil || totalVFs == 0 {
			continue
		}

		// Get interface name for PF
		pfInterface := getInterfaceNameForPF(pciAddr)

		pf := PF{
			PFPCI:     pciAddr,
			Interface: pfInterface,
			VFs:       []VF{},
		}

		// Discover VFs
		for i := 0; i < totalVFs; i++ {
			vfPCI := fmt.Sprintf("%s-vf%d", pciAddr, i)
			vfInterface := getInterfaceNameForVF(vfPCI)

			vf := VF{
				VFPCI:     vfPCI,
				PFPCI:     pciAddr,
				Interface: vfInterface,
				State:     "unknown",
			}

			pf.VFs = append(pf.VFs, vf)
		}

		pfs = append(pfs, pf)
	}

	return pfs, nil
}
