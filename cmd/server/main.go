package main

import (
	"context"
	"flag"
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
	"google.golang.org/grpc"
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
						pkg.WithError(err).Warn("Network interface monitoring error")
						time.Sleep(5 * time.Second) // Retry after delay
					}
				}
			}
		}()
		pkg.Debug("Network interface monitoring enabled")
	} else {
		pkg.Debug("Network interface directory not available, skipping monitoring")
	}
}

// watchPciDevices monitors /sys/bus/pci/devices for PCI changes
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
						pkg.WithError(err).Warn("PCI device monitoring error")
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		pkg.Debug("PCI device monitoring enabled")
	} else {
		pkg.Debug("PCI devices directory not available, skipping monitoring")
	}
}

// watchDriverBindings monitors driver binding changes
func (s *server) watchDriverBindings() {
	// Monitor vfio driver specifically
	if _, err := os.Stat("/sys/bus/pci/drivers/vfio-pci"); err == nil {
		vfioWatcher := &DeviceWatcher{
			path:     "/sys/bus/pci/drivers/vfio-pci",
			events:   make(chan DeviceEvent, 100),
			stopChan: make(chan struct{}),
		}
		s.watchers = append(s.watchers, vfioWatcher)

		go func() {
			for {
				select {
				case <-vfioWatcher.stopChan:
					return
				default:
					if err := s.monitorDirectory(vfioWatcher.path, "driver", vfioWatcher.events); err != nil {
						pkg.WithError(err).Warn("VFIO driver monitoring error")
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		pkg.Debug("VFIO driver monitoring enabled")
	} else {
		pkg.Debug("VFIO driver not available, skipping monitoring")
	}

	// Monitor Mellanox driver (mlx5_core is the actual driver being used)
	if _, err := os.Stat("/sys/bus/pci/drivers/mlx5_core"); err == nil {
		mlxWatcher := &DeviceWatcher{
			path:     "/sys/bus/pci/drivers/mlx5_core",
			events:   make(chan DeviceEvent, 100),
			stopChan: make(chan struct{}),
		}
		s.watchers = append(s.watchers, mlxWatcher)

		go func() {
			for {
				select {
				case <-mlxWatcher.stopChan:
					return
				default:
					if err := s.monitorDirectory(mlxWatcher.path, "driver", mlxWatcher.events); err != nil {
						pkg.WithError(err).Warn("Mellanox driver monitoring error")
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		pkg.Debug("Mellanox driver monitoring enabled")
	} else {
		pkg.Debug("Mellanox driver not available, skipping monitoring")
	}
}

// watchSriovConfigurations monitors SR-IOV VF count changes
func (s *server) watchSriovConfigurations() {
	// Monitor all PCI devices for sriov_numvfs changes
	go func() {
		for {
			time.Sleep(10 * time.Second) // Check every 10 seconds
			s.checkSriovChanges()
		}
	}()
}

// monitorDirectory uses fsnotify to monitor directory changes
func (s *server) monitorDirectory(path, eventType string, events chan<- DeviceEvent) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	// Add the directory to watch
	err = watcher.Add(path)
	if err != nil {
		return fmt.Errorf("failed to add directory to watcher: %v", err)
	}

	// Process events
	for {
		select {
		case event := <-watcher.Events:
			// Only process events for files (not directories)
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Write) {
				deviceName := filepath.Base(event.Name)
				var action string

				if event.Has(fsnotify.Create) {
					action = "created"
				} else if event.Has(fsnotify.Remove) {
					action = "deleted"
				} else if event.Has(fsnotify.Write) {
					action = "modified"
				}

				events <- DeviceEvent{
					Type:      eventType,
					Action:    action,
					Device:    deviceName,
					Timestamp: time.Now(),
				}
			}
		case err := <-watcher.Errors:
			pkg.WithField("path", path).WithError(err).Debug("fsnotify error")
			return err
		}
	}
}

// processDeviceEvents handles device change events
func (s *server) processDeviceEvents() {
	for {
		select {
		case event := <-s.collectEvents():
			pkg.WithFields(map[string]interface{}{
				"type":   event.Type,
				"action": event.Action,
				"device": event.Device,
			}).Debug("Device change detected")
			s.handleDeviceChange(event)
		}
	}
}

// collectEvents collects events from all watchers
func (s *server) collectEvents() <-chan DeviceEvent {
	events := make(chan DeviceEvent, 100)

	for _, watcher := range s.watchers {
		go func(w *DeviceWatcher) {
			for event := range w.events {
				events <- event
			}
		}(watcher)
	}

	return events
}

// handleDeviceChange processes device change events
func (s *server) handleDeviceChange(event DeviceEvent) {
	switch event.Type {
	case "interface":
		s.handleInterfaceChange(event)
	case "pci":
		s.handlePciChange(event)
	case "driver":
		s.handleDriverChange(event)
	}
}

// handleInterfaceChange handles network interface changes
func (s *server) handleInterfaceChange(event DeviceEvent) {
	if event.Action == "created" || event.Action == "deleted" {
		pkg.WithField("action", event.Action).WithField("device", event.Device).Debug("Network interface change, refreshing device list")
		s.refreshDeviceList()
	}
}

// handlePciChange handles PCI device changes
func (s *server) handlePciChange(event DeviceEvent) {
	pkg.WithField("action", event.Action).WithField("device", event.Device).Debug("PCI device change, refreshing device list")
	s.refreshDeviceList()
}

// handleDriverChange handles driver binding changes
func (s *server) handleDriverChange(event DeviceEvent) {
	pkg.WithField("action", event.Action).WithField("device", event.Device).Debug("Driver binding change, refreshing device list")
	s.refreshDeviceList()
}

// checkSriovChanges checks for SR-IOV configuration changes
func (s *server) checkSriovChanges() {
	// This is a simplified check - in practice you'd want to cache previous values
	// and compare to detect actual changes
	pkg.Debug("Checking SR-IOV configurations...")
	// Implementation would check sriov_numvfs files for changes
}

// watchEthtoolChanges monitors ethtool parameters for changes
func (s *server) watchEthtoolChanges() {
	pkg.Debug("Starting ethtool parameter monitoring...")

	go func() {
		ticker := time.NewTicker(15 * time.Second) // Check every 15 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.checkEthtoolChanges()
			}
		}
	}()
}

// checkEthtoolChanges compares current ethtool values with cached values
func (s *server) checkEthtoolChanges() {
	s.devicesLock.RLock()
	devices := make([]pkg.Device, len(s.devices))
	copy(devices, s.devices)
	s.devicesLock.RUnlock()

	changed := false
	for _, device := range devices {
		if device.LogicalName == "" {
			continue
		}

		// Get current ethtool info
		currentEthtool, err := pkg.GetEthtoolInfo(device.LogicalName)
		if err != nil {
			continue // Skip devices that don't support ethtool
		}

		// Check if we have cached values
		s.ethtoolLock.RLock()
		cachedEthtool, exists := s.ethtoolCache[device.LogicalName]
		s.ethtoolLock.RUnlock()

		if !exists {
			// First time seeing this device, cache the values
			s.ethtoolLock.Lock()
			s.ethtoolCache[device.LogicalName] = currentEthtool
			s.ethtoolLock.Unlock()
			continue
		}

		// Compare ring buffer values
		if cachedEthtool.Ring.RxPending != currentEthtool.Ring.RxPending ||
			cachedEthtool.Ring.TxPending != currentEthtool.Ring.TxPending {

			pkg.WithFields(map[string]interface{}{
				"device": device.LogicalName,
				"rx_old": cachedEthtool.Ring.RxPending,
				"rx_new": currentEthtool.Ring.RxPending,
				"tx_old": cachedEthtool.Ring.TxPending,
				"tx_new": currentEthtool.Ring.TxPending,
			}).Debug("Ethtool ring buffer change detected")

			changed = true
		}

		// Compare channel values
		if cachedEthtool.Channels.RxCount != currentEthtool.Channels.RxCount ||
			cachedEthtool.Channels.TxCount != currentEthtool.Channels.TxCount ||
			cachedEthtool.Channels.CombinedCount != currentEthtool.Channels.CombinedCount {

			pkg.WithFields(map[string]interface{}{
				"device":       device.LogicalName,
				"rx_old":       cachedEthtool.Channels.RxCount,
				"rx_new":       currentEthtool.Channels.RxCount,
				"tx_old":       cachedEthtool.Channels.TxCount,
				"tx_new":       currentEthtool.Channels.TxCount,
				"combined_old": cachedEthtool.Channels.CombinedCount,
				"combined_new": currentEthtool.Channels.CombinedCount,
			}).Debug("Ethtool channel change detected")

			changed = true
		}

		// Update cache with current values
		s.ethtoolLock.Lock()
		s.ethtoolCache[device.LogicalName] = currentEthtool
		s.ethtoolLock.Unlock()
	}

	if changed {
		pkg.Debug("Ethtool changes detected, refreshing device list...")
		s.refreshDeviceList()
	}
}

// refreshDeviceList refreshes the device list
func (s *server) refreshDeviceList() {
	pkg.Debug("Refreshing device list...")

	s.devicesLock.Lock()
	defer s.devicesLock.Unlock()

	// Re-collect all device information
	devices, err := pkg.ParseLshwDynamic()
	if err != nil {
		pkg.WithError(err).Error("Failed to refresh devices")
		return
	}

	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		pkg.WithError(err).Warn("Failed to refresh PCI info")
	}

	devices, err = pkg.AttachEthtoolInfo(devices)
	if err != nil {
		pkg.WithError(err).Warn("Failed to refresh ethtool info")
	}

	s.devices = devices
	s.lastUpdate = time.Now()

	// Update ethtool cache with new values
	s.ethtoolLock.Lock()
	for _, device := range devices {
		if device.LogicalName != "" && device.EthtoolInfo != nil {
			s.ethtoolCache[device.LogicalName] = device.EthtoolInfo
		}
	}
	s.ethtoolLock.Unlock()

	pkg.WithField("device_count", len(devices)).Debug("Device list refreshed")
}

func (s *server) ListDevices(ctx context.Context, in *pb.ListDevicesRequest) (*pb.ListDevicesResponse, error) {
	s.devicesLock.RLock()
	defer s.devicesLock.RUnlock()

	pkg.WithField("device_count", len(s.devices)).Info("ListDevices request received")
	resp := &pb.ListDevicesResponse{}
	for _, d := range s.devices {
		device := &pb.Device{
			PciAddress:   d.PCIAddress,
			Name:         d.Name,
			Driver:       d.Driver,
			Vendor:       d.Vendor,
			Product:      d.Product,
			SriovCapable: d.SRIOVCapable,
			// Add NUMA topology information
			NumaNode:     int32(d.NUMANode),
			NumaDistance: make(map[int32]int32),
		}

		// Add NUMA distance information
		for node, distance := range d.NUMADistance {
			device.NumaDistance[int32(node)] = int32(distance)
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

		// Add ethtool information
		if d.EthtoolInfo != nil {
			pkg.WithField("device", d.Name).Debug("Adding ethtool info for device")
			ethtoolInfo := &pb.EthtoolInfo{}

			// Add features
			for _, feature := range d.EthtoolInfo.Features {
				ethtoolInfo.Features = append(ethtoolInfo.Features, &pb.EthtoolFeature{
					Name:    feature.Name,
					Enabled: feature.Enabled,
					Fixed:   feature.Fixed,
				})
			}

			// Add ring information
			ethtoolInfo.Ring = &pb.EthtoolRingInfo{
				RxMaxPending:      d.EthtoolInfo.Ring.RxMaxPending,
				RxMiniMaxPending:  d.EthtoolInfo.Ring.RxMiniMaxPending,
				RxJumboMaxPending: d.EthtoolInfo.Ring.RxJumboMaxPending,
				TxMaxPending:      d.EthtoolInfo.Ring.TxMaxPending,
				RxPending:         d.EthtoolInfo.Ring.RxPending,
				RxMiniPending:     d.EthtoolInfo.Ring.RxMiniPending,
				RxJumboPending:    d.EthtoolInfo.Ring.RxJumboPending,
				TxPending:         d.EthtoolInfo.Ring.TxPending,
			}

			// Debug: print ring values
			pkg.WithFields(map[string]interface{}{
				"device":     d.Name,
				"rx_pending": ethtoolInfo.Ring.RxPending,
				"tx_pending": ethtoolInfo.Ring.TxPending,
			}).Debug("Ring values for device")

			// Add channel information
			ethtoolInfo.Channels = &pb.EthtoolChannelInfo{
				MaxRx:         d.EthtoolInfo.Channels.MaxRx,
				MaxTx:         d.EthtoolInfo.Channels.MaxTx,
				MaxOther:      d.EthtoolInfo.Channels.MaxOther,
				MaxCombined:   d.EthtoolInfo.Channels.MaxCombined,
				RxCount:       d.EthtoolInfo.Channels.RxCount,
				TxCount:       d.EthtoolInfo.Channels.TxCount,
				OtherCount:    d.EthtoolInfo.Channels.OtherCount,
				CombinedCount: d.EthtoolInfo.Channels.CombinedCount,
			}

			device.EthtoolInfo = ethtoolInfo
			pkg.WithFields(map[string]interface{}{
				"device":        d.Name,
				"feature_count": len(ethtoolInfo.Features),
				"is_nil":        device.EthtoolInfo == nil,
			}).Debug("Successfully set ethtool info for device")
		} else {
			pkg.WithField("device", d.Name).Debug("No ethtool info for device")
		}

		resp.Devices = append(resp.Devices, device)
	}
	return resp, nil
}

// RefreshDevices manually refreshes the device list
func (s *server) RefreshDevices(ctx context.Context, in *pb.RefreshDevicesRequest) (*pb.RefreshDevicesResponse, error) {
	pkg.Info("Manual refresh requested")

	s.refreshDeviceList()

	s.devicesLock.RLock()
	deviceCount := len(s.devices)
	s.devicesLock.RUnlock()

	return &pb.RefreshDevicesResponse{
		Success:     true,
		Message:     fmt.Sprintf("Device list refreshed successfully. %d devices found.", deviceCount),
		DeviceCount: int32(deviceCount),
	}, nil
}

// debugPrintDeviceInfo prints detailed device information for debugging
func debugPrintDeviceInfo(devices []pkg.Device) {
	pkg.WithField("total_devices", len(devices)).Debug("Device Information Collection Summary")

	sriovCount := 0
	networkCount := 0

	for i, device := range devices {
		// Show progress for network devices during discovery
		if device.Name != "" {
			pkg.WithFields(map[string]interface{}{
				"device_index": i + 1,
				"name":         device.Name,
				"pci":          device.PCIAddress,
				"vendor":       device.Vendor,
				"product":      device.Product,
				"sriov":        device.SRIOVCapable,
			}).Debug("Processing network device")
		}

		// Enhanced context information (debug only)
		if device.Description != "" {
			pkg.WithField("description", device.Description).Debug("Device description")
		}
		if device.Serial != "" {
			pkg.WithField("serial", device.Serial).Debug("Device serial")
		}
		if device.Size != "" {
			pkg.WithField("size", device.Size).Debug("Device size")
		}
		if device.Capacity != "" {
			pkg.WithField("capacity", device.Capacity).Debug("Device capacity")
		}
		if device.Clock != "" {
			pkg.WithField("clock", device.Clock).Debug("Device clock")
		}
		if device.Width != "" {
			pkg.WithField("width", device.Width).Debug("Device width")
		}
		if device.Class != "" {
			pkg.WithField("class", device.Class).Debug("Device class")
		}
		if device.SubClass != "" {
			pkg.WithField("subclass", device.SubClass).Debug("Device subclass")
		}
		if len(device.Capabilities) > 0 {
			pkg.WithField("capabilities", device.Capabilities).Debug("Device capabilities")
		}

		// NUMA topology information (debug only)
		if device.NUMANode != -1 {
			pkg.WithField("numa_node", device.NUMANode).Debug("Device NUMA node")
			if len(device.NUMADistance) > 0 {
				var distances []string
				for node, distance := range device.NUMADistance {
					distances = append(distances, fmt.Sprintf("Node %d: %d", node, distance))
				}
				pkg.WithField("numa_distances", strings.Join(distances, ", ")).Debug("Device NUMA distances")
			}
		} else {
			pkg.Debug("Device has no NUMA affinity")
		}

		// Detailed capability information (debug only)
		if len(device.DetailedCapabilities) > 0 {
			pkg.Debug("Device detailed capabilities:")
			for capName, cap := range device.DetailedCapabilities {
				pkg.WithFields(map[string]interface{}{
					"capability_id":   cap.ID,
					"capability_name": capName,
					"description":     cap.Description,
				}).Debug("Device capability")
			}
		}

		if device.SRIOVCapable && device.SRIOVInfo != nil {
			sriovCount++
			pkg.WithFields(map[string]interface{}{
				"total_vfs":     device.SRIOVInfo.TotalVFs,
				"number_of_vfs": device.SRIOVInfo.NumberOfVFs,
				"vf_offset":     device.SRIOVInfo.VFOffset,
				"vf_stride":     device.SRIOVInfo.VFStride,
				"vf_device_id":  device.SRIOVInfo.VFDeviceID,
			}).Debug("Device SR-IOV details")
		}

		if device.Name != "" {
			networkCount++
		}
	}

	pkg.WithFields(map[string]interface{}{
		"network_devices":   networkCount,
		"sriov_devices":     sriovCount,
		"non_sriov_devices": len(devices) - sriovCount,
	}).Debug("Device discovery summary")
}

func main() {
	// Parse command line flags
	var (
		useFile  = flag.Bool("file", false, "Use static lshw file for testing (default: dynamic lshw)")
		lshwFile = flag.String("lshw-file", "lshw-network.json", "Path to lshw JSON file (when using -file)")
		debug    = flag.Bool("debug", false, "Enable debug logging")
	)
	flag.Parse()

	// Configure logging level based on debug flag
	if *debug {
		pkg.SetLogLevelFromString("debug")
	} else {
		pkg.SetLogLevelFromString("info")
	}

	startTime := time.Now()
	pkg.Info("Starting SR-IOV Manager Server...")

	var devices []pkg.Device
	var err error

	if *useFile {
		// Development/Testing mode: Use static file
		pkg.WithField("file", *lshwFile).Info("Development mode: Parsing lshw data from file")
		if _, err := os.Stat(*lshwFile); os.IsNotExist(err) {
			pkg.WithField("file", *lshwFile).Warn("File not found, using empty device list")
			devices = []pkg.Device{}
		} else {
			devices, err = pkg.ParseLshwFromFile(*lshwFile)
			if err != nil {
				pkg.WithField("file", *lshwFile).WithError(err).Error("Failed to parse lshw file")
				pkg.Info("Falling back to empty device list...")
				devices = []pkg.Device{}
			} else {
				pkg.WithField("count", len(devices)).Info("Successfully parsed devices from lshw file")
			}
		}
	} else {
		// Production mode: Run lshw dynamically
		pkg.Info("Production mode: Running lshw -class network -json dynamically")
		lshwStart := time.Now()
		devices, err = pkg.ParseLshwDynamic()
		if err != nil {
			pkg.WithError(err).Error("Failed to run lshw")
			pkg.Info("Falling back to empty device list...")
			devices = []pkg.Device{}
		} else {
			pkg.WithField("count", len(devices)).WithField("duration", time.Since(lshwStart)).Info("Successfully gathered devices from lshw")
		}
	}

	pkg.Info("Enriching devices with PCI information...")
	enrichStart := time.Now()
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		pkg.WithError(err).Warn("Failed to enrich devices with PCI info")
		pkg.Info("Continuing with basic device information...")
	} else {
		pkg.WithField("duration", time.Since(enrichStart)).Info("PCI enrichment completed")
	}

	pkg.Info("Enriching devices with ethtool information...")
	ethtoolStart := time.Now()
	devices, err = pkg.AttachEthtoolInfo(devices)
	if err != nil {
		pkg.WithError(err).Warn("Failed to enrich devices with ethtool info")
		pkg.Info("Continuing without ethtool information...")
	} else {
		pkg.WithField("duration", time.Since(ethtoolStart)).Info("Ethtool enrichment completed")
		// Debug: count devices with ethtool info
		ethtoolCount := 0
		for _, d := range devices {
			if d.EthtoolInfo != nil {
				ethtoolCount++
			}
		}
		pkg.WithField("ethtool_count", ethtoolCount).WithField("total_count", len(devices)).Debug("Devices with ethtool info")
	}

	// Print detailed device information for debugging (only in debug mode)
	if *debug {
		debugPrintDeviceInfo(devices)
	}

	pkg.Info("Starting gRPC server...")
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		pkg.WithError(err).Fatal("Failed to listen")
	}

	grpcServer := grpc.NewServer()
	srv := &server{devices: devices}
	pb.RegisterSRIOVManagerServer(grpcServer, srv)

	totalStartupTime := time.Since(startTime)
	pkg.WithField("port", ":50051").Info("SR-IOV manager gRPC server ready")
	pkg.WithField("startup_time", totalStartupTime).Info("Server startup completed")
	pkg.Info("Server is ready to accept connections...")

	// Start monitoring
	srv.StartDeviceMonitoring()

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			pkg.WithError(err).Fatal("Failed to serve")
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	pkg.Info("Shutting down SR-IOV Manager Server...")

	// Clean up watchers
	for _, watcher := range srv.watchers {
		close(watcher.stopChan)
	}

	// Graceful shutdown of gRPC server
	grpcServer.GracefulStop()
	pkg.Info("Server shutdown complete")
}
