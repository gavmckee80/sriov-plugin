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
	s.sriovCache.representors = make(map[string]*types.RepresentorInfo)

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

		// Get vendor and device IDs for filtering
		vendorID := getVendorID(pciAddr)
		deviceID := getDeviceID(pciAddr)

		// Check if vendor is allowed based on configuration
		if s.config != nil && !s.config.IsVendorAllowed(vendorID) {
			s.logger.WithFields(logrus.Fields{
				"pci":       pciAddr,
				"vendor":    vendorID,
				"device":    deviceID,
				"interface": pfInterface,
			}).Debug("skipping device - vendor not allowed")
			continue
		}

		s.logger.WithFields(logrus.Fields{
			"pci":         pciAddr,
			"interface":   pfInterface,
			"vendor_id":   vendorID,
			"device_id":   deviceID,
			"vendor_name": getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).VendorName,
			"device_name": getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).DeviceName,
		}).Debug("found SR-IOV capable device")

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
			VendorID:         vendorID,
			DeviceID:         deviceID,
			SubsysVendor:     getSubsysVendor(pciAddr),
			SubsysDevice:     getSubsysDevice(pciAddr),
			Description:      getDeviceDescription(pciAddr),
			VendorName:       getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).VendorName,
			DeviceName:       getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).DeviceName,
			SubsysVendorName: getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysVendorName,
			SubsysDeviceName: getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysDeviceName,
			EswitchMode:      getEswitchMode(pfInterface),
			VFs:              make(map[string]*types.VFInfo),
		}

		s.sriovCache.pfs[pciAddr] = pfInfo

		s.logger.WithFields(logrus.Fields{
			"pf":           pciAddr,
			"interface":    pfInterface,
			"total_vfs":    totalVFs,
			"enabled_vfs":  numVFs,
			"eswitch_mode": pfInfo.EswitchMode,
			"vendor":       vendorID,
			"device":       deviceID,
		}).Info("discovered SR-IOV PF")

		// Discover VFs if SR-IOV is enabled
		if numVFs > 0 {
			s.discoverVFsForPF(pciAddr, pfInfo)
		}

		// Discover representors only if enabled and device meets criteria
		if numVFs > 0 && s.config != nil && s.config.Discovery.EnableRepresentorDiscovery {
			// Check switchdev mode if enabled
			if s.config.Discovery.EnableSwitchdevModeCheck {
				if pfInfo.EswitchMode == "switchdev" && supportsEswitchMode(pfInfo.VendorID, pfInfo.DeviceID) {
					// Additional safety check: verify that the device actually supports representors
					if supportsRepresentors(pfInfo.VendorID, pfInfo.DeviceID, pfInterface) {
						s.logger.WithFields(logrus.Fields{
							"pf":        pciAddr,
							"interface": pfInterface,
							"mode":      pfInfo.EswitchMode,
						}).Debug("PF is in switchdev mode and supports representors, discovering representors")
						s.discoverRepresentorsForPF(pciAddr, pfInfo)
					} else {
						s.logger.WithFields(logrus.Fields{
							"pf":        pciAddr,
							"interface": pfInterface,
							"vendor":    pfInfo.VendorID,
							"device":    pfInfo.DeviceID,
						}).Debug("skipping representor discovery - device does not support representors despite being in switchdev mode")
					}
				} else if pfInfo.EswitchMode == "switchdev" && !supportsEswitchMode(pfInfo.VendorID, pfInfo.DeviceID) {
					s.logger.WithFields(logrus.Fields{
						"pf":        pciAddr,
						"interface": pfInterface,
						"vendor":    pfInfo.VendorID,
						"device":    pfInfo.DeviceID,
					}).Debug("skipping representor discovery - device does not support e-switch mode")
				} else {
					s.logger.WithFields(logrus.Fields{
						"pf":        pciAddr,
						"interface": pfInterface,
						"num_vfs":   numVFs,
						"mode":      pfInfo.EswitchMode,
					}).Debug("skipping representor discovery - SR-IOV disabled or not in switchdev mode")
				}
			} else {
				// Switchdev mode check disabled, discover representors for all devices with VFs
				s.logger.WithFields(logrus.Fields{
					"pf":        pciAddr,
					"interface": pfInterface,
				}).Debug("switchdev mode check disabled, discovering representors")
				s.discoverRepresentorsForPF(pciAddr, pfInfo)
			}
		} else if numVFs > 0 && (s.config == nil || !s.config.Discovery.EnableRepresentorDiscovery) {
			s.logger.WithFields(logrus.Fields{
				"pf":        pciAddr,
				"interface": pfInterface,
			}).Debug("skipping representor discovery - representor discovery disabled")
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

	// Link representors to VFs
	s.linkRepresentorsToVFs(pfPCI, pfInfo)
}

// discoverRepresentorsForPF discovers representors associated with a Physical Function
func (s *server) discoverRepresentorsForPF(pfPCI string, pfInfo *types.PFInfo) {
	pfInfo.Representors = make(map[string]*types.RepresentorInfo)

	// Get PF interface name
	pfInterface := getInterfaceName(pfPCI)
	if pfInterface == "" {
		s.logger.WithField("pf", pfPCI).Debug("no interface found for PF, skipping representor discovery")
		return
	}

	// Double-check that the interface is in switchdev mode using stored value
	if pfInfo.EswitchMode != "switchdev" {
		s.logger.WithFields(logrus.Fields{
			"pf":        pfPCI,
			"interface": pfInterface,
			"mode":      pfInfo.EswitchMode,
		}).Debug("PF interface is not in switchdev mode, skipping representor discovery")
		return
	}

	// Additional safety check: verify representor support before proceeding
	if !supportsRepresentors(pfInfo.VendorID, pfInfo.DeviceID, pfInterface) {
		s.logger.WithFields(logrus.Fields{
			"pf":        pfPCI,
			"interface": pfInterface,
			"vendor":    pfInfo.VendorID,
			"device":    pfInfo.DeviceID,
		}).Debug("device does not support representors, skipping representor discovery")
		return
	}

	s.logger.WithFields(logrus.Fields{
		"pf":        pfPCI,
		"interface": pfInterface,
	}).Debug("starting representor discovery for PF")

	// Look for representors in the same namespace as the PF
	representors := findRepresentorsForPF(pfPCI, pfInterface)

	for _, rep := range representors {
		repInfo := &types.RepresentorInfo{
			InterfaceName:    rep.InterfaceName,
			PCIAddress:       rep.PCIAddress,
			Driver:           rep.Driver,
			VFIndex:          rep.VFIndex,
			PFPCIAddress:     pfPCI,
			LinkState:        rep.LinkState,
			LinkSpeed:        rep.LinkSpeed,
			NUMANode:         rep.NUMANode,
			MTU:              rep.MTU,
			MACAddress:       rep.MACAddress,
			Features:         rep.Features,
			Channels:         rep.Channels,
			Rings:            rep.Rings,
			Properties:       rep.Properties,
			DeviceClass:      rep.DeviceClass,
			Class:            rep.Class,
			Description:      rep.Description,
			VendorID:         rep.VendorID,
			DeviceID:         rep.DeviceID,
			SubsysVendor:     rep.SubsysVendor,
			SubsysDevice:     rep.SubsysDevice,
			VendorName:       rep.VendorName,
			DeviceName:       rep.DeviceName,
			SubsysVendorName: rep.SubsysVendorName,
			SubsysDeviceName: rep.SubsysDeviceName,
			AssociatedVF:     rep.AssociatedVF,
			RepresentorType:  rep.RepresentorType,
		}

		pfInfo.Representors[rep.InterfaceName] = repInfo
		s.sriovCache.representors[rep.InterfaceName] = repInfo
		s.logger.WithFields(logrus.Fields{
			"pf":            pfPCI,
			"representor":   rep.InterfaceName,
			"vf_index":      rep.VFIndex,
			"associated_vf": rep.AssociatedVF,
		}).Debug("discovered representor")
	}
}

// findRepresentorsForPF finds representors associated with a Physical Function
func findRepresentorsForPF(pfPCI, pfInterface string) []RepresentorData {
	var representors []RepresentorData

	// Method 1: Look for representors in /sys/class/net
	netDevices, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return representors
	}

	// fmt.Printf("DEBUG: Scanning %d network devices for representors of PF %s (interface: %s)\n", len(netDevices), pfPCI, pfInterface)

	for _, device := range netDevices {
		interfaceName := device.Name()

		// Skip the PF interface itself
		if interfaceName == pfInterface {
			continue
		}

		// Check if this is a representor for our PF
		if isRepresentorForPF(interfaceName, pfPCI, pfInterface) {
			// fmt.Printf("DEBUG: Found representor: %s for PF %s\n", interfaceName, pfPCI)
			repData := getRepresentorData(interfaceName, pfPCI)
			if repData != nil {
				representors = append(representors, *repData)
			}
		}
	}

	return representors
}

// isRepresentorForPF checks if an interface is a representor for a specific PF
func isRepresentorForPF(interfaceName, pfPCI, pfInterface string) bool {
	// Get the PCI address for this interface
	repPCI := getPCIAddressForInterface(interfaceName)
	if repPCI == "" {
		return false
	}

	// Method 1: Check for Mellanox representors (vendor 0x15b3, driver mlx5e_rep)
	vendorID := getVendorID(repPCI)
	driver := getDriverName(repPCI)

	if vendorID == "0x15b3" && driver == "mlx5e_rep" {
		// Additional check: make sure this representor belongs to our PF
		// by checking if the representor's PCI address is in the same domain as our PF
		pfDomain := strings.Split(pfPCI, ":")[0]
		repDomain := strings.Split(repPCI, ":")[0]
		if pfDomain == repDomain {
			return true
		}
	}

	// Method 2: Check for representor properties in sysfs (phys_switch_id)
	representorPath := filepath.Join("/sys/class/net", interfaceName, "phys_switch_id")
	if _, err := os.Stat(representorPath); err == nil {
		// Read the switch ID and compare with PF
		pfSwitchPath := filepath.Join("/sys/class/net", pfInterface, "phys_switch_id")
		if pfSwitchData, err := os.ReadFile(pfSwitchPath); err == nil {
			if repSwitchData, err := os.ReadFile(representorPath); err == nil {
				if strings.TrimSpace(string(pfSwitchData)) == strings.TrimSpace(string(repSwitchData)) {
					return true
				}
			}
		}
	}

	// Method 3: Check for representor port number (phys_port_name)
	portPath := filepath.Join("/sys/class/net", interfaceName, "phys_port_name")
	if _, err := os.Stat(portPath); err == nil {
		if portData, err := os.ReadFile(portPath); err == nil {
			portName := strings.TrimSpace(string(portData))
			if strings.Contains(portName, "pf") || strings.Contains(portName, "vf") {
				return true
			}
		}
	}

	// Method 4: Additional safety check - verify this is not a regular interface
	// Skip interfaces that are likely regular network interfaces
	if isRegularNetworkInterface(interfaceName) {
		return false
	}

	return false
}

// isRegularNetworkInterface checks if an interface is likely a regular network interface
// rather than a representor
func isRegularNetworkInterface(interfaceName string) bool {
	// Common regular interface patterns
	regularPatterns := []string{
		"eth", "en", "em", "p", "bond", "br", "veth", "docker", "cali", "flannel",
	}

	for _, pattern := range regularPatterns {
		if strings.HasPrefix(interfaceName, pattern) {
			// Additional check: if it's a pattern that could be a representor,
			// we need to be more careful
			if pattern == "en" || pattern == "p" {
				// These could be representors, so we need to check further
				continue
			}
			return true
		}
	}

	return false
}

// isSwitchdevMode checks if a network interface is in switchdev mode
func isSwitchdevMode(interfaceName string) bool {
	if interfaceName == "" {
		return false
	}

	// Check for switchdev mode by looking for the switchdev directory
	switchdevPath := filepath.Join("/sys/class/net", interfaceName, "switchdev")
	if _, err := os.Stat(switchdevPath); err == nil {
		return true
	}

	// Check for Mellanox devlink mode (compat/devlink/mode)
	devlinkModePath := filepath.Join("/sys/class/net", interfaceName, "compat/devlink/mode")
	if _, err := os.Stat(devlinkModePath); err == nil {
		if data, err := os.ReadFile(devlinkModePath); err == nil {
			mode := strings.TrimSpace(string(data))
			return mode == "switchdev"
		}
	}

	// Alternative check: look for eswitch mode
	eswitchPath := filepath.Join("/sys/class/net", interfaceName, "eswitch_mode")
	if _, err := os.Stat(eswitchPath); err == nil {
		if data, err := os.ReadFile(eswitchPath); err == nil {
			mode := strings.TrimSpace(string(data))
			// "switchdev" indicates switchdev mode, "legacy" indicates legacy mode
			return mode == "switchdev"
		}
	}

	return false
}

// getEswitchMode gets the eswitch mode for a network interface
func getEswitchMode(interfaceName string) string {
	if interfaceName == "" {
		return ""
	}

	// Check for Mellanox devlink mode (compat/devlink/mode)
	devlinkModePath := filepath.Join("/sys/class/net", interfaceName, "compat/devlink/mode")
	if _, err := os.Stat(devlinkModePath); err == nil {
		if data, err := os.ReadFile(devlinkModePath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}

	// Check for eswitch mode
	eswitchPath := filepath.Join("/sys/class/net", interfaceName, "eswitch_mode")
	if _, err := os.Stat(eswitchPath); err == nil {
		if data, err := os.ReadFile(eswitchPath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}

	// Check for switchdev directory as an alternative indicator
	switchdevPath := filepath.Join("/sys/class/net", interfaceName, "switchdev")
	if _, err := os.Stat(switchdevPath); err == nil {
		return "switchdev"
	}

	return ""
}

// getRepresentorData gets detailed information about a representor
func getRepresentorData(interfaceName, pfPCI string) *RepresentorData {
	// Get PCI address of the representor
	pciAddr := getPCIAddressForInterface(interfaceName)
	if pciAddr == "" {
		return nil
	}

	// Extract VF index from representor name or properties
	vfIndex := extractVFIndexFromRepresentor(interfaceName, pciAddr)

	// Determine representor type
	representorType := determineRepresentorType(interfaceName, pciAddr)

	// Get associated VF PCI address
	associatedVF := getAssociatedVFPCI(pfPCI, vfIndex)

	return &RepresentorData{
		InterfaceName:    interfaceName,
		PCIAddress:       pciAddr,
		Driver:           getInterfaceDriver(interfaceName),
		VFIndex:          vfIndex,
		PFPCIAddress:     pfPCI,
		LinkState:        getLinkState(pciAddr),
		LinkSpeed:        getLinkSpeed(pciAddr),
		NUMANode:         getNUMANode(pciAddr),
		MTU:              getMTU(pciAddr),
		MACAddress:       getMACAddress(pciAddr),
		Features:         getRepresentorFeatures(interfaceName),
		Channels:         getRepresentorChannels(interfaceName),
		Rings:            getRepresentorRings(interfaceName),
		Properties:       getRepresentorProperties(interfaceName),
		DeviceClass:      getDeviceClass(pciAddr),
		Class:            getDeviceClass(pciAddr),
		Description:      getDeviceDescription(pciAddr),
		VendorID:         getVendorID(pciAddr),
		DeviceID:         getDeviceID(pciAddr),
		SubsysVendor:     getSubsysVendor(pciAddr),
		SubsysDevice:     getSubsysDevice(pciAddr),
		VendorName:       getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).VendorName,
		DeviceName:       getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).DeviceName,
		SubsysVendorName: getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysVendorName,
		SubsysDeviceName: getPCIVendorDeviceInfo(getVendorID(pciAddr), getDeviceID(pciAddr), getSubsysVendor(pciAddr), getSubsysDevice(pciAddr)).SubsysDeviceName,
		AssociatedVF:     associatedVF,
		RepresentorType:  representorType,
	}
}

// RepresentorData holds representor information during discovery
type RepresentorData struct {
	InterfaceName    string
	PCIAddress       string
	Driver           string
	VFIndex          int
	PFPCIAddress     string
	LinkState        string
	LinkSpeed        string
	NUMANode         string
	MTU              string
	MACAddress       string
	Features         map[string]bool
	Channels         map[string]int
	Rings            map[string]int
	Properties       map[string]string
	DeviceClass      string
	Class            string
	Description      string
	VendorID         string
	DeviceID         string
	SubsysVendor     string
	SubsysDevice     string
	VendorName       string
	DeviceName       string
	SubsysVendorName string
	SubsysDeviceName string
	AssociatedVF     string
	RepresentorType  string
}

// getPCIAddressForInterface gets the PCI address for a network interface
func getPCIAddressForInterface(interfaceName string) string {
	// Look for PCI address in /sys/class/net/{interface}/device
	devicePath := filepath.Join("/sys/class/net", interfaceName, "device")
	if _, err := os.Stat(devicePath); err == nil {
		// Read the symlink to get PCI address
		if link, err := os.Readlink(devicePath); err == nil {
			// The link format is typically "../../../0000:31:00.0"
			// We need to extract the PCI address from the end
			parts := strings.Split(link, "/")
			for i := len(parts) - 1; i >= 0; i-- {
				part := parts[i]
				// Check if this looks like a PCI address (contains ":")
				if strings.Contains(part, ":") {
					return part
				}
			}
		}
	}
	return ""
}

// extractVFIndexFromRepresentor extracts VF index from representor properties
func extractVFIndexFromRepresentor(interfaceName, pciAddr string) int {
	// Method 1: Check phys_port_name for VF index (most reliable)
	portPath := filepath.Join("/sys/class/net", interfaceName, "phys_port_name")
	if _, err := os.Stat(portPath); err == nil {
		if portData, err := os.ReadFile(portPath); err == nil {
			portName := strings.TrimSpace(string(portData))
			if strings.Contains(portName, "vf") {
				parts := strings.Split(portName, "vf")
				if len(parts) > 1 {
					if index, err := strconv.Atoi(parts[1]); err == nil {
						return index
					}
				}
			}
		}
	}

	// Method 2: Check for Mellanox representor naming pattern as fallback
	// For Mellanox cards, representors follow the pattern: {pf_interface}pf{vf_index}
	if strings.Contains(interfaceName, "vf") {
		parts := strings.Split(interfaceName, "vf")
		if len(parts) > 1 {
			if index, err := strconv.Atoi(parts[1]); err == nil {
				return index
			}
		}
	}

	// Method 3: Check for legacy representor naming pattern as fallback
	if strings.Contains(interfaceName, "rep") {
		parts := strings.Split(interfaceName, "rep")
		if len(parts) > 1 {
			if index, err := strconv.Atoi(parts[1]); err == nil {
				return index
			}
		}
	}

	return -1
}

// determineRepresentorType determines the type of representor
func determineRepresentorType(interfaceName, pciAddr string) string {
	// Since we only discover representors in switchdev mode, all representors are switchdev type
	// Check for switchdev mode indicators
	switchdevPath := filepath.Join("/sys/class/net", interfaceName, "switchdev")
	if _, err := os.Stat(switchdevPath); err == nil {
		return "switchdev"
	}

	// Check for eswitch mode
	eswitchPath := filepath.Join("/sys/class/net", interfaceName, "eswitch_mode")
	if _, err := os.Stat(eswitchPath); err == nil {
		if data, err := os.ReadFile(eswitchPath); err == nil {
			mode := strings.TrimSpace(string(data))
			if mode == "switchdev" {
				return "switchdev"
			}
		}
	}

	// If we can't determine the mode but this is a representor interface, assume switchdev
	// since we only discover representors in switchdev mode
	if strings.Contains(interfaceName, "vf") || strings.Contains(interfaceName, "rep") {
		return "switchdev"
	}

	// This should not happen since we only discover representors in switchdev mode
	return "switchdev"
}

// getAssociatedVFPCI gets the PCI address of the associated VF
func getAssociatedVFPCI(pfPCI string, vfIndex int) string {
	if vfIndex < 0 {
		return ""
	}
	return fmt.Sprintf("%s-vf%d", pfPCI, vfIndex)
}

// getRepresentorFeatures gets features for a representor interface
func getRepresentorFeatures(interfaceName string) map[string]bool {
	features := make(map[string]bool)

	// Use ethtool to get features
	ethHandleOnce.Do(func() {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			return
		}
	})

	if ethHandle != nil {
		if ethFeatures, err := ethHandle.Features(interfaceName); err == nil {
			for feature, enabled := range ethFeatures {
				features[feature] = enabled
			}
		}
	}

	return features
}

// getRepresentorChannels gets channel information for a representor
func getRepresentorChannels(interfaceName string) map[string]int {
	return getChannelCounts(interfaceName)
}

// getRepresentorRings gets ring buffer information for a representor
func getRepresentorRings(interfaceName string) map[string]int {
	return getRingBufferSizes(interfaceName)
}

// getRepresentorProperties gets properties for a representor
func getRepresentorProperties(interfaceName string) map[string]string {
	properties := make(map[string]string)

	// Get various representor properties from sysfs
	propertyFiles := []string{
		"phys_switch_id",
		"phys_port_name",
		"phys_port_id",
		"switchdev",
	}

	for _, propFile := range propertyFiles {
		propPath := filepath.Join("/sys/class/net", interfaceName, propFile)
		if data, err := os.ReadFile(propPath); err == nil {
			properties[propFile] = strings.TrimSpace(string(data))
		}
	}

	return properties
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

// getInterfaceDriver gets the driver name for a network interface
func getInterfaceDriver(interfaceName string) string {
	// Use ethtool library to get driver name (more accurate for representors)
	ethHandleOnce.Do(func() {
		var err error
		ethHandle, err = ethtool.NewEthtool()
		if err != nil {
			return
		}
	})

	if ethHandle != nil {
		if driver, err := ethHandle.DriverName(interfaceName); err == nil {
			return driver
		}
	}

	// Fallback to sysfs method
	driverPath := filepath.Join("/sys/class/net", interfaceName, "device/driver")
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

// linkRepresentorsToVFs links representors to their associated VFs
func (s *server) linkRepresentorsToVFs(pfPCI string, pfInfo *types.PFInfo) {
	// Iterate through representors and link them to VFs
	for repInterface, repInfo := range pfInfo.Representors {
		if repInfo.AssociatedVF != "" {
			// Find the VF in the cache
			if vfInfo, exists := s.sriovCache.vfs[repInfo.AssociatedVF]; exists {
				// Link the representor to the VF
				vfInfo.Representor = repInfo
				s.logger.WithFields(logrus.Fields{
					"vf":          repInfo.AssociatedVF,
					"representor": repInterface,
					"vf_index":    repInfo.VFIndex,
				}).Debug("linked representor to VF")
			}
		}
	}
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

// supportsEswitchMode determines if a device supports e-switch mode based on vendor and device IDs
func supportsEswitchMode(vendorID, deviceID string) bool {
	// Known vendors that support e-switch mode
	eswitchVendors := map[string]bool{
		"0x15b3": true, // Mellanox Technologies
		"0x8086": true, // Intel Corporation (some devices)
		"0x10df": true, // Emulex Corporation
		"0x1077": true, // QLogic Corp
		"0x14e4": true, // Broadcom Inc
		"0x1924": true, // Solarflare Communications
		"0x19e5": true, // Huawei Technologies Co., Ltd
	}

	// Check if vendor supports e-switch mode
	if !eswitchVendors[vendorID] {
		return false
	}

	// For vendors that support e-switch, check specific device IDs if needed
	switch vendorID {
	case "0x15b3": // Mellanox
		// Most Mellanox devices support e-switch, but some older ones might not
		// For now, assume all Mellanox devices support it
		return true
	case "0x8086": // Intel
		// Intel devices that support e-switch (mostly newer ones)
		eswitchDevices := map[string]bool{
			"0x1572": true, // Intel Ethernet Controller X710
			"0x1583": true, // Intel Ethernet Controller X710
			"0x1584": true, // Intel Ethernet Controller X710
			"0x1585": true, // Intel Ethernet Controller X710
			"0x1586": true, // Intel Ethernet Controller X710
			"0x1587": true, // Intel Ethernet Controller X710
			"0x1588": true, // Intel Ethernet Controller X710
			"0x1589": true, // Intel Ethernet Controller X710
			"0x158a": true, // Intel Ethernet Controller X710
			"0x158b": true, // Intel Ethernet Controller X710
			"0x37d0": true, // Intel Ethernet Controller E810
			"0x37d1": true, // Intel Ethernet Controller E810
			"0x37d2": true, // Intel Ethernet Controller E810
			"0x37d3": true, // Intel Ethernet Controller E810
		}
		return eswitchDevices[deviceID]
	default:
		// For other vendors that support e-switch, assume all devices support it
		return true
	}
}

// supportsRepresentors determines if a device actually supports representors
// This is a more stringent check than supportsEswitchMode
func supportsRepresentors(vendorID, deviceID, interfaceName string) bool {
	// First check if the device supports e-switch mode
	if !supportsEswitchMode(vendorID, deviceID) {
		return false
	}

	// Additional checks for specific vendors and devices
	switch vendorID {
	case "0x15b3": // Mellanox
		// Check for Mellanox representor driver support
		// Look for mlx5e_rep driver in the system
		if hasMellanoxRepresentorDriver() {
			return true
		}
		// Fallback: check if the interface has representor-related sysfs entries
		return hasRepresentorSysfsEntries(interfaceName)
	case "0x8086": // Intel
		// Intel devices that support representors (subset of e-switch devices)
		representorDevices := map[string]bool{
			"0x37d0": true, // Intel Ethernet Controller E810
			"0x37d1": true, // Intel Ethernet Controller E810
			"0x37d2": true, // Intel Ethernet Controller E810
			"0x37d3": true, // Intel Ethernet Controller E810
		}
		if representorDevices[deviceID] {
			return hasRepresentorSysfsEntries(interfaceName)
		}
		return false
	default:
		// For other vendors, check if they have representor sysfs entries
		return hasRepresentorSysfsEntries(interfaceName)
	}
}

// hasMellanoxRepresentorDriver checks if the Mellanox representor driver is loaded
func hasMellanoxRepresentorDriver() bool {
	// Check if mlx5e_rep driver is loaded
	driverPath := "/sys/bus/pci/drivers/mlx5e_rep"
	if _, err := os.Stat(driverPath); err == nil {
		return true
	}
	return false
}

// hasRepresentorSysfsEntries checks if an interface has representor-related sysfs entries
func hasRepresentorSysfsEntries(interfaceName string) bool {
	if interfaceName == "" {
		return false
	}

	// Check for representor-specific sysfs entries
	representorPaths := []string{
		filepath.Join("/sys/class/net", interfaceName, "phys_switch_id"),
		filepath.Join("/sys/class/net", interfaceName, "phys_port_name"),
		filepath.Join("/sys/class/net", interfaceName, "phys_port_id"),
	}

	for _, path := range representorPaths {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}

	return false
}
