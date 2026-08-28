package server

import (
	"bytes"
	"slices"

	"github.com/google/uuid"
)

type PlacementIntent struct {
	JobName       string
	TaskGroupName string
	Count         int
	Nodes         []*Node
	Allocations   []*Allocation
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
	for _, alloc := range intent.Allocations {
		if alloc.Node != nil {
			replicaCounts[alloc.Node.ID]++
		}
	}

	for i := 0; i < intent.Count; i++ {
		var target *Node
		for _, node := range nodes {
			if node.Status != NodeStatusHealthy {
				continue
			}
			if target == nil || replicaCounts[node.ID] < replicaCounts[target.ID] {
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
	}

	return result
}
