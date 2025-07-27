package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	pb "sriov-plugin/proto"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var logger = logrus.New()

// getInterfaceNameForVF attempts to find the interface name for a VF
func getInterfaceNameForVF(vfPCI string) string {
	// Extract PF PCI and VF number from VF PCI address
	// Format: 0000:31:00.0-vf15 -> PF: 0000:31:00.0, VF: 15
	if idx := strings.LastIndex(vfPCI, "-vf"); idx > 0 {
		pfPCI := vfPCI[:idx]
		vfNumStr := vfPCI[idx+3:] // Remove "-vf" prefix

		// Look in PF's net directory for VF interfaces
		netPath := fmt.Sprintf("/sys/bus/pci/devices/%s/net", pfPCI)

		if entries, err := os.ReadDir(netPath); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					interfaceName := entry.Name()
					// Check if this interface corresponds to our VF
					// VF interfaces typically have patterns like:
					// - ens60f0npf0vf15 (for VF 15)
					// - eth100 (for VF 100)
					if strings.Contains(interfaceName, fmt.Sprintf("vf%s", vfNumStr)) ||
						(strings.HasPrefix(interfaceName, "eth") && len(interfaceName) > 3) {
						// For eth interfaces, check if the number matches
						if strings.HasPrefix(interfaceName, "eth") {
							if ethNum, err := strconv.Atoi(interfaceName[3:]); err == nil {
								if vfNum, err := strconv.Atoi(vfNumStr); err == nil && ethNum == vfNum {
									return interfaceName
								}
							}
						} else {
							return interfaceName
						}
					}
				}
			}
		}
	}

	// If no interface found, return empty string
	return ""
}

// formatVFName returns a user-friendly name for a VF
func formatVFName(vfPCI string) string {
	interfaceName := getInterfaceNameForVF(vfPCI)
	if interfaceName != "" {
		// Extract VF number from PCI address
		if idx := strings.LastIndex(vfPCI, "-vf"); idx > 0 {
			vfNum := vfPCI[idx+3:] // Remove "-vf" prefix
			return fmt.Sprintf("%s vf %s", interfaceName, vfNum)
		}
	}
	// Fallback to PCI address if no interface name found
	return vfPCI
}

var rootCmd = &cobra.Command{
	Use:   "sriovctl",
	Short: "SR-IOV management CLI",
	Long:  `A command line interface for managing SR-IOV devices and pools.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		logger.WithError(err).Fatal("command execution failed")
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

func listDevices(cmd *cobra.Command, args []string) error {
	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	resp, err := client.ListDevices(context.Background(), &pb.Empty{})
	if err != nil {
		logger.WithError(err).Fatal("failed to list devices")
	}

	for _, pf := range resp.Pfs {
		fmt.Printf("PF %s ():\n", pf.PfPci)
		for _, vf := range pf.Vfs {
			fmt.Printf("  VF %s iface=%s allocated=%t masked=%t pool=%s\n",
				vf.VfPci, vf.Interface, vf.Allocated, vf.Masked, vf.Pool)
		}
	}
	return nil
}

func getStatus(cmd *cobra.Command, args []string) error {
	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	resp, err := client.GetStatus(context.Background(), &pb.Empty{})
	if err != nil {
		logger.WithError(err).Fatal("failed to get status")
	}

	fmt.Println("Pool Status:")
	for _, pool := range resp.Pools {
		fmt.Printf("  %s: %d total, %d allocated, %d masked, %d free (%.1f%%)\n",
			pool.Name, pool.Total, pool.Allocated, pool.Masked, pool.Free, pool.PercentFree)
	}
	return nil
}

func allocateVFs(cmd *cobra.Command, args []string) error {
	pfPCI, _ := cmd.Flags().GetString("pf")
	count, _ := cmd.Flags().GetInt("count")
	numaNode, _ := cmd.Flags().GetString("numa")
	requiredFeatures, _ := cmd.Flags().GetStringSlice("required-features")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	req := &pb.AllocationRequest{
		PfPci:            pfPCI,
		Count:            uint32(count),
		NumaNode:         numaNode,
		RequiredFeatures: requiredFeatures,
		DryRun:           dryRun,
	}

	resp, err := client.AllocateVFs(context.Background(), req)
	if err != nil {
		logger.WithError(err).Fatal("failed to allocate VFs")
	}

	if len(resp.AllocatedVfs) == 0 {
		logger.Warn("no VFs were allocated")
		return nil
	}

	fmt.Printf("Allocated %d VFs:\n", len(resp.AllocatedVfs))
	for _, vf := range resp.AllocatedVfs {
		fmt.Printf("  %s (pool: %s)\n", vf.VfPci, vf.Pool)
	}
	return nil
}

func releaseVFs(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		logger.Fatal("no VF PCI addresses provided")
	}

	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	req := &pb.ReleaseRequest{
		VfPcis: args,
	}

	resp, err := client.ReleaseVFs(context.Background(), req)
	if err != nil {
		logger.WithError(err).Fatal("failed to release VFs")
	}

	fmt.Printf("Released %d VFs:\n", len(resp.Released))
	for _, vf := range resp.Released {
		fmt.Printf("  %s\n", vf)
	}
	return nil
}

func maskVF(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		logger.Fatal("exactly one VF PCI address required")
	}

	reason, _ := cmd.Flags().GetString("reason")
	if reason == "" {
		logger.Fatal("reason is required for masking")
	}

	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	req := &pb.MaskRequest{
		VfPci:  args[0],
		Reason: reason,
	}

	resp, err := client.MaskVF(context.Background(), req)
	if err != nil {
		logger.WithError(err).Fatal("failed to mask VF")
	}

	if resp.Success {
		logger.WithField("vf", formatVFName(args[0])).Info("VF masked successfully")
	} else {
		logger.WithField("vf", formatVFName(args[0])).Error("failed to mask VF")
	}
	return nil
}

func unmaskVF(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		logger.Fatal("exactly one VF PCI address required")
	}

	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	req := &pb.UnmaskRequest{
		VfPci: args[0],
	}

	resp, err := client.UnmaskVF(context.Background(), req)
	if err != nil {
		logger.WithError(err).Fatal("failed to unmask VF")
	}

	if resp.Success {
		logger.WithField("vf", formatVFName(args[0])).Info("VF unmasked successfully")
	} else {
		logger.WithField("vf", formatVFName(args[0])).Error("failed to unmask VF")
	}
	return nil
}

func listPools(cmd *cobra.Command, args []string) error {
	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	resp, err := client.ListPools(context.Background(), &pb.Empty{})
	if err != nil {
		logger.WithError(err).Fatal("failed to list pools")
	}

	fmt.Println("Available pools:")
	for _, name := range resp.Names {
		fmt.Printf("  %s\n", name)
	}
	return nil
}

func getPoolConfig(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		logger.Fatal("exactly one pool name required")
	}

	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	req := &pb.PoolQuery{
		Name: args[0],
	}

	resp, err := client.GetPoolConfig(context.Background(), req)
	if err != nil {
		logger.WithError(err).Fatal("failed to get pool config")
	}

	fmt.Printf("Pool: %s\n", resp.Name)
	fmt.Printf("  PF PCI: %s\n", resp.PfPci)
	fmt.Printf("  VF Range: %s\n", resp.VfRange)
	fmt.Printf("  Masked: %t\n", resp.Mask)
	if resp.Mask {
		fmt.Printf("  Mask Reason: %s\n", resp.MaskReason)
	}
	fmt.Printf("  NUMA: %s\n", resp.Numa)
	fmt.Printf("  Required Features: %v\n", resp.RequiredFeatures)
	return nil
}

func dumpInterfaces(cmd *cobra.Command, args []string) error {
	client, conn, err := getClient()
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to server")
	}
	defer conn.Close()

	resp, err := client.DumpInterfaces(context.Background(), &pb.Empty{})
	if err != nil {
		logger.WithError(err).Fatal("failed to dump interfaces")
	}

	// Pretty print the JSON
	var prettyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(resp.JsonData), &prettyJSON); err != nil {
		logger.WithError(err).Fatal("failed to parse JSON response")
	}

	prettyBytes, err := json.MarshalIndent(prettyJSON, "", "  ")
	if err != nil {
		logger.WithError(err).Fatal("failed to format JSON")
	}

	fmt.Printf("Interface Dump (Version: %s, Timestamp: %s):\n", resp.Version, resp.Timestamp)
	fmt.Println(string(prettyBytes))
	return nil
}

func init() {
	// Configure logrus
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logger.SetLevel(logrus.InfoLevel)

	// List command
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all SR-IOV devices",
		RunE:  listDevices,
	}
	rootCmd.AddCommand(listCmd)

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Get status of all pools",
		RunE:  getStatus,
	}
	rootCmd.AddCommand(statusCmd)

	// Allocate command
	allocateCmd := &cobra.Command{
		Use:   "allocate",
		Short: "Allocate VFs from a pool",
		RunE:  allocateVFs,
	}
	allocateCmd.Flags().String("pf", "", "PF PCI address")
	allocateCmd.Flags().Int("count", 1, "Number of VFs to allocate")
	allocateCmd.Flags().String("numa", "", "NUMA node preference")
	allocateCmd.Flags().StringSlice("required-features", []string{}, "Required features")
	allocateCmd.Flags().Bool("dry-run", false, "Dry run mode")
	allocateCmd.MarkFlagRequired("pf")
	rootCmd.AddCommand(allocateCmd)

	// Release command
	releaseCmd := &cobra.Command{
		Use:   "release [VF_PCI_ADDRESSES...]",
		Short: "Release allocated VFs",
		Args:  cobra.MinimumNArgs(1),
		RunE:  releaseVFs,
	}
	rootCmd.AddCommand(releaseCmd)

	// Mask command
	maskCmd := &cobra.Command{
		Use:   "mask [VF_PCI_ADDRESS]",
		Short: "Mask a VF",
		Args:  cobra.ExactArgs(1),
		RunE:  maskVF,
	}
	maskCmd.Flags().String("reason", "", "Reason for masking")
	maskCmd.MarkFlagRequired("reason")
	rootCmd.AddCommand(maskCmd)

	// Unmask command
	unmaskCmd := &cobra.Command{
		Use:   "unmask [VF_PCI_ADDRESS]",
		Short: "Unmask a VF",
		Args:  cobra.ExactArgs(1),
		RunE:  unmaskVF,
	}
	rootCmd.AddCommand(unmaskCmd)

	// List pools command
	listPoolsCmd := &cobra.Command{
		Use:   "pools",
		Short: "List all pools",
		RunE:  listPools,
	}
	rootCmd.AddCommand(listPoolsCmd)

	// Get pool config command
	getPoolConfigCmd := &cobra.Command{
		Use:   "pool-config [POOL_NAME]",
		Short: "Get configuration for a specific pool",
		Args:  cobra.ExactArgs(1),
		RunE:  getPoolConfig,
	}
	rootCmd.AddCommand(getPoolConfigCmd)

	// Dump interfaces command
	dumpCmd := &cobra.Command{
		Use:   "dump",
		Short: "Dump comprehensive interface information in JSON format",
		Long:  "Get detailed information about all interfaces, pools, allocations, and statistics in JSON format",
		RunE:  dumpInterfaces,
	}
	rootCmd.AddCommand(dumpCmd)
}
