package spec

import (
	"crypto/sha256"
	"encoding/json"
	"time"
)

// UpdateStrategy controls how old allocations are replaced during a job revision.
type UpdateStrategy string

const (
	// UpdateRecreate and UpdateRolling describe allocation replacement strategies.
	UpdateRecreate UpdateStrategy = "recreate"
	// UpdateRolling replaces allocations incrementally.
	UpdateRolling UpdateStrategy = "rolling"
)

// Valid reports whether s is a supported update strategy.
func (s UpdateStrategy) Valid() bool {
	return s == "" || s == UpdateRecreate || s == UpdateRolling
}

// Runtime identifies the OCI runtime used for a task group.
type Runtime string

const (
	// RuntimeDefault uses the orchestrator default OCI runtime.
	RuntimeDefault Runtime = ""
	// RuntimeRunc selects the runc OCI runtime.
	RuntimeRunc Runtime = "runc"
	// RuntimeRunsc selects the runsc OCI runtime.
	RuntimeRunsc Runtime = "runsc"
)

// Valid reports whether r is a supported runtime.
func (r Runtime) Valid() bool {
	return r == RuntimeDefault || r == RuntimeRunc || r == RuntimeRunsc
}

// APIAccessScope controls where an injected control-plane credential may operate.
type APIAccessScope string

const (
	// APIAccessNamespace restricts the credential to the job's namespace.
	APIAccessNamespace APIAccessScope = "namespace"
	// APIAccessCluster permits cluster-scoped access.
	APIAccessCluster APIAccessScope = "cluster"
)

// Valid reports whether s is a supported API access scope.
func (s APIAccessScope) Valid() bool {
	return s == APIAccessNamespace || s == APIAccessCluster
}

// APIAccessLevel controls what an injected control-plane credential may do.
type APIAccessLevel string

const (
	// APIAccessRead permits read-only API operations.
	APIAccessRead APIAccessLevel = "read"
	// APIAccessWrite permits mutations as well as reads.
	APIAccessWrite APIAccessLevel = "write"
)

// Valid reports whether a is a supported API access level.
func (a APIAccessLevel) Valid() bool {
	return a == APIAccessRead || a == APIAccessWrite
}

// APIAccessSpec requests a least-privilege control-plane credential for a task group.
type APIAccessSpec struct {
	Scope  APIAccessScope `yaml:"scope" json:"scope"`
	Access APIAccessLevel `yaml:"access" json:"access"`
}

// TaskNetworkMode controls how a task container joins the network.
type TaskNetworkMode string

const (
	// TaskNetworkIsolated gives the container a private network namespace with no external routes.
	TaskNetworkIsolated TaskNetworkMode = ""
	// TaskNetworkHost joins the host network namespace directly.
	TaskNetworkHost TaskNetworkMode = "host"
	// TaskNetworkWireGuard gives the container a private namespace connected to the WireGuard mesh.
	TaskNetworkWireGuard TaskNetworkMode = "wireguard"
)

// Valid reports whether m is a supported task network mode.
func (m TaskNetworkMode) Valid() bool {
	return m == TaskNetworkIsolated || m == TaskNetworkHost || m == TaskNetworkWireGuard
}

// HealthCheckType identifies a supported health-check implementation.
type HealthCheckType string

const (
	// HealthCheckHTTP and the following values select a health-check mechanism.
	HealthCheckHTTP HealthCheckType = "http"
	// HealthCheckTCP performs a TCP health check.
	HealthCheckTCP HealthCheckType = "tcp"
	// HealthCheckScript performs a command health check.
	HealthCheckScript HealthCheckType = "script"
)

// Valid reports whether t is a supported health-check type.
func (t HealthCheckType) Valid() bool {
	return t == HealthCheckHTTP || t == HealthCheckTCP || t == HealthCheckScript
}

// JobSpec describes a job and its task groups.
type JobSpec struct {
	Name       string          `yaml:"name" json:"name"`
	Namespace  string          `yaml:"namespace" json:"namespace"`
	TaskGroups []TaskGroupSpec `yaml:"task_groups" json:"task_groups"`
}

// UpdateSpec configures allocation replacement for a task group.
type UpdateSpec struct {
	Strategy    UpdateStrategy `yaml:"strategy" json:"strategy"`
	MaxParallel int            `yaml:"max_parallel,omitempty" json:"max_parallel,omitempty"`
}

// TaskGroupSpec describes a scalable group of tasks.
type TaskGroupSpec struct {
	Name        string             `yaml:"name" json:"name"`
	Count       int                `yaml:"count" json:"count"`
	Runtime     Runtime            `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Tasks       []TaskSpec         `yaml:"tasks" json:"tasks"`
	Labels      map[string]string  `yaml:"labels,omitempty" json:"labels,omitempty"`
	APIAccess   *APIAccessSpec     `yaml:"api_access,omitempty" json:"api_access,omitempty"`
	Restart     *RestartPolicySpec `yaml:"restart,omitempty" json:"restart,omitempty"`
	Constraints []ConstraintSpec   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	Update      *UpdateSpec        `yaml:"update,omitempty" json:"update,omitempty"`
}

// ConstraintSpec requires a node attribute to match a value.
type ConstraintSpec struct {
	Attribute string `yaml:"attribute" json:"attribute"`
	Value     string `yaml:"value" json:"value"`
}

// RestartPolicySpec configures retries for failed tasks.
type RestartPolicySpec struct {
	MaxRestarts int           `yaml:"max_restarts" json:"max_restarts"`
	Window      time.Duration `yaml:"window" json:"window"`
}

// TaskNetworkingSpec configures a task's network attachment.
type TaskNetworkingSpec struct {
	Mode  TaskNetworkMode `yaml:"mode,omitempty" json:"mode,omitempty"`
	Ports []PortSpec      `yaml:"ports,omitempty" json:"ports,omitempty"`
}

// TaskSpec describes a container task.
type TaskSpec struct {
	Name        string              `yaml:"name" json:"name"`
	Image       string              `yaml:"image" json:"image"`
	Env         map[string]string   `yaml:"env" json:"env,omitempty"`
	Networking  *TaskNetworkingSpec `yaml:"networking,omitempty" json:"networking,omitempty"`
	Volumes     []VolumeSpec        `yaml:"volumes" json:"volumes,omitempty"`
	Resources   *ResourcesSpec      `yaml:"resources" json:"resources,omitempty"`
	HealthCheck *HealthCheckSpec    `yaml:"health_check" json:"health_check,omitempty"`
	Secrets     []SecretRefSpec     `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

// SecretTarget identifies how a secret is delivered to a task.
type SecretTarget string

const (
	// SecretTargetEnv and SecretTargetFile select the delivery mechanism.
	SecretTargetEnv SecretTarget = "env"
	// SecretTargetFile delivers a secret as a file.
	SecretTargetFile SecretTarget = "file"
)

// SecretRefSpec maps a stored secret into a task.
type SecretRefSpec struct {
	Name   string       `yaml:"name" json:"name"`
	Target SecretTarget `yaml:"target" json:"target"`
	Env    string       `yaml:"env,omitempty" json:"env,omitempty"`
	Path   string       `yaml:"path,omitempty" json:"path,omitempty"`
	Mode   uint32       `yaml:"mode,omitempty" json:"mode,omitempty"`
}

// PortSpec reserves a port used directly by a host-networked task.
type PortSpec struct {
	Port int `yaml:"port" json:"port"`
}

// ResourcesSpec describes task CPU and memory requirements.
type ResourcesSpec struct {
	CPU    int      `yaml:"cpu" json:"cpu"`
	Memory ByteSize `yaml:"memory" json:"memory"`
}

// HealthCheckSpec configures task health monitoring.
type HealthCheckSpec struct {
	Type      HealthCheckType `yaml:"type" json:"type"`
	Port      int             `yaml:"port" json:"port"`
	Path      string          `yaml:"path" json:"path,omitempty"`
	Command   []string        `yaml:"command" json:"command,omitempty"`
	Interval  time.Duration   `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout   time.Duration   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Threshold int             `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

// VolumeSpec mounts a host volume into a task.
type VolumeSpec struct {
	Name       string `yaml:"name" json:"name"`
	Path       string `yaml:"path" json:"path"`
	HostVolume string `yaml:"host_volume,omitempty" json:"host_volume,omitempty"`
	ReadOnly   bool   `yaml:"read_only,omitempty" json:"read_only,omitempty"`
}

// TaskGroupContentHash returns a SHA-256 hex digest of the task group fields
// that affect running containers. Labels, update strategy, and count are
// excluded so that changes to those fields can be detected as metadata-only.
func TaskGroupContentHash(g *TaskGroupSpec) string {
	hashable := struct {
		Name        string
		Runtime     Runtime
		Tasks       []TaskSpec
		APIAccess   *APIAccessSpec
		Restart     *RestartPolicySpec
		Constraints []ConstraintSpec
	}{
		Name:        g.Name,
		Runtime:     g.Runtime,
		Tasks:       g.Tasks,
		APIAccess:   g.APIAccess,
		Restart:     g.Restart,
		Constraints: g.Constraints,
	}
	raw, _ := json.Marshal(hashable)
	h := sha256.Sum256(raw)
	return string(h[:])
}
