package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/discovery"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type Agent struct {
	nodeID      uuid.UUID
	allocations map[string]*Allocation

	log *slog.Logger

	runtime runtime.ContainerRuntime
	health  *health.HealthManager
	restart *RestartController
	ports   *PortManager
	volumes *VolumeManager
	service discovery.ServiceRegistry
	server  *client.ServerClient
	mu      sync.RWMutex
}

type Allocation struct {
	ID string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	ServiceID   string
	ServiceIDs  []string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
	Status      string
}

const heartbeatInterval = 10 * time.Second

func NewAgent(log *slog.Logger, runtime runtime.ContainerRuntime, health *health.HealthManager, restart *RestartController, ports *PortManager, volumes *VolumeManager, service discovery.ServiceRegistry, server *client.ServerClient, nodeID uuid.UUID) *Agent {
	agent := &Agent{
		nodeID:      nodeID,
		allocations: make(map[string]*Allocation),

		log: log,

		runtime: runtime,
		health:  health,
		restart: restart,
		ports:   ports,
		volumes: volumes,
		service: service,
		server:  server,
	}

	return agent
}

func (a *Agent) Init(ctx context.Context) error {
	_, err := a.server.RegisterNode(ctx, &client.NodeInfo{ID: a.nodeID, Host: "127.0.0.1", Port: 8127})
	if err != nil {
		return fmt.Errorf("register node: %w", err)
	}

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

func (a *Agent) RunAllocation(ctx context.Context, allocID string, jobName string, groupName string, taskName string, spec *spec.TaskSpec) error {
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
		mount, err := a.volumes.Create(jobName, taskName, v)
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
	_, err = a.runtime.Create(ctx, runtime.CreateOptions{
		ID:     containerID,
		Image:  spec.Image,
		Env:    spec.Env,
		Mounts: mounts,
	})
	if err != nil {
		return fmt.Errorf("create container %s: %w", containerID, err)
	}

	err = a.runtime.Start(ctx, containerID)
	if err != nil {
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
		ID: allocID,

		JobName:   jobName,
		GroupName: groupName,
		TaskName:  spec.Name,
		Spec:      spec,

		ContainerID: containerID,
		ServiceID:   "0",
		Ports:       ports,
		Mounts:      mounts,
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

	for {
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
			}
		}
	}
}
