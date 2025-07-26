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
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI-X":            "Enable+ Count=128 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
			},
		},
		{
			Bus:          "0000:01:00.1",
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
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI-X":            "Enable+ Count=128 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
			},
		},
		{
			Bus:          "0000:02:00.0",
			KernelDriver: "ionic",
			VendorName:   "Pensando Systems",
			DeviceName:   "DSC Ethernet Controller",
			VendorID:     "1b4b",
			DeviceID:     "1001",
			SubVendorID:  "1b4b",
			SubDeviceID:  "1001",
			Class:        "0200",
			Revision:     "00",
			SRIOVCapable: false,
			SRIOVInfo:    nil,
			Capabilities: map[string]string{
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI-X":            "Enable+ Count=64 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
			},
		},
		{
			Bus:          "0000:03:00.0",
			KernelDriver: "igb",
			VendorName:   "Intel Corporation",
			DeviceName:   "I350 Gigabit Network Connection",
			VendorID:     "8086",
			DeviceID:     "1521",
			SubVendorID:  "8086",
			SubDeviceID:  "1521",
			Class:        "0200",
			Revision:     "01",
			SRIOVCapable: false,
			SRIOVInfo:    nil,
			Capabilities: map[string]string{
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI":              "Enable+ Count=1/1 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
			},
		},
		{
			Bus:          "0000:04:00.0",
			KernelDriver: "i40e",
			VendorName:   "Intel Corporation",
			DeviceName:   "Ethernet Controller X710 for 10GbE SFP+",
			VendorID:     "8086",
			DeviceID:     "1572",
			SubVendorID:  "8086",
			SubDeviceID:  "1572",
			Class:        "0200",
			Revision:     "01",
			SRIOVCapable: true,
			SRIOVInfo: &SRIOVInfo{
				IOVCap:                 "Migration-, Interrupt Message Number: 000",
				IOVCtl:                 "Enable+ Migration- Interrupt- MSE+ ARIHierarchy+",
				IOVSta:                 "Migration-",
				InitialVFs:             32,
				TotalVFs:               32,
				NumberOfVFs:            8,
				FunctionDependencyLink: "00",
				VFOffset:               1,
				VFStride:               1,
				VFDeviceID:             "1573",
				SupportedPageSize:      "000007ff",
				SystemPageSize:         "00000001",
				Region0:                "Memory at 000006418c000000 (64-bit, prefetchable)",
				VFMigration:            "offset: 00000000, BIR: 0",
			},
			Capabilities: map[string]string{
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI-X":            "Enable+ Count=128 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
			},
		},
		{
			Bus:          "0000:05:00.0",
			KernelDriver: "bnxt_en",
			VendorName:   "Broadcom Inc.",
			DeviceName:   "NetXtreme-E BCM57140",
			VendorID:     "14e4",
			DeviceID:     "1654",
			SubVendorID:  "14e4",
			SubDeviceID:  "1654",
			Class:        "0200",
			Revision:     "01",
			SRIOVCapable: false,
			SRIOVInfo:    nil,
			Capabilities: map[string]string{
				"Power Management": "PME#- DSI- D1- D2- AuxCurrent=0mA PME(D0-,D1-,D2-,D3hot-,D3cold-)",
				"MSI":              "Enable+ Count=1/1 Masked-",
				"PCI Express":      "MaxPayload 512 bytes, MaxReadReq 512 bytes",
				"VPD":              "Access: Read/Write",
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
