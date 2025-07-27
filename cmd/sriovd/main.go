package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"sriov-plugin/internal/config"
	pb "sriov-plugin/proto"

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
}

func (s *server) reloadConfig() {
	cfg, err := config.LoadConfig(s.cfgPath)
	if err != nil {
		log.Printf("failed to reload config: %v", err)
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
			log.Printf("invalid vf_range in pool %s: %v", pool.Name, err)
			continue
		}
		for _, vf := range vfIndices {
			vfAddr := fmt.Sprintf("%s-vf%d", pool.PfPCI, vf)
			if pool.Mask {
				s.masked[vfAddr] = true
				s.maskReason[vfAddr] = pool.MaskReason
				log.Printf("masked VF %s due to pool %s (%s)", vfAddr, pool.Name, pool.MaskReason)
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
	log.Println("config reloaded")
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

	return nil, fmt.Errorf("pool not found: %s", req.Name)
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
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterSriovDeviceManagerServer(grpcServer, s)
	log.Println("gRPC server started on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
