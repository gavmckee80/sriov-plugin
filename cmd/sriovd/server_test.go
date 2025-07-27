package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	pb "sriov-plugin/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func setupTestServer(t *testing.T) (*server, *grpc.ClientConn) {
	// Create a temporary config file for testing
	configContent := `
pools:
  - name: "test-pool"
    pf_pci: "0000:01:00.0"
    vf_range: "0-3"
    mask: false
    required_features: ["rx_checksum"]
    numa: "0"
  - name: "reserved-pool"
    pf_pci: "0000:01:00.0"
    vf_range: "4-5"
    mask: true
    mask_reason: "Reserved for testing"
    required_features: []
    numa: "0"
`
	tmpfile, err := os.CreateTemp("", "test-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(configContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	// Create server
	s := &server{
		allocated:  make(map[string]bool),
		masked:     make(map[string]bool),
		maskReason: make(map[string]string),
		allowedPFs: make(map[string]bool),
		cfgPath:    tmpfile.Name(),
	}
	s.reloadConfig()

	// Create gRPC server with bufconn
	lis = bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	pb.RegisterSriovDeviceManagerServer(grpcServer, s)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Errorf("failed to serve: %v", err)
		}
	}()

	// Create client connection
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	return s, conn
}

func TestListDevices(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.ListDevices(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("ListDevices failed: %v", err)
	}

	if len(resp.Pfs) == 0 {
		t.Error("Expected at least one PF")
	}

	// Check that we have the expected pools
	foundTestPool := false
	foundReservedPool := false
	for _, pf := range resp.Pfs {
		for _, vf := range pf.Vfs {
			if vf.Pool == "test-pool" {
				foundTestPool = true
			}
			if vf.Pool == "reserved-pool" {
				foundReservedPool = true
			}
		}
	}

	if !foundTestPool {
		t.Error("Expected to find test-pool")
	}
	if !foundReservedPool {
		t.Error("Expected to find reserved-pool")
	}
}

func TestGetStatus(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.GetStatus(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if len(resp.Pools) == 0 {
		t.Error("Expected at least one pool")
	}

	// Check that we have the expected pools
	foundTestPool := false
	foundReservedPool := false
	for _, pool := range resp.Pools {
		if pool.Name == "test-pool" {
			foundTestPool = true
			if pool.Total != 4 {
				t.Errorf("Expected test-pool to have 4 total VFs, got %d", pool.Total)
			}
			if pool.Masked != 0 {
				t.Errorf("Expected test-pool to have 0 masked VFs, got %d", pool.Masked)
			}
		}
		if pool.Name == "reserved-pool" {
			foundReservedPool = true
			if pool.Total != 2 {
				t.Errorf("Expected reserved-pool to have 2 total VFs, got %d", pool.Total)
			}
			if pool.Masked != 2 {
				t.Errorf("Expected reserved-pool to have 2 masked VFs, got %d", pool.Masked)
			}
		}
	}

	if !foundTestPool {
		t.Error("Expected to find test-pool")
	}
	if !foundReservedPool {
		t.Error("Expected to find reserved-pool")
	}
}

func TestAllocateVFs(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Allocate 2 VFs
	req := &pb.AllocationRequest{
		PfPci: "0000:01:00.0",
		Count: 2,
	}

	resp, err := client.AllocateVFs(ctx, req)
	if err != nil {
		t.Fatalf("AllocateVFs failed: %v", err)
	}

	if len(resp.AllocatedVfs) != 2 {
		t.Errorf("Expected 2 allocated VFs, got %d", len(resp.AllocatedVfs))
	}

	// Check that the VFs are from the test-pool (not reserved)
	for _, vf := range resp.AllocatedVfs {
		if vf.Pool != "test-pool" {
			t.Errorf("Expected VF to be from test-pool, got %s", vf.Pool)
		}
		if !vf.Allocated {
			t.Error("Expected VF to be marked as allocated")
		}
	}
}

func TestReleaseVFs(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// First allocate some VFs
	allocReq := &pb.AllocationRequest{
		PfPci: "0000:01:00.0",
		Count: 2,
	}

	allocResp, err := client.AllocateVFs(ctx, allocReq)
	if err != nil {
		t.Fatalf("AllocateVFs failed: %v", err)
	}

	// Then release them
	var vfPcis []string
	for _, vf := range allocResp.AllocatedVfs {
		vfPcis = append(vfPcis, vf.VfPci)
	}

	releaseReq := &pb.ReleaseRequest{
		VfPcis: vfPcis,
	}

	releaseResp, err := client.ReleaseVFs(ctx, releaseReq)
	if err != nil {
		t.Fatalf("ReleaseVFs failed: %v", err)
	}

	if len(releaseResp.Released) != 2 {
		t.Errorf("Expected 2 released VFs, got %d", len(releaseResp.Released))
	}
}

func TestMaskVF(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Mask a VF
	req := &pb.MaskRequest{
		VfPci:  "0000:01:00.0-vf0",
		Reason: "Test masking",
	}

	resp, err := client.MaskVF(ctx, req)
	if err != nil {
		t.Fatalf("MaskVF failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected MaskVF to succeed")
	}
}

func TestUnmaskVF(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Unmask a VF
	req := &pb.UnmaskRequest{
		VfPci: "0000:01:00.0-vf0",
	}

	resp, err := client.UnmaskVF(ctx, req)
	if err != nil {
		t.Fatalf("UnmaskVF failed: %v", err)
	}

	if !resp.Success {
		t.Error("Expected UnmaskVF to succeed")
	}
}

func TestListPools(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.ListPools(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("ListPools failed: %v", err)
	}

	if len(resp.Names) != 2 {
		t.Errorf("Expected 2 pool names, got %d", len(resp.Names))
	}

	// Check for expected pool names
	foundTestPool := false
	foundReservedPool := false
	for _, name := range resp.Names {
		if name == "test-pool" {
			foundTestPool = true
		}
		if name == "reserved-pool" {
			foundReservedPool = true
		}
	}

	if !foundTestPool {
		t.Error("Expected to find test-pool")
	}
	if !foundReservedPool {
		t.Error("Expected to find reserved-pool")
	}
}

func TestGetPoolConfig(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	req := &pb.PoolQuery{
		Name: "test-pool",
	}

	resp, err := client.GetPoolConfig(ctx, req)
	if err != nil {
		t.Fatalf("GetPoolConfig failed: %v", err)
	}

	if resp.Name != "test-pool" {
		t.Errorf("Expected pool name 'test-pool', got '%s'", resp.Name)
	}

	if resp.PfPci != "0000:01:00.0" {
		t.Errorf("Expected PF PCI '0000:01:00.0', got '%s'", resp.PfPci)
	}

	if resp.VfRange != "0-3" {
		t.Errorf("Expected VF range '0-3', got '%s'", resp.VfRange)
	}

	if resp.Mask {
		t.Error("Expected mask to be false")
	}

	if len(resp.RequiredFeatures) != 1 {
		t.Errorf("Expected 1 required feature, got %d", len(resp.RequiredFeatures))
	}

	if resp.RequiredFeatures[0] != "rx_checksum" {
		t.Errorf("Expected required feature 'rx_checksum', got '%s'", resp.RequiredFeatures[0])
	}
}
