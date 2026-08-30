package api

import (
	"github.com/clofour/trellis/internal/network"
	"github.com/clofour/trellis/internal/spec"
)

type AllocationRequest struct {
	Namespace    string                  `json:"namespace,omitempty"`
	JobName      string                  `json:"job_name"`
	GroupName    string                  `json:"group_name"`
	Name         string                  `json:"name"`
	Tasks        []spec.TaskSpec         `json:"tasks"`
	Runtime      string                  `json:"runtime,omitempty"`
	WireGuard    bool                    `json:"wireguard"`
	NetworkPlan  *network.Plan           `json:"network_plan,omitempty"`
	NetworkMode  string                  `json:"network_mode,omitempty"`
	EnvOverrides map[string]string       `json:"env_overrides,omitempty"`
	Restart      *spec.RestartPolicySpec `json:"restart,omitempty"`
}
