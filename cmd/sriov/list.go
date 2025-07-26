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
	// List command flags
	listServerAddr  string
	listTimeout     time.Duration
	listDeviceName  string
	listFormat      string
	listTableFormat string
	listRefresh     bool
	listLogLevel    string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List SR-IOV devices",
	Long: `List all discovered SR-IOV devices with various output formats.

Available formats:
  • table (default) - Pretty-printed table
  • json - JSON output
  • csv - CSV format
  • simple - Tab-separated values
  • detailed - Verbose text output

Table formats:
  • default - Standard table with basic info
  • extended - Extended table with description
  • numa - NUMA-focused view
  • sriov - SR-IOV specific format

Examples:
  sriov list                                    # Default table format
  sriov list --format json                     # JSON output
  sriov list --table-format sriov             # SR-IOV specific format
  sriov list --device-name ens60f0np0         # Filter by device name
  sriov list --refresh                         # Trigger manual refresh`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)

	// Add flags
	listCmd.Flags().StringVar(&listServerAddr, "server", "localhost:50051", "gRPC server address")
	listCmd.Flags().DurationVar(&listTimeout, "timeout", 5*time.Second, "Connection timeout")
	listCmd.Flags().StringVar(&listDeviceName, "device-name", "", "Filter by device name (exact match)")
	listCmd.Flags().StringVar(&listFormat, "format", "table", "Output format: table, json, simple, csv, detailed")
	listCmd.Flags().StringVar(&listTableFormat, "table-format", "default", "Table format: default, extended, numa, sriov")
	listCmd.Flags().BoolVar(&listRefresh, "refresh", false, "Trigger manual refresh of device list")
	listCmd.Flags().StringVar(&listLogLevel, "log-level", "warn", "Log level: debug, info, warn, error")

	// Mark flags as required if needed
	// listCmd.MarkFlagRequired("server")
}

func runList(cmd *cobra.Command, args []string) error {
	// Set log level from flag
	if err := pkg.SetLogLevelFromString(listLogLevel); err != nil {
		return fmt.Errorf("invalid log level: %v", err)
	}

	// Validate format
	switch strings.ToLower(listFormat) {
	case "table", "json", "simple", "csv", "detailed":
		// Valid format
	default:
		return fmt.Errorf("invalid format: %s. Use: table, json, simple, csv, or detailed", listFormat)
	}

	// Connect to server
	pkg.Info("Connecting to SR-IOV server at %s...", listServerAddr)
	conn, err := grpc.Dial(listServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	pkg.Info("Connected successfully")

	c := proto.NewSRIOVManagerClient(conn)

	// Handle refresh command
	if listRefresh {
		pkg.Info("Triggering manual refresh...")
		ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
		defer cancel()

		resp, err := c.RefreshDevices(ctx, &proto.RefreshDevicesRequest{})
		if err != nil {
			return fmt.Errorf("failed to refresh devices: %v", err)
		}

		if resp.Success {
			pkg.Info("Success: %s", resp.Message)
		} else {
			pkg.Error("Error: Refresh failed: %s", resp.Message)
		}
		return nil
	}

	// List devices
	ctx, cancel := context.WithTimeout(context.Background(), listTimeout)
	defer cancel()

	pkg.Info("Requesting device list...")
	r, err := c.ListDevices(ctx, &proto.ListDevicesRequest{})
	if err != nil {
		return fmt.Errorf("could not list devices: %v", err)
	}

	pkg.Info("Received %d devices", len(r.Devices))

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

		// Add detailed capabilities if available
		if len(d.DetailedCapabilities) > 0 {
			deviceInfo.DetailedCapabilities = make(map[string]DetailedCapabilityInfo)
			for name, cap := range d.DetailedCapabilities {
				deviceInfo.DetailedCapabilities[name] = DetailedCapabilityInfo{
					ID:          cap.Id,
					Name:        cap.Name,
					Status:      cap.Status,
					Description: cap.Description,
					Parameters:  cap.Parameters,
				}
			}
		}

		// Add ethtool information
		if d.EthtoolInfo != nil {
			deviceInfo.EthtoolInfo = &EthtoolInfo{
				Features: make([]EthtoolFeature, len(d.EthtoolInfo.Features)),
				Ring: EthtoolRingInfo{
					RxMaxPending:      d.EthtoolInfo.Ring.RxMaxPending,
					RxMiniMaxPending:  d.EthtoolInfo.Ring.RxMiniMaxPending,
					RxJumboMaxPending: d.EthtoolInfo.Ring.RxJumboMaxPending,
					TxMaxPending:      d.EthtoolInfo.Ring.TxMaxPending,
					RxPending:         d.EthtoolInfo.Ring.RxPending,
					RxMiniPending:     d.EthtoolInfo.Ring.RxMiniPending,
					RxJumboPending:    d.EthtoolInfo.Ring.RxJumboPending,
					TxPending:         d.EthtoolInfo.Ring.TxPending,
				},
				Channels: EthtoolChannelInfo{
					MaxRx:         d.EthtoolInfo.Channels.MaxRx,
					MaxTx:         d.EthtoolInfo.Channels.MaxTx,
					MaxOther:      d.EthtoolInfo.Channels.MaxOther,
					MaxCombined:   d.EthtoolInfo.Channels.MaxCombined,
					RxCount:       d.EthtoolInfo.Channels.RxCount,
					TxCount:       d.EthtoolInfo.Channels.TxCount,
					OtherCount:    d.EthtoolInfo.Channels.OtherCount,
					CombinedCount: d.EthtoolInfo.Channels.CombinedCount,
				},
			}

			for j, feature := range d.EthtoolInfo.Features {
				deviceInfo.EthtoolInfo.Features[j] = EthtoolFeature{
					Name:    feature.Name,
					Enabled: feature.Enabled,
					Fixed:   feature.Fixed,
				}
			}
		}

		devices[i] = deviceInfo
	}

	// Filter by device name if specified
	if listDeviceName != "" {
		var filteredDevices []DeviceInfo
		for _, device := range devices {
			if device.Name == listDeviceName {
				filteredDevices = append(filteredDevices, device)
			}
		}
		devices = filteredDevices
		pkg.Info("Filtered to %d devices matching '%s'", len(devices), listDeviceName)
	}

	// Format output based on user preference
	switch strings.ToLower(listFormat) {
	case "json":
		fmt.Println(formatDeviceJSON(devices))
	case "simple":
		fmt.Println(formatDeviceSimple(devices))
	case "csv":
		fmt.Println(formatDeviceCSV(devices))
	case "detailed":
		fmt.Println(formatDeviceDetailed(devices))
	case "table":
		switch strings.ToLower(listTableFormat) {
		case "default":
			fmt.Println(formatDeviceTable(devices))
		case "extended":
			fmt.Println(formatDeviceTableExtended(devices))
		case "numa":
			fmt.Println(formatDeviceTableNUMA(devices))
		case "sriov":
			fmt.Println(formatDeviceTableSRIOV(devices))
		default:
			fmt.Println(formatDeviceTable(devices))
		}
	default:
		fmt.Println(formatDeviceTable(devices))
	}

	return nil
}
