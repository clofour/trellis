package api

type RunRequest struct {
	JobName     string              `json:"job_name"`
	GroupName   string              `json:"group_name"`
	Allocations []AllocationRequest `json:"allocations"`
}

type AllocationRequest struct {
	Name        string             `json:"job_name"`
	Image       string             `json:"image"`
	Env         map[string]string  `json:"env"`
	Ports       []PortRequest      `json:"ports"`
	Volumes     []VolumeRequest    `json:"volumes"`
	HealthCheck HealthCheckRequest `json:"health_check"`
}
