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
	pb "sriov-plugin/proto"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
)

type poolLabel struct {
	name string
	pf   string
	vfs  map[string]bool
	cfg  *pb.PoolConfig
}

type server struct {
	pb.UnimplementedSriovDeviceManagerServer
	mu         sync.Mutex
	allocated  map[string]bool
	masked     map[string]bool
	maskReason map[string]string
	allowedPFs map[string]bool
	vfToPool   map[string]string
	poolMap    map[string]*poolLabel
	cfgPath    string
	logger     *logrus.Logger
}

func (s *server) reloadConfig() {
	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		s.logger.WithError(err).Error("failed to reload config")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allowedPFs = make(map[string]bool)
	s.masked = make(map[string]bool)
	s.maskReason = make(map[string]string)

	for _, pool := range cfg.Pools {
		s.allowedPFs[pool.PfPCI] = true
		vfIndices, err := config.ParseVFRange(pool.VFRange)
		if err != nil {
			s.logger.WithError(err).WithField("pool", pool.Name).Error("invalid vf_range in pool")
			continue
		}
		for _, vf := range vfIndices {
			vfAddr := fmt.Sprintf("%s-vf%d", pool.PfPCI, vf)
			if pool.Mask {
				s.masked[vfAddr] = true
				s.maskReason[vfAddr] = pool.MaskReason
				s.logger.WithFields(logrus.Fields{
					"vf":     vfAddr,
					"pool":   pool.Name,
					"reason": pool.MaskReason,
				}).Info("masked VF due to pool configuration")
			}
			s.tagVFWithPool(vfAddr, pool.Name, &pb.PoolConfig{
				Name:             pool.Name,
				PfPci:            pool.PfPCI,
				VfRange:          pool.VFRange,
				Mask:             pool.Mask,
				MaskReason:       pool.MaskReason,
				RequiredFeatures: pool.RequiredFeatures,
				Numa:             pool.NUMA,
			})
		}
	}
	s.logger.Info("config reloaded")
}

func (s *server) tagVFWithPool(vfPCI, poolName string, cfg *pb.PoolConfig) {
	if s.vfToPool == nil {
		s.vfToPool = make(map[string]string)
	}
	s.vfToPool[vfPCI] = poolName
	if s.poolMap == nil {
		s.poolMap = make(map[string]*poolLabel)
	}
	key := fmt.Sprintf("%s:%s", poolName, pciToPF(vfPCI))
	if _, ok := s.poolMap[key]; !ok {
		s.poolMap[key] = &poolLabel{name: poolName, pf: pciToPF(vfPCI), vfs: make(map[string]bool), cfg: cfg}
	}
	s.poolMap[key].vfs[vfPCI] = true
}

func pciToPF(vfPci string) string {
	if idx := strings.Index(vfPci, "-vf"); idx > 0 {
		return vfPci[:idx]
	}
	return ""
}

// ListDevices implements the gRPC ListDevices method
func (s *server) ListDevices(ctx context.Context, req *pb.Empty) (*pb.DeviceList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pfs []*pb.PF
	for _, pool := range s.poolMap {
		pf := &pb.PF{
			PfPci: pool.pf,
			Pool:  pool.name,
		}

		// Add VFs for this pool
		for vfPCI := range pool.vfs {
			vf := &pb.VF{
				VfPci:       vfPCI,
				PfPci:       pool.pf,
				Allocated:   s.allocated[vfPCI],
				Masked:      s.masked[vfPCI],
				Pool:        pool.name,
				LastUpdated: time.Now().Format(time.RFC3339),
			}
			pf.Vfs = append(pf.Vfs, vf)
		}
		pfs = append(pfs, pf)
	}

	return &pb.DeviceList{Pfs: pfs}, nil
}

// GetStatus implements the gRPC GetStatus method
func (s *server) GetStatus(ctx context.Context, req *pb.Empty) (*pb.StatusReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var pools []*pb.PoolSummary
	for _, pool := range s.poolMap {
		total := uint32(len(pool.vfs))
		allocated := uint32(0)
		masked := uint32(0)

		for vfPCI := range pool.vfs {
			if s.allocated[vfPCI] {
				allocated++
			}
			if s.masked[vfPCI] {
				masked++
			}
		}

		free := total - allocated - masked
		percentFree := float32(0)
		if total > 0 {
			percentFree = float32(free) / float32(total) * 100
		}

		summary := &pb.PoolSummary{
			PfPci:       pool.pf,
			Name:        pool.name,
			Total:       total,
			Allocated:   allocated,
			Masked:      masked,
			Free:        free,
			PercentFree: percentFree,
		}
		pools = append(pools, summary)
	}

	return &pb.StatusReport{Pools: pools}, nil
}

// AllocateVFs implements the gRPC AllocateVFs method
func (s *server) AllocateVFs(ctx context.Context, req *pb.AllocationRequest) (*pb.AllocationResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allocatedVFs []*pb.VF
	message := "Allocation completed"

	// Find available VFs in the specified PF
	for _, pool := range s.poolMap {
		if pool.pf != req.PfPci {
			continue
		}

		available := 0
		for vfPCI := range pool.vfs {
			if !s.allocated[vfPCI] && !s.masked[vfPCI] {
				available++
			}
		}

		if available >= int(req.Count) {
			// Allocate VFs
			allocated := 0
			for vfPCI := range pool.vfs {
				if allocated >= int(req.Count) {
					break
				}
				if !s.allocated[vfPCI] && !s.masked[vfPCI] {
					s.allocated[vfPCI] = true
					vf := &pb.VF{
						VfPci:       vfPCI,
						PfPci:       pool.pf,
						Allocated:   true,
						Pool:        pool.name,
						LastUpdated: time.Now().Format(time.RFC3339),
					}
					allocatedVFs = append(allocatedVFs, vf)
					allocated++
				}
			}
			break
		}
	}

	if len(allocatedVFs) == 0 {
		message = "No available VFs found"
	}

	return &pb.AllocationResponse{
		AllocatedVfs: allocatedVFs,
		Message:      message,
	}, nil
}

// ReleaseVFs implements the gRPC ReleaseVFs method
func (s *server) ReleaseVFs(ctx context.Context, req *pb.ReleaseRequest) (*pb.ReleaseResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var released []string
	for _, vfPCI := range req.VfPcis {
		if s.allocated[vfPCI] {
			s.allocated[vfPCI] = false
			released = append(released, vfPCI)
		}
	}

	return &pb.ReleaseResponse{
		Released: released,
		Message:  fmt.Sprintf("Released %d VFs", len(released)),
	}, nil
}

// MaskVF implements the gRPC MaskVF method
func (s *server) MaskVF(ctx context.Context, req *pb.MaskRequest) (*pb.MaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.allocated[req.VfPci] {
		return &pb.MaskResponse{
			Success: false,
			Message: "Cannot mask allocated VF",
		}, nil
	}

	s.masked[req.VfPci] = true
	s.maskReason[req.VfPci] = req.Reason

	return &pb.MaskResponse{
		Success: true,
		Message: "VF masked successfully",
	}, nil
}

// UnmaskVF implements the gRPC UnmaskVF method
func (s *server) UnmaskVF(ctx context.Context, req *pb.UnmaskRequest) (*pb.UnmaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.masked[req.VfPci] = false
	delete(s.maskReason, req.VfPci)

	return &pb.UnmaskResponse{
		Success: true,
		Message: "VF unmasked successfully",
	}, nil
}

// ListPools implements the gRPC ListPools method
func (s *server) ListPools(ctx context.Context, req *pb.Empty) (*pb.PoolList, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var names []string
	for _, pool := range s.poolMap {
		names = append(names, pool.name)
	}

	return &pb.PoolList{Names: names}, nil
}

// GetPoolConfig implements the gRPC GetPoolConfig method
func (s *server) GetPoolConfig(ctx context.Context, req *pb.PoolQuery) (*pb.PoolConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pool := range s.poolMap {
		if pool.name == req.Name {
			return pool.cfg, nil
		}
	}
	return nil, fmt.Errorf("pool %s not found", req.Name)
}

func (s *server) DumpInterfaces(ctx context.Context, req *pb.Empty) (*pb.InterfaceDump, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create comprehensive dump structure
	dump := map[string]interface{}{
		"server_info": map[string]interface{}{
			"version":     "1.0.0",
			"timestamp":   time.Now().Format(time.RFC3339),
			"config_path": s.cfgPath,
		},
		"pools":              map[string]interface{}{},
		"physical_functions": map[string]interface{}{},
		"virtual_functions":  map[string]interface{}{},
		"allocations": map[string]interface{}{
			"allocated_vfs": []string{},
			"masked_vfs":    []string{},
		},
		"statistics": map[string]interface{}{
			"total_pfs":     0,
			"total_vfs":     0,
			"allocated_vfs": 0,
			"masked_vfs":    0,
			"available_vfs": 0,
		},
	}

	// Collect pool information
	for _, pool := range s.poolMap {
		vfs := []string{}
		for vf := range pool.vfs {
			vfs = append(vfs, vf)
		}

		dump["pools"].(map[string]interface{})[pool.name] = map[string]interface{}{
			"pf_pci":            pool.cfg.PfPci,
			"vf_range":          pool.cfg.VfRange,
			"mask":              pool.cfg.Mask,
			"mask_reason":       pool.cfg.MaskReason,
			"required_features": pool.cfg.RequiredFeatures,
			"numa":              pool.cfg.Numa,
			"vf_count":          len(pool.vfs),
			"vfs":               vfs,
		}
	}

	// Collect PF information
	pfMap := make(map[string]map[string]interface{})
	for _, pool := range s.poolMap {
		pfPCI := pool.pf
		if _, exists := pfMap[pfPCI]; !exists {
			pfMap[pfPCI] = map[string]interface{}{
				"pools":     []string{},
				"vf_count":  0,
				"allocated": 0,
				"masked":    0,
				"available": 0,
			}
		}
		pfMap[pfPCI]["pools"] = append(pfMap[pfPCI]["pools"].([]string), pool.name)
		pfMap[pfPCI]["vf_count"] = pfMap[pfPCI]["vf_count"].(int) + len(pool.vfs)
	}

	// Collect VF information
	vfDetails := make(map[string]map[string]interface{})
	allocatedVFs := []string{}
	maskedVFs := []string{}

	// Collect all VFs from pools
	for _, pool := range s.poolMap {
		for vfPCI := range pool.vfs {
			vfDetails[vfPCI] = map[string]interface{}{
				"allocated": s.allocated[vfPCI],
				"masked":    s.masked[vfPCI],
				"pool":      s.vfToPool[vfPCI],
			}
			if s.allocated[vfPCI] {
				allocatedVFs = append(allocatedVFs, vfPCI)
			}
			if s.masked[vfPCI] {
				maskedVFs = append(maskedVFs, vfPCI)
				vfDetails[vfPCI]["mask_reason"] = s.maskReason[vfPCI]
			}
		}
	}

	// Update statistics
	totalVFs := len(s.allocated)
	allocatedCount := len(allocatedVFs)
	maskedCount := len(maskedVFs)
	availableCount := totalVFs - allocatedCount - maskedCount

	dump["allocations"].(map[string]interface{})["allocated_vfs"] = allocatedVFs
	dump["allocations"].(map[string]interface{})["masked_vfs"] = maskedVFs
	dump["virtual_functions"] = vfDetails
	dump["physical_functions"] = pfMap

	dump["statistics"].(map[string]interface{})["total_pfs"] = len(pfMap)
	dump["statistics"].(map[string]interface{})["total_vfs"] = totalVFs
	dump["statistics"].(map[string]interface{})["allocated_vfs"] = allocatedCount
	dump["statistics"].(map[string]interface{})["masked_vfs"] = maskedCount
	dump["statistics"].(map[string]interface{})["available_vfs"] = availableCount

	// Convert to JSON
	jsonData, err := json.MarshalIndent(dump, "", "  ")
	if err != nil {
		s.logger.WithError(err).Error("failed to marshal interface dump to JSON")
		return nil, fmt.Errorf("failed to generate JSON dump: %v", err)
	}

	return &pb.InterfaceDump{
		JsonData:  string(jsonData),
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "1.0.0",
	}, nil
}

func main() {
	configPath := flag.String("config", "config.yaml", "Path to config file")
	flag.Parse()

	s := &server{
		allocated:  make(map[string]bool),
		masked:     make(map[string]bool),
		maskReason: make(map[string]string),
		allowedPFs: make(map[string]bool),
		cfgPath:    *configPath,
		logger:     logrus.New(),
	}
	s.reloadConfig()

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGHUP)
		for range ch {
			s.reloadConfig()
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		s.logger.WithError(err).Fatal("failed to listen")
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSriovDeviceManagerServer(grpcServer, s)
	s.logger.Info("gRPC server started on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		s.logger.WithError(err).Fatal("failed to serve")
	}
}
