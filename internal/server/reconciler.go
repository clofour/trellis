package server

import (
	"context"
	"fmt"

	"github.com/clofour/trellis/internal/api"
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
	var actions []Action
	valid := make([]*Allocation, 0, len(s.allocations))
	for _, allocation := range s.allocations {
		job := s.jobs[allocation.JobName]
		if job == nil || allocation.Status == AllocationStatusUnhealthy || allocation.Revision < job.Revision {
			actions = append(actions, Action{Type: ActionStop, Allocation: allocation})
			continue
		}
		valid = append(valid, allocation)
	}

	for jobName, job := range s.jobs {
		for _, group := range job.Spec.TaskGroups {
			for taskIndex := range group.Tasks {
				task := &group.Tasks[taskIndex]
				var current []*Allocation
				for _, alloc := range valid {
					if alloc.JobName == jobName && alloc.TaskGroupName == group.Name && alloc.Task != nil && alloc.Task.Name == task.Name {
						current = append(current, alloc)
					}
				}
				for len(current) > group.Count {
					actions = append(actions, Action{Type: ActionStop, Allocation: current[len(current)-1]})
					current = current[:len(current)-1]
				}
				missing := group.Count - len(current)
				placements := Schedule(&PlacementIntent{JobName: jobName, TaskGroupName: group.Name, Count: missing, Nodes: s.nodePointers(), Allocations: append(valid, s.allocations...)})
				for _, placement := range placements {
					node := s.nodes[placement.NodeID]
					name := fmt.Sprintf("%s-%s-%s-%s", jobName, group.Name, task.Name, uuid.NewString()[:8])
					actions = append(actions, Action{Type: ActionStart, Allocation: &Allocation{JobName: jobName, TaskGroupName: group.Name, Name: name, Task: task, Node: node, Status: AllocationStatusPending, Revision: job.Revision}})
				}
			}
		}
	}

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
		request := &api.AllocationRequest{JobName: alloc.JobName, GroupName: alloc.TaskGroupName, Name: alloc.Name, Task: alloc.Task}
		if err := s.client.RunAllocation(ctx, address, request); err != nil {
			return err
		}
		s.allocations = append(s.allocations, alloc)
	case ActionStop:
		if err := s.client.StopAllocation(ctx, address, alloc.Name); err != nil {
			return err
		}
		for i, existing := range s.allocations {
			if existing == alloc {
				s.allocations = append(s.allocations[:i], s.allocations[i+1:]...)
				break
			}
		}
	}
	return nil
}
