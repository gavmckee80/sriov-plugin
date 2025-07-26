package pkg

import (
	"reflect"
	"testing"

	"github.com/TimRots/gutil-linux/pci"
)

func TestParseLshw(t *testing.T) {
	devices, err := ParseLshw("../lshw-network.json")
	if err != nil {
		t.Fatalf("ParseLshw returned error: %v", err)
	}
	if len(devices) != 28 {
		t.Fatalf("expected 28 devices, got %d", len(devices))
	}
	d := devices[0]
	expected := Device{
		PCIAddress: "0000:09:00.0",
		Name:       "enp9s0np0",
		Driver:     "ionic",
		Vendor:     "Pensando Systems",
		Product:    "DSC Ethernet Controller",
	}
	if !reflect.DeepEqual(d, expected) {
		t.Errorf("first device mismatch\nexpected: %#v\nactual:   %#v", expected, d)
	}
}

func TestAttachPciInfo(t *testing.T) {
	// stub parsePciDevices
	old := parsePciDevices
	defer func() { parsePciDevices = old }()
	parsePciDevices = func() ([]pci.PciDevice, error) {
		return []pci.PciDevice{{
			Bus:          "0000:01:00.0",
			KernelDriver: "stubdriver",
			VendorName:   "StubVendor",
			DeviceName:   "StubDevice",
		}}, nil
	}
	devs := []Device{{PCIAddress: "0000:01:00.0"}}
	enriched, err := AttachPciInfo(devs)
	if err != nil {
		t.Fatalf("AttachPciInfo returned error: %v", err)
	}
	got := enriched[0]
	if got.Driver != "stubdriver" || got.Vendor != "StubVendor" || got.Product != "StubDevice" {
		t.Errorf("AttachPciInfo did not populate fields: %#v", got)
	}
}
