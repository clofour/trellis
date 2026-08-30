package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/auth"
	"github.com/clofour/trellis/internal/catalog"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/spec"
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
	mu           sync.RWMutex
	reconcileMu  sync.Mutex
	mutationMu   sync.Mutex
	controlEpoch uint64
	leaderSince  time.Time
	now          func() time.Time
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
	Hash         string
	ControlEpoch uint64 `json:"control_epoch,omitempty"`
}

type NodeRegistration struct {
	ID                 uuid.UUID
	Host               string
	Port               int
	CPU                int
	Memory             int64
	OS                 string
	Arch               string
	Labels             map[string]string
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
	OS                 string
	Arch               string
	Labels             map[string]string
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
	OS                 string
	Arch               string
	Labels             map[string]string
	Status             NodeStatus
	WireGuardPublicKey string
	WireGuardEndpoint  string
	LastHeartbeat      time.Time
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
	mu            sync.Mutex
	Namespace     string
	JobName       string
	TaskGroupName string
	// ID is the durable scheduler identity. Name is retained for decoding state
	// written before allocation IDs were explicit.
	ID          string `json:"allocation_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Generation  uint64 `json:"generation"`
	JobRevision int    `json:"job_revision"`
	Tasks       []spec.TaskSpec
	// Task is retained only to reload allocations written by older versions.
	Task *spec.TaskSpec `json:",omitempty"`
	// Status is a compatibility projection for old clients and old persisted
	// records. New code makes decisions from Phase and Health.
	Status AllocationStatus `json:"status,omitempty"`
	Phase  lifecycle.Phase  `json:"phase,omitempty"`
	Health lifecycle.Health `json:"health,omitempty"`
	lifecycle.Diagnostic
	Node     *Node
	Revision int               `json:"revision,omitempty"`
	Ports    []api.PortMapping `json:"ports,omitempty"`
}

func (a *Allocation) AllocationID() string {
	if a.ID != "" {
		return a.ID
	}
	return a.Name
}

func (a *Allocation) normalize(now time.Time) {
	if a.ID == "" {
		a.ID = a.Name
	}
	if a.Name == "" {
		a.Name = a.ID
	}
	if a.Generation == 0 {
		a.Generation = 1
	}
	if a.JobRevision == 0 {
		a.JobRevision = a.Revision
	}
	if a.Revision == 0 {
		a.Revision = a.JobRevision
	}
	if !a.Phase.Valid() {
		a.Phase, a.Health = lifecycle.Legacy(string(a.Status))
	}
	if !a.Health.Valid() {
		a.Health = lifecycle.HealthUnknown
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.TransitionedAt.IsZero() {
		a.TransitionedAt = a.CreatedAt
	}
	a.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))
}

func (a *Allocation) Transition(to lifecycle.Phase, now time.Time, reason, message string) error {
	a.normalize(now)
	if err := lifecycle.Transition(a.Phase, to); err != nil {
		return err
	}
	if a.Phase != to {
		a.Phase = to
		a.TransitionedAt = now
	}
	a.Reason, a.Message = reason, message
	a.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, a.Health))
	return nil
}

func (a *Allocation) SetHealth(health lifecycle.Health) error {
	if !health.Valid() {
		return fmt.Errorf("invalid allocation health %q", health)
	}
	a.Health = health
	a.Status = AllocationStatus(lifecycle.CompatibilityStatus(a.Phase, health))
	return nil
}

func (a *Allocation) UnmarshalJSON(data []byte) error {
	type plain Allocation
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	// Assign fields individually to avoid copying the sync.Mutex.
	a.Namespace = decoded.Namespace
	a.JobName = decoded.JobName
	a.TaskGroupName = decoded.TaskGroupName
	a.ID = decoded.ID
	a.Name = decoded.Name
	a.Generation = decoded.Generation
	a.JobRevision = decoded.JobRevision
	a.Tasks = decoded.Tasks
	a.Task = decoded.Task
	a.Status = decoded.Status
	a.Phase = decoded.Phase
	a.Health = decoded.Health
	a.Diagnostic = decoded.Diagnostic
	a.Node = decoded.Node
	a.Revision = decoded.Revision
	a.Ports = decoded.Ports
	a.normalize(time.Now().UTC())
	return nil
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
		s.client = client.NewAgentClient(token, s.clientTLS)
		s.controlEpoch = cluster.ControlEpoch
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
	s.controlEpoch = cluster.ControlEpoch
	s.client = client.NewAgentClient(token, s.clientTLS)

	return token, nil
}

func (s *Server) SetClientTLS(cfg *tls.Config) {
	s.clientTLS = cfg
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
		CPU:  nodeRegistration.CPU, Memory: nodeRegistration.Memory,
		OS: nodeRegistration.OS, Arch: nodeRegistration.Arch, Labels: nodeRegistration.Labels, Status: status,
		WireGuardPublicKey: nodeRegistration.WireGuardPublicKey, WireGuardEndpoint: nodeRegistration.WireGuardEndpoint,
		LastHeartbeat: s.now().UTC(),
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
	node.OS, node.Arch = nodeRegistration.OS, nodeRegistration.Arch
	node.Labels = nodeRegistration.Labels
	node.WireGuardPublicKey, node.WireGuardEndpoint = nodeRegistration.WireGuardPublicKey, nodeRegistration.WireGuardEndpoint

	return nil
}

func (s *Server) Heartbeat(ctx context.Context, nodeID uuid.UUID, actual []api.AllocationStatus) error {
	s.mu.Lock()
	node, ok := s.nodes[nodeID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("node not found")
	}

	if node.Status != NodeStatusDraining {
		node.Status = NodeStatusHealthy
	}
	node.LastHeartbeat = time.Now()
	owned := make([]*Allocation, 0)
	for _, allocation := range s.allocations {
		if allocation.Node == node {
			owned = append(owned, allocation)
		}
	}
	summary := &NodeSummary{ID: node.ID, Host: node.Host, Port: node.Port, CPU: node.CPU, Memory: node.Memory, OS: node.OS, Arch: node.Arch, Labels: node.Labels, Status: node.Status, WireGuardPublicKey: node.WireGuardPublicKey, WireGuardEndpoint: node.WireGuardEndpoint, LastHeartbeat: node.LastHeartbeat}
	s.mu.Unlock()
	if err := s.state.PutNode(ctx, node.ID.String(), summary); err != nil {
		return fmt.Errorf("persist node heartbeat: %w", err)
	}
	type statusInfo struct {
		Generation uint64
		Phase      lifecycle.Phase
		Health     lifecycle.Health
		Ports      []api.PortMapping
		Tasks      int
	}
	statuses := make(map[string]statusInfo, len(actual))
	for _, a := range actual {
		phase, health := a.Phase, a.Health
		if !phase.Valid() || !health.Valid() {
			phase, health = lifecycle.Legacy(a.Status)
		}
		info := statuses[a.ID]
		if info.Tasks == 0 {
			info.Generation, info.Phase, info.Health = a.Generation, phase, health
		} else {
			if phase != lifecycle.PhaseRunning {
				info.Phase = phase
			}
			if info.Health == lifecycle.HealthUnhealthy || health == lifecycle.HealthUnhealthy {
				info.Health = lifecycle.HealthUnhealthy
			} else if info.Health == lifecycle.HealthUnknown || health == lifecycle.HealthUnknown {
				info.Health = lifecycle.HealthUnknown
			} else {
				info.Health = lifecycle.HealthHealthy
			}
		}
		info.Tasks++
		info.Ports = append(info.Ports, a.Ports...)
		statuses[a.ID] = info
	}
	var changed []*Allocation
	for _, a := range owned {
		a.mu.Lock()
		a.normalize(time.Now().UTC())
		info, ok := statuses[a.AllocationID()]
		if !ok || (info.Generation != 0 && info.Generation != a.Generation) {
			a.mu.Unlock()
			continue // absence is not proof of loss or failure
		}
		if info.Phase.Valid() && lifecycle.CanTransition(a.Phase, info.Phase) {
			_ = a.Transition(info.Phase, time.Now().UTC(), "", "")
		}
		_ = a.SetHealth(info.Health)
		if len(info.Ports) > 0 {
			a.Ports = info.Ports
		}
		changed = append(changed, a)
		a.mu.Unlock()
	}
	for _, allocation := range changed {
		allocation.mu.Lock()
		if err := s.state.PutAllocation(ctx, allocation); err != nil {
			allocation.mu.Unlock()
			return fmt.Errorf("persist allocation observation: %w", err)
		}
		allocation.mu.Unlock()
	}

	s.refreshCatalog()
	return nil
}

func (s *Server) HeartbeatResponse(nodeID uuid.UUID) api.HeartbeatResponse {
	s.mu.RLock()
	epoch, leaderSince := s.controlEpoch, s.leaderSince
	allocations := append([]*Allocation(nil), s.allocations...)
	s.mu.RUnlock()
	response := api.HeartbeatResponse{Epoch: epoch, OrphanConfirmation: !leaderSince.IsZero() && s.now().Sub(leaderSince) >= leaderRecoveryGrace}
	for _, allocation := range allocations {
		allocation.mu.Lock()
		if allocation.Node != nil && allocation.Node.ID == nodeID && allocation.Phase != lifecycle.PhaseStopped && allocation.Phase != lifecycle.PhaseFailed && allocation.Phase != lifecycle.PhaseLost {
			response.Desired = append(response.Desired, api.DesiredAllocation{ID: allocation.AllocationID(), Generation: allocation.Generation})
		}
		allocation.mu.Unlock()
	}
	return response
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
		nodes[summary.ID] = &Node{ID: summary.ID, Host: summary.Host, Port: summary.Port, CPU: summary.CPU, Memory: summary.Memory, OS: summary.OS, Arch: summary.Arch, Labels: summary.Labels, Status: status, WireGuardPublicKey: summary.WireGuardPublicKey, WireGuardEndpoint: summary.WireGuardEndpoint, LastHeartbeat: summary.LastHeartbeat}
	}
	allocations := make([]*Allocation, 0, len(allocationMap))
	for _, allocation := range allocationMap {
		allocation.normalize(time.Now().UTC())
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
	summary := &NodeSummary{ID: node.ID, Host: node.Host, Port: node.Port, CPU: node.CPU, Memory: node.Memory, OS: node.OS, Arch: node.Arch, Labels: node.Labels, Status: node.Status, WireGuardPublicKey: node.WireGuardPublicKey, WireGuardEndpoint: node.WireGuardEndpoint, LastHeartbeat: node.LastHeartbeat}
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
			a.normalize(time.Now().UTC())
			if a.Phase == lifecycle.PhaseRunning {
				r.Running++
			}
			if a.Health == lifecycle.HealthHealthy {
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
	specCopy := *job.Spec
	r := &api.JobStatusResponse{Name: name, Revision: job.Revision, Spec: &specCopy}
	for _, g := range job.Spec.TaskGroups {
		r.Desired += g.Count
	}
	for _, a := range s.allocations {
		if a.Namespace != namespace || a.JobName != name {
			continue
		}
		a.normalize(time.Now().UTC())
		ar := api.AllocationResponse{ID: a.AllocationID(), Group: a.TaskGroupName, Status: string(a.Status), Phase: a.Phase, Health: a.Health, Generation: a.Generation, JobRevision: a.JobRevision, CreatedAt: a.CreatedAt, LastTransitionAt: a.TransitionedAt, Reason: a.Reason, Message: a.Message, Attempt: a.Attempt, NextRetryAt: a.NextRetryAt}
		if a.Node != nil {
			ar.NodeID = a.Node.ID
		}
		r.Allocations = append(r.Allocations, ar)
		if a.Phase == lifecycle.PhaseRunning {
			r.Running++
		}
		if a.Health == lifecycle.HealthHealthy {
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

func (s *Server) ListServices(namespace string, filter *catalog.ListFilter) api.ServiceListResponse {
	return s.catalog.List(namespace, filter)
}

func (s *Server) Catalog() *catalog.ServiceCatalog {
	return s.catalog
}

func (s *Server) refreshCatalog() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	namespaced := make(map[string][]catalog.ServiceInstance)
	for _, a := range s.allocations {
		a.normalize(time.Now().UTC())
		if a.Phase != lifecycle.PhaseRunning || a.Health != lifecycle.HealthHealthy {
			continue
		}
		var labels map[string]string
		job := s.jobs[jobKey(a.Namespace, a.JobName)]
		if job != nil {
			for _, g := range job.Spec.TaskGroups {
				if g.Name == a.TaskGroupName {
					labels = g.Labels
					break
				}
			}
		}
		var address string
		if a.Node != nil {
			address = a.Node.Host
		}
		namespaced[a.Namespace] = append(namespaced[a.Namespace], catalog.ServiceInstance{
			ID:      a.Name,
			Job:     a.JobName,
			Group:   a.TaskGroupName,
			Address: address,
			Ports:   a.Ports,
			Labels:  labels,
		})
	}

	seen := make(map[string]bool)
	for ns, instances := range namespaced {
		s.catalog.Update(ns, instances)
		seen[ns] = true
	}
	for _, a := range s.allocations {
		if !seen[a.Namespace] {
			s.catalog.Update(a.Namespace, nil)
		}
	}
}

func (s *Server) TokenManager() *auth.TokenManager {
	return s.tokenManager
}

func (s *Server) ServerAddr() string {
	return s.serverAddr
}

func (s *Server) SetClusterJoiner(j ClusterJoiner) {
	s.joiner = j
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
