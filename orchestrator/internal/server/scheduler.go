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
	Tasks         []spec.TaskSpec
	// Task is deprecated; Tasks represents the colocated scheduling unit.
	Task *spec.TaskSpec
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
			for _, task := range alloc.Tasks {
				if task.Resources != nil {
					usedCPU[alloc.Node.ID] += task.Resources.CPU
					usedMemory[alloc.Node.ID] += int64(task.Resources.Memory)
				}
			}
		}
	}

	for i := 0; i < intent.Count; i++ {
		var target *Node
		var reqCPU int
		var reqMemory int64
		for _, task := range intent.Tasks {
			if task.Resources != nil {
				reqCPU += task.Resources.CPU
				reqMemory += int64(task.Resources.Memory)
			}
		}
		if len(intent.Tasks) == 0 && intent.Task != nil && intent.Task.Resources != nil {
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
				// Best fit compares normalized utilization rather than adding
				// incomparable CPU and byte units.
				utilization := placementUtilization(node, usedCPU[node.ID]+reqCPU, usedMemory[node.ID]+reqMemory)
				targetUtilization := placementUtilization(target, usedCPU[target.ID]+reqCPU, usedMemory[target.ID]+reqMemory)
				better = utilization > targetUtilization
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

func placementUtilization(node *Node, cpu int, memory int64) float64 {
	var cpuRatio, memoryRatio float64
	if node.CPU > 0 {
		cpuRatio = float64(cpu) / float64(node.CPU)
	}
	if node.Memory > 0 {
		memoryRatio = float64(memory) / float64(node.Memory)
	}
	return max(cpuRatio, memoryRatio)
}
