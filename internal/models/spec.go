package models

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
	Ports       []string
	Env         map[string]string
	Resources   *ResourcesSpec
	HealthCheck *HealthCheckSpec
	Volumes     []VolumeSpec
}

type ResourcesSpec struct {
	CPU    int
	Memory int
}

type HealthCheckSpec struct {
	Type    string
	Port    int
	Path    string
	Command string
}

type VolumeSpec struct {
	Name string
	Path string
}
