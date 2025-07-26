package main

import (
	"context"
	"log"
	"time"

	pb "example.com/sriov-plugin/proto"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewSRIOVManagerClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.ListDevices(ctx, &pb.ListDevicesRequest{})
	if err != nil {
		log.Fatalf("could not list devices: %v", err)
	}
	for _, d := range r.Devices {
		log.Printf("Device %s (%s) driver=%s vendor=%s product=%s", d.Name, d.PciAddress, d.Driver, d.Vendor, d.Product)
	}
}
