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

	slices.SortFunc(intent.Nodes, func(a, b *Node) int {
		return bytes.Compare(a.ID[:], b.ID[:])
	})

	replicaCounts := make(map[uuid.UUID]int)
	for _, alloc := range intent.Allocations {
		replicaCounts[alloc.Node.ID]++
	}

	for i := 0; i < intent.Count; i++ {
		var target *Node
		for _, node := range intent.Nodes {
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
	}

	return result
}
