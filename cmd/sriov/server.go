package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"example.com/sriov-plugin/pkg"
	pb "example.com/sriov-plugin/proto"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var (
	// Server command flags
	serverPort     int
	serverConfig   string
	serverLogLevel string
)

// server implements the SRIOVManager gRPC server
type server struct {
	pb.UnimplementedSRIOVManagerServer
	devices      []pkg.Device
	devicesLock  sync.RWMutex
	lastUpdate   time.Time
	watchers     []*DeviceWatcher
	ethtoolCache map[string]*pkg.EthtoolInfo // Cache for ethtool info
	ethtoolLock  sync.RWMutex
}

// DeviceWatcher monitors for device changes
type DeviceWatcher struct {
	path     string
	events   chan DeviceEvent
	stopChan chan struct{}
}

// DeviceEvent represents a device change event
type DeviceEvent struct {
	Type      string // "interface", "pci", "driver", "sriov"
	Action    string // "created", "deleted", "modified"
	Device    string
	Timestamp time.Time
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the SR-IOV gRPC server",
	Long: `Start the SR-IOV gRPC server that provides device discovery and management services.

The server will:
  • Discover SR-IOV devices on the system
  • Provide gRPC endpoints for device listing
  • Support real-time device monitoring
  • Handle device refresh requests

Examples:
  sriov server                    # Start server on default port 50051
  sriov server --port 8080       # Start server on custom port
  sriov server --config config.yaml  # Use custom configuration`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add flags
	serverCmd.Flags().IntVar(&serverPort, "port", 50051, "gRPC server port")
	serverCmd.Flags().StringVar(&serverConfig, "config", "", "Configuration file path")
	serverCmd.Flags().StringVar(&serverLogLevel, "log-level", "info", "Log level: debug, info, warn, error")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Set log level from flag
	if err := pkg.SetLogLevelFromString(serverLogLevel); err != nil {
		return fmt.Errorf("invalid log level: %v", err)
	}

	// Create server instance
	s := &server{
		ethtoolCache: make(map[string]*pkg.EthtoolInfo),
	}

	// Start device monitoring
	s.StartDeviceMonitoring()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterSRIOVManagerServer(grpcServer, s)

	// Start server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	pkg.Info("Starting SR-IOV gRPC server on port %d", serverPort)

	// Handle graceful shutdown
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			pkg.Error("Failed to serve: %v", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	pkg.Info("Shutting down server...")
	grpcServer.GracefulStop()

	return nil
}

// StartDeviceMonitoring starts monitoring for device changes
func (s *server) StartDeviceMonitoring() {
	pkg.Info("Starting device change monitoring...")

	// Initialize ethtool cache
	s.ethtoolCache = make(map[string]*pkg.EthtoolInfo)

	// Monitor network interfaces
	s.watchNetworkInterfaces()

	// Monitor PCI devices
	s.watchPciDevices()

	// Monitor driver bindings
	s.watchDriverBindings()

	// Monitor SR-IOV configurations
	s.watchSriovConfigurations()

	// Monitor ethtool changes
	s.watchEthtoolChanges()

	// Start event processing
	go s.processDeviceEvents()
}

// watchNetworkInterfaces monitors /sys/class/net for interface changes
func (s *server) watchNetworkInterfaces() {
	if _, err := os.Stat("/sys/class/net"); err == nil {
		watcher := &DeviceWatcher{
			path:     "/sys/class/net",
			events:   make(chan DeviceEvent, 100),
			stopChan: make(chan struct{}),
		}
		s.watchers = append(s.watchers, watcher)

		go func() {
			for {
				select {
				case <-watcher.stopChan:
					return
				default:
					// Use inotify to watch for interface changes
					if err := s.monitorDirectory(watcher.path, "interface", watcher.events); err != nil {
						pkg.Warn("Warning: Network interface monitoring error: %v", err)
						time.Sleep(5 * time.Second) // Retry after delay
					}
				}
			}
		}()
		pkg.Info("Network interface monitoring enabled")
	} else {
		pkg.Warn("Warning: /sys/class/net not accessible, network interface monitoring disabled")
	}
}

// watchPciDevices monitors /sys/bus/pci/devices for PCI device changes
func (s *server) watchPciDevices() {
	if _, err := os.Stat("/sys/bus/pci/devices"); err == nil {
		watcher := &DeviceWatcher{
			path:     "/sys/bus/pci/devices",
			events:   make(chan DeviceEvent, 100),
			stopChan: make(chan struct{}),
		}
		s.watchers = append(s.watchers, watcher)

		go func() {
			for {
				select {
				case <-watcher.stopChan:
					return
				default:
					if err := s.monitorDirectory(watcher.path, "pci", watcher.events); err != nil {
						pkg.Warn("Warning: PCI device monitoring error: %v", err)
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		pkg.Info("PCI device monitoring enabled")
	} else {
		pkg.Warn("Warning: /sys/bus/pci/devices not accessible, PCI monitoring disabled")
	}
}

// watchDriverBindings monitors driver binding changes
func (s *server) watchDriverBindings() {
	driverPaths := []string{
		"/sys/bus/pci/drivers",
		"/sys/bus/pci/devices",
	}

	for _, path := range driverPaths {
		if _, err := os.Stat(path); err == nil {
			watcher := &DeviceWatcher{
				path:     path,
				events:   make(chan DeviceEvent, 100),
				stopChan: make(chan struct{}),
			}
			s.watchers = append(s.watchers, watcher)

			go func(w *DeviceWatcher) {
				for {
					select {
					case <-w.stopChan:
						return
					default:
						if err := s.monitorDirectory(w.path, "driver", w.events); err != nil {
							pkg.Warn("Warning: Driver binding monitoring error: %v", err)
							time.Sleep(5 * time.Second)
						}
					}
				}
			}(watcher)
		}
	}
	pkg.Info("Driver binding monitoring enabled")
}

// watchSriovConfigurations monitors SR-IOV specific configurations
func (s *server) watchSriovConfigurations() {
	// Monitor SR-IOV configurations in /sys/class/net
	go func() {
		for {
			s.checkSriovChanges()
			time.Sleep(10 * time.Second) // Check every 10 seconds
		}
	}()
	pkg.Info("SR-IOV configuration monitoring enabled")
}

// monitorDirectory uses fsnotify to monitor directory changes
func (s *server) monitorDirectory(path, eventType string, events chan<- DeviceEvent) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Add the directory to watch
	if err := watcher.Add(path); err != nil {
		return fmt.Errorf("failed to add path to watcher: %v", err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return fmt.Errorf("watcher events channel closed")
			}

			// Determine action based on event
			var action string
			switch {
			case event.Has(fsnotify.Create):
				action = "created"
			case event.Has(fsnotify.Remove):
				action = "deleted"
			case event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod):
				action = "modified"
			default:
				continue // Skip other events
			}

			// Extract device name from path
			deviceName := filepath.Base(event.Name)

			// Create device event
			deviceEvent := DeviceEvent{
				Type:      eventType,
				Action:    action,
				Device:    deviceName,
				Timestamp: time.Now(),
			}

			// Send event
			select {
			case events <- deviceEvent:
				pkg.Debug("Device event: %s %s %s", eventType, action, deviceName)
			default:
				pkg.Warn("Warning: Event channel full, dropping event")
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return fmt.Errorf("watcher errors channel closed")
			}
			pkg.Error("Watcher error: %v", err)
		}
	}
}

// processDeviceEvents processes device change events
func (s *server) processDeviceEvents() {
	events := s.collectEvents()

	for event := range events {
		s.handleDeviceChange(event)
	}
}

// collectEvents collects events from all watchers
func (s *server) collectEvents() <-chan DeviceEvent {
	eventChan := make(chan DeviceEvent, 1000)

	go func() {
		defer close(eventChan)

		for _, watcher := range s.watchers {
			go func(w *DeviceWatcher) {
				for event := range w.events {
					select {
					case eventChan <- event:
					default:
						pkg.Warn("Warning: Event channel full, dropping event")
					}
				}
			}(watcher)
		}

		// Keep the goroutine alive
		select {}
	}()

	return eventChan
}

// handleDeviceChange handles device change events
func (s *server) handleDeviceChange(event DeviceEvent) {
	switch event.Type {
	case "interface":
		s.handleInterfaceChange(event)
	case "pci":
		s.handlePciChange(event)
	case "driver":
		s.handleDriverChange(event)
	case "sriov":
		s.checkSriovChanges()
	}
}

// handleInterfaceChange handles network interface changes
func (s *server) handleInterfaceChange(event DeviceEvent) {
	pkg.Info("Interface change detected: %s %s", event.Action, event.Device)
	// Trigger device list refresh
	s.refreshDeviceList()
}

// handlePciChange handles PCI device changes
func (s *server) handlePciChange(event DeviceEvent) {
	pkg.Info("PCI device change detected: %s %s", event.Action, event.Device)
	// Trigger device list refresh
	s.refreshDeviceList()
}

// handleDriverChange handles driver binding changes
func (s *server) handleDriverChange(event DeviceEvent) {
	pkg.Info("Driver binding change detected: %s %s", event.Action, event.Device)
	// Trigger device list refresh
	s.refreshDeviceList()
}

// checkSriovChanges checks for SR-IOV configuration changes
func (s *server) checkSriovChanges() {
	// This would check for SR-IOV specific changes
	// For now, just log that we're checking
	pkg.Debug("Checking SR-IOV configurations...")
}

// watchEthtoolChanges monitors ethtool information changes
func (s *server) watchEthtoolChanges() {
	go func() {
		for {
			s.checkEthtoolChanges()
			time.Sleep(30 * time.Second) // Check every 30 seconds
		}
	}()
	pkg.Info("Ethtool monitoring enabled")
}

// checkEthtoolChanges checks for ethtool information changes
func (s *server) checkEthtoolChanges() {
	s.ethtoolLock.Lock()
	defer s.ethtoolLock.Unlock()

	// Get current devices
	devices, err := pkg.ParseLshwDynamic()
	if err != nil {
		pkg.Error("Error getting PCI devices for ethtool check: %v", err)
		return
	}

	// Attach PCI info
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		pkg.Error("Error attaching PCI info for ethtool check: %v", err)
		return
	}

	for _, device := range devices {
		if device.Name == "" {
			continue // Skip devices without interface names
		}

		// Get current ethtool info
		currentInfo, err := pkg.GetEthtoolInfo(device.Name)
		if err != nil {
			continue // Skip if we can't get ethtool info
		}

		// Check if we have cached info
		cachedInfo, exists := s.ethtoolCache[device.Name]
		if !exists {
			// First time seeing this device
			s.ethtoolCache[device.Name] = currentInfo
			continue
		}

		// Compare current with cached
		if !ethtoolInfoEqual(cachedInfo, currentInfo) {
			pkg.Debug("Ethtool change detected for %s", device.Name)
			s.ethtoolCache[device.Name] = currentInfo
			// Trigger device list refresh
			s.refreshDeviceList()
		}
	}
}

// ethtoolInfoEqual compares two ethtool info structures
func ethtoolInfoEqual(a, b *pkg.EthtoolInfo) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Compare basic fields
	if a.Ring.RxMaxPending != b.Ring.RxMaxPending ||
		a.Ring.TxMaxPending != b.Ring.TxMaxPending ||
		a.Channels.MaxRx != b.Channels.MaxRx ||
		a.Channels.MaxTx != b.Channels.MaxTx {
		return false
	}

	// Compare features
	if len(a.Features) != len(b.Features) {
		return false
	}

	for i, feature := range a.Features {
		if i >= len(b.Features) {
			return false
		}
		if feature.Name != b.Features[i].Name ||
			feature.Enabled != b.Features[i].Enabled {
			return false
		}
	}

	return true
}

// refreshDeviceList refreshes the device list
func (s *server) refreshDeviceList() {
	s.devicesLock.Lock()
	defer s.devicesLock.Unlock()

	pkg.Info("Refreshing device list...")

	// Get current devices
	devices, err := pkg.ParseLshwDynamic()
	if err != nil {
		pkg.Error("Error refreshing devices: %v", err)
		return
	}

	// Attach PCI info
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		pkg.Error("Error attaching PCI info: %v", err)
		return
	}

	// Update device list
	s.devices = devices
	s.lastUpdate = time.Now()

	pkg.Info("Device list refreshed: %d devices found", len(devices))

	// Debug print device information
	debugPrintDeviceInfo(devices)
}

// ListDevices implements the gRPC ListDevices method
func (s *server) ListDevices(ctx context.Context, in *pb.ListDevicesRequest) (*pb.ListDevicesResponse, error) {
	s.devicesLock.RLock()
	defer s.devicesLock.RUnlock()

	// If no devices loaded or devices are stale, refresh
	if len(s.devices) == 0 || time.Since(s.lastUpdate) > 30*time.Second {
		s.devicesLock.RUnlock()
		s.refreshDeviceList()
		s.devicesLock.RLock()
	}

	// Convert devices to protobuf format
	pbDevices := make([]*pb.Device, len(s.devices))
	for i, device := range s.devices {
		pbDevice := &pb.Device{
			PciAddress:   device.PCIAddress,
			Name:         device.Name,
			Driver:       device.Driver,
			Vendor:       device.Vendor,
			Product:      device.Product,
			SriovCapable: device.SRIOVCapable,
			NumaNode:     int32(device.NUMANode),
			NumaDistance: make(map[int32]int32),
		}

		// Add NUMA distance information
		for node, distance := range device.NUMADistance {
			pbDevice.NumaDistance[int32(node)] = int32(distance)
		}

		// Add detailed capabilities if available
		if len(device.DetailedCapabilities) > 0 {
			pbDevice.DetailedCapabilities = make(map[string]*pb.DetailedCapability)
			for name, cap := range device.DetailedCapabilities {
				pbDevice.DetailedCapabilities[name] = &pb.DetailedCapability{
					Id:          cap.ID,
					Name:        cap.Name,
					Status:      cap.Status,
					Description: cap.Description,
					Parameters:  cap.Parameters,
				}
			}
		}

		// Add ethtool information if available
		if device.EthtoolInfo != nil {
			pbDevice.EthtoolInfo = &pb.EthtoolInfo{
				Features: make([]*pb.EthtoolFeature, len(device.EthtoolInfo.Features)),
				Ring: &pb.EthtoolRingInfo{
					RxMaxPending:      device.EthtoolInfo.Ring.RxMaxPending,
					RxMiniMaxPending:  device.EthtoolInfo.Ring.RxMiniMaxPending,
					RxJumboMaxPending: device.EthtoolInfo.Ring.RxJumboMaxPending,
					TxMaxPending:      device.EthtoolInfo.Ring.TxMaxPending,
					RxPending:         device.EthtoolInfo.Ring.RxPending,
					RxMiniPending:     device.EthtoolInfo.Ring.RxMiniPending,
					RxJumboPending:    device.EthtoolInfo.Ring.RxJumboPending,
					TxPending:         device.EthtoolInfo.Ring.TxPending,
				},
				Channels: &pb.EthtoolChannelInfo{
					MaxRx:         device.EthtoolInfo.Channels.MaxRx,
					MaxTx:         device.EthtoolInfo.Channels.MaxTx,
					MaxOther:      device.EthtoolInfo.Channels.MaxOther,
					MaxCombined:   device.EthtoolInfo.Channels.MaxCombined,
					RxCount:       device.EthtoolInfo.Channels.RxCount,
					TxCount:       device.EthtoolInfo.Channels.TxCount,
					OtherCount:    device.EthtoolInfo.Channels.OtherCount,
					CombinedCount: device.EthtoolInfo.Channels.CombinedCount,
				},
			}

			for j, feature := range device.EthtoolInfo.Features {
				pbDevice.EthtoolInfo.Features[j] = &pb.EthtoolFeature{
					Name:    feature.Name,
					Enabled: feature.Enabled,
					Fixed:   feature.Fixed,
				}
			}
		}

		pbDevices[i] = pbDevice
	}

	return &pb.ListDevicesResponse{
		Devices: pbDevices,
	}, nil
}

// RefreshDevices implements the gRPC RefreshDevices method
func (s *server) RefreshDevices(ctx context.Context, in *pb.RefreshDevicesRequest) (*pb.RefreshDevicesResponse, error) {
	s.refreshDeviceList()

	return &pb.RefreshDevicesResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully refreshed %d devices", len(s.devices)),
	}, nil
}

// debugPrintDeviceInfo prints detailed device information for debugging
func debugPrintDeviceInfo(devices []pkg.Device) {
	pkg.Debug("=== Device Information ===")
	for i, device := range devices {
		pkg.Debug("Device %d:", i+1)
		pkg.Debug("  PCI Address: %s", device.PCIAddress)
		pkg.Debug("  Name: %s", device.Name)
		pkg.Debug("  Driver: %s", device.Driver)
		pkg.Debug("  Vendor: %s", device.Vendor)
		pkg.Debug("  Product: %s", device.Product)
		pkg.Debug("  SR-IOV Capable: %t", device.SRIOVCapable)

		// NUMA topology information
		if device.NUMANode != -1 {
			pkg.Debug("  NUMA Node: %d", device.NUMANode)
			if len(device.NUMADistance) > 0 {
				var distances []string
				for node, distance := range device.NUMADistance {
					distances = append(distances, fmt.Sprintf("Node %d: %d", node, distance))
				}
				pkg.Debug("  NUMA Distances: %s", strings.Join(distances, ", "))
			}
		} else {
			pkg.Debug("  NUMA: No affinity")
		}

		// Detailed capabilities
		if len(device.DetailedCapabilities) > 0 {
			pkg.Debug("  Detailed Capabilities:")
			for name, cap := range device.DetailedCapabilities {
				pkg.Debug("    %s: %s (%s)", name, cap.Name, cap.Status)
				if cap.Description != "" {
					pkg.Debug("      Description: %s", cap.Description)
				}
				if len(cap.Parameters) > 0 {
					var params []string
					for k, v := range cap.Parameters {
						params = append(params, fmt.Sprintf("%s=%s", k, v))
					}
					pkg.Debug("      Parameters: %s", strings.Join(params, ", "))
				}
			}
		}

		// Ethtool information
		if device.EthtoolInfo != nil {
			pkg.Debug("  Ethtool Information:")
			pkg.Debug("    Ring Buffer:")
			pkg.Debug("      RX Max Pending: %d", device.EthtoolInfo.Ring.RxMaxPending)
			pkg.Debug("      TX Max Pending: %d", device.EthtoolInfo.Ring.TxMaxPending)
			pkg.Debug("      RX Pending: %d", device.EthtoolInfo.Ring.RxPending)
			pkg.Debug("      TX Pending: %d", device.EthtoolInfo.Ring.TxPending)

			pkg.Debug("    Channels:")
			pkg.Debug("      Max RX: %d", device.EthtoolInfo.Channels.MaxRx)
			pkg.Debug("      Max TX: %d", device.EthtoolInfo.Channels.MaxTx)
			pkg.Debug("      Max Combined: %d", device.EthtoolInfo.Channels.MaxCombined)
			pkg.Debug("      RX Count: %d", device.EthtoolInfo.Channels.RxCount)
			pkg.Debug("      TX Count: %d", device.EthtoolInfo.Channels.TxCount)
			pkg.Debug("      Combined Count: %d", device.EthtoolInfo.Channels.CombinedCount)

			if len(device.EthtoolInfo.Features) > 0 {
				pkg.Debug("    Features:")
				for _, feature := range device.EthtoolInfo.Features {
					status := "disabled"
					if feature.Enabled {
						status = "enabled"
					}
					pkg.Debug("      %s: %s", feature.Name, status)
				}
			}
		}

		pkg.Debug("")
	}
	pkg.Debug("=== End Device Information ===")
}
