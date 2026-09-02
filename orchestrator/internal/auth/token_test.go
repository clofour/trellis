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
func (m memStore) Put(_ context.Context, key string, value []byte) error { m[key] = value; return nil }
func (m memStore) Delete(_ context.Context, key string) error { delete(m, key); return nil }

func TestTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	mgr := NewTokenManager(memStore{}, "test")

	token, err := mgr.CreateToken(ctx, AccessNamespace, AccessRead, "acme")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := mgr.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	scope, access, namespace, ok := DecodeScope(saved.Namespace)
	if !ok || scope != AccessNamespace || access != AccessRead || namespace != "acme" {
		t.Fatalf("unexpected scope: %#v", saved)
	}

	saved, err = mgr.ValidateToken(ctx, "invalid-token")
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		t.Fatalf("expected nil scope for invalid token, got %#v", saved)
	}
}

func TestGetOrCreateReturnsStableTokenPerScope(t *testing.T) {
	ctx := context.Background()
	mgr := NewTokenManager(memStore{}, "test")

	token1, err := mgr.GetOrCreateToken(ctx, AccessCluster, AccessRead, "")
	if err != nil {
		t.Fatal(err)
	}
	token2, err := mgr.GetOrCreateToken(ctx, AccessCluster, AccessRead, "")
	if err != nil {
		t.Fatal(err)
	}
	if token1 != token2 {
		t.Fatalf("expected stable token, got %q and %q", token1, token2)
	}
	writeToken, err := mgr.GetOrCreateToken(ctx, AccessCluster, AccessWrite, "")
	if err != nil {
		t.Fatal(err)
	}
	if writeToken == token1 {
		t.Fatal("read and write access must use distinct credentials")
	}
}

func TestScopeEncoding(t *testing.T) {
	encoded := EncodeScope(AccessNamespace, AccessWrite, "acme")
	scope, access, namespace, ok := DecodeScope(encoded)
	if !ok || scope != AccessNamespace || access != AccessWrite || namespace != "acme" {
		t.Fatalf("decoded %q as %q/%q/%q ok=%t", encoded, scope, access, namespace, ok)
	}
}
