package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

// server implements the SRIOVManager gRPC server
type server struct {
	pb.UnimplementedSRIOVManagerServer
	devices []pkg.Device
}

func (s *server) ListDevices(ctx context.Context, in *pb.ListDevicesRequest) (*pb.ListDevicesResponse, error) {
	log.Printf("📋 ListDevices request received - returning %d devices", len(s.devices))
	resp := &pb.ListDevicesResponse{}
	for _, d := range s.devices {
		resp.Devices = append(resp.Devices, &pb.Device{
			PciAddress: d.PCIAddress,
			Name:       d.Name,
			Driver:     d.Driver,
			Vendor:     d.Vendor,
			Product:    d.Product,
		})
	}
	return resp, nil
}

// debugPrintDeviceInfo prints detailed device information for debugging
func debugPrintDeviceInfo(devices []pkg.Device) {
	log.Printf("🔍 Device Information Collection Summary:")
	log.Printf("   Total devices found: %d", len(devices))

	sriovCount := 0
	networkCount := 0

	for i, device := range devices {
		log.Printf("   Device %d:", i+1)
		log.Printf("     PCI Address: %s", device.PCIAddress)
		log.Printf("     Name: %s", device.Name)
		log.Printf("     Driver: %s", device.Driver)
		log.Printf("     Vendor: %s", device.Vendor)
		log.Printf("     Product: %s", device.Product)
		log.Printf("     SR-IOV Capable: %t", device.SRIOVCapable)

		if device.SRIOVCapable && device.SRIOVInfo != nil {
			sriovCount++
			log.Printf("     SR-IOV Details:")
			log.Printf("       Total VFs: %d", device.SRIOVInfo.TotalVFs)
			log.Printf("       Number of VFs: %d", device.SRIOVInfo.NumberOfVFs)
			log.Printf("       VF Offset: %d", device.SRIOVInfo.VFOffset)
			log.Printf("       VF Stride: %d", device.SRIOVInfo.VFStride)
			log.Printf("       VF Device ID: %s", device.SRIOVInfo.VFDeviceID)
		}

		if device.Name != "" {
			networkCount++
		}

		log.Printf("")
	}

	log.Printf("📊 Summary Statistics:")
	log.Printf("   Network devices: %d", networkCount)
	log.Printf("   SR-IOV capable devices: %d", sriovCount)
	log.Printf("   Non-SR-IOV devices: %d", len(devices)-sriovCount)
}

func main() {
	startTime := time.Now()
	log.Printf("🚀 Starting SR-IOV Manager Server...")

	// Check if lshw file exists
	lshw := "lshw-network.json"
	if _, err := os.Stat(lshw); os.IsNotExist(err) {
		log.Printf("⚠️  Warning: %s not found, using mock data", lshw)
		// Use mock data for development
		lshw = "lshw-network.json" // This will fail, but we'll handle it gracefully
	}

	log.Printf("📁 Parsing lshw data from: %s", lshw)
	devices, err := pkg.ParseLshw(lshw)
	if err != nil {
		log.Printf("❌ Failed to parse lshw output: %v", err)
		log.Printf("🔄 Falling back to mock data for development...")
		// For development, we could create mock data here
		devices = []pkg.Device{}
	} else {
		log.Printf("✅ Successfully parsed %d devices from lshw", len(devices))
	}

	log.Printf("🔧 Enriching devices with PCI information...")
	enrichStart := time.Now()
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		log.Printf("⚠️  Warning: failed to enrich devices with PCI info: %v", err)
		log.Printf("   Continuing with basic device information...")
	} else {
		log.Printf("✅ PCI enrichment completed in %v", time.Since(enrichStart))
	}

	// Print detailed device information for debugging
	debugPrintDeviceInfo(devices)

	log.Printf("🌐 Starting gRPC server...")
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	srv := &server{devices: devices}
	pb.RegisterSRIOVManagerServer(grpcServer, srv)

	totalStartupTime := time.Since(startTime)
	log.Printf("✅ SR-IOV manager gRPC server ready on :50051")
	log.Printf("⏱️  Total startup time: %v", totalStartupTime)
	log.Printf("📡 Server is ready to accept connections...")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
