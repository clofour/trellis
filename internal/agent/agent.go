package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/discovery"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type Agent struct {
	nodeID      uuid.UUID
	allocations map[string]*Allocation

	log *slog.Logger

	runtime  runtime.ContainerRuntime
	health   *health.HealthManager
	restart  *RestartController
	ports    *PortManager
	volumes  *VolumeManager
	service  discovery.ServiceRegistry
	network  network.Manager
	server   *client.ServerClient
	nodeInfo client.NodeInfo
	mu       sync.RWMutex
}

type Allocation struct {
	ID     string
	Tenant string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	ServiceID   string
	ServiceIDs  []string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
	Network     *network.Attachment
	Status      string
}

const heartbeatInterval = 10 * time.Second

func NewAgent(log *slog.Logger, runtime runtime.ContainerRuntime, health *health.HealthManager, restart *RestartController, ports *PortManager, volumes *VolumeManager, service discovery.ServiceRegistry, server *client.ServerClient, nodeID uuid.UUID) *Agent {
	agent := &Agent{
		nodeID:      nodeID,
		allocations: make(map[string]*Allocation),

		log: log,

		runtime:  runtime,
		health:   health,
		restart:  restart,
		ports:    ports,
		volumes:  volumes,
		service:  service,
		network:  network.DisabledManager{},
		server:   server,
		nodeInfo: client.NodeInfo{ID: nodeID, Host: "127.0.0.1", Port: 8127},
	}

	return agent
}

func (a *Agent) SetNetworkManager(manager network.Manager) {
	if manager != nil {
		a.network = manager
	}
}

func (a *Agent) SetWireGuardIdentity(publicKey, endpoint string) {
	a.nodeInfo.WireGuardPublicKey, a.nodeInfo.WireGuardEndpoint = publicKey, endpoint
}

func (a *Agent) SetAdvertiseAddress(host string, port int) {
	a.nodeInfo.Host = host
	a.nodeInfo.Port = port
}

func (a *Agent) SetResources(cpu int, memory int64, osName, arch string) {
	a.nodeInfo.CPU, a.nodeInfo.Memory, a.nodeInfo.OS, a.nodeInfo.Arch = cpu, memory, osName, arch
}

func (a *Agent) Init(ctx context.Context) error {
	a.health.Subscriber = a
	a.restart.Subscriber = a

	go a.runHeartbeatLoop(ctx)
	go a.restart.RunDetectionLoop(ctx)
	return nil
}

func (a *Agent) GetAllocations(ctx context.Context) []*Allocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*Allocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		result = append(result, alloc)
	}

	return result
}

func (a *Agent) RunAllocation(ctx context.Context, allocID, tenant, jobName, groupName, taskName string, taskSpec *spec.TaskSpec, isolation *spec.IsolationSpec, networkPlan *network.Plan) error {
	spec := taskSpec
	if spec == nil {
		return fmt.Errorf("task spec is required")
	}
	if allocID == "" {
		return fmt.Errorf("allocation ID is required")
	}
	a.mu.RLock()
	_, exists := a.allocations[allocID]
	a.mu.RUnlock()
	if exists {
		return fmt.Errorf("allocation %s already exists", allocID)
	}

	var ports []*runtime.Port
	for _, p := range spec.Ports {
		port, err := a.ports.Claim(p)
		if err != nil {
			return fmt.Errorf("claim port %d: %w", p.HostPort, err)
		}

		ports = append(ports, port)
	}

	var mounts []*runtime.Mount
	for _, v := range spec.Volumes {
		mount, err := a.volumes.Create(filepath.Join("tenants", tenant, jobName), taskName, v)
		if err != nil {
			return fmt.Errorf("create volume %s: %w", v.Name, err)
		}

		mounts = append(mounts, mount)
	}

	err := a.runtime.Pull(ctx, spec.Image)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", spec.Image, err)
	}

	containerID := allocID
	var netAttachment *network.Attachment
	if isolation != nil && isolation.Network != nil {
		if networkPlan == nil {
			return fmt.Errorf("automatic WireGuard network plan is required")
		}
		netAttachment, err = a.network.Attach(ctx, network.AttachRequest{AllocationID: allocID, Tenant: tenant, Network: tenant, Plan: *networkPlan})
		if err != nil {
			return fmt.Errorf("attach WireGuard network: %w", err)
		}
	}
	_, err = a.runtime.Create(ctx, runtime.CreateOptions{
		ID:     containerID,
		Image:  spec.Image,
		Env:    spec.Env,
		Mounts: mounts,
		CPU: func() int {
			if spec.Resources != nil {
				return spec.Resources.CPU
			}
			return 0
		}(),
		Memory: func() int64 {
			if spec.Resources != nil {
				return int64(spec.Resources.Memory)
			}
			return 0
		}(),
		Runtime: func() string {
			if isolation != nil {
				return isolation.Runtime
			}
			return ""
		}(),
		NetworkNamespace: func() string {
			if netAttachment != nil {
				return netAttachment.Namespace
			}
			return ""
		}(),
	})
	if err != nil {
		_ = a.network.Detach(ctx, netAttachment)
		return fmt.Errorf("create container %s: %w", containerID, err)
	}

	err = a.runtime.Start(ctx, containerID)
	if err != nil {
		_ = a.runtime.Remove(ctx, containerID)
		_ = a.network.Detach(ctx, netAttachment)
		return fmt.Errorf("start container %s: %w", containerID, err)
	}

	if spec.HealthCheck != nil {
		check := *spec.HealthCheck
		for _, p := range ports {
			if p.ContainerPort == check.Port {
				check.Port = p.HostPort
				break
			}
		}
		a.health.RegisterTask(allocID, containerID, &check)
	}

	a.restart.Track(ctx, allocID)

	alloc := &Allocation{
		ID:     allocID,
		Tenant: tenant,

		JobName:   jobName,
		GroupName: groupName,
		TaskName:  spec.Name,
		Spec:      spec,

		ContainerID: containerID,
		ServiceID:   "0",
		Ports:       ports,
		Mounts:      mounts,
		Network:     netAttachment,
		Status:      "running",
	}
	a.mu.Lock()
	a.allocations[allocID] = alloc
	a.mu.Unlock()
	if spec.HealthCheck == nil {
		if err := a.OnHealthy(ctx, allocID); err != nil {
			return fmt.Errorf("register allocation service: %w", err)
		}
	}

	return nil
}

func (a *Agent) Logs(ctx context.Context, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	a.mu.RLock()
	alloc := a.allocations[allocID]
	a.mu.RUnlock()
	if alloc == nil {
		return nil, fmt.Errorf("allocation %s not found", allocID)
	}
	return a.runtime.Logs(ctx, alloc.ContainerID, follow, tail)
}

func (a *Agent) StopAllocation(ctx context.Context, allocID string) error {
	a.mu.RLock()
	alloc, ok := a.allocations[allocID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("allocation %s not found", allocID)
	}

	containerID := alloc.ContainerID

	err := a.runtime.Stop(ctx, containerID)
	if err != nil {
		return fmt.Errorf("stop container %s: %w", containerID, err)
	}

	if err = a.network.Detach(ctx, alloc.Network); err != nil {
		return fmt.Errorf("detach allocation network: %w", err)
	}
	err = a.runtime.Remove(ctx, containerID)
	if err != nil {
		return fmt.Errorf("remove container %s: %w", containerID, err)
	}

	for _, id := range alloc.ServiceIDs {
		_ = a.service.Deregister(ctx, id)
	}

	a.health.DeregisterTask(allocID)

	if err = a.restart.Untrack(ctx, allocID); err != nil {
		return fmt.Errorf("untrack allocation %s: %w", allocID, err)
	}

	for _, p := range alloc.Ports {
		err := a.ports.Release(p)
		if err != nil {
			return fmt.Errorf("release port %d: %w", p.HostPort, err)
		}
	}
	alloc.Ports = nil

	a.mu.Lock()
	delete(a.allocations, allocID)
	a.mu.Unlock()

	return nil
}

func (a *Agent) OnHealthy(ctx context.Context, allocID string) error {
	a.mu.Lock()
	alloc, ok := a.allocations[allocID]
	if ok {
		alloc.Status = "healthy"
	}
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("alloc %s not found", allocID)
	}

	for _, p := range alloc.Ports {
		id := fmt.Sprintf("%s-%d", allocID, p.HostPort)
		if err := a.service.Register(ctx, id, alloc.TaskName, "127.0.0.1", p.HostPort); err != nil {
			return err
		}
		a.mu.Lock()
		alloc.ServiceIDs = append(alloc.ServiceIDs, id)
		a.mu.Unlock()
	}

	return nil
}

func (a *Agent) OnUnhealthy(ctx context.Context, allocID string) error {
	a.mu.Lock()
	if alloc := a.allocations[allocID]; alloc != nil {
		alloc.Status = "unhealthy"
	}
	a.mu.Unlock()
	a.mu.RLock()
	alloc := a.allocations[allocID]
	a.mu.RUnlock()
	if alloc != nil {
		for _, id := range alloc.ServiceIDs {
			_ = a.service.Deregister(ctx, id)
		}
	}

	return nil
}

func (a *Agent) OnFailed(allocID string) {
	a.mu.Lock()
	if alloc := a.allocations[allocID]; alloc != nil {
		alloc.Status = "unhealthy"
	}
	a.mu.Unlock()
}

func (a *Agent) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	registered := false

	for {
		if !registered {
			if _, err := a.server.RegisterNode(ctx, &a.nodeInfo); err != nil {
				a.log.Error("register node failed", "error", err)
			} else {
				registered = true
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			actual := make([]api.AllocationStatus, 0, len(a.allocations))
			for _, alloc := range a.allocations {
				actual = append(actual, api.AllocationStatus{ID: alloc.ID, Status: alloc.Status})
			}
			a.mu.RUnlock()
			err := a.server.SendHeartbeat(ctx, a.nodeID, &client.Heartbeat{
				NodeID:      a.nodeID,
				Timestamp:   time.Now(),
				Allocations: actual,
			})
			if err != nil {
				a.log.Error("send heartbeat failed", "error", err)
				registered = false
			}
		}
	}
}
