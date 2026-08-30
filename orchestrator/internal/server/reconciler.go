package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/lifecycle"
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

const (
	allocationLossTimeout = 45 * time.Second
	leaderRecoveryGrace   = 30 * time.Second
	maxExecutionAttempts  = 8
)

func retryDelay(id string, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	shift := min(attempt-1, 6)
	base := time.Second * time.Duration(1<<shift)
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", id, attempt)))
	return base + time.Duration(binary.BigEndian.Uint16(h[:2])%500)*time.Millisecond
}

func agentOperationCode(err error) api.OperationCode {
	var operation *client.AgentOperationError
	if errors.As(err, &operation) {
		return operation.Response.Code
	}
	return ""
}

// Reconcile converges the in-memory allocation set on the latest job specs.
func (s *Server) Reconcile(ctx context.Context) {
	start := time.Now()
	s.reconcileMu.Lock()
	defer func() {
		s.reconcileMu.Unlock()
		if s.metrics != nil {
			s.metrics.ReconcileDuration.Observe(time.Since(start).Seconds())
		}
	}()
	now := s.now().UTC()
	s.mu.Lock()
	for _, node := range s.nodes {
		if node.Status == NodeStatusHealthy && now.Sub(node.LastHeartbeat) > 3*heartbeatInterval {
			node.Status = NodeStatusUnhealthy
		}
	}
	var actions []Action
	valid := make([]*Allocation, 0, len(s.allocations))
	for _, allocation := range s.allocations {
		allocation.mu.Lock()
		allocation.normalize(now)
		job := s.jobs[jobKey(allocation.Namespace, allocation.JobName)]
		if allocation.Phase == lifecycle.PhaseStopped || allocation.Phase == lifecycle.PhaseFailed || allocation.Phase == lifecycle.PhaseLost {
			allocation.mu.Unlock()
			continue
		}
		if allocation.NextRetryAt != nil && now.Before(*allocation.NextRetryAt) {
			valid = append(valid, allocation)
			allocation.mu.Unlock()
			continue
		}
		if job == nil || allocation.JobRevision < job.Revision {
			actions = append(actions, Action{Type: ActionStop, Allocation: allocation})
			allocation.mu.Unlock()
			continue
		}
		if allocation.Node == nil || allocation.Node.Status != NodeStatusHealthy {
			if now.Sub(s.leaderSince) >= leaderRecoveryGrace && allocation.Node != nil && !allocation.Node.LastHeartbeat.IsZero() && now.Sub(allocation.Node.LastHeartbeat) >= allocationLossTimeout {
				_ = allocation.Transition(lifecycle.PhaseLost, now, "node_unavailable", "node did not re-register before the allocation loss timeout")
				_ = s.state.PutAllocation(context.WithoutCancel(ctx), allocation)
			}
			allocation.mu.Unlock()
			continue
		}
		if allocation.Phase == lifecycle.PhasePlaced || allocation.Phase == lifecycle.PhaseStarting || allocation.Phase == lifecycle.PhaseStopping {
			actionType := ActionStart
			if allocation.Phase == lifecycle.PhaseStopping {
				actionType = ActionStop
			}
			actions = append(actions, Action{Type: actionType, Allocation: allocation})
		}
		valid = append(valid, allocation)
		allocation.mu.Unlock()
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
				allocation := &Allocation{ID: name, Name: name, Namespace: namespace, JobName: jobName, TaskGroupName: group.Name, Tasks: group.Tasks, Node: node, Generation: 1, JobRevision: job.Revision, Revision: job.Revision, Phase: lifecycle.PhasePlaced, Health: lifecycle.HealthUnknown, Diagnostic: lifecycle.Diagnostic{CreatedAt: now, TransitionedAt: now}}
				allocation.normalize(now)
				actions = append(actions, Action{Type: ActionStart, Allocation: allocation})
				s.allocations = append(s.allocations, allocation)
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
	alloc.mu.Lock()
	defer alloc.mu.Unlock()
	now := s.now().UTC()
	alloc.normalize(now)
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
		request := &api.AllocationRequest{AllocationID: alloc.AllocationID(), Generation: alloc.Generation, JobRevision: alloc.JobRevision, Epoch: s.controlEpoch, Namespace: alloc.Namespace, JobName: alloc.JobName, GroupName: alloc.TaskGroupName, Name: alloc.AllocationID(), Tasks: alloc.Tasks, Runtime: groupRuntime, WireGuard: wireGuard, NetworkMode: groupNetworkMode, Restart: groupRestart}
		if wireGuard {
			plan, err := s.networkPlan(alloc.Namespace, alloc.Node)
			if err != nil {
				s.mu.RUnlock()
				return err
			}
			request.NetworkPlan = plan
		}
		s.mu.RUnlock()
		hashInput := *request
		hashInput.Epoch, hashInput.ExecutionHash = 0, ""
		raw, _ := json.Marshal(hashInput)
		hash := sha256.Sum256(raw)
		request.ExecutionHash = hex.EncodeToString(hash[:])
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
		if alloc.Phase == lifecycle.PhasePlaced || alloc.Phase == lifecycle.PhaseStopped || alloc.Phase == lifecycle.PhaseFailed || alloc.Phase == lifecycle.PhaseLost {
			if err := alloc.Transition(lifecycle.PhaseStarting, now, "", ""); err != nil {
				return err
			}
		}
		if err := s.state.PutAllocation(ctx, alloc); err != nil {
			return fmt.Errorf("persist allocation: %w", err)
		}
		if err := s.client.RunAllocation(ctx, address, request); err != nil {
			if code := agentOperationCode(err); code == api.OperationStaleEpoch {
				return err
			} else if code == api.OperationStaleGeneration || code == api.OperationConflict {
				_ = alloc.Transition(lifecycle.PhaseFailed, now, string(code), err.Error())
				alloc.NextRetryAt = nil
				_ = s.state.PutAllocation(context.WithoutCancel(ctx), alloc)
				return err
			}
			alloc.Attempt++
			alloc.Reason, alloc.Message = "agent_start_failed", err.Error()
			if alloc.Attempt >= maxExecutionAttempts {
				_ = alloc.Transition(lifecycle.PhaseFailed, now, "retry_limit", err.Error())
				alloc.NextRetryAt = nil
			} else {
				next := now.Add(retryDelay(alloc.AllocationID(), alloc.Attempt))
				alloc.NextRetryAt = &next
			}
			_ = s.state.PutAllocation(context.WithoutCancel(ctx), alloc)
			return err
		}
		alloc.Attempt, alloc.NextRetryAt = 0, nil
		_ = alloc.Transition(lifecycle.PhaseRunning, now, "", "")
		if err := s.state.PutAllocation(ctx, alloc); err != nil {
			return fmt.Errorf("persist running allocation: %w", err)
		}
	case ActionStop:
		if alloc.Phase != lifecycle.PhaseStopping {
			if err := alloc.Transition(lifecycle.PhaseStopping, now, "", ""); err != nil {
				return err
			}
			if err := s.state.PutAllocation(ctx, alloc); err != nil {
				return err
			}
		}
		if alloc.Node.Status == NodeStatusHealthy || alloc.Node.Status == NodeStatusDraining {
			if err := s.client.StopAllocation(ctx, address, &api.StopAllocationRequest{AllocationID: alloc.AllocationID(), Generation: alloc.Generation, Epoch: s.controlEpoch}); err != nil {
				if code := agentOperationCode(err); code == api.OperationStaleEpoch || code == api.OperationStaleGeneration {
					return err
				}
				alloc.Attempt++
				alloc.Reason, alloc.Message = "agent_stop_failed", err.Error()
				next := now.Add(retryDelay(alloc.AllocationID(), alloc.Attempt))
				alloc.NextRetryAt = &next
				_ = s.state.PutAllocation(context.WithoutCancel(ctx), alloc)
				return err
			}
		}
		alloc.Attempt, alloc.NextRetryAt = 0, nil
		_ = alloc.Transition(lifecycle.PhaseStopped, now, "", "")
		if err := s.state.PutAllocation(ctx, alloc); err != nil {
			return fmt.Errorf("persist stopped allocation: %w", err)
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
