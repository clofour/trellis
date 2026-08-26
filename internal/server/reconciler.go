package server

import (
	"context"

	"github.com/clofour/trellis/internal/client"
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

func (s *Server) Reconcile(ctx context.Context) {
	var actions []Action

	replicaCounts := make(map[string]map[string]int)

	for _, allocation := range s.allocations {
		job, found := s.jobs[allocation.JobName]

		if !found {
			actions = append(actions, Action{
				Type:       ActionStop,
				Allocation: allocation,
			})
			continue
		}

		switch {
		case allocation.Status == AllocationStatusUnhealthy:
			actions = append(actions, Action{
				Type:       ActionStop,
				Allocation: allocation,
			})

		case allocation.Revision < job.Revision:
			actions = append(actions, Action{
				Type:       ActionStop,
				Allocation: allocation,
			})

		case allocation.Status == AllocationStatusHealthy:
			if replicaCounts[allocation.JobName] == nil {
				replicaCounts[allocation.JobName] = make(map[string]int)
			}
			replicaCounts[allocation.JobName][allocation.TaskGroupName]++
		}
	}

	for jobName, job := range s.jobs {
		spec := job.Spec
		for _, taskGroup := range spec.TaskGroups {
			taskGroupName := taskGroup.Name

			desiredCount := taskGroup.Count
			currentCount := replicaCounts[jobName][taskGroupName]

			if desiredCount < currentCount {
				delta := currentCount - desiredCount

				for i := 0; i < delta; i++ {
					actions = append(actions, Action{
						Type: ActionStop,
						Allocation: ,
					})
				}
			}

			if desiredCount > currentCount {
				delta := desiredCount - currentCount

				placements := Schedule(&PlacementIntent{
						JobName: jobName,
						TaskGroupName: taskGroupName,
						Count: delta,
						Nodes: s.Nodes,
						Allocations: s.Allocations,
				})

				for _, placement := range placements {
					actions = append(actions, Action{
						Type:       ActionStart,
						Allocation: &Allocation{
							JobName:       jobName,
							TaskGroupName: taskGroupName,
							Status:        AllocationStatusPending,
							Revision:      job.Revision,
						},
					})
				}
			}
		}
	}

	for _, action := range actions {
		s.Execute(ctx, &action)
	}
}

func (s *Server) Execute(ctx context.Context, action *Action) {
	alloc := action.Allocation

	switch {
	case action.Type == ActionStart:
		s.client.RunAllocation(ctx, alloc.Node.Host, &client.NodeInfo{})

	case action.Type == ActionStop:
		s.client.StopAllocation(ctx, alloc.Node.Host, alloc.Name)
	}
}
