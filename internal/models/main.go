package models

type Job struct {
	Name       string
	TaskGroups []TaskGroup
}

type TaskGroup struct {
	Name  string
	Count int
	Tasks []Task
}

type Task struct {
	Name        string
	Image       string
	Ports       []string
	Env         map[string]string
	Resources   *Resources
	HealthCheck *HealthCheck
	Volumes     []Volume
}

type Resources struct {
	CPU    int
	Memory int
}

type HealthCheck struct {
	Type    string
	Port    int
	Path    string
	Command string
}

type Volume struct {
	Name string
	Path string
}

type Mount struct {
	Name          string
	HostPath      string
	ContainerPath string
}
