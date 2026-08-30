package spec

import (
	"crypto/sha256"
	"encoding/json"
	"time"
)

// UpdateStrategy controls how old allocations are replaced during a job revision.
type UpdateStrategy string

const (
	UpdateRecreate UpdateStrategy = "recreate"
	UpdateRolling  UpdateStrategy = "rolling"
)

func (s UpdateStrategy) Valid() bool {
	return s == "" || s == UpdateRecreate || s == UpdateRolling
}

// Runtime identifies the OCI runtime used for a task group.
type Runtime string

const (
	RuntimeDefault Runtime = ""
	RuntimeRunc    Runtime = "runc"
	RuntimeRunsc   Runtime = "runsc"
)

func (r Runtime) Valid() bool {
	return r == RuntimeDefault || r == RuntimeRunc || r == RuntimeRunsc
}

// NetworkMode controls how a task group joins the host network.
type NetworkMode string

const (
	NetworkModeIsolated NetworkMode = ""
	NetworkModeHost     NetworkMode = "host"
)

func (m NetworkMode) Valid() bool {
	return m == NetworkModeIsolated || m == NetworkModeHost
}

// HealthCheckType identifies a supported health-check implementation.
type HealthCheckType string

const (
	HealthCheckHTTP   HealthCheckType = "http"
	HealthCheckTCP    HealthCheckType = "tcp"
	HealthCheckScript HealthCheckType = "script"
)

func (t HealthCheckType) Valid() bool {
	return t == HealthCheckHTTP || t == HealthCheckTCP || t == HealthCheckScript
}

type JobSpec struct {
	Name       string          `yaml:"name" json:"name"`
	Namespace  string          `yaml:"namespace" json:"namespace"`
	Network    *NetworkSpec    `yaml:"network,omitempty" json:"network,omitempty"`
	TaskGroups []TaskGroupSpec `yaml:"task_groups" json:"task_groups"`
}

// NetworkSpec selects an implementation mechanism for the namespace network.
// Namespace isolation and network identity never depend on this setting.
type NetworkSpec struct {
	WireGuard bool `yaml:"wireguard" json:"wireguard"`
}

type UpdateSpec struct {
	Strategy    UpdateStrategy `yaml:"strategy" json:"strategy"`
	MaxParallel int            `yaml:"max_parallel,omitempty" json:"max_parallel,omitempty"`
}

type TaskGroupSpec struct {
	Name        string             `yaml:"name" json:"name"`
	Count       int                `yaml:"count" json:"count"`
	Runtime     Runtime            `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Tasks       []TaskSpec         `yaml:"tasks" json:"tasks"`
	Labels      map[string]string  `yaml:"labels,omitempty" json:"labels,omitempty"`
	NetworkMode NetworkMode        `yaml:"network_mode,omitempty" json:"network_mode,omitempty"`
	APIAccess   bool               `yaml:"api_access,omitempty" json:"api_access,omitempty"`
	Restart     *RestartPolicySpec `yaml:"restart,omitempty" json:"restart,omitempty"`
	Constraints []ConstraintSpec   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	Update      *UpdateSpec        `yaml:"update,omitempty" json:"update,omitempty"`
}

type ConstraintSpec struct {
	Attribute string `yaml:"attribute" json:"attribute"`
	Value     string `yaml:"value" json:"value"`
}

type RestartPolicySpec struct {
	MaxRestarts int           `yaml:"max_restarts" json:"max_restarts"`
	Window      time.Duration `yaml:"window" json:"window"`
}

type TaskSpec struct {
	Name        string            `yaml:"name" json:"name"`
	Image       string            `yaml:"image" json:"image"`
	Env         map[string]string `yaml:"env" json:"env,omitempty"`
	Ports       []PortSpec        `yaml:"ports" json:"ports,omitempty"`
	Volumes     []VolumeSpec      `yaml:"volumes" json:"volumes,omitempty"`
	Resources   *ResourcesSpec    `yaml:"resources" json:"resources,omitempty"`
	HealthCheck *HealthCheckSpec  `yaml:"health_check" json:"health_check,omitempty"`
	Secrets     []SecretRefSpec   `yaml:"secrets,omitempty" json:"secrets,omitempty"`
}

type SecretTarget string

const (
	SecretTargetEnv  SecretTarget = "env"
	SecretTargetFile SecretTarget = "file"
)

type SecretRefSpec struct {
	Name   string       `yaml:"name" json:"name"`
	Target SecretTarget `yaml:"target" json:"target"`
	Env    string       `yaml:"env,omitempty" json:"env,omitempty"`
	Path   string       `yaml:"path,omitempty" json:"path,omitempty"`
	Mode   uint32       `yaml:"mode,omitempty" json:"mode,omitempty"`
}

type PortSpec struct {
	HostPort      int `yaml:"host_port" json:"host_port"`
	ContainerPort int `yaml:"container_port" json:"container_port"`
}

type ResourcesSpec struct {
	CPU    int `yaml:"cpu" json:"cpu"`
	Memory int `yaml:"memory" json:"memory"`
}

type HealthCheckSpec struct {
	Type      HealthCheckType `yaml:"type" json:"type"`
	Port      int             `yaml:"port" json:"port"`
	Path      string          `yaml:"path" json:"path,omitempty"`
	Command   []string        `yaml:"command" json:"command,omitempty"`
	Interval  time.Duration   `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout   time.Duration   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Threshold int             `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

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
		NetworkMode NetworkMode
		APIAccess   bool
		Restart     *RestartPolicySpec
		Constraints []ConstraintSpec
	}{
		Name:        g.Name,
		Runtime:     g.Runtime,
		Tasks:       g.Tasks,
		NetworkMode: g.NetworkMode,
		APIAccess:   g.APIAccess,
		Restart:     g.Restart,
		Constraints: g.Constraints,
	}
	raw, _ := json.Marshal(hashable)
	h := sha256.Sum256(raw)
	return string(h[:])
}
