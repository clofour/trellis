package server

import (
	"context"
	"fmt"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
)

func jobKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "\x00" + name
}

func requestedResources(jobSpec *spec.JobSpec) (cpu, memory int) {
	for _, group := range jobSpec.TaskGroups {
		for _, task := range group.Tasks {
			if task.Resources != nil {
				cpu += task.Resources.CPU * group.Count
				memory += task.Resources.Memory * group.Count
			}
		}
	}
	return
}

func (s *Server) RegisterJob(ctx context.Context, namespace string, jobSpec *spec.JobSpec) error {
	if err := spec.Validate(jobSpec); err != nil {
		return fmt.Errorf("validate job: %w", err)
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	s.mu.Lock()
	if jobSpec.Namespace != namespace {
		s.mu.Unlock()
		return fmt.Errorf("job namespace does not match request namespace")
	}
	key := jobKey(namespace, jobSpec.Name)
	revision := 1
	if existing := s.jobs[key]; existing != nil {
		revision = existing.Revision + 1
	}
	job := &Job{Spec: jobSpec, Revision: revision}
	s.mu.Unlock()
	if err := s.state.PutJob(ctx, key, job); err != nil {
		return fmt.Errorf("save job remotely: %w", err)
	}
	s.mu.Lock()
	s.jobs[key] = job
	s.mu.Unlock()
	return nil
}

func (s *Server) ListJobs(namespace string) api.JobListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(api.JobListResponse, 0, len(s.jobs))
	for key, job := range s.jobs {
		if job.Spec.Namespace != namespace {
			continue
		}
		r := api.JobStatusResponse{Name: job.Spec.Name, Revision: job.Revision}
		for _, group := range job.Spec.TaskGroups {
			r.Desired += group.Count
		}
		for _, allocation := range s.allocations {
			if jobKey(allocation.Namespace, allocation.JobName) != key {
				continue
			}
			allocation.mu.Lock()
			allocation.normalize(s.now().UTC())
			if allocation.Phase == lifecycle.PhaseRunning {
				r.Running++
			}
			if allocation.Health == lifecycle.HealthHealthy {
				r.Healthy++
			}
			allocation.mu.Unlock()
		}
		result = append(result, r)
	}
	return result
}

func (s *Server) GetJob(namespace, name string) (*api.JobStatusResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobKey(namespace, name)]
	if !ok {
		return nil, false
	}
	specCopy := *job.Spec
	response := &api.JobStatusResponse{Name: name, Revision: job.Revision, Spec: &specCopy}
	for _, group := range job.Spec.TaskGroups {
		response.Desired += group.Count
	}
	for _, allocation := range s.allocations {
		if allocation.Namespace != namespace || allocation.JobName != name {
			continue
		}
		allocation.mu.Lock()
		allocation.normalize(s.now().UTC())
		item := api.AllocationResponse{ID: allocation.ID, Group: allocation.TaskGroupName, Status: allocation.compatibilityStatus(), Phase: allocation.Phase, Health: allocation.Health, Generation: allocation.Generation, JobRevision: allocation.JobRevision, CreatedAt: allocation.CreatedAt, LastTransitionAt: allocation.TransitionedAt, Reason: allocation.Reason, Message: allocation.Message, Attempt: allocation.Attempt, NextRetryAt: allocation.NextRetryAt}
		if allocation.Node != nil {
			item.NodeID = allocation.Node.ID
		}
		if allocation.Phase == lifecycle.PhaseRunning {
			response.Running++
		}
		if allocation.Health == lifecycle.HealthHealthy {
			response.Healthy++
		}
		allocation.mu.Unlock()
		response.Allocations = append(response.Allocations, item)
	}
	return response, true
}

func (s *Server) DeleteJob(ctx context.Context, namespace, name string) error {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	s.mu.RLock()
	key := jobKey(namespace, name)
	_, ok := s.jobs[key]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job %s not found", name)
	}
	if err := s.state.DeleteJob(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.jobs, key)
	s.mu.Unlock()
	s.Reconcile(ctx)
	return nil
}
