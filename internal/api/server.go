package api

import (
	"time"

	"github.com/google/uuid"
)

type NodeStatusResponse string

const (
	StatusHealthy   NodeStatusResponse = "healthy"
	StatusUnhealthy NodeStatusResponse = "unhealthy"
	StatusDraining  NodeStatusResponse = "draining"
)

type NodeResponse struct {
	ID            uuid.UUID          `json:"id"`
	Host          string             `json:"host"`
	Port          int                `json:"port"`
	Status        NodeStatusResponse `json:"status"`
	LastHeartbeat time.Time          `json:"last_heartbeat"`
}

type NodeListResponse = []NodeResponse

type NodeRegistrationRequest struct {
	ID     uuid.UUID `json:"id"`
	Host   string    `json:"host"`
	Port   int       `json:"port"`
	CPU    int       `json:"cpu"`
	Memory int64     `json:"memory"`
	OS     string    `json:"os"`
	Arch   string    `json:"arch"`
}

type NodeRegistrationResponse struct {
	ID uuid.UUID `json:"id"`
}

type HeartbeatRequest struct {
	NodeID    uuid.UUID `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

type JobRegistrationRequest struct {
	Name       string                         `json:"name"`
	TaskGroups []TaskGroupRegistrationRequest `json:"task_groups"`
}

type TaskGroupRegistrationRequest struct {
	Name  string                    `json:"name"`
	Count int                       `json:"count"`
	Tasks []TaskRegistrationRequest `json:"tasks"`
}

type TaskRegistrationRequest struct {
	Name        string                          `json:"name"`
	Image       string                          `json:"image"`
	Env         map[string]string               `json:"env"`
	Ports       []PortRegistrationRequest       `json:"ports"`
	Volumes     []VolumeRegistrationRequest     `json:"volumes"`
	Resources   *ResourcesRegistrationRequest   `json:"resources"`
	HealthCheck *HealthCheckRegistrationRequest `json:"health_check"`
}

type PortRegistrationRequest struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

type ResourcesRegistrationRequest struct {
	CPU    int `json:"cpu"`
	Memory int `json:"memory"`
}

type HealthCheckRegistrationRequest struct {
	Type    string   `json:"type"`
	Port    int      `json:"port"`
	Path    string   `json:"path"`
	Command []string `json:"command"`
}

type VolumeRegistrationRequest struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
