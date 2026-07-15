package spec

type JobSpec struct {
	Name       string
	TaskGroups []TaskGroupSpec
}

type TaskGroupSpec struct {
	Name  string
	Count int
	Tasks []TaskSpec
}

type TaskSpec struct {
	Name        string
	Image       string
	Env         map[string]string
	Ports       []PortSpec
	Volumes     []VolumeSpec
	Resources   *ResourcesSpec
	HealthCheck *HealthCheckSpec
}

type PortSpec struct {
	HostPort      int
	ContainerPort int
}

type ResourcesSpec struct {
	CPU    int
	Memory int
}

type HealthCheckSpec struct {
	Type    string
	Port    int
	Path    string
	Command []string
}

type VolumeSpec struct {
	Name string
	Path string
}
