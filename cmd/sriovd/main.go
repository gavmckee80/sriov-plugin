package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"sriov-plugin/internal/config"
	"sriov-plugin/pkg/types"
	pb "sriov-plugin/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedSriovDeviceManagerServer
	logger *logrus.Logger
	config *config.Config

	// SR-IOV discovery cache
	sriovCache struct {
		sync.RWMutex
		pfs          map[string]*types.PFInfo
		vfs          map[string]*types.VFInfo
		representors map[string]*types.RepresentorInfo
	}

	// File system monitor for real-time SR-IOV changes
	fsMonitor *fsMonitor
}

// DumpInterfaces implements the gRPC DumpInterfaces method
func (s *server) DumpInterfaces(ctx context.Context, req *pb.Empty) (*pb.InterfaceDump, error) {
	s.sriovCache.RLock()
	defer s.sriovCache.RUnlock()

	// Create structured SR-IOV data
	sriovData := &types.SRIOVData{
		PhysicalFunctions: s.sriovCache.pfs,
		VirtualFunctions:  s.sriovCache.vfs,
		Representors:      s.sriovCache.representors,
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(sriovData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal dump: %v", err)
	}

	return &pb.InterfaceDump{
		JsonData:  string(jsonData),
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
	}, nil
}

func main() {
	// Parse command-line flags
	var (
		configFile           = flag.String("config", "config.yaml", "Path to configuration file")
		allowedVendors       = flag.String("allowed-vendors", "", "Comma-separated list of allowed vendor IDs (e.g., 0x15b3,0x8086)")
		excludedVendors      = flag.String("excluded-vendors", "", "Comma-separated list of excluded vendor IDs (e.g., 0x1234,0x5678)")
		enableRepresentors   = flag.Bool("enable-representors", true, "Enable representor discovery")
		enableSwitchdevCheck = flag.Bool("enable-switchdev-check", true, "Enable switchdev mode checking")
		port                 = flag.String("port", "50051", "gRPC server port")
	)
	flag.Parse()

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Load configuration
	var cfg *config.Config
	var err error

	if _, err := os.Stat(*configFile); err == nil {
		// Load from file
		cfg, err = config.LoadConfig(*configFile)
		if err != nil {
			logger.WithError(err).Fatal("failed to load configuration file")
		}
		logger.WithField("config_file", *configFile).Info("loaded configuration from file")
	} else {
		// Create default configuration
		cfg = &config.Config{
			Discovery: config.DiscoveryConfig{
				EnableRepresentorDiscovery: *enableRepresentors,
				EnableSwitchdevModeCheck:   *enableSwitchdevCheck,
			},
		}
		logger.Info("using default configuration")
	}

	// Override configuration with command-line flags
	if *allowedVendors != "" {
		vendors := parseVendorList(*allowedVendors)
		cfg.Discovery.AllowedVendorIDs = vendors
		logger.WithField("allowed_vendors", vendors).Info("set allowed vendors from command line")
	}

	if *excludedVendors != "" {
		vendors := parseVendorList(*excludedVendors)
		cfg.Discovery.ExcludedVendorIDs = vendors
		logger.WithField("excluded_vendors", vendors).Info("set excluded vendors from command line")
	}

	// Override representor discovery setting
	if !*enableRepresentors {
		cfg.Discovery.EnableRepresentorDiscovery = false
		logger.Info("representor discovery disabled via command line")
	}

	// Override switchdev check setting
	if !*enableSwitchdevCheck {
		cfg.Discovery.EnableSwitchdevModeCheck = false
		logger.Info("switchdev mode checking disabled via command line")
	}

	s := &server{
		logger: logger,
		config: cfg,
	}

	// Log discovery configuration
	logger.WithFields(logrus.Fields{
		"allowed_vendors":        cfg.Discovery.AllowedVendorIDs,
		"excluded_vendors":       cfg.Discovery.ExcludedVendorIDs,
		"enable_representors":    cfg.Discovery.EnableRepresentorDiscovery,
		"enable_switchdev_check": cfg.Discovery.EnableSwitchdevModeCheck,
	}).Info("discovery configuration")

	// Perform initial SR-IOV discovery
	if err := s.discoverSRIOVDevices(); err != nil {
		logger.WithError(err).Fatal("failed to discover SR-IOV devices")
	}

	// Initialize and start file system monitoring
	fsMonitor, err := newFSMonitor(s)
	if err != nil {
		logger.WithError(err).Fatal("failed to create file system monitor")
	}
	s.fsMonitor = fsMonitor

	if err := s.fsMonitor.start(); err != nil {
		logger.WithError(err).Fatal("failed to start file system monitoring")
	}

	// Start gRPC server
	serverAddr := ":" + *port
	lis, err := net.Listen("tcp", serverAddr)
	if err != nil {
		logger.WithError(err).Fatal("failed to listen")
	}

	// Configure gRPC server with larger message size limits
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB
		grpc.MaxSendMsgSize(100*1024*1024), // 100MB
	)
	pb.RegisterSriovDeviceManagerServer(grpcServer, s)

	logger.WithField("port", *port).Info("Starting SR-IOV management server")

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		logger.Info("Shutting down server...")
		if s.fsMonitor != nil {
			s.fsMonitor.stop()
		}
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(lis); err != nil {
		logger.WithError(err).Fatal("failed to serve")
	}
}

// parseVendorList parses a comma-separated list of vendor IDs
func parseVendorList(vendorList string) []string {
	if vendorList == "" {
		return nil
	}

	vendors := make([]string, 0)
	for _, vendor := range strings.Split(vendorList, ",") {
		vendor = strings.TrimSpace(vendor)
		if vendor != "" {
			vendors = append(vendors, vendor)
		}
	}
	return vendors
}
