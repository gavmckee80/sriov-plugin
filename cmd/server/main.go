package main

import (
	"context"
	"flag"
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
		device := &pb.Device{
			PciAddress:   d.PCIAddress,
			Name:         d.Name,
			Driver:       d.Driver,
			Vendor:       d.Vendor,
			Product:      d.Product,
			SriovCapable: d.SRIOVCapable,
		}

		// Add detailed capabilities
		if len(d.DetailedCapabilities) > 0 {
			device.DetailedCapabilities = make(map[string]*pb.DetailedCapability)
			for name, cap := range d.DetailedCapabilities {
				device.DetailedCapabilities[name] = &pb.DetailedCapability{
					Id:          cap.ID,
					Name:        cap.Name,
					Version:     cap.Version,
					Status:      cap.Status,
					Parameters:  cap.Parameters,
					Description: cap.Description,
				}
			}
		}

		resp.Devices = append(resp.Devices, device)
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

		// Enhanced context information
		if device.Description != "" {
			log.Printf("     Description: %s", device.Description)
		}
		if device.Serial != "" {
			log.Printf("     Serial: %s", device.Serial)
		}
		if device.Size != "" {
			log.Printf("     Size: %s", device.Size)
		}
		if device.Capacity != "" {
			log.Printf("     Capacity: %s", device.Capacity)
		}
		if device.Clock != "" {
			log.Printf("     Clock: %s", device.Clock)
		}
		if device.Width != "" {
			log.Printf("     Width: %s", device.Width)
		}
		if device.Class != "" {
			log.Printf("     Class: %s", device.Class)
		}
		if device.SubClass != "" {
			log.Printf("     SubClass: %s", device.SubClass)
		}
		if len(device.Capabilities) > 0 {
			log.Printf("     Capabilities: %v", device.Capabilities)
		}

		// Detailed capability information
		if len(device.DetailedCapabilities) > 0 {
			log.Printf("     Detailed Capabilities:")
			for capName, cap := range device.DetailedCapabilities {
				log.Printf("       [%s] %s: %s", cap.ID, capName, cap.Description)
			}
		}

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
	// Parse command line flags
	var (
		useFile  = flag.Bool("file", false, "Use static lshw file for testing (default: dynamic lshw)")
		lshwFile = flag.String("lshw-file", "lshw-network.json", "Path to lshw JSON file (when using -file)")
	)
	flag.Parse()

	startTime := time.Now()
	log.Printf("🚀 Starting SR-IOV Manager Server...")

	var devices []pkg.Device
	var err error

	if *useFile {
		// Development/Testing mode: Use static file
		log.Printf("📁 Development mode: Parsing lshw data from file: %s", *lshwFile)
		if _, err := os.Stat(*lshwFile); os.IsNotExist(err) {
			log.Printf("⚠️  Warning: %s not found, using empty device list", *lshwFile)
			devices = []pkg.Device{}
		} else {
			devices, err = pkg.ParseLshwFromFile(*lshwFile)
			if err != nil {
				log.Printf("❌ Failed to parse lshw file: %v", err)
				log.Printf("🔄 Falling back to empty device list...")
				devices = []pkg.Device{}
			} else {
				log.Printf("✅ Successfully parsed %d devices from lshw file", len(devices))
			}
		}
	} else {
		// Production mode: Run lshw dynamically
		log.Printf("🔍 Production mode: Running lshw -class network -json dynamically")
		lshwStart := time.Now()
		devices, err = pkg.ParseLshwDynamic()
		if err != nil {
			log.Printf("❌ Failed to run lshw: %v", err)
			log.Printf("🔄 Falling back to empty device list...")
			devices = []pkg.Device{}
		} else {
			log.Printf("✅ Successfully gathered %d devices from lshw in %v", len(devices), time.Since(lshwStart))
		}
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
