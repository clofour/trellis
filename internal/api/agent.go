package api

import "github.com/clofour/trellis/internal/spec"

type AllocationRequest struct {
	JobName   string         `json:"job_name"`
	GroupName string         `json:"group_name"`
	Name      string         `json:"name"`
	Task      *spec.TaskSpec `json:"task"`
}
