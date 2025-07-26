package main

import (
	"context"
	"log"
	"net"

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

func main() {
	lshw := "lshw-network.json"
	devices, err := pkg.ParseLshw(lshw)
	if err != nil {
		log.Fatalf("failed to parse lshw output: %v", err)
	}

	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		log.Printf("failed to enrich devices with pci info: %v", err)
	}

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSRIOVManagerServer(grpcServer, &server{devices: devices})
	log.Println("SR-IOV manager gRPC server running on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
