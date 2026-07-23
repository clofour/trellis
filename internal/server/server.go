package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/clofour/trellis/internal/models"
	"github.com/clofour/trellis/internal/spec"
	"github.com/clofour/trellis/internal/state"
	"github.com/clofour/trellis/internal/storage"

	"github.com/google/uuid"
)

const reconcileInterval = 10 * time.Second

type Server struct {
	log     *slog.Logger
	storage *storage.LocalStorage
	state   *state.StateController

	cluster     *models.Cluster
	nodes       map[uuid.UUID]*models.Node
	jobs        map[string]*Job
	allocations []*Allocation
}

type NodeRegistration struct {
	ID     uuid.UUID
	Host   string
	Port   int
	CPU    int
	Memory int64
	OS     string
	Arch   string
}

type Job struct {
	Spec       *spec.JobSpec
	TaskGroups map[string]*TaskGroup
	Revision   int
}

type TaskGroup struct {
	Spec        *spec.TaskGroupSpec
	Tasks       []*Task
	Allocations []*Allocation
}

type Task struct {
	Spec *spec.TaskSpec
}

type AllocationStatus string

const (
	StatusPending   AllocationStatus = "pending"
	StatusHealthy   AllocationStatus = "healthy"
	StatusUnhealthy AllocationStatus = "unhealthy"
)

type Allocation struct {
	JobName       string
	TaskGroupName string
	Status        AllocationStatus
	Revision      int
}

func NewServer(log *slog.Logger, storage *storage.LocalStorage, state *state.StateController) *Server {
	return &Server{
		log:     log.With("component", "server"),
		storage: storage,
		state:   state,
		nodes:   make(map[uuid.UUID]*models.Node),
	}
}

func (s *Server) Init(ctx context.Context) (string, error) {
	cluster, err := s.state.GetCluster(ctx)
	if err != nil {
		return "", fmt.Errorf("get cluster: %w", err)
	}

	if cluster != nil {
		s.log.Info("cluster already initialized")

		s.cluster = cluster
		return "", nil
	}

	b := make([]byte, 32)
	rand.Read(b)

	token := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	err = s.storage.Put("token", hashHex)
	if err != nil {
		return "", fmt.Errorf("save cluster locally: %w", err)
	}

	cluster = &models.Cluster{
		Hash: hashHex,
	}

	err = s.state.PutCluster(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("save cluster remotely: %w", err)
	}

	s.cluster = cluster

	return token, nil
}

func (s *Server) Run(ctx context.Context) {
	go s.runReconcileLoop(ctx)
}

func (s *Server) ListNodes(ctx context.Context) []models.Node {
	result := make([]models.Node, 0, len(s.nodes))

	for _, node := range s.nodes {
		result = append(result, *node)
	}

	return result
}

func (s *Server) RegisterNode(ctx context.Context, nodeRegistration *NodeRegistration) error {
	err := s.state.PutNode(ctx, nodeRegistration.ID.String(), &models.NodeSummary{
		ID:   nodeRegistration.ID,
		Host: nodeRegistration.Host,
		Port: nodeRegistration.Port,
	})
	if err != nil {
		return fmt.Errorf("save node remotely: %w", err)
	}

	s.nodes[nodeRegistration.ID] = &models.Node{
		ID:            nodeRegistration.ID,
		Host:          nodeRegistration.Host,
		Port:          nodeRegistration.Port,
		Status:        models.StatusHealthy,
		LastHeartbeat: time.Now(),
	}

	return nil
}

func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID) error {
	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found")
	}

	node.Status = models.StatusHealthy
	node.LastHeartbeat = time.Now()

	return nil
}

func (s *Server) RegisterJob(ctx context.Context, jobSpec *spec.JobSpec) error {
	err := s.state.PutJob(ctx, jobSpec.Name, jobSpec)
	if err != nil {
		return fmt.Errorf("save job remotely: %w", err)
	}

	s.jobs[jobSpec.Name] = &Job{
		Spec:     jobSpec,
		Revision: 0,
	}

	return nil
}

func (s *Server) RunAllocation(ctx context.Context, allocation *Allocation) error {
	return nil
}

func (s *Server) StopAllocation(ctx context.Context, allocation *Allocation) error {
	return nil
}

func (s *Server) ValidateAPIToken(ctx context.Context, token string) bool {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	return subtle.ConstantTimeCompare([]byte(token), []byte(hashHex)) == 1
}

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
