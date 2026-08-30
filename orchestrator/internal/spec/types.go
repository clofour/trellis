package spec

import "time"

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

type TaskGroupSpec struct {
	Name        string             `yaml:"name" json:"name"`
	Count       int                `yaml:"count" json:"count"`
	Runtime     string             `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Tasks       []TaskSpec         `yaml:"tasks" json:"tasks"`
	Labels      map[string]string  `yaml:"labels,omitempty" json:"labels,omitempty"`
	NetworkMode string             `yaml:"network_mode,omitempty" json:"network_mode,omitempty"`
	APIAccess   bool               `yaml:"api_access,omitempty" json:"api_access,omitempty"`
	Restart     *RestartPolicySpec `yaml:"restart,omitempty" json:"restart,omitempty"`
	Constraints []ConstraintSpec   `yaml:"constraints,omitempty" json:"constraints,omitempty"`
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
	Type      string        `yaml:"type" json:"type"`
	Port      int           `yaml:"port" json:"port"`
	Path      string        `yaml:"path" json:"path,omitempty"`
	Command   []string      `yaml:"command" json:"command,omitempty"`
	Interval  time.Duration `yaml:"interval,omitempty" json:"interval,omitempty"`
	Timeout   time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Threshold int           `yaml:"threshold,omitempty" json:"threshold,omitempty"`
}

type VolumeSpec struct {
	Name string `yaml:"name" json:"name"`
	Path string `yaml:"path" json:"path"`
}
