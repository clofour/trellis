package server

import (
	"context"
	"fmt"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/google/uuid"
)

func (s *Server) ListNodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		result = append(result, *node)
	}
	return result
}

func (s *Server) RegisterNode(ctx context.Context, registration *NodeRegistration) error {
	s.mu.RLock()
	status := NodeStatusHealthy
	if existing := s.nodes[registration.ID]; existing != nil && existing.Status == NodeStatusDraining {
		status = NodeStatusDraining
	}
	s.mu.RUnlock()
	now := s.now().UTC()
	summary := &NodeSummary{
		ID: registration.ID, Host: registration.Host, Port: registration.Port,
		CPU: registration.CPU, Memory: registration.Memory, OS: registration.OS,
		Arch: registration.Arch, Labels: registration.Labels, Status: status,
		WireGuardPublicKey: registration.WireGuardPublicKey,
		WireGuardEndpoint: registration.WireGuardEndpoint, LastHeartbeat: now,
	}
	if err := s.state.PutNode(ctx, registration.ID.String(), summary); err != nil {
		return fmt.Errorf("save node remotely: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	node := s.nodes[registration.ID]
	if node == nil {
		node = &Node{ID: registration.ID}
		s.nodes[registration.ID] = node
	}
	node.Host, node.Port = registration.Host, registration.Port
	if node.Status != NodeStatusDraining {
		node.Status = NodeStatusHealthy
	}
	node.LastHeartbeat = now
	node.CPU, node.Memory = registration.CPU, registration.Memory
	node.OS, node.Arch, node.Labels = registration.OS, registration.Arch, registration.Labels
	node.WireGuardPublicKey, node.WireGuardEndpoint = registration.WireGuardPublicKey, registration.WireGuardEndpoint
	return nil
}

func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus) error {
	s.mu.Lock()
	node, ok := s.nodes[nodeID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("node not found")
	}
	if node.Status != NodeStatusDraining {
		node.Status = NodeStatusHealthy
	}
	node.LastHeartbeat = s.now().UTC()
	owned := make([]*Allocation, 0)
	for _, allocation := range s.allocations {
		if allocation.Node == node {
			owned = append(owned, allocation)
		}
	}
	summary := &NodeSummary{ID: node.ID, Host: node.Host, Port: node.Port, CPU: node.CPU, Memory: node.Memory, OS: node.OS, Arch: node.Arch, Labels: node.Labels, Status: node.Status, WireGuardPublicKey: node.WireGuardPublicKey, WireGuardEndpoint: node.WireGuardEndpoint, LastHeartbeat: node.LastHeartbeat}
	s.mu.Unlock()
	if err := s.state.PutNode(ctx, node.ID.String(), summary); err != nil {
		return fmt.Errorf("persist node heartbeat: %w", err)
	}

	type statusInfo struct {
		Generation uint64
		Phase      lifecycle.Phase
		Health     lifecycle.Health
		Ports      []api.PortMapping
		Tasks      int
	}
	statuses := make(map[string]statusInfo, len(actual))
	for _, observed := range actual {
		phase, health := observed.Phase, observed.Health
		if !phase.Valid() || !health.Valid() {
			phase, health = lifecycle.Legacy(observed.Status)
		}
		info := statuses[observed.ID]
		if info.Tasks == 0 {
			info.Generation, info.Phase, info.Health = observed.Generation, phase, health
		} else {
			if phase != lifecycle.PhaseRunning {
				info.Phase = phase
			}
			if info.Health == lifecycle.HealthUnhealthy || health == lifecycle.HealthUnhealthy {
				info.Health = lifecycle.HealthUnhealthy
			} else if info.Health == lifecycle.HealthUnknown || health == lifecycle.HealthUnknown {
				info.Health = lifecycle.HealthUnknown
			} else {
				info.Health = lifecycle.HealthHealthy
			}
		}
		info.Tasks++
		info.Ports = append(info.Ports, observed.Ports...)
		statuses[observed.ID] = info
	}

	var changed []*Allocation
	for _, allocation := range owned {
		allocation.mu.Lock()
		allocation.normalize(s.now().UTC())
		info, ok := statuses[allocation.ID]
		if !ok || (info.Generation != 0 && info.Generation != allocation.Generation) {
			allocation.mu.Unlock()
			continue
		}
		if info.Phase.Valid() && lifecycle.CanTransition(allocation.Phase, info.Phase) {
			_ = allocation.Transition(info.Phase, s.now().UTC(), "", "")
		}
		_ = allocation.SetHealth(info.Health)
		if len(info.Ports) > 0 {
			allocation.Ports = info.Ports
		}
		changed = append(changed, allocation)
		allocation.mu.Unlock()
	}
	for _, allocation := range changed {
		allocation.mu.Lock()
		if err := s.state.PutAllocation(ctx, allocation); err != nil {
			allocation.mu.Unlock()
			return fmt.Errorf("persist allocation observation: %w", err)
		}
		allocation.mu.Unlock()
	}
	s.refreshCatalog()
	return nil
}

func (s *Server) HeartbeatResponse(nodeID uuid.UUID) api.HeartbeatResponse {
	s.mu.RLock()
	epoch, leaderSince := s.controlEpoch, s.leaderSince
	allocations := append([]*Allocation(nil), s.allocations...)
	s.mu.RUnlock()
	response := api.HeartbeatResponse{Epoch: epoch, OrphanConfirmation: !leaderSince.IsZero() && s.now().Sub(leaderSince) >= leaderRecoveryGrace}
	for _, allocation := range allocations {
		allocation.mu.Lock()
		if allocation.Node != nil && allocation.Node.ID == nodeID && allocation.Phase != lifecycle.PhaseStopped && allocation.Phase != lifecycle.PhaseFailed && allocation.Phase != lifecycle.PhaseLost {
			response.Desired = append(response.Desired, api.DesiredAllocation{ID: allocation.ID, Generation: allocation.Generation})
		}
		allocation.mu.Unlock()
	}
	return response
}

func (s *Server) DrainNode(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	node := s.nodes[id]
	if node == nil {
		s.mu.Unlock()
		return fmt.Errorf("node not found")
	}
	previousStatus := node.Status
	node.Status = NodeStatusDraining
	summary := &NodeSummary{ID: node.ID, Host: node.Host, Port: node.Port, CPU: node.CPU, Memory: node.Memory, OS: node.OS, Arch: node.Arch, Labels: node.Labels, Status: node.Status, WireGuardPublicKey: node.WireGuardPublicKey, WireGuardEndpoint: node.WireGuardEndpoint, LastHeartbeat: node.LastHeartbeat}
	s.mu.Unlock()
	if err := s.state.PutNode(ctx, id.String(), summary); err != nil {
		s.mu.Lock()
		if current := s.nodes[id]; current == node && current.Status == NodeStatusDraining {
			current.Status = previousStatus
		}
		s.mu.Unlock()
		return err
	}
	s.Reconcile(ctx)
	return nil
}
