package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
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

	runtime    runtime.ContainerRuntime
	health     *health.HealthManager
	restart    *RestartController
	ports      *PortManager
	volumes    *VolumeManager
	network    network.Manager
	server     *client.ServerClient
	nodeInfo   client.NodeInfo
	dnsServers []string
	mu         sync.RWMutex
}

type Allocation struct {
	ID        string
	Namespace string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
	Network     *network.Attachment
	Status      string
}

const heartbeatInterval = 10 * time.Second

var (
	ErrAllocationNotFound = errors.New("allocation not found")
	ErrAllocationExists   = errors.New("allocation already exists")
)

func NewAgent(log *slog.Logger, runtime runtime.ContainerRuntime, health *health.HealthManager, restart *RestartController, ports *PortManager, volumes *VolumeManager, server *client.ServerClient, nodeID uuid.UUID) *Agent {
	agent := &Agent{
		nodeID:      nodeID,
		allocations: make(map[string]*Allocation),

		log: log,

		runtime: runtime,
		health:  health,
		restart: restart,
		ports:   ports,
		volumes: volumes,
		network: network.DisabledManager{},
		server:  server,
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

func (a *Agent) SetDNSServers(servers []string) {
	a.dnsServers = servers
}

func (a *Agent) SetAdvertiseAddress(host string, port int) {
	a.nodeInfo.Host = host
	a.nodeInfo.Port = port
}

func (a *Agent) SetResources(cpu int, memory int64, osName, arch string) {
	a.nodeInfo.CPU, a.nodeInfo.Memory, a.nodeInfo.OS, a.nodeInfo.Arch = cpu, memory, osName, arch
}

func (a *Agent) Init(ctx context.Context) {
	a.health.Subscriber = a
	a.health.SetContext(ctx)
	a.restart.Subscriber = a

	go a.runHeartbeatLoop(ctx)
	go a.restart.RunDetectionLoop(ctx)
}

func (a *Agent) GetAllocations() []*Allocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*Allocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		copy := *alloc
		copy.Ports = append([]*runtime.Port(nil), alloc.Ports...)
		copy.Mounts = append([]*runtime.Mount(nil), alloc.Mounts...)
		result = append(result, &copy)
	}

	return result
}

func (a *Agent) RunAllocation(ctx context.Context, allocID, namespace, jobName, groupName, taskName string, taskSpec *spec.TaskSpec, groupRuntime string, wireGuard bool, networkPlan *network.Plan, networkMode string, envOverrides map[string]string) error {
	spec := taskSpec
	if spec == nil {
		return fmt.Errorf("task spec is required")
	}
	if allocID == "" {
		return fmt.Errorf("allocation ID is required")
	}
	a.mu.Lock()
	_, exists := a.allocations[allocID]
	if exists {
		a.mu.Unlock()
		return fmt.Errorf("%w: %s", ErrAllocationExists, allocID)
	}
	alloc := &Allocation{ID: allocID, Namespace: namespace, JobName: jobName, GroupName: groupName, TaskName: taskName, Spec: spec, Status: "starting"}
	a.allocations[allocID] = alloc
	a.mu.Unlock()
	committed := false
	containerCreated := false
	containerStarted := false
	tracked := false
	healthRegistered := false
	var netAttachment *network.Attachment
	var ports []*runtime.Port
	defer func() {
		if committed {
			return
		}
		if healthRegistered {
			a.health.DeregisterTask(allocID)
		}
		if tracked {
			_ = a.restart.Untrack(allocID)
		}
		if containerStarted {
			_ = a.runtime.Stop(context.WithoutCancel(ctx), allocID)
		}
		if containerCreated {
			_ = a.runtime.Remove(context.WithoutCancel(ctx), allocID)
		}
		_ = a.network.Detach(context.WithoutCancel(ctx), netAttachment)
		for _, p := range ports {
			_ = a.ports.Release(p)
		}
		a.mu.Lock()
		delete(a.allocations, allocID)
		a.mu.Unlock()
	}()

	for _, p := range spec.Ports {
		port, err := a.ports.Claim(p)
		if err != nil {
			return fmt.Errorf("claim port %d: %w", p.HostPort, err)
		}

		ports = append(ports, port)
	}

	var mounts []*runtime.Mount
	for _, v := range spec.Volumes {
		mount, err := a.volumes.Create(filepath.Join("namespaces", namespace, jobName), taskName, v)
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
	hostMode := networkMode == "host"
	if wireGuard && !hostMode {
		if networkPlan == nil {
			return fmt.Errorf("automatic WireGuard network plan is required")
		}
		netAttachment, err = a.network.Attach(ctx, network.AttachRequest{AllocationID: allocID, Namespace: namespace, Network: namespace, Plan: *networkPlan})
		if err != nil {
			return fmt.Errorf("attach WireGuard network: %w", err)
		}
	}
	env := make(map[string]string, len(spec.Env)+len(envOverrides))
	for k, v := range spec.Env {
		env[k] = v
	}
	for k, v := range envOverrides {
		env[k] = v
	}
	_, err = a.runtime.Create(ctx, runtime.CreateOptions{
		ID:     containerID,
		Image:  spec.Image,
		Env:    env,
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
		Runtime: groupRuntime,
		NetworkNamespace: func() string {
			if netAttachment != nil {
				return netAttachment.NetworkNamespace
			}
			return ""
		}(),
		DNSServers: a.dnsServers,
	})
	if err != nil {
		return fmt.Errorf("create container %s: %w", containerID, err)
	}
	containerCreated = true

	err = a.runtime.Start(ctx, containerID)
	if err != nil {
		return fmt.Errorf("start container %s: %w", containerID, err)
	}
	containerStarted = true

	if spec.HealthCheck != nil {
		check := *spec.HealthCheck
		for _, p := range ports {
			if p.ContainerPort == check.Port {
				check.Port = p.HostPort
				break
			}
		}
		a.health.RegisterTask(allocID, containerID, &check)
		healthRegistered = true
	}

	a.restart.Track(allocID)
	tracked = true

	ready := &Allocation{
		ID:        allocID,
		Namespace: namespace,

		JobName:   jobName,
		GroupName: groupName,
		TaskName:  spec.Name,
		Spec:      spec,

		ContainerID: containerID,
		Ports:       ports,
		Mounts:      mounts,
		Network:     netAttachment,
		Status:      "running",
	}
	a.mu.Lock()
	a.allocations[allocID] = ready
	a.mu.Unlock()
	if spec.HealthCheck == nil {
		if err := a.OnHealthy(ctx, allocID); err != nil {
			return fmt.Errorf("register allocation service: %w", err)
		}
	}
	committed = true

	return nil
}

func (a *Agent) Logs(ctx context.Context, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	a.mu.RLock()
	alloc := a.allocations[allocID]
	a.mu.RUnlock()
	if alloc == nil {
		return nil, fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}
	return a.runtime.Logs(ctx, alloc.ContainerID, follow, tail)
}

func (a *Agent) StopAllocation(ctx context.Context, allocID string) error {
	a.mu.RLock()
	stored, ok := a.allocations[allocID]
	var alloc Allocation
	if ok {
		alloc = *stored
		alloc.Ports = append([]*runtime.Port(nil), stored.Ports...)
	}
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}

	containerID := alloc.ContainerID

	var errs []error
	if err := a.runtime.Stop(ctx, containerID); err != nil {
		errs = append(errs, fmt.Errorf("stop container %s: %w", containerID, err))
	}
	if err := a.network.Detach(ctx, alloc.Network); err != nil {
		errs = append(errs, fmt.Errorf("detach allocation network: %w", err))
	}
	if err := a.runtime.Remove(ctx, containerID); err != nil {
		errs = append(errs, fmt.Errorf("remove container %s: %w", containerID, err))
	}

	a.health.DeregisterTask(allocID)

	if err := a.restart.Untrack(allocID); err != nil {
		errs = append(errs, fmt.Errorf("untrack allocation %s: %w", allocID, err))
	}

	for _, p := range alloc.Ports {
		err := a.ports.Release(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("release port %d: %w", p.HostPort, err))
		}
	}
	a.mu.Lock()
	delete(a.allocations, allocID)
	a.mu.Unlock()

	return errors.Join(errs...)
}

func (a *Agent) OnHealthy(_ context.Context, allocID string) error {
	a.mu.Lock()
	alloc, ok := a.allocations[allocID]
	if ok {
		alloc.Status = "healthy"
	}
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("alloc %s not found", allocID)
	}
	return nil
}

func (a *Agent) OnUnhealthy(_ context.Context, allocID string) error {
	a.mu.Lock()
	alloc := a.allocations[allocID]
	if alloc != nil {
		alloc.Status = "unhealthy"
	}
	a.mu.Unlock()
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
			if a.server.Ready() {
				if _, err := a.server.RegisterNode(ctx, &a.nodeInfo); err != nil {
					a.log.Error("register node failed", "error", err)
				} else {
					registered = true
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !a.server.Ready() {
				continue
			}
			a.mu.RLock()
			actual := make([]api.AllocationStatus, 0, len(a.allocations))
			for _, alloc := range a.allocations {
				ports := make([]api.PortMapping, 0, len(alloc.Ports))
				for _, p := range alloc.Ports {
					ports = append(ports, api.PortMapping{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
				}
				actual = append(actual, api.AllocationStatus{ID: alloc.ID, Status: alloc.Status, Ports: ports})
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
