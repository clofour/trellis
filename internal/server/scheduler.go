package server

import (
	"bytes"
	"slices"

	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

type PlacementIntent struct {
	JobName       string
	TaskGroupName string
	Count         int
	Nodes         []*Node
	Allocations   []*Allocation
	Task          *spec.TaskSpec
}

type Placement struct {
	TaskGroupName string
	NodeID        uuid.UUID
}

func Schedule(intent *PlacementIntent) []Placement {
	result := make([]Placement, 0, intent.Count)

	nodes := slices.Clone(intent.Nodes)
	slices.SortFunc(nodes, func(a, b *Node) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	replicaCounts := make(map[uuid.UUID]int)
	usedCPU := make(map[uuid.UUID]int)
	usedMemory := make(map[uuid.UUID]int64)
	for _, alloc := range intent.Allocations {
		if alloc.Node != nil {
			replicaCounts[alloc.Node.ID]++
			if alloc.Task != nil && alloc.Task.Resources != nil {
				usedCPU[alloc.Node.ID] += alloc.Task.Resources.CPU
				usedMemory[alloc.Node.ID] += int64(alloc.Task.Resources.Memory)
			}
		}
	}

	for i := 0; i < intent.Count; i++ {
		var target *Node
		var reqCPU int
		var reqMemory int64
		if intent.Task != nil && intent.Task.Resources != nil {
			reqCPU = intent.Task.Resources.CPU
			reqMemory = int64(intent.Task.Resources.Memory)
		}
		for _, node := range nodes {
			if node.Status != NodeStatusHealthy {
				continue
			}
			if (node.CPU > 0 && usedCPU[node.ID]+reqCPU > node.CPU) || (node.Memory > 0 && usedMemory[node.ID]+reqMemory > node.Memory) {
				continue
			}
			better := target == nil
			if reqCPU == 0 && reqMemory == 0 {
				better = target == nil || replicaCounts[node.ID] < replicaCounts[target.ID]
			} else if target != nil {
				// Best fit leaves the least aggregate spare capacity.
				spare := int64(node.CPU-usedCPU[node.ID]-reqCPU)*1024*1024 + node.Memory - usedMemory[node.ID] - reqMemory
				targetSpare := int64(target.CPU-usedCPU[target.ID]-reqCPU)*1024*1024 + target.Memory - usedMemory[target.ID] - reqMemory
				better = spare < targetSpare
			}
			if better {
				target = node
			}
		}
		if target == nil {
			break
		}

		result = append(result, Placement{
			TaskGroupName: intent.TaskGroupName,
			NodeID:        target.ID,
		})
		replicaCounts[target.ID]++
		usedCPU[target.ID] += reqCPU
		usedMemory[target.ID] += reqMemory
	}

	return result
}
