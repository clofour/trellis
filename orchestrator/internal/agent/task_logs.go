package agent

import (
	"context"
	"fmt"
	"io"
)

// TaskLogs opens logs for one task in a scheduler allocation.
func (a *Agent) TaskLogs(ctx context.Context, allocationID, task string, follow bool, tail int) (io.ReadCloser, error) {
	a.mu.RLock()
	var match *Allocation
	matches := 0
	for _, allocation := range a.allocations {
		if allocation.AllocationID != allocationID {
			continue
		}
		if task != "" && allocation.TaskName != task {
			continue
		}
		match = allocation
		matches++
	}
	a.mu.RUnlock()

	if matches == 0 || match == nil {
		if task == "" {
			return nil, fmt.Errorf("%w: %s", ErrAllocationNotFound, allocationID)
		}
		return nil, fmt.Errorf("%w: allocation %s has no task %q", ErrAllocationNotFound, allocationID, task)
	}
	if task == "" && matches > 1 {
		return nil, fmt.Errorf("allocation %s has multiple tasks; specify task", allocationID)
	}
	return a.runtime.Logs(ctx, match.ContainerID, follow, tail)
}
