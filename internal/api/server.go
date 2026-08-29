package api

import (
	"time"

	"github.com/clofour/trellis/internal/spec"
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
	CPU           int                `json:"cpu"`
	Memory        int64              `json:"memory"`
}

type NodeListResponse = []NodeResponse

type NodeRegistrationRequest struct {
	ID                 uuid.UUID `json:"id"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	CPU                int       `json:"cpu"`
	Memory             int64     `json:"memory"`
	OS                 string    `json:"os"`
	Arch               string    `json:"arch"`
	WireGuardPublicKey string    `json:"wireguard_public_key,omitempty"`
	WireGuardEndpoint  string    `json:"wireguard_endpoint,omitempty"`
}

type NodeRegistrationResponse struct {
	ID uuid.UUID `json:"id"`
}

type HeartbeatRequest struct {
	NodeID      uuid.UUID          `json:"id"`
	Timestamp   time.Time          `json:"timestamp"`
	Allocations []AllocationStatus `json:"allocations,omitempty"`
}

type AllocationStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type JobRegistrationRequest struct {
	Spec spec.JobSpec `json:"spec"`
}

type JobStatusResponse struct {
	Name        string               `json:"name"`
	Revision    int                  `json:"revision"`
	Desired     int                  `json:"desired"`
	Running     int                  `json:"running"`
	Healthy     int                  `json:"healthy"`
	Allocations []AllocationResponse `json:"allocations"`
}

type AllocationResponse struct {
	ID     string    `json:"id"`
	Group  string    `json:"group"`
	Task   string    `json:"task"`
	NodeID uuid.UUID `json:"node_id"`
	Status string    `json:"status"`
}
