package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
)

func (a *Agent) PrepareStart(ctx context.Context, request *api.AllocationRequest) error {
	if err := a.AcceptEpoch(request.Epoch); err != nil {
		return err
	}
	a.mu.RLock()
	var oldIDs []string
	for id, allocation := range a.allocations {
		if allocation.AllocationID != request.AllocationID {
			continue
		}
		if allocation.Generation > request.Generation {
			a.mu.RUnlock()
			return fmt.Errorf("%w: current %d, requested %d", ErrStaleGeneration, allocation.Generation, request.Generation)
		}
		if allocation.Generation == request.Generation && allocation.ExecutionHash != request.ExecutionHash {
			a.mu.RUnlock()
			return fmt.Errorf("%w: allocation %s generation %d", ErrExecutionConflict, request.AllocationID, request.Generation)
		}
		if allocation.Generation < request.Generation {
			oldIDs = append(oldIDs, id)
		}
	}
	a.mu.RUnlock()
	for _, id := range oldIDs {
		if err := a.StopAllocation(ctx, id); err != nil {
			return fmt.Errorf("replace older generation: %w", err)
		}
	}
	return nil
}

func (a *Agent) StopGroup(ctx context.Context, request *api.StopAllocationRequest) error {
	if err := a.AcceptEpoch(request.Epoch); err != nil {
		return err
	}
	a.mu.RLock()
	var ids []string
	for id, allocation := range a.allocations {
		if allocation.AllocationID != request.AllocationID {
			continue
		}
		if allocation.Generation > request.Generation {
			a.mu.RUnlock()
			return fmt.Errorf("%w: current %d, requested %d", ErrStaleGeneration, allocation.Generation, request.Generation)
		}
		if allocation.Generation == request.Generation {
			ids = append(ids, id)
		}
	}
	a.mu.RUnlock()
	var errs []error
	for _, id := range ids {
		if err := a.StopAllocation(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *Agent) RunAllocation(ctx context.Context, allocID, schedulerID string, generation uint64, jobRevision int, executionHash, namespace, jobName, groupName, taskName string, taskSpec *spec.TaskSpec, groupRuntime string, wireGuard bool, networkPlan *network.Plan, networkMode string, envOverrides map[string]string, restartPolicy *spec.RestartPolicySpec) error {
	if taskSpec == nil {
		return fmt.Errorf("task spec is required")
	}
	if allocID == "" {
		return fmt.Errorf("allocation ID is required")
	}

	a.mu.Lock()
	existing := a.allocations[allocID]
	if existing != nil {
		a.mu.Unlock()
		if existing.AllocationID == schedulerID && existing.Generation == generation && existing.ExecutionHash == executionHash {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrAllocationExists, allocID)
	}
	working := &Allocation{ID: allocID, ContainerID: allocID, AllocationID: schedulerID, Generation: generation, JobRevision: jobRevision, ExecutionHash: executionHash, Namespace: namespace, JobName: jobName, GroupName: groupName, TaskName: taskName, Spec: taskSpec, Status: "starting", Health: "unknown"}
	a.allocations[allocID] = cloneAllocation(working)
	a.mu.Unlock()
	if err := a.persistAllocation(working); err != nil {
		a.mu.Lock()
		delete(a.allocations, allocID)
		a.mu.Unlock()
		return fmt.Errorf("persist starting allocation: %w", err)
	}

	committed, containerCreated, containerStarted, tracked, healthRegistered := false, false, false, false, false
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
			_ = a.reconciler.Untrack(allocID)
		}
		if containerStarted {
			_ = a.runtime.Stop(context.WithoutCancel(ctx), allocID)
		}
		if containerCreated {
			_ = a.runtime.Remove(context.WithoutCancel(ctx), allocID)
		}
		_ = a.network.Detach(context.WithoutCancel(ctx), netAttachment)
		for _, port := range ports {
			_ = a.ports.Release(port)
		}
		a.mu.Lock()
		delete(a.allocations, allocID)
		a.mu.Unlock()
		_ = a.deleteAllocationRecord(allocID)
	}()

	for _, portSpec := range taskSpec.Ports {
		port, err := a.ports.Claim(portSpec)
		if err != nil {
			return fmt.Errorf("claim port %d: %w", portSpec.HostPort, err)
		}
		ports = append(ports, port)
		working.Ports = append([]*runtime.Port(nil), ports...)
		a.publishAllocation(working)
		if err := a.persistAllocation(working); err != nil {
			return fmt.Errorf("persist port claim: %w", err)
		}
	}

	var mounts []*runtime.Mount
	for _, volume := range taskSpec.Volumes {
		mount, err := a.volumes.Create(namespace, jobName, taskName, volume)
		if err != nil {
			return fmt.Errorf("create volume %s: %w", volume.Name, err)
		}
		mounts = append(mounts, mount)
		working.Mounts = append([]*runtime.Mount(nil), mounts...)
	}
	a.publishAllocation(working)
	if err := a.persistAllocation(working); err != nil {
		return fmt.Errorf("persist volume metadata: %w", err)
	}

	if err := a.runtime.Pull(ctx, taskSpec.Image); err != nil {
		return fmt.Errorf("pull image %s: %w", taskSpec.Image, err)
	}

	hostMode := networkMode == "host"
	if wireGuard && !hostMode {
		if networkPlan == nil {
			return fmt.Errorf("automatic WireGuard network plan is required")
		}
		var err error
		netAttachment, err = a.network.Attach(ctx, network.AttachRequest{AllocationID: allocID, Namespace: namespace, Network: namespace, Plan: *networkPlan})
		if err != nil {
			return fmt.Errorf("attach WireGuard network: %w", err)
		}
		working.Network = netAttachment
		a.publishAllocation(working)
		if err := a.persistAllocation(working); err != nil {
			return fmt.Errorf("persist network attachment: %w", err)
		}
	}

	env := make(map[string]string, len(taskSpec.Env)+len(envOverrides))
	for key, value := range taskSpec.Env {
		env[key] = value
	}
	for key, value := range envOverrides {
		env[key] = value
	}
	a.mu.RLock()
	cluster := a.cluster
	dnsServers := append([]string(nil), a.dnsServers...)
	a.mu.RUnlock()
	labels := map[string]string{
		"trellis.cluster": cluster, "trellis.allocation-id": schedulerID,
		"trellis.allocation-generation": strconv.FormatUint(generation, 10),
		"trellis.namespace": namespace, "trellis.job": jobName,
		"trellis.task-group": groupName, "trellis.task": taskName,
	}
	_, err := a.runtime.Create(ctx, runtime.CreateOptions{
		ID: allocID, Image: taskSpec.Image, Env: env, Mounts: mounts,
		CPU: func() int { if taskSpec.Resources != nil { return taskSpec.Resources.CPU }; return 0 }(),
		Memory: func() int64 { if taskSpec.Resources != nil { return int64(taskSpec.Resources.Memory) }; return 0 }(),
		Runtime: groupRuntime,
		NetworkNamespace: func() string { if netAttachment != nil { return netAttachment.NetworkNamespace }; return "" }(),
		DNSServers: dnsServers, Labels: labels,
	})
	if err != nil {
		observed, inspectErr := a.runtime.Inspect(context.WithoutCancel(ctx), allocID)
		if inspectErr != nil || observed.Labels["trellis.allocation-id"] != schedulerID || observed.Labels["trellis.allocation-generation"] != labels["trellis.allocation-generation"] {
			return fmt.Errorf("create container %s: %w", allocID, err)
		}
	}
	containerCreated = true
	if err := a.runtime.Start(ctx, allocID); err != nil {
		observed, inspectErr := a.runtime.Inspect(context.WithoutCancel(ctx), allocID)
		if inspectErr != nil || observed.Status != runtime.StatusRunning {
			return fmt.Errorf("start container %s: %w", allocID, err)
		}
	}
	containerStarted = true

	if taskSpec.HealthCheck != nil {
		check := *taskSpec.HealthCheck
		for _, port := range ports {
			if port.ContainerPort == check.Port {
				check.Port = port.HostPort
				break
			}
		}
		a.health.RegisterTask(allocID, allocID, &check)
		healthRegistered = true
	}
	a.reconciler.Track(allocID, taskSpec.HealthCheck != nil, restartPolicy)
	tracked = true

	ready := &Allocation{ID: allocID, AllocationID: schedulerID, Generation: generation, JobRevision: jobRevision, ExecutionHash: executionHash, Restart: restartPolicy, Namespace: namespace, JobName: jobName, GroupName: groupName, TaskName: taskSpec.Name, Spec: taskSpec, ContainerID: allocID, Ports: ports, Mounts: mounts, Network: netAttachment, Status: "running", Health: "unknown"}
	if taskSpec.HealthCheck == nil {
		ready.Health = "healthy"
	}
	if err := a.persistAllocation(ready); err != nil {
		return fmt.Errorf("persist allocation: %w", err)
	}
	a.publishAllocation(ready)
	if taskSpec.HealthCheck == nil {
		if err := a.reconciler.ObserveHealth(allocID, true); err != nil {
			return fmt.Errorf("mark allocation healthy: %w", err)
		}
	}
	committed = true
	return nil
}

func (a *Agent) Logs(ctx context.Context, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	a.mu.RLock()
	allocation := cloneAllocation(a.allocations[allocID])
	a.mu.RUnlock()
	if allocation == nil {
		return nil, fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}
	return a.runtime.Logs(ctx, allocation.ContainerID, follow, tail)
}

func (a *Agent) StopAllocation(ctx context.Context, allocID string) error {
	a.mu.RLock()
	allocation := cloneAllocation(a.allocations[allocID])
	a.mu.RUnlock()
	if allocation == nil {
		return fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}

	var errs []error
	a.health.DeregisterTask(allocID)
	if err := a.reconciler.Untrack(allocID); err != nil {
		errs = append(errs, fmt.Errorf("untrack allocation %s: %w", allocID, err))
	}
	if err := a.runtime.Stop(ctx, allocation.ContainerID); err != nil {
		errs = append(errs, fmt.Errorf("stop container %s: %w", allocation.ContainerID, err))
	}
	if err := a.network.Detach(ctx, allocation.Network); err != nil {
		errs = append(errs, fmt.Errorf("detach allocation network: %w", err))
	}
	if err := a.runtime.Remove(ctx, allocation.ContainerID); err != nil {
		errs = append(errs, fmt.Errorf("remove container %s: %w", allocation.ContainerID, err))
	}
	for _, port := range allocation.Ports {
		if err := a.ports.Release(port); err != nil {
			errs = append(errs, fmt.Errorf("release port %d: %w", port.HostPort, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if err := a.deleteAllocationRecord(allocID); err != nil {
		return fmt.Errorf("delete allocation record: %w", err)
	}
	a.mu.Lock()
	delete(a.allocations, allocID)
	a.mu.Unlock()
	return nil
}
