package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/storage"
)

func (a *Agent) ConfigureDurability(local *storage.LocalStorage, cluster string) {
	a.mu.Lock()
	a.local, a.cluster = local, cluster
	a.mu.Unlock()
}

func (a *Agent) AcceptEpoch(epoch uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if epoch < a.epoch {
		return fmt.Errorf("%w: received %d, highest accepted %d", ErrStaleEpoch, epoch, a.epoch)
	}
	if epoch == a.epoch {
		return nil
	}
	if a.local != nil {
		if err := a.local.Put("agent/control-epoch", epoch); err != nil {
			return fmt.Errorf("persist control-plane epoch: %w", err)
		}
	}
	a.epoch = epoch
	return nil
}

func allocationRecordKey(id string) string {
	return "agent/allocations/" + base64.RawURLEncoding.EncodeToString([]byte(id))
}

func (a *Agent) persistAllocation(allocation *Allocation) error {
	a.mu.RLock()
	local := a.local
	a.mu.RUnlock()
	if local == nil {
		return nil
	}
	return local.Put(allocationRecordKey(allocation.ID), allocation)
}

func (a *Agent) deleteAllocationRecord(id string) error {
	a.mu.RLock()
	local := a.local
	a.mu.RUnlock()
	if local == nil {
		return nil
	}
	return local.Delete(allocationRecordKey(id))
}

func (a *Agent) recover(ctx context.Context) error {
	a.mu.RLock()
	local, cluster := a.local, a.cluster
	a.mu.RUnlock()
	if local == nil {
		return nil
	}
	var epoch uint64
	if err := local.Get("agent/control-epoch", &epoch); err == nil {
		a.mu.Lock()
		a.epoch = epoch
		a.mu.Unlock()
	}

	records, recordErrs := local.ListRaw("agent/allocations")
	stored := make(map[string]*Allocation, len(records))
	for name, raw := range records {
		var allocation Allocation
		if err := json.Unmarshal(raw, &allocation); err != nil {
			a.log.Error("skip malformed allocation record", "record", name, "error", err)
			continue
		}
		stored[allocation.ContainerID] = &allocation
	}
	for _, err := range recordErrs {
		a.log.Error("read allocation recovery record", "error", err)
	}

	managed, ok := a.runtime.(runtime.ManagedRuntime)
	if !ok {
		return nil
	}
	containers, err := managed.ListManaged(ctx, cluster)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(containers))
	for _, container := range containers {
		seen[container.ID] = true
		allocation := stored[container.ID]
		hadRecord := allocation != nil
		if allocation == nil {
			generation, _ := strconv.ParseUint(container.Labels["trellis.allocation-generation"], 10, 64)
			allocation = &Allocation{ID: container.ID, ContainerID: container.ID, AllocationID: container.Labels["trellis.allocation-id"], Generation: generation, Namespace: container.Labels["trellis.namespace"], JobName: container.Labels["trellis.job"], GroupName: container.Labels["trellis.task-group"], TaskName: container.Labels["trellis.task"], Status: "running", Health: "unknown"}
		}
		if allocation.AllocationID == "" || allocation.Generation == 0 {
			a.log.Warn("leave unidentifiable Trellis container untouched", "container", container.ID)
			continue
		}
		if hadRecord && allocation.Status == "starting" && (container.Status == runtime.StatusCreated || container.Status == runtime.StatusStopped) {
			if err := a.runtime.Start(ctx, container.ID); err != nil {
				a.log.Error("resume interrupted allocation start", "container", container.ID, "error", err)
				continue
			}
			allocation.Status = "running"
			container.Status = runtime.StatusRunning
		}
		if container.Status != runtime.StatusRunning && container.Status != runtime.StatusCreated && container.Status != runtime.StatusStopped {
			continue
		}
		for _, port := range allocation.Ports {
			if err := a.ports.Adopt(port); err != nil {
				a.log.Error("recover port claim", "allocation", allocation.AllocationID, "error", err)
			}
		}
		a.publishAllocation(allocation)
		if allocation.Spec != nil {
			if allocation.Spec.HealthCheck != nil {
				check := *allocation.Spec.HealthCheck
				for _, port := range allocation.Ports {
					if port.ContainerPort == check.Port {
						check.Port = port.HostPort
						break
					}
				}
				a.health.RegisterTask(allocation.ID, allocation.ContainerID, &check)
			}
			a.reconciler.TrackRecovered(allocation.ID, allocation.Spec.HealthCheck != nil, allocation.Restart, allocation.RestartAttempts, allocation.RestartWindow)
		} else {
			a.reconciler.Track(allocation.ID, false, nil)
		}
		if err := a.persistAllocation(allocation); err != nil {
			a.log.Error("refresh recovered allocation record", "allocation", allocation.AllocationID, "error", err)
		}
	}

	for containerID, allocation := range stored {
		if containerID == "" || seen[containerID] {
			continue
		}
		for _, port := range allocation.Ports {
			_ = a.ports.Adopt(port)
			_ = a.ports.Release(port)
		}
		_ = a.network.Detach(context.WithoutCancel(ctx), allocation.Network)
		_ = a.deleteAllocationRecord(allocation.ID)
	}
	return nil
}
