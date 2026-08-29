package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
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
	networkPool netip.Prefix
	mu          sync.RWMutex
	reconcileMu sync.Mutex
	mutationMu  sync.Mutex
}

func (s *Server) AllocationLogs(ctx context.Context, id string, follow bool, tail int) (io.ReadCloser, error) {
	return s.AllocationLogsForNamespace(ctx, "", id, follow, tail)
}

func (s *Server) AllocationLogsForNamespace(ctx context.Context, namespace, id string, follow bool, tail int) (io.ReadCloser, error) {
	s.mu.RLock()
	var found *Allocation
	for _, alloc := range s.allocations {
		if alloc.Name == id && alloc.Namespace == namespace {
			found = alloc
			break
		}
	}
	if found == nil || found.Node == nil {
		s.mu.RUnlock()
		return nil, fmt.Errorf("allocation not found")
	}
	address := fmt.Sprintf("%s:%d", found.Node.Host, found.Node.Port)
	s.mu.RUnlock()
	return s.client.Logs(ctx, address, id, follow, tail)
}

type Cluster struct {
	Hash string
}

type NodeRegistration struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	WireGuardPublicKey string
	WireGuardEndpoint  string
}

type Node struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	Status             NodeStatus
	LastHeartbeat      time.Time
	CPU                int
	Memory             int64
	WireGuardPublicKey string
	WireGuardEndpoint  string
}

type NodeStatus string

const (
	NodeStatusHealthy   NodeStatus = "healthy"
	NodeStatusUnhealthy NodeStatus = "unhealthy"
	NodeStatusDraining  NodeStatus = "draining"
)

type NodeSummary struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	Status             NodeStatus
	WireGuardPublicKey string
	WireGuardEndpoint  string
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
	Namespace     string
	JobName       string
	TaskGroupName string
	Name          string
	Tasks         []spec.TaskSpec
	// Task is retained only to reload allocations written by older versions.
	Task     *spec.TaskSpec `json:",omitempty"`
	Status   AllocationStatus
	Node     *Node
	Revision int
}

func NewServer(log *slog.Logger, storage *storage.LocalStorage, state *StateController) *Server {
	pool := netip.MustParsePrefix("10.64.0.0/10")
	return &Server{
		log:         log.With("component", "server"),
		storage:     storage,
		state:       state,
		client:      &client.AgentClient{},
		nodes:       make(map[uuid.UUID]*Node),
		jobs:        make(map[string]*Job),
		networkPool: pool,
	}
}

func (s *Server) SetNetworkPool(pool string) error {
	p, err := netip.ParsePrefix(pool)
	if err != nil || !p.Addr().Is4() || p.Bits() > 16 {
		return fmt.Errorf("WireGuard pool must be an IPv4 prefix of /16 or larger")
	}
	s.networkPool = p.Masked()
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

		s.cluster = cluster
		token := configuredToken
		if token == "" {
			if err := s.storage.Get("token", &token); err != nil && !os.IsNotExist(unwrapPathError(err)) {
				return "", fmt.Errorf("load local cluster token: %w", err)
			}
		}
		if token == "" || !validateToken(cluster, token) {
			return "", fmt.Errorf("cluster token is missing or does not match cluster")
		}
		s.client = client.NewAgentClient(token)
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

func validateToken(cluster *Cluster, token string) bool {
	hash := sha256.Sum256([]byte(token))
	hashHex := hex.EncodeToString(hash[:])
	return subtle.ConstantTimeCompare([]byte(hashHex), []byte(cluster.Hash)) == 1
}

func (s *Server) Run(ctx context.Context) {
	go s.runReconcileLoop(ctx)
}

func (s *Server) ListNodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Node, 0, len(s.nodes))

	for _, node := range s.nodes {
		result = append(result, *node)
	}

	return result
}

func (s *Server) RegisterNode(ctx context.Context, nodeRegistration *NodeRegistration) error {
	s.mu.RLock()
	status := NodeStatusHealthy
	if existing := s.nodes[nodeRegistration.ID]; existing != nil && existing.Status == NodeStatusDraining {
		status = NodeStatusDraining
	}
	s.mu.RUnlock()
	err := s.state.PutNode(ctx, nodeRegistration.ID.String(), &NodeSummary{
		ID:   nodeRegistration.ID,
		Host: nodeRegistration.Host,
		Port: nodeRegistration.Port,
		CPU:  nodeRegistration.CPU, Memory: nodeRegistration.Memory, Status: status,
		WireGuardPublicKey: nodeRegistration.WireGuardPublicKey, WireGuardEndpoint: nodeRegistration.WireGuardEndpoint,
	})
	if err != nil {
		return fmt.Errorf("save node remotely: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	node := s.nodes[nodeRegistration.ID]
	if node == nil {
		node = &Node{ID: nodeRegistration.ID}
		s.nodes[nodeRegistration.ID] = node
	}
	node.Host, node.Port = nodeRegistration.Host, nodeRegistration.Port
	if node.Status != NodeStatusDraining {
		node.Status = NodeStatusHealthy
	}
	node.LastHeartbeat = time.Now()
	node.CPU, node.Memory = nodeRegistration.CPU, nodeRegistration.Memory
	node.WireGuardPublicKey, node.WireGuardEndpoint = nodeRegistration.WireGuardPublicKey, nodeRegistration.WireGuardEndpoint

	return nil
}

func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found")
	}

	if node.Status != NodeStatusDraining {
		node.Status = NodeStatusHealthy
	}
	node.LastHeartbeat = time.Now()
	statuses := make(map[string]AllocationStatus, len(actual))
	for _, a := range actual {
		statuses[a.ID] = AllocationStatus(a.Status)
	}
	for _, a := range s.allocations {
		if a.Node == node {
			if status, ok := statuses[a.Name]; ok {
				a.Status = status
			} else {
				a.Status = AllocationStatusUnhealthy
			}
		}
	}

	return nil
}

func jobKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return namespace + "\x00" + name
}

func requestedResources(jobSpec *spec.JobSpec) (cpu, memory int) {
	for _, group := range jobSpec.TaskGroups {
		for _, task := range group.Tasks {
			if task.Resources != nil {
				cpu += task.Resources.CPU * group.Count
				memory += task.Resources.Memory * group.Count
			}
		}
	}
	return
}

func (s *Server) RegisterJob(ctx context.Context, namespace string, jobSpec *spec.JobSpec) error {
	if err := spec.Validate(jobSpec); err != nil {
		return fmt.Errorf("validate job: %w", err)
	}
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	s.mu.Lock()
	revision := 1
	if jobSpec.Namespace != namespace {
		s.mu.Unlock()
		return fmt.Errorf("job namespace does not match request namespace")
	}
	key := jobKey(namespace, jobSpec.Name)
	if existing := s.jobs[key]; existing != nil {
		revision = existing.Revision + 1
	}
	job := &Job{
		Spec:     jobSpec,
		Revision: revision,
	}
	s.mu.Unlock()
	if err := s.state.PutJob(ctx, key, job); err != nil {
		return fmt.Errorf("save job remotely: %w", err)
	}
	s.mu.Lock()
	s.jobs[key] = job
	s.mu.Unlock()

	return nil
}

// Reload reconstructs the durable control-plane state before a leadership term starts.
func (s *Server) Reload(ctx context.Context) error {
	jobs, err := s.state.ListJobs(ctx)
	if err != nil {
		return fmt.Errorf("load jobs: %w", err)
	}
	nodeSummaries, err := s.state.ListNodes(ctx)
	if err != nil {
		return fmt.Errorf("load nodes: %w", err)
	}
	allocationMap, err := s.state.ListAllocations(ctx)
	if err != nil {
		return fmt.Errorf("load allocations: %w", err)
	}
	nodes := make(map[uuid.UUID]*Node, len(nodeSummaries))
	for _, summary := range nodeSummaries {
		status := NodeStatusUnhealthy
		if summary.Status == NodeStatusDraining {
			status = NodeStatusDraining
		}
		nodes[summary.ID] = &Node{ID: summary.ID, Host: summary.Host, Port: summary.Port, CPU: summary.CPU, Memory: summary.Memory, Status: status, WireGuardPublicKey: summary.WireGuardPublicKey, WireGuardEndpoint: summary.WireGuardEndpoint}
	}
	allocations := make([]*Allocation, 0, len(allocationMap))
	for _, allocation := range allocationMap {
		if allocation.Node != nil {
			allocation.Node = nodes[allocation.Node.ID]
		}
		allocations = append(allocations, allocation)
	}
	s.mu.Lock()
	s.jobs = jobs
	s.nodes = nodes
	s.allocations = allocations
	s.mu.Unlock()
	return nil
}

func (s *Server) DrainNode(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	node := s.nodes[id]
	if node == nil {
		s.mu.Unlock()
		return fmt.Errorf("node not found")
	}
	previousStatus := node.Status
	node.Status = NodeStatusDraining
	summary := &NodeSummary{ID: node.ID, Host: node.Host, Port: node.Port, CPU: node.CPU, Memory: node.Memory, Status: node.Status}
	s.mu.Unlock()
	if err := s.state.PutNode(ctx, id.String(), summary); err != nil {
		s.mu.Lock()
		if current := s.nodes[id]; current == node && current.Status == NodeStatusDraining {
			current.Status = previousStatus
		}
		s.mu.Unlock()
		return err
	}
	s.Reconcile(ctx)
	return nil
}

func (s *Server) ListJobs(namespace string) api.JobListResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(api.JobListResponse, 0, len(s.jobs))
	for key, job := range s.jobs {
		if job.Spec.Namespace != namespace {
			continue
		}
		name := job.Spec.Name
		r := api.JobStatusResponse{Name: name, Revision: job.Revision}
		for _, g := range job.Spec.TaskGroups {
			r.Desired += g.Count
		}
		for _, a := range s.allocations {
			if jobKey(a.Namespace, a.JobName) != key {
				continue
			}
			r.Running++
			if a.Status == AllocationStatusHealthy {
				r.Healthy++
			}
		}
		result = append(result, r)
	}
	return result
}

func (s *Server) GetJob(namespace, name string) (*api.JobStatusResponse, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[jobKey(namespace, name)]
	if !ok {
		return nil, false
	}
	r := &api.JobStatusResponse{Name: name, Revision: job.Revision}
	for _, g := range job.Spec.TaskGroups {
		r.Desired += g.Count
	}
	for _, a := range s.allocations {
		if a.Namespace != namespace || a.JobName != name {
			continue
		}
		ar := api.AllocationResponse{ID: a.Name, Group: a.TaskGroupName, Status: string(a.Status)}
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

func (s *Server) DeleteJob(ctx context.Context, namespace, name string) error {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	s.mu.RLock()
	key := jobKey(namespace, name)
	_, ok := s.jobs[key]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("job %s not found", name)
	}
	if err := s.state.DeleteJob(ctx, key); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.jobs, key)
	s.mu.Unlock()
	s.Reconcile(ctx)
	return nil
}

func (s *Server) ValidateAPIToken(token string) bool {
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
