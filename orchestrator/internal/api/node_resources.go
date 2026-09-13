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

type nodeResourceMetadata struct {
	CPUCapacity       int        `json:"cpu_capacity,omitempty"`
	MemoryCapacity    int64      `json:"memory_capacity,omitempty"`
	CPUAllocatable    int        `json:"cpu_allocatable,omitempty"`
	MemoryAllocatable int64      `json:"memory_allocatable,omitempty"`
	CPUUsage          *float64   `json:"cpu_usage,omitempty"`
	MemoryUsed        *int64     `json:"memory_used,omitempty"`
	MemoryAvailable   *int64     `json:"memory_available,omitempty"`
	MetricsAt         *time.Time `json:"metrics_at,omitempty"`
}

func addNodeResourceMetadata(raw []byte, metadata nodeResourceMetadata) ([]byte, error) {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	metadataRaw, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}
	var fields map[string]any
	if err := json.Unmarshal(metadataRaw, &fields); err != nil {
		return nil, err
	}
	for key, value := range fields {
		object[key] = value
	}
	return json.Marshal(object)
}

type nodeRegistrationAlias NodeRegistrationRequest

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
	raw, err := json.Marshal(wireRequest)
	if err != nil {
		return nil, err
	}
	return addNodeResourceMetadata(raw, nodeResourceMetadata{
		CPUCapacity: request.CPU, MemoryCapacity: request.Memory,
		CPUAllocatable: allocatableCPU, MemoryAllocatable: allocatableMemory,
	})
}

func (request *NodeRegistrationRequest) UnmarshalJSON(data []byte) error {
	var wire nodeRegistrationAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var metadata nodeResourceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	*request = NodeRegistrationRequest(wire)
	capacityCPU, capacityMemory := metadata.CPUCapacity, metadata.MemoryCapacity
	allocatableCPU, allocatableMemory := metadata.CPUAllocatable, metadata.MemoryAllocatable
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
	raw, err := json.Marshal(heartbeatAlias(request))
	if err != nil {
		return nil, err
	}
	return addNodeResourceMetadata(raw, nodeResourceMetadata{
		CPUCapacity: state.CPUCapacity, MemoryCapacity: state.MemoryCapacity,
		CPUAllocatable: state.CPUAllocatable, MemoryAllocatable: state.MemoryAllocatable,
		CPUUsage: state.CPUUsage, MemoryUsed: state.MemoryUsed,
		MemoryAvailable: state.MemoryAvailable, MetricsAt: state.MetricsAt,
	})
}

func (request *HeartbeatRequest) UnmarshalJSON(data []byte) error {
	var wire heartbeatAlias
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var metadata nodeResourceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return err
	}
	*request = HeartbeatRequest(wire)
	storeNodeResourceState(request.NodeID, nodeResourceState{
		CPUCapacity: metadata.CPUCapacity, MemoryCapacity: metadata.MemoryCapacity,
		CPUAllocatable: metadata.CPUAllocatable, MemoryAllocatable: metadata.MemoryAllocatable,
		CPUUsage: metadata.CPUUsage, MemoryUsed: metadata.MemoryUsed,
		MemoryAvailable: metadata.MemoryAvailable, MetricsAt: metadata.MetricsAt,
	})
	return nil
}

type nodeResponseAlias NodeResponse

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
	raw, err := json.Marshal(nodeResponseAlias(response))
	if err != nil {
		return nil, err
	}
	return addNodeResourceMetadata(raw, nodeResourceMetadata{
		CPUCapacity: state.CPUCapacity, MemoryCapacity: state.MemoryCapacity,
		CPUAllocatable: state.CPUAllocatable, MemoryAllocatable: state.MemoryAllocatable,
		CPUUsage: state.CPUUsage, MemoryUsed: state.MemoryUsed,
		MemoryAvailable: state.MemoryAvailable, MetricsAt: state.MetricsAt,
	})
}
