package state

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/api"
)

type ConsulStore struct {
	client *api.Client
}

func NewConsulStore() (*ConsulStore, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, err
	}

	return &ConsulStore{
		client: client,
	}, nil
}

func (c *ConsulStore) Get(ctx context.Context, key string) ([]byte, error) {
	kv := c.client.KV()

	data, _, err := kv.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	if data == nil {
		return nil, nil
	}

	return data.Value, nil
}

func (c *ConsulStore) List(ctx context.Context, prefix string) (map[string][]byte, error) {
	kv := c.client.KV()

	data, _, err := kv.List(prefix, nil)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", prefix, err)
	}

	result := make(map[string][]byte)
	for _, pair := range data {
		result[pair.Key] = pair.Value
	}

	return result, nil
}

func (c *ConsulStore) Put(ctx context.Context, key string, value []byte) error {
	kv := c.client.KV()

	p := &api.KVPair{
		Key:   key,
		Value: value,
	}

	_, err := kv.Put(p, nil)
	if err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}

	return nil
}

func (c *ConsulStore) Delete(ctx context.Context, key string) error {
	kv := c.client.KV()

	_, err := kv.Delete(key, nil)
	if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}

	return nil
}
