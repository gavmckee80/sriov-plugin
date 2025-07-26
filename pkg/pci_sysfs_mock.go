package pkg

import "strings"

// SRIOVInfo holds SR-IOV specific information
type SRIOVInfo struct {
	IOVCap                 string
	IOVCtl                 string
	IOVSta                 string
	InitialVFs             int
	TotalVFs               int
	NumberOfVFs            int
	FunctionDependencyLink string
	VFOffset               int
	VFStride               int
	VFDeviceID             string
	SupportedPageSize      string
	SystemPageSize         string
	Region0                string
	VFMigration            string
}

// MockParseSysfsPciDevices returns mock sysfs PCI devices for testing
func MockParseSysfsPciDevices() ([]SysfsPciDevice, error) {
	// Create mock sysfs devices directly
	sysfsDevices := []SysfsPciDevice{
		{
			Bus:          "0000:01:00.0",
			KernelDriver: "mlx5_core",
			VendorName:   "Mellanox Technologies",
			DeviceName:   "MT2910 Family [ConnectX-7]",
			VendorID:     "15b3",
			DeviceID:     "101e",
			SubVendorID:  "15b3",
			SubDeviceID:  "101e",
			Class:        "0200",
			Revision:     "00",
			SRIOVCapable: true,
			NUMANode:     0,
			NUMADistance: map[int]int{0: 10, 1: 20},
			SRIOVInfo: &SRIOVInfo{
				IOVCap:                 "Migration-, Interrupt Message Number: 000",
				IOVCtl:                 "Enable+ Migration- Interrupt- MSE+ ARIHierarchy+",
				IOVSta:                 "Migration-",
				InitialVFs:             16,
				TotalVFs:               16,
				NumberOfVFs:            4,
				FunctionDependencyLink: "00",
				VFOffset:               2,
				VFStride:               1,
				VFDeviceID:             "101e",
				SupportedPageSize:      "000007ff",
				SystemPageSize:         "00000001",
				Region0:                "Memory at 000006418c000000 (64-bit, prefetchable)",
				VFMigration:            "offset: 00000000, BIR: 0",
			},
			Capabilities: map[string]string{
				"msix":  "Enable+ Count=128 Masked-",
				"pciex": "MSI: Enable- Count=1/1 Maskable- 64bit+",
				"pm":    "D3hot D3cold",
				"sriov": "Enable+ Migration- Interrupt- MSE+ ARIHierarchy+",
			},
			DetailedCapabilities: map[string]DetailedCapability{
				"MSI-X": {
					ID:          "11",
					Name:        "MSI-X",
					Version:     "1.0",
					Status:      "Enabled",
					Description: "Message Signaled Interrupts with Extended Capability",
					Parameters: map[string]string{
						"count":  "128",
						"masked": "No",
					},
				},
				"PCIe": {
					ID:          "10",
					Name:        "PCIe",
					Version:     "2.0",
					Status:      "Active",
					Description: "PCI Express",
					Parameters: map[string]string{
						"max_speed":     "8.0 GT/s",
						"current_speed": "8.0 GT/s",
					},
				},
				"PM": {
					ID:          "01",
					Name:        "PM",
					Version:     "1.2",
					Status:      "D3hot D3cold",
					Description: "Power Management",
					Parameters: map[string]string{
						"version":      "1.2",
						"capabilities": "D3hot D3cold",
					},
				},
				"SR-IOV": {
					ID:          "10",
					Name:        "SR-IOV",
					Version:     "1.1",
					Status:      "Enabled",
					Description: "Single Root I/O Virtualization",
					Parameters: map[string]string{
						"total_vfs": "16",
						"num_vfs":   "4",
						"vf_offset": "2",
						"vf_stride": "1",
					},
				},
			},
		},
		{
			Bus:          "0000:02:00.0",
			KernelDriver: "pensando_dsc",
			VendorName:   "Pensando Systems",
			DeviceName:   "DSC Ethernet Controller",
			VendorID:     "1dd8",
			DeviceID:     "1003",
			SubVendorID:  "1dd8",
			SubDeviceID:  "1003",
			Class:        "0200",
			Revision:     "00",
			SRIOVCapable: true,
			NUMANode:     1,
			NUMADistance: map[int]int{0: 20, 1: 10},
			SRIOVInfo: &SRIOVInfo{
				IOVCap:                 "Migration-, Interrupt Message Number: 000",
				IOVCtl:                 "Enable+ Migration- Interrupt- MSE+ ARIHierarchy+",
				IOVSta:                 "Migration-",
				InitialVFs:             8,
				TotalVFs:               8,
				NumberOfVFs:            1,
				FunctionDependencyLink: "00",
				VFOffset:               1,
				VFStride:               1,
				VFDeviceID:             "1003",
				SupportedPageSize:      "000007ff",
				SystemPageSize:         "00000001",
				Region0:                "Memory at 000006418c000000 (64-bit, prefetchable)",
				VFMigration:            "offset: 00000000, BIR: 0",
			},
			Capabilities: map[string]string{
				"msix":  "Enable+ Count=64 Masked-",
				"pciex": "MSI: Enable- Count=1/1 Maskable- 64bit+",
				"pm":    "D3hot D3cold",
				"sriov": "Enable+ Migration- Interrupt- MSE+ ARIHierarchy+",
			},
			DetailedCapabilities: map[string]DetailedCapability{
				"MSI-X": {
					ID:          "11",
					Name:        "MSI-X",
					Version:     "1.0",
					Status:      "Enabled",
					Description: "Message Signaled Interrupts with Extended Capability",
					Parameters: map[string]string{
						"count":  "64",
						"masked": "No",
					},
				},
				"PCIe": {
					ID:          "10",
					Name:        "PCIe",
					Version:     "2.0",
					Status:      "Active",
					Description: "PCI Express",
					Parameters: map[string]string{
						"max_speed":     "8.0 GT/s",
						"current_speed": "8.0 GT/s",
					},
				},
				"PM": {
					ID:          "01",
					Name:        "PM",
					Version:     "1.2",
					Status:      "D3hot D3cold",
					Description: "Power Management",
					Parameters: map[string]string{
						"version":      "1.2",
						"capabilities": "D3hot D3cold",
					},
				},
				"SR-IOV": {
					ID:          "10",
					Name:        "SR-IOV",
					Version:     "1.1",
					Status:      "Enabled",
					Description: "Single Root I/O Virtualization",
					Parameters: map[string]string{
						"total_vfs": "8",
						"num_vfs":   "1",
						"vf_offset": "1",
						"vf_stride": "1",
					},
				},
			},
		},
	}

	return sysfsDevices, nil
}

// MockParseSysfsPciDevicesWithSRIOV returns only SR-IOV capable sysfs devices
func MockParseSysfsPciDevicesWithSRIOV() ([]SysfsPciDevice, error) {
	allDevices, err := MockParseSysfsPciDevices()
	if err != nil {
		return nil, err
	}

	var sriovDevices []SysfsPciDevice
	for _, device := range allDevices {
		if device.SRIOVCapable {
			sriovDevices = append(sriovDevices, device)
		}
	}

	return sriovDevices, nil
}

// MockParseSysfsPciDevicesByVendor returns sysfs devices filtered by vendor
func MockParseSysfsPciDevicesByVendor(vendorFilter string) ([]SysfsPciDevice, error) {
	allDevices, err := MockParseSysfsPciDevices()
	if err != nil {
		return nil, err
	}

	var filtered []SysfsPciDevice
	for _, device := range allDevices {
		if vendorFilter == "" || containsIgnoreCase(device.VendorName, vendorFilter) {
			filtered = append(filtered, device)
		}
	}

	return filtered, nil
}

// MockParseSysfsPciDevicesByDriver returns sysfs devices filtered by driver
func MockParseSysfsPciDevicesByDriver(driverFilter string) ([]SysfsPciDevice, error) {
	allDevices, err := MockParseSysfsPciDevices()
	if err != nil {
		return nil, err
	}

	var filtered []SysfsPciDevice
	for _, device := range allDevices {
		if driverFilter == "" || containsIgnoreCase(device.KernelDriver, driverFilter) {
			filtered = append(filtered, device)
		}
	}

	return filtered, nil
}

// containsIgnoreCase checks if a string contains another string (case insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(strings.ToLower(s) == strings.ToLower(substr) ||
			strings.Contains(strings.ToLower(s), strings.ToLower(substr)))
}
