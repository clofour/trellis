package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/lifecycle"
)

func (a *Agent) reconcileDesired(ctx context.Context, response *api.HeartbeatResponse) {
	if response == nil || !response.OrphanConfirmation {
		return
	}
	if err := a.AcceptEpoch(response.Epoch); err != nil {
		return
	}
	desired := make(map[string]bool, len(response.Desired))
	for _, allocation := range response.Desired {
		desired[fmt.Sprintf("%s/%d", allocation.ID, allocation.Generation)] = true
	}
	a.mu.Lock()
	var collect []string
	for id, allocation := range a.allocations {
		key := fmt.Sprintf("%s/%d", allocation.AllocationID, allocation.Generation)
		if desired[key] {
			delete(a.orphans, key)
			continue
		}
		a.orphans[key]++
		if a.orphans[key] >= 2 {
			collect = append(collect, id)
		}
	}
	a.mu.Unlock()
	for _, id := range collect {
		if err := a.StopAllocation(context.WithoutCancel(ctx), id); err != nil {
			a.log.Error("collect confirmed orphan", "task", id, "error", err)
		}
	}
}

func (a *Agent) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	registered := false
	for {
		if !registered {
			if a.server.Ready() {
				a.mu.RLock()
				nodeInfo := a.nodeInfo
				a.mu.RUnlock()
				if _, err := a.server.RegisterNode(ctx, &nodeInfo); err != nil {
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
			if !a.server.Ready() || !registered {
				continue
			}
			a.mu.RLock()
			actual := make([]api.AllocationStatus, 0, len(a.allocations))
			for _, allocation := range a.allocations {
				ports := make([]api.PortMapping, 0, len(allocation.Ports))
				for _, port := range allocation.Ports {
					ports = append(ports, api.PortMapping{HostPort: port.HostPort, ContainerPort: port.ContainerPort})
				}
				phase, health := lifecycle.Phase(allocation.Status), lifecycle.Health(allocation.Health)
				actual = append(actual, api.AllocationStatus{ID: allocation.AllocationID, Generation: allocation.Generation, Task: allocation.TaskName, Phase: phase, Health: health, Status: lifecycle.CompatibilityStatus(phase, health), Ports: ports})
			}
			a.mu.RUnlock()
			response, err := a.server.SendHeartbeat(ctx, a.nodeID, &client.Heartbeat{NodeID: a.nodeID, Timestamp: time.Now(), Allocations: actual})
			if err != nil {
				a.log.Error("send heartbeat failed", "error", err)
				registered = false
			} else {
				a.reconcileDesired(ctx, response)
			}
		}
	}
}
