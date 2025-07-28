package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// fsMonitor handles file system monitoring for SR-IOV device changes
type fsMonitor struct {
	watcher *fsnotify.Watcher
	server  *server
	logger  *logrus.Logger
	stopCh  chan struct{}
}

// newFSMonitor creates a new file system monitor
func newFSMonitor(s *server) (*fsMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &fsMonitor{
		watcher: watcher,
		server:  s,
		logger:  s.logger,
		stopCh:  make(chan struct{}),
	}, nil
}

// start begins monitoring SR-IOV sysfs directories
func (fm *fsMonitor) start() error {
	fm.logger.Info("Starting optimized file system monitoring for SR-IOV changes")

	// Watch the main PCI devices directory for new devices
	if err := fm.watcher.Add("/sys/bus/pci/devices"); err != nil {
		return err
	}

	// Watch existing SR-IOV capable devices (PFs only)
	if err := fm.watchExistingSRIOVDevices(); err != nil {
		return err
	}

	// Start the event processing goroutine
	go fm.processEvents()

	return nil
}

// stop stops the file system monitoring
func (fm *fsMonitor) stop() {
	fm.logger.Info("Stopping file system monitoring")
	close(fm.stopCh)
	fm.watcher.Close()
}

// watchExistingSRIOVDevices adds watches for existing SR-IOV capable devices (PFs only)
func (fm *fsMonitor) watchExistingSRIOVDevices() error {
	devices, err := filepath.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		return err
	}

	for _, device := range devices {
		pciAddr := filepath.Base(device)

		// Check if device supports SR-IOV
		sriovTotalPath := filepath.Join(device, "sriov_totalvfs")
		if _, err := os.Stat(sriovTotalPath); err == nil {
			// Get vendor and device IDs for filtering
			vendorID := getVendorID(pciAddr)
			deviceID := getDeviceID(pciAddr)
			interfaceName := getInterfaceName(pciAddr)
			vendorInfo := getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr))

			// Check if vendor is allowed based on configuration
			if fm.server.config != nil && !fm.server.config.IsVendorAllowed(vendorID) {
				fm.logger.WithFields(logrus.Fields{
					"device":      pciAddr,
					"interface":   interfaceName,
					"vendor_id":   vendorID,
					"vendor_name": vendorInfo.VendorName,
					"device_id":   deviceID,
					"device_name": vendorInfo.DeviceName,
				}).Debug("skipping file system watch - vendor not allowed")
				continue
			}

			// Watch only the specific files that can change in the PF directory
			if err := fm.watchPFDirectory(device, pciAddr); err != nil {
				fm.logger.WithError(err).WithField("device", device).Warn("failed to watch PF directory")
			}
		}
	}

	return nil
}

// watchPFDirectory watches only the specific files in a PF directory that can change
func (fm *fsMonitor) watchPFDirectory(devicePath, pciAddr string) error {
	// Get device information for enhanced logging
	interfaceName := getInterfaceName(pciAddr)
	vendorID := getVendorID(pciAddr)
	deviceID := getDeviceID(pciAddr)
	vendorInfo := getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr))

	// Watch the PF directory itself for new VF directories being created/removed
	if err := fm.watcher.Add(devicePath); err != nil {
		return err
	}
	fm.logger.WithFields(logrus.Fields{
		"device":      pciAddr,
		"interface":   interfaceName,
		"vendor_id":   vendorID,
		"vendor_name": vendorInfo.VendorName,
		"device_id":   deviceID,
		"device_name": vendorInfo.DeviceName,
	}).Debug("added watch for PF directory")

	// Watch specific files that can change in the PF
	criticalFiles := []string{
		"sriov_numvfs",   // Number of VFs enabled
		"sriov_totalvfs", // Total VFs supported (can change on driver reload)
	}

	for _, file := range criticalFiles {
		filePath := filepath.Join(devicePath, file)
		if err := fm.watcher.Add(filePath); err != nil {
			fm.logger.WithError(err).WithFields(logrus.Fields{
				"device":      pciAddr,
				"interface":   interfaceName,
				"vendor_id":   vendorID,
				"vendor_name": vendorInfo.VendorName,
				"device_id":   deviceID,
				"device_name": vendorInfo.DeviceName,
				"file":        file,
			}).Warn("failed to watch critical file")
		} else {
			fm.logger.WithFields(logrus.Fields{
				"device":      pciAddr,
				"interface":   interfaceName,
				"vendor_id":   vendorID,
				"vendor_name": vendorInfo.VendorName,
				"device_id":   deviceID,
				"device_name": vendorInfo.DeviceName,
				"file":        file,
			}).Debug("added watch for critical file")
		}
	}

	// Watch the network interface directory for eswitch mode changes (only for PFs)
	if err := fm.watchPFNetworkInterface(pciAddr); err != nil {
		fm.logger.WithError(err).WithFields(logrus.Fields{
			"device":      pciAddr,
			"interface":   interfaceName,
			"vendor_id":   vendorID,
			"vendor_name": vendorInfo.VendorName,
			"device_id":   deviceID,
			"device_name": vendorInfo.DeviceName,
		}).Warn("failed to watch PF network interface")
	}

	return nil
}

// watchPFNetworkInterface watches only the PF network interface for eswitch mode changes
func (fm *fsMonitor) watchPFNetworkInterface(pciAddr string) error {
	// Get device information for enhanced logging
	vendorID := getVendorID(pciAddr)
	deviceID := getDeviceID(pciAddr)
	vendorInfo := getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr))

	// Get the network interface name for this PCI device
	netPath := filepath.Join("/sys/bus/pci/devices", pciAddr, "net")

	entries, err := os.ReadDir(netPath)
	if err != nil {
		// No network interface found for this device
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			interfaceName := entry.Name()

			// Only watch the main PF interface, not VF interfaces or representors
			if !fm.isVFOrRepresentorInterface(interfaceName) {
				interfacePath := filepath.Join("/sys/class/net", interfaceName)

				// Watch the interface directory for eswitch mode changes
				if err := fm.watcher.Add(interfacePath); err != nil {
					fm.logger.WithError(err).WithFields(logrus.Fields{
						"device":      pciAddr,
						"interface":   interfaceName,
						"vendor_id":   vendorID,
						"vendor_name": vendorInfo.VendorName,
						"device_id":   deviceID,
						"device_name": vendorInfo.DeviceName,
					}).Warn("failed to watch PF network interface")
				} else {
					fm.logger.WithFields(logrus.Fields{
						"device":      pciAddr,
						"interface":   interfaceName,
						"vendor_id":   vendorID,
						"vendor_name": vendorInfo.VendorName,
						"device_id":   deviceID,
						"device_name": vendorInfo.DeviceName,
					}).Debug("added watch for PF network interface")
				}
			}
		}
	}

	return nil
}

// isVFOrRepresentorInterface checks if an interface name indicates it's a VF or representor
func (fm *fsMonitor) isVFOrRepresentorInterface(interfaceName string) bool {
	// VF interfaces typically contain 'v' followed by a number
	if strings.Contains(interfaceName, "v") && fm.hasVFPattern(interfaceName) {
		return true
	}

	// Representor interfaces typically contain 'pf' and 'vf' patterns
	if strings.Contains(interfaceName, "pf") && strings.Contains(interfaceName, "vf") {
		return true
	}

	// Simple representor interfaces like eth100, eth101, etc.
	if strings.HasPrefix(interfaceName, "eth") && fm.isNumericSuffix(interfaceName[3:]) {
		return true
	}

	return false
}

// hasVFPattern checks if interface name has VF pattern (e.g., ens60f0v0, enp70s0v0)
func (fm *fsMonitor) hasVFPattern(interfaceName string) bool {
	// Look for patterns like 'v0', 'v1', 'v10', etc.
	for i := 0; i < len(interfaceName)-1; i++ {
		if interfaceName[i] == 'v' {
			// Check if the next character is a digit
			if i+1 < len(interfaceName) && interfaceName[i+1] >= '0' && interfaceName[i+1] <= '9' {
				return true
			}
		}
	}
	return false
}

// isNumericSuffix checks if a string is numeric
func (fm *fsMonitor) isNumericSuffix(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// processEvents handles file system events
func (fm *fsMonitor) processEvents() {
	for {
		select {
		case event, ok := <-fm.watcher.Events:
			if !ok {
				return
			}
			fm.handleEvent(event)
		case err, ok := <-fm.watcher.Errors:
			if !ok {
				return
			}
			fm.logger.WithError(err).Error("file system monitor error")
		case <-fm.stopCh:
			return
		}
	}
}

// handleEvent processes a single file system event
func (fm *fsMonitor) handleEvent(event fsnotify.Event) {
	// Extract PCI address from the path
	devicePath := event.Name

	// Handle both device directories and files within device directories
	var pciAddr string
	var interfaceName string

	if strings.Contains(devicePath, "/sys/bus/pci/devices/") {
		// Extract PCI address from the path
		parts := strings.Split(devicePath, "/sys/bus/pci/devices/")
		if len(parts) > 1 {
			devicePart := parts[1]
			// Split by '/' to get the PCI address (first part)
			deviceParts := strings.Split(devicePart, "/")
			if len(deviceParts) > 0 {
				pciAddr = deviceParts[0]
			}
		}
	} else if strings.Contains(devicePath, "/sys/class/net/") {
		// Handle network interface events
		parts := strings.Split(devicePath, "/sys/class/net/")
		if len(parts) > 1 {
			interfacePart := parts[1]
			// Split by '/' to get the interface name (first part)
			interfaceParts := strings.Split(interfacePart, "/")
			if len(interfaceParts) > 0 {
				interfaceName = interfaceParts[0]
			}
		}

		// For network interface events, find the associated PCI device
		if interfaceName != "" {
			pciAddr = fm.getPCIAddressForInterface(interfaceName)
		}
	}

	// Skip if not a valid PCI address
	if pciAddr == "" || !strings.Contains(pciAddr, ":") {
		return
	}

	// Check if this is an SR-IOV related change
	if fm.isSRIOVEvent(event) {
		fm.logger.WithField("pci", pciAddr).Info("SR-IOV change detected, triggering rediscovery")

		// Debounce rediscovery to avoid multiple rapid updates
		go fm.debouncedRediscovery(pciAddr)
	}
}

// isSRIOVEvent checks if the event is related to SR-IOV configuration
func (fm *fsMonitor) isSRIOVEvent(event fsnotify.Event) bool {
	// Check for changes to sriov_numvfs (enabling/disabling VFs)
	if strings.HasSuffix(event.Name, "sriov_numvfs") {
		return true
	}

	// Check for changes to sriov_totalvfs (device capability changes)
	if strings.HasSuffix(event.Name, "sriov_totalvfs") {
		return true
	}

	// Check for new VF directories being created/removed
	if strings.Contains(event.Name, "virtfn") {
		return true
	}

	// Check for eswitch mode changes (only on PF interfaces)
	if strings.HasSuffix(event.Name, "eswitch_mode") {
		// Only trigger if this is a PF interface, not VF or representor
		interfaceName := fm.extractInterfaceName(event.Name)
		if interfaceName != "" && !fm.isVFOrRepresentorInterface(interfaceName) {
			return true
		}
	}

	// Check for new device directories (new PCI devices)
	if event.Op&fsnotify.Create == fsnotify.Create {
		// Check if the new directory has SR-IOV capabilities
		sriovTotalPath := filepath.Join(event.Name, "sriov_totalvfs")
		if _, err := os.Stat(sriovTotalPath); err == nil {
			return true
		}
	}

	return false
}

// extractInterfaceName extracts interface name from a sysfs path
func (fm *fsMonitor) extractInterfaceName(path string) string {
	if strings.Contains(path, "/sys/class/net/") {
		parts := strings.Split(path, "/sys/class/net/")
		if len(parts) > 1 {
			interfacePart := parts[1]
			interfaceParts := strings.Split(interfacePart, "/")
			if len(interfaceParts) > 0 {
				return interfaceParts[0]
			}
		}
	}
	return ""
}

// debouncedRediscovery performs rediscovery with debouncing to avoid rapid updates
func (fm *fsMonitor) debouncedRediscovery(pciAddr string) {
	// Use a simple debouncing mechanism
	time.Sleep(500 * time.Millisecond)

	fm.logger.WithField("pci", pciAddr).Info("performing rediscovery")

	// Perform full rediscovery
	if err := fm.server.discoverSRIOVDevices(); err != nil {
		fm.logger.WithError(err).Error("failed to rediscover SR-IOV devices")
	} else {
		fm.logger.Info("SR-IOV rediscovery completed successfully")
	}
}

// addDeviceWatch adds a watch for a specific device
func (fm *fsMonitor) addDeviceWatch(pciAddr string) error {
	// Check if vendor is allowed based on configuration
	if fm.server.config != nil {
		vendorID := getVendorID(pciAddr)
		if !fm.server.config.IsVendorAllowed(vendorID) {
			deviceID := getDeviceID(pciAddr)
			interfaceName := getInterfaceName(pciAddr)
			vendorInfo := getPCIVendorDeviceInfo(vendorID, deviceID, getSubsysVendor(pciAddr), getSubsysDevice(pciAddr))

			fm.logger.WithFields(logrus.Fields{
				"device":      pciAddr,
				"interface":   interfaceName,
				"vendor_id":   vendorID,
				"vendor_name": vendorInfo.VendorName,
				"device_id":   deviceID,
				"device_name": vendorInfo.DeviceName,
			}).Debug("skipping dynamic device watch - vendor not allowed")
			return nil // Don't add watch for disallowed vendors
		}
	}

	devicePath := filepath.Join("/sys/bus/pci/devices", pciAddr)
	return fm.watcher.Add(devicePath)
}

// removeDeviceWatch removes a watch for a specific device
func (fm *fsMonitor) removeDeviceWatch(pciAddr string) error {
	devicePath := filepath.Join("/sys/bus/pci/devices", pciAddr)
	return fm.watcher.Remove(devicePath)
}

// getPCIAddressForInterface gets the PCI address for a network interface
func (fm *fsMonitor) getPCIAddressForInterface(interfaceName string) string {
	// Look for PCI address in /sys/class/net/{interface}/device
	devicePath := filepath.Join("/sys/class/net", interfaceName, "device")
	if _, err := os.Stat(devicePath); err == nil {
		// Read the symlink to get PCI address
		if link, err := os.Readlink(devicePath); err == nil {
			// Extract PCI address from the symlink path
			parts := strings.Split(link, "/")
			for i, part := range parts {
				if part == "devices" && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}
