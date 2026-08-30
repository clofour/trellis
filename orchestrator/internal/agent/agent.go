package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/clofour/trellis/internal/api"
	"github.com/clofour/trellis/internal/client"
	"github.com/clofour/trellis/internal/health"
	"github.com/clofour/trellis/internal/lifecycle"
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/runtime"
	"github.com/clofour/trellis/internal/spec"
	"github.com/clofour/trellis/internal/storage"
	"github.com/google/uuid"
)

type Agent struct {
	nodeID      uuid.UUID
	allocations map[string]*Allocation

	log *slog.Logger

	runtime    runtime.ContainerRuntime
	health     *health.HealthManager
	reconciler *AllocationReconciler
	ports      *PortManager
	volumes    *VolumeManager
	network    network.Manager
	server     *client.ServerClient
	nodeInfo   client.NodeInfo
	dnsServers []string
	local      *storage.LocalStorage
	cluster    string
	epoch      uint64
	orphans    map[string]int
	mu         sync.RWMutex
}

type Allocation struct {
	ID              string
	AllocationID    string
	Generation      uint64
	JobRevision     int
	ExecutionHash   string
	Restart         *spec.RestartPolicySpec
	RestartAttempts int
	RestartWindow   time.Time
	Namespace       string

	JobName   string
	GroupName string
	TaskName  string
	Spec      *spec.TaskSpec

	ContainerID string
	Ports       []*runtime.Port
	Mounts      []*runtime.Mount
	SecretDir   string
	Network     *network.Attachment
	Status      string
	Health      string
}

const heartbeatInterval = 10 * time.Second

var (
	ErrAllocationNotFound = errors.New("allocation not found")
	ErrAllocationExists   = errors.New("allocation already exists")
	ErrStaleEpoch         = errors.New("stale control-plane epoch")
	ErrStaleGeneration    = errors.New("stale allocation generation")
	ErrExecutionConflict  = errors.New("allocation execution metadata conflict")
)

func (a *Agent) ConfigureDurability(local *storage.LocalStorage, cluster string) {
	a.local, a.cluster = local, cluster
}

func (a *Agent) AcceptEpoch(epoch uint64) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if epoch < a.epoch {
		return fmt.Errorf("%w: received %d, highest accepted %d", ErrStaleEpoch, epoch, a.epoch)
	}
	if epoch == a.epoch {
		return nil
	}
	if a.local != nil {
		if err := a.local.Put("agent/control-epoch", epoch); err != nil {
			return fmt.Errorf("persist control-plane epoch: %w", err)
		}
	}
	a.epoch = epoch
	return nil
}

func allocationRecordKey(id string) string {
	return "agent/allocations/" + base64.RawURLEncoding.EncodeToString([]byte(id))
}

func (a *Agent) persistAllocation(allocation *Allocation) error {
	if a.local == nil {
		return nil
	}
	return a.local.Put(allocationRecordKey(allocation.ID), allocation)
}

func (a *Agent) deleteAllocationRecord(id string) error {
	if a.local == nil {
		return nil
	}
	return a.local.Delete(allocationRecordKey(id))
}

func NewAgent(log *slog.Logger, runtime runtime.ContainerRuntime, health *health.HealthManager, reconciler *AllocationReconciler, ports *PortManager, volumes *VolumeManager, server *client.ServerClient, nodeID uuid.UUID) *Agent {
	agent := &Agent{
		nodeID:      nodeID,
		allocations: make(map[string]*Allocation),
		orphans:     make(map[string]int),

		log: log,

		runtime:    runtime,
		health:     health,
		reconciler: reconciler,
		ports:      ports,
		volumes:    volumes,
		network:    network.DisabledManager{},
		server:     server,
		nodeInfo:   client.NodeInfo{ID: nodeID, Host: "127.0.0.1", Port: 8127},
	}

	return agent
}

func (a *Agent) SetNetworkManager(manager network.Manager) {
	if manager != nil {
		a.network = manager
	}
}

func (a *Agent) SetWireGuardIdentity(publicKey, endpoint string) {
	a.nodeInfo.WireGuardPublicKey, a.nodeInfo.WireGuardEndpoint = publicKey, endpoint
}

func (a *Agent) SetDNSServers(servers []string) {
	a.dnsServers = servers
}

func (a *Agent) SetAdvertiseAddress(host string, port int) {
	a.nodeInfo.Host = host
	a.nodeInfo.Port = port
}

func (a *Agent) SetResources(cpu int, memory int64, osName, arch string) {
	a.nodeInfo.CPU, a.nodeInfo.Memory, a.nodeInfo.OS, a.nodeInfo.Arch = cpu, memory, osName, arch
}

func (a *Agent) SetLabels(labels map[string]string) {
	a.nodeInfo.Labels = labels
}

func (a *Agent) Init(ctx context.Context) {
	a.health.Subscriber = a
	a.health.SetContext(ctx)
	a.reconciler.Subscriber = a
	if err := a.recover(ctx); err != nil {
		a.log.Error("recover allocations", "error", err)
	}

	go a.runHeartbeatLoop(ctx)
	go a.reconciler.Run(ctx)
}

func (a *Agent) recover(ctx context.Context) error {
	if a.local == nil {
		return nil
	}
	var epoch uint64
	if err := a.local.Get("agent/control-epoch", &epoch); err == nil {
		a.epoch = epoch
	}
	records, recordErrs := a.local.ListRaw("agent/allocations")
	stored := make(map[string]*Allocation, len(records))
	for name, raw := range records {
		var allocation Allocation
		if err := json.Unmarshal(raw, &allocation); err != nil {
			a.log.Error("skip malformed allocation record", "record", name, "error", err)
			continue
		}
		stored[allocation.ContainerID] = &allocation
	}
	for _, err := range recordErrs {
		a.log.Error("read allocation recovery record", "error", err)
	}
	managed, ok := a.runtime.(runtime.ManagedRuntime)
	if !ok {
		return nil
	}
	containers, err := managed.ListManaged(ctx, a.cluster)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(containers))
	for _, container := range containers {
		seen[container.ID] = true
		allocation := stored[container.ID]
		hadRecord := allocation != nil
		if allocation == nil {
			generation, _ := strconv.ParseUint(container.Labels["trellis.allocation-generation"], 10, 64)
			allocation = &Allocation{ID: container.ID, ContainerID: container.ID, AllocationID: container.Labels["trellis.allocation-id"], Generation: generation, Namespace: container.Labels["trellis.namespace"], JobName: container.Labels["trellis.job"], GroupName: container.Labels["trellis.task-group"], TaskName: container.Labels["trellis.task"], Status: "running", Health: "unknown"}
		}
		if allocation.AllocationID == "" || allocation.Generation == 0 {
			a.log.Warn("leave unidentifiable Trellis container untouched", "container", container.ID)
			continue
		}
		if hadRecord && allocation.Status == "starting" && (container.Status == runtime.StatusCreated || container.Status == runtime.StatusStopped) {
			if err := a.runtime.Start(ctx, container.ID); err != nil {
				a.log.Error("resume interrupted allocation start", "container", container.ID, "error", err)
				continue
			}
			allocation.Status = "running"
			container.Status = runtime.StatusRunning
		}
		if container.Status == runtime.StatusRunning || container.Status == runtime.StatusCreated || container.Status == runtime.StatusStopped {
			for _, port := range allocation.Ports {
				if err := a.ports.Adopt(port); err != nil {
					a.log.Error("recover port claim", "allocation", allocation.AllocationID, "error", err)
				}
			}
			a.allocations[allocation.ID] = allocation
			if allocation.Spec != nil {
				if allocation.Spec.HealthCheck != nil {
					check := *allocation.Spec.HealthCheck
					for _, port := range allocation.Ports {
						if port.ContainerPort == check.Port {
							check.Port = port.HostPort
							break
						}
					}
					a.health.RegisterTask(allocation.ID, allocation.ContainerID, &check)
				}
				a.reconciler.TrackRecovered(allocation.ID, allocation.Spec.HealthCheck != nil, allocation.Restart, allocation.RestartAttempts, allocation.RestartWindow)
			} else {
				a.reconciler.Track(allocation.ID, false, nil)
			}
			if err := a.persistAllocation(allocation); err != nil {
				a.log.Error("refresh recovered allocation record", "allocation", allocation.AllocationID, "error", err)
			}
		}
	}
	for containerID, allocation := range stored {
		if containerID == "" || seen[containerID] {
			continue
		}
		for _, port := range allocation.Ports {
			_ = a.ports.Adopt(port)
			_ = a.ports.Release(port)
		}
		_ = a.network.Detach(context.WithoutCancel(ctx), allocation.Network)
		_ = a.deleteAllocationRecord(allocation.ID)
	}
	return nil
}

func (a *Agent) GetAllocations() []*Allocation {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*Allocation, 0, len(a.allocations))
	for _, alloc := range a.allocations {
		copy := *alloc
		copy.Ports = append([]*runtime.Port(nil), alloc.Ports...)
		copy.Mounts = append([]*runtime.Mount(nil), alloc.Mounts...)
		result = append(result, &copy)
	}

	return result
}

func (a *Agent) PrepareStart(ctx context.Context, request *api.AllocationRequest) error {
	if err := a.AcceptEpoch(request.Epoch); err != nil {
		return err
	}
	a.mu.RLock()
	var oldIDs []string
	for id, allocation := range a.allocations {
		if allocation.AllocationID != request.AllocationID {
			continue
		}
		if allocation.Generation > request.Generation {
			a.mu.RUnlock()
			return fmt.Errorf("%w: current %d, requested %d", ErrStaleGeneration, allocation.Generation, request.Generation)
		}
		if allocation.Generation == request.Generation && allocation.ExecutionHash != request.ExecutionHash {
			a.mu.RUnlock()
			return fmt.Errorf("%w: allocation %s generation %d", ErrExecutionConflict, request.AllocationID, request.Generation)
		}
		if allocation.Generation < request.Generation {
			oldIDs = append(oldIDs, id)
		}
	}
	a.mu.RUnlock()
	for _, id := range oldIDs {
		if err := a.StopAllocation(ctx, id); err != nil {
			return fmt.Errorf("replace older generation: %w", err)
		}
	}
	return nil
}

func (a *Agent) StopGroup(ctx context.Context, request *api.StopAllocationRequest) error {
	if err := a.AcceptEpoch(request.Epoch); err != nil {
		return err
	}
	a.mu.RLock()
	var ids []string
	for id, allocation := range a.allocations {
		if allocation.AllocationID != request.AllocationID {
			continue
		}
		if allocation.Generation > request.Generation {
			a.mu.RUnlock()
			return fmt.Errorf("%w: current %d, requested %d", ErrStaleGeneration, allocation.Generation, request.Generation)
		}
		if allocation.Generation == request.Generation {
			ids = append(ids, id)
		}
	}
	a.mu.RUnlock()
	var errs []error
	for _, id := range ids {
		if err := a.StopAllocation(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (a *Agent) reconcileDesired(ctx context.Context, response *api.HeartbeatResponse) {
	if response == nil || !response.OrphanConfirmation {
		return
	}
	if err := a.AcceptEpoch(response.Epoch); err != nil {
		return
	}
	type desiredState struct {
		wanted   bool
		draining bool
	}
	desired := make(map[string]desiredState, len(response.Desired))
	for _, allocation := range response.Desired {
		desired[fmt.Sprintf("%s/%d", allocation.ID, allocation.Generation)] = desiredState{wanted: true, draining: allocation.Draining}
	}
	a.mu.Lock()
	var collect []string
	for id, allocation := range a.allocations {
		key := fmt.Sprintf("%s/%d", allocation.AllocationID, allocation.Generation)
		state := desired[key]
		if state.wanted {
			delete(a.orphans, key)
			if state.draining {
				_ = a.reconciler.Untrack(allocation.ID)
			}
			continue
		}
		a.orphans[key]++
		if a.orphans[key] >= 2 {
			collect = append(collect, id)
		}
	}
	a.mu.Unlock()
	for _, id := range collect {
		if err := a.StopAllocation(context.WithoutCancel(ctx), id); err != nil {
			a.log.Error("collect confirmed orphan", "task", id, "error", err)
		}
	}
}

func (a *Agent) RunAllocation(ctx context.Context, allocID, schedulerID string, generation uint64, jobRevision int, executionHash, namespace, jobName, groupName, taskName string, taskSpec *spec.TaskSpec, groupRuntime string, wireGuard bool, networkPlan *network.Plan, networkMode string, envOverrides map[string]string, delivered []api.DeliveredSecret, restartPolicy *spec.RestartPolicySpec) error {
	spec := taskSpec
	if spec == nil {
		return fmt.Errorf("task spec is required")
	}
	if allocID == "" {
		return fmt.Errorf("allocation ID is required")
	}
	a.mu.Lock()
	existing := a.allocations[allocID]
	if existing != nil {
		a.mu.Unlock()
		if existing.AllocationID == schedulerID && existing.Generation == generation && existing.ExecutionHash == executionHash {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrAllocationExists, allocID)
	}
	alloc := &Allocation{ID: allocID, ContainerID: allocID, AllocationID: schedulerID, Generation: generation, JobRevision: jobRevision, ExecutionHash: executionHash, Namespace: namespace, JobName: jobName, GroupName: groupName, TaskName: taskName, Spec: spec, Status: "starting", Health: "unknown"}
	a.allocations[allocID] = alloc
	a.mu.Unlock()
	if err := a.persistAllocation(alloc); err != nil {
		a.mu.Lock()
		delete(a.allocations, allocID)
		a.mu.Unlock()
		return fmt.Errorf("persist starting allocation: %w", err)
	}
	committed := false
	containerCreated := false
	containerStarted := false
	tracked := false
	healthRegistered := false
	var netAttachment *network.Attachment
	var ports []*runtime.Port
	var secretDir string
	defer func() {
		if committed {
			return
		}
		if healthRegistered {
			a.health.DeregisterTask(allocID)
		}
		if tracked {
			_ = a.reconciler.Untrack(allocID)
		}
		if containerStarted {
			_ = a.runtime.Stop(context.WithoutCancel(ctx), allocID)
		}
		if containerCreated {
			_ = a.runtime.Remove(context.WithoutCancel(ctx), allocID)
		}
		_ = a.network.Detach(context.WithoutCancel(ctx), netAttachment)
		for _, p := range ports {
			_ = a.ports.Release(p)
		}
		if secretDir != "" {
			_ = os.RemoveAll(secretDir)
		}
		a.mu.Lock()
		delete(a.allocations, allocID)
		a.mu.Unlock()
		_ = a.deleteAllocationRecord(allocID)
	}()

	for _, p := range spec.Ports {
		port, err := a.ports.Claim(p)
		if err != nil {
			return fmt.Errorf("claim port %d: %w", p.HostPort, err)
		}

		ports = append(ports, port)
		alloc.Ports = append([]*runtime.Port(nil), ports...)
		if err := a.persistAllocation(alloc); err != nil {
			return fmt.Errorf("persist port claim: %w", err)
		}
	}

	var mounts []*runtime.Mount
	for _, v := range spec.Volumes {
		mount, err := a.volumes.Create(namespace, jobName, taskName, v)
		if err != nil {
			return fmt.Errorf("create volume %s: %w", v.Name, err)
		}

		mounts = append(mounts, mount)
		alloc.Mounts = append([]*runtime.Mount(nil), mounts...)
	}
	if err := a.persistAllocation(alloc); err != nil {
		return fmt.Errorf("persist volume metadata: %w", err)
	}

	err := a.runtime.Pull(ctx, spec.Image)
	if err != nil {
		return fmt.Errorf("pull image %s: %w", spec.Image, err)
	}

	containerID := allocID
	alloc.ContainerID = containerID
	hostMode := networkMode == "host"
	if wireGuard && !hostMode {
		if networkPlan == nil {
			return fmt.Errorf("automatic WireGuard network plan is required")
		}
		netAttachment, err = a.network.Attach(ctx, network.AttachRequest{AllocationID: allocID, Namespace: namespace, Network: namespace, Plan: *networkPlan})
		if err != nil {
			return fmt.Errorf("attach WireGuard network: %w", err)
		}
		alloc.Network = netAttachment
		if err := a.persistAllocation(alloc); err != nil {
			return fmt.Errorf("persist network attachment: %w", err)
		}
	}
	env := make(map[string]string, len(spec.Env)+len(envOverrides))
	for k, v := range spec.Env {
		env[k] = v
	}
	for k, v := range envOverrides {
		env[k] = v
	}
	secretDir, secretEnv, secretMounts, err := prepareSecrets(allocID, taskName, delivered)
	if err != nil {
		return err
	}
	alloc.SecretDir = secretDir
	for k, v := range secretEnv {
		env[k] = v
	}
	mounts = append(mounts, secretMounts...)
	alloc.Mounts = append([]*runtime.Mount(nil), mounts...)
	labels := map[string]string{
		"trellis.cluster":               a.cluster,
		"trellis.allocation-id":         schedulerID,
		"trellis.allocation-generation": strconv.FormatUint(generation, 10),
		"trellis.namespace":             namespace,
		"trellis.job":                   jobName,
		"trellis.task-group":            groupName,
		"trellis.task":                  taskName,
	}
	_, err = a.runtime.Create(ctx, runtime.CreateOptions{
		ID:     containerID,
		Image:  spec.Image,
		Env:    env,
		Mounts: mounts,
		CPU: func() int {
			if spec.Resources != nil {
				return spec.Resources.CPU
			}
			return 0
		}(),
		Memory: func() int64 {
			if spec.Resources != nil {
				return int64(spec.Resources.Memory)
			}
			return 0
		}(),
		Runtime: groupRuntime,
		NetworkNamespace: func() string {
			if netAttachment != nil {
				return netAttachment.NetworkNamespace
			}
			return ""
		}(),
		DNSServers: a.dnsServers,
		Labels:     labels,
	})
	if err != nil {
		observed, inspectErr := a.runtime.Inspect(context.WithoutCancel(ctx), containerID)
		if inspectErr != nil || observed.Labels["trellis.allocation-id"] != schedulerID || observed.Labels["trellis.allocation-generation"] != labels["trellis.allocation-generation"] {
			return fmt.Errorf("create container %s: %w", containerID, err)
		}
	}
	containerCreated = true

	err = a.runtime.Start(ctx, containerID)
	if err != nil {
		observed, inspectErr := a.runtime.Inspect(context.WithoutCancel(ctx), containerID)
		if inspectErr != nil || observed.Status != runtime.StatusRunning {
			return fmt.Errorf("start container %s: %w", containerID, err)
		}
	}
	containerStarted = true

	if spec.HealthCheck != nil {
		check := *spec.HealthCheck
		for _, p := range ports {
			if p.ContainerPort == check.Port {
				check.Port = p.HostPort
				break
			}
		}
		a.health.RegisterTask(allocID, containerID, &check)
		healthRegistered = true
	}

	a.reconciler.Track(allocID, spec.HealthCheck != nil, restartPolicy)
	tracked = true

	ready := &Allocation{
		ID:            allocID,
		AllocationID:  schedulerID,
		Generation:    generation,
		JobRevision:   jobRevision,
		ExecutionHash: executionHash,
		Restart:       restartPolicy,
		Namespace:     namespace,

		JobName:   jobName,
		GroupName: groupName,
		TaskName:  spec.Name,
		Spec:      spec,

		ContainerID: containerID,
		Ports:       ports,
		Mounts:      mounts,
		SecretDir:   secretDir,
		Network:     netAttachment,
		Status:      "running",
		Health:      "unknown",
	}
	if spec.HealthCheck == nil {
		ready.Health = "healthy"
	}
	if err := a.persistAllocation(ready); err != nil {
		return fmt.Errorf("persist allocation: %w", err)
	}
	a.mu.Lock()
	a.allocations[allocID] = ready
	a.mu.Unlock()
	if spec.HealthCheck == nil {
		if err := a.reconciler.ObserveHealth(allocID, true); err != nil {
			return fmt.Errorf("mark allocation healthy: %w", err)
		}
	}
	committed = true

	return nil
}

func (a *Agent) Logs(ctx context.Context, allocID string, follow bool, tail int) (io.ReadCloser, error) {
	a.mu.RLock()
	alloc := a.allocations[allocID]
	a.mu.RUnlock()
	if alloc == nil {
		return nil, fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}
	return a.runtime.Logs(ctx, alloc.ContainerID, follow, tail)
}

func (a *Agent) StopAllocation(ctx context.Context, allocID string) error {
	a.mu.RLock()
	stored, ok := a.allocations[allocID]
	var alloc Allocation
	if ok {
		alloc = *stored
		alloc.Ports = append([]*runtime.Port(nil), stored.Ports...)
	}
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrAllocationNotFound, allocID)
	}

	containerID := alloc.ContainerID

	var errs []error
	// Stop observation and reconciliation before tearing down runtime state so a
	// periodic reconcile cannot race an intentional stop and restart the task.
	a.health.DeregisterTask(allocID)
	if err := a.reconciler.Untrack(allocID); err != nil {
		errs = append(errs, fmt.Errorf("untrack allocation %s: %w", allocID, err))
	}

	if err := a.runtime.Stop(ctx, containerID); err != nil {
		errs = append(errs, fmt.Errorf("stop container %s: %w", containerID, err))
	}
	if err := a.network.Detach(ctx, alloc.Network); err != nil {
		errs = append(errs, fmt.Errorf("detach allocation network: %w", err))
	}
	if err := a.runtime.Remove(ctx, containerID); err != nil {
		errs = append(errs, fmt.Errorf("remove container %s: %w", containerID, err))
	}
	if alloc.SecretDir != "" {
		if err := os.RemoveAll(alloc.SecretDir); err != nil {
			errs = append(errs, fmt.Errorf("remove secret files: %w", err))
		}
	}

	for _, p := range alloc.Ports {
		err := a.ports.Release(p)
		if err != nil {
			errs = append(errs, fmt.Errorf("release port %d: %w", p.HostPort, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if err := a.deleteAllocationRecord(allocID); err != nil {
		return fmt.Errorf("delete allocation record: %w", err)
	}
	a.mu.Lock()
	delete(a.allocations, allocID)
	a.mu.Unlock()

	return nil
}

func prepareSecrets(allocID, taskName string, delivered []api.DeliveredSecret) (string, map[string]string, []*runtime.Mount, error) {
	env := map[string]string{}
	var taskSecrets []api.DeliveredSecret
	for _, secret := range delivered {
		if secret.Task == taskName {
			taskSecrets = append(taskSecrets, secret)
		}
	}
	if len(taskSecrets) == 0 {
		return "", env, nil, nil
	}
	dir, err := os.MkdirTemp("/dev/shm", "trellis-secret-"+filepath.Base(allocID)+"-")
	if err != nil {
		return "", nil, nil, fmt.Errorf("create memory-backed secret directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return "", nil, nil, err
	}
	var mounts []*runtime.Mount
	for i, secret := range taskSecrets {
		switch secret.Target {
		case spec.SecretTargetEnv:
			env[secret.Env] = string(secret.Value)
		case spec.SecretTargetFile:
			hostPath := filepath.Join(dir, fmt.Sprintf("secret-%d", i))
			mode := os.FileMode(secret.Mode)
			if mode == 0 {
				mode = 0o400
			}
			file, err := os.OpenFile(hostPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
			if err != nil {
				_ = os.RemoveAll(dir)
				return "", nil, nil, fmt.Errorf("create secret file: %w", err)
			}
			if _, err = file.Write(secret.Value); err == nil {
				err = file.Sync()
			}
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
			if err != nil {
				_ = os.RemoveAll(dir)
				return "", nil, nil, fmt.Errorf("write secret file: %w", err)
			}
			mounts = append(mounts, &runtime.Mount{HostPath: hostPath, ContainerPath: secret.Path, ReadOnly: true})
		default:
			_ = os.RemoveAll(dir)
			return "", nil, nil, fmt.Errorf("unsupported secret target")
		}
	}
	return dir, env, mounts, nil
}

// OnHealthy and OnUnhealthy are observation callbacks from the health manager.
// They intentionally do not mutate allocation status directly; lifecycle state
// transitions are centralized in the allocation reconciler.
func (a *Agent) OnHealthy(_ context.Context, allocID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if allocation := a.allocations[allocID]; allocation != nil {
		allocation.Health = "healthy"
		return a.persistAllocation(allocation)
	}
	return nil
}

func (a *Agent) OnUnhealthy(_ context.Context, allocID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if allocation := a.allocations[allocID]; allocation != nil {
		allocation.Health = "unhealthy"
		return a.persistAllocation(allocation)
	}
	return nil
}

func (a *Agent) OnReconciledStatus(allocID, status string) {
	a.mu.Lock()
	if alloc := a.allocations[allocID]; alloc != nil {
		if status == "healthy" || status == "unhealthy" {
			alloc.Health = status
		} else {
			alloc.Status = status
		}
		if err := a.persistAllocation(alloc); err != nil {
			a.log.Error("persist reconciled allocation", "allocation", alloc.AllocationID, "error", err)
		}
	}
	a.mu.Unlock()
}

func (a *Agent) OnRestartState(allocID string, attempts int, window time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if allocation := a.allocations[allocID]; allocation != nil {
		allocation.RestartAttempts, allocation.RestartWindow = attempts, window
		if err := a.persistAllocation(allocation); err != nil {
			a.log.Error("persist restart tracking", "allocation", allocation.AllocationID, "error", err)
		}
	}
}

func (a *Agent) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	registered := false

	for {
		if !registered {
			if a.server.Ready() {
				a.nodeInfo.Volumes = a.volumes.AvailableHostVolumes()
				if _, err := a.server.RegisterNode(ctx, &a.nodeInfo); err != nil {
					a.log.Error("register node failed", "error", err)
				} else {
					registered = true
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !a.server.Ready() {
				continue
			}
			if !registered {
				continue
			}
			a.mu.RLock()
			actual := make([]api.AllocationStatus, 0, len(a.allocations))
			for _, alloc := range a.allocations {
				ports := make([]api.PortMapping, 0, len(alloc.Ports))
				for _, p := range alloc.Ports {
					ports = append(ports, api.PortMapping{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
				}
				actual = append(actual, api.AllocationStatus{ID: alloc.AllocationID, Generation: alloc.Generation, Task: alloc.TaskName, Phase: lifecycle.Phase(alloc.Status), Health: lifecycle.Health(alloc.Health), Status: lifecycle.CompatibilityStatus(lifecycle.Phase(alloc.Status), lifecycle.Health(alloc.Health)), Ports: ports})
			}
			a.mu.RUnlock()
			response, err := a.server.SendHeartbeat(ctx, a.nodeID, &client.Heartbeat{
				NodeID:      a.nodeID,
				Timestamp:   time.Now(),
				Allocations: actual,
				Volumes:     a.volumes.AvailableHostVolumes(),
			})
			if err != nil {
				a.log.Error("send heartbeat failed", "error", err)
				registered = false
			} else {
				a.reconcileDesired(ctx, response)
			}
		}
	}
}
