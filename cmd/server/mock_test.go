package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"testing"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func init() {
	lis = bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterSRIOVManagerServer(s, &server{})
	go func() {
		if err := s.Serve(lis); err != nil {
			panic(err)
		}
	}()
}

func bufDialer(context.Context, string) (net.Conn, error) {
	return lis.Dial()
}

func TestServerWithMockData(t *testing.T) {
	// Override PCI parsing with sysfs mock
	pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevices)

	// Create mock lshw data
	mockLshwData := []map[string]interface{}{
		{
			"businfo":     "pci@0000:01:00.0",
			"logicalname": "eth0",
			"vendor":      "Mellanox Technologies",
			"product":     "MT2910 Family [ConnectX-7]",
			"configuration": map[string]interface{}{
				"driver": "mlx5_core",
			},
		},
		{
			"businfo":     "pci@0000:02:00.0",
			"logicalname": "eth1",
			"vendor":      "Intel Corporation",
			"product":     "I350 Gigabit Network Connection",
			"configuration": map[string]interface{}{
				"driver": "igb",
			},
		},
	}

	// Write mock data to temp file
	tmpfile, err := os.CreateTemp("", "mock-lshw-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if err := json.NewEncoder(tmpfile).Encode(mockLshwData); err != nil {
		t.Fatalf("Failed to write mock data: %v", err)
	}
	tmpfile.Close()

	// Parse mock lshw data
	devices, err := pkg.ParseLshwFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to parse mock lshw data: %v", err)
	}

	// Enrich devices
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("Failed to enrich devices: %v", err)
	}

	// Test gRPC server
	srv := &server{devices: devices}
	resp, err := srv.ListDevices(context.Background(), &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}

	if len(resp.Devices) != 2 {
		t.Errorf("Expected 2 devices, got %d", len(resp.Devices))
	}

	// Verify first device (Mellanox)
	if resp.Devices[0].PciAddress != "0000:01:00.0" {
		t.Errorf("Expected PCI address 0000:01:00.0, got %s", resp.Devices[0].PciAddress)
	}
	if resp.Devices[0].Vendor != "Mellanox Technologies" {
		t.Errorf("Expected vendor Mellanox Technologies, got %s", resp.Devices[0].Vendor)
	}
	if resp.Devices[0].Driver != "mlx5_core" {
		t.Errorf("Expected driver mlx5_core, got %s", resp.Devices[0].Driver)
	}

	// Verify second device (Intel)
	if resp.Devices[1].PciAddress != "0000:02:00.0" {
		t.Errorf("Expected PCI address 0000:02:00.0, got %s", resp.Devices[1].PciAddress)
	}
	if resp.Devices[1].Vendor != "Intel Corporation" {
		t.Errorf("Expected vendor Intel Corporation, got %s", resp.Devices[1].Vendor)
	}
	if resp.Devices[1].Driver != "igb" {
		t.Errorf("Expected driver igb, got %s", resp.Devices[1].Driver)
	}
}

func TestServerWithEmptyData(t *testing.T) {
	// Override PCI parsing with empty mock
	pkg.SetParsePciDevices(func() ([]pkg.SysfsPciDevice, error) {
		return []pkg.SysfsPciDevice{}, nil
	})

	// Create empty mock lshw data
	mockLshwData := []map[string]interface{}{}

	// Write mock data to temp file
	tmpfile, err := os.CreateTemp("", "mock-lshw-empty-*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if err := json.NewEncoder(tmpfile).Encode(mockLshwData); err != nil {
		t.Fatalf("Failed to write mock data: %v", err)
	}
	tmpfile.Close()

	// Parse mock lshw data
	devices, err := pkg.ParseLshwFromFile(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to parse mock lshw data: %v", err)
	}

	// Enrich devices
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("Failed to enrich devices: %v", err)
	}

	// Test gRPC server
	srv := &server{devices: devices}
	resp, err := srv.ListDevices(context.Background(), &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}

	if len(resp.Devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(resp.Devices))
	}
}
