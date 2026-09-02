// Package api defines the wire protocol shared by Trellis components.
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
	Tasks         []spec.TaskSpec         `json:"tasks"`
	Runtime       string                  `json:"runtime,omitempty"`
	NetworkPlan   *network.Plan           `json:"network_plan,omitempty"`
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
	OperationOK OperationCode = "ok"
	// OperationStaleEpoch indicates that the request used an old leadership epoch.
	OperationStaleEpoch OperationCode = "stale_epoch"
	// OperationStaleGeneration indicates that the request used an old allocation generation.
	OperationStaleGeneration OperationCode = "stale_generation"
	// OperationConflict indicates a conflicting allocation execution.
	OperationConflict OperationCode = "execution_conflict"
	// OperationFailed indicates that an agent operation failed.
	OperationFailed OperationCode = "operation_failed"
)

// OperationResponse reports the outcome of an agent operation.
type OperationResponse struct {
	Code       OperationCode `json:"code"`
	Message    string        `json:"message,omitempty"`
	Generation uint64        `json:"generation,omitempty"`
	Epoch      uint64        `json:"epoch,omitempty"`
}
