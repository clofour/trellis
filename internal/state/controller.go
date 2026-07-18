package state

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/clofour/trellis/internal/models"
)

type StateController struct {
	store StateStore

	cluster string
}

const trellisNamespace = "trellis"

func NewStateController(store StateStore, cluster string) *StateController {
	return &StateController{
		store:   store,
		cluster: cluster,
	}
}

func (s *StateController) GetCluster(ctx context.Context) (*models.Cluster, error) {
	key := fmt.Sprintf("%s/%s/meta", trellisNamespace, s.cluster)

	var cluster models.Cluster
	found, err := s.get(ctx, key, &cluster)
	if err != nil {
		return nil, fmt.Errorf("get cluster: %w", err)
	}
	if !found {
		return nil, nil
	}

	return &cluster, nil
}

func (s *StateController) PutCluster(ctx context.Context, cluster *models.Cluster) error {
	key := fmt.Sprintf("%s/%s/meta", trellisNamespace, s.cluster)

	err := s.put(ctx, key, cluster)
	if err != nil {
		return fmt.Errorf("put cluster: %w", err)
	}

	return nil
}

func (s *StateController) PutNode(ctx context.Context, id string, node *models.NodeSummary) error {
	key := fmt.Sprintf("%s/%s/nodes/%s", trellisNamespace, s.cluster, id)

	err := s.put(ctx, key, node)
	if err != nil {
		return fmt.Errorf("put node: %w", err)
	}

	return nil
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
