// Package auth creates and validates Trellis authentication tokens.
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
	"strings"

	"github.com/clofour/trellis/internal/state"
)

// TokenScope is the authorization context attached to an authenticated request.
type TokenScope struct {
	Namespace string `json:"namespace"`
}

// AccessScope controls where a generated API credential may operate.
type AccessScope string

const (
	// AccessNamespace restricts a generated credential to one namespace.
	AccessNamespace AccessScope = "namespace"
	// AccessCluster permits cluster-wide operations allowed by the access level.
	AccessCluster AccessScope = "cluster"
)

// AccessLevel controls whether a generated API credential may mutate state.
type AccessLevel string

const (
	// AccessRead grants observation-only API access.
	AccessRead AccessLevel = "read"
	// AccessWrite grants ordinary mutation API access within the credential scope.
	AccessWrite AccessLevel = "write"
)

const authorizationPrefix = "@trellis-auth:"

// EncodeScope returns the request-context value for a scoped credential.
func EncodeScope(scope AccessScope, access AccessLevel, namespace string) string {
	if scope == AccessCluster {
		return authorizationPrefix + "cluster:" + string(access)
	}
	return authorizationPrefix + "namespace:" + string(access) + ":" + namespace
}

// DecodeScope decodes a generated credential context.
func DecodeScope(value string) (scope AccessScope, access AccessLevel, namespace string, ok bool) {
	if !strings.HasPrefix(value, authorizationPrefix) {
		return "", "", "", false
	}
	parts := strings.Split(value[len(authorizationPrefix):], ":")
	switch {
	case len(parts) == 2 && parts[0] == string(AccessCluster):
		access = AccessLevel(parts[1])
		if access != AccessRead && access != AccessWrite {
			return "", "", "", false
		}
		return AccessCluster, access, "", true
	case len(parts) == 3 && parts[0] == string(AccessNamespace):
		access = AccessLevel(parts[1])
		if (access != AccessRead && access != AccessWrite) || parts[2] == "" {
			return "", "", "", false
		}
		return AccessNamespace, access, parts[2], true
	default:
		return "", "", "", false
	}
}

// TokenManager creates and validates persisted scoped tokens.
type TokenManager struct {
	store   state.Store
	cluster string
}

// NewTokenManager creates a token manager backed by state storage.
func NewTokenManager(store state.Store, cluster string) *TokenManager {
	return &TokenManager{store: store, cluster: cluster}
}

// CreateToken creates and persists a token with the requested scope and access.
func (m *TokenManager) CreateToken(ctx context.Context, scope AccessScope, access AccessLevel, namespace string) (string, error) {
	if scope != AccessNamespace && scope != AccessCluster {
		return "", fmt.Errorf("invalid token scope %q", scope)
	}
	if access != AccessRead && access != AccessWrite {
		return "", fmt.Errorf("invalid token access %q", access)
	}
	if scope == AccessNamespace && namespace == "" {
		return "", fmt.Errorf("namespace token requires a namespace")
	}
	if scope == AccessCluster {
		namespace = ""
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	data, err := json.Marshal(&TokenScope{Namespace: EncodeScope(scope, access, namespace)})
	if err != nil {
		return "", fmt.Errorf("marshal scope: %w", err)
	}
	key := fmt.Sprintf("trellis/%s/tokens/%s", m.cluster, hashHex)
	if err := m.store.Put(ctx, key, data); err != nil {
		return "", fmt.Errorf("store token: %w", err)
	}
	return token, nil
}

// ValidateToken returns the scope for a valid generated token.
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
	if _, _, _, ok := DecodeScope(scope.Namespace); !ok {
		return nil, fmt.Errorf("stored token has invalid authorization scope")
	}
	return &scope, nil
}

// GetOrCreateToken returns the persistent token for the requested scope/access pair.
func (m *TokenManager) GetOrCreateToken(ctx context.Context, scope AccessScope, access AccessLevel, namespace string) (string, error) {
	if scope == AccessCluster {
		namespace = ""
	}
	mapping := fmt.Sprintf("%s/%s/%s", scope, access, namespace)
	mappingHash := sha256.Sum256([]byte(mapping))
	rawKey := fmt.Sprintf("trellis/%s/scoped-tokens/%s", m.cluster, hex.EncodeToString(mappingHash[:]))
	existing, err := m.store.Get(ctx, rawKey)
	if err != nil {
		return "", fmt.Errorf("lookup existing token: %w", err)
	}
	if existing != nil {
		token := string(existing)
		if validated, err := m.ValidateToken(ctx, token); err == nil && validated != nil {
			return token, nil
		}
	}
	token, err := m.CreateToken(ctx, scope, access, namespace)
	if err != nil {
		return "", err
	}
	if err := m.store.Put(ctx, rawKey, []byte(token)); err != nil {
		return "", fmt.Errorf("store scoped token mapping: %w", err)
	}
	return token, nil
}

// ValidateClusterToken compares a token with a stored cluster token hash.
func ValidateClusterToken(clusterHash, candidate string) bool {
	hash := sha256.Sum256([]byte(candidate))
	hashHex := hex.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(hashHex), []byte(clusterHash)) == 1
}
