package server

import (
	"context"
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
		case allocation.Status == StatusUnhealthy:
			actions = append(actions, Action{
				Type:       ActionStop,
				Allocation: allocation,
			})
			actions = append(actions, Action{
				Type:       ActionStart,
				Allocation: allocation,
			})

		case allocation.Revision < job.Revision:
			actions = append(actions, Action{
				Type:       ActionStop,
				Allocation: allocation,
			})
			actions = append(actions, Action{
				Type:       ActionStart,
				Allocation: allocation,
			})

		case allocation.Status == StatusHealthy:
			if replicaCounts[allocation.JobName] == nil {
				replicaCounts[allocation.JobName] = make(map[string]int)
			}
			replicaCounts[allocation.JobName][allocation.TaskGroupName]++
		}
	}

	for jobName, job := range s.jobs {
		for taskGroupName, taskGroup := range job.TaskGroups {
			desiredCount := taskGroup.Spec.Count
			currentCount := replicaCounts[jobName][taskGroupName]

			if desiredCount < currentCount {
				for i := currentCount; i < desiredCount; i++ {
					actions = append(actions, Action{
						Type: ActionStop,
						Allocation: &Allocation{
							JobName:       jobName,
							TaskGroupName: taskGroupName,
							Status:        StatusPending,
							Revision:      job.Revision,
						},
					})
				}
			}

			if desiredCount > currentCount {
				delta := desiredCount - currentCount

				for i := 0; i < delta; i++ {
					actions = append(actions, Action{
						Type:       ActionStart,
						Allocation: taskGroup.Allocations[i],
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
	return
}
