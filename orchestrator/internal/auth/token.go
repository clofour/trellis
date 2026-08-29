package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/clofour/trellis/internal/state"
)

type TokenScope struct {
	Namespace string `json:"namespace"`
}

type TokenManager struct {
	store   state.StateStore
	cluster string
}

func NewTokenManager(store state.StateStore, cluster string) *TokenManager {
	return &TokenManager{store: store, cluster: cluster}
}

func (m *TokenManager) CreateNamespaceToken(ctx context.Context, namespace string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	scope := &TokenScope{Namespace: namespace}
	data, err := json.Marshal(scope)
	if err != nil {
		return "", fmt.Errorf("marshal scope: %w", err)
	}

	key := fmt.Sprintf("trellis/%s/tokens/%s", m.cluster, hashHex)
	if err := m.store.Put(ctx, key, data); err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}

	rawKey := fmt.Sprintf("trellis/%s/namespace-tokens/%s", m.cluster, namespace)
	if err := m.store.Put(ctx, rawKey, []byte(token)); err != nil {
		return "", fmt.Errorf("store namespace token mapping: %w", err)
	}

	return token, nil
}

func (m *TokenManager) ValidateToken(ctx context.Context, rawToken string) (*TokenScope, error) {
	hash := sha256.Sum256([]byte(rawToken))
	hashHex := hex.EncodeToString(hash[:])

	key := fmt.Sprintf("trellis/%s/tokens/%s", m.cluster, hashHex)
	data, err := m.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("lookup token: %w", err)
	}
	if data == nil {
		return nil, nil
	}

	var scope TokenScope
	if err := json.Unmarshal(data, &scope); err != nil {
		return nil, fmt.Errorf("unmarshal scope: %w", err)
	}
	return &scope, nil
}

func (m *TokenManager) GetOrCreateNamespaceToken(ctx context.Context, namespace string) (string, error) {
	rawKey := fmt.Sprintf("trellis/%s/namespace-tokens/%s", m.cluster, namespace)
	existing, err := m.store.Get(ctx, rawKey)
	if err != nil {
		return "", fmt.Errorf("lookup existing token: %w", err)
	}
	if existing != nil {
		token := string(existing)
		hash := sha256.Sum256([]byte(token))
		hashHex := hex.EncodeToString(hash[:])
		key := fmt.Sprintf("trellis/%s/tokens/%s", m.cluster, hashHex)
		data, err := m.store.Get(ctx, key)
		if err == nil && data != nil {
			return token, nil
		}
	}
	return m.CreateNamespaceToken(ctx, namespace)
}

func ValidateClusterToken(clusterHash, candidate string) bool {
	hash := sha256.Sum256([]byte(candidate))
	hashHex := hex.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(hashHex), []byte(clusterHash)) == 1
}
