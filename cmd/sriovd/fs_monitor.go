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
	fm.logger.Info("Starting file system monitoring for SR-IOV changes")

	// Watch the main PCI devices directory
	if err := fm.watcher.Add("/sys/bus/pci/devices"); err != nil {
		return err
	}

	// Watch existing SR-IOV capable devices
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

// watchExistingSRIOVDevices adds watches for existing SR-IOV capable devices
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
			// Watch the device directory for changes
			if err := fm.watcher.Add(device); err != nil {
				fm.logger.WithError(err).WithField("device", device).Warn("failed to watch device")
			} else {
				fm.logger.WithField("device", pciAddr).Debug("added watch for SR-IOV device")
			}
		}
	}

	return nil
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
	devicePath := filepath.Join("/sys/bus/pci/devices", pciAddr)
	return fm.watcher.Add(devicePath)
}

// removeDeviceWatch removes a watch for a specific device
func (fm *fsMonitor) removeDeviceWatch(pciAddr string) error {
	devicePath := filepath.Join("/sys/bus/pci/devices", pciAddr)
	return fm.watcher.Remove(devicePath)
}
