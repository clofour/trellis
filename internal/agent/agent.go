package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
}

type Allocation struct {
	ID string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	ServiceID   string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
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

func (a *Agent) Init(ctx context.Context) {
	a.server.RegisterNode(ctx, &client.NodeInfo{})

	a.health.Subscriber = a
	a.restart.subscriber = a

	go a.runHeartbeatLoop(ctx)
	go a.restart.RunDetectionLoop(ctx)
}

func (a *Agent) GetAllocations(ctx context.Context) []*Allocation {
	result := make([]*Allocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		result = append(result, alloc)
	}

	return result
}

func (a *Agent) RunAllocation(ctx context.Context, jobName string, groupName string, taskName string, spec *spec.TaskSpec) error {
	allocID := fmt.Sprintf("%s-%s-%s-%d", jobName, groupName, taskName, 0)

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

	a.health.RegisterTask(allocID, containerID, spec.HealthCheck)

	a.restart.Track(ctx, allocID)

	alloc := &Allocation{
		ID: allocID,

		JobName:   jobName,
		GroupName: groupName,
		TaskName:  taskName,
		Spec:      spec,

		ContainerID: containerID,
		ServiceID:   "0",
		Ports:       ports,
		Mounts:      mounts,
	}
	a.allocations[allocID] = alloc

	return nil
}

func (a *Agent) StopAllocation(ctx context.Context, allocID string) error {
	alloc, ok := a.allocations[allocID]
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

	a.service.Deregister(ctx, allocID)

	a.health.DeregisterTask(allocID)

	a.restart.Untrack(ctx, allocID)

	for _, p := range alloc.Ports {
		err := a.ports.Release(p)
		if err != nil {
			return fmt.Errorf("release port %d: %w", p.HostPort, err)
		}
	}
	alloc.Ports = nil

	delete(a.allocations, allocID)

	return nil
}

func (a *Agent) OnHealthy(ctx context.Context, allocID string) error {
	alloc, ok := a.allocations[allocID]
	if !ok {
		return fmt.Errorf("alloc %s not found", allocID)
	}

	for _, p := range alloc.Ports {
		a.service.Register(ctx, allocID, alloc.TaskName, "127.0.0.1", p.HostPort)
	}

	return nil
}

func (a *Agent) OnUnhealthy(ctx context.Context, allocID string) error {
	a.service.Deregister(ctx, allocID)

	return nil
}

func (a *Agent) OnFailed(allocID string) {
	// mark allocation as failed
	// where should state be stored? should heartbeat just gather state from all modules?
}

func (a *Agent) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.server.SendHeartbeat(ctx, a.nodeID, &client.Heartbeat{
				NodeID:    a.nodeID,
				Timestamp: time.Now(),
			})
		}
	}
}
