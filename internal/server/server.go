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
	"os"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/spec"
	"github.com/clofour/trellis/internal/storage"

	"github.com/google/uuid"
)

const reconcileInterval = 10 * time.Second
const heartbeatInterval = 10 * time.Second

type Server struct {
	log     *slog.Logger
	storage *storage.LocalStorage
	state   *StateController
	client  *client.AgentClient

	cluster     *Cluster
	nodes       map[uuid.UUID]*Node
	jobs        map[string]*Job
	allocations []*Allocation
	mu          sync.RWMutex
}

type Cluster struct {
	Hash string
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

type Node struct {
	ID            uuid.UUID
	Host          string
	Port          int
	Status        NodeStatus
	LastHeartbeat time.Time
	CPU           int
	Memory        int64
}

type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "healthy"
	NodeStatusUnhealthy NodeStatus = "unhealthy"
	NodeStatusDraining  NodeStatus = "draining"
)

type NodeSummary struct {
	ID   uuid.UUID
	Host string
	Port int
}

type Job struct {
	Spec     *spec.JobSpec
	Revision int
}

type AllocationStatus string

const (
	AllocationStatusPending   AllocationStatus = "pending"
	AllocationStatusHealthy   AllocationStatus = "healthy"
	AllocationStatusUnhealthy AllocationStatus = "unhealthy"
)

type Allocation struct {
	JobName       string
	TaskGroupName string
	Name          string
	Task          *spec.TaskSpec
	Status        AllocationStatus
	Node          *Node
	Revision      int
}

func NewServer(log *slog.Logger, storage *storage.LocalStorage, state *StateController) *Server {
	return &Server{
		log:     log.With("component", "server"),
		storage: storage,
		state:   state,
		client:  &client.AgentClient{},
		nodes:   make(map[uuid.UUID]*Node),
		jobs:    make(map[string]*Job),
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
		var token string
		if err := s.storage.Get("token", &token); err != nil && !os.IsNotExist(unwrapPathError(err)) {
			return "", fmt.Errorf("load local cluster token: %w", err)
		}
		s.client = client.NewAgentClient(token)
		return "", nil
	}

	b := make([]byte, 32)
	rand.Read(b)

	token := base64.RawURLEncoding.EncodeToString(b)

	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	err = s.storage.Put("token", token)
	if err != nil {
		return "", fmt.Errorf("save cluster locally: %w", err)
	}

	cluster = &Cluster{
		Hash: hashHex,
	}

	err = s.state.PutCluster(ctx, cluster)
	if err != nil {
		return "", fmt.Errorf("save cluster remotely: %w", err)
	}

	s.cluster = cluster
	s.client = client.NewAgentClient(token)

	return token, nil
}

func (s *Server) Run(ctx context.Context) {
	go s.runReconcileLoop(ctx)
}

func (s *Server) ListNodes(ctx context.Context) []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Node, 0, len(s.nodes))

	for _, node := range s.nodes {
		result = append(result, *node)
	}

	return result
}

func (s *Server) RegisterNode(ctx context.Context, nodeRegistration *NodeRegistration) error {
	err := s.state.PutNode(ctx, nodeRegistration.ID.String(), &NodeSummary{
		ID:   nodeRegistration.ID,
		Host: nodeRegistration.Host,
		Port: nodeRegistration.Port,
	})
	if err != nil {
		return fmt.Errorf("save node remotely: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[nodeRegistration.ID] = &Node{
		ID:            nodeRegistration.ID,
		Host:          nodeRegistration.Host,
		Port:          nodeRegistration.Port,
		Status:        NodeStatusHealthy,
		LastHeartbeat: time.Now(),
		CPU:           nodeRegistration.CPU, Memory: nodeRegistration.Memory,
	}

	return nil
}

func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found")
	}

	node.Status = NodeStatusHealthy
	node.LastHeartbeat = time.Now()
	statuses := make(map[string]AllocationStatus, len(actual))
	for _, a := range actual {
		statuses[a.ID] = AllocationStatus(a.Status)
	}
	for _, a := range s.allocations {
		if a.Node == node {
			if status, ok := statuses[a.Name]; ok {
				a.Status = status
			}
		}
	}

	return nil
}

func (s *Server) RegisterJob(ctx context.Context, jobSpec *spec.JobSpec) error {
	if err := spec.Validate(jobSpec); err != nil {
		return fmt.Errorf("validate job: %w", err)
	}
	err := s.state.PutJob(ctx, jobSpec.Name, jobSpec)
	if err != nil {
		return fmt.Errorf("save job remotely: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	revision := 1
	if existing := s.jobs[jobSpec.Name]; existing != nil {
		revision = existing.Revision + 1
	}
	s.jobs[jobSpec.Name] = &Job{
		Spec:     jobSpec,
		Revision: revision,
	}

	return nil
}

func (s *Server) GetJob(name string) (*api.JobStatusResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[name]
	if !ok {
		return nil, false
	}
	r := &api.JobStatusResponse{Name: name, Revision: job.Revision}
	for _, g := range job.Spec.TaskGroups {
		r.Desired += g.Count * len(g.Tasks)
	}
	for _, a := range s.allocations {
		if a.JobName != name {
			continue
		}
		ar := api.AllocationResponse{ID: a.Name, Group: a.TaskGroupName, Status: string(a.Status)}
		if a.Task != nil {
			ar.Task = a.Task.Name
		}
		if a.Node != nil {
			ar.NodeID = a.Node.ID
		}
		r.Allocations = append(r.Allocations, ar)
		r.Running++
		if a.Status == AllocationStatusHealthy {
			r.Healthy++
		}
	}
	return r, true
}

func (s *Server) DeleteJob(ctx context.Context, name string) error {
	s.mu.Lock()
	_, ok := s.jobs[name]
	if ok {
		delete(s.jobs, name)
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("job %s not found", name)
	}
	if err := s.state.DeleteJob(ctx, name); err != nil {
		return err
	}
	s.Reconcile(ctx)
	return nil
}

func (s *Server) RunAllocation(ctx context.Context, allocation *Allocation) error {
	return nil
}

func (s *Server) StopAllocation(ctx context.Context, allocation *Allocation) error {
	return nil
}

func (s *Server) ValidateAPIToken(ctx context.Context, token string) bool {
	if s.cluster == nil {
		return false
	}
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])

	return subtle.ConstantTimeCompare([]byte(hashHex), []byte(s.cluster.Hash)) == 1
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
