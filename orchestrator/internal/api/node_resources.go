package api

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/clofour/trellis/internal/nodecapacity"
	"github.com/google/uuid"
)

type nodeResourceState struct {
	CPUCapacity       int
	MemoryCapacity    int64
	CPUAllocatable    int
	MemoryAllocatable int64
	CPUUsage          *float64
	MemoryUsed        *int64
	MemoryAvailable   *int64
	MetricsAt         *time.Time
}

var nodeResourceStates sync.Map

func loadNodeResourceState(id uuid.UUID) nodeResourceState {
	if state, ok := nodeResourceStates.Load(id); ok {
		return state.(nodeResourceState)
	}
	return nodeResourceState{}
}

func storeNodeResourceState(id uuid.UUID, update nodeResourceState) {
	state := loadNodeResourceState(id)
	if update.CPUCapacity != 0 {
		state.CPUCapacity = update.CPUCapacity
	}
	if update.MemoryCapacity != 0 {
		state.MemoryCapacity = update.MemoryCapacity
	}
	if update.CPUAllocatable != 0 {
		state.CPUAllocatable = update.CPUAllocatable
	}
	if update.MemoryAllocatable != 0 {
		state.MemoryAllocatable = update.MemoryAllocatable
	}
	if update.CPUUsage != nil {
		state.CPUUsage = update.CPUUsage
	}
	if update.MemoryUsed != nil {
		state.MemoryUsed = update.MemoryUsed
	}
	if update.MemoryAvailable != nil {
		state.MemoryAvailable = update.MemoryAvailable
	}
	if update.MetricsAt != nil {
		state.MetricsAt = update.MetricsAt
	}
	nodeResourceStates.Store(id, state)
}

type nodeRegistrationAlias NodeRegistrationRequest

type nodeRegistrationWire struct {
	nodeRegistrationAlias
	CPUCapacity       int   `json:"cpu_capacity,omitempty"`
	MemoryCapacity    int64 `json:"memory_capacity,omitempty"`
	CPUAllocatable    int   `json:"cpu_allocatable,omitempty"`
	MemoryAllocatable int64 `json:"memory_allocatable,omitempty"`
}

// MarshalJSON resolves the node's schedulable capacity before registration.
// The legacy cpu/memory fields continue to carry the values used by the
// scheduler, while explicit capacity/allocatable fields make the distinction
// visible to newer clients.
func (request NodeRegistrationRequest) MarshalJSON() ([]byte, error) {
	allocatableCPU, allocatableMemory, err := nodecapacity.Resolve(request.CPU, request.Memory)
	if err != nil {
		return nil, err
	}
	wireRequest := nodeRegistrationAlias(request)
	wireRequest.CPU = allocatableCPU
	wireRequest.Memory = allocatableMemory
	storeNodeResourceState(request.ID, nodeResourceState{
		CPUCapacity: request.CPU, MemoryCapacity: request.Memory,
		CPUAllocatable: allocatableCPU, MemoryAllocatable: allocatableMemory,
	})
	return json.Marshal(nodeRegistrationWire{
		nodeRegistrationAlias: wireRequest,
		CPUCapacity: request.CPU, MemoryCapacity: request.Memory,
		CPUAllocatable: allocatableCPU, MemoryAllocatable: allocatableMemory,
	})
}

func (request *NodeRegistrationRequest) UnmarshalJSON(data []byte) error {
	var wire nodeRegistrationWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*request = NodeRegistrationRequest(wire.nodeRegistrationAlias)
	capacityCPU, capacityMemory := wire.CPUCapacity, wire.MemoryCapacity
	allocatableCPU, allocatableMemory := wire.CPUAllocatable, wire.MemoryAllocatable
	if capacityCPU == 0 {
		capacityCPU = request.CPU
	}
	if capacityMemory == 0 {
		capacityMemory = request.Memory
	}
	if allocatableCPU == 0 {
		allocatableCPU = request.CPU
	}
	if allocatableMemory == 0 {
		allocatableMemory = request.Memory
	}
	storeNodeResourceState(request.ID, nodeResourceState{
		CPUCapacity: capacityCPU, MemoryCapacity: capacityMemory,
		CPUAllocatable: allocatableCPU, MemoryAllocatable: allocatableMemory,
	})
	return nil
}

type heartbeatAlias HeartbeatRequest

type heartbeatWire struct {
	heartbeatAlias
	CPUCapacity       int        `json:"cpu_capacity,omitempty"`
	MemoryCapacity    int64      `json:"memory_capacity,omitempty"`
	CPUAllocatable    int        `json:"cpu_allocatable,omitempty"`
	MemoryAllocatable int64      `json:"memory_allocatable,omitempty"`
	CPUUsage          *float64   `json:"cpu_usage,omitempty"`
	MemoryUsed        *int64     `json:"memory_used,omitempty"`
	MemoryAvailable   *int64     `json:"memory_available,omitempty"`
	MetricsAt         *time.Time `json:"metrics_at,omitempty"`
}

// MarshalJSON attaches fresh whole-host resource observations to the ordinary
// node heartbeat. Scheduling remains request-based and never consumes these
// live utilization values.
func (request HeartbeatRequest) MarshalJSON() ([]byte, error) {
	state := loadNodeResourceState(request.NodeID)
	if metrics, ok := nodecapacity.SampleHostMetrics(); ok {
		if metrics.CPUValid {
			value := metrics.CPUUsage
			state.CPUUsage = &value
		}
		if metrics.MemoryValid {
			used, available := metrics.MemoryUsed, metrics.MemoryAvailable
			state.MemoryUsed = &used
			state.MemoryAvailable = &available
		}
		collected := metrics.CollectedAt
		state.MetricsAt = &collected
		storeNodeResourceState(request.NodeID, state)
	}
	return json.Marshal(heartbeatWire{
		heartbeatAlias: heartbeatAlias(request),
		CPUCapacity: state.CPUCapacity, MemoryCapacity: state.MemoryCapacity,
		CPUAllocatable: state.CPUAllocatable, MemoryAllocatable: state.MemoryAllocatable,
		CPUUsage: state.CPUUsage, MemoryUsed: state.MemoryUsed,
		MemoryAvailable: state.MemoryAvailable, MetricsAt: state.MetricsAt,
	})
}

func (request *HeartbeatRequest) UnmarshalJSON(data []byte) error {
	var wire heartbeatWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*request = HeartbeatRequest(wire.heartbeatAlias)
	storeNodeResourceState(request.NodeID, nodeResourceState{
		CPUCapacity: wire.CPUCapacity, MemoryCapacity: wire.MemoryCapacity,
		CPUAllocatable: wire.CPUAllocatable, MemoryAllocatable: wire.MemoryAllocatable,
		CPUUsage: wire.CPUUsage, MemoryUsed: wire.MemoryUsed,
		MemoryAvailable: wire.MemoryAvailable, MetricsAt: wire.MetricsAt,
	})
	return nil
}

type nodeResponseAlias NodeResponse

type nodeResponseWire struct {
	nodeResponseAlias
	CPUCapacity       int        `json:"cpu_capacity"`
	MemoryCapacity    int64      `json:"memory_capacity"`
	CPUAllocatable    int        `json:"cpu_allocatable"`
	MemoryAllocatable int64      `json:"memory_allocatable"`
	CPUUsage          *float64   `json:"cpu_usage,omitempty"`
	MemoryUsed        *int64     `json:"memory_used,omitempty"`
	MemoryAvailable   *int64     `json:"memory_available,omitempty"`
	MetricsAt         *time.Time `json:"metrics_at,omitempty"`
}

// MarshalJSON exposes both physical and schedulable capacity plus the most
// recent whole-host utilization observation through node status.
func (response NodeResponse) MarshalJSON() ([]byte, error) {
	state := loadNodeResourceState(response.ID)
	if state.CPUAllocatable == 0 {
		state.CPUAllocatable = response.CPU
	}
	if state.MemoryAllocatable == 0 {
		state.MemoryAllocatable = response.Memory
	}
	if state.CPUCapacity == 0 {
		state.CPUCapacity = state.CPUAllocatable
	}
	if state.MemoryCapacity == 0 {
		state.MemoryCapacity = state.MemoryAllocatable
	}
	return json.Marshal(nodeResponseWire{
		nodeResponseAlias: nodeResponseAlias(response),
		CPUCapacity: state.CPUCapacity, MemoryCapacity: state.MemoryCapacity,
		CPUAllocatable: state.CPUAllocatable, MemoryAllocatable: state.MemoryAllocatable,
		CPUUsage: state.CPUUsage, MemoryUsed: state.MemoryUsed,
		MemoryAvailable: state.MemoryAvailable, MetricsAt: state.MetricsAt,
	})
}
