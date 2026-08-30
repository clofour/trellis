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
	"github.com/clofour/trellis/internal/spec"
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
		job := s.jobs[jobKey(allocation.Namespace, allocation.JobName)]
		if job == nil || allocation.Status == AllocationStatusUnhealthy || allocation.Revision < job.Revision || allocation.Node == nil || allocation.Node.Status != NodeStatusHealthy {
			actions = append(actions, Action{Type: ActionStop, Allocation: allocation})
			continue
		}
		valid = append(valid, allocation)
	}

	for _, job := range s.jobs {
		jobName := job.Spec.Name
		namespace := job.Spec.Namespace
		for _, group := range job.Spec.TaskGroups {
			var current []*Allocation
			for _, alloc := range valid {
				if alloc.Namespace == namespace && alloc.JobName == jobName && alloc.TaskGroupName == group.Name {
					current = append(current, alloc)
				}
			}
			for len(current) > group.Count {
				actions = append(actions, Action{Type: ActionStop, Allocation: current[len(current)-1]})
				current = current[:len(current)-1]
			}
			placements := Schedule(&PlacementIntent{Namespace: namespace, JobName: jobName, TaskGroupName: group.Name, Count: group.Count - len(current), Nodes: s.nodePointers(), Allocations: valid, Tasks: group.Tasks, Constraints: group.Constraints})
			for _, placement := range placements {
				node := s.nodes[placement.NodeID]
				name := fmt.Sprintf("%s-%s-%s-%s", namespace, jobName, group.Name, uuid.NewString()[:8])
				actions = append(actions, Action{Type: ActionStart, Allocation: &Allocation{Namespace: namespace, JobName: jobName, TaskGroupName: group.Name, Name: name, Tasks: group.Tasks, Node: node, Status: AllocationStatusPending, Revision: job.Revision}})
			}
		}
	}
	s.mu.Unlock()

	for i := range actions {
		if err := s.Execute(ctx, &actions[i]); err != nil {
			s.log.Error("reconcile action failed", "action", actions[i].Type, "allocation", actions[i].Allocation.Name, "error", err)
		}
	}
	s.refreshCatalog()
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
		job := s.jobs[jobKey(alloc.Namespace, alloc.JobName)]
		if job == nil {
			s.mu.RUnlock()
			return fmt.Errorf("job %s was deleted before allocation start", alloc.JobName)
		}
		var groupRuntime string
		var groupNetworkMode string
		var groupAPIAccess bool
		var groupRestart *spec.RestartPolicySpec
		for _, group := range job.Spec.TaskGroups {
			if group.Name == alloc.TaskGroupName {
				groupRuntime = group.Runtime
				groupNetworkMode = group.NetworkMode
				groupAPIAccess = group.APIAccess
				groupRestart = group.Restart
				break
			}
		}
		hostMode := groupNetworkMode == "host"
		wireGuard := job.Spec.Network != nil && job.Spec.Network.WireGuard && !hostMode
		request := &api.AllocationRequest{Namespace: alloc.Namespace, JobName: alloc.JobName, GroupName: alloc.TaskGroupName, Name: alloc.Name, Tasks: alloc.Tasks, Runtime: groupRuntime, WireGuard: wireGuard, NetworkMode: groupNetworkMode, Restart: groupRestart}
		if wireGuard {
			plan, err := s.networkPlan(alloc.Namespace, alloc.Node)
			if err != nil {
				s.mu.RUnlock()
				return err
			}
			request.NetworkPlan = plan
		}
		s.mu.RUnlock()
		if groupAPIAccess && s.tokenManager != nil {
			token, err := s.tokenManager.GetOrCreateNamespaceToken(ctx, alloc.Namespace)
			if err != nil {
				return fmt.Errorf("create namespace token: %w", err)
			}
			request.EnvOverrides = map[string]string{
				"TRELLIS_TOKEN": token,
				"TRELLIS_ADDR":  s.serverAddr,
			}
		}
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

func (s *Server) networkPlan(namespace string, target *Node) (*network.Plan, error) {
	local := namespaceNodeSubnet(s.networkPool, namespace, target.ID)
	plan := &network.Plan{CIDR: local.String(), Gateway: local.Addr().Next().String(), WireGuardAddress: wireGuardAddress(namespace, target.ID)}
	seen := map[string]uuid.UUID{local.String(): target.ID}
	for _, node := range s.nodes {
		if node.ID == target.ID || node.WireGuardPublicKey == "" || node.WireGuardEndpoint == "" {
			continue
		}
		subnet := namespaceNodeSubnet(s.networkPool, namespace, node.ID)
		if previous, ok := seen[subnet.String()]; ok && previous != node.ID {
			return nil, fmt.Errorf("automatic network subnet collision between nodes %s and %s", previous, node.ID)
		}
		seen[subnet.String()] = node.ID
		plan.Peers = append(plan.Peers, network.PeerPlan{PublicKey: node.WireGuardPublicKey, Endpoint: node.WireGuardEndpoint, AllowedIPs: []string{subnet.String()}})
	}
	for _, job := range s.jobs {
		if job.Spec.Network == nil || !job.Spec.Network.WireGuard || job.Spec.Namespace == namespace {
			continue
		}
		for _, node := range s.nodes {
			other := namespaceNodeSubnet(s.networkPool, job.Spec.Namespace, node.ID)
			if owner, exists := seen[other.String()]; exists {
				return nil, fmt.Errorf("automatic network subnet %s for namespace %q conflicts with namespace %q on node %s", other, job.Spec.Namespace, namespace, owner)
			}
		}
	}
	return plan, nil
}

func namespaceNodeSubnet(pool netip.Prefix, namespace string, node uuid.UUID) netip.Prefix {
	h := sha256.Sum256(append([]byte(namespace+"\x00"), node[:]...))
	base := binary.BigEndian.Uint32(pool.Addr().AsSlice())
	available := uint32(1) << uint32(24-pool.Bits())
	index := binary.BigEndian.Uint32(h[:4]) % available
	b := base + index*256
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{byte(b >> 24), byte(b >> 16), byte(b >> 8), byte(b)}), 24)
}

func wireGuardAddress(namespace string, node uuid.UUID) string {
	h := sha256.Sum256(append([]byte(namespace+"wg"), node[:]...))
	return fmt.Sprintf("169.254.%d.%d/32", h[0], max(byte(1), h[1]))
}
