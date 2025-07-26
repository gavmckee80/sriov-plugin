package pkg

import (
	"encoding/json"
	"os"
	"strings"
)

// parsePciDevices is defined as a variable so it can be
// overridden in tests.
// Default to sysfs implementation for better performance and reliability
var parsePciDevices = ParseSysfsPciDevices

// SetParsePciDevices allows overriding the parsePciDevices function for testing
func SetParsePciDevices(fn func() ([]SysfsPciDevice, error)) {
	parsePciDevices = fn
}

// Device holds information about a network device
// parsed from lshw and lspci

type Device struct {
	PCIAddress string
	Name       string
	Driver     string
	Vendor     string
	Product    string
	// Enhanced fields for SR-IOV information
	SRIOVCapable bool
	SRIOVInfo    *SRIOVInfo
}

// ParseLshw parses a lshw -class network -json output file
func ParseLshw(path string) ([]Device, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var raw []map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}

	var devices []Device
	for _, item := range raw {
		businfo, _ := item["businfo"].(string)
		// businfo is like pci@0000:09:00.0
		pciAddr := strings.TrimPrefix(businfo, "pci@")
		conf, _ := item["configuration"].(map[string]any)
		driver, _ := conf["driver"].(string)
		logicalName, _ := item["logicalname"].(string)
		vendor, _ := item["vendor"].(string)
		product, _ := item["product"].(string)

		devices = append(devices, Device{
			PCIAddress: pciAddr,
			Name:       logicalName,
			Driver:     driver,
			Vendor:     vendor,
			Product:    product,
		})
	}
	return devices, nil
}

// AttachPciInfo enriches devices with data from enhanced PCI parsing
func AttachPciInfo(devices []Device) ([]Device, error) {
	pciDevs, err := parsePciDevices()
	if err != nil {
		return devices, err
	}
	// index by bus (pci address) to find kernel driver etc
	info := make(map[string]SysfsPciDevice)
	for _, d := range pciDevs {
		info[d.Bus] = d
	}
	for i, dev := range devices {
		if p, ok := info[dev.PCIAddress]; ok {
			if dev.Driver == "" {
				dev.Driver = p.KernelDriver
			}
			if dev.Vendor == "" {
				dev.Vendor = p.VendorName
			}
			if dev.Product == "" {
				dev.Product = p.DeviceName
			}
			// Add enhanced SR-IOV information
			dev.SRIOVCapable = p.SRIOVCapable
			dev.SRIOVInfo = p.SRIOVInfo
			devices[i] = dev
		}
	}
	return devices, nil
}
