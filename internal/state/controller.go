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

	value, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("get key %s: %w", key, err)
	}

	var cluster *models.Cluster
	err = json.Unmarshal(value, cluster)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", key, err)
	}

	return cluster, nil
}

func (s *StateController) PutCluster(ctx context.Context, cluster *models.Cluster) error {
	value, err := json.Marshal(cluster)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}

	key := fmt.Sprintf("%s/%s/meta", trellisNamespace, s.cluster)

	err = s.store.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("get key %s: %w", key, err)
	}

	return nil
}
