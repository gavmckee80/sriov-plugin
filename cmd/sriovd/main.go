package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"sriov-plugin/pkg/types"
	pb "sriov-plugin/proto"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedSriovDeviceManagerServer
	logger *logrus.Logger

	// SR-IOV discovery cache
	sriovCache struct {
		sync.RWMutex
		pfs map[string]*types.PFInfo
		vfs map[string]*types.VFInfo
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
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	s := &server{
		logger: logger,
	}

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
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.WithError(err).Fatal("failed to listen")
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSriovDeviceManagerServer(grpcServer, s)

	logger.Info("Starting SR-IOV management server on :50051")

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
