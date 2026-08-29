package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/netip"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/network"
	"github.com/google/uuid"
)

type ActionType string

const (
	ActionStart ActionType = "start"
	ActionStop  ActionType = "stop"
)

type Action struct {
	Type       ActionType
	Allocation *Allocation
}

// Reconcile converges the in-memory allocation set on the latest job specs.
func (s *Server) Reconcile(ctx context.Context) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	s.mu.Lock()
	for _, node := range s.nodes {
		if node.Status == NodeStatusHealthy && time.Since(node.LastHeartbeat) > 3*heartbeatInterval {
			node.Status = NodeStatusUnhealthy
		}
	}
	var actions []Action
	valid := make([]*Allocation, 0, len(s.allocations))
	for _, allocation := range s.allocations {
		job := s.jobs[jobKey(allocation.Tenant, allocation.JobName)]
		if job == nil || allocation.Status == AllocationStatusUnhealthy || allocation.Revision < job.Revision || allocation.Node == nil || allocation.Node.Status != NodeStatusHealthy {
			actions = append(actions, Action{Type: ActionStop, Allocation: allocation})
			continue
		}
		valid = append(valid, allocation)
	}

	for _, job := range s.jobs {
		jobName := job.Spec.Name
		tenant := job.Spec.Tenant
		for _, group := range job.Spec.TaskGroups {
			for taskIndex := range group.Tasks {
				task := &group.Tasks[taskIndex]
				var current []*Allocation
				for _, alloc := range valid {
					if alloc.Tenant == tenant && alloc.JobName == jobName && alloc.TaskGroupName == group.Name && alloc.Task != nil && alloc.Task.Name == task.Name {
						current = append(current, alloc)
					}
				}
				for len(current) > group.Count {
					actions = append(actions, Action{Type: ActionStop, Allocation: current[len(current)-1]})
					current = current[:len(current)-1]
				}
				missing := group.Count - len(current)
				placements := Schedule(&PlacementIntent{JobName: jobName, TaskGroupName: group.Name, Count: missing, Nodes: s.nodePointers(), Allocations: valid, Task: task})
				for _, placement := range placements {
					node := s.nodes[placement.NodeID]
					name := fmt.Sprintf("%s-%s-%s-%s", jobName, group.Name, task.Name, uuid.NewString()[:8])
					actions = append(actions, Action{Type: ActionStart, Allocation: &Allocation{Tenant: tenant, JobName: jobName, TaskGroupName: group.Name, Name: name, Task: task, Node: node, Status: AllocationStatusPending, Revision: job.Revision}})
				}
			}
		}
	}
	s.mu.Unlock()

	for i := range actions {
		if err := s.Execute(ctx, &actions[i]); err != nil {
			s.log.Error("reconcile action failed", "action", actions[i].Type, "allocation", actions[i].Allocation.Name, "error", err)
		}
	}
}

func (s *Server) nodePointers() []*Node {
	nodes := make([]*Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (s *Server) Execute(ctx context.Context, action *Action) error {
	alloc := action.Allocation
	address := fmt.Sprintf("%s:%d", alloc.Node.Host, alloc.Node.Port)
	switch action.Type {
	case ActionStart:
		s.mu.RLock()
		job := s.jobs[jobKey(alloc.Tenant, alloc.JobName)]
		if job == nil {
			s.mu.RUnlock()
			return fmt.Errorf("job %s was deleted before allocation start", alloc.JobName)
		}
		request := &api.AllocationRequest{Tenant: alloc.Tenant, JobName: alloc.JobName, GroupName: alloc.TaskGroupName, Name: alloc.Name, Task: alloc.Task, Isolation: job.Spec.Isolation}
		if job.Spec.Isolation != nil {
			plan, err := s.networkPlan(alloc.Tenant, alloc.Node)
			if err != nil {
				s.mu.RUnlock()
				return err
			}
			request.NetworkPlan = plan
		}
		s.mu.RUnlock()
		if err := s.state.PutAllocation(ctx, alloc); err != nil {
			return fmt.Errorf("persist allocation: %w", err)
		}
		if err := s.client.RunAllocation(ctx, address, request); err != nil {
			_ = s.state.DeleteAllocation(ctx, alloc.Name)
			return err
		}
		s.mu.Lock()
		s.allocations = append(s.allocations, alloc)
		s.mu.Unlock()
	case ActionStop:
		if alloc.Node.Status == NodeStatusHealthy || alloc.Node.Status == NodeStatusDraining {
			if err := s.client.StopAllocation(ctx, address, alloc.Name); err != nil {
				return err
			}
		}
		if err := s.state.DeleteAllocation(ctx, alloc.Name); err != nil {
			return fmt.Errorf("delete allocation state: %w", err)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, existing := range s.allocations {
			if existing == alloc {
				s.allocations = append(s.allocations[:i], s.allocations[i+1:]...)
				break
			}
		}
	}
	return nil
}

func (s *Server) networkPlan(tenant string, target *Node) (*network.Plan, error) {
	local := tenantNodeSubnet(s.networkPool, tenant, target.ID)
	plan := &network.Plan{CIDR: local.String(), Gateway: local.Addr().Next().String(), WireGuardAddress: wireGuardAddress(tenant, target.ID)}
	seen := map[string]uuid.UUID{local.String(): target.ID}
	for _, node := range s.nodes {
		if node.ID == target.ID || node.WireGuardPublicKey == "" || node.WireGuardEndpoint == "" {
			continue
		}
		subnet := tenantNodeSubnet(s.networkPool, tenant, node.ID)
		if previous, ok := seen[subnet.String()]; ok && previous != node.ID {
			return nil, fmt.Errorf("automatic network subnet collision between nodes %s and %s", previous, node.ID)
		}
		seen[subnet.String()] = node.ID
		plan.Peers = append(plan.Peers, network.PeerPlan{PublicKey: node.WireGuardPublicKey, Endpoint: node.WireGuardEndpoint, AllowedIPs: []string{subnet.String()}})
	}
	for _, job := range s.jobs {
		if job.Spec.Isolation == nil || job.Spec.Tenant == tenant {
			continue
		}
		for _, node := range s.nodes {
			other := tenantNodeSubnet(s.networkPool, job.Spec.Tenant, node.ID)
			if owner, exists := seen[other.String()]; exists {
				return nil, fmt.Errorf("automatic network subnet %s for tenant %q conflicts with tenant %q on node %s", other, job.Spec.Tenant, tenant, owner)
			}
		}
	}
	return plan, nil
}

func tenantNodeSubnet(pool netip.Prefix, tenant string, node uuid.UUID) netip.Prefix {
	h := sha256.Sum256(append([]byte(tenant+"\x00"), node[:]...))
	base := binary.BigEndian.Uint32(pool.Addr().AsSlice())
	available := uint32(1) << uint32(24-pool.Bits())
	index := binary.BigEndian.Uint32(h[:4]) % available
	b := base + index*256
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{byte(b >> 24), byte(b >> 16), byte(b >> 8), byte(b)}), 24)
}

func wireGuardAddress(tenant string, node uuid.UUID) string {
	h := sha256.Sum256(append([]byte(tenant+"wg"), node[:]...))
	return fmt.Sprintf("169.254.%d.%d/32", h[0], max(byte(1), h[1]))
}
