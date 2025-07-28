package main

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"sriov-plugin/internal/config"
	"sriov-plugin/pkg/types"
	pb "sriov-plugin/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

var lis *bufconn.Listener

func setupTestServer(t *testing.T) (*server, *grpc.ClientConn) {
	// Create a temporary config file for testing
	configContent := `
discovery:
  allowed_vendor_ids:
    - "0x15b3"
    - "0x8086"
  excluded_vendor_ids: []
  enable_representor_discovery: true
  enable_switchdev_mode_check: true

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
		logger: logrus.New(),
	}

	// Load configuration
	cfg, err := config.LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	s.config = cfg

	// Initialize the SR-IOV cache
	s.sriovCache.pfs = make(map[string]*types.PFInfo)
	s.sriovCache.vfs = make(map[string]*types.VFInfo)
	s.sriovCache.representors = make(map[string]*types.RepresentorInfo)

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

func TestDumpInterfaces(t *testing.T) {
	_, conn := setupTestServer(t)
	defer conn.Close()

	client := pb.NewSriovDeviceManagerClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := client.DumpInterfaces(ctx, &pb.Empty{})
	if err != nil {
		t.Fatalf("DumpInterfaces failed: %v", err)
	}

	// Verify that the response has the expected fields
	if resp.JsonData == "" {
		t.Error("DumpInterfaces returned empty JSON data")
	}
	if resp.Timestamp == "" {
		t.Error("DumpInterfaces returned empty timestamp")
	}
	if resp.Version == "" {
		t.Error("DumpInterfaces returned empty version")
	}

	t.Logf("DumpInterfaces returned JSON data of length %d", len(resp.JsonData))
}
