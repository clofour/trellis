package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/clofour/trellis/internal/state"
)

// StateController persists typed server state.
type StateController struct {
	store state.Store

	cluster string
}

const trellisNamespace = "trellis"

// NewStateController creates a typed state controller.
func NewStateController(store state.Store, cluster string) *StateController {
	return &StateController{
		store:   store,
		cluster: cluster,
	}
}

// GetCluster loads cluster state.
func (s *StateController) GetCluster(ctx context.Context) (*Cluster, error) {
	key := fmt.Sprintf("%s/%s/meta", trellisNamespace, s.cluster)

	var cluster Cluster
	found, err := s.get(ctx, key, &cluster)
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}
	if !found {
		return nil, nil
	}

	return &cluster, nil
}

// PutCluster persists cluster state.
func (s *StateController) PutCluster(ctx context.Context, cluster *Cluster) error {
	key := fmt.Sprintf("%s/%s/meta", trellisNamespace, s.cluster)

	err := s.put(ctx, key, cluster)
	if err != nil {
		return fmt.Errorf("put cluster: %w", err)
	}

	return nil
}

// PutNode persists a node summary.
func (s *StateController) PutNode(ctx context.Context, id string, node *NodeSummary) error {
	key := fmt.Sprintf("%s/%s/nodes/%s", trellisNamespace, s.cluster, id)

	err := s.put(ctx, key, node)
	if err != nil {
		return fmt.Errorf("put node: %w", err)
	}

	return nil
}

// ListJobs loads all persisted jobs.
func (s *StateController) ListJobs(ctx context.Context) (map[string]*Job, error) {
	prefix := fmt.Sprintf("%s/%s/jobs/", trellisNamespace, s.cluster)
	values, err := listValues[Job](ctx, s.store, prefix)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*Job, len(values))
	for _, job := range values {
		result[jobKey(job.Spec.Namespace, job.Spec.Name)] = job
	}
	return result, nil
}

// PutJob persists a job.
func (s *StateController) PutJob(ctx context.Context, id string, job *Job) error {
	key := fmt.Sprintf("%s/%s/jobs/%s", trellisNamespace, s.cluster, url.QueryEscape(id))

	err := s.put(ctx, key, job)
	if err != nil {
		return fmt.Errorf("put job: %w", err)
	}

	return nil
}

// DeleteJob removes a persisted job.
func (s *StateController) DeleteJob(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s/%s/jobs/%s", trellisNamespace, s.cluster, url.QueryEscape(id))
	if err := s.store.Delete(ctx, key); err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	return nil
}

// ListNodes loads all persisted nodes.
func (s *StateController) ListNodes(ctx context.Context) (map[string]*NodeSummary, error) {
	prefix := fmt.Sprintf("%s/%s/nodes/", trellisNamespace, s.cluster)
	return listValues[NodeSummary](ctx, s.store, prefix)
}

// ListAllocations loads all persisted allocations.
func (s *StateController) ListAllocations(ctx context.Context) (map[string]*Allocation, error) {
	prefix := fmt.Sprintf("%s/%s/allocations/", trellisNamespace, s.cluster)
	return listValues[Allocation](ctx, s.store, prefix)
}

// PutAllocation persists an allocation.
func (s *StateController) PutAllocation(ctx context.Context, allocation *Allocation) error {
	key := fmt.Sprintf("%s/%s/allocations/%s", trellisNamespace, s.cluster, allocation.ID)
	return s.put(ctx, key, allocation)
}

// DeleteAllocation removes a persisted allocation.
func (s *StateController) DeleteAllocation(ctx context.Context, id string) error {
	key := fmt.Sprintf("%s/%s/allocations/%s", trellisNamespace, s.cluster, id)
	return s.store.Delete(ctx, key)
}

func listValues[T any](ctx context.Context, store state.Store, prefix string) (map[string]*T, error) {
	raw, err := store.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list prefix %s: %w", prefix, err)
	}
	result := make(map[string]*T, len(raw))
	for key, value := range raw {
		var item T
		if err := json.Unmarshal(value, &item); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w", key, err)
		}
		result[key[len(prefix):]] = &item
	}
	return result, nil
}

func (s *StateController) get(ctx context.Context, key string, value any) (bool, error) {
	raw, err := s.store.Get(ctx, key)
	if err != nil {
		return false, fmt.Errorf("get key %s: %w", key, err)
	}
	if raw == nil {
		return false, nil
	}

	err = json.Unmarshal(raw, value)
	if err != nil {
		return true, fmt.Errorf("unmarshal json: %w", err)
	}

	return true, nil
}

func (s *StateController) put(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	err = s.store.Put(ctx, key, raw)
	if err != nil {
		return fmt.Errorf("put key %s: %w", key, err)
	}

	return nil
}
