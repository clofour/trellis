package agent

import (
	"context"
	"time"
)

func (a *Agent) OnHealthy(_ context.Context, allocID string) error {
	return a.updateObservedState(allocID, func(allocation *Allocation) { allocation.Health = "healthy" })
}

func (a *Agent) OnUnhealthy(_ context.Context, allocID string) error {
	return a.updateObservedState(allocID, func(allocation *Allocation) { allocation.Health = "unhealthy" })
}

func (a *Agent) updateObservedState(allocID string, update func(*Allocation)) error {
	a.mu.Lock()
	allocation := a.allocations[allocID]
	if allocation == nil {
		a.mu.Unlock()
		return nil
	}
	update(allocation)
	snapshot := cloneAllocation(allocation)
	a.mu.Unlock()
	return a.persistAllocation(snapshot)
}

func (a *Agent) OnReconciledStatus(allocID, status string) {
	if err := a.updateObservedState(allocID, func(allocation *Allocation) {
		if status == "healthy" || status == "unhealthy" {
			allocation.Health = status
		} else {
			allocation.Status = status
		}
	}); err != nil {
		a.log.Error("persist reconciled allocation", "allocation", allocID, "error", err)
	}
}

func (a *Agent) OnRestartState(allocID string, attempts int, window time.Time) {
	if err := a.updateObservedState(allocID, func(allocation *Allocation) {
		allocation.RestartAttempts, allocation.RestartWindow = attempts, window
	}); err != nil {
		a.log.Error("persist restart tracking", "allocation", allocID, "error", err)
	}
}
