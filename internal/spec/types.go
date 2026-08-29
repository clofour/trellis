package spec

type JobSpec struct {
	Name       string          `yaml:"name" json:"name"`
	Tenant     string          `yaml:"tenant,omitempty" json:"tenant,omitempty"`
	Isolation  *IsolationSpec  `yaml:"isolation,omitempty" json:"isolation,omitempty"`
	TaskGroups []TaskGroupSpec `yaml:"task_groups" json:"task_groups"`
}

// IsolationSpec opts a job into the restrictions used by an untrusted tenant.
// Omitting it preserves Trellis' trusted, single-tenant behaviour.
type IsolationSpec struct {
	Runtime string         `yaml:"runtime" json:"runtime"`
	Network *WireGuardSpec `yaml:"network,omitempty" json:"network,omitempty"`
	Quota   *ResourcesSpec `yaml:"quota,omitempty" json:"quota,omitempty"`
}

type WireGuardSpec struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
	// Network is retained for manifest compatibility. Trellis derives the
	// effective network from Tenant and never trusts this user-supplied value.
	Network string `yaml:"network,omitempty" json:"network,omitempty"`
}

type TaskGroupSpec struct {
	Name  string     `yaml:"name" json:"name"`
	Count int        `yaml:"count" json:"count"`
	Tasks []TaskSpec `yaml:"tasks" json:"tasks"`
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
	Type    string   `yaml:"type" json:"type"`
	Port    int      `yaml:"port" json:"port"`
	Path    string   `yaml:"path" json:"path,omitempty"`
	Command []string `yaml:"command" json:"command,omitempty"`
}

type VolumeSpec struct {
	Name string `yaml:"name" json:"name"`
	Path string `yaml:"path" json:"path"`
}
