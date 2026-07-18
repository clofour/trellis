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
