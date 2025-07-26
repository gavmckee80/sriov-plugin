package pkg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// VendorDatabase holds parsed PCI vendor and device information
type VendorDatabase struct {
	Vendors map[string]VendorInfo
}

// VendorInfo holds vendor and device information
type VendorInfo struct {
	Name    string
	Devices map[string]DeviceInfo
}

// DeviceInfo holds device information
type DeviceInfo struct {
	Name string
}

// EnhancedPciDevice holds comprehensive PCI device information (for compatibility)
type EnhancedPciDevice struct {
	Bus          string
	KernelDriver string
	VendorName   string
	DeviceName   string
	VendorID     string
	DeviceID     string
	SubVendorID  string
	SubDeviceID  string
	Class        string
	SubClass     string
	ProgIF       string
	Revision     string
	// SR-IOV specific fields
	SRIOVCapable bool
	SRIOVInfo    *SRIOVInfo
	// Additional capabilities
	Capabilities map[string]string
}

// SysfsPciDevice holds PCI device information from sysfs
type SysfsPciDevice struct {
	Bus          string
	KernelDriver string
	VendorName   string
	DeviceName   string
	VendorID     string
	DeviceID     string
	SubVendorID  string
	SubDeviceID  string
	Class        string
	SubClass     string
	ProgIF       string
	Revision     string
	// SR-IOV specific fields
	SRIOVCapable bool
	SRIOVInfo    *SRIOVInfo
	// Additional capabilities
	Capabilities map[string]string
}

// loadVendorDatabase loads and parses the PCI vendor database
func loadVendorDatabase() (*VendorDatabase, error) {
	db := &VendorDatabase{
		Vendors: make(map[string]VendorInfo),
	}

	// Try to load from system location first
	paths := []string{
		"/usr/share/hwdata/pci.ids",
		"/usr/share/pci.ids",
		"/usr/share/misc/pci.ids",
	}

	var pciIDsPath string
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			pciIDsPath = path
			break
		}
	}

	if pciIDsPath == "" {
		// If not found, create a minimal database with common vendors
		return createMinimalVendorDB(), nil
	}

	file, err := os.Open(pciIDsPath)
	if err != nil {
		return createMinimalVendorDB(), nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentVendor *VendorInfo
	var currentDevice *DeviceInfo

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse vendor line (format: 8086  Intel Corporation)
		if match := regexp.MustCompile(`^([0-9a-f]{4})\s+(.+)$`).FindStringSubmatch(line); len(match) > 2 {
			if currentVendor != nil {
				db.Vendors[currentVendor.Name] = *currentVendor
			}
			currentVendor = &VendorInfo{
				Name:    match[1],
				Devices: make(map[string]DeviceInfo),
			}
			currentDevice = nil
		}

		// Parse device line (format: 	1572  Ethernet Controller X710 for 10GbE SFP+)
		if strings.HasPrefix(line, "\t") && currentVendor != nil {
			if match := regexp.MustCompile(`^\t([0-9a-f]{4})\s+(.+)$`).FindStringSubmatch(line); len(match) > 2 {
				currentDevice = &DeviceInfo{
					Name: match[2],
				}
				currentVendor.Devices[match[1]] = *currentDevice
			}
		}
	}

	// Add the last vendor
	if currentVendor != nil {
		db.Vendors[currentVendor.Name] = *currentVendor
	}

	return db, scanner.Err()
}

// createMinimalVendorDB creates a minimal vendor database with common vendors
func createMinimalVendorDB() *VendorDatabase {
	db := &VendorDatabase{
		Vendors: make(map[string]VendorInfo),
	}

	// Add common vendors
	vendors := map[string]string{
		"8086": "Intel Corporation",
		"15b3": "Mellanox Technologies",
		"1b4b": "Pensando Systems",
		"1d0f": "Amazon.com, Inc.",
		"10ee": "Xilinx Corporation",
		"14e4": "Broadcom Inc.",
		"1969": "Qualcomm Atheros",
		"10de": "NVIDIA Corporation",
	}

	for vendorID, vendorName := range vendors {
		db.Vendors[vendorID] = VendorInfo{
			Name:    vendorName,
			Devices: make(map[string]DeviceInfo),
		}
	}

	return db
}

// enrichDeviceWithVendorDB enriches device information with vendor database data
func enrichDeviceWithVendorDB(device *SysfsPciDevice, db *VendorDatabase) {
	// Look up vendor name
	if vendor, exists := db.Vendors[device.VendorID]; exists {
		device.VendorName = vendor.Name
	}

	// Look up device name
	if vendor, exists := db.Vendors[device.VendorID]; exists {
		if dev, exists := vendor.Devices[device.DeviceID]; exists {
			device.DeviceName = dev.Name
		}
	}
}

// ParseSysfsPciDevices parses PCI device information from /sys/bus/pci/devices
func ParseSysfsPciDevices() ([]SysfsPciDevice, error) {
	// Load vendor database for name resolution
	vendorDB, err := loadVendorDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to load vendor database: %v", err)
	}

	var devices []SysfsPciDevice
	sysfsPath := "/sys/bus/pci/devices"

	// Walk through all PCI devices in sysfs
	err = filepath.Walk(sysfsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process directories that look like PCI addresses
		if !info.IsDir() || !isPciAddress(info.Name()) {
			return nil
		}

		device, err := parseSysfsPciDevice(path, info.Name(), vendorDB)
		if err != nil {
			// Log error but continue with other devices
			fmt.Printf("Warning: failed to parse device %s: %v\n", info.Name(), err)
			return nil
		}

		devices = append(devices, device)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk sysfs: %v", err)
	}

	return devices, nil
}

// parseSysfsPciDevice parses a single PCI device from sysfs
func parseSysfsPciDevice(devicePath, deviceName string, vendorDB *VendorDatabase) (SysfsPciDevice, error) {
	device := SysfsPciDevice{
		Bus:          deviceName,
		Capabilities: make(map[string]string),
	}

	// Parse vendor and device IDs
	if err := parseDeviceIds(devicePath, &device); err != nil {
		return device, fmt.Errorf("failed to parse device IDs: %v", err)
	}

	// Parse class information
	if err := parseDeviceClass(devicePath, &device); err != nil {
		return device, fmt.Errorf("failed to parse device class: %v", err)
	}

	// Parse kernel driver
	if err := parseKernelDriver(devicePath, &device); err != nil {
		// Driver might not be loaded, continue
	}

	// Parse SR-IOV information
	if err := parseSysfsSRIOVInfo(devicePath, &device); err != nil {
		// SR-IOV might not be available, continue
	}

	// Parse PCI capabilities
	if err := parsePciCapabilities(devicePath, &device); err != nil {
		// Capabilities might not be accessible, continue
	}

	// Enrich with vendor database information
	enrichSysfsDeviceWithVendorDB(&device, vendorDB)

	return device, nil
}

// isPciAddress checks if a string looks like a PCI address
func isPciAddress(name string) bool {
	// PCI addresses are in format: dddd:dd:dd.d
	// Example: 0000:01:00.0
	if len(name) != 12 {
		return false
	}

	// Check format: dddd:dd:dd.d
	if name[4] != ':' || name[7] != ':' || name[10] != '.' {
		return false
	}

	// Check that all other characters are hex digits
	for i, c := range name {
		if i == 4 || i == 7 || i == 10 {
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}

	return true
}

// parseDeviceIds parses vendor and device IDs from sysfs
func parseDeviceIds(devicePath string, device *SysfsPciDevice) error {
	// Read vendor ID
	vendorPath := filepath.Join(devicePath, "vendor")
	vendorData, err := os.ReadFile(vendorPath)
	if err != nil {
		return err
	}
	device.VendorID = strings.TrimSpace(string(vendorData))

	// Read device ID
	devicePathFile := filepath.Join(devicePath, "device")
	deviceData, err := os.ReadFile(devicePathFile)
	if err != nil {
		return err
	}
	device.DeviceID = strings.TrimSpace(string(deviceData))

	// Read subsystem vendor ID (optional)
	subVendorPath := filepath.Join(devicePath, "subsystem_vendor")
	if subVendorData, err := os.ReadFile(subVendorPath); err == nil {
		device.SubVendorID = strings.TrimSpace(string(subVendorData))
	}

	// Read subsystem device ID (optional)
	subDevicePath := filepath.Join(devicePath, "subsystem_device")
	if subDeviceData, err := os.ReadFile(subDevicePath); err == nil {
		device.SubDeviceID = strings.TrimSpace(string(subDeviceData))
	}

	// Read revision (optional)
	revisionPath := filepath.Join(devicePath, "revision")
	if revisionData, err := os.ReadFile(revisionPath); err == nil {
		device.Revision = strings.TrimSpace(string(revisionData))
	}

	return nil
}

// parseDeviceClass parses device class information from sysfs
func parseDeviceClass(devicePath string, device *SysfsPciDevice) error {
	// Read class
	classPath := filepath.Join(devicePath, "class")
	classData, err := os.ReadFile(classPath)
	if err != nil {
		return err
	}

	classStr := strings.TrimSpace(string(classData))
	if len(classStr) >= 6 {
		device.Class = classStr[2:6] // Skip "0x" prefix
		if len(classStr) >= 8 {
			device.SubClass = classStr[6:8]
		}
		if len(classStr) >= 10 {
			device.ProgIF = classStr[8:10]
		}
	}

	return nil
}

// parseKernelDriver parses kernel driver information from sysfs
func parseKernelDriver(devicePath string, device *SysfsPciDevice) error {
	// Check if driver is loaded
	driverPath := filepath.Join(devicePath, "driver")
	if driverLink, err := os.Readlink(driverPath); err == nil {
		// Extract driver name from symlink
		driverName := filepath.Base(driverLink)
		device.KernelDriver = driverName
	}
	return nil
}

// parseSysfsSRIOVInfo parses SR-IOV information from sysfs
func parseSysfsSRIOVInfo(devicePath string, device *SysfsPciDevice) error {
	// Check if SR-IOV capability exists
	sriovPath := filepath.Join(devicePath, "sriov_totalvfs")
	if _, err := os.Stat(sriovPath); os.IsNotExist(err) {
		// SR-IOV not available
		return nil
	}

	device.SRIOVCapable = true
	device.SRIOVInfo = &SRIOVInfo{}

	// Parse SR-IOV parameters
	if err := parseSRIOVParameters(devicePath, device.SRIOVInfo); err != nil {
		return err
	}

	return nil
}

// parseSRIOVParameters parses SR-IOV specific parameters from sysfs
func parseSRIOVParameters(devicePath string, sriov *SRIOVInfo) error {
	// Parse total VFs
	if totalVFsData, err := os.ReadFile(filepath.Join(devicePath, "sriov_totalvfs")); err == nil {
		if totalVFs, err := strconv.Atoi(strings.TrimSpace(string(totalVFsData))); err == nil {
			sriov.TotalVFs = totalVFs
		}
	}

	// Parse number of VFs
	if numVFsData, err := os.ReadFile(filepath.Join(devicePath, "sriov_numvfs")); err == nil {
		if numVFs, err := strconv.Atoi(strings.TrimSpace(string(numVFsData))); err == nil {
			sriov.NumberOfVFs = numVFs
		}
	}

	// Parse VF device ID
	if vfDeviceIDData, err := os.ReadFile(filepath.Join(devicePath, "sriov_vf_device")); err == nil {
		sriov.VFDeviceID = strings.TrimSpace(string(vfDeviceIDData))
	}

	// Parse VF vendor ID
	if vfVendorIDData, err := os.ReadFile(filepath.Join(devicePath, "sriov_vf_vendor")); err == nil {
		sriov.VFDeviceID = strings.TrimSpace(string(vfVendorIDData))
	}

	// Parse VF offset
	if vfOffsetData, err := os.ReadFile(filepath.Join(devicePath, "sriov_vf_offset")); err == nil {
		if vfOffset, err := strconv.Atoi(strings.TrimSpace(string(vfOffsetData))); err == nil {
			sriov.VFOffset = vfOffset
		}
	}

	// Parse VF stride
	if vfStrideData, err := os.ReadFile(filepath.Join(devicePath, "sriov_vf_stride")); err == nil {
		if vfStride, err := strconv.Atoi(strings.TrimSpace(string(vfStrideData))); err == nil {
			sriov.VFStride = vfStride
		}
	}

	return nil
}

// parsePciCapabilities parses PCI capabilities from sysfs
func parsePciCapabilities(devicePath string, device *SysfsPciDevice) error {
	// Parse MSI-X capabilities
	if err := parseMSIXCapability(devicePath, device); err != nil {
		// MSI-X might not be available, continue
	}

	// Parse PCI Express capabilities
	if err := parsePCIExpressCapability(devicePath, device); err != nil {
		// PCIe might not be available, continue
	}

	// Parse Power Management capabilities
	if err := parsePowerManagementCapability(devicePath, device); err != nil {
		// PM might not be available, continue
	}

	return nil
}

// parseMSIXCapability parses MSI-X capability information
func parseMSIXCapability(devicePath string, device *SysfsPciDevice) error {
	msixPath := filepath.Join(devicePath, "msi_irqs")
	if _, err := os.Stat(msixPath); os.IsNotExist(err) {
		return fmt.Errorf("MSI-X not available")
	}

	// Read MSI-X information
	if msixData, err := os.ReadFile(msixPath); err == nil {
		device.Capabilities["MSI-X"] = fmt.Sprintf("Available: %s", strings.TrimSpace(string(msixData)))
	}

	return nil
}

// parsePCIExpressCapability parses PCI Express capability information
func parsePCIExpressCapability(devicePath string, device *SysfsPciDevice) error {
	pciePath := filepath.Join(devicePath, "pcie_cap")
	if _, err := os.Stat(pciePath); os.IsNotExist(err) {
		return fmt.Errorf("PCIe capability not available")
	}

	// Read PCIe link status
	linkStatusPath := filepath.Join(devicePath, "pcie_link_status")
	if linkData, err := os.ReadFile(linkStatusPath); err == nil {
		device.Capabilities["PCI Express"] = fmt.Sprintf("Link Status: %s", strings.TrimSpace(string(linkData)))
	}

	return nil
}

// parsePowerManagementCapability parses Power Management capability information
func parsePowerManagementCapability(devicePath string, device *SysfsPciDevice) error {
	pmPath := filepath.Join(devicePath, "power")
	if _, err := os.Stat(pmPath); os.IsNotExist(err) {
		return fmt.Errorf("Power management not available")
	}

	// Read power state
	powerStatePath := filepath.Join(pmPath, "runtime_status")
	if powerData, err := os.ReadFile(powerStatePath); err == nil {
		device.Capabilities["Power Management"] = fmt.Sprintf("Status: %s", strings.TrimSpace(string(powerData)))
	}

	return nil
}

// enrichSysfsDeviceWithVendorDB enriches sysfs device information with vendor database data
func enrichSysfsDeviceWithVendorDB(device *SysfsPciDevice, db *VendorDatabase) {
	// Look up vendor name
	if vendor, exists := db.Vendors[device.VendorID]; exists {
		device.VendorName = vendor.Name
	}

	// Look up device name
	if vendor, exists := db.Vendors[device.VendorID]; exists {
		if dev, exists := vendor.Devices[device.DeviceID]; exists {
			device.DeviceName = dev.Name
		}
	}
}

// Convert SysfsPciDevice to EnhancedPciDevice for compatibility
func (s *SysfsPciDevice) ToEnhancedPciDevice() EnhancedPciDevice {
	return EnhancedPciDevice{
		Bus:          s.Bus,
		KernelDriver: s.KernelDriver,
		VendorName:   s.VendorName,
		DeviceName:   s.DeviceName,
		VendorID:     s.VendorID,
		DeviceID:     s.DeviceID,
		SubVendorID:  s.SubVendorID,
		SubDeviceID:  s.SubDeviceID,
		Class:        s.Class,
		SubClass:     s.SubClass,
		ProgIF:       s.ProgIF,
		Revision:     s.Revision,
		SRIOVCapable: s.SRIOVCapable,
		SRIOVInfo:    s.SRIOVInfo,
		Capabilities: s.Capabilities,
	}
}
