package api

import (
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/spec"
)

// AllocationRequest describes an allocation for an agent to start.
type AllocationRequest struct {
	AllocationID  string                  `json:"allocation_id"`
	Generation    uint64                  `json:"generation"`
	JobRevision   int                     `json:"job_revision"`
	Epoch         uint64                  `json:"epoch"`
	ExecutionHash string                  `json:"execution_hash"`
	Namespace     string                  `json:"namespace,omitempty"`
	JobName       string                  `json:"job_name"`
	GroupName     string                  `json:"group_name"`
	Name          string                  `json:"name"`
	Tasks         []spec.TaskSpec         `json:"tasks"`
	Runtime       string                  `json:"runtime,omitempty"`
	WireGuard     bool                    `json:"wireguard"`
	NetworkPlan   *network.Plan           `json:"network_plan,omitempty"`
	NetworkMode   string                  `json:"network_mode,omitempty"`
	EnvOverrides  map[string]string       `json:"env_overrides,omitempty"`
	Restart       *spec.RestartPolicySpec `json:"restart,omitempty"`
	Secrets       []DeliveredSecret       `json:"secrets,omitempty"`
}

// StopAllocationRequest identifies an allocation generation to stop.
type StopAllocationRequest struct {
	AllocationID string `json:"allocation_id"`
	Generation   uint64 `json:"generation"`
	Epoch        uint64 `json:"epoch"`
}

// OperationCode identifies the result of an agent operation.
type OperationCode string

const (
	// OperationOK and the following values describe agent operation results.
	OperationOK              OperationCode = "ok"
	OperationStaleEpoch      OperationCode = "stale_epoch"
	OperationStaleGeneration OperationCode = "stale_generation"
	OperationConflict        OperationCode = "execution_conflict"
	OperationFailed          OperationCode = "operation_failed"
)

// OperationResponse reports the outcome of an agent operation.
type OperationResponse struct {
	Code       OperationCode `json:"code"`
	Message    string        `json:"message,omitempty"`
	Generation uint64        `json:"generation,omitempty"`
	Epoch      uint64        `json:"epoch,omitempty"`
}
