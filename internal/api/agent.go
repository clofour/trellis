package api

type RunRequest struct {
	JobName     string
	GroupName   string
	Allocations []AllocationRequest
}

type AllocationRequest struct {
	Name        string
	Image       string
	Env         map[string]string
	Ports       []PortRequest
	Volumes     []VolumeRequest
	HealthCheck HealthCheckRequest
}

type PortRequest struct {
	HostPort      int
	ContainerPort int
}

type VolumeRequest struct {
	Name string
	Path string
}

type HealthCheckRequest struct {
	Type    string
	Port    int
	Path    string
	Command []string
}
