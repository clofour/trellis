package server

import (
	"context"
	"testing"

	"github.com/clofour/trellis/internal/auth"
	"github.com/clofour/trellis/internal/spec"
)

type apiAccessStore map[string][]byte

func (m apiAccessStore) Get(_ context.Context, key string) ([]byte, error) { return m[key], nil }
func (m apiAccessStore) List(_ context.Context, prefix string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for key, value := range m {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key] = value
		}
	}
	return result, nil
}
func (m apiAccessStore) Put(_ context.Context, key string, value []byte) error { m[key] = value; return nil }
func (m apiAccessStore) Delete(_ context.Context, key string) error { delete(m, key); return nil }

func TestAPIAccessTokenClusterRead(t *testing.T) {
	manager := auth.NewTokenManager(apiAccessStore{}, "test")
	s := &Server{tokenManager: manager}
	requested := &spec.APIAccessSpec{Scope: spec.APIAccessCluster, Access: spec.APIAccessRead}
	token, err := s.apiAccessToken(context.Background(), requested, "default")
	if err != nil {
		t.Fatalf("cluster/read api access: %v", err)
	}
	stored, err := manager.ValidateToken(context.Background(), token)
	if err != nil || stored == nil {
		t.Fatalf("validate generated token: %v", err)
	}
	scope, access, namespace, ok := auth.DecodeScope(stored.Namespace)
	if !ok || scope != auth.AccessCluster || access != auth.AccessRead || namespace != "" {
		t.Fatalf("unexpected generated scope: %q", stored.Namespace)
	}
}

func TestAPIAccessTokenNone(t *testing.T) {
	s := &Server{}
	token, err := s.apiAccessToken(context.Background(), nil, "default")
	if err != nil {
		t.Fatalf("disabled api access: %v", err)
	}
	if token != "" {
		t.Fatalf("disabled api access returned token %q", token)
	}
}
