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
	ID            uuid.UUID
	Host          string
	Port          int
	Status        NodeStatusResponse
	LastHeartbeat time.Time
}

type NodeListResponse = []NodeResponse

type NodeRegistrationRequest struct {
	ID     uuid.UUID
	Host   string
	Port   int
	CPU    int
	Memory int64
	OS     string
	Arch   string
}

type NodeRegistrationResponse struct {
	ID uuid.UUID
}

type HeartbeatRequest struct {
	NodeID    uuid.UUID
	Timestamp time.Time
}
