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

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

func agentOperationCode(err error) api.OperationCode {
	var operation *client.AgentOperationError
	if errors.As(err, &operation) {
		return operation.Response.Code
	}
	return ""
}

func (s *Server) Execute(ctx context.Context, action *Action) error {
	switch action.Type {
	case ActionStart:
		return s.executeStart(ctx, action.Allocation)
	case ActionStop:
		return s.executeStop(ctx, action.Allocation)
	default:
		return fmt.Errorf("unknown allocation action %q", action.Type)
	}
}

// executeStart snapshots server-owned state while holding Server.mu before
// Allocation.mu. Once the snapshot is complete, the server lock is released
// before any durable write or RPC, preserving a single lock order without
// holding the global state lock across I/O.
func (s *Server) executeStart(ctx context.Context, alloc *Allocation) error {
	s.mu.RLock()
	alloc.mu.Lock()
	now := s.now().UTC()
	alloc.normalize(now)
	if alloc.Node == nil {
		alloc.mu.Unlock()
		s.mu.RUnlock()
		return fmt.Errorf("allocation %s has no node", alloc.ID)
	}
	job := s.jobs[jobKey(alloc.Namespace, alloc.JobName)]
	if job == nil {
		alloc.mu.Unlock()
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
	request := &api.AllocationRequest{AllocationID: alloc.ID, Generation: alloc.Generation, JobRevision: alloc.JobRevision, Epoch: s.controlEpoch, Namespace: alloc.Namespace, JobName: alloc.JobName, GroupName: alloc.TaskGroupName, Name: alloc.ID, Tasks: alloc.Tasks, Runtime: groupRuntime, WireGuard: wireGuard, NetworkMode: groupNetworkMode, Restart: groupRestart}
	if wireGuard {
		plan, err := s.networkPlanLocked(alloc.Namespace, alloc.Node)
		if err != nil {
			alloc.mu.Unlock()
			s.mu.RUnlock()
			return err
		}
		request.NetworkPlan = plan
	}
	address := fmt.Sprintf("%s:%d", alloc.Node.Host, alloc.Node.Port)
	tokenManager := s.tokenManager
	serverAddr := s.serverAddr
	s.mu.RUnlock()
	defer alloc.mu.Unlock()

	hashInput := *request
	hashInput.Epoch, hashInput.ExecutionHash = 0, ""
	raw, err := json.Marshal(hashInput)
	if err != nil {
		return fmt.Errorf("hash allocation request: %w", err)
	}
	hash := sha256.Sum256(raw)
	request.ExecutionHash = hex.EncodeToString(hash[:])
	if groupAPIAccess && tokenManager != nil {
		token, err := tokenManager.GetOrCreateNamespaceToken(ctx, alloc.Namespace)
		if err != nil {
			return fmt.Errorf("create namespace token: %w", err)
		}
		request.EnvOverrides = map[string]string{"TRELLIS_TOKEN": token, "TRELLIS_ADDR": serverAddr}
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
			next := now.Add(retryDelay(alloc.ID, alloc.Attempt))
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
	return nil
}

func (s *Server) executeStop(ctx context.Context, alloc *Allocation) error {
	s.mu.RLock()
	alloc.mu.Lock()
	now := s.now().UTC()
	alloc.normalize(now)
	if alloc.Node == nil {
		alloc.mu.Unlock()
		s.mu.RUnlock()
		return fmt.Errorf("allocation %s has no node", alloc.ID)
	}
	address := fmt.Sprintf("%s:%d", alloc.Node.Host, alloc.Node.Port)
	nodeStatus := alloc.Node.Status
	epoch := s.controlEpoch
	s.mu.RUnlock()
	defer alloc.mu.Unlock()

	if alloc.Phase != lifecycle.PhaseStopping {
		if err := alloc.Transition(lifecycle.PhaseStopping, now, "", ""); err != nil {
			return err
		}
		if err := s.state.PutAllocation(ctx, alloc); err != nil {
			return err
		}
	}
	if nodeStatus == NodeStatusHealthy || nodeStatus == NodeStatusDraining {
		if err := s.client.StopAllocation(ctx, address, &api.StopAllocationRequest{AllocationID: alloc.ID, Generation: alloc.Generation, Epoch: epoch}); err != nil {
			if code := agentOperationCode(err); code == api.OperationStaleEpoch || code == api.OperationStaleGeneration {
				return err
			}
			alloc.Attempt++
			alloc.Reason, alloc.Message = "agent_stop_failed", err.Error()
			next := now.Add(retryDelay(alloc.ID, alloc.Attempt))
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
	return nil
}

// networkPlanLocked requires s.mu to be held for reading.
func (s *Server) networkPlanLocked(namespace string, target *Node) (*network.Plan, error) {
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
