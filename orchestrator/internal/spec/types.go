package spec

import (
	"crypto/sha256"
	"encoding/json"
	"time"
)

type UpdateStrategy string

const (
	UpdateRecreate UpdateStrategy = "recreate"
	UpdateRolling  UpdateStrategy = "rolling"
)

func (s UpdateStrategy) Valid() bool { return s == "" || s == UpdateRecreate || s == UpdateRolling }

type Runtime string

const (
	RuntimeDefault Runtime = ""
	RuntimeRunc    Runtime = "runc"
	RuntimeRunsc   Runtime = "runsc"
)

func (r Runtime) Valid() bool { return r == RuntimeDefault || r == RuntimeRunc || r == RuntimeRunsc }

type APIAccessScope string

const (
	APIAccessNamespace APIAccessScope = "namespace"
	APIAccessCluster   APIAccessScope = "cluster"
)

func (s APIAccessScope) Valid() bool { return s == APIAccessNamespace || s == APIAccessCluster }

type APIAccessLevel string

const (
	APIAccessRead  APIAccessLevel = "read"
	APIAccessWrite APIAccessLevel = "write"
)

func (a APIAccessLevel) Valid() bool { return a == APIAccessRead || a == APIAccessWrite }

type APIAccessSpec struct {
	Scope  APIAccessScope `yaml:"scope" json:"scope"`
	Access APIAccessLevel `yaml:"access" json:"access"`
}

type TaskNetworkMode string

const (
	TaskNetworkIsolated  TaskNetworkMode = ""
	TaskNetworkHost      TaskNetworkMode = "host"
	TaskNetworkWireGuard TaskNetworkMode = "wireguard"
)

func (m TaskNetworkMode) Valid() bool {
	return m == TaskNetworkIsolated || m == TaskNetworkHost || m == TaskNetworkWireGuard
}

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
	TaskGroups []TaskGroupSpec `yaml:"task_groups" json:"task_groups"`
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
	APIAccess   *APIAccessSpec     `yaml:"api_access,omitempty" json:"api_access,omitempty"`
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

type TaskNetworkingSpec struct {
	Mode  TaskNetworkMode `yaml:"mode,omitempty" json:"mode,omitempty"`
	Ports []PortSpec      `yaml:"ports,omitempty" json:"ports,omitempty"`
}

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

// PortSpec reserves the single port used directly by a host-networked task.
// HostPort is not part of either the YAML or JSON model; it exists only so
// agent diagnostics compiled against PortSpec can name the already-normalized
// port without reintroducing a second user-facing port concept.
type PortSpec struct {
	Port     int `yaml:"port" json:"port"`
	HostPort int `yaml:"-" json:"-"`
}

type ResourcesSpec struct {
	CPU    int      `yaml:"cpu" json:"cpu"`
	Memory ByteSize `yaml:"memory" json:"memory"`
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
