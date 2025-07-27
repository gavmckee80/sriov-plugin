package sriov

import (
	"testing"
)

func TestNewDeviceManager(t *testing.T) {
	dm := NewDeviceManager()
	if dm == nil {
		t.Error("NewDeviceManager() returned nil")
	}
}

func TestGetSRIOVDevices(t *testing.T) {
	dm := NewDeviceManager()

	// This test will only work if there are actual SR-IOV devices on the system
	// In a real test environment, you might want to mock the filesystem
	pfs, err := dm.GetSRIOVDevices()

	// We can't guarantee there are SR-IOV devices on the test system,
	// so we just check that the function doesn't panic and returns a valid result
	if err != nil {
		// It's okay if there are no SR-IOV devices
		t.Logf("GetSRIOVDevices() returned error (expected if no SR-IOV devices): %v", err)
		return
	}

	if pfs == nil {
		t.Log("GetSRIOVDevices() returned nil slice (expected if no SR-IOV devices)")
		return
	}

	// If we have PFs, check their structure
	for i, pf := range pfs {
		if pf.PFPCI == "" {
			t.Errorf("PF %d has empty PCI address", i)
		}

		// Check VFs structure
		for j, vf := range pf.VFs {
			if vf.VFPCI == "" {
				t.Errorf("VF %d in PF %d has empty PCI address", j, i)
			}
			if vf.PFPCI != pf.PFPCI {
				t.Errorf("VF %d in PF %d has mismatched PF PCI: got %s, want %s",
					j, i, vf.PFPCI, pf.PFPCI)
			}
		}
	}
}

func TestVFStructure(t *testing.T) {
	vf := VF{
		VFPCI:       "0000:01:00.0-vf0",
		PFPCI:       "0000:01:00.0",
		Interface:   "eth0",
		Representor: "eth0_rep",
		NUMANode:    "0",
		LinkState:   "up",
		LinkSpeed:   "10G",
		Allocated:   false,
		Masked:      false,
		Features:    map[string]bool{"rx_checksum": true},
		RxRings:     4,
		TxRings:     4,
		RxMax:       1024,
		TxMax:       1024,
		RxChannels:  4,
		TxChannels:  4,
		LastUpdated: "2023-01-01T00:00:00Z",
		Driver:      "mlx5_core",
		Mode:        "legacy",
		State:       "available",
		Pool:        "test-pool",
	}

	if vf.VFPCI != "0000:01:00.0-vf0" {
		t.Errorf("Expected VFPCI '0000:01:00.0-vf0', got '%s'", vf.VFPCI)
	}

	if vf.PFPCI != "0000:01:00.0" {
		t.Errorf("Expected PFPCI '0000:01:00.0', got '%s'", vf.PFPCI)
	}

	if vf.Interface != "eth0" {
		t.Errorf("Expected Interface 'eth0', got '%s'", vf.Interface)
	}

	if !vf.Features["rx_checksum"] {
		t.Error("Expected rx_checksum feature to be true")
	}

	if vf.RxRings != 4 {
		t.Errorf("Expected RxRings 4, got %d", vf.RxRings)
	}

	if vf.Pool != "test-pool" {
		t.Errorf("Expected Pool 'test-pool', got '%s'", vf.Pool)
	}
}

func TestPFStructure(t *testing.T) {
	vf := VF{
		VFPCI:     "0000:01:00.0-vf0",
		PFPCI:     "0000:01:00.0",
		Interface: "eth0",
		State:     "available",
		Pool:      "test-pool",
	}

	pf := PF{
		PFPCI:     "0000:01:00.0",
		Interface: "eth0",
		VFs:       []VF{vf},
		Pool:      "test-pool",
	}

	if pf.PFPCI != "0000:01:00.0" {
		t.Errorf("Expected PFPCI '0000:01:00.0', got '%s'", pf.PFPCI)
	}

	if pf.Interface != "eth0" {
		t.Errorf("Expected Interface 'eth0', got '%s'", pf.Interface)
	}

	if len(pf.VFs) != 1 {
		t.Errorf("Expected 1 VF, got %d", len(pf.VFs))
	}

	if pf.VFs[0].VFPCI != "0000:01:00.0-vf0" {
		t.Errorf("Expected VF PCI '0000:01:00.0-vf0', got '%s'", pf.VFs[0].VFPCI)
	}

	if pf.Pool != "test-pool" {
		t.Errorf("Expected Pool 'test-pool', got '%s'", pf.Pool)
	}
}
