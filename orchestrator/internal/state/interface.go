package state

import "context"

// StateStore defines persistent key-value state operations.
//
//nolint:revive // StateStore is retained as the established public interface name.
type StateStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) (map[string][]byte, error)
	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
}
