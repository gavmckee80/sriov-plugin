package sriov

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DeviceManager handles SR-IOV device operations
type DeviceManager struct {
	// Add fields as needed
}

// VF represents a Virtual Function
type VF struct {
	VFPCI       string
	PFPCI       string
	Interface   string
	Representor string
	NUMANode    string
	LinkState   string
	LinkSpeed   string
	Allocated   bool
	Masked      bool
	Features    map[string]bool
	RxRings     uint32
	TxRings     uint32
	RxMax       uint32
	TxMax       uint32
	RxChannels  uint32
	TxChannels  uint32
	LastUpdated string
	Driver      string
	Mode        string
	State       string
	Pool        string
}

// PF represents a Physical Function
type PF struct {
	PFPCI     string
	Interface string
	VFs       []VF
	Pool      string
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

		// Read current VFs
		numVFsPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "sriov_numvfs")
		numVFsData, err := os.ReadFile(numVFsPath)
		if err != nil {
			continue
		}
		numVFs, err := strconv.Atoi(strings.TrimSpace(string(numVFsData)))
		if err != nil {
			continue
		}

		// Get network interface name
		netPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "net")
		netDirs, err := os.ReadDir(netPath)
		var interfaceName string
		if err == nil && len(netDirs) > 0 {
			interfaceName = netDirs[0].Name()
		}

		pf := PF{
			PFPCI:     pciAddr,
			Interface: interfaceName,
			VFs:       make([]VF, 0, numVFs),
		}

		// Discover VFs
		for i := 0; i < numVFs; i++ {
			vfPCI := fmt.Sprintf("%s-vf%d", pciAddr, i)
			vf := VF{
				VFPCI:       vfPCI,
				PFPCI:       pciAddr,
				Interface:   fmt.Sprintf("%svf%d", interfaceName, i),
				LastUpdated: time.Now().Format(time.RFC3339),
				State:       "available",
			}
			pf.VFs = append(pf.VFs, vf)
		}

		pfs = append(pfs, pf)
	}

	return pfs, nil
}

// NewDeviceManager creates a new device manager instance
func NewDeviceManager() *DeviceManager {
	return &DeviceManager{}
}
