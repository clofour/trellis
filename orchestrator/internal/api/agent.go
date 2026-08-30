package api

import (
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/spec"
)

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

type StopAllocationRequest struct {
	AllocationID string `json:"allocation_id"`
	Generation   uint64 `json:"generation"`
	Epoch        uint64 `json:"epoch"`
}

type OperationCode string

const (
	OperationOK              OperationCode = "ok"
	OperationStaleEpoch      OperationCode = "stale_epoch"
	OperationStaleGeneration OperationCode = "stale_generation"
	OperationConflict        OperationCode = "execution_conflict"
	OperationFailed          OperationCode = "operation_failed"
)

type OperationResponse struct {
	Code       OperationCode `json:"code"`
	Message    string        `json:"message,omitempty"`
	Generation uint64        `json:"generation,omitempty"`
	Epoch      uint64        `json:"epoch,omitempty"`
}
