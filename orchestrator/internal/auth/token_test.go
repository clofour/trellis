package auth

import (
	"context"
	"testing"
)

type memStore map[string][]byte

func (m memStore) Get(_ context.Context, key string) ([]byte, error) { return m[key], nil }
func (m memStore) List(_ context.Context, prefix string) (map[string][]byte, error) {
	r := map[string][]byte{}
	for k, v := range m {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			r[k] = v
		}
	}
	return r, nil
}
func (m memStore) Put(_ context.Context, key string, value []byte) error {
	m[key] = value
	return nil
}
func (m memStore) Delete(_ context.Context, key string) error { delete(m, key); return nil }

func TestTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	mgr := NewTokenManager(memStore{}, "test")

	token, err := mgr.CreateNamespaceToken(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	scope, err := mgr.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if scope == nil || scope.Namespace != "acme" {
		t.Fatalf("unexpected scope: %#v", scope)
	}

	scope, err = mgr.ValidateToken(ctx, "invalid-token")
	if err != nil {
		t.Fatal(err)
	}
	if scope != nil {
		t.Fatalf("expected nil scope for invalid token, got %#v", scope)
	}
}

func TestGetOrCreateReturnsStableToken(t *testing.T) {
	ctx := context.Background()
	mgr := NewTokenManager(memStore{}, "test")

	token1, err := mgr.GetOrCreateNamespaceToken(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	token2, err := mgr.GetOrCreateNamespaceToken(ctx, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if token1 != token2 {
		t.Fatalf("expected stable token, got %q and %q", token1, token2)
	}
}
