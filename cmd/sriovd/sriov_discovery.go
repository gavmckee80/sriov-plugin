package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"sriov-plugin/pkg/types"

	"github.com/safchain/ethtool"
	"github.com/sirupsen/logrus"
)

var (
	sriovCache = struct {
		pfs map[string]*types.PFInfo
		vfs map[string]*types.VFInfo
	}{
		pfs: make(map[string]*types.PFInfo),
		vfs: make(map[string]*types.VFInfo),
	}

	// Cache for pci.ids data to avoid parsing the file multiple times
	pciIDsCache     map[string]map[string]string
	pciIDsCacheOnce sync.Once

	// Global ethtool handle to avoid creating new handles for each interface
	ethHandle     *ethtool.Ethtool
	ethHandleOnce sync.Once
)

// discoverSRIOVDevices discovers all SR-IOV capable devices and their VFs
func (s *server) discoverSRIOVDevices() error {
	s.sriovCache.Lock()
	defer s.sriovCache.Unlock()

	s.sriovCache.pfs = make(map[string]*types.PFInfo)
	s.sriovCache.vfs = make(map[string]*types.VFInfo)

	s.logger.Info("Starting SR-IOV device discovery")

	// Scan /sys/bus/pci/devices for SR-IOV capable devices
	devices, err := os.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return fmt.Errorf("failed to read PCI devices: %v", err)
	}

	s.logger.WithField("total_devices", len(devices)).Info("scanning PCI devices")

	s.logger.Info("starting device scan loop")

	for _, device := range devices {
		// Skip only if it's not a directory and not a symlink
		// PCI devices can be symlinks, so we need to check both
		if !device.IsDir() {
			// Check if it's a symlink by trying to read it
			devicePath := filepath.Join("/sys/bus/pci/devices", device.Name())
			if _, err := os.Stat(devicePath); os.IsNotExist(err) {
				continue
			}
		}

		pciAddr := device.Name()

		// Skip non-PCI devices
		if !strings.Contains(pciAddr, ":") {
			continue
		}

		sriovTotalPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "sriov_totalvfs")

		// Check if device supports SR-IOV
		if _, err := os.Stat(sriovTotalPath); os.IsNotExist(err) {
			continue
		}

		s.logger.WithField("pci", pciAddr).Debug("found SR-IOV capable device")

		// Read total VFs
		totalVFsData, err := os.ReadFile(sriovTotalPath)
		if err != nil {
			s.logger.WithError(err).WithField("pf", pciAddr).Warn("failed to read total VFs")
			continue
		}

		totalVFs, err := strconv.Atoi(strings.TrimSpace(string(totalVFsData)))
		if err != nil || totalVFs == 0 {
			s.logger.WithFields(logrus.Fields{
				"pci":       pciAddr,
				"total_vfs": strings.TrimSpace(string(totalVFsData)),
				"error":     err,
			}).Debug("skipping device - no VFs or parse error")
			continue
		}

		// Read current number of VFs
		numVFsPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "sriov_numvfs")
		numVFsData, err := os.ReadFile(numVFsPath)
		if err != nil {
			s.logger.WithError(err).WithField("pf", pciAddr).Warn("failed to read current VFs")
			continue
		}

		numVFs, err := strconv.Atoi(strings.TrimSpace(string(numVFsData)))
		if err != nil {
			s.logger.WithError(err).WithField("pf", pciAddr).Warn("failed to parse current VFs")
			continue
		}

		// Get PF interface name
		pfInterface := getInterfaceName(pciAddr)

		pfInfo := &types.PFInfo{
			PCIAddress:       pciAddr,
			InterfaceName:    pfInterface,
			Driver:           getDriverName(pciAddr),
			TotalVFs:         totalVFs,
			NumVFs:           numVFs,
			SRIOVEnabled:     numVFs > 0,
			NUMANode:         getNUMANode(pciAddr),
			LinkState:        getLinkState(pciAddr),
			LinkSpeed:        getLinkSpeed(pciAddr),
			MTU:              getMTU(pciAddr),
			MACAddress:       getMACAddress(pciAddr),
			Features:         getPFFeatures(pciAddr),
			Channels:         getPFChannels(pciAddr),
			Rings:            getPFRings(pciAddr),
			Properties:       getPFProperties(pciAddr),
			Capabilities:     getPCICapabilities(pciAddr),
			DeviceClass:      getDeviceClass(pciAddr),
			VendorID:         getVendorID(pciAddr),
			DeviceID:         getDeviceID(pciAddr),
			SubsysVendor:     getSubsysVendor(pciAddr),
			SubsysDevice:     getSubsysDevice(pciAddr),
			Description:      getDeviceDescription(pciAddr),
			VendorName:       getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).VendorName,
			DeviceName:       getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).DeviceName,
			SubsysVendorName: getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysVendorName,
			SubsysDeviceName: getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysDeviceName,
			VFs:              make(map[string]*types.VFInfo),
		}

		s.sriovCache.pfs[pciAddr] = pfInfo

		s.logger.WithFields(logrus.Fields{
			"pf":          pciAddr,
			"interface":   pfInterface,
			"total_vfs":   totalVFs,
			"enabled_vfs": numVFs,
		}).Info("discovered SR-IOV PF")

		// Discover VFs if SR-IOV is enabled
		if numVFs > 0 {
			s.discoverVFsForPF(pciAddr, pfInfo)
		}
	}

	s.logger.WithFields(logrus.Fields{
		"pfs": len(s.sriovCache.pfs),
		"vfs": len(s.sriovCache.vfs),
	}).Info("SR-IOV discovery completed")

	return nil
}

// discoverVFsForPF discovers all VFs for a given PF
func (s *server) discoverVFsForPF(pfPCI string, pfInfo *types.PFInfo) {
	s.logger.WithField("pf", pfPCI).Info("discovering VFs")

	// Look for virtfn* links in the PF directory
	virtfnPath := filepath.Join("/sys/bus/pci/devices", pfPCI, "virtfn*")
	matches, err := filepath.Glob(virtfnPath)
	if err != nil {
		s.logger.WithError(err).WithField("pf", pfPCI).Error("failed to glob virtfn links")
		return
	}

	for _, match := range matches {
		linkTarget, err := os.Readlink(match)
		if err != nil {
			s.logger.WithError(err).WithField("virtfn", match).Warn("failed to read virtfn link")
			continue
		}

		// Extract VF number from virtfn path
		// Format: /sys/bus/pci/devices/0000:31:00.0/virtfn15 -> ../0000:31:00.2
		virtfnParts := strings.Split(match, "virtfn")
		if len(virtfnParts) < 2 {
			continue
		}

		vfNumStr := virtfnParts[1]
		vfIndex, err := strconv.Atoi(vfNumStr)
		if err != nil {
			s.logger.WithError(err).WithField("vf_num", vfNumStr).Warn("failed to parse VF number")
			continue
		}

		// Extract the real VF PCI address from the link target
		// Format: ../0000:31:00.2 -> 0000:31:00.2
		if strings.HasPrefix(linkTarget, "../") {
			vfPCI := linkTarget[3:] // Remove "../" prefix

			// Get VF interface name
			vfInterface := getInterfaceName(vfPCI)

			vfInfo := &types.VFInfo{
				PCIAddress:       vfPCI,
				PFPCIAddress:     pfPCI,
				InterfaceName:    vfInterface,
				VFIndex:          vfIndex,
				Driver:           getDriverName(vfPCI),
				LinkState:        getLinkState(vfPCI),
				LinkSpeed:        getLinkSpeed(vfPCI),
				NUMANode:         getNUMANode(vfPCI),
				MTU:              getMTU(vfPCI),
				MACAddress:       getMACAddress(vfPCI),
				Features:         getVFFeatures(vfPCI),
				Channels:         getVFChannels(vfPCI),
				Rings:            getVFRings(vfPCI),
				Properties:       getVFProperties(vfPCI),
				Capabilities:     getPCICapabilities(vfPCI),
				DeviceClass:      getDeviceClass(vfPCI),
				VendorID:         getVendorID(vfPCI),
				DeviceID:         getDeviceID(vfPCI),
				SubsysVendor:     getSubsysVendor(vfPCI),
				SubsysDevice:     getSubsysDevice(vfPCI),
				Description:      getDeviceDescription(vfPCI),
				VendorName:       getPCIVendorDeviceInfo(getVendorID(vfPCI), getDeviceID(vfPCI), getSubsysVendor(vfPCI), getSubsysDevice(vfPCI)).VendorName,
				DeviceName:       getPCIVendorDeviceInfo(getVendorID(vfPCI), getDeviceID(vfPCI), getSubsysVendor(vfPCI), getSubsysDevice(vfPCI)).DeviceName,
				SubsysVendorName: getPCIVendorDeviceInfo(getVendorID(vfPCI), getDeviceID(vfPCI), getSubsysVendor(vfPCI), getSubsysDevice(vfPCI)).SubsysVendorName,
				SubsysDeviceName: getPCIVendorDeviceInfo(getVendorID(vfPCI), getDeviceID(vfPCI), getSubsysVendor(vfPCI), getSubsysDevice(vfPCI)).SubsysDeviceName,

				Allocated: false,
				Masked:    false,
				Pool:      "",
			}

			pfInfo.VFs[vfPCI] = vfInfo
			s.sriovCache.vfs[vfPCI] = vfInfo

			s.logger.WithFields(logrus.Fields{
				"vf":        vfPCI,
				"pf":        pfPCI,
				"vf_index":  vfIndex,
				"interface": vfInterface,
			}).Debug("discovered VF")
		}
	}

	s.logger.WithFields(logrus.Fields{
		"pf":       pfPCI,
		"vf_count": len(pfInfo.VFs),
	}).Info("VF discovery completed for PF")
}

// getInterfaceName gets the interface name for a PCI device
func getInterfaceName(pciAddr string) string {
	netPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "net")

	if entries, err := os.ReadDir(netPath); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				return entry.Name()
			}
		}
	}

	return ""
}

// getDriverName gets the driver name for a PCI device
func getDriverName(pciAddr string) string {
	driverPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "driver")
	if link, err := os.Readlink(driverPath); err == nil {
		return filepath.Base(link)
	}
	return ""
}

// getNUMANode gets the NUMA node for a PCI device
func getNUMANode(pciAddr string) string {
	numaPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "numa_node")
	if data, err := os.ReadFile(numaPath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getLinkState gets the link state for a PCI device
func getLinkState(pciAddr string) string {
	// For now, return a default value
	// This could be enhanced to read from sysfs if available
	return "up"
}

// getLinkSpeed gets the link speed for a PCI device
func getLinkSpeed(pciAddr string) string {
	// For now, return a default value
	// This could be enhanced to read from sysfs if available
	return "unknown"
}

// getMTU gets the MTU for a PCI device
func getMTU(pciAddr string) string {
	// For now, return a default value
	// This could be enhanced to read from sysfs if available
	return "1500"
}

// getMACAddress gets the MAC address for a PCI device
func getMACAddress(pciAddr string) string {
	// For now, return a default value
	// This could be enhanced to read from sysfs if available
	return ""
}

// getPFFeatures gets the features for a PF
func getPFFeatures(pfPCI string) map[string]bool {
	interfaceName := getInterfaceName(pfPCI)
	if interfaceName == "" {
		return nil
	}
	return getFeatureFlags(interfaceName)
}

// getPFChannels gets the channels for a PF
func getPFChannels(pfPCI string) map[string]int {
	interfaceName := getInterfaceName(pfPCI)
	if interfaceName == "" {
		return nil
	}
	return getChannelCounts(interfaceName)
}

// getPFRings gets the rings for a PF
func getPFRings(pfPCI string) map[string]int {
	interfaceName := getInterfaceName(pfPCI)
	if interfaceName == "" {
		return nil
	}
	return getRingBufferSizes(interfaceName)
}

// getPFProperties gets the properties for a PF
func getPFProperties(pfPCI string) map[string]string {
	// For now, return empty map
	// This could be enhanced to read from sysfs if available
	return make(map[string]string)
}

// getVFFeatures gets the features for a VF
func getVFFeatures(vfPCI string) map[string]bool {
	interfaceName := getInterfaceName(vfPCI)
	if interfaceName == "" {
		return nil
	}
	return getFeatureFlags(interfaceName)
}

// getVFChannels gets the channels for a VF
func getVFChannels(vfPCI string) map[string]int {
	interfaceName := getInterfaceName(vfPCI)
	if interfaceName == "" {
		return nil
	}
	return getChannelCounts(interfaceName)
}

// getVFRings gets the rings for a VF
func getVFRings(vfPCI string) map[string]int {
	interfaceName := getInterfaceName(vfPCI)
	if interfaceName == "" {
		return nil
	}
	return getRingBufferSizes(interfaceName)
}

// getVFProperties gets the properties for a VF
func getVFProperties(vfPCI string) map[string]string {
	// For now, return empty map
	// This could be enhanced to read from sysfs if available
	return make(map[string]string)
}

// getPCICapabilities gets PCI capabilities from sysfs
func getPCICapabilities(pciAddr string) []types.PCICapability {
	capabilities := []types.PCICapability{}

	// Read PCI config space from sysfs
	configPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "config")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		logrus.WithError(err).WithField("pci", pciAddr).Debug("failed to read PCI config")
		return capabilities
	}

	if len(configData) < 64 {
		logrus.WithField("pci", pciAddr).Debug("PCI config too short")
		return capabilities
	}

	// Parse PCI capabilities from config space
	capabilities = parsePCICapabilities(configData)

	return capabilities
}

// parsePCICapabilities parses PCI capabilities from config space data
func parsePCICapabilities(configData []byte) []types.PCICapability {
	capabilities := []types.PCICapability{}

	// Start at capability pointer (offset 0x34)
	if len(configData) < 0x38 {
		return capabilities
	}

	capPtr := uint8(configData[0x34])
	if capPtr == 0 {
		return capabilities
	}

	// Follow capability chain
	for capPtr != 0 && capPtr < uint8(len(configData)-8) {
		if capPtr+8 > uint8(len(configData)) {
			break
		}

		capID := uint8(configData[capPtr])
		nextCap := uint8(configData[capPtr+1])

		capability := types.PCICapability{
			ID:   fmt.Sprintf("%02x", capID),
			Name: getCapabilityName(capID),
		}

		// Add version info for specific capabilities
		if capID == 0x10 { // PCI Express
			if capPtr+2 < uint8(len(configData)) {
				version := uint8(configData[capPtr+2])
				capability.Version = fmt.Sprintf("v%d", version)
			}
		}

		capabilities = append(capabilities, capability)
		capPtr = nextCap
	}

	return capabilities
}

// getCapabilityName returns the human-readable name for a PCI capability ID
func getCapabilityName(capID uint8) string {
	names := map[uint8]string{
		0x01: "Power Management",
		0x02: "AGP",
		0x03: "VPD",
		0x04: "Slot Identification",
		0x05: "MSI",
		0x09: "Vendor Specific",
		0x0a: "Debug Port",
		0x0b: "CompactPCI Hot Swap",
		0x0c: "PCI-X",
		0x10: "PCI Express",
		0x11: "MSI-X",
		0x12: "SATA Data/Index Conf.",
		0x13: "Advanced Features",
		0x14: "Virtual Channel",
		0x15: "Device Serial Number",
		0x16: "Power Budgeting",
		0x19: "Vendor Specific Information",
		0x1a: "RCRB Header",
		0x1b: "Vendor Specific Information",
		0x1c: "Root Complex Link Declaration",
		0x1d: "Root Complex Internal Link Control",
		0x1e: "Root Complex Event Collector Endpoint Association",
		0x1f: "Multi-Function Virtual Channel",
		0x20: "Virtual Channel Resource",
		0x21: "RCRB Header",
		0x22: "Vendor Specific Information",
		0x23: "Root Complex Link Declaration",
		0x24: "Root Complex Internal Link Control",
		0x25: "Root Complex Event Collector Endpoint Association",
		0x26: "Multi-Function Virtual Channel",
		0x27: "Virtual Channel Resource",
		0x28: "RCRB Header",
		0x29: "Vendor Specific Information",
		0x2a: "Root Complex Link Declaration",
		0x2b: "Root Complex Internal Link Control",
		0x2c: "Root Complex Event Collector Endpoint Association",
		0x2d: "Multi-Function Virtual Channel",
		0x2e: "Virtual Channel Resource",
		0x2f: "RCRB Header",
		0x30: "Vendor Specific Information",
		0x31: "Root Complex Link Declaration",
		0x32: "Root Complex Internal Link Control",
		0x33: "Root Complex Event Collector Endpoint Association",
		0x34: "Multi-Function Virtual Channel",
		0x35: "Virtual Channel Resource",
		0x36: "RCRB Header",
		0x37: "Vendor Specific Information",
		0x38: "Root Complex Link Declaration",
		0x39: "Root Complex Internal Link Control",
		0x3a: "Root Complex Event Collector Endpoint Association",
		0x3b: "Multi-Function Virtual Channel",
		0x3c: "Virtual Channel Resource",
		0x3d: "RCRB Header",
		0x3e: "Vendor Specific Information",
		0x3f: "Root Complex Link Declaration",
		0x40: "Root Complex Internal Link Control",
		0x41: "Root Complex Event Collector Endpoint Association",
		0x42: "Multi-Function Virtual Channel",
		0x43: "Virtual Channel Resource",
		0x44: "RCRB Header",
		0x45: "Vendor Specific Information",
		0x46: "Root Complex Link Declaration",
		0x47: "Root Complex Internal Link Control",
		0x48: "Root Complex Event Collector Endpoint Association",
		0x49: "Multi-Function Virtual Channel",
		0x4a: "Virtual Channel Resource",
		0x4b: "RCRB Header",
		0x4c: "Vendor Specific Information",
		0x4d: "Root Complex Link Declaration",
		0x4e: "Root Complex Internal Link Control",
		0x4f: "Root Complex Event Collector Endpoint Association",
		0x50: "Multi-Function Virtual Channel",
		0x51: "Virtual Channel Resource",
		0x52: "RCRB Header",
		0x53: "Vendor Specific Information",
		0x54: "Root Complex Link Declaration",
		0x55: "Root Complex Internal Link Control",
		0x56: "Root Complex Event Collector Endpoint Association",
		0x57: "Multi-Function Virtual Channel",
		0x58: "Virtual Channel Resource",
		0x59: "RCRB Header",
		0x5a: "Vendor Specific Information",
		0x5b: "Root Complex Link Declaration",
		0x5c: "Root Complex Internal Link Control",
		0x5d: "Root Complex Event Collector Endpoint Association",
		0x5e: "Multi-Function Virtual Channel",
		0x5f: "Virtual Channel Resource",
		0x60: "RCRB Header",
		0x61: "Vendor Specific Information",
		0x62: "Root Complex Link Declaration",
		0x63: "Root Complex Internal Link Control",
		0x64: "Root Complex Event Collector Endpoint Association",
		0x65: "Multi-Function Virtual Channel",
		0x66: "Virtual Channel Resource",
		0x67: "RCRB Header",
		0x68: "Vendor Specific Information",
		0x69: "Root Complex Link Declaration",
		0x6a: "Root Complex Internal Link Control",
		0x6b: "Root Complex Event Collector Endpoint Association",
		0x6c: "Multi-Function Virtual Channel",
		0x6d: "Virtual Channel Resource",
		0x6e: "RCRB Header",
		0x6f: "Vendor Specific Information",
		0x70: "Root Complex Link Declaration",
		0x71: "Root Complex Internal Link Control",
		0x72: "Root Complex Event Collector Endpoint Association",
		0x73: "Multi-Function Virtual Channel",
		0x74: "Virtual Channel Resource",
		0x75: "RCRB Header",
		0x76: "Vendor Specific Information",
		0x77: "Root Complex Link Declaration",
		0x78: "Root Complex Internal Link Control",
		0x79: "Root Complex Event Collector Endpoint Association",
		0x7a: "Multi-Function Virtual Channel",
		0x7b: "Virtual Channel Resource",
		0x7c: "RCRB Header",
		0x7d: "Vendor Specific Information",
		0x7e: "Root Complex Link Declaration",
		0x7f: "Root Complex Internal Link Control",
		0x80: "Root Complex Event Collector Endpoint Association",
		0x81: "Multi-Function Virtual Channel",
		0x82: "Virtual Channel Resource",
		0x83: "RCRB Header",
		0x84: "Vendor Specific Information",
		0x85: "Root Complex Link Declaration",
		0x86: "Root Complex Internal Link Control",
		0x87: "Root Complex Event Collector Endpoint Association",
		0x88: "Multi-Function Virtual Channel",
		0x89: "Virtual Channel Resource",
		0x8a: "RCRB Header",
		0x8b: "Vendor Specific Information",
		0x8c: "Root Complex Link Declaration",
		0x8d: "Root Complex Internal Link Control",
		0x8e: "Root Complex Event Collector Endpoint Association",
		0x8f: "Multi-Function Virtual Channel",
		0x90: "Virtual Channel Resource",
		0x91: "RCRB Header",
		0x92: "Vendor Specific Information",
		0x93: "Root Complex Link Declaration",
		0x94: "Root Complex Internal Link Control",
		0x95: "Root Complex Event Collector Endpoint Association",
		0x96: "Multi-Function Virtual Channel",
		0x97: "Virtual Channel Resource",
		0x98: "RCRB Header",
		0x99: "Vendor Specific Information",
		0x9a: "Root Complex Link Declaration",
		0x9b: "Root Complex Internal Link Control",
		0x9c: "Root Complex Event Collector Endpoint Association",
		0x9d: "Multi-Function Virtual Channel",
		0x9e: "Virtual Channel Resource",
		0x9f: "RCRB Header",
		0xa0: "Vendor Specific Information",
		0xa1: "Root Complex Link Declaration",
		0xa2: "Root Complex Internal Link Control",
		0xa3: "Root Complex Event Collector Endpoint Association",
		0xa4: "Multi-Function Virtual Channel",
		0xa5: "Virtual Channel Resource",
		0xa6: "RCRB Header",
		0xa7: "Vendor Specific Information",
		0xa8: "Root Complex Link Declaration",
		0xa9: "Root Complex Internal Link Control",
		0xaa: "Root Complex Event Collector Endpoint Association",
		0xab: "Multi-Function Virtual Channel",
		0xac: "Virtual Channel Resource",
		0xad: "RCRB Header",
		0xae: "Vendor Specific Information",
		0xaf: "Root Complex Link Declaration",
		0xb0: "Root Complex Internal Link Control",
		0xb1: "Root Complex Event Collector Endpoint Association",
		0xb2: "Multi-Function Virtual Channel",
		0xb3: "Virtual Channel Resource",
		0xb4: "RCRB Header",
		0xb5: "Vendor Specific Information",
		0xb6: "Root Complex Link Declaration",
		0xb7: "Root Complex Internal Link Control",
		0xb8: "Root Complex Event Collector Endpoint Association",
		0xb9: "Multi-Function Virtual Channel",
		0xba: "Virtual Channel Resource",
		0xbb: "RCRB Header",
		0xbc: "Vendor Specific Information",
		0xbd: "Root Complex Link Declaration",
		0xbe: "Root Complex Internal Link Control",
		0xbf: "Root Complex Event Collector Endpoint Association",
		0xc0: "Multi-Function Virtual Channel",
		0xc1: "Virtual Channel Resource",
		0xc2: "RCRB Header",
		0xc3: "Vendor Specific Information",
		0xc4: "Root Complex Link Declaration",
		0xc5: "Root Complex Internal Link Control",
		0xc6: "Root Complex Event Collector Endpoint Association",
		0xc7: "Multi-Function Virtual Channel",
		0xc8: "Virtual Channel Resource",
		0xc9: "RCRB Header",
		0xca: "Vendor Specific Information",
		0xcb: "Root Complex Link Declaration",
		0xcc: "Root Complex Internal Link Control",
		0xcd: "Root Complex Event Collector Endpoint Association",
		0xce: "Multi-Function Virtual Channel",
		0xcf: "Virtual Channel Resource",
		0xd0: "RCRB Header",
		0xd1: "Vendor Specific Information",
		0xd2: "Root Complex Link Declaration",
		0xd3: "Root Complex Internal Link Control",
		0xd4: "Root Complex Event Collector Endpoint Association",
		0xd5: "Multi-Function Virtual Channel",
		0xd6: "Virtual Channel Resource",
		0xd7: "RCRB Header",
		0xd8: "Vendor Specific Information",
		0xd9: "Root Complex Link Declaration",
		0xda: "Root Complex Internal Link Control",
		0xdb: "Root Complex Event Collector Endpoint Association",
		0xdc: "Multi-Function Virtual Channel",
		0xdd: "Virtual Channel Resource",
		0xde: "RCRB Header",
		0xdf: "Vendor Specific Information",
		0xe0: "Root Complex Link Declaration",
		0xe1: "Root Complex Internal Link Control",
		0xe2: "Root Complex Event Collector Endpoint Association",
		0xe3: "Multi-Function Virtual Channel",
		0xe4: "Virtual Channel Resource",
		0xe5: "RCRB Header",
		0xe6: "Vendor Specific Information",
		0xe7: "Root Complex Link Declaration",
		0xe8: "Root Complex Internal Link Control",
		0xe9: "Root Complex Event Collector Endpoint Association",
		0xea: "Multi-Function Virtual Channel",
		0xeb: "Virtual Channel Resource",
		0xec: "RCRB Header",
		0xed: "Vendor Specific Information",
		0xee: "Root Complex Link Declaration",
		0xef: "Root Complex Internal Link Control",
		0xf0: "Root Complex Event Collector Endpoint Association",
		0xf1: "Multi-Function Virtual Channel",
		0xf2: "Virtual Channel Resource",
		0xf3: "RCRB Header",
		0xf4: "Vendor Specific Information",
		0xf5: "Root Complex Link Declaration",
		0xf6: "Root Complex Internal Link Control",
		0xf7: "Root Complex Event Collector Endpoint Association",
		0xf8: "Multi-Function Virtual Channel",
		0xf9: "Virtual Channel Resource",
		0xfa: "RCRB Header",
		0xfb: "Vendor Specific Information",
		0xfc: "Root Complex Link Declaration",
		0xfd: "Root Complex Internal Link Control",
		0xfe: "Root Complex Event Collector Endpoint Association",
		0xff: "Multi-Function Virtual Channel",
	}

	if name, exists := names[capID]; exists {
		return name
	}
	return fmt.Sprintf("Unknown Capability 0x%02x", capID)
}

// getDeviceClass gets the device class for a PCI device
func getDeviceClass(pciAddr string) string {
	classPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "class")
	if data, err := os.ReadFile(classPath); err == nil {
		class := strings.TrimSpace(string(data))
		// Convert hex class to human readable
		if len(class) >= 6 {
			classCode := class[2:6] // Skip "0x" prefix
			return getDeviceClassDescription(classCode)
		}
		return class
	}
	return ""
}

// getVendorID gets the vendor ID for a PCI device
func getVendorID(pciAddr string) string {
	vendorPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "vendor")
	if data, err := os.ReadFile(vendorPath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getDeviceID gets the device ID for a PCI device
func getDeviceID(pciAddr string) string {
	devicePath := filepath.Join("/sys/bus/pci/devices", pciAddr, "device")
	if data, err := os.ReadFile(devicePath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getSubsysVendor gets the subsystem vendor ID for a PCI device
func getSubsysVendor(pciAddr string) string {
	subsysVendorPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "subsystem_vendor")
	if data, err := os.ReadFile(subsysVendorPath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getSubsysDevice gets the subsystem device ID for a PCI device
func getSubsysDevice(pciAddr string) string {
	subsysDevicePath := filepath.Join("/sys/bus/pci/devices", pciAddr, "subsystem_device")
	if data, err := os.ReadFile(subsysDevicePath); err == nil {
		return strings.TrimSpace(string(data))
	}
	return ""
}

// getDeviceDescription gets a human-readable description for a PCI device
func getDeviceDescription(pciAddr string) string {
	vendorID := getVendorID(pciAddr)
	deviceID := getDeviceID(pciAddr)
	deviceClass := getDeviceClass(pciAddr)

	if vendorID == "" || deviceID == "" {
		return ""
	}

	// Try to get a more descriptive name based on vendor and device IDs
	// This could be enhanced with a vendor/device database
	desc := fmt.Sprintf("%s (Vendor %s Device %s)", deviceClass, vendorID, deviceID)

	return desc
}

// getDeviceClassDescription converts PCI class code to human readable description
func getDeviceClassDescription(classCode string) string {
	classMap := map[string]string{
		"0200": "network",
		"0201": "network",
		"0202": "network",
		"0203": "network",
		"0204": "network",
		"0205": "network",
		"0206": "network",
		"0207": "network",
		"0208": "network",
		"0209": "network",
		"020a": "network",
		"020b": "network",
		"020c": "network",
		"020d": "network",
		"020e": "network",
		"020f": "network",
		"0300": "display",
		"0400": "multimedia",
		"0500": "memory",
		"0600": "bridge",
		"0700": "simple_communications",
		"0800": "base_system_peripherals",
		"0900": "input_devices",
		"0a00": "docking_stations",
		"0b00": "processors",
		"0c00": "serial_bus_controllers",
		"0d00": "wireless_controllers",
		"0e00": "intelligent_controllers",
		"0f00": "satellite_communications",
		"1000": "encryption_decryption",
		"1100": "data_acquisition",
		"ff00": "unassigned_class",
	}

	if desc, exists := classMap[classCode]; exists {
		return desc
	}
	return fmt.Sprintf("unknown_class_%s", classCode)
}

// getRingBufferSizes gets ring buffer sizes for a network interface
func getRingBufferSizes(interfaceName string) map[string]int {
	if interfaceName == "" {
		logrus.WithField("interface", interfaceName).Debug("empty interface name, skipping ring buffer discovery")
		return nil
	}

	logrus.WithField("interface", interfaceName).Debug("getting ring buffer sizes")

	// Get global ethtool handle
	ethHandleOnce.Do(func() {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			logrus.WithError(err).Debug("failed to create global ethtool handle")
		}
	})

	if ethHandle == nil {
		logrus.WithField("interface", interfaceName).Debug("no ethtool handle available")
		return nil
	}

	rings, err := ethHandle.GetRing(interfaceName)
	if err != nil {
		logrus.WithError(err).WithField("interface", interfaceName).Debug("failed to get ring parameters")
		return nil
	}

	result := make(map[string]int)
	result["rx_max"] = int(rings.RxMaxPending)
	result["tx_max"] = int(rings.TxMaxPending)
	result["rx_mini_max"] = int(rings.RxMiniMaxPending)
	result["rx_jumbo_max"] = int(rings.RxJumboMaxPending)
	result["rx_current"] = int(rings.RxPending)
	result["tx_current"] = int(rings.TxPending)
	result["rx_mini_current"] = int(rings.RxMiniPending)
	result["rx_jumbo_current"] = int(rings.RxJumboPending)

	logrus.WithFields(logrus.Fields{
		"interface": interfaceName,
		"rings":     result,
	}).Debug("successfully retrieved ring buffer sizes")

	return result
}

// getChannelCounts gets channel counts for a network interface
func getChannelCounts(interfaceName string) map[string]int {
	if interfaceName == "" {
		logrus.WithField("interface", interfaceName).Debug("empty interface name, skipping channel discovery")
		return nil
	}

	logrus.WithField("interface", interfaceName).Debug("getting channel counts")

	// Get global ethtool handle
	ethHandleOnce.Do(func() {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			logrus.WithError(err).Debug("failed to create global ethtool handle")
		}
	})

	if ethHandle == nil {
		logrus.WithField("interface", interfaceName).Debug("no ethtool handle available")
		return nil
	}

	channels, err := ethHandle.GetChannels(interfaceName)
	if err != nil {
		logrus.WithError(err).WithField("interface", interfaceName).Debug("failed to get channel parameters")
		return nil
	}

	result := make(map[string]int)
	result["rx"] = int(channels.RxCount)
	result["tx"] = int(channels.TxCount)
	result["other"] = int(channels.OtherCount)
	result["combined"] = int(channels.CombinedCount)

	logrus.WithFields(logrus.Fields{
		"interface": interfaceName,
		"channels":  result,
	}).Debug("successfully retrieved channel counts")

	return result
}

// getFeatureFlags gets feature flags for a network interface
func getFeatureFlags(interfaceName string) map[string]bool {
	if interfaceName == "" {
		logrus.WithField("interface", interfaceName).Debug("empty interface name, skipping feature discovery")
		return nil
	}

	logrus.WithField("interface", interfaceName).Debug("getting feature flags")

	// Get global ethtool handle
	ethHandleOnce.Do(func() {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			logrus.WithError(err).Debug("failed to create global ethtool handle")
		}
	})

	if ethHandle == nil {
		logrus.WithField("interface", interfaceName).Debug("no ethtool handle available")
		return nil
	}

	features, err := ethHandle.Features(interfaceName)
	if err != nil {
		logrus.WithError(err).WithField("interface", interfaceName).Debug("failed to get features")
		return nil
	}

	result := make(map[string]bool)
	for name, enabled := range features {
		result[name] = enabled
	}

	logrus.WithFields(logrus.Fields{
		"interface": interfaceName,
		"features":  len(result),
	}).Debug("successfully retrieved feature flags")

	return result
}

// PCIVendorDevice represents vendor and device information from pci.ids
type PCIVendorDevice struct {
	VendorName       string
	DeviceName       string
	SubsysVendorName string
	SubsysDeviceName string
}

// parsePCIIDs parses the pci.ids file to get vendor and device names
func parsePCIIDs() map[string]map[string]string {
	vendorMap := make(map[string]map[string]string)

	// Read the pci.ids file
	data, err := os.ReadFile("/usr/share/hwdata/pci.ids")
	if err != nil {
		logrus.WithError(err).Debug("failed to read pci.ids file")
		return vendorMap
	}

	lines := strings.Split(string(data), "\n")
	var currentVendor string

	for _, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this is a vendor line (4 hex digits at start, not indented)
		if len(line) >= 4 && !strings.HasPrefix(originalLine, "\t") && !strings.HasPrefix(originalLine, " ") {
			// Split on first space or tab
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				vendorID := parts[0]
				vendorName := strings.Join(parts[1:], " ")
				currentVendor = vendorID
				vendorMap[vendorID] = make(map[string]string)
				vendorMap[vendorID]["vendor_name"] = vendorName
			}
		} else if currentVendor != "" && (strings.HasPrefix(originalLine, "\t") || strings.HasPrefix(originalLine, " ")) {
			// Check if this is a device line (indented with spaces or tabs)
			line = strings.TrimSpace(line)
			if len(line) >= 4 {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					deviceID := parts[0]
					deviceName := strings.Join(parts[1:], " ")
					if vendorMap[currentVendor] == nil {
						vendorMap[currentVendor] = make(map[string]string)
					}
					vendorMap[currentVendor][deviceID] = deviceName
				}
			}
		}
	}

	return vendorMap
}

// getPCIVendorDeviceInfo gets vendor and device names from pci.ids
func getPCIVendorDeviceInfo(vendorID, deviceID, subsysVendorID, subsysDeviceID string) PCIVendorDevice {
	// Remove "0x" prefix if present
	vendorID = strings.TrimPrefix(vendorID, "0x")
	deviceID = strings.TrimPrefix(deviceID, "0x")
	subsysVendorID = strings.TrimPrefix(subsysVendorID, "0x")
	subsysDeviceID = strings.TrimPrefix(subsysDeviceID, "0x")

	// Get cached pci.ids data (parsed only once)
	pciIDsCacheOnce.Do(func() {
		pciIDsCache = parsePCIIDs()
	})

	result := PCIVendorDevice{}

	// Get vendor name
	if vendorInfo, exists := pciIDsCache[vendorID]; exists {
		result.VendorName = vendorInfo["vendor_name"]

		// Get device name
		if deviceName, exists := vendorInfo[deviceID]; exists {
			result.DeviceName = deviceName
		}
	}

	// Get subsystem vendor and device names
	if subsysVendorInfo, exists := pciIDsCache[subsysVendorID]; exists {
		result.SubsysVendorName = subsysVendorInfo["vendor_name"]

		// Get subsystem device name
		if subsysDeviceName, exists := subsysVendorInfo[subsysDeviceID]; exists {
			result.SubsysDeviceName = subsysDeviceName
		}
	}

	return result
}
