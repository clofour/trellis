package auth

import (
	"context"
	"strings"
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
	principal := Principal{Kind: CredentialOperator, Scope: AccessNamespace, Access: AccessRead, Namespace: "acme"}

	token, err := mgr.CreateToken(ctx, principal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(token, "trls_op_") {
		t.Fatalf("operator token has unexpected prefix: %q", token)
	}
	saved, err := mgr.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Kind != CredentialOperator || saved.Scope != AccessNamespace || saved.Access != AccessRead || saved.Namespace != "acme" || saved.CreatedAt.IsZero() {
		t.Fatalf("unexpected principal: %#v", saved)
	}

	saved, err = mgr.ValidateToken(ctx, "invalid-token")
	if err != nil {
		t.Fatal(err)
	}
	if saved != nil {
		t.Fatalf("expected nil principal for invalid token, got %#v", saved)
	}
}

func TestWorkloadTokenCarriesWorkloadKind(t *testing.T) {
	ctx := context.Background()
	mgr := NewTokenManager(memStore{}, "test")

	token1, err := mgr.GetOrCreateWorkloadToken(ctx, AccessNamespace, AccessRead, "acme")
	if err != nil {
		t.Fatal(err)
	}
	token2, err := mgr.GetOrCreateWorkloadToken(ctx, AccessNamespace, AccessRead, "acme")
	if err != nil {
		t.Fatal(err)
	}
	if token1 != token2 {
		t.Fatalf("expected stable workload token, got %q and %q", token1, token2)
	}
	if !strings.HasPrefix(token1, "trls_wl_") {
		t.Fatalf("workload token has unexpected prefix: %q", token1)
	}

	saved, err := mgr.ValidateToken(ctx, token1)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Kind != CredentialWorkload || saved.Scope != AccessNamespace || saved.Access != AccessRead || saved.Namespace != "acme" {
		t.Fatalf("unexpected workload principal: %#v", saved)
	}
}

func TestPrincipalValidation(t *testing.T) {
	for _, principal := range []Principal{
		{Kind: CredentialOperator, Scope: AccessNamespace, Access: AccessRead},
		{Kind: CredentialOperator, Scope: AccessCluster, Access: AccessWrite, Namespace: "acme"},
		{Kind: CredentialOperator, Scope: AccessNamespace, Access: AccessRead, Namespace: "acme", Subject: &CredentialSubject{Namespace: "acme", Job: "api", TaskGroup: "web"}},
		{Kind: CredentialWorkload, Scope: AccessNamespace, Access: AccessRead, Namespace: "other", Subject: &CredentialSubject{Namespace: "acme", Job: "api", TaskGroup: "web"}},
	} {
		if err := principal.Validate(); err == nil {
			t.Fatalf("expected principal to be invalid: %#v", principal)
		}
	}
}
