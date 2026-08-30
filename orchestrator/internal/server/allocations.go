package server

import (
	"time"

	"github.com/clofour/trellis/internal/api"
)

type AllocationListFilter struct {
	Job   string
	Label string // "key:value" or key-existence format
}

// ListAllocations exposes allocations as the public query surface for running
// work. Labels are task-group metadata; address and ports are derived runtime
// details. A blank namespace lists allocations across namespaces for
// cluster-scoped callers.
func (s *Server) ListAllocations(namespace string, filter *AllocationListFilter) api.AllocationListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(api.AllocationListResponse, 0, len(s.allocations))
	for _, allocation := range s.allocations {
		allocation.normalize(time.Now().UTC())
		if namespace != "" && allocation.Namespace != namespace {
			continue
		}
		if filter != nil && filter.Job != "" && allocation.JobName != filter.Job {
			continue
		}

		labels := s.allocationLabelsLocked(allocation)
		if filter != nil && filter.Label != "" && !matchAllocationLabel(labels, filter.Label) {
			continue
		}

		response := api.AllocationResponse{
			ID:               allocation.AllocationID(),
			Job:              allocation.JobName,
			Group:            allocation.TaskGroupName,
			Namespace:        allocation.Namespace,
			Status:           string(allocation.Status),
			Phase:            allocation.Phase,
			Health:           allocation.Health,
			Generation:       allocation.Generation,
			JobRevision:      allocation.JobRevision,
			CreatedAt:        allocation.CreatedAt,
			LastTransitionAt: allocation.TransitionedAt,
			Reason:           allocation.Reason,
			Message:          allocation.Message,
			Attempt:          allocation.Attempt,
			NextRetryAt:      allocation.NextRetryAt,
			Labels:           labels,
			Ports:            allocation.Ports,
		}
		if allocation.Node != nil {
			response.NodeID = allocation.Node.ID
			response.Address = allocation.Node.Host
		}
		result = append(result, response)
	}
	return result
}

func (s *Server) allocationLabelsLocked(allocation *Allocation) map[string]string {
	job := s.jobs[jobKey(allocation.Namespace, allocation.JobName)]
	if job == nil {
		return nil
	}
	for _, group := range job.Spec.TaskGroups {
		if group.Name == allocation.TaskGroupName {
			return group.Labels
		}
	}
	return nil
}

func matchAllocationLabel(labels map[string]string, filter string) bool {
	for i := range filter {
		if filter[i] == ':' {
			key, value := filter[:i], filter[i+1:]
			return labels[key] == value
		}
	}
	_, ok := labels[filter]
	return ok
}
