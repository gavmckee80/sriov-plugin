package main

import (
	"context"
	"testing"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
)

func TestMain(t *testing.T) {
	// Test basic parsing
	devs, err := pkg.ParseLshwFromFile("../../lshw-network.json")
	if err != nil {
		t.Skipf("Skipping test - no lshw file: %v", err)
	}

	if len(devs) == 0 {
		t.Skip("Skipping test - no devices found")
	}

	// Test enrichment
	devs, err = pkg.AttachPciInfo(devs)
	if err != nil {
		t.Fatalf("Failed to enrich devices: %v", err)
	}

	// Test gRPC response
	srv := &server{devices: devs}
	resp, err := srv.ListDevices(context.Background(), &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("Failed to list devices: %v", err)
	}

	if len(resp.Devices) != len(devs) {
		t.Errorf("Expected %d devices, got %d", len(devs), len(resp.Devices))
	}
}
