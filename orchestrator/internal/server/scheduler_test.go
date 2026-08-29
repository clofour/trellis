package server

import (
	"testing"

	"github.com/clofour/trellis/internal/spec"
	"github.com/google/uuid"
)

func TestScheduleBalancesAndSkipsUnhealthyNodes(t *testing.T) {
	a := &Node{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Status: NodeStatusHealthy}
	b := &Node{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Status: NodeStatusHealthy}
	bad := &Node{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Status: NodeStatusUnhealthy}

	placements := Schedule(&PlacementIntent{Count: 4, Nodes: []*Node{bad, b, a}})
	if len(placements) != 4 {
		t.Fatalf("got %d placements, want 4", len(placements))
	}
	counts := map[uuid.UUID]int{}
	for _, placement := range placements {
		counts[placement.NodeID]++
	}
	if counts[a.ID] != 2 || counts[b.ID] != 2 || counts[bad.ID] != 0 {
		t.Fatalf("unexpected placement counts: %#v", counts)
	}
}

func TestScheduleRespectsResourcesAndDrainingNodes(t *testing.T) {
	a := &Node{ID: uuid.New(), Status: NodeStatusHealthy, CPU: 1000, Memory: 1024}
	b := &Node{ID: uuid.New(), Status: NodeStatusHealthy, CPU: 2000, Memory: 2048}
	draining := &Node{ID: uuid.New(), Status: NodeStatusDraining, CPU: 10000, Memory: 10000}
	task := &spec.TaskSpec{Resources: &spec.ResourcesSpec{CPU: 750, Memory: 700}}
	placements := Schedule(&PlacementIntent{Count: 4, Nodes: []*Node{a, b, draining}, Task: task})
	if len(placements) != 3 {
		t.Fatalf("got %d placements, want 3", len(placements))
	}
	counts := map[uuid.UUID]int{}
	for _, placement := range placements {
		counts[placement.NodeID]++
	}
	if counts[a.ID] != 1 || counts[b.ID] != 2 || counts[draining.ID] != 0 {
		t.Fatalf("unexpected placements: %#v", counts)
	}
}

func TestScheduleTreatsTaskGroupAsOneResourceUnit(t *testing.T) {
	node := &Node{ID: uuid.New(), Status: NodeStatusHealthy}
	node.CPU, node.Memory = 1000, 1024
	tasks := []spec.TaskSpec{
		{Name: "app", Resources: &spec.ResourcesSpec{CPU: 600, Memory: 256}},
		{Name: "proxy", Resources: &spec.ResourcesSpec{CPU: 500, Memory: 256}},
	}
	placements := Schedule(&PlacementIntent{Count: 1, Nodes: []*Node{node}, Tasks: tasks})
	if len(placements) != 0 {
		t.Fatalf("placed a group whose aggregate task resources exceed the node: %#v", placements)
	}
}
