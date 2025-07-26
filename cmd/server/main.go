package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
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
	log.Printf("Starting device change monitoring...")

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
						log.Printf("Warning: Network interface monitoring error: %v", err)
						time.Sleep(5 * time.Second) // Retry after delay
					}
				}
			}
		}()
		log.Printf("Network interface monitoring enabled")
	} else {
		log.Printf("Network interface directory not available, skipping monitoring")
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
						log.Printf("Warning: PCI device monitoring error: %v", err)
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		log.Printf("PCI device monitoring enabled")
	} else {
		log.Printf("PCI devices directory not available, skipping monitoring")
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
						log.Printf("Warning: VFIO driver monitoring error: %v", err)
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		log.Printf("VFIO driver monitoring enabled")
	} else {
		log.Printf("VFIO driver not available, skipping monitoring")
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
						log.Printf("Warning: Mellanox driver monitoring error: %v", err)
						time.Sleep(5 * time.Second)
					}
				}
			}
		}()
		log.Printf("Mellanox driver monitoring enabled")
	} else {
		log.Printf("Mellanox driver not available, skipping monitoring")
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
			log.Printf("fsnotify error for %s: %v", path, err)
			return err
		}
	}
}

// processDeviceEvents handles device change events
func (s *server) processDeviceEvents() {
	for {
		select {
		case event := <-s.collectEvents():
			log.Printf("Device change detected: %s %s %s", event.Type, event.Action, event.Device)
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
		log.Printf("Network interface %s %s, refreshing device list", event.Action, event.Device)
		s.refreshDeviceList()
	}
}

// handlePciChange handles PCI device changes
func (s *server) handlePciChange(event DeviceEvent) {
	log.Printf("PCI device %s %s, refreshing device list", event.Action, event.Device)
	s.refreshDeviceList()
}

// handleDriverChange handles driver binding changes
func (s *server) handleDriverChange(event DeviceEvent) {
	log.Printf("Driver binding change: %s %s, refreshing device list", event.Action, event.Device)
	s.refreshDeviceList()
}

// checkSriovChanges checks for SR-IOV configuration changes
func (s *server) checkSriovChanges() {
	// This is a simplified check - in practice you'd want to cache previous values
	// and compare to detect actual changes
	log.Printf("Checking SR-IOV configurations...")
	// Implementation would check sriov_numvfs files for changes
}

// watchEthtoolChanges monitors ethtool parameters for changes
func (s *server) watchEthtoolChanges() {
	log.Printf("Starting ethtool parameter monitoring...")

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

			log.Printf("Ethtool ring buffer change detected for %s: RX %d->%d, TX %d->%d",
				device.LogicalName,
				cachedEthtool.Ring.RxPending, currentEthtool.Ring.RxPending,
				cachedEthtool.Ring.TxPending, currentEthtool.Ring.TxPending)

			changed = true
		}

		// Compare channel values
		if cachedEthtool.Channels.RxCount != currentEthtool.Channels.RxCount ||
			cachedEthtool.Channels.TxCount != currentEthtool.Channels.TxCount ||
			cachedEthtool.Channels.CombinedCount != currentEthtool.Channels.CombinedCount {

			log.Printf("Ethtool channel change detected for %s: RX %d->%d, TX %d->%d, Combined %d->%d",
				device.LogicalName,
				cachedEthtool.Channels.RxCount, currentEthtool.Channels.RxCount,
				cachedEthtool.Channels.TxCount, currentEthtool.Channels.TxCount,
				cachedEthtool.Channels.CombinedCount, currentEthtool.Channels.CombinedCount)

			changed = true
		}

		// Update cache with current values
		s.ethtoolLock.Lock()
		s.ethtoolCache[device.LogicalName] = currentEthtool
		s.ethtoolLock.Unlock()
	}

	if changed {
		log.Printf("Ethtool changes detected, refreshing device list...")
		s.refreshDeviceList()
	}
}

// refreshDeviceList refreshes the device list
func (s *server) refreshDeviceList() {
	log.Printf("Refreshing device list...")

	s.devicesLock.Lock()
	defer s.devicesLock.Unlock()

	// Re-collect all device information
	devices, err := pkg.ParseLshwDynamic()
	if err != nil {
		log.Printf("Error: Failed to refresh devices: %v", err)
		return
	}

	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		log.Printf("Warning: Failed to refresh PCI info: %v", err)
	}

	devices, err = pkg.AttachEthtoolInfo(devices)
	if err != nil {
		log.Printf("Warning: Failed to refresh ethtool info: %v", err)
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

	log.Printf("Device list refreshed: %d devices", len(devices))
}

func (s *server) ListDevices(ctx context.Context, in *pb.ListDevicesRequest) (*pb.ListDevicesResponse, error) {
	s.devicesLock.RLock()
	defer s.devicesLock.RUnlock()

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

		// Add ethtool information
		if d.EthtoolInfo != nil {
			fmt.Printf("DEBUG: Adding ethtool info for device %s\n", d.Name)
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
			fmt.Printf("DEBUG: Ring values - RxPending: %d, TxPending: %d\n",
				ethtoolInfo.Ring.RxPending, ethtoolInfo.Ring.TxPending)

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
			fmt.Printf("DEBUG: Successfully set ethtool info for device %s with %d features\n", d.Name, len(ethtoolInfo.Features))
			fmt.Printf("DEBUG: Device.EthtoolInfo is nil: %t\n", device.EthtoolInfo == nil)
		} else {
			fmt.Printf("DEBUG: No ethtool info for device %s\n", d.Name)
		}

		resp.Devices = append(resp.Devices, device)
	}
	return resp, nil
}

// RefreshDevices manually refreshes the device list
func (s *server) RefreshDevices(ctx context.Context, in *pb.RefreshDevicesRequest) (*pb.RefreshDevicesResponse, error) {
	log.Printf("Manual refresh requested")

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
	log.Printf("Device Information Collection Summary:")
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
	log.Printf("Starting SR-IOV Manager Server...")

	var devices []pkg.Device
	var err error

	if *useFile {
		// Development/Testing mode: Use static file
		log.Printf("Development mode: Parsing lshw data from file: %s", *lshwFile)
		if _, err := os.Stat(*lshwFile); os.IsNotExist(err) {
			log.Printf("Warning: %s not found, using empty device list", *lshwFile)
			devices = []pkg.Device{}
		} else {
			devices, err = pkg.ParseLshwFromFile(*lshwFile)
			if err != nil {
				log.Printf("Error: Failed to parse lshw file: %v", err)
				log.Printf("Falling back to empty device list...")
				devices = []pkg.Device{}
			} else {
				log.Printf("Successfully parsed %d devices from lshw file", len(devices))
			}
		}
	} else {
		// Production mode: Run lshw dynamically
		log.Printf("Production mode: Running lshw -class network -json dynamically")
		lshwStart := time.Now()
		devices, err = pkg.ParseLshwDynamic()
		if err != nil {
			log.Printf("Error: Failed to run lshw: %v", err)
			log.Printf("Falling back to empty device list...")
			devices = []pkg.Device{}
		} else {
			log.Printf("Successfully gathered %d devices from lshw in %v", len(devices), time.Since(lshwStart))
		}
	}

	log.Printf("Enriching devices with PCI information...")
	enrichStart := time.Now()
	devices, err = pkg.AttachPciInfo(devices)
	if err != nil {
		log.Printf("Warning: failed to enrich devices with PCI info: %v", err)
		log.Printf("   Continuing with basic device information...")
	} else {
		log.Printf("PCI enrichment completed in %v", time.Since(enrichStart))
	}

	log.Printf("Enriching devices with ethtool information...")
	ethtoolStart := time.Now()
	devices, err = pkg.AttachEthtoolInfo(devices)
	if err != nil {
		log.Printf("Warning: failed to enrich devices with ethtool info: %v", err)
		log.Printf("   Continuing without ethtool information...")
	} else {
		log.Printf("Ethtool enrichment completed in %v", time.Since(ethtoolStart))
		// Debug: count devices with ethtool info
		ethtoolCount := 0
		for _, d := range devices {
			if d.EthtoolInfo != nil {
				ethtoolCount++
			}
		}
		log.Printf("Devices with ethtool info: %d/%d", ethtoolCount, len(devices))
	}

	// Print detailed device information for debugging
	debugPrintDeviceInfo(devices)

	log.Printf("Starting gRPC server...")
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Error: Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	srv := &server{devices: devices}
	pb.RegisterSRIOVManagerServer(grpcServer, srv)

	totalStartupTime := time.Since(startTime)
	log.Printf("SR-IOV manager gRPC server ready on :50051")
	log.Printf("Total startup time: %v", totalStartupTime)
	log.Printf("Server is ready to accept connections...")

	// Start monitoring
	srv.StartDeviceMonitoring()

	// Start gRPC server in a goroutine
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Error: Failed to serve: %v", err)
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutting down SR-IOV Manager Server...")

	// Clean up watchers
	for _, watcher := range srv.watchers {
		close(watcher.stopChan)
	}

	// Graceful shutdown of gRPC server
	grpcServer.GracefulStop()
	log.Printf("Server shutdown complete")
}
