package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/auth"
	"github.com/clofour/trellis/internal/catalog"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"
	"github.com/google/uuid"
)

const reconcileInterval = 10 * time.Second
const heartbeatInterval = 10 * time.Second

type ClusterJoiner interface {
	AddVoter(id, address string) error
}

type Server struct {
	log     *slog.Logger
	storage *storage.LocalStorage
	state   *StateController
	client  *client.AgentClient

	cluster      *Cluster
	nodes        map[uuid.UUID]*Node
	jobs         map[string]*Job
	allocations  []*Allocation
	networkPool  netip.Prefix
	tokenManager *auth.TokenManager
	catalog      *catalog.ServiceCatalog
	serverAddr   string
	joiner       ClusterJoiner
	clientTLS    *tls.Config

	// Locking contract:
	//   mutationMu serializes durable API mutations and leadership epochs.
	//   reconcileMu prevents overlapping reconcile passes.
	//   mu protects server-owned maps, slices, cluster metadata and node state.
	//   Allocation.mu protects mutable fields on an allocation.
	// When both are required, acquire Server.mu before Allocation.mu. Never
	// perform the inverse; executor.go snapshots server state in that order.
	mu          sync.RWMutex
	reconcileMu sync.Mutex
	mutationMu  sync.Mutex

	controlEpoch uint64
	leaderSince  time.Time
	now          func() time.Time
	metrics      *Metrics
}

func NewServer(log *slog.Logger, storage *storage.LocalStorage, state *StateController, store state.StateStore, cluster, serverAddr string) *Server {
	pool := netip.MustParsePrefix("10.64.0.0/10")
	return &Server{
		log:          log.With("component", "server"),
		storage:      storage,
		state:        state,
		client:       &client.AgentClient{},
		nodes:        make(map[uuid.UUID]*Node),
		jobs:         make(map[string]*Job),
		networkPool:  pool,
		tokenManager: auth.NewTokenManager(store, cluster),
		catalog:      catalog.New(),
		serverAddr:   serverAddr,
		now:          time.Now,
	}
}

func (s *Server) AllocationLogs(ctx context.Context, id string, follow bool, tail int) (io.ReadCloser, error) {
	return s.AllocationLogsForNamespace(ctx, "", id, follow, tail)
}

func (s *Server) AllocationLogsForNamespace(ctx context.Context, namespace, id string, follow bool, tail int) (io.ReadCloser, error) {
	s.mu.RLock()
	var found *Allocation
	for _, alloc := range s.allocations {
		if alloc.ID == id && alloc.Namespace == namespace {
			found = alloc
			break
		}
	}
	if found == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("allocation not found")
	}
	found.mu.Lock()
	if found.Node == nil {
		found.mu.Unlock()
		s.mu.RUnlock()
		return nil, fmt.Errorf("allocation not found")
	}
	address := fmt.Sprintf("%s:%d", found.Node.Host, found.Node.Port)
	found.mu.Unlock()
	s.mu.RUnlock()
	return s.client.Logs(ctx, address, id, follow, tail)
}

// AcquireLeadership durably advances the fencing epoch. The Raft-backed write
// can only succeed on the current leader, so no separate lock service is
// required.
func (s *Server) AcquireLeadership(ctx context.Context) error {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	s.mu.Lock()
	s.cluster.ControlEpoch++
	epoch := s.cluster.ControlEpoch
	s.mu.Unlock()
	if err := s.state.PutCluster(ctx, s.cluster); err != nil {
		return fmt.Errorf("persist control-plane epoch: %w", err)
	}
	s.mu.Lock()
	s.controlEpoch = epoch
	s.leaderSince = s.now()
	s.mu.Unlock()
	return nil
}

func (s *Server) SetNetworkPool(pool string) error {
	p, err := netip.ParsePrefix(pool)
	if err != nil || !p.Addr().Is4() || p.Bits() > 16 {
		return fmt.Errorf("WireGuard pool must be an IPv4 prefix of /16 or larger")
	}
	s.mu.Lock()
	s.networkPool = p.Masked()
	s.mu.Unlock()
	return nil
}

func (s *Server) Init(ctx context.Context) (string, error) {
	return s.InitWithToken(ctx, "")
}

func (s *Server) InitWithToken(ctx context.Context, configuredToken string) (string, error) {
	cluster, err := s.state.GetCluster(ctx)
	if err != nil {
		return "", fmt.Errorf("get cluster: %w", err)
	}

	if cluster != nil {
		s.log.Info("cluster already initialized")
		token := configuredToken
		if token == "" {
			if err := s.storage.Get("token", &token); err != nil && !os.IsNotExist(unwrapPathError(err)) {
				return "", fmt.Errorf("load local cluster token: %w", err)
			}
		}
		if token == "" || !validateToken(cluster, token) {
			return "", fmt.Errorf("cluster token is missing or does not match cluster")
		}
		s.mu.Lock()
		s.cluster = cluster
		s.client = client.NewAgentClient(token, s.clientTLS)
		s.controlEpoch = cluster.ControlEpoch
		s.mu.Unlock()
		return "", nil
	}

	token := configuredToken
	if token == "" {
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return "", fmt.Errorf("generate cluster token: %w", err)
		}
		token = base64.RawURLEncoding.EncodeToString(b)
	}

	hash := sha256.Sum256([]byte(token))
	if err := s.storage.Put("token", token); err != nil {
		return "", fmt.Errorf("save cluster locally: %w", err)
	}
	cluster = &Cluster{Hash: hex.EncodeToString(hash[:])}
	if err := s.state.PutCluster(ctx, cluster); err != nil {
		return "", fmt.Errorf("save cluster remotely: %w", err)
	}

	s.mu.Lock()
	s.cluster = cluster
	s.controlEpoch = cluster.ControlEpoch
	s.client = client.NewAgentClient(token, s.clientTLS)
	s.mu.Unlock()
	return token, nil
}

func (s *Server) SetClientTLS(cfg *tls.Config) {
	s.mu.Lock()
	s.clientTLS = cfg
	s.mu.Unlock()
}

func (s *Server) ClusterCA() (certPEM, keyPEM string, err error) {
	if err := s.storage.Get("tls/ca-cert", &certPEM); err != nil {
		return "", "", fmt.Errorf("load CA cert: %w", err)
	}
	if err := s.storage.Get("tls/ca-key", &keyPEM); err != nil {
		return "", "", fmt.Errorf("load CA key: %w", err)
	}
	return certPEM, keyPEM, nil
}

func validateToken(cluster *Cluster, token string) bool {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(hashHex), []byte(cluster.Hash)) == 1
}

func (s *Server) Run(ctx context.Context) { go s.runReconcileLoop(ctx) }

func (s *Server) ValidateAPIToken(token string) bool {
	s.mu.RLock()
	cluster := s.cluster
	s.mu.RUnlock()
	if cluster == nil {
		return false
	}
	return validateToken(cluster, token)
}

func unwrapPathError(err error) error {
	for {
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return err
		}
		err = u.Unwrap()
	}
}

func (s *Server) TokenManager() *auth.TokenManager { return s.tokenManager }
func (s *Server) ServerAddr() string                { return s.serverAddr }
func (s *Server) SetClusterJoiner(j ClusterJoiner)  { s.joiner = j }

func (s *Server) runReconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Reconcile(ctx)
		}
	}
}
