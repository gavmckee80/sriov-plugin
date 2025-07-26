package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

// TestServerWithMockData tests the server with mock device data
func TestServerWithMockData(t *testing.T) {
	// Create mock lshw data
	mockLshwData := `[
		{
			"id": "network",
			"class": "network",
			"claimed": true,
			"handle": "PCI:0000:01:00.0",
			"description": "Ethernet interface",
			"product": "MT2910 Family [ConnectX-7]",
			"vendor": "Mellanox Technologies",
			"physid": "0",
			"businfo": "pci@0000:01:00.0",
			"logicalname": "ens1f0np0",
			"configuration": {
				"driver": "mlx5_core"
			}
		},
		{
			"id": "network",
			"class": "network",
			"claimed": true,
			"handle": "PCI:0000:02:00.0",
			"description": "Ethernet interface",
			"product": "DSC Ethernet Controller",
			"vendor": "Pensando Systems",
			"physid": "0",
			"businfo": "pci@0000:02:00.0",
			"logicalname": "enp2s0np0",
			"configuration": {
				"driver": "ionic"
			}
		}
	]`

	tmpfile, err := os.CreateTemp("", "mock-lshw-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(mockLshwData)); err != nil {
		t.Fatalf("failed to write mock data: %v", err)
	}
	tmpfile.Close()

	// Parse devices with mock data
	devices, err := pkg.ParseLshw(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to parse lshw: %v", err)
	}

	// Override PCI parsing with sysfs mock
	pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevices)

	// Enrich devices
	enriched, err := pkg.AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("failed to enrich devices: %v", err)
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}

	grpcServer := grpc.NewServer()
	srv := &server{devices: enriched}
	pb.RegisterSRIOVManagerServer(grpcServer, srv)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test gRPC client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewSRIOVManagerClient(conn)
	resp, err := client.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}

	// Verify response
	if len(resp.Devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(resp.Devices))
	}

	// Check first device (Mellanox)
	device1 := resp.Devices[0]
	if device1.PciAddress != "0000:01:00.0" {
		t.Errorf("expected PCI address 0000:01:00.0, got %s", device1.PciAddress)
	}
	if device1.Name != "ens1f0np0" {
		t.Errorf("expected name ens1f0np0, got %s", device1.Name)
	}
	if device1.Driver != "mlx5_core" {
		t.Errorf("expected driver mlx5_core, got %s", device1.Driver)
	}
	if device1.Vendor != "Mellanox Technologies" {
		t.Errorf("expected vendor Mellanox Technologies, got %s", device1.Vendor)
	}
	if device1.Product != "MT2910 Family [ConnectX-7]" {
		t.Errorf("expected product MT2910 Family [ConnectX-7], got %s", device1.Product)
	}

	// Check second device (Pensando)
	device2 := resp.Devices[1]
	if device2.PciAddress != "0000:02:00.0" {
		t.Errorf("expected PCI address 0000:02:00.0, got %s", device2.PciAddress)
	}
	if device2.Name != "enp2s0np0" {
		t.Errorf("expected name enp2s0np0, got %s", device2.Name)
	}
	if device2.Driver != "ionic" {
		t.Errorf("expected driver ionic, got %s", device2.Driver)
	}
	if device2.Vendor != "Pensando Systems" {
		t.Errorf("expected vendor Pensando Systems, got %s", device2.Vendor)
	}
	if device2.Product != "DSC Ethernet Controller" {
		t.Errorf("expected product DSC Ethernet Controller, got %s", device2.Product)
	}
}

// TestServerWithEmptyData tests the server with no devices
func TestServerWithEmptyData(t *testing.T) {
	// Create empty mock lshw data
	mockLshwData := `[]`

	tmpfile, err := os.CreateTemp("", "mock-lshw-empty-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(mockLshwData)); err != nil {
		t.Fatalf("failed to write mock data: %v", err)
	}
	tmpfile.Close()

	// Parse devices with empty data
	devices, err := pkg.ParseLshw(tmpfile.Name())
	if err != nil {
		t.Fatalf("failed to parse lshw: %v", err)
	}

	// Override PCI parsing with sysfs mock
	pkg.SetParsePciDevices(pkg.MockParseSysfsPciDevices)

	// Enrich devices
	enriched, err := pkg.AttachPciInfo(devices)
	if err != nil {
		t.Fatalf("failed to enrich devices: %v", err)
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}

	grpcServer := grpc.NewServer()
	srv := &server{devices: enriched}
	pb.RegisterSRIOVManagerServer(grpcServer, srv)

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Test gRPC client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewSRIOVManagerClient(conn)
	resp, err := client.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}

	// Verify empty response
	if len(resp.Devices) != 0 {
		t.Fatalf("expected 0 devices, got %d", len(resp.Devices))
	}
}
