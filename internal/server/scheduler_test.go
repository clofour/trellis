package server

import (
	"testing"

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
