package pkg

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/TimRots/gutil-linux/pci"
)

// parsePciDevices is defined as a variable so it can be
// overridden in tests.
var parsePciDevices = pci.ParsePciDevices

// Device holds information about a network device
// parsed from lshw and lspci

type Device struct {
	PCIAddress string
	Name       string
	Driver     string
	Vendor     string
	Product    string
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

// AttachPciInfo enriches devices with data from gutil-linux pci parsing
func AttachPciInfo(devices []Device) ([]Device, error) {
	pciDevs, err := parsePciDevices()
	if err != nil {
		return devices, err
	}
	// index by bus (pci address) to find kernel driver etc
	info := make(map[string]pci.PciDevice)
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
			devices[i] = dev
		}
	}
	return devices, nil
}
