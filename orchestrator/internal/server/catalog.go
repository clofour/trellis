package server

import (
	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/catalog"
	"github.com/clofour/trellis/internal/lifecycle"
)

func (s *Server) ListServices(namespace string, filter *catalog.ListFilter) api.ServiceListResponse {
	return s.catalog.List(namespace, filter)
}

func (s *Server) Catalog() *catalog.ServiceCatalog { return s.catalog }

func (s *Server) refreshCatalog() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespaced := make(map[string][]catalog.ServiceInstance)
	for _, allocation := range s.allocations {
		allocation.mu.Lock()
		allocation.normalize(s.now().UTC())
		if allocation.Phase != lifecycle.PhaseRunning || allocation.Health != lifecycle.HealthHealthy {
			allocation.mu.Unlock()
			continue
		}
		var labels map[string]string
		job := s.jobs[jobKey(allocation.Namespace, allocation.JobName)]
		if job != nil {
			for _, group := range job.Spec.TaskGroups {
				if group.Name == allocation.TaskGroupName {
					labels = group.Labels
					break
				}
			}
		}
		var address string
		if allocation.Node != nil {
			address = allocation.Node.Host
		}
		instance := catalog.ServiceInstance{ID: allocation.ID, Job: allocation.JobName, Group: allocation.TaskGroupName, Address: address, Ports: append([]api.PortMapping(nil), allocation.Ports...), Labels: labels}
		namespace := allocation.Namespace
		allocation.mu.Unlock()
		namespaced[namespace] = append(namespaced[namespace], instance)
	}

	seen := make(map[string]bool)
	for namespace, instances := range namespaced {
		s.catalog.Update(namespace, instances)
		seen[namespace] = true
	}
	for _, allocation := range s.allocations {
		if !seen[allocation.Namespace] {
			s.catalog.Update(allocation.Namespace, nil)
		}
	}
}
