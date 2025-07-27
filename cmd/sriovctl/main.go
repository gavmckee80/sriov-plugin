package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	pb "sriov-plugin/proto"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var rootCmd = &cobra.Command{
	Use:   "sriovctl",
	Short: "SR-IOV device manager CLI",
	Long:  `Manage and inspect SR-IOV network devices via gRPC.`,
}

func main() {
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(allocateCmd)
	rootCmd.AddCommand(releaseCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getClient() (pb.SriovDeviceManagerClient, *grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, nil, err
	}
	return pb.NewSriovDeviceManagerClient(conn), conn, nil
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all devices and their VFs",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := getClient()
		if err != nil {
			return err
		}
		defer conn.Close()
		reply, err := client.ListDevices(context.Background(), &pb.Empty{})
		if err != nil {
			return err
		}
		for _, pf := range reply.Pfs {
			fmt.Printf("PF %s (%s):\n", pf.PfPci, pf.Interface)
			for _, vf := range pf.Vfs {
				fmt.Printf("  VF %s iface=%s allocated=%v masked=%v pool=%s\n", vf.VfPci, vf.Interface, vf.Allocated, vf.Masked, vf.Pool)
			}
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show allocation status per pool",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, conn, err := getClient()
		if err != nil {
			return err
		}
		defer conn.Close()
		resp, err := client.GetStatus(context.Background(), &pb.Empty{})
		if err != nil {
			return err
		}
		for _, pool := range resp.Pools {
			fmt.Printf("Pool: %s on PF %s\n", pool.Name, pool.PfPci)
			fmt.Printf("  Total: %d  Allocated: %d  Masked: %d  Free: %d (%.1f%%)\n\n",
				pool.Total, pool.Allocated, pool.Masked, pool.Free, pool.PercentFree)
		}
		return nil
	},
}

var allocateCmd = &cobra.Command{
	Use:   "allocate",
	Short: "Allocate VFs from a PF",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pf == "" || count <= 0 {
			return fmt.Errorf("--pf and --count are required")
		}
		client, conn, err := getClient()
		if err != nil {
			return err
		}
		defer conn.Close()
		req := &pb.AllocationRequest{
			PfPci:            pf,
			Count:            uint32(count),
			NumaNode:         numa,
			RequiredFeatures: strings.Split(features, ","),
			DryRun:           dryRun,
		}
		resp, err := client.AllocateVFs(context.Background(), req)
		if err != nil {
			return err
		}
		fmt.Println(resp.Message)
		for _, vf := range resp.AllocatedVfs {
			fmt.Printf("Allocated VF: %s iface=%s\n", vf.VfPci, vf.Interface)
		}
		return nil
	},
}

var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "Release previously allocated VFs",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("must provide one or more VF PCI addresses")
		}
		client, conn, err := getClient()
		if err != nil {
			return err
		}
		defer conn.Close()
		req := &pb.ReleaseRequest{VfPcis: args}
		resp, err := client.ReleaseVFs(context.Background(), req)
		if err != nil {
			return err
		}
		fmt.Println("Released:", strings.Join(resp.Released, ", "))
		return nil
	},
}

var pf string
var count int
var numa string
var features string
var dryRun bool

func init() {
	allocateCmd.Flags().StringVar(&pf, "pf", "", "PF PCI address")
	allocateCmd.Flags().IntVar(&count, "count", 0, "Number of VFs to allocate")
	allocateCmd.Flags().StringVar(&numa, "numa", "", "NUMA node affinity (optional)")
	allocateCmd.Flags().StringVar(&features, "features", "", "Comma-separated list of required features")
	allocateCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Simulate allocation without committing")
}
