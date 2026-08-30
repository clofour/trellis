package agent

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
	"github.com/clofour/trellis/internal/storage"
	"github.com/google/uuid"
)

type Agent struct {
	nodeID      uuid.UUID
	allocations map[string]*Allocation
	orphans     map[string]int

	log *slog.Logger

	runtime    runtime.ContainerRuntime
	health     *health.HealthManager
	reconciler *AllocationReconciler
	ports      *PortManager
	volumes    *VolumeManager
	network    network.Manager
	server     *client.ServerClient
	nodeInfo   client.NodeInfo
	dnsServers []string
	local      *storage.LocalStorage
	cluster    string
	epoch      uint64

	// mu owns the allocation registry, fencing epoch, orphan counters and node
	// configuration. Allocation values published in the registry are treated as
	// immutable snapshots; long-running start/stop work mutates private copies
	// and republishes them instead of exposing partially updated objects.
	mu sync.RWMutex
}

type Allocation struct {
	ID              string
	AllocationID    string
	Generation      uint64
	JobRevision     int
	ExecutionHash   string
	Restart         *spec.RestartPolicySpec
	RestartAttempts int
	RestartWindow   time.Time
	Namespace       string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
	Network     *network.Attachment
	Status      string
	Health      string
}

const heartbeatInterval = 10 * time.Second

var (
	ErrAllocationNotFound = errors.New("allocation not found")
	ErrAllocationExists   = errors.New("allocation already exists")
	ErrStaleEpoch         = errors.New("stale control-plane epoch")
	ErrStaleGeneration    = errors.New("stale allocation generation")
	ErrExecutionConflict  = errors.New("allocation execution metadata conflict")
)

func NewAgent(log *slog.Logger, runtime runtime.ContainerRuntime, health *health.HealthManager, reconciler *AllocationReconciler, ports *PortManager, volumes *VolumeManager, server *client.ServerClient, nodeID uuid.UUID) *Agent {
	return &Agent{
		nodeID:      nodeID,
		allocations: make(map[string]*Allocation),
		orphans:     make(map[string]int),
		log:         log,
		runtime:     runtime,
		health:      health,
		reconciler:  reconciler,
		ports:       ports,
		volumes:     volumes,
		network:     network.DisabledManager{},
		server:      server,
		nodeInfo:    client.NodeInfo{ID: nodeID, Host: "127.0.0.1", Port: 8127},
	}
}

func (a *Agent) SetNetworkManager(manager network.Manager) {
	if manager == nil {
		return
	}
	a.mu.Lock()
	a.network = manager
	a.mu.Unlock()
}

func (a *Agent) SetWireGuardIdentity(publicKey, endpoint string) {
	a.mu.Lock()
	a.nodeInfo.WireGuardPublicKey, a.nodeInfo.WireGuardEndpoint = publicKey, endpoint
	a.mu.Unlock()
}

func (a *Agent) SetDNSServers(servers []string) {
	a.mu.Lock()
	a.dnsServers = append([]string(nil), servers...)
	a.mu.Unlock()
}

func (a *Agent) SetAdvertiseAddress(host string, port int) {
	a.mu.Lock()
	a.nodeInfo.Host, a.nodeInfo.Port = host, port
	a.mu.Unlock()
}

func (a *Agent) SetResources(cpu int, memory int64, osName, arch string) {
	a.mu.Lock()
	a.nodeInfo.CPU, a.nodeInfo.Memory, a.nodeInfo.OS, a.nodeInfo.Arch = cpu, memory, osName, arch
	a.mu.Unlock()
}

func (a *Agent) SetLabels(labels map[string]string) {
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	a.mu.Lock()
	a.nodeInfo.Labels = copy
	a.mu.Unlock()
}

func (a *Agent) Init(ctx context.Context) {
	a.health.Subscriber = a
	a.health.SetContext(ctx)
	a.reconciler.Subscriber = a
	if err := a.recover(ctx); err != nil {
		a.log.Error("recover allocations", "error", err)
	}
	go a.runHeartbeatLoop(ctx)
	go a.reconciler.Run(ctx)
}

func cloneAllocation(allocation *Allocation) *Allocation {
	if allocation == nil {
		return nil
	}
	copy := *allocation
	copy.Ports = append([]*runtime.Port(nil), allocation.Ports...)
	copy.Mounts = append([]*runtime.Mount(nil), allocation.Mounts...)
	return &copy
}

func (a *Agent) publishAllocation(allocation *Allocation) {
	a.mu.Lock()
	a.allocations[allocation.ID] = cloneAllocation(allocation)
	a.mu.Unlock()
}

func (a *Agent) GetAllocations() []*Allocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*Allocation, 0, len(a.allocations))
	for _, allocation := range a.allocations {
		result = append(result, cloneAllocation(allocation))
	}
	return result
}
