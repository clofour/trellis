package server

import (
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
		allocation.mu.Lock()
		allocation.normalize(s.now().UTC())
		if namespace != "" && allocation.Namespace != namespace {
			allocation.mu.Unlock()
			continue
		}
		if filter != nil && filter.Job != "" && allocation.JobName != filter.Job {
			allocation.mu.Unlock()
			continue
		}
		labels := s.allocationLabelsLocked(allocation)
		if filter != nil && filter.Label != "" && !matchAllocationLabel(labels, filter.Label) {
			allocation.mu.Unlock()
			continue
		}
		response := api.AllocationResponse{
			ID: allocation.ID, Job: allocation.JobName, Group: allocation.TaskGroupName,
			Namespace: allocation.Namespace, Status: allocation.compatibilityStatus(),
			Phase: allocation.Phase, Health: allocation.Health, Generation: allocation.Generation,
			JobRevision: allocation.JobRevision, CreatedAt: allocation.CreatedAt,
			LastTransitionAt: allocation.TransitionedAt, Reason: allocation.Reason,
			Message: allocation.Message, Attempt: allocation.Attempt,
			NextRetryAt: allocation.NextRetryAt, Labels: labels,
			Ports: append([]api.PortMapping(nil), allocation.Ports...),
		}
		if allocation.Node != nil {
			response.NodeID = allocation.Node.ID
			response.Address = allocation.Node.Host
		}
		allocation.mu.Unlock()
		result = append(result, response)
	}
	return result
}

func (s *Server) AllocationEvents(namespace, id string) (api.AllocationEventListResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, allocation := range s.allocations {
		if allocation.ID != id || namespace != "" && allocation.Namespace != namespace {
			continue
		}
		allocation.mu.Lock()
		if allocation.Events == nil {
			allocation.mu.Unlock()
			return api.AllocationEventListResponse{}, true
		}
		entries := allocation.Events.Entries()
		allocation.mu.Unlock()
		result := make(api.AllocationEventListResponse, len(entries))
		for i, event := range entries {
			result[i] = api.AllocationEventResponse{Phase: event.Phase, Reason: event.Reason, Message: event.Message, At: event.At}
		}
		return result, true
	}
	return nil, false
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
