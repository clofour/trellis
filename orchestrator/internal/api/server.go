package api

import (
	"time"

	"github.com/clofour/trellis/internal/lifecycle"
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

type DesiredAllocation struct {
	ID         string `json:"id"`
	Generation uint64 `json:"generation"`
}

type HeartbeatResponse struct {
	Epoch              uint64              `json:"epoch"`
	Desired            []DesiredAllocation `json:"desired"`
	OrphanConfirmation bool                `json:"orphan_confirmation"`
}

type AllocationStatus struct {
	ID         string           `json:"id"`
	Generation uint64           `json:"generation,omitempty"`
	Task       string           `json:"task,omitempty"`
	Phase      lifecycle.Phase  `json:"phase,omitempty"`
	Health     lifecycle.Health `json:"health,omitempty"`
	Status     string           `json:"status"`
	Ports      []PortMapping    `json:"ports,omitempty"`
}

type PortMapping struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
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
	ID               string            `json:"id"`
	Job              string            `json:"job,omitempty"`
	Group            string            `json:"group"`
	Task             string            `json:"task,omitempty"`
	Namespace        string            `json:"namespace,omitempty"`
	NodeID           uuid.UUID         `json:"node_id"`
	Labels           map[string]string `json:"labels,omitempty"`
	Address          string            `json:"address,omitempty"`
	Ports            []PortMapping     `json:"ports,omitempty"`
	Status           string            `json:"status"`
	Phase            lifecycle.Phase   `json:"phase"`
	Health           lifecycle.Health  `json:"health"`
	Generation       uint64            `json:"generation"`
	JobRevision      int               `json:"job_revision"`
	CreatedAt        time.Time         `json:"created_at"`
	LastTransitionAt time.Time         `json:"last_transition_at"`
	Reason           string            `json:"reason,omitempty"`
	Message          string            `json:"message,omitempty"`
	Attempt          int               `json:"attempt"`
	NextRetryAt      *time.Time        `json:"next_retry_at,omitempty"`
}

type AllocationListResponse = []AllocationResponse

type AllocationEventResponse struct {
	Phase   lifecycle.Phase `json:"phase"`
	Reason  string          `json:"reason,omitempty"`
	Message string          `json:"message,omitempty"`
	At      time.Time       `json:"at"`
}

type AllocationEventListResponse = []AllocationEventResponse

type JobListResponse = []JobStatusResponse

type ServiceEntry struct {
	ID        string            `json:"id"`
	Job       string            `json:"job"`
	Group     string            `json:"group"`
	Namespace string            `json:"namespace"`
	Labels    map[string]string `json:"labels,omitempty"`
	Address   string            `json:"address"`
	Ports     []PortMapping     `json:"ports,omitempty"`
	Status    string            `json:"status"`
}

type ServiceListResponse = []ServiceEntry

type RaftJoinRequest struct {
	ID          string `json:"id"`
	RaftAddress string `json:"raft_address"`
}

type RaftJoinResponse struct {
	CACert string `json:"ca_cert"`
	CAKey  string `json:"ca_key"`
}
