package server

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/clofour/trellis/internal/lifecycle"
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

// Reconcile only decides what should happen next. Agent RPC execution lives in
// executor.go so placement/lifecycle convergence is separate from side effects.
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
			for _, allocation := range valid {
				if allocation.Namespace == namespace && allocation.JobName == jobName && allocation.TaskGroupName == group.Name {
					current = append(current, allocation)
				}
			}
			for len(current) > group.Count {
				actions = append(actions, Action{Type: ActionStop, Allocation: current[len(current)-1]})
				current = current[:len(current)-1]
			}
			placements := Schedule(&PlacementIntent{Namespace: namespace, JobName: jobName, TaskGroupName: group.Name, Count: group.Count - len(current), Nodes: s.nodePointers(), Allocations: valid, Tasks: group.Tasks, Constraints: group.Constraints})
			for _, placement := range placements {
				node := s.nodes[placement.NodeID]
				id := fmt.Sprintf("%s-%s-%s-%s", namespace, jobName, group.Name, uuid.NewString()[:8])
				allocation := &Allocation{ID: id, Namespace: namespace, JobName: jobName, TaskGroupName: group.Name, Tasks: group.Tasks, Node: node, Generation: 1, JobRevision: job.Revision, Phase: lifecycle.PhasePlaced, Health: lifecycle.HealthUnknown, Diagnostic: lifecycle.Diagnostic{CreatedAt: now, TransitionedAt: now}}
				allocation.normalize(now)
				actions = append(actions, Action{Type: ActionStart, Allocation: allocation})
				s.allocations = append(s.allocations, allocation)
			}
		}
	}
	s.mu.Unlock()

	for i := range actions {
		if err := s.Execute(ctx, &actions[i]); err != nil {
			s.log.Error("reconcile action failed", "action", actions[i].Type, "allocation", actions[i].Allocation.ID, "error", err)
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
