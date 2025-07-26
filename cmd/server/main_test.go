package main

import (
	"context"
	"net"
	"testing"
	"time"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

func TestListDevices(t *testing.T) {
	devs, err := pkg.ParseLshw("../../lshw-network.json")
	if err != nil {
		t.Fatalf("failed to parse lshw: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSRIOVManagerServer(grpcServer, &server{devices: devs})
	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	c := pb.NewSRIOVManagerClient(conn)
	resp, err := c.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		t.Fatalf("ListDevices error: %v", err)
	}
	if len(resp.Devices) != len(devs) {
		t.Fatalf("expected %d devices, got %d", len(devs), len(resp.Devices))
	}
}
