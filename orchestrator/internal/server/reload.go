package server

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Reload reconstructs durable control-plane state before a leadership term
// starts. Persisted records are normalized by StateController, so this method
// only reconnects allocation node references to the current in-memory registry.
func (s *Server) Reload(ctx context.Context) error {
	jobs, err := s.state.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	nodeSummaries, err := s.state.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("load nodes: %w", err)
	}
	allocationMap, err := s.state.ListAllocations(ctx)
	if err != nil {
		return fmt.Errorf("load allocations: %w", err)
	}

	nodes := make(map[uuid.UUID]*Node, len(nodeSummaries))
	for _, summary := range nodeSummaries {
		status := NodeStatusUnhealthy
		if summary.Status == NodeStatusDraining {
			status = NodeStatusDraining
		}
		nodes[summary.ID] = &Node{ID: summary.ID, Host: summary.Host, Port: summary.Port, CPU: summary.CPU, Memory: summary.Memory, OS: summary.OS, Arch: summary.Arch, Labels: summary.Labels, Status: status, WireGuardPublicKey: summary.WireGuardPublicKey, WireGuardEndpoint: summary.WireGuardEndpoint, LastHeartbeat: summary.LastHeartbeat}
	}
	allocations := make([]*Allocation, 0, len(allocationMap))
	for _, allocation := range allocationMap {
		if allocation.Node != nil {
			allocation.Node = nodes[allocation.Node.ID]
		}
		allocations = append(allocations, allocation)
	}

	s.mu.Lock()
	s.jobs, s.nodes, s.allocations = jobs, nodes, allocations
	s.mu.Unlock()
	return nil
}
