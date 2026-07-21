package api

type PortRequest struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

type VolumeRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type HealthCheckRequest struct {
	Type    string   `json:"type"`
	Port    int      `json:"port"`
	Path    string   `json:"path"`
	Command []string `json:"command"`
}
