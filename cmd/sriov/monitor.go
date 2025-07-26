package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"example.com/sriov-plugin/pkg"
	"example.com/sriov-plugin/proto"
)

var (
	// Monitor command flags
	monitorServerAddr string
	monitorTimeout    time.Duration
	monitorInterval   time.Duration
	monitorFormat     string
	monitorLogLevel   string
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor SR-IOV devices in real-time",
	Long: `Monitor SR-IOV devices in real-time with continuous updates.

This command will:
  • Connect to the SR-IOV server
  • Continuously monitor device changes
  • Display updates in real-time
  • Show device status changes

Examples:
  sriov monitor                    # Monitor with default settings
  sriov monitor --interval 5s     # Update every 5 seconds
  sriov monitor --format json     # Output in JSON format`,
	RunE: runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)

	// Add flags
	monitorCmd.Flags().StringVar(&monitorServerAddr, "server", "localhost:50051", "gRPC server address")
	monitorCmd.Flags().DurationVar(&monitorTimeout, "timeout", 5*time.Second, "Connection timeout")
	monitorCmd.Flags().DurationVar(&monitorInterval, "interval", 10*time.Second, "Monitoring interval")
	monitorCmd.Flags().StringVar(&monitorFormat, "format", "table", "Output format: table, json, simple")
	monitorCmd.Flags().StringVar(&monitorLogLevel, "log-level", "warn", "Log level: debug, info, warn, error")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	// Set log level from flag
	if err := pkg.SetLogLevelFromString(monitorLogLevel); err != nil {
		return fmt.Errorf("invalid log level: %v", err)
	}

	// Connect to server
	pkg.Info("Connecting to SR-IOV server at %s...", monitorServerAddr)
	conn, err := grpc.Dial(monitorServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	pkg.Info("Connected successfully")
	pkg.Info("Starting real-time monitoring (interval: %v)", monitorInterval)

	c := proto.NewSRIOVManagerClient(conn)

	// Monitor loop
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()

	lastDeviceCount := 0
	lastDevices := ""

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), monitorTimeout)

			// Get device list
			r, err := c.ListDevices(ctx, &proto.ListDevicesRequest{})
			cancel()

			if err != nil {
				pkg.Error("Error getting devices: %v", err)
				continue
			}

			// Convert to DeviceInfo for consistent formatting
			devices := make([]DeviceInfo, len(r.Devices))
			for i, d := range r.Devices {
				deviceInfo := DeviceInfo{
					PCIAddress:   d.PciAddress,
					Name:         d.Name,
					Driver:       d.Driver,
					Vendor:       d.Vendor,
					Product:      d.Product,
					SRIOVCapable: d.SriovCapable,
					NUMANode:     int(d.NumaNode),
					NUMADistance: make(map[int]int),
				}

				// Add NUMA distance information
				for node, distance := range d.NumaDistance {
					deviceInfo.NUMADistance[int(node)] = int(distance)
				}

				devices[i] = deviceInfo
			}

			// Check if there are changes
			currentDeviceCount := len(devices)
			currentDevices := ""

			switch strings.ToLower(monitorFormat) {
			case "json":
				currentDevices = formatDeviceJSON(devices)
			case "simple":
				currentDevices = formatDeviceSimple(devices)
			default:
				currentDevices = formatDeviceTable(devices)
			}

			// Only print if there are changes
			if currentDeviceCount != lastDeviceCount || currentDevices != lastDevices {
				pkg.Info("=== Device Update (%s) ===", time.Now().Format("15:04:05"))
				pkg.Info("Device count: %d", currentDeviceCount)
				fmt.Println(currentDevices)
				pkg.Info("=== End Update ===")

				lastDeviceCount = currentDeviceCount
				lastDevices = currentDevices
			}
		}
	}
}
